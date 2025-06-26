use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

use crate::error::{CollectorError, ProcessError};
use crate::metrics::UnifiedMetrics;

/// Core collector trait that both OTel and NRI implementations use
#[async_trait]
pub trait PostgresCollector: Send + Sync {
    type Config: CollectorConfig;
    type Output: MetricOutput;
    
    async fn collect(&self) -> Result<MetricBatch, CollectorError>;
    async fn process(&self, batch: MetricBatch) -> Result<Self::Output, ProcessError>;
    fn capabilities(&self) -> Capabilities;
}

/// Configuration trait for collectors
pub trait CollectorConfig: Send + Sync {
    fn validate(&self) -> Result<(), CollectorError>;
    fn merge_with(&mut self, other: Self) -> Result<(), CollectorError>;
}

/// Output trait for different metric formats
pub trait MetricOutput: Send + Sync {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError>;
    fn content_type(&self) -> &'static str;
}

/// Batch of collected metrics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetricBatch {
    pub metrics: UnifiedMetrics,
    pub metadata: CollectionMetadata,
    pub timestamp: chrono::DateTime<chrono::Utc>,
}

/// Metadata about the collection
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CollectionMetadata {
    pub collector_version: String,
    pub postgres_version: String,
    pub instance_id: String,
    pub collection_duration_ms: u64,
    pub errors: Vec<String>,
    pub warnings: Vec<String>,
}

/// Capabilities detected for the PostgreSQL instance
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Capabilities {
    pub version: u64,
    pub is_rds: bool,
    pub extensions: HashMap<String, ExtensionInfo>,
    pub has_superuser: bool,
    pub has_ebpf_support: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExtensionInfo {
    pub name: String,
    pub version: String,
    pub enabled: bool,
}

impl Capabilities {
    pub fn has_extension(&self, name: &str) -> bool {
        self.extensions
            .get(name)
            .map(|e| e.enabled)
            .unwrap_or(false)
    }
    
    pub fn supports_wait_events(&self) -> bool {
        self.has_extension("pg_stat_statements") && 
        (self.has_extension("pg_wait_sampling") || self.is_rds)
    }
    
    pub fn supports_blocking_sessions(&self) -> bool {
        match self.version {
            12 | 13 => true,
            v if v >= 14 => self.has_extension("pg_stat_statements"),
            _ => false,
        }
    }
    
    pub fn supports_individual_queries(&self) -> bool {
        self.has_extension("pg_stat_monitor") || self.is_rds
    }
}