#!/bin/bash
# Comprehensive E2E Test Runner - Runs all available tests with proper error handling

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Database Intelligence MVP - Comprehensive E2E Tests${NC}"
echo -e "${BLUE}========================================${NC}"

# Function to check prerequisites
check_prerequisites() {
    echo -e "\n${YELLOW}Checking prerequisites...${NC}"
    
    # Check PostgreSQL
    if docker exec e2e-postgres psql -U postgres -d e2e_test -c "SELECT 1" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PostgreSQL is accessible${NC}"
    else
        echo -e "${RED}✗ PostgreSQL not accessible${NC}"
        echo -e "${YELLOW}Run: docker-compose -f tests/e2e/docker-compose.e2e.yml up -d${NC}"
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
}

# Function to run a test file
run_test_file() {
    local test_name=$1
    local test_file=$2
    local test_pattern=$3
    
    echo -e "\n${YELLOW}Running: $test_name${NC}"
    
    if [ ! -f "$test_file" ]; then
        echo -e "${RED}✗ Test file not found: $test_file${NC}"
        return 1
    fi
    
    # Run test with proper error handling
    if go test -v -run "$test_pattern" "$test_file" -tags=e2e -timeout=2m > test_output.log 2>&1; then
        echo -e "${GREEN}✓ $test_name PASSED${NC}"
        return 0
    else
        # Check if tests were skipped
        if grep -q "no tests to run" test_output.log; then
            echo -e "${YELLOW}⚠ $test_name SKIPPED (no tests found)${NC}"
            return 2
        else
            echo -e "${RED}✗ $test_name FAILED${NC}"
            # Show error details
            grep -E "(FAIL|Error:|panic:|--- FAIL)" test_output.log | head -10
            return 1
        fi
    fi
}

# Main execution
main() {
    check_prerequisites
    
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}Running Comprehensive E2E Test Suite${NC}"
    echo -e "${BLUE}========================================${NC}"
    
    cd tests/e2e || exit 1
    
    PASSED=0
    FAILED=0
    SKIPPED=0
    
    # Test Suite 1: Basic E2E Tests
    echo -e "\n${BLUE}=== Basic E2E Tests ===${NC}"
    if run_test_file "Real E2E Pipeline" "real_e2e_test.go" "TestRealE2EPipeline"; then
        ((PASSED++))
    elif [ $? -eq 2 ]; then
        ((SKIPPED++))
    else
        ((FAILED++))
    fi
    
    if run_test_file "Real Query Patterns" "real_e2e_test.go" "TestRealQueryPatterns"; then
        ((PASSED++))
    elif [ $? -eq 2 ]; then
        ((SKIPPED++))
    else
        ((FAILED++))
    fi
    
    if run_test_file "Database Error Scenarios" "real_e2e_test.go" "TestDatabaseErrorScenarios"; then
        ((PASSED++))
    elif [ $? -eq 2 ]; then
        ((SKIPPED++))
    else
        ((FAILED++))
    fi
    
    # Test Suite 2: Processor Validation (Limited due to infrastructure)
    echo -e "\n${BLUE}=== Processor Validation Tests ===${NC}"
    echo -e "${YELLOW}Note: Running with limited validation due to Prometheus endpoint issues${NC}"
    
    if run_test_file "Processor Validation" "processor_validation_test.go" "TestCustomProcessorValidation"; then
        ((PASSED++))
    elif [ $? -eq 2 ]; then
        ((SKIPPED++))
    else
        ((FAILED++))
    fi
    
    # Test Suite 3: Other Test Files (if they exist and are runnable)
    echo -e "\n${BLUE}=== Additional Test Suites ===${NC}"
    
    for test_file in security_pii_test.go performance_scale_test.go error_scenarios_test.go; do
        if [ -f "$test_file" ]; then
            test_name=$(echo $test_file | sed 's/_test.go//' | sed 's/_/ /g' | awk '{for(i=1;i<=NF;i++) $i=toupper(substr($i,1,1)) substr($i,2)} 1')
            if run_test_file "$test_name" "$test_file" "Test.*"; then
                ((PASSED++))
            elif [ $? -eq 2 ]; then
                ((SKIPPED++))
            else
                ((FAILED++))
            fi
        fi
    done
    
    # Clean up
    rm -f test_output.log
    
    # Collector Status Check
    echo -e "\n${BLUE}=== Collector Status ===${NC}"
    
    # Check output file
    if docker exec e2e-collector test -f /var/lib/otel/e2e-output.json; then
        OUTPUT_SIZE=$(docker exec e2e-collector ls -la /var/lib/otel/e2e-output.json | awk '{print $5}')
        echo -e "${GREEN}✓ Collector output file exists ($(echo $OUTPUT_SIZE | awk '{printf "%.2f MB", $1/1024/1024}'))${NC}"
        
        # Count metrics
        PG_COUNT=$(docker exec e2e-collector tail -1000 /var/lib/otel/e2e-output.json 2>/dev/null | grep -c "postgresql" || echo "0")
        MYSQL_COUNT=$(docker exec e2e-collector tail -1000 /var/lib/otel/e2e-output.json 2>/dev/null | grep -c "mysql" || echo "0")
        
        echo -e "PostgreSQL metrics in recent output: ${GREEN}$PG_COUNT${NC}"
        echo -e "MySQL metrics in recent output: ${GREEN}$MYSQL_COUNT${NC}"
    else
        echo -e "${RED}✗ Collector output file not found${NC}"
    fi
    
    # Summary
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}Test Summary${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo -e "Passed:  ${GREEN}$PASSED${NC}"
    echo -e "Failed:  ${RED}$FAILED${NC}"
    echo -e "Skipped: ${YELLOW}$SKIPPED${NC}"
    echo -e "Total:   $((PASSED + FAILED + SKIPPED))"
    
    # Known Issues
    echo -e "\n${YELLOW}Known Infrastructure Issues:${NC}"
    echo -e "1. Prometheus endpoint (port 8890) not responding - metrics validation limited"
    echo -e "2. MySQL performance_schema permissions - some MySQL metrics unavailable"
    echo -e "3. File exporter only writing metrics, not logs - processor validation limited"
    
    # Recommendations
    echo -e "\n${BLUE}Recommendations:${NC}"
    echo -e "1. Review ${BLUE}comprehensive_test_report.md${NC} for detailed findings"
    echo -e "2. Fix infrastructure issues listed above for full test coverage"
    echo -e "3. Run individual test files for more detailed output"
    
    if [ $FAILED -eq 0 ]; then
        echo -e "\n${GREEN}✅ All runnable tests passed!${NC}"
        return 0
    else
        echo -e "\n${RED}❌ Some tests failed${NC}"
        return 1
    fi
}

# Run main function
main "$@"