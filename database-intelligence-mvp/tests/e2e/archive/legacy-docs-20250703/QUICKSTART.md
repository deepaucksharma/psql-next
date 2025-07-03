# E2E Tests Quick Start Guide

This guide helps you quickly set up and run the end-to-end tests for the PostgreSQL Database Intelligence collector.

## Prerequisites

### Required Software
- Docker Desktop (or Docker Engine + Docker Compose)
- Go 1.21 or later
- Make (optional but recommended)
- PostgreSQL client tools (psql)

### New Relic Account (for NRDB tests)
- New Relic account with API access
- Account ID, License Key, and API Key

## Quick Setup

### 1. Clone the Repository
```bash
git clone https://github.com/database-intelligence-mvp/database-intelligence-mvp.git
cd database-intelligence-mvp
```

### 2. Install Dependencies
```bash
# Install Go dependencies
go mod download

# Install test tools
go install gotest.tools/gotestsum@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### 3. Set Environment Variables

Create a `.env.test` file in the project root:
```bash
# PostgreSQL Configuration
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=test_user
export POSTGRES_PASSWORD=test_password
export POSTGRES_DB=test_db
export POSTGRES_LOG_PATH=/var/log/postgresql/postgresql.log

# New Relic Configuration (optional, for NRDB tests)
export NEW_RELIC_ACCOUNT_ID=your_account_id
export NEW_RELIC_LICENSE_KEY=your_license_key
export NEW_RELIC_API_KEY=your_api_key
export NEW_RELIC_OTLP_ENDPOINT=otlp.nr-data.net:4317

# Test Configuration
export TEST_TIMEOUT=30m
export LOG_LEVEL=info
```

### 4. Start Test Environment
```bash
# Source environment variables
source .env.test

# Start PostgreSQL container
cd tests/e2e
make docker-up

# Verify PostgreSQL is running
docker ps | grep postgres-test
```

## Running Tests

### Run All E2E Tests
```bash
make test
```

### Run Specific Test Categories

#### Plan Intelligence Tests Only
```bash
go test -v -run TestPlanIntelligenceE2E ./tests/e2e/
```

#### ASH Tests Only
```bash
go test -v -run TestASHE2E ./tests/e2e/
```

#### Integration Tests Only
```bash
go test -v -run TestFullIntegrationE2E ./tests/e2e/
```

#### NRDB Validation Tests (requires New Relic credentials)
```bash
go test -v -run TestNRQLDashboardQueries ./tests/e2e/
```

#### Performance Tests
```bash
go test -v -run TestPerformanceE2E -timeout 30m ./tests/e2e/
```

#### Monitoring Tests
```bash
go test -v -run TestMonitoringAndAlerts ./tests/e2e/
```

### Run with Coverage
```bash
make test-coverage
```

### Run Benchmarks
```bash
make test-benchmark
```

## Test Scenarios

### 1. Basic Functionality Test
Tests core collector functionality:
```bash
go test -v -run "TestPlanIntelligenceE2E/AutoExplainLogCollection" ./tests/e2e/
```

### 2. Load Testing
Tests performance under load:
```bash
go test -v -run "TestPerformanceE2E/SustainedHighLoad" ./tests/e2e/
```

### 3. NRQL Query Validation
Validates all dashboard queries:
```bash
go test -v -run "TestNRQLDashboardQueries/PostgreSQLOverviewDashboard" ./tests/e2e/
```

### 4. Circuit Breaker Testing
Tests resilience features:
```bash
go test -v -run "TestPlanIntelligenceE2E/CircuitBreakerProtection" ./tests/e2e/
```

## Debugging Failed Tests

### 1. Enable Debug Logging
```bash
export LOG_LEVEL=debug
go test -v -run TestName ./tests/e2e/
```

### 2. View Container Logs
```bash
# PostgreSQL logs
docker logs postgres-test

# View test PostgreSQL data
docker exec -it postgres-test psql -U test_user -d test_db
```

### 3. Check Test Artifacts
```bash
# Test results are saved in
ls -la test-results/current/

# View test summary
cat test-results/current/summary.txt

# View collector logs
cat test-results/current/collector.log
```

### 4. Run Single Test with Verbose Output
```bash
go test -v -run "TestName/SubTestName" ./tests/e2e/ -args -debug
```

## Common Issues and Solutions

### Issue: PostgreSQL container won't start
```bash
# Check if port 5432 is already in use
lsof -i :5432

# Stop conflicting service or use different port
export POSTGRES_PORT=5433
```

### Issue: Collector not found
```bash
# Install OpenTelemetry Collector
wget https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.88.0/otelcol_0.88.0_$(uname -s)_$(uname -m).tar.gz
tar -xvf otelcol_*.tar.gz
sudo mv otelcol /usr/local/bin/
```

### Issue: NRQL tests failing
```bash
# Verify New Relic credentials
curl -X POST https://api.newrelic.com/graphql \
  -H "Content-Type: application/json" \
  -H "API-Key: $NEW_RELIC_API_KEY" \
  -d '{"query":"{ actor { user { name email } } }"}'
```

### Issue: Out of memory during tests
```bash
# Increase Docker memory limit
# Docker Desktop: Preferences > Resources > Memory > 8GB

# Or run subset of tests
go test -v -short ./tests/e2e/
```

## Continuous Integration

### GitHub Actions
The repository includes GitHub Actions workflow for automated testing:
- `.github/workflows/e2e-tests.yml`

### Running Locally with Act
```bash
# Install act
brew install act

# Run E2E workflow locally
act -j e2e-tests
```

## Writing New Tests

### 1. Create Test File
```go
// tests/e2e/my_feature_test.go
package e2e

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestMyFeatureE2E(t *testing.T) {
    testEnv := setupTestEnvironment(t)
    defer testEnv.Cleanup()
    
    // Your test code here
}
```

### 2. Use Test Helpers
```go
// Generate test data
generateSlowQueries(t, testEnv.PostgresDB)

// Apply load pattern
applyLoadPattern(t, testEnv.PostgresDB, HeavyLoadPattern)

// Wait for metrics
require.Eventually(t, func() bool {
    metrics := testEnv.NRDBExporter.GetMetrics()
    return len(metrics) > 0
}, 30*time.Second, 1*time.Second)
```

### 3. Validate Results
```go
// Check metrics
metrics := testEnv.NRDBExporter.GetMetricsByName("postgresql.query.execution")
require.NotEmpty(t, metrics)

// Validate attributes
for _, m := range metrics {
    require.NotEmpty(t, m.Attributes["query.normalized"])
    require.NotEmpty(t, m.Attributes["db.name"])
}
```

## Next Steps

1. **Explore Test Coverage**: Review `tests/e2e/README.md` for detailed test descriptions
2. **Check NRQL Queries**: See `dashboards/nrql-queries.md` for all validated queries
3. **Review Data Model**: Understand metrics in `docs/NRDB_DATA_MODEL.md`
4. **Contribute**: Add new test scenarios for missing coverage

## Support

- **Issues**: Create an issue in the GitHub repository
- **Documentation**: Check `/docs` directory for detailed documentation
- **CI Failures**: Review GitHub Actions logs for detailed error messages