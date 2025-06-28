use anyhow::{Context, Result};
use async_trait::async_trait;
use chrono::{DateTime, Utc};
use sqlx::{PgPool, Row};
use std::collections::HashMap;
use std::time::{Duration, Instant};
use tracing::{debug, error, info, warn};

use crate::config::CollectorConfig;
use crate::metrics::{
    ActiveSessionMetric, BlockingSessionMetric, DatabaseMetric, ExecutionPlanMetric,
    ExtendedSlowQueryMetrics, IndexMetric, LockMetric, SlowQueryMetric, TableMetric,
    UnifiedMetrics, WaitEventMetric,
};

/// Core collection engine that handles all PostgreSQL telemetry gathering
pub struct UnifiedCollectionEngine {
    pool: PgPool,
    config: CollectorConfig,
    plan_cache: PlanCache,
    query_fingerprints: QueryFingerprintCache,
    collection_stats: CollectionStats,
}

/// Cache for tracking query plan changes
struct PlanCache {
    cache: lru::LruCache<String, PlanFingerprint>,
}

/// Fingerprint for a query execution plan
#[derive(Clone, PartialEq)]
struct PlanFingerprint {
    hash: String,
    cost: f64,
    timestamp: DateTime<Utc>,
    node_count: u32,
}

/// Cache for query fingerprints and normalization
struct QueryFingerprintCache {
    cache: lru::LruCache<String, QueryFingerprint>,
}

#[derive(Clone)]
struct QueryFingerprint {
    normalized_text: String,
    parameter_count: u32,
    tables: Vec<String>,
}

/// Statistics about collection performance
#[derive(Default)]
struct CollectionStats {
    last_collection_duration: Duration,
    queries_collected: u64,
    errors_encountered: u64,
    plan_changes_detected: u64,
}

impl UnifiedCollectionEngine {
    pub async fn new(config: CollectorConfig) -> Result<Self> {
        let pool = PgPool::connect(&config.connection_string)
            .await
            .context("Failed to connect to PostgreSQL")?;

        Ok(Self {
            pool,
            config,
            plan_cache: PlanCache::new(10000),
            query_fingerprints: QueryFingerprintCache::new(10000),
            collection_stats: CollectionStats::default(),
        })
    }

    /// Main collection entry point with comprehensive error handling
    pub async fn collect_metrics(&mut self) -> Result<UnifiedMetrics> {
        let start = Instant::now();
        let mut metrics = UnifiedMetrics::new();

        // Set collection metadata
        metrics.collection_timestamp = Utc::now();
        metrics.collector_version = env!("CARGO_PKG_VERSION").to_string();

        // Detect database capabilities
        let capabilities = self.detect_capabilities().await?;
        metrics.capabilities = capabilities.clone();

        // Collect metrics in parallel where possible
        let (database_metrics, slow_queries, wait_events, locks, blocking_sessions) = tokio::join!(
            self.collect_database_metrics(),
            self.collect_slow_queries(&capabilities),
            self.collect_wait_events(&capabilities),
            self.collect_locks(),
            self.collect_blocking_sessions()
        );

        // Handle results with detailed error context
        metrics.database_metrics = database_metrics
            .context("Failed to collect database metrics")
            .unwrap_or_else(|e| {
                error!("Database metrics collection failed: {}", e);
                self.collection_stats.errors_encountered += 1;
                vec![]
            });

        metrics.slow_queries = slow_queries
            .context("Failed to collect slow queries")
            .unwrap_or_else(|e| {
                error!("Slow query collection failed: {}", e);
                self.collection_stats.errors_encountered += 1;
                vec![]
            });

        metrics.wait_events = wait_events
            .context("Failed to collect wait events")
            .unwrap_or_else(|e| {
                error!("Wait event collection failed: {}", e);
                self.collection_stats.errors_encountered += 1;
                vec![]
            });

        metrics.locks = locks.unwrap_or_default();
        metrics.blocking_sessions = blocking_sessions.unwrap_or_default();

        // Collect table and index metrics if not in minimal mode
        if !self.config.minimal_mode {
            metrics.table_metrics = self.collect_table_metrics().await.unwrap_or_default();
            metrics.index_metrics = self.collect_index_metrics().await.unwrap_or_default();
        }

        // Collect execution plans for top queries
        if self.config.enable_execution_plans {
            self.collect_execution_plans(&mut metrics).await;
        }

        // Update collection statistics
        self.collection_stats.last_collection_duration = start.elapsed();
        self.collection_stats.queries_collected = metrics.slow_queries.len() as u64;

        // Add self-monitoring metrics
        metrics.collector_metrics = Some(self.get_collector_metrics());

        Ok(metrics)
    }

