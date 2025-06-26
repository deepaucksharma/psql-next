# PostgreSQL Unified Collector - Comprehensive Improvements

## Executive Summary

This document outlines comprehensive improvements to align the PostgreSQL Unified Collector with the reference-grade architecture, creating a single collector that:
- Supersets every metric from nri-postgresql QPM
- Adds plan/wait-state/OS-delay/ASH depth capabilities
- Ships as a reusable Rust crate with both OTLP and NRI export paths

## 1. Core Architecture Improvements

### 1.1 Modular Rust Crate Structure

```rust
// Actual crate structure in the implementation
// Workspace root: Cargo.toml

// Core crates
crate core          // Core types and traits (UnifiedMetrics, CollectorConfig)
crate nri_adapter   // New Relic Infrastructure JSON conversion
crate otel_adapter  // OpenTelemetry OTLP conversion
crate query_engine  // SQL query execution with version compatibility
crate extensions    // PostgreSQL extension detection and management
crate collectors    // Metric collection implementations
crate config        // TOML-based configuration management
```

### 1.2 Common Plan Model (CPM) Implementation

```rust
// crates/core/src/models.rs
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use serde_json::Value as JsonValue;

// UnifiedMetrics represents the canonical query execution model
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UnifiedMetrics {
    // Base metrics matching OHI exactly
    pub slow_queries: Vec<SlowQueryMetric>,
    pub wait_events: Vec<WaitEventMetric>,
    pub blocking_sessions: Vec<BlockingSessionMetric>,
    pub individual_queries: Vec<IndividualQueryMetric>,
    pub execution_plans: Vec<ExecutionPlanMetric>,
    
    // Enhanced metrics (optional)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub per_execution_traces: Option<Vec<ExecutionTrace>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kernel_metrics: Option<Vec<KernelMetric>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub active_session_history: Option<Vec<ASHSample>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub plan_changes: Option<Vec<PlanChangeEvent>>,
    
    // Metadata
    pub collection_metadata: CollectionMetadata,
}

// SlowQueryMetric - OHI compatible structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SlowQueryMetric {
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
    
    // Extended fields (optional)
    #[serde(skip_serializing_if = "Option::is_none")]
    pub extended_metrics: Option<ExtendedSlowQueryMetrics>,
}

// ExecutionPlanMetric captures plan details
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutionPlanMetric {
    pub query_id: String,
    pub plan_id: String,
    pub plan_hash: String,
    pub database_name: String,
    pub plan_json: JsonValue,
    pub total_cost: f64,
    pub first_seen: DateTime<Utc>,
    pub last_seen: DateTime<Utc>,
    pub execution_count: i64,
    pub avg_duration_ms: f64,
    pub is_regression: bool,
    pub previous_plan_id: Option<String>,
}

// ASHSample for Active Session History
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ASHSample {
    pub sample_time: DateTime<Utc>,
    pub pid: i32,
    pub query_id: Option<i64>,
    pub database_name: String,
    pub username: String,
    pub state: String,
    pub wait_event_type: Option<String>,
    pub wait_event: Option<String>,
    pub on_cpu: bool,
    pub cpu_id: Option<i32>,
}
```

### 1.3 Shared Memory Ring Buffer Integration

