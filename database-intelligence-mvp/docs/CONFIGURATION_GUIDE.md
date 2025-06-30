# Configuration Guide

This guide provides correct configuration syntax and examples for the Database Intelligence Collector.

## Table of Contents
1. [Environment Variables](#environment-variables)
2. [Required Processors](#required-processors)
3. [Memory Configuration](#memory-configuration)
4. [SQL Query Receivers](#sql-query-receivers)
5. [Complete Examples](#complete-examples)

## Environment Variables

### Correct Syntax
All environment variables must use the `${env:VAR_NAME:-default}` syntax:

```yaml
# ✅ CORRECT
endpoint: ${env:POSTGRES_HOST:-localhost}:${env:POSTGRES_PORT:-5432}
username: ${env:POSTGRES_USER:-postgres}
password: ${env:POSTGRES_PASSWORD:-postgres}

# ❌ INCORRECT (old syntax)
endpoint: ${POSTGRES_HOST:localhost}:${POSTGRES_PORT:5432}
```

### Common Environment Variables
```yaml
# Database connections
${env:POSTGRES_HOST:-localhost}
${env:POSTGRES_PORT:-5432}
${env:POSTGRES_USER:-postgres}
${env:POSTGRES_PASSWORD:-postgres}
${env:POSTGRES_DB:-postgres}

${env:MYSQL_HOST:-localhost}
${env:MYSQL_PORT:-3306}
${env:MYSQL_USER:-root}
${env:MYSQL_PASSWORD:-mysql}
${env:MYSQL_DB:-mysql}

# New Relic
${env:NEW_RELIC_LICENSE_KEY}
${env:OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}

# Collector metadata
${env:HOSTNAME}
${env:ENVIRONMENT:-production}
```

## Required Processors

### Resource Processor (CRITICAL)
The resource processor with `collector.name = 'otelcol'` is **required** for dashboard queries to work:

```yaml
processors:
  resource:
    attributes:
      - key: collector.name
        value: otelcol
        action: upsert
      - key: collector.instance.id
        value: ${env:HOSTNAME}
        action: upsert
      - key: deployment.environment
        value: ${env:ENVIRONMENT:-production}
        action: upsert
```

### Memory Limiter
Use MiB values, not percentages:

```yaml
processors:
  memory_limiter:
    check_interval: 2s
    limit_mib: 1024      # ✅ CORRECT
    spike_limit_mib: 256 # ✅ CORRECT
    
    # ❌ INCORRECT (deprecated)
    # limit_percentage: 75
    # spike_limit_percentage: 20
```

## SQL Query Receivers

### PostgreSQL Query Receiver
Must include `logs` or `metrics` configuration:

```yaml
receivers:
  sqlquery/postgresql:
    driver: postgres
    datasource: "host=${env:POSTGRES_HOST:-localhost} port=${env:POSTGRES_PORT:-5432} user=${env:POSTGRES_USER:-postgres} password=${env:POSTGRES_PASSWORD:-postgres} dbname=${env:POSTGRES_DB:-postgres} sslmode=disable"
    collection_interval: 300s
    queries:
      - sql: |
          SELECT
            queryid::text as query_id,
            query as query_text,
            round(mean_exec_time::numeric, 2) as avg_duration_ms,
            calls as execution_count,
            round(total_exec_time::numeric, 2) as total_duration_ms,
            current_database() as database_name
          FROM pg_stat_statements
          WHERE mean_exec_time > 50
          ORDER BY mean_exec_time DESC
          LIMIT 10
        logs:  # ✅ REQUIRED
          - body_column: query_text
            attributes:
              query_id: query_id
              avg_duration_ms: avg_duration_ms
              execution_count: execution_count
              total_duration_ms: total_duration_ms
              database_name: database_name
```

### MySQL Query Receiver
```yaml
receivers:
  sqlquery/mysql:
    driver: mysql
    datasource: "${env:MYSQL_USER:-root}:${env:MYSQL_PASSWORD:-mysql}@tcp(${env:MYSQL_HOST:-localhost}:${env:MYSQL_PORT:-3306})/${env:MYSQL_DB:-mysql}"
    collection_interval: 300s
    queries:
      - sql: |
          SELECT
            DIGEST as query_id,
            DIGEST_TEXT as query_text,
            ROUND(AVG_TIMER_WAIT/1000000, 2) as avg_duration_ms,
            COUNT_STAR as execution_count,
            ROUND((AVG_TIMER_WAIT * COUNT_STAR)/1000000, 2) as total_duration_ms,
            SCHEMA_NAME as database_name
          FROM performance_schema.events_statements_summary_by_digest
          WHERE SCHEMA_NAME IS NOT NULL
          ORDER BY AVG_TIMER_WAIT DESC
          LIMIT 10
        logs:  # ✅ REQUIRED
          - body_column: query_text
            attributes:
              query_id: query_id
              avg_duration_ms: avg_duration_ms
              execution_count: execution_count
              total_duration_ms: total_duration_ms
              database_name: database_name
```

## Complete Examples

### Minimal Production Configuration
```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  zpages:
    endpoint: 0.0.0.0:55679

receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST:-localhost}:${env:POSTGRES_PORT:-5432}
    username: ${env:POSTGRES_USER:-postgres}
    password: ${env:POSTGRES_PASSWORD:-postgres}
    databases:
      - ${env:POSTGRES_DB:-postgres}
    collection_interval: 60s
    tls:
      insecure: true

  mysql:
    endpoint: ${env:MYSQL_HOST:-localhost}:${env:MYSQL_PORT:-3306}
    username: ${env:MYSQL_USER:-root}
    password: ${env:MYSQL_PASSWORD:-mysql}
    database: ${env:MYSQL_DB:-mysql}
    collection_interval: 60s

processors:
  memory_limiter:
    check_interval: 2s
    limit_mib: 1024
    spike_limit_mib: 256

  resource:
    attributes:
      - key: collector.name
        value: otelcol
        action: upsert

  batch:
    timeout: 30s
    send_batch_size: 100

exporters:
  otlp/newrelic:
    endpoint: ${env:OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
    compression: gzip

service:
  extensions: [health_check, zpages]
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp/newrelic]
  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: 0.0.0.0:8888
```

### With Query Logs and Custom Processors
```yaml
# ... (extensions and receivers as above) ...

processors:
  memory_limiter:
    check_interval: 2s
    limit_mib: 1024
    spike_limit_mib: 256

  resource:
    attributes:
      - key: collector.name
        value: otelcol
        action: upsert

  transform/metrics:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          - set(unit, "1") where unit == ""

  transform/logs:
    error_mode: ignore
    log_statements:
      - context: log
        statements:
          - set(attributes["avg_duration_ms"], Double(attributes["avg_duration_ms"]))
          - set(attributes["execution_count"], Int(attributes["execution_count"]))

  transform/sanitize_pii:
    error_mode: ignore
    log_statements:
      - context: log
        statements:
          - replace_all_patterns(attributes["query_text"], "'[^']*'", "'[REDACTED]'")

  # Custom processors (if built)
  adaptive_sampler:
    in_memory_only: true
    default_sampling_rate: 0.1
    rules:
      - name: slow_queries
        conditions:
          - attribute: avg_duration_ms
            operator: gt
            value: 1000
        sample_rate: 1.0

  circuit_breaker:
    failure_threshold: 10
    open_state_timeout: 30s

  plan_attribute_extractor:
    enabled: true
    safe_mode: true

  verification:
    enabled: true
    pii_detection:
      enabled: true

  batch:
    timeout: 30s

service:
  extensions: [health_check, zpages]
  pipelines:
    metrics/databases:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, resource, transform/metrics, batch]
      exporters: [otlp/newrelic]
      
    logs/queries:
      receivers: [sqlquery/postgresql, sqlquery/mysql]
      processors: [memory_limiter, resource, transform/logs, transform/sanitize_pii, adaptive_sampler, circuit_breaker, plan_attribute_extractor, verification, batch]
      exporters: [otlp/newrelic]
```

## Deprecated Features

### Do NOT Use
1. **memory_ballast extension** - Use memory_limiter processor instead
2. **Percentage-based memory limits** - Use MiB values
3. **Old environment variable syntax** - Use ${env:} prefix
4. **SQL query receiver without logs/metrics** - Always specify output format

## Validation

Always validate your configuration:
```bash
./dist/database-intelligence-collector validate --config=your-config.yaml
```

## Troubleshooting

### Common Errors

1. **"unknown type: health_check"**
   - Health check extension not built into collector
   - Use only available extensions

2. **"invalid keys: attributes"**
   - SQL query receiver needs logs or metrics section
   - See examples above

3. **Environment variable not expanding**
   - Use ${env:VAR:-default} syntax
   - Check variable is exported

4. **No data in dashboards**
   - Ensure collector.name = 'otelcol' is set
   - Check resource processor is in pipeline