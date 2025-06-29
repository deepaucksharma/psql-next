# Experimental Features Guide

This guide documents the advanced, experimental features available in the Database Intelligence MVP. These features extend beyond standard OpenTelemetry components and offer specialized capabilities for database monitoring.

## Overview

The experimental mode provides advanced capabilities:

*   **Active Session History (ASH)**: 1-second granularity session monitoring.
*   **Adaptive Sampling**: Intelligent, query-aware sampling.
*   **Circuit Breaker**: Automatic database protection.
*   **Plan Analysis**: Query execution plan tracking (when `pg_querylens` available).
*   **Multi-Database Support**: Unified collection across database fleets.

## Quick Start

To utilize experimental features, you typically need to build a custom collector. Refer to the `CUSTOM_BUILD.md` for detailed instructions on building the custom collector.

1.  **Build Custom Collector**: `./quickstart.sh --experimental build` (installs OCB, compiles custom Go components, creates Docker image, runs integration tests).
2.  **Start in Experimental Mode**: `./quickstart.sh --experimental all` or `./quickstart.sh --experimental start`.
3.  **Monitor Experimental Features**: `./quickstart.sh --experimental status`, `./quickstart.sh --experimental logs`, `open http://localhost:3001` (Grafana, if configured).

## Feature Details

### Active Session History (ASH)

Provides second-by-second visibility into database activity (active sessions, wait events, blocking relationships, query context, resource consumption).

```yaml
receivers:
  postgresqlquery:
    ash_sampling:
      enabled: true
      interval: 1s
      buffer_size: 3600
```

### Adaptive Sampling

Intelligently adjusts sampling rates based on query characteristics (cost, error rate, volume). This processor is now considered Production Ready.

```yaml
processors:
  adaptivesampler:
    strategies:
      - type: "query_cost"
        high_cost_threshold_ms: 1000
        high_cost_sampling: 100
        low_cost_sampling: 25
```

### Circuit Breaker

Protects databases from monitoring overhead by opening the circuit on repeated failures, monitoring response times, and tracking memory pressure. This processor is now considered Production Ready.

```yaml
processors:
  circuitbreaker:
    failure_threshold: 5
    success_threshold: 2
    databases:
      default:
        max_error_rate: 0.1
        max_latency_ms: 5000
```

### Multi-Database Federation

Collects from multiple databases with unified configuration.

```yaml
receivers:
  postgresqlquery:
    databases:
      - name: primary
        dsn: "${env:PG_PRIMARY_DSN}"
        tags:
          role: primary
          region: us-east-1
```

## Configuration Examples

*   **Minimal Experimental Setup**: Refer to `config/experimental-minimal.yaml` (if available).
*   **Full Experimental Setup**: Refer to `config/collector-experimental.yaml` (if available).

## Monitoring Experimental Components

Key metrics include `db_intelligence_ash_samples_total`, `db_intelligence_circuitbreaker_open`, `db_intelligence_adaptivesampler_current_rate`, and `db_intelligence_plan_regressions_detected_total`.

## Gradual Adoption

It is recommended to adopt experimental features gradually:

*   **Phase 1: Circuit Breaker Only**.
*   **Phase 2: Add Adaptive Sampling**.
*   **Phase 3: Enable ASH**.

## Troubleshooting

*   **Custom Collector Won't Build**: Check Go version (1.21+), clear module cache, rebuild.
*   **High Memory Usage**: Reduce ASH buffer, tune memory limits (`GOMEMLIMIT`, `GOGC`).
*   **Circuit Breaker Too Sensitive**: Increase `failure_threshold`, longer `timeout`.

## Best Practices

1.  **Start Small**: Enable one experimental feature at a time.
2.  **Monitor Closely**: Watch resource usage during rollout.
3.  **Test First**: Use test databases before production.
4.  **Have Rollback Plan**: Keep standard configuration ready.
5.  **Document Changes**: Track which features are enabled.

## Future Enhancements

*   **Coming Soon**: Query plan collection (pending `pg_querylens`), Redis state storage for multi-instance deployment, MySQL ASH equivalent, advanced APM correlation.
*   **In Development**: ML-based anomaly detection, automated root cause analysis, predictive scaling recommendations.

## FAQ

*   **Can I run experimental features in production?**: Yes, but start with shadow deployment and gradual rollout.
*   **Performance Impact**: Expect 2-3x more CPU and memory usage compared to standard mode.
*   **Mix Components?**: Yes, any combination makes sense.
*   **How to Contribute?**: Test features and provide feedback via GitHub issues.

## Support

*   GitHub Issues, GitHub Discussions, Documentation.
