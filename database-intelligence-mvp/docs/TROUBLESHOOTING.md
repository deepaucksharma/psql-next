# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with the Database Intelligence OTEL Collector.

## Table of Contents
- [Common Issues](#common-issues)
- [Debugging Tools](#debugging-tools)
- [Performance Issues](#performance-issues)
- [Configuration Problems](#configuration-problems)
- [Database Connectivity](#database-connectivity)
- [Metrics Not Appearing](#metrics-not-appearing)
- [Error Messages](#error-messages)

## Common Issues

### Collector Won't Start

**Symptoms:**
- Collector exits immediately after starting
- No logs produced
- Health check endpoint not responding

**Solutions:**

1. **Check configuration syntax:**
```bash
./dist/otelcol-db-intelligence validate --config=config/collector.yaml
```

2. **Verify all environment variables are set:**
```bash
# Check required variables
echo $NEW_RELIC_LICENSE_KEY
echo $POSTGRES_HOST
echo $POSTGRES_USER
echo $POSTGRES_PASSWORD
```

3. **Check for port conflicts:**
```bash
# Check if ports are already in use
lsof -i :13133  # Health check
lsof -i :8888   # Prometheus metrics
lsof -i :8889   # Internal metrics
```

4. **Review startup logs:**
```bash
./dist/otelcol-db-intelligence --config=config/collector.yaml --log-level=debug
```

### High Memory Usage

**Symptoms:**
- Collector consuming excessive memory
- OOM kills
- Slow performance

**Solutions:**

1. **Adjust memory limiter settings:**
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 70  # Reduce from 80
    spike_limit_percentage: 20  # Reduce from 30
```

2. **Reduce batch sizes:**
```yaml
processors:
  batch:
    timeout: 5s  # Reduce from 10s
    send_batch_size: 5000  # Reduce from 10000
```

3. **Enable adaptive sampling more aggressively:**
```yaml
processors:
  database_intelligence/adaptivesampler:
    min_sampling_rate: 0.05  # 5% instead of 10%
    high_cost_threshold_ms: 500  # Lower threshold
```

### Metrics Not Reaching New Relic

**Symptoms:**
- Metrics visible in Prometheus endpoint but not in New Relic
- OTLP export errors in logs
- API key errors

**Solutions:**

1. **Verify New Relic configuration:**
```yaml
exporters:
  otlp/newrelic:
    endpoint: https://otlp.nr-data.net:4317  # US datacenter
    # endpoint: https://otlp.eu01.nr-data.net:4317  # EU datacenter
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
```

2. **Check network connectivity:**
```bash
# Test OTLP endpoint
telnet otlp.nr-data.net 4317

# Test with curl
curl -I https://otlp.nr-data.net:4317
```

3. **Enable debug logging for exporter:**
```yaml
service:
  telemetry:
    logs:
      level: debug
      output_paths: ["stdout", "collector.log"]
```

4. **Check for rate limiting:**
Look for these patterns in logs:
- "429 Too Many Requests"
- "rate limit exceeded"
- "api-key invalid"

## Debugging Tools

### Health Check Endpoint

The health check provides basic status:
```bash
curl http://localhost:13133/
```

Response when healthy:
```json
{"status":"Server available"}
```

### Prometheus Metrics Endpoint

View all collected metrics:
```bash
# All metrics
curl http://localhost:8888/metrics

# Filter database metrics
curl http://localhost:8888/metrics | grep -E "^db_|^postgresql_|^mysql_"

# Check collector internals
curl http://localhost:8888/metrics | grep -E "^otelcol_"
```

### zPages (Debug UI)

Access detailed internal state:
```bash
# Open in browser
open http://localhost:55679/debug/

# Available pages:
# /debug/servicez - Service information
# /debug/pipelinez - Pipeline metrics
# /debug/extensionz - Extension information
# /debug/tracez - Internal traces
```

### pprof (Performance Profiling)

Profile CPU and memory usage:
```bash
# CPU profile
go tool pprof http://localhost:1777/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://localhost:1777/debug/pprof/heap

# Goroutine profile
curl http://localhost:1777/debug/pprof/goroutine?debug=1
```

## Performance Issues

### Slow Metric Collection

**Issue:** Metrics taking too long to collect

**Solutions:**

1. **Increase collection intervals:**
```yaml
receivers:
  postgresql:
    collection_interval: 120s  # Increase from 60s
    
  sqlquery/postgresql:
    queries:
      - sql: "..."
        collection_interval: 300s  # Increase from 60s
```

2. **Optimize SQL queries:**
```yaml
receivers:
  sqlquery/postgresql:
    queries:
      - sql: |
          SELECT ... FROM pg_stat_statements
          WHERE calls > 100  -- Add filter
          LIMIT 50  -- Reduce limit
```

3. **Enable circuit breaker:**
```yaml
processors:
  database_intelligence/circuitbreaker:
    failure_threshold: 3  # More sensitive
    cooldown_period: 300s  # Longer cooldown
```

### High CPU Usage

**Issue:** Collector using excessive CPU

**Solutions:**

1. **Reduce verification frequency:**
```yaml
processors:
  database_intelligence/verification:
    health_checks:
      interval: 300s  # Increase from 60s
    metric_quality:
      enabled: false  # Disable if not critical
```

2. **Simplify PII patterns:**
```yaml
processors:
  transform:
    metric_statements:
      - context: datapoint
        statements:
          # Use simpler patterns
          - replace_pattern(attributes["query.text"], "\\d{4,}", "[NUM]")
```

## Configuration Problems

### Invalid Processor Configuration

**Error:** `error decoding 'processors': unknown type: "database_intelligence/adaptivesampler"`

**Solution:** Ensure the processor is included in the build:
```yaml
# ocb-config.yaml
processors:
  - gomod: github.com/database-intelligence-mvp/processors/adaptivesampler v0.0.0
```

### Environment Variable Not Resolved

**Error:** `cannot resolve the configuration: environment variable "POSTGRES_PASSWORD" has empty value`

**Solution:**
```bash
# Export the variable
export POSTGRES_PASSWORD="your_password"

# Or use a default in config
password: ${env:POSTGRES_PASSWORD:-default_password}
```

## Database Connectivity

### PostgreSQL Connection Failed

**Error:** `pq: password authentication failed for user "monitor"`

**Solutions:**

1. **Verify credentials:**
```bash
# Test connection
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DATABASE -c "SELECT 1"
```

2. **Check pg_hba.conf:**
```sql
-- On PostgreSQL server
SHOW hba_file;
-- Ensure appropriate authentication method
```

3. **Grant necessary permissions:**
```sql
-- Required permissions for monitoring
GRANT pg_monitor TO monitor_user;
GRANT SELECT ON pg_stat_statements TO monitor_user;
```

### MySQL Connection Failed

**Error:** `Error 1045: Access denied for user 'monitor'@'host'`

**Solutions:**

1. **Verify credentials:**
```bash
mysql -h $MYSQL_HOST -u $MYSQL_USER -p$MYSQL_PASSWORD -e "SELECT 1"
```

2. **Grant permissions:**
```sql
-- Required permissions
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'monitor'@'%';
```

## Metrics Not Appearing

### PostgreSQL Metrics Missing

**Issue:** No PostgreSQL metrics in Prometheus endpoint

**Checklist:**
1. Is pg_stat_statements enabled?
```sql
SHOW shared_preload_libraries;
-- Should include pg_stat_statements

CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

2. Is the receiver in a pipeline?
```yaml
service:
  pipelines:
    metrics:
      receivers: [postgresql]  # Must be listed
```

3. Check receiver logs:
```bash
./dist/otelcol-db-intelligence --config=config/collector.yaml --log-level=debug 2>&1 | grep postgresql
```

### Query Metrics Missing

**Issue:** SQLQuery receiver not producing metrics

**Solutions:**

1. **Test query manually:**
```bash
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DATABASE -c "YOUR_QUERY_HERE"
```

2. **Check query timeout:**
```yaml
receivers:
  sqlquery/postgresql:
    queries:
      - sql: "..."
        timeout: 30s  # Increase if needed
```

## Error Messages

### "Context deadline exceeded"

**Meaning:** Operation timed out

**Solutions:**
- Increase timeouts in receiver configuration
- Check database performance
- Reduce query complexity

### "Memory limit exceeded"

**Meaning:** Memory limiter triggered

**Solutions:**
- Increase memory limits
- Reduce batch sizes
- Enable more aggressive sampling

### "Circuit breaker open"

**Meaning:** Too many failures, circuit breaker activated

**Solutions:**
- Check database health
- Review error logs for root cause
- Wait for cooldown period or restart collector

### "PII detected in metrics"

**Meaning:** Verification processor found potential PII

**Solutions:**
- Review PII patterns in transform processor
- Add field to exclusion list if false positive
- Enable PII redaction instead of alerting

## Getting Help

If you can't resolve an issue:

1. **Collect diagnostic information:**
```bash
# Collector version
./dist/otelcol-db-intelligence --version

# Configuration (sanitized)
cat config/collector.yaml | sed 's/password.*/password: [REDACTED]/'

# Recent logs
tail -n 1000 collector.log

# Metrics snapshot
curl http://localhost:8888/metrics > metrics-snapshot.txt
```

2. **Check known issues:**
- GitHub Issues: https://github.com/database-intelligence-mvp/issues
- OTEL Collector Issues: https://github.com/open-telemetry/opentelemetry-collector/issues

3. **Community resources:**
- OpenTelemetry Slack: #otel-collector
- New Relic Community: https://discuss.newrelic.com/