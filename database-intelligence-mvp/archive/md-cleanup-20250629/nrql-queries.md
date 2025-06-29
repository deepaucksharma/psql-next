# NRQL Query Library for Database Intelligence

This document provides a comprehensive collection of NRQL queries for monitoring the Database Intelligence Collector and databases.

## Table of Contents

1. [Collector Health Queries](#collector-health-queries)
2. [Database Performance Queries](#database-performance-queries)
3. [PostgreSQL Specific Queries](#postgresql-specific-queries)
4. [MySQL Specific Queries](#mysql-specific-queries)
5. [Alert Condition Queries](#alert-condition-queries)
6. [Troubleshooting Queries](#troubleshooting-queries)

## Collector Health Queries

### Basic Health Check

```sql
-- Is the collector running?
SELECT latest(otelcol_process_uptime) as 'Uptime (seconds)', 
       latest(timestamp) as 'Last Seen'
FROM Metric 
WHERE otel.library.name LIKE 'otelcol%' 
SINCE 5 minutes ago
```

### Data Flow Metrics

```sql
-- Data flow through the pipeline
SELECT rate(sum(otelcol_receiver_accepted_metric_points), 1 minute) as 'Received/min',
       rate(sum(otelcol_processor_accepted_metric_points), 1 minute) as 'Processed/min',
       rate(sum(otelcol_exporter_sent_metric_points), 1 minute) as 'Exported/min'
FROM Metric 
WHERE otel.library.name LIKE 'otelcol%' 
TIMESERIES AUTO
```

### Memory Usage

```sql
-- Collector memory consumption
SELECT average(otelcol_process_runtime_heap_alloc_bytes)/1024/1024 as 'Heap MB',
       average(otelcol_process_runtime_total_sys_memory_bytes)/1024/1024 as 'System MB',
       max(otelcol_process_runtime_total_alloc_bytes)/1024/1024 as 'Peak Alloc MB'
FROM Metric 
WHERE otel.library.name LIKE 'otelcol%' 
TIMESERIES AUTO
```

### Error Rates

```sql
-- Export failures and errors
SELECT rate(sum(otelcol_exporter_send_failed_metric_points), 1 minute) as 'Failed Exports/min',
       rate(sum(otelcol_receiver_refused_metric_points), 1 minute) as 'Refused Metrics/min'
FROM Metric 
WHERE otel.library.name LIKE 'otelcol%' 
FACET exporter, receiver
TIMESERIES AUTO
```

## Database Performance Queries

### Multi-Database Overview

```sql
-- All databases status and connections
SELECT latest(db_up) as 'Status',
       latest(db_connections_active) as 'Active',
       latest(db_connections_idle) as 'Idle',
       latest(db_connections_max) as 'Max',
       percentage(latest(db_connections_active), latest(db_connections_max)) as 'Usage %'
FROM Metric 
WHERE db_system IN ('postgresql', 'mysql')
FACET db_system, db_name
SINCE 5 minutes ago
```

### Query Performance Comparison

```sql
-- Compare query performance across databases
SELECT average(db_query_mean_duration) as 'Avg Duration',
       max(db_query_max_duration) as 'Max Duration',
       sum(db_query_count) as 'Query Count'
FROM Metric 
WHERE db_system IN ('postgresql', 'mysql')
FACET db_system, db_name
SINCE 1 hour ago
```

### Connection Pool Health

```sql
-- Connection pool utilization
SELECT average(db_connections_active) as 'Avg Active',
       max(db_connections_active) as 'Peak Active',
       average(db_connections_blocked) as 'Avg Blocked',
       percentage(average(db_connections_active), average(db_connections_max)) as 'Avg Utilization %'
FROM Metric 
WHERE db_system IN ('postgresql', 'mysql')
FACET db_name
TIMESERIES 5 minutes
```

## PostgreSQL Specific Queries

### Top Slow Queries

```sql
-- Slowest queries by average duration
SELECT average(db_query_mean_duration) as 'Avg Duration (ms)',
       sum(db_query_calls) as 'Total Calls',
       average(db_query_rows) as 'Avg Rows',
       latest(db_statement_type) as 'Type'
FROM Metric 
WHERE db_system = 'postgresql' 
  AND db_query_mean_duration > 100
FACET db_query_hash
SINCE 1 hour ago
LIMIT 20
```

### Cache Performance

```sql
-- Buffer cache hit ratio
SELECT (sum(postgresql_blocks_hit) / 
        (sum(postgresql_blocks_hit) + sum(postgresql_blocks_read))) * 100 
        as 'Cache Hit Ratio %'
FROM Metric 
WHERE db_system = 'postgresql'
TIMESERIES 5 minutes
```

### Table Maintenance

```sql
-- Tables needing vacuum
SELECT latest(db_table_dead_tuples) as 'Dead Tuples',
       latest(db_table_live_tuples) as 'Live Tuples',
       percentage(latest(db_table_dead_tuples), 
                 latest(db_table_live_tuples)) as 'Bloat %'
FROM Metric 
WHERE db_system = 'postgresql' 
  AND db_table_dead_tuples > 10000
FACET schemaname, tablename
SINCE 1 hour ago
LIMIT 20
```

### Replication Health

```sql
-- Replication lag monitoring
SELECT average(postgresql_replication_lag_seconds) as 'Avg Lag (s)',
       max(postgresql_replication_lag_seconds) as 'Max Lag (s)'
FROM Metric 
WHERE db_system = 'postgresql'
FACET application_name
TIMESERIES 5 minutes
```

### Lock Analysis

```sql
-- Database locks and wait events
SELECT count(*) as 'Lock Count'
FROM Metric 
WHERE db_system = 'postgresql' 
  AND db_wait_event_type IS NOT NULL
FACET db_wait_event_type, db_wait_event
SINCE 10 minutes ago
```

## MySQL Specific Queries

### Query Performance

```sql
-- MySQL slow query analysis
SELECT rate(sum(mysql_global_status_slow_queries), 1 minute) as 'Slow Queries/min',
       average(mysql_global_status_questions) as 'Questions/sec'
FROM Metric 
WHERE db_system = 'mysql'
TIMESERIES 5 minutes
```

### InnoDB Performance

```sql
-- InnoDB buffer pool efficiency
SELECT (sum(mysql_global_status_innodb_buffer_pool_read_requests) - 
        sum(mysql_global_status_innodb_buffer_pool_reads)) / 
        sum(mysql_global_status_innodb_buffer_pool_read_requests) * 100 
        as 'Buffer Pool Hit Ratio %'
FROM Metric 
WHERE db_system = 'mysql'
TIMESERIES 5 minutes
```

### Connection Metrics

```sql
-- MySQL connection statistics
SELECT latest(mysql_global_status_threads_connected) as 'Connected',
       latest(mysql_global_status_threads_running) as 'Running',
       latest(mysql_global_variables_max_connections) as 'Max Allowed'
FROM Metric 
WHERE db_system = 'mysql'
FACET db_name
SINCE 5 minutes ago
```

## Alert Condition Queries

### Database Down Alert

```sql
-- Alert when database is unreachable
SELECT latest(db_up) 
FROM Metric 
WHERE db_system IN ('postgresql', 'mysql')
FACET db_name
```

### High Connection Usage Alert

```sql
-- Alert when connections exceed 80% of max
SELECT percentage(latest(db_connections_active), 
                 latest(db_connections_max)) as 'Connection Usage %'
FROM Metric 
WHERE db_system IN ('postgresql', 'mysql')
FACET db_name
```

### Slow Query Alert

```sql
-- Alert on queries exceeding threshold
SELECT average(db_query_mean_duration)
FROM Metric 
WHERE db_system IN ('postgresql', 'mysql')
  AND db_query_mean_duration > 5000
FACET db_name
```

### Collector Memory Alert

```sql
-- Alert on high collector memory usage
SELECT average(otelcol_process_runtime_heap_alloc_bytes)/1024/1024/1024 as 'Heap GB'
FROM Metric 
WHERE otel.library.name LIKE 'otelcol%'
```

### Export Failure Alert

```sql
-- Alert on export failures
SELECT rate(sum(otelcol_exporter_send_failed_metric_points), 5 minute)
FROM Metric 
WHERE otel.library.name LIKE 'otelcol%'
FACET exporter
```

## Troubleshooting Queries

### No Data Investigation

```sql
-- Check when metrics were last received
SELECT latest(timestamp) as 'Last Metric Time',
       count(*) as 'Metric Count'
FROM Metric 
WHERE otel.library.name LIKE 'otelcol%' 
   OR db_system IN ('postgresql', 'mysql')
FACET otel.library.name, db_system
SINCE 30 minutes ago
```

### Pipeline Bottleneck Analysis

```sql
-- Identify where metrics are getting stuck
SELECT latest(otelcol_receiver_accepted_metric_points) as 'Received',
       latest(otelcol_processor_accepted_metric_points) as 'Processed',
       latest(otelcol_exporter_sent_metric_points) as 'Exported',
       latest(otelcol_exporter_queue_size) as 'Queue Size'
FROM Metric 
WHERE otel.library.name LIKE 'otelcol%'
SINCE 5 minutes ago
```

### Circuit Breaker Status

```sql
-- Check circuit breaker states (experimental mode)
SELECT latest(otelcol_circuitbreaker_state) as 'State',
       latest(otelcol_circuitbreaker_consecutive_failures) as 'Failures',
       latest(otelcol_circuitbreaker_requests_total) as 'Total Requests'
FROM Metric 
WHERE otel.library.name = 'otelcol/circuitbreaker'
FACET database
SINCE 10 minutes ago
```

### Adaptive Sampling Analysis

```sql
-- Monitor adaptive sampling behavior (experimental mode)
SELECT average(otelcol_adaptivesampler_sampling_rate) * 100 as 'Sampling %',
       sum(otelcol_adaptivesampler_decisions_total) as 'Total Decisions',
       sum(otelcol_adaptivesampler_sampled_total) as 'Sampled'
FROM Metric 
WHERE otel.library.name = 'otelcol/adaptivesampler'
FACET rule
SINCE 30 minutes ago
```

### Data Quality Check

```sql
-- Verify metric attributes and values
SELECT uniques(db_system) as 'Database Types',
       uniques(db_name) as 'Database Names',
       uniques(otel.library.name) as 'OTEL Libraries',
       count(*) as 'Total Metrics'
FROM Metric 
WHERE otel.library.name LIKE 'otelcol%' 
   OR db_system IS NOT NULL
SINCE 1 hour ago
```

## Usage Tips

1. **Time Ranges**: Adjust `SINCE` clauses based on your needs
2. **Faceting**: Add `FACET` to break down metrics by dimensions
3. **Alerting**: Use these queries as bases for New Relic alert conditions
4. **Dashboards**: Import these queries into custom dashboards
5. **Variables**: Replace hardcoded values with dashboard variables for flexibility

## Query Optimization

- Use `SINCE` to limit data scanned
- Add `WHERE` clauses to filter early
- Use `FACET` with `LIMIT` for large result sets
- Prefer `latest()` over `average()` for current state
- Use `TIMESERIES AUTO` for appropriate granularity