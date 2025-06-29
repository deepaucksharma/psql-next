-- PostgreSQL Initialization Script for Database Intelligence Collector
-- This script sets up the necessary extensions, users, and sample data

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS pg_buffercache;
CREATE EXTENSION IF NOT EXISTS pg_stat_user_functions;

-- Create monitoring user with appropriate permissions
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_user WHERE usename = 'monitoring_user') THEN
        CREATE USER monitoring_user WITH PASSWORD 'monitoring';
    END IF;
END
$$;

-- Grant necessary permissions to monitoring user
GRANT pg_monitor TO monitoring_user;
GRANT SELECT ON pg_stat_statements TO monitoring_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO monitoring_user;

-- Create sample schema for testing
CREATE SCHEMA IF NOT EXISTS sample_app;

-- Create sample tables
CREATE TABLE IF NOT EXISTS sample_app.users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP,
    is_active BOOLEAN DEFAULT true
);

CREATE TABLE IF NOT EXISTS sample_app.products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    stock_quantity INTEGER NOT NULL DEFAULT 0,
    category VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sample_app.orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES sample_app.users(id),
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'pending',
    total_amount DECIMAL(10, 2),
    shipping_address TEXT
);

CREATE TABLE IF NOT EXISTS sample_app.order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES sample_app.orders(id),
    product_id INTEGER REFERENCES sample_app.products(id),
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10, 2) NOT NULL,
    subtotal DECIMAL(10, 2) GENERATED ALWAYS AS (quantity * unit_price) STORED
);

-- Create indexes for performance testing
CREATE INDEX IF NOT EXISTS idx_users_email ON sample_app.users(email);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON sample_app.users(created_at);
CREATE INDEX IF NOT EXISTS idx_products_category ON sample_app.products(category);
CREATE INDEX IF NOT EXISTS idx_products_price ON sample_app.products(price);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON sample_app.orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_date ON sample_app.orders(order_date);
CREATE INDEX IF NOT EXISTS idx_order_items_order ON sample_app.order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_order_items_product ON sample_app.order_items(product_id);

-- Create a function to generate sample data
CREATE OR REPLACE FUNCTION sample_app.generate_test_data(
    num_users INTEGER DEFAULT 1000,
    num_products INTEGER DEFAULT 500,
    num_orders INTEGER DEFAULT 5000
) RETURNS void AS $$
DECLARE
    i INTEGER;
    user_id INTEGER;
    product_id INTEGER;
    num_items INTEGER;
BEGIN
    -- Generate users
    FOR i IN 1..num_users LOOP
        INSERT INTO sample_app.users (username, email, last_login, is_active)
        VALUES (
            'user_' || i,
            'user' || i || '@example.com',
            CURRENT_TIMESTAMP - (random() * INTERVAL '365 days'),
            random() > 0.1
        );
    END LOOP;
    
    -- Generate products
    FOR i IN 1..num_products LOOP
        INSERT INTO sample_app.products (name, description, price, stock_quantity, category)
        VALUES (
            'Product ' || i,
            'Description for product ' || i,
            (random() * 1000)::DECIMAL(10, 2),
            (random() * 100)::INTEGER,
            CASE (random() * 5)::INTEGER
                WHEN 0 THEN 'Electronics'
                WHEN 1 THEN 'Clothing'
                WHEN 2 THEN 'Books'
                WHEN 3 THEN 'Home'
                ELSE 'Other'
            END
        );
    END LOOP;
    
    -- Generate orders
    FOR i IN 1..num_orders LOOP
        user_id := 1 + (random() * (num_users - 1))::INTEGER;
        
        INSERT INTO sample_app.orders (user_id, order_date, status, total_amount, shipping_address)
        VALUES (
            user_id,
            CURRENT_TIMESTAMP - (random() * INTERVAL '90 days'),
            CASE (random() * 4)::INTEGER
                WHEN 0 THEN 'pending'
                WHEN 1 THEN 'processing'
                WHEN 2 THEN 'shipped'
                ELSE 'delivered'
            END,
            0, -- Will be updated later
            'Address for user ' || user_id
        )
        RETURNING id INTO user_id; -- Reuse variable for order_id
        
        -- Generate order items (1-5 items per order)
        num_items := 1 + (random() * 4)::INTEGER;
        FOR i IN 1..num_items LOOP
            product_id := 1 + (random() * (num_products - 1))::INTEGER;
            
            INSERT INTO sample_app.order_items (order_id, product_id, quantity, unit_price)
            SELECT 
                user_id,
                product_id,
                1 + (random() * 5)::INTEGER,
                price
            FROM sample_app.products WHERE id = product_id;
        END LOOP;
        
        -- Update order total
        UPDATE sample_app.orders 
        SET total_amount = (SELECT SUM(subtotal) FROM sample_app.order_items WHERE order_id = user_id)
        WHERE id = user_id;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Create views for monitoring queries
CREATE OR REPLACE VIEW sample_app.active_users_summary AS
SELECT 
    COUNT(*) as total_users,
    COUNT(*) FILTER (WHERE is_active) as active_users,
    COUNT(*) FILTER (WHERE last_login > CURRENT_TIMESTAMP - INTERVAL '7 days') as recently_active
FROM sample_app.users;

CREATE OR REPLACE VIEW sample_app.order_statistics AS
SELECT 
    DATE_TRUNC('day', order_date) as order_day,
    COUNT(*) as order_count,
    SUM(total_amount) as daily_revenue,
    COUNT(DISTINCT user_id) as unique_customers
FROM sample_app.orders
GROUP BY DATE_TRUNC('day', order_date);

-- Create stored procedures for testing query complexity
CREATE OR REPLACE FUNCTION sample_app.get_user_order_history(
    p_user_id INTEGER,
    p_limit INTEGER DEFAULT 10
) RETURNS TABLE (
    order_id INTEGER,
    order_date TIMESTAMP,
    status VARCHAR,
    total_amount DECIMAL,
    item_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        o.id,
        o.order_date,
        o.status,
        o.total_amount,
        COUNT(oi.id) as item_count
    FROM sample_app.orders o
    LEFT JOIN sample_app.order_items oi ON o.id = oi.order_id
    WHERE o.user_id = p_user_id
    GROUP BY o.id, o.order_date, o.status, o.total_amount
    ORDER BY o.order_date DESC
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions on sample schema
GRANT USAGE ON SCHEMA sample_app TO monitoring_user;
GRANT SELECT ON ALL TABLES IN SCHEMA sample_app TO monitoring_user;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA sample_app TO monitoring_user;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA sample_app TO monitoring_user;

-- Generate initial test data (small dataset for quick startup)
SELECT sample_app.generate_test_data(100, 50, 500);

-- Create a simple heartbeat table for monitoring
CREATE TABLE IF NOT EXISTS public.monitoring_heartbeat (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'healthy'
);

-- Insert initial heartbeat
INSERT INTO public.monitoring_heartbeat (status) VALUES ('initialized');

-- Output confirmation
DO $$
BEGIN
    RAISE NOTICE 'Database initialization complete!';
    RAISE NOTICE 'Monitoring user created: monitoring_user';
    RAISE NOTICE 'Sample data generated in schema: sample_app';
    RAISE NOTICE 'Extensions enabled: pg_stat_statements, pg_buffercache';
END
$$;