```rust
// crates/extensions/src/shm_reader.rs
use std::sync::atomic::{AtomicU64, Ordering};
use memmap2::{Mmap, MmapOptions};
use std::fs::File;
use std::path::Path;

// SHMReader reads metrics from pg_querylens extension
pub struct SHMReader {
    mmap: Mmap,
    ring: *const RingBuffer,
    decoder: ProtobufDecoder,
}

// RingBuffer represents the lock-free ring structure
#[repr(C)]
pub struct RingBuffer {
    head: AtomicU64,     // Atomic read position
    tail: AtomicU64,     // Atomic write position  
    size: u64,
    data: [u8; 0],       // Flexible array member
}

impl SHMReader {
    pub fn connect(shm_path: &Path) -> Result<Self, Box<dyn std::error::Error>> {
        // Open shared memory file
        let file = File::open(shm_path)?;
        
        // Memory map the file
        let mmap = unsafe { MmapOptions::new().map(&file)? };
        
        // Set up ring buffer pointer
        let ring = mmap.as_ptr() as *const RingBuffer;
        
        Ok(Self {
            mmap,
            ring,
            decoder: ProtobufDecoder::new(),
        })
    }
    
    pub fn read_metrics(&mut self) -> Result<Vec<QueryMetrics>, Box<dyn std::error::Error>> {
        let mut metrics = Vec::new();
        let ring = unsafe { &*self.ring };
        
        // Read from ring buffer using atomic operations
        loop {
            let head = ring.head.load(Ordering::Acquire);
            let tail = ring.tail.load(Ordering::Acquire);
            
            if head == tail {
                break; // No new data
            }
            
            // Read and decode protobuf message
            let data = unsafe {
                std::slice::from_raw_parts(
                    ring.data.as_ptr().add(head as usize),
                    (tail - head) as usize
                )
            };
            
            let metric = self.decoder.decode(data)?;
            metrics.push(metric);
            
            // Update read position
            ring.head.store(tail, Ordering::Release);
        }
        
        Ok(metrics)
    }
}
```

## 2. Enhanced Metric Collection

### 2.1 Live Plan Capture

```rust
// crates/collectors/src/plan_capture.rs
use sqlx::{PgPool, postgres::PgRow};
use std::time::Duration;
use tokio::time::timeout;
use serde_json::Value as JsonValue;
use chrono::Utc;

pub struct PlanCapture {
    pool: PgPool,
    cache: PlanCache,
    dict_table: String,
}

impl PlanCapture {
    pub async fn capture_plan_for_query(
        &self, 
        query_id: &str, 
        query_text: &str
    ) -> Result<ExecutionPlanMetric, Box<dyn std::error::Error>> {
        // Check cache first
        if let Some(cached) = self.cache.get(query_id).await {
            return Ok(cached);
        }
        
        // Execute EXPLAIN with timeout
        let explain_query = format!("EXPLAIN (FORMAT JSON, BUFFERS) {}", query_text);
        
        let plan_json: JsonValue = timeout(
            Duration::from_millis(100),
            sqlx::query_scalar(&explain_query)
                .fetch_one(&self.pool)
        ).await??;
        
        // Calculate plan hash
        let plan_hash = self.calculate_plan_hash(&plan_json);
        
        // Create plan metric
        let plan = ExecutionPlanMetric {
            query_id: query_id.to_string(),
            plan_id: plan_hash.clone(),
            plan_hash: plan_hash.clone(),
            database_name: self.get_current_database(&self.pool).await?,
            plan_json,
            total_cost: self.extract_total_cost(&plan_json),
            first_seen: Utc::now(),
            last_seen: Utc::now(),
            execution_count: 1,
            avg_duration_ms: 0.0,
            is_regression: false,
            previous_plan_id: None,
        };
        
        // Store in dictionary
        self.store_plan(&plan).await?;
        self.cache.put(query_id.to_string(), plan.clone()).await;
        
        Ok(plan)
    }
}
```

### 2.2 eBPF Integration for Kernel Metrics

```rust
// crates/collectors/src/ebpf/mod.rs
#[cfg(feature = "ebpf")]
use aya::{
    Bpf,
    programs::{UProbe, TracePoint},
    maps::perf::AsyncPerfEventArray,
};
use tokio::sync::mpsc;

pub struct EBPFManager {
    bpf: Bpf,
    perf_array: AsyncPerfEventArray<MapData>,
    events_tx: mpsc::Sender<KernelEvent>,
}

#[cfg(feature = "ebpf")]
impl EBPFManager {
    pub async fn attach_to_postgres(&mut self, pid: i32) -> Result<(), Box<dyn std::error::Error>> {
        // Load pre-compiled eBPF program
        let mut bpf = Bpf::load(include_bytes!("../../../bpf/target/bpfel-unknown-none/release/query_latency"))?;
        
        // Attach uprobe to PostgreSQL
        let program: &mut UProbe = bpf.program_mut("trace_exec_simple_query").unwrap().try_into()?;
        program.load()?;
        program.attach(Some("exec_simple_query"), 0, "/usr/lib/postgresql/15/bin/postgres", Some(pid))?;
        
        // Set up perf event array
        let mut perf_array = AsyncPerfEventArray::try_from(bpf.map_mut("events")?)?;
        
        // Start event processing
        for cpu_id in online_cpus()? {
            let mut buf = perf_array.open(cpu_id, None)?;
            let tx = self.events_tx.clone();
            
            tokio::spawn(async move {
                let mut buffers = vec![BytesMut::with_capacity(1024); 10];
                
                loop {
                    let events = buf.read_events(&mut buffers).await.unwrap();
                    for buf in buffers.iter_mut().take(events.read) {
                        let event: KernelEvent = buf.try_into().unwrap();
                        let _ = tx.send(event).await;
                    }
                }
            });
        }
        
        Ok(())
    }
}
```

