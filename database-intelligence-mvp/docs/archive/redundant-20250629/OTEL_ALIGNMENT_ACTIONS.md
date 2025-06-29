# OTEL Alignment Action Plan

## ✅ Current State Analysis

The `config/collector.yaml` is already well-aligned with OTEL-first principles:
- ✅ Using `sqlquery` receiver (standard OTEL)
- ✅ Using standard processors: `memory_limiter`, `transform`, `resource`, `batch`
- ✅ Using `otlp` exporter for New Relic
- ✅ Proper health checks and extensions
- ✅ Safety-first with timeouts and limits

## 🔧 Immediate Actions

### 1. Add Standard Database Receivers
```yaml
# Add to receivers section for richer metrics
receivers:
  # Standard PostgreSQL metrics
  postgresql:
    endpoint: ${env:PG_HOST}:${env:PG_PORT}
    username: ${env:PG_USER}
    password: ${env:PG_PASSWORD}
    databases:
      - ${env:PG_DATABASE}
    tls:
      insecure: true
    collection_interval: 60s

  # Standard MySQL metrics  
  mysql:
    endpoint: ${env:MYSQL_HOST}:${env:MYSQL_PORT}
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    database: ${env:MYSQL_DATABASE}
    collection_interval: 60s

  # Keep existing sqlquery receivers for custom queries
  sqlquery/postgresql:
    # ... existing configuration for query stats
    
  sqlquery/mysql:
    # ... existing configuration for query stats
```

### 2. Update Pipelines
```yaml
service:
  pipelines:
    # Standard metrics from database receivers
    metrics/databases:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp/newrelic]
    
    # Custom query intelligence (existing)
    logs/database_intelligence:
      receivers: [sqlquery/postgresql, sqlquery/mysql]
      processors: [memory_limiter, transform/sanitize_pii, resource, probabilistic_sampler, batch]
      exporters: [otlp/newrelic]
```

### 3. Remove Custom Code That Duplicates OTEL

#### Components to Remove:
```bash
# These duplicate OTEL functionality
rm -rf receivers/postgresqlquery/  # Use sqlquery receiver
rm -rf application/collection_service.go  # OTEL handles collection
rm -rf domain/database/  # Use OTEL resource attributes
rm -rf domain/telemetry/  # Use OTEL pdata
```

#### Components to Refactor as Processors:
```bash
# These provide unique value - convert to OTEL processors
processors/adaptivesampler/  # Keep but simplify
processors/circuitbreaker/   # Keep but activate
domain/query/                # Keep for plan analysis
```

### 4. Simplify Configuration Files

#### Keep Only These Configs:
```bash
config/
├── collector.yaml           # Main production config
├── collector-dev.yaml       # Development config
└── examples/
    ├── postgresql-only.yaml # Example for PostgreSQL only
    └── mysql-only.yaml      # Example for MySQL only
```

#### Remove These:
```bash
rm config/collector-*.yaml  # Remove all variants
rm -rf configs/            # Remove duplicate directory
```

### 5. Update Custom Processors for True Gaps

#### Adaptive Sampler (Only if OTEL's probabilistic sampler insufficient)
```go
// processors/adaptivesampler/processor.go
package adaptivesampler

import (
    "context"
    "go.opentelemetry.io/collector/pdata/plog"
    "go.opentelemetry.io/collector/processor"
)

type Config struct {
    // Simple config focused on the gap
    HighCostThresholdMs float64 `mapstructure:"high_cost_threshold_ms"`
    ErrorBoostFactor    float64 `mapstructure:"error_boost_factor"`
}

func createLogsProcessor(ctx context.Context, params processor.CreateSettings, cfg component.Config, next consumer.Logs) (processor.Logs, error) {
    // Simple processor that adjusts sampling based on query cost
    return &adaptiveSampler{
        config: cfg.(*Config),
        next:   next,
    }, nil
}
```

#### Circuit Breaker (Only if databases need protection)
```go
// processors/circuitbreaker/processor.go
package circuitbreaker

type Config struct {
    ErrorThreshold   float64 `mapstructure:"error_threshold"`
    BlockDurationSec int     `mapstructure:"block_duration_sec"`
}

func (cb *circuitBreaker) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
    // Simple: if error rate too high, drop logs to protect database
    if cb.errorRate > cb.config.ErrorThreshold {
        return plog.NewLogs(), nil // Drop
    }
    return cb.next.ConsumeLogs(ctx, ld)
}
```

### 6. Deployment Simplification

#### Docker Compose (Simple)
```yaml
version: '3.8'
services:
  collector:
    image: otel/opentelemetry-collector-contrib:latest
    volumes:
      - ./config/collector.yaml:/etc/otelcol/config.yaml
    environment:
      - NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY}
      - PG_HOST=postgres
      - MYSQL_HOST=mysql
    ports:
      - "13133:13133"  # Health
      - "8888:8888"    # Metrics
```

#### Kubernetes (Production)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: collector
        image: otel/opentelemetry-collector-contrib:latest
        volumeMounts:
        - name: config
          mountPath: /etc/otelcol
```

### 7. Documentation Consolidation

#### Final Documentation Structure:
```
README.md                    # Getting started (2 pages)
docs/
├── CONFIGURATION.md        # How to configure (3 pages)
├── DEPLOYMENT.md          # Deploy to Docker/K8s (2 pages)
├── CUSTOM_PROCESSORS.md   # Our 2-3 custom processors (2 pages)
└── TROUBLESHOOTING.md     # Common issues (1 page)
```

Total: ~10 pages instead of 49 files

## 📋 Migration Checklist

### Week 1: OTEL Standard Components
- [ ] Add postgresql receiver
- [ ] Add mysql receiver  
- [ ] Test standard metrics pipeline
- [ ] Verify data in New Relic

### Week 2: Remove Redundancy
- [ ] Remove custom receivers
- [ ] Remove unnecessary domain code
- [ ] Consolidate configurations
- [ ] Update documentation

### Week 3: Custom Processors (If Needed)
- [ ] Evaluate if adaptive sampling needed
- [ ] Evaluate if circuit breaker needed
- [ ] Implement as simple OTEL processors
- [ ] Test with production workload

## 🎯 Success Metrics

1. **Code Reduction**: Remove 70% of custom code
2. **Config Files**: From 15+ to 3 files  
3. **Documentation**: From 49 to 5 files
4. **Deployment Time**: < 5 minutes
5. **Maintenance**: Standard OTEL updates

## 🚀 Final Architecture

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│  PostgreSQL │────▶│                  │────▶│ New Relic   │
├─────────────┤     │   OTEL Collector │     └─────────────┘
│    MySQL    │────▶│                  │
└─────────────┘     │  Standard:       │
                    │  - Receivers      │
                    │  - Processors     │
                    │  - Exporters      │
                    │                  │
                    │  Custom (maybe):  │
                    │  - Adaptive?      │
                    │  - Circuit?       │
                    └──────────────────┘
```

This approach maximizes OTEL components while preserving the ability to add custom processors only where OTEL truly has gaps.