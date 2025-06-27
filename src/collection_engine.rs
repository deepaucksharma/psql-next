use anyhow::Result;
use async_trait::async_trait;
use sqlx::{postgres::PgPoolOptions, PgConnection, PgPool, Row};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::RwLock;
use tracing::{info, warn, error};

use postgres_collector_core::{
    Capabilities, CollectorError, CommonParameters,
    ExtensionInfo, UnifiedMetrics,
    SlowQueryMetric,
    IndividualQueryMetric, ExecutionPlanMetric, ProcessError,
};
use postgres_extensions::{ExtensionManager, OHIValidations, ActiveSessionSampler};
use postgres_query_engine::OHICompatibleQueryExecutor;

use crate::config::CollectorConfig;
use crate::pgbouncer::PgBouncerCollector;
use crate::sanitizer::{QuerySanitizer, SanitizationMode};
use crate::exporter::MetricExporter;

pub struct UnifiedCollectionEngine {
    // Core components
    connection_pool: PgPool,
    extension_manager: ExtensionManager,
    query_executor: OHICompatibleQueryExecutor,
    
    // Optional components
    #[cfg(feature = "ebpf")]
    ebpf_engine: Option<EbpfEngine>,
    ash_sampler: Option<ActiveSessionSampler>,
    pgbouncer_collector: Option<PgBouncerCollector>,
    
    // Configuration
    config: CollectorConfig,
    
    // Adapters using dynamic dispatch
    adapters: Vec<Box<dyn MetricAdapterDyn>>,
    
    // Cached capabilities
    capabilities: Arc<RwLock<Option<Capabilities>>>,
    
    // Query sanitizer
    query_sanitizer: Option<QuerySanitizer>,
    
    // Metric exporter
    exporter: MetricExporter,
}

impl UnifiedCollectionEngine {
    pub async fn new(config: CollectorConfig) -> Result<Self, CollectorError> {
        let connection_pool = PgPoolOptions::new()
            .max_connections(config.max_connections)
            .acquire_timeout(Duration::from_secs(config.connect_timeout_secs))
            .connect(&config.connection_string)
            .await?;
        
        let extension_manager = ExtensionManager::new();
        let query_executor = OHICompatibleQueryExecutor::new();
        
        let ash_sampler = if config.enable_ash {
            let mut sampler = ActiveSessionSampler::new(
                Duration::from_secs(config.ash_sample_interval_secs),
                Duration::from_secs(config.ash_retention_hours * 3600),
            );
            
            if let Some(limit_mb) = config.ash_max_memory_mb {
                sampler = sampler.with_memory_limit(limit_mb);
            }
            
            Some(sampler)
        } else {
            None
        };
        
        // Initialize PgBouncer collector if configured
        let pgbouncer_collector = if let Some(pgb_config) = &config.pgbouncer {
            if pgb_config.enabled {
                match PgBouncerCollector::new(&pgb_config.admin_connection_string).await {
                    Ok(collector) => {
                        info!("PgBouncer collector initialized");
                        Some(collector)
                    },
                    Err(e) => {
                        warn!("Failed to initialize PgBouncer collector: {}", e);
                        None
                    }
                }
            } else {
                None
            }
        } else {
            None
        };
        
        // Initialize query sanitizer if enabled
        let query_sanitizer = if config.sanitize_query_text {
            let mode = match config.sanitization_mode.as_deref() {
                Some("full") => SanitizationMode::Full,
                Some("smart") => SanitizationMode::Smart,
                Some("none") => SanitizationMode::None,
                _ => SanitizationMode::Smart,
            };
            Some(QuerySanitizer::new(mode))
        } else {
            None
        };
        
        Ok(Self {
            connection_pool,
            extension_manager,
            query_executor,
            #[cfg(feature = "ebpf")]
            ebpf_engine: None,
            ash_sampler,
            pgbouncer_collector,
            config,
            adapters: Vec::new(),
            capabilities: Arc::new(RwLock::new(None)),
            query_sanitizer,
            exporter: MetricExporter::new(),
        })
    }
    
    pub fn add_adapter(&mut self, adapter: Box<dyn MetricAdapterDyn>) {
        self.adapters.push(adapter);
    }
    
