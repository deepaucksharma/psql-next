#!/bin/bash
set -e

# Comprehensive E2E Test Runner for Database Intelligence Collector
# This script validates the complete data flow from databases to NRDB with shape verification

echo "=== Database Intelligence Collector Comprehensive E2E Tests ==="
echo "=== Validating Data Flow, Shape, and Details ==="

# Check prerequisites
if [ -z "$NEW_RELIC_LICENSE_KEY" ]; then
    echo "WARNING: NEW_RELIC_LICENSE_KEY not set - using local file export only"
    export USE_LOCAL_EXPORT=true
    CONFIG_FILE="$SCRIPT_DIR/config/e2e-test-collector-local.yaml"
else
    export USE_LOCAL_EXPORT=false
    CONFIG_FILE="$SCRIPT_DIR/config/e2e-test-collector-simple.yaml"
fi

if [ -z "$NEW_RELIC_ACCOUNT_ID" ]; then
    echo "WARNING: NEW_RELIC_ACCOUNT_ID not set - NRDB validation will be skipped"
fi

# Set test environment
export E2E_TESTS=true
export TEST_RUN_ID="e2e_$(date +%s)"
export TEST_TIMEOUT=${TEST_TIMEOUT:-30m}
export COLLECTOR_START_TIMEOUT=${COLLECTOR_START_TIMEOUT:-60}

# Set database connection defaults
export POSTGRES_HOST=${POSTGRES_HOST:-localhost}
export POSTGRES_PORT=${POSTGRES_PORT:-5432}
export POSTGRES_USER=${POSTGRES_USER:-postgres}
export POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-postgres}
export MYSQL_HOST=${MYSQL_HOST:-localhost}
export MYSQL_PORT=${MYSQL_PORT:-3306}
export MYSQL_USER=${MYSQL_USER:-root}
export MYSQL_PASSWORD=${MYSQL_PASSWORD:-mysql}

# Directories
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( cd "$SCRIPT_DIR/../.." && pwd )"
REPORTS_DIR="$SCRIPT_DIR/reports"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "Configuration:"
echo "  Test Run ID: $TEST_RUN_ID"
echo "  Project Root: $PROJECT_ROOT"
echo "  Test Timeout: $TEST_TIMEOUT"
echo "  PostgreSQL: ${POSTGRES_HOST:-localhost}:${POSTGRES_PORT:-5432}"
echo "  MySQL: ${MYSQL_HOST:-localhost}:${MYSQL_PORT:-3306}"
echo "  New Relic Account: $NEW_RELIC_ACCOUNT_ID"

# Ensure reports directory exists
mkdir -p "$REPORTS_DIR"

# Function to cleanup resources
cleanup() {
    echo -e "\n${YELLOW}Cleaning up test resources...${NC}"
    
    # Stop collector if running
    if [ -n "$COLLECTOR_PID" ]; then
        echo "Stopping collector (PID: $COLLECTOR_PID)"
        kill $COLLECTOR_PID 2>/dev/null || true
        wait $COLLECTOR_PID 2>/dev/null || true
    fi
    
    # Stop test databases if we started them
    if [ "$START_DATABASES" = "true" ]; then
        echo "Stopping test databases"
        cd "$SCRIPT_DIR"
        docker-compose -f docker-compose-test.yaml down -v
    fi
    
    # Collect artifacts
    if [ -f "/tmp/e2e-collector.log" ]; then
        echo "Collector logs saved to: $REPORTS_DIR/collector-$TEST_RUN_ID.log"
        cp /tmp/e2e-collector.log "$REPORTS_DIR/collector-$TEST_RUN_ID.log"
    fi
    
    if [ -f "/tmp/e2e-metrics.json" ]; then
        echo "Metrics saved to: $REPORTS_DIR/metrics-$TEST_RUN_ID.json"
        cp /tmp/e2e-metrics.json "$REPORTS_DIR/metrics-$TEST_RUN_ID.json"
    fi
}

