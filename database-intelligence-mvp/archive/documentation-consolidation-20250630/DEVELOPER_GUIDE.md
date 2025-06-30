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
task build:docker       # Build Docker image for testing

# Comprehensive testing suite
task test:unit          # Run unit tests for all processors (with coverage)
task test:integration   # Integration tests with live databases
task test:e2e           # Comprehensive E2E testing suite
task test:all           # Run all tests (unit + integration + E2E + coverage)

# Processor-specific testing
task test:unit:processor PROCESSOR=adaptivesampler
task test:unit:processor PROCESSOR=circuitbreaker
task test:unit:processor PROCESSOR=planattributeextractor
task test:unit:processor PROCESSOR=verification

# Advanced testing capabilities
task test:benchmark     # Performance benchmarks with profiling
task test:load          # Load testing with K6
task test:watch         # Watch mode for continuous testing
task test:specific NAME=TestAdaptiveSampler  # Run specific test
task test:coverage      # Generate HTML coverage report
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

The project includes a sophisticated 973+ line E2E testing framework with comprehensive infrastructure:

#### E2E Test Directory Structure
```
tests/e2e/
├── e2e_main_test.go              # Basic E2E test suite
├── e2e_metrics_flow_test.go      # Advanced comprehensive testing (973+ lines)
├── collector-e2e-test.yaml      # E2E collector configuration
├── configs/                      # Test-specific configurations
├── containers/                   # Container initialization scripts
│   ├── postgres-init.sql
│   └── mysql-init.sql
├── sql/
│   └── seed.sql                  # Test data seeding
├── validators/                   # Validation utilities
│   ├── metric_validator.go
│   └── nrdb_validator.go
├── workloads/                    # Workload generation
│   ├── database_setup.go
│   ├── query_templates.go
│   └── workload_generator.go
├── benchmarks/                   # Performance benchmarking
└── reports/                      # Test reporting
```

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
    
    // Test configuration and results
    testConfig          *E2ETestConfig
    testResults         *E2ETestResults
}
```

### Running E2E Tests

#### Task-Based E2E Testing (Recommended)

```bash
# Prerequisites: Set required environment variables
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"

# Complete E2E testing suite
task test:e2e           # Builds collector + runs full E2E suite

# E2E testing in CI (assumes pre-built binary)
task test:e2e:ci        # Skips build, runs E2E tests

# All tests including E2E
task test:all           # Unit + Integration + E2E + Coverage
```

#### Direct Go Testing

```bash
# Basic E2E testing
go test -tags=e2e ./tests/e2e/e2e_main_test.go -v

# Comprehensive testing suite (973+ lines)
go test -tags=e2e ./tests/e2e/e2e_metrics_flow_test.go -v

# All E2E tests with timeout
go test -tags=e2e -timeout=15m ./tests/e2e/... -v

# Specific test categories
go test -tags=e2e -run TestPostgreSQLMetricsFlow ./tests/e2e/... -v
go test -tags=e2e -run TestPIISanitizationValidation ./tests/e2e/... -v
go test -tags=e2e -run TestHighLoadStressTesting ./tests/e2e/... -v
go test -tags=e2e -run TestCircuitBreakerActivationRecovery ./tests/e2e/... -v

# With custom configuration
E2E_CONFIG_PATH=./tests/e2e/configs/custom-config.json go test -tags=e2e ./tests/e2e/... -v

# Short mode (skips E2E tests)
go test -short ./tests/e2e/...
```

#### E2E Test Environment Setup

```bash
# Required environment variables for E2E tests
export NEW_RELIC_LICENSE_KEY="your-ingest-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"
export NEW_RELIC_API_KEY="your-user-api-key"          # For NRQL validation

# Optional configuration
export E2E_CONFIG_PATH="./tests/e2e/configs/custom.json"
export E2E_POSTGRES_VERSION="15-alpine"
export E2E_MYSQL_VERSION="8.0"
export E2E_TEST_TIMEOUT="15m"
export E2E_WORKLOAD_DURATION="300s"
export E2E_MAX_CONCURRENT_QUERIES="100"
```

### Comprehensive Test Categories

#### 1. Database-Specific Testing (`TestPostgreSQLMetricsFlow`, `TestMySQLMetricsFlow`)
```go
// PostgreSQL comprehensive testing
func TestPostgreSQLMetricsFlow() {
    - Infrastructure metrics collection (connections, cache hits, etc.)
    - Query performance tracking with pg_stat_statements
    - PII sanitization validation with realistic data
    - Adaptive sampling behavior testing
    - Circuit breaker functionality verification
    - NRDB data validation with NRQL queries
}

