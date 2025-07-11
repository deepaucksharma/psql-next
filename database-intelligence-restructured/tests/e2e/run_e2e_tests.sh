#!/bin/bash

# E2E Test Runner for Database Intelligence Project
# This script runs comprehensive end-to-end tests with real databases and New Relic

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DOCKER_COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"
TEST_TIMEOUT="30m"
COVERAGE_DIR="$SCRIPT_DIR/coverage"

# Load .env file if it exists
if [[ -f "$SCRIPT_DIR/.env" ]]; then
    echo "Loading environment from .env file..."
    set -a  # Export all variables
    source "$SCRIPT_DIR/.env"
    set +a  # Stop exporting
elif [[ -f "$PROJECT_ROOT/.env" ]]; then
    echo "Loading environment from project .env file..."
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
fi

# Test configuration
export TEST_ENV="${TEST_ENV:-local}"
export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
export POSTGRES_PORT="${POSTGRES_PORT:-5432}"
export POSTGRES_USER="${POSTGRES_USER:-postgres}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
export POSTGRES_DB="${POSTGRES_DB:-testdb}"

export MYSQL_HOST="${MYSQL_HOST:-localhost}"
export MYSQL_PORT="${MYSQL_PORT:-3306}"
export MYSQL_USER="${MYSQL_USER:-root}"
export MYSQL_PASSWORD="${MYSQL_PASSWORD:-root}"
export MYSQL_DB="${MYSQL_DB:-testdb}"
export MYSQL_ENABLED="${MYSQL_ENABLED:-true}"

# Collector configuration
export COLLECTOR_BINARY="${COLLECTOR_BINARY:-$PROJECT_ROOT/core/cmd/collector/collector}"
export COLLECTOR_ENDPOINT="${COLLECTOR_ENDPOINT:-localhost:4317}"
export METRICS_ENDPOINT="${METRICS_ENDPOINT:-localhost:8889}"
export HEALTH_ENDPOINT="${HEALTH_ENDPOINT:-localhost:13133}"

# Function to print colored output
print_color() {
    local color=$1
    shift
    echo -e "${color}$@${NC}"
}

# Function to check prerequisites
check_prerequisites() {
    print_color $BLUE "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_color $RED "Docker is required but not installed"
        exit 1
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        print_color $RED "Docker Compose is required but not installed"
        exit 1
    fi
    
    # Check Go
    if ! command -v go &> /dev/null; then
        print_color $RED "Go is required but not installed"
        exit 1
    fi
    
    # Check New Relic credentials
    if [[ -z "${NEW_RELIC_LICENSE_KEY:-}" ]]; then
        print_color $YELLOW "WARNING: NEW_RELIC_LICENSE_KEY not set - New Relic tests will be skipped"
    else
        print_color $GREEN "✓ New Relic License Key found"
    fi
    
    if [[ -z "${NEW_RELIC_ACCOUNT_ID:-}" ]]; then
        print_color $YELLOW "WARNING: NEW_RELIC_ACCOUNT_ID not set - verification tests will be skipped"
    else
        print_color $GREEN "✓ New Relic Account ID found: ${NEW_RELIC_ACCOUNT_ID}"
    fi
    
    if [[ -z "${NEW_RELIC_USER_KEY:-}" ]] && [[ -z "${NEW_RELIC_API_KEY:-}" ]]; then
        print_color $YELLOW "WARNING: NEW_RELIC_USER_KEY/API_KEY not set - NRDB queries will fail"
    else
        # Set API_KEY from USER_KEY if not already set
        export NEW_RELIC_API_KEY="${NEW_RELIC_API_KEY:-${NEW_RELIC_USER_KEY:-}}"
        print_color $GREEN "✓ New Relic API Key configured"
    fi
    
    print_color $GREEN "✓ Prerequisites check passed"
}

# Function to build collector
build_collector() {
    print_color $BLUE "Building collector binary..."
    
    cd "$PROJECT_ROOT/core/cmd/collector"
    go build -o collector .
    
    if [[ ! -f "$COLLECTOR_BINARY" ]]; then
        print_color $RED "Failed to build collector binary"
        exit 1
    fi
    
    print_color $GREEN "✓ Collector built successfully"
}

