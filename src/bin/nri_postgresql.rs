use anyhow::Result;
use clap::Parser;
use std::env;
use tracing::{error, info};

use postgres_unified_collector::{
    UnifiedCollectionEngine, CollectorConfig, CollectionMode,
    adapters::NRIMetricAdapter, MetricAdapter, MetricOutput,
};

#[derive(Parser, Debug)]
#[command(author, version, about = "New Relic Infrastructure PostgreSQL Integration", long_about = None)]
struct Args {
    /// Enable metrics collection
    #[arg(long)]
    metrics: bool,
    
    /// Enable inventory collection (not implemented)
    #[arg(long)]
    inventory: bool,
    
    /// Collection mode for specific metric type
    #[arg(long)]
    mode: Option<String>,
}

#[tokio::main]
async fn main() -> Result<()> {
    // Initialize basic logging
    tracing_subscriber::fmt()
        .with_target(false)
        .with_level(true)
        .init();
    
    let args = Args::parse();
    
    if !args.metrics {
        info!("Metrics collection not enabled, exiting");
        return Ok(());
    }
    
    // Build configuration from environment variables (OHI compatibility)
    let config = build_config_from_env()?;
    
    // Create collection engine
    let mut engine = UnifiedCollectionEngine::new(config.clone()).await?;
    
    // Add NRI adapter
    let entity_key = format!("{}:{}", config.host, config.port);
    engine.add_adapter(Box::new(NRIMetricAdapter::new(entity_key)));
    
    // Collect metrics once
    match engine.collect_all_metrics().await {
        Ok(metrics) => {
            // If mode is specified, filter metrics
            let filtered_metrics = if let Some(mode) = args.mode {
                filter_metrics_by_mode(metrics, &mode)
            } else {
                metrics
            };
            
            // Output NRI format to stdout
            let adapter = NRIMetricAdapter::new(format!("{}:{}", config.host, config.port));
            match adapter.adapt(&filtered_metrics).await {
                Ok(output) => {
                    let bytes = output.serialize()?;
                    let json = String::from_utf8_lossy(&bytes);
                    println!("{}", json);
                }
                Err(e) => {
                    error!("Failed to adapt metrics: {}", e);
                    std::process::exit(1);
                }
            }
        }
        Err(e) => {
            error!("Failed to collect metrics: {}", e);
            std::process::exit(1);
        }
    }
    
    Ok(())
}

fn build_config_from_env() -> Result<CollectorConfig> {
    let mut config = CollectorConfig::default();
    
    // Connection settings
    config.host = env::var("HOSTNAME").unwrap_or_else(|_| "localhost".to_string());
    config.port = env::var("PORT")
        .unwrap_or_else(|_| "5432".to_string())
        .parse()
        .unwrap_or(5432);
    
    let username = env::var("USERNAME").unwrap_or_else(|_| "postgres".to_string());
    let password = env::var("PASSWORD").unwrap_or_default();
    let database = env::var("DATABASE").unwrap_or_else(|_| "postgres".to_string());
    
    config.connection_string = format!(
        "postgresql://{}:{}@{}:{}/{}",
        username, password, config.host, config.port, database
    );
    
    // Database list from COLLECTION_LIST
    if let Ok(collection_list) = env::var("COLLECTION_LIST") {
        // Parse JSON format: {"postgres": {"schemas": ["public"]}}
        if let Ok(json) = serde_json::from_str::<serde_json::Value>(&collection_list) {
            if let Some(obj) = json.as_object() {
                config.databases = obj.keys().cloned().collect();
            }
        }
    } else {
        config.databases = vec![database];
    }
    
    // SSL settings
    if env::var("ENABLE_SSL").unwrap_or_default() == "true" {
        config.connection_string.push_str("?sslmode=require");
    }
    
    // Query monitoring settings
    config.query_monitoring_count_threshold = env::var("QUERY_MONITORING_COUNT_THRESHOLD")
        .unwrap_or_else(|_| "20".to_string())
        .parse()
        .unwrap_or(20);
    
    config.query_monitoring_response_time_threshold = env::var("QUERY_MONITORING_RESPONSE_TIME_THRESHOLD")
        .unwrap_or_else(|_| "500".to_string())
        .parse()
        .unwrap_or(500);
    
    // Extended metrics
    config.enable_extended_metrics = env::var("ENABLE_EXTENDED_METRICS")
        .unwrap_or_default() == "true";
    
    config.enable_ebpf = env::var("ENABLE_EBPF")
        .unwrap_or_default() == "true";
    
    config.enable_ash = env::var("ENABLE_ASH")
        .unwrap_or_default() == "true";
    
    // Set to NRI mode
    config.collection_mode = CollectionMode::Nri;
    
    Ok(config)
}

fn filter_metrics_by_mode(
    mut metrics: postgres_unified_collector::UnifiedMetrics,
    mode: &str,
) -> postgres_unified_collector::UnifiedMetrics {
    match mode {
        "slow_queries" => {
            metrics.wait_events.clear();
            metrics.blocking_sessions.clear();
            metrics.individual_queries.clear();
            metrics.execution_plans.clear();
        }
        "wait_events" => {
            metrics.slow_queries.clear();
            metrics.blocking_sessions.clear();
            metrics.individual_queries.clear();
            metrics.execution_plans.clear();
        }
        "blocking_sessions" => {
            metrics.slow_queries.clear();
            metrics.wait_events.clear();
            metrics.individual_queries.clear();
            metrics.execution_plans.clear();
        }
        "individual_queries" => {
            metrics.slow_queries.clear();
            metrics.wait_events.clear();
            metrics.blocking_sessions.clear();
            metrics.execution_plans.clear();
        }
        "execution_plans" => {
            metrics.slow_queries.clear();
            metrics.wait_events.clear();
            metrics.blocking_sessions.clear();
            metrics.individual_queries.clear();
        }
        _ => {} // Return all metrics
    }
    
    metrics
}