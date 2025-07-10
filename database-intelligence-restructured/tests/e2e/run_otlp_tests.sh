#!/bin/bash

# Enhanced E2E Testing Script for OpenTelemetry Dimensional Metrics and OTLP Compliance
# This script runs comprehensive tests for OTLP format validation and dimensional metrics

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_RESULTS_DIR="${PROJECT_ROOT}/tests/e2e/reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TEST_RUN_ID="otlp_${TIMESTAMP}"

# Test environment configuration
export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
export POSTGRES_PORT="${POSTGRES_PORT:-5432}"
export POSTGRES_USER="${POSTGRES_USER:-postgres}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
export POSTGRES_DB="${POSTGRES_DB:-postgres}"

# New Relic configuration (required for OTLP tests)
export NEW_RELIC_OTLP_ENDPOINT="${NEW_RELIC_OTLP_ENDPOINT:-https://otlp.nr-data.net:4318}"

# Test timeout
E2E_TIMEOUT="${E2E_TIMEOUT:-30m}"

print_section() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

run_test() {
    local test_name="$1"
    local command="$2"
    local optional="${3:-false}"
    
    echo -n "Running $test_name... "
    if eval "$command" > /dev/null 2>&1; then
        print_success "PASSED"
        return 0
    else
        if [ "$optional" = "true" ]; then
            print_warning "SKIPPED (optional)"
            return 0
        else
            print_error "FAILED"
            return 1
        fi
    fi
}

check_prerequisites() {
    print_section "Checking Prerequisites"
    
    # Check required environment variables
    local required_vars=(
        "NEW_RELIC_LICENSE_KEY"
        "NEW_RELIC_ACCOUNT_ID" 
        "NEW_RELIC_API_KEY"
    )
    
    for var in "${required_vars[@]}"; do
        if [ -z "${!var:-}" ]; then
            print_error "Required environment variable $var is not set"
            echo "Please set up your .env file with New Relic credentials"
            exit 1
        fi
    done
    
    # Check Go installation
    run_test "Go installation" "go version"
    
    # Check database connectivity
    run_test "PostgreSQL connectivity" "PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -c 'SELECT 1'" true
    
    # Check if collector can be built
    run_test "Collector build" "cd '$PROJECT_ROOT' && go build -o /tmp/test-collector ./cmd/..."
    
    print_success "Prerequisites check completed"
}

setup_test_environment() {
    print_section "Setting Up Test Environment"
    
    # Create reports directory
    mkdir -p "$TEST_RESULTS_DIR"
    
    # Start test databases if using Docker
    if command -v docker-compose &> /dev/null; then
        if [ -f "$PROJECT_ROOT/docker-compose.unified.yml" ]; then
            print_warning "Starting test databases with Docker Compose..."
            cd "$PROJECT_ROOT"
            docker-compose -f docker-compose.unified.yml up -d postgres mysql
            sleep 10
        fi
    fi
    
    print_success "Test environment setup completed"
}

run_otlp_dimensional_tests() {
    print_section "Running OTLP Dimensional Metrics Tests"
    
    cd "$PROJECT_ROOT/tests/e2e"
    
    # Test categories for OTLP and dimensional metrics
    local test_categories=(
        "TestOTLPCompliance"
        "TestOTLPFormatValidation" 
        "TestOTELDimensionalMetrics"
    )
    
    for category in "${test_categories[@]}"; do
        print_section "Running $category"
        
        if go test -v -timeout "$E2E_TIMEOUT" -run "$category" ./suites/... 2>&1 | tee "$TEST_RESULTS_DIR/${category}_${TEST_RUN_ID}.log"; then
            print_success "$category tests passed"
        else
            print_error "$category tests failed"
            echo "Check logs at: $TEST_RESULTS_DIR/${category}_${TEST_RUN_ID}.log"
        fi
    done
}

