# Developer Guide - Database Intelligence Collector

## ✅ Production-Ready Development Environment

Welcome to the Database Intelligence Collector - a sophisticated OpenTelemetry-based monitoring solution with 5,000+ lines of production-grade code, comprehensive E2E testing, and enterprise-ready infrastructure.

## Quick Start for Developers

### Prerequisites

```bash
# Required tools
- Go 1.21+
- Docker & Docker Compose
- OpenTelemetry Collector Builder (OCB)
- Task (build automation)

# Install Task (replaces 30+ shell scripts)
brew install go-task/tap/go-task  # macOS
# or: sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin  # Linux
```

### Initial Setup

```bash
# Clone and setup
git clone https://github.com/your-org/database-intelligence-mvp
cd database-intelligence-mvp

# Quick development setup (handles everything)
task quickstart

# Or step-by-step:
task install-tools    # Install OCB and dependencies
task build            # Build collector with working components
task dev:up           # Start development environment
```

## Architecture Overview for Developers

### Core Philosophy: OTEL-First with Smart Extensions

```
┌─────────────────────────────────────────────────────────────┐
│                Production-Ready Architecture                │
│                                                             │
│  Database Sources → Standard OTEL → Custom Intelligence    │
│  (PostgreSQL/MySQL)  (Receivers/    (4 Sophisticated      │
│                      Processors/     Processors)           │
│                      Exporters)                            │
└─────────────────────────────────────────────────────────────┘
```

### Custom Processors (Production Ready)

#### 1. Adaptive Sampler (`processors/adaptivesampler/` - 576 lines)
```go
// Core processor interface
type adaptiveSamplerProcessor struct {
    config          *Config
    rules           []CompiledRule         // Expression-based rules
    cache           *lru.Cache             // LRU cache with TTL
    stateManager    *stateManager          // In-memory state only
    rateLimiters    map[string]*rateLimiter
}

// Rule evaluation with compiled expressions
func (p *adaptiveSamplerProcessor) evaluateRules(attrs pcommon.Map) (bool, float64, string) {
    // Advanced rule engine with graceful error handling
}
```

**Key Features**:
- **✅ Expression-based rule engine** with condition evaluation
- **✅ In-memory state management** (no external dependencies)
- **✅ LRU caching** with TTL for performance
- **✅ Rate limiting** per rule with adaptive adjustment
- **✅ Graceful degradation** when attributes are missing

#### 2. Circuit Breaker (`processors/circuitbreaker/` - 922 lines)
```go
type circuitBreakerProcessor struct {
    circuits           map[string]*DatabaseCircuit  // Per-database protection
    config             *Config
    throughputMonitor  *ThroughputMonitor
    errorClassifier    *ErrorClassifier
    memoryMonitor      *MemoryMonitor              // Resource protection
}

type DatabaseCircuit struct {
    state        State                    // Closed/Open/Half-Open
    failureCount int
    successCount int
    errorRate    float64
    mutex        sync.RWMutex            // Thread-safe
}
```

**Key Features**:
- **✅ Per-database circuits** with independent state machines
- **✅ Three-state FSM** (Closed → Open → Half-Open)
- **✅ Adaptive timeouts** based on performance patterns
- **✅ Resource monitoring** (CPU, memory thresholds)
- **✅ New Relic error detection** and cardinality protection

#### 3. Plan Attribute Extractor (`processors/planattributeextractor/` - 391 lines)
```go
type planAttributeExtractorProcessor struct {
    config        *Config
    parsers       map[string]PlanParser    // PostgreSQL/MySQL parsers
    hashGenerator *PlanHashGenerator
    cache         *AttributeCache          // Plan deduplication
}

// Safe plan parsing (no database calls)
func (p *planAttributeExtractorProcessor) extractPlanAttributes(lr plog.LogRecord) error {
    // Parse existing plan data, generate derived attributes
}
```

