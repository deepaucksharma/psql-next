SELECT 
  DIGEST as query_hash,
  LEFT(DIGEST_TEXT, 200) as query_text,
  SCHEMA_NAME as db_schema,
  COUNT_STAR as exec_count,
  SUM_TIMER_WAIT/1000000000 as total_time_sec,
  AVG_TIMER_WAIT/1000000 as avg_time_ms,
  MIN_TIMER_WAIT/1000000 as min_time_ms,
  MAX_TIMER_WAIT/1000000 as max_time_ms,
  SUM_LOCK_TIME/1000000 as total_lock_ms,
  SUM_ROWS_EXAMINED as total_rows_examined,
  SUM_ROWS_SENT as total_rows_sent,
  SUM_SELECT_SCAN as full_scans,
  SUM_NO_INDEX_USED as no_index_used_count,
  SUM_NO_GOOD_INDEX_USED as no_good_index_used_count,
  SUM_CREATED_TMP_DISK_TABLES as tmp_disk_tables,
  SUM_SORT_SCAN as sort_scans,
  FIRST_SEEN,
  LAST_SEEN
FROM performance_schema.events_statements_summary_by_digest
WHERE COUNT_STAR > 0
  AND DIGEST_TEXT NOT LIKE '%performance_schema%'
  AND DIGEST_TEXT NOT LIKE '%information_schema%'
ORDER BY SUM_TIMER_WAIT DESC
LIMIT 100;
