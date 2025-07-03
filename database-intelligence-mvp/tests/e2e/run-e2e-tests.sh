#!/bin/bash
# Unified E2E Test Runner - World-Class Testing Framework
# Orchestrates both shell and Go-based testing with comprehensive reporting

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../../scripts/lib/common.sh"

# Configuration
DEFAULT_CONFIG="${SCRIPT_DIR}/config/unified_test_config.yaml"
OUTPUT_DIR="${SCRIPT_DIR}/results/$(date +%Y%m%d_%H%M%S)"
ORCHESTRATOR_BINARY="${SCRIPT_DIR}/orchestrator/orchestrator"

# Test execution parameters
ENVIRONMENT="local"
PARALLEL_MODE=true
MAX_CONCURRENCY=4
TIMEOUT="30m"
VERBOSE=false
DRY_RUN=false
CONTINUE_ON_ERROR=false
SUITE_FILTER=""
REPORT_FORMATS="json,html,junit"

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Unified E2E Test Runner for Database Intelligence MVP

OPTIONS:
    -e, --environment ENV       Test environment (local, kubernetes, ci) [default: local]
    -c, --config FILE          Test configuration file [default: config/unified_test_config.yaml]
    -s, --suite SUITE          Test suite to run (can be specified multiple times)
    -p, --parallel             Enable parallel execution [default: true]
    -j, --max-concurrency N    Maximum concurrent test suites [default: 4]
    -t, --timeout DURATION     Global timeout for test execution [default: 30m]
    -o, --output DIR           Output directory for results [default: auto-generated]
    -f, --format FORMATS       Report formats (json,html,junit) [default: json,html,junit]
    -v, --verbose              Enable verbose logging
    -n, --dry-run              Show execution plan without running tests
    --continue-on-error        Continue execution after test failures
    --build                    Build orchestrator before running tests
    --clean                    Clean up previous test artifacts
    --quick                    Run quick test suite only
    --full                     Run full comprehensive test suite
    --security                 Run security and compliance tests only
    --performance              Run performance tests only
    -h, --help                 Show this help message

EXAMPLES:
    # Run all tests in local environment
    $0

    # Run specific test suites
    $0 --suite core_pipeline --suite security

    # Run in Kubernetes with verbose output
    $0 --environment kubernetes --verbose

    # Quick validation run
    $0 --quick --dry-run

    # Full performance and security testing
    $0 --full --security --performance --timeout 60m

    # Parallel execution with custom concurrency
    $0 --parallel --max-concurrency 8

    # Generate specific report formats
    $0 --format json,html --output /tmp/test-results

SUITE TYPES:
    core_pipeline     - Core data pipeline functionality
    database          - Database integration testing
    security          - Security and compliance validation
    performance       - Load, stress, and endurance testing
    newrelic          - New Relic integration testing
    failure_scenarios - Failure recovery testing
    deployment        - Deployment and scaling testing

ENVIRONMENTS:
    local       - Docker Compose based local testing
    kubernetes  - Kubernetes cluster testing
    ci          - Optimized for CI/CD pipelines

EOF
}

parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -e|--environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            -c|--config)
                DEFAULT_CONFIG="$2"
                shift 2
                ;;
            -s|--suite)
                if [[ -z "$SUITE_FILTER" ]]; then
                    SUITE_FILTER="$2"
                else
                    SUITE_FILTER="$SUITE_FILTER,$2"
                fi
                shift 2
                ;;
            -p|--parallel)
                PARALLEL_MODE=true
                shift
                ;;
            -j|--max-concurrency)
                MAX_CONCURRENCY="$2"
                shift 2
                ;;
            -t|--timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            -o|--output)
                OUTPUT_DIR="$2"
                shift 2
                ;;
            -f|--format)
                REPORT_FORMATS="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -n|--dry-run)
                DRY_RUN=true
                shift
                ;;
            --continue-on-error)
                CONTINUE_ON_ERROR=true
                shift
                ;;
            --build)
                BUILD_ORCHESTRATOR=true
                shift
                ;;
            --clean)
                CLEAN_ARTIFACTS=true
                shift
                ;;
            --quick)
                SUITE_FILTER="core_pipeline"
                TIMEOUT="10m"
                shift
                ;;
            --full)
                SUITE_FILTER="core_pipeline,database,security,performance,newrelic"
                TIMEOUT="60m"
                shift
                ;;
            --security)
                if [[ -z "$SUITE_FILTER" ]]; then
                    SUITE_FILTER="security"
                else
                    SUITE_FILTER="$SUITE_FILTER,security"
                fi
                shift
                ;;
            --performance)
                if [[ -z "$SUITE_FILTER" ]]; then
                    SUITE_FILTER="performance"
                else
                    SUITE_FILTER="$SUITE_FILTER,performance"
                fi
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
}

