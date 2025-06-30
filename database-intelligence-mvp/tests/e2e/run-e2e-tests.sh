#!/bin/bash
set -e

# E2E Test Runner for Database Intelligence Collector
# This script runs end-to-end tests that validate data flow from databases to New Relic

echo "=== Database Intelligence Collector E2E Tests ==="

# Check prerequisites
if [ -z "$NEW_RELIC_LICENSE_KEY" ]; then
    echo "ERROR: NEW_RELIC_LICENSE_KEY environment variable is required"
    exit 1
fi

if [ -z "$NEW_RELIC_ACCOUNT_ID" ]; then
    echo "ERROR: NEW_RELIC_ACCOUNT_ID environment variable is required"
    exit 1
fi

# Set test environment
export E2E_TESTS=true
export TEST_RUN_ID=$(date +%s)
export TEST_TIMEOUT=${TEST_TIMEOUT:-30m}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "Test Configuration:"
echo "  Run ID: $TEST_RUN_ID"
echo "  Timeout: $TEST_TIMEOUT"
echo "  PostgreSQL: ${POSTGRES_HOST:-localhost}:${POSTGRES_PORT:-5432}"
echo "  MySQL: ${MYSQL_HOST:-localhost}:${MYSQL_PORT:-3306}"
echo "  New Relic Account: $NEW_RELIC_ACCOUNT_ID"

# Function to cleanup resources
cleanup() {
    echo -e "\n${YELLOW}Cleaning up test resources...${NC}"
    
    # Stop collector if running
    if [ -n "$COLLECTOR_PID" ]; then
        echo "Stopping collector (PID: $COLLECTOR_PID)"
        kill $COLLECTOR_PID 2>/dev/null || true
    fi
    
    # Stop test databases if we started them
    if [ "$START_DATABASES" = "true" ]; then
        echo "Stopping test databases"
        docker-compose -f tests/e2e/docker-compose-test.yaml down -v
    fi
    
    # Collect logs
    if [ -f "/tmp/e2e-collector.log" ]; then
        echo "Collector logs saved to: tests/e2e/reports/collector-$TEST_RUN_ID.log"
        mkdir -p tests/e2e/reports
        cp /tmp/e2e-collector.log tests/e2e/reports/collector-$TEST_RUN_ID.log
    fi
    
    # Collect metrics
    if [ -f "/tmp/e2e-metrics.json" ]; then
        echo "Metrics saved to: tests/e2e/reports/metrics-$TEST_RUN_ID.json"
        cp /tmp/e2e-metrics.json tests/e2e/reports/metrics-$TEST_RUN_ID.json
    fi
}

# Set trap for cleanup
trap cleanup EXIT

# Check if databases are available
echo -e "\n${YELLOW}Checking database connectivity...${NC}"
if ! nc -z ${POSTGRES_HOST:-localhost} ${POSTGRES_PORT:-5432} 2>/dev/null; then
    echo "PostgreSQL not available, starting test databases..."
    START_DATABASES=true
    docker-compose -f tests/e2e/docker-compose-test.yaml up -d
    echo "Waiting for databases to be ready..."
    sleep 30
fi

# Build collector if needed
if [ ! -f "./dist/database-intelligence-collector" ]; then
    echo -e "\n${YELLOW}Building collector...${NC}"
    make build
fi

# Start collector
echo -e "\n${YELLOW}Starting collector with e2e configuration...${NC}"
./dist/database-intelligence-collector \
    --config=tests/e2e/config/e2e-test-collector.yaml \
    --set=service.telemetry.logs.level=debug &

COLLECTOR_PID=$!
echo "Collector started with PID: $COLLECTOR_PID"

# Wait for collector to be healthy
echo "Waiting for collector to be healthy..."
HEALTH_CHECK_ATTEMPTS=30
for i in $(seq 1 $HEALTH_CHECK_ATTEMPTS); do
    if curl -sf http://localhost:13133/health > /dev/null 2>&1; then
        echo -e "${GREEN}Collector is healthy${NC}"
        break
    fi
    if [ $i -eq $HEALTH_CHECK_ATTEMPTS ]; then
        echo -e "${RED}Collector failed to become healthy${NC}"
        exit 1
    fi
    sleep 2
done

# Run the tests
echo -e "\n${YELLOW}Running E2E tests...${NC}"
go test -v -timeout=$TEST_TIMEOUT ./tests/e2e/... -run TestEndToEndDataFlow

TEST_EXIT_CODE=$?

# Generate test report
echo -e "\n${YELLOW}Generating test report...${NC}"
cat > tests/e2e/reports/summary-$TEST_RUN_ID.txt <<EOF
E2E Test Summary
================
Run ID: $TEST_RUN_ID
Date: $(date)
Exit Code: $TEST_EXIT_CODE

Environment:
- PostgreSQL: ${POSTGRES_HOST:-localhost}:${POSTGRES_PORT:-5432}
- MySQL: ${MYSQL_HOST:-localhost}:${MYSQL_PORT:-3306}
- New Relic Account: $NEW_RELIC_ACCOUNT_ID

Test Results:
EOF

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✓ All E2E tests passed${NC}" | tee -a tests/e2e/reports/summary-$TEST_RUN_ID.txt
else
    echo -e "${RED}✗ E2E tests failed${NC}" | tee -a tests/e2e/reports/summary-$TEST_RUN_ID.txt
fi

# Query NRDB for validation metrics
echo -e "\n${YELLOW}Querying NRDB for validation metrics...${NC}"
NRQL_QUERY="SELECT count(*) FROM Metric WHERE test.run_id = '$TEST_RUN_ID' SINCE 10 minutes ago"
echo "NRQL Query: $NRQL_QUERY" >> tests/e2e/reports/summary-$TEST_RUN_ID.txt

exit $TEST_EXIT_CODE