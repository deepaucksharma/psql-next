# Comprehensive Deployment Guide

This guide covers all deployment scenarios for the Database Intelligence Collector, from development to production, including feature detection setup and troubleshooting.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Development Deployment](#development-deployment)
3. [Production Deployment](#production-deployment)
4. [Feature Detection Setup](#feature-detection-setup)
5. [Kubernetes Deployment](#kubernetes-deployment)
6. [Docker Deployment](#docker-deployment)
7. [Configuration Management](#configuration-management)
8. [Monitoring & Troubleshooting](#monitoring--troubleshooting)
9. [Security Considerations](#security-considerations)
10. [Migration from OHI](#migration-from-ohi)

## Prerequisites

### System Requirements

- **Memory**: 512MB minimum, 2GB recommended
- **CPU**: 2 cores minimum, 4 cores recommended
- **Disk**: 10GB for logs and temporary data
- **Network**: Outbound HTTPS to New Relic endpoints

### Software Requirements

- **Go**: 1.21+ (for building from source)
- **Docker**: 20.10+ (for containerized deployment)
- **Kubernetes**: 1.24+ (for K8s deployment)
- **PostgreSQL**: 12+ or MySQL 5.7+

### Database Prerequisites

#### PostgreSQL Setup

```sql
-- Create monitoring user
CREATE USER otel_monitor WITH PASSWORD 'secure_password';
GRANT pg_monitor TO otel_monitor;
GRANT SELECT ON pg_stat_statements TO otel_monitor;

-- Enable required extensions (requires superuser)
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Configure PostgreSQL for optimal monitoring
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET track_io_timing = on;
ALTER SYSTEM SET track_functions = 'all';
SELECT pg_reload_conf();
```

#### MySQL Setup

```sql
-- Create monitoring user
CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'secure_password';
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';

-- Enable Performance Schema (if not enabled)
SET GLOBAL performance_schema = ON;
SET GLOBAL slow_query_log = ON;
```

## Development Deployment

### Local Development

```bash
# Clone repository
git clone https://github.com/database-intelligence-mvp/database-intelligence-mvp.git
cd database-intelligence-mvp

# Install dependencies
make install-tools
make deps

# Build collector
make build

# Run with development config
./dist/database-intelligence-collector --config=configs/postgresql-maximum-extraction.yaml
```

### Development Docker Compose

```bash
# Start full development stack
docker-compose -f docker-compose.dev.yml up -d

# View logs
docker-compose -f docker-compose.dev.yml logs -f collector

# Stop stack
docker-compose -f docker-compose.dev.yml down
```

### Development Configuration

```yaml
# configs/postgresql-maximum-extraction.yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
  zpages:
    endpoint: 0.0.0.0:55679

receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    databases: [testdb]
    collection_interval: 10s  # Faster for development

processors:
  batch:
    timeout: 5s  # Faster batching for development

exporters:
  debug:
    verbosity: detailed  # See all data
  
service:
  telemetry:
    logs:
      level: debug  # Verbose logging
```

## Production Deployment

### Production Build

```bash
# Build optimized binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w" \
  -o dist/database-intelligence-collector .

# Build Docker image
docker build -t database-intelligence:v2.0.0 \
  --build-arg VERSION=v2.0.0 \
  --build-arg BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ) \
  -f deployments/docker/Dockerfile .
```

### Production Configuration

```yaml
# configs/postgresql-maximum-extraction.yaml
receivers:
  enhancedsql/postgresql:
    driver: postgres
    datasource: "${POSTGRES_DSN}"
    feature_detection:
      enabled: true
      cache_duration: 1h  # Longer cache in production
    collection_interval: 60s
    
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    databases: ${POSTGRES_DATABASES}
    tls:
      ca_file: /etc/ssl/certs/postgres-ca.crt
      insecure_skip_verify: false

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 80
    spike_limit_percentage: 25
    
  circuitbreaker:
    failure_threshold: 5
    timeout: 30s
    memory_threshold_mb: 400
    cpu_threshold_percent: 80
    
  adaptivesampler:
    in_memory_only: true
    rules:
      - name: slow_queries
        condition: 'duration_ms > 1000'
        sample_rate: 1.0
      - name: errors
        condition: 'error != nil'
        sample_rate: 1.0
    default_sample_rate: 0.1
    
  batch:
    timeout: 30s
    send_batch_size: 1000

exporters:
  otlp/newrelic:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    compression: gzip
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 60s
    sending_queue:
      enabled: true
      num_consumers: 4
      queue_size: 1000

service:
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [postgresql, enhancedsql/postgresql]
      processors: [memory_limiter, circuitbreaker, adaptivesampler, batch]
      exporters: [otlp/newrelic]
  telemetry:
    logs:
      level: info
      encoding: json
    metrics:
      level: detailed
      address: 0.0.0.0:8888
```

### Environment Variables

```bash
# .env.production
# Database Connections
POSTGRES_HOST=prod-db.example.com
POSTGRES_PORT=5432
POSTGRES_USER=otel_monitor
POSTGRES_PASSWORD=<vault:secret/database/postgres#password>
POSTGRES_DATABASES=["production", "analytics"]
POSTGRES_DSN=host=prod-db.example.com port=5432 user=otel_monitor password=xxx dbname=production sslmode=require

# New Relic
NEW_RELIC_LICENSE_KEY=<vault:secret/newrelic#license_key>
NEW_RELIC_OTLP_ENDPOINT=otlp.nr-data.net:4317

# Resource Management
GOGC=80
GOMEMLIMIT=1800MiB
MEMORY_LIMIT_PERCENTAGE=80

# Feature Flags
ENABLE_FEATURE_DETECTION=true
ENABLE_ADAPTIVE_SAMPLER=true
ENABLE_CIRCUIT_BREAKER=true
ENABLE_PII_SANITIZATION=true
```

## Feature Detection Setup

### Automatic Feature Detection

The collector automatically detects:

1. **Database Extensions**
   - PostgreSQL: pg_stat_statements, pg_stat_monitor, pg_wait_sampling, auto_explain
   - MySQL: performance_schema tables

2. **Database Capabilities**
   - PostgreSQL: track_io_timing, track_functions, shared_preload_libraries
   - MySQL: performance_schema enabled, slow_query_log

3. **Cloud Providers**
   - AWS RDS/Aurora
   - Google Cloud SQL
   - Azure Database

### Manual Feature Configuration

Override automatic detection when needed:

```yaml
receivers:
  enhancedsql/postgresql:
    feature_detection:
      enabled: true
      override_features:
        extensions:
          pg_stat_statements: 
            available: true
            version: "1.10"
        capabilities:
          track_io_timing: "on"
        cloud_provider: "aws_rds"
```

### Query Library Configuration

```yaml
# config/queries/postgresql_queries.yaml
queries:
  - name: advanced_slow_queries
    category: slow_queries
    priority: 100  # Highest priority
    sql: |
      SELECT queryid, query, mean_exec_time
      FROM pg_stat_monitor
      WHERE mean_exec_time > $1
    requirements:
      required_extensions: ["pg_stat_monitor"]
      required_capabilities: ["track_io_timing"]
    
  - name: basic_slow_queries
    category: slow_queries
    priority: 10  # Fallback
    sql: |
      SELECT pid, query, now() - query_start as duration
      FROM pg_stat_activity
      WHERE state = 'active'
    requirements: []  # Always available
```

## Kubernetes Deployment

### Namespace and RBAC

```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: database-intelligence

---
# k8s/rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: database-intelligence-collector
  namespace: database-intelligence

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: database-intelligence-collector
rules:
  - apiGroups: [""]
    resources: ["nodes", "nodes/metrics", "services", "endpoints", "pods"]
    verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: database-intelligence-collector
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: database-intelligence-collector
subjects:
  - kind: ServiceAccount
    name: database-intelligence-collector
    namespace: database-intelligence
```

### Deployment with Feature Detection

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: database-intelligence-collector
  namespace: database-intelligence
spec:
  replicas: 2
  selector:
    matchLabels:
      app: database-intelligence-collector
  template:
    metadata:
      labels:
        app: database-intelligence-collector
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8888"
    spec:
      serviceAccountName: database-intelligence-collector
      containers:
      - name: collector
        image: database-intelligence:v2.0.0
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2"
        env:
        - name: GOGC
          value: "80"
        - name: GOMEMLIMIT
          value: "1800MiB"
        - name: ENABLE_FEATURE_DETECTION
          value: "true"
        envFrom:
        - secretRef:
            name: database-credentials
        - configMapRef:
            name: collector-config
        ports:
        - containerPort: 8888
          name: metrics
        - containerPort: 13133
          name: health
        livenessProbe:
          httpGet:
            path: /
            port: 13133
          initialDelaySeconds: 30
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /
            port: 13133
          initialDelaySeconds: 5
          periodSeconds: 10
        volumeMounts:
        - name: config
          mountPath: /etc/otelcol
        - name: queries
          mountPath: /etc/queries
      volumes:
      - name: config
        configMap:
          name: collector-config
      - name: queries
        configMap:
          name: query-library
```

### Horizontal Pod Autoscaling

```yaml
# k8s/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: database-intelligence-collector
  namespace: database-intelligence
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: database-intelligence-collector
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 100
        periodSeconds: 60
```

## Docker Deployment

### Production Docker Compose

```yaml
# docker-compose.production.yml
version: '3.8'

services:
  collector:
    image: database-intelligence:v2.0.0
    container_name: db-intelligence-collector
    restart: unless-stopped
    environment:
      - GOGC=80
      - GOMEMLIMIT=1800MiB
    env_file:
      - .env.production
    volumes:
      - ./configs/postgresql-maximum-extraction.yaml:/etc/otelcol/config.yaml:ro
      - ./config/queries:/etc/queries:ro
      - collector-data:/var/lib/otelcol
    ports:
      - "8888:8888"   # Metrics
      - "13133:13133" # Health
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 512M
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:13133/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

volumes:
  collector-data:
    driver: local
```

### Docker Swarm Deployment

```bash
# Create secrets
echo "your-license-key" | docker secret create new_relic_license_key -
echo "db-password" | docker secret create postgres_password -

# Deploy stack
docker stack deploy -c docker-stack.yml db-intelligence
```

## Configuration Management

### Using ConfigMaps (Kubernetes)

```bash
# Create config from file
kubectl create configmap collector-config \
  --from-file=config.yaml=configs/postgresql-maximum-extraction.yaml \
  -n database-intelligence

# Create query library
kubectl create configmap query-library \
  --from-file=postgresql.yaml=config/queries/postgresql_queries.yaml \
  --from-file=mysql.yaml=config/queries/mysql_queries.yaml \
  -n database-intelligence
```

### Using Secrets

```bash
# Kubernetes secrets
kubectl create secret generic database-credentials \
  --from-literal=POSTGRES_PASSWORD='secure-password' \
  --from-literal=NEW_RELIC_LICENSE_KEY='your-key' \
  -n database-intelligence

# Docker secrets
printf "secure-password" | docker secret create postgres_password -
```

### Configuration Validation

```bash
# Validate configuration before deployment
./dist/database-intelligence-collector validate \
  --config=configs/postgresql-maximum-extraction.yaml

# Test configuration with dry-run
./dist/database-intelligence-collector \
  --config=configs/postgresql-maximum-extraction.yaml \
  --dry-run
```

## Monitoring & Troubleshooting

### Health Checks

```bash
# Check collector health
curl http://localhost:13133/health

# Check metrics endpoint
curl http://localhost:8888/metrics | grep otelcol_

# Check feature detection status
curl http://localhost:8888/metrics | grep db_feature
```

### Debugging Feature Detection

```bash
# Enable debug logging for feature detection
export OTEL_LOG_LEVEL=debug

# Check detected features
curl http://localhost:8888/metrics | grep -E 'db_feature_(extension|capability)'

# View circuit breaker status
curl http://localhost:8888/metrics | grep circuitbreaker
```

### Common Issues

#### Missing Extensions

```bash
# Check PostgreSQL extensions
psql -c "SELECT * FROM pg_extension WHERE extname LIKE 'pg_stat%'"

# Install missing extension
psql -c "CREATE EXTENSION pg_stat_statements"
```

#### Permission Errors

```sql
-- PostgreSQL: Grant necessary permissions
GRANT pg_monitor TO otel_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO otel_monitor;

-- MySQL: Grant performance schema access
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
```

#### High Memory Usage

```yaml
# Adjust memory limits
processors:
  memory_limiter:
    limit_percentage: 75  # Lower limit
    spike_limit_percentage: 20  # Lower spike allowance
  
  adaptivesampler:
    default_sample_rate: 0.05  # More aggressive sampling
```

### Performance Tuning

```yaml
# Optimize for high-volume environments
processors:
  batch:
    timeout: 60s  # Larger batches
    send_batch_size: 5000
    send_batch_max_size: 10000
    
  adaptivesampler:
    cache_size: 100000  # Larger dedup cache
    
exporters:
  otlp/newrelic:
    sending_queue:
      num_consumers: 10  # More parallel exports
      queue_size: 5000   # Larger queue
```

## Security Considerations

### TLS Configuration

```yaml
receivers:
  postgresql:
    tls:
      ca_file: /etc/ssl/certs/postgres-ca.crt
      cert_file: /etc/ssl/certs/postgres-client.crt
      key_file: /etc/ssl/private/postgres-client.key
      min_version: "1.2"
      
exporters:
  otlp/newrelic:
    tls:
      insecure: false
      ca_file: /etc/ssl/certs/ca-certificates.crt
```

### PII Protection

```yaml
processors:
  transform:
    error_mode: ignore
    metric_statements:
      - context: datapoint
        statements:
          # Redact sensitive patterns
          - replace_all_patterns(attributes["query"], "'[^']*'", "'[REDACTED]'")
          - replace_pattern(attributes["query"], "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b", "[CARD]")
          - replace_pattern(attributes["query"], "\\b\\d{3}-\\d{2}-\\d{4}\\b", "[SSN]")
```

### Network Policies

```yaml
# k8s/network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: database-intelligence-collector
  namespace: database-intelligence
spec:
  podSelector:
    matchLabels:
      app: database-intelligence-collector
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 8888  # Metrics only from monitoring namespace
  egress:
  - to:
    - namespaceSelector: {}
      podSelector:
        matchLabels:
          app: postgresql
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443  # HTTPS to New Relic
```

## Migration from OHI

### Side-by-Side Deployment

```yaml
# Run both OHI and OTEL collector
services:
  ohi-postgresql:
    image: newrelic/nri-postgresql:latest
    environment:
      - POSTGRES_HOST=localhost
      - POSTGRES_PORT=5432
    
  otel-collector:
    image: database-intelligence:v2.0.0
    environment:
      - ENABLE_OHI_COMPATIBILITY=true
    volumes:
      - ./config/collector-ohi-migration.yaml:/etc/otelcol/config.yaml
```

### Validation

```bash
# Compare metrics between OHI and OTEL
./scripts/validate-migration.sh \
  --ohi-metrics http://localhost:9999/metrics \
  --otel-metrics http://localhost:8888/metrics \
  --output comparison-report.html
```

### Gradual Migration

1. **Phase 1**: Deploy OTEL collector alongside OHI
2. **Phase 2**: Validate metric parity for 1 week
3. **Phase 3**: Switch dashboards to OTEL metrics
4. **Phase 4**: Disable OHI after validation period

## Best Practices

1. **Start Simple**: Begin with basic configuration and add features gradually
2. **Monitor Resource Usage**: Set appropriate limits and monitor collector metrics
3. **Enable Feature Detection**: Let the collector adapt to your database setup
4. **Use Sampling**: Configure adaptive sampling for high-volume environments
5. **Secure Credentials**: Use secrets management for database passwords
6. **Regular Updates**: Keep the collector updated for new features and fixes
7. **Test Thoroughly**: Validate in staging before production deployment

## Support

- Documentation: [/docs](../docs/)
- Issues: [GitHub Issues](https://github.com/database-intelligence-mvp/database-intelligence-mvp/issues)
- Community: [#database-intelligence Slack](https://otel-community.slack.com)