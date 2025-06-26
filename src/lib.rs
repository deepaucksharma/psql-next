pub mod collection_engine;
pub mod config;
pub mod adapters;

pub use collection_engine::*;
pub use config::*;

// Re-export core types
pub use postgres_collector_core::*;
pub use postgres_query_engine as query_engine;
pub use postgres_extensions as extensions;
pub use postgres_nri_adapter as nri_adapter;
pub use postgres_otel_adapter as otel_adapter;