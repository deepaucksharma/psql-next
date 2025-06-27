use serde::{Deserialize, Serialize};
use std::path::Path;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CollectorConfig {
    // Connection settings - now supports multiple instances
    pub connection_string: String,  // Default/primary instance
    pub host: String,
    pub port: u16,
    pub databases: Vec<String>,
    pub max_connections: u32,
    pub connect_timeout_secs: u64,
    
    // Multi-instance support
    pub instances: Option<Vec<InstanceConfig>>,
    
    // Collection settings
    pub collection_interval_secs: u64,
    pub collection_mode: CollectionMode,
    
    // OHI compatibility settings
    pub query_monitoring_count_threshold: i32,
    pub query_monitoring_response_time_threshold: i32,
    pub max_query_length: usize,
    
    // Extended metrics
    pub enable_extended_metrics: bool,
    pub enable_ebpf: bool,
    pub enable_ash: bool,
    pub ash_sample_interval_secs: u64,
    pub ash_retention_hours: u64,
    pub ash_max_memory_mb: Option<usize>,
    
    // Security settings
    pub sanitize_query_text: bool,
    pub sanitization_mode: Option<String>,  // "full", "smart", "none"
    
    // Output settings
    pub outputs: OutputConfig,
    
    // Sampling configuration
    pub sampling: SamplingConfig,
    
    // PgBouncer monitoring
    pub pgbouncer: Option<PgBouncerConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum CollectionMode {
    Otel,
    Nri,
    Hybrid,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OutputConfig {
    pub nri: Option<NRIOutputConfig>,
    pub otlp: Option<OTLPOutputConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NRIOutputConfig {
    pub enabled: bool,
    pub entity_key: String,
    pub integration_name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OTLPOutputConfig {
    pub enabled: bool,
    pub endpoint: String,
    pub compression: String,
    pub timeout_secs: u64,
    pub headers: Vec<(String, String)>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SamplingConfig {
    pub mode: SamplingMode,
    pub base_sample_rate: f64,
    pub rules: Vec<SamplingRule>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SamplingMode {
    Fixed,
    Adaptive,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SamplingRule {
    pub condition: String,
    pub sample_rate: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PgBouncerConfig {
    pub enabled: bool,
    pub admin_connection_string: String,
    pub collection_interval_secs: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InstanceConfig {
    pub name: String,  // Unique identifier for this instance
    pub connection_string: String,
    pub host: String,
    pub port: u16,
    pub databases: Vec<String>,
    pub enabled: bool,
    
    // Instance-specific overrides
    pub query_monitoring_count_threshold: Option<i32>,
    pub query_monitoring_response_time_threshold: Option<i32>,
    pub enable_extended_metrics: Option<bool>,
    pub enable_ash: Option<bool>,
    
    // Instance-specific PgBouncer
    pub pgbouncer: Option<PgBouncerConfig>,
}

impl Default for CollectorConfig {
    fn default() -> Self {
        Self {
            connection_string: "postgresql://postgres:password@localhost:5432/postgres".to_string(),
            host: "localhost".to_string(),
            port: 5432,
            databases: vec!["postgres".to_string()],
            max_connections: 5,
            connect_timeout_secs: 30,
            
            instances: None,
            
            collection_interval_secs: 60,
            collection_mode: CollectionMode::Hybrid,
            
            query_monitoring_count_threshold: 20,
            query_monitoring_response_time_threshold: 500,
            max_query_length: 4095,
            
            enable_extended_metrics: false,
            enable_ebpf: false,
            enable_ash: false,
            ash_sample_interval_secs: 1,
            ash_retention_hours: 1,
            ash_max_memory_mb: Some(100),
            
            sanitize_query_text: false,
            sanitization_mode: Some("smart".to_string()),
            
            outputs: OutputConfig {
                nri: Some(NRIOutputConfig {
                    enabled: true,
                    entity_key: "${HOSTNAME}:${PORT}".to_string(),
                    integration_name: "com.newrelic.postgresql".to_string(),
                }),
                otlp: Some(OTLPOutputConfig {
                    enabled: true,
                    endpoint: "http://localhost:4317".to_string(),
                    compression: "gzip".to_string(),
                    timeout_secs: 30,
                    headers: vec![],
                }),
            },
            
            sampling: SamplingConfig {
                mode: SamplingMode::Fixed,
                base_sample_rate: 1.0,
                rules: vec![],
            },
            
            pgbouncer: None,
        }
    }
}

impl CollectorConfig {
    pub fn from_file<P: AsRef<Path>>(path: P) -> Result<Self, config::ConfigError> {
        let settings = config::Config::builder()
            .add_source(config::File::from(path.as_ref()))
            .add_source(config::Environment::with_prefix("POSTGRES_COLLECTOR"))
            .build()?;
        
        settings.try_deserialize()
    }
    
    pub fn validate(&self) -> Result<(), String> {
        if self.databases.is_empty() {
            return Err("At least one database must be specified".to_string());
        }
        
        if self.collection_interval_secs == 0 {
            return Err("Collection interval must be greater than 0".to_string());
        }
        
        if self.sampling.base_sample_rate < 0.0 || self.sampling.base_sample_rate > 1.0 {
            return Err("Base sample rate must be between 0.0 and 1.0".to_string());
        }
        
        Ok(())
    }
}