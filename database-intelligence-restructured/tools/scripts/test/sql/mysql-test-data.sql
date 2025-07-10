-- MySQL test data for Database Intelligence testing
-- This script populates the database with realistic test data for comprehensive testing

USE testdb;

-- Insert test users with realistic data (including PII for detection testing)
INSERT INTO users (email, username, password_hash, phone, ssn, created_at) VALUES
('john.doe@example.com', 'johndoe', '$2b$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj/UGF/KfPQu', '555-123-4567', '123-45-6789', '2024-01-15 10:30:00'),
('jane.smith@company.com', 'janesmith', '$2b$12$XnJhJ7k3qWbYxDgF4GpLKe8fH9mN3pR7sV1wZ5yA2bC4dE6fG8hI', '555-987-6543', '987-65-4321', '2024-01-20 14:15:00'),
('bob.wilson@email.org', 'bobwilson', '$2b$12$YpKlM8n4rXcZxFhG5JqOLf9gI0oP4qR8tW2yB6zA3cD5eF7gH9iJ', '555-456-7890', '456-78-9012', '2024-02-01 09:45:00'),
('alice.johnson@test.com', 'alicej', '$2b$12$ZqLmN9o5sYdAyGiH6KrPMg0hJ1pQ5rS9uX3zA7yB4cE6fG8hI0jK', '555-789-0123', '789-01-2345', '2024-02-10 16:20:00'),
('charlie.brown@demo.net', 'charlieb', '$2b$12$ArMnO0p6tZeBzHjI7LsQNh1iK2qR6sT0vY4zB8yC5dF7gH9iJ1kL', '555-234-5678', '234-56-7890', '2024-02-15 11:00:00');

-- Insert products
INSERT INTO products (name, description, price, category, sku, stock_quantity) VALUES
('Gaming Laptop', 'High-performance gaming laptop with RTX graphics', 1599.99, 'Electronics', 'LAPTOP-GAMING-01', 25),
('Wireless Keyboard', 'Mechanical wireless keyboard with RGB lighting', 149.99, 'Electronics', 'KEYBOARD-MECH-01', 100),
('Gaming Mouse', 'Precision gaming mouse with customizable buttons', 79.99, 'Electronics', 'MOUSE-GAMING-01', 150),
('Monitor 27"', '4K gaming monitor with 144Hz refresh rate', 399.99, 'Electronics', 'MONITOR-27-4K', 40),
('USB Hub', '7-port USB 3.0 hub with fast charging', 39.99, 'Electronics', 'USB-HUB-7PORT', 200);

-- Insert orders with various patterns
INSERT INTO orders (user_id, order_number, total_amount, status, shipping_address, credit_card, created_at) VALUES
(1, 'MYS-2024-001', 1599.99, 'delivered', '123 Main St, Anytown, ST 12345', '4532-1234-5678-9012', '2024-01-16 10:30:00'),
(2, 'MYS-2024-002', 229.98, 'shipped', '456 Oak Ave, Other City, ST 67890', '5555-4444-3333-2222', '2024-02-01 14:20:00'),
(3, 'MYS-2024-003', 479.98, 'completed', '789 Pine Rd, Another Town, ST 11111', '4000-1111-2222-3333', '2024-02-15 09:15:00'),
(4, 'MYS-2024-004', 39.99, 'delivered', '321 Elm St, Some City, ST 22222', '3782-8224-6310-005', '2024-02-20 16:45:00'),
(5, 'MYS-2024-005', 1829.97, 'pending', '654 Maple Dr, New Town, ST 33333', '6011-1111-1111-1117', '2024-03-01 11:30:00');

-- Insert order items
INSERT INTO order_items (order_id, product_id, quantity, unit_price, total_price) VALUES
-- Order 1: Gaming Laptop
(1, 1, 1, 1599.99, 1599.99),
-- Order 2: Keyboard + Mouse
(2, 2, 1, 149.99, 149.99),
(2, 3, 1, 79.99, 79.99),
-- Order 3: Monitor + Gaming Mouse
(3, 4, 1, 399.99, 399.99),
(3, 3, 1, 79.99, 79.99),
-- Order 4: USB Hub
(4, 5, 1, 39.99, 39.99),
-- Order 5: Gaming Laptop + Monitor + Keyboard + Mouse + USB Hub
(5, 1, 1, 1599.99, 1599.99),
(5, 4, 1, 399.99, 399.99),
(5, 2, 1, 149.99, 149.99),
(5, 3, 1, 79.99, 79.99),
(5, 5, 1, 39.99, 39.99);

