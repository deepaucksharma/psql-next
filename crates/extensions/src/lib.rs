use std::collections::HashMap;
use std::time::Duration;
use anyhow::Result;
use sqlx::{PgConnection, Row};
use postgres_collector_core::{CollectorError, ExtensionInfo};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExtensionConfig {
    pub pg_stat_statements: Option<PgStatStatementsConfig>,
    pub pg_stat_monitor: Option<PgStatMonitorConfig>,
    pub pg_wait_sampling: Option<PgWaitSamplingConfig>,
}

impl Default for ExtensionConfig {
    fn default() -> Self {
        Self {
            pg_stat_statements: None,
            pg_stat_monitor: None,
            pg_wait_sampling: None,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgStatStatementsConfig {
    pub version: String,
    pub track: String,
    pub max: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgStatMonitorConfig {
    pub version: String,
    pub pgsm_normalized_query: bool,
    pub pgsm_enable_query_plan: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgWaitSamplingConfig {
    pub version: String,
    pub sample_period: Duration,
}

pub struct ExtensionManager {
    extensions: HashMap<String, ExtensionInfo>,
    compatibility_matrix: CompatibilityMatrix,
}

impl ExtensionManager {
    pub fn new() -> Self {
        Self {
            extensions: HashMap::new(),
            compatibility_matrix: CompatibilityMatrix::default(),
        }
    }
    
    pub async fn detect_and_configure(
        &mut self,
        conn: &mut PgConnection,
    ) -> Result<ExtensionConfig, CollectorError> {
        // Detect installed extensions
        let installed = self.detect_installed_extensions(conn).await?;
        
        // Check compatibility
        let mut config = ExtensionConfig::default();
        
        // pg_stat_statements (required for OHI compatibility)
        if let Some(info) = installed.get("pg_stat_statements") {
            config.pg_stat_statements = Some(PgStatStatementsConfig {
                version: info.version.clone(),
                track: "all".to_string(),
                max: 10000,
            });
        }
        
        // pg_stat_monitor (enhanced individual queries)
        if let Some(info) = installed.get("pg_stat_monitor") {
            if self.compatibility_matrix.is_compatible("pg_stat_monitor", &info.version) {
                config.pg_stat_monitor = Some(PgStatMonitorConfig {
                    version: info.version.clone(),
                    pgsm_normalized_query: true,
                    pgsm_enable_query_plan: true,
                });
            }
        }
        
        // pg_wait_sampling (wait events)
        if let Some(info) = installed.get("pg_wait_sampling") {
            config.pg_wait_sampling = Some(PgWaitSamplingConfig {
                version: info.version.clone(),
                sample_period: Duration::from_millis(10),
            });
        }
        
        self.extensions = installed;
        Ok(config)
    }
    
    pub async fn detect_installed_extensions(
        &self,
        conn: &mut PgConnection,
    ) -> Result<HashMap<String, ExtensionInfo>, CollectorError> {
        let rows = sqlx::query("SELECT extname, extversion FROM pg_extension")
        .fetch_all(conn)
        .await?;
        
        let mut extensions = HashMap::new();
        for row in rows {
            let extname: String = row.get("extname");
            let extversion: String = row.get("extversion");
            extensions.insert(
                extname.clone(),
                ExtensionInfo {
                    name: extname,
                    version: extversion,
                    enabled: true,
                },
            );
        }
        
        Ok(extensions)
    }
}

/// OHI Validation helpers
pub struct OHIValidations;

impl OHIValidations {
    pub fn check_slow_query_metrics_fetch_eligibility(
        extensions: &HashMap<String, ExtensionInfo>,
    ) -> bool {
        extensions.get("pg_stat_statements")
            .map(|e| e.enabled)
            .unwrap_or(false)
    }
    
    pub fn check_wait_event_metrics_fetch_eligibility(
        extensions: &HashMap<String, ExtensionInfo>,
    ) -> bool {
        extensions.get("pg_stat_statements")
            .map(|e| e.enabled)
            .unwrap_or(false)
            && extensions.get("pg_wait_sampling")
                .map(|e| e.enabled)
                .unwrap_or(false)
    }
    
    pub fn check_blocking_session_metrics_fetch_eligibility(
        extensions: &HashMap<String, ExtensionInfo>,
        version: u64,
    ) -> bool {
        // Version 12 and 13 don't require pg_stat_statements
        if version == 12 || version == 13 {
            return true;
        }
        extensions.get("pg_stat_statements")
            .map(|e| e.enabled)
            .unwrap_or(false)
    }
    
    pub fn check_individual_query_metrics_fetch_eligibility(
        extensions: &HashMap<String, ExtensionInfo>,
    ) -> bool {
        extensions.get("pg_stat_monitor")
            .map(|e| e.enabled)
            .unwrap_or(false)
    }
    
    pub fn check_postgres_version_support_for_query_monitoring(version: u64) -> bool {
        version >= 12
    }
}

/// Extension compatibility matrix
#[derive(Default)]
pub struct CompatibilityMatrix {
    rules: HashMap<String, Vec<VersionRule>>,
}

struct VersionRule {
    min_version: Option<String>,
    max_version: Option<String>,
}

impl CompatibilityMatrix {
    pub fn is_compatible(&self, extension: &str, version: &str) -> bool {
        // For now, assume all versions are compatible
        // In a real implementation, this would check version compatibility
        true
    }
}

/// Active Session History Sampler
use tokio::sync::RwLock;
use std::sync::Arc;
use std::collections::VecDeque;
use postgres_collector_core::ASHSample;

pub struct ActiveSessionSampler {
    sample_interval: Duration,
    retention_period: Duration,
    samples: Arc<RwLock<VecDeque<ASHSample>>>,
    max_samples: usize,
    max_memory_mb: usize,
}

impl ActiveSessionSampler {
    pub fn new(sample_interval: Duration, retention_period: Duration) -> Self {
        // Calculate max samples based on retention and interval
        let max_samples = (retention_period.as_secs() / sample_interval.as_secs()) as usize;
        
        Self {
            sample_interval,
            retention_period,
            samples: Arc::new(RwLock::new(VecDeque::with_capacity(max_samples))),
            max_samples,
            max_memory_mb: 100, // Default 100MB limit
        }
    }
    
    pub fn with_memory_limit(mut self, limit_mb: usize) -> Self {
        self.max_memory_mb = limit_mb;
        self
    }
    
    pub async fn start_sampling(&self, conn_pool: sqlx::PgPool) {
        let samples = self.samples.clone();
        let interval = self.sample_interval;
        let retention = self.retention_period;
        let max_samples = self.max_samples;
        let max_memory_mb = self.max_memory_mb;
        
        tokio::spawn(async move {
            let mut interval_timer = tokio::time::interval(interval);
            
            loop {
                interval_timer.tick().await;
                
                if let Ok(mut conn) = conn_pool.acquire().await {
                    if let Ok(current_samples) = Self::capture_active_sessions(&mut conn).await {
                        let mut samples_guard = samples.write().await;
                        
                        // Check memory usage before adding samples
                        let estimated_memory_mb = Self::estimate_memory_usage(&samples_guard);
                        if estimated_memory_mb > max_memory_mb {
                            tracing::warn!(
                                "ASH memory limit exceeded ({} MB > {} MB), removing old samples",
                                estimated_memory_mb,
                                max_memory_mb
                            );
                            // Remove 10% of oldest samples
                            let to_remove = samples_guard.len() / 10;
                            for _ in 0..to_remove {
                                samples_guard.pop_front();
                            }
                        }
                        
                        for sample in current_samples {
                            samples_guard.push_back(sample);
                            
                            // Enforce max samples limit
                            if samples_guard.len() > max_samples {
                                samples_guard.pop_front();
                            }
                        }
                        
                        // Maintain retention window
                        let cutoff = chrono::Utc::now() - chrono::Duration::from_std(retention).unwrap();
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
    
    async fn capture_active_sessions(conn: &mut PgConnection) -> Result<Vec<ASHSample>, CollectorError> {
        let query = r#"
            SELECT
                pid,
                usename,
                datname,
                query_id as query_id,
                state,
                wait_event_type,
                wait_event,
                query,
                backend_type,
                $1::timestamptz as sample_time
            FROM pg_stat_activity
            WHERE state != 'idle'
                AND pid != pg_backend_pid()
        "#;
        
        let now = chrono::Utc::now();
        let rows = sqlx::query(query)
            .bind(now)
            .fetch_all(conn)
            .await?;
        
        let mut samples = Vec::new();
        for row in rows {
            samples.push(ASHSample {
                sample_time: now,
                pid: row.get("pid"),
                usename: row.get("usename"),
                datname: row.get("datname"),
                query_id: row.get("query_id"),
                state: row.get("state"),
                wait_event_type: row.get("wait_event_type"),
                wait_event: row.get("wait_event"),
                query: row.get("query"),
                backend_type: row.get("backend_type"),
            });
        }
        
        Ok(samples)
    }
    
    pub async fn get_recent_samples(&self) -> Vec<ASHSample> {
        self.samples.read().await.iter().cloned().collect()
    }
    
    /// Estimate memory usage of samples in MB
    fn estimate_memory_usage(samples: &VecDeque<ASHSample>) -> usize {
        // Estimate size per sample:
        // - Fixed fields: ~200 bytes
        // - String fields (query text): avg 500 bytes
        // - Total per sample: ~700 bytes
        const BYTES_PER_SAMPLE: usize = 700;
        
        let total_bytes = samples.len() * BYTES_PER_SAMPLE;
        total_bytes / (1024 * 1024) // Convert to MB
    }
    
    /// Get current memory usage stats
    pub async fn get_memory_stats(&self) -> (usize, usize, usize) {
        let samples = self.samples.read().await;
        let count = samples.len();
        let memory_mb = Self::estimate_memory_usage(&samples);
        (count, memory_mb, self.max_memory_mb)
    }
    
    /// Clear all samples (for emergency memory recovery)
    pub async fn clear_samples(&self) {
        let mut samples = self.samples.write().await;
        samples.clear();
        tracing::info!("Cleared all ASH samples");
    }
}