#!/bin/bash
# Automated setup validation tests for Database Intelligence MVP
# Validates both Standard and Experimental deployments

set -euo pipefail

# Script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

# Test configuration
MODE="${1:-standard}"
TIMEOUT=300  # 5 minutes max for all tests
TEST_RESULTS=()
TESTS_PASSED=0
TESTS_FAILED=0

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Helper functions
log() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    TEST_RESULTS+=("PASS: $1")
    ((TESTS_PASSED++))
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    TEST_RESULTS+=("FAIL: $1")
    ((TESTS_FAILED++))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Wait for condition with timeout
wait_for_condition() {
    local condition="$1"
    local timeout="$2"
    local message="$3"
    
    local elapsed=0
    while ! eval "$condition"; do
        if [[ $elapsed -ge $timeout ]]; then
            return 1
        fi
        sleep 2
        ((elapsed+=2))
    done
    return 0
}

# Test 1: Verify deployment is running
test_deployment_running() {
    log "Testing deployment status..."
    
    local container_name="db-intel-primary"
    local health_port=13133
    local metrics_port=8888
    
    if [[ "$MODE" == "experimental" ]]; then
        container_name="db-intel-experimental"
        health_port=13134
        metrics_port=8889
    fi
    
    # Check container is running
    if docker ps --format "table {{.Names}}" | grep -q "$container_name"; then
        pass "Container $container_name is running"
    else
        fail "Container $container_name is not running"
        return 1
    fi
    
    # Check health endpoint
    if curl -sf "http://localhost:${health_port}/" > /dev/null; then
        pass "Health endpoint responding on port $health_port"
    else
        fail "Health endpoint not responding on port $health_port"
    fi
    
    # Check metrics endpoint
    if curl -sf "http://localhost:${metrics_port}/metrics" > /dev/null; then
        pass "Metrics endpoint responding on port $metrics_port"
    else
        fail "Metrics endpoint not responding on port $metrics_port"
    fi
}

