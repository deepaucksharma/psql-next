# Elasticsearch Receiver - Domain-Driven Design Analysis

## Executive Summary

The Elasticsearch receiver implements a sophisticated telemetry collection system for Elasticsearch clusters, following Domain-Driven Design principles. The receiver is organized into three primary bounded contexts:

1. **Metric Collection Context**: Manages the orchestration of data collection from Elasticsearch
2. **Elasticsearch Integration Context**: Handles API communication and data model translation
3. **Telemetry Export Context**: Transforms Elasticsearch metrics into OpenTelemetry format

## Bounded Contexts

### 1. Metric Collection Context

**Core Responsibility**: Orchestrate periodic collection of metrics from Elasticsearch clusters

**Aggregates**:
- **ElasticsearchScraper** (Aggregate Root)
  - Manages scraping lifecycle
  - Coordinates collection across nodes, clusters, and indices
  - Enforces collection intervals and error handling policies

**Domain Services**:
- **ScrapingOrchestrator**: Coordinates multi-endpoint metric collection
- **ErrorAggregator**: Collects and reports partial failures

**Value Objects**:
- **ScrapeErrors**: Immutable collection of errors from scrape attempts
- **ScraperSettings**: Configuration for scraping behavior

### 2. Elasticsearch Integration Context

**Core Responsibility**: Abstract Elasticsearch API complexity and provide domain-specific data models

**Aggregates**:
- **ElasticsearchClient** (Aggregate Root)
  - Manages HTTP connection lifecycle
  - Handles authentication and version negotiation
  - Provides typed API methods

**Value Objects**:
- **NodeStats**: Comprehensive node metrics (JVM, indices, thread pools)
- **ClusterHealth**: Cluster-wide health status
- **IndexStats**: Index-specific metrics
- **ClusterStats**: Aggregated cluster statistics
- **ClusterMetadata**: Version and cluster identification

**Domain Services**:
- **VersionDetector**: Determines Elasticsearch version for compatibility
- **AuthenticationService**: Manages credential-based access

### 3. Telemetry Export Context

**Core Responsibility**: Transform Elasticsearch domain objects into OpenTelemetry metrics

**Aggregates**:
- **MetricsBuilder** (Aggregate Root)
  - Constructs OpenTelemetry metrics from ES data
  - Manages resource attribution
  - Enforces metric naming conventions

**Value Objects**:
- **ResourceBuilder**: Constructs resource attributes
- **MetricConfig**: Generated metric definitions
- **AttributeSet**: Metric-specific attributes

## Ubiquitous Language

| Term | Context | Definition | Invariants |
|------|---------|------------|------------|
| **Scraper** | Metric Collection | Component that periodically collects metrics | Must respect collection interval |
| **Node** | ES Integration | Individual Elasticsearch instance | Must have unique node ID |
| **Cluster** | ES Integration | Group of ES nodes working together | Must have cluster name |
| **Index** | ES Integration | ES data storage unit | Name must match filter patterns |
| **Shard** | ES Integration | Index subdivision for distribution | Can be primary or replica |
| **MetricBuilder** | Telemetry Export | Constructs OTLP metrics from ES data | Must set proper resource attributes |
| **ResourceAttribute** | Telemetry Export | Metadata identifying metric source | Must include cluster.name |
| **ScrapeError** | Metric Collection | Failed collection attempt | Must include endpoint and reason |
| **CircuitBreaker** | ES Integration | ES memory protection mechanism | Has defined limits |
| **ThreadPool** | ES Integration | ES worker thread group | Must have queue metrics |

## Aggregate Invariants

### ElasticsearchScraper Invariants

1. **Collection Ordering**
   - Must collect cluster metadata before node/index metrics
   - Version detection must precede feature-specific calls
   - Resource attributes must be set before metric emission

2. **Error Handling**
   - Partial failures must not stop other collections
   - All errors must be aggregated and reported
   - Must continue operation despite individual endpoint failures

