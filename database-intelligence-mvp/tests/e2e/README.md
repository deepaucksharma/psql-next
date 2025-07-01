# End-to-End Tests

This directory contains comprehensive end-to-end tests for the Database Intelligence Collector, covering the complete data flow from PostgreSQL to New Relic Database (NRDB).

## Test Structure

### Test Files

1. **`plan_intelligence_test.go`** - Tests for Plan Intelligence features
   - Auto-explain log collection
   - Plan anonymization and PII protection
   - Regression detection
   - Circuit breaker protection
   - NRDB export validation

2. **`ash_test.go`** - Tests for Active Session History (ASH)
   - Session sampling at 1-second intervals
   - Wait event analysis and categorization
   - Blocking chain detection
   - Adaptive sampling under load
   - Query activity tracking
   - Time-window aggregation

3. **`integration_test.go`** - Full integration tests
   - Plan Intelligence + ASH correlation
   - Regression detection with wait analysis
   - Adaptive sampling under varying load
   - Circuit breaker integration
   - Memory pressure handling
   - Feature detection and graceful degradation

4. **`test_environment.go`** - Test infrastructure
   - PostgreSQL container management
   - Collector process lifecycle
   - Mock NRDB exporter
   - Metrics and logs collection
   - Test utilities

## Running Tests

### Prerequisites

- Docker installed and running
- Go 1.21+
- OpenTelemetry Collector binary in PATH
- Sufficient resources (4GB RAM, 2 CPU cores)

### Run All E2E Tests

```bash
# Run all E2E tests
go test -v ./tests/e2e/

# Run with timeout (recommended)
go test -v -timeout 30m ./tests/e2e/

# Run specific test suite
go test -v -run TestPlanIntelligenceE2E ./tests/e2e/
go test -v -run TestASHE2E ./tests/e2e/
go test -v -run TestFullIntegrationE2E ./tests/e2e/
```

### Run Individual Tests

```bash
# Test plan anonymization
go test -v -run TestPlanIntelligenceE2E/PlanAnonymization ./tests/e2e/

# Test blocking detection
go test -v -run TestASHE2E/BlockingDetection ./tests/e2e/

# Test memory pressure
go test -v -run TestFullIntegrationE2E/MemoryPressureHandling ./tests/e2e/
```

### Skip E2E Tests (for quick builds)

```bash
# Skip E2E tests using short mode
go test -short ./...
```

## Test Scenarios

### Plan Intelligence Tests

1. **Auto-Explain Log Collection**
   - Generates slow queries that trigger auto_explain
   - Verifies plan and execution time metrics
   - Validates query attribution

2. **Plan Anonymization**
   - Tests PII detection and redaction
   - Verifies email, SSN, credit card anonymization
   - Ensures plan structure preservation

3. **Regression Detection**
   - Creates index changes to trigger plan changes
   - Verifies statistical regression detection
   - Validates regression severity metrics

4. **Circuit Breaker**
   - Simulates auto_explain errors
   - Verifies circuit breaker activation
   - Tests graceful degradation

### ASH Tests

1. **Session Sampling**
   - Creates concurrent sessions with different states
   - Verifies session count metrics by state
   - Tests 1-second sampling interval

2. **Wait Event Analysis**
   - Generates various wait events (Lock, IO, CPU)
   - Verifies wait event categorization
   - Tests severity assignment

3. **Blocking Detection**
   - Creates explicit blocking chains
   - Verifies blocking/blocked session metrics
   - Tests chain depth detection

4. **Adaptive Sampling**
   - Creates high session counts
   - Verifies sampling rate adjustment
   - Tests priority sampling rules

5. **Query Activity Tracking**
   - Executes same query from multiple sessions
   - Verifies query-level metrics
   - Tests query duration tracking

### Integration Tests

1. **Plan + ASH Correlation**
   - Verifies query_id correlation between systems
   - Tests unified view of performance data

2. **Regression + Wait Analysis**
   - Creates scenarios with both plan changes and waits
   - Tests comprehensive problem detection

3. **Load Testing**
   - Tests with varying session counts (10-200)
   - Verifies adaptive behavior
   - Monitors resource usage

4. **Circuit Breaker Integration**
   - Tests multiple failure scenarios
   - Verifies protective mechanisms

5. **Memory Management**
   - Tests high cardinality scenarios
   - Verifies memory limiter effectiveness

6. **NRDB Validation**
   - Verifies complete metric export
   - Validates NRDB payload structure
   - Tests metric enrichment

## Test Environment

The test environment provides:

### PostgreSQL Container
- PostgreSQL 15 with required extensions
- Auto-explain pre-configured
- Test schema with sample data
- Log file access

### Mock Infrastructure
- Mock NRDB exporter endpoint
- Metrics and logs storage
- Health check endpoints

### Test Utilities
- Metric search and validation
- Log parsing helpers
- Database activity generators
- Failure simulation methods

## Test Data

### Schema
- `users` table with PII data
- `orders` table for join queries
- `ash_test_table` for ASH testing
- `metrics` table for mixed workloads

### Generated Activity
- Slow queries for plan collection
- Concurrent sessions for ASH
- Blocking chains
- Various wait events

## Debugging Tests

### Enable Debug Logging

```bash
# Set debug log level
export LOG_LEVEL=debug
go test -v -run TestName ./tests/e2e/
```

### Inspect Test Artifacts

```bash
# View PostgreSQL logs
docker logs <postgres-container-id>

# View collector logs
tail -f /tmp/test-logs/collector.log

# Check metrics endpoint
curl http://localhost:8888/metrics
```

### Common Issues

1. **Container Startup Timeout**
   - Increase wait timeout in test
   - Check Docker resource limits

2. **Collector Not Healthy**
   - Verify collector binary in PATH
   - Check port availability
   - Review configuration

3. **Missing Metrics**
   - Verify PostgreSQL extensions
   - Check auto_explain settings
   - Review log file permissions

## Contributing

When adding new E2E tests:

1. Follow existing test patterns
2. Use test environment utilities
3. Clean up resources properly
4. Document test scenarios
5. Consider test execution time

## CI/CD Integration

```yaml
# Example GitHub Actions workflow
name: E2E Tests
on: [push, pull_request]

jobs:
  e2e-tests:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.21'
    
    - name: Install collector
      run: |
        wget https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.88.0/otelcol_0.88.0_linux_amd64.tar.gz
        tar -xvf otelcol_0.88.0_linux_amd64.tar.gz
        sudo mv otelcol /usr/local/bin/
    
    - name: Run E2E tests
      run: go test -v -timeout 20m ./tests/e2e/
```