# Feature Detection and Graceful Fallback

The Database Intelligence Collector includes sophisticated feature detection and graceful fallback mechanisms to ensure reliable monitoring across diverse database environments.

## Overview

Feature detection automatically identifies available database extensions, capabilities, and cloud providers to:
- Select optimal queries for data collection
- Gracefully degrade when features are missing
- Prevent errors from missing extensions
- Adapt to cloud provider limitations

## Architecture

```
┌─────────────────┐     ┌──────────────────┐     ┌────────────────┐
│                 │     │                  │     │                │
│  Feature        │────▶│  Query           │────▶│  Enhanced SQL  │
│  Detector       │     │  Selector        │     │  Receiver      │
│                 │     │                  │     │                │
└────────┬────────┘     └──────────────────┘     └────────────────┘
         │                                                 │
         │              ┌──────────────────┐              │
         └─────────────▶│                  │◀─────────────┘
                        │  Circuit Breaker │
                        │  (Feature Aware) │
                        │                  │
                        └──────────────────┘
```

## Components

### 1. Feature Detector

Automatically detects:
- **Extensions**: pg_stat_statements, pg_stat_monitor, pg_wait_sampling, etc.
- **Capabilities**: track_io_timing, performance_schema, slow_query_log
- **Cloud Providers**: AWS RDS, Aurora, Google Cloud SQL, Azure Database
- **Permissions**: Read access to system views and tables

### 2. Query Selector

Selects optimal queries based on:
- Available features (extensions, capabilities)
- Priority ordering (prefer advanced features)
- Fallback strategies (use basic queries when advanced unavailable)
- Performance considerations

### 3. Enhanced SQL Receiver

A feature-aware SQL receiver that:
- Performs initial feature detection
- Caches detection results
- Automatically refreshes feature status
- Emits feature availability metrics

### 4. Feature-Aware Circuit Breaker

Protects databases by:
- Recognizing feature-related errors
- Disabling queries that require missing features
- Implementing exponential backoff
- Providing fallback query suggestions

## Configuration

### Basic Configuration

```yaml
receivers:
  enhancedsql/postgresql:
    driver: postgres
    datasource: "..."
    
    feature_detection:
      enabled: true                    # Enable feature detection
      cache_duration: 5m              # Cache results for 5 minutes
      refresh_interval: 30m           # Refresh features every 30 minutes
      timeout_per_check: 3s           # Timeout for each check
      retry_attempts: 3               # Retry failed detections
      skip_cloud_detection: false     # Detect cloud providers
```

### Custom Query Definitions

```yaml
custom_queries:
  - name: advanced_slow_queries
    category: slow_queries
    priority: 100                     # Higher priority = preferred
    sql: "SELECT ... FROM pg_stat_monitor ..."
    requirements:
      required_extensions: ["pg_stat_monitor"]
      required_capabilities: ["track_io_timing"]
    
  - name: basic_slow_queries
    category: slow_queries
    priority: 10                      # Lower priority = fallback
    sql: "SELECT ... FROM pg_stat_activity ..."
    requirements: []                  # No special requirements
```

### Circuit Breaker Error Patterns

```yaml
processors:
  circuitbreaker:
    error_patterns:
      - pattern: "relation.*does not exist"
        action: disable_query         # Disable the query
        feature: extension
        backoff: 30m                  # Wait 30 minutes before retry
        
      - pattern: "permission denied"
        action: disable_query
        feature: permissions
        backoff: 1h
        
      - pattern: "extension.*not installed"
        action: use_fallback          # Try fallback query
        feature: extension
        backoff: 5m
```

## PostgreSQL Feature Detection

### Extensions Detected

| Extension | Purpose | Fallback Strategy |
|-----------|---------|-------------------|
| pg_stat_statements | Query performance statistics | Use pg_stat_activity for current queries |
| pg_stat_monitor | Advanced query metrics with percentiles | Fall back to pg_stat_statements |
| pg_wait_sampling | Wait event sampling | Use pg_stat_activity wait events |
| auto_explain | Query plan logging | Disable plan collection |
| pg_stat_kcache | Kernel-level metrics | Disable OS metrics |

### Capabilities Detected

| Capability | Purpose | Detection Method |
|------------|---------|------------------|
| track_io_timing | I/O timing in query stats | SHOW track_io_timing |
| track_functions | Function call statistics | SHOW track_functions |
| shared_preload_libraries | Loaded extensions | SHOW shared_preload_libraries |
| statement_timeout | Query timeout support | SHOW statement_timeout |

### Cloud Provider Detection

```sql
-- AWS RDS Detection
SELECT setting FROM pg_settings 
WHERE name = 'rds.superuser_reserved_connections';

-- Aurora Detection
SELECT aurora_version();

-- Google Cloud SQL Detection
SHOW cloudsql.iam_authentication;

-- Azure Database Detection
SELECT setting FROM pg_settings 
WHERE name LIKE 'azure.%';
```

## MySQL Feature Detection

### Features Detected

| Feature | Purpose | Fallback Strategy |
|---------|---------|-------------------|
| performance_schema | Detailed performance metrics | Use SHOW STATUS |
| events_statements_summary | Query digest statistics | Use PROCESSLIST |
| slow_query_log | Historical slow queries | Use current queries |
| innodb_metrics | InnoDB statistics | Use SHOW ENGINE INNODB STATUS |

### Capabilities Detected

