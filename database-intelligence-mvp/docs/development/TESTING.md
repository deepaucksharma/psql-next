# Testing Guide

This guide covers testing strategies, procedures, and best practices for the Database Intelligence Collector.

## Testing Overview

The project uses a multi-layered testing approach:
- **Unit Tests**: Test individual components in isolation
- **Integration Tests**: Test component interactions
- **End-to-End Tests**: Test complete data flow
- **Performance Tests**: Benchmark and load testing
- **Validation Tests**: Configuration and security validation

## Test Structure

```
tests/
├── unit/                    # Unit tests
├── integration/            # Integration tests
├── e2e/                    # End-to-end tests
├── performance/            # Performance tests
├── fixtures/               # Test data and configurations
└── helpers/                # Test utilities
```

## Running Tests

### Quick Test Commands

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests
make test-integration

# Run with coverage
make test-coverage

# Run specific package tests
go test ./processors/adaptivesampler/...

# Run specific test
go test -run TestAdaptiveSampler_ProcessMetrics ./processors/adaptivesampler/

# Run with verbose output
go test -v ./...

# Run with race detection
go test -race ./...
```

### Test Configuration

Create `test.env` for test-specific settings:
```bash
# Test databases
TEST_POSTGRES_HOST=localhost
TEST_POSTGRES_PORT=5433
TEST_POSTGRES_USER=test_user
TEST_POSTGRES_PASSWORD=test_pass

TEST_MYSQL_HOST=localhost
TEST_MYSQL_PORT=3307
TEST_MYSQL_USER=test_user
TEST_MYSQL_PASSWORD=test_pass

# Test timeouts
TEST_TIMEOUT=30s
TEST_RETRY_COUNT=3
```

## Unit Testing

### Testing Processors

```go
package adaptivesampler

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.opentelemetry.io/collector/pdata/pmetric"
    "go.uber.org/zap/zaptest"
)

func TestAdaptiveSampler_ShouldSample(t *testing.T) {
    tests := []struct {
        name           string
        config         Config
        metric         pmetric.Metric
        expectedResult bool
        expectedError  string
    }{
        {
            name: "sample slow query",
            config: Config{
                Rules: []Rule{
                    {
                        Name: "slow_queries",
                        Conditions: []Condition{
                            {
                                Attribute: "duration_ms",
                                Operator:  "gt",
                                Value:     "1000",
                            },
                        },
                        SampleRate: 1.0,
                    },
                },
                DefaultSampleRate: 0.1,
                InMemoryOnly:     true,
            },
            metric:         createMetricWithAttribute("duration_ms", 2000),
            expectedResult: true,
        },
        {
            name: "skip fast query",
            config: Config{
                Rules: []Rule{
                    {
                        Name: "slow_queries",
                        Conditions: []Condition{
                            {
                                Attribute: "duration_ms",
                                Operator:  "gt",
                                Value:     "1000",
                            },
                        },
                        SampleRate: 0.0,
                    },
                },
                DefaultSampleRate: 0.0,
                InMemoryOnly:     true,
            },
            metric:         createMetricWithAttribute("duration_ms", 500),
            expectedResult: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            logger := zaptest.NewLogger(t)
            sampler, err := newAdaptiveSampler(logger, &tt.config, nil)
            require.NoError(t, err)
            
            result, err := sampler.shouldSample(tt.metric)
            
            if tt.expectedError != "" {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.expectedError)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.expectedResult, result)
            }
        })
    }
}

// Helper functions
func createMetricWithAttribute(key string, value interface{}) pmetric.Metric {
    metric := pmetric.NewMetric()
    metric.SetName("test.metric")
    dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
    
    switch v := value.(type) {
    case int:
        dp.Attributes().PutInt(key, int64(v))
    case string:
        dp.Attributes().PutStr(key, v)
    case float64:
        dp.Attributes().PutDouble(key, v)
    }
    
    return metric
}
```

### Testing State Management

```go
func TestCircuitBreaker_StateTransitions(t *testing.T) {
    cb := newTestCircuitBreaker(t)
    
    // Test closed -> open transition
    for i := 0; i < 5; i++ {
        cb.recordFailure("test_db")
    }
    assert.Equal(t, StateOpen, cb.getState("test_db"))
    
    // Test open -> half-open transition
    time.Sleep(cb.config.OpenStateTimeout)
    cb.allowRequest("test_db")
    assert.Equal(t, StateHalfOpen, cb.getState("test_db"))
    
    // Test half-open -> closed transition
    cb.recordSuccess("test_db")
    assert.Equal(t, StateClosed, cb.getState("test_db"))
}
```

### Testing with Mocks

```go
type mockConsumer struct {
    mock.Mock
    receivedMetrics []pmetric.Metrics
}

