# Testing Guide

Comprehensive testing strategy for Database Intelligence with OpenTelemetry.

## 🧪 Testing Overview

This project uses a multi-layered testing approach:

1. **Unit Tests**: Component-level testing
2. **Integration Tests**: Cross-component testing  
3. **E2E Tests**: Full pipeline validation
4. **Performance Tests**: Load and scale testing
5. **Configuration Tests**: YAML validation

## 🚀 Quick Test Run

```bash
# Run all tests
make test

# Run specific test suites
make test-unit          # Unit tests only
make test-integration   # Integration tests
make test-e2e          # End-to-end tests
make test-performance  # Performance tests

# Run tests with coverage
make test-coverage
```

## 🔬 Unit Testing

### Running Unit Tests

```bash
# All unit tests
go test ./tests/unit/...

# Specific component
go test ./tests/unit/receivers/

# With verbose output
go test -v ./tests/unit/receivers/

# With coverage
go test -cover ./tests/unit/...
```

### Example Unit Test

```go
func TestPostgreSQLReceiver(t *testing.T) {
    cfg := &Config{
        Endpoint: "postgresql://localhost:5432/test",
        Username: "test",
        Password: "test",
        CollectionInterval: 30 * time.Second,
    }
    
    receiver := NewPostgreSQLReceiver(cfg, zap.NewNop())
    
    // Test configuration validation
    err := receiver.Validate()
    assert.NoError(t, err)
    
    // Test metric collection (mocked)
    metrics, err := receiver.CollectMetrics(context.Background())
    assert.NoError(t, err)
    assert.NotEmpty(t, metrics)
}
```

## 🔗 Integration Testing

### Test Environment Setup

```bash
# Start test databases
docker-compose -f tests/docker-compose.test.yml up -d

# Wait for databases to be ready
./tests/wait-for-databases.sh

# Run integration tests
go test ./tests/integration/...

# Cleanup
docker-compose -f tests/docker-compose.test.yml down
```

## 🎯 End-to-End Testing

### E2E Test Framework

Located in `tests/e2e/`, the framework provides:

- Real database workload generation
- OTLP endpoint mocking
- Metric validation
- Performance measurement

### Running E2E Tests

```bash
# Setup test environment
cd tests/e2e
make setup

# Run all E2E tests
make test

# Run specific test suite
make test-postgresql
make test-mysql
make test-newrelic-integration

# Run with real New Relic (requires API key)
NEW_RELIC_LICENSE_KEY=your_key make test-newrelic
```

### E2E Test Scenarios

1. **Basic Metrics Collection**:
   ```bash
   go test -run TestBasicMetricsCollection
   ```

2. **High-Load Scenarios**:
   ```bash
   go test -run TestHighLoadCollection
   ```

3. **Error Handling**:
   ```bash
   go test -run TestDatabaseConnectionLoss
   ```

4. **Configuration Validation**:
   ```bash
   go test -run TestConfigurationValidation
   ```

## 📊 Performance Testing

### Load Testing

```bash
# Run load tests
cd tests/performance
go test -run TestLoadPerformance

# Specific load scenarios
go test -run TestHighVolumeMetrics
go test -run TestMemoryUsage
go test -run TestCPUUsage
```

### Benchmark Tests

```go
func BenchmarkMetricProcessing(b *testing.B) {
    processor := setupTestProcessor()
    metrics := generateTestMetrics(1000)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        processor.ProcessMetrics(context.Background(), metrics)
    }
}
```

## ⚙️ Configuration Testing

### YAML Validation

```bash
# Validate all configuration files
make validate-configs

# Validate specific config
make validate-config CONFIG=configs/examples/enhanced-mode-full.yaml
```

## 🔍 Test Utilities

### Test Environment

```go
type TestEnvironment struct {
    PostgresContainer testcontainers.Container
    MySQLContainer    testcontainers.Container
    CollectorProcess  *exec.Cmd
    TempDir          string
}

func NewTestEnvironment() *TestEnvironment {
    env := &TestEnvironment{}
    env.setupDatabases()
    env.setupCollector()
    return env
}
```

### Mock Exporters

```go
type MockExporter struct {
    ReceivedMetrics []pdata.Metrics
    mutex          sync.Mutex
}

func (m *MockExporter) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    m.ReceivedMetrics = append(m.ReceivedMetrics, md)
    return nil
}
```

## 🚨 Testing Best Practices

### 1. Test Isolation

```go
func TestWithIsolation(t *testing.T) {
    // Each test gets its own database schema
    schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
    db := setupTestDatabase()
    _, err := db.Exec(fmt.Sprintf("CREATE SCHEMA %s", schema))
    require.NoError(t, err)
    defer db.Exec(fmt.Sprintf("DROP SCHEMA %s CASCADE", schema))
}
```

### 2. Deterministic Tests

```go
func TestDeterministic(t *testing.T) {
    // Use fixed test data
    testMetrics := []Metric{
        {Name: "test.metric1", Value: 100},
        {Name: "test.metric2", Value: 200},
    }
    
    // Seed random generators
    rand.Seed(12345)
    
    result := processMetrics(testMetrics)
    expectedResult := []ProcessedMetric{
        {Name: "test.metric1", ProcessedValue: 110},
        {Name: "test.metric2", ProcessedValue: 220},
    }
    
    assert.Equal(t, expectedResult, result)
}
```

## 🔧 Continuous Integration

### GitHub Actions

```yaml
name: Test Suite
on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: '1.21'
      
      - name: Run unit tests
        run: make test-unit
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v3
      - name: Run integration tests
        run: make test-integration
```

## 🎯 Test Coverage Goals

| Component | Target Coverage | Current Status |
|-----------|----------------|----------------|
| Receivers | 90% | ✅ Achieved |
| Processors | 85% | ✅ Achieved |
| Exporters | 80% | ✅ Achieved |
| E2E Tests | 75% | 🔄 In Progress |

## 🐛 Debugging Tests

### Running Tests in Debug Mode

```bash
# Enable debug logging
OTEL_LOG_LEVEL=debug go test -v ./tests/...

# Run specific test with debugger
dlv test ./tests/e2e/ -- -test.run TestSpecificTest

# Run with race detection
go test -race ./tests/...
```

This testing guide provides comprehensive coverage of all testing aspects for the Database Intelligence project.