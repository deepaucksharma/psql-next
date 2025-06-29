# Known Issues and Workarounds

This document lists known issues in the Database Intelligence MVP and their workarounds.

## Build Issues

### 1. Module Path Inconsistencies

**Issue**: Build fails with module not found errors
```
Error: failed to resolve go module github.com/newrelic/database-intelligence-mvp
```

**Root Cause**: Different module paths in various configuration files
- `go.mod`: `github.com/database-intelligence-mvp`
- `otelcol-builder.yaml`: `github.com/newrelic/database-intelligence-mvp`

**Workaround**:
```bash
# Fix all module references
sed -i 's|github.com/newrelic/database-intelligence-mvp|github.com/database-intelligence-mvp|g' otelcol-builder.yaml
sed -i 's|github.com/database-intelligence/|github.com/database-intelligence-mvp/|g' ocb-config.yaml

# Then rebuild
make clean
make build
```

**Permanent Fix**: Update all configuration files to use consistent module path

### 2. Custom OTLP Exporter TODOs

**Issue**: Custom OTLP exporter has unimplemented functions
```go
// TODO: implement conversion logic
panic("not implemented")
```

**Root Cause**: Incomplete implementation in `exporters/otlpexporter/`

**Workaround**: Use standard OTLP exporter instead
```yaml
exporters:
  otlp:  # Use standard OTLP, not otlp/custom
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
```

**Permanent Fix**: Either complete the custom exporter or remove it entirely

## Configuration Issues

### 3. Environment Variable Not Set

**Issue**: Collector fails to start with "environment variable has empty value"

**Root Cause**: Required environment variables not set

**Workaround**:
```bash
# Set all required variables
export POSTGRES_HOST=localhost
export POSTGRES_USER=monitor
export POSTGRES_PASSWORD=password
export NEW_RELIC_LICENSE_KEY=your_key

# Or use defaults in config
password: ${env:POSTGRES_PASSWORD:-default_password}
```

**Permanent Fix**: Add validation script that checks all required env vars

### 4. PostgreSQL pg_stat_statements Not Available

**Issue**: No query performance metrics collected

**Root Cause**: pg_stat_statements extension not enabled

**Workaround**:
```sql
-- As superuser
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
-- Restart PostgreSQL
-- Then:
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

**Permanent Fix**: Add pre-flight check in collector to verify extension

## Runtime Issues

### 5. High Memory Usage

**Issue**: Collector uses excessive memory and gets OOM killed

**Root Cause**: Large number of unique queries or high cardinality metrics

**Workaround**:
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 70  # Reduce from 80
    spike_limit_percentage: 20  # Reduce from 30
    
  database_intelligence/adaptivesampler:
    min_sampling_rate: 0.05  # Sample only 5%
    deduplication_window: 300s
    max_dedup_entries: 5000  # Reduce from 10000
```

**Permanent Fix**: Implement cardinality limiting in processors

### 6. Circuit Breaker Triggering Too Often

**Issue**: Circuit breaker opens frequently, stopping metric collection

**Root Cause**: Database queries timing out or high error rate

**Workaround**:
```yaml
processors:
  database_intelligence/circuitbreaker:
    failure_threshold: 10  # Increase from 5
    timeout: 60s  # Increase from 30s
    cooldown_period: 300s  # Increase from 60s
```

**Permanent Fix**: Add adaptive thresholds based on database load

### 7. PII Not Being Sanitized

**Issue**: Sensitive data appearing in metrics

**Root Cause**: PII patterns not matching actual data format

**Workaround**:
```yaml
processors:
  transform:
    metric_statements:
      - context: datapoint
        statements:
          # Add more aggressive patterns
          - replace_pattern(attributes["query.text"], "\\b\\d{4,}\\b", "[NUM]")
          - replace_pattern(attributes["query.text"], "'[^']*'", "'[REDACTED]'")
          
  database_intelligence/verification:
    pii_detection:
      sensitivity: high  # Increase sensitivity
      action: drop  # Drop instead of alert
```

**Permanent Fix**: Implement learning PII detection

## Performance Issues

### 8. Slow Metric Collection

**Issue**: Metrics take too long to collect, causing timeouts

**Root Cause**: Complex queries or large result sets

