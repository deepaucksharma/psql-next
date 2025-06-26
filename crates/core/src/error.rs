use thiserror::Error;

#[derive(Error, Debug)]
pub enum CollectorError {
    #[error("Database connection error: {0}")]
    DatabaseError(#[from] sqlx::Error),
    
    #[error("Configuration error: {0}")]
    ConfigError(String),
    
    #[error("Extension not available: {0}")]
    ExtensionNotAvailable(String),
    
    #[error("Unsupported PostgreSQL version: {0}")]
    UnsupportedVersion(u64),
    
    #[error("Query execution error: {0}")]
    QueryError(String),
    
    #[error("Capability check failed: {0}")]
    CapabilityError(String),
    
    #[error("Collection timeout after {0} seconds")]
    Timeout(u64),
    
    #[error("General error: {0}")]
    General(#[from] anyhow::Error),
}

#[derive(Error, Debug)]
pub enum ProcessError {
    #[error("Serialization error: {0}")]
    SerializationError(#[from] serde_json::Error),
    
    #[error("Adapter error: {0}")]
    AdapterError(String),
    
    #[error("Export error: {0}")]
    ExportError(String),
    
    #[error("Validation error: {0}")]
    ValidationError(String),
    
    #[error("General error: {0}")]
    General(#[from] anyhow::Error),
}