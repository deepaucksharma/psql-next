# Troubleshooting Guide

## Overview

Comprehensive troubleshooting guide for the Database Intelligence OpenTelemetry Collector covering common issues, diagnostic procedures, and resolution steps.

## Quick Diagnostics

### Health Check
```bash
# Basic health check
curl http://localhost:13133/health
# Expected: HTTP 200 with "OK" or health status

# Detailed health status
curl http://localhost:13133/status
```

### Metrics Verification
```bash
# Check if metrics are being collected
curl http://localhost:8888/metrics | grep postgresql
curl http://localhost:8888/metrics | grep mysql

# Count metric types
curl -s http://localhost:8888/metrics | grep -E "^postgresql_|^mysql_" | wc -l

# Check for errors
curl -s http://localhost:8888/metrics | grep -i error
```

### Log Analysis
```bash
# View recent logs
tail -f collector.log

# Search for errors
grep -i error collector.log | tail -20
grep -i "connection refused" collector.log
grep -i "authentication failed" collector.log
```

## Common Issues

### 1. No Metrics in New Relic

#### Symptoms
- Health check passes
- Local metrics visible
- No data in New Relic dashboards

#### Diagnosis
```bash
# Check New Relic connectivity
curl -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  https://api.newrelic.com/v2/applications.json

# Verify license key format
echo $NEW_RELIC_LICENSE_KEY | wc -c
# Should be 40 characters + newline

# Check OTLP export logs
grep -i "otlp" collector.log | tail -10
grep -i "403\|401\|400" collector.log
```

#### Resolution
```bash
# 1. Verify license key
export NEW_RELIC_LICENSE_KEY="your_correct_license_key"

# 2. Check account ID
export NEW_RELIC_ACCOUNT_ID="your_account_id"

# 3. Test with debug exporter
# Add to config:
exporters:
  debug:
    verbosity: detailed
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      exporters: [debug, otlp]  # Add debug first
```

### 2. Database Connection Failed

#### Symptoms
```
connection refused
authentication failed  
timeout connecting to database
```

#### Diagnosis
```bash
# Test database connectivity
psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT 1"
mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASSWORD -e "SELECT 1"

# Check network connectivity
telnet $POSTGRES_HOST $POSTGRES_PORT
nc -zv $MYSQL_HOST $MYSQL_PORT

# Verify credentials
echo "Host: $POSTGRES_HOST, Port: $POSTGRES_PORT, User: $POSTGRES_USER"
echo "Password length: $(echo $POSTGRES_PASSWORD | wc -c)"
```

#### Resolution
```yaml
# 1. Fix authentication
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}  # Ensure correct password
    ssl_mode: disable  # Or 'require' if SSL needed

# 2. Add connection timeout
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 10s
    timeout: 30s  # Add timeout

# 3. For Docker environments
# Use service names instead of localhost
postgresql:
  endpoint: postgres:5432  # Service name in Docker Compose
```

### 3. High Memory Usage

#### Symptoms
- Memory usage > 1GB
- Out of memory errors
- Container restarts

#### Diagnosis
```bash
# Check memory usage
ps aux | grep database-intelligence-collector
top -p $(pgrep database-intelligence-collector)

# Memory metrics
curl -s http://localhost:8888/metrics | grep memory
curl -s http://localhost:8888/metrics | grep process_resident_memory_bytes
```

#### Resolution
```yaml
# 1. Add memory limiter
processors:
  memory_limiter:
    limit_mib: 512        # Set appropriate limit
    spike_limit_mib: 128  # 25% of limit
    check_interval: 1s

# 2. Optimize batch processing
processors:
  batch:
    timeout: 5s           # Faster batching
    send_batch_size: 1024 # Smaller batches
    send_batch_max_size: 2048

# 3. Reduce collection frequency
receivers:
  postgresql:
    collection_interval: 30s  # Increase interval
  sqlquery/slow_queries:
    collection_interval: 60s  # Less frequent for expensive queries
```

### 4. Slow Query Collection Not Working

#### Symptoms
- No `postgres.slow_queries.*` metrics
- Empty query results in logs

#### Diagnosis
```bash
# Check if pg_stat_statements is installed
psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT * FROM pg_extension WHERE extname = 'pg_stat_statements'"

# Check if there are slow queries
psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT count(*) FROM pg_stat_statements WHERE mean_exec_time > 100"

# Test the SQL query manually
psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -c "
SELECT 
  queryid::text as query_id,
  query as query_text,
  datname as database_name,
  calls as execution_count,
  mean_exec_time as avg_elapsed_time_ms
FROM pg_stat_statements pss
JOIN pg_database pd ON pd.oid = pss.dbid
WHERE mean_exec_time > 100
LIMIT 5"
```

