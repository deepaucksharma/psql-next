# Database Prerequisites

This document outlines the essential database prerequisites for the Database Intelligence MVP.

## PostgreSQL Requirements

### Essential Prerequisites

1.  **`pg_stat_statements` Extension**: Must be installed and enabled (requires `postgresql.conf` update and restart).
    ```sql
    CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
    ```
2.  **Read Replica**: Dedicated read replica, network accessible from collector.
3.  **Monitoring User**: Read-only user with `CONNECT`, `USAGE ON SCHEMA public`, `SELECT ON ALL TABLES IN SCHEMA public`, and `SELECT ON pg_stat_statements` privileges.

### Optional Enhancements

1.  **`auto_explain` Module**: For zero-impact collection (requires `postgresql.conf` update).
    ```sql
    shared_preload_libraries = 'pg_stat_statements,auto_explain'
    auto_explain.log_min_duration = '100ms'
    auto_explain.log_format = 'json'
    ```
2.  **Custom Plan Function**: DEPRECATED - not required for MVP.

### Validation Checklist

*   `pg_stat_statements` shows data.
*   Read replica is accessible.
*   Monitoring user can connect and read `pg_stat_statements`.
*   Replication lag is acceptable (< 30s).
*   No performance impact on primary.

## MySQL Requirements

### Essential Prerequisites

1.  **Performance Schema Enabled**: Must be ON (requires `my.cnf` update and restart).
    ```sql
    [mysqld]
    performance_schema = ON
    ```
2.  **Statement Digests Configured**: Enabled in `performance_schema.setup_consumers`.
3.  **Read Replica or Secondary**: Must handle additional read load.
4.  **Monitoring User**: User with `SELECT`, `PROCESS`, `REPLICATION CLIENT` on `*.*`, and `SELECT` on `performance_schema.*`, `mysql.*`.

### Limitations in MVP

MySQL MVP provides query metadata only, not full execution plans, due to EXPLAIN risks and lack of safe timeout mechanisms.

### Validation Checklist

*   Performance schema is ON.
*   Statement digests are collected.
*   Read replica is accessible.
*   Monitoring user can query `performance_schema`.
*   Digest history is retained.

## MongoDB Requirements

### For Profiler Collection

1.  **Enable Profiling** (on secondary): `db.setProfilingLevel(1, { slowms: 100 })`.
2.  **Create Monitoring User**: User with `read` on `admin` and `local`, `clusterMonitor` on `admin` roles.

### For Log Collection

1.  **Configure Logging**: `systemLog` to file, `logAppend: true`, `verbosity: 1`.
2.  **Set Slow Operation Threshold**: `operationProfiling.slowOpThresholdMs: 100`.

## Common Gotchas

*   **Extension Loading**: Most extensions require server restart.
*   **Permission Propagation**: New permissions may need explicit schema refreshes.
*   **Replica Lag**: Can make collected data stale.
*   **Log Rotation**: Can break `filelog` receiver mid-parse.
*   **Connection Limits**: Monitor connection pool exhaustion.
