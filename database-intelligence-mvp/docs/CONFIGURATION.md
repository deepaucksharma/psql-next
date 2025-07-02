# Configuration Guide

Comprehensive configuration reference for the Database Intelligence Collector, covering all receivers, processors, and exporters with practical examples.

## Table of Contents

1. [Configuration Overview](#configuration-overview)
2. [Environment Variables](#environment-variables)
3. [Receivers Configuration](#receivers-configuration)
4. [Processors Configuration](#processors-configuration)
5. [Exporters Configuration](#exporters-configuration)
6. [Service Configuration](#service-configuration)
7. [Complete Examples](#complete-examples)
8. [Best Practices](#best-practices)

## Configuration Overview

The collector uses YAML configuration files with environment variable substitution support. Configuration can be provided via:

- Command line: `--config=/path/to/config.yaml`
- Multiple files: `--config=/path/to/first.yaml --config=/path/to/second.yaml`
- Environment expansion: `${ENV_VAR}` or `${ENV_VAR:-default_value}`

### Basic Structure

```yaml
receivers:
  # Data collection components

processors:
  # Data processing pipeline

exporters:
  # Data export destinations

extensions:
  # Additional capabilities

service:
  extensions: [health_check, pprof]
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlp]
```

## Environment Variables

### Required Variables

```bash
# Database Connection
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=monitoring
export POSTGRES_PASSWORD=secure_password
export POSTGRES_DATABASE=postgres

# New Relic
export NEW_RELIC_LICENSE_KEY=your-license-key
export NEW_RELIC_OTLP_ENDPOINT=otlp.nr-data.net:4317

# Environment
export ENVIRONMENT=production
```

### Optional Variables

```bash
# Performance Tuning
export COLLECTION_INTERVAL=10s
export BATCH_TIMEOUT=10s
export MEMORY_LIMIT_MIB=512

# Feature Flags
export ENABLE_QUERYLENS=true
export ENABLE_ASH=true
export ENABLE_PII_DETECTION=true

# Cost Control
export MONTHLY_BUDGET_USD=100
export PRICING_TIER=standard
```

## Receivers Configuration

### PostgreSQL Receiver

Collects native PostgreSQL metrics:

```yaml
receivers:
  postgresql:
    # Connection settings
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    transport: tcp
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    
    # Database selection
    databases:
      - ${POSTGRES_DATABASE}  # Specific database
      # - "*"                 # All databases
    
    # Collection settings
    collection_interval: ${COLLECTION_INTERVAL:-10s}
    initial_delay: 1s
    
    # SSL/TLS configuration
    tls:
      insecure: false
      insecure_skip_verify: false
      ca_file: /path/to/ca.crt
      cert_file: /path/to/client.crt
      key_file: /path/to/client.key
    
    # Metrics selection (optional)
    metrics:
      postgresql.database.size:
        enabled: true
      postgresql.connections.active:
        enabled: true
      postgresql.transactions.committed:
        enabled: true
```

### MySQL Receiver

Collects MySQL performance metrics:

```yaml
receivers:
  mysql:
    endpoint: ${MYSQL_HOST}:${MYSQL_PORT}
    username: ${MYSQL_USER}
    password: ${MYSQL_PASSWORD}
    database: ${MYSQL_DATABASE}
    
    collection_interval: 10s
    
    # Custom queries
    queries:
      - name: thread_counts
        sql: "SELECT variable_value FROM performance_schema.global_status WHERE variable_name LIKE 'Threads_%'"
        
    metrics:
      mysql.threads:
        enabled: true
```

### SQLQuery Receiver (pg_querylens)

Executes custom SQL queries for advanced metrics:

```yaml
receivers:
  sqlquery:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DATABASE} sslmode=require"
    
    queries:
      # pg_querylens integration
      - sql: |
          SELECT 
            queryid,
            plan_id,
            plan_text,
            mean_exec_time_ms,
            calls,
            rows,
            shared_blks_hit,
            shared_blks_read,
            planning_time_ms
          FROM pg_querylens.current_plans
          WHERE last_execution > NOW() - INTERVAL '5 minutes'
        
        collection_interval: 30s
        
        metrics:
          - metric_name: db.querylens.query.execution_time_mean
            value_column: mean_exec_time_ms
            value_type: double
            data_point_type: gauge
            unit: ms
          
          - metric_name: db.querylens.query.calls
            value_column: calls
            value_type: int
            data_point_type: sum
        
        resource_attributes:
          - key: db.querylens.queryid
            value_column: queryid
          - key: db.querylens.plan_id
            value_column: plan_id
      
      # Active Session History
      - sql: |
          SELECT 
            pid,
            state,
            wait_event_type,
            wait_event,
            query
          FROM pg_stat_activity
          WHERE state != 'idle'
        
        collection_interval: 1s
        
        metrics:
          - metric_name: postgresql.ash.active_sessions
            value_type: int
            value_expression: "1"
            data_point_type: gauge
```

## Processors Configuration

### Memory Limiter

Prevents OOM conditions:

```yaml
processors:
  memory_limiter:
    # Check memory usage every second
    check_interval: 1s
    
    # Hard limit - will drop data
    limit_mib: ${MEMORY_LIMIT_MIB:-512}
    
    # Soft limit - will refuse new data
    spike_limit_mib: ${MEMORY_SPIKE_LIMIT_MIB:-128}
    
    # Percentage of limit to trigger GC
    limit_percentage: 80
    spike_limit_percentage: 25
```

### Adaptive Sampler

Intelligent sampling based on rules:

```yaml
processors:
  adaptivesampler:
    # Use in-memory state only (no persistence)
    in_memory_only: true
    
    # Default sampling rate (0.0 to 1.0)
    default_sampling_rate: 0.1
    
    # Sampling rules with CEL expressions
    rules:
      - name: slow_queries
        expression: 'attributes["db.statement.duration"] > 1000'
        sample_rate: 1.0
        priority: 100
      
      - name: errors
        expression: 'attributes["db.statement.error"] != ""'
        sample_rate: 1.0
        priority: 99
      
      - name: plan_changes
        expression: 'attributes["db.plan.changed"] == true'
        sample_rate: 1.0
        priority: 98
      
      - name: high_io
        expression: 'metrics["db.io.blocks_read"] > 10000'
        sample_rate: 0.5
        priority: 50
    
    # Cache configuration
    cache_size: 10000
    cache_ttl: 5m
```

### Circuit Breaker

Protects databases from monitoring overhead:

```yaml
processors:
  circuitbreaker:
    # Global settings
    failure_threshold: 0.5
    timeout: 30s
    half_open_requests: 5
    recovery_timeout: 60s
    
    # Per-database configuration
    databases:
      - name: production_db
        failure_threshold: 0.3
        timeout: 20s
        metrics_to_track:
          - db.query.duration
          - db.connections.active
      
      - name: analytics_db
        failure_threshold: 0.7
        timeout: 60s
    
    # New Relic error detection
    error_patterns:
      - "NrIntegrationError"
      - "connection refused"
      - "timeout"
```

### Plan Attribute Extractor

Extracts intelligence from query plans:

```yaml
processors:
  planattributeextractor:
    # Safety settings
    safe_mode: true
    timeout_ms: 100
    error_mode: ignore  # ignore | propagate
    
    # pg_querylens integration
    querylens:
      enabled: ${ENABLE_QUERYLENS:-true}
      plan_history_hours: 24
      regression_detection:
        enabled: true
        time_increase: 1.5      # 50% slower
        io_increase: 2.0        # 100% more I/O
        cost_increase: 2.0      # 100% higher cost
      alert_on_regression: true
    
    # Query anonymization
    query_anonymization:
      enabled: true
      attributes_to_anonymize:
        - query_text
        - db.statement
        - db.query
      generate_fingerprint: true
      fingerprint_attribute: db.query.fingerprint
    
    # PostgreSQL rules
    postgresql_rules:
      detection_jsonpath: "0.Plan"
      extractions:
        db.query.plan.cost: "0.Plan.Total Cost"
        db.query.plan.rows: "0.Plan.Plan Rows"
        db.query.plan.operation: "0.Plan.Node Type"
```

### Verification Processor

Ensures data quality and compliance:

```yaml
processors:
  verification:
    # PII Detection
    pii_detection:
      enabled: ${ENABLE_PII_DETECTION:-true}
      action: redact  # redact | hash | drop
      
      patterns:
        # Built-in patterns
        - ssn
        - credit_card
        - email
        - phone
        - ip_address
        
        # Custom patterns
        - name: employee_id
          pattern: 'EMP[0-9]{6}'
          action: hash
    
    # Data quality checks
    quality_checks:
      enabled: true
      checks:
        - name: valid_duration
          expression: 'metrics["duration"] >= 0'
          action: drop
        
        - name: reasonable_size
          expression: 'metrics["size"] < 1000000000'
          action: flag
    
    # Cardinality control
    cardinality_limits:
      enabled: true
      default_limit: 10000
      
      limits:
        - attribute: db.statement
          limit: 5000
        - attribute: user.id
          limit: 50000
    
    # Auto-tuning
    auto_tuning:
      enabled: true
      learning_period: 24h
      adjustment_interval: 1h
```

### Cost Control Processor

Manages monitoring costs:

```yaml
processors:
  costcontrol:
    # Budget settings
    monthly_budget_usd: ${MONTHLY_BUDGET_USD:-100}
    pricing_tier: ${PRICING_TIER:-standard}  # standard | data_plus
    
    # Cardinality reduction when over budget
    metric_cardinality_limit: 100000
    trace_cardinality_limit: 50000
    log_cardinality_limit: 75000
    
    # Aggressive mode settings
    aggressive_mode:
      enabled: true
      threshold: 0.8  # Enable at 80% of budget
      
      sampling_rates:
        metrics: 0.1
        traces: 0.05
        logs: 0.01
      
      drop_attributes:
        - db.statement
        - http.user_agent
        - user.email
```

### NR Error Monitor

Proactive error detection:

```yaml
processors:
  nrerrormonitor:
    enabled: true
    
    # Error detection settings
    max_attribute_length: 4095
    max_attributes_per_event: 255
    cardinality_limit: 100000
    
    # Alert thresholds
    alert_threshold: 0.1  # 10% error rate
    cardinality_warning_threshold: 80000
    
    # Pattern detection
    error_patterns:
      - pattern: "attribute.*too long"
        severity: warning
        action: truncate
      
      - pattern: "cardinality.*exceeded"
        severity: critical
        action: alert
```

### Query Correlator

Links related queries and transactions:

```yaml
processors:
  querycorrelator:
    # Correlation windows
    session_timeout: 30m
    transaction_timeout: 5m
    
    # Correlation attributes
    correlation_attributes:
      - session.id
      - transaction.id
      - user.id
      - application.name
    
    # Relationship detection
    detect_relationships:
      enabled: true
      patterns:
        - parent_child
        - sequential
        - parallel
```

### Batch Processor

Optimizes data transmission:

```yaml
processors:
  batch:
    timeout: ${BATCH_TIMEOUT:-10s}
    send_batch_size: 1000
    send_batch_max_size: 2000
```

### Transform Processor

Adds custom attributes and transformations:

```yaml
processors:
  transform:
    error_mode: ignore
    
    metric_statements:
      - context: datapoint
        statements:
          # Add custom attributes
          - set(attributes["environment"], "${ENVIRONMENT}")
          - set(attributes["region"], "${AWS_REGION:-us-east-1}")
          
          # Calculate derived metrics
          - set(attributes["cache_hit_ratio"], 
              attributes["blocks_hit"] / (attributes["blocks_hit"] + attributes["blocks_read"]))
          
          # Flag problematic queries
          - set(attributes["needs_optimization"], 
              attributes["duration_ms"] > 1000 || attributes["blocks_read"] > 10000)
```

## Exporters Configuration

### OTLP Exporter (New Relic)

```yaml
exporters:
  otlp:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    
    compression: gzip
    
    tls:
      insecure: false
      insecure_skip_verify: false
    
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
```

### Prometheus Exporter

```yaml
exporters:
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: database_intelligence
    
    const_labels:
      environment: ${ENVIRONMENT}
      service: database-monitoring
    
    resource_to_telemetry_conversion:
      enabled: true
```

### Debug Exporter

```yaml
exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 100
```

## Service Configuration

### Extensions

```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    path: "/health"
    check_collector_pipeline:
      enabled: true
      interval: 5s
      exporter_failure_threshold: 5
  
  pprof:
    endpoint: 0.0.0.0:1777
  
  zpages:
    endpoint: 0.0.0.0:55679
  
  memory_ballast:
    size_mib: 64
```

### Service Pipelines

```yaml
service:
  extensions: [health_check, pprof, zpages, memory_ballast]
  
  pipelines:
    # Main metrics pipeline
    metrics:
      receivers: [postgresql, sqlquery]
      processors: [memory_limiter, adaptivesampler, circuitbreaker, 
                   planattributeextractor, verification, costcontrol, 
                   nrerrormonitor, transform, batch]
      exporters: [otlp, prometheus]
    
    # Debug pipeline (development only)
    metrics/debug:
      receivers: [postgresql]
      processors: [memory_limiter]
      exporters: [debug]
  
  telemetry:
    logs:
      level: ${LOG_LEVEL:-info}
      development: false
      encoding: json
      output_paths: ["stdout"]
      error_output_paths: ["stderr"]
    
    metrics:
      level: detailed
      address: 0.0.0.0:8888
```

## Complete Examples

### Minimal Configuration

```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: monitoring
    password: ${POSTGRES_PASSWORD}
    databases: [postgres]

processors:
  batch:
    timeout: 10s

exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlp]
```

### Production Configuration

See [config/collector-advanced.yaml](../config/collector-advanced.yaml) for a complete production configuration.

### Development Configuration

```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    databases: ["*"]
    collection_interval: 5s

processors:
  memory_limiter:
    limit_mib: 256
  
  adaptivesampler:
    default_sampling_rate: 1.0  # 100% for development

exporters:
  debug:
    verbosity: detailed
  
  prometheus:
    endpoint: 0.0.0.0:8889

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, adaptivesampler]
      exporters: [debug, prometheus]
  
  telemetry:
    logs:
      level: debug
```

## Best Practices

### 1. Security

- Always use environment variables for sensitive data
- Enable TLS for database connections
- Use least-privilege database users
- Rotate credentials regularly

### 2. Performance

- Start with conservative sampling rates
- Monitor collector resource usage
- Use batch processor for network efficiency
- Enable compression for exports

### 3. Reliability

- Always include memory_limiter processor
- Configure circuit breakers for production
- Set up health checks and monitoring
- Use persistent queues for critical data

### 4. Cost Management

- Enable cost control processor
- Set appropriate sampling rates
- Monitor cardinality metrics
- Use adaptive sampling rules

### 5. Debugging

- Use debug exporter during development
- Enable debug logging when troubleshooting
- Monitor internal collector metrics
- Use zpages extension for live debugging

## Configuration Validation

Validate your configuration before deployment:

```bash
# Validate configuration syntax
./database-intelligence-collector validate --config=config.yaml

# Dry run to test configuration
./database-intelligence-collector --config=config.yaml --dry-run

# Check effective configuration
./database-intelligence-collector --config=config.yaml --feature-gates=+confmap.unifyEnvVarExpansion
```

## Troubleshooting

### Common Issues

1. **Connection Refused**
   - Check database host and port
   - Verify network connectivity
   - Check firewall rules

2. **Authentication Failed**
   - Verify credentials
   - Check database user permissions
   - Ensure proper SSL mode

3. **High Memory Usage**
   - Reduce batch sizes
   - Increase sampling rates
   - Lower collection frequency

4. **Data Not Appearing**
   - Check exporter configuration
   - Verify API keys
   - Monitor error logs

### Debug Commands

```bash
# Check collector status
curl http://localhost:13133/health

# View internal metrics
curl http://localhost:8888/metrics

# Access pprof data
go tool pprof http://localhost:1777/debug/pprof/heap

# View zpages
open http://localhost:55679/debug/tracez
```