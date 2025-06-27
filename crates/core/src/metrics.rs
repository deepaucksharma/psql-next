use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Unified metric structure that encompasses all OHI metrics plus extensions
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct UnifiedMetrics {
    // Base metrics matching OHI exactly
    pub slow_queries: Vec<SlowQueryMetric>,
    pub wait_events: Vec<WaitEventMetric>,
    pub blocking_sessions: Vec<BlockingSessionMetric>,
    pub individual_queries: Vec<IndividualQueryMetric>,
    pub execution_plans: Vec<ExecutionPlanMetric>,
    
    // Enhanced metrics
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub per_execution_traces: Vec<ExecutionTrace>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub kernel_metrics: Vec<KernelMetric>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub active_session_history: Vec<ASHSample>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub plan_changes: Vec<PlanChangeEvent>,
    
    // PgBouncer metrics - using generic Value for now
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pgbouncer_metrics: Option<serde_json::Value>,
}

/// Exactly matches OHI SlowRunningQueryMetrics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SlowQueryMetric {
    pub newrelic: Option<String>,              // OHI compatibility
    pub query_id: Option<i64>,
    pub query_text: Option<String>,            // Anonymized, max 4095 chars
    pub database_name: Option<String>,
    pub schema_name: Option<String>,
    pub execution_count: Option<i64>,
    pub avg_elapsed_time_ms: Option<f64>,
    pub avg_disk_reads: Option<f64>,
    pub avg_disk_writes: Option<f64>,
    pub statement_type: Option<String>,
    pub collection_timestamp: Option<String>,
    pub individual_query: Option<String>,      // For RDS mode
    
    // Extended fields (nullable for OHI compatibility)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub extended_metrics: Option<ExtendedSlowQueryMetrics>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExtendedSlowQueryMetrics {
    pub percentile_latencies: LatencyPercentiles,
    pub cpu_time_breakdown: CpuTimeBreakdown,
    pub memory_stats: MemoryStatistics,
    pub cache_stats: CacheStatistics,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ebpf_metrics: Option<EbpfQueryMetrics>,
}

/// Exactly matches OHI WaitEventMetrics
#[derive(Debug, Clone, Serialize, Deserialize)]
#[derive(sqlx::FromRow)]
pub struct WaitEventMetric {
    pub pid: Option<i32>,
    pub wait_event_type: Option<String>,
    pub wait_event: Option<String>,
    pub wait_time_ms: Option<f64>,
    pub state: Option<String>,
    pub usename: Option<String>,
    pub database_name: Option<String>,
    pub query_id: Option<i64>,
    pub query_text: Option<String>,
    pub collection_timestamp: Option<String>,
}

/// Exactly matches OHI BlockingSessionMetrics
#[derive(Debug, Clone, Serialize, Deserialize)]
#[derive(sqlx::FromRow)]
pub struct BlockingSessionMetric {
    pub blocking_pid: Option<i32>,
    pub blocked_pid: Option<i32>,
    pub blocking_query: Option<String>,
    pub blocked_query: Option<String>,
    pub blocking_database: Option<String>,
    pub blocked_database: Option<String>,
    pub blocking_user: Option<String>,
    pub blocked_user: Option<String>,
    pub blocking_duration_ms: Option<f64>,
    pub blocked_duration_ms: Option<f64>,
    pub lock_type: Option<String>,
    pub collection_timestamp: Option<String>,
}

/// Exactly matches OHI IndividualQueryMetrics
#[derive(Debug, Clone, Serialize, Deserialize)]
#[derive(sqlx::FromRow)]
pub struct IndividualQueryMetric {
    pub pid: Option<i32>,
    pub query_id: Option<i64>,
    pub query_text: Option<String>,
    pub state: Option<String>,
    pub wait_event_type: Option<String>,
    pub wait_event: Option<String>,
    pub usename: Option<String>,
    pub database_name: Option<String>,
    pub backend_start: Option<String>,
    pub xact_start: Option<String>,
    pub query_start: Option<String>,
    pub state_change: Option<String>,
    pub backend_type: Option<String>,
    pub collection_timestamp: Option<String>,
}

/// Exactly matches OHI ExecutionPlanMetrics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutionPlanMetric {
    pub query_id: Option<i64>,
    pub query_text: Option<String>,
    pub database_name: Option<String>,
    pub plan: Option<serde_json::Value>,
    pub plan_text: Option<String>,
    pub total_cost: Option<f64>,
    pub execution_time_ms: Option<f64>,
    pub planning_time_ms: Option<f64>,
    pub collection_timestamp: Option<String>,
}

// Extended metric types
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutionTrace {
    pub query_id: i64,
    pub trace_id: String,
    pub span_id: String,
    pub parent_span_id: Option<String>,
    pub operation: String,
    pub duration_ms: f64,
    pub attributes: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KernelMetric {
    pub query_id: i64,
    pub cpu_time_ms: f64,
    pub io_wait_ms: f64,
    pub context_switches: u64,
    pub page_faults: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ASHSample {
    pub sample_time: chrono::DateTime<chrono::Utc>,
    pub pid: i32,
    pub usename: String,
    pub datname: String,
    pub query_id: Option<i64>,
    pub state: String,
    pub wait_event_type: Option<String>,
    pub wait_event: Option<String>,
    pub query: Option<String>,
    pub backend_type: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PlanChangeEvent {
    pub query_id: i64,
    pub old_plan_hash: String,
    pub new_plan_hash: String,
    pub change_timestamp: chrono::DateTime<chrono::Utc>,
    pub old_cost: f64,
    pub new_cost: f64,
    pub reason: String,
}

// Supporting structures
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LatencyPercentiles {
    pub p50: f64,
    pub p75: f64,
    pub p90: f64,
    pub p95: f64,
    pub p99: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CpuTimeBreakdown {
    pub parse_time_ms: f64,
    pub plan_time_ms: f64,
    pub execute_time_ms: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MemoryStatistics {
    pub peak_memory_kb: i64,
    pub avg_memory_kb: i64,
    pub temp_files_created: i64,
    pub temp_bytes_written: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheStatistics {
    pub hit_ratio: f64,
    pub blocks_hit: i64,
    pub blocks_read: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EbpfQueryMetrics {
    pub kernel_cpu_time_ms: f64,
    pub io_wait_time_ms: f64,
    pub scheduler_wait_time_ms: f64,
    pub syscall_count: u64,
    pub context_switches: u64,
}