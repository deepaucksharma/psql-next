# MySQL Wait-Based Monitoring - E2E Test Guide

This guide provides comprehensive instructions for running end-to-end tests that validate the entire MySQL wait-based monitoring pipeline from data collection to New Relic dashboards.

## Prerequisites

### 1. Environment Setup
Ensure you have the following installed:
- Go 1.19 or later
- MySQL 8.0+ with Performance Schema enabled
- Docker and Docker Compose (optional, for containerized setup)
- Access to New Relic account with OTLP endpoint access

### 2. New Relic Credentials
The following environment variables must be set (available in `.env` file):
```bash
NEW_RELIC_LICENSE_KEY=<your-license-key>
NEW_RELIC_ACCOUNT_ID=<your-account-id>
NEW_RELIC_API_KEY=<your-api-key>
NEW_RELIC_OTLP_ENDPOINT=otlp.nr-data.net:4317
```

### 3. MySQL Configuration
Ensure MySQL is running with:
- Performance Schema enabled
- Wait instruments enabled
- Monitor user created with appropriate permissions

## Quick Start

### Basic Validation
Run basic connectivity tests first:
```bash
cd tests
./run_basic_test.sh
```

This validates:
- MySQL connectivity
- Performance Schema configuration
- New Relic credentials
- Basic workload generation

### Running All E2E Tests
```bash
cd tests/e2e
./run_tests_with_env.sh all
```

## Test Suites

### 1. Comprehensive E2E Validation (`comprehensive_validation_test.go`)
Validates the entire pipeline:
- MySQL setup and Performance Schema
- Workload generation (IO, Lock, CPU intensive)
- Collector metrics validation
- NRDB data presence
- Dashboard data availability
- Advisory generation
- End-to-end latency

**Run individually:**
```bash
./run_tests_with_env.sh comprehensive
```

### 2. Dashboard Coverage Tests (`dashboard_coverage_test.go`)
Ensures all metrics are visualized:
- Metric usage validation (90%+ coverage required)
- Widget data validation
- Advisory accuracy testing
- Data quality checks

**Run individually:**
```bash
./run_tests_with_env.sh coverage
```

### 3. Performance Impact Tests (`performance_validation_test.go`)
Validates monitoring overhead:
- CPU overhead (<1%)
- Memory usage (<384MB)
- Query overhead (<0.5ms)
- Collection latency (<5ms)
- Scalability under load

**Run individually:**
```bash
./run_tests_with_env.sh performance
```

### 4. Data Generation Tests
Tests realistic workload generation:
- IO-intensive patterns
- Lock contention scenarios
- CPU-intensive queries
- Slow query generation
- Advisory triggers

**Run individually:**
```bash
./run_tests_with_env.sh data-gen
```

## Test Data Generation

The `data_generator.go` provides realistic MySQL workload patterns:

### Workload Types
1. **IO-Intensive**: Full table scans, missing indexes
2. **Lock-Intensive**: Transaction conflicts, deadlocks
3. **CPU-Intensive**: Complex aggregations, joins
4. **Slow Queries**: Intentionally slow operations
5. **Mixed Workload**: Combination of all patterns

### Usage Example
```go
// Create data generator
dg, err := NewDataGenerator(mysqlDSN)

// Setup test schema
err = dg.SetupTestSchema()

// Populate base data
err = dg.PopulateBaseData()

// Generate specific workload
dg.GenerateIOIntensiveWorkload(5*time.Minute, 10) // duration, concurrency

// Generate mixed workload
dg.GenerateMixedWorkload(10*time.Minute, 50)

// Get metrics
metrics := dg.GetMetrics()
```

## Expected Test Results

### Successful Test Output
```
=== MySQL Wait-Based Monitoring E2E Test Suite ===
✓ All required environment variables loaded

▶ Running TestComprehensiveE2EValidation...
  ✓ Stage1_ValidateMySQL passed
  ✓ Stage2_GenerateTestWorkload passed
  ✓ Stage3_ValidateCollectorMetrics passed
  ✓ Stage4_ValidateNRDBData passed
  ✓ Stage5_ValidateDashboards passed
  ✓ Stage6_ValidateAdvisories passed
  ✓ Stage7_ValidateEndToEndLatency passed (latency: 42s)

=== Test Summary ===
✓ All tests passed!
```

### Key Metrics Validated
1. **Wait Profile Metrics**: 95%+ coverage
2. **Advisory Generation**: All types detected
3. **Data Quality**: No negative values, percentages within bounds
4. **Performance Impact**: <1% CPU, <384MB memory
5. **E2E Latency**: <90 seconds

## Troubleshooting

### Common Issues

1. **MySQL Connection Failed**
   - Check MySQL is running: `mysql -u root -p`
   - Verify Performance Schema: `SELECT @@performance_schema;`
   - Check monitor user exists

2. **New Relic API Errors**
   - Verify API key is valid
   - Check account ID is correct
   - Ensure OTLP endpoint is accessible

3. **No Metrics in NRDB**
   - Check collectors are running
   - Verify gateway configuration
   - Check for errors in collector logs

4. **Test Timeouts**
   - Increase timeout: `go test -timeout 30m`
   - Check network connectivity
   - Verify New Relic ingestion is working

### Debug Mode
Enable verbose output:
```bash
export DEBUG_VERBOSITY=detailed
./run_tests_with_env.sh all
```

## Test Reports

Test results are saved to:
- `test-reports/e2e-test-report-<timestamp>.html` - HTML report
- `test-reports/e2e-test-results-<timestamp>.json` - JSON results
- `test-reports/e2e-test-<timestamp>.log` - Detailed logs

View the HTML report in a browser for a comprehensive summary.

## Continuous Integration

For CI/CD pipelines:
```yaml
- name: Run E2E Tests
  env:
    NEW_RELIC_LICENSE_KEY: ${{ secrets.NEW_RELIC_LICENSE_KEY }}
    NEW_RELIC_ACCOUNT_ID: ${{ secrets.NEW_RELIC_ACCOUNT_ID }}
    NEW_RELIC_API_KEY: ${{ secrets.NEW_RELIC_API_KEY }}
  run: |
    cd tests/e2e
    ./run_tests_with_env.sh all
```

## Advanced Testing

### Custom Test Scenarios
Create custom test scenarios by modifying workload parameters:
```go
// Custom high-concurrency test
dg.GenerateLockIntensiveWorkload(
    duration:    10*time.Minute,
    concurrency: 100,  // High concurrency for stress testing
)
```

### Performance Baselines
Adjust performance baselines in `performance_validation_test.go`:
```go
baseline := PerformanceBaseline{
    MaxCPUOverhead:     1.0,  // 1% CPU overhead
    MaxMemoryMB:        384,  // 384MB memory  
    MaxLatencyMs:       5,    // 5ms collection latency
    MaxQueryOverheadMs: 0.5,  // 0.5ms per query
}
```

## Next Steps

After successful E2E tests:
1. Deploy to production with monitoring
2. Set up alerts based on advisory metrics
3. Create custom dashboards for your workload
4. Fine-tune collection intervals and sampling
5. Document your specific use cases

For questions or issues, refer to the main project documentation or file an issue.