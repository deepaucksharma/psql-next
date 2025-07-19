-- Enable performance schema wait events
SET GLOBAL performance_schema = ON;

-- Enable wait event consumers
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME IN (
  'events_waits_current',
  'events_waits_history',
  'events_waits_history_long',
  'global_instrumentation',
  'thread_instrumentation'
);

-- Enable wait instruments
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/%';

-- Enable file I/O instruments
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/io/file/%';

-- Enable lock instruments
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/lock/%';

-- Enable mutex instruments
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/synch/mutex/%';