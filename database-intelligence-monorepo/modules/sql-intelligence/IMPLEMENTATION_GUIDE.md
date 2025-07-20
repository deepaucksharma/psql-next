# SQL Intelligence Module - Implementation Guide

## Overview

This guide provides step-by-step instructions for implementing the required changes to fix the sql-intelligence module's critical flaws and transform it into a true query intelligence engine.

## Pre-Implementation Checklist

- [ ] Backup current configuration state
- [ ] Document any custom modifications in existing configs
- [ ] Ensure test environment is available
- [ ] Review REQUIRED_CHANGES.md document
- [ ] Have New Relic API access for validation

## Step 1: Configuration Cleanup (30 minutes)

### 1.1 Create Archive and Backup

```bash
cd modules/sql-intelligence/config

# Create archive directory
mkdir -p archive/$(date +%Y%m%d)

# Move all backup files
mv *.backup-* archive/$(date +%Y%m%d)/

# Archive redundant configs
cp collector-enhanced.yaml archive/$(date +%Y%m%d)/
cp collector-enterprise*.yaml archive/$(date +%Y%m%d)/
cp collector-plan-intelligence.yaml archive/$(date +%Y%m%d)/
```

### 1.2 Extract Valuable Logic

Before deleting files, extract valuable components:

```bash
# Extract the comprehensive SQL query from plan-intelligence
grep -A 500 "COMPREHENSIVE_QUERY" collector-plan-intelligence.yaml > /tmp/comprehensive-query.sql

# Extract advanced processors
grep -A 50 "transform/query_plan_analysis" collector-plan-intelligence.yaml > /tmp/advanced-processors.yaml
```

### 1.3 Clean Configuration Directory

```bash
# Remove redundant files
rm -f collector-enhanced.yaml
rm -f collector-enterprise*.yaml
rm -f collector-plan-intelligence.yaml

# Verify only essential files remain
ls -la
# Should show: collector.yaml, collector-test.yaml, (maybe) business-mappings.yaml
```

## Step 2: Rebuild collector.yaml (1 hour)

### 2.1 Create New Unified Configuration

Create a new collector.yaml that combines the best features:

