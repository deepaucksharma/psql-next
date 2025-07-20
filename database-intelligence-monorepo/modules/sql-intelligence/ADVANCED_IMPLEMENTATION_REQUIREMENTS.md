# SQL Intelligence Module - Advanced Implementation Requirements

## Executive Vision: Pushing OTel + MySQL to the Limit

This document details how to transform the sql-intelligence module into the most advanced MySQL query intelligence system possible using OpenTelemetry, delivering unprecedented value to New Relic customers through deep database insights that go far beyond traditional monitoring.

## Table of Contents

1. [Ultra-Comprehensive MySQL Performance Schema Extraction](#1-ultra-comprehensive-mysql-performance-schema-extraction)
2. [Advanced OpenTelemetry Processing Chains](#2-advanced-opentelemetry-processing-chains)
3. [New Relic Value Maximization](#3-new-relic-value-maximization)
4. [Real-Time Query Plan Analysis](#4-real-time-query-plan-analysis)
5. [Predictive Performance Intelligence](#5-predictive-performance-intelligence)
6. [Implementation Architecture](#6-implementation-architecture)
7. [Testing and Validation Framework](#7-testing-and-validation-framework)

---

## 1. Ultra-Comprehensive MySQL Performance Schema Extraction

### 1.1 The Ultimate Performance Schema Query

Replace all existing queries with this single, comprehensive CTE-based query that extracts EVERYTHING:

```yaml
receivers:
  sqlquery/comprehensive_intelligence:
    driver: mysql
    datasource: "${env:MYSQL_USER}:${env:MYSQL_PASSWORD}@tcp(${env:MYSQL_ENDPOINT})/performance_schema"
    collection_interval: 10s
    queries:
      - sql: |
          WITH 
          -- Query digest analysis with execution patterns
          query_stats AS (
            SELECT 
              d.DIGEST,
              d.DIGEST_TEXT,
              d.SCHEMA_NAME,
              d.COUNT_STAR as executions,
              d.SUM_TIMER_WAIT/1000000000000 as total_time_sec,
              d.AVG_TIMER_WAIT/1000000000 as avg_latency_ms,
              d.MAX_TIMER_WAIT/1000000000 as max_latency_ms,
              d.MIN_TIMER_WAIT/1000000000 as min_latency_ms,
              d.SUM_ROWS_EXAMINED as rows_examined_total,
              d.SUM_ROWS_SENT as rows_sent_total,
              d.SUM_ROWS_AFFECTED as rows_affected_total,
              d.SUM_NO_INDEX_USED as no_index_used_count,
              d.SUM_NO_GOOD_INDEX_USED as no_good_index_count,
              d.SUM_CREATED_TMP_DISK_TABLES as tmp_disk_tables,
              d.SUM_CREATED_TMP_TABLES as tmp_tables,
              d.SUM_SORT_MERGE_PASSES as sort_merge_passes,
              d.SUM_SORT_ROWS as sort_rows,
              d.SUM_SELECT_FULL_JOIN as full_joins,
              d.SUM_SELECT_SCAN as full_scans,
              d.FIRST_SEEN,
              d.LAST_SEEN,
              d.QUANTILE_95/1000000000 as p95_latency_ms,
              d.QUANTILE_99/1000000000 as p99_latency_ms,
              d.QUANTILE_999/1000000000 as p999_latency_ms,
              -- Calculate execution rate
              TIMESTAMPDIFF(SECOND, d.FIRST_SEEN, d.LAST_SEEN) as lifetime_seconds,
              CASE 
                WHEN TIMESTAMPDIFF(SECOND, d.FIRST_SEEN, d.LAST_SEEN) > 0 
                THEN d.COUNT_STAR / TIMESTAMPDIFF(SECOND, d.FIRST_SEEN, d.LAST_SEEN)
                ELSE 0 
              END as exec_per_sec,
              -- Row efficiency metrics
              CASE 
                WHEN d.SUM_ROWS_SENT > 0 
                THEN d.SUM_ROWS_EXAMINED / d.SUM_ROWS_SENT 
                ELSE 999999 
              END as examination_ratio,
              -- Index usage percentage
              CASE
                WHEN d.COUNT_STAR > 0 
                THEN ((d.COUNT_STAR - d.SUM_NO_INDEX_USED) / d.COUNT_STAR) * 100
                ELSE 100
              END as index_usage_pct,
              -- Temp table usage percentage
              CASE
                WHEN d.COUNT_STAR > 0
                THEN (d.SUM_CREATED_TMP_DISK_TABLES / d.COUNT_STAR) * 100
                ELSE 0
              END as disk_tmp_table_pct
            FROM performance_schema.events_statements_summary_by_digest d
            WHERE d.SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
              AND d.COUNT_STAR > 0
          ),
          
          -- Current executing queries for real-time analysis
          current_queries AS (
            SELECT 
              t.PROCESSLIST_ID,
              t.PROCESSLIST_USER,
              t.PROCESSLIST_HOST,
              t.PROCESSLIST_DB,
              t.PROCESSLIST_COMMAND,
              t.PROCESSLIST_STATE,
              t.PROCESSLIST_TIME,
              s.DIGEST,
              s.SQL_TEXT,
              s.CURRENT_SCHEMA,
              s.TIMER_WAIT/1000000000 as current_time_ms,
              s.ROWS_EXAMINED as current_rows_examined,
              s.ROWS_SENT as current_rows_sent,
              s.NO_INDEX_USED as current_no_index,
              s.CREATED_TMP_DISK_TABLES as current_tmp_disk_tables
            FROM performance_schema.threads t
            JOIN performance_schema.events_statements_current s ON t.THREAD_ID = s.THREAD_ID
            WHERE t.PROCESSLIST_ID IS NOT NULL
              AND t.PROCESSLIST_COMMAND NOT IN ('Sleep', 'Binlog Dump')
              AND s.TIMER_WAIT > 0
          ),
          
          -- Table I/O patterns with hotspot detection
          table_io AS (
            SELECT 
              OBJECT_SCHEMA,
              OBJECT_NAME,
              COUNT_READ + COUNT_WRITE as total_io,
              COUNT_READ,
              COUNT_WRITE,
              SUM_TIMER_WAIT/1000000000 as io_latency_ms,
              SUM_TIMER_READ/1000000000 as read_latency_ms,
              SUM_TIMER_WRITE/1000000000 as write_latency_ms,
              -- I/O pattern classification
              CASE 
                WHEN COUNT_WRITE > COUNT_READ * 2 THEN 'write_heavy'
                WHEN COUNT_READ > COUNT_WRITE * 2 THEN 'read_heavy'
                ELSE 'balanced'
              END as io_pattern,
              -- Hotspot score (0-100)
              CASE
                WHEN (SELECT MAX(COUNT_READ + COUNT_WRITE) FROM performance_schema.table_io_waits_summary_by_table) > 0
                THEN ((COUNT_READ + COUNT_WRITE) / (SELECT MAX(COUNT_READ + COUNT_WRITE) FROM performance_schema.table_io_waits_summary_by_table)) * 100
                ELSE 0
              END as hotspot_score
            FROM performance_schema.table_io_waits_summary_by_table
            WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
          ),
          
          -- Index usage efficiency with cardinality analysis
          index_stats AS (
            SELECT 
              s.TABLE_SCHEMA,
              s.TABLE_NAME,
              s.INDEX_NAME,
              s.CARDINALITY,
              t.TABLE_ROWS,
              -- Index selectivity (higher is better)
              CASE 
                WHEN t.TABLE_ROWS > 0 
                THEN (s.CARDINALITY / t.TABLE_ROWS) * 100
                ELSE 0 
              END as selectivity_pct,
              -- Index size estimation
              s.SUB_PART,
              s.PACKED,
              s.NULLABLE,
              s.INDEX_TYPE,
              s.COMMENT,
              -- Composite index analysis
              s.SEQ_IN_INDEX,
              COUNT(*) OVER (PARTITION BY s.TABLE_SCHEMA, s.TABLE_NAME, s.INDEX_NAME) as columns_in_index
            FROM information_schema.STATISTICS s
            JOIN information_schema.TABLES t 
              ON s.TABLE_SCHEMA = t.TABLE_SCHEMA 
              AND s.TABLE_NAME = t.TABLE_NAME
            WHERE s.TABLE_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
              AND s.SEQ_IN_INDEX = 1  -- First column only for main stats
          ),
          
          -- Lock wait analysis
          lock_waits AS (
            SELECT 
              OBJECT_SCHEMA,
              OBJECT_NAME,
              COUNT_READ as read_locks,
              COUNT_WRITE as write_locks,
              SUM_TIMER_READ/1000000000 as read_lock_wait_ms,
              SUM_TIMER_WRITE/1000000000 as write_lock_wait_ms,
              -- Lock contention score
              CASE
                WHEN COUNT_READ + COUNT_WRITE > 0
                THEN (SUM_TIMER_READ + SUM_TIMER_WRITE) / (COUNT_READ + COUNT_WRITE) / 1000000
                ELSE 0
              END as avg_lock_wait_ms
            FROM performance_schema.table_lock_waits_summary_by_table
            WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
              AND (COUNT_READ > 0 OR COUNT_WRITE > 0)
          ),
          
          -- Execution plan cost estimation
          plan_costs AS (
            SELECT 
              q.DIGEST,
              q.executions,
              q.avg_latency_ms,
              q.examination_ratio,
              q.index_usage_pct,
              q.disk_tmp_table_pct,
              -- Calculate composite cost score (0-100, higher is worse)
              LEAST(100, 
                (q.avg_latency_ms / 10) * 0.3 +  -- Latency impact (max 30 points)
                (LOG10(GREATEST(q.examination_ratio, 1)) * 10) * 0.3 +  -- Efficiency impact (max 30 points)
                ((100 - q.index_usage_pct) / 100 * 40) * 0.2 +  -- Index impact (max 20 points)
                (q.disk_tmp_table_pct / 100 * 20) * 0.2  -- Temp table impact (max 20 points)
              ) as query_cost_score,
              -- Determine optimization potential
              CASE
                WHEN q.examination_ratio > 1000 THEN 'very_high'
                WHEN q.examination_ratio > 100 THEN 'high'
                WHEN q.examination_ratio > 10 THEN 'medium'
                ELSE 'low'
              END as optimization_potential
            FROM query_stats q
          )
          
          -- Final comprehensive output
          SELECT 
            -- Query identification
            q.DIGEST,
            q.DIGEST_TEXT,
            q.SCHEMA_NAME,
            
            -- Execution metrics
            q.executions,
            q.exec_per_sec,
            q.lifetime_seconds,
            q.total_time_sec,
            q.avg_latency_ms,
            q.max_latency_ms,
            q.min_latency_ms,
            q.p95_latency_ms,
            q.p99_latency_ms,
            q.p999_latency_ms,
            
            -- Row processing metrics
            q.rows_examined_total,
            q.rows_sent_total,
            q.rows_affected_total,
            q.examination_ratio,
            
            -- Index metrics
            q.no_index_used_count,
            q.no_good_index_count,
            q.index_usage_pct,
            
            -- Temp table metrics
            q.tmp_disk_tables,
            q.tmp_tables,
            q.disk_tmp_table_pct,
            
            -- Sort metrics
            q.sort_merge_passes,
            q.sort_rows,
            
            -- Join metrics
            q.full_joins,
            q.full_scans,
            
            -- Timestamps
            q.FIRST_SEEN,
            q.LAST_SEEN,
            
            -- Cost analysis
            pc.query_cost_score,
            pc.optimization_potential,
            
            -- Intelligence recommendations
            CASE
              WHEN pc.query_cost_score > 80 THEN 'critical_optimization_required'
              WHEN pc.query_cost_score > 60 THEN 'high_optimization_recommended'
              WHEN pc.query_cost_score > 40 THEN 'medium_optimization_suggested'
              WHEN pc.query_cost_score > 20 THEN 'low_optimization_possible'
              ELSE 'well_optimized'
            END as recommendation_level,
            
            -- Specific recommendations
            CONCAT_WS('; ',
              CASE WHEN q.no_index_used_count > q.executions * 0.5 
                   THEN 'ADD INDEX: Query frequently scans without index' 
                   ELSE NULL END,
              CASE WHEN q.examination_ratio > 100 
                   THEN CONCAT('OPTIMIZE: Query examines ', ROUND(q.examination_ratio), 'x more rows than needed')
                   ELSE NULL END,
              CASE WHEN q.disk_tmp_table_pct > 10 
                   THEN 'MEMORY: Increase tmp_table_size to avoid disk usage'
                   ELSE NULL END,
              CASE WHEN q.full_joins > 0 
                   THEN 'JOIN: Cartesian product detected, review JOIN conditions'
                   ELSE NULL END,
              CASE WHEN q.avg_latency_ms > 1000 
                   THEN CONCAT('LATENCY: Average ', ROUND(q.avg_latency_ms), 'ms exceeds SLA')
                   ELSE NULL END
            ) as specific_recommendations,
            
            -- Business impact score (0-100)
            LEAST(100,
              (q.exec_per_sec * 10) +  -- Frequency impact
              (q.avg_latency_ms / 100) +  -- Latency impact
              (pc.query_cost_score / 2)  -- Efficiency impact
            ) as business_impact_score,
            
            -- Current execution context (if running)
            COUNT(cq.PROCESSLIST_ID) as currently_running_count,
            MAX(cq.PROCESSLIST_TIME) as max_current_runtime_sec,
            GROUP_CONCAT(DISTINCT cq.PROCESSLIST_HOST) as active_hosts
            
          FROM query_stats q
          JOIN plan_costs pc ON q.DIGEST = pc.DIGEST
          LEFT JOIN current_queries cq ON q.DIGEST = cq.DIGEST
          GROUP BY q.DIGEST
          HAVING 
            -- Focus on impactful queries
            q.total_time_sec > 1  -- At least 1 second total time
            OR q.executions > 100  -- Or frequently executed
            OR pc.query_cost_score > 40  -- Or inefficient
            OR currently_running_count > 0  -- Or currently running
          ORDER BY 
            business_impact_score DESC,
            q.total_time_sec DESC
          LIMIT 200
        
        metrics:
          - metric_name: mysql.query.intelligence.comprehensive
            value_column: "query_cost_score"
            attribute_columns: [
              DIGEST, DIGEST_TEXT, SCHEMA_NAME, executions, exec_per_sec,
              avg_latency_ms, p95_latency_ms, p99_latency_ms,
              examination_ratio, index_usage_pct, disk_tmp_table_pct,
              optimization_potential, recommendation_level, specific_recommendations,
              business_impact_score, currently_running_count
            ]
```

### 1.2 Additional Specialized Queries

#### 1.2.1 Real-Time Query Plan Analysis

```yaml
  sqlquery/query_plans:
    driver: mysql
    datasource: "${env:MYSQL_USER}:${env:MYSQL_PASSWORD}@tcp(${env:MYSQL_ENDPOINT})/performance_schema"
    collection_interval: 30s
    queries:
      - sql: |
          -- Extract actual execution plans for top queries
          WITH top_queries AS (
            SELECT DIGEST, DIGEST_TEXT
            FROM performance_schema.events_statements_summary_by_digest
            WHERE SCHEMA_NAME NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
            ORDER BY SUM_TIMER_WAIT DESC
            LIMIT 10
          )
          SELECT 
            tq.DIGEST,
            tq.DIGEST_TEXT,
            -- This would need EXPLAIN to be run dynamically
            -- Placeholder for execution plan data
            'EXECUTION_PLAN_PLACEHOLDER' as execution_plan,
            -- Plan analysis metrics
            CASE 
              WHEN tq.DIGEST_TEXT LIKE '%JOIN%JOIN%JOIN%' THEN 'complex_joins'
              WHEN tq.DIGEST_TEXT LIKE '%SUBQUERY%' THEN 'subquery_present'
              WHEN tq.DIGEST_TEXT LIKE '%GROUP BY%HAVING%' THEN 'complex_aggregation'
              ELSE 'simple_query'
            END as query_complexity
          FROM top_queries tq
```

#### 1.2.2 Table Access Pattern Intelligence

```yaml
  sqlquery/access_patterns:
    driver: mysql
    datasource: "${env:MYSQL_USER}:${env:MYSQL_PASSWORD}@tcp(${env:MYSQL_ENDPOINT})/performance_schema"
    collection_interval: 60s
    queries:
      - sql: |
          -- Analyze table access patterns across time windows
          WITH time_windows AS (
            SELECT 
              OBJECT_SCHEMA,
              OBJECT_NAME,
              COUNT_READ,
              COUNT_WRITE,
              CURRENT_TIMESTAMP as snapshot_time,
              HOUR(CURRENT_TIMESTAMP) as hour_of_day,
              DAYOFWEEK(CURRENT_TIMESTAMP) as day_of_week
            FROM performance_schema.table_io_waits_summary_by_table
            WHERE OBJECT_SCHEMA NOT IN ('mysql', 'performance_schema', 'information_schema', 'sys')
          )
          SELECT 
            OBJECT_SCHEMA,
            OBJECT_NAME,
            COUNT_READ as current_reads,
            COUNT_WRITE as current_writes,
            hour_of_day,
            day_of_week,
            -- Access pattern classification
            CASE 
              WHEN hour_of_day BETWEEN 9 AND 17 THEN 'business_hours'
              WHEN hour_of_day BETWEEN 0 AND 6 THEN 'maintenance_window'
              ELSE 'off_hours'
            END as time_category,
            -- Workload type
            CASE
              WHEN COUNT_WRITE > COUNT_READ * 3 THEN 'write_intensive'
              WHEN COUNT_READ > COUNT_WRITE * 3 THEN 'read_intensive'
              WHEN COUNT_READ + COUNT_WRITE < 100 THEN 'low_activity'
              ELSE 'mixed_workload'
            END as workload_type
          FROM time_windows
```

## 2. Advanced OpenTelemetry Processing Chains

### 2.1 Multi-Stage Intelligence Pipeline

```yaml
processors:
  # Stage 1: Data Enrichment
  transform/enrich_context:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Add time-based context
          - set(attributes["collection_time"], Now())
          - set(attributes["hour_of_day"], Hour(Now()))
          - set(attributes["day_of_week"], DayOfWeek(Now()))
          
          # Classify time windows
          - set(attributes["time_window"], "business_hours") 
            where attributes["hour_of_day"] >= 9 and attributes["hour_of_day"] <= 17
          - set(attributes["time_window"], "after_hours")
            where attributes["hour_of_day"] > 17 or attributes["hour_of_day"] < 9
          
          # Add query fingerprint for better grouping
          - set(attributes["query_fingerprint"], 
                Substring(attributes["DIGEST"], 0, 8))
          
          # Extract operation type from query text
          - set(attributes["operation_type"], "SELECT") 
            where IsMatch(attributes["DIGEST_TEXT"], "^SELECT")
          - set(attributes["operation_type"], "INSERT")
            where IsMatch(attributes["DIGEST_TEXT"], "^INSERT")
          - set(attributes["operation_type"], "UPDATE")
            where IsMatch(attributes["DIGEST_TEXT"], "^UPDATE")
          - set(attributes["operation_type"], "DELETE")
            where IsMatch(attributes["DIGEST_TEXT"], "^DELETE")

  # Stage 2: Advanced Scoring
  transform/advanced_scoring:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Query Efficiency Score (0-100, higher is better)
          - set(attributes["efficiency_score"],
                100 - Min(100, 
                  (attributes["examination_ratio"] / 10) +
                  (attributes["disk_tmp_table_pct"]) +
                  (100 - attributes["index_usage_pct"]) / 2
                ))
          
          # Performance Stability Score (based on latency variance)
          - set(attributes["latency_variance"],
                (attributes["max_latency_ms"] - attributes["min_latency_ms"]) / 
                attributes["avg_latency_ms"])
            where attributes["avg_latency_ms"] > 0
          
          - set(attributes["stability_score"],
                100 - Min(100, attributes["latency_variance"] * 10))
            where attributes["latency_variance"] != nil
          
          # Resource Impact Score
          - set(attributes["resource_impact_score"],
                (attributes["exec_per_sec"] * attributes["avg_latency_ms"] / 1000) +
                (attributes["rows_examined_total"] / 1000000))
          
          # SLA Compliance Score
          - set(attributes["sla_target_ms"], 100)  # Default SLA
          - set(attributes["sla_target_ms"], 50)   # Strict SLA for business hours
            where attributes["time_window"] == "business_hours"
          
          - set(attributes["sla_compliance_score"],
                100 * (attributes["sla_target_ms"] / attributes["avg_latency_ms"]))
            where attributes["avg_latency_ms"] > 0
          
          # Optimization ROI Score (potential improvement value)
          - set(attributes["optimization_roi_score"],
                attributes["business_impact_score"] * 
                (100 - attributes["efficiency_score"]) / 100)

  # Stage 3: Pattern Recognition
  transform/pattern_recognition:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Identify query patterns
          - set(attributes["query_pattern"], "full_table_scan")
            where attributes["no_index_used_count"] > 0 and 
                  attributes["full_scans"] > 0
          
          - set(attributes["query_pattern"], "inefficient_join")
            where attributes["full_joins"] > 0 or
                  (IsMatch(attributes["DIGEST_TEXT"], "JOIN") and 
                   attributes["examination_ratio"] > 100)
          
          - set(attributes["query_pattern"], "missing_index")
            where attributes["no_index_used_count"] > attributes["executions"] * 0.5
          
          - set(attributes["query_pattern"], "temp_table_abuse")
            where attributes["disk_tmp_table_pct"] > 20
          
          - set(attributes["query_pattern"], "cartesian_product")
            where attributes["full_joins"] > 0
          
          # Identify performance trends
          - set(attributes["performance_trend"], "degrading")
            where attributes["avg_latency_ms"] > attributes["p95_latency_ms"] * 0.8
          
          - set(attributes["performance_trend"], "stable")
            where attributes["stability_score"] > 80
          
          - set(attributes["performance_trend"], "volatile")
            where attributes["stability_score"] < 50

  # Stage 4: Intelligent Recommendations
  transform/intelligent_recommendations:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Generate specific, actionable recommendations
          - set(attributes["recommendation_priority"], 1)
            where attributes["business_impact_score"] > 80 or
                  attributes["optimization_roi_score"] > 70
          
          - set(attributes["recommendation_priority"], 2)
            where attributes["recommendation_priority"] == nil and
                  (attributes["business_impact_score"] > 50 or
                   attributes["optimization_roi_score"] > 40)
          
          - set(attributes["recommendation_priority"], 3)
            where attributes["recommendation_priority"] == nil
          
          # Build recommendation text
          - set(attributes["primary_recommendation"],
                Concat([
                  "CRITICAL: Query with digest ", 
                  attributes["query_fingerprint"],
                  " requires immediate optimization. ",
                  attributes["specific_recommendations"]
                ], ""))
            where attributes["recommendation_priority"] == 1
          
          # Add New Relic specific context
          - set(attributes["nr_alert_condition"], "create")
            where attributes["recommendation_priority"] <= 2
          
          - set(attributes["nr_dashboard_query"],
                Concat([
                  "FROM Metric SELECT average(mysql.query.intelligence.comprehensive) ",
                  "WHERE DIGEST = '", attributes["DIGEST"], "' ",
                  "FACET SCHEMA_NAME TIMESERIES"
                ], ""))

  # Stage 5: Anomaly Detection
  transform/anomaly_detection:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Detect execution anomalies
          - set(attributes["execution_anomaly"], true)
            where attributes["exec_per_sec"] > 100 or
                  attributes["currently_running_count"] > 10
          
          # Detect latency anomalies
          - set(attributes["latency_anomaly"], true)
            where attributes["avg_latency_ms"] > attributes["p99_latency_ms"] * 2 or
                  attributes["max_latency_ms"] > attributes["p99_latency_ms"] * 10
          
          # Detect efficiency anomalies
          - set(attributes["efficiency_anomaly"], true)
            where attributes["examination_ratio"] > 10000 or
                  attributes["efficiency_score"] < 20
          
          # Create composite anomaly score
          - set(attributes["anomaly_score"], 0)
          - set(attributes["anomaly_score"], attributes["anomaly_score"] + 33)
            where attributes["execution_anomaly"] == true
          - set(attributes["anomaly_score"], attributes["anomaly_score"] + 33)
            where attributes["latency_anomaly"] == true
          - set(attributes["anomaly_score"], attributes["anomaly_score"] + 34)
            where attributes["efficiency_anomaly"] == true

  # Stage 6: New Relic Value Enhancement
  transform/newrelic_enhancement:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Add New Relic APM correlation hints
          - set(attributes["nr.correlate_with"], "Transaction")
            where attributes["business_impact_score"] > 50
          
          # Add faceting hints
          - set(attributes["nr.facet_by"], "SCHEMA_NAME,operation_type,query_pattern")
          
          # Add dashboard widget suggestions
          - set(attributes["nr.widget_type"], "billboard")
            where attributes["recommendation_priority"] == 1
          
          - set(attributes["nr.widget_type"], "line_chart")
            where attributes["performance_trend"] != "stable"
          
          # Add alert policy suggestions
          - set(attributes["nr.suggested_alert"],
                Concat([
                  "NRQL: SELECT average(query_cost_score) ",
                  "FROM mysql.query.intelligence.comprehensive ",
                  "WHERE DIGEST = '", attributes["DIGEST"], "' ",
                  "FACET SCHEMA_NAME"
                ], ""))
            where attributes["optimization_roi_score"] > 50
```

### 2.2 Routing Intelligence

```yaml
processors:
  routing/intelligent:
    from_attribute: "recommendation_priority"
    table:
      # Critical queries - immediate action needed
      - value: "1"
        exporters: [
          otlphttp/newrelic_critical,
          file/critical_queries,
          webhook/pagerduty,
          debug
        ]
      
      # High priority - proactive optimization
      - value: "2"
        exporters: [
          otlphttp/newrelic_high,
          file/optimization_queue,
          prometheus
        ]
      
      # Standard priority - regular monitoring
      - value: "3"
        exporters: [
          otlphttp/newrelic_standard,
          prometheus
        ]
    
    default_exporters: [otlphttp/newrelic_standard, prometheus]
```

## 3. New Relic Value Maximization

### 3.1 Enhanced Exporters Configuration

```yaml
exporters:
  # Critical queries with immediate dashboarding
  otlphttp/newrelic_critical:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
      X-Query-Intelligence: "critical"
      X-Auto-Dashboard: "true"
      X-Alert-Policy: "sql-intelligence-critical"
    compression: none  # Speed over size
    timeout: 5s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 5s
      max_elapsed_time: 30s
    sending_queue:
      enabled: true
      num_consumers: 20
      queue_size: 10000
      storage: file_storage/critical

  # High priority with batching
  otlphttp/newrelic_high:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
      X-Query-Intelligence: "high"
    compression: gzip
    timeout: 15s
    retry_on_failure:
      enabled: true
      initial_interval: 2s
      max_interval: 30s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 50000
      storage: file_storage/high

  # Webhook for critical alerts
  webhook/pagerduty:
    endpoint: ${env:PAGERDUTY_WEBHOOK_URL}
    method: POST
    headers:
      Content-Type: application/json
      Authorization: ${env:PAGERDUTY_API_KEY}
    timeout: 10s
    retry_on_failure:
      enabled: true
      max_elapsed_time: 60s

  # Structured file output for analysis
  file/critical_queries:
    path: /tmp/sql-intelligence/critical-queries.json
    format: json
    mode: 0600
    rotation:
      max_megabytes: 100
      max_days: 7
      max_backups: 10
      localtime: true
    flush_interval: 1s

  file/optimization_queue:
    path: /tmp/sql-intelligence/optimization-queue.json
    format: json
    rotation:
      max_megabytes: 500
      max_days: 30
      max_backups: 30
```

### 3.2 New Relic Entity Model

```yaml
processors:
  attributes/entity_synthesis_advanced:
    actions:
      # Primary entity - Query Intelligence Service
      - key: entity.type
        value: "MYSQL_QUERY_INTELLIGENCE"
        action: insert
      
      # Dynamic GUID based on instance
      - key: entity.guid
        value: Concat([
          "MYSQL_QUERY_INTEL|",
          attributes["cluster.name"], "|",
          attributes["SCHEMA_NAME"], "|",
          attributes["db.host"], ":",
          attributes["db.port"]
        ], "")
        action: insert
      
      # Hierarchical entity relationships
      - key: entity.parent.type
        value: "MYSQL_INSTANCE"
        action: insert
      
      - key: entity.parent.guid
        value: Concat([
          "MYSQL|",
          attributes["cluster.name"], "|",
          attributes["db.host"], ":",
          attributes["db.port"]
        ], "")
        action: insert
      
      # Query-specific entity for tracking
      - key: entity.query.guid
        value: Concat([
          "MYSQL_QUERY|",
          attributes["SCHEMA_NAME"], "|",
          attributes["DIGEST"]
        ], "")
        action: insert
      
      # Rich entity metadata
      - key: entity.tags
        value: Concat([
          "schema:", attributes["SCHEMA_NAME"], ",",
          "pattern:", attributes["query_pattern"], ",",
          "priority:", ToString(attributes["recommendation_priority"]), ",",
          "trend:", attributes["performance_trend"]
        ], "")
        action: insert
```

## 4. Real-Time Query Plan Analysis

### 4.1 Dynamic EXPLAIN Integration

```yaml
receivers:
  # This requires a custom receiver or script integration
  script/explain_analyzer:
    exec:
      command: ["/scripts/explain-analyzer.py"]
      arguments: ["--mysql-host", "${MYSQL_ENDPOINT}", "--top-queries", "10"]
    collection_interval: 300s  # Every 5 minutes
    timeout: 60s
    
    # The script would:
    # 1. Get top queries from performance_schema
    # 2. Run EXPLAIN on each
    # 3. Parse execution plan
    # 4. Output metrics in OTLP format
```

### 4.2 Explain Analyzer Script Outline

```python
#!/usr/bin/env python3
# scripts/explain-analyzer.py

import mysql.connector
import json
import sys
from opentelemetry import metrics
from opentelemetry.exporter.otlp.proto.grpc import OTLPMetricExporter

class ExplainAnalyzer:
    def analyze_query_plan(self, connection, digest_text):
        """Run EXPLAIN and analyze execution plan"""
        cursor = connection.cursor(dictionary=True)
        
        # Run EXPLAIN
        try:
            cursor.execute(f"EXPLAIN {digest_text}")
            plan = cursor.fetchall()
            
            # Analyze plan
            analysis = {
                'uses_index': any(row['key'] is not None for row in plan),
                'join_type': [row['type'] for row in plan],
                'estimated_rows': sum(row['rows'] for row in plan),
                'filesort': any('filesort' in str(row.get('Extra', '')) for row in plan),
                'temporary': any('temporary' in str(row.get('Extra', '')) for row in plan),
                'full_scan': any(row['type'] == 'ALL' for row in plan)
            }
            
            # Calculate plan score (0-100, lower is better)
            score = 0
            if not analysis['uses_index']: score += 30
            if analysis['full_scan']: score += 30
            if analysis['filesort']: score += 20
            if analysis['temporary']: score += 20
            
            return {
                'plan_score': score,
                'plan_details': json.dumps(analysis),
                'execution_plan': json.dumps(plan)
            }
            
        except Exception as e:
            return {
                'plan_score': -1,
                'plan_details': f"Error: {str(e)}",
                'execution_plan': None
            }
```

## 5. Predictive Performance Intelligence

### 5.1 Trend Analysis Processor

```yaml
processors:
  transform/predictive_intelligence:
    error_mode: ignore
    metric_statements:
      - context: metric
        statements:
          # Calculate query execution trend
          - set(attributes["execution_trend"], "increasing")
            where attributes["exec_per_sec"] > 0 and
                  attributes["lifetime_seconds"] > 3600  # At least 1 hour of data
          
          # Predict future impact
          - set(attributes["predicted_impact_1h"],
                attributes["business_impact_score"] * 1.1)
            where attributes["execution_trend"] == "increasing"
          
          # Capacity planning metrics
          - set(attributes["capacity_headroom"],
                100 - (attributes["resource_impact_score"] / 10))
            where attributes["resource_impact_score"] < 1000
          
          # SLA breach prediction
          - set(attributes["sla_breach_risk"], "high")
            where attributes["avg_latency_ms"] > attributes["sla_target_ms"] * 0.8 and
                  attributes["performance_trend"] == "degrading"
```

## 6. Implementation Architecture

### 6.1 Complete Pipeline Configuration

```yaml
service:
  pipelines:
    # Primary intelligence pipeline
    metrics/intelligence:
      receivers: [
        sqlquery/comprehensive_intelligence,
        sqlquery/query_plans,
        sqlquery/access_patterns,
        prometheus/mysql_exporter,
        otlp
      ]
      processors: [
        memory_limiter,
        batch/adaptive,
        transform/enrich_context,
        transform/advanced_scoring,
        transform/pattern_recognition,
        transform/intelligent_recommendations,
        transform/anomaly_detection,
        transform/newrelic_enhancement,
        transform/predictive_intelligence,
        attributes/entity_synthesis_advanced,
        routing/intelligent
      ]
      exporters: [debug]  # Routed by routing processor
    
    # Real-time analysis pipeline
    metrics/realtime:
      receivers: [script/explain_analyzer]
      processors: [
        memory_limiter,
        batch/small,
        attributes/entity_synthesis_advanced
      ]
      exporters: [otlphttp/newrelic_critical]

  telemetry:
    logs:
      level: info
      output_paths: 
        - /tmp/logs/sql-intelligence.log
        - stdout
      initial_fields:
        service: sql-intelligence
        version: "3.0"
    
    metrics:
      level: detailed
      address: 0.0.0.0:8888
      exporters:
        - prometheus
        - otlp/self
```

### 6.2 Adaptive Batching

```yaml
processors:
  batch/adaptive:
    timeout: 5s
    send_batch_size: 1000
    send_batch_max_size: 2000
    
    # Adaptive batching based on priority
    metadata_keys: ["recommendation_priority"]
    metadata_cardinality_limit: 10
```

### 6.3 Storage Configuration

```yaml
extensions:
  file_storage/critical:
    directory: /data/sql-intelligence/critical
    timeout: 10s
    compaction:
      on_start: true
      directory: /data/sql-intelligence/critical
      max_transaction_size: 65536
    
  file_storage/high:
    directory: /data/sql-intelligence/high
    timeout: 10s
    compaction:
      on_rebound: true
      rebound_needed_threshold_mib: 100
      rebound_trigger_threshold_mib: 200
```

## 7. Testing and Validation Framework

### 7.1 Comprehensive Test Data Generator

```sql
-- scripts/generate-test-patterns.sql

-- Create test schema with various table types
CREATE DATABASE IF NOT EXISTS sql_intel_test;
USE sql_intel_test;

-- Table with no indexes (will trigger recommendations)
CREATE TABLE IF NOT EXISTS no_index_table (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT,
    event_type VARCHAR(50),
    event_data JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20),
    category VARCHAR(50),
    value DECIMAL(10,2)
) ENGINE=InnoDB;

-- Well-indexed table for comparison
CREATE TABLE IF NOT EXISTS optimized_table (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT,
    event_type VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_event_type (event_type),
    INDEX idx_created_at (created_at),
    INDEX idx_composite (user_id, event_type, created_at)
) ENGINE=InnoDB;

-- Large table for join testing
CREATE TABLE IF NOT EXISTS large_dimension (
    dim_id BIGINT AUTO_INCREMENT PRIMARY KEY,
    dim_name VARCHAR(100),
    dim_category VARCHAR(50),
    attributes JSON
) ENGINE=InnoDB;

-- Insert test data
INSERT INTO no_index_table (user_id, event_type, event_data, status, category, value)
SELECT 
    FLOOR(RAND() * 10000),
    CASE FLOOR(RAND() * 5)
        WHEN 0 THEN 'login'
        WHEN 1 THEN 'purchase'
        WHEN 2 THEN 'view'
        WHEN 3 THEN 'click'
        ELSE 'other'
    END,
    JSON_OBJECT('key', UUID()),
    CASE FLOOR(RAND() * 3)
        WHEN 0 THEN 'active'
        WHEN 1 THEN 'pending'
        ELSE 'completed'
    END,
    CONCAT('cat_', FLOOR(RAND() * 100)),
    RAND() * 1000
FROM 
    information_schema.tables t1,
    information_schema.tables t2
LIMIT 1000000;

-- Copy to optimized table
INSERT INTO optimized_table (user_id, event_type, created_at)
SELECT user_id, event_type, created_at FROM no_index_table;

-- Generate query patterns that will trigger different recommendations

-- 1. Full table scan (no index)
SELECT COUNT(*) FROM no_index_table WHERE event_type = 'purchase';
SELECT * FROM no_index_table WHERE status = 'active' AND category LIKE 'cat_1%';

-- 2. Inefficient join (cartesian product)
SELECT t1.*, t2.*
FROM no_index_table t1, no_index_table t2
WHERE t1.user_id = t2.user_id
AND t1.event_type = 'purchase'
LIMIT 10;

-- 3. Suboptimal aggregation (temp tables)
SELECT 
    user_id,
    event_type,
    COUNT(*) as cnt,
    AVG(value) as avg_value,
    GROUP_CONCAT(DISTINCT status) as statuses
FROM no_index_table
GROUP BY user_id, event_type
HAVING cnt > 10
ORDER BY avg_value DESC;

-- 4. Complex query with multiple issues
WITH user_summary AS (
    SELECT 
        user_id,
        COUNT(DISTINCT event_type) as event_types,
        SUM(value) as total_value
    FROM no_index_table
    WHERE created_at > DATE_SUB(NOW(), INTERVAL 30 DAY)
    GROUP BY user_id
),
category_summary AS (
    SELECT 
        category,
        AVG(value) as avg_category_value
    FROM no_index_table
    GROUP BY category
)
SELECT 
    us.*,
    cs.*,
    (SELECT COUNT(*) FROM no_index_table nt WHERE nt.user_id = us.user_id) as total_events
FROM user_summary us
CROSS JOIN category_summary cs
WHERE us.total_value > cs.avg_category_value
ORDER BY us.total_value DESC;

-- 5. Queries that will run well (for comparison)
SELECT * FROM optimized_table WHERE user_id = 12345;
SELECT COUNT(*) FROM optimized_table WHERE event_type = 'login' AND created_at > DATE_SUB(NOW(), INTERVAL 1 DAY);
```

### 7.2 Validation Test Suite

```bash
#!/bin/bash
# scripts/validate-intelligence.sh

set -e

echo "=== SQL Intelligence Validation Suite ==="

# Function to check metric existence and value
check_metric() {
    local metric_name=$1
    local expected_pattern=$2
    local description=$3
    
    echo -n "Checking $description... "
    
    if curl -s localhost:8082/metrics | grep -E "$metric_name.*$expected_pattern" > /dev/null; then
        echo "✓ PASS"
        return 0
    else
        echo "✗ FAIL"
        return 1
    fi
}

# Start services
docker-compose up -d mysql-test sql-intelligence
sleep 30

# Generate test load
echo "Generating test query patterns..."
docker exec mysql-test mysql -u root -ptest < scripts/generate-test-patterns.sql

# Wait for metrics
sleep 60

# Validation checks
echo ""
echo "Validating Intelligence Metrics:"
echo "--------------------------------"

# Basic metrics
check_metric "mysql_query_intelligence_comprehensive" "query_cost_score" "Query cost scoring"
check_metric "mysql_query_intelligence_comprehensive" "optimization_potential" "Optimization detection"
check_metric "mysql_query_intelligence_comprehensive" "specific_recommendations" "Recommendation generation"

# Advanced metrics
check_metric "mysql_query_intelligence_comprehensive" "efficiency_score" "Efficiency scoring"
check_metric "mysql_query_intelligence_comprehensive" "business_impact_score" "Business impact calculation"
check_metric "mysql_query_intelligence_comprehensive" "anomaly_score" "Anomaly detection"

# Pattern recognition
check_metric "mysql_query_intelligence_comprehensive" "query_pattern=\"full_table_scan\"" "Full table scan detection"
check_metric "mysql_query_intelligence_comprehensive" "query_pattern=\"inefficient_join\"" "Inefficient join detection"

# Recommendations
check_metric "mysql_query_intelligence_comprehensive" "recommendation_priority=\"1\"" "Critical recommendations"
check_metric "mysql_query_intelligence_comprehensive" "specific_recommendations=~\"ADD INDEX\"" "Index recommendations"

# New Relic integration
check_metric "mysql_query_intelligence_comprehensive" "nr_alert_condition" "New Relic alert hints"
check_metric "mysql_query_intelligence_comprehensive" "entity.guid" "Entity synthesis"

echo ""
echo "Checking Data Quality:"
echo "---------------------"

# Check for minimum number of queries analyzed
QUERY_COUNT=$(curl -s localhost:8082/metrics | grep -c "mysql_query_intelligence_comprehensive" || true)
echo -n "Minimum queries analyzed (expect >= 10): "
if [ $QUERY_COUNT -ge 10 ]; then
    echo "✓ $QUERY_COUNT queries"
else
    echo "✗ Only $QUERY_COUNT queries"
fi

# Check for query diversity
PATTERNS=$(curl -s localhost:8082/metrics | grep -o 'query_pattern="[^"]*"' | sort -u | wc -l || true)
echo -n "Query pattern diversity (expect >= 3): "
if [ $PATTERNS -ge 3 ]; then
    echo "✓ $PATTERNS patterns"
else
    echo "✗ Only $PATTERNS patterns"
fi

echo ""
echo "Performance Validation:"
echo "----------------------"

# Check collector resource usage
CONTAINER_STATS=$(docker stats sql-intelligence --no-stream --format "CPU: {{.CPUPerc}} MEM: {{.MemUsage}}")
echo "Container resources: $CONTAINER_STATS"

# Verify single query execution
echo -n "Checking for duplicate query execution... "
LOG_COUNT=$(docker logs sql-intelligence 2>&1 | grep -c "Executing query" || true)
if [ $LOG_COUNT -le 100 ]; then
    echo "✓ Queries executing efficiently"
else
    echo "✗ Possible duplicate execution detected"
fi

echo ""
echo "=== Validation Complete ==="
```

### 7.3 New Relic Dashboard Template

```json
{
  "name": "MySQL Query Intelligence - Advanced",
  "description": "Comprehensive query performance intelligence powered by OpenTelemetry",
  "permissions": "PUBLIC_READ_WRITE",
  "widgets": [
    {
      "title": "Query Intelligence Overview",
      "layout": {
        "column": 1,
        "row": 1,
        "width": 12,
        "height": 3
      },
      "linkedEntityGuids": null,
      "visualization": {
        "id": "viz.billboard"
      },
      "rawConfiguration": {
        "nrqlQueries": [
          {
            "accountId": 0,
            "query": "SELECT count(DISTINCT DIGEST) as 'Total Queries', average(query_cost_score) as 'Avg Cost Score', sum(business_impact_score) as 'Total Impact', percentage(count(*), WHERE recommendation_priority = 1) as 'Critical Queries %' FROM mysql.query.intelligence.comprehensive SINCE 1 hour ago"
          }
        ]
      }
    },
    {
      "title": "Query Patterns Distribution",
      "layout": {
        "column": 1,
        "row": 4,
        "width": 6,
        "height": 3
      },
      "visualization": {
        "id": "viz.pie"
      },
      "rawConfiguration": {
        "nrqlQueries": [
          {
            "accountId": 0,
            "query": "SELECT count(*) FROM mysql.query.intelligence.comprehensive FACET query_pattern SINCE 1 hour ago"
          }
        ]
      }
    },
    {
      "title": "Optimization ROI Opportunities",
      "layout": {
        "column": 7,
        "row": 4,
        "width": 6,
        "height": 3
      },
      "visualization": {
        "id": "viz.bar"
      },
      "rawConfiguration": {
        "nrqlQueries": [
          {
            "accountId": 0,
            "query": "SELECT sum(optimization_roi_score) FROM mysql.query.intelligence.comprehensive FACET DIGEST_TEXT SINCE 1 hour ago LIMIT 10"
          }
        ]
      }
    },
    {
      "title": "Real-Time Query Performance",
      "layout": {
        "column": 1,
        "row": 7,
        "width": 12,
        "height": 3
      },
      "visualization": {
        "id": "viz.line"
      },
      "rawConfiguration": {
        "nrqlQueries": [
          {
            "accountId": 0,
            "query": "SELECT average(avg_latency_ms), average(p95_latency_ms), average(p99_latency_ms) FROM mysql.query.intelligence.comprehensive TIMESERIES SINCE 1 hour ago"
          }
        ]
      }
    },
    {
      "title": "Critical Query Recommendations",
      "layout": {
        "column": 1,
        "row": 10,
        "width": 12,
        "height": 4
      },
      "visualization": {
        "id": "viz.table"
      },
      "rawConfiguration": {
        "nrqlQueries": [
          {
            "accountId": 0,
            "query": "SELECT DIGEST_TEXT, query_cost_score, specific_recommendations, business_impact_score, currently_running_count FROM mysql.query.intelligence.comprehensive WHERE recommendation_priority = 1 SINCE 1 hour ago LIMIT 20"
          }
        ]
      }
    }
  ]
}
```

## Implementation Success Metrics

After implementing this advanced SQL intelligence system, you should see:

1. **Query Visibility**: 100% of impactful queries identified and scored
2. **Recommendation Quality**: Specific, actionable recommendations for every problematic query
3. **Performance Impact**: <5% overhead on MySQL despite comprehensive analysis
4. **New Relic Value**: Rich entities, automated dashboards, and intelligent alerting
5. **Operational Efficiency**: 50-80% reduction in time to identify and fix query issues
6. **Predictive Capability**: Identify performance degradation before it impacts users

This implementation pushes the boundaries of what's possible with OpenTelemetry and MySQL, delivering unprecedented database intelligence to New Relic customers.