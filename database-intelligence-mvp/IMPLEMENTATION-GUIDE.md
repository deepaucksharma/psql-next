# Database Intelligence MVP - Implementation Guide

## Current State Overview

This guide clarifies the relationship between documented features and actual implementations. All code and configurations are preserved, with clear indicators of their production readiness.

## Implementation Tiers

### Tier 1: Production Ready (v1.0.0) ✅

These components are actively used in the current deployment:

| Component | Type | Location | Status |
|-----------|------|----------|--------|
| Standard SQL Receiver | OTEL Native | `config/collector.yaml` | ✅ Production |
| Memory Limiter | OTEL Native | `config/collector.yaml` | ✅ Production |
| Transform Processor | OTEL Native | `config/collector.yaml` | ✅ Production |
| Probabilistic Sampler | OTEL Native | `config/collector.yaml` | ✅ Production |
| Batch Processor | OTEL Native | `config/collector.yaml` | ✅ Production |
| OTLP Exporter | OTEL Native | `config/collector.yaml` | ✅ Production |
| Leader Election | OTEL Extension | `deploy/k8s/ha-deployment.yaml` | ✅ Production |

### Tier 2: Experimental Components 🚧

These are fully implemented but not integrated into the production build:

| Component | Type | Location | Integration Status |
|-----------|------|----------|-------------------|
| PostgreSQL Query Receiver | Custom Go | `/receivers/postgresqlquery/` | 🚧 Requires build integration |
| Adaptive Sampler | Custom Go | `/processors/adaptivesampler/` | 🚧 Requires build integration |
| Circuit Breaker | Custom Go | `/processors/circuitbreaker/` | 🚧 Requires build integration |
| Plan Attribute Extractor | Custom Go | `/processors/planattributeextractor/` | 🚧 Requires build integration |
| Verification Processor | Custom Go | `/processors/verification/` | 🚧 Requires build integration |
| OTLP PostgreSQL Exporter | Custom Go | `/exporters/otlpexporter/` | 🚧 Requires build integration |

### Tier 3: Planned Features 📋

These are documented but not yet implemented:

| Feature | Documentation Claims | Implementation Plan |
|---------|---------------------|---------------------|
| Query Plan Collection | Safe EXPLAIN execution | Requires pg_querylens extension |
| ASH (Active Session History) | 1-second sampling | Implemented in postgresqlquery receiver |
| Plan Regression Detection | Automatic detection | Implemented in postgresqlquery receiver |
| Multi-Database Federation | Unified view | Requires state coordination |

## Architecture Reconciliation

### Production Architecture (What's Running Now)

```yaml
# Current Reality - Using Standard Components
Database → sqlquery → memory_limiter → transform → sampler → batch → otlp → New Relic
           ↑
    Leader Election ensures single active collector
```

### Experimental Architecture (Future Vision)

```yaml
# Future State - With Custom Components
Database → postgresqlquery → circuitbreaker → adaptivesampler → verification → otlp → New Relic
           ↑                        ↑                ↑              ↑
    ASH Sampling           Per-DB Isolation   Smart Sampling   Real-time Feedback
```

## Configuration Reference

### Production Configuration

**File**: `config/collector.yaml`

```yaml
# This is what actually runs in production
receivers:
  sqlquery/postgresql:
    driver: postgres
    dsn: "${env:PG_REPLICA_DSN}"
    collection_interval: 300s
    enabled_when_leader: true  # HA support
    queries:
      - sql: |
          -- Returns metadata only, not plans
          SELECT query_id, query_text, avg_duration_ms...
```

**Note**: The `plan_metadata` field returns a static JSON indicating plans are not available.

### Experimental Configurations

**Location**: Various files in `/receivers/`, `/processors/`, `/exporters/`

These components are production-quality code but require:
1. A custom collector build process
2. Integration testing
3. Performance validation

## Deployment Patterns

### Current Production Pattern

```yaml
# HA Deployment with Leader Election
apiVersion: apps/v1
kind: Deployment
metadata:
  name: db-intelligence-collector
spec:
  replicas: 3  # Safe with leader election
```

### Experimental Patterns

```yaml
# Single Instance (for custom processors with state)
spec:
  replicas: 1  # Required if using stateful processors
```

## Building Custom Components

**Note**: This section is for future development. Production deployments use pre-built images.

### Prerequisites

