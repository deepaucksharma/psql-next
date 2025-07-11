-- Create sample database for testing
CREATE DATABASE IF NOT EXISTS ecommerce;
USE ecommerce;

-- Create sample tables
CREATE TABLE IF NOT EXISTS customers (
    customer_id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_email (email),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS products (
    product_id INT AUTO_INCREMENT PRIMARY KEY,
    sku VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    stock_quantity INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_sku (sku),
    INDEX idx_price (price),
    FULLTEXT INDEX idx_name_desc (name, description)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS orders (
    order_id INT AUTO_INCREMENT PRIMARY KEY,
    customer_id INT NOT NULL,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status ENUM('pending', 'processing', 'shipped', 'delivered', 'cancelled') DEFAULT 'pending',
    total_amount DECIMAL(10, 2) NOT NULL,
    FOREIGN KEY (customer_id) REFERENCES customers(customer_id),
    INDEX idx_customer_id (customer_id),
    INDEX idx_order_date (order_date),
    INDEX idx_status (status)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS order_items (
    order_item_id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity INT NOT NULL,
    unit_price DECIMAL(10, 2) NOT NULL,
    FOREIGN KEY (order_id) REFERENCES orders(order_id),
    FOREIGN KEY (product_id) REFERENCES products(product_id),
    INDEX idx_order_id (order_id),
    INDEX idx_product_id (product_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS inventory_log (
    log_id INT AUTO_INCREMENT PRIMARY KEY,
    product_id INT NOT NULL,
    change_type ENUM('restock', 'sale', 'adjustment') NOT NULL,
    quantity_change INT NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (product_id) REFERENCES products(product_id),
    INDEX idx_product_timestamp (product_id, timestamp)
) ENGINE=InnoDB;

-- Insert sample data
INSERT INTO customers (email, first_name, last_name) VALUES
    ('john.doe@example.com', 'John', 'Doe'),
    ('jane.smith@example.com', 'Jane', 'Smith'),
    ('bob.johnson@example.com', 'Bob', 'Johnson'),
    ('alice.williams@example.com', 'Alice', 'Williams'),
    ('charlie.brown@example.com', 'Charlie', 'Brown');

INSERT INTO products (sku, name, description, price, stock_quantity) VALUES
    ('LAPTOP001', 'Professional Laptop', 'High-performance laptop for professionals', 1299.99, 50),
    ('MOUSE001', 'Wireless Mouse', 'Ergonomic wireless mouse', 29.99, 200),
    ('KEYBOARD001', 'Mechanical Keyboard', 'RGB mechanical keyboard', 89.99, 100),
    ('MONITOR001', '27" 4K Monitor', 'Ultra HD monitor for productivity', 499.99, 30),
    ('HEADSET001', 'Gaming Headset', 'Premium gaming headset with surround sound', 79.99, 75);

-- Create stored procedures for generating load
DELIMITER //

CREATE PROCEDURE generate_orders(IN num_orders INT)
BEGIN
    DECLARE i INT DEFAULT 0;
    DECLARE customer_id INT;
    DECLARE product_id INT;
    DECLARE quantity INT;
    DECLARE price DECIMAL(10, 2);
    
    WHILE i < num_orders DO
        -- Random customer
        SELECT customer_id INTO customer_id FROM customers ORDER BY RAND() LIMIT 1;
        
        -- Create order
        INSERT INTO orders (customer_id, total_amount) VALUES (customer_id, 0);
        SET @order_id = LAST_INSERT_ID();
        
        -- Add 1-5 random items
        SET @items = FLOOR(1 + RAND() * 5);
        SET @j = 0;
        SET @total = 0;
        
        WHILE @j < @items DO
            SELECT product_id, price INTO product_id, price FROM products ORDER BY RAND() LIMIT 1;
            SET quantity = FLOOR(1 + RAND() * 5);
            
            INSERT INTO order_items (order_id, product_id, quantity, unit_price) 
            VALUES (@order_id, product_id, quantity, price);
            
            SET @total = @total + (quantity * price);
            SET @j = @j + 1;
        END WHILE;
        
        -- Update order total
        UPDATE orders SET total_amount = @total WHERE order_id = @order_id;
        
        SET i = i + 1;
    END WHILE;
END//

CREATE PROCEDURE simulate_slow_query()
BEGIN
    -- Intentionally slow query for testing
    SELECT 
        c.customer_id,
        c.email,
        COUNT(DISTINCT o.order_id) as order_count,
        SUM(o.total_amount) as total_spent,
        AVG(o.total_amount) as avg_order_value,
        GROUP_CONCAT(DISTINCT p.name) as products_purchased
    FROM customers c
    LEFT JOIN orders o ON c.customer_id = o.customer_id
    LEFT JOIN order_items oi ON o.order_id = oi.order_id
    LEFT JOIN products p ON oi.product_id = p.product_id
    WHERE o.order_date >= DATE_SUB(NOW(), INTERVAL 1 YEAR)
    GROUP BY c.customer_id, c.email
    HAVING order_count > 0
    ORDER BY total_spent DESC;
END//

DELIMITER ;

-- Grant permissions to app user
GRANT ALL PRIVILEGES ON ecommerce.* TO 'appuser'@'%';
FLUSH PRIVILEGES;