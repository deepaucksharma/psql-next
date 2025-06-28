# Operations Guide

## Daily Operations

### Health Monitoring

#### Collector Health Metrics

Monitor these key metrics via the collector's self-telemetry:

```
# Collection success rate
otelcol_receiver_accepted_log_records / otelcol_receiver_refused_log_records

# Processing performance
otelcol_processor_batch_batch_send_size
otelcol_processor_batch_timeout_trigger_send

# Memory pressure
runtime_memory_heap_alloc_bytes / memory_limiter_limit

# State storage health
otelcol_file_storage_operation_duration
```

#### Database Impact Metrics

Critical metrics to monitor on your databases:

```sql
-- PostgreSQL: Check replica lag
SELECT 
  now() - pg_last_xact_replay_timestamp() AS replication_lag,
  pg_is_in_recovery() AS is_replica;

-- Check connection count
SELECT count(*) 
FROM pg_stat_activity 
WHERE usename = 'newrelic_monitor';

-- Monitor statement timeout impacts
SELECT count(*) 
FROM pg_stat_statements 
WHERE mean_exec_time > 2000  -- Our timeout threshold
  AND query LIKE '%EXPLAIN%';
```

```sql
-- MySQL: Check performance schema overhead
SELECT * FROM performance_schema.setup_consumers;

-- Monitor connection usage
SELECT user, count(*) 
FROM information_schema.processlist 
WHERE user = 'newrelic_monitor' 
GROUP BY user;
```

### Safety Checks

#### Pre-Operation Checklist

Before any configuration change:

1. **Verify Replica Status**
   ```bash
   # PostgreSQL
   psql -h $REPLICA_HOST -c "SELECT pg_is_in_recovery();"

   # MySQL
   mysql -h $REPLICA_HOST -e "SHOW SLAVE STATUS\G"
   ```

2. **Check Current Load**
   ```bash
   # Collector metrics endpoint
   curl -s http://collector:8888/metrics | grep -E "(memory|cpu|gc)"
   ```

3. **Validate Configuration**
   ```bash
   otelcol validate --config=new-config.yaml
   ```

### Emergency Stop Procedures

If database impact detected:

1. **Immediate Stop**
   ```bash
   # Kubernetes
   kubectl scale statefulset nr-db-intelligence --replicas=0

   # Docker
   docker stop nr-db-intelligence-collector
   ```

2. **Increase Collection Interval**
   ```yaml
   # Emergency config - 5 minute interval
   receivers:
     sqlquery:
       collection_interval: 300s
   ```

3. **Disable Problematic Queries**
   ```yaml
   # Comment out expensive receivers
   receivers:
     # sqlquery/postgresql_plans_safe:
     #   ...
   ```

## Troubleshooting Guide

### Issue: No Data in New Relic

**Diagnosis Steps**:

1. **Check collector logs**
   ```bash
   kubectl logs -f nr-db-intelligence-0
   # Look for "Exporting logs" messages
   ```

2. **Verify receiver execution**
   ```bash
   # Check for SQL errors
   grep -i "error.*sql" /var/log/otel/collector.log
   ```

3. **Validate credentials**
   ```bash
   # Test database connection
   psql "$PG_REPLICA_DSN" -c "SELECT 1;"
   ```

4. **Check New Relic ingestion**
   ```bash
   # Look for 4xx/5xx responses
   grep "otlp.*response" /var/log/otel/collector.log
   ```

### Issue: High Memory Usage

**Symptoms**:
- OOMKilled pods
- Slow processing
- Dropped data

**Solutions**:

1. **Reduce batch size**
   ```yaml
   processors:
     batch:
       send_batch_size: 50  # Reduced from 100
   ```

2. **Increase memory limit interval**
   ```yaml
   processors:
     memory_limiter:
       check_interval: 500ms  # More frequent checks
   ```

3. **Lower sampling rates**
   ```yaml
   processors:
     adaptive_sampler:
       rules:
         - name: default
           sample_rate: 0.01  # Reduced from 0.1
   ```

### Issue: State Storage Problems

**Symptoms**:
- Duplicate data
- Inconsistent sampling
- "File storage error" logs

**Solutions**:

1. **Check disk space**
   ```bash
   df -h /var/lib/otel/storage
   ```

2. **Clear corrupted state**
   ```bash
   # Stop collector first!
   rm -rf /var/lib/otel/storage/*
   ```

3. **Verify permissions**
   ```bash
   ls -la /var/lib/otel/storage
   # Should be writable by UID 10001
   ```

## Performance Tuning

### Collector Optimization

**CPU Optimization**:
```yaml
service:
  telemetry:
    metrics:
      level: basic  # Reduce from "detailed"
```

**Memory Optimization**:
```yaml
processors:
  batch:
    timeout: 5s  # Reduce from 10s
    send_batch_max_size: 150  # Reduce from 200
```

**Network Optimization**:
```yaml
exporters:
  otlp:
    compression: gzip  # Already optimal
    sending_queue:
      queue_size: 50  # Reduce from 100
```

### Database Query Optimization

**PostgreSQL: Use Prepared Statements**
```sql
-- Create prepared statement for reuse
PREPARE explain_plan (text) AS
SELECT pg_get_json_plan($1);
```

**MySQL: Optimize Performance Schema**
```sql
-- Limit history size
UPDATE performance_schema.setup_consumers
SET ENABLED = 'NO'
WHERE NAME NOT LIKE '%current%'
  AND NAME NOT LIKE '%digest%';
```

## Maintenance Windows

### Weekly Tasks

1. **State Storage Cleanup**
   ```bash
   # During low-activity period
   find /var/lib/otel/storage -name "*.tmp" -mtime +7 -delete
   ```

2. **Log Rotation Verification**
   ```bash
   # Ensure logs are rotating
   ls -la /var/log/otel/*.gz | wc -l
   ```

3. **Metrics Review**
   - Collection success rate trend
   - Memory usage pattern
   - Database impact assessment

### Monthly Tasks

1. **Configuration Review**
   - Update sampling rules based on data volume
   - Adjust intervals based on value received
   - Review PII sanitization effectiveness

2. **Security Audit**
   - Rotate database passwords
   - Review access logs
   - Update collector image

3. **Capacity Planning**
   - Project storage growth
   - Plan for additional databases
   - Review resource utilization

## Incident Response

### Playbook: Database Performance Impact

**Trigger**: Database CPU spike correlated with collection

**Response**:
1. Immediately scale collector to 0
2. Check pg_stat_statements for EXPLAIN queries
3. Increase statement_timeout in config
4. Restart with longer collection_interval
5. Gradually reduce interval while monitoring

### Playbook: Data Quality Issues

**Trigger**: Missing or corrupted plans in New Relic

**Response**:
1. Check sanitization processor logs
2. Verify plan_attribute_extractor errors
3. Review sample of raw plans
4. Adjust parsing rules if needed
5. Clear state storage if corrupted

## Monitoring Dashboard

Key panels for your operational dashboard:

1. **Collection Health**
   - Plans collected per minute
   - Collection success rate
   - Error types and frequency

2. **Resource Usage**
   - Collector CPU and memory
   - State storage size
   - Network bandwidth

3. **Database Impact**
   - Replica lag trend
   - Connection count
   - Query execution time

4. **Data Quality**
   - Sampling rates
   - Deduplication effectiveness
   - Parse failure rate