SELECT 
  t.PROCESSLIST_ID as thread_id,
  t.PROCESSLIST_USER as user,
  t.PROCESSLIST_HOST as host,
  t.PROCESSLIST_DB as db,
  t.PROCESSLIST_COMMAND as command,
  t.PROCESSLIST_TIME as time,
  t.PROCESSLIST_STATE as state,
  LEFT(t.PROCESSLIST_INFO, 100) as query,
  ewc.EVENT_NAME as wait_event,
  ewc.TIMER_WAIT/1000000 as wait_ms,
  ewc.OBJECT_SCHEMA,
  ewc.OBJECT_NAME
FROM performance_schema.threads t
LEFT JOIN performance_schema.events_waits_current ewc
  ON t.THREAD_ID = ewc.THREAD_ID
WHERE t.PROCESSLIST_ID IS NOT NULL
  AND t.PROCESSLIST_COMMAND != 'Sleep'
  AND ewc.EVENT_NAME IS NOT NULL
ORDER BY ewc.TIMER_WAIT DESC
LIMIT 50;
