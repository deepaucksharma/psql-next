# Query Log Collection Setup Guide

## Overview

The Database Intelligence Collector supports advanced query log analysis through custom processors. This guide covers how to configure database query logging and integrate it with the collector.

## Table of Contents

1. [PostgreSQL Query Logging](#postgresql-query-logging)
2. [MySQL Query Logging](#mysql-query-logging)
3. [Log Collection Configuration](#log-collection-configuration)
4. [Custom Processors for Query Analysis](#custom-processors-for-query-analysis)
5. [Performance Considerations](#performance-considerations)
6. [Troubleshooting](#troubleshooting)

## PostgreSQL Query Logging

### 1. Enable Query Logging

Edit `postgresql.conf`:

```ini
# Basic query logging
logging_collector = on
log_directory = '/var/log/postgresql'
log_filename = 'postgresql-%Y-%m-%d_%H%M%S.log'
log_rotation_age = 1d
log_rotation_size = 100MB

# What to log
log_statement = 'all'              # Log all statements
log_duration = on                  # Log statement duration
log_min_duration_statement = 100   # Log queries slower than 100ms
log_line_prefix = '%t [%p] %u@%d ' # Timestamp, PID, user@database

# Additional useful settings
log_checkpoints = on
log_connections = on
log_disconnections = on
log_lock_waits = on
log_temp_files = 0

# For query plan analysis
auto_explain.log_min_duration = 1000  # Log plans for queries > 1s
auto_explain.log_analyze = true
auto_explain.log_buffers = true
auto_explain.log_format = json
```

### 2. Enable pg_stat_statements

```sql
-- Create extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Configure in postgresql.conf
shared_preload_libraries = 'pg_stat_statements,auto_explain'
pg_stat_statements.track = all
pg_stat_statements.track_utility = on
pg_stat_statements.max = 10000
```

### 3. Sample Log Output

```
2025-06-30 12:45:00.123 UTC [12345] monitor@production LOG:  duration: 250.567 ms  statement: SELECT COUNT(*) FROM orders WHERE status = 'pending'
2025-06-30 12:45:01.456 UTC [12346] app@production LOG:  duration: 1234.890 ms  plan:
	{
	  "Plan": {
	    "Node Type": "Hash Join",
	    "Total Cost": 250.00,
	    "Plan Rows": 100,
	    "Plan Width": 32,
	    "Actual Rows": 150,
	    "Actual Loops": 1
	  }
	}
```

## MySQL Query Logging

### 1. Enable Slow Query Log

Edit `my.cnf`:

```ini
[mysqld]
# Slow query log
slow_query_log = 1
slow_query_log_file = /var/log/mysql/slow.log
long_query_time = 0.1  # Log queries slower than 100ms
log_queries_not_using_indexes = 1

# General query log (use with caution in production)
general_log = 0  # Set to 1 to enable
general_log_file = /var/log/mysql/general.log

# Binary logging for audit
log_bin = /var/log/mysql/mysql-bin
binlog_format = ROW
expire_logs_days = 7

# Performance schema for detailed metrics
performance_schema = ON
performance_schema_events_statements_history_size = 1000
```

### 2. Enable Performance Schema

```sql
-- Enable statement instrumentation
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE '%statement/%';

-- Enable consumers
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME LIKE '%events_statements_%';

-- Create monitoring user
CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'secure_password';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
GRANT PROCESS ON *.* TO 'otel_monitor'@'%';
```

### 3. Sample Log Output

```
# Time: 2025-06-30T12:45:01.456789Z
# User@Host: app[app] @ web-server-1 [10.0.0.5]  Id: 12345
# Query_time: 0.250567  Lock_time: 0.000123 Rows_sent: 1  Rows_examined: 50000
SET timestamp=1719754321;
SELECT COUNT(*) FROM orders WHERE created_at > DATE_SUB(NOW(), INTERVAL 7 DAY);
```

## Log Collection Configuration

### 1. OpenTelemetry Collector Configuration

```yaml
receivers:
  # PostgreSQL log collection
  filelog/postgres:
    include: 
      - /var/log/postgresql/*.log
    start_at: beginning
    operators:
      # Parse PostgreSQL log format
      - type: regex_parser
        regex: '^(?P<timestamp>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3} \w+) \[(?P<pid>\d+)\] (?P<user>\w+)@(?P<database>\w+) (?P<level>\w+):  '
        timestamp:
          parse_from: attributes.timestamp
          layout: '2006-01-02 15:04:05.000 MST'
      
      # Extract duration and statement
      - type: regex_parser
        if: 'body matches "duration:"'
        regex: 'duration: (?P<duration>[\d.]+) ms  statement: (?P<query>.*)'
        
      # Extract query plan if present
      - type: regex_parser
        if: 'body matches "plan:"'
        regex: 'plan:\s*(?P<plan_json>\{.*\})'
        
      # Add metadata
      - type: add
        field: resource.db.system
        value: postgresql
        
  # MySQL slow query log collection
  filelog/mysql:
    include: 
      - /var/log/mysql/slow.log
    start_at: beginning
    multiline:
      line_start_pattern: '^# Time:'
    operators:
      # Parse MySQL slow query format
      - type: regex_parser
        regex: '# User@Host: (?P<user>\w+)\[.*?\] @ (?P<host>[\w\-\.]+).*Id:\s*(?P<connection_id>\d+)'
        
      - type: regex_parser
        regex: '# Query_time: (?P<query_time>[\d.]+)\s+Lock_time: (?P<lock_time>[\d.]+)\s+Rows_sent: (?P<rows_sent>\d+)\s+Rows_examined: (?P<rows_examined>\d+)'
        
      - type: regex_parser
        regex: 'SET timestamp=(?P<timestamp>\d+);'
        timestamp:
          parse_from: attributes.timestamp
          layout_type: epoch
          
      - type: add
        field: resource.db.system
        value: mysql

processors:
  # Extract query plan attributes
  planattributeextractor:
    safe_mode: true
    timeout_ms: 1000
    error_mode: ignore
    postgresql_rules:
      detection_jsonpath: "$.plan_json"
      extractions:
        "db.query.plan.cost": "$.Plan['Total Cost']"
        "db.query.plan.rows": "$.Plan['Plan Rows']"
        "db.query.plan.operation": "$.Plan['Node Type']"
    mysql_rules:
      detection_jsonpath: "$.query_time"
      extractions:
        "db.query.duration_ms": "float($.query_time) * 1000"
        "db.query.rows_examined": "$.rows_examined"
        
  # Adaptive sampling for query logs
  adaptivesampler:
    in_memory_only: true
    default_sampling_rate: 10  # Sample 10% by default
    rules:
      - name: "slow_queries"
        condition: |
          float(attributes["duration"]) > 1000 or 
          float(attributes["query_time"]) > 1.0
        sampling_rate: 100  # Always sample slow queries
        
      - name: "error_queries"
        condition: 'attributes["level"] == "ERROR"'
        sampling_rate: 100  # Always sample errors
        
      - name: "fast_queries"
        condition: |
          float(attributes["duration"]) < 10 or 
          float(attributes["query_time"]) < 0.01
        sampling_rate: 1  # Sample 1% of fast queries
        
  # Circuit breaker for protection
  circuitbreaker:
    failure_threshold: 10
    timeout: 30s
    half_open_requests: 5
    databases:
      - name: "postgresql"
        max_queries_per_second: 1000
      - name: "mysql"
        max_queries_per_second: 1000
```

### 2. Docker Volume Mounts

```yaml
services:
  otel-collector:
    volumes:
      # PostgreSQL logs
      - /var/lib/postgresql/data/log:/var/log/postgresql:ro
      # MySQL logs
      - /var/lib/mysql/log:/var/log/mysql:ro
      # Collector config
      - ./config/collector.yaml:/etc/otel/config.yaml:ro
```

### 3. Kubernetes ConfigMap for Log Paths

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: log-paths
data:
  postgres.path: "/var/log/postgresql"
  mysql.path: "/var/log/mysql"
```

## Custom Processors for Query Analysis

### 1. Plan Attribute Extractor

Extracts query execution plan details:

```yaml
planattributeextractor:
  postgresql_rules:
    extractions:
      # Cost metrics
      "db.query.plan.total_cost": "$.Plan['Total Cost']"
      "db.query.plan.startup_cost": "$.Plan['Startup Cost']"
      
      # Row estimates
      "db.query.plan.estimated_rows": "$.Plan['Plan Rows']"
      "db.query.plan.actual_rows": "$.Plan['Actual Rows']"
      
      # Operation details
      "db.query.plan.node_type": "$.Plan['Node Type']"
      "db.query.plan.join_type": "$.Plan['Join Type']"
      
      # Performance indicators
      "db.query.plan.shared_hit_blocks": "$.Plan['Shared Hit Blocks']"
      "db.query.plan.shared_read_blocks": "$.Plan['Shared Read Blocks']"
      
  # Derived attributes
  derived_attributes:
    "db.query.plan.has_seq_scan": "contains(plan_json, 'Seq Scan')"
    "db.query.plan.has_nested_loop": "contains(plan_json, 'Nested Loop')"
    "db.query.plan.efficiency_score": "actual_rows / (total_cost + 1)"
```

### 2. Adaptive Sampler

Intelligent sampling based on query characteristics:

```yaml
adaptivesampler:
  rules:
    # Always capture problematic queries
    - name: "high_cost_queries"
      condition: 'float(attributes["db.query.plan.total_cost"]) > 10000'
      sampling_rate: 100
      
    # Sample based on operation type
    - name: "sequential_scans"
      condition: 'attributes["db.query.plan.has_seq_scan"] == "true"'
      sampling_rate: 50
      
    # Time-based sampling
    - name: "peak_hours"
      condition: 'hour(timestamp) >= 9 and hour(timestamp) <= 17'
      sampling_rate: 25
      
    # Database-specific rules
    - name: "critical_database"
      condition: 'attributes["database"] == "production"'
      sampling_rate: 50
```

### 3. Circuit Breaker

Protects databases from monitoring overhead:

```yaml
circuitbreaker:
  # Global settings
  failure_threshold: 5
  timeout: 30s
  
  # Per-database limits
  databases:
    - name: "production"
      max_queries_per_second: 100
      error_threshold: 0.1  # 10% error rate
      
    - name: "staging"
      max_queries_per_second: 500
      error_threshold: 0.2  # 20% error rate
```

## Performance Considerations

### 1. Log Rotation

Implement proper log rotation to prevent disk space issues:

```bash
# PostgreSQL logrotate config
/var/log/postgresql/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 postgres postgres
    postrotate
        /usr/bin/pg_ctl reload -D /var/lib/postgresql/data
    endscript
}
```

### 2. Resource Limits

Set appropriate resource limits in collector config:

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 20
    
service:
  telemetry:
    resource:
      # Limit file handles for log reading
      limits:
        open_files: 1024
```

### 3. Sampling Strategies

Balance between data completeness and performance:

- **Development**: 100% sampling
- **Staging**: 25-50% sampling
- **Production**: 10% baseline, 100% for errors/slow queries

## Troubleshooting

### 1. Common Issues

**Logs not being collected:**
```bash
# Check file permissions
ls -la /var/log/postgresql/
ls -la /var/log/mysql/

# Verify collector can read logs
docker exec otel-collector cat /var/log/postgresql/postgresql.log
```

**High memory usage:**
```yaml
# Reduce batch size
processors:
  batch:
    send_batch_size: 100
    timeout: 5s
```

**Missing query plans:**
```sql
-- PostgreSQL: Ensure auto_explain is loaded
SHOW shared_preload_libraries;
SELECT * FROM pg_extension WHERE extname = 'auto_explain';
```

### 2. Debug Mode

Enable debug logging for processors:

```yaml
processors:
  planattributeextractor:
    enable_debug_logging: true
    
service:
  telemetry:
    logs:
      level: debug
```

### 3. Validation

Test log parsing with sample data:

```bash
# Create test log
echo '2025-06-30 12:00:00.123 UTC [1234] user@db LOG:  duration: 123.456 ms  statement: SELECT 1' > test.log

# Run collector with test config
./otelcol --config=test-config.yaml
```

## Best Practices

1. **Start Small**: Begin with slow query logs before enabling general logging
2. **Use Sampling**: Implement intelligent sampling to reduce data volume
3. **Monitor Overhead**: Track the performance impact of log collection
4. **Secure Logs**: Ensure proper permissions and PII sanitization
5. **Regular Cleanup**: Implement log rotation and archival policies
6. **Test Thoroughly**: Validate parsing rules with real log samples

## Next Steps

1. Review [ARCHITECTURE.md](./ARCHITECTURE.md) for system design
2. See [DEPLOYMENT.md](./DEPLOYMENT.md) for production setup
3. Check [RUNBOOK.md](./RUNBOOK.md) for operational procedures