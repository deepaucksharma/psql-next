-- MySQL Initialization Script for Database Intelligence Collector
-- This script sets up the necessary configurations, users, and sample data

-- Enable performance schema if not already enabled
-- Note: This typically requires server restart with performance_schema=ON

-- Create monitoring user with appropriate permissions
CREATE USER IF NOT EXISTS 'monitoring_user'@'%' IDENTIFIED BY 'monitoring';

-- Grant necessary permissions to monitoring user
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'monitoring_user'@'%';
GRANT SELECT ON performance_schema.* TO 'monitoring_user'@'%';
GRANT SELECT ON mysql.* TO 'monitoring_user'@'%';
GRANT SELECT ON sys.* TO 'monitoring_user'@'%';

-- Use the testdb database
USE testdb;

-- Create sample tables
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP NULL,
    is_active BOOLEAN DEFAULT true,
    INDEX idx_email (email),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    stock_quantity INT NOT NULL DEFAULT 0,
    category VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_category (category),
    INDEX idx_price (price)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'pending',
    total_amount DECIMAL(10, 2),
    shipping_address TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id),
    INDEX idx_user_id (user_id),
    INDEX idx_order_date (order_date)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS order_items (
    id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT,
    product_id INT,
    quantity INT NOT NULL,
    unit_price DECIMAL(10, 2) NOT NULL,
    subtotal DECIMAL(10, 2) GENERATED ALWAYS AS (quantity * unit_price) STORED,
    FOREIGN KEY (order_id) REFERENCES orders(id),
    FOREIGN KEY (product_id) REFERENCES products(id),
    INDEX idx_order_id (order_id),
    INDEX idx_product_id (product_id)
) ENGINE=InnoDB;

-- Create stored procedure to generate sample data
DELIMITER //

CREATE PROCEDURE IF NOT EXISTS generate_test_data(
    IN num_users INT,
    IN num_products INT,
    IN num_orders INT
)
BEGIN
    DECLARE i INT DEFAULT 1;
    DECLARE user_id INT;
    DECLARE product_id INT;
    DECLARE num_items INT;
    DECLARE order_id INT;
    DECLARE j INT;
    
    -- Disable foreign key checks for faster insertion
    SET FOREIGN_KEY_CHECKS = 0;
    
    -- Generate users
    WHILE i <= num_users DO
        INSERT INTO users (username, email, last_login, is_active)
        VALUES (
            CONCAT('user_', i),
            CONCAT('user', i, '@example.com'),
            DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 365) DAY),
            RAND() > 0.1
        );
        SET i = i + 1;
    END WHILE;
    
    -- Generate products
    SET i = 1;
    WHILE i <= num_products DO
        INSERT INTO products (name, description, price, stock_quantity, category)
        VALUES (
            CONCAT('Product ', i),
            CONCAT('Description for product ', i),
            ROUND(RAND() * 1000, 2),
            FLOOR(RAND() * 100),
            ELT(FLOOR(RAND() * 5) + 1, 'Electronics', 'Clothing', 'Books', 'Home', 'Other')
        );
        SET i = i + 1;
    END WHILE;
    
    -- Generate orders
    SET i = 1;
    WHILE i <= num_orders DO
        SET user_id = FLOOR(RAND() * num_users) + 1;
        
        INSERT INTO orders (user_id, order_date, status, total_amount, shipping_address)
        VALUES (
            user_id,
            DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 90) DAY),
            ELT(FLOOR(RAND() * 4) + 1, 'pending', 'processing', 'shipped', 'delivered'),
            0,
            CONCAT('Address for user ', user_id)
        );
        
        SET order_id = LAST_INSERT_ID();
        
        -- Generate order items (1-5 items per order)
        SET num_items = FLOOR(RAND() * 5) + 1;
        SET j = 1;
        
        WHILE j <= num_items DO
            SET product_id = FLOOR(RAND() * num_products) + 1;
            
            INSERT INTO order_items (order_id, product_id, quantity, unit_price)
            SELECT 
                order_id,
                product_id,
                FLOOR(RAND() * 5) + 1,
                price
            FROM products WHERE id = product_id;
            
            SET j = j + 1;
        END WHILE;
        
        -- Update order total
        UPDATE orders o
        SET total_amount = (
            SELECT SUM(subtotal) 
            FROM order_items 
            WHERE order_id = o.id
        )
        WHERE id = order_id;
        
        SET i = i + 1;
    END WHILE;
    
    -- Re-enable foreign key checks
    SET FOREIGN_KEY_CHECKS = 1;
