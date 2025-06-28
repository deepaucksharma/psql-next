# Detailed Configuration Guide

## Collector Configuration Structure

### Overview

The configuration follows a standard OTEL Collector pattern with four main sections:

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
    # Connection Configuration
    driver: postgres
    dsn: "${env:PG_REPLICA_DSN}"
    
    # Safety Controls
    collection_interval: 60s     # Conservative for MVP
    timeout: 5s                  # Connection timeout
    max_idle_time: 60s          # Close idle connections
    max_lifetime: 300s          # Rotate connections
    
    # The Query - This is Your Core IP
    queries:
      - sql: |
          -- SAFETY FIRST: These timeouts are mandatory
          SET LOCAL statement_timeout = '2000';  -- 2 seconds max
          SET LOCAL lock_timeout = '100';        -- 100ms for locks
          
          -- Single query focus for safety
          WITH worst_query AS (
            SELECT 
              queryid,
              query,
              mean_exec_time,
              calls,
              -- Calculate total impact
              (mean_exec_time * calls) as total_impact
            FROM pg_stat_statements
            WHERE 
              mean_exec_time > 100      -- Only slow queries
              AND calls > 10             -- Somewhat frequent
              AND query NOT LIKE '%pg_%' -- Skip system queries
              AND query NOT LIKE '%EXPLAIN%' -- Avoid recursion
            ORDER BY total_impact DESC
            LIMIT 1  -- Critical: Only one query per cycle
          )
          SELECT
            w.queryid::text as query_id,
            w.query as query_text,
            w.mean_exec_time as avg_duration_ms,
            w.calls as execution_count,
            -- This is where the magic happens
            pg_get_json_plan(w.query) as plan
          FROM worst_query w;
```

**Key Configuration Decisions**:
- `LIMIT 1`: We consciously collect only the worst query to minimize impact
- `SET LOCAL`: PostgreSQL-specific safety that prevents runaway queries
- **Filter Criteria**: Balance between relevance (slow) and stability (frequent)

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
            DIGEST,
            DIGEST_TEXT,
            COUNT_STAR as execution_count,
            AVG_TIMER_WAIT/1000000 as avg_duration_ms,
            SUM_ROWS_EXAMINED/COUNT_STAR as avg_rows_examined,
            -- MVP: Metadata only, no EXPLAIN
            JSON_OBJECT(
              'system', 'mysql',
              'digest', DIGEST,
              'text', LEFT(DIGEST_TEXT, 1000),
              'avg_rows', SUM_ROWS_EXAMINED/COUNT_STAR,
              'execution_count', COUNT_STAR
            ) as plan_metadata
          FROM performance_schema.events_statements_summary_by_digest
          WHERE 
            SCHEMA_NAME = DATABASE()
            AND AVG_TIMER_WAIT > 100000000
            AND COUNT_STAR > 10
          ORDER BY (AVG_TIMER_WAIT * COUNT_STAR) DESC
          LIMIT 1;
```

**Why No EXPLAIN for MySQL**:
- No statement-level timeout mechanism
- EXPLAIN can acquire metadata locks
- Would need to run on primary (dangerous)

### File Log Receiver (Zero Impact)

```yaml
receivers:
  filelog/pg_auto_explain:
    # File Discovery
    include: 
      - /var/log/postgresql/*.log
      - /var/log/postgresql/*/*.log  # Dated subdirectories
    exclude:
      - /var/log/postgresql/*.gz     # Skip compressed
    
    # Start from end (don't process history)
    start_at: end
    
    # Critical: Handle multi-line plans
    multiline:
      line_start_pattern: '^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}'
      max_log_lines: 1000  # Prevent memory explosion
      timeout: 5s          # Force flush if incomplete
    
    # Parsing Pipeline
    operators:
      # Stage 1: Extract plan from log line
      - type: regex_parser
        id: plan_extractor
        regex: 'duration:\s+(?P<duration>\d+\.\d+)\s+ms\s+plan:\s*(?P<plan>\{[\s\S]*?\n\})'
        on_error: send  # Don't drop on parse failure
        
      # Stage 2: Parse JSON (if successful)
      - type: json_parser
        id: json_parser
        parse_from: attributes.plan
        parse_to: body
        on_error: send_quiet
        
      # Stage 3: Add metadata
      - type: add_attributes
        id: metadata_adder
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
    # Check interval - how often to measure
    check_interval: 1s
    
    # Hard limit - start dropping data
    limit_mib: 512
    
    # Soft limit - start applying backpressure
    spike_limit_mib: 128
    
    # Calculate memory holistically
    ballast_size_mib: 64  # Reserved headroom
```

### PII Sanitizer

```yaml
processors:
  transform/sanitize_pii:
    error_mode: ignore  # Don't fail on sanitization errors
    
    log_statements:
      - context: log
        statements:
          # Email - Most common PII
          - replace_all_patterns(
              attributes,
              "value",
              "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b",
              "[EMAIL]"
            )
          
          # SSN - US Specific
          - replace_pattern(body, "\\b\\d{3}-\\d{2}-\\d{4}\\b", "[SSN]")
          
          # Credit Card - Multiple formats
          - replace_pattern(
              body,
              "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b",
              "[CARD]"
            )
          
          # Phone - North American
          - replace_pattern(
              body,
              "\\b(?:\\+1[\\s-]?)?\\(?\\d{3}\\)?[\\s-]?\\d{3}[\\s-]?\\d{4}\\b",
              "[PHONE]"
            )
          
          # SQL Literals - Critical for plans
          - replace_pattern(
              attributes["db.statement"],
              "'[^']*'",
              "'?'"
            )
```

