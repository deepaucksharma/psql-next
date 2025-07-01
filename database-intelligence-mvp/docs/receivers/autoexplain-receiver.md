# Auto-Explain Receiver

The Auto-Explain receiver collects and analyzes PostgreSQL execution plans from auto_explain logs, providing insights into query performance and detecting plan regressions.

## Overview

The Auto-Explain receiver monitors PostgreSQL log files for auto_explain output, parses execution plans, and generates metrics about query performance. It includes advanced features like plan anonymization, regression detection, and intelligent storage management.

## Features

- **Safe Plan Collection**: Collects plans from logs without impacting database performance
- **Plan Anonymization**: Removes PII and sensitive data from execution plans
- **Regression Detection**: Identifies performance degradations using statistical analysis
- **Multiple Log Formats**: Supports JSON, CSV, and text log formats
- **Plan Versioning**: Tracks plan history and changes over time
- **Intelligent Caching**: LRU cache for efficient plan storage

## Configuration

### Basic Configuration

```yaml
receivers:
  autoexplain:
    log_path: /var/log/postgresql/postgresql.log
    log_format: json  # json, csv, or text
    
    database:
      endpoint: localhost:5432
      username: postgres
      password: postgres
      database: postgres
```

### Full Configuration

```yaml
receivers:
  autoexplain:
    log_path: /var/log/postgresql/postgresql.log
    log_format: json
    
    # Database connection for enrichment
    database:
      endpoint: localhost:5432
      username: postgres
      password: postgres
      database: postgres
      ssl_mode: disable
      max_connections: 10
    
    # Plan collection settings
    plan_collection:
      enabled: true
      min_duration: 100ms           # Only collect plans for queries > 100ms
      max_plans_per_query: 10       # Keep last 10 plans per query
      retention_duration: 24h       # Keep plans for 24 hours
      
      # Regression detection
      regression_detection:
        enabled: true
        performance_degradation_threshold: 0.2  # Alert on 20% slowdown
        cost_increase_threshold: 0.5           # Alert on 50% cost increase
        min_executions: 10                     # Need 10 executions for comparison
        statistical_confidence: 0.95           # 95% confidence level
        
        # Node-specific analyzers
        node_analyzers:
          - type: "Seq Scan"
            cost_weight: 1.5
            alert_on_table_size: 1000000  # Alert if seq scan on table > 1M rows
          
          - type: "Nested Loop"
            cost_weight: 2.0
            alert_on_rows: 10000  # Alert if nested loop processes > 10k rows
    
    # Plan anonymization
    plan_anonymization:
      enabled: true
      anonymize_filters: true          # Anonymize WHERE conditions
      anonymize_join_conditions: true  # Anonymize JOIN conditions
      remove_cost_estimates: false     # Keep cost estimates
      hash_literals: true              # Replace literals with hashes
      
      # Sensitive node types to anonymize
      sensitive_node_types:
        - Filter
        - Index Cond
        - Recheck Cond
        - Function Scan
        - Hash Cond
        - Merge Cond
      
      # Patterns to detect and anonymize
      sensitive_patterns:
        - email
        - ssn
        - credit_card
        - phone
        - ip_address
        - api_key
        - password
```

## Metrics

The Auto-Explain receiver generates the following metrics:

### Query Performance Metrics

- **`db.postgresql.query.plan_time`** (gauge)
  - Description: Time spent planning the query
  - Unit: milliseconds
  - Attributes: `query_id`, `database`, `username`

- **`db.postgresql.query.exec_time`** (gauge)
  - Description: Time spent executing the query
  - Unit: milliseconds
  - Attributes: `query_id`, `database`, `username`

- **`db.postgresql.query.rows`** (gauge)
  - Description: Number of rows returned by the query
  - Unit: rows
  - Attributes: `query_id`, `database`

### Plan Metrics

- **`db.postgresql.plan.cost`** (gauge)
  - Description: Estimated cost of the execution plan
  - Unit: cost units
  - Attributes: `query_id`, `plan_hash`, `node_type`

- **`db.postgresql.plan.changes`** (counter)
  - Description: Number of plan changes detected
  - Unit: changes
  - Attributes: `query_id`, `change_type`

### Regression Metrics

- **`db.postgresql.plan.regression`** (gauge)
  - Description: Plan regression severity (0-1 scale)
  - Unit: ratio
  - Attributes: `query_id`, `regression_type`, `old_plan_hash`, `new_plan_hash`

- **`db.postgresql.plan.regression.detected`** (counter)
  - Description: Number of plan regressions detected
  - Unit: regressions
  - Attributes: `query_id`, `regression_type`

## Setup Requirements

### PostgreSQL Configuration

Enable auto_explain in PostgreSQL:

