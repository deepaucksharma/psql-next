# MySQL Receiver - Domain-Driven Design Analysis

## Executive Summary

The MySQL receiver implements a comprehensive database monitoring system organized into four primary bounded contexts:

1. **Performance Metrics Context**: Collects and transforms MySQL performance indicators
2. **MySQL Protocol Context**: Manages database connections and query execution
3. **Storage Engine Context**: Monitors InnoDB-specific metrics and operations
4. **Replication Monitoring Context**: Tracks MySQL replication topology and health

## Bounded Contexts

### 1. Performance Metrics Context

**Core Responsibility**: Orchestrate collection of MySQL performance metrics across multiple subsystems

**Aggregates**:
- **MySQLScraper** (Aggregate Root)
  - Coordinates metric collection cycles
  - Handles partial failure scenarios
  - Maps MySQL variables to metrics
  - Manages collection timing

**Value Objects**:
- **MetricMapping**: Maps MySQL variable to OTel metric
- **CollectionInterval**: Scraping frequency
- **MetricType**: Counter, gauge, or histogram
- **CommandType**: MySQL command categories

**Domain Services**:
- **MetricsMapper**: Transforms MySQL values to metrics
- **ErrorAggregator**: Collects non-fatal errors
- **UnitConverter**: Ensures proper metric units

**Invariants**:
- All mapped variables must produce valid metrics
- Collection must continue despite partial failures
- Metric names must follow OTel conventions
- Resource attributes must be set for all metrics

### 2. MySQL Protocol Context

**Core Responsibility**: Abstract MySQL wire protocol and provide reliable query execution

**Aggregates**:
- **MySQLClient** (Aggregate Root)
  - Manages database connection lifecycle
  - Executes monitoring queries
  - Handles version detection
  - Provides typed query results

**Value Objects**:
- **DSN**: Data Source Name for connection
- **GlobalStatus**: Key-value status variables
- **ServerVersion**: MySQL/MariaDB version info
- **QueryResult**: Typed query response

**Domain Services**:
- **ConnectionBuilder**: Constructs DSN from config
- **QueryExecutor**: Runs SQL with timeout
- **VersionDetector**: Determines server variant
- **ResultParser**: Converts rows to domain objects

**Invariants**:
- Connection must be validated before queries
- Version must be detected for compatibility
- Queries must handle NULL values
- Connection must support configured transport

### 3. Storage Engine Context (InnoDB)

**Core Responsibility**: Monitor InnoDB storage engine internals and performance

**Aggregates**:
- **InnoDBMonitor** (Aggregate Root)
  - Tracks buffer pool performance
  - Monitors page operations
  - Measures log I/O
  - Reports row operations

**Value Objects**:
- **BufferPoolStats**: Memory usage and efficiency
- **PageOperation**: Read, write, create, etc.
- **LogMetrics**: Redo log write statistics
- **RowOperation**: Insert, update, delete counts

**Domain Services**:
- **BufferPoolAnalyzer**: Calculates efficiency metrics
- **PageOperationTracker**: Aggregates page activities
- **LogMonitor**: Tracks redo log performance

**Invariants**:
- Buffer pool metrics require InnoDB engine
- Page operations must be non-negative
- Log sequence numbers must increase
- Row operations must be cumulative

### 4. Replication Monitoring Context

**Core Responsibility**: Track MySQL replication topology, lag, and health

**Aggregates**:
- **ReplicationMonitor** (Aggregate Root)
  - Detects replication role
  - Measures replication lag
  - Tracks I/O and SQL threads
  - Handles version differences

**Value Objects**:
- **ReplicaStatus**: Complete replication state
- **ReplicationLag**: Seconds behind source
- **ThreadStatus**: I/O and SQL thread states
- **SourceInfo**: Master server details

**Domain Services**:
- **LagCalculator**: Computes replication delay
- **ThreadMonitor**: Tracks thread health
- **CompatibilityHandler**: Manages MySQL/MariaDB differences

**Invariants**:
- Replica status only valid on replicas
- Thread status must be "Yes" or "No"
- Lag must be non-negative or NULL
- Source info must include host and port

## Domain Events

### Collection Events
- `ScraperStarted`: MySQL connection established
- `MetricsCollected`: Successful collection cycle
- `CollectionError`: Non-fatal collection failure
- `ScraperStopped`: Clean shutdown

### Connection Events
- `ConnectionEstablished`: MySQL connected
- `VersionDetected`: Server variant identified
- `ConnectionLost`: Network or server failure
- `AuthenticationFailed`: Invalid credentials

### Query Events
- `QueryExecuted`: SQL completed successfully
- `QueryTimeout`: Exceeded time limit
- `ResultParsed`: Data extracted from rows
- `NullValueHandled`: Missing metric skipped

