## OHI to OpenTelemetry Query Mapping for Dashboard

# OHI to OpenTelemetry Query Mapping for Dashboard

This document maps all OHI NRQL queries to their OpenTelemetry equivalents.

## Bird's-Eye View Page

### 1. Database Widget
**OHI Query:**
```sql
SELECT uniqueCount(query_id) from PostgresSlowQueries facet database_name
```
**OTEL Query:**
```sql
SELECT uniqueCount(attributes.db.postgresql.query_id) FROM Metric 
WHERE metricName LIKE 'postgres.slow_queries%' 
FACET attributes.db.name
```

### 2. Average Execution Time
**OHI Query:**
```sql
SELECT latest(avg_elapsed_time_ms) from PostgresSlowQueries 
where query_text!='<insufficient privilege>' facet query_text
```
**OTEL Query:**
```sql
SELECT latest(postgres.slow_queries.elapsed_time) FROM Metric 
WHERE attributes.db.statement != '<insufficient privilege>' 
FACET attributes.db.statement
```

### 3. Execution Counts Over Time
**OHI Query:**
```sql
SELECT count(execution_count) from PostgresSlowQueries TIMESERIES
```
**OTEL Query:**
```sql
SELECT sum(postgres.slow_queries.count) FROM Metric TIMESERIES
```

### 4. Top Wait Events
**OHI Query:**
```sql
SELECT latest(total_wait_time_ms) from PostgresWaitEvents 
facet wait_event_name where wait_event_name!='<nil>'
```
**OTEL Query:**
```sql
SELECT sum(postgres.wait_events) FROM Metric 
FACET attributes.db.wait_event.name 
WHERE attributes.db.wait_event.name IS NOT NULL
```

### 5. Top N Slowest Queries (Table)
**OHI Query:**
```sql
SELECT latest(database_name), latest(query_text), latest(schema_name), 
       latest(execution_count), latest(avg_elapsed_time_ms), 
       latest(avg_disk_reads), latest(avg_disk_writes), latest(statement_type) 
FROM PostgresSlowQueries facet query_id limit max
```
**OTEL Query:**
```sql
SELECT latest(attributes.db.name) as 'Database', 
       latest(attributes.db.statement) as 'Query', 
       latest(attributes.db.schema) as 'Schema', 
       latest(postgres.slow_queries.count) as 'Execution Count', 
       latest(postgres.slow_queries.elapsed_time) as 'Avg Elapsed Time (ms)', 
       latest(postgres.slow_queries.disk_reads) as 'Avg Disk Reads', 
       latest(postgres.slow_queries.disk_writes) as 'Avg Disk Writes', 
       latest(attributes.db.operation) as 'Statement Type' 
FROM Metric WHERE metricName LIKE 'postgres.slow_queries%' 
FACET attributes.db.postgresql.query_id LIMIT MAX
```

### 6. Disk IO Usage - Reads
**OHI Query:**
```sql
SELECT latest(avg_disk_reads) as 'Average Disk Reads' 
From PostgresSlowQueries facet database_name TIMESERIES
```
**OTEL Query:**
```sql
SELECT average(postgres.slow_queries.disk_reads) as 'Average Disk Reads' 
FROM Metric FACET attributes.db.name TIMESERIES
```

### 7. Disk IO Usage - Writes
**OHI Query:**
```sql
SELECT average(avg_disk_writes) as 'Average Disk Writes' 
From PostgresSlowQueries facet database_name TIMESERIES
```
**OTEL Query:**
```sql
SELECT average(postgres.slow_queries.disk_writes) as 'Average Disk Writes' 
FROM Metric FACET attributes.db.name TIMESERIES
```

### 8. Blocking Details
**OHI Query:**
```sql
SELECT latest(blocked_pid), latest(blocked_query), latest(blocked_query_id),
       latest(blocked_query_start) as 'Blocked Query Timeseries',
       latest(database_name), latest(blocking_pid), latest(blocking_query),
       latest(blocking_query_id), latest(blocking_query_start) as 'Blocking Query Timeseries',
       latest(blocking_database) as 'Database' 
from PostgresBlockingSessions facet blocked_pid
```
**OTEL Query:**
```sql
SELECT latest(attributes.db.blocking.blocked_pid) as 'Blocked PID', 
       latest(attributes.db.blocking.blocked_query) as 'Blocked Query', 
       latest(attributes.db.blocking.blocked_query_id) as 'Blocked Query ID', 
       latest(attributes.blocked_query_start) as 'Blocked Query Start', 
       latest(attributes.db.name) as 'Database', 
       latest(attributes.db.blocking.blocking_pid) as 'Blocking PID', 
       latest(attributes.db.blocking.blocking_query) as 'Blocking Query', 
       latest(attributes.db.blocking.blocking_query_id) as 'Blocking Query ID', 
       latest(attributes.blocking_query_start) as 'Blocking Query Start', 
       latest(attributes.blocking_database) as 'Blocking Database' 
FROM Metric WHERE metricName = 'postgres.blocking_sessions' 
FACET attributes.db.blocking.blocked_pid
```

## Query Details Page