```yaml
# SQL Intelligence - Consolidated Configuration
# Comprehensive query performance analysis and SQL intelligence metrics

receivers:
  # Enhanced slow query analysis with comprehensive metrics
  sqlquery/slow_queries:
    driver: mysql
    datasource: "${env:MYSQL_USER}:${env:MYSQL_PASSWORD}@tcp(${env:MYSQL_ENDPOINT})/performance_schema"
    collection_interval: 15s
    queries:
      - sql: |
          SELECT 
            DIGEST,
            SCHEMA_NAME,
            DIGEST_TEXT,
            COUNT_STAR as exec_count,
            SUM_TIMER_WAIT/1000000000 as total_latency_ms,
            AVG_TIMER_WAIT/1000000000 as avg_latency_ms,
            MAX_TIMER_WAIT/1000000000 as max_latency_ms,
            MIN_TIMER_WAIT/1000000000 as min_latency_ms,
            SUM_ROWS_EXAMINED as rows_examined_total,
            AVG_ROWS_EXAMINED as rows_examined_avg,
            SUM_ROWS_SENT as rows_sent_total,
            AVG_ROWS_SENT as rows_sent_avg,
            SUM_NO_INDEX_USED as no_index_used_count,
            SUM_NO_GOOD_INDEX_USED as no_good_index_count,
            SUM_SORT_ROWS as sort_rows_total,
            SUM_SORT_SCAN as sort_scan_count,
            SUM_CREATED_TMP_TABLES as tmp_tables_created,
            SUM_CREATED_TMP_DISK_TABLES as tmp_disk_tables_created,
            FIRST_SEEN,
            LAST_SEEN,
            QUANTILE_95,
            QUANTILE_99
          FROM performance_schema.events_statements_summary_by_digest
          WHERE SCHEMA_NAME IS NOT NULL
            AND DIGEST_TEXT NOT LIKE '%performance_schema%'
            AND COUNT_STAR > 0
            AND (
              SUM_TIMER_WAIT > 1000000000  -- Total time > 1 second
              OR SUM_NO_INDEX_USED > 100   -- Frequent missing indexes
              OR AVG_ROWS_EXAMINED / GREATEST(AVG_ROWS_SENT, 1) > 100  -- Inefficient
              OR SUM_CREATED_TMP_DISK_TABLES > 0  -- Disk temp tables
            )
          ORDER BY (SUM_TIMER_WAIT * COUNT_STAR) DESC
          LIMIT 100
        metrics:
          # ... (all metrics as shown in consolidated config)

  # Query plan analysis for intelligence
  sqlquery/query_plans:
    driver: mysql
    datasource: "${env:MYSQL_USER}:${env:MYSQL_PASSWORD}@tcp(${env:MYSQL_ENDPOINT})/performance_schema"
    collection_interval: 30s
    queries:
      - sql: |
          WITH query_analysis AS (
            SELECT 
              DIGEST,
              COUNT_STAR as exec_count,
              SUM_TIMER_WAIT/1000000000 as total_time_ms,
              AVG_TIMER_WAIT/1000000000 as avg_time_ms,
              SUM_ROWS_EXAMINED as rows_examined,
              SUM_ROWS_SENT as rows_sent,
              SUM_NO_INDEX_USED as no_index_used,
              CASE 
                WHEN SUM_ROWS_SENT > 0 THEN 
                  SUM_ROWS_EXAMINED / SUM_ROWS_SENT 
                ELSE 999999 
              END as examination_ratio,
              CASE
                WHEN COUNT_STAR > 0 AND SUM_NO_INDEX_USED > 0 THEN
                  (SUM_NO_INDEX_USED / COUNT_STAR) * 100
                ELSE 0
              END as no_index_percentage
            FROM performance_schema.events_statements_summary_by_digest
            WHERE SCHEMA_NAME IS NOT NULL
              AND COUNT_STAR > 10  -- Minimum execution threshold
          )
          SELECT 
            DIGEST,
            exec_count,
            avg_time_ms,
            examination_ratio,
            no_index_percentage,
            CASE
              WHEN avg_time_ms > 1000 THEN 'critical'
              WHEN avg_time_ms > 500 THEN 'high'
              WHEN avg_time_ms > 100 THEN 'medium'
              ELSE 'low'
            END as latency_severity,
            CASE
              WHEN examination_ratio > 1000 THEN 'optimize_query'
              WHEN no_index_percentage > 50 THEN 'add_index'
              WHEN avg_time_ms > 1000 THEN 'review_query'
              ELSE 'ok'
            END as recommendation_type
          FROM query_analysis
          ORDER BY (avg_time_ms * exec_count) DESC
          LIMIT 50
        metrics:
          - metric_name: mysql.query.intelligence.score
            value_column: "examination_ratio"
            attribute_columns: [DIGEST, latency_severity, recommendation_type]

# ... rest of configuration following single pipeline pattern
```

### 2.2 Implement Single Pipeline Architecture

Replace the dual pipeline with a single, intelligent pipeline:

```yaml
service:
  pipelines:
    metrics:
      receivers: [
        sqlquery/slow_queries,
        sqlquery/table_stats,
        sqlquery/index_usage,
        sqlquery/table_locks,
        sqlquery/schema_stats,
        sqlquery/query_plans,
        prometheus/core_metrics,
        otlp
      ]
      processors: [
        memory_limiter,
        batch,
        attributes,
        resource,
        transform/query_intelligence,
        transform/impact_scoring,
        transform/recommendations,
        metrictransform/standardize,
        attributes/newrelic,
        attributes/entity_synthesis,
        routing/priority
      ]
      exporters: [otlphttp/newrelic, prometheus, debug]
```

## Step 3: Implement Intelligence Processors (2 hours)

### 3.1 Query Intelligence Processor

Add comprehensive analysis:

```yaml
processors:
  transform/query_intelligence:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Index Efficiency Score (0-100)
          - set(attributes["index_efficiency_score"], 
                100 * (1 - (attributes["no_index_used_count"] / attributes["exec_count"])))
            where attributes["exec_count"] > 0 and name == "mysql.query.exec_total"
          
          # Query Cost Score 
          - set(attributes["query_cost_score"], 
                attributes["rows_examined_avg"] / Greatest(attributes["rows_sent_avg"], 1))
            where attributes["rows_sent_avg"] != nil
          
          # Temp Table Impact Score
          - set(attributes["temp_table_impact"], 
                (attributes["tmp_disk_tables_created"] * 100) + 
                (attributes["tmp_tables_created"] * 10))
            where name == "mysql.query.exec_total"
          
          # Calculate overall efficiency (0-100, lower is better)
          - set(attributes["query_efficiency_score"],
                (attributes["query_cost_score"] * 0.4) +
                ((100 - attributes["index_efficiency_score"]) * 0.4) +
                (attributes["temp_table_impact"] * 0.2))
            where attributes["query_cost_score"] != nil
```

