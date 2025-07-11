# Configuration Guide

## Overview

Complete configuration reference for the Database Intelligence OpenTelemetry Collector with PostgreSQL, MySQL, and New Relic integration.

## Quick Configuration

### Basic PostgreSQL
```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: ${POSTGRES_PASSWORD}
    databases:
      - postgres
      - testdb
    collection_interval: 10s

processors:
  batch:
    timeout: 10s
    send_batch_size: 1024

exporters:
  debug:
    verbosity: detailed
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [debug, otlp]
```

### Basic MySQL
```yaml
receivers:
  mysql:
    endpoint: localhost:3306
    username: root
    password: ${MYSQL_PASSWORD}
    collection_interval: 10s

processors:
  batch:
    timeout: 10s

exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [mysql]
      processors: [batch]
      exporters: [otlp]
```

## Advanced Configuration

### Complete PostgreSQL with Custom Metrics
```yaml
receivers:
  # Standard PostgreSQL metrics
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    databases: 
      - postgres
      - ${POSTGRES_DB}
    collection_interval: 10s
    
  # Slow queries from pg_stat_statements
  sqlquery/slow_queries:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} sslmode=disable"
    collection_interval: 15s
    queries:
      - sql: |
          SELECT 
            queryid::text as query_id,
            query as query_text,
            datname as database_name,
            calls as execution_count,
            mean_exec_time as avg_elapsed_time_ms,
            shared_blks_read as avg_disk_reads,
            shared_blks_written as avg_disk_writes,
            CASE 
              WHEN query LIKE 'SELECT%' THEN 'SELECT'
              WHEN query LIKE 'INSERT%' THEN 'INSERT'
              WHEN query LIKE 'UPDATE%' THEN 'UPDATE'
              WHEN query LIKE 'DELETE%' THEN 'DELETE'
              ELSE 'OTHER'
            END as statement_type,
            'public' as schema_name
          FROM pg_stat_statements pss
          JOIN pg_database pd ON pd.oid = pss.dbid
          WHERE mean_exec_time > 100
        metrics:
          - metric_name: postgres.slow_queries.count
            value_column: execution_count
            value_type: int
            attributes: [query_id, query_text, database_name, statement_type, schema_name]
          - metric_name: postgres.slow_queries.elapsed_time
            value_column: avg_elapsed_time_ms
            value_type: double
            unit: ms
            attributes: [query_id, query_text, database_name, statement_type, schema_name]
          - metric_name: postgres.slow_queries.disk_reads
            value_column: avg_disk_reads
            value_type: double
            attributes: [query_id, database_name]
          - metric_name: postgres.slow_queries.disk_writes
            value_column: avg_disk_writes
            value_type: double
            attributes: [query_id, database_name]

  # Wait events
  sqlquery/wait_events:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} sslmode=disable"
    collection_interval: 10s
    queries:
      - sql: |
          SELECT 
            COALESCE(wait_event, 'CPU') as wait_event_name,
            COALESCE(wait_event_type, 'CPU') as wait_category,
            COUNT(*) as count,
            datname as database_name
          FROM pg_stat_activity 
          WHERE state = 'active'
          GROUP BY wait_event, wait_event_type, datname
        metrics:
          - metric_name: postgres.wait_events
            value_column: count
            value_type: int
            attributes: [wait_event_name, wait_category, database_name]

  # Blocking sessions
  sqlquery/blocking_sessions:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} sslmode=disable"
    collection_interval: 10s
    queries:
      - sql: |
          SELECT
            blocked.pid as blocked_pid,
            blocked.query as blocked_query,
            COALESCE(blocked.queryid::text, '0') as blocked_query_id,
            blocked.datname as database_name,
            blocking.pid as blocking_pid,
            blocking.query as blocking_query,
            COALESCE(blocking.queryid::text, '0') as blocking_query_id,
            blocking.datname as blocking_database
          FROM pg_stat_activity blocked
          JOIN pg_stat_activity blocking 
            ON blocking.pid = ANY(pg_blocking_pids(blocked.pid))
          WHERE blocked.state = 'active'
        metrics:
          - metric_name: postgres.blocking_sessions
            value_column: blocked_pid
            value_type: int
            attributes: [blocked_pid, blocked_query, blocked_query_id, database_name, blocking_pid, blocking_query, blocking_query_id, blocking_database]

processors:
  # Resource attributes
  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: insert
      - key: environment
        value: ${ENVIRONMENT:production}
        action: insert

  # Transform OHI attributes to OTEL semantic conventions  
  transform:
    metric_statements:
      - context: datapoint
        statements:
          # Slow queries transformations
          - set(attributes["db.name"], attributes["database_name"]) where attributes["database_name"] != nil
          - set(attributes["db.statement"], attributes["query_text"]) where attributes["query_text"] != nil
          - set(attributes["db.postgresql.query_id"], attributes["query_id"]) where attributes["query_id"] != nil
          - set(attributes["db.operation"], attributes["statement_type"]) where attributes["statement_type"] != nil
          - set(attributes["db.schema"], attributes["schema_name"]) where attributes["schema_name"] != nil
          - set(attributes["db.system"], "postgresql")
          
          # Wait events transformations
          - set(attributes["db.wait_event.name"], attributes["wait_event_name"]) where attributes["wait_event_name"] != nil
          - set(attributes["db.wait_event.category"], attributes["wait_category"]) where attributes["wait_category"] != nil
          
          # Blocking sessions transformations
          - set(attributes["db.blocking.blocked_pid"], attributes["blocked_pid"]) where attributes["blocked_pid"] != nil
          - set(attributes["db.blocking.blocking_pid"], attributes["blocking_pid"]) where attributes["blocking_pid"] != nil
          - set(attributes["db.blocking.blocked_query"], attributes["blocked_query"]) where attributes["blocked_query"] != nil
          - set(attributes["db.blocking.blocking_query"], attributes["blocking_query"]) where attributes["blocking_query"] != nil
          
          # Clean up original attributes
          - delete_key(attributes, "database_name")
          - delete_key(attributes, "query_text")  
          - delete_key(attributes, "query_id")
          - delete_key(attributes, "statement_type")
          - delete_key(attributes, "schema_name")
          - delete_key(attributes, "wait_event_name")
          - delete_key(attributes, "wait_category")

  # Memory protection
  memory_limiter:
    limit_mib: 512
    spike_limit_mib: 128

  # Efficient batching
  batch:
    timeout: 10s
    send_batch_size: 1024
    send_batch_max_size: 2048

exporters:
  # Debug for local testing
  debug:
    verbosity: detailed
    
  # New Relic export
  otlp:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    compression: gzip
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

extensions:
  # Health check endpoint
  health_check:
    endpoint: 0.0.0.0:13133

service:
  extensions: [health_check]
  
  pipelines:
    metrics:
      receivers: [postgresql, sqlquery/slow_queries, sqlquery/wait_events, sqlquery/blocking_sessions]
      processors: [memory_limiter, resource, transform, batch]
      exporters: [debug, otlp]
      
  telemetry:
    logs:
      level: info
    metrics:
      address: 0.0.0.0:8888
```