**Key Features**:
- **✅ Multi-database support** (PostgreSQL, MySQL)
- **✅ Safe mode enforced** (no direct database EXPLAIN calls)
- **✅ Plan hash generation** for deduplication
- **✅ Derived attributes** (cost calculations, scan types)
- **✅ Graceful degradation** when plan data unavailable

#### 4. Verification Processor (`processors/verification/` - 1,353 lines)
```go
type verificationProcessor struct {
    validators    []QualityValidator       // Pluggable validation
    piiDetector   *PIIDetector            // Enhanced PII detection
    healthMonitor *HealthMonitor
    autoTuner     *AutoTuningEngine       // Dynamic optimization
    selfHealer    *SelfHealingEngine
}

// Enhanced PII detection patterns
var PIIPatterns = map[string]*regexp.Regexp{
    "credit_card": regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`),
    "ssn":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
    "email":       regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
    "phone":       regexp.MustCompile(`\b\d{3}[-.]\d{3}[-.]\d{4}\b`),
}
```

**Key Features**:
- **✅ Enhanced PII detection** (credit cards, SSNs, emails, phones)
- **✅ Data quality validation** with configurable rules
- **✅ Auto-tuning capabilities** for performance optimization
- **✅ Self-healing engine** for automatic issue resolution
- **✅ Health monitoring** with component status tracking

### Production Infrastructure (`internal/`)

#### Health Monitoring (`internal/health/checker.go`)
```go
type HealthChecker struct {
    components map[string]HealthCheckFunc
    status     *ComponentStatus
    mutex      sync.RWMutex
}

// Component health checking
func (hc *HealthChecker) CheckHealth(ctx context.Context) *HealthStatus {
    // Comprehensive component health validation
}
```

#### Performance Optimization (`internal/performance/optimizer.go`)
```go
type PerformanceOptimizer struct {
    objectPools    map[string]*sync.Pool    // Object pooling
    memoryMonitor  *MemoryMonitor
    cacheManager   *CacheManager
}

// Object pooling for frequently allocated structures
func (po *PerformanceOptimizer) GetPlanAttributePool() *sync.Pool {
    return po.objectPools["plan_attributes"]
}
```

#### Rate Limiting (`internal/ratelimit/limiter.go`)
```go
type RateLimiter struct {
    limiters map[string]*perDatabaseLimiter    // Per-database limits
    config   *RateLimitConfig
    metrics  *RateLimitMetrics
}

// Adaptive rate limiting per database
func (rl *RateLimiter) Allow(database string) bool {
    // Advanced rate limiting with adaptive adjustment
}
```

## Development Workflow

### Building and Testing

```bash
# Core development commands
task build              # Build collector with current working components
task test:unit          # Run unit tests for all processors
task test:integration   # Integration tests with live databases
task test:e2e           # Comprehensive E2E testing suite

# Processor-specific testing
task test:processor PROCESSOR=adaptivesampler
task test:processor PROCESSOR=circuitbreaker
task test:processor PROCESSOR=planattributeextractor
task test:processor PROCESSOR=verification

# Performance testing
task test:benchmark     # Benchmark all processors
task test:performance   # Performance validation
task test:load         # Load testing scenarios
```

### Development Environment

```bash
# Start development environment
task dev:up             # Start all services (PostgreSQL, MySQL, collector)
task dev:watch          # Hot reload mode for development
task dev:logs           # View logs from all services
task dev:down          # Stop development environment

# Health monitoring
task health-check       # Check collector and component health
task metrics           # View collector metrics
task debug             # Debug mode with detailed logging
```

### Configuration Development

```bash
# Configuration management
task config:generate ENV=development    # Generate dev config
task config:validate                   # Validate all configurations
task config:test                      # Test configuration changes

# Environment-specific configs
config/
├── base.yaml                    # Base configuration template
└── environments/
    ├── development.yaml         # Development overrides
    ├── staging.yaml            # Staging overrides
    └── production.yaml         # Production overrides
