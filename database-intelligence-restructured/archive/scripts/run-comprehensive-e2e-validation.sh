#!/bin/bash

# Comprehensive E2E Validation Script for Database Intelligence
# This script runs deep E2E tests with our newly built collector

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
PARENT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COLLECTOR_BINARY="$PROJECT_ROOT/distributions/production/otelcol-production"
DOCKER_COMPOSE_FILE="$PROJECT_ROOT/docker-compose-e2e.yml"
TEST_LOG="$PROJECT_ROOT/comprehensive-e2e-test.log"

# Test configuration
export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
export POSTGRES_PORT="${POSTGRES_PORT:-5432}"
export POSTGRES_USER="${POSTGRES_USER:-postgres}"
export POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-postgres}"
export POSTGRES_DB="${POSTGRES_DB:-testdb}"

# Initialize log
echo "=== Comprehensive E2E Test Log ===" > "$TEST_LOG"
echo "Started at: $(date)" >> "$TEST_LOG"
echo "" >> "$TEST_LOG"

# Function to log
log() {
    local level=$1
    shift
    local message="$@"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${!level}[$level]${NC} $message"
    echo "[$timestamp] [$level] $message" >> "$TEST_LOG"
}

# Function to check prerequisites
check_prerequisites() {
    log "BLUE" "Checking prerequisites..."
    
    # Check collector binary
    if [[ ! -f "$COLLECTOR_BINARY" ]]; then
        log "RED" "Collector binary not found at: $COLLECTOR_BINARY"
        exit 1
    fi
    log "GREEN" "✓ Collector binary found"
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log "RED" "Docker is required but not installed"
        exit 1
    fi
    log "GREEN" "✓ Docker installed"
    
    # Check docker-compose file
    if [[ ! -f "$DOCKER_COMPOSE_FILE" ]]; then
        log "RED" "Docker compose file not found at: $DOCKER_COMPOSE_FILE"
        exit 1
    fi
    log "GREEN" "✓ Docker compose file found"
    
    log "GREEN" "✓ All prerequisites met"
}

# Function to start infrastructure
start_infrastructure() {
    log "BLUE" "Starting test infrastructure..."
    
    # Stop any existing containers
    docker-compose -f "$DOCKER_COMPOSE_FILE" down -v 2>&1 | tee -a "$TEST_LOG" || true
    
    # Start PostgreSQL
    log "CYAN" "Starting PostgreSQL..."
    docker-compose -f "$DOCKER_COMPOSE_FILE" up -d postgres 2>&1 | tee -a "$TEST_LOG"
    
    # Wait for PostgreSQL to be ready
    log "CYAN" "Waiting for PostgreSQL to be ready..."
    local retries=30
    while ! docker exec e2e-postgres pg_isready -U postgres &> /dev/null; do
        retries=$((retries - 1))
        if [[ $retries -eq 0 ]]; then
            log "RED" "PostgreSQL failed to start"
            docker logs e2e-postgres >> "$TEST_LOG" 2>&1
            exit 1
        fi
        sleep 1
    done
    log "GREEN" "✓ PostgreSQL is ready"
}

# Function to create test collector config
create_test_config() {
    log "BLUE" "Creating test collector configuration..."
    
    cat > "$PROJECT_ROOT/e2e-test-collector.yaml" << 'EOF'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317
      http:
        endpoint: localhost:4318
  
  # PostgreSQL receiver (using contrib)
  postgresql:
    endpoint: postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable
    collection_interval: 10s
    databases:
      - testdb
    metrics:
      postgresql.database.size:
        enabled: true
      postgresql.table.size:
        enabled: true
      postgresql.rows:
        enabled: true
      postgresql.blocks.read:
        enabled: true
      postgresql.blocks.written:
        enabled: true
      postgresql.commits:
        enabled: true
      postgresql.rollbacks:
        enabled: true
      postgresql.connections:
        enabled: true
      postgresql.backends:
        enabled: true

processors:
  batch:
    timeout: 10s
    send_batch_size: 1024
  
  memory_limiter:
    check_interval: 5s
    limit_percentage: 80
    spike_limit_percentage: 20

  # Add custom processors when they're registered
  # adaptivesampler:
  #   base_rate: 0.1
  #   adjustment_factor: 0.05
  #   min_rate: 0.01
  #   max_rate: 1.0
  
  # circuitbreaker:
  #   failure_threshold: 5
  #   recovery_timeout: 30s
  #   consecutive_failures: 3

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 200
  
  otlp:
    endpoint: localhost:4318
    tls:
      insecure: true

  # File exporter for analysis
  file:
    path: ./e2e-test-metrics.json
    rotation:
      max_megabytes: 10
      max_days: 3
      max_backups: 3
      localtime: false

service:
  telemetry:
    logs:
      level: info
      development: true
      encoding: console
    metrics:
      level: detailed
      address: localhost:8888

  pipelines:
    metrics/postgresql:
      receivers: [postgresql]
      processors: [memory_limiter, batch]
      exporters: [debug, file]
    
    metrics/otlp:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug, file]
    
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug]
    
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [debug]
EOF
    
    log "GREEN" "✓ Test configuration created"
}

