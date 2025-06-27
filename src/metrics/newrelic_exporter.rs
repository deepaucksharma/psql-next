use anyhow::Result;
use opentelemetry::{global, KeyValue};
use opentelemetry_otlp::{ExportConfig, Protocol, WithExportConfig};
use opentelemetry_sdk::{
    metrics::{
        reader::{DefaultAggregationSelector, DefaultTemporalitySelector},
        MeterProviderBuilder, PeriodicReader, SdkMeterProvider,
    },
    runtime,
    Resource,
};
use std::time::Duration;
use tonic::metadata::{MetadataKey, MetadataMap, MetadataValue};

/// New Relic OTLP configuration
pub struct NewRelicConfig {
    pub api_key: String,
    pub endpoint: String,
    pub region: NewRelicRegion,
    pub export_interval: Duration,
    pub export_timeout: Duration,
}

#[derive(Debug, Clone)]
pub enum NewRelicRegion {
    US,
    EU,
}

impl NewRelicRegion {
    pub fn otlp_endpoint(&self, use_grpc: bool) -> String {
        match (self, use_grpc) {
            (NewRelicRegion::US, true) => "https://otlp.nr-data.net:4317".to_string(),
            (NewRelicRegion::US, false) => "https://otlp.nr-data.net:4318/v1/metrics".to_string(),
            (NewRelicRegion::EU, true) => "https://otlp.eu01.nr-data.net:4317".to_string(),
            (NewRelicRegion::EU, false) => "https://otlp.eu01.nr-data.net:4318/v1/metrics".to_string(),
        }
    }
}

/// Initialize New Relic OTLP exporter
pub fn init_new_relic_metrics(
    config: NewRelicConfig,
    resource: Resource,
) -> Result<SdkMeterProvider> {
    // Create headers with API key
    let mut headers = MetadataMap::new();
    headers.insert(
        MetadataKey::from_static("api-key"),
        MetadataValue::try_from(&config.api_key)?,
    );
    
    // Configure OTLP exporter
    let export_config = ExportConfig {
        endpoint: config.endpoint,
        protocol: Protocol::HttpBinary, // Use HTTP for better compatibility
        timeout: config.export_timeout,
        headers: Some(headers),
    };
    
    // Create OTLP exporter with New Relic specific settings
    let exporter = opentelemetry_otlp::new_exporter()
        .http()
        .with_endpoint(config.region.otlp_endpoint(false))
        .with_timeout(config.export_timeout)
        .with_headers(HashMap::from([
            ("api-key".to_string(), config.api_key),
        ]))
        .build_metrics_exporter(
            // Use Delta temporality for New Relic
            Box::new(DefaultTemporalitySelector::new()),
            Box::new(DefaultAggregationSelector::new()),
        )?;
    
    // Create periodic reader
    let reader = PeriodicReader::builder(exporter, runtime::Tokio)
        .with_interval(config.export_interval)
        .build();
    
    // Build meter provider
    let provider = MeterProviderBuilder::default()
        .with_resource(resource)
        .with_reader(reader)
        .build();
    
    // Set as global provider
    global::set_meter_provider(provider.clone());
    
    Ok(provider)
}

/// New Relic metric export configuration optimized for their platform
pub struct NewRelicExportSettings {
    /// Batch size - New Relic recommends 1000 metrics per request
    pub batch_size: usize,
    
    /// Export interval - typically 30-60 seconds
    pub export_interval: Duration,
    
    /// Maximum concurrent exports
    pub max_concurrent_exports: usize,
    
    /// Enable compression
    pub enable_compression: bool,
}

impl Default for NewRelicExportSettings {
    fn default() -> Self {
        Self {
            batch_size: 1000,
            export_interval: Duration::from_secs(30),
            max_concurrent_exports: 2,
            enable_compression: true,
        }
    }
}

/// Helper to create New Relic compatible metric attributes
pub struct NewRelicAttributes;

impl NewRelicAttributes {
    /// Create standard database attributes
    pub fn database(name: &str, host: &str, port: u16) -> Vec<KeyValue> {
        vec![
            KeyValue::new("db.system", "postgresql"),
            KeyValue::new("db.name", name.to_string()),
            KeyValue::new("db.host", host.to_string()),
            KeyValue::new("db.port", port as i64),
            KeyValue::new("db.connection_string", format!("postgresql://{}:{}/{}", host, port, name)),
        ]
    }
    
    /// Create query attributes with proper cardinality control
    pub fn query(
        operation: &str,
        fingerprint: &str,
        normalized_text: &str,
        user: &str,
    ) -> Vec<KeyValue> {
        vec![
            KeyValue::new("db.operation", operation.to_string()),
            KeyValue::new("db.query.fingerprint", fingerprint.to_string()),
            KeyValue::new("db.query.normalized", truncate_query(normalized_text, 1000)),
            KeyValue::new("db.user", user.to_string()),
        ]
    }
    
    /// Create wait event attributes
    pub fn wait_event(event_type: &str, event_name: &str) -> Vec<KeyValue> {
        vec![
            KeyValue::new("postgresql.wait.type", event_type.to_string()),
            KeyValue::new("postgresql.wait.event", event_name.to_string()),
        ]
    }
    
    /// Create table/index attributes
    pub fn table(schema: &str, table: &str, index: Option<&str>) -> Vec<KeyValue> {
        let mut attrs = vec![
            KeyValue::new("db.schema", schema.to_string()),
            KeyValue::new("db.table", table.to_string()),
        ];
        
        if let Some(idx) = index {
            attrs.push(KeyValue::new("db.index", idx.to_string()));
        }
        
        attrs
    }
}

/// Truncate query text to manage cardinality
fn truncate_query(query: &str, max_len: usize) -> String {
    if query.len() <= max_len {
        query.to_string()
    } else {
        format!("{}...", &query[..max_len])
    }
}

/// New Relic specific metric naming conventions
pub struct NewRelicMetricNaming;

impl NewRelicMetricNaming {
    /// Convert generic metric name to New Relic convention
    pub fn format_metric_name(category: &str, metric: &str) -> String {
        format!("postgresql.{}.{}", category, metric)
    }
    
    /// Standard metric categories
    pub const QUERY: &'static str = "query";
    pub const CONNECTION: &'static str = "connection";
    pub const REPLICATION: &'static str = "replication";
    pub const CHECKPOINT: &'static str = "checkpoint";
    pub const VACUUM: &'static str = "vacuum";
    pub const LOCK: &'static str = "lock";
    pub const TABLE: &'static str = "table";
    pub const INDEX: &'static str = "index";
    pub const WAIT: &'static str = "wait";
}

use std::collections::HashMap;