## Comprehensive E2E Test Strategy for New Relic Integration & OHI Feature Parity

# Comprehensive E2E Test Strategy for New Relic Integration & OHI Feature Parity

## Executive Summary

This document outlines the comprehensive end-to-end testing strategy for the Database Intelligence project, with a primary focus on ensuring New Relic compatibility and maintaining feature parity with the Infrastructure agent and Database On-Host Integration (OHI).

## Table of Contents

1. [Primary Testing Objectives](#primary-testing-objectives)
2. [OHI Feature Parity Test Suites](#ohi-feature-parity-test-suites)
3. [Integration Point Test Cases](#integration-point-test-cases)
4. [Processor Integration Tests for NR](#processor-integration-tests-for-nr)
5. [Comprehensive Test Scenarios](#comprehensive-test-scenarios)
6. [Test Data Requirements](#test-data-requirements)
7. [Validation Criteria](#validation-criteria)
8. [Test Execution Framework](#test-execution-framework)
9. [Success Metrics](#success-metrics)

## Primary Testing Objectives

- **Feature Parity**: Ensure 95%+ metric compatibility with Infrastructure agent and Database OHI
- **NRDB Data Model**: Validate OpenTelemetry metrics map correctly to New Relic's data model
- **Query Compatibility**: Existing NRQL queries and dashboards continue working
- **Entity Synthesis**: Database entities appear correctly in New Relic UI
- **Alert Migration**: Alert conditions trigger appropriately with new metrics

## OHI Feature Parity Test Suites

### A. Core Metric Validation Tests

#### PostgreSQL OHI Metrics Mapping
```yaml
PostgreSQL OHI → OpenTelemetry Mapping:
  - db.commitsPerSecond → postgresql.transactions.committed
  - db.rollbacksPerSecond → postgresql.transactions.rolled_back
  - db.bufferHitRatio → calculated from postgresql.blocks.hit/read
  - db.database.sizeInBytes → postgresql.database.size
  - db.connections.active → postgresql.connections.active
  - db.connections.max → postgresql.connections.max
  - db.replication.lagInBytes → postgresql.replication.lag_bytes
  - db.bgwriter.checkpointsScheduledPerSecond → postgresql.bgwriter.checkpoints_scheduled
  - db.bgwriter.buffersWrittenByBackgroundWriterPerSecond → postgresql.bgwriter.buffers_written
```

#### MySQL OHI Metrics Mapping
```yaml
MySQL OHI → OpenTelemetry Mapping:
  - db.innodb.bufferPoolDataPages → mysql.innodb.buffer_pool_pages.data
  - db.innodb.bufferPoolPagesFlushedPerSecond → mysql.innodb.buffer_pool_pages_flushed
  - db.queryCacheHitsPerSecond → mysql.query_cache.hits
  - db.queryCacheSizeInBytes → mysql.query_cache.size
  - db.handler.writePerSecond → mysql.handler.write
  - db.handler.readRndNextPerSecond → mysql.handler.read_rnd_next
  - db.replication.secondsBehindMaster → mysql.replication.seconds_behind_master
  - db.replication.lagInMilliseconds → mysql.replication.lag_ms
```

### B. Query Performance Data Tests

#### 1. pg_stat_statements Integration
- **Query Fingerprinting**: Verify query normalization matches OHI patterns
- **Execution Thresholds**: Test minimum execution count (20 calls) before reporting
- **Slow Query Detection**: Validate queries >500ms are properly flagged
- **Query Text Handling**: Ensure proper anonymization and PII removal

#### 2. Query Attributes Validation
```yaml
Required Attributes:
  - queryid → db.querylens.queryid
  - query_text → db.statement (anonymized)
  - execution_count → db.query.calls
  - avg_elapsed_time_ms → db.query.execution_time_mean
  - total_exec_time → db.query.total_time
  - rows → db.query.rows
  - plan_hash → db.plan.hash
```

### C. New Relic Data Model Tests

#### 1. Event Type Mapping
```sql
-- OHI Query Pattern
SELECT average(db.connections.active) 
FROM PostgreSQLSample 
WHERE hostname = 'prod-db-1'
SINCE 1 hour ago

-- OTEL Compatible Query
SELECT average(postgresql.connections.active) 
FROM Metric 
WHERE db.system = 'postgresql' 
  AND host.name = 'prod-db-1'
SINCE 1 hour ago
```

#### 2. Required Attributes for Entity Synthesis
```yaml
Entity Attributes:
  - entity.name: "database:postgresql:prod-db-1:production_db"
  - entity.type: "POSTGRESQL_DATABASE"
  - entity.guid: Auto-generated
  - nr.entity.guid: Same as entity.guid
  
Integration Metadata:
  - integration.name: "database-intelligence"
  - integration.version: "1.0.0"
  - instrumentation.provider: "opentelemetry"
  
Standard Dimensions:
  - hostname: Host identifier
  - database_name: Database name
  - db.system: "postgresql" or "mysql"
  - db.connection_string: Sanitized connection info
```

## Integration Point Test Cases

### A. OTLP Export Validation

#### Test 1: Metric Format Compliance
```go
func TestMetricFormatCompliance(t *testing.T) {
    // Verify metric naming conventions
    // - Hierarchical naming: postgresql.<category>.<metric>
    // - No spaces or special characters
    // - Lowercase with dots as separators
    
    // Check attribute cardinality
    // - Max 10,000 unique values per attribute
    // - Alert when approaching limits
    
    // Validate data point types
    // - Gauge: Current values (connections, size)
    // - Sum: Cumulative values (commits, rollbacks)
    // - Histogram: Distribution (query execution times)
    
    // Test metric batching
    // - Batch size limits
    // - Compression effectiveness
    // - Network efficiency
}
```

#### Test 2: Error Prevention
```go
func TestNrIntegrationErrorPrevention(t *testing.T) {
    // Attribute length validation
    // - Max 4096 characters per attribute value
    // - Truncation with ellipsis for longer values
    
    // Metric name validation
    // - Max 255 characters
    // - Valid character set
    
    // Rate limiting behavior
    // - Respect New Relic API limits
    // - Implement backoff strategies
    
    // Error handling
    // - Graceful degradation
    // - Error metric reporting
}
```

### B. Dashboard Compatibility Tests

#### Test 1: NRQL Query Validation
```yaml
Standard OHI Dashboard Queries:
  - Connection Monitoring:
      NRQL: "SELECT average(db.connections.active) FROM PostgreSQLSample FACET hostname"
      
  - Buffer Cache Performance:
      NRQL: "SELECT average(db.bufferHitRatio) FROM PostgreSQLSample TIMESERIES"
      
  - Replication Lag:
      NRQL: "SELECT max(db.replication.lagInBytes) FROM PostgreSQLSample WHERE db.replication.isPrimary = false"
      
  - Query Performance:
      NRQL: "SELECT average(avg_elapsed_time_ms) FROM PostgresSlowQueries FACET query_text"
```

#### Test 2: Visual Parity Tests
- Chart rendering with new metric names
- Time series continuity during migration
- Faceted query results
- Alert threshold visualization
- Entity relationship maps

### C. Alert Condition Tests

#### Test 1: Threshold-Based Alerts
```yaml
Alert Scenarios:
  - High Connection Count:
      Condition: postgresql.connections.active > 80% of max
      Duration: 5 minutes
      
  - Replication Lag:
      Condition: postgresql.replication.lag_bytes > 100MB
      Duration: 2 minutes
      
  - Query Performance Degradation:
      Condition: db.query.execution_time_mean > baseline * 2
      Duration: 10 minutes
      
  - Buffer Cache Hit Ratio:
      Condition: (blocks.hit / (blocks.hit + blocks.read)) < 0.90
      Duration: 15 minutes
```

#### Test 2: Anomaly Detection
- Baseline establishment from historical data
- Standard deviation calculations
- Seasonal pattern recognition
- Alert suppression during maintenance

## Processor Integration Tests for NR

### A. NR Error Monitor Processor

```yaml
Test Scenarios:
  - Long Attribute Values:
      Input: Query with 5000 character SQL
      Expected: Truncated to 4096 with "..." suffix
      
  - Invalid Metric Names:
      Input: "metric name with spaces!"
      Expected: Sanitized to "metric_name_with_spaces"
      
  - High Cardinality Detection:
      Input: 15,000 unique query patterns
      Expected: Warning at 10,000, filtering at 12,000
      
  - Metric Explosion Prevention:
      Input: Rapidly changing attribute values
      Expected: Rate limiting and aggregation
```

### B. Cost Control Processor

```yaml
New Relic Cost Optimization:
  - Data Volume Tracking:
      Price: $0.35/GB standard, $0.55/GB Data Plus
      Monitoring: Real-time GB usage
      
  - Cardinality Reduction Strategies:
      - Remove high-cardinality attributes
      - Aggregate similar queries
      - Sample non-critical metrics
      
  - Budget Enforcement:
      Thresholds:
        - 80%: Reduce sampling rates
        - 90%: Drop non-essential attributes
        - 95%: Emergency mode (critical metrics only)
```

### C. Verification Processor

```yaml
Data Quality for New Relic:
  - Required Field Validation:
      Fields: [db.name, db.operation, db.system, host.name]
      Action: Reject if missing
      
  - PII Detection/Redaction:
      Patterns: [email, phone, SSN, credit card]
      Action: Replace with "REDACTED_[TYPE]"
      
  - Entity Attribute Completeness:
      Required: [entity.name, entity.type]
      Action: Synthesize if missing
      
  - Metric Type Validation:
      Types: [gauge, sum, histogram]
      Action: Convert or reject invalid types
```

## Comprehensive Test Scenarios

### A. Side-by-Side Validation

```yaml
Test Configuration:
  Environment:
    - OHI Collector: Infrastructure agent v1.x
    - OTEL Collector: Database Intelligence v1.0
    - Target Database: Same PostgreSQL/MySQL instance
    
  Execution:
    - Duration: 24 hours
    - Workload: Production-like patterns
    - Collection: Parallel to NRDB
    
  Validation:
    - Metric presence: 100% coverage
    - Value accuracy: ±5% tolerance
    - Timing alignment: <1 minute skew
    - Attribute mapping: Complete
```

### B. Migration Simulation

```yaml
Migration Phases:
  Phase 1 - Baseline (Week 1):
    - OHI only collection
    - Establish performance baselines
    - Document current dashboards/alerts
    
  Phase 2 - Dual-Run (Week 2-3):
    - Both collectors active
    - Compare metrics in NRDB
    - Validate dashboard compatibility
    - Test alert conditions
    
  Phase 3 - OTEL Primary (Week 4):
    - OTEL as primary, OHI as backup
    - Monitor for gaps or issues
    - User acceptance testing
    
  Phase 4 - Cutover (Week 5):
    - OHI disabled
    - OTEL only
    - Final validation
    - Performance comparison
```

### C. Load and Scale Tests

```yaml
Test Scenarios:
  Small Scale:
    - Databases: 10
    - Queries/sec: 1,000
    - Metrics/min: 10,000
    
  Medium Scale:
    - Databases: 50
    - Queries/sec: 5,000
    - Metrics/min: 50,000
    
  Large Scale:
    - Databases: 250+
    - Queries/sec: 10,000+
    - Metrics/min: 100,000+
    
  Stress Conditions:
    - High cardinality: 50,000+ unique queries
    - PII-heavy: 30% queries with sensitive data
    - Plan changes: 100+ per hour
    - Connection churn: 1000+ connects/disconnects per minute
```

## Test Data Requirements

### A. Realistic Workloads

```sql
-- OLTP Patterns (60% of load)
-- High-frequency simple queries
SELECT * FROM users WHERE id = ?;
UPDATE orders SET status = ? WHERE order_id = ?;
INSERT INTO events (user_id, event_type, timestamp) VALUES (?, ?, ?);

-- OLAP Patterns (30% of load)
-- Complex analytical queries
SELECT 
    date_trunc('hour', created_at) as hour,
    COUNT(*) as order_count,
    SUM(total_amount) as revenue
FROM orders
WHERE created_at >= NOW() - INTERVAL '7 days'
GROUP BY hour
ORDER BY hour;

-- Maintenance Operations (10% of load)
VACUUM ANALYZE large_table;
CREATE INDEX CONCURRENTLY idx_new ON table(column);
ANALYZE table_name;
```

### B. Edge Cases and Error Conditions

```sql
-- PII Data Patterns
SELECT * FROM users WHERE email = 'john.doe@example.com';
SELECT * FROM customers WHERE ssn = '123-45-6789';
UPDATE accounts SET credit_card = '4111-1111-1111-1111' WHERE id = ?;

-- Malformed Queries
SELECT * FORM users;  -- Syntax error
SELECT * FROM non_existent_table;  -- Table not found

-- Performance Edge Cases
-- Extremely long query (>4096 chars)
SELECT ... FROM table1 JOIN table2 ... JOIN table50 ...;

-- Unicode and special characters
SELECT * FROM products WHERE name = '🚀 Rocket™ Widget®';

-- Plan Regression Scenarios
-- Force different execution plans
SET enable_seqscan = off;
SELECT * FROM large_table WHERE non_indexed_column = ?;
```

## Validation Criteria

### A. Metric Accuracy Requirements

| Metric Type | Tolerance | Validation Method |
|------------|-----------|-------------------|
| Counters | Exact match | Delta comparison |
| Gauges | ±5% | Statistical analysis |
| Rates | ±5% | Time-window average |
| Percentiles | ±10% | Distribution comparison |

### B. Feature Coverage Checklist

- ✅ **Core Metrics**
  - ✓ Connection metrics
  - ✓ Transaction rates
  - ✓ Buffer cache statistics
  - ✓ Database sizes
  - ✓ Replication lag

- ✅ **Query Intelligence**
  - ✓ Query performance metrics
  - ✓ Execution plans
  - ✓ Plan change detection
  - ✓ Query fingerprinting
  - ✓ Slow query log

- ✅ **Advanced Features**
  - ✓ Wait event analysis
  - ✓ Blocking detection
  - ✓ PII protection
  - ✓ Cost optimization
  - ✓ Circuit breaker protection

### C. Operational Requirements

| Requirement | Target | Measurement |
|-------------|--------|-------------|
| Dashboard Compatibility | 100% | NRQL query success |
| Alert Functionality | 100% | Alert trigger accuracy |
| Entity Synthesis | 100% | UI entity presence |
| API Compatibility | 100% | API response validation |
| Data Latency | <60s | End-to-end timing |

## Test Execution Framework

### A. Automated Validation Pipeline

```bash
#!/bin/bash
# E2E Test Execution Pipeline

# Phase 1: Infrastructure Setup (5 minutes)
echo "=== Phase 1: Infrastructure Setup ==="
- Deploy PostgreSQL and MySQL test instances
- Configure OHI and OTEL collectors
- Initialize workload generators
- Verify connectivity

# Phase 2: Data Collection (30 minutes)
echo "=== Phase 2: Data Collection ==="
- Generate OLTP workload patterns
- Execute OLAP queries
- Inject error scenarios
- Create PII test data
- Monitor collection rates

# Phase 3: Validation (15 minutes)
echo "=== Phase 3: Validation ==="
- Query NRDB for OHI metrics
- Query NRDB for OTEL metrics
- Compare metric values
- Validate attributes
- Generate accuracy reports

# Phase 4: Compatibility Testing (10 minutes)
echo "=== Phase 4: Compatibility Testing ==="
- Execute dashboard NRQL queries
- Test alert conditions
- Verify entity synthesis
- Check UI integration
```

### B. Continuous Validation Schedule

```yaml
Daily Tests (2 hours):
  - Core metric accuracy validation
  - Dashboard query compatibility
  - Alert condition verification
  - Basic load testing
  
Weekly Tests (8 hours):
  - Full OHI parity validation
  - Scale testing (250 databases)
  - Security/PII validation
  - Performance benchmarking
  
Release Tests (24 hours):
  - Complete migration simulation
  - Breaking change detection
  - Regression testing
  - User acceptance testing
  
Monthly Tests (48 hours):
  - Disaster recovery scenarios
  - Long-running stability tests
  - Memory leak detection
  - Cost projection validation
```

## Success Metrics

### Quantitative Metrics

| Metric | Target | Critical Threshold |
|--------|--------|-------------------|
| Metric Parity | ≥95% | <90% |
| Query Compatibility | 100% | <95% |
| Data Delivery | ≥99.9% | <99% |
| Processing Latency | <5ms p95 | >10ms p95 |
| Memory Overhead | <5% vs OHI | >10% vs OHI |
| CPU Overhead | <5% vs OHI | >10% vs OHI |

### Qualitative Metrics

- **Zero Feature Regression**: No loss of functionality from OHI
- **Enhanced Capabilities**: New features add value (plan intelligence, circuit breaker)
- **User Experience**: Seamless migration with no dashboard/alert disruption
- **Operational Simplicity**: Easier to deploy and manage than OHI
- **Cost Efficiency**: Lower or equal New Relic ingestion costs

## Appendix

### A. Test Configuration Files

```yaml
# e2e-newrelic-test-config.yaml
test_suites:
  ohi_parity:
    enabled: true
    comparison_tolerance: 0.05
    metric_mappings: "config/ohi-metric-mappings.yaml"
    
  metric_accuracy:
    enabled: true
    collection_duration: 30m
    validation_queries: "config/nrql-validation-queries.yaml"
    
  query_performance:
    enabled: true
    workload_profile: "production"
    slow_query_threshold: 500ms
    
  scale_testing:
    enabled: true
    database_count: 250
    queries_per_second: 10000
```

### B. Validation Query Examples

```sql
-- Validate connection metrics
SELECT 
  average(postgresql.connections.active) as otel_active,
  average(db.connections.active) as ohi_active,
  abs(1 - (average(postgresql.connections.active) / average(db.connections.active))) as variance
FROM Metric, PostgreSQLSample
WHERE hostname = 'test-host'
SINCE 30 minutes ago

-- Validate query performance data
SELECT 
  count(*) as query_count,
  average(db.query.execution_time_mean) as avg_exec_time,
  max(db.query.execution_time_max) as max_exec_time
FROM Metric
WHERE db.system = 'postgresql'
  AND db.querylens.queryid IS NOT NULL
SINCE 1 hour ago
FACET db.querylens.queryid
```

### C. Troubleshooting Guide

Common issues and resolutions:

1. **Missing Metrics**
   - Check collector logs for errors
   - Verify OTLP endpoint configuration
   - Confirm New Relic license key

2. **Metric Value Discrepancies**
   - Review unit conversions (seconds vs milliseconds)
   - Check rate calculations
   - Verify collection intervals

3. **Dashboard Incompatibility**
   - Update NRQL queries for new metric names
   - Add db.system filter for multi-database queries
   - Check attribute name changes

4. **Alert Failures**
   - Reconfigure thresholds for new metric names
   - Update FACET clauses
   - Verify entity filtering

This comprehensive strategy ensures complete New Relic compatibility while validating all OHI features are preserved or enhanced in the OpenTelemetry implementation.
