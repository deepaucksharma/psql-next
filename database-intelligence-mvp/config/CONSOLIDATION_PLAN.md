# Configuration Consolidation Plan

## Current State: 15+ Configuration Files 😱

We have too many configuration files that create confusion and maintenance overhead.

## Target State: 3 Configuration Files ✅

### 1. `collector.yaml` (Production)
Main production configuration with:
- Standard OTEL receivers (postgresql, mysql, sqlquery)
- Standard processors (batch, memory_limiter, transform)
- OTLP exporter to New Relic
- Optional custom processors (adaptive sampler, circuit breaker)

### 2. `collector-dev.yaml` (Development)
Development configuration with:
- Same as production but with debug exporter
- Lower collection intervals for testing
- Verbose logging

### 3. `examples/minimal.yaml` (Reference)
Minimal example showing:
- Single PostgreSQL receiver
- Basic processors
- OTLP export

## Files to Remove

```bash
# Remove these redundant configurations
rm config/collector-experimental.yaml      # Experimental features now in main
rm config/collector-ha.yaml               # HA is default behavior
rm config/collector-newrelic-optimized.yaml # Main config is optimized
rm config/collector-nr-test.yaml          # Use collector-dev.yaml
rm config/collector-ohi-compatible.yaml   # OTEL is the standard
rm config/collector-postgresql.yaml       # Merged into main
rm config/collector-unified.yaml          # Main config is unified
rm config/collector-with-postgresql-receiver.yaml # Default in main
rm config/collector-with-verification.yaml # Verification in tests
rm config/collector-test.yaml             # Use collector-dev.yaml
rm config/collector-simple.yaml           # See examples/minimal.yaml
rm config/attribute-mapping.yaml          # Use transform processor
```

## Migration for Each File

### collector-experimental.yaml → collector.yaml
```yaml
# Enable experimental features via environment variables
processors:
  database_intelligence/adaptive_sampler:
    enabled: ${env:ENABLE_ADAPTIVE_SAMPLER:-false}
```

### collector-ha.yaml → collector.yaml
```yaml
# HA is built into OTEL with proper deployment
service:
  extensions: [health_check]  # Health checks enable HA
```

### collector-postgresql.yaml → collector.yaml
```yaml
# PostgreSQL is included by default
receivers:
  postgresql:
    # Configuration already in main
```

## Implementation Steps

1. Create consolidated `collector.yaml` ✓
2. Create `collector-dev.yaml` for development
3. Create `examples/minimal.yaml` for reference
4. Update documentation to reference only these 3 files
5. Archive old configurations in `config/archived/`
6. Update all references in code and docs