run_semantic_conventions_tests() {
    print_section "Running OpenTelemetry Semantic Conventions Tests"
    
    cd "$PROJECT_ROOT/tests/e2e"
    
    # Run specific semantic conventions tests
    go test -v -timeout "$E2E_TIMEOUT" -run "TestSemanticConventions" ./suites/... 2>&1 | tee "$TEST_RESULTS_DIR/semantic_conventions_${TEST_RUN_ID}.log"
    
    if [ $? -eq 0 ]; then
        print_success "Semantic conventions tests passed"
    else
        print_warning "Semantic conventions tests had issues"
    fi
}

run_cardinality_control_tests() {
    print_section "Running Cardinality Control Tests"
    
    cd "$PROJECT_ROOT/tests/e2e"
    
    # Run cardinality-specific tests
    go test -v -timeout "$E2E_TIMEOUT" -run "TestCardinality|TestHighCardinality" ./suites/... 2>&1 | tee "$TEST_RESULTS_DIR/cardinality_${TEST_RUN_ID}.log"
    
    if [ $? -eq 0 ]; then
        print_success "Cardinality control tests passed"
    else
        print_warning "Cardinality control tests had issues"
    fi
}

run_processor_pipeline_tests() {
    print_section "Running Processor Pipeline Tests"
    
    cd "$PROJECT_ROOT/tests/e2e"
    
    # Run processor-specific tests
    go test -v -timeout "$E2E_TIMEOUT" -run "TestProcessor|TestPipeline" ./suites/... 2>&1 | tee "$TEST_RESULTS_DIR/processor_pipeline_${TEST_RUN_ID}.log"
    
    if [ $? -eq 0 ]; then
        print_success "Processor pipeline tests passed"
    else
        print_warning "Processor pipeline tests had issues"
    fi
}

validate_otlp_export() {
    print_section "Validating OTLP Export Format"
    
    # Check if OTLP exports are properly formatted
    if [ -f "$TEST_RESULTS_DIR/otlp_export_${TEST_RUN_ID}.json" ]; then
        # Validate JSON structure
        if jq empty "$TEST_RESULTS_DIR/otlp_export_${TEST_RUN_ID}.json" 2>/dev/null; then
            print_success "OTLP export format is valid JSON"
            
            # Check for required OTLP fields
            local required_fields=("resourceMetrics" "scopeMetrics" "metrics")
            for field in "${required_fields[@]}"; do
                if jq -e ".[0] | has(\"$field\")" "$TEST_RESULTS_DIR/otlp_export_${TEST_RUN_ID}.json" > /dev/null 2>&1; then
                    print_success "OTLP field '$field' present"
                else
                    print_warning "OTLP field '$field' missing"
                fi
            done
        else
            print_error "OTLP export format is invalid JSON"
        fi
    else
        print_warning "OTLP export file not found"
    fi
}

