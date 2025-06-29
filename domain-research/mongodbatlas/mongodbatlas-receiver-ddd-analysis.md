# MongoDB Atlas Receiver - Domain-Driven Design Analysis

## Executive Summary

The MongoDB Atlas receiver implements a comprehensive cloud monitoring system for MongoDB Atlas, organized into five primary bounded contexts:

1. **Multi-Tenant Metrics Context**: Manages hierarchical metric collection across organizations, projects, and clusters
2. **Atlas API Integration Context**: Handles authenticated API communication with rate limiting
3. **Log Aggregation Context**: Collects and processes various log types with checkpointing
4. **Alert Management Context**: Processes alerts via webhooks or polling
5. **Event Streaming Context**: Captures organizational and project events

## Bounded Contexts

### 1. Multi-Tenant Metrics Context

**Core Responsibility**: Navigate MongoDB Atlas's hierarchical structure to collect metrics at appropriate levels

**Aggregates**:
- **MetricsReceiver** (Aggregate Root)
  - Orchestrates collection across organizational hierarchy
  - Manages time windows for API queries
  - Handles project/cluster filtering
  - Coordinates metric aggregation

**Value Objects**:
- **Organization**: Top-level tenant identifier
- **Project** (Group): Collection of clusters
- **Process**: Individual MongoDB instance
- **TimeWindow**: API query time constraints
- **Granularity**: Metric resolution (e.g., PT1M)

**Domain Services**:
- **HierarchyNavigator**: Traverses org→project→cluster→process
- **MetricPoller**: Executes time-windowed API queries
- **FilterService**: Applies include/exclude rules

**Invariants**:
- Organizations contain projects, projects contain clusters
- Time windows must not exceed API limits
- Filtered clusters must be skipped at all levels
- Metrics must maintain hierarchical attribution

### 2. Atlas API Integration Context

**Core Responsibility**: Provide reliable, authenticated access to MongoDB Atlas APIs

**Aggregates**:
- **AtlasClient** (Aggregate Root)
  - Wraps official MongoDB Atlas SDK
  - Implements retry strategies
  - Handles pagination automatically
  - Manages authentication lifecycle

**Value Objects**:
- **DigestCredentials**: Public/private API key pair
- **PageToken**: Pagination state
- **RateLimitResponse**: 429 response details
- **APIEndpoint**: Versioned API paths

**Domain Services**:
- **RetryStrategy**: Exponential backoff with jitter
- **PaginationHandler**: Automatic page traversal
- **RateLimitHandler**: Respects API rate limits
- **AuthenticationService**: Digest auth implementation

**Invariants**:
- API keys must be valid Base64
- Retry attempts must have increasing delays
- Pagination must complete or fail entirely
- Rate limits must trigger backoff

### 3. Log Aggregation Context

**Core Responsibility**: Collect, parse, and transform various MongoDB log types

**Aggregates**:
- **CombinedLogsReceiver** (Aggregate Root)
  - Manages multiple log sub-receivers
  - Coordinates checkpoint storage
  - Handles lifecycle of all log types

- **LogPoller** (Sub-Aggregate)
  - Polls specific log types
  - Maintains checkpoint state
  - Handles log decompression

**Value Objects**:
- **LogEntry**: Parsed log line
- **LogCheckpoint**: Last processed timestamp
- **LogType**: mongodb.gz, mongos.gz, audit
- **Severity**: Log level mapping

**Domain Services**:
- **LogDecoder**: Parses gzipped logs
- **CheckpointManager**: Persists processing state
- **LogTransformer**: Converts to OTel format
- **TimeExtractor**: Extracts timestamps from logs

**Invariants**:
- Checkpoints must monotonically increase
- Log parsing must handle version differences
- Gzipped content must be validated
- Missing logs must not lose checkpoint

### 4. Alert Management Context

**Core Responsibility**: Process MongoDB Atlas alerts through webhooks or polling

**Aggregates**:
- **AlertReceiver** (Aggregate Root)
  - Supports dual mode: listen/poll
  - Validates webhook signatures
  - Deduplicates alerts
  - Maintains alert checkpoint

**Value Objects**:
- **Alert**: Structured alert data
- **AlertCheckpoint**: Last processed time
- **WebhookSignature**: HMAC validation
- **AlertMode**: LISTEN or POLL

**Domain Services**:
- **WebhookServer**: HTTPS endpoint for alerts
- **SignatureValidator**: HMAC-SHA256 verification
- **AlertPoller**: Active alert retrieval
- **AlertDeduplicator**: Prevents reprocessing

**Invariants**:
- Webhook signatures must validate
- Poll mode must respect checkpoint
- Alerts must not be duplicated
- TLS required for webhook endpoint

### 5. Event Streaming Context

