# MySQL Wait-Based Monitoring - Operational Runbooks

## Table of Contents
- [Common Issues and Solutions](#common-issues-and-solutions)
- [Performance Tuning Guide](#performance-tuning-guide)
- [Troubleshooting Procedures](#troubleshooting-procedures)
- [Emergency Response](#emergency-response)
- [Maintenance Procedures](#maintenance-procedures)

## Common Issues and Solutions

### 1. High Wait Percentage Alert

**Symptoms:**
- Alert: "Query Wait Time > 80%"
- User complaints about slow queries
- Increased response times

**Diagnosis Steps:**
```bash
# 1. Check wait category breakdown
./scripts/monitor-waits.sh waits

# 2. Identify specific queries
curl -s http://localhost:9091/metrics | grep 'wait_severity="critical"' | sort -k2 -nr

# 3. Review advisories
curl -s http://localhost:9091/metrics | grep 'advisor_type=' | grep -v '#'
```

**Solutions by Wait Category:**

#### I/O Waits
```sql
-- Check for missing indexes
SELECT 
    query_hash,
    advisor.recommendation
FROM new_relic_metrics
WHERE advisor.type = 'missing_index'
    AND wait.category = 'io'
ORDER BY wait_percentage DESC;

-- Add recommended index
ALTER TABLE table_name ADD INDEX idx_name (column1, column2);
```

#### Lock Waits
```sql
-- Find blocking sessions
SELECT * FROM information_schema.innodb_trx
WHERE trx_state = 'LOCK WAIT';

-- Kill blocking session if necessary
KILL CONNECTION <blocking_thread_id>;

-- Review transaction isolation
SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;
```

#### CPU Waits
```bash
# Check system load
top -n 1 | head -5

# Review query complexity
mysql -e "SELECT * FROM performance_schema.events_statements_summary_by_digest 
         WHERE AVG_TIMER_WAIT > 1000000000 
         ORDER BY SUM_SORT_SCAN DESC LIMIT 10"
```

### 2. Plan Regression Detected

**Symptoms:**
- Alert: "Query Plan Regression"
- Query suddenly 5x slower
- Plan fingerprint changed

**Diagnosis:**
```bash
# 1. Compare metrics
curl -s http://localhost:9091/metrics | grep '<query_hash>' | grep -E 'rows_examined|full_scans'

# 2. Check table statistics
mysql -e "SELECT * FROM information_schema.tables 
         WHERE table_schema = 'your_db' 
         AND update_time < DATE_SUB(NOW(), INTERVAL 7 DAY)"
```

**Solutions:**
```sql
-- Update table statistics
ANALYZE TABLE table_name;

-- Force index if needed (temporary fix)
SELECT /*+ INDEX(table_name index_name) */ * FROM table_name WHERE ...;

-- Review recent schema changes
SELECT * FROM information_schema.columns
WHERE table_schema = 'your_db'
AND column_name LIKE '%_new%';
```

### 3. Blocking Chain Alert

**Symptoms:**
- Alert: "Database Blocking Chain > 60s"
- Multiple queries waiting
- Application timeouts

**Immediate Response:**
```sql
-- 1. Identify blocking chain
SELECT 
    r.trx_id waiting_trx_id,
    r.trx_mysql_thread_id waiting_thread,
    r.trx_query waiting_query,
    b.trx_id blocking_trx_id,
    b.trx_mysql_thread_id blocking_thread,
    b.trx_query blocking_query
FROM information_schema.innodb_lock_waits w
JOIN information_schema.innodb_trx b ON b.trx_id = w.blocking_trx_id
JOIN information_schema.innodb_trx r ON r.trx_id = w.requesting_trx_id;

-- 2. Kill blocking transaction if critical
KILL <blocking_thread_id>;
```

**Prevention:**
```yaml
best_practices:
  - Use shorter transactions
  - Access tables in consistent order
  - Use appropriate isolation levels
  - Add missing indexes to prevent lock escalation
```

### 4. Collector Overhead Too High

**Symptoms:**
- MySQL host CPU increase > 1%
- Collector memory > 400MB
- Performance degradation

**Diagnosis:**
```bash
# Check collector metrics
curl -s http://localhost:8888/metrics | grep -E 'process_resident_memory_bytes|process_cpu_seconds_total'

# Review collection frequency
grep collection_interval /etc/otel/mysql-edge-collector.yaml
```

**Solutions:**

1. **Adjust Collection Intervals:**
```yaml
# Light load systems
receivers:
  mysql/waits:
    collection_interval: 60s
  sqlquery/waits:
    collection_interval: 60s

# Heavy load systems
receivers:
  mysql/waits:
    collection_interval: 30s
    queries:
      - sql: "... LIMIT 50"  # Reduce from 100
```

2. **Enable Sampling:**
```yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 50  # Sample 50% of metrics
```

3. **Reduce Memory Limit:**
```yaml
processors:
  memory_limiter:
    limit_mib: 256  # Reduce from 384
    spike_limit_mib: 32
```

## Performance Tuning Guide

### Collection Optimization Matrix

| System Type | Collection Interval | Query Limit | Memory Limit | Sampling |
|-------------|-------------------|-------------|--------------|----------|
| Development | 60s | 50 | 256MB | None |
| Light Load | 30s | 75 | 384MB | None |
| Medium Load | 20s | 100 | 384MB | 20% low severity |
| Heavy Load | 15s | 100 | 512MB | 50% medium/low |
| Critical | 10s | 200 | 768MB | Custom rules |

### Tuning Configurations

#### 1. Light Load Configuration
```yaml
# /etc/otel/mysql-edge-collector-light.yaml
receivers:
  mysql/waits:
    collection_interval: 60s
  
  sqlquery/waits:
    collection_interval: 60s
    queries:
      - sql: "... LIMIT 50"

processors:
  memory_limiter:
    limit_mib: 256
  
  filter/cardinality:
    metrics:
      datapoint:
        - 'attributes["wait.severity"] in ["critical", "high"]'
        - 'attributes["advisor.priority"] in ["P0", "P1"]'
```

#### 2. Heavy Load Configuration
```yaml
# /etc/otel/mysql-edge-collector-heavy.yaml
receivers:
  mysql/waits:
    collection_interval: 30s
  
  sqlquery/waits:
    collection_interval: 30s
    queries:
      - sql: |
          SELECT ... 
          WHERE AVG_TIMER_WAIT > 10000000  -- >10ms only
          LIMIT 100

processors:
  memory_limiter:
    limit_mib: 512
  
  probabilistic_sampler:
    sampling_percentage: 80
  
  filter/aggressive:
    metrics:
      datapoint:
        - 'attributes["wait_percentage"] > 50'
        - 'attributes["exec_count"] > 100'
```

### Query-Specific Thresholds

```yaml
# Custom thresholds for critical queries
transform/custom_thresholds:
  metric_statements:
    - context: datapoint
      statements:
        # Critical business query - tighter thresholds
        - set(attributes["custom.threshold"], 100)
          where attributes["query_hash"] == "critical_query_hash_1"
        
        # Batch job - relaxed thresholds
        - set(attributes["custom.threshold"], 5000)
          where attributes["query_hash"] == "batch_job_hash"
        
        # Mark as critical if exceeds custom threshold
        - set(attributes["wait.severity"], "critical")
          where attributes["avg_time_ms"] > attributes["custom.threshold"]
```

## Troubleshooting Procedures

### No Metrics Appearing

```bash
# 1. Check collector status
systemctl status otel-collector-edge

# 2. Check logs
journalctl -u otel-collector-edge -n 100

# 3. Verify Performance Schema
mysql -u monitor -p -e "
SELECT * FROM performance_schema.setup_instruments 
WHERE NAME LIKE 'wait/%' AND ENABLED = 'NO' LIMIT 10;"

# 4. Test connectivity
mysql -h localhost -u $MYSQL_MONITOR_USER -p$MYSQL_MONITOR_PASS -e "SELECT 1"

# 5. Check metrics pipeline
curl -s http://localhost:8888/metrics | grep receiver_accepted_metric_points
```

### Missing Advisories

```bash
# 1. Verify advisory processing
curl -s http://localhost:9091/metrics | grep advisor_type | wc -l

# 2. Check gateway logs
docker logs otel-gateway 2>&1 | grep -i error | tail -20

# 3. Test advisory rules manually
curl -s http://localhost:9091/metrics | \
  awk '/no_index_used_count/{ni=$2} /avg_time_ms/{at=$2} /query_hash/{qh=$2} 
       END{if(ni>0 && at>100) print "Should have missing_index advisory for",qh}'

# 4. Review processor configuration
grep -A 20 "transform/advisors" /etc/otel/mysql-edge-collector.yaml
```

### High Cardinality Issues

```bash
# 1. Check unique query count
curl -s http://localhost:9091/metrics | grep mysql_query_wait_profile | \
  awk -F'"' '{print $2}' | sort -u | wc -l

# 2. Review cardinality control
curl -s http://localhost:8889/metrics | grep filter_cardinality_datapoints_filtered

# 3. Identify high cardinality dimensions
curl -s http://localhost:9091/metrics | grep mysql_query_wait_profile | \
  awk -F'[{}]' '{print $2}' | tr ',' '\n' | sort | uniq -c | sort -nr | head
```

## Emergency Response

### Critical Performance Degradation

**Immediate Actions:**
```bash
# 1. Reduce collection overhead
cat << EOF | sudo tee /tmp/emergency-config.yaml
receivers:
  mysql/waits:
    collection_interval: 300s  # 5 minutes
processors:
  filter/emergency:
    metrics:
      datapoint:
        - 'attributes["advisor.priority"] == "P0"'  # Critical only
EOF

# 2. Reload collector
sudo systemctl reload otel-collector-edge

# 3. Identify top issues
mysql -e "
SELECT DIGEST_TEXT, COUNT_STAR, AVG_TIMER_WAIT/1e9 as avg_ms
FROM performance_schema.events_statements_summary_by_digest 
ORDER BY SUM_TIMER_WAIT DESC LIMIT 5"

# 4. Apply emergency fixes
# - Kill long-running queries
# - Disable problematic features
# - Add emergency indexes
```

### Collector Crash Loop

```bash
# 1. Stop collector
sudo systemctl stop otel-collector-edge

# 2. Run in debug mode
sudo -u otel otelcol-contrib --config=/etc/otel/mysql-edge-collector.yaml \
  --log-level=debug 2>&1 | head -100

# 3. Common fixes
# - Reduce memory limit
# - Disable problematic receivers
# - Clear corrupted state

# 4. Start with minimal config
sudo -u otel otelcol-contrib --config=/etc/otel/minimal.yaml
```

## Maintenance Procedures

### Weekly Maintenance

```bash
#!/bin/bash
# weekly-maintenance.sh

echo "=== MySQL Monitoring Weekly Maintenance ==="

# 1. Review top advisories
echo "Top Performance Issues:"
curl -s http://localhost:9091/metrics | \
  grep advisor_type | sort | uniq -c | sort -nr | head -10

# 2. Check collector health
echo -e "\nCollector Health:"
curl -s http://localhost:8888/metrics | \
  grep -E 'process_uptime|dropped_metric_points|queue_size'

# 3. Analyze cardinality trends
echo -e "\nCardinality Analysis:"
curl -s http://localhost:9091/metrics | \
  grep mysql_query_wait_profile | wc -l

# 4. Update Performance Schema if needed
mysql -e "
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'wait/%' AND ENABLED = 'NO'"

# 5. Generate report
./scripts/generate-weekly-report.sh
```

### Monthly Optimization

1. **Review Query Patterns:**
   - Identify queries that improved/degraded
   - Update custom thresholds
   - Archive resolved advisories

2. **Tune Collection:**
   - Adjust intervals based on overhead
   - Update sampling rules
   - Optimize query limits

3. **Update Baselines:**
   - Calculate new p95/p99 thresholds
   - Update alert thresholds
   - Document pattern changes

### Upgrade Procedures

```bash
# 1. Test new version
docker run -d --name test-collector \
  -v /etc/otel/mysql-edge-collector.yaml:/etc/config.yaml \
  otel/opentelemetry-collector-contrib:new-version \
  --config=/etc/config.yaml

# 2. Validate metrics
sleep 30
docker exec test-collector curl -s http://localhost:8888/metrics

# 3. Phased rollout
./scripts/deploy-production.sh rollout 25
# Monitor for 24h
./scripts/deploy-production.sh rollout 50
# Monitor for 24h
./scripts/deploy-production.sh complete

# 4. Rollback if needed
./scripts/deploy-production.sh rollback
```

## Key Metrics Reference

| Metric | Normal Range | Warning | Critical | Action |
|--------|--------------|---------|----------|--------|
| wait_percentage | < 50% | 50-80% | > 80% | Check advisories |
| mysql.blocking.active | 0 | 5-30s | > 30s | Review locks |
| collector memory | < 300MB | 300-384MB | > 384MB | Tune config |
| collector CPU | < 0.5% | 0.5-1% | > 1% | Reduce collection |
| dropped metrics | 0 | > 0 | > 100/min | Increase resources |

## Contact Information

- **On-Call DBA**: dba-oncall@company.com
- **Platform Team**: platform@company.com
- **Escalation**: Use PagerDuty integration
- **Documentation**: Internal wiki /mysql-monitoring