# New Relic NRQL Queries for Database Intelligence

This document contains useful NRQL queries for monitoring databases through the Database Intelligence OpenTelemetry Collector.

## Basic Health Queries

### Database Availability
```sql
-- Check if databases are responding
SELECT latest(postgresql.up) as 'PostgreSQL Status', 
       latest(mysql.up) as 'MySQL Status' 
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
FACET host.name 
SINCE 5 minutes ago
```

### Connection Pool Usage
```sql
-- Monitor connection pool utilization
SELECT latest(postgresql.backends) / latest(postgresql.backends.max) * 100 as 'PG Connection %',
       latest(mysql.threads.connected) / latest(mysql.threads.max) * 100 as 'MySQL Connection %'
FROM Metric 
WHERE service.name = 'database-intelligence-collector'
FACET database.name
SINCE 30 minutes ago
```

## Performance Queries

### Query Performance Overview
```sql
-- Average query duration by database type
SELECT average(database.query.duration) as 'Avg Duration (ms)',
       percentile(database.query.duration, 50) as 'P50',
       percentile(database.query.duration, 95) as 'P95',
       percentile(database.query.duration, 99) as 'P99'
FROM Metric 
WHERE service.name = 'database-intelligence-collector'
FACET database.type, database.name
SINCE 1 hour ago
```

### Slow Query Detection
```sql
-- Find queries taking more than 1 second
SELECT query.text, 
       average(query.duration) as 'Avg Duration',
       max(query.duration) as 'Max Duration',
       count(*) as 'Count'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND query.duration > 1000
FACET query.text
SINCE 1 hour ago
LIMIT 20
```

### Cache Hit Ratios
```sql
-- PostgreSQL cache effectiveness
SELECT (sum(postgresql.cache.hit) / 
        (sum(postgresql.cache.hit) + sum(postgresql.cache.miss))) * 100 
       as 'Cache Hit Ratio %'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND database.type = 'postgresql'
FACET database.name
TIMESERIES
SINCE 1 hour ago
```

## Resource Utilization

### Database Size Growth
```sql
-- Track database size over time
SELECT latest(database.size.bytes) / 1024 / 1024 / 1024 as 'Size (GB)'
FROM Metric 
WHERE service.name = 'database-intelligence-collector'
FACET database.name
TIMESERIES
SINCE 1 week ago
```

### Table Bloat Analysis
```sql
-- PostgreSQL table bloat
SELECT table.name,
       latest(table.size.bytes) / 1024 / 1024 as 'Size (MB)',
       latest(table.bloat.ratio) as 'Bloat Ratio'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND database.type = 'postgresql'
  AND table.bloat.ratio > 1.5
SINCE 1 hour ago
```

## Lock and Wait Analysis

### Lock Wait Events
```sql
-- Monitor lock contention
SELECT sum(lock.wait.count) as 'Lock Waits',
       average(lock.wait.duration) as 'Avg Wait Time'
FROM Metric 
WHERE service.name = 'database-intelligence-collector'
FACET database.type, lock.type
TIMESERIES
SINCE 1 hour ago
```

### Active Session History
```sql
-- ASH data for wait event analysis
SELECT count(*) as 'Sessions'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND metric.name = 'ash.session'
FACET wait.event.class, wait.event.name
TIMESERIES
SINCE 30 minutes ago
```

## Query Intelligence (with Custom Processors)

### Query Pattern Analysis
```sql
-- Analyze query patterns
SELECT query.pattern,
       count(*) as 'Executions',
       average(query.duration) as 'Avg Duration',
       sum(query.rows) as 'Total Rows'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND query.pattern IS NOT NULL
FACET query.pattern
SINCE 1 hour ago
LIMIT 50
```

### Query Cost Tracking
```sql
-- Monitor query costs (from cost control processor)
SELECT sum(query.cost.units) as 'Total Cost Units',
       sum(query.cost.usd) as 'Estimated Cost ($)'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND processor = 'costcontrol'
FACET database.name, query.type
TIMESERIES
SINCE 1 day ago
```

