# Troubleshooting Guide

This guide covers common issues and solutions for both production and experimental deployments.

## Quick Diagnostics

### Check Component Health

```bash
# Production collector
curl -s http://localhost:13133/ | jq .

# Experimental collector
curl -s http://localhost:13134/ | jq .

# Check metrics endpoint
curl -s http://localhost:8888/metrics | grep otelcol_receiver_accepted
```

### View Logs

```bash
# Docker deployments
docker logs db-intel-primary         # Production
docker logs db-intel-experimental    # Experimental

# Kubernetes deployments
kubectl logs -n db-intelligence deployment/db-intelligence-collector
```

## Common Issues

### 1. Collector Won't Start

#### Symptom
```
Error: failed to get config: cannot unmarshal the configuration
```

#### Causes and Solutions

**Invalid YAML syntax**
```bash
# Validate configuration
docker run --rm -v $(pwd)/config:/config \
  otel/opentelemetry-collector-contrib:0.88.0 \
  validate --config=/config/collector.yaml
```

**Missing environment variables**
```bash
# Check required variables
env | grep -E "(PG_REPLICA_DSN|NEW_RELIC_LICENSE_KEY|OTLP_ENDPOINT)"

# Set missing variables
export PG_REPLICA_DSN="postgres://user:pass@host:5432/db"
export NEW_RELIC_LICENSE_KEY="your-key-here"
```

**Custom components not found (experimental)**
```bash
# Ensure custom collector is built
./scripts/build-custom-collector.sh --with-docker
```

### 2. No Data Being Collected

#### Symptom
- Metrics show 0 accepted records
- No logs being exported

#### Debugging Steps

1. **Check database connectivity**
```bash
# Test PostgreSQL connection
psql "$PG_REPLICA_DSN" -c "SELECT 1"

# Test from within container
docker exec db-intel-primary psql "$PG_REPLICA_DSN" -c "SELECT 1"
```

2. **Verify permissions**
```sql
-- Check pg_stat_statements is accessible
SELECT count(*) FROM pg_stat_statements;

-- Check required permissions
SELECT has_table_privilege('pg_stat_statements', 'SELECT');
```

3. **Check leader election (HA mode)**
```bash
# Only one instance should be collecting
kubectl get lease -n db-intelligence db-intelligence-leader -o yaml
```

4. **Verify queries are returning data**
```bash
# Run the collection query manually
psql "$PG_REPLICA_DSN" -f config/queries/postgresql-metadata.sql
```

### 3. Circuit Breaker Keeps Opening

#### Symptom
```
level=warn msg="Circuit breaker opened" database=primary reason="high error rate"
```

#### Solutions

1. **Increase thresholds**
```yaml
processors:
  circuitbreaker:
    failure_threshold: 10  # Increase from 5
    success_threshold: 3   # Increase from 2
    databases:
      default:
        max_error_rate: 0.2  # Increase from 0.1
```

2. **Check database health**
```sql
-- Check active connections
SELECT count(*) FROM pg_stat_activity;

-- Check for long-running queries
SELECT pid, now() - query_start as duration, query 
FROM pg_stat_activity 
WHERE state != 'idle' 
ORDER BY duration DESC;
```

3. **Reduce collection frequency**
```yaml
receivers:
  postgresqlquery:
    collection:
      interval: 120s  # Increase from 60s
```

### 4. High Memory Usage

#### Symptom
- Container OOMKilled
- Memory limit exceeded warnings

#### Solutions

1. **Increase memory limits**
```yaml
# Docker
mem_limit: 4g

# Kubernetes
resources:
  limits:
    memory: 4Gi
```

2. **Tune memory limiter**
```yaml
processors:
  memory_limiter:
    check_interval: 2s
    limit_mib: 3072     # 75% of container limit
    spike_limit_mib: 512
```

3. **Reduce buffer sizes**
```yaml
receivers:
  postgresqlquery:
    ash_sampling:
      buffer_size: 1800  # Reduce from 3600
```

### 5. Adaptive Sampler Not Adjusting

#### Symptom
- Sampling rate stays at initial value
- No rate adjustments in logs

#### Debugging

1. **Check state persistence**
```bash
# For memory state (single instance)
curl -s http://localhost:8888/metrics | grep adaptivesampler_state

# For Redis state
redis-cli -h localhost -p 6380 keys "adaptivesampler:*"
```

2. **Verify strategy triggers**
```yaml
processors:
  adaptivesampler:
    strategies:
      - type: "query_cost"
        high_cost_threshold_ms: 100  # Lower if needed
```

3. **Enable debug logging**
```yaml
service:
  telemetry:
    logs:
      level: debug
      encoding: json
```