### 3.2 Impact Scoring Processor

Determine business impact:

```yaml
  transform/impact_scoring:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Calculate query impact based on frequency and latency
          - set(attributes["execution_impact"], 
                attributes["exec_count"] * attributes["avg_latency_ms"] / 1000)
            where name == "mysql.query.latency_avg_ms"
          
          # Determine severity
          - set(attributes["severity"], "critical") 
            where attributes["execution_impact"] > 10000 or 
                  attributes["query_efficiency_score"] > 80
          
          - set(attributes["severity"], "high")
            where attributes["severity"] == nil and 
                  (attributes["execution_impact"] > 1000 or 
                   attributes["query_efficiency_score"] > 60)
          
          - set(attributes["severity"], "medium")
            where attributes["severity"] == nil and 
                  (attributes["execution_impact"] > 100 or 
                   attributes["query_efficiency_score"] > 40)
          
          - set(attributes["severity"], "low")
            where attributes["severity"] == nil
```

### 3.3 Recommendations Processor

Generate actionable insights:

```yaml
  transform/recommendations:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Index recommendations
          - set(attributes["recommendation.type"], "add_index")
            where attributes["no_index_used_count"] > 0 and 
                  attributes["index_efficiency_score"] < 50
          
          - set(attributes["recommendation.text"], 
                Concat(["Query missing index (", 
                       ToString(attributes["no_index_used_count"]), 
                       " executions without index). Consider adding index for: ",
                       attributes["DIGEST_TEXT"]], ""))
            where attributes["recommendation.type"] == "add_index"
          
          # Query optimization recommendations
          - set(attributes["recommendation.type"], "optimize_query")
            where attributes["query_cost_score"] > 100
          
          - set(attributes["recommendation.text"],
                Concat(["Query examines ", 
                       ToString(attributes["query_cost_score"]), 
                       "x more rows than it returns. Review query logic and JOIN conditions."], ""))
            where attributes["recommendation.type"] == "optimize_query"
          
          # Temp table recommendations
          - set(attributes["recommendation.type"], "reduce_temp_tables")
            where attributes["tmp_disk_tables_created"] > 0
          
          - set(attributes["recommendation.text"],
                "Query creates disk-based temporary tables. Consider increasing tmp_table_size or optimizing GROUP BY/ORDER BY.")
            where attributes["recommendation.type"] == "reduce_temp_tables"
          
          # Set priority
          - set(attributes["recommendation.priority"], 
                Case(attributes["severity"],
                     "critical", 1,
                     "high", 2,
                     "medium", 3,
                     4))
```

## Step 4: Standardize Metrics (1 hour)

### 4.1 Implement Metric Transform

```yaml
processors:
  metrictransform/standardize:
    transforms:
      # Standardize metric names to: mysql.<object>.<measurement>.<unit>
      - include: mysql.query.exec_total
        new_name: mysql.query.executions.total
        action: update
      
      - include: mysql.query.latency_ms
        new_name: mysql.query.latency.milliseconds
        action: update
      
      - include: mysql.query.latency_avg_ms
        new_name: mysql.query.latency_average.milliseconds
        action: update
      
      - include: mysql.query.rows_examined_total
        new_name: mysql.query.rows_examined.total
        action: update
      
      - include: mysql.table.io.read.latency_ms
        new_name: mysql.table.io_latency_read.milliseconds
        action: update
      
      # Add units to metrics missing them
      - include: mysql.index.cardinality
        new_name: mysql.index.cardinality.count
        action: update
```

### 4.2 Fix Entity Synthesis

```yaml
processors:
  attributes/entity_synthesis:
    actions:
      - key: entity.type
        value: "MYSQL_QUERY_INTELLIGENCE"
        action: insert
      
      # Extract host and port from instance label
      - key: db.host
        from_attribute: instance
        action: extract
        pattern: '^([^:]+):'
      
      - key: db.port  
        from_attribute: instance
        action: extract
        pattern: ':(\d+)'
      
      # Create dynamic GUID
      - key: entity.guid
        value: Concat(["MYSQL_INTEL|",
                       attributes["cluster.name"], "|",
                       attributes["db.host"], ":",
                       attributes["db.port"]], "")
        action: insert
      
      - key: entity.name
        value: Concat([attributes["db.host"], "-sql-intelligence"], "")
        action: insert
```

