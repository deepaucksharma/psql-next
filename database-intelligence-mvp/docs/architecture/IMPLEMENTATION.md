# Technical Implementation Deep Dive

This document provides a comprehensive technical analysis of the Database Intelligence Collector implementation, including code structure, design patterns, and implementation details.

## Project Structure

```
database-intelligence-mvp/
├── main.go                          # Entry point with component registration
├── go.mod                           # Module definition
├── ocb-config.yaml                  # OpenTelemetry Collector Builder config
├── otelcol-builder.yaml             # Alternative builder configuration
│
├── processors/                      # Custom processors (3,242 lines total)
│   ├── adaptivesampler/            # Intelligent sampling (576 lines)
│   │   ├── processor.go            # Main processor implementation
│   │   ├── config.go               # Configuration structures
│   │   ├── factory.go              # Factory pattern implementation
│   │   ├── rules.go                # Rule evaluation engine
│   │   └── metrics.go              # Processor metrics
│   │
│   ├── circuitbreaker/             # Database protection (922 lines)
│   │   ├── processor.go            # Circuit breaker state machine
│   │   ├── config.go               # Configuration with per-DB settings
│   │   ├── factory.go              # Factory with health checker
│   │   ├── circuit.go              # Individual circuit implementation
│   │   └── health.go               # Health checking logic
│   │
│   ├── planattributeextractor/     # Query plan parsing (391 lines)
│   │   ├── processor.go            # Plan extraction logic
│   │   ├── config.go               # Parser configuration
│   │   ├── factory.go              # Factory with parser pool
│   │   ├── parser.go               # PostgreSQL/MySQL parsers
│   │   └── cache.go                # Plan caching implementation
│   │
│   └── verification/               # Data quality & PII (1,353 lines)
│       ├── processor.go            # Main verification logic
│       ├── config.go               # PII patterns configuration
│       ├── factory.go              # Factory with detector setup
│       ├── pii_detector.go         # Pattern matching engine
│       ├── quality_checker.go      # Data quality validation
│       └── auto_tuner.go           # ML-based tuning
│
├── internal/                        # Internal packages
│   ├── health/                     # Health monitoring system
│   │   ├── checker.go              # Component health checks
│   │   └── server.go               # Health HTTP server
│   │
│   ├── ratelimit/                  # Rate limiting implementation
│   │   ├── limiter.go              # Token bucket implementation
│   │   └── adaptive.go             # Adaptive rate adjustment
│   │
│   └── performance/                # Performance optimization
│       ├── optimizer.go            # Memory/CPU optimization
│       └── pool.go                 # Object pooling
│
├── config/                         # Configuration files
│   ├── collector-production.yaml   # Production configuration
│   ├── collector-resilient.yaml    # Resilient single-instance config
│   └── pii-detection-enhanced.yaml # Enhanced PII patterns
│
└── scripts/                        # Operational scripts
    ├── generate-config.sh          # Configuration generator
    └── validate-env.sh             # Environment validator
```

## Core Implementation Details

### 1. Main Entry Point (`main.go`)

```go
package main

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    
    // Custom processors
    "github.com/database-intelligence-mvp/processors/adaptivesampler"
    "github.com/database-intelligence-mvp/processors/circuitbreaker"
    "github.com/database-intelligence-mvp/processors/planattributeextractor"
    "github.com/database-intelligence-mvp/processors/verification"
)

func main() {
    factories, err := components()
    if err != nil {
        log.Fatal(err)
    }
    
    info := component.BuildInfo{
        Command:     "database-intelligence-collector",
        Description: "OpenTelemetry Collector with database intelligence",
        Version:     "1.0.0",
    }
    
    if err := run(otelcol.CollectorSettings{
        BuildInfo: info,
        Factories: factories,
    }); err != nil {
        log.Fatal(err)
    }
}

func components() (otelcol.Factories, error) {
    var err error
    factories := otelcol.Factories{}
    
    // Register custom processors
    factories.Processors, err = component.MakeProcessorFactoryMap(
        adaptivesampler.NewFactory(),
        circuitbreaker.NewFactory(),
        planattributeextractor.NewFactory(),
        verification.NewFactory(),
    )
    
    return factories, err
}
```

### 2. Factory Pattern Implementation

Each processor follows the OpenTelemetry factory pattern:

