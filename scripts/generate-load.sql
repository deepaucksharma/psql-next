-- Load generation script for PostgreSQL monitoring test

-- Random user activity
INSERT INTO users (username, email) 
SELECT 
    'user_' || generate_series,
    'user_' || generate_series || '@example.com'
FROM generate_series(1, 10)
ON CONFLICT (email) DO NOTHING;

-- Random product queries
SELECT * FROM products WHERE price > random() * 1000 LIMIT 10;
SELECT * FROM products WHERE inventory_count < random() * 100 ORDER BY price DESC;

-- Random order creation
INSERT INTO orders (user_id, total, status)
SELECT 
    (random() * 3 + 1)::int,
    random() * 1000,
    CASE WHEN random() > 0.5 THEN 'completed' ELSE 'pending' END
FROM generate_series(1, 5);

-- Complex queries for monitoring
WITH user_orders AS (
    SELECT u.username, COUNT(o.id) as order_count, SUM(o.total) as total_spent
    FROM users u
    LEFT JOIN orders o ON u.id = o.user_id
    GROUP BY u.username
)
SELECT * FROM user_orders WHERE total_spent > 100;

-- Update operations
UPDATE products 
SET inventory_count = inventory_count - 1 
WHERE id = (random() * 3 + 1)::int AND inventory_count > 0;

-- Analytics queries
SELECT 
    DATE_TRUNC('hour', created_at) as hour,
    COUNT(*) as order_count,
    AVG(total) as avg_order_value
FROM orders
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour DESC;

-- Force some slow queries
SELECT pg_sleep(random() * 0.5);
