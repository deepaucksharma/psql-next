# MongoDB Maximum Metrics Extraction Guide

This guide demonstrates how to extract 90+ metrics from MongoDB using only stock OpenTelemetry components.

## Overview

The `mongodb-maximum-extraction.yaml` configuration demonstrates:
- **90+ distinct metrics** from MongoDB
- **Real-time operation monitoring** via currentOp
- **WiredTiger storage engine stats**
- **Replication and oplog metrics**
- **Lock and concurrency analysis**
- **Atlas cloud metrics** (if applicable)

## Quick Start

```bash
# 1. Set environment variables
export MONGODB_HOST=localhost
export MONGODB_PORT=27017
export MONGODB_USER=admin
export MONGODB_PASSWORD=your_password
export NEW_RELIC_LICENSE_KEY=your_license_key

# 2. Run the collector
docker run -d \
  --name otel-mongodb-max \
  -v $(pwd)/configs/mongodb-maximum-extraction.yaml:/etc/otelcol/config.yaml \
  -e MONGODB_HOST \
  -e MONGODB_PORT \
  -e MONGODB_USER \
  -e MONGODB_PASSWORD \
  -e NEW_RELIC_LICENSE_KEY \
  -p 8890:8890 \
  otel/opentelemetry-collector-contrib:latest
```

## Prerequisites

### 1. Create Monitoring User

```javascript
// Connect to admin database
use admin

// Create monitoring user
db.createUser({
  user: "otel_monitor",
  pwd: "secure_password",
  roles: [
    { role: "clusterMonitor", db: "admin" },
    { role: "read", db: "local" },
    { role: "readAnyDatabase", db: "admin" }
  ]
})
```

### 2. Enable Profiling (Optional)

```javascript
// Enable profiling for slow query detection
db.setProfilingLevel(1, { slowms: 100 })

// Check profiling status
db.getProfilingStatus()
```

## Metrics Categories

### 1. Core MongoDB Metrics (50+)

The standard MongoDB receiver provides:
- **Cache Operations**: hits, misses, evictions
- **Collections**: count per database
- **Connections**: current, available, used
- **Cursors**: open, timed out
- **Documents**: operations count
- **Global Lock**: acquisition time
- **Index Usage**: access counts
- **Memory**: resident, virtual, page faults
- **Network I/O**: bytes in/out, requests
- **Operations**: command rates by type
- **Replication**: lag, oplog window
- **Storage**: data size, index size

### 2. Real-Time Operations (currentOp)

Active operation monitoring:
- **Active Sessions**: Count by state
- **Long-Running Queries**: Operations >10s
- **Lock Wait Analysis**: Blocked operations
- **Operation Types**: Distribution of commands

### 3. Atlas Cloud Metrics (40+)

If using MongoDB Atlas:
- **Disk IOPS**: Read/write operations
- **Disk Latency**: Average and max
- **CPU Usage**: System and process level
- **Memory Metrics**: Cache, heap usage
- **Network Metrics**: Connections, throughput
- **Query Targeting**: Scanned vs returned
- **Oplog Details**: Rate and window

### 4. WiredTiger Storage

Internal storage metrics:
- **Cache Statistics**: Dirty pages, evictions
- **Checkpoint Operations**: Duration, frequency
- **Transaction Metrics**: Active, rolled back
- **Compression Ratios**: Data efficiency

### 5. Replication Metrics

Replica set health:
- **Replication Lag**: Secondary delay
- **Oplog Size**: Current usage
- **Election Metrics**: Primary changes
- **Member States**: Health per node

## Configuration Breakdown

### Multi-Source Collection

```yaml
receivers:
  # Core MongoDB metrics
  mongodb:
    hosts:
      - endpoint: ${MONGODB_HOST}:${MONGODB_PORT}
    collection_interval: 10s
    
  # Atlas cloud metrics
  mongodbatlas:
    projects:
      - name: ${MONGODB_ATLAS_PROJECT}
    collection_interval: 60s
    
  # Custom metrics via exec
  exec/mongodb_custom:
    commands:
      - 'echo "db.currentOp()" | mongosh ...'
    collection_interval: 5s
```

### Intelligent Processing

```yaml
transform/add_metadata:
  metric_statements:
    # Classify operation performance
    - set(attributes["operation.performance"], "slow") 
      where name == "mongodb.operation.latency.time" 
      and value >= 100
      
    # Classify lock severity
    - set(attributes["lock.severity"], "high") 
      where name == "mongodb.lock.acquire.wait_time" 
      and value >= 1000
```