```go
// Example: Adaptive Sampler Factory
package adaptivesampler

const (
    TypeStr = "adaptive_sampler"  // Must match config
)

func NewFactory() processor.Factory {
    return processor.NewFactory(
        TypeStr,
        createDefaultConfig,
        processor.WithMetrics(createMetricsProcessor, stability.StabilityLevelBeta),
    )
}

func createDefaultConfig() component.Config {
    return &Config{
        InMemoryOnly:      true,  // Force in-memory
        DefaultSampleRate: 0.1,
        Rules:            []Rule{},
        Deduplication: DeduplicationConfig{
            Enabled:   true,
            CacheSize: 10000,
            TTL:       60 * time.Second,
        },
    }
}

func createMetricsProcessor(
    ctx context.Context,
    set processor.CreateSettings,
    cfg component.Config,
    nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
    oCfg := cfg.(*Config)
    
    return newAdaptiveSampler(set.Logger, oCfg, nextConsumer)
}
```

### 3. Processor Implementation Pattern

All processors follow a consistent implementation pattern:

```go
type baseProcessor struct {
    config       component.Config
    logger       *zap.Logger
    nextConsumer consumer.Metrics
    metrics      *processorMetrics
    
    // Processor-specific state
    stateMutex   sync.RWMutex
    // ...
}

// Lifecycle methods
func (p *baseProcessor) Start(ctx context.Context, host component.Host) error {
    p.logger.Info("Starting processor", zap.String("type", p.config.ID().String()))
    
    // Initialize state
    // Start background workers
    // Register health checks
    
    return nil
}

func (p *baseProcessor) Shutdown(ctx context.Context) error {
    p.logger.Info("Shutting down processor")
    
    // Stop workers
    // Cleanup resources
    // Flush state
    
    return nil
}

// Core processing method
func (p *baseProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
    // Pre-processing checks
    if err := p.validateInput(md); err != nil {
        return err
    }
    
    // Process metrics
    processed, err := p.processMetrics(ctx, md)
    if err != nil {
        p.metrics.recordError(err)
        return err
    }
    
    // Pass to next consumer
    return p.nextConsumer.ConsumeMetrics(ctx, processed)
}
```

### 4. State Management Architecture

#### In-Memory State Design

```go
// Adaptive Sampler State
type processorState struct {
    // Deduplication cache with LRU eviction
    deduplicationCache *lru.Cache[string, time.Time]
    
    // Per-rule rate limiters
    ruleLimiters map[string]*rateLimiter
    
    // Metrics tracking
    metrics struct {
        processed   uint64
        dropped     uint64
        cacheHits   uint64
        cacheMisses uint64
    }
    
    // Concurrency control
    stateMutex sync.RWMutex
}

// Circuit Breaker State
type circuitState struct {
    circuits map[string]*circuit  // Per-database circuits
    mutex    sync.RWMutex
}

type circuit struct {
    state              State  // closed, open, half-open
    failures           int
    lastFailureTime    time.Time
    lastTransitionTime time.Time
    mutex              sync.RWMutex
}
```

#### State Persistence (Removed)

The previous file-based persistence has been completely removed:

```go
// OLD CODE (REMOVED):
// func (p *processor) loadState() error {
//     data, err := os.ReadFile(p.config.StateFile)
//     ...
// }

// NEW CODE:
func (p *processor) initializeState() error {
    // Only in-memory initialization
    p.state = &processorState{
        deduplicationCache: lru.New[string, time.Time](p.config.CacheSize),
        ruleLimiters:      make(map[string]*rateLimiter),
    }
    return nil
}
```

### 5. Performance Optimization Techniques

#### Object Pooling

```go
// Plan parser pooling reduces allocations
var parserPool = sync.Pool{
    New: func() interface{} {
        return &planParser{
            buffer: make([]byte, 0, 4096),
            decoder: json.NewDecoder(nil),
        }
    },
}

func (p *planAttributeExtractor) parsePlan(data []byte) (*ParsedPlan, error) {
    parser := parserPool.Get().(*planParser)
    defer parserPool.Put(parser)
    
    return parser.Parse(data)
}
```

#### Efficient String Building

```go
// Use strings.Builder for efficient concatenation
func buildCacheKey(fields []string, attrs pcommon.Map) string {
    var builder strings.Builder
    builder.Grow(256)  // Pre-allocate
    
    for i, field := range fields {
        if i > 0 {
            builder.WriteByte('|')
        }
        if val, ok := attrs.Get(field); ok {
            builder.WriteString(val.Str())
        }
    }
    
    return builder.String()
}
```

