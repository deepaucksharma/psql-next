WITH wait_summary AS (
  SELECT 
    ews.THREAD_ID,
    ews.EVENT_NAME as wait_type,
    COUNT(*) as wait_count,
    SUM(ews.TIMER_WAIT) as total_wait,
    AVG(ews.TIMER_WAIT) as avg_wait,
    MAX(ews.TIMER_WAIT) as max_wait
  FROM performance_schema.events_waits_history_long ews
  WHERE ews.TIMER_WAIT > 0
    AND ews.EVENT_NAME NOT LIKE 'idle%'
    AND ews.END_EVENT_ID IS NOT NULL
  GROUP BY ews.THREAD_ID, ews.EVENT_NAME
),
statement_waits AS (
  SELECT 
    esh.THREAD_ID,
    esh.DIGEST,
    esh.DIGEST_TEXT,
    esh.CURRENT_SCHEMA,
    esh.TIMER_WAIT as statement_time,
    esh.LOCK_TIME,
    esh.ROWS_EXAMINED,
    esh.ROWS_SENT,
    esh.NO_INDEX_USED,
    esh.NO_GOOD_INDEX_USED,
    esh.CREATED_TMP_TABLES,
    esh.CREATED_TMP_DISK_TABLES,
    esh.SELECT_FULL_JOIN,
    esh.SELECT_SCAN
  FROM performance_schema.events_statements_history_long esh
  WHERE esh.DIGEST IS NOT NULL
    AND esh.TIMER_WAIT > 1000000
)
SELECT 
  sw.DIGEST as query_hash,
  LEFT(sw.DIGEST_TEXT, 100) as query_text,
  sw.CURRENT_SCHEMA as db_schema,
  ws.wait_type,
  ws.wait_count,
  ws.total_wait/1000000 as total_wait_ms,
  ws.avg_wait/1000000 as avg_wait_ms,
  ws.max_wait/1000000 as max_wait_ms,
  sw.statement_time/1000000 as statement_time_ms,
  sw.LOCK_TIME/1000000 as lock_time_ms,
  sw.ROWS_EXAMINED,
  sw.NO_INDEX_USED,
  sw.NO_GOOD_INDEX_USED,
  sw.CREATED_TMP_DISK_TABLES as tmp_disk_tables,
  sw.SELECT_FULL_JOIN as full_joins,
  sw.SELECT_SCAN as full_scans,
  COALESCE((ws.total_wait / NULLIF(sw.statement_time, 0)) * 100, 0) as wait_percentage
FROM statement_waits sw
LEFT JOIN wait_summary ws ON sw.THREAD_ID = ws.THREAD_ID
WHERE ws.total_wait > 0
ORDER BY ws.total_wait DESC
LIMIT 100;