-- Insert analytics data
INSERT INTO daily_sales (date, total_orders, total_revenue, average_order_value, unique_customers) VALUES
('2024-01-16', 1, 1599.99, 1599.99, 1),
('2024-02-01', 1, 229.98, 229.98, 1),
('2024-02-15', 1, 479.98, 479.98, 1),
('2024-02-20', 1, 39.99, 39.99, 1),
('2024-03-01', 1, 1829.97, 1829.97, 1);

-- Insert product performance data
INSERT INTO product_performance (product_id, date, views, orders, revenue) VALUES
(1, '2024-01-16', 120, 1, 1599.99),
(2, '2024-02-01', 45, 1, 149.99),
(3, '2024-02-01', 38, 1, 79.99),
(4, '2024-02-15', 62, 1, 399.99),
(3, '2024-02-15', 25, 1, 79.99),
(5, '2024-02-20', 15, 1, 39.99),
(1, '2024-03-01', 95, 1, 1599.99),
(4, '2024-03-01', 55, 1, 399.99),
(2, '2024-03-01', 40, 1, 149.99),
(3, '2024-03-01', 32, 1, 79.99),
(5, '2024-03-01', 18, 1, 39.99);

-- Insert some query logs
INSERT INTO query_logs (query_text, query_hash, execution_time_ms, rows_affected, user_id, database_name, executed_at) VALUES
('SELECT * FROM users WHERE email = ?', MD5('SELECT * FROM users WHERE email = ?'), 12, 1, 1, 'testdb', '2024-03-01 10:30:00'),
('INSERT INTO orders (...) VALUES (...)', MD5('INSERT INTO orders (...) VALUES (...)'), 18, 1, 2, 'testdb', '2024-03-01 10:35:00'),
('UPDATE products SET stock_quantity = ? WHERE id = ?', MD5('UPDATE products SET stock_quantity = ? WHERE id = ?'), 8, 1, 1, 'testdb', '2024-03-01 10:40:00'),
('SELECT COUNT(*) FROM orders WHERE created_at >= ?', MD5('SELECT COUNT(*) FROM orders WHERE created_at >= ?'), 5, 0, 3, 'testdb', '2024-03-01 10:45:00'),
('SELECT * FROM customer_lifetime_value ORDER BY lifetime_value DESC LIMIT 10', MD5('SELECT * FROM customer_lifetime_value ORDER BY lifetime_value DESC LIMIT 10'), 35, 10, 2, 'testdb', '2024-03-01 10:50:00');

-- Execute various queries to generate performance schema data
SELECT COUNT(*) FROM users;
SELECT COUNT(*) FROM orders;
SELECT COUNT(*) FROM products;

-- Execute some complex queries to generate interesting execution plans
SELECT 
    u.email,
    COUNT(o.id) as order_count,
    SUM(o.total_amount) as total_spent
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.email
ORDER BY total_spent DESC;

SELECT 
    p.name,
    SUM(oi.quantity) as total_sold,
    SUM(oi.total_price) as total_revenue
FROM products p
JOIN order_items oi ON p.id = oi.product_id
JOIN orders o ON oi.order_id = o.id
WHERE o.status IN ('completed', 'delivered', 'shipped')
GROUP BY p.id, p.name
ORDER BY total_revenue DESC;

-- Execute queries with different complexity levels for testing
-- Simple query
SELECT email FROM users WHERE id = 1;

-- Medium complexity query
SELECT o.order_number, o.total_amount, u.email
FROM orders o
JOIN users u ON o.user_id = u.id
WHERE o.created_at >= '2024-02-01'
ORDER BY o.created_at DESC;

-- Complex query with subqueries
SELECT 
    u.email,
    u.username,
    (SELECT COUNT(*) FROM orders WHERE user_id = u.id) as order_count,
    (SELECT COALESCE(SUM(total_amount), 0) FROM orders WHERE user_id = u.id) as lifetime_value,
    (SELECT MAX(created_at) FROM orders WHERE user_id = u.id) as last_order_date
FROM users u
WHERE u.created_at >= '2024-01-01'
ORDER BY lifetime_value DESC;

COMMIT;