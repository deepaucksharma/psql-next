# PostgreSQL Unified Collector - Developer Documentation

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Code Structure](#code-structure)
3. [Development Setup](#development-setup)
4. [API Documentation](#api-documentation)
5. [Testing Framework](#testing-framework)
6. [Performance Optimization](#performance-optimization)
7. [Troubleshooting](#troubleshooting)
8. [Contributing Guidelines](#contributing-guidelines)
9. [Release Process](#release-process)

## Architecture Overview

### High-Level Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   PostgreSQL    │───▶│  Unified         │───▶│  Output         │
│   Database      │    │  Collector       │    │  Adapters       │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌──────────────────┐    ┌─────────────────┐
                       │  Extensions &    │    │ NRI + OTLP      │
                       │  Query Engine    │    │ Simultaneous    │
                       └──────────────────┘    └─────────────────┘
```

### Core Components

#### 1. Collection Engine (`src/collection_engine.rs`)
The heart of the collector, responsible for:
- **Connection Management**: PostgreSQL connection pooling with configurable limits
- **Extension Detection**: Automatic discovery of PostgreSQL extensions (pg_stat_statements, pg_wait_sampling, etc.)
- **Metrics Collection**: Orchestrates all metric gathering operations
- **Query Sanitization**: PII protection and query text sanitization
- **Adapter Management**: Handles multiple output format adapters simultaneously

```rust
pub struct UnifiedCollectionEngine {
    connection_pool: PgPool,           // PostgreSQL connection pool
    extension_manager: ExtensionManager, // Extension detection and management
    query_executor: OHICompatibleQueryExecutor, // Query execution engine
    ash_sampler: Option<ActiveSessionSampler>, // Active Session History
    pgbouncer_collector: Option<PgBouncerCollector>, // PgBouncer metrics
    config: CollectorConfig,           // Configuration
    adapters: Vec<Box<dyn MetricAdapterDyn>>, // Output adapters
    capabilities: Arc<RwLock<Option<Capabilities>>>, // Cached capabilities
    query_sanitizer: Option<QuerySanitizer>, // Query sanitization
    exporter: MetricExporter,          // HTTP export functionality
}
```

#### 2. Adapter System
**Dynamic Dispatch Pattern**: The collector uses trait objects for adapter management:

```rust
#[async_trait]
pub trait MetricAdapterDyn: Send + Sync {
    async fn adapt_dyn(&self, metrics: &UnifiedMetrics) -> Result<Box<dyn MetricOutputDyn>, ProcessError>;
    fn name(&self) -> &str;
}
```

**Benefits**:
- **Runtime Flexibility**: Add/remove adapters without recompilation
- **Simultaneous Output**: Multiple formats can be generated simultaneously
- **Type Safety**: Compile-time guarantees with runtime flexibility

#### 3. Query Engine (`crates/query-engine/`)
**OHI-Compatible Execution**: Maintains compatibility with existing New Relic Infrastructure queries while adding OTLP support.

**Key Features**:
- **Parameterized Queries**: Prevents SQL injection
- **Extension-Aware**: Adapts queries based on available PostgreSQL extensions
- **Performance Optimized**: Efficient query execution with connection reuse

#### 4. Extension Management (`crates/extensions/`)
**Automatic Discovery**: Runtime detection of PostgreSQL extensions and capabilities.

```rust
pub struct ExtensionConfig {
    pub pg_stat_statements: Option<PgStatStatementsConfig>,
    pub pg_wait_sampling: Option<PgWaitSamplingConfig>,
    pub pg_stat_monitor: Option<PgStatMonitorConfig>,
}
```

### Data Flow Architecture

```
PostgreSQL Extensions → Query Engine → Collection Engine → Adapters → Output
     │                      │              │               │          │
     ▼                      ▼              ▼               ▼          ▼
pg_stat_statements    Parameterized   Metrics        NRI Format   stdout
pg_wait_sampling      SQL Queries     Aggregation    OTLP Format  HTTP/gRPC
pg_stat_monitor       Connection      Sanitization   Custom       File
                      Pooling         Validation     Formats      ...
```

## Code Structure

### Project Layout

```
psql-next/
├── src/                           # Main application source
│   ├── main.rs                    # Application entry point
│   ├── collection_engine.rs       # Core collection logic
│   ├── config.rs                  # Configuration management
│   ├── health.rs                  # Health check endpoints
│   ├── exporter.rs               # HTTP export functionality
│   ├── sanitizer.rs              # Query sanitization
│   └── pgbouncer.rs              # PgBouncer integration
├── crates/                       # Rust workspace crates
│   ├── core/                     # Core types and traits
│   │   ├── src/lib.rs           # Public API definitions
│   │   └── src/types.rs         # Core data structures
│   ├── nri-adapter/             # New Relic Infrastructure adapter
│   │   ├── src/lib.rs           # NRI format implementation
│   │   └── src/serializer.rs    # JSON serialization
│   ├── otel-adapter/            # OpenTelemetry adapter
│   │   ├── src/lib.rs           # OTLP format implementation
│   │   └── src/metrics.rs       # OTLP metrics conversion
│   ├── query-engine/            # SQL query execution
│   │   ├── src/lib.rs           # Query executor
│   │   ├── src/queries/         # SQL query definitions
│   │   └── src/utils.rs         # Query utilities
│   └── extensions/              # PostgreSQL extension handling
│       ├── src/lib.rs           # Extension manager
│       └── src/validators.rs    # Extension validation
├── configs/                     # Configuration files
├── examples/                    # Configuration examples
├── scripts/                     # Operational scripts
├── deployments/                 # Deployment manifests
└── docs/                       # Additional documentation
```

### Module Dependencies

```mermaid
graph TD
    A[main.rs] --> B[collection_engine.rs]
    B --> C[core crate]
    B --> D[nri-adapter]
    B --> E[otel-adapter]
    B --> F[query-engine]
    B --> G[extensions]
    B --> H[config.rs]
    B --> I[sanitizer.rs]
    B --> J[pgbouncer.rs]
    B --> K[exporter.rs]
    A --> L[health.rs]
    
    D --> C
    E --> C
    F --> C
    G --> C
```

### Key Traits and Interfaces

#### MetricAdapter Trait
```rust
#[async_trait]
pub trait MetricAdapter: Send + Sync {
    type Output: MetricOutput;
    
    async fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, ProcessError>;
}
```

#### MetricOutput Trait
```rust
pub trait MetricOutput: Send + Sync {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError>;
    fn content_type(&self) -> &'static str;
}
```

#### PostgresCollector Trait
```rust
#[async_trait]
pub trait PostgresCollector: Send + Sync {
    async fn collect(&self, conn: &mut PgConnection, params: &CommonParameters) 
        -> Result<UnifiedMetrics, CollectorError>;
}
```

## Development Setup

### Prerequisites

1. **Rust Toolchain**:
   ```bash
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
   rustup default stable
   rustup component add clippy rustfmt
   ```

2. **PostgreSQL Development Environment**:
   ```bash
   # macOS
   brew install postgresql
   
   # Ubuntu/Debian
   sudo apt-get install postgresql postgresql-contrib libpq-dev
   
   # Enable extensions
   sudo -u postgres psql -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
   sudo -u postgres psql -c "CREATE EXTENSION IF NOT EXISTS pg_wait_sampling;"
   ```

3. **Docker and Docker Compose**:
   ```bash
   # Install Docker Desktop or Docker Engine + Docker Compose
   docker --version
   docker-compose --version
   ```

### Local Development Setup

1. **Clone and Setup**:
   ```bash
   git clone https://github.com/deepaucksharma/psql-next.git
   cd psql-next
   cp .env.example .env
   ```

2. **Configure Environment**:
   ```bash
   # Edit .env with your settings
   vim .env
   
   # Required variables:
   export POSTGRES_CONNECTION_STRING="postgresql://user:pass@localhost:5432/dbname"
   export NEW_RELIC_API_KEY="your_api_key"
   export NEW_RELIC_ACCOUNT_ID="your_account_id"
   export NEW_RELIC_REGION="US"  # or "EU"
   ```

3. **Build and Test**:
   ```bash
   # Build all crates
   cargo build --release
   
   # Run tests
   cargo test --all
   
   # Run clippy linting
   cargo clippy --all-targets --all-features
   
   # Format code
   cargo fmt
   ```

### IDE Configuration

#### VS Code Setup
Create `.vscode/settings.json`:
```json
{
    "rust-analyzer.checkOnSave.command": "clippy",
    "rust-analyzer.cargo.features": "all",
    "editor.formatOnSave": true,
    "[rust]": {
        "editor.defaultFormatter": "rust-lang.rust-analyzer"
    }
}
```

#### IntelliJ IDEA Setup
1. Install Rust plugin
2. Configure Rust toolchain: Settings → Languages & Frameworks → Rust
3. Enable Clippy: Settings → Editor → Inspections → Rust → Clippy

### Environment Profiles

#### Development Profile
```bash
# Start development stack
./scripts/run.sh start

# Generate test data
./scripts/run.sh test

# Tail logs
docker-compose logs -f postgres-collector
```

#### Testing Profile
```bash
# Run unit tests
cargo test --lib

# Run integration tests
cargo test --test integration

# Run with test data generation
./scripts/run.sh e2e
```

#### Production Profile
```bash
# Build optimized release
cargo build --release --locked

# Run production configuration
RUST_LOG=info ./target/release/postgres-collector \
  --config /etc/postgres-collector/config.toml
```

## API Documentation

### Configuration API

#### TOML Configuration
```toml
# Database connection
[database]
host = "localhost"
port = 5432
username = "postgres"
password = "password"
database = "postgres"
max_connections = 10
connect_timeout_secs = 30

# Collection settings
[collection]
interval_secs = 30
enable_extended_metrics = true
enable_ash = true
ash_sample_interval_secs = 5
ash_retention_hours = 24
ash_max_memory_mb = 100

# Query monitoring
query_monitoring_count_threshold = 100
query_monitoring_response_time_threshold = 1000
sanitize_query_text = true
sanitization_mode = "smart"  # "none", "smart", "full"

# Output formats
[outputs.nri]
enabled = true

[outputs.otlp]
enabled = true
endpoint = "http://localhost:4318"
protocol = "http"  # "http" or "grpc"

[outputs.otlp.headers]
"x-api-key" = "${NEW_RELIC_API_KEY}"

# PgBouncer (optional)
[pgbouncer]
enabled = false
admin_connection_string = "postgresql://pgbouncer:pass@localhost:6432/pgbouncer"

# Health monitoring
[health]
enabled = true
bind_address = "0.0.0.0:8080"
```

#### Environment Variable Override
All configuration values can be overridden with environment variables:
```bash
# Database connection
export POSTGRES_HOST="prod-db.example.com"
export POSTGRES_PORT="5432"
export POSTGRES_USERNAME="collector"
export POSTGRES_PASSWORD="${DB_PASSWORD}"

# New Relic configuration
export NEW_RELIC_API_KEY="${NRQL_API_KEY}"
export NEW_RELIC_ACCOUNT_ID="1234567"
export NEW_RELIC_REGION="EU"

# Collection settings
export COLLECTION_INTERVAL_SECS="60"
export ENABLE_EXTENDED_METRICS="true"
```

### Health Check API

#### Health Endpoint (`GET /health`)
```json
{
  "status": "healthy",
  "timestamp": "2025-06-27T10:30:00Z",
  "last_collection": "2025-06-27T10:29:30Z",
  "metrics_sent": 1500,
  "metrics_failed": 0
}
```

#### Readiness Endpoint (`GET /ready`)
```json
{
  "status": "ready",
  "timestamp": "2025-06-27T10:30:00Z"
}
```

#### Metrics Endpoint (`GET /metrics`)
```
# HELP postgres_collector_collections_total Total number of metric collections
# TYPE postgres_collector_collections_total counter
postgres_collector_collections_total 150

# HELP postgres_collector_collection_duration_seconds Time spent collecting metrics
# TYPE postgres_collector_collection_duration_seconds histogram
postgres_collector_collection_duration_seconds_bucket{le="0.1"} 120
postgres_collector_collection_duration_seconds_bucket{le="0.5"} 145
postgres_collector_collection_duration_seconds_bucket{le="1.0"} 150
postgres_collector_collection_duration_seconds_bucket{le="+Inf"} 150
postgres_collector_collection_duration_seconds_sum 8.5
postgres_collector_collection_duration_seconds_count 150
```

### Command Line Interface

#### Basic Usage
```bash
# Run with default configuration
postgres-collector

# Run with custom config file
postgres-collector --config /path/to/config.toml

# Run with log level
postgres-collector --log-level debug

# Validate configuration
postgres-collector --validate-config --config config.toml

# Show version information
postgres-collector --version
```

#### Advanced Options
```bash
# Override specific configuration
postgres-collector \
  --database-host prod-db.example.com \
  --collection-interval 60 \
  --enable-extended-metrics false

# Run once and exit (useful for cron jobs)
postgres-collector --once

# Dry run (validate without collecting)
postgres-collector --dry-run
```

### Programmatic API

#### Creating a Custom Adapter
```rust
use async_trait::async_trait;
use postgres_collector_core::{MetricAdapter, MetricOutput, UnifiedMetrics, ProcessError};
use serde_json::Value;

pub struct CustomAdapter {
    endpoint: String,
}

#[async_trait]
impl MetricAdapter for CustomAdapter {
    type Output = CustomOutput;
    
    async fn adapt(&self, metrics: &UnifiedMetrics) -> Result<Self::Output, ProcessError> {
        // Transform metrics to custom format
        let custom_data = transform_metrics(metrics)?;
        
        Ok(CustomOutput {
            data: custom_data,
            endpoint: self.endpoint.clone(),
        })
    }
}

pub struct CustomOutput {
    data: Value,
    endpoint: String,
}

impl MetricOutput for CustomOutput {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError> {
        serde_json::to_vec(&self.data)
            .map_err(|e| ProcessError::SerializationError(e.to_string()))
    }
    
    fn content_type(&self) -> &'static str {
        "application/json"
    }
}

fn transform_metrics(metrics: &UnifiedMetrics) -> Result<Value, ProcessError> {
    // Custom transformation logic
    let mut result = serde_json::Map::new();
    
    // Transform slow queries
    if !metrics.slow_queries.is_empty() {
        result.insert("slow_queries".to_string(), 
                     serde_json::to_value(&metrics.slow_queries)?);
    }
    
    // Transform other metrics...
    
    Ok(Value::Object(result))
}
```

#### Using the Collection Engine Programmatically
```rust
use postgres_collector::{
    CollectorConfig, UnifiedCollectionEngine,
    nri_adapter::NRIAdapter,
    otel_adapter::OTELAdapter,
};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Load configuration
    let config = CollectorConfig::from_file("config.toml")?;
    
    // Create collection engine
    let mut engine = UnifiedCollectionEngine::new(config).await?;
    
    // Add adapters
    engine.add_adapter(Box::new(NRIAdapter::new()));
    engine.add_adapter(Box::new(OTELAdapter::new()));
    
    // Collect metrics
    let metrics = engine.collect_all_metrics().await?;
    
    // Send metrics through all adapters
    engine.send_metrics(&metrics).await?;
    
    Ok(())
}
```

## Testing Framework

### Unit Tests

#### Testing Collection Logic
```rust
#[cfg(test)]
mod tests {
    use super::*;
    use sqlx::PgPool;
    use testcontainers::{clients::Cli, images::postgres::Postgres, Container};
    
    #[tokio::test]
    async fn test_slow_query_collection() {
        // Setup test database
        let docker = Cli::default();
        let container = docker.run(Postgres::default());
        let connection_string = format!(
            "postgresql://postgres:password@localhost:{}/postgres",
            container.get_host_port_ipv4(5432)
        );
        
        // Create test data
        let pool = PgPool::connect(&connection_string).await.unwrap();
        setup_test_data(&pool).await;
        
        // Test collection
        let config = test_config(&connection_string);
        let engine = UnifiedCollectionEngine::new(config).await.unwrap();
        let metrics = engine.collect_all_metrics().await.unwrap();
        
        // Assertions
        assert!(!metrics.slow_queries.is_empty());
        assert_eq!(metrics.slow_queries[0].database_name, Some("postgres".to_string()));
    }
    
    async fn setup_test_data(pool: &PgPool) {
        sqlx::query("CREATE EXTENSION IF NOT EXISTS pg_stat_statements")
            .execute(pool)
            .await
            .unwrap();
            
        // Generate slow queries
        for i in 0..10 {
            sqlx::query(&format!("SELECT pg_sleep(0.1); SELECT {}", i))
                .execute(pool)
                .await
                .unwrap();
        }
    }
}
```

#### Testing Adapters
```rust
#[tokio::test]
async fn test_nri_adapter() {
    let adapter = NRIAdapter::new();
    let test_metrics = create_test_metrics();
    
    let output = adapter.adapt(&test_metrics).await.unwrap();
    let serialized = output.serialize().unwrap();
    
    // Validate JSON structure
    let json: serde_json::Value = serde_json::from_slice(&serialized).unwrap();
    assert_eq!(json["protocol_version"], "4");
    assert!(json["data"].is_array());
}

#[tokio::test]
async fn test_otlp_adapter() {
    let adapter = OTELAdapter::new();
    let test_metrics = create_test_metrics();
    
    let output = adapter.adapt(&test_metrics).await.unwrap();
    let serialized = output.serialize().unwrap();
    
    // Validate OTLP protobuf structure
    let metrics_data = prost::Message::decode(&serialized[..]).unwrap();
    // Add specific OTLP validation
}
```

### Integration Tests

#### End-to-End Testing
```rust
#[tokio::test]
async fn test_e2e_collection_and_export() {
    // Setup test environment
    let test_env = TestEnvironment::new().await;
    
    // Start collector
    let config = test_env.create_config();
    let mut engine = UnifiedCollectionEngine::new(config).await.unwrap();
    engine.add_adapter(Box::new(TestAdapter::new()));
    
    // Generate test load
    test_env.generate_slow_queries(10).await;
    
    // Collect and verify
    let metrics = engine.collect_all_metrics().await.unwrap();
    engine.send_metrics(&metrics).await.unwrap();
    
    // Verify metrics were exported
    let exported_data = test_env.get_exported_data().await;
    assert!(!exported_data.is_empty());
}
```

#### Performance Testing
```rust
#[tokio::test]
async fn test_collection_performance() {
    let config = performance_test_config();
    let engine = UnifiedCollectionEngine::new(config).await.unwrap();
    
    let start = std::time::Instant::now();
    
    // Collect metrics multiple times
    for _ in 0..100 {
        let _metrics = engine.collect_all_metrics().await.unwrap();
    }
    
    let duration = start.elapsed();
    
    // Assert performance requirements
    assert!(duration.as_millis() < 10000, "Collection took too long: {:?}", duration);
}
```

### Test Utilities

#### Test Environment Setup
```bash
#!/bin/bash
# scripts/setup-test-env.sh

# Start test PostgreSQL
docker run -d --name test-postgres \
  -e POSTGRES_PASSWORD=test \
  -p 5433:5432 \
  postgres:15-alpine

# Wait for startup
sleep 5

# Enable extensions
docker exec test-postgres psql -U postgres -c "CREATE EXTENSION pg_stat_statements;"
docker exec test-postgres psql -U postgres -c "CREATE EXTENSION pg_wait_sampling;"

# Create test data
docker exec test-postgres psql -U postgres -f /test-data.sql
```

#### Mock Servers
```rust
use wiremock::{MockServer, Mock, ResponseTemplate};
use wiremock::matchers::{method, path};

async fn setup_mock_otlp_server() -> MockServer {
    let mock_server = MockServer::start().await;
    
    Mock::given(method("POST"))
        .and(path("/v1/metrics"))
        .respond_with(ResponseTemplate::new(200))
        .mount(&mock_server)
        .await;
    
    mock_server
}
```

## Performance Optimization

### Memory Management

#### Connection Pooling
```rust
// Optimal pool configuration
let pool = PgPoolOptions::new()
    .max_connections(5)                    // Limit connections
    .acquire_timeout(Duration::from_secs(30)) // Prevent hanging
    .idle_timeout(Duration::from_secs(300))   // Release idle connections
    .max_lifetime(Duration::from_secs(3600))  // Rotate connections
    .connect(&connection_string)
    .await?;
```

#### Memory-Bounded Collections
```rust
// ASH sampler with memory limits
let mut sampler = ActiveSessionSampler::new(
    Duration::from_secs(5),   // Sample interval
    Duration::from_hours(24), // Retention period
);
sampler = sampler.with_memory_limit(100); // 100MB limit
```

#### Query Result Limiting
```rust
// Limit expensive queries
const MAX_SLOW_QUERIES: usize = 1000;
const MAX_EXECUTION_PLANS: usize = 10;

let slow_queries = self.query_executor
    .execute_slow_queries(&mut conn, &params)
    .await?
    .into_iter()
    .take(MAX_SLOW_QUERIES)
    .collect();
```

### CPU Optimization

#### Async Processing
```rust
// Parallel metric collection
let (slow_queries, blocking_sessions, wait_events) = tokio::try_join!(
    self.collect_slow_queries(&mut conn, &params),
    self.collect_blocking_sessions(&mut conn, &params),
    self.collect_wait_events(&mut conn, &params)
)?;
```

#### Efficient Serialization
```rust
// Pre-allocate buffers
let mut buffer = Vec::with_capacity(1024 * 1024); // 1MB buffer
serde_json::to_writer(&mut buffer, &metrics)?;
```

#### Query Optimization
```sql
-- Use indexes for time-based filtering
SELECT queryid, query, calls, mean_exec_time
FROM pg_stat_statements 
WHERE last_updated > NOW() - INTERVAL '5 minutes'
ORDER BY mean_exec_time DESC 
LIMIT 100;

-- Create supporting indexes
CREATE INDEX IF NOT EXISTS idx_pg_stat_statements_last_updated 
ON pg_stat_statements(last_updated);
```

### Network Optimization

#### HTTP/2 and Compression
```rust
// Configure HTTP client for OTLP
let client = reqwest::Client::builder()
    .http2_prior_knowledge()
    .gzip(true)
    .timeout(Duration::from_secs(30))
    .build()?;
```

#### Batch Processing
```rust
// Batch metrics for efficiency
const BATCH_SIZE: usize = 100;

for chunk in metrics.slow_queries.chunks(BATCH_SIZE) {
    let batch = create_batch(chunk);
    self.send_batch(batch).await?;
}
```

### Configuration Tuning

#### Production Configuration
```toml
# High-performance production settings
[collection]
interval_secs = 60  # Reduce frequency for large systems
max_connections = 5  # Limit database load

[outputs.otlp]
batch_size = 1000    # Larger batches
compression = "gzip" # Enable compression
timeout_secs = 30    # Reasonable timeout

[query_monitoring]
count_threshold = 1000        # Higher thresholds
response_time_threshold = 5000 # 5 seconds
```

### Monitoring Performance

#### Built-in Metrics
```rust
// Performance tracking
#[derive(Debug)]
pub struct PerformanceMetrics {
    pub collection_duration: Duration,
    pub serialization_duration: Duration,
    pub export_duration: Duration,
    pub memory_usage: usize,
    pub query_count: usize,
}

// Track in collection engine
let start = Instant::now();
let metrics = self.collect_all_metrics().await?;
let collection_duration = start.elapsed();

info!(
    "Collection completed in {:?}, {} queries processed",
    collection_duration,
    metrics.slow_queries.len()
);
```

#### Resource Monitoring
```bash
# Monitor resource usage
docker stats postgres-collector

# Check memory usage
cargo build --release
valgrind --tool=massif ./target/release/postgres-collector

# Profile CPU usage
perf record -g ./target/release/postgres-collector
perf report
```

## Troubleshooting

### Common Issues

#### Connection Problems

**Issue**: `Connection refused to PostgreSQL`
```bash
# Check PostgreSQL status
sudo systemctl status postgresql

# Verify connection string
psql "postgresql://user:pass@host:port/db"

# Check network connectivity
telnet host port
```

**Issue**: `Too many connections`
```toml
# Reduce max_connections in config
[database]
max_connections = 3  # Reduce from default 10
```

**Issue**: `SSL connection required`
```toml
# Add SSL configuration
[database]
sslmode = "require"
sslcert = "/path/to/client.crt"
sslkey = "/path/to/client.key"
sslrootcert = "/path/to/ca.crt"
```

#### Extension Issues

**Issue**: `pg_stat_statements extension not found`
```sql
-- Install extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Add to postgresql.conf
shared_preload_libraries = 'pg_stat_statements'

-- Restart PostgreSQL
sudo systemctl restart postgresql
```

**Issue**: `Permission denied for pg_stat_statements`
```sql
-- Grant permissions
GRANT SELECT ON pg_stat_statements TO monitoring_user;
GRANT EXECUTE ON FUNCTION pg_stat_statements_reset() TO monitoring_user;
```

#### Memory Issues

**Issue**: `Out of memory errors`
```toml
# Reduce memory usage
[collection]
ash_max_memory_mb = 50  # Reduce from 100
enable_extended_metrics = false

[query_monitoring]
count_threshold = 500   # Reduce from 1000
```

**Issue**: `Memory leaks in long-running processes`
```rust
// Enable memory debugging
RUST_LOG=debug,postgres_collector=trace ./postgres-collector

// Use jemalloc allocator
[dependencies]
jemallocator = "0.5"

// In main.rs
#[global_allocator]
static ALLOC: jemallocator::Jemalloc = jemallocator::Jemalloc;
```

#### OTLP Export Issues

**Issue**: `404 Not Found on OTLP endpoint`
```toml
# Ensure correct endpoint path
[outputs.otlp]
endpoint = "https://otlp.nr-data.net:4318"  # Includes port
# Collector automatically appends /v1/metrics
```

**Issue**: `Authentication failed`
```bash
# Verify API key
curl -H "Api-Key: ${NEW_RELIC_API_KEY}" \
     "https://api.newrelic.com/v2/account.json"

# Check account ID
echo $NEW_RELIC_ACCOUNT_ID
```

**Issue**: `Timeout errors`
```toml
[outputs.otlp]
timeout_secs = 60  # Increase timeout
retry_attempts = 3  # Add retries
```

### Debugging Tools

#### Logging Configuration
```bash
# Enable debug logging
export RUST_LOG=debug

# Specific module logging
export RUST_LOG=postgres_collector::collection_engine=trace

# JSON structured logging
export RUST_LOG_FORMAT=json
```

#### SQL Query Debugging
```sql
-- Monitor active queries
SELECT pid, query, state, query_start 
FROM pg_stat_activity 
WHERE query NOT ILIKE '%pg_stat_activity%';

-- Check slow queries
SELECT query, calls, mean_exec_time, rows
FROM pg_stat_statements 
ORDER BY mean_exec_time DESC 
LIMIT 10;

-- Monitor locks
SELECT 
    blocked_locks.pid AS blocked_pid,
    blocked_activity.usename AS blocked_user,
    blocking_locks.pid AS blocking_pid,
    blocking_activity.usename AS blocking_user,
    blocked_activity.query AS blocked_statement,
    blocking_activity.query AS current_statement_in_blocking_process
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;
```

#### Network Debugging
```bash
# Test OTLP connectivity
curl -X POST \
  -H "Content-Type: application/x-protobuf" \
  -H "Api-Key: ${NEW_RELIC_API_KEY}" \
  --data-binary "@test-metrics.pb" \
  "https://otlp.nr-data.net:4318/v1/metrics"

# Monitor network traffic
sudo tcpdump -i any -A host otlp.nr-data.net

# Check DNS resolution
nslookup otlp.nr-data.net
```

#### Container Debugging
```bash
# Check container logs
docker-compose logs -f postgres-collector

# Enter container for debugging
docker-compose exec postgres-collector /bin/bash

# Check container resources
docker stats postgres-collector

# Inspect container configuration
docker inspect postgres-collector
```

### Error Reference

#### Common Error Codes
- `E001`: Database connection failed
- `E002`: Extension not available
- `E003`: Query execution timeout
- `E004`: Serialization error
- `E005`: OTLP export failed
- `E006`: Configuration validation error
- `E007`: Memory limit exceeded
- `E008`: Authentication failed

#### Recovery Procedures

**Automatic Recovery**:
```rust
// Implement retry logic
const MAX_RETRIES: usize = 3;
const RETRY_DELAY: Duration = Duration::from_secs(5);

for attempt in 1..=MAX_RETRIES {
    match self.collect_metrics().await {
        Ok(metrics) => return Ok(metrics),
        Err(e) if attempt < MAX_RETRIES => {
            warn!("Collection failed (attempt {}): {}", attempt, e);
            tokio::time::sleep(RETRY_DELAY).await;
        }
        Err(e) => return Err(e),
    }
}
```

**Manual Recovery**:
```bash
# Restart collector
docker-compose restart postgres-collector

# Clear cache and restart
docker-compose down
docker system prune -f
docker-compose up -d

# Reset PostgreSQL statistics
sudo -u postgres psql -c "SELECT pg_stat_statements_reset();"
```

## Contributing Guidelines

### Code Standards

#### Rust Style Guide
- Use `cargo fmt` for formatting
- Follow [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/)
- Use `cargo clippy` for linting
- Maintain test coverage > 80%

#### Commit Message Format
```
type(scope): short description

Longer description if needed.

- Bullet points for multiple changes
- Use present tense ("add" not "added")
- Reference issues (#123)
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

#### Pull Request Process
1. Fork the repository
2. Create feature branch: `git checkout -b feature/new-adapter`
3. Make changes with tests
4. Run quality checks: `cargo test && cargo clippy && cargo fmt`
5. Update documentation
6. Submit pull request with clear description

### Testing Requirements

#### Required Tests
- Unit tests for all public functions
- Integration tests for adapters
- End-to-end tests for complete workflows
- Performance regression tests

#### Test Structure
```rust
#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_unit_functionality() {
        // Unit test
    }
    
    #[tokio::test]
    async fn test_async_functionality() {
        // Async unit test
    }
    
    #[test]
    #[ignore = "integration"]
    fn test_integration() {
        // Integration test (run with --ignored)
    }
}
```

### Documentation Requirements

- Public APIs must have rustdoc comments
- Include usage examples in docstrings
- Update README.md for user-facing changes
- Add architecture decision records (ADRs) for major changes

### Release Process

#### Version Numbering
Follow [Semantic Versioning](https://semver.org/):
- `MAJOR.MINOR.PATCH`
- Major: Breaking changes
- Minor: New features (backward compatible)
- Patch: Bug fixes

#### Release Checklist
1. Update version in `Cargo.toml`
2. Update `CHANGELOG.md`
3. Run full test suite
4. Build release artifacts
5. Create git tag: `git tag v1.2.3`
6. Push tag: `git push origin v1.2.3`
7. Create GitHub release with artifacts

#### Continuous Integration

**GitHub Actions Workflow**:
```yaml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
    - uses: actions/checkout@v3
    - uses: actions-rs/toolchain@v1
      with:
        toolchain: stable
    - run: cargo test --all
    - run: cargo clippy -- -D warnings
    - run: cargo fmt -- --check
```

## Conclusion

This documentation provides a comprehensive guide for developers working on the PostgreSQL Unified Collector. For additional questions or clarifications, please:

1. Check the [GitHub Issues](https://github.com/deepaucksharma/psql-next/issues)
2. Review existing [Pull Requests](https://github.com/deepaucksharma/psql-next/pulls)
3. Join discussions in [GitHub Discussions](https://github.com/deepaucksharma/psql-next/discussions)

Happy coding! 🚀