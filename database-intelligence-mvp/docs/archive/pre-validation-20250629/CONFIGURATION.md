# Detailed Configuration Guide

## Overview

This guide details the OpenTelemetry Collector configuration for Database Intelligence MVP.

## Configuration Structure

```yaml
receivers:   # Data ingestion
processors:  # Data transformation
exporters:   # Data destination
service:     # Pipeline assembly
```

## Receivers Configuration

### PostgreSQL Query Plan Receiver

```yaml
receivers:
  sqlquery/postgresql_plans_safe:
    driver: postgres
    dsn: "${env:PG_REPLICA_DSN}"
    collection_interval: 60s
    timeout: 5s
    max_open_connections: 2
    max_idle_connections: 1
    queries:
      - sql: |
          SET LOCAL statement_timeout = '2000';
          SET LOCAL lock_timeout = '100';
          WITH worst_query AS (
            SELECT 
              queryid, query, mean_exec_time, calls, (mean_exec_time * calls) as total_impact
            FROM pg_stat_statements
            WHERE mean_exec_time > 100 AND calls > 10 AND query NOT LIKE '%pg_%' AND query NOT LIKE '%EXPLAIN%'
            ORDER BY total_impact DESC
            LIMIT 1
          )
          SELECT
            w.queryid::text as query_id, w.query as query_text, w.mean_exec_time as avg_duration_ms, w.calls as execution_count,
            pg_get_json_plan(w.query) as plan
          FROM worst_query w;
```

**Key Decisions**:
*   `LIMIT 1`: Collects only the worst query to minimize impact.
*   `SET LOCAL`: PostgreSQL-specific safety to prevent runaway queries.
*   **Filter Criteria**: Balances relevance (slow) and stability (freqent).

### MySQL Performance Schema Receiver

```yaml
receivers:
  sqlquery/mysql_plans_safe:
    driver: mysql
    dsn: "${env:MYSQL_READONLY_DSN}"
    collection_interval: 60s
    timeout: 5s
    queries:
      - sql: |
          SELECT 
            DIGEST, DIGEST_TEXT, COUNT_STAR as execution_count, AVG_TIMER_WAIT/1000000 as avg_duration_ms,
            SUM_ROWS_EXAMINED/COUNT_STAR as avg_rows_examined,
            JSON_OBJECT('system', 'mysql', 'digest', DIGEST, 'text', LEFT(DIGEST_TEXT, 1000), 'avg_rows', SUM_ROWS_EXAMINED/COUNT_STAR, 'execution_count', COUNT_STAR) as plan_metadata
          FROM performance_schema.events_statements_summary_by_digest
          WHERE SCHEMA_NAME = DATABASE() AND AVG_TIMER_WAIT > 100000000 AND COUNT_STAR > 10
          ORDER BY (AVG_TIMER_WAIT * COUNT_STAR) DESC
          LIMIT 1;
```

**Why No EXPLAIN for MySQL**:
*   No statement-level timeout mechanism.
*   EXPLAIN can acquire metadata locks.
*   Would need to run on primary (dangerous).

### File Log Receiver (Zero Impact)

```yaml
receivers:
  filelog/pg_auto_explain:
    include: 
      - /var/log/postgresql/*.log
    exclude:
      - /var/log/postgresql/*.gz
    start_at: end
    multiline:
      line_start_pattern: '^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}'
      max_log_lines: 1000
      timeout: 5s
    operators:
      - type: regex_parser
        regex: 'duration:\s+(?P<duration>\d+\.\d+)\s+ms.*plan:\s*(?P<plan>\{[\s\S]*?\n\})'
        on_error: send
      - type: json_parser
        parse_from: attributes.plan
        parse_to: body
        on_error: send_quiet
      - type: add_attributes
        attributes:
          db.system: postgresql
          db.source: auto_explain
          db.query.duration_ms: EXPR(attributes.duration)
          correlation.available: "false"
          collection.method: zero_impact
```

## Processors Configuration

### Memory Limiter (Mandatory First)

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
    ballast_size_mib: 64
