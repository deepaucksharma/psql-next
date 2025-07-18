SELECT 
  bt.trx_id,
  bt.trx_state,
  bt.trx_started,
  bt.trx_wait_started,
  TIMESTAMPDIFF(SECOND, bt.trx_wait_started, NOW()) as wait_duration,
  bt.trx_mysql_thread_id as waiting_thread,
  SUBSTRING(bt.trx_query, 1, 100) as waiting_query,
  blt.trx_mysql_thread_id as blocking_thread,
  SUBSTRING(blt.trx_query, 1, 100) as blocking_query,
  l.lock_mode,
  l.lock_type,
  l.object_schema,
  l.object_name as lock_table,
  l.index_name as lock_index
FROM information_schema.innodb_trx bt
JOIN performance_schema.data_lock_waits dlw 
  ON bt.trx_mysql_thread_id = dlw.REQUESTING_THREAD_ID
JOIN information_schema.innodb_trx blt 
  ON dlw.BLOCKING_THREAD_ID = blt.trx_mysql_thread_id
JOIN performance_schema.data_locks l
  ON dlw.REQUESTING_ENGINE_LOCK_ID = l.ENGINE_LOCK_ID
WHERE bt.trx_wait_started IS NOT NULL;