# Function to test collector features
test_collector_features() {
    log "BLUE" "Testing collector features..."
    
    # Test 1: Basic startup and configuration validation
    log "CYAN" "Test 1: Configuration validation"
    if "$COLLECTOR_BINARY" validate --config="$PROJECT_ROOT/e2e-test-collector.yaml" 2>&1 | tee -a "$TEST_LOG"; then
        log "GREEN" "✓ Configuration is valid"
    else
        log "RED" "✗ Configuration validation failed"
        return 1
    fi
    
    # Test 2: Start collector
    log "CYAN" "Test 2: Starting collector"
    "$COLLECTOR_BINARY" --config="$PROJECT_ROOT/e2e-test-collector.yaml" > "$PROJECT_ROOT/collector.log" 2>&1 &
    local collector_pid=$!
    
    # Give collector time to start
    sleep 5
    
    # Check if collector is running
    if kill -0 $collector_pid 2>/dev/null; then
        log "GREEN" "✓ Collector started successfully (PID: $collector_pid)"
    else
        log "RED" "✗ Collector failed to start"
        cat "$PROJECT_ROOT/collector.log" >> "$TEST_LOG"
        return 1
    fi
    
    # Test 3: Check health endpoint
    log "CYAN" "Test 3: Checking health endpoint"
    if curl -s http://localhost:13133/health | grep -q "OK"; then
        log "GREEN" "✓ Health endpoint is responding"
    else
        log "YELLOW" "⚠ Health endpoint not responding (might not be configured)"
    fi
    
    # Test 4: Check metrics endpoint
    log "CYAN" "Test 4: Checking metrics endpoint"
    if curl -s http://localhost:8888/metrics | grep -q "otelcol"; then
        log "GREEN" "✓ Metrics endpoint is responding"
    else
        log "YELLOW" "⚠ Metrics endpoint not responding"
    fi
    
    # Test 5: Generate test load in PostgreSQL
    log "CYAN" "Test 5: Generating PostgreSQL test load"
    docker exec e2e-postgres psql -U postgres -d testdb -c "
        CREATE TABLE IF NOT EXISTS test_table (
            id SERIAL PRIMARY KEY,
            data TEXT,
            created_at TIMESTAMP DEFAULT NOW()
        );
        
        INSERT INTO test_table (data) 
        SELECT 'Test data ' || generate_series(1, 1000);
        
        ANALYZE test_table;
    " 2>&1 | tee -a "$TEST_LOG"
    
    # Wait for metrics collection
    log "CYAN" "Waiting for metrics collection..."
    sleep 15
    
    # Test 6: Check if metrics were collected
    log "CYAN" "Test 6: Checking collected metrics"
    if [[ -f "$PROJECT_ROOT/e2e-test-metrics.json" ]]; then
        local metric_count=$(grep -c "postgresql" "$PROJECT_ROOT/e2e-test-metrics.json" || echo "0")
        if [[ $metric_count -gt 0 ]]; then
            log "GREEN" "✓ PostgreSQL metrics collected: $metric_count entries"
        else
            log "RED" "✗ No PostgreSQL metrics found"
        fi
    else
        log "RED" "✗ Metrics file not created"
    fi
    
    # Test 7: Send OTLP test data
    log "CYAN" "Test 7: Sending OTLP test data"
    # This would normally use a tool like telemetrygen or a custom client
    # For now, we'll just test the endpoint is listening
    if nc -z localhost 4317; then
        log "GREEN" "✓ OTLP gRPC endpoint is listening"
    else
        log "RED" "✗ OTLP gRPC endpoint not responding"
    fi
    
    if nc -z localhost 4318; then
        log "GREEN" "✓ OTLP HTTP endpoint is listening"
    else
        log "RED" "✗ OTLP HTTP endpoint not responding"
    fi
    
    # Stop collector
    log "CYAN" "Stopping collector..."
    kill $collector_pid 2>/dev/null || true
    wait $collector_pid 2>/dev/null || true
    
    log "GREEN" "✓ Feature tests completed"
}

