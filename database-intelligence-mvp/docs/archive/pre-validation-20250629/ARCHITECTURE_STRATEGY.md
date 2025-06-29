# Database Intelligence MVP - Architecture Strategy

## Core Principle: OTEL-First with DDD for Gaps

### 🎯 Architectural Guidelines

1. **Maximize OTEL Components**: Use standard OpenTelemetry receivers, processors, and exporters wherever possible
2. **DDD for Custom Logic**: Apply Domain-Driven Design only when OTEL components don't meet specific needs
3. **Clear Boundaries**: Maintain clean separation between OTEL pipeline and custom domain logic

## 📊 Component Strategy

### ✅ Use OTEL Components For:

#### Receivers (Use Standard OTEL)
- `postgresql` - PostgreSQL metrics collection
- `mysql` - MySQL metrics collection  
- `sqlquery` - Custom SQL queries for metrics
- `hostmetrics` - System metrics
- `prometheus` - Scraping Prometheus endpoints

#### Processors (Use Standard OTEL)
- `batch` - Batching telemetry data
- `memory_limiter` - Preventing OOM
- `resource` - Adding resource attributes
- `attributes` - Manipulating attributes
- `transform` - Data transformation
- `filter` - Filtering telemetry
- `probabilistic_sampler` - Basic sampling

#### Exporters (Use Standard OTEL)
- `otlp` - Send to New Relic/other OTLP endpoints
- `prometheus` - Expose metrics endpoint
- `debug` - Development debugging
- `logging` - Structured logging

### 🔧 Build Custom Components (with DDD) Only For:

#### Gap 1: Advanced Adaptive Sampling
**Why**: OTEL's probabilistic sampler doesn't adapt based on query performance
**Solution**: Custom processor using DDD pattern
```go
// Domain model
type AdaptiveSamplingPolicy struct {
    QueryID         string
    ImportanceScore float64
    SamplingRate    float64
}

// OTEL Processor wrapper
type AdaptiveSamplerProcessor struct {
    // Domain service
    samplingService *domain.AdaptiveSamplingService
}
```

#### Gap 2: Query Plan Intelligence
**Why**: OTEL can't analyze PostgreSQL query plans
**Solution**: Custom processor for plan extraction
```go
// Domain model  
type QueryPlan struct {
    QueryID    string
    PlanJSON   json.RawMessage
    Complexity QueryComplexity
}

// OTEL Processor wrapper
type QueryPlanProcessor struct {
    // Domain service
    planAnalyzer *domain.QueryPlanAnalyzer
}
```

#### Gap 3: Circuit Breaker for Database Protection  
**Why**: OTEL doesn't have database-aware circuit breaking
**Solution**: Custom processor with domain logic
```go
// Domain model
type DatabaseHealth struct {
    DatabaseID   string
    HealthScore  float64
    CircuitState CircuitState
}

// OTEL Processor wrapper
type CircuitBreakerProcessor struct {
    // Domain service
    healthMonitor *domain.DatabaseHealthMonitor
}
```

## 🏗️ Recommended Architecture

```yaml
# OTEL Pipeline Configuration
receivers:
  # Standard OTEL receivers
  postgresql:
    endpoint: ${env:PG_HOST}:${env:PG_PORT}
    username: ${env:PG_USER}
    password: ${env:PG_PASSWORD}
    
  sqlquery:
    # Query for data OTEL receivers can't get
    driver: postgres
    dsn: ${env:PG_DSN}
    queries:
      - sql: "SELECT query plans, etc..."

processors:
  # Standard OTEL processors first
  memory_limiter:
    limit_mib: 512
    
  batch:
    timeout: 10s
    
  # Custom processors only for gaps
  adaptive_sampler:
    # Uses DDD domain service internally
    storage_backend: redis
    
  circuit_breaker:
    # Uses DDD domain model
    error_threshold: 0.5

exporters:
  # Standard OTEL exporters
  otlp:
    endpoint: https://otlp.nr-data.net:4317

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, batch]
      exporters: [otlp]
      
    logs/advanced:
      receivers: [sqlquery]
      processors: [memory_limiter, adaptive_sampler, circuit_breaker, batch]
      exporters: [otlp]
```