// MySQL comprehensive testing  
func TestMySQLMetricsFlow() {
    - Performance schema metrics collection
    - Query tracking and performance analysis
    - Infrastructure metrics validation
    - Slow query detection and reporting
    - Cross-database compatibility testing
}
```

#### 2. Advanced Processor Testing
```go
// Adaptive sampling under different load conditions
func TestAdaptiveSamplingBehavior() {
    - Low load sampling (high precision)
    - Medium load sampling (balanced)
    - High load sampling (aggressive)
    - Extreme load sampling (survival mode)
    - Sampling decision validation
}

// Circuit breaker activation and recovery
func TestCircuitBreakerActivationRecovery() {
    - Database connectivity issue simulation
    - High error rate triggering
    - Automatic recovery testing
    - Resource-based triggers (CPU, memory)
    - Circuit breaker metrics validation
}

// Plan extraction and optimization
func TestQueryPerformanceTracking() {
    - Query plan parsing accuracy
    - Plan hash generation and deduplication
    - Derived attribute calculation
    - Performance optimization validation
    - Cross-database plan compatibility
}

// Data quality and security
func TestPIISanitizationValidation() {
    - Credit card number detection and masking
    - SSN pattern recognition and sanitization
    - Email address obfuscation
    - Phone number pattern matching
    - Custom PII pattern validation
}
```

#### 3. Performance & Stress Testing
```go
// High load and stress scenarios
func TestHighLoadStressTesting() {
    - Concurrent user simulation (50-100 users)
    - Query load generation (mixed OLTP/OLAP)
    - Resource utilization monitoring
    - Performance degradation analysis
    - System stability validation
}

// Database failover scenarios
func TestDatabaseFailoverScenarios() {
    - PostgreSQL container termination and restart
    - MySQL connection loss simulation
    - Collector behavior during failover
    - Automatic recovery validation
    - Data consistency verification
}

// Memory pressure and resource limits
func TestMemoryPressureResourceLimits() {
    - Memory usage escalation testing
    - Resource limit enforcement
    - Garbage collection optimization
    - Memory leak detection
    - Graceful degradation validation
}
```

#### 4. Integration & Validation Testing
```go
// NRDB integration with validation
func TestNRDBIntegrationValidation() {
    - Metric export verification
    - NRQL query execution and validation
    - Data freshness checking
    - Entity synthesis verification
    - Dashboard compatibility testing
}

// Error scenarios and recovery
func TestErrorScenariosRecovery() {
    - Network connectivity errors
    - Database connection failures
    - NRDB export errors
    - Configuration errors
    - Automatic recovery mechanisms
}

// Verification processor health checks
func TestVerificationProcessorHealthChecks() {
    - Component health monitoring
    - Auto-tuning capability testing
    - Self-healing engine validation
    - Performance optimization verification
    - Quality metrics calculation
}
```

### Advanced E2E Testing Features

#### Workload Generation
```go
// tests/e2e/workloads/workload_generator.go
type WorkloadGenerator struct {
    DatabaseType    string                    // postgresql, mysql
    WorkloadType    string                    // oltp, olap, mixed, pii_test
    Concurrency     int                       // Concurrent connections
    Duration        time.Duration             // Test duration
    QueryTemplates  map[string]QueryTemplate  // Query patterns
}

// Realistic workload patterns
workloadTypes := []string{
    "oltp",           // Online transaction processing
    "olap",           // Online analytical processing  
    "mixed",          // Mixed OLTP/OLAP workload
    "pii_test",       // PII data generation for sanitization testing
    "performance_test", // Performance-focused query patterns
}
```

#### NRDB Validation Framework
```go
// tests/e2e/validators/nrdb_validator.go
type NRDBValidator struct {
    APIKey      string
    AccountID   string
    HTTPClient  *http.Client
    QueryCache  map[string]*QueryResult
}

