use anyhow::Result;
use async_trait::async_trait;
use sqlx::{postgres::PgPoolOptions, PgConnection, PgPool, Row};
use std::time::Duration;
use tracing::{info, warn, error};
use serde::{Deserialize, Serialize};

use postgres_collector_core::{CollectorError};

/// PgBouncer statistics collector
pub struct PgBouncerCollector {
    connection_pool: PgPool,
}

/// PgBouncer pool statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgBouncerPoolStats {
    pub database: String,
    pub user: String,
    pub cl_active: i32,      // Client connections that are linked to server connections and executing queries
    pub cl_waiting: i32,     // Client connections waiting for a server connection
    pub sv_active: i32,      // Server connections in use by a client
    pub sv_idle: i32,        // Server connections idle and ready for a client query
    pub sv_used: i32,        // Server connections idle more than server_check_delay
    pub sv_tested: i32,      // Server connections currently being tested
    pub sv_login: i32,       // Server connections currently logging in
    pub maxwait: i32,        // Maximum time a client waited for a server connection
    pub maxwait_us: i64,     // Maximum wait time in microseconds
    pub pool_mode: String,   // Pool mode (session, transaction, statement)
}

/// PgBouncer database statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgBouncerDatabaseStats {
    pub database: String,
    pub host: String,
    pub port: i32,
    pub pool_size: i32,
    pub reserve_pool: i32,
    pub max_connections: i32,
    pub current_connections: i32,
    pub paused: bool,
    pub disabled: bool,
}

/// PgBouncer client statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgBouncerClientStats {
    pub database: String,
    pub user: String,
    pub state: String,
    pub addr: String,
    pub port: i32,
    pub local_addr: String,
    pub local_port: i32,
    pub connect_time: String,
    pub request_time: String,
    pub wait_time: i64,
    pub close_needed: bool,
}

/// PgBouncer server statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgBouncerServerStats {
    pub database: String,
    pub user: String,
    pub state: String,
    pub addr: String,
    pub port: i32,
    pub local_addr: String,
    pub local_port: i32,
    pub connect_time: String,
    pub request_time: String,
}

/// PgBouncer global statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgBouncerGlobalStats {
    pub total_xact_count: i64,
    pub total_query_count: i64,
    pub total_received: i64,
    pub total_sent: i64,
    pub total_xact_time: i64,
    pub total_query_time: i64,
    pub total_wait_time: i64,
    pub avg_xact_count: i64,
    pub avg_query_count: i64,
    pub avg_recv: i64,
    pub avg_sent: i64,
    pub avg_xact_time: i64,
    pub avg_query_time: i64,
    pub avg_wait_time: i64,
}

/// Combined PgBouncer metrics
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PgBouncerMetrics {
    pub pools: Vec<PgBouncerPoolStats>,
    pub databases: Vec<PgBouncerDatabaseStats>,
    pub clients: Vec<PgBouncerClientStats>,
    pub servers: Vec<PgBouncerServerStats>,
    pub global_stats: Option<PgBouncerGlobalStats>,
}

impl PgBouncerCollector {
    pub async fn new(admin_connection_string: &str) -> Result<Self, CollectorError> {
        // PgBouncer admin interface requires a specific database name "pgbouncer"
        let admin_conn_str = if admin_connection_string.contains("pgbouncer") {
            admin_connection_string.to_string()
        } else {
            // Parse and modify the connection string to use pgbouncer database
            let mut parts = admin_connection_string.split('/').collect::<Vec<_>>();
            if parts.len() > 1 {
                let last_idx = parts.len() - 1;
                parts[last_idx] = "pgbouncer";
                parts.join("/")
            } else {
                format!("{}/pgbouncer", admin_connection_string.trim_end_matches('/'))
            }
        };
        
        let connection_pool = PgPoolOptions::new()
            .max_connections(2) // PgBouncer admin is lightweight
            .acquire_timeout(Duration::from_secs(5))
            .connect(&admin_conn_str)
            .await
            .map_err(|e| CollectorError::DatabaseError(e))?;
        
        Ok(Self {
            connection_pool,
        })
    }
    
    pub async fn collect_metrics(&self) -> Result<PgBouncerMetrics, CollectorError> {
        let mut metrics = PgBouncerMetrics::default();
        
        // Collect pool statistics
        match self.collect_pool_stats().await {
            Ok(pools) => metrics.pools = pools,
            Err(e) => warn!("Failed to collect PgBouncer pool stats: {}", e),
        }
        
        // Collect database statistics
        match self.collect_database_stats().await {
            Ok(databases) => metrics.databases = databases,
            Err(e) => warn!("Failed to collect PgBouncer database stats: {}", e),
        }
        
        // Collect client statistics
        match self.collect_client_stats().await {
            Ok(clients) => metrics.clients = clients,
            Err(e) => warn!("Failed to collect PgBouncer client stats: {}", e),
        }
        
        // Collect server statistics
        match self.collect_server_stats().await {
            Ok(servers) => metrics.servers = servers,
            Err(e) => warn!("Failed to collect PgBouncer server stats: {}", e),
        }
        
        // Collect global statistics
        match self.collect_global_stats().await {
            Ok(global) => metrics.global_stats = Some(global),
            Err(e) => warn!("Failed to collect PgBouncer global stats: {}", e),
        }
        
        Ok(metrics)
    }
    
