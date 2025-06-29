# MongoDB Receiver - Domain-Driven Design Analysis

## Executive Summary

The MongoDB receiver implements a comprehensive metrics collection system for MongoDB databases, organized into four primary bounded contexts:

1. **Database Metrics Collection Context**: Orchestrates metric gathering from MongoDB instances
2. **MongoDB Protocol Context**: Manages MongoDB connections and command execution
3. **Topology Discovery Context**: Handles replica set member discovery and monitoring
4. **Telemetry Transformation Context**: Converts MongoDB metrics to OpenTelemetry format

## Bounded Contexts

### 1. Database Metrics Collection Context

**Core Responsibility**: Orchestrate comprehensive metric collection across databases, collections, and server operations

**Aggregates**:
- **MongoDBScraper** (Aggregate Root)
  - Manages collection lifecycle
  - Coordinates multi-database metric gathering
  - Handles partial failure aggregation
  - Maintains client connections

**Domain Services**:
- **MetricsCollector**: Orchestrates collection across all databases
- **ErrorAggregator**: Accumulates non-fatal collection errors
- **RateCalculator**: Computes operations per second metrics

**Value Objects**:
- **DatabaseName**: Valid MongoDB database identifier
- **CollectionName**: Valid collection identifier
- **MetricPath**: Dot-notation path in MongoDB response
- **OperationType**: Enumeration of MongoDB operations

**Invariants**:
- Must collect admin metrics before database-specific metrics
- Database iteration must handle partial failures
- All databases must be attempted even if some fail
- Resource attributes must be set per database

### 2. MongoDB Protocol Context

**Core Responsibility**: Abstract MongoDB wire protocol and provide type-safe command execution

**Aggregates**:
- **MongoDBClient** (Aggregate Root)
  - Encapsulates MongoDB driver connection
  - Provides command execution interface
  - Manages connection lifecycle

**Value Objects**:
- **ServerStatus**: Server-wide performance metrics
- **DBStats**: Database-level statistics
- **TopStats**: Operation latency metrics
- **IndexStats**: Index usage statistics
- **MongoDBVersion**: Server version information

**Domain Services**:
- **CommandExecutor**: Runs MongoDB commands with timeout
- **ResponseParser**: Extracts metrics from BSON responses
- **ConnectionValidator**: Ensures connection health

**Invariants**:
- Commands must have timeout enforcement
- Connection must be validated before use
- BSON responses must handle missing fields gracefully
- Version must be detected for feature compatibility

### 3. Topology Discovery Context

**Core Responsibility**: Discover and maintain connections to all replica set members

**Aggregates**:
- **TopologyManager** (Aggregate Root)
  - Discovers secondary nodes
  - Maintains per-node connections
  - Handles topology changes

**Value Objects**:
- **ReplicaSetMember**: Node role and connection info
- **NodeRole**: PRIMARY, SECONDARY, ARBITER
- **ConnectionEndpoint**: Host and port combination

**Domain Services**:
- **SecondaryDiscoverer**: Finds replica set secondaries
- **ConnectionPool**: Manages per-node connections
- **TopologyMonitor**: Tracks membership changes

**Invariants**:
- Primary connection must exist before discovering secondaries
- Direct connection mode must skip discovery
- Each node requires separate client connection
- Arbiters must be excluded from metric collection

### 4. Telemetry Transformation Context

**Core Responsibility**: Transform MongoDB metrics into OpenTelemetry metric format

**Aggregates**:
- **MetricsBuilder** (Aggregate Root)
  - Constructs OpenTelemetry metrics
  - Manages resource attribution
  - Batches metric emission

**Value Objects**:
- **ResourceAttributes**: Server address, port, database
- **MetricAttributes**: Collection, operation, lock type, etc.
- **MetricDataPoint**: Value with timestamp and attributes

**Domain Services**:
- **AttributeMapper**: Maps MongoDB fields to OTel attributes
- **UnitConverter**: Ensures correct metric units
- **MetricNamer**: Generates consistent metric names

**Invariants**:
- Every metric must have resource attributes
- Metric names must follow OTel conventions
- Units must be correctly specified
- Attributes must use predefined keys

## Domain Events

### Collection Events
- `ScraperStarted`: Initialization complete
- `DatabaseDiscovered`: New database found
- `CollectionDiscovered`: New collection found
- `MetricsCollected`: Successful metric gathering
- `CollectionError`: Non-fatal collection failure
- `ScraperStopped`: Graceful shutdown

### Connection Events
- `ClientConnected`: MongoDB connection established
- `AuthenticationSucceeded`: Credentials accepted
- `SecondaryDiscovered`: Replica set member found
- `ConnectionLost`: Network or server failure
- `ReconnectAttempted`: Recovery in progress

