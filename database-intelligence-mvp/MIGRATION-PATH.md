# Migration Path: From Standard to Experimental Components

This document provides a step-by-step guide for migrating from the standard OpenTelemetry components (currently in production) to the experimental custom components.

## Overview

The Database Intelligence MVP has two operational modes:

1. **Standard Mode** (Production v1.0.0): Uses only standard OpenTelemetry components
2. **Experimental Mode**: Includes custom Go components with advanced features

## Why Migrate?

### Current Limitations (Standard Mode)

- Basic query metadata collection only
- No query execution plans
- Simple probabilistic sampling (25% fixed)
- No Active Session History (ASH)
- Limited database protection mechanisms
- No automatic query regression detection

### Benefits of Experimental Components

- **ASH Sampling**: 1-second granularity session monitoring
- **Adaptive Sampling**: Intelligent sampling based on query cost and error rates
- **Circuit Breaker**: Automatic protection for struggling databases
- **Plan Collection**: Query execution plan analysis (requires pg_querylens)
- **Multi-Database Support**: Unified collection from multiple databases
- **Cloud Provider Optimization**: AWS RDS, Azure, GCP-specific features

## Migration Prerequisites

### 1. Infrastructure Requirements

```yaml
# Experimental mode requirements
Memory: 1-2GB (vs 512MB for standard)
CPU: 500m-1000m (vs 300m for standard)
Instances: 1 (stateful components require single instance)
Storage: Optional Redis for multi-instance state
```

### 2. Database Requirements

```sql
-- PostgreSQL extensions needed
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS pg_wait_sampling;  -- For ASH

-- Future: pg_querylens for safe EXPLAIN
-- CREATE EXTENSION IF NOT EXISTS pg_querylens;
```

### 3. Build Requirements

```bash
# Install Go 1.21+
# Install OpenTelemetry Collector Builder
go install go.opentelemetry.io/collector/cmd/builder@latest
```

## Migration Steps

### Phase 1: Build Custom Collector (Day 1)

1. **Build the custom binary**:
   ```bash
   cd database-intelligence-mvp
   ./scripts/build-custom-collector.sh --with-docker --with-tests
   ```

2. **Verify the build**:
   ```bash
   # Check binary
   ./dist/db-intelligence-custom --version
   
   # Run integration tests
   ./tests/integration/test-experimental-components.sh
   ```

3. **Test in development environment**:
   ```bash
   # Use experimental configuration
   ./dist/db-intelligence-custom --config=config/collector-experimental.yaml
   ```

### Phase 2: Shadow Deployment (Week 1-2)

Deploy experimental collector alongside production:

1. **Create shadow deployment**:
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: db-intelligence-experimental
     namespace: db-intelligence
   spec:
     replicas: 1  # Single instance for stateful components
     template:
       spec:
         containers:
         - name: collector
           image: db-intelligence-custom:latest
           args: ["--config=/etc/otel/collector-experimental.yaml"]
   ```

2. **Configure for shadow mode**:
   ```yaml
   # In collector-experimental.yaml
   exporters:
     otlp/shadow:
       endpoint: "${env:SHADOW_OTLP_ENDPOINT}"
       headers:
         api-key: "${env:SHADOW_LICENSE_KEY}"
   ```

3. **Monitor both deployments**:
   - Compare metrics between standard and experimental
   - Validate data quality
   - Check resource usage

### Phase 3: Gradual Feature Enablement (Week 3-4)

Enable features incrementally:

1. **Week 3 - Circuit Breaker Only**:
   ```yaml
   processors:
     - memory_limiter
     - circuitbreaker  # Add this
     - transform/sanitize_pii
     - probabilistic_sampler  # Keep standard sampler
     - batch
   ```

2. **Week 4 - Adaptive Sampler**:
   ```yaml
   processors:
     - memory_limiter
     - circuitbreaker
     - transform/sanitize_pii
     - adaptivesampler  # Replace probabilistic
     - batch
   ```

### Phase 4: Full Migration (Week 5-6)

1. **Switch receivers**:
   ```yaml
   receivers:
     postgresqlquery:  # Custom receiver with ASH
       # Full configuration
   ```

2. **Update deployment**:
   ```bash
   # Scale down standard deployment
   kubectl scale deployment db-intelligence-collector --replicas=0
   
   # Promote experimental to primary
   kubectl patch deployment db-intelligence-experimental \
     -p '{"metadata":{"name":"db-intelligence-collector"}}'
   ```

3. **Enable advanced features**:
   - ASH sampling
   - Plan collection (when pg_querylens ready)
   - Multi-database federation

## Rollback Plan

At any phase, you can rollback:

```bash
# Quick rollback to standard
kubectl scale deployment db-intelligence-collector --replicas=3
kubectl scale deployment db-intelligence-experimental --replicas=0