validate_prerequisites() {
    log_info "Validating prerequisites..."
    
    # Check required tools
    local required_tools=("docker" "docker-compose" "go" "curl" "jq")
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            log_error "Required tool not found: $tool"
            return 1
        fi
    done
    
    # Check Go version
    local go_version
    go_version=$(go version | awk '{print $3}' | sed 's/go//')
    if ! version_ge "$go_version" "1.21.0"; then
        log_error "Go version 1.21+ required, found: $go_version"
        return 1
    fi
    
    # Check Docker daemon
    if ! docker info &> /dev/null; then
        log_error "Docker daemon not running"
        return 1
    fi
    
    # Check configuration file
    if [[ ! -f "$DEFAULT_CONFIG" ]]; then
        log_error "Configuration file not found: $DEFAULT_CONFIG"
        return 1
    fi
    
    # Validate environment
    case "$ENVIRONMENT" in
        local|kubernetes|ci)
            ;;
        *)
            log_error "Invalid environment: $ENVIRONMENT"
            return 1
            ;;
    esac
    
    log_success "Prerequisites validation completed"
}

version_ge() {
    local version1=$1
    local version2=$2
    printf '%s\n%s\n' "$version2" "$version1" | sort -V -C
}

setup_environment() {
    log_info "Setting up test environment..."
    
    # Create output directory
    mkdir -p "$OUTPUT_DIR"
    
    # Set environment variables
    export TEST_EXECUTION_ID="e2e_${ENVIRONMENT}_$(date +%s)"
    export TEST_OUTPUT_DIR="$OUTPUT_DIR"
    export TEST_ENVIRONMENT="$ENVIRONMENT"
    export TEST_VERBOSE="$VERBOSE"
    
    # Setup environment-specific configuration
    case "$ENVIRONMENT" in
        local)
            setup_local_environment
            ;;
        kubernetes)
            setup_kubernetes_environment
            ;;
        ci)
            setup_ci_environment
            ;;
    esac
    
    log_success "Environment setup completed"
}

setup_local_environment() {
    log_debug "Setting up local Docker environment..."
    
    # Check available resources
    local available_memory
    available_memory=$(docker system info | grep "Total Memory" | awk '{print $3}' | sed 's/GiB//')
    if (( $(echo "$available_memory < 4" | bc -l) )); then
        log_warning "Low memory available: ${available_memory}GB. Recommend 4GB+ for full testing"
    fi
    
    # Pull required images
    log_info "Pulling required Docker images..."
    docker-compose -f "${SCRIPT_DIR}/docker-compose.e2e.yml" pull --quiet
}

setup_kubernetes_environment() {
    log_debug "Setting up Kubernetes environment..."
    
    # Check kubectl access
    if ! kubectl cluster-info &> /dev/null; then
        log_error "No access to Kubernetes cluster"
        return 1
    fi
    
    # Create test namespace
    local namespace="e2e-test-$(date +%s)"
    export TEST_NAMESPACE="$namespace"
    
    kubectl create namespace "$namespace" || true
    kubectl config set-context --current --namespace="$namespace"
}

