# OTEL-First Refactoring Plan

## 🎯 Goal: Maximize OTEL, Minimize Custom Code

### Phase 1: Audit & Remove Redundancies

#### 1.1 Replace Custom Receivers with OTEL
```yaml
# REMOVE: receivers/postgresqlquery/
# REPLACE WITH: Standard OTEL receivers

receivers:
  # For metrics
  postgresql:
    endpoint: ${env:PG_HOST}:5432
    username: ${env:PG_USER}
    password: ${env:PG_PASSWORD}
    collection_interval: 10s
    
  # For custom queries (query plans, etc)
  sqlquery/plans:
    driver: postgres
    dsn: ${env:PG_DSN}
    queries:
      - sql: |
          SELECT queryid, 
                 query,
                 mean_exec_time,
                 calls
          FROM pg_stat_statements
          WHERE mean_exec_time > 1000
```

#### 1.2 Remove Redundant Domain Code
- ❌ Remove: `domain/database/` - Use OTEL resource attributes
- ❌ Remove: `domain/telemetry/` - Use OTEL pdata
- ❌ Remove: `application/collection_service.go` - Use OTEL pipelines
- ✅ Keep: `domain/query/` - For query plan analysis (OTEL gap)
- ✅ Keep: `domain/shared/` - For custom processor shared logic

### Phase 2: Refactor Custom Logic into OTEL Processors

#### 2.1 Adaptive Sampler Processor
```go
// processors/adaptivesampler/processor.go
package adaptivesampler

import (
    "go.opentelemetry.io/collector/processor"
    "github.com/database-intelligence-mvp/domain/sampling"
)

type adaptiveSamplerProcessor struct {
    // Use DDD service for complex sampling logic
    samplingService *sampling.AdaptiveSamplingService
}

func (asp *adaptiveSamplerProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
    // Use OTEL interfaces, delegate to domain service
    return asp.samplingService.ApplySampling(md)
}
```

#### 2.2 Circuit Breaker Processor
```go
// processors/circuitbreaker/processor.go
package circuitbreaker

type circuitBreakerProcessor struct {
    // Use DDD for health monitoring logic
    healthService *health.DatabaseHealthService
}

func (cbp *circuitBreakerProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
    // Check health using domain service
    if cbp.healthService.ShouldBlock(md) {
        return pmetric.NewMetrics(), nil // Drop metrics
    }
    return md, nil
}
```

### Phase 3: Simplify Configuration

#### 3.1 Single Collector Configuration
```yaml
# config/collector.yaml - THE ONLY CONFIG FILE NEEDED

# Standard OTEL Receivers
receivers:
  postgresql:
    endpoint: ${env:PG_HOST}:5432
    username: ${env:PG_USER}
    password: ${env:PG_PASSWORD}
    databases: [${env:PG_DATABASE}]
    collection_interval: 10s

  mysql:
    endpoint: ${env:MYSQL_HOST}:3306
    username: ${env:MYSQL_USER}
    password: ${env:MYSQL_PASSWORD}
    database: ${env:MYSQL_DATABASE}
    collection_interval: 10s

  # For data OTEL can't get
  sqlquery/query_plans:
    driver: postgres
    dsn: ${env:PG_DSN}
    collection_interval: 60s
    queries:
      - sql: |
          WITH slow_queries AS (
            SELECT queryid, query, mean_exec_time
            FROM pg_stat_statements 
            WHERE mean_exec_time > 1000
            ORDER BY mean_exec_time DESC
            LIMIT 10
          )
          SELECT 
            sq.queryid,
            sq.query,
            sq.mean_exec_time,
            pg_get_query_plan(sq.query) as plan
          FROM slow_queries sq

# Standard OTEL Processors
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512

  batch:
    timeout: 10s
    send_batch_size: 1000

  resource:
    attributes:
      - key: service.name
        value: database-intelligence
        action: insert

  # Custom processors for OTEL gaps
  database_intelligence/adaptive_sampler:
    backend: redis
    redis_endpoint: ${env:REDIS_ENDPOINT}
    
  database_intelligence/circuit_breaker:
    error_threshold: 0.5
    timeout: 30s

# Standard OTEL Exporters  
exporters:
  otlp:
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}

  prometheus:
    endpoint: 0.0.0.0:8888

# Service configuration
service:
  extensions: [health_check]
  
  pipelines:
    # Standard metrics pipeline
    metrics:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, batch, resource]
      exporters: [otlp, prometheus]
    
    # Advanced pipeline with custom processors
    logs/query_intelligence:
      receivers: [sqlquery/query_plans]
      processors: 
        - memory_limiter
        - database_intelligence/adaptive_sampler
        - database_intelligence/circuit_breaker
        - batch
      exporters: [otlp]
```