# Or switch configurations
kubectl set image deployment/db-intelligence-collector \
  collector=otel/opentelemetry-collector-contrib:0.88.0
```

## Configuration Comparison

### Standard Configuration
```yaml
# Simple, proven, stateless
receivers:
  sqlquery/postgresql:
    driver: postgres
    collection_interval: 300s
    queries:
      - sql: "SELECT ... FROM pg_stat_statements"

processors:
  - probabilistic_sampler:
      sampling_percentage: 25
```

### Experimental Configuration
```yaml
# Rich features, stateful, requires careful management
receivers:
  postgresqlquery:
    ash_sampling:
      enabled: true
      interval: 1s
    plan_collection:
      enabled: true
      
processors:
  - adaptivesampler:
      strategies:
        - type: "query_cost"
          high_cost_threshold_ms: 1000
```

## Monitoring During Migration

### Key Metrics to Watch

```promql
# Resource usage
rate(container_cpu_usage_seconds_total[5m])
container_memory_working_set_bytes

# Data quality
otelcol_receiver_accepted_log_records_total
otelcol_processor_dropped_log_records_total

# Circuit breaker status
db_intelligence_circuitbreaker_open_total
db_intelligence_circuitbreaker_blocked_total

# Sampling rates
db_intelligence_adaptivesampler_current_rate
```

### Alerts to Configure

```yaml
- alert: ExperimentalCollectorHighMemory
  expr: container_memory_working_set_bytes > 1.5e9
  annotations:
    summary: "Experimental collector using >1.5GB memory"

- alert: CircuitBreakerOpen
  expr: db_intelligence_circuitbreaker_open > 0
  annotations:
    summary: "Circuit breaker opened for database protection"
```

## Validation Checklist

### Before Migration
- [ ] Custom collector built and tested
- [ ] Integration tests passing
- [ ] Shadow deployment validated
- [ ] Rollback plan tested
- [ ] Team trained on new components

### During Migration
- [ ] Metrics parity confirmed
- [ ] No increase in database load
- [ ] Resource usage within limits
- [ ] No data loss or corruption
- [ ] Circuit breaker functioning

### After Migration
- [ ] All experimental features enabled
- [ ] Standard deployment decommissioned
- [ ] Documentation updated
- [ ] Monitoring adjusted
- [ ] Performance baselined

## Common Issues and Solutions

### Issue: High Memory Usage
```yaml
# Solution: Tune memory limits
processors:
  memory_limiter:
    limit_mib: 2048
    spike_limit_mib: 512
```

### Issue: Circuit Breaker Too Sensitive
```yaml
# Solution: Adjust thresholds
processors:
  circuitbreaker:
    failure_threshold: 10  # Increase from 5
    success_threshold: 3   # Increase from 2
```

### Issue: State Coordination Needed
```yaml
# Solution: Add Redis for state
processors:
  adaptivesampler:
    state_storage:
      type: redis
      endpoint: redis:6379
```

## Support and Troubleshooting

1. **Logs**: Check collector logs for component initialization
2. **Metrics**: Monitor `/metrics` endpoint for component health
3. **Debug**: Use `zpages` extension for detailed tracing
4. **Rollback**: Always possible to revert to standard mode

## Timeline Summary

- **Day 1**: Build and test custom collector
- **Week 1-2**: Shadow deployment and validation
- **Week 3-4**: Gradual feature enablement
- **Week 5-6**: Full migration and optimization
- **Week 7+**: Advanced features and tuning

Remember: The migration is reversible at any stage. Take it slow, monitor carefully, and prioritize database safety above all else.