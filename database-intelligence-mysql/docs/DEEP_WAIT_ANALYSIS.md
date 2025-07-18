# Deep Analysis: MySQL Wait-Based Performance Monitoring

## Executive Summary

This document provides a comprehensive analysis of the MySQL wait-based monitoring implementation, including root cause analysis of common issues and deep fixes rather than superficial patches.

## Architecture Analysis

### 1. Wait Event Collection Architecture

```
MySQL Performance Schema
    ↓
Wait Events (events_waits_*)
    ↓
Statement Digests (events_statements_summary_by_digest)
    ↓
Edge Collector (SQL queries every 10s)
    ↓
Gateway (Advisory Processing)
    ↓
New Relic (Visualization & Alerting)
```

### Key Design Decisions:
- **10-second collection interval**: Balances overhead vs granularity
- **Digest-based aggregation**: Reduces cardinality while maintaining query identity
- **Two-tier processing**: Edge collection + gateway enrichment

## Deep Dive: Performance Schema Configuration

### Root Cause Analysis

The Performance Schema must be properly configured for wait analysis. Common issues:

1. **Incomplete Instrumentation**
   - Root cause: Default MySQL configs disable many wait instruments
   - Deep fix: Enable all wait instruments at startup
   ```sql
   UPDATE performance_schema.setup_instruments 
   SET ENABLED = 'YES', TIMED = 'YES' 
   WHERE NAME LIKE 'wait/%';
   ```

2. **Consumer Hierarchy**
   - Root cause: MySQL's consumer hierarchy means disabling a parent disables all children
   - Deep fix: Enable consumers in correct order
   ```sql
   -- Enable in this order (parent to child)
   UPDATE performance_schema.setup_consumers 
   SET ENABLED = 'YES' 
   WHERE NAME IN (
     'global_instrumentation',
     'thread_instrumentation', 
     'events_statements_current',
     'events_statements_history',
     'events_waits_current',
     'events_waits_history'
   );
   ```

3. **Memory Allocation**
   - Root cause: Performance Schema pre-allocates memory at startup
   - Deep fix: Set appropriate sizing parameters
   ```ini
   [mysqld]
   performance_schema_digests_size = 10000
   performance_schema_events_statements_history_size = 100
   performance_schema_events_waits_history_size = 100
   ```

## Wait Categories Deep Analysis

### 1. I/O Waits (`wait/io/file/*`)

**Common Issues:**
- Missing indexes causing table scans
- Poor buffer pool configuration
- Disk subsystem bottlenecks

**Deep Fixes:**
```sql
-- Identify queries causing most I/O
SELECT 
    s.DIGEST_TEXT,
    s.SUM_ROWS_EXAMINED,
    s.SUM_ROWS_SENT,
    s.SUM_NO_INDEX_USED,
    s.SUM_NO_GOOD_INDEX_USED,
    w.total_wait_time_ms
FROM performance_schema.events_statements_summary_by_digest s
JOIN (
    SELECT 
        THREAD_ID,
        SUM(TIMER_WAIT)/1000000 as total_wait_time_ms
    FROM performance_schema.events_waits_history_long
    WHERE EVENT_NAME LIKE 'wait/io/file/%'
    GROUP BY THREAD_ID
) w ON s.THREAD_ID = w.THREAD_ID
WHERE s.SUM_NO_INDEX_USED > 0
ORDER BY w.total_wait_time_ms DESC;
```

### 2. Lock Waits (`wait/lock/*`)

**Root Causes:**
- Long-running transactions
- Lock escalation from row to table
- Deadlock retry storms

**Deep Fixes:**
```sql
-- Transaction age analysis
SELECT 
    trx.trx_id,
    trx.trx_started,
    TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) as age_seconds,
    trx.trx_query,
    locks.lock_mode,
    locks.lock_type,
    locks.lock_table
FROM information_schema.innodb_trx trx
JOIN information_schema.innodb_locks locks 
    ON trx.trx_id = locks.lock_trx_id
WHERE TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) > 5
ORDER BY age_seconds DESC;

-- Lock wait chain analysis
WITH RECURSIVE lock_chain AS (
    SELECT 
        blocking_trx_id as trx_id,
        0 as level
    FROM information_schema.innodb_lock_waits
    WHERE requesting_trx_id NOT IN (
        SELECT blocking_trx_id 
        FROM information_schema.innodb_lock_waits
    )
    
    UNION ALL
    
    SELECT 
        lw.requesting_trx_id,
        lc.level + 1
    FROM lock_chain lc
    JOIN information_schema.innodb_lock_waits lw
        ON lc.trx_id = lw.blocking_trx_id
)
SELECT * FROM lock_chain;
```

