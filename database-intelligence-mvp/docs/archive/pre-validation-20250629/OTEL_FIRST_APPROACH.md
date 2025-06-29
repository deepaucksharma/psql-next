# OTEL-First Architecture

## Overview

We've adopted an OTEL-first architecture that maximizes the use of standard OpenTelemetry components and only implements custom code where OTEL has true gaps.

## Key Changes

### 1. Standard Receivers Replace Custom Code

**Before**: Custom `postgresqlquery` receiver with 1000+ lines of code
**After**: Standard OTEL receivers that are maintained by the community

```yaml
receivers:
  # Standard PostgreSQL metrics
  postgresql:
    endpoint: localhost:5432
    username: monitor
    password: pass
    
  # Custom queries via sqlquery receiver  
  sqlquery/stats:
    driver: postgres
    dsn: "postgres://..."
    queries:
      - sql: "SELECT * FROM pg_stat_statements"
```

### 2. Transform Processor for PII Sanitization

**Before**: Custom PII sanitizer processor
**After**: Standard transform processor with regex

```yaml
processors:
  transform/sanitize:
    metric_statements:
      - context: datapoint
        statements:
          - replace_pattern(attributes["query"], "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b", "[EMAIL]")
```

### 3. Custom Processors Only for Gaps

We only keep two custom processors that address real OTEL gaps:

1. **Adaptive Sampler**: Query performance-based sampling (no OTEL equivalent)
2. **Circuit Breaker**: Database protection from monitoring overhead (no OTEL equivalent)

### 4. Simplified Configuration

From 15+ config files to just 3:
- `collector.yaml` - Production configuration
- `collector-dev.yaml` - Development with debug output
- `examples/` - Example configurations

## Benefits

1. **90% Less Custom Code**: Most functionality from standard OTEL
2. **Automatic Updates**: Benefit from OTEL community improvements
3. **Better Documentation**: Can use standard OTEL docs
4. **Easier Maintenance**: Less custom code to maintain
5. **Broader Compatibility**: Works with any OTEL backend

## Migration Path

See [MIGRATION_TO_OTEL.md](../MIGRATION_TO_OTEL.md) for detailed migration instructions.