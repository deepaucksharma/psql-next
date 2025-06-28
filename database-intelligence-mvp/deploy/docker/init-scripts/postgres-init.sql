-- PostgreSQL initialization script for Database Intelligence MVP
-- Creates monitoring user and enables required extensions

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Create monitoring user with read-only access
CREATE USER newrelic_monitor WITH PASSWORD 'monitor123';

-- Grant necessary permissions
GRANT pg_read_all_settings TO newrelic_monitor;
GRANT pg_read_all_stats TO newrelic_monitor;
GRANT CONNECT ON DATABASE testdb TO newrelic_monitor;

-- Switch to testdb
\c testdb

-- Grant schema permissions
GRANT USAGE ON SCHEMA public TO newrelic_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO newrelic_monitor;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO newrelic_monitor;

-- Create sample tables for testing
CREATE TABLE IF NOT EXISTS customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    customer_id INTEGER REFERENCES customers(id),
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    total_amount DECIMAL(10, 2),
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2),
    stock_quantity INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES orders(id),
    product_id INTEGER REFERENCES products(id),
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10, 2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);

-- Insert sample data
INSERT INTO customers (name, email) VALUES
    ('John Doe', 'john.doe@example.com'),
    ('Jane Smith', 'jane.smith@example.com'),
    ('Bob Johnson', 'bob.johnson@example.com'),
    ('Alice Williams', 'alice.williams@example.com'),
    ('Charlie Brown', 'charlie.brown@example.com');

INSERT INTO products (name, description, price, stock_quantity) VALUES
    ('Laptop Pro', 'High-performance laptop for professionals', 1299.99, 50),
    ('Wireless Mouse', 'Ergonomic wireless mouse', 29.99, 200),
    ('USB-C Hub', 'Multi-port USB-C hub', 49.99, 150),
    ('Mechanical Keyboard', 'RGB mechanical keyboard', 129.99, 75),
    ('4K Monitor', '27-inch 4K IPS monitor', 499.99, 30),
    ('Webcam HD', '1080p HD webcam', 79.99, 100),
    ('Desk Lamp', 'LED desk lamp with USB charging', 39.99, 120),
    ('Laptop Stand', 'Adjustable laptop stand', 34.99, 80);

-- Create functions for generating varied workload
CREATE OR REPLACE FUNCTION generate_random_order() RETURNS void AS $$
DECLARE
    v_customer_id INTEGER;
    v_order_id INTEGER;
    v_num_items INTEGER;
    v_product_id INTEGER;
    v_quantity INTEGER;
    v_price DECIMAL(10, 2);
    v_total DECIMAL(10, 2) := 0;
BEGIN
    -- Select random customer
    SELECT id INTO v_customer_id FROM customers ORDER BY RANDOM() LIMIT 1;
    
    -- Create order
    INSERT INTO orders (customer_id, total_amount) 
    VALUES (v_customer_id, 0) 
    RETURNING id INTO v_order_id;
    
    -- Add random number of items (1-5)
    v_num_items := floor(random() * 5 + 1)::int;
    
    FOR i IN 1..v_num_items LOOP
        -- Select random product
        SELECT id, price INTO v_product_id, v_price 
        FROM products ORDER BY RANDOM() LIMIT 1;
        
        -- Random quantity (1-3)
        v_quantity := floor(random() * 3 + 1)::int;
        
        -- Insert order item
        INSERT INTO order_items (order_id, product_id, quantity, unit_price)
        VALUES (v_order_id, v_product_id, v_quantity, v_price);
        
        v_total := v_total + (v_price * v_quantity);
    END LOOP;
    
    -- Update order total
    UPDATE orders SET total_amount = v_total WHERE id = v_order_id;
END;
$$ LANGUAGE plpgsql;

-- Grant execute permission to monitoring user
GRANT EXECUTE ON FUNCTION generate_random_order() TO newrelic_monitor;

-- Create a view for monitoring user
CREATE OR REPLACE VIEW database_statistics AS
SELECT 
    current_database() as database_name,
    schemaname,
    tablename,
    n_live_tup,
    n_dead_tup,
    last_vacuum,
    last_autovacuum
FROM pg_stat_all_tables
WHERE schemaname = 'public';

GRANT SELECT ON database_statistics TO newrelic_monitor;

-- Verify pg_stat_statements is working
SELECT pg_stat_statements_reset();

-- Log successful initialization
DO $$
BEGIN
    RAISE NOTICE 'PostgreSQL initialization completed successfully';
    RAISE NOTICE 'Monitoring user: newrelic_monitor';
    RAISE NOTICE 'pg_stat_statements extension: enabled';
END $$;