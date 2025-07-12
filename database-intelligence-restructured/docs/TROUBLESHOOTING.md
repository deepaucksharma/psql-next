# Troubleshooting Guide

Common issues and solutions for Database Intelligence with OpenTelemetry.

## 🚨 Quick Diagnosis

### Health Check

```bash
# Check collector health
curl http://localhost:13133/health

# Expected response: {"status":"OK"}
```

### Common Issues Checklist

1. ✅ **Database connectivity**: Can you connect to the database?
2. ✅ **New Relic credentials**: Is your license key valid?
3. ✅ **Configuration syntax**: Is your YAML valid?
4. ✅ **Permissions**: Does the database user have required permissions?
5. ✅ **Network**: Can the collector reach New Relic?

## 🔌 Database Connection Issues

### PostgreSQL Connection Problems

**Symptoms**: `connection refused`, `authentication failed`

```bash
# Test database connection
psql "${DB_ENDPOINT}" -c "SELECT 1"

# Check connection string format
echo "${DB_ENDPOINT}"
# Should be: postgresql://user:pass@host:port/db
```

**Solutions**:

1. **Check credentials**:
   ```sql
   -- Verify user exists and has permissions
   SELECT usename, usesuper FROM pg_user WHERE usename = 'otel_monitor';
   
   -- Grant required permissions
   GRANT CONNECT ON DATABASE mydb TO otel_monitor;
   GRANT USAGE ON SCHEMA public TO otel_monitor;
   GRANT SELECT ON ALL TABLES IN SCHEMA public TO otel_monitor;
   ```

2. **Check network connectivity**:
   ```bash
   # Test port connectivity
   telnet postgres-host 5432
   
   # Check firewall rules
   sudo ufw status
   ```

3. **Verify SSL settings**:
   ```yaml
   receivers:
     postgresql:
       endpoint: "${DB_ENDPOINT}"
       tls:
         insecure_skip_verify: false  # Set to true for testing
   ```

### MySQL Connection Problems

**Symptoms**: `Access denied`, `Can't connect to MySQL server`

```bash
# Test MySQL connection
mysql -h "${MYSQL_HOST}" -u "${MYSQL_USER}" -p"${MYSQL_PASSWORD}" -e "SELECT 1"
```

**Solutions**:

1. **Check user permissions**:
   ```sql
   -- Create monitoring user
   CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'secure_password';
   GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
   GRANT SELECT ON information_schema.* TO 'otel_monitor'@'%';
   GRANT PROCESS ON *.* TO 'otel_monitor'@'%';
   FLUSH PRIVILEGES;
   ```

2. **Check bind address**:
   ```bash
   # Ensure MySQL binds to correct interface
   grep bind-address /etc/mysql/mysql.conf.d/mysqld.cnf
   # Should be: bind-address = 0.0.0.0
   ```

## 📊 No Metrics in New Relic

### Verify New Relic Integration

```bash
# Test New Relic API connectivity
curl -H "Api-Key: ${NEW_RELIC_LICENSE_KEY}" \
  https://api.newrelic.com/v2/applications.json

# Test OTLP endpoint
curl -v -X POST "${NEW_RELIC_OTLP_ENDPOINT}/v1/metrics" \
  -H "Api-Key: ${NEW_RELIC_LICENSE_KEY}" \
  -H "Content-Type: application/x-protobuf"
```

### Common New Relic Issues

1. **Invalid license key**:
   ```bash
   # Check license key format (should be 40 characters)
   echo "${NEW_RELIC_LICENSE_KEY}" | wc -c
   
   # Verify license key in New Relic UI
   # Go to: one.newrelic.com → API Keys
   ```

2. **Wrong OTLP endpoint**:
   ```bash
   # US endpoint
   export NEW_RELIC_OTLP_ENDPOINT="https://otlp.nr-data.net:4318"
   
   # EU endpoint  
   export NEW_RELIC_OTLP_ENDPOINT="https://otlp.eu01.nr-data.net:4318"
   ```

3. **Data not appearing**:
   ```bash
   # Check for data in New Relic (may take 1-2 minutes)
   # Query: FROM Metric SELECT * WHERE service.name = 'your-service'
   ```

### Enable Debug Logging

```yaml
exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 100

service:
  pipelines:
    metrics:
      exporters: [debug, otlp]  # Add debug exporter
```

## 🧠 High Memory Usage

### Memory Monitoring

```bash
# Check collector memory usage
ps aux | grep otelcol

# Docker memory usage
docker stats otel-collector

# Kubernetes memory usage
kubectl top pods -l app=otel-collector
```

### Memory Optimization