# Test 2: Verify data collection
test_data_collection() {
    log "Testing data collection..."
    
    local metrics_port=8888
    if [[ "$MODE" == "experimental" ]]; then
        metrics_port=8889
    fi
    
    # Get initial metrics
    local initial_accepted=$(curl -s "http://localhost:${metrics_port}/metrics" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
    
    # Wait up to 2 minutes for data collection
    log "Waiting for data collection (up to 2 minutes)..."
    
    if wait_for_condition \
        "curl -s http://localhost:${metrics_port}/metrics | grep -q 'otelcol_receiver_accepted_log_records_total'" \
        120 \
        "metrics to appear"; then
        
        sleep 10  # Wait a bit more for metrics to increment
        
        local current_accepted=$(curl -s "http://localhost:${metrics_port}/metrics" | grep "otelcol_receiver_accepted_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
        
        if [[ "$current_accepted" -gt "$initial_accepted" ]]; then
            pass "Data collection verified (accepted: $current_accepted)"
        else
            fail "No new data collected (still at: $current_accepted)"
        fi
    else
        fail "Metrics endpoint not producing data"
    fi
}

# Test 3: Verify experimental features (if applicable)
test_experimental_features() {
    if [[ "$MODE" != "experimental" ]]; then
        return
    fi
    
    log "Testing experimental features..."
    
    local metrics_port=8889
    local metrics=$(curl -s "http://localhost:${metrics_port}/metrics")
    
    # Check for ASH metrics
    if echo "$metrics" | grep -q "db_intelligence_ash_samples_total"; then
        pass "ASH sampling metrics found"
    else
        warn "ASH sampling metrics not found (may need more time)"
    fi
    
    # Check for circuit breaker metrics
    if echo "$metrics" | grep -q "db_intelligence_circuitbreaker"; then
        pass "Circuit breaker metrics found"
    else
        fail "Circuit breaker metrics not found"
    fi
    
    # Check for adaptive sampler metrics
    if echo "$metrics" | grep -q "db_intelligence_adaptivesampler"; then
        pass "Adaptive sampler metrics found"
    else
        warn "Adaptive sampler metrics not found (may need more time)"
    fi
}

# Test 4: Verify resource usage
test_resource_usage() {
    log "Testing resource usage..."
    
    local container_name="db-intel-primary"
    local expected_memory_mb=512
    
    if [[ "$MODE" == "experimental" ]]; then
        container_name="db-intel-experimental"
        expected_memory_mb=2048
    fi
    
    # Get container stats
    local stats=$(docker stats --no-stream --format "json" "$container_name" 2>/dev/null || echo "{}")
    
    if [[ "$stats" != "{}" ]]; then
        # Extract memory usage (this is platform-dependent)
        local mem_usage=$(echo "$stats" | jq -r '.MemUsage' | awk '{print $1}' | sed 's/[^0-9.]//g')
        local mem_unit=$(echo "$stats" | jq -r '.MemUsage' | awk '{print $1}' | sed 's/[0-9.]//g')
        
        # Convert to MB if needed
        if [[ "$mem_unit" == "GiB" ]]; then
            mem_usage=$(echo "$mem_usage * 1024" | bc)
        fi
        
        log "Memory usage: ${mem_usage}MB (expected < ${expected_memory_mb}MB)"
        
        # Just log, don't fail on memory (it varies)
        if (( $(echo "$mem_usage < $expected_memory_mb" | bc -l) )); then
            pass "Memory usage within limits"
        else
            warn "Memory usage higher than expected"
        fi
    else
        warn "Could not get container stats"
    fi
}

# Test 5: Verify data export
test_data_export() {
    log "Testing data export..."
    
    local metrics_port=8888
    if [[ "$MODE" == "experimental" ]]; then
        metrics_port=8889
    fi
    
    # Check exporter metrics
    local exported=$(curl -s "http://localhost:${metrics_port}/metrics" | grep "otelcol_exporter_sent_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
    
    if [[ "$exported" -gt 0 ]]; then
        pass "Data export verified (exported: $exported records)"
    else
        # Check for export errors
        local export_errors=$(curl -s "http://localhost:${metrics_port}/metrics" | grep "otelcol_exporter_send_failed_log_records_total" | tail -1 | awk '{print $2}' || echo "0")
        
        if [[ "$export_errors" -gt 0 ]]; then
            fail "Data export failing (errors: $export_errors)"
        else
            warn "No data exported yet (may need more time or valid license key)"
        fi
    fi
}

# Test 6: Verify configuration
test_configuration() {
    log "Testing configuration..."
    
    local container_name="db-intel-primary"
    if [[ "$MODE" == "experimental" ]]; then
        container_name="db-intel-experimental"
    fi
    
    # Check if required environment variables are set
    local env_vars=$(docker inspect "$container_name" --format '{{range .Config.Env}}{{println .}}{{end}}')
    
    if echo "$env_vars" | grep -q "PG_REPLICA_DSN="; then
        pass "PostgreSQL DSN configured"
    else
        fail "PostgreSQL DSN not configured"
    fi
    
    if echo "$env_vars" | grep -q "NEW_RELIC_LICENSE_KEY="; then
        pass "New Relic license key configured"
    else
        fail "New Relic license key not configured"
    fi
}

# Test 7: Verify logs for errors
test_logs_for_errors() {
    log "Checking logs for errors..."
    
    local container_name="db-intel-primary"
    if [[ "$MODE" == "experimental" ]]; then
        container_name="db-intel-experimental"
    fi
    
    # Get recent logs
    local logs=$(docker logs "$container_name" --tail 100 2>&1)
    
    # Check for common error patterns
    local error_count=0
    
    if echo "$logs" | grep -i "panic" > /dev/null; then
        fail "Found panic in logs"
        ((error_count++))
    fi
    
    if echo "$logs" | grep -i "fatal" > /dev/null; then
        fail "Found fatal error in logs"
        ((error_count++))
    fi
    
    if echo "$logs" | grep -i "failed to connect" > /dev/null; then
        warn "Found connection failures in logs (check database access)"
        ((error_count++))
    fi
    
    if [[ $error_count -eq 0 ]]; then
        pass "No critical errors found in logs"
    fi
}

# Generate test report
generate_report() {
    echo ""
    echo "======================================"
    echo "    SETUP VALIDATION REPORT"
    echo "======================================"
    echo ""
    echo "Mode: $(echo $MODE | tr '[:lower:]' '[:upper:]')"
    echo "Date: $(date)"
    echo ""
    echo "Test Summary:"
    echo "  ✅ Passed: $TESTS_PASSED"
    echo "  ❌ Failed: $TESTS_FAILED"
    echo ""
    
    echo "Detailed Results:"
    for result in "${TEST_RESULTS[@]}"; do
        echo "  $result"
    done
    echo ""
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        echo -e "${GREEN}✅ All validation tests passed!${NC}"
        echo ""
        echo "Your Database Intelligence MVP deployment is working correctly."
    else
        echo -e "${RED}❌ Some validation tests failed!${NC}"
        echo ""
        echo "Please check the failed tests and review:"
        echo "  - Container logs: docker logs db-intel-${MODE/experimental/experimental}"
        echo "  - Troubleshooting guide: TROUBLESHOOTING-GUIDE.md"
    fi
    
    echo ""
    echo "======================================"
}

# Cleanup function
cleanup() {
    log "Cleaning up test resources..."
    # Add any cleanup needed
}

# Main execution
main() {
    echo "Database Intelligence MVP - Setup Validation"
    echo ""
    
    # Check if deployment exists
    local container_name="db-intel-primary"
    if [[ "$MODE" == "experimental" ]]; then
        container_name="db-intel-experimental"
    fi
    
    if ! docker ps --format "table {{.Names}}" | grep -q "$container_name"; then
        echo -e "${RED}Error: No $MODE deployment found!${NC}"
        echo ""
        echo "Please deploy first:"
        if [[ "$MODE" == "experimental" ]]; then
            echo "  ./quickstart.sh --experimental start"
        else
            echo "  ./quickstart.sh start"
        fi
        exit 1
    fi
    
    # Set trap for cleanup
    trap cleanup EXIT
    
    # Run all tests
    test_deployment_running
    test_data_collection
    test_experimental_features
    test_resource_usage
    test_data_export
    test_configuration
    test_logs_for_errors
    
    # Generate report
    generate_report
    
    # Exit with appropriate code
    if [[ $TESTS_FAILED -gt 0 ]]; then
        exit 1
    else
        exit 0
    fi
}

# Show usage
if [[ "${1:-}" == "--help" ]] || [[ "${1:-}" == "-h" ]]; then
    echo "Usage: $0 [standard|experimental]"
    echo ""
    echo "Validate Database Intelligence MVP deployment."
    echo ""
    echo "Options:"
    echo "  standard      Validate standard mode deployment (default)"
    echo "  experimental  Validate experimental mode deployment"
    echo ""
    exit 0
fi

# Run main
main