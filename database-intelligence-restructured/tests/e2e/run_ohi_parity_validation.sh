#!/bin/bash

# OHI Parity Validation Runner
# This script runs comprehensive validation to ensure OHI parity

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
CONFIG_DIR="$SCRIPT_DIR/configs/validation"
OUTPUT_DIR="$SCRIPT_DIR/validation-reports"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Default values
MODE="quick"
ENVIRONMENT="development"
CONTINUOUS=false
DASHBOARD_FILE=""
GENERATE_REPORT=true
VERBOSE=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --mode)
            MODE="$2"
            shift 2
            ;;
        --env)
            ENVIRONMENT="$2"
            shift 2
            ;;
        --continuous)
            CONTINUOUS=true
            shift
            ;;
        --dashboard)
            DASHBOARD_FILE="$2"
            shift 2
            ;;
        --no-report)
            GENERATE_REPORT=false
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --mode MODE         Validation mode: quick, comprehensive, drift, all (default: quick)"
            echo "  --env ENV          Environment: development, staging, production (default: development)"
            echo "  --continuous       Run continuous validation (daemon mode)"
            echo "  --dashboard FILE   Path to dashboard JSON file to validate"
            echo "  --no-report       Skip report generation"
            echo "  --verbose         Enable verbose output"
            echo "  --help           Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Log function
log() {
    local level=$1
    shift
    local message="$@"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    case $level in
        INFO)
            echo -e "${BLUE}[INFO]${NC} ${timestamp} - ${message}"
            ;;
        SUCCESS)
            echo -e "${GREEN}[SUCCESS]${NC} ${timestamp} - ${message}"
            ;;
        WARNING)
            echo -e "${YELLOW}[WARNING]${NC} ${timestamp} - ${message}"
            ;;
        ERROR)
            echo -e "${RED}[ERROR]${NC} ${timestamp} - ${message}"
            ;;
    esac
}

# Check prerequisites
check_prerequisites() {
    log INFO "Checking prerequisites..."
    
    # Check for required environment variables
    local required_vars=(
        "NEW_RELIC_ACCOUNT_ID"
        "NEW_RELIC_API_KEY"
        "NEW_RELIC_LICENSE_KEY"
    )
    
    for var in "${required_vars[@]}"; do
        if [[ -z "${!var:-}" ]]; then
            log ERROR "Required environment variable $var is not set"
            exit 1
        fi
    done
    
    # Check for required tools
    local required_tools=(
        "go"
        "docker"
        "docker-compose"
        "jq"
    )
    
    for tool in "${required_tools[@]}"; do
        if ! command -v $tool &> /dev/null; then
            log ERROR "Required tool '$tool' is not installed"
            exit 1
        fi
    done
    
    # Check Docker is running
    if ! docker info &> /dev/null; then
        log ERROR "Docker is not running"
        exit 1
    fi
    
    log SUCCESS "Prerequisites check passed"
}

# Setup test environment
setup_environment() {
    log INFO "Setting up test environment..."
    
    # Start databases if not running
    if ! docker ps | grep -q "postgres"; then
        log INFO "Starting PostgreSQL..."
        docker-compose -f "$PROJECT_ROOT/docker-compose.yml" up -d postgres
        sleep 10
    fi
    
    if ! docker ps | grep -q "mysql"; then
        log INFO "Starting MySQL..."
        docker-compose -f "$PROJECT_ROOT/docker-compose.yml" up -d mysql
        sleep 10
    fi
    
    # Build collector if needed
    if [[ ! -f "$PROJECT_ROOT/dist/database-intelligence" ]]; then
        log INFO "Building collector..."
        cd "$PROJECT_ROOT"
        make build
    fi
    
    # Start collector
    log INFO "Starting collector..."
    # Implementation depends on your collector setup
    
    log SUCCESS "Test environment ready"
}

# Run quick validation
run_quick_validation() {
    log INFO "Running quick validation..."
    
    cd "$SCRIPT_DIR"
    
    local test_output="$OUTPUT_DIR/quick_validation_${TIMESTAMP}.log"
    
    if [[ "$VERBOSE" == "true" ]]; then
        go test -v -run TestOHIParityValidation/TestDatabaseQueryDistribution ./suites/... | tee "$test_output"
        go test -v -run TestOHIParityValidation/TestAverageExecutionTime ./suites/... | tee -a "$test_output"
        go test -v -run TestOHIParityValidation/TestTopWaitEvents ./suites/... | tee -a "$test_output"
    else
        go test -run TestOHIParityValidation/TestDatabaseQueryDistribution ./suites/... > "$test_output" 2>&1
        go test -run TestOHIParityValidation/TestAverageExecutionTime ./suites/... >> "$test_output" 2>&1
        go test -run TestOHIParityValidation/TestTopWaitEvents ./suites/... >> "$test_output" 2>&1
    fi
    
    # Check results
    if grep -q "FAIL" "$test_output"; then
        log ERROR "Quick validation failed. See $test_output for details"
        return 1
    else
        log SUCCESS "Quick validation passed"
        return 0
    fi
}

