# End-to-End PostgreSQL Metrics Validation Queries

## Overview
Use these NRQL queries in New Relic to validate that all PostgreSQL metrics are being collected properly from both Config-Only and Custom deployment modes.

## 1. Basic Connectivity Validation

### Check if any PostgreSQL metrics are arriving
```sql
SELECT count(*) FROM Metric 
WHERE metricName LIKE 'postgresql%' 
SINCE 5 minutes ago
```

### List all unique PostgreSQL metrics being collected
```sql
SELECT uniques(metricName) FROM Metric 
WHERE metricName LIKE 'postgresql%' 
SINCE 1 hour ago
```

### Check metrics by deployment mode
```sql
SELECT uniqueCount(metricName) as 'Unique Metrics', 
       count(*) as 'Total Data Points' 
FROM Metric 
WHERE metricName LIKE 'postgresql%' 
FACET deployment.mode 
SINCE 30 minutes ago
```

## 2. Connection Metrics Validation

### postgresql.backends - Active connections
```sql
SELECT latest(postgresql.backends) as 'Active Connections' 
FROM Metric 
FACET deployment.mode 
TIMESERIES AUTO 
SINCE 30 minutes ago
```

### postgresql.connection.max - Maximum connections
```sql
SELECT latest(postgresql.connection.max) as 'Max Connections' 
FROM Metric 
FACET deployment.mode 
SINCE 10 minutes ago
```

### Connection state distribution (Custom mode with ASH)
```sql
SELECT sum(db.ash.active_sessions) as 'Sessions' 
FROM Metric 
WHERE deployment.mode = 'custom' 
FACET attributes.state 
SINCE 10 minutes ago
```

## 3. Transaction Metrics Validation

### postgresql.commits - Commit rate
```sql
SELECT rate(sum(postgresql.commits), 1 minute) as 'Commits/min' 
FROM Metric 
FACET deployment.mode 
TIMESERIES AUTO 
SINCE 1 hour ago
```

### postgresql.rollbacks - Rollback rate
```sql
SELECT rate(sum(postgresql.rollbacks), 1 minute) as 'Rollbacks/min' 
FROM Metric 
FACET deployment.mode 
TIMESERIES AUTO 
SINCE 1 hour ago
```

### Transaction ratio
```sql
SELECT sum(postgresql.commits) / (sum(postgresql.commits) + sum(postgresql.rollbacks)) * 100 as 'Commit %' 
FROM Metric 
FACET deployment.mode 
SINCE 1 hour ago
```

## 4. Block I/O Metrics Validation

### postgresql.blks_hit - Buffer cache hits
```sql
SELECT rate(sum(postgresql.blks_hit), 1 minute) as 'Buffer Hits/min' 
FROM Metric 
FACET deployment.mode 
TIMESERIES AUTO 
SINCE 30 minutes ago
```

### postgresql.blks_read - Disk reads
```sql
SELECT rate(sum(postgresql.blks_read), 1 minute) as 'Disk Reads/min' 
FROM Metric 
FACET deployment.mode 
TIMESERIES AUTO 
SINCE 30 minutes ago
```

### Buffer hit ratio
```sql
SELECT sum(postgresql.blks_hit) / (sum(postgresql.blks_hit) + sum(postgresql.blks_read)) * 100 as 'Buffer Hit %' 
FROM Metric 
FACET deployment.mode 
SINCE 1 hour ago
```

## 5. Row Operation Metrics Validation

### postgresql.rows - Row operations by type
```sql
SELECT rate(sum(postgresql.rows), 1 minute) as 'Rows/min' 
FROM Metric 
WHERE metricName = 'postgresql.rows' 
FACET deployment.mode, operation 
TIMESERIES AUTO 
SINCE 30 minutes ago
```

### postgresql.database.rows - Database-level row operations
```sql
SELECT sum(postgresql.database.rows) as 'Total Rows' 
FROM Metric 
FACET deployment.mode, db.name 
SINCE 30 minutes ago
```

## 6. Index Metrics Validation

### postgresql.index.scans - Index usage
```sql
SELECT sum(postgresql.index.scans) as 'Index Scans' 
FROM Metric 
FACET deployment.mode 
SINCE 30 minutes ago
```

### postgresql.sequential_scans - Sequential scans (missing indexes)
```sql
SELECT sum(postgresql.sequential_scans) as 'Sequential Scans' 
FROM Metric 
FACET deployment.mode 
SINCE 30 minutes ago
```