**Core Responsibility**: Capture and process organizational and project events

**Aggregates**:
- **EventReceiver** (Aggregate Root)
  - Polls organization and project events
  - Filters by event types
  - Maintains per-scope checkpoints

**Value Objects**:
- **Event**: Typed event data
- **EventType**: Categories of events
- **EventScope**: Organization or Project
- **EventCheckpoint**: Per-scope timestamp

**Domain Services**:
- **EventPoller**: Time-based event retrieval
- **EventFilter**: Type-based filtering
- **EventTransformer**: Convert to logs
- **ScopeManager**: Handles multi-scope polling

**Invariants**:
- Events must be processed in order
- Checkpoints per scope must be independent
- Event types must be validated
- Time windows must not overlap

## Domain Events

### Collection Events
- `MetricsCollectionStarted`: Polling cycle begins
- `OrganizationDiscovered`: New org found
- `ProjectEnumerated`: Projects listed
- `ClusterMetricsCollected`: Cluster data retrieved
- `CollectionCycleCompleted`: All metrics gathered

### API Events
- `RateLimitEncountered`: 429 response received
- `RetryAttemptStarted`: Backoff initiated
- `PaginationCompleted`: All pages retrieved
- `AuthenticationFailed`: Invalid credentials

### Log Events
- `LogPollInitiated`: Log retrieval started
- `LogsParsed`: Entries extracted
- `CheckpointUpdated`: Progress saved
- `LogDecodingFailed`: Parse error

### Alert Events
- `WebhookReceived`: Alert via HTTPS
- `SignatureValidated`: HMAC verified
- `AlertProcessed`: Converted to log
- `DuplicateAlertSkipped`: Already processed

## Architectural Patterns

### 1. Strategy Pattern (Collection Modes)
```go
type AlertMode string
const (
    AlertModeListen AlertMode = "LISTEN"
    AlertModePoll   AlertMode = "POLL"
)
```

### 2. Composite Pattern (Combined Receivers)
```go
type combinedLogsReceiver struct {
    alerts     *alertsReceiver
    logs       *logsReceiver
    events     *eventsReceiver
    accessLogs *accessLogsReceiver
}
```

### 3. Repository Pattern (Checkpointing)
```go
type storage interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte) error
}
```

### 4. Factory Pattern
```go
func (f *factory) createMetricsReceiver(...) (receiver.Metrics, error)
func (f *factory) createCombinedLogsReceiver(...) (receiver.Logs, error)
```

## Anti-Patterns Addressed

1. **No API Flooding**: Rate limit handling with backoff
2. **No Data Loss**: Checkpoint persistence
3. **No Infinite Loops**: Pagination limits
4. **No Security Bypass**: HMAC validation required
5. **No Silent Failures**: Comprehensive error logging

## Testing Strategy

### Unit Testing
- Mock Atlas client for API calls
- Test pagination edge cases
- Validate checkpoint logic
- Test log parsing formats

### Integration Testing
- Test against Atlas sandbox
- Validate webhook signatures
- Test rate limit handling
- Verify metric accuracy

### Security Testing
- HMAC signature validation
- TLS certificate verification
- API key security
- Webhook endpoint hardening

## Performance Considerations

1. **API Efficiency**: Batch requests where possible
2. **Pagination**: Configurable page sizes
3. **Caching**: Reuse organization/project lists
4. **Parallel Processing**: Could parallelize cluster queries
5. **Checkpoint Efficiency**: Minimize storage operations

## Security Model

1. **Authentication**: Digest auth with API keys
2. **Webhook Security**: HMAC-SHA256 signatures
3. **TLS**: Required for webhook endpoints
4. **Credential Storage**: Secure key management
5. **Audit Trail**: Event collection provides audit capability

## Evolution Points

1. **New Metrics**: Extend metric types collected
2. **Streaming**: Real-time metric streaming
3. **Advanced Filtering**: Complex cluster selectors
4. **Multi-Region**: Regional API endpoint support
5. **Compression**: Support for compressed API responses

## Error Handling Philosophy

1. **Graceful Degradation**: Continue with partial data
2. **Checkpoint Recovery**: Resume from last known good state
3. **Detailed Context**: Include org/project/cluster in errors
4. **Retry Intelligence**: Exponential backoff with jitter
5. **Observability**: Emit metrics about collection health

## Conclusion

The MongoDB Atlas receiver demonstrates sophisticated DDD for cloud service integration:
- Clear separation between API mechanics and domain logic
- Rich modeling of Atlas's multi-tenant hierarchy
- Robust state management with checkpointing
- Multiple collection strategies (push/pull)
- Security-first design with webhook validation

The architecture successfully handles the complexity of monitoring a multi-tenant cloud database service while maintaining reliability, security, and operational simplicity.