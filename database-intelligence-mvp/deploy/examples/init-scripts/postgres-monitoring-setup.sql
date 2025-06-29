-- PostgreSQL monitoring user setup for Database Intelligence MVP
-- Creates a dedicated monitoring user with minimal required privileges

-- Create monitoring user
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT FROM pg_catalog.pg_roles 
        WHERE rolname = 'monitoring'
    ) THEN
        CREATE USER monitoring WITH PASSWORD 'monitoring123';
    END IF;
END
$$;

-- Grant necessary permissions for monitoring
-- Basic connection rights
GRANT CONNECT ON DATABASE testdb TO monitoring;

-- Access to pg_stat_statements for query monitoring
GRANT SELECT ON pg_stat_statements TO monitoring;

-- Access to system catalogs and views for infrastructure monitoring
GRANT SELECT ON pg_stat_activity TO monitoring;
GRANT SELECT ON pg_stat_database TO monitoring;
GRANT SELECT ON pg_stat_user_tables TO monitoring;
GRANT SELECT ON pg_stat_user_indexes TO monitoring;
GRANT SELECT ON pg_statio_user_tables TO monitoring;
GRANT SELECT ON pg_statio_user_indexes TO monitoring;

-- Access to lock information
GRANT SELECT ON pg_locks TO monitoring;

-- Access to replication statistics (if applicable)
GRANT SELECT ON pg_stat_replication TO monitoring;

-- Access to background writer statistics
GRANT SELECT ON pg_stat_bgwriter TO monitoring;

-- Access to checkpoint statistics (PostgreSQL 15+)
GRANT SELECT ON pg_stat_checkpointer TO monitoring;

-- Access to WAL statistics
GRANT SELECT ON pg_stat_wal TO monitoring;

-- Access to test schema for monitoring test queries
GRANT USAGE ON SCHEMA public TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO monitoring;

-- Allow monitoring user to see current settings
GRANT SELECT ON pg_settings TO monitoring;

-- Function to check monitoring user permissions
CREATE OR REPLACE FUNCTION check_monitoring_permissions()
RETURNS TABLE(
    object_type text,
    object_name text,
    has_permission boolean
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        'table'::text as object_type,
        'pg_stat_statements' as object_name,
        has_table_privilege('monitoring', 'pg_stat_statements', 'SELECT') as has_permission
    UNION ALL
    SELECT 
        'table'::text as object_type,
        'pg_stat_activity' as object_name,
        has_table_privilege('monitoring', 'pg_stat_activity', 'SELECT') as has_permission
    UNION ALL
    SELECT 
        'table'::text as object_type,
        'test_table' as object_name,
        has_table_privilege('monitoring', 'test_table', 'SELECT') as has_permission;
END;
$$ LANGUAGE plpgsql;

-- Display current monitoring setup
SELECT 'Monitoring user setup completed' as status;
SELECT * FROM check_monitoring_permissions();