### 2.3 Active Session History Implementation

```rust
// crates/collectors/src/ash_sampler.rs
use sqlx::PgPool;
use tokio::time::{interval, Duration};
use std::sync::Arc;
use tokio::sync::RwLock;

pub struct ASHSampler {
    interval: Duration,
    retention: Duration,
    ring_buffer: Arc<RwLock<RingBuffer<ASHSample>>>,
    aggregator: Arc<RwLock<Aggregator>>,
}

impl ASHSampler {
    pub async fn start(&self, pool: PgPool) {
        let mut ticker = interval(self.interval);
        
        loop {
            ticker.tick().await;
            
            match self.capture_sample(&pool).await {
                Ok(samples) => {
                    // Add to ring buffer
                    self.ring_buffer.write().await.add_batch(samples.clone());
                    
                    // Aggregate for metrics
                    self.aggregator.write().await.process(&samples);
                }
                Err(e) => {
                    tracing::error!("ASH sampling failed: {}", e);
                }
            }
        }
    }

    async fn capture_sample(&self, pool: &PgPool) -> Result<Vec<ASHSample>, sqlx::Error> {
        let query = r#"
            SELECT 
                clock_timestamp() as sample_time,
                pid,
                usename,
                datname,
                query_id,
                state,
                wait_event_type,
                wait_event,
                LEFT(query, 100) as query_sample,
                backend_type,
                -- CPU state would come from eBPF if available
                false as on_cpu,
                null::int as cpu_id
            FROM pg_stat_activity
            WHERE state != 'idle'
            AND pid != pg_backend_pid()
        "#;
        
        let rows = sqlx::query_as::<_, ASHSample>(query)
            .fetch_all(pool)
            .await?;
        
        Ok(rows)
    }
}
```

## 3. Unified Export Layer

### 3.1 Dual Export Support

```go
// export/manager.go
package export

type ExportManager struct {
    adapters []Adapter
    config   ExportConfig
}

// Adapter interface for different export formats
type Adapter interface {
    Name() string
    Export(metrics *model.UnifiedMetrics) error
    SupportsStreaming() bool
}

func (m *ExportManager) Export(metrics *model.UnifiedMetrics) error {
    var g errgroup.Group
    
    // Export to all configured adapters in parallel
    for _, adapter := range m.adapters {
        adapter := adapter
        g.Go(func() error {
            return adapter.Export(metrics)
        })
    }
    
    return g.Wait()
}

// Configure based on deployment mode
func NewExportManager(config ExportConfig) *ExportManager {
    manager := &ExportManager{config: config}
    
    if config.NRIEnabled {
        manager.adapters = append(manager.adapters, 
            nri.NewAdapter(config.NRI))
    }
    
    if config.OTLPEnabled {
        manager.adapters = append(manager.adapters,
            otlp.NewAdapter(config.OTLP))
    }
    
    return manager
}
```

### 3.2 OTLP Semantic Convention Mapping

