# New Relic Dashboard Queries for Database Intelligence MVP

## Overview Dashboard

### 1. Database Operations Overview
```sql
SELECT rate(sum(postgresql.operations), 1 minute) as 'Operations/min' 
FROM Metric 
WHERE metricName = 'postgresql.operations' 
FACET operation 
SINCE 30 minutes ago 
TIMESERIES
```

### 2. Table Activity Heatmap
```sql
SELECT sum(postgresql.blocks_read) as 'Blocks Read' 
FROM Metric 
WHERE postgresql.table.name IS NOT NULL 
FACET postgresql.table.name 
SINCE 1 hour ago
```

### 3. Database Size Trends
```sql
SELECT latest(postgresql.table.size) as 'Table Size' 
FROM Metric 
WHERE postgresql.table.name IS NOT NULL 
FACET postgresql.table.name 
SINCE 24 hours ago 
TIMESERIES
```

### 4. Background Writer Activity
```sql
SELECT sum(postgresql.bgwriter.checkpoint.count) as 'Checkpoints',
       sum(postgresql.bgwriter.buffers.writes) as 'Buffer Writes'
FROM Metric 
WHERE metricName LIKE 'postgresql.bgwriter%' 
SINCE 1 hour ago 
TIMESERIES
```

### 5. Index Usage Efficiency
```sql
SELECT rate(sum(postgresql.index.scans), 1 minute) as 'Index Scans/min' 
FROM Metric 
WHERE postgresql.table.name IS NOT NULL 
FACET postgresql.table.name 
SINCE 30 minutes ago
```

## Alerting Queries

### High Table Growth Alert
```sql
SELECT max(postgresql.table.size) - min(postgresql.table.size) as 'Growth' 
FROM Metric 
WHERE postgresql.table.name IS NOT NULL 
FACET postgresql.table.name 
SINCE 1 hour ago
```
Alert when: Growth > 1000000 (1MB)

### Vacuum Activity Monitor
```sql
SELECT sum(postgresql.table.vacuum.count) as 'Vacuum Count' 
FROM Metric 
WHERE postgresql.table.name IS NOT NULL 
FACET postgresql.table.name 
SINCE 24 hours ago
```
Alert when: Vacuum Count < 1

## Entity Synthesis Verification

### Database Entities
```sql
SELECT uniques(entity.name), uniques(entity.guid) 
FROM Metric 
WHERE entity.type = 'DATABASE' 
AND instrumentation.provider = 'opentelemetry' 
SINCE 1 hour ago
```

## Data Quality Monitoring

### Collection Gaps
```sql
SELECT uniqueCount(timestamp) as 'Data Points' 
FROM Metric 
WHERE collector.name = 'database-intelligence' 
FACET metricName 
SINCE 1 hour ago 
TIMESERIES 1 minute
```

### Metric Cardinality
```sql
SELECT uniqueCount(metricName) as 'Unique Metrics',
       uniqueCount(postgresql.table.name) as 'Unique Tables',
       count(*) as 'Total Data Points'
FROM Metric 
WHERE instrumentation.provider = 'opentelemetry' 
SINCE 1 hour ago
```