# CouchDB Receiver - Domain-Driven Design Analysis

## Executive Summary

The CouchDB receiver implements a document database monitoring system organized into four primary bounded contexts:

1. **Statistics API Context**: Interfaces with CouchDB's stats REST API
2. **HTTP Operations Context**: Monitors request/response patterns and performance
3. **Database Operations Context**: Tracks database and file operations
4. **Node Management Context**: Handles node identification and resource attribution

## Bounded Contexts

### 1. Statistics API Context

**Core Responsibility**: Retrieve and parse statistics from CouchDB's REST API

**Aggregates**:
- **StatsCollector** (Aggregate Root)
  - Fetches statistics from _stats endpoint
  - Manages API authentication
  - Handles version compatibility
  - Parses nested JSON responses

**Value Objects**:
- **StatsEndpoint**: API path construction
- **NodeName**: Target node identifier (_local)
- **StatsResponse**: Raw JSON statistics
- **ApiVersion**: CouchDB version (2.3+ or 3.1+)

**Domain Services**:
- **HttpClient**: Manages authenticated requests
- **ResponseParser**: Extracts metrics from JSON
- **EndpointBuilder**: Constructs API paths
- **AuthenticationHandler**: Basic auth management

**Invariants**:
- Endpoint must include node name
- Authentication required for all requests
- Response must be valid JSON
- Stats structure varies by version

### 2. HTTP Operations Context

**Core Responsibility**: Monitor HTTP request patterns, methods, and response codes

**Aggregates**:
- **HttpMetricsMonitor** (Aggregate Root)
  - Tracks requests by method
  - Monitors response status codes
  - Measures request timing
  - Categorizes view requests

**Value Objects**:
- **HttpMethod**: GET, POST, PUT, DELETE, etc.
- **StatusCode**: HTTP response codes (2xx, 3xx, 4xx, 5xx)
- **RequestTime**: Average request duration
- **ViewType**: Temporary or permanent views

**Domain Services**:
- **MethodCounter**: Aggregates by HTTP method
- **StatusAnalyzer**: Groups by status code
- **TimingCalculator**: Computes averages
- **ViewClassifier**: Separates view types

**Invariants**:
- Methods must be valid HTTP verbs
- Status codes follow HTTP standards
- Request times in milliseconds
- View counts are cumulative

### 3. Database Operations Context

**Core Responsibility**: Track database-level operations and resource usage

**Aggregates**:
- **DatabaseMonitor** (Aggregate Root)
  - Counts open databases
  - Tracks file descriptors
  - Monitors read/write operations
  - Measures bulk operations

**Value Objects**:
- **DatabaseCount**: Number of open databases
- **FileDescriptorCount**: Open file handles
- **OperationType**: database_reads or database_writes
- **BulkRequestCount**: Bulk operation frequency

**Domain Services**:
- **ResourceTracker**: Monitors system resources
- **OperationCounter**: Tracks DB operations
- **BulkAnalyzer**: Measures bulk patterns
- **ResourceCalculator**: Computes usage

**Invariants**:
- Open counts must be non-negative
- Operations are monotonic counters
- File descriptors have system limits
- Bulk requests subset of total requests

### 4. Node Management Context

**Core Responsibility**: Manage node identity and resource attribution

**Aggregates**:
- **NodeManager** (Aggregate Root)
  - Identifies local node
  - Manages node metadata
  - Handles multi-node awareness
  - Provides resource attributes

**Value Objects**:
- **LocalNode**: Default node identifier
- **NodeMetadata**: Node-specific information
- **ResourceAttributes**: OTel resource tags
- **ClusterRole**: Node's cluster position

**Domain Services**:
- **NodeIdentifier**: Determines node name
- **MetadataProvider**: Supplies node info
- **AttributeBuilder**: Creates resource tags
- **ClusterAwareness**: Future multi-node support

**Invariants**:
- Node name must be non-empty
- Local node always accessible
- Resource attributes required
- Node name immutable per scrape

## Domain Events

