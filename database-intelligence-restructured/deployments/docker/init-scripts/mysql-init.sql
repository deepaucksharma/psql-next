-- MySQL initialization script for database intelligence monitoring and E2E tests

-- Create monitoring user with necessary permissions
CREATE USER IF NOT EXISTS 'monitoring'@'%' IDENTIFIED BY 'monitoring_password';

-- Grant necessary permissions for monitoring
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'monitoring'@'%';
GRANT SELECT ON performance_schema.* TO 'monitoring'@'%';
GRANT SELECT ON mysql.* TO 'monitoring'@'%';

-- Enable performance schema if not already enabled
SET GLOBAL performance_schema = ON;

-- Configure performance schema consumers
UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME = 'events_statements_history';
UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME = 'events_statements_history_long';
UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME = 'events_statements_current';

-- Configure performance schema instruments
UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME LIKE 'statement/%';
UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME LIKE 'wait/io/file/%';
UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME LIKE 'wait/lock/%';

-- Create test database
CREATE DATABASE IF NOT EXISTS testdb;
USE testdb;

-- Create comprehensive monitoring tables
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) DEFAULT 'pending',
    total_amount DECIMAL(10, 2),
    FOREIGN KEY (user_id) REFERENCES users(id),
    INDEX idx_status (status),
    INDEX idx_user_id (user_id)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock_quantity INT DEFAULT 0,
    category VARCHAR(100),
    INDEX idx_category (category)
) ENGINE=InnoDB;

-- Create simple E2E test tables
CREATE TABLE IF NOT EXISTS test_users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS test_orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    amount DECIMAL(10,2),
    status VARCHAR(20),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES test_users(id)
) ENGINE=InnoDB;

-- Create a table for testing table locks
CREATE TABLE IF NOT EXISTS test_locks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    data VARCHAR(255),
    lock_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- Insert sample data into comprehensive monitoring tables
INSERT INTO users (username, email) VALUES 
    ('john_doe', 'john@example.com'),
    ('jane_smith', 'jane@example.com'),
    ('test_user', 'test@example.com')
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP;

INSERT INTO products (name, price, stock_quantity, category) VALUES
    ('Product A', 29.99, 100, 'Electronics'),
    ('Product B', 49.99, 50, 'Electronics'),
    ('Product C', 19.99, 200, 'Books'),
    ('Product D', 9.99, 500, 'Books'),
    ('Product E', 99.99, 25, 'Electronics');

-- Insert sample data into E2E test tables
INSERT INTO test_users (username, email) VALUES
    ('user1', 'user1@example.com'),
    ('user2', 'user2@example.com'),
    ('user3', 'user3@example.com')
ON DUPLICATE KEY UPDATE email = VALUES(email);

INSERT INTO test_orders (user_id, amount, status) VALUES
    (1, 99.99, 'completed'),
    (1, 149.50, 'pending'),
    (2, 75.00, 'completed'),
    (3, 200.00, 'cancelled');

-- Create stored procedure for testing slow queries
DELIMITER //
CREATE PROCEDURE IF NOT EXISTS slow_procedure()
BEGIN
    DECLARE counter INT DEFAULT 0;
    WHILE counter < 1000 DO
        SELECT COUNT(*) FROM users WHERE id = FLOOR(RAND() * 100);
        SET counter = counter + 1;
    END WHILE;
END//
DELIMITER ;

-- Create function for testing
DELIMITER //
CREATE FUNCTION IF NOT EXISTS get_user_order_count(input_user_id INT) RETURNS INT
READS SQL DATA
DETERMINISTIC
BEGIN
    DECLARE order_count INT;
    SELECT COUNT(*) INTO order_count FROM orders WHERE user_id = input_user_id;
    RETURN order_count;
END//
DELIMITER ;

-- Grant monitoring user access to test database
GRANT SELECT ON testdb.* TO 'monitoring'@'%';

-- Enable slow query log
SET GLOBAL slow_query_log = ON;
SET GLOBAL long_query_time = 1;

-- Enable general query log (disabled by default, enable for debugging)
SET GLOBAL general_log = OFF;

-- Configure InnoDB monitoring
SET GLOBAL innodb_monitor_enable = 'all';

-- Additional monitoring configuration
SET GLOBAL log_queries_not_using_indexes = ON;
SET GLOBAL log_slow_admin_statements = ON;
SET GLOBAL log_slow_slave_statements = ON;

-- Performance optimization settings
SET GLOBAL query_cache_type = ON;
SET GLOBAL query_cache_size = 67108864; -- 64MB

-- Flush privileges to ensure all permissions are applied
FLUSH PRIVILEGES;