**Workaround**:
```yaml
receivers:
  sqlquery/postgresql:
    queries:
      - sql: |
          SELECT ... FROM pg_stat_statements
          WHERE calls > 100  -- Add filter
          AND mean_exec_time > 10  -- Only slow queries
          LIMIT 50  -- Reduce limit
        timeout: 30s  -- Increase timeout
```

**Permanent Fix**: Implement query optimization in sqlquery receiver

### 9. Verification Processor Overhead

**Issue**: Verification processor adds significant latency

**Root Cause**: Too frequent health checks and validations

**Workaround**:
```yaml
processors:
  database_intelligence/verification:
    health_checks:
      interval: 300s  # Increase from 60s
    metric_quality:
      enabled: false  # Disable if not critical
    auto_tuning:
      enabled: false  # Disable to reduce overhead
```

**Permanent Fix**: Implement async verification

## Deployment Issues

### 10. Container Image Size

**Issue**: Docker image is very large (>1GB)

**Root Cause**: Including all OTEL components even if unused

**Workaround**: Use multi-stage build
```dockerfile
FROM golang:1.21-alpine AS builder
# Build only needed components

FROM alpine:3.19
# Copy only binary
```

**Permanent Fix**: Create minimal custom distribution

### 11. State File Permissions

**Issue**: Adaptive sampler can't write state file

**Root Cause**: Incorrect file permissions in container

**Workaround**:
```yaml
# In Kubernetes
securityContext:
  fsGroup: 2000
  runAsUser: 1000
  
# In Docker
user: "1000:1000"
volumes:
  - ./state:/var/lib/otel:rw
```

**Permanent Fix**: Use external state store (Redis)

## Integration Issues

### 12. New Relic Data Not Appearing

**Issue**: Metrics sent but not visible in New Relic

**Root Cause**: Incorrect endpoint or missing attributes

**Workaround**:
```yaml
exporters:
  otlp/newrelic:
    endpoint: https://otlp.nr-data.net:4317  # Ensure HTTPS and correct port
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}  # Verify key is correct
    retry_on_failure:
      enabled: true
      max_elapsed_time: 300s
      
processors:
  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: insert  # Required attribute
```

**Permanent Fix**: Add New Relic validation in exporter

### 13. Prometheus Metrics Format

**Issue**: Prometheus can't scrape metrics endpoint

**Root Cause**: Incorrect metric naming or format

**Workaround**:
```yaml
exporters:
  prometheus:
    namespace: db_intel  # Use valid namespace
    const_labels:
      job: database_intelligence
    resource_to_telemetry_conversion:
      enabled: true
```

**Permanent Fix**: Validate Prometheus naming conventions

## Scaling Issues

### 14. Horizontal Scaling Limitations

**Issue**: Can't run multiple collector instances

**Root Cause**: File-based state in adaptive sampler

**Workaround**: Shard by database
```yaml
# Instance 1
receivers:
  postgresql:
    databases: [db1, db2]
    
# Instance 2  
receivers:
  postgresql:
    databases: [db3, db4]
```

**Permanent Fix**: Implement Redis-based shared state

### 15. Memory Leak in Long-Running Instances

**Issue**: Memory usage grows over time

**Root Cause**: State accumulation in processors

**Workaround**: Restart periodically
```yaml
# Kubernetes
spec:
  template:
    spec:
      containers:
      - name: collector
        livenessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - '[ $(cat /proc/1/status | grep VmRSS | awk '\''{print $2}'\'') -lt 1048576 ]'
          periodSeconds: 300
          failureThreshold: 1
```

**Permanent Fix**: Implement proper state cleanup

## Monitoring Issues

### 16. Missing Internal Metrics

**Issue**: Can't see processor-specific metrics

**Root Cause**: Internal telemetry not configured

**Workaround**:
```yaml
service:
  telemetry:
    metrics:
      level: detailed
      address: 0.0.0.0:8889  # Different from Prometheus port
      
# Then access at http://localhost:8889/metrics
```

**Permanent Fix**: Expose all processor metrics by default

## Summary

Most issues have straightforward workarounds. The most critical issues to address are:

1. **Module path inconsistencies** - Prevents building
2. **State management** - Limits horizontal scaling  
3. **Memory management** - Can cause production outages

For production deployments, apply all relevant workarounds and monitor for these issues.