/// OHI-compatible query strings
pub mod ohi_queries {
    pub const SLOW_QUERIES_V12: &str = r#"
        SELECT 'newrelic' as newrelic,
            pss.queryid AS query_id,
            LEFT(pss.query, 4095) AS query_text,
            pd.datname AS database_name,
            current_schema() AS schema_name,
            pss.calls AS execution_count,
            ROUND((pss.total_time / pss.calls)::numeric, 3) AS avg_elapsed_time_ms,
            pss.shared_blks_read / pss.calls AS avg_disk_reads,
            pss.shared_blks_written / pss.calls AS avg_disk_writes,
            CASE
                WHEN pss.query ILIKE 'SELECT%%' THEN 'SELECT'
                WHEN pss.query ILIKE 'INSERT%%' THEN 'INSERT'
                WHEN pss.query ILIKE 'UPDATE%%' THEN 'UPDATE'
                WHEN pss.query ILIKE 'DELETE%%' THEN 'DELETE'
                ELSE 'OTHER'
            END AS statement_type,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM pg_stat_statements pss
        JOIN pg_database pd ON pss.dbid = pd.oid
        WHERE pd.datname in (%s)
            AND pss.query NOT ILIKE 'EXPLAIN (FORMAT JSON)%%'
            AND pss.query NOT ILIKE 'SELECT $1 as newrelic%%'
            AND pss.query NOT ILIKE 'WITH wait_history AS%%'
            AND pss.query NOT ILIKE 'select -- BLOATQUERY%%'
            AND pss.query NOT ILIKE 'select -- INDEXQUERY%%'
            AND pss.query NOT ILIKE 'SELECT -- TABLEQUERY%%'
            AND pss.query NOT ILIKE 'SELECT table_schema%%'
        ORDER BY avg_elapsed_time_ms DESC
        LIMIT %d;
    "#;
    
    pub const SLOW_QUERIES_V13_ABOVE: &str = r#"
        SELECT 'newrelic' as newrelic,
            pss.queryid AS query_id,
            LEFT(pss.query, 4095) AS query_text,
            pd.datname AS database_name,
            current_schema() AS schema_name,
            pss.calls AS execution_count,
            ROUND((pss.total_exec_time / pss.calls)::numeric, 3) AS avg_elapsed_time_ms,
            pss.shared_blks_read / pss.calls AS avg_disk_reads,
            pss.shared_blks_written / pss.calls AS avg_disk_writes,
            CASE
                WHEN pss.query ILIKE 'SELECT%%' THEN 'SELECT'
                WHEN pss.query ILIKE 'INSERT%%' THEN 'INSERT'
                WHEN pss.query ILIKE 'UPDATE%%' THEN 'UPDATE'
                WHEN pss.query ILIKE 'DELETE%%' THEN 'DELETE'
                ELSE 'OTHER'
            END AS statement_type,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM pg_stat_statements pss
        JOIN pg_database pd ON pss.dbid = pd.oid
        WHERE pd.datname in (%s)
            AND pss.query NOT ILIKE 'EXPLAIN (FORMAT JSON)%%'
            AND pss.query NOT ILIKE 'SELECT $1 as newrelic%%'
            AND pss.query NOT ILIKE 'WITH wait_history AS%%'
            AND pss.query NOT ILIKE 'select -- BLOATQUERY%%'
            AND pss.query NOT ILIKE 'select -- INDEXQUERY%%'
            AND pss.query NOT ILIKE 'SELECT -- TABLEQUERY%%'
            AND pss.query NOT ILIKE 'SELECT table_schema%%'
        ORDER BY avg_elapsed_time_ms DESC
        LIMIT %d;
    "#;
    
    pub const WAIT_EVENTS: &str = r#"
        WITH wait_history AS (
            SELECT 
                event_time,
                pid,
                wait_event_type,
                wait_event,
                LAG(event_time) OVER (PARTITION BY pid ORDER BY event_time) AS prev_time
            FROM pg_wait_sampling_history
        )
        SELECT
            wh.pid,
            wh.wait_event_type,
            wh.wait_event,
            EXTRACT(EPOCH FROM (wh.event_time - wh.prev_time)) * 1000 AS wait_time_ms,
            psa.state,
            psa.usename,
            psa.datname AS database_name,
            psa.query_id,
            LEFT(psa.query, 4095) AS query_text,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM wait_history wh
        JOIN pg_stat_activity psa ON wh.pid = psa.pid
        WHERE wh.prev_time IS NOT NULL
            AND psa.datname IN (%s)
            AND psa.state != 'idle'
        ORDER BY wait_time_ms DESC
        LIMIT %d;
    "#;
    
    pub const WAIT_EVENTS_RDS: &str = r#"
        SELECT
            pid,
            wait_event_type,
            wait_event,
            0 AS wait_time_ms,
            state,
            usename,
            datname AS database_name,
            query_id,
            LEFT(query, 4095) AS query_text,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM pg_stat_activity
        WHERE datname IN (%s)
            AND state != 'idle'
            AND wait_event IS NOT NULL
        LIMIT %d;
    "#;
    
    pub const BLOCKING_V12_13: &str = r#"
        SELECT
            blocking.pid AS blocking_pid,
            blocked.pid AS blocked_pid,
            LEFT(blocking.query, 4095) AS blocking_query,
            LEFT(blocked.query, 4095) AS blocked_query,
            blocking.datname AS blocking_database,
            blocked.datname AS blocked_database,
            blocking.usename AS blocking_user,
            blocked.usename AS blocked_user,
            EXTRACT(EPOCH FROM (NOW() - blocking.query_start)) * 1000 AS blocking_duration_ms,
            EXTRACT(EPOCH FROM (NOW() - blocked.query_start)) * 1000 AS blocked_duration_ms,
            'AccessShareLock' AS lock_type,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM pg_stat_activity blocked
        JOIN pg_stat_activity blocking ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
        WHERE blocked.datname IN (%s)
        LIMIT %d;
    "#;
    
