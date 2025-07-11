-- pg_querylens extension SQL definitions

-- Create schema for extension objects
CREATE SCHEMA IF NOT EXISTS querylens;

-- Statistics view
CREATE OR REPLACE VIEW querylens.query_stats AS
SELECT 
    queryid,
    planid,
    calls,
    total_time::numeric / 1000000.0 AS total_time_seconds,
    mean_time::numeric / 1000.0 AS mean_time_ms,
    rows,
    shared_blks_hit,
    shared_blks_read,
    temp_blks_written,
    last_execution,
    first_seen,
    userid,
    dbid,
    current_database() as database_name
FROM pg_querylens_stats();

COMMENT ON VIEW querylens.query_stats IS 'Query execution statistics collected by pg_querylens';

-- Real-time event stream function
CREATE OR REPLACE FUNCTION querylens.event_stream(
    since_timestamp timestamptz DEFAULT NULL
)
RETURNS TABLE (
    event_time timestamptz,
    event_type text,
    queryid bigint,
    planid bigint,
    execution_time_ms numeric,
    rows_returned bigint,
    database_name text,
    user_name text
)
AS 'MODULE_PATHNAME', 'pg_querylens_event_stream'
LANGUAGE C VOLATILE;

COMMENT ON FUNCTION querylens.event_stream IS 'Stream real-time query execution events';

-- Reset statistics function
CREATE OR REPLACE FUNCTION querylens.reset()
RETURNS void
AS 'MODULE_PATHNAME', 'pg_querylens_reset'
LANGUAGE C VOLATILE;

COMMENT ON FUNCTION querylens.reset IS 'Reset all pg_querylens statistics';

-- Extension info function
CREATE OR REPLACE FUNCTION querylens.info()
RETURNS TABLE (
    setting_name text,
    setting_value text,
    description text
)
AS 'MODULE_PATHNAME', 'pg_querylens_info'
LANGUAGE C STABLE;

COMMENT ON FUNCTION querylens.info IS 'Get pg_querylens configuration and status';

-- Top queries by execution time
CREATE OR REPLACE VIEW querylens.top_queries_by_time AS
SELECT 
    queryid,
    calls,
    total_time_seconds,
    mean_time_ms,
    total_time_seconds / NULLIF(EXTRACT(EPOCH FROM (now() - first_seen)), 0) AS queries_per_second,
    rows / NULLIF(calls, 0) AS avg_rows,
    database_name
FROM querylens.query_stats
ORDER BY total_time_seconds DESC
LIMIT 100;

COMMENT ON VIEW querylens.top_queries_by_time IS 'Top 100 queries by total execution time';

-- Queries with plan changes
CREATE OR REPLACE VIEW querylens.plan_changes AS
WITH plan_history AS (
    SELECT 
        queryid,
        planid,
        first_seen,
        ROW_NUMBER() OVER (PARTITION BY queryid ORDER BY first_seen) as plan_version
    FROM querylens.query_stats
    GROUP BY queryid, planid, first_seen
)
SELECT 
    queryid,
    COUNT(DISTINCT planid) as plan_count,
    MIN(first_seen) as first_seen,
    MAX(first_seen) as last_change
FROM plan_history
GROUP BY queryid
HAVING COUNT(DISTINCT planid) > 1
ORDER BY plan_count DESC;

COMMENT ON VIEW querylens.plan_changes IS 'Queries that have had execution plan changes';

-- Performance regression detection
CREATE OR REPLACE VIEW querylens.performance_regressions AS
WITH recent_performance AS (
    SELECT 
        queryid,
        mean_time_ms,
        calls,
        last_execution,
        AVG(mean_time_ms) OVER (
            PARTITION BY queryid 
            ORDER BY last_execution 
            ROWS BETWEEN 10 PRECEDING AND 1 PRECEDING
        ) as historical_avg
    FROM querylens.query_stats
    WHERE last_execution > now() - interval '1 hour'
)
SELECT 
    queryid,
    mean_time_ms as current_time_ms,
    historical_avg as historical_time_ms,
    (mean_time_ms - historical_avg) / NULLIF(historical_avg, 0) * 100 as pct_increase,
    calls,
    last_execution
FROM recent_performance
WHERE mean_time_ms > historical_avg * 1.5  -- 50% slower than historical average
    AND calls > 10  -- Enough executions to be meaningful
ORDER BY pct_increase DESC;

COMMENT ON VIEW querylens.performance_regressions IS 'Queries showing performance regression';

-- Grant permissions
GRANT USAGE ON SCHEMA querylens TO PUBLIC;
GRANT SELECT ON ALL TABLES IN SCHEMA querylens TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA querylens TO PUBLIC;