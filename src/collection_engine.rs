use anyhow::Result;
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
    IndividualQueryMetric, ExecutionPlanMetric,
};
use postgres_extensions::{ExtensionManager, ActiveSessionSampler};
use postgres_query_engine::SafeQueryExecutor;
use postgres_otel_adapter::OTelAdapter;

use crate::config::CollectorConfig;
use crate::pgbouncer::PgBouncerCollector;
use crate::sanitizer::{QuerySanitizer, SanitizationMode};
use crate::exporter::MetricExporter;
use crate::metrics::DimensionalMetrics;

pub struct UnifiedCollectionEngine {
    // Core components
    connection_pool: PgPool,
    extension_manager: ExtensionManager,
    
    // Optional components
    #[cfg(feature = "ebpf")]
    ebpf_engine: Option<EbpfEngine>,
    ash_sampler: Option<ActiveSessionSampler>,
    pgbouncer_collector: Option<PgBouncerCollector>,
    
    // Configuration
    config: CollectorConfig,
    
    // OpenTelemetry adapter
    otel_adapter: OTelAdapter,
    
    // Dimensional metrics
    dimensional_metrics: Option<DimensionalMetrics>,
    
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
        let otel_adapter = OTelAdapter::new(config.outputs.otlp.endpoint.clone());
        
        let ash_sampler = if config.enable_ash {
            let mut sampler = ActiveSessionSampler::new(
                Duration::from_secs(config.ash_sample_interval_secs),
                Duration::from_secs(config.ash_retention_hours * 3600),
            );
            
            if let Some(limit_mb) = config.ash_max_memory_mb {
                sampler = sampler.with_memory_limit(limit_mb);
            }
            
            // Start the ASH sampling background task
            sampler.start_sampling(connection_pool.clone()).await;
            info!("Active Session History sampling started with {}s interval", config.ash_sample_interval_secs);
            
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
            #[cfg(feature = "ebpf")]
            ebpf_engine: None,
            ash_sampler,
            pgbouncer_collector,
            config,
            otel_adapter,
            dimensional_metrics: None,
            capabilities: Arc::new(RwLock::new(None)),
            query_sanitizer,
            exporter: MetricExporter::new(),
        })
    }
    
    pub fn set_dimensional_metrics(&mut self, metrics: DimensionalMetrics) {
        self.dimensional_metrics = Some(metrics);
    }
    
    pub async fn detect_capabilities(&mut self) -> Result<Capabilities, CollectorError> {
        let mut conn = self.connection_pool.acquire().await?;
        
        // Get version
        let version_row = sqlx::query("SELECT current_setting('server_version_num')::integer / 10000 AS version")
        .fetch_one(&mut *conn)
        .await?;
        
        let version: Option<i32> = version_row.get("version");
        let version = version.unwrap_or(12) as u64;
        
        // Use the instance extension manager for detection
        let ext_config = self.extension_manager.detect_and_configure(&mut conn).await?;
        
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
            extensions: extensions.clone(),
            has_superuser: self.check_superuser(&mut conn).await?,
            has_ebpf_support: cfg!(feature = "ebpf"),
        };
        
        // Cache capabilities
        *self.capabilities.write().await = Some(capabilities.clone());
        
        info!("Detected PostgreSQL capabilities: version={}, extensions={}, is_rds={}, has_superuser={}", 
              version, capabilities.extensions.len(), is_rds, capabilities.has_superuser);
        
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
    
    pub async fn collect_all_metrics(&mut self) -> Result<UnifiedMetrics, CollectorError> {
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
        
        // Collect slow queries if pg_stat_statements is available
        if caps.extensions.contains_key("pg_stat_statements") {
            info!("Collecting slow query metrics");
            let mut slow_queries = SafeQueryExecutor::execute_slow_queries(&mut conn, &params, caps.version)
                .await?;
            
            // Sanitize query text if enabled
            if let Some(sanitizer) = &self.query_sanitizer {
                for query in &mut slow_queries {
                    if let Some(text) = query.query_text.clone() {
                        // Add warning if PII detected
                        if let Some(warning) = sanitizer.get_pii_warning(&text) {
                            warn!("PII detected in query: {}", warning);
                        }
                        query.query_text = Some(sanitizer.sanitize(&text));
                    }
                }
            }
            
            metrics.slow_queries = slow_queries;
        }
        
        // Collect blocking sessions
        info!("Collecting blocking session metrics");
        metrics.blocking_sessions = SafeQueryExecutor::execute_blocking_sessions(&mut conn, &params, caps.version, caps.is_rds)
            .await?;
        
        // Collect wait events
        info!("Collecting wait event metrics");
        metrics.wait_events = SafeQueryExecutor::execute_wait_events(&mut conn, &params, caps.is_rds)
            .await?;
        
        // Collect individual queries
        info!("Collecting individual query metrics");
        let mut individual_queries = SafeQueryExecutor::execute_individual_queries(&mut conn, &params, caps.version, caps.is_rds)
            .await?;
        
        // Sanitize individual query text
        if let Some(sanitizer) = &self.query_sanitizer {
            for query in &mut individual_queries {
                if let Some(text) = &query.query_text {
                    query.query_text = Some(sanitizer.sanitize(text));
                }
            }
        }
        
        metrics.individual_queries = individual_queries;
        
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
        
        // Record metrics using dimensional approach
        if let Some(dim_metrics) = &self.dimensional_metrics {
            self.record_dimensional_metrics(dim_metrics, &metrics, collection_duration)?;
        }
        
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
    
    fn record_dimensional_metrics(
        &self,
        dim_metrics: &DimensionalMetrics,
        metrics: &UnifiedMetrics,
        collection_duration: Duration,
    ) -> Result<(), CollectorError> {
        use opentelemetry::KeyValue;
        
        // Record collection duration
        dim_metrics.record_collection_duration(collection_duration.as_secs_f64());
        
        // Record slow queries
        for slow_query in &metrics.slow_queries {
            let attrs = OTelAdapter::query_attributes(slow_query);
            
            // Record query duration
            if let Some(mean_exec_time) = slow_query.mean_exec_time {
                dim_metrics.record_query_duration(mean_exec_time, &attrs);
            }
            
            // Record query count
            if let Some(calls) = slow_query.calls {
                dim_metrics.record_query_count(calls as u64, &attrs);
            }
            
            // Record rows
            if let Some(rows) = slow_query.rows {
                dim_metrics.record_query_rows(rows as u64, &attrs);
            }
            
            // Record shared buffer hits
            if let Some(shared_blks_hit) = slow_query.shared_blks_hit {
                dim_metrics.record_shared_buffer_hits(shared_blks_hit as u64, &attrs);
            }
            
            // Record shared buffer reads
            if let Some(shared_blks_read) = slow_query.shared_blks_read {
                dim_metrics.record_shared_buffer_reads(shared_blks_read as u64, &attrs);
            }
            
            // Record temp blocks
            if let Some(temp_blks_written) = slow_query.temp_blks_written {
                dim_metrics.record_temp_blocks_written(temp_blks_written as u64, &attrs);
            }
        }
        
        // Record wait events
        for wait_event in &metrics.wait_events {
            let attrs = OTelAdapter::wait_event_attributes(wait_event);
            
            // Record wait duration - using 1.0 as placeholder since actual duration not in metric
            dim_metrics.record_wait_duration(1.0, &attrs);
            
            // Record wait count
            dim_metrics.record_wait_count(1, &attrs);
        }
        
        // Record blocking sessions
        for blocking_session in &metrics.blocking_sessions {
            let attrs = OTelAdapter::blocking_session_attributes(blocking_session);
            
            // Record blocking time - using blocking_duration if available
            if let Some(duration) = &blocking_session.blocking_duration {
                // Parse duration string to seconds
                if let Ok(seconds) = parse_interval_to_seconds(duration) {
                    dim_metrics.record_blocking_time(seconds, &attrs);
                }
            }
            
            // Record blocking session count
            dim_metrics.record_blocking_sessions(1, &attrs);
        }
        
        // Record database metrics
        let db_attrs = vec![
            KeyValue::new("db.name", self.config.databases.first().cloned().unwrap_or_default()),
            KeyValue::new("db.system", "postgresql"),
        ];
        
        // Record metric counts
        dim_metrics.record_slow_queries_collected(metrics.slow_queries.len() as u64, &db_attrs);
        
        Ok(())
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
        use postgres_collector_core::MetricOutput;
        
        // Adapt metrics to OTel format
        let output = self.otel_adapter.adapt(metrics)
            .map_err(|e| CollectorError::General(anyhow::anyhow!("Failed to adapt metrics: {}", e)))?;
        
        // Serialize the output
        let data = output.serialize()
            .map_err(|e| CollectorError::General(anyhow::anyhow!("Failed to serialize metrics: {}", e)))?;
        
        info!("Successfully serialized metrics ({} bytes)", data.len());
        
        let otlp_config = &self.config.outputs.otlp;
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
                Ok(_) => {
                    info!("OTLP metrics sent to {}", endpoint);
                    Ok(())
                }
                Err(e) => {
                    error!("Failed to export OTLP metrics to {}: {}", endpoint, e);
                    Err(CollectorError::General(
                        anyhow::anyhow!("OTLP export failed: {}", e)
                    ))
                }
            }
        } else {
            warn!("OTLP output is disabled in configuration");
            Ok(())
        }
    }
}

