# SAP HANA Receiver - Domain-Driven Design Analysis

## Executive Summary

The SAP HANA receiver implements an in-memory database monitoring system organized into five primary bounded contexts:

1. **Service Management Context**: Monitors HANA services and their resource usage
2. **Memory Analytics Context**: Tracks column store, row store, and component memory
3. **Host Resource Context**: Monitors CPU, disk, and system-level resources
4. **Transaction & Lock Context**: Tracks transactions, blocks, and concurrency
5. **Schema Operations Context**: Monitors schema-level read/write/merge operations

## Bounded Contexts

### 1. Service Management Context

**Core Responsibility**: Monitor SAP HANA services and their operational status

**Aggregates**:
- **ServiceMonitor** (Aggregate Root)
  - Tracks service status (active/inactive)
  - Monitors thread counts and states
  - Measures service-specific memory
  - Identifies service roles

**Value Objects**:
- **ServiceName**: indexserver, nameserver, etc.
- **ServiceStatus**: Active or stopped state
- **ThreadCount**: Worker threads per service
- **ServiceMemory**: Memory per component
- **ServicePort**: Network endpoint

**Domain Services**:
- **ServiceEnumerator**: Lists all services
- **StatusChecker**: Validates service health
- **ThreadAnalyzer**: Categorizes thread usage
- **ComponentMapper**: Maps memory to services

**Invariants**:
- Service names must be predefined types
- Active services must have threads
- Memory values in bytes
- Port numbers unique per host

### 2. Memory Analytics Context

**Core Responsibility**: Analyze SAP HANA's multi-level memory architecture

**Aggregates**:
- **MemoryAnalyzer** (Aggregate Root)
  - Tracks column store memory (main/delta)
  - Monitors row store memory (fixed/variable)
  - Measures component-specific memory
  - Calculates memory distribution

**Value Objects**:
- **ColumnStoreMemory**: Main and delta partitions
- **RowStoreMemory**: Fixed and variable parts
- **ComponentMemory**: Service-specific allocations
- **MemoryState**: Used, allocated, free
- **SchemaMemory**: Per-schema usage

**Domain Services**:
- **MemoryCalculator**: Aggregates memory types
- **SchemaAggregator**: Groups by schema
- **ComponentAnalyzer**: Breaks down by component
- **UsageTracker**: Monitors growth patterns

**Invariants**:
- Used memory ≤ allocated memory
- Column store dominates memory usage
- Delta merge affects memory patterns
- Component memory sums to total

### 3. Host Resource Context

**Core Responsibility**: Monitor physical and virtual host resources

**Aggregates**:
- **HostResourceMonitor** (Aggregate Root)
  - Tracks CPU utilization by type
  - Monitors disk usage and I/O
  - Measures swap space usage
  - Reports network statistics

**Value Objects**:
- **CpuType**: User, system, I/O wait, idle
- **DiskUsage**: Used and total bytes
- **SwapMetrics**: Swap in/out rates
- **NetworkStats**: Packet rates
- **HostName**: Physical host identifier

**Domain Services**:
- **CpuAnalyzer**: Breaks down CPU usage
- **DiskMonitor**: Tracks storage utilization
- **SwapDetector**: Identifies memory pressure
- **ResourceAggregator**: Combines metrics

**Invariants**:
- CPU percentages sum to 100%
- Disk usage ≤ disk size
- Swap usage indicates memory pressure
- I/O wait affects performance

### 4. Transaction & Lock Context

**Core Responsibility**: Monitor database concurrency and blocking

**Aggregates**:
- **TransactionMonitor** (Aggregate Root)
  - Tracks active transactions
  - Identifies blocked transactions
  - Monitors lock wait times
  - Categorizes transaction types

**Value Objects**:
- **TransactionId**: Unique transaction identifier
- **TransactionType**: User, internal, external
- **BlockingState**: Blocked or blocking
- **LockType**: Read, write, exclusive
- **WaitTime**: Lock wait duration

**Domain Services**:
- **BlockingDetector**: Finds blocking chains
- **TransactionClassifier**: Categorizes by type
- **LockAnalyzer**: Identifies contention
- **DeadlockDetector**: Finds circular waits

**Invariants**:
- Blocked transactions have blockers
- Internal transactions are system-generated
- Lock wait times are cumulative
- Deadlocks must be resolved

### 5. Schema Operations Context

**Core Responsibility**: Track schema-level data operations and performance

**Aggregates**:
- **SchemaOperationsTracker** (Aggregate Root)
  - Monitors read operations
  - Tracks write operations
  - Measures merge operations
  - Analyzes operation patterns

