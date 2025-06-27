use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::Path;
use std::time::Duration;

/// OTEL-aligned hierarchical configuration for PostgreSQL receiver
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReceiverConfig {
    /// Connection configuration
    #[serde(default)]
    pub connection: ConnectionConfig,
    
    /// Collection configuration
    #[serde(default)]
    pub collection: CollectionConfig,
    
    /// Metrics configuration
    #[serde(default)]
    pub metrics: MetricsConfig,
    
    /// Features configuration
    #[serde(default)]
    pub features: FeaturesConfig,
    
    /// Resource attributes
    #[serde(default)]
    pub resource_attributes: HashMap<String, String>,
    
    /// TLS configuration
    pub tls: Option<TLSConfig>,
    
    /// Circuit breaker configuration
    #[serde(default)]
    pub circuit_breaker: CircuitBreakerConfig,
    
    /// Limits configuration
    #[serde(default)]
    pub limits: LimitsConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConnectionConfig {
    pub endpoint: String,
    pub database: String,
    #[serde(default = "default_max_connections")]
    pub max_connections: u32,
    #[serde(default = "default_min_connections")]
    pub min_connections: u32,
    #[serde(default = "default_timeout")]
    pub timeout_seconds: u64,
    #[serde(default = "default_idle_timeout")]
    pub idle_timeout_seconds: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CollectionConfig {
    #[serde(default = "default_interval")]
    pub interval_seconds: u64,
    #[serde(default = "default_query_timeout")]
    pub query_timeout_seconds: u64,
    #[serde(default = "default_max_concurrent")]
    pub max_concurrent_queries: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetricsConfig {
    #[serde(default)]
    pub backends: MetricConfig,
    #[serde(default)]
    pub database_size: MetricConfig,
    #[serde(default)]
    pub table_stats: TableStatsConfig,
    #[serde(default)]
    pub slow_queries: SlowQueriesConfig,
    #[serde(default)]
    pub wait_events: MetricConfig,
    #[serde(default)]
    pub locks: MetricConfig,
    #[serde(default)]
    pub transactions: MetricConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetricConfig {
    #[serde(default = "default_true")]
    pub enabled: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TableStatsConfig {
    #[serde(default = "default_true")]
    pub enabled: bool,
    #[serde(default)]
    pub include_system_tables: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SlowQueriesConfig {
    #[serde(default = "default_true")]
    pub enabled: bool,
    #[serde(default = "default_slow_query_threshold")]
    pub threshold_ms: u64,
    #[serde(default = "default_slow_query_limit")]
    pub limit: u32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeaturesConfig {
    #[serde(default)]
    pub extended_metrics: bool,
    #[serde(default)]
    pub wait_events: bool,
    #[serde(default)]
    pub ebpf: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TLSConfig {
    pub insecure_skip_verify: bool,
    pub ca_file: Option<String>,
    pub cert_file: Option<String>,
    pub key_file: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CircuitBreakerConfig {
    #[serde(default = "default_failure_threshold")]
    pub failure_threshold: u32,
    #[serde(default = "default_reset_timeout")]
    pub reset_timeout_seconds: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LimitsConfig {
    #[serde(default = "default_max_series_per_metric")]
    pub max_series_per_metric: usize,
    #[serde(default = "default_max_daily_series")]
    pub max_daily_series: usize,
    #[serde(default = "default_cache_size")]
    pub cache_size_per_shard: usize,
    #[serde(default = "default_sampling_rate")]
    pub sampling_rate: f64,
    #[serde(default = "default_adaptive_sampling")]
    pub adaptive_sampling: bool,
}

// Default implementations
impl Default for ReceiverConfig {
    fn default() -> Self {
        Self {
            connection: ConnectionConfig::default(),
            collection: CollectionConfig::default(),
            metrics: MetricsConfig::default(),
            features: FeaturesConfig::default(),
            resource_attributes: HashMap::new(),
            tls: None,
            circuit_breaker: CircuitBreakerConfig::default(),
            limits: LimitsConfig::default(),
        }
    }
}

impl Default for ConnectionConfig {
    fn default() -> Self {
        Self {
            endpoint: "postgres://localhost:5432/postgres".to_string(),
            database: "postgres".to_string(),
            max_connections: default_max_connections(),
            min_connections: default_min_connections(),
            timeout_seconds: default_timeout(),
            idle_timeout_seconds: default_idle_timeout(),
        }
    }
}

impl Default for CollectionConfig {
    fn default() -> Self {
        Self {
            interval_seconds: default_interval(),
            query_timeout_seconds: default_query_timeout(),
            max_concurrent_queries: default_max_concurrent(),
        }
    }
}

impl Default for MetricsConfig {
    fn default() -> Self {
        Self {
            backends: MetricConfig::default(),
            database_size: MetricConfig::default(),
            table_stats: TableStatsConfig::default(),
            slow_queries: SlowQueriesConfig::default(),
            wait_events: MetricConfig::default(),
            locks: MetricConfig::default(),
            transactions: MetricConfig::default(),
        }
    }
}

impl Default for MetricConfig {
    fn default() -> Self {
        Self {
            enabled: default_true(),
        }
    }
}

impl Default for TableStatsConfig {
    fn default() -> Self {
        Self {
            enabled: default_true(),
            include_system_tables: false,
        }
    }
}

impl Default for SlowQueriesConfig {
    fn default() -> Self {
        Self {
            enabled: default_true(),
            threshold_ms: default_slow_query_threshold(),
            limit: default_slow_query_limit(),
        }
    }
}

impl Default for FeaturesConfig {
    fn default() -> Self {
        Self {
            extended_metrics: false,
            wait_events: false,
            ebpf: false,
        }
    }
}

impl Default for CircuitBreakerConfig {
    fn default() -> Self {
        Self {
            failure_threshold: default_failure_threshold(),
            reset_timeout_seconds: default_reset_timeout(),
        }
    }
}

impl Default for LimitsConfig {
    fn default() -> Self {
        Self {
            max_series_per_metric: default_max_series_per_metric(),
            max_daily_series: default_max_daily_series(),
            cache_size_per_shard: default_cache_size(),
            sampling_rate: default_sampling_rate(),
            adaptive_sampling: default_adaptive_sampling(),
        }
    }
}

// Default value functions
fn default_true() -> bool { true }
fn default_max_connections() -> u32 { 10 }
fn default_min_connections() -> u32 { 2 }
fn default_timeout() -> u64 { 30 }
fn default_idle_timeout() -> u64 { 600 }
fn default_interval() -> u64 { 60 }
fn default_query_timeout() -> u64 { 30 }
fn default_max_concurrent() -> usize { 5 }
fn default_slow_query_threshold() -> u64 { 100 }
fn default_slow_query_limit() -> u32 { 1000 }
fn default_failure_threshold() -> u32 { 5 }
fn default_reset_timeout() -> u64 { 60 }
fn default_max_series_per_metric() -> usize { 90_000 }
fn default_max_daily_series() -> usize { 2_500_000 }
fn default_cache_size() -> usize { 10_000 }
fn default_sampling_rate() -> f64 { 0.1 }
fn default_adaptive_sampling() -> bool { true }

/// Validation errors for configuration
#[derive(Debug)]
pub struct ValidationErrors {
    errors: Vec<(String, String)>,
}

impl ValidationErrors {
    pub fn new() -> Self {
        Self { errors: Vec::new() }
    }
    
    pub fn add(&mut self, field: &str, message: &str) {
        self.errors.push((field.to_string(), message.to_string()));
    }
    
    pub fn has_errors(&self) -> bool {
        !self.errors.is_empty()
    }
    
    pub fn into_result(self) -> Result<(), String> {
        if self.errors.is_empty() {
            Ok(())
        } else {
            let messages: Vec<String> = self.errors
                .into_iter()
                .map(|(field, msg)| format!("{}: {}", field, msg))
                .collect();
            Err(messages.join(", "))
        }
    }
}

impl ReceiverConfig {
    /// Load configuration from file with environment variable override
    pub fn from_file<P: AsRef<Path>>(path: P) -> Result<Self, config::ConfigError> {
        let settings = config::Config::builder()
            .add_source(config::File::from(path.as_ref()))
            .add_source(config::Environment::with_prefix("POSTGRES_RECEIVER"))
            .build()?;
        
        settings.try_deserialize()
    }
    
    /// Validate the configuration
    pub fn validate(&self) -> Result<(), ValidationErrors> {
        let mut errors = ValidationErrors::new();
        
        // Connection validation
        if self.connection.endpoint.is_empty() {
            errors.add("connection.endpoint", "Endpoint cannot be empty");
        } else if let Err(_) = url::Url::parse(&self.connection.endpoint) {
            errors.add("connection.endpoint", "Invalid PostgreSQL URL");
        }
        
        if self.connection.max_connections < self.connection.min_connections {
            errors.add("connection", "max_connections must be >= min_connections");
        }
        
        // Collection validation
        if self.collection.interval_seconds < 10 {
            errors.add("collection.interval", "Interval must be at least 10 seconds");
        }
        
        if self.collection.query_timeout_seconds == 0 {
            errors.add("collection.query_timeout", "Query timeout must be > 0");
        }
        
        // Metrics validation
        if self.metrics.slow_queries.enabled && self.metrics.slow_queries.threshold_ms == 0 {
            errors.add("metrics.slow_queries.threshold_ms", "Threshold must be > 0");
        }
        
        // TLS validation
        if let Some(tls) = &self.tls {
            if !tls.insecure_skip_verify && tls.ca_file.is_none() {
                errors.add("tls.ca_file", "CA file required when TLS verification is enabled");
            }
        }
        
        // Limits validation
        if self.limits.sampling_rate < 0.0 || self.limits.sampling_rate > 1.0 {
            errors.add("limits.sampling_rate", "Sampling rate must be between 0.0 and 1.0");
        }
        
        if errors.has_errors() {
            Err(errors)
        } else {
            Ok(())
        }
    }
}