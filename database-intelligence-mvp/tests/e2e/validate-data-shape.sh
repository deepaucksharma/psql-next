#!/bin/bash
set -e

# Data Shape Validation Script
# Validates the structure and content of collected metrics

echo "=== E2E Data Shape Validation ==="

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check if metrics file exists
METRICS_FILE="${1:-/tmp/e2e-metrics.json}"
if [ ! -f "$METRICS_FILE" ]; then
    echo -e "${RED}Metrics file not found: $METRICS_FILE${NC}"
    exit 1
fi

echo "Validating: $METRICS_FILE"

# Validation counters
TOTAL_CHECKS=0
PASSED_CHECKS=0

# Helper function for validation
validate() {
    local description="$1"
    local command="$2"
    local expected="$3"
    
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    echo -n "Checking $description... "
    
    result=$(eval "$command" 2>/dev/null || echo "0")
    
    # Clean up result (remove newlines and spaces)
    result=$(echo "$result" | tr -d '\n' | tr -d ' ')
    
    # Check if numeric comparison is needed
    if [[ "$expected" =~ ^[0-9]+$ ]] && [[ "$result" =~ ^[0-9]+$ ]]; then
        if [ "$result" -ge "$expected" ]; then
            echo -e "${GREEN}✓${NC} ($result)"
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
            return 0
        else
            echo -e "${RED}✗${NC} (expected: >=$expected, got: $result)"
            return 1
        fi
    else
        if [ "$result" = "$expected" ]; then
            echo -e "${GREEN}✓${NC} ($result)"
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
            return 0
        else
            echo -e "${RED}✗${NC} (expected: $expected, got: $result)"
            return 1
        fi
    fi
}

echo -e "\n${YELLOW}1. Overall Structure${NC}"
validate "JSON validity" "jq -e . $METRICS_FILE > /dev/null && echo 1" "1"
validate "resourceMetrics exists" "jq -e '.resourceMetrics' $METRICS_FILE > /dev/null && echo 1" "1"
validate "Has metrics data" "jq '.resourceMetrics | length' $METRICS_FILE" "1"

echo -e "\n${YELLOW}2. PostgreSQL Metrics${NC}"
validate "PostgreSQL backends metric" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | select(.name==\"postgresql.backends\") | .name' $METRICS_FILE | wc -l" "1"
validate "PostgreSQL commits metric" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | select(.name==\"postgresql.commits\") | .name' $METRICS_FILE | wc -l" "1"
validate "PostgreSQL db_size metric" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | select(.name==\"postgresql.db_size\") | .name' $METRICS_FILE | wc -l" "1"
validate "PostgreSQL metric count" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[].name' $METRICS_FILE | grep -c postgresql" "10"

echo -e "\n${YELLOW}3. MySQL Metrics${NC}"
validate "MySQL threads metric" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | select(.name==\"mysql.threads\") | .name' $METRICS_FILE | wc -l" "1"
validate "MySQL buffer_pool metrics" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[].name' $METRICS_FILE | grep -c 'mysql.buffer_pool'" "5"
validate "MySQL operations metric" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[] | select(.name==\"mysql.operations\") | .name' $METRICS_FILE | wc -l" "1"
validate "MySQL metric count" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[].name' $METRICS_FILE | grep -c mysql" "15"

echo -e "\n${YELLOW}4. Test Attributes${NC}"
validate "test.environment attribute" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[].attributes[] | select(.key==\"test.environment\") | .value.stringValue' $METRICS_FILE | grep -c 'e2e'" "1"
validate "test.run_id attribute" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[].attributes[] | select(.key==\"test.run_id\") | .key' $METRICS_FILE | wc -l" "1"
validate "collector.name attribute" "jq -r '.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[].attributes[] | select(.key==\"collector.name\") | .value.stringValue' $METRICS_FILE | grep -c 'otelcol'" "1"

echo -e "\n${YELLOW}5. Data Point Structure${NC}"
validate "Timestamps present" "jq '.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[] | select(.timeUnixNano == null)' $METRICS_FILE | wc -l" "0"
validate "Values are numeric" "jq '.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[] | select(.asInt == null and .asDouble == null)' $METRICS_FILE | wc -l" "0"
validate "StartTimestamp present" "jq '.resourceMetrics[].scopeMetrics[].metrics[].sum.dataPoints[] | select(.startTimeUnixNano == null)' $METRICS_FILE | wc -l" "0"

echo -e "\n${YELLOW}6. Metric Metadata${NC}"
validate "Metric descriptions" "jq '.resourceMetrics[].scopeMetrics[].metrics[] | select(.description == null or .description == \"\")' $METRICS_FILE | wc -l" "0"
validate "Metric units" "jq '.resourceMetrics[].scopeMetrics[].metrics[] | select(.unit == null)' $METRICS_FILE | wc -l" "0"
validate "Aggregation temporality" "jq '.resourceMetrics[].scopeMetrics[].metrics[].sum | select(.aggregationTemporality == null)' $METRICS_FILE | wc -l" "0"

echo -e "\n${YELLOW}7. Resource Attributes${NC}"
validate "Database name for PostgreSQL" "jq -r '.resourceMetrics[] | select(.resource.attributes[]?.value.stringValue | contains(\"postgres\")) | .resource.attributes[]?.value.stringValue' $METRICS_FILE | grep -c postgres" "1"
validate "Instrumentation scope" "jq '.resourceMetrics[].scopeMetrics[] | select(.scope.name == null)' $METRICS_FILE | wc -l" "0"

# Summary
echo -e "\n${YELLOW}=== Validation Summary ===${NC}"
echo "Total checks: $TOTAL_CHECKS"
echo "Passed: $PASSED_CHECKS"
echo "Failed: $((TOTAL_CHECKS - PASSED_CHECKS))"

if [ $PASSED_CHECKS -eq $TOTAL_CHECKS ]; then
    echo -e "\n${GREEN}✓ All data shape validations passed!${NC}"
    exit 0
else
    echo -e "\n${RED}✗ Some data shape validations failed${NC}"
    echo -e "${YELLOW}This may indicate issues with metric collection or processing${NC}"
    exit 1
fi