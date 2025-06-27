# Pragmatic PostgreSQL Collector Architecture

## Design Philosophy

Keep what adds value, remove what adds complexity.

## Architecture: Streamlined 2-Tier

```
┌─────────────────┐      ┌──────────────────┐      ┌─────────────────┐
│   PostgreSQL    │      │  Smart Collector │      │   New Relic     │
│                 │◄─────│                  │─────►│                 │
│ pg_stat_*       │ SQL  │  - Fingerprinting│ OTLP │  OTLP Endpoint  │
│                 │      │  - Batching      │      │                 │
└─────────────────┘      │  - Sampling      │      └─────────────────┘
                         └──────────────────┘

Removed: Intermediate OTEL Collector (unnecessary hop)
Kept: OTLP protocol (industry standard, good compression)
```

## What We Keep (Good Parts)

### 1. Query Fingerprinting (Simplified)
**Why**: Essential for cardinality control
**How**: Use proper SQL parser, not regex

```rust
use sqlparser::dialect::PostgreSqlDialect;
use sqlparser::parser::Parser;
use xxhash_rust::xxh3::xxh3_128;

pub struct QueryFingerprinter {
    parser: Parser,
    cache: lru::LruCache<String, u128>,
}

impl QueryFingerprinter {
    pub fn new(cache_size: usize) -> Self {
        Self {
            parser: Parser::new(&PostgreSqlDialect {}),
            cache: lru::LruCache::new(cache_size),
        }
    }
    
    pub fn fingerprint(&mut self, query: &str) -> u128 {
        // Check cache first
        if let Some(&fp) = self.cache.get(query) {
            return fp;
        }
        
        // Parse and normalize
        let normalized = match self.parser.try_with_sql(query) {
            Ok(ast) => self.normalize_ast(ast),
            Err(_) => {
                // Fallback for unparseable queries
                self.simple_normalize(query)
            }
        };
        
        let fingerprint = xxh3_128(normalized.as_bytes());
        self.cache.put(query.to_string(), fingerprint);
        fingerprint
    }
}
```

### 2. Dimensional Metrics (Controlled)
**Why**: Valuable for analysis
**How**: Fixed dimensions, no dynamic growth

```rust
#[derive(Clone, Copy)]
pub enum QueryType {
    Select,
    Insert,
    Update,
    Delete,
    Other,
}

#[derive(Clone, Copy)]
pub enum ConnectionState {
    Active,
    Idle,
    IdleInTransaction,
}

pub struct Dimensions {
    pub database: InternedString,      // Interned for memory efficiency
    pub query_type: QueryType,         // Enum, not string
    pub connection_state: ConnectionState,
    pub user: UserId,                  // Numeric ID, not string
}
```

### 3. OTLP Protocol (Direct to New Relic)
**Why**: Compression, batching, standard protocol
**How**: Skip intermediate collector

```rust
use opentelemetry_otlp::WithExportConfig;

pub struct NewRelicExporter {
    client: opentelemetry_otlp::TonicExporterBuilder,
    batch_size: usize,
    batch_timeout: Duration,
}

impl NewRelicExporter {
    pub fn new(license_key: &str) -> Result<Self> {
        let client = opentelemetry_otlp::new_exporter()
            .tonic()
            .with_endpoint("https://otlp.nr-data.net:4317")
            .with_headers(vec![("api-key", license_key)])
            .with_compression(Compression::Gzip);
            
        Ok(Self {
            client,
            batch_size: 1000,
            batch_timeout: Duration::from_secs(10),
        })
    }
}
```

## What We Remove (Over-Engineering)

### 1. ❌ Intermediate OTEL Collector
- Adds latency and complexity
- Another process to manage
- Direct OTLP to New Relic works fine

### 2. ❌ Complex Cardinality Management
Replace with simple LRU:
```rust
pub struct CardinalityController {
    queries: lru::LruCache<u128, QueryStats>,
    tables: lru::LruCache<u32, ()>,
    users: lru::LruCache<u32, ()>,
}

// No silent drops, just LRU eviction of least recently seen
```

### 3. ❌ Regex-Based Parsing
Replace with proper SQL parser or simpler logic

