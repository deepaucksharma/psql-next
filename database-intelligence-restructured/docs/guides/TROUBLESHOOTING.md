# PostgreSQL Metrics Collection Troubleshooting Guide

## Overview
This guide helps troubleshoot common issues when PostgreSQL metrics are not appearing in New Relic or are incomplete.

## Quick Diagnostics Checklist

### 1. Service Health Check
```bash
# Check if all containers are running
docker ps | grep db-intel

# Check PostgreSQL connectivity
docker exec db-intel-postgres pg_isready -U postgres

# Check collector health
curl -s http://localhost:4318/v1/metrics | grep -c postgresql  # Config-Only
curl -s http://localhost:5318/v1/metrics | grep -c postgresql  # Custom
```

### 2. Verify Environment Variables
```bash
# Check collector environment
docker exec db-intel-collector-config-only env | grep -E "(POSTGRES|NEW_RELIC|DEPLOYMENT)"

# Verify New Relic credentials
echo "License Key: ${NEW_RELIC_LICENSE_KEY:0:10}..."
echo "Account ID: $NEW_RELIC_ACCOUNT_ID"
```

### 3. Quick Metric Check in New Relic
```sql
-- Check if any metrics are arriving
SELECT count(*) FROM Metric 
WHERE deployment.mode IN ('config-only', 'custom') 
SINCE 5 minutes ago

-- List all PostgreSQL metrics
SELECT uniques(metricName) FROM Metric 
WHERE metricName LIKE 'postgresql%' 
SINCE 30 minutes ago
```

## Common Issues and Solutions

### Issue 1: No Metrics Appearing in New Relic

**Symptoms:**
- No data in dashboards
- NRQL queries return no results
- Collectors appear to be running

**Troubleshooting Steps:**

1. **Check collector logs for errors:**
```bash
docker logs --tail 100 db-intel-collector-config-only 2>&1 | grep -E "(error|ERROR|failed)"
```

2. **Verify OTLP endpoint connectivity:**
```bash
docker exec db-intel-collector-config-only curl -s https://otlp.nr-data.net:4317
# Should return: Client sent an HTTP request to an HTTPS server
```

3. **Check for authentication errors:**
```bash
docker logs db-intel-collector-config-only 2>&1 | grep -i "unauthorized\|401\|403"
```

