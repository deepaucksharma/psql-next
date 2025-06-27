# PostgreSQL Unified Collector - New Relic Integration Guide

## Architecture Overview

```
┌─────────────────┐      ┌──────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│   PostgreSQL    │      │ Unified Collector│      │ OTEL Collector  │      │   New Relic     │
│   Database      │◄─────│                  │─────►│                 │─────►│                 │
│                 │      │ - Dimensions     │ OTLP │ - Processing    │ OTLP │ - NRDB Storage  │
│ pg_stat_statements     │ - Cardinality    │      │ - Filtering     │      │ - Entity Synth  │
│ pg_stat_activity       │ - Fingerprinting │      │ - Batching      │      │ - NRQL Queries  │
└─────────────────┘      └──────────────────┘      └─────────────────┘      └─────────────────┘
```

## Key Design Decisions

### 1. Dimensional Metrics Design

The collector implements a sophisticated dimensional metrics model optimized for New Relic:

- **Resource Attributes**: Define the PostgreSQL entity
- **Metric Attributes**: Provide query-time flexibility
- **Cardinality Control**: Prevent dimension explosion
- **Smart Categorization**: Automatic query classification

### 2. Query Fingerprinting

```rust
// Normalizes queries for cardinality control
"SELECT * FROM users WHERE id = 123" → "SELECT * FROM users WHERE id = ?"
"INSERT INTO logs VALUES ('data', 456)" → "INSERT INTO logs VALUES (?, ?)"
```

### 3. Cardinality Management

- **Top-K Tracking**: Only top 1000 queries, 500 tables, 100 users
- **Dynamic Aggregation**: Less important values → "other"
- **Allowlists**: Priority dimensions always tracked
- **Circuit Breakers**: Stop collection if limits exceeded

## Deployment

### 1. Quick Start

```bash
# Set your New Relic license key
export NEW_RELIC_LICENSE_KEY="your-ingest-license-key"

# Start the stack
docker-compose -f docker-compose-newrelic.yml up -d

# Check health
curl http://localhost:8081/health | jq .
```

### 2. Verify in New Relic

```sql
-- Check if metrics are arriving
FROM Metric 
SELECT count(*) 
WHERE metricName LIKE 'postgresql.%' 
SINCE 5 minutes ago

-- View query performance
FROM Metric
SELECT 
  average(postgresql.query.duration) as 'avg_duration',
  percentile(postgresql.query.duration, 95) as 'p95_duration',
  count(postgresql.query.duration) as 'query_count'
WHERE db.system = 'postgresql'
FACET db.operation
TIMESERIES 1 minute
SINCE 30 minutes ago

-- Connection pool monitoring
FROM Metric
SELECT 
  latest(postgresql.connection.count) as 'connections',
  latest(postgresql.connection.utilization) as 'utilization'
WHERE db.system = 'postgresql'
FACET connection.state
TIMESERIES 1 minute
```

## Configuration Deep Dive

### Resource Attributes (Entity Definition)

```toml
[outputs.otlp.resource_attributes]
"service.name" = "postgresql"              # Primary entity type
"service.namespace" = "production"         # Environment grouping
"db.system" = "postgresql"                 # Database type
"server.address" = "postgres-prod-01"      # Unique instance identifier
"cluster.name" = "production-cluster"      # Cluster grouping
```

### Dimension Configuration

```toml
[cardinality_limits]
max_query_fingerprints = 1000    # Limit unique queries
max_table_names = 500            # Limit tracked tables
max_user_names = 100             # Limit tracked users

[dimensions.allowlists]
important_users = ["app_user", "admin_user"]        # Always track these
important_applications = ["web_app", "api_service"]  # Priority apps
important_schemas = ["public", "analytics"]          # Key schemas
```

### Metric Filtering

```yaml
# In otel-collector-newrelic.yaml
filter/cardinality:
  metrics:
    metric:
      # Drop system queries
      - 'attributes["query.category"] == "system"'
      # Drop monitoring queries
      - 'attributes["query.text"] != nil and IsMatch(attributes["query.text"], ".*pg_stat.*")'
```

## Advanced Features

### 1. Query Categorization

