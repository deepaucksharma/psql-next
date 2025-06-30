# Testing Guide

This guide provides a quick reference for testing the Database Intelligence Collector. For comprehensive E2E testing documentation, see [E2E Testing Complete Guide](../E2E_TESTING_COMPLETE.md).

## Testing Philosophy

The Database Intelligence Collector uses **end-to-end testing exclusively**. We validate the complete data pipeline from database metrics collection through processing to final storage in NRDB. This approach ensures:

1. **Real-world validation**: Tests verify actual data flow in production-like conditions
2. **Integration confidence**: All components are tested together as they work in production
3. **Business value focus**: Tests validate that metrics provide actionable insights in New Relic

## Test Structure

```
tests/
└── e2e/                         # End-to-end tests only
    ├── nrdb_validation_test.go  # Core NRDB validation
    ├── e2e_main_test.go        # Test suite setup
    ├── e2e_metrics_flow_test.go # Metrics flow validation
    ├── run-e2e-tests.sh        # Test runner script
    ├── docker-compose-test.yaml # Test environment
    ├── config/
    │   └── e2e-test-collector.yaml
    ├── sql/
    │   ├── postgres-init.sql
    │   └── mysql-init.sql
    ├── validators/
    │   └── nrdb_validator.go
    └── reports/                # Test results
```

## Prerequisites

### 1. New Relic Account
```bash
export NEW_RELIC_LICENSE_KEY=your_license_key
export NEW_RELIC_ACCOUNT_ID=your_account_id
```

### 2. Test Databases
Either have PostgreSQL and MySQL running locally, or let the test framework start them:
```bash
# Optional: specify custom database hosts
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
```

### 3. Build the Collector
```bash
make build
# or
task build
```

## Running Tests

### Quick Start
```bash
# Run all e2e tests
make test

# Or using Task
task test
```

### Detailed Test Execution
```bash
# Run with test script (recommended)
./tests/e2e/run-e2e-tests.sh

# Run directly with Go
E2E_TESTS=true go test -v -timeout=30m ./tests/e2e/...

# Run specific test
E2E_TESTS=true go test -v ./tests/e2e/... -run TestPostgreSQLMetricsFlow
```

### CI/CD Integration
```yaml
# GitHub Actions example
- name: Run E2E Tests
  env:
    NEW_RELIC_LICENSE_KEY: ${{ secrets.NEW_RELIC_LICENSE_KEY }}
    NEW_RELIC_ACCOUNT_ID: ${{ secrets.NEW_RELIC_ACCOUNT_ID }}
  run: make test-e2e
```

## Test Scenarios

### 1. Database Metrics Flow Tests

#### PostgreSQL Metrics Flow
```go
func testPostgreSQLMetricsFlow(t *testing.T) {
    // Generate test workload
    generatePostgreSQLLoad()
    
    // Wait for collection and export
    time.Sleep(2 * time.Minute)
    
    // Query NRDB for metrics
    results := queryNRDB("SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql.%'")
    
    // Validate metrics exist
    assert.Greater(t, results[0]["count"], 0)
}
```

#### MySQL Metrics Flow
Similar validation for MySQL metrics ensuring data flows correctly.

### 2. Processor Validation Tests

#### Adaptive Sampling
```sql
-- Verify slow queries are sampled at 100%
SELECT count(*) FROM Metric 
WHERE sampled = true AND duration_ms > 1000 
SINCE 10 minutes ago
```

#### Circuit Breaker
Tests protection mechanisms by monitoring circuit state changes.

#### Plan Extraction
```sql
-- Verify query plans are extracted
SELECT count(*) FROM Metric 
WHERE plan.hash IS NOT NULL 
SINCE 10 minutes ago
```

#### PII Protection
```sql
-- Ensure no PII in metrics
SELECT count(*) FROM Metric 
WHERE query_text LIKE '%@%' OR query_text LIKE '%SSN%' 
SINCE 10 minutes ago
```

### 3. Data Completeness Tests

Validates all expected metric types are present:
- `postgresql.database.size`
- `postgresql.backends`
- `mysql.threads`
- `mysql.slow_queries`
- And more...

## Writing New E2E Tests

### Test Template
```go
func (t *NRDBValidationTest) testNewFeature(tt *testing.T) {
    // 1. Generate test data
    db := connectToDatabase(tt)
    generateSpecificWorkload(db)
    
    // 2. Wait for data pipeline
    time.Sleep(2 * time.Minute)
    
    // 3. Query NRDB
    query := fmt.Sprintf(`{
        actor {
            account(id: %s) {
                nrql(query: "SELECT ... FROM Metric WHERE ... SINCE 10 minutes ago") {
                    results
                }
            }
        }
    }`, t.nrAccountID)
    
    results := t.queryNRDB(tt, query)
    
    // 4. Validate results
    assert.NotEmpty(tt, results)
    assert.Greater(tt, results[0]["count"], 0)
}
```

