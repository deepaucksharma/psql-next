#!/bin/bash
# Build script for custom OpenTelemetry Collector with experimental components
#
# This script builds a custom collector binary that includes both standard
# and experimental components for the Database Intelligence MVP.

set -euo pipefail

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
log() {
    echo -e "${GREEN}[BUILD]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check for Go
    if ! command -v go &> /dev/null; then
        error "Go is not installed. Please install Go 1.21 or later."
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    log "Found Go version: $GO_VERSION"
    
    # Check for builder
    if ! command -v builder &> /dev/null; then
        warning "OpenTelemetry Collector Builder not found. Installing..."
        go install go.opentelemetry.io/collector/cmd/builder@latest
        
        # Add GOPATH/bin to PATH if not already there
        export PATH="${PATH}:$(go env GOPATH)/bin"
    fi
    
    log "Prerequisites satisfied"
}

# Prepare build environment
prepare_build() {
    log "Preparing build environment..."
    
    cd "$PROJECT_ROOT"
    
    # Create dist directory
    mkdir -p dist
    
    # Ensure all custom components have go.mod files
    for component in receivers/postgresqlquery processors/adaptivesampler processors/circuitbreaker processors/planattributeextractor processors/verification exporters/otlpexporter; do
        if [ -d "$component" ] && [ ! -f "$component/go.mod" ]; then
            warning "Creating go.mod for $component"
            (
                cd "$component"
                go mod init github.com/newrelic/database-intelligence-mvp/$component
                go mod tidy
            )
        fi
    done
}

# Build custom collector
build_collector() {
    log "Building custom collector..."
    
    cd "$PROJECT_ROOT"
    
    # Run the builder
    if [ -f "otelcol-builder.yaml" ]; then
        log "Using configuration: otelcol-builder.yaml"
        builder --config=otelcol-builder.yaml --skip-compilation=false --verbose
        
        if [ $? -eq 0 ]; then
            log "Build successful!"
            log "Binary location: $PROJECT_ROOT/dist/db-intelligence-custom"
            
            # Make binary executable
            chmod +x dist/db-intelligence-custom
            
            # Show binary info
            log "Binary info:"
            file dist/db-intelligence-custom
            ls -lh dist/db-intelligence-custom
        else
            error "Build failed!"
            exit 1
        fi
    else
        error "otelcol-builder.yaml not found!"
        exit 1
    fi
}

# Create Docker image
create_docker_image() {
    log "Creating Docker image..."
    
    cd "$PROJECT_ROOT"
    
    # Create Dockerfile for custom collector
    cat > dist/Dockerfile <<'EOF'
FROM alpine:3.19

# Install ca-certificates for TLS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN addgroup -g 10001 -S otel && \
    adduser -u 10001 -S otel -G otel

# Copy custom collector binary
COPY db-intelligence-custom /otelcol-custom

# Set ownership
RUN chown -R otel:otel /otelcol-custom

# Switch to non-root user
USER otel

# Expose ports
EXPOSE 4317 4318 8888 13133 55679

# Set entrypoint
ENTRYPOINT ["/otelcol-custom"]
CMD ["--config", "/etc/otel/config.yaml"]
EOF

    # Build Docker image
    docker build -t db-intelligence-custom:latest -f dist/Dockerfile dist/
    
    if [ $? -eq 0 ]; then
        log "Docker image created: db-intelligence-custom:latest"
    else
        error "Docker image creation failed!"
        exit 1
    fi
}

# Create test configuration
create_test_config() {
    log "Creating test configuration..."
    
    mkdir -p dist/config
    
    cat > dist/config/test-config.yaml <<'EOF'
# Minimal test configuration for custom collector
extensions:
  health_check:

receivers:
  postgresqlquery:
    connection:
      dsn: "postgres://localhost:5432/test?sslmode=disable"
    collection:
      interval: 10s

processors:
  circuitbreaker:
    failure_threshold: 3
  
  batch:
    timeout: 10s

exporters:
  logging:
    verbosity: detailed

service:
  extensions: [health_check]
  pipelines:
    logs:
      receivers: [postgresqlquery]
      processors: [circuitbreaker, batch]
      exporters: [logging]
EOF

    log "Test configuration created: dist/config/test-config.yaml"
}

# Run integration tests
run_tests() {
    log "Running integration tests..."
    
    cd "$PROJECT_ROOT"
    
    # Test that custom binary starts
    log "Testing binary startup..."
    timeout 5s ./dist/db-intelligence-custom --config=dist/config/test-config.yaml --dry-run || true
    
    if [ $? -eq 124 ]; then
        log "Binary started successfully (timeout expected for dry-run)"
    else
        warning "Binary test completed with unexpected exit code"
    fi
    
    # Run component tests if available
    for component in receivers/postgresqlquery processors/adaptivesampler processors/circuitbreaker; do
        if [ -d "$component" ] && [ -f "$component/go.mod" ]; then
            log "Testing $component..."
            (
                cd "$component"
                go test -v ./... -timeout 30s || warning "Tests failed for $component"
            )
        fi
    done
}

# Main execution
main() {
    log "Starting custom collector build process..."
    
    check_prerequisites
    prepare_build
    build_collector
    
    # Optional steps
    if [[ "${1:-}" == "--with-docker" ]]; then
        create_docker_image
    fi
    
    if [[ "${1:-}" == "--with-tests" ]] || [[ "${2:-}" == "--with-tests" ]]; then
        create_test_config
        run_tests
    fi
    
    log "Build process completed!"
    echo ""
    log "Next steps:"
    echo "  1. Test the binary: ./dist/db-intelligence-custom --config=config/collector-experimental.yaml"
    echo "  2. Build Docker image: $0 --with-docker"
    echo "  3. Run tests: $0 --with-tests"
    echo ""
    log "To use in production:"
    echo "  1. Update docker-compose.yaml to use 'db-intelligence-custom:latest' image"
    echo "  2. Mount the experimental configuration"
    echo "  3. Monitor closely during initial deployment"
}

# Run main function
main "$@"