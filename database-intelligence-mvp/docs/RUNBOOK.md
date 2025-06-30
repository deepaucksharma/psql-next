# Database Intelligence Collector - Operations Runbook

## Table of Contents

1. [Overview](#overview)
2. [Startup Procedures](#startup-procedures)
3. [Health Monitoring](#health-monitoring)
4. [Common Issues and Solutions](#common-issues-and-solutions)
5. [Performance Tuning](#performance-tuning)
6. [Emergency Procedures](#emergency-procedures)
7. [Maintenance Operations](#maintenance-operations)
8. [Troubleshooting Guide](#troubleshooting-guide)

## Overview

The Database Intelligence Collector is an OpenTelemetry-based monitoring solution for PostgreSQL and MySQL databases. This runbook provides operational procedures for managing the collector in production.

### Key Components

- **Data Collection**: PostgreSQL/MySQL receivers collecting metrics and query data
- **Processing Pipeline**: Custom processors for sampling, enrichment, and safety
- **Export**: OTLP export to New Relic with optional Prometheus metrics

### Critical Metrics to Monitor

- Collector memory usage (target: <512MB)
- Pipeline throughput (records/second)
- Circuit breaker status (per database)
- Sampling rates and dropped records
- Export success rates

## Startup Procedures

### Pre-flight Checks

```bash
# 1. Verify environment variables
./scripts/validate-env.sh

# 2. Test database connectivity
./scripts/test-connections.sh

# 3. Validate configuration
otelcol validate --config=config/collector-production.yaml

# 4. Check resource limits
ulimit -n  # Should be at least 65536
```

### Normal Startup

```bash
# 1. Start collector with production config
otelcol --config=config/collector-production.yaml \
        --set=service.telemetry.logs.level=info

# 2. Verify startup (wait 30 seconds)
curl -s http://localhost:13133/health/ready | jq .

# 3. Check metrics endpoint
curl -s http://localhost:8888/metrics | grep otelcol_
```

### Startup Validation

```bash
# Check all components are healthy
curl -s http://localhost:13133/health/ready | jq '.components | keys'

# Expected output:
# ["adaptive_sampler", "circuit_breaker", "planattributeextractor", "verification"]

# Verify data flow
curl -s http://localhost:8888/metrics | grep -E 'otelcol_processor_processed|otelcol_exporter_sent'
```

## Health Monitoring

### Health Check Endpoints

| Endpoint | Purpose | Expected Response |
|----------|---------|-------------------|
| `:13133/health/live` | Kubernetes liveness | 200 OK |
| `:13133/health/ready` | Kubernetes readiness | 200 OK if healthy |
| `:13133/health` | Detailed health status | JSON with component status |
| `:8888/metrics` | Prometheus metrics | Metrics in Prometheus format |
| `:55679/debug/tracez` | zPages trace debugging | HTML trace viewer |

### Key Health Indicators

```bash
# Check overall health
curl -s http://localhost:13133/health | jq '.healthy'

# Check memory usage
curl -s http://localhost:8888/metrics | grep 'process_resident_memory_bytes'

# Check pipeline status
curl -s http://localhost:13133/health | jq '.pipeline_status'

# Check circuit breaker states
curl -s http://localhost:13133/health | jq '.components.circuit_breaker.metrics'
```

### Monitoring Dashboard Queries

```promql
# Collector health score (0=unhealthy, 1=healthy)
up{job="otel-collector"}

# Memory usage percentage
100 * (process_resident_memory_bytes / otelcol_process_memory_limit)

# Processing rate
rate(otelcol_processor_accepted_spans[5m])

# Error rate
rate(otelcol_exporter_send_failed_spans[5m]) / rate(otelcol_exporter_sent_spans[5m])

# Circuit breaker trips
increase(otelcol_circuitbreaker_trips_total[1h])
```

## Common Issues and Solutions

### Issue: High Memory Usage

**Symptoms**: Memory usage >75% of limit, potential OOM kills

**Diagnosis**:
```bash
# Check current memory usage
curl -s http://localhost:8888/metrics | grep -E 'go_memstats_alloc_bytes|process_resident_memory'

# Check cache sizes
curl -s http://localhost:13133/health | jq '.components.adaptive_sampler.metrics.cache_size'

# Check batch processor queue
curl -s http://localhost:8888/metrics | grep 'otelcol_processor_batch_batch_size'
```

**Solutions**:
1. Reduce cache sizes:
   ```yaml
   adaptive_sampler:
     deduplication:
       cache_size: 5000  # Reduced from 10000
   ```

2. Adjust batch settings:
   ```yaml
   batch:
     send_batch_size: 500  # Reduced from 1000
     timeout: 5s          # Reduced from 10s
   ```

3. Enable memory limiter backpressure:
   ```yaml
   memory_limiter:
     limit_percentage: 65  # Reduced from 75
     spike_limit_percentage: 15  # Reduced from 20
   ```

### Issue: Circuit Breaker Frequently Opening

**Symptoms**: Databases showing as unhealthy, data collection stops

**Diagnosis**:
```bash
# Check circuit breaker status
curl -s http://localhost:13133/health | jq '.components.circuit_breaker.metrics.database_states'

# Check database response times
curl -s http://localhost:8888/metrics | grep 'otelcol_receiver_accepted_metric_points'

# Review logs for errors
journalctl -u otel-collector -n 100 | grep "circuit.*open"
```

**Solutions**:
1. Increase circuit breaker thresholds:
   ```yaml
   circuit_breaker:
     failure_threshold: 10  # Increased from 5
     open_state_timeout: 60s  # Increased from 30s
   ```

2. Reduce query load:
   ```yaml
   postgresql:
     collection_interval: 60s  # Increased from 30s
   ```

3. Check database performance:
   ```sql
   -- Check for long-running queries
   SELECT pid, now() - pg_stat_activity.query_start AS duration, query 
   FROM pg_stat_activity 
   WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';
   ```

### Issue: Export Failures to New Relic

**Symptoms**: Data not appearing in New Relic, export errors in logs

**Diagnosis**:
```bash
# Check export metrics
curl -s http://localhost:8888/metrics | grep -E 'otelcol_exporter_sent|otelcol_exporter_send_failed'

# Check for cardinality errors
journalctl -u otel-collector -n 100 | grep -i "cardinality\|NrIntegrationError"

# Verify API key
curl -X POST https://otlp.nr-data.net:4317/v1/metrics \
  -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  -H "Content-Type: application/x-protobuf"
```

**Solutions**:
1. Enable aggressive sampling:
   ```yaml
   adaptive_sampler:
     default_sample_rate: 0.01  # Very aggressive sampling
     rules:
       - name: high_cardinality_protection
         conditions:
           - attribute: unique_values
             operator: gt
             value: 1000
         sample_rate: 0.001
   ```

2. Add cardinality reduction:
   ```yaml
   transform:
     metric_statements:
       - context: metric
         statements:
           - truncate_all(attributes, 100)  # Limit attribute length
           - delete_key(attributes, "query_text") where attributes["sensitive"] == true
   ```

3. Enable export retry with backoff:
   ```yaml
   otlp/newrelic:
     retry_on_failure:
       enabled: true
       initial_interval: 30s
       max_interval: 300s
       max_elapsed_time: 900s
   ```

## Performance Tuning

### Baseline Performance Targets

| Metric | Target | Warning | Critical |
|--------|--------|---------|----------|
| Memory Usage | <256MB | >384MB | >450MB |
| CPU Usage | <20% | >50% | >80% |
| Processing Latency | <5ms | >10ms | >50ms |
| Export Queue Size | <1000 | >5000 | >10000 |
| Sampling Rate | Dynamic | <0.001 | 0 |

### Optimization Procedures

#### 1. Query Optimization

```yaml
# Reduce query frequency for non-critical databases
sqlquery/analytics:
  collection_interval: 300s  # 5 minutes for analytics DB
  
sqlquery/production:
  collection_interval: 30s   # 30 seconds for production
```

#### 2. Sampling Optimization

```yaml
adaptive_sampler:
  # Environment-specific sampling
  environment_overrides:
    production:
      slow_query_threshold_ms: 2000
      max_records_per_second: 500
      rules:
        - name: business_critical
          conditions:
            - attribute: db.name
              operator: eq
              value: "orders"
          sample_rate: 1.0  # Always sample critical DB
```

#### 3. Batch Processing Optimization

```yaml
batch:
  # Optimize for throughput vs latency
  send_batch_size: ${BATCH_SIZE:1000}
  send_batch_max_size: ${BATCH_MAX_SIZE:2000}
  timeout: ${BATCH_TIMEOUT:10s}
```

### Load Testing Procedure

```bash
# 1. Generate test load
./scripts/load-test.sh --duration=300 --rps=1000

# 2. Monitor during test
watch -n 1 'curl -s http://localhost:8888/metrics | grep -E "rate|latency|memory"'

# 3. Analyze results
./scripts/analyze-load-test.sh --output=load-test-report.html
```

## Emergency Procedures

### Emergency Stop

```bash
# Graceful shutdown (allows pipeline drain)
kill -TERM $(pgrep otelcol)

# Force stop if graceful fails after 30s
kill -KILL $(pgrep otelcol)
```

### Circuit Breaker Manual Control

```bash
# Open circuit breaker for specific database
curl -X POST http://localhost:13133/circuit_breaker/open \
  -d '{"database": "production_primary"}'

# Close circuit breaker
curl -X POST http://localhost:13133/circuit_breaker/close \
  -d '{"database": "production_primary"}'
```

### Data Pipeline Bypass

```yaml
# Emergency config with minimal processing
service:
  pipelines:
    metrics/emergency:
      receivers: [postgresql]
      processors: [memory_limiter, batch]  # Skip custom processors
      exporters: [otlp/newrelic]
```

### Rollback Procedure

```bash
# 1. Keep last known good config
cp config/collector-production.yaml config/collector-production.backup.yaml

# 2. Rollback if needed
otelcol --config=config/collector-production.backup.yaml

# 3. Verify rollback success
curl -s http://localhost:13133/health/ready
```

## Maintenance Operations

### Configuration Updates

```bash
# 1. Validate new configuration
otelcol validate --config=config/collector-new.yaml

# 2. Test in dry-run mode
otelcol --config=config/collector-new.yaml --dry-run

# 3. Apply configuration with zero downtime
./scripts/rolling-update.sh --config=config/collector-new.yaml
```

### Cache Maintenance

```bash
# Clear adaptive sampler cache
curl -X POST http://localhost:13133/cache/clear \
  -d '{"component": "adaptive_sampler"}'

# Clear plan extractor cache
curl -X POST http://localhost:13133/cache/clear \
  -d '{"component": "plan_extractor"}'
```

### Log Rotation

```bash
# Configure log rotation
cat > /etc/logrotate.d/otel-collector << EOF
/var/log/otel/collector.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0644 otel otel
    postrotate
        kill -USR1 $(pgrep otelcol) > /dev/null 2>&1 || true
    endscript
}
EOF
```

## Troubleshooting Guide

### Debug Mode Activation

```bash
# Enable debug logging
otelcol --config=config/collector-production.yaml \
        --set=service.telemetry.logs.level=debug

# Enable specific component debugging
export GODEBUG=gctrace=1  # GC debugging
export OTEL_RESOURCE_ATTRIBUTES="debug.mode=true"
```

### Common Debug Commands

```bash
# Check goroutine count (memory leaks)
curl -s http://localhost:1777/debug/pprof/goroutine?debug=1 | grep goroutine

# CPU profiling
curl -s http://localhost:1777/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Heap profiling
curl -s http://localhost:1777/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Trace analysis
curl -s http://localhost:55679/debug/tracez
```

### Log Analysis Patterns

```bash
# Find processing errors
grep -E "ERROR.*processor" /var/log/otel/collector.log

# Find export failures
grep -E "failed.*export|export.*failed" /var/log/otel/collector.log

# Find circuit breaker events
grep -E "circuit.*open|circuit.*close" /var/log/otel/collector.log

# Find memory pressure events
grep -E "memory.*limit|OOM|allocation.*failed" /var/log/otel/collector.log
```

### Recovery Procedures

#### From Memory Pressure
1. Reduce batch sizes
2. Clear caches
3. Increase memory limit
4. Enable aggressive sampling

#### From Export Failures
1. Check New Relic API status
2. Verify credentials
3. Reduce cardinality
4. Enable local buffering

#### From Database Overload
1. Increase collection intervals
2. Reduce query complexity
3. Enable circuit breakers
4. Add connection pooling

## Appendix: Quick Reference

### Environment Variables

```bash
# Required
NEW_RELIC_LICENSE_KEY    # New Relic API key
POSTGRES_HOST/PORT/USER  # PostgreSQL connection
MYSQL_HOST/PORT/USER     # MySQL connection

# Tuning
SLOW_QUERY_THRESHOLD     # Slow query threshold (ms)
MAX_RECORDS_PER_SECOND   # Rate limit
BATCH_SIZE               # Batch processor size
MEMORY_LIMIT_MB          # Memory limit

# Debugging
OTEL_LOG_LEVEL          # Log level (debug/info/warn/error)
GODEBUG                 # Go runtime debug flags
```

### Useful Aliases

```bash
alias otel-health='curl -s http://localhost:13133/health | jq .'
alias otel-metrics='curl -s http://localhost:8888/metrics'
alias otel-logs='journalctl -u otel-collector -f'
alias otel-restart='systemctl restart otel-collector'
```

### Support Contacts

- **On-Call**: Use PagerDuty integration
- **Slack**: #database-intelligence-alerts
- **Documentation**: https://docs.company.com/otel-collector
- **Source Code**: https://github.com/company/database-intelligence-mvp