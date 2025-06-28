# Troubleshooting Guide

## Quick Diagnosis Flowchart

```
No data in New Relic?
├─> Check collector logs for errors
│   ├─> SQL errors? → Check prerequisites & credentials
│   ├─> Connection errors? → Verify network & endpoints
│   └─> Export errors? → Validate license key
│
├─> Check collector metrics (:8888/metrics)
│   ├─> No received logs? → Receiver configuration issue
│   ├─> No exported logs? → Pipeline or exporter issue
│   └─> Drops/errors? → Check processor logs
│
└─> Verify data flow
    ├─> Database → Can you run EXPLAIN manually?
    ├─> Collector → Is it running and healthy?
    └─> New Relic → Check ingestion limits
```

```
High resource usage?
├─> Memory issues
│   ├─> Check memory_limiter logs
│   ├─> Reduce batch sizes
│   └─> Lower sampling rates
│
├─> CPU issues
│   ├─> Increase collection interval
│   ├─> Reduce concurrent queries
│   └─> Disable expensive processors
│
└─> Storage issues
    ├─> Check disk usage
    ├─> Clear old state files
    └─> Reduce cache sizes
```

## Common Issues & Solutions

### Issue 1: No Plans Collected

**Symptoms**:
- Collector running but no data in New Relic
- No errors in logs
- Metrics show 0 received records

**Diagnosis**:
```bash
# 1. Test database connectivity
psql "$PG_REPLICA_DSN" -c "SELECT 1;"

# 2. Check pg_stat_statements
psql "$PG_REPLICA_DSN" -c "SELECT count(*) FROM pg_stat_statements;"

# 3. Verify EXPLAIN works
psql "$PG_REPLICA_DSN" -c "EXPLAIN (FORMAT JSON) SELECT 1;"

# 4. Check receiver logs specifically
kubectl logs nr-db-intel-0 | grep -i "sqlquery"
```

**Solutions**:

1. **pg_stat_statements not enabled**:
   ```sql
   -- As superuser
   CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

   -- Verify
   SELECT * FROM pg_stat_statements LIMIT 1;
   ```

2. **No slow queries to collect**:
   ```yaml
   # Lower threshold temporarily
   receivers:
     sqlquery:
       queries:
         - sql: |
             -- Reduced threshold
             WHERE mean_exec_time > 10  -- Was 100
   ```

3. **Collection interval too long**:
   ```yaml
   # Reduce for testing
   collection_interval: 10s  # Was 60s
   ```

### Issue 2: Connection Pool Exhaustion

**Symptoms**:
- "too many connections" errors
- Database connection limit reached
- Intermittent collection failures

**Diagnosis**:
```sql
-- PostgreSQL: Check connections
SELECT 
  usename,
  application_name,
  count(*),
  state
FROM pg_stat_activity
GROUP BY usename, application_name, state
ORDER BY count(*) DESC;

-- MySQL: Check connections
SHOW PROCESSLIST;
SELECT user, count(*) 
FROM information_schema.processlist 
GROUP BY user;
```

**Solutions**:

1. **Reduce connection pool size**:
   ```yaml
   receivers:
     sqlquery:
       # Add connection limits
       max_open_connections: 2
       max_idle_connections: 1
       connection_max_lifetime: 60s
   ```

2. **Use connection pooler**:
   ```yaml
   # Point to PgBouncer instead
   dsn: "postgres://user:pass@pgbouncer:6432/db"
   ```

3. **Increase database limit**:
   ```sql
   -- PostgreSQL
   ALTER SYSTEM SET max_connections = 200;
   -- Requires restart
   ```

### Issue 3: PII Leakage

**Symptoms**:
- Sensitive data visible in New Relic
- Email addresses in queries
- Customer IDs exposed

**Diagnosis**:
```bash
# Search for common PII patterns
kubectl logs nr-db-intel-0 | grep -E "([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})"

# Check sanitization processor
kubectl logs nr-db-intel-0 | grep "sanitize_pii"
```

**Solutions**:

1. **Add more patterns**:
   ```yaml
   processors:
     transform/sanitize_pii:
       log_statements:
         - context: log
           statements:
             # Add custom pattern
             - replace_pattern(
                 body,
                 "customer_id: \d+",
                 "customer_id: [REDACTED]"
               )
   ```

2. **Enable strict mode**:
   ```yaml
   # Remove all literals
   - replace_pattern(
       attributes["db.statement"],
       "'[^']*'",
       "'?'"
     )
   - replace_pattern(
       attributes["db.statement"],
       "\d+",
       "?"
     )
   ```

### Issue 4: State Corruption

**Symptoms**:
- Duplicate data after restart
- Inconsistent sampling
- "State file corrupted" errors

**Diagnosis**:
```bash
# Check state files
ls -la /var/lib/otel/storage/
file /var/lib/otel/storage/*

# Look for corruption errors
kubectl logs nr-db-intel-0 | grep -i "storage.*error"
```

