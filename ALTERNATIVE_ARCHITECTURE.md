# Alternative Architecture: What We Should Have Built

## The Simple, Correct Solution

### Architecture: Direct PostgreSQL → New Relic

```
┌─────────────────┐      ┌──────────────────┐      ┌─────────────────┐
│   PostgreSQL    │      │  Simple Exporter │      │   New Relic     │
│                 │◄─────│                  │─────►│                 │
│ pg_stat_*       │ SQL  │  Single Binary   │ HTTP │  Metric API     │
└─────────────────┘      └──────────────────┘      └─────────────────┘
```

### Complete Implementation (200 lines vs 2000)

```rust
use anyhow::Result;
use serde::Serialize;
use sqlx::{PgPool, postgres::PgPoolOptions};
use std::time::{Duration, SystemTime, UNIX_EPOCH};
use reqwest::Client;
use tokio::time::interval;

#[derive(Serialize)]
struct MetricBatch {
    common: CommonAttributes,
    metrics: Vec<Metric>,
}

#[derive(Serialize)]
struct CommonAttributes {
    timestamp: u64,
    #[serde(rename = "interval.ms")]
    interval_ms: u64,
    attributes: Attributes,
}

#[derive(Serialize)]
struct Attributes {
    #[serde(rename = "service.name")]
    service_name: &'static str,
    #[serde(rename = "db.system")]
    db_system: &'static str,
    #[serde(rename = "host.name")]
    host_name: String,
}

#[derive(Serialize)]
struct Metric {
    name: &'static str,
    #[serde(rename = "type")]
    metric_type: &'static str,
    value: f64,
    timestamp: u64,
    attributes: MetricAttributes,
}

#[derive(Serialize)]
struct MetricAttributes {
    #[serde(rename = "db.name")]
    db_name: String,
    #[serde(rename = "db.operation", skip_serializing_if = "Option::is_none")]
    operation: Option<String>,
    #[serde(rename = "query.hash", skip_serializing_if = "Option::is_none")]
    query_hash: Option<String>,
}

struct Exporter {
    pool: PgPool,
    client: Client,
    nr_api_key: String,
    hostname: String,
}

impl Exporter {
    async fn new(database_url: &str, nr_api_key: String) -> Result<Self> {
        let pool = PgPoolOptions::new()
            .max_connections(2)  // We only need read access
            .acquire_timeout(Duration::from_secs(5))
            .connect(database_url).await?;
            
        let hostname = hostname::get()?.to_string_lossy().to_string();
        
        Ok(Self {
            pool,
            client: Client::new(),
            nr_api_key,
            hostname,
        })
    }
    
    async fn collect_and_send(&self) -> Result<()> {
        let timestamp = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
        let mut metrics = Vec::new();
        
        // Collect slow queries
        let slow_queries = sqlx::query!(
            r#"
            SELECT 
                datname as db_name,
                queryid::text as query_hash,
                LEFT(query, 50) as query_prefix,
                calls,
                mean_exec_time as avg_ms,
                stddev_exec_time as stddev_ms,
                rows
            FROM pg_stat_statements s
            JOIN pg_database d ON s.dbid = d.oid
            WHERE mean_exec_time > 100  -- Only queries > 100ms
            ORDER BY mean_exec_time DESC
            LIMIT 100  -- Top 100 slow queries
            "#
        ).fetch_all(&self.pool).await?;
        
        for query in slow_queries {
            // Query duration metric
            metrics.push(Metric {
                name: "db.query.duration",
                metric_type: "gauge",
                value: query.avg_ms.unwrap_or(0.0),
                timestamp,
                attributes: MetricAttributes {
                    db_name: query.db_name.unwrap_or_default(),
                    operation: detect_operation(&query.query_prefix.unwrap_or_default()),
                    query_hash: query.query_hash,
                },
            });
            
            // Query count metric
            metrics.push(Metric {
                name: "db.query.count",
                metric_type: "count",
                value: query.calls.unwrap_or(0) as f64,
                timestamp,
                attributes: MetricAttributes {
                    db_name: query.db_name.unwrap_or_default(),
                    operation: detect_operation(&query.query_prefix.unwrap_or_default()),
                    query_hash: query.query_hash.clone(),
                },
            });
        }
        
        // Collect connection metrics
        let connections = sqlx::query!(
            r#"
            SELECT 
                datname,
                count(*) FILTER (WHERE state = 'active') as active,
                count(*) FILTER (WHERE state = 'idle') as idle,
                count(*) FILTER (WHERE state = 'idle in transaction') as idle_in_transaction,
                count(*) as total
            FROM pg_stat_activity
            WHERE datname IS NOT NULL
            GROUP BY datname
            "#
        ).fetch_all(&self.pool).await?;
        
        for conn in connections {
            metrics.push(Metric {
                name: "db.connections.active",
                metric_type: "gauge",
                value: conn.active.unwrap_or(0) as f64,
                timestamp,
                attributes: MetricAttributes {
                    db_name: conn.datname.unwrap_or_default(),
                    operation: None,
                    query_hash: None,
                },
            });
            
            metrics.push(Metric {
                name: "db.connections.idle",
                metric_type: "gauge",
                value: conn.idle.unwrap_or(0) as f64,
                timestamp,
                attributes: MetricAttributes {
                    db_name: conn.datname.unwrap_or_default(),
                    operation: None,
                    query_hash: None,
                },
            });
        }
        
        // Send to New Relic
        if !metrics.is_empty() {
            let batch = MetricBatch {
                common: CommonAttributes {
                    timestamp,
                    interval_ms: 30_000,
                    attributes: Attributes {
                        service_name: "postgresql",
                        db_system: "postgresql",
                        host_name: self.hostname.clone(),
                    },
                },
                metrics,
            };
            
            self.send_metrics(vec![batch]).await?;
        }
        
        Ok(())
    }
    
    async fn send_metrics(&self, batches: Vec<MetricBatch>) -> Result<()> {
        let response = self.client
            .post("https://metric-api.newrelic.com/metric/v1")
            .header("Api-Key", &self.nr_api_key)
            .json(&batches)
            .timeout(Duration::from_secs(10))
            .send()
            .await?;
            
        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await?;
            anyhow::bail!("New Relic API error {}: {}", status, body);
        }
        
        Ok(())
    }
}

fn detect_operation(query: &str) -> Option<String> {
    let query_upper = query.to_uppercase();
    if query_upper.starts_with("SELECT") {
        Some("SELECT".to_string())
    } else if query_upper.starts_with("INSERT") {
        Some("INSERT".to_string())
    } else if query_upper.starts_with("UPDATE") {
        Some("UPDATE".to_string())
    } else if query_upper.starts_with("DELETE") {
        Some("DELETE".to_string())
    } else {
        None
    }
}

#[tokio::main]
async fn main() -> Result<()> {
    env_logger::init();
    
    let database_url = std::env::var("DATABASE_URL")?;
    let nr_api_key = std::env::var("NEW_RELIC_API_KEY")?;
    
    let exporter = Exporter::new(&database_url, nr_api_key).await?;
    let mut interval = interval(Duration::from_secs(30));
    
    log::info!("PostgreSQL exporter started");
    
    loop {
        interval.tick().await;
        
        if let Err(e) = exporter.collect_and_send().await {
            log::error!("Failed to collect/send metrics: {}", e);
            // Continue running - don't crash on temporary failures
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_detect_operation() {
        assert_eq!(detect_operation("SELECT * FROM users"), Some("SELECT".to_string()));
        assert_eq!(detect_operation("select * from users"), Some("SELECT".to_string()));
        assert_eq!(detect_operation("INSERT INTO users"), Some("INSERT".to_string()));
        assert_eq!(detect_operation("WITH cte AS"), None);
    }
}
```