setup_ci_environment() {
    log_debug "Setting up CI environment..."
    
    # Optimize for CI/CD
    export CI_MODE=true
    export TEST_PARALLEL=true
    export TEST_QUICK_MODE=true
    
    # Reduce resource usage
    MAX_CONCURRENCY=2
    TIMEOUT="20m"
}

build_orchestrator() {
    if [[ "${BUILD_ORCHESTRATOR:-false}" == "true" ]] || [[ ! -f "$ORCHESTRATOR_BINARY" ]]; then
        log_info "Building test orchestrator..."
        
        cd "${SCRIPT_DIR}/orchestrator"
        go mod tidy
        go build -o orchestrator -ldflags="-s -w" .
        
        log_success "Orchestrator built successfully"
    fi
}

clean_artifacts() {
    if [[ "${CLEAN_ARTIFACTS:-false}" == "true" ]]; then
        log_info "Cleaning previous test artifacts..."
        
        # Remove old test results (keep last 5)
        find "${SCRIPT_DIR}/results" -maxdepth 1 -type d -name "*_*" | sort -r | tail -n +6 | xargs rm -rf
        
        # Clean Docker resources
        docker system prune -f --volumes || true
        
        log_success "Artifacts cleaned"
    fi
}

run_pre_test_validation() {
    log_info "Running pre-test validation..."
    
    # Validate configuration
    if command -v yq &> /dev/null; then
        if ! yq eval . "$DEFAULT_CONFIG" > /dev/null 2>&1; then
            log_error "Invalid YAML configuration file: $DEFAULT_CONFIG"
            return 1
        fi
    fi
    
    # Check system resources
    check_system_resources
    
    # Validate database connectivity (for local environment)
    if [[ "$ENVIRONMENT" == "local" ]]; then
        validate_database_connectivity
    fi
    
    log_success "Pre-test validation completed"
}

check_system_resources() {
    local cpu_cores
    local memory_gb
    local disk_gb
    
    cpu_cores=$(nproc)
    memory_gb=$(free -g | awk '/^Mem:/{print $2}')
    disk_gb=$(df -BG . | awk 'NR==2{print $4}' | sed 's/G//')
    
    log_debug "System resources: CPU=${cpu_cores} cores, Memory=${memory_gb}GB, Disk=${disk_gb}GB"
    
    # Check minimum requirements
    if (( cpu_cores < 2 )); then
        log_warning "Low CPU cores: $cpu_cores. Recommend 4+ cores for optimal performance"
    fi
    
    if (( memory_gb < 4 )); then
        log_warning "Low memory: ${memory_gb}GB. Recommend 8GB+ for full testing"
    fi
    
    if (( disk_gb < 10 )); then
        log_warning "Low disk space: ${disk_gb}GB. Recommend 20GB+ for test artifacts"
    fi
}

validate_database_connectivity() {
    log_debug "Validating database connectivity..."
    
    # Start test databases if not running
    docker-compose -f "${SCRIPT_DIR}/docker-compose.e2e.yml" up -d postgres mysql
    
    # Wait for databases to be ready
    local max_attempts=30
    local attempt=1
    
    while (( attempt <= max_attempts )); do
        if docker-compose -f "${SCRIPT_DIR}/docker-compose.e2e.yml" exec -T postgres pg_isready -U postgres &> /dev/null; then
            break
        fi
        
        log_debug "Waiting for PostgreSQL to be ready (attempt $attempt/$max_attempts)..."
        sleep 2
        ((attempt++))
    done
    
    if (( attempt > max_attempts )); then
        log_error "PostgreSQL failed to start within expected time"
        return 1
    fi
    
    # Similar check for MySQL
    attempt=1
    while (( attempt <= max_attempts )); do
        if docker-compose -f "${SCRIPT_DIR}/docker-compose.e2e.yml" exec -T mysql mysqladmin ping -h localhost &> /dev/null; then
            break
        fi
        
        log_debug "Waiting for MySQL to be ready (attempt $attempt/$max_attempts)..."
        sleep 2
        ((attempt++))
    done
    
    if (( attempt > max_attempts )); then
        log_error "MySQL failed to start within expected time"
        return 1
    fi
    
    log_success "Database connectivity validated"
}

