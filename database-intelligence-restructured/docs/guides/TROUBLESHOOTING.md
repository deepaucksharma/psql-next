# Troubleshooting Guide

This comprehensive guide helps you diagnose and resolve issues with Database Intelligence collectors.

## Table of Contents
- [Quick Diagnostics](#quick-diagnostics)
- [General Issues](#general-issues)
- [PostgreSQL Issues](#postgresql-issues)
- [MySQL Issues](#mysql-issues)
- [MongoDB Issues](#mongodb-issues)
- [MSSQL Issues](#mssql-issues)
- [Oracle Issues](#oracle-issues)
- [Performance Issues](#performance-issues)
- [New Relic Integration](#new-relic-integration)
- [Deployment Issues](#deployment-issues)
- [Advanced Debugging](#advanced-debugging)

## Quick Diagnostics

### 1. Run Health Check
```bash
# Full system validation
./scripts/validate-e2e.sh

# Specific database test
./scripts/test-database-config.sh postgresql 60

# Integration test
./scripts/test-integration.sh all
```

### 2. Check Common Issues
```bash
# View collector logs
docker logs otel-collector-postgresql

# Check metrics endpoint
curl -s http://localhost:8888/metrics | grep -E "(accepted|refused|failed)"

# Validate configuration
./scripts/validate-config.sh postgresql
```

## General Issues

### No Metrics Appearing

1. **Check collector status**:
   ```bash
   docker logs otel-collector
   ```

2. **Verify configuration**:
   ```bash
   ./scripts/validate-config.sh configs/postgresql-maximum-extraction.yaml
   ```

3. **Test connectivity**:
   ```bash
   ./scripts/test-database-config.sh postgresql 60
   ```

4. **Check New Relic**:
   ```sql
   SELECT count(*) FROM Metric 
   WHERE collector.name = 'database-intelligence-*' 
   SINCE 5 minutes ago
   ```

### Authentication Errors

1. **Verify credentials**:
   ```bash
   # Test database connection directly
   psql -h localhost -U postgres -d postgres
   mysql -h localhost -u root -p
   mongosh mongodb://localhost:27017
   sqlcmd -S localhost -U sa -P password
   sqlplus user/pass@localhost:1521/ORCLPDB1
   ```

2. **Check environment variables**:
   ```bash
   env | grep -E "(POSTGRES|MYSQL|MONGODB|MSSQL|ORACLE)"
   ```

### High Memory Usage

1. **Adjust memory limits**:
   ```yaml
   processors:
     memory_limiter:
       limit_mib: 512  # Reduce limit
       spike_limit_mib: 128
   ```

2. **Reduce cardinality**:
   ```yaml
   processors:
     filter/reduce_cardinality:
       metrics:
         exclude:
           metric_names:
             - "*.query.*"  # Exclude per-query metrics
   ```

## PostgreSQL Issues

### pg_stat_statements Not Available

```sql
-- Enable extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Verify it's loaded
SELECT * FROM pg_extension WHERE extname = 'pg_stat_statements';

-- Check postgresql.conf
SHOW shared_preload_libraries;
```

### Connection Pool Exhausted

```yaml
# Reduce collection frequency
sqlquery/ash:
  collection_interval: 10s  # Increase from 1s
```

### Replication Metrics Missing

```sql
-- Check if replica
SELECT pg_is_in_recovery();

-- Verify replication slots
SELECT * FROM pg_replication_slots;
```

## MySQL Issues

### Performance Schema Disabled

```sql
-- Check if enabled
SHOW VARIABLES LIKE 'performance_schema';

-- Enable in my.cnf
[mysqld]
performance_schema=ON

-- Restart MySQL
systemctl restart mysql
```

### Access Denied Errors

```sql
-- Grant required permissions
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
FLUSH PRIVILEGES;
```

### Slow Query Metrics Missing

```sql
-- Check slow query log
SHOW VARIABLES LIKE 'slow_query_log%';

-- Enable if needed
SET GLOBAL slow_query_log = 'ON';
SET GLOBAL long_query_time = 1;
```

## MongoDB Issues

### Authentication Failed

```javascript
// Verify user exists
use admin
db.getUsers()

// Create monitoring user
db.createUser({
  user: "otel_monitor",
  pwd: "password",
  roles: [
    { role: "clusterMonitor", db: "admin" },
    { role: "read", db: "local" }
  ]
})
```

### currentOp Permission Denied

```javascript
// Grant required role
db.grantRolesToUser("otel_monitor", [
  { role: "clusterMonitor", db: "admin" }
])
```

### Atlas Metrics Not Working

1. Verify API keys are set
2. Check project name matches exactly
3. Ensure IP whitelist includes collector

## MSSQL Issues

### Connection Timeout

```yaml
# Increase timeout in connection string
datasource: "sqlserver://user:pass@host:1433?connection+timeout=30"
```

### Permission Errors

```sql
-- Grant required permissions
GRANT VIEW SERVER STATE TO otel_monitor;
GRANT VIEW ANY DEFINITION TO otel_monitor;

-- For each database
USE [YourDatabase];
GRANT VIEW DATABASE STATE TO otel_monitor;
```

### Always On AG Metrics Missing

```sql
-- Check if AG is configured
SELECT * FROM sys.availability_groups;

-- Verify permissions
SELECT * FROM fn_my_permissions(NULL, 'SERVER')
WHERE permission_name LIKE '%VIEW%';
```

## Oracle Issues

### ORA-12154: TNS Error

```yaml
# Use full connection string
datasource: "oracle://user:pass@(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=localhost)(PORT=1521))(CONNECT_DATA=(SERVICE_NAME=ORCLPDB1)))"
```

### Missing V$ Views

```sql
-- Grant access
GRANT SELECT ANY DICTIONARY TO otel_monitor;
GRANT SELECT ON V_$SESSION TO otel_monitor;
GRANT SELECT ON V_$SYSSTAT TO otel_monitor;
```

### Character Set Issues

```bash
# Set NLS_LANG
export NLS_LANG=AMERICAN_AMERICA.AL32UTF8
```

## Performance Issues

### Collector Using Too Much CPU

1. **Reduce collection frequency**:
   ```yaml
   collection_interval: 60s  # Increase intervals
   ```

2. **Enable sampling**:
   ```yaml
   processors:
     probabilistic_sampler:
       sampling_percentage: 10
   ```

### Metrics Delayed

1. **Adjust batch settings**:
   ```yaml
   processors:
     batch:
       timeout: 5s  # Reduce timeout
       send_batch_size: 500  # Smaller batches
   ```

### Network Timeouts

```yaml
exporters:
  otlp/newrelic:
    timeout: 60s  # Increase timeout
    retry_on_failure:
      max_elapsed_time: 600s
```

## New Relic Integration

### Invalid License Key

```bash
# Verify key format
echo $NEW_RELIC_LICENSE_KEY | wc -c  # Should be 40 characters

# Test with curl
curl -X POST https://metric-api.newrelic.com/metric/v1 \
  -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  -H "Content-Type: application/json" \
  -d '[{"metrics":[]}]'
```

### Wrong Region

```yaml
# For EU region
exporters:
  otlp/newrelic:
    endpoint: otlp.eu01.nr-data.net:4317
```

### Metrics Not Appearing

1. **Check account ID**:
   ```sql
   SELECT count(*) FROM Metric 
   WHERE true 
   SINCE 1 hour ago
   ```

2. **Verify metric names**:
   ```sql
   SELECT uniques(metricName) FROM Metric 
   WHERE metricName LIKE 'postgresql%' 
   SINCE 1 hour ago
   ```

## Debug Mode

Enable debug logging:

```yaml
service:
  telemetry:
    logs:
      level: debug
      
exporters:
  debug:
    verbosity: detailed
```

## Getting Help

1. **Check logs**: Always start with collector logs
2. **Validate config**: Use provided validation scripts
3. **Test connectivity**: Ensure database is reachable
4. **Review permissions**: Database user needs specific grants
5. **Open an issue**: https://github.com/newrelic/database-intelligence/issues

## Deployment Issues

### Docker Issues

#### Container Exits Immediately
```bash
# Check exit code and logs
docker ps -a | grep otel
docker logs otel-collector-postgresql

# Common fixes
# 1. Fix config syntax errors
./scripts/validate-config.sh postgresql

# 2. Ensure config file is mounted correctly
docker run --rm -v $(pwd)/configs/postgresql-maximum-extraction.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector-contrib:latest --dry-run
```

#### Permission Denied Errors
```bash
# Fix file permissions
chmod 644 configs/*.yaml
chmod 755 scripts/*.sh

# For SELinux systems
chcon -Rt svirt_sandbox_file_t configs/
```

### Kubernetes Issues

#### ConfigMap Not Found
```bash
# Verify ConfigMap exists
kubectl get configmap -n database-intelligence

# Recreate if missing
kubectl create configmap otel-config \
  --from-file=configs/postgresql-maximum-extraction.yaml \
  -n database-intelligence
```

#### Pod CrashLoopBackOff
```bash
# Check pod logs
kubectl logs -n database-intelligence otel-postgresql-xxxxx

# Check events
kubectl describe pod -n database-intelligence otel-postgresql-xxxxx

# Common fixes
# 1. Increase memory limits
# 2. Fix configuration errors
# 3. Verify secrets are mounted
```

## Advanced Debugging

### Enable Detailed Logging
```yaml
service:
  telemetry:
    logs:
      level: debug
      development: true
      encoding: console
      disable_caller: false
      disable_stacktrace: false
      output_paths:
        - stdout
        - /var/log/otel-debug.log
```

### Trace Pipeline Processing
```yaml
service:
  telemetry:
    traces:
      level: detailed
      propagators: [tracecontext, baggage]
```

### Export to Debug Endpoint
```yaml
exporters:
  debug:
    verbosity: detailed
    sampling_initial: 2
    sampling_thereafter: 1

service:
  pipelines:
    metrics/debug:
      receivers: [postgresql]
      exporters: [debug]
```

### Performance Profiling
```bash
# Run performance benchmark
./scripts/benchmark-performance.sh postgresql 300

# Check metric cardinality
./scripts/check-metric-cardinality.sh postgresql 60

# Monitor resource usage
docker stats otel-collector-postgresql
```

### Network Debugging
```bash
# Test database connectivity
docker exec otel-collector-postgresql nc -zv ${POSTGRESQL_HOST} ${POSTGRESQL_PORT}

# Check DNS resolution
docker exec otel-collector-postgresql nslookup ${POSTGRESQL_HOST}

# Trace network path
docker exec otel-collector-postgresql traceroute ${POSTGRESQL_HOST}
```

## Configuration Issues

### Environment Variable Not Expanding
```yaml
# Wrong - missing env: prefix
endpoint: ${NEW_RELIC_OTLP_ENDPOINT}

# Correct
endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}

# With default value
endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT:-otlp.nr-data.net:4317}
```

### Pipeline Not Processing Metrics
```yaml
# Check pipeline is defined in service
service:
  pipelines:
    metrics:
      receivers: [postgresql]  # Must match receiver name
      processors: [batch]      # Optional
      exporters: [otlp/newrelic]  # Must match exporter name
```

### Receiver Configuration Errors
```yaml
# Common mistakes
receivers:
  postgresql:
    # Wrong - using connection_string
    connection_string: "postgresql://..."
    
    # Correct - use endpoint format
    endpoint: localhost:5432
    username: postgres
    password: ${env:POSTGRES_PASSWORD}
```

## Metric Cardinality Issues

### Identifying High Cardinality
```bash
# Run cardinality analysis
./scripts/check-metric-cardinality.sh postgresql

# Check specific metrics
curl -s http://localhost:8888/metrics | \
  grep -E "^postgresql" | \
  cut -d'{' -f1 | \
  sort | uniq -c | \
  sort -nr | head -20
```

### Reducing Cardinality
```yaml
processors:
  # Drop high-cardinality attributes
  attributes/drop_query_id:
    actions:
      - key: query_id
        action: delete
      - key: session_id
        action: delete
        
  # Filter out detailed metrics
  filter/reduce:
    metrics:
      exclude:
        match_type: regexp
        metric_names:
          - ".*\\.query\\..*"
          - ".*\\.session\\..*"
```

## Common Error Messages

| Error | Cause | Solution |
|-------|-------|----------|
| `connection refused` | Database not running or wrong host/port | Verify database is running and accessible |
| `authentication failed` | Wrong credentials | Check username/password |
| `permission denied` | Missing grants | Grant required permissions |
| `context deadline exceeded` | Timeout | Increase timeout values |
| `no such host` | DNS resolution failed | Verify hostname |
| `SSL/TLS required` | Security enforcement | Enable TLS in configuration |
| `out of memory` | Memory limit exceeded | Increase memory_limiter |
| `invalid configuration` | YAML syntax error | Validate with `yq` or online validator |
| `duplicate key` | Same key defined twice | Remove duplicate configuration |
| `unknown field` | Typo in configuration | Check spelling and indentation |
| `pipeline not used` | Pipeline defined but not in service | Add to service.pipelines section |

## Emergency Recovery

### Collector Consuming All Resources
```bash
# Stop immediately
docker stop otel-collector-postgresql

# Start with minimal config
cat > /tmp/minimal.yaml << EOF
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  debug:
    verbosity: basic
service:
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [debug]
EOF

docker run --rm -v /tmp/minimal.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector-contrib:latest
```

### Rollback to Previous Version
```bash
# Tag current config
git tag -a "broken-$(date +%Y%m%d)" -m "Broken configuration"

# Revert to last known good
git checkout <last-good-commit> -- configs/

# Or use specific collector version
docker pull otel/opentelemetry-collector-contrib:0.88.0
```

## Prevention Best Practices

1. **Always validate before deploying**
   ```bash
   ./scripts/validate-config.sh postgresql
   ./scripts/test-integration.sh postgresql
   ```

2. **Start with minimal configuration**
   - Test basic connectivity first
   - Add metrics incrementally
   - Monitor resource usage

3. **Use staging environment**
   - Test all changes in non-production
   - Run performance benchmarks
   - Verify metric appearance in New Relic

4. **Monitor collector health**
   - Set up alerts for collector metrics
   - Track memory and CPU usage
   - Monitor error rates

5. **Regular maintenance**
   - Update collectors quarterly
   - Review and optimize configurations
   - Clean up unused metrics
