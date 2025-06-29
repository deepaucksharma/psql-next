# Migration Guide: Custom Components to Standard OTEL

## Overview
This guide shows how to migrate from custom components to standard OTEL components, following the OTEL-first architecture.

## 1. PostgreSQL Query Receiver → SQL Query Receiver

### Before (Custom Receiver)
```yaml
receivers:
  postgresqlquery:
    dsn: "postgres://user:pass@host:5432/db"
    collection_interval: 300s
    query_stats:
      enabled: true
    plan_collection:
      enabled: true
```

### After (Standard OTEL)
```yaml
receivers:
  # Use standard PostgreSQL receiver for metrics
  postgresql:
    endpoint: host:5432
    username: user
    password: pass
    databases: [db]
    collection_interval: 60s
    
  # Use sqlquery receiver for custom queries
  sqlquery/stats:
    driver: postgres
    dsn: "postgres://user:pass@host:5432/db"
    collection_interval: 300s
    queries:
      - sql: |
          SELECT queryid, query, mean_exec_time, calls
          FROM pg_stat_statements
          WHERE mean_exec_time > 100
        metrics:
          - metric_name: db.query.duration
            value_column: mean_exec_time
            attribute_columns: [queryid, query]
```

## 2. Custom Processors to Standard OTEL

### PII Sanitization: Custom → Transform Processor
```yaml
# Before (Custom)
processors:
  pii_sanitizer:
    patterns:
      - email
      - credit_card
      
# After (OTEL Transform)
processors:
  transform/sanitize:
    metric_statements:
      - context: datapoint
        statements:
          - replace_pattern(attributes["query"], "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b", "[EMAIL]")
```

### Basic Sampling: Custom → Probabilistic Sampler
```yaml
# Before (Custom)
processors:
  custom_sampler:
    sample_rate: 0.25
    
# After (OTEL Standard)
processors:
  probabilistic_sampler:
    sampling_percentage: 25.0
```

## 3. When to Keep Custom Processors

Only keep custom processors for true OTEL gaps:

### Adaptive Sampler (Keep if needed)
```yaml
processors:
  # Custom - no OTEL equivalent for query-based adaptive sampling
  database_intelligence/adaptive_sampler:
    high_cost_threshold_ms: 1000
    error_boost_factor: 2.0
    backend: memory  # or redis for distributed
```

### Circuit Breaker (Keep if needed)
```yaml
processors:
  # Custom - no OTEL equivalent for database protection
  database_intelligence/circuit_breaker:
    error_threshold: 0.5
    reset_timeout: 30s
    backend: memory  # or redis for distributed
```

## 4. Configuration Consolidation

### Remove These Files
```bash
# Too many variants - keep only one
rm config/collector-experimental.yaml
rm config/collector-ha.yaml
rm config/collector-newrelic-optimized.yaml
rm config/collector-nr-test.yaml
rm config/collector-ohi-compatible.yaml
rm config/collector-postgresql.yaml
rm config/collector-unified.yaml
rm config/collector-with-postgresql-receiver.yaml
rm config/collector-with-verification.yaml
rm config/attribute-mapping.yaml  # Use transform processor
```

### Keep Only
```
config/
├── collector.yaml              # Production config
├── collector-dev.yaml          # Development with debug
└── examples/
    ├── minimal.yaml           # Minimal setup
    └── advanced.yaml          # With custom processors
```

## 5. Code Removal Plan

### Phase 1: Remove Duplicate Functionality
```bash
# These duplicate OTEL components
rm -rf receivers/postgresqlquery/      # Use sqlquery receiver
rm -rf exporters/customnewrelic/       # Use otlp exporter
rm -rf processors/planattributeextractor/  # Use transform processor
```

### Phase 2: Refactor Custom Processors
```bash
# Keep but simplify
processors/adaptivesampler/     # Simplify to just the adaptive logic
processors/circuitbreaker/      # Simplify to just circuit breaking
```

### Phase 3: Remove Unused Domain Code
```bash
# Remove if not used by custom processors
rm -rf domain/database/         # Use OTEL attributes
rm -rf domain/telemetry/        # Use OTEL pdata
rm -rf application/             # Use OTEL pipelines
```

## 6. Testing Migration

### Step 1: Run Both Configs
```bash
# Start with existing config
./otelcol --config=config/collector-old.yaml

# In parallel, start new config
./otelcol --config=config/collector.yaml --metrics-addr=:8889
```

### Step 2: Compare Metrics
```bash
# Compare outputs
curl http://localhost:8888/metrics > old-metrics.txt
curl http://localhost:8889/metrics > new-metrics.txt
diff old-metrics.txt new-metrics.txt
```

### Step 3: Gradual Migration
```yaml
# Use feature flags
receivers:
  postgresql:
    enabled: ${env:USE_STANDARD_RECEIVER:-false}
    
service:
  pipelines:
    metrics/standard:
      receivers: [postgresql]  # New
    metrics/custom:
      receivers: [postgresqlquery]  # Old
```

## 7. Benefits After Migration

1. **Maintenance**: Automatic updates with OTEL releases
2. **Compatibility**: Works with any OTEL backend
3. **Simplicity**: Less custom code to maintain
4. **Documentation**: Can use standard OTEL docs
5. **Community**: Benefit from OTEL community improvements

## 8. Final Configuration Example

```yaml
# Minimal OTEL-first configuration
receivers:
  postgresql:
    endpoint: ${env:PG_ENDPOINT}
    username: ${env:PG_USER}
    password: ${env:PG_PASSWORD}
    
processors:
  batch:
    timeout: 10s
    
exporters:
  otlp:
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      api-key: ${env:API_KEY}
      
service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlp]
```

This is 90% simpler than the custom implementation!