#### Resolution
```sql
-- 1. Install pg_stat_statements extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- 2. Add to postgresql.conf
shared_preload_libraries = 'pg_stat_statements'
pg_stat_statements.track = all

-- 3. Restart PostgreSQL and verify
SELECT * FROM pg_stat_statements LIMIT 1;
```

```yaml
# 4. Lower threshold for testing
sqlquery/slow_queries:
  queries:
    - sql: |
        SELECT ...
        WHERE mean_exec_time > 1  -- Lower threshold
```

### 5. Wait Events Not Captured

#### Symptoms
- No `postgres.wait_events` metrics
- Empty wait event data

#### Diagnosis
```bash
# Check for active sessions with wait events
psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -c "
SELECT 
  wait_event, 
  wait_event_type, 
  count(*) 
FROM pg_stat_activity 
WHERE state = 'active' 
GROUP BY wait_event, wait_event_type"
```

#### Resolution
```sql
-- Generate wait events for testing
SELECT pg_sleep(2), count(*) FROM pg_stat_activity;
```

```yaml
# Adjust collection interval
sqlquery/wait_events:
  collection_interval: 5s  # More frequent collection
  queries:
    - sql: |
        SELECT 
          COALESCE(wait_event, 'CPU') as wait_event_name,
          COALESCE(wait_event_type, 'CPU') as wait_category,
          COUNT(*) as count,
          datname as database_name
        FROM pg_stat_activity 
        WHERE state IN ('active', 'idle in transaction')  -- Include more states
        GROUP BY wait_event, wait_event_type, datname
```

### 6. Configuration Validation Issues

#### Symptoms
```
invalid configuration
unknown receiver
processor not found
```

#### Diagnosis
```bash
# Validate configuration syntax
./database-intelligence-collector --config=config.yaml --dry-run

# Check for typos in component names
grep -n "postgresql" config.yaml
grep -n "receivers:" config.yaml
grep -n "processors:" config.yaml
```

#### Resolution
```yaml
# 1. Verify component names match exactly
receivers:
  postgresql:  # Not 'postgres'
    endpoint: localhost:5432

# 2. Check pipeline references
service:
  pipelines:
    metrics:
      receivers: [postgresql]     # Must match receiver name
      processors: [batch]        # Must match processor name
      exporters: [otlp]          # Must match exporter name

# 3. Validate YAML syntax
# Use a YAML validator or:
python -c "import yaml; yaml.safe_load(open('config.yaml'))"
```

## Performance Issues

### 7. High CPU Usage

#### Symptoms
- CPU usage > 80%
- Slow response times
- Processing delays

#### Diagnosis
```bash
# Check CPU usage
top -p $(pgrep database-intelligence-collector)

# Check processing metrics
curl -s http://localhost:8888/metrics | grep otelcol_processor
curl -s http://localhost:8888/metrics | grep duration
```

#### Resolution
```yaml
# 1. Reduce collection frequency
receivers:
  postgresql:
    collection_interval: 20s  # Increase interval
  sqlquery/slow_queries:
    collection_interval: 60s

# 2. Optimize processors
processors:
  batch:
    timeout: 1s           # Faster batching
    send_batch_size: 2048 # Larger batches

# 3. Add sampling
processors:
  probabilistic_sampler:
    sampling_percentage: 50  # Sample 50% of metrics
```

### 8. Network Connectivity Issues

#### Symptoms
```
connection timeout
network unreachable
DNS resolution failed
```

#### Diagnosis
```bash
# Test network connectivity
ping $POSTGRES_HOST
telnet $POSTGRES_HOST $POSTGRES_PORT

# Check DNS resolution
nslookup $POSTGRES_HOST
dig $POSTGRES_HOST

# Test New Relic connectivity
curl -v https://otlp.nr-data.net:4317
```

#### Resolution
```yaml
# 1. Add timeouts and retries
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    timeout: 30s

exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    timeout: 30s
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

# 2. For container environments
# Use fully qualified domain names
postgresql:
  endpoint: postgres.database.svc.cluster.local:5432
```

## Docker-Specific Issues

### 9. Container Startup Issues

#### Symptoms
- Container exits immediately
- Health check fails
- Port binding errors

#### Diagnosis
```bash
# Check container logs
docker logs database-intelligence-collector

# Check container status
docker ps -a | grep database-intelligence

# Verify port availability
netstat -tulpn | grep :13133
```

#### Resolution
```bash
# 1. Check port conflicts
docker run -p 13134:13133 database-intelligence:latest  # Use different port

# 2. Verify environment variables
docker run --env-file .env database-intelligence:latest

# 3. Debug mode
docker run -it --entrypoint /bin/sh database-intelligence:latest
```

### 10. Docker Compose Issues