The collector automatically categorizes queries:

- **system**: PostgreSQL internal queries
- **vacuum**: Maintenance operations
- **ddl**: Schema changes
- **dml**: Data modifications
- **select**: Read queries

### 2. Smart Sampling

For high-volume environments:

```toml
[slow_queries]
sample_rate = 0.1  # Sample 10% of queries
min_duration_ms = 1000  # Only track queries > 1s
```

### 3. Lock Analysis

Track lock contention with dimensional data:

```sql
FROM Metric
SELECT sum(postgresql.lock.wait_time) as 'total_wait'
WHERE db.system = 'postgresql'
FACET lock.type, lock.mode, table.name
SINCE 1 hour ago
```

## Monitoring Best Practices

### 1. Create Dashboards

Key widgets for PostgreSQL monitoring:

- **Query Performance**: p50, p95, p99 latencies by operation
- **Connection Pool**: Utilization percentage over time
- **Lock Contention**: Wait times by table and lock type
- **Query Volume**: Requests per second by category

### 2. Set Up Alerts

Critical alerts for production:

```sql
-- High query latency
FROM Metric
SELECT percentile(postgresql.query.duration, 95)
WHERE db.system = 'postgresql'
FACET db.operation

-- Connection pool exhaustion
FROM Metric
SELECT latest(postgresql.connection.utilization)
WHERE db.system = 'postgresql'

-- Deadlock detection
FROM Metric
SELECT sum(postgresql.deadlock.count)
WHERE db.system = 'postgresql'
```

### 3. Entity Synthesis

New Relic creates PostgreSQL entities based on resource attributes:

- Entity Type: `POSTGRESQL_INSTANCE`
- Entity Name: `{service.name}-{server.address}`
- Entity Tags: All resource attributes

## Troubleshooting

### 1. No Metrics in New Relic

Check OTEL Collector logs:
```bash
docker logs otel-collector-newrelic --tail 100
```

Common issues:
- Invalid license key
- Wrong endpoint (use https://otlp.nr-data.net:4317)
- Network connectivity

### 2. High Cardinality Warnings

Check cardinality status:
```bash
curl http://localhost:8081/health | jq .cardinality
```

Adjust limits in config:
```toml
[cardinality_limits]
max_query_fingerprints = 500  # Reduce from 1000
```

### 3. Missing Dimensions

Verify allowlists include important values:
```toml
[dimensions.allowlists]
important_users = ["app_user", "analytics_user", "admin_user"]
```

## Performance Tuning

### 1. Collection Intervals

Balance freshness vs. load:
```toml
[slow_queries]
interval = 60  # Increase from 30s for less critical metrics

[connections]
interval = 30  # Keep frequent for active monitoring
```

### 2. Batch Settings

Optimize for New Relic ingestion:
```yaml
batch:
  timeout: 5s              # Send every 5 seconds
  send_batch_size: 1000    # Or when 1000 metrics accumulated
```

### 3. Memory Management

OTEL Collector memory limits:
```yaml
memory_limiter:
  limit_mib: 512
  spike_limit_mib: 128
  check_interval: 1s
```

## Integration with New Relic Features

### 1. Service Maps

PostgreSQL instances appear in service maps when:
- APM agents report database calls
- Resource attributes match database spans

### 2. Logs in Context

Correlate logs with metrics:
```toml
[outputs.otlp.resource_attributes]
"service.instance.id" = "postgres-prod-01"  # Match with log attributes
```

### 3. Workloads

Group PostgreSQL instances:
```sql
-- Create workload
name: 'Production PostgreSQL'
query: 'db.system = "postgresql" AND service.namespace = "production"'
```

## Next Steps

1. **Extend Metrics**: Add table statistics, index usage, vacuum progress
2. **Add Traces**: Instrument application queries with OpenTelemetry
3. **Log Integration**: Ship PostgreSQL logs with proper parsing
4. **Custom Dashboards**: Build team-specific views
5. **Automation**: Use Terraform for dashboard/alert provisioning

The solution provides enterprise-grade PostgreSQL monitoring with full dimensional metrics support, optimized specifically for New Relic's strengths in data analysis and visualization.