### Index efficiency ratio
```sql
SELECT sum(postgresql.index.scans) / (sum(postgresql.index.scans) + sum(postgresql.sequential_scans)) * 100 as 'Index Usage %' 
FROM Metric 
FACET deployment.mode 
SINCE 1 hour ago
```

## 7. Lock and Deadlock Metrics Validation

### postgresql.deadlocks - Deadlock detection
```sql
SELECT sum(postgresql.deadlocks) as 'Deadlocks' 
FROM Metric 
FACET deployment.mode 
TIMESERIES AUTO 
SINCE 1 hour ago
```

### postgresql.locks - Lock statistics
```sql
SELECT sum(postgresql.locks) as 'Total Locks' 
FROM Metric 
FACET deployment.mode 
SINCE 30 minutes ago
```

### Blocked sessions (Custom mode only)
```sql
SELECT sum(db.ash.blocked_sessions) as 'Blocked Sessions' 
FROM Metric 
WHERE deployment.mode = 'custom' 
TIMESERIES AUTO 
SINCE 30 minutes ago
```

## 8. WAL Metrics Validation

### postgresql.wal.lag - Replication lag
```sql
SELECT latest(postgresql.wal.lag) as 'WAL Lag' 
FROM Metric 
FACET deployment.mode 
TIMESERIES AUTO 
SINCE 30 minutes ago
```

### postgresql.wal.age - WAL age
```sql
SELECT latest(postgresql.wal.age) as 'WAL Age' 
FROM Metric 
FACET deployment.mode 
SINCE 10 minutes ago
```

### postgresql.wal.delay - WAL write delay
```sql
SELECT average(postgresql.wal.delay) as 'Avg WAL Delay' 
FROM Metric 
FACET deployment.mode 
SINCE 30 minutes ago
```

## 9. Background Writer Metrics Validation

### postgresql.bgwriter.checkpoint.count - Checkpoint frequency
```sql
SELECT rate(sum(postgresql.bgwriter.checkpoint.count), 1 minute) as 'Checkpoints/min' 
FROM Metric 
FACET deployment.mode 
TIMESERIES AUTO 
SINCE 1 hour ago
```

### Checkpoint metrics (new metrics added)
```sql
SELECT rate(sum(postgresql.bgwriter.stat.checkpoints_timed), 1 minute) as 'Timed Checkpoints/min',
       rate(sum(postgresql.bgwriter.stat.checkpoints_req), 1 minute) as 'Requested Checkpoints/min' 
FROM Metric 
FACET deployment.mode 
SINCE 30 minutes ago
```

## 10. Database Size Metrics Validation

### postgresql.database.size - Database sizes
```sql
SELECT latest(postgresql.database.size) / 1024 / 1024 as 'Size (MB)' 
FROM Metric 
FACET deployment.mode, db.name 
SINCE 10 minutes ago
```

### postgresql.table.size - Table sizes
```sql
SELECT latest(postgresql.table.size) / 1024 / 1024 as 'Size (MB)' 
FROM Metric 
FACET deployment.mode, table 
SINCE 10 minutes ago 
LIMIT 20
```

## 11. Vacuum Metrics Validation

### postgresql.table.vacuum.count - Vacuum activity
```sql
SELECT sum(postgresql.table.vacuum.count) as 'Vacuum Count' 
FROM Metric 
FACET deployment.mode 
SINCE 1 hour ago
```

### postgresql.live_rows vs dead rows
```sql
SELECT latest(postgresql.live_rows) as 'Live Rows',
       latest(postgresql.dead_rows) as 'Dead Rows' 
FROM Metric 
FACET deployment.mode 
SINCE 30 minutes ago
```

## 12. Temporary Files Metrics Validation

### postgresql.temp_files - Temp file creation
```sql
SELECT sum(postgresql.temp_files) as 'Temp Files' 
FROM Metric 
FACET deployment.mode 
TIMESERIES AUTO 
SINCE 1 hour ago
```

## 13. Custom Mode Exclusive Metrics

### ASH (Active Session History) metrics
```sql
SELECT sum(db.ash.active_sessions) as 'Active',
       sum(db.ash.blocked_sessions) as 'Blocked',
       sum(db.ash.long_running_queries) as 'Long Running' 
FROM Metric 
WHERE deployment.mode = 'custom' 
TIMESERIES AUTO 
SINCE 30 minutes ago
```

### Wait event analysis
```sql
SELECT sum(db.ash.wait_events) as 'Wait Events' 
FROM Metric 
WHERE deployment.mode = 'custom' 
FACET attributes.wait_event_type, attributes.wait_event_name 
SINCE 30 minutes ago 
LIMIT 20
```

