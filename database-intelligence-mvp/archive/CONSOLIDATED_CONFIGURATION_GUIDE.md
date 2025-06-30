# Database Intelligence MVP - Consolidated Configuration Guide

## Overview

This guide consolidates all configuration knowledge from the Database Intelligence MVP archive, providing comprehensive examples and best practices for all deployment scenarios.

## Configuration Evolution

### Historical Context
The project evolved from 17+ overlapping configuration files to 3 streamlined configurations:

- **Original State**: Complex, redundant configurations causing maintenance overhead
- **Consolidation Phase**: Identified core patterns and eliminated duplication
- **Current State**: Minimal, simplified, and production configurations

### Core Configuration Files

#### 1. collector-minimal.yaml
Basic functionality for development and testing
```yaml
receivers:
  postgresql:
    endpoint: "${POSTGRES_HOST:localhost}:${POSTGRES_PORT:5432}"
    transport: tcp
    username: "${POSTGRES_USER:postgres}"
    password: "${POSTGRES_PASSWORD:postgres}"
    databases: ["${POSTGRES_DB:testdb}"]
    collection_interval: 30s

processors:
  memory_limiter:
    limit_mib: 256
  batch:
    timeout: 1s
    send_batch_size: 1000

exporters:
  debug:
    verbosity: detailed
  file:
    path: ./metrics.json

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, batch]
      exporters: [debug, file]
  telemetry:
    logs:
      level: debug
```

#### 2. collector-simplified.yaml  
Standard OTEL components for production readiness
```yaml
receivers:
  postgresql:
    endpoint: "${POSTGRES_HOST:localhost}:${POSTGRES_PORT:5432}"
    transport: tcp
    username: "${POSTGRES_USER:postgres}"
    password: "${POSTGRES_PASSWORD:postgres}"
    databases: ["${POSTGRES_DB:testdb}"]
    collection_interval: 10s
    
  mysql:
    endpoint: "${MYSQL_HOST:localhost}:${MYSQL_PORT:3306}"
    transport: tcp
    username: "${MYSQL_USER:root}"
    password: "${MYSQL_PASSWORD:mysql}"
    database: "${MYSQL_DB:testdb}"
    collection_interval: 10s

processors:
  memory_limiter:
    limit_mib: 512
    spike_limit_mib: 128
    check_interval: 5s
    
  batch:
    timeout: 1s
    send_batch_size: 1000
    send_batch_max_size: 1500
    
  resource:
    attributes:
      - key: service.name
        value: "database-intelligence-collector"
        action: upsert
      - key: environment
        value: "${ENVIRONMENT:development}"
        action: upsert

exporters:
  otlp:
    endpoint: "https://otlp.nr-data.net:4317"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"
    sending_queue:
      enabled: true
      num_consumers: 4
      queue_size: 100
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

service:
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp]
  telemetry:
    logs:
      level: info
    metrics:
      address: ":8888"
  extensions: [health_check]
```