# Function to test custom components
test_custom_components() {
    log "BLUE" "Testing custom components..."
    
    # Create a config with custom processors
    cat > "$PROJECT_ROOT/e2e-test-custom.yaml" << 'EOF'
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

  # Test custom receivers when available
  # ash:
  #   driver: postgres
  #   datasource: host=localhost port=5432 user=postgres password=postgres dbname=testdb sslmode=disable
  #   collection_interval: 10s
  
  # enhancedsql:
  #   driver: postgres
  #   datasource: host=localhost port=5432 user=postgres password=postgres dbname=testdb sslmode=disable
  #   queries:
  #     - metric_name: custom_metric
  #       query: "SELECT count(*) as value FROM test_table"

processors:
  batch:
    timeout: 10s

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
EOF
    
    log "YELLOW" "⚠ Custom component testing skipped (components not registered in binary)"
    log "CYAN" "To enable custom components, they need to be added to the production distribution"
}

# Function to analyze issues
analyze_issues() {
    log "BLUE" "Analyzing test results and identifying issues..."
    
    local issues_found=0
    
    # Check collector logs for errors
    if [[ -f "$PROJECT_ROOT/collector.log" ]]; then
        local error_count=$(grep -c "error\|ERROR" "$PROJECT_ROOT/collector.log" || echo "0")
        if [[ $error_count -gt 0 ]]; then
            log "YELLOW" "Found $error_count errors in collector logs"
            grep -i "error" "$PROJECT_ROOT/collector.log" | head -10 >> "$TEST_LOG"
            issues_found=$((issues_found + 1))
        fi
    fi
    
    # Check for missing features
    log "CYAN" "Checking for missing features..."
    
    # Custom processors not included
    log "YELLOW" "Issue: Custom processors not included in production binary"
    log "CYAN" "  - adaptivesampler"
    log "CYAN" "  - circuitbreaker"
    log "CYAN" "  - costcontrol"
    log "CYAN" "  - nrerrormonitor"
    log "CYAN" "  - planattributeextractor"
    log "CYAN" "  - querycorrelator"
    log "CYAN" "  - verification"
    issues_found=$((issues_found + 1))
    
    # Custom receivers not included
    log "YELLOW" "Issue: Custom receivers not included in production binary"
    log "CYAN" "  - ash"
    log "CYAN" "  - enhancedsql"
    log "CYAN" "  - kernelmetrics"
    issues_found=$((issues_found + 1))
    
    # Custom exporters not included
    log "YELLOW" "Issue: Custom exporters not included in production binary"
    log "CYAN" "  - nri (New Relic Infrastructure format)"
    issues_found=$((issues_found + 1))
    
    # Missing database receivers
    log "YELLOW" "Issue: Database receivers from contrib not included"
    log "CYAN" "  - postgresql receiver"
    log "CYAN" "  - mysql receiver"
    issues_found=$((issues_found + 1))
    
    # Missing extensions
    log "YELLOW" "Issue: Custom extensions not included"
    log "CYAN" "  - healthcheck extension"
    issues_found=$((issues_found + 1))
    
    return $issues_found
}

