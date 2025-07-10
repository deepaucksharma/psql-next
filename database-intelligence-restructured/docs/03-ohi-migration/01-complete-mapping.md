## Complete OHI to OpenTelemetry Metrics Mapping

# Complete OHI to OpenTelemetry Metrics Mapping

## Overview
This document provides a comprehensive mapping of all PostgreSQL OHI (On-Host Integration) metrics found in the dashboard to their OpenTelemetry dimensional metric equivalents.

## OHI Events and Their Metrics

### 1. PostgresSlowQueries Event

**OHI Attributes Used in Dashboard:**
- `query_id` - Unique identifier for the query
- `database_name` - Database where query executed
- `query_text` - SQL query text
- `schema_name` - Schema name
- `execution_count` - Number of times executed
- `avg_elapsed_time_ms` - Average execution time in milliseconds
- `avg_disk_reads` - Average disk blocks read
- `avg_disk_writes` - Average disk blocks written
- `statement_type` - Type of SQL statement (SELECT, INSERT, etc.)

**OpenTelemetry Mapping:**
```yaml
metric_name: db.statement.execution
attributes:
  - db.system: "postgresql"
  - db.name: "${database_name}"
  - db.statement: "${query_text}"
  - db.operation: "${statement_type}"
  - db.postgresql.query_id: "${query_id}"
  - db.schema: "${schema_name}"
values:
  - execution_count (counter)
  - avg_elapsed_time_ms (gauge)
  - avg_disk_reads (gauge)
  - avg_disk_writes (gauge)
```

### 2. PostgresWaitEvents Event

**OHI Attributes Used in Dashboard:**
- `wait_event_name` - Name of the wait event
- `wait_category` - Category of wait (CPU, IO, Lock, etc.)
- `total_wait_time_ms` - Total time spent waiting
- `database_name` - Database name
- `query_id` - Associated query ID

**OpenTelemetry Mapping:**
```yaml
metric_name: db.wait_events.time
unit: ms
attributes:
  - db.system: "postgresql"
  - db.wait_event.name: "${wait_event_name}"
  - db.wait_event.category: "${wait_category}"
  - db.name: "${database_name}"
  - db.postgresql.query_id: "${query_id}"
value: total_wait_time_ms (gauge)
```

### 3. PostgresBlockingSessions Event

**OHI Attributes Used in Dashboard:**
- `blocked_pid` - Process ID of blocked session
- `blocked_query` - Query being blocked
- `blocked_query_id` - ID of blocked query
- `blocked_query_start` - Start time of blocked query
- `database_name` - Database name
- `blocking_pid` - Process ID of blocking session
- `blocking_query` - Query doing the blocking
- `blocking_query_id` - ID of blocking query
- `blocking_query_start` - Start time of blocking query
- `blocking_database` - Database of blocking session

**OpenTelemetry Mapping:**
```yaml
metric_name: db.blocking_sessions.count
attributes:
  - db.system: "postgresql"
  - db.name: "${database_name}"
  - db.blocking.blocked_pid: "${blocked_pid}"
  - db.blocking.blocked_query: "${blocked_query}"
  - db.blocking.blocked_query_id: "${blocked_query_id}"
  - db.blocking.blocking_pid: "${blocking_pid}"
  - db.blocking.blocking_query: "${blocking_query}"
  - db.blocking.blocking_query_id: "${blocking_query_id}"
value: 1 (gauge, presence indicator)
```

### 4. PostgresIndividualQueries Event

**OHI Attributes Used in Dashboard:**
- `query_text` - SQL query text
- `avg_cpu_time_ms` - Average CPU time
- `query_id` - Query identifier
- `plan_id` - Execution plan identifier

**OpenTelemetry Mapping:**
```yaml
metric_name: db.query.cpu_time
unit: ms
attributes:
  - db.system: "postgresql"
  - db.statement: "${query_text}"
  - db.postgresql.query_id: "${query_id}"
  - db.postgresql.plan_id: "${plan_id}"
value: avg_cpu_time_ms (gauge)
```

### 5. PostgresExecutionPlanMetrics Event