```go
// export/otlp/converter.go
package otlp

import (
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
    semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func (c *OTLPConverter) ConvertSlowQuery(sq *model.SlowQueryMetric) []metric.Metric {
    attrs := []attribute.KeyValue{
        semconv.DBSystemPostgreSQL,
        semconv.DBName(sq.DatabaseName),
        semconv.DBStatement(sq.QueryText),
        attribute.String("db.postgresql.query_id", sq.QueryID),
        attribute.String("db.operation.type", sq.StatementType),
    }
    
    metrics := []metric.Metric{
        // Duration histogram
        c.histogram.Record(context.Background(), sq.AvgElapsedTimeMs,
            metric.WithAttributes(attrs...)),
        
        // Execution count
        c.counter.Add(context.Background(), sq.ExecutionCount,
            metric.WithAttributes(attrs...)),
        
        // Buffer metrics
        c.gauge.Record(context.Background(), sq.AvgDiskReads,
            metric.WithAttributes(append(attrs,
                attribute.String("buffer.operation", "read"))...)),
    }
    
    // Add extended metrics if available
    if sq.ExtendedMetrics != nil {
        metrics = append(metrics, c.convertExtended(sq.ExtendedMetrics, attrs)...)
    }
    
    return metrics
}
```

## 4. PostgreSQL Extension Design

### 4.1 pg_querylens Extension

```c
// pg_querylens/querylens.c
#include "postgres.h"
#include "executor/executor.h"
#include "storage/ipc.h"
#include "utils/guc.h"

typedef struct QueryMetrics {
    uint64_t query_id;
    char plan_id[64];
    double duration_ms;
    double cpu_ms;
    double io_ms;
    int64_t rows_returned;
    int64_t shared_hits;
    int64_t shared_reads;
    int64_t temp_bytes;
    WaitEventCounts wait_events;
} QueryMetrics;

// Shared memory ring buffer
typedef struct {
    LWLock *lock;
    uint64_t head;
    uint64_t tail;
    size_t size;
    char data[FLEXIBLE_ARRAY_MEMBER];
} MetricsRingBuffer;

static ExecutorStart_hook_type prev_ExecutorStart = NULL;
static ExecutorRun_hook_type prev_ExecutorRun = NULL;
static ExecutorEnd_hook_type prev_ExecutorEnd = NULL;

// Hook implementations
static void ql_ExecutorStart(QueryDesc *queryDesc, int eflags) {
    if (prev_ExecutorStart)
        prev_ExecutorStart(queryDesc, eflags);
    else
        standard_ExecutorStart(queryDesc, eflags);
    
    // Initialize per-query context
    QueryContext *ctx = palloc0(sizeof(QueryContext));
    ctx->start_time = GetCurrentTimestamp();
    ctx->query_id = queryDesc->plannedstmt->queryId;
    
    // Calculate plan hash
    ctx->plan_id = calculate_plan_hash(queryDesc->plannedstmt);
    
    queryDesc->totaltime = InstrAlloc(1, INSTRUMENT_ALL);
}

static void ql_ExecutorEnd(QueryDesc *queryDesc) {
    // Collect final metrics
    QueryMetrics metrics = {0};
    metrics.query_id = queryDesc->plannedstmt->queryId;
    strcpy(metrics.plan_id, ctx->plan_id);
    
    if (queryDesc->totaltime) {
        metrics.duration_ms = queryDesc->totaltime->total * 1000.0;
        metrics.rows_returned = queryDesc->totaltime->ntuples;
        metrics.shared_hits = queryDesc->totaltime->shared_blks_hit;
        metrics.shared_reads = queryDesc->totaltime->shared_blks_read;
        metrics.temp_bytes = queryDesc->totaltime->temp_blks_written * BLCKSZ;
    }
    
    // Write to ring buffer
    write_to_ring_buffer(&metrics);
    
    if (prev_ExecutorEnd)
        prev_ExecutorEnd(queryDesc);
    else
        standard_ExecutorEnd(queryDesc);
}
```

## 5. Deployment Improvements

### 5.1 Single Binary, Multiple Modes

