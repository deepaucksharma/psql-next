use anyhow::Result;
use clap::{Parser, ValueEnum};
use std::time::Duration;
use std::sync::Arc;
use std::net::SocketAddr;
use tokio::time;
use tokio::sync::RwLock;
use tokio::signal;
use tracing::{error, info, warn};
use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

use postgres_unified_collector::{
    UnifiedCollectionEngine, CollectorConfig, CollectionMode,
    adapters::{NRIMetricAdapter, OTelMetricAdapter},
    health::{HealthServer, HealthStatus},
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
    
    /// Health check server address
    #[arg(long, default_value = "0.0.0.0:8080")]
    health_addr: String,
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
    config.validate().map_err(|e| anyhow::anyhow!(e))?;
    
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
    
    // Start health check server
    let health_status = Arc::new(RwLock::new(HealthStatus::default()));
    let health_server = HealthServer::new(health_status.clone());
    let health_addr: SocketAddr = args.health_addr.parse()?;
    
    tokio::spawn(async move {
        if let Err(e) = health_server.start(health_addr).await {
            error!("Health server error: {}", e);
        }
    });
    
    info!("Health check server started on {}", args.health_addr);
    
    // Start collection loop with graceful shutdown
    let mut interval = time::interval(Duration::from_secs(config.collection_interval_secs));
    info!("Starting collection loop with {}s interval", config.collection_interval_secs);
    
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
                
                // Update health status
                {
                    let mut status = health_status.write().await;
                    status.last_collection_time = Some(chrono::Utc::now());
                    status.is_healthy = true;
                    status.last_collection_error = None;
                }
                
                if !args.dry_run {
                    // Send metrics through adapters
                    match engine.send_metrics(&metrics).await {
                        Ok(_) => {
                            info!("Metrics sent successfully through all adapters");
                            let mut status = health_status.write().await;
                            status.metrics_sent += 1;
                        }
                        Err(e) => {
                            error!("Failed to send metrics: {}", e);
                            let mut status = health_status.write().await;
                            status.metrics_failed += 1;
                        }
                    }
                } else {
                    info!("Dry run mode - metrics not sent");
                    info!("Would have sent metrics to {} adapters", 
                        match config.collection_mode {
                            CollectionMode::Nri => 1,
                            CollectionMode::Otel => 1,
                            CollectionMode::Hybrid => 2,
                        }
                    );
                }
                    }
                    Err(e) => {
                error!("Failed to collect metrics: {}", e);
                let mut status = health_status.write().await;
                status.is_healthy = false;
                status.last_collection_error = Some(e.to_string());
                    }
                }
            }
            _ = signal::ctrl_c() => {
                info!("Received shutdown signal, gracefully stopping...");
                break;
            }
        }
    }
    
    info!("PostgreSQL Unified Collector shutdown complete");
    Ok(())
}

fn expand_variables(template: &str, config: &CollectorConfig) -> String {
    template
        .replace("${HOSTNAME}", &config.host)
        .replace("${PORT}", &config.port.to_string())
}