## Environment Variables

### Required Variables
```bash
# Database credentials
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_password
POSTGRES_DB=testdb

MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=your_password
MYSQL_DB=testdb

# New Relic integration
NEW_RELIC_LICENSE_KEY=your_license_key
NEW_RELIC_ACCOUNT_ID=your_account_id

# Optional
ENVIRONMENT=production
```

### .env File
```bash
# Database Intelligence - Environment Configuration

# PostgreSQL Configuration
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=testdb

# MySQL Configuration  
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=mysql
MYSQL_DB=testdb

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your_license_key_here
NEW_RELIC_USER_KEY=your_user_key_here
NEW_RELIC_ACCOUNT_ID=your_account_id

# Optional Settings
ENVIRONMENT=development
LOG_LEVEL=info
```

## SSL/TLS Configuration

### PostgreSQL SSL
```yaml
receivers:
  postgresql:
    endpoint: postgres.example.com:5432
    username: postgres
    password: ${POSTGRES_PASSWORD}
    ssl_mode: require
    ssl_cert: /certs/client.crt
    ssl_key: /certs/client.key
    ssl_ca: /certs/ca.crt
```

### MySQL SSL
```yaml
receivers:
  mysql:
    endpoint: mysql.example.com:3306
    username: root
    password: ${MYSQL_PASSWORD}
    tls_config:
      insecure: false
      cert_file: /certs/client.crt
      key_file: /certs/client.key
      ca_file: /certs/ca.crt
```

