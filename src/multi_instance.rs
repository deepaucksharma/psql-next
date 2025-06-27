use anyhow::Result;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use tokio::task::JoinHandle;
use tracing::{info, warn, error};

use crate::collection_engine::UnifiedCollectionEngine;
use crate::config::{CollectorConfig, InstanceConfig};
use postgres_collector_core::{UnifiedMetrics, CollectorError};

/// Manages multiple PostgreSQL instance collectors
pub struct MultiInstanceManager {
    instances: Arc<RwLock<HashMap<String, InstanceCollector>>>,
    base_config: CollectorConfig,
}

struct InstanceCollector {
    name: String,
    engine: UnifiedCollectionEngine,
    handle: Option<JoinHandle<()>>,
    metrics: Arc<RwLock<Option<UnifiedMetrics>>>,
}

/// Combined metrics from all instances
#[derive(Debug, Clone, Default)]
pub struct MultiInstanceMetrics {
    pub instances: HashMap<String, UnifiedMetrics>,
    pub collection_timestamp: chrono::DateTime<chrono::Utc>,
}

impl MultiInstanceManager {
    pub async fn new(config: CollectorConfig) -> Result<Self> {
        Ok(Self {
            instances: Arc::new(RwLock::new(HashMap::new())),
            base_config: config,
        })
    }
    
    /// Initialize all configured instances
    pub async fn initialize(&self) -> Result<()> {
        // First, add the primary instance from base config
        let primary_config = self.create_instance_config_from_base("primary");
        self.add_instance("primary".to_string(), primary_config).await?;
        
        // Then add any additional instances
        if let Some(instances) = &self.base_config.instances {
            for inst_config in instances {
                if inst_config.enabled {
                    let config = self.merge_instance_config(inst_config);
                    self.add_instance(inst_config.name.clone(), config).await?;
                }
            }
        }
        
        Ok(())
    }
    
    /// Create instance config from base configuration
    fn create_instance_config_from_base(&self, name: &str) -> CollectorConfig {
        let mut config = self.base_config.clone();
        // Clear instances to avoid recursion
        config.instances = None;
        config
    }
    
    /// Merge instance-specific config with base config
    fn merge_instance_config(&self, instance: &InstanceConfig) -> CollectorConfig {
        let mut config = self.base_config.clone();
        
        // Override connection settings
        config.connection_string = instance.connection_string.clone();
        config.host = instance.host.clone();
        config.port = instance.port;
        config.databases = instance.databases.clone();
        
        // Override monitoring thresholds if specified
        if let Some(threshold) = instance.query_monitoring_count_threshold {
            config.query_monitoring_count_threshold = threshold;
        }
        if let Some(threshold) = instance.query_monitoring_response_time_threshold {
            config.query_monitoring_response_time_threshold = threshold;
        }
        
        // Override feature flags if specified
        if let Some(extended) = instance.enable_extended_metrics {
            config.enable_extended_metrics = extended;
        }
        if let Some(ash) = instance.enable_ash {
            config.enable_ash = ash;
        }
        
        // Set instance-specific PgBouncer config
        if let Some(pgb) = &instance.pgbouncer {
            config.pgbouncer = Some(pgb.clone());
        }
        
        // Clear instances to avoid recursion
        config.instances = None;
        
        config
    }
    
    /// Add a new instance to monitor
    async fn add_instance(&self, name: String, config: CollectorConfig) -> Result<()> {
        info!("Adding instance '{}' to multi-instance manager", name);
        
        match UnifiedCollectionEngine::new(config).await {
            Ok(engine) => {
                let collector = InstanceCollector {
                    name: name.clone(),
                    engine,
                    handle: None,
                    metrics: Arc::new(RwLock::new(None)),
                };
                
                self.instances.write().await.insert(name.clone(), collector);
                info!("Successfully added instance '{}'", name);
                Ok(())
            }
            Err(e) => {
                error!("Failed to create collector for instance '{}': {}", name, e);
                Err(e.into())
            }
        }
    }
    
    /// Collect metrics from all instances
    pub async fn collect_all(&self) -> Result<MultiInstanceMetrics> {
        let mut combined = MultiInstanceMetrics {
            instances: HashMap::new(),
            collection_timestamp: chrono::Utc::now(),
        };
        
        let instances = self.instances.read().await;
        
        // Collect from each instance in parallel
        let mut tasks = Vec::new();
        
        for (name, collector) in instances.iter() {
            let name = name.clone();
            let engine = &collector.engine;
            let metrics_store = collector.metrics.clone();
            
            // Create a future for collection
            let fut = async move {
                match engine.collect_all_metrics().await {
                    Ok(metrics) => {
                        *metrics_store.write().await = Some(metrics.clone());
                        Ok((name, metrics))
                    }
                    Err(e) => {
                        error!("Failed to collect metrics from instance '{}': {}", name, e);
                        Err((name, e))
                    }
                }
            };
            
            tasks.push(fut);
        }
        
        // Wait for all collections to complete
        let results = futures::future::join_all(tasks).await;
        
        // Process results
        for result in results {
            match result {
                Ok((name, metrics)) => {
                    combined.instances.insert(name, metrics);
                }
                Err((name, _)) => {
                    warn!("Skipping instance '{}' due to collection error", name);
                }
            }
        }
        
        Ok(combined)
    }
    
    /// Send metrics for all instances through their configured adapters
    pub async fn send_all_metrics(&self) -> Result<()> {
        let instances = self.instances.read().await;
        let mut errors = Vec::new();
        
        for (name, collector) in instances.iter() {
            if let Some(metrics) = &*collector.metrics.read().await {
                match collector.engine.send_metrics(metrics).await {
                    Ok(_) => info!("Successfully sent metrics for instance '{}'", name),
                    Err(e) => {
                        error!("Failed to send metrics for instance '{}': {}", name, e);
                        errors.push(format!("{}: {}", name, e));
                    }
                }
            }
        }
        
        if !errors.is_empty() {
            return Err(anyhow::anyhow!(
                "Failed to send metrics for some instances: {}",
                errors.join(", ")
            ));
        }
        
        Ok(())
    }
    
    /// Get metrics for a specific instance
    pub async fn get_instance_metrics(&self, name: &str) -> Option<UnifiedMetrics> {
        let instances = self.instances.read().await;
        
        if let Some(collector) = instances.get(name) {
            collector.metrics.read().await.clone()
        } else {
            None
        }
    }
    
    /// List all managed instances
    pub async fn list_instances(&self) -> Vec<String> {
        self.instances.read().await.keys().cloned().collect()
    }
    
    /// Remove an instance from monitoring
    pub async fn remove_instance(&self, name: &str) -> Result<()> {
        let mut instances = self.instances.write().await;
        
        if let Some(mut collector) = instances.remove(name) {
            // Cancel any running collection task
            if let Some(handle) = collector.handle.take() {
                handle.abort();
            }
            info!("Removed instance '{}' from monitoring", name);
            Ok(())
        } else {
            Err(anyhow::anyhow!("Instance '{}' not found", name))
        }
    }
    
    /// Health check for all instances
    pub async fn health_check(&self) -> HashMap<String, bool> {
        let mut health = HashMap::new();
        let instances = self.instances.read().await;
        
        for (name, collector) in instances.iter() {
            // Instance is healthy if we have recent metrics
            let is_healthy = collector.metrics.read().await.is_some();
            health.insert(name.clone(), is_healthy);
        }
        
        health
    }
}