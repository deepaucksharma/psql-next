-- Simple PostgreSQL initialization
-- Enable pg_stat_statements extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create monitoring user
CREATE USER newrelic_monitor WITH PASSWORD 'monitor123';
GRANT pg_read_all_settings TO newrelic_monitor;
GRANT pg_read_all_stats TO newrelic_monitor;

-- Create and populate sample tables
CREATE TABLE customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100)
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER,
    total DECIMAL(10,2)
);

-- Insert sample data
INSERT INTO customers (name, email) VALUES 
('John Doe', 'john@example.com'),
('Jane Smith', 'jane@example.com');

INSERT INTO orders (customer_id, total) VALUES 
(1, 100.00),
(2, 200.00);

-- Grant permissions
GRANT CONNECT ON DATABASE testdb TO newrelic_monitor;
GRANT USAGE ON SCHEMA public TO newrelic_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO newrelic_monitor;