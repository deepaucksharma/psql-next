# Configuration Guide

This guide covers all configuration options for Database Intelligence.

## Table of Contents
- [Environment Variables](#environment-variables)
- [Config-Only Mode](#config-only-mode)
- [Custom Mode](#custom-mode)
- [PostgreSQL Metrics](#postgresql-metrics)
- [Processors](#processors)
- [Exporters](#exporters)

## Environment Variables

### Required Variables

```bash
# New Relic Configuration
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"

# PostgreSQL Connection
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="5432"
export POSTGRES_USER="postgres"
export POSTGRES_PASSWORD="postgres"
export POSTGRES_DB="testdb"
```

### Optional Variables

```bash
# New Relic Endpoint (default: https://otlp.nr-data.net:4317)
export NEW_RELIC_OTLP_ENDPOINT="https://otlp.nr-data.net:4317"

# Service Identification
export OTEL_SERVICE_NAME="database-intelligence"
export DEPLOYMENT_MODE="config-only"  # or "custom"

# Performance Tuning
export OTEL_COLLECTOR_MEMORY_LIMIT="512MiB"
export OTEL_COLLECTOR_CPU_LIMIT="1000m"
```

## Config-Only Mode

Complete configuration for standard OpenTelemetry components:

```yaml
receivers:
  # PostgreSQL Receiver
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    transport: tcp
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DB}
    collection_interval: 10s
    tls:
      insecure: true
    metrics:
      # All 35+ metrics enabled
      postgresql.backends:
        enabled: true
      postgresql.bgwriter.buffers.allocated:
        enabled: true
      postgresql.bgwriter.buffers.writes:
        enabled: true
      postgresql.bgwriter.checkpoint.count:
        enabled: true
      postgresql.bgwriter.duration:
        enabled: true
      postgresql.bgwriter.maxwritten:
        enabled: true
      postgresql.bgwriter.stat.checkpoints_timed:
        enabled: true
      postgresql.bgwriter.stat.checkpoints_req:
        enabled: true
      postgresql.blocks_read:
        enabled: true
      postgresql.blks_hit:
        enabled: true
      postgresql.blks_read:
        enabled: true
      postgresql.buffer.hit:
        enabled: true
      postgresql.commits:
        enabled: true
      postgresql.conflicts:
        enabled: true
      postgresql.connection.max:
        enabled: true
      postgresql.database.count:
        enabled: true
      postgresql.database.locks:
        enabled: true
      postgresql.database.rows:
        enabled: true
      postgresql.database.size:
        enabled: true
      postgresql.deadlocks:
        enabled: true
      postgresql.index.scans:
        enabled: true
      postgresql.index.size:
        enabled: true
      postgresql.live_rows:
        enabled: true
      postgresql.locks:
        enabled: true
      postgresql.operations:
        enabled: true
      postgresql.replication.data_delay:
        enabled: true
      postgresql.rollbacks:
        enabled: true
      postgresql.rows:
        enabled: true
      postgresql.sequential_scans:
        enabled: true
      postgresql.stat_activity.count:
        enabled: true
      postgresql.table.count:
        enabled: true
      postgresql.table.size:
        enabled: true
      postgresql.table.vacuum.count:
        enabled: true
      postgresql.temp_files:
        enabled: true
      postgresql.wal.age:
        enabled: true
      postgresql.wal.delay:
        enabled: true
      postgresql.wal.lag:
        enabled: true

  # SQL Query Receiver for Custom Metrics
  sqlquery/postgresql:
    driver: postgres
    datasource: "host=${env:POSTGRES_HOST} port=${env:POSTGRES_PORT} user=${env:POSTGRES_USER} password=${env:POSTGRES_PASSWORD} dbname=${env:POSTGRES_DB} sslmode=disable"
    collection_interval: 30s
    queries:
      # Connection state monitoring
      - sql: |
          SELECT 
            state,
            COUNT(*) as connection_count
          FROM pg_stat_activity
          WHERE pid != pg_backend_pid()
          GROUP BY state
        metrics:
          - metric_name: pg.connection_count
            value_column: "connection_count"
            attribute_columns: ["state"]
            value_type: int
            
      # Wait event monitoring
      - sql: |
          SELECT 
            wait_event_type,
            wait_event,
            COUNT(*) as count
          FROM pg_stat_activity
          WHERE wait_event IS NOT NULL
          GROUP BY wait_event_type, wait_event
        metrics:
          - metric_name: pg.wait_events
            value_column: "count"
            attribute_columns: ["wait_event_type", "wait_event"]
            value_type: int

      # Database operations
      - sql: |
          SELECT 
            datname,
            numbackends,
            tup_returned,
            tup_fetched,
            tup_inserted,
            tup_updated,
            tup_deleted,
            conflicts,
            temp_files,
            temp_bytes,
            deadlocks,
            blks_read,
            blks_hit
          FROM pg_stat_database
          WHERE datname NOT IN ('template0', 'template1')
        metrics:
          - metric_name: pg.database.operations
            value_column: "tup_returned"
            attribute_columns: ["datname"]
            value_type: int
            data_type: sum

  # Host Metrics
  hostmetrics:
    collection_interval: 10s
    scrapers:
      cpu:
        metrics:
          system.cpu.utilization:
            enabled: true
      memory:
        metrics:
          system.memory.utilization:
            enabled: true
      disk:
        metrics:
          system.disk.io:
            enabled: true
          system.disk.operations:
            enabled: true
      network:
        metrics:
          system.network.io:
            enabled: true
          system.network.errors:
            enabled: true

processors:
  # Add deployment mode attribute
  attributes:
    actions:
      - key: deployment.mode
        value: config-only
        action: insert
      - key: service.name
        value: ${env:OTEL_SERVICE_NAME}
        action: insert
      - key: service.version
        value: "2.0.0"
        action: insert

  # Resource detection
  resourcedetection:
    detectors: [env, system, docker]
    system:
      hostname_sources: ["os"]
    docker:
      use_hostname_if_present: true

  # Batch processing
  batch:
    timeout: 10s
    send_batch_size: 1000

  # Memory limiter
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128

exporters:
  # New Relic OTLP exporter
  otlp:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
    compression: gzip
    
  # Debug exporter (optional)
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 100

service:
  telemetry:
    logs:
      level: info
      encoding: console
    metrics:
      level: detailed

  pipelines:
    metrics:
      receivers: 
        - postgresql
        - sqlquery/postgresql
        - hostmetrics
      processors:
        - memory_limiter
        - resourcedetection
        - attributes
        - batch
      exporters:
        - otlp
        # - debug  # Uncomment for troubleshooting
```

## Custom Mode

Additional components for enhanced monitoring:

```yaml
receivers:
  # All config-only receivers plus:
  
  # ASH (Active Session History) receiver
  ash:
    driver: postgres
    datasource: "host=${env:POSTGRES_HOST} port=${env:POSTGRES_PORT} user=${env:POSTGRES_USER} password=${env:POSTGRES_PASSWORD} dbname=${env:POSTGRES_DB} sslmode=disable"
    collection_interval: 1s
    sampling:
      base_rate: 1.0
      min_rate: 0.1
      max_rate: 1.0
      low_session_threshold: 50
      high_session_threshold: 500
      always_sample_blocked: true
      always_sample_long_running: true
      always_sample_maintenance: true
    buffer_size: 10000
    retention_duration: 1h
    aggregation_windows:
      - 1m
      - 5m
      - 15m
      - 1h
    enable_feature_detection: true
    enable_wait_analysis: true
    enable_blocking_analysis: true
    enable_anomaly_detection: true
    slow_query_threshold_ms: 1000
    blocked_session_threshold: 5

  # Enhanced SQL receiver
  enhancedsql:
    driver: postgres
    datasource: "host=${env:POSTGRES_HOST} port=${env:POSTGRES_PORT} user=${env:POSTGRES_USER} password=${env:POSTGRES_PASSWORD} dbname=${env:POSTGRES_DB} sslmode=disable"
    collection_interval: 30s
    queries:
      - name: query_stats
        sql: |
          SELECT 
            queryid::text as query_id,
            LEFT(query, 100) as query_text,
            calls,
            total_exec_time,
            mean_exec_time,
            rows,
            shared_blks_hit,
            shared_blks_read
          FROM pg_stat_statements
          WHERE query NOT LIKE '%pg_stat_statements%'
          ORDER BY total_exec_time DESC
          LIMIT 100

processors:
  # All config-only processors plus:
  
  # Adaptive sampling
  adaptivesampler:
    sampling_percentage: 100
    evaluation_interval: 30s
    decision_wait: 10s
    num_traces: 100000
    expected_new_traces_per_sec: 10000
    policies:
      - policy_type: adaptive
        sampling_percentage: 100
        
  # Circuit breaker
  circuitbreaker:
    failure_threshold: 5
    recovery_timeout: 30s
    metrics_limit: 100000
    
  # Cost control
  costcontrol:
    max_datapoints_per_minute: 1000000
    enforcement_mode: drop
    
  # Query plan extraction
  planattributeextractor:
    timeout: 5s
    cache_size: 1000
    extract_parameters: true
    
  # Query correlation
  querycorrelator:
    correlation_window: 5m
    max_correlated_queries: 100
    
  # OHI transformation
  ohitransform:
    transform_rules:
      - source_metric: "db.ash.active_sessions"
        target_event: "PostgresSlowQueries"
        mappings:
          "db.postgresql.query_id": "query_id"
          "db.query.execution_time_mean": "avg_elapsed_time_ms"

exporters:
  # All config-only exporters plus:
  
  # New Relic Infrastructure exporter
  nri:
    license_key: ${env:NEW_RELIC_LICENSE_KEY}
    events:
      enabled: true
    metrics:
      enabled: false

extensions:
  # PostgreSQL query extension
  postgresqlquery:
    datasource: "host=${env:POSTGRES_HOST} port=${env:POSTGRES_PORT} user=${env:POSTGRES_USER} password=${env:POSTGRES_PASSWORD} dbname=${env:POSTGRES_DB} sslmode=disable"
    
  # Health check
  health_check:
    endpoint: 0.0.0.0:13133
    
  # Performance profiler
  pprof:
    endpoint: 0.0.0.0:1777

service:
  extensions: [health_check, pprof, postgresqlquery]
  
  pipelines:
    metrics:
      receivers: 
        - postgresql
        - ash
        - enhancedsql
        - hostmetrics
      processors:
        - memory_limiter
        - circuitbreaker
        - adaptivesampler
        - costcontrol
        - planattributeextractor
        - querycorrelator
        - resourcedetection
        - attributes
        - batch
      exporters:
        - otlp
        
    # Events pipeline for OHI compatibility
    metrics/events:
      receivers:
        - ash
        - enhancedsql
      processors:
        - memory_limiter
        - ohitransform
        - batch
      exporters:
        - nri
```

## PostgreSQL Metrics

### Standard Metrics (35+)

| Metric | Description | Unit |
|--------|-------------|------|
| postgresql.backends | Number of backends | connections |
| postgresql.bgwriter.buffers.allocated | Buffers allocated | buffers |
| postgresql.bgwriter.buffers.writes | Buffer writes | writes |
| postgresql.bgwriter.checkpoint.count | Checkpoint count | checkpoints |
| postgresql.bgwriter.duration | Checkpoint duration | ms |
| postgresql.blocks_read | Blocks read | blocks |
| postgresql.blks_hit | Buffer hits | hits |
| postgresql.blks_read | Disk reads | reads |
| postgresql.commits | Transactions committed | transactions |
| postgresql.rollbacks | Transactions rolled back | transactions |
| postgresql.deadlocks | Deadlocks detected | deadlocks |
| postgresql.database.size | Database size | bytes |
| postgresql.table.size | Table size | bytes |
| postgresql.index.size | Index size | bytes |
| postgresql.rows | Row operations | operations |
| postgresql.temp_files | Temporary files created | files |
| postgresql.wal.lag | WAL replication lag | bytes |

### Custom Metrics (SQL Query Receiver)

| Metric | Description | Attributes |
|--------|-------------|------------|
| pg.connection_count | Connections by state | state |
| pg.wait_events | Wait events | wait_event_type, wait_event |
| pg.database.operations | Database operations | datname |

### Enhanced Metrics (Custom Mode Only)

| Metric | Description | Attributes |
|--------|-------------|------------|
| db.ash.active_sessions | Active sessions | state, wait_event |
| db.ash.blocked_sessions | Blocked sessions | blocking_pid |
| db.ash.long_running_queries | Long queries | query_start |
| postgres.slow_queries.* | Query statistics | query_id, plan_type |

## Processors

### Standard Processors

1. **attributes** - Add/modify attributes
2. **batch** - Batch metrics before sending
3. **memory_limiter** - Prevent OOM conditions
4. **resourcedetection** - Detect host/container resources

### Enhanced Processors (Custom Mode)

1. **adaptivesampler** - Dynamic sampling based on load
2. **circuitbreaker** - Protect against overload
3. **costcontrol** - Limit data points per minute
4. **planattributeextractor** - Extract query plans
5. **querycorrelator** - Correlate related queries
6. **ohitransform** - OHI compatibility

## Exporters

### OTLP Exporter (Both Modes)

```yaml
otlp:
  endpoint: otlp.nr-data.net:4317
  headers:
    api-key: ${env:NEW_RELIC_LICENSE_KEY}
  compression: gzip
  retry_on_failure:
    enabled: true
    initial_interval: 5s
    max_interval: 30s
    max_elapsed_time: 300s
```

### Debug Exporter (Development)

```yaml
debug:
  verbosity: detailed
  sampling_initial: 10
  sampling_thereafter: 100
```

## Configuration Best Practices

1. **Collection Intervals**
   - PostgreSQL metrics: 10-30s
   - SQL queries: 30-60s
   - ASH: 1s (custom mode)
   - Host metrics: 10s

2. **Memory Management**
   - Set appropriate memory limits
   - Use batch processor
   - Enable compression

3. **Security**
   - Use environment variables for credentials
   - Enable TLS for database connections
   - Restrict collector network access

4. **Performance**
   - Adjust collection intervals based on load
   - Use sampling in high-volume environments
   - Monitor collector resource usage

5. **Troubleshooting**
   - Enable debug exporter temporarily
   - Check collector logs
   - Verify metric flow with NRQL

## Environment-Specific Configurations

### Development
```yaml
service:
  telemetry:
    logs:
      level: debug
  pipelines:
    metrics:
      exporters: [debug, otlp]
```

### Production
```yaml
processors:
  memory_limiter:
    limit_mib: 2048
  batch:
    send_batch_size: 5000
service:
  telemetry:
    logs:
      level: error
```

## Validation

Verify configuration:
```bash
# Validate YAML syntax
docker run --rm -v $(pwd)/config.yaml:/config.yaml \
  otel/opentelemetry-collector-contrib:latest \
  --config=/config.yaml --dry-run

# Check metrics flow
curl -s http://localhost:13133/metrics | grep otelcol_receiver_accepted_metric_points
```