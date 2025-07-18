# MySQL Maximum Metrics Extraction Guide

This guide demonstrates how to extract 80+ metrics from MySQL using only stock OpenTelemetry components.

## Overview

The `mysql-maximum-extraction.yaml` configuration demonstrates:
- **80+ distinct metrics** from MySQL
- **Performance Schema integration** for query analysis
- **InnoDB internals monitoring**
- **Replication lag tracking**
- **Connection pool analysis**
- **Table and index statistics**

## Quick Start

```bash
# 1. Set environment variables
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=your_password
export NEW_RELIC_LICENSE_KEY=your_license_key

# 2. Run the collector
docker run -d \
  --name otel-mysql-max \
  -v $(pwd)/configs/mysql-maximum-extraction.yaml:/etc/otelcol/config.yaml \
  -e MYSQL_HOST \
  -e MYSQL_PORT \
  -e MYSQL_USER \
  -e MYSQL_PASSWORD \
  -e NEW_RELIC_LICENSE_KEY \
  -p 8889:8889 \
  otel/opentelemetry-collector-contrib:latest
```

## Prerequisites

### 1. Enable Performance Schema

```sql
-- Check if performance_schema is enabled
SHOW VARIABLES LIKE 'performance_schema';

-- If not enabled, add to my.cnf:
[mysqld]
performance_schema=ON
```

### 2. Grant Required Permissions

```sql
-- Create monitoring user
CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'secure_password';

-- Grant necessary permissions
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON information_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON mysql.* TO 'otel_monitor'@'%';

FLUSH PRIVILEGES;
```

## Metrics Categories

### 1. Core MySQL Metrics (40+)

The standard MySQL receiver provides:
- **Buffer Pool**: data pages, operations, usage
- **Commands**: execution counts by type
- **Connections**: active, errors, max allowed
- **InnoDB**: row locks, waits, deadlocks
- **Operations**: selects, inserts, updates, deletes
- **Query Performance**: slow queries, client counts
- **Replication**: lag time, SQL delay
- **Table I/O**: wait counts and time
- **Thread Pool**: active threads

### 2. Performance Schema Metrics

Detailed query analysis from `events_statements_summary_by_digest`:
- **Query Execution Count**: How many times each query ran
- **Average Latency**: Mean execution time per query
- **Max Latency**: Worst-case execution time
- **Rows Examined**: Average rows scanned
- **Temporary Tables**: Created during execution
- **Sort Operations**: Merge passes required

### 3. Connection Pool Analysis

Real-time connection monitoring:
- **Total Connections**: By user, host, database
- **Active vs Idle**: Connection state distribution
- **Stale Connections**: Idle for >10 minutes
- **Connection Efficiency**: Active percentage

### 4. Table and Index Statistics

Storage and performance metrics:
- **Table Sizes**: Data and index sizes in MB
- **Row Counts**: Records per table
- **Fragmentation**: Free space percentage
- **Storage Engine**: InnoDB, MyISAM, etc.

### 5. InnoDB Buffer Pool Details

Memory management insights:
- **Page Distribution**: Free, modified, old pages
- **Hit Rate**: Cache efficiency
- **Pending I/O**: Reads and writes
- **Page Activity**: Young/not young pages

## Configuration Breakdown

### Multi-Pipeline Architecture

```yaml
service:
  pipelines:
    # High-frequency session monitoring (5s)
    metrics/high_frequency:
      receivers: [sqlquery/processlist]
      
    # Standard MySQL metrics (10s)
    metrics/standard:
      receivers: [mysql, sqlquery/innodb, hostmetrics]
      
    # Performance analysis (30s)
    metrics/performance:
      receivers: [sqlquery/performance_schema]
      
    # Table analytics (60s)
    metrics/analytics:
      receivers: [sqlquery/table_stats]
```

### Smart Transformations

```yaml
transform/add_metadata:
  metric_statements:
    # Classify slow queries
    - set(attributes["query.classification"], "slow") 
      where name == "mysql.query.avg_latency" and value > 1000
      
    # Classify table sizes
    - set(attributes["table.size_category"], "large") 
      where name == "mysql.table.data_size" and value >= 1000
```

## Performance Tuning

### 1. Reduce Query Result Sets

Limit Performance Schema queries:
```yaml
sqlquery/performance_schema:
  queries:
    - sql: |
        SELECT ... FROM events_statements_summary_by_digest
        WHERE COUNT_STAR > 10  # Only frequent queries
        ORDER BY SUM_TIMER_WAIT DESC
        LIMIT 50  # Top 50 by total time
```

### 2. Filter System Schemas

```yaml
filter/reduce_cardinality:
  metrics:
    metric:
      - 'attributes["schema_name"] == "mysql"'
      - 'attributes["schema_name"] == "sys"'
```

### 3. Adjust Collection Intervals

- **5s**: Active sessions (critical)
- **10s**: Core metrics
- **30s**: Performance schema
- **60s**: Table statistics

## Monitoring Best Practices

### 1. Key Metrics to Alert On

- `mysql.connection_pool.active_percentage` > 80%
- `mysql.innodb.row_lock.time` > 1000ms
- `mysql.replication.lag` > 60s
- `mysql.query.slow.count` increasing
- `mysql.table.fragmentation` > 30%

### 2. Dashboard Organization

- **Overview**: Connections, QPS, key indicators
- **Performance**: Query analysis, slow queries
- **InnoDB**: Buffer pool, row locks, I/O
- **Replication**: Lag, thread status
- **Storage**: Table sizes, fragmentation

### 3. Optimization Workflow

1. Monitor slow query counts
2. Analyze query digest metrics
3. Check table fragmentation
4. Review connection pool efficiency
5. Optimize based on findings

## Troubleshooting

### No Performance Schema Metrics

```sql
-- Verify performance_schema is enabled
SHOW VARIABLES LIKE 'performance_schema';

-- Check available consumers
SELECT * FROM performance_schema.setup_consumers;

-- Enable statement digest
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME = 'statements_digest';
```

### High Memory Usage

1. Reduce Performance Schema history:
```sql
SET GLOBAL performance_schema_events_statements_history_size = 10;
```

2. Limit query result sets in config
3. Increase batch timeout

### Missing Replication Metrics

```sql
-- Check replication status
SHOW REPLICA STATUS\G

-- Ensure REPLICATION CLIENT privilege
SHOW GRANTS FOR 'otel_monitor'@'%';
```

## Example Queries

### Find Slow Queries in New Relic

```sql
SELECT average(mysql.query.avg_latency) 
FROM Metric 
WHERE deployment.mode = 'config-only-mysql-max' 
FACET query_digest 
SINCE 1 hour ago
```

### Connection Pool Efficiency

```sql
SELECT average(mysql.connection_pool.active_percentage) 
FROM Metric 
WHERE deployment.mode = 'config-only-mysql-max' 
FACET application_name, session_user 
SINCE 1 hour ago
```

### Table Growth Tracking

```sql
SELECT latest(mysql.table.data_size) 
FROM Metric 
WHERE deployment.mode = 'config-only-mysql-max' 
FACET schema_name, table_name 
SINCE 1 week ago 
COMPARE WITH 1 week ago
```

## Conclusion

This configuration extracts 80+ metrics from MySQL using only OpenTelemetry configuration:
- ✅ No custom code required
- ✅ Production-ready monitoring
- ✅ Deep performance insights
- ✅ Automatic metadata enrichment
- ✅ Multi-pipeline optimization

The same patterns can be adapted for MariaDB, Percona Server, and other MySQL-compatible databases.