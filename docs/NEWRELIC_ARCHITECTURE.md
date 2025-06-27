# PostgreSQL Collector - New Relic Dimensional Metrics Architecture

## Overview

This document describes the architectural design for sending PostgreSQL metrics to New Relic using OpenTelemetry Protocol (OTLP) with a focus on dimensional metrics that leverage New Relic's powerful querying and visualization capabilities.

## Dimensional Metrics Model

### Core Concepts

1. **Metrics as Measurements**: Each metric represents a specific measurement (e.g., query execution time)
2. **Dimensions as Context**: Attributes provide dimensional context for filtering and grouping
3. **Entity Synthesis**: Resource attributes map to New Relic entities
4. **Faceted Queries**: Dimensions enable NRQL faceted queries

### Metric Design Principles

```
Metric Name: postgresql.query.duration
Dimensions:
  - database.name
  - schema.name
  - query.operation (SELECT, INSERT, UPDATE, DELETE)
  - query.fingerprint (normalized query hash)
  - query.id
  - host.name
  - service.name
  - service.instance.id
```

## New Relic OTLP Requirements

### Endpoint Configuration

```yaml
# US Datacenter
endpoint: https://otlp.nr-data.net:4317  # gRPC
endpoint: https://otlp.nr-data.net:4318  # HTTP

# EU Datacenter  
endpoint: https://otlp.eu01.nr-data.net:4317  # gRPC
endpoint: https://otlp.eu01.nr-data.net:4318  # HTTP
```

### Authentication

```yaml
headers:
  api-key: "YOUR_NEW_RELIC_LICENSE_KEY"
```

### Aggregation Temporality

New Relic prefers **Delta temporality** for Counter and Histogram metrics:
- Counters: Report the change since last export
- Histograms: Report distribution since last export
- Gauges: Current value (no temporality)

## Metric Taxonomy

### 1. Query Performance Metrics

```yaml
postgresql.query.duration:
  type: Histogram
  unit: milliseconds
  description: Query execution duration
  dimensions:
    - database.name
    - schema.name
    - query.operation
    - query.fingerprint
    - query.normalized_text
    - user.name
    - application.name

postgresql.query.count:
  type: Counter
  unit: 1
  description: Number of query executions
  dimensions: [same as above]

postgresql.query.rows:
  type: Histogram
  unit: 1
  description: Rows returned/affected by queries
  dimensions: [same as above]
```

### 2. Wait Event Metrics

```yaml
postgresql.wait.duration:
  type: Histogram
  unit: milliseconds
  description: Wait event duration
  dimensions:
    - wait.event_type (Lock, IO, CPU, IPC, etc.)
    - wait.event_name
    - database.name
    - query.fingerprint
    - backend.type

postgresql.wait.count:
  type: Counter
  unit: 1
  description: Wait event occurrences
  dimensions: [same as above]
```

### 3. Connection Metrics

```yaml
postgresql.connection.count:
  type: Gauge
  unit: 1
  description: Current connection count
  dimensions:
    - connection.state (active, idle, idle_in_transaction)
    - database.name
    - user.name
    - application.name

postgresql.connection.utilization:
  type: Gauge
  unit: percent
  description: Connection pool utilization
  dimensions:
    - database.name
```

### 4. Lock Metrics

```yaml
postgresql.lock.wait_duration:
  type: Histogram
  unit: milliseconds
  description: Lock wait duration
  dimensions:
    - lock.type
    - lock.mode
    - database.name
    - table.name
    - blocking.query_fingerprint
    - blocked.query_fingerprint

postgresql.lock.deadlock.count:
  type: Counter
  unit: 1
  description: Deadlock occurrences
  dimensions:
    - database.name
```

### 5. Table/Index Metrics

```yaml
postgresql.table.size:
  type: Gauge
  unit: bytes
  description: Table size on disk
  dimensions:
    - database.name
    - schema.name
    - table.name

postgresql.table.rows:
  type: Gauge
  unit: 1
  description: Estimated row count
  dimensions: [same as above]

postgresql.index.scans:
  type: Counter
  unit: 1
  description: Index scan count
  dimensions:
    - database.name
    - schema.name
    - table.name
    - index.name

postgresql.table.sequential_scans:
  type: Counter
  unit: 1
  description: Sequential scan count
  dimensions:
    - database.name
    - schema.name
    - table.name
```

