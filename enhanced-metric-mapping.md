# Enhanced OHI Metric Mapping with Reference Architecture

## Overview

This document details the enhanced metric mapping that maintains 100% backward compatibility with OHI while adding the comprehensive metrics from the reference architecture.

## 1. Enhanced Metric Structure

### 1.1 SlowQueryMetrics Evolution

```rust
// Enhanced slow query metrics maintaining OHI compatibility
#[derive(Serialize, Deserialize, Clone)]
pub struct EnhancedSlowQueryMetric {
    // === OHI Compatible Fields (MUST maintain exact names) ===
    pub newrelic: Option<String>,
    pub query_id: Option<String>,
    pub query_text: Option<String>,
    pub database_name: Option<String>,
    pub schema_name: Option<String>,
    pub execution_count: Option<i64>,
    pub avg_elapsed_time_ms: Option<f64>,
    pub avg_disk_reads: Option<f64>,
    pub avg_disk_writes: Option<f64>,
    pub statement_type: Option<String>,
    pub collection_timestamp: Option<String>,
    pub individual_query: Option<String>,
    
    // === New Reference Architecture Fields ===
    #[serde(skip_serializing_if = "Option::is_none")]
    pub plan_id: Option<String>,
    
    #[serde(skip_serializing_if = "Option::is_none")]
    pub histogram_buckets: Option<Vec<HistogramBucket>>,
    
    #[serde(skip_serializing_if = "Option::is_none")]
    pub p95_latency_ms: Option<f64>,
    
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cpu_time_ms: Option<f64>,
    
    #[serde(skip_serializing_if = "Option::is_none")]
    pub io_time_ms: Option<f64>,
    
    #[serde(skip_serializing_if = "Option::is_none")]
    pub wait_event_summary: Option<WaitEventSummary>,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct HistogramBucket {
    pub le: f64,        // Less than or equal to
    pub count: i64,     // Number of observations
}

#[derive(Serialize, Deserialize, Clone)]
pub struct WaitEventSummary {
    pub total_wait_time_ms: f64,
    pub top_wait_events: Vec<WaitEventDetail>,
}

#[derive(Serialize, Deserialize, Clone)]
pub struct WaitEventDetail {
    pub wait_event_class: String,
    pub wait_event: String,
    pub count: i64,
    pub total_time_ms: f64,
}
```

### 1.2 New Execution Plan Metrics

```rust
// Comprehensive execution plan tracking
#[derive(Serialize, Deserialize, Clone)]
pub struct ExecutionPlanMetric {
    // Base fields
    pub query_id: String,
    pub plan_id: String,
    pub plan_hash: String,
    pub database_name: String,
    
    // Plan details
    pub plan_json: serde_json::Value,
    pub total_cost: f64,
    pub first_seen: String,
    pub last_seen: String,
    pub execution_count: i64,
    
    // Performance tracking
    pub avg_duration_ms: f64,
    pub min_duration_ms: f64,
    pub max_duration_ms: f64,
    pub stddev_duration_ms: f64,
    
    // Plan regression detection
    pub is_regression: bool,
    pub previous_plan_id: Option<String>,
    pub performance_change_pct: Option<f64>,
}
```

### 1.3 Active Session History (ASH) Metrics

```rust
#[derive(Serialize, Deserialize, Clone)]
pub struct ASHSample {
    pub sample_time: String,
    pub pid: i32,
    pub query_id: Option<i64>,
    pub database_name: String,
    pub username: String,
    pub application_name: String,
    
    // Session state
    pub state: String,
    pub wait_event_type: Option<String>,
    pub wait_event: Option<String>,
    pub backend_type: String,
    
    // CPU state (from eBPF)
    pub on_cpu: bool,
    pub cpu_id: Option<i32>,
    
    // Query context
    pub query_start: Option<String>,
    pub state_change: Option<String>,
    pub blocking_pids: Vec<i32>,
    
    // Resource usage
    pub memory_mb: Option<f64>,
    pub temp_files_mb: Option<f64>,
}
```