### 1. Individual Query Details
**OHI Query:**
```sql
SELECT latest(query_text), latest(avg_cpu_time_ms or 'NA'), latest(query_id) 
from PostgresIndividualQueries facet plan_id limit max
```
**OTEL Query:**
```sql
SELECT latest(attributes.db.statement) as 'Query Text', 
       latest(postgres.individual_queries.cpu_time) as 'Avg CPU Time (ms)', 
       latest(attributes.db.postgresql.query_id) as 'Query ID' 
FROM Metric WHERE metricName = 'postgres.individual_queries.cpu_time' 
FACET attributes.db.postgresql.plan_id LIMIT MAX
```

### 2. Query Execution Plan Details
**OHI Query:**
```sql
SELECT latest(node_type), latest(query_id), latest(query_text),
       latest(total_cost), latest(startup_cost), latest(plan_rows),
       latest(actual_startup_time), latest(actual_total_time), 
       latest(actual_rows), latest(actual_loops), latest(shared_hit_block),
       latest(shared_read_blocks), latest(shared_dirtied_blocks),
       latest(shared_written_blocks), latest(local_hit_block),
       latest(local_read_blocks), latest(local_dirtied_blocks),
       latest(local_written_blocks), latest(temp_read_block),
       latest(temp_written_blocks), latest(database_name) 
from PostgresExecutionPlanMetrics facet plan_id, level_id
```
**OTEL Query:**
```sql
SELECT latest(attributes.db.plan.node_type) as 'Node Type', 
       latest(attributes.db.postgresql.query_id) as 'Query ID', 
       latest(attributes.query_text) as 'Query Text', 
       latest(postgres.execution_plan.cost) as 'Total Cost', 
       latest(attributes.startup_cost) as 'Startup Cost', 
       latest(postgres.execution_plan.rows) as 'Plan Rows', 
       latest(attributes.actual_startup_time) as 'Actual Startup Time', 
       latest(postgres.execution_plan.time) as 'Actual Total Time', 
       latest(attributes.actual_rows) as 'Actual Rows', 
       latest(attributes.actual_loops) as 'Actual Loops', 
       latest(postgres.execution_plan.blocks_hit) as 'Shared Hit Blocks', 
       latest(postgres.execution_plan.blocks_read) as 'Shared Read Blocks', 
       latest(attributes.shared_dirtied_blocks) as 'Shared Dirtied', 
       latest(attributes.shared_written_blocks) as 'Shared Written', 
       latest(attributes.local_hit_block) as 'Local Hit', 
       latest(attributes.local_read_blocks) as 'Local Read', 
       latest(attributes.local_dirtied_blocks) as 'Local Dirtied', 
       latest(attributes.local_written_blocks) as 'Local Written', 
       latest(attributes.temp_read_block) as 'Temp Read', 
       latest(attributes.temp_written_blocks) as 'Temp Written', 
       latest(attributes.db.name) as 'Database' 
FROM Metric WHERE metricName LIKE 'postgres.execution_plan%' 
FACET attributes.db.postgresql.plan_id, attributes.db.plan.level 
ORDER BY attributes.db.plan.level ASC
```

## Wait Time Analysis Page

### 1. Top Wait Events Over Time
**OHI Query:**
```sql
SELECT latest(total_wait_time_ms) from PostgresWaitEvents 
facet wait_event_name, wait_category where wait_event_name != '<nil>' timeseries
```
**OTEL Query:**
```sql
SELECT sum(postgres.wait_events) FROM Metric 
FACET attributes.db.wait_event.name, attributes.db.wait_event.category 
WHERE attributes.db.wait_event.name IS NOT NULL TIMESERIES
```

### 2. Additional Core PostgreSQL Metrics
**OTEL Query (New):**
```sql
SELECT latest(postgresql.backends) as 'Active Connections', 
       latest(postgresql.commits) as 'Commits/sec', 
       latest(postgresql.rollbacks) as 'Rollbacks/sec', 
       latest(postgresql.db_size) as 'Database Size (bytes)', 
       latest(postgresql.deadlocks) as 'Deadlocks', 
       latest(postgresql.temp_files) as 'Temp Files' 
FROM Metric WHERE attributes.db.system = 'postgresql' 
FACET attributes.postgresql.database.name
```

## Key Mapping Principles

1. **Event to Metric Mapping**:
   - `PostgresSlowQueries` → `postgres.slow_queries.*` metrics
   - `PostgresWaitEvents` → `postgres.wait_events` metric
   - `PostgresBlockingSessions` → `postgres.blocking_sessions` metric
   - `PostgresIndividualQueries` → `postgres.individual_queries.*` metrics
   - `PostgresExecutionPlanMetrics` → `postgres.execution_plan.*` metrics

2. **Attribute Mapping**:
   - `database_name` → `attributes.db.name`
   - `query_text` → `attributes.db.statement`
   - `query_id` → `attributes.db.postgresql.query_id`
   - `statement_type` → `attributes.db.operation`
   - `schema_name` → `attributes.db.schema`
   - `wait_event_name` → `attributes.db.wait_event.name`
   - `wait_category` → `attributes.db.wait_event.category`

3. **Metric Selection**:
   - Use `FROM Metric` instead of event names
   - Filter by `metricName` when needed
   - Access attributes via `attributes.` prefix

4. **Aggregations**:
   - `count()` on events → `sum()` on counter metrics
   - `latest()` works the same for gauge metrics
   - `average()` for averaging gauge values