### Configuration (Environment Variables)

```bash
DATABASE_URL=postgresql://user:pass@localhost/db
NEW_RELIC_API_KEY=YOUR_INGEST_KEY
RUST_LOG=info
```

### Dockerfile (10 lines)

```dockerfile
FROM rust:1.82 as builder
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
COPY src ./src
RUN cargo build --release

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/target/release/pg-exporter /usr/local/bin/
CMD ["pg-exporter"]
```

### Why This Is Better

1. **Simplicity**
   - 200 lines vs 2000+ lines
   - Single binary, single process
   - No intermediate collectors

2. **Performance**
   - Direct SQL queries (indexed)
   - No regex parsing
   - No global locks
   - Minimal allocations

3. **Reliability**
   - Connection pooling built-in
   - Graceful error handling
   - No silent data drops
   - Continues on failures

4. **Observability**
   - Standard logging
   - Errors are visible
   - Easy to debug

5. **Cost**
   - pg_stat_statements already aggregates
   - Only top 100 queries sent
   - 30-second batching
   - ~1KB per metric

6. **Maintenance**
   - No complex config files
   - No cardinality management
   - No fingerprinting bugs
   - Standard SQL queries

### Cardinality Math

```
Dimensions:
- db_name: ~10 databases max
- operation: 4 values (SELECT, INSERT, UPDATE, DELETE)  
- query_hash: 100 (top queries)

Total cardinality: 10 × 4 × 100 = 4,000 series max

vs Original design: 
- 1000 fingerprints × 500 tables × 100 users = 50,000,000 potential series!
```

### The Lesson

**We built a space shuttle to cross the street.**

The real requirements were:
1. Get PostgreSQL metrics to New Relic
2. Don't blow up cardinality
3. Don't lose data

The simple solution achieves all three in 1/10th the code, with better performance and reliability.

**Engineering is about judgment, not complexity.**