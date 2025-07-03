#!/bin/bash
# E2E Test Runner Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
TEST_TIMEOUT="${TEST_TIMEOUT:-30m}"
COMPOSE_FILE="testdata/docker-compose.test.yml"
TEST_RESULTS_DIR="test-results"

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

cleanup() {
    log_info "Cleaning up test environment..."
    docker-compose -f $COMPOSE_FILE down -v || true
    rm -rf $TEST_RESULTS_DIR/current
}

# Ensure cleanup on exit
trap cleanup EXIT

# Main script
main() {
    log_info "Starting E2E Test Suite"
    
    # Check prerequisites
    log_info "Checking prerequisites..."
    command -v docker >/dev/null 2>&1 || { log_error "Docker is required but not installed."; exit 1; }
    command -v docker-compose >/dev/null 2>&1 || { log_error "Docker Compose is required but not installed."; exit 1; }
    command -v go >/dev/null 2>&1 || { log_error "Go is required but not installed."; exit 1; }
    
    # Create test results directory
    mkdir -p $TEST_RESULTS_DIR/current
    
    # Set test run ID
    export TEST_RUN_ID="e2e-$(date +%Y%m%d-%H%M%S)"
    log_info "Test Run ID: $TEST_RUN_ID"
    
    # Build test containers if needed
    if [ "$BUILD_CONTAINERS" = "true" ]; then
        log_info "Building test containers..."
        docker-compose -f $COMPOSE_FILE build
    fi
    
    # Start test environment
    log_info "Starting test environment..."
    docker-compose -f $COMPOSE_FILE up -d postgres-test otlp-mock
    
    # Wait for PostgreSQL to be ready
    log_info "Waiting for PostgreSQL to be ready..."
    for i in {1..30}; do
        if docker-compose -f $COMPOSE_FILE exec -T postgres-test pg_isready -U test_user -d test_db >/dev/null 2>&1; then
            log_info "PostgreSQL is ready!"
            break
        fi
        if [ $i -eq 30 ]; then
            log_error "PostgreSQL failed to start in time"
            exit 1
        fi
        sleep 2
    done
    
    # Start collector
    log_info "Starting OpenTelemetry Collector..."
    docker-compose -f $COMPOSE_FILE up -d otel-collector
    
    # Wait for collector to be healthy
    log_info "Waiting for collector to be ready..."
    for i in {1..30}; do
        if curl -sf http://localhost:13133/health >/dev/null 2>&1; then
            log_info "Collector is ready!"
            break
        fi
        if [ $i -eq 30 ]; then
            log_error "Collector failed to start in time"
            exit 1
        fi
        sleep 2
    done
    
    # Run tests based on mode
    case "${TEST_MODE:-all}" in
        "unit")
            log_info "Running unit tests only..."
            go test -v -short ./... | tee $TEST_RESULTS_DIR/current/unit-tests.log
            ;;
        "integration")
            log_info "Running integration tests..."
            go test -v -run TestFullIntegrationE2E -timeout $TEST_TIMEOUT ./... | tee $TEST_RESULTS_DIR/current/integration-tests.log
            ;;
        "performance")
            log_info "Running performance tests..."
            go test -v -run TestPerformanceE2E -timeout $TEST_TIMEOUT ./... | tee $TEST_RESULTS_DIR/current/performance-tests.log
            ;;
        "benchmark")
            log_info "Running benchmarks..."
            go test -bench=. -benchmem -run=^$ ./... | tee $TEST_RESULTS_DIR/current/benchmarks.log
            ;;
        "all")
            log_info "Running all E2E tests..."
            
            # Run tests in sequence
            log_info "1/4: Plan Intelligence tests..."
            go test -v -run TestPlanIntelligenceE2E -timeout $TEST_TIMEOUT ./... | tee $TEST_RESULTS_DIR/current/plan-intelligence-tests.log
            
            log_info "2/4: ASH tests..."
            go test -v -run TestASHE2E -timeout $TEST_TIMEOUT ./... | tee $TEST_RESULTS_DIR/current/ash-tests.log
            
            log_info "3/4: Integration tests..."
            go test -v -run TestFullIntegrationE2E -timeout $TEST_TIMEOUT ./... | tee $TEST_RESULTS_DIR/current/integration-tests.log
            
            log_info "4/4: Metrics validation..."
            go test -v -run TestMetricsToNRDBMapping -timeout $TEST_TIMEOUT ./... | tee $TEST_RESULTS_DIR/current/metrics-validation-tests.log
            ;;
        *)
            log_error "Unknown test mode: $TEST_MODE"
            exit 1
            ;;
    esac
    
    # Collect artifacts
    log_info "Collecting test artifacts..."
    
    # Get collector logs
    docker-compose -f $COMPOSE_FILE logs otel-collector > $TEST_RESULTS_DIR/current/collector.log 2>&1
    
    # Get PostgreSQL logs
    docker-compose -f $COMPOSE_FILE logs postgres-test > $TEST_RESULTS_DIR/current/postgres.log 2>&1
    
    # Export Prometheus metrics
    curl -s http://localhost:8888/metrics > $TEST_RESULTS_DIR/current/prometheus-metrics.txt 2>&1 || true
    
    # Get mock OTLP requests
    curl -s http://localhost:4317/mockserver/retrieve?type=REQUESTS > $TEST_RESULTS_DIR/current/otlp-requests.json 2>&1 || true
    
    # Generate test report
    log_info "Generating test report..."
    generate_report
    
    # Archive results
    ARCHIVE_NAME="e2e-results-${TEST_RUN_ID}.tar.gz"
    tar -czf $TEST_RESULTS_DIR/$ARCHIVE_NAME -C $TEST_RESULTS_DIR current
    log_info "Test results archived: $TEST_RESULTS_DIR/$ARCHIVE_NAME"
    
    log_info "E2E tests completed successfully!"
}