**OHI Attributes Used in Dashboard:**
- `plan_id` - Execution plan identifier
- `level_id` - Level in plan hierarchy
- `node_type` - Type of plan node
- `query_id` - Associated query ID
- `query_text` - SQL query text
- `total_cost` - Total estimated cost
- `startup_cost` - Startup cost estimate
- `plan_rows` - Estimated rows
- `actual_startup_time` - Actual startup time
- `actual_total_time` - Actual total time
- `actual_rows` - Actual rows returned
- `actual_loops` - Number of loops
- `shared_hit_block` - Shared buffer hits
- `shared_read_blocks` - Shared blocks read
- `shared_dirtied_blocks` - Shared blocks dirtied
- `shared_written_blocks` - Shared blocks written
- `local_hit_block` - Local buffer hits
- `local_read_blocks` - Local blocks read
- `local_dirtied_blocks` - Local blocks dirtied
- `local_written_blocks` - Local blocks written
- `temp_read_block` - Temp blocks read
- `temp_written_blocks` - Temp blocks written
- `database_name` - Database name

**OpenTelemetry Mapping:**
```yaml
# Cost metrics
metric_name: db.plan.cost
attributes:
  - db.system: "postgresql"
  - db.name: "${database_name}"
  - db.postgresql.query_id: "${query_id}"
  - db.postgresql.plan_id: "${plan_id}"
  - db.plan.node_type: "${node_type}"
  - db.plan.level: "${level_id}"
values:
  - total_cost (gauge)
  - startup_cost (gauge)

# Time metrics
metric_name: db.plan.time
unit: ms
attributes: [same as above]
values:
  - actual_startup_time (gauge)
  - actual_total_time (gauge)

# Row metrics
metric_name: db.plan.rows
attributes: [same as above]
values:
  - plan_rows (gauge)
  - actual_rows (gauge)
  - actual_loops (counter)

# Block I/O metrics
metric_name: db.plan.blocks
attributes: [same as above]
values:
  - shared_hit_block (counter)
  - shared_read_blocks (counter)
  - shared_dirtied_blocks (counter)
  - shared_written_blocks (counter)
  - local_hit_block (counter)
  - local_read_blocks (counter)
  - local_dirtied_blocks (counter)
  - local_written_blocks (counter)
  - temp_read_block (counter)
  - temp_written_blocks (counter)
```

## OpenTelemetry Collector Configuration

To collect these metrics, use the following collector configuration:

```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: ${POSTGRES_PASSWORD}
    databases:
      - postgres
    collection_interval: 10s
    
  sqlquery/slow_queries:
    driver: postgres
    datasource: ${POSTGRES_DSN}
    collection_interval: 15s
    queries:
      - sql: |
          SELECT 
            queryid::text as query_id,
            query as query_text,
            datname as database_name,
            calls as execution_count,
            mean_exec_time as avg_elapsed_time_ms,
            total_exec_time as total_time_ms,
            rows as total_rows,
            shared_blks_read as avg_disk_reads,
            shared_blks_written as avg_disk_writes,
            'SELECT' as statement_type,
            mean_exec_time/1000.0 as avg_cpu_time_ms
          FROM pg_stat_statements
          JOIN pg_database ON pg_database.oid = dbid
          WHERE mean_exec_time > 100
        metrics:
          - metric_name: db.statement.execution.count
            value_column: execution_count
            value_type: int
            attributes: [query_id, query_text, database_name, statement_type]
          - metric_name: db.statement.execution.time
            value_column: avg_elapsed_time_ms
            value_type: double
            unit: ms
            attributes: [query_id, query_text, database_name, statement_type]
          - metric_name: db.statement.disk.reads
            value_column: avg_disk_reads
            value_type: double
            attributes: [query_id, query_text, database_name]
          - metric_name: db.statement.disk.writes
            value_column: avg_disk_writes
            value_type: double
            attributes: [query_id, query_text, database_name]
          - metric_name: db.statement.cpu.time
            value_column: avg_cpu_time_ms
            value_type: double
            unit: ms
            attributes: [query_id, query_text, database_name]

  sqlquery/wait_events:
    driver: postgres
    datasource: ${POSTGRES_DSN}
    collection_interval: 10s
    queries:
      - sql: |
          SELECT 
            wait_event,
            wait_event_type,
            count(*) as count,
            datname,
            query
          FROM pg_stat_activity
          WHERE wait_event IS NOT NULL
          GROUP BY wait_event, wait_event_type, datname, query
        metrics:
          - metric_name: db.wait_events.count
            value_column: count
            value_type: int
            attributes: [wait_event, wait_event_type, datname, query]

  sqlquery/blocking_sessions:
    driver: postgres
    datasource: ${POSTGRES_DSN}
    collection_interval: 10s
    queries:
      - sql: |
          SELECT
            blocked.pid as blocked_pid,
            blocked.query as blocked_query,
            blocked.query_start as blocked_query_start,
            blocked.datname as database_name,
            blocking.pid as blocking_pid,
            blocking.query as blocking_query,
            blocking.query_start as blocking_query_start,
            blocking.datname as blocking_database
          FROM pg_stat_activity blocked
          JOIN pg_stat_activity blocking 
            ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
        metrics:
          - metric_name: db.blocking_sessions.active
            value_column: blocked_pid
            value_type: int
            attributes: [blocked_pid, blocked_query, database_name, blocking_pid, blocking_query, blocking_database]

processors:
  attributes:
    actions:
      - key: db.system
        value: postgresql
        action: insert
      
  transform:
    metric_statements:
      # Map wait event attributes to standard format
      - context: datapoint
        statements:
          - set(attributes["db.wait_event.name"], attributes["wait_event"]) where attributes["wait_event"] != nil
          - set(attributes["db.wait_event.category"], attributes["wait_event_type"]) where attributes["wait_event_type"] != nil
          - set(attributes["db.name"], attributes["datname"]) where attributes["datname"] != nil
          - delete_key(attributes, "wait_event")
          - delete_key(attributes, "wait_event_type")
          - delete_key(attributes, "datname")

exporters:
  otlp:
    endpoint: https://otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql, sqlquery/slow_queries, sqlquery/wait_events, sqlquery/blocking_sessions]
      processors: [attributes, transform]
      exporters: [otlp]
```

