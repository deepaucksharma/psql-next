# Technical Reference: OpenTelemetry for PostgreSQL

## Table of Contents
1. [Architecture Overview](#architecture-overview)
2. [Collector Configuration](#collector-configuration)
3. [PostgreSQL Receiver Setup](#postgresql-receiver-setup)
4. [Metric Definitions](#metric-definitions)
5. [Security Configuration](#security-configuration)
6. [Performance Tuning](#performance-tuning)
7. [Integration Patterns](#integration-patterns)
8. [Troubleshooting Guide](#troubleshooting-guide)

## Architecture Overview

### Component Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     PostgreSQL Cluster                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Primary    │  │  Replica 1   │  │  Replica 2   │     │
│  │  pgbouncer   │  │  pgbouncer   │  │  pgbouncer   │     │
│  │  pg_exporter │  │  pg_exporter │  │  pg_exporter │     │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │
│         │                  │                  │              │
└─────────┼──────────────────┼──────────────────┼──────────────┘
          │                  │                  │
          └──────────────────┴──────────────────┘
                             │
                    ┌────────▼────────┐
                    │ OTel Collector  │
                    │   (DaemonSet)   │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │ OTel Gateway    │
                    │   (StatefulSet) │
                    └────────┬────────┘
                             │
                ┌────────────┴────────────┐
                │                         │
        ┌───────▼────────┐      ┌────────▼────────┐
        │   Prometheus   │      │    S3/GCS       │
        │  (Hot Storage) │      │ (Cold Storage)  │
        └────────────────┘      └─────────────────┘
```

### Network Flow

```yaml
network_topology:
  collection_layer:
    postgres_to_collector:
      protocol: tcp
      port: 5432
      security: tls_required
    
    exporter_to_collector:
      protocol: http
      port: 9187
      path: /metrics
  
  processing_layer:
    collector_to_gateway:
      protocol: grpc
      port: 4317
      compression: gzip
      batch_size: 1000
    
    gateway_to_storage:
      prometheus:
        protocol: http
        port: 9090
        path: /api/v1/write
      
      s3:
        protocol: https
        port: 443
        path: /metrics-bucket
```

## PostgreSQL Monitoring User Setup

### Required Permissions

```sql
-- Create dedicated monitoring user
CREATE USER otel_monitor WITH PASSWORD 'CHANGE_ME_USE_SECRETS_MANAGER';

-- Grant necessary permissions for monitoring
GRANT pg_monitor TO otel_monitor;
GRANT CONNECT ON DATABASE postgres TO otel_monitor;

-- For each database to monitor:
\c your_database
GRANT CONNECT ON DATABASE your_database TO otel_monitor;
GRANT USAGE ON SCHEMA pg_catalog TO otel_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO otel_monitor;

-- For custom monitoring queries
GRANT SELECT ON pg_stat_statements TO otel_monitor;
GRANT SELECT ON pg_stat_kcache TO otel_monitor;

-- For replication monitoring
GRANT SELECT ON pg_stat_replication TO otel_monitor;

-- Connection limits to prevent exhaustion
ALTER USER otel_monitor CONNECTION LIMIT 10;
```

### pg_hba.conf Configuration

```
# TYPE  DATABASE        USER            ADDRESS                 METHOD
host    all            otel_monitor    10.0.0.0/8              md5
host    all            otel_monitor    172.16.0.0/12           md5
hostssl all            otel_monitor    0.0.0.0/0               cert
```

## Collector Configuration

### Base Configuration

```yaml
# otel-collector-config.yaml
receivers:
  # PostgreSQL native receiver
  postgresql:
    endpoint: ${env:POSTGRES_ENDPOINT}
    transport: tcp
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DB}
    collection_interval: 10s
    tls:
      insecure: false
      ca_file: /etc/ssl/certs/ca-certificates.crt
      cert_file: /etc/ssl/certs/postgres-cert.pem
      key_file: /etc/ssl/private/postgres-key.pem

  # Prometheus receiver for pg_exporter compatibility
  prometheus:
    config:
      scrape_configs:
        - job_name: 'postgres_exporter'
          scrape_interval: 15s
          static_configs:
            - targets: ['localhost:9187']
          metric_relabel_configs:
            - source_labels: [__name__]
              regex: 'pg_.*'
              action: keep

  # Host metrics for collector health
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
      memory:
      disk:
      network:

processors:
  # Add resource attributes
  resource:
    attributes:
      - key: service.name
        value: "postgresql"
        action: insert
      - key: deployment.environment
        value: ${env:ENVIRONMENT}
        action: insert
      - key: db.system
        value: "postgresql"
        action: insert
      - key: db.cluster
        value: ${env:CLUSTER_NAME}
        action: insert
      - key: db.version
        from_attribute: postgresql.version
        action: insert
      - key: db.role
        value: ${env:DB_ROLE}  # primary, replica, standby
        action: insert

  # Batch for efficiency
  batch:
    send_batch_size: 10000
    send_batch_max_size: 20000
    timeout: 5s

  # Memory limiter to prevent OOM
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 25

  # Metrics transform
  metricstransform:
    transforms:
      - include: pg_stat_database_.*
        match_type: regexp
        action: update
        operations:
          - action: add_label
            new_label: db.operation
            new_value: database_stats
      
      - include: pg_stat_user_tables_.*
        match_type: regexp
        action: update
        operations:
          - action: add_label
            new_label: db.operation
            new_value: table_stats

  # Filter out unnecessary metrics
  filter:
    metrics:
      exclude:
        match_type: regexp
        metric_names:
          - .*_test_.*
          - .*_debug_.*

exporters:
  # Primary storage: Prometheus
  prometheusremotewrite:
    endpoint: "${env:PROMETHEUS_ENDPOINT}"
    tls:
      insecure: false
    headers:
      X-Prometheus-Remote-Write-Version: "0.1.0"
    resource_to_telemetry_conversion:
      enabled: true
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

  # Long-term storage: S3
  awss3:
    s3uploader:
      region: ${env:AWS_REGION}
      s3_bucket: ${env:S3_BUCKET}
      s3_prefix: "postgres-metrics"
      s3_partition: minute
    marshaler: otlp_proto

  # Debug output (disabled in production)
  logging:
    loglevel: info
    sampling_initial: 10
    sampling_thereafter: 100

  # Health check endpoint
  health_check:
    path: "/health"
    endpoint: "0.0.0.0:13133"

service:
  pipelines:
    metrics:
      receivers: [postgresql, prometheus, hostmetrics]
      processors: [memory_limiter, resource, metricstransform, filter, batch]
      exporters: [prometheusremotewrite, awss3]

  extensions: [health_check, pprof, zpages]
  
  telemetry:
    logs:
      level: info
      initial_fields:
        service: "otel-collector"
    metrics:
      level: detailed
      address: 0.0.0.0:8888
```

### Advanced Configurations

#### High Availability Setup

```yaml
# otel-collector-ha.yaml
receivers:
  postgresql:
    endpoint: ${env:POSTGRES_ENDPOINT}
    # Connection pooling for HA
    connection_pool:
      max_idle_conns: 5
      max_open_conns: 10
      conn_max_lifetime: 300s
      conn_max_idle_time: 90s
    
    # Failover configuration
    replicas:
      - endpoint: ${env:POSTGRES_REPLICA1}
        role: replica
      - endpoint: ${env:POSTGRES_REPLICA2}
        role: replica
    
    # Query routing
    query_config:
      replication_stats:
        target: primary
        interval: 30s
      read_only_stats:
        target: replica
        interval: 15s

processors:
  groupbyattrs:
    keys:
      - db.cluster
      - db.instance
      - db.operation
  
  # Deduplication for HA
  dedupe:
    cache_size: 10000
    cache_ttl: 60s
    identifier_keys:
      - db.instance
      - metric.name
      - timestamp

exporters:
  loadbalancing:
    protocol:
      otlp:
        endpoint: dns:///otel-gateway:4317
        tls:
          insecure: false
    resolver:
      dns:
        hostname: otel-gateway
        port: 4317
```

## PostgreSQL Receiver Setup

### Receiver Configuration Details

```yaml
postgresql_receiver:
  # Connection settings
  connection:
    endpoint: "postgres://host:5432/database"
    transport: "tcp"
    
    # Authentication
    auth:
      username: "otel_monitor"
      password: "${env:MONITOR_PASSWORD}"
      
    # SSL/TLS
    tls:
      mode: "require"  # disable, allow, prefer, require, verify-ca, verify-full
      cert_file: "/path/to/client-cert.pem"
      key_file: "/path/to/client-key.pem"
      ca_file: "/path/to/ca-cert.pem"
      server_name: "postgres.example.com"
  
  # Collection settings
  collection:
    # Metrics collection interval
    interval: "10s"
    
    # Initial delay before first collection
    initial_delay: "10s"
    
    # Query timeout
    timeout: "5s"
    
    # Databases to monitor (empty = all)
    databases:
      - "production_db"
      - "analytics_db"
    
    # Exclude databases
    exclude_databases:
      - "template0"
      - "template1"
      - "postgres"
  
  # Metric configuration
  metrics:
    # Database metrics
    postgresql.database.size:
      enabled: true
      unit: "By"
      description: "Database size in bytes"
    
    postgresql.database.connections:
      enabled: true
      unit: "1"
      description: "Number of active connections"
    
    postgresql.database.commits:
      enabled: true
      unit: "1"
      description: "Number of commits"
      
    postgresql.database.rollbacks:
      enabled: true
      unit: "1"
      description: "Number of rollbacks"
    
    # Table metrics
    postgresql.table.size:
      enabled: true
      unit: "By"
      description: "Table size in bytes"
    
    postgresql.table.vacuum_count:
      enabled: true
      unit: "1"
      description: "Number of vacuum operations"
    
    # Index metrics
    postgresql.index.size:
      enabled: true
      unit: "By"
      description: "Index size in bytes"
    
    postgresql.index.scans:
      enabled: true
      unit: "1"
      description: "Number of index scans"
    
    # Replication metrics
    postgresql.replication.lag:
      enabled: true
      unit: "s"
      description: "Replication lag in seconds"
    
    # BGWriter metrics
    postgresql.bgwriter.checkpoints:
      enabled: true
      unit: "1"
      description: "Number of checkpoints"
    
    postgresql.bgwriter.buffers_written:
      enabled: true
      unit: "1"
      description: "Buffers written by bgwriter"
  
  # Resource attributes
  resource_attributes:
    db.system:
      enabled: true
      default: "postgresql"
    
    db.postgresql.version:
      enabled: true
    
    db.cluster.name:
      enabled: true
      default: "${env:CLUSTER_NAME}"
```

### Custom Queries

```yaml
custom_queries:
  # Critical: WAL generation rate (for capacity planning)
  - query: |
      SELECT 
        CASE 
          WHEN pg_is_in_recovery() THEN 'replica'
          ELSE 'primary'
        END as server_role,
        pg_current_wal_lsn() as current_lsn,
        pg_wal_lsn_diff(pg_current_wal_lsn(), '0/0') as total_wal_bytes
    metrics:
      - name: "postgresql.wal.bytes_generated"
        value_column: "total_wal_bytes"
        data_type: counter
        unit: "bytes"
        attributes:
          - column: "server_role"
  
  # Critical: Replication slot lag (prevents WAL removal)
  - query: |
      SELECT
        slot_name,
        slot_type,
        active,
        pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn) as lag_bytes,
        EXTRACT(epoch FROM (now() - last_confirmed_at)) as seconds_since_confirm
      FROM pg_replication_slots
      WHERE slot_type = 'logical'
    metrics:
      - name: "postgresql.replication_slot.lag_bytes"
        value_column: "lag_bytes"
        data_type: gauge
        unit: "bytes"
        attributes:
          - column: "slot_name"
          - column: "active"
  
  # Critical: Lock monitoring
  - query: |
      SELECT 
        wait_event_type,
        wait_event,
        COUNT(*) as waiting_sessions
      FROM pg_stat_activity
      WHERE wait_event IS NOT NULL
      GROUP BY wait_event_type, wait_event
    metrics:
      - name: "postgresql.locks.waiting_sessions"
        value_column: "waiting_sessions"
        data_type: gauge
        unit: "sessions"
        attributes:
          - column: "wait_event_type"
          - column: "wait_event"
  
  # Critical: Vacuum progress
  - query: |
      SELECT
        schemaname,
        tablename,
        n_dead_tup,
        n_live_tup,
        EXTRACT(epoch FROM (now() - last_vacuum)) as seconds_since_vacuum,
        EXTRACT(epoch FROM (now() - last_autovacuum)) as seconds_since_autovacuum
      FROM pg_stat_user_tables
      WHERE n_dead_tup > 1000
      ORDER BY n_dead_tup DESC
      LIMIT 50
    metrics:
      - name: "postgresql.vacuum.dead_tuples"
        value_column: "n_dead_tup"
        data_type: gauge
        unit: "rows"
        attributes:
          - column: "schemaname"
          - column: "tablename"
  
  # Table bloat estimation
  - query: |
      SELECT
        schemaname,
        tablename,
        pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size,
        n_live_tup,
        n_dead_tup,
        round(100 * n_dead_tup / NULLIF(n_live_tup + n_dead_tup, 0), 2) as dead_ratio
      FROM pg_stat_user_tables
      WHERE n_dead_tup > 1000
      ORDER BY n_dead_tup DESC
    metrics:
      - name: "custom.table.bloat"
        value_column: "dead_ratio"
        data_type: gauge
        unit: "%"
        attributes:
          - column: "schemaname"
            name: "schema"
          - column: "tablename"
            name: "table"
  
  # Long running queries
  - query: |
      SELECT
        pid,
        usename,
        application_name,
        client_addr,
        backend_start,
        state,
        wait_event_type,
        wait_event,
        extract(epoch from (now() - query_start)) as query_duration,
        query
      FROM pg_stat_activity
      WHERE state != 'idle'
        AND query_start < now() - interval '1 minute'
    metrics:
      - name: "custom.query.long_running"
        value_column: "query_duration"
        data_type: gauge
        unit: "s"
        attributes:
          - column: "usename"
            name: "user"
          - column: "application_name"
            name: "application"
          - column: "state"
            name: "query_state"
```

## Metric Definitions

### Core PostgreSQL Metrics

```yaml
metric_catalog:
  # Database Level Metrics
  database_metrics:
    - name: postgresql.database.size
      type: gauge
      unit: bytes
      description: "Total size of the database"
      query: "SELECT pg_database_size(datname) FROM pg_database"
      labels:
        - database_name
        - cluster_name
    
    - name: postgresql.database.connections
      type: gauge
      unit: connections
      description: "Number of active connections to the database"
      query: "SELECT numbackends FROM pg_stat_database"
      labels:
        - database_name
        - connection_state
    
    - name: postgresql.database.transactions
      type: counter
      unit: transactions
      description: "Number of transactions"
      query: "SELECT xact_commit + xact_rollback FROM pg_stat_database"
      labels:
        - database_name
        - transaction_type
    
    - name: postgresql.database.blocks
      type: counter
      unit: blocks
      description: "Number of disk blocks read/hit"
      query: "SELECT blks_read, blks_hit FROM pg_stat_database"
      labels:
        - database_name
        - block_type
    
    - name: postgresql.database.tuples
      type: counter
      unit: tuples
      description: "Number of tuples returned/fetched/inserted/updated/deleted"
      query: "SELECT tup_returned, tup_fetched, tup_inserted, tup_updated, tup_deleted FROM pg_stat_database"
      labels:
        - database_name
        - operation
    
    - name: postgresql.database.conflicts
      type: counter
      unit: conflicts
      description: "Number of queries canceled due to conflicts"
      query: "SELECT conflicts FROM pg_stat_database"
      labels:
        - database_name
        - conflict_type
    
    - name: postgresql.database.deadlocks
      type: counter
      unit: deadlocks
      description: "Number of deadlocks detected"
      query: "SELECT deadlocks FROM pg_stat_database"
      labels:
        - database_name
    
    - name: postgresql.database.checksum_failures
      type: counter
      unit: failures
      description: "Number of data page checksum failures"
      query: "SELECT checksum_failures FROM pg_stat_database"
      labels:
        - database_name
  
  # Table Level Metrics
  table_metrics:
    - name: postgresql.table.size
      type: gauge
      unit: bytes
      description: "Total size of the table including indexes"
      query: "SELECT pg_total_relation_size(schemaname||'.'||tablename) FROM pg_stat_user_tables"
      labels:
        - schema_name
        - table_name
    
    - name: postgresql.table.rows
      type: gauge
      unit: rows
      description: "Estimated number of rows"
      query: "SELECT n_live_tup FROM pg_stat_user_tables"
      labels:
        - schema_name
        - table_name
    
    - name: postgresql.table.dead_rows
      type: gauge
      unit: rows
      description: "Estimated number of dead rows"
      query: "SELECT n_dead_tup FROM pg_stat_user_tables"
      labels:
        - schema_name
        - table_name
    
    - name: postgresql.table.modifications
      type: counter
      unit: modifications
      description: "Number of rows inserted/updated/deleted"
      query: "SELECT n_tup_ins, n_tup_upd, n_tup_del FROM pg_stat_user_tables"
      labels:
        - schema_name
        - table_name
        - operation
    
    - name: postgresql.table.vacuum
      type: counter
      unit: operations
      description: "Number of vacuum operations"
      query: "SELECT vacuum_count, autovacuum_count FROM pg_stat_user_tables"
      labels:
        - schema_name
        - table_name
        - vacuum_type
    
    - name: postgresql.table.analyze
      type: counter
      unit: operations
      description: "Number of analyze operations"
      query: "SELECT analyze_count, autoanalyze_count FROM pg_stat_user_tables"
      labels:
        - schema_name
        - table_name
        - analyze_type
  
  # Index Level Metrics
  index_metrics:
    - name: postgresql.index.size
      type: gauge
      unit: bytes
      description: "Size of the index"
      query: "SELECT pg_relation_size(indexrelid) FROM pg_stat_user_indexes"
      labels:
        - schema_name
        - table_name
        - index_name
    
    - name: postgresql.index.scans
      type: counter
      unit: scans
      description: "Number of index scans initiated"
      query: "SELECT idx_scan FROM pg_stat_user_indexes"
      labels:
        - schema_name
        - table_name
        - index_name
    
    - name: postgresql.index.tuples
      type: counter
      unit: tuples
      description: "Number of index entries returned/fetched"
      query: "SELECT idx_tup_read, idx_tup_fetch FROM pg_stat_user_indexes"
      labels:
        - schema_name
        - table_name
        - index_name
        - operation
  
  # Replication Metrics
  replication_metrics:
    - name: postgresql.replication.lag
      type: gauge
      unit: bytes
      description: "Replication lag in bytes"
      query: |
        SELECT 
          pg_wal_lsn_diff(pg_current_wal_lsn(), flush_lsn) as lag_bytes
        FROM pg_stat_replication
      labels:
        - application_name
        - client_addr
        - state
    
    - name: postgresql.replication.lag_seconds
      type: gauge
      unit: seconds
      description: "Replication lag in seconds"
      query: |
        SELECT 
          extract(epoch from (now() - pg_last_xact_replay_timestamp())) as lag_seconds
      labels:
        - replica_name
  
  # Connection Pool Metrics
  connection_metrics:
    - name: postgresql.connections.active
      type: gauge
      unit: connections
      description: "Number of active connections"
      query: "SELECT count(*) FROM pg_stat_activity WHERE state = 'active'"
      labels:
        - database_name
        - user_name
        - application_name
    
    - name: postgresql.connections.idle
      type: gauge
      unit: connections
      description: "Number of idle connections"
      query: "SELECT count(*) FROM pg_stat_activity WHERE state = 'idle'"
      labels:
        - database_name
        - user_name
        - application_name
    
    - name: postgresql.connections.waiting
      type: gauge
      unit: connections
      description: "Number of connections waiting for locks"
      query: "SELECT count(*) FROM pg_stat_activity WHERE wait_event IS NOT NULL"
      labels:
        - database_name
        - wait_event_type
        - wait_event
  
  # Performance Metrics
  performance_metrics:
    - name: postgresql.cache_hit_ratio
      type: gauge
      unit: ratio
      description: "Cache hit ratio"
      query: |
        SELECT 
          sum(blks_hit) / NULLIF(sum(blks_hit) + sum(blks_read), 0) as ratio
        FROM pg_stat_database
      labels:
        - database_name
    
    - name: postgresql.transaction_rate
      type: gauge
      unit: tps
      description: "Transactions per second"
      query: |
        SELECT 
          (xact_commit + xact_rollback) / 
          EXTRACT(EPOCH FROM (now() - stats_reset)) as tps
        FROM pg_stat_database
      labels:
        - database_name
    
    - name: postgresql.checkpoint_time
      type: gauge
      unit: milliseconds
      description: "Time spent in checkpoint processing"
      query: "SELECT checkpoint_write_time + checkpoint_sync_time FROM pg_stat_bgwriter"
      labels:
        - checkpoint_type
```

### Derived Metrics

```yaml
derived_metrics:
  # Business KPIs
  - name: database_health_score
    expression: |
      (
        (postgresql_cache_hit_ratio > 0.99) * 25 +
        (postgresql_connections_active < 100) * 25 +
        (postgresql_replication_lag_seconds < 1) * 25 +
        (postgresql_deadlocks_total < 1) * 25
      )
    unit: score
    description: "Overall database health score (0-100)"
  
  # Capacity Planning
  - name: connection_saturation
    expression: |
      postgresql_connections_active / postgresql_settings_max_connections
    unit: ratio
    description: "Connection pool saturation"
  
  - name: storage_growth_rate
    expression: |
      rate(postgresql_database_size[1h])
    unit: bytes_per_second
    description: "Database growth rate"
  
  # Performance Indicators
  - name: query_performance_index
    expression: |
      1 - (rate(postgresql_slow_queries[5m]) / rate(postgresql_queries_total[5m]))
    unit: ratio
    description: "Query performance index (1 = perfect)"
```

## Security Configuration

### Authentication & Authorization

```yaml
security_config:
  # PostgreSQL user for monitoring
  postgresql_user:
    username: "otel_monitor"
    permissions:
      - "pg_monitor"  # PostgreSQL monitoring role
      - "pg_read_all_stats"
      - "pg_read_all_settings"
    
    # Restricted permissions
    grants:
      - "GRANT CONNECT ON DATABASE * TO otel_monitor"
      - "GRANT USAGE ON SCHEMA pg_catalog TO otel_monitor"
      - "GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO otel_monitor"
    
    # Connection limits
    connection_limit: 10
    
    # Password policy
    password_policy:
      rotation_days: 90
      complexity: high
      storage: "aws_secrets_manager"
  
  # TLS Configuration
  tls:
    # Server-side TLS
    server:
      enabled: true
      cert_file: "/etc/ssl/certs/postgres-server.crt"
      key_file: "/etc/ssl/private/postgres-server.key"
      ca_file: "/etc/ssl/certs/postgres-ca.crt"
      
      # TLS versions
      min_version: "1.2"
      max_version: "1.3"
      
      # Cipher suites
      cipher_suites:
        - "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"
        - "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
        - "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"
        - "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256"
    
    # Client-side TLS
    client:
      enabled: true
      verify_mode: "verify-full"
      cert_file: "/etc/ssl/certs/otel-client.crt"
      key_file: "/etc/ssl/private/otel-client.key"
      ca_file: "/etc/ssl/certs/postgres-ca.crt"
      
      # Certificate validation
      check_hostname: true
      check_expiry: true
      min_days_before_expiry: 30
  
  # Network Security
  network:
    # IP allowlist
    allowed_ips:
      - "10.0.0.0/8"      # Internal network
      - "172.16.0.0/12"   # Private network
    
    # Firewall rules
    firewall_rules:
      - name: "allow_otel_collectors"
        source: "10.0.1.0/24"
        destination_port: 5432
        protocol: "tcp"
        action: "allow"
    
    # Rate limiting
    rate_limiting:
      enabled: true
      requests_per_second: 100
      burst: 200
```

### Secrets Management

```yaml
secrets_management:
  # AWS Secrets Manager
  aws_secrets_manager:
    enabled: true
    region: "us-east-1"
    secrets:
      - name: "postgres/otel-monitor/password"
        key: "password"
        rotation_enabled: true
        rotation_lambda: "arn:aws:lambda:us-east-1:123456789:function:rotate-secrets"
  
  # HashiCorp Vault
  vault:
    enabled: false
    address: "https://vault.example.com"
    auth_method: "kubernetes"
    path: "database/creds/otel-monitor"
    ttl: "24h"
    max_ttl: "168h"
  
  # Kubernetes Secrets
  kubernetes_secrets:
    enabled: true
    namespace: "monitoring"
    secrets:
      - name: "postgres-credentials"
        type: "Opaque"
        data:
          username: "b3RlbF9tb25pdG9y"  # base64 encoded
          password: "${ENCRYPTED_PASSWORD}"
```

## Performance Tuning

### Collector Optimization

```yaml
performance_tuning:
  # Resource allocation
  resources:
    requests:
      memory: "512Mi"
      cpu: "500m"
    limits:
      memory: "2Gi"
      cpu: "2000m"
  
  # JVM settings (if using Java-based collector)
  jvm_options:
    - "-Xmx1g"
    - "-Xms512m"
    - "-XX:MaxDirectMemorySize=512m"
    - "-XX:+UseG1GC"
    - "-XX:MaxGCPauseMillis=100"
  
  # Collection optimization
  collection:
    # Batch processing
    batch_size: 10000
    batch_timeout: "5s"
    
    # Connection pooling
    connection_pool:
      size: 10
      timeout: "30s"
      idle_timeout: "300s"
      max_lifetime: "1800s"
    
    # Query optimization
    query_options:
      timeout: "5s"
      fetch_size: 1000
      cache_prepared_statements: true
      prepared_statement_cache_size: 250
  
  # Network optimization
  network:
    # TCP settings
    tcp_no_delay: true
    tcp_keepalive: true
    tcp_keepalive_time: 600
    tcp_keepalive_interval: 60
    tcp_keepalive_probes: 3
    
    # Buffer sizes
    send_buffer_size: "4MB"
    receive_buffer_size: "4MB"
    
    # Compression
    compression:
      enabled: true
      type: "zstd"
      level: 3
```

### PostgreSQL Optimization

```sql
-- Optimize PostgreSQL for monitoring
-- Run these on the PostgreSQL server

-- Create monitoring-specific indexes
CREATE INDEX CONCURRENTLY idx_pg_stat_activity_state 
ON pg_catalog.pg_stat_activity(state) 
WHERE state != 'idle';

CREATE INDEX CONCURRENTLY idx_pg_stat_activity_wait_event 
ON pg_catalog.pg_stat_activity(wait_event_type, wait_event) 
WHERE wait_event IS NOT NULL;

-- Create monitoring views for better performance
CREATE OR REPLACE VIEW monitoring.connection_stats AS
SELECT 
    datname as database_name,
    usename as user_name,
    application_name,
    state,
    count(*) as connection_count,
    max(backend_start) as oldest_connection,
    avg(EXTRACT(EPOCH FROM (now() - backend_start))) as avg_connection_age
FROM pg_stat_activity
WHERE pid != pg_backend_pid()
GROUP BY datname, usename, application_name, state;

-- Grant permissions to monitoring user
GRANT SELECT ON monitoring.connection_stats TO otel_monitor;

-- Configure PostgreSQL for efficient stats collection
ALTER SYSTEM SET track_activities = on;
ALTER SYSTEM SET track_counts = on;
ALTER SYSTEM SET track_functions = 'all';
ALTER SYSTEM SET track_io_timing = on;
ALTER SYSTEM SET track_wal_io_timing = on;

-- Optimize statistics collector
ALTER SYSTEM SET stats_temp_directory = '/run/postgresql';
ALTER SYSTEM SET stats_fetch_consistency = 'snapshot';

-- Apply changes
SELECT pg_reload_conf();
```

## Integration Patterns

### Prometheus Integration

```yaml
# prometheus-config.yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  # OTel Collector metrics
  - job_name: 'otel-collector'
    static_configs:
      - targets: ['otel-collector:8888']
    metric_relabel_configs:
      - source_labels: [__name__]
        regex: 'otelcol_.*'
        action: keep

remote_write:
  - url: http://otel-gateway:9090/api/v1/write
    write_relabel_configs:
      - source_labels: [__name__]
        regex: 'postgresql_.*'
        action: keep
    queue_config:
      capacity: 10000
      max_shards: 30
      min_shards: 1
      max_samples_per_send: 5000
      batch_send_deadline: 5s
      min_backoff: 30ms
      max_backoff: 100ms
    metadata_config:
      send: true
      send_interval: 1m
```

### Grafana Integration

```json
{
  "datasources": [
    {
      "name": "PostgreSQL-OTel",
      "type": "prometheus",
      "url": "http://prometheus:9090",
      "access": "proxy",
      "jsonData": {
        "timeInterval": "15s",
        "queryTimeout": "60s",
        "httpMethod": "POST"
      }
    }
  ],
  "dashboards": [
    {
      "name": "PostgreSQL Overview",
      "uid": "postgresql-overview",
      "panels": [
        {
          "title": "Database Size",
          "targets": [
            {
              "expr": "postgresql_database_size_bytes",
              "legendFormat": "{{database_name}}"
            }
          ]
        },
        {
          "title": "Active Connections",
          "targets": [
            {
              "expr": "postgresql_connections_active",
              "legendFormat": "{{database_name}} - {{state}}"
            }
          ]
        },
        {
          "title": "Transaction Rate",
          "targets": [
            {
              "expr": "rate(postgresql_database_transactions_total[5m])",
              "legendFormat": "{{database_name}} - {{transaction_type}}"
            }
          ]
        },
        {
          "title": "Cache Hit Ratio",
          "targets": [
            {
              "expr": "postgresql_cache_hit_ratio",
              "legendFormat": "{{database_name}}"
            }
          ]
        }
      ]
    }
  ]
}
```

### Alerting Integration

```yaml
# alerting-rules.yaml
groups:
  - name: postgresql_critical
    interval: 30s
    rules:
      # Database Down
      - alert: PostgreSQLDown
        expr: pg_up == 0
        for: 1m
        labels:
          severity: critical
          team: database
          page: true
        annotations:
          summary: "PostgreSQL instance {{$labels.instance}} is down"
          description: "PostgreSQL has been down for more than 1 minute"
          runbook: "https://wiki/runbooks/postgresql-down"
      
      # Replication broken
      - alert: PostgreSQLReplicationBroken
        expr: pg_stat_replication_count == 0 and pg_is_in_recovery() == 0
        for: 5m
        labels:
          severity: critical
          team: database
        annotations:
          summary: "PostgreSQL replication is broken on {{$labels.instance}}"
          description: "Primary server has no connected replicas"
      
      # WAL accumulation (disk space risk)
      - alert: PostgreSQLWALAccumulation
        expr: pg_wal_count > 100
        for: 10m
        labels:
          severity: critical
          team: database
        annotations:
          summary: "WAL files accumulating on {{$labels.instance}}"
          description: "{{$value}} WAL files present, check replication slots"
      
      # Transaction wraparound warning
      - alert: PostgreSQLTransactionWraparound
        expr: pg_database_age > 1500000000
        for: 5m
        labels:
          severity: critical
          team: database
          page: true
        annotations:
          summary: "Transaction ID wraparound risk on {{$labels.database}}"
          description: "Database age: {{$value}}, autovacuum may be failing"

  - name: postgresql_performance
    interval: 30s
    rules:
      # Cache hit ratio
      - alert: PostgreSQLLowCacheHitRatio
        expr: |
          (sum by (instance, database) (pg_stat_database_blks_hit)) /
          (sum by (instance, database) (pg_stat_database_blks_hit + pg_stat_database_blks_read)) < 0.9
        for: 15m
        labels:
          severity: warning
          team: database
        annotations:
          summary: "Low cache hit ratio on {{$labels.database}}"
          description: "Cache hit ratio: {{$value | humanizePercentage}}"
      
      # Connection saturation
      - alert: PostgreSQLConnectionSaturation
        expr: |
          sum by (instance) (pg_stat_database_numbackends) / 
          pg_settings_max_connections > 0.8
        for: 5m
        labels:
          severity: warning
          team: database
        annotations:
          summary: "Connection pool near saturation on {{$labels.instance}}"
          description: "{{$value | humanizePercentage}} of max connections used"
      
      # Long running transactions
      - alert: PostgreSQLLongRunningTransaction
        expr: pg_stat_activity_max_tx_duration > 3600
        for: 5m
        labels:
          severity: warning
          team: database
        annotations:
          summary: "Long running transaction on {{$labels.instance}}"
          description: "Transaction running for {{$value}} seconds"
      
      # Table bloat
      - alert: PostgreSQLTableBloat
        expr: pg_stat_user_tables_n_dead_tup > 100000
        for: 30m
        labels:
          severity: warning
          team: database
        annotations:
          summary: "Table {{$labels.schemaname}}.{{$labels.tablename}} has high bloat"
          description: "{{$value}} dead tuples, consider vacuum"

  - name: postgresql_resources
    interval: 30s
    rules:
      # Disk space
      - alert: PostgreSQLLowDiskSpace
        expr: |
          (node_filesystem_avail_bytes{mountpoint="/var/lib/postgresql"} / 
           node_filesystem_size_bytes{mountpoint="/var/lib/postgresql"}) < 0.1
        for: 5m
        labels:
          severity: critical
          team: database
        annotations:
          summary: "Low disk space for PostgreSQL on {{$labels.instance}}"
          description: "Only {{$value | humanizePercentage}} disk space remaining"
      
      # Checkpoint frequency
      - alert: PostgreSQLFrequentCheckpoints
        expr: rate(pg_stat_bgwriter_checkpoints_req[5m]) > 0.1
        for: 10m
        labels:
          severity: warning
          team: database
        annotations:
          summary: "Frequent checkpoints on {{$labels.instance}}"
          description: "{{$value}} checkpoints per second, consider increasing checkpoint_segments"
```

## Production Deployment Examples

### Kubernetes DaemonSet for PostgreSQL Monitoring

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
  namespace: monitoring
data:
  config.yaml: |
    receivers:
      postgresql:
        endpoint: ${env:POSTGRES_ENDPOINT}
        username: ${env:POSTGRES_USER}
        password: ${env:POSTGRES_PASSWORD}
        databases: []  # Empty means all databases
        collection_interval: 15s
        tls:
          insecure_skip_verify: false
          ca_file: /etc/postgresql/ca.crt
    
    processors:
      batch:
        send_batch_size: 5000
        timeout: 10s
      
      memory_limiter:
        limit_mib: 512
        spike_limit_mib: 128
        check_interval: 5s
    
    exporters:
      prometheusremotewrite:
        endpoint: http://prometheus:9090/api/v1/write
        resource_to_telemetry_conversion:
          enabled: true
    
    service:
      pipelines:
        metrics:
          receivers: [postgresql]
          processors: [memory_limiter, batch]
          exporters: [prometheusremotewrite]

---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: otel-collector-postgresql
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: otel-collector-postgresql
  template:
    metadata:
      labels:
        app: otel-collector-postgresql
    spec:
      serviceAccountName: otel-collector
      containers:
      - name: otel-collector
        image: otel/opentelemetry-collector-contrib:0.88.0
        args: ["--config=/etc/otel/config.yaml"]
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 512Mi
        env:
        - name: POSTGRES_ENDPOINT
          valueFrom:
            fieldRef:
              fieldPath: status.hostIP
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: postgres-monitoring
              key: username
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-monitoring
              key: password
        volumeMounts:
        - name: config
          mountPath: /etc/otel
        - name: postgres-ca
          mountPath: /etc/postgresql
      volumes:
      - name: config
        configMap:
          name: otel-collector-config
      - name: postgres-ca
        secret:
          secretName: postgres-ca-cert
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
```

### Docker Compose for Development

```yaml
version: '3.8'

services:
  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.88.0
    command: ["--config=/etc/otel/config.yaml"]
    volumes:
      - ./otel-config.yaml:/etc/otel/config.yaml
      - ./certs:/etc/ssl/certs
    environment:
      - POSTGRES_ENDPOINT=postgres:5432
      - POSTGRES_USER=otel_monitor
      - POSTGRES_PASSWORD_FILE=/run/secrets/pg_password
      - ENVIRONMENT=development
    secrets:
      - pg_password
    networks:
      - monitoring
    depends_on:
      - postgres

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_PASSWORD_FILE=/run/secrets/pg_root_password
      - POSTGRES_INITDB_ARGS=--data-checksums
    volumes:
      - ./init-monitoring-user.sql:/docker-entrypoint-initdb.d/init.sql
      - postgres_data:/var/lib/postgresql/data
    secrets:
      - pg_root_password
    networks:
      - monitoring
    command: >
      postgres
      -c shared_preload_libraries='pg_stat_statements'
      -c pg_stat_statements.track=all
      -c track_io_timing=on

secrets:
  pg_password:
    file: ./secrets/pg_password.txt
  pg_root_password:
    file: ./secrets/pg_root_password.txt

volumes:
  postgres_data:

networks:
  monitoring:
```

## Troubleshooting Guide

### Common Issues

#### 1. Connection Issues

```bash
# Check collector logs
kubectl logs -n monitoring otel-collector-xxxxx | grep ERROR

# Test PostgreSQL connectivity
psql -h postgres-host -U otel_monitor -d postgres -c "SELECT version()"

# Verify permissions
psql -h postgres-host -U otel_monitor -d postgres -c "\du otel_monitor"

# Check network connectivity
telnet postgres-host 5432
nc -zv postgres-host 5432

# SSL/TLS debugging
openssl s_client -connect postgres-host:5432 -starttls postgres
```

#### 2. Metric Collection Issues

```bash
# Check collector metrics endpoint
curl http://otel-collector:8888/metrics | grep postgres

# Verify receiver status
curl http://otel-collector:13133/debug/tracez

# Check for query timeouts
kubectl logs -n monitoring otel-collector-xxxxx | grep -i timeout

# Validate custom queries
psql -h postgres-host -U otel_monitor -d postgres -f custom_queries.sql
```

#### 3. Performance Issues

```bash
# Check collector resource usage
kubectl top pod -n monitoring otel-collector-xxxxx

# Analyze slow queries
psql -h postgres-host -U postgres -d postgres -c "
SELECT 
    query,
    calls,
    total_exec_time,
    mean_exec_time,
    stddev_exec_time
FROM pg_stat_statements
WHERE mean_exec_time > 1000
ORDER BY mean_exec_time DESC
LIMIT 10;"

# Check for lock contention
psql -h postgres-host -U postgres -d postgres -c "
SELECT 
    blocked_locks.pid AS blocked_pid,
    blocked_activity.usename AS blocked_user,
    blocking_locks.pid AS blocking_pid,
    blocking_activity.usename AS blocking_user,
    blocked_activity.query AS blocked_statement,
    blocking_activity.query AS current_statement_in_blocking_process
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype
    AND blocking_locks.DATABASE IS NOT DISTINCT FROM blocked_locks.DATABASE
    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
    AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.GRANTED;"
```

### Debug Configuration

```yaml
# debug-collector-config.yaml
service:
  telemetry:
    logs:
      level: debug
      development: true
      encoding: json
      output_paths: ["stdout", "/var/log/otel/collector.log"]
      error_output_paths: ["stderr", "/var/log/otel/error.log"]
    
    metrics:
      level: detailed
      address: 0.0.0.0:8888
  
  extensions: [health_check, pprof, zpages]

extensions:
  pprof:
    endpoint: 0.0.0.0:6060
  
  zpages:
    endpoint: 0.0.0.0:55679
  
  health_check:
    endpoint: 0.0.0.0:13133
    path: /health
    check_collector_pipeline:
      enabled: true
      interval: 5s
      exporter_failure_threshold: 5

processors:
  debug:
    verbosity: detailed
    sampling_initial: 10
    sampling_thereafter: 100
```

### Recovery Procedures

```bash
#!/bin/bash
# recovery.sh - Collector recovery procedure

set -e

NAMESPACE="monitoring"
DEPLOYMENT="otel-collector"

echo "Starting OTel Collector recovery..."

# 1. Scale down
kubectl scale deployment/$DEPLOYMENT -n $NAMESPACE --replicas=0

# 2. Clear persistent data if corrupted
kubectl delete pvc -n $NAMESPACE -l app=otel-collector

# 3. Update configuration
kubectl create configmap otel-collector-config \
    --from-file=config.yaml \
    -n $NAMESPACE \
    --dry-run=client -o yaml | kubectl apply -f -

# 4. Scale up gradually
kubectl scale deployment/$DEPLOYMENT -n $NAMESPACE --replicas=1
sleep 30

# 5. Verify health
kubectl wait --for=condition=ready pod -l app=otel-collector -n $NAMESPACE --timeout=300s

# 6. Scale to desired replicas
kubectl scale deployment/$DEPLOYMENT -n $NAMESPACE --replicas=3

echo "Recovery completed"
```