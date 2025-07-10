# Test Configuration Guide

## Overview

All test configurations are now centralized and managed through environment variables to improve security and flexibility.

## Setup

1. Copy the example configuration:
   ```bash
   cp test.env.example test.env
   ```

2. Edit `test.env` with your actual values:
   ```bash
   # Database credentials
   TEST_MYSQL_PASSWORD=your-mysql-password
   TEST_POSTGRES_PASSWORD=your-postgres-password
   
   # New Relic credentials
   TEST_NEWRELIC_API_KEY=your-api-key
   ```

3. Run tests:
   ```bash
   go test ./...
   ```

## Configuration Options

### Database Settings
- `TEST_MYSQL_HOST` - MySQL host (default: localhost)
- `TEST_MYSQL_PORT` - MySQL port (default: 3306)
- `TEST_MYSQL_USER` - MySQL username (default: root)
- `TEST_MYSQL_PASSWORD` - MySQL password (default: mysql)
- `TEST_MYSQL_DATABASE` - MySQL database name (default: test)

- `TEST_POSTGRES_HOST` - PostgreSQL host (default: localhost)
- `TEST_POSTGRES_PORT` - PostgreSQL port (default: 5432)
- `TEST_POSTGRES_USER` - PostgreSQL username (default: postgres)
- `TEST_POSTGRES_PASSWORD` - PostgreSQL password (default: postgres)
- `TEST_POSTGRES_DATABASE` - PostgreSQL database name (default: test)

### New Relic Settings
- `TEST_NEWRELIC_API_KEY` - New Relic API key (required for E2E tests)
- `TEST_NEWRELIC_ENDPOINT` - New Relic OTLP endpoint (default: https://otlp.nr-data.net)

### Test Control
- `TEST_SKIP_INTEGRATION` - Skip integration tests (default: false)
- `TEST_SKIP_E2E` - Skip end-to-end tests (default: false)
- `TEST_SKIP_PERFORMANCE` - Skip performance tests (default: false)
- `TEST_VERBOSE` - Enable verbose test output (default: false)

### Docker Settings
- `TEST_DOCKER_NETWORK` - Docker network name (default: test-network)
- `TEST_DOCKER_MYSQL_IMAGE` - MySQL Docker image (default: mysql:8.0)
- `TEST_DOCKER_POSTGRES_IMAGE` - PostgreSQL Docker image (default: postgres:14)

## Usage in Tests

```go
import "github.com/database-intelligence/tests/testconfig"

func TestMyFeature(t *testing.T) {
    cfg := testconfig.Get()
    
    // Skip if E2E tests are disabled
    if cfg.SkipE2E {
        t.Skip("E2E tests are disabled")
    }
    
    // Use configuration
    db, err := sql.Open("mysql", cfg.MySQLDSN())
    // ...
}
```

## Security Notes

- Never commit `test.env` to version control
- Use CI/CD secret management for production environments
- Rotate test credentials regularly
- Use separate credentials for testing vs production

## Migration from Hardcoded Values

Tests are being migrated from hardcoded credentials to use the centralized configuration. During the transition:
1. New tests should use `testconfig`
2. Existing tests will be updated incrementally
3. Both approaches will work temporarily

## Troubleshooting

1. **Tests skip with "credentials not set"**
   - Ensure `test.env` exists and contains required values
   - Check environment variable names match exactly

2. **Connection failures**
   - Verify database services are running
   - Check network connectivity
   - Ensure credentials are correct

3. **Docker tests fail**
   - Verify Docker is running
   - Check Docker network exists
   - Ensure no port conflicts