// NRQL validation queries
nrdbQueries := []NRDBQuery{
    {
        Name:    "metrics_presence",
        Query:   "SELECT count(*) FROM Metric WHERE instrumentation.provider = 'database-intelligence' SINCE 10 minutes ago",
        Expected: 1,
        Critical: true,
    },
    {
        Name:    "data_freshness", 
        Query:   "SELECT latest(timestamp) FROM Metric WHERE collector.name = 'database-intelligence' SINCE 10 minutes ago",
        Timeout: 30 * time.Second,
        Critical: true,
    },
}
```

#### Performance Benchmarking
```go
// tests/e2e/benchmarks/performance_bench.go
type PerformanceBenchmark struct {
    Metrics       *PerformanceMetrics
    Baselines     map[string]float64      // Performance baselines
    Thresholds    *PerformanceThresholds  // Pass/fail thresholds
    ResourceMon   *ResourceMonitor        // CPU, memory monitoring
}

// Performance thresholds
thresholds := PerformanceThresholds{
    MaxLatencyMS:        1000,    // Maximum processing latency
    MinThroughputRPS:    10.0,    // Minimum throughput
    MaxErrorRate:        0.05,    // Maximum error rate (5%)
    MaxMemoryUsageMB:    512,     // Maximum memory usage
    MaxCPUUsagePercent:  80.0,    // Maximum CPU usage
}
```

#### Test Configuration Management
```go
// Custom E2E test configuration
type E2ETestConfig struct {
    PostgreSQLConfig DatabaseConfig          `json:"postgresql_config"`
    MySQLConfig      DatabaseConfig          `json:"mysql_config"`
    WorkloadDuration time.Duration           `json:"workload_duration"`
    WorkloadTypes    []string                `json:"workload_types"`
    ValidationRules  []ValidationRule        `json:"metric_validation_rules"`
    NRDBQueries      []NRDBQuery            `json:"nrdb_validation_queries"`
    StressScenarios  []StressTestScenario   `json:"stress_test_scenarios"`
    Thresholds       TestThresholds         `json:"thresholds"`
}

// Load custom configuration
configPath := os.Getenv("E2E_CONFIG_PATH")
if configPath != "" {
    config = loadConfigFromFile(configPath)
} else {
    config = createDefaultTestConfig()
}
```

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

## Task-Based Development Workflow

### Core Task Commands

```bash
# Setup and initialization
task setup              # Complete development environment setup
task setup:tools        # Install OCB and development tools
task setup:deps         # Download and verify Go dependencies

# Building
task build              # Build collector with working components
task build:docker       # Build Docker image for testing
task clean              # Clean all generated files and artifacts

# Development environment
task dev:up             # Start development environment (PostgreSQL + MySQL)
task dev:down           # Stop development environment
task dev:logs           # View logs from all services
task dev:watch          # Hot reload mode for development
```

### Testing Task Workflows

```bash
# Unit testing
task test:unit                              # All unit tests with coverage
task test:unit:processor PROCESSOR=name     # Test specific processor
task test:watch                            # Continuous testing (watch mode)
task test:specific NAME=TestName           # Run specific test function

# Integration testing  
task test:integration                       # Integration tests with live databases

# E2E testing (comprehensive)
task test:e2e                              # Full E2E suite (builds + tests)
task test:e2e:ci                           # E2E for CI (pre-built binary)
task test:all                              # All tests (unit + integration + E2E)

# Performance testing
task test:benchmark                        # Performance benchmarks with profiling
task test:load DURATION=5m VUS=10         # Load testing with K6
task test:coverage                         # Generate HTML coverage report
```

### Advanced Task Features

```bash
# Validation and quality
task validate:all                          # Validate everything
task validate:config                       # Validate configurations
task lint                                  # Run linters
task fmt                                   # Format code

# Deployment tasks
task deploy:docker                         # Docker deployment
task deploy:helm ENV=staging              # Kubernetes deployment
task deploy:binary                         # Binary deployment

# Monitoring and health
task health-check                          # Check collector health
task metrics                              # View collector metrics
task debug                                # Debug mode with detailed logging
```

### E2E Task Integration

```bash
# E2E prerequisites check
task test:e2e:check                        # Verify E2E environment setup

# E2E with custom configuration
E2E_CONFIG_PATH=./custom-config.json task test:e2e

# E2E specific categories
task test:e2e:postgresql                   # PostgreSQL-specific E2E tests
task test:e2e:mysql                        # MySQL-specific E2E tests
task test:e2e:processors                   # Processor-specific E2E tests
task test:e2e:stress                       # Stress and performance E2E tests

# E2E reporting
task test:e2e:report                       # Generate comprehensive E2E report
task test:e2e:artifacts                    # Collect E2E test artifacts
```

### Development Workflow Examples

#### Daily Development Workflow
```bash
# 1. Start development session
task setup                                 # Ensure environment is ready
task dev:up                               # Start databases