# Function to start test infrastructure
start_infrastructure() {
    print_color $BLUE "Starting test infrastructure..."
    
    # Stop any existing containers
    docker-compose -f "$DOCKER_COMPOSE_FILE" down -v || true
    
    # Start databases
    docker-compose -f "$DOCKER_COMPOSE_FILE" up -d postgres mysql prometheus
    
    # Wait for databases to be ready
    print_color $BLUE "Waiting for databases to be ready..."
    
    # Wait for PostgreSQL
    local retries=30
    while ! docker exec e2e-postgres pg_isready -U postgres &> /dev/null; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            print_color $RED "PostgreSQL failed to start"
            exit 1
        fi
        sleep 1
    done
    print_color $GREEN "✓ PostgreSQL is ready"
    
    # Wait for MySQL
    if [[ "$MYSQL_ENABLED" == "true" ]]; then
        retries=30
        while ! docker exec e2e-mysql mysqladmin ping -h localhost --silent &> /dev/null; do
            retries=$((retries - 1))
            if [[ $retries -eq 0 ]]; then
                print_color $RED "MySQL failed to start"
                exit 1
            fi
            sleep 1
        done
        print_color $GREEN "✓ MySQL is ready"
    fi
    
    print_color $GREEN "✓ Test infrastructure started"
}

# Function to run tests
run_tests() {
    local test_suite="${1:-all}"
    
    print_color $BLUE "Running E2E tests (suite: $test_suite)..."
    
    # Create coverage directory
    mkdir -p "$COVERAGE_DIR"
    
    # Set test flags
    local test_flags="-v -timeout=$TEST_TIMEOUT"
    
    if [[ "${COVERAGE_ENABLED:-false}" == "true" ]]; then
        test_flags="$test_flags -coverprofile=$COVERAGE_DIR/e2e.coverage -covermode=atomic"
    fi
    
    # Change to test directory
    cd "$SCRIPT_DIR"
    
    case "$test_suite" in
        "all")
            print_color $BLUE "Running all E2E test suites..."
            go test $test_flags ./suites -run TestAll
            ;;
        "comprehensive")
            print_color $BLUE "Running comprehensive E2E test..."
            go test $test_flags ./suites -run TestComprehensiveSuite
            ;;
        "custom-processors")
            print_color $BLUE "Running custom processors tests..."
            go test $test_flags ./suites -run TestCustomProcessorsSuite
            ;;
        "mode-comparison")
            print_color $BLUE "Running mode comparison tests..."
            go test $test_flags ./suites -run TestModeComparisonSuite
            ;;
        "ash-plan")
            print_color $BLUE "Running ASH and plan analysis tests..."
            go test $test_flags ./suites -run TestASHPlanAnalysisSuite
            ;;
        "performance")
            print_color $BLUE "Running performance and scale tests..."
            go test $test_flags ./suites -run TestPerformanceScaleSuite
            ;;
        "newrelic")
            print_color $BLUE "Running New Relic validation tests..."
            go test $test_flags ./suites -run TestNewRelicValidationSuite
            ;;
        *)
            print_color $RED "Unknown test suite: $test_suite"
            echo "Available suites: all, comprehensive, verification, adapters, database, performance"
            exit 1
            ;;
    esac
}

# Function to generate test report
generate_report() {
    print_color $BLUE "Generating test report..."
    
    if [[ -f "$COVERAGE_DIR/e2e.coverage" ]]; then
        # Generate coverage report
        go tool cover -html="$COVERAGE_DIR/e2e.coverage" -o "$COVERAGE_DIR/coverage.html"
        print_color $GREEN "✓ Coverage report generated: $COVERAGE_DIR/coverage.html"
        
        # Print coverage summary
        local coverage=$(go tool cover -func="$COVERAGE_DIR/e2e.coverage" | grep total | awk '{print $3}')
        print_color $BLUE "Total coverage: $coverage"
    fi
    
    # Generate test summary
    local report_file="$SCRIPT_DIR/test-report-$(date +%Y%m%d-%H%M%S).txt"
    {
        echo "E2E Test Report"
        echo "==============="
        echo "Date: $(date)"
        echo "Environment: $TEST_ENV"
        echo ""
        echo "Configuration:"
        echo "  PostgreSQL: $POSTGRES_HOST:$POSTGRES_PORT"
        echo "  MySQL: $MYSQL_HOST:$MYSQL_PORT (enabled: $MYSQL_ENABLED)"
        echo "  Collector: $COLLECTOR_ENDPOINT"
        echo ""
        echo "Test Results:"
        echo "--------------"
    } > "$report_file"
    
    print_color $GREEN "✓ Test report saved: $report_file"
}

