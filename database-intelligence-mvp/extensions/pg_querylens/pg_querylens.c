/*
 * pg_querylens - PostgreSQL extension for zero-overhead query telemetry
 * 
 * This extension implements shared memory based telemetry collection
 * with minimal performance impact on the database.
 */

#include "postgres.h"
#include "fmgr.h"
#include "utils/builtins.h"
#include "utils/timestamp.h"
#include "storage/ipc.h"
#include "storage/lwlock.h"
#include "storage/shmem.h"
#include "storage/spin.h"
#include "utils/guc.h"
#include "executor/executor.h"
#include "tcop/utility.h"
#include "parser/analyze.h"
#include "pgstat.h"

PG_MODULE_MAGIC;

/* GUC variables */
static int querylens_max_queries = 5000;
static int querylens_buffer_size = 1048576; /* 1MB */
static bool querylens_enabled = true;
static int querylens_sample_rate = 100; /* Sample 100% by default */

/* Shared memory structures */
typedef struct QueryLensEntry
{
    uint64      queryid;            /* Query identifier */
    uint64      planid;             /* Plan identifier */
    int64       total_time;         /* Total execution time in microseconds */
    int64       mean_time;          /* Mean execution time */
    int64       calls;              /* Number of executions */
    int64       rows;               /* Total rows returned */
    double      shared_blks_hit;    /* Shared blocks hit */
    double      shared_blks_read;   /* Shared blocks read */
    double      temp_blks_written;  /* Temp blocks written */
    TimestampTz last_execution;     /* Last execution timestamp */
    TimestampTz first_seen;         /* First seen timestamp */
    int32       userid;             /* User ID */
    int32       dbid;               /* Database ID */
    slock_t     mutex;              /* Per-entry spinlock */
} QueryLensEntry;

typedef struct QueryLensSharedState
{
    LWLock     *lock;               /* Protects the hash table */
    int         query_count;        /* Current number of tracked queries */
    int         buffer_pos;         /* Current position in ring buffer */
    Size        buffer_size;        /* Total buffer size */
    QueryLensEntry *entries;        /* Array of entries */
    
    /* Ring buffer for real-time events */
    char       *event_buffer;       /* Circular buffer for events */
    int         event_write_pos;    /* Write position */
    int         event_read_pos;     /* Read position */
    
    /* Statistics */
    int64       total_queries;      /* Total queries processed */
    int64       queries_sampled;    /* Queries actually sampled */
    int64       buffer_overflows;   /* Number of buffer overflows */
    TimestampTz stats_reset_time;   /* Last stats reset */
} QueryLensSharedState;

/* Hook definitions */
static ExecutorStart_hook_type prev_ExecutorStart = NULL;
static ExecutorRun_hook_type prev_ExecutorRun = NULL;
static ExecutorFinish_hook_type prev_ExecutorFinish = NULL;
static ExecutorEnd_hook_type prev_ExecutorEnd = NULL;

/* Shared memory pointer */
static QueryLensSharedState *querylens_state = NULL;

/* Function declarations */
void _PG_init(void);
void _PG_fini(void);

static void querylens_ExecutorStart(QueryDesc *queryDesc, int eflags);
static void querylens_ExecutorRun(QueryDesc *queryDesc, ScanDirection direction,
                                  uint64 count, bool execute_once);
static void querylens_ExecutorFinish(QueryDesc *queryDesc);
static void querylens_ExecutorEnd(QueryDesc *queryDesc);

static void querylens_startup(void);
static void querylens_shmem_startup(void);
static Size querylens_memsize(void);
static QueryLensEntry *querylens_find_entry(uint64 queryid, bool create);
static void querylens_record_query(QueryDesc *queryDesc, double total_time);
static uint64 querylens_compute_planid(PlannedStmt *plan);

/* SQL-callable functions */
PG_FUNCTION_INFO_V1(pg_querylens_stats);
PG_FUNCTION_INFO_V1(pg_querylens_reset);
PG_FUNCTION_INFO_V1(pg_querylens_info);

/*
 * Module initialization
 */