#### Batch Processing

```go
// Process metrics in batches for efficiency
func (p *processor) processMetricsBatch(metrics []pmetric.Metric) error {
    const batchSize = 100
    
    for i := 0; i < len(metrics); i += batchSize {
        end := min(i+batchSize, len(metrics))
        batch := metrics[i:end]
        
        if err := p.processBatch(batch); err != nil {
            return err
        }
    }
    
    return nil
}
```

### 6. Concurrency Patterns

#### Read-Write Locks

```go
// Optimize for read-heavy workloads
func (p *processor) checkCache(key string) (bool, time.Time) {
    p.stateMutex.RLock()
    defer p.stateMutex.RUnlock()
    
    return p.deduplicationCache.Get(key)
}

func (p *processor) updateCache(key string, timestamp time.Time) {
    p.stateMutex.Lock()
    defer p.stateMutex.Unlock()
    
    p.deduplicationCache.Add(key, timestamp)
}
```

#### Channel-Based Communication

```go
// Circuit breaker health checks
type healthChecker struct {
    checkInterval time.Duration
    checks        chan healthCheckRequest
    results       chan healthCheckResult
}

func (h *healthChecker) run(ctx context.Context) {
    ticker := time.NewTicker(h.checkInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            h.performChecks()
        case req := <-h.checks:
            h.handleCheck(req)
        }
    }
}
```

### 7. Error Handling Patterns

#### Graceful Degradation

```go
func (p *processor) processWithFallback(ctx context.Context, metric pmetric.Metric) error {
    // Try primary processing
    if err := p.processPrimary(ctx, metric); err != nil {
        p.logger.Warn("Primary processing failed, using fallback",
            zap.Error(err),
            zap.String("metric", metric.Name()))
        
        // Fallback to basic processing
        return p.processFallback(ctx, metric)
    }
    
    return nil
}
```

#### Circuit Breaker Pattern

```go
func (cb *circuitBreaker) call(fn func() error) error {
    cb.mutex.RLock()
    state := cb.state
    cb.mutex.RUnlock()
    
    switch state {
    case StateClosed:
        err := fn()
        if err != nil {
            cb.recordFailure()
        } else {
            cb.recordSuccess()
        }
        return err
        
    case StateOpen:
        return ErrCircuitOpen
        
    case StateHalfOpen:
        err := fn()
        if err != nil {
            cb.transitionToOpen()
        } else {
            cb.transitionToClosed()
        }
        return err
    }
}
```

### 8. Metrics and Observability

#### Processor Metrics

```go
type processorMetrics struct {
    processedCounter   metric.Int64Counter
    droppedCounter     metric.Int64Counter
    latencyHistogram   metric.Float64Histogram
    errorCounter       metric.Int64Counter
    
    // Custom metrics
    cacheHitRate      metric.Float64ObservableGauge
    samplingRate      metric.Float64ObservableGauge
}

func (m *processorMetrics) record(ctx context.Context, duration time.Duration, err error) {
    m.processedCounter.Add(ctx, 1)
    m.latencyHistogram.Record(ctx, duration.Seconds())
    
    if err != nil {
        m.errorCounter.Add(ctx, 1,
            metric.WithAttributes(
                attribute.String("error_type", classifyError(err)),
            ))
    }
}
```

#### Health Reporting

```go
type componentHealth struct {
    Healthy   bool                   `json:"healthy"`
    Message   string                 `json:"message,omitempty"`
    Metrics   map[string]interface{} `json:"metrics"`
    LastCheck time.Time              `json:"last_check"`
}

func (p *processor) Health() componentHealth {
    p.stateMutex.RLock()
    defer p.stateMutex.RUnlock()
    
    return componentHealth{
        Healthy: true,
        Metrics: map[string]interface{}{
            "cache_size":     p.deduplicationCache.Len(),
            "cache_capacity": p.deduplicationCache.Cap(),
            "sample_rate":    p.currentSampleRate(),
            "rules_active":   len(p.config.Rules),
        },
        LastCheck: time.Now(),
    }
}
```

### 9. Configuration Management

#### Environment Variable Support

