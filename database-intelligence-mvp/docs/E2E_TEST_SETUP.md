# End-to-End Test Setup Guide

This guide documents the complete setup for running E2E tests with the Database Intelligence Collector.

## Overview

The E2E test suite validates:
1. Database metrics collection from PostgreSQL and MySQL
2. Proper attribute enrichment
3. Data export to New Relic (or local file for testing)
4. Dashboard creation and metric validation

## Prerequisites

- Go 1.21+ installed
- Docker (for test databases)
- New Relic account (optional, for full E2E)
- Built collector binary in `dist/` directory

## Configuration Fixes Applied

### 1. Environment Variable Syntax
All environment variables now use the proper `${env:VAR_NAME:-default}` syntax:
```yaml
# Old (incorrect)
endpoint: ${POSTGRES_HOST:localhost}

# New (correct)
endpoint: ${env:POSTGRES_HOST:-localhost}
```

### 2. Memory Limiter Configuration
Fixed from percentage-based to MiB-based configuration:
```yaml
# Old (incorrect)
memory_limiter:
  limit_percentage: 75
  spike_limit_percentage: 20

# New (correct)
memory_limiter:
  limit_mib: 512
  spike_limit_mib: 128
```

### 3. SQL Query Receiver Format
Fixed to include proper logs configuration:
```yaml
sqlquery/postgresql:
  queries:
    - sql: "SELECT ..."
      logs:  # Required for query log collection
        - body_column: query_text
          attributes:
            query_id: query_id
            avg_duration_ms: avg_duration_ms
```

### 4. Extension Availability
The pre-built collector only includes `zpages` extension, not `health_check` or `memory_ballast`.

## Test Configurations

### 1. Simple Test Config (`config/simple-test.yaml`)
Basic configuration for testing database metrics collection:
```yaml
receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST:-localhost}:5432
  mysql:
    endpoint: ${env:MYSQL_HOST:-localhost}:3306

exporters:
  debug:
    verbosity: detailed
  file:
    path: /tmp/e2e-metrics.json
```

### 2. Full E2E Test Config (`tests/e2e/config/working-test-config.yaml`)
Complete configuration with all processors and exporters.

## Running Tests

### Quick Test with Existing Databases

1. **Validate configuration:**
   ```bash
   ./dist/database-intelligence-collector validate --config=config/simple-test.yaml
   ```

2. **Run collector:**
   ```bash
   ./dist/database-intelligence-collector --config=config/simple-test.yaml
   ```

3. **Check metrics:**
   ```bash
   # View metrics in debug output
   # Or check file export
   cat /tmp/e2e-metrics.json | jq .
   ```

### Full E2E Test Suite

1. **Set environment variables:**
   ```bash
   export NEW_RELIC_LICENSE_KEY=your-key
   export NEW_RELIC_ACCOUNT_ID=your-account
   export NEW_RELIC_USER_KEY=your-user-key
   ```

2. **Run E2E tests:**
   ```bash
   make test-e2e
   ```

   This will:
   - Start test databases (if needed)
   - Run the collector
   - Execute validation tests
   - Generate reports in `tests/e2e/reports/`

### Dashboard Creation Test

```bash
# Create dashboard
node scripts/create-database-dashboard.js

# Validate metrics
node tests/e2e/dashboard-metrics-validation.js
```

## Verified Metrics

### PostgreSQL Metrics (12 required)
- ✅ `postgresql.backends` - Active connections
- ✅ `postgresql.commits` - Transaction commits
- ✅ `postgresql.rollbacks` - Transaction rollbacks
- ✅ `postgresql.db_size` - Database size
- ✅ `postgresql.table.count` - Number of tables
- ✅ `postgresql.bgwriter.buffers.allocated` - Buffer allocations
- ✅ `postgresql.bgwriter.buffers.writes` - Buffer writes
- ✅ `postgresql.bgwriter.checkpoint.count` - Checkpoints
- ✅ `postgresql.bgwriter.duration` - Checkpoint duration
- ✅ `postgresql.bgwriter.maxwritten` - Max written stops
- ✅ `postgresql.connection.max` - Max connections
- ✅ `postgresql.database.count` - Database count

### MySQL Metrics (11 required)
- ✅ `mysql.buffer_pool.data_pages` - Buffer pool pages
- ✅ `mysql.buffer_pool.limit` - Buffer pool size
- ✅ `mysql.buffer_pool.operations` - Buffer operations
- ✅ `mysql.buffer_pool.page_flushes` - Page flushes
- ✅ `mysql.buffer_pool.pages` - Page statistics
- ✅ `mysql.buffer_pool.usage` - Buffer usage
- ✅ `mysql.double_writes` - Double write operations
- ✅ `mysql.handlers` - Handler statistics
- ✅ `mysql.locks` - Lock statistics
- ✅ `mysql.log_operations` - Log operations
- ✅ `mysql.operations` - InnoDB operations

## Troubleshooting

### Common Issues

1. **"unknown type: health_check"**
   - Remove `health_check` from extensions
   - Use only `zpages` extension

2. **"invalid keys: attributes"**
   - SQL query receiver needs `logs` section with attributes
   - See configuration examples above

3. **Environment variable not expanding**
   - Use `${env:VAR:-default}` syntax
   - Don't use old `${VAR:default}` format

4. **Collector won't build**
   - Version compatibility issues exist
   - Use pre-built binary in `dist/` directory

### Debug Commands

```bash
# Check collector components
./dist/database-intelligence-collector components

# Validate any config
./dist/database-intelligence-collector validate --config=path/to/config.yaml

# Run with debug logging
./dist/database-intelligence-collector --config=config.yaml --set=service.telemetry.logs.level=debug
```

## Next Steps

1. **For production deployment:**
   - Fix module path inconsistencies in build configs
   - Build collector with proper versioning
   - Add health check extension support

2. **For testing:**
   - Use working configurations documented here
   - Monitor `/tmp/e2e-metrics.json` for metric validation
   - Check debug output for troubleshooting