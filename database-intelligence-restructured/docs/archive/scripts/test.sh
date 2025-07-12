#!/bin/bash

# Unified test runner for database-intelligence
# Consolidates all test and validation functionality

set -e

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/scripts/lib/common.sh"

# Test modes
MODE=${1:-"e2e"}
REPORT_FORMAT=${REPORT_FORMAT:-"markdown"}
REPORT_DIR="test-reports"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

usage() {
    cat << EOF
Usage: $0 [MODE] [OPTIONS]

Modes:
  e2e               Full E2E test with databases (default)
  smoke             Quick smoke test with minimal config
  validate          Validate project structure only
  production        Production-style test with monitoring
  comprehensive     Comprehensive test with detailed reporting
  
Options:
  REPORT_FORMAT=    Output format: markdown (default), html, json
  SKIP_BUILD=true   Skip building collector
  KEEP_RUNNING=true Keep services running after test
  VERBOSE=true      Enable verbose output

Examples:
  $0                           # Run default E2E test
  $0 smoke                     # Quick smoke test
  $0 validate                  # Structure validation only
  $0 production               # Production monitoring test
  VERBOSE=true $0 comprehensive # Detailed comprehensive test
EOF
}

# Ensure test environment
setup_test_environment() {
    log_info "Setting up test environment..."
    
    # Check prerequisites
    if ! check_prerequisites; then
        log_error "Prerequisites check failed"
        exit 1
    fi
    
    # Ensure env file
    ensure_env_file
    
    # Create report directory
    mkdir -p "$REPORT_DIR"
    
    # Sync workspace if needed
    sync_go_workspace
}

# E2E test mode
run_e2e_test() {
    local report_file="$REPORT_DIR/e2e-test-$TIMESTAMP.$REPORT_FORMAT"
    
    log_info "Running E2E tests..."
    
    # Start report
    {
        generate_report_header "E2E Test Report"
        echo "## Test Configuration"
        echo "- Mode: E2E"
        echo "- Timestamp: $TIMESTAMP"
        echo ""
    } > "$report_file"
    
    # Build collector if needed
    if [ "$SKIP_BUILD" != "true" ]; then
        log_info "Building E2E collector..."
        if ! ./build.sh e2e; then
            log_error "Failed to build E2E collector"
            echo "❌ Build failed" >> "$report_file"
            return 1
        fi
        echo "✅ Collector built successfully" >> "$report_file"
    fi
    
    # Start databases
    start_databases
    echo "✅ Databases started" >> "$report_file"
    
    # Test connectivity
    if test_database_connectivity; then
        echo "✅ Database connectivity verified" >> "$report_file"
    else
        echo "❌ Database connectivity failed" >> "$report_file"
        stop_databases
        return 1
    fi
    
    # Create test config
    local test_config="$REPORT_DIR/e2e-test-config.yaml"
    create_e2e_test_config "$test_config"
    
    # Run collector
    log_info "Starting collector..."
    local collector_bin="tests/e2e/e2e-test-collector"
    if [ ! -f "$collector_bin" ]; then
        collector_bin="distributions/production/otelcol-production"
    fi
    
    if [ -f "$collector_bin" ]; then
        # Validate config first
        if validate_collector_config "$test_config" "$collector_bin"; then
            echo "✅ Configuration validated" >> "$report_file"
        else
            echo "❌ Configuration validation failed" >> "$report_file"
            stop_databases
            return 1
        fi
        
        # Run collector in background
        $collector_bin --config="$test_config" > "$REPORT_DIR/collector-$TIMESTAMP.log" 2>&1 &
        local collector_pid=$!
        
        # Wait for collector to start
        sleep 5
        
        # Check if collector is running
        if kill -0 $collector_pid 2>/dev/null; then
            echo "✅ Collector started successfully" >> "$report_file"
            
            # Run tests
            run_e2e_test_suite "$report_file"
            
            # Stop collector
            kill $collector_pid 2>/dev/null || true
            wait $collector_pid 2>/dev/null || true
        else
            echo "❌ Collector failed to start" >> "$report_file"
            cat "$REPORT_DIR/collector-$TIMESTAMP.log" >> "$report_file"
        fi
    else
        echo "❌ Collector binary not found" >> "$report_file"
    fi
    
    # Cleanup
    if [ "$KEEP_RUNNING" != "true" ]; then
        stop_databases
    fi
    
    # Final report
    echo "" >> "$report_file"
    echo "## Summary" >> "$report_file"
    echo "Report saved to: $report_file" >> "$report_file"
    
    log_success "E2E test completed. Report: $report_file"
}