## 📁 Simplified Project Structure

```
database-intelligence-mvp/
├── config/
│   └── collector.yaml          # OTEL configuration
├── custom/                     # Custom components only
│   ├── processors/            
│   │   ├── adaptivesampler/   # Gap: Adaptive sampling
│   │   ├── circuitbreaker/    # Gap: DB protection
│   │   └── queryplan/         # Gap: Plan analysis
│   └── domain/                # DDD models for custom logic
│       ├── sampling/
│       ├── health/
│       └── analysis/
├── deploy/
│   ├── docker/
│   └── kubernetes/
└── docs/
    ├── README.md
    ├── CONFIGURATION.md
    └── CUSTOM_COMPONENTS.md
```

## 🚀 Implementation Phases

### Phase 1: Pure OTEL (Week 1)
1. Deploy standard OTEL collectors
2. Configure postgresql & mysql receivers  
3. Use standard processors (batch, memory_limiter)
4. Send to New Relic via OTLP exporter
5. **Deliver value immediately**

### Phase 2: Identify Gaps (Week 2)
1. Monitor what's missing with pure OTEL
2. Document specific gaps
3. Validate need for custom components
4. Design DDD models for gaps only

### Phase 3: Build Custom Components (Week 3-4)
1. Implement adaptive sampler (if needed)
2. Add circuit breaker (if needed)
3. Create query plan analyzer (if needed)
4. Each as a proper OTEL processor

## 🎯 Decision Framework

### When to Use OTEL Components:
- ✅ Component exists in OTEL contrib
- ✅ Meets 80% of requirements
- ✅ Can be configured to meet needs
- ✅ Well-maintained and stable

### When to Build Custom (with DDD):
- ✅ No OTEL component exists
- ✅ OTEL component missing critical features
- ✅ Domain-specific logic required
- ✅ Complex state management needed

### When NOT to Build Custom:
- ❌ OTEL component exists but needs minor config
- ❌ Can achieve goal with processor chain
- ❌ Feature is "nice to have" not critical

## 📋 Migration Plan

### From Current State:
1. **Remove unnecessary DDD code** that duplicates OTEL
2. **Keep DDD patterns** only for custom processors
3. **Simplify configuration** to one main file
4. **Reduce documentation** to essential guides

### Configuration Simplification:
```yaml
# Before: Multiple complex configs
# After: One simple config with clear sections

receivers:
  # All standard OTEL receivers
  
processors:
  # Standard OTEL processors
  # Custom processors clearly marked
  
exporters:
  # Standard OTEL exporters

service:
  pipelines:
    # Clear pipeline definitions
```

## 🔍 Current Codebase Assessment

### Keep (It's OTEL-First):
- ✅ Standard receiver configurations
- ✅ OTLP exporter setup
- ✅ Basic processor chain
- ✅ Health check extensions

### Refactor (Make it Custom Processor):
- 🔄 `receivers/postgresqlquery` → Use `sqlquery` receiver
- 🔄 `domain/*` → Only for custom processor logic
- 🔄 `application/*` → Integrate into processors

### Remove (Duplicates OTEL):
- ❌ Custom metrics collection (use postgresql receiver)
- ❌ Custom batching logic (use batch processor)
- ❌ Custom export logic (use OTLP exporter)

## 📈 Benefits of This Approach

1. **Faster Time to Value**: Start with OTEL, add custom later
2. **Easier Maintenance**: Leverage OTEL updates
3. **Better Compatibility**: Standard OTEL interfaces
4. **Clearer Architecture**: OTEL pipeline + custom processors
5. **Reduced Complexity**: Only build what OTEL can't do

## 🎬 Next Steps

1. **Audit current implementation** against OTEL capabilities
2. **Remove redundant custom code** that OTEL handles
3. **Refactor remaining custom logic** into OTEL processors
4. **Simplify configuration** to single file
5. **Update documentation** to reflect OTEL-first approach

This strategy provides a clear path to a simpler, more maintainable architecture that maximizes OTEL components while preserving the ability to add domain-specific logic where truly needed.