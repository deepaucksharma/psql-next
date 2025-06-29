#!/bin/bash
# Generate OTEL collector configuration based on environment

set -euo pipefail

# Default values
PG_HOST=${PG_HOST:-localhost}
PG_PORT=${PG_PORT:-5432}
PG_USER=${PG_USER:-postgres}
PG_PASSWORD=${PG_PASSWORD:-postgres}
PG_DATABASE=${PG_DATABASE:-postgres}

MYSQL_HOST=${MYSQL_HOST:-localhost}
MYSQL_PORT=${MYSQL_PORT:-3306}
MYSQL_USER=${MYSQL_USER:-root}
MYSQL_PASSWORD=${MYSQL_PASSWORD:-mysql}
MYSQL_DATABASE=${MYSQL_DATABASE:-mysql}

NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY:-}
OTLP_ENDPOINT=${OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}

# Features
ENABLE_ADAPTIVE_SAMPLING=${ENABLE_ADAPTIVE_SAMPLING:-true}
ENABLE_CIRCUIT_BREAKER=${ENABLE_CIRCUIT_BREAKER:-true}
ENABLE_VERIFICATION=${ENABLE_VERIFICATION:-true}
ENABLE_PII_SANITIZATION=${ENABLE_PII_SANITIZATION:-true}

# Generate configuration
cat <<EOF
# Auto-generated OTEL collector configuration
# Generated at: $(date)

extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    path: "/"
    
  pprof:
    endpoint: 0.0.0.0:1777
    
  zpages:
    endpoint: 0.0.0.0:55679

receivers:
  # PostgreSQL monitoring
  postgresql:
    endpoint: ${PG_HOST}:${PG_PORT}
    username: ${PG_USER}
    password: ${PG_PASSWORD}
    databases:
      - ${PG_DATABASE}
    collection_interval: 60s
    tls:
      insecure: true
      
  # MySQL monitoring  
  mysql:
    endpoint: ${MYSQL_HOST}:${MYSQL_PORT}
    username: ${MYSQL_USER}
    password: ${MYSQL_PASSWORD}
    database: ${MYSQL_DATABASE}
    collection_interval: 60s
    
  # Query performance monitoring
  sqlquery/postgresql:
    driver: postgres
    datasource: "host=${PG_HOST} port=${PG_PORT} user=${PG_USER} password=${PG_PASSWORD} dbname=${PG_DATABASE} sslmode=disable"
    queries:
      - sql: |
          SELECT 
            queryid,
            query,
            calls,
            total_exec_time,
            mean_exec_time,
            stddev_exec_time,
            rows,
            shared_blks_hit,
            shared_blks_read,
            blk_read_time,
            blk_write_time
          FROM pg_stat_statements
          WHERE query NOT LIKE '%pg_stat_statements%'
          ORDER BY total_exec_time DESC
          LIMIT 100
        metrics:
          - metric_name: db.query.exec_time.total
            value_column: total_exec_time
          - metric_name: db.query.exec_time.mean
            value_column: mean_exec_time
          - metric_name: db.query.calls
            value_column: calls
            
  # Prometheus scraper for self-monitoring
  prometheus:
    config:
      scrape_configs:
        - job_name: 'otel-collector'
          scrape_interval: 10s
          static_configs:
            - targets: ['localhost:8888']

processors:
  # Memory limiter prevents OOMs
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 30
    
  # Batch processor for efficiency
  batch:
    timeout: 10s
    send_batch_size: 10000
    
  # Resource processor adds metadata
  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: insert
      - key: deployment.environment
        value: \${DEPLOYMENT_ENV:-production}
        action: insert
EOF

# Add PII sanitization if enabled
if [ "$ENABLE_PII_SANITIZATION" = "true" ]; then
    cat <<EOF
        
  # PII sanitization
  transform:
    metric_statements:
      - context: datapoint
        statements:
          - replace_pattern(attributes["query_text"], "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b", "[EMAIL]")
          - replace_pattern(attributes["query_text"], "\\b\\d{3}-\\d{2}-\\d{4}\\b", "[SSN]")
          - replace_pattern(attributes["query_text"], "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b", "[CARD]")
EOF
fi

# Add custom processors if enabled
if [ "$ENABLE_ADAPTIVE_SAMPLING" = "true" ]; then
    cat <<EOF
    
  # Adaptive sampling based on query performance
  database_intelligence/adaptivesampler:
    min_sampling_rate: 0.1
    max_sampling_rate: 1.0
    high_cost_threshold_ms: 1000
    deduplication_window: 300s
    state_file: /tmp/adaptive_sampler_state.json
EOF
fi

if [ "$ENABLE_CIRCUIT_BREAKER" = "true" ]; then
    cat <<EOF
    
  # Circuit breaker to protect databases
  database_intelligence/circuitbreaker:
    failure_threshold: 5
    success_threshold: 2
    timeout: 30s
    cooldown_period: 60s
    half_open_max_requests: 3
    monitor_databases:
      - postgresql
      - mysql
EOF
fi

if [ "$ENABLE_VERIFICATION" = "true" ]; then
    cat <<EOF
    
  # Verification processor for quality assurance
  database_intelligence/verification:
    health_checks:
      enabled: true
      interval: 60s
      thresholds:
        memory_percent: 80
        cpu_percent: 90
        disk_percent: 95
    metric_quality:
      enabled: true
      required_fields: ["database", "table", "operation"]
      max_cardinality: 10000
    pii_detection:
      enabled: true
      sensitivity: medium
EOF
fi

# Exporters section
cat <<EOF

exporters:
EOF

# Add New Relic exporter if license key is set
if [ -n "$NEW_RELIC_LICENSE_KEY" ]; then
    cat <<EOF
  # New Relic OTLP exporter
  otlp/newrelic:
    endpoint: ${OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    tls:
      insecure: false
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
      
EOF
fi

# Always add Prometheus exporter
cat <<EOF
  # Prometheus exporter for local metrics
  prometheus:
    endpoint: 0.0.0.0:8888
    namespace: db_intelligence
    
  # Debug exporter for troubleshooting
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 200

service:
  extensions: [health_check, pprof, zpages]
  
  pipelines:
    metrics/infrastructure:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, resource, batch]
      exporters: [$([ -n "$NEW_RELIC_LICENSE_KEY" ] && echo "otlp/newrelic, ")prometheus]
      
    metrics/queries:
      receivers: [sqlquery/postgresql]
      processors: [memory_limiter$([ "$ENABLE_PII_SANITIZATION" = "true" ] && echo ", transform")$([ "$ENABLE_ADAPTIVE_SAMPLING" = "true" ] && echo ", database_intelligence/adaptivesampler")$([ "$ENABLE_CIRCUIT_BREAKER" = "true" ] && echo ", database_intelligence/circuitbreaker")$([ "$ENABLE_VERIFICATION" = "true" ] && echo ", database_intelligence/verification"), batch]
      exporters: [$([ -n "$NEW_RELIC_LICENSE_KEY" ] && echo "otlp/newrelic, ")prometheus]
      
    metrics/internal:
      receivers: [prometheus]
      processors: [memory_limiter, batch]
      exporters: [prometheus]
      
  telemetry:
    logs:
      level: info
      output_paths: ["stdout"]
      error_output_paths: ["stderr"]
    metrics:
      level: detailed
      address: 0.0.0.0:8889
EOF