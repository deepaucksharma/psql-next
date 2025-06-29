# Implementation Guide: Single-Server PostgreSQL Collector

## Overview

This guide describes the simplified implementation optimized for single-server deployment, following DDD principles and OpenTelemetry best practices.

## Architecture Summary

### Key Design Principles

1. **Separation of Concerns**: Receivers only ingest data, processors transform it, exporters send it
2. **Domain-Driven Design**: Clear bounded contexts for database, query, and telemetry domains
3. **Simplicity First**: In-memory state with optional file persistence
4. **Standard Components**: Use OpenTelemetry contrib components where possible

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ Collection  │  │  Database    │  │  Query Analysis  │  │
│  │ Service     │  │  Service     │  │  Service         │  │
│  └─────────────┘  └──────────────┘  └──────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                     Domain Layer                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Database   │  │    Query     │  │   Telemetry      │  │
│  │  Context    │  │   Context    │  │    Context       │  │
│  └─────────────┘  └──────────────┘  └──────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                 Infrastructure Layer                         │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  In-Memory  │  │  Simple      │  │   Event Bus      │  │
│  │ Repositories│  │ State Store  │  │                  │  │
│  └─────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Details

### 1. PostgreSQL Receiver (Refactored)

**Location**: `receivers/postgresqlquery/receiver_refactored.go`

**Key Features**:
- Pure data ingestion - no processing logic
- Collects pg_stat_statements, ASH data, and wait events
- Outputs raw metrics and logs for processing

**Example Usage**:
```go
// The receiver only collects data
func (r *postgresqlQueryRefactoredReceiver) collectStatStatements(
    ctx context.Context,
    db *sql.DB,
    resource pmetric.ResourceMetrics,
    logResource plog.ResourceLogs,
) {
    // Query pg_stat_statements
    // Create raw metrics - no sanitization or sampling
    // Let processors handle transformation
}
```

### 2. Circuit Breaker Processor (Generic)

**Location**: `processors/circuitbreaker/processor_refactored.go`

**Key Features**:
- Generic design - works with any backend, not just databases
- In-memory state with optional file persistence
- Configurable circuit identification

**Configuration**:
```yaml
processors:
  circuitbreaker/simple:
    circuit_id_attribute: "db.name"  # Or "service.name" for other backends
    failure_threshold: 5
    timeout: 30s
    persistence:
      enabled: true
      path: "/var/lib/otel/circuit_states.json"
```

### 3. Adaptive Sampler Processor

**Location**: `processors/adaptivesampler/processor_refactored.go`

**Key Features**:
- In-memory deduplication with LRU eviction
- Multiple sampling strategies
- No external dependencies

**Configuration**:
```yaml
processors:
  adaptivesampler/simple:
    max_memory_mb: 100
    dedup_cache_size: 10000
    strategies:
      - name: "errors"
        type: "always_sample"
        condition: "severity >= ERROR"
      - name: "slow_queries"
        type: "threshold"
        threshold_ms: 1000
```

### 4. Infrastructure Components

#### Simple State Store
**Location**: `infrastructure/simple_state_store.go`

- In-memory storage with LRU cache
- Optional file persistence on shutdown
- Thread-safe operations

#### Event Bus
**Location**: `infrastructure/event_bus.go`

- Simple in-memory pub/sub
- Synchronous event handling
- No external message broker needed

### 5. Domain Layer

The domain layer remains pure with no infrastructure concerns:

```go
// Domain entity example
type Query struct {
    id              QueryID
    text            string
    normalizedText  string
    database        DatabaseID
    metrics         PerformanceMetrics
    lastExecuted    time.Time
}

// Business logic in the entity
func (q *Query) IsSlowQuery(threshold Duration) bool {
    return q.metrics.AverageDuration.IsSlowerThan(threshold)
}
```

## Deployment Guide

### 1. Build the Collector

