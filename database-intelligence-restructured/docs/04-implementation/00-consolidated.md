# Implementation Analysis

This document consolidates all documentation in this section.

---


## PostgreSQL Metrics Implementation Analysis Report

# PostgreSQL Metrics Implementation Analysis Report

## Executive Summary

This report analyzes the current PostgreSQL monitoring implementation to identify which metrics are collected using out-of-the-box (OOTB) OpenTelemetry components versus custom implementations. All data has been verified against actual metrics in New Relic Database (NRDB).

**Analysis Date**: July 10, 2025  
**Account ID**: 3630072  
**Total Unique PostgreSQL Metrics**: 34

## Metric Categories

### 1. OOTB PostgreSQL Receiver Metrics (OpenTelemetry Community)

The following metrics are collected using the standard OpenTelemetry PostgreSQL receiver:

| Metric Name | Type | Description | Key Attributes |
|------------|------|-------------|----------------|
| `postgresql.backends` | Gauge | Number of backend connections | - `postgresql.database.name`<br>- `db.system: postgresql` |
| `postgresql.db_size` | Gauge | Database disk usage in bytes | - `postgresql.database.name`<br>- `unit: By` |
| `postgresql.commits` | Counter | Number of commits | - `postgresql.database.name`<br>- Cumulative count |
| `postgresql.rollbacks` | Counter | Number of rollbacks | - `postgresql.database.name` |
| `postgresql.connection.max` | Gauge | Maximum connections allowed | - Standard OOTB metric |
| `postgresql.database.count` | Gauge | Number of databases | - Standard OOTB metric |
| `postgresql.operations` | Counter | Database operations | - Operation type faceted |
| `postgresql.bgwriter.buffers.writes` | Counter | Buffer writes by background writer | - Write source type |
| `postgresql.bgwriter.buffers.allocated` | Counter | Buffers allocated | - Background writer metric |
| `postgresql.bgwriter.checkpoint.count` | Counter | Checkpoint count | - Checkpoint type |
| `postgresql.bgwriter.duration` | Gauge | Checkpoint duration | - Time in milliseconds |
| `postgresql.bgwriter.maxwritten` | Counter | Max written stops | - Background writer metric |
| `postgresql.blocks_read` | Counter | Blocks read from disk | - By database/table |
| `postgresql.index.scans` | Counter | Index scans | - `postgresql.index.name`<br>- `postgresql.table.name` |
| `postgresql.index.size` | Gauge | Index size | - `postgresql.index.name` |
| `postgresql.table.size` | Gauge | Table size | - `postgresql.table.name` |
| `postgresql.table.count` | Gauge | Number of tables | - By database |
| `postgresql.table.vacuum.count` | Counter | Vacuum operations | - `postgresql.table.name` |
| `postgresql.rows` | Counter | Rows affected | - By operation type |

**OOTB Receiver Configuration**:
```yaml
postgresql:
  endpoint: localhost:5432
  username: postgres
  password: pass
  collection_interval: 10s
  databases:
    - postgres
```

### 2. Custom SQLQuery Receiver Metrics

The following metrics are collected using custom SQL queries via the SQLQuery receiver:

| Metric Name | Type | Description | Source Query | Custom Attributes |
|------------|------|-------------|--------------|-------------------|
| `postgres.slow_queries.count` | Gauge | Execution count per query | pg_stat_statements | - `query_id`<br>- `query_text`<br>- `statement_type` |
| `postgres.slow_queries.elapsed_time` | Gauge | Average execution time (ms) | pg_stat_statements | - `db.statement` (transformed)<br>- `db.operation` (transformed) |
| `postgres.slow_queries.disk_reads` | Gauge | Average disk reads per query | pg_stat_statements | - `db.postgresql.query_id` (transformed) |
| `postgres.slow_queries.disk_writes` | Gauge | Average disk writes per query | pg_stat_statements | - `db.schema` (transformed) |
| `postgres.slow_queries.cpu_time` | Gauge | CPU time per query | pg_stat_statements | - Same as elapsed_time |
| `postgres.wait_events` | Gauge | Wait event occurrences | pg_stat_activity | - `db.wait_event.name`<br>- `db.wait_event.category` |
| `postgres.blocking_sessions` | Gauge | Active blocking sessions | pg_stat_activity + pg_blocking_pids() | - `db.blocking.blocked_pid`<br>- `db.blocking.blocking_pid` |
| `postgres.individual_queries.cpu_time` | Gauge | CPU time for individual queries | pg_stat_statements | - `db.postgresql.plan_id` |
| `postgres.execution_plan.cost` | Gauge | Query execution plan cost | pg_stat_statements (simplified) | - `db.plan.node_type`<br>- `db.plan.level` |
| `postgres.execution_plan.time` | Gauge | Actual execution time | pg_stat_statements | - Plan-level metrics |
| `postgres.execution_plan.rows` | Gauge | Rows processed | pg_stat_statements | - Plan-level metrics |
| `postgres.execution_plan.blocks_hit` | Counter | Cache hits | pg_stat_statements | - Plan-level metrics |
| `postgres.execution_plan.blocks_read` | Counter | Disk reads | pg_stat_statements | - Plan-level metrics |

