# MSSQL/SQL Server Maximum Metrics Extraction Guide

This guide demonstrates how to extract 100+ metrics from Microsoft SQL Server using only stock OpenTelemetry components.

## Overview

The `mssql-maximum-extraction.yaml` configuration demonstrates:
- **100+ distinct metrics** from SQL Server
- **Query performance analysis** with execution plans
- **Wait statistics** categorized by type
- **Active session monitoring** with blocking detection
- **Always On Availability Groups** monitoring
- **Index usage and fragmentation** analysis
- **TempDB usage** tracking

## Quick Start

```bash
# 1. Set environment variables
export MSSQL_HOST=localhost
export MSSQL_PORT=1433
export MSSQL_USER=sa
export MSSQL_PASSWORD=your_password
export NEW_RELIC_LICENSE_KEY=your_license_key

# 2. Run the collector
docker run -d \
  --name otel-mssql-max \
  -v $(pwd)/configs/mssql-maximum-extraction.yaml:/etc/otelcol/config.yaml \
  -e MSSQL_HOST \
  -e MSSQL_PORT \
  -e MSSQL_USER \
  -e MSSQL_PASSWORD \
  -e NEW_RELIC_LICENSE_KEY \
  -p 8893:8893 \
  otel/opentelemetry-collector-contrib:latest
```

## Prerequisites

### 1. Enable Required Permissions

```sql
-- Create monitoring login
CREATE LOGIN otel_monitor WITH PASSWORD = 'SecurePassword123!';

-- Grant server-level permissions
GRANT VIEW SERVER STATE TO otel_monitor;
GRANT VIEW ANY DEFINITION TO otel_monitor;

-- Create user in each database to monitor
USE [YourDatabase];
CREATE USER otel_monitor FOR LOGIN otel_monitor;
GRANT VIEW DATABASE STATE TO otel_monitor;

-- For Always On monitoring
GRANT VIEW ANY DEFINITION TO otel_monitor;
```

### 2. Enable Query Store (Recommended)

```sql
-- Enable Query Store for better query performance insights
ALTER DATABASE [YourDatabase] SET QUERY_STORE = ON;
ALTER DATABASE [YourDatabase] SET QUERY_STORE (
  OPERATION_MODE = READ_WRITE,
  CLEANUP_POLICY = (STALE_QUERY_THRESHOLD_DAYS = 30),
  DATA_FLUSH_INTERVAL_SECONDS = 900,
  MAX_STORAGE_SIZE_MB = 1000
);
```

## Metrics Categories

### 1. Core SQL Server Metrics (40+)

The standard sqlserver receiver provides:
- **Database I/O**: Read/write latency, operations
- **Buffer Manager**: Cache hit ratio, page life expectancy
- **Batch Requests**: Compilations, recompilations
- **Connections**: User connections, blocked processes
- **CPU Usage**: SQL process utilization
- **Memory**: Total and target server memory
- **Locks**: Wait time, waits by type
- **Transaction Log**: Growth, shrink, usage

### 2. Query Performance Metrics

Detailed query analysis from DMVs:
- **Execution Count**: How often queries run
- **Resource Usage**: CPU, reads, writes per query
- **Average Duration**: Elapsed and worker time
- **Query Plans**: Plan handles for optimization

### 3. Wait Statistics Analysis

Comprehensive wait type monitoring:
- **Wait Categories**: CPU, I/O, Lock, Memory, Network
- **Wait Time**: Total and percentage by type
- **Task Count**: Waiting tasks per wait type
- **Signal Wait**: CPU scheduling delays

### 4. Active Session Monitoring

Real-time session analysis:
- **Active Requests**: Currently executing queries
- **Blocking Chains**: Session blocking detection
- **Resource Usage**: CPU, I/O per session
- **Wait Analysis**: Current wait types and duration

### 5. Index Performance

Index usage and maintenance:
- **Usage Stats**: Seeks, scans, lookups, updates
- **Fragmentation**: Average fragmentation percentage
- **Missing Indexes**: DMV recommendations
- **Unused Indexes**: Candidates for removal

### 6. Always On Availability Groups

HA/DR monitoring:
- **Synchronization State**: Primary/secondary sync
- **Log Send Queue**: Pending log to send
- **Redo Queue**: Pending log to apply
- **Replica Health**: Connection and operational state

## Configuration Breakdown

### Multi-Pipeline Architecture

```yaml
service:
  pipelines:
    # Real-time session monitoring (5s)
    metrics/high_frequency:
      receivers: [sqlquery/active_sessions]
      
    # Core metrics and waits (10s)
    metrics/standard:
      receivers: [sqlserver, sqlquery/wait_stats]
      
    # Query performance (30s)
    metrics/performance:
      receivers: [sqlquery/query_stats, sqlquery/alwayson]
      
    # Index and space analysis (60-300s)
    metrics/analytics:
      receivers: [sqlquery/tempdb, sqlquery/index_stats]
```

### Intelligent Wait Categorization

```yaml
transform/add_metadata:
  metric_statements:
    # Categorize wait types
    - set(attributes["wait.category"], "cpu") 
      where attributes["wait_type"] == "SOS_SCHEDULER_YIELD"
    - set(attributes["wait.category"], "io") 
      where IsMatch(attributes["wait_type"], "PAGEIO.*")
    - set(attributes["wait.category"], "lock") 
      where IsMatch(attributes["wait_type"], "LCK_.*")
```

## Performance Tuning