### 1.4 Kernel-Level Metrics (eBPF)

```rust
#[derive(Serialize, Deserialize, Clone)]
pub struct KernelQueryMetrics {
    pub query_id: String,
    pub pid: i32,
    pub timestamp: String,
    
    // CPU metrics
    pub cpu_user_ns: i64,
    pub cpu_system_ns: i64,
    pub cpu_delay_ns: i64,
    pub voluntary_ctxt_switches: i64,
    pub involuntary_ctxt_switches: i64,
    
    // I/O metrics
    pub io_wait_ns: i64,
    pub block_io_delay_ns: i64,
    pub read_syscalls: i64,
    pub write_syscalls: i64,
    pub read_bytes: i64,
    pub write_bytes: i64,
    
    // Memory metrics
    pub page_faults: i64,
    pub major_faults: i64,
    pub rss_bytes: i64,
}
```

## 2. Enhanced Query Implementations

### 2.1 Slow Queries with Extended Metrics

```sql
-- Enhanced slow query for PostgreSQL 13+ with reference architecture metrics
WITH query_stats AS (
    SELECT 
        'newrelic' as newrelic,
        pss.queryid AS query_id,
        LEFT(pss.query, 4095) AS query_text,
        pd.datname AS database_name,
        current_schema() AS schema_name,
        pss.calls AS execution_count,
        ROUND((pss.total_exec_time / pss.calls)::numeric, 3) AS avg_elapsed_time_ms,
        pss.shared_blks_read / pss.calls AS avg_disk_reads,
        pss.shared_blks_written / pss.calls AS avg_disk_writes,
        CASE
            WHEN pss.query ILIKE 'SELECT%%' THEN 'SELECT'
            WHEN pss.query ILIKE 'INSERT%%' THEN 'INSERT'
            WHEN pss.query ILIKE 'UPDATE%%' THEN 'UPDATE'
            WHEN pss.query ILIKE 'DELETE%%' THEN 'DELETE'
            ELSE 'OTHER'
        END AS statement_type,
        to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp,
        
        -- New metrics from reference architecture
        pss.mean_exec_time AS mean_time_ms,
        pss.min_exec_time AS min_time_ms,
        pss.max_exec_time AS max_time_ms,
        pss.stddev_exec_time AS stddev_time_ms,
        
        -- For histogram calculation
        pss.total_exec_time,
        pss.calls as total_calls,
        
        -- Buffer metrics
        pss.shared_blks_hit,
        pss.shared_blks_read AS shared_reads,
        pss.shared_blks_dirtied,
        pss.shared_blks_written,
        pss.temp_blks_read,
        pss.temp_blks_written,
        
        -- I/O timing if available
        pss.blk_read_time,
        pss.blk_write_time
    FROM pg_stat_statements pss
    JOIN pg_database pd ON pss.dbid = pd.oid
    WHERE pd.datname = current_database()
        AND pss.query NOT ILIKE 'EXPLAIN%%'
        AND pss.query NOT ILIKE '%%pg_stat_statements%%'
),
wait_events AS (
    -- Join with pg_wait_sampling if available
    SELECT 
        queryid,
        event_type,
        event,
        COUNT(*) as wait_count,
        SUM(count) as total_waits
    FROM pg_wait_sampling_profile
    WHERE queryid IS NOT NULL
    GROUP BY queryid, event_type, event
),
plan_info AS (
    -- Get plan information from pg_querylens extension
    SELECT 
        query_id,
        plan_id,
        plan_hash
    FROM pg_querylens.current_plans
)
SELECT 
    qs.*,
    
    -- Add plan information
    pi.plan_id,
    pi.plan_hash,
    
    -- Calculate percentiles (p95)
    CASE 
        WHEN qs.stddev_time_ms > 0 
        THEN qs.mean_time_ms + (1.645 * qs.stddev_time_ms)
        ELSE qs.max_time_ms
    END AS p95_latency_ms,
    
    -- CPU/IO split (from extension or estimation)
    CASE 
        WHEN qs.blk_read_time > 0 OR qs.blk_write_time > 0
        THEN qs.avg_elapsed_time_ms - (qs.blk_read_time + qs.blk_write_time) / qs.total_calls
        ELSE qs.avg_elapsed_time_ms * 0.7 -- Estimate 70% CPU
    END AS cpu_time_ms,
    
    CASE
        WHEN qs.blk_read_time > 0 OR qs.blk_write_time > 0
        THEN (qs.blk_read_time + qs.blk_write_time) / qs.total_calls
        ELSE qs.avg_elapsed_time_ms * 0.3 -- Estimate 30% I/O
    END AS io_time_ms,
    
    -- Wait event summary
    json_build_object(
        'total_wait_time_ms', COALESCE(SUM(we.total_waits), 0),
        'top_wait_events', array_agg(
            json_build_object(
                'wait_event_class', we.event_type,
                'wait_event', we.event,
                'count', we.wait_count,
                'total_time_ms', we.total_waits
            ) ORDER BY we.total_waits DESC
        ) FILTER (WHERE we.event IS NOT NULL)
    ) AS wait_event_summary
    
FROM query_stats qs
LEFT JOIN plan_info pi ON qs.query_id = pi.query_id::text
LEFT JOIN wait_events we ON qs.query_id = we.queryid
GROUP BY qs.*, pi.plan_id, pi.plan_hash
ORDER BY qs.avg_elapsed_time_ms DESC
LIMIT %d;
```

