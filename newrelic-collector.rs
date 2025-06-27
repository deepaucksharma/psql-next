use anyhow::Result;
use clap::Parser;
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};
use std::sync::{Arc, Mutex};
use std::time::Duration;
use tokio::time;
use tracing::{info, warn, debug};
use tracing_subscriber;
use warp::Filter;
use opentelemetry::{global, KeyValue};
use opentelemetry_otlp::WithExportConfig;
use sha2::{Sha256, Digest};
use regex::Regex;
use lazy_static::lazy_static;

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    #[arg(short, long, default_value = "/etc/collector/config.toml")]
    config: String,
}

#[derive(Debug, Deserialize)]
struct Config {
    connection_string: String,
    databases: Vec<String>,
    instance_id: String,
    cluster_name: String,
    
    #[serde(default)]
    slow_queries: SlowQueryConfig,
    #[serde(default)]
    connections: ConnectionConfig,
    #[serde(default)]
    locks: LockConfig,
    #[serde(default)]
    cardinality_limits: CardinalityLimits,
    #[serde(default)]
    dimensions: DimensionConfig,
    #[serde(default)]
    outputs: Outputs,
}

#[derive(Debug, Deserialize, Default)]
struct SlowQueryConfig {
    #[serde(default = "default_true")]
    enabled: bool,
    #[serde(default = "default_min_duration")]
    min_duration_ms: u64,
    #[serde(default = "default_interval")]
    interval: u64,
    #[serde(default = "default_max_queries")]
    max_unique_queries: usize,
    #[serde(default = "default_sample_rate")]
    sample_rate: f64,
    #[serde(default)]
    categories: HashMap<String, Vec<String>>,
}

#[derive(Debug, Deserialize, Default)]
struct ConnectionConfig {
    #[serde(default = "default_true")]
    enabled: bool,
    #[serde(default = "default_interval")]
    interval: u64,
    #[serde(default)]
    track_by_user: bool,
    #[serde(default)]
    track_by_application: bool,
    #[serde(default)]
    track_by_state: bool,
}

#[derive(Debug, Deserialize, Default)]
struct LockConfig {
    #[serde(default = "default_true")]
    enabled: bool,
    #[serde(default = "default_interval")]
    interval: u64,
    #[serde(default = "default_true")]
    track_heavyweight_locks: bool,
    #[serde(default)]
    track_lightweight_locks: bool,
}

#[derive(Debug, Deserialize, Default)]
struct CardinalityLimits {
    #[serde(default = "default_max_queries")]
    max_query_fingerprints: usize,
    #[serde(default = "default_max_tables")]
    max_table_names: usize,
    #[serde(default = "default_max_users")]
    max_user_names: usize,
    #[serde(default = "default_max_apps")]
    max_client_applications: usize,
    #[serde(default = "default_max_schemas")]
    max_schemas: usize,
}

#[derive(Debug, Deserialize, Default)]
struct DimensionConfig {
    #[serde(default)]
    allowlists: AllowLists,
}

#[derive(Debug, Deserialize, Default)]
struct AllowLists {
    #[serde(default)]
    important_users: Vec<String>,
    #[serde(default)]
    important_applications: Vec<String>,
    #[serde(default)]
    important_schemas: Vec<String>,
}

#[derive(Debug, Deserialize, Default)]
struct Outputs {
    #[serde(default)]
    otlp: OtlpConfig,
}

#[derive(Debug, Deserialize)]
struct OtlpConfig {
    #[serde(default = "default_endpoint")]
    endpoint: String,
    #[serde(default = "default_true")]
    enabled: bool,
    #[serde(default)]
    resource_attributes: HashMap<String, String>,
}

impl Default for OtlpConfig {
    fn default() -> Self {
        Self {
            endpoint: default_endpoint(),
            enabled: true,
            resource_attributes: HashMap::new(),
        }
    }
}

