# SQL Query Receiver - Domain-Driven Design Analysis

## Executive Summary

The SQL Query receiver implements a generic database monitoring system organized into four primary bounded contexts:

1. **Query Execution Context**: Manages custom SQL query execution across database types
2. **Data Transformation Context**: Converts query results to metrics or logs
3. **Database Abstraction Context**: Provides unified interface for multiple SQL databases
4. **State Management Context**: Handles incremental data collection with tracking

## Bounded Contexts

### 1. Query Execution Context

**Core Responsibility**: Execute user-defined SQL queries safely and efficiently across different databases

**Aggregates**:
- **QueryExecutor** (Aggregate Root)
  - Manages query lifecycle
  - Handles periodic execution
  - Coordinates multiple queries
  - Enforces execution intervals

**Value Objects**:
- **SQLQuery**: User-defined query text
- **CollectionInterval**: Execution frequency
- **QueryResult**: Raw result rows
- **ExecutionContext**: Query runtime metadata

**Domain Services**:
- **QueryScheduler**: Manages periodic execution
- **QueryValidator**: Ensures query safety
- **ResultFetcher**: Retrieves query results
- **ErrorHandler**: Manages query failures

**Invariants**:
- Queries must be SELECT statements (read-only)
- Collection interval must be positive
- Query execution must respect timeouts
- Failed queries must not stop other queries

### 2. Data Transformation Context

**Core Responsibility**: Transform SQL result sets into OpenTelemetry metrics or logs

**Aggregates**:
- **MetricTransformer** (Aggregate Root)
  - Maps columns to metric values
  - Extracts metric attributes
  - Manages aggregation temporality
  - Handles data type conversions

- **LogTransformer** (Aggregate Root)
  - Maps rows to log records
  - Extracts log attributes
  - Manages tracking state
  - Handles incremental collection

**Value Objects**:
- **ColumnMapping**: Column to metric/attribute mapping
- **MetricType**: Gauge or Sum
- **AggregationTemporality**: Delta or Cumulative
- **TrackingValue**: Last processed row identifier
- **LogBody**: Transformed log content

**Domain Services**:
- **TypeConverter**: SQL types to OTel types
- **AttributeExtractor**: Column to attribute mapping
- **ValueExtractor**: Column to metric value
- **TrackingManager**: Incremental processing state

**Invariants**:
- Value column must exist for metrics
- Numeric columns required for metric values
- Tracking column must be monotonic for logs
- Attribute columns must have valid names

### 3. Database Abstraction Context

**Core Responsibility**: Provide unified interface for multiple SQL database types

**Aggregates**:
- **DatabaseClient** (Aggregate Root)
  - Abstracts database differences
  - Manages connection pooling
  - Handles driver specifics
  - Provides query execution

**Value Objects**:
- **ConnectionString**: Database-specific DSN
- **DriverName**: Database type identifier
- **PoolConfig**: Connection pool settings
- **QueryTimeout**: Execution time limit

**Domain Services**:
- **DriverRegistry**: Maps database types to drivers
- **ConnectionPoolManager**: Manages connection lifecycle
- **DialectAdapter**: Handles SQL dialect differences
- **TypeMapper**: Database-specific type handling

**Invariants**:
- Connection string must be valid for driver
- Pool size must be positive
- Driver must be registered and imported
- Connections must be returned to pool

### 4. State Management Context

**Core Responsibility**: Enable incremental data collection through persistent state tracking

**Aggregates**:
- **TrackingStateManager** (Aggregate Root)
  - Persists last processed values
  - Enables exactly-once semantics
  - Manages state per query
  - Handles state recovery

**Value Objects**:
- **TrackingColumn**: Column used for incremental tracking
- **StorageKey**: Unique query identifier
- **CheckpointValue**: Last processed value
- **StateSnapshot**: Complete tracking state

**Domain Services**:
- **StorageClient**: Persists state externally
- **StateSerializer**: Converts state to storage format
- **RecoveryHandler**: Restores state on startup
- **CheckpointManager**: Updates tracking values

**Invariants**:
- Tracking values must increase monotonically
- State must be persisted before acknowledgment
- Recovery must handle missing state
- Concurrent updates must be serialized

## Domain Events

### Query Events
- `QueryScheduled`: Execution timer triggered
- `QueryExecuted`: SQL completed successfully
- `QueryFailed`: Execution error occurred
- `ResultsRetrieved`: Data ready for transformation

### Transformation Events
- `MetricCreated`: Row converted to metric
- `LogCreated`: Row converted to log
- `AttributeExtracted`: Column mapped to attribute
- `TrackingUpdated`: Checkpoint advanced

### Connection Events
- `ConnectionAcquired`: Pool connection obtained
- `ConnectionReleased`: Connection returned
- `PoolExhausted`: No connections available
- `ConnectionFailed`: Database unreachable

### State Events
- `StateLoaded`: Tracking state recovered
- `CheckpointSaved`: Progress persisted
- `StateCorrupted`: Invalid state detected
- `StateMissing`: No previous state found

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **Query** | Execution | User-defined SQL SELECT | Read-only operation |
| **Scraper** | Execution | Periodic query executor | One per query |
| **StringMap** | Transformation | Row as string key-value | All values as strings |
| **Tracking Column** | State | Incremental processing key | Monotonic values |
| **Value Column** | Transformation | Source for metric value | Must be numeric |
| **Attribute Column** | Transformation | Source for attributes | String convertible |
| **Storage Client** | State | External state persistence | Must be configured |
| **Driver** | Abstraction | Database-specific adapter | Must be imported |

## Anti-Patterns Addressed

1. **No DDL/DML**: Only SELECT queries allowed
2. **No Connection Leaks**: Proper pool management
3. **No Data Loss**: Tracking for exactly-once
4. **No Type Assumptions**: String-based transformation
5. **No Driver Lock-in**: Abstracted database access

## Architectural Patterns

### 1. Abstract Factory Pattern
```go
type DbClientFactory func(
    driver string,
    source string,
    opts ClientOpts,
    tls configtls.ClientConfig,
) (dbClient, error)
```

### 2. Strategy Pattern (Metric Types)
```go
type MetricCfg struct {
    MetricName       string
    ValueColumn      string
    DataType         configtelemetry.Type
    Aggregation      configtelemetry.AggregationTemporality
    AttributeColumns []string
}
```

### 3. Adapter Pattern (Database Wrappers)
```go
type dbWrapper interface {
    query(ctx context.Context, args ...any) (rowsWrapper, error)
    close() error
}
```

### 4. Repository Pattern (State Storage)
```go
type Client interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte) error
}
```

## Testing Strategy

### Unit Testing
- Mock database client
- Test query result transformation
- Validate configuration parsing
- Test state management

### Integration Testing
- Test against real databases
- Validate driver compatibility
- Test connection pooling
- Verify tracking persistence

### Multi-Database Testing
- PostgreSQL specific types
- MySQL compatibility
- Oracle dialect differences
- SQL Server variations

## Performance Considerations

1. **Connection Pooling**: Reuse database connections
2. **Batch Processing**: Multiple metrics per query
3. **Incremental Collection**: Process only new data
4. **Parallel Queries**: Concurrent execution
5. **Result Streaming**: Handle large result sets

## Security Model

1. **Read-Only Queries**: SELECT statements only
2. **Connection Security**: TLS/SSL support
3. **Credential Management**: Secure configuration
4. **Query Injection**: Parameterized queries
5. **Audit Trail**: Query logging options

## Evolution Points

1. **Prepared Statements**: Query caching
2. **Dynamic Queries**: Template support
3. **Join Support**: Multi-table queries
4. **Aggregation Push-down**: Database-side rollups
5. **Change Data Capture**: Real-time updates

## Error Handling Philosophy

1. **Query Isolation**: Failures don't cascade
2. **Partial Success**: Collect what's available
3. **Clear Diagnostics**: Include query context
4. **Retry Logic**: Transient failure handling
5. **Graceful Degradation**: Continue operation

## Conclusion

The SQL Query receiver demonstrates flexible DDD principles:
- Generic design supporting any SQL database
- Clean separation between query execution and data transformation
- Robust state management for reliability
- Extensible architecture for new databases
- Production-ready error handling

The architecture successfully enables custom monitoring scenarios while maintaining database independence and operational reliability.