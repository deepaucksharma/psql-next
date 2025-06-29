# Configuration Files

This directory contains the active OTEL collector configuration files for the Database Intelligence MVP.

## Active Configurations

### 1. collector.yaml (Production)
Full production configuration with all features:
- Standard OTEL receivers (PostgreSQL, MySQL, SQLQuery)
- All processors (standard + custom)
- PII sanitization
- Adaptive sampling
- Circuit breaker protection
- Verification and quality checks
- New Relic and Prometheus export

**Use this for:** Production deployments with full monitoring capabilities

### 2. collector-simplified.yaml (Development)
Simplified configuration with essential features:
- PostgreSQL monitoring only
- Basic processors (memory limiter, batch, transform)
- Custom processors disabled by default
- New Relic and Prometheus export

**Use this for:** Development, testing, and getting started quickly

### 3. collector-minimal.yaml (Testing)
Minimal configuration for basic connectivity:
- PostgreSQL receiver only
- Memory limiter and batch processors
- Debug and Prometheus exporters only
- No custom processors

**Use this for:** Testing database connectivity and basic metric collection

## Configuration Guidelines

### Environment Variables
All configurations use environment variables for sensitive data:
```bash
# Required
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=monitor_user
export POSTGRES_PASSWORD=secure_password
export POSTGRES_DATABASE=production
export NEW_RELIC_LICENSE_KEY=your_key_here

# Optional
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=monitor_user
export MYSQL_PASSWORD=secure_password
export MYSQL_DATABASE=production
```

### Choosing a Configuration

1. **Starting out?** Use `collector-minimal.yaml` to verify connectivity
2. **Development?** Use `collector-simplified.yaml` for standard monitoring
3. **Production?** Use `collector.yaml` with all features enabled

### Customization

To enable custom processors in simplified configuration:
```yaml
processors:
  # Uncomment to enable
  database_intelligence/adaptivesampler:
    min_sampling_rate: 0.1
    max_sampling_rate: 1.0
    
service:
  pipelines:
    metrics:
      processors: [memory_limiter, database_intelligence/adaptivesampler, batch]
```

## Validation

Before using any configuration:
```bash
# Validate syntax
./dist/otelcol-db-intelligence validate --config=config/collector.yaml

# Test with dry run
./dist/otelcol-db-intelligence --config=config/collector.yaml --dry-run
```

## Migration from Old Configs

If you were using archived configurations, migrate as follows:
- `collector-experimental.yaml` → Use `collector.yaml`
- `collector-test.yaml` → Use `collector-minimal.yaml`
- `collector-dev.yaml` → Use `collector-simplified.yaml`
- Any other archived config → Start with `collector-simplified.yaml`

## Examples

See `/deploy/examples/` for complete deployment examples using these configurations.