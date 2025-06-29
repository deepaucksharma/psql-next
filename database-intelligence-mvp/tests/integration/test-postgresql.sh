#!/bin/bash
# Database Intelligence MVP - PostgreSQL Integration Tests
# Tests PostgreSQL plan collection and processing pipeline

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
TEST_DB_NAME="db_intelligence_test"
TEST_SCHEMA="test_schema"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test state
TESTS_PASSED=0
TESTS_FAILED=0
CLEANUP_REQUIRED=false

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_failure() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Cleanup function
cleanup() {
    if [[ "$CLEANUP_REQUIRED" == "true" ]] && [[ -n "${PG_TEST_DSN:-}" ]]; then
        log_info "Cleaning up test environment..."
        
        # Drop test schema and data
        psql "$PG_TEST_DSN" -c "DROP SCHEMA IF EXISTS $TEST_SCHEMA CASCADE;" 2>/dev/null || true
        
        # Reset pg_stat_statements
        psql "$PG_TEST_DSN" -c "SELECT pg_stat_statements_reset();" 2>/dev/null || true
        
        log_info "Cleanup completed"
    fi
}

# Set up cleanup trap
trap cleanup EXIT

# Check prerequisites
check_prerequisites() {
    log_info "Checking test prerequisites..."
    
    # Check if psql is available
    if ! command -v psql &> /dev/null; then
        log_failure "psql not available - install PostgreSQL client"
        exit 1
    fi
    
    # Check if test DSN is provided
    if [[ -z "${PG_TEST_DSN:-}" ]]; then
        log_failure "PG_TEST_DSN environment variable not set"
        log_info "Example: export PG_TEST_DSN='postgres://user:pass@localhost:5432/testdb'"
        exit 1
    fi
    
    # Test basic connectivity
    if ! psql "$PG_TEST_DSN" -c "SELECT 1;" &> /dev/null; then
        log_failure "Cannot connect to test database"
        log_info "Check your PG_TEST_DSN: ${PG_TEST_DSN%:*}:***@${PG_TEST_DSN##*@}"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Set up test environment
setup_test_environment() {
    log_info "Setting up test environment..."
    
    # Create test schema
    psql "$PG_TEST_DSN" -c "CREATE SCHEMA IF NOT EXISTS $TEST_SCHEMA;" || {
        log_failure "Failed to create test schema"
        return 1
    }
    
    # Create test tables
    psql "$PG_TEST_DSN" << EOF
CREATE TABLE IF NOT EXISTS $TEST_SCHEMA.users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS $TEST_SCHEMA.orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES $TEST_SCHEMA.users(id),
    amount DECIMAL(10,2),
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_email ON $TEST_SCHEMA.users(email);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON $TEST_SCHEMA.orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_date ON $TEST_SCHEMA.orders(order_date);
EOF
    
    # Insert test data
    psql "$PG_TEST_DSN" << EOF
INSERT INTO $TEST_SCHEMA.users (email, name) VALUES 
    ('test1@example.com', 'Test User 1'),
    ('test2@example.com', 'Test User 2'),
    ('test3@example.com', 'Test User 3')
ON CONFLICT (email) DO NOTHING;

INSERT INTO $TEST_SCHEMA.orders (user_id, amount) 
SELECT u.id, random() * 1000 
FROM $TEST_SCHEMA.users u, generate_series(1, 10) 
ON CONFLICT DO NOTHING;
EOF
    
    # Reset pg_stat_statements to start fresh
    psql "$PG_TEST_DSN" -c "SELECT pg_stat_statements_reset();" || {
        log_warning "Could not reset pg_stat_statements (may not be installed)"
    }
    
    CLEANUP_REQUIRED=true
    log_success "Test environment setup completed"
}

# Test pg_stat_statements availability
test_pg_stat_statements() {
    log_info "Testing pg_stat_statements availability..."
    
    # Check if extension exists
    local ext_exists
    ext_exists=$(psql "$PG_TEST_DSN" -t -c "SELECT count(*) FROM pg_extension WHERE extname = 'pg_stat_statements';" | xargs)
    
    if [[ "$ext_exists" -eq 0 ]]; then
        log_failure "pg_stat_statements extension not installed"
        log_info "Install with: CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
        return 1
    fi
    
    # Check if we can query it
    if psql "$PG_TEST_DSN" -c "SELECT count(*) FROM pg_stat_statements LIMIT 1;" &> /dev/null; then
        log_success "pg_stat_statements is accessible"
    else
        log_failure "Cannot access pg_stat_statements"
        return 1
    fi
    
    return 0
}

# Generate test workload
generate_test_workload() {
    log_info "Generating test workload..."
    
    # Execute various query patterns to populate pg_stat_statements
    local queries=(
        "SELECT count(*) FROM $TEST_SCHEMA.users;"
        "SELECT * FROM $TEST_SCHEMA.users WHERE email = 'test1@example.com';"
        "SELECT u.name, count(o.id) FROM $TEST_SCHEMA.users u LEFT JOIN $TEST_SCHEMA.orders o ON u.id = o.user_id GROUP BY u.id, u.name;"
        "SELECT * FROM $TEST_SCHEMA.orders WHERE amount > 500 ORDER BY order_date DESC;"
        "SELECT u.email FROM $TEST_SCHEMA.users u WHERE u.id IN (SELECT DISTINCT user_id FROM $TEST_SCHEMA.orders WHERE amount > 100);"
    )
    
    # Execute each query multiple times to accumulate statistics
    for query in "${queries[@]}"; do
        for i in {1..5}; do
            psql "$PG_TEST_DSN" -c "$query" &> /dev/null || true
        done
    done
    
    # Add some intentionally slow queries
    psql "$PG_TEST_DSN" -c "SELECT pg_sleep(0.1);" &> /dev/null || true
    psql "$PG_TEST_DSN" -c "SELECT count(*) FROM $TEST_SCHEMA.users u1, $TEST_SCHEMA.users u2 WHERE u1.id <> u2.id;" &> /dev/null || true
    
    log_success "Test workload generated"
}

# Test basic plan collection query
test_plan_collection_query() {
    log_info "Testing plan collection query..."
    
    # The actual query used by the collector
    local collector_query="
    SET LOCAL statement_timeout = '2000';
    SET LOCAL lock_timeout = '100';
    
    WITH worst_query AS (
      SELECT 
        queryid,
        query,
        mean_exec_time,
        calls,
        total_exec_time,
        (mean_exec_time * calls) as impact_score
      FROM pg_stat_statements
      WHERE 
        mean_exec_time > 10  -- Lowered threshold for testing
        AND calls > 1
        AND query NOT LIKE '%pg_%'
        AND query NOT LIKE '%EXPLAIN%'
        AND query NOT LIKE '%information_schema%'
        AND length(query) < 4000
      ORDER BY impact_score DESC
      LIMIT 1
    )
    SELECT
      w.queryid::text as query_id,
      w.query as query_text,
      w.mean_exec_time as avg_duration_ms,
      w.calls as execution_count,
      w.total_exec_time as total_duration_ms,
      w.impact_score::bigint as impact_score,
      now() as collection_timestamp,
      current_database() as database_name,
      version() as pg_version
    FROM worst_query w;"
    
    # Execute the query
    local result
    result=$(psql "$PG_TEST_DSN" -t -c "$collector_query" 2>&1)
    
    if [[ $? -eq 0 ]] && [[ -n "$result" ]]; then
        log_success "Plan collection query executed successfully"
        
        # Check if we got meaningful data
        local row_count
        row_count=$(echo "$result" | wc -l | xargs)
        if [[ "$row_count" -gt 0 ]]; then
            log_success "Query returned $row_count row(s) of data"
        else
            log_warning "Query executed but returned no data"
        fi
    else
        log_failure "Plan collection query failed: $result"
        return 1
    fi
    
    return 0
}

# Test EXPLAIN functionality
test_explain_functionality() {
    log_info "Testing EXPLAIN functionality..."
    
    # Test basic EXPLAIN
    local explain_query="EXPLAIN (FORMAT JSON) SELECT count(*) FROM $TEST_SCHEMA.users;"
    local explain_result
    explain_result=$(psql "$PG_TEST_DSN" -t -c "$explain_query" 2>&1)
    
    if [[ $? -eq 0 ]]; then
        log_success "Basic EXPLAIN works"
        
        # Validate JSON format
        if echo "$explain_result" | jq . &> /dev/null; then
            log_success "EXPLAIN returns valid JSON"
        else
            log_failure "EXPLAIN JSON is malformed"
            return 1
        fi
    else
        log_failure "EXPLAIN failed: $explain_result"
        return 1
    fi
    
    # Test EXPLAIN with timeout safety
    local safe_explain="
    SET LOCAL statement_timeout = '1000';
    EXPLAIN (FORMAT JSON) SELECT pg_sleep(0.1);"
    
    if psql "$PG_TEST_DSN" -c "$safe_explain" &> /dev/null; then
        log_success "EXPLAIN with timeout safety works"
    else
        log_failure "EXPLAIN timeout safety failed"
        return 1
    fi
    
    return 0
}

# Test query safety mechanisms
test_query_safety() {
    log_info "Testing query safety mechanisms..."
    
    # Test statement timeout
    log_info "Testing statement timeout..."
    local timeout_start timeout_end duration
    timeout_start=$(date +%s)
    
    # This should timeout quickly
    if ! psql "$PG_TEST_DSN" -c "SET LOCAL statement_timeout = '500'; SELECT pg_sleep(2);" &> /dev/null; then
        timeout_end=$(date +%s)
        duration=$((timeout_end - timeout_start))
        
        if [[ "$duration" -lt 3 ]]; then
            log_success "Statement timeout works (${duration}s)"
        else
            log_failure "Statement timeout too slow (${duration}s)"
            return 1
        fi
    else
        log_failure "Statement timeout not working"
        return 1
    fi
    
    # Test lock timeout
    log_info "Testing lock timeout..."
    if psql "$PG_TEST_DSN" -c "SET LOCAL lock_timeout = '100'; SELECT 1;" &> /dev/null; then
        log_success "Lock timeout setting works"
    else
        log_failure "Lock timeout setting failed"
        return 1
    fi
    
    return 0
}

# Test data collection patterns
test_data_collection_patterns() {
    log_info "Testing data collection patterns..."
    
    # Test that we can identify slow queries
    local slow_query_check="
    SELECT count(*) 
    FROM pg_stat_statements 
    WHERE mean_exec_time > 10 
      AND calls > 0;"
    
    local slow_count
    slow_count=$(psql "$PG_TEST_DSN" -t -c "$slow_query_check" | xargs)
    
    if [[ "$slow_count" -gt 0 ]]; then
        log_success "Found $slow_count potentially slow queries"
    else
        log_warning "No slow queries found (may be expected in test environment)"
    fi
    
    # Test that we can identify frequent queries
    local frequent_query_check="
    SELECT count(*) 
    FROM pg_stat_statements 
    WHERE calls > 3;"
    
    local frequent_count
    frequent_count=$(psql "$PG_TEST_DSN" -t -c "$frequent_query_check" | xargs)
    
    if [[ "$frequent_count" -gt 0 ]]; then
        log_success "Found $frequent_count frequently executed queries"
    else
        log_warning "No frequent queries found"
    fi
    
    return 0
}

# Test PII sanitization patterns
test_pii_sanitization() {
    log_info "Testing PII sanitization patterns..."
    
    # Create queries with PII-like content
    local pii_queries=(
        "SELECT * FROM $TEST_SCHEMA.users WHERE email = 'john.doe@company.com'"
        "SELECT * FROM $TEST_SCHEMA.users WHERE name = 'John Doe' AND id = 123456789"
        "SELECT * FROM $TEST_SCHEMA.orders WHERE amount = 1234.56"
    )
    
    # Execute PII-containing queries
    for query in "${pii_queries[@]}"; do
        psql "$PG_TEST_DSN" -c "$query" &> /dev/null || true
    done
    
    # Check if pg_stat_statements captured them
    local pii_captured
    pii_captured=$(psql "$PG_TEST_DSN" -t -c "
        SELECT count(*) 
        FROM pg_stat_statements 
        WHERE query LIKE '%@%' 
           OR query LIKE '%123456789%';" | xargs)
    
    if [[ "$pii_captured" -gt 0 ]]; then
        log_success "PII-containing queries captured for sanitization testing"
    else
        log_warning "No PII-containing queries found in pg_stat_statements"
    fi
    
    return 0
}

# Test performance characteristics
test_performance_characteristics() {
    log_info "Testing performance characteristics..."
    
    # Test query execution time
    local start_time end_time duration
    start_time=$(date +%s%N)
    
    psql "$PG_TEST_DSN" -c "SELECT count(*) FROM pg_stat_statements;" &> /dev/null
    
    end_time=$(date +%s%N)
    duration=$(((end_time - start_time) / 1000000)) # Convert to milliseconds
    
    if [[ "$duration" -lt 1000 ]]; then
        log_success "pg_stat_statements query performance acceptable (${duration}ms)"
    else
        log_warning "pg_stat_statements query slow (${duration}ms)"
    fi
    
    # Test concurrent connection handling
    log_info "Testing concurrent connections..."
    local connection_test="SELECT count(*) FROM pg_stat_activity WHERE usename = current_user;"
    
    # Execute multiple concurrent queries
    for i in {1..3}; do
        psql "$PG_TEST_DSN" -c "$connection_test" &> /dev/null &
    done
    wait
    
    log_success "Concurrent connection test completed"
    
    return 0
}

# Test replica safety
test_replica_safety() {
    log_info "Testing replica safety..."
    
    # Check if we're on a replica
    local is_replica
    is_replica=$(psql "$PG_TEST_DSN" -t -c "SELECT pg_is_in_recovery();" | xargs)
    
    if [[ "$is_replica" == "t" ]]; then
        log_success "Connected to read replica (safe for monitoring)"
        
        # Test replica lag if applicable
        local lag_query="SELECT EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp()));"
        local lag_result
        lag_result=$(psql "$PG_TEST_DSN" -t -c "$lag_query" 2>/dev/null | xargs || echo "N/A")
        
        if [[ "$lag_result" != "N/A" ]]; then
            local lag_seconds
            lag_seconds=$(echo "$lag_result" | cut -d. -f1)
            if [[ "$lag_seconds" -lt 60 ]]; then
                log_success "Replica lag acceptable (${lag_seconds}s)"
            else
                log_warning "High replica lag detected (${lag_seconds}s)"
            fi
        fi
    else
        log_warning "Connected to primary database - use replica in production"
    fi
    
    # Test read-only operations
    if psql "$PG_TEST_DSN" -c "SELECT 1;" &> /dev/null; then
        log_success "Read operations work correctly"
    else
        log_failure "Read operations failed"
        return 1
    fi
    
    return 0
}

# Generate test report
generate_test_report() {
    local total_tests=$((TESTS_PASSED + TESTS_FAILED))
    local pass_rate
    
    if [[ "$total_tests" -gt 0 ]]; then
        pass_rate=$(( (TESTS_PASSED * 100) / total_tests ))
    else
        pass_rate=0
    fi
    
    echo ""
    echo "=============================================="
    echo "PostgreSQL Integration Test Report"
    echo "=============================================="
    echo "Total Tests: $total_tests"
    echo "Passed: $TESTS_PASSED"
    echo "Failed: $TESTS_FAILED"
    echo "Pass Rate: ${pass_rate}%"
    echo ""
    
    if [[ "$TESTS_FAILED" -eq 0 ]]; then
        log_success "All PostgreSQL integration tests passed!"
        echo ""
        echo "Your PostgreSQL environment is ready for Database Intelligence MVP."
        echo ""
        echo "Next steps:"
        echo "1. Deploy the collector using the deployment scripts"
        echo "2. Monitor collector logs for successful data collection"
        echo "3. Verify data appears in New Relic"
        return 0
    else
        log_failure "Some PostgreSQL integration tests failed"
        echo ""
        echo "Please address the failed tests before deploying the collector."
        echo "Consult PREREQUISITES.md for setup requirements."
        return 1
    fi
}

# Main test execution
main() {
    echo "Database Intelligence MVP - PostgreSQL Integration Tests"
    echo "======================================================="
    echo ""
    
    # Load environment if available
    if [[ -f "$PROJECT_ROOT/.env" ]]; then
        # shellcheck source=/dev/null
        source "$PROJECT_ROOT/.env"
    fi
    
    # Run test suite
    check_prerequisites
    setup_test_environment
    
    echo ""
    echo "Running PostgreSQL integration tests..."
    echo ""
    
    test_pg_stat_statements
    generate_test_workload
    test_plan_collection_query
    test_explain_functionality
    test_query_safety
    test_data_collection_patterns
    test_pii_sanitization
    test_performance_characteristics
    test_replica_safety
    
    # Generate and show report
    generate_test_report
}

# Run main function
main "$@"