-- Initialize PostgreSQL extensions for monitoring
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Optional extensions for enhanced monitoring
-- CREATE EXTENSION IF NOT EXISTS pg_wait_sampling;
-- CREATE EXTENSION IF NOT EXISTS pg_stat_monitor;

-- Configure pg_stat_statements
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET pg_stat_statements.track = 'all';
ALTER SYSTEM SET pg_stat_statements.max = 10000;

-- Grant necessary permissions
GRANT EXECUTE ON FUNCTION pg_stat_statements_reset() TO postgres;
GRANT pg_monitor TO postgres;