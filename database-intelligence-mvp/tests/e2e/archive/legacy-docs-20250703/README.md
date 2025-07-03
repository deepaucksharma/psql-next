# End-to-End Tests for PostgreSQL Database Intelligence

This directory contains comprehensive E2E tests that validate the entire data flow from PostgreSQL through the OpenTelemetry collector to New Relic Database (NRDB).

## Test Coverage

### 1. Plan Intelligence Tests (`plan_intelligence_test.go`)
- Auto-explain log collection
- Plan anonymization and PII protection
- Plan regression detection
- NRDB metric export validation
- Circuit breaker functionality

### 2. Active Session History Tests (`ash_test.go`)
- 1-second session sampling
- Wait event analysis
- Blocking session detection
- Adaptive sampling under load
- Query activity tracking
- Time window aggregation

### 3. Integration Tests (`integration_test.go`)
- Combined Plan Intelligence + ASH correlation
- Regression detection with wait analysis
- Memory pressure handling
- End-to-end data flow validation

### 4. NRDB Validation Tests (`nrdb_validation_test.go`)
- NRQL query validation
- Dashboard query testing
- Metric attribute verification
- Data integrity checks
- Alert query validation

### 5. Performance Tests (`performance_test.go`)
- Baseline performance metrics
- High cardinality query handling
- Sustained load testing
- Burst load scenarios
- Memory usage under stress

### 6. Monitoring Tests (`monitoring_test.go`)
- Collector health metrics
- Pipeline metrics validation
- Prometheus endpoint testing
- Circuit breaker metrics
- Alert rule validation

## Running the Tests

### Prerequisites

1. Docker and Docker Compose installed
2. Go 1.21+ installed
3. New Relic account (for NRDB validation tests)

### Environment Variables

For basic tests:
```bash
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=test_user
export POSTGRES_PASSWORD=test_password
export POSTGRES_DB=test_db
export POSTGRES_LOG_PATH=/var/log/postgresql/postgresql.log
```

For New Relic integration tests:
```bash
export NEW_RELIC_ACCOUNT_ID=your_account_id
export NEW_RELIC_LICENSE_KEY=your_license_key
export NEW_RELIC_API_KEY=your_api_key
export NEW_RELIC_OTLP_ENDPOINT=otlp.nr-data.net:4317
```

### Running Tests

#### All E2E Tests
```bash
make test
```

#### Specific Test Categories
```bash
# Unit tests only (fast)
make test-unit

# Integration tests
make test-integration

# Performance tests
make test-performance

# Benchmarks
make test-benchmark

# Specific test
make test-specific TEST=TestPlanIntelligenceE2E
```

#### With Coverage
```bash
make test-coverage
```

#### In CI Mode
```bash
make ci
```

### Test Environment

#### Starting Test Environment
```bash
make docker-up
```

#### Stopping Test Environment
```bash
make docker-down
```

### Test Output

Test results are stored in the `test-results/` directory:
- `test-results/current/` - Current test run results
- `test-results/e2e-results-*.tar.gz` - Archived test results

Each test run includes:
- Test logs for each category
- Collector logs
- PostgreSQL logs
- Prometheus metrics snapshot
- OTLP request logs
- Summary report

## NRQL Queries Validated

The tests validate all NRQL queries used in New Relic dashboards:

### PostgreSQL Overview Dashboard
- Active connections by database
- Transaction rate (commits/rollbacks per minute)
- Cache hit ratio
- Database size
- Top queries by execution count

### Plan Intelligence Dashboard
- Plan changes over time
- Plan regression detection
- Query performance trends
- Top regressions by cost increase
- Plan node analysis
- Query plan distribution

### Active Session History Dashboard
- Active sessions over time by state
- Wait event distribution
- Top wait events
- Blocking analysis
- Session activity by query
- Resource utilization

### Integrated Intelligence Dashboard
- Query performance with wait analysis
- Plan regression impact
- Query health scores
- Adaptive sampling effectiveness

### Alert Queries
- High plan regression rate
- Excessive lock waits
- Query performance degradation
- Database connection saturation
- Circuit breaker activation

## Troubleshooting

### Common Issues

1. **PostgreSQL not starting**: Check Docker logs
   ```bash
   docker-compose -f testdata/docker-compose.test.yml logs postgres-test
   ```

2. **Collector not healthy**: Check collector logs
   ```bash
   docker-compose -f testdata/docker-compose.test.yml logs otel-collector
   ```

3. **NRQL tests failing**: Verify New Relic credentials
   ```bash
   curl -X POST https://api.newrelic.com/graphql \
     -H "Content-Type: application/json" \
     -H "API-Key: $NEW_RELIC_API_KEY" \
     -d '{"query":"{ actor { user { name email } } }"}'
   ```

### Debug Mode

Run tests with debug output:
```bash
go test -v -run TestName -debug
```

View collector metrics:
```bash
curl http://localhost:8888/metrics
```

View zpages:
```bash
open http://localhost:55679/debug/tracez
```

## Contributing

When adding new tests:

1. Follow the existing test structure
2. Add appropriate test data generation
3. Include cleanup in defer statements
4. Document any new environment variables
5. Update this README with new test coverage

## Performance Baselines

Expected performance metrics (on standard hardware):
- Plan collection: < 5ms overhead per query
- ASH sampling: < 1ms per sample
- Memory usage: < 500MB under normal load
- Metric export latency: < 100ms p95

## License

See LICENSE file in the repository root.