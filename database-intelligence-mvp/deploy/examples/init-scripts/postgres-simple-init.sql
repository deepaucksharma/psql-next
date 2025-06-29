-- Simple PostgreSQL initialization for Database Intelligence MVP
-- Creates basic test data and structures for monitoring

-- Enable pg_stat_statements extension for query monitoring
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create test table for generating monitoring data
CREATE TABLE IF NOT EXISTS test_table (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    value NUMERIC,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert some initial test data
INSERT INTO test_table (name, value) 
SELECT 
    'test_record_' || i,
    RANDOM() * 1000
FROM generate_series(1, 1000) i
ON CONFLICT DO NOTHING;

-- Create an index for testing
CREATE INDEX IF NOT EXISTS idx_test_table_name ON test_table(name);
CREATE INDEX IF NOT EXISTS idx_test_table_value ON test_table(value);

-- Create a view for testing complex queries
CREATE OR REPLACE VIEW test_summary AS
SELECT 
    LEFT(name, 10) as name_prefix,
    COUNT(*) as record_count,
    AVG(value) as avg_value,
    MAX(value) as max_value,
    MIN(value) as min_value
FROM test_table
GROUP BY LEFT(name, 10);

-- Function to generate load (for testing purposes)
CREATE OR REPLACE FUNCTION generate_test_load()
RETURNS void AS $$
BEGIN
    -- Insert some random data
    INSERT INTO test_table (name, value) 
    VALUES ('load_test_' || NOW()::text, RANDOM() * 1000);
    
    -- Perform some queries that will show up in pg_stat_statements
    PERFORM COUNT(*) FROM test_table;
    PERFORM AVG(value) FROM test_table WHERE value > 500;
    PERFORM * FROM test_summary;
END;
$$ LANGUAGE plpgsql;