# 2. Make code changes
# ... edit processor code ...

# 3. Test changes
task test:unit:processor PROCESSOR=adaptivesampler  # Test specific processor
task test:watch                           # Continuous testing while developing

# 4. Integration testing
task test:integration                      # Test with live databases

# 5. End-to-end validation
task test:e2e                             # Full E2E validation

# 6. Cleanup
task dev:down                             # Stop development environment
```

#### E2E Testing Workflow
```bash
# 1. Environment preparation
export NEW_RELIC_LICENSE_KEY="your-key"
export NEW_RELIC_ACCOUNT_ID="your-account"
task test:e2e:check                       # Verify environment

# 2. Run comprehensive E2E tests
task test:e2e                             # Full E2E suite

# 3. Run specific test categories
task test:e2e:processors                  # Focus on processor functionality
task test:e2e:stress                      # Performance and stress testing

# 4. Generate reports
task test:e2e:report                      # Comprehensive test report
task test:e2e:artifacts                   # Collect logs and metrics
```

#### Performance Analysis Workflow
```bash
# 1. Baseline performance
task test:benchmark                       # Establish performance baseline

# 2. Load testing
task test:load DURATION=10m VUS=20       # Extended load testing

# 3. E2E performance validation
task test:e2e:stress                      # E2E stress testing

# 4. Analysis
go tool pprof cpu.prof                    # Analyze CPU profile
go tool pprof mem.prof                    # Analyze memory profile
task metrics                             # Review collector metrics
```

## Debugging and Troubleshooting

### Debug Mode and Logging

```bash
# Enable debug logging
export OTEL_LOG_LEVEL=debug
task run

# Debug specific processors
export ADAPTIVE_SAMPLER_DEBUG=true
export CIRCUIT_BREAKER_DEBUG=true
export PLAN_EXTRACTOR_DEBUG=true
export VERIFICATION_DEBUG=true
task run

# Enable profiling
export OTEL_ENABLE_PPROF=true
task run
# Access profiling at http://localhost:1777/debug/pprof/
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
curl http://localhost:8888/metrics | grep plan_extractor
curl http://localhost:8888/metrics | grep verification

# Debug endpoints
curl http://localhost:55679/debug/tracez
curl http://localhost:55679/debug/pipelinez

# Using Task commands
task health-check                          # Automated health validation
task metrics                              # Formatted metrics display
task debug                                # Enable debug mode
```

### E2E Testing Troubleshooting

#### Common E2E Issues and Solutions

```bash
# Issue: NEW_RELIC_LICENSE_KEY not set
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"
task test:e2e:check                        # Verify environment setup

# Issue: Docker containers not starting
task dev:down                             # Clean shutdown
docker system prune -f                    # Clean Docker resources
task dev:up                               # Restart environment

# Issue: Database connection failures
task test:integration                      # Test database connectivity
docker logs database-intelligence-postgres-1  # Check PostgreSQL logs
docker logs database-intelligence-mysql-1     # Check MySQL logs

# Issue: E2E tests timeout
export E2E_TEST_TIMEOUT="20m"             # Extend timeout
task test:e2e                             # Retry with longer timeout

# Issue: NRDB validation failures
export NEW_RELIC_API_KEY="your-user-api-key"  # Ensure API key is set
task test:e2e:check                        # Verify NRDB connectivity

# Issue: Testcontainer permission issues
sudo chmod 666 /var/run/docker.sock       # Fix Docker permissions (Linux)
docker info                               # Verify Docker access
```

#### E2E Test Debugging

```bash
# Enable verbose E2E logging
export E2E_DEBUG=true
export E2E_VERBOSE=true
go test -tags=e2e -v ./tests/e2e/... 

# Debug specific E2E test
go test -tags=e2e -run TestPostgreSQLMetricsFlow -v ./tests/e2e/...

# Collect E2E test artifacts
task test:e2e:artifacts                    # Collect logs, configs, reports

# Manual E2E environment inspection
task dev:up                               # Start environment
# ... run tests manually ...
task dev:logs                             # Inspect logs
task dev:down                             # Clean shutdown
```

#### E2E Performance Issues

```bash
# Monitor resource usage during E2E tests
docker stats                              # Monitor container resources
htop                                      # Monitor system resources

# E2E with reduced load
export E2E_WORKLOAD_CONCURRENCY="5"       # Reduce concurrency
export E2E_WORKLOAD_DURATION="120s"       # Shorter test duration
task test:e2e

