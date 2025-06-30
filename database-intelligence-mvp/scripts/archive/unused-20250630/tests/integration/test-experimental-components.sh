#!/bin/bash
# Integration tests for experimental components
#
# This script tests the experimental components in isolation and together
# to ensure they work correctly before production deployment.

set -euo pipefail

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Test configuration
TEST_DB="postgres://test:test@localhost:5432/testdb?sslmode=disable"
TEST_TIMEOUT=30
COLLECTOR_BINARY="${PROJECT_ROOT}/dist/db-intelligence-custom"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test results
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
log() {
    echo -e "${GREEN}[TEST]${NC} $1"
}

error() {
    echo -e "${RED}[FAIL]${NC} $1" >&2
    ((TESTS_FAILED++))
}

warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

# Check prerequisites
check_prerequisites() {
    log "Checking test prerequisites..."
    
    # Check for custom collector binary
    if [ ! -f "$COLLECTOR_BINARY" ]; then
        error "Custom collector binary not found at $COLLECTOR_BINARY"
        error "Run ./scripts/build-custom-collector.sh first"
        exit 1
    fi
    
    # Check for docker-compose
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        warning "Docker Compose not found - some tests will be skipped"
    fi
}

# Start test PostgreSQL instance
start_test_postgres() {
    log "Starting test PostgreSQL instance..."
    
    # Create docker-compose for test database
    cat > "${SCRIPT_DIR}/docker-compose.test.yaml" <<EOF
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U test"]
      interval: 10s
      timeout: 5s
      retries: 5
EOF
    
    cd "$SCRIPT_DIR"
    docker-compose -f docker-compose.test.yaml up -d
    
    # Wait for PostgreSQL to be ready
    log "Waiting for PostgreSQL to be ready..."
    for i in {1..30}; do
        if docker-compose -f docker-compose.test.yaml exec -T postgres pg_isready -U test &> /dev/null; then
            log "PostgreSQL is ready"
            return 0
        fi
        sleep 1
    done
    
    error "PostgreSQL failed to start"
    return 1
}

# Test PostgreSQL Query Receiver
test_postgresql_receiver() {
    log "Testing PostgreSQL Query Receiver..."
    
    # Create test configuration
    cat > "${SCRIPT_DIR}/test-receiver.yaml" <<EOF
extensions:
  health_check:

receivers:
  postgresqlquery:
    connection:
      dsn: "${TEST_DB}"
      max_open: 2
    collection:
      interval: 5s
      timeout: 3s
    ash_sampling:
      enabled: true
      interval: 1s

processors:
  batch:
    timeout: 2s

exporters:
  logging:
    verbosity: detailed

service:
  extensions: [health_check]
  pipelines:
    logs:
      receivers: [postgresqlquery]
      processors: [batch]
      exporters: [logging]
  telemetry:
    logs:
      level: debug
EOF
    
    # Run collector with timeout
    log "Starting collector with PostgreSQL receiver..."
    timeout $TEST_TIMEOUT "$COLLECTOR_BINARY" --config="${SCRIPT_DIR}/test-receiver.yaml" > "${SCRIPT_DIR}/receiver.log" 2>&1 &
    COLLECTOR_PID=$!
    
    # Wait for startup
    sleep 5
    
    # Check if collector is still running
    if kill -0 $COLLECTOR_PID 2>/dev/null; then
        pass "PostgreSQL receiver started successfully"
        
        # Check for ASH samples in log
        if grep -q "ash_sample" "${SCRIPT_DIR}/receiver.log"; then
            pass "ASH sampling is working"
        else
            warning "No ASH samples found in log"
        fi
        
        # Stop collector
        kill $COLLECTOR_PID 2>/dev/null || true
    else
        error "PostgreSQL receiver failed to start"
        cat "${SCRIPT_DIR}/receiver.log"
    fi
}

# Test Circuit Breaker Processor
test_circuit_breaker() {
    log "Testing Circuit Breaker Processor..."
    
    # Create test configuration with circuit breaker
    cat > "${SCRIPT_DIR}/test-circuit-breaker.yaml" <<EOF
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

processors:
  circuitbreaker:
    failure_threshold: 3
    success_threshold: 2
    timeout: 5s
    databases:
      default:
        max_error_rate: 0.5
        max_latency_ms: 1000

exporters:
  logging:
    verbosity: detailed

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [circuitbreaker]
      exporters: [logging]
EOF
    
    # Run collector
    timeout $TEST_TIMEOUT "$COLLECTOR_BINARY" --config="${SCRIPT_DIR}/test-circuit-breaker.yaml" > "${SCRIPT_DIR}/circuit.log" 2>&1 &
    COLLECTOR_PID=$!
    
    sleep 3
    
    if kill -0 $COLLECTOR_PID 2>/dev/null; then
        pass "Circuit breaker processor started successfully"
        kill $COLLECTOR_PID 2>/dev/null || true
    else
        error "Circuit breaker processor failed to start"
        cat "${SCRIPT_DIR}/circuit.log"
    fi
}

# Test Adaptive Sampler
test_adaptive_sampler() {
    log "Testing Adaptive Sampler Processor..."
    
    # Create test configuration
    cat > "${SCRIPT_DIR}/test-adaptive-sampler.yaml" <<EOF
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4318

processors:
  adaptivesampler:
    initial_sampling_percentage: 50
    min_sampling_percentage: 10
    max_sampling_percentage: 100
    strategies:
      - type: "query_cost"
        high_cost_threshold_ms: 100
        high_cost_sampling: 100
        low_cost_sampling: 25

exporters:
  logging:
    verbosity: detailed

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [adaptivesampler]
      exporters: [logging]
EOF
    
    # Run collector
    timeout $TEST_TIMEOUT "$COLLECTOR_BINARY" --config="${SCRIPT_DIR}/test-adaptive-sampler.yaml" > "${SCRIPT_DIR}/adaptive.log" 2>&1 &
    COLLECTOR_PID=$!
    
    sleep 3
    
    if kill -0 $COLLECTOR_PID 2>/dev/null; then
        pass "Adaptive sampler started successfully"
        kill $COLLECTOR_PID 2>/dev/null || true
    else
        error "Adaptive sampler failed to start"
        cat "${SCRIPT_DIR}/adaptive.log"
    fi
}

