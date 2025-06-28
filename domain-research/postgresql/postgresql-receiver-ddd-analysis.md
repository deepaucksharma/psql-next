# PostgreSQL Receiver - Domain-Driven Design Analysis

## Executive Summary

The PostgreSQL receiver implements a sophisticated database monitoring system organized into five primary bounded contexts:

1. **Database Metrics Context**: Collects comprehensive database statistics and performance metrics
2. **Query Analysis Context**: Monitors and analyzes query performance and patterns
3. **Table & Index Management Context**: Tracks table and index usage, efficiency, and maintenance
4. **Replication Monitoring Context**: Observes replication topology and lag
5. **Connection Management Context**: Handles database connections with optional pooling

## Bounded Contexts

### 1. Database Metrics Context

**Core Responsibility**: Collect system-wide database performance metrics from PostgreSQL statistics collector

**Aggregates**:
- **PostgreSQLScraper** (Aggregate Root)
  - Orchestrates metric collection across databases
  - Manages metric caching for delta calculations
  - Coordinates concurrent collection
  - Handles database filtering

**Value Objects**:
- **DatabaseStats**: Statistics from pg_stat_database
- **BGWriterStats**: Background writer performance
- **CheckpointerStats**: Checkpoint activity metrics
- **ConnectionMetrics**: Active/max connections
- **WALMetrics**: Write-ahead log age and size

**Domain Services**:
- **MetricCollector**: Gathers metrics from multiple sources
- **DeltaCalculator**: Computes rate metrics from counters
- **CacheManager**: LRU cache for metric history
- **ErrorAggregator**: Collects partial failures

**Invariants**:
- All databases must be attempted unless filtered
- Delta calculations require previous values
- Metrics must have consistent timestamps
- Resource attributes must include database name

### 2. Query Analysis Context

**Core Responsibility**: Monitor, analyze, and obfuscate database queries for performance insights

**Aggregates**:
- **QueryMonitor** (Aggregate Root)
  - Collects active query samples
  - Tracks top queries by execution time
  - Manages query plan caching
  - Handles query obfuscation

**Value Objects**:
- **QuerySample**: Active query snapshot
- **TopQuery**: Historical query statistics
- **QueryPlan**: Execution plan details
- **ObfuscatedQuery**: Sanitized query text
- **QueryIdentifier**: Unique query fingerprint

**Domain Services**:
- **QueryObfuscator**: Removes sensitive data
- **PlanExplainer**: Retrieves execution plans
- **TopQuerySelector**: Priority queue for top queries
- **PlanCache**: TTL cache for execution plans

**Invariants**:
- Query text must be obfuscated before emission
- Top queries limited to configured count
- Plans cached to avoid repeated EXPLAIN
- Query IDs must be consistent

### 3. Table & Index Management Context

**Core Responsibility**: Monitor table and index usage, efficiency, and maintenance operations

**Aggregates**:
- **TableMonitor** (Aggregate Root)
  - Tracks table statistics and I/O
  - Monitors vacuum and analyze operations
  - Measures table sizes and growth

- **IndexMonitor** (Aggregate Root)
  - Tracks index usage and efficiency
  - Monitors index sizes
  - Identifies unused indexes

**Value Objects**:
- **TableStats**: Row counts, operations, vacuum info
- **TableIOStats**: Block reads, hits, misses
- **IndexStats**: Scans, reads, fetches
- **VacuumInfo**: Last vacuum/analyze timestamps

**Domain Services**:
- **SizeCalculator**: Computes table/index sizes
- **VacuumAnalyzer**: Tracks maintenance operations
- **UsageAnalyzer**: Identifies hot/cold objects

**Invariants**:
- Table size cannot be negative
- Dead tuples tracked for vacuum needs
- Index scans indicate usage patterns
- Schema must be tracked if enabled

### 4. Replication Monitoring Context

**Core Responsibility**: Monitor PostgreSQL replication topology, lag, and health

**Aggregates**:
- **ReplicationMonitor** (Aggregate Root)
  - Tracks replication connections
  - Measures replication lag
  - Monitors pending WAL bytes

**Value Objects**:
- **ReplicationSlot**: Slot name and state
- **ReplicationLag**: Time and byte lag
- **StandbyInfo**: Standby server details
- **WALPosition**: LSN positions

**Domain Services**:
- **LagCalculator**: Computes various lag metrics
- **TopologyMapper**: Identifies replication structure
- **HealthChecker**: Validates replication health

**Invariants**:
- Lag values must be non-negative
- LSN positions must increase
- Standby state must be valid
- Slot names must be unique

### 5. Connection Management Context

**Core Responsibility**: Efficiently manage database connections with optional pooling

**Aggregates**:
- **ConnectionFactory** (Aggregate Root)
  - Creates and manages client connections
  - Implements pooling strategies
  - Handles connection lifecycle

**Value Objects**:
- **ConnectionString**: DSN for PostgreSQL
- **PoolConfig**: Pool size and timeout settings
- **TransportType**: TCP or Unix socket
- **SSLConfig**: TLS/SSL parameters

**Domain Services**:
- **ConnectionBuilder**: Constructs connection strings
- **PoolManager**: Manages connection pools
- **SSLConfigurator**: Handles secure connections

