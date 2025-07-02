# Troubleshooting Guide - Database Intelligence Collector

## Table of Contents

1. [Common Issues](#common-issues)
2. [Installation Problems](#installation-problems)
3. [Connection Issues](#connection-issues)
4. [Data Collection Issues](#data-collection-issues)
5. [Performance Problems](#performance-problems)
6. [New Relic Integration Issues](#new-relic-integration-issues)
7. [pg_querylens Issues](#pg_querylens-issues)
8. [Processor-Specific Issues](#processor-specific-issues)
9. [Kubernetes Deployment Issues](#kubernetes-deployment-issues)
10. [Debugging Tools](#debugging-tools)
11. [Support and Resources](#support-and-resources)

## Common Issues

### Collector Won't Start

**Symptoms:**
- Container crashes immediately
- Exit code 1 or segmentation fault
- No logs produced

**Solutions:**

1. **Check configuration syntax:**
```bash
# Validate configuration
./database-intelligence-collector validate --config=config.yaml

# Common syntax errors:
# - Missing colons after keys
# - Incorrect indentation (use spaces, not tabs)
# - Unclosed quotes
```

2. **Verify environment variables:**
```bash
# List all required environment variables
env | grep -E 'POSTGRES_|MYSQL_|NEW_RELIC_'

# Ensure no variables are empty
if [ -z "$NEW_RELIC_LICENSE_KEY" ]; then
  echo "ERROR: NEW_RELIC_LICENSE_KEY is not set"
fi
```

3. **Check memory limits:**
```yaml
# Increase memory if collector is OOM killed
resources:
  limits:
    memory: 1Gi  # Increase from default 512Mi
  requests:
    memory: 512Mi
```

### High Memory Usage

**Symptoms:**
- Memory usage > 80% of limit
- Frequent garbage collection
- OOMKilled events

**Solutions:**

1. **Enable memory limiter processor:**
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 450  # Leave headroom below container limit
    spike_limit_mib: 100
```

2. **Reduce batch sizes:**
```yaml
processors:
  batch:
    send_batch_size: 500  # Reduce from 1000
    timeout: 5s  # Reduce from 10s
```

3. **Increase sampling:**
```yaml
processors:
  adaptivesampler:
    default_sampling_rate: 0.1  # Sample only 10%
```

### Missing Metrics

**Symptoms:**
- No data in New Relic
- Partial metrics only
- Gaps in data

**Solutions:**

1. **Check collector logs:**
```bash
kubectl logs -f deployment/database-intelligence -n database-intelligence | grep ERROR
```

2. **Verify receivers are working:**
```bash
# Check internal metrics
curl http://localhost:8888/metrics | grep receiver_accepted_metric_points
```

3. **Enable debug exporter temporarily:**
```yaml
exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 100

service:
  pipelines:
    metrics/debug:
      receivers: [postgresql]
      processors: [batch]
      exporters: [debug]
```

## Installation Problems

### Extension Installation Failed

**Error:** `ERROR: could not open extension control file "/usr/share/postgresql/14/extension/pg_querylens.control"`

**Solution:**
```bash
# Install pg_querylens from source
git clone https://github.com/pgexperts/pg_querylens.git
cd pg_querylens
make && sudo make install

# Or use package manager
sudo apt-get install postgresql-14-querylens
```

### Permission Denied

**Error:** `permission denied for schema pg_catalog`

**Solution:**
```sql
-- Grant required permissions
GRANT pg_monitor TO monitoring;
GRANT USAGE ON SCHEMA pg_catalog TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO monitoring;

-- For pg_querylens
GRANT USAGE ON SCHEMA pg_querylens TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_querylens TO monitoring;
```

## Connection Issues

### Database Connection Refused

**Error:** `dial tcp: connect: connection refused`

**Solutions:**

1. **Check database is running:**
```bash
# PostgreSQL
systemctl status postgresql
pg_isready -h localhost -p 5432

# MySQL
systemctl status mysql
mysqladmin ping -h localhost
```

2. **Verify network connectivity:**
```bash
# From collector pod
kubectl exec -it deployment/database-intelligence -- nc -zv postgres-host 5432
```

3. **Check firewall rules:**
```bash
# Allow collector IP
sudo ufw allow from <collector-ip> to any port 5432
```

4. **Verify pg_hba.conf:**
```bash
# Add entry for monitoring user
host    all    monitoring    10.0.0.0/8    scram-sha-256
```

### SSL/TLS Issues

**Error:** `SSL connection required`

**Solutions:**

1. **Configure SSL mode:**
```yaml
receivers:
  postgresql:
    tls:
      insecure: false
      ca_file: /etc/ssl/certs/ca-certificates.crt
      cert_file: /etc/ssl/certs/client-cert.pem
      key_file: /etc/ssl/private/client-key.pem
```

2. **For testing only - disable SSL:**
```yaml
receivers:
  postgresql:
    sslmode: disable  # NOT for production
```

### Authentication Failed

**Error:** `password authentication failed for user "monitoring"`

**Solutions:**

1. **Reset password:**
```sql
ALTER USER monitoring WITH PASSWORD 'new_secure_password';
```

2. **Check authentication method:**
```sql
-- View pg_hba.conf settings
SELECT * FROM pg_hba_file_rules;
```

3. **Use environment variable:**
```yaml
receivers:
  postgresql:
    password: ${POSTGRES_PASSWORD}
```

## Data Collection Issues

### No pg_stat_statements Data

**Error:** `relation "pg_stat_statements" does not exist`

**Solution:**
```sql
-- Enable extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Configure postgresql.conf
shared_preload_libraries = 'pg_stat_statements'
pg_stat_statements.track = all
pg_stat_statements.max = 10000

-- Restart PostgreSQL
sudo systemctl restart postgresql
```

### Incomplete Metrics

**Symptom:** Some metrics missing

**Solutions:**

1. **Check metric configuration:**
```yaml
receivers:
  postgresql:
    metrics:
      postgresql.database.size:
        enabled: true
      postgresql.connections:
        enabled: true
```

2. **Verify permissions:**
```sql
-- Test as monitoring user
SET ROLE monitoring;
SELECT * FROM pg_stat_database LIMIT 1;
SELECT * FROM pg_stat_activity LIMIT 1;
```

### Stale Data

**Symptom:** Metrics not updating

**Solutions:**

1. **Check collection interval:**
```yaml
receivers:
  postgresql:
    collection_interval: 10s  # Not too frequent
```

2. **Verify circuit breaker state:**
```bash
# Check if circuit breaker is open
curl http://localhost:8888/metrics | grep circuit_breaker_state
```

## Performance Problems

### Slow Query Collection

**Symptom:** Collection takes > 10 seconds

**Solutions:**

1. **Optimize queries:**
```sql
-- Add index for pg_stat_statements
CREATE INDEX CONCURRENTLY idx_pg_stat_statements_total_time 
ON pg_stat_statements(total_exec_time DESC);
```

2. **Limit query results:**
```yaml
sqlquery:
  queries:
    - query: |
        SELECT * FROM pg_stat_statements 
        ORDER BY total_exec_time DESC 
        LIMIT 100  -- Reduce from 1000
```

3. **Increase timeout:**
```yaml
sqlquery:
  timeout: 30s  # Increase from default 10s
```

### High CPU Usage

**Solutions:**

1. **Reduce processing load:**
```yaml
processors:
  adaptivesampler:
    default_sampling_rate: 0.1
    
  batch:
    send_batch_size: 500
    timeout: 10s
```

2. **Disable expensive processors:**
```yaml
# Temporarily disable plan extraction
processors:
  # planattributeextractor:  # Comment out
```

3. **Profile CPU usage:**
```bash
# Enable pprof
curl http://localhost:1777/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

## New Relic Integration Issues

### No Data in New Relic

**Solutions:**

1. **Verify license key:**
```bash
# Test API key
curl -X POST https://api.newrelic.com/graphql \
  -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  -d '{"query": "{ actor { user { name } } }"}'
```

2. **Check OTLP endpoint:**
```yaml
exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317  # US datacenter
    # endpoint: otlp.eu01.nr-data.net:4317  # EU datacenter
```

3. **Enable retry and logging:**
```yaml
exporters:
  otlp:
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_elapsed_time: 300s
    sending_queue:
      enabled: true
      queue_size: 1000
```

### NrIntegrationError

**Error:** `NrIntegrationError: Attribute limit exceeded`

**Solutions:**

1. **Enable NR error monitor:**
```yaml
processors:
  nrerrormonitor:
    enabled: true
    max_attribute_length: 4095
    max_attributes_per_event: 254
```

2. **Reduce cardinality:**
```yaml
processors:
  verification:
    cardinality_protection:
      enabled: true
      max_unique_queries: 5000
```

### Data Rejected

**Error:** `413 Request Entity Too Large`

**Solutions:**

1. **Reduce batch size:**
```yaml
processors:
  batch:
    send_batch_size: 100  # Reduce significantly
    send_batch_max_size: 200
```

2. **Enable compression:**
```yaml
exporters:
  otlp:
    compression: gzip
```

## pg_querylens Issues

### No pg_querylens Data

**Solutions:**

1. **Verify extension is enabled:**
```sql
-- Check extension
SELECT * FROM pg_extension WHERE extname = 'pg_querylens';

-- Check configuration
SHOW pg_querylens.enabled;
SHOW shared_preload_libraries;
```

2. **Grant permissions:**
```sql
GRANT USAGE ON SCHEMA pg_querylens TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_querylens TO monitoring;
```

3. **Check data collection:**
```sql
-- Should return rows
SELECT COUNT(*) FROM pg_querylens.queries;
SELECT COUNT(*) FROM pg_querylens.plans;
```

### Plan Text Too Large

**Error:** `plan text exceeds maximum length`

**Solutions:**

1. **Increase plan size limit:**
```sql
ALTER SYSTEM SET pg_querylens.max_plan_length = 20000;
SELECT pg_reload_conf();
```

2. **Configure processor limit:**
```yaml
processors:
  planattributeextractor:
    max_plan_size: 20480  # 20KB
```

### Missing Plan Changes

**Solutions:**

1. **Check plan history window:**
```yaml
processors:
  planattributeextractor:
    querylens:
      plan_history_hours: 48  # Increase from 24
```

2. **Verify plan tracking:**
```sql
-- Check for multiple plans per query
SELECT queryid, COUNT(DISTINCT plan_id) 
FROM pg_querylens.plans 
GROUP BY queryid 
HAVING COUNT(DISTINCT plan_id) > 1;
```

## Processor-Specific Issues

### Adaptive Sampler Not Working

**Solutions:**

1. **Check rule syntax:**
```yaml
processors:
  adaptivesampler:
    rules:
      - name: slow_queries
        expression: 'attributes["duration"] > 1000'  # CEL expression
        sample_rate: 1.0
```

2. **Enable debug logging:**
```yaml
processors:
  adaptivesampler:
    debug: true  # Log rule evaluations
```

### Circuit Breaker Always Open

**Solutions:**

1. **Check failure threshold:**
```yaml
processors:
  circuitbreaker:
    failure_threshold: 0.5  # 50% failure rate
    success_threshold: 2    # Need 2 successes to close
```

2. **Increase timeout:**
```yaml
processors:
  circuitbreaker:
    timeout: 60s  # Increase from 30s
    recovery_timeout: 300s  # 5 minutes
```

### PII Not Being Redacted

**Solutions:**

1. **Check patterns:**
```yaml
processors:
  verification:
    pii_detection:
      enabled: true
      patterns:
        - ssn
        - credit_card
        - email
      custom_patterns:
        - name: employee_id
          pattern: 'EMP[0-9]{6}'
```

2. **Verify action:**
```yaml
processors:
  verification:
    pii_detection:
      action: redact  # Not 'drop' or 'hash'
```

## Kubernetes Deployment Issues

### Pod Stuck in Pending

**Solutions:**

1. **Check resource availability:**
```bash
kubectl describe pod <pod-name> -n database-intelligence
kubectl get events -n database-intelligence
```

2. **Verify node selectors:**
```yaml
# Remove if nodes don't have labels
nodeSelector: {}
# nodeSelector:
#   node-role: monitoring
```

3. **Check PVC:**
```bash
kubectl get pvc -n database-intelligence
kubectl describe pvc <pvc-name> -n database-intelligence
```

### CrashLoopBackOff

**Solutions:**

1. **Check logs:**
```bash
kubectl logs <pod-name> -n database-intelligence --previous
```

2. **Increase startup probe:**
```yaml
startupProbe:
  initialDelaySeconds: 60  # More time to start
  failureThreshold: 30
```

3. **Check security context:**
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 10001  # Ensure user exists in image
```

### Service Not Accessible

**Solutions:**

1. **Check service endpoints:**
```bash
kubectl get endpoints -n database-intelligence
kubectl get svc -n database-intelligence
```

2. **Test connectivity:**
```bash
# Port forward for testing
kubectl port-forward svc/database-intelligence 8888:8888 -n database-intelligence

# Test metrics endpoint
curl http://localhost:8888/metrics
```

## Debugging Tools

### Enable Debug Logging

```yaml
service:
  telemetry:
    logs:
      level: debug
      development: true
```

### Use Debug Exporter

```yaml
exporters:
  debug:
    verbosity: detailed
    
service:
  pipelines:
    metrics/debug:
      receivers: [postgresql]
      exporters: [debug]
```

### Internal Metrics

```bash
# Collector metrics
curl http://localhost:8888/metrics

# Prometheus metrics
curl http://localhost:8889/metrics

# Health check
curl http://localhost:13133/health

# pprof debugging
curl http://localhost:1777/debug/pprof/
```

### Test Queries

```sql
-- Test monitoring user access
SET ROLE monitoring;

-- Test pg_stat views
SELECT * FROM pg_stat_database LIMIT 1;
SELECT * FROM pg_stat_activity LIMIT 1;
SELECT * FROM pg_stat_statements LIMIT 1;

-- Test pg_querylens
SELECT * FROM pg_querylens.queries LIMIT 1;
SELECT * FROM pg_querylens.plans LIMIT 1;
```

## Support and Resources

### Documentation
- [Configuration Guide](CONFIGURATION.md)
- [Architecture Overview](ARCHITECTURE.md)
- [pg_querylens Integration](PG_QUERYLENS_INTEGRATION.md)

### Community Support
- GitHub Issues: https://github.com/database-intelligence-mvp/database-intelligence-collector/issues
- New Relic Explorers Hub: https://discuss.newrelic.com

### Debug Checklist

When reporting issues, include:
- [ ] Collector version
- [ ] Configuration file (sanitized)
- [ ] Error messages from logs
- [ ] Output of `curl http://localhost:8888/metrics`
- [ ] Database version and extensions
- [ ] Kubernetes events and pod describe output

### Emergency Contacts
- On-call: oncall@database-intelligence.io
- Slack: #database-intelligence-support