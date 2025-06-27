use anyhow::Result;
use sqlx::{PgConnection, Row};
use postgres_collector_core::{
    CollectorError, CommonParameters, 
    SlowQueryMetric, WaitEventMetric, BlockingSessionMetric,
    IndividualQueryMetric,
};

/// Safe parameterized queries for PostgreSQL metrics collection
pub struct SafeQueryExecutor;

impl SafeQueryExecutor {
    pub async fn execute_slow_queries(
        conn: &mut PgConnection,
        params: &CommonParameters,
        version: u64,
    ) -> Result<Vec<SlowQueryMetric>, CollectorError> {
        let databases: Vec<&str> = params.databases.split(',').collect();
        let limit = params.query_monitoring_count_threshold;
        
        let query = if version >= 13 {
            r#"
            SELECT 
                pss.queryid AS query_id,
                LEFT(pss.query, 4095) AS query_text,
                pd.datname AS database_name,
                current_schema() AS schema_name,
                pss.calls AS execution_count,
                (pss.total_exec_time / NULLIF(pss.calls, 0)) AS avg_elapsed_time_ms,
                (pss.shared_blks_read::float8 / NULLIF(pss.calls, 0)) AS avg_disk_reads,
                (pss.shared_blks_written::float8 / NULLIF(pss.calls, 0)) AS avg_disk_writes,
                CASE
                    WHEN pss.query ILIKE 'SELECT%' THEN 'SELECT'
                    WHEN pss.query ILIKE 'INSERT%' THEN 'INSERT'
                    WHEN pss.query ILIKE 'UPDATE%' THEN 'UPDATE'
                    WHEN pss.query ILIKE 'DELETE%' THEN 'DELETE'
                    ELSE 'OTHER'
                END AS statement_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp,
                NULL::text AS individual_query
            FROM pg_stat_statements pss
            JOIN pg_database pd ON pss.dbid = pd.oid
            WHERE pd.datname = ANY($1)
                AND pss.calls >= $2
                AND (pss.total_exec_time / NULLIF(pss.calls, 0)) >= $3
                AND pss.query NOT ILIKE 'EXPLAIN (FORMAT JSON)%'
            ORDER BY avg_elapsed_time_ms DESC
            LIMIT $4
            "#
        } else {
            r#"
            SELECT 
                pss.queryid AS query_id,
                LEFT(pss.query, 4095) AS query_text,
                pd.datname AS database_name,
                current_schema() AS schema_name,
                pss.calls AS execution_count,
                (pss.total_time / NULLIF(pss.calls, 0)) AS avg_elapsed_time_ms,
                (pss.shared_blks_read::float8 / NULLIF(pss.calls, 0)) AS avg_disk_reads,
                (pss.shared_blks_written::float8 / NULLIF(pss.calls, 0)) AS avg_disk_writes,
                CASE
                    WHEN pss.query ILIKE 'SELECT%' THEN 'SELECT'
                    WHEN pss.query ILIKE 'INSERT%' THEN 'INSERT'
                    WHEN pss.query ILIKE 'UPDATE%' THEN 'UPDATE'
                    WHEN pss.query ILIKE 'DELETE%' THEN 'DELETE'
                    ELSE 'OTHER'
                END AS statement_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp,
                NULL::text AS individual_query
            FROM pg_stat_statements pss
            JOIN pg_database pd ON pss.dbid = pd.oid
            WHERE pd.datname = ANY($1)
                AND pss.calls >= $2
                AND (pss.total_time / NULLIF(pss.calls, 0)) >= $3
                AND pss.query NOT ILIKE 'EXPLAIN (FORMAT JSON)%'
            ORDER BY avg_elapsed_time_ms DESC
            LIMIT $4
            "#
        };

        let rows = sqlx::query(query)
            .bind(&databases)
            .bind(params.query_monitoring_count_threshold)
            .bind(params.query_monitoring_response_time_threshold as f64)
            .bind(limit)
            .fetch_all(conn)
            .await?;

        let mut metrics = Vec::new();
        for row in rows {
            metrics.push(SlowQueryMetric {
                query_id: row.get("query_id"),
                query_text: row.get("query_text"),
                database_name: row.get("database_name"),
                schema_name: row.get("schema_name"),
                execution_count: row.get("execution_count"),
                avg_elapsed_time_ms: row.get("avg_elapsed_time_ms"),
                avg_disk_reads: row.get("avg_disk_reads"),
                avg_disk_writes: row.get("avg_disk_writes"),
                statement_type: row.get("statement_type"),
                collection_timestamp: row.get("collection_timestamp"),
                individual_query: row.get("individual_query"),
                extended_metrics: None,
            });
        }

        Ok(metrics)
    }

