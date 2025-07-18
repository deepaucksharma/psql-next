#!/bin/bash
# Unified test runner for Database Intelligence

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test type from argument
TEST_TYPE=${1:-all}
DATABASE=${2:-all}

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo -e "${BLUE}=== Database Intelligence Test Runner ===${NC}"
echo -e "Test Type: ${YELLOW}$TEST_TYPE${NC}"
echo -e "Database: ${YELLOW}$DATABASE${NC}"
echo ""

# Test results tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Function to run a test suite
run_test_suite() {
    local suite_name=$1
    local test_command=$2
    
    echo -e "${BLUE}Running $suite_name tests...${NC}"
    ((TOTAL_TESTS++))
    
    if eval "$test_command"; then
        echo -e "${GREEN}✓ $suite_name tests passed${NC}"
        ((PASSED_TESTS++))
    else
        echo -e "${RED}✗ $suite_name tests failed${NC}"
        ((FAILED_TESTS++))
    fi
}

# Run tests based on type
case "$TEST_TYPE" in
    unit)
        echo -e "${YELLOW}Running unit tests...${NC}"
        run_test_suite "Configuration validation" "$ROOT_DIR/scripts/validation/validate-config.sh"
        run_test_suite "Metric naming" "$ROOT_DIR/scripts/validation/validate-metric-naming.sh"
        ;;
    
    integration)
        echo -e "${YELLOW}Running integration tests...${NC}"
        if [ "$DATABASE" = "all" ]; then
            for db in postgresql mysql mongodb mssql oracle; do
                run_test_suite "$db integration" "$ROOT_DIR/scripts/testing/test-database-config.sh $db 30"
            done
        else
            run_test_suite "$DATABASE integration" "$ROOT_DIR/scripts/testing/test-database-config.sh $DATABASE 30"
        fi
        ;;
    
    e2e)
        echo -e "${YELLOW}Running end-to-end tests...${NC}"
        run_test_suite "E2E validation" "$ROOT_DIR/scripts/validation/validate-e2e.sh"
        run_test_suite "Integration test" "$ROOT_DIR/scripts/testing/test-integration.sh $DATABASE"
        ;;
    
    performance)
        echo -e "${YELLOW}Running performance tests...${NC}"
        if [ "$DATABASE" != "all" ]; then
            run_test_suite "$DATABASE performance" "$ROOT_DIR/scripts/testing/benchmark-performance.sh $DATABASE 60"
            run_test_suite "$DATABASE cardinality" "$ROOT_DIR/scripts/testing/check-metric-cardinality.sh $DATABASE"
        else
            echo -e "${RED}Please specify a database for performance testing${NC}"
            exit 1
        fi
        ;;
    
    all)
        echo -e "${YELLOW}Running all test suites...${NC}"
        # Run all test types
        $0 unit $DATABASE
        $0 integration $DATABASE
        $0 e2e $DATABASE
        ;;
    
    *)
        echo -e "${RED}Unknown test type: $TEST_TYPE${NC}"
        echo "Usage: $0 [unit|integration|e2e|performance|all] [database]"
        exit 1
        ;;
esac

# Summary
echo -e "\n${BLUE}=== Test Summary ===${NC}"
echo -e "Total Tests: $TOTAL_TESTS"
echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed: ${RED}$FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}✗ Some tests failed!${NC}"
    exit 1
fi