    /// Detect available PostgreSQL extensions and features
    async fn detect_capabilities(&self) -> Result<HashMap<String, bool>> {
        let mut capabilities = HashMap::new();

        // Check PostgreSQL version
        let version_query = "SELECT version()";
        let version: String = sqlx::query_scalar(version_query)
            .fetch_one(&self.pool)
            .await?;

        capabilities.insert("postgres_version".to_string(), true);
        
        // Detect if running on AWS RDS
        let is_rds = version.contains("rds") || version.contains("aurora");
        capabilities.insert("is_rds".to_string(), is_rds);

        // Check for pg_stat_statements
        let has_statements = sqlx::query(
            "SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'"
        )
        .fetch_optional(&self.pool)
        .await?
        .is_some();
        
        capabilities.insert("pg_stat_statements".to_string(), has_statements);

        // Check if pg_stat_statements tracks I/O timing
        if has_statements {
            let track_io: Option<String> = sqlx::query_scalar(
                "SELECT setting FROM pg_settings WHERE name = 'pg_stat_statements.track'"
            )
            .fetch_optional(&self.pool)
            .await?;
            
            capabilities.insert(
                "pg_stat_statements_track_io".to_string(),
                track_io.as_deref() == Some("all") || track_io.as_deref() == Some("top")
            );
        }

        // Check for pg_wait_sampling (not available on RDS)
        if !is_rds {
            let has_wait_sampling = sqlx::query(
                "SELECT 1 FROM pg_extension WHERE extname = 'pg_wait_sampling'"
            )
            .fetch_optional(&self.pool)
            .await?
            .is_some();
            
            capabilities.insert("pg_wait_sampling".to_string(), has_wait_sampling);
        }

        // Check for auto_explain
        let auto_explain_enabled: Option<String> = sqlx::query_scalar(
            "SELECT setting FROM pg_settings WHERE name = 'auto_explain.log_min_duration'"
        )
        .fetch_optional(&self.pool)
        .await?;
        
        capabilities.insert(
            "auto_explain".to_string(),
            auto_explain_enabled.is_some() && auto_explain_enabled != Some("-1".to_string())
        );

        // Check for pg_stat_kcache (CPU/IO stats)
        let has_kcache = sqlx::query(
            "SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_kcache'"
        )
        .fetch_optional(&self.pool)
        .await?
        .is_some();
        
        capabilities.insert("pg_stat_kcache".to_string(), has_kcache);

        Ok(capabilities)
    }