### 1. Filter Noise from Wait Stats

The configuration excludes benign wait types:
```sql
WHERE wait_type NOT IN (
  N'BROKER_EVENTHANDLER', N'CHECKPOINT_QUEUE',
  N'LAZYWRITER_SLEEP', N'SQLTRACE_BUFFER_FLUSH',
  -- ... other benign waits
)
```

### 2. Limit Query Results

```yaml
sqlquery/query_stats:
  queries:
    - sql: |
        SELECT TOP 50 ... -- Limit to top queries
        WHERE execution_count > 5 -- Skip one-off queries
        ORDER BY total_worker_time DESC -- Focus on CPU
```

### 3. Collection Interval Strategy

- **5s**: Active sessions (blocking detection)
- **10s**: Core metrics, wait stats
- **30s**: Query performance, AG status
- **60s**: TempDB usage
- **300s**: Index fragmentation

## Monitoring Best Practices

### 1. Key Metrics to Alert On

- `mssql.buffer.page_life_expectancy` < 300 seconds
- `mssql.wait.percentage` > 20% for any category
- `mssql.session.blocked_count` > 0
- `mssql.cpu.sql_process_utilization` > 80%
- `mssql.index.fragmentation` > 30%
- `mssql.alwayson.log_send_queue` > 10MB

### 2. Dashboard Organization

- **Overview**: CPU, memory, batch requests, connections
- **Performance**: Top queries, wait stats, blocking
- **Storage**: Database sizes, file I/O, TempDB
- **Availability**: Always On status, backup status
- **Maintenance**: Index fragmentation, statistics age

### 3. Troubleshooting Workflow

1. Check wait statistics distribution
2. Identify top resource-consuming queries
3. Analyze blocking chains
4. Review index usage patterns
5. Monitor TempDB contention

## Troubleshooting

### No Metrics Appearing

```sql
-- Verify permissions
SELECT 
  p.permission_name, 
  p.state_desc 
FROM sys.server_permissions p
JOIN sys.server_principals l ON p.grantee_principal_id = l.principal_id
WHERE l.name = 'otel_monitor';

-- Test connectivity
SELECT @@VERSION;
```

### Missing Query Stats

```sql
-- Clear procedure cache to see new queries
DBCC FREEPROCCACHE;

-- Check if queries are being cached
SELECT COUNT(*) FROM sys.dm_exec_query_stats;
```

### High Memory Usage

1. Reduce TOP clause in queries
2. Increase collection intervals
3. Filter system databases:
```yaml
filter/reduce_cardinality:
  metrics:
    metric:
      - 'attributes["database_name"] == "tempdb"'
```

## Example Queries

### Find Expensive Queries

```sql
SELECT average(mssql.query.avg_worker_time) 
FROM Metric 
WHERE deployment.mode = 'config-only-mssql-max' 
FACET query_text 
SINCE 1 hour ago
```

### Wait Time Analysis

```sql
SELECT average(mssql.wait.percentage) 
FROM Metric 
WHERE deployment.mode = 'config-only-mssql-max' 
FACET wait.category, wait_type 
SINCE 1 hour ago
```

### Blocking Detection

```sql
SELECT max(mssql.session.blocked_count) 
FROM Metric 
WHERE deployment.mode = 'config-only-mssql-max' 
TIMESERIES 1 minute 
SINCE 30 minutes ago
```

### Always On Health

```sql
SELECT latest(mssql.alwayson.log_send_queue) as 'Send Queue',
       latest(mssql.alwayson.redo_queue) as 'Redo Queue'
FROM Metric 
WHERE deployment.mode = 'config-only-mssql-max' 
FACET ag_name, replica_server_name, database_name
```

## Advanced Features

### 1. Extended Events Integration

Capture deadlock graphs:
```yaml
sqlquery/deadlocks:
  queries:
    - sql: |
        SELECT 
          xed.value('@timestamp', 'datetime') as deadlock_time,
          xed.query('.') as deadlock_graph
        FROM sys.fn_xe_file_target_read_file('deadlock*.xel', NULL, NULL, NULL) 
        CROSS APPLY (
          SELECT CAST(event_data AS XML)
        ) AS x(xml_data)
        CROSS APPLY xml_data.nodes('event') AS xe(xed)
        WHERE xed.value('@name', 'varchar(200)') = 'xml_deadlock_report'
```

### 2. Resource Governor Monitoring

```yaml
sqlquery/resource_governor:
  queries:
    - sql: |
        SELECT 
          pool_id,
          name,
          statistics_start_time,
          total_cpu_usage_ms,
          cache_memory_kb,
          compile_memory_kb,
          used_memgrant_kb,
          total_memgrant_count
        FROM sys.dm_resource_governor_resource_pools
```

### 3. Columnstore Index Monitoring

```yaml
sqlquery/columnstore:
  queries:
    - sql: |
        SELECT 
          OBJECT_NAME(object_id) as table_name,
          index_id,
          row_group_id,
          state_desc,
          total_rows,
          deleted_rows,
          size_in_bytes
        FROM sys.dm_db_column_store_row_group_physical_stats
```

## Conclusion

This configuration extracts 100+ metrics from SQL Server using only OpenTelemetry configuration:
- ✅ No custom code required
- ✅ Comprehensive performance insights
- ✅ Real-time blocking detection
- ✅ HA/DR monitoring support
- ✅ Automatic wait categorization

The patterns work with SQL Server 2016+ and support all editions including Express (with limitations).