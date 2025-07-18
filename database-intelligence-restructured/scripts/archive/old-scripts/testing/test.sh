#!/bin/bash
# Unified test script for Database Intelligence
# Runs all test suites with proper organization

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$PROJECT_ROOT/scripts/utils/common.sh"

# Test configuration
TEST_TYPE="${1:-all}"
COVERAGE="${COVERAGE:-false}"
VERBOSE="${VERBOSE:-false}"
PARALLEL="${PARALLEL:-true}"

usage() {
    cat << EOF
Usage: $0 [TEST_TYPE] [OPTIONS]

Run Database Intelligence tests

Test Types:
  all              Run all tests (default)
  unit             Run unit tests only
  integration      Run integration tests only
  e2e              Run end-to-end tests only
  components       Test individual components
  performance      Run performance tests
  otlp             Run OTLP compliance tests

Environment Variables:
  COVERAGE=true    Generate coverage reports
  VERBOSE=true     Enable verbose output
  PARALLEL=false   Disable parallel test execution

Examples:
  $0 unit                        # Run unit tests
  $0 all COVERAGE=true          # Run all tests with coverage
  $0 e2e VERBOSE=true           # Run E2E tests verbosely

EOF
    exit 1
}

# Setup test environment
setup_test_env() {
    log_info "Setting up test environment..."
    
    # Load test environment variables
    load_env_file "$PROJECT_ROOT/.env.test"
    
    # Create test directories
    mkdir -p "$PROJECT_ROOT/test-results"
    mkdir -p "$PROJECT_ROOT/coverage"
    
    # Start test databases if needed
    if [[ "$TEST_TYPE" == "integration" || "$TEST_TYPE" == "e2e" || "$TEST_TYPE" == "all" ]]; then
        start_test_databases
    fi
}

# Start test databases
start_test_databases() {
    log_info "Starting test databases..."
    
    cd "$PROJECT_ROOT/deployments/docker"
    docker_compose_up "compose/docker-compose-databases.yaml" "postgres mysql"
    
    # Wait for databases to be ready
    wait_for_service localhost 5432 60
    wait_for_service localhost 3306 60
    
    # Initialize test data
    initialize_test_data
}

# Initialize test data
initialize_test_data() {
    log_info "Initializing test data..."
    
    # PostgreSQL
    PGPASSWORD="${DB_POSTGRES_PASSWORD:-postgres}" psql \
        -h localhost -p 5432 -U postgres -d postgres \
        -f "$PROJECT_ROOT/deployments/docker/init-scripts/postgres-init.sql"
    
    # MySQL
    mysql -h localhost -P 3306 -u root -proot \
        < "$PROJECT_ROOT/deployments/docker/init-scripts/mysql-init.sql"
}

# Run unit tests
run_unit_tests() {
    log_info "Running unit tests..."
    
    cd "$PROJECT_ROOT"
    
    local test_args="-v"
    
    if [[ "$COVERAGE" == "true" ]]; then
        test_args="$test_args -coverprofile=coverage/unit.out -covermode=atomic"
    fi
    
    if [[ "$PARALLEL" == "true" ]]; then
        test_args="$test_args -parallel 4"
    fi
    
    # Run tests for each component
    go test $test_args ./components/processors/...
    go test $test_args ./components/receivers/...
    go test $test_args ./components/exporters/...
    go test $test_args ./common/...
    
    log_success "Unit tests completed"
}

# Run integration tests
run_integration_tests() {
    log_info "Running integration tests..."
    
    cd "$PROJECT_ROOT"
    
    local test_args="-v -tags=integration"
    
    if [[ "$COVERAGE" == "true" ]]; then
        test_args="$test_args -coverprofile=coverage/integration.out"
    fi
    
    # Run integration tests
    go test $test_args ./tests/integration/...
    
    log_success "Integration tests completed"
}

