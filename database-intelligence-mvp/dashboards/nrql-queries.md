# NRQL Queries for PostgreSQL Database Intelligence

This document contains all the NRQL queries used in the PostgreSQL Database Intelligence dashboards. These queries are validated by the E2E tests in `tests/e2e/nrdb_validation_test.go`.

## Table of Contents
- [PostgreSQL Overview Dashboard](#postgresql-overview-dashboard)
- [Plan Intelligence Dashboard](#plan-intelligence-dashboard)
- [Active Session History Dashboard](#active-session-history-dashboard)
- [Integrated Intelligence Dashboard](#integrated-intelligence-dashboard)
- [Alert Queries](#alert-queries)
- [Troubleshooting Queries](#troubleshooting-queries)

## PostgreSQL Overview Dashboard

### Active Connections
```sql
SELECT latest(postgresql.connections.active) 
FROM Metric 
WHERE db.system = 'postgresql' 
FACET db.name 
SINCE 5 minutes ago
```

### Transaction Rate
```sql
SELECT rate(sum(postgresql.transactions.committed), 1 minute) as 'Commits/min',
       rate(sum(postgresql.transactions.rolled_back), 1 minute) as 'Rollbacks/min'
FROM Metric 
WHERE db.system = 'postgresql' 
TIMESERIES 
SINCE 30 minutes ago
```

### Cache Hit Ratio
```sql
SELECT (sum(postgresql.blocks.hit) / 
        (sum(postgresql.blocks.hit) + sum(postgresql.blocks.read))) * 100 as 'Cache Hit %'
FROM Metric 
WHERE db.system = 'postgresql' 
FACET db.name 
SINCE 1 hour ago
```

### Database Size
```sql
SELECT latest(postgresql.database.size) / 1024 / 1024 as 'Size (MB)'
FROM Metric 
WHERE db.system = 'postgresql' 
FACET db.name
```

### Top Queries by Execution Count
```sql
SELECT count(*) as 'Executions', 
       average(query.exec_time_ms) as 'Avg Time (ms)'
FROM Metric 
WHERE metricName = 'postgresql.query.execution' 
FACET query.normalized 
LIMIT 10 
SINCE 1 hour ago
```

## Plan Intelligence Dashboard

### Plan Changes Over Time
```sql
SELECT count(*) 
FROM Metric 
WHERE metricName = 'postgresql.plan.change' 
FACET query.normalized, plan.change_type 
TIMESERIES
SINCE 1 hour ago
```

### Plan Regressions
```sql
SELECT count(*) as 'Regressions', 
       average(plan.cost_increase_ratio) as 'Avg Cost Increase'
FROM Metric 
WHERE metricName = 'postgresql.plan.regression' 
TIMESERIES 
SINCE 2 hours ago
```

### Query Performance Trend
```sql
SELECT average(query.exec_time_ms) as 'Avg Exec Time', 
       average(query.plan_time_ms) as 'Avg Plan Time',
       percentile(query.exec_time_ms, 95) as 'p95 Exec Time'
FROM Metric 
WHERE metricName = 'postgresql.query.execution' 
FACET query.normalized 
TIMESERIES 
SINCE 3 hours ago
```

### Top Plan Regressions
```sql
SELECT query.normalized, 
       plan.old_cost, 
       plan.new_cost,
       plan.cost_increase_ratio,
       plan.performance_impact
FROM Metric 
WHERE metricName = 'postgresql.plan.regression' 
ORDER BY plan.cost_increase_ratio DESC 
LIMIT 20 
SINCE 24 hours ago
```

### Plan Node Analysis
```sql
SELECT count(*) 
FROM Metric 
WHERE metricName = 'postgresql.plan.node' 
FACET plan.node_type, plan.issue_type 
SINCE 1 hour ago
```

### Query Plan Distribution
```sql
SELECT uniqueCount(plan.hash) as 'Unique Plans',
       count(*) as 'Total Executions'
FROM Metric 
WHERE metricName = 'postgresql.query.execution' 
  AND plan.hash IS NOT NULL
FACET query.normalized 
SINCE 6 hours ago
```

## Active Session History Dashboard

### Active Sessions Over Time
```sql
SELECT count(*) 
FROM Metric 
WHERE metricName = 'postgresql.ash.session' 
FACET session.state 
TIMESERIES 
SINCE 30 minutes ago
```

### Wait Event Distribution
```sql
SELECT sum(wait.duration_ms) 
FROM Metric 
WHERE metricName = 'postgresql.ash.wait_event' 
FACET wait.event_type, wait.event_name 
SINCE 1 hour ago
```

### Top Wait Events
```sql
SELECT sum(wait.duration_ms) as 'Total Wait Time',
       count(*) as 'Wait Count',
       average(wait.duration_ms) as 'Avg Wait'
FROM Metric 
WHERE metricName = 'postgresql.ash.wait_event' 
FACET wait.event_name 
ORDER BY sum(wait.duration_ms) DESC 
LIMIT 10 
SINCE 1 hour ago
```

### Blocking Analysis
```sql
SELECT blocking.query as 'Blocking Query',
       blocked.query as 'Blocked Query',
       count(*) as 'Block Count',
       max(block.duration_ms) as 'Max Block Duration'
FROM Metric 
WHERE metricName = 'postgresql.ash.blocking' 
FACET blocking.pid, blocked.pid 
SINCE 30 minutes ago
```

### Session Activity by Query
```sql
SELECT uniqueCount(session.pid) as 'Unique Sessions',
       count(*) as 'Total Samples'
FROM Metric 
WHERE metricName = 'postgresql.ash.session' 
  AND session.state = 'active'
FACET query.normalized 
TIMESERIES 
SINCE 1 hour ago
```

### Resource Utilization
```sql
SELECT average(session.cpu_usage) as 'CPU %',
       average(session.memory_mb) as 'Memory MB',
       sum(session.io_wait_ms) as 'IO Wait'
FROM Metric 
WHERE metricName = 'postgresql.ash.session' 
FACET session.backend_type 
SINCE 30 minutes ago
```

## Integrated Intelligence Dashboard

### Query Performance with Wait Analysis
```sql
SELECT average(query.exec_time_ms) as 'Exec Time',
       sum(wait.duration_ms) as 'Wait Time',
       count(DISTINCT plan.hash) as 'Plan Count'
FROM Metric 
WHERE metricName IN ('postgresql.query.execution', 'postgresql.ash.wait_event')
FACET query.normalized 
SINCE 2 hours ago
```

### Plan Regression Impact
```sql
SELECT plan.regression_detected as 'Has Regression',
       average(session.count) as 'Active Sessions',
       sum(wait.duration_ms) as 'Total Wait'
FROM Metric 
WHERE query.normalized IS NOT NULL
FACET query.normalized 
SINCE 1 hour ago
```

### Query Health Score
```sql
SELECT query.normalized,
       (100 - (plan.regression_count * 10 + 
       wait.excessive_count * 5 + 
       (query.exec_time_ms / 100))) as 'Health Score'
FROM Metric 
WHERE metricName = 'postgresql.query.health' 
ORDER BY 'Health Score' ASC 
LIMIT 20 
SINCE 24 hours ago
```

### Adaptive Sampling Effectiveness
```sql
SELECT sampling.rule as 'Rule',
       sampling.rate as 'Sample Rate',
       count(*) as 'Samples Collected',
       uniqueCount(query.normalized) as 'Unique Queries'
FROM Metric 
WHERE metricName = 'postgresql.adaptive_sampling' 
FACET sampling.rule 
SINCE 1 hour ago
```

## Alert Queries

### High Plan Regression Rate
```sql
SELECT count(*) 
FROM Metric 
WHERE metricName = 'postgresql.plan.regression' 
SINCE 5 minutes ago
```
**Alert Threshold**: > 5 regressions in 5 minutes

### Excessive Lock Waits
```sql
SELECT sum(wait.duration_ms) 
FROM Metric 
WHERE metricName = 'postgresql.ash.wait_event' 
  AND wait.event_type = 'Lock' 
SINCE 5 minutes ago
```
**Alert Threshold**: > 30,000ms total wait time

### Query Performance Degradation
```sql
SELECT percentile(query.exec_time_ms, 95) 
FROM Metric 
WHERE metricName = 'postgresql.query.execution' 
FACET query.normalized 
SINCE 10 minutes ago 
COMPARE WITH 1 hour ago
```
**Alert Threshold**: > 2x increase in p95 latency

### Database Connection Saturation
```sql
SELECT (latest(postgresql.connections.active) / 
        latest(postgresql.connections.max)) * 100 as 'Connection Usage %'
FROM Metric 
WHERE db.system = 'postgresql' 
FACET db.name
```
**Alert Threshold**: > 90% connection usage

### Circuit Breaker Activation
```sql
SELECT count(*) 
FROM Metric 
WHERE metricName = 'otelcol.processor.circuitbreaker.triggered' 
SINCE 5 minutes ago
```
**Alert Threshold**: > 0 (any activation)

## Troubleshooting Queries

### Verify Data Collection
```sql
SELECT count(*) 
FROM Metric 
WHERE db.system = 'postgresql' 
FACET metricName 
SINCE 5 minutes ago
```

### Check Attribute Cardinality
```sql
SELECT uniqueCount(query.normalized) as 'Unique Queries',
       uniqueCount(session.pid) as 'Unique Sessions',
       uniqueCount(wait.event_name) as 'Unique Wait Events'
FROM Metric 
WHERE db.system = 'postgresql' 
SINCE 1 hour ago
```

### Missing Data Investigation
```sql
SELECT latest(timestamp) 
FROM Metric 
WHERE db.system = 'postgresql' 
FACET metricName 
SINCE 1 hour ago
```

### Collector Health Check
```sql
SELECT latest(otelcol_process_uptime) as 'Uptime',
       latest(otelcol_process_runtime_heap_alloc_bytes) / 1024 / 1024 as 'Heap MB',
       latest(otelcol_receiver_accepted_metric_points) as 'Accepted Metrics',
       latest(otelcol_exporter_sent_metric_points) as 'Sent Metrics'
FROM Metric 
WHERE service.name = 'postgresql-database-intelligence' 
SINCE 5 minutes ago
```

## Best Practices

1. **Time Windows**: Use appropriate time windows based on metric collection intervals
   - Infrastructure metrics: 5-30 minutes
   - Plan changes: 1-24 hours
   - ASH data: 30 minutes - 2 hours

2. **Faceting**: Always facet by relevant dimensions
   - `db.name` for multi-database environments
   - `query.normalized` for query-level analysis
   - `session.state` for session analysis

3. **Aggregation**: Use appropriate aggregation functions
   - `latest()` for gauge metrics
   - `sum()` for counters
   - `average()` for latency metrics
   - `percentile()` for SLA monitoring

4. **Performance**: Limit result sets for dashboard performance
   - Use `LIMIT` for top-N queries
   - Use time windows to reduce data volume
   - Consider sampling for high-cardinality metrics