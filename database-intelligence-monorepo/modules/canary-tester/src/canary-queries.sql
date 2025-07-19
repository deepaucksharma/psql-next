-- Canary Test Queries for MySQL Synthetic Monitoring
-- These queries test various aspects of database availability and performance

-- 1. Basic connectivity test
-- Name: ping_test
-- Description: Simple query to verify database connectivity
-- SLI: availability
-- Expected: Should always return 1
SELECT 1 AS ping;

-- 2. Current timestamp test
-- Name: timestamp_test
-- Description: Verify database can return current time
-- SLI: availability, clock_sync
-- Expected: Should return current timestamp
SELECT NOW() AS current_time, CONNECTION_ID() AS connection_id;

-- 3. Simple table operations
-- Name: create_canary_table
-- Description: Create a canary test table if not exists
-- SLI: ddl_availability
CREATE TABLE IF NOT EXISTS canary_test (
    id INT AUTO_INCREMENT PRIMARY KEY,
    test_name VARCHAR(100),
    test_value VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_created_at (created_at)
);

-- 4. Insert performance test
-- Name: insert_test
-- Description: Test insert operation performance
-- SLI: write_latency, write_availability
INSERT INTO canary_test (test_name, test_value) 
VALUES ('canary_insert', UUID());

-- 5. Select performance test
-- Name: select_recent_test
-- Description: Test select operation on recent data
-- SLI: read_latency, read_availability
SELECT COUNT(*) AS record_count, 
       MAX(created_at) AS latest_record,
       MIN(created_at) AS oldest_record
FROM canary_test 
WHERE created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR);

-- 6. Join performance test
-- Name: self_join_test
-- Description: Test join operation performance
-- SLI: complex_query_latency
SELECT t1.id, t1.test_name, t2.test_value
FROM canary_test t1
JOIN canary_test t2 ON t1.id = t2.id
WHERE t1.created_at > DATE_SUB(NOW(), INTERVAL 5 MINUTE)
LIMIT 10;

-- 7. Aggregation test
-- Name: aggregation_test
-- Description: Test aggregation functions
-- SLI: aggregation_latency
SELECT test_name,
       COUNT(*) AS count,
       MIN(created_at) AS first_seen,
       MAX(created_at) AS last_seen
FROM canary_test
WHERE created_at > DATE_SUB(NOW(), INTERVAL 1 HOUR)
GROUP BY test_name;

-- 8. Transaction test
-- Name: transaction_test
-- Description: Test transaction commit performance
-- SLI: transaction_latency
START TRANSACTION;
INSERT INTO canary_test (test_name, test_value) VALUES ('transaction_test', 'start');
UPDATE canary_test SET test_value = 'updated' WHERE test_name = 'transaction_test' AND created_at > DATE_SUB(NOW(), INTERVAL 1 MINUTE);
COMMIT;

-- 9. Index performance test
-- Name: index_scan_test
-- Description: Test index scan performance
-- SLI: index_scan_latency
SELECT id, test_name, created_at 
FROM canary_test 
WHERE created_at BETWEEN DATE_SUB(NOW(), INTERVAL 30 MINUTE) AND NOW()
ORDER BY created_at DESC
LIMIT 100;

-- 10. Database metadata test
-- Name: metadata_test
-- Description: Test ability to query database metadata
-- SLI: metadata_availability
SELECT 
    TABLE_SCHEMA,
    TABLE_NAME,
    TABLE_ROWS,
    DATA_LENGTH,
    INDEX_LENGTH,
    CREATE_TIME
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE()
AND TABLE_NAME = 'canary_test';

-- 11. Connection pool test
-- Name: connection_test
-- Description: Test connection handling
-- SLI: connection_availability
SELECT 
    ID,
    USER,
    HOST,
    DB,
    COMMAND,
    TIME,
    STATE
FROM information_schema.PROCESSLIST
WHERE DB = DATABASE()
LIMIT 10;

-- 12. Cleanup old canary data
-- Name: cleanup_test
-- Description: Remove old canary test data to prevent table growth
-- SLI: maintenance_operations
DELETE FROM canary_test 
WHERE created_at < DATE_SUB(NOW(), INTERVAL 24 HOUR)
LIMIT 1000;

-- 13. Table statistics update
-- Name: analyze_table_test
-- Description: Update table statistics for query optimizer
-- SLI: maintenance_latency
ANALYZE TABLE canary_test;

-- 14. Lock wait test
-- Name: lock_wait_test
-- Description: Test for lock wait timeout (should be fast)
-- SLI: lock_wait_time
SELECT GET_LOCK('canary_test_lock', 1) AS lock_acquired,
       RELEASE_LOCK('canary_test_lock') AS lock_released;

-- 15. Full table scan test (small table)
-- Name: full_scan_test
-- Description: Controlled full table scan test
-- SLI: full_scan_latency
SELECT COUNT(DISTINCT test_name) AS unique_tests,
       AVG(LENGTH(test_value)) AS avg_value_length
FROM canary_test;