execute_tests() {
    log_info "Executing unified E2E tests..."
    
    local orchestrator_args=()
    
    # Build orchestrator arguments
    orchestrator_args+=(--config "$DEFAULT_CONFIG")
    orchestrator_args+=(--env "$ENVIRONMENT")
    orchestrator_args+=(--output "$OUTPUT_DIR")
    orchestrator_args+=(--timeout "$TIMEOUT")
    orchestrator_args+=(--max-concurrency "$MAX_CONCURRENCY")
    
    if [[ "$PARALLEL_MODE" == "true" ]]; then
        orchestrator_args+=(--parallel)
    fi
    
    if [[ "$VERBOSE" == "true" ]]; then
        orchestrator_args+=(--verbose)
    fi
    
    if [[ "$DRY_RUN" == "true" ]]; then
        orchestrator_args+=(--dry-run)
    fi
    
    if [[ "$CONTINUE_ON_ERROR" == "true" ]]; then
        orchestrator_args+=(--continue-on-error)
    fi
    
    # Add suite filters
    if [[ -n "$SUITE_FILTER" ]]; then
        IFS=',' read -ra suites <<< "$SUITE_FILTER"
        for suite in "${suites[@]}"; do
            orchestrator_args+=(--suite "$suite")
        done
    fi
    
    # Execute orchestrator
    log_info "Starting test orchestrator with args: ${orchestrator_args[*]}"
    
    local start_time
    start_time=$(date +%s)
    
    local exit_code=0
    if ! "$ORCHESTRATOR_BINARY" "${orchestrator_args[@]}"; then
        exit_code=$?
        log_error "Test execution failed with exit code: $exit_code"
    fi
    
    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - start_time))
    
    log_info "Test execution completed in ${duration}s"
    
    return $exit_code
}

generate_reports() {
    log_info "Generating comprehensive test reports..."
    
    # Generate HTML dashboard
    if [[ "$REPORT_FORMATS" == *"html"* ]]; then
        generate_html_dashboard
    fi
    
    # Generate JUnit XML for CI integration
    if [[ "$REPORT_FORMATS" == *"junit"* ]]; then
        generate_junit_report
    fi
    
    # Generate executive summary
    generate_executive_summary
    
    log_success "Reports generated in: $OUTPUT_DIR"
}

generate_html_dashboard() {
    log_debug "Generating HTML dashboard..."
    
    local dashboard_file="$OUTPUT_DIR/dashboard.html"
    local result_file="$OUTPUT_DIR/execution_result.json"
    
    if [[ -f "$result_file" ]]; then
        # Use Go template or Node.js to generate dashboard
        if command -v node &> /dev/null; then
            node "${SCRIPT_DIR}/reporting/generate_dashboard.js" "$result_file" "$dashboard_file"
        else
            # Fallback to simple HTML generation
            generate_simple_html_report "$result_file" "$dashboard_file"
        fi
    fi
}