## Dashboard Query Mappings

### Original OHI Queries to OTEL Queries

1. **Database Query Count**
   - OHI: `SELECT uniqueCount(query_id) from PostgresSlowQueries facet database_name`
   - OTEL: `SELECT uniqueCount(db.postgresql.query_id) FROM Metric WHERE metricName = 'db.statement.execution.count' FACET db.name`

2. **Average Execution Time**
   - OHI: `SELECT latest(avg_elapsed_time_ms) from PostgresSlowQueries where query_text!='<insufficient privilege>' facet query_text`
   - OTEL: `SELECT latest(db.statement.execution.time) FROM Metric WHERE db.statement != '<insufficient privilege>' FACET db.statement`

3. **Execution Counts Over Time**
   - OHI: `SELECT count(execution_count) from PostgresSlowQueries TIMESERIES`
   - OTEL: `SELECT sum(db.statement.execution.count) FROM Metric TIMESERIES`

4. **Top Wait Events**
   - OHI: `SELECT latest(total_wait_time_ms) from PostgresWaitEvents facet wait_event_name where wait_event_name!='<nil>'`
   - OTEL: `SELECT sum(db.wait_events.count) FROM Metric FACET db.wait_event.name WHERE db.wait_event.name IS NOT NULL`

5. **Disk I/O Metrics**
   - OHI: `SELECT latest(avg_disk_reads) as 'Average Disk Reads' From PostgresSlowQueries facet database_name TIMESERIES`
   - OTEL: `SELECT average(db.statement.disk.reads) FROM Metric FACET db.name TIMESERIES`

## Implementation Status

| OHI Event | OTEL Implementation | Status | Notes |
|-----------|-------------------|---------|--------|
| PostgresSlowQueries | db.statement.* metrics | ⚠️ Partial | Need to fix pg_stat_statements query |
| PostgresWaitEvents | db.wait_events.* metrics | ✅ Working | Successfully collecting |
| PostgresBlockingSessions | db.blocking_sessions.* metrics | ❌ Not Working | Need blocking scenario |
| PostgresIndividualQueries | db.query.cpu_time | ❌ Not Implemented | Requires pg_stat_statements |
| PostgresExecutionPlanMetrics | db.plan.* metrics | ❌ Not Implemented | Requires auto_explain |

## Next Steps

1. Fix pg_stat_statements query to properly join with pg_database
2. Implement blocking session detection
3. Add auto_explain configuration for execution plan metrics
4. Create transform rules to map all attributes correctly
5. Test each metric with actual queries and scenarios