| Capability | Detection Method | Fallback |
|------------|------------------|----------|
| performance_schema_enabled | SELECT @@performance_schema | Basic metrics only |
| slow_query_log | SELECT @@slow_query_log | No historical data |
| query_cache_type | SELECT @@query_cache_type | No cache metrics |

## Query Priority System

Queries are selected based on priority (higher = better):

```yaml
# Priority 100: Most advanced features
pg_stat_monitor with CPU metrics, percentiles, WAL stats

# Priority 90: Advanced features
pg_stat_statements with I/O timing

# Priority 50: Basic features
pg_stat_statements without timing

# Priority 10: Fallback
pg_stat_activity (current queries only)
```

## Graceful Degradation Examples

### Example 1: Missing pg_stat_statements

```yaml
# Attempted query (fails)
SELECT * FROM pg_stat_statements

# Error detected
ERROR: relation "pg_stat_statements" does not exist

# Action taken
- Query disabled for 30 minutes
- Fallback to pg_stat_activity query
- Metric emitted: db.feature.extension.available{extension="pg_stat_statements"} = 0
```

### Example 2: RDS Limitations

```yaml
# Attempted feature
pg_wait_sampling extension

# Error detected
ERROR: feature not supported on AWS RDS

# Action taken
- Feature marked unavailable
- Wait event collection disabled
- Use basic pg_stat_activity wait info
```

### Example 3: Permission Issues

```yaml
# Attempted query
SELECT * FROM performance_schema.events_statements_summary_by_digest

# Error detected
ERROR: Access denied for user

# Action taken
- Query disabled for 1 hour
- Fallback to PROCESSLIST
- Alert generated for missing permissions
```

## Metrics Emitted

### Feature Availability Metrics

```prometheus
# Extension availability
db.feature.extension.available{extension="pg_stat_statements",version="1.10"} 1
db.feature.extension.available{extension="pg_wait_sampling"} 0

# Capability availability
db.feature.capability.available{capability="track_io_timing",value="on"} 1
db.feature.capability.available{capability="performance_schema"} 1

# Detection health
db.feature.detection.errors{} 2
db.feature.detection.age{} 300  # seconds since last detection
db.feature.total{type="all"} 15
db.feature.total{type="available"} 12

# Query compatibility
db.query.compatibility{category="slow_queries"} 1
db.query.compatibility{category="wait_events"} 0
```

### Collector Operational Metrics

```prometheus
# Fallback usage
enhancedsql.fallback.count{query="slow_queries"} 5

# Disabled queries
circuitbreaker.disabled_queries{feature="pg_stat_statements"} 1

# Feature-related errors
circuitbreaker.feature_errors{feature="extension",pattern="not_exist"} 10
```

## Best Practices

### 1. Enable Required Extensions

```sql
-- PostgreSQL
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET track_io_timing = on;
SELECT pg_reload_conf();

-- MySQL
SET GLOBAL performance_schema = ON;
SET GLOBAL slow_query_log = ON;
```

### 2. Grant Necessary Permissions

```sql
-- PostgreSQL
GRANT pg_monitor TO monitoring_user;
GRANT SELECT ON pg_stat_statements TO monitoring_user;

-- MySQL
GRANT SELECT ON performance_schema.* TO 'monitoring_user'@'%';
GRANT PROCESS ON *.* TO 'monitoring_user'@'%';
```

### 3. Configure Appropriate Timeouts

```yaml
feature_detection:
  cache_duration: 5m      # Frequent for development
  # cache_duration: 1h    # Less frequent for production
  
  refresh_interval: 30m   # How often to recheck features
  timeout_per_check: 3s   # Prevent long-running detection
```

### 4. Monitor Feature Detection

Create alerts for:
- High detection error rates
- Critical features unavailable
- Excessive fallback usage
- Disabled queries

### 5. Test Fallback Scenarios

```bash
# Test with minimal PostgreSQL
docker run -d postgres:15-alpine

# Test with full-featured PostgreSQL
docker run -d postgres:15 \
  -c shared_preload_libraries=pg_stat_statements,auto_explain

# Test with RDS-like restrictions
# Use custom test image that mimics RDS limitations
```

## Troubleshooting

### Check Current Feature Status

```yaml
# View feature detection metrics
curl http://localhost:8888/metrics | grep db.feature

# Check collector logs
docker logs collector | grep -i feature

# View disabled queries
curl http://localhost:8888/metrics | grep disabled_queries
```

### Common Issues

1. **Extension exists but queries fail**
   - Check permissions: User may lack SELECT on extension views
   - Check search_path: Extension may be in different schema

2. **Feature detection times out**
   - Increase timeout_per_check
   - Check database load
   - Verify network connectivity

3. **Fallback queries perform poorly**
   - Tune fallback query parameters
   - Consider custom query definitions
   - Adjust collection intervals

4. **Circuit breaker too aggressive**
   - Adjust error patterns
   - Increase backoff times
   - Check for transient errors

## Future Enhancements

1. **Automatic Extension Installation**
   - Detect missing extensions
   - Provide installation commands
   - Automate where possible

2. **Performance Impact Analysis**
   - Measure overhead of each query type
   - Automatically adjust collection frequency
   - Recommend optimal feature set

3. **Cloud-Specific Optimizations**
   - RDS Enhanced Monitoring integration
   - CloudWatch metrics correlation
   - Azure Monitor integration

4. **Machine Learning Integration**
   - Predict feature availability
   - Optimize query selection
   - Anomaly detection for feature changes