### Replication Events
- `ReplicaDetected`: Server is replica
- `LagMeasured`: Replication delay calculated
- `ThreadStopped`: Replication thread failed
- `SourceChanged`: Master server switched

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **Global Status** | MySQL | Server-wide variables | Must be numeric or convertible |
| **InnoDB** | Storage Engine | Default MySQL storage engine | Required for engine metrics |
| **Buffer Pool** | InnoDB | In-memory data cache | Size must be positive |
| **Replica** | Replication | Server receiving replicated data | Must have source configured |
| **Binlog** | Replication | Binary log for replication | Position must increase |
| **Performance Schema** | MySQL | Detailed performance data | Must be enabled for some metrics |
| **Information Schema** | MySQL | Database metadata | Always available |
| **DSN** | Connection | Connection string format | Must include transport and host |
| **Statement Event** | Performance | Query execution record | Requires performance schema |

## Anti-Patterns Addressed

1. **No Blocking Queries**: All queries use context timeout
2. **No Missing Metrics**: Graceful handling of NULLs
3. **No Version Assumptions**: Runtime compatibility detection
4. **No Connection Leaks**: Proper lifecycle management
5. **No Silent Failures**: All errors logged and counted

## Architectural Patterns

### 1. Repository Pattern
```go
type client interface {
    getGlobalStats() (map[string]sql.NullInt64, error)
    getInnodbStats() (map[string]sql.NullInt64, error)
    // ... other query methods
}
```

### 2. Factory Pattern
```go
func NewFactory() receiver.Factory {
    return receiver.NewFactory(
        metadata.Type,
        createDefaultConfig,
        receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
    )
}
```

### 3. Strategy Pattern (Version Handling)
```go
// Different queries for MySQL vs MariaDB
if version.flavor == mariadb {
    query = replicaStatusQueryMariaDB
} else {
    query = replicaStatusQuery
}
```

### 4. Builder Pattern (DSN Construction)
```go
func buildDSN(cfg *Config) string {
    // Constructs DSN from configuration components
}
```

## Aggregate Invariants Detail

### MySQLScraper Aggregate
1. **Collection Invariants**
   - Must attempt all query types
   - Errors in one query don't stop others
   - Metrics must have timestamps
   - Resource attributes set once

2. **Mapping Invariants**
   - Known variables produce specific metrics
   - Unknown variables are ignored
   - Counters must be cumulative
   - Gauges represent current state

### MySQLClient Aggregate
1. **Connection Invariants**
   - Single connection per scraper
   - Connection validated on start
   - Graceful shutdown required
   - Reconnection not automatic

2. **Query Invariants**
   - All queries read-only
   - Results must be deterministic
   - NULL handling consistent
   - Timeout enforcement required

## Testing Strategy

### Unit Testing
- Mock client for scraper tests
- Test metric mappings
- Validate DSN construction
- Test error aggregation

### Integration Testing
- Test against real MySQL/MariaDB
- Validate all query paths
- Test authentication methods
- Verify TLS connections

### Compatibility Testing
- Test multiple MySQL versions
- Validate MariaDB differences
- Test with/without performance schema
- Handle missing privileges

## Performance Considerations

1. **Query Efficiency**: Optimized monitoring queries
2. **Connection Pooling**: Single persistent connection
3. **Batch Collection**: All metrics in one cycle
4. **Selective Monitoring**: Configurable metric sets
5. **Resource Usage**: Minimal overhead on MySQL

## Security Model

1. **Authentication**: Username/password required
2. **Authorization**: SELECT privileges sufficient
3. **Encryption**: TLS/SSL support
4. **Password Security**: Native password plugin support
5. **Principle of Least Privilege**: Read-only queries

## Evolution Points

1. **New Metrics**: Add to metadata.yaml
2. **Query Optimization**: Enhance for large deployments
3. **Multi-Source Replication**: Extended topology support
4. **Custom Metrics**: User-defined queries
5. **Streaming Replication**: Real-time lag monitoring

## Error Handling Philosophy

1. **Partial Success**: Collect available metrics
2. **Clear Attribution**: Identify failing queries
3. **Graceful Degradation**: Continue despite errors
4. **Diagnostic Context**: Include query in errors
5. **Recovery**: No automatic reconnection

## Conclusion

The MySQL receiver exemplifies mature DDD principles:
- Clear separation of MySQL protocol from metrics domain
- Rich modeling of MySQL-specific concepts
- Robust error handling for production deployments
- Version-aware implementation for compatibility
- Security-conscious design with minimal privileges

The architecture successfully monitors MySQL performance while maintaining simplicity, reliability, and operational safety.