#### Symptoms
```
service 'postgres' failed to build
network not found
volume mount failed
```

#### Diagnosis
```bash
# Check Docker Compose logs
docker-compose logs database-intelligence
docker-compose logs postgres

# Verify network connectivity
docker-compose exec database-intelligence ping postgres
```

#### Resolution
```yaml
# 1. Fix service dependencies
services:
  database-intelligence:
    depends_on:
      postgres:
        condition: service_healthy
      mysql:
        condition: service_healthy

# 2. Use correct service names
environment:
  POSTGRES_HOST: postgres  # Service name, not localhost
  MYSQL_HOST: mysql

# 3. Add health checks
postgres:
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U postgres"]
    interval: 30s
    timeout: 10s
    retries: 3
```

## Kubernetes-Specific Issues

### 11. Pod CrashLoopBackOff

#### Symptoms
- Pod continuously restarting
- CrashLoopBackOff status
- Failed readiness/liveness probes

#### Diagnosis
```bash
# Check pod status
kubectl get pods -l app=database-intelligence

# Check events
kubectl describe pod -l app=database-intelligence

# Check logs
kubectl logs -l app=database-intelligence --previous
```

#### Resolution
```yaml
# 1. Increase probe timeouts
livenessProbe:
  httpGet:
    path: /health
    port: 13133
  initialDelaySeconds: 60  # Increase delay
  periodSeconds: 30
  timeoutSeconds: 10
  failureThreshold: 5      # Increase threshold

# 2. Check resource limits
resources:
  requests:
    memory: 512Mi
    cpu: 500m
  limits:
    memory: 2Gi           # Increase limits
    cpu: 1000m

# 3. Debug with sleep
containers:
- name: collector
  command: ["sleep", "3600"]  # Debug mode
```

### 12. Secret/ConfigMap Issues

#### Symptoms
```
secret not found
permission denied
configuration error
```

#### Diagnosis
```bash
# Check secrets
kubectl get secrets -n database-intelligence
kubectl describe secret db-intelligence-secrets

# Check configmap
kubectl get configmap db-intelligence-config -o yaml

# Verify RBAC
kubectl auth can-i get secrets --as=system:serviceaccount:database-intelligence:database-intelligence
```

#### Resolution
```bash
# 1. Create missing secrets
kubectl create secret generic db-intelligence-secrets \
  --from-literal=new-relic-license-key="your_key" \
  --from-literal=postgres-password="your_password"

# 2. Verify secret mounting
kubectl exec -it deployment/database-intelligence -- env | grep NEW_RELIC

# 3. Check RBAC permissions
kubectl apply -f rbac.yaml
```

## Debug Tools and Commands

### Comprehensive Debug Script
```bash
#!/bin/bash
# debug-collector.sh

echo "=== Database Intelligence Collector Debug ==="

echo "1. Health Check"
curl -s http://localhost:13133/health || echo "Health check failed"

echo -e "\n2. Metrics Summary"
METRICS=$(curl -s http://localhost:8888/metrics)
echo "Total metrics: $(echo "$METRICS" | wc -l)"
echo "PostgreSQL metrics: $(echo "$METRICS" | grep -c postgresql)"
echo "MySQL metrics: $(echo "$METRICS" | grep -c mysql)"

echo -e "\n3. Database Connectivity"
pg_isready -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER || echo "PostgreSQL not ready"
mysqladmin ping -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASSWORD || echo "MySQL not ready"

echo -e "\n4. New Relic Connectivity"
curl -s -o /dev/null -w "%{http_code}" -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  https://api.newrelic.com/v2/applications.json || echo "New Relic API failed"

echo -e "\n5. Recent Logs"
tail -20 collector.log

echo -e "\n6. Resource Usage"
ps aux | grep database-intelligence-collector | grep -v grep

echo "=== Debug Complete ==="
```

### Log Analysis Script
```bash
#!/bin/bash
# analyze-logs.sh

LOG_FILE=${1:-collector.log}

echo "=== Log Analysis for $LOG_FILE ==="

echo "Error Summary:"
grep -i error "$LOG_FILE" | tail -10

echo -e "\nConnection Issues:"
grep -i "connection\|timeout\|refused" "$LOG_FILE" | tail -5

echo -e "\nAuthentication Issues:"
grep -i "auth\|permission\|denied" "$LOG_FILE" | tail -5

echo -e "\nNew Relic Export Issues:"
grep -i "otlp\|export\|403\|401" "$LOG_FILE" | tail -5

echo -e "\nPerformance Issues:"
grep -i "memory\|cpu\|timeout" "$LOG_FILE" | tail -5
```

This troubleshooting guide provides comprehensive diagnostic and resolution procedures for all common issues encountered with the Database Intelligence OpenTelemetry Collector.