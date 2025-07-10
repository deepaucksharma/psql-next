-- PostgreSQL test data for Database Intelligence testing
-- This script populates the database with realistic test data for comprehensive testing

BEGIN;

-- Insert test users with realistic data (including PII for detection testing)
INSERT INTO ecommerce.users (email, username, password_hash, phone, ssn, created_at) VALUES
('john.doe@example.com', 'johndoe', '$2b$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj/UGF/KfPQu', '555-123-4567', '123-45-6789', '2024-01-15 10:30:00'),
('jane.smith@company.com', 'janesmith', '$2b$12$XnJhJ7k3qWbYxDgF4GpLKe8fH9mN3pR7sV1wZ5yA2bC4dE6fG8hI', '555-987-6543', '987-65-4321', '2024-01-20 14:15:00'),
('bob.wilson@email.org', 'bobwilson', '$2b$12$YpKlM8n4rXcZxFhG5JqOLf9gI0oP4qR8tW2yB6zA3cD5eF7gH9iJ', '555-456-7890', '456-78-9012', '2024-02-01 09:45:00'),
('alice.johnson@test.com', 'alicej', '$2b$12$ZqLmN9o5sYdAyGiH6KrPMg0hJ1pQ5rS9uX3zA7yB4cE6fG8hI0jK', '555-789-0123', '789-01-2345', '2024-02-10 16:20:00'),
('charlie.brown@demo.net', 'charlieb', '$2b$12$ArMnO0p6tZeBzHjI7LsQNh1iK2qR6sT0vY4zB8yC5dF7gH9iJ1kL', '555-234-5678', '234-56-7890', '2024-02-15 11:00:00'),
('diana.prince@wonder.com', 'dianaprince', '$2b$12$BsMoP1q7uAfCzIkJ8MtROi2jL3rS7tU1wZ5zA8yD6eG8hI0jK2lM', '555-345-6789', '345-67-8901', '2024-03-01 13:30:00'),
('test.user@privacy.org', 'testuser', '$2b$12$CtNoQ2r8vBgDzJlK9NuSPj3kM4sT8uV2xA6zB9yE7fH9iJ1kL3mN', '555-567-8901', '567-89-0123', '2024-03-05 08:15:00'),
('secure.account@bank.com', 'secureuser', '$2b$12$DuOpR3s9wChEzKmL0OvTQk4lN5tU9vW3yB7zC0yF8gI0jK2lM4nO', '555-678-9012', '678-90-1234', '2024-03-10 15:45:00');

-- Insert product categories and products
INSERT INTO ecommerce.products (name, description, price, category_id, sku, stock_quantity) VALUES
('Laptop Pro 15"', 'High-performance laptop with 15-inch display', 1299.99, 1, 'LAPTOP-PRO-15', 50),
('Wireless Mouse', 'Ergonomic wireless mouse with precision tracking', 29.99, 1, 'MOUSE-WIRELESS-01', 200),
('USB-C Cable', 'Durable USB-C charging cable, 6ft length', 19.99, 1, 'USB-C-CABLE-6FT', 500),
('Smartphone X', 'Latest smartphone with advanced camera system', 899.99, 2, 'PHONE-X-128GB', 100),
('Tablet Air', 'Lightweight tablet perfect for productivity', 549.99, 2, 'TABLET-AIR-64GB', 75),
('Bluetooth Headphones', 'Noise-cancelling over-ear headphones', 199.99, 3, 'HEADPHONES-BT-NC', 150),
('Smart Watch', 'Fitness tracking smartwatch with GPS', 299.99, 3, 'WATCH-SMART-GPS', 80),
('Desk Chair', 'Ergonomic office chair with lumbar support', 249.99, 4, 'CHAIR-OFFICE-ERG', 30),
('Standing Desk', 'Height-adjustable standing desk', 399.99, 4, 'DESK-STANDING-ADJ', 25),
('Coffee Maker', 'Programmable coffee maker with thermal carafe', 89.99, 5, 'COFFEE-MAKER-PROG', 60);

