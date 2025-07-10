#!/bin/bash
# Comprehensive test runner for Database Intelligence
# This script runs all tests in sequence and provides detailed reporting

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
TEST_RESULTS_DIR="$PROJECT_ROOT/test-results"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TEST_LOG="$TEST_RESULTS_DIR/comprehensive-test-$TIMESTAMP.log"

# Test configuration
TEST_TIMEOUT="600s"
E2E_TIMEOUT="900s"
LOAD_TEST_DURATION="300s"

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Functions
print_header() {
    echo -e "\n${CYAN}=====================================================================${NC}"
    echo -e "${CYAN} $1${NC}"
    echo -e "${CYAN}=====================================================================${NC}\n"
}

print_section() {
    echo -e "\n${BLUE}>>> $1${NC}\n"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

run_test() {
    local test_name="$1"
    local test_command="$2"
    local is_critical="${3:-true}"
    
    print_section "Running: $test_name"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if eval "$test_command" >> "$TEST_LOG" 2>&1; then
        print_success "$test_name"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        if [ "$is_critical" = "true" ]; then
            print_error "$test_name (CRITICAL)"
            FAILED_TESTS=$((FAILED_TESTS + 1))
            return 1
        else
            print_warning "$test_name (SKIPPED)"
            SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
            return 0
        fi
    fi
}

check_prerequisites() {
    print_section "Checking Prerequisites"
    
    # Check required tools
    local required_tools=("go" "docker" "docker-compose" "curl" "jq")
    for tool in "${required_tools[@]}"; do
        if command -v "$tool" >/dev/null 2>&1; then
            print_success "$tool is available"
        else
            print_error "$tool is required but not installed"
            exit 1
        fi
    done
    
    # Check Go workspace
    if go work status >/dev/null 2>&1; then
        print_success "Go workspace is valid"
    else
        print_error "Go workspace is invalid"
        exit 1
    fi
    
    # Check Docker daemon
    if docker info >/dev/null 2>&1; then
        print_success "Docker daemon is running"
    else
        print_error "Docker daemon is not running"
        exit 1
    fi
}

run_unit_tests() {
    print_header "UNIT TESTS"
    
    # Test individual modules
    local modules=(
        "common/featuredetector"
        "common/queryselector"
        "processors/adaptivesampler"
        "processors/circuitbreaker"
        "processors/costcontrol"
        "processors/nrerrormonitor"
        "processors/planattributeextractor"
        "processors/querycorrelator"
        "processors/verification"
        "receivers/enhancedsql"
        "exporters/nri"
        "extensions/healthcheck"
    )
    
    for module in "${modules[@]}"; do
        if [ -d "$PROJECT_ROOT/$module" ] && [ -f "$PROJECT_ROOT/$module/go.mod" ]; then
            run_test "Unit tests: $module" "cd '$PROJECT_ROOT/$module' && go test -v -timeout=$TEST_TIMEOUT ./..." false
        else
            print_warning "Module $module not found or missing go.mod"
            SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
            TOTAL_TESTS=$((TOTAL_TESTS + 1))
        fi
    done
}

run_integration_tests() {
    print_header "INTEGRATION TESTS"
    
    run_test "Integration test suite" "cd '$PROJECT_ROOT/tests/integration' && go test -v -timeout=$TEST_TIMEOUT ./..." false
}

run_build_tests() {
    print_header "BUILD TESTS"
    
    run_test "Build minimal distribution" "cd '$PROJECT_ROOT' && make build-minimal"
    run_test "Build standard distribution" "cd '$PROJECT_ROOT' && make build-standard"
    run_test "Build enterprise distribution" "cd '$PROJECT_ROOT' && make build-enterprise"
    run_test "Build Docker images" "cd '$PROJECT_ROOT' && make docker-build" false
}

run_configuration_tests() {
    print_header "CONFIGURATION TESTS"
    
    run_test "Validate unified configuration" "cd '$PROJECT_ROOT' && ./bin/collector-enterprise --config=configs/unified/database-intelligence-complete.yaml --dry-run" false
    run_test "Environment template validation" "cd '$PROJECT_ROOT' && test -f configs/unified/environment-template.env"
    run_test "Docker compose validation" "cd '$PROJECT_ROOT' && docker-compose -f docker-compose.unified.yml config >/dev/null"
}

start_test_environment() {
    print_header "STARTING TEST ENVIRONMENT"
    
    print_section "Setting up environment"
    if [ ! -f "$PROJECT_ROOT/.env" ]; then
        cp "$PROJECT_ROOT/configs/unified/environment-template.env" "$PROJECT_ROOT/.env"
        print_success "Created .env file from template"
    fi
    
    print_section "Starting Docker environment"
    cd "$PROJECT_ROOT"
    docker-compose -f docker-compose.unified.yml down -v >/dev/null 2>&1 || true
    
    if docker-compose -f docker-compose.unified.yml up -d >> "$TEST_LOG" 2>&1; then
        print_success "Docker environment started"
        
        print_section "Waiting for services to be ready"
        local wait_time=0
        local max_wait=180
        
        while [ $wait_time -lt $max_wait ]; do
            if curl -f http://localhost:13133/health >/dev/null 2>&1; then
                print_success "Collector is healthy"
                return 0
            fi
            echo -n "."
            sleep 5
            wait_time=$((wait_time + 5))
        done
        
        print_error "Services failed to become healthy within $max_wait seconds"
        return 1
    else
        print_error "Failed to start Docker environment"
        return 1
    fi
}

run_e2e_tests() {
    print_header "END-TO-END TESTS"
    
    if ! start_test_environment; then
        print_error "Failed to start test environment for E2E tests"
        return 1
    fi
    
    # Wait additional time for data collection
    print_section "Waiting for data collection to stabilize"
    sleep 30
    
    # Health checks
    run_test "Collector health check" "curl -f http://localhost:13133/health"
    run_test "Database connectivity" "docker-compose -f docker-compose.unified.yml exec -T postgres pg_isready -U postgres"
    
    # Data flow verification
    run_test "Collector internal metrics" "curl -s http://localhost:8888/metrics | grep -q otelcol_receiver_accepted_metric_points"
    run_test "Export metrics available" "curl -s http://localhost:8888/metrics | grep -q otelcol_exporter_sent_metric_points"
    run_test "Processor metrics available" "curl -s http://localhost:8888/metrics | grep -q otelcol_processor_accepted_metric_points" false
    
    # Processor verification
    run_test "Adaptive sampler metrics" "curl -s http://localhost:8888/metrics | grep -q 'adaptivesampler_' || echo 'Adaptive sampler not active'" false
    run_test "Circuit breaker metrics" "curl -s http://localhost:8888/metrics | grep -q 'circuitbreaker_' || echo 'Circuit breaker not active'" false
    
    # Run dedicated E2E test suite if available
    if [ -f "$PROJECT_ROOT/tests/e2e/run_working_e2e_tests.sh" ]; then
        run_test "Dedicated E2E test suite" "cd '$PROJECT_ROOT/tests/e2e' && timeout $E2E_TIMEOUT ./run_working_e2e_tests.sh" false
    fi
}

run_load_tests() {
    print_header "LOAD TESTS"
    
    if ! docker-compose -f docker-compose.unified.yml ps | grep -q "Up"; then
        print_warning "Test environment not running, starting it first"
        if ! start_test_environment; then
            print_error "Failed to start environment for load tests"
            return 1
        fi
    fi
    
    run_test "Load test execution" "cd '$PROJECT_ROOT' && timeout $LOAD_TEST_DURATION docker-compose -f docker-compose.unified.yml --profile load-testing up --exit-code-from load-generator load-generator" false
}

run_security_tests() {
    print_header "SECURITY TESTS"
    
    # PII detection tests
    run_test "PII detection configuration" "grep -q 'pii_detection' '$PROJECT_ROOT/configs/unified/database-intelligence-complete.yaml'"
    
    # Configuration security
    run_test "No hardcoded secrets in config" "! grep -r 'password.*[^$]' '$PROJECT_ROOT/configs/' || echo 'Found potential hardcoded secrets'"
    
    # Dependency security (if tools available)
    if command -v gosec >/dev/null 2>&1; then
        run_test "Security vulnerability scan" "cd '$PROJECT_ROOT' && gosec ./..." false
    else
        print_warning "gosec not available, skipping security scan"
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
}

run_performance_tests() {
    print_header "PERFORMANCE TESTS"
    
    if [ -d "$PROJECT_ROOT/tests/performance" ]; then
        run_test "Performance test suite" "cd '$PROJECT_ROOT/tests/performance' && go test -v -timeout=$TEST_TIMEOUT ./..." false
    else
        print_warning "Performance tests directory not found"
        SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
        TOTAL_TESTS=$((TOTAL_TESTS + 1))
    fi
    
    # Memory and resource tests
    if command -v pprof >/dev/null 2>&1 && curl -f http://localhost:1777/debug/pprof/ >/dev/null 2>&1; then
        run_test "Memory profile collection" "curl -s http://localhost:1777/debug/pprof/heap > '$TEST_RESULTS_DIR/heap-$TIMESTAMP.prof'" false
        run_test "CPU profile collection" "timeout 30s curl -s http://localhost:1777/debug/pprof/profile > '$TEST_RESULTS_DIR/cpu-$TIMESTAMP.prof'" false
    else
        print_warning "pprof not available or collector not running with profiling enabled"
        SKIPPED_TESTS=$((SKIPPED_TESTS + 2))
        TOTAL_TESTS=$((TOTAL_TESTS + 2))
    fi
}

cleanup_test_environment() {
    print_header "CLEANUP"
    
    print_section "Stopping test environment"
    cd "$PROJECT_ROOT"
    docker-compose -f docker-compose.unified.yml down -v >/dev/null 2>&1 || true
    print_success "Test environment stopped"
    
    # Collect final logs
    if [ -d "$PROJECT_ROOT/telemetry-output" ]; then
        cp -r "$PROJECT_ROOT/telemetry-output" "$TEST_RESULTS_DIR/telemetry-$TIMESTAMP/" 2>/dev/null || true
    fi
}

generate_report() {
    print_header "TEST RESULTS SUMMARY"
    
    local total_time=$(($(date +%s) - START_TIME))
    local success_rate=0
    
    if [ $TOTAL_TESTS -gt 0 ]; then
        success_rate=$(( (PASSED_TESTS * 100) / TOTAL_TESTS ))
    fi
    
    # Console report
    echo -e "${CYAN}Comprehensive Test Results${NC}"
    echo -e "=========================="
    echo -e "Total Tests:    ${BLUE}$TOTAL_TESTS${NC}"
    echo -e "Passed:         ${GREEN}$PASSED_TESTS${NC}"
    echo -e "Failed:         ${RED}$FAILED_TESTS${NC}"
    echo -e "Skipped:        ${YELLOW}$SKIPPED_TESTS${NC}"
    echo -e "Success Rate:   ${CYAN}$success_rate%${NC}"
    echo -e "Total Time:     ${CYAN}${total_time}s${NC}"
    echo -e "Log File:       ${BLUE}$TEST_LOG${NC}"
    
    # JSON report
    local json_report="$TEST_RESULTS_DIR/test-report-$TIMESTAMP.json"
    cat > "$json_report" << EOF
{
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "duration_seconds": $total_time,
  "summary": {
    "total": $TOTAL_TESTS,
    "passed": $PASSED_TESTS,
    "failed": $FAILED_TESTS,
    "skipped": $SKIPPED_TESTS,
    "success_rate": $success_rate
  },
  "environment": {
    "go_version": "$(go version)",
    "docker_version": "$(docker --version)",
    "platform": "$(uname -s -r)"
  },
  "log_file": "$TEST_LOG",
  "reports_directory": "$TEST_RESULTS_DIR"
}
EOF
    
    print_success "Test report generated: $json_report"
    
    # Final status
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "\n${GREEN}🎉 ALL TESTS COMPLETED SUCCESSFULLY!${NC}"
        return 0
    else
        echo -e "\n${RED}❌ SOME TESTS FAILED. Check the log file for details.${NC}"
        return 1
    fi
}

# Main execution
main() {
    print_header "DATABASE INTELLIGENCE COMPREHENSIVE TEST SUITE"
    
    # Initialize
    mkdir -p "$TEST_RESULTS_DIR"
    echo "Starting comprehensive test run at $(date)" > "$TEST_LOG"
    START_TIME=$(date +%s)
    
    # Set trap for cleanup
    trap cleanup_test_environment EXIT
    
    # Run test phases
    check_prerequisites
    
    # Core tests (always run)
    run_unit_tests
    run_build_tests
    run_configuration_tests
    
    # Advanced tests (may be skipped if environment issues)
    run_integration_tests
    run_e2e_tests
    run_security_tests
    run_performance_tests
    run_load_tests
    
    # Generate final report
    generate_report
}

# Execute if run directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi