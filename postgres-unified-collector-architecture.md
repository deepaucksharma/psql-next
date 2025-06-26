# Unified PostgreSQL Collector Architecture

## Architecture Overview

```yaml
postgres_unified_collector:
  name: "PostgreSQL Universal Collector (PUC)"
  version: "1.0.0"
  
  core_components:
    - collector_core       # Metric collection engine
    - data_models         # Unified data structures
    - adapters           # Output format adapters
    - extension_manager  # PostgreSQL extension interface
    - ebpf_engine       # Kernel-level metrics
    - query_engine      # SQL execution layer
```

## Core Architecture Design

```rust
// Core collector trait that both OTel and NRI implementations use
pub trait PostgresCollector: Send + Sync {
    type Config: CollectorConfig;
    type Output: MetricOutput;
    
    async fn collect(&self) -> Result<MetricBatch, CollectorError>;
    async fn process(&self, batch: MetricBatch) -> Result<Self::Output, ProcessError>;
    fn capabilities(&self) -> Capabilities;
}

// Unified metric structure that encompasses all OHI metrics plus extensions
pub struct UnifiedMetrics {
    // Base metrics matching OHI exactly
    pub slow_queries: Vec<SlowQueryMetric>,
    pub wait_events: Vec<WaitEventMetric>,
    pub blocking_sessions: Vec<BlockingSessionMetric>,
    pub individual_queries: Vec<IndividualQueryMetric>,
    pub execution_plans: Vec<ExecutionPlanMetric>,
    
    // Enhanced metrics
    pub per_execution_traces: Vec<ExecutionTrace>,
    pub kernel_metrics: Vec<KernelMetric>,
    pub active_session_history: Vec<ASHSample>,
    pub plan_changes: Vec<PlanChangeEvent>,
    
    // Metadata
    pub collection_metadata: CollectionMetadata,
}
```

## Data Model Architecture

### 1. Base Metrics (OHI Compatible)

```rust
// Exactly matches OHI SlowRunningQueryMetrics
#[derive(Serialize, Deserialize, Clone)]
pub struct SlowQueryMetric {
    pub newrelic: Option<String>,              // OHI compatibility
    pub query_id: Option<String>,
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

#[derive(Serialize, Deserialize, Clone)]
pub struct ExtendedSlowQueryMetrics {
    pub percentile_latencies: LatencyPercentiles,
    pub cpu_time_breakdown: CpuTimeBreakdown,
    pub memory_stats: MemoryStatistics,
    pub cache_stats: CacheStatistics,
    pub ebpf_metrics: Option<EbpfQueryMetrics>,
}

// Similar structures for other OHI metric types...
```

### 2. Pluggable Output Adapters

```rust
// Adapter pattern for different output formats
pub trait MetricAdapter {
    type Output;
    
    fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, AdapterError>;
}

// New Relic Infrastructure Agent Adapter
pub struct NRIAdapter {
    pub entity_key: String,
    pub integration_version: String,
}

impl MetricAdapter for NRIAdapter {
    type Output = integration::IntegrationProtocol;
    
    fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, AdapterError> {
        let mut integration = Integration::new("com.newrelic.postgresql", "2.0.0");
        let entity = integration.entity(&self.entity_key, "pg-instance")?;
        
        // Convert to NRI format exactly as OHI expects
        for metric in &metrics.slow_queries {
            let metric_set = entity.new_metric_set("PostgresSlowQueries");
            self.populate_ohi_fields(&metric_set, metric)?;
            
            // Add extended fields only if enabled
            if let Some(extended) = &metric.extended_metrics {
                self.add_extended_fields(&metric_set, extended)?;
            }
        }
        
        Ok(integration.protocol())
    }
}

// OpenTelemetry Adapter
pub struct OTelAdapter {
    pub resource: Resource,
    pub instrumentation_scope: InstrumentationScope,
}

impl MetricAdapter for OTelAdapter {
    type Output = opentelemetry::proto::collector::metrics::v1::ExportMetricsServiceRequest;
    
    fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, AdapterError> {
        let mut metric_data = Vec::new();
        
        // Convert to OTLP format
        for metric in &metrics.slow_queries {
            metric_data.push(self.create_gauge_metric(
                "postgresql.query.duration",
                metric.avg_elapsed_time_ms,
                vec![
                    KeyValue::new("query_id", metric.query_id.clone()),
                    KeyValue::new("database", metric.database_name.clone()),
                    KeyValue::new("statement_type", metric.statement_type.clone()),
                ],
            )?);
            
            // Add extended metrics as separate instruments
            if let Some(extended) = &metric.extended_metrics {
                metric_data.extend(self.create_extended_metrics(metric, extended)?);
            }
        }
        
        Ok(self.build_export_request(metric_data))
    }
}
```