## Custom Processors Configuration

### Adaptive Sampler
```yaml
processors:
  adaptivesampler:
    default_sample_rate: 0.1
    rules:
      - name: high_volume_queries
        condition: 'attributes["db.operation"] == "SELECT"'
        sample_rate: 0.05
        priority: 1
      - name: error_queries  
        condition: 'attributes["error"] != nil'
        sample_rate: 1.0
        priority: 10
    cache_size: 10000
```

### Circuit Breaker
```yaml
processors:
  circuitbreaker:
    failure_threshold: 5
    success_threshold: 3
    timeout: 60s
    max_requests: 10
    interval: 30s
```

### Cost Control
```yaml
processors:
  costcontrol:
    monthly_budget: 100.0
    currency: USD
    data_plus_pricing: true
    cardinality_limits:
      critical: 10000
      warning: 8000
    actions:
      - threshold: 0.8
        action: reduce_sampling
      - threshold: 0.9
        action: drop_attributes
      - threshold: 0.95
        action: emergency_mode
```

### Verification
```yaml
processors:
  verification:
    pii_detection:
      enabled: true
      patterns:
        - email
        - ssn
        - credit_card
        - phone
      action: redact
    data_quality:
      required_fields: [db.name, db.system]
      null_tolerance: 0.05
    cardinality_management:
      max_unique_values: 10000
      sampling_rate: 0.1
```

## Performance Tuning

### Memory Optimization
```yaml
processors:
  memory_limiter:
    limit_mib: 1024        # Adjust based on available memory
    spike_limit_mib: 256   # 25% of limit
    check_interval: 1s

  batch:
    timeout: 5s            # Faster batching for high volume
    send_batch_size: 2048  # Larger batches
    send_batch_max_size: 4096
```

### Collection Intervals
```yaml
receivers:
  postgresql:
    collection_interval: 10s  # Standard metrics
    
  sqlquery/slow_queries:
    collection_interval: 30s  # Expensive queries less frequent
    
  sqlquery/wait_events:
    collection_interval: 5s   # Wait events more frequent
```

### Export Optimization
```yaml
exporters:
  otlp:
    endpoint: otlp.nr-data.net:4317
    compression: gzip
    timeout: 30s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 60s
      max_elapsed_time: 300s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
```

## Multi-Database Configuration

### Multiple PostgreSQL Instances
```yaml
receivers:
  postgresql/primary:
    endpoint: pg-primary:5432
    username: postgres
    password: ${POSTGRES_PASSWORD}
    databases: [production]
    collection_interval: 10s
    
  postgresql/replica:
    endpoint: pg-replica:5432
    username: postgres
    password: ${POSTGRES_PASSWORD}
    databases: [production]
    collection_interval: 30s  # Less frequent for replicas

processors:
  resource/primary:
    attributes:
      - key: postgresql.instance.role
        value: primary
        action: insert
      - key: postgresql.instance.name
        value: pg-primary
        action: insert
        
  resource/replica:
    attributes:
      - key: postgresql.instance.role
        value: replica
        action: insert
      - key: postgresql.instance.name
        value: pg-replica
        action: insert

service:
  pipelines:
    metrics/primary:
      receivers: [postgresql/primary]
      processors: [resource/primary, batch]
      exporters: [otlp]
      
    metrics/replica:
      receivers: [postgresql/replica]
      processors: [resource/replica, batch]
      exporters: [otlp]
```

## Troubleshooting Configuration

### Debug Configuration
```yaml
exporters:
  debug:
    verbosity: detailed
    
  file:
    path: ./metrics.json
    
service:
  pipelines:
    metrics:
      exporters: [debug, file, otlp]
      
  telemetry:
    logs:
      level: debug
      output_paths: [stdout, ./collector.log]
```

### Health Monitoring
```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    check_collector_pipeline:
      enabled: true
      interval: 5s
      exporter_failure_threshold: 5
      
  pprof:
    endpoint: 0.0.0.0:1777
    
  zpages:
    endpoint: 0.0.0.0:55679
```

This configuration guide provides production-ready settings for comprehensive database monitoring with the Database Intelligence OpenTelemetry Collector.