1. **Add memory limiter**:
   ```yaml
   processors:
     memory_limiter:
       limit_mib: 512        # 512MB limit
       spike_limit_mib: 128  # 128MB spike protection
       check_interval: 1s
   
   service:
     pipelines:
       metrics:
         processors: [memory_limiter, ...]  # First processor
   ```

2. **Reduce collection frequency**:
   ```yaml
   receivers:
     postgresql:
       collection_interval: 60s  # From 30s
     sqlquery:
       collection_interval: 300s # From 60s
   ```

3. **Limit metric cardinality**:
   ```yaml
   processors:
     filter:
       metrics:
         datapoint:
           - 'attributes["table_name"] == "pg_temp_*"'  # Drop temp tables
   ```

## ⚡ High CPU Usage

### CPU Monitoring

```bash
# Check CPU usage
top -p $(pidof otelcol)

# Profile CPU usage
curl http://localhost:1777/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

### CPU Optimization

1. **Optimize batch processing**:
   ```yaml
   processors:
     batch:
       timeout: 10s
       send_batch_size: 2048   # Larger batches
       send_batch_max_size: 4096
   ```

2. **Reduce processor complexity**:
   ```yaml
   processors:
     - resource     # Keep lightweight processors
     - batch
   ```

3. **Limit concurrent queries**:
   ```yaml
   receivers:
     sqlquery:
       max_concurrent_queries: 2  # Reduce from default
   ```

## 🌐 Network Issues

### Connectivity Testing

```bash
# Test database connectivity
nc -zv postgres-host 5432
nc -zv mysql-host 3306

# Test New Relic connectivity
nc -zv otlp.nr-data.net 4318

# Check DNS resolution
nslookup postgres-host
nslookup otlp.nr-data.net
```

### Firewall Configuration

```bash
# Allow outbound HTTPS (443) for New Relic
sudo ufw allow out 443

# Allow database ports (if collector is external)
sudo ufw allow out 5432  # PostgreSQL
sudo ufw allow out 3306  # MySQL

# Allow health check port
sudo ufw allow 13133
```

## 📝 Configuration Issues

### YAML Validation

```bash
# Validate YAML syntax
python -c "import yaml; yaml.safe_load(open('config.yaml'))"

# OpenTelemetry config validation
otelcol --config=config.yaml --dry-run
```

### Common Configuration Errors

1. **Missing required fields**:
   ```yaml
   # ❌ Missing endpoint
   receivers:
     postgresql: {}
   
   # ✅ Correct
   receivers:
     postgresql:
       endpoint: "${DB_ENDPOINT}"
   ```

2. **Invalid processor order**:
   ```yaml
   # ❌ Wrong order (memory_limiter should be first)
   processors: [batch, memory_limiter, resource]
   
   # ✅ Correct order
   processors: [memory_limiter, resource, batch]
   ```

3. **Undefined environment variables**:
   ```bash
   # Check environment variables
   env | grep -E "(DB_|NEW_RELIC_)"
   
   # Set missing variables
   export DB_ENDPOINT="postgresql://user:pass@host:5432/db"
   ```

## 🔍 Log Analysis

### Enable Debug Logging

```yaml
service:
  telemetry:
    logs:
      level: debug
      output_paths:
        - stdout
        - /var/log/otelcol/collector.log
```

### Log Patterns to Look For

1. **Connection errors**:
   ```bash
   grep -i "connection\|connect" collector.log
   grep -i "refused\|timeout" collector.log
   ```

2. **Authentication errors**:
   ```bash
   grep -i "auth\|permission\|denied" collector.log
   ```

3. **Export errors**:
   ```bash
   grep -i "export\|send\|failed" collector.log
   ```

4. **Memory issues**:
   ```bash
   grep -i "memory\|oom\|limit" collector.log
   ```

## 🚨 Emergency Procedures

### Collector Crash Recovery

```bash
# Check if collector is running
ps aux | grep otelcol

# Restart collector
sudo systemctl restart otelcol

# Check status
sudo systemctl status otelcol

# View recent logs
journalctl -u otelcol --since "5 minutes ago"
```

### Database Overload Protection

```yaml
# Emergency configuration with minimal collection
receivers:
  postgresql:
    collection_interval: 300s  # 5 minutes
    
processors:
  memory_limiter:
    limit_mib: 256
  
  # Remove all non-essential processors
  batch: {}
```

### Quick Disable

```bash
# Stop collector
sudo systemctl stop otelcol

# Disable collector
sudo systemctl disable otelcol

# Kill running processes
pkill -f otelcol
```

This troubleshooting guide covers the most common issues and provides systematic approaches to diagnosis and resolution.