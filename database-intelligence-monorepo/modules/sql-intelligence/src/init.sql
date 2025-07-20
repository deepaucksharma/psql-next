-- SQL Intelligence Module - MySQL Initialization Script
-- Ensures performance_schema is properly configured for comprehensive monitoring

-- Performance schema is enabled by default in MySQL 8.0
-- Verify it's enabled
SELECT @@performance_schema;

-- Configure statement events collection
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'statement/%';

-- Enable events_statements_summary tables
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME LIKE 'events_statements_%';

-- Enable table I/O instrumentation
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/io/table/%';

-- Enable lock instrumentation
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/lock/table/%';

-- Note: History sizes are read-only and must be set at server startup
-- Current sizes can be checked with:
SELECT @@performance_schema_events_statements_history_size;
SELECT @@performance_schema_events_statements_history_long_size;

-- Configure memory instrumentation
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES' 
WHERE NAME LIKE 'memory/%';

-- Enable all stages for comprehensive monitoring
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'stage/%';

-- Create test database and tables for integration testing
CREATE DATABASE IF NOT EXISTS test_intelligence;
USE test_intelligence;

-- Table without indexes (for testing index recommendations)
CREATE TABLE IF NOT EXISTS no_index_table (
    id INT PRIMARY KEY AUTO_INCREMENT,
    data VARCHAR(255),
    status INT,
    category VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Table with good indexes
CREATE TABLE IF NOT EXISTS indexed_table (
    id INT PRIMARY KEY AUTO_INCREMENT,
    data VARCHAR(255),
    status INT,
    category VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_status (status),
    INDEX idx_category (category),
    INDEX idx_created (created_at),
    INDEX idx_status_category (status, category)
);

-- Table for testing lock contention
CREATE TABLE IF NOT EXISTS lock_test_table (
    id INT PRIMARY KEY AUTO_INCREMENT,
    counter INT DEFAULT 0,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Generate sample data
INSERT INTO no_index_table (data, status, category) 
SELECT 
    CONCAT('data-', LPAD(seq.n, 6, '0')),
    FLOOR(RAND() * 10),
    ELT(FLOOR(RAND() * 5) + 1, 'A', 'B', 'C', 'D', 'E')
FROM (
    SELECT a.N + b.N * 10 + c.N * 100 AS n
    FROM (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
          UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) a
    CROSS JOIN (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
                UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) b
    CROSS JOIN (SELECT 0 AS N UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
                UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9) c
) seq
WHERE seq.n < 1000;

-- Copy data to indexed table
INSERT INTO indexed_table (data, status, category)
SELECT data, status, category FROM no_index_table;

-- Initialize lock test table
INSERT INTO lock_test_table (counter) VALUES (0);

-- Create stored procedures for testing
DELIMITER //

CREATE PROCEDURE IF NOT EXISTS generate_slow_query()
BEGIN
    -- Intentionally slow query for testing
    SELECT SLEEP(0.5), COUNT(*) 
    FROM no_index_table t1 
    CROSS JOIN no_index_table t2 
    WHERE t1.data LIKE '%test%' 
    LIMIT 1;
END//

CREATE PROCEDURE IF NOT EXISTS generate_table_scan()
BEGIN
    -- Query without index usage
    SELECT COUNT(*), category, status
    FROM no_index_table 
    WHERE data LIKE '%5%' 
    GROUP BY category, status
    ORDER BY COUNT(*) DESC;
END//

CREATE PROCEDURE IF NOT EXISTS generate_temp_table_query()
BEGIN
    -- Query that creates temp tables
    SELECT 
        t1.category,
        COUNT(DISTINCT t1.id) as unique_ids,
        GROUP_CONCAT(t1.data ORDER BY t1.created_at) as data_list,
        AVG(t2.status) as avg_status
    FROM no_index_table t1
    JOIN no_index_table t2 ON t1.category = t2.category
    GROUP BY t1.category
    HAVING COUNT(DISTINCT t1.id) > 10
    ORDER BY unique_ids DESC;
END//

CREATE PROCEDURE IF NOT EXISTS simulate_lock_contention()
BEGIN
    DECLARE i INT DEFAULT 0;
    START TRANSACTION;
    UPDATE lock_test_table SET counter = counter + 1 WHERE id = 1;
    -- Hold lock for a moment
    SELECT SLEEP(0.1);
    COMMIT;
END//

DELIMITER ;

-- Root user already has all necessary permissions by default

-- Output confirmation
SELECT 'SQL Intelligence initialization complete' as status;