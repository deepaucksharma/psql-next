-- Enable wait analysis in Performance Schema
-- This script configures MySQL for comprehensive wait-time monitoring

-- Enable Performance Schema instrumentation for wait analysis
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/%' 
   OR NAME LIKE 'statement/%'
   OR NAME LIKE 'stage/%';

-- Enable all wait event categories
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES'
WHERE NAME LIKE 'wait/io/%'        -- I/O waits
   OR NAME LIKE 'wait/lock/%'      -- Lock waits
   OR NAME LIKE 'wait/synch/%'     -- Synchronization waits
   OR NAME LIKE 'wait/net/%'       -- Network waits
   OR NAME LIKE 'wait/os/%';       -- OS waits

-- Enable consumers for correlation
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME IN (
  'events_waits_current',
  'events_waits_history',
  'events_waits_history_long',
  'events_statements_current',
  'events_statements_history',
  'events_statements_history_long',
  'events_stages_current',
  'events_stages_history',
  'events_stages_history_long'
);

-- Enable statement digest computation
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME = 'statements_digest';

-- Size history tables appropriately for wait analysis
-- Note: These require restart in MySQL 8.0, so they're set in docker-compose command
-- SET GLOBAL performance_schema_events_waits_history_size = 10000;
-- SET GLOBAL performance_schema_events_waits_history_long_size = 100000;
-- SET GLOBAL performance_schema_events_statements_history_long_size = 10000;

-- Note: Monitor user already created and granted in 01-create-monitoring-user.sql
-- Just ensure user exists
SELECT CONCAT('Monitor user exists: ', user, '@', host) FROM mysql.user WHERE user = 'otel_monitor';

-- Note: Views cannot be created in performance_schema database
-- These queries should be used directly by the monitoring tools

-- Sample query for current wait profile by statement
/*
SELECT 
    esh.DIGEST,
    esh.DIGEST_TEXT,
    COUNT(DISTINCT ews.THREAD_ID) as thread_count,
    COUNT(*) as wait_events,
    SUM(ews.TIMER_WAIT)/1000000000 as total_wait_ms,
    AVG(ews.TIMER_WAIT)/1000000000 as avg_wait_ms,
    MAX(ews.TIMER_WAIT)/1000000000 as max_wait_ms,
    GROUP_CONCAT(DISTINCT 
        SUBSTRING_INDEX(ews.EVENT_NAME, '/', -1) 
        ORDER BY ews.EVENT_NAME
    ) as wait_types
FROM events_statements_history_long esh
JOIN events_waits_history_long ews 
    ON esh.THREAD_ID = ews.THREAD_ID
    AND ews.TIMER_START BETWEEN esh.TIMER_START AND esh.TIMER_END
WHERE esh.DIGEST IS NOT NULL
    AND ews.EVENT_NAME NOT LIKE 'idle%'
GROUP BY esh.DIGEST, esh.DIGEST_TEXT;
*/

-- Sample query for blocking analysis
/*
SELECT 
    waiting.PROCESSLIST_ID as waiting_thread,
    waiting.PROCESSLIST_USER as waiting_user,
    waiting.PROCESSLIST_HOST as waiting_host,
    waiting.PROCESSLIST_DB as waiting_db,
    waiting.PROCESSLIST_COMMAND as waiting_command,
    waiting.PROCESSLIST_TIME as waiting_time,
    waiting.PROCESSLIST_STATE as waiting_state,
    waiting.PROCESSLIST_INFO as waiting_query,
    blocking.PROCESSLIST_ID as blocking_thread,
    blocking.PROCESSLIST_USER as blocking_user,
    blocking.PROCESSLIST_HOST as blocking_host,
    blocking.PROCESSLIST_INFO as blocking_query
FROM threads waiting
JOIN threads blocking 
    ON waiting.PROCESSLIST_ID != blocking.PROCESSLIST_ID
WHERE waiting.PROCESSLIST_COMMAND = 'Query'
    AND waiting.PROCESSLIST_STATE LIKE '%lock%'
    AND blocking.PROCESSLIST_COMMAND != 'Sleep';
*/

-- Flush privileges
FLUSH PRIVILEGES;

-- Log initialization complete
SELECT 'Wait analysis configuration completed' as status;