# ADR-0001: Unified Collector Architecture

## Status
Accepted

## Context
New Relic needs a PostgreSQL metrics collector that can support both the existing New Relic Infrastructure (NRI) format and the emerging OpenTelemetry Protocol (OTLP) format. The collector must be:

1. **Backward Compatible**: Support existing NRI v4 protocol
2. **Future Ready**: Support OTLP for modern observability
3. **Performant**: Handle high-volume metric collection
4. **Secure**: Protect sensitive query data and credentials
5. **Extensible**: Allow addition of new output formats

## Decision
We will implement a unified collector with a pluggable adapter architecture:

### Core Components
1. **Collection Engine**: Single metrics collection system
2. **Adapter Pattern**: Pluggable output format adapters
3. **Dynamic Dispatch**: Runtime adapter management using trait objects
4. **Dual Output**: Simultaneous NRI and OTLP export

### Architecture Pattern
```
PostgreSQL → Collection Engine → [NRI Adapter, OTLP Adapter, ...] → Outputs
```

## Rationale

### 1. Unified vs. Separate Collectors
**Considered Options:**
- A) Separate collectors for NRI and OTLP
- B) Unified collector with adapters

**Decision: B) Unified collector**

**Reasons:**
- **Single Source of Truth**: One collection logic reduces inconsistencies
- **Resource Efficiency**: Shared PostgreSQL connections and query execution
- **Maintenance**: Single codebase for core collection logic
- **Consistency**: Same metric timestamps and collection intervals

### 2. Adapter Architecture
**Pattern**: Dynamic dispatch with trait objects

```rust
#[async_trait]
pub trait MetricAdapterDyn: Send + Sync {
    async fn adapt_dyn(&self, metrics: &UnifiedMetrics) -> Result<Box<dyn MetricOutputDyn>, ProcessError>;
    fn name(&self) -> &str;
}
```

**Benefits:**
- **Runtime Flexibility**: Add/remove adapters without recompilation
- **Type Safety**: Compile-time guarantees with runtime flexibility
- **Testability**: Easy mocking and testing of individual adapters

### 3. Simultaneous Output
**Decision**: Support simultaneous multi-format output

**Benefits:**
- **Migration Path**: Gradual transition from NRI to OTLP
- **Redundancy**: Multiple monitoring systems can consume data
- **A/B Testing**: Compare formats with identical data

## Implementation Details

### Adapter Interface
```rust
pub trait MetricOutputDyn: Send + Sync {
    fn serialize(&self) -> Result<Vec<u8>, ProcessError>;
    fn content_type(&self) -> &'static str;
}
```

### Collection Flow
1. **Detect Capabilities**: PostgreSQL version and extensions
2. **Collect Metrics**: Single unified collection
3. **Adapt Formats**: Transform to NRI/OTLP simultaneously
4. **Export**: Send to configured endpoints

### Error Handling
- **Isolated Failures**: One adapter failure doesn't affect others
- **Graceful Degradation**: Continue with working adapters
- **Detailed Logging**: Per-adapter error reporting

## Consequences

### Positive
- **Flexibility**: Easy to add new output formats
- **Consistency**: Same data across all formats
- **Performance**: Shared collection and connection pooling
- **Maintainability**: Single codebase for core logic

### Negative
- **Complexity**: Dynamic dispatch adds runtime overhead
- **Memory Usage**: Multiple format transformations
- **Dependencies**: More crates and potential conflicts

### Risks
- **Performance**: Multiple serialization passes
- **Memory**: Holding multiple format representations
- **Testing**: Complex integration testing required

## Alternatives Considered

### 1. Format-Specific Collectors
**Rejected**: Would duplicate collection logic and PostgreSQL connections

### 2. Static Compilation
**Rejected**: Less flexible, requires recompilation for new formats

### 3. Configuration-Based Selection
**Rejected**: Doesn't support simultaneous output

## Monitoring Success
- **Performance**: Collection latency < 100ms
- **Memory**: Stable memory usage under 100MB
- **Reliability**: >99.9% collection success rate
- **Format Parity**: Identical data across formats

## Related Decisions
- ADR-0002: Query Sanitization Strategy
- ADR-0003: Connection Pooling Implementation
- ADR-0004: OTLP Protocol Selection