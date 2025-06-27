# PostgreSQL Collector - New Relic Dimensional Metrics Architecture

## Executive Summary

We've redesigned the PostgreSQL Unified Collector with a deep focus on dimensional metrics optimized for New Relic's NRDB backend. The solution removes Prometheus and Grafana, sending metrics directly to New Relic via OTLP with sophisticated cardinality management.

## Key Architectural Decisions

### 1. **Pure OTLP to New Relic**
- Direct integration via `https://otlp.nr-data.net:4317`
- No intermediate storage (removed Prometheus)
- No visualization layer (removed Grafana)
- Single path: PostgreSQL → Collector → OTEL Collector → New Relic

### 2. **Dimensional Metrics Hierarchy**

```yaml
Resource Attributes (Entity-level):
  service.name: postgresql
  service.namespace: production
  server.address: postgres-prod-01
  cluster.name: production-cluster
  
Metric Dimensions (Query-time flexibility):
  db.name: testdb
  db.operation: SELECT|INSERT|UPDATE|DELETE
  db.schema: public|analytics|reporting
  query.fingerprint: normalized_hash
  connection.state: active|idle|idle_in_transaction
  lock.type: relation|page|tuple
  user.name: app_user|analytics_user|other
```

### 3. **Cardinality Control System**

**Problem**: Unrestricted dimensions can create millions of unique time series
**Solution**: Multi-layered cardinality control

```rust
// Top-K Tracking
max_query_fingerprints = 1000  // Only top 1000 queries
max_table_names = 500          // Only top 500 tables
max_user_names = 100           // Only top 100 users

// Smart Aggregation
if cardinality_exceeded {
    dimension_value = "other"
}

// Allowlists for Critical Dimensions
important_users = ["app_user", "admin_user"]  // Always tracked
```

### 4. **Query Fingerprinting**

**Purpose**: Reduce infinite query variations to manageable patterns

```
Original: SELECT * FROM users WHERE id = 12345 AND status = 'active'
Fingerprint: SELECT * FROM users WHERE id = ? AND status = ?
Hash: a1b2c3d4e5f67890
```

**Benefits**:
- Groups similar queries together
- Reduces cardinality from millions to thousands
- Preserves query structure for analysis

### 5. **New Relic Optimizations**

**Delta Temporality**: Counters report incremental changes
```rust
.with_temporality_selector(|kind| match kind {
    InstrumentKind::Counter => Temporality::Delta,
    _ => Temporality::Cumulative,
})
```

**Entity Synthesis**: Resource attributes create PostgreSQL entities
```toml
[outputs.otlp.resource_attributes.custom]
"newrelic.entity.type" = "POSTGRESQL_INSTANCE"
"newrelic.entity.name" = "postgres-prod-01"
```

## Metrics Taxonomy

### Query Performance
- `postgresql.query.duration` (histogram, ms)
- `postgresql.query.rows` (histogram, count)
- `postgresql.query.io.blocks` (counter, blocks)

### Connections
- `postgresql.connection.count` (gauge, connections)
- `postgresql.connection.utilization` (gauge, %)
- `postgresql.connection.wait_time` (histogram, ms)

### Locks
- `postgresql.lock.wait_time` (histogram, ms)
- `postgresql.deadlock.count` (counter, events)

### Replication
- `postgresql.replication.lag` (gauge, bytes)

## Implementation Highlights

### 1. **Cardinality Tracker**
```rust
struct CardinalityTracker {
    query_fingerprints: Arc<Mutex<HashSet<String>>>,
    table_names: Arc<Mutex<HashSet<String>>>,
    user_names: Arc<Mutex<HashSet<String>>>,
    limits: CardinalityLimits,
}
```

### 2. **Query Categorization**
```toml
[slow_queries.categories]
system = ["^SELECT.*FROM\\s+pg_", "^SELECT.*FROM\\s+information_schema"]
vacuum = ["^VACUUM", "^ANALYZE"]
ddl = ["^CREATE", "^ALTER", "^DROP"]
dml = ["^INSERT", "^UPDATE", "^DELETE"]
```

### 3. **Dimension Filtering**
```yaml
filter/cardinality:
  metrics:
    metric:
      - 'attributes["query.category"] == "system"'  # Drop system queries
```

## New Relic Benefits

### 1. **NRQL Query Power**
```sql
-- Find slowest queries by fingerprint
SELECT percentile(postgresql.query.duration, 95) 
FROM Metric 
WHERE db.system = 'postgresql'
FACET query.fingerprint
SINCE 1 hour ago

-- Connection pool analysis
SELECT latest(postgresql.connection.count)
FROM Metric
WHERE db.system = 'postgresql'
FACET connection.state, user.name
TIMESERIES 1 minute
```

### 2. **Entity-Centric Monitoring**
- Automatic PostgreSQL entity creation
- Relationship mapping with APM services
- Service map integration

### 3. **Scalability**
- Handles thousands of queries/second
- Cardinality protection prevents explosion
- Efficient batching and compression

## Files Created

1. **docker-compose-newrelic.yml** - Stack configuration
2. **otel-collector-newrelic.yaml** - OTEL Collector config with New Relic export
3. **config-newrelic.toml** - Collector configuration with dimensions
4. **newrelic-collector.rs** - Implementation with cardinality control
5. **DIMENSIONAL_METRICS_ARCHITECTURE.md** - Complete design document
6. **NEWRELIC_INTEGRATION_GUIDE.md** - Deployment and usage guide

## Key Differentiators

1. **Not Just Metrics**: Dimensional data model for deep analysis
2. **Cardinality Safety**: Prevents runaway costs and performance issues
3. **New Relic Native**: Optimized for NRDB storage and NRQL queries
4. **Production Ready**: Handles scale with circuit breakers
5. **Zero Lock-in**: Uses open standards (OTLP) throughout

The solution provides enterprise-grade PostgreSQL monitoring with sophisticated dimensional metrics, specifically architected for New Relic's strengths in unified observability.