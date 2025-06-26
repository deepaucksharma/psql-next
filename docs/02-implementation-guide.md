# PostgreSQL Unified Collector - Implementation Guide

## Table of Contents
1. [Getting Started](#getting-started)
2. [Building from Source](#building-from-source)
3. [Core Implementation Details](#core-implementation-details)
4. [Adding New Metrics](#adding-new-metrics)
5. [Extending the Collector](#extending-the-collector)
6. [Testing Strategy](#testing-strategy)
7. [Performance Optimization](#performance-optimization)
8. [Troubleshooting](#troubleshooting)

## Getting Started

### Prerequisites
- Rust 1.70 or later
- PostgreSQL 12+ (for testing)
- Docker (optional, for containerized testing)
- Make (optional, for build automation)

### Quick Start
```bash
# Clone the repository
git clone https://github.com/newrelic/postgres-unified-collector
cd postgres-unified-collector

# Build the project
cargo build --release

# Run tests
cargo test

# Run with default configuration
./target/release/postgres-unified-collector --config config.toml
```

## Building from Source

### Standard Build
```bash
# Development build with all features
cargo build

# Release build with optimizations
cargo build --release

# Build with specific features
cargo build --release --features "nri,otlp,extended-metrics"

# Build without eBPF support (for compatibility)
cargo build --release --no-default-features --features "nri,otlp"
```

### Cross-Compilation
```bash
# For Linux x86_64 (most common)
cargo build --release --target x86_64-unknown-linux-gnu

# For ARM64 (AWS Graviton, Apple Silicon)
cargo build --release --target aarch64-unknown-linux-gnu

# For Alpine Linux (musl)
cargo build --release --target x86_64-unknown-linux-musl
```

### Docker Build
```dockerfile
# Multi-stage build for minimal image
FROM rust:1.70 as builder
WORKDIR /app
COPY . .
RUN cargo build --release

FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/target/release/postgres-unified-collector /usr/local/bin/
ENTRYPOINT ["postgres-unified-collector"]
```

## Core Implementation Details

### 1. Project Structure
```
postgres-unified-collector/
├── Cargo.toml                 # Workspace configuration
├── crates/
│   ├── core/                  # Core types and traits
│   │   ├── src/
│   │   │   ├── lib.rs
│   │   │   ├── models.rs      # UnifiedMetrics, metric types
│   │   │   └── errors.rs      # Error types
│   │   └── Cargo.toml
│   ├── nri-adapter/           # New Relic Infrastructure adapter
│   │   ├── src/
│   │   │   ├── lib.rs
│   │   │   ├── converter.rs   # Metric conversion
│   │   │   └── protocol.rs    # NRI JSON protocol
│   │   └── Cargo.toml
│   ├── otel-adapter/          # OpenTelemetry adapter
│   │   ├── src/
│   │   │   ├── lib.rs
│   │   │   └── exporter.rs    # OTLP export
│   │   └── Cargo.toml
│   ├── query-engine/          # SQL query execution
│   │   ├── src/
│   │   │   ├── lib.rs
│   │   │   ├── queries.rs     # OHI-compatible queries
│   │   │   └── executor.rs    # Query execution logic
│   │   └── Cargo.toml
│   └── extensions/            # PostgreSQL extension support
│       ├── src/
│       │   ├── lib.rs
│       │   ├── detector.rs    # Extension detection
│       │   └── ash.rs         # Active Session History
│       └── Cargo.toml
├── src/
│   ├── main.rs                # Binary entry point
│   ├── config.rs              # Configuration management
│   └── collection_engine.rs   # Main collection orchestration
└── tests/                     # Integration tests
```

### 2. Collection Engine Implementation

The collection engine is the core component that orchestrates metric collection:

```rust
// src/collection_engine.rs
pub struct UnifiedCollectionEngine {
    connection_pool: PgPool,
    extension_manager: ExtensionManager,
    query_executor: OHICompatibleQueryExecutor,
    ash_sampler: Option<ActiveSessionSampler>,
    config: CollectorConfig,
    capabilities: Arc<RwLock<Option<Capabilities>>>,
}

impl UnifiedCollectionEngine {
    pub async fn collect_all_metrics(&self) -> Result<UnifiedMetrics, CollectorError> {
        // 1. Detect capabilities
        let caps = self.detect_capabilities().await?;
        
        // 2. Build parameters matching OHI
        let params = self.build_common_parameters(&caps);
        
        // 3. Collect metrics based on capabilities
        let mut metrics = UnifiedMetrics::default();
        
        if caps.has_extension("pg_stat_statements") {
            metrics.slow_queries = self.collect_slow_queries(&params).await?;
        }
        
        if caps.has_extension("pg_wait_sampling") && !caps.is_rds {
            metrics.wait_events = self.collect_wait_events(&params).await?;
        }
        
        // 4. Enrich with extended metrics if enabled
        if self.config.enable_extended_metrics {
            self.collect_extended_metrics(&mut metrics, &caps).await?;
        }
        
        Ok(metrics)
    }
}
```

### 3. Query Compatibility Layer

Maintaining OHI compatibility requires exact query matching:

```rust
// crates/query-engine/src/queries.rs
pub mod ohi_queries {
    pub const SLOW_QUERIES_V13_ABOVE: &str = r#"
        SELECT 'newrelic' as newrelic,
            pss.queryid AS query_id,
            LEFT(pss.query, 4095) AS query_text,
            pd.datname AS database_name,
            current_schema() AS schema_name,
            pss.calls AS execution_count,
            ROUND((pss.total_exec_time / pss.calls)::numeric, 3) AS avg_elapsed_time_ms,
            pss.shared_blks_read / pss.calls AS avg_disk_reads,
            pss.shared_blks_written / pss.calls AS avg_disk_writes,
            CASE
                WHEN pss.query ILIKE 'SELECT%%' THEN 'SELECT'
                WHEN pss.query ILIKE 'INSERT%%' THEN 'INSERT'
                WHEN pss.query ILIKE 'UPDATE%%' THEN 'UPDATE'
                WHEN pss.query ILIKE 'DELETE%%' THEN 'DELETE'
                ELSE 'OTHER'
            END AS statement_type,
            to_char(NOW() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS collection_timestamp
        FROM pg_stat_statements pss
        JOIN pg_database pd ON pss.dbid = pd.oid
        WHERE pd.datname = current_database()
            AND pss.query NOT ILIKE 'EXPLAIN%%'
        ORDER BY avg_elapsed_time_ms DESC
        LIMIT $1
    "#;
}
```

### 4. Adapter Implementation

Each output format has its own adapter:

```rust
// crates/nri-adapter/src/converter.rs
pub struct NRIAdapter {
    entity_key: String,
    integration_version: String,
}

impl NRIAdapter {
    pub fn adapt(&self, metrics: &UnifiedMetrics) -> Result<String, AdapterError> {
        let mut integration = Integration::new("com.newrelic.postgresql", "2.0.0");
        let entity = integration.entity(&self.entity_key, "pg-instance")?;
        
        // Convert each metric type to NRI format
        for metric in &metrics.slow_queries {
            let metric_set = entity.new_metric_set("PostgresSlowQueries");
            self.populate_slow_query_metrics(&metric_set, metric)?;
        }
        
        Ok(integration.to_json()?)
    }
}
```

## Adding New Metrics

### 1. Define the Metric Structure
```rust
// crates/core/src/models.rs
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CustomMetric {
    pub metric_name: String,
    pub value: f64,
    pub labels: HashMap<String, String>,
    pub timestamp: DateTime<Utc>,
}
```

### 2. Add Collection Logic
```rust
// src/collection_engine.rs
impl UnifiedCollectionEngine {
    async fn collect_custom_metrics(&self, conn: &mut PgConnection) -> Result<Vec<CustomMetric>> {
        let query = r#"
            SELECT 
                metric_name,
                metric_value,
                labels
            FROM custom_metrics_view
        "#;
        
        sqlx::query_as::<_, CustomMetric>(query)
            .fetch_all(conn)
            .await
            .map_err(Into::into)
    }
}
```

### 3. Update Unified Metrics
```rust
// crates/core/src/models.rs
pub struct UnifiedMetrics {
    // ... existing fields ...
    
    #[serde(skip_serializing_if = "Option::is_none")]
    pub custom_metrics: Option<Vec<CustomMetric>>,
}
```

### 4. Add Adapter Support
```rust
// crates/nri-adapter/src/converter.rs
impl NRIAdapter {
    fn adapt_custom_metrics(&self, entity: &mut Entity, metrics: &[CustomMetric]) {
        for metric in metrics {
            let metric_set = entity.new_metric_set("CustomMetrics");
            metric_set.add_metric(&metric.metric_name, metric.value, MetricType::GAUGE);
            
            for (key, value) in &metric.labels {
                metric_set.add_attribute(key, value);
            }
        }
    }
}
```

## Extending the Collector

### 1. Adding a New Extension
```rust
// crates/extensions/src/custom_extension.rs
pub struct CustomExtension {
    name: String,
    min_version: i32,
}

impl Extension for CustomExtension {
    fn name(&self) -> &str {
        &self.name
    }
    
    fn is_available(&self, conn: &PgConnection) -> Result<bool> {
        let result = sqlx::query!(
            "SELECT 1 FROM pg_extension WHERE extname = $1",
            self.name
        )
        .fetch_optional(conn)
        .await?;
        
        Ok(result.is_some())
    }
    
    fn collect_metrics(&self, conn: &PgConnection) -> Result<Vec<Metric>> {
        // Extension-specific collection logic
    }
}
```

### 2. Adding eBPF Support
```rust
// crates/extensions/src/ebpf/mod.rs
#[cfg(feature = "ebpf")]
use aya::{Bpf, programs::UProbe, maps::perf::AsyncPerfEventArray};

pub struct PostgresEBPF {
    bpf: Bpf,
    query_latency_events: AsyncPerfEventArray<MapData>,
}

#[cfg(feature = "ebpf")]
impl PostgresEBPF {
    pub fn attach_to_postgres(&mut self, pid: i32) -> Result<()> {
        let program: &mut UProbe = self.bpf
            .program_mut("trace_query_start")
            .unwrap()
            .try_into()?;
        
        program.load()?;
        program.attach(
            Some("exec_simple_query"),
            0,
            "/usr/lib/postgresql/15/bin/postgres",
            Some(pid)
        )?;
        
        Ok(())
    }
}
```

### 3. Custom Sampling Rules
```rust
// src/sampling.rs
pub struct SamplingRule {
    pub name: String,
    pub condition: Box<dyn Fn(&UnifiedMetrics) -> bool>,
    pub sample_rate: f64,
}

impl SamplingEngine {
    pub fn add_rule(&mut self, rule: SamplingRule) {
        self.rules.push(rule);
    }
    
    pub fn should_sample(&self, metrics: &UnifiedMetrics) -> bool {
        for rule in &self.rules {
            if (rule.condition)(metrics) {
                return rand::random::<f64>() < rule.sample_rate;
            }
        }
        true
    }
}
```

## Testing Strategy

### 1. Unit Tests
```rust
#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_ohi_field_compatibility() {
        let metric = SlowQueryMetric {
            newrelic: Some("newrelic".to_string()),
            query_id: Some("12345".to_string()),
            // ... all OHI fields
        };
        
        let json = serde_json::to_value(&metric).unwrap();
        
        // Verify all OHI fields are present
        assert_eq!(json["newrelic"], "newrelic");
        assert_eq!(json["query_id"], "12345");
    }
}
```

### 2. Integration Tests
```rust
// tests/integration_test.rs
#[tokio::test]
async fn test_full_collection_cycle() {
    let config = CollectorConfig::from_file("tests/config/test.toml").unwrap();
    let engine = UnifiedCollectionEngine::new(config).await.unwrap();
    
    let metrics = engine.collect_all_metrics().await.unwrap();
    
    assert!(!metrics.slow_queries.is_empty());
    assert_eq!(metrics.slow_queries[0].newrelic, Some("newrelic".to_string()));
}
```

### 3. Performance Tests
```rust
#[bench]
fn bench_metric_collection(b: &mut Bencher) {
    let rt = tokio::runtime::Runtime::new().unwrap();
    let engine = setup_test_engine();
    
    b.iter(|| {
        rt.block_on(async {
            let _ = engine.collect_all_metrics().await;
        });
    });
}
```

## Performance Optimization

### 1. Connection Pooling
```rust
let pool = PgPoolOptions::new()
    .max_connections(5)
    .min_connections(2)
    .acquire_timeout(Duration::from_secs(30))
    .idle_timeout(Duration::from_secs(600))
    .connect(&database_url)
    .await?;
```

### 2. Query Optimization
- Use prepared statements
- Batch queries when possible
- Add appropriate indexes
- Use EXPLAIN ANALYZE for query tuning

### 3. Memory Management
```rust
// Pre-allocate collections
let mut metrics = Vec::with_capacity(expected_count);

// Use streaming for large result sets
let mut stream = sqlx::query_as::<_, Metric>(query).fetch(&pool);
while let Some(metric) = stream.try_next().await? {
    process_metric(metric);
}
```

### 4. Async Optimization
```rust
// Collect metrics in parallel
let (slow_queries, wait_events, blocking) = tokio::join!(
    collect_slow_queries(&pool),
    collect_wait_events(&pool),
    collect_blocking_sessions(&pool)
);
```

## Troubleshooting

### Common Issues

#### 1. Connection Failures
```bash
# Check connectivity
postgres-unified-collector test-connection --config config.toml

# Enable debug logging
RUST_LOG=debug postgres-unified-collector --config config.toml
```

#### 2. Missing Metrics
```sql
-- Verify extensions
SELECT * FROM pg_extension WHERE extname IN ('pg_stat_statements', 'pg_wait_sampling');

-- Check permissions
SELECT has_database_privilege(current_user, current_database(), 'CONNECT');
```

#### 3. Performance Issues
```bash
# Profile CPU usage
perf record -g postgres-unified-collector --config config.toml
perf report

# Memory profiling
valgrind --tool=massif postgres-unified-collector --config config.toml
```

### Debug Mode
```toml
# config.toml
[debug]
enabled = true
log_queries = true
log_metrics = true
slow_query_threshold_ms = 100
```

### Health Checks
```rust
// Implement health endpoint
async fn health_check(State(engine): State<Arc<UnifiedCollectionEngine>>) -> impl IntoResponse {
    match engine.check_health().await {
        Ok(_) => (StatusCode::OK, "healthy"),
        Err(e) => (StatusCode::SERVICE_UNAVAILABLE, e.to_string()),
    }
}
```

## Best Practices

### 1. Error Handling
- Use `Result<T, E>` for all fallible operations
- Implement graceful degradation
- Log errors with context
- Never panic in production code

### 2. Configuration
- Use environment variables for secrets
- Validate configuration on startup
- Provide sensible defaults
- Document all options

### 3. Monitoring
- Instrument with metrics
- Add structured logging
- Implement trace spans
- Export self-metrics

### 4. Security
- Never log sensitive data
- Use prepared statements
- Validate all inputs
- Follow principle of least privilege

## Next Steps

- [Deployment Guide](03-deployment-operations.md) - Installing and running in production
- [Metrics Reference](04-metrics-reference.md) - Complete metric documentation
- [Migration Guide](05-migration-guide.md) - Upgrading from nri-postgresql