    pub const BLOCKING_V14_ABOVE: &str = r#"
        WITH blocking_tree AS (
            SELECT
                blocking.pid AS blocking_pid,
                blocked.pid AS blocked_pid,
                blocking.queryid AS blocking_queryid,
                blocked.queryid AS blocked_queryid,
                LEFT(blocking.query, 4095) AS blocking_query,
                LEFT(blocked.query, 4095) AS blocked_query,
                blocking.datname AS blocking_database,
                blocked.datname AS blocked_database,
                blocking.usename AS blocking_user,
                blocked.usename AS blocked_user,
                EXTRACT(EPOCH FROM (NOW() - blocking.query_start)) * 1000 AS blocking_duration_ms,
                EXTRACT(EPOCH FROM (NOW() - blocked.query_start)) * 1000 AS blocked_duration_ms,
                'AccessShareLock' AS lock_type
            FROM pg_stat_activity blocked
            JOIN pg_stat_activity blocking ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
            WHERE blocked.datname IN (%s)
        )
        SELECT
            blocking_pid,
            blocked_pid,
            blocking_query,
            blocked_query,
            blocking_database,
            blocked_database,
            blocking_user,
            blocked_user,
            blocking_duration_ms,
            blocked_duration_ms,
            lock_type,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM blocking_tree
        LIMIT %d;
    "#;
    
    pub const BLOCKING_RDS: &str = r#"
        SELECT
            blockers.pid AS blocking_pid,
            waiters.pid AS blocked_pid,
            LEFT(blockers.query, 4095) AS blocking_query,
            LEFT(waiters.query, 4095) AS blocked_query,
            blockers.datname AS blocking_database,
            waiters.datname AS blocked_database,
            blockers.usename AS blocking_user,
            waiters.usename AS blocked_user,
            EXTRACT(EPOCH FROM (NOW() - blockers.query_start)) * 1000 AS blocking_duration_ms,
            EXTRACT(EPOCH FROM (NOW() - waiters.query_start)) * 1000 AS blocked_duration_ms,
            waiters.wait_event_type AS lock_type,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM pg_stat_activity waiters
        JOIN pg_stat_activity blockers ON blockers.pid = ANY(pg_blocking_pids(waiters.pid))
        WHERE waiters.wait_event_type = 'Lock'
            AND waiters.datname IN (%s)
        LIMIT %d;
    "#;
    
    pub const INDIVIDUAL_V12: &str = r#"
        SELECT
            pid,
            queryid AS query_id,
            LEFT(query, 4095) AS query_text,
            state,
            wait_event_type,
            wait_event,
            usename,
            datname AS database_name,
            to_char(backend_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS backend_start,
            to_char(xact_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS xact_start,
            to_char(query_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS query_start,
            to_char(state_change AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS state_change,
            backend_type,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM pg_stat_activity
        WHERE datname IN (%s)
            AND state != 'idle'
            AND pid != pg_backend_pid()
        LIMIT %d;
    "#;
    
    pub const INDIVIDUAL_V13_ABOVE: &str = r#"
        SELECT
            pid,
            queryid AS query_id,
            LEFT(query, 4095) AS query_text,
            state,
            wait_event_type,
            wait_event,
            usename,
            datname AS database_name,
            to_char(backend_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS backend_start,
            to_char(xact_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS xact_start,
            to_char(query_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS query_start,
            to_char(state_change AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS state_change,
            backend_type,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM pg_stat_activity
        WHERE datname IN (%s)
            AND state != 'idle'
            AND pid != pg_backend_pid()
        LIMIT %d;
    "#;
    
    pub const EXTENSION_CHECK: &str = "SELECT extname FROM pg_extension;";
    
    pub const VERSION_CHECK: &str = "SELECT current_setting('server_version_num')::integer / 10000 AS version;";
}

/// Extended queries for enhanced metrics
pub mod extended_queries {
    pub const ASH_SAMPLE: &str = r#"
        SELECT
            NOW() as sample_time,
            pid,
            usename,
            datname,
            queryid as query_id,
            state,
            wait_event_type,
            wait_event,
            LEFT(query, 4095) as query,
            backend_type
        FROM pg_stat_activity
        WHERE state != 'idle'
            AND pid != pg_backend_pid();
    "#;
    
    pub const PLAN_HISTORY: &str = r#"
        SELECT DISTINCT ON (queryid)
            queryid AS query_id,
            query,
            plan,
            plans AS plan_count,
            total_plan_time,
            mean_plan_time
        FROM pg_stat_statements
        WHERE queryid IS NOT NULL
        ORDER BY queryid, calls DESC;
    "#;
    
    pub const BUFFER_STATS_DETAIL: &str = r#"
        SELECT
            queryid AS query_id,
            shared_blks_hit,
            shared_blks_read,
            shared_blks_dirtied,
            shared_blks_written,
            local_blks_hit,
            local_blks_read,
            local_blks_dirtied,
            local_blks_written,
            temp_blks_read,
            temp_blks_written,
            blk_read_time,
            blk_write_time
        FROM pg_stat_statements
        WHERE queryid IS NOT NULL;
    "#;
}