```

## Comprehensive E2E Testing Framework

### Advanced Testing Architecture

The project includes a sophisticated 973+ line E2E testing framework with:

#### Test Infrastructure Components
```go
type E2EMetricsFlowTestSuite struct {
    // Core infrastructure
    pgContainer         *postgres.PostgresContainer
    mysqlContainer      *mysql.MySQLContainer
    collector           *otelcol.Collector
    
    // Advanced testing utilities
    workloadGenerators  map[string]*WorkloadGenerator
    metricValidator     *MetricValidator
    performanceBench    *PerformanceBenchmark
    nrdbValidator       *NRDBValidator
    resourceMonitor     *ResourceMonitor
    stressTestManager   *StressTestManager
}
```

#### Running E2E Tests

```bash
# Basic E2E testing
go test -tags=e2e ./tests/e2e/e2e_main_test.go -v

# Comprehensive testing suite
go test -tags=e2e ./tests/e2e/e2e_metrics_flow_test.go -v

# Specific test categories
go test -tags=e2e -run TestPostgreSQLMetricsFlow ./tests/e2e/... -v
go test -tags=e2e -run TestPIISanitizationValidation ./tests/e2e/... -v
go test -tags=e2e -run TestHighLoadStressTesting ./tests/e2e/... -v

# With custom configuration
E2E_CONFIG_PATH=./test-config.json go test -tags=e2e ./tests/e2e/... -v
```

### Test Categories

#### 1. Database-Specific Testing
- **PostgreSQL Flow**: Complete metrics collection and validation
- **MySQL Flow**: Performance schema and infrastructure metrics
- **Cross-Database**: Performance comparison and compatibility

#### 2. Processor Testing
- **Adaptive Sampling**: Behavior under different load conditions
- **Circuit Breaker**: Activation, recovery, and resource protection
- **Plan Extraction**: Query plan parsing and optimization
- **Verification**: Data quality and PII sanitization

#### 3. Performance & Stress Testing
- **High Load**: Concurrent user simulation with realistic workloads
- **Resource Limits**: Memory pressure and CPU utilization testing
- **Failover**: Database failover scenarios and recovery
- **NRDB Integration**: Direct New Relic validation with NRQL queries

## Development Best Practices

### Code Organization

```
processors/
├── adaptivesampler/
│   ├── processor.go          # Main processor logic
│   ├── config.go            # Configuration structures
│   ├── rules.go             # Rule engine implementation
│   ├── cache.go             # LRU cache management
│   └── processor_test.go    # Comprehensive unit tests
├── circuitbreaker/
│   ├── processor.go          # Circuit breaker logic
│   ├── circuit.go           # Per-database circuit state
│   ├── monitor.go           # Resource monitoring
│   └── processor_test.go    # State machine testing
└── [similar structure for other processors]
```

### Error Handling Patterns

```go
// Graceful degradation pattern used throughout
func (p *processor) ProcessLogs(ctx context.Context, logs plog.Logs) (plog.Logs, error) {
    for i := 0; i < logs.ResourceLogs().Len(); i++ {
        resourceLogs := logs.ResourceLogs().At(i)
        
        if err := p.processResourceLogs(ctx, resourceLogs); err != nil {
            // Log error but continue processing
            p.logger.Warn("Processing error", zap.Error(err))
            continue
        }
    }
    return logs, nil  // Never block the pipeline
}
```

### Memory Management

```go
// Object pooling pattern for frequently allocated structures
var planAttributePool = sync.Pool{
    New: func() interface{} {
        return &PlanAttributes{
            Operations: make([]Operation, 0, 10),
            Indexes:    make([]IndexUsage, 0, 5),
        }
    },
}

func (p *processor) getPlanAttributes() *PlanAttributes {
    attrs := planAttributePool.Get().(*PlanAttributes)
    attrs.Reset()  // Clear previous data
    return attrs
}