    pub async fn execute_wait_events(
        conn: &mut PgConnection,
        params: &CommonParameters,
        is_rds: bool,
    ) -> Result<Vec<WaitEventMetric>, CollectorError> {
        let databases: Vec<&str> = params.databases.split(',').collect();
        
        let query = if is_rds {
            // RDS mode: Direct from pg_stat_activity
            r#"
            SELECT 
                psa.pid,
                psa.wait_event_type,
                psa.wait_event,
                EXTRACT(EPOCH FROM (NOW() - psa.query_start)) * 1000 AS wait_time_ms,
                psa.state,
                psa.usename,
                psa.datname AS database_name,
                pss.queryid AS query_id,
                LEFT(psa.query, 4095) AS query_text,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM pg_stat_activity psa
            LEFT JOIN pg_stat_statements pss ON psa.query = pss.query
            WHERE psa.datname = ANY($1)
                AND psa.wait_event IS NOT NULL
                AND psa.state != 'idle'
            ORDER BY wait_time_ms DESC
            LIMIT 100
            "#
        } else {
            // Non-RDS mode: Use pg_wait_sampling if available
            r#"
            WITH wait_history AS (
                SELECT 
                    event_time,
                    pid,
                    wait_event_type,
                    wait_event,
                    LAG(event_time) OVER (PARTITION BY pid ORDER BY event_time) AS prev_time
                FROM pg_wait_sampling_history
                WHERE event_time > NOW() - INTERVAL '5 minutes'
            )
            SELECT
                wh.pid,
                wh.wait_event_type,
                wh.wait_event,
                EXTRACT(EPOCH FROM (wh.event_time - COALESCE(wh.prev_time, wh.event_time))) * 1000 AS wait_time_ms,
                psa.state,
                psa.usename,
                psa.datname AS database_name,
                pss.queryid AS query_id,
                LEFT(psa.query, 4095) AS query_text,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM wait_history wh
            JOIN pg_stat_activity psa ON wh.pid = psa.pid
            LEFT JOIN pg_stat_statements pss ON psa.query = pss.query
            WHERE psa.datname = ANY($1)
                AND wh.wait_event IS NOT NULL
            ORDER BY wait_time_ms DESC
            LIMIT 100
            "#
        };

        let rows = sqlx::query(query)
            .bind(&databases)
            .fetch_all(conn)
            .await?;

        let mut metrics = Vec::new();
        for row in rows {
            metrics.push(WaitEventMetric {
                pid: row.get("pid"),
                wait_event_type: row.get("wait_event_type"),
                wait_event: row.get("wait_event"),
                wait_time_ms: row.get("wait_time_ms"),
                state: row.get("state"),
                usename: row.get("usename"),
                database_name: row.get("database_name"),
                query_id: row.get("query_id"),
                query_text: row.get("query_text"),
                collection_timestamp: row.get("collection_timestamp"),
            });
        }

        Ok(metrics)
    }

