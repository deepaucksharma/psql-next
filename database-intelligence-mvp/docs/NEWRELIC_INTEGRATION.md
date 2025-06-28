# New Relic Integration Guide for Database Intelligence MVP

## Overview

This guide documents the enhanced integration between the Database Intelligence MVP and New Relic, incorporating best practices from OpenTelemetry-New Relic integration patterns. The implementation addresses critical concerns including silent failures, cardinality management, entity synthesis, and performance optimization.

## Table of Contents

1. [Critical Concepts](#critical-concepts)
2. [Configuration](#configuration)
3. [Monitoring Silent Failures](#monitoring-silent-failures)
4. [Cardinality Management](#cardinality-management)
5. [Entity Synthesis](#entity-synthesis)
6. [Circuit Breaker Integration](#circuit-breaker-integration)
7. [Performance Optimization](#performance-optimization)
8. [Troubleshooting](#troubleshooting)
9. [Production Deployment](#production-deployment)

## Critical Concepts

### The Silent Failure Problem

**Key Insight**: Successful OTLP exports (202 Accepted) do not guarantee data ingestion in New Relic.

```mermaid
sequenceDiagram
    participant Collector
    participant NR_Endpoint as New Relic OTLP
    participant NR_Pipeline as NR Processing
    participant NR_Storage as NR Storage
    
    Collector->>NR_Endpoint: POST /v1/logs
    NR_Endpoint-->>Collector: 202 Accepted ✓
    Note over Collector: Logs show "success"
    
    NR_Endpoint->>NR_Pipeline: Async processing
    
    alt Validation Failure
        NR_Pipeline->>NR_Pipeline: Schema/Cardinality violation
        NR_Pipeline->>NR_Storage: Write NrIntegrationError
        Note over NR_Storage: Data dropped silently!
    else Success
        NR_Pipeline->>NR_Storage: Store data
    end
```

### Monitoring Requirements

You **MUST** monitor `NrIntegrationError` events:

```sql
SELECT count(*) 
FROM NrIntegrationError 
WHERE newRelicFeature = 'Metrics'
FACET message 
SINCE 1 hour ago
```

## Configuration

### Enhanced Collector Configuration

The `collector-newrelic-optimized.yaml` includes:

1. **Protocol Optimization**: HTTP/protobuf over gRPC
2. **Compression**: zstd for JSON-heavy database plans
3. **Batching**: Optimized for New Relic's 10,000 limit
4. **Entity Synthesis**: Required attributes for correlation
5. **Cardinality Control**: Multi-stage reduction pipeline

```yaml
exporters:
  otlp/newrelic:
    endpoint: "${env:OTLP_ENDPOINT:-https://otlp.nr-data.net:4318}"
    protocol: "http/protobuf"  # Better than gRPC for databases
    compression: zstd           # Better for JSON plans
    
    # Critical: timeout < batch timeout
    timeout: 25s
    
    # Queue configuration for bursts
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
      storage: file_storage  # Persist during restarts
```

### Environment Variables

```bash
# Required
NEW_RELIC_LICENSE_KEY=your-license-key
OTLP_ENDPOINT=https://otlp.nr-data.net:4318

# Database connections (read replicas only)
PG_REPLICA_DSN=postgres://monitor:pass@replica:5432/db?sslmode=require

# Performance tuning
COLLECTION_INTERVAL_SECONDS=60
QUERY_TIMEOUT_MS=3000
SAMPLING_PERCENTAGE=25
```

## Monitoring Silent Failures

### NrIntegrationError Monitoring

Create alerts for integration errors:

```sql
-- Alert when errors detected
SELECT count(*) 
FROM NrIntegrationError 
WHERE newRelicFeature = 'Metrics' 
  AND (message LIKE '%database%' OR message LIKE '%otel%')
SINCE 5 minutes ago
-- Alert threshold: > 10 errors
```

### Validation Dashboard

Import the provided `dashboard-config.json` to monitor:
- Integration errors
- Cardinality warnings
- Entity synthesis success
- Circuit breaker status
- Sampling effectiveness

## Cardinality Management

### The Challenge

Database monitoring creates extreme cardinality:
- Infinite query text variations
- Multiple databases × schemas × tables
- User × application × query combinations

### Multi-Stage Reduction Pipeline

1. **Query Normalization**
   ```yaml
   transform/query_normalization:
     # Replace numbers with placeholders
     - set(attributes["db.query.normalized"], 
         RegexReplace(body["query_text"], "\\b\\d+\\b", "?"))
   ```

2. **Fingerprinting**
   ```yaml
   # Create stable fingerprint
   - set(attributes["db.query.fingerprint"], 
       SHA256(attributes["db.query.normalized"]))
   ```

3. **Attribute Removal**
   ```yaml
   attributes/cleanup:
     actions:
       - key: backend_pid
         action: delete
       - key: client_addr
         action: hash
   ```

4. **Grouping**
   ```yaml
   groupbyattrs:
     keys:
       - database_name
       - db.query.fingerprint
       - db.query.complexity
   ```

### Monitoring Cardinality

```sql
-- Track compression effectiveness
SELECT 
  uniqueCount(db.query.fingerprint) as 'Patterns',
  uniqueCount(query_text) as 'Raw Queries',
  percentage(uniqueCount(db.query.fingerprint), 
    uniqueCount(query_text)) as 'Compression %'
FROM Log
FACET database_name
SINCE 1 hour ago
```

## Entity Synthesis

### Required Attributes

For proper entity creation and correlation:

```yaml
resource/entity_synthesis:
  attributes:
    # Database entity
    - key: "entity.guid"
      value: "DATABASE|${env:DEPLOYMENT_ENV}|${database_name}"
      action: upsert
    
    - key: "entity.type"
      value: "DATABASE"
      action: insert
    
    # Service correlation
    - key: "service.name"
      value: "database-intelligence-postgresql"
      action: insert
    
    # Infrastructure correlation
    - key: "host.id"
      value: "${env:HOSTNAME}"
      action: insert
    
    # New Relic requirements
    - key: "instrumentation.provider"
      value: "opentelemetry"
      action: insert
```

### Validation

```sql
-- Verify entity creation
SELECT uniques(entity.guid) as 'Database Entities',
       latest(entity.type),
       latest(entity.name)
FROM Log
WHERE entity.type = 'DATABASE'
FACET database_name
SINCE 1 hour ago
```

## Circuit Breaker Integration

### Per-Database Circuit Breakers

The enhanced circuit breaker tracks state per database:

```go
type databaseCircuitState struct {
    state        State
    failureCount int
    errorRate    float64
    avgDuration  time.Duration
}
```

### New Relic Error Detection

The circuit breaker detects New Relic-specific errors:

```go
nrErrorPatterns := []string{
    "cardinality",
    "NrIntegrationError",
    "api-key",
    "rate limit",
    "quota exceeded",
    "unique time series",
}
```

### Monitoring

```sql
-- Database circuit status
SELECT 
  latest(cb.state) as 'State',
  sum(cb.opened_count) as 'Opens',
  average(cb.error_rate) as 'Error Rate'
FROM Log
WHERE cb.state IS NOT NULL
FACET database_name
SINCE 1 hour ago
```

## Performance Optimization

### Batch Size Optimization

Based on New Relic's limits:

```yaml
batch/newrelic_optimized:
  timeout: 30s
  send_batch_size: 5000      # 50% of limit
  send_batch_max_size: 9000  # 90% of limit
```

### Adaptive Sampling

Priority-based sampling reduces data volume:

```yaml
# Always sample slow queries
- set(attributes["sampling.priority"], 100)
    where body["duration_ms"] > 1000

# Sample 10% of normal queries  
- set(attributes["sampling.priority"], 10)
    where body["duration_ms"] <= 1000
```

### Memory Management

```yaml
memory_limiter:
  check_interval: 1s
  limit_percentage: 75
  spike_limit_percentage: 20
  ballast_size_mib: 256  # Prevent GC thrashing
```

## Troubleshooting

### Common Issues

1. **Data Not Appearing**
   ```sql
   -- Check for errors first
   SELECT count(*) FROM NrIntegrationError 
   WHERE newRelicFeature = 'Metrics' SINCE 1 hour ago
   
   -- Then verify data arrival
   SELECT count(*) FROM Log
   WHERE collector.name = 'database-intelligence'
   SINCE 10 minutes ago
   ```

2. **High Cardinality Rejection**
   - Check `NrIntegrationError` for "cardinality" messages
   - Review unique pattern count
   - Increase sampling or improve normalization

3. **Entity Correlation Missing**
   ```sql
   -- Find missing correlations
   SELECT count(*) as 'Missing'
   FROM Log
   WHERE entity.guid IS NULL
     AND database_name IS NOT NULL
   FACET database_name
   ```

### Debug Mode

Enable debug logging:

```yaml
service:
  telemetry:
    logs:
      level: debug
      encoding: json
```

## Production Deployment

### Kubernetes Configuration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
data:
  collector.yaml: |
    # Use collector-newrelic-optimized.yaml content
    
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: database-intelligence-collector
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: otel-collector
        image: otel/opentelemetry-collector-contrib:0.89.0
        env:
        - name: GOMEMLIMIT
          value: "460MiB"  # 90% of limit
        resources:
          limits:
            memory: 512Mi
            cpu: 500m
          requests:
            memory: 256Mi
            cpu: 100m
```

### High Availability Setup

Deploy collectors in multiple availability zones:

```yaml
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: app
          operator: In
          values:
          - database-intelligence
      topologyKey: "kubernetes.io/hostname"
```

### Monitoring Checklist

- [ ] NrIntegrationError alerts configured
- [ ] Cardinality monitoring dashboard imported
- [ ] Circuit breaker alerts set up
- [ ] Entity correlation validated
- [ ] Sampling effectiveness reviewed
- [ ] Export success rate > 99%

## Best Practices Summary

1. **Always monitor NrIntegrationError** - Your only visibility into failures
2. **Implement aggressive cardinality control** - Database queries explode cardinality
3. **Use per-database circuit breakers** - Isolate failures
4. **Configure entity synthesis correctly** - Required for correlation
5. **Use HTTP/protobuf protocol** - Better for large payloads
6. **Set appropriate batch sizes** - Respect New Relic limits
7. **Implement priority sampling** - Focus on slow queries
8. **Monitor continuously** - Use provided NRQL queries

## Next Steps

1. Deploy the enhanced configuration
2. Import monitoring dashboards
3. Set up alerts based on NRQL queries
4. Validate entity synthesis
5. Monitor cardinality trends
6. Tune sampling rates based on volume

For additional support, refer to the [OpenTelemetry-New Relic detailed guide](../../../otel-newrelic-detailed-guide.md).