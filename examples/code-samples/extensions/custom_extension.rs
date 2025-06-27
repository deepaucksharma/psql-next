// Example: Custom PostgreSQL Extension Integration
// This demonstrates how to integrate a custom PostgreSQL extension

use async_trait::async_trait;
use postgres_collector_core::{CollectorError, CommonParameters};
use serde::{Deserialize, Serialize};
use sqlx::{PgConnection, Row};
use std::collections::HashMap;
use tracing::{info, warn};

/// Custom extension for monitoring PostgreSQL connection pooling
/// This example shows integration with pgbouncer_stats or similar
pub struct CustomPoolExtension {
    config: PoolExtensionConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolExtensionConfig {
    pub enabled: bool,
    pub collection_interval_secs: u64,
    pub track_client_connections: bool,
    pub track_server_connections: bool,
    pub alert_thresholds: PoolAlertThresholds,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolAlertThresholds {
    pub max_client_connections: i32,
    pub max_server_connections: i32,
    pub min_available_connections: i32,
    pub max_wait_time_ms: i32,
}

impl Default for PoolExtensionConfig {
    fn default() -> Self {
        Self {
            enabled: true,
            collection_interval_secs: 30,
            track_client_connections: true,
            track_server_connections: true,
            alert_thresholds: PoolAlertThresholds {
                max_client_connections: 1000,
                max_server_connections: 100,
                min_available_connections: 5,
                max_wait_time_ms: 5000,
            },
        }
    }
}

/// Metrics collected from the custom pool extension
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolMetrics {
    pub timestamp: String,
    pub database: String,
    pub pools: Vec<PoolStats>,
    pub global_stats: GlobalPoolStats,
    pub alerts: Vec<PoolAlert>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolStats {
    pub pool_name: String,
    pub database: String,
    pub user: String,
    pub client_connections: PoolConnectionStats,
    pub server_connections: PoolConnectionStats,
    pub query_stats: QueryStats,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolConnectionStats {
    pub active: i32,
    pub waiting: i32,
    pub total: i32,
    pub max_connections: i32,
    pub avg_wait_time_ms: f64,
    pub max_wait_time_ms: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QueryStats {
    pub total_queries: i64,
    pub queries_per_second: f64,
    pub avg_query_time_ms: f64,
    pub total_query_time_ms: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GlobalPoolStats {
    pub total_client_connections: i32,
    pub total_server_connections: i32,
    pub total_pools: i32,
    pub total_databases: i32,
    pub memory_usage_mb: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolAlert {
    pub level: AlertLevel,
    pub pool_name: String,
    pub message: String,
    pub value: f64,
    pub threshold: f64,
    pub timestamp: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum AlertLevel {
    Warning,
    Critical,
}

impl CustomPoolExtension {
    pub fn new(config: PoolExtensionConfig) -> Self {
        Self { config }
    }

    /// Check if the extension is available in the database
    pub async fn is_available(&self, conn: &mut PgConnection) -> Result<bool, CollectorError> {
        // Check for pgbouncer_stats or similar extension
        let result = sqlx::query(
            "SELECT 1 FROM information_schema.tables 
             WHERE table_schema = 'public' 
             AND table_name = 'pgbouncer_stats'"
        )
        .fetch_optional(conn)
        .await?;

        Ok(result.is_some())
    }

    /// Collect pool metrics from the database
    pub async fn collect_metrics(
        &self,
        conn: &mut PgConnection,
        _params: &CommonParameters,
    ) -> Result<PoolMetrics, CollectorError> {
        if !self.config.enabled {
            return Err(CollectorError::General(anyhow::anyhow!(
                "Pool extension is disabled"
            )));
        }

        info!("Collecting custom pool metrics");

        let pools = self.collect_pool_stats(conn).await?;
        let global_stats = self.collect_global_stats(conn).await?;
        let alerts = self.generate_alerts(&pools, &global_stats);

        Ok(PoolMetrics {
            timestamp: chrono::Utc::now().to_rfc3339(),
            database: "pgbouncer".to_string(), // or extract from params
            pools,
            global_stats,
            alerts,
        })
    }

    async fn collect_pool_stats(&self, conn: &mut PgConnection) -> Result<Vec<PoolStats>, CollectorError> {
        // Example query for pgbouncer SHOW POOLS command results
        let rows = sqlx::query(
            r#"
            SELECT 
                pool_name,
                database,
                "user",
                cl_active,
                cl_waiting,
                cl_total,
                sv_active,
                sv_waiting,
                sv_total,
                maxwait,
                maxwait_us,
                pool_mode
            FROM pgbouncer_pools_stats 
            WHERE database != 'pgbouncer'
            ORDER BY pool_name
            "#
        )
        .fetch_all(conn)
        .await?;

        let mut pool_stats = Vec::new();

        for row in rows {
            let pool_name: String = row.get("pool_name");
            let database: String = row.get("database");
            let user: String = row.get("user");
            
            // Client connection stats
            let cl_active: i32 = row.get("cl_active");
            let cl_waiting: i32 = row.get("cl_waiting");
            let cl_total: i32 = row.get("cl_total");
            
            // Server connection stats
            let sv_active: i32 = row.get("sv_active");
            let sv_waiting: i32 = row.get("sv_waiting");
            let sv_total: i32 = row.get("sv_total");
            
            // Wait time stats
            let maxwait: i32 = row.get("maxwait");
            let maxwait_us: i64 = row.get("maxwait_us");

            // Get query stats for this pool
            let query_stats = self.collect_query_stats_for_pool(conn, &pool_name).await?;

            pool_stats.push(PoolStats {
                pool_name,
                database,
                user,
                client_connections: PoolConnectionStats {
                    active: cl_active,
                    waiting: cl_waiting,
                    total: cl_total,
                    max_connections: 100, // From config or additional query
                    avg_wait_time_ms: (maxwait_us as f64) / 1000.0,
                    max_wait_time_ms: maxwait,
                },
                server_connections: PoolConnectionStats {
                    active: sv_active,
                    waiting: sv_waiting,
                    total: sv_total,
                    max_connections: 25, // From config
                    avg_wait_time_ms: 0.0, // Server-side wait time
                    max_wait_time_ms: 0,
                },
                query_stats,
            });
        }

        Ok(pool_stats)
    }

    async fn collect_query_stats_for_pool(
        &self,
        conn: &mut PgConnection,
        pool_name: &str,
    ) -> Result<QueryStats, CollectorError> {
        // Example query for pool-specific statistics
        let row = sqlx::query(
            r#"
            SELECT 
                total_xact_count,
                total_query_count,
                total_received,
                total_sent,
                total_xact_time,
                total_query_time,
                avg_xact_time,
                avg_query_time
            FROM pgbouncer_stats 
            WHERE pool_name = $1
            "#
        )
        .bind(pool_name)
        .fetch_optional(conn)
        .await?;

        if let Some(row) = row {
            let total_query_count: i64 = row.get("total_query_count");
            let total_query_time: i64 = row.get("total_query_time");
            let avg_query_time: i64 = row.get("avg_query_time");

            // Calculate queries per second (simplified)
            let queries_per_second = total_query_count as f64 / 60.0; // Rough estimate

            Ok(QueryStats {
                total_queries: total_query_count,
                queries_per_second,
                avg_query_time_ms: (avg_query_time as f64) / 1000.0, // Convert microseconds
                total_query_time_ms: total_query_time,
            })
        } else {
            Ok(QueryStats {
                total_queries: 0,
                queries_per_second: 0.0,
                avg_query_time_ms: 0.0,
                total_query_time_ms: 0,
            })
        }
    }

    async fn collect_global_stats(&self, conn: &mut PgConnection) -> Result<GlobalPoolStats, CollectorError> {
        // Aggregate statistics across all pools
        let row = sqlx::query(
            r#"
            SELECT 
                SUM(cl_active + cl_waiting) as total_client_connections,
                SUM(sv_active + sv_waiting) as total_server_connections,
                COUNT(DISTINCT pool_name) as total_pools,
                COUNT(DISTINCT database) as total_databases
            FROM pgbouncer_pools_stats 
            WHERE database != 'pgbouncer'
            "#
        )
        .fetch_one(conn)
        .await?;

        let total_client_connections: i64 = row.get("total_client_connections");
        let total_server_connections: i64 = row.get("total_server_connections");
        let total_pools: i64 = row.get("total_pools");
        let total_databases: i64 = row.get("total_databases");

        // Get memory usage (platform-specific)
        let memory_usage_mb = self.get_memory_usage().await.unwrap_or(0.0);

        Ok(GlobalPoolStats {
            total_client_connections: total_client_connections as i32,
            total_server_connections: total_server_connections as i32,
            total_pools: total_pools as i32,
            total_databases: total_databases as i32,
            memory_usage_mb,
        })
    }

    async fn get_memory_usage(&self) -> Result<f64, CollectorError> {
        // Platform-specific memory usage collection
        // This is a simplified example
        #[cfg(target_os = "linux")]
        {
            use std::fs;
            let status = fs::read_to_string("/proc/self/status")?;
            for line in status.lines() {
                if line.starts_with("VmRSS:") {
                    let parts: Vec<&str> = line.split_whitespace().collect();
                    if parts.len() >= 2 {
                        if let Ok(kb) = parts[1].parse::<f64>() {
                            return Ok(kb / 1024.0); // Convert KB to MB
                        }
                    }
                }
            }
        }
        Ok(0.0)
    }

    fn generate_alerts(&self, pools: &[PoolStats], global: &GlobalPoolStats) -> Vec<PoolAlert> {
        let mut alerts = Vec::new();
        let now = chrono::Utc::now().to_rfc3339();

        // Check global thresholds
        if global.total_client_connections > self.config.alert_thresholds.max_client_connections {
            alerts.push(PoolAlert {
                level: AlertLevel::Critical,
                pool_name: "global".to_string(),
                message: "High client connection count".to_string(),
                value: global.total_client_connections as f64,
                threshold: self.config.alert_thresholds.max_client_connections as f64,
                timestamp: now.clone(),
            });
        }

        // Check per-pool thresholds
        for pool in pools {
            // High wait times
            if pool.client_connections.max_wait_time_ms > self.config.alert_thresholds.max_wait_time_ms {
                alerts.push(PoolAlert {
                    level: AlertLevel::Warning,
                    pool_name: pool.pool_name.clone(),
                    message: "High client wait time".to_string(),
                    value: pool.client_connections.max_wait_time_ms as f64,
                    threshold: self.config.alert_thresholds.max_wait_time_ms as f64,
                    timestamp: now.clone(),
                });
            }

            // Low available connections
            let available_connections = pool.server_connections.max_connections - pool.server_connections.active;
            if available_connections < self.config.alert_thresholds.min_available_connections {
                alerts.push(PoolAlert {
                    level: AlertLevel::Critical,
                    pool_name: pool.pool_name.clone(),
                    message: "Low available server connections".to_string(),
                    value: available_connections as f64,
                    threshold: self.config.alert_thresholds.min_available_connections as f64,
                    timestamp: now.clone(),
                });
            }
        }

        alerts
    }
}

// Integration with the main extension manager
pub struct ExtensionRegistry {
    extensions: HashMap<String, Box<dyn CustomExtension>>,
}

#[async_trait]
pub trait CustomExtension: Send + Sync {
    async fn is_available(&self, conn: &mut PgConnection) -> Result<bool, CollectorError>;
    async fn collect(&self, conn: &mut PgConnection, params: &CommonParameters) -> Result<serde_json::Value, CollectorError>;
    fn name(&self) -> &str;
}

#[async_trait]
impl CustomExtension for CustomPoolExtension {
    async fn is_available(&self, conn: &mut PgConnection) -> Result<bool, CollectorError> {
        CustomPoolExtension::is_available(self, conn).await
    }

    async fn collect(&self, conn: &mut PgConnection, params: &CommonParameters) -> Result<serde_json::Value, CollectorError> {
        let metrics = self.collect_metrics(conn, params).await?;
        serde_json::to_value(metrics)
            .map_err(|e| CollectorError::General(anyhow::anyhow!("Serialization error: {}", e)))
    }

    fn name(&self) -> &str {
        "custom_pool_extension"
    }
}

impl ExtensionRegistry {
    pub fn new() -> Self {
        Self {
            extensions: HashMap::new(),
        }
    }

    pub fn register<T: CustomExtension + 'static>(&mut self, name: String, extension: T) {
        self.extensions.insert(name, Box::new(extension));
    }

    pub async fn detect_and_collect(
        &self,
        conn: &mut PgConnection,
        params: &CommonParameters,
    ) -> Result<HashMap<String, serde_json::Value>, CollectorError> {
        let mut results = HashMap::new();

        for (name, extension) in &self.extensions {
            if extension.is_available(conn).await? {
                info!("Extension {} is available, collecting metrics", name);
                match extension.collect(conn, params).await {
                    Ok(metrics) => {
                        results.insert(name.clone(), metrics);
                    }
                    Err(e) => {
                        warn!("Failed to collect metrics from extension {}: {}", name, e);
                    }
                }
            } else {
                info!("Extension {} is not available", name);
            }
        }

        Ok(results)
    }
}

// Example usage and testing
#[cfg(test)]
mod tests {
    use super::*;
    use sqlx::PgPool;

    async fn setup_test_pool() -> PgPool {
        // Setup test database with mock data
        let pool = PgPool::connect("postgresql://test:test@localhost/test").await.unwrap();
        
        // Create mock pgbouncer_stats tables for testing
        sqlx::query(
            r#"
            CREATE TABLE IF NOT EXISTS pgbouncer_pools_stats (
                pool_name TEXT,
                database TEXT,
                "user" TEXT,
                cl_active INTEGER,
                cl_waiting INTEGER,
                cl_total INTEGER,
                sv_active INTEGER,
                sv_waiting INTEGER,
                sv_total INTEGER,
                maxwait INTEGER,
                maxwait_us BIGINT,
                pool_mode TEXT
            )
            "#
        )
        .execute(&pool)
        .await
        .unwrap();

        // Insert test data
        sqlx::query(
            r#"
            INSERT INTO pgbouncer_pools_stats VALUES
            ('test_pool_1', 'test_db', 'test_user', 5, 2, 7, 3, 0, 3, 100, 100000, 'transaction'),
            ('test_pool_2', 'prod_db', 'prod_user', 15, 5, 20, 8, 1, 9, 250, 250000, 'session')
            "#
        )
        .execute(&pool)
        .await
        .unwrap();

        pool
    }

    #[tokio::test]
    async fn test_pool_extension_collection() {
        let pool = setup_test_pool().await;
        let mut conn = pool.acquire().await.unwrap();

        let config = PoolExtensionConfig::default();
        let extension = CustomPoolExtension::new(config);

        let params = CommonParameters {
            version: 15,
            databases: "test_db".to_string(),
            query_monitoring_count_threshold: 100,
            query_monitoring_response_time_threshold: 1000,
            host: "localhost".to_string(),
            port: "5432".to_string(),
            is_rds: false,
        };

        let metrics = extension.collect_metrics(&mut conn, &params).await.unwrap();

        assert_eq!(metrics.pools.len(), 2);
        assert_eq!(metrics.global_stats.total_pools, 2);
        assert!(metrics.global_stats.total_client_connections > 0);
    }

    #[tokio::test]
    async fn test_alert_generation() {
        let config = PoolExtensionConfig {
            alert_thresholds: PoolAlertThresholds {
                max_client_connections: 10, // Low threshold for testing
                max_server_connections: 5,
                min_available_connections: 2,
                max_wait_time_ms: 100,
            },
            ..Default::default()
        };

        let extension = CustomPoolExtension::new(config);

        // Create test data that should trigger alerts
        let pools = vec![PoolStats {
            pool_name: "test_pool".to_string(),
            database: "test_db".to_string(),
            user: "test_user".to_string(),
            client_connections: PoolConnectionStats {
                active: 8,
                waiting: 5,
                total: 13,
                max_connections: 20,
                avg_wait_time_ms: 150.0,
                max_wait_time_ms: 200, // Exceeds threshold
            },
            server_connections: PoolConnectionStats {
                active: 8, // Only 2 available (10 max - 8 active)
                waiting: 0,
                total: 8,
                max_connections: 10,
                avg_wait_time_ms: 0.0,
                max_wait_time_ms: 0,
            },
            query_stats: QueryStats {
                total_queries: 1000,
                queries_per_second: 10.0,
                avg_query_time_ms: 50.0,
                total_query_time_ms: 50000,
            },
        }];

        let global_stats = GlobalPoolStats {
            total_client_connections: 15, // Exceeds threshold of 10
            total_server_connections: 8,
            total_pools: 1,
            total_databases: 1,
            memory_usage_mb: 25.0,
        };

        let alerts = extension.generate_alerts(&pools, &global_stats);

        assert!(alerts.len() >= 2); // Should have multiple alerts
        
        let critical_alerts: Vec<_> = alerts.iter()
            .filter(|a| matches!(a.level, AlertLevel::Critical))
            .collect();
        assert!(!critical_alerts.is_empty());
    }

    #[tokio::test]
    async fn test_extension_registry() {
        let mut registry = ExtensionRegistry::new();
        
        let config = PoolExtensionConfig::default();
        let extension = CustomPoolExtension::new(config);
        
        registry.register("pool_monitor".to_string(), extension);

        // Test with mock connection would require more setup
        // This demonstrates the API structure
        assert_eq!(registry.extensions.len(), 1);
    }
}

// Example configuration integration
/*
Add to your main config.toml:

[extensions.custom_pool]
enabled = true
collection_interval_secs = 30
track_client_connections = true
track_server_connections = true

[extensions.custom_pool.alert_thresholds]
max_client_connections = 1000
max_server_connections = 100
min_available_connections = 5
max_wait_time_ms = 5000
*/