pub mod collection_engine;
pub mod config;
pub mod exporter;
pub mod health;
pub mod metrics;
pub mod pgbouncer;
pub mod multi_instance;
pub mod sanitizer;

// Re-export specific items to avoid conflicts
pub use collection_engine::UnifiedCollectionEngine;
pub use config::{CollectorConfig, OutputConfig, OTLPOutputConfig};

// Re-export core types selectively
pub use postgres_collector_core::{
    UnifiedMetrics, ProcessError, MetricOutput,
    SlowQueryMetric, WaitEventMetric, BlockingSessionMetric,
    IndividualQueryMetric, ExecutionPlanMetric,
    Capabilities, CollectorError, CommonParameters,
    ExtensionInfo, MetricBatch, PostgresCollector
};