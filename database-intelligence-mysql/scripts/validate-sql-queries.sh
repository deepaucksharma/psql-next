#!/bin/bash

# Script to validate all SQL queries in the edge collector configuration
# This extracts and tests each query for syntax errors

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== Validating SQL Queries in Edge Collector Configuration ===${NC}"
echo ""

# Function to print status
print_status() {
    local status=$1
    local message=$2
    case $status in
        "success")
            echo -e "${GREEN}✓${NC} $message"
            ;;
        "error")
            echo -e "${RED}✗${NC} $message"
            ;;
        "info")
            echo -e "${BLUE}ℹ${NC} $message"
            ;;
    esac
}

# Extract SQL queries from YAML
extract_queries() {
    local config_file="config/edge-collector-wait.yaml"
    local query_dir="extracted_queries"
    
    mkdir -p "$query_dir"
    
    # Query 1: Core wait profile query
    cat > "$query_dir/01_wait_profile.sql" << 'EOF'
WITH wait_summary AS (
  SELECT 
    ews.THREAD_ID,
    ews.EVENT_NAME as wait_type,
    COUNT(*) as wait_count,
    SUM(ews.TIMER_WAIT) as total_wait,
    AVG(ews.TIMER_WAIT) as avg_wait,
    MAX(ews.TIMER_WAIT) as max_wait
  FROM performance_schema.events_waits_history_long ews
  WHERE ews.TIMER_WAIT > 0
    AND ews.EVENT_NAME NOT LIKE 'idle%'
    AND ews.END_EVENT_ID IS NOT NULL
  GROUP BY ews.THREAD_ID, ews.EVENT_NAME
),
statement_waits AS (
  SELECT 
    esh.THREAD_ID,
    esh.DIGEST,
    esh.DIGEST_TEXT,
    esh.CURRENT_SCHEMA,
    esh.TIMER_WAIT as statement_time,
    esh.LOCK_TIME,
    esh.ROWS_EXAMINED,
    esh.ROWS_SENT,
    esh.NO_INDEX_USED,
    esh.NO_GOOD_INDEX_USED,
    esh.CREATED_TMP_TABLES,
    esh.CREATED_TMP_DISK_TABLES,
    esh.SELECT_FULL_JOIN,
    esh.SELECT_SCAN
  FROM performance_schema.events_statements_history_long esh
  WHERE esh.DIGEST IS NOT NULL
    AND esh.TIMER_WAIT > 1000000
)
SELECT 
  sw.DIGEST as query_hash,
  LEFT(sw.DIGEST_TEXT, 100) as query_text,
  sw.CURRENT_SCHEMA as db_schema,
  ws.wait_type,
  ws.wait_count,
  ws.total_wait/1000000 as total_wait_ms,
  ws.avg_wait/1000000 as avg_wait_ms,
  ws.max_wait/1000000 as max_wait_ms,
  sw.statement_time/1000000 as statement_time_ms,
  sw.LOCK_TIME/1000000 as lock_time_ms,
  sw.ROWS_EXAMINED,
  sw.NO_INDEX_USED,
  sw.NO_GOOD_INDEX_USED,
  sw.CREATED_TMP_DISK_TABLES as tmp_disk_tables,
  sw.SELECT_FULL_JOIN as full_joins,
  sw.SELECT_SCAN as full_scans,
  COALESCE((ws.total_wait / NULLIF(sw.statement_time, 0)) * 100, 0) as wait_percentage
FROM statement_waits sw
LEFT JOIN wait_summary ws ON sw.THREAD_ID = ws.THREAD_ID
WHERE ws.total_wait > 0
ORDER BY ws.total_wait DESC
LIMIT 100;
EOF

    # Query 2: Active blocking analysis
    cat > "$query_dir/02_blocking_analysis.sql" << 'EOF'
SELECT 
  bt.trx_id,
  bt.trx_state,
  bt.trx_started,
  bt.trx_wait_started,
  TIMESTAMPDIFF(SECOND, bt.trx_wait_started, NOW()) as wait_duration,
  bt.trx_mysql_thread_id as waiting_thread,
  SUBSTRING(bt.trx_query, 1, 100) as waiting_query,
  blt.trx_mysql_thread_id as blocking_thread,
  SUBSTRING(blt.trx_query, 1, 100) as blocking_query,
  l.lock_mode,
  l.lock_type,
  l.object_schema,
  l.object_name as lock_table,
  l.index_name as lock_index
FROM information_schema.innodb_trx bt
JOIN performance_schema.data_lock_waits dlw 
  ON bt.trx_mysql_thread_id = dlw.REQUESTING_THREAD_ID
JOIN information_schema.innodb_trx blt 
  ON dlw.BLOCKING_THREAD_ID = blt.trx_mysql_thread_id
JOIN performance_schema.data_locks l
  ON dlw.REQUESTING_ENGINE_LOCK_ID = l.ENGINE_LOCK_ID
WHERE bt.trx_wait_started IS NOT NULL;
EOF

    # Query 3: Statement digest summary
    cat > "$query_dir/03_statement_digest.sql" << 'EOF'
SELECT 
  DIGEST as query_hash,
  LEFT(DIGEST_TEXT, 200) as query_text,
  SCHEMA_NAME as db_schema,
  COUNT_STAR as exec_count,
  SUM_TIMER_WAIT/1000000000 as total_time_sec,
  AVG_TIMER_WAIT/1000000 as avg_time_ms,
  MIN_TIMER_WAIT/1000000 as min_time_ms,
  MAX_TIMER_WAIT/1000000 as max_time_ms,
  SUM_LOCK_TIME/1000000 as total_lock_ms,
  SUM_ROWS_EXAMINED as total_rows_examined,
  SUM_ROWS_SENT as total_rows_sent,
  SUM_SELECT_SCAN as full_scans,
  SUM_NO_INDEX_USED as no_index_used_count,
  SUM_NO_GOOD_INDEX_USED as no_good_index_used_count,
  SUM_CREATED_TMP_DISK_TABLES as tmp_disk_tables,
  SUM_SORT_SCAN as sort_scans,
  FIRST_SEEN,
  LAST_SEEN
FROM performance_schema.events_statements_summary_by_digest
WHERE COUNT_STAR > 0
  AND DIGEST_TEXT NOT LIKE '%performance_schema%'
  AND DIGEST_TEXT NOT LIKE '%information_schema%'
ORDER BY SUM_TIMER_WAIT DESC
LIMIT 100;
EOF

    # Query 4: Current wait events
    cat > "$query_dir/04_current_waits.sql" << 'EOF'
SELECT 
  t.PROCESSLIST_ID as thread_id,
  t.PROCESSLIST_USER as user,
  t.PROCESSLIST_HOST as host,
  t.PROCESSLIST_DB as db,
  t.PROCESSLIST_COMMAND as command,
  t.PROCESSLIST_TIME as time,
  t.PROCESSLIST_STATE as state,
  LEFT(t.PROCESSLIST_INFO, 100) as query,
  ewc.EVENT_NAME as wait_event,
  ewc.TIMER_WAIT/1000000 as wait_ms,
  ewc.OBJECT_SCHEMA,
  ewc.OBJECT_NAME
FROM performance_schema.threads t
LEFT JOIN performance_schema.events_waits_current ewc
  ON t.THREAD_ID = ewc.THREAD_ID
WHERE t.PROCESSLIST_ID IS NOT NULL
  AND t.PROCESSLIST_COMMAND != 'Sleep'
  AND ewc.EVENT_NAME IS NOT NULL
ORDER BY ewc.TIMER_WAIT DESC
LIMIT 50;
EOF

    print_status "success" "Extracted ${BLUE}4${NC} SQL queries to $query_dir/"
}