generate_simple_html_report() {
    local result_file=$1
    local output_file=$2
    
    cat > "$output_file" << 'EOF'
<!DOCTYPE html>
<html>
<head>
    <title>E2E Test Results</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .header { background: #f4f4f4; padding: 20px; border-radius: 5px; }
        .passed { color: green; }
        .failed { color: red; }
        .suite { margin: 20px 0; padding: 15px; border-left: 4px solid #ddd; }
        .suite.passed { border-left-color: green; }
        .suite.failed { border-left-color: red; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background: #f4f4f4; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Database Intelligence E2E Test Results</h1>
        <p>Generated: $(date)</p>
        <p>Environment: $ENVIRONMENT</p>
    </div>
    
    <div id="summary">
        <h2>Execution Summary</h2>
        <!-- Summary will be populated by JavaScript -->
    </div>
    
    <div id="results">
        <h2>Test Suite Results</h2>
        <!-- Results will be populated by JavaScript -->
    </div>
    
    <script>
        // Load and display results
        fetch('./execution_result.json')
            .then(response => response.json())
            .then(data => displayResults(data))
            .catch(error => console.error('Error loading results:', error));
            
        function displayResults(data) {
            // Implementation would parse and display the JSON results
            console.log('Test results loaded:', data);
        }
    </script>
</body>
</html>
EOF
}

generate_junit_report() {
    log_debug "Generating JUnit XML report..."
    
    local junit_file="$OUTPUT_DIR/junit_results.xml"
    local result_file="$OUTPUT_DIR/execution_result.json"
    
    if [[ -f "$result_file" ]] && command -v jq &> /dev/null; then
        # Generate JUnit XML from JSON results
        "${SCRIPT_DIR}/reporting/json_to_junit.sh" "$result_file" "$junit_file"
    fi
}

generate_executive_summary() {
    log_debug "Generating executive summary..."
    
    local summary_file="$OUTPUT_DIR/executive_summary.md"
    local result_file="$OUTPUT_DIR/execution_result.json"
    
    if [[ -f "$result_file" ]]; then
        cat > "$summary_file" << EOF
# E2E Test Execution Summary

**Execution ID**: $(jq -r '.execution_id // "unknown"' "$result_file")
**Environment**: $ENVIRONMENT
**Date**: $(date)
**Duration**: $(jq -r '.duration // "unknown"' "$result_file")

## Overall Status
$(if jq -e '.status == "passed"' "$result_file" > /dev/null; then echo "✅ **PASSED**"; else echo "❌ **FAILED**"; fi)

## Test Suite Summary
$(jq -r '.summary // {}' "$result_file" | jq -r 'to_entries[] | "- \(.key): \(.value)"')

## Detailed Results
See the full results in: [execution_result.json](./execution_result.json)

## Dashboard
Interactive dashboard: [dashboard.html](./dashboard.html)

---
Generated by Database Intelligence E2E Testing Framework
EOF
    fi
}

cleanup_on_exit() {
    local exit_code=$?
    
    log_info "Performing cleanup operations..."
    
    # Stop any running containers
    if [[ "$ENVIRONMENT" == "local" ]]; then
        docker-compose -f "${SCRIPT_DIR}/docker-compose.e2e.yml" down --volumes --remove-orphans || true
    fi
    
    # Cleanup Kubernetes resources
    if [[ "$ENVIRONMENT" == "kubernetes" ]] && [[ -n "${TEST_NAMESPACE:-}" ]]; then
        kubectl delete namespace "$TEST_NAMESPACE" --wait=false || true
    fi
    
    # Compress test artifacts if successful
    if [[ $exit_code -eq 0 ]] && [[ -d "$OUTPUT_DIR" ]]; then
        log_info "Compressing test artifacts..."
        tar -czf "${OUTPUT_DIR}.tar.gz" -C "$(dirname "$OUTPUT_DIR")" "$(basename "$OUTPUT_DIR")"
    fi
    
    log_info "Cleanup completed"
    exit $exit_code
}

main() {
    # Set up signal handlers
    trap cleanup_on_exit EXIT INT TERM
    
    # Parse command line arguments
    parse_arguments "$@"
    
    # Validate prerequisites
    validate_prerequisites
    
    # Setup test environment
    setup_environment
    
    # Build orchestrator if needed
    build_orchestrator
    
    # Clean artifacts if requested
    clean_artifacts
    
    # Run pre-test validation
    run_pre_test_validation
    
    # Execute tests
    local test_exit_code=0
    if ! execute_tests; then
        test_exit_code=$?
    fi
    
    # Generate reports
    generate_reports
    
    # Print final summary
    if [[ $test_exit_code -eq 0 ]]; then
        log_success "E2E testing completed successfully!"
        log_info "Results available in: $OUTPUT_DIR"
    else
        log_error "E2E testing failed with exit code: $test_exit_code"
        log_info "Check results in: $OUTPUT_DIR"
    fi
    
    return $test_exit_code
}

# Run main function
main "$@"