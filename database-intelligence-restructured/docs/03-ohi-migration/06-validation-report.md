## OHI to OpenTelemetry Validation Report

# OHI to OpenTelemetry Validation Report

## Executive Summary

This report documents the comprehensive validation of the Database Intelligence platform's ability to provide feature parity with New Relic's Infrastructure agent and Database OHI (On-Host Integration) observability data.

### Key Findings

1. **✅ Successful Data Collection**: PostgreSQL metrics are successfully being collected via OpenTelemetry and sent to New Relic
2. **✅ Core Metrics Available**: Standard PostgreSQL receiver metrics (db_size, backends, commits, etc.) are working correctly
3. **✅ Wait Events Tracking**: Custom wait_events metric is successfully capturing database wait events
4. **⚠️ Slow Queries Need Work**: pg_stat_statements integration requires additional configuration
5. **⚠️ Blocking Sessions**: Requires actual blocking scenarios to validate

## Validation Details

### 1. Environment Setup

- **PostgreSQL**: Version 15.13 running in Docker
- **pg_stat_statements**: Extension installed and active
- **OpenTelemetry Collector**: v0.91.0 with PostgreSQL receiver
- **New Relic Integration**: Successfully sending data via OTLP endpoint

### 2. Metrics Validation

#### Successfully Validated Metrics

| Metric Name | OHI Equivalent | Status | Notes |
|------------|----------------|---------|-------|
| postgresql.db_size | Database disk usage | ✅ Working | Average: 7.68MB |
| postgresql.backends | Active connections | ✅ Working | Average: 5.3 connections |
| postgresql.commits | Transaction commits | ✅ Working | Rate: 48.8/min |
| postgresql.bgwriter.buffers.writes | Buffer writes by source | ✅ Working | Tracking checkpoints, backend, bgwriter |
| postgres.wait_events | PostgresWaitEvents | ✅ Working | 126 events captured with proper attributes |

#### Metrics Requiring Additional Work

| Metric Name | OHI Event | Issue | Solution |
|-------------|-----------|-------|----------|
| postgres.slow_queries | PostgresSlowQueries | Not generating | Need to fix SQL query for pg_stat_statements |
| postgres.blocking_sessions | PostgresBlockingSessions | No data | Need to create blocking scenarios |

### 3. Dashboard Widget Compatibility

Analyzed 14 widgets from the PostgreSQL OHI dashboard:

#### Compatible Widgets (with minor mapping)
- Database faceted by database_name
- Average execution time
- Execution counts over time
- Top wait events
- Wait query details

#### Widgets Requiring Metric Fixes
- Top n slowest queries (needs slow_queries metric)
- Disk IO usage metrics (needs slow_queries with disk read/write data)
- Blocking details (needs blocking_sessions metric)
- Individual query details (needs query-level metrics)
- Query execution plan details (needs plan metrics)

### 4. Technical Issues Encountered and Resolved

1. **Environment Variable Loading**: Manual .env loader implemented
2. **NRDB Query Format**: Fixed GraphQL schema issues
3. **PostgreSQL Authentication**: Corrected password configuration
4. **pg_stat_statements**: Successfully installed extension
5. **Docker Networking**: Used existing PostgreSQL container

### 5. Data Volume Statistics

Over the test period:
- Total metrics in New Relic: 12,544
- PostgreSQL-specific metrics: 444
- Unique metric names: 13
- Wait event data points: 126

## Recommendations

### Immediate Actions

1. **Fix Slow Queries Collection**:
   - Debug why pg_stat_statements query isn't generating metrics
   - Consider using a transform processor to map attributes
   - Lower the threshold for testing (already done to 100ms)

2. **Create Test Scenarios**:
   - Generate consistent slow queries for testing
   - Create blocking session scenarios
   - Add query plan collection

3. **Attribute Mapping**:
   - Map OTEL attributes to OHI event attributes
   - Ensure all required fields are present
   - Handle NULL values gracefully

### Long-term Improvements

1. **Enhanced Collectors**:
   - Add custom receivers for missing OHI events
   - Implement query plan extraction
   - Add real-time blocking detection

2. **Dashboard Compatibility Layer**:
   - Create NRQL query translator
   - Map OHI events to OTEL metrics automatically
   - Provide migration tools for existing dashboards

3. **Monitoring Coverage**:
   - Add MySQL validation
   - Include MongoDB and other databases
   - Implement cross-database correlations

## Test Artifacts

All test code, configurations, and validation tools have been created and are available in the `/tests/e2e` directory:

- `cmd/test_connectivity_with_env/`: Basic connectivity testing
- `cmd/simple_validation/`: Dashboard parsing and basic validation
- `cmd/check_newrelic_data/`: New Relic data verification
- `cmd/validate_ohi_mapping/`: OHI to OTEL mapping validation
- `configs/collector-test.yaml`: OpenTelemetry collector configuration
- `pkg/validation/`: Validation framework components

## Conclusion

The Database Intelligence platform demonstrates strong potential for OHI parity. Core metrics are successfully collected and transmitted to New Relic. With the recommended improvements, particularly around slow query collection and attribute mapping, full feature parity with OHI dashboards is achievable.

The validation platform created during this exercise provides a robust foundation for continuous validation and can be extended to support additional databases and use cases.