## Step 5: Implement Priority Routing (30 minutes)

### 5.1 Add Routing Processor

```yaml
processors:
  routing/priority:
    from_attribute: "severity"
    table:
      - value: "critical"
        exporters: [otlphttp/newrelic_critical, file/alerts, debug]
      
      - value: "high"
        exporters: [otlphttp/newrelic_critical, prometheus]
      
      - value: "medium"
        exporters: [otlphttp/newrelic_standard, prometheus]
    
    default_exporters: [otlphttp/newrelic_standard, prometheus]

exporters:
  # High-priority exporter for critical queries
  otlphttp/newrelic_critical:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
      X-Priority: high
    compression: none  # No compression for speed
    timeout: 10s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 10s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
  
  # Alert file for critical issues
  file/alerts:
    path: /tmp/sql-intelligence/critical-queries.json
    rotation:
      max_megabytes: 10
      max_days: 3
      max_backups: 5
    format: json
```

## Step 6: Update Docker Configuration (15 minutes)

### 6.1 Simplify docker-compose.yaml

```yaml
services:
  sql-intelligence:
    build: .
    environment:
      # ... existing env vars ...
      # Remove COLLECTOR_CONFIG variable
    volumes:
      - ./config:/etc/otel:ro
      - ./logs:/tmp/logs
      - ./scripts/init.sql:/docker-entrypoint-initdb.d/init.sql:ro  # Uncomment!
    command: ["--config", "/etc/otel/collector.yaml"]  # Always use collector.yaml
```

### 6.2 Fix init.sql

Ensure performance_schema is properly configured:

```sql
-- Enable performance_schema if not already enabled
SET GLOBAL performance_schema = ON;

-- Ensure statement events are collected
UPDATE performance_schema.setup_instruments 
SET ENABLED = 'YES', TIMED = 'YES' 
WHERE NAME LIKE 'statement/%';

-- Enable events_statements_summary tables
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME LIKE 'events_statements_%';

-- Increase history length for better analysis
SET GLOBAL performance_schema_events_statements_history_size = 1000;
SET GLOBAL performance_schema_events_statements_history_long_size = 1000;
```

## Step 7: Enhance Testing (45 minutes)

### 7.1 Create Load Generation Script

```bash
#!/bin/bash
# scripts/generate-test-load.sh

echo "Generating diverse query patterns for testing..."

# Create test schema
mysql -h localhost -P 3306 -u root -ptest <<EOF
CREATE DATABASE IF NOT EXISTS test_intel;
USE test_intel;

-- Table without indexes (except primary key)
CREATE TABLE IF NOT EXISTS no_index_table (
    id INT PRIMARY KEY AUTO_INCREMENT,
    data VARCHAR(255),
    status INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Table with indexes
CREATE TABLE IF NOT EXISTS indexed_table (
    id INT PRIMARY KEY AUTO_INCREMENT,
    data VARCHAR(255),
    status INT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_status (status),
    INDEX idx_created (created_at)
);

-- Populate tables
INSERT INTO no_index_table (data, status) 
SELECT CONCAT('data-', RAND()), FLOOR(RAND() * 10)
FROM information_schema.tables t1, information_schema.tables t2
LIMIT 10000;

INSERT INTO indexed_table (data, status)
SELECT data, status FROM no_index_table;

-- Generate queries that should trigger recommendations

-- 1. Query without index (should trigger add_index recommendation)
SELECT COUNT(*) FROM no_index_table WHERE data LIKE 'data-1%';
SELECT * FROM no_index_table WHERE status = 5 AND data LIKE '%test%';

-- 2. Inefficient join (should trigger optimize_query)
SELECT t1.*, t2.*
FROM no_index_table t1
CROSS JOIN no_index_table t2
WHERE t1.data = t2.data
LIMIT 10;

-- 3. Query creating temp tables
SELECT status, COUNT(*) as cnt, GROUP_CONCAT(data)
FROM no_index_table
GROUP BY status
ORDER BY cnt DESC;

-- 4. Slow query
SELECT SLEEP(2), COUNT(*) FROM no_index_table;

-- 5. Efficient query for comparison
SELECT * FROM indexed_table WHERE status = 5;

EOF

echo "Test load generation complete!"
```

### 7.2 Update Makefile Test Target