func (m *mockConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
    m.receivedMetrics = append(m.receivedMetrics, md)
    args := m.Called(ctx, md)
    return args.Error(0)
}

func TestProcessor_ConsumeMetrics(t *testing.T) {
    mockConsumer := new(mockConsumer)
    mockConsumer.On("ConsumeMetrics", mock.Anything, mock.Anything).Return(nil)
    
    processor := newTestProcessor(t, mockConsumer)
    
    metrics := generateTestMetrics(10)
    err := processor.ConsumeMetrics(context.Background(), metrics)
    
    require.NoError(t, err)
    mockConsumer.AssertExpectations(t)
    assert.Len(t, mockConsumer.receivedMetrics, 1)
}
```

## Integration Testing

### Database Integration Tests

```go
// +build integration

package integration

import (
    "database/sql"
    "testing"
    "time"
    
    _ "github.com/lib/pq"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgreSQLReceiver_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Start PostgreSQL container
    ctx := context.Background()
    req := testcontainers.ContainerRequest{
        Image:        "postgres:15",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "testdb",
        },
        WaitingFor: wait.ForSQL("5432/tcp", "postgres", func(host string, port nat.Port) string {
            return fmt.Sprintf("postgres://postgres:test@%s:%s/testdb?sslmode=disable", host, port.Port())
        }),
    }
    
    postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
    require.NoError(t, err)
    defer postgres.Terminate(ctx)
    
    // Get connection details
    host, err := postgres.Host(ctx)
    require.NoError(t, err)
    
    port, err := postgres.MappedPort(ctx, "5432")
    require.NoError(t, err)
    
    // Configure and start receiver
    config := &postgresqlreceiver.Config{
        Endpoint:  fmt.Sprintf("%s:%s", host, port.Port()),
        Username:  "postgres",
        Password:  "test",
        Databases: []string{"testdb"},
    }
    
    receiver := createTestReceiver(t, config)
    err = receiver.Start(ctx, componenttest.NewNopHost())
    require.NoError(t, err)
    defer receiver.Shutdown(ctx)
    
    // Wait for metrics
    time.Sleep(2 * time.Second)
    
    // Verify metrics collected
    metrics := receiver.GetCollectedMetrics()
    assert.Greater(t, len(metrics), 0)
    
    // Verify specific metrics
    hasMetric := false
    for _, metric := range metrics {
        if metric.Name() == "postgresql.database.size" {
            hasMetric = true
            break
        }
    }
    assert.True(t, hasMetric, "Expected postgresql.database.size metric")
}
```

### Processor Pipeline Tests

```go
func TestProcessorPipeline_Integration(t *testing.T) {
    // Create pipeline
    pipeline := createTestPipeline(t, PipelineConfig{
        Processors: []string{
            "memory_limiter",
            "adaptive_sampler",
            "circuit_breaker",
            "batch",
        },
    })
    
    // Send test data
    testMetrics := generateDiverseMetrics(1000)
    err := pipeline.ConsumeMetrics(context.Background(), testMetrics)
    require.NoError(t, err)
    
    // Wait for processing
    time.Sleep(100 * time.Millisecond)
    
    // Verify output
    output := pipeline.GetOutput()
    
    // Should have fewer metrics due to sampling
    assert.Less(t, len(output), 1000)
    
    // Verify sampling rules applied
    for _, metric := range output {
        attrs := metric.Attributes()
        if duration, ok := attrs.Get("duration_ms"); ok {
            // Slow queries should all be present
            if duration.Int() > 1000 {
                assert.True(t, true, "Slow query sampled")
            }
        }
    }
}
```

## End-to-End Testing

### Complete Flow Test

```go
func TestCollector_EndToEnd(t *testing.T) {
    // Start test environment
    env := startTestEnvironment(t)
    defer env.Cleanup()
    
    // Start collector
    collector := startCollector(t, "config/test-e2e.yaml", env)
    defer collector.Shutdown()
    
    // Generate database load
    generateDatabaseLoad(t, env.PostgresURL, LoadProfile{
        Duration:      30 * time.Second,
        QPS:           100,
        SlowQueryRate: 0.1,
    })
    
    // Wait for data to flow through
    time.Sleep(5 * time.Second)
    
    // Verify metrics in export destination
    exportedMetrics := env.GetExportedMetrics()
    
    // Verify key metrics present
    metricsFound := map[string]bool{
        "postgresql.database.size":     false,
        "postgresql.backends":          false,
        "postgresql.commits":           false,
        "postgresql.rollbacks":         false,
        "sqlquery.duration":           false,
    }
    
    for _, metric := range exportedMetrics {
        metricsFound[metric.Name()] = true
    }
    
    for name, found := range metricsFound {
        assert.True(t, found, "Metric %s not found", name)
    }
    
    // Verify sampling worked
    slowQueries := countSlowQueries(exportedMetrics)
    totalQueries := countTotalQueries(exportedMetrics)
    samplingRate := float64(slowQueries) / float64(totalQueries)
    
    assert.InDelta(t, 0.1, samplingRate, 0.05, "Sampling rate should be ~10%")
}
```

### New Relic Integration Test

```go
func TestNewRelicExport_E2E(t *testing.T) {
    if os.Getenv("NEW_RELIC_LICENSE_KEY") == "" {
        t.Skip("NEW_RELIC_LICENSE_KEY not set")
    }
    
    // Configure collector with New Relic export
    config := `
receivers:
  postgresql:
    endpoint: ${TEST_POSTGRES_HOST}:5432
    username: ${TEST_POSTGRES_USER}
    password: ${TEST_POSTGRES_PASSWORD}

processors:
  batch:
    timeout: 10s

exporters:
  otlp/newrelic:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlp/newrelic]
`
    
    collector := startCollectorWithConfig(t, config)
    defer collector.Shutdown()
    
    // Wait for metrics to be exported
    time.Sleep(2 * time.Minute)
    
    // Query New Relic for metrics
    client := newrelic.NewClient(os.Getenv("NEW_RELIC_LICENSE_KEY"))
    
    query := `
        SELECT count(*) 
        FROM Metric 
        WHERE metricName LIKE 'postgresql.%' 
        SINCE 5 minutes ago
    `
    
    result, err := client.Query(query)
    require.NoError(t, err)
    
    count := result[0]["count"].(float64)
    assert.Greater(t, count, 0.0, "Should have PostgreSQL metrics in New Relic")
}
```

## Performance Testing

### Benchmark Tests

```go
func BenchmarkAdaptiveSampler_ProcessMetrics(b *testing.B) {
    configs := []struct {
        name   string
        rules  int
        cache  int
    }{
        {"small", 5, 1000},
        {"medium", 20, 10000},
        {"large", 100, 50000},
    }
    
    for _, cfg := range configs {
        b.Run(cfg.name, func(b *testing.B) {
            sampler := createBenchmarkSampler(b, cfg.rules, cfg.cache)
            metrics := generateMetrics(1000)
            
            b.ResetTimer()
            b.ReportAllocs()
            
            for i := 0; i < b.N; i++ {
                _, err := sampler.ProcessMetrics(context.Background(), metrics)
                if err != nil {
                    b.Fatal(err)
                }
            }
            
            metricsPerSec := float64(1000*b.N) / b.Elapsed().Seconds()
            b.ReportMetric(metricsPerSec, "metrics/sec")
            b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*1000), "ns/metric")
        })
    }
}
```

### Load Testing

```go
func TestCollector_LoadTest(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping load test")
    }
    
    collector := startCollector(t, "config/load-test.yaml")
    defer collector.Shutdown()
    
    // Metrics collection
    metrics := &sync.Map{}
    go collectMetrics(t, collector, metrics)
    
    // Generate increasing load
    loadProfiles := []LoadProfile{
        {QPS: 100, Duration: 1 * time.Minute},
        {QPS: 500, Duration: 1 * time.Minute},
        {QPS: 1000, Duration: 1 * time.Minute},
        {QPS: 5000, Duration: 1 * time.Minute},
    }
    
    for _, profile := range loadProfiles {
        t.Logf("Testing with %d QPS", profile.QPS)
        
        generateLoad(t, collector, profile)
        
        // Check metrics
        checkMetrics(t, metrics, MetricExpectations{
            MaxMemoryMB:      512,
            MaxCPUPercent:    80,
            MaxLatencyMs:     100,
            MinThroughputQPS: profile.QPS * 0.9,
        })
    }
    
    // Generate report
    generateLoadTestReport(t, metrics)
}
```

### Memory Leak Detection

```go
func TestCollector_MemoryLeak(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping memory leak test")
    }
    
    collector := startCollector(t, "config/memory-test.yaml")
    defer collector.Shutdown()
    
    // Baseline memory
    runtime.GC()
    baseline := getCurrentMemory()
    
    // Run for extended period
    for i := 0; i < 100; i++ {
        metrics := generateLargeMetrics(10000)
        err := collector.ConsumeMetrics(context.Background(), metrics)
        require.NoError(t, err)
        
        if i%10 == 0 {
            runtime.GC()
            current := getCurrentMemory()
            growth := current - baseline
            
            t.Logf("Iteration %d: Memory %d MB (growth: %d MB)", 
                i, current/1024/1024, growth/1024/1024)
            
            // Fail if memory grows too much
            assert.Less(t, growth, int64(100*1024*1024), 
                "Memory growth exceeds 100MB")
        }
        
        time.Sleep(100 * time.Millisecond)
    }
}
```

## Configuration Validation

### Config Test Suite

```go
func TestConfiguration_Validation(t *testing.T) {
    testCases := []struct {
        name        string
        configFile  string
        shouldError bool
        errorMsg    string
    }{
        {
            name:        "valid production config",
            configFile:  "config/collector-production.yaml",
            shouldError: false,
        },
        {
            name:        "invalid sampling rate",
            configFile:  "fixtures/invalid-sampling-rate.yaml",
            shouldError: true,
            errorMsg:    "sample_rate must be between 0 and 1",
        },
        {
            name:        "missing required fields",
            configFile:  "fixtures/missing-fields.yaml",
            shouldError: true,
            errorMsg:    "endpoint is required",
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            cfg, err := loadConfig(tc.configFile)
            if err != nil {
                if tc.shouldError {
                    assert.Contains(t, err.Error(), tc.errorMsg)
                    return
                }
                t.Fatal(err)
            }
            
            err = cfg.Validate()
            if tc.shouldError {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tc.errorMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Environment Variable Expansion

```go
func TestConfiguration_EnvVarExpansion(t *testing.T) {
    // Set test environment variables
    os.Setenv("TEST_DB_HOST", "testhost")
    os.Setenv("TEST_DB_PASS", "testpass")
    os.Setenv("TEST_SAMPLE_RATE", "0.5")
    defer os.Unsetenv("TEST_DB_HOST")
    defer os.Unsetenv("TEST_DB_PASS")
    defer os.Unsetenv("TEST_SAMPLE_RATE")
    
    config := `
receivers:
  postgresql:
    endpoint: ${TEST_DB_HOST}:5432
    password: ${TEST_DB_PASS}

processors:
  adaptive_sampler:
    default_sample_rate: ${TEST_SAMPLE_RATE}
`
    
    cfg := parseConfig(t, config)
    
    assert.Equal(t, "testhost:5432", cfg.Receivers.PostgreSQL.Endpoint)
    assert.Equal(t, "testpass", cfg.Receivers.PostgreSQL.Password)
    assert.Equal(t, 0.5, cfg.Processors.AdaptiveSampler.DefaultSampleRate)
}
```

## Test Helpers

### Test Fixtures

```go
// fixtures/metrics.go
func GenerateTestMetrics(count int) pmetric.Metrics {
    md := pmetric.NewMetrics()
    rm := md.ResourceMetrics().AppendEmpty()
    sm := rm.ScopeMetrics().AppendEmpty()
    
    for i := 0; i < count; i++ {
        metric := sm.Metrics().AppendEmpty()
        metric.SetName(fmt.Sprintf("test.metric.%d", i))
        metric.SetEmptyGauge()
        
        dp := metric.Gauge().DataPoints().AppendEmpty()
        dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
        dp.SetDoubleValue(rand.Float64() * 100)
        
        // Add random attributes
        dp.Attributes().PutStr("host", fmt.Sprintf("host-%d", i%5))
        dp.Attributes().PutInt("duration_ms", int64(rand.Intn(5000)))
    }
    
    return md
}
```

### Test Environment

```go
type TestEnvironment struct {
    PostgresContainer testcontainers.Container
    MySQLContainer    testcontainers.Container
    Collector         *TestCollector
    ExportBuffer      *bytes.Buffer
}

func StartTestEnvironment(t *testing.T) *TestEnvironment {
    ctx := context.Background()
    
    // Start PostgreSQL
    postgresReq := testcontainers.ContainerRequest{
        Image:        "postgres:15",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
        },
        WaitingFor: wait.ForListeningPort("5432/tcp"),
    }
    
    postgres, err := testcontainers.GenericContainer(ctx,
        testcontainers.GenericContainerRequest{
            ContainerRequest: postgresReq,
            Started:          true,
        })
    require.NoError(t, err)
    
    // Similar for MySQL...
    
    return &TestEnvironment{
        PostgresContainer: postgres,
        // ...
    }
}
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run unit tests
        run: make test-unit
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run integration tests
        env:
          TEST_POSTGRES_HOST: localhost
          TEST_POSTGRES_PORT: 5432
        run: make test-integration
```

## Test Coverage

### Coverage Requirements

- Overall: 80% minimum
- Critical paths: 90% minimum
- New code: 85% minimum

### Coverage Commands

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out

# Check coverage threshold
go-coverage-threshold -t 80 coverage.out

# Package-specific coverage
go test -cover ./processors/adaptivesampler/
```

### Coverage Report

```
github.com/database-intelligence-mvp/processors/adaptivesampler    85.2%
github.com/database-intelligence-mvp/processors/circuitbreaker     82.7%
github.com/database-intelligence-mvp/processors/planattributeextractor  78.4%
github.com/database-intelligence-mvp/processors/verification       88.1%
github.com/database-intelligence-mvp/internal/health              91.3%
github.com/database-intelligence-mvp/internal/ratelimit           79.5%
```

---

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025