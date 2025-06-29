# Database Intelligence MVP - Implementation Guide

## Overview

This guide details the two implementation paths for the Database Intelligence MVP:
1.  **Standard Mode**: Production-ready using proven OpenTelemetry components.
2.  **Experimental Mode**: Advanced features using custom Go components.

## Implementation Tiers

### Tier 1: Production Ready (v1.0.0) ✅

Components actively used in current deployment:
*   Standard SQL Receiver, Memory Limiter, Transform Processor, Probabilistic Sampler, Batch Processor, OTLP Exporter, Leader Election.

### Tier 2: Experimental Components 🚧

Fully implemented but not integrated into production build (require custom build integration):
*   PostgreSQL Query Receiver, Adaptive Sampler, Circuit Breaker, Plan Attribute Extractor, Verification Processor, OTLP PostgreSQL Exporter.

### Tier 3: Planned Features 📋

Documented but not yet implemented:
*   Query Plan Collection (requires `pg_querylens`), ASH (Active Session History), Plan Regression Detection, Multi-Database Federation.

## Architecture Reconciliation

*   **Production Architecture (Current Reality)**: `Database → sqlquery → memory_limiter → transform → sampler → batch → otlp → New Relic` (Leader Election ensures single active collector).
*   **Experimental Architecture (Future Vision)**: `Database → postgresqlquery → circuitbreaker → adaptivesampler → verification → otlp → New Relic` (with ASH Sampling, Per-DB Isolation, Smart Sampling, Real-time Feedback).

## Configuration Reference

*   **Production Configuration**: `config/collector.yaml` (returns metadata only, not plans).
*   **Experimental Configurations**: Various files in `/receivers/`, `/processors/`, `/exporters/` (production-quality code, but require custom build, integration testing, performance validation).

## Deployment Patterns

*   **Current Production Pattern**: HA Deployment with Leader Election (`replicas: 3`).
*   **Experimental Patterns**: Single Instance (for custom processors with state, `replicas: 1`).

## Building Custom Components

(Note: For future development; production deployments use pre-built images.)

**Prerequisites**: Install `builder` (`go install go.opentelemetry.io/collector/cmd/builder@latest`).

**Build Process**:
```bash
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
builder --config=otelcol-builder.yaml
```

## Choosing Your Implementation

*   **Standard Mode**: For immediate production deployment, stability, minimal operational complexity.
*   **Experimental Mode**: For Active Session History (ASH), query-aware sampling, automatic database protection, and single-instance deployment management.
*   **Hybrid Approach**: Run both side-by-side, or start Standard and add Experimental later.

## Performance Characteristics

| Metric | Production (Standard) | Experimental (Custom) |
|---|---|---|
| CPU Usage | 100-300m | 200-500m |
| Memory Usage | 256-512Mi | 512Mi-1Gi |
| Network | <1Mbps | 1-5Mbps |
| Query Overhead | <0.1% | 0.1-0.5% |
| Instances | 3 | 1 (until state coordination) |

## Known Limitations

*   **Production**: No Query Plans (returns placeholder JSON), Basic Sampling (probabilistic only), Limited Correlation.
*   **Experimental**: Requires custom collector build, some processors require single instance, limited production validation.

## Verification Checklist

*   **Production**: Using `config/collector.yaml`, leader election enabled, DSN points to read replica, memory limits configured, sampling percentage appropriate.
*   **Experimental**: Custom collector built, single instance deployment (if stateful), monitoring for new metrics, gradual rollout plan, rollback procedure documented.

## Support Matrix

| Feature | Production Support | Experimental Support |
|---|---|---|
| PostgreSQL Metadata | ✅ Full | ✅ Enhanced |
| MySQL Metadata | ✅ Full | ⚠️ Limited |
| Query Plans | ❌ Not Collected | 🚧 In Development |
| ASH Sampling | ❌ Not Available | 🚧 Implemented |
| Circuit Breaker | ❌ Not Active | 🚧 Ready to Test |
| Adaptive Sampling | ❌ Using Probabilistic | 🚧 Ready to Test |

## Next Steps

*   **Production Users**: Deploy standard config, monitor, adjust sampling, await custom component release.
*   **Early Adopters**: Review experimental code, set up custom build, test in non-production, provide feedback.

## Important Notes

*   All code is preserved.
*   Documentation now matches reality (clear tier system).
*   Upgrade path is defined (from standard to custom components).
*   Both approaches are valid (Standard for stability, Custom for features).
