-- PostgreSQL initialization script for Database Intelligence testing
-- This script sets up the database for comprehensive testing

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create test schemas
CREATE SCHEMA IF NOT EXISTS ecommerce;
CREATE SCHEMA IF NOT EXISTS analytics;
CREATE SCHEMA IF NOT EXISTS audit;

-- Create test tables with various data types and patterns
CREATE TABLE IF NOT EXISTS ecommerce.users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    phone VARCHAR(20),
    ssn VARCHAR(11), -- For PII detection testing
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT true
);

CREATE TABLE IF NOT EXISTS ecommerce.products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    category_id INTEGER,
    sku VARCHAR(100) UNIQUE,
    stock_quantity INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ecommerce.orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES ecommerce.users(id),
    order_number VARCHAR(50) UNIQUE NOT NULL,
    total_amount DECIMAL(12,2) NOT NULL,
    order_status VARCHAR(20) DEFAULT 'pending',
    shipping_address TEXT,
    credit_card VARCHAR(19), -- For PII detection testing
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    shipped_at TIMESTAMP,
    delivered_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS ecommerce.order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES ecommerce.orders(id),
    product_id INTEGER REFERENCES ecommerce.products(id),
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    total_price DECIMAL(12,2) NOT NULL
);

-- Analytics tables for complex queries
CREATE TABLE IF NOT EXISTS analytics.daily_sales (
    date DATE PRIMARY KEY,
    total_orders INTEGER,
    total_revenue DECIMAL(15,2),
    average_order_value DECIMAL(10,2),
    unique_customers INTEGER
);

CREATE TABLE IF NOT EXISTS analytics.product_performance (
    product_id INTEGER,
    date DATE,
    views INTEGER DEFAULT 0,
    orders INTEGER DEFAULT 0,
    revenue DECIMAL(12,2) DEFAULT 0,
    PRIMARY KEY (product_id, date)
);

-- Audit table for tracking changes
CREATE TABLE IF NOT EXISTS audit.query_logs (
    id SERIAL PRIMARY KEY,
    query_text TEXT,
    query_hash VARCHAR(64),
    execution_time_ms INTEGER,
    rows_affected INTEGER,
    user_id INTEGER,
    database_name VARCHAR(100),
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance testing
CREATE INDEX IF NOT EXISTS idx_users_email ON ecommerce.users(email);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON ecommerce.users(created_at);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON ecommerce.orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON ecommerce.orders(created_at);
CREATE INDEX IF NOT EXISTS idx_orders_status ON ecommerce.orders(order_status);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON ecommerce.order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_order_items_product_id ON ecommerce.order_items(product_id);
CREATE INDEX IF NOT EXISTS idx_analytics_date ON analytics.daily_sales(date);
CREATE INDEX IF NOT EXISTS idx_product_perf_date ON analytics.product_performance(date);
CREATE INDEX IF NOT EXISTS idx_query_logs_executed_at ON audit.query_logs(executed_at);

-- Create views for complex query testing
CREATE OR REPLACE VIEW ecommerce.customer_lifetime_value AS
SELECT 
    u.id,
    u.email,
    u.username,
    COUNT(o.id) as total_orders,
    COALESCE(SUM(o.total_amount), 0) as lifetime_value,
    AVG(o.total_amount) as average_order_value,
    MIN(o.created_at) as first_order_date,
    MAX(o.created_at) as last_order_date
FROM ecommerce.users u
LEFT JOIN ecommerce.orders o ON u.id = o.user_id
GROUP BY u.id, u.email, u.username;

CREATE OR REPLACE VIEW analytics.monthly_revenue AS
SELECT 
    DATE_TRUNC('month', created_at) as month,
    COUNT(*) as total_orders,
    SUM(total_amount) as total_revenue,
    AVG(total_amount) as avg_order_value,
    COUNT(DISTINCT user_id) as unique_customers
FROM ecommerce.orders
WHERE order_status IN ('completed', 'shipped', 'delivered')
GROUP BY DATE_TRUNC('month', created_at)
ORDER BY month;

-- Create stored procedures for testing
CREATE OR REPLACE FUNCTION ecommerce.get_user_orders(user_email VARCHAR)
RETURNS TABLE(
    order_id INTEGER,
    order_number VARCHAR,
    total_amount DECIMAL,
    order_status VARCHAR,
    created_at TIMESTAMP
) AS $$
BEGIN
    RETURN QUERY
    SELECT o.id, o.order_number, o.total_amount, o.order_status, o.created_at
    FROM ecommerce.orders o
    JOIN ecommerce.users u ON o.user_id = u.id
    WHERE u.email = user_email
    ORDER BY o.created_at DESC;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION analytics.calculate_daily_stats(target_date DATE)
RETURNS VOID AS $$
BEGIN
    INSERT INTO analytics.daily_sales (date, total_orders, total_revenue, average_order_value, unique_customers)
    SELECT 
        target_date,
        COUNT(*),
        COALESCE(SUM(total_amount), 0),
        COALESCE(AVG(total_amount), 0),
        COUNT(DISTINCT user_id)
    FROM ecommerce.orders
    WHERE DATE(created_at) = target_date
    ON CONFLICT (date) DO UPDATE SET
        total_orders = EXCLUDED.total_orders,
        total_revenue = EXCLUDED.total_revenue,
        average_order_value = EXCLUDED.average_order_value,
        unique_customers = EXCLUDED.unique_customers;
END;
$$ LANGUAGE plpgsql;

-- Create triggers for audit logging
CREATE OR REPLACE FUNCTION audit.log_query()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit.query_logs (query_text, query_hash, user_id, database_name)
    VALUES (
        current_query(),
        MD5(current_query()),
        COALESCE(NEW.user_id, OLD.user_id),
        current_database()
    );
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

-- Apply triggers to key tables
DROP TRIGGER IF EXISTS user_audit_trigger ON ecommerce.users;
CREATE TRIGGER user_audit_trigger
    AFTER INSERT OR UPDATE OR DELETE ON ecommerce.users
    FOR EACH ROW EXECUTE FUNCTION audit.log_query();

DROP TRIGGER IF EXISTS order_audit_trigger ON ecommerce.orders;
CREATE TRIGGER order_audit_trigger
    AFTER INSERT OR UPDATE OR DELETE ON ecommerce.orders
    FOR EACH ROW EXECUTE FUNCTION audit.log_query();

-- Configure pg_stat_statements for query analysis
SELECT pg_stat_statements_reset();

-- Grant necessary permissions
GRANT USAGE ON SCHEMA ecommerce TO postgres;
GRANT USAGE ON SCHEMA analytics TO postgres;
GRANT USAGE ON SCHEMA audit TO postgres;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA ecommerce TO postgres;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA analytics TO postgres;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA audit TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA ecommerce TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA analytics TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA audit TO postgres;

-- Set up auto_explain for query plan analysis
LOAD 'auto_explain';
SET auto_explain.log_min_duration = 100;
SET auto_explain.log_analyze = true;
SET auto_explain.log_verbose = true;
SET auto_explain.log_timing = true;
SET auto_explain.log_nested_statements = true;

COMMIT;