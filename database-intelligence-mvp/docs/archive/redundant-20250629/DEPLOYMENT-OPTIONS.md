# Deployment Options

The Database Intelligence MVP offers two deployment modes: Standard and Experimental.

## Quick Decision Guide

*   **Choose Standard Mode if**: You need production-ready stability, simple components, minimal configuration, and standard monitoring needs.
*   **Choose Experimental Mode if**: You need advanced features like ASH sampling, intelligent adaptive sampling, circuit breaker protection, and can accept higher resource usage.

## Standard Mode (Production-Ready)

Uses battle-tested OpenTelemetry components.

### Features
*   Query metadata collection, performance metrics from `pg_stat_statements`.
*   High availability with leader election.
*   PII sanitization, 25% probabilistic sampling.
*   Low resource usage (512MB RAM).

### Quick Start
```bash
./quickstart.sh all
```

### Architecture
```
Database → SQL Query Receiver → Memory Limiter → Transform → Sampler → Batch → New Relic
```

### Resource Requirements
*   Memory: 256-512MB
*   CPU: 100-300m
*   Instances: 3 (with HA)

## Experimental Mode (Advanced Features)

Includes custom Go components with advanced capabilities.

### Features
*   All Standard Mode features, plus:
*   Active Session History (1-second samples).
*   Adaptive sampling based on query cost.
*   Circuit breaker for database protection.
*   Multi-database federation, plan analysis readiness, cloud provider optimization.

### Quick Start
```bash
./quickstart.sh --experimental all
```

### Architecture
```
Database → PostgreSQL Query Receiver → Circuit Breaker → Adaptive Sampler → Verification → New Relic
                    ↓
              ASH Sampling
```

### Resource Requirements
*   Memory: 1-2GB
*   CPU: 500m-1000m
*   Instances: 1 (stateful components)

## Feature Comparison

| Feature | Standard | Experimental |
|---|---|---|
| Query Metadata | ✅ | ✅ |
| Performance Metrics | ✅ | ✅ |
| High Availability | ✅ | ⚠️ Single Instance |
| ASH Sampling | ❌ | ✅ 1-second |
| Adaptive Sampling | ❌ | ✅ Query-aware |
| Circuit Breaker | ❌ | ✅ Auto-protection |
| Multi-Database | ⚠️ Manual | ✅ Federated |
| Resource Usage | Low | Medium-High |
| Build Required | No | Yes |

## Deployment Scenarios

*   **Production Database Monitoring**: Standard Mode (`./quickstart.sh all`).
*   **Development Environment**: Experimental Mode (`./quickstart.sh --experimental all`).
*   **Performance Troubleshooting**: Experimental Mode with ASH (`./quickstart.sh --experimental build` then `./quickstart.sh --experimental start`).
*   **Large Database Fleet**: Start Standard, then add Experimental to a subset.

## Switching Between Modes

*   **Standard to Experimental**: `./quickstart.sh --experimental build`, `./quickstart.sh stop`, `./quickstart.sh --experimental start`.
*   **Experimental to Standard**: `./quickstart.sh --experimental stop`, `./quickstart.sh start`.

## Monitoring Your Deployment

*   **Standard Mode**: Health (`curl http://localhost:13133/`), Metrics (`curl http://localhost:8888/metrics`).
*   **Experimental Mode**: Health (`curl http://localhost:13134/`), Metrics (`curl http://localhost:8889/metrics`), Grafana (`open http://localhost:3001`).

## Best Practices

*   **Standard Mode**: Deploy with 3 replicas for HA, use read replicas only, monitor resource usage, adjust sampling rate.
*   **Experimental Mode**: Start with single database, monitor circuit breaker, tune adaptive sampling, watch memory usage.

## FAQ

*   **Which mode to start with?**: Standard, unless experimental features are specifically needed.
*   **Can I run both simultaneously?**: Yes, they use different ports.
*   **Is experimental mode production-ready?**: Code quality is high, but needs more production testing.
*   **When will experimental features become standard?**: Based on community feedback and production validation.

## Next Steps

*   **Standard Mode**: Deploy to production, set up alerting, create dashboards, monitor performance.
*   **Experimental Mode**: Test in development, validate features, measure overhead, provide feedback.

## Getting Help

*   **Documentation**: See feature-specific guides.
*   **Issues**: Report bugs on GitHub.
*   **Discussion**: Ask questions in GitHub Discussions.
*   **Support**: Contact New Relic support (standard mode only).
