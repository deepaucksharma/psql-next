# Troubleshooting Guide

This guide covers common issues and solutions for both production and experimental deployments.

## Quick Diagnostics

*   **Check Component Health**: `curl -s http://localhost:13133/` (Standard) or `http://localhost:13134/` (Experimental).
*   **View Logs**: `docker logs db-intel-primary` (Docker) or `kubectl logs -n db-intelligence deployment/db-intelligence-collector` (Kubernetes).

## Common Issues

### 1. Collector Won't Start

*   **Symptom**: `Error: failed to get config: cannot unmarshal the configuration`.
*   **Causes/Solutions**: Invalid YAML syntax (validate with `otelcol validate`), missing environment variables (check `.env`), custom components not found (ensure custom collector is built).

### 2. No Data Being Collected

*   **Symptom**: Metrics show 0 accepted records, no logs exported.
*   **Debugging**: Check database connectivity (`psql`), verify permissions (`pg_stat_statements`), check leader election (HA mode), verify queries return data.

### 3. Circuit Breaker Keeps Opening

*   **Symptom**: `level=warn msg="Circuit breaker opened" database=primary reason="high error rate"`.
*   **Solutions**: Increase thresholds in config, check database health (active connections, long-running queries), reduce collection frequency.

### 4. High Memory Usage

*   **Symptom**: Container OOMKilled, memory limit exceeded warnings.
*   **Solutions**: Increase memory limits (Docker/Kubernetes), tune memory limiter, reduce ASH buffer sizes.

### 5. Adaptive Sampler Not Adjusting

*   **Symptom**: Sampling rate stays at initial value, no rate adjustments in logs.
*   **Debugging**: Check state persistence (memory/Redis), verify strategy triggers, enable debug logging.

### 6. ASH Sampling Missing Data

*   **Symptom**: No ASH samples in output, `ash_sample` field empty.
*   **Solutions**: Verify `pg_wait_sampling` extension, check sampling configuration.

### 7. Plan Collection Not Working

*   **Symptom**: `plan_metadata` shows `plan_available: false`, no execution plans in output.
*   **Current Status**: Requires `pg_querylens` extension (not yet available).
*   **Workaround**: Use query metadata and `pg_stat_statements` for performance analysis.

## Performance Tuning

*   **Reduce Database Load**: Increase collection intervals, limit concurrent connections.
*   **Optimize Network Usage**: Increase batch sizes, enable compression.
*   **Memory Optimization**: Use `GOGC` and `GOMEMLIMIT` environment variables.

## Debug Mode

*   **Enable Detailed Logging**: Set `service.telemetry.logs.level: debug` in collector config.
*   **Use Debug Endpoints**: ZPages (`http://localhost:55680/debug/tracez`), pprof (`http://localhost:6061/debug/pprof/heap`).
*   **Dry Run Mode**: Test configuration without starting (`./dist/db-intelligence-custom --config=config/collector.yaml --dry-run`).

## Getting Help

### Collect Diagnostics

Use `collect-diagnostics.sh` script to gather logs, metrics, and sanitized config.

### Support Channels

*   GitHub Issues (bugs/features), GitHub Discussions (questions), New Relic Support.

### Known Limitations

*   Plan Collection (pending `pg_querylens`).
*   Multi-Instance State (requires external Redis).
*   MySQL ASH (not implemented).
*   Cross-Database Correlation (limited).

## Emergency Procedures

*   **Disable Collection Immediately**: `docker-compose stop db-intelligence-primary` or `kubectl scale deployment db-intelligence-collector --replicas=0`.
*   **Revert to Standard Components**: `kubectl set image deployment/db-intelligence-collector collector=otel/opentelemetry-collector-contrib:0.88.0`.
*   **Clear State and Restart**: `docker-compose down`, `docker volume rm database-intelligence-mvp_collector-data`, `docker-compose up -d`.
