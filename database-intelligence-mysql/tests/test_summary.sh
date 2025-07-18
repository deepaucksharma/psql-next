#!/bin/bash

# MySQL Wait-Based Monitoring - Test Summary Script
# Provides a comprehensive overview of the E2E testing setup

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${BLUE}==================================================================${NC}"
echo -e "${BLUE}     MySQL Wait-Based Monitoring - E2E Test Summary${NC}"
echo -e "${BLUE}==================================================================${NC}"
echo ""

# Check environment setup
echo -e "${CYAN}1. Environment Setup${NC}"
echo -e "${CYAN}-------------------${NC}"

# Check .env file
if [ -f "../.env" ]; then
    echo -e "${GREEN}✓${NC} Environment file (.env) found"
    # Count configured variables
    var_count=$(grep -v '^#' ../. | grep -v '^$' | wc -l)
    echo -e "  └─ ${var_count} variables configured"
else
    echo -e "${RED}✗${NC} Environment file (.env) NOT found"
fi

# Check required environment variables
echo -e "\n${CYAN}2. Required Credentials${NC}"
echo -e "${CYAN}----------------------${NC}"

check_env_var() {
    local var_name=$1
    local var_value="${!var_name}"
    
    if [ -n "$var_value" ]; then
        if [[ "$var_name" == *"KEY"* ]]; then
            # Mask sensitive values
            echo -e "${GREEN}✓${NC} $var_name: ${var_value:0:10}...${var_value: -4}"
        else
            echo -e "${GREEN}✓${NC} $var_name: $var_value"
        fi
    else
        echo -e "${RED}✗${NC} $var_name: NOT SET"
    fi
}

# Load .env if exists
if [ -f "../.env" ]; then
    export $(cat ../.env | grep -v '^#' | xargs) 2>/dev/null
fi

check_env_var "NEW_RELIC_LICENSE_KEY"
check_env_var "NEW_RELIC_ACCOUNT_ID"
check_env_var "NEW_RELIC_API_KEY"
check_env_var "MYSQL_HOST"
check_env_var "MYSQL_USER"

# Test suite overview
echo -e "\n${CYAN}3. Available Test Suites${NC}"
echo -e "${CYAN}-----------------------${NC}"

test_files=(
    "comprehensive_validation_test.go:Comprehensive E2E validation of entire pipeline"
    "dashboard_coverage_test.go:Dashboard and metric coverage validation"
    "performance_validation_test.go:Performance impact and regression testing"
    "data_generator.go:Realistic workload generation (no mocks)"
    "basic_validation_test.go:Basic connectivity and setup validation"
)

for test_info in "${test_files[@]}"; do
    IFS=':' read -r file desc <<< "$test_info"
    if [ -f "e2e/$file" ]; then
        echo -e "${GREEN}✓${NC} $file"
        echo -e "  └─ $desc"
    else
        echo -e "${RED}✗${NC} $file (NOT FOUND)"
    fi
done

# Test capabilities
echo -e "\n${CYAN}4. Test Capabilities${NC}"
echo -e "${CYAN}-------------------${NC}"

capabilities=(
    "MySQL Performance Schema validation"
    "Wait-based metric collection"
    "Advisory generation and accuracy"
    "New Relic NRDB data validation"
    "Dashboard widget coverage (95%+)"
    "Performance overhead measurement"
    "End-to-end latency tracking"
    "Realistic workload generation"
    "Data quality validation"
    "Regression detection"
)

for cap in "${capabilities[@]}"; do
    echo -e "${GREEN}✓${NC} $cap"
done

# Workload patterns
echo -e "\n${CYAN}5. Workload Patterns Generated${NC}"
echo -e "${CYAN}-----------------------------${NC}"

patterns=(
    "IO-Intensive:Full table scans, missing indexes"
    "Lock-Intensive:Transaction conflicts, deadlocks"
    "CPU-Intensive:Complex aggregations, joins"
    "Slow Queries:Long-running operations"
    "Mixed Workload:Combination of all patterns"
)

