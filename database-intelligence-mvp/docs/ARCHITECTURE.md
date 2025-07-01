# Database Intelligence Collector - Architecture Guide

Comprehensive architecture documentation covering system design, implementation details, and operational considerations for the Database Intelligence MVP.

## Table of Contents

1. [System Overview](#system-overview)
2. [Architecture Principles](#architecture-principles)
3. [Component Architecture](#component-architecture)
4. [Data Flow](#data-flow)
5. [Custom Processors Implementation](#custom-processors-implementation)
6. [Technical Implementation Details](#technical-implementation-details)
7. [State Management](#state-management)
8. [Performance Architecture](#performance-architecture)
9. [Security Architecture](#security-architecture)
10. [Deployment Architecture](#deployment-architecture)
11. [Integration Architecture](#integration-architecture)
12. [Project Structure](#project-structure)
13. [Future Considerations](#future-considerations)

## System Overview

The Database Intelligence Collector is an OpenTelemetry-based monitoring solution enhanced with 7 sophisticated custom processors (>5,000 lines of production code). It follows an OTEL-first architecture, using standard components where possible and custom processors for database-specific features, enterprise requirements, and OHI migration support.

### Key Characteristics
- **OTEL-First**: Leverages standard OpenTelemetry components
- **Intelligent Processing**: 7 custom processors for sampling, protection, enrichment, compliance, cost control, and correlation
- **OHI Compatible**: Full metric parity with New Relic On-Host Integration
- **Enterprise Ready**: Cost control, error monitoring, and security features
- **Resilient Design**: Graceful degradation with circuit breakers
- **Zero-Persistence**: In-memory state for operational simplicity
- **Production-Ready**: Comprehensive E2E testing and validation

## Architecture Principles

### 1. OTEL-First Design
Use standard OpenTelemetry components wherever possible:
```yaml
# Standard components preferred
receivers: [postgresql, mysql, sqlquery]
processors: [memory_limiter, batch, transform, resource]
exporters: [otlp, prometheus, debug]

# Custom processors for database intelligence and enterprise features
processors: [memory_limiter, adaptive_sampler, circuit_breaker, 
            plan_extractor, verification, cost_control,
            nr_error_monitor, query_correlator, metric_transform, batch]
```

### 2. Graceful Degradation
```mermaid
graph LR
    A[Full Pipeline] --> B[Reduced Sampling]
    B --> C[Basic Processing]
    C --> D[Emergency Mode]
    
    A -.->|Processor Failure| B
    B -.->|Resource Pressure| C
    C -.->|System Overload| D
```

### 3. Modular Design Principles
- **Independent**: Each processor operates independently
- **Resilient**: Graceful degradation on component failure
- **Efficient**: Minimal resource usage with optimization
- **Observable**: Self-telemetry and comprehensive monitoring

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
            CC[Cost Control]
            NM[NR Error Monitor]
            QC[Query Correlator]
            MT[Metric Transform]
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
    
    ML --> MT
    MT --> AS
    AS --> CB
    CB --> PE
    PE --> VE
    VE --> QC
    QC --> CC
    CC --> NM
    NM --> BA
    
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
    class ML,AS,CB,PE,VE,CC,NM,QC,MT,BA processor
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
Metrics → Memory Limiter → Metric Transform → Sampling → Circuit Breaking → 
          Enrichment → Verification → Correlation → Cost Control → 
          Error Monitoring → Batching
```
- Each processor operates independently
- Failures in one processor don't affect others
- State is maintained in-memory only
- OHI compatibility transformations applied early

### 3. Export Phase
```
Batched Metrics → Exporters → Destinations
```
- Multiple exporters can run simultaneously
- Retry logic with exponential backoff
- Compression for efficient transmission

## Custom Processors Implementation

### Core Database Processors

#### 1. Adaptive Sampler (576 lines)

**Purpose**: Intelligent sampling based on configurable rules

**Architecture**:
```go
type adaptiveSamplerProcessor struct {
    cfg                Config
    logger             *zap.Logger
    deduplicationCache *lru.Cache[string, time.Time]
    ruleLimiters       map[string]*rateLimiter
    stateMutex         sync.RWMutex
}
```

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

#### 2. Circuit Breaker (922 lines)

**Purpose**: Protect databases from overload

**Architecture**:
```go
type circuitBreakerProcessor struct {
    config         Config
    logger         *zap.Logger
    circuits       map[string]*circuit
    mutex          sync.RWMutex
    healthChecker  *healthChecker
}
```

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

#### 3. Plan Attribute Extractor (391 lines)

**Purpose**: Extract and analyze query execution plans

**Architecture**:
```go
type planAttributeExtractor struct {
    config        Config
    logger        *zap.Logger
    parserCache   *lru.Cache[string, *ParsedPlan]
    parserPool    sync.Pool
    metrics       *extractorMetrics
}
```

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

#### 4. Verification Processor (1,353 lines)

**Purpose**: Data quality and compliance

**Architecture**:
```go
type verificationProcessor struct {
    config          Config
    logger          *zap.Logger
    piiDetector     *piiDetector
    qualityChecker  *qualityChecker
    autoTuner       *autoTuner
    metrics         *verificationMetrics
}
```

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

### Enterprise Processors

#### 5. Cost Control Processor (892 lines)

**Purpose**: Manage telemetry costs through intelligent data reduction

**Architecture**:
```go
type costControlProcessor struct {
    config            *Config
    logger            *zap.Logger
    costTracker       *costTracker
    metricCardinality map[string]*cardinalityTracker
    nextTraces        consumer.Traces
    nextMetrics       consumer.Metrics
    nextLogs          consumer.Logs
}
```

**Key Features**:
- Monthly budget enforcement
- Cardinality reduction
- Aggressive mode when over budget
- Data Plus pricing support

#### 6. NR Error Monitor (654 lines)

**Purpose**: Proactively detect patterns that cause NrIntegrationError

**Architecture**:
```go
type nrErrorMonitor struct {
    config       *Config
    logger       *zap.Logger
    errorCounts  map[string]*errorTracker
    nextConsumer consumer.Metrics
}
```

**Key Features**:
- Attribute length validation
- Cardinality monitoring
- Semantic convention checks
- Alert generation

### Migration Processors

#### 7. Query Correlator (450 lines)

**Purpose**: Correlate queries with database and table metrics

**Architecture**:
```go
type queryCorrelator struct {
    config        *Config
    logger        *zap.Logger
    queryIndex    map[string]*queryInfo
    tableIndex    map[string]*tableInfo
    databaseIndex map[string]*databaseInfo
}
```

**Key Features**:
- Query-to-table correlation
- Performance categorization
- Load contribution calculation
- Maintenance indicators

## Technical Implementation Details

### Main Entry Point

```go
package main

import (
    "go.opentelemetry.io/collector/component"
    "go.opentelemetry.io/collector/otelcol"
    
    // Core database processors
    "github.com/database-intelligence-mvp/processors/adaptivesampler"
    "github.com/database-intelligence-mvp/processors/circuitbreaker"
    "github.com/database-intelligence-mvp/processors/planattributeextractor"
    "github.com/database-intelligence-mvp/processors/verification"
    
    // Enterprise processors
    "github.com/database-intelligence-mvp/processors/costcontrol"
    "github.com/database-intelligence-mvp/processors/nrerrormonitor"
    "github.com/database-intelligence-mvp/processors/querycorrelator"
)

func components() (otelcol.Factories, error) {
    factories.Processors, err = component.MakeProcessorFactoryMap(
        // Core processors
        adaptivesampler.NewFactory(),
        circuitbreaker.NewFactory(),
        planattributeextractor.NewFactory(),
        verification.NewFactory(),
        
        // Enterprise processors
        costcontrol.NewFactory(),
        nrerrormonitor.NewFactory(),
        querycorrelator.NewFactory(),
    )
    return factories, err
}
```

### Factory Pattern Implementation

Each processor follows the OpenTelemetry factory pattern:

```go
const (
    TypeStr = "adaptive_sampler"  // Must match config
)

func NewFactory() processor.Factory {
    return processor.NewFactory(
        TypeStr,
        createDefaultConfig,
        processor.WithMetrics(createMetricsProcessor, stability.StabilityLevelBeta),
    )
}
```

### Processor Implementation Pattern

```go
type baseProcessor struct {
    config       component.Config
    logger       *zap.Logger
    nextConsumer consumer.Metrics
    metrics      *processorMetrics
    
    // Processor-specific state
    stateMutex   sync.RWMutex
}

func (p *baseProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
    // Pre-processing checks
    if err := p.validateInput(md); err != nil {
        return err
    }
    
    // Process metrics
    processed, err := p.processMetrics(ctx, md)
    if err != nil {
        p.metrics.recordError(err)
        return err
    }
    
    // Pass to next consumer
    return p.nextConsumer.ConsumeMetrics(ctx, processed)
}
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

## Performance Architecture

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

### Memory Usage Breakdown
```
Base Collector:          ~50MB
Adaptive Sampler:        ~50-100MB (cache dependent)
Circuit Breaker:         ~10MB (minimal state)
Plan Extractor:          ~50MB (parser cache)
Verification:            ~30MB (pattern engine)
Batch Processor:         ~50MB (queue buffer)
---
Total:                   ~240-340MB typical
```

### CPU Usage Profile
```
Metric Reception:        5-10%
Adaptive Sampling:       10-15% (rule evaluation)
Circuit Breaking:        <1% (state checks)
Plan Extraction:         15-20% (JSON parsing)
Verification:            10-15% (regex matching)
Export/Serialization:    10-15%
---
Total:                   50-75% of 1 core typical
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

## Integration Architecture

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

## Project Structure

```
database-intelligence-mvp/
├── main.go                          # Entry point with component registration
├── go.mod                           # Module definition
├── ocb-config.yaml                  # OpenTelemetry Collector Builder config
├── otelcol-builder.yaml             # Alternative builder configuration
│
├── processors/                      # Custom processors (3,242 lines total)
│   ├── adaptivesampler/            # Intelligent sampling (576 lines)
│   │   ├── processor.go            # Main processor implementation
│   │   ├── config.go               # Configuration structures
│   │   ├── factory.go              # Factory pattern implementation
│   │   ├── rules.go                # Rule evaluation engine
│   │   └── metrics.go              # Processor metrics
│   │
│   ├── circuitbreaker/             # Database protection (922 lines)
│   │   ├── processor.go            # Circuit breaker state machine
│   │   ├── config.go               # Configuration with per-DB settings
│   │   ├── factory.go              # Factory with health checker
│   │   ├── circuit.go              # Individual circuit implementation
│   │   └── health.go               # Health checking logic
│   │
│   ├── planattributeextractor/     # Query plan parsing (391 lines)
│   │   ├── processor.go            # Plan extraction logic
│   │   ├── config.go               # Parser configuration
│   │   ├── factory.go              # Factory with parser pool
│   │   ├── parser.go               # PostgreSQL/MySQL parsers
│   │   └── cache.go                # Plan caching implementation
│   │
│   └── verification/               # Data quality & PII (1,353 lines)
│       ├── processor.go            # Main verification logic
│       ├── config.go               # PII patterns configuration
│       ├── factory.go              # Factory with detector setup
│       ├── pii_detector.go         # Pattern matching engine
│       ├── quality_checker.go      # Data quality validation
│       └── auto_tuner.go           # ML-based tuning
│
├── internal/                        # Internal packages
│   ├── health/                     # Health monitoring system
│   │   ├── checker.go              # Component health checks
│   │   └── server.go               # Health HTTP server
│   │
│   ├── ratelimit/                  # Rate limiting implementation
│   │   ├── limiter.go              # Token bucket implementation
│   │   └── adaptive.go             # Adaptive rate adjustment
│   │
│   └── performance/                # Performance optimization
│       ├── optimizer.go            # Memory/CPU optimization
│       └── pool.go                 # Object pooling
│
├── config/                         # Configuration files
│   ├── collector-production.yaml   # Production configuration
│   ├── collector-resilient.yaml    # Resilient single-instance config
│   └── pii-detection-enhanced.yaml # Enhanced PII patterns
│
└── scripts/                        # Operational scripts
    ├── generate-config.sh          # Configuration generator
    └── validate-env.sh             # Environment validator
```

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

## Future Considerations

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

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025