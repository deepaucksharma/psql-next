# Experimental Features Guide

This guide documents the experimental features available in the Database Intelligence MVP and how to use them.

## Overview

The experimental mode provides advanced capabilities that go beyond the standard OpenTelemetry components:

- **Active Session History (ASH)**: 1-second granularity session monitoring
- **Adaptive Sampling**: Intelligent, query-aware sampling
- **Circuit Breaker**: Automatic database protection
- **Plan Analysis**: Query execution plan tracking (when pg_querylens available)
- **Multi-Database Support**: Unified collection across database fleets

## Quick Start

### 1. Build the Custom Collector

```bash
# Build experimental collector with all custom components
./quickstart.sh --experimental build
```

This will:
- Install the OpenTelemetry Collector Builder
- Compile all custom Go components
- Create a Docker image with experimental features
- Run integration tests

### 2. Start in Experimental Mode

```bash
# Complete setup with experimental components
./quickstart.sh --experimental all

# Or just start if already configured
./quickstart.sh --experimental start
```

### 3. Monitor Experimental Features

```bash
# Check status with experimental metrics
./quickstart.sh --experimental status

# View logs
./quickstart.sh --experimental logs

# Access monitoring dashboard
open http://localhost:3001  # Grafana (admin/admin)
```

## Feature Details

### Active Session History (ASH)

Provides second-by-second visibility into database activity:

```yaml
receivers:
  postgresqlquery:
    ash_sampling:
      enabled: true
      interval: 1s        # Sample every second
      buffer_size: 3600   # Keep 1 hour of samples
```

**What it captures**:
- Active sessions and their wait events
- Blocking relationships
- Query execution context
- Resource consumption patterns

**Use cases**:
- Troubleshoot transient performance issues
- Identify blocking chains
- Analyze wait event patterns

### Adaptive Sampling

Intelligently adjusts sampling rates based on query characteristics:

```yaml
processors:
  adaptivesampler:
    strategies:
      - type: "query_cost"
        high_cost_threshold_ms: 1000
        high_cost_sampling: 100    # Sample all slow queries
        low_cost_sampling: 25      # Sample 25% of fast queries
      
      - type: "error_rate"
        error_threshold: 0.05
        error_sampling: 100        # Sample all errors
        normal_sampling: 50
```

**Benefits**:
- Captures all important queries
- Reduces data volume for routine queries
- Adapts to workload changes

### Circuit Breaker

Protects databases from monitoring overhead:

```yaml
processors:
  circuitbreaker:
    failure_threshold: 5
    success_threshold: 2
    databases:
      default:
        max_error_rate: 0.1
        max_latency_ms: 5000
```

**Protection mechanisms**:
- Opens circuit on repeated failures
- Monitors database response times
- Tracks memory pressure
- Provides automatic backoff

### Multi-Database Federation

Collect from multiple databases with unified configuration:

```yaml
receivers:
  postgresqlquery:
    databases:
      - name: primary
        dsn: "${env:PG_PRIMARY_DSN}"
        tags:
          role: primary
          region: us-east-1
      
      - name: analytics
        dsn: "${env:PG_ANALYTICS_DSN}"
        tags:
          role: analytics
          region: us-west-2
```

## Configuration Examples

### Minimal Experimental Setup

```yaml
# config/experimental-minimal.yaml
receivers:
  postgresqlquery:
    connection:
      dsn: "${env:PG_REPLICA_DSN}"
    collection:
      interval: 60s

processors:
  memory_limiter:
    limit_mib: 1024
  
  circuitbreaker:
    failure_threshold: 5
  
  batch:
    timeout: 30s

exporters:
  otlp/newrelic:
    endpoint: "${env:OTLP_ENDPOINT}"
    headers:
      api-key: "${env:NEW_RELIC_LICENSE_KEY}"

service:
  pipelines:
    logs:
      receivers: [postgresqlquery]
      processors: [memory_limiter, circuitbreaker, batch]
      exporters: [otlp/newrelic]
```

### Full Experimental Setup

See `config/collector-experimental.yaml` for a complete configuration with all features enabled.

## Monitoring Experimental Components

### Key Metrics

```promql
# ASH sampling rate
rate(db_intelligence_ash_samples_total[5m])

# Circuit breaker status (0=closed, 1=open)
db_intelligence_circuitbreaker_open

# Adaptive sampling rate
db_intelligence_adaptivesampler_current_rate

# Plan regression detections
increase(db_intelligence_plan_regressions_detected_total[1h])
```

### Grafana Dashboard

The experimental deployment includes a pre-configured Grafana dashboard:

1. Access Grafana: http://localhost:3001
2. Login: admin/admin
3. Navigate to: Dashboards → Database Intelligence - Experimental

### Debug Endpoints

```bash
# ZPages for tracing
open http://localhost:55680/debug/tracez

# pprof for profiling
go tool pprof http://localhost:6061/debug/pprof/heap
```

## Gradual Adoption

### Phase 1: Circuit Breaker Only

Start with just the circuit breaker for safety:

```yaml
processors:
  - memory_limiter
  - circuitbreaker
  - transform/sanitize_pii
  - probabilistic_sampler  # Keep standard sampler
  - batch
```

### Phase 2: Add Adaptive Sampling

Replace probabilistic with adaptive sampling:

```yaml
processors:
  - memory_limiter
  - circuitbreaker
  - transform/sanitize_pii
  - adaptivesampler  # Smart sampling
  - batch
```

### Phase 3: Enable ASH

Switch to the custom receiver:

```yaml
receivers:
  postgresqlquery:  # Custom receiver
    ash_sampling:
      enabled: true
```

## Troubleshooting

### Custom Collector Won't Build

```bash
# Check Go version (need 1.21+)
go version

# Clear module cache
go clean -modcache

# Rebuild
./quickstart.sh --experimental build
```

### High Memory Usage

```yaml
# Reduce ASH buffer
receivers:
  postgresqlquery:
    ash_sampling:
      buffer_size: 900  # 15 minutes instead of 1 hour

# Tune memory limits
environment:
  GOMEMLIMIT: 1500MiB
  GOGC: 50
```

### Circuit Breaker Too Sensitive

```yaml
processors:
  circuitbreaker:
    failure_threshold: 10  # Increase tolerance
    timeout: 60s          # Longer timeout
```

## Best Practices

1. **Start Small**: Enable one experimental feature at a time
2. **Monitor Closely**: Watch resource usage during rollout
3. **Test First**: Use test databases before production
4. **Have Rollback Plan**: Keep standard configuration ready
5. **Document Changes**: Track which features are enabled

## Future Enhancements

### Coming Soon

- Query plan collection (pending pg_querylens)
- Redis state storage for multi-instance deployment
- MySQL ASH equivalent
- Advanced correlation with APM data

### In Development

- ML-based anomaly detection
- Automated root cause analysis
- Predictive scaling recommendations

## FAQ

**Q: Can I run experimental features in production?**
A: Yes, but start with shadow deployment and gradual rollout.

**Q: What's the performance impact?**
A: Expect 2-3x more CPU and memory usage compared to standard mode.

**Q: Can I mix standard and experimental components?**
A: Yes, you can use any combination that makes sense for your needs.

**Q: How do I contribute?**
A: Test the features and provide feedback via GitHub issues.

## Support

For experimental features:
- GitHub Issues: Feature requests and bug reports
- GitHub Discussions: Questions and community support
- Documentation: This guide and inline code comments

Remember: Experimental features are provided as-is but are intended to become production features based on user feedback and testing.