## Collection Engine Architecture

```rust
pub struct UnifiedCollectionEngine {
    // Core components
    connection_pool: PgConnectionPool,
    extension_manager: ExtensionManager,
    query_executor: QueryExecutor,
    
    // Optional components
    ebpf_engine: Option<EbpfEngine>,
    ash_sampler: Option<ActiveSessionSampler>,
    
    // Configuration
    config: CollectorConfig,
    
    // Adapters
    adapters: Vec<Box<dyn MetricAdapter>>,
}

impl UnifiedCollectionEngine {
    pub async fn collect_all_metrics(&self) -> Result<UnifiedMetrics, CollectionError> {
        // Detect capabilities
        let caps = self.detect_capabilities().await?;
        
        // Collect base OHI metrics
        let mut metrics = UnifiedMetrics::default();
        
        // Always collect these (OHI compatibility)
        metrics.slow_queries = self.collect_slow_queries(&caps).await?;
        
        // Conditional collection based on extensions
        if caps.has_extension("pg_stat_statements") {
            if self.config.version >= 14 && !self.config.is_rds {
                metrics.blocking_sessions = self.collect_blocking_sessions_v14().await?;
            } else {
                metrics.blocking_sessions = self.collect_blocking_sessions_legacy().await?;
            }
        }
        
        if caps.has_extension("pg_wait_sampling") && !self.config.is_rds {
            metrics.wait_events = self.collect_wait_events().await?;
        } else if self.config.is_rds {
            // RDS fallback mode
            metrics.wait_events = self.collect_wait_events_rds().await?;
        }
        
        if caps.has_extension("pg_stat_monitor") && !self.config.is_rds {
            metrics.individual_queries = self.collect_individual_queries().await?;
        } else if self.config.is_rds {
            // RDS mode: correlate through text
            let individual_queries = self.correlate_queries_by_text(&metrics.slow_queries).await?;
            metrics.individual_queries = individual_queries;
        }
        
        // Collect execution plans
        metrics.execution_plans = self.collect_execution_plans(&metrics.individual_queries).await?;
        
        // Extended metrics if enabled
        if self.config.enable_extended_metrics {
            self.collect_extended_metrics(&mut metrics, &caps).await?;
        }
        
        Ok(metrics)
    }
    
    async fn collect_extended_metrics(
        &self,
        metrics: &mut UnifiedMetrics,
        caps: &Capabilities,
    ) -> Result<(), CollectionError> {
        // eBPF metrics
        if let Some(ebpf) = &self.ebpf_engine {
            if caps.has_ebpf_support {
                let ebpf_metrics = ebpf.collect_metrics().await?;
                self.enrich_with_ebpf_data(metrics, ebpf_metrics)?;
            }
        }
        
        // Active Session History
        if let Some(ash) = &self.ash_sampler {
            metrics.active_session_history = ash.get_recent_samples().await?;
        }
        
        // Plan change detection
        metrics.plan_changes = self.detect_plan_changes(metrics).await?;
        
        Ok(())
    }
}
```

## Query Implementation Layer

```rust
// Modular query implementation supporting version differences
pub struct QueryEngine {
    queries: QueryRegistry,
}

impl QueryEngine {
    pub fn new() -> Self {
        let mut queries = QueryRegistry::new();
        
        // Register OHI-compatible queries
        queries.register("slow_queries_v12", queries::SlowQueriesForV12);
        queries.register("slow_queries_v13+", queries::SlowQueriesForV13AndAbove);
        queries.register("wait_events", queries::WaitEvents);
        queries.register("wait_events_rds", queries::WaitEventsFromPgStatActivity);
        queries.register("blocking_v12_13", queries::BlockingQueriesForV12AndV13);
        queries.register("blocking_v14+", queries::BlockingQueriesForV14AndAbove);
        queries.register("blocking_rds", queries::RDSPostgresBlockingQuery);
        queries.register("individual_v12", queries::IndividualQuerySearchV12);
        queries.register("individual_v13+", queries::IndividualQuerySearchV13AndAbove);
        
        // Register extended queries
        queries.register("ash_sample", include_str!("queries/ash_sample.sql"));
        queries.register("plan_history", include_str!("queries/plan_history.sql"));
        queries.register("buffer_stats_detail", include_str!("queries/buffer_stats.sql"));
        
        Self { queries }
    }
    
    pub async fn execute_versioned<T: FromRow>(
        &self,
        conn: &PgConnection,
        query_key: &str,
        version: u64,
        params: QueryParams,
    ) -> Result<Vec<T>, QueryError> {
        let query = self.select_query_version(query_key, version)?;
        let formatted = self.format_query(query, params)?;
        
        sqlx::query_as::<_, T>(&formatted)
            .fetch_all(conn)
            .await
            .map_err(QueryError::from)
    }
}
```

