# Oracle Database Maximum Metrics Extraction Guide

This guide demonstrates how to extract 120+ metrics from Oracle Database using only stock OpenTelemetry components.

## Overview

The `oracle-maximum-extraction.yaml` configuration demonstrates:
- **120+ distinct metrics** from Oracle Database
- **V$ view integration** for real-time monitoring
- **Wait event analysis** with categorization
- **ASM and RAC** monitoring support
- **Data Guard** replication metrics
- **Tablespace and segment** management
- **SQL performance** with execution plans

## Quick Start

```bash
# 1. Set environment variables
export ORACLE_HOST=localhost
export ORACLE_PORT=1521
export ORACLE_SERVICE=ORCLPDB1
export ORACLE_USER=system
export ORACLE_PASSWORD=your_password
export NEW_RELIC_LICENSE_KEY=your_license_key

# 2. Run the collector
docker run -d \
  --name otel-oracle-max \
  -v $(pwd)/configs/oracle-maximum-extraction.yaml:/etc/otelcol/config.yaml \
  -e ORACLE_HOST \
  -e ORACLE_PORT \
  -e ORACLE_SERVICE \
  -e ORACLE_USER \
  -e ORACLE_PASSWORD \
  -e NEW_RELIC_LICENSE_KEY \
  -p 8894:8894 \
  otel/opentelemetry-collector-contrib:latest
```

## Prerequisites

### 1. Create Monitoring User

```sql
-- Create monitoring user
CREATE USER otel_monitor IDENTIFIED BY "SecurePassword123";

-- Grant necessary privileges
GRANT CREATE SESSION TO otel_monitor;
GRANT SELECT ANY DICTIONARY TO otel_monitor;
GRANT SELECT_CATALOG_ROLE TO otel_monitor;

-- For specific monitoring needs
GRANT SELECT ON V_$SESSION TO otel_monitor;
GRANT SELECT ON V_$SYSTEM_EVENT TO otel_monitor;
GRANT SELECT ON V_$SYSSTAT TO otel_monitor;
GRANT SELECT ON V_$SGAINFO TO otel_monitor;
GRANT SELECT ON V_$INSTANCE TO otel_monitor;
GRANT SELECT ON V_$DATABASE TO otel_monitor;
GRANT SELECT ON V_$SQLAREA TO otel_monitor;
GRANT SELECT ON DBA_DATA_FILES TO otel_monitor;
GRANT SELECT ON DBA_FREE_SPACE TO otel_monitor;
GRANT SELECT ON DBA_TABLESPACES TO otel_monitor;

-- For RAC monitoring
GRANT SELECT ON GV_$INSTANCE TO otel_monitor;
GRANT SELECT ON GV_$SESSION TO otel_monitor;

-- For ASM monitoring (if using ASM)
GRANT SELECT ON V_$ASM_DISKGROUP TO otel_monitor;
```

### 2. Enable Statistics Collection

```sql
-- Ensure statistics are being collected
EXEC DBMS_STATS.GATHER_DATABASE_STATS;

-- Set statistics level
ALTER SYSTEM SET STATISTICS_LEVEL='TYPICAL' SCOPE=BOTH;

-- Enable timed statistics
ALTER SYSTEM SET TIMED_STATISTICS=TRUE SCOPE=BOTH;
```

## Metrics Categories

### 1. Core Database Metrics

Fundamental database health indicators:
- **Database Status**: Open mode, role, protection mode
- **Instance Uptime**: Time since startup
- **SGA Components**: Buffer cache, shared pool, large pool
- **Buffer Cache Hit Ratio**: Cache efficiency

### 2. Session and Connection Metrics

Active session monitoring:
- **Session Count**: By status, state, wait class
- **Blocked Sessions**: Blocking chain detection
- **Wait Time**: Maximum and average by event
- **Connection Pool**: User sessions vs background

### 3. SQL Performance Metrics

Top SQL analysis:
- **Execution Statistics**: Count, elapsed time, CPU time
- **Resource Usage**: Buffer gets, disk reads
- **Average Performance**: Per-execution metrics
- **Plan Hash Values**: For plan stability