    pub async fn detect_capabilities(&self) -> Result<Capabilities, CollectorError> {
        let mut conn = self.connection_pool.acquire().await?;
        
        // Get version
        let version_row = sqlx::query("SELECT current_setting('server_version_num')::integer / 10000 AS version")
        .fetch_one(&mut *conn)
        .await?;
        
        let version: Option<i32> = version_row.get("version");
        let version = version.unwrap_or(12) as u64;
        
        // Detect extensions
        let mut extension_manager = ExtensionManager::new();
        let ext_config = extension_manager.detect_and_configure(&mut conn).await?;
        
        // Check if RDS
        let is_rds = self.check_is_rds(&mut conn).await?;
        
        // Build capabilities
        let mut extensions = HashMap::new();
        if let Some(ref pg_stat_statements) = ext_config.pg_stat_statements {
            extensions.insert(
                "pg_stat_statements".to_string(),
                ExtensionInfo {
                    name: "pg_stat_statements".to_string(),
                    version: pg_stat_statements.version.clone(),
                    enabled: true,
                },
            );
        }
        
        if let Some(ref pg_wait_sampling) = ext_config.pg_wait_sampling {
            extensions.insert(
                "pg_wait_sampling".to_string(),
                ExtensionInfo {
                    name: "pg_wait_sampling".to_string(),
                    version: pg_wait_sampling.version.to_string(),
                    enabled: true,
                },
            );
        }
        
        if let Some(ref pg_stat_monitor) = ext_config.pg_stat_monitor {
            extensions.insert(
                "pg_stat_monitor".to_string(),
                ExtensionInfo {
                    name: "pg_stat_monitor".to_string(),
                    version: pg_stat_monitor.version.clone(),
                    enabled: true,
                },
            );
        }
        
        let capabilities = Capabilities {
            version,
            is_rds,
            extensions,
            has_superuser: self.check_superuser(&mut conn).await?,
            has_ebpf_support: cfg!(feature = "ebpf"),
        };
        
        // Cache capabilities
        *self.capabilities.write().await = Some(capabilities.clone());
        
        Ok(capabilities)
    }
    
    async fn check_is_rds(&self, conn: &mut PgConnection) -> Result<bool, CollectorError> {
        let result = sqlx::query("SELECT 1 FROM pg_settings WHERE name = 'rds.superuser_reserved_connections'")
        .fetch_optional(conn)
        .await?;
        
        Ok(result.is_some())
    }
    
    async fn check_superuser(&self, conn: &mut PgConnection) -> Result<bool, CollectorError> {
        let result = sqlx::query("SELECT current_setting('is_superuser') = 'on' AS is_superuser")
        .fetch_one(conn)
        .await?;
        
        let is_superuser: Option<bool> = result.get("is_superuser");
        Ok(is_superuser.unwrap_or(false))
    }
    
