# NRDB Data Model for PostgreSQL Database Intelligence

This document describes how PostgreSQL metrics and intelligence data are mapped to New Relic Database (NRDB) for storage and querying.

## Overview

The Database Intelligence Collector sends metrics to New Relic using the OpenTelemetry Protocol (OTLP). These metrics are transformed and stored in NRDB with specific naming conventions and attributes that enable powerful querying capabilities.

## Metric Naming Conventions

All metrics follow a hierarchical naming pattern:

```
postgresql.<category>.<specific_metric>
```

### Categories:
- `postgresql.connections.*` - Connection pool metrics
- `postgresql.transactions.*` - Transaction metrics
- `postgresql.blocks.*` - Buffer cache metrics
- `postgresql.database.*` - Database-level metrics
- `postgresql.query.*` - Query performance metrics
- `postgresql.plan.*` - Query plan intelligence
- `postgresql.ash.*` - Active Session History metrics

## Core Metrics

### Infrastructure Metrics

| Metric Name | Type | Description | Key Attributes |
|------------|------|-------------|----------------|
| `postgresql.connections.active` | Gauge | Current active connections | `db.name`, `db.system` |
| `postgresql.connections.idle` | Gauge | Current idle connections | `db.name`, `db.system` |
| `postgresql.connections.max` | Gauge | Maximum allowed connections | `db.name`, `db.system` |
| `postgresql.transactions.committed` | Counter | Committed transactions | `db.name`, `db.system` |
| `postgresql.transactions.rolled_back` | Counter | Rolled back transactions | `db.name`, `db.system` |
| `postgresql.blocks.hit` | Counter | Buffer cache hits | `db.name`, `db.system` |
| `postgresql.blocks.read` | Counter | Disk blocks read | `db.name`, `db.system` |
| `postgresql.database.size` | Gauge | Database size in bytes | `db.name`, `db.system` |

### Query Performance Metrics

| Metric Name | Type | Description | Key Attributes |
|------------|------|-------------|----------------|
| `postgresql.query.execution` | Histogram | Query execution metrics | `query.normalized`, `query.user`, `query.database`, `query.application_name` |
| `query.exec_time_ms` | Attribute | Execution time in ms | - |
| `query.plan_time_ms` | Attribute | Planning time in ms | - |
| `query.rows_affected` | Attribute | Number of rows affected | - |

### Plan Intelligence Metrics

| Metric Name | Type | Description | Key Attributes |
|------------|------|-------------|----------------|
| `postgresql.plan.change` | Event | Plan change detected | `query.normalized`, `plan.old_hash`, `plan.new_hash`, `plan.change_type` |
| `postgresql.plan.regression` | Event | Plan regression detected | `query.normalized`, `plan.cost_increase_ratio`, `plan.performance_impact` |
| `postgresql.plan.node` | Event | Plan node analysis | `plan.node_type`, `plan.issue_type`, `query.normalized` |

### Active Session History Metrics

| Metric Name | Type | Description | Key Attributes |
|------------|------|-------------|----------------|
| `postgresql.ash.session` | Gauge | Session snapshot | `session.pid`, `session.state`, `session.backend_type`, `query.normalized` |
| `postgresql.ash.wait_event` | Histogram | Wait event duration | `wait.event_type`, `wait.event_name`, `session.pid`, `query.normalized` |
| `postgresql.ash.blocking` | Event | Blocking detected | `blocking.pid`, `blocked.pid`, `blocking.query`, `blocked.query` |

## Attribute Definitions

### Standard Attributes (from OpenTelemetry semantic conventions)

| Attribute | Description | Example |
|-----------|-------------|---------|
| `db.system` | Database system | `postgresql` |
| `db.name` | Database name | `production_db` |
| `db.user` | Database user | `app_user` |
| `service.name` | Service identifier | `postgresql-database-intelligence` |
| `deployment.environment` | Environment | `production`, `staging`, `development` |
| `host.id` | Host identifier | `i-1234567890abcdef0` |

### Query Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `query.normalized` | Normalized query (hashed for PII) | `SELECT * FROM users WHERE id = ?` |
| `query.user` | User executing query | `app_user` |
| `query.database` | Database for query | `production_db` |
| `query.application_name` | Application name | `web-api` |
| `query.hash` | Query hash | `a1b2c3d4e5f6` |

### Plan Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `plan.hash` | Plan hash | `p1q2r3s4t5u6` |
| `plan.node_type` | Plan node type | `Seq Scan`, `Index Scan`, `Nested Loop` |
| `plan.old_hash` | Previous plan hash | `p0q0r0s0t0u0` |
| `plan.new_hash` | New plan hash | `p1q2r3s4t5u6` |
| `plan.change_type` | Type of change | `improvement`, `regression`, `neutral` |
| `plan.cost_increase_ratio` | Cost increase ratio | `2.5` |
| `plan.performance_impact` | Performance impact | `high`, `medium`, `low` |
| `plan.anonymized` | Plan is anonymized | `true`, `false` |
| `plan.issue_type` | Detected issue | `missing_index`, `bad_estimate`, `expensive_operation` |

