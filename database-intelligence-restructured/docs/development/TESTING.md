# Testing Guide

Comprehensive testing guide for Database Intelligence.

## Table of Contents
- [Test Structure](#test-structure)
- [Running Tests](#running-tests)
- [Unit Testing](#unit-testing)
- [Integration Testing](#integration-testing)
- [E2E Testing](#e2e-testing)
- [Load Testing](#load-testing)
- [Test Tools](#test-tools)
- [Writing Tests](#writing-tests)

## Test Structure

```
tests/
├── unit/              # Unit tests
├── integration/       # Integration tests
├── e2e/              # End-to-end tests
├── load/             # Load/performance tests
├── fixtures/         # Test data files
└── helpers/          # Test utilities
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

# Run E2E tests
make test-e2e

# Run with coverage
make test-coverage

# Run specific package tests
go test -v ./components/receivers/ash/...

# Run specific test
go test -v -run TestASHReceiver ./components/receivers/ash/
```

### Test Environment Setup

```bash
# Start test PostgreSQL
docker run -d \
  --name postgres-test \
  -e POSTGRES_PASSWORD=test \
  -p 5433:5432 \
  postgres:15

# Create test database
docker exec postgres-test psql -U postgres -c "CREATE DATABASE test_db;"

# Run test data generator
go run tools/postgres-test-generator/main.go \
  -host=localhost \
  -port=5433 \
  -database=test_db
```

## Unit Testing

### Component Testing

```go
// ash_receiver_test.go
func TestASHReceiver(t *testing.T) {
    cfg := &Config{
        Datasource: "host=localhost port=5433 dbname=test_db",
        CollectionInterval: time.Second,
    }
    
    receiver := newASHReceiver(cfg, zap.NewNop())
    require.NotNil(t, receiver)
    
    // Test configuration validation
    err := cfg.Validate()
    assert.NoError(t, err)
    
    // Test metric generation
    metrics := receiver.generateMetrics()
    assert.Greater(t, len(metrics), 0)
}
```

### Mock Testing

```go
// Use testify/mock for dependencies
type mockDatabase struct {
    mock.Mock
}

func (m *mockDatabase) Query(ctx context.Context, query string) (*sql.Rows, error) {
    args := m.Called(ctx, query)
    return args.Get(0).(*sql.Rows), args.Error(1)
}

func TestWithMockDB(t *testing.T) {
    mockDB := new(mockDatabase)
    mockDB.On("Query", mock.Anything, mock.Anything).Return(mockRows(), nil)
    
    receiver := &ashReceiver{db: mockDB}
    err := receiver.collectMetrics()
    assert.NoError(t, err)
    mockDB.AssertExpectations(t)
}
```

### Table-Driven Tests

```go
func TestSamplingDecision(t *testing.T) {
    tests := []struct {
        name     string
        sample   *ASHSample
        rate     float64
        expected bool
    }{
        {
            name: "always sample blocked",
            sample: &ASHSample{BlockingPID: 123},
            rate: 0.1,
            expected: true,
        },
        {
            name: "sample based on rate",
            sample: &ASHSample{State: "active"},
            rate: 1.0,
            expected: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            sampler := NewAdaptiveSampler(tt.rate)
            result := sampler.ShouldSample(tt.sample)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

## Integration Testing

### Database Integration

```go
// Requires real PostgreSQL instance
func TestDatabaseIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    db, err := sql.Open("postgres", getTestDSN())
    require.NoError(t, err)
    defer db.Close()
    
    // Test actual queries
    rows, err := db.Query("SELECT * FROM pg_stat_activity")
    require.NoError(t, err)
    defer rows.Close()
    
    count := 0
    for rows.Next() {
        count++
    }
    assert.Greater(t, count, 0)
}
```

### Collector Integration

```go
func TestCollectorIntegration(t *testing.T) {
    // Create test collector
    factories, err := testcomponents.NewDefaultFactories()
    require.NoError(t, err)
    
    cfg := testutil.CreateDefaultConfig()
    collector, err := testutil.NewCollector(factories, cfg)
    require.NoError(t, err)
    
    // Start collector
    err = collector.Start(context.Background())
    require.NoError(t, err)
    defer collector.Shutdown()
    
    // Verify metrics are collected
    time.Sleep(5 * time.Second)
    metrics := collector.GetMetrics()
    assert.Greater(t, len(metrics), 0)
}
```

## E2E Testing

### Full Pipeline Test

```go
func TestE2EPipeline(t *testing.T) {
    // Start all components
    compose := testutil.NewDockerCompose("docker-compose-test.yaml")
    err := compose.Up()
    require.NoError(t, err)
    defer compose.Down()
    
    // Wait for services
    err = compose.WaitForService("collector", 30*time.Second)
    require.NoError(t, err)
    
    // Generate test load
    generator := testutil.NewLoadGenerator()
    generator.Run(t, 60*time.Second)
    
    // Verify metrics in New Relic
    client := newrelic.NewClient(os.Getenv("NEW_RELIC_LICENSE_KEY"))
    metrics, err := client.QueryMetrics(
        "SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql%'",
    )
    require.NoError(t, err)
    assert.Greater(t, metrics.Count, 100)
}
```

### Metric Validation

```go
func TestMetricValidation(t *testing.T) {
    expectedMetrics := []string{
        "postgresql.backends",
        "postgresql.commits",
        "postgresql.rollbacks",
        "postgresql.deadlocks",
        // ... all expected metrics
    }
    
    // Collect actual metrics
    actualMetrics := collectMetricsForDuration(5 * time.Minute)
    
    // Verify all expected metrics present
    for _, expected := range expectedMetrics {
        assert.Contains(t, actualMetrics, expected, 
            "Missing metric: %s", expected)
    }
}
```

## Load Testing

### Performance Benchmarks

```go
func BenchmarkASHCollection(b *testing.B) {
    receiver := setupTestReceiver(b)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        receiver.collectASHSamples()
    }
}

func BenchmarkMetricProcessing(b *testing.B) {
    processor := setupTestProcessor(b)
    metrics := generateTestMetrics(1000)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        processor.ProcessMetrics(metrics)
    }
}
```

### Load Test Scenarios

```go
func TestHighLoadScenario(t *testing.T) {
    // Configure for high load
    cfg := &Config{
        CollectionInterval: 100 * time.Millisecond,
        BufferSize: 100000,
    }
    
    collector := NewCollector(cfg)
    collector.Start()
    defer collector.Stop()
    
    // Generate high load
    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            generateLoad(1000)
        }()
    }
    wg.Wait()
    
    // Verify no data loss
    stats := collector.GetStats()
    assert.Equal(t, 100000, stats.ProcessedCount)
    assert.Zero(t, stats.DroppedCount)
}
```

## Test Tools

### PostgreSQL Test Generator

```bash
# Generate all metric types
go run tools/postgres-test-generator/main.go \
  -workers=20 \
  -deadlocks=true \
  -temp-files=true \
  -blocking=true
```

### Load Generator

```bash
# Run different load patterns
go run tools/load-generator/main.go -pattern=simple -qps=10
go run tools/load-generator/main.go -pattern=complex -qps=50
go run tools/load-generator/main.go -pattern=stress -qps=1000
```

### Metric Verification

```bash
# Verify all metrics are collected
./scripts/verify-metrics.sh

# Generate validation queries
./scripts/validate-metrics-e2e.sh
```

## Writing Tests

### Test Guidelines

1. **Test Naming**: Use descriptive names
   ```go
   func TestASHReceiver_CollectMetrics_WithBlockedSessions(t *testing.T)
   ```

2. **Test Structure**: Arrange-Act-Assert
   ```go
   func TestSomething(t *testing.T) {
       // Arrange
       receiver := setupReceiver()
       
       // Act
       result := receiver.Process()
       
       // Assert
       assert.NoError(t, result)
   }
   ```

3. **Test Isolation**: Each test should be independent
   ```go
   func TestWithIsolation(t *testing.T) {
       // Create fresh test environment
       db := createTestDB(t)
       defer cleanupTestDB(db)
       
       // Test logic here
   }
   ```

4. **Error Testing**: Test error conditions
   ```go
   func TestErrorHandling(t *testing.T) {
       receiver := &ashReceiver{
           db: nil, // Force error
       }
       
       err := receiver.Start()
       assert.Error(t, err)
       assert.Contains(t, err.Error(), "database connection")
   }
   ```

### Test Helpers

```go
// Test database helper
func setupTestDB(t *testing.T) *sql.DB {
    t.Helper()
    
    db, err := sql.Open("postgres", getTestDSN())
    require.NoError(t, err)
    
    t.Cleanup(func() {
        db.Close()
    })
    
    return db
}

// Test metrics helper
func assertMetricExists(t *testing.T, metrics []Metric, name string) {
    t.Helper()
    
    for _, m := range metrics {
        if m.Name == name {
            return
        }
    }
    t.Errorf("Metric %s not found", name)
}

// Test configuration helper
func createTestConfig() *Config {
    return &Config{
        Datasource: getTestDSN(),
        CollectionInterval: time.Second,
        Sampling: SamplingConfig{
            BaseRate: 1.0,
        },
    }
}
```

### Test Fixtures

```go
// Load test data
func loadTestFixture(t *testing.T, name string) []byte {
    t.Helper()
    
    data, err := os.ReadFile(filepath.Join("testdata", name))
    require.NoError(t, err)
    
    return data
}

// Create test samples
func createTestASHSamples(count int) []*ASHSample {
    samples := make([]*ASHSample, count)
    
    for i := 0; i < count; i++ {
        samples[i] = &ASHSample{
            Timestamp: time.Now(),
            SessionID: fmt.Sprintf("session-%d", i),
            State:     "active",
        }
    }
    
    return samples
}
```

## Test Coverage

### Generate Coverage Report

```bash
# Run with coverage
go test -coverprofile=coverage.out ./...

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

# View coverage in terminal
go tool cover -func=coverage.out
```

### Coverage Requirements

- Unit tests: >80% coverage
- Integration tests: Key paths covered
- E2E tests: Happy path + critical errors

## CI/CD Integration

### GitHub Actions

```yaml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run tests
        run: make test-coverage
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
```

### Pre-commit Hooks

```bash
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: go-test
        name: go test
        entry: go test ./...
        language: system
        pass_filenames: false
```

## Debugging Tests

### Verbose Output

```bash
# Run with verbose output
go test -v ./...

# Run with specific log level
LOG_LEVEL=debug go test -v ./...
```

### Test Timeouts

```go
func TestWithTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Test with timeout
    select {
    case result := <-doWork(ctx):
        assert.NotNil(t, result)
    case <-ctx.Done():
        t.Fatal("Test timed out")
    }
}
```

### Race Detection

```bash
# Run with race detector
go test -race ./...

# Fix data races
var mu sync.Mutex
mu.Lock()
defer mu.Unlock()
// Access shared data
```

## Best Practices

1. **Fast Tests**: Keep unit tests under 1 second
2. **Deterministic**: Avoid time-dependent tests
3. **Clear Failures**: Provide helpful error messages
4. **Test Data**: Use realistic test data
5. **Cleanup**: Always clean up resources
6. **Parallel Tests**: Use `t.Parallel()` where safe
7. **Skip Conditions**: Use `t.Skip()` appropriately
8. **Benchmarks**: Include performance benchmarks