use anyhow::Result;
use clap::Parser;
use std::time::Duration;
use tokio::time;
use tracing::{error, info};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

use postgres_otel_collector::{
    UnifiedCollectionEngine, CollectorConfig,
    metrics::{init_new_relic_metrics, NewRelicConfig, DimensionalMetrics, create_resource},
};

#[derive(Parser, Debug)]
#[command(author, version, about = "PostgreSQL OpenTelemetry Collector", long_about = None)]
struct Args {
    /// Path to configuration file
    #[arg(short, long, default_value = "otel-config.toml")]
    config: String,
    
    /// OTLP endpoint override
    #[arg(long, env = "OTLP_ENDPOINT")]
    endpoint: Option<String>,
    
    /// Enable debug logging
    #[arg(short, long)]
    debug: bool,
    
    /// Export to console instead of OTLP
    #[arg(long)]
    console: bool,
}

#[tokio::main]
async fn main() -> Result<()> {
    let args = Args::parse();
    
    // Initialize tracing with OpenTelemetry
    init_telemetry(&args)?;
    
    info!("Starting PostgreSQL OpenTelemetry Collector");
    
    // Load configuration
    let mut config = CollectorConfig::from_file(&args.config)?;
    
    // Override endpoint if specified
    if let Some(endpoint) = args.endpoint {
        config.outputs.otlp.endpoint = endpoint;
    }
    
    // Validate configuration
    config.validate().map_err(|e| anyhow::anyhow!(e))?;
    
    info!("Configuration loaded successfully");
    info!("OTLP endpoint: {}", config.outputs.otlp.endpoint);
    
    // Create collection engine
    let mut engine = UnifiedCollectionEngine::new(config.clone()).await?;
    
    // Check if OTLP is enabled
    if !config.outputs.otlp.enabled {
        return Err(anyhow::anyhow!("OTLP output is disabled in configuration"));
    }
    
    if args.console {
        info!("Console mode enabled - metrics will be printed to stdout");
    }
    
    // Initialize New Relic OpenTelemetry metrics
    if !args.console {
        let newrelic_config = NewRelicConfig {
            api_key: config.outputs.otlp.newrelic_api_key.clone()
                .ok_or_else(|| anyhow::anyhow!("NEW_RELIC_API_KEY is required"))?,
            region: config.outputs.otlp.newrelic_region.clone(),
            endpoint_override: if let Some(endpoint) = args.endpoint {
                Some(endpoint)
            } else {
                None
            },
        };
        
        // Create resource attributes
        let resource = create_resource(&config);
        
        // Initialize New Relic metrics
        let meter_provider = init_new_relic_metrics(newrelic_config, resource)?;
        
        // Create dimensional metrics
        let dimensional_metrics = DimensionalMetrics::new(&meter_provider);
        
        // Set dimensional metrics in the collection engine
        engine.set_dimensional_metrics(dimensional_metrics);
        
        info!("New Relic OpenTelemetry metrics initialized");
    } else {
        info!("Console mode - skipping OpenTelemetry setup");
    }
    
    // Start collection loop
    let mut interval = time::interval(Duration::from_secs(config.collection_interval_secs));
    info!("Starting collection loop with {}s interval", config.collection_interval_secs);
    
    // Handle shutdown gracefully
    let mut shutdown_handle = tokio::spawn(async move {
        tokio::signal::ctrl_c().await.ok();
        info!("Shutting down...");
    });
    
    loop {
        tokio::select! {
            _ = interval.tick() => {
                match engine.collect_all_metrics().await {
                    Ok(metrics) => {
                        info!(
                            "Collected metrics: {} slow queries, {} wait events, {} blocking sessions",
                            metrics.slow_queries.len(),
                            metrics.wait_events.len(),
                            metrics.blocking_sessions.len()
                        );
                        
                        if args.console {
                            // Print to console
                            println!("Collected {} slow queries, {} wait events, {} blocking sessions",
                                metrics.slow_queries.len(),
                                metrics.wait_events.len(),
                                metrics.blocking_sessions.len()
                            );
                            // Could also print actual metrics JSON here if needed
                        } else {
                            // Send metrics via OTLP
                            match engine.send_metrics(&metrics).await {
                                Ok(_) => info!("Metrics sent successfully"),
                                Err(e) => error!("Failed to send metrics: {}", e),
                            }
                        }
                    }
                    Err(e) => {
                        error!("Failed to collect metrics: {}", e);
                    }
                }
            }
            _ = &mut shutdown_handle => {
                info!("Shutdown complete");
                break;
            }
        }
    }
    
    Ok(())
}

fn init_telemetry(args: &Args) -> Result<()> {
    let level = if args.debug {
        tracing::Level::DEBUG
    } else {
        tracing::Level::INFO
    };
    
    // Set up tracing subscriber
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::fmt::layer()
                .with_target(false)
                .with_thread_ids(true)
                .with_level(true)
        )
        .with(tracing_subscriber::filter::LevelFilter::from_level(level))
        .init();
    
    Ok(())
}