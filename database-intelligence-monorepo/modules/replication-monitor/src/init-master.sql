-- Master initialization script
-- This script sets up the master server for replication

-- Create replication user
CREATE USER IF NOT EXISTS 'replication_user'@'%' IDENTIFIED WITH mysql_native_password BY 'replication_password';
GRANT REPLICATION SLAVE ON *.* TO 'replication_user'@'%';
GRANT RELOAD, PROCESS, SHOW DATABASES, REPLICATION CLIENT ON *.* TO 'replication_user'@'%';
FLUSH PRIVILEGES;

-- Create monitoring user for collector
CREATE USER IF NOT EXISTS 'monitoring'@'%' IDENTIFIED BY 'monitoring_password';
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'monitoring'@'%';
GRANT SELECT ON performance_schema.* TO 'monitoring'@'%';
FLUSH PRIVILEGES;

-- Create test database and tables
CREATE DATABASE IF NOT EXISTS test_db;
USE test_db;

-- Create sample tables for replication testing
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL,
    email VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_username (username),
    INDEX idx_email (email)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    total_amount DECIMAL(10, 2) NOT NULL,
    status ENUM('pending', 'processing', 'completed', 'cancelled') DEFAULT 'pending',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_status (status),
    INDEX idx_order_date (order_date),
    FOREIGN KEY (user_id) REFERENCES users(id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS order_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT NOT NULL,
    product_name VARCHAR(100) NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    unit_price DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_order_id (order_id),
    FOREIGN KEY (order_id) REFERENCES orders(id)
) ENGINE=InnoDB;

-- Create stored procedure for generating test data
DELIMITER //

CREATE PROCEDURE generate_test_data(IN num_users INT, IN num_orders INT)
BEGIN
    DECLARE i INT DEFAULT 0;
    DECLARE j INT DEFAULT 0;
    DECLARE user_id INT;
    
    -- Generate users
    WHILE i < num_users DO
        INSERT INTO users (username, email)
        VALUES (
            CONCAT('user_', i),
            CONCAT('user_', i, '@example.com')
        );
        SET i = i + 1;
    END WHILE;
    
    -- Generate orders
    SET i = 0;
    WHILE i < num_orders DO
        SET user_id = FLOOR(1 + RAND() * num_users);
        INSERT INTO orders (user_id, total_amount, status)
        VALUES (
            user_id,
            ROUND(10 + RAND() * 990, 2),
            ELT(FLOOR(1 + RAND() * 4), 'pending', 'processing', 'completed', 'cancelled')
        );
        
        -- Generate order items
        SET j = 0;
        WHILE j < FLOOR(1 + RAND() * 5) DO
            INSERT INTO order_items (order_id, product_name, quantity, unit_price)
            VALUES (
                LAST_INSERT_ID(),
                CONCAT('Product_', FLOOR(1 + RAND() * 100)),
                FLOOR(1 + RAND() * 10),
                ROUND(5 + RAND() * 95, 2)
            );
            SET j = j + 1;
        END WHILE;
        
        SET i = i + 1;
    END WHILE;
END//

-- Create procedure for continuous data generation (for lag testing)
CREATE PROCEDURE generate_continuous_data()
BEGIN
    DECLARE counter INT DEFAULT 0;
    
    WHILE TRUE DO
        INSERT INTO users (username, email)
        VALUES (
            CONCAT('auto_user_', counter, '_', UNIX_TIMESTAMP()),
            CONCAT('auto_user_', counter, '_', UNIX_TIMESTAMP(), '@example.com')
        );
        
        INSERT INTO orders (user_id, total_amount, status)
        VALUES (
            LAST_INSERT_ID(),
            ROUND(10 + RAND() * 990, 2),
            'pending'
        );
        
        SET counter = counter + 1;
        
        -- Sleep for a short time to control data generation rate
        DO SLEEP(0.1);
        
        -- Reset counter after 10000 to prevent overflow
        IF counter > 10000 THEN
            SET counter = 0;
        END IF;
    END WHILE;
END//

DELIMITER ;

-- Generate initial test data
CALL generate_test_data(100, 500);

-- Create view for monitoring replication health
CREATE VIEW replication_health AS
SELECT 
    NOW() as check_time,
    @@server_id as server_id,
    @@hostname as hostname,
    'master' as role,
    (SELECT COUNT(*) FROM users) as user_count,
    (SELECT COUNT(*) FROM orders) as order_count,
    (SELECT COUNT(*) FROM order_items) as order_item_count;

-- Enable performance schema instruments for replication monitoring
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE '%replication%';

UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME LIKE '%replication%';

-- Log master status
SHOW MASTER STATUS;