#### 3. collector-production.yaml
Full feature set with all custom processors
```yaml
receivers:
  postgresql:
    endpoint: "${POSTGRES_HOST:localhost}:${POSTGRES_PORT:5432}"
    transport: tcp
    username: "${POSTGRES_USER:postgres}"
    password: "${POSTGRES_PASSWORD:postgres}"
    databases: ["${POSTGRES_DB:testdb}"]
    collection_interval: 10s
    tls:
      ca_file: "${POSTGRES_CA_FILE}"
      cert_file: "${POSTGRES_CERT_FILE}"
      key_file: "${POSTGRES_KEY_FILE}"
      
  mysql:
    endpoint: "${MYSQL_HOST:localhost}:${MYSQL_PORT:3306}"
    transport: tcp
    username: "${MYSQL_USER:root}"
    password: "${MYSQL_PASSWORD:mysql}"
    database: "${MYSQL_DB:testdb}"
    collection_interval: 10s
    tls:
      ca_file: "${MYSQL_CA_FILE}"
      cert_file: "${MYSQL_CERT_FILE}"
      key_file: "${MYSQL_KEY_FILE}"
      
  sqlquery:
    driver: postgres
    datasource: "postgresql://${POSTGRES_USER:postgres}:${POSTGRES_PASSWORD:postgres}@${POSTGRES_HOST:localhost}:${POSTGRES_PORT:5432}/${POSTGRES_DB:testdb}?sslmode=disable"
    queries:
      - sql: |
          SELECT 
            schemaname,
            tablename,
            n_tup_ins as inserts,
            n_tup_upd as updates,
            n_tup_del as deletes,
            n_tup_hot_upd as hot_updates,
            n_live_tup as live_tuples,
            n_dead_tup as dead_tuples
          FROM pg_stat_user_tables
        metrics:
          - metric_name: postgresql.table.operations
            value_column: inserts
            attribute_columns: [schemaname, tablename]
            static_attributes:
              operation: insert
          - metric_name: postgresql.table.operations  
            value_column: updates
            attribute_columns: [schemaname, tablename]
            static_attributes:
              operation: update

processors:
  memory_limiter:
    limit_mib: 1024
    spike_limit_mib: 256
    check_interval: 5s
    
  circuit_breaker:
    failure_threshold: 5
    timeout: 30s
    half_open_requests: 3
    databases:
      postgresql:
        max_cardinality: 10000
        rate_limit: 1000
      mysql:
        max_cardinality: 8000
        rate_limit: 800
        
  adaptive_sampler:
    rules:
      - name: "slow_queries"
        condition: "duration_ms > 1000"
        sampling_rate: 100
      - name: "error_queries"
        condition: "error_code != ''"
        sampling_rate: 100
      - name: "frequent_queries"
        condition: "query_count > 100"
        sampling_rate: 5
    default_sampling_rate: 10
    state_file: "/var/lib/otel/adaptive_sampler.state"
    deduplication:
      enabled: true
      cache_size: 10000
      ttl: 300s
      
  plan_extractor:
    enabled: true
    safe_mode: true
    timeout: 30s
    cache_size: 1000
    cache_ttl: 300s
    databases:
      postgresql:
        explain_format: json
        extract_costs: true
      mysql:
        explain_format: json
        extract_costs: true
    error_mode: "ignore"
    
  verification:
    pii_detection:
      enabled: true
      patterns:
        - credit_card
        - ssn  
        - email
        - phone
      replacement_char: "*"
      context_size: 10
    quality_checks:
      required_attributes:
        - db.system
        - db.name
        - host.name
      value_ranges:
        duration_ms: [0, 300000]
        connections: [0, 1000]
      cardinality_limits:
        query_text: 1000
        table_name: 500
    auto_tuning:
      enabled: true
      learning_period: "24h"
      adjustment_factor: 0.1
      
  batch:
    timeout: 1s
    send_batch_size: 1000
    send_batch_max_size: 1500
    
  resource:
    attributes:
      - key: service.name
        value: "database-intelligence-collector"
        action: upsert
      - key: service.version
        value: "${COLLECTOR_VERSION:1.0.0}"
        action: upsert
      - key: environment
        value: "${ENVIRONMENT:production}"
        action: upsert
      - key: datacenter
        value: "${DATACENTER:us-east-1}"
        action: upsert

exporters:
  otlp:
    endpoint: "https://otlp.nr-data.net:4317"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"
    compression: gzip
    sending_queue:
      enabled: true
      num_consumers: 8
      queue_size: 500
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    timeout: 30s
    
  prometheus:
    endpoint: ":9090"
    const_labels:
      service: "database-intelligence-collector"
      environment: "${ENVIRONMENT:production}"

extensions:
  health_check:
    endpoint: ":13133"
    path: "/health"
  pprof:
    endpoint: ":1777"
  zpages:
    endpoint: ":55679"

service:
  pipelines:
    metrics:
      receivers: [postgresql, mysql, sqlquery]
      processors: [memory_limiter, circuit_breaker, adaptive_sampler, 
                   plan_extractor, verification, resource, batch]
      exporters: [otlp, prometheus]
  telemetry:
    logs:
      level: info
      output_paths: ["/var/log/otel/collector.log"]
    metrics:
      address: ":8888"
      level: detailed
    resource:
      service.name: "database-intelligence-collector"
      service.version: "${COLLECTOR_VERSION:1.0.0}"
  extensions: [health_check, pprof, zpages]
```

