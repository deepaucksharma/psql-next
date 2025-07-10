# Testing Guide

## Overview

The Database Intelligence project includes comprehensive testing at multiple levels:
- Unit tests for individual components
- Integration tests for component interactions
- End-to-end (E2E) tests for full system validation
- Performance and load testing

## Test Structure

```
tests/
├── benchmarks/        # Performance benchmarks
├── e2e/              # End-to-end tests
├── integration/      # Integration tests
├── performance/      # Load and stress tests
└── fixtures/         # Test data and configurations
```

## Running Tests

### All Tests
```bash
make test-all
```

### Unit Tests
```bash
make test-unit
```

### Integration Tests
```bash
make test-integration
```

### E2E Tests
```bash
make test-e2e
```

### Performance Tests
```bash
make test-performance
```

## E2E Testing

The E2E test suite validates the complete data pipeline from databases through the collector to exporters.

### Prerequisites
- Docker and Docker Compose
- Go 1.21+
- New Relic account (for New Relic export tests)

### Running E2E Tests

1. **Set up environment**:
   ```bash
   cp configs/templates/environment-template.env .env
   # Edit .env with your settings
   ```

2. **Start test environment**:
   ```bash
   docker-compose -f deployments/docker/compose/docker-compose.test.yaml up -d
   ```

3. **Run E2E tests**:
   ```bash
   cd tests/e2e
   go test -v ./...
   ```

### E2E Test Coverage

- Database connectivity (PostgreSQL, MySQL)
- Metric collection accuracy
- Query plan extraction
- PII detection and redaction
- Export to Prometheus
- Export to New Relic
- Error handling and recovery
- Performance under load

### Writing E2E Tests

Example E2E test:

```go
func TestPostgreSQLMetricsE2E(t *testing.T) {
    // Setup test environment
    env := framework.NewTestEnvironment(t)
    defer env.Cleanup()
    
    // Start collector
    collector := env.StartCollector("configs/examples/collector-e2e-test.yaml")
    
    // Generate database load
    env.GenerateLoad("postgresql", 100)
    
    // Verify metrics
    metrics := env.GetMetrics("postgresql.database.size")
    assert.Greater(t, len(metrics), 0)
}
```

## Integration Testing

Integration tests verify component interactions without external dependencies.

### Running Integration Tests

```bash
cd tests/integration
go test -v ./...
```

### Integration Test Scenarios

- Processor pipeline validation
- Configuration loading and validation
- Feature detection accuracy
- Multi-database support

## Performance Testing

Performance tests ensure the collector can handle production loads.

### Running Performance Tests

```bash
cd tests/performance
go test -bench=. -benchmem
```

### Performance Benchmarks

- Processor throughput
- Memory usage under load
- CPU utilization
- Network latency impact

## Test Configuration

Test configurations are stored in `tests/fixtures/configs/`:
- `e2e-minimal.yaml` - Minimal E2E test configuration
- `e2e-comprehensive.yaml` - Full feature test configuration
- `e2e-performance.yaml` - Performance test configuration

## Continuous Integration

All tests run automatically on:
- Pull requests
- Commits to main branch
- Nightly builds

See `.github/workflows/` for CI configuration.

## Troubleshooting Tests

### Common Issues

1. **Database connection failures**:
   - Ensure Docker containers are running
   - Check database credentials in .env
   - Verify network connectivity

2. **Metric verification failures**:
   - Allow time for metrics to be collected (usually 30s)
   - Check collector logs for errors
   - Verify exporters are configured correctly

3. **Performance test failures**:
   - Ensure sufficient system resources
   - Close other applications
   - Adjust performance thresholds if needed

### Debug Mode

Run tests with debug logging:
```bash
OTEL_LOG_LEVEL=debug go test -v ./...
```

### Test Reports

Test results are saved to `test-results/`:
- JUnit XML reports for CI
- Coverage reports
- Performance profiles
