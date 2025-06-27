# PostgreSQL Dimensional Metrics Architecture for New Relic

## Overview

This document outlines the dimensional metrics architecture for PostgreSQL monitoring, specifically designed for New Relic's OTLP ingestion and NRDB storage model.

## Metric Taxonomy

### 1. Query Performance Metrics

#### `postgresql.query.duration`
- **Type**: Histogram
- **Unit**: milliseconds
- **Description**: Distribution of query execution times
- **Dimensions**:
  - `db.name` - Database name (low cardinality)
  - `db.schema` - Schema name (medium cardinality)
  - `db.operation` - Operation type: SELECT, INSERT, UPDATE, DELETE (low cardinality)
  - `query.fingerprint` - Normalized query hash (high cardinality, controlled)
  - `query.status` - success, error, timeout (low cardinality)
  - `query.plan_type` - sequential, index, bitmap (low cardinality)

#### `postgresql.query.rows`
- **Type**: Histogram
- **Unit**: count
- **Description**: Number of rows affected/returned by queries
- **Dimensions**:
  - `db.name`
  - `db.operation`
  - `query.fingerprint`

#### `postgresql.query.io.blocks`
- **Type**: Counter
- **Unit**: blocks
- **Description**: Disk blocks read/written
- **Dimensions**:
  - `db.name`
  - `io.direction` - read, write
  - `io.type` - shared, local, temp

### 2. Connection Metrics

#### `postgresql.connection.count`
- **Type**: Gauge
- **Unit**: connections
- **Description**: Current connection count
- **Dimensions**:
  - `db.name`
  - `connection.state` - active, idle, idle_in_transaction, fastpath
  - `connection.type` - client, replication, background
  - `user.name` - Connected user (medium cardinality)
  - `client.application` - Application name

#### `postgresql.connection.wait_time`
- **Type**: Histogram
- **Unit**: milliseconds
- **Description**: Time spent waiting for connections
- **Dimensions**:
  - `db.name`
  - `wait.type` - lock, io, buffer, network

### 3. Lock Metrics

#### `postgresql.lock.wait_time`
- **Type**: Histogram
- **Unit**: milliseconds
- **Description**: Time spent waiting for locks
- **Dimensions**:
  - `db.name`
  - `lock.type` - relation, page, tuple, transaction
  - `lock.mode` - AccessShare, RowExclusive, etc.
  - `table.name` - Table involved (high cardinality, controlled)

#### `postgresql.deadlock.count`
- **Type**: Counter
- **Unit**: events
- **Description**: Number of deadlocks detected
- **Dimensions**:
  - `db.name`

### 4. Replication Metrics

#### `postgresql.replication.lag`
- **Type**: Gauge
- **Unit**: bytes
- **Description**: Replication lag in bytes
- **Dimensions**:
  - `replication.role` - primary, standby
  - `replication.slot` - Slot name
  - `replication.state` - streaming, catchup, sync

## Resource Attributes (Entity-level)

These attributes identify the PostgreSQL instance and remain constant:

```yaml
resource:
  attributes:
    # Service identification
    service.name: "postgresql"
    service.namespace: "production"
    service.version: "15.3"
    
    # Database identification
    db.system: "postgresql"
    db.connection_string: "postgresql://host:5432"
    server.address: "postgres-prod-01.example.com"
    server.port: 5432
    
    # Infrastructure context
    cloud.provider: "aws"
    cloud.region: "us-east-1"
    cloud.availability_zone: "us-east-1a"
    host.name: "ip-10-0-1-50"
    
    # New Relic specific
    newrelic.source: "postgresql.otel.collector"
    telemetry.sdk.name: "opentelemetry"
    telemetry.sdk.language: "rust"
    telemetry.sdk.version: "0.21.0"
```

## Cardinality Management Strategy

### High Cardinality Dimensions (Carefully Controlled)

1. **Query Fingerprints**
   - Use query normalization to reduce cardinality
   - Implement top-K tracking (e.g., top 1000 queries)
   - Add sampling for extremely high-volume queries

2. **Table Names**
   - Limit to most accessed tables
   - Group small tables into "other" category
   - Use schema prefixing for namespace isolation

3. **User Names**
   - Group system users separately
   - Implement allowlist for important users
   - Aggregate others as "application_user"

### Cardinality Limits

```yaml
cardinality_limits:
  query.fingerprint: 1000      # Top 1000 unique queries
  table.name: 500              # Top 500 tables
  user.name: 100               # Top 100 users
  client.application: 50       # Top 50 applications
```

## New Relic Specific Optimizations

### 1. Metric Naming Convention
Following New Relic's recommended format:
- Prefix: `postgresql.`
- Category: `query.`, `connection.`, `lock.`, etc.
- Metric: `duration`, `count`, `lag`

### 2. Units Mapping
```yaml
unit_mappings:
  milliseconds: "ms"
  microseconds: "us"
  bytes: "By"
  connections: "{connections}"
  queries: "{queries}"
  percentage: "%"
```

