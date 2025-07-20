#!/bin/bash

# SQL Intelligence - Test Load Generation Script
# Generates diverse query patterns to test intelligence features

set -e

echo "========================================="
echo "SQL Intelligence Test Load Generator"
echo "========================================="

# Configuration
MYSQL_HOST="${MYSQL_HOST:-localhost}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-test}"

# Function to execute MySQL commands
mysql_exec() {
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" -e "$1" 2>/dev/null
}

# Function to execute MySQL commands on specific database
mysql_exec_db() {
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" "$1" -e "$2" 2>/dev/null
}

echo ""
echo "1. Verifying MySQL connection..."
if mysql_exec "SELECT 1" > /dev/null; then
    echo "✓ MySQL connection successful"
else
    echo "✗ Failed to connect to MySQL"
    exit 1
fi

echo ""
echo "2. Generating query patterns..."

# Pattern 1: Queries without indexes (should trigger index recommendations)
echo "   - Generating table scan queries..."
for i in {1..5}; do
    mysql_exec_db test_intelligence "
        SELECT COUNT(*), data 
        FROM no_index_table 
        WHERE data LIKE '%pattern$i%' 
        GROUP BY data
        LIMIT 10;
    "
done

# Pattern 2: Inefficient joins (should trigger optimization recommendations)
echo "   - Generating inefficient join queries..."
mysql_exec_db test_intelligence "
    SELECT t1.id, t1.data, t2.status, t2.category
    FROM no_index_table t1
    CROSS JOIN no_index_table t2
    WHERE t1.data = t2.data
    AND t1.status > 5
    LIMIT 100;
"

# Pattern 3: Queries creating temp tables on disk
echo "   - Generating temp table queries..."
mysql_exec_db test_intelligence "
    SELECT 
        category,
        status,
        COUNT(*) as cnt,
        GROUP_CONCAT(data ORDER BY created_at) as data_list,
        AVG(LENGTH(data)) as avg_length,
        STD(status) as status_std
    FROM no_index_table
    GROUP BY category, status
    HAVING cnt > 50
    ORDER BY cnt DESC, avg_length DESC;
"

# Pattern 4: Slow queries using stored procedures
echo "   - Generating slow queries..."
for i in {1..3}; do
    mysql_exec_db test_intelligence "CALL generate_slow_query();"
done

# Pattern 5: Full table scans
echo "   - Generating full table scans..."
mysql_exec_db test_intelligence "CALL generate_table_scan();"

# Pattern 6: Complex aggregations with temp tables
echo "   - Generating complex aggregation queries..."
mysql_exec_db test_intelligence "CALL generate_temp_table_query();"

# Pattern 7: Lock contention simulation
echo "   - Simulating lock contention..."
for i in {1..5}; do
    mysql_exec_db test_intelligence "CALL simulate_lock_contention();" &
done
wait

# Pattern 8: Subqueries (should be identified as complex)
echo "   - Generating subquery patterns..."
mysql_exec_db test_intelligence "
    SELECT 
        t1.category,
        t1.data,
        (SELECT COUNT(*) FROM no_index_table t2 
         WHERE t2.category = t1.category AND t2.status > t1.status) as higher_status_count,
        (SELECT AVG(status) FROM no_index_table t3 
         WHERE t3.category = t1.category) as category_avg_status
    FROM no_index_table t1
    WHERE t1.status IN (
        SELECT DISTINCT status 
        FROM no_index_table 
        WHERE category = 'A' 
        ORDER BY status DESC 
        LIMIT 5
    )
    LIMIT 50;
"

# Pattern 9: Queries with sorting (should show sort operations)
echo "   - Generating sort-heavy queries..."
mysql_exec_db test_intelligence "
    SELECT data, status, category, created_at
    FROM no_index_table
    WHERE status BETWEEN 3 AND 7
    ORDER BY LENGTH(data) DESC, status ASC, created_at DESC
    LIMIT 100;
"

# Pattern 10: Well-optimized queries for comparison
echo "   - Generating optimized queries..."
for i in {1..10}; do
    mysql_exec_db test_intelligence "
        SELECT id, data, status, category
        FROM indexed_table
        WHERE status = $i
        ORDER BY id
        LIMIT 10;
    "
done

# Pattern 11: Mixed read/write workload
echo "   - Generating mixed workload..."
for i in {1..5}; do
    # Read
    mysql_exec_db test_intelligence "
        SELECT COUNT(*) FROM indexed_table WHERE category = 'B';
    "
    # Write
    mysql_exec_db test_intelligence "
        UPDATE lock_test_table SET counter = counter + 1 WHERE id = 1;
    "
done

# Pattern 12: Index effectiveness test
echo "   - Testing index effectiveness..."
mysql_exec_db test_intelligence "
    -- Should use index
    SELECT * FROM indexed_table WHERE status = 5 AND category = 'C';
    
    -- Should not use index effectively
    SELECT * FROM indexed_table WHERE data LIKE '%test%';
    
    -- Partial index usage
    SELECT * FROM indexed_table WHERE status IN (1,2,3,4,5) AND data LIKE 'data-1%';
"

echo ""
echo "3. Generating current load statistics..."
mysql_exec "
    SELECT 
        'Active Queries' as metric,
        COUNT(*) as value
    FROM information_schema.PROCESSLIST
    WHERE COMMAND != 'Sleep'
    
    UNION ALL
    
    SELECT 
        'Total Statements Captured' as metric,
        COUNT(*) as value
    FROM performance_schema.events_statements_summary_by_digest
    WHERE SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
    
    UNION ALL
    
    SELECT 
        'Slow Queries (>1s)' as metric,
        COUNT(*) as value
    FROM performance_schema.events_statements_summary_by_digest
    WHERE SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
        AND AVG_TIMER_WAIT > 1000000000;
"

echo ""
echo "========================================="
echo "✓ Test load generation complete!"
echo "========================================="
echo ""
echo "The SQL Intelligence module should now be collecting:"
echo "- Query cost scores and optimization recommendations"
echo "- Index effectiveness metrics"
echo "- Lock contention analysis"
echo "- Table access patterns"
echo "- Business impact scores"
echo ""
echo "Check metrics at: http://localhost:8082/metrics"