### 3. Network Waits (`wait/io/socket/*`)

**Root Causes:**
- Network latency
- Large result sets
- Connection pool exhaustion

**Deep Fixes:**
```sql
-- Identify queries returning large result sets
SELECT 
    DIGEST_TEXT,
    SUM_ROWS_SENT,
    AVG_ROWS_SENT,
    COUNT_STAR as exec_count,
    SUM_ROWS_SENT / COUNT_STAR as avg_rows_per_query
FROM performance_schema.events_statements_summary_by_digest
WHERE SUM_ROWS_SENT > 1000000
ORDER BY SUM_ROWS_SENT DESC;
```

## Advisory Generation Deep Dive

### Intelligent Advisory Logic

The advisory system should provide actionable insights, not just report problems:

```yaml
# Enhanced advisory processor configuration
processors:
  transform/intelligent_advisors:
    metric_statements:
      - context: datapoint
        statements:
          # Composite advisory for missing index + high wait
          - set(attributes["advisor.type"], "critical_missing_index")
            where attributes["no_index_used_count"] > 0 
              and attributes["wait_percentage"] > 80
              and attributes["exec_count"] > 1000
          
          # Detect query plan regression
          - set(attributes["advisor.type"], "plan_regression")
            where attributes["rows_examined_ratio"] > attributes["historical_p95_ratio"] * 2
              and attributes["exec_count"] > 100
          
          # Lock contention pattern
          - set(attributes["advisor.type"], "lock_escalation")
            where attributes["lock_wait_time_ms"] > 1000
              and attributes["lock_type"] == "TABLE"
              and attributes["previous_lock_type"] == "ROW"
          
          # Temp table to disk
          - set(attributes["advisor.type"], "temp_table_disk")
            where attributes["created_tmp_disk_tables"] > 0
              and attributes["query_pattern"] =~ ".*GROUP BY.*HAVING.*"
```

### Advisory Priority Algorithm

```yaml
# Priority calculation based on business impact
- set(attributes["advisor.priority"], "P0")
  where (
    attributes["wait_percentage"] > 90 
    and attributes["exec_count"] > 10000
  ) or (
    attributes["user_impact_score"] > 0.8
  ) or (
    attributes["query_tag"] == "payment_critical"
  )
```

## Connection Pool Optimization

### Root Cause: Pool Starvation

Many "database performance" issues are actually connection pool issues:

```sql
-- Monitor connection usage
SELECT 
    user,
    host,
    COUNT(*) as connection_count,
    SUM(IF(command = 'Sleep', 1, 0)) as idle_connections,
    SUM(IF(time > 60 AND command != 'Sleep', 1, 0)) as long_running,
    MAX(time) as max_connection_age
FROM information_schema.processlist
GROUP BY user, host
ORDER BY connection_count DESC;
```

### Deep Fix: Adaptive Pool Sizing

```go
// Adaptive connection pool manager
type AdaptivePool struct {
    minSize     int
    maxSize     int
    currentSize int
    waitTime    prometheus.Histogram
}

func (p *AdaptivePool) AdjustSize(metrics PoolMetrics) {
    avgWaitTime := metrics.AvgWaitTime()
    utilization := metrics.Utilization()
    
    if avgWaitTime > 100*time.Millisecond && utilization > 0.8 {
        // Increase pool size
        p.currentSize = min(p.currentSize + 5, p.maxSize)
    } else if utilization < 0.3 && p.currentSize > p.minSize {
        // Decrease pool size
        p.currentSize = max(p.currentSize - 1, p.minSize)
    }
}
```

## Query Pattern Analysis

### Identifying Problematic Patterns

```sql
-- Find queries with high wait variance
WITH query_stats AS (
    SELECT 
        DIGEST,
        DIGEST_TEXT,
        COUNT_STAR as exec_count,
        AVG_TIMER_WAIT as avg_time,
        STDDEV_TIMER_WAIT as stddev_time,
        MIN_TIMER_WAIT as min_time,
        MAX_TIMER_WAIT as max_time,
        STDDEV_TIMER_WAIT / AVG_TIMER_WAIT as cv  -- Coefficient of variation
    FROM performance_schema.events_statements_summary_by_digest
    WHERE COUNT_STAR > 100
)
SELECT 
    DIGEST_TEXT,
    exec_count,
    avg_time/1000000 as avg_ms,
    stddev_time/1000000 as stddev_ms,
    cv,
    max_time/1000000 as max_ms
FROM query_stats
WHERE cv > 2  -- High variance queries
ORDER BY cv DESC;
```

