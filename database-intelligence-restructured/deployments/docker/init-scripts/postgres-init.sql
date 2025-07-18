-- PostgreSQL initialization script for database intelligence monitoring and E2E tests

-- First connect to default postgres database
\c postgres;

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Configure pg_stat_statements
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET pg_stat_statements.track = 'all';
ALTER SYSTEM SET pg_stat_statements.max = 10000;

-- Create monitoring users with necessary permissions
CREATE USER IF NOT EXISTS monitoring WITH PASSWORD 'monitoring_password';
CREATE USER IF NOT EXISTS newrelic_monitor WITH PASSWORD 'monitor123';

-- Grant necessary permissions for monitoring
GRANT pg_monitor TO monitoring;
GRANT CONNECT ON DATABASE postgres TO monitoring;
GRANT pg_read_all_settings TO newrelic_monitor;
GRANT pg_read_all_stats TO newrelic_monitor;

-- Drop and recreate test database
DROP DATABASE IF EXISTS testdb;
CREATE DATABASE testdb;

-- Connect to test database
\c testdb;

-- Enable extensions in test database
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create schemas
CREATE SCHEMA IF NOT EXISTS app;
CREATE SCHEMA IF NOT EXISTS public;

-- Create app schema tables (comprehensive monitoring setup)
CREATE TABLE IF NOT EXISTS app.users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS app.orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES app.users(id),
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) DEFAULT 'pending',
    total_amount DECIMAL(10, 2)
);

CREATE TABLE IF NOT EXISTS app.products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock_quantity INTEGER DEFAULT 0,
    category VARCHAR(100)
);

-- Create public schema tables (simple E2E testing)
CREATE TABLE IF NOT EXISTS public.test_users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS public.test_orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES public.test_users(id),
    amount DECIMAL(10,2),
    status VARCHAR(20),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS public.customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100)
);

CREATE TABLE IF NOT EXISTS public.orders (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER,
    total DECIMAL(10,2)
);

-- Create indexes for performance optimization
CREATE INDEX idx_app_users_email ON app.users(email);
CREATE INDEX idx_app_orders_user_id ON app.orders(user_id);
CREATE INDEX idx_app_orders_status ON app.orders(status);
CREATE INDEX idx_app_products_category ON app.products(category);

-- Insert sample data into app schema
INSERT INTO app.users (username, email) VALUES 
    ('john_doe', 'john@example.com'),
    ('jane_smith', 'jane@example.com'),
    ('test_user', 'test@example.com')
ON CONFLICT (username) DO NOTHING;

INSERT INTO app.products (name, price, stock_quantity, category) VALUES
    ('Product A', 29.99, 100, 'Electronics'),
    ('Product B', 49.99, 50, 'Electronics'),
    ('Product C', 19.99, 200, 'Books')
ON CONFLICT DO NOTHING;

-- Insert sample data into public schema
INSERT INTO public.test_users (username, email) VALUES
    ('user1', 'user1@example.com'),
    ('user2', 'user2@example.com'),
    ('user3', 'user3@example.com')
ON CONFLICT (username) DO NOTHING;

INSERT INTO public.test_orders (user_id, amount, status) VALUES
    (1, 99.99, 'completed'),
    (1, 149.50, 'pending'),
    (2, 75.00, 'completed'),
    (3, 200.00, 'cancelled');

INSERT INTO public.customers (name, email) VALUES 
    ('John Doe', 'john@example.com'),
    ('Jane Smith', 'jane@example.com');

INSERT INTO public.orders (customer_id, total) VALUES 
    (1, 100.00),
    (2, 200.00);

-- Create functions for testing slow queries
CREATE OR REPLACE FUNCTION app.slow_function() RETURNS INTEGER AS $$
DECLARE
    result INTEGER := 0;
BEGIN
    -- Simulate slow operation
    PERFORM pg_sleep(1);
    SELECT COUNT(*) INTO result FROM app.users;
    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Grant monitoring user access to test database and schemas
GRANT CONNECT ON DATABASE testdb TO monitoring;
GRANT CONNECT ON DATABASE testdb TO newrelic_monitor;

-- Grant schema usage
GRANT USAGE ON SCHEMA app TO monitoring;
GRANT USAGE ON SCHEMA public TO monitoring;
GRANT USAGE ON SCHEMA public TO newrelic_monitor;

-- Grant table access
GRANT SELECT ON ALL TABLES IN SCHEMA app TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO newrelic_monitor;

-- Grant function execution
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA app TO monitoring;

-- Enable query tracking and logging
ALTER DATABASE testdb SET log_statement = 'all';
ALTER DATABASE testdb SET log_duration = on;

-- Additional monitoring configuration
ALTER DATABASE testdb SET track_counts = on;
ALTER DATABASE testdb SET track_io_timing = on;
ALTER DATABASE testdb SET track_functions = 'all';