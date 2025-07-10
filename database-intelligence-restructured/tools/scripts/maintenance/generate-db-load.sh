#!/bin/bash
# Generate test load for PostgreSQL and MySQL databases

echo "🚀 Generating database load..."

# PostgreSQL load
echo "📊 PostgreSQL: Creating tables and data..."
docker exec db-intelligence-postgres psql -U postgres -d testdb <<EOF
-- Create sample schema if not exists
CREATE SCHEMA IF NOT EXISTS sample_app;

-- Create tables
CREATE TABLE IF NOT EXISTS sample_app.users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sample_app.products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price DECIMAL(10,2),
    stock_quantity INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sample_app.orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES sample_app.users(id),
    product_id INTEGER REFERENCES sample_app.products(id),
    quantity INTEGER NOT NULL,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'pending'
);

-- Insert sample data
INSERT INTO sample_app.users (username, email) 
SELECT 
    'user_' || generate_series,
    'user' || generate_series || '@example.com'
FROM generate_series(1, 100)
ON CONFLICT DO NOTHING;

INSERT INTO sample_app.products (name, price, stock_quantity)
SELECT 
    'Product ' || generate_series,
    (random() * 1000)::decimal(10,2),
    (random() * 100)::integer
FROM generate_series(1, 50)
ON CONFLICT DO NOTHING;

-- Generate some queries
SELECT COUNT(*) FROM sample_app.users;
SELECT COUNT(*) FROM sample_app.products;
SELECT pg_database_size(current_database());

-- Create some activity
UPDATE sample_app.users SET last_login = NOW() WHERE id = (random() * 100)::integer;
UPDATE sample_app.products SET stock_quantity = stock_quantity - 1 WHERE id = (random() * 50)::integer;

ANALYZE sample_app.users;
ANALYZE sample_app.products;
EOF

# MySQL load  
echo "📊 MySQL: Creating tables and data..."
docker exec db-intelligence-mysql mysql -u root -pmysql testdb <<EOF
-- Create tables
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price DECIMAL(10,2),
    stock_quantity INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    product_id INT,
    quantity INT NOT NULL,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'pending',
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (product_id) REFERENCES products(id)
);

-- Insert sample data
INSERT IGNORE INTO users (username, email) 
SELECT 
    CONCAT('mysql_user_', id),
    CONCAT('mysql_user', id, '@example.com')
FROM (
    SELECT @row := @row + 1 AS id 
    FROM (SELECT 0 UNION ALL SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7 UNION ALL SELECT 8 UNION ALL SELECT 9) t1,
    (SELECT 0 UNION ALL SELECT 1 UNION ALL SELECT 2 UNION ALL SELECT 3 UNION ALL SELECT 4 UNION ALL SELECT 5 UNION ALL SELECT 6 UNION ALL SELECT 7 UNION ALL SELECT 8 UNION ALL SELECT 9) t2,
    (SELECT @row:=0) t3
    LIMIT 100
) AS numbers;

-- Generate some queries
SELECT COUNT(*) FROM users;
SELECT COUNT(*) FROM products;
SELECT table_schema, SUM(data_length + index_length) / 1024 / 1024 AS size_mb 
FROM information_schema.tables 
WHERE table_schema = 'testdb' 
GROUP BY table_schema;

-- Create some activity
UPDATE users SET last_login = NOW() WHERE id = FLOOR(1 + (RAND() * 100));
UPDATE products SET stock_quantity = stock_quantity - 1 WHERE id = FLOOR(1 + (RAND() * 50));
EOF

echo "✅ Database load generation complete!"
echo ""
echo "📈 Run continuous load with:"
echo "   while true; do $0; sleep 10; done"