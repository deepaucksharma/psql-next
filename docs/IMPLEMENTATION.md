# Implementation Details

## Architecture Overview

The PostgreSQL Unified Collector is built with a modular architecture that separates collection, processing, and output concerns:

```
┌─────────────────────────────────────────────────────────────────────┐
│                     PostgreSQL Database                              │
│  ┌─────────────────┬──────────────────┬─────────────────────────┐  │
│  │ pg_stat_statements │ pg_stat_activity │ pg_locks + other views │  │
│  └─────────────────┴──────────────────┴─────────────────────────┘  │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
┌─────────────────────────────────▼───────────────────────────────────┐
│                        Collection Engine                             │
│  ┌─────────────────┬──────────────────┬─────────────────────────┐  │
│  │ Query Executor  │ Extension Manager │ Capability Detection    │  │
│  └─────────────────┴──────────────────┴─────────────────────────┘  │
│  ┌─────────────────┬──────────────────┬─────────────────────────┐  │
│  │ Query Sanitizer │ ASH Sampler      │ PgBouncer Collector    │  │
│  └─────────────────┴──────────────────┴─────────────────────────┘  │
└─────────────────────────────────┬───────────────────────────────────┘
                                  │
                          Unified Metrics
                                  │
        ┌─────────────────────────┴─────────────────────────┐
        │                                                   │
┌───────▼────────┐                                 ┌────────▼────────┐
│  NRI Adapter   │                                 │  OTLP Adapter   │
│                │                                 │                 │
│ JSON (stdout)  │                                 │ HTTP/Protobuf   │
└────────────────┘                                 └─────────────────┘
        │                                                   │
        ▼                                                   ▼
New Relic Infrastructure                           OpenTelemetry Collector
```

## Core Components

### 1. Collection Engine (`src/collection_engine.rs`)

The heart of the system, responsible for:
- Managing database connections with connection pooling
- Orchestrating metric collection
- Handling capability detection
- Managing adapters for different output formats

Key features:
- **Asynchronous collection** using Tokio
- **Connection pooling** with SQLx
- **Error isolation** - failures in one metric type don't affect others
- **Dynamic dispatch** for heterogeneous adapter storage

### 2. Query Engine (`crates/query-engine/`)

Executes OHI-compatible queries with version awareness:
- **Version-specific queries** for PostgreSQL 12, 13, 14+
- **RDS compatibility** with special handling
- **Efficient batching** to minimize database load
- **Response time filtering** at query level

### 3. Output Adapters

#### NRI Adapter (`crates/nri-adapter/`)
- Outputs JSON to stdout for Infrastructure agent consumption
- Maintains 100% compatibility with nri-postgresql format
- Implements New Relic's integration protocol v4

#### OTLP Adapter (`crates/otel-adapter/`)
- Sends metrics via HTTP/Protobuf to OpenTelemetry collectors
- Supports compression and custom headers
- Maps PostgreSQL metrics to OTLP metric types

### 4. Extension Manager (`crates/extensions/`)

Handles PostgreSQL extension detection and configuration:
- **pg_stat_statements**: Required for query metrics
- **pg_wait_sampling**: Enhanced wait event data
- **pg_stat_monitor**: Alternative to pg_stat_statements
- **Graceful degradation** when extensions are missing

### 5. Advanced Features

#### Active Session History (ASH)
- Oracle-style session sampling
- Memory-bounded with automatic eviction
- Configurable retention and sampling intervals
- Rich session state tracking

#### PgBouncer Integration
- Monitors connection pooler statistics
- Pool, database, client, and server metrics
- Automatic admin connection handling

#### Multi-Instance Support
- Single collector monitors multiple databases
- Parallel collection with error isolation
- Instance-specific configuration overrides

#### Query Sanitization
- Three modes: Full, Smart, None
- Detects: emails, SSNs, credit cards, phone numbers, IPs
- Preserves query structure while removing sensitive data

## Data Flow

### 1. Collection Phase
```rust
// Simplified collection flow
let metrics = UnifiedMetrics {
    slow_queries: query_executor.collect_slow_queries().await?,
    wait_events: query_executor.collect_wait_events().await?,
    blocking_sessions: query_executor.collect_blocking_sessions().await?,
    // ... other metric types
};
```

### 2. Processing Phase
- Query text sanitization
- Response time threshold filtering
- Metric enrichment with metadata

### 3. Output Phase
```rust
// Dynamic adapter dispatch
for adapter in &self.adapters {
    let output = adapter.adapt_dyn(&metrics).await?;
    let data = output.serialize()?;
    
    match adapter.name() {
        "NRI" => println!("{}", String::from_utf8_lossy(&data)),
        "OpenTelemetry" => self.exporter.export_http(...).await?,
        _ => warn!("Unknown adapter"),
    }
}
```

## Key Design Decisions

### 1. Rust Language Choice
- **Memory safety** without garbage collection
- **Performance** comparable to C/C++
- **Strong typing** catches errors at compile time
- **Async/await** for efficient I/O

### 2. Unified Metrics Model
- Single internal representation for all metrics
- Adapters transform to output-specific formats
- Enables easy addition of new output formats

### 3. Dynamic Dispatch for Adapters
```rust
// Trait for heterogeneous storage
pub trait MetricAdapterDyn: Send + Sync {
    async fn adapt_dyn(&self, metrics: &UnifiedMetrics) 
        -> Result<Box<dyn MetricOutputDyn>, ProcessError>;
    fn name(&self) -> &str;
}
```

### 4. Configuration Flexibility
- TOML configuration files
- Environment variable overrides
- Runtime mode switching

## Performance Optimizations

### 1. Connection Pooling
- Reuses database connections
- Configurable pool size
- Automatic connection health checks

### 2. Efficient Queries
- Batched collection
- Index-aware query construction
- Minimal data transfer

### 3. Memory Management
- Bounded collections (ASH sampling)
- Streaming large result sets
- Automatic cleanup

### 4. Parallel Collection
- Multi-instance metrics collected concurrently
- Independent error handling per instance

## Error Handling

### 1. Graceful Degradation
- Missing extensions don't crash collector
- Individual metric failures isolated
- Partial results still sent

### 2. Health Monitoring
- `/health` endpoint with collection status
- `/ready` for Kubernetes readiness
- `/metrics` for Prometheus scraping

### 3. Circuit Breaker
- Prevents cascading failures
- Automatic recovery with backoff
- Configurable thresholds

## Security Considerations

### 1. Query Sanitization
- Automatic PII removal
- Configurable sensitivity levels
- Audit trail for sanitized queries

### 2. Least Privilege
- Uses pg_monitor role
- No superuser required
- Read-only operations

### 3. Credential Management
- Environment variable support
- Kubernetes secrets integration
- No hardcoded credentials

## Testing Strategy

### 1. Unit Tests
- Component isolation
- Mock database connections
- Property-based testing

### 2. Integration Tests
- Real PostgreSQL instances
- Extension compatibility
- Output format validation

### 3. End-to-End Tests
- Full collection pipeline
- Metric verification
- Performance benchmarks

## Future Enhancements

### 1. Additional Output Formats
- Prometheus exposition format
- StatsD protocol
- Custom webhooks

### 2. Advanced Analytics
- Anomaly detection
- Trend analysis
- Predictive alerts

### 3. Enhanced eBPF Integration
- Kernel-level metrics
- Zero-overhead profiling
- Network traffic analysis