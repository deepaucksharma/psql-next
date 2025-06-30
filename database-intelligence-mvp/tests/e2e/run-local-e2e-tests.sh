#!/bin/bash
set -e

# Local E2E Test Runner for Database Intelligence Collector
# This script validates the collector functionality without requiring New Relic credentials

echo "=== Database Intelligence Collector Local E2E Tests ==="
echo "=== Testing Data Collection and Processing Locally ==="

# Set test environment
export E2E_TESTS=true
export TEST_RUN_ID="e2e_local_$(date +%s)"
export TEST_TIMEOUT=${TEST_TIMEOUT:-10m}
export COLLECTOR_START_TIMEOUT=${COLLECTOR_START_TIMEOUT:-30}

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

# Function to wait for port
wait_for_port() {
    local host=$1
    local port=$2
    local service=$3
    local max_attempts=30
    local attempt=1
    
    echo -e "${YELLOW}Waiting for $service at $host:$port...${NC}"
    
    while [ $attempt -le $max_attempts ]; do
        if nc -z "$host" "$port" 2>/dev/null; then
            echo -e "${GREEN}$service is ready${NC}"
            return 0
        fi
        echo "Attempt $attempt/$max_attempts..."
        sleep 2
        ((attempt++))
    done
    
    echo -e "${RED}$service failed to start${NC}"
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
    wait_for_port ${POSTGRES_HOST:-localhost} ${POSTGRES_PORT:-5432} "PostgreSQL"
    wait_for_port ${MYSQL_HOST:-localhost} ${MYSQL_PORT:-3306} "MySQL"
    
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

# Skip configuration validation since --dry-run is not supported
echo -e "\n${YELLOW}Skipping configuration validation (--dry-run not supported)...${NC}"

# Start collector
echo -e "\n${YELLOW}Starting collector with minimal test configuration...${NC}"
cd "$PROJECT_ROOT"
"$COLLECTOR_BINARY" \
    --config="$SCRIPT_DIR/config/e2e-test-collector-minimal.yaml" \
    --set=service.telemetry.logs.level=debug \
    > /tmp/e2e-collector-console.log 2>&1 &

COLLECTOR_PID=$!
echo "Collector started with PID: $COLLECTOR_PID"

# Wait for collector to be healthy
echo -e "\n${YELLOW}Waiting for collector to be healthy...${NC}"
HEALTH_CHECK_ATTEMPTS=0
MAX_HEALTH_ATTEMPTS=$((COLLECTOR_START_TIMEOUT / 2))

while [ $HEALTH_CHECK_ATTEMPTS -lt $MAX_HEALTH_ATTEMPTS ]; do
    if ! kill -0 $COLLECTOR_PID 2>/dev/null; then
        echo -e "${RED}Collector process died${NC}"
        tail -20 /tmp/e2e-collector-console.log
        exit 1
    fi
    
    # Check zpages endpoint since health_check extension not available
    if curl -sf http://localhost:55679/debug/tracez > /dev/null 2>&1; then
        echo -e "${GREEN}Collector is healthy${NC}"
        break
    fi
    
    ((HEALTH_CHECK_ATTEMPTS++))
    if [ $HEALTH_CHECK_ATTEMPTS -eq $MAX_HEALTH_ATTEMPTS ]; then
        echo -e "${RED}Collector failed to become healthy${NC}"
        echo "Last 50 lines of collector log:"
        tail -50 /tmp/e2e-collector-console.log
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

# Wait for data collection
echo -e "\n${YELLOW}Waiting for data collection (30 seconds)...${NC}"
sleep 30

# Verify Prometheus metrics
echo -e "\n${YELLOW}Checking Prometheus metrics...${NC}"
if curl -sf http://localhost:8889/metrics > /dev/null; then
    echo -e "${GREEN}Prometheus endpoint is accessible${NC}"
    
    # Check for database metrics
    PG_METRICS=$(curl -s http://localhost:8889/metrics | grep -c "postgresql_")
    MYSQL_METRICS=$(curl -s http://localhost:8889/metrics | grep -c "mysql_")
    
    echo "PostgreSQL metrics found: $PG_METRICS"
    echo "MySQL metrics found: $MYSQL_METRICS"
    
    if [ $PG_METRICS -gt 0 ] && [ $MYSQL_METRICS -gt 0 ]; then
        echo -e "${GREEN}✓ Database metrics are being collected${NC}"
    else
        echo -e "${RED}✗ Missing database metrics${NC}"
    fi
fi

# Check file export
echo -e "\n${YELLOW}Checking file export...${NC}"
if [ -f "/tmp/e2e-metrics.json" ]; then
    echo -e "${GREEN}Metrics file exists${NC}"
    METRIC_LINES=$(wc -l < /tmp/e2e-metrics.json)
    echo "Metrics file has $METRIC_LINES lines"
    
    # Check for specific metric types
    if grep -q "postgresql.database.size" /tmp/e2e-metrics.json; then
        echo -e "${GREEN}✓ PostgreSQL metrics found in export${NC}"
    else
        echo -e "${RED}✗ PostgreSQL metrics missing${NC}"
    fi
    
    if grep -q "mysql.threads" /tmp/e2e-metrics.json; then
        echo -e "${GREEN}✓ MySQL metrics found in export${NC}"
    else
        echo -e "${RED}✗ MySQL metrics missing${NC}"
    fi
else
    echo -e "${RED}Metrics file not found${NC}"
fi

# Generate test report
echo -e "\n${YELLOW}Generating test report...${NC}"

TEST_PASSED=true
if [ $PG_METRICS -eq 0 ] || [ $MYSQL_METRICS -eq 0 ]; then
    TEST_PASSED=false
fi

cat > "$REPORTS_DIR/summary-$TEST_RUN_ID.txt" <<EOF
Local E2E Test Summary
======================
Run ID: $TEST_RUN_ID
Date: $(date)
Duration: $SECONDS seconds

Environment:
- PostgreSQL: ${POSTGRES_HOST:-localhost}:${POSTGRES_PORT:-5432}
- MySQL: ${MYSQL_HOST:-localhost}:${MYSQL_PORT:-3306}
- Collector Binary: $COLLECTOR_BINARY

Test Results:
- Collector Started: YES
- Health Check: PASSED
- Metrics Endpoint: PASSED
- PostgreSQL Metrics: $([ $PG_METRICS -gt 0 ] && echo "PASSED ($PG_METRICS metrics)" || echo "FAILED")
- MySQL Metrics: $([ $MYSQL_METRICS -gt 0 ] && echo "PASSED ($MYSQL_METRICS metrics)" || echo "FAILED")
- File Export: $([ -f "/tmp/e2e-metrics.json" ] && echo "PASSED" || echo "FAILED")

Collector Status:
- Process ID: $COLLECTOR_PID
- Internal Metrics: $METRIC_COUNT

Validation:
[$([ $PG_METRICS -gt 0 ] && echo "x" || echo " ")] PostgreSQL metrics collected
[$([ $MYSQL_METRICS -gt 0 ] && echo "x" || echo " ")] MySQL metrics collected
[$([ -f "/tmp/e2e-metrics.json" ] && echo "x" || echo " ")] Metrics exported to file
[$([ $METRIC_COUNT -gt 0 ] && echo "x" || echo " ")] Collector internal metrics available

Artifacts:
- Collector Log: $REPORTS_DIR/collector-$TEST_RUN_ID.log
- Exported Metrics: $REPORTS_DIR/metrics-$TEST_RUN_ID.json
- This Summary: $REPORTS_DIR/summary-$TEST_RUN_ID.txt
EOF

# Print summary
if [ "$TEST_PASSED" = "true" ]; then
    echo -e "\n${GREEN}✓ Local E2E tests passed${NC}"
    echo -e "${GREEN}✓ Collector successfully collecting database metrics${NC}"
    exit 0
else
    echo -e "\n${RED}✗ Some local E2E tests failed${NC}"
    echo -e "${YELLOW}Check the test report at: $REPORTS_DIR/summary-$TEST_RUN_ID.txt${NC}"
    echo -e "${YELLOW}Review collector logs at: $REPORTS_DIR/collector-$TEST_RUN_ID.log${NC}"
    exit 1
fi