### 2.2 Active Session History Query

```sql
-- ASH sampling query with eBPF enrichment
WITH ash_sample AS (
    SELECT 
        clock_timestamp() as sample_time,
        a.pid,
        a.usename as username,
        a.datname as database_name,
        a.application_name,
        a.state,
        a.wait_event_type,
        a.wait_event,
        a.backend_type,
        a.query_start,
        a.state_change,
        a.query,
        
        -- Query ID from pg_stat_activity
        CASE 
            WHEN a.query_id IS NOT NULL THEN a.query_id
            WHEN s.queryid IS NOT NULL THEN s.queryid
            ELSE NULL
        END as query_id,
        
        -- Blocking information
        array_agg(blocked.pid) FILTER (WHERE blocked.pid IS NOT NULL) as blocking_pids,
        
        -- Resource usage
        (SELECT rss_bytes / 1024.0 / 1024.0 FROM pg_stat_process WHERE pid = a.pid) as memory_mb,
        (SELECT temp_bytes / 1024.0 / 1024.0 FROM pg_stat_process WHERE pid = a.pid) as temp_files_mb
        
    FROM pg_stat_activity a
    LEFT JOIN pg_stat_statements s ON a.query = s.query
    LEFT JOIN pg_locks blocked_lock ON blocked_lock.pid = a.pid AND NOT blocked_lock.granted
    LEFT JOIN pg_locks blocking_lock ON blocking_lock.transactionid = blocked_lock.transactionid 
        AND blocking_lock.granted
    LEFT JOIN pg_stat_activity blocked ON blocked.pid = blocking_lock.pid
    WHERE a.state != 'idle'
        AND a.pid != pg_backend_pid()
    GROUP BY a.pid, a.usename, a.datname, a.application_name, a.state, 
             a.wait_event_type, a.wait_event, a.backend_type, a.query_start,
             a.state_change, a.query, a.query_id, s.queryid
),
ebpf_data AS (
    -- Get CPU state from eBPF if available
    SELECT 
        pid,
        on_cpu,
        cpu_id,
        cpu_user_ns,
        cpu_system_ns
    FROM pg_querylens.ebpf_process_state
    WHERE sample_time >= NOW() - INTERVAL '2 seconds'
)
SELECT 
    ash.*,
    COALESCE(ebpf.on_cpu, false) as on_cpu,
    ebpf.cpu_id
FROM ash_sample ash
LEFT JOIN ebpf_data ebpf ON ash.pid = ebpf.pid;
```

### 2.3 Execution Plan Collection

