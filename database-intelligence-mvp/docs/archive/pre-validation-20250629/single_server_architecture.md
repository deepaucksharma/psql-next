# Single Server Architecture

## Overview

This document describes the simplified architecture optimized for single-server deployment, following DDD principles while removing unnecessary HA complexity.

## Key Design Decisions

### 1. Simplified State Management
- **In-Memory State**: Use in-memory repositories for all domain entities
- **File-Based Persistence**: Optional file-based persistence for critical state (e.g., circuit breaker states)
- **No External Dependencies**: Remove Redis, etcd, or other external state stores

### 2. Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    OpenTelemetry Collector                   │
├─────────────────────────────────────────────────────────────┤
│                         Receivers                            │
│  ┌─────────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ PostgreSQL      │  │ File Log     │  │ SQL Query    │  │
│  │ (Refactored)    │  │ Receiver     │  │ Receiver     │  │
│  └─────────────────┘  └──────────────┘  └──────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                         Processors                           │
│  ┌─────────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │ Circuit Breaker │  │ Adaptive     │  │ Transform    │  │
│  │ (Generic)       │  │ Sampler      │  │ Processor    │  │
│  └─────────────────┘  └──────────────┘  └──────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                         Exporters                            │
│  ┌─────────────────┐  ┌──────────────┐                     │
│  │ OTLP            │  │ File         │                     │
│  │ (Standard)      │  │ Exporter     │                     │
│  └─────────────────┘  └──────────────┘                     │
└─────────────────────────────────────────────────────────────┘
```

### 3. Domain Layer (Simplified)

```
domain/
├── shared/           # Shared kernel
├── database/        # Database bounded context
├── query/           # Query performance context  
└── telemetry/       # Telemetry context

infrastructure/
├── repositories/    # In-memory implementations
└── persistence/     # Optional file-based persistence

application/
├── services/        # Application services
└── collectors/      # Collector orchestration
```

### 4. Key Simplifications

#### Circuit Breaker
- In-memory state only
- Optional file persistence on shutdown
- No distributed coordination needed

#### Adaptive Sampler
- In-memory deduplication cache
- LRU eviction for memory management
- No external state synchronization

#### PostgreSQL Receiver
- Pure data ingestion only
- No internal processing
- Delegates all logic to processors

## Implementation Strategy

### Phase 1: Core Refactoring
1. Simplify existing components
2. Remove HA-specific code
3. Implement in-memory repositories

### Phase 2: Integration
1. Wire components using OCB
2. Create simplified configuration
3. Test end-to-end flow

### Phase 3: Optimization
1. Add file-based persistence where needed
2. Implement memory management
3. Performance tuning

## Configuration Example

```yaml
receivers:
  postgresqlquery/simple:
    databases:
      - name: "production"
        dsn: "${POSTGRES_DSN}"
    collection_interval: 10s
    enable_ash_sampling: true
    enable_wait_sampling: true

processors:
  circuitbreaker/simple:
    failure_threshold: 5
    timeout: 30s
    persistence:
      enabled: true
      path: "/var/lib/otel/circuit_states.json"
  
  adaptivesampler/simple:
    max_memory_mb: 100
    dedup_cache_size: 10000
    default_sampling_rate: 0.1
    strategies:
      - name: "errors"
        type: "always_sample"
        condition: "severity >= ERROR"

exporters:
  otlp:
    endpoint: "${OTLP_ENDPOINT}"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"

service:
  pipelines:
    metrics:
      receivers: [postgresqlquery/simple]
      processors: [circuitbreaker/simple, adaptivesampler/simple]
      exporters: [otlp]
```

## Benefits

1. **Simplicity**: Easier to deploy, manage, and debug
2. **Performance**: No network overhead for state management
3. **Reliability**: Fewer moving parts, fewer failure modes
4. **Resource Efficient**: Lower memory and CPU usage

## Limitations

1. **No HA**: Single point of failure
2. **State Loss**: In-memory state lost on restart (mitigated by file persistence)
3. **No Horizontal Scaling**: Cannot distribute load across multiple instances

## Migration Path

If HA is needed in the future:
1. Implement StateStore interfaces
2. Add Redis/etcd backends
3. Enable distributed mode via configuration
4. No changes to core business logic required