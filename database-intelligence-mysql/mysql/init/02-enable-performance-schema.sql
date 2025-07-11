-- Enable Performance Schema consumers for detailed metrics
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME IN (
    'events_statements_current',
    'events_statements_history',
    'events_statements_history_long',
    'events_waits_current',
    'events_waits_history',
    'events_waits_history_long',
    'global_instrumentation',
    'thread_instrumentation',
    'statements_digest'
);

-- Enable statement instrumentation
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'statement/%';

-- Enable wait instrumentation for I/O
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/io/table/%';

UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/io/file/%';

UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/lock/table/%';

-- Enable memory instrumentation
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES' 
WHERE NAME LIKE 'memory/%';

-- Set appropriate history sizes
SET GLOBAL performance_schema_events_statements_history_size = 100;
SET GLOBAL performance_schema_events_statements_history_long_size = 10000;
SET GLOBAL performance_schema_events_waits_history_size = 100;
SET GLOBAL performance_schema_events_waits_history_long_size = 10000;

-- Verify settings
SELECT * FROM performance_schema.setup_consumers WHERE ENABLED = 'YES';
SELECT COUNT(*) AS enabled_instruments FROM performance_schema.setup_instruments WHERE ENABLED = 'YES';