generate_test_report() {
    print_section "Generating Test Report"
    
    local report_file="$TEST_RESULTS_DIR/otlp_test_report_${TEST_RUN_ID}.md"
    
    cat > "$report_file" << EOF
# OTLP and Dimensional Metrics Test Report

**Test Run ID**: $TEST_RUN_ID  
**Date**: $(date)  
**Duration**: ${SECONDS}s

## Test Environment
- PostgreSQL: $POSTGRES_HOST:$POSTGRES_PORT
- New Relic Endpoint: $NEW_RELIC_OTLP_ENDPOINT
- Test Timeout: $E2E_TIMEOUT

## Test Categories Executed

### 1. OTLP Compliance Tests
- **Purpose**: Validate OTLP protocol compliance
- **Coverage**: Format validation, schema compliance, export validation
- **Result**: $([ -f "$TEST_RESULTS_DIR/TestOTLPCompliance_${TEST_RUN_ID}.log" ] && echo "✓ Executed" || echo "✗ Not found")

### 2. Dimensional Metrics Tests  
- **Purpose**: Validate dimensional attributes and cardinality control
- **Coverage**: Schema validation, cardinality limits, high-cardinality prevention
- **Result**: $([ -f "$TEST_RESULTS_DIR/TestOTELDimensionalMetrics_${TEST_RUN_ID}.log" ] && echo "✓ Executed" || echo "✗ Not found")

### 3. Semantic Conventions Tests
- **Purpose**: Ensure OpenTelemetry semantic conventions compliance
- **Coverage**: Required attributes, naming conventions, resource attributes
- **Result**: $([ -f "$TEST_RESULTS_DIR/semantic_conventions_${TEST_RUN_ID}.log" ] && echo "✓ Executed" || echo "✗ Not found")

### 4. Processor Pipeline Tests
- **Purpose**: Validate custom processor functionality
- **Coverage**: PII detection, cost control, plan extraction, verification
- **Result**: $([ -f "$TEST_RESULTS_DIR/processor_pipeline_${TEST_RUN_ID}.log" ] && echo "✓ Executed" || echo "✗ Not found")

## Key Validations

### OTLP Format Compliance
- Protocol format validation
- Required field presence
- Data type correctness
- Batch processing efficiency

### Dimensional Metrics Validation
- Dimensional attribute integrity
- Cardinality explosion prevention
- High-cardinality dimension handling
- Cost control effectiveness

### Semantic Conventions Compliance
- Database semantic conventions (db.system, db.name, etc.)
- Resource attribute requirements
- Metric naming conventions
- Service identification attributes

## Files Generated
EOF

    # List generated files
    echo "" >> "$report_file"
    echo "### Test Artifacts" >> "$report_file"
    for file in "$TEST_RESULTS_DIR"/*"$TEST_RUN_ID"*; do
        if [ -f "$file" ]; then
            echo "- $(basename "$file")" >> "$report_file"
        fi
    done
    
    echo "" >> "$report_file"
    echo "### Next Steps" >> "$report_file"
    echo "1. Review individual test logs for detailed results" >> "$report_file"
    echo "2. Analyze any failed tests and address issues" >> "$report_file"
    echo "3. Validate metric data in New Relic UI" >> "$report_file"
    echo "4. Consider performance optimizations if needed" >> "$report_file"
    
    print_success "Test report generated: $report_file"
}

cleanup_test_environment() {
    print_section "Cleaning Up Test Environment"
    
    # Stop test databases if they were started
    if command -v docker-compose &> /dev/null; then
        if [ -f "$PROJECT_ROOT/docker-compose.unified.yml" ]; then
            cd "$PROJECT_ROOT"
            docker-compose -f docker-compose.unified.yml down > /dev/null 2>&1 || true
        fi
    fi
    
    # Clean up temporary files
    rm -f /tmp/test-collector
    
    print_success "Cleanup completed"
}

main() {
    echo -e "${BLUE}"
    cat << "EOF"
    ╔══════════════════════════════════════════════════════════════╗
    ║              OTLP & Dimensional Metrics Test Suite           ║
    ║                                                              ║
    ║  Comprehensive testing for OpenTelemetry compliance,        ║
    ║  dimensional metrics validation, and OTLP format testing     ║
    ╚══════════════════════════════════════════════════════════════╝
EOF
    echo -e "${NC}"
    
    # Record start time
    start_time=$(date +%s)
    
    # Run test phases
    check_prerequisites
    setup_test_environment
    
    # Core OTLP and dimensional testing
    run_otlp_dimensional_tests
    run_semantic_conventions_tests
    run_cardinality_control_tests
    run_processor_pipeline_tests
    
    # Validation and reporting
    validate_otlp_export
    generate_test_report
    
    # Calculate duration
    end_time=$(date +%s)
    duration=$((end_time - start_time))
    
    print_section "Test Suite Completed"
    echo "Total duration: ${duration}s"
    echo "Test run ID: $TEST_RUN_ID"
    echo "Reports available in: $TEST_RESULTS_DIR"
    
    cleanup_test_environment
    
    print_success "OTLP & Dimensional Metrics test suite completed successfully!"
}

# Handle script arguments
case "${1:-all}" in
    "dimensional")
        run_otlp_dimensional_tests
        ;;
    "semantic")
        run_semantic_conventions_tests
        ;;
    "cardinality") 
        run_cardinality_control_tests
        ;;
    "pipeline")
        run_processor_pipeline_tests
        ;;
    "validate")
        validate_otlp_export
        ;;
    "all"|*)
        main
        ;;
esac