## Configuration Overlay System

### Environment-Specific Configurations

#### Base Configuration Template
```yaml
# config/base.yaml
defaults: &defaults
  service:
    telemetry:
      logs:
        level: info
      metrics:
        address: ":8888"
  processors:
    memory_limiter:
      check_interval: 5s
    batch:
      timeout: 1s
```

#### Development Overlay
```yaml
# config/development.yaml
<<: *defaults
service:
  telemetry:
    logs:
      level: debug
processors:
  memory_limiter:
    limit_mib: 256
exporters:
  debug:
    verbosity: detailed
```

#### Staging Overlay  
```yaml
# config/staging.yaml  
<<: *defaults
processors:
  memory_limiter:
    limit_mib: 512
  circuit_breaker:
    failure_threshold: 3
exporters:
  otlp:
    endpoint: "https://staging-otlp.nr-data.net:4317"
```

#### Production Overlay
```yaml
# config/production.yaml
<<: *defaults
processors:
  memory_limiter:
    limit_mib: 1024
  circuit_breaker:
    failure_threshold: 5
exporters:
  otlp:
    endpoint: "https://otlp.nr-data.net:4317"
    compression: gzip
```

## Custom Processor Configurations

### Adaptive Sampler Advanced Configuration
```yaml
adaptive_sampler:
  # Rule-based sampling
  rules:
    - name: "critical_errors"
      condition: "severity = 'CRITICAL' or error_code LIKE 'FATAL%'"
      sampling_rate: 100
      priority: 1
      
    - name: "slow_queries"  
      condition: "duration_ms > 5000"
      sampling_rate: 100
      priority: 2
      
    - name: "frequent_operations"
      condition: "operation_count > 1000 AND duration_ms < 100"
      sampling_rate: 1
      priority: 3
      
    - name: "business_critical_tables"
      condition: "table_name IN ('orders', 'payments', 'users')"
      sampling_rate: 50
      priority: 4
      
  # Default sampling for unmatched metrics
  default_sampling_rate: 10
  
  # State persistence
  state_file: "/var/lib/otel/adaptive_sampler.state"
  state_sync_interval: "30s"
  
  # Deduplication
  deduplication:
    enabled: true
    algorithm: "lru"
    cache_size: 10000
    ttl: 300s
    key_attributes: ["query_hash", "table_name"]
    
  # Performance tuning
  batch_size: 1000
  processing_timeout: "5s"
  rule_evaluation_cache: 1000
```

### Circuit Breaker Advanced Configuration
```yaml
circuit_breaker:
  # Global settings
  default_failure_threshold: 5
  default_timeout: 30s
  default_half_open_requests: 3
  
  # Per-database configuration
  databases:
    postgresql:
      failure_threshold: 10
      timeout: 60s
      half_open_requests: 5
      max_cardinality: 15000
      rate_limit: 2000
      error_patterns:
        - "connection refused"
        - "timeout"
        - "too many connections"
      
    mysql:
      failure_threshold: 8
      timeout: 45s  
      half_open_requests: 3
      max_cardinality: 12000
      rate_limit: 1500
      error_patterns:
        - "connection refused"
        - "max_connections"
        - "lock wait timeout"
        
  # Advanced features
  adaptive_timeout:
    enabled: true
    min_timeout: 10s
    max_timeout: 300s
    adjustment_factor: 1.5
    
  self_healing:
    enabled: true
    recovery_check_interval: 10s
    gradual_recovery: true
    recovery_steps: [25, 50, 75, 100]
    
  monitoring:
    emit_state_metrics: true
    emit_timing_metrics: true
    metric_labels: ["database", "state"]
```

