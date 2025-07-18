# Redis Enhanced Receiver

Enhanced Redis receiver with support for cluster mode, sentinel, and advanced metrics collection.

## Features

### Core Features
- **Server Info Metrics** - All sections from INFO command
- **Command Statistics** - Per-command execution metrics
- **Memory Statistics** - Detailed memory usage breakdown
- **Latency Monitoring** - Event latency tracking and histograms
- **Slow Log Analysis** - Track and analyze slow queries
- **Client Monitoring** - Connected client details and buffers

### Cluster Support
- **Multi-Node Metrics** - Collect metrics from all cluster nodes
- **Slot Distribution** - Monitor slot assignment across nodes
- **Cluster Health** - State, known nodes, slot failures
- **Per-Node Collection** - Individual node performance metrics
- **Automatic Discovery** - Discover all nodes in cluster

### Sentinel Support
- **Master Monitoring** - Track master status and failovers
- **Replica Monitoring** - Replica health and lag
- **Sentinel Metrics** - Sentinel-specific health metrics
- **Automatic Failover** - Handle master changes transparently

### Advanced Features
- **Stream Metrics** - Redis Streams monitoring
- **Module Support** - Track loaded modules
- **Custom Commands** - Execute custom Redis commands for metrics
- **TLS Support** - Secure connections with certificate validation
- **Connection Pooling** - Efficient connection management

## Configuration

### Basic Configuration

```yaml
receivers:
  redis:
    endpoint: localhost:6379
    collection_interval: 60s
    metrics:
      server_info:
        server: true
        clients: true
        memory: true
        persistence: true
        stats: true
        replication: true
        cpu: true
        keyspace: true
      command_stats: true
      keyspace_stats: true
      latency_stats: true
      memory_stats: true
```

### Redis Cluster Configuration

```yaml
receivers:
  redis:
    collection_interval: 60s
    cluster:
      enabled: true
      nodes:
        - redis-cluster-1:6379
        - redis-cluster-2:6379
        - redis-cluster-3:6379
      collect_per_node_metrics: true
      collect_cluster_info: true
      collect_slot_metrics: true
      route_by_latency: true
```

### Redis Sentinel Configuration

```yaml
receivers:
  redis:
    collection_interval: 60s
    sentinel:
      enabled: true
      master_name: mymaster
      sentinel_addrs:
        - sentinel-1:26379
        - sentinel-2:26379
        - sentinel-3:26379
      collect_sentinel_metrics: true
```

### TLS Configuration

```yaml
receivers:
  redis:
    endpoint: secure-redis:6379
    tls:
      enabled: true
      ca_file: /path/to/ca.pem
      cert_file: /path/to/cert.pem
      key_file: /path/to/key.pem
      server_name: redis.example.com
```

### Slow Log Configuration

```yaml
receivers:
  redis:
    endpoint: localhost:6379
    slow_log:
      enabled: true
      max_entries: 128
      include_commands: true
      track_position: true  # Only process new entries
```

### Custom Metrics Configuration

```yaml
receivers:
  redis:
    endpoint: localhost:6379
    metrics:
      custom_commands:
        - name: redis.custom.stream_length
          command: XLEN
          args: ["mystream"]
          type: gauge
          description: Length of mystream
          
        - name: redis.custom.list_length
          command: LLEN
          args: ["mylist"]
          type: gauge
          description: Length of mylist
```

## Metrics

### Server Metrics
- `redis.server.uptime` - Server uptime in seconds
- `redis.clients.connected` - Number of connected clients
- `redis.clients.blocked` - Number of blocked clients
- `redis.memory.used` - Total memory allocated by Redis
- `redis.memory.rss` - Memory allocated by the OS
- `redis.memory.fragmentation_ratio` - Memory fragmentation ratio
- `redis.evicted_keys` - Total number of evicted keys
- `redis.expired_keys` - Total number of expired keys