### 6. ASH Sampling Missing Data

#### Symptom
- No ASH samples in output
- `ash_sample` field empty

#### Solutions

1. **Verify pg_wait_sampling extension**
```sql
-- Check extension
SELECT * FROM pg_extension WHERE extname = 'pg_wait_sampling';

-- Install if missing
CREATE EXTENSION pg_wait_sampling;
```

2. **Check sampling configuration**
```yaml
receivers:
  postgresqlquery:
    ash_sampling:
      enabled: true
      interval: 1s  # Must be >= 1s
```

### 7. Plan Collection Not Working

#### Symptom
- `plan_metadata` shows `plan_available: false`
- No execution plans in output

#### Current Status
Plan collection requires the `pg_querylens` extension which is not yet available. This is expected behavior in the current version.

#### Workaround
Use query metadata and pg_stat_statements statistics for performance analysis until plan collection is enabled.

## Performance Tuning

### Reduce Database Load

```yaml
# Increase collection intervals
receivers:
  postgresqlquery:
    collection:
      interval: 300s  # 5 minutes
    ash_sampling:
      interval: 5s    # Reduce ASH frequency

# Limit concurrent connections
    connection:
      max_open: 2
      max_idle: 1
```

### Optimize Network Usage

```yaml
# Increase batch sizes
processors:
  batch:
    timeout: 60s
    send_batch_size: 200
    send_batch_max_size: 500

# Enable compression
exporters:
  otlp/newrelic:
    compression: gzip
```

### Memory Optimization

```yaml
# Use environment variables
environment:
  GOGC: 50           # More aggressive GC
  GOMEMLIMIT: 1GiB   # Hard memory limit
```

## Debug Mode

### Enable Detailed Logging

```yaml
# In collector config
service:
  telemetry:
    logs:
      level: debug
      encoding: json
      output_paths: ["stdout", "/tmp/collector.log"]
```

### Use Debug Endpoints

```bash
# ZPages (experimental only)
open http://localhost:55680/debug/tracez
open http://localhost:55680/debug/pipelinez

# pprof (experimental only)
go tool pprof http://localhost:6061/debug/pprof/heap
go tool pprof http://localhost:6061/debug/pprof/profile
```

### Dry Run Mode

```bash
# Test configuration without starting
./dist/db-intelligence-custom --config=config/collector.yaml --dry-run
```

## Getting Help

### Collect Diagnostics

```bash
# Create diagnostics bundle
cat > collect-diagnostics.sh << 'EOF'
#!/bin/bash
DIAG_DIR="diagnostics-$(date +%Y%m%d-%H%M%S)"
mkdir -p $DIAG_DIR

# Collect logs
docker logs db-intel-primary > $DIAG_DIR/collector.log 2>&1
docker logs db-intel-experimental > $DIAG_DIR/experimental.log 2>&1

# Collect metrics
curl -s http://localhost:8888/metrics > $DIAG_DIR/metrics.txt

# Collect config (sanitized)
cp config/collector.yaml $DIAG_DIR/
sed -i 's/api-key: .*/api-key: REDACTED/' $DIAG_DIR/*.yaml

# System info
docker version > $DIAG_DIR/docker-version.txt
docker-compose version >> $DIAG_DIR/docker-version.txt

# Create archive
tar -czf $DIAG_DIR.tar.gz $DIAG_DIR
echo "Diagnostics collected: $DIAG_DIR.tar.gz"
EOF

chmod +x collect-diagnostics.sh
./collect-diagnostics.sh
```

### Support Channels

1. **GitHub Issues**: For bugs and feature requests
2. **GitHub Discussions**: For questions and community support
3. **New Relic Support**: For New Relic-specific issues

### Known Limitations

1. **Plan Collection**: Not available until pg_querylens extension is ready
2. **Multi-Instance State**: Requires external state store (Redis)
3. **MySQL ASH**: Not implemented for MySQL databases
4. **Cross-Database Correlation**: Limited in current version

## Emergency Procedures

### Disable Collection Immediately

```bash
# Docker
docker-compose stop db-intelligence-primary

# Kubernetes
kubectl scale deployment db-intelligence-collector --replicas=0
```

### Revert to Standard Components

```bash
# Switch to standard configuration
kubectl set image deployment/db-intelligence-collector \
  collector=otel/opentelemetry-collector-contrib:0.88.0

# Update configmap to use standard components
kubectl edit configmap db-intelligence-config
```

### Clear State and Restart

```bash
# Stop all services
docker-compose down

# Clear volumes
docker volume rm database-intelligence-mvp_collector-data

# Restart fresh
docker-compose up -d
```