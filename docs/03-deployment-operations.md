# PostgreSQL Unified Collector - Deployment & Operations Guide

## Table of Contents
1. [Deployment Overview](#deployment-overview)
2. [Installation Methods](#installation-methods)
3. [Configuration Reference](#configuration-reference)
4. [Deployment Patterns](#deployment-patterns)
5. [Cloud Provider Deployments](#cloud-provider-deployments)
6. [Monitoring & Operations](#monitoring-operations)
7. [Security Best Practices](#security-best-practices)
8. [Troubleshooting Guide](#troubleshooting-guide)

## Deployment Overview

The PostgreSQL Unified Collector supports multiple deployment patterns to fit your infrastructure:

- **Binary Installation**: Direct installation on Linux systems
- **Container Deployment**: Docker/Kubernetes native deployments
- **Infrastructure Agent Integration**: Drop-in replacement for nri-postgresql
- **Cloud-Native**: Optimized for RDS, Cloud SQL, and managed services

### Deployment Decision Tree
```
Is this for Kubernetes?
├─ Yes → Use Helm Chart or Kubernetes manifests
└─ No → Is this for containers?
    ├─ Yes → Use Docker Compose
    └─ No → Is New Relic Infrastructure Agent installed?
        ├─ Yes → Deploy as NRI integration
        └─ No → Use systemd service
```

## Installation Methods

### 1. Binary Installation

#### Download Pre-built Binary
```bash
# Latest release
curl -LO https://github.com/newrelic/postgres-unified-collector/releases/latest/download/postgres-unified-collector_linux_amd64.tar.gz
tar -xzf postgres-unified-collector_linux_amd64.tar.gz
sudo mv postgres-unified-collector /usr/local/bin/
sudo chmod +x /usr/local/bin/postgres-unified-collector

# Verify installation
postgres-unified-collector --version
```

#### Systemd Service Installation
```bash
# Create service user
sudo useradd -r -s /bin/false postgres-collector

# Create directories
sudo mkdir -p /etc/postgres-collector
sudo mkdir -p /var/lib/postgres-collector
sudo chown postgres-collector:postgres-collector /var/lib/postgres-collector

# Install service file
sudo tee /etc/systemd/system/postgres-collector.service > /dev/null <<EOF
[Unit]
Description=PostgreSQL Unified Collector
Documentation=https://docs.newrelic.com/postgres-unified-collector
After=network.target

[Service]
Type=simple
User=postgres-collector
Group=postgres-collector
ExecStart=/usr/local/bin/postgres-unified-collector --config /etc/postgres-collector/config.toml
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=postgres-collector

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/postgres-collector

# Resource limits
LimitNOFILE=65536
MemoryLimit=512M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable postgres-collector
sudo systemctl start postgres-collector
```

### 2. New Relic Infrastructure Agent Integration

#### As Drop-in Replacement
```bash
# Backup existing integration
sudo mv /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql \
        /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql.backup

# Install unified collector
sudo cp postgres-unified-collector /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql
sudo chmod +x /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql

# Configuration remains the same
# /etc/newrelic-infra/integrations.d/postgresql-config.yml
```

#### New Installation
```yaml
# /etc/newrelic-infra/integrations.d/postgresql-config.yml
integrations:
  - name: nri-postgresql
    env:
      # Connection settings
      HOSTNAME: localhost
      PORT: 5432
      USERNAME: ${POSTGRES_USER}
      PASSWORD: ${POSTGRES_PASSWORD}
      DATABASE: postgres
      ENABLE_SSL: true
      
      # Collection settings
      METRICS: true
      INVENTORY: true
      COLLECTION_LIST: '{"postgres": {"schemas": ["public", "app"]}}'
      TIMEOUT: 30
      
      # Query monitoring
      QUERY_MONITORING: true
      QUERY_MONITORING_COUNT_THRESHOLD: 20
      QUERY_MONITORING_RESPONSE_TIME_THRESHOLD: 500
      
      # Extended features
      ENABLE_EXTENDED_METRICS: true
      ENABLE_ASH: true
      ASH_SAMPLE_INTERVAL: 1s
    
    interval: 60s
```

### 3. Docker Deployment

#### Standalone Container
```bash
docker run -d \
  --name postgres-collector \
  -e POSTGRES_HOST=postgres.example.com \
  -e POSTGRES_USER=monitoring \
  -e POSTGRES_PASSWORD=${POSTGRES_PASSWORD} \
  -e NEW_RELIC_LICENSE_KEY=${NR_LICENSE_KEY} \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp.nr-data.net:4317 \
  -v $(pwd)/config.toml:/etc/postgres-collector/config.toml:ro \
  newrelic/postgres-unified-collector:latest
```

#### Docker Compose
```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: mysecretpassword
      POSTGRES_DB: myapp
    volumes:
      - postgres_data:/var/lib/postgresql/data
    
  postgres-collector:
    image: newrelic/postgres-unified-collector:latest
    depends_on:
      - postgres
    environment:
      POSTGRES_COLLECTOR_MODE: hybrid
      POSTGRES_HOST: postgres
      POSTGRES_PORT: 5432
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: mysecretpassword
      POSTGRES_DATABASE: myapp
      
      # New Relic settings
      NEW_RELIC_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
      
      # OpenTelemetry settings
      OTEL_EXPORTER_OTLP_ENDPOINT: ${OTEL_ENDPOINT:-http://otel-collector:4317}
      
      # Features
      ENABLE_EXTENDED_METRICS: "true"
      ENABLE_ASH: "true"
    volumes:
      - ./config.toml:/etc/postgres-collector/config.toml:ro
    restart: unless-stopped

volumes:
  postgres_data:
```

### 4. Kubernetes Deployment

#### Helm Installation
```bash
# Add repository
helm repo add newrelic https://helm.newrelic.com
helm repo update

# Install with custom values
helm install postgres-collector newrelic/postgres-unified-collector \
  --set postgres.host=postgres-primary.database.svc.cluster.local \
  --set postgres.credentials.existingSecret=postgres-credentials \
  --set newrelic.licenseKey=${NEW_RELIC_LICENSE_KEY} \
  --set mode=hybrid
```

#### Manual Kubernetes Deployment
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-collector-config
  namespace: monitoring
data:
  config.toml: |
    [collector]
    mode = "hybrid"
    collection_interval_secs = 60
    
    [postgres]
    host = "postgres-primary.database.svc.cluster.local"
    port = 5432
    databases = ["postgres", "app"]
    
    [features]
    enable_extended_metrics = true
    enable_ash = true
    ash_sample_interval_ms = 1000
    
    [export.nri]
    enabled = true
    
    [export.otlp]
    enabled = true
    endpoint = "http://otel-collector.monitoring:4317"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres-collector
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres-collector
  template:
    metadata:
      labels:
        app: postgres-collector
    spec:
      serviceAccountName: postgres-collector
      containers:
      - name: collector
        image: newrelic/postgres-unified-collector:latest
        env:
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: postgres-credentials
              key: username
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-credentials
              key: password
        - name: NEW_RELIC_LICENSE_KEY
          valueFrom:
            secretKeyRef:
              name: newrelic-license
              key: key
        volumeMounts:
        - name: config
          mountPath: /etc/postgres-collector
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
      volumes:
      - name: config
        configMap:
          name: postgres-collector-config
```

## Configuration Reference

### Complete Configuration File
```toml
# /etc/postgres-collector/config.toml

[collector]
# Collector mode: "nri", "otel", or "hybrid"
mode = "hybrid"

# Collection interval in seconds
collection_interval_secs = 60

# Number of worker threads
worker_threads = 4

# Enable debug logging
debug = false

[postgres]
# Connection settings
host = "localhost"
port = 5432
username = "postgres"
password = "${POSTGRES_PASSWORD}"  # Environment variable substitution
databases = ["postgres", "app_db"]

# Connection pool settings
max_connections = 5
min_connections = 2
connect_timeout_secs = 30
idle_timeout_secs = 600

# SSL settings
ssl_mode = "prefer"  # disable, prefer, require, verify-ca, verify-full
ssl_cert = "/path/to/client-cert.pem"
ssl_key = "/path/to/client-key.pem"
ssl_root_cert = "/path/to/ca-cert.pem"

[features]
# Enable extended metrics beyond OHI compatibility
enable_extended_metrics = true

# Active Session History
enable_ash = true
ash_sample_interval_ms = 1000
ash_retention_minutes = 60

# eBPF support (requires CAP_SYS_ADMIN)
enable_ebpf = false

# Query plan collection
enable_plan_collection = true
plan_collection_timeout_ms = 100

[ohi_compatibility]
# Maintain exact OHI metric names
preserve_metric_names = true

# Query monitoring thresholds (OHI compatible)
query_monitoring_count_threshold = 20
query_monitoring_response_time_threshold = 500

# Query text length limit
max_query_length = 4095

[sampling]
# Adaptive sampling configuration
enabled = true
base_sample_rate = 1.0

[[sampling.rules]]
name = "slow_queries"
condition = "avg_elapsed_time_ms > 1000"
sample_rate = 1.0

[[sampling.rules]]
name = "high_frequency"
condition = "execution_count > 1000"
sample_rate = 0.1

[export.nri]
# New Relic Infrastructure export
enabled = true
entity_key = "${HOSTNAME}:${PORT}"
integration_name = "com.newrelic.postgresql"
integration_version = "2.0.0"

[export.otlp]
# OpenTelemetry export
enabled = true
endpoint = "${OTEL_EXPORTER_OTLP_ENDPOINT}"
protocol = "grpc"  # grpc or http
compression = "gzip"
timeout_secs = 30

# Headers for authentication
[export.otlp.headers]
api-key = "${NEW_RELIC_API_KEY}"

# Resource attributes
[export.otlp.resource_attributes]
service.name = "postgresql"
service.namespace = "${ENVIRONMENT}"
service.version = "${VERSION}"

[logging]
# Log level: trace, debug, info, warn, error
level = "info"

# Log format: json or text
format = "json"

# Log output: stdout, stderr, or file path
output = "stdout"

# Log file settings (if output is a file)
max_size_mb = 100
max_backups = 5
max_age_days = 7
compress = true

[health]
# Health check endpoint
enabled = true
port = 8080
path = "/health"

# Readiness check settings
check_database_connection = true
check_extension_availability = true
```

### Environment Variables

All configuration values can be overridden with environment variables:

```bash
# Pattern: POSTGRES_COLLECTOR_<SECTION>_<KEY>
export POSTGRES_COLLECTOR_MODE=hybrid
export POSTGRES_COLLECTOR_POSTGRES_HOST=postgres.example.com
export POSTGRES_COLLECTOR_POSTGRES_PASSWORD=secret
export POSTGRES_COLLECTOR_EXPORT_NRI_ENABLED=true
export POSTGRES_COLLECTOR_EXPORT_OTLP_ENDPOINT=https://otlp.nr-data.net:4317
```

## Deployment Patterns

### 1. Sidecar Pattern (Kubernetes)
```yaml
spec:
  containers:
  - name: postgres
    image: postgres:15
    # PostgreSQL container config
    
  - name: collector
    image: newrelic/postgres-unified-collector:latest
    env:
    - name: POSTGRES_HOST
      value: "localhost"  # Sidecar connection
    # Shared network namespace
```

### 2. DaemonSet Pattern (Node-wide monitoring)
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: postgres-collector
spec:
  selector:
    matchLabels:
      app: postgres-collector
  template:
    spec:
      hostNetwork: true  # Access all PostgreSQL instances on node
      containers:
      - name: collector
        image: newrelic/postgres-unified-collector:latest
        env:
        - name: DISCOVER_POSTGRES
          value: "true"
        securityContext:
          privileged: true  # For eBPF
```

### 3. Centralized Collector
Deploy a single collector instance that monitors multiple PostgreSQL instances:

```toml
# Multi-instance configuration
[[postgres.instances]]
name = "primary"
host = "postgres-primary.example.com"
port = 5432
databases = ["prod_db"]

[[postgres.instances]]
name = "replica"
host = "postgres-replica.example.com"
port = 5432
databases = ["prod_db"]
```

## Cloud Provider Deployments

### AWS RDS
```yaml
# RDS-optimized configuration
[postgres]
host = "myinstance.abc123.us-east-1.rds.amazonaws.com"
port = 5432

[features]
# RDS doesn't support custom extensions
enable_ebpf = false
rds_mode = true  # Enables RDS-specific adaptations

[aws]
# Use IAM authentication
use_iam_auth = true
region = "us-east-1"
```

### Google Cloud SQL
```yaml
# Cloud SQL with Cloud Run
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: postgres-collector
  annotations:
    run.googleapis.com/cloudsql-instances: PROJECT:REGION:INSTANCE
spec:
  template:
    spec:
      containers:
      - image: newrelic/postgres-unified-collector:latest
        env:
        - name: POSTGRES_HOST
          value: "/cloudsql/PROJECT:REGION:INSTANCE"
```

### Azure Database for PostgreSQL
```toml
[postgres]
host = "myserver.postgres.database.azure.com"
username = "user@myserver"

[azure]
# Use managed identity
use_managed_identity = true
```

## Monitoring & Operations

### 1. Health Checks

#### HTTP Health Endpoint
```bash
# Liveness check
curl http://localhost:8080/health

# Response
{
  "status": "healthy",
  "timestamp": "2024-01-20T10:30:00Z",
  "version": "1.0.0",
  "uptime_seconds": 3600
}

# Readiness check
curl http://localhost:8080/ready

# Response
{
  "ready": true,
  "checks": {
    "database_connection": "ok",
    "extension_pg_stat_statements": "available",
    "extension_pg_wait_sampling": "unavailable"
  }
}
```

#### Prometheus Metrics
```bash
# Metrics endpoint (if enabled)
curl http://localhost:8080/metrics

# Key metrics
postgres_collector_up{version="1.0.0"} 1
postgres_collector_collections_total{status="success"} 1234
postgres_collector_collections_duration_seconds{quantile="0.99"} 0.5
postgres_collector_metrics_collected_total{type="slow_queries"} 5678
```

### 2. Logging

#### Structured Logging
```json
{
  "timestamp": "2024-01-20T10:30:00Z",
  "level": "info",
  "msg": "Collection completed",
  "duration_ms": 250,
  "metrics_collected": {
    "slow_queries": 45,
    "wait_events": 120,
    "blocking_sessions": 3
  },
  "trace_id": "abc123"
}
```

#### Debug Mode
```bash
# Enable debug logging
RUST_LOG=debug postgres-unified-collector --config config.toml

# Enable query logging
RUST_LOG=postgres_unified_collector::query_engine=trace
```

### 3. Performance Monitoring

#### Key Metrics to Monitor
- **Collection Duration**: Should be < 1 second
- **Memory Usage**: Should be < 256MB typical
- **CPU Usage**: Should be < 5% of one core
- **Connection Pool**: Monitor active/idle connections
- **Error Rate**: Should be < 0.1%

#### Alerting Rules
```yaml
# Prometheus alerting rules
groups:
- name: postgres_collector
  rules:
  - alert: CollectorDown
    expr: up{job="postgres-collector"} == 0
    for: 5m
    
  - alert: CollectionSlow
    expr: postgres_collector_collection_duration_seconds{quantile="0.99"} > 2
    for: 10m
    
  - alert: HighErrorRate
    expr: rate(postgres_collector_errors_total[5m]) > 0.1
    for: 5m
```

## Security Best Practices

### 1. PostgreSQL User Setup
```sql
-- Create monitoring user with minimal privileges
CREATE USER monitoring WITH PASSWORD 'secure_password';
GRANT pg_monitor TO monitoring;
GRANT CONNECT ON DATABASE myapp TO monitoring;
GRANT USAGE ON SCHEMA public TO monitoring;

-- For pg_stat_statements
GRANT SELECT ON pg_stat_statements TO monitoring;
```

### 2. Network Security
- Use SSL/TLS for all connections
- Implement network segmentation
- Use private endpoints in cloud environments
- Restrict collector access to PostgreSQL only

### 3. Secrets Management
```bash
# Use external secret managers
export POSTGRES_PASSWORD=$(vault kv get -field=password secret/postgres)

# Kubernetes secrets
kubectl create secret generic postgres-credentials \
  --from-literal=username=monitoring \
  --from-literal=password=${POSTGRES_PASSWORD}

# AWS Secrets Manager
[aws]
secrets_manager_secret_id = "arn:aws:secretsmanager:region:account:secret:name"
```

### 4. Runtime Security
```yaml
# Kubernetes SecurityContext
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL
    add:
    - NET_BIND_SERVICE  # For health endpoint
```

## Troubleshooting Guide

### Common Issues

#### 1. Connection Failures
```bash
# Test connection
postgres-unified-collector test --config config.toml

# Common causes:
# - Incorrect credentials
# - Network connectivity
# - SSL certificate issues
# - pg_hba.conf restrictions
```

#### 2. Missing Metrics
```sql
-- Check extension availability
SELECT name, installed_version 
FROM pg_available_extensions 
WHERE name IN ('pg_stat_statements', 'pg_wait_sampling');

-- Verify permissions
\du monitoring
```

#### 3. High Memory Usage
```toml
# Tune collection settings
[postgres]
max_connections = 2  # Reduce pool size

[sampling]
base_sample_rate = 0.5  # Sample 50% of metrics

[features]
ash_retention_minutes = 30  # Reduce retention
```

#### 4. Export Failures
```bash
# Test OTLP connectivity
curl -X POST ${OTEL_ENDPOINT}/v1/metrics \
  -H "api-key: ${API_KEY}" \
  -H "Content-Type: application/json"

# Check NRI output
postgres-unified-collector --mode=nri --dry-run
```

### Debug Commands
```bash
# Validate configuration
postgres-unified-collector validate --config config.toml

# Test specific component
postgres-unified-collector test --component=postgres --config config.toml

# Generate support bundle
postgres-unified-collector support-bundle --output=bundle.tar.gz
```

## Maintenance

### Upgrades
```bash
# Rolling upgrade (Kubernetes)
kubectl set image deployment/postgres-collector \
  collector=newrelic/postgres-unified-collector:v1.1.0

# Binary upgrade
systemctl stop postgres-collector
cp /usr/local/bin/postgres-unified-collector /usr/local/bin/postgres-unified-collector.backup
wget -O /usr/local/bin/postgres-unified-collector https://...
systemctl start postgres-collector
```

### Backup Configuration
```bash
# Backup current configuration
tar -czf postgres-collector-backup-$(date +%Y%m%d).tar.gz \
  /etc/postgres-collector/ \
  /var/lib/postgres-collector/
```

## Next Steps

- [Metrics Reference](04-metrics-reference.md) - Complete list of collected metrics
- [Migration Guide](05-migration-guide.md) - Upgrading from nri-postgresql
- [Architecture Overview](01-architecture-overview.md) - System design details