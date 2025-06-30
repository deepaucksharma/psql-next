# Development Guide

This guide covers local development setup, contributing guidelines, and best practices for the Database Intelligence Collector project.

## Development Environment Setup

### Prerequisites

- **Go 1.21+**: Required for building
- **Docker & Docker Compose**: For running databases
- **Task**: Build automation tool
- **Git**: Version control
- **IDE**: VS Code or GoLand recommended

### Quick Setup

```bash
# Clone repository
git clone https://github.com/database-intelligence-mvp/database-intelligence-mvp.git
cd database-intelligence-mvp

# Install dependencies
make install-tools

# Start development databases
docker-compose up -d postgres mysql

# Build collector
make build

# Run tests
make test

# Start collector with hot reload
make dev
```

## Project Structure

```
database-intelligence-mvp/
├── main.go                     # Entry point
├── go.mod                      # Go module definition
├── Makefile                    # Build automation
├── Taskfile.yml               # Task automation
│
├── processors/                 # Custom processors
│   ├── adaptivesampler/       # Sampling processor
│   ├── circuitbreaker/        # Circuit breaker
│   ├── planattributeextractor/# Plan extraction
│   └── verification/          # Data validation
│
├── internal/                   # Internal packages
│   ├── health/                # Health monitoring
│   ├── ratelimit/             # Rate limiting
│   └── performance/           # Optimizations
│
├── config/                     # Configuration files
├── scripts/                    # Utility scripts
├── tests/                      # Test files
└── docs/                       # Documentation
```

## Development Workflow

### 1. Setting Up Your Fork

```bash
# Fork the repository on GitHub

# Clone your fork
git clone https://github.com/YOUR_USERNAME/database-intelligence-mvp.git
cd database-intelligence-mvp

# Add upstream remote
git remote add upstream https://github.com/database-intelligence-mvp/database-intelligence-mvp.git

# Keep your fork updated
git fetch upstream
git checkout main
git merge upstream/main
```

### 2. Creating a Feature Branch

```bash
# Create feature branch
git checkout -b feature/your-feature-name

# Or bugfix branch
git checkout -b bugfix/issue-description
```

### 3. Making Changes

```bash
# Make your changes
vim processors/adaptivesampler/processor.go

# Run tests
make test

# Check formatting
make fmt

# Run linter
make lint

# Build and test locally
make build && ./dist/database-intelligence-collector --config=config/dev.yaml
```

### 4. Committing Changes

```bash
# Stage changes
git add -A

# Commit with descriptive message
git commit -m "feat(adaptive-sampler): add dynamic threshold adjustment

- Implement threshold calculation based on historical data
- Add configuration for adjustment sensitivity
- Include tests for edge cases"

# Push to your fork
git push origin feature/your-feature-name
```

## Coding Standards

### Go Style Guide