### Plan Extractor Advanced Configuration
```yaml
plan_extractor:
  # Safety settings
  enabled: true
  safe_mode: true
  timeout: 30s
  max_plan_size: 1048576  # 1MB
  
  # Caching
  cache_size: 2000
  cache_ttl: 600s
  cache_hit_ratio_target: 0.8
  
  # Database-specific settings
  databases:
    postgresql:
      explain_format: json
      explain_options:
        analyze: false
        verbose: false  
        costs: true
        buffers: false
      extract_costs: true
      extract_rows: true
      extract_operations: true
      plan_hash_algorithm: "sha256"
      
    mysql:
      explain_format: json
      explain_options:
        extended: false
        partitions: true
      extract_costs: true
      extract_rows: true
      extract_operations: true
      plan_hash_algorithm: "sha256"
      
  # Processing options
  error_mode: "ignore"  # ignore, warn, fail
  parallel_processing: true
  max_concurrent_plans: 10
  
  # Output configuration
  attribute_mapping:
    plan_hash: "plan.hash"
    total_cost: "plan.total_cost"
    estimated_rows: "plan.estimated_rows"
    actual_rows: "plan.actual_rows"
    operation_type: "plan.operation"
    
  # Performance tuning
  processing_queue_size: 1000
  batch_processing: true
  batch_size: 100
```

### Verification Processor Advanced Configuration
```yaml
verification:
  # PII Detection
  pii_detection:
    enabled: true
    patterns:
      credit_card:
        regex: '\b(?:\d{4}[-\s]?){3}\d{4}\b'
        replacement: "****-****-****-****"
        confidence_threshold: 0.9
      ssn:
        regex: '\b\d{3}-\d{2}-\d{4}\b'
        replacement: "***-**-****"
        confidence_threshold: 0.95
      email:
        regex: '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b'
        replacement: "****@****.***"
        confidence_threshold: 0.8
      phone:
        regex: '\b\d{3}-\d{3}-\d{4}\b'
        replacement: "***-***-****"
        confidence_threshold: 0.9
    context_analysis:
      enabled: true
      context_size: 20
      keywords: ["password", "ssn", "social", "card", "account"]
      
  # Data Quality Checks
  quality_checks:
    required_attributes:
      - db.system
      - db.name
      - host.name
      - service.name
    attribute_validation:
      db.system:
        allowed_values: ["postgresql", "mysql"]
      duration_ms:
        min_value: 0
        max_value: 300000
      connections:
        min_value: 0
        max_value: 1000
    value_ranges:
      cpu_usage: [0, 100]
      memory_usage: [0, 1073741824]  # 1GB
    cardinality_limits:
      query_text: 2000
      table_name: 1000
      user_name: 500
      
  # Auto-tuning
  auto_tuning:
    enabled: true
    learning_period: "24h"
    adjustment_factor: 0.1
    adaptation_triggers:
      - high_cardinality
      - processing_delays
      - memory_pressure
    metrics_collection:
      enabled: true
      collection_interval: "5m"
      
  # Performance optimization
  processing:
    parallel_workers: 4
    queue_size: 5000
    batch_processing: true
    batch_size: 500
    timeout: "10s"
    
  # Compliance and auditing
  audit:
    enabled: true
    log_pii_detections: true
    log_quality_failures: true
    audit_log_path: "/var/log/otel/verification_audit.log"
    retention_period: "30d"
```

## Environment Variables

### Required Variables
```bash
# Database connections
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="5432"
export POSTGRES_USER="postgres"
export POSTGRES_PASSWORD="postgres"
export POSTGRES_DB="testdb"

export MYSQL_HOST="localhost"
export MYSQL_PORT="3306"
export MYSQL_USER="root"
export MYSQL_PASSWORD="mysql"
export MYSQL_DB="testdb"

# New Relic integration
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"

# Environment identification
export ENVIRONMENT="production"
export DATACENTER="us-east-1"
export COLLECTOR_VERSION="1.0.0"
```

### Optional Variables
```bash
# TLS configuration
export POSTGRES_CA_FILE="/etc/ssl/certs/postgres-ca.crt"
export POSTGRES_CERT_FILE="/etc/ssl/certs/postgres-client.crt"
export POSTGRES_KEY_FILE="/etc/ssl/private/postgres-client.key"

export MYSQL_CA_FILE="/etc/ssl/certs/mysql-ca.crt"
export MYSQL_CERT_FILE="/etc/ssl/certs/mysql-client.crt"
export MYSQL_KEY_FILE="/etc/ssl/private/mysql-client.key"

# Advanced settings
export OTEL_LOG_LEVEL="info"
export OTEL_MEMORY_LIMIT_MIB="1024"
export OTEL_BATCH_SIZE="1000"
export OTEL_BATCH_TIMEOUT="1s"
```

