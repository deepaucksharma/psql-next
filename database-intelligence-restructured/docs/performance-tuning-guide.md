# Performance Tuning Guide for Database Intelligence OTel

## Overview

This guide provides comprehensive performance tuning strategies for the Database Intelligence OpenTelemetry Collector deployment, covering both config-only and enhanced modes.

## Performance Baseline Metrics

### Key Performance Indicators (KPIs)

| Metric | Target | Warning | Critical |
|--------|--------|---------|----------|
| Collector CPU Usage | < 20% | > 50% | > 80% |
| Collector Memory Usage | < 1GB | > 2GB | > 4GB |
| Database Query Overhead | < 1% | > 3% | > 5% |
| Metric Ingestion Latency | < 10s | > 30s | > 60s |
| Data Points Per Minute | < 1M | > 5M | > 10M |

## Collector Performance Tuning

### 1. Memory Optimization

#### Configure Memory Limiter
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 2048        # Hard limit
    spike_limit_mib: 512   # Spike allowance
    
  # Use memory ballast for stable GC
extensions:
  memory_ballast:
    size_mib: 512  # 25% of limit_mib
```

#### Optimize Batch Processor
```yaml
processors:
  batch:
    timeout: 5s              # Reduce from 10s
    send_batch_size: 2000    # Increase batch size
    send_batch_max_size: 4000
```

### 2. CPU Optimization

#### Parallel Processing
```yaml
service:
  telemetry:
    resource:
      # Use all available cores
      "process.runtime.go.max_procs": ${GOMAXPROCS}
```

#### Optimize Collection Intervals
```yaml
receivers:
  # Tier 1: Critical metrics (frequent)
  postgresql/critical:
    collection_interval: 30s
    metrics:
      postgresql.connections.*:
        enabled: true
        
  # Tier 2: Performance metrics (normal)
  postgresql/performance:
    collection_interval: 60s
    metrics:
      postgresql.query.*:
        enabled: true
        
  # Tier 3: Analytical metrics (infrequent)
  postgresql/analytical:
    collection_interval: 5m
    metrics:
      postgresql.table.*:
        enabled: true
```

### 3. Network Optimization

#### Compression and Batching
```yaml
exporters:
  otlp:
    compression: gzip
    sending_queue:
      enabled: true
      num_consumers: 10      # Parallel senders
      queue_size: 5000
    retry_on_failure:
      enabled: true
      initial_interval: 1s   # Fast initial retry
      max_interval: 30s
      max_elapsed_time: 300s
```

## Database Query Optimization

### 1. Query Tuning

#### Optimize SQL Queries
```yaml
sqlquery:
  # Use prepared statements
  queries:
    - sql: |
        -- Use CTEs for efficiency
        WITH active_connections AS (
          SELECT state, COUNT(*) as count
          FROM pg_stat_activity
          WHERE pid != pg_backend_pid()
          GROUP BY state
        )
        SELECT 
          state,
          count,
          count::float / current_setting('max_connections')::int * 100 as percent
        FROM active_connections
```

#### Index Optimization
```sql
-- Create indexes for monitoring queries
CREATE INDEX CONCURRENTLY idx_pg_stat_activity_state 
ON pg_catalog.pg_stat_activity(state);

-- For pg_stat_statements
CREATE INDEX idx_pg_stat_statements_queryid 
ON pg_stat_statements(queryid);
```

### 2. Connection Pooling

#### PgBouncer Configuration
```ini
# pgbouncer.ini
[databases]
monitoring = host=localhost port=5432 dbname=postgres

[pgbouncer]
pool_mode = session
max_client_conn = 100
default_pool_size = 25
reserve_pool_size = 5
reserve_pool_timeout = 3
server_lifetime = 3600
```

## Metric Cardinality Management

### 1. Attribute Filtering

```yaml
processors:
  attributes:
    # Remove high-cardinality attributes
    actions:
      - key: query.text
        action: delete
      - key: client.address
        action: hash
      - key: user.name
        pattern: ^app_.*
        action: keep
```

### 2. Metric Aggregation

```yaml
processors:
  metricstransform:
    transforms:
      # Aggregate by schema instead of table
      - include: postgresql.table.*
        match_type: regexp
        action: aggregate
        aggregation_type: sum
        dimensions: [schema_name]
```

### 3. Sampling Strategies

```yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 10  # Sample 10% of traces
    
  adaptive_sampler:
    rules:
      - metric_pattern: "postgresql.query.*"
        conditions:
          - duration > 1000  # Always sample slow queries
            sample_rate: 1.0
          - duration < 100   # Sample 1% of fast queries
            sample_rate: 0.01
