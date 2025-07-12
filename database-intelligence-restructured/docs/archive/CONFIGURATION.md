# Configuration Reference

Complete reference for configuring Database Intelligence with OpenTelemetry.

## 🏗️ Configuration Architecture

```yaml
# High-level structure
receivers:      # How to collect data
  postgresql: {}
  mysql: {}
  sqlquery: {}

processors:     # How to transform data
  resource: {}
  batch: {}
  transform: {}

exporters:      # Where to send data
  otlp: {}
  prometheus: {}

service:        # How to connect components
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [resource, batch]
      exporters: [otlp]
```

## 📊 Receivers Configuration

### PostgreSQL Receiver

```yaml
receivers:
  postgresql:
    # Required: Database connection
    endpoint: "${DB_ENDPOINT}"  # postgresql://user:pass@host:port/db
    username: "${DB_USERNAME}"
    password: "${DB_PASSWORD}"
    
    # Optional: Connection settings
    databases: ["*"]            # Monitor all databases
    collection_interval: 30s    # How often to collect
    transport: tcp              # tcp or unix socket
    
    # Optional: Metric selection
    metrics:
      postgresql.connections.active:
        enabled: true
      postgresql.database.size:
        enabled: true
      postgresql.commits:
        enabled: true
```

**Required Database Permissions**:
```sql
-- Minimum permissions for PostgreSQL monitoring
GRANT CONNECT ON DATABASE your_db TO otel_monitor;
GRANT USAGE ON SCHEMA public TO otel_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO otel_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA information_schema TO otel_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO otel_monitor;
```

### MySQL Receiver

```yaml
receivers:
  mysql:
    # Required: Database connection
    endpoint: "${DB_ENDPOINT}"  # mysql://user:pass@host:port/db
    username: "${DB_USERNAME}"
    password: "${DB_PASSWORD}"
    
    # Optional: Connection settings
    collection_interval: 30s
    transport: tcp
    
    # Optional: Metric selection
    metrics:
      mysql.connections:
        enabled: true
      mysql.buffer_pool.pages:
        enabled: true
```

### SQL Query Receiver (Custom Metrics)

```yaml
receivers:
  sqlquery:
    driver: postgres  # or mysql
    datasource: "${DB_ENDPOINT}"
    collection_interval: 60s
    
    queries:
      # Example: Table sizes
      - sql: |
          SELECT 
            schemaname,
            tablename,
            pg_total_relation_size(schemaname||'.'||tablename) as size_bytes
          FROM pg_tables 
          WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
          ORDER BY size_bytes DESC
          LIMIT 20
        metrics:
          - metric_name: custom.table.size
            value_column: size_bytes
            value_type: gauge
            unit: By
            attribute_columns:
              - schemaname
              - tablename
```

## ⚙️ Processors Configuration

### Resource Processor (Required)

```yaml
processors:
  resource:
    attributes:
      - key: service.name
        value: "${SERVICE_NAME}"
        action: upsert
      - key: deployment.environment  
        value: "${ENVIRONMENT}"
        action: upsert
      - key: db.system
        value: "postgresql"  # or mysql
        action: insert
```

### Memory Limiter (Recommended)

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512        # Total memory limit
    spike_limit_mib: 128  # Spike protection
```

### Batch Processor (Required)

```yaml
processors:
  batch:
    timeout: 10s           # Max time to wait
    send_batch_size: 1024  # Batch size
    send_batch_max_size: 2048  # Max batch size
```

## 📤 Exporters Configuration

### OTLP Exporter (New Relic)

```yaml
exporters:
  otlp:
    endpoint: "${NEW_RELIC_OTLP_ENDPOINT}"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"
    compression: gzip
    timeout: 30s
    
    # Retry configuration
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
```

## 🔧 Service Configuration

### Basic Pipeline

```yaml
service:
  extensions: [health_check]
  
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp]
```

## 🌍 Environment Variables

| Variable | Description | Required | Example |
|----------|-------------|----------|---------|
| `NEW_RELIC_LICENSE_KEY` | New Relic license key | Yes | `abc123def456...` |
| `NEW_RELIC_OTLP_ENDPOINT` | OTLP endpoint URL | Yes | `https://otlp.nr-data.net:4318` |
| `DB_ENDPOINT` | Database connection string | Yes | `postgresql://user:pass@host:5432/db` |
| `SERVICE_NAME` | Service identifier | Yes | `prod-postgres-01` |
| `ENVIRONMENT` | Environment name | Yes | `production` |

## 🔗 Configuration Examples

Ready-to-use configurations:
- [`config-only-base.yaml`](../configs/examples/config-only-base.yaml) - PostgreSQL
- [`config-only-mysql.yaml`](../configs/examples/config-only-mysql.yaml) - MySQL
- [`config-only-working.yaml`](../configs/examples/config-only-working.yaml) - Multi-database