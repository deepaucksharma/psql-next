# Integration Patterns Analysis

## Critical Integration Problems

### 1. Tight OpenTelemetry Coupling
```go
// Everything depends on OTel internals
import (
    "go.opentelemetry.io/collector/pdata/pmetric"
    "go.opentelemetry.io/collector/component"
)

// Can't swap implementations
type processor struct {
    next component.MetricsConsumer  // Locked to OTel
}
```
**Impact**: Version lock-in, can't test without OTel, upgrade nightmares

### 2. No Database Abstraction
```go
// SQL scattered everywhere
func (r *postgresqlReceiver) collect() {
    rows, err := r.db.Query("SELECT * FROM pg_stat_database")
    // Direct SQL in receiver code!
}

func (r *mysqlReceiver) collect() {
    rows, err := r.db.Query("SHOW STATUS")  
    // Different SQL, same pattern
}
```
**Impact**: SQL injection risk, no query optimization, code duplication

### 3. No Integration Interfaces
```go
// Each integration is unique
type PostgreSQLReceiver struct{ /* fields */ }
type MySQLReceiver struct{ /* different fields */ }
type PrometheusReceiver struct{ /* totally different */ }

// No common interface!
```
**Impact**: Can't add new integrations easily, no code reuse

## Required Fixes

### Fix 1: Add Abstraction Layer
```go
// Define our own interfaces
type MetricsProcessor interface {
    Process(context.Context, Metrics) (Metrics, error)
}

type Metrics interface {
    Count() int
    Get(index int) Metric
}

// Adapter for OTel
type OTelAdapter struct {
    processor MetricsProcessor
}

func (a *OTelAdapter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
    metrics := adaptFromOTel(md)
    processed, err := a.processor.Process(ctx, metrics)
    if err != nil {
        return err
    }
    return a.next.ConsumeMetrics(ctx, adaptToOTel(processed))
}
```

### Fix 2: Database Query Abstraction
```go
type DatabaseQuerier interface {
    Query(ctx context.Context, query Query) (Result, error)
}

type Query struct {
    Name   string
    SQL    string
    Params []interface{}
}

// Safe, reusable queries
var PostgresQueries = map[string]Query{
    "database_stats": {
        Name: "database_stats",
        SQL:  "SELECT datname, numbackends FROM pg_stat_database WHERE datname = $1",
    },
}
```

### Fix 3: Standard Integration Interface
```go
type Integration interface {
    Name() string
    Start(context.Context) error
    Stop(context.Context) error
    Collect(context.Context) (Metrics, error)
}

type BaseIntegration struct {
    name   string
    logger *zap.Logger
}

// All integrations use same pattern
type PostgreSQLIntegration struct {
    BaseIntegration
    querier DatabaseQuerier
}
```

## Integration Patterns

### Factory Pattern
```go
type IntegrationFactory interface {
    Type() string
    Create(config Config) (Integration, error)
}

var factories = map[string]IntegrationFactory{
    "postgresql": &PostgreSQLFactory{},
    "mysql":      &MySQLFactory{},
}

func CreateIntegration(type string, config Config) (Integration, error) {
    factory, ok := factories[type]
    if !ok {
        return nil, fmt.Errorf("unknown integration type: %s", type)
    }
    return factory.Create(config)
}
```

### Adapter Pattern
```go
// Adapt external formats to internal
type PrometheusAdapter struct {
    endpoint string
}

func (p *PrometheusAdapter) Collect(ctx context.Context) (Metrics, error) {
    // Fetch Prometheus metrics
    promMetrics := p.fetchPrometheus()
    
    // Convert to internal format
    return p.convertMetrics(promMetrics), nil
}
```

### Repository Pattern
```go
type MetricRepository interface {
    Save(ctx context.Context, metrics Metrics) error
    Query(ctx context.Context, filter Filter) (Metrics, error)
}

// Implementations for different backends
type PostgreSQLRepository struct{}
type RedisRepository struct{}
type FileRepository struct{}
```

## Migration Requirements

### Step 1: Define Interfaces
```go
// pkg/integration/interfaces.go
package integration

type Collector interface {
    Collect(context.Context) (Metrics, error)
}

type Processor interface {
    Process(context.Context, Metrics) (Metrics, error)
}

type Exporter interface {
    Export(context.Context, Metrics) error
}
```

### Step 2: Create Adapters
```go
// Wrap existing code
type LegacyAdapter struct {
    oldReceiver *oldReceiver
}

func (l *LegacyAdapter) Collect(ctx context.Context) (Metrics, error) {
    // Adapt old to new
}
```

### Step 3: Standardize Patterns
- Use factory pattern for all components
- Implement standard lifecycle
- Add standard error handling
- Use context everywhere

## Success Metrics
- All integrations implement standard interface
- Zero SQL in component code
- Can mock any integration
- New integration in < 1 day
- 90% code reuse between integrations