    pub async fn execute_blocking_sessions(
        conn: &mut PgConnection,
        params: &CommonParameters,
        version: u64,
        is_rds: bool,
    ) -> Result<Vec<BlockingSessionMetric>, CollectorError> {
        let databases: Vec<&str> = params.databases.split(',').collect();
        
        let query = if is_rds {
            // RDS simplified blocking query
            r#"
            WITH blocking_tree AS (
                SELECT
                    blockers.pid AS blocking_pid,
                    blockers.query AS blocking_query,
                    blockers.datname AS blocking_database,
                    blockers.usename AS blocking_user,
                    EXTRACT(EPOCH FROM (NOW() - blockers.query_start)) * 1000 AS blocking_duration_ms,
                    blocked.pid AS blocked_pid,
                    blocked.query AS blocked_query,
                    blocked.datname AS blocked_database,
                    blocked.usename AS blocked_user,
                    EXTRACT(EPOCH FROM (NOW() - blocked.query_start)) * 1000 AS blocked_duration_ms,
                    'Lock' AS lock_type
                FROM pg_stat_activity blockers
                JOIN pg_stat_activity blocked ON true
                WHERE blockers.pid = ANY(
                    SELECT blocking_pid 
                    FROM pg_locks 
                    WHERE NOT granted
                )
                AND blocked.pid = ANY(
                    SELECT pid 
                    FROM pg_locks 
                    WHERE NOT granted
                )
                AND blockers.pid != blocked.pid
                AND blockers.datname = ANY($1)
            )
            SELECT 
                blocking_pid,
                blocked_pid,
                LEFT(blocking_query, 4095) AS blocking_query,
                LEFT(blocked_query, 4095) AS blocked_query,
                blocking_database,
                blocked_database,
                blocking_user,
                blocked_user,
                blocking_duration_ms,
                blocked_duration_ms,
                lock_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM blocking_tree
            LIMIT 100
            "#
        } else if version >= 14 {
            // PostgreSQL 14+ with better lock info
            r#"
            WITH blocking_tree AS (
                SELECT
                    blockers.pid AS blocking_pid,
                    blockers.query AS blocking_query,
                    blockers.datname AS blocking_database,
                    blockers.usename AS blocking_user,
                    EXTRACT(EPOCH FROM (NOW() - blockers.query_start)) * 1000 AS blocking_duration_ms,
                    blocked.pid AS blocked_pid,
                    blocked.query AS blocked_query,
                    blocked.datname AS blocked_database,
                    blocked.usename AS blocked_user,
                    EXTRACT(EPOCH FROM (NOW() - blocked.query_start)) * 1000 AS blocked_duration_ms,
                    blocked_locks.locktype AS lock_type
                FROM pg_locks blocked_locks
                JOIN pg_stat_activity blocked ON blocked.pid = blocked_locks.pid
                JOIN pg_locks blocking_locks ON 
                    blocking_locks.locktype = blocked_locks.locktype
                    AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
                    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
                    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
                    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
                    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
                    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
                    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
                    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
                    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
                    AND blocking_locks.pid != blocked_locks.pid
                JOIN pg_stat_activity blockers ON blockers.pid = blocking_locks.pid
                WHERE NOT blocked_locks.granted
                    AND blocking_locks.granted
                    AND blocked.datname = ANY($1)
            )
            SELECT 
                blocking_pid,
                blocked_pid,
                LEFT(blocking_query, 4095) AS blocking_query,
                LEFT(blocked_query, 4095) AS blocked_query,
                blocking_database,
                blocked_database,
                blocking_user,
                blocked_user,
                blocking_duration_ms,
                blocked_duration_ms,
                lock_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM blocking_tree
            LIMIT 100
            "#
        } else {
            // PostgreSQL 12-13
            r#"
            WITH blocking_tree AS (
                SELECT
                    blockers.pid AS blocking_pid,
                    blockers.query AS blocking_query,
                    blockers.datname AS blocking_database,
                    blockers.usename AS blocking_user,
                    EXTRACT(EPOCH FROM (NOW() - blockers.query_start)) * 1000 AS blocking_duration_ms,
                    blocked.pid AS blocked_pid,
                    blocked.query AS blocked_query,
                    blocked.datname AS blocked_database,
                    blocked.usename AS blocked_user,
                    EXTRACT(EPOCH FROM (NOW() - blocked.query_start)) * 1000 AS blocked_duration_ms,
                    'Lock' AS lock_type
                FROM pg_stat_activity blockers
                JOIN pg_stat_activity blocked ON true
                WHERE blockers.pid IN (
                    SELECT DISTINCT blocking_pid 
                    FROM (
                        SELECT 
                            kl.pid AS blocking_pid,
                            bl.pid AS blocked_pid
                        FROM pg_locks bl
                        JOIN pg_locks kl ON 
                            kl.locktype = bl.locktype
                            AND kl.database IS NOT DISTINCT FROM bl.database
                            AND kl.relation IS NOT DISTINCT FROM bl.relation
                            AND kl.page IS NOT DISTINCT FROM bl.page
                            AND kl.tuple IS NOT DISTINCT FROM bl.tuple
                            AND kl.virtualxid IS NOT DISTINCT FROM bl.virtualxid
                            AND kl.transactionid IS NOT DISTINCT FROM bl.transactionid
                            AND kl.classid IS NOT DISTINCT FROM bl.classid
                            AND kl.objid IS NOT DISTINCT FROM bl.objid
                            AND kl.objsubid IS NOT DISTINCT FROM bl.objsubid
                            AND kl.pid != bl.pid
                        WHERE NOT bl.granted AND kl.granted
                    ) AS l
                )
                AND blocked.pid IN (
                    SELECT pid FROM pg_locks WHERE NOT granted
                )
                AND blockers.datname = ANY($1)
            )
            SELECT 
                blocking_pid,
                blocked_pid,
                LEFT(blocking_query, 4095) AS blocking_query,
                LEFT(blocked_query, 4095) AS blocked_query,
                blocking_database,
                blocked_database,
                blocking_user,
                blocked_user,
                blocking_duration_ms,
                blocked_duration_ms,
                lock_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM blocking_tree
            LIMIT 100
            "#
        };

        let rows = sqlx::query(query)
            .bind(&databases)
            .fetch_all(conn)
            .await?;

        let mut metrics = Vec::new();
        for row in rows {
            metrics.push(BlockingSessionMetric {
                blocking_pid: row.get("blocking_pid"),
                blocked_pid: row.get("blocked_pid"),
                blocking_query: row.get("blocking_query"),
                blocked_query: row.get("blocked_query"),
                blocking_database: row.get("blocking_database"),
                blocked_database: row.get("blocked_database"),
                blocking_user: row.get("blocking_user"),
                blocked_user: row.get("blocked_user"),
                blocking_duration_ms: row.get("blocking_duration_ms"),
                blocked_duration_ms: row.get("blocked_duration_ms"),
                lock_type: row.get("lock_type"),
                collection_timestamp: row.get("collection_timestamp"),
            });
        }

        Ok(metrics)
    }