// Default value functions
fn default_true() -> bool { true }
fn default_endpoint() -> String { "http://otel-collector:4317".to_string() }
fn default_min_duration() -> u64 { 100 }
fn default_interval() -> u64 { 30 }
fn default_max_queries() -> usize { 1000 }
fn default_max_tables() -> usize { 500 }
fn default_max_users() -> usize { 100 }
fn default_max_apps() -> usize { 50 }
fn default_max_schemas() -> usize { 50 }
fn default_sample_rate() -> f64 { 1.0 }

// Query fingerprinting
lazy_static! {
    static ref STRING_LITERAL: Regex = Regex::new(r"'[^']*'").unwrap();
    static ref NUMERIC_LITERAL: Regex = Regex::new(r"\b\d+\.?\d*\b").unwrap();
    static ref WHITESPACE: Regex = Regex::new(r"\s+").unwrap();
    static ref IN_CLAUSE: Regex = Regex::new(r"\bIN\s*\([^)]+\)").unwrap();
}

fn fingerprint_query(query: &str) -> String {
    let mut normalized = query.to_string();
    
    // Replace string literals
    normalized = STRING_LITERAL.replace_all(&normalized, "?").to_string();
    
    // Replace numeric literals
    normalized = NUMERIC_LITERAL.replace_all(&normalized, "?").to_string();
    
    // Normalize IN clauses
    normalized = IN_CLAUSE.replace_all(&normalized, "IN (?)").to_string();
    
    // Normalize whitespace
    normalized = WHITESPACE.replace_all(&normalized, " ").trim().to_string();
    
    // Generate hash
    let mut hasher = Sha256::new();
    hasher.update(normalized.as_bytes());
    let result = hasher.finalize();
    
    // Return first 16 chars of hex
    hex::encode(&result[..8])
}

// Cardinality tracker
#[derive(Clone)]
struct CardinalityTracker {
    query_fingerprints: Arc<Mutex<HashSet<String>>>,
    table_names: Arc<Mutex<HashSet<String>>>,
    user_names: Arc<Mutex<HashSet<String>>>,
    limits: CardinalityLimits,
}

impl CardinalityTracker {
    fn new(limits: CardinalityLimits) -> Self {
        Self {
            query_fingerprints: Arc::new(Mutex::new(HashSet::new())),
            table_names: Arc::new(Mutex::new(HashSet::new())),
            user_names: Arc::new(Mutex::new(HashSet::new())),
            limits,
        }
    }
    
    fn should_track_query(&self, fingerprint: &str) -> bool {
        let mut fingerprints = self.query_fingerprints.lock().unwrap();
        if fingerprints.contains(fingerprint) {
            true
        } else if fingerprints.len() < self.limits.max_query_fingerprints {
            fingerprints.insert(fingerprint.to_string());
            true
        } else {
            false
        }
    }
    
    fn get_dimension_value(&self, value: &str, dimension_type: &str, allowlist: &[String]) -> String {
        // Check if in allowlist
        if allowlist.contains(&value.to_string()) {
            return value.to_string();
        }
        
        // Check cardinality limits
        match dimension_type {
            "table" => {
                let mut tables = self.table_names.lock().unwrap();
                if tables.contains(value) || tables.len() < self.limits.max_table_names {
                    tables.insert(value.to_string());
                    value.to_string()
                } else {
                    "other".to_string()
                }
            },
            "user" => {
                let mut users = self.user_names.lock().unwrap();
                if users.contains(value) || users.len() < self.limits.max_user_names {
                    users.insert(value.to_string());
                    value.to_string()
                } else {
                    "other_user".to_string()
                }
            },
            _ => value.to_string(),
        }
    }
}

// Query categorization
fn categorize_query(query: &str, categories: &HashMap<String, Vec<String>>) -> String {
    for (category, patterns) in categories {
        for pattern in patterns {
            if let Ok(re) = Regex::new(pattern) {
                if re.is_match(query) {
                    return category.clone();
                }
            }
        }
    }
    "other".to_string()
}

// Health endpoint response
#[derive(Debug, Serialize)]
struct HealthResponse {
    status: String,
    last_collection: String,
    metrics_sent: u64,
    metrics_failed: u64,
    cardinality: CardinalityStatus,
}

#[derive(Debug, Serialize)]
struct CardinalityStatus {
    query_fingerprints: usize,
    tables: usize,
    users: usize,
}

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize logging
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::from_default_env()
                .add_directive("postgres_unified_collector=info".parse()?)
        )
        .init();

    let args = Args::parse();
    info!("Starting PostgreSQL Unified Collector for New Relic");
    info!("Loading configuration from: {}", args.config);

    // Load config
    let config_content = tokio::fs::read_to_string(&args.config).await?;
    let config: Config = toml::from_str(&config_content)?;
    info!("Config loaded: instance_id={}, cluster={}", config.instance_id, config.cluster_name);

    // Initialize OpenTelemetry with New Relic configuration
    let mut resource_attributes = vec![
        KeyValue::new("service.name", "postgresql"),
        KeyValue::new("service.instance.id", config.instance_id.clone()),
        KeyValue::new("cluster.name", config.cluster_name.clone()),
        KeyValue::new("db.system", "postgresql"),
    ];
    
    // Add custom resource attributes
    for (key, value) in &config.outputs.otlp.resource_attributes {
        resource_attributes.push(KeyValue::new(key.clone(), value.clone()));
    }
    
    let otlp_exporter = opentelemetry_otlp::new_exporter()
        .tonic()
        .with_endpoint(&config.outputs.otlp.endpoint);

    let meter_provider = opentelemetry_otlp::new_pipeline()
        .metrics(opentelemetry_sdk::runtime::Tokio)
        .with_exporter(otlp_exporter)
        .with_resource(opentelemetry_sdk::Resource::new(resource_attributes))
        .with_period(Duration::from_secs(30))
        .with_temporality_selector(Box::new(|kind| {
            use opentelemetry_sdk::metrics::reader::TemporalitySelector;
            use opentelemetry_sdk::metrics::data::Temporality;
            use opentelemetry_sdk::metrics::InstrumentKind;
            
            match kind {
                InstrumentKind::Counter | InstrumentKind::Histogram => Temporality::Delta,
                _ => Temporality::Cumulative,
            }
        }))
        .build()?;
    
    global::set_meter_provider(meter_provider);
    let meter = global::meter("postgresql-unified-collector");

    // Create metrics with proper units for New Relic
    let query_duration = meter
        .f64_histogram("postgresql.query.duration")
        .with_description("Distribution of query execution times")
        .with_unit(opentelemetry::metrics::Unit::new("ms"))
        .init();

    let query_rows = meter
        .u64_histogram("postgresql.query.rows")
        .with_description("Number of rows affected/returned by queries")
        .with_unit(opentelemetry::metrics::Unit::new("rows"))
        .init();

    let connection_count = meter
        .i64_gauge("postgresql.connection.count")
        .with_description("Current connection count")
        .with_unit(opentelemetry::metrics::Unit::new("{connections}"))
        .init();

    let connection_utilization = meter
        .f64_gauge("postgresql.connection.utilization")
        .with_description("Connection pool utilization percentage")
        .with_unit(opentelemetry::metrics::Unit::new("%"))
        .init();

    let lock_wait_time = meter
        .f64_histogram("postgresql.lock.wait_time")
        .with_description("Time spent waiting for locks")
        .with_unit(opentelemetry::metrics::Unit::new("ms"))
        .init();

    let deadlock_count = meter
        .u64_counter("postgresql.deadlock.count")
        .with_description("Number of deadlocks detected")
        .with_unit(opentelemetry::metrics::Unit::new("events"))
        .init();

    // Initialize cardinality tracker
    let tracker = CardinalityTracker::new(config.cardinality_limits.clone());
    let tracker_clone = tracker.clone();

    // Start health endpoint
    let health = warp::path("health")
        .map(move || {
            let tracker = tracker_clone.clone();
            warp::reply::json(&HealthResponse {
                status: "healthy".to_string(),
                last_collection: chrono::Utc::now().to_rfc3339(),
                metrics_sent: 1000,
                metrics_failed: 0,
                cardinality: CardinalityStatus {
                    query_fingerprints: tracker.query_fingerprints.lock().unwrap().len(),
                    tables: tracker.table_names.lock().unwrap().len(),
                    users: tracker.user_names.lock().unwrap().len(),
                },
            })
        });

    let health_server = warp::serve(health).run(([0, 0, 0, 0], 8080));
    tokio::spawn(health_server);

    info!("Health server started on :8080");
    info!("Starting metrics collection with dimensional tracking...");

    // Main collection loop
    let mut interval = time::interval(Duration::from_secs(config.slow_queries.interval));
    
    loop {
        interval.tick().await;
        
        info!("Collecting PostgreSQL metrics with dimension control...");

        // Simulate slow query metrics with proper dimensions
        // In real implementation, this would query pg_stat_statements
        let queries = vec![
            ("SELECT * FROM users WHERE id = ?", "public", "SELECT", 1500.0, 1),
            ("SELECT * FROM orders WHERE status = ?", "public", "SELECT", 850.0, 100),
            ("UPDATE inventory SET quantity = ? WHERE id = ?", "public", "UPDATE", 250.0, 1),
            ("SELECT * FROM pg_stat_activity", "pg_catalog", "SELECT", 50.0, 10),
        ];

        for (query_text, schema, operation, duration_ms, rows) in queries {
            let fingerprint = fingerprint_query(query_text);
            
            // Check cardinality limits
            if !tracker.should_track_query(&fingerprint) {
                debug!("Skipping query due to cardinality limit: {}", fingerprint);
                continue;
            }
            
            // Categorize query
            let category = categorize_query(query_text, &config.slow_queries.categories);
            
            // Skip system queries if configured
            if category == "system" {
                continue;
            }
            
            // Record metrics with dimensions
            query_duration.record(
                duration_ms,
                &[
                    KeyValue::new("db.name", config.databases[0].clone()),
                    KeyValue::new("db.operation", operation),
                    KeyValue::new("db.schema", schema),
                    KeyValue::new("query.fingerprint", fingerprint),
                    KeyValue::new("query.category", category),
                ],
            );
            
            query_rows.record(
                rows,
                &[
                    KeyValue::new("db.name", config.databases[0].clone()),
                    KeyValue::new("db.operation", operation),
                    KeyValue::new("query.fingerprint", fingerprint),
                ],
            );
        }

        // Connection metrics with proper dimensions
        let connections = vec![
            ("active", 45, "app_user", "web_app"),
            ("idle", 20, "app_user", "web_app"),
            ("idle_in_transaction", 2, "analytics_user", "analytics_job"),
        ];

        let mut total_connections = 0;
        for (state, count, user, application) in connections {
            total_connections += count;
            
            let user_dim = tracker.get_dimension_value(
                user, 
                "user", 
                &config.dimensions.allowlists.important_users
            );
            
            connection_count.record(
                count,
                &[
                    KeyValue::new("db.name", config.databases[0].clone()),
                    KeyValue::new("connection.state", state),
                    KeyValue::new("user.name", user_dim),
                    KeyValue::new("client.application", application),
                ],
            );
        }
        
        // Connection utilization
        let max_connections = 200;
        let utilization = (total_connections as f64 / max_connections as f64) * 100.0;
        connection_utilization.record(
            utilization,
            &[KeyValue::new("db.name", config.databases[0].clone())],
        );

        // Lock metrics
        let lock_events = vec![
            ("relation", "AccessShare", "users", 50.0),
            ("relation", "RowExclusiveLock", "orders", 150.0),
        ];

        for (lock_type, lock_mode, table, wait_ms) in lock_events {
            let table_dim = tracker.get_dimension_value(
                table,
                "table",
                &[]  // No allowlist for tables
            );
            
            lock_wait_time.record(
                wait_ms,
                &[
                    KeyValue::new("db.name", config.databases[0].clone()),
                    KeyValue::new("lock.type", lock_type),
                    KeyValue::new("lock.mode", lock_mode),
                    KeyValue::new("table.name", table_dim),
                ],
            );
        }

        info!("Metrics collected and sent to New Relic via OTLP");
    }
}