// Placeholder for eBPF engine
// Helper function to parse PostgreSQL interval to seconds
fn parse_interval_to_seconds(interval: &str) -> Result<f64, CollectorError> {
    // Simple parser for common PostgreSQL interval formats
    // Examples: "00:05:23", "1 day 02:30:00", "00:00:01.234"
    
    // Try to parse as HH:MM:SS or HH:MM:SS.FFF
    if let Some(time_part) = interval.split(' ').last() {
        let parts: Vec<&str> = time_part.split(':').collect();
        if parts.len() >= 3 {
            let hours: f64 = parts[0].parse().unwrap_or(0.0);
            let minutes: f64 = parts[1].parse().unwrap_or(0.0);
            let seconds: f64 = parts[2].parse().unwrap_or(0.0);
            
            let mut total_seconds = hours * 3600.0 + minutes * 60.0 + seconds;
            
            // Check for days
            if interval.contains("day") {
                if let Some(day_part) = interval.split(" day").next() {
                    if let Ok(days) = day_part.trim().parse::<f64>() {
                        total_seconds += days * 86400.0;
                    }
                }
            }
            
            return Ok(total_seconds);
        }
    }
    
    // If we can't parse, return 0
    Ok(0.0)
}

#[cfg(feature = "ebpf")]
pub struct EbpfEngine;

#[cfg(feature = "ebpf")]
impl EbpfEngine {
    pub async fn collect_metrics(&self) -> Result<Vec<KernelMetric>, CollectorError> {
        Ok(Vec::new())
    }
}