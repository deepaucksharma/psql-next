#!/bin/bash
# Build script for E2E test collector with all custom processors
set -euo pipefail

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
        error "Go is not installed. Please install Go 1.23 or later."
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    log "Found Go version: $GO_VERSION"
    
    # Check for builder
    if ! command -v builder &> /dev/null; then
        warning "OpenTelemetry Collector Builder not found. Installing..."
        go install go.opentelemetry.io/collector/cmd/builder@v0.92.0
        
        # Add GOPATH/bin to PATH if not already there
        export PATH="${PATH}:$(go env GOPATH)/bin"
    fi
    
    log "Prerequisites satisfied"
}

# Prepare custom processors
prepare_processors() {
    log "Preparing custom processors..."
    
    # Navigate to processors directory
    cd ../../processors
    
    # List of all custom processors
    PROCESSORS=(
        "adaptivesampler"
        "circuitbreaker"
        "planattributeextractor"
        "querycorrelator"
        "verification"
        "costcontrol"
        "nrerrormonitor"
    )
    
    # Initialize each processor if needed
    for proc in "${PROCESSORS[@]}"; do
        if [ -d "$proc" ]; then
            log "Preparing $proc processor..."
            cd "$proc"
            
            # Update go.mod with consistent module name
            if [ -f "go.mod" ]; then
                # Check if module name needs updating
                if ! grep -q "module github.com/database-intelligence/processors/$proc" go.mod; then
                    warning "Updating module name for $proc"
                    rm -f go.mod go.sum
                    go mod init github.com/database-intelligence/processors/$proc
                fi
            else
                go mod init github.com/database-intelligence/processors/$proc
            fi
            
            # Tidy up dependencies
            go mod tidy || warning "go mod tidy failed for $proc"
            
            cd ..
        else
            error "Processor directory not found: $proc"
        fi
    done
    
    # Return to e2e directory
    cd ../tests/e2e
}

# Build collector
build_collector() {
    log "Building custom collector with all processors..."
    
    # Create dist directory
    mkdir -p dist
    
    # Run the builder
    builder --config=otelcol-builder-all-processors.yaml --skip-compilation=false --verbose
    
    if [ $? -eq 0 ]; then
        log "Build successful!"
        
        # Make binary executable
        chmod +x dist/db-intelligence-e2e-collector
        
        # Show binary info
        log "Binary info:"
        file dist/db-intelligence-e2e-collector
        ls -lh dist/db-intelligence-e2e-collector
    else
        error "Build failed!"
        exit 1
    fi
}

# Create test configuration
create_test_config() {
    log "Creating test configuration with all processors..."
    
    cat > all-processors-test-config.yaml <<'EOF'
# E2E test configuration with all custom processors
receivers:
  postgresql:
    endpoint: localhost:5432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 10s
    tls:
      insecure: true

processors:
  # Memory limiter - first in pipeline
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
  
  # Custom processors
  adaptivesampler:
    sampling_percentage: 100
    min_sampling_percentage: 10
    max_sampling_percentage: 100
    adjustment_period: 30s
  
  circuitbreaker:
    failure_threshold: 5
    recovery_timeout: 30s
    half_open_max_requests: 3
  
  planattributeextractor:
    extract_plans: true
    anonymize_queries: false
    max_plan_size_bytes: 10240
  
  querycorrelator:
    correlation_window: 5m
    max_queries_tracked: 1000
  
  verification:
    verify_metrics: true
    expected_metrics:
      - postgresql.backends
      - postgresql.database.size
      - postgresql.table.size
  
  costcontrol:
    max_metrics_per_minute: 10000
    enforcement_mode: log
  
  nrerrormonitor:
    report_interval: 1m
    max_errors_tracked: 100
  
  # Standard processors
  attributes:
    actions:
      - key: test.run.id
        value: ${TEST_RUN_ID}
        action: insert
      - key: test.type
        value: all_processors
        action: insert
  
  batch:
    timeout: 10s
    send_batch_size: 100

exporters:
  otlp:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 30s
  
  logging:
    verbosity: detailed
    sampling_initial: 10
    sampling_thereafter: 100

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [
        memory_limiter,
        adaptivesampler,
        circuitbreaker,
        planattributeextractor,
        querycorrelator,
        verification,
        costcontrol,
        nrerrormonitor,
        attributes,
        batch
      ]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: info
      output_paths: ["stdout", "collector.log"]
    metrics:
      level: detailed
      address: 0.0.0.0:8888
EOF

    log "Test configuration created: all-processors-test-config.yaml"
}

# Main execution
main() {
    log "Starting E2E test collector build process..."
    
    check_prerequisites
    prepare_processors
    build_collector
    create_test_config
    
    log "Build process completed!"
    echo ""
    log "Next steps:"
    echo "  1. Test the binary: ./dist/db-intelligence-e2e-collector --config=all-processors-test-config.yaml"
    echo "  2. Run processor tests: go test -v ./all_processors_test.go"
    echo ""
}

# Run main function
main "$@"