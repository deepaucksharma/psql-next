# Redis Receiver - Domain-Driven Design Analysis

## Executive Summary

The Redis receiver implements a focused monitoring system for Redis databases, organized into four primary bounded contexts:

1. **Redis Metrics Context**: Collects and transforms Redis INFO command output
2. **Redis Protocol Context**: Manages Redis connections and command execution
3. **Keyspace Analytics Context**: Monitors database-level key statistics
4. **Command Performance Context**: Tracks command execution and latency

## Bounded Contexts

### 1. Redis Metrics Context

**Core Responsibility**: Parse and transform Redis INFO output into structured metrics

**Aggregates**:
- **RedisScraper** (Aggregate Root)
  - Orchestrates metric collection cycles
  - Manages uptime tracking for accurate timestamps
  - Coordinates INFO section parsing
  - Maps INFO keys to metric recorders

**Value Objects**:
- **InfoData**: Parsed key-value pairs from INFO
- **MetricTimestamp**: Time-adjusted based on uptime
- **CPUState**: CPU time by category (sys, user, etc.)
- **MemoryMetrics**: Used, peak, RSS, fragmentation

**Domain Services**:
- **InfoParser**: Parses INFO command output
- **MetricMapper**: Routes INFO keys to recorders
- **UptimeCalculator**: Adjusts timestamps
- **SectionExtractor**: Splits INFO by sections

**Invariants**:
- INFO must be parsed before metric recording
- Uptime required for timestamp calculation
- All known INFO keys must map to metrics
- Unknown keys logged but not fatal

### 2. Redis Protocol Context

**Core Responsibility**: Abstract Redis wire protocol and manage connections

**Aggregates**:
- **RedisClient** (Aggregate Root)
  - Encapsulates go-redis client
  - Manages connection lifecycle
  - Executes INFO commands
  - Handles authentication

**Value Objects**:
- **ConnectionConfig**: Endpoint, auth, TLS settings
- **InfoCommand**: Command specification
- **RedisResponse**: Raw INFO output
- **Delimiter**: Line ending format (CRLF)

**Domain Services**:
- **ConnectionBuilder**: Creates Redis connections
- **AuthenticationHandler**: Manages ACL/password auth
- **TLSConfigurator**: Sets up secure connections
- **CommandExecutor**: Runs Redis commands

**Invariants**:
- Connection must be established before commands
- Authentication required if configured
- TLS settings must be complete if enabled
- INFO command must be read-only

### 3. Keyspace Analytics Context

**Core Responsibility**: Monitor per-database key statistics and expiration patterns

**Aggregates**:
- **KeyspaceMonitor** (Aggregate Root)
  - Tracks per-database metrics
  - Parses keyspace INFO section
  - Monitors key expiration patterns

**Value Objects**:
- **DatabaseID**: Numeric database identifier
- **KeyspaceStats**: Keys, expires, avg_ttl
- **ExpirationRatio**: Expires/keys percentage
- **TTLDistribution**: Average TTL metrics

**Domain Services**:
- **KeyspaceParser**: Extracts db-specific stats
- **ExpirationAnalyzer**: Calculates ratios
- **DatabaseEnumerator**: Lists active databases

**Invariants**:
- Database IDs must be non-negative
- Expires cannot exceed total keys
- Average TTL must be positive or zero
- Keyspace format must match pattern

### 4. Command Performance Context

**Core Responsibility**: Track Redis command execution statistics and latency

**Aggregates**:
- **CommandMonitor** (Aggregate Root)
  - Collects per-command statistics
  - Tracks latency percentiles
  - Calculates operations per second

**Value Objects**:
- **CommandName**: Redis command identifier
- **CommandStats**: Calls, usec, usec_per_call
- **LatencyPercentile**: p50, p99, p99.9
- **CommandRate**: Operations per second

**Domain Services**:
- **CommandStatsParser**: Parses commandstats section
- **LatencyCalculator**: Computes percentiles
- **RateCalculator**: Derives ops/sec from calls

**Invariants**:
- Command names must be valid Redis commands
- Latency values must be non-negative
- Call counts must be cumulative
- Percentiles must be ordered (p50 < p99 < p99.9)

## Domain Events

### Collection Events
- `ScraperStarted`: Redis connection established
- `InfoRetrieved`: INFO command successful
- `MetricsRecorded`: Collection cycle complete
- `ScraperStopped`: Clean shutdown