### Performance Metrics
- `redis.connections.received` - Total connections received
- `redis.commands.processed` - Total commands processed
- `redis.commands.per_second` - Instantaneous ops per second
- `redis.net.input.bytes` - Total network input
- `redis.net.output.bytes` - Total network output
- `redis.keyspace.hits` - Number of successful key lookups
- `redis.keyspace.misses` - Number of failed key lookups
- `redis.keyspace.hits.ratio` - Hit rate ratio

### Command Metrics
- `redis.commands.calls` - Number of calls per command
- `redis.commands.usec` - Total microseconds per command
- `redis.commands.usec_per_call` - Average microseconds per call

### Persistence Metrics
- `redis.persistence.rdb.changes_since_last_save` - Changes since last save
- `redis.persistence.rdb.saves_in_progress` - Background saves in progress
- `redis.persistence.aof.enabled` - AOF enabled status
- `redis.persistence.aof.rewrite_in_progress` - AOF rewrite status
- `redis.persistence.aof.current_size` - Current AOF file size

### Replication Metrics
- `redis.replication.role` - Current role (master/slave)
- `redis.replication.connected_slaves` - Number of connected slaves
- `redis.replication.master_link_up` - Master link status
- `redis.replication.slave.lag` - Per-slave replication lag
- `redis.replication.slave.offset` - Slave replication offset

### Cluster Metrics
- `redis.cluster.state` - Cluster state (ok/fail)
- `redis.cluster.slots_assigned` - Number of assigned slots
- `redis.cluster.slots_ok` - Slots in OK state
- `redis.cluster.slots_fail` - Slots in FAIL state
- `redis.cluster.known_nodes` - Number of known nodes
- `redis.cluster.node.connected` - Per-node connection status
- `redis.cluster.node.slots` - Slots per node

### Memory Breakdown Metrics
- `redis.memory.peak_allocated` - Peak allocated memory
- `redis.memory.startup_allocated` - Startup memory
- `redis.memory.replication_backlog` - Replication backlog size
- `redis.memory.clients_normal` - Memory used by clients
- `redis.memory.aof_buffer` - AOF buffer size
- `redis.memory.db.overhead` - Per-database overhead

### Latency Metrics
- `redis.latency.latest` - Latest latency per event type
- `redis.latency.percentile` - Latency percentiles (p50, p99, p99.9)

### Slow Log Metrics
- `redis.slowlog.count` - Number of slow log entries
- `redis.slowlog.max_duration` - Maximum slow query duration
- `redis.slowlog.by_command` - Slow queries per command type

## Resource Attributes

All metrics include:
- `redis.instance` - Redis instance endpoint
- `redis.database` - Database number (if applicable)
- Custom attributes from configuration

Additional attributes for specific metrics:
- `command` - Command name for command stats
- `database` - Database name for keyspace stats
- `node` - Node address for cluster metrics
- `event` - Event type for latency metrics

## Requirements

- Redis 2.8+ (basic features)
- Redis 5.0+ (stream metrics)
- Redis 6.0+ (ACL support)
- Redis 7.0+ (latency histograms)

## Performance Considerations

1. **Client List** - Can be expensive on instances with many connections
2. **Cluster Per-Node Metrics** - Increases load proportionally to cluster size
3. **Slow Log** - Consider `max_entries` setting for busy instances
4. **Memory Stats** - MEMORY STATS command can be slow on large datasets

## Troubleshooting

### Connection Issues
- Verify endpoint format (host:port)
- Check authentication credentials
- Ensure network connectivity
- Verify TLS configuration if enabled

### Missing Metrics
- Check Redis version compatibility
- Verify user permissions (INFO, CONFIG GET, etc.)
- Some metrics require specific Redis configurations

### Cluster Issues
- Ensure all cluster nodes are accessible
- Check cluster state with CLUSTER INFO
- Verify network connectivity between collector and all nodes

### High Memory Usage
- Reduce collection frequency
- Disable expensive metrics (client_list, memory_stats)
- Limit cluster per-node collection