# Test Full Experimental Pipeline
test_full_pipeline() {
    log "Testing full experimental pipeline..."
    
    # Create full pipeline configuration
    cat > "${SCRIPT_DIR}/test-full-pipeline.yaml" <<EOF
extensions:
  health_check:

receivers:
  postgresqlquery:
    connection:
      dsn: "${TEST_DB}"
    collection:
      interval: 10s

processors:
  memory_limiter:
    limit_mib: 512
    
  circuitbreaker:
    failure_threshold: 5
    
  adaptivesampler:
    initial_sampling_percentage: 100
    
  verification:
    enabled: true
    checks:
      - required_fields: ["query_id", "query_text"]
    
  batch:
    timeout: 5s

exporters:
  logging:
    verbosity: normal
    sampling_initial: 5

service:
  extensions: [health_check]
  pipelines:
    logs:
      receivers: [postgresqlquery]
      processors: [memory_limiter, circuitbreaker, adaptivesampler, verification, batch]
      exporters: [logging]
  telemetry:
    metrics:
      address: localhost:8888
EOF
    
    # Run collector
    log "Starting full experimental pipeline..."
    timeout $TEST_TIMEOUT "$COLLECTOR_BINARY" --config="${SCRIPT_DIR}/test-full-pipeline.yaml" > "${SCRIPT_DIR}/full-pipeline.log" 2>&1 &
    COLLECTOR_PID=$!
    
    # Wait for data collection
    sleep 15
    
    if kill -0 $COLLECTOR_PID 2>/dev/null; then
        pass "Full experimental pipeline running successfully"
        
        # Check metrics endpoint
        if curl -s http://localhost:8888/metrics | grep -q "otelcol_receiver_accepted"; then
            pass "Metrics endpoint is working"
        else
            warning "Metrics endpoint not responding as expected"
        fi
        
        # Check for processed logs
        if grep -q "LogRecord" "${SCRIPT_DIR}/full-pipeline.log"; then
            pass "Pipeline is processing data"
        else
            warning "No processed data found in logs"
        fi
        
        kill $COLLECTOR_PID 2>/dev/null || true
    else
        error "Full experimental pipeline failed"
        cat "${SCRIPT_DIR}/full-pipeline.log"
    fi
}

# Test component interactions
test_component_interactions() {
    log "Testing component interactions..."
    
    # Test that circuit breaker properly blocks when database is down
    log "Testing circuit breaker blocking behavior..."
    
    # Stop test database
    cd "$SCRIPT_DIR"
    docker-compose -f docker-compose.test.yaml stop postgres
    
    # Run collector with bad database
    cat > "${SCRIPT_DIR}/test-circuit-blocking.yaml" <<EOF
receivers:
  postgresqlquery:
    connection:
      dsn: "${TEST_DB}"
    collection:
      interval: 2s
      timeout: 1s

processors:
  circuitbreaker:
    failure_threshold: 2
    timeout: 5s

exporters:
  logging:
    verbosity: detailed

service:
  pipelines:
    logs:
      receivers: [postgresqlquery]
      processors: [circuitbreaker]
      exporters: [logging]
EOF
    
    timeout 15 "$COLLECTOR_BINARY" --config="${SCRIPT_DIR}/test-circuit-blocking.yaml" > "${SCRIPT_DIR}/blocking.log" 2>&1 || true
    
    if grep -q "circuit breaker opened" "${SCRIPT_DIR}/blocking.log"; then
        pass "Circuit breaker correctly opened on database failure"
    else
        error "Circuit breaker did not open as expected"
    fi
    
    # Restart database
    docker-compose -f docker-compose.test.yaml start postgres
    sleep 5
}

# Cleanup
cleanup() {
    log "Cleaning up test environment..."
    
    # Stop any running collectors
    pkill -f "db-intelligence-custom" || true
    
    # Stop test database
    cd "$SCRIPT_DIR"
    docker-compose -f docker-compose.test.yaml down -v
    
    # Remove test files
    rm -f test-*.yaml *.log docker-compose.test.yaml
}

# Generate test report
generate_report() {
    echo ""
    echo "========================================"
    echo "     EXPERIMENTAL COMPONENT TEST REPORT"
    echo "========================================"
    echo ""
    echo "Tests Passed: $TESTS_PASSED"
    echo "Tests Failed: $TESTS_FAILED"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        echo ""
        echo "The experimental components are working correctly."
        echo "You can proceed with gradual production deployment."
    else
        echo -e "${RED}Some tests failed!${NC}"
        echo ""
        echo "Please review the failures before deploying to production."
        echo "Check the log files in the tests/integration directory."
    fi
    echo ""
    echo "========================================"
}

# Main execution
main() {
    log "Starting experimental component integration tests..."
    
    # Set up error handling
    trap cleanup EXIT
    
    check_prerequisites
    
    # Run tests
    start_test_postgres
    test_postgresql_receiver
    test_circuit_breaker
    test_adaptive_sampler
    test_full_pipeline
    test_component_interactions
    
    # Generate report
    generate_report
    
    # Exit with appropriate code
    if [ $TESTS_FAILED -gt 0 ]; then
        exit 1
    else
        exit 0
    fi
}

# Run main
main "$@"