### Session Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `session.pid` | Process ID | `12345` |
| `session.state` | Session state | `active`, `idle`, `idle in transaction` |
| `session.backend_type` | Backend type | `client backend`, `autovacuum`, `background writer` |
| `session.wait_event_type` | Wait event type | `Lock`, `IO`, `CPU` |
| `session.wait_event` | Specific wait event | `relation`, `extend`, `WALWrite` |
| `session.blocking_pid` | PID of blocking session | `12346` |
| `session.cpu_usage` | CPU usage percentage | `45.2` |
| `session.memory_mb` | Memory usage in MB | `128.5` |

### Wait Event Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `wait.event_type` | Wait event category | `Lock`, `IO`, `Client`, `Extension` |
| `wait.event_name` | Specific wait event | `relation`, `DataFileRead`, `ClientRead` |
| `wait.duration_ms` | Wait duration in ms | `150` |
| `wait.severity` | Wait severity | `high`, `medium`, `low` |

## NRQL Query Patterns

### Basic Patterns

1. **Latest Value Query**:
```sql
SELECT latest(metric_name) 
FROM Metric 
WHERE db.system = 'postgresql'
```

2. **Time Series Query**:
```sql
SELECT average(metric_name) 
FROM Metric 
WHERE db.system = 'postgresql' 
TIMESERIES 1 minute 
SINCE 1 hour ago
```

3. **Faceted Query**:
```sql
SELECT count(*) 
FROM Metric 
WHERE metricName = 'postgresql.query.execution' 
FACET query.normalized 
SINCE 1 hour ago
```

4. **Percentile Query**:
```sql
SELECT percentile(query.exec_time_ms, 50, 95, 99) 
FROM Metric 
WHERE metricName = 'postgresql.query.execution' 
SINCE 1 hour ago
```

### Advanced Patterns

1. **Correlated Metrics**:
```sql
SELECT average(query.exec_time_ms) as 'Exec Time',
       sum(wait.duration_ms) as 'Wait Time'
FROM Metric 
WHERE metricName IN ('postgresql.query.execution', 'postgresql.ash.wait_event')
  AND query.normalized IS NOT NULL
FACET query.normalized
```

2. **Change Detection**:
```sql
SELECT count(*) 
FROM Metric 
WHERE metricName = 'postgresql.plan.regression' 
COMPARE WITH 1 hour ago
```

3. **Top-N Analysis**:
```sql
SELECT sum(wait.duration_ms) 
FROM Metric 
WHERE metricName = 'postgresql.ash.wait_event' 
FACET wait.event_name 
ORDER BY sum(wait.duration_ms) DESC 
LIMIT 10
```

## Data Retention and Sampling

### Retention Policies

- **High-frequency metrics** (1s interval): 8 days
- **Standard metrics** (10s interval): 30 days
- **Aggregated metrics** (1m interval): 13 months

### Adaptive Sampling

The collector implements adaptive sampling to control data volume:

1. **Always sampled**:
   - Plan regressions
   - Blocking sessions
   - Queries > 1s execution time

2. **Reduced sampling**:
   - High-frequency queries (sample 10%)
   - Idle sessions (sample 20%)
   - Successful fast queries (sample 5%)

## Best Practices

### 1. Attribute Naming
- Use dots for hierarchy: `query.normalized`, `plan.hash`
- Use underscores within names: `exec_time_ms`, `cost_increase_ratio`
- Keep names descriptive but concise

### 2. Cardinality Management
- Normalize queries to reduce cardinality
- Hash sensitive values
- Use sampling for high-cardinality metrics
- Set limits on unique values per attribute

### 3. Query Optimization
- Use appropriate time windows
- Leverage faceting for grouping
- Use LIMIT for top-N queries
- Consider using summary metrics for dashboards

### 4. PII Protection
- Never send raw query text
- Hash or redact sensitive values
- Use normalized queries
- Implement client-side anonymization

## Troubleshooting

### Missing Data
```sql
-- Check data flow
SELECT count(*) 
FROM Metric 
WHERE db.system = 'postgresql' 
FACET metricName 
SINCE 5 minutes ago

-- Check last data point
SELECT latest(timestamp) 
FROM Metric 
WHERE db.system = 'postgresql' 
FACET metricName
```

### High Cardinality
```sql
-- Check cardinality
SELECT uniqueCount(query.normalized) as 'Unique Queries',
       uniqueCount(session.pid) as 'Unique PIDs'
FROM Metric 
WHERE db.system = 'postgresql' 
SINCE 1 hour ago
```

### Data Quality
```sql
-- Check for nulls
SELECT count(*) 
FROM Metric 
WHERE db.system = 'postgresql' 
  AND query.normalized IS NULL 
SINCE 1 hour ago
```