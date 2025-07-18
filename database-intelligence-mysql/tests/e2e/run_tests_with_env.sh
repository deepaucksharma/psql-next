#!/bin/bash

# Load environment variables and run E2E tests
# This script ensures all New Relic credentials are properly loaded

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== MySQL Wait-Based Monitoring E2E Test Runner ===${NC}"

# Check if .env file exists
ENV_FILE="../../.env"
if [ ! -f "$ENV_FILE" ]; then
    echo -e "${RED}Error: .env file not found at $ENV_FILE${NC}"
    exit 1
fi

# Load environment variables
echo -e "${YELLOW}Loading environment variables...${NC}"
export $(cat "$ENV_FILE" | grep -v '^#' | xargs)

# Verify required environment variables
required_vars=(
    "NEW_RELIC_LICENSE_KEY"
    "NEW_RELIC_ACCOUNT_ID"
    "NEW_RELIC_API_KEY"
    "MYSQL_HOST"
    "MYSQL_USER"
    "MYSQL_PASSWORD"
)

for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo -e "${RED}Error: Required environment variable $var is not set${NC}"
        exit 1
    fi
done

echo -e "${GREEN}✓ All required environment variables loaded${NC}"

# Create test configuration with actual credentials
cat > test_config.yaml << EOF
# Test configuration with actual New Relic credentials
newrelic:
  license_key: ${NEW_RELIC_LICENSE_KEY}
  account_id: ${NEW_RELIC_ACCOUNT_ID}
  api_key: ${NEW_RELIC_API_KEY}
  endpoint: ${NEW_RELIC_OTLP_ENDPOINT}

mysql:
  host: ${MYSQL_HOST}
  port: ${MYSQL_PORT}
  user: ${MYSQL_USER}
  password: ${MYSQL_PASSWORD}
  database: ${MYSQL_DATABASE}

collector:
  edge_port: ${EDGE_COLLECTOR_PORT:-8888}
  gateway_port: ${GATEWAY_COLLECTOR_PORT:-4317}
  prometheus_port: ${PROMETHEUS_GATEWAY_PORT:-9091}
EOF

echo -e "${YELLOW}Running comprehensive E2E tests...${NC}"

# Run different test suites based on argument
case "${1:-all}" in
    "quick")
        echo -e "${BLUE}Running quick validation tests...${NC}"
        go test -v -short -tags=integration ./...
        ;;
    "coverage")
        echo -e "${BLUE}Running dashboard coverage tests...${NC}"
        go test -v -run TestDashboardMetricCoverage ./dashboard_coverage_test.go
        go test -v -run TestAdvisoryAccuracy ./dashboard_coverage_test.go
        go test -v -run TestDataQualityValidation ./dashboard_coverage_test.go
        ;;
    "performance")
        echo -e "${BLUE}Running performance validation tests...${NC}"
        go test -v -run TestMonitoringPerformanceImpact ./performance_validation_test.go
        go test -v -run TestRegressionDetection ./performance_validation_test.go
        ;;
    "comprehensive")
        echo -e "${BLUE}Running comprehensive validation test...${NC}"
        go test -v -run TestComprehensiveE2EValidation ./comprehensive_validation_test.go
        ;;
    "data-gen")
        echo -e "${BLUE}Running data generation test...${NC}"
        go test -v -run TestDataGeneration ./data_generator_test.go
        ;;
    "all")
        echo -e "${BLUE}Running all E2E tests...${NC}"
        
        # Initialize test results
        FAILED_TESTS=0
        
        # Run each test suite
        test_suites=(
            "TestComprehensiveE2EValidation"
            "TestDashboardMetricCoverage"
            "TestAdvisoryAccuracy"
            "TestDataQualityValidation"
            "TestMonitoringPerformanceImpact"
        )
        
        for suite in "${test_suites[@]}"; do
            echo -e "\n${YELLOW}▶ Running $suite...${NC}"
            if go test -v -run "$suite" -timeout 30m ./...; then
                echo -e "${GREEN}✓ $suite passed${NC}"
            else
                echo -e "${RED}✗ $suite failed${NC}"
                ((FAILED_TESTS++))
            fi
        done
        
        # Summary
        echo -e "\n${CYAN}=== Test Summary ===${NC}"
        if [ $FAILED_TESTS -eq 0 ]; then
            echo -e "${GREEN}✓ All tests passed!${NC}"
            exit 0
        else
            echo -e "${RED}✗ $FAILED_TESTS test(s) failed${NC}"
            exit 1
        fi
        ;;
    *)
        echo "Usage: $0 {all|quick|coverage|performance|comprehensive|data-gen}"
        echo ""
        echo "  all           - Run all E2E tests (default)"
        echo "  quick         - Run quick validation tests only"
        echo "  coverage      - Run dashboard and metric coverage tests"
        echo "  performance   - Run performance impact tests"
        echo "  comprehensive - Run comprehensive E2E validation"
        echo "  data-gen      - Run data generation tests"
        exit 1
        ;;
esac

# Cleanup
rm -f test_config.yaml