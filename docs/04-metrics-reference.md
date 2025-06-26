# PostgreSQL Unified Collector - Metrics Reference

## Table of Contents
1. [Metrics Overview](#metrics-overview)
2. [OHI-Compatible Metrics](#ohi-compatible-metrics)
3. [Extended Metrics](#extended-metrics)
4. [Metric Attributes](#metric-attributes)
5. [Query Examples](#query-examples)
6. [Dashboards and Alerts](#dashboards-and-alerts)

## Metrics Overview

The PostgreSQL Unified Collector provides comprehensive metrics in two categories:

1. **OHI-Compatible Metrics**: 100% compatible with existing nri-postgresql metrics
2. **Extended Metrics**: Additional metrics from the reference architecture including histograms, ASH, and kernel metrics

### Metric Naming Conventions

- **NRI Mode**: Uses OHI event types (e.g., `PostgresSlowQueries`)
- **OTLP Mode**: Uses OpenTelemetry conventions (e.g., `db.postgresql.query.duration`)
- **Hybrid Mode**: Exports both formats simultaneously

## OHI-Compatible Metrics

### 1. PostgresSlowQueries

Tracks query performance metrics from pg_stat_statements.

**Event Type**: `PostgresSlowQueries`

| Metric Name | Type | Description | Unit |
|------------|------|-------------|------|
| `newrelic` | attribute | Fixed value "newrelic" for compatibility | - |
| `query_id` | attribute | Unique query identifier | - |
| `query_text` | attribute | Anonymized query text (max 4095 chars) | - |
| `database_name` | attribute | Database name | - |
| `schema_name` | attribute | Schema name | - |
| `execution_count` | gauge | Number of times executed | count |
| `avg_elapsed_time_ms` | gauge | Average execution time | milliseconds |
| `avg_disk_reads` | gauge | Average disk blocks read | blocks |
| `avg_disk_writes` | gauge | Average disk blocks written | blocks |
| `statement_type` | attribute | Query type (SELECT, INSERT, UPDATE, DELETE, OTHER) | - |
| `collection_timestamp` | attribute | Collection time in UTC | ISO 8601 |
| `individual_query` | attribute | Actual query text (RDS mode only) | - |

**Collection Requirements**:
- Extension: `pg_stat_statements`
- Minimum PostgreSQL version: 12

**Example Query**:
```sql
FROM PostgresSlowQueries 
SELECT average(avg_elapsed_time_ms) 
WHERE database_name = 'production' 
FACET statement_type 
SINCE 1 hour ago
```

### 2. PostgresWaitEvents

Tracks wait events from pg_wait_sampling or pg_stat_activity.

**Event Type**: `PostgresWaitEvents`

| Metric Name | Type | Description | Unit |
|------------|------|-------------|------|
| `query_id` | attribute | Query identifier | - |
| `pid` | attribute | Process ID | - |
| `wait_event_type` | attribute | Wait event class (Lock, IO, etc.) | - |
| `wait_event` | attribute | Specific wait event | - |
| `count` | gauge | Number of samples | count |
| `duration_ms` | gauge | Total wait time | milliseconds |
| `database_name` | attribute | Database name | - |
| `collection_timestamp` | attribute | Collection time | ISO 8601 |

**Collection Requirements**:
- Extension: `pg_wait_sampling` (preferred) or fallback to pg_stat_activity
- Not available in RDS mode without extension

### 3. PostgresBlockingSessions

Tracks blocking and blocked database sessions.

**Event Type**: `PostgresBlockingSessions`

| Metric Name | Type | Description | Unit |
|------------|------|-------------|------|
| `blocking_pid` | attribute | PID of blocking session | - |
| `blocked_pid` | attribute | PID of blocked session | - |
| `blocking_query` | attribute | Query causing the block | - |
| `blocked_query` | attribute | Query being blocked | - |
| `blocking_duration_ms` | gauge | How long the block has existed | milliseconds |
| `database_name` | attribute | Database name | - |
| `blocking_lock_type` | attribute | Type of lock | - |
| `blocking_relation` | attribute | Table/index being locked | - |
| `collection_timestamp` | attribute | Collection time | ISO 8601 |

**Version-specific Queries**:
- PostgreSQL 12-13: Uses different query structure
- PostgreSQL 14+: Enhanced with more detail

### 4. PostgresIndividualQueries

Detailed metrics for individual query executions.

**Event Type**: `PostgresIndividualQueries`

| Metric Name | Type | Description | Unit |
|------------|------|-------------|------|
| `query_id` | attribute | Query identifier | - |
| `query_text` | attribute | Full query text | - |
| `pid` | attribute | Process ID | - |
| `user` | attribute | PostgreSQL user | - |
| `database_name` | attribute | Database name | - |
| `state` | attribute | Query state (active, idle, etc.) | - |
| `wait_event_type` | attribute | Current wait event type | - |
| `wait_event` | attribute | Current wait event | - |
| `backend_start` | attribute | Backend start time | ISO 8601 |
| `xact_start` | attribute | Transaction start time | ISO 8601 |
| `query_start` | attribute | Query start time | ISO 8601 |
| `state_change` | attribute | Last state change time | ISO 8601 |
| `backend_type` | attribute | Backend type | - |

**Collection Requirements**:
- Extension: `pg_stat_monitor` or RDS mode correlation

### 5. PostgresExecutionPlanMetrics

Query execution plan details.

**Event Type**: `PostgresExecutionPlanMetrics`

| Metric Name | Type | Description | Unit |
|------------|------|-------------|------|
| `query_id` | attribute | Query identifier | - |
| `query_text` | attribute | Query text | - |
| `database_name` | attribute | Database name | - |
| `plan` | attribute | JSON execution plan | JSON |
| `plan_text` | attribute | Text execution plan | - |
| `total_cost` | gauge | Estimated total cost | cost units |
| `execution_time_ms` | gauge | Actual execution time | milliseconds |
| `planning_time_ms` | gauge | Planning time | milliseconds |
| `collection_timestamp` | attribute | Collection time | ISO 8601 |

**Collection Requirements**:
- Requires EXPLAIN permission
- Limited by timeout setting (default 100ms)

## Extended Metrics

### 1. Query Latency Histogram

Provides distribution of query execution times.

**OTLP Metric**: `db.postgresql.query.duration`

| Bucket (ms) | Description |
|------------|-------------|
| 0.1 | Sub-millisecond queries |
| 0.5 | Fast queries |
| 1 | Normal queries |
| 5 | Slightly slow |
| 10 | Slow queries |
| 50 | Very slow |
| 100 | Concerning |
| 500 | Critical |
| 1000 | Very critical |
| 5000+ | Extremely slow |

**Example Usage**:
```promql
histogram_quantile(0.95,
  sum(rate(db_postgresql_query_duration_bucket[5m])) by (le, db_name)
)
```

### 2. Active Session History (ASH)

1-second resolution sampling of all active sessions.

**Event Type**: `PostgresASH` (NRI) / `db.postgresql.ash.sample` (OTLP)

| Metric Name | Type | Description | Unit |
|------------|------|-------------|------|
| `sample_time` | attribute | Sample timestamp | timestamp |
| `pid` | attribute | Process ID | - |
| `query_id` | attribute | Query identifier | - |
| `database_name` | attribute | Database name | - |
| `username` | attribute | PostgreSQL user | - |
| `state` | attribute | Session state | - |
| `wait_event_type` | attribute | Wait event type | - |
| `wait_event` | attribute | Specific wait event | - |
| `on_cpu` | attribute | Whether on CPU (eBPF) | boolean |
| `cpu_id` | attribute | CPU ID (eBPF) | - |
| `blocking_pids` | attribute | PIDs blocking this session | array |
| `memory_mb` | gauge | Session memory usage | megabytes |
| `temp_files_mb` | gauge | Temporary file usage | megabytes |

**ASH Analysis Queries**:
```sql
-- Top wait events over time
FROM PostgresASH 
SELECT count(*) 
WHERE wait_event IS NOT NULL 
FACET wait_event 
TIMESERIES 1 minute 
SINCE 1 hour ago

-- Active session count
FROM PostgresASH 
SELECT uniqueCount(pid) 
WHERE state = 'active' 
TIMESERIES 1 minute
```

### 3. Kernel Metrics (eBPF)

Low-level kernel metrics for query execution.

**Event Type**: `PostgresKernelMetrics`

| Metric Name | Type | Description | Unit |
|------------|------|-------------|------|
| `query_id` | attribute | Query identifier | - |
| `pid` | attribute | Process ID | - |
| `cpu_user_ns` | gauge | User CPU time | nanoseconds |
| `cpu_system_ns` | gauge | System CPU time | nanoseconds |
| `cpu_delay_ns` | gauge | CPU scheduling delay | nanoseconds |
| `voluntary_ctxt_switches` | counter | Voluntary context switches | count |
| `involuntary_ctxt_switches` | counter | Involuntary context switches | count |
| `io_wait_ns` | gauge | I/O wait time | nanoseconds |
| `block_io_delay_ns` | gauge | Block I/O delay | nanoseconds |
| `read_syscalls` | counter | Read system calls | count |
| `write_syscalls` | counter | Write system calls | count |
| `read_bytes` | counter | Bytes read | bytes |
| `write_bytes` | counter | Bytes written | bytes |

**Requirements**:
- eBPF feature enabled
- CAP_SYS_ADMIN capability
- Linux kernel 5.4+

### 4. Plan Change Events

Tracks query plan changes and regressions.

**Event Type**: `PostgresPlanChanges`

| Metric Name | Type | Description | Unit |
|------------|------|-------------|------|
| `query_id` | attribute | Query identifier | - |
| `old_plan_id` | attribute | Previous plan hash | - |
| `new_plan_id` | attribute | New plan hash | - |
| `old_cost` | gauge | Previous plan cost | cost units |
| `new_cost` | gauge | New plan cost | cost units |
| `performance_change_pct` | gauge | Performance change | percentage |
| `is_regression` | attribute | Whether this is a regression | boolean |
| `change_timestamp` | attribute | When change detected | timestamp |

## Metric Attributes

### Common Attributes

All metrics include these attributes:

| Attribute | Description | Example |
|-----------|-------------|---------|
| `collector.version` | Collector version | "1.0.0" |
| `postgres.version` | PostgreSQL version | "15.2" |
| `postgres.host` | Host identifier | "postgres-primary" |
| `postgres.port` | Port number | "5432" |
| `postgres.is_rds` | Whether RDS instance | "false" |
| `environment` | Environment tag | "production" |

### OTLP Semantic Conventions

When exported via OTLP, metrics follow OpenTelemetry database conventions:

| OTel Attribute | Description | Example |
|----------------|-------------|---------|
| `db.system` | Database system | "postgresql" |
| `db.connection_string` | Sanitized connection | "postgresql://host:5432" |
| `db.name` | Database name | "myapp" |
| `db.operation` | Operation type | "SELECT" |
| `db.statement` | SQL statement | "SELECT * FROM..." |
| `service.name` | Service name | "postgresql" |
| `service.version` | Service version | "15.2" |

## Query Examples

### NRQL (New Relic Query Language)

#### Top Slow Queries
```sql
FROM PostgresSlowQueries 
SELECT average(avg_elapsed_time_ms) as 'Avg Duration', 
       sum(execution_count) as 'Executions'
WHERE database_name = 'production'
FACET query_id 
LIMIT 20
SINCE 1 hour ago
```

#### Wait Event Analysis
```sql
FROM PostgresWaitEvents 
SELECT sum(duration_ms) 
FACET wait_event_type, wait_event 
WHERE duration_ms > 100 
SINCE 1 hour ago
```

#### Blocking Session Duration
```sql
FROM PostgresBlockingSessions 
SELECT max(blocking_duration_ms) 
FACET blocking_query 
WHERE blocking_duration_ms > 5000 
SINCE 30 minutes ago
```

#### ASH CPU vs Wait Time
```sql
FROM PostgresASH
SELECT percentage(count(*), WHERE on_cpu = true) as 'CPU %',
       percentage(count(*), WHERE wait_event IS NOT NULL) as 'Wait %'
TIMESERIES 1 minute
SINCE 1 hour ago
```

### PromQL (Prometheus Query Language)

#### Query Duration Percentiles
```promql
histogram_quantile(0.95,
  sum(rate(db_postgresql_query_duration_bucket[5m])) by (le, database_name)
)
```

#### Query Execution Rate
```promql
sum(rate(db_postgresql_query_total[5m])) by (database_name, statement_type)
```

#### Buffer Cache Hit Ratio
```promql
(
  sum(rate(db_postgresql_blocks_hit[5m])) by (database_name)
  /
  sum(rate(db_postgresql_blocks_read[5m]) + rate(db_postgresql_blocks_hit[5m])) by (database_name)
) * 100
```

## Dashboards and Alerts

### Pre-built Dashboards

#### 1. Query Performance Dashboard
- Top slow queries by duration and frequency
- Query duration percentiles (p50, p95, p99)
- Statement type breakdown
- Execution count trends

#### 2. Wait Event Analysis
- Wait event heatmap
- Top wait events by time
- Wait event trends
- Lock wait analysis

#### 3. Active Session History
- Session count over time
- CPU vs wait time ratio
- Top SQL by active sessions
- Blocking session graph

#### 4. Resource Utilization
- Buffer cache efficiency
- Disk I/O patterns
- Connection pool usage
- Memory consumption

### Alert Examples

#### Critical Alerts

```yaml
# Long-running query
alert: LongRunningQuery
expr: max(db_postgresql_query_duration{quantile="0.99"}) > 30000
for: 5m
severity: critical

# High blocking session count
alert: HighBlockingSessions
expr: count(postgres_blocking_sessions) > 10
for: 5m
severity: critical

# Low cache hit ratio
alert: LowCacheHitRatio
expr: |
  (
    sum(rate(db_postgresql_blocks_hit[5m]))
    /
    sum(rate(db_postgresql_blocks_hit[5m]) + rate(db_postgresql_blocks_read[5m]))
  ) < 0.9
for: 10m
severity: warning
```

### Custom Metrics

To add custom metrics, query any PostgreSQL view or table:

```toml
[[custom_queries]]
name = "replication_lag"
query = """
SELECT 
  client_addr,
  pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) as lag_bytes
FROM pg_stat_replication
"""
interval = "30s"
metrics = [
  {name = "postgres.replication.lag_bytes", type = "gauge", value_column = "lag_bytes"}
]
```

## Performance Impact

### Metric Collection Overhead

| Metric Type | Typical Overhead | Notes |
|------------|------------------|-------|
| Slow Queries | < 0.1% | Uses existing pg_stat_statements |
| Wait Events | < 0.5% | Depends on sampling rate |
| Blocking Sessions | < 0.1% | Lightweight query |
| Individual Queries | < 1% | Can be higher with many active queries |
| Execution Plans | Variable | Limited by timeout |
| ASH | < 0.5% | 1-second sampling |
| Kernel Metrics | < 1% | eBPF overhead |

### Optimization Tips

1. **Adjust Collection Intervals**: Increase intervals for non-critical metrics
2. **Use Sampling**: Enable adaptive sampling for high-volume databases
3. **Limit Plan Collection**: Set appropriate timeouts and limits
4. **Disable Unused Features**: Turn off eBPF or ASH if not needed
5. **Index Optimization**: Ensure pg_stat_statements queries are fast

## Metric Retention

### New Relic Retention
- Default: 8 days for metrics
- Extended: 30+ days with Data Plus
- Downsampling: Automatic after 2 hours

### Prometheus Retention
- Depends on local storage configuration
- Typical: 15 days raw, 1 year downsampled
- Consider remote write for long-term storage

## Next Steps

- [Migration Guide](05-migration-guide.md) - Upgrade from nri-postgresql
- [Deployment Guide](03-deployment-operations.md) - Installation and configuration
- [Architecture Overview](01-architecture-overview.md) - System design details