void
_PG_init(void)
{
    /* Define GUC variables */
    DefineCustomIntVariable("pg_querylens.max_queries",
                           "Maximum number of queries to track",
                           NULL,
                           &querylens_max_queries,
                           5000,
                           100,
                           100000,
                           PGC_POSTMASTER,
                           0,
                           NULL,
                           NULL,
                           NULL);

    DefineCustomIntVariable("pg_querylens.buffer_size",
                           "Size of the event buffer in bytes",
                           NULL,
                           &querylens_buffer_size,
                           1048576,
                           65536,
                           10485760,
                           PGC_POSTMASTER,
                           0,
                           NULL,
                           NULL,
                           NULL);

    DefineCustomBoolVariable("pg_querylens.enabled",
                            "Enable query telemetry collection",
                            NULL,
                            &querylens_enabled,
                            true,
                            PGC_SUSET,
                            0,
                            NULL,
                            NULL,
                            NULL);

    DefineCustomIntVariable("pg_querylens.sample_rate",
                           "Percentage of queries to sample (1-100)",
                           NULL,
                           &querylens_sample_rate,
                           100,
                           1,
                           100,
                           PGC_SUSET,
                           0,
                           NULL,
                           NULL,
                           NULL);

    /* Request shared memory */
    RequestAddinShmemSpace(querylens_memsize());
    RequestNamedLWLockTranche("pg_querylens", 1);

    /* Install hooks */
    prev_ExecutorStart = ExecutorStart_hook;
    ExecutorStart_hook = querylens_ExecutorStart;
    
    prev_ExecutorRun = ExecutorRun_hook;
    ExecutorRun_hook = querylens_ExecutorRun;
    
    prev_ExecutorFinish = ExecutorFinish_hook;
    ExecutorFinish_hook = querylens_ExecutorFinish;
    
    prev_ExecutorEnd = ExecutorEnd_hook;
    ExecutorEnd_hook = querylens_ExecutorEnd;

    /* Initialize shared memory */
    prev_shmem_startup_hook = shmem_startup_hook;
    shmem_startup_hook = querylens_shmem_startup;
}

/*
 * Module cleanup
 */
void
_PG_fini(void)
{
    /* Restore previous hooks */
    ExecutorStart_hook = prev_ExecutorStart;
    ExecutorRun_hook = prev_ExecutorRun;
    ExecutorFinish_hook = prev_ExecutorFinish;
    ExecutorEnd_hook = prev_ExecutorEnd;
    shmem_startup_hook = prev_shmem_startup_hook;
}

/*
 * Calculate required shared memory size
 */
static Size
querylens_memsize(void)
{
    Size size;

    size = MAXALIGN(sizeof(QueryLensSharedState));
    size = add_size(size, mul_size(querylens_max_queries, sizeof(QueryLensEntry)));
    size = add_size(size, querylens_buffer_size);

    return size;
}

/*
 * Initialize shared memory
 */
static void
querylens_shmem_startup(void)
{
    bool found;
    Size size = querylens_memsize();

    if (prev_shmem_startup_hook)
        prev_shmem_startup_hook();

    /* Initialize shared memory */
    LWLockAcquire(AddinShmemInitLock, LW_EXCLUSIVE);

    querylens_state = ShmemInitStruct("pg_querylens",
                                     size,
                                     &found);

    if (!found)
    {
        /* First time initialization */
        memset(querylens_state, 0, size);
        
        querylens_state->lock = &(GetNamedLWLockTranche("pg_querylens"))->lock;
        querylens_state->query_count = 0;
        querylens_state->buffer_pos = 0;
        querylens_state->buffer_size = querylens_buffer_size;
        querylens_state->entries = (QueryLensEntry *)
            ((char *) querylens_state + MAXALIGN(sizeof(QueryLensSharedState)));
        querylens_state->event_buffer = (char *) querylens_state->entries +
            mul_size(querylens_max_queries, sizeof(QueryLensEntry));
        querylens_state->event_write_pos = 0;
        querylens_state->event_read_pos = 0;
        querylens_state->stats_reset_time = GetCurrentTimestamp();
        
        /* Initialize entry mutexes */
        for (int i = 0; i < querylens_max_queries; i++)
        {
            SpinLockInit(&querylens_state->entries[i].mutex);
        }
    }

    LWLockRelease(AddinShmemInitLock);
}

/*
 * Executor hooks
 */
static void
querylens_ExecutorStart(QueryDesc *queryDesc, int eflags)
{
    if (!querylens_enabled)
        goto standard_ExecutorStart;

    /* Sample based on configured rate */
    if (random() % 100 >= querylens_sample_rate)
        goto standard_ExecutorStart;

    /* Store start time in queryDesc */
    queryDesc->totaltime = palloc(sizeof(instr_time));
    INSTR_TIME_SET_CURRENT(*(instr_time *)queryDesc->totaltime);

standard_ExecutorStart:
    if (prev_ExecutorStart)
        prev_ExecutorStart(queryDesc, eflags);
    else
        standard_ExecutorStart(queryDesc, eflags);
}

static void
querylens_ExecutorRun(QueryDesc *queryDesc, ScanDirection direction,
                     uint64 count, bool execute_once)
{
    if (prev_ExecutorRun)
        prev_ExecutorRun(queryDesc, direction, count, execute_once);
    else
        standard_ExecutorRun(queryDesc, direction, count, execute_once);
}

static void
querylens_ExecutorFinish(QueryDesc *queryDesc)
{
    if (prev_ExecutorFinish)
        prev_ExecutorFinish(queryDesc);
    else
        standard_ExecutorFinish(queryDesc);
}