**Solutions**:

1. **Clear state and restart**:
   ```bash
   # Scale down
   kubectl scale statefulset nr-db-intel --replicas=0

   # Clear state
   kubectl exec nr-db-intel-0 -- rm -rf /var/lib/otel/storage/*

   # Scale up
   kubectl scale statefulset nr-db-intel --replicas=1
   ```

2. **Disable state temporarily**:
   ```yaml
   processors:
     adaptive_sampler:
       # Disable deduplication
       deduplication:
         enabled: false
   ```

### Issue 5: Performance Degradation

**Symptoms**:
- Slow query processing
- Increasing latency
- Memory growth over time

**Diagnosis**:
```bash
# Check processing metrics
curl -s localhost:8888/metrics | grep -E "(processor_.*_duration|processor_.*_count)"

# Monitor memory growth
while true; do
  curl -s localhost:8888/metrics | grep "memory_heap_alloc"
  sleep 10
done
```

**Solutions**:

1. **Optimize regex patterns**:
   ```yaml
   # Use more specific patterns
   processors:
     transform/sanitize_pii:
       # Bad: .* 
       # Good: \b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b
   ```

2. **Reduce processing complexity**:
   ```yaml
   processors:
     plan_attribute_extractor:
       # Extract only essential attributes
       extractions:
         db.query.plan.cost: "$[0].Plan['Total Cost']"
         # Remove complex extractions
   ```

### Issue 6: Clock Skew

**Symptoms**:
- Future timestamps in data
- Correlation failures
- "Invalid timestamp" errors

**Diagnosis**:
```bash
# Check system time
date
ntpq -p

# Compare with database time
psql -c "SELECT now();"
```

**Solutions**:

1. **Enable NTP**:
   ```bash
   # On collector host
   systemctl enable ntpd
   systemctl start ntpd
   ```

2. **Add timestamp validation**:
   ```yaml
   processors:
     transform/validate_time:
       log_statements:
         - context: log
           statements:
             - set(timestamp, Now()) where timestamp > Now()
   ```

## Performance Tuning

### Memory Optimization

```yaml
# Reduce memory usage
processors:
  memory_limiter:
    limit_mib: 256  # Reduced from 512
    spike_limit_mib: 64  # Reduced from 128
    
  batch:
    send_batch_size: 50  # Reduced from 100
    timeout: 5s  # Reduced from 10s
    
  adaptive_sampler:
    deduplication:
      cache_size: 5000  # Reduced from 10000
```

### CPU Optimization

```yaml
# Reduce CPU usage
receivers:
  sqlquery:
    collection_interval: 120s  # Increased from 60s
    
processors:
  # Disable expensive processors
  # plan_context_enricher:
  #   enabled: false
```

### Network Optimization

```yaml
exporters:
  otlp:
    compression: gzip  # Ensure enabled
    
    # Batch more aggressively
    sending_queue:
      enabled: true
      queue_size: 200  # Increased
      
    # Retry less aggressively  
    retry_on_failure:
      max_elapsed_time: 60s  # Reduced from 300s
```

## Debug Tools

### Enable Debug Logging

```yaml
service:
  telemetry:
    logs:
      level: debug
      
  # Enable specific component debugging
  processors:
    plan_attribute_extractor:
      debug: true
      log_extracted_attributes: true
```

### Use zpages Extension

```yaml
extensions:
  zpages:
    endpoint: 0.0.0.0:55679
    
service:
  extensions: [zpages]
  
# Access at http://collector:55679/debug/tracez
```

### Manual Testing

```bash
# Test EXPLAIN manually
cat << 'EOF' > test_explain.sql
SET LOCAL statement_timeout = '2s';
EXPLAIN (FORMAT JSON) 
SELECT * FROM pg_stat_statements 
ORDER BY total_exec_time DESC 
LIMIT 1;
EOF

psql "$PG_REPLICA_DSN" -f test_explain.sql

# Test file log parsing
echo '2024-01-01 10:00:00 UTC [123]: duration: 150.5 ms  plan: {"Plan": {"Node Type": "Seq Scan"}}' \
  | otelcol --config=test-config.yaml
```

## Getting Help

### Before Asking for Help

1. Check all logs
2. Verify prerequisites
3. Test components individually
4. Simplify configuration
5. Check known issues

### Information to Provide

```bash
# Collector version
otelcol --version

# Configuration (sanitized)
cat config.yaml | sed 's/api-key:.*/api-key: [REDACTED]/'

# Recent logs
kubectl logs nr-db-intel-0 --tail=100

# Metrics snapshot
curl -s localhost:8888/metrics > metrics.txt

# Database version
psql -c "SELECT version();"
```

### Where to Get Help

- GitHub Issues (for bugs)
- Community Slack
- Support Ticket (customers)
- Stack Overflow (tagged 'opentelemetry')