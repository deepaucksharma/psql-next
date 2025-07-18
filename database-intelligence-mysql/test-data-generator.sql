-- Test data generator for wait analysis validation

-- Create test database and tables
CREATE DATABASE IF NOT EXISTS wait_test;
USE wait_test;

-- Create a table with various index scenarios
CREATE TABLE IF NOT EXISTS test_orders (
    id INT PRIMARY KEY AUTO_INCREMENT,
    customer_id INT,
    order_date DATETIME,
    status VARCHAR(20),
    total_amount DECIMAL(10,2),
    description TEXT,
    INDEX idx_customer (customer_id),
    INDEX idx_date (order_date)
) ENGINE=InnoDB;

-- Create a table without indexes (for full scan testing)
CREATE TABLE IF NOT EXISTS test_logs (
    id INT PRIMARY KEY AUTO_INCREMENT,
    log_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    message TEXT,
    severity VARCHAR(10)
) ENGINE=InnoDB;

-- Stored procedure to generate test workload
DELIMITER //

CREATE PROCEDURE generate_test_workload(
    IN iterations INT,
    IN workload_type VARCHAR(20)
)
BEGIN
    DECLARE i INT DEFAULT 0;
    
    WHILE i < iterations DO
        CASE workload_type
            WHEN 'io' THEN
                -- I/O intensive queries
                SELECT COUNT(*) FROM test_orders 
                WHERE description LIKE CONCAT('%', RAND(), '%');
                
            WHEN 'lock' THEN
                -- Lock intensive queries
                START TRANSACTION;
                UPDATE test_orders 
                SET status = 'processing' 
                WHERE id = FLOOR(1 + RAND() * 1000);
                SELECT SLEEP(0.1);
                COMMIT;
                
            WHEN 'cpu' THEN
                -- CPU intensive queries
                SELECT 
                    customer_id,
                    COUNT(*) as cnt,
                    AVG(total_amount) as avg_amount,
                    STDDEV(total_amount) as stddev_amount
                FROM test_orders
                GROUP BY customer_id
                HAVING cnt > 5
                ORDER BY avg_amount DESC;
                
            WHEN 'mixed' THEN
                -- Mixed workload
                IF i % 3 = 0 THEN
                    CALL generate_test_workload(1, 'io');
                ELSEIF i % 3 = 1 THEN
                    CALL generate_test_workload(1, 'lock');
                ELSE
                    CALL generate_test_workload(1, 'cpu');
                END IF;
                
        END CASE;
        
        SET i = i + 1;
    END WHILE;
END//

DELIMITER ;

-- Insert sample data
INSERT INTO test_orders (customer_id, order_date, status, total_amount, description)
SELECT 
    FLOOR(1 + RAND() * 1000),
    DATE_SUB(NOW(), INTERVAL FLOOR(RAND() * 365) DAY),
    ELT(FLOOR(1 + RAND() * 4), 'pending', 'processing', 'completed', 'cancelled'),
    ROUND(RAND() * 1000, 2),
    CONCAT('Order description ', UUID())
FROM 
    (SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5) t1,
    (SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5) t2,
    (SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5) t3,
    (SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5) t4
ON DUPLICATE KEY UPDATE id=id;

-- Enable wait analysis
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/%';

UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME LIKE '%waits%';

SELECT 'Test data created. Run: CALL generate_test_workload(100, "mixed");' as message;