### Query intelligence metrics
```sql
SELECT count(*) as 'Queries Analyzed' 
FROM Metric 
WHERE deployment.mode = 'custom' 
AND attributes.query_plan_type IS NOT NULL 
FACET attributes.query_plan_type 
SINCE 1 hour ago
```

## 14. SQL Query Receiver Metrics (Config-Only)

### Custom query metrics from sqlquery receiver
```sql
SELECT latest(pg.connection_count) as 'Connections' 
FROM Metric 
WHERE deployment.mode = 'config-only' 
FACET state 
SINCE 10 minutes ago
```

### Wait events from SQL queries
```sql
SELECT sum(pg.wait_events) as 'Wait Events' 
FROM Metric 
WHERE deployment.mode = 'config-only' 
FACET wait_event_type, wait_event 
SINCE 30 minutes ago
```

## 15. Comprehensive Metric Coverage Report

### Full metric inventory by mode
```sql
FROM Metric 
SELECT uniqueCount(metricName) as 'Total Unique Metrics',
       uniqueCount(metricName) filter (WHERE metricName LIKE 'postgresql.%') as 'Standard PG Metrics',
       uniqueCount(metricName) filter (WHERE metricName LIKE 'db.ash.%') as 'ASH Metrics',
       uniqueCount(metricName) filter (WHERE metricName LIKE 'pg.%') as 'SQL Query Metrics',
       count(*) as 'Total Data Points',
       rate(count(*), 1 minute) as 'DPM'
WHERE metricName LIKE 'postgresql%' OR metricName LIKE 'db.ash%' OR metricName LIKE 'pg.%'
FACET deployment.mode 
SINCE 1 hour ago
```

### Missing metrics detection
```sql
-- Run this query and compare with expected list
SELECT uniques(metricName) FROM Metric 
WHERE deployment.mode = 'config-only' 
AND metricName LIKE 'postgresql%' 
SINCE 1 hour ago
```

## Troubleshooting Queries

### Check for metric collection errors
```sql
SELECT count(*) as 'Error Count', 
       latest(error.message) as 'Last Error' 
FROM Log 
WHERE service.name LIKE 'db-intel%' 
AND message LIKE '%error%' 
FACET service.name 
SINCE 30 minutes ago
```

### Verify collector is running
```sql
SELECT latest(timestamp) as 'Last Seen' 
FROM Metric 
WHERE deployment.mode IN ('config-only', 'custom') 
FACET deployment.mode 
SINCE 10 minutes ago
```

### Data freshness check
```sql
SELECT percentile(timestamp - reportingTimestamp, 95) as 'P95 Latency (ms)' 
FROM Metric 
WHERE metricName LIKE 'postgresql%' 
FACET deployment.mode 
SINCE 10 minutes ago
```

## Expected Metrics Checklist

Use this query to get a complete list of metrics being collected, then compare with the expected list:

```sql
SELECT uniques(metricName) FROM Metric 
WHERE (deployment.mode = 'config-only' OR deployment.mode = 'custom')
AND (metricName LIKE 'postgresql%' OR metricName LIKE 'db.ash%' OR metricName LIKE 'pg.%')
SINCE 2 hours ago
```

Expected metrics for Config-Only mode (35+):
- postgresql.backends
- postgresql.bgwriter.buffers.allocated
- postgresql.bgwriter.buffers.writes
- postgresql.bgwriter.checkpoint.count
- postgresql.bgwriter.duration
- postgresql.bgwriter.maxwritten
- postgresql.bgwriter.stat.checkpoints_timed
- postgresql.bgwriter.stat.checkpoints_req
- postgresql.blocks_read
- postgresql.blks_hit
- postgresql.blks_read
- postgresql.buffer.hit
- postgresql.commits
- postgresql.conflicts
- postgresql.connection.max
- postgresql.database.count
- postgresql.database.locks
- postgresql.database.rows
- postgresql.database.size
- postgresql.deadlocks
- postgresql.index.scans
- postgresql.index.size
- postgresql.live_rows
- postgresql.locks
- postgresql.operations
- postgresql.replication.data_delay
- postgresql.rollbacks
- postgresql.rows
- postgresql.sequential_scans
- postgresql.stat_activity.count
- postgresql.table.count
- postgresql.table.size
- postgresql.table.vacuum.count
- postgresql.temp_files
- postgresql.wal.age
- postgresql.wal.delay
- postgresql.wal.lag

Additional Custom mode metrics:
- db.ash.active_sessions
- db.ash.wait_events
- db.ash.blocked_sessions
- db.ash.long_running_queries
- postgres.slow_queries.*
- And more enhanced metrics