generate_report() {
    cat > $TEST_RESULTS_DIR/current/summary.txt <<EOF
E2E Test Summary
================
Test Run ID: $TEST_RUN_ID
Date: $(date)
Duration: $SECONDS seconds

Test Results:
-------------
EOF
    
    # Parse test results
    for log in $TEST_RESULTS_DIR/current/*-tests.log; do
        if [ -f "$log" ]; then
            test_name=$(basename "$log" .log)
            if grep -q "FAIL" "$log"; then
                echo "❌ $test_name: FAILED" >> $TEST_RESULTS_DIR/current/summary.txt
            elif grep -q "PASS" "$log"; then
                echo "✅ $test_name: PASSED" >> $TEST_RESULTS_DIR/current/summary.txt
            else
                echo "⚠️  $test_name: UNKNOWN" >> $TEST_RESULTS_DIR/current/summary.txt
            fi
        fi
    done
    
    # Add metrics summary
    echo -e "\nCollector Metrics:" >> $TEST_RESULTS_DIR/current/summary.txt
    if [ -f "$TEST_RESULTS_DIR/current/prometheus-metrics.txt" ]; then
        grep -E "otelcol_receiver_accepted_metric_points|otelcol_processor_dropped_metric_points|otelcol_exporter_sent_metric_points" \
            $TEST_RESULTS_DIR/current/prometheus-metrics.txt >> $TEST_RESULTS_DIR/current/summary.txt || true
    fi
    
    cat $TEST_RESULTS_DIR/current/summary.txt
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --mode)
            TEST_MODE="$2"
            shift 2
            ;;
        --timeout)
            TEST_TIMEOUT="$2"
            shift 2
            ;;
        --build)
            BUILD_CONTAINERS="true"
            shift
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --mode <mode>      Test mode: all, unit, integration, performance, benchmark (default: all)"
            echo "  --timeout <time>   Test timeout (default: 30m)"
            echo "  --build           Build containers before running tests"
            echo "  --help            Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Run main function
main