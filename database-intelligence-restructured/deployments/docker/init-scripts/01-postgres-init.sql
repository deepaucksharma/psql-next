-- PostgreSQL initialization script for database intelligence monitoring

-- Create monitoring user with necessary permissions
CREATE USER monitoring WITH PASSWORD 'monitoring_password';

-- Grant necessary permissions for monitoring
GRANT pg_monitor TO monitoring;
GRANT CONNECT ON DATABASE postgres TO monitoring;

-- Enable extensions needed for monitoring
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Configure pg_stat_statements
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET pg_stat_statements.track = 'all';
ALTER SYSTEM SET pg_stat_statements.max = 10000;

-- Create test database and tables
CREATE DATABASE IF NOT EXISTS testdb;

\c testdb;

-- Create sample schema
CREATE SCHEMA IF NOT EXISTS app;

-- Create sample tables
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

-- Create indexes
CREATE INDEX idx_users_email ON app.users(email);
CREATE INDEX idx_orders_user_id ON app.orders(user_id);
CREATE INDEX idx_orders_status ON app.orders(status);
CREATE INDEX idx_products_category ON app.products(category);

-- Insert sample data
INSERT INTO app.users (username, email) VALUES 
    ('john_doe', 'john@example.com'),
    ('jane_smith', 'jane@example.com'),
    ('test_user', 'test@example.com');

INSERT INTO app.products (name, price, stock_quantity, category) VALUES
    ('Product A', 29.99, 100, 'Electronics'),
    ('Product B', 49.99, 50, 'Electronics'),
    ('Product C', 19.99, 200, 'Books');

-- Create some intentionally slow queries for testing
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

-- Grant monitoring user access to test database
GRANT CONNECT ON DATABASE testdb TO monitoring;
GRANT USAGE ON SCHEMA app TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA app TO monitoring;

-- Enable query tracking
ALTER DATABASE testdb SET log_statement = 'all';
ALTER DATABASE testdb SET log_duration = on;