### 4. Wait Event Analysis

System-wide wait statistics:
- **Wait Classes**: User I/O, System I/O, Concurrency, etc.
- **Time Distribution**: Percentage of total wait time
- **Wait Counts**: Frequency of each wait event
- **Average Wait Time**: Per-event duration

### 5. Tablespace Management

Space usage and allocation:
- **Space Metrics**: Total, used, free space
- **Usage Percentage**: For proactive management
- **Autoextend Status**: Maximum possible size
- **Contents Type**: Permanent, temporary, undo

### 6. ASM Disk Groups

Automatic Storage Management:
- **Disk Group Status**: Mounted, dismounted
- **Space Usage**: Total, free, usable
- **Redundancy Type**: External, normal, high
- **Offline Disks**: Failed disk detection

### 7. RAC Monitoring

Real Application Clusters:
- **Instance Status**: All nodes in cluster
- **Interconnect Stats**: Network performance
- **Global Cache**: Block transfers
- **Service Distribution**: Load balancing

### 8. Data Guard Metrics

Disaster recovery monitoring:
- **Database Role**: Primary, standby
- **Protection Mode**: Maximum availability, performance
- **Switchover Status**: Ready state
- **Apply Lag**: Standby delay

## Configuration Breakdown

### SQLQuery Receiver Pattern

```yaml
sqlquery/oracle_core:
  driver: oracle
  datasource: "oracle://user:pass@host:port/service"
  collection_interval: 10s
  queries:
    - sql: "SELECT ... FROM v$database"
      metrics:
        - metric_name: oracle.database.metric
          value_column: column_name
          attribute_columns: [attr1, attr2]
```

### Multi-Pipeline Strategy

```yaml
service:
  pipelines:
    # Critical session monitoring (5s)
    metrics/high_frequency:
      receivers: [sqlquery/oracle_sessions]
      
    # Core metrics and waits (10s)
    metrics/standard:
      receivers: [sqlquery/oracle_core, sqlquery/oracle_waits]
      
    # SQL performance (30s)
    metrics/performance:
      receivers: [sqlquery/oracle_performance, sqlquery/oracle_rac]
      
    # Space and maintenance (60s)
    metrics/analytics:
      receivers: [sqlquery/oracle_tablespace, sqlquery/oracle_asm]
```

### Intelligent Categorization

```yaml
transform/add_metadata:
  metric_statements:
    # Classify wait severity
    - set(attributes["wait.severity"], "critical") 
      where name == "oracle.wait.time_percent" and value > 20
      
    # Classify tablespace usage
    - set(attributes["tablespace.status"], "critical") 
      where name == "oracle.tablespace.used_percent" and value > 90
```

## Performance Tuning

### 1. Optimize SQL Queries

```sql
-- Use ROWNUM to limit results
SELECT * FROM (
  SELECT ... FROM v$sqlarea
  ORDER BY elapsed_time DESC
) WHERE ROWNUM <= 50

-- Filter noise
WHERE executions > 10  -- Skip one-off queries
  AND parsing_schema_name NOT IN ('SYS', 'SYSTEM')
```

### 2. Collection Interval Tuning

- **5s**: Active sessions (blocking detection)
- **10s**: Core metrics, buffer cache
- **30s**: SQL performance, RAC status
- **60s**: Tablespace usage, ASM status
- **300s**: Segment statistics

### 3. Resource Management

```yaml
# Limit result sets
filter/reduce_cardinality:
  metrics:
    metric:
      - 'name == "oracle.sql.executions" and value < 100'
```

## Monitoring Best Practices

### 1. Key Metrics to Alert On

- `oracle.buffer_cache.hit_ratio` < 90%
- `oracle.tablespace.used_percent` > 85%
- `oracle.sessions.blocked` > 0
- `oracle.wait.time_percent` > 15% (non-idle)
- `oracle.redo.switches_per_hour` > 20
- `oracle.asm.diskgroup.used_percent` > 80%

### 2. Dashboard Layout

