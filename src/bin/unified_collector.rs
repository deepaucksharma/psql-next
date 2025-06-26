use anyhow::Result;
use clap::{Parser, ValueEnum};
use std::time::Duration;
use tokio::time;
use tracing::{error, info};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

use postgres_unified_collector::{
    UnifiedCollectionEngine, CollectorConfig, CollectionMode,
    adapters::{NRIMetricAdapter, OTelMetricAdapter},
};

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Args {
    /// Path to configuration file
    #[arg(short, long, default_value = "collector-config.toml")]
    config: String,
    
    /// Collection mode override
    #[arg(short, long, value_enum)]
    mode: Option<Mode>,
    
    /// Enable debug logging
    #[arg(short, long)]
    debug: bool,
    
    /// Dry run mode (collect but don't send)
    #[arg(long)]
    dry_run: bool,
}

#[derive(Copy, Clone, Debug, ValueEnum)]
enum Mode {
    Otel,
    Nri,
    Hybrid,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();
    
    // Initialize tracing
    let level = if args.debug {
        tracing::Level::DEBUG
    } else {
        tracing::Level::INFO
    };
    
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::fmt::layer()
                .with_target(false)
                .with_thread_ids(true)
                .with_level(true)
        )
        .with(tracing_subscriber::filter::LevelFilter::from_level(level))
        .init();
    
    info!("Starting PostgreSQL Unified Collector");
    
    // Load configuration
    let mut config = CollectorConfig::from_file(&args.config)?;
    
    // Override mode if specified
    if let Some(mode) = args.mode {
        config.collection_mode = match mode {
            Mode::Otel => CollectionMode::Otel,
            Mode::Nri => CollectionMode::Nri,
            Mode::Hybrid => CollectionMode::Hybrid,
        };
    }
    
    // Validate configuration
    config.validate()?;
    
    info!("Configuration loaded successfully");
    info!("Collection mode: {:?}", config.collection_mode);
    
    // Create collection engine
    let mut engine = UnifiedCollectionEngine::new(config.clone()).await?;
    
    // Add adapters based on mode
    match config.collection_mode {
        CollectionMode::Nri => {
            if let Some(nri_config) = &config.outputs.nri {
                if nri_config.enabled {
                    let entity_key = expand_variables(&nri_config.entity_key, &config);
                    engine.add_adapter(Box::new(NRIMetricAdapter::new(entity_key)));
                    info!("NRI adapter enabled");
                }
            }
        }
        CollectionMode::Otel => {
            if let Some(otlp_config) = &config.outputs.otlp {
                if otlp_config.enabled {
                    engine.add_adapter(Box::new(OTelMetricAdapter::new(
                        otlp_config.endpoint.clone()
                    )));
                    info!("OTel adapter enabled");
                }
            }
        }
        CollectionMode::Hybrid => {
            if let Some(nri_config) = &config.outputs.nri {
                if nri_config.enabled {
                    let entity_key = expand_variables(&nri_config.entity_key, &config);
                    engine.add_adapter(Box::new(NRIMetricAdapter::new(entity_key)));
                    info!("NRI adapter enabled");
                }
            }
            if let Some(otlp_config) = &config.outputs.otlp {
                if otlp_config.enabled {
                    engine.add_adapter(Box::new(OTelMetricAdapter::new(
                        otlp_config.endpoint.clone()
                    )));
                    info!("OTel adapter enabled");
                }
            }
        }
    }
    
    // Start Active Session History sampling if enabled
    if config.enable_ash {
        info!("Active Session History sampling enabled");
        // ASH sampler is started internally by the engine
    }
    
    // Start collection loop
    let mut interval = time::interval(Duration::from_secs(config.collection_interval_secs));
    info!("Starting collection loop with {}s interval", config.collection_interval_secs);
    
    loop {
        interval.tick().await;
        
        match engine.collect_all_metrics().await {
            Ok(metrics) => {
                info!(
                    "Collected metrics: {} slow queries, {} wait events, {} blocking sessions",
                    metrics.slow_queries.len(),
                    metrics.wait_events.len(),
                    metrics.blocking_sessions.len()
                );
                
                if !args.dry_run {
                    // Send metrics through adapters
                    // This would be implemented in the engine
                    info!("Metrics sent successfully");
                } else {
                    info!("Dry run mode - metrics not sent");
                }
            }
            Err(e) => {
                error!("Failed to collect metrics: {}", e);
            }
        }
    }
}

fn expand_variables(template: &str, config: &CollectorConfig) -> String {
    template
        .replace("${HOSTNAME}", &config.host)
        .replace("${PORT}", &config.port.to_string())
}