#### 3.2 Remove These Config Files
```bash
# Delete redundant configs
rm config/collector-*.yaml  # Keep only collector.yaml
rm config/attribute-mapping.yaml  # Use transform processor
rm configs/*  # Remove duplicate directory
```

### Phase 4: Simplify Documentation

#### 4.1 Consolidate to Essential Docs
```
docs/
├── README.md                    # Quick start guide
├── CONFIGURATION.md            # How to configure
├── CUSTOM_PROCESSORS.md        # Only our custom processors
├── DEPLOYMENT.md               # Docker & K8s
└── TROUBLESHOOTING.md         # Common issues
```

#### 4.2 Remove Redundant Docs
```bash
# Remove overlapping documentation
rm -rf documentation/  # 49 files!
rm -rf strategy/      # Move key points to main docs
rm -rf domain-research/  # Academic, not practical
```

### Phase 5: Update Build System

#### 5.1 Simplified Makefile
```makefile
# Makefile - Focused on custom processors only

.PHONY: build test deploy

# Build custom collector with our processors
build:
	builder --config=ocb-config.yaml

# Test custom processors
test:
	go test ./processors/...

# Deploy
deploy:
	docker-compose up -d

# Development
dev:
	docker-compose -f docker-compose.dev.yaml up
```

#### 5.2 Minimal OCB Config
```yaml
# ocb-config.yaml - Only what we add to OTEL

dist:
  name: database-intelligence
  description: "OTEL + Custom DB Intelligence Processors"
  output_path: ./dist
  otelcol_version: "0.128.0"

extensions:
  - gomod: go.opentelemetry.io/collector/extension/healthcheckextension v0.128.0

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/postgresqlreceiver v0.128.0
  - gomod: go.opentelemetry.io/collector/receiver/mysqlreceiver v0.128.0
  - gomod: go.opentelemetry.io/collector/receiver/sqlqueryreceiver v0.128.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.128.0
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.128.0
  # Our custom processors
  - gomod: github.com/database-intelligence-mvp/processors/adaptivesampler v0.1.0
  - gomod: github.com/database-intelligence-mvp/processors/circuitbreaker v0.1.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.128.0
  - gomod: go.opentelemetry.io/collector/exporter/prometheusexporter v0.128.0
```

### Phase 6: Migration Steps

#### Week 1: Use Pure OTEL
1. Deploy with just standard receivers
2. Validate metrics flow to New Relic
3. Identify actual gaps

#### Week 2: Build Only What's Missing
1. Implement adaptive sampler IF needed
2. Add circuit breaker IF databases struggle
3. Keep everything else OTEL

#### Week 3: Clean Up
1. Remove unused code
2. Consolidate documentation
3. Simplify deployment

## 📊 Before/After Comparison

### Before:
- 15+ configuration files
- 49 documentation files
- Custom receivers duplicating OTEL
- Complex DDD throughout
- Difficult to understand

### After:
- 1 main configuration file
- 5 documentation files
- Standard OTEL receivers
- DDD only for processors
- Clear and simple

## ✅ Success Criteria

1. **90% OTEL Components**: Most functionality from standard OTEL
2. **10% Custom Code**: Only for true gaps
3. **Single Config**: One collector.yaml
4. **Clear Documentation**: 5 files max
5. **Fast Deployment**: < 5 minutes to production

This refactoring plan will dramatically simplify the project while maintaining all valuable functionality.