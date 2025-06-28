# PostgreSQL Query Receiver

The PostgreSQL Query Receiver collects advanced telemetry from PostgreSQL databases, including slow query analysis, execution plan regression detection, wait event monitoring, and Active Session History (ASH) sampling.

## Features

- **Slow Query Analysis**: Collects detailed metrics from `pg_stat_statements`
- **Plan Regression Detection**: Monitors query execution plans for performance regressions
- **Active Session History (ASH)**: High-frequency sampling of database activity
- **Wait Event Analysis**: Tracks database wait events and blocking sessions
- **Multi-Database Support**: Monitor multiple PostgreSQL instances simultaneously
- **Adaptive Sampling**: Intelligent sampling based on query characteristics
- **Cloud Provider Optimization**: Automatic detection of RDS, Azure, and GCP environments
- **PII Sanitization**: Optional sanitization of sensitive data in queries

## Requirements

### PostgreSQL Extensions

- `pg_stat_statements` (required for slow query analysis)
- `pg_wait_sampling` (optional, for enhanced wait event analysis)
- `pg_stat_kcache` (optional, for kernel-level metrics)
- `pg_querylens` (optional, future support for query optimization)

### PostgreSQL Version

- PostgreSQL 12 or higher recommended
- PostgreSQL 10+ supported with reduced functionality

## Configuration

### Basic Example

```yaml
receivers:
  postgresqlquery:
    databases:
      - name: "myapp_db"
        dsn: "postgresql://user:pass@localhost:5432/myapp"
    collection_interval: 60s
    slow_query_threshold_ms: 100.0
```

### Full Configuration

```yaml
receivers:
  postgresqlquery:
    # List of databases to monitor
    databases:
      - name: "production_db"
        dsn: "postgresql://user:pass@host:5432/dbname?sslmode=require"
        enabled: true
        max_open_connections: 3
        max_idle_connections: 2
        connection_max_lifetime: 5m
        connection_max_idle_time: 1m
        # Database-specific overrides
        collection_interval: 30s
        slow_query_threshold_ms: 500.0
    
    # Global collection settings
    collection_interval: 60s
    query_timeout: 10s
    slow_query_threshold_ms: 100.0
    max_queries_per_cycle: 100
    max_plans_per_cycle: 20
    plan_collection_threshold_ms: 1000.0
    
    # Feature flags
    enable_plan_regression: true
    enable_ash: true
    ash_sampling_interval: 1s
    enable_extended_metrics: true
    minimal_mode: false
    
    # Safety and security
    max_errors_per_database: 10
    sanitize_pii: true
    
    # Adaptive sampling
    adaptive_sampling:
      enabled: true
      default_rate: 1.0
      max_queries_per_minute: 1000
      max_memory_mb: 100
      rules:
        - name: "always_sample_slow"
          priority: 100
          conditions:
            - attribute: "mean_time_ms"
              operator: "gt"
              value: 1000.0
          sample_rate: 1.0
```

## Metrics

### Query Metrics

| Metric | Description | Unit | Attributes |
|--------|-------------|------|------------|
| `postgresql.query.mean_time` | Average execution time | ms | query_id |
| `postgresql.query.calls` | Number of executions | {calls} | query_id |
| `postgresql.query.total_time` | Total execution time | ms | query_id |
| `postgresql.query.rows` | Total rows returned | {rows} | query_id |

### Wait Event Metrics

| Metric | Description | Unit | Attributes |
|--------|-------------|------|------------|
| `postgresql.wait_event.count` | Wait event occurrences | {events} | wait_event_type, wait_event |
| `postgresql.ash.active_sessions` | Average active sessions | {sessions} | - |
| `postgresql.ash.waiting_sessions` | Average waiting sessions | {sessions} | - |

### Table Metrics

| Metric | Description | Unit | Attributes |
|--------|-------------|------|------------|
| `postgresql.table.seq_scan` | Sequential scans | {scans} | schema, table |
| `postgresql.table.idx_scan` | Index scans | {scans} | schema, table |
| `postgresql.table.n_live_tup` | Live rows | {rows} | schema, table |
| `postgresql.table.n_dead_tup` | Dead rows | {rows} | schema, table |

## Logs

The receiver emits structured logs for:

