# Configuration Migration Guide

This guide helps you migrate from old configuration syntax to the new format.

## Automated Fixes Applied

1. **Environment Variables**: Changed from `${VAR:default}` to `${env:VAR:-default}`
2. **Memory Limiter**: Changed from percentage to MiB values
3. **Deprecated Extensions**: Removed memory_ballast references

## Manual Fixes Required

### 1. Add Resource Processor

All configurations must include a resource processor with collector.name:

```yaml
processors:
  resource:
    attributes:
      - key: collector.name
        value: otelcol
        action: upsert
```

### 2. Fix SQL Query Receivers

All sqlquery receivers must have logs or metrics configuration:

```yaml
sqlquery/postgresql:
  queries:
    - sql: "SELECT ..."
      logs:
        - body_column: query_text
          attributes:
            query_id: query_id
            avg_duration_ms: avg_duration_ms
```

### 3. Update Service Pipelines

Ensure processors are in correct order:

```yaml
service:
  pipelines:
    metrics:
      processors: [memory_limiter, resource, transform/metrics, batch]
```

## Files Requiring Manual Review

