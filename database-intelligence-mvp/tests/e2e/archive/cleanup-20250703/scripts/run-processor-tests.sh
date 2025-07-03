#!/bin/bash
# Test custom processors with the database intelligence collector

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Source common functions
source "${SCRIPT_DIR}/../../scripts/lib/common.sh"

log_info "Starting processor tests..."

# Check if custom collector binary exists
COLLECTOR_BINARY="${SCRIPT_DIR}/../../database-intelligence-collector"
if [[ ! -f "$COLLECTOR_BINARY" ]]; then
    log_warning "Custom collector binary not found, building it..."
    cd ../..
    go build -o database-intelligence-collector .
    cd "$SCRIPT_DIR"
fi

# Create a test configuration with all custom processors
cat > processor-test-config.yaml << 'EOF'
extensions:
  health_check:
    endpoint: 0.0.0.0:13133

receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DB}
    collection_interval: 10s
    tls:
      insecure: true

  mysql:
    endpoint: ${env:MYSQL_HOST}:${env:MYSQL_PORT}
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    database: ${env:MYSQL_DB}
    collection_interval: 10s

processors:
  # Test verification processor
  verification:
    enabled: true
    data_completeness:
      enabled: true
      required_attributes:
        - "db.name"
        - "db.system"
    semantic_correctness:
      enabled: true
    performance_tracking:
      enabled: true
    error_mode: "ignore"

  # Test adaptive sampler
  adaptivesampler:
    max_sampling_rate: 100
    min_sampling_rate: 1
    target_metrics_per_minute: 1000
    adjustment_period: 30s

  # Test circuit breaker
  circuitbreaker:
    enabled: true
    failure_threshold: 5
    recovery_timeout: 30s
    half_open_requests: 3

  # Test query correlator
  querycorrelator:
    retention_period: 1h
    cleanup_interval: 10m
    enable_table_correlation: true
    enable_database_correlation: true

  # Test cost control
  costcontrol:
    enabled: true
    max_metrics_per_minute: 10000
    max_metric_cardinality: 1000
    enforcement_mode: "log"

  # Test NR error monitor
  nrerrormonitor:
    enabled: true
    batch_timeout: 10s
    batch_size: 100
    alert_on_error: true

  # Test plan attribute extractor
  planattributeextractor:
    safe_mode: true
    postgresql_rules:
      extractions:
        "db.query.plan.rows": "Plan.Plan Rows"
        "db.query.plan.cost": "Plan.Total Cost"
    mysql_rules:
      extractions:
        "db.query.rows_examined": "rows_examined"

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 100

  prometheus:
    endpoint: "0.0.0.0:8890"
    resource_to_telemetry_conversion:
      enabled: true

service:
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [
        verification,
        adaptivesampler,
        circuitbreaker,
        querycorrelator,
        costcontrol,
        nrerrormonitor,
        planattributeextractor
      ]
      exporters: [debug, prometheus]
  telemetry:
    logs:
      level: debug
EOF

# Start the custom collector
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres
export POSTGRES_DB=testdb
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=root
export MYSQL_DB=testdb

log_info "Starting custom collector with all processors..."
"$COLLECTOR_BINARY" --config=processor-test-config.yaml &
COLLECTOR_PID=$!

# Wait for collector to start
sleep 10

# Check if collector is running
if ! kill -0 $COLLECTOR_PID 2>/dev/null; then
    log_error "Collector failed to start"
    exit 1
fi

log_info "Collector started successfully with PID $COLLECTOR_PID"

# Wait for metrics
sleep 30

# Check metrics endpoint
log_info "Checking processor metrics..."
METRICS=$(curl -s http://localhost:8890/metrics || echo "Failed to get metrics")

# Check for processor-specific metrics
PROCESSORS=(
    "verification_processor"
    "adaptive_sampler"
    "circuit_breaker"
    "query_correlator"
    "cost_control"
    "nr_error_monitor"
    "plan_attribute_extractor"
)

for processor in "${PROCESSORS[@]}"; do
    if echo "$METRICS" | grep -q "$processor"; then
        log_success "Found metrics for $processor"
    else
        log_warning "No metrics found for $processor"
    fi
done

# Check collector logs for processor activity
log_info "Checking collector logs..."
if [[ -f collector.log ]]; then
    for processor in "${PROCESSORS[@]}"; do
        if grep -q "$processor" collector.log; then
            log_success "Found log entries for $processor"
        else
            log_warning "No log entries found for $processor"
        fi
    done
fi

# Cleanup
log_info "Stopping collector..."
kill $COLLECTOR_PID || true
wait $COLLECTOR_PID 2>/dev/null || true

log_success "Processor tests completed!"