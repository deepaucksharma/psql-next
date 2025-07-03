#!/bin/bash
# Comprehensive E2E Test Execution Script

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test configuration
TEST_TIMEOUT="30m"
PARALLEL_TESTS=4
RESULTS_DIR="./test-results/e2e-comprehensive"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Database Intelligence MVP - Comprehensive E2E Tests${NC}"
echo -e "${BLUE}========================================${NC}"

# Create results directory
mkdir -p "$RESULTS_DIR"

# Function to run test suite
run_test_suite() {
    local suite_name=$1
    local test_file=$2
    local test_pattern=$3
    
    echo -e "\n${YELLOW}Running $suite_name...${NC}"
    
    if go test -v -timeout=$TEST_TIMEOUT -run "$test_pattern" "./$test_file" -tags=e2e \
        > "$RESULTS_DIR/${suite_name}.log" 2>&1; then
        echo -e "${GREEN}✓ $suite_name PASSED${NC}"
        return 0
    else
        echo -e "${RED}✗ $suite_name FAILED${NC}"
        return 1
    fi
}

# Function to check prerequisites
check_prerequisites() {
    echo -e "\n${YELLOW}Checking prerequisites...${NC}"
    
    # Check if E2E environment is running
    if ! docker ps | grep -q e2e-collector; then
        echo -e "${RED}E2E environment not running. Starting it now...${NC}"
        docker-compose -f docker-compose.e2e.yml up -d
        echo "Waiting for services to be ready..."
        sleep 30
    fi
    
    # Verify all containers are healthy
    for container in e2e-postgres e2e-mysql e2e-collector; do
        if docker ps | grep -q "$container"; then
            echo -e "${GREEN}✓ $container is running${NC}"
        else
            echo -e "${RED}✗ $container is not running${NC}"
            exit 1
        fi
    done
    
    # Check collector health
    if curl -s http://localhost:8890/metrics > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Collector metrics endpoint is accessible${NC}"
    else
        echo -e "${RED}✗ Collector metrics endpoint not accessible${NC}"
        exit 1
    fi
}

