# Configuration Reference

This document provides a comprehensive reference for configuring the Database Intelligence OTEL Collector.

## Table of Contents
- [Environment Variables](#environment-variables)
- [Receivers Configuration](#receivers-configuration)
- [Processors Configuration](#processors-configuration)
- [Exporters Configuration](#exporters-configuration)
- [Service Configuration](#service-configuration)
- [Complete Examples](#complete-examples)

## Environment Variables

The collector uses environment variables for sensitive data and deployment-specific settings:

```bash
# Database Credentials
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=monitor_user
POSTGRES_PASSWORD=secure_password
POSTGRES_DATABASE=production

MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=monitor_user
MYSQL_PASSWORD=secure_password
MYSQL_DATABASE=production

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key_here
OTLP_ENDPOINT=https://otlp.nr-data.net:4317

# Feature Flags
ENABLE_ADAPTIVE_SAMPLING=true
ENABLE_CIRCUIT_BREAKER=true
ENABLE_VERIFICATION=true
ENABLE_PII_SANITIZATION=true

# Resource Settings
MEMORY_LIMIT_PERCENTAGE=80
BATCH_SIZE=10000
COLLECTION_INTERVAL=60s
```

## Receivers Configuration

### PostgreSQL Receiver (Standard OTEL)

```yaml
receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DATABASE}
    collection_interval: 60s
    tls:
      insecure: true
      insecure_skip_verify: false
      ca_file: /path/to/ca.crt
      cert_file: /path/to/client.crt
      key_file: /path/to/client.key
    initial_delay: 10s
    resource_attributes:
      db.system: postgresql
```

### MySQL Receiver (Standard OTEL)

```yaml
receivers:
  mysql:
    endpoint: ${env:MYSQL_HOST}:${env:MYSQL_PORT}
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    database: ${env:MYSQL_DATABASE}
    collection_interval: 60s
    transport: tcp
    allow_native_passwords: true
    tls:
      insecure: false
    resource_attributes:
      db.system: mysql
```

### SQLQuery Receiver (Standard OTEL)

```yaml
receivers:
  sqlquery/postgresql:
    driver: postgres
    datasource: "host=${env:POSTGRES_HOST} port=${env:POSTGRES_PORT} user=${env:POSTGRES_USER} password=${env:POSTGRES_PASSWORD} dbname=${env:POSTGRES_DATABASE} sslmode=disable"
    collection_interval: 60s
    queries:
      # Query Performance from pg_stat_statements
      - sql: |
          SELECT 
            queryid,
            LEFT(query, 100) as query_text,
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
            value_type: double
            unit: ms
            monotonic: true
            attributes:
              - column_name: queryid
                attribute_name: query.id
              - column_name: query_text
                attribute_name: query.text
          - metric_name: db.query.exec_time.mean
            value_column: mean_exec_time
            value_type: double
            unit: ms
          - metric_name: db.query.calls
            value_column: calls
            value_type: int
            monotonic: true
            
      # Active Sessions (ASH-like)
      - sql: |
          SELECT 
            state,
            wait_event_type,
            wait_event,
            query,
            COUNT(*) as session_count
          FROM pg_stat_activity
          WHERE state != 'idle'
          GROUP BY state, wait_event_type, wait_event, query
        metrics:
          - metric_name: db.sessions.active
            value_column: session_count
            value_type: int
            attributes:
              - column_name: state
                attribute_name: session.state
              - column_name: wait_event_type
                attribute_name: wait.type
              - column_name: wait_event
                attribute_name: wait.event
```

## Processors Configuration

### Standard OTEL Processors

#### Memory Limiter
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 30
```

#### Batch Processor
```yaml
processors:
  batch:
    timeout: 10s
    send_batch_size: 10000
    send_batch_max_size: 11000
```

#### Resource Processor
```yaml
processors:
  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: insert
      - key: deployment.environment
        value: ${env:DEPLOYMENT_ENV}
        action: insert
      - key: service.version
        value: ${env:SERVICE_VERSION}
        action: insert
      - key: host.name
        from_attribute: host.name
        action: insert
```

#### Transform Processor (PII Sanitization)
```yaml
processors:
  transform:
    error_mode: ignore
    metric_statements:
      - context: datapoint
        statements:
          # Email sanitization
          - replace_pattern(attributes["query.text"], "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b", "[EMAIL]")
          # SSN sanitization
          - replace_pattern(attributes["query.text"], "\\b\\d{3}-\\d{2}-\\d{4}\\b", "[SSN]")
          # Credit card sanitization
          - replace_pattern(attributes["query.text"], "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b", "[CARD]")
          # Phone number sanitization
          - replace_pattern(attributes["query.text"], "\\b\\d{3}[-.]?\\d{3}[-.]?\\d{4}\\b", "[PHONE]")
          # UUID sanitization
          - replace_pattern(attributes["query.text"], "\\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\\b", "[UUID]")
```

### Custom Processors (Gap Fillers)

#### Adaptive Sampler
```yaml
processors:
  database_intelligence/adaptivesampler:
    # Sampling rates
    min_sampling_rate: 0.1      # 10% minimum
    max_sampling_rate: 1.0      # 100% for slow queries
    
    # Performance thresholds
    high_cost_threshold_ms: 1000   # Queries > 1s are high cost
    error_sampling_rate: 1.0       # Always sample errors
    
    # Deduplication
    deduplication_window: 300s     # 5 minutes
    max_dedup_entries: 10000       # Limit memory usage
    
    # State persistence
    state_file: /var/lib/otelcol/adaptive_sampler_state.json
    save_interval: 60s
    
    # Sampling rules (optional)
    rules:
      - name: "slow_queries"
        condition: "attributes[\"db.query.exec_time.mean\"] > 1000"
        sampling_rate: 1.0
      - name: "frequent_queries"
        condition: "attributes[\"db.query.calls\"] > 10000"
        sampling_rate: 0.5
```

#### Circuit Breaker
```yaml
processors:
  database_intelligence/circuitbreaker:
    # Circuit breaker thresholds
    failure_threshold: 5           # Failures to open circuit
    success_threshold: 2           # Successes to close circuit
    timeout: 30s                   # Timeout for operations
    cooldown_period: 60s           # Time in open state
    half_open_max_requests: 3      # Requests in half-open
    
    # Monitoring configuration
    monitor_databases:
      - postgresql
      - mysql
    
    # Resource protection
    resource_limits:
      max_cpu_percent: 80
      max_memory_mb: 1024
      max_goroutines: 1000
    
    # Database-specific settings
    database_configs:
      postgresql:
        query_timeout: 5s
        connection_timeout: 10s
        max_connections: 10
      mysql:
        query_timeout: 5s
        connection_timeout: 10s
        max_connections: 10
```

#### Verification Processor
```yaml
processors:
  database_intelligence/verification:
    # Health monitoring
    health_checks:
      enabled: true
      interval: 60s
      thresholds:
        memory_percent: 80
        cpu_percent: 90
        disk_percent: 95
        error_rate: 0.05
      database_connectivity:
        timeout: 5s
        retry_count: 3
    
    # Metric quality validation
    metric_quality:
      enabled: true
      required_fields: 
        - "db.system"
        - "db.name"
        - "service.name"
      schema_validation:
        strict: false
        max_cardinality: 10000
        check_types: true
      duplicate_detection:
        enabled: true
        window: 60s
    
    # PII detection
    pii_detection:
      enabled: true
      sensitivity: medium  # low, medium, high
      scan_fields:
        - "query.text"
        - "error.message"
        - "db.statement"
      patterns:
        email: "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b"
        ssn: "\\b\\d{3}-\\d{2}-\\d{4}\\b"
        credit_card: "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"
      action: "alert"  # alert, redact, drop
    
    # Auto-tuning
    auto_tuning:
      enabled: true
      analysis_window: 5m
      confidence_threshold: 0.8
      max_change_percent: 20
      parameters:
        - sampling_rate
        - batch_size
        - collection_interval
    
    # Self-healing
    self_healing:
      enabled: true
      max_retry_attempts: 3
      backoff_multiplier: 2.0
      memory_pressure_threshold: 0.8
      actions:
        - garbage_collection
        - cache_clear
        - connection_reset
```

## Exporters Configuration

### OTLP Exporter (New Relic)
```yaml
exporters:
  otlp/newrelic:
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
    compression: gzip
    tls:
      insecure: false
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
    timeout: 30s
```

### Prometheus Exporter
```yaml
exporters:
  prometheus:
    endpoint: 0.0.0.0:8888
    namespace: db_intelligence
    const_labels:
      environment: ${env:DEPLOYMENT_ENV}
    metric_expiration: 5m
    enable_open_metrics: true
```

### Debug Exporter
```yaml
exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 200
```

## Service Configuration

### Extensions
```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    path: "/"
    check_collector_pipeline:
      enabled: true
      interval: 5s
      exporter_failure_threshold: 5
    
  pprof:
    endpoint: 0.0.0.0:1777
    block_profile_fraction: 0
    mutex_profile_fraction: 0
    
  zpages:
    endpoint: 0.0.0.0:55679
```

### Service Pipelines
```yaml
service:
  extensions: [health_check, pprof, zpages]
  
  pipelines:
    # Infrastructure metrics pipeline
    metrics/infrastructure:
      receivers: [postgresql, mysql]
      processors: 
        - memory_limiter
        - resource
        - batch
      exporters: [otlp/newrelic, prometheus]
    
    # Query performance pipeline
    metrics/queries:
      receivers: [sqlquery/postgresql, sqlquery/mysql]
      processors:
        - memory_limiter
        - transform  # PII sanitization
        - database_intelligence/adaptivesampler
        - database_intelligence/circuitbreaker
        - database_intelligence/verification
        - batch
        - resource
      exporters: [otlp/newrelic, prometheus]
    
    # Internal telemetry pipeline
    metrics/internal:
      receivers: [prometheus]
      processors: [memory_limiter, batch]
      exporters: [prometheus]
      
  telemetry:
    logs:
      level: ${env:LOG_LEVEL:-info}
      development: false
      encoding: json
      output_paths: ["stdout", "/var/log/otelcol/collector.log"]
      error_output_paths: ["stderr"]
      initial_fields:
        service: "database-intelligence"
    metrics:
      level: detailed
      address: 0.0.0.0:8889
    traces:
      processors:
        - batch
```

## Complete Examples

### Minimal Development Configuration
```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    databases: [postgres]

processors:
  memory_limiter:
  batch:

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, batch]
      exporters: [debug]
```

### Production Configuration
See `config/collector.yaml` for a complete production-ready configuration.

### High-Security Configuration
```yaml
# Additional security measures
processors:
  transform:
    error_mode: ignore
    metric_statements:
      # Aggressive PII removal
      - context: datapoint
        statements:
          - delete_key(attributes, "query.text") where attributes["contains_pii"] == "true"
          
  database_intelligence/verification:
    pii_detection:
      sensitivity: high
      action: drop  # Drop metrics with PII
      
receivers:
  postgresql:
    tls:
      insecure: false
      ca_file: /secrets/ca.crt
      cert_file: /secrets/client.crt
      key_file: /secrets/client.key
```

## Configuration Best Practices

1. **Use Environment Variables**: Never hardcode credentials
2. **Enable Health Checks**: Essential for production monitoring
3. **Set Resource Limits**: Prevent collector from consuming too many resources
4. **Configure Batching**: Reduces API calls and improves efficiency
5. **Enable Circuit Breakers**: Protect databases from monitoring overhead
6. **Implement PII Sanitization**: Comply with privacy regulations
7. **Use Verification**: Ensure data quality and system health
8. **Monitor Internal Metrics**: Track collector performance
9. **Configure Retries**: Handle transient failures gracefully
10. **Set Appropriate Timeouts**: Prevent hanging operations