## Deployment Architecture

```yaml
# Deployment configuration supporting both modes
deployment:
  # Mode 1: Standalone OTel Collector
  otel_collector_mode:
    binary: "postgres-unified-collector"
    config:
      receivers:
        postgresql:
          endpoint: "${POSTGRES_HOST}:5432"
          username: "${POSTGRES_USER}"
          collection_interval: 60s
          enable_extended_metrics: true
          
      processors:
        batch:
          timeout: 10s
          send_batch_size: 1000
          
      exporters:
        otlp:
          endpoint: "otel-collector:4317"
        prometheus:
          endpoint: "0.0.0.0:9090"
          
  # Mode 2: New Relic Infrastructure Integration
  nri_integration_mode:
    binary: "nri-postgresql-unified"
    config_protocol: "v4"
    integration:
      name: "com.newrelic.postgresql"
      protocol_version: 4
      
    # Backward compatible with existing nri-postgresql args
    env:
      HOSTNAME: "${POSTGRES_HOST}"
      PORT: "5432"
      USERNAME: "${POSTGRES_USER}"
      DATABASE: "${POSTGRES_DB}"
      COLLECTION_LIST: '{"postgres": {"schemas": ["public"]}}'
      ENABLE_SSL: true
      QUERY_MONITORING_COUNT_THRESHOLD: 20
      QUERY_MONITORING_RESPONSE_TIME_THRESHOLD: 500
      
  # Mode 3: Hybrid - Both outputs simultaneously
  hybrid_mode:
    binary: "postgres-unified-collector"
    adapters:
      - type: "nri"
        config:
          integration_name: "com.newrelic.postgresql"
      - type: "otlp"
        config:
          endpoint: "otel-collector:4317"
```

## Extension Manager Architecture

```rust
pub struct ExtensionManager {
    extensions: HashMap<String, Extension>,
    compatibility_matrix: CompatibilityMatrix,
}

impl ExtensionManager {
    pub async fn detect_and_configure(
        &mut self,
        conn: &PgConnection,
    ) -> Result<ExtensionConfig, Error> {
        // Detect installed extensions
        let installed = self.detect_installed_extensions(conn).await?;
        
        // Check compatibility
        let mut config = ExtensionConfig::default();
        
        // pg_stat_statements (required for OHI compatibility)
        if let Some(ver) = installed.get("pg_stat_statements") {
            config.pg_stat_statements = Some(PgStatStatementsConfig {
                version: ver.clone(),
                track: "all",
                max: 10000,
            });
        }
        
        // pg_stat_monitor (enhanced individual queries)
        if let Some(ver) = installed.get("pg_stat_monitor") {
            if self.compatibility_matrix.is_compatible("pg_stat_monitor", ver) {
                config.pg_stat_monitor = Some(PgStatMonitorConfig {
                    version: ver.clone(),
                    pgsm_normalized_query: true,
                    pgsm_enable_query_plan: true,
                });
            }
        }
        
        // pg_wait_sampling (wait events)
        if let Some(ver) = installed.get("pg_wait_sampling") {
            config.pg_wait_sampling = Some(PgWaitSamplingConfig {
                version: ver.clone(),
                sample_period: Duration::from_millis(10),
            });
        }
        
        Ok(config)
    }
}
```

## eBPF Integration Layer

