# SQL Server Receiver - Domain-Driven Design Analysis

## Executive Summary

The SQL Server receiver implements a comprehensive database monitoring system with platform-aware collection strategies, organized into five primary bounded contexts:

1. **Performance Counter Context**: Collects metrics via Windows Performance Counters or SQL DMVs
2. **Database I/O Context**: Monitors file-level I/O operations and latencies
3. **Query Intelligence Context**: Analyzes query performance and execution patterns
4. **Wait Statistics Context**: Tracks SQL Server wait events and bottlenecks
5. **Platform Abstraction Context**: Manages Windows vs Linux collection differences

## Bounded Contexts

### 1. Performance Counter Context

**Core Responsibility**: Collect SQL Server performance metrics through platform-appropriate mechanisms

**Aggregates**:
- **PerformanceCounterCollector** (Aggregate Root)
  - Windows: Direct Win32 API access
  - Linux: SQL DMV queries
  - Maps counters to metrics
  - Handles counter hierarchies

**Value Objects**:
- **CounterPath**: Object\Counter\Instance format
- **CounterValue**: Raw counter reading
- **CounterType**: Rate, ratio, or absolute
- **InstanceName**: SQL Server instance identifier

**Domain Services**:
- **CounterRegistry**: Maps counter names to metrics
- **DeltaCalculator**: Computes rate metrics
- **CounterWatcher**: Windows-specific monitoring
- **DMVQuerier**: SQL-based counter retrieval

**Invariants**:
- Counter paths must follow Windows format
- Instance names must be valid
- Rate counters require previous values
- Counter values must be numeric

### 2. Database I/O Context

**Core Responsibility**: Monitor file-level I/O performance for all databases

**Aggregates**:
- **DatabaseIOMonitor** (Aggregate Root)
  - Tracks read/write operations
  - Measures I/O latencies
  - Monitors stall times
  - Aggregates by database and file

**Value Objects**:
- **FileID**: Database file identifier
- **IOOperation**: Read or write type
- **StallTime**: I/O wait duration
- **BytesTransferred**: I/O volume
- **FileType**: Data or log file

**Domain Services**:
- **IOStatsCollector**: Queries sys.dm_io_virtual_file_stats
- **LatencyCalculator**: Computes average latencies
- **FileMapper**: Associates files with databases

**Invariants**:
- Stall time ≤ elapsed time
- File IDs must exist in database
- Bytes must be non-negative
- Latency calculations require non-zero operations

### 3. Query Intelligence Context

**Core Responsibility**: Analyze query performance patterns and resource consumption

**Aggregates**:
- **QueryAnalyzer** (Aggregate Root)
  - Collects top queries by cost
  - Samples active queries
  - Obfuscates sensitive data
  - Caches query plans

**Value Objects**:
- **QueryHash**: Unique query identifier
- **ExecutionStats**: CPU, reads, duration
- **QueryText**: Obfuscated SQL text
- **ExecutionPlan**: XML query plan
- **BlockingInfo**: Blocking chain details

**Domain Services**:
- **QueryObfuscator**: Removes sensitive data
- **PlanCache**: Stores execution plans
- **TopQuerySelector**: Ranks queries by cost
- **ActiveQuerySampler**: Captures running queries

**Invariants**:
- Query text must be obfuscated
- Execution stats must be cumulative
- Plans cached by query hash
- Blocking chains must be acyclic

### 4. Wait Statistics Context

**Core Responsibility**: Track SQL Server wait events to identify performance bottlenecks

**Aggregates**:
- **WaitStatsMonitor** (Aggregate Root)
  - Tracks wait types and durations
  - Filters system wait types
  - Calculates wait percentages
  - Supports Azure SQL DB

**Value Objects**:
- **WaitType**: Categorized wait reason
- **WaitTime**: Total wait duration
- **SignalWaitTime**: CPU queue time
- **WaitingTasks**: Count of waiting tasks

**Domain Services**:
- **WaitTypeFilter**: Excludes benign waits
- **WaitCategorizer**: Groups related waits
- **DeltaTracker**: Computes wait deltas

**Invariants**:
- Signal wait ≤ total wait time
- Wait counts must be non-negative
- System waits must be filtered
- Azure SQL DB uses different DMV

### 5. Platform Abstraction Context

**Core Responsibility**: Abstract platform differences between Windows and Linux SQL Server

**Aggregates**:
- **PlatformCollector** (Aggregate Root)
  - Selects appropriate collection method
  - Manages platform-specific resources
  - Handles connection differences

**Value Objects**:
- **Platform**: Windows or Linux/Others
- **CollectionMethod**: PerformanceCounters or SQL
- **ComputerName**: Windows machine name
- **ConnectionString**: Database connection

**Domain Services**:
- **PlatformDetector**: Determines runtime OS
- **CollectorFactory**: Creates platform scrapers
- **NameResolver**: Handles instance naming

**Invariants**:
- Windows requires computer name for named instances
- Linux requires database connection
- Collection method must match platform
- Instance names follow platform conventions