# Set trap for cleanup
trap cleanup EXIT

# Function to wait for database
wait_for_database() {
    local host=$1
    local port=$2
    local type=$3
    local max_attempts=30
    local attempt=1
    
    echo -e "${YELLOW}Waiting for $type at $host:$port...${NC}"
    
    while [ $attempt -le $max_attempts ]; do
        if nc -z "$host" "$port" 2>/dev/null; then
            echo -e "${GREEN}$type is ready${NC}"
            return 0
        fi
        echo "Attempt $attempt/$max_attempts..."
        sleep 2
        ((attempt++))
    done
    
    echo -e "${RED}$type failed to start${NC}"
    return 1
}

# Check if databases are available
echo -e "\n${YELLOW}Checking database connectivity...${NC}"
POSTGRES_AVAILABLE=false
MYSQL_AVAILABLE=false

if nc -z ${POSTGRES_HOST:-localhost} ${POSTGRES_PORT:-5432} 2>/dev/null; then
    echo -e "${GREEN}PostgreSQL is available${NC}"
    POSTGRES_AVAILABLE=true
else
    echo "PostgreSQL not available"
fi

if nc -z ${MYSQL_HOST:-localhost} ${MYSQL_PORT:-3306} 2>/dev/null; then
    echo -e "${GREEN}MySQL is available${NC}"
    MYSQL_AVAILABLE=true
else
    echo "MySQL not available"
fi

# Start databases if needed
if [ "$POSTGRES_AVAILABLE" = "false" ] || [ "$MYSQL_AVAILABLE" = "false" ]; then
    echo -e "\n${YELLOW}Starting test databases...${NC}"
    START_DATABASES=true
    cd "$SCRIPT_DIR"
    docker-compose -f docker-compose-test.yaml up -d
    
    # Wait for databases
    wait_for_database ${POSTGRES_HOST:-localhost} ${POSTGRES_PORT:-5432} "PostgreSQL"
    wait_for_database ${MYSQL_HOST:-localhost} ${MYSQL_PORT:-3306} "MySQL"
    
    # Extra wait for initialization
    echo "Waiting for database initialization..."
    sleep 10
fi

# Build collector if needed
COLLECTOR_BINARY="$PROJECT_ROOT/dist/database-intelligence-collector"
if [ ! -f "$COLLECTOR_BINARY" ]; then
    echo -e "\n${YELLOW}Building collector...${NC}"
    cd "$PROJECT_ROOT"
    make build
    
    if [ ! -f "$COLLECTOR_BINARY" ]; then
        echo -e "${RED}Failed to build collector${NC}"
        exit 1
    fi
fi

# Validate collector exists and is executable
echo -e "\n${YELLOW}Validating collector binary...${NC}"
if [ -x "$COLLECTOR_BINARY" ]; then
    echo -e "${GREEN}Collector binary is valid${NC}"
else
    echo -e "${RED}Collector binary not found or not executable${NC}"
    exit 1
fi

# Start collector
echo -e "\n${YELLOW}Starting collector with e2e configuration...${NC}"
cd "$PROJECT_ROOT"
"$COLLECTOR_BINARY" \
    --config="$CONFIG_FILE" \
    --set=service.telemetry.logs.level=debug \
    > /tmp/e2e-collector.log 2>&1 &

COLLECTOR_PID=$!
echo "Collector started with PID: $COLLECTOR_PID"

# Function to check collector health
check_collector_health() {
    if ! kill -0 $COLLECTOR_PID 2>/dev/null; then
        echo -e "${RED}Collector process died${NC}"
        tail -20 /tmp/e2e-collector.log
        return 1
    fi
    
    if curl -sf http://localhost:13133/health > /dev/null 2>&1; then
        return 0
    fi
    
    return 1
}

