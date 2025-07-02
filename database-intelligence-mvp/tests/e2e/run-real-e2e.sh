#!/bin/bash
# Run real E2E tests without mocks

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Real E2E Test Suite (No Mocks)${NC}"
echo -e "${GREEN}========================================${NC}"

# Clean up any previous runs
echo -e "\n${YELLOW}Cleaning up previous test environment...${NC}"
docker-compose -f docker-compose.e2e.yml down -v 2>/dev/null || true

# Start the real test environment
echo -e "\n${YELLOW}Starting real test environment...${NC}"
docker-compose -f docker-compose.e2e.yml up -d postgres-e2e mysql-e2e

# Wait for databases to be ready
echo -e "\n${YELLOW}Waiting for databases to be healthy...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    PG_READY=$(docker exec e2e-postgres pg_isready -U postgres 2>/dev/null || echo "not ready")
    MYSQL_READY=$(docker exec e2e-mysql mysqladmin ping -h localhost -uroot -proot 2>/dev/null || echo "not ready")
    
    if [[ "$PG_READY" == *"accepting connections"* ]] && [[ "$MYSQL_READY" == *"alive"* ]]; then
        echo -e "${GREEN}✓ Databases are ready${NC}"
        break
    fi
    
    echo -n "."
    sleep 2
    RETRY_COUNT=$((RETRY_COUNT + 1))
done

if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
    echo -e "\n${RED}Databases failed to become healthy${NC}"
    docker-compose -f docker-compose.e2e.yml logs
    exit 1
fi

# Start the collector with real configuration
echo -e "\n${YELLOW}Starting OpenTelemetry Collector...${NC}"
docker-compose -f docker-compose.e2e.yml up -d otel-collector-e2e

# Wait for collector to be ready
echo -e "\n${YELLOW}Waiting for collector to start...${NC}"
sleep 10

# Check collector status
if docker ps | grep -q e2e-collector; then
    echo -e "${GREEN}✓ Collector is running${NC}"
else
    echo -e "${RED}✗ Collector failed to start${NC}"
    docker logs e2e-collector
    exit 1
fi

# Run real E2E tests
echo -e "\n${YELLOW}Running real E2E tests...${NC}"
go test -v -run "TestRealE2EPipeline|TestRealQueryPatterns|TestDatabaseErrorScenarios" ./... -tags=e2e

# Collect test results
echo -e "\n${YELLOW}Collecting test results...${NC}"
mkdir -p test-results

# Get collector logs
docker logs e2e-collector > test-results/collector.log 2>&1

# Get metrics snapshot
curl -s http://localhost:8890/metrics > test-results/prometheus-metrics.txt 2>/dev/null || true

# Get file output
docker exec e2e-collector cat /var/lib/otel/e2e-output.json > test-results/e2e-output.json 2>/dev/null || true

# Analyze results
echo -e "\n${YELLOW}========================================${NC}"
echo -e "${YELLOW}Test Results Analysis${NC}"
echo -e "${YELLOW}========================================${NC}"

# Check for key metrics
echo -e "\n${YELLOW}PostgreSQL Metrics:${NC}"
if grep -q "postgresql_backends" test-results/prometheus-metrics.txt 2>/dev/null; then
    echo -e "${GREEN}✓ PostgreSQL backend metrics found${NC}"
else
    echo -e "${RED}✗ PostgreSQL backend metrics missing${NC}"
fi

if grep -q "postgresql_commits" test-results/prometheus-metrics.txt 2>/dev/null; then
    echo -e "${GREEN}✓ PostgreSQL transaction metrics found${NC}"
else
    echo -e "${RED}✗ PostgreSQL transaction metrics missing${NC}"
fi

echo -e "\n${YELLOW}MySQL Metrics:${NC}"
if grep -q "mysql_buffer_pool" test-results/prometheus-metrics.txt 2>/dev/null; then
    echo -e "${GREEN}✓ MySQL buffer pool metrics found${NC}"
else
    echo -e "${RED}✗ MySQL buffer pool metrics missing${NC}"
fi

echo -e "\n${YELLOW}Custom Processor Metrics:${NC}"
if grep -q "dbintel_cost_bytes_ingested" test-results/prometheus-metrics.txt 2>/dev/null; then
    echo -e "${GREEN}✓ Cost control metrics found${NC}"
else
    echo -e "${RED}✗ Cost control metrics missing${NC}"
fi

if grep -q "dbintel_circuit_breaker" test-results/prometheus-metrics.txt 2>/dev/null; then
    echo -e "${GREEN}✓ Circuit breaker metrics found${NC}"
else
    echo -e "${RED}✗ Circuit breaker metrics missing${NC}"
fi

# Check for PII redaction
echo -e "\n${YELLOW}PII Sanitization:${NC}"
if grep -q "REDACTED" test-results/e2e-output.json 2>/dev/null; then
    echo -e "${GREEN}✓ PII redaction is working${NC}"
    
    # Check specific PII patterns are not present
    if ! grep -E "(john\.doe@example\.com|123-45-6789|4111-1111-1111-1111)" test-results/e2e-output.json 2>/dev/null; then
        echo -e "${GREEN}✓ No raw PII found in output${NC}"
    else
        echo -e "${RED}✗ Raw PII found in output!${NC}"
    fi
else
    echo -e "${YELLOW}⚠ PII redaction not verified${NC}"
fi

# Check for query logs
echo -e "\n${YELLOW}Query Logs:${NC}"
if grep -q "pg_stat_statements" test-results/e2e-output.json 2>/dev/null; then
    echo -e "${GREEN}✓ Query logs being collected${NC}"
else
    echo -e "${RED}✗ Query logs not found${NC}"
fi

# Final summary
echo -e "\n${YELLOW}========================================${NC}"
echo -e "${YELLOW}Test artifacts saved in: ./test-results/${NC}"
echo -e "${YELLOW}========================================${NC}"

# Optionally keep environment running
if [ "$1" == "--keep" ]; then
    echo -e "\n${YELLOW}Test environment kept running. To stop:${NC}"
    echo "  docker-compose -f docker-compose.e2e.yml down -v"
else
    echo -e "\n${YELLOW}Cleaning up test environment...${NC}"
    docker-compose -f docker-compose.e2e.yml down -v
fi

echo -e "\n${GREEN}Real E2E tests completed!${NC}"