```sql
-- Execution plan dictionary maintenance
WITH new_plans AS (
    -- Detect new plans from recent executions
    SELECT DISTINCT
        e.query_id,
        e.plan_id,
        e.plan_hash,
        e.database_name,
        MIN(e.timestamp) as first_seen,
        MAX(e.timestamp) as last_seen,
        COUNT(*) as execution_count,
        AVG(e.duration_ms) as avg_duration_ms,
        MIN(e.duration_ms) as min_duration_ms,
        MAX(e.duration_ms) as max_duration_ms,
        STDDEV(e.duration_ms) as stddev_duration_ms
    FROM pg_querylens.execution_metrics e
    LEFT JOIN pg_querylens.plan_dictionary p ON e.plan_id = p.plan_id
    WHERE p.plan_id IS NULL
        AND e.timestamp >= NOW() - INTERVAL '5 minutes'
    GROUP BY e.query_id, e.plan_id, e.plan_hash, e.database_name
),
plan_regressions AS (
    -- Detect plan changes and regressions
    SELECT 
        n.query_id,
        n.plan_id as new_plan_id,
        o.plan_id as old_plan_id,
        n.avg_duration_ms as new_avg_ms,
        o.avg_duration_ms as old_avg_ms,
        ((n.avg_duration_ms - o.avg_duration_ms) / o.avg_duration_ms * 100) as change_pct,
        CASE 
            WHEN n.avg_duration_ms > o.avg_duration_ms * 1.2 THEN true
            ELSE false
        END as is_regression
    FROM new_plans n
    JOIN LATERAL (
        SELECT plan_id, avg_duration_ms
        FROM pg_querylens.plan_dictionary
        WHERE query_id = n.query_id
            AND plan_id != n.plan_id
        ORDER BY last_seen DESC
        LIMIT 1
    ) o ON true
)
SELECT 
    np.*,
    pr.is_regression,
    pr.old_plan_id as previous_plan_id,
    pr.change_pct as performance_change_pct,
    
    -- Get actual plan JSON
    pg_querylens.get_plan_json(np.query_id, np.plan_id) as plan_json,
    
    -- Calculate total cost from plan
    (pg_querylens.get_plan_json(np.query_id, np.plan_id)->>'Total Cost')::float as total_cost
    
FROM new_plans np
LEFT JOIN plan_regressions pr ON np.query_id = pr.query_id AND np.plan_id = pr.new_plan_id;
```

## 3. Metric Export Mapping

### 3.1 NRI Format Extensions