### 4. ❌ Global Locks
Replace with sharded structure:
```rust
pub struct ShardedTracker<T> {
    shards: Vec<RwLock<lru::LruCache<T, ()>>>,
}

impl<T: Hash> ShardedTracker<T> {
    fn track(&self, item: &T) -> bool {
        let shard_idx = hash(item) % self.shards.len();
        let mut shard = self.shards[shard_idx].write();
        shard.put(item.clone(), ());
        true  // Always track, LRU handles limits
    }
}
```

### 5. ❌ TOML Configuration
Replace with simpler config:
```rust
#[derive(Deserialize, Clone)]
pub struct Config {
    pub database_url: String,
    pub new_relic_api_key: String,
    
    #[serde(default = "defaults::batch_size")]
    pub batch_size: usize,
    
    #[serde(default = "defaults::collection_interval")]
    pub collection_interval_secs: u64,
    
    #[serde(default = "defaults::slow_query_threshold")]
    pub slow_query_threshold_ms: f64,
}

// Load from environment with defaults
impl Config {
    pub fn from_env() -> Result<Self> {
        envy::from_env()
    }
}
```

## Pragmatic Implementation

### Core Collector Loop

```rust
pub struct PostgresCollector {
    pool: PgPool,
    exporter: NewRelicExporter,
    fingerprinter: QueryFingerprinter,
    metrics: CollectorMetrics,
}

impl PostgresCollector {
    pub async fn run(self) -> Result<()> {
        let mut interval = tokio::time::interval(Duration::from_secs(30));
        let mut shutdown = signal::ctrl_c();
        
        loop {
            tokio::select! {
                _ = interval.tick() => {
                    if let Err(e) = self.collect_cycle().await {
                        self.metrics.collection_errors.increment(1);
                        tracing::error!(error = %e, "Collection cycle failed");
                        // Continue running - don't crash
                    }
                }
                _ = &mut shutdown => {
                    tracing::info!("Shutdown signal received");
                    self.flush_remaining().await?;
                    break;
                }
            }
        }
        Ok(())
    }
    
    async fn collect_cycle(&self) -> Result<()> {
        let start = Instant::now();
        
        // Parallel collection
        let (queries, connections, locks) = tokio::try_join!(
            self.collect_query_stats(),
            self.collect_connection_stats(),
            self.collect_lock_stats()
        )?;
        
        self.metrics.collection_duration.record(start.elapsed());
        
        // Convert to OTLP metrics
        let mut metrics = Vec::with_capacity(queries.len() + connections.len());
        
        for query in queries {
            metrics.push(self.query_to_metric(query));
        }
        
        for conn in connections {
            metrics.push(self.connection_to_metric(conn));
        }
        
        // Send with retry
        self.exporter.send_with_retry(metrics).await?;
        
        Ok(())
    }
}
```

### Efficient Query Collection

```rust
impl PostgresCollector {
    async fn collect_query_stats(&self) -> Result<Vec<QueryStat>> {
        // Single efficient query
        let rows = sqlx::query!(
            r#"
            WITH database_map AS (
                SELECT oid, datname 
                FROM pg_database 
                WHERE datname NOT IN ('template0', 'template1')
            )
            SELECT 
                d.datname as database,
                s.userid::oid::regrole::text as username,
                s.queryid,
                LEFT(s.query, 100) as query_sample,
                s.calls,
                s.mean_exec_time,
                s.stddev_exec_time,
                s.rows,
                s.shared_blks_hit + s.shared_blks_read as total_blocks
            FROM pg_stat_statements s
            JOIN database_map d ON s.dbid = d.oid
            WHERE s.mean_exec_time > $1
            ORDER BY s.mean_exec_time DESC
            LIMIT 500
            "#,
            self.config.slow_query_threshold_ms
        )
        .fetch_all(&self.pool)
        .await?;
        
        let mut stats = Vec::with_capacity(rows.len());
        
        for row in rows {
            let fingerprint = self.fingerprinter.fingerprint(&row.query_sample);
            
            stats.push(QueryStat {
                database: self.intern_string(&row.database),
                username: self.intern_string(&row.username),
                fingerprint,
                query_type: detect_query_type(&row.query_sample),
                calls: row.calls,
                mean_duration_ms: row.mean_exec_time,
                total_rows: row.rows,
            });
        }
        
        Ok(stats)
    }
}
```

