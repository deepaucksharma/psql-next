# Aerospike Receiver - Domain-Driven Design Analysis

## Executive Summary

The Aerospike receiver implements a distributed database monitoring system organized into four primary bounded contexts:

1. **Node Metrics Context**: Collects system-level performance metrics from individual nodes
2. **Namespace Analytics Context**: Monitors data container statistics and operations
3. **Cluster Topology Context**: Manages node discovery and connection strategies
4. **Info Protocol Context**: Handles Aerospike's text-based statistics protocol

## Bounded Contexts

### 1. Node Metrics Context

**Core Responsibility**: Collect and transform node-level performance indicators

**Aggregates**:
- **NodeScraper** (Aggregate Root)
  - Orchestrates node metric collection
  - Manages connection lifecycle
  - Transforms info responses to metrics
  - Handles authentication and TLS

**Value Objects**:
- **NodeName**: Unique node identifier
- **NodeStats**: Parsed statistics map
- **ConnectionMetrics**: Client, fabric, heartbeat counts
- **MemoryMetrics**: System and data memory usage
- **QueryMetrics**: Success, error, timeout counts

**Domain Services**:
- **InfoParser**: Parses semicolon-delimited responses
- **MetricEmitter**: Transforms stats to OTel metrics
- **ConnectionManager**: Handles node connections
- **AuthenticationService**: Manages enterprise auth

**Invariants**:
- Node name must be non-empty
- Connection counts must be non-negative
- Memory values in bytes
- Query counts are cumulative

### 2. Namespace Analytics Context

**Core Responsibility**: Monitor data container performance and capacity

**Aggregates**:
- **NamespaceMonitor** (Aggregate Root)
  - Tracks per-namespace metrics
  - Monitors storage utilization
  - Analyzes query patterns
  - Measures transaction rates

**Value Objects**:
- **NamespaceName**: Container identifier
- **DiskUsage**: Used and free bytes
- **MemoryUsage**: Data and index memory
- **TransactionStats**: Read/write operations
- **ScanMetrics**: Aggregation and UDF scans

**Domain Services**:
- **NamespaceEnumerator**: Lists all namespaces
- **CapacityCalculator**: Computes usage percentages
- **OperationAnalyzer**: Categorizes operations
- **ScanTracker**: Monitors scan operations

**Invariants**:
- Namespace names must be valid
- Used space ≤ total space
- Transaction counts monotonic
- Scan operations categorized correctly

### 3. Cluster Topology Context

**Core Responsibility**: Manage cluster discovery and connection strategies

**Aggregates**:
- **ClusterConnector** (Aggregate Root)
  - Discovers cluster nodes
  - Manages connection pools
  - Handles failover
  - Supports subset mode

**Value Objects**:
- **Endpoint**: Host:port combination
- **ClusterMode**: Full or subset collection
- **NodeEndpoint**: Discovered node address
- **ConnectionPolicy**: Timeout and retry settings

**Domain Services**:
- **NodeDiscoverer**: Finds cluster members
- **ConnectionPoolManager**: Manages connections
- **LoadBalancer**: Distributes requests
- **FailoverHandler**: Manages node failures

**Invariants**:
- Endpoints must be valid addresses
- Port range 1-65535
- Cluster mode determines discovery
- At least one node must be reachable

### 4. Info Protocol Context

**Core Responsibility**: Abstract Aerospike's text-based statistics protocol

**Aggregates**:
- **InfoClient** (Aggregate Root)
  - Executes info commands
  - Parses responses
  - Handles protocol errors
  - Manages command queue

**Value Objects**:
- **InfoCommand**: Command string ("node", "statistics")
- **InfoResponse**: Raw text response
- **ParsedStats**: Key-value map
- **ProtocolError**: Command failures

**Domain Services**:
- **CommandExecutor**: Sends info commands
- **ResponseParser**: Parses delimited format
- **ErrorHandler**: Manages protocol errors
- **CommandValidator**: Ensures valid commands

**Invariants**:
- Commands must be non-empty
- Responses use semicolon delimiter
- Key-value pairs use equals separator
- Malformed entries logged but skipped

## Domain Events

### Node Events
- `NodeConnected`: Connection established
- `NodeStatsCollected`: Metrics retrieved
- `NodeDisconnected`: Connection lost
- `AuthenticationFailed`: Invalid credentials