**Solutions:**
- Verify NEW_RELIC_LICENSE_KEY is correct
- Ensure NEW_RELIC_OTLP_ENDPOINT is set correctly (default: https://otlp.nr-data.net:4317)
- Check if your account has OTLP ingestion enabled

### Issue 2: Missing Specific PostgreSQL Metrics

**Symptoms:**
- Some metrics appear but others are missing
- Expected metrics like `postgresql.deadlocks` not showing

**Troubleshooting Steps:**

1. **Check PostgreSQL permissions:**
```bash
docker exec db-intel-postgres psql -U postgres -c "
SELECT has_database_privilege('postgres', 'testdb', 'CONNECT');
SELECT has_table_privilege('postgres', 'pg_stat_database', 'SELECT');
SELECT has_table_privilege('postgres', 'pg_stat_activity', 'SELECT');
"
```

2. **Verify metrics are enabled in config:**
```bash
docker exec db-intel-collector-config-only cat /etc/otel-collector-config.yaml | grep -A 2 "postgresql.deadlocks"
```

3. **Check if PostgreSQL extensions are enabled:**
```bash
docker exec db-intel-postgres psql -U postgres -d testdb -c "
SELECT * FROM pg_available_extensions WHERE name = 'pg_stat_statements';
"
```

**Solutions:**
- Grant necessary permissions to PostgreSQL user
- Enable missing metrics in config-only-mode.yaml
- Install required PostgreSQL extensions:
```sql
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

### Issue 3: PostgreSQL Connection Errors

**Symptoms:**
- Collector logs show connection refused or timeout
- No PostgreSQL metrics but host metrics work

**Troubleshooting Steps:**

1. **Test PostgreSQL connectivity from collector:**
```bash
docker exec db-intel-collector-config-only sh -c "
  apt-get update && apt-get install -y postgresql-client
  psql -h postgres -U postgres -d testdb -c 'SELECT 1'
"
```

2. **Check PostgreSQL configuration:**
```bash
docker exec db-intel-postgres cat /var/lib/postgresql/data/postgresql.conf | grep -E "listen_addresses|port"
docker exec db-intel-postgres cat /var/lib/postgresql/data/pg_hba.conf | tail -10
```

**Solutions:**
- Ensure PostgreSQL is listening on correct interface
- Update pg_hba.conf to allow collector connections
- Verify network connectivity between containers

### Issue 4: High Cardinality or Missing Attributes

**Symptoms:**
- Metrics appear but without expected attributes
- Can't filter by database name or table name

**Troubleshooting Steps:**

1. **Check available attributes:**
```sql
SELECT keyset() FROM Metric 
WHERE metricName = 'postgresql.backends' 
SINCE 1 hour ago 
LIMIT 1
```

2. **Verify resource detection:**
```bash
docker logs db-intel-collector-config-only 2>&1 | grep -i "resource"
```

**Solutions:**
- Ensure resourcedetection processor is configured
- Add missing attributes via attributes processor
- Check if attributes are being dropped by any processor

### Issue 5: Collector Performance Issues

**Symptoms:**
- High CPU/memory usage
- Metrics lag or arrive late
- Collector restarts frequently

**Troubleshooting Steps:**

1. **Check collector metrics:**
```bash
# Memory usage
docker stats db-intel-collector-config-only --no-stream

# Check for memory pressure
docker logs db-intel-collector-config-only 2>&1 | grep -i "memory"
```

2. **Review batch settings:**
```yaml
batch:
  timeout: 10s
  send_batch_size: 1000  # Reduce if seeing memory issues
```

**Solutions:**
- Increase memory_limiter limits
- Reduce collection_interval for receivers
- Enable sampling in custom mode
- Reduce batch size

### Issue 6: SQL Query Receiver Not Working

**Symptoms:**
- Custom metrics from sqlquery receiver missing
- Standard metrics work but pg.* metrics don't appear

**Troubleshooting Steps:**

1. **Test SQL queries manually:**
```bash
docker exec db-intel-postgres psql -U postgres -d testdb -c "
SELECT state, COUNT(*) as connection_count 
FROM pg_stat_activity 
WHERE pid != pg_backend_pid() 
GROUP BY state;
"
```

2. **Check for SQL errors in logs:**
```bash
docker logs db-intel-collector-config-only 2>&1 | grep -A 5 -B 5 "sqlquery"
```

**Solutions:**
- Verify SQL syntax is PostgreSQL-compatible
- Ensure queries don't take too long (timeout)
- Check column names match metric configuration

## Performance Optimization

### Reduce Metric Volume
```yaml
# Increase collection intervals
postgresql:
  collection_interval: 30s  # Instead of 10s

# Disable less critical metrics
postgresql.bgwriter.buffers.writes:
  enabled: false
```

### Enable Sampling (Custom Mode)
```yaml
adaptivesampler:
  sampling_percentage: 50  # Sample 50% of metrics
```

### Optimize SQL Queries
```yaml
sqlquery/postgresql:
  collection_interval: 60s  # Reduce frequency
  queries:
    - sql: "SELECT ... LIMIT 100"  # Add limits
```

## Verification Commands

### Generate Test Load
```bash
# Run PostgreSQL-specific load generator
cd tools/load-generator
go run main.go -pattern=mixed -qps=50

# Or use the comprehensive test generator
cd tools/postgres-test-generator
go run main.go -workers=10
```

### Monitor Metric Flow
```bash
# Watch metrics in real-time
watch -n 5 'docker logs --tail 20 db-intel-collector-config-only 2>&1 | grep postgresql'

# Check metric count growth
while true; do
  curl -s http://localhost:4318/v1/metrics | grep -c "postgresql" | ts
  sleep 10
done
```

### Validate in New Relic
```sql
-- Metric freshness check
SELECT 
  deployment.mode,
  latest(timestamp) as 'Last Data',
  now() - latest(timestamp) as 'Age (ms)'
FROM Metric 
WHERE metricName LIKE 'postgresql%' 
FACET deployment.mode 
SINCE 10 minutes ago

-- Metric coverage by mode
SELECT 
  uniqueCount(metricName) as 'Unique Metrics',
  rate(count(*), 1 minute) as 'DPM'
FROM Metric 
WHERE metricName LIKE 'postgresql%' 
FACET deployment.mode 
SINCE 1 hour ago
```

## Getting Help

1. **Collect Diagnostic Information:**
```bash
./scripts/verify-metrics.sh > diagnostics.txt
docker logs db-intel-collector-config-only > collector.log 2>&1
docker exec db-intel-postgres pg_dump -s testdb > schema.sql
```

2. **Check Documentation:**
- [OpenTelemetry PostgreSQL Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/postgresqlreceiver)
- [New Relic OTLP Documentation](https://docs.newrelic.com/docs/apis/otlp/)

3. **Enable Debug Logging:**
```yaml
service:
  telemetry:
    logs:
      level: debug  # Change from info to debug
```

4. **Test with Debug Exporter:**
```yaml
exporters:
  debug:
    verbosity: detailed
    sampling_initial: 10  # Show first 10 data points
```

## Prevention Best Practices

1. **Always verify configuration before deployment:**
```bash
docker run --rm -v $(pwd)/config-only-mode.yaml:/config.yaml \
  otel/opentelemetry-collector-contrib:0.105.0 \
  --config=/config.yaml --dry-run
```

2. **Monitor collector health:**
- Set up alerts for collector restarts
- Monitor collector memory/CPU usage
- Track metric ingestion rates

3. **Use incremental rollout:**
- Test with a single database first
- Gradually increase collection scope
- Monitor impact on PostgreSQL performance

4. **Regular validation:**
- Run verification scripts daily
- Compare expected vs actual metrics
- Review collector logs for warnings