```

### PII Sanitizer

```yaml
processors:
  transform/sanitize_pii:
    error_mode: ignore
    log_statements:
      - context: log
        statements:
          - replace_all_patterns(attributes, "value", "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b", "[EMAIL]")
          - replace_pattern(body, "\\b\\d{3}-\\d{2}-\\d{4}\\b", "[SSN]")
          - replace_pattern(body, "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b", "[CARD]")
          - replace_pattern(body, "\\b(?:\\+1[\\s-]?)?\\(?\\d{3}\\)?\\?[\\s-]?\\d{3}[\\s-]?\\d{4}\\b", "[PHONE]")
          - replace_pattern(attributes["db.statement"], "'[^']*'", "'?'")
```

### Plan Attribute Extractor

```yaml
processors:
  plan_attribute_extractor:
    timeout_ms: 100
    error_mode: ignore
    postgresql_rules:
      detection_jsonpath: "$[0].Plan"
      extractions:
        db.query.plan.cost: "$[0].Plan['Total Cost']"
        db.query.plan.rows: "$[0].Plan['Plan Rows']"
        db.query.plan.width: "$[0].Plan['Plan Width']"
        db.query.plan.operation: "$[0].Plan['Node Type']"
      derived:
        db.query.plan.has_seq_scan: |
          HAS_SUBSTR(TO_STRING(body), "Seq Scan")
        db.query.plan.has_nested_loop: |
          HAS_SUBSTR(TO_STRING(body), "Nested Loop")
        db.query.plan.depth: |
          JSON_DEPTH(body)
    mysql_rules:
      detection_jsonpath: "$.system"
      extractions:
        db.query.plan.rows: "$.avg_rows"
        db.query.digest: "$.digest"
    hash_config:
      include:
        - db.statement
        - db.query.plan.operation
        - db.query.plan.cost
      output: db.query.plan.hash
```

### Adaptive Sampler

```yaml
processors:
  adaptive_sampler:
    state_storage:
      type: file_storage
      file_storage:
        directory: /var/lib/otel/sampling_state
        sync_interval: 10s
        compaction_interval: 300s
        max_size_mb: 100
    deduplication:
      enabled: true
      cache_size: 10000
      window_seconds: 300
      hash_attribute: db.query.plan.hash
    rules:
      - name: critical_queries
        priority: 100
        sample_rate: 1.0
        conditions:
          - attribute: db.query.avg_duration_ms
            operator: gt
            value: 1000
      - name: missing_indexes  
        priority: 90
        sample_rate: 1.0
        conditions:
          - attribute: db.query.plan.has_seq_scan
            operator: eq
            value: true
          - attribute: db.query.plan.rows
            operator: gt
            value: 10000
      - name: high_frequency
        priority: 50
        sample_rate: 0.01
        conditions:
          - attribute: db.query.calls
            operator: gt
            value: 1000
      - name: default
        priority: 0
        sample_rate: 0.1
```

## Exporter Configuration

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

```yaml
service:
  extensions: 
    - health_check
    - file_storage
    - zpages
  pipelines:
    logs/database_plans:
      receivers: 
        - sqlquery/postgresql_plans_safe
        - sqlquery/mysql_plans_safe
        - filelog/pg_auto_explain
      processors:
        - memory_limiter
        - transform/sanitize_pii
        - plan_attribute_extractor
        - adaptive_sampler
        - batch
      exporters:
        - otlp
  telemetry:
    logs:
      level: info
      encoding: json
      output_paths: ["/var/log/otel/collector.log"]
      error_output_paths: ["stderr"]
    metrics:
      level: detailed
      address: 0.0.0.0:8888
```

## Environment Variables

Required environment variables:

```bash
export NEW_RELIC_LICENSE_KEY="your-license-key"
export OTLP_ENDPOINT="https://otlp.nr-data.net:4317"
export PG_REPLICA_DSN="postgres://newrelic_monitor:password@replica.host:5432/dbname?sslmode=require"
export MYSQL_READONLY_DSN="newrelic_monitor:password@tcp(replica.host:3306)/dbname?tls=true"
```

## Configuration Validation

Validate your configuration using:

```bash
otelcol validate --config=config.yaml
otelcol --config=config.yaml --dry-run
otelcol --config=config.yaml --set=receivers.sqlquery.collection_interval=300s
```
