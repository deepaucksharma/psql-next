#!/bin/bash
# Run comprehensive E2E tests for Database Intelligence Collector

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Database Intelligence E2E Test Suite${NC}"
echo -e "${GREEN}========================================${NC}"

# Check prerequisites
echo -e "\n${YELLOW}Checking prerequisites...${NC}"

if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker is not installed${NC}"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo -e "${RED}Go is not installed${NC}"
    exit 1
fi

# Clean up any previous test runs
echo -e "\n${YELLOW}Cleaning up previous test runs...${NC}"
docker-compose -f docker-compose.e2e.yml down -v 2>/dev/null || true
rm -rf output/*

# Build the collector
echo -e "\n${YELLOW}Building Database Intelligence Collector...${NC}"
cd ../..
make build
cd tests/e2e

# Start E2E test environment
echo -e "\n${YELLOW}Starting E2E test environment...${NC}"
docker-compose -f docker-compose.e2e.yml up -d

# Wait for services to be healthy
echo -e "\n${YELLOW}Waiting for services to be ready...${NC}"
MAX_RETRIES=60
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if docker-compose -f docker-compose.e2e.yml ps | grep -q "healthy"; then
        echo -e "${GREEN}✓ All services are healthy${NC}"
        break
    fi
    
    echo -n "."
    sleep 2
    RETRY_COUNT=$((RETRY_COUNT + 1))
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "\n${RED}Services failed to become healthy${NC}"
    docker-compose -f docker-compose.e2e.yml logs
    exit 1
fi

# Run the E2E tests
echo -e "\n${YELLOW}Running E2E tests...${NC}"
mkdir -p output

# Set test environment variables
export E2E_TEST=true
export TEST_POSTGRES_HOST=localhost
export TEST_POSTGRES_PORT=5433
export TEST_MYSQL_HOST=localhost
export TEST_MYSQL_PORT=3307

# Run Go tests
if go test -v -timeout 10m ./... -tags=e2e; then
    TEST_RESULT=0
    echo -e "\n${GREEN}✓ All E2E tests passed${NC}"
else
    TEST_RESULT=1
    echo -e "\n${RED}✗ E2E tests failed${NC}"
fi

# Collect test artifacts
echo -e "\n${YELLOW}Collecting test artifacts...${NC}"
mkdir -p test-results

# Copy collector logs
docker-compose -f docker-compose.e2e.yml logs otel-collector-e2e > test-results/collector.log 2>&1

# Copy output files
cp -r output/* test-results/ 2>/dev/null || true

# Get metrics snapshot
curl -s http://localhost:8890/metrics > test-results/metrics.txt 2>/dev/null || true

# Get mock server requests
curl -s http://localhost:4319/mockserver/retrieve?type=REQUESTS > test-results/nrdb-requests.json 2>/dev/null || true

# Print summary
echo -e "\n${YELLOW}========================================${NC}"
echo -e "${YELLOW}E2E Test Summary${NC}"
echo -e "${YELLOW}========================================${NC}"

# Check key validations
echo -e "\n${YELLOW}Key Validations:${NC}"

# 1. Check if metrics were collected
if grep -q "postgresql_backends" test-results/metrics.txt 2>/dev/null; then
    echo -e "${GREEN}✓ PostgreSQL metrics collected${NC}"
else
    echo -e "${RED}✗ PostgreSQL metrics missing${NC}"
fi

if grep -q "mysql_buffer_pool" test-results/metrics.txt 2>/dev/null; then
    echo -e "${GREEN}✓ MySQL metrics collected${NC}"
else
    echo -e "${RED}✗ MySQL metrics missing${NC}"
fi

# 2. Check if PII was sanitized
if [ -f test-results/e2e-output.json ]; then
    if grep -q "REDACTED" test-results/e2e-output.json; then
        echo -e "${GREEN}✓ PII sanitization working${NC}"
    else
        echo -e "${YELLOW}⚠ PII sanitization not verified${NC}"
    fi
fi

# 3. Check if data was sent to NRDB
if [ -f test-results/nrdb-requests.json ]; then
    REQUEST_COUNT=$(jq length test-results/nrdb-requests.json 2>/dev/null || echo "0")
    if [ "$REQUEST_COUNT" -gt "0" ]; then
        echo -e "${GREEN}✓ Data sent to NRDB: $REQUEST_COUNT requests${NC}"
    else
        echo -e "${RED}✗ No data sent to NRDB${NC}"
    fi
fi

# 4. Check processor health
if grep -q "otelcol_processor_accepted_metric_points" test-results/metrics.txt 2>/dev/null; then
    echo -e "${GREEN}✓ Processors are processing data${NC}"
else
    echo -e "${RED}✗ Processor metrics missing${NC}"
fi

# Clean up if requested
if [ "$1" != "--keep" ]; then
    echo -e "\n${YELLOW}Cleaning up test environment...${NC}"
    docker-compose -f docker-compose.e2e.yml down -v
else
    echo -e "\n${YELLOW}Test environment kept running. To stop:${NC}"
    echo "  docker-compose -f docker-compose.e2e.yml down -v"
fi

echo -e "\n${YELLOW}Test artifacts saved in: ./test-results/${NC}"

exit $TEST_RESULT