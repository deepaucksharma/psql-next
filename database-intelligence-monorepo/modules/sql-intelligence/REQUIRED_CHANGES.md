# SQL Intelligence Module - Required Changes

## Executive Summary

The sql-intelligence module suffers from severe configuration management issues, architectural flaws, and fails to deliver on its promise of providing actual "intelligence" about query performance. This document outlines the required changes to transform it from a simple data forwarder into a true query analysis engine.

## 1. Critical Issues to Fix

### 1.1 Configuration Sprawl and Duplication

**Current State:**
- 8+ different collector YAML files with duplicated content
- Multiple timestamped backups (collector.yaml.backup-*)
- No clear source of truth
- Bug fixes applied to one file miss others

**Required Changes:**
```bash
# Step 1: Backup current state
mkdir -p config/archive
mv config/*.backup-* config/archive/

# Step 2: Consolidate configurations
# Keep ONLY these files:
# - collector.yaml (main config with all intelligence features)
# - collector-test.yaml (for testing with mock data)
# Delete all others including:
# - collector-enhanced.yaml
# - collector-enterprise.yaml
# - collector-enterprise-working.yaml
# - collector-plan-intelligence.yaml (merge valuable logic into main)
```

**New collector.yaml Structure:**
```yaml
# Single source of truth combining best features from all variants
receivers:
  sqlquery/slow_queries:
    # Enhanced query analysis from collector-plan-intelligence.yaml
  sqlquery/table_stats:
    # Comprehensive table I/O metrics
  sqlquery/index_usage:
    # Advanced index efficiency analysis
  sqlquery/query_plans:
    # Query execution plan analysis (from plan-intelligence)
```

### 1.2 Broken Dual-Pipeline Architecture

**Current Flawed Implementation:**
```yaml
service:
  pipelines:
    metrics/standard:
      receivers: [sqlquery/slow_queries, sqlquery/table_stats, sqlquery/index_usage]
      # Executes all queries once
    metrics/critical:
      receivers: [sqlquery/slow_queries, sqlquery/table_stats, sqlquery/index_usage]
      # Executes THE SAME queries again!
```

**Required Fix:**
```yaml
service:
  pipelines:
    metrics:
      receivers: [sqlquery/slow_queries, sqlquery/table_stats, sqlquery/index_usage, sqlquery/query_plans]
      processors: [
        memory_limiter,
        batch,
        attributes,
        resource,
        transform/query_intelligence,    # Calculate scores and recommendations
        transform/impact_analysis,       # Determine severity
        attributes/newrelic,
        attributes/entity_synthesis,
        routing/priority                 # Route based on impact, not duplicate
      ]
      exporters: [otlphttp/newrelic, prometheus, debug]

processors:
  routing/priority:
    from_attribute: "query.impact_score"
    table:
      - value: "critical"
        exporters: [otlphttp/newrelic_critical, file/alerts]
      - value: "high"
        exporters: [otlphttp/newrelic_critical]
    default_exporters: [otlphttp/newrelic_standard, prometheus]
```

### 1.3 Missing Intelligence Features

**Current State:**
- Basic threshold checks (query > 1000ms = needs optimization)
- No index efficiency scoring
- No query cost analysis
- No actionable recommendations

**Required Intelligence Processors:**
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
            where attributes["exec_count"] > 0
          
          # Query Cost Score based on rows examined vs sent
          - set(attributes["query_cost_score"], 
                attributes["rows_examined_total"] / attributes["rows_sent_total"])
            where attributes["rows_sent_total"] > 0
          
          # Temp Table Impact
          - set(attributes["temp_table_penalty"], 
                attributes["tmp_disk_tables_created"] * 10 + attributes["tmp_tables_created"])
          
          # Overall Impact Score
          - set(attributes["query.impact_score"], 
                (attributes["total_latency_ms"] / 1000) * 
                (attributes["exec_count"] / 100) * 
                (attributes["query_cost_score"] / 100))

  transform/recommendations:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Generate actionable recommendations
          - set(attributes["recommendation.type"], "add_index")
            where attributes["no_index_used_count"] > 0
          
          - set(attributes["recommendation.text"], 
                Concat(["Consider adding index on table in query: ", 
                       attributes["DIGEST_TEXT"]], ""))
            where attributes["recommendation.type"] == "add_index"
          
          - set(attributes["recommendation.type"], "optimize_query")
            where attributes["query_cost_score"] > 1000
          
          - set(attributes["recommendation.priority"], "critical")
            where attributes["query.impact_score"] > 100
```

## 2. Major Issues to Address

### 2.1 Metric Naming Standardization

**Current Inconsistent Naming:**
- mysql.query.exec_total
- mysql.table.io.read.latency_ms
- mysql.index.cardinality

**Required Naming Convention:**
```yaml
processors:
  metrictransform:
    transforms:
      # Standardize all metrics to: mysql.<object>.<measurement>.<unit>
      - include: mysql.query.exec_total
        new_name: mysql.query.executions.total
      
      - include: mysql.query.latency_ms
        new_name: mysql.query.latency.milliseconds
      
      - include: mysql.table.io.read.latency_ms
        new_name: mysql.table.io_latency.read.milliseconds
      
      # Add units where missing
      - include: mysql.index.cardinality
        new_name: mysql.index.cardinality.count