### Proper Error Handling

```rust
#[derive(Debug, thiserror::Error)]
pub enum CollectorError {
    #[error("Database connection failed: {0}")]
    Database(#[from] sqlx::Error),
    
    #[error("Export failed: {0}")]
    Export(#[from] ExportError),
    
    #[error("Configuration invalid: {0}")]
    Config(String),
}

pub struct RetryPolicy {
    max_attempts: u32,
    initial_delay: Duration,
    max_delay: Duration,
}

impl NewRelicExporter {
    async fn send_with_retry(&self, metrics: Vec<Metric>) -> Result<()> {
        let mut attempt = 0;
        let mut delay = self.retry_policy.initial_delay;
        
        loop {
            match self.send_internal(metrics.clone()).await {
                Ok(()) => return Ok(()),
                Err(e) if attempt < self.retry_policy.max_attempts => {
                    tracing::warn!(
                        attempt = attempt + 1,
                        error = %e,
                        "Export failed, retrying"
                    );
                    tokio::time::sleep(delay).await;
                    delay = (delay * 2).min(self.retry_policy.max_delay);
                    attempt += 1;
                }
                Err(e) => return Err(e),
            }
        }
    }
}
```

### Observability Built-In

```rust
pub struct CollectorMetrics {
    // Prometheus metrics for monitoring the collector itself
    pub collection_duration: Histogram,
    pub queries_collected: Counter,
    pub export_duration: Histogram,
    pub export_errors: Counter,
    pub active_connections: Gauge,
}

// Expose on /metrics endpoint
pub async fn metrics_endpoint(metrics: Arc<CollectorMetrics>) -> impl Reply {
    let encoder = prometheus::TextEncoder::new();
    let metric_families = prometheus::gather();
    encoder.encode_to_string(&metric_families)
        .unwrap_or_else(|_| "error encoding metrics".to_string())
}
```

## Configuration Examples

### Minimal (Environment Variables)
```bash
DATABASE_URL=postgresql://localhost/mydb
NEW_RELIC_API_KEY=xxx
```

### Advanced (JSON)
```json
{
  "database_url": "postgresql://localhost/mydb",
  "new_relic_api_key": "xxx",
  "batch_size": 500,
  "collection_interval_secs": 30,
  "slow_query_threshold_ms": 100,
  "max_queries_tracked": 1000
}
```

## Benefits of This Approach

### 1. **Simplicity**
- Single binary, single process
- Direct PostgreSQL → New Relic
- ~800 lines instead of 2000+

### 2. **Performance**
- LRU cache for fingerprints
- Sharded tracking (no global locks)
- Efficient SQL queries
- Proper connection pooling

### 3. **Reliability**
- Graceful error handling
- Retry with backoff
- Continues on failures
- Observability built-in

### 4. **Pragmatic**
- Keeps valuable features (fingerprinting, dimensions)
- Removes complexity (intermediate collector, regex parsing)
- Standard patterns (LRU, sharding)

### 5. **Maintainable**
- Clear separation of concerns
- Testable components
- Standard error handling
- Familiar patterns

## Testing Strategy

```rust
#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_fingerprinting() {
        let mut fp = QueryFingerprinter::new(100);
        
        assert_eq!(
            fp.fingerprint("SELECT * FROM users WHERE id = 1"),
            fp.fingerprint("SELECT * FROM users WHERE id = 999")
        );
    }
    
    #[tokio::test]
    async fn test_collection_with_mock_db() {
        let db = MockDatabase::new();
        db.expect_query_stats()
            .returning(|| Ok(vec![/* test data */]));
            
        let collector = PostgresCollector::new_with_db(db);
        let stats = collector.collect_query_stats().await.unwrap();
        
        assert_eq!(stats.len(), expected_count);
    }
}
```

## Migration Path

1. **Phase 1**: Deploy new collector alongside old
2. **Phase 2**: Compare metrics, ensure parity  
3. **Phase 3**: Switch traffic, monitor
4. **Phase 4**: Decommission old stack

This pragmatic approach delivers 80% of the value with 20% of the complexity.