```bash
# Use OpenTelemetry Collector Builder
cat > otelcol-builder.yaml <<EOF
dist:
  name: postgresql-collector
  description: PostgreSQL collector for single server
  output_path: ./dist

receivers:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver v0.96.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.96.0
  - gomod: github.com/database-intelligence-mvp/receivers/postgresqlquery v1.0.0

processors:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/memorylimiterprocessor v0.96.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/batchprocessor v0.96.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.96.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.96.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.96.0
  - gomod: github.com/database-intelligence-mvp/processors/circuitbreaker v1.0.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.96.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.96.0
  - gomod: go.opentelemetry.io/collector/exporter/loggingexporter v0.96.0

extensions:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.96.0
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.96.0
EOF

# Build
ocb --config otelcol-builder.yaml
```

### 2. Configure Environment

```bash
# Create environment file
cat > /etc/otel/postgresql-collector.env <<EOF
# Database connection
POSTGRES_DSN=postgresql://monitor:password@localhost:5432/mydb?sslmode=require
DB_NAME=mydb

# OTLP endpoint
OTLP_ENDPOINT=https://otlp.nr-data.net:4317
NEW_RELIC_LICENSE_KEY=your-license-key

# Environment
ENVIRONMENT=production
HOSTNAME=$(hostname)
EOF
```

### 3. Run the Collector

```bash
# Create data directory
mkdir -p /var/lib/otel

# Run with simple configuration
./dist/postgresql-collector \
  --config /path/to/configs/collector-simple.yaml \
  --set=service.telemetry.logs.level=info
```

### 4. Systemd Service

```ini
[Unit]
Description=PostgreSQL OpenTelemetry Collector
After=network.target

[Service]
Type=simple
User=otel
Group=otel
EnvironmentFile=/etc/otel/postgresql-collector.env
ExecStart=/usr/local/bin/postgresql-collector --config /etc/otel/collector-simple.yaml
Restart=on-failure
RestartSec=10

# State persistence
ExecStartPre=/bin/mkdir -p /var/lib/otel
ExecStopPost=/bin/sync

[Install]
WantedBy=multi-user.target
```

## Monitoring and Troubleshooting

### Health Check
```bash
curl http://localhost:13133/health
```

### Metrics
The collector exposes its own metrics on port 8888:
```bash
curl http://localhost:8888/metrics
```

### Debug Logs
Enable debug logging:
```yaml
service:
  telemetry:
    logs:
      level: debug
```

### Common Issues

1. **Circuit Breaker Opening Frequently**
   - Check database health
   - Adjust failure threshold
   - Review timeout settings

2. **High Memory Usage**
   - Reduce dedup cache size
   - Lower batch sizes
   - Enable memory limiter

3. **Missing Metrics**
   - Verify database permissions
   - Check if extensions are installed
   - Review collector logs

## Performance Tuning

### For Small Deployments (< 10 databases)
```yaml
receivers:
  postgresqlquery/advanced:
    collection_interval: 30s
    max_queries_per_cycle: 500

processors:
  batch:
    timeout: 30s
    send_batch_size: 500
```

### For Large Deployments (> 50 databases)
```yaml
receivers:
  postgresqlquery/advanced:
    collection_interval: 60s
    max_queries_per_cycle: 100

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000
  
  memory_limiter:
    limit_mib: 1024
```

## Migration from Complex to Simple

If migrating from a complex multi-server setup:

1. **Remove External Dependencies**
   - Remove Redis configuration
   - Remove leader election
   - Remove distributed tracing

2. **Enable File Persistence**
   - Configure persistence paths
   - Set appropriate file permissions
   - Test recovery after restart

3. **Adjust Sampling Rates**
   - Start with conservative rates
   - Monitor resource usage
   - Gradually increase as needed

## Future Enhancements

When ready to scale beyond single server:

1. **Add External State Store**
   - Implement Redis StateStore
   - Enable distributed mode
   - Configure leader election

2. **Horizontal Scaling**
   - Deploy multiple collectors
   - Partition by database
   - Use consistent hashing

3. **Advanced Features**
   - ML-based anomaly detection
   - Predictive scaling
   - Custom dashboards

## Conclusion

This simplified implementation provides a robust, maintainable solution for single-server PostgreSQL monitoring while following best practices and leaving room for future growth.