```

### 2.2 Dynamic Entity Synthesis

**Current Broken Implementation:**
```yaml
attributes/entity_synthesis:
  actions:
    - key: entity.guid
      value: "MYSQL|${env:CLUSTER_NAME}|${env:MYSQL_ENDPOINT}"  # Static!
```

**Required Dynamic Implementation:**
```yaml
attributes/entity_synthesis:
  actions:
    - key: entity.type
      value: "MYSQL_QUERY_INTELLIGENCE"
      action: insert
    - key: entity.guid
      # Dynamic based on actual data source
      value: Concat(["MYSQL_INTEL|", 
                     attributes["cluster.name"], "|",
                     attributes["db.host"], ":",
                     attributes["db.port"]], "")
      action: insert
    - key: entity.name
      value: Concat([attributes["db.host"], "-query-intelligence"], "")
      action: insert
```

### 2.3 Query Optimization

**Current Arbitrary Limits:**
```sql
ORDER BY total_latency_ms DESC
LIMIT 20  -- Arbitrary, might miss critical queries
```

**Required Impact-Based Filtering:**
```sql
-- In sqlquery/slow_queries
WHERE SCHEMA_NAME IS NOT NULL
  AND DIGEST_TEXT NOT LIKE '%performance_schema%'
  AND COUNT_STAR > 0
  AND (
    SUM_TIMER_WAIT > 1000000000  -- Queries taking > 1 second total
    OR SUM_NO_INDEX_USED > 100   -- Frequently missing indexes
    OR AVG_ROWS_EXAMINED / GREATEST(AVG_ROWS_SENT, 1) > 100  -- Inefficient
  )
ORDER BY (SUM_TIMER_WAIT * COUNT_STAR) DESC  -- Impact = latency × frequency
LIMIT 50
```

## 3. Implementation Plan

### Phase 1: Configuration Cleanup (Immediate)
1. Archive all backup files
2. Create single authoritative collector.yaml
3. Update docker-compose.yaml to use only collector.yaml
4. Remove COLLECTOR_CONFIG environment variable options

### Phase 2: Architecture Fix (Day 1)
1. Implement single-pipeline architecture
2. Add routing processor for priority handling
3. Remove duplicate pipeline definitions
4. Test to ensure queries execute only once

### Phase 3: Intelligence Features (Day 2-3)
1. Port advanced analysis from collector-plan-intelligence.yaml
2. Implement query cost scoring
3. Add index efficiency calculations
4. Generate actionable recommendations
5. Create impact-based severity classification

### Phase 4: Standardization (Day 4)
1. Implement metrictransform for consistent naming
2. Fix entity synthesis for multi-instance support
3. Update documentation with new metric names
4. Create migration guide for dashboards

### Phase 5: Testing and Validation (Day 5)
1. Create load generation script for integration tests
2. Verify all intelligence features are working
3. Validate New Relic entity creation
4. Performance test single vs dual pipeline

## 4. Testing Requirements

### Integration Test Enhancement
```makefile
test-integration:
	@echo "Starting MySQL and generating test load..."
	docker-compose up -d mysql-test
	sleep 10
	
	# Generate queries with various patterns
	docker exec mysql-test mysql -u root -ptest -e "
	  CREATE TABLE IF NOT EXISTS test_table (id INT PRIMARY KEY, data VARCHAR(255));
	  -- Query without index (should trigger no_index_used)
	  SELECT * FROM test_table WHERE data = 'test';
	  -- Inefficient query
	  SELECT * FROM test_table t1 JOIN test_table t2 ON t1.data = t2.data;
	"
	
	# Start sql-intelligence
	docker-compose up -d sql-intelligence
	sleep 30
	
	# Verify metrics are generated with intelligence
	@echo "Verifying intelligence metrics..."
	curl -s localhost:8082/metrics | grep -E "(index_efficiency_score|query_cost_score|recommendation)" || \
	  (echo "FAIL: Intelligence metrics not found" && exit 1)
	
	@echo "✓ Integration test passed"
```

### Validation Checklist
- [ ] Only one collector.yaml exists in config/
- [ ] Queries execute only once per collection interval
- [ ] Intelligence scores are calculated for all queries
- [ ] Recommendations are generated for problematic queries
- [ ] Metrics follow standardized naming convention
- [ ] Entity GUIDs are unique per database instance
- [ ] High-impact queries are routed to critical exporters

## 5. Expected Outcomes

After implementing these changes:

1. **Configuration Management**: Single source of truth, no duplication
2. **Performance**: 50% reduction in database load (queries run once, not twice)
3. **Intelligence**: Automatic identification of problematic queries with actionable recommendations
4. **Observability**: Clear metrics with consistent naming and proper entity mapping
5. **Maintainability**: Simplified configuration that's easier to understand and modify

## 6. Rollback Plan

If issues arise:
1. The archived configurations in config/archive/ can be restored
2. Docker image tags should be used to version the changes
3. Feature flags can disable new processors while maintaining collection

## 7. Success Metrics

- Zero duplicate query executions
- 100% of slow queries have impact scores
- 100% of queries missing indexes have recommendations
- All metrics follow naming convention
- Entity creation works for multi-instance deployments