-- PostgreSQL E2E test initialization script

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create test schema with various data types
CREATE SCHEMA IF NOT EXISTS e2e_test;

-- Users table with PII data for testing sanitization
CREATE TABLE e2e_test.users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    ssn VARCHAR(11),
    phone VARCHAR(20),
    credit_card VARCHAR(20),
    name VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Orders table for testing query correlation
CREATE TABLE e2e_test.orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES e2e_test.users(id),
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    total_amount DECIMAL(10,2),
    status VARCHAR(50)
);

-- Large table for testing expensive queries
CREATE TABLE e2e_test.events (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50),
    event_data JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for plan testing
CREATE INDEX idx_users_email ON e2e_test.users(email);
CREATE INDEX idx_orders_user_date ON e2e_test.orders(user_id, order_date);
CREATE INDEX idx_events_type_date ON e2e_test.events(event_type, created_at);

-- Insert test data with PII
INSERT INTO e2e_test.users (email, ssn, phone, credit_card, name) VALUES
    ('john.doe@example.com', '123-45-6789', '555-123-4567', '4111-1111-1111-1111', 'John Doe'),
    ('jane.smith@example.com', '987-65-4321', '555-987-6543', '5500-0000-0000-0004', 'Jane Smith'),
    ('test@example.com', '456-78-9012', '555-456-7890', '3400-0000-0000-009', 'Test User');

-- Insert orders
INSERT INTO e2e_test.orders (user_id, total_amount, status) 
SELECT 
    ((RANDOM() * 2)::INTEGER + 1),  -- Random between 1 and 3
    (RANDOM() * 1000)::DECIMAL(10,2),
    CASE (RANDOM() * 3)::INTEGER 
        WHEN 0 THEN 'pending'
        WHEN 1 THEN 'completed'
        ELSE 'cancelled'
    END
FROM generate_series(1, 100);

-- Insert many events for cardinality testing
INSERT INTO e2e_test.events (event_type, event_data)
SELECT 
    'event_type_' || (RANDOM() * 10)::INTEGER,
    jsonb_build_object(
        'value', RANDOM() * 1000,
        'timestamp', NOW() - (RANDOM() * INTERVAL '30 days')
    )
FROM generate_series(1, 10000);

-- Create functions for generating different query patterns
CREATE OR REPLACE FUNCTION e2e_test.generate_simple_query()
RETURNS TABLE(id INTEGER, email VARCHAR) AS $$
BEGIN
    RETURN QUERY SELECT u.id, u.email FROM e2e_test.users u;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION e2e_test.generate_join_query()
RETURNS TABLE(user_name VARCHAR, order_count BIGINT, total_spent DECIMAL) AS $$
BEGIN
    RETURN QUERY 
    SELECT 
        u.name,
        COUNT(o.id),
        COALESCE(SUM(o.total_amount), 0)
    FROM e2e_test.users u
    LEFT JOIN e2e_test.orders o ON u.id = o.user_id
    GROUP BY u.name;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION e2e_test.generate_expensive_query()
RETURNS TABLE(event_type VARCHAR, event_count BIGINT, avg_value NUMERIC) AS $$
BEGIN
    RETURN QUERY 
    SELECT 
        e.event_type,
        COUNT(*),
        AVG((e.event_data->>'value')::NUMERIC)
    FROM e2e_test.events e
    WHERE e.created_at > NOW() - INTERVAL '7 days'
    GROUP BY e.event_type
    ORDER BY COUNT(*) DESC;
END;
$$ LANGUAGE plpgsql;

-- Enable statement tracking
ALTER SYSTEM SET pg_stat_statements.track = 'all';
ALTER SYSTEM SET pg_stat_statements.track_utility = 'on';
ALTER SYSTEM SET pg_stat_statements.max = 10000;

-- Grant permissions
GRANT USAGE ON SCHEMA e2e_test TO postgres;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA e2e_test TO postgres;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA e2e_test TO postgres;