```rust
impl NRIAdapter {
    fn adapt_enhanced_slow_query(&self, metric: &EnhancedSlowQueryMetric) -> MetricSet {
        let mut metric_set = MetricSet::new("PostgresSlowQueries");
        
        // All OHI fields first (maintain compatibility)
        if let Some(v) = &metric.newrelic {
            metric_set.add_attribute("newrelic", v);
        }
        // ... (all other OHI fields)
        
        // Add reference architecture fields
        if let Some(plan_id) = &metric.plan_id {
            metric_set.add_attribute("plan_id", plan_id);
        }
        
        if let Some(p95) = &metric.p95_latency_ms {
            metric_set.add_gauge("p95_latency_ms", *p95);
        }
        
        if let Some(cpu_time) = &metric.cpu_time_ms {
            metric_set.add_gauge("cpu_time_ms", *cpu_time);
        }
        
        if let Some(io_time) = &metric.io_time_ms {
            metric_set.add_gauge("io_time_ms", *io_time);
        }
        
        // Histogram as JSON attribute
        if let Some(buckets) = &metric.histogram_buckets {
            metric_set.add_attribute("latency_histogram", 
                serde_json::to_string(buckets).unwrap());
        }
        
        // Wait events as nested JSON
        if let Some(wait_summary) = &metric.wait_event_summary {
            metric_set.add_attribute("wait_events", 
                serde_json::to_string(wait_summary).unwrap());
        }
        
        metric_set
    }
    
    fn create_new_event_types(&self, metrics: &UnifiedMetrics) -> Vec<MetricSet> {
        let mut event_types = vec![];
        
        // New event type: PostgresExecutionPlans
        for plan in &metrics.execution_plans {
            let mut metric_set = MetricSet::new("PostgresExecutionPlans");
            metric_set.add_attribute("query_id", &plan.query_id);
            metric_set.add_attribute("plan_id", &plan.plan_id);
            metric_set.add_attribute("plan_hash", &plan.plan_hash);
            metric_set.add_attribute("database_name", &plan.database_name);
            metric_set.add_gauge("total_cost", plan.total_cost);
            metric_set.add_gauge("avg_duration_ms", plan.avg_duration_ms);
            metric_set.add_attribute("is_regression", plan.is_regression.to_string());
            metric_set.add_attribute("plan_json", plan.plan_json.to_string());
            event_types.push(metric_set);
        }
        
        // New event type: PostgresASH
        for sample in &metrics.active_session_history {
            let mut metric_set = MetricSet::new("PostgresASH");
            metric_set.add_attribute("sample_time", &sample.sample_time);
            metric_set.add_attribute("pid", sample.pid.to_string());
            metric_set.add_attribute("database_name", &sample.database_name);
            metric_set.add_attribute("state", &sample.state);
            if let Some(wait_event) = &sample.wait_event {
                metric_set.add_attribute("wait_event", wait_event);
            }
            metric_set.add_attribute("on_cpu", sample.on_cpu.to_string());
            event_types.push(metric_set);
        }
        
        // New event type: PostgresKernelMetrics
        for kernel in &metrics.kernel_metrics {
            let mut metric_set = MetricSet::new("PostgresKernelMetrics");
            metric_set.add_attribute("query_id", &kernel.query_id);
            metric_set.add_gauge("cpu_user_ns", kernel.cpu_user_ns as f64);
            metric_set.add_gauge("cpu_system_ns", kernel.cpu_system_ns as f64);
            metric_set.add_gauge("io_wait_ns", kernel.io_wait_ns as f64);
            metric_set.add_gauge("context_switches", 
                (kernel.voluntary_ctxt_switches + kernel.involuntary_ctxt_switches) as f64);
            event_types.push(metric_set);
        }
        
        event_types
    }
}
```

### 3.2 OTLP Semantic Conventions

```rust
impl OTLPAdapter {
    fn create_metrics(&self, metrics: &UnifiedMetrics) -> Vec<Metric> {
        let mut otlp_metrics = vec![];
        
        // Query duration histogram with buckets
        let duration_histogram = self.meter
            .f64_histogram("db.postgresql.query.duration")
            .with_unit("ms")
            .with_description("Query execution duration")
            .init();
            
        // CPU/IO breakdown gauges
        let cpu_time_gauge = self.meter
            .f64_gauge("db.postgresql.query.cpu_time")
            .with_unit("ms")
            .with_description("CPU time spent in query execution")
            .init();
            
        let io_time_gauge = self.meter
            .f64_gauge("db.postgresql.query.io_time") 
            .with_unit("ms")
            .with_description("I/O time spent in query execution")
            .init();
            
        // Wait event metrics
        let wait_event_counter = self.meter
            .u64_counter("db.postgresql.wait_events")
            .with_description("Count of wait events by type")
            .init();
            
        // ASH gauge
        let active_sessions_gauge = self.meter
            .i64_gauge("db.postgresql.active_sessions")
            .with_description("Active session count by state")
            .init();
            
        // Plan regression events
        let plan_regression_events = self.meter
            .u64_counter("db.postgresql.plan_regressions")
            .with_description("Count of detected plan regressions")
            .init();
            
        // Convert metrics
        for query in &metrics.slow_queries {
            let attrs = vec![
                KeyValue::new("db.system", "postgresql"),
                KeyValue::new("db.name", query.database_name.clone()),
                KeyValue::new("db.statement", query.query_text.clone()),
                KeyValue::new("db.operation.type", query.statement_type.clone()),
                KeyValue::new("db.postgresql.query_id", query.query_id.clone()),
            ];
            
            // Record histogram
            if let Some(buckets) = &query.histogram_buckets {
                for bucket in buckets {
                    duration_histogram.record(bucket.le, &attrs);
                }
            }
            
            // Record CPU/IO split
            if let Some(cpu_ms) = query.cpu_time_ms {
                cpu_time_gauge.record(cpu_ms, &attrs);
            }
            if let Some(io_ms) = query.io_time_ms {
                io_time_gauge.record(io_ms, &attrs);
            }
        }
        
        otlp_metrics
    }
}
```

