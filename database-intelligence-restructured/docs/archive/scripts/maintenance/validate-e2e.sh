#!/bin/bash
# End-to-End Validation Script for Database Intelligence Collector
# This script validates that all processors communicate properly and data flows correctly

set -e

echo "=== Database Intelligence E2E Validation ==="

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
COLLECTOR_BINARY="./bin/dbintel"
CONFIG_FILE="config/test-pipeline.yaml"
METRICS_FILE="/tmp/collector-output.json"
COLLECTOR_LOG="/tmp/e2e-collector.log"
PASSED=0
FAILED=0

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    pkill -f dbintel 2>/dev/null || true
    rm -f $METRICS_FILE $COLLECTOR_LOG /tmp/test-*.json
}
trap cleanup EXIT

# Test result tracking
test_pass() {
    PASSED=$((PASSED + 1))
    echo -e "${GREEN}✓ $1${NC}"
}

test_fail() {
    FAILED=$((FAILED + 1))
    echo -e "${RED}✗ $1${NC}"
}

# Check prerequisites
echo -e "\n${YELLOW}Checking prerequisites...${NC}"

# Check if we need to build
if [ ! -f "$COLLECTOR_BINARY" ]; then
    echo -e "${YELLOW}Building collector...${NC}"
    if make build > /dev/null 2>&1; then
        test_pass "Collector built successfully"
    else
        test_fail "Failed to build collector"
        exit 1
    fi
else
    test_pass "Collector binary found"
fi

# Run unit tests for processors
echo -e "\n${YELLOW}Running processor unit tests...${NC}"
processors=("adaptivesampler" "circuitbreaker" "planattributeextractor" "verification" "costcontrol" "nrerrormonitor" "querycorrelator")

for proc in "${processors[@]}"; do
    if [ -d "processors/$proc" ]; then
        if go test ./processors/$proc/... > /dev/null 2>&1; then
            test_pass "$proc tests passed"
        else
            test_fail "$proc tests failed"
        fi
    fi
done

# Test processor communication with synthetic data
echo -e "\n${YELLOW}Testing processor communication...${NC}"

# Create test metric with PII and plan data
cat > /tmp/test-metric.json << 'EOF'
{
  "resourceMetrics": [{
    "resource": {
      "attributes": [{
        "key": "service.name",
        "value": { "stringValue": "test-db" }
      }, {
        "key": "db.system",
        "value": { "stringValue": "postgresql" }
      }]
    },
    "scopeMetrics": [{
      "metrics": [{
        "name": "db.query.duration",
        "histogram": {
          "dataPoints": [{
            "timeUnixNano": "1234567890000000000",
            "sum": 123.45,
            "count": 1,
            "attributes": [{
              "key": "db.statement",
              "value": { "stringValue": "SELECT * FROM users WHERE email='test@example.com' AND ssn='123-45-6789'" }
            }, {
              "key": "db.plan.json",
              "value": { "stringValue": "{\"Plan\": {\"Node Type\": \"Seq Scan\", \"Relation Name\": \"users\"}}" }
            }, {
              "key": "db.user",
              "value": { "stringValue": "postgres" }
            }, {
              "key": "db.name",
              "value": { "stringValue": "testdb" }
            }]
          }]
        }
      }]
    }]
  }]
}
EOF

# Run integration test
echo -e "\n${YELLOW}Running integration tests...${NC}"
cd tests/integration > /dev/null 2>&1 || {
    test_fail "Integration test directory not found"
    cd - > /dev/null
}

if go test -v -run TestFullProcessorPipeline 2>&1 | grep -q "PASS"; then
    test_pass "Full processor pipeline test"
else
    test_fail "Full processor pipeline test"
fi

if go test -v -run TestProcessorChaining 2>&1 | grep -q "PASS"; then
    test_pass "Processor chaining test"
else
    test_fail "Processor chaining test"
fi

if go test -v -run TestProcessorErrorHandling 2>&1 | grep -q "PASS"; then
    test_pass "Error handling test"
else
    test_fail "Error handling test"
fi

cd - > /dev/null 2>&1

# Test with real collector if config exists
if [ -f "$CONFIG_FILE" ]; then
    echo -e "\n${YELLOW}Testing with real collector...${NC}"
    
    # Start collector
    rm -f $METRICS_FILE
    $COLLECTOR_BINARY --config=$CONFIG_FILE > $COLLECTOR_LOG 2>&1 &
    COLLECTOR_PID=$!
    
    # Wait for startup
    sleep 5
    
    # Check if running
    if kill -0 $COLLECTOR_PID 2>/dev/null; then
        test_pass "Collector started successfully"
        
        # Wait for some data
        sleep 10
        
        # Check output file
        if [ -f "$METRICS_FILE" ] && [ -s "$METRICS_FILE" ]; then
            test_pass "Metrics file generated"
            
            # Check for PII sanitization
            if grep -q "test@example.com" $METRICS_FILE 2>/dev/null; then
                test_fail "PII not sanitized (email found)"
            else
                test_pass "PII sanitization working"
            fi
            
            # Check for plan hash
            if grep -q "db.query.plan.hash" $METRICS_FILE 2>/dev/null; then
                test_pass "Plan extraction working"
            else
                test_fail "Plan extraction not working"
            fi
        else
            test_fail "No metrics output generated"
        fi
        
        # Stop collector
        kill $COLLECTOR_PID 2>/dev/null || true
    else
        test_fail "Collector failed to start"
        echo "Last 10 lines of log:"
        tail -10 $COLLECTOR_LOG 2>/dev/null
    fi
else
    echo -e "${YELLOW}Skipping real collector test (config not found)${NC}"
fi

# Summary
echo -e "\n${YELLOW}========================================${NC}"
echo -e "${YELLOW}E2E Validation Summary${NC}"
echo -e "${YELLOW}========================================${NC}"
echo -e "Tests Passed: ${GREEN}$PASSED${NC}"
echo -e "Tests Failed: ${RED}$FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}✅ All validation tests passed!${NC}"
    echo -e "${GREEN}Processors are communicating properly.${NC}"
    exit 0
else
    echo -e "\n${RED}❌ Some validation tests failed.${NC}"
    echo -e "${RED}Please check the failing tests above.${NC}"
    exit 1
fi