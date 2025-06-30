# End-to-End Tests

This directory contains end-to-end tests that validate the complete data flow from source databases to New Relic Database (NRDB).

## Overview

The e2e tests focus on validating:
1. **Data Collection**: Metrics are collected from PostgreSQL and MySQL
2. **Processing**: All processors function correctly (sampling, circuit breaking, plan extraction, verification)
3. **Export**: Data successfully arrives in NRDB
4. **Data Quality**: Metrics have correct attributes and values
5. **Completeness**: All expected metrics are present

## Prerequisites

1. **New Relic Account**
   ```bash
   export NEW_RELIC_LICENSE_KEY=your_license_key
   export NEW_RELIC_ACCOUNT_ID=your_account_id
   ```

2. **Databases** (optional - will be started automatically if not available)
   ```bash
   export POSTGRES_HOST=localhost
   export POSTGRES_PORT=5432
   export MYSQL_HOST=localhost
   export MYSQL_PORT=3306
   ```

## Running Tests

### Quick Run
```bash
# Run all e2e tests
./tests/e2e/run-e2e-tests.sh
```

### Manual Run
```bash
# Set environment
export E2E_TESTS=true
export TEST_RUN_ID=$(date +%s)

# Start test databases (if needed)
docker-compose -f tests/e2e/docker-compose-test.yaml up -d

# Run tests
go test -v -timeout=30m ./tests/e2e/...
```

### CI/CD Run
```bash
# GitHub Actions example
- name: Run E2E Tests
  env:
    NEW_RELIC_LICENSE_KEY: ${{ secrets.NEW_RELIC_LICENSE_KEY }}
    NEW_RELIC_ACCOUNT_ID: ${{ secrets.NEW_RELIC_ACCOUNT_ID }}
  run: make test-e2e
```

## Test Structure

```
e2e/
├── README.md                    # This file
├── run-e2e-tests.sh            # Main test runner
├── nrdb_validation_test.go     # Core NRDB validation tests
├── e2e_main_test.go           # Test suite setup
├── e2e_metrics_flow_test.go   # Detailed metrics flow tests
├── docker-compose-test.yaml    # Test database setup
├── config/
│   └── e2e-test-collector.yaml # Collector configuration
├── sql/
│   ├── postgres-init.sql      # PostgreSQL test data
│   └── mysql-init.sql         # MySQL test data
├── validators/
│   └── nrdb_validator.go      # NRDB query helpers
└── reports/                   # Test results and logs
```

## Test Scenarios

### 1. PostgreSQL Metrics Flow
- Validates PostgreSQL receiver collects metrics
- Generates test workload with various query patterns
- Verifies metrics appear in NRDB with correct values

### 2. MySQL Metrics Flow
- Validates MySQL receiver collects metrics
- Generates test workload
- Verifies metrics in NRDB

### 3. Custom Query Metrics
- Tests SQL query receiver
- Validates custom metrics collection
- Verifies attribute enrichment

### 4. Processor Validation
- **Adaptive Sampling**: Verifies slow queries are sampled at 100%
- **Circuit Breaker**: Tests protection mechanisms
- **Plan Extraction**: Validates query plans are extracted
- **PII Protection**: Ensures no sensitive data in metrics

### 5. Data Completeness
- Verifies all expected metric types are present
- Validates metric attributes
- Checks data freshness

## NRQL Queries Used

### Metric Count
```sql
SELECT count(*) FROM Metric 
WHERE metricName LIKE 'postgresql.%' 
SINCE 10 minutes ago
```

### Sampling Validation
```sql
SELECT count(*) FROM Metric 
WHERE sampled = true AND duration_ms > 1000 
SINCE 10 minutes ago
```

### PII Check
```sql
SELECT count(*) FROM Metric 
WHERE query_text LIKE '%@%' OR query_text LIKE '%SSN%' 
SINCE 10 minutes ago
```

### Data Freshness
```sql
SELECT latest(timestamp) FROM Metric 
WHERE test.run_id = 'YOUR_RUN_ID' 
SINCE 10 minutes ago
```

## Debugging Failed Tests

### 1. Check Collector Logs
```bash
cat tests/e2e/reports/collector-${TEST_RUN_ID}.log
```

### 2. Check Exported Metrics
```bash
jq . tests/e2e/reports/metrics-${TEST_RUN_ID}.json | less
```

### 3. Query NRDB Directly
Use New Relic Query Builder to run:
```sql
SELECT * FROM Metric 
WHERE test.run_id = 'YOUR_RUN_ID' 
SINCE 1 hour ago
LIMIT 100
```

### 4. Verify Connectivity
```bash
# Test database connections
psql -h localhost -U postgres -d testdb -c "SELECT 1"
mysql -h localhost -u root -pmysql -e "SELECT 1"

# Test New Relic API
curl -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  https://api.newrelic.com/v2/applications.json
```

## Adding New Tests

To add new e2e tests:

1. Add test function to `nrdb_validation_test.go`
2. Generate appropriate test data
3. Wait for data to flow (typically 2 minutes)
4. Query NRDB to validate results
5. Assert on expected values

Example:
```go
func (t *NRDBValidationTest) testNewFeature(tt *testing.T) {
    // Generate test data
    t.generateTestData(tt)
    
    // Wait for processing
    time.Sleep(2 * time.Minute)
    
    // Query NRDB
    results := t.queryNRDB(tt, "SELECT count(*) FROM Metric WHERE ...")
    
    // Validate
    assert.Greater(tt, results[0]["count"], 0)
}
```

## Performance Considerations

- Tests typically take 5-10 minutes to complete
- Each test generates ~1000 metrics
- Data appears in NRDB within 1-2 minutes
- Use TEST_RUN_ID to isolate test data

## Cleanup

Test data is automatically tagged with `test.run_id` for easy identification and cleanup. To remove test data from NRDB:

```sql
-- View test data
SELECT count(*), uniques(test.run_id) FROM Metric 
WHERE test.environment = 'e2e' 
SINCE 1 day ago

-- Data will age out according to your retention policy
```