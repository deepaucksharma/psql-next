#!/bin/bash

# Validate E2E Test Setup Script
# This script checks if all E2E test prerequisites are met

echo "=== E2E Test Setup Validation ==="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Track issues
ISSUES=0

# Function to check and report
check_status() {
    local test_name="$1"
    local condition="$2"
    local fix_hint="$3"
    
    if eval "$condition"; then
        echo -e "${GREEN}✓${NC} $test_name"
    else
        echo -e "${RED}✗${NC} $test_name"
        if [ -n "$fix_hint" ]; then
            echo -e "  ${YELLOW}Fix:${NC} $fix_hint"
        fi
        ((ISSUES++))
    fi
}

echo -e "\n${YELLOW}1. Checking Environment${NC}"
check_status "Go installed" "command -v go >/dev/null 2>&1" "Install Go from https://golang.org"
check_status "Docker installed" "command -v docker >/dev/null 2>&1" "Install Docker from https://docker.com"
check_status "Make installed" "command -v make >/dev/null 2>&1" "Install make via package manager"

echo -e "\n${YELLOW}2. Checking Build Tools${NC}"
check_status "OCB/builder installed" "command -v builder >/dev/null 2>&1 || [ -f ~/go/bin/builder ]" "Run 'make install-tools'"

echo -e "\n${YELLOW}3. Checking Test Environment${NC}"
check_status "Project root exists" "[ -d ./tests/e2e ]" "Run from project root directory"
check_status "Test configs exist" "[ -f ./tests/e2e/config/e2e-test-collector-simple.yaml ]" "Missing test configuration files"
check_status "SQL init scripts exist" "[ -f ./tests/e2e/sql/postgres-init.sql ] && [ -f ./tests/e2e/sql/mysql-init.sql ]" "Missing database initialization scripts"

echo -e "\n${YELLOW}4. Checking Collector Binary${NC}"
check_status "Collector binary exists" "[ -f ./dist/database-intelligence-collector ]" "Run 'make build'"
if [ -f ./dist/database-intelligence-collector ]; then
    check_status "Collector is executable" "[ -x ./dist/database-intelligence-collector ]" "Run 'chmod +x ./dist/database-intelligence-collector'"
fi

echo -e "\n${YELLOW}5. Checking Test Scripts${NC}"
check_status "Basic test runner exists" "[ -f ./tests/e2e/run-e2e-tests.sh ]" "Missing test runner script"
check_status "Comprehensive test runner exists" "[ -f ./tests/e2e/run-comprehensive-e2e-tests.sh ]" "Missing comprehensive test runner"
check_status "Local test runner exists" "[ -f ./tests/e2e/run-local-e2e-tests.sh ]" "Missing local test runner"

echo -e "\n${YELLOW}6. Checking New Relic Credentials (Optional)${NC}"
if [ -n "$NEW_RELIC_LICENSE_KEY" ]; then
    echo -e "${GREEN}✓${NC} NEW_RELIC_LICENSE_KEY is set"
else
    echo -e "${YELLOW}!${NC} NEW_RELIC_LICENSE_KEY not set (local testing only)"
fi

if [ -n "$NEW_RELIC_ACCOUNT_ID" ]; then
    echo -e "${GREEN}✓${NC} NEW_RELIC_ACCOUNT_ID is set"
else
    echo -e "${YELLOW}!${NC} NEW_RELIC_ACCOUNT_ID not set (NRDB validation disabled)"
fi

echo -e "\n${YELLOW}7. Checking Database Connectivity${NC}"
if nc -z localhost 5432 2>/dev/null; then
    echo -e "${GREEN}✓${NC} PostgreSQL is reachable on localhost:5432"
else
    echo -e "${YELLOW}!${NC} PostgreSQL not reachable (will start test containers)"
fi

if nc -z localhost 3306 2>/dev/null; then
    echo -e "${GREEN}✓${NC} MySQL is reachable on localhost:3306"
else
    echo -e "${YELLOW}!${NC} MySQL not reachable (will start test containers)"
fi

echo -e "\n${YELLOW}8. Checking Module Consistency${NC}"
MODULE_NAME=$(grep "^module" go.mod | awk '{print $2}')
check_status "go.mod module: $MODULE_NAME" "[ -n '$MODULE_NAME' ]" "Invalid go.mod file"

# Check if processors are properly referenced
if grep -q "$MODULE_NAME/processors" go.mod; then
    echo -e "${GREEN}✓${NC} Processor modules properly referenced"
else
    echo -e "${YELLOW}!${NC} Processor modules may have incorrect paths"
fi

echo -e "\n${YELLOW}9. Summary${NC}"
if [ $ISSUES -eq 0 ]; then
    echo -e "${GREEN}✓ All E2E test prerequisites are met!${NC}"
    echo ""
    echo "You can now run E2E tests with:"
    echo "  - Local testing: ./tests/e2e/run-local-e2e-tests.sh"
    echo "  - Basic E2E: ./tests/e2e/run-e2e-tests.sh"
    echo "  - Comprehensive: ./tests/e2e/run-comprehensive-e2e-tests.sh"
else
    echo -e "${RED}✗ Found $ISSUES issues that need to be fixed${NC}"
    echo ""
    echo "Fix the issues above, then run this script again."
fi

exit $ISSUES