```go
func expandEnvVars(cfg *Config) {
    // Expand environment variables in configuration
    cfg.SlowQueryThreshold = expandEnvInt("SLOW_QUERY_THRESHOLD", cfg.SlowQueryThreshold)
    cfg.MaxRecordsPerSecond = expandEnvInt("MAX_RECORDS_PER_SECOND", cfg.MaxRecordsPerSecond)
    
    // Apply environment-specific overrides
    if env := os.Getenv("ENVIRONMENT"); env != "" {
        if override, ok := cfg.EnvironmentOverrides[env]; ok {
            mergeConfig(cfg, override)
        }
    }
}
```

#### Validation

```go
func (cfg *Config) Validate() error {
    if cfg.DefaultSampleRate < 0 || cfg.DefaultSampleRate > 1 {
        return fmt.Errorf("default_sample_rate must be between 0 and 1")
    }
    
    for i, rule := range cfg.Rules {
        if err := rule.Validate(); err != nil {
            return fmt.Errorf("rule %d (%s): %w", i, rule.Name, err)
        }
    }
    
    if cfg.Deduplication.Enabled && cfg.Deduplication.CacheSize <= 0 {
        return fmt.Errorf("deduplication cache_size must be positive")
    }
    
    return nil
}
```

### 10. Testing Patterns

#### Unit Test Structure

```go
func TestAdaptiveSampler_ProcessMetrics(t *testing.T) {
    tests := []struct {
        name           string
        config         Config
        inputMetrics   pmetric.Metrics
        expectedCount  int
        expectedError  bool
    }{
        {
            name: "basic sampling",
            config: Config{
                DefaultSampleRate: 0.5,
                InMemoryOnly:     true,
            },
            inputMetrics:  generateTestMetrics(100),
            expectedCount: 50,  // Approximately
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            processor := newTestProcessor(t, tt.config)
            
            output, err := processor.ProcessMetrics(context.Background(), tt.inputMetrics)
            
            if tt.expectedError {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.expectedCount, output.MetricCount())
            }
        })
    }
}
```

#### Benchmark Tests

```go
func BenchmarkAdaptiveSampler_ProcessMetrics(b *testing.B) {
    processor := newBenchmarkProcessor(b)
    metrics := generateLargeMetricSet(10000)
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        _, err := processor.ProcessMetrics(context.Background(), metrics.Clone())
        if err != nil {
            b.Fatal(err)
        }
    }
    
    b.ReportMetric(float64(metrics.MetricCount())/float64(b.Elapsed().Seconds()), "metrics/sec")
}
```

## Design Decisions

### 1. In-Memory Only State
- **Decision**: Remove all file-based persistence
- **Rationale**: Simplifies deployment, improves performance, eliminates I/O bottlenecks
- **Trade-off**: State lost on restart (acceptable for sampling/caching use cases)

### 2. Single Instance Architecture
- **Decision**: No distributed state or coordination
- **Rationale**: Reduces complexity, easier operations, faster recovery
- **Trade-off**: No high availability (mitigated by fast restart)

### 3. Graceful Degradation
- **Decision**: Each processor can fail independently
- **Rationale**: Prevents cascade failures, maintains partial functionality
- **Implementation**: Try-catch patterns, fallback logic, skip on error

### 4. Resource Bounds
- **Decision**: Hard limits on all caches and buffers
- **Rationale**: Prevents memory leaks, ensures predictable resource usage
- **Implementation**: LRU caches, bounded channels, pool limits

## Performance Considerations

### Memory Usage Breakdown
```
Base Collector:          ~50MB
Adaptive Sampler:        ~50-100MB (cache dependent)
Circuit Breaker:         ~10MB (minimal state)
Plan Extractor:          ~50MB (parser cache)
Verification:            ~30MB (pattern engine)
Batch Processor:         ~50MB (queue buffer)
---
Total:                   ~240-340MB typical
```

### CPU Usage Profile
```
Metric Reception:        5-10%
Adaptive Sampling:       10-15% (rule evaluation)
Circuit Breaking:        <1% (state checks)
Plan Extraction:         15-20% (JSON parsing)
Verification:            10-15% (regex matching)
Export/Serialization:    10-15%
---
Total:                   50-75% of 1 core typical
```

### Optimization Opportunities
1. **Parallel Processing**: Plan extraction could be parallelized
2. **SIMD Operations**: PII pattern matching could use SIMD
3. **Memory Mapping**: Large plan cache could use mmap
4. **GPU Acceleration**: Complex regex matching on GPU

---

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025