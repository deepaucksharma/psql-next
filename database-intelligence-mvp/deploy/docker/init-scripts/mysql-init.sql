-- MySQL initialization script for Database Intelligence MVP
-- Creates monitoring user and sets up performance schema

-- Create monitoring user with read-only access
CREATE USER IF NOT EXISTS 'newrelic_monitor'@'%' IDENTIFIED BY 'monitor123';

-- Grant necessary permissions for monitoring
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'newrelic_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'newrelic_monitor'@'%';
GRANT SELECT ON mysql.* TO 'newrelic_monitor'@'%';

-- Create sample database and tables
USE testdb;

-- Create sample tables
CREATE TABLE IF NOT EXISTS customers (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    customer_id INT,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    total_amount DECIMAL(10, 2),
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (customer_id) REFERENCES customers(id),
    INDEX idx_customer_id (customer_id),
    INDEX idx_status (status)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2),
    stock_quantity INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS order_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT,
    product_id INT,
    quantity INT NOT NULL,
    unit_price DECIMAL(10, 2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES orders(id),
    FOREIGN KEY (product_id) REFERENCES products(id),
    INDEX idx_order_id (order_id),
    INDEX idx_product_id (product_id)
) ENGINE=InnoDB;

-- Insert sample data
INSERT INTO customers (name, email) VALUES
    ('John Doe', 'john.doe@example.com'),
    ('Jane Smith', 'jane.smith@example.com'),
    ('Bob Johnson', 'bob.johnson@example.com'),
    ('Alice Williams', 'alice.williams@example.com'),
    ('Charlie Brown', 'charlie.brown@example.com');

INSERT INTO products (name, description, price, stock_quantity) VALUES
    ('Laptop Pro', 'High-performance laptop for professionals', 1299.99, 50),
    ('Wireless Mouse', 'Ergonomic wireless mouse', 29.99, 200),
    ('USB-C Hub', 'Multi-port USB-C hub', 49.99, 150),
    ('Mechanical Keyboard', 'RGB mechanical keyboard', 129.99, 75),
    ('4K Monitor', '27-inch 4K IPS monitor', 499.99, 30),
    ('Webcam HD', '1080p HD webcam', 79.99, 100),
    ('Desk Lamp', 'LED desk lamp with USB charging', 39.99, 120),
    ('Laptop Stand', 'Adjustable laptop stand', 34.99, 80);

-- Create stored procedure for generating workload
DELIMITER //

CREATE PROCEDURE generate_random_order()
BEGIN
    DECLARE v_customer_id INT;
    DECLARE v_order_id INT;
    DECLARE v_num_items INT;
    DECLARE v_product_id INT;
    DECLARE v_quantity INT;
    DECLARE v_price DECIMAL(10, 2);
    DECLARE v_total DECIMAL(10, 2) DEFAULT 0;
    DECLARE i INT DEFAULT 0;
    
    -- Select random customer
    SELECT id INTO v_customer_id FROM customers ORDER BY RAND() LIMIT 1;
    
    -- Create order
    INSERT INTO orders (customer_id, total_amount) VALUES (v_customer_id, 0);
    SET v_order_id = LAST_INSERT_ID();
    
    -- Add random number of items (1-5)
    SET v_num_items = FLOOR(RAND() * 5 + 1);
    
    WHILE i < v_num_items DO
        -- Select random product
        SELECT id, price INTO v_product_id, v_price 
        FROM products ORDER BY RAND() LIMIT 1;
        
        -- Random quantity (1-3)
        SET v_quantity = FLOOR(RAND() * 3 + 1);
        
        -- Insert order item
        INSERT INTO order_items (order_id, product_id, quantity, unit_price)
        VALUES (v_order_id, v_product_id, v_quantity, v_price);
        
        SET v_total = v_total + (v_price * v_quantity);
        SET i = i + 1;
    END WHILE;
    
    -- Update order total
    UPDATE orders SET total_amount = v_total WHERE id = v_order_id;
END//

DELIMITER ;

-- Grant execute permission
GRANT EXECUTE ON testdb.* TO 'newrelic_monitor'@'%';

-- Create view for monitoring
CREATE OR REPLACE VIEW database_statistics AS
SELECT 
    table_schema,
    table_name,
    table_rows,
    data_length,
    index_length,
    data_free,
    auto_increment
FROM information_schema.tables
WHERE table_schema = 'testdb';

-- Grant permissions on the view
GRANT SELECT ON testdb.database_statistics TO 'newrelic_monitor'@'%';

-- Ensure Performance Schema is properly configured
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE '%statement%';

UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME LIKE '%statement%';

-- Flush privileges
FLUSH PRIVILEGES;

-- Verify setup
SELECT 'MySQL initialization completed successfully' AS status;