```

## Enhanced Mode Performance

### 1. Circuit Breaker Tuning

```yaml
processors:
  circuitbreaker:
    # Aggressive thresholds for production
    thresholds:
      database_cpu_percent: 70    # Lower threshold
      collector_cpu_percent: 60   
      query_time_ms: 500         # Faster queries only
      memory_usage_percent: 70
    
    # Faster recovery
    timeout: 30s
    half_open_max_requests: 20
```

### 2. ASH Receiver Optimization

```yaml
receivers:
  ash:
    sampling:
      interval: 2s           # Increase from 1s
      batch_size: 200        # Larger batches
      compression: true
      
    retention:
      in_memory: 30m         # Reduce from 1h
      circular_buffer: true
      
    features:
      session_sampling:
        sample_percent: 50   # Sample 50% of sessions
```

### 3. Plan Cache Optimization

```yaml
processors:
  planattributeextractor:
    plan_cache:
      size: 5000            # Larger cache
      ttl: 1h               # Longer TTL
      eviction: lru
    
    # Reduce extraction overhead
    extract_fields:
      - plan_hash          # Essential only
      - total_cost
```

## Resource Allocation

### 1. Container Resources

```yaml
# Kubernetes
resources:
  requests:
    memory: "1Gi"
    cpu: "1"
  limits:
    memory: "4Gi"
    cpu: "4"
```

### 2. JVM-Style Tuning (Go Runtime)

```yaml
# Environment variables
GOGC: 100              # Default GC target
GOMEMLIMIT: 3750MiB    # 90% of container limit
GOMAXPROCS: 4          # Match CPU limit
```

## Monitoring Collector Performance

### 1. Self-Telemetry

```yaml
service:
  telemetry:
    metrics:
      level: detailed
      address: 0.0.0.0:8888
      
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: 'otelcol'
          scrape_interval: 30s
          static_configs:
            - targets: ['localhost:8888']
```

### 2. Key Metrics to Monitor

```sql
-- New Relic queries for collector health
-- CPU usage
SELECT average(`otelcol_process_cpu_seconds`) 
FROM Metric 
WHERE service.name = 'otel-collector'

-- Memory usage
SELECT latest(`otelcol_process_memory_rss`) / 1024 / 1024 as 'Memory (MB)'
FROM Metric

-- Processing rate
SELECT rate(sum(`otelcol_processor_batch_batch_send_size`), 1 minute) 
FROM Metric
```

## Performance Testing

### 1. Load Testing Script

```bash
#!/bin/bash
# generate-load.sh

# Simulate high query load
for i in {1..100}; do
  psql -c "SELECT pg_sleep(0.1); SELECT * FROM pg_stat_activity;" &
done

# Monitor collector metrics
while true; do
  curl -s localhost:8888/metrics | grep -E "(cpu|memory|batch)"
  sleep 5
done
```

### 2. Stress Testing Configuration

```yaml
# stress-test-config.yaml
receivers:
  synthetic:
    # Generate synthetic metrics for testing
    interval: 100ms
    metrics_per_interval: 10000
    
processors:
  batch:
    timeout: 1s
    send_batch_size: 10000
```

## Optimization Checklist

### Pre-Production
- [ ] Baseline performance metrics established
- [ ] Database indexes created for monitoring queries
- [ ] Connection pooling configured
- [ ] Resource limits set appropriately
- [ ] Cardinality limits configured

### Production Deployment
- [ ] Tiered collection intervals implemented
- [ ] Sampling rules configured
- [ ] Circuit breakers enabled
- [ ] Compression enabled
- [ ] Batch sizes optimized

### Ongoing Optimization
- [ ] Monitor collector self-metrics
- [ ] Review high-cardinality metrics weekly
- [ ] Adjust sampling rates based on load
- [ ] Update collection intervals seasonally
- [ ] Profile collector during peak times

## Troubleshooting Performance Issues

### High CPU Usage
1. Increase collection intervals
2. Enable sampling
3. Reduce receiver concurrency
4. Check for expensive SQL queries

### High Memory Usage
1. Reduce batch sizes
2. Lower queue sizes
3. Enable memory limiter
4. Check for cardinality explosions

### Slow Metric Delivery
1. Increase batch timeout
2. Add more export workers
3. Check network latency
4. Verify New Relic isn't rate limiting

## Summary

Performance tuning is an iterative process. Start with conservative settings and gradually optimize based on observed behavior. Monitor both the collector and database impact continuously, and adjust configurations to maintain the balance between observability coverage and system performance.