### API Events
- `StatsRequested`: API call initiated
- `StatsReceived`: Response obtained
- `AuthenticationFailed`: Invalid credentials
- `ApiTimeout`: Request exceeded limit

### HTTP Events
- `RequestRateChanged`: Traffic pattern shift
- `ErrorRateIncreased`: Higher 4xx/5xx responses
- `ViewLoadHigh`: Heavy view usage
- `BulkOperationSpike`: Bulk request surge

### Database Events
- `DatabasesOpened`: New databases created
- `FileDescriptorLimit`: Approaching OS limit
- `WriteLoadHigh`: Heavy write operations
- `ReadWriteImbalance`: Skewed operations

### Node Events
- `NodeIdentified`: Node name determined
- `NodeUnreachable`: Connection failed
- `VersionDetected`: CouchDB version found
- `ConfigurationChanged`: Settings updated

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **_stats** | API | Statistics endpoint | Returns JSON metrics |
| **_local** | Node | Local node identifier | Default target node |
| **View** | HTTP | MapReduce query | Can be temporary or saved |
| **Bulk Request** | Operations | Multi-document operation | Atomic batch processing |
| **File Descriptor** | Resources | OS file handle | Limited by OS settings |
| **Database** | Storage | Document container | Named collection |
| **Replication** | Operations | Database sync | Creates HTTP traffic |
| **Design Document** | Views | View definitions | Contains map/reduce |

## Anti-Patterns Addressed

1. **No Direct Database Access**: Uses stats API only
2. **No Heavy Queries**: Lightweight stats endpoint
3. **No State Modification**: Read-only operations
4. **No Credential Storage**: Uses OTel config
5. **No Connection Pooling Issues**: Single HTTP client

## Architectural Patterns

### 1. Repository Pattern (HTTP Client)
```go
type client interface {
    Get(path string) ([]byte, error)
    GetStats(nodeName string) ([]byte, error)
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

### 3. Scraper Pattern
```go
type couchdbScraper struct {
    client   client
    settings receiver.Settings
    config   *Config
    mb       *metadata.MetricsBuilder
}
```

### 4. Path Navigation Pattern
```go
// Navigate nested JSON safely
func getValueFromBody(body, key string) (float64, error) {
    path := strings.Split(key, "/")
    // Traverse JSON path
}
```

## Testing Strategy

### Unit Testing
- Mock HTTP client for scraper tests
- Test JSON parsing with fixtures
- Validate metric mappings
- Test authentication headers

### Integration Testing
- Test against real CouchDB
- Validate version compatibility
- Test error scenarios
- Verify metric accuracy

### Version Testing
- CouchDB 2.3 compatibility
- CouchDB 3.x compatibility
- Handle schema differences
- Future version support

## Performance Considerations

1. **Single Endpoint**: One API call per scrape
2. **Lightweight Stats**: Pre-aggregated by CouchDB
3. **No Pagination**: Single response
4. **Efficient Parsing**: Single JSON traversal
5. **Resource Reuse**: HTTP client persistence

## Security Model

1. **Authentication**: Basic auth required
2. **Authorization**: Read access to _stats
3. **No Admin Access**: Statistics only
4. **Credential Security**: OTel secret handling
5. **TLS Support**: HTTPS connections

## Evolution Points

1. **Cluster Support**: Multi-node monitoring
2. **Replication Metrics**: Sync statistics
3. **Shard Metrics**: Distributed data stats
4. **Compaction Stats**: Maintenance metrics
5. **Custom Stats**: User-defined metrics

## Error Handling Philosophy

1. **Partial Success**: Individual metric failures OK
2. **Clear Attribution**: Error includes metric name
3. **Non-Fatal**: Continue on errors
4. **Diagnostic Logging**: HTTP status codes
5. **Graceful Degradation**: Collect what's available

## Conclusion

The CouchDB receiver demonstrates focused DDD principles:
- Clear separation of API interaction from metric domains
- Simple but effective stats collection
- Version-aware implementation
- Security-conscious design
- Extensible for cluster monitoring

The architecture successfully monitors CouchDB instances with minimal overhead while maintaining flexibility for future enhancements.