# Run comprehensive validation
run_comprehensive_validation() {
    log INFO "Running comprehensive validation..."
    
    cd "$SCRIPT_DIR"
    
    local test_output="$OUTPUT_DIR/comprehensive_validation_${TIMESTAMP}.log"
    
    if [[ "$VERBOSE" == "true" ]]; then
        go test -v -timeout 30m ./suites/ohi_parity_validation_test.go | tee "$test_output"
    else
        go test -timeout 30m ./suites/ohi_parity_validation_test.go > "$test_output" 2>&1
    fi
    
    # Extract results summary
    local passed=$(grep -c "PASSED" "$test_output" || true)
    local failed=$(grep -c "FAILED" "$test_output" || true)
    local accuracy=$(grep "Average Accuracy:" "$test_output" | awk '{print $3}')
    
    log INFO "Validation Summary:"
    log INFO "  Passed: $passed"
    log INFO "  Failed: $failed"
    log INFO "  Average Accuracy: $accuracy"
    
    if [[ $failed -eq 0 ]]; then
        log SUCCESS "Comprehensive validation passed"
        return 0
    else
        log ERROR "Comprehensive validation failed"
        return 1
    fi
}

# Run drift detection
run_drift_detection() {
    log INFO "Running drift detection..."
    
    cd "$SCRIPT_DIR"
    
    # Run drift detection using historical data
    go run ./cmd/drift-detector/main.go \
        --history-dir "$OUTPUT_DIR" \
        --baseline-window 168h \
        --detection-window 24h \
        --threshold 0.02 \
        --output "$OUTPUT_DIR/drift_report_${TIMESTAMP}.json"
    
    # Check for drift
    local drift_severity=$(jq -r '.severity' "$OUTPUT_DIR/drift_report_${TIMESTAMP}.json")
    
    case $drift_severity in
        "NONE"|"LOW")
            log SUCCESS "No significant drift detected"
            return 0
            ;;
        "MEDIUM")
            log WARNING "Medium drift detected"
            return 0
            ;;
        "HIGH"|"CRITICAL")
            log ERROR "High/Critical drift detected"
            return 1
            ;;
    esac
}

# Run continuous validation
run_continuous_validation() {
    log INFO "Starting continuous validation..."
    
    cd "$SCRIPT_DIR"
    
    # Create PID file
    local pid_file="$OUTPUT_DIR/continuous_validator.pid"
    
    # Check if already running
    if [[ -f "$pid_file" ]]; then
        local pid=$(cat "$pid_file")
        if ps -p $pid > /dev/null 2>&1; then
            log ERROR "Continuous validator is already running (PID: $pid)"
            exit 1
        fi
    fi
    
    # Start continuous validator
    nohup go run ./cmd/continuous-validator/main.go \
        --config "$CONFIG_DIR/continuous_validation.yaml" \
        --dashboard "$DASHBOARD_FILE" \
        --output-dir "$OUTPUT_DIR" \
        > "$OUTPUT_DIR/continuous_validator.log" 2>&1 &
    
    local pid=$!
    echo $pid > "$pid_file"
    
    log SUCCESS "Continuous validator started (PID: $pid)"
    log INFO "Logs: $OUTPUT_DIR/continuous_validator.log"
}

# Generate validation report
generate_report() {
    log INFO "Generating validation report..."
    
    cd "$SCRIPT_DIR"
    
    # Collect all test results
    local report_file="$OUTPUT_DIR/validation_report_${TIMESTAMP}.html"
    
    go run ./cmd/report-generator/main.go \
        --input-dir "$OUTPUT_DIR" \
        --output "$report_file" \
        --format html \
        --include-charts \
        --include-recommendations
    
    log SUCCESS "Report generated: $report_file"
    
    # Open report in browser if available
    if command -v open &> /dev/null; then
        open "$report_file"
    elif command -v xdg-open &> /dev/null; then
        xdg-open "$report_file"
    fi
}

# Cleanup function
cleanup() {
    log INFO "Cleaning up..."
    
    # Stop collector if we started it
    # Implementation depends on your setup
    
    log INFO "Cleanup complete"
}

# Set trap for cleanup
trap cleanup EXIT

# Main execution
main() {
    log INFO "Starting OHI Parity Validation"
    log INFO "Mode: $MODE"
    log INFO "Environment: $ENVIRONMENT"
    
    # Check prerequisites
    check_prerequisites
    
    # Setup environment
    setup_environment
    
    # Run validation based on mode
    case $MODE in
        quick)
            run_quick_validation
            ;;
        comprehensive)
            run_comprehensive_validation
            ;;
        drift)
            run_drift_detection
            ;;
        all)
            run_quick_validation
            run_comprehensive_validation
            run_drift_detection
            ;;
        *)
            log ERROR "Unknown mode: $MODE"
            exit 1
            ;;
    esac
    
    # Run continuous validation if requested
    if [[ "$CONTINUOUS" == "true" ]]; then
        run_continuous_validation
    fi
    
    # Generate report if requested
    if [[ "$GENERATE_REPORT" == "true" ]]; then
        generate_report
    fi
    
    log SUCCESS "OHI Parity Validation complete"
}

# Run main function
main "$@"