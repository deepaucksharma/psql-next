use anyhow::Result;
use clap::Parser;
use std::time::Duration;
use tokio::time;
use tracing::{error, info};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

use postgres_unified_collector::{
    UnifiedCollectionEngine, CollectorConfig, CollectionMode,
    adapters::OTelMetricAdapter,
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
    config.collection_mode = CollectionMode::Otel;
    
    // Override endpoint if specified
    if let Some(endpoint) = args.endpoint {
        if let Some(otlp_config) = &mut config.outputs.otlp {
            otlp_config.endpoint = endpoint;
        }
    }
    
    // Validate configuration
    config.validate().map_err(|e| anyhow::anyhow!(e))?;
    
    info!("Configuration loaded successfully");
    info!("OTLP endpoint: {}", 
        config.outputs.otlp.as_ref()
            .map(|c| c.endpoint.as_str())
            .unwrap_or("not configured")
    );
    
    // Create collection engine
    let mut engine = UnifiedCollectionEngine::new(config.clone()).await?;
    
    // Add OTel adapter
    if let Some(otlp_config) = &config.outputs.otlp {
        if otlp_config.enabled && !args.console {
            engine.add_adapter(Box::new(OTelMetricAdapter::new(
                otlp_config.endpoint.clone()
            )));
            info!("OTel adapter enabled");
        }
    }
    
    // Skip OpenTelemetry SDK setup for now - just use the adapter
    info!("OpenTelemetry provider setup skipped - using simplified adapter");
    
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