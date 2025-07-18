-- Create test database and tables for wait analysis testing
CREATE DATABASE IF NOT EXISTS wait_analysis_test;
USE wait_analysis_test;

-- Table with various index scenarios for I/O wait testing
CREATE TABLE IF NOT EXISTS orders (
    order_id INT PRIMARY KEY AUTO_INCREMENT,
    customer_id INT NOT NULL,
    order_date DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    total_amount DECIMAL(10,2) NOT NULL,
    shipping_address TEXT,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_customer (customer_id),
    INDEX idx_date (order_date),
    INDEX idx_status (status)
) ENGINE=InnoDB;

-- Table without proper indexes (for missing index detection)
CREATE TABLE IF NOT EXISTS order_items (
    item_id INT PRIMARY KEY AUTO_INCREMENT,
    order_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    unit_price DECIMAL(10,2) NOT NULL,
    discount DECIMAL(5,2) DEFAULT 0,
    -- Intentionally missing foreign key index on order_id
    FOREIGN KEY (order_id) REFERENCES orders(order_id)
) ENGINE=InnoDB;

-- Table for lock contention testing
CREATE TABLE IF NOT EXISTS inventory (
    product_id INT PRIMARY KEY,
    product_name VARCHAR(255) NOT NULL,
    quantity_available INT NOT NULL DEFAULT 0,
    reserved_quantity INT NOT NULL DEFAULT 0,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    version INT NOT NULL DEFAULT 0
) ENGINE=InnoDB;

-- Table for large scan testing
CREATE TABLE IF NOT EXISTS audit_log (
    log_id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id INT,
    action VARCHAR(50),
    object_type VARCHAR(50),
    object_id INT,
    details JSON,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    -- No indexes on purpose to cause full table scans
) ENGINE=InnoDB;

-- Insert test data
DELIMITER //

CREATE PROCEDURE IF NOT EXISTS generate_test_data()
BEGIN
    DECLARE i INT DEFAULT 0;
    DECLARE j INT DEFAULT 0;
    
    -- Generate customers and orders
    WHILE i < 1000 DO
        -- Insert order
        INSERT INTO orders (customer_id, order_date, status, total_amount, shipping_address)
        VALUES (
            FLOOR(1 + RAND() * 100),
            DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 365) DAY),
            CASE FLOOR(RAND() * 4) 
                WHEN 0 THEN 'pending'
                WHEN 1 THEN 'processing'
                WHEN 2 THEN 'shipped'
                ELSE 'completed'
            END,
            ROUND(10 + RAND() * 990, 2),
            CONCAT('Address ', i)
        );
        
        SET @last_order_id = LAST_INSERT_ID();
        
        -- Insert order items (1-5 items per order)
        SET j = 0;
        WHILE j < FLOOR(1 + RAND() * 5) DO
            INSERT INTO order_items (order_id, product_id, quantity, unit_price)
            VALUES (
                @last_order_id,
                FLOOR(1 + RAND() * 50),
                FLOOR(1 + RAND() * 10),
                ROUND(5 + RAND() * 95, 2)
            );
            SET j = j + 1;
        END WHILE;
        
        SET i = i + 1;
    END WHILE;
    
    -- Generate inventory
    SET i = 1;
    WHILE i <= 50 DO
        INSERT INTO inventory (product_id, product_name, quantity_available, reserved_quantity)
        VALUES (i, CONCAT('Product ', i), FLOOR(RAND() * 1000), 0);
        SET i = i + 1;
    END WHILE;
    
    -- Generate audit log entries
    SET i = 0;
    WHILE i < 10000 DO
        INSERT INTO audit_log (user_id, action, object_type, object_id, details, ip_address)
        VALUES (
            FLOOR(1 + RAND() * 100),
            CASE FLOOR(RAND() * 4)
                WHEN 0 THEN 'CREATE'
                WHEN 1 THEN 'UPDATE'
                WHEN 2 THEN 'DELETE'
                ELSE 'VIEW'
            END,
            CASE FLOOR(RAND() * 3)
                WHEN 0 THEN 'order'
                WHEN 1 THEN 'product'
                ELSE 'customer'
            END,
            FLOOR(1 + RAND() * 1000),
            JSON_OBJECT('timestamp', NOW(), 'source', 'test'),
            CONCAT(
                FLOOR(RAND() * 256), '.', 
                FLOOR(RAND() * 256), '.', 
                FLOOR(RAND() * 256), '.', 
                FLOOR(RAND() * 256)
            )
        );
        SET i = i + 1;
    END WHILE;
END//

-- Stored procedures to generate different wait patterns
CREATE PROCEDURE IF NOT EXISTS generate_io_waits()
BEGIN
    -- Force table scan on large table without proper index
    SELECT COUNT(*) INTO @count 
    FROM order_items oi
    JOIN orders o ON oi.order_id = o.order_id
    WHERE o.order_date > DATE_SUB(NOW(), INTERVAL 30 DAY)
        AND oi.quantity > 5;
    
    -- Another I/O intensive query
    SELECT 
        customer_id,
        COUNT(*) as order_count,
        SUM(total_amount) as total_spent
    FROM orders
    WHERE status IN ('completed', 'shipped')
    GROUP BY customer_id
    HAVING total_spent > 1000;
END//

CREATE PROCEDURE IF NOT EXISTS generate_lock_waits()
BEGIN
    DECLARE EXIT HANDLER FOR SQLEXCEPTION
    BEGIN
        ROLLBACK;
    END;
    
    START TRANSACTION;
    
    -- Lock multiple inventory rows
    UPDATE inventory 
    SET reserved_quantity = reserved_quantity + 1,
        version = version + 1
    WHERE product_id IN (1, 2, 3, 4, 5)
    ORDER BY product_id;
    
    -- Simulate some processing time
    DO SLEEP(0.1);
    
    COMMIT;
END//

CREATE PROCEDURE IF NOT EXISTS generate_temp_table_waits()
BEGIN
    -- Query that creates temp tables on disk
    SELECT 
        o.customer_id,
        COUNT(DISTINCT o.order_id) as order_count,
        COUNT(DISTINCT oi.product_id) as unique_products,
        GROUP_CONCAT(DISTINCT o.status) as order_statuses,
        SUM(oi.quantity * oi.unit_price) as total_value,
        AVG(oi.quantity * oi.unit_price) as avg_order_value,
        MAX(o.order_date) as last_order_date
    FROM orders o
    JOIN order_items oi ON o.order_id = oi.order_id
    LEFT JOIN inventory i ON oi.product_id = i.product_id
    GROUP BY o.customer_id
    HAVING order_count > 5
    ORDER BY total_value DESC, last_order_date DESC
    LIMIT 100;
END//

DELIMITER ;

-- Generate initial test data
CALL generate_test_data();

-- Create events to generate continuous workload (disabled by default)
CREATE EVENT IF NOT EXISTS e_generate_io_load
ON SCHEDULE EVERY 10 SECOND
DISABLE
DO CALL generate_io_waits();

CREATE EVENT IF NOT EXISTS e_generate_lock_load
ON SCHEDULE EVERY 5 SECOND
DISABLE
DO CALL generate_lock_waits();

-- Grant permissions to monitor user
GRANT SELECT, EXECUTE ON wait_analysis_test.* TO 'otel_monitor'@'%';

SELECT 'Test workload setup completed' as status;