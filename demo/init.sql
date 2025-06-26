-- Initialize PostgreSQL for monitoring
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create test table for workload
CREATE TABLE IF NOT EXISTS test_table (
    id SERIAL PRIMARY KEY,
    data TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert some initial data
INSERT INTO test_table (data) 
SELECT md5(random()::text) 
FROM generate_series(1, 1000);

-- Create monitoring user (for production use)
-- CREATE USER monitoring WITH PASSWORD 'monitoring123';
-- GRANT pg_monitor TO monitoring;
-- GRANT CONNECT ON DATABASE testdb TO monitoring;
-- GRANT USAGE ON SCHEMA public TO monitoring;
-- GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring;