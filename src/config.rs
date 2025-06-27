use serde::{Deserialize, Serialize};
use std::path::Path;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CollectorConfig {
    // Connection settings - now supports multiple instances
    #[serde(alias = "POSTGRES_CONNECTION_STRING")]
    pub connection_string: String,  // Default/primary instance
    #[serde(alias = "POSTGRES_HOST")]
    pub host: String,
    #[serde(alias = "POSTGRES_PORT")]
    pub port: u16,
    #[serde(alias = "POSTGRES_DATABASES")]
    pub databases: Vec<String>,
    #[serde(alias = "POSTGRES_MAX_CONNECTIONS")]
    pub max_connections: u32,
    #[serde(alias = "POSTGRES_CONNECT_TIMEOUT_SECS")]
    pub connect_timeout_secs: u64,
    
    // Multi-instance support
    pub instances: Option<Vec<InstanceConfig>>,
    
    // Collection settings
    #[serde(alias = "COLLECTION_INTERVAL_SECS")]
    pub collection_interval_secs: u64,
    #[serde(alias = "COLLECTION_MODE")]
    pub collection_mode: CollectionMode,
    
    // OHI compatibility settings
    #[serde(alias = "QUERY_MONITORING_COUNT_THRESHOLD")]
    pub query_monitoring_count_threshold: i32,
    #[serde(alias = "QUERY_MONITORING_RESPONSE_TIME_THRESHOLD")]
    pub query_monitoring_response_time_threshold: i32,
    #[serde(alias = "MAX_QUERY_LENGTH")]
    pub max_query_length: usize,
    
    // Extended metrics
    #[serde(alias = "ENABLE_EXTENDED_METRICS")]
    pub enable_extended_metrics: bool,
    #[serde(alias = "ENABLE_EBPF")]
    pub enable_ebpf: bool,
    #[serde(alias = "ENABLE_ASH")]
    pub enable_ash: bool,
    #[serde(alias = "ASH_SAMPLE_INTERVAL_SECS")]
    pub ash_sample_interval_secs: u64,
    #[serde(alias = "ASH_RETENTION_HOURS")]
    pub ash_retention_hours: u64,
    #[serde(alias = "ASH_MAX_MEMORY_MB")]
    pub ash_max_memory_mb: Option<usize>,
    
    // Security settings
    #[serde(alias = "SANITIZE_QUERY_TEXT")]
    pub sanitize_query_text: bool,
    #[serde(alias = "SANITIZATION_MODE")]
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
    #[serde(alias = "NRI_ENABLED")]
    pub enabled: bool,
    #[serde(alias = "NRI_ENTITY_KEY")]
    pub entity_key: String,
    #[serde(alias = "NRI_INTEGRATION_NAME")]
    pub integration_name: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OTLPOutputConfig {
    #[serde(alias = "OTLP_ENABLED")]
    pub enabled: bool,
    #[serde(alias = "OTLP_ENDPOINT")]
    pub endpoint: String,
    #[serde(alias = "OTLP_COMPRESSION")]
    pub compression: String,
    #[serde(alias = "OTLP_TIMEOUT_SECS")]
    pub timeout_secs: u64,
    #[serde(alias = "OTLP_HEADERS")]
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
            .add_source(config::Environment::default()) // Support unprefixed environment variables
            .build()?;
        
        settings.try_deserialize()
    }

    pub fn from_env() -> Result<Self, config::ConfigError> {
        let settings = config::Config::builder()
            .add_source(config::Environment::default())
            .add_source(config::Environment::with_prefix("POSTGRES_COLLECTOR"))
            .build()?;
        
        settings.try_deserialize()
    }

    pub fn from_env_and_file<P: AsRef<Path>>(path: Option<P>) -> Result<Self, config::ConfigError> {
        let mut builder = config::Config::builder()
            .add_source(config::Environment::default())
            .add_source(config::Environment::with_prefix("POSTGRES_COLLECTOR"));
        
        if let Some(path) = path {
            builder = builder.add_source(config::File::from(path.as_ref()));
        }
        
        let settings = builder.build()?;
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