## Memory and Resource Management

### Deep Fix: Prevent Memory Leaks

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 384
    spike_limit_mib: 64
    
  # Custom processor to detect and prevent leaks
  resource_monitor:
    thresholds:
      memory_growth_rate: 10  # MB/minute
      goroutine_count: 1000
    actions:
      - type: force_gc
        when: memory_growth_rate > 10
      - type: circuit_break
        when: goroutine_count > 1000
```

## Production Deployment Best Practices

### 1. Gradual Rollout with Validation

```bash
#!/bin/bash
# Phased deployment with validation gates

deploy_phase() {
    local phase=$1
    local percentage=$2
    
    echo "Deploying phase: $phase ($percentage%)"
    
    # Deploy to percentage of hosts
    ansible-playbook deploy.yml --limit "mysql_hosts[0:${percentage}%]"
    
    # Wait and validate
    sleep 300
    
    # Check error rates
    error_rate=$(curl -s "$MONITORING_API/errorRate?service=mysql-collector")
    if (( $(echo "$error_rate > 0.01" | bc -l) )); then
        echo "Error rate too high: $error_rate"
        rollback $phase
        return 1
    fi
    
    # Check performance impact
    mysql_cpu=$(curl -s "$MONITORING_API/cpuUsage?service=mysql")
    if (( $(echo "$mysql_cpu > 85" | bc -l) )); then
        echo "MySQL CPU too high: $mysql_cpu%"
        rollback $phase
        return 1
    fi
    
    return 0
}

# Phased deployment
deploy_phase "canary" 1 || exit 1
deploy_phase "pilot" 10 || exit 1
deploy_phase "partial" 50 || exit 1
deploy_phase "complete" 100 || exit 1
```

### 2. Circuit Breaker Pattern

```go
// Circuit breaker for collector
type CircuitBreaker struct {
    failures     int
    lastFailTime time.Time
    state        State
    threshold    int
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == Open {
        if time.Since(cb.lastFailTime) > 30*time.Second {
            cb.state = HalfOpen
        } else {
            return ErrCircuitOpen
        }
    }
    
    err := fn()
    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()
        if cb.failures >= cb.threshold {
            cb.state = Open
        }
        return err
    }
    
    // Success
    if cb.state == HalfOpen {
        cb.state = Closed
    }
    cb.failures = 0
    return nil
}
```

## Troubleshooting Decision Tree

```
Performance Issue Detected
    │
    ├─> High Wait Percentage (>80%)
    │   │
    │   ├─> I/O Waits → Check indexes, buffer pool
    │   ├─> Lock Waits → Check transactions, isolation level
    │   └─> Network Waits → Check result set size, connection pool
    │
    ├─> Plan Change Detected
    │   │
    │   ├─> Statistics Outdated → Run ANALYZE TABLE
    │   ├─> Data Distribution Changed → Review indexes
    │   └─> MySQL Version Changed → Test query plans
    │
    └─> Advisory Generated
        │
        ├─> P0 Priority → Immediate action required
        ├─> P1 Priority → Address within 24 hours
        └─> P2 Priority → Include in next maintenance
```

## Monitoring Health Metrics

Key metrics to track the monitoring system itself:

1. **Collection Lag**: Time between event occurrence and metric emission
2. **Advisory Accuracy**: False positive rate of generated advisories
3. **Resource Overhead**: CPU and memory usage of collectors
4. **Data Loss Rate**: Dropped metrics due to queue overflow

```sql
-- Monitor the monitoring system
SELECT 
    'collector_lag' as metric,
    TIMESTAMPDIFF(SECOND, 
        MAX(LAST_SEEN), 
        NOW()
    ) as value
FROM performance_schema.events_statements_summary_by_digest

UNION ALL

SELECT 
    'active_queries' as metric,
    COUNT(*) as value
FROM information_schema.processlist
WHERE command != 'Sleep'

UNION ALL

SELECT 
    'monitoring_overhead_pct' as metric,
    (
        SELECT SUM(TIME)
        FROM information_schema.processlist
        WHERE user = 'otel_monitor'
    ) / (
        SELECT SUM(TIME)
        FROM information_schema.processlist
    ) * 100 as value;
```

## Conclusion

Effective wait-based monitoring requires:
1. **Deep understanding** of MySQL internals
2. **Proper configuration** at every layer
3. **Intelligent processing** to generate actionable insights
4. **Continuous validation** of the monitoring system itself

The key is to fix root causes, not symptoms, and to build a system that provides value without adding significant overhead.