Follow the official [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

#### Key Points:
- Use `gofmt` for formatting
- Follow standard Go project layout
- Write idiomatic Go code
- Document exported functions

### Code Organization

```go
// Package comment describes the package
package adaptivesampler

import (
    // Standard library imports first
    "context"
    "fmt"
    "time"
    
    // External imports second
    "go.opentelemetry.io/collector/component"
    "go.uber.org/zap"
    
    // Internal imports last
    "github.com/database-intelligence-mvp/internal/cache"
)

// Constants at package level
const (
    TypeStr = "adaptive_sampler"
    defaultCacheSize = 10000
)

// Types follow constants
type Config struct {
    // Exported fields with json/yaml tags
    CacheSize int `mapstructure:"cache_size"`
}

// Constructor functions
func NewFactory() processor.Factory {
    return processor.NewFactory(
        TypeStr,
        createDefaultConfig,
        processor.WithMetrics(createMetricsProcessor, stability.StabilityLevelBeta),
    )
}

// Methods on types
func (c *Config) Validate() error {
    if c.CacheSize <= 0 {
        return fmt.Errorf("cache_size must be positive")
    }
    return nil
}
```

### Error Handling

```go
// Always check errors
result, err := processMetric(metric)
if err != nil {
    // Add context to errors
    return fmt.Errorf("failed to process metric %s: %w", metric.Name(), err)
}

// Use error variables for sentinel errors
var (
    ErrCircuitOpen = errors.New("circuit breaker is open")
    ErrCacheFull   = errors.New("cache is full")
)

// Type assertions should check success
if sampler, ok := processor.(*adaptiveSampler); ok {
    // Use sampler
} else {
    return fmt.Errorf("processor is not adaptive sampler")
}
```

### Logging

```go
// Use structured logging
logger.Info("Processing metric",
    zap.String("name", metric.Name()),
    zap.Int("datapoints", metric.DataPointCount()),
    zap.Duration("latency", latency),
)

// Log levels:
// - Debug: Detailed information for debugging
// - Info: General informational messages
// - Warn: Warning conditions
// - Error: Error conditions
// - Fatal: Fatal errors that cause shutdown
```

## Testing

### Unit Tests

```go
func TestAdaptiveSampler_SampleMetric(t *testing.T) {
    // Table-driven tests
    tests := []struct {
        name          string
        config        Config
        metric        pmetric.Metric
        expectedSample bool
        expectedError  error
    }{
        {
            name: "sample slow query",
            config: Config{
                Rules: []Rule{{
                    Name: "slow_queries",
                    Conditions: []Condition{{
                        Attribute: "duration_ms",
                        Operator:  "gt",
                        Value:     "1000",
                    }},
                    SampleRate: 1.0,
                }},
            },
            metric:         createMetricWithDuration(2000),
            expectedSample: true,
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            sampler := newTestSampler(t, tt.config)
            
            sampled, err := sampler.shouldSample(tt.metric)
            
            if tt.expectedError != nil {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.expectedError.Error())
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.expectedSample, sampled)
            }
        })
    }
}
```

### Integration Tests

```go
func TestPipeline_EndToEnd(t *testing.T) {
    // Start test databases
    postgres := startTestPostgres(t)
    defer postgres.Stop()
    
    // Create collector with test config
    collector := startTestCollector(t, testConfig{
        Receivers: []string{"postgresql"},
        Processors: []string{"adaptive_sampler", "circuit_breaker"},
        Exporters: []string{"inmemory"},
    })
    defer collector.Shutdown()
    
    // Generate test load
    generateTestQueries(t, postgres, 100)
    
    // Wait for processing
    time.Sleep(5 * time.Second)
    
    // Verify results
    metrics := collector.GetExportedMetrics()
    assert.Greater(t, len(metrics), 0)
    
    // Check sampling worked
    assert.Less(t, len(metrics), 100, "Should have sampled metrics")
}
```

### Benchmarks

```go
func BenchmarkAdaptiveSampler_ProcessBatch(b *testing.B) {
    sampler := createBenchmarkSampler(b)
    metrics := generateLargeMetricSet(10000)
    
    b.ResetTimer()
    b.ReportAllocs()
    
    for i := 0; i < b.N; i++ {
        err := sampler.ProcessMetrics(context.Background(), metrics)
        if err != nil {
            b.Fatal(err)
        }
    }
    
    b.ReportMetric(float64(len(metrics))/float64(b.Elapsed().Seconds()), "metrics/sec")
}
```

## Building Custom Processors

### Processor Template

```go
package myprocessor

import (
    "context"
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/consumer"
    "go.opentelemetry.io/collector/processor"
    "go.opentelemetry.io/collector/processor/processorhelper"
)

const (
    TypeStr = "my_processor"
)

// Config holds processor configuration
type Config struct {
    Setting string `mapstructure:"setting"`
}

// Validate configuration
func (cfg *Config) Validate() error {
    if cfg.Setting == "" {
        return errors.New("setting is required")
    }
    return nil
}

// NewFactory creates processor factory
func NewFactory() processor.Factory {
    return processor.NewFactory(
        TypeStr,
        createDefaultConfig,
        processor.WithMetrics(createMetricsProcessor, component.StabilityLevelBeta),
    )
}

func createDefaultConfig() component.Config {
    return &Config{
        Setting: "default",
    }
}

func createMetricsProcessor(
    ctx context.Context,
    set processor.CreateSettings,
    cfg component.Config,
    nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
    oCfg := cfg.(*Config)
    
    mp := &myProcessor{
        config: oCfg,
        logger: set.Logger,
    }
    
    return processorhelper.NewMetricsProcessor(
        ctx,
        set,
        cfg,
        nextConsumer,
        mp.processMetrics,
        processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
    )
}

type myProcessor struct {
    config *Config
    logger *zap.Logger
}

func (p *myProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
    // Process metrics here
    return md, nil
}
```

### Registering Your Processor

1. Add to `ocb-config.yaml`:
```yaml
processors:
  - gomod: github.com/database-intelligence-mvp/processors/myprocessor v0.0.0
```

2. Add to `main.go`:
```go
import "github.com/database-intelligence-mvp/processors/myprocessor"

func components() (otelcol.Factories, error) {
    factories.Processors = append(factories.Processors, myprocessor.NewFactory())
}
```

## Debugging

### Local Debugging

```bash
# Run with debug logging
./dist/database-intelligence-collector \
    --config=config/dev.yaml \
    --set=service.telemetry.logs.level=debug

# Enable pprof
./dist/database-intelligence-collector \
    --config=config/dev.yaml \
    --feature-gates=+enableProfilingEndpoint
```

### VS Code Launch Configuration

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug Collector",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/main.go",
            "env": {
                "POSTGRES_HOST": "localhost",
                "POSTGRES_USER": "postgres",
                "POSTGRES_PASSWORD": "postgres"
            },
            "args": [
                "--config=${workspaceFolder}/config/dev.yaml",
                "--set=service.telemetry.logs.level=debug"
            ]
        }
    ]
}
```

### Common Issues

#### Module Dependencies
```bash
# Update dependencies
go mod tidy

