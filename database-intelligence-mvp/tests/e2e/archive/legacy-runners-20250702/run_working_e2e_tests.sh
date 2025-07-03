#!/bin/bash
# Working E2E Test Runner - Runs tests that actually work

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Database Intelligence MVP - Working E2E Tests${NC}"
echo -e "${BLUE}========================================${NC}"

# Function to check prerequisites
check_prerequisites() {
    echo -e "\n${YELLOW}Checking prerequisites...${NC}"
    
    # Check PostgreSQL
    if docker exec e2e-postgres psql -U postgres -d e2e_test -c "SELECT 1" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PostgreSQL is accessible${NC}"
    else
        echo -e "${RED}✗ PostgreSQL not accessible${NC}"
        exit 1
    fi
    
    # Check MySQL
    if docker exec e2e-mysql mysql -uroot -proot e2e_test -e "SELECT 1" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ MySQL is accessible${NC}"
    else
        echo -e "${RED}✗ MySQL not accessible${NC}"
        exit 1
    fi
    
    # Check collector
    if docker ps | grep -q e2e-collector; then
        echo -e "${GREEN}✓ Collector is running${NC}"
    else
        echo -e "${RED}✗ Collector not running${NC}"
        exit 1
    fi
    
    # Check collector output
    OUTPUT_SIZE=$(docker exec e2e-collector ls -la /var/lib/otel/e2e-output.json 2>/dev/null | awk '{print $5}' || echo "0")
    if [ "$OUTPUT_SIZE" != "0" ]; then
        echo -e "${GREEN}✓ Collector is writing data ($(numfmt --to=iec-i --suffix=B $OUTPUT_SIZE))${NC}"
    else
        echo -e "${YELLOW}⚠ No collector output yet${NC}"
    fi
}

# Function to run a test
run_test() {
    local test_name=$1
    local test_file=$2
    local test_pattern=$3
    
    echo -e "\n${YELLOW}Running: $test_name${NC}"
    
    if [ ! -f "$test_file" ]; then
        echo -e "${RED}✗ Test file not found: $test_file${NC}"
        return 1
    fi
    
    # Run test with proper tags and timeout
    if go test -v -run "$test_pattern" "$test_file" -tags=e2e -timeout=2m 2>&1 | tee test.log | grep -E "(PASS|FAIL|SKIP)"; then
        if grep -q "FAIL" test.log; then
            echo -e "${RED}✗ $test_name FAILED${NC}"
            return 1
        elif grep -q "SKIP" test.log; then
            echo -e "${YELLOW}⚠ $test_name SKIPPED${NC}"
            return 2
        else
            echo -e "${GREEN}✓ $test_name PASSED${NC}"
            return 0
        fi
    else
        echo -e "${RED}✗ $test_name ERROR${NC}"
        return 1
    fi
}

# Main execution
main() {
    check_prerequisites
    
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}Running Working E2E Tests${NC}"
    echo -e "${BLUE}========================================${NC}"
    
    cd tests/e2e || exit 1
    
    PASSED=0
    FAILED=0
    SKIPPED=0
    
    # Test 1: Basic E2E Pipeline
    if run_test "Basic E2E Pipeline" "real_e2e_test.go" "TestRealE2EPipeline"; then
        ((PASSED++))
    elif [ $? -eq 2 ]; then
        ((SKIPPED++))
    else
        ((FAILED++))
    fi
    
    # Test 2: Real Query Patterns
    if run_test "Real Query Patterns" "real_e2e_test.go" "TestRealQueryPatterns"; then
        ((PASSED++))
    elif [ $? -eq 2 ]; then
        ((SKIPPED++))
    else
        ((FAILED++))
    fi
    
    # Test 3: Database Error Scenarios
    if run_test "Database Error Scenarios" "real_e2e_test.go" "TestDatabaseErrorScenarios"; then
        ((PASSED++))
    elif [ $? -eq 2 ]; then
        ((SKIPPED++))
    else
        ((FAILED++))
    fi
    
    # Clean up
    rm -f test.log
    
    # Summary
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}Test Summary${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo -e "Passed:  ${GREEN}$PASSED${NC}"
    echo -e "Failed:  ${RED}$FAILED${NC}"
    echo -e "Skipped: ${YELLOW}$SKIPPED${NC}"
    echo -e "Total:   $((PASSED + FAILED + SKIPPED))"
    
    # Collector metrics summary
    echo -e "\n${YELLOW}Collector Metrics Summary:${NC}"
    
    # PostgreSQL metrics
    PG_METRICS=$(docker exec e2e-collector tail -1000 /var/lib/otel/e2e-output.json 2>/dev/null | \
        jq -r '.resourceMetrics[]?.scopeMetrics[]?.metrics[]?.name' 2>/dev/null | \
        grep -c "postgresql" || echo "0")
    echo -e "PostgreSQL metrics collected: ${GREEN}$PG_METRICS${NC}"
    
    # MySQL metrics  
    MYSQL_METRICS=$(docker exec e2e-collector tail -1000 /var/lib/otel/e2e-output.json 2>/dev/null | \
        jq -r '.resourceMetrics[]?.scopeMetrics[]?.metrics[]?.name' 2>/dev/null | \
        grep -c "mysql" || echo "0")
    echo -e "MySQL metrics collected: ${GREEN}$MYSQL_METRICS${NC}"
    
    # Output file size
    OUTPUT_SIZE=$(docker exec e2e-collector ls -la /var/lib/otel/e2e-output.json 2>/dev/null | awk '{print $5}' || echo "0")
    echo -e "Output file size: ${GREEN}$(numfmt --to=iec-i --suffix=B $OUTPUT_SIZE)${NC}"
    
    # Recent metrics
    echo -e "\n${YELLOW}Recent Metrics (last 5):${NC}"
    docker exec e2e-collector tail -100 /var/lib/otel/e2e-output.json 2>/dev/null | \
        jq -r '.resourceMetrics[]?.scopeMetrics[]?.metrics[]?.name' 2>/dev/null | \
        tail -5 | sed 's/^/  /'
    
    if [ $FAILED -eq 0 ]; then
        echo -e "\n${GREEN}✅ All working tests passed!${NC}"
        return 0
    else
        echo -e "\n${RED}❌ Some tests failed${NC}"
        return 1
    fi
}

# Run main function
main "$@"