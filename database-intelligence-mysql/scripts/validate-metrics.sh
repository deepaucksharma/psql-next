#!/bin/bash

# Validate MySQL metrics collection
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== MySQL Metrics Validation ==="

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090/metrics}"
MYSQL_HOST="${MYSQL_HOST:-localhost}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-rootpassword}"

# Function to check if a metric exists
check_metric() {
    local metric_name=$1
    local description=$2
    
    if curl -s "$PROMETHEUS_URL" | grep -q "^$metric_name"; then
        echo -e "${GREEN}✓${NC} $description: $metric_name"
        return 0
    else
        echo -e "${RED}✗${NC} $description: $metric_name"
        return 1
    fi
}

# Function to get metric value
get_metric_value() {
    local metric_name=$1
    curl -s "$PROMETHEUS_URL" | grep "^$metric_name" | awk '{print $2}' | head -1
}

# Function to validate metric value
validate_metric_value() {
    local metric_name=$1
    local expected_op=$2  # gt, lt, eq, ge, le
    local expected_value=$3
    local description=$4
    
    local actual_value=$(get_metric_value "$metric_name")
    
    if [ -z "$actual_value" ]; then
        echo -e "${RED}✗${NC} $description: metric not found"
        return 1
    fi
    
    case $expected_op in
        "gt")
            if (( $(echo "$actual_value > $expected_value" | bc -l) )); then
                echo -e "${GREEN}✓${NC} $description: $actual_value > $expected_value"
                return 0
            fi
            ;;
        "ge")
            if (( $(echo "$actual_value >= $expected_value" | bc -l) )); then
                echo -e "${GREEN}✓${NC} $description: $actual_value >= $expected_value"
                return 0
            fi
            ;;
        "lt")
            if (( $(echo "$actual_value < $expected_value" | bc -l) )); then
                echo -e "${GREEN}✓${NC} $description: $actual_value < $expected_value"
                return 0
            fi
            ;;
        "le")
            if (( $(echo "$actual_value <= $expected_value" | bc -l) )); then
                echo -e "${GREEN}✓${NC} $description: $actual_value <= $expected_value"
                return 0
            fi
            ;;
        "eq")
            if (( $(echo "$actual_value == $expected_value" | bc -l) )); then
                echo -e "${GREEN}✓${NC} $description: $actual_value == $expected_value"
                return 0
            fi
            ;;
    esac
    
    echo -e "${RED}✗${NC} $description: $actual_value (expected $expected_op $expected_value)"
    return 1
}

# Function to run MySQL query
mysql_query() {
    local query=$1
    mysql -h "$MYSQL_HOST" -P "$MYSQL_PORT" -u "$MYSQL_USER" -p"$MYSQL_PASSWORD" -sN -e "$query" 2>/dev/null
}

# Main validation
main() {
    local failed=0
    
    echo -e "\n${YELLOW}Checking prerequisites...${NC}"
    
    # Check if Prometheus endpoint is accessible
    if ! curl -s "$PROMETHEUS_URL" > /dev/null; then
        echo -e "${RED}✗${NC} Prometheus endpoint not accessible at $PROMETHEUS_URL"
        exit 1
    fi
    echo -e "${GREEN}✓${NC} Prometheus endpoint is accessible"
    
    # Check MySQL connection
    if ! mysql_query "SELECT 1" > /dev/null; then
        echo -e "${RED}✗${NC} Cannot connect to MySQL"
        exit 1
    fi
    echo -e "${GREEN}✓${NC} MySQL connection successful"
    
    echo -e "\n${YELLOW}Validating core MySQL metrics...${NC}"
    
    # Check connection metrics
    check_metric "mysql_connections_active" "Active connections" || ((failed++))
    check_metric "mysql_connections_total" "Total connections" || ((failed++))
    check_metric "mysql_connections_aborted_total" "Aborted connections" || ((failed++))
    
    # Check query metrics
    check_metric "mysql_queries_total" "Total queries" || ((failed++))
    check_metric "mysql_slow_queries_total" "Slow queries" || ((failed++))
    check_metric "mysql_questions_total" "Questions" || ((failed++))
    
    # Check InnoDB metrics
    echo -e "\n${YELLOW}Validating InnoDB metrics...${NC}"
    check_metric "mysql_buffer_pool_usage" "Buffer pool usage" || ((failed++))
    check_metric "mysql_buffer_pool_pages_total" "Buffer pool pages" || ((failed++))
    check_metric "mysql_innodb_row_operations_total" "Row operations" || ((failed++))
    check_metric "mysql_innodb_row_lock_waits_total" "Row lock waits" || ((failed++))
    
    # Check Performance Schema metrics
    echo -e "\n${YELLOW}Validating Performance Schema metrics...${NC}"
    check_metric "mysql_statement_event_count_total" "Statement events" || ((failed++))
    check_metric "mysql_statement_event_wait_time_total" "Statement wait time" || ((failed++))
    check_metric "mysql_table_io_wait_count_total" "Table I/O waits" || ((failed++))
    check_metric "mysql_table_io_wait_time_total" "Table I/O wait time" || ((failed++))
    
    # Validate metric values
    echo -e "\n${YELLOW}Validating metric values...${NC}"
    
    # Get actual connection count from MySQL
    local actual_connections=$(mysql_query "SELECT COUNT(*) FROM information_schema.processlist")
    validate_metric_value "mysql_connections_active" "ge" "1" "Active connections should be >= 1" || ((failed++))
    
    # Queries should be increasing
    validate_metric_value "mysql_queries_total" "gt" "0" "Total queries should be > 0" || ((failed++))
    
    # Buffer pool usage should be reasonable
    validate_metric_value "mysql_buffer_pool_usage" "ge" "0" "Buffer pool usage should be >= 0" || ((failed++))
    validate_metric_value "mysql_buffer_pool_usage" "le" "1" "Buffer pool usage should be <= 1" || ((failed++))
    
    # Check replication metrics if replica is configured
    if [ -n "$MYSQL_REPLICA_HOST" ]; then
        echo -e "\n${YELLOW}Validating replication metrics...${NC}"
        check_metric "mysql_replica_time_behind_source_seconds" "Replica lag" || ((failed++))
        check_metric "mysql_replica_sql_delay_seconds" "SQL delay" || ((failed++))
        
        # Replica lag should be low
        validate_metric_value "mysql_replica_time_behind_source_seconds" "lt" "5" "Replica lag should be < 5 seconds" || ((failed++))
    fi
    
    # Summary
    echo -e "\n${YELLOW}Validation Summary${NC}"
    echo "==================="
    if [ $failed -eq 0 ]; then
        echo -e "${GREEN}✓ All validations passed!${NC}"
        exit 0
    else
        echo -e "${RED}✗ $failed validations failed${NC}"
        exit 1
    fi
}

# Run main function
main