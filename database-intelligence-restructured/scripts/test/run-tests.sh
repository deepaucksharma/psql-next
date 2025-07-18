#!/bin/bash
# Unified test runner for Database Intelligence

set -e

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$SCRIPT_DIR/../utils/common.sh"

# Test configuration
TEST_TYPE=${1:-all}
DATABASE=${2:-all}
VERBOSE=${VERBOSE:-false}

# Show usage
usage() {
    cat << EOF
Database Intelligence Test Runner

Usage: $0 [test-type] [database] [options]

Test Types:
  unit         Run unit tests
  integration  Run integration tests
  e2e          Run end-to-end tests
  performance  Run performance tests
  config       Test configurations only
  all          Run all tests (default)

Databases:
  postgresql   Test PostgreSQL only
  mysql        Test MySQL only
  mongodb      Test MongoDB only
  redis        Test Redis only
  all          Test all databases (default)

Options:
  -v, --verbose    Enable verbose output
  -h, --help       Show this help message

Examples:
  $0                              # Run all tests
  $0 unit                         # Run unit tests only
  $0 integration postgresql       # Run PostgreSQL integration tests
  $0 e2e all -v                  # Run all E2E tests with verbose output

Environment Variables:
  VERBOSE=true     Enable verbose output
  SKIP_BUILD=true  Skip building before tests
  TEST_TIMEOUT=300 Test timeout in seconds
EOF
    exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        *)
            shift
            ;;
    esac
done

print_header "Database Intelligence Test Runner"
log_info "Test Type: $TEST_TYPE"
log_info "Database: $DATABASE"
log_info "Verbose: $VERBOSE"
echo

# Test results tracking
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
TEST_START_TIME=$(date +%s)

# Function to run a test suite
run_test_suite() {
    local suite_name=$1
    local test_script=$2
    local args="${3:-}"
    
    print_separator
    log_info "Running $suite_name tests..."
    ((TOTAL_TESTS++))
    
    local start_time=$(date +%s)
    
    if [[ -x "$test_script" ]]; then
        if $test_script $args; then
            local end_time=$(date +%s)
            local duration=$((end_time - start_time))
            log_success "$suite_name tests passed (${duration}s)"
            ((PASSED_TESTS++))
        else
            log_error "$suite_name tests failed"
            ((FAILED_TESTS++))
        fi
    else
        log_warning "Test script not found or not executable: $test_script"
        ((FAILED_TESTS++))
    fi
}

# Build if needed
if [[ "$SKIP_BUILD" != "true" ]] && [[ "$TEST_TYPE" != "config" ]]; then
    log_info "Building collector..."
    if ! "$SCRIPT_DIR/../build/build.sh" production; then
        log_error "Build failed"
        exit 1
    fi
fi

# Run tests based on type
case "$TEST_TYPE" in
    unit)
        run_test_suite "Unit" "$SCRIPT_DIR/unit.sh" "$DATABASE"
        ;;
        
    integration)
        run_test_suite "Integration" "$SCRIPT_DIR/integration.sh" "$DATABASE"
        ;;
        
    e2e)
        run_test_suite "E2E" "$ROOT_DIR/tests/e2e/run_e2e_tests.sh" "$DATABASE"
        ;;
        
    performance)
        run_test_suite "Performance" "$SCRIPT_DIR/performance.sh" "$DATABASE"
        ;;
        
    config)
        run_test_suite "Configuration" "$SCRIPT_DIR/config-validation.sh" "$DATABASE"
        ;;
        
    all)
        run_test_suite "Unit" "$SCRIPT_DIR/unit.sh" "$DATABASE"
        run_test_suite "Integration" "$SCRIPT_DIR/integration.sh" "$DATABASE"
        run_test_suite "Configuration" "$SCRIPT_DIR/config-validation.sh" "$DATABASE"
        run_test_suite "E2E" "$ROOT_DIR/tests/e2e/run_e2e_tests.sh" "$DATABASE"
        if [[ "$DATABASE" != "all" ]]; then
            run_test_suite "Performance" "$SCRIPT_DIR/performance.sh" "$DATABASE"
        fi
        ;;
        
    *)
        log_error "Unknown test type: $TEST_TYPE"
        usage
        ;;
esac

# Calculate total time
TEST_END_TIME=$(date +%s)
TOTAL_DURATION=$((TEST_END_TIME - TEST_START_TIME))

# Print summary
print_separator
print_header "Test Summary"
echo
echo "Total Tests:  $TOTAL_TESTS"
echo "Passed:       $PASSED_TESTS"
echo "Failed:       $FAILED_TESTS"
echo "Duration:     ${TOTAL_DURATION}s"
echo

# Generate test report
REPORT_FILE="$ROOT_DIR/test-report-$(date +%Y%m%d-%H%M%S).txt"
cat > "$REPORT_FILE" << EOF
Database Intelligence Test Report
Generated: $(date)

Configuration:
- Test Type: $TEST_TYPE
- Database: $DATABASE
- Duration: ${TOTAL_DURATION}s

Results:
- Total Tests: $TOTAL_TESTS
- Passed: $PASSED_TESTS
- Failed: $FAILED_TESTS
- Success Rate: $(( PASSED_TESTS * 100 / TOTAL_TESTS ))%

EOF

if [[ $FAILED_TESTS -gt 0 ]]; then
    log_error "Some tests failed. See $REPORT_FILE for details."
    exit 1
else
    log_success "All tests passed! Report saved to $REPORT_FILE"
    exit 0
fi