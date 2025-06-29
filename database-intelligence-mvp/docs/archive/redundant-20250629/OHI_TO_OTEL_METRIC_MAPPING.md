# OHI to OpenTelemetry Metrics Mapping

## Overview

This document shows how PostgreSQL On-Host Integration (OHI) capabilities are mapped to OpenTelemetry dimensional metrics.

## Key Principles

1. **Dimensional Metrics Only**: All data is collected as proper OTEL metrics with dimensions (attributes)
2. **No Events/Logs**: Unlike OHI which uses event types, we use metric names and attributes
3. **Cardinality Control**: Query patterns are normalized to prevent metric explosion
4. **Same Capabilities**: All OHI monitoring capabilities are preserved

## Metric Mappings

### 1. PostgreSQLSample → PostgreSQL Receiver Metrics

| OHI Metric | OTEL Metric | Dimensions |
|------------|-------------|------------|
| `db.bgwriter.checkpointsScheduledPerSecond` | `postgresql.bgwriter.checkpoint.count` | - |
| `db.bgwriter.checkpointWriteTimeInMillisecondsPerSecond` | `postgresql.bgwriter.duration` | type=write |
| `db.bgwriter.buffersWrittenByBackgroundWriterPerSecond` | `postgresql.bgwriter.buffers.writes` | source=background |
| `db.bufferHitRatio` | calculated from `postgresql.blocks_read` | source={heap,index,toast} |
| `db.commitsPerSecond` | `postgresql.commits` | - |
| `db.rollbacksPerSecond` | `postgresql.rollbacks` | - |
| `db.reads.blocksPerSecond` | `postgresql.blocks_read` | source={heap,index,toast} |
| `db.writes.blocksPerSecond` | `postgresql.blocks_written` | - |

### 2. PostgresSlowQueries → Query Performance Metrics

| OHI Field | OTEL Metric | Dimensions |
|-----------|-------------|------------|
| `execution_count` | `db.query.count` | query_id, database_name, schema_name, statement_type |
| `avg_elapsed_time_ms` | `db.query.mean_duration` | query_id, database_name, schema_name, statement_type |
| `total_exec_time` | `db.query.duration` | query_id, database_name, schema_name, statement_type |
| `avg_disk_reads` | `db.io.disk_reads` | query_id, database_name |
| `avg_disk_writes` | `db.io.disk_writes` | query_id, database_name |
| `rows` | `db.query.rows` | query_id, database_name, statement_type |

### 3. PostgresWaitEvents → Wait Event Metrics

| OHI Field | OTEL Metric | Dimensions |
|-----------|-------------|------------|
| wait event counts | `db.wait_events` | database_name, wait_event_type, wait_event |

### 4. PostgresBlockingSessions → Connection Metrics

| OHI Field | OTEL Metric | Dimensions |
|-----------|-------------|------------|
| blocked sessions | `db.connections.blocked` | database_name |
| active sessions | `db.connections.active` | database_name |
| idle sessions | `db.connections.idle` | database_name |
| waiting sessions | `db.connections.waiting` | database_name |

### 5. Additional Metrics (Beyond OHI)

| Capability | OTEL Metric | Dimensions |
|------------|-------------|------------|
| Replication lag | `db.replication.lag` | application_name, state, sync_state |
| Replication lag time | `db.replication.lag_time` | application_name, state |
| Cache hit ratio | `db.io.cache_hits` | query_id, database_name |

## Query Examples

### OHI Query:
```sql
SELECT average(avg_elapsed_time_ms)
FROM PostgresSlowQueries
WHERE database_name = 'production'
FACET statement_type
```

### Equivalent OTEL Query:
```sql
SELECT average(db.query.mean_duration)
FROM Metric
WHERE db.system = 'postgresql'
  AND database_name = 'production'
FACET statement_type
```

### OHI Query:
```sql
SELECT count(*)
FROM PostgresBlockingSessions
WHERE database_name = 'production'
```

### Equivalent OTEL Query:
```sql
SELECT latest(db.connections.blocked)
FROM Metric
WHERE db.system = 'postgresql'
  AND database_name = 'production'
```

## Advantages of OTEL Approach

1. **Standard Metrics**: Uses OpenTelemetry semantic conventions
2. **Better Aggregation**: Proper metric types (counter, gauge, histogram)
3. **Flexible Queries**: Can aggregate across multiple dimensions
4. **Lower Storage**: Metrics are more efficient than event logs
5. **Better Performance**: Pre-aggregated at collection time

## Configuration Highlights

```yaml
receivers:
  postgresql:  # Standard PostgreSQL metrics
  sqlquery/postgresql_queries:  # Query performance metrics

processors:
  transform/query_normalization:  # Normalize queries for cardinality
  filter/cardinality:  # Drop low-value metrics
  
pipelines:
  metrics:  # Single pipeline for all metrics
    receivers: [postgresql, sqlquery/postgresql_queries]
    exporters: [otlp/newrelic]
```

## Migration Guide

1. **Dashboards**: Update NRQL queries to use Metric instead of event types
2. **Alerts**: Change from event-based to metric-based conditions
3. **Attributes**: Use metric dimensions instead of event attributes
4. **Aggregation**: Leverage OTEL's built-in aggregation temporality