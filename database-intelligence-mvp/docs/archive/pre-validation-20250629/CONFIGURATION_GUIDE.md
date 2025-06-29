# Configuration Guide

## Overview

We've simplified our configuration approach to just 3 main configuration files following OTEL-first principles.

## Configuration Files

### 1. Production Configuration (`config/collector.yaml`)

The main production configuration that:
- Uses standard OTEL receivers for PostgreSQL and MySQL
- Implements PII sanitization with transform processor
- Includes optional custom processors for adaptive sampling and circuit breaking
- Exports to New Relic via OTLP

**When to use**: Production deployments

### 2. Development Configuration (`config/collector-dev.yaml`)

Development-friendly configuration that:
- Enables debug output and file export
- Uses faster collection intervals (10s vs 60s)
- Includes profiling endpoint
- Higher sampling rates for testing
- Logs to both console and files

**When to use**: Local development and testing

### 3. Minimal Example (`config/examples/minimal.yaml`)

Simplest possible configuration showing:
- Single PostgreSQL receiver
- Basic batch processor
- OTLP export to New Relic

**When to use**: As a starting point for new deployments

## Key Configuration Sections

### Receivers (Standard OTEL)

```yaml
receivers:
  # Database metrics
  postgresql:
    endpoint: ${env:PG_HOST}:${env:PG_PORT}
    username: ${env:PG_USER}
    password: ${env:PG_PASSWORD}
    
  # Custom queries
  sqlquery/postgresql:
    driver: postgres
    dsn: ${env:PG_DSN}
    queries:
      - sql: "SELECT * FROM pg_stat_statements"
```

### Processors (Mostly Standard)

```yaml
processors:
  # Standard OTEL processors
  batch:
    timeout: 10s
  
  memory_limiter:
    limit_mib: 512
    
  transform/sanitize:
    metric_statements:
      - context: datapoint
        statements:
          - replace_pattern(attributes["query"], "emails", "[EMAIL]")
  
  # Custom only for gaps
  database_intelligence/adaptive_sampler:
    enabled: ${env:ENABLE_ADAPTIVE:-false}
```

### Exporters (Standard OTEL)

```yaml
exporters:
  otlp:
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      api-key: ${env:API_KEY}
```

## Environment Variables

### Required
- `NEW_RELIC_LICENSE_KEY` - Your New Relic license key
- `PG_HOST`, `PG_PORT`, `PG_USER`, `PG_PASSWORD` - PostgreSQL connection

### Optional
- `ENABLE_ADAPTIVE_SAMPLER` - Enable adaptive sampling (default: false)
- `ENABLE_CIRCUIT_BREAKER` - Enable circuit breaker (default: false)
- `DEPLOYMENT_ENV` - Environment name (default: production)

## Migration from Old Configs

If you're using one of the old configuration files:

1. **collector-experimental.yaml** → Use `collector.yaml` with `ENABLE_ADAPTIVE_SAMPLER=true`
2. **collector-ha.yaml** → Use `collector.yaml` (HA is built-in with proper deployment)
3. **collector-postgresql.yaml** → Use `collector.yaml` (includes PostgreSQL by default)
4. **collector-simple.yaml** → Use `examples/minimal.yaml`
5. **collector-test.yaml** → Use `collector-dev.yaml`

## Best Practices

1. **Start Simple**: Begin with `examples/minimal.yaml` and add components as needed
2. **Use Environment Variables**: Don't hardcode credentials
3. **Enable Features Gradually**: Start with standard components, add custom processors only if needed
4. **Monitor Performance**: Use the metrics endpoint to monitor collector health
5. **Test Locally**: Use `collector-dev.yaml` to test changes before production

## Troubleshooting

### No Metrics Appearing
1. Check receiver configuration and database connectivity
2. Verify credentials via environment variables
3. Check collector logs for errors

### High Memory Usage
1. Adjust `memory_limiter` processor settings
2. Reduce batch size
3. Increase collection intervals

### PII in Logs
1. Ensure `transform/sanitize` processor is in the pipeline
2. Add more sanitization patterns as needed

## Example Deployment

```bash
# Production
export NEW_RELIC_LICENSE_KEY=your-key
export PG_HOST=prod-db.example.com
export PG_USER=monitor
export PG_PASSWORD=secure-password

./otelcol --config=config/collector.yaml

# Development
export PG_HOST=localhost
./otelcol --config=config/collector-dev.yaml
```