func (p *processor) putPlanAttributes(attrs *PlanAttributes) {
    planAttributePool.Put(attrs)
}
```

### Observability Patterns

```go
// Self-monitoring throughout processors
func (p *processor) ProcessMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
    start := time.Now()
    defer func() {
        p.metrics.ProcessingDuration.Record(time.Since(start).Milliseconds())
        p.metrics.ProcessedMetrics.Add(int64(metrics.MetricCount()))
    }()
    
    // Actual processing logic
    return p.processMetrics(ctx, metrics)
}
```

## Debugging and Troubleshooting

### Debug Mode

```bash
# Enable debug logging
export OTEL_LOG_LEVEL=debug
task run

# Debug specific processor
export ADAPTIVE_SAMPLER_DEBUG=true
export CIRCUIT_BREAKER_DEBUG=true
task run
```

### Common Development Issues

#### 1. Processor Registration
```go
// Ensure processors are registered in main.go
func main() {
    factories, err := otelcol.DefaultFactories()
    if err != nil {
        log.Fatal(err)
    }
    
    // Register custom processors
    factories.Processors[adaptivesampler.TypeStr] = adaptivesampler.NewFactory()
    factories.Processors[circuitbreaker.TypeStr] = circuitbreaker.NewFactory()
    // ... other processors
}
```

#### 2. Module Path Issues
```bash
# Standardize module paths if build fails
task fix:module-paths
```

#### 3. Memory Issues
```bash
# Monitor memory usage during development
task monitor:memory

# Enable memory profiling
export OTEL_ENABLE_PPROF=true
curl http://localhost:1777/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

### Health Monitoring

```bash
# Check component health
curl http://localhost:13133/health
curl http://localhost:13133/health/ready

# View processor metrics
curl http://localhost:8888/metrics | grep adaptive_sampler
curl http://localhost:8888/metrics | grep circuit_breaker

# Debug endpoints
curl http://localhost:55679/debug/tracez
curl http://localhost:55679/debug/pipelinez
```

## Configuration for Developers

### Development Configuration Template

```yaml
# config/environments/development.yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    transport: tcp
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    databases:
      - postgres
    collection_interval: 10s
    tls:
      insecure: true

processors:
  memory_limiter:
    limit_mib: 256
    check_interval: 5s
  
  adaptive_sampler:
    in_memory_only: true
    cache_size: 1000
    cache_ttl: 300s
    rules:
      - name: "high_duration"
        condition: "duration_ms > 1000"
        sampling_rate: 1.0
      - name: "normal_queries"
        condition: "duration_ms <= 1000"
        sampling_rate: 0.1
    default_sampling_rate: 0.01
  
  circuit_breaker:
    failure_threshold: 5
    timeout: 30s
    half_open_requests: 3
    
exporters:
  debug:
    verbosity: detailed
  otlp:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  extensions: [health_check, pprof, zpages]
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, adaptive_sampler, circuit_breaker]
      exporters: [debug, otlp]
  telemetry:
    logs:
      level: debug
    metrics:
      level: detailed
      address: 0.0.0.0:8888
```

### Environment Variables

```bash
# Database connections
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=password
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=password

# New Relic integration
export NEW_RELIC_LICENSE_KEY=your-license-key
export NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318

# Development settings
export ENVIRONMENT=development
export LOG_LEVEL=debug
export ENABLE_PPROF=true
```

## Contributing Guidelines

### Adding New Processors

1. **Create processor structure**:
```bash
mkdir processors/newprocessor
cd processors/newprocessor
```

2. **Implement required interfaces**:
```go
// processor.go
type newProcessor struct {
    config *Config
    logger *zap.Logger
}

func (p *newProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
    // Implementation
}

// Implement consumer.Metrics interface
func (p *newProcessor) Capabilities() consumer.Capabilities { /* */ }
func (p *newProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error { /* */ }
```