```makefile
test-integration: test-integration-cleanup
	@echo "=== SQL Intelligence Integration Test ==="
	
	# Start MySQL and wait for it
	docker-compose up -d mysql-test
	@echo "Waiting for MySQL to be ready..."
	@for i in {1..30}; do \
		docker-compose exec mysql-test mysqladmin ping -h localhost --silent && break || \
		(echo "Waiting for MySQL... $$i/30" && sleep 2); \
	done
	
	# Generate test load
	@echo "Generating test query patterns..."
	chmod +x scripts/generate-test-load.sh
	./scripts/generate-test-load.sh
	
	# Start sql-intelligence module
	docker-compose up -d sql-intelligence
	@echo "Waiting for sql-intelligence to collect metrics..."
	sleep 45
	
	# Validate metrics
	@echo "Validating intelligence metrics..."
	@METRICS=$$(curl -s localhost:8082/metrics); \
	echo "$$METRICS" | grep -q "mysql_query_executions_total" || \
		(echo "FAIL: Basic metrics not found" && exit 1); \
	echo "$$METRICS" | grep -q "index_efficiency_score" || \
		(echo "FAIL: Intelligence scores not found" && exit 1); \
	echo "$$METRICS" | grep -q "recommendation.type" || \
		(echo "FAIL: Recommendations not generated" && exit 1); \
	echo "$$METRICS" | grep -q "severity=\"critical\|high\"" || \
		(echo "FAIL: Severity classification not working" && exit 1)
	
	@echo "✓ Integration test passed!"

test-integration-cleanup:
	docker-compose down -v 2>/dev/null || true
```

## Step 8: Validation and Rollout (1 hour)

### 8.1 Run Validation Script

```bash
cd modules/sql-intelligence
./validate-changes.sh
```

### 8.2 Test in Development

```bash
# Start with test configuration
docker-compose up -d

# Generate load
./scripts/generate-test-load.sh

# Monitor logs
docker-compose logs -f sql-intelligence

# Check metrics
curl -s localhost:8082/metrics | grep -E "(intelligence|recommendation|severity)"
```

### 8.3 Verify New Relic Integration

1. Check entity creation in New Relic One
2. Verify metrics are flowing with proper names
3. Confirm recommendations appear as attributes
4. Validate critical queries are tagged appropriately

### 8.4 Performance Comparison

Compare before and after:
- Query execution count (should be halved)
- CPU usage of collector
- MySQL load from performance_schema queries
- Metric cardinality in New Relic

## Step 9: Documentation Updates (30 minutes)

### 9.1 Update Module README

Document:
- New intelligence features
- Metric naming conventions
- Recommendation types
- Configuration parameters

### 9.2 Create Dashboard Templates

Create New Relic dashboards for:
- Query Performance Overview
- Index Efficiency Analysis
- Query Recommendations
- Performance Trends

### 9.3 Migration Guide

For teams using old metric names:
- Mapping of old to new metric names
- Dashboard query updates needed
- Alert condition modifications

## Rollback Plan

If issues occur:

1. **Quick Rollback**:
   ```bash
   cd config
   cp archive/$(date +%Y%m%d)/collector.yaml ./
   docker-compose restart sql-intelligence
   ```

2. **Feature Flags**:
   Add environment variables to disable new features:
   ```yaml
   ENABLE_INTELLIGENCE: false
   ENABLE_RECOMMENDATIONS: false
   ```

3. **Gradual Rollout**:
   - Start with one database instance
   - Monitor for 24 hours
   - Expand to additional instances

## Success Criteria

- [ ] Single configuration file (collector.yaml)
- [ ] Queries execute only once per interval
- [ ] All slow queries have intelligence scores
- [ ] Recommendations generated for problematic queries
- [ ] Metrics follow standardized naming
- [ ] Entity GUIDs unique per instance
- [ ] Critical queries routed to priority pipeline
- [ ] Integration tests pass consistently
- [ ] New Relic dashboards show intelligence data
- [ ] Performance impact < 10% vs previous version

## Ongoing Maintenance

1. **Weekly Reviews**:
   - Check recommendation accuracy
   - Tune thresholds based on feedback
   - Update severity classifications

2. **Monthly Updates**:
   - Add new recommendation types
   - Enhance intelligence algorithms
   - Optimize query performance

3. **Quarterly Planning**:
   - Review architecture decisions
   - Plan new intelligence features
   - Update documentation