- **Slow queries** exceeding the configured threshold
- **Plan regressions** when query execution plans change
- **Blocking sessions** from ASH analysis

## Adaptive Sampling

Adaptive sampling reduces data volume while maintaining visibility into important queries:

```yaml
adaptive_sampling:
  rules:
    - name: "always_sample_errors"
      conditions:
        - attribute: "error_count"
          operator: "gt"
          value: 0
      sample_rate: 1.0
    
    - name: "sample_by_duration"
      conditions:
        - attribute: "mean_time_ms"
          operator: "between"
          value: [100, 1000]
      sample_rate: 0.5
```

### Supported Operators

- `eq`, `ne`: Equal, not equal
- `gt`, `lt`, `gte`, `lte`: Comparison operators
- `contains`: String contains
- `regex`: Regular expression match

### Supported Attributes

- `mean_time_ms`: Average query execution time
- `total_time_ms`: Total query execution time
- `calls`: Number of query executions
- `rows`: Rows returned
- `temp_blocks`: Temporary blocks used
- `error_count`: Query error count
- `query_type`: Type of query (SELECT, INSERT, etc.)
- `user_id`: Database user
- `database_id`: Database name
- `application_name`: Application name from connection

## Security Considerations

### Connection Security

- Always use SSL/TLS connections in production
- Use connection strings with `sslmode=require` or `sslmode=verify-full`
- Store credentials securely using environment variables or secret management

### PII Sanitization

When `sanitize_pii: true`, the receiver:
- Replaces string literals with '?'
- Masks potential IDs (numeric values > 4 digits)
- Removes email addresses
- Normalizes whitespace

### Required Permissions

The monitoring user needs:
- `CONNECT` privilege on target databases
- `SELECT` privilege on `pg_stat_statements` view
- `SELECT` privilege on `pg_stat_activity` view
- `SELECT` privilege on system catalogs

Example user setup:
```sql
CREATE USER monitoring_user WITH PASSWORD 'secure_password';
GRANT CONNECT ON DATABASE myapp TO monitoring_user;
GRANT pg_monitor TO monitoring_user;  -- PostgreSQL 10+
```

## Performance Considerations

### Resource Usage

- Each database connection uses ~2-5MB of memory
- ASH sampling with 1s interval adds ~1-2% CPU overhead
- Plan analysis caches up to 10,000 query plans by default

### Optimization Tips

1. **Use connection pooling**: Configure appropriate connection pool sizes
2. **Enable minimal mode**: For high-traffic databases with limited monitoring needs
3. **Adjust sampling intervals**: Balance between visibility and overhead
4. **Configure adaptive sampling**: Reduce data volume while maintaining insights

## Troubleshooting

### Common Issues

1. **Extension not found**
   ```sql
   CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
   ```

2. **Permission denied**
   - Ensure monitoring user has required privileges
   - Check `pg_hba.conf` for connection permissions

3. **High memory usage**
   - Reduce `max_queries_per_cycle`
   - Enable adaptive sampling
   - Increase `collection_interval`

### Debug Logging

Enable debug logging for troubleshooting:
```yaml
service:
  telemetry:
    logs:
      level: debug
```

## Cloud Provider Specifics

### AWS RDS

- Auto-detected when version contains "rds" or "aurora"
- `pg_stat_statements.track` must be set to `ALL`
- Performance Insights provides complementary data

### Azure Database for PostgreSQL

- Auto-detected when version contains "azure"
- Query Store provides additional insights
- Some extensions may not be available

### Google Cloud SQL

- Auto-detected when version contains "cloudsql"
- Query Insights available for additional metrics
- Enable `pg_stat_statements` in database flags

## Integration with Existing Monitoring

The PostgreSQL Query Receiver complements the standard PostgreSQL receiver:

```yaml
receivers:
  # Advanced query and performance monitoring
  postgresqlquery:
    databases:
      - name: "myapp"
        dsn: "postgresql://localhost:5432/myapp"
  
  # Standard PostgreSQL metrics
  postgresql:
    endpoint: localhost:5432
    databases:
      - myapp
```

This provides complete observability:
- Infrastructure metrics (CPU, memory, disk)
- Database metrics (connections, transactions, locks)
- Query performance (slow queries, plan changes)
- Wait event analysis (bottlenecks, blocking)
- Application insights (via ASH sampling)