```yaml
# Unified binary configuration
deployment:
  binary: "pgquerylens-collector"
  
  modes:
    # Mode 1: Infrastructure Agent Integration
    nri:
      command: "pgquerylens-collector --mode=nri"
      output: stdout
      format: "nri-json-v4"
      
    # Mode 2: OpenTelemetry Receiver
    otel:
      command: "pgquerylens-collector --mode=otel"
      grpc_port: 4317
      http_port: 4318
      
    # Mode 3: Hybrid (both outputs)
    hybrid:
      command: "pgquerylens-collector --mode=hybrid"
      outputs:
        - nri
        - otlp
```

### 5.2 Kubernetes Operator CRD

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: postgresqlmonitors.querylens.io
spec:
  group: querylens.io
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              postgresConnection:
                type: object
                properties:
                  host: string
                  port: integer
                  secretRef:
                    type: object
                    
              collection:
                type: object
                properties:
                  interval: string
                  enableExtension: boolean
                  enableEBPF: boolean
                  enableASH: boolean
                  
              export:
                type: object
                properties:
                  mode: string  # "nri", "otel", "hybrid"
                  nri:
                    type: object
                  otlp:
                    type: object
```

## 6. Migration Path

### 6.1 Phased Rollout

```yaml
migration_phases:
  phase1_preparation:
    duration: "2 weeks"
    tasks:
      - Install pg_querylens extension on test databases
      - Deploy collector in shadow mode
      - Validate metric parity with existing OHI
      
  phase2_pilot:
    duration: "4 weeks"
    tasks:
      - Enable on 10% of production databases
      - Monitor performance impact (target <1μs overhead)
      - Collect feedback on new metrics
      
  phase3_expansion:
    duration: "6 weeks"
    tasks:
      - Roll out to 50% of databases
      - Enable eBPF features on supported systems
      - Begin using plan regression detection
      
  phase4_completion:
    duration: "2 weeks"
    tasks:
      - Complete rollout to 100%
      - Deprecate old nri-postgresql
      - Enable all advanced features
```

### 6.2 Backward Compatibility

```go
// Maintain OHI compatibility flag
type CompatibilityMode struct {
    OHIv1Compatible     bool
    PreserveFieldNames  bool
    LimitQueryLength    bool
    DisableExtended     bool
}

func (c *Collector) CollectWithCompatibility(mode CompatibilityMode) (*Metrics, error) {
    metrics := c.Collect()
    
    if mode.OHIv1Compatible {
        // Ensure all OHI fields are populated
        metrics.EnsureOHIFields()
        
        if mode.LimitQueryLength {
            metrics.TruncateQueries(4095)
        }
        
        if mode.DisableExtended {
            metrics.ClearExtendedFields()
        }
    }
    
    return metrics, nil
}
```

## 7. Performance Optimizations

### 7.1 Adaptive Sampling

```go
// core/agg/sampler.go
package agg

type AdaptiveSampler struct {
    rules     []SamplingRule
    rateLimit RateLimiter
}

type SamplingRule struct {
    Condition  string
    SampleRate float64
    Priority   int
}

func (s *AdaptiveSampler) ShouldSample(metric *model.CommonPlanModel) bool {
    // Always sample slow queries
    if metric.Execution.DurationMs > 1000 {
        return true
    }
    
    // Always sample queries with errors
    if metric.Execution.ErrorCode != "" {
        return true
    }
    
    // Apply rule-based sampling
    for _, rule := range s.rules {
        if s.evaluateRule(rule, metric) {
            return rand.Float64() < rule.SampleRate
        }
    }
    
    // Default sampling
    return s.rateLimit.Allow()
}
```

## 8. Summary of Improvements

1. **Unified Architecture**: Single Go module with pluggable exporters
2. **Enhanced Metrics**: Full spectrum from basic counters to kernel-level tracing
3. **Live Plan Capture**: Automatic plan collection with regression detection
4. **Shared Memory Integration**: Lock-free communication with PostgreSQL
5. **eBPF Support**: Kernel-level metrics without overhead
6. **Active Session History**: 1-second granularity session sampling
7. **Backward Compatibility**: 100% OHI metric coverage maintained
8. **Cloud-Native**: Kubernetes operator and container-first design
9. **Performance**: <1μs hook overhead, adaptive sampling
10. **Vendor Neutral**: Works with any OTLP backend or New Relic