-- Insert realistic orders with various patterns
INSERT INTO ecommerce.orders (user_id, order_number, total_amount, order_status, shipping_address, credit_card, created_at) VALUES
(1, 'ORD-2024-001', 1329.98, 'delivered', '123 Main St, Anytown, ST 12345', '4532-1234-5678-9012', '2024-01-16 10:30:00'),
(1, 'ORD-2024-002', 29.99, 'delivered', '123 Main St, Anytown, ST 12345', '4532-1234-5678-9012', '2024-02-05 14:20:00'),
(2, 'ORD-2024-003', 1449.98, 'shipped', '456 Oak Ave, Other City, ST 67890', '5555-4444-3333-2222', '2024-02-15 09:15:00'),
(3, 'ORD-2024-004', 199.99, 'completed', '789 Pine Rd, Another Town, ST 11111', '4000-1111-2222-3333', '2024-02-20 16:45:00'),
(4, 'ORD-2024-005', 649.98, 'delivered', '321 Elm St, Some City, ST 22222', '3782-8224-6310-005', '2024-03-01 11:30:00'),
(5, 'ORD-2024-006', 89.99, 'pending', '654 Maple Dr, New Town, ST 33333', '6011-1111-1111-1117', '2024-03-10 13:20:00'),
(6, 'ORD-2024-007', 299.99, 'shipped', '987 Cedar Ln, Final City, ST 44444', '4532-1234-5678-9012', '2024-03-15 08:45:00'),
(2, 'ORD-2024-008', 69.98, 'delivered', '456 Oak Ave, Other City, ST 67890', '5555-4444-3333-2222', '2024-03-18 15:10:00'),
(7, 'ORD-2024-009', 549.99, 'completed', '111 First St, Privacy Town, ST 55555', '4000-0000-0000-0002', '2024-03-20 12:00:00'),
(8, 'ORD-2024-010', 1599.97, 'processing', '222 Second Ave, Secure City, ST 66666', '4111-1111-1111-1111', '2024-03-22 10:30:00');

-- Insert order items
INSERT INTO ecommerce.order_items (order_id, product_id, quantity, unit_price, total_price) VALUES
-- Order 1: Laptop + Mouse
(1, 1, 1, 1299.99, 1299.99),
(1, 2, 1, 29.99, 29.99),
-- Order 2: Just mouse
(2, 2, 1, 29.99, 29.99),
-- Order 3: Laptop + Smartphone
(3, 1, 1, 1299.99, 1299.99),
(3, 4, 1, 149.99, 149.99), -- Discounted phone
-- Order 4: Headphones
(4, 6, 1, 199.99, 199.99),
-- Order 5: Tablet + Smart Watch
(5, 5, 1, 549.99, 549.99),
(5, 7, 1, 99.99, 99.99), -- Discounted watch
-- Order 6: Coffee Maker
(6, 10, 1, 89.99, 89.99),
-- Order 7: Smart Watch
(7, 7, 1, 299.99, 299.99),
-- Order 8: USB Cable + Mouse
(8, 3, 2, 19.99, 39.98),
(8, 2, 1, 29.99, 29.99),
-- Order 9: Tablet
(9, 5, 1, 549.99, 549.99),
-- Order 10: Laptop + Desk + Chair
(10, 1, 1, 1299.99, 1299.99),
(10, 8, 1, 249.99, 249.99),
(10, 9, 1, 49.99, 49.99); -- Discounted desk

-- Insert analytics data
INSERT INTO analytics.daily_sales (date, total_orders, total_revenue, average_order_value, unique_customers) VALUES
('2024-01-16', 1, 1329.98, 1329.98, 1),
('2024-02-05', 1, 29.99, 29.99, 1),
('2024-02-15', 1, 1449.98, 1449.98, 1),
('2024-02-20', 1, 199.99, 199.99, 1),
('2024-03-01', 1, 649.98, 649.98, 1),
('2024-03-10', 1, 89.99, 89.99, 1),
('2024-03-15', 1, 299.99, 299.99, 1),
('2024-03-18', 1, 69.98, 69.98, 1),
('2024-03-20', 1, 549.99, 549.99, 1),
('2024-03-22', 1, 1599.97, 1599.97, 1);