```rust
pub struct EbpfEngine {
    programs: HashMap<String, Program>,
    perf_buffers: HashMap<String, PerfBuffer>,
}

impl EbpfEngine {
    pub fn new() -> Result<Self, EbpfError> {
        let mut engine = Self {
            programs: HashMap::new(),
            perf_buffers: HashMap::new(),
        };
        
        // Load eBPF programs
        engine.load_program("query_latency", include_bytes!("bpf/query_latency.o"))?;
        engine.load_program("wait_analysis", include_bytes!("bpf/wait_analysis.o"))?;
        engine.load_program("io_trace", include_bytes!("bpf/io_trace.o"))?;
        
        Ok(engine)
    }
    
    pub async fn enrich_query_metrics(
        &self,
        query: &mut SlowQueryMetric,
    ) -> Result<(), EbpfError> {
        if let Some(query_id) = &query.query_id {
            // Get kernel-level metrics for this query
            let kernel_metrics = self.get_kernel_metrics(query_id).await?;
            
            if let Some(extended) = &mut query.extended_metrics {
                extended.ebpf_metrics = Some(EbpfQueryMetrics {
                    kernel_cpu_time_ms: kernel_metrics.cpu_time.as_millis() as f64,
                    io_wait_time_ms: kernel_metrics.io_wait.as_millis() as f64,
                    scheduler_wait_time_ms: kernel_metrics.sched_wait.as_millis() as f64,
                    syscall_count: kernel_metrics.syscall_count,
                    context_switches: kernel_metrics.context_switches,
                });
            }
        }
        
        Ok(())
    }
}
```

## Active Session History Implementation

```rust
pub struct ActiveSessionSampler {
    sample_interval: Duration,
    retention_period: Duration,
    samples: Arc<RwLock<VecDeque<ASHSample>>>,
}

impl ActiveSessionSampler {
    pub async fn start_sampling(&self, conn_pool: PgConnectionPool) {
        let samples = self.samples.clone();
        let interval = self.sample_interval;
        
        tokio::spawn(async move {
            let mut interval_timer = tokio::time::interval(interval);
            
            loop {
                interval_timer.tick().await;
                
                if let Ok(conn) = conn_pool.get().await {
                    if let Ok(current_samples) = Self::capture_active_sessions(&conn).await {
                        let mut samples_guard = samples.write().await;
                        
                        for sample in current_samples {
                            samples_guard.push_back(sample);
                        }
                        
                        // Maintain retention window
                        let cutoff = Instant::now() - self.retention_period;
                        while let Some(front) = samples_guard.front() {
                            if front.sample_time < cutoff {
                                samples_guard.pop_front();
                            } else {
                                break;
                            }
                        }
                    }
                }
            }
        });
    }
    
    async fn capture_active_sessions(conn: &PgConnection) -> Result<Vec<ASHSample>, Error> {
        let query = r#"
            SELECT
                pid,
                usename,
                datname,
                query_id,
                state,
                wait_event_type,
                wait_event,
                query,
                backend_type,
                NOW() as sample_time
            FROM pg_stat_activity
            WHERE state != 'idle'
                AND pid != pg_backend_pid()
        "#;
        
        sqlx::query_as::<_, ASHSample>(query)
            .fetch_all(conn)
            .await
            .map_err(Error::from)
    }
}
```

## Configuration Management

```toml
# Unified configuration supporting both modes
[collector]
mode = "hybrid"  # "otel", "nri", or "hybrid"
collection_interval = "60s"

[postgresql]
host = "localhost"
port = 5432
username = "postgres"
database = "postgres"
ssl_mode = "prefer"

# OHI compatibility settings
[ohi_compatibility]
enabled = true
query_monitoring_count_threshold = 20
query_monitoring_response_time_threshold = 500
max_query_length = 4095

# Extended metrics
[extended_metrics]
enabled = true
ebpf_enabled = true
ash_enabled = true
ash_sample_interval = "1s"
ash_retention = "1h"

# Output configuration
[outputs.nri]
enabled = true
entity_key = "${HOSTNAME}:${PORT}"
integration_name = "com.newrelic.postgresql"

[outputs.otlp]
enabled = true
endpoint = "http://otel-collector:4317"
compression = "gzip"
timeout = "30s"

# Adaptive sampling
[sampling]
mode = "adaptive"
base_sample_rate = 1.0

[[sampling.rules]]
condition = "query_count > 1000/min"
sample_rate = 0.1

[[sampling.rules]]
condition = "avg_elapsed_time_ms > 1000"
sample_rate = 1.0

[[sampling.rules]]
condition = "statement_type = 'DDL'"
sample_rate = 1.0
```

## Build and Packaging

