-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Create test schema
CREATE SCHEMA IF NOT EXISTS test_schema;

-- Create sample tables
CREATE TABLE IF NOT EXISTS test_schema.users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS test_schema.orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES test_schema.users(id),
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    total_amount DECIMAL(10, 2),
    status VARCHAR(20) DEFAULT 'pending'
);

CREATE TABLE IF NOT EXISTS test_schema.order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES test_schema.orders(id),
    product_name VARCHAR(100),
    quantity INTEGER,
    price DECIMAL(10, 2)
);

-- Create indexes
CREATE INDEX idx_users_email ON test_schema.users(email);
CREATE INDEX idx_orders_user_id ON test_schema.orders(user_id);
CREATE INDEX idx_orders_status ON test_schema.orders(status);
CREATE INDEX idx_order_items_order_id ON test_schema.order_items(order_id);

-- Insert sample data
INSERT INTO test_schema.users (username, email) VALUES
    ('john_doe', 'john@example.com'),
    ('jane_smith', 'jane@example.com'),
    ('bob_wilson', 'bob@example.com');

-- Create function for slow query simulation
CREATE OR REPLACE FUNCTION test_schema.simulate_slow_query(delay_seconds INTEGER)
RETURNS void AS $$
BEGIN
    PERFORM pg_sleep(delay_seconds);
END;
$$ LANGUAGE plpgsql;

-- Grant permissions for monitoring
GRANT USAGE ON SCHEMA test_schema TO postgres;
GRANT SELECT ON ALL TABLES IN SCHEMA test_schema TO postgres;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA test_schema TO postgres;

-- Reset pg_stat_statements
SELECT pg_stat_statements_reset();