### 3. Aggregation Temporality
- Use **Delta** temporality for counters (New Relic preference)
- Use **Cumulative** for gauges
- Configure appropriate collection intervals (30s default)

### 4. Batching Configuration
```yaml
batch:
  timeout: 10s
  send_batch_size: 1000
  send_batch_max_size: 2000
```

## OTLP Export Configuration

```yaml
exporters:
  otlp:
    endpoint: "https://otlp.nr-data.net:4317"
    headers:
      "api-key": "${NEW_RELIC_LICENSE_KEY}"
    compression: gzip
    timeout: 30s
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
```

## Query Fingerprinting Algorithm

```rust
fn fingerprint_query(query: &str) -> String {
    // 1. Normalize whitespace
    let normalized = query.trim().replace(r"\s+", " ");
    
    // 2. Replace literals
    let without_strings = replace_string_literals(&normalized, "?");
    let without_numbers = replace_numeric_literals(&without_strings, "?");
    
    // 3. Lowercase keywords
    let lowercase = lowercase_keywords(&without_numbers);
    
    // 4. Generate hash
    let hash = sha256(&lowercase);
    
    // 5. Return first 16 chars of hex
    hex::encode(&hash)[..16].to_string()
}
```

## Metric Collection Patterns

### 1. Slow Query Collection
```rust
struct SlowQueryMetric {
    duration_ms: f64,
    fingerprint: String,
    operation: String,
    schema: String,
    rows_affected: i64,
    disk_reads: i64,
    disk_writes: i64,
}

impl SlowQueryMetric {
    fn to_otlp(&self, meter: &Meter) -> Result<()> {
        let duration = meter
            .f64_histogram("postgresql.query.duration")
            .with_unit("ms")
            .init();
            
        duration.record(
            self.duration_ms,
            &[
                KeyValue::new("db.operation", self.operation.clone()),
                KeyValue::new("db.schema", self.schema.clone()),
                KeyValue::new("query.fingerprint", self.fingerprint.clone()),
            ],
        );
        
        Ok(())
    }
}
```

### 2. Connection Pool Monitoring
```rust
struct ConnectionMetrics {
    active: i64,
    idle: i64,
    waiting: i64,
    max_connections: i64,
}

impl ConnectionMetrics {
    fn to_otlp(&self, meter: &Meter) -> Result<()> {
        let gauge = meter
            .i64_gauge("postgresql.connection.count")
            .with_unit("{connections}")
            .init();
            
        gauge.record(
            self.active,
            &[KeyValue::new("connection.state", "active")],
        );
        
        gauge.record(
            self.idle,
            &[KeyValue::new("connection.state", "idle")],
        );
        
        // Connection utilization as percentage
        let utilization = meter
            .f64_gauge("postgresql.connection.utilization")
            .with_unit("%")
            .init();
            
        let percent = (self.active + self.idle) as f64 / self.max_connections as f64 * 100.0;
        utilization.record(percent, &[]);
        
        Ok(())
    }
}
```

## New Relic NRQL Query Examples

```sql
-- Top slow queries by p95 duration
SELECT percentile(postgresql.query.duration, 95) as 'p95_duration'
FROM Metric
WHERE db.system = 'postgresql'
FACET query.fingerprint
SINCE 1 hour ago

-- Connection pool utilization
SELECT average(postgresql.connection.utilization) as 'avg_utilization'
FROM Metric
WHERE db.system = 'postgresql'
FACET db.name
TIMESERIES 5 minutes

-- Lock wait time by table
SELECT sum(postgresql.lock.wait_time) as 'total_wait_time'
FROM Metric
WHERE db.system = 'postgresql'
FACET table.name
SINCE 1 hour ago

-- Query performance by operation type
SELECT 
  count(postgresql.query.duration) as 'query_count',
  average(postgresql.query.duration) as 'avg_duration',
  percentile(postgresql.query.duration, 95) as 'p95_duration'
FROM Metric
WHERE db.system = 'postgresql'
FACET db.operation
TIMESERIES 5 minutes
```

## Best Practices for New Relic

1. **Use Resource Attributes for Entity Creation**
   - New Relic creates entities based on resource attributes
   - Ensure consistent `service.name` and `server.address`

2. **Implement Metric Filtering**
   - Filter out system queries
   - Exclude monitoring queries
   - Focus on user-initiated operations

3. **Add Custom Attributes Sparingly**
   - Each attribute increases cardinality
   - Use allowlists for high-cardinality dimensions
   - Aggregate less important values

4. **Optimize Collection Intervals**
   - 30 seconds for most metrics
   - 60 seconds for less critical metrics
   - 10 seconds for critical SLIs

5. **Implement Circuit Breakers**
   - Stop collecting if cardinality exceeds limits
   - Alert on cardinality issues
   - Automatic dimension reduction under pressure