## 4. Configuration for Enhanced Metrics

```yaml
# Enhanced metric collection configuration
metric_collection:
  # OHI compatibility mode
  ohi_compatibility:
    enabled: true
    maintain_field_names: true
    query_text_limit: 4095
    
  # Reference architecture features
  enhanced_features:
    histogram_buckets:
      enabled: true
      buckets: [0.1, 0.5, 1, 5, 10, 50, 100, 500, 1000, 5000]
      
    percentile_calculation:
      enabled: true
      percentiles: [50, 75, 90, 95, 99]
      
    cpu_io_split:
      enabled: true
      method: "blk_timing"  # or "ebpf" if available
      
    wait_event_tracking:
      enabled: true
      top_n: 10
      min_significance_ms: 1.0
      
    plan_tracking:
      enabled: true
      capture_json: true
      detect_regressions: true
      regression_threshold_pct: 20
      
    active_session_history:
      enabled: true
      sample_interval: "1s"
      retention_period: "1h"
      
    kernel_metrics:
      enabled: true
      require_cap_sys_admin: false  # Use eBPF if available
      
  # Export configuration
  export:
    nri:
      enabled: true
      include_extended_event_types: true
      event_types:
        - PostgresSlowQueries          # OHI compatible
        - PostgresWaitEvents           # OHI compatible
        - PostgresBlockingSessions     # OHI compatible
        - PostgresIndividualQueries    # OHI compatible
        - PostgresExecutionPlanMetrics # OHI compatible
        - PostgresExecutionPlans       # New
        - PostgresASH                  # New
        - PostgresKernelMetrics        # New
        
    otlp:
      enabled: true
      use_semantic_conventions: true
      metric_prefix: "db.postgresql"
```

## 5. Backward Compatibility Guarantees

```rust
// Ensure 100% OHI compatibility
#[cfg(test)]
mod compatibility_tests {
    use super::*;
    
    #[test]
    fn test_ohi_field_preservation() {
        let enhanced_metric = EnhancedSlowQueryMetric {
            // OHI fields
            newrelic: Some("newrelic".to_string()),
            query_id: Some("12345".to_string()),
            // ... all OHI fields
            
            // New fields
            plan_id: Some("plan_abc".to_string()),
            p95_latency_ms: Some(123.45),
            // ...
        };
        
        // When serialized for NRI with OHI mode
        let nri_output = NRIAdapter::new()
            .with_ohi_compatibility(true)
            .adapt(&enhanced_metric);
            
        // All OHI fields must be present
        assert!(nri_output.contains("query_id"));
        assert!(nri_output.contains("avg_elapsed_time_ms"));
        
        // New fields are optional
        if !ohi_compatibility_mode {
            assert!(nri_output.contains("plan_id"));
            assert!(nri_output.contains("p95_latency_ms"));
        }
    }
}
```

## Summary

The enhanced metric mapping provides:

1. **100% OHI Backward Compatibility**: All existing fields and event types preserved
2. **Histogram Support**: Latency distribution buckets for accurate percentiles
3. **CPU/IO Split**: Breakdown of time spent in CPU vs I/O operations
4. **Plan Tracking**: Automatic plan capture and regression detection
5. **Active Session History**: 1-second resolution session state sampling
6. **Kernel Metrics**: eBPF-based CPU delays and context switches
7. **Wait Event Details**: Comprehensive wait event tracking per query
8. **Flexible Export**: Works with both NRI and OTLP formats

This design ensures that existing OHI users can adopt the enhanced collector without any changes to their dashboards or alerts, while new users get access to the full spectrum of advanced metrics.