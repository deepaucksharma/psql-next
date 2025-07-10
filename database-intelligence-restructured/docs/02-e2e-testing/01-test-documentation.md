## End-to-End Test Documentation

# End-to-End Test Documentation

## Overview
This document describes all E2E test scenarios, their purpose, and expected outcomes for the Database Intelligence OpenTelemetry Collector.

## Test Execution
All tests require the following environment variables:
- `NEW_RELIC_LICENSE_KEY`: New Relic license key
- `NEW_RELIC_API_KEY` or `NEW_RELIC_USER_KEY`: For NRDB queries
- `NEW_RELIC_ACCOUNT_ID`: New Relic account ID
- `NEW_RELIC_OTLP_ENDPOINT`: (Optional) Defaults to https://otlp.nr-data.net:4317

## Test Scenarios

### 1. PostgreSQL Metrics Collection (`docker_postgres_test.go`)
**Purpose**: Verify PostgreSQL metrics are collected and sent to New Relic

**Test Steps**:
1. Start PostgreSQL container (postgres:15-alpine)
2. Create test schema with tables and indexes
3. Start OTEL collector with PostgreSQL receiver
4. Generate database activity
5. Wait for metrics transmission

**Expected Outcomes**:
- ✅ 19 PostgreSQL metric types collected
- ✅ Metrics include: connection.max, database.count, table.size, index.size, bgwriter.*, etc.
- ✅ Custom attributes attached (test.run.id, environment, test.type)
- ✅ Metrics visible in NRDB within 1-2 minutes

**Verification**: Run `TestVerifyPostgreSQLMetricsInNRDB`

### 2. MySQL Metrics Collection (`docker_mysql_test.go`)
**Purpose**: Verify MySQL metrics are collected and sent to New Relic

**Test Steps**:
1. Start MySQL container (mysql:8.0)
2. Create test database with tables and indexes
3. Start OTEL collector with MySQL receiver
4. Generate database activity
5. Wait for metrics transmission

**Expected Outcomes**:
- ✅ 25 MySQL metric types collected
- ✅ Metrics include: buffer_pool.*, table.io.wait.*, threads, operations, etc.
- ✅ Custom attributes attached
- ✅ Entity mapping (MYSQLNODE entity type)

**Verification**: Run `TestVerifyMySQLMetricsInNRDB`

### 3. Processor Behavior Tests (`processor_behavior_test.go`)
**Purpose**: Test standard OTEL processors work correctly

#### 3.1 Batch Processor
**Configuration**:
```yaml
batch:
  timeout: 10s
  send_batch_size: 100
  send_batch_max_size: 200
```
**Expected**: Metrics batched and sent in groups

#### 3.2 Filter Processor
**Configuration**:
```yaml
filter:
  metrics:
    include:
      match_type: regexp
      metric_names:
        - "postgresql\\.table\\..*"
        - "postgresql\\.index\\..*"
```
**Expected**: Only table and index metrics pass through (verified: 12 metrics vs 24)

#### 3.3 Resource Processor
**Configuration**:
```yaml
resource:
  attributes:
    - key: service.name
      value: database-intelligence-e2e
    - key: cloud.provider
      value: test-cloud
```
**Expected**: Resource attributes added to all metrics

### 4. Metric Accuracy Test (`metric_accuracy_test.go`)
**Purpose**: Verify collector reports accurate metric values

**Test Steps**:
1. Create tables with known row counts:
   - small_table: 10 rows
   - medium_table: 100 rows
   - large_table: 1000 rows
2. Collect metrics
3. Compare with database statistics

**Expected Outcomes**:
- ✅ Table count matches (3 tables)
- ✅ Table sizes reported correctly
- ✅ Connection count accurate
- ⚠️ Row counts may show 0 due to pg_stat timing

### 5. NRDB Query Verification Tests
**Purpose**: Automated verification of metrics in New Relic

#### Queries Used:
1. Find test runs by ID
2. List unique metric names
3. Verify custom attributes
4. Check metric values
5. Validate entity mapping

### 6. Custom Processor Tests (Planned)
**Note**: Requires building custom collector with all 7 processors

#### Processors to Test:
1. **adaptivesampler**: Dynamic sampling based on load
2. **circuitbreaker**: Protection under high load
3. **planattributeextractor**: Extract query execution plans
4. **querycorrelator**: Correlate related queries
5. **verification**: Validate metric accuracy
6. **costcontrol**: Limit metric volume
7. **nrerrormonitor**: Track processing errors

## Test Utilities

### Framework Components
- `TestEnvironment`: Manages test infrastructure
- `TestCollector`: Wraps collector lifecycle
- `queryNRDB()`: GraphQL queries to NRDB

### Helper Functions
- Database connection management
- Container lifecycle management
- Metric verification queries

## Running Tests

### Individual Tests
```bash
# PostgreSQL test
go test -v -run TestDockerPostgreSQLCollection

# MySQL test  
go test -v -run TestDockerMySQLCollection

# Processor tests
go test -v -run TestProcessorBehaviors

# Metric accuracy
go test -v -run TestMetricAccuracy
```

### Verification Tests
```bash
# Verify PostgreSQL metrics
export TEST_RUN_ID="docker_postgres_XXXXXX"
go test -v -run TestVerifyPostgreSQLMetricsInNRDB

# Verify MySQL metrics
export TEST_RUN_ID="docker_mysql_XXXXXX" 
go test -v -run TestVerifyMySQLMetricsInNRDB
```

### All E2E Tests
```bash
go test -v ./tests/e2e/... -timeout 30m
```

## Troubleshooting

### Common Issues
1. **Docker not running**: Ensure Docker daemon is started
2. **Port conflicts**: Tests use ports 5432, 15432, 25432 for PostgreSQL; 3306 for MySQL
3. **Metrics not found**: Wait 2-3 minutes for processing, check test.run.id
4. **SSL errors**: PostgreSQL receiver may log SSL warnings (not critical)

### Debug Tools
- `TestDebugMetrics`: Explore what's in NRDB
- Collector logs: Check Docker logs for the collector container
- NRDB Explorer: Use New Relic UI to query metrics

## Future Enhancements
1. Test collector with multiple database instances
2. Connection failure recovery tests
3. High load performance tests
4. SSL/TLS connection tests
5. Memory usage monitoring
6. Long-running stability tests (1+ hours)
7. Schema change handling
8. Configuration hot-reload

## Success Metrics
- All tests pass consistently
- Metrics appear in NRDB within 2 minutes
- No data loss during normal operation
- Processors behave as configured
- Resource usage stays within limits