# Download dependencies
go mod download

# Verify dependencies
go mod verify

# Update specific dependency
go get -u github.com/some/package@latest
```

#### Build Issues
```bash
# Clean build cache
go clean -cache

# Rebuild with verbose output
go build -v ./...

# Check for compilation errors
go vet ./...
```

## Performance Optimization

### Profiling

```go
// CPU profiling
import _ "net/http/pprof"

func init() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
}

// Profile analysis
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
go tool pprof http://localhost:6060/debug/pprof/heap
```

### Memory Optimization

```go
// Use object pools
var metricPool = sync.Pool{
    New: func() interface{} {
        return pmetric.NewMetrics()
    },
}

func getMetric() pmetric.Metrics {
    return metricPool.Get().(pmetric.Metrics)
}

func putMetric(m pmetric.Metrics) {
    m.Reset()
    metricPool.Put(m)
}

// Pre-allocate slices
metrics := make([]pmetric.Metric, 0, expectedSize)

// Use strings.Builder for concatenation
var builder strings.Builder
builder.Grow(estimatedSize)
builder.WriteString("prefix")
result := builder.String()
```

## Contributing

### Pull Request Process

1. **Fork and Clone**: Fork the repository and clone locally
2. **Branch**: Create a feature branch from `main`
3. **Develop**: Make changes following coding standards
4. **Test**: Add/update tests for your changes
5. **Document**: Update documentation if needed
6. **Commit**: Use conventional commit messages
7. **Push**: Push to your fork
8. **PR**: Open a pull request with clear description

### PR Checklist

- [ ] Tests pass locally (`make test`)
- [ ] Code is formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated
- [ ] Commits are signed
- [ ] PR description explains changes
- [ ] Breaking changes noted

### Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style changes
- `refactor`: Code refactoring
- `perf`: Performance improvement
- `test`: Test changes
- `chore`: Build process or auxiliary tool changes

Example:
```
feat(adaptive-sampler): add rule priority support

- Rules can now have priority values
- Higher priority rules are evaluated first
- Ties are broken by rule name

Closes #123
```

## Release Process

### Version Tagging

```bash
# Create release tag
git tag -s v1.2.0 -m "Release version 1.2.0"

# Push tag
git push upstream v1.2.0
```

### Release Notes Template

```markdown
## What's Changed

### Features
- Feature description (#PR)

### Bug Fixes
- Fix description (#PR)

### Performance
- Performance improvement (#PR)

### Documentation
- Documentation updates (#PR)

### Breaking Changes
- Breaking change description

## Upgrade Guide
Instructions for upgrading from previous version

## Contributors
Thanks to all contributors!
```

---

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025