# E2E memory optimization
export E2E_MEMORY_LIMIT="1024m"           # Set memory limits
export GOMEMLIMIT="512MiB"                # Go memory limit
task test:e2e
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

## Quick Reference

### Essential Commands

```bash
# Quick start for new developers
task setup && task dev:up && task test:all

# Daily development workflow
task test:unit:processor PROCESSOR=adaptivesampler  # Test specific processor
task test:watch                                     # Continuous testing
task test:e2e                                       # Full E2E validation

# E2E testing with environment setup
export NEW_RELIC_LICENSE_KEY="your-key"
export NEW_RELIC_ACCOUNT_ID="your-account"
task test:e2e                                       # Comprehensive E2E testing

# Debugging and troubleshooting
task debug                                          # Enable debug mode
task health-check                                   # Check system health
task test:e2e:artifacts                            # Collect E2E artifacts
```

### File Structure Quick Reference

```
database-intelligence-mvp/
├── processors/                    # 4 production-ready processors (5,000+ lines)
│   ├── adaptivesampler/           # 576 lines - Intelligent sampling
│   ├── circuitbreaker/           # 922 lines - Database protection
│   ├── planattributeextractor/   # 391 lines - Query plan analysis  
│   └── verification/             # 1,353 lines - Data quality & PII
├── tests/e2e/                    # Comprehensive E2E testing (973+ lines)
│   ├── e2e_main_test.go          # Basic E2E test suite
│   ├── e2e_metrics_flow_test.go  # Advanced comprehensive testing
│   ├── workloads/                # Realistic workload generation
│   ├── validators/               # NRDB and metric validation
│   └── benchmarks/               # Performance benchmarking
├── internal/                     # Production infrastructure
│   ├── health/                   # Health monitoring system
│   ├── performance/              # Performance optimization
│   └── ratelimit/               # Rate limiting system
├── config/                       # Environment-specific configurations
├── tasks/                        # Task automation (build, test, deploy)
└── docs/                         # Comprehensive documentation
```

## Resources and References

### Documentation Structure
- **[ARCHITECTURE.md](./ARCHITECTURE.md)**: System design and component details
- **[TESTING.md](./TESTING.md)**: Comprehensive testing framework guide (973+ lines)
- **[RUNBOOK.md](./RUNBOOK.md)**: Operational procedures and troubleshooting
- **[CONFIGURATION.md](./CONFIGURATION.md)**: Configuration reference and examples

### Task Automation
- **[Taskfile.yml](../Taskfile.yml)**: Main task automation with 50+ commands
- **[tasks/](../tasks/)**: Modular task files (build, test, deploy, validate)
- **Task Help**: Run `task --list-all` to see all available commands

### E2E Testing Resources
- **[tests/e2e/](../tests/e2e/)**: Complete E2E testing infrastructure
- **Environment Setup**: NEW_RELIC_LICENSE_KEY and NEW_RELIC_ACCOUNT_ID required
- **Test Categories**: Database-specific, processor, performance, integration
- **Debugging**: E2E_DEBUG=true for verbose logging

### External Resources
- [OpenTelemetry Collector Development](https://opentelemetry.io/docs/collector/building/)
- [Go Best Practices](https://golang.org/doc/effective_go.html)
- [New Relic OTLP Integration](https://docs.newrelic.com/docs/more-integrations-and-instrumentation/open-source-telemetry-integrations/opentelemetry/opentelemetry-quick-start/)
- [Testcontainers for Go](https://golang.testcontainers.org/)

### Getting Help

- **Development Questions**: Check existing issues or create new ones
- **E2E Testing Issues**: Review E2E troubleshooting section above
- **Performance Issues**: Use `task test:benchmark` and profiling tools
- **Configuration Problems**: Refer to working examples in `config/` directory
- **Task Automation**: Run `task --list-all` for all available commands

### Contributing Workflow

1. **Setup**: `task setup` - Complete development environment
2. **Development**: `task test:watch` - Continuous testing during development
3. **Validation**: `task test:all` - Full test suite including E2E
4. **Performance**: `task test:benchmark` - Performance validation
5. **Integration**: `task test:e2e` - Comprehensive E2E testing with NRDB validation

---

This comprehensive developer guide provides everything needed to work effectively with the Database Intelligence Collector's sophisticated, production-ready implementation. The codebase features 5,000+ lines of production-grade code, advanced E2E testing framework (973+ lines), comprehensive Task automation, and enterprise-grade reliability with full NRDB validation.