```sql
-- Enable auto_explain globally
ALTER SYSTEM SET shared_preload_libraries = 'auto_explain';
ALTER SYSTEM SET auto_explain.log_min_duration = 100;  -- Log queries > 100ms
ALTER SYSTEM SET auto_explain.log_analyze = true;      -- Include actual times
ALTER SYSTEM SET auto_explain.log_buffers = true;      -- Include buffer usage
ALTER SYSTEM SET auto_explain.log_format = 'json';     -- Use JSON format
ALTER SYSTEM SET auto_explain.log_nested_statements = true;

-- Reload configuration
SELECT pg_reload_conf();
```

### Log File Permissions

Ensure the collector has read access to PostgreSQL logs:

```bash
# Grant read permissions
chmod 644 /var/log/postgresql/postgresql.log

# Or add collector user to postgres group
usermod -a -G postgres otel-collector
```

## Plan Anonymization

The receiver includes sophisticated plan anonymization to protect sensitive data:

### Anonymization Features

1. **Literal Value Replacement**
   - Email addresses → `<EMAIL_REDACTED>`
   - SSNs → `<SSN_REDACTED>`
   - Credit cards → `<CC_REDACTED>`
   - Phone numbers → `<PHONE_REDACTED>`
   - IP addresses → `<IP_REDACTED>`

2. **Pattern Detection**
   - Uses regex patterns to identify sensitive data
   - Configurable pattern list
   - Context-aware anonymization

3. **Structural Preservation**
   - Maintains plan structure for analysis
   - Preserves cost estimates and statistics
   - Keeps node relationships intact

### Example

Before anonymization:
```json
{
  "Node Type": "Index Scan",
  "Index Name": "users_email_idx",
  "Filter": "(email = 'john.doe@example.com')",
  "Rows Removed by Filter": 0
}
```

After anonymization:
```json
{
  "Node Type": "Index Scan",
  "Index Name": "users_email_idx",
  "Filter": "(email = '<EMAIL_REDACTED>')",
  "Rows Removed by Filter": 0
}
```

## Regression Detection

The receiver uses statistical analysis to detect plan regressions:

### Detection Methods

1. **Statistical Significance Testing**
   - Uses Welch's t-test for comparing execution times
   - Configurable confidence levels (default: 95%)
   - Requires minimum execution count for reliability

2. **Cost-Based Analysis**
   - Compares estimated costs between plans
   - Weighted by node type importance
   - Alerts on significant cost increases

3. **Node-Level Analysis**
   - Detects problematic node types (e.g., large sequential scans)
   - Identifies missing indexes
   - Tracks join method changes

### Regression Types

- **Performance Regression**: Execution time increase > threshold
- **Cost Regression**: Plan cost increase > threshold
- **Plan Structure Change**: Major changes in plan structure
- **Resource Regression**: Increased buffer/IO usage

## Best Practices

1. **Log Rotation**
   - Configure appropriate log rotation to manage disk space
   - Ensure receiver handles log rotation gracefully
   - Use `log_truncate_on_rotation = off` for continuity

2. **Sampling Strategy**
   - Start with higher `log_min_duration` (e.g., 1000ms)
   - Gradually decrease as you understand load
   - Use sampling for high-traffic databases

3. **Resource Management**
   - Monitor receiver memory usage
   - Adjust `max_plans_per_query` based on cardinality
   - Configure appropriate retention periods

4. **Security**
   - Always enable plan anonymization in production
   - Review anonymization patterns regularly
   - Use read-only database connections

## Troubleshooting

### Common Issues

1. **Log Parsing Errors**
   - Verify log format matches configuration
   - Check for log file encoding issues
   - Ensure complete JSON objects in logs

2. **High Memory Usage**
   - Reduce `max_plans_per_query`
   - Decrease `retention_duration`
   - Enable plan compression

3. **Missing Plans**
   - Verify auto_explain is loaded
   - Check `log_min_duration` setting
   - Ensure log file permissions

### Debug Mode

Enable debug logging:

```yaml
service:
  telemetry:
    logs:
      level: debug
      encoding: json
```

## Example Pipeline

Complete example with processors and exporters:

```yaml
receivers:
  autoexplain:
    log_path: /var/log/postgresql/postgresql.log
    log_format: json
    plan_collection:
      enabled: true
      regression_detection:
        enabled: true

processors:
  memory_limiter:
    limit_percentage: 80
  
  resource:
    attributes:
      - key: service.name
        value: postgresql
        action: upsert

exporters:
  otlp:
    endpoint: localhost:4317

service:
  pipelines:
    metrics:
      receivers: [autoexplain]
      processors: [memory_limiter, resource]
      exporters: [otlp]
```

## Performance Impact

- **Minimal Database Impact**: Reads logs only, no database queries for plan collection
- **Efficient Parsing**: Optimized parsers for each log format
- **Smart Caching**: LRU cache prevents memory bloat
- **Async Processing**: Non-blocking log file monitoring

## Integration with Other Components

- **Plan Attribute Extractor**: Enriches metrics with plan details
- **Circuit Breaker**: Protects against log parsing storms
- **Adaptive Sampler**: Adjusts collection based on load
- **ASH Receiver**: Correlates plans with session activity