## Domain Events

### Performance Events
- `CounterCollected`: Performance counter read
- `DeltaCalculated`: Rate metric computed
- `CounterMissing`: Expected counter not found
- `CollectionCycleComplete`: All counters gathered

### I/O Events
- `IOStallDetected`: High I/O wait time
- `FileGrowth`: Database file expanded
- `IOPatternChanged`: Read/write ratio shift
- `LatencyThresholdExceeded`: Slow I/O detected

### Query Events
- `TopQueryIdentified`: High-cost query found
- `QueryBlocked`: Blocking chain detected
- `PlanCached`: Execution plan stored
- `QueryObfuscated`: Sensitive data removed

### Wait Events
- `WaitTypeIncreased`: Significant wait growth
- `NewWaitTypeDetected`: Previously unseen wait
- `CPUPressureDetected`: High signal waits
- `WaitStatsReset`: Statistics cleared

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **DMV** | SQL Server | Dynamic Management View | Read-only system view |
| **Performance Counter** | Windows | System performance metric | Object\Counter\Instance |
| **Wait Type** | Wait Stats | Reason for waiting | Categorized by resource |
| **Query Hash** | Query Intel | Query fingerprint | Consistent across executions |
| **Stall Time** | I/O | I/O wait duration | Per file metric |
| **Instance** | Platform | SQL Server installation | Named or default |
| **Signal Wait** | Wait Stats | CPU scheduling delay | Component of total wait |
| **Batch Request** | Performance | T-SQL batch execution | Rate counter |

## Anti-Patterns Addressed

1. **No Platform Lock-in**: Supports both Windows and Linux
2. **No Sensitive Data**: Query obfuscation applied
3. **No Performance Impact**: Lightweight DMV queries
4. **No Connection Pooling Issues**: Proper lifecycle management
5. **No Missing Instances**: Handles named instances correctly

## Architectural Patterns

### 1. Abstract Factory Pattern
```go
// Platform-specific factories
func createMetricsReceiver(ctx context.Context, ...) (receiver.Metrics, error) {
    if runtime.GOOS == "windows" {
        return newWindowsReceiver(...)
    }
    return newSQLReceiver(...)
}
```

### 2. Strategy Pattern (Collection Methods)
```go
type scraper interface {
    scrape(context.Context) (pmetric.Metrics, error)
}

type sqlServerPCScraper struct{} // Windows
type sqlServerScraperHelper struct{} // SQL-based
```

### 3. Template Method Pattern
```go
// Query templates with instance filtering
const sqlServerDatabaseIOQuery = `
SELECT ... FROM sys.dm_io_virtual_file_stats(NULL, NULL)
{{if .instanceName}} WHERE ... {{end}}
`
```

### 4. Cache-Aside Pattern
```go
// LRU cache for metric deltas
recordDataPoint := func(mb *metadata.MetricsBuilder, ...) {
    key := fmt.Sprintf("%s::%s::%s", counterName, instance, database)
    if oldVal, ok := lruCache.Get(key); ok {
        delta := val - oldVal
        // Record delta
    }
    lruCache.Add(key, val)
}
```

## Testing Strategy

### Unit Testing
- Mock performance counters
- Test query obfuscation
- Validate delta calculations
- Test platform detection

### Integration Testing
- Test against real SQL Server
- Validate all DMV queries
- Test Windows counters
- Verify Azure SQL DB support

### Platform Testing
- Windows Server validation
- Linux SQL Server testing
- Named instance handling
- Connection string formats

## Performance Considerations

1. **Query Efficiency**: Optimized DMV queries
2. **Counter Overhead**: Direct API on Windows
3. **Caching**: LRU for delta calculations
4. **Batch Collection**: Group related metrics
5. **Connection Reuse**: Persistent connections

## Security Model

1. **Authentication**: SQL or Windows auth
2. **Authorization**: VIEW SERVER STATE permission
3. **Query Safety**: Read-only DMV access
4. **Data Protection**: Query obfuscation
5. **Connection Security**: TLS support

## Evolution Points

1. **Always On Support**: Availability group metrics
2. **In-Memory OLTP**: Memory-optimized metrics
3. **ColumnStore**: Column index statistics
4. **Query Store**: Built-in performance data
5. **Extended Events**: Real-time monitoring

## Error Handling Philosophy

1. **Platform Resilience**: Fallback collection methods
2. **Partial Success**: Collect available metrics
3. **Clear Diagnostics**: Platform-specific errors
4. **Graceful Degradation**: Reduce functionality
5. **Recovery**: Automatic reconnection

## Conclusion

The SQL Server receiver exemplifies sophisticated DDD principles:
- Platform-aware design supporting diverse deployments
- Rich domain model capturing SQL Server concepts
- Clean abstraction between collection methods
- Security-first approach with query obfuscation
- Production-ready for enterprise SQL Server monitoring

The architecture successfully handles the complexity of cross-platform SQL Server monitoring while maintaining consistency and reliability.