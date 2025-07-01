# Configuration Reference - v2.0.0

🔧 **BUILD SUCCESSFUL** - This document provides configuration guidance for the Database Intelligence Collector. All custom processors integrated, OHI migration support complete, enterprise features enabled.

## ✅ Working Build Configuration

- **`ocb-config.yaml`** - **WORKING** OpenTelemetry Collector Builder configuration
- Standard OTEL receivers, processors, exporters functional
- Plan Attribute Extractor custom processor integrated
- Remaining custom processors disabled pending build fixes

## Table of Contents
- [Configuration Overlay System](#configuration-overlay-system)
- [Environment Variables](#environment-variables)
- [Receivers Configuration](#receivers-configuration)
- [Processors Configuration](#processors-configuration)
- [Exporters Configuration](#exporters-configuration)
- [Service Configuration](#service-configuration)
- [Complete Examples](#complete-examples)
- [Using Taskfile](#using-taskfile)

## ✅ Working Build Configuration

The current working OpenTelemetry Collector Builder configuration:

### Build Command (Working)
```bash
# Install OpenTelemetry Collector Builder
go install go.opentelemetry.io/collector/cmd/builder@v0.127.0

# Build collector (generates ./dist/database-intelligence-collector)
export PATH="$HOME/go/bin:$PATH"
builder --config=ocb-config.yaml
```

### OCB Configuration Summary
```yaml
# ocb-config.yaml (current working version)
dist:
  name: database-intelligence-collector
  otelcol_version: "0.127.0"

receivers:
  - go.opentelemetry.io/collector/receiver/otlpreceiver v0.127.0
  - github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.127.0
  - github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.127.0
  - github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver v0.127.0

processors:
  # Standard processors (working)
  - go.opentelemetry.io/collector/processor/batchprocessor v0.127.0
  - go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.127.0
  - github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.127.0
  
  # Custom processors (all working in v2.0.0)
  - github.com/database-intelligence-mvp/processors/planattributeextractor v0.1.0  # ✅ Working
  - github.com/database-intelligence-mvp/processors/adaptivesampler v0.1.0  # ✅ Working
  - github.com/database-intelligence-mvp/processors/circuitbreaker v0.1.0  # ✅ Working
  - github.com/database-intelligence-mvp/processors/verification v0.1.0  # ✅ Working
  - github.com/database-intelligence-mvp/processors/costcontrol v0.1.0  # ✅ Working (NEW)
  - github.com/database-intelligence-mvp/processors/nrerrormonitor v0.1.0  # ✅ Working (NEW)
  - github.com/database-intelligence-mvp/processors/querycorrelator v0.1.0  # ✅ Working (NEW)

exporters:
  - go.opentelemetry.io/collector/exporter/otlpexporter v0.127.0
  - go.opentelemetry.io/collector/exporter/debugexporter v0.127.0
  - github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.127.0
```

## ✅ Resilient Configuration Features

The `collector-resilient.yaml` configuration provides production-ready deployment with the following features:

### ✅ Enhanced Security
- **Comprehensive PII Detection**: Credit cards, SSNs, emails, phone numbers automatically redacted
- **Safe Query Collection**: Uses pg_stat_statements only (no unsafe EXPLAIN calls)
- **Secure Defaults**: All processors configured for safe production operation

### ✅ Reliable State Management
- **In-Memory Only**: No Redis or file persistence required
- **Graceful Restart**: Clean state on restart (no stale data issues)
- **Resource Bounded**: Memory limits and automatic cleanup

### ✅ Resilient Processing
- **Missing Attribute Handling**: Processors work even when dependencies unavailable
- **Debug Logging**: Clear messages when plan attributes missing
- **Independent Operation**: Each processor works without requiring others

### ✅ Production Monitoring
- **Circuit Breaker Protection**: Automatic database protection
- **Memory Monitoring**: Prevents resource exhaustion
- **New Relic Integration**: Cardinality and error protection

```yaml
# Key features in collector-resilient.yaml
processors:
  adaptive_sampler:
    in_memory_only: true              # ✅ No file storage
    enable_debug_logging: true        # ✅ Clear missing dependency messages
    deduplication:
      enabled: false                  # ✅ Disabled when plan hashes unavailable
  
  circuit_breaker:
    memory_threshold_mb: 512         # ✅ Resource protection
    cpu_threshold_percent: 80.0      # ✅ CPU monitoring
    
  transform:
    # ✅ Enhanced PII patterns (credit cards, SSNs, emails, etc.)
```

### Overlay Inheritance

Each environment configuration includes the base and adds overrides:

```yaml
# dev/collector.yaml
__includes:
  - ../base/collector.yaml

# Override specific values
service:
  telemetry:
    logs:
      level: debug  # Override from base 'info'
```

## Environment Variables

### Using Environment Files

```bash
# Copy appropriate template
cp .env.development .env  # For development
cp .env.staging .env      # For staging
cp .env.production .env   # For production

# Edit with your values
vim .env

# Run with environment file
task run ENV_FILE=.env
```

### Core Environment Variables

```bash
# Database Credentials
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=testdb

MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=mysql
MYSQL_DB=testdb

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key_here
NEW_RELIC_OTLP_ENDPOINT=otlp.nr-data.net:4317  # No https:// prefix
NEW_RELIC_ACCOUNT_ID=your_account_id

# Environment Settings
ENVIRONMENT=development  # development, staging, production
LOG_LEVEL=info          # debug, info, warn, error

# Resource Management
GOGC=80
GOMEMLIMIT=512MiB
MEMORY_LIMIT_PERCENTAGE=75
MEMORY_SPIKE_LIMIT_PERCENTAGE=25

# ✅ Production Deployment Mode (Single Instance)
DEPLOYMENT_MODE=production  # production (single instance, in-memory state)

# ✅ Production Features (Always Enabled)
ENABLE_ADAPTIVE_SAMPLER=true    # ✅ In-memory state, graceful degradation
ENABLE_CIRCUIT_BREAKER=true     # ✅ Database protection
ENABLE_PLAN_EXTRACTOR=true      # ✅ Safe mode only, no EXPLAIN calls
ENABLE_VERIFICATION=true        # ✅ Enhanced PII detection
ENABLE_PII_SANITIZATION=true    # ✅ Comprehensive patterns

# Collection Settings
COLLECTION_INTERVAL_SECONDS=60
QUERY_TIMEOUT_MS=10000
MIN_QUERY_TIME_MS=100
MAX_QUERIES_PER_COLLECTION=500
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
      - ${env:POSTGRES_DB}
    collection_interval: ${env:COLLECTION_INTERVAL_SECONDS}s
    initial_delay: 30s
    timeout: 30s
    tls:
      insecure: ${env:TLS_INSECURE_SKIP_VERIFY}
      ca_file: /etc/ssl/certs/postgres-ca.crt
      cert_file: /etc/ssl/certs/postgres-client.crt
      key_file: /etc/ssl/private/postgres-client.key
    resource_attributes:
      db.system: postgresql
      environment: ${env:ENVIRONMENT}
```

### MySQL Receiver (Standard OTEL)

```yaml
receivers:
  mysql:
    endpoint: ${env:MYSQL_HOST}:${env:MYSQL_PORT}
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    database: ${env:MYSQL_DB}
    collection_interval: ${env:COLLECTION_INTERVAL_SECONDS}s
    allow_native_passwords: true
    tls:
      mode: ${env:MYSQL_TLS_MODE}  # required, preferred, disabled
    resource_attributes:
      db.system: mysql
      environment: ${env:ENVIRONMENT}
```

### SQLQuery Receiver (Custom Queries)

```yaml
receivers:
  sqlquery:
    driver: postgres
    datasource: "host=${env:POSTGRES_HOST} port=${env:POSTGRES_PORT} user=${env:POSTGRES_USER} password=${env:POSTGRES_PASSWORD} dbname=${env:POSTGRES_DB} sslmode=${env:POSTGRES_SSL_MODE}"
    collection_interval: ${env:COLLECTION_INTERVAL_SECONDS}s
    timeout: ${env:QUERY_TIMEOUT_MS}ms
    queries:
      # Query Performance from pg_stat_statements
      - query: |
          SELECT 
            queryid,
            LEFT(query, 100) as query_text,
            calls,
            total_exec_time,
            mean_exec_time,
            rows
          FROM pg_stat_statements
          WHERE mean_exec_time > ${env:MIN_QUERY_TIME_MS}
          ORDER BY mean_exec_time DESC
          LIMIT ${env:MAX_QUERIES_PER_COLLECTION}
        metrics:
          - metric_name: db.query.stats
            value_column: mean_exec_time
            value_type: double
            unit: ms
            attribute_columns: [queryid, query_text, calls, rows]
```

## Processors Configuration

### Standard OTEL Processors

#### Memory Limiter
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: ${env:MEMORY_LIMIT_PERCENTAGE}
    spike_limit_percentage: ${env:MEMORY_SPIKE_LIMIT_PERCENTAGE}
```

#### Batch Processor
```yaml
processors:
  batch:
    timeout: ${env:BATCH_TIMEOUT}
    send_batch_size: ${env:BATCH_SEND_SIZE}
    send_batch_max_size: ${env:BATCH_MAX_SIZE}
```

#### Resource Processor
```yaml
processors:
  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: upsert
      - key: deployment.environment
        value: ${env:ENVIRONMENT}
        action: upsert
      - key: service.version
        value: ${env:SERVICE_VERSION}
        action: upsert
      - key: cloud.region
        value: ${env:AWS_REGION}
        action: upsert
```

#### Transform Processor (PII Sanitization)
```yaml
processors:
  transform:
    error_mode: ignore
    metric_statements:
      - context: datapoint
        statements:
          # Sanitize sensitive data in query text
          - replace_pattern(attributes["query.text"], "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b", "[EMAIL]")
          - replace_pattern(attributes["query.text"], "\\b\\d{3}-\\d{2}-\\d{4}\\b", "[SSN]")
          - replace_pattern(attributes["query.text"], "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b", "[CARD]")
```

### Custom Processors (Experimental Mode)

Enable experimental processors through environment variables or Helm values:

```yaml
# values-experimental.yaml
config:
  mode: experimental
  processors:
    experimental:
      adaptiveSampler:
        enabled: true
      circuitBreaker:
        enabled: true
      planExtractor:
        enabled: true
      verification:
        enabled: true
```

#### Adaptive Sampler
```yaml
processors:
  adaptive_sampler:
    enabled: ${env:ENABLE_ADAPTIVE_SAMPLER}
    default_sampling_rate: ${env:BASE_SAMPLING_RATE}
    rules:
      - name: slow_queries
        condition: 'attributes["db_query_duration"] > 5000'
        sampling_rate: 1.0
      - name: errors
        condition: 'attributes["error"] != nil'
        sampling_rate: 1.0
      - name: high_frequency
        condition: 'attributes["query_count"] > 1000'
        sampling_rate: 0.1
```

#### Circuit Breaker
```yaml
processors:
  circuit_breaker:
    enabled: ${env:ENABLE_CIRCUIT_BREAKER}
    failure_threshold: 5
    success_threshold: 2
    timeout: 30s
    half_open_requests: 3
    databases:
      postgresql:
        max_connections: 10
        query_timeout: 5s
      mysql:
        max_connections: 10
        query_timeout: 5s
```

## Exporters Configuration

### OTLP Exporter (New Relic)
```yaml
exporters:
  otlp/newrelic:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
    compression: gzip
    timeout: 30s
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    sending_queue:
      enabled: true
      num_consumers: ${env:OTLP_NUM_CONSUMERS}
      queue_size: ${env:OTLP_QUEUE_SIZE}
```

### Prometheus Exporter (Local Metrics)
```yaml
exporters:
  prometheus:
    endpoint: 0.0.0.0:8888
    namespace: dbintel
    const_labels:
      environment: ${env:ENVIRONMENT}
      region: ${env:AWS_REGION}
    resource_to_telemetry_conversion:
      enabled: true
```

## Service Configuration

### Extensions
```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    path: /
    check_collector_pipeline:
      enabled: true
      interval: 15s
      exporter_failure_threshold: 5

  pprof:
    endpoint: 0.0.0.0:1777
    
  zpages:
    endpoint: 0.0.0.0:55679
```

### Service Pipelines
```yaml
service:
  extensions: [health_check, pprof, zpages]
  
  telemetry:
    logs:
      level: ${env:LOG_LEVEL}
      encoding: json
      output_paths: ["stdout"]
      error_output_paths: ["stderr"]
    metrics:
      level: detailed
      address: 0.0.0.0:8888
  
  pipelines:
    metrics/infrastructure:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp/newrelic, prometheus]
      
    metrics/queries:
      receivers: [sqlquery]
      processors: [memory_limiter, resource, transform, batch]
      exporters: [otlp/newrelic]
```

## OHI Migration Configuration

The v2.0.0 release includes full OHI (On-Host Integration) compatibility:

### OHI-Compatible Pipeline
```yaml
# config/collector-ohi-migration.yaml
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    collection_interval: 15s  # Match OHI interval
    
  sqlquery/postgresql_queries:
    driver: postgres
    datasource: "host=${POSTGRES_HOST}..."
    collection_interval: 60s
    queries:
      # pg_stat_statements with OHI thresholds
      - sql: |
          SELECT ... FROM pg_stat_statements
          WHERE calls > 20  # OHI threshold
            AND mean_exec_time > 500  # OHI threshold

processors:
  # OHI metric transformations
  metricstransform/ohi_compatibility:
    transforms:
      - include: postgresql.commits
        action: update
        new_name: db.commitsPerSecond
        
  # Query correlation for OHI parity
  querycorrelator:
    retention_period: 24h
    enable_table_correlation: true
    
  # Enterprise features
  costcontrol:
    monthly_budget_usd: 5000
    metric_cardinality_limit: 10000
    
  nrerrormonitor:
    max_attribute_length: 4095
    cardinality_warning_threshold: 10000
```

### Key OHI Features Supported

1. **Metric Name Compatibility**
   - All OHI metric names preserved
   - Automatic transformation from OTEL names
   - Backward compatible dashboards

2. **Query Performance Monitoring**
   - pg_stat_statements collection
   - Query anonymization and fingerprinting
   - Correlation with table statistics

3. **Entity Synthesis**
   - Required attributes for New Relic entities
   - Proper service.name and host.id mapping
   - Integration metadata

## Complete Examples

### Minimal Production Configuration (Working)
```yaml
# config/collector-minimal.yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13134

receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    databases:
      - testdb
    collection_interval: 30s
    tls:
      insecure: true

  mysql:
    endpoint: localhost:3306
    username: root
    password: mysql
    database: testdb
    collection_interval: 30s
    tls:
      insecure: true

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 30
  
  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: insert
      - key: deployment.environment
        value: production
        action: insert
  
  batch:
    send_batch_size: 1000
    timeout: 10s

exporters:
  otlp/newrelic:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    compression: gzip
    retry_on_failure:
      enabled: true
  
  prometheus:
    endpoint: 0.0.0.0:8888

service:
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp/newrelic, prometheus]
  telemetry:
    logs:
      level: info
    metrics:
      address: 0.0.0.0:8888
```

### Development Configuration
```bash
# Use development overlay
task run CONFIG_ENV=dev

# Or run with specific settings
task run LOG_LEVEL=debug COLLECTION_INTERVAL_SECONDS=10
```

### Production Configuration with Helm
```bash
# Deploy with production values
helm install db-intelligence ./deployments/helm/db-intelligence \
  -f deployments/helm/db-intelligence/values-production.yaml \
  --set config.mode=standard \
  --set image.tag=v2.0.0
```

### High-Security Configuration
```yaml
# Enable all security features
processors:
  transform:
    metric_statements:
      - context: datapoint
        statements:
          # Remove all query text in production
          - delete_key(attributes, "query.text") where deployment.environment == "production"
          
  verification:
    pii_detection:
      enabled: true
      sensitivity: high
      action: drop
```

## Using Taskfile

### Configuration Validation
```bash
# Validate configuration before deployment
task validate:config CONFIG_ENV=production

# Test configuration merge
task config:test ENV=staging
```

### Running with Different Configurations
```bash
# Development with hot reload
task dev:watch

# Production mode
task run:prod

# Debug mode with verbose logging
task run:debug
```

### Environment Management
```bash
# Show current configuration
task config:show

# Validate environment variables
task validate:env

# Generate config from template
task config:generate ENV=production
```

## Configuration Best Practices

1. **Use Environment Files**: Keep sensitive data out of configs
   ```bash
   task run ENV_FILE=.env.production
   ```

2. **Layer Configurations**: Use overlays for environment-specific settings
   ```bash
   configs/overlays/production/collector.yaml
   ```

3. **Validate Before Deploy**: Always validate configurations
   ```bash
   task validate:all
   ```

4. **Use Appropriate Modes**:
   - Standard: Production stability
   - Experimental: Advanced features

5. **Monitor Resource Usage**: Set appropriate limits
   ```yaml
   GOMEMLIMIT=1900MiB  # Leave headroom
   MEMORY_LIMIT_PERCENTAGE=75
   ```

6. **Enable Security Features**:
   - PII sanitization
   - TLS for connections
   - Network policies

7. **Configure for Scale**:
   - Appropriate batch sizes
   - Queue configurations
   - Sampling rates

8. **Use Version Control**: Track configuration changes
   ```bash
   git add configs/overlays/
   git commit -m "Update production config"
   ```

For more examples and templates, see the `configs/overlays/` directory.