### 6. Replication Metrics

```yaml
postgresql.replication.lag:
  type: Gauge
  unit: bytes
  description: Replication lag in bytes
  dimensions:
    - replication.role (primary, standby)
    - replication.slot_name
    - replication.client_addr

postgresql.replication.delay:
  type: Gauge
  unit: milliseconds
  description: Replication delay
  dimensions: [same as above]
```

## Resource Attributes (Entity Mapping)

Resource attributes define the entity in New Relic:

```yaml
resource:
  attributes:
    # Service Identity
    service.name: "postgresql"
    service.namespace: "production"
    service.instance.id: "${HOSTNAME}:${PORT}"
    service.version: "15.3"
    
    # Host Information
    host.name: "${HOSTNAME}"
    host.id: "${HOST_ID}"
    
    # Cloud Provider (if applicable)
    cloud.provider: "aws"
    cloud.account.id: "123456789"
    cloud.region: "us-east-1"
    cloud.availability_zone: "us-east-1a"
    
    # Database Specific
    db.system: "postgresql"
    db.version: "15.3"
    db.connection_string: "postgresql://host:5432"
    
    # Environment
    deployment.environment: "production"
    
    # New Relic Specific
    newrelic.entity.type: "POSTGRESQL_INSTANCE"
    newrelic.entity.guid: "auto-generated"
```

## NRQL Query Examples

### Top Slow Queries
```sql
SELECT 
  average(postgresql.query.duration) as 'Avg Duration',
  count(postgresql.query.count) as 'Executions'
FROM Metric
WHERE database.name = 'production'
FACET query.normalized_text
SINCE 1 hour ago
LIMIT 20
```

### Wait Event Analysis
```sql
SELECT 
  sum(postgresql.wait.duration) as 'Total Wait Time'
FROM Metric
FACET wait.event_type, wait.event_name
WHERE database.name = 'production'
SINCE 1 hour ago
```

### Connection Pool Health
```sql
SELECT 
  latest(postgresql.connection.count) as 'Connections',
  latest(postgresql.connection.utilization) as 'Utilization %'
FROM Metric
FACET connection.state
WHERE service.instance.id = 'prod-db-1:5432'
TIMESERIES
```

### Lock Contention
```sql
SELECT 
  histogram(postgresql.lock.wait_duration) as 'Lock Wait Distribution'
FROM Metric
WHERE database.name = 'production'
FACET lock.type, table.name
SINCE 1 hour ago
```

## Implementation Guidelines

### 1. Metric Batching
- Batch size: 1000 metrics per request
- Export interval: 30 seconds
- Use compression (gzip)

### 2. Cardinality Management
- Limit query.fingerprint cardinality with smart normalization
- Use query.operation for high-level grouping
- Implement sampling for high-cardinality dimensions

### 3. Error Handling
- Implement exponential backoff for retries
- Buffer metrics during network issues
- Alert on export failures

### 4. Performance Optimization
- Pre-aggregate where possible
- Use resource detection for static attributes
- Implement efficient metric collection

## New Relic Integration Benefits

1. **Automatic Entity Synthesis**: PostgreSQL instances appear as entities
2. **Distributed Tracing**: Correlate database queries with application traces
3. **Alerting**: Create alerts on any dimension combination
4. **Dashboards**: Pre-built PostgreSQL dashboards with dimensional filtering
5. **AI-Powered Insights**: Anomaly detection on dimensional metrics
6. **Workload Management**: Group PostgreSQL instances into workloads

## Best Practices

1. **Consistent Naming**: Follow OpenTelemetry semantic conventions
2. **Meaningful Dimensions**: Include dimensions that enable troubleshooting
3. **Resource Efficiency**: Balance granularity with cardinality
4. **Time Alignment**: Ensure metrics align to collection intervals
5. **Entity Mapping**: Properly set resource attributes for entity synthesis