### Plan Attribute Extractor

```yaml
processors:
  plan_attribute_extractor:
    # Timeout per record
    timeout_ms: 100
    
    # Error handling
    error_mode: ignore  # Continue on extraction failure
    
    # PostgreSQL Extraction Rules
    postgresql_rules:
      # Detection: Does it look like a PG plan?
      detection_jsonpath: "$[0].Plan"
      
      # Extractions (JSONPath)
      extractions:
        db.query.plan.cost: "$[0].Plan['Total Cost']"
        db.query.plan.rows: "$[0].Plan['Plan Rows']"
        db.query.plan.width: "$[0].Plan['Plan Width']"
        db.query.plan.operation: "$[0].Plan['Node Type']"
        
      # Derived attributes
      derived:
        db.query.plan.has_seq_scan: |
          HAS_SUBSTR(TO_STRING(body), "Seq Scan")
        db.query.plan.has_nested_loop: |
          HAS_SUBSTR(TO_STRING(body), "Nested Loop")
        db.query.plan.depth: |
          JSON_DEPTH(body)
    
    # MySQL Extraction Rules  
    mysql_rules:
      # Different structure for MySQL metadata
      detection_jsonpath: "$.system"
      
      extractions:
        db.query.plan.rows: "$.avg_rows"
        db.query.digest: "$.digest"
        
    # Hash Generation
    hash_config:
      # Attributes to include in hash
      include:
        - db.statement
        - db.query.plan.operation
        - db.query.plan.cost
      
      # Output attribute
      output: db.query.plan.hash
```

### Adaptive Sampler

```yaml
processors:
  adaptive_sampler:
    # State Management - Critical Configuration
    state_storage:
      type: file_storage
      
      # File storage configuration
      file_storage:
        directory: /var/lib/otel/sampling_state
        sync_interval: 10s
        compaction_interval: 300s
        max_size_mb: 100
    
    # Deduplication Configuration
    deduplication:
      enabled: true
      cache_size: 10000        # Max unique hashes to track
      window_seconds: 300      # 5-minute window
      
    # Sampling Rules (Evaluated in Order)
    rules:
      # Rule 1: Always sample critical queries
      - name: critical_queries
        priority: 100
        sample_rate: 1.0
        conditions:
          - attribute: db.query.avg_duration_ms
            operator: gt
            value: 1000
      
      # Rule 2: Sample queries with missing indexes
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
            
      # Rule 3: Sample high-frequency queries less
      - name: high_frequency
        priority: 50
        sample_rate: 0.01
        conditions:
          - attribute: db.query.calls
            operator: gt
            value: 1000
            
      # Default rule (must be last)
      - name: default
        priority: 0
        sample_rate: 0.1
```

## Exporter Configuration

```yaml
exporters:
  otlp:
    # Endpoint discovery
    endpoint: "${env:OTLP_ENDPOINT:-https://otlp.nr-data.net:4317}"
    
    # Authentication
    headers:
      api-key: "${env:NEW_RELIC_LICENSE_KEY}"
    
    # Compression (reduces bandwidth by ~70%)
    compression: gzip
    
    # Reliability settings
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
      
    # Security
    tls:
      insecure: false
      insecure_skip_verify: false
      
    # Performance
    sending_queue:
      enabled: true
      num_consumers: 4
      queue_size: 100
```

## Service Configuration

```yaml
service:
  # Extensions to load
  extensions: 
    - health_check
    - file_storage
    - zpages  # Optional: debugging endpoints
  
  # Pipeline definition
  pipelines:
    logs/database_plans:
      receivers: 
        - sqlquery/postgresql_plans_safe
        - sqlquery/mysql_plans_safe
        - filelog/pg_auto_explain
      processors:
        - memory_limiter          # Always first
        - transform/sanitize_pii  # Security second
        - plan_attribute_extractor
        - adaptive_sampler
        - batch                   # Always last
      exporters:
        - otlp
  
  # Telemetry (self-monitoring)
  telemetry:
    logs:
      level: info
      development: false
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
# New Relic
export NEW_RELIC_LICENSE_KEY="your-license-key"
export OTLP_ENDPOINT="https://otlp.nr-data.net:4317"

# PostgreSQL
export PG_REPLICA_DSN="postgres://newrelic_monitor:password@replica.host:5432/dbname?sslmode=require"

# MySQL  
export MYSQL_READONLY_DSN="newrelic_monitor:password@tcp(replica.host:3306)/dbname?tls=true"
```

## Configuration Validation

Before deploying, validate your configuration:

```bash
# Syntax validation
otelcol validate --config=config.yaml

# Dry run (starts and immediately stops)
otelcol --config=config.yaml --dry-run

# Test with minimal data
otelcol --config=config.yaml --set=receivers.sqlquery.collection_interval=300s
```