# Function to cleanup
cleanup() {
    print_color $BLUE "Cleaning up..."
    
    # Stop infrastructure
    if [[ "${KEEP_INFRASTRUCTURE:-false}" != "true" ]]; then
        docker-compose -f "$DOCKER_COMPOSE_FILE" down -v || true
        print_color $GREEN "✓ Test infrastructure stopped"
    else
        print_color $YELLOW "Infrastructure kept running (KEEP_INFRASTRUCTURE=true)"
    fi
}

# Main execution
main() {
    local test_suite="${1:-all}"
    
    print_color $BLUE "=== Database Intelligence E2E Test Runner ==="
    print_color $BLUE "Test Suite: $test_suite"
    echo ""
    
    # Set trap for cleanup
    trap cleanup EXIT
    
    # Run steps
    check_prerequisites
    
    # Build collector if needed
    if [[ ! -f "$COLLECTOR_BINARY" ]] || [[ "${REBUILD_COLLECTOR:-false}" == "true" ]]; then
        build_collector
    fi
    
    # Start infrastructure
    start_infrastructure
    
    # Run tests
    if run_tests "$test_suite"; then
        print_color $GREEN "✓ All tests passed!"
        local exit_code=0
    else
        print_color $RED "✗ Some tests failed"
        local exit_code=1
    fi
    
    # Generate report
    generate_report
    
    exit $exit_code
}

# Show help
show_help() {
    cat << EOF
Usage: $0 [TEST_SUITE] [OPTIONS]

Run end-to-end tests for Database Intelligence project

TEST_SUITE:
    all              Run all test suites (default)
    comprehensive    Run comprehensive E2E test suite
    custom-processors Run custom processors integration tests
    mode-comparison  Run config-only vs enhanced mode comparison tests
    ash-plan         Run ASH and plan analysis tests
    performance      Run performance and scale tests
    newrelic         Run New Relic validation tests

OPTIONS:
    Environment variables:
        TEST_ENV                 Test environment name (default: local)
        COVERAGE_ENABLED         Enable coverage collection (default: false)
        KEEP_INFRASTRUCTURE      Keep infrastructure running after tests (default: false)
        REBUILD_COLLECTOR        Force rebuild of collector binary (default: false)
        
        PostgreSQL Configuration:
        POSTGRES_HOST            PostgreSQL host (default: localhost)
        POSTGRES_PORT            PostgreSQL port (default: 5432)
        POSTGRES_USER            PostgreSQL user (default: postgres)
        POSTGRES_PASSWORD        PostgreSQL password (default: postgres)
        POSTGRES_DB              PostgreSQL database (default: testdb)
        
        MySQL Configuration:
        MYSQL_HOST               MySQL host (default: localhost)
        MYSQL_PORT               MySQL port (default: 3306)
        MYSQL_USER               MySQL user (default: root)
        MYSQL_PASSWORD           MySQL password (default: root)
        MYSQL_DB                 MySQL database (default: testdb)
        MYSQL_ENABLED            Enable MySQL tests (default: true)
        
        New Relic Configuration:
        NEW_RELIC_LICENSE_KEY    New Relic license key (required for NR tests)
        NEW_RELIC_ACCOUNT_ID     New Relic account ID (required for verification)
        NEW_RELIC_API_KEY        New Relic API key (required for verification)
        NEW_RELIC_OTLP_ENDPOINT  New Relic OTLP endpoint (default: otlp.nr-data.net:4317)

Examples:
    # Run all tests
    $0
    
    # Run specific test suite
    $0 comprehensive
    
    # Run with coverage
    COVERAGE_ENABLED=true $0
    
    # Keep infrastructure running
    KEEP_INFRASTRUCTURE=true $0
    
    # Run with custom PostgreSQL
    POSTGRES_HOST=db.example.com POSTGRES_PORT=5433 $0
    
    # Run only PostgreSQL tests (disable MySQL)
    MYSQL_ENABLED=false $0

EOF
}

# Parse arguments
if [[ "${1:-}" == "--help" ]] || [[ "${1:-}" == "-h" ]]; then
    show_help
    exit 0
fi

# Run main
main "${1:-all}"