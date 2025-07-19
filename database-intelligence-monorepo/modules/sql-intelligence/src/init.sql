-- Enable performance schema if not already enabled
SET GLOBAL performance_schema = ON;

-- Ensure statement digests are enabled
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME LIKE '%statements%';

-- Enable all statement instruments
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'statement/%';

-- Increase digest text length for better query visibility
SET GLOBAL performance_schema_max_digest_length = 4096;
SET GLOBAL performance_schema_max_sql_text_length = 4096;