**Value Objects**:
- **SchemaName**: Database schema identifier
- **OperationType**: Read, write, merge, memory
- **OperationCount**: Cumulative counters
- **OperationLatency**: Time metrics
- **TableCount**: Tables per schema

**Domain Services**:
- **OperationCounter**: Tracks operations
- **MergeAnalyzer**: Monitors delta merges
- **SchemaProfiler**: Analyzes access patterns
- **PerformanceCalculator**: Computes rates

**Invariants**:
- Operation counts are monotonic
- Merges affect read performance
- Write operations create delta entries
- Schema names are unique

## Domain Events

### Service Events
- `ServiceStarted`: Service becomes active
- `ServiceStopped`: Service shutdown
- `ThreadPoolExhausted`: Thread limit reached
- `ComponentMemoryHigh`: Memory threshold exceeded

### Memory Events
- `MemoryPressureDetected`: Low free memory
- `DeltaMergeRequired`: Delta size threshold
- `UnloadCandidate`: Tables for unloading
- `AllocationFailed`: Memory request denied

### Resource Events
- `CpuSaturation`: High CPU usage
- `DiskSpaceLow`: Storage threshold
- `SwapActivityHigh`: Memory pressure
- `IoBottleneck`: Disk I/O saturation

### Transaction Events
- `LongRunningTransaction`: Duration threshold
- `BlockingChainDetected`: Multi-level blocks
- `DeadlockOccurred`: Circular dependency
- `LockTimeoutExceeded`: Wait threshold

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **Column Store** | Memory | Columnar data storage | Main memory engine |
| **Row Store** | Memory | Row-based storage | For OLTP workloads |
| **Delta Merge** | Operations | Consolidate delta to main | Periodic optimization |
| **Indexserver** | Service | Main data service | Handles queries |
| **Nameserver** | Service | System metadata | Topology management |
| **MDC** | System | Multi-tenant containers | Isolated databases |
| **Savepoint** | Operations | Consistent disk state | Periodic persistence |
| **Unload** | Memory | Move to disk | Memory optimization |

## Anti-Patterns Addressed

1. **No Heavy Queries**: Uses system monitoring views
2. **No User Data Access**: Metadata only
3. **No DDL Operations**: Read-only monitoring
4. **No Long Transactions**: Quick metric queries
5. **No Resource Competition**: Lightweight queries

## Architectural Patterns

### 1. Repository Pattern
```go
type client interface {
    collectDataFromQuery(string) ([]map[string]string, error)
}
```

### 2. Query Object Pattern
```go
type monitoringQuery struct {
    query   string
    ordinal int
    stats   []queryStat
}
```

### 3. Builder Pattern (Metrics)
```go
// Multiple builders for different resources
mbMap := make(map[resourceKey]*metadata.MetricsBuilder)
```

### 4. Factory Pattern
```go
func createClient(config *Config) (*sapHanaClient, error) {
    // Creates configured database client
}
```

## Testing Strategy

### Unit Testing
- Mock database client
- Test query result parsing
- Validate metric mappings
- Test NULL handling

### Integration Testing
- Test against SAP HANA Express
- Validate all system views
- Test authentication
- Verify TLS connections

### Performance Testing
- Query execution time
- Memory usage monitoring
- Connection pool efficiency
- Concurrent collection

## Performance Considerations

1. **System Views**: Pre-aggregated monitoring data
2. **Connection Pooling**: Reuse database connections
3. **Query Optimization**: Efficient system view access
4. **Batch Collection**: Multiple queries per scrape
5. **Resource Grouping**: Minimize metric cardinality

## Security Model

1. **Authentication**: Username/password required
2. **Authorization**: MONITORING role needed
3. **Encryption**: TLS support for connections
4. **Audit Trail**: Query execution logged
5. **Principle of Least Privilege**: Read-only access

## Evolution Points

1. **MDC Support**: Multi-tenant monitoring
2. **Scale-Out**: Distributed HANA systems
3. **Smart Data Access**: Remote source monitoring
4. **Dynamic Tiering**: Extended storage metrics
5. **Workload Classes**: Resource pool monitoring

## Error Handling Philosophy

1. **Partial Success**: Continue on query failures
2. **NULL Tolerance**: Skip incomplete rows
3. **Connection Resilience**: Reconnect on failure
4. **Clear Diagnostics**: Log query context
5. **Graceful Degradation**: Collect available metrics

## Conclusion

The SAP HANA receiver demonstrates enterprise database DDD principles:
- Rich domain model reflecting HANA's architecture
- Clear separation of memory, service, and operation concerns
- Production-ready monitoring with minimal impact
- Flexible query system for comprehensive coverage
- Security-conscious design for enterprise deployments

The architecture successfully monitors SAP HANA's complex in-memory computing platform while maintaining simplicity and reliability.