END//

DELIMITER ;

-- Create views for monitoring queries
CREATE OR REPLACE VIEW active_users_summary AS
SELECT 
    COUNT(*) as total_users,
    SUM(CASE WHEN is_active THEN 1 ELSE 0 END) as active_users,
    SUM(CASE WHEN last_login > DATE_SUB(NOW(), INTERVAL 7 DAY) THEN 1 ELSE 0 END) as recently_active
FROM users;

CREATE OR REPLACE VIEW order_statistics AS
SELECT 
    DATE(order_date) as order_day,
    COUNT(*) as order_count,
    SUM(total_amount) as daily_revenue,
    COUNT(DISTINCT user_id) as unique_customers
FROM orders
GROUP BY DATE(order_date);

-- Create function to get user order history
DELIMITER //

CREATE FUNCTION IF NOT EXISTS get_user_total_orders(p_user_id INT)
RETURNS INT
DETERMINISTIC
READS SQL DATA
BEGIN
    DECLARE total_orders INT;
    
    SELECT COUNT(*) INTO total_orders
    FROM orders
    WHERE user_id = p_user_id;
    
    RETURN total_orders;
END//

CREATE PROCEDURE IF NOT EXISTS get_user_order_history(
    IN p_user_id INT,
    IN p_limit INT
)
BEGIN
    SELECT 
        o.id as order_id,
        o.order_date,
        o.status,
        o.total_amount,
        COUNT(oi.id) as item_count
    FROM orders o
    LEFT JOIN order_items oi ON o.id = oi.order_id
    WHERE o.user_id = p_user_id
    GROUP BY o.id, o.order_date, o.status, o.total_amount
    ORDER BY o.order_date DESC
    LIMIT p_limit;
END//

DELIMITER ;

-- Create slow query for testing
DELIMITER //

CREATE PROCEDURE IF NOT EXISTS generate_slow_query()
BEGIN
    -- Intentionally slow query for testing monitoring
    SELECT 
        u.username,
        COUNT(DISTINCT o.id) as order_count,
        SUM(o.total_amount) as total_spent,
        GROUP_CONCAT(DISTINCT p.category) as categories_purchased
    FROM users u
    CROSS JOIN orders o
    LEFT JOIN order_items oi ON o.id = oi.order_id
    LEFT JOIN products p ON oi.product_id = p.id
    WHERE u.id = o.user_id
    GROUP BY u.username
    HAVING order_count > 0
    ORDER BY total_spent DESC;
END//

DELIMITER ;

-- Grant permissions on testdb to monitoring user
GRANT SELECT ON testdb.* TO 'monitoring_user'@'%';
GRANT EXECUTE ON testdb.* TO 'monitoring_user'@'%';

-- Generate initial test data
CALL generate_test_data(100, 50, 500);

-- Create monitoring heartbeat table
CREATE TABLE IF NOT EXISTS monitoring_heartbeat (
    id INT AUTO_INCREMENT PRIMARY KEY,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'healthy'
) ENGINE=InnoDB;

-- Insert initial heartbeat
INSERT INTO monitoring_heartbeat (status) VALUES ('initialized');

-- Create events table for performance schema monitoring
CREATE TABLE IF NOT EXISTS monitoring_events (
    id INT AUTO_INCREMENT PRIMARY KEY,
    event_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    event_type VARCHAR(50),
    event_data JSON,
    INDEX idx_event_time (event_time),
    INDEX idx_event_type (event_type)
) ENGINE=InnoDB;

-- Enable query logging for monitoring (if needed)
-- SET GLOBAL log_output = 'TABLE';
-- SET GLOBAL general_log = 'ON';
-- SET GLOBAL slow_query_log = 'ON';
-- SET GLOBAL long_query_time = 1;

-- Output confirmation
SELECT 'Database initialization complete!' as status;
SELECT 'Monitoring user created: monitoring_user' as info;
SELECT 'Sample data generated in database: testdb' as info;
SELECT COUNT(*) as user_count FROM users;
SELECT COUNT(*) as product_count FROM products;
SELECT COUNT(*) as order_count FROM orders;