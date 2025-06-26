use anyhow::Result;
use async_trait::async_trait;
use sqlx::{postgres::PgPoolOptions, PgConnection, PgPool, Row};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::RwLock;
use tracing::{info, warn, error};

use postgres_collector_core::{
    Capabilities, CollectionMetadata, CollectorError, CommonParameters,
    ExtensionInfo, MetricBatch, PostgresCollector, UnifiedMetrics,
    SlowQueryMetric, WaitEventMetric, BlockingSessionMetric,
    IndividualQueryMetric, ExecutionPlanMetric, ProcessError,
};
use postgres_extensions::{ExtensionManager, OHIValidations, ActiveSessionSampler};
use postgres_query_engine::{OHICompatibleQueryExecutor, QueryParams};

use crate::config::CollectorConfig;

pub struct UnifiedCollectionEngine {
    // Core components
    connection_pool: PgPool,
    extension_manager: ExtensionManager,
    query_executor: OHICompatibleQueryExecutor,
    
    // Optional components
    #[cfg(feature = "ebpf")]
    ebpf_engine: Option<EbpfEngine>,
    ash_sampler: Option<ActiveSessionSampler>,
    
    // Configuration
    config: CollectorConfig,
    
    // Adapters - simplified for now
    // adapters: Vec<Box<dyn MetricAdapter>>,
    
    // Cached capabilities
    capabilities: Arc<RwLock<Option<Capabilities>>>,
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
            Some(ActiveSessionSampler::new(
                Duration::from_secs(1),
                Duration::from_secs(3600),
            ))
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
            config,
            // adapters: Vec::new(),
            capabilities: Arc::new(RwLock::new(None)),
        })
    }
    
    pub fn add_adapter<T: MetricOutput + 'static>(&mut self, adapter: Box<dyn MetricAdapter<Output = T>>) {
        // For now, we'll need to handle this differently since we can't store heterogeneous types
        // This is a limitation we'll need to address in the actual implementation
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
        if ext_config.pg_stat_statements.is_some() {
            extensions.insert(
                "pg_stat_statements".to_string(),
                ExtensionInfo {
                    name: "pg_stat_statements".to_string(),
                    version: ext_config.pg_stat_statements.as_ref().unwrap().version.clone(),
                    enabled: true,
                },
            );
        }
        
        if ext_config.pg_wait_sampling.is_some() {
            extensions.insert(
                "pg_wait_sampling".to_string(),
                ExtensionInfo {
                    name: "pg_wait_sampling".to_string(),
                    version: ext_config.pg_wait_sampling.as_ref().unwrap().version.to_string(),
                    enabled: true,
                },
            );
        }
        
        if ext_config.pg_stat_monitor.is_some() {
            extensions.insert(
                "pg_stat_monitor".to_string(),
                ExtensionInfo {
                    name: "pg_stat_monitor".to_string(),
                    version: ext_config.pg_stat_monitor.as_ref().unwrap().version.clone(),
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
            metrics.slow_queries = self.query_executor
                .execute_slow_queries(&mut conn, &params)
                .await?;
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
            let individual_queries = self.correlate_queries_by_text(&metrics.slow_queries, &mut conn).await?;
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
        for mut aq in active_queries {
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
        let explain_query = format!("EXPLAIN (FORMAT JSON) {}", query_text);
        
        let result = sqlx::query(&explain_query)
            .fetch_one(conn)
            .await?;
        
        // Parse the JSON plan - for now just create a placeholder
        let plan_json = serde_json::json!({"plan": "placeholder"});
        
        Ok(ExecutionPlanMetric {
            query_id: None, // Would need to compute hash
            query_text: Some(query_text.to_string()),
            database_name: Some(self.config.databases[0].clone()),
            plan: Some(plan_json),
            plan_text: None,
            total_cost: None, // Would need to extract from plan
            execution_time_ms: None,
            planning_time_ms: None,
            collection_timestamp: Some(chrono::Utc::now().to_rfc3339()),
        })
    }
    
    async fn collect_extended_metrics(
        &self,
        metrics: &mut UnifiedMetrics,
        caps: &Capabilities,
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
}

#[async_trait]
pub trait MetricAdapter: Send + Sync {
    type Output: MetricOutput;
    
    async fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, ProcessError>;
}

pub trait MetricOutput: Send + Sync {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError>;
    fn content_type(&self) -> &'static str;
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