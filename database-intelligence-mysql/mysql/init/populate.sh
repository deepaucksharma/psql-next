#!/bin/bash
# Populate MySQL with realistic data quickly
# Version: 1.0.0

echo "Populating MySQL with data..."

# Generate lots of data quickly
docker exec mysql-primary mysql -u root -prootpassword ecommerce -e "
-- Generate more products using existing schema
INSERT INTO products (sku, name, description, price, stock_quantity)
SELECT 
    CONCAT('SKU-', LPAD(n, 6, '0')),
    CONCAT('Product ', n, ' - ', ELT(1 + MOD(n, 10), 
        'Laptop', 'Mouse', 'Keyboard', 'Monitor', 'Headphones',
        'Desk', 'Chair', 'Lamp', 'Cable', 'Adapter')),
    CONCAT('High quality ', ELT(1 + MOD(n, 5), 
        'electronic device', 'office furniture', 'computer accessory', 
        'premium item', 'professional equipment')),
    ROUND(10 + RAND() * 990, 2),
    FLOOR(10 + RAND() * 200)
FROM (
    SELECT a.N + b.N * 10 + c.N * 100 AS n
    FROM 
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) a,
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) b,
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4) c
    WHERE a.N + b.N * 10 + c.N * 100 > 5
) numbers
ON DUPLICATE KEY UPDATE stock_quantity = stock_quantity;

-- Generate more customers
INSERT INTO customers (username, email, full_name, address, phone, registration_date)
SELECT 
    CONCAT('user', n),
    CONCAT('customer', n, '@example.com'),
    CONCAT(
        ELT(1 + MOD(n, 10), 'John', 'Jane', 'Bob', 'Alice', 'Charlie', 
            'David', 'Emma', 'Frank', 'Grace', 'Henry'),
        ' ',
        ELT(1 + MOD(n, 10), 'Smith', 'Johnson', 'Williams', 'Brown', 'Jones',
            'Garcia', 'Miller', 'Davis', 'Wilson', 'Moore')
    ),
    CONCAT(n, ' Main Street, City ', MOD(n, 50)),
    CONCAT('555-', LPAD(1000 + n, 4, '0')),
    DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 365) DAY)
FROM (
    SELECT a.N + b.N * 10 + c.N * 100 AS n
    FROM 
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) a,
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) b,
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4) c
    WHERE a.N + b.N * 10 + c.N * 100 > 0
) numbers
ON DUPLICATE KEY UPDATE phone = phone;

-- Generate orders
INSERT INTO orders (customer_id, total_amount, status, created_at)
SELECT 
    1 + MOD(n, (SELECT COUNT(*) FROM customers)),
    ROUND(20 + RAND() * 480, 2),
    ELT(1 + MOD(n, 5), 'pending', 'processing', 'shipped', 'delivered', 'cancelled'),
    DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 90) DAY)
FROM (
    SELECT a.N + b.N * 10 + c.N * 100 AS n
    FROM 
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) a,
        (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
         UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) b,
        (SELECT 0 AS N UNION SELECT 1) c
) numbers;

-- Generate order items
INSERT INTO order_items (order_id, product_id, quantity, price)
SELECT 
    o.order_id,
    p.product_id,
    1 + FLOOR(RAND() * 3),
    p.price
FROM orders o
CROSS JOIN (SELECT product_id, price FROM products ORDER BY RAND() LIMIT 3) p
WHERE o.order_id <= 100
ON DUPLICATE KEY UPDATE quantity = quantity;

-- Create some indexes to generate different query patterns
CREATE INDEX IF NOT EXISTS idx_order_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_order_date ON orders(created_at);
CREATE INDEX IF NOT EXISTS idx_product_price ON products(price);
CREATE INDEX IF NOT EXISTS idx_customer_email ON customers(email);

-- Update inventory to create activity
UPDATE products SET stock_quantity = stock_quantity - 1 WHERE stock_quantity > 10 ORDER BY RAND() LIMIT 50;
UPDATE products SET stock_quantity = stock_quantity + 10 WHERE stock_quantity < 20 ORDER BY RAND() LIMIT 20;

-- Run some analytical queries to populate performance_schema
SELECT COUNT(*) as total_products, AVG(price) as avg_price, MAX(price) as max_price FROM products;
SELECT status, COUNT(*) as order_count, SUM(total_amount) as revenue FROM orders GROUP BY status;
SELECT p.name, SUM(oi.quantity) as units_sold FROM order_items oi JOIN products p ON oi.product_id = p.product_id GROUP BY p.product_id ORDER BY units_sold DESC LIMIT 10;

SHOW STATUS;
SHOW ENGINE INNODB STATUS;
" 2>/dev/null

echo "✅ Data population complete!"
echo
echo "Database stats:"
docker exec mysql-primary mysql -u root -prootpassword ecommerce -e "
SELECT 'Products' as entity, COUNT(*) as count FROM products
UNION ALL
SELECT 'Customers', COUNT(*) FROM customers
UNION ALL
SELECT 'Orders', COUNT(*) FROM orders
UNION ALL
SELECT 'Order Items', COUNT(*) FROM order_items;
" 2>/dev/null