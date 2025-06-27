-- Create pg_stat_statements extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create a test schema
CREATE SCHEMA IF NOT EXISTS test_schema;

-- Create a test table
CREATE TABLE IF NOT EXISTS test_schema.test_table (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert some test data
INSERT INTO test_schema.test_table (name) VALUES 
    ('Test Record 1'),
    ('Test Record 2'),
    ('Test Record 3');

-- Create a slow query function for testing
CREATE OR REPLACE FUNCTION test_schema.simulate_slow_query(delay_seconds FLOAT)
RETURNS VOID AS $$
BEGIN
    PERFORM pg_sleep(delay_seconds);
END;
$$ LANGUAGE plpgsql;

-- Execute a slow query to generate metrics
SELECT test_schema.simulate_slow_query(2.0);

-- Create a function to generate blocking sessions
CREATE OR REPLACE FUNCTION test_schema.create_blocking_session()
RETURNS VOID AS $$
BEGIN
    -- This will create a blocking scenario when called
    LOCK TABLE test_schema.test_table IN ACCESS EXCLUSIVE MODE;
    PERFORM pg_sleep(5);
END;
$$ LANGUAGE plpgsql;

-- Reset pg_stat_statements for clean metrics
SELECT pg_stat_statements_reset();