## Performance Tuning

### 1. Optimize currentOp Frequency

```yaml
exec/mongodb_custom:
  collection_interval: 5s  # Adjust based on load
  timeout: 10s  # Prevent hanging
```

### 2. Filter System Databases

```yaml
filter/reduce_cardinality:
  metrics:
    metric:
      # Exclude system databases
      - 'attributes["database.name"] == "admin"'
      - 'attributes["database.name"] == "config"'
      - 'attributes["database.name"] == "local"'
```

### 3. Collection Interval Strategy

- **5s**: currentOp (critical operations)
- **10s**: Core metrics
- **60s**: Atlas metrics, statistics

## Monitoring Best Practices

### 1. Key Metrics to Alert On

- `mongodb.lock.acquire.wait_time` > 1000ms
- `mongodb.connections.current` > 80% of max
- `mongodb.replication.lag` > 60s
- `mongodb.cursors.timeout.count` increasing
- `mongodb.memory.usage` > 80% available

### 2. Dashboard Structure

- **Overview**: Connections, operations/sec, errors
- **Performance**: Query latency, lock waits
- **Replication**: Lag, oplog metrics
- **Resources**: Memory, disk, network
- **WiredTiger**: Cache efficiency, checkpoints

### 3. Query Optimization Workflow

1. Monitor operation latency trends
2. Identify slow operations via currentOp
3. Check index usage statistics
4. Review lock wait patterns
5. Optimize based on findings

## Troubleshooting

### No Metrics Appearing

```javascript
// Verify connectivity
db.runCommand({ ping: 1 })

// Check user permissions
db.runCommand({ connectionStatus: 1, showPrivileges: true })

// Test currentOp access
db.currentOp()
```

### Missing Atlas Metrics

1. Verify Atlas API credentials:
```bash
export MONGODB_ATLAS_PUBLIC_KEY=your_public_key
export MONGODB_ATLAS_PRIVATE_KEY=your_private_key
```

2. Check project name matches exactly
3. Ensure API key has project read access

### High Memory Usage

1. Reduce currentOp frequency
2. Limit metrics per receiver:
```yaml
mongodb:
  metrics:
    mongodb.index.access.count:
      enabled: false  # Disable high-cardinality metrics
```

## Example Queries

### Find Slow Operations

```sql
SELECT average(mongodb.operation.latency.time) 
FROM Metric 
WHERE deployment.mode = 'config-only-mongodb-max' 
AND operation.performance = 'slow'
FACET operation.type 
SINCE 1 hour ago
```

### Lock Wait Analysis

```sql
SELECT max(mongodb.lock.acquire.wait_time) 
FROM Metric 
WHERE deployment.mode = 'config-only-mongodb-max' 
FACET database.name, collection.name 
SINCE 30 minutes ago
```

### Connection Pool Health

```sql
SELECT latest(mongodb.connections.current) as 'Active',
       latest(mongodb.connections.available) as 'Available'
FROM Metric 
WHERE deployment.mode = 'config-only-mongodb-max' 
TIMESERIES
```

### Replication Lag Tracking

```sql
SELECT max(mongodb.replication.lag) 
FROM Metric 
WHERE deployment.mode = 'config-only-mongodb-max' 
FACET mongodb.replica_set.member 
SINCE 6 hours ago
```

## Advanced Features

### 1. Custom Business Metrics

Add application-specific metrics:
```yaml
exec/business_metrics:
  commands:
    - 'mongosh --eval "db.users.count()"'
    - 'mongosh --eval "db.orders.count({status: \'pending\'})"'
  collection_interval: 300s
```

### 2. Aggregation Pipeline Metrics

Monitor complex aggregations:
```yaml
exec/aggregation_stats:
  commands:
    - |
      mongosh --eval '
        db.orders.aggregate([
          {$group: {_id: "$status", count: {$sum: 1}}}
        ])'
  collection_interval: 60s
```

### 3. Change Stream Monitoring

Track real-time changes:
```yaml
exec/change_streams:
  commands:
    - 'mongosh --eval "db.adminCommand({getParameter: 1, changeStreamOptions: 1})"'
  collection_interval: 30s
```

## Conclusion

This configuration extracts 90+ metrics from MongoDB using only OpenTelemetry configuration:
- ✅ No custom code required
- ✅ Real-time operation visibility
- ✅ Comprehensive storage metrics
- ✅ Atlas cloud integration
- ✅ Intelligent classification

The patterns work with MongoDB 4.0+ and are compatible with replica sets and sharded clusters.