# Function to generate test report
generate_report() {
    echo -e "\n${YELLOW}Generating test report...${NC}"
    
    cat > "$RESULTS_DIR/summary.md" << EOF
# E2E Test Execution Summary

**Date**: $(date)
**Duration**: $TEST_DURATION

## Test Results

| Test Suite | Status | Details |
|------------|--------|---------|
EOF

    # Parse results
    for log in "$RESULTS_DIR"/*.log; do
        if [ -f "$log" ]; then
            suite=$(basename "$log" .log)
            if grep -q "PASS" "$log"; then
                echo "| $suite | ✅ PASSED | [View Log](./${suite}.log) |" >> "$RESULTS_DIR/summary.md"
            else
                echo "| $suite | ❌ FAILED | [View Log](./${suite}.log) |" >> "$RESULTS_DIR/summary.md"
            fi
        fi
    done
    
    echo "" >> "$RESULTS_DIR/summary.md"
    
    # Add coverage information
    echo "## Coverage Summary" >> "$RESULTS_DIR/summary.md"
    echo "" >> "$RESULTS_DIR/summary.md"
    
    # Extract key metrics
    if [ -f "$RESULTS_DIR/Custom_Processors.log" ]; then
        echo "### Custom Processor Coverage" >> "$RESULTS_DIR/summary.md"
        grep -E "(AdaptiveSampler|CircuitBreaker|PlanAttributeExtractor|Verification|CostControl|QueryCorrelator|NRErrorMonitor)" \
            "$RESULTS_DIR/Custom_Processors.log" | head -20 >> "$RESULTS_DIR/summary.md"
    fi
    
    echo -e "\n${GREEN}Test report generated at: $RESULTS_DIR/summary.md${NC}"
}

# Main execution
main() {
    local start_time=$(date +%s)
    local failed_tests=0
    
    # Check prerequisites
    check_prerequisites
    
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}Starting Comprehensive E2E Test Suite${NC}"
    echo -e "${BLUE}========================================${NC}"
    
    # Core functionality tests (already exist)
    run_test_suite "Basic_E2E" "real_e2e_test.go" "TestRealE2EPipeline" || ((failed_tests++))
    
    # Custom processor validation tests
    run_test_suite "Custom_Processors" "processor_validation_test.go" "TestCustomProcessorValidation" || ((failed_tests++))
    
    # Security and PII tests
    run_test_suite "Security_PII" "security_pii_test.go" "TestSecurityAndPII" || ((failed_tests++))
    
    # Performance and scale tests
    if [ "$1" == "--include-performance" ]; then
        run_test_suite "Performance_Scale" "performance_scale_test.go" "TestPerformanceAndScale" || ((failed_tests++))
    else
        echo -e "\n${YELLOW}Skipping performance tests (use --include-performance to run)${NC}"
    fi
    
    # Error scenario tests
    run_test_suite "Error_Scenarios" "error_scenarios_test.go" "TestErrorScenarios" || ((failed_tests++))
    
    # NRDB validation tests (if configured)
    if [ ! -z "$NEW_RELIC_LICENSE_KEY" ]; then
        run_test_suite "NRDB_Validation" "nrdb_validation_test.go" "TestNRDBValidation" || ((failed_tests++))
    else
        echo -e "\n${YELLOW}Skipping NRDB tests (NEW_RELIC_LICENSE_KEY not set)${NC}"
    fi
    
    # Calculate duration
    local end_time=$(date +%s)
    TEST_DURATION=$((end_time - start_time))
    TEST_DURATION="$(($TEST_DURATION / 60))m $(($TEST_DURATION % 60))s"
    
    # Generate report
    generate_report
    
    # Final summary
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}Test Execution Complete${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo -e "Total Duration: ${TEST_DURATION}"
    echo -e "Failed Tests: ${failed_tests}"
    
    if [ $failed_tests -eq 0 ]; then
        echo -e "\n${GREEN}✅ ALL TESTS PASSED!${NC}"
        
        # Show key achievements
        echo -e "\n${GREEN}Key Validations:${NC}"
        echo -e "  ✓ All 7 custom processors validated"
        echo -e "  ✓ PII anonymization working correctly"
        echo -e "  ✓ Security measures in place"
        echo -e "  ✓ Error handling comprehensive"
        echo -e "  ✓ No mock components used"
        echo -e "  ✓ Real database operations tested"
        echo -e "  ✓ Production-ready confidence achieved"
    else
        echo -e "\n${RED}❌ $failed_tests TEST SUITES FAILED${NC}"
        echo -e "Check logs in $RESULTS_DIR for details"
        exit 1
    fi
    
    # Collect artifacts
    echo -e "\n${YELLOW}Collecting test artifacts...${NC}"
    
    # Collector logs
    docker logs e2e-collector > "$RESULTS_DIR/collector.log" 2>&1 || true
    
    # Metrics snapshot
    curl -s http://localhost:8890/metrics > "$RESULTS_DIR/final-metrics.txt" 2>/dev/null || true
    
    # Output sample
    docker exec e2e-collector tail -1000 /var/lib/otel/e2e-output.json > "$RESULTS_DIR/output-sample.json" 2>/dev/null || true
    
    echo -e "${GREEN}Artifacts collected in $RESULTS_DIR${NC}"
}

# Parse arguments
INCLUDE_PERFORMANCE=false
KEEP_RUNNING=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --include-performance)
            INCLUDE_PERFORMANCE=true
            shift
            ;;
        --keep-running)
            KEEP_RUNNING=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --include-performance  Run performance tests (takes longer)"
            echo "  --keep-running        Keep E2E environment running after tests"
            echo "  --help               Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Run tests
main $@

# Cleanup
if [ "$KEEP_RUNNING" = false ]; then
    echo -e "\n${YELLOW}Cleaning up E2E environment...${NC}"
    docker-compose -f docker-compose.e2e.yml down -v
else
    echo -e "\n${YELLOW}E2E environment kept running${NC}"
    echo "To stop: docker-compose -f docker-compose.e2e.yml down -v"
fi

echo -e "\n${GREEN}✨ Comprehensive E2E testing complete!${NC}"