## Docker Compose Configuration

### Multi-Environment Support
```yaml
# docker-compose.yaml
version: '3.8'

services:
  collector:
    image: database-intelligence-collector:latest
    profiles:
      - development
      - staging  
      - production
    environment:
      - ENVIRONMENT=${ENVIRONMENT:-development}
    volumes:
      - ./config:/etc/otel:ro
      - collector-data:/var/lib/otel
    ports:
      - "8888:8888"   # Metrics endpoint
      - "13133:13133" # Health check
    depends_on:
      - postgres
      - mysql
      
  postgres:
    image: postgres:13
    profiles: [development, staging]
    environment:
      POSTGRES_DB: testdb
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./tests/e2e/sql/postgres-init.sql:/docker-entrypoint-initdb.d/init.sql
    ports:
      - "5432:5432"
      
  mysql:
    image: mysql:8.0
    profiles: [development, staging]
    environment:
      MYSQL_DATABASE: testdb
      MYSQL_USER: mysql
      MYSQL_PASSWORD: mysql
      MYSQL_ROOT_PASSWORD: mysql
    volumes:
      - mysql-data:/var/lib/mysql
      - ./tests/e2e/sql/mysql-init.sql:/docker-entrypoint-initdb.d/init.sql
    ports:
      - "3306:3306"

volumes:
  collector-data:
  postgres-data:
  mysql-data:
```

### Profile Usage
```bash
# Development with local databases
docker-compose --profile development up

# Staging environment
docker-compose --profile staging up

# Production (external databases)
docker-compose --profile production up
```

## Kubernetes Configuration

### Helm Chart Values
```yaml
# values.yaml
replicaCount: 3

image:
  repository: database-intelligence-collector
  tag: "1.0.0"
  pullPolicy: IfNotPresent

config:
  environment: production
  logLevel: info
  memoryLimit: 1024Mi
  
resources:
  limits:
    memory: 1Gi
    cpu: 500m
  requests:
    memory: 512Mi
    cpu: 250m
    
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
  
database:
  postgresql:
    host: postgres.database.svc.cluster.local
    port: 5432
    ssl: true
  mysql:
    host: mysql.database.svc.cluster.local  
    port: 3306
    ssl: true
    
monitoring:
  serviceMonitor:
    enabled: true
  grafana:
    dashboardsEnabled: true
```

### ConfigMap Template
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: collector-config
data:
  collector.yaml: |
    {{ .Values.config | toYaml | indent 4 }}
```

## Validation and Testing

### Configuration Validation
```bash
# Validate configuration syntax
otelcol-builder validate --config collector.yaml

# Test configuration with dry-run
./collector --config collector.yaml --dry-run

# Validate against schema
yq eval-all 'select(fileIndex == 0) * select(fileIndex == 1)' \
  schema.yaml collector.yaml
```

### Integration Testing
```bash
# Test with minimal configuration
export OTEL_CONFIG=collector-minimal.yaml
./tests/e2e/run-e2e-tests.sh

# Test with full configuration  
export OTEL_CONFIG=collector-production.yaml
./tests/e2e/run-comprehensive-e2e-tests.sh
```

## Migration Guide

### From Legacy Configurations
1. **Identify current configuration files**
2. **Map to new consolidated structure**
3. **Update environment variables**
4. **Test with validation scripts**
5. **Deploy with gradual rollout**

### Configuration Mapping
```bash
# Legacy to new mapping
collector-dev.yaml         → collector-minimal.yaml
collector-simple.yaml      → collector-simplified.yaml  
collector-production.yaml  → collector-production.yaml (updated)
collector-ha.yaml          → Use Kubernetes scaling
collector-experimental.yaml → Custom processor configs
```

---

**Document Status**: Production Ready  
**Last Updated**: 2025-06-30  
**Coverage**: Complete consolidation of all configuration examples and best practices