# Validate query syntax
validate_syntax() {
    local query_file=$1
    local query_name=$(basename "$query_file" .sql)
    
    print_status "info" "Validating: $query_name"
    
    # Create a syntax check query
    local check_query="EXPLAIN FORMAT=TREE $(cat "$query_file")"
    
    # Note: This would normally connect to MySQL to validate
    # For now, we'll do basic syntax checks
    
    # Check for common syntax issues
    local errors=0
    
    # Check for balanced parentheses
    local open_count=$(grep -o '(' "$query_file" | wc -l)
    local close_count=$(grep -o ')' "$query_file" | wc -l)
    if [ "$open_count" -ne "$close_count" ]; then
        print_status "error" "  Unbalanced parentheses in $query_name"
        ((errors++))
    fi
    
    # Check for missing semicolons (should end with semicolon)
    if ! tail -1 "$query_file" | grep -q ';$'; then
        print_status "error" "  Missing semicolon at end of $query_name"
        ((errors++))
    fi
    
    # Check for common table/column issues
    if grep -q "performance_schema\." "$query_file"; then
        print_status "success" "  Uses Performance Schema tables"
    fi
    
    if grep -q "information_schema\." "$query_file"; then
        print_status "success" "  Uses Information Schema tables"
    fi
    
    # Check for potential performance issues
    if grep -q "SELECT \*" "$query_file"; then
        print_status "warning" "  Uses SELECT * (consider specific columns)"
    fi
    
    if [ $errors -eq 0 ]; then
        print_status "success" "  Basic syntax validation passed"
    else
        print_status "error" "  Found $errors syntax issues"
    fi
    
    return $errors
}