    /// Collect comprehensive slow query metrics with rich context
    async fn collect_slow_queries(
        &mut self,
        capabilities: &HashMap<String, bool>,
    ) -> Result<Vec<SlowQueryMetric>> {
        if !capabilities.get("pg_stat_statements").unwrap_or(&false) {
            warn!("pg_stat_statements not available, skipping slow query collection");
            return Ok(vec![]);
        }

        let mut queries = vec![];
        
        // Build query based on available features
        let mut query = r#"
            WITH query_stats AS (
                SELECT 
                    s.queryid::text as query_id,
                    s.query,
                    s.userid::text,
                    s.dbid::text,
                    s.calls,
                    s.total_exec_time,
                    s.mean_exec_time,
                    s.stddev_exec_time,
                    s.min_exec_time,
                    s.max_exec_time,
                    s.rows,
                    s.shared_blks_hit,
                    s.shared_blks_read,
                    s.shared_blks_dirtied,
                    s.shared_blks_written,
                    s.local_blks_hit,
                    s.local_blks_read,
                    s.temp_blks_read,
                    s.temp_blks_written"#.to_string();

        // Add I/O timing if available
        if capabilities.get("pg_stat_statements_track_io").unwrap_or(&false) {
            query.push_str(r#",
                    s.blk_read_time,
                    s.blk_write_time"#);
        }

        // Add pg_stat_kcache metrics if available
        if capabilities.get("pg_stat_kcache").unwrap_or(&false) {
            query.push_str(r#",
                    k.user_time,
                    k.system_time,
                    k.minflts,
                    k.majflts,
                    k.nswaps,
                    k.reads,
                    k.writes,
                    k.msgsnds,
                    k.msgrcvs,
                    k.nsignals,
                    k.nvcsws,
                    k.nivcsws"#);
        }

        query.push_str(r#"
                FROM pg_stat_statements s"#);

        if capabilities.get("pg_stat_kcache").unwrap_or(&false) {
            query.push_str(r#"
                LEFT JOIN pg_stat_kcache() k ON s.queryid = k.queryid 
                    AND s.userid = k.userid AND s.dbid = k.dbid"#);
        }

        query.push_str(r#"
                WHERE s.mean_exec_time > $1
                ORDER BY s.total_exec_time DESC
                LIMIT $2
            )
            SELECT * FROM query_stats"#);

        let rows = sqlx::query(&query)
            .bind(self.config.slow_query_threshold_ms as f64)
            .bind(self.config.max_queries_per_cycle as i64)
            .fetch_all(&self.pool)
            .await?;

        for row in rows {
            let query_id: String = row.get("query_id");
            let query_text: String = row.get("query");
            
            // Normalize and fingerprint the query
            let fingerprint = self.query_fingerprints.get_or_create(&query_text);
            
            let mut metric = SlowQueryMetric {
                query_id: query_id.clone(),
                query_text: query_text.clone(),
                normalized_query: fingerprint.normalized_text.clone(),
                database_id: row.get("dbid"),
                user_id: row.get("userid"),
                execution_count: row.get::<i64, _>("calls") as u64,
                total_time_ms: row.get::<f64, _>("total_exec_time"),
                mean_time_ms: row.get::<f64, _>("mean_exec_time"),
                stddev_time_ms: row.get::<f64, _>("stddev_exec_time"),
                min_time_ms: row.get::<f64, _>("min_exec_time"),
                max_time_ms: row.get::<f64, _>("max_exec_time"),
                rows_returned: row.get::<i64, _>("rows") as u64,
                shared_blocks_hit: row.get::<i64, _>("shared_blks_hit") as u64,
                shared_blocks_read: row.get::<i64, _>("shared_blks_read") as u64,
                temp_blocks_read: row.get::<i64, _>("temp_blks_read") as u64,
                temp_blocks_written: row.get::<i64, _>("temp_blks_written") as u64,
                execution_plan: None,
                extended_metrics: None,
                tags: HashMap::new(),
            };

            // Calculate derived metrics
            let total_blocks = metric.shared_blocks_hit + metric.shared_blocks_read;
            if total_blocks > 0 {
                let cache_hit_ratio = metric.shared_blocks_hit as f64 / total_blocks as f64;
                metric.tags.insert("cache_hit_ratio".to_string(), format!("{:.2}", cache_hit_ratio));
            }

            // Add extended metrics if available
            let mut extended = ExtendedSlowQueryMetrics::default();
            
            if capabilities.get("pg_stat_statements_track_io").unwrap_or(&false) {
                extended.io_read_time_ms = row.try_get("blk_read_time").unwrap_or(0.0);
                extended.io_write_time_ms = row.try_get("blk_write_time").unwrap_or(0.0);
            }

            if capabilities.get("pg_stat_kcache").unwrap_or(&false) {
                extended.cpu_user_time_ms = row.try_get("user_time").unwrap_or(0.0);
                extended.cpu_system_time_ms = row.try_get("system_time").unwrap_or(0.0);
                extended.memory_faults = row.try_get::<i64, _>("minflts").unwrap_or(0) as u64 +
                                        row.try_get::<i64, _>("majflts").unwrap_or(0) as u64;
                extended.disk_reads = row.try_get::<i64, _>("reads").unwrap_or(0) as u64;
                extended.disk_writes = row.try_get::<i64, _>("writes").unwrap_or(0) as u64;
                extended.context_switches = row.try_get::<i64, _>("nvcsws").unwrap_or(0) as u64 +
                                           row.try_get::<i64, _>("nivcsws").unwrap_or(0) as u64;
            }

            // Calculate percentiles if we have enough data
            if metric.execution_count > 100 {
                // Estimate percentiles using Chebyshev's inequality
                let mean = metric.mean_time_ms;
                let stddev = metric.stddev_time_ms;
                
                extended.percentile_50_ms = mean;
                extended.percentile_95_ms = mean + 1.96 * stddev;
                extended.percentile_99_ms = mean + 2.58 * stddev;
                
                // Ensure percentiles are within min/max bounds
                extended.percentile_95_ms = extended.percentile_95_ms.min(metric.max_time_ms);
                extended.percentile_99_ms = extended.percentile_99_ms.min(metric.max_time_ms);
            }

            metric.extended_metrics = Some(extended);
            
            // Add query classification tags
            self.classify_query(&mut metric);
            
            queries.push(metric);
        }

        Ok(queries)
    }

    /// Classify queries for better insights
    fn classify_query(&self, metric: &mut SlowQueryMetric) {
        let query_lower = metric.query_text.to_lowercase();
        
        // Query type classification
        let query_type = if query_lower.starts_with("select") {
            "select"
        } else if query_lower.starts_with("insert") {
            "insert"
        } else if query_lower.starts_with("update") {
            "update"
        } else if query_lower.starts_with("delete") {
            "delete"
        } else if query_lower.contains("create") || query_lower.contains("alter") {
            "ddl"
        } else {
            "other"
        };
        
        metric.tags.insert("query_type".to_string(), query_type.to_string());
        
        // Complexity indicators
        if query_lower.contains("join") {
            metric.tags.insert("has_joins".to_string(), "true".to_string());
            
            let join_count = query_lower.matches("join").count();
            metric.tags.insert("join_count".to_string(), join_count.to_string());
        }
        
        if query_lower.contains("subquery") || query_lower.contains("exists") {
            metric.tags.insert("has_subquery".to_string(), "true".to_string());
        }
        
        if query_lower.contains("group by") {
            metric.tags.insert("has_aggregation".to_string(), "true".to_string());
        }
        
        if query_lower.contains("order by") {
            metric.tags.insert("has_sorting".to_string(), "true".to_string());
        }
        
        // Performance classification
        let performance_class = match metric.mean_time_ms {
            t if t < 10.0 => "fast",
            t if t < 100.0 => "normal",
            t if t < 1000.0 => "slow",
            _ => "very_slow"
        };
        
        metric.tags.insert("performance_class".to_string(), performance_class.to_string());
        
        // Resource usage classification
        if metric.temp_blocks_written > 0 {
            metric.tags.insert("uses_temp_storage".to_string(), "true".to_string());
        }
        
        let total_blocks = metric.shared_blocks_hit + metric.shared_blocks_read;
        if total_blocks > 10000 {
            metric.tags.insert("high_block_usage".to_string(), "true".to_string());
        }
    }

    /// Collect execution plans for top queries with regression detection
    async fn collect_execution_plans(&mut self, metrics: &mut UnifiedMetrics) {
        let plans_to_collect = metrics.slow_queries
            .iter()
            .take(self.config.max_plans_per_cycle)
            .filter(|q| q.mean_time_ms > self.config.plan_collection_threshold_ms)
            .collect::<Vec<_>>();

        for query_metric in plans_to_collect {
            match self.get_execution_plan(&query_metric.query_id, &query_metric.query_text).await {
                Ok(plan) => {
                    // Check for plan regression
                    let is_regression = self.plan_cache.check_regression(
                        &query_metric.query_id,
                        &plan.plan_hash
                    );
                    
                    if is_regression {
                        self.collection_stats.plan_changes_detected += 1;
                        warn!(
                            "Plan regression detected for query {}: {} -> {}",
                            query_metric.query_id,
                            self.plan_cache.get_previous_hash(&query_metric.query_id).unwrap_or_default(),
                            plan.plan_hash
                        );
                    }
                    
                    metrics.execution_plans.push(ExecutionPlanMetric {
                        query_id: query_metric.query_id.clone(),
                        plan_hash: plan.plan_hash,
                        plan_json: plan.plan_json,
                        is_regression,
                        previous_plan_hash: if is_regression {
                            self.plan_cache.get_previous_hash(&query_metric.query_id)
                        } else {
                            None
                        },
                        total_cost: plan.total_cost,
                        execution_time_ms: Some(query_metric.mean_time_ms),
                        timestamp: Utc::now(),
                    });
                }
                Err(e) => {
                    debug!("Failed to collect execution plan for query {}: {}", 
                           query_metric.query_id, e);
                }
            }
        }
    }

    /// Get execution plan for a query with timeout protection
    async fn get_execution_plan(&self, query_id: &str, query_text: &str) -> Result<QueryPlan> {
        // Set a timeout for EXPLAIN to prevent hanging
        let timeout_query = "SET LOCAL statement_timeout = '5000'"; // 5 second timeout
        sqlx::query(timeout_query).execute(&self.pool).await?;
        
        // Prepare the query for EXPLAIN (remove parameters)
        let prepared_query = self.prepare_query_for_explain(query_text)?;
        
        let explain_query = format!("EXPLAIN (FORMAT JSON, BUFFERS, ANALYZE FALSE) {}", prepared_query);
        
        let plan_json: serde_json::Value = sqlx::query_scalar(&explain_query)
            .fetch_one(&self.pool)
            .await
            .context("Failed to get execution plan")?;
        
        // Extract plan details
        let plan = self.parse_execution_plan(plan_json)?;
        
        Ok(plan)
    }

    /// Prepare a query for EXPLAIN by handling parameters
    fn prepare_query_for_explain(&self, query_text: &str) -> Result<String> {
        // Simple parameter replacement - in production, use proper SQL parser
        let mut prepared = query_text.to_string();
        
        // Replace common parameter placeholders
        for i in 1..=20 {
            prepared = prepared.replace(&format!("${}", i), "1");
        }
        
        // Add LIMIT if not present to prevent large result sets
        if !prepared.to_lowercase().contains("limit") {
            prepared.push_str(" LIMIT 1");
        }
        
        Ok(prepared)
    }

    /// Parse execution plan JSON and extract key metrics
    fn parse_execution_plan(&self, plan_json: serde_json::Value) -> Result<QueryPlan> {
        let plan_array = plan_json.as_array()
            .context("Invalid plan format")?;
        
        let plan_obj = plan_array.get(0)
            .and_then(|p| p.get("Plan"))
            .context("No plan found")?;
        
        let total_cost = plan_obj.get("Total Cost")
            .and_then(|c| c.as_f64())
            .unwrap_or(0.0);
        
        let node_count = self.count_plan_nodes(plan_obj);
        
        // Generate a stable hash of the plan structure
        let plan_hash = self.generate_plan_hash(plan_obj);
        
        Ok(QueryPlan {
            plan_json: plan_json.to_string(),
            plan_hash,
            total_cost,
            node_count,
        })
    }

    /// Count nodes in execution plan recursively
    fn count_plan_nodes(&self, plan: &serde_json::Value) -> u32 {
        let mut count = 1;
        
        if let Some(plans) = plan.get("Plans").and_then(|p| p.as_array()) {
            for child_plan in plans {
                count += self.count_plan_nodes(child_plan);
            }
        }
        
        count
    }

    /// Generate stable hash for plan structure
    fn generate_plan_hash(&self, plan: &serde_json::Value) -> String {
        use sha2::{Sha256, Digest};
        
        let mut hasher = Sha256::new();
        
        // Include node type
        if let Some(node_type) = plan.get("Node Type").and_then(|n| n.as_str()) {
            hasher.update(node_type.as_bytes());
        }
        
        // Include join type if present
        if let Some(join_type) = plan.get("Join Type").and_then(|j| j.as_str()) {
            hasher.update(join_type.as_bytes());
        }
        
        // Include index name if present
        if let Some(index_name) = plan.get("Index Name").and_then(|i| i.as_str()) {
            hasher.update(index_name.as_bytes());
        }
        
        // Recursively hash child plans
        if let Some(plans) = plan.get("Plans").and_then(|p| p.as_array()) {
            for child_plan in plans {
                let child_hash = self.generate_plan_hash(child_plan);
                hasher.update(child_hash.as_bytes());
            }
        }
        
        format!("{:x}", hasher.finalize())
    }

    /// Collect wait events with sampling
    async fn collect_wait_events(
        &self,
        capabilities: &HashMap<String, bool>,
    ) -> Result<Vec<WaitEventMetric>> {
        // Use pg_wait_sampling if available, otherwise fall back to sampling pg_stat_activity
        if capabilities.get("pg_wait_sampling").unwrap_or(&false) {
            self.collect_wait_events_from_extension().await
        } else {
            self.collect_wait_events_from_activity().await
        }
    }

    /// Collect wait events from pg_wait_sampling extension
    async fn collect_wait_events_from_extension(&self) -> Result<Vec<WaitEventMetric>> {
        let query = r#"
            SELECT 
                event_type,
                event,
                count,
                sum_time_ms
            FROM pg_wait_sampling_profile
            WHERE sum_time_ms > 0
            ORDER BY sum_time_ms DESC
            LIMIT 100
        "#;
        
        let rows = sqlx::query(query)
            .fetch_all(&self.pool)
            .await?;
        
        let mut events = vec![];
        
        for row in rows {
            events.push(WaitEventMetric {
                wait_event_type: row.get("event_type"),
                wait_event: row.get("event"),
                count: row.get::<i64, _>("count") as u64,
                total_time_ms: row.get::<f64, _>("sum_time_ms"),
                queries_affected: vec![], // Would need to join with pg_stat_activity
            });
        }
        
        Ok(events)
    }

    /// Fallback: Sample wait events from pg_stat_activity
    async fn collect_wait_events_from_activity(&self) -> Result<Vec<WaitEventMetric>> {
        // Take multiple samples to get better coverage
        let mut wait_samples: HashMap<(String, String), WaitEventAccumulator> = HashMap::new();
        
        for _ in 0..5 {
            let query = r#"
                SELECT 
                    wait_event_type,
                    wait_event,
                    query,
                    count(*) as count
                FROM pg_stat_activity
                WHERE wait_event IS NOT NULL
                    AND state = 'active'
                GROUP BY wait_event_type, wait_event, query
            "#;
            
            let rows = sqlx::query(query)
                .fetch_all(&self.pool)
                .await?;
            
            for row in rows {
                let event_type: String = row.get("wait_event_type");
                let event: String = row.get("wait_event");
                let query: Option<String> = row.try_get("query").ok();
                let count: i64 = row.get("count");
                
                let key = (event_type.clone(), event.clone());
                let accumulator = wait_samples.entry(key).or_insert_with(|| {
                    WaitEventAccumulator {
                        event_type,
                        event,
                        sample_count: 0,
                        queries: vec![],
                    }
                });
                
                accumulator.sample_count += count;
                if let Some(q) = query {
                    if !accumulator.queries.contains(&q) {
                        accumulator.queries.push(q);
                    }
                }
            }
            
            // Small delay between samples
            tokio::time::sleep(Duration::from_millis(200)).await;
        }
        
        // Convert accumulated samples to metrics
        let mut events: Vec<WaitEventMetric> = wait_samples
            .into_iter()
            .map(|(_, acc)| WaitEventMetric {
                wait_event_type: acc.event_type,
                wait_event: acc.event,
                count: acc.sample_count as u64,
                total_time_ms: acc.sample_count as f64 * 200.0, // Rough estimate
                queries_affected: acc.queries,
            })
            .collect();
        
        // Sort by estimated impact
        events.sort_by(|a, b| b.total_time_ms.partial_cmp(&a.total_time_ms).unwrap());
        
        Ok(events)
    }

    /// Collect comprehensive database metrics
    async fn collect_database_metrics(&self) -> Result<Vec<DatabaseMetric>> {
        let query = r#"
            WITH db_stats AS (
                SELECT 
                    d.datname,
                    d.datid::text,
                    pg_database_size(d.datname) as size_bytes,
                    s.numbackends,
                    s.xact_commit,
                    s.xact_rollback,
                    s.blks_read,
                    s.blks_hit,
                    s.tup_returned,
                    s.tup_fetched,
                    s.tup_inserted,
                    s.tup_updated,
                    s.tup_deleted,
                    s.conflicts,
                    s.temp_files,
                    s.temp_bytes,
                    s.deadlocks,
                    s.checksum_failures,
                    s.blk_read_time,
                    s.blk_write_time,
                    s.stats_reset
                FROM pg_database d
                JOIN pg_stat_database s ON d.datid = s.datid
                WHERE d.datname NOT IN ('template0', 'template1')
            ),
            connections AS (
                SELECT 
                    datname,
                    state,
                    count(*) as count
                FROM pg_stat_activity
                GROUP BY datname, state
            )
            SELECT 
                ds.*,
                COALESCE(c_active.count, 0) as active_connections,
                COALESCE(c_idle.count, 0) as idle_connections,
                COALESCE(c_idle_tx.count, 0) as idle_in_transaction_connections
            FROM db_stats ds
            LEFT JOIN connections c_active 
                ON ds.datname = c_active.datname AND c_active.state = 'active'
            LEFT JOIN connections c_idle 
                ON ds.datname = c_idle.datname AND c_idle.state = 'idle'
            LEFT JOIN connections c_idle_tx 
                ON ds.datname = c_idle_tx.datname AND c_idle_tx.state = 'idle in transaction'
        "#;
        
        let rows = sqlx::query(query)
            .fetch_all(&self.pool)
            .await?;
        
        let mut metrics = vec![];
        
        for row in rows {
            let mut metric = DatabaseMetric {
                database_name: row.get("datname"),
                database_id: row.get("datid"),
                size_bytes: row.get::<i64, _>("size_bytes") as u64,
                connection_count: row.get::<i32, _>("numbackends") as u32,
                active_connections: row.get::<i64, _>("active_connections") as u32,
                idle_connections: row.get::<i64, _>("idle_connections") as u32,
                idle_in_transaction: row.get::<i64, _>("idle_in_transaction_connections") as u32,
                transactions_committed: row.get::<i64, _>("xact_commit") as u64,
                transactions_rolled_back: row.get::<i64, _>("xact_rollback") as u64,
                blocks_read: row.get::<i64, _>("blks_read") as u64,
                blocks_hit: row.get::<i64, _>("blks_hit") as u64,
                tuples_returned: row.get::<i64, _>("tup_returned") as u64,
                tuples_fetched: row.get::<i64, _>("tup_fetched") as u64,
                tuples_inserted: row.get::<i64, _>("tup_inserted") as u64,
                tuples_updated: row.get::<i64, _>("tup_updated") as u64,
                tuples_deleted: row.get::<i64, _>("tup_deleted") as u64,
                conflicts: row.get::<i64, _>("conflicts") as u64,
                deadlocks: row.get::<i64, _>("deadlocks") as u64,
                checksum_failures: row.try_get::<i64, _>("checksum_failures").unwrap_or(0) as u64,
                temp_files: row.get::<i64, _>("temp_files") as u64,
                temp_bytes: row.get::<i64, _>("temp_bytes") as u64,
                blk_read_time: row.get("blk_read_time"),
                blk_write_time: row.get("blk_write_time"),
                cache_hit_ratio: 0.0,
                transaction_rollback_ratio: 0.0,
            };
            
            // Calculate derived metrics
            let total_blocks = metric.blocks_hit + metric.blocks_read;
            if total_blocks > 0 {
                metric.cache_hit_ratio = metric.blocks_hit as f64 / total_blocks as f64;
            }
            
            let total_transactions = metric.transactions_committed + metric.transactions_rolled_back;
            if total_transactions > 0 {
                metric.transaction_rollback_ratio = 
                    metric.transactions_rolled_back as f64 / total_transactions as f64;
            }
            
            metrics.push(metric);
        }
        
        Ok(metrics)
    }

    /// Collect lock information
    async fn collect_locks(&self) -> Result<Vec<LockMetric>> {
        let query = r#"
            WITH lock_info AS (
                SELECT 
                    l.locktype,
                    l.database,
                    l.relation::regclass::text as relation,
                    l.page,
                    l.tuple,
                    l.virtualxid,
                    l.transactionid::text,
                    l.classid::regclass::text as classid,
                    l.objid,
                    l.objsubid,
                    l.virtualtransaction,
                    l.pid,
                    l.mode,
                    l.granted,
                    l.fastpath,
                    a.query,
                    a.state,
                    a.wait_event_type,
                    a.wait_event,
                    a.application_name,
                    a.client_addr::text,
                    now() - a.query_start as query_duration,
                    now() - a.xact_start as transaction_duration
                FROM pg_locks l
                JOIN pg_stat_activity a ON l.pid = a.pid
                WHERE l.database = (SELECT oid FROM pg_database WHERE datname = current_database())
            )
            SELECT * FROM lock_info
            WHERE granted = false OR mode IN ('ExclusiveLock', 'AccessExclusiveLock')
            ORDER BY granted, query_duration DESC NULLS LAST
        "#;
        
        let rows = sqlx::query(query)
            .fetch_all(&self.pool)
            .await?;
        
        let mut locks = vec![];
        
        for row in rows {
            locks.push(LockMetric {
                lock_type: row.get("locktype"),
                database_id: row.try_get("database").ok(),
                relation: row.try_get("relation").ok(),
                mode: row.get("mode"),
                granted: row.get("granted"),
                pid: row.get("pid"),
                query: row.try_get("query").ok(),
                wait_duration_ms: if !row.get::<bool, _>("granted") {
                    row.try_get::<i64, _>("query_duration")
                        .map(|d| d as f64 / 1000.0)
                        .ok()
                } else {
                    None
                },
                blocking_pids: vec![], // Would need additional query to find blockers
            });
        }
        
        Ok(locks)
    }

    /// Collect blocking session information
    async fn collect_blocking_sessions(&self) -> Result<Vec<BlockingSessionMetric>> {
        let query = r#"
            WITH blocking_info AS (
                SELECT 
                    blocked_locks.pid AS blocked_pid,
                    blocked_activity.usename AS blocked_user,
                    blocked_activity.application_name AS blocked_application,
                    blocked_activity.client_addr::text AS blocked_client,
                    blocked_activity.query AS blocked_query,
                    blocked_activity.state AS blocked_state,
                    now() - blocked_activity.query_start AS blocked_duration,
                    blocking_locks.pid AS blocking_pid,
                    blocking_activity.usename AS blocking_user,
                    blocking_activity.application_name AS blocking_application,
                    blocking_activity.client_addr::text AS blocking_client,
                    blocking_activity.query AS blocking_query,
                    blocking_activity.state AS blocking_state,
                    now() - blocking_activity.query_start AS blocking_duration
                FROM pg_locks blocked_locks
                JOIN pg_stat_activity blocked_activity ON blocked_locks.pid = blocked_activity.pid
                JOIN pg_locks blocking_locks 
                    ON blocking_locks.locktype = blocked_locks.locktype
                    AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
                    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
                    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
                    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
                    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
                    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
                    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
                    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
                    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
                    AND blocking_locks.pid != blocked_locks.pid
                JOIN pg_stat_activity blocking_activity ON blocking_locks.pid = blocking_activity.pid
                WHERE NOT blocked_locks.granted
            )
            SELECT * FROM blocking_info
            ORDER BY blocked_duration DESC
        "#;
        
        let rows = sqlx::query(query)
            .fetch_all(&self.pool)
            .await?;
        
        let mut sessions = vec![];
        
        for row in rows {
            sessions.push(BlockingSessionMetric {
                blocked_pid: row.get("blocked_pid"),
                blocked_query: row.get("blocked_query"),
                blocked_duration_ms: row.get::<i64, _>("blocked_duration") as f64 / 1000.0,
                blocking_pid: row.get("blocking_pid"),
                blocking_query: row.get("blocking_query"),
                blocking_duration_ms: row.get::<i64, _>("blocking_duration") as f64 / 1000.0,
            });
        }
        
        Ok(sessions)
    }

    /// Collect table-level metrics
    async fn collect_table_metrics(&self) -> Result<Vec<TableMetric>> {
        let query = r#"
            WITH table_stats AS (
                SELECT 
                    schemaname,
                    tablename,
                    pg_relation_size(schemaname||'.'||tablename) as size_bytes,
                    n_tup_ins,
                    n_tup_upd,
                    n_tup_del,
                    n_tup_hot_upd,
                    n_live_tup,
                    n_dead_tup,
                    n_mod_since_analyze,
                    last_vacuum,
                    last_autovacuum,
                    last_analyze,
                    last_autoanalyze,
                    vacuum_count,
                    autovacuum_count,
                    analyze_count,
                    autoanalyze_count,
                    seq_scan,
                    seq_tup_read,
                    idx_scan,
                    idx_tup_fetch
                FROM pg_stat_user_tables
            ),
            io_stats AS (
                SELECT 
                    schemaname,
                    tablename,
                    heap_blks_read,
                    heap_blks_hit,
                    idx_blks_read,
                    idx_blks_hit,
                    toast_blks_read,
                    toast_blks_hit
                FROM pg_statio_user_tables
            )
            SELECT 
                t.*,
                i.heap_blks_read,
                i.heap_blks_hit,
                i.idx_blks_read,
                i.idx_blks_hit
            FROM table_stats t
            JOIN io_stats i USING (schemaname, tablename)
            WHERE t.n_live_tup > 1000  -- Focus on non-trivial tables
            ORDER BY t.size_bytes DESC
            LIMIT 100
        "#;
        
        let rows = sqlx::query(query)
            .fetch_all(&self.pool)
            .await?;
        
        let mut tables = vec![];
        
        for row in rows {
            let mut metric = TableMetric {
                schema_name: row.get("schemaname"),
                table_name: row.get("tablename"),
                size_bytes: row.get::<i64, _>("size_bytes") as u64,
                row_count: row.get::<i64, _>("n_live_tup") as u64,
                dead_row_count: row.get::<i64, _>("n_dead_tup") as u64,
                rows_inserted: row.get::<i64, _>("n_tup_ins") as u64,
                rows_updated: row.get::<i64, _>("n_tup_upd") as u64,
                rows_deleted: row.get::<i64, _>("n_tup_del") as u64,
                rows_hot_updated: row.get::<i64, _>("n_tup_hot_upd") as u64,
                sequential_scans: row.get::<i64, _>("seq_scan") as u64,
                sequential_reads: row.get::<i64, _>("seq_tup_read") as u64,
                index_scans: row.get::<i64, _>("idx_scan") as u64,
                index_reads: row.get::<i64, _>("idx_tup_fetch") as u64,
                heap_blocks_read: row.get::<i64, _>("heap_blks_read") as u64,
                heap_blocks_hit: row.get::<i64, _>("heap_blks_hit") as u64,
                last_vacuum: row.try_get("last_vacuum").ok(),
                last_analyze: row.try_get("last_analyze").ok(),
                vacuum_count: row.get::<i64, _>("vacuum_count") as u64,
                analyze_count: row.get::<i64, _>("analyze_count") as u64,
                bloat_ratio: 0.0,
                cache_hit_ratio: 0.0,
            };
            
            // Calculate bloat ratio
            if metric.row_count > 0 {
                metric.bloat_ratio = metric.dead_row_count as f64 / 
                    (metric.row_count + metric.dead_row_count) as f64;
            }
            
            // Calculate cache hit ratio
            let total_blocks = metric.heap_blocks_read + metric.heap_blocks_hit;
            if total_blocks > 0 {
                metric.cache_hit_ratio = metric.heap_blocks_hit as f64 / total_blocks as f64;
            }
            
            tables.push(metric);
        }
        
        Ok(tables)
    }

    /// Collect index metrics
    async fn collect_index_metrics(&self) -> Result<Vec<IndexMetric>> {
        let query = r#"
            WITH index_stats AS (
                SELECT 
                    schemaname,
                    tablename,
                    indexname,
                    idx_scan,
                    idx_tup_read,
                    idx_tup_fetch,
                    pg_relation_size(indexrelid) as size_bytes
                FROM pg_stat_user_indexes
            ),
            index_io AS (
                SELECT 
                    schemaname,
                    tablename,
                    indexname,
                    idx_blks_read,
                    idx_blks_hit
                FROM pg_statio_user_indexes
            )
            SELECT 
                s.*,
                i.idx_blks_read,
                i.idx_blks_hit,
                pg_get_indexdef(s.indexrelid) as index_definition
            FROM index_stats s
            JOIN index_io i USING (schemaname, tablename, indexname)
            ORDER BY s.idx_scan DESC
            LIMIT 100
        "#;
        
        let rows = sqlx::query(query)
            .fetch_all(&self.pool)
            .await?;
        
        let mut indexes = vec![];
        
        for row in rows {
            let idx_scan = row.get::<i64, _>("idx_scan") as u64;
            let size_bytes = row.get::<i64, _>("size_bytes") as u64;
            
            indexes.push(IndexMetric {
                schema_name: row.get("schemaname"),
                table_name: row.get("tablename"),
                index_name: row.get("indexname"),
                size_bytes,
                scans: idx_scan,
                tuples_read: row.get::<i64, _>("idx_tup_read") as u64,
                tuples_fetched: row.get::<i64, _>("idx_tup_fetch") as u64,
                blocks_read: row.get::<i64, _>("idx_blks_read") as u64,
                blocks_hit: row.get::<i64, _>("idx_blks_hit") as u64,
                is_unique: row.get::<String, _>("index_definition").contains("UNIQUE"),
                is_primary: row.get::<String, _>("index_definition").contains("PRIMARY KEY"),
                is_partial: row.get::<String, _>("index_definition").contains("WHERE"),
                is_unused: idx_scan == 0 && size_bytes > 1024 * 1024, // Unused if no scans and > 1MB
            });
        }
        
        Ok(indexes)
    }

    /// Get collector self-metrics
    fn get_collector_metrics(&self) -> serde_json::Value {
        serde_json::json!({
            "collection_duration_ms": self.collection_stats.last_collection_duration.as_millis(),
            "queries_collected": self.collection_stats.queries_collected,
            "errors_encountered": self.collection_stats.errors_encountered,
            "plan_changes_detected": self.collection_stats.plan_changes_detected,
            "collector_version": env!("CARGO_PKG_VERSION"),
        })
    }
}

// Helper structs

struct QueryPlan {
    plan_json: String,
    plan_hash: String,
    total_cost: f64,
    node_count: u32,
}

struct WaitEventAccumulator {
    event_type: String,
    event: String,
    sample_count: i64,
    queries: Vec<String>,
}

impl PlanCache {
    fn new(capacity: usize) -> Self {
        Self {
            cache: lru::LruCache::new(capacity),
        }
    }
    
    fn check_regression(&mut self, query_id: &str, new_hash: &str) -> bool {
        if let Some(existing) = self.cache.get(query_id) {
            if existing.hash != new_hash {
                // Plan changed - update cache and return true
                self.cache.put(query_id.to_string(), PlanFingerprint {
                    hash: new_hash.to_string(),
                    cost: 0.0, // Would be updated from actual plan
                    timestamp: Utc::now(),
                    node_count: 0,
                });
                return true;
            }
        } else {
            // First time seeing this query
            self.cache.put(query_id.to_string(), PlanFingerprint {
                hash: new_hash.to_string(),
                cost: 0.0,
                timestamp: Utc::now(),
                node_count: 0,
            });
        }
        false
    }
    
    fn get_previous_hash(&self, query_id: &str) -> Option<String> {
        self.cache.peek(query_id).map(|f| f.hash.clone())
    }
}

impl QueryFingerprintCache {
    fn new(capacity: usize) -> Self {
        Self {
            cache: lru::LruCache::new(capacity),
        }
    }
    
    fn get_or_create(&mut self, query_text: &str) -> QueryFingerprint {
        // Simple normalization - in production use pg_query or similar
        let normalized = query_text
            .to_lowercase()
            .split_whitespace()
            .collect::<Vec<_>>()
            .join(" ");
        
        if let Some(existing) = self.cache.get(&normalized) {
            existing.clone()
        } else {
            let fingerprint = QueryFingerprint {
                normalized_text: normalized.clone(),
                parameter_count: query_text.matches('$').count() as u32,
                tables: vec![], // Would parse from query
            };
            
            self.cache.put(normalized, fingerprint.clone());
            fingerprint
        }
    }
}