# Honest Limitations & Workarounds

## Architectural Limitations

1.  **Single Instance Constraint**: Collector must run as a single instance due to file-based state storage, limiting horizontal scaling and creating a single point of failure. Workarounds include sharding collectors; future solution involves external state stores (Redis/Memcached).
2.  **No Automatic APM Correlation**: Cannot automatically link database queries to APM traces due to lack of trace context propagation in database drivers/engines. Workarounds include manual correlation; a true solution requires industry-wide adoption of SQL comment injection standards.
3.  **Limited MySQL Support**: No safe real-time execution plans due to MySQL's lack of statement-level timeouts and risky EXPLAIN operations on primary databases. Workarounds include using slow query logs or manual EXPLAIN; future enhancements involve safe stored procedures.

## Functional Limitations

1.  **Query Sampling Granularity**: Only analyzes single worst query per cycle, missing moderate issues or workload patterns. Workarounds include reducing collection intervals or using multiple collectors.
2.  **Historical Data Gaps**: Ephemeral and limited state storage (5-minute deduplication window), leading to state loss on restarts. Workarounds include increasing deduplication windows or exporting raw data.
3.  **PII Detection Limitations**: Regex-based PII detection is imperfect, risking sensitive data leaks. Additional safeguards include network isolation, encryption, and regular audits.

## Operational Limitations

1.  **Database Prerequisites**: Requires significant database configuration (e.g., `pg_stat_statements`, Performance Schema), often needing DBA involvement. Workarounds include automating checks and providing setup scripts.
2.  **Resource Scaling**: Memory usage grows with database activity, limiting practical throughput. Workarounds include multiple collector instances, increased memory limits, or reduced sampling rates.
3.  **Error Handling Gaps**: Some errors are silent failures (e.g., malformed plans), making detection difficult. Workarounds include comprehensive logging and metric alerts.

## Security Limitations

1.  **Credential Management**: Database credentials in environment variables pose risks (visibility, no rotation). Mitigation includes Kubernetes secrets and separate credentials.
2.  **Query Visibility**: Full query text collection, even with PII scrubbing, raises privacy concerns. Mitigation includes query allowlists and data retention limits.

## Performance Limitations

1.  **Collection Latency**: Not real-time monitoring (60-second interval, ~2 minutes total latency), unsuitable for sub-minute issues. Workarounds include combining with APM or focusing on trends.

## Data Fidelity Limitations

1.  **Sampling Accuracy**: Statistical sampling may miss rare or intermittent issues. Mitigation includes adjusting sampling rules and combining with other monitoring.
2.  **Plan Format Variations**: Plan formats vary by version/configuration, requiring best-effort parsing and manual analysis. 

## Integration Limitations

1.  **Dashboard Capabilities**: Raw JSON plans in New Relic limit visual analysis. Workarounds include exporting to external tools or custom parsing scripts.

## Planning Around Limitations

*   **Honest Communication**: Be upfront about limitations, provide workarounds, and set realistic timelines.
*   **Incremental Value**: Despite limitations, the MVP provides visibility into slow queries and basic performance metrics, forming a foundation for enhancement.
*   **Clear Evolution Path**: Each limitation has a path forward through short-term workarounds, medium-term enhancements, and long-term solutions.