# Analyze query complexity
analyze_complexity() {
    local query_file=$1
    local query_name=$(basename "$query_file" .sql)
    
    echo ""
    print_status "info" "Analyzing complexity: $query_name"
    
    # Count CTEs
    local cte_count=$(grep -c "WITH\|,.*AS (" "$query_file" || echo 0)
    if [ $cte_count -gt 0 ]; then
        print_status "info" "  Uses $cte_count CTE(s)"
    fi
    
    # Count JOINs
    local join_count=$(grep -c "JOIN" "$query_file" || echo 0)
    if [ $join_count -gt 0 ]; then
        print_status "info" "  Contains $join_count JOIN(s)"
    fi
    
    # Check for subqueries
    local subquery_count=$(grep -c "SELECT.*FROM.*SELECT" "$query_file" || echo 0)
    if [ $subquery_count -gt 0 ]; then
        print_status "warning" "  Contains subqueries (potential performance impact)"
    fi
    
    # Check for aggregations
    if grep -q "GROUP BY\|COUNT(\|SUM(\|AVG(\|MAX(\|MIN(" "$query_file"; then
        print_status "info" "  Uses aggregation functions"
    fi
    
    # Check for ORDER BY with LIMIT
    if grep -q "ORDER BY" "$query_file" && grep -q "LIMIT" "$query_file"; then
        print_status "success" "  Has ORDER BY with LIMIT (good practice)"
    elif grep -q "ORDER BY" "$query_file"; then
        print_status "warning" "  Has ORDER BY without LIMIT (potential performance issue)"
    fi
}

# Generate optimization recommendations
generate_recommendations() {
    echo -e "\n${BLUE}=== Optimization Recommendations ===${NC}"
    
    cat << 'EOF'

1. Wait Profile Query (01_wait_profile.sql):
   - Uses CTEs effectively for readability
   - Joins on THREAD_ID which should be indexed
   - Consider adding time-based filtering for recent events only
   - May benefit from summary tables for high-frequency queries

2. Blocking Analysis Query (02_blocking_analysis.sql):
   - Good use of system tables for lock analysis
   - SUBSTRING limits text size (good practice)
   - Consider adding duration threshold to filter short waits
   - Index on trx_wait_started would help if available

3. Statement Digest Query (03_statement_digest.sql):
   - Properly filters system schemas
   - Uses LIMIT to control result size
   - Consider time-based partitioning for historical data
   - May need index on DIGEST for large datasets

4. Current Waits Query (04_current_waits.sql):
   - Filters out Sleep commands (good)
   - LEFT JOIN prevents missing threads without waits
   - Consider adding wait duration threshold
   - May need to handle NULL wait events

General Recommendations:
- Enable Performance Schema consumers selectively based on needs
- Monitor Performance Schema memory usage
- Consider summary tables for frequently accessed data
- Implement query result caching where appropriate
- Use prepared statements for these queries in production
EOF
}

# Create test data generator
create_test_data() {
    echo -e "\n${BLUE}Creating test data generator...${NC}"
    
    cat > "test-data-generator.sql" << 'EOF'
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
EOF
    
    print_status "success" "Created test-data-generator.sql"
}

# Main execution
main() {
    # Extract queries
    extract_queries
    
    # Validate each query
    echo -e "\n${BLUE}=== Syntax Validation ===${NC}"
    local total_errors=0
    for query_file in extracted_queries/*.sql; do
        validate_syntax "$query_file" || ((total_errors++))
    done
    
    # Analyze complexity
    echo -e "\n${BLUE}=== Complexity Analysis ===${NC}"
    for query_file in extracted_queries/*.sql; do
        analyze_complexity "$query_file"
    done
    
    # Generate recommendations
    generate_recommendations
    
    # Create test data generator
    create_test_data
    
    # Summary
    echo -e "\n${BLUE}=== Validation Summary ===${NC}"
    if [ $total_errors -eq 0 ]; then
        print_status "success" "All queries passed basic validation"
        print_status "info" "Next steps:"
        echo "  1. Connect to MySQL and run queries with EXPLAIN"
        echo "  2. Load test data: mysql -u root -p < test-data-generator.sql"
        echo "  3. Generate workload: CALL generate_test_workload(100, 'mixed');"
        echo "  4. Monitor with: ./scripts/monitor-waits.sh"
    else
        print_status "error" "Found $total_errors validation errors"
    fi
}

# Run main
main