# Smoke test mode
run_smoke_test() {
    log_info "Running smoke test..."
    
    # Simple test with hostmetrics only
    local smoke_config="$REPORT_DIR/smoke-config.yaml"
    cat > "$smoke_config" << 'EOF'
receivers:
  hostmetrics:
    scrapers:
      cpu:
      memory:
      disk:

processors:
  batch:
    timeout: 10s

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [hostmetrics]
      processors: [batch]
      exporters: [debug]
EOF
    
    # Find collector binary
    local collector_bin=""
    for bin in distributions/*/otelcol-* tests/e2e/e2e-test-collector; do
        if [ -f "$bin" ]; then
            collector_bin="$bin"
            break
        fi
    done
    
    if [ -z "$collector_bin" ]; then
        log_error "No collector binary found"
        return 1
    fi
    
    # Run test
    log_info "Running collector for 30 seconds..."
    timeout 30s $collector_bin --config="$smoke_config" || true
    
    log_success "Smoke test completed"
}

# Validation mode
run_validation() {
    local report_file="$REPORT_DIR/validation-$TIMESTAMP.md"
    
    log_info "Running project structure validation..."
    
    {
        generate_report_header "Project Structure Validation Report"
        
        local total_checks=0
        local passed_checks=0
        
        # Check processors
        echo "## Processors Validation"
        for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
            total_checks=$((total_checks + 1))
            if [ -d "processors/$processor" ] && [ -f "processors/$processor/go.mod" ]; then
                echo "✅ $processor processor"
                passed_checks=$((passed_checks + 1))
            else
                echo "❌ $processor processor missing or incomplete"
            fi
        done
        echo ""
        
        # Check receivers
        echo "## Receivers Validation"
        for receiver in ash enhancedsql kernelmetrics; do
            total_checks=$((total_checks + 1))
            if [ -d "receivers/$receiver" ] && [ -f "receivers/$receiver/go.mod" ]; then
                echo "✅ $receiver receiver"
                passed_checks=$((passed_checks + 1))
            else
                echo "❌ $receiver receiver missing or incomplete"
            fi
        done
        echo ""
        
        # Check distributions
        echo "## Distributions Validation"
        for dist in minimal production enterprise; do
            total_checks=$((total_checks + 1))
            if [ -d "distributions/$dist" ] && [ -f "distributions/$dist/go.mod" ]; then
                echo "✅ $dist distribution"
                passed_checks=$((passed_checks + 1))
            else
                echo "❌ $dist distribution missing or incomplete"
            fi
        done
        echo ""
        
        # Summary
        local success_rate=$(calculate_success_rate $passed_checks $total_checks)
        echo "## Summary"
        echo "- Total checks: $total_checks"
        echo "- Passed: $passed_checks"
        echo "- Failed: $((total_checks - passed_checks))"
        echo "- Success rate: $success_rate%"
        
    } > "$report_file"
    
    log_success "Validation completed. Report: $report_file"
}

# Production mode
run_production_test() {
    log_info "Running production-style test..."
    
    # Check for New Relic credentials
    if [ -z "$NEW_RELIC_API_KEY" ]; then
        source .env 2>/dev/null || true
    fi
    
    if [ -z "$NEW_RELIC_API_KEY" ] || [ "$NEW_RELIC_API_KEY" = "your-api-key-here" ]; then
        log_warning "NEW_RELIC_API_KEY not configured, using debug exporter instead"
    fi
    
    # Start databases
    start_databases
    
    # Build if needed
    if [ "$SKIP_BUILD" != "true" ]; then
        ./build.sh production
    fi
    
    # Start collector
    local collector_bin="distributions/production/otelcol-production"
    if [ -f "$collector_bin" ]; then
        log_info "Starting production collector..."
        $collector_bin --config="configs/production.yaml" &
        local collector_pid=$!
        
        # Monitor endpoints
        log_info "Monitoring collector endpoints..."
        sleep 5
        
        # Check health
        if curl -s http://localhost:13133/health > /dev/null; then
            log_success "Health endpoint responding"
        fi
        
        # Check metrics
        if curl -s http://localhost:8888/metrics > /dev/null; then
            log_success "Metrics endpoint responding"
        fi
        
        # Let it run for a bit
        if [ "$KEEP_RUNNING" = "true" ]; then
            log_info "Collector running. Press Ctrl+C to stop..."
            wait $collector_pid
        else
            sleep 30
            kill $collector_pid 2>/dev/null || true
        fi
    else
        log_error "Production collector not found"
        return 1
    fi
    
    # Cleanup
    if [ "$KEEP_RUNNING" != "true" ]; then
        stop_databases
    fi
    
    log_success "Production test completed"
}

# Helper functions
create_e2e_test_config() {
    local config_file="$1"
    
    cat > "$config_file" << 'EOF'
receivers:
  postgresql:
    endpoint: localhost:5432
    username: testuser
    password: testpass
    databases:
      - testdb
    collection_interval: 10s
  
  mysql:
    endpoint: localhost:3306
    username: testuser
    password: testpass
    database: testdb
    collection_interval: 10s
  
  hostmetrics:
    scrapers:
      cpu:
      memory:

processors:
  batch:
    timeout: 10s
  
  memory_limiter:
    check_interval: 1s
    limit_mib: 512

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 10
  
  file:
    path: /tmp/otel-metrics.json
    rotation:
      enabled: true
      max_megabytes: 10
      max_days: 3

service:
  pipelines:
    metrics:
      receivers: [postgresql, mysql, hostmetrics]
      processors: [memory_limiter, batch]
      exporters: [debug, file]
  
  extensions: [health_check, zpages]
  
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  
  zpages:
    endpoint: 0.0.0.0:55679
EOF
}

run_e2e_test_suite() {
    local report_file="$1"
    
    echo "" >> "$report_file"
    echo "## Test Suite Execution" >> "$report_file"
    
    # Test OTLP endpoint
    if nc -z localhost 4317 2>/dev/null; then
        echo "✅ OTLP endpoint (4317) is listening" >> "$report_file"
    else
        echo "❌ OTLP endpoint (4317) not responding" >> "$report_file"
    fi
    
    # Test metrics endpoint
    if curl -s http://localhost:8888/metrics > /dev/null; then
        echo "✅ Metrics endpoint (8888) responding" >> "$report_file"
    else
        echo "❌ Metrics endpoint (8888) not responding" >> "$report_file"
    fi
    
    # Test health endpoint
    if curl -s http://localhost:13133/health > /dev/null; then
        echo "✅ Health endpoint (13133) responding" >> "$report_file"
    else
        echo "❌ Health endpoint (13133) not responding" >> "$report_file"
    fi
    
    # Check for metrics in output file
    sleep 10  # Give time for metrics to be collected
    if [ -f "/tmp/otel-metrics.json" ]; then
        echo "✅ Metrics file created" >> "$report_file"
        local line_count=$(wc -l < /tmp/otel-metrics.json)
        echo "  - Lines written: $line_count" >> "$report_file"
    else
        echo "❌ No metrics file found" >> "$report_file"
    fi
}

# Main execution
main() {
    setup_test_environment
    
    case "$MODE" in
        e2e)
            run_e2e_test
            ;;
        smoke)
            run_smoke_test
            ;;
        validate)
            run_validation
            ;;
        production)
            run_production_test
            ;;
        comprehensive)
            # Run all tests
            run_validation
            run_smoke_test
            run_e2e_test
            ;;
        help|--help|-h)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown mode: $MODE"
            usage
            exit 1
            ;;
    esac
}

# Run main function
main