### Connection Events
- `ClientConnected`: Redis connection ready
- `AuthenticationSucceeded`: ACL/password accepted
- `ConnectionLost`: Network or server failure
- `TLSHandshakeCompleted`: Secure connection

### Parsing Events
- `InfoParsed`: Raw output transformed
- `KeyspaceDetected`: Database metrics found
- `CommandStatsFound`: Command data available
- `ParseWarning`: Non-fatal parsing issue

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **INFO** | Redis | Server statistics command | Read-only operation |
| **Keyspace** | Redis | Per-database key statistics | Format: dbN:keys=X,expires=Y |
| **RDB** | Redis | Redis Database file | Persistence mechanism |
| **AOF** | Redis | Append Only File | Durability log |
| **TTL** | Keyspace | Time To Live | Seconds until expiration |
| **RSS** | Memory | Resident Set Size | OS memory view |
| **Eviction** | Memory | Key removal under pressure | Policy-driven |
| **Replica** | Replication | Read-only copy | Follows primary |
| **Lag** | Replication | Replica delay in bytes | Non-negative |

## Anti-Patterns Addressed

1. **No Command Execution**: Only INFO command used
2. **No Key Access**: No data inspection
3. **No Blocking Operations**: Fast INFO retrieval
4. **No Write Operations**: Read-only monitoring
5. **No Cluster Assumptions**: Works with standalone

## Architectural Patterns

### 1. Repository Pattern
```go
type client interface {
    retrieveInfo() (string, error)
    delimiter() string
    close() error
}
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
type infoParser struct {
    client client
}

func (p *infoParser) info() (info, error) {
    // Parse INFO output into structured data
}
```

### 4. Strategy Pattern (Metric Recording)
```go
type metricRecorder func(mb *metadata.MetricsBuilder, info, now pcommon.Timestamp)

var recorders = map[string]metricRecorder{
    "used_cpu_sys": recordCPUSysUsed,
    "used_memory": recordMemoryUsed,
    // ... more mappings
}
```

## Aggregate Invariants Detail

### RedisScraper Aggregate
1. **Collection Invariants**
   - INFO must be retrieved once per scrape
   - All sections processed in single cycle
   - Timestamp consistency across metrics
   - Resource attributes set once

2. **State Management**
   - Uptime tracked between scrapes
   - Previous uptime for restart detection
   - Client lifecycle managed properly

### RedisClient Aggregate
1. **Connection Invariants**
   - Single connection pool per receiver
   - Authentication before commands
   - Graceful shutdown required
   - No automatic reconnection in scraper

2. **Protocol Invariants**
   - INFO command format fixed
   - Response always text-based
   - Delimiter always CRLF
   - Sections separated by blank lines

## Testing Strategy

### Unit Testing
- Mock client for scraper tests
- Test INFO parsing edge cases
- Validate metric mappings
- Test keyspace parsing

### Integration Testing
- Test against real Redis
- Validate all INFO sections
- Test authentication methods
- Verify TLS connections

### Compatibility Testing
- Test multiple Redis versions
- Validate new INFO fields
- Handle missing sections
- Test Redis forks (KeyDB, etc.)

## Performance Considerations

1. **Single Command**: One INFO call per scrape
2. **Efficient Parsing**: Linear scan of output
3. **Connection Pooling**: Reuse connections
4. **Selective Metrics**: Configurable collection
5. **Low Overhead**: INFO command is fast

## Security Model

1. **Authentication**: Password or ACL username/password
2. **Authorization**: INFO command only
3. **Encryption**: TLS/SSL support
4. **No Data Access**: Metadata only
5. **Read-Only**: No state modification

## Evolution Points

1. **Cluster Support**: Monitor cluster topology
2. **Module Metrics**: Redis module statistics
3. **Stream Metrics**: Redis Streams monitoring
4. **Memory Analysis**: Detailed memory breakdown
5. **Slow Log**: Query performance tracking

## Error Handling Philosophy

1. **Graceful Degradation**: Missing keys ignored
2. **Partial Success**: Collect available metrics
3. **Clear Warnings**: Log parsing issues
4. **No Fatal Errors**: Continue operation
5. **Connection Resilience**: Reconnect on next scrape

## Conclusion

The Redis receiver demonstrates focused DDD principles:
- Clear separation of Redis protocol from metrics
- Simple but effective INFO parsing
- Minimal performance impact on Redis
- Security-conscious design
- Extensible for new Redis features

The architecture successfully monitors Redis instances comprehensively while maintaining simplicity and operational safety.