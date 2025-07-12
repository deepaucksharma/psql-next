#!/bin/bash
# Unified build script for Database Intelligence Collector
# Consolidates all build functionality into one script

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Source utilities
source "$PROJECT_ROOT/scripts/utils/common.sh"

# Build configurations
BINARY_NAME="database-intelligence-collector"
BUILDER_VERSION="v0.105.0"

# Build mode (default: production)
BUILD_MODE="${1:-production}"
SKIP_TESTS="${SKIP_TESTS:-false}"
VERBOSE="${VERBOSE:-false}"

usage() {
    cat << EOF
Usage: $0 [MODE] [OPTIONS]

Build Database Intelligence Collector

Modes:
  production        Build production-ready collector (default)
  minimal          Build minimal collector (standard components only)
  enterprise       Build enterprise collector (all features)
  all              Build all distributions
  docker           Build Docker images
  multiplatform    Build for multiple platforms

Environment Variables:
  SKIP_TESTS=true   Skip test execution
  VERBOSE=true      Enable verbose output
  DOCKER_TAG=tag    Docker image tag (default: latest)

Examples:
  $0 production                    # Build production collector
  $0 all                          # Build all distributions
  $0 docker DOCKER_TAG=v1.0.0     # Build Docker images with tag

EOF
    exit 1
}

# Ensure builder is installed
ensure_builder() {
    if ! command -v builder &> /dev/null; then
        log_info "Installing OpenTelemetry Collector Builder..."
        go install go.opentelemetry.io/collector/cmd/builder@${BUILDER_VERSION}
    fi
}

# Clean previous builds
clean_builds() {
    log_info "Cleaning previous builds..."
    rm -rf "$PROJECT_ROOT/distributions/*/bin"
    rm -f "$PROJECT_ROOT/distributions/*/${BINARY_NAME}"
    rm -f "$PROJECT_ROOT/distributions/*/otelcol-*"
}

# Build minimal distribution
build_minimal() {
    log_info "Building minimal distribution..."
    ensure_builder
    
    cd "$PROJECT_ROOT"
    builder --config=otelcol-builder-config-minimal.yaml
    
    if [[ -f "distributions/minimal/${BINARY_NAME}" ]]; then
        log_success "Minimal distribution built successfully"
        chmod +x "distributions/minimal/${BINARY_NAME}"
    else
        log_error "Minimal distribution build failed"
        exit 1
    fi
}

# Build production distribution
build_production() {
    log_info "Building production distribution..."
    ensure_builder
    
    cd "$PROJECT_ROOT"
    
    # Use the complete config for production
    if [[ -f "otelcol-builder-config-complete.yaml" ]]; then
        builder --config=otelcol-builder-config-complete.yaml
    else
        builder --config=otelcol-builder-config.yaml
    fi
    
    # Check multiple possible output locations
    local built=false
    for output in "distributions/production/${BINARY_NAME}" \
                  "distributions/production/database-intelligence" \
                  "_build/${BINARY_NAME}"; do
        if [[ -f "$output" ]]; then
            if [[ "$output" != "distributions/production/${BINARY_NAME}" ]]; then
                mv "$output" "distributions/production/${BINARY_NAME}"
            fi
            chmod +x "distributions/production/${BINARY_NAME}"
            log_success "Production distribution built successfully"
            built=true
            break
        fi
    done
    
    if [[ "$built" == "false" ]]; then
        log_error "Production distribution build failed"
        exit 1
    fi
}

# Build enterprise distribution
build_enterprise() {
    log_info "Building enterprise distribution..."
    ensure_builder
    
    cd "$PROJECT_ROOT"
    
    # Enterprise uses the same complete config but different branding
    if [[ -f "otelcol-builder-config-complete.yaml" ]]; then
        # Create temporary enterprise config
        cp otelcol-builder-config-complete.yaml otelcol-builder-config-enterprise.yaml
        sed -i 's/production/enterprise/g' otelcol-builder-config-enterprise.yaml
        
        builder --config=otelcol-builder-config-enterprise.yaml
        rm -f otelcol-builder-config-enterprise.yaml
    fi
    
    if [[ -f "distributions/enterprise/${BINARY_NAME}" ]]; then
        log_success "Enterprise distribution built successfully"
        chmod +x "distributions/enterprise/${BINARY_NAME}"
    else
        log_warning "Enterprise distribution not configured, skipping..."
    fi
}

# Build Docker images
build_docker() {
    log_info "Building Docker images..."
    
    local tag="${DOCKER_TAG:-latest}"
    
    # Build standard image
    docker build -t database-intelligence:${tag} \
        -f deployments/docker/Dockerfile.standard \
        "$PROJECT_ROOT"
    
    # Build enterprise image if available
    if [[ -f "deployments/docker/Dockerfile.enterprise" ]]; then
        docker build -t database-intelligence-enterprise:${tag} \
            -f deployments/docker/Dockerfile.enterprise \
            "$PROJECT_ROOT"
    fi
    
    log_success "Docker images built successfully"
}

# Build for multiple platforms
build_multiplatform() {
    log_info "Building for multiple platforms..."
    
    cd "$PROJECT_ROOT"
    
    # Ensure production binary exists
    if [[ ! -f "distributions/production/${BINARY_NAME}" ]]; then
        build_production
    fi
    
    # Build for each platform
    local platforms=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64")
    
    for platform in "${platforms[@]}"; do
        IFS='/' read -r os arch <<< "$platform"
        local output="distributions/production/${BINARY_NAME}-${os}-${arch}"
        
        if [[ "$os" == "windows" ]]; then
            output="${output}.exe"
        fi
        
        log_info "Building for ${os}/${arch}..."
        
        cd distributions/production
        GOOS=$os GOARCH=$arch go build -o "$(basename $output)" .
        cd "$PROJECT_ROOT"
        
        log_success "Built: $output"
    done
}

# Run tests after build
run_tests() {
    if [[ "$SKIP_TESTS" == "true" ]]; then
        log_info "Skipping tests (SKIP_TESTS=true)"
        return
    fi
    
    log_info "Running component tests..."
    
    cd "$PROJECT_ROOT"
    
    # Run component tests
    if [[ -f "scripts/test/test-components.sh" ]]; then
        ./scripts/test/test-components.sh
    else
        go test ./components/... -v
    fi
    
    log_success "Tests completed"
}

# Main execution
main() {
    case "$BUILD_MODE" in
        minimal)
            clean_builds
            build_minimal
            ;;
        production|prod)
            clean_builds
            build_production
            run_tests
            ;;
        enterprise)
            clean_builds
            build_enterprise
            run_tests
            ;;
        all)
            clean_builds
            build_minimal
            build_production
            build_enterprise
            run_tests
            ;;
        docker)
            build_docker
            ;;
        multiplatform|multi)
            build_multiplatform
            ;;
        help|--help|-h)
            usage
            ;;
        *)
            log_error "Unknown build mode: $BUILD_MODE"
            usage
            ;;
    esac
    
    log_success "Build completed successfully!"
}

# Run main
main