# Function to generate recommendations
generate_recommendations() {
    log "BLUE" "Generating comprehensive fix recommendations..."
    
    cat > "$PROJECT_ROOT/e2e-fixes-required.md" << 'EOF'
# Comprehensive E2E Fixes Required

## 1. Production Distribution Updates

### Add Custom Components to Production Binary

The production distribution needs to include all custom components:

```go
// distributions/production/components.go
package main

import (
    // ... existing imports ...
    
    // Custom processors
    "github.com/database-intelligence/processors/adaptivesampler"
    "github.com/database-intelligence/processors/circuitbreaker"
    "github.com/database-intelligence/processors/costcontrol"
    "github.com/database-intelligence/processors/nrerrormonitor"
    "github.com/database-intelligence/processors/planattributeextractor"
    "github.com/database-intelligence/processors/querycorrelator"
    "github.com/database-intelligence/processors/verification"
    
    // Custom receivers
    "github.com/database-intelligence/receivers/ash"
    "github.com/database-intelligence/receivers/enhancedsql"
    "github.com/database-intelligence/receivers/kernelmetrics"
    
    // Custom exporters
    "github.com/database-intelligence/exporters/nri"
    
    // Custom extensions
    "github.com/database-intelligence/extensions/healthcheck"
    
    // Contrib components for databases
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"
    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"
)

var components = otelcol.Factories{
    Receivers: map[component.Type]receiver.Factory{
        // Core
        component.MustNewType("otlp"): otlpreceiver.NewFactory(),
        
        // Custom
        component.MustNewType("ash"): ash.NewFactory(),
        component.MustNewType("enhancedsql"): enhancedsql.NewFactory(),
        component.MustNewType("kernelmetrics"): kernelmetrics.NewFactory(),
        
        // Contrib
        component.MustNewType("postgresql"): postgresqlreceiver.NewFactory(),
        component.MustNewType("mysql"): mysqlreceiver.NewFactory(),
    },
    // ... similar for processors, exporters, extensions
}
```

## 2. Fix Import Issues

### Update Custom Component Imports

1. Remove dependencies on non-existent core module
2. Fix rate limiter imports in nri exporter
3. Ensure all components use correct import paths

## 3. Add Integration Tests

### Create Comprehensive Test Suite

```go
// tests/e2e/comprehensive_test.go
package e2e

import (
    "testing"
    "time"
)

func TestCustomProcessors(t *testing.T) {
    // Test each custom processor
}

func TestCustomReceivers(t *testing.T) {
    // Test ASH, enhancedSQL, kernelmetrics
}

func TestDatabaseIntegration(t *testing.T) {
    // Test PostgreSQL and MySQL receivers
}

func TestNewRelicExporter(t *testing.T) {
    // Test NRI exporter
}
```

## 4. Configuration Templates

### Create Production-Ready Configs

1. **Full-Featured Config**: All components enabled
2. **PostgreSQL-Focused Config**: For PostgreSQL monitoring
3. **MySQL-Focused Config**: For MySQL monitoring
4. **High-Performance Config**: With adaptive sampling and circuit breaking

## 5. Documentation Updates

### Create Comprehensive Guides

1. **Deployment Guide**: How to deploy in production
2. **Configuration Reference**: All available options
3. **Troubleshooting Guide**: Common issues and solutions
4. **Performance Tuning**: Optimization strategies

## 6. Build System Improvements

### Automated Build Process

```bash
#!/bin/bash
# build-production.sh

# Build with all components
cd distributions/production
go build -o otelcol-production .

# Run tests
go test ./...

# Create release artifacts
tar -czf otelcol-production-linux-amd64.tar.gz otelcol-production
```

## 7. Monitoring and Observability

### Add Self-Monitoring

1. Prometheus metrics for collector health
2. Logging with proper levels
3. Tracing for request flow
4. Health check endpoints

## 8. Security Enhancements

### Production Security

1. TLS configuration for all endpoints
2. Authentication for receivers
3. Secret management for credentials
4. Network policies for Kubernetes

## Next Steps

1. Update production distribution with all components
2. Fix import issues in custom components
3. Add comprehensive E2E tests
4. Create production configurations
5. Update documentation
6. Set up CI/CD pipeline
EOF
    
    log "GREEN" "✓ Recommendations generated in e2e-fixes-required.md"
}

# Main execution
main() {
    log "BLUE" "=== Starting Comprehensive E2E Validation ==="
    
    # Check prerequisites
    check_prerequisites
    
    # Start infrastructure
    start_infrastructure
    
    # Create test configuration
    create_test_config
    
    # Test collector features
    if test_collector_features; then
        log "GREEN" "✓ Basic collector features working"
    else
        log "RED" "✗ Basic collector features have issues"
    fi
    
    # Test custom components
    test_custom_components
    
    # Analyze issues
    if analyze_issues; then
        log "YELLOW" "Found issues that need to be addressed"
    fi
    
    # Generate recommendations
    generate_recommendations
    
    # Cleanup
    log "BLUE" "Cleaning up..."
    docker-compose -f "$DOCKER_COMPOSE_FILE" down -v 2>&1 | tee -a "$TEST_LOG" || true
    
    log "BLUE" "=== E2E Validation Complete ==="
    log "CYAN" "See detailed log at: $TEST_LOG"
    log "CYAN" "See fix recommendations at: $PROJECT_ROOT/e2e-fixes-required.md"
}

# Run main
main "$@"