    pub async fn collect_all_metrics(&self) -> Result<UnifiedMetrics, CollectorError> {
        let start_time = Instant::now();
        
        // Detect capabilities
        let caps = self.detect_capabilities().await?;
        
        // Create common parameters
        let params = CommonParameters {
            version: caps.version,
            databases: self.config.databases.join(","),
            query_monitoring_count_threshold: CommonParameters::validate_count_threshold(
                self.config.query_monitoring_count_threshold,
            ),
            query_monitoring_response_time_threshold: CommonParameters::validate_response_threshold(
                self.config.query_monitoring_response_time_threshold,
            ),
            host: self.config.host.clone(),
            port: self.config.port.to_string(),
            is_rds: caps.is_rds,
        };
        
        // Collect base OHI metrics
        let mut metrics = UnifiedMetrics::default();
        let mut conn = self.connection_pool.acquire().await?;
        
        // Always collect slow queries if pg_stat_statements is available
        if OHIValidations::check_slow_query_metrics_fetch_eligibility(&caps.extensions) {
            info!("Collecting slow query metrics");
            let mut slow_queries = self.query_executor
                .execute_slow_queries(&mut conn, &params)
                .await?;
            
            // Sanitize query text if enabled
            if let Some(sanitizer) = &self.query_sanitizer {
                for query in &mut slow_queries {
                    if let Some(text) = query.query_text.clone() {
                        query.query_text = Some(sanitizer.sanitize(&text));
                        
                        // Add warning if PII detected
                        if let Some(warning) = sanitizer.get_pii_warning(&text) {
                            warn!("PII detected in query: {}", warning);
                        }
                    }
                }
            }
            
            metrics.slow_queries = slow_queries;
        } else {
            warn!("pg_stat_statements not available, skipping slow query metrics");
        }
        
        // Collect blocking sessions based on version
        if OHIValidations::check_blocking_session_metrics_fetch_eligibility(&caps.extensions, caps.version) {
            info!("Collecting blocking session metrics");
            metrics.blocking_sessions = self.query_executor
                .execute_blocking_sessions(&mut conn, &params)
                .await?;
        }
        
        // Collect wait events
        if OHIValidations::check_wait_event_metrics_fetch_eligibility(&caps.extensions) && !caps.is_rds {
            info!("Collecting wait event metrics");
            metrics.wait_events = self.query_executor
                .execute_wait_events(&mut conn, &params)
                .await?;
        } else if caps.is_rds {
            // RDS fallback mode
            info!("Collecting wait events in RDS mode");
            metrics.wait_events = self.query_executor
                .execute_wait_events(&mut conn, &params)
                .await?;
        }
        
        // Collect individual queries
        if OHIValidations::check_individual_query_metrics_fetch_eligibility(&caps.extensions) && !caps.is_rds {
            info!("Collecting individual query metrics");
            metrics.individual_queries = self.query_executor
                .execute_individual_queries(&mut conn, &params)
                .await?;
        } else if caps.is_rds {
            // RDS mode: correlate through text
            info!("Collecting individual queries in RDS mode");
            let mut individual_queries = self.correlate_queries_by_text(&metrics.slow_queries, &mut conn).await?;
            
            // Sanitize individual query text
            if let Some(sanitizer) = &self.query_sanitizer {
                for query in &mut individual_queries {
                    if let Some(text) = &query.query_text {
                        query.query_text = Some(sanitizer.sanitize(text));
                    }
                }
            }
            
            metrics.individual_queries = individual_queries;
        }
        
        // Collect execution plans
        if !metrics.individual_queries.is_empty() {
            info!("Collecting execution plans");
            metrics.execution_plans = self.collect_execution_plans(&metrics.individual_queries, &mut conn).await?;
        }
        
        // Extended metrics if enabled
        if self.config.enable_extended_metrics {
            self.collect_extended_metrics(&mut metrics, &caps).await?;
        }
        
        // PgBouncer metrics if enabled
        if let Some(pgb_collector) = &self.pgbouncer_collector {
            info!("Collecting PgBouncer metrics");
            match pgb_collector.collect_metrics().await {
                Ok(pgb_metrics) => {
                    metrics.pgbouncer_metrics = Some(serde_json::to_value(pgb_metrics).unwrap_or(serde_json::Value::Null));
                    info!("PgBouncer metrics collected successfully");
                }
                Err(e) => {
                    warn!("Failed to collect PgBouncer metrics: {}", e);
                }
            }
        }
        
        let collection_duration = start_time.elapsed();
        info!("Metrics collection completed in {:?}", collection_duration);
        
        Ok(metrics)
    }
    
