# Operations Guide

## Daily Operations

### Health Monitoring

*   **Collector Health Metrics**: Monitor collection success rate, processing performance, memory pressure, and state storage health.
*   **Database Impact Metrics**: Monitor PostgreSQL replica lag, connection count, and statement timeout impacts; MySQL Performance Schema overhead and connection usage.

### Safety Checks

*   **Pre-Operation Checklist**: Verify replica status, check current load, validate configuration.
*   **Emergency Stop Procedures**: Immediately stop collector (Kubernetes: `kubectl scale statefulset nr-db-intelligence --replicas=0`; Docker: `docker stop nr-db-intelligence-collector`), increase collection interval, or disable problematic queries.

## Troubleshooting Guide

### Issue: No Data in New Relic

*   **Diagnosis**: Check collector logs for export/SQL errors, validate credentials, check New Relic ingestion responses.

### Issue: High Memory Usage

*   **Symptoms**: OOMKilled pods, slow processing, dropped data.
*   **Solutions**: Reduce batch size, increase memory limiter interval, lower sampling rates.

### Issue: State Storage Problems

*   **Symptoms**: Duplicate data, inconsistent sampling, "File storage error" logs.
*   **Solutions**: Check disk space, clear corrupted state, verify permissions.

## Performance Tuning

### Collector Optimization

*   **CPU Optimization**: Reduce telemetry metrics level.
*   **Memory Optimization**: Reduce batch timeout and max batch size.
*   **Network Optimization**: Ensure gzip compression, reduce sending queue size.

### Database Query Optimization

*   **PostgreSQL**: Use prepared statements for EXPLAIN plans.
*   **MySQL**: Limit Performance Schema history size.

## Maintenance Windows

### Weekly Tasks

*   State storage cleanup, log rotation verification, metrics review (collection success, memory usage, database impact).

### Monthly Tasks

*   Configuration review (sampling rules, intervals, PII sanitization), security audit (password rotation, access logs, image updates), capacity planning (storage growth, new databases).

## Incident Response

### Playbook: Database Performance Impact

*   **Trigger**: Database CPU spike correlated with collection.
*   **Response**: Scale collector to 0, check `pg_stat_statements` for EXPLAIN queries, increase `statement_timeout`, restart with longer interval, gradually reduce interval.

### Playbook: Data Quality Issues

*   **Trigger**: Missing or corrupted plans in New Relic.
*   **Response**: Check sanitization processor logs, verify `plan_attribute_extractor` errors, review raw plans, adjust parsing rules, clear state storage.

## Monitoring Dashboard

Key panels for your operational dashboard:

1.  **Collection Health**: Plans collected per minute, collection success rate, error types.
2.  **Resource Usage**: Collector CPU/memory, state storage size, network bandwidth.
3.  **Database Impact**: Replica lag trend, connection count, query execution time.
4.  **Data Quality**: Sampling rates, deduplication effectiveness, parse failure rate.