for pattern_info in "${patterns[@]}"; do
    IFS=':' read -r pattern desc <<< "$pattern_info"
    echo -e "${YELLOW}▸${NC} ${pattern}: $desc"
done

# Metrics validated
echo -e "\n${CYAN}6. Key Metrics Validated${NC}"
echo -e "${CYAN}-----------------------${NC}"

metrics=(
    "mysql.query.wait_profile - Query wait time analysis"
    "mysql.blocking.active - Active blocking sessions"
    "mysql.advisor.* - Performance advisories"
    "mysql.statement.digest - Statement performance"
    "mysql.current.waits - Real-time wait events"
)

for metric in "${metrics[@]}"; do
    echo -e "${BLUE}◆${NC} $metric"
done

# Performance baselines
echo -e "\n${CYAN}7. Performance Baselines${NC}"
echo -e "${CYAN}-----------------------${NC}"

echo -e "CPU Overhead:        ${YELLOW}<1%${NC}"
echo -e "Memory Usage:        ${YELLOW}<384MB${NC}"
echo -e "Query Overhead:      ${YELLOW}<0.5ms${NC}"
echo -e "Collection Latency:  ${YELLOW}<5ms${NC}"
echo -e "E2E Latency:         ${YELLOW}<90s${NC}"

# Quick commands
echo -e "\n${CYAN}8. Quick Test Commands${NC}"
echo -e "${CYAN}---------------------${NC}"

echo -e "${YELLOW}Basic validation:${NC}"
echo "  ./run_basic_test.sh"

echo -e "\n${YELLOW}All E2E tests:${NC}"
echo "  cd e2e && ./run_tests_with_env.sh all"

echo -e "\n${YELLOW}Specific test suite:${NC}"
echo "  ./run_tests_with_env.sh coverage      # Dashboard coverage"
echo "  ./run_tests_with_env.sh performance   # Performance impact"
echo "  ./run_tests_with_env.sh comprehensive # Full validation"

echo -e "\n${YELLOW}Generate test report:${NC}"
echo "  ./run_all_tests.sh"

# Check current status
echo -e "\n${CYAN}9. Current System Status${NC}"
echo -e "${CYAN}-----------------------${NC}"

# Check if MySQL is accessible
if command -v mysql &> /dev/null; then
    if mysql -h"${MYSQL_HOST:-localhost}" -u"${MYSQL_USER:-root}" -p"${MYSQL_PASSWORD:-rootpassword}" -e "SELECT 1" &>/dev/null; then
        echo -e "${GREEN}✓${NC} MySQL is accessible"
    else
        echo -e "${RED}✗${NC} MySQL is NOT accessible"
    fi
else
    echo -e "${YELLOW}⚠${NC} mysql client not found"
fi

# Check collector endpoints
check_endpoint() {
    local name=$1
    local url=$2
    
    if curl -s -f "$url" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} $name is running"
    else
        echo -e "${RED}✗${NC} $name is NOT accessible"
    fi
}

check_endpoint "Edge Collector metrics" "http://localhost:8888/metrics"
check_endpoint "Gateway Prometheus" "http://localhost:9091/metrics"
check_endpoint "Collector health" "http://localhost:13133/health"

echo -e "\n${CYAN}10. Next Steps${NC}"
echo -e "${CYAN}-------------${NC}"

echo "1. Ensure MySQL and collectors are running"
echo "2. Load environment variables: source ../.env"
echo "3. Run basic validation: ./run_basic_test.sh"
echo "4. Run full E2E tests: cd e2e && ./run_tests_with_env.sh all"
echo "5. Check test reports in test-reports/ directory"

echo -e "\n${BLUE}==================================================================${NC}"
echo -e "For detailed instructions, see: ${YELLOW}E2E_TEST_GUIDE.md${NC}"
echo -e "${BLUE}==================================================================${NC}"