# Feature Detection Troubleshooting Guide

This guide helps diagnose and resolve common issues with the feature detection and graceful fallback system.

## Table of Contents

1. [Quick Diagnostics](#quick-diagnostics)
2. [Common Issues](#common-issues)
3. [PostgreSQL Troubleshooting](#postgresql-troubleshooting)
4. [MySQL Troubleshooting](#mysql-troubleshooting)
5. [Circuit Breaker Issues](#circuit-breaker-issues)
6. [Performance Problems](#performance-problems)
7. [Debugging Tools](#debugging-tools)
8. [Resolution Steps](#resolution-steps)

## Quick Diagnostics

### Check Feature Detection Status

```bash
# View feature detection metrics
curl -s http://localhost:8888/metrics | grep db_feature

# Check for extension availability
curl -s http://localhost:8888/metrics | grep 'db_feature_extension_available'

# Check for capability status
curl -s http://localhost:8888/metrics | grep 'db_feature_capability_available'

# View disabled queries
curl -s http://localhost:8888/metrics | grep 'circuitbreaker_disabled_queries'

# Check fallback usage
curl -s http://localhost:8888/metrics | grep 'enhancedsql_fallback_count'
```

### View Collector Logs

```bash
# Check for feature detection logs
docker logs collector 2>&1 | grep -i "feature"

# View circuit breaker actions
docker logs collector 2>&1 | grep -i "circuit"

# Check for permission errors
docker logs collector 2>&1 | grep -i "permission denied"

# View query fallback events
docker logs collector 2>&1 | grep -i "fallback"
```

## Common Issues

### 1. No Features Detected

**Symptoms:**
- All `db_feature_extension_available` metrics show 0
- No query performance metrics collected
- Logs show "No features detected"

**Diagnosis:**
```bash
# Check database connection
psql -h localhost -U postgres -c "SELECT version()"

# Verify collector can connect
docker exec collector /collector test --config=/etc/otelcol/config.yaml
```

**Solutions:**
- [Check Database Permissions](#database-permissions)
- [Verify Network Connectivity](#network-connectivity)
- [Review Configuration](#configuration-issues)

### 2. Extensions Not Found

**Symptoms:**
- Logs show "relation pg_stat_statements does not exist"
- Feature metrics show extension as unavailable
- Queries are being disabled

**Diagnosis:**
```sql
-- Check installed extensions
SELECT * FROM pg_extension;

-- Check available extensions
SELECT * FROM pg_available_extensions 
WHERE name IN ('pg_stat_statements', 'pg_stat_monitor', 'pg_wait_sampling');

-- Check shared_preload_libraries
SHOW shared_preload_libraries;
```

**Solutions:**
- [Install Missing Extensions](#installing-extensions)
- [Configure shared_preload_libraries](#postgresql-configuration)

### 3. Queries Timing Out

**Symptoms:**
- Feature detection takes too long
- Timeout errors in logs
- Incomplete feature detection

**Diagnosis:**
```yaml
# Check timeout settings
feature_detection:
  timeout_per_check: 3s  # Increase if needed
```

**Solutions:**
- Increase timeout values
- Check database load
- Optimize feature detection queries

## PostgreSQL Troubleshooting

### Database Permissions

The monitoring user needs specific permissions:

```sql
-- Create monitoring role with necessary permissions
CREATE ROLE monitoring_role;

-- Grant system monitoring privileges
GRANT pg_monitor TO monitoring_role;

-- Grant access to statistics views
GRANT SELECT ON pg_stat_statements TO monitoring_role;
GRANT SELECT ON pg_stat_statements_info TO monitoring_role;

-- Create user and assign role
CREATE USER otel_monitor WITH PASSWORD 'secure_password';
GRANT monitoring_role TO otel_monitor;

-- For pg_stat_monitor
GRANT SELECT ON pg_stat_monitor TO monitoring_role;

-- For pg_wait_sampling
GRANT EXECUTE ON FUNCTION pg_wait_sampling_get_current() TO monitoring_role;
```

### Installing Extensions

```sql
-- Install pg_stat_statements (requires superuser)
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Install pg_stat_monitor (alternative to pg_stat_statements)
CREATE EXTENSION IF NOT EXISTS pg_stat_monitor;

-- Install pg_wait_sampling
CREATE EXTENSION IF NOT EXISTS pg_wait_sampling;

-- Verify installation
SELECT * FROM pg_extension WHERE extname LIKE 'pg_%';
```

### PostgreSQL Configuration

Edit `postgresql.conf`:

```ini
# Required for pg_stat_statements
shared_preload_libraries = 'pg_stat_statements'

# Recommended settings
pg_stat_statements.max = 10000
pg_stat_statements.track = all
pg_stat_statements.track_utility = on
pg_stat_statements.save = on

# Enable timing information
track_io_timing = on
track_functions = 'all'
track_activity_query_size = 4096

# For pg_wait_sampling
shared_preload_libraries = 'pg_stat_statements,pg_wait_sampling'
```

Restart PostgreSQL after changes:
```bash
sudo systemctl restart postgresql
# or
pg_ctl restart
```

### Cloud Provider Limitations

#### AWS RDS
```sql
-- Check if running on RDS
SHOW rds.extensions;

-- Enable pg_stat_statements on RDS
-- (via parameter group, not CREATE EXTENSION)

-- Check RDS-specific settings
SELECT * FROM pg_settings WHERE name LIKE 'rds.%';
```

#### Google Cloud SQL
```sql
-- Check Cloud SQL indicators
SHOW cloudsql.iam_authentication;

-- Enable extensions via Cloud Console
-- Extensions must be enabled through the UI
```

#### Azure Database
```sql
-- Check Azure indicators
SELECT * FROM pg_settings WHERE name LIKE 'azure.%';

-- Enable extensions via Azure Portal
-- Some extensions have limitations
```

## MySQL Troubleshooting

### Performance Schema Setup

```sql
-- Check if Performance Schema is enabled
SELECT @@performance_schema;

-- Enable Performance Schema (requires restart)
-- Add to my.cnf:
-- [mysqld]
-- performance_schema = ON

-- Verify Performance Schema tables
SELECT TABLE_NAME 
FROM information_schema.TABLES 
WHERE TABLE_SCHEMA = 'performance_schema' 
  AND TABLE_NAME LIKE 'events_statements%';

-- Grant permissions
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
```

### MySQL User Permissions

```sql
-- Create monitoring user with necessary grants
CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'secure_password';

-- Basic monitoring grants
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON mysql.* TO 'otel_monitor'@'%';

-- For slow query log access
GRANT SELECT ON mysql.slow_log TO 'otel_monitor'@'%';

-- Flush privileges
FLUSH PRIVILEGES;
```

### Slow Query Log Configuration

```sql
-- Enable slow query log
SET GLOBAL slow_query_log = ON;
SET GLOBAL long_query_time = 1.0;
SET GLOBAL log_output = 'TABLE';

-- Verify settings
SHOW VARIABLES LIKE 'slow_query%';
SHOW VARIABLES LIKE 'long_query_time';
```

## Circuit Breaker Issues

### Queries Being Disabled

**Problem:** Queries are being disabled too aggressively

**Solution 1:** Adjust error patterns
```yaml
circuitbreaker:
  error_patterns:
    - pattern: "relation.*does not exist"
      action: disable_query  # Change to 'use_fallback'
      backoff: 30m          # Reduce backoff time
```

**Solution 2:** Increase failure threshold
```yaml
circuitbreaker:
  failure_threshold: 10  # Increase from 5
  success_threshold: 3
```

### Circuit Breaker Stuck Open

**Problem:** Circuit breaker won't close after fixing issues

**Solution:**
```bash
# Restart collector to reset circuit state
docker restart collector

# Or wait for backoff period to expire
# Check current backoff in metrics
curl -s http://localhost:8888/metrics | grep circuitbreaker_backoff_seconds
```

## Performance Problems

### High Memory Usage

**Problem:** Feature detection causing high memory usage

**Solutions:**

1. Increase cache duration to reduce detection frequency:
```yaml
feature_detection:
  cache_duration: 30m     # Increase from 5m
  refresh_interval: 2h    # Increase from 30m
```

2. Limit concurrent checks:
```yaml
feature_detection:
  max_concurrent_checks: 2  # Limit parallel detection
```

3. Adjust memory limits:
```yaml
processors:
  memory_limiter:
    limit_percentage: 75
    spike_limit_percentage: 20
```

### Slow Feature Detection

**Problem:** Feature detection takes too long

**Solutions:**

1. Skip unnecessary checks:
```yaml
feature_detection:
  skip_cloud_detection: true  # If not using cloud databases
  skip_version_check: true    # If version doesn't matter
```

2. Optimize detection queries:
```yaml
feature_detection:
  lightweight_mode: true  # Use simpler detection queries
```

3. Increase timeouts:
```yaml
feature_detection:
  timeout_per_check: 10s  # Increase from 3s
```

## Debugging Tools

### Enable Debug Logging

```yaml
service:
  telemetry:
    logs:
      level: debug
      encoding: json
      
# Or via environment variable
OTEL_LOG_LEVEL=debug
```

### Test Feature Detection

Create a test configuration:
```yaml
# test-feature-detection.yaml
receivers:
  enhancedsql/test:
    driver: postgres
    datasource: "..."
    feature_detection:
      enabled: true
      debug_mode: true  # Extra logging
    queries: []  # No queries, just detection

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [enhancedsql/test]
      exporters: [debug]
```

Run test:
```bash
./collector --config=test-feature-detection.yaml
```

### Manual Feature Check

```go
// test-features.go
package main

import (
    "github.com/database-intelligence-mvp/common/featuredetector"
)

func main() {
    detector := featuredetector.NewPostgreSQLDetector(db)
    features, err := detector.DetectFeatures(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Detected features: %+v\n", features)
}
```

## Resolution Steps

### Step 1: Identify the Problem

1. Check collector health:
   ```bash
   curl http://localhost:13133/health
   ```

2. Review metrics:
   ```bash
   curl http://localhost:8888/metrics | grep -E '(db_feature|circuitbreaker|error)'
   ```

3. Check logs:
   ```bash
   docker logs collector --tail 100 | grep -i error
   ```

### Step 2: Verify Database Setup

1. Test connection:
   ```bash
   psql -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT 1"
   ```

2. Check permissions:
   ```sql
   SELECT has_table_privilege('otel_monitor', 'pg_stat_statements', 'SELECT');
   ```

3. Verify extensions:
   ```sql
   SELECT * FROM pg_extension;
   ```

### Step 3: Fix Configuration

1. Validate configuration:
   ```bash
   ./collector validate --config=collector.yaml
   ```

2. Test with minimal config:
   ```yaml
   # minimal-test.yaml
   receivers:
     postgresql:
       endpoint: localhost:5432
       username: postgres
       password: postgres
   exporters:
     debug:
   service:
     pipelines:
       metrics:
         receivers: [postgresql]
         exporters: [debug]
   ```

### Step 4: Progressive Enhancement

1. Start with basic receiver
2. Add feature detection
3. Enable circuit breaker
4. Add custom queries
5. Enable all processors

### Step 5: Monitor and Adjust

1. Watch metrics:
   ```bash
   watch -n 5 'curl -s http://localhost:8888/metrics | grep db_feature'
   ```

2. Monitor logs:
   ```bash
   docker logs -f collector | grep -i feature
   ```

3. Adjust configuration based on results

## Getting Help

If issues persist:

1. **Collect Diagnostics:**
   ```bash
   ./scripts/collect-diagnostics.sh > diagnostics.txt
   ```

2. **Check Documentation:**
   - [Feature Detection Guide](./FEATURE_DETECTION.md)
   - [Configuration Reference](./CONFIGURATION.md)
   - [Architecture Overview](./ARCHITECTURE.md)

3. **Report Issues:**
   - Include diagnostics output
   - Provide configuration (sanitized)
   - Include relevant logs
   - Specify database versions

4. **Community Support:**
   - Slack: #database-intelligence
   - GitHub Issues: [Report Issue](https://github.com/database-intelligence-mvp/issues)