3. **Configuration Consistency**
   - Node filters must be validated before use
   - Index filters must compile as valid regexes
   - Endpoints must be valid URLs

### ElasticsearchClient Invariants

1. **Connection Management**
   - Must maintain single HTTP client instance
   - Authentication headers must be set if credentials provided
   - Version compatibility header must be sent for ES8+

2. **API Consistency**
   - Response parsing must handle version differences
   - Must validate response status codes
   - Timeout must be enforced on all requests

### MetricsBuilder Invariants

1. **Resource Attribution**
   - Every metric must have cluster.name resource
   - Node metrics must include node.name
   - Index metrics must include index name

2. **Metric Integrity**
   - Metric names must follow OpenTelemetry conventions
   - Units must be correctly specified
   - Attributes must use predefined keys

## Domain Events

### Collection Events
- `ScrapeStarted`: Collection cycle initiated
- `ClusterMetadataCollected`: Version and cluster info retrieved
- `NodeMetricsCollected`: Node statistics gathered
- `ClusterMetricsCollected`: Cluster health/stats gathered
- `IndexMetricsCollected`: Index statistics gathered
- `ScrapeCompleted`: Collection cycle finished
- `ScrapeErrorOccurred`: Collection failure (partial or complete)

### Client Events
- `ClientConnected`: HTTP client established
- `AuthenticationFailed`: Credentials rejected
- `VersionDetected`: ES version determined
- `RequestTimeout`: API call exceeded timeout
- `ResponseParseError`: Invalid API response

## Anti-Patterns Addressed

1. **No Synchronous Blocking**: All API calls are async-capable
2. **No Global State**: Each scraper instance is independent
3. **No Tight Coupling**: Client abstraction allows testing
4. **No Silent Failures**: All errors are tracked and reported
5. **No Version Assumptions**: Dynamic version detection

## Architectural Patterns

### 1. Repository Pattern
```go
type Client interface {
    NodeStats(ctx context.Context, nodes []string) (*model.NodeStats, error)
    ClusterHealth(ctx context.Context) (*model.ClusterHealth, error)
    // ... other methods
}
```

### 2. Factory Pattern
```go
func (f *elasticsearchReceiverFactory) createMetricsReceiver(
    ctx context.Context,
    params receiver.CreateSettings,
    rConf component.Config,
    consumer consumer.Metrics,
) (receiver.Metrics, error)
```

### 3. Builder Pattern
```go
type MetricsBuilder struct {
    // Builds metrics with proper attribution
}
```

### 4. Strategy Pattern
- Different collection strategies based on version
- Configurable authentication strategies

## Testing Strategy

### Unit Testing
- Mock client for scraper tests
- Validate metric building logic
- Test configuration validation

### Integration Testing
- Test against real Elasticsearch versions
- Validate metric output format
- Test error scenarios

### Property-Based Testing
- Metric invariants (resource presence)
- Configuration validation rules
- Filter pattern matching

## Performance Considerations

1. **Concurrent Collection**: Node metrics can be collected in parallel
2. **Caching**: Version detection cached for session
3. **Filtering**: Client-side filtering reduces data transfer
4. **Batch Processing**: Metrics batched for efficiency

## Security Model

1. **Authentication**: Basic auth with secure credential storage
2. **Authorization**: Read-only API access required
3. **Network**: HTTPS support with certificate validation
4. **Secrets**: No credential logging or metric exposure

## Evolution Points

1. **New Metrics**: Add to metadata.yaml
2. **Version Support**: Extend version detection
3. **Authentication**: Add OAuth, API key support
4. **Performance**: Add connection pooling
5. **Resilience**: Circuit breaker for flaky clusters

## Conclusion

The Elasticsearch receiver demonstrates mature DDD principles:
- Clear bounded contexts with defined responsibilities
- Rich domain model with proper invariants
- Clean abstractions for testability
- Robust error handling with partial failure support
- Performance-conscious design with filtering and batching

The architecture supports evolution while maintaining backward compatibility and operational reliability.