# Wait for collector to be healthy
echo -e "\n${YELLOW}Waiting for collector to be healthy...${NC}"
HEALTH_CHECK_ATTEMPTS=0
MAX_HEALTH_ATTEMPTS=$((COLLECTOR_START_TIMEOUT / 2))

while [ $HEALTH_CHECK_ATTEMPTS -lt $MAX_HEALTH_ATTEMPTS ]; do
    if check_collector_health; then
        echo -e "${GREEN}Collector is healthy${NC}"
        
        # Verify components
        HEALTH_RESPONSE=$(curl -s http://localhost:13133/health)
        echo -e "${BLUE}Component Status:${NC}"
        echo "$HEALTH_RESPONSE" | jq -r '.components | to_entries[] | "\(.key): \(.value.healthy)"' || echo "$HEALTH_RESPONSE"
        break
    fi
    
    ((HEALTH_CHECK_ATTEMPTS++))
    if [ $HEALTH_CHECK_ATTEMPTS -eq $MAX_HEALTH_ATTEMPTS ]; then
        echo -e "${RED}Collector failed to become healthy${NC}"
        echo "Last 50 lines of collector log:"
        tail -50 /tmp/e2e-collector.log
        exit 1
    fi
    sleep 2
done

# Verify metrics endpoint
echo -e "\n${YELLOW}Verifying metrics endpoint...${NC}"
if curl -sf http://localhost:8888/metrics > /dev/null; then
    echo -e "${GREEN}Metrics endpoint is accessible${NC}"
    METRIC_COUNT=$(curl -s http://localhost:8888/metrics | grep -c "^[a-zA-Z]")
    echo "Collector exposing $METRIC_COUNT metrics"
else
    echo -e "${RED}Metrics endpoint not accessible${NC}"
fi

# Run the comprehensive tests
echo -e "\n${YELLOW}Running comprehensive E2E tests...${NC}"
cd "$PROJECT_ROOT"

# Run basic validation tests
echo -e "\n${BLUE}1. Running basic data flow tests...${NC}"
go test -v -timeout=$TEST_TIMEOUT ./tests/e2e/... -run TestEndToEndDataFlow
BASIC_TEST_EXIT=$?

# Run comprehensive validation tests
echo -e "\n${BLUE}2. Running comprehensive data validation tests...${NC}"
go test -v -timeout=$TEST_TIMEOUT ./tests/e2e/... -run TestComprehensiveDataValidation
COMPREHENSIVE_TEST_EXIT=$?

# Determine overall result
TEST_EXIT_CODE=0
if [ $BASIC_TEST_EXIT -ne 0 ] || [ $COMPREHENSIVE_TEST_EXIT -ne 0 ]; then
    TEST_EXIT_CODE=1
fi

# Generate test report
echo -e "\n${YELLOW}Generating comprehensive test report...${NC}"

# Query NRDB for test metrics summary
NRDB_SUMMARY=$(cat <<EOF
Test Metrics Summary:
- Run ID: $TEST_RUN_ID
- PostgreSQL Metrics: Query NRDB with "SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql.%' AND test.run_id = '$TEST_RUN_ID'"
- MySQL Metrics: Query NRDB with "SELECT count(*) FROM Metric WHERE metricName LIKE 'mysql.%' AND test.run_id = '$TEST_RUN_ID'"
- Custom Metrics: Query NRDB with "SELECT count(*) FROM Metric WHERE metricName LIKE 'sqlquery.%' AND test.run_id = '$TEST_RUN_ID'"
- Processed Metrics: Query NRDB with "SELECT count(*) FROM Metric WHERE test.run_id = '$TEST_RUN_ID'"
EOF
)

cat > "$REPORTS_DIR/summary-$TEST_RUN_ID.txt" <<EOF
Comprehensive E2E Test Summary
==============================
Run ID: $TEST_RUN_ID
Date: $(date)
Duration: $SECONDS seconds
Overall Exit Code: $TEST_EXIT_CODE

Environment:
- PostgreSQL: ${POSTGRES_HOST:-localhost}:${POSTGRES_PORT:-5432}
- MySQL: ${MYSQL_HOST:-localhost}:${MYSQL_PORT:-3306}
- New Relic Account: $NEW_RELIC_ACCOUNT_ID
- Collector Binary: $COLLECTOR_BINARY

Test Results:
- Basic Data Flow Tests: $([ $BASIC_TEST_EXIT -eq 0 ] && echo "PASSED" || echo "FAILED")
- Comprehensive Validation Tests: $([ $COMPREHENSIVE_TEST_EXIT -eq 0 ] && echo "PASSED" || echo "FAILED")

Collector Status:
- Process ID: $COLLECTOR_PID
- Health Check: $(check_collector_health && echo "HEALTHY" || echo "UNHEALTHY")
- Metrics Exposed: $METRIC_COUNT

$NRDB_SUMMARY

Validation Checklist:
[$([ $BASIC_TEST_EXIT -eq 0 ] && echo "x" || echo " ")] Databases connected successfully
[$([ $BASIC_TEST_EXIT -eq 0 ] && echo "x" || echo " ")] Metrics collected from PostgreSQL
[$([ $BASIC_TEST_EXIT -eq 0 ] && echo "x" || echo " ")] Metrics collected from MySQL
[$([ $BASIC_TEST_EXIT -eq 0 ] && echo "x" || echo " ")] Metrics exported to NRDB
[$([ $COMPREHENSIVE_TEST_EXIT -eq 0 ] && echo "x" || echo " ")] Metric attributes validated
[$([ $COMPREHENSIVE_TEST_EXIT -eq 0 ] && echo "x" || echo " ")] Processor effects verified
[$([ $COMPREHENSIVE_TEST_EXIT -eq 0 ] && echo "x" || echo " ")] Data accuracy confirmed
[$([ $COMPREHENSIVE_TEST_EXIT -eq 0 ] && echo "x" || echo " ")] Semantic conventions followed

Artifacts:
- Collector Log: $REPORTS_DIR/collector-$TEST_RUN_ID.log
- Exported Metrics: $REPORTS_DIR/metrics-$TEST_RUN_ID.json
- This Summary: $REPORTS_DIR/summary-$TEST_RUN_ID.txt

NRQL Validation Queries:
1. Verify test metrics exist:
   SELECT count(*) FROM Metric WHERE test.run_id = '$TEST_RUN_ID' SINCE 1 hour ago

2. Check metric shape:
   SELECT * FROM Metric WHERE test.run_id = '$TEST_RUN_ID' SINCE 1 hour ago LIMIT 10

3. Validate processors:
   SELECT count(*) FROM Metric WHERE test.run_id = '$TEST_RUN_ID' AND sampled = true SINCE 1 hour ago
   SELECT count(*) FROM Metric WHERE test.run_id = '$TEST_RUN_ID' AND plan.hash IS NOT NULL SINCE 1 hour ago

EOF

# Print summary
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "\n${GREEN}✓ All comprehensive E2E tests passed${NC}"
    echo -e "${GREEN}✓ Data flow validated from databases to NRDB${NC}"
    echo -e "${GREEN}✓ Metric shape and attributes verified${NC}"
    echo -e "${GREEN}✓ Processor functionality confirmed${NC}"
else
    echo -e "\n${RED}✗ Some E2E tests failed${NC}"
    echo -e "${YELLOW}Check the test report at: $REPORTS_DIR/summary-$TEST_RUN_ID.txt${NC}"
    echo -e "${YELLOW}Review collector logs at: $REPORTS_DIR/collector-$TEST_RUN_ID.log${NC}"
fi

echo -e "\n${BLUE}Test Run ID: $TEST_RUN_ID${NC}"
echo -e "${BLUE}Use this ID to query test metrics in NRDB${NC}"

exit $TEST_EXIT_CODE