    async fn collect_pool_stats(&self) -> Result<Vec<PgBouncerPoolStats>, CollectorError> {
        let mut conn = self.connection_pool.acquire().await?;
        
        let rows = sqlx::query("SHOW POOLS")
            .fetch_all(&mut *conn)
            .await?;
        
        let mut pools = Vec::new();
        for row in rows {
            pools.push(PgBouncerPoolStats {
                database: row.get("database"),
                user: row.get("user"),
                cl_active: row.get("cl_active"),
                cl_waiting: row.get("cl_waiting"),
                sv_active: row.get("sv_active"),
                sv_idle: row.get("sv_idle"),
                sv_used: row.get("sv_used"),
                sv_tested: row.get("sv_tested"),
                sv_login: row.get("sv_login"),
                maxwait: row.get("maxwait"),
                maxwait_us: row.get::<i64, _>("maxwait_us"),
                pool_mode: row.get("pool_mode"),
            });
        }
        
        Ok(pools)
    }
    
    async fn collect_database_stats(&self) -> Result<Vec<PgBouncerDatabaseStats>, CollectorError> {
        let mut conn = self.connection_pool.acquire().await?;
        
        let rows = sqlx::query("SHOW DATABASES")
            .fetch_all(&mut *conn)
            .await?;
        
        let mut databases = Vec::new();
        for row in rows {
            databases.push(PgBouncerDatabaseStats {
                database: row.get("name"),
                host: row.get("host"),
                port: row.get("port"),
                pool_size: row.get("pool_size"),
                reserve_pool: row.get("reserve_pool"),
                max_connections: row.get("max_connections"),
                current_connections: row.get("current_connections"),
                paused: row.get::<i32, _>("paused") == 1,
                disabled: row.get::<i32, _>("disabled") == 1,
            });
        }
        
        Ok(databases)
    }
    
    async fn collect_client_stats(&self) -> Result<Vec<PgBouncerClientStats>, CollectorError> {
        let mut conn = self.connection_pool.acquire().await?;
        
        let rows = sqlx::query("SHOW CLIENTS")
            .fetch_all(&mut *conn)
            .await?;
        
        let mut clients = Vec::new();
        for row in rows {
            clients.push(PgBouncerClientStats {
                database: row.get("database"),
                user: row.get("user"),
                state: row.get("state"),
                addr: row.get("addr"),
                port: row.get("port"),
                local_addr: row.get("local_addr"),
                local_port: row.get("local_port"),
                connect_time: row.get("connect_time"),
                request_time: row.get("request_time"),
                wait_time: row.get::<i64, _>("wait_us") / 1000, // Convert to ms
                close_needed: row.get::<i32, _>("close_needed") == 1,
            });
        }
        
        Ok(clients)
    }
    
    async fn collect_server_stats(&self) -> Result<Vec<PgBouncerServerStats>, CollectorError> {
        let mut conn = self.connection_pool.acquire().await?;
        
        let rows = sqlx::query("SHOW SERVERS")
            .fetch_all(&mut *conn)
            .await?;
        
        let mut servers = Vec::new();
        for row in rows {
            servers.push(PgBouncerServerStats {
                database: row.get("database"),
                user: row.get("user"),
                state: row.get("state"),
                addr: row.get("addr"),
                port: row.get("port"),
                local_addr: row.get("local_addr"),
                local_port: row.get("local_port"),
                connect_time: row.get("connect_time"),
                request_time: row.get("request_time"),
            });
        }
        
        Ok(servers)
    }
    
    async fn collect_global_stats(&self) -> Result<PgBouncerGlobalStats, CollectorError> {
        let mut conn = self.connection_pool.acquire().await?;
        
        let row = sqlx::query("SHOW STATS")
            .fetch_one(&mut *conn)
            .await?;
        
        Ok(PgBouncerGlobalStats {
            total_xact_count: row.get("total_xact_count"),
            total_query_count: row.get("total_query_count"),
            total_received: row.get("total_received"),
            total_sent: row.get("total_sent"),
            total_xact_time: row.get("total_xact_time"),
            total_query_time: row.get("total_query_time"),
            total_wait_time: row.get("total_wait_time"),
            avg_xact_count: row.get("avg_xact_count"),
            avg_query_count: row.get("avg_query_count"),
            avg_recv: row.get("avg_recv"),
            avg_sent: row.get("avg_sent"),
            avg_xact_time: row.get("avg_xact_time"),
            avg_query_time: row.get("avg_query_time"),
            avg_wait_time: row.get("avg_wait_time"),
        })
    }
    
    /// Check if this is a PgBouncer admin connection
    pub async fn validate_connection(&self) -> Result<bool, CollectorError> {
        let mut conn = self.connection_pool.acquire().await?;
        
        // Try to run a PgBouncer-specific command
        match sqlx::query("SHOW VERSION")
            .fetch_optional(&mut *conn)
            .await
        {
            Ok(Some(row)) => {
                let version: String = row.get("version");
                info!("Connected to PgBouncer version: {}", version);
                Ok(true)
            }
            Ok(None) => Ok(false),
            Err(e) => {
                error!("Failed to validate PgBouncer connection: {}", e);
                Ok(false)
            }
        }
    }
}