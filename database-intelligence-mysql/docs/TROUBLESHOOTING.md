# Troubleshooting Guide

## Common Issues and Solutions

### 1. No Metrics Appearing in New Relic

#### Symptoms
- No MySQL metrics visible in New Relic
- Empty dashboards
- No data when querying for `mysql.*` metrics

#### Solutions

1. **Verify API Key Configuration**
```bash
# Check if API key is set correctly
docker compose exec otel-collector env | grep NEW_RELIC_API_KEY

# Verify it's not the default placeholder
grep NEW_RELIC_API_KEY .env
```

2. **Check Collector Logs**
```bash
# View recent logs
docker compose logs otel-collector --tail=50

# Look for export errors
docker compose logs otel-collector | grep -i "error\|failed\|refused"
```

3. **Verify Network Connectivity**
```bash
# Test connection to New Relic
docker compose exec otel-collector nc -zv otlp.nr-data.net 4317

# For EU datacenter
docker compose exec otel-collector nc -zv otlp.eu01.nr-data.net 4317
```

4. **Check Collector Health**
```bash
curl -v http://localhost:13133/
# Should return "Server available"
```

### 2. MySQL Connection Errors

#### Symptoms
- "Access denied" errors in collector logs
- "Can't connect to MySQL server" errors
- No metrics being collected

#### Solutions

1. **Verify MySQL is Running**
```bash
docker compose ps
# All services should show "running" and "healthy"
```

2. **Test MySQL Monitoring User**
```bash
# Test connection as monitoring user
docker compose exec mysql-primary mysql -uotel_monitor -potelmonitorpass -e "SELECT 1;"

# Check grants
docker compose exec mysql-primary mysql -uotel_monitor -potelmonitorpass -e "SHOW GRANTS;"
```

3. **Recreate Monitoring User**
```bash
docker compose exec mysql-primary mysql -uroot -prootpassword <<EOF
DROP USER IF EXISTS 'otel_monitor'@'%';
CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'otelmonitorpass';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON information_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON mysql.* TO 'otel_monitor'@'%';
GRANT SELECT ON sys.* TO 'otel_monitor'@'%';
FLUSH PRIVILEGES;
EOF
```

### 3. Replication Not Working

#### Symptoms
- Replica shows as not running
- High replication lag
- No replica metrics

#### Solutions

1. **Check Replication Status**
```bash
docker compose exec mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G"
```

2. **Reset Replication**
```bash
# Stop and reset replica
docker compose exec mysql-replica mysql -uroot -prootpassword <<EOF
STOP SLAVE;
RESET SLAVE ALL;
CHANGE MASTER TO
    MASTER_HOST='mysql-primary',
    MASTER_USER='root',
    MASTER_PASSWORD='rootpassword',
    MASTER_AUTO_POSITION=1;
START SLAVE;
EOF
```

3. **Check Binary Logs**
```bash
# On primary
docker compose exec mysql-primary mysql -uroot -prootpassword -e "SHOW MASTER STATUS;"

# On replica
docker compose exec mysql-replica mysql -uroot -prootpassword -e "SHOW SLAVE STATUS\G" | grep -E "Master_Log_File|Read_Master_Log_Pos"
```

### 4. High Memory Usage

#### Symptoms
- OTel collector using excessive memory
- Container being killed (OOM)
- Slow metric export

#### Solutions

1. **Check Current Usage**
```bash
docker stats otel-collector
```

2. **Adjust Memory Limits**
Edit `docker-compose.yml`:
```yaml
services:
  otel-collector:
    environment:
      GOMEMLIMIT: "1000MiB"  # Reduce from 1750MiB
    deploy:
      resources:
        limits:
          memory: 1200M
```

3. **Reduce Batch Size**
Edit `otel/config/otel-collector-config.yaml`:
```yaml
processors:
  batch:
    send_batch_size: 500  # Reduce from 1000
    timeout: 5s          # Reduce from 10s
```

### 5. Missing Specific Metrics

#### Symptoms
- Some metrics not appearing
- Incomplete dashboard data
- Missing table/index metrics

