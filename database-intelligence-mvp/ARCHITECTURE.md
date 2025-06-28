# Technical Architecture Deep Dive

## Core Design Decisions

### The Single-Instance Constraint

**Decision**: Use file storage for processor state management

**Rationale**: Simplicity for MVP, avoiding external dependencies

**Critical Constraint**: This creates a fundamental scaling limitation. The collector MUST run as a single instance. Multiple instances will have inconsistent state, leading to:
- Duplicate data ingestion
- Inconsistent sampling behavior
- Unpredictable deduplication

**Future Path**: External state store (Redis/Memcached) is required before horizontal scaling.

### The Correlation Gap

**Reality**: Database queries and APM traces live in separate worlds. 

**Why It's Hard**:
1. Database connections are pooled - trace context is lost
2. SQL has no standard for context propagation
3. Requires changes to every database driver

**Interim Solution**: Manual correlation based on:
- Timestamp alignment
- Query fingerprint matching
- Duration correlation

**Long-term Solution**: Industry-wide effort to propagate trace context through SQL comments.

## Component Architecture

### Data Flow with Integrated Verification

```
Database → Receiver → Memory Limiter → PII Sanitizer → Attribute Extractor → 
                                                            ↓
                                                      Entity Synthesis
                                                            ↓
                                                    Circuit Breaker
                                                            ↓
                                                    VERIFICATION ← Real-time Feedback
                                                            ↓       ↓
                                                        Sampler     NR Monitoring
                                                            ↓
                                                         Batch → New Relic
                                                            ↑
            └────────────── File Storage (State Persistence) ───────────────────┘
```

### Verification Layer

**New Addition**: Integrated verification processor that provides real-time feedback on:

1. **Data Quality Verification**
   - Entity synthesis validation
   - Query normalization effectiveness
   - PII sanitization confirmation
   - Cardinality monitoring

2. **Integration Health Monitoring**
   - NrIntegrationError detection
   - Data freshness tracking
   - Circuit breaker state monitoring
   - Export success rates

3. **Feedback Mechanisms**
   - Real-time alerts via logs
   - Health report generation
   - Remediation suggestions
   - Metrics export for dashboards

### Receiver Layer

**Philosophy**: Configure standard receivers with safety constraints rather than building custom ones.

**Key Safety Mechanisms**:
1. Statement-level timeouts (PostgreSQL: `SET LOCAL`)
2. Query result limits (`LIMIT 1`)
3. Read-replica enforcement (connection string)
4. Circuit breaker patterns (receiver timeout)

### Processor Pipeline

**Order Matters**: The pipeline order is critical for safety and efficiency.

1. **memory_limiter**: First line of defense against OOM
2. **transform/sanitize_pii**: Security before processing
3. **plan_attribute_extractor**: Lightweight parsing only
4. **plan_context_enricher**: Heavy lifting (disabled in MVP)
5. **adaptive_sampler**: Intelligent data reduction
6. **batch**: Network efficiency

### State Management

**Current**: File storage with these characteristics:
- Survives collector restarts
- Requires persistent volume
- Single-instance only
- No cross-node coordination

**Implications**:
- Deploy as StatefulSet with `replicas: 1`
- Or DaemonSet (one per node)
- Never as Deployment with `replicas > 1`

## Security Architecture

### Defense in Depth

1. **Network**: Read-replica endpoints only
2. **Authentication**: Read-only database users
3. **Authorization**: Minimal required permissions
4. **Data**: PII sanitization before processing
5. **Transport**: TLS to New Relic

### PII Protection

**Sanitization Patterns**:
- Email addresses → `[EMAIL]`
- SSNs → `[SSN]`
- Credit cards → `[CARD]`
- Phone numbers → `[PHONE]`
- SQL literals → Removed entirely

**Hash-based Correlation**: Sanitized values are hashed deterministically for correlation without exposing data.