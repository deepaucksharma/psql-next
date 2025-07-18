# MongoDB Enhanced Receiver

Enhanced MongoDB receiver with support for replica sets, sharding, and advanced metrics collection.

## Features

### Core Features
- **Server Status Metrics** - Connections, memory, operations, locks, etc.
- **Database & Collection Stats** - Size, object counts, indexes
- **Index Statistics** - Usage patterns and access counts  
- **Current Operations** - Active queries and their durations
- **WiredTiger Metrics** - Cache and transaction statistics

### Replica Set Support
- **Replica Set Status** - Member states, health, ping times
- **Oplog Metrics** - Size, window, entry counts
- **Replication Lag** - Per-member lag tracking with threshold alerts
- **Automatic Primary Detection** - Tracks primary changes

### Sharding Support
- **Balancer Status** - Enabled state and active migrations
- **Chunk Distribution** - Chunks per shard tracking
- **Config Server Metrics** - When connected to mongos

### Query Monitoring
- **Profile Collection** - Slow query detection
- **Query Shapes** - Normalized query patterns
- **Execution Plans** - Optional plan collection
- **Operation Types** - Breakdown by operation type

### Custom Metrics
- Define custom metrics using MongoDB commands
- Extract values using JSON paths
- Support for gauge, counter, and histogram types

## Configuration

### Basic Configuration

```yaml
receivers:
  mongodb:
    uri: mongodb://localhost:27017
    collection_interval: 60s
    metrics:
      server_status: true
      database_stats: true
      collection_stats: true
      index_stats: true
```

### Replica Set Configuration

```yaml
receivers:
  mongodb:
    uri: mongodb://mongo1:27017,mongo2:27017,mongo3:27017/?replicaSet=rs0
    collection_interval: 60s
    replica_set:
      enabled: true
      collect_oplog_metrics: true
      collect_repl_lag_metrics: true
      oplog_window: 1h
      lag_threshold: 30s
```

### Sharded Cluster Configuration

```yaml
receivers:
  mongodb:
    uri: mongodb://mongos1:27017,mongos2:27017
    collection_interval: 60s
    sharding:
      enabled: true
      collect_chunk_metrics: true
      collect_balancer_metrics: true
      chunk_metrics_interval: 5m
```

### TLS Configuration

```yaml
receivers:
  mongodb:
    uri: mongodb://secure-mongo:27017
    tls:
      enabled: true
      ca_file: /path/to/ca.pem
      cert_file: /path/to/cert.pem
      key_file: /path/to/key.pem
      insecure: false
```

### Query Monitoring Configuration

```yaml
receivers:
  mongodb:
    uri: mongodb://localhost:27017
    query_monitoring:
      enabled: true
      profile_level: 1  # 0=off, 1=slow ops, 2=all
      slow_op_threshold: 100ms
      max_queries: 1000
      collect_query_plans: true
      collect_query_shapes: true
```

### Custom Metrics Configuration

```yaml
receivers:
  mongodb:
    uri: mongodb://localhost:27017
    metrics:
      custom_metrics:
        - name: mongodb.custom.connection_pool_size
          command: connectionPoolStats
          database: admin
          value_path: totalCreated
          type: gauge
          description: Total connections created
          labels:
            pool_name: hosts.0.poolName
```

## Metrics

### Server Metrics
- `mongodb.connections.current` - Current connection count
- `mongodb.connections.available` - Available connections
- `mongodb.connections.total_created` - Total connections created
- `mongodb.memory.resident` - Resident memory (MiB)
- `mongodb.memory.virtual` - Virtual memory (MiB)
- `mongodb.operations.count` - Operations by type
- `mongodb.network.bytes_in` - Network bytes received
- `mongodb.network.bytes_out` - Network bytes sent
- `mongodb.locks.acquire.count` - Lock acquisitions by type/mode
- `mongodb.locks.acquire.time` - Time acquiring locks

### Database Metrics
- `mongodb.database.size` - Data size (bytes)
- `mongodb.database.storage_size` - Storage size (bytes)
- `mongodb.database.index_size` - Index size (bytes)
- `mongodb.database.collections` - Number of collections
- `mongodb.database.objects` - Number of objects
- `mongodb.database.indexes` - Number of indexes

### Collection Metrics
- `mongodb.collection.size` - Collection size (bytes)
- `mongodb.collection.storage_size` - Storage size (bytes)
- `mongodb.collection.count` - Document count
- `mongodb.collection.avg_obj_size` - Average object size
- `mongodb.collection.indexes` - Number of indexes
- `mongodb.collection.index_size` - Total index size
- `mongodb.index.size` - Individual index sizes
- `mongodb.index.access.count` - Index access counts

### Replica Set Metrics
- `mongodb.replica_set.state` - Member state code
- `mongodb.replica_set.member.state` - Per-member state
- `mongodb.replica_set.member.health` - Member health (0/1)
- `mongodb.replica_set.member.ping` - Ping time (ms)
- `mongodb.replica_set.lag` - Replication lag (seconds)
- `mongodb.replica_set.lag.threshold_exceeded` - Lag alert
- `mongodb.oplog.size` - Oplog size (bytes)
- `mongodb.oplog.window` - Oplog time window (seconds)

### Sharding Metrics
- `mongodb.balancer.enabled` - Balancer status (0/1)
- `mongodb.balancer.migrations.active` - Active migrations
- `mongodb.chunks.count` - Chunks per shard

### WiredTiger Metrics
- `mongodb.wiredtiger.cache.bytes_in_cache` - Cache usage
- `mongodb.wiredtiger.cache.max_bytes` - Max cache size
- `mongodb.wiredtiger.cache.bytes_read` - Bytes read
- `mongodb.wiredtiger.cache.bytes_written` - Bytes written
- `mongodb.wiredtiger.cache.evictions` - Page evictions
- `mongodb.wiredtiger.transactions.begins` - Transactions started
- `mongodb.wiredtiger.transactions.commits` - Transactions committed
- `mongodb.wiredtiger.transactions.rollbacks` - Transactions rolled back

## Resource Attributes

All metrics include:
- `mongodb.instance` - MongoDB host/port
- `mongodb.database` - Database name (when applicable)
- `mongodb.collection` - Collection name (when applicable)
- Custom attributes from configuration

## Requirements

- MongoDB 3.6+ (4.0+ recommended)
- Network access to MongoDB instance(s)
- Appropriate MongoDB permissions:
  - `clusterMonitor` role for basic metrics
  - `readAnyDatabase` for database/collection stats
  - Additional permissions for profiling

## Performance Considerations

1. **Collection Stats** - Can be expensive on large deployments
2. **Index Stats** - Uses aggregation pipeline, may impact performance
3. **Profile Collection** - Affects database performance when enabled
4. **Chunk Metrics** - Can be slow on large sharded clusters

## Troubleshooting

### Connection Issues
- Verify URI format and credentials
- Check network connectivity
- Ensure TLS configuration matches server

### Missing Metrics
- Check user permissions
- Verify feature availability in MongoDB version
- Check if replica set/sharding is properly configured

### Performance Impact
- Increase collection interval for large deployments
- Disable expensive metrics (index_stats, collection_stats)
- Use database/collection filters to limit scope