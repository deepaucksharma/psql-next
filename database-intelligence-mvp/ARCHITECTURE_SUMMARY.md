# Architecture Summary - Database Intelligence MVP

## Quick Reference Architecture

### System Components
```
┌─────────────────┐     ┌─────────────────┐
│   PostgreSQL    │     │     MySQL       │
│   (12+)         │     │     (8.0+)      │
└────────┬────────┘     └────────┬────────┘
         │                       │
         ▼                       ▼
┌─────────────────────────────────────────┐
│          RECEIVERS LAYER                 │
│  • PostgreSQL Receiver                  │
│  • MySQL Receiver                       │
│  • Enhanced SQL Receiver                │
└────────────────┬────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────┐
│         PROCESSORS PIPELINE              │
│ ┌─────────────────────────────────────┐ │
│ │ 1. Memory Limiter (75% limit)       │ │
│ ├─────────────────────────────────────┤ │
│ │ 2. Adaptive Sampler (rule-based)    │ │
│ ├─────────────────────────────────────┤ │
│ │ 3. Circuit Breaker (per-database)   │ │
│ ├─────────────────────────────────────┤ │
│ │ 4. Plan Attribute Extractor         │ │
│ ├─────────────────────────────────────┤ │
│ │ 5. Verification (PII detection)     │ │
│ ├─────────────────────────────────────┤ │
│ │ 6. Cost Control (budget enforce)    │ │
│ ├─────────────────────────────────────┤ │
│ │ 7. NR Error Monitor (validation)    │ │
│ ├─────────────────────────────────────┤ │
│ │ 8. Query Correlator (linking)       │ │
│ ├─────────────────────────────────────┤ │
│ │ 9. Batch Processor (1024 default)   │ │
│ └─────────────────────────────────────┘ │
└────────────────┬────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────┐
│          EXPORTERS LAYER                 │
│  • OTLP/HTTP → New Relic                │
│  • Prometheus Metrics                   │
│  • Debug (optional)                     │
└─────────────────────────────────────────┘
```

### Deployment Architecture

#### Development
```
Developer Machine
    │
    └── Docker Compose
         ├── PostgreSQL (test data)
         ├── MySQL (test data)
         ├── Collector (debug mode)
         ├── Prometheus
         └── Grafana
```

#### Production - Kubernetes
```
┌─────────────────────────────────────────┐
│           Kubernetes Cluster             │
│ ┌─────────────────────────────────────┐ │
│ │      Namespace: db-intelligence      │ │
│ │ ┌─────────────┐  ┌────────────────┐ │ │
│ │ │ Collector   │  │ Collector      │ │ │
│ │ │ Pod 1       │  │ Pod 2-10       │ │ │
│ │ │ (primary)   │  │ (replicas)     │ │ │
│ │ └─────────────┘  └────────────────┘ │ │
│ │         │               │            │ │
│ │         └───────┬───────┘            │ │
│ │                 ▼                    │ │
│ │          Load Balancer               │ │
│ │                 │                    │ │
│ └─────────────────┼────────────────────┘ │
│                   │                      │
└───────────────────┼──────────────────────┘
                    │
                    ▼
            ┌──────────────┐
            │  New Relic   │
            │    OTLP      │
            └──────────────┘
```

### Data Flow Patterns

#### Query Processing Flow
```
SQL Query → Receiver → Anonymization → Sampling → Analysis → Export
    │           │            │            │          │         │
    └───────────┴────────────┴────────────┴──────────┴─────────┴── Metrics
```

#### Error Handling Flow
```
Error Detection → Circuit Open → Backoff → Health Check → Recovery
       │               │            │           │             │
       └───────────────┴────────────┴───────────┴─────────────┴── State
```

### Key Architectural Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| State Management | In-memory only | Operational simplicity |
| Processing Model | Synchronous pipeline | Predictable behavior |
| Deployment | Container-native | Cloud portability |
| Integration | OTLP standard | Vendor neutrality |
| Security | Defense in depth | Multiple protection layers |
| Scaling | Horizontal (limited) | Stateless design |

### Performance Characteristics

| Metric | Value | Target |
|--------|-------|--------|
| Throughput | 10K queries/sec | 5K queries/sec |
| Latency | <5ms | <10ms |
| Memory | 256-512MB | <1GB |
| CPU | 15-30% | <50% |
| Availability | 99.5% | 99% |

### Security Layers

1. **Network**: mTLS, network policies
2. **Application**: Query anonymization, PII detection
3. **Container**: Non-root, read-only FS
4. **Kubernetes**: RBAC, pod security policies
5. **Data**: Encryption in transit

### Operational Touchpoints

#### Health Monitoring
- `/health` - Basic health check
- `/ready` - Readiness probe
- `:8888/metrics` - Prometheus metrics
- `:55679/debug/tracez` - Trace debugging

#### Configuration
- Environment variables (primary)
- ConfigMaps (Kubernetes)
- File-based (development)
- Overlays (environment-specific)

#### Troubleshooting
- Structured JSON logs
- Debug endpoints (pprof)
- Trace sampling
- Metric dashboards

### Technology Stack

| Layer | Technology | Version |
|-------|------------|---------|
| Language | Go | 1.21+ |
| Framework | OpenTelemetry | v0.129.0 |
| Container | Docker | 20.10+ |
| Orchestration | Kubernetes | 1.24+ |
| Monitoring | Prometheus | 2.x |
| Visualization | Grafana | 8.x |
| Databases | PostgreSQL/MySQL | 12+/8.0+ |

### Architectural Principles

1. **Simplicity First**: Avoid unnecessary complexity
2. **Fail Fast**: Early detection and rejection
3. **Graceful Degradation**: Partial failures don't cascade
4. **Observable by Default**: Comprehensive metrics
5. **Security in Depth**: Multiple protection layers
6. **Cloud Native**: Container-first design

### Future Architecture Evolution

#### Phase 1: Enhanced Reliability
- External state store (Redis)
- Persistent queuing (Kafka)
- Multi-region deployment

#### Phase 2: Advanced Features
- Streaming processing
- ML-based anomaly detection
- Predictive analytics

#### Phase 3: Platform Extension
- Plugin marketplace
- Multi-database support
- Custom processor SDK

---

This architecture balances production readiness with operational simplicity, providing a solid foundation for database observability while maintaining clear paths for future enhancement.