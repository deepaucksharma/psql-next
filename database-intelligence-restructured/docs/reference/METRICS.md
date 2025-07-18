# Database Metrics Reference

Complete reference for all metrics collected by Database Intelligence across PostgreSQL, MySQL, MongoDB, and Redis.

## Table of Contents
- [PostgreSQL Metrics](#postgresql-metrics)
- [MySQL Metrics](#mysql-metrics)
- [MongoDB Metrics](#mongodb-metrics)
- [Redis Metrics](#redis-metrics)
- [NRQL Query Examples](#nrql-query-examples)

## PostgreSQL Metrics

### Config-Only Mode Metrics

### Connection Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.backends` | Number of backends currently connected | Gauge | connections |
| `postgresql.connection.max` | Maximum number of concurrent connections | Gauge | connections |
| `postgresql.stat_activity.count` | Connections by state | Gauge | connections |
| `pg.connection_count` | Custom metric for connection states | Gauge | connections |

### Transaction Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.commits` | Number of transactions committed | Counter | transactions |
| `postgresql.rollbacks` | Number of transactions rolled back | Counter | transactions |
| `postgresql.deadlocks` | Number of deadlocks detected | Counter | deadlocks |

### Block I/O Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.blocks_read` | Number of disk blocks read | Counter | blocks |
| `postgresql.blks_hit` | Number of times disk blocks were found in buffer cache | Counter | hits |
| `postgresql.blks_read` | Number of disk blocks read | Counter | reads |
| `postgresql.buffer.hit` | Buffer cache hit ratio | Gauge | ratio |

### Database Size Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.database.count` | Number of databases | Gauge | databases |
| `postgresql.database.size` | Size of each database | Gauge | bytes |
| `postgresql.table.count` | Number of tables per database | Gauge | tables |
| `postgresql.table.size` | Size of each table | Gauge | bytes |
| `postgresql.index.size` | Size of indexes | Gauge | bytes |

### Row Operation Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.rows` | Row operations (tup_returned, tup_fetched, etc.) | Counter | operations |
| `postgresql.database.rows` | Database-level row operations | Counter | operations |
| `postgresql.operations` | Various database operations | Counter | operations |
| `postgresql.live_rows` | Estimated number of live rows | Gauge | rows |

### Index Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.index.scans` | Number of index scans initiated | Counter | scans |
| `postgresql.sequential_scans` | Number of sequential scans initiated | Counter | scans |

### Background Writer Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.bgwriter.buffers.allocated` | Number of buffers allocated | Counter | buffers |
| `postgresql.bgwriter.buffers.writes` | Number of buffers written | Counter | writes |
| `postgresql.bgwriter.checkpoint.count` | Number of checkpoints performed | Counter | checkpoints |
| `postgresql.bgwriter.duration` | Time spent writing/syncing | Gauge | milliseconds |
| `postgresql.bgwriter.maxwritten` | Number of times max written stopped cleaning | Counter | times |
| `postgresql.bgwriter.stat.checkpoints_timed` | Scheduled checkpoints | Counter | checkpoints |
| `postgresql.bgwriter.stat.checkpoints_req` | Requested checkpoints | Counter | checkpoints |

### WAL & Replication Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.wal.age` | Age of oldest WAL file | Gauge | seconds |
| `postgresql.wal.delay` | WAL write delay | Gauge | seconds |
| `postgresql.wal.lag` | Replication lag in bytes | Gauge | bytes |
| `postgresql.replication.data_delay` | Replication delay in seconds | Gauge | seconds |

### Lock Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.database.locks` | Number of locks held | Gauge | locks |
| `postgresql.locks` | Lock statistics | Gauge | locks |
| `postgresql.conflicts` | Number of queries canceled due to conflicts | Counter | conflicts |

### Vacuum Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.table.vacuum.count` | Number of vacuum operations | Counter | operations |

### Temporary File Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgresql.temp_files` | Number of temporary files created | Counter | files |

### Host Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `system.cpu.utilization` | CPU utilization | Gauge | ratio |
| `system.memory.utilization` | Memory utilization | Gauge | ratio |
| `system.disk.io` | Disk I/O | Counter | bytes |

## MySQL Metrics

### Core MySQL Receiver Metrics (40+)

#### Buffer Pool Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mysql.buffer_pool.data_pages` | Number of data pages in buffer pool | Gauge | pages |
| `mysql.buffer_pool.limit` | Buffer pool size limit | Gauge | bytes |
| `mysql.buffer_pool.operations` | Buffer pool operations | Counter | operations |
| `mysql.buffer_pool.page_flushes` | Page flush operations | Counter | flushes |
| `mysql.buffer_pool.pages` | Buffer pool page statistics | Gauge | pages |
| `mysql.buffer_pool.usage` | Buffer pool usage | Gauge | bytes |

#### Connection Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mysql.connection.count` | Current connections | Gauge | connections |
| `mysql.connection.errors` | Connection errors | Counter | errors |
| `mysql.threads` | Thread statistics | Gauge | threads |
| `mysql.connection_pool.active_percentage` | Active connection percentage | Gauge | % |

#### Command Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mysql.commands` | Command execution counts | Counter | commands |
| `mysql.prepared_stmt_count` | Prepared statements | Gauge | statements |
| `mysql.queries` | Query rate | Counter | queries |
| `mysql.slow_queries` | Slow query count | Counter | queries |

#### InnoDB Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mysql.innodb_row_lock_time` | Row lock wait time | Counter | milliseconds |
| `mysql.innodb_row_lock_waits` | Row lock waits | Counter | waits |
| `mysql.innodb_data_reads` | Data reads | Counter | reads |
| `mysql.innodb_data_writes` | Data writes | Counter | writes |

#### Table I/O Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mysql.table.io.wait.count` | Table I/O wait events | Counter | waits |
| `mysql.table.io.wait.time` | Table I/O wait time | Counter | picoseconds |
| `mysql.index.io.wait.count` | Index I/O wait events | Counter | waits |
| `mysql.index.io.wait.time` | Index I/O wait time | Counter | picoseconds |

#### Replication Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mysql.replica.lag` | Replication lag | Gauge | seconds |
| `mysql.replica.sql_delay` | SQL thread delay | Gauge | seconds |
| `mysql.replication.io_running` | IO thread status | Gauge | status |
| `mysql.replication.sql_running` | SQL thread status | Gauge | status |

### Performance Schema Metrics

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mysql.query.executions` | Query execution count | Gauge | executions |
| `mysql.query.avg_latency` | Average query latency | Gauge | ms |
| `mysql.query.max_latency` | Maximum query latency | Gauge | ms |
| `mysql.query.rows_examined_avg` | Average rows examined | Gauge | rows |

## MongoDB Metrics

### Core MongoDB Receiver Metrics (50+)

#### Cache Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mongodb.cache.operations` | Cache operations | Counter | operations |
| `mongodb.cache.hit_ratio` | Cache hit ratio | Gauge | ratio |

#### Collection & Database Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mongodb.collection.count` | Number of collections | Gauge | collections |
| `mongodb.database.count` | Number of databases | Gauge | databases |
| `mongodb.data.size` | Data size | Gauge | bytes |
| `mongodb.storage.size` | Storage size | Gauge | bytes |

#### Connection Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mongodb.connection.count` | Current connections | Gauge | connections |
| `mongodb.network.bytes_in` | Network bytes received | Counter | bytes |
| `mongodb.network.bytes_out` | Network bytes sent | Counter | bytes |
| `mongodb.network.request.count` | Network requests | Counter | requests |

#### Operation Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mongodb.operation.count` | Operation counts | Counter | operations |
| `mongodb.operation.latency.time` | Operation latency | Gauge | microseconds |
| `mongodb.document.operation.count` | Document operations | Counter | operations |
| `mongodb.ash.sessions` | Active sessions (ASH-like) | Gauge | sessions |

#### Lock Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mongodb.global_lock.time` | Global lock time | Counter | microseconds |
| `mongodb.lock.acquire.count` | Lock acquisitions | Counter | acquisitions |
| `mongodb.lock.acquire.wait_count` | Lock wait count | Counter | waits |
| `mongodb.lock.acquire.wait_time` | Lock wait time | Counter | microseconds |

#### Memory Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mongodb.memory.usage` | Memory usage | Gauge | bytes |
| `mongodb.memory.resident` | Resident memory | Gauge | bytes |
| `mongodb.memory.virtual` | Virtual memory | Gauge | bytes |

### MongoDB Atlas Metrics (40+)

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mongodbatlas.process.cpu.usage` | CPU usage | Gauge | percent |
| `mongodbatlas.disk.partition.iops.average` | Disk IOPS | Gauge | operations/s |
| `mongodbatlas.disk.partition.latency.average` | Disk latency | Gauge | milliseconds |
| `mongodbatlas.process.cache.io` | Cache I/O | Counter | bytes |
| `mongodbatlas.process.db.query_executor.scanned` | Documents scanned | Counter | documents |

## MSSQL/SQL Server Metrics

### Core SQL Server Receiver Metrics (40+)

#### Database I/O Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `sqlserver.database.io.read_latency` | Read latency | Gauge | ms |
| `sqlserver.database.io.write_latency` | Write latency | Gauge | ms |
| `sqlserver.database.operations` | Database operations | Counter | operations |
| `sqlserver.database.size` | Database size | Gauge | bytes |
| `sqlserver.database.transactions` | Transaction count | Counter | transactions |

#### Buffer Manager Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `sqlserver.buffer.cache_hit_ratio` | Buffer cache hit ratio | Gauge | ratio |
| `sqlserver.buffer.checkpoint_pages` | Checkpoint pages/sec | Counter | pages/s |
| `sqlserver.buffer.page_life_expectancy` | Page life expectancy | Gauge | seconds |
| `sqlserver.buffer.page_operations` | Page operations | Counter | operations |

#### Batch and Compilation Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `sqlserver.batch.requests` | Batch requests/sec | Counter | batches/s |
| `sqlserver.batch.sql_compilations` | SQL compilations/sec | Counter | compilations/s |
| `sqlserver.batch.sql_recompilations` | SQL recompilations/sec | Counter | recompilations/s |

#### Connection and Lock Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `sqlserver.connection.count` | Connection count | Gauge | connections |
| `sqlserver.user.connection.count` | User connections | Gauge | connections |
| `sqlserver.process.blocked` | Blocked processes | Gauge | processes |
| `sqlserver.lock.wait_time` | Lock wait time | Counter | ms |
| `sqlserver.lock.waits` | Lock waits/sec | Counter | waits/s |

### Custom SQL Query Metrics

#### Query Performance Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mssql.query.execution_count` | Query execution count | Gauge | executions |
| `mssql.query.avg_logical_reads` | Average logical reads | Gauge | pages |
| `mssql.query.avg_worker_time` | Average CPU time | Gauge | μs |
| `mssql.query.avg_elapsed_time` | Average elapsed time | Gauge | μs |

#### Wait Statistics Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mssql.wait.time_ms` | Wait time by type | Gauge | ms |
| `mssql.wait.tasks_count` | Waiting tasks count | Gauge | tasks |
| `mssql.wait.percentage` | Wait time percentage | Gauge | % |

#### Session Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `mssql.session.active_count` | Active sessions | Gauge | sessions |
| `mssql.session.wait_time` | Session wait time | Gauge | ms |
| `mssql.session.cpu_time` | Session CPU time | Gauge | ms |
| `mssql.session.blocked_count` | Blocked sessions | Gauge | sessions |

## Oracle Database Metrics

### Core Database Metrics

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `oracle.database.uptime` | Database uptime | Gauge | seconds |
| `oracle.sga.size` | SGA component sizes | Gauge | MB |
| `oracle.buffer_cache.hit_ratio` | Buffer cache hit ratio | Gauge | % |

### Session and Connection Metrics

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `oracle.sessions.count` | Session count by state | Gauge | sessions |
| `oracle.sessions.blocked` | Blocked sessions | Gauge | sessions |
| `oracle.sessions.max_wait_time` | Maximum wait time | Gauge | seconds |

### SQL Performance Metrics

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `oracle.sql.executions` | SQL execution count | Gauge | executions |
| `oracle.sql.avg_elapsed_time` | Average elapsed time | Gauge | ms |
| `oracle.sql.avg_cpu_time` | Average CPU time | Gauge | ms |
| `oracle.sql.avg_buffer_gets` | Average buffer gets | Gauge | blocks |

### Wait Event Metrics

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `oracle.wait.total_waits` | Total wait count | Gauge | waits |
| `oracle.wait.time_waited` | Time waited | Gauge | seconds |
| `oracle.wait.avg_wait_time` | Average wait time | Gauge | seconds |
| `oracle.wait.time_percent` | Wait time percentage | Gauge | % |

### Tablespace Metrics

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `oracle.tablespace.size` | Tablespace total size | Gauge | MB |
| `oracle.tablespace.used` | Tablespace used space | Gauge | MB |
| `oracle.tablespace.free` | Tablespace free space | Gauge | MB |
| `oracle.tablespace.used_percent` | Used percentage | Gauge | % |

### ASM Metrics

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `oracle.asm.diskgroup.total` | Diskgroup total size | Gauge | MB |
| `oracle.asm.diskgroup.free` | Diskgroup free space | Gauge | MB |
| `oracle.asm.diskgroup.used_percent` | Used percentage | Gauge | % |

### RAC Metrics

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `oracle.rac.instance.status` | Instance status | Gauge | status |

### Redo Log Metrics

| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `oracle.redo.switches_per_hour` | Log switches per hour | Gauge | switches |
| `oracle.redo.size_per_hour` | Redo size per hour | Gauge | MB |
| `system.disk.operations` | Disk operations | Counter | operations |
| `system.network.io` | Network I/O | Counter | bytes |
| `system.network.errors` | Network errors | Counter | errors |

## Custom Mode Metrics

All Config-Only metrics plus:

### ASH (Active Session History) Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `db.ash.active_sessions` | Number of active sessions | Gauge | sessions |
| `db.ash.wait_events` | Wait event occurrences | Counter | events |
| `db.ash.blocked_sessions` | Number of blocked sessions | Gauge | sessions |
| `db.ash.long_running_queries` | Queries exceeding threshold | Gauge | queries |

### Query Intelligence Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `postgres.slow_queries.count` | Slow query executions | Counter | queries |
| `postgres.slow_queries.elapsed_time` | Query execution time | Gauge | milliseconds |
| `postgres.slow_queries.rows` | Rows processed | Gauge | rows |
| `postgres.slow_queries.shared_blks_hit` | Shared buffer hits | Counter | blocks |
| `postgres.slow_queries.shared_blks_read` | Shared buffer reads | Counter | blocks |

### Intelligent Processing Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `adaptive_sampling_rate` | Current sampling rate | Gauge | ratio |
| `circuit_breaker_state` | Circuit breaker status | Gauge | state |
| `circuit_breaker_trips` | Circuit breaker activations | Counter | trips |
| `cost_control_datapoints_processed` | Points processed | Counter | points |
| `cost_control_datapoints_dropped` | Points dropped for cost | Counter | points |

### Kernel Metrics
| Metric Name | Description | Type | Unit |
|-------------|-------------|------|------|
| `kernel.cpu.pressure` | CPU pressure metrics | Gauge | ratio |
| `kernel.memory.pressure` | Memory pressure metrics | Gauge | ratio |
| `kernel.io.pressure` | I/O pressure metrics | Gauge | ratio |

## Metric Details

### postgresql.backends
- **Description**: Current number of backend connections
- **Query**: `SELECT numbackends FROM pg_stat_database`
- **Attributes**: 
  - `db.name`: Database name
- **Use Case**: Monitor connection pool usage, detect connection leaks

### postgresql.commits
- **Description**: Number of transactions committed
- **Query**: `SELECT xact_commit FROM pg_stat_database`
- **Attributes**:
  - `db.name`: Database name
- **Use Case**: Monitor transaction throughput

### postgresql.deadlocks
- **Description**: Number of deadlocks detected
- **Query**: `SELECT deadlocks FROM pg_stat_database`
- **Attributes**:
  - `db.name`: Database name
- **Use Case**: Identify locking issues

### postgresql.database.size
- **Description**: Size of each database in bytes
- **Query**: `SELECT pg_database_size(datname)`
- **Attributes**:
  - `db.name`: Database name
- **Use Case**: Capacity planning, growth tracking

### db.ash.active_sessions
- **Description**: Real-time session activity
- **Query**: Custom ASH implementation
- **Attributes**:
  - `state`: Session state (active, idle, etc.)
  - `wait_event_type`: Type of wait event
  - `wait_event_name`: Specific wait event
- **Use Case**: Performance troubleshooting

## NRQL Query Examples

### Connection Monitoring
```sql
-- Current connections by state
SELECT latest(postgresql.backends) 
FROM Metric 
FACET db.name 
WHERE deployment.mode = 'config-only'

-- Connection pool usage
SELECT latest(postgresql.backends) / latest(postgresql.connection.max) * 100 as 'Pool Usage %' 
FROM Metric 
WHERE deployment.mode = 'config-only'
```

### Transaction Analysis
```sql
-- Transaction rate
SELECT rate(sum(postgresql.commits), 1 minute) as 'Commits/min',
       rate(sum(postgresql.rollbacks), 1 minute) as 'Rollbacks/min'
FROM Metric 
FACET deployment.mode 
TIMESERIES

-- Rollback ratio
SELECT sum(postgresql.rollbacks) / (sum(postgresql.commits) + sum(postgresql.rollbacks)) * 100 as 'Rollback %'
FROM Metric 
FACET db.name 
SINCE 1 hour ago
```

### Performance Metrics
```sql
-- Buffer cache hit ratio
SELECT sum(postgresql.blks_hit) / (sum(postgresql.blks_hit) + sum(postgresql.blks_read)) * 100 as 'Cache Hit %'
FROM Metric 
FACET deployment.mode 
SINCE 1 hour ago

-- Index usage efficiency
SELECT sum(postgresql.index.scans) / (sum(postgresql.index.scans) + sum(postgresql.sequential_scans)) * 100 as 'Index Usage %'
FROM Metric 
FACET db.name 
SINCE 1 hour ago
```

### Database Growth
```sql
-- Database size over time
SELECT latest(postgresql.database.size) / 1024 / 1024 / 1024 as 'Size (GB)'
FROM Metric 
FACET db.name 
TIMESERIES 1 hour 
SINCE 1 week ago

-- Table size ranking
SELECT latest(postgresql.table.size) / 1024 / 1024 as 'Size (MB)'
FROM Metric 
FACET table_name 
LIMIT 20
```

### Wait Event Analysis (Custom Mode)
```sql
-- Top wait events
SELECT sum(db.ash.wait_events) 
FROM Metric 
WHERE deployment.mode = 'custom' 
FACET attributes.wait_event_type, attributes.wait_event_name 
SINCE 30 minutes ago

-- Blocked sessions over time
SELECT sum(db.ash.blocked_sessions) 
FROM Metric 
WHERE deployment.mode = 'custom' 
TIMESERIES 5 minutes 
SINCE 1 hour ago
```

### Query Performance (Custom Mode)
```sql
-- Slowest queries
SELECT average(postgres.slow_queries.elapsed_time) as 'Avg Time (ms)',
       sum(postgres.slow_queries.count) as 'Executions'
FROM Metric 
WHERE deployment.mode = 'custom' 
FACET attributes.query_id 
SINCE 1 hour ago 
LIMIT 10

-- Query plans distribution
SELECT count(*) 
FROM Metric 
WHERE deployment.mode = 'custom' 
AND attributes.query_plan_type IS NOT NULL 
FACET attributes.query_plan_type 
SINCE 1 hour ago
```

### Cost Analysis
```sql
-- Data points by mode
SELECT rate(count(*), 1 minute) as 'DPM'
FROM Metric 
WHERE metricName LIKE 'postgresql%' 
FACET deployment.mode 
TIMESERIES 10 minutes 
SINCE 1 hour ago

-- Cost optimization impact
SELECT sum(cost_control_datapoints_dropped) as 'Points Dropped',
       sum(cost_control_datapoints_processed) as 'Points Processed'
FROM Metric 
WHERE deployment.mode = 'custom' 
SINCE 1 hour ago
```

## Metric Collection Best Practices

1. **Essential Metrics** (Always Enable)
   - postgresql.backends
   - postgresql.commits/rollbacks
   - postgresql.deadlocks
   - postgresql.database.size

2. **Performance Metrics** (Enable for Troubleshooting)
   - postgresql.blks_hit/read
   - postgresql.index.scans
   - postgresql.sequential_scans
   - postgresql.temp_files

3. **Replication Metrics** (Enable if Using Replication)
   - postgresql.wal.*
   - postgresql.replication.*

4. **Custom Mode Metrics** (Enable for Deep Analysis)
   - db.ash.*
   - postgres.slow_queries.*
   - Query intelligence metrics

## Alerts Based on Metrics

### Critical Alerts
```sql
-- High connection count
SELECT latest(postgresql.backends) 
FROM Metric 
WHERE latest(postgresql.backends) > 0.8 * latest(postgresql.connection.max)

-- Deadlocks detected
SELECT sum(postgresql.deadlocks) 
FROM Metric 
WHERE sum(postgresql.deadlocks) > 0 
SINCE 5 minutes ago

-- Replication lag
SELECT latest(postgresql.wal.lag) 
FROM Metric 
WHERE latest(postgresql.wal.lag) > 10000000
```

### Warning Alerts
```sql
-- Low cache hit ratio
SELECT sum(postgresql.blks_hit) / (sum(postgresql.blks_hit) + sum(postgresql.blks_read)) * 100 
FROM Metric 
WHERE sum(postgresql.blks_hit) / (sum(postgresql.blks_hit) + sum(postgresql.blks_read)) * 100 < 90 
SINCE 10 minutes ago

-- High rollback ratio
SELECT sum(postgresql.rollbacks) / (sum(postgresql.commits) + sum(postgresql.rollbacks)) * 100 
FROM Metric 
WHERE sum(postgresql.rollbacks) / (sum(postgresql.commits) + sum(postgresql.rollbacks)) * 100 > 10 
SINCE 10 minutes ago
```