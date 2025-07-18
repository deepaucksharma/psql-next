# MySQL Monitoring E2E Tests

This directory contains end-to-end (E2E) and integration tests for MySQL monitoring with OpenTelemetry.

## Test Structure

```
tests/
├── e2e/                    # End-to-end tests
│   ├── framework/          # Test framework and utilities
│   │   ├── environment.go  # Test environment setup
│   │   └── validator.go    # Metric validation utilities
│   └── mysql_validation_test.go  # Main E2E validation tests
├── integration/            # Integration tests
│   └── mysql_container_test.go   # Container-based integration tests
├── fixtures/               # Test configurations and data
│   └── test-config.yaml    # Test OTel collector config
├── go.mod                  # Go module definition
└── README.md              # This file
```

## Running Tests

### Prerequisites

- Go 1.22 or later
- Docker and Docker Compose
- MySQL client (for validation scripts)

### Quick Start

```bash
# Run all tests
make test

# Run specific test suites
make test-unit
make test-integration
make test-e2e

# Validate metrics collection
make validate-metrics
```

### Using Test Scripts

```bash
# Run all tests with detailed output
./scripts/run-tests.sh all

# Run only integration tests
./scripts/run-tests.sh integration

# Run E2E tests
./scripts/run-tests.sh e2e
```

## Test Types

### E2E Tests (`e2e/mysql_validation_test.go`)

End-to-end tests validate the complete monitoring pipeline:
- MySQL metrics collection via OpenTelemetry
- Metric accuracy and completeness
- Performance Schema integration
- InnoDB metrics
- Replication metrics (when configured)
- Query performance metrics

### Integration Tests (`integration/mysql_container_test.go`)

Integration tests use Testcontainers to:
- Spin up isolated MySQL instances
- Test different MySQL versions
- Validate collector configuration
- Test workload generation and metric collection

### Framework Components

#### Test Environment (`framework/environment.go`)
- Manages MySQL connections
- Creates test data
- Generates known workloads
- Handles environment configuration

#### Metric Validator (`framework/validator.go`)
- Validates metric existence
- Checks metric values with tolerance
- Validates MySQL-specific metrics
- Parses Prometheus format output

## Environment Variables

Tests use these environment variables (with defaults):

```bash
MYSQL_HOST=localhost          # MySQL host
MYSQL_PORT=3306              # MySQL port
MYSQL_USER=root              # MySQL username
MYSQL_PASSWORD=rootpassword  # MySQL password
MYSQL_DATABASE=test          # Database name
PROMETHEUS_URL=http://localhost:9090/metrics  # Metrics endpoint
```

## Test Configuration

The test configuration (`fixtures/test-config.yaml`) includes:
- MySQL receiver configuration
- Custom SQL queries for additional metrics
- Memory limiting and batching
- Multiple exporters (Prometheus, File, Debug)

## Writing New Tests

### Adding E2E Tests

```go
func (s *MySQLValidationTestSuite) TestNewMetric() {
    s.Run("Description of test", func() {
        // Validate metric exists
        exists, err := s.validator.ValidateMetricExists("mysql_new_metric")
        s.NoError(err)
        s.True(exists, "New metric should exist")
        
        // Validate metric value
        result := s.validator.ValidateMetricValue("mysql_new_metric", expectedValue)
        s.True(result.Passed, "New metric validation failed")
    })
}
```

### Adding Integration Tests

```go
func (s *MySQLContainerTestSuite) TestNewFeature() {
    s.Run("Test new feature", func() {
        // Setup test data
        _, err := s.db.Exec("CREATE TABLE test_feature (...)")
        s.NoError(err)
        
        // Generate workload
        // ...
        
        // Validate results
        // ...
    })
}
```

## Troubleshooting

### Common Issues

1. **Docker not running**
   ```bash
   # Start Docker daemon
   sudo systemctl start docker  # Linux
   # Or open Docker Desktop on macOS/Windows
   ```

2. **MySQL connection failed**
   ```bash
   # Check MySQL is running
   docker ps | grep mysql
   
   # Check connection parameters
   mysql -h localhost -u root -p
   ```

3. **Metrics not found**
   ```bash
   # Check collector logs
   docker logs otel-collector
   
   # Verify Prometheus endpoint
   curl http://localhost:9090/metrics
   ```

## CI/CD Integration

These tests are designed to run in CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run MySQL E2E Tests
  run: |
    make docker-up
    make test-e2e
    make validate-metrics
```

## Contributing

When adding new tests:
1. Follow the existing test patterns
2. Add appropriate documentation
3. Ensure tests are idempotent
4. Clean up resources properly
5. Use meaningful test descriptions