### Best Practices

1. **Isolate Test Data**: Use `test.run_id` to tag test metrics
2. **Wait Appropriately**: Allow 1-2 minutes for data to appear in NRDB
3. **Clean Assertions**: Test one thing per test function
4. **Meaningful Names**: Use descriptive test and metric names

## Debugging Failed Tests

### 1. Check Collector Logs
```bash
# View collector logs from test run
cat tests/e2e/reports/collector-${TEST_RUN_ID}.log

# Check for errors
grep ERROR tests/e2e/reports/collector-*.log
```

### 2. Verify Exported Metrics
```bash
# View metrics that would be exported
jq . tests/e2e/reports/metrics-${TEST_RUN_ID}.json | less
```

### 3. Query NRDB Directly
Use New Relic Query Builder:
```sql
SELECT * FROM Metric 
WHERE test.run_id = 'YOUR_TEST_RUN_ID' 
SINCE 1 hour ago
LIMIT 100
```

### 4. Common Issues

#### No Metrics in NRDB
- Check NEW_RELIC_LICENSE_KEY is valid
- Verify network connectivity to New Relic
- Look for NrIntegrationError events

#### Database Connection Failures
```bash
# Test connectivity
psql -h localhost -U postgres -d testdb -c "SELECT 1"
mysql -h localhost -u root -pmysql -e "SELECT 1"
```

#### Timing Issues
- Increase wait times if metrics don't appear
- Check collector startup time in logs
- Verify collection intervals in config

## Test Environment

### Local Test Databases
The test framework can start databases automatically:
```yaml
# tests/e2e/docker-compose-test.yaml
services:
  postgres-test:
    image: postgres:15
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: testdb
    
  mysql-test:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: mysql
      MYSQL_DATABASE: testdb
```

### Test Configuration
```yaml
# tests/e2e/config/e2e-test-collector.yaml
receivers:
  postgresql:
    collection_interval: 10s  # Faster for testing
    
processors:
  adaptive_sampler:
    rules:
      - name: test_queries
        conditions:
          - attribute: query_text
            operator: contains
            value: e2e_test
        sample_rate: 1.0  # Sample all test queries
        
exporters:
  otlp/newrelic:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
```

## Performance Considerations

- Each test run generates ~1000-2000 metrics
- Tests typically complete in 5-10 minutes
- Data appears in NRDB within 1-2 minutes
- Use TEST_RUN_ID to isolate parallel test runs

## Continuous Integration

### GitHub Actions
```yaml
name: E2E Tests
on: [push, pull_request]

jobs:
  e2e-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build Collector
        run: make build
      
      - name: Run E2E Tests
        env:
          NEW_RELIC_LICENSE_KEY: ${{ secrets.NEW_RELIC_LICENSE_KEY }}
          NEW_RELIC_ACCOUNT_ID: ${{ secrets.NEW_RELIC_ACCOUNT_ID }}
        run: make test-e2e
```

### Test Reports
Test results are saved in `tests/e2e/reports/`:
- `summary-${TEST_RUN_ID}.txt` - Test summary
- `collector-${TEST_RUN_ID}.log` - Collector logs
- `metrics-${TEST_RUN_ID}.json` - Exported metrics

## NRQL Queries for Validation

### Basic Metric Count
```sql
SELECT count(*) FROM Metric 
WHERE test.environment = 'e2e' 
SINCE 30 minutes ago
```

### Validate Processing
```sql
SELECT count(*), average(value) FROM Metric 
WHERE metricName = 'postgresql.database.size' 
FACET db.name 
SINCE 10 minutes ago
```

### Check Data Freshness
```sql
SELECT latest(timestamp) FROM Metric 
WHERE test.run_id = 'YOUR_RUN_ID' 
SINCE 10 minutes ago
```

### Processor Effectiveness
```sql
-- Sampling rate
SELECT percentage(count(*), WHERE sampled = true) FROM Metric 
WHERE duration_ms > 0 
SINCE 1 hour ago

-- Circuit breaker trips
SELECT count(*) FROM Metric 
WHERE circuit_breaker.state = 'open' 
SINCE 1 hour ago
```

## Summary

The Database Intelligence Collector's testing strategy focuses on **end-to-end validation** of the complete data pipeline. By testing against real databases and validating data in NRDB, we ensure the collector works correctly in production environments. This approach provides confidence that:

1. Metrics are collected accurately from databases
2. Processors function correctly in the pipeline
3. Data arrives in New Relic with proper attributes
4. The system provides business value through actionable insights

---

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025