static void
querylens_ExecutorEnd(QueryDesc *queryDesc)
{
    if (querylens_enabled && queryDesc->totaltime)
    {
        instr_time end_time;
        double total_time;
        
        INSTR_TIME_SET_CURRENT(end_time);
        INSTR_TIME_SUBTRACT(end_time, *(instr_time *)queryDesc->totaltime);
        total_time = INSTR_TIME_GET_DOUBLE(end_time);
        
        /* Record the query execution */
        querylens_record_query(queryDesc, total_time);
        
        pfree(queryDesc->totaltime);
        queryDesc->totaltime = NULL;
    }

    if (prev_ExecutorEnd)
        prev_ExecutorEnd(queryDesc);
    else
        standard_ExecutorEnd(queryDesc);
}

/*
 * Record query execution details
 */
static void
querylens_record_query(QueryDesc *queryDesc, double total_time)
{
    uint64 queryid;
    uint64 planid;
    QueryLensEntry *entry;
    
    if (!querylens_state)
        return;
    
    /* Get query and plan identifiers */
    queryid = queryDesc->plannedstmt->queryId;
    planid = querylens_compute_planid(queryDesc->plannedstmt);
    
    /* Find or create entry */
    entry = querylens_find_entry(queryid, true);
    if (!entry)
        return;
    
    /* Update entry under spinlock */
    SpinLockAcquire(&entry->mutex);
    
    entry->queryid = queryid;
    entry->planid = planid;
    entry->calls++;
    entry->total_time += (int64)(total_time * 1000000.0); /* Convert to microseconds */
    entry->mean_time = entry->total_time / entry->calls;
    entry->rows += queryDesc->estate->es_processed;
    entry->last_execution = GetCurrentTimestamp();
    
    if (entry->first_seen == 0)
        entry->first_seen = entry->last_execution;
    
    entry->userid = GetUserId();
    entry->dbid = MyDatabaseId;
    
    /* Update block statistics if available */
    if (queryDesc->totaltime && queryDesc->instrument)
    {
        entry->shared_blks_hit += queryDesc->instrument->ntuples;
        entry->shared_blks_read += queryDesc->instrument->nloops;
    }
    
    SpinLockRelease(&entry->mutex);
    
    /* Update global statistics */
    querylens_state->total_queries++;
    querylens_state->queries_sampled++;
}

/*
 * Find or create an entry for a query
 */
static QueryLensEntry *
querylens_find_entry(uint64 queryid, bool create)
{
    QueryLensEntry *entry = NULL;
    int i;
    
    LWLockAcquire(querylens_state->lock, LW_EXCLUSIVE);
    
    /* Search existing entries */
    for (i = 0; i < querylens_state->query_count; i++)
    {
        if (querylens_state->entries[i].queryid == queryid)
        {
            entry = &querylens_state->entries[i];
            break;
        }
    }
    
    /* Create new entry if not found */
    if (!entry && create && querylens_state->query_count < querylens_max_queries)
    {
        entry = &querylens_state->entries[querylens_state->query_count];
        memset(entry, 0, sizeof(QueryLensEntry));
        entry->queryid = queryid;
        querylens_state->query_count++;
    }
    
    LWLockRelease(querylens_state->lock);
    
    return entry;
}

/*
 * Compute a hash of the query plan for change detection
 */
static uint64
querylens_compute_planid(PlannedStmt *plan)
{
    /* Simplified plan hashing - in production, implement proper plan hashing */
    uint64 hash = 0;
    
    if (plan)
    {
        hash = plan->commandType;
        hash = (hash << 16) | (plan->hasReturning ? 1 : 0);
        hash = (hash << 16) | (plan->hasModifyingCTE ? 1 : 0);
        hash = (hash << 16) | list_length(plan->rtable);
    }
    
    return hash;
}

/*
 * SQL function to retrieve statistics
 */
Datum
pg_querylens_stats(PG_FUNCTION_ARGS)
{
    /* Implementation would return a set of statistics records */
    PG_RETURN_NULL();
}

/*
 * SQL function to reset statistics
 */
Datum
pg_querylens_reset(PG_FUNCTION_ARGS)
{
    if (!querylens_state)
        PG_RETURN_VOID();
    
    LWLockAcquire(querylens_state->lock, LW_EXCLUSIVE);
    
    querylens_state->query_count = 0;
    querylens_state->buffer_pos = 0;
    querylens_state->event_write_pos = 0;
    querylens_state->event_read_pos = 0;
    querylens_state->total_queries = 0;
    querylens_state->queries_sampled = 0;
    querylens_state->buffer_overflows = 0;
    querylens_state->stats_reset_time = GetCurrentTimestamp();
    
    memset(querylens_state->entries, 0, 
           mul_size(querylens_max_queries, sizeof(QueryLensEntry)));
    
    LWLockRelease(querylens_state->lock);
    
    PG_RETURN_VOID();
}

/*
 * SQL function to get extension info
 */
Datum
pg_querylens_info(PG_FUNCTION_ARGS)
{
    /* Return extension configuration and status */
    PG_RETURN_NULL();
}