### PII Detection Alerts
```sql
-- Check for PII detection events
SELECT count(*) as 'PII Detections'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND processor = 'verification'
  AND pii.detected = true
FACET pii.type, table.name
SINCE 1 hour ago
```

## Replication and HA

### Replication Lag Monitoring
```sql
-- PostgreSQL replication lag
SELECT latest(postgresql.replication.lag) as 'Lag (seconds)'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND database.type = 'postgresql'
FACET primary.name, replica.name
SINCE 5 minutes ago
```

### MySQL Replication Status
```sql
-- MySQL replication health
SELECT latest(mysql.replication.lag) as 'Seconds Behind Master',
       latest(mysql.replication.io.running) as 'IO Thread',
       latest(mysql.replication.sql.running) as 'SQL Thread'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND database.type = 'mysql'
FACET replica.name
SINCE 5 minutes ago
```

## Alerting Queries

### High Connection Usage Alert
```sql
-- Alert when connection usage > 80%
SELECT percentage(count(*), 
  WHERE postgresql.backends / postgresql.backends.max > 0.8 
  OR mysql.threads.connected / mysql.threads.max > 0.8)
FROM Metric 
WHERE service.name = 'database-intelligence-collector'
FACET database.name
```

### Query Performance Degradation
```sql
-- Detect performance degradation
SELECT average(query.duration) as 'Current Avg',
       average(query.duration) as 'Baseline Avg' 
FROM Metric 
WHERE service.name = 'database-intelligence-collector'
COMPARE WITH 1 hour ago
FACET database.name
```

### Circuit Breaker Status
```sql
-- Monitor circuit breaker activations
SELECT latest(circuit.breaker.status) as 'Status',
       sum(circuit.breaker.trips) as 'Trip Count'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND processor = 'circuitbreaker'
FACET database.name
SINCE 1 hour ago
```

## Cost and Budget Monitoring

### Daily Cost Tracking
```sql
-- Track daily database operation costs
SELECT sum(cost.daily.usd) as 'Daily Cost ($)',
       latest(cost.budget.daily.usd) as 'Daily Budget ($)',
       (sum(cost.daily.usd) / latest(cost.budget.daily.usd)) * 100 as 'Budget Used %'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND processor = 'costcontrol'
SINCE 1 day ago
```

### Cost by Query Type
```sql
-- Breakdown costs by query type
SELECT sum(query.cost.usd) as 'Cost ($)'
FROM Metric 
WHERE service.name = 'database-intelligence-collector'
FACET query.type
SINCE 1 day ago
```

## Advanced Diagnostics

### Query Plan Distribution
```sql
-- Analyze query execution plans
SELECT count(*) as 'Count'
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND plan.node.type IS NOT NULL
FACET plan.node.type, plan.cost.category
SINCE 1 hour ago
```

### Correlation Analysis
```sql
-- Find correlated metrics (from query correlator)
SELECT correlation.coefficient,
       correlation.metric1,
       correlation.metric2
FROM Metric 
WHERE service.name = 'database-intelligence-collector' 
  AND processor = 'querycorrelator'
  AND abs(correlation.coefficient) > 0.7
SINCE 1 hour ago
```

## Usage Tips

1. **Time Windows**: Adjust `SINCE` clauses based on your needs:
   - Real-time monitoring: `SINCE 5 minutes ago`
   - Performance analysis: `SINCE 1 hour ago`
   - Trend analysis: `SINCE 1 week ago`

2. **Filtering**: Add additional WHERE clauses:
   - By environment: `AND deployment.environment = 'production'`
   - By specific database: `AND database.name = 'myapp'`
   - By host: `AND host.name = 'db-server-1'`

3. **Aggregations**: Use different aggregation functions:
   - `average()` for typical values
   - `percentile(metric, 95)` for P95 values
   - `rate(count(*), 1 minute)` for rates
   - `derivative(metric, 1 minute)` for change rates

4. **Alerting**: Convert queries to alerts by:
   - Adding threshold conditions
   - Setting appropriate time windows
   - Including relevant facets for granular alerts