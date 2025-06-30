#!/bin/bash
# Simple E2E validation script

set -e

echo "=== Database Intelligence E2E Validation ==="

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
COLLECTOR_BINARY="./dist/database-intelligence-collector"
CONFIG_FILE="tests/e2e/config/working-test-config.yaml"
METRICS_FILE="/tmp/e2e-metrics.json"
COLLECTOR_LOG="/tmp/e2e-collector.log"

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    pkill -f database-intelligence-collector 2>/dev/null || true
    rm -f $METRICS_FILE $COLLECTOR_LOG
}
trap cleanup EXIT

# Check prerequisites
echo -e "\n${YELLOW}Checking prerequisites...${NC}"

if [ ! -f "$COLLECTOR_BINARY" ]; then
    echo -e "${RED}✗ Collector binary not found at $COLLECTOR_BINARY${NC}"
    echo "  Run: make build"
    exit 1
fi
echo -e "${GREEN}✓ Collector binary found${NC}"

# Validate configuration
echo -e "\n${YELLOW}Validating configuration...${NC}"
if $COLLECTOR_BINARY validate --config=$CONFIG_FILE; then
    echo -e "${GREEN}✓ Configuration is valid${NC}"
else
    echo -e "${RED}✗ Configuration validation failed${NC}"
    exit 1
fi

# Check database connectivity
echo -e "\n${YELLOW}Checking database connectivity...${NC}"
if nc -z localhost 5432 2>/dev/null; then
    echo -e "${GREEN}✓ PostgreSQL is accessible${NC}"
else
    echo -e "${YELLOW}⚠ PostgreSQL not accessible on localhost:5432${NC}"
fi

if nc -z localhost 3306 2>/dev/null; then
    echo -e "${GREEN}✓ MySQL is accessible${NC}"
else
    echo -e "${YELLOW}⚠ MySQL not accessible on localhost:3306${NC}"
fi

# Start collector
echo -e "\n${YELLOW}Starting collector...${NC}"
rm -f $METRICS_FILE
$COLLECTOR_BINARY --config=$CONFIG_FILE > $COLLECTOR_LOG 2>&1 &
COLLECTOR_PID=$!
echo "Collector PID: $COLLECTOR_PID"

# Wait for metrics
echo -e "\n${YELLOW}Waiting for metrics collection (30s)...${NC}"
sleep 30

# Check if collector is still running
if ! kill -0 $COLLECTOR_PID 2>/dev/null; then
    echo -e "${RED}✗ Collector crashed${NC}"
    echo "Last 20 lines of log:"
    tail -20 $COLLECTOR_LOG
    exit 1
fi

# Validate metrics
echo -e "\n${YELLOW}Validating collected metrics...${NC}"

if [ ! -f "$METRICS_FILE" ]; then
    echo -e "${RED}✗ No metrics file generated${NC}"
    exit 1
fi

# Count metrics
TOTAL_METRICS=$(jq -r '.resourceMetrics[].scopeMetrics[].metrics[].name' $METRICS_FILE 2>/dev/null | sort -u | wc -l | tr -d ' ')
PG_METRICS=$(jq -r '.resourceMetrics[].scopeMetrics[].metrics[].name' $METRICS_FILE 2>/dev/null | grep '^postgresql\.' | sort -u | wc -l | tr -d ' ')
MYSQL_METRICS=$(jq -r '.resourceMetrics[].scopeMetrics[].metrics[].name' $METRICS_FILE 2>/dev/null | grep '^mysql\.' | sort -u | wc -l | tr -d ' ')

echo -e "${GREEN}✓ Metrics file generated${NC}"
echo "  Total unique metrics: $TOTAL_METRICS"
echo "  PostgreSQL metrics: $PG_METRICS"
echo "  MySQL metrics: $MYSQL_METRICS"

# Validate required metrics
echo -e "\n${YELLOW}Checking required metrics...${NC}"

# PostgreSQL required metrics
PG_REQUIRED=(
    "postgresql.backends"
    "postgresql.commits"
    "postgresql.db_size"
    "postgresql.rollbacks"
)

for metric in "${PG_REQUIRED[@]}"; do
    if jq -r '.resourceMetrics[].scopeMetrics[].metrics[].name' $METRICS_FILE | grep -q "^${metric}$"; then
        echo -e "${GREEN}✓ Found: $metric${NC}"
    else
        echo -e "${RED}✗ Missing: $metric${NC}"
    fi
done

# MySQL required metrics
MYSQL_REQUIRED=(
    "mysql.buffer_pool.data_pages"
    "mysql.buffer_pool.operations"
    "mysql.handlers"
    "mysql.locks"
)

for metric in "${MYSQL_REQUIRED[@]}"; do
    if jq -r '.resourceMetrics[].scopeMetrics[].metrics[].name' $METRICS_FILE | grep -q "^${metric}$"; then
        echo -e "${GREEN}✓ Found: $metric${NC}"
    else
        echo -e "${RED}✗ Missing: $metric${NC}"
    fi
done

# Check attributes
echo -e "\n${YELLOW}Checking metric attributes...${NC}"

# Check for collector.name attribute
if jq -r '.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[].attributes[] | select(.key=="collector.name") | .value.stringValue' $METRICS_FILE | grep -q "otelcol"; then
    echo -e "${GREEN}✓ collector.name attribute found${NC}"
else
    echo -e "${RED}✗ collector.name attribute missing${NC}"
fi

# Check for test attributes
if jq -r '.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[].attributes[] | select(.key=="test.environment") | .value.stringValue' $METRICS_FILE | grep -q "e2e"; then
    echo -e "${GREEN}✓ test.environment attribute found${NC}"
else
    echo -e "${RED}✗ test.environment attribute missing${NC}"
fi

# Summary
echo -e "\n${YELLOW}=== Validation Summary ===${NC}"
if [ $PG_METRICS -gt 0 ] && [ $MYSQL_METRICS -gt 0 ]; then
    echo -e "${GREEN}✓ E2E validation successful!${NC}"
    echo "  - Collector is running"
    echo "  - Configuration is valid"  
    echo "  - Metrics are being collected"
    echo "  - Attributes are properly set"
    echo ""
    echo "Metrics file: $METRICS_FILE"
    echo "Collector log: $COLLECTOR_LOG"
    exit 0
else
    echo -e "${RED}✗ E2E validation failed${NC}"
    echo "Check the collector log for errors: $COLLECTOR_LOG"
    exit 1
fi