**Invariants**:
- Connection strings must be valid
- Pool sizes must be positive
- SSL mode must be supported
- Transport type must be TCP or Unix

## Domain Events

### Collection Events
- `ScraperStarted`: Collection cycle initiated
- `DatabaseDiscovered`: New database found
- `MetricsCollected`: Successful metric gathering
- `CollectionError`: Partial or complete failure
- `CacheUpdated`: Metric history stored

### Query Events
- `QuerySampleCollected`: Active query captured
- `TopQueryIdentified`: High-cost query found
- `QueryObfuscated`: Sensitive data removed
- `PlanCached`: Execution plan stored
- `PlanExpired`: Cache entry removed

### Table/Index Events
- `TableVacuumed`: Maintenance operation detected
- `IndexUnused`: Zero-scan index found
- `TableBloated`: High dead tuple ratio
- `SizeThresholdExceeded`: Object grew significantly

### Replication Events
- `ReplicationLagIncreased`: Lag exceeded threshold
- `StandbyConnected`: New replica joined
- `StandbyDisconnected`: Replica connection lost
- `SlotCreated`: New replication slot

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **pg_stat_*** | PostgreSQL | System statistics views | Updated by stats collector |
| **Tuple** | Table | Row in PostgreSQL terminology | Can be live or dead |
| **VACUUM** | Maintenance | Reclaims dead tuple space | Required periodically |
| **LSN** | Replication | Log Sequence Number | Monotonically increasing |
| **WAL** | Storage | Write-Ahead Log | Required for durability |
| **Bloat** | Table | Wasted space from dead tuples | Affects performance |
| **Toast** | Storage | The Oversized-Attribute Storage | For large values |
| **Blks** | I/O | Blocks (8KB pages) | Unit of I/O |
| **Bgwriter** | Background | Background writer process | Writes dirty buffers |

## Anti-Patterns Addressed

1. **No Superuser Required**: Uses standard monitoring views
2. **No Query Logging**: Samples active queries only
3. **No Sensitive Data**: Query obfuscation applied
4. **No Connection Explosion**: Optional pooling
5. **No Blocking Queries**: Read-only with timeouts

## Architectural Patterns

### 1. Repository Pattern
```go
type client interface {
    getDatabaseStats(ctx context.Context, databases []string) ([]databaseStats, error)
    getTableStats(ctx context.Context, db string) ([]tableStats, error)
    // ... other query methods
}
```

### 2. Factory Pattern with Feature Gates
```go
func createClientFactory(featuregate *featuregate.Registry) clientFactory {
    if featuregate.IsEnabled(connectionPoolingID) {
        return &poolClientFactory{}
    }
    return &defaultClientFactory{}
}
```

### 3. Template Pattern (SQL Queries)
```go
// Uses Go templates for dynamic SQL generation
querySampleTemplate.tmpl
topQueryTemplate.tmpl
```

### 4. Cache-Aside Pattern
```go
// LRU cache for metrics, TTL cache for plans
lruCache := cache.NewLRUCache[string, *metadata.MetricsBuilder](...)
ttlCache := cache.NewTTLCache[string, string](...)
```

### 5. Priority Queue Pattern
```go
// Top query selection using heap
topQueries := &queryPriorityQueue{}
heap.Init(topQueries)
```

## Testing Strategy

### Unit Testing
- Mock client for scraper tests
- Test delta calculations
- Validate query obfuscation
- Test priority queue logic

### Integration Testing
- Test against real PostgreSQL
- Validate all pg_stat queries
- Test connection pooling
- Verify replication metrics

### Performance Testing
- Concurrent collection scaling
- Cache effectiveness
- Pool sizing optimization
- Query performance impact

## Performance Considerations

1. **Connection Pooling**: Reduces connection overhead
2. **Concurrent Collection**: Parallel database queries
3. **Metric Caching**: Avoids recalculation
4. **Plan Caching**: Reduces EXPLAIN overhead
5. **Selective Collection**: Database filtering

## Security Model

1. **Authentication**: Username/password or peer auth
2. **Authorization**: SELECT on pg_stat views
3. **Encryption**: Full SSL/TLS support
4. **Query Safety**: Read-only operations
5. **Data Protection**: Query obfuscation

## Evolution Points

1. **Partition Monitoring**: Table partition metrics
2. **Extension Metrics**: pg_stat_statements, etc.
3. **Custom Metrics**: User-defined queries
4. **Streaming Replication**: More detailed lag metrics
5. **Auto-discovery**: Dynamic database detection

## Error Handling Philosophy

1. **Partial Success**: Collect available metrics
2. **Database Isolation**: Failures don't cascade
3. **Clear Attribution**: Error includes database
4. **Graceful Degradation**: Feature fallbacks
5. **Retry Logic**: Transient failure handling

## Conclusion

The PostgreSQL receiver exemplifies mature DDD principles:
- Rich domain model capturing PostgreSQL concepts
- Clear bounded contexts with defined responsibilities
- Sophisticated caching and performance optimizations
- Security-first design with query obfuscation
- Extensible architecture supporting new PostgreSQL features

The design successfully monitors PostgreSQL databases comprehensively while maintaining operational efficiency and security.