#### Solutions

1. **Check Performance Schema**
```bash
# Verify Performance Schema is enabled
docker compose exec mysql-primary mysql -uotel_monitor -potelmonitorpass -e "SHOW VARIABLES LIKE 'performance_schema';"

# Check consumers
docker compose exec mysql-primary mysql -uotel_monitor -potelmonitorpass -e "SELECT * FROM performance_schema.setup_consumers WHERE ENABLED = 'YES';"
```

2. **Enable Missing Consumers**
```bash
docker compose exec mysql-primary mysql -uroot -prootpassword <<EOF
UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME LIKE '%statement%';
UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME LIKE 'statement/%';
UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME LIKE 'wait/io/table/%';
EOF
```

### 6. Slow Query Detection Not Working

#### Symptoms
- No slow queries reported
- `mysql.query.slow.count` always zero
- Missing query performance data

#### Solutions

1. **Check Slow Query Log Settings**
```bash
docker compose exec mysql-primary mysql -uotel_monitor -potelmonitorpass -e "SHOW VARIABLES LIKE '%slow%';"
```

2. **Adjust Slow Query Threshold**
```bash
docker compose exec mysql-primary mysql -uroot -prootpassword -e "SET GLOBAL long_query_time = 0.5;"
```

3. **Generate Test Slow Queries**
```bash
docker compose exec mysql-primary mysql -uappuser -papppassword ecommerce -e "CALL simulate_slow_query();"
```

### 7. Docker Compose Issues

#### Symptoms
- Services failing to start
- "Cannot find docker-compose.yml"
- Permission denied errors

#### Solutions

1. **Ensure Docker Compose V2**
```bash
# Check version
docker compose version

# If using old version, upgrade or use:
docker-compose up -d  # Note the hyphen
```

2. **Fix Permissions**
```bash
# Make scripts executable
chmod +x scripts/*.sh

# Fix MySQL init scripts
chmod 644 mysql/init/*.sql
```

3. **Clean Start**
```bash
# Remove all containers and volumes
docker compose down -v

# Remove old data
rm -rf mysql/data

# Fresh start
./scripts/setup.sh
```

### 8. New Relic Query Issues

#### Common NRQL Queries for Debugging

1. **Check if any metrics are arriving**
```sql
SELECT count(*) 
FROM Metric 
WHERE instrumentation.provider = 'opentelemetry' 
SINCE 5 minutes ago
```

2. **List all MySQL metrics**
```sql
SELECT uniques(metricName) 
FROM Metric 
WHERE metricName LIKE 'mysql.%' 
SINCE 1 hour ago
```

3. **Check collector status**
```sql
SELECT latest(timestamp) 
FROM Metric 
WHERE instrumentation.name = 'mysql-otel-collector' 
FACET mysql.instance.endpoint
```

### 9. Performance Issues

#### Symptoms
- High CPU usage
- Slow metric collection
- Delayed exports

#### Solutions

1. **Increase Collection Interval**
Edit `otel/config/otel-collector-config.yaml`:
```yaml
receivers:
  mysql/primary:
    collection_interval: 30s  # Increase from 10s
```

2. **Disable Expensive Metrics**
```yaml
metrics:
  mysql.table.io.wait.count:
    enabled: false  # Disable table I/O metrics
  mysql.index.io.wait.count:
    enabled: false  # Disable index I/O metrics
```

3. **Add Sampling**
```yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 10  # Only keep 10% of data points
```

## Getting Help

If these solutions don't resolve your issue:

1. **Collect Diagnostic Information**
```bash
# Save all logs
docker compose logs > diagnostics.log

# Get service status
docker compose ps >> diagnostics.log

# Get collector config
docker compose exec otel-collector cat /etc/otel-collector-config.yaml >> diagnostics.log
```

2. **Check Component Versions**
```bash
docker compose exec otel-collector /otelcol-contrib --version
docker compose exec mysql-primary mysql --version
```

3. **Contact Support**
- New Relic Support: https://support.newrelic.com/
- OpenTelemetry Community: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues