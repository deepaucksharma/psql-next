# Oracle DB Receiver - Domain-Driven Design Analysis

## Executive Summary

The Oracle DB receiver implements an enterprise database monitoring system organized into four primary bounded contexts:

1. **Performance Metrics Context**: Collects system-wide performance indicators from V$ views
2. **Oracle Protocol Context**: Manages database connections and SQL execution
3. **Resource Management Context**: Monitors database resource usage and limits
4. **Storage Management Context**: Tracks tablespace utilization and capacity

## Bounded Contexts

### 1. Performance Metrics Context

**Core Responsibility**: Collect and transform Oracle performance statistics from dynamic performance views

**Aggregates**:
- **OracleScraper** (Aggregate Root)
  - Orchestrates metric collection from multiple sources
  - Manages collection timing and cycles
  - Aggregates errors from partial failures
  - Coordinates multiple specialized clients

**Value Objects**:
- **SysStatMetric**: System statistic from V$SYSSTAT
- **MetricRow**: Key-value pairs from query results
- **MetricTimestamp**: Collection time for metrics
- **StatisticName**: Oracle statistic identifier

**Domain Services**:
- **MetricMapper**: Maps Oracle statistics to OTel metrics
- **ValueConverter**: Handles Oracle numeric types
- **ErrorAggregator**: Collects non-fatal errors

**Invariants**:
- All V$SYSSTAT values must be numeric
- Collection timestamp must be consistent across metrics
- Known statistics must map to defined metrics
- Resource attributes must include instance name

### 2. Oracle Protocol Context

**Core Responsibility**: Abstract Oracle database protocol and provide reliable query execution

**Aggregates**:
- **DBClient** (Aggregate Root)
  - Encapsulates database connection
  - Executes monitoring queries
  - Handles result set transformation
  - Manages connection lifecycle

**Value Objects**:
- **DataSource**: Oracle connection string
- **ServiceName**: Oracle service identifier
- **InstanceName**: Database instance name
- **QueryResult**: Typed row data

**Domain Services**:
- **ConnectionBuilder**: Constructs Oracle URLs
- **QueryExecutor**: Runs SQL with proper context
- **ResultMapper**: Converts rows to domain objects
- **TypeConverter**: Handles Oracle-specific types

**Invariants**:
- Connection must be validated before queries
- All queries must be read-only
- NULL values must be handled gracefully
- Instance name must be extracted from connection

### 3. Resource Management Context

**Core Responsibility**: Monitor Oracle resource utilization against configured limits

**Aggregates**:
- **ResourceMonitor** (Aggregate Root)
  - Tracks resource usage and limits
  - Monitors sessions by status/type
  - Reports resource utilization percentages

**Value Objects**:
- **ResourceLimit**: Current and max values
- **SessionInfo**: Status and type attributes
- **ResourceType**: Processes, sessions, transactions
- **UtilizationRatio**: Usage percentage

**Domain Services**:
- **SessionAnalyzer**: Groups sessions by attributes
- **LimitCalculator**: Computes usage percentages
- **ResourceTracker**: Monitors limit changes

**Invariants**:
- Current usage cannot exceed limit
- Limit value -1 means unlimited
- Session counts must be non-negative
- Resource names must be predefined

### 4. Storage Management Context

**Core Responsibility**: Monitor tablespace usage and capacity planning

**Aggregates**:
- **TablespaceMonitor** (Aggregate Root)
  - Tracks space usage per tablespace
  - Monitors both used and allocated space
  - Reports by tablespace type

**Value Objects**:
- **TablespaceName**: Unique tablespace identifier
- **SpaceMetrics**: Used bytes and allocated bytes
- **TablespaceType**: PERMANENT, TEMPORARY, UNDO
- **ContentType**: Data storage classification

**Domain Services**:
- **SpaceCalculator**: Computes usage metrics
- **TablespaceAnalyzer**: Categorizes tablespaces
- **UsageTrendTracker**: Monitors growth patterns

**Invariants**:
- Used space cannot exceed allocated space
- Tablespace names must be unique
- All tablespaces must have a type
- Metrics must be in bytes

## Domain Events

### Collection Events
- `ScraperStarted`: Oracle connection established
- `MetricsCollected`: Successful collection cycle
- `PartialFailure`: Some queries failed
- `ScraperStopped`: Clean shutdown

### Connection Events
- `ConnectionEstablished`: Oracle connected
- `InstanceDetected`: Instance name extracted
- `ConnectionLost`: Network or database failure
- `AuthenticationFailed`: Invalid credentials

### Query Events
- `QueryExecuted`: SQL completed successfully
- `ResultSetProcessed`: Rows converted to metrics
- `NullValueSkipped`: NULL metric ignored
- `TypeConversionError`: Oracle type handling failed

### Resource Events
- `ResourceLimitReached`: Usage at maximum
- `SessionCreated`: New session detected
- `SessionTerminated`: Session ended
- `ResourceExhausted`: No capacity remaining

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **V$ View** | Oracle | Dynamic performance view | Read-only, real-time data |
| **SysSTat** | Performance | System-wide statistic | Cumulative counter |
| **Instance** | Oracle | Database instance | Unique within server |
| **Service** | Oracle | Database service name | Connection endpoint |
| **Session** | Resource | Database connection | Has status and type |
| **Tablespace** | Storage | Logical storage unit | Contains segments |
| **PGA** | Memory | Program Global Area | Per-session memory |
| **Parse** | Performance | SQL statement analysis | Hard or soft parse |
| **Resource Limit** | Resource | Configured maximum | -1 means unlimited |

## Anti-Patterns Addressed

1. **No Privileged Access**: Only SELECT on V$ views required
2. **No Blocking Queries**: Context-based timeouts
3. **No Silent Failures**: Comprehensive error reporting
4. **No Type Assumptions**: Explicit Oracle type handling
5. **No Connection Leaks**: Proper lifecycle management

## Architectural Patterns

### 1. Repository Pattern
```go
type dbClient interface {
    metricRows(ctx context.Context) ([]metricRow, error)
}
```

### 2. Factory Pattern
```go
func NewFactory() receiver.Factory {
    return receiver.NewFactory(
        metadata.Type,
        createDefaultConfig,
        receiver.WithMetrics(f.createMetricsReceiver, stability),
    )
}
```

### 3. Strategy Pattern (Multiple Clients)
```go
type oracleScraper struct {
    statsClient              dbClient
    sessionCountClient       dbClient
    systemResourceLimits     dbClient
    tablespaceUsageClient    dbClient
}
```

### 4. Adapter Pattern (Type Conversion)
```go
// Handles Oracle-specific types like []uint8
switch v := value.(type) {
case []uint8:
    return string(v)
// ... other type conversions
}
```

## Aggregate Invariants Detail

### OracleScraper Aggregate
1. **Collection Invariants**
   - All clients must be initialized
   - Queries execute independently
   - Timestamp consistency across metrics
   - Instance name required for resources

2. **Error Handling Invariants**
   - Individual query failures don't stop collection
   - All errors must be aggregated
   - Partial results are acceptable
   - Zero metrics is valid outcome

### DBClient Aggregate
1. **Connection Invariants**
   - Single connection per client
   - Connection string immutable
   - Graceful shutdown required
   - No automatic reconnection

2. **Query Invariants**
   - Read-only access enforced
   - Result columns dynamic
   - NULL handling consistent
   - Context cancellation respected

## Testing Strategy

### Unit Testing
- Mock dbClient for scraper tests
- Test metric mappings
- Validate type conversions
- Test error aggregation

### Integration Testing
- Test against real Oracle instances
- Validate all V$ view queries
- Test authentication methods
- Verify instance name extraction

### Performance Testing
- Query execution time
- Memory usage with large results
- Connection pool efficiency
- Concurrent query handling

## Performance Considerations

1. **Query Optimization**: Efficient V$ view access
2. **Connection Pooling**: Reuse database connections
3. **Selective Metrics**: Configurable metric collection
4. **Batch Processing**: Multiple queries per cycle
5. **Resource Impact**: Minimal load on Oracle

## Security Model

1. **Authentication**: Username/password required
2. **Authorization**: SELECT on V$ views only
3. **No DBA Access**: Regular user sufficient
4. **Connection Security**: TNS encryption support
5. **Credential Protection**: No password logging

## Evolution Points

1. **New Metrics**: Add V$ view queries
2. **RAC Support**: Real Application Clusters
3. **ASM Metrics**: Automatic Storage Management
4. **Wait Events**: Detailed performance analysis
5. **Historical Data**: AWR integration

## Error Handling Philosophy

1. **Resilient Collection**: Continue despite failures
2. **Detailed Context**: Include query identification
3. **Partial Success**: Emit available metrics
4. **Clear Diagnostics**: Actionable error messages
5. **Graceful Degradation**: Reduce functionality safely

## Conclusion

The Oracle DB receiver demonstrates enterprise-grade DDD principles:
- Clear separation between Oracle domain and metrics
- Robust handling of Oracle-specific data types
- Multiple specialized clients for different metric domains
- Production-ready error handling and partial failures
- Security-conscious design with minimal privileges

The architecture successfully monitors Oracle database performance while maintaining reliability, security, and operational simplicity in enterprise environments.