### Namespace Events
- `NamespaceDiscovered`: New namespace found
- `CapacityThresholdReached`: High usage detected
- `ScanInitiated`: Scan operation started
- `QueryPatternChanged`: Usage shift detected

### Cluster Events
- `ClusterDiscovered`: Full topology known
- `NodeAdded`: New node joined
- `NodeRemoved`: Node left cluster
- `TopologyChanged`: Cluster reconfigured

### Protocol Events
- `InfoCommandSent`: Request initiated
- `ResponseReceived`: Data returned
- `ParseError`: Malformed response
- `CommandTimeout`: No response received

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **Node** | Aerospike | Database server instance | Has unique node ID |
| **Namespace** | Aerospike | Data container/database | Contains sets and records |
| **Set** | Aerospike | Table equivalent | Within namespace |
| **Info Protocol** | Protocol | Text command interface | Request-response pattern |
| **Fabric** | Network | Inter-node communication | Cluster mesh network |
| **Heartbeat** | Network | Node liveness check | Periodic health signal |
| **UDF** | Operations | User-Defined Function | Lua scripts on server |
| **TTL** | Data | Time To Live | Record expiration |

## Anti-Patterns Addressed

1. **No Data Access**: Only metadata/statistics collected
2. **No Cluster State Changes**: Read-only monitoring
3. **No Heavy Operations**: Lightweight info commands
4. **No Connection Flooding**: Connection pooling
5. **No Missing Nodes**: Supports partial collection

## Architectural Patterns

### 1. Strategy Pattern (Connection Modes)
```go
type Cluster interface {
    GetNodes() ([]*Node, error)
    GetHosts() []*as.Host
    Close() error
}

// FullCluster for cluster-wide collection
// SubsetCluster for specific nodes only
```

### 2. Factory Pattern
```go
func NewFactory() receiver.Factory {
    return receiver.NewFactory(
        metadata.Type,
        createDefaultConfig,
        receiver.WithMetrics(createMetricsReceiver, stability),
    )
}
```

### 3. Parser Pattern
```go
// Info protocol parser
func parseInfo(response string) map[string]string {
    // Parse "key1=value1;key2=value2" format
}
```

### 4. Parallel Collection Pattern
```go
// Concurrent namespace collection
for _, ns := range namespaces {
    wg.Add(1)
    go func(namespace string) {
        defer wg.Done()
        r.scrapeNamespace(node, namespace)
    }(ns)
}
```

## Testing Strategy

### Unit Testing
- Mock info client for scraper tests
- Test parser with various formats
- Validate metric transformations
- Test configuration validation

### Integration Testing
- Test against real Aerospike
- Validate all info commands
- Test authentication scenarios
- Verify TLS connections

### Cluster Testing
- Multi-node collection
- Node failure scenarios
- Topology changes
- Partial availability

## Performance Considerations

1. **Connection Pooling**: Reuse node connections
2. **Parallel Collection**: Concurrent namespace queries
3. **Lightweight Protocol**: Text-based info commands
4. **Selective Collection**: Configure specific metrics
5. **Efficient Parsing**: Single-pass parser

## Security Model

1. **Authentication**: Username/password for Enterprise
2. **Authorization**: Read-only info access
3. **Encryption**: TLS support with tlsname
4. **No Data Access**: Statistics only
5. **Audit Trail**: Connection logging

## Evolution Points

1. **XDR Metrics**: Cross-datacenter replication
2. **Secondary Index Stats**: Index performance
3. **Batch Operations**: Batch read/write metrics
4. **Strong Consistency**: SC mode metrics
5. **Rack Awareness**: Topology metrics

## Error Handling Philosophy

1. **Partial Success**: Continue on node failures
2. **Connection Resilience**: Recreate failed clients
3. **Parse Tolerance**: Skip malformed entries
4. **Clear Diagnostics**: Log command context
5. **Graceful Degradation**: Collect available data

## Conclusion

The Aerospike receiver demonstrates distributed system DDD principles:
- Clear separation between node and namespace concerns
- Flexible topology management for various deployments
- Clean abstraction over info protocol
- Production-ready error handling
- Scalable architecture for large clusters

The design successfully monitors Aerospike deployments from single nodes to large distributed clusters while maintaining simplicity and reliability.