    async fn correlate_queries_by_text(
        &self,
        slow_queries: &[SlowQueryMetric],
        conn: &mut PgConnection,
    ) -> Result<Vec<IndividualQueryMetric>, CollectorError> {
        // RDS mode: Get individual queries from pg_stat_activity
        let active_queries = sqlx::query_as::<_, IndividualQueryMetric>(
            "SELECT 
                pid,
                queryid AS query_id,
                LEFT(query, 4095) AS query_text,
                state,
                wait_event_type,
                wait_event,
                usename,
                datname AS database_name,
                to_char(backend_start AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS backend_start,
                to_char(xact_start AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS xact_start,
                to_char(query_start AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS query_start,
                to_char(state_change AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS state_change,
                backend_type,
                to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"') AS collection_timestamp
            FROM pg_stat_activity
            WHERE query IS NOT NULL AND query != ''"
        )
        .fetch_all(conn)
        .await?;
        
        // Create normalized text map from slow queries
        let mut slow_query_map = HashMap::new();
        for sq in slow_queries {
            if let Some(text) = &sq.query_text {
                let normalized = postgres_query_engine::utils::anonymize_and_normalize(text);
                slow_query_map.insert(normalized, sq.query_id.clone());
            }
        }
        
        // Filter active queries that match slow queries
        let mut correlated = Vec::new();
        for aq in active_queries {
            if let Some(text) = &aq.query_text {
                let normalized = postgres_query_engine::utils::anonymize_and_normalize(text);
                if slow_query_map.contains_key(&normalized) {
                    correlated.push(aq);
                }
            }
        }
        
        Ok(correlated)
    }
    
    async fn collect_execution_plans(
        &self,
        individual_queries: &[IndividualQueryMetric],
        conn: &mut PgConnection,
    ) -> Result<Vec<ExecutionPlanMetric>, CollectorError> {
        let mut plans = Vec::new();
        
        for query in individual_queries.iter().take(10) { // Limit to top 10
            if let Some(query_text) = &query.query_text {
                match self.get_execution_plan(query_text, conn).await {
                    Ok(plan) => plans.push(plan),
                    Err(e) => warn!("Failed to get execution plan: {}", e),
                }
            }
        }
        
        Ok(plans)
    }
    
    async fn get_execution_plan(
        &self,
        query_text: &str,
        conn: &mut PgConnection,
    ) -> Result<ExecutionPlanMetric, CollectorError> {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};
        
        // Escape single quotes in the query text
        let escaped_query = query_text.replace("'", "''");
        let explain_query = format!("EXPLAIN (FORMAT JSON, BUFFERS, ANALYZE) {}", escaped_query);
        
        let result: (serde_json::Value,) = sqlx::query_as(&explain_query)
            .fetch_one(conn)
            .await?;
        
        let plan_json = result.0;
        
        // Extract plan details
        let mut total_cost = None;
        let mut execution_time_ms = None;
        let mut planning_time_ms = None;
        
        if let Some(plan_array) = plan_json.as_array() {
            if let Some(first_plan) = plan_array.first() {
                // Extract execution time
                if let Some(exec_time) = first_plan.get("Execution Time") {
                    execution_time_ms = exec_time.as_f64();
                }
                
                // Extract planning time
                if let Some(plan_time) = first_plan.get("Planning Time") {
                    planning_time_ms = plan_time.as_f64();
                }
                
                // Extract total cost from the Plan object
                if let Some(plan) = first_plan.get("Plan") {
                    if let Some(cost) = plan.get("Total Cost") {
                        total_cost = cost.as_f64();
                    }
                }
            }
        }
        
        // Compute query hash for query_id
        let mut hasher = DefaultHasher::new();
        query_text.hash(&mut hasher);
        let query_id = hasher.finish() as i64;
        
        Ok(ExecutionPlanMetric {
            query_id: Some(query_id),
            query_text: Some(query_text.to_string()),
            database_name: self.config.databases.first().cloned(),
            plan: Some(plan_json),
            plan_text: None,
            total_cost,
            execution_time_ms,
            planning_time_ms,
            collection_timestamp: Some(chrono::Utc::now().to_rfc3339()),
        })
    }
    
    async fn collect_extended_metrics(
        &self,
        metrics: &mut UnifiedMetrics,
        _caps: &Capabilities,
    ) -> Result<(), CollectorError> {
        // Active Session History
        if let Some(ash) = &self.ash_sampler {
            metrics.active_session_history = ash.get_recent_samples().await;
        }
        
        // eBPF metrics
        #[cfg(feature = "ebpf")]
        if let Some(ebpf) = &self.ebpf_engine {
            if caps.has_ebpf_support {
                let ebpf_metrics = ebpf.collect_metrics().await?;
                self.enrich_with_ebpf_data(metrics, ebpf_metrics)?;
            }
        }
        
        // Plan change detection would go here
        
        Ok(())
    }
    
    #[cfg(feature = "ebpf")]
    async fn enrich_with_ebpf_data(
        &self,
        metrics: &mut UnifiedMetrics,
        ebpf_metrics: Vec<KernelMetric>,
    ) -> Result<(), CollectorError> {
        // Enrich existing metrics with kernel-level data
        metrics.kernel_metrics = ebpf_metrics;
        Ok(())
    }
    
    pub async fn send_metrics(&self, metrics: &UnifiedMetrics) -> Result<(), CollectorError> {
        let mut errors = Vec::new();
        
        for adapter in &self.adapters {
            match adapter.adapt_dyn(metrics).await {
                Ok(output) => {
                    match output.serialize() {
                        Ok(data) => {
                            info!(
                                "Successfully serialized metrics for {} adapter ({} bytes)",
                                adapter.name(),
                                data.len()
                            );
                            
                            // Send metrics based on adapter type
                            match adapter.name() {
                                "NRI" => {
                                    // NRI outputs to stdout for infrastructure agent to capture
                                    println!("{}", String::from_utf8_lossy(&data));
                                    info!("NRI metrics sent to stdout");
                                }
                                "OpenTelemetry" => {
                                    // OTLP sends to configured endpoint
                                    if let Some(otlp_config) = &self.config.outputs.otlp {
                                        if otlp_config.enabled {
                                            // Append the correct path for OTLP HTTP metrics endpoint
                                            let endpoint = if otlp_config.endpoint.ends_with('/') {
                                                format!("{}v1/metrics", otlp_config.endpoint)
                                            } else {
                                                format!("{}/v1/metrics", otlp_config.endpoint)
                                            };
                                            
                                            match self.exporter.export_http(
                                                &endpoint,
                                                data,
                                                output.content_type(),
                                                &otlp_config.headers,
                                            ).await {
                                                Ok(_) => info!("OTLP metrics sent to {}", endpoint),
                                                Err(e) => {
                                                    error!("Failed to export OTLP metrics to {}: {}", endpoint, e);
                                                    errors.push(format!("OTLP export failed: {}", e));
                                                }
                                            }
                                        }
                                    }
                                }
                                _ => {
                                    warn!("Unknown adapter type: {}", adapter.name());
                                }
                            }
                        }
                        Err(e) => {
                            error!("Failed to serialize metrics for {}: {}", adapter.name(), e);
                            errors.push(format!("{}: serialization failed - {}", adapter.name(), e));
                        }
                    }
                }
                Err(e) => {
                    error!("Failed to adapt metrics for {}: {}", adapter.name(), e);
                    errors.push(format!("{}: adaptation failed - {}", adapter.name(), e));
                }
            }
        }
        
        if !errors.is_empty() {
            return Err(CollectorError::General(
                anyhow::anyhow!("Failed to send metrics: {}", errors.join("; "))
            ));
        }
        
        Ok(())
    }
}

#[async_trait]
pub trait MetricAdapter: Send + Sync {
    type Output: postgres_collector_core::MetricOutput;
    
    async fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, ProcessError>;
}

// Dynamic dispatch trait for storing heterogeneous adapters
#[async_trait]
pub trait MetricAdapterDyn: Send + Sync {
    async fn adapt_dyn(&self, metrics: &UnifiedMetrics) -> Result<Box<dyn MetricOutputDyn>, ProcessError>;
    fn name(&self) -> &str;
}

// Dynamic dispatch trait for outputs
pub trait MetricOutputDyn: Send + Sync {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError>;
    fn content_type(&self) -> &'static str;
}

// Blanket implementation to convert concrete outputs to dynamic
impl<T: postgres_collector_core::MetricOutput + Send + Sync + 'static> MetricOutputDyn for T {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError> {
        postgres_collector_core::MetricOutput::serialize(self)
    }
    
    fn content_type(&self) -> &'static str {
        postgres_collector_core::MetricOutput::content_type(self)
    }
}

// Placeholder for eBPF engine
#[cfg(feature = "ebpf")]
pub struct EbpfEngine;

#[cfg(feature = "ebpf")]
impl EbpfEngine {
    pub async fn collect_metrics(&self) -> Result<Vec<KernelMetric>, CollectorError> {
        Ok(Vec::new())
    }
}