### 3. Infrastructure Agent Metrics (Not from OTel)

These metrics appear in NRDB but are from the New Relic Infrastructure agent, not our OpenTelemetry collector:

- `newrelic.goldenmetrics.infra.postgresqlinstance.scheduledCheckpoints`
- `newrelic.goldenmetrics.infra.postgresqlinstance.buffersAllocated`
- `newrelic.goldenmetrics.infra.postgresqlinstance.requestedCheckpoints`

## Attribute Transformations

### Transform Processor Mappings

The following attribute transformations are applied to align with OpenTelemetry semantic conventions:

| Original Attribute | Transformed Attribute | Applied To |
|-------------------|----------------------|------------|
| `database_name` | `db.name` | All custom metrics |
| `query_text` | `db.statement` | Slow query metrics |
| `query_id` | `db.postgresql.query_id` | Query-related metrics |
| `statement_type` | `db.operation` | Slow query metrics |
| `schema_name` | `db.schema` | Query metrics |
| `wait_event_name` | `db.wait_event.name` | Wait event metrics |
| `wait_category` | `db.wait_event.category` | Wait event metrics |
| `node_type` | `db.plan.node_type` | Execution plan metrics |
| `level_id` | `db.plan.level` | Execution plan metrics |
| `plan_id` | `db.postgresql.plan_id` | Plan metrics |

### Resource Attributes (Added by Processors)

All metrics receive these resource attributes:
- `environment: e2e-test`
- `service.name: database-intelligence`
- `db.system: postgresql`

## Key Findings

### 1. OOTB vs Custom Split
- **18 metrics** (53%) from OOTB PostgreSQL receiver
- **13 metrics** (38%) from custom SQLQuery receivers
- **3 metrics** (9%) from Infrastructure agent (not OTel)

### 2. Custom Implementation Rationale

Custom implementations were necessary for:

1. **Query Performance Metrics**: pg_stat_statements data not available in OOTB receiver
2. **Wait Events**: Real-time wait analysis requires pg_stat_activity queries
3. **Blocking Sessions**: Complex joins with pg_blocking_pids() function
4. **Execution Plans**: Would require auto_explain extension in OOTB

### 3. Data Enrichment

Custom metrics provide significantly more context:
- Query text and identifiers
- Statement type classification
- Wait event categorization
- Blocking relationship mapping
- Execution plan details

### 4. Collection Intervals

- OOTB metrics: 10-second intervals
- Custom slow queries: 15-second intervals
- Wait events: 10-second intervals
- Execution plans: 60-second intervals

## Verification Summary

All metrics verified in NRDB show:
- ✅ Proper attribute transformation working
- ✅ Resource attributes correctly applied
- ✅ Both OOTB and custom metrics flowing
- ✅ Semantic conventions followed
- ✅ No data loss or duplication

## Recommendations

1. **Standardization**: Consider contributing query performance metrics back to OpenTelemetry PostgreSQL receiver
2. **Performance**: Monitor overhead of custom SQL queries, especially pg_stat_statements joins
3. **Coverage**: Add custom receivers for missing OHI events (e.g., replication metrics)
4. **Documentation**: Document why each custom metric requires special handling

## Conclusion

The implementation successfully combines OOTB OpenTelemetry components (53% of metrics) with necessary custom implementations (38% of metrics) to achieve full PostgreSQL observability. The custom implementations are justified by the need for query-level insights and real-time blocking analysis not available in the standard receiver.

---


## PostgreSQL Metrics Implementation Analysis Summary

# PostgreSQL Metrics Implementation Analysis Summary

## Implementation Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    PostgreSQL Database                          │
├─────────────────────────────────────────────────────────────────┤
│  System Catalogs          │  Statistics Views                   │
│  - pg_database            │  - pg_stat_statements              │
│  - pg_stat_user_tables    │  - pg_stat_activity                │
│  - pg_stat_user_indexes   │  - pg_blocking_pids()              │
└─────────────────────────────────────────────────────────────────┘
                    │                         │
                    ▼                         ▼
