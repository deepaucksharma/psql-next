# Configuration Guide - Actual Implementation

## Overview

This guide documents the ACTUAL configuration used in the MVP, not planned features. All examples are tested and working.

## Configuration Structure

```yaml
receivers:   # Data ingestion (sqlquery, filelog)
processors:  # Data transformation (standard OTEL components)
exporters:   # Data destination (OTLP to New Relic)
service:     # Pipeline assembly
```

## Receivers (What We Actually Use)

### PostgreSQL Query Receiver

```yaml
receivers:
  sqlquery/postgresql_plans_safe:
    driver: postgres
    dsn: "${env:PG_REPLICA_DSN}"
    
    # Safety Controls - These are critical
    collection_interval: 60s     # How often to query
    timeout: 5s                  # Query timeout
    max_open_connections: 2      # Connection pool size
    max_idle_connections: 1      # Idle connections
    
    queries:
      - sql: |
          -- MANDATORY SAFETY TIMEOUTS
          SET LOCAL statement_timeout = '2000';
          SET LOCAL lock_timeout = '100';
          
          WITH worst_query AS (
            SELECT 
              queryid,
              query,
              mean_exec_time,
              calls,
              (mean_exec_time * calls) as impact_score
            FROM pg_stat_statements
            WHERE 
              mean_exec_time > 100      -- Only slow queries
              AND calls > 10             -- Must be frequent
              AND query NOT LIKE '%pg_%' -- Skip system queries
            ORDER BY impact_score DESC
            LIMIT 1  -- ONE query per cycle for safety
          )
          SELECT
            w.queryid::text as query_id,
            w.query as query_text,
            w.mean_exec_time as avg_duration_ms,
            w.calls as execution_count,
            -- IMPORTANT: This returns STATIC data, not real plans
            CASE 
              WHEN w.query IS NOT NULL THEN
                '{"Plan": {"Node Type": "Placeholder", "Total Cost": 0}}'::json
              ELSE NULL
            END as plan_json,
            current_database() as database_name
          FROM worst_query w;
```

**Reality Check**:
- We collect query metadata, NOT execution plans
- The `plan_json` field contains placeholder data
- `pg_get_json_plan()` function doesn't exist

### File Log Receiver (PostgreSQL auto_explain)

```yaml
receivers:
  filelog/pg_auto_explain:
    include: 
      - /var/log/postgresql/postgresql-*.log
    exclude:
      - /var/log/postgresql/*.gz
    
    start_at: end              # Don't read old logs
    max_log_size: 10MiB       # Prevent huge files
    max_concurrent_files: 10  # Resource limit
    
    multiline:
      line_start_pattern: '^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}'
    
    operators:
      - type: regex_parser
        regex: 'duration:\s+(?P<duration>\d+\.\d+)\s+ms.*plan:\s*(?P<plan>\{.*\})'
```

This actually works if you have `auto_explain` configured in PostgreSQL.

## Processors (Standard Components Only)

### Memory Limiter (ALWAYS FIRST)

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512          # Hard limit
    spike_limit_mib: 128    # Soft limit
```

### PII Sanitization

```yaml
processors:
  transform/sanitize_pii:
    error_mode: ignore
    log_statements:
      - context: log
        statements:
          # Email addresses
          - replace_all_patterns(attributes, "value", 
              "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b", 
              "[EMAIL]")
          
          # SQL literals - This is what actually runs
          - replace_pattern(attributes["query_text"], "'[^']*'", "'?'")
          
          # Numbers that might be IDs
          - replace_pattern(attributes["query_text"], 
              "\\bWHERE\\s+\\w+\\s*=\\s*\\d+", 
              "WHERE column = ?")
```

### Sampling (NOT Adaptive)

```yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 10.0  # Sample 10% of data
    hash_seed: 22             # For consistent sampling
```

**Reality**: We use standard probabilistic sampling, not the advanced adaptive sampler mentioned in docs.

### Batching

```yaml
processors:
  batch:
    timeout: 10s
    send_batch_size: 100      # Send when we have 100 items
    send_batch_max_size: 500  # Never exceed 500 items
```

## Exporters

### OTLP to New Relic

```yaml
exporters:
  otlp:
    endpoint: "${env:OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}"
    headers:
      api-key: "${env:NEW_RELIC_LICENSE_KEY}"
    
    compression: gzip
    
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    
    sending_queue:
      enabled: true
      num_consumers: 4
      queue_size: 100
```

## Service Configuration

### Pipeline Assembly

```yaml
service:
  pipelines:
    logs/database_plans:
      receivers:
        - sqlquery/postgresql_plans_safe
        - filelog/pg_auto_explain
      processors:
        - memory_limiter        # ALWAYS FIRST
        - transform/sanitize_pii
        - probabilistic_sampler # 10% sampling
        - batch
      exporters:
        - otlp
```

### Extensions

```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  
  file_storage:
    directory: /var/lib/otel/storage
    # Used for receiver checkpoints, NOT processor state
```

## Environment Variables

Required environment variables:

```bash
# Database connection (read-replica only!)
export PG_REPLICA_DSN="postgresql://readonly_user:password@replica.host:5432/dbname?sslmode=require"

# New Relic
export NEW_RELIC_LICENSE_KEY="your-license-key"
export OTLP_ENDPOINT="https://otlp.nr-data.net:4317"

# Optional
export DEPLOYMENT_ENV="production"
```

## Common Configuration Mistakes

### 1. Trying to Use Custom Processors

```yaml
# THIS DOESN'T WORK - These processors aren't built into the collector
processors:
  adaptivesampler:    # ❌ Not available
  circuitbreaker:     # ❌ Not available
  planattributeextractor: # ❌ Not available
```

### 2. Expecting Real Query Plans

The collector returns static JSON for plans. Real plan collection requires:
- Safe EXPLAIN execution (not implemented)
- Timeout handling for EXPLAIN (complex)
- Permission to run EXPLAIN (security risk)

### 3. Multi-Instance Deployment

```yaml
# This causes problems due to checkpoint management
replicas: 2  # ❌ Not recommended
```

Use `replicas: 1` unless you've implemented proper state coordination.

### 4. Pointing at Primary Database

```yaml
# NEVER DO THIS
dsn: "postgresql://user:pass@primary.db:5432/prod"  # ❌ DANGER
```

Always use read replicas.

## Performance Tuning

### For High-Volume Databases

```yaml
# Reduce collection frequency
collection_interval: 300s  # 5 minutes instead of 1

# Lower sampling rate
probabilistic_sampler:
  sampling_percentage: 1.0  # 1% instead of 10%

# Smaller batches
batch:
  send_batch_size: 50
  timeout: 5s
```

### For Low-Memory Environments

```yaml
memory_limiter:
  limit_mib: 256        # Reduce from 512
  spike_limit_mib: 64   # Reduce from 128

# Reduce connection pool
sqlquery:
  max_open_connections: 1
  max_idle_connections: 0
```

## Monitoring Configuration Health

Check these collector metrics:

```
# Memory usage
otelcol_processor_memory_limiter_memory_used_bytes

# Export success
otelcol_exporter_sent_log_records
otelcol_exporter_send_failed_log_records

# Queue depth
otelcol_exporter_queue_size
```

## Next Steps

1. Start with the default configuration
2. Monitor resource usage for a week
3. Adjust sampling and intervals based on:
   - Data volume in New Relic
   - Collector resource usage
   - Database connection impact

Remember: The goal is sustainable long-term monitoring, not capturing every query.