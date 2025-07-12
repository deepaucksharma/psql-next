# Custom Components Integration Guide

This guide explains how to integrate and use the custom receivers and processors in the Database Intelligence OpenTelemetry Collector.

## Table of Contents
- [Overview](#overview)
- [Building with Custom Components](#building-with-custom-components)
- [Custom Receivers](#custom-receivers)
  - [ASH Receiver](#ash-receiver)
  - [Enhanced SQL Receiver](#enhanced-sql-receiver)
  - [Kernel Metrics Receiver](#kernel-metrics-receiver)
- [Custom Processors](#custom-processors)
  - [Adaptive Sampler](#adaptive-sampler-processor)
  - [Circuit Breaker](#circuit-breaker-processor)
  - [Plan Attribute Extractor](#plan-attribute-extractor-processor)
  - [Verification Processor](#verification-processor)
  - [Cost Control Processor](#cost-control-processor)
  - [NR Error Monitor](#nr-error-monitor-processor)
  - [Query Correlator](#query-correlator-processor)
- [Configuration Examples](#configuration-examples)
- [Performance Considerations](#performance-considerations)
- [Troubleshooting](#troubleshooting)

## Overview

The Database Intelligence Collector includes custom components that extend the standard OpenTelemetry Collector functionality with advanced database monitoring capabilities:

**Custom Receivers** collect specialized metrics:
- **ASH**: Active Session History for wait event analysis
- **Enhanced SQL**: Custom SQL queries for detailed metrics
- **Kernel Metrics**: System-level database performance metrics

**Custom Processors** add intelligence to the pipeline:
- **Adaptive Sampler**: Intelligent sampling based on query patterns
- **Circuit Breaker**: Protects databases from overload
- **Plan Extractor**: Analyzes query execution plans
- **Verification**: PII detection and data validation
- **Cost Control**: Monitors and controls data costs
- **NR Error Monitor**: New Relic-specific error tracking
- **Query Correlator**: Correlates queries with application traces

## Building with Custom Components

### Prerequisites
- Go 1.21 or later
- OpenTelemetry Collector Builder (ocb) v0.105.0

### Build Steps

1. **Install the builder**:
   ```bash
   go install go.opentelemetry.io/collector/cmd/builder@v0.105.0
   ```

2. **Build the collector with all components**:
   ```bash
   builder --config=otelcol-builder-config-complete.yaml
   ```

3. **Verify the build**:
   ```bash
   ./distributions/production/database-intelligence-collector components
   ```

### Using Pre-built Script
```bash
./scripts/build-collector.sh
```

## Custom Receivers

### ASH Receiver

The Active Session History (ASH) receiver collects real-time session data similar to Oracle's ASH, providing visibility into database wait events and active sessions.

#### Configuration
```yaml
receivers:
  ash:
    databases:
      - type: postgresql
        endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
        username: ${env:POSTGRES_USER}
        password: ${env:POSTGRES_PASSWORD}
        database: ${env:POSTGRES_DB}
        collection_interval: 10s   # How often to collect samples
        sample_interval: 1s        # Sampling frequency
        max_sessions: 100          # Max sessions to track
        
      - type: mysql
        endpoint: ${env:MYSQL_HOST}:${env:MYSQL_PORT}
        username: ${env:MYSQL_USER}
        password: ${env:MYSQL_PASSWORD}
        database: ${env:MYSQL_DB}
        collection_interval: 10s
        sample_interval: 1s
```

#### Metrics Collected
- `ash.session.count`: Number of active sessions
- `ash.wait.time`: Time spent in wait events
- `ash.cpu.time`: CPU time used by sessions
- `ash.query.hash`: Query identifier for grouping

#### Attributes
- `wait.event.class`: Wait event category (e.g., CPU, IO, Lock)
- `wait.event.name`: Specific wait event
- `session.id`: Database session identifier
- `query.hash`: Query fingerprint
- `user.name`: Database user

### Enhanced SQL Receiver

Executes custom SQL queries to collect metrics not available through standard receivers.

#### Configuration
```yaml
receivers:
  enhancedsql:
    postgresql:
      - endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
        username: ${env:POSTGRES_USER}
        password: ${env:POSTGRES_PASSWORD}
        database: ${env:POSTGRES_DB}
        collection_interval: 60s
        queries:
          - name: "slow_queries"
            sql: |
              SELECT query, calls, total_time, mean_time
              FROM pg_stat_statements
              WHERE mean_time > 1000
              ORDER BY mean_time DESC
              LIMIT 20
            metrics:
              - name: "query.calls"
                value_column: "calls"
                unit: "1"
                attributes:
                  - "query"
              - name: "query.total_time"
                value_column: "total_time"
                unit: "ms"
                attributes:
                  - "query"
                  
          - name: "table_stats"
            sql: |
              SELECT schemaname, tablename, 
                     n_tup_ins, n_tup_upd, n_tup_del,
                     n_live_tup, n_dead_tup
              FROM pg_stat_user_tables
            metrics:
              - name: "table.inserts"
                value_column: "n_tup_ins"
                monotonic: true
                attributes:
                  - "schemaname"
                  - "tablename"
```

#### Best Practices
- Keep queries lightweight to avoid impacting database performance
- Use appropriate collection intervals (longer for expensive queries)
- Include LIMIT clauses to bound result sets
- Test queries manually before adding to configuration

### Kernel Metrics Receiver

Collects OS-level metrics specific to database performance monitoring.

#### Configuration
```yaml
receivers:
  kernelmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
        enabled: true
        metrics:
          - iowait_percentage    # Important for database I/O
          - steal_percentage     # For virtualized environments
      memory:
        enabled: true
        metrics:
          - page_faults
          - swap_usage
          - buffer_cache_size
      disk:
        enabled: true
        devices: ["/dev/sda", "/dev/sdb"]  # Database storage devices
        metrics:
          - read_latency
          - write_latency
          - queue_depth
      network:
        enabled: true
        interfaces: ["eth0", "eth1"]  # Database network interfaces
      filesystem:
        enabled: true
        mount_points: ["/var/lib/postgresql", "/var/lib/mysql"]
```

#### Metrics Focus
- **I/O Wait**: Critical for database performance
- **Page Cache**: Database buffer efficiency
- **Disk Latency**: Storage subsystem performance
- **Network Throughput**: For distributed databases

## Custom Processors

### Adaptive Sampler Processor

Intelligently samples database telemetry based on query patterns and importance.

#### Configuration
```yaml
processors:
  adaptivesampler:
    # Base sampling rate (percentage)
    sampling_percentage: ${env:ADAPTIVE_SAMPLING_PERCENTAGE:-10}
    
    # Maximum traces per second (rate limiting)
    max_traces_per_second: ${env:MAX_TRACES_PER_SECOND:-100}
    
    # Cache configuration
    cache_size: ${env:SAMPLED_CACHE_SIZE:-100000}
    
    # Query-type specific sampling
    query_types:
      select: ${env:SELECT_SAMPLING_PERCENTAGE:-5}      # Sample 5% of SELECTs
      insert: ${env:INSERT_SAMPLING_PERCENTAGE:-20}     # Sample 20% of INSERTs
      update: ${env:UPDATE_SAMPLING_PERCENTAGE:-20}     # Sample 20% of UPDATEs
      delete: ${env:DELETE_SAMPLING_PERCENTAGE:-50}     # Sample 50% of DELETEs
      ddl: 100                                          # Always sample DDL
      system: 100                                       # Always sample system queries
      
    # Always sample queries matching these patterns
    always_sample_patterns:
      - "ALTER TABLE.*"
      - "DROP.*"
      - "TRUNCATE.*"
      - ".*LOCK TABLE.*"
      
    # Never sample queries matching these patterns
    never_sample_patterns:
      - "SELECT 1"  # Health checks
      - "SELECT version()"
      
    # Duration-based sampling (milliseconds)
    duration_thresholds:
      - threshold: 1000    # > 1s
        percentage: 100    # Always sample
      - threshold: 500     # > 500ms
        percentage: 50     # Sample 50%
      - threshold: 100     # > 100ms
        percentage: 20     # Sample 20%
```

#### Sampling Logic
1. Checks always/never sample patterns first
2. Applies duration-based rules
3. Falls back to query type percentages
4. Enforces global rate limit

### Circuit Breaker Processor

Protects databases from overload by stopping metric collection when errors exceed thresholds.

#### Configuration
```yaml
processors:
  circuit_breaker:
    # Failure detection
    max_consecutive_failures: ${env:CIRCUIT_BREAKER_MAX_FAILURES:-5}
    failure_threshold_percent: ${env:CIRCUIT_BREAKER_FAILURE_THRESHOLD:-50}
    
    # Timing configuration
    timeout: ${env:CIRCUIT_BREAKER_TIMEOUT:-30s}
    recovery_timeout: ${env:CIRCUIT_BREAKER_RECOVERY_TIMEOUT:-60s}
    
    # Per-database circuit breakers
    per_database: ${env:PER_DATABASE_CIRCUIT:-true}
    
    # Health check configuration
    health_check_interval: ${env:HEALTH_CHECK_INTERVAL:-10s}
    health_check_query: "SELECT 1"
    
    # Actions when circuit opens
    on_open:
      - log_level: "error"
      - alert: true
      - reduce_collection_interval: 5x  # Reduce load
```

#### Circuit States
- **Closed**: Normal operation
- **Open**: Blocking all requests after threshold exceeded
- **Half-Open**: Testing if database recovered

### Plan Attribute Extractor Processor

Analyzes query execution plans to add insights as attributes.

#### Configuration
```yaml
processors:
  planattributeextractor:
    # Enable plan collection
    enabled: ${env:ENABLE_PLAN_ANALYSIS:-true}
    
    # Caching for performance
    cache_enabled: ${env:PLAN_CACHE_ENABLED:-true}
    cache_size: ${env:PLAN_CACHE_SIZE:-1000}
    cache_ttl: ${env:PLAN_CACHE_TTL:-3600s}
    
    # Query handling
    max_query_length: ${env:MAX_QUERY_LENGTH:-4096}
    anonymize: ${env:ENABLE_ANONYMIZATION:-true}
    
    # Plan analysis features
    extract_features:
      - scan_types        # Sequential, index, bitmap scans
      - join_types        # Nested loop, hash, merge joins
      - sort_operations   # In-memory vs disk sorts
      - index_usage       # Which indexes were used
      - cost_estimates    # Planner cost estimates
      
    # Attribute enrichment
    add_attributes:
      - "plan.total_cost"
      - "plan.scan_type"
      - "plan.join_type"
      - "plan.uses_index"
      - "plan.estimated_rows"
      - "plan.actual_rows"
      - "plan.loops"
```

#### Plan Insights
- Identifies missing indexes
- Detects inefficient join strategies
- Highlights table scan operations
- Tracks plan changes over time

### Verification Processor

Validates data quality and detects potential PII.

#### Configuration
```yaml
processors:
  verification:
    # PII Detection
    pii_detection_enabled: ${env:ENABLE_PII_DETECTION:-true}
    pii_patterns:
      ssn: '\d{3}-\d{2}-\d{4}'
      credit_card: '\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}'
      email: '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}'
      phone: '\+?1?\d{10,14}'
      
    # Data validation
    validation_enabled: ${env:ENABLE_DATA_VALIDATION:-true}
    validation_rules:
      - field: "query.duration"
        min: 0
        max: 3600000  # 1 hour max
      - field: "connection.count"
        min: 0
        max: 1000
        
    # Field length limits
    max_field_length: ${env:MAX_FIELD_LENGTH:-1000}
    truncate_fields: true
    
    # Sampling for performance
    sample_rate: ${env:VERIFICATION_SAMPLE_RATE:-0.1}  # Check 10% of data
    
    # Actions on PII detection
    on_pii_detected:
      - redact: true        # Replace with [REDACTED]
      - add_attribute: "pii.detected=true"
      - increment_metric: "pii.detections.total"
```

### Cost Control Processor

Monitors data volume and estimates costs for budget management.

#### Configuration
```yaml
processors:
  costcontrol:
    # Budget configuration
    daily_budget_usd: ${env:DAILY_BUDGET_USD:-100}
    monthly_budget_usd: ${env:MONTHLY_BUDGET_USD:-3000}
    
    # Cost model
    cost_per_gb: ${env:COST_PER_GB:-0.25}
    cost_per_million_events: ${env:COST_PER_MILLION_EVENTS:-2.00}
    
    # Alert thresholds
    alert_threshold_percent: ${env:COST_ALERT_THRESHOLD:-80}
    
    # Enforcement
    enforcement_enabled: ${env:COST_ENFORCEMENT_ENABLED:-false}
    enforcement_action: "throttle"  # or "drop"
    
    # Cost allocation
    track_by:
      - database.name
      - query.type
      - deployment.environment
      
    # Reporting
    report_interval: 1h
    report_attributes:
      - "cost.hourly.usd"
      - "cost.daily.usd"
      - "cost.monthly.projected.usd"
      - "budget.remaining.percentage"
```

### NR Error Monitor Processor

Monitors New Relic integration health and errors.

#### Configuration
```yaml
processors:
  nrerrormonitor:
    # Error detection
    error_threshold: ${env:NR_ERROR_THRESHOLD:-10}
    error_window: 5m
    
    # Validation
    validation_interval: ${env:NR_VALIDATION_INTERVAL:-300s}
    validation_enabled: ${env:ENABLE_NR_VALIDATION:-true}
    
    # New Relic API validation
    validate_endpoints:
      - metric_api: "https://metric-api.newrelic.com/metric/v1"
      - trace_api: "https://trace-api.newrelic.com/trace/v1"
      
    # Error categorization
    error_categories:
      - authentication: "401|403|Invalid API Key"
      - rate_limit: "429|Rate limit exceeded"
      - payload_size: "413|Payload too large"
      - format: "400|Invalid format"
      
    # Auto-recovery actions
    on_error:
      authentication:
        - alert: "critical"
        - pause_exports: true
      rate_limit:
        - backoff: "exponential"
        - reduce_batch_size: 0.5
      payload_size:
        - split_batch: true
        - compress: "gzip"
```

### Query Correlator Processor

Correlates database queries with application traces and metrics.

#### Configuration
```yaml
processors:
  querycorrelator:
    # Correlation window
    correlation_window: ${env:CORRELATION_WINDOW:-30s}
    max_correlations: ${env:MAX_CORRELATIONS:-1000}
    
    # Trace correlation
    trace_correlation_enabled: ${env:ENABLE_TRACE_CORRELATION:-true}
    trace_id_attributes:
      - "trace.id"
      - "span.id"
      - "parent.span.id"
      
    # Application correlation
    application_attributes:
      - "service.name"
      - "service.version"
      - "http.route"
      - "http.method"
      
    # Query fingerprinting
    query_normalization: true
    query_fingerprint_algorithm: "md5"
    
    # Correlation storage
    storage_backend: "memory"  # or "redis"
    redis_endpoint: "localhost:6379"
    
    # Enrichment
    enrich_with:
      - application_metrics
      - infrastructure_metrics
      - user_context
```

## Configuration Examples

### Basic Configuration with Custom Components
```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    collection_interval: 60s
    
processors:
  memory_limiter:
    limit_mib: 512
  batch:
    timeout: 1s

exporters:
  otlphttp:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, batch]
      exporters: [otlphttp]
```

### Advanced Configuration with All Custom Components
```yaml
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:5432
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 60s
    
  ash:
    databases:
      - type: postgresql
        endpoint: ${POSTGRES_HOST}:5432
        username: ${POSTGRES_USER}
        password: ${POSTGRES_PASSWORD}
        collection_interval: 10s
        
  enhancedsql:
    postgresql:
      - endpoint: ${POSTGRES_HOST}:5432
        username: ${POSTGRES_USER}
        password: ${POSTGRES_PASSWORD}
        collection_interval: 300s
        queries:
          - name: "custom_metrics"
            sql: "SELECT * FROM pg_stat_statements"
            
  kernelmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
        enabled: true
      disk:
        enabled: true

processors:
  memory_limiter:
    limit_mib: 1024
    
  adaptivesampler:
    sampling_percentage: 10
    query_types:
      select: 5
      
  circuit_breaker:
    max_consecutive_failures: 5
    
  planattributeextractor:
    enabled: true
    cache_enabled: true
    
  verification:
    pii_detection_enabled: true
    
  costcontrol:
    daily_budget_usd: 100
    
  nrerrormonitor:
    error_threshold: 10
    
  querycorrelator:
    trace_correlation_enabled: true
    
  batch:
    timeout: 1s

exporters:
  otlphttp:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics/advanced:
      receivers: [postgresql, ash, enhancedsql, kernelmetrics]
      processors: 
        - memory_limiter
        - adaptivesampler
        - circuit_breaker
        - planattributeextractor
        - verification
        - costcontrol
        - nrerrormonitor
        - querycorrelator
        - batch
      exporters: [otlphttp]
```

## Performance Considerations

### Resource Usage
- **ASH Receiver**: ~50MB RAM per 1000 active sessions
- **Enhanced SQL**: Depends on query complexity
- **Kernel Metrics**: ~20MB RAM
- **Processors**: ~100-500MB total depending on cache sizes

### Optimization Tips
1. **Tune collection intervals** based on metric importance
2. **Use sampling** for high-volume metrics
3. **Enable caching** in processors where available
4. **Set appropriate memory limits**
5. **Monitor collector's own metrics**

### Recommended Settings by Scale
- **Small (< 10 databases)**: 512MB RAM, 30s intervals
- **Medium (10-100 databases)**: 2GB RAM, 60s intervals
- **Large (> 100 databases)**: 4GB+ RAM, 120s intervals, enable sampling

## Troubleshooting

### Common Issues

#### High Memory Usage
```yaml
# Reduce cache sizes
processors:
  planattributeextractor:
    cache_size: 100  # Reduced from 1000
  adaptivesampler:
    cache_size: 10000  # Reduced from 100000
```

#### Circuit Breaker Triggering
```bash
# Check logs for circuit breaker events
grep "circuit.*open" collector.log

# Increase thresholds if false positives
export CIRCUIT_BREAKER_MAX_FAILURES=10
export CIRCUIT_BREAKER_FAILURE_THRESHOLD=70
```

#### PII Detection Performance
```yaml
# Reduce sampling rate
processors:
  verification:
    sample_rate: 0.01  # Check only 1% of data
```

### Debug Mode
Enable debug logging for specific components:
```yaml
service:
  telemetry:
    logs:
      level: debug
      development: true
      
    # Component-specific debugging
    metrics:
      level: detailed
      readers:
        - periodic:
            interval: 10s
```

### Metrics for Monitoring
The collector exposes its own metrics at `:8888/metrics`:
- `otelcol_receiver_accepted_metric_points`
- `otelcol_processor_dropped_metric_points`
- `otelcol_exporter_sent_metric_points`
- Custom component metrics (e.g., `circuit_breaker_state`)

### Support and Resources
- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [New Relic OpenTelemetry Docs](https://docs.newrelic.com/docs/more-integrations/open-source-telemetry-integrations/opentelemetry/introduction-opentelemetry-new-relic/)
- Component source code in `components/` directory
- Integration tests in `tests/e2e/`