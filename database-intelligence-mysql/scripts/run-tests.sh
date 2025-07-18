#!/bin/bash

# Run tests for MySQL monitoring
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "=== MySQL Monitoring Test Runner ==="
echo "Project root: $PROJECT_ROOT"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test mode selection
TEST_MODE="${1:-all}"

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    case $status in
        "success")
            echo -e "${GREEN}✓${NC} $message"
            ;;
        "error")
            echo -e "${RED}✗${NC} $message"
            ;;
        "info")
            echo -e "${YELLOW}ℹ${NC} $message"
            ;;
    esac
}

# Function to check prerequisites
check_prerequisites() {
    print_status "info" "Checking prerequisites..."
    
    # Check for Go
    if ! command -v go &> /dev/null; then
        print_status "error" "Go is not installed"
        exit 1
    fi
    print_status "success" "Go is installed: $(go version)"
    
    # Check for Docker
    if ! command -v docker &> /dev/null; then
        print_status "error" "Docker is not installed"
        exit 1
    fi
    print_status "success" "Docker is installed: $(docker --version)"
    
    # Check if Docker is running
    if ! docker info &> /dev/null; then
        print_status "error" "Docker daemon is not running"
        exit 1
    fi
    print_status "success" "Docker daemon is running"
}

# Function to setup test environment
setup_test_env() {
    print_status "info" "Setting up test environment..."
    
    # Export environment variables
    export MYSQL_HOST="${MYSQL_HOST:-localhost}"
    export MYSQL_PORT="${MYSQL_PORT:-3306}"
    export MYSQL_USER="${MYSQL_USER:-root}"
    export MYSQL_PASSWORD="${MYSQL_PASSWORD:-rootpassword}"
    export MYSQL_DATABASE="${MYSQL_DATABASE:-test}"
    
    # Check if MySQL is running
    if docker ps | grep -q mysql; then
        print_status "success" "MySQL container is running"
    else
        print_status "info" "Starting MySQL container..."
        cd "$PROJECT_ROOT"
        docker-compose up -d mysql-primary mysql-replica
        sleep 10  # Wait for MySQL to be ready
    fi
    
    # Check if OTel collector is running
    if docker ps | grep -q otel-collector; then
        print_status "success" "OTel collector is running"
    else
        print_status "info" "Starting OTel collector..."
        cd "$PROJECT_ROOT"
        docker-compose up -d otel-collector
        sleep 5  # Wait for collector to be ready
    fi
}

# Function to run unit tests
run_unit_tests() {
    print_status "info" "Running unit tests..."
    cd "$PROJECT_ROOT/tests"
    
    if go test -v -race -cover ./... -tags=unit; then
        print_status "success" "Unit tests passed"
    else
        print_status "error" "Unit tests failed"
        return 1
    fi
}

# Function to run integration tests
run_integration_tests() {
    print_status "info" "Running integration tests..."
    cd "$PROJECT_ROOT/tests"
    
    # Install dependencies
    go mod download
    
    if go test -v -race ./integration/... -timeout 10m; then
        print_status "success" "Integration tests passed"
    else
        print_status "error" "Integration tests failed"
        return 1
    fi
}

# Function to run e2e tests
run_e2e_tests() {
    print_status "info" "Running end-to-end tests..."
    cd "$PROJECT_ROOT/tests"
    
    # Download OTel collector if needed
    if [ ! -f "./otelcol" ] && [ ! -f "otelcol-contrib" ]; then
        print_status "info" "Downloading OpenTelemetry Collector..."
        curl -L https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.96.0/otelcol-contrib_0.96.0_linux_amd64.tar.gz | tar xz
    fi
    
    if go test -v ./e2e/... -timeout 15m; then
        print_status "success" "E2E tests passed"
    else
        print_status "error" "E2E tests failed"
        return 1
    fi
}

# Function to generate test report
generate_report() {
    print_status "info" "Generating test report..."
    cd "$PROJECT_ROOT/tests"
    
    # Run tests with coverage
    go test -v -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    
    print_status "success" "Coverage report generated: tests/coverage.html"
}

# Function to cleanup
cleanup() {
    print_status "info" "Cleaning up test environment..."
    
    if [ "$CLEANUP_AFTER_TEST" = "true" ]; then
        cd "$PROJECT_ROOT"
        docker-compose down -v
        print_status "success" "Test environment cleaned up"
    else
        print_status "info" "Test environment left running (set CLEANUP_AFTER_TEST=true to auto-cleanup)"
    fi
}

# Main execution
main() {
    echo ""
    check_prerequisites
    echo ""
    
    case $TEST_MODE in
        "unit")
            run_unit_tests
            ;;
        "integration")
            setup_test_env
            run_integration_tests
            ;;
        "e2e")
            setup_test_env
            run_e2e_tests
            ;;
        "all")
            setup_test_env
            echo ""
            run_unit_tests
            echo ""
            run_integration_tests
            echo ""
            run_e2e_tests
            echo ""
            generate_report
            ;;
        *)
            echo "Usage: $0 [unit|integration|e2e|all]"
            exit 1
            ;;
    esac
    
    local exit_code=$?
    echo ""
    cleanup
    
    if [ $exit_code -eq 0 ]; then
        print_status "success" "All tests completed successfully!"
    else
        print_status "error" "Some tests failed"
        exit $exit_code
    fi
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

# Run main function
main