# Database Intelligence Collector - Architecture Overview

## System Architecture

The Database Intelligence Collector is built on OpenTelemetry's extensible architecture, enhanced with custom processors for database-specific monitoring needs.

### Design Principles

1. **OTEL-First**: Use standard components where possible
2. **Modular**: Each processor is independent and optional
3. **Resilient**: Graceful degradation on component failure
4. **Efficient**: Minimal resource usage with optimization
5. **Observable**: Self-telemetry and comprehensive monitoring

## Component Architecture

```mermaid
graph TB
    subgraph "Data Sources"
        PG[(PostgreSQL)]
        MY[(MySQL)]
    end
    
    subgraph "OTEL Collector"
        subgraph "Receivers"
            PGR[PostgreSQL Receiver]
            MYR[MySQL Receiver]
            SQLr[SQL Query Receiver]
        end
        
        subgraph "Processors"
            ML[Memory Limiter]
            AS[Adaptive Sampler]
            CB[Circuit Breaker]
            PE[Plan Extractor]
            VE[Verification]
            BA[Batch]
        end
        
        subgraph "Exporters"
            OTLP[OTLP/New Relic]
            PROM[Prometheus]
            DEBUG[Debug]
        end
        
        subgraph "Internal Systems"
            HM[Health Monitor]
            RL[Rate Limiter]
            ST[State Manager]
            CA[Cache Manager]
        end
    end
    
    subgraph "Monitoring"
        HEALTH[/health Endpoints]
        METRICS[/metrics Endpoint]
        TRACES[/debug/tracez]
    end
    
    PG -->|metrics| PGR
    MY -->|metrics| MYR
    
    PGR --> ML
    MYR --> ML
    SQLr --> ML
    
    ML --> AS
    AS --> CB
    CB --> PE
    PE --> VE
    VE --> BA
    
    BA --> OTLP
    BA --> PROM
    BA --> DEBUG
    
    HM -.->|monitors| AS
    HM -.->|monitors| CB
    HM -.->|monitors| PE
    HM -.->|monitors| VE
    
    RL -.->|controls| PGR
    RL -.->|controls| MYR
    
    ST -.->|provides| AS
    ST -.->|provides| CB
    
    CA -.->|provides| AS
    CA -.->|provides| PE
    
    HM --> HEALTH
    PROM --> METRICS
    DEBUG --> TRACES
    
    classDef receiver fill:#e1f5fe,stroke:#01579b,stroke-width:2px
    classDef processor fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
    classDef exporter fill:#e8f5e9,stroke:#1b5e20,stroke-width:2px
    classDef internal fill:#fff3e0,stroke:#e65100,stroke-width:2px
    classDef monitoring fill:#fce4ec,stroke:#880e4f,stroke-width:2px
    
    class PGR,MYR,SQLr receiver
    class ML,AS,CB,PE,VE,BA processor
    class OTLP,PROM,DEBUG exporter
    class HM,RL,ST,CA internal
    class HEALTH,METRICS,TRACES monitoring
```

## Data Flow

### 1. Collection Phase
```
Database → Receiver → Initial Metrics
```
- Receivers connect to databases using native protocols
- Collect metrics at configured intervals (default: 30s)
- Transform to OTLP format

### 2. Processing Phase
```
Metrics → Memory Limiter → Sampling → Circuit Breaking → Enrichment → Verification → Batching
```
- Each processor operates independently
- Failures in one processor don't affect others
- State is maintained in-memory only

### 3. Export Phase
```
Batched Metrics → Exporters → Destinations
```
- Multiple exporters can run simultaneously
- Retry logic with exponential backoff
- Compression for efficient transmission

## Custom Processors Detail

### Adaptive Sampler (576 lines)
**Purpose**: Intelligent sampling based on configurable rules

**Key Features**:
- Expression-based rule evaluation
- LRU cache for deduplication
- Dynamic sampling rates
- In-memory state only

**Configuration**:
```yaml
adaptive_sampler:
  in_memory_only: true  # Always true in production
  rules:
    - name: slow_queries
      conditions:
        - attribute: duration_ms
          operator: gt
          value: 1000
      sample_rate: 1.0
  default_sample_rate: 0.1
```

### Circuit Breaker (922 lines)
**Purpose**: Protect databases from overload

**Key Features**:
- Per-database circuit breakers
- Three states: closed, open, half-open
- Automatic recovery with backoff
- Resource-based triggers

**State Transitions**:
```
Closed → (failures > threshold) → Open
Open → (timeout elapsed) → Half-Open
Half-Open → (success) → Closed
Half-Open → (failure) → Open
```

### Plan Attribute Extractor (391 lines)
**Purpose**: Extract and analyze query execution plans