3. **Add comprehensive tests**:
```go
// processor_test.go
func TestNewProcessor(t *testing.T) {
    // Unit tests with 80%+ coverage
}

func BenchmarkNewProcessor(b *testing.B) {
    // Performance benchmarks
}
```

4. **Update build configuration**:
```yaml
# ocb-config.yaml
processors:
  - gomod: github.com/database-intelligence-mvp/processors/newprocessor v0.0.0
    path: ./processors/newprocessor
```

5. **Register in main.go**:
```go
factories.Processors[newprocessor.TypeStr] = newprocessor.NewFactory()
```

### Testing Requirements

- **Unit tests**: 80%+ coverage for new code
- **Integration tests**: Database interaction testing
- **E2E tests**: Add relevant test cases to E2E suite
- **Performance tests**: Benchmark critical paths
- **Documentation**: Update relevant .md files

### Code Review Checklist

- [ ] Follows error handling patterns (graceful degradation)
- [ ] Implements proper observability (metrics, logs)
- [ ] Uses object pooling for frequently allocated structures
- [ ] Thread-safe implementation with proper mutex usage
- [ ] Comprehensive unit tests with edge cases
- [ ] Performance benchmarks for critical paths
- [ ] Documentation updated (architecture, configuration)
- [ ] E2E test coverage for new functionality

## Performance Optimization

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./processors/...
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=. ./processors/...
go tool pprof mem.prof

# Continuous profiling in development
export OTEL_ENABLE_PPROF=true
task run
# Access http://localhost:1777/debug/pprof/
```

### Optimization Patterns

#### 1. Object Pooling
```go
// Use sync.Pool for frequently allocated objects
var metricPool = sync.Pool{
    New: func() interface{} {
        return &ProcessedMetric{
            Attributes: make(map[string]interface{}, 10),
        }
    },
}
```

#### 2. Batch Processing
```go
// Process in batches to reduce overhead
func (p *processor) processBatch(metrics []pmetric.Metric) error {
    const batchSize = 100
    for i := 0; i < len(metrics); i += batchSize {
        end := i + batchSize
        if end > len(metrics) {
            end = len(metrics)
        }
        if err := p.processBatchChunk(metrics[i:end]); err != nil {
            return err
        }
    }
    return nil
}
```

#### 3. Caching Strategies
```go
// LRU cache with TTL for expensive operations
cache, _ := lru.NewWithEvict(1000, func(key, value interface{}) {
    // Cleanup on eviction
})

// Cache with TTL
type CacheEntry struct {
    Value     interface{}
    ExpiresAt time.Time
}
```

## Resources and References

### Documentation Structure
- **[ARCHITECTURE.md](./ARCHITECTURE.md)**: System design and component details
- **[TESTING.md](./TESTING.md)**: Comprehensive testing framework guide
- **[RUNBOOK.md](./RUNBOOK.md)**: Operational procedures and troubleshooting
- **[CONFIGURATION.md](./CONFIGURATION.md)**: Configuration reference and examples

### External Resources
- [OpenTelemetry Collector Development](https://opentelemetry.io/docs/collector/building/)
- [Go Best Practices](https://golang.org/doc/effective_go.html)
- [New Relic OTLP Integration](https://docs.newrelic.com/docs/more-integrations-and-instrumentation/open-source-telemetry-integrations/opentelemetry/opentelemetry-quick-start/)

### Getting Help

- **Development Questions**: Check existing issues or create new ones
- **Performance Issues**: Use profiling tools and performance testing suite
- **Configuration Problems**: Refer to working examples in `config/` directory
- **Testing Issues**: Review E2E testing framework documentation

---

This guide provides comprehensive information for developers working on the Database Intelligence Collector. The codebase represents a sophisticated, production-ready implementation with advanced features, comprehensive testing, and enterprise-grade reliability.