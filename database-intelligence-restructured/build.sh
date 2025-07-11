#!/bin/bash

# Unified build script for database-intelligence
# Consolidates all build functionality with options

set -e

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/scripts/lib/common.sh"

# Build modes
MODE=${1:-"all"}
VERBOSE=${VERBOSE:-false}

usage() {
    cat << EOF
Usage: $0 [MODE] [OPTIONS]

Modes:
  all               Build all distributions (default)
  minimal           Build minimal distribution only
  production        Build production distribution only
  enterprise        Build enterprise distribution only
  e2e               Build E2E test collector with OCB
  test              Build and run component tests
  
Options:
  VERBOSE=true      Enable verbose output
  SKIP_TESTS=true   Skip testing phase

Examples:
  $0                          # Build all distributions
  $0 production               # Build production only
  $0 test                     # Build and test all components
  $0 e2e                      # Build E2E collector with OCB
  VERBOSE=true $0 all         # Verbose build of all distributions
EOF
}

# Build functions
build_distribution() {
    local dist_name="$1"
    local dist_path="distributions/$dist_name"
    
    if [ ! -d "$dist_path" ]; then
        log_error "Distribution directory not found: $dist_path"
        return 1
    fi
    
    log_info "Building $dist_name distribution..."
    
    cd "$dist_path"
    
    # Clean previous build
    rm -f otelcol-$dist_name database-intelligence
    
    # Build
    if [ "$VERBOSE" = "true" ]; then
        go build -v -o otelcol-$dist_name .
    else
        go build -o otelcol-$dist_name .
    fi
    
    if [ -f "otelcol-$dist_name" ]; then
        log_success "Built $dist_name distribution successfully"
        # Create symlink for compatibility
        ln -sf otelcol-$dist_name database-intelligence
    else
        log_error "Failed to build $dist_name distribution"
        return 1
    fi
    
    cd "$SCRIPT_DIR"
}

build_all_distributions() {
    log_info "Building all distributions..."
    
    local failed=0
    
    for dist in minimal production enterprise; do
        if build_distribution "$dist"; then
            log_success "$dist built successfully"
        else
            log_error "$dist build failed"
            failed=$((failed + 1))
        fi
    done
    
    if [ $failed -gt 0 ]; then
        log_error "$failed distribution(s) failed to build"
        return 1
    fi
    
    log_success "All distributions built successfully"
}

build_e2e_collector() {
    log_info "Building E2E test collector with OpenTelemetry Builder..."
    
    # Install OCB if not present
    if ! command -v ocb &> /dev/null; then
        log_info "Installing OpenTelemetry Collector Builder..."
        go install go.opentelemetry.io/collector/cmd/builder@v0.92.0
        export PATH=$PATH:$(go env GOPATH)/bin
    fi
    
    # Create builder config if not exists
    if [ ! -f "tools/builder/otelcol-builder.yaml" ]; then
        log_error "Builder configuration not found: tools/builder/otelcol-builder.yaml"
        return 1
    fi
    
    # Build with OCB
    log_info "Building collector with OCB..."
    if [ "$VERBOSE" = "true" ]; then
        ocb --config=tools/builder/otelcol-builder.yaml --verbose
    else
        ocb --config=tools/builder/otelcol-builder.yaml
    fi
    
    if [ -f "tests/e2e/e2e-test-collector" ]; then
        log_success "E2E test collector built successfully"
    else
        log_error "Failed to build E2E test collector"
        return 1
    fi
}

test_components() {
    log_info "Testing all components..."
    
    sync_go_workspace
    
    local test_results=()
    local failed=0
    
    # Test processors
    log_info "Testing processors..."
    for processor in adaptivesampler circuitbreaker costcontrol nrerrormonitor planattributeextractor querycorrelator verification; do
        if [ -d "processors/$processor" ]; then
            log_info "Testing $processor processor..."
            if (cd processors/$processor && go test ./...); then
                test_results+=("✅ $processor processor")
            else
                test_results+=("❌ $processor processor")
                failed=$((failed + 1))
            fi
        fi
    done
    
    # Test receivers
    log_info "Testing receivers..."
    for receiver in ash enhancedsql kernelmetrics; do
        if [ -d "receivers/$receiver" ]; then
            log_info "Testing $receiver receiver..."
            if (cd receivers/$receiver && go test ./...); then
                test_results+=("✅ $receiver receiver")
            else
                test_results+=("❌ $receiver receiver")
                failed=$((failed + 1))
            fi
        fi
    done
    
    # Print test summary
    echo
    log_info "Test Summary:"
    printf '%s\n' "${test_results[@]}"
    echo
    
    if [ $failed -gt 0 ]; then
        log_error "$failed component(s) failed tests"
        return 1
    else
        log_success "All components passed tests"
    fi
}

# Main execution
main() {
    case "$MODE" in
        all)
            build_all_distributions
            if [ "$SKIP_TESTS" != "true" ]; then
                test_components
            fi
            ;;
        minimal|production|enterprise)
            build_distribution "$MODE"
            ;;
        e2e)
            build_e2e_collector
            ;;
        test)
            # Build production first for testing
            build_distribution "production"
            test_components
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