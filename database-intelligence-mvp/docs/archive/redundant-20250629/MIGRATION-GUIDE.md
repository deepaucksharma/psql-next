# Migration Guide: Documentation to Reality

## For New Users

If you're just starting, use these files:

1.  **README**: Use `README-ALIGNED.md` (reflects what actually works).
2.  **Configuration**: Use `config/collector.yaml` (tested configuration).
3.  **Architecture**: Read `ARCHITECTURE-REALITY.md` (shows the actual design).
4.  **Deployment**: Use single-instance deployment only.

## For Existing Users

If you've been trying to use features that don't work:

### Query Plan Collection

*   **Expected**: Real PostgreSQL execution plans.
*   **Reality**: Static JSON placeholder.
*   **Migration**: Stop expecting plan data; use `auto_explain` logs (if available); await safe EXPLAIN implementation.

### Adaptive Sampling

*   **Expected**: Smart workload-based sampling.
*   **Reality**: Simple 10% probabilistic sampling.
*   **Migration**: Adjust `sampling_percentage` in `probabilistic_sampler`; monitor data volume manually.

### High Availability

*   **Expected**: Multi-instance with leader election.
*   **Reality**: Single instance only.
*   **Migration**: Scale down to 1 replica; remove StatefulSet configurations; ignore HA deployment examples.

### Circuit Breaker

*   **Expected**: Per-database failure isolation.
*   **Reality**: Not active in the collector.
*   **Migration**: Implement connection timeouts; monitor failures via collector metrics; manual intervention for problem databases.

## Configuration Changes

*   **Remove Non-Existent Processors**: Remove `adaptivesampler`, `circuitbreaker`, `planattributeextractor` from processor configurations.
*   **Fix Database Expectations**: Acknowledge metadata-only collection instead of real plans.
*   **Single Instance Deployment**: Use `replicas: 1` for consistency.

## Monitoring Adjustments

*   **Metrics That Don't Exist**: `database_intelligence_adaptive_sampling_rate`, `database_intelligence_plan_changes_detected`, `database_intelligence_circuit_breaker_state`.
*   **Metrics That Actually Work**: `otelcol_receiver_accepted_log_records`, `otelcol_processor_dropped_log_records`, `otelcol_exporter_sent_log_records`, `otelcol_process_memory_rss`.

## File Structure Cleanup

*   **Move Experimental Code**: Move unintegrated components to an `experimental` directory.
*   **Update Documentation**: Replace `README.md`, `CONFIGURATION.md`, `ARCHITECTURE.md` with aligned versions; add `IMPLEMENTATION-STATUS.md`.

## Communication Template

Use the provided template to communicate updates to your team, clearly outlining what works and what doesn't, and required actions.

## Timeline for Feature Delivery

*   **Q1 2024**: Documentation alignment, single instance stability.
*   **Q2 2024**: Circuit breaker integration, safe EXPLAIN for top query.
*   **Q3 2024**: Basic adaptive sampling, plan change detection.
*   **Q4 2024**: Multi-instance support, full plan analysis.

## Support During Migration

*   Check `IMPLEMENTATION-STATUS.md` for feature availability.
*   Use standard OTEL docs for component behavior.
*   Focus on what works rather than what's planned.
*   Report issues with actual features, not documented ones.