# Run E2E tests
run_e2e_tests() {
    log_info "Running end-to-end tests..."
    
    cd "$PROJECT_ROOT/tests/e2e"
    
    # Build test collector if needed
    if [[ ! -f "$PROJECT_ROOT/distributions/production/database-intelligence-collector" ]]; then
        log_info "Building test collector..."
        "$PROJECT_ROOT/scripts/build/build.sh" production
    fi
    
    # Run E2E test suite
    if [[ -f "run_e2e_tests.sh" ]]; then
        ./run_e2e_tests.sh
    else
        go test -v ./suites/...
    fi
    
    log_success "E2E tests completed"
}

# Run component tests
run_component_tests() {
    log_info "Running component tests..."
    
    cd "$PROJECT_ROOT"
    
    # Test each component can be built
    log_info "Testing processor builds..."
    go build ./components/processors/...
    
    log_info "Testing receiver builds..."
    go build ./components/receivers/...
    
    log_info "Testing exporter builds..."
    go build ./components/exporters/...
    
    # Run component-specific tests
    go test -v ./components/processors/adaptivesampler/...
    go test -v ./components/processors/circuitbreaker/...
    go test -v ./components/receivers/ash/...
    
    log_success "Component tests completed"
}

# Run performance tests
run_performance_tests() {
    log_info "Running performance tests..."
    
    cd "$PROJECT_ROOT"
    
    # Run benchmarks
    go test -bench=. -benchmem ./components/processors/...
    
    # Run load tests if available
    if [[ -d "tests/performance" ]]; then
        go test -v ./tests/performance/...
    fi
    
    log_success "Performance tests completed"
}

# Run OTLP compliance tests
run_otlp_tests() {
    log_info "Running OTLP compliance tests..."
    
    cd "$PROJECT_ROOT/tests/e2e"
    
    if [[ -f "run_otlp_tests.sh" ]]; then
        ./run_otlp_tests.sh
    else
        log_warning "OTLP test script not found"
    fi
    
    log_success "OTLP tests completed"
}

# Generate test report
generate_test_report() {
    if [[ "$COVERAGE" != "true" ]]; then
        return
    fi
    
    log_info "Generating test coverage report..."
    
    cd "$PROJECT_ROOT"
    
    # Merge coverage files
    if command -v gocovmerge &> /dev/null; then
        gocovmerge coverage/*.out > coverage/combined.out
    fi
    
    # Generate HTML report
    go tool cover -html=coverage/combined.out -o coverage/report.html
    
    # Calculate total coverage
    local total_coverage=$(go tool cover -func=coverage/combined.out | grep total | awk '{print $3}')
    
    log_success "Test coverage: $total_coverage"
    log_info "Coverage report: coverage/report.html"
}

# Cleanup test environment
cleanup_test_env() {
    log_info "Cleaning up test environment..."
    
    # Stop test databases
    if [[ "$TEST_TYPE" == "integration" || "$TEST_TYPE" == "e2e" || "$TEST_TYPE" == "all" ]]; then
        cd "$PROJECT_ROOT/deployments/docker"
        docker_compose_down "compose/docker-compose-databases.yaml"
    fi
    
    # Clean test artifacts (optional)
    if [[ "${CLEAN_AFTER:-false}" == "true" ]]; then
        rm -rf "$PROJECT_ROOT/test-results"
        rm -rf "$PROJECT_ROOT/coverage"
    fi
}

# Main execution
main() {
    case "$TEST_TYPE" in
        unit)
            setup_test_env
            run_unit_tests
            ;;
        integration)
            setup_test_env
            run_integration_tests
            ;;
        e2e)
            setup_test_env
            run_e2e_tests
            ;;
        components)
            setup_test_env
            run_component_tests
            ;;
        performance|perf)
            setup_test_env
            run_performance_tests
            ;;
        otlp)
            setup_test_env
            run_otlp_tests
            ;;
        all)
            setup_test_env
            run_unit_tests
            run_integration_tests
            run_component_tests
            run_e2e_tests
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            log_error "Unknown test type: $TEST_TYPE"
            usage
            ;;
    esac
    
    # Generate reports
    generate_test_report
    
    # Cleanup
    trap cleanup_test_env EXIT
    
    log_success "All tests completed successfully!"
}

# Run main
main