-- Insert product performance data
INSERT INTO analytics.product_performance (product_id, date, views, orders, revenue) VALUES
(1, '2024-01-16', 45, 1, 1299.99),
(2, '2024-01-16', 12, 1, 29.99),
(1, '2024-02-15', 38, 1, 1299.99),
(4, '2024-02-15', 25, 1, 149.99),
(6, '2024-02-20', 18, 1, 199.99),
(5, '2024-03-01', 32, 1, 549.99),
(7, '2024-03-01', 15, 1, 99.99),
(10, '2024-03-10', 8, 1, 89.99),
(7, '2024-03-15', 22, 1, 299.99),
(3, '2024-03-18', 5, 2, 39.98),
(2, '2024-03-18', 10, 1, 29.99),
(5, '2024-03-20', 28, 1, 549.99),
(1, '2024-03-22', 55, 1, 1299.99),
(8, '2024-03-22', 12, 1, 249.99),
(9, '2024-03-22', 8, 1, 49.99);

-- Insert some audit logs for testing
INSERT INTO audit.query_logs (query_text, query_hash, execution_time_ms, rows_affected, user_id, database_name, executed_at) VALUES
('SELECT * FROM ecommerce.users WHERE email = ?', MD5('SELECT * FROM ecommerce.users WHERE email = ?'), 15, 1, 1, 'testdb', '2024-03-01 10:30:00'),
('INSERT INTO ecommerce.orders (...) VALUES (...)', MD5('INSERT INTO ecommerce.orders (...) VALUES (...)'), 25, 1, 2, 'testdb', '2024-03-01 10:35:00'),
('UPDATE ecommerce.products SET stock_quantity = ? WHERE id = ?', MD5('UPDATE ecommerce.products SET stock_quantity = ? WHERE id = ?'), 12, 1, 1, 'testdb', '2024-03-01 10:40:00'),
('SELECT COUNT(*) FROM ecommerce.orders WHERE created_at >= ?', MD5('SELECT COUNT(*) FROM ecommerce.orders WHERE created_at >= ?'), 8, 0, 3, 'testdb', '2024-03-01 10:45:00'),
('SELECT * FROM ecommerce.customer_lifetime_value ORDER BY lifetime_value DESC LIMIT 10', MD5('SELECT * FROM ecommerce.customer_lifetime_value ORDER BY lifetime_value DESC LIMIT 10'), 45, 10, 2, 'testdb', '2024-03-01 10:50:00');

-- Generate some query statistics
SELECT pg_stat_statements_reset();

-- Execute various queries to generate statistics
SELECT COUNT(*) FROM ecommerce.users;
SELECT COUNT(*) FROM ecommerce.orders;
SELECT COUNT(*) FROM ecommerce.products;

-- Execute some complex queries to generate interesting execution plans
SELECT 
    u.email,
    COUNT(o.id) as order_count,
    SUM(o.total_amount) as total_spent
FROM ecommerce.users u
LEFT JOIN ecommerce.orders o ON u.id = o.user_id
GROUP BY u.id, u.email
ORDER BY total_spent DESC;

SELECT 
    p.name,
    SUM(oi.quantity) as total_sold,
    SUM(oi.total_price) as total_revenue
FROM ecommerce.products p
JOIN ecommerce.order_items oi ON p.id = oi.product_id
JOIN ecommerce.orders o ON oi.order_id = o.id
WHERE o.order_status IN ('completed', 'delivered', 'shipped')
GROUP BY p.id, p.name
ORDER BY total_revenue DESC;

-- Execute queries with different complexity levels for testing
-- Simple query
SELECT email FROM ecommerce.users WHERE id = 1;

-- Medium complexity query
SELECT o.order_number, o.total_amount, u.email
FROM ecommerce.orders o
JOIN ecommerce.users u ON o.user_id = u.id
WHERE o.created_at >= '2024-03-01'
ORDER BY o.created_at DESC;

-- Complex query with subqueries
SELECT 
    u.email,
    u.username,
    (SELECT COUNT(*) FROM ecommerce.orders WHERE user_id = u.id) as order_count,
    (SELECT COALESCE(SUM(total_amount), 0) FROM ecommerce.orders WHERE user_id = u.id) as lifetime_value,
    (SELECT MAX(created_at) FROM ecommerce.orders WHERE user_id = u.id) as last_order_date
FROM ecommerce.users u
WHERE u.created_at >= '2024-01-01'
ORDER BY lifetime_value DESC;

-- Update some statistics
ANALYZE;

COMMIT;