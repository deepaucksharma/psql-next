#!/bin/bash
# Integration test script for database collectors

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
DATABASE=${1:-all}
TIMEOUT=${2:-120}  # 2 minutes default

echo -e "${BLUE}=== Database Intelligence Integration Tests ===${NC}"
echo -e "Target: ${YELLOW}$DATABASE${NC}"
echo -e "Timeout: ${YELLOW}$TIMEOUT seconds${NC}"
echo ""

# Test results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
TEST_RESULTS=""

# Function to run a test
run_test() {
    local test_name=$1
    local test_command=$2
    local expected_result=$3
    
    ((TOTAL_TESTS++))
    echo -n -e "${YELLOW}Testing $test_name... ${NC}"
    
    if eval "$test_command" > /dev/null 2>&1; then
        if [ -n "$expected_result" ]; then
            # Check for expected result
            if eval "$expected_result" > /dev/null 2>&1; then
                echo -e "${GREEN}✓${NC}"
                ((PASSED_TESTS++))
                TEST_RESULTS+="\n✓ $test_name: PASSED"
            else
                echo -e "${RED}✗${NC}"
                ((FAILED_TESTS++))
                TEST_RESULTS+="\n✗ $test_name: FAILED (result check)"
            fi
        else
            echo -e "${GREEN}✓${NC}"
            ((PASSED_TESTS++))
            TEST_RESULTS+="\n✓ $test_name: PASSED"
        fi
    else
        echo -e "${RED}✗${NC}"
        ((FAILED_TESTS++))
        TEST_RESULTS+="\n✗ $test_name: FAILED"
    fi
}

# Function to test a specific database
test_database() {
    local db=$1
    local config_file="configs/${db}-maximum-extraction.yaml"
    
    echo -e "\n${BLUE}[Testing $db]${NC}"
    
    # Check if config exists
    if [ ! -f "$config_file" ]; then
        echo -e "${RED}Error: Configuration not found: $config_file${NC}"
        return 1
    fi
    
    # Test 1: Configuration validation
    run_test "Configuration syntax" \
        "./scripts/validate-config.sh $db" \
        ""
    
    # Test 2: Start collector
    run_test "Collector startup" \
        "docker run -d --name otel-test-${db} --rm \
         -v \$(pwd)/$config_file:/etc/otelcol/config.yaml \
         -e NEW_RELIC_LICENSE_KEY=dummy_key \
         -e ${db^^}_HOST=localhost \
         -e ${db^^}_PORT=5432 \
         -e ${db^^}_USER=test \
         -e ${db^^}_PASSWORD=test \
         -p 8889:8888 \
         otel/opentelemetry-collector-contrib:latest" \
        "sleep 5 && docker ps | grep -q otel-test-${db}"
    
    # Test 3: Health check
    if docker ps | grep -q otel-test-${db}; then
        run_test "Health endpoint" \
            "curl -s http://localhost:8889/health" \
            ""
        
        # Test 4: Metrics endpoint
        run_test "Metrics endpoint" \
            "curl -s http://localhost:8889/metrics | grep -q otelcol" \
            ""
        
        # Test 5: Check for errors in logs
        run_test "No startup errors" \
            "! docker logs otel-test-${db} 2>&1 | grep -i 'error'" \
            ""
        
        # Test 6: Pipeline verification
        run_test "Pipeline active" \
            "docker logs otel-test-${db} 2>&1 | grep -q 'Everything is ready'" \
            ""
        
        # Cleanup
        docker stop otel-test-${db} > /dev/null 2>&1 || true
    else
        echo -e "${RED}Skipping remaining tests - collector not running${NC}"
        ((FAILED_TESTS+=4))
        ((TOTAL_TESTS+=4))
    fi
}

# Function to test environment setup
test_environment() {
    echo -e "\n${BLUE}[Testing Environment]${NC}"
    
    # Test Docker
    run_test "Docker available" \
        "docker --version" \
        ""
    
    # Test required directories
    run_test "Configs directory" \
        "[ -d configs ]" \
        ""
    
    run_test "Scripts directory" \
        "[ -d scripts ]" \
        ""
    
    # Test env templates
    run_test "Environment templates" \
        "[ -d configs/env-templates ]" \
        ""
}

# Function to test New Relic connectivity
test_newrelic() {
    echo -e "\n${BLUE}[Testing New Relic Integration]${NC}"
    
    if [ -z "$NEW_RELIC_LICENSE_KEY" ]; then
        echo -e "${YELLOW}Warning: NEW_RELIC_LICENSE_KEY not set, skipping NR tests${NC}"
        return
    fi
    
    # Test with minimal config
    cat > /tmp/test-nr-config.yaml << EOF
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:
    timeout: 10s

exporters:
  otlp/newrelic:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: \${env:NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlp/newrelic]
EOF

    run_test "New Relic connectivity" \
        "docker run -d --name otel-test-nr --rm \
         -v /tmp/test-nr-config.yaml:/etc/otelcol/config.yaml \
         -e NEW_RELIC_LICENSE_KEY \
         -p 4317:4317 \
         otel/opentelemetry-collector-contrib:latest && \
         sleep 5 && \
         docker logs otel-test-nr 2>&1 | grep -q 'Everything is ready'" \
        ""
    
    # Cleanup
    docker stop otel-test-nr > /dev/null 2>&1 || true
    rm -f /tmp/test-nr-config.yaml
}

# Main test execution
echo -e "${YELLOW}Starting integration tests...${NC}"

# Always test environment first
test_environment

# Test New Relic if configured
if [ "$DATABASE" = "all" ] || [ "$DATABASE" = "newrelic" ]; then
    test_newrelic
fi

# Test databases
if [ "$DATABASE" = "all" ]; then
    for db in postgresql mysql mongodb mssql oracle; do
        test_database "$db"
    done
elif [ "$DATABASE" != "newrelic" ]; then
    test_database "$DATABASE"
fi

# Generate summary report
echo -e "\n${BLUE}=== Integration Test Summary ===${NC}"
echo -e "Total Tests: ${TOTAL_TESTS}"
echo -e "Passed: ${GREEN}${PASSED_TESTS}${NC}"
echo -e "Failed: ${RED}${FAILED_TESTS}${NC}"
echo -e "\nDetailed Results:"
echo -e "$TEST_RESULTS"

# Save test report
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
REPORT_FILE="integration_test_report_${TIMESTAMP}.txt"
cat > "$REPORT_FILE" << EOF
Database Intelligence Integration Test Report
Timestamp: $(date)
Target: $DATABASE

Summary:
- Total Tests: $TOTAL_TESTS
- Passed: $PASSED_TESTS
- Failed: $FAILED_TESTS
- Success Rate: $(echo "scale=2; $PASSED_TESTS * 100 / $TOTAL_TESTS" | bc)%

Detailed Results:
$TEST_RESULTS

Environment:
- Docker: $(docker --version)
- OS: $(uname -s)
- Working Directory: $(pwd)
EOF

echo -e "\n${YELLOW}Test report saved to: $REPORT_FILE${NC}"

# Exit with appropriate code
if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}✗ Some tests failed!${NC}"
    exit 1
fi