```yaml
# Multi-output build configuration
build:
  targets:
    # Standalone OTel collector
    - name: "postgres-otel-collector"
      binary: "postgres-unified-collector"
      features: ["otel", "ebpf", "extended"]
      
    # New Relic Infrastructure integration
    - name: "nri-postgresql-v2"
      binary: "nri-postgresql"
      features: ["nri", "ohi-compat"]
      
    # Library for embedding
    - name: "libpostgres-collector"
      type: "cdylib"
      features: ["ffi", "core"]
      
  packages:
    # RPM/DEB for Infrastructure agent
    - type: "rpm"
      name: "nri-postgresql"
      files:
        - src: "target/release/nri-postgresql"
          dst: "/var/db/newrelic-infra/newrelic-integrations/bin/"
        - src: "postgresql-config.yml"
          dst: "/etc/newrelic-infra/integrations.d/"
          
    # Container image
    - type: "docker"
      name: "postgres-unified-collector"
      base: "alpine:3.18"
      binary: "postgres-unified-collector"
```

## Integration Examples

### 1. Using with New Relic Infrastructure Agent

```yaml
# /etc/newrelic-infra/integrations.d/postgresql-config.yml
integrations:
  - name: nri-postgresql
    env:
      METRICS: true
      INVENTORY: true
      HOSTNAME: postgresql.example.com
      PORT: 5432
      USERNAME: ${POSTGRES_USER}
      PASSWORD: ${POSTGRES_PASSWORD}
      DATABASE: postgres
      COLLECTION_LIST: '{"postgres": {"schemas": ["public", "app"]}}'
      ENABLE_SSL: true
      TRUST_SERVER_CERTIFICATE: false
      SSL_ROOT_CERT_LOCATION: /etc/ssl/certs/ca-cert.pem
      TIMEOUT: 30
      # Query performance monitoring
      QUERY_MONITORING: true
      QUERY_MONITORING_COUNT_THRESHOLD: 20
      QUERY_MONITORING_RESPONSE_TIME_THRESHOLD: 500
      # Extended metrics
      ENABLE_EXTENDED_METRICS: true
      ENABLE_EBPF: true
      ENABLE_ASH: true
```

### 2. Using as OTel Collector

```yaml
# otel-collector-config.yaml
receivers:
  postgresql/unified:
    endpoint: postgresql.example.com:5432
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - postgres
      - app_db
    collection_interval: 60s
    initial_delay: 10s
    
    # Feature flags
    features:
      extended_metrics: true
      ebpf_integration: true
      active_session_history: true
      plan_change_detection: true
    
    # OHI compatibility mode
    ohi_compatibility:
      enabled: true
      event_types:
        - PostgresSlowQueries
        - PostgresWaitEvents
        - PostgresBlockingSessions
        - PostgresIndividualQueries
        - PostgresExecutionPlanMetrics

processors:
  batch:
    timeout: 10s
    send_batch_size: 1000
    
  attributes:
    actions:
      - key: service.name
        value: "postgresql"
        action: insert
      - key: deployment.environment
        from_attribute: ENVIRONMENT
        action: insert

exporters:
  otlp:
    endpoint: otel-backend:4317
    
  prometheus:
    endpoint: 0.0.0.0:9090
    namespace: postgresql
    
service:
  pipelines:
    metrics:
      receivers: [postgresql/unified]
      processors: [batch, attributes]
      exporters: [otlp, prometheus]
```

### 3. Programmatic Usage

```rust
use postgres_unified_collector::{
    UnifiedCollectionEngine,
    CollectorConfig,
    NRIAdapter,
    OTelAdapter,
};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Create collector
    let config = CollectorConfig::from_env()?;
    let engine = UnifiedCollectionEngine::new(config).await?;
    
    // Add adapters based on mode
    if env::var("NRI_MODE").is_ok() {
        engine.add_adapter(Box::new(NRIAdapter::new()));
    }
    
    if env::var("OTEL_MODE").is_ok() {
        engine.add_adapter(Box::new(OTelAdapter::new()));
    }
    
    // Start collection loop
    let mut interval = tokio::time::interval(Duration::from_secs(60));
    
    loop {
        interval.tick().await;
        
        // Collect metrics
        let metrics = engine.collect_all_metrics().await?;
        
        // Send through all configured adapters
        engine.send_metrics(metrics).await?;
    }
}
```

## Summary

This unified architecture provides:

1. **100% OHI Metric Coverage**: All existing metrics are collected with identical schemas
2. **Dual Mode Operation**: Works as both OTel collector and NRI integration
3. **Extended Capabilities**: eBPF, ASH, plan detection, and more
4. **Backward Compatibility**: Drop-in replacement for existing nri-postgresql
5. **Cloud-Native Design**: Kubernetes-ready with horizontal scaling support
6. **Flexible Deployment**: Binary, container, or embedded library options
7. **Unified Codebase**: Single implementation for multiple output formats