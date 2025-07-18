# Unified Deployment Guide

This comprehensive guide covers all deployment scenarios for Database Intelligence with OpenTelemetry collectors.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Deployment Options](#deployment-options)
- [Production Deployment](#production-deployment)
- [Security Considerations](#security-considerations)
- [Monitoring & Operations](#monitoring--operations)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### System Requirements
- Docker 20.10+ or Kubernetes 1.19+
- 2GB RAM minimum (4GB recommended)
- 10GB disk space
- Network access to database hosts
- New Relic account with license key

### Required Tools
```bash
# Check Docker
docker --version

# Check Docker Compose (optional)
docker-compose --version

# Check Kubernetes (optional)
kubectl version --client
```

## Quick Start

### 1. Clone Repository
```bash
git clone <repository-url>
cd database-intelligence-restructured
```

### 2. Set Environment Variables
```bash
# Copy template for your database
cp configs/env-templates/postgresql.env .env

# Edit with your values
vi .env
```

### 3. Start Collector
```bash
# Single database
./scripts/test-database-config.sh postgresql

# All databases
./scripts/start-all-databases.sh
```

## Deployment Options

### Option 1: Docker (Recommended for Testing)

#### Single Database Collector
```bash
docker run -d \
  --name otel-postgresql \
  -v $(pwd)/configs/postgresql-maximum-extraction.yaml:/etc/otelcol/config.yaml \
  --env-file .env \
  -p 8888:8888 \
  otel/opentelemetry-collector-contrib:latest
```

#### Multiple Databases with Docker Compose
```bash
# Start all services
docker-compose -f docker-compose.databases.yml up -d

# View logs
docker-compose -f docker-compose.databases.yml logs -f

# Stop all services
docker-compose -f docker-compose.databases.yml down
```

### Option 2: Kubernetes

#### ConfigMap Creation
```bash
# Create namespace
kubectl create namespace database-intelligence

# Create config from file
kubectl create configmap otel-config \
  --from-file=configs/postgresql-maximum-extraction.yaml \
  -n database-intelligence

# Create secrets
kubectl create secret generic database-credentials \
  --from-env-file=.env \
  -n database-intelligence
```

#### Deployment Manifest
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-postgresql
  namespace: database-intelligence
spec:
  replicas: 1
  selector:
    matchLabels:
      app: otel-postgresql
  template:
    metadata:
      labels:
        app: otel-postgresql
    spec:
      containers:
      - name: otel-collector
        image: otel/opentelemetry-collector-contrib:latest
        volumeMounts:
        - name: config
          mountPath: /etc/otelcol
        envFrom:
        - secretRef:
            name: database-credentials
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
      volumes:
      - name: config
        configMap:
          name: otel-config
---
apiVersion: v1
kind: Service
metadata:
  name: otel-postgresql
  namespace: database-intelligence
spec:
  selector:
    app: otel-postgresql
  ports:
  - name: metrics
    port: 8888
  - name: health
    port: 13133
```

#### Deploy to Kubernetes
```bash
kubectl apply -f k8s/otel-postgresql-deployment.yaml
```

### Option 3: Standalone Binary

#### Download Collector
```bash
# Linux
wget https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.88.0/otelcol-contrib_0.88.0_linux_amd64.tar.gz
tar -xzf otelcol-contrib_0.88.0_linux_amd64.tar.gz

# macOS
wget https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.88.0/otelcol-contrib_0.88.0_darwin_amd64.tar.gz
tar -xzf otelcol-contrib_0.88.0_darwin_amd64.tar.gz
```

#### Run Collector
```bash
# Set environment variables
export NEW_RELIC_LICENSE_KEY=your_key
export POSTGRESQL_HOST=localhost
export POSTGRESQL_PORT=5432
export POSTGRESQL_USER=postgres
export POSTGRESQL_PASSWORD=password

# Run collector
./otelcol-contrib --config=configs/postgresql-maximum-extraction.yaml
```

## Production Deployment

### High Availability Setup

#### Multiple Collectors
```yaml
# docker-compose-ha.yml
version: '3.8'

services:
  otel-postgresql-1:
    image: otel/opentelemetry-collector-contrib:latest
    volumes:
      - ./configs/postgresql-maximum-extraction.yaml:/etc/otelcol/config.yaml
    env_file: .env
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 512M

  nginx:
    image: nginx:alpine
    ports:
      - "8888:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
    depends_on:
      - otel-postgresql-1
```

### Resource Optimization

#### Memory Limits
```yaml
# In your config file
processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 25
```

#### Batch Processing
```yaml
processors:
  batch:
    timeout: 10s
    send_batch_size: 8192
    send_batch_max_size: 16384
```

### Scaling Strategies

#### Horizontal Scaling
```bash
# Kubernetes
kubectl scale deployment otel-postgresql --replicas=3 -n database-intelligence

# Docker Swarm
docker service scale otel_postgresql=3
```

#### Vertical Scaling
```yaml
# Update resource limits
resources:
  requests:
    memory: "2Gi"
    cpu: "1000m"
  limits:
    memory: "4Gi"
    cpu: "2000m"
```

## Security Considerations

### 1. Secrets Management

#### Using Docker Secrets
```bash
# Create secrets
echo "your_password" | docker secret create postgres_password -
echo "your_license_key" | docker secret create newrelic_key -

# Use in compose
services:
  collector:
    secrets:
      - postgres_password
      - newrelic_key
    environment:
      POSTGRESQL_PASSWORD_FILE: /run/secrets/postgres_password
      NEW_RELIC_LICENSE_KEY_FILE: /run/secrets/newrelic_key
```

#### Using Kubernetes Secrets
```bash
# Create secret
kubectl create secret generic db-credentials \
  --from-literal=password=your_password \
  --from-literal=license_key=your_key \
  -n database-intelligence
```

### 2. Network Security

#### Restrict Collector Access
```yaml
# iptables rules
iptables -A INPUT -p tcp --dport 8888 -s 10.0.0.0/8 -j ACCEPT
iptables -A INPUT -p tcp --dport 8888 -j DROP
```

#### TLS Configuration
```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
        tls:
          cert_file: /certs/server.crt
          key_file: /certs/server.key
```

### 3. RBAC for Databases

```sql
-- PostgreSQL
CREATE USER otel_monitor WITH PASSWORD 'secure_password';
GRANT pg_monitor TO otel_monitor;
GRANT CONNECT ON DATABASE yourdb TO otel_monitor;

-- MySQL
CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'secure_password';
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
```

## Monitoring & Operations

### Health Checks

#### Docker Health Check
```yaml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:13133/health"]
  interval: 30s
  timeout: 10s
  retries: 3
```

#### Kubernetes Probes
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 13133
  initialDelaySeconds: 30
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /health
    port: 13133
  initialDelaySeconds: 10
  periodSeconds: 10
```

### Monitoring the Collectors

#### Prometheus Metrics
```yaml
# Access collector metrics
curl http://localhost:8888/metrics

# Key metrics to monitor
- otelcol_receiver_accepted_metric_points
- otelcol_receiver_refused_metric_points
- otelcol_exporter_sent_metric_points
- otelcol_processor_batch_batch_size_trigger_send
- process_runtime_memstats_sys_bytes
```

#### Create Alerts
```yaml
# Prometheus alert rules
groups:
  - name: otel_collector
    rules:
    - alert: CollectorHighMemory
      expr: process_runtime_memstats_sys_bytes > 2147483648
      for: 5m
      annotations:
        summary: "Collector memory usage is high"
        
    - alert: CollectorExportFailures
      expr: rate(otelcol_exporter_send_failed_metric_points[5m]) > 0
      for: 5m
      annotations:
        summary: "Collector failing to export metrics"
```

### Log Management

#### Configure Logging
```yaml
service:
  telemetry:
    logs:
      level: info
      output_paths: ["stdout", "/var/log/otel/collector.log"]
      error_output_paths: ["stderr", "/var/log/otel/error.log"]
```

#### Log Rotation
```bash
# /etc/logrotate.d/otel-collector
/var/log/otel/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0644 otel otel
}
```

## Troubleshooting

### Common Issues

#### 1. Collector Won't Start
```bash
# Check logs
docker logs otel-postgresql

# Validate configuration
./scripts/validate-config.sh postgresql

# Test with minimal config
docker run --rm \
  -v $(pwd)/configs/minimal-test.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otelcol/config.yaml --dry-run
```

#### 2. No Metrics Appearing
```bash
# Check connectivity
curl http://localhost:8888/metrics

# Verify database connection
docker exec otel-postgresql nc -zv ${POSTGRESQL_HOST} ${POSTGRESQL_PORT}

# Check permissions
docker exec -it otel-postgresql /bin/sh
# Then test database connection manually
```

#### 3. High Memory Usage
```bash
# Check cardinality
./scripts/check-metric-cardinality.sh postgresql

# Reduce collection frequency
# Edit config to increase intervals
```

### Performance Tuning

#### Run Performance Benchmark
```bash
./scripts/benchmark-performance.sh postgresql 300
```

#### Optimize Based on Results
1. **High CPU**: Increase batch timeout, reduce collection frequency
2. **High Memory**: Add filters, reduce cardinality
3. **Export Failures**: Check network, increase timeout

### Debug Mode

#### Enable Debug Logging
```yaml
service:
  telemetry:
    logs:
      level: debug
```

#### Trace Individual Pipelines
```yaml
service:
  telemetry:
    metrics:
      level: detailed
      address: localhost:8889
```

## Best Practices

### 1. Start Small
- Begin with one database
- Use minimal configuration
- Gradually add metrics

### 2. Monitor Resource Usage
- Set memory limits
- Track CPU usage
- Watch cardinality

### 3. Use Environment Variables
- Never hardcode credentials
- Use .env files
- Rotate passwords regularly

### 4. Regular Updates
- Keep collectors updated
- Review configurations quarterly
- Update documentation

### 5. Backup Configurations
```bash
# Version control configs
git add configs/
git commit -m "Production configuration backup"

# Regular backups
tar -czf configs-backup-$(date +%Y%m%d).tar.gz configs/
```

## Support

For additional help:
1. Check [Troubleshooting Guide](./TROUBLESHOOTING.md)
2. Review [Architecture Documentation](../reference/ARCHITECTURE.md)
3. Consult OpenTelemetry documentation
4. Contact New Relic support

## Next Steps

1. **Development**: Start with Docker deployment
2. **Staging**: Use Kubernetes with limited resources
3. **Production**: Implement HA setup with monitoring
4. **Optimization**: Run benchmarks and tune performance