    pub async fn execute_individual_queries(
        conn: &mut PgConnection,
        params: &CommonParameters,
        version: u64,
        is_rds: bool,
    ) -> Result<Vec<IndividualQueryMetric>, CollectorError> {
        let databases: Vec<&str> = params.databases.split(',').collect();
        
        let query = if is_rds || version < 13 {
            // RDS or older versions: Direct from pg_stat_activity
            r#"
            SELECT 
                psa.pid,
                pss.queryid AS query_id,
                LEFT(psa.query, 4095) AS query_text,
                psa.state,
                psa.wait_event_type,
                psa.wait_event,
                psa.usename,
                psa.datname AS database_name,
                to_char(psa.backend_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS backend_start,
                to_char(psa.xact_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS xact_start,
                to_char(psa.query_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS query_start,
                to_char(psa.state_change AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS state_change,
                psa.backend_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM pg_stat_activity psa
            LEFT JOIN pg_stat_statements pss ON psa.query = pss.query
            WHERE psa.datname = ANY($1)
                AND psa.query IS NOT NULL 
                AND psa.query != ''
                AND psa.state != 'idle'
            ORDER BY psa.query_start
            LIMIT 100
            "#
        } else {
            // PostgreSQL 13+ with better activity tracking
            r#"
            SELECT 
                psa.pid,
                pss.queryid AS query_id,
                LEFT(psa.query, 4095) AS query_text,
                psa.state,
                psa.wait_event_type,
                psa.wait_event,
                psa.usename,
                psa.datname AS database_name,
                to_char(psa.backend_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS backend_start,
                to_char(psa.xact_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS xact_start,
                to_char(psa.query_start AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS query_start,
                to_char(psa.state_change AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS state_change,
                psa.backend_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
            FROM pg_stat_activity psa
            LEFT JOIN pg_stat_statements pss ON 
                psa.query = pss.query 
                AND psa.datid = pss.dbid
            WHERE psa.datname = ANY($1)
                AND psa.query IS NOT NULL 
                AND psa.query != ''
                AND psa.state != 'idle'
                AND psa.backend_type = 'client backend'
            ORDER BY psa.query_start
            LIMIT 100
            "#
        };

        let rows = sqlx::query(query)
            .bind(&databases)
            .fetch_all(conn)
            .await?;

        let mut metrics = Vec::new();
        for row in rows {
            metrics.push(IndividualQueryMetric {
                pid: row.get("pid"),
                query_id: row.get("query_id"),
                query_text: row.get("query_text"),
                state: row.get("state"),
                wait_event_type: row.get("wait_event_type"),
                wait_event: row.get("wait_event"),
                usename: row.get("usename"),
                database_name: row.get("database_name"),
                backend_start: row.get("backend_start"),
                xact_start: row.get("xact_start"),
                query_start: row.get("query_start"),
                state_change: row.get("state_change"),
                backend_type: row.get("backend_type"),
                collection_timestamp: row.get("collection_timestamp"),
            });
        }

        Ok(metrics)
    }
}