### Command Events
- `CommandExecuted`: MongoDB command completed
- `CommandTimeout`: Exceeded time limit
- `ResponseParsed`: Metrics extracted from response
- `FieldMissing`: Expected metric not found

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **Scraper** | Collection | Component that periodically gathers metrics | Must respect configured interval |
| **Database** | MongoDB | Logical grouping of collections | Name must be valid identifier |
| **Collection** | MongoDB | Document storage unit | Must belong to a database |
| **Operation** | MongoDB | Read/write action type | Must be known operation type |
| **Lock Mode** | MongoDB | Concurrency control type | Must be R, W, r, or w |
| **WiredTiger** | MongoDB | Default storage engine | Metrics available in 3.0+ |
| **Replica Set** | Topology | Group of MongoDB nodes | Must have primary |
| **Direct Connection** | Topology | Single node connection | Skips topology discovery |
| **Resource Attribute** | Telemetry | Metric source identifier | Must include server address |

## Anti-Patterns Addressed

1. **No Blocking Operations**: All commands use context timeout
2. **No Silent Failures**: Partial errors are accumulated and reported
3. **No Hardcoded Metrics**: All metrics defined in metadata.yaml
4. **No Connection Leaks**: Proper lifecycle management
5. **No Version Assumptions**: Runtime version detection

## Architectural Patterns

### 1. Repository Pattern
```go
type client interface {
    ServerStatus(ctx context.Context, dbName string) (bson.M, error)
    DBStats(ctx context.Context, dbName string) (bson.M, error)
    // ... other database operations
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

### 3. Scraper Pattern
```go
type mongodbScraper struct {
    client       client
    config       *Config
    mb           *metadata.MetricsBuilder
}
```

### 4. Builder Pattern
```go
// MetricsBuilder constructs metrics with proper attribution
mb.RecordMongodbCollectionCountDataPoint(now, collectionCount,
    serverName, portName, databaseName)
```

## Aggregate Invariants Detail

### MongoDBScraper Aggregate
1. **Collection Order Invariants**
   - Admin database metrics collected first
   - User databases collected alphabetically
   - Each database fully processed before next

2. **Connection Invariants**
   - Primary client must be established in start()
   - Secondary clients created after primary
   - All clients closed in shutdown()

3. **Error Handling Invariants**
   - Non-fatal errors must not stop collection
   - All errors must be aggregated
   - Fatal errors only for complete failure

### MongoDBClient Aggregate
1. **Command Execution Invariants**
   - All commands must use context timeout
   - Response must be BSON document
   - Nil responses must return error
   - Connection must be validated

2. **Connection Management Invariants**
   - Options must be immutable after creation
   - Authentication must be set if provided
   - TLS must be configured if enabled
   - Direct connection must disable discovery

## Testing Strategy

### Unit Testing
- Mock client for scraper tests
- Test metric recording logic
- Validate configuration options
- Test error aggregation

### Integration Testing
- Test against real MongoDB instances
- Validate all metric paths
- Test authentication scenarios
- Verify TLS connections

### Topology Testing
- Test replica set discovery
- Validate secondary connections
- Test direct connection mode
- Handle topology changes

## Performance Considerations

1. **Connection Pooling**: MongoDB driver handles pooling
2. **Parallel Collection**: Could parallelize database iteration
3. **Command Timeout**: 1-minute default prevents hanging
4. **Metric Batching**: All metrics emitted together
5. **Secondary Load**: Distributed reads across replica set

## Security Model

1. **Authentication**: Username/password via SCRAM
2. **Authorization**: Requires read access to admin database
3. **Encryption**: Full TLS/SSL support
4. **Certificate Validation**: Enabled by default
5. **Credential Protection**: No logging of passwords

## Evolution Points

1. **New Metrics**: Add to metadata.yaml
2. **MongoDB Versions**: Extend version detection
3. **Authentication**: Add LDAP, x509 support
4. **Sharding**: Add shard-specific metrics
5. **Change Streams**: Real-time metric updates

## Error Handling Philosophy

1. **Graceful Degradation**: Collect what's available
2. **Detailed Diagnostics**: Include database/collection in errors
3. **Partial Success**: Report collected metrics even with errors
4. **Clear Attribution**: Know which component failed
5. **Recovery Capable**: Transient failures don't stop collector

## Conclusion

The MongoDB receiver exemplifies mature DDD principles:
- Clear bounded contexts separating concerns
- Rich domain model with MongoDB-specific concepts
- Robust error handling supporting partial failures
- Clean abstractions over MongoDB protocol
- Extensible architecture for new metrics and features

The design successfully bridges MongoDB's document-oriented world with OpenTelemetry's metric model while maintaining operational reliability and security.