┌─────────────────────────┐    ┌──────────────────────────────────┐
│   OOTB PostgreSQL       │    │    Custom SQLQuery Receivers     │
│      Receiver           │    ├──────────────────────────────────┤
├─────────────────────────┤    │ • sqlquery/slow_queries          │
│ • postgresql.backends   │    │ • sqlquery/wait_events           │
│ • postgresql.db_size    │    │ • sqlquery/blocking_sessions     │
│ • postgresql.commits    │    │ • sqlquery/individual_queries    │
│ • postgresql.rollbacks  │    │ • sqlquery/execution_plans       │
│ • postgresql.operations │    └──────────────────────────────────┘
│ • postgresql.blocks_*   │                    │
│ • postgresql.bgwriter.* │                    │
│ • postgresql.index.*    │                    │
│ • postgresql.table.*    │                    │
└─────────────────────────┘                    │
                    │                          │
                    ▼                          ▼
            ┌────────────────────────────────────┐
            │      Transform Processor           │
            ├────────────────────────────────────┤
            │ Attribute Mappings:                │
            │ • database_name → db.name          │
            │ • query_text → db.statement        │
            │ • query_id → db.postgresql.query_id│
            │ • statement_type → db.operation    │
            │ • wait_event_name → db.wait_event.*│
            └────────────────────────────────────┘
                            │
                            ▼
            ┌────────────────────────────────────┐
            │    Resource & Attributes Processor │
            ├────────────────────────────────────┤
            │ • db.system: postgresql            │
            │ • environment: e2e-test            │
            │ • service.name: database-intel...  │
            └────────────────────────────────────┘
                            │
                            ▼
            ┌────────────────────────────────────┐
            │         OTLP Exporter              │
            │    → New Relic OTLP Endpoint       │
            └────────────────────────────────────┘
```

## Data Collection Breakdown

### 1. Out-of-the-Box (OOTB) OpenTelemetry PostgreSQL Receiver

**What it collects:**
- **Connection Metrics**: Active backends, max connections
- **Database Metrics**: Size, count
- **Transaction Metrics**: Commits, rollbacks, deadlocks
- **Performance Metrics**: Block reads/hits, buffer cache
- **Background Writer**: Checkpoints, buffers, duration
- **Table Statistics**: Size, vacuum counts, rows affected
- **Index Statistics**: Scans, size

**Collection Method**: Direct connection to PostgreSQL using Go's `database/sql` driver

**Key Characteristics:**
- No custom SQL required
- Standard PostgreSQL statistics views
- Semantic conventions compliant
- 10-second collection interval
- Minimal configuration needed

### 2. Custom SQLQuery Receiver Implementation

**What it collects:**
- **Query Performance** (pg_stat_statements):
  - Execution counts and times
  - Disk I/O per query
  - Query text and identifiers
  - CPU time estimates
  
- **Wait Events** (pg_stat_activity):
  - Real-time wait analysis
  - Wait categories and types
  - Associated queries
  
- **Blocking Sessions** (pg_blocking_pids):
  - Active blocks
  - Blocking/blocked query details
  - Process relationships
  
- **Execution Plans** (simplified from pg_stat_statements):
  - Cost estimates
  - Row counts
  - I/O statistics

**Collection Method**: Custom SQL queries executed via SQLQuery receiver

**Key Characteristics:**
- Requires pg_stat_statements extension
- Complex SQL with CTEs and joins
- Custom attribute extraction
- Variable collection intervals (10-60s)
- Semantic transformation required

## Why Custom Implementation Was Necessary

### 1. Query-Level Insights
- OOTB receiver provides database/table level metrics only
- No query text or performance data in standard receiver
- pg_stat_statements requires specific SQL queries

### 2. Real-Time Wait Analysis
- pg_stat_activity sampling not in OOTB receiver
- Wait events critical for performance troubleshooting
- Requires point-in-time queries

### 3. Blocking Detection
- Complex pg_blocking_pids() function calls
- Relationship mapping between sessions
- Not standard PostgreSQL statistics

### 4. OHI Feature Parity
- New Relic OHI collects these specific metrics
- Customer expectation for migration
- Dashboard compatibility requirements

## Data Verification Results

Based on NRDB analysis:

1. **Total Metrics**: 34 unique PostgreSQL-related metrics
2. **OOTB Metrics**: 18 (53%)
3. **Custom Metrics**: 13 (38%)
4. **Infrastructure Agent**: 3 (9%)

All metrics show proper:
- ✅ Attribute transformation
- ✅ Resource labeling
- ✅ Semantic conventions
- ✅ Data flow to New Relic

## Key Insights

1. **Hybrid Approach**: Successfully combines OOTB components with necessary customizations
2. **Semantic Compliance**: All custom metrics transformed to follow OpenTelemetry conventions
3. **Performance Impact**: Custom queries add overhead but provide essential insights
4. **Maintainability**: Clear separation between standard and custom implementations

## Recommendations

1. **Upstream Contribution**: Consider contributing query performance metrics to OpenTelemetry
2. **Extension Detection**: Add automatic pg_stat_statements detection
3. **Query Optimization**: Monitor and optimize custom SQL query performance
4. **Documentation**: Maintain clear documentation on why each custom metric exists

This implementation achieves the goal of full PostgreSQL observability while maximizing use of standard OpenTelemetry components where possible.

---