```bash
# Install builder
GO111MODULE=on go install go.opentelemetry.io/collector/cmd/builder@latest

# Create builder config
cat > otelcol-builder.yaml <<EOF
dist:
  name: otelcol-custom
  description: Custom OTel Collector with Database Intelligence components
  output_path: ./dist

receivers:
  - gomod: github.com/database-intelligence-mvp/receivers/postgresqlquery v0.0.0
    path: ./receivers/postgresqlquery

processors:
  - gomod: github.com/database-intelligence-mvp/processors/adaptivesampler v0.0.0
    path: ./processors/adaptivesampler
  - gomod: github.com/database-intelligence-mvp/processors/circuitbreaker v0.0.0
    path: ./processors/circuitbreaker

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.96.0
EOF

# Build custom collector
builder --config=otelcol-builder.yaml
```

## Migration Paths

### From Standard to Custom Components

1. **Current State**: Standard OTEL components
2. **Phase 1**: Add circuit breaker (most stable custom component)
3. **Phase 2**: Replace probabilistic with adaptive sampler
4. **Phase 3**: Switch to postgresqlquery receiver
5. **Phase 4**: Enable verification processor

### From Metadata to Plans

1. **Current State**: Query metadata only
2. **Phase 1**: Deploy pg_querylens extension
3. **Phase 2**: Enable plan collection for top query
4. **Phase 3**: Expand to top-N queries
5. **Phase 4**: Full plan analysis with regression detection

## Performance Characteristics

### Production (Standard Components)

| Metric | Value | Notes |
|--------|-------|-------|
| CPU Usage | 100-300m | Per instance |
| Memory Usage | 256-512Mi | With memory_limiter |
| Network | <1Mbps | With 25% sampling |
| Query Overhead | <0.1% | Single query per interval |
| Instances | 3 | With leader election |

### Experimental (Custom Components)

| Metric | Expected Value | Notes |
|--------|----------------|-------|
| CPU Usage | 200-500m | Additional processing |
| Memory Usage | 512Mi-1Gi | State management |
| Network | 1-5Mbps | Richer telemetry |
| Query Overhead | 0.1-0.5% | Multiple queries, EXPLAIN |
| Instances | 1 | Until state coordination |

## Known Limitations

### Production Limitations

1. **No Query Plans**: Returns placeholder JSON
2. **Basic Sampling**: Probabilistic only (25% default)
3. **Limited Correlation**: No automatic APM linkage

### Experimental Component Limitations

1. **Build Integration**: Requires custom collector build
2. **State Management**: Some processors require single instance
3. **Testing Coverage**: Limited production validation

## Verification Checklist

### For Production Deployment

- [ ] Using `config/collector.yaml` (not experimental configs)
- [ ] Leader election enabled for HA
- [ ] PG_REPLICA_DSN points to read replica
- [ ] Memory limits configured
- [ ] Sampling percentage appropriate for volume

### For Experimental Features

- [ ] Custom collector built successfully
- [ ] Single instance deployment (if using stateful processors)
- [ ] Monitoring for new metrics
- [ ] Gradual rollout plan
- [ ] Rollback procedure documented

## Support Matrix

| Feature | Production Support | Experimental Support |
|---------|-------------------|---------------------|
| PostgreSQL Metadata | ✅ Full | ✅ Enhanced |
| MySQL Metadata | ✅ Full | ⚠️ Limited |
| Query Plans | ❌ Not Collected | 🚧 In Development |
| ASH Sampling | ❌ Not Available | 🚧 Implemented |
| Circuit Breaker | ❌ Not Active | 🚧 Ready to Test |
| Adaptive Sampling | ❌ Using Probabilistic | 🚧 Ready to Test |

## Next Steps

### For Production Users

1. Deploy using standard configuration
2. Monitor with provided dashboards
3. Adjust sampling based on volume
4. Wait for official custom component release

### For Early Adopters

1. Review experimental component code
2. Set up custom build pipeline
3. Test in non-production environment
4. Provide feedback via GitHub issues

## FAQ

**Q: Why aren't custom components in production?**
A: They require additional testing and a custom build process. The standard components provide a stable foundation.

**Q: When will query plans be collected?**
A: When the pg_querylens extension is completed and safe EXPLAIN execution is validated.

**Q: Can I use multiple instances with custom components?**
A: Only with stateless processors. Stateful processors (like adaptive sampler) require state coordination.

**Q: Is the experimental code production-quality?**
A: Yes, the code quality is high, but integration and operational characteristics need validation.

## Important Notes

- 📌 **All code is preserved** - Nothing has been removed
- 📌 **Documentation now matches reality** - Clear tier system
- 📌 **Upgrade path is defined** - From standard to custom components
- 📌 **Both approaches are valid** - Standard for stability, custom for features

This guide will be updated as components move from experimental to production tiers.