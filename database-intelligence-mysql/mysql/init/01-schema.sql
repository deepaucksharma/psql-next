-- Combined MySQL initialization script
-- Creates monitoring user, enables performance schema, and sets up sample database

-- 1. Create monitoring user with necessary privileges
CREATE USER IF NOT EXISTS 'otel_monitor'@'%' IDENTIFIED BY 'otelmonitorpass';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON mysql.* TO 'otel_monitor'@'%';
GRANT SELECT ON sys.* TO 'otel_monitor'@'%';
FLUSH PRIVILEGES;

-- 2. Ensure Performance Schema is properly configured
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES'
WHERE NAME LIKE '%statement/%' OR NAME LIKE '%stage/%';

UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES'
WHERE NAME LIKE '%events%' OR NAME LIKE '%statements%' OR NAME LIKE '%stages%';

-- 3. Create sample ecommerce database
CREATE DATABASE IF NOT EXISTS ecommerce;
USE ecommerce;

-- Customers table
CREATE TABLE IF NOT EXISTS customers (
    id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_email (email),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Products table
CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    sku VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    stock_quantity INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_sku (sku),
    INDEX idx_price (price),
    INDEX idx_stock (stock_quantity)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    customer_id INT NOT NULL,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) DEFAULT 'pending',
    total_amount DECIMAL(10, 2) NOT NULL,
    FOREIGN KEY (customer_id) REFERENCES customers(id),
    INDEX idx_customer (customer_id),
    INDEX idx_date (order_date),
    INDEX idx_status (status),
    INDEX idx_customer_date (customer_id, order_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Order items table
CREATE TABLE IF NOT EXISTS order_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity INT NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    FOREIGN KEY (order_id) REFERENCES orders(id),
    FOREIGN KEY (product_id) REFERENCES products(id),
    INDEX idx_order (order_id),
    INDEX idx_product (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Shopping cart table
CREATE TABLE IF NOT EXISTS shopping_cart (
    id INT AUTO_INCREMENT PRIMARY KEY,
    customer_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (customer_id) REFERENCES customers(id),
    FOREIGN KEY (product_id) REFERENCES products(id),
    UNIQUE KEY unique_customer_product (customer_id, product_id),
    INDEX idx_customer (customer_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Create stored procedures for workload generation
DELIMITER //

CREATE PROCEDURE IF NOT EXISTS place_order(IN p_customer_id INT)
BEGIN
    DECLARE v_order_id INT;
    DECLARE v_total DECIMAL(10, 2) DEFAULT 0;
    
    START TRANSACTION;
    
    -- Create order
    INSERT INTO orders (customer_id, status, total_amount)
    VALUES (p_customer_id, 'pending', 0);
    
    SET v_order_id = LAST_INSERT_ID();
    
    -- Move items from cart to order
    INSERT INTO order_items (order_id, product_id, quantity, price)
    SELECT v_order_id, sc.product_id, sc.quantity, p.price
    FROM shopping_cart sc
    JOIN products p ON sc.product_id = p.id
    WHERE sc.customer_id = p_customer_id;
    
    -- Calculate total
    SELECT SUM(quantity * price) INTO v_total
    FROM order_items
    WHERE order_id = v_order_id;
    
    -- Update order total
    UPDATE orders SET total_amount = v_total WHERE id = v_order_id;
    
    -- Clear shopping cart
    DELETE FROM shopping_cart WHERE customer_id = p_customer_id;
    
    -- Update product stock
    UPDATE products p
    JOIN order_items oi ON p.id = oi.product_id
    SET p.stock_quantity = p.stock_quantity - oi.quantity
    WHERE oi.order_id = v_order_id;
    
    COMMIT;
END//

CREATE PROCEDURE IF NOT EXISTS browse_products()
BEGIN
    -- Simulate various product browsing queries
    SELECT * FROM products WHERE price < 50 ORDER BY RAND() LIMIT 10;
    SELECT * FROM products WHERE stock_quantity > 0 ORDER BY price DESC LIMIT 5;
    SELECT name, price FROM products WHERE name LIKE '%a%' LIMIT 20;
END//

CREATE PROCEDURE IF NOT EXISTS run_analytics()
BEGIN
    -- Daily sales summary
    SELECT DATE(order_date) as sale_date, COUNT(*) as order_count, SUM(total_amount) as revenue
    FROM orders
    WHERE order_date >= DATE_SUB(NOW(), INTERVAL 7 DAY)
    GROUP BY DATE(order_date);
    
    -- Top selling products
    SELECT p.name, SUM(oi.quantity) as units_sold, SUM(oi.quantity * oi.price) as revenue
    FROM order_items oi
    JOIN products p ON oi.product_id = p.id
    JOIN orders o ON oi.order_id = o.id
    WHERE o.order_date >= DATE_SUB(NOW(), INTERVAL 30 DAY)
    GROUP BY p.id
    ORDER BY units_sold DESC
    LIMIT 10;
END//

CREATE PROCEDURE IF NOT EXISTS manage_cart(IN p_customer_id INT, IN p_product_id INT, IN p_quantity INT)
BEGIN
    IF p_quantity > 0 THEN
        INSERT INTO shopping_cart (customer_id, product_id, quantity)
        VALUES (p_customer_id, p_product_id, p_quantity)
        ON DUPLICATE KEY UPDATE quantity = p_quantity;
    ELSE
        DELETE FROM shopping_cart 
        WHERE customer_id = p_customer_id AND product_id = p_product_id;
    END IF;
END//

DELIMITER ;

-- Grant execute permissions to app user
GRANT EXECUTE ON ecommerce.* TO 'appuser'@'%';
GRANT SELECT, INSERT, UPDATE, DELETE ON ecommerce.* TO 'appuser'@'%';

-- Performance optimization
ANALYZE TABLE customers, products, orders, order_items, shopping_cart;