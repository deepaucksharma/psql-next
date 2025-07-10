# End-to-End Tests for Database Intelligence

This directory contains comprehensive end-to-end tests that verify the complete functionality of the Database Intelligence system with real databases and New Relic integration.

## Overview

The e2e tests validate:
- Database connectivity and metrics collection (PostgreSQL & MySQL)
- All 7 custom processors functionality
- Query plan extraction and analysis
- PII detection and security features
- High-volume performance handling
- Data accuracy between source databases and NRDB
- Error tracking and recovery
- Custom attributes and tags
- Full pipeline integration

## Prerequisites

1. **Docker & Docker Compose** - For running test databases
2. **Go 1.21+** - For running tests
3. **New Relic Account** (optional but recommended)
   - License Key for data export
   - Account ID and API Key for verification

## Quick Start

```bash
# Run all e2e tests
./run_e2e_tests.sh

# Run specific test suite
./run_e2e_tests.sh comprehensive

# Run with New Relic verification
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"
export NEW_RELIC_API_KEY="your-api-key"
./run_e2e_tests.sh verification
```

## Test Suites

### 1. Comprehensive E2E Test (`comprehensive_e2e_test.go`)
Complete integration test covering:
- Basic metrics collection
- All processor verification
- Query plan extraction
- PII detection
- High volume performance
- MySQL integration
- Data accuracy
- Failure recovery

### 2. New Relic Verification (`newrelic_verification_test.go`)
Validates data accuracy in NRDB:
- PostgreSQL metrics accuracy
- Query performance metrics
- Query plan tracking
- Error and exception tracking
- Custom attributes and tags
- Data completeness over time

### 3. Adapter Integration (`adapter_integration_test.go`)
Tests all adapters:
- PostgreSQL receiver
- MySQL receiver
- SQLQuery receiver
- All processors pipeline
- Exporter integration
- NRI exporter
- ASH receiver
- Enhanced SQL receiver

### 4. Database to NRDB Verification (`database_to_nrdb_verification_test.go`)
Comprehensive data verification:
- Checksum-based integrity
- Timestamp accuracy
- Attribute preservation
- Extreme values handling
- NULL and empty values
- Special SQL types
- Query plan accuracy
- Plan change detection

## Configuration

### Environment Variables

```bash
# PostgreSQL Configuration
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres
export POSTGRES_DB=testdb

# MySQL Configuration
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=root
export MYSQL_DB=testdb
export MYSQL_ENABLED=true

# New Relic Configuration
export NEW_RELIC_LICENSE_KEY=your-license-key
export NEW_RELIC_ACCOUNT_ID=your-account-id
export NEW_RELIC_API_KEY=your-api-key
export NEW_RELIC_OTLP_ENDPOINT=otlp.nr-data.net:4317

# Test Configuration
export TEST_ENV=local
export COVERAGE_ENABLED=true
export KEEP_INFRASTRUCTURE=false
```

### Test Configuration File

See `e2e-test-config.yaml` for detailed test configuration including:
- Test suite parameters
- Performance baselines
- Security settings
- Test data scales
- Workload patterns

## Running Tests

### Run All Tests
```bash
./run_e2e_tests.sh all
```

### Run Specific Suite
```bash
# Comprehensive tests
./run_e2e_tests.sh comprehensive

# New Relic verification
./run_e2e_tests.sh verification

# Adapter tests
./run_e2e_tests.sh adapters

# Database verification
./run_e2e_tests.sh database

# Performance tests
./run_e2e_tests.sh performance
```

### Run with Coverage
```bash
COVERAGE_ENABLED=true ./run_e2e_tests.sh
```

### Keep Infrastructure Running
```bash
KEEP_INFRASTRUCTURE=true ./run_e2e_tests.sh
```

### Custom Database Connection
```bash
POSTGRES_HOST=remote-db.example.com \
POSTGRES_PORT=5433 \
POSTGRES_PASSWORD=secure-pass \
./run_e2e_tests.sh
```

## Test Framework

### Components

1. **TestEnvironment** (`framework/test_environment.go`)
   - Manages database connections
   - Handles configuration
   - Provides cleanup utilities

2. **NRDBClient** (`framework/nrdb_client.go`)
   - Queries New Relic Database
   - Verifies metrics and logs
   - Compares data accuracy

3. **TestCollector** (`framework/test_collector.go`)
   - Manages collector lifecycle
   - Updates configuration
   - Retrieves logs

4. **TestDataGenerator** (`framework/test_utils.go`)
   - Generates test schemas
   - Populates test data
   - Simulates workloads
   - Cleanup utilities

## Writing New Tests

### Example Test Structure

```go
func (s *YourTestSuite) TestNewFeature() {
    s.T().Log("Testing new feature...")
    
    // Setup test data
    err := s.setupTestData()
    require.NoError(s.T(), err)
    
    // Perform operations
    s.performOperations()
    
    // Wait for collection
    time.Sleep(65 * time.Second)
    
    // Verify in NRDB
    ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
    defer cancel()
    
    err = s.nrdb.VerifyMetric(ctx, "your.metric", map[string]interface{}{
        "attribute": "value",
    }, "5 minutes ago")
    assert.NoError(s.T(), err)
    
    // Cleanup
    s.cleanupTestData()
}
```

## Troubleshooting

### Common Issues

1. **Docker containers not starting**
   ```bash
   # Check container status
   docker-compose -f docker-compose.yml ps
   
   # View logs
   docker-compose -f docker-compose.yml logs postgres
   ```

2. **Collector build fails**
   ```bash
   # Manually build collector
   cd ../../core/cmd/collector
   go build -o collector .
   ```

3. **Tests timeout**
   - Increase timeout: `TEST_TIMEOUT=60m ./run_e2e_tests.sh`
   - Check network connectivity
   - Verify New Relic endpoints

4. **No data in NRDB**
   - Verify license key is correct
   - Check collector logs for export errors
   - Ensure correct endpoint configuration

### Debug Mode

```bash
# Enable debug logging
LOG_LEVEL=debug ./run_e2e_tests.sh

# Keep infrastructure for debugging
KEEP_INFRASTRUCTURE=true ./run_e2e_tests.sh

# Check collector logs
docker logs e2e-otel-collector
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: Run E2E Tests
  env:
    NEW_RELIC_LICENSE_KEY: ${{ secrets.NEW_RELIC_LICENSE_KEY }}
    NEW_RELIC_ACCOUNT_ID: ${{ secrets.NEW_RELIC_ACCOUNT_ID }}
    NEW_RELIC_API_KEY: ${{ secrets.NEW_RELIC_API_KEY }}
  run: |
    cd tests/e2e
    ./run_e2e_tests.sh all
```

### Jenkins

```groovy
stage('E2E Tests') {
    environment {
        NEW_RELIC_LICENSE_KEY = credentials('new-relic-license')
        NEW_RELIC_ACCOUNT_ID = credentials('new-relic-account')
        NEW_RELIC_API_KEY = credentials('new-relic-api')
    }
    steps {
        sh 'cd tests/e2e && ./run_e2e_tests.sh all'
    }
}
```

## Performance Baselines

Expected performance metrics:
- PostgreSQL query p95: < 50ms
- MySQL query p95: < 45ms
- Collector processing p95: < 10ms
- Export latency p95: < 100ms
- Error rate: < 1%
- Drop rate: < 0.1%

## Contributing

When adding new tests:
1. Follow existing test patterns
2. Add appropriate timeouts
3. Clean up test data
4. Document new test cases
5. Update this README

## License

See project LICENSE file.