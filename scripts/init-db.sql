-- PostgreSQL initialization script for collector testing

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create test schema
CREATE SCHEMA IF NOT EXISTS test_schema;

-- Create test tables
CREATE TABLE IF NOT EXISTS test_schema.users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255),
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS test_schema.orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES test_schema.users(id),
    amount DECIMAL(10,2),
    status VARCHAR(50),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Create function for generating slow queries
CREATE OR REPLACE FUNCTION test_schema.simulate_slow_query(delay_seconds NUMERIC)
RETURNS void AS $$
BEGIN
    PERFORM pg_sleep(delay_seconds);
END;
$$ LANGUAGE plpgsql;

-- Insert sample data
INSERT INTO test_schema.users (email) 
SELECT 'user' || i || '@example.com' 
FROM generate_series(1, 100) i;

-- Create some initial slow queries for testing
SELECT test_schema.simulate_slow_query(0.5);
SELECT pg_sleep(0.3);

-- Grant permissions to monitoring user
GRANT USAGE ON SCHEMA test_schema TO postgres;
GRANT SELECT ON ALL TABLES IN SCHEMA test_schema TO postgres;