- **Overview**: Database status, uptime, sessions
- **Performance**: Top SQL, wait events, buffer cache
- **Storage**: Tablespace usage, ASM groups
- **RAC Status**: Node health, interconnect stats
- **Data Guard**: Replication lag, protection status

### 3. Troubleshooting Approach

1. Check wait event distribution
2. Identify top SQL by resource usage
3. Review session blocking
4. Monitor tablespace growth
5. Verify backup and archive status

## Troubleshooting

### No Metrics Appearing

```sql
-- Test connectivity
SELECT * FROM v$version;

-- Verify permissions
SELECT * FROM session_privs WHERE privilege LIKE '%SELECT%';

-- Check if views are accessible
SELECT COUNT(*) FROM v$session;
```

### Oracle Driver Issues

```yaml
# Ensure Oracle Instant Client is available
# For containerized deployments:
FROM otel/opentelemetry-collector-contrib:latest
RUN apt-get update && apt-get install -y \
  libaio1 \
  && wget https://download.oracle.com/otn_software/linux/instantclient/instantclient-basic-linux.zip \
  && unzip instantclient-basic-linux.zip -d /opt/oracle
ENV LD_LIBRARY_PATH=/opt/oracle/instantclient_21_1:$LD_LIBRARY_PATH
```

### Performance Impact

1. Use sampling for expensive queries:
```sql
SELECT * FROM (
  SELECT ... FROM v$sql SAMPLE(10)  -- 10% sample
)
```

2. Increase collection intervals during peak hours

## Example Queries

### Top SQL by CPU

```sql
SELECT average(oracle.sql.avg_cpu_time) 
FROM Metric 
WHERE deployment.mode = 'config-only-oracle-max' 
FACET sql_id, sql_text_sample 
SINCE 1 hour ago
```

### Wait Event Distribution

```sql
SELECT average(oracle.wait.time_percent) 
FROM Metric 
WHERE deployment.mode = 'config-only-oracle-max' 
  AND wait_class != 'Idle'
FACET wait_class, event 
SINCE 1 hour ago
```

### Tablespace Growth

```sql
SELECT latest(oracle.tablespace.used_percent) 
FROM Metric 
WHERE deployment.mode = 'config-only-oracle-max' 
FACET tablespace_name 
COMPARE WITH 1 day ago
```

### RAC Node Status

```sql
SELECT latest(oracle.rac.instance.status) 
FROM Metric 
WHERE deployment.mode = 'config-only-oracle-max' 
FACET instance_name, host_name
```

## Advanced Features

### 1. AWR Integration

```yaml
sqlquery/oracle_awr:
  queries:
    - sql: |
        SELECT 
          snap_id,
          begin_interval_time,
          end_interval_time,
          stat_name,
          value
        FROM dba_hist_sysstat
        WHERE snap_id = (SELECT MAX(snap_id) FROM dba_hist_snapshot)
```

### 2. Partition Monitoring

```yaml
sqlquery/oracle_partitions:
  queries:
    - sql: |
        SELECT 
          table_owner,
          table_name,
          partition_name,
          tablespace_name,
          num_rows,
          blocks,
          last_analyzed
        FROM dba_tab_partitions
        WHERE table_owner NOT IN ('SYS', 'SYSTEM')
```

### 3. Flashback Status

```yaml
sqlquery/oracle_flashback:
  queries:
    - sql: |
        SELECT 
          oldest_flashback_scn,
          oldest_flashback_time,
          retention_target,
          flashback_size,
          estimated_flashback_size
        FROM v$flashback_database_log
```

## Multi-Tenant (CDB/PDB) Support

```yaml
sqlquery/oracle_multitenant:
  queries:
    - sql: |
        SELECT 
          con_id,
          name,
          open_mode,
          restricted,
          total_size
        FROM v$containers
```

## Conclusion

This configuration extracts 120+ metrics from Oracle Database using only OpenTelemetry configuration:
- ✅ No custom code required
- ✅ Comprehensive V$ view coverage
- ✅ RAC and ASM support
- ✅ Data Guard monitoring
- ✅ Intelligent categorization

The patterns work with Oracle Database 12c+ and support Enterprise Edition features when available.