**Key Features**:
- PostgreSQL EXPLAIN parsing
- MySQL execution plan support
- Plan hash generation
- Timeout protection

**Extracted Attributes**:
- Total cost
- Execution time
- Plan hash
- Operation types

### Verification Processor (1,353 lines)
**Purpose**: Data quality and compliance

**Key Features**:
- PII detection (SSN, CC, email, etc.)
- Data validation rules
- Auto-tuning capabilities
- Streaming processing

**PII Patterns**:
```regex
SSN: \b\d{3}-\d{2}-\d{4}\b
Credit Card: \b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b
Email: \b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b
```

## State Management

### In-Memory Architecture
All processors use in-memory state management:

```go
type ProcessorState struct {
    sync.RWMutex
    cache        *lru.Cache
    limiters     map[string]*rateLimiter
    circuits     map[string]*circuitBreaker
}
```

### State Characteristics
- **Volatile**: State is lost on restart
- **Bounded**: LRU eviction prevents unbounded growth
- **Concurrent**: Thread-safe with fine-grained locking
- **Efficient**: Minimal memory overhead

## Performance Optimization

### 1. Memory Management
- Object pooling for frequently allocated structures
- Bounded caches with LRU eviction
- Aggressive garbage collection tuning

### 2. Processing Optimization
- Parallel processing where applicable
- Early termination for filtered metrics
- Batch processing for efficiency

### 3. Caching Strategy
- Plan parsing results cached
- Deduplication cache for sampling
- Circuit breaker state cache

## Monitoring and Observability

### Health Monitoring
```json
GET /health
{
  "healthy": true,
  "components": {
    "adaptive_sampler": {"healthy": true, "cache_size": 8500},
    "circuit_breaker": {"healthy": true, "open_circuits": 0},
    "plan_extractor": {"healthy": true, "cache_hits": 0.85},
    "verification": {"healthy": true, "pii_detections": 42}
  }
}
```

### Metrics Exposed
```prometheus
# Processor metrics
otelcol_processor_accepted_metric_points
otelcol_processor_refused_metric_points
otelcol_processor_dropped_metric_points

# Custom metrics
adaptive_sampler_cache_hit_rate
circuit_breaker_state{database="...", state="..."}
plan_extractor_parse_duration_ms
verification_pii_detections_total
```

## Security Architecture

### Data Protection
1. **PII Detection**: Configurable patterns for sensitive data
2. **Data Sanitization**: Automatic removal of detected PII
3. **No Persistence**: No sensitive data written to disk
4. **Secure Transport**: TLS for all external connections

### Resource Protection
1. **Memory Limits**: Hard limits with backpressure
2. **Rate Limiting**: Per-database rate limits
3. **Circuit Breaking**: Automatic protection from overload
4. **Timeout Protection**: All operations have timeouts

## Deployment Architecture

### Single Instance Model
```
┌─────────────────────────────┐
│   OTEL Collector Instance   │
│                             │
│  ┌───────────────────────┐  │
│  │   Resource Limits     │  │
│  │   Memory: 512MB       │  │
│  │   CPU: 4 cores        │  │
│  └───────────────────────┘  │
│                             │
│  ┌───────────────────────┐  │
│  │   State (In-Memory)   │  │
│  │   • Sampling cache    │  │
│  │   • Circuit states    │  │
│  │   • Rate limiters     │  │
│  └───────────────────────┘  │
└─────────────────────────────┘
```

### High Availability Considerations
While the current architecture is single-instance:
1. **Fast Recovery**: 2-3 second startup time
2. **Stateless Design**: No critical state to recover
3. **Health Checks**: Quick detection of failures
4. **Simple Restart**: Systemd or container restart

## Integration Points

### Input Integration
- **PostgreSQL**: Native protocol, pg_stat views
- **MySQL**: Native protocol, information_schema
- **Custom Queries**: SQL query receiver for specific metrics

### Output Integration
- **OTLP**: Standard protocol for observability platforms
- **Prometheus**: Local metrics scraping
- **Debug**: Real-time debugging output

### Operational Integration
- **Kubernetes**: Liveness/readiness probes
- **Monitoring**: Prometheus metrics endpoint
- **Debugging**: zPages for trace analysis
- **Configuration**: Environment variable substitution

## Future Architecture Considerations

### Potential Enhancements
1. **Distributed State**: Redis/etcd for shared state
2. **Horizontal Scaling**: Multiple collector instances
3. **Advanced Analytics**: ML-based anomaly detection
4. **Event Streaming**: Kafka integration

### Maintaining Simplicity
The current architecture prioritizes:
- Operational simplicity over complexity
- Reliability over advanced features
- Performance over flexibility
- Clear failure modes over hidden complexity

---

**Architecture Version**: 1.0.0  
**Last Updated**: June 30, 2025