-- MySQL initialization script for Database Intelligence testing
-- This script sets up the database for comprehensive testing

-- Create test database if not exists
CREATE DATABASE IF NOT EXISTS testdb;
USE testdb;

-- Create test tables with various data types and patterns
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    phone VARCHAR(20),
    ssn VARCHAR(11), -- For PII detection testing
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT true
);

CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10,2) NOT NULL,
    category VARCHAR(100),
    sku VARCHAR(100) UNIQUE,
    stock_quantity INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    order_number VARCHAR(50) UNIQUE NOT NULL,
    total_amount DECIMAL(12,2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    shipping_address TEXT,
    credit_card VARCHAR(19), -- For PII detection testing
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    shipped_at TIMESTAMP NULL,
    delivered_at TIMESTAMP NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS order_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT,
    product_id INT,
    quantity INT NOT NULL,
    unit_price DECIMAL(10,2) NOT NULL,
    total_price DECIMAL(12,2) NOT NULL,
    FOREIGN KEY (order_id) REFERENCES orders(id),
    FOREIGN KEY (product_id) REFERENCES products(id)
);

-- Analytics tables for complex queries
CREATE TABLE IF NOT EXISTS daily_sales (
    date DATE PRIMARY KEY,
    total_orders INT,
    total_revenue DECIMAL(15,2),
    average_order_value DECIMAL(10,2),
    unique_customers INT
);

CREATE TABLE IF NOT EXISTS product_performance (
    product_id INT,
    date DATE,
    views INT DEFAULT 0,
    orders INT DEFAULT 0,
    revenue DECIMAL(12,2) DEFAULT 0,
    PRIMARY KEY (product_id, date)
);

-- Audit table for tracking changes
CREATE TABLE IF NOT EXISTS query_logs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    query_text TEXT,
    query_hash VARCHAR(64),
    execution_time_ms INT,
    rows_affected INT,
    user_id INT,
    database_name VARCHAR(100),
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for performance testing
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_created_at ON users(created_at);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_created_at ON orders(created_at);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);
CREATE INDEX idx_daily_sales_date ON daily_sales(date);
CREATE INDEX idx_product_perf_date ON product_performance(date);
CREATE INDEX idx_query_logs_executed_at ON query_logs(executed_at);

-- Create views for complex query testing
CREATE OR REPLACE VIEW customer_lifetime_value AS
SELECT 
    u.id,
    u.email,
    u.username,
    COUNT(o.id) as total_orders,
    COALESCE(SUM(o.total_amount), 0) as lifetime_value,
    AVG(o.total_amount) as average_order_value,
    MIN(o.created_at) as first_order_date,
    MAX(o.created_at) as last_order_date
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.email, u.username;

CREATE OR REPLACE VIEW monthly_revenue AS
SELECT 
    DATE_FORMAT(created_at, '%Y-%m-01') as month,
    COUNT(*) as total_orders,
    SUM(total_amount) as total_revenue,
    AVG(total_amount) as avg_order_value,
    COUNT(DISTINCT user_id) as unique_customers
FROM orders
WHERE status IN ('completed', 'shipped', 'delivered')
GROUP BY DATE_FORMAT(created_at, '%Y-%m-01')
ORDER BY month;

-- Create stored procedures for testing
DELIMITER //

CREATE PROCEDURE GetUserOrders(IN user_email VARCHAR(255))
BEGIN
    SELECT o.id, o.order_number, o.total_amount, o.status, o.created_at
    FROM orders o
    JOIN users u ON o.user_id = u.id
    WHERE u.email = user_email
    ORDER BY o.created_at DESC;
END //

CREATE PROCEDURE CalculateDailyStats(IN target_date DATE)
BEGIN
    INSERT INTO daily_sales (date, total_orders, total_revenue, average_order_value, unique_customers)
    SELECT 
        target_date,
        COUNT(*),
        COALESCE(SUM(total_amount), 0),
        COALESCE(AVG(total_amount), 0),
        COUNT(DISTINCT user_id)
    FROM orders
    WHERE DATE(created_at) = target_date
    ON DUPLICATE KEY UPDATE
        total_orders = VALUES(total_orders),
        total_revenue = VALUES(total_revenue),
        average_order_value = VALUES(average_order_value),
        unique_customers = VALUES(unique_customers);
END //

DELIMITER ;

-- Enable general query log for testing
SET GLOBAL general_log = 'ON';
SET GLOBAL log_output = 'TABLE';

COMMIT;