# Database Intelligence OTel Deployment Guide

## Overview

This guide provides step-by-step instructions for deploying the Database Intelligence solution using OpenTelemetry Collector in both config-only and enhanced modes.

## Prerequisites

### System Requirements

- **Operating System**: Linux (Ubuntu 20.04+, RHEL 8+, Amazon Linux 2)
- **Memory**: Minimum 2GB (config-only), 4GB (enhanced mode)
- **CPU**: 2 cores minimum, 4 cores recommended
- **Disk**: 10GB for logs and temporary storage
- **Network**: Outbound HTTPS access to New Relic endpoints

### Software Requirements

- Docker 20.10+ (for containerized deployment)
- systemd (for service deployment)
- PostgreSQL 11+ or MySQL 5.7+ with appropriate extensions
- Valid New Relic license key

### Database Prerequisites

#### PostgreSQL
```sql
-- Required for basic metrics
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Grant read permissions
CREATE ROLE otel_monitor WITH LOGIN PASSWORD 'secure_password';
GRANT pg_read_all_stats TO otel_monitor;
GRANT CONNECT ON DATABASE your_database TO otel_monitor;

-- For enhanced mode (optional)
CREATE EXTENSION IF NOT EXISTS pg_querylens;  -- Custom extension
```

#### MySQL
```sql
-- Create monitoring user
CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'secure_password';

-- Grant necessary permissions
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
GRANT SELECT ON mysql.* TO 'otel_monitor'@'%';
GRANT SELECT ON sys.* TO 'otel_monitor'@'%';
```

## Deployment Options

### Option 1: Docker Deployment (Recommended)

#### Config-Only Mode

1. **Create configuration directory**
```bash
mkdir -p /opt/otelcol/config
mkdir -p /opt/otelcol/data
```

2. **Copy configuration file**
```bash
# For PostgreSQL
curl -o /opt/otelcol/configs/postgresql-maximum-extraction.yaml \
  https://raw.githubusercontent.com/db-otel/database-intelligence/main/configs/examples/config-only-base.yaml

# For MySQL
curl -o /opt/otelcol/configs/postgresql-maximum-extraction.yaml \
  https://raw.githubusercontent.com/db-otel/database-intelligence/main/configs/examples/config-only-mysql.yaml
```

3. **Create environment file**
```bash
cat > /opt/otelcol/.env << EOF
# Database Configuration
DB_ENDPOINT=postgresql://db-host:5432/mydb
DB_USERNAME=otel_monitor
DB_PASSWORD=secure_password
DB_HOST=db-host
DB_PORT=5432
DATABASE_NAME=mydb

# New Relic Configuration
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
NEW_RELIC_LICENSE_KEY=your_license_key_here

# Service Configuration
SERVICE_NAME=postgresql-prod-01
ENVIRONMENT=production
CLUSTER_NAME=main-cluster
EOF
```

4. **Create Docker Compose file**
```yaml
# docker-compose.yaml
version: '3.8'

services:
  otelcol:
    image: otel/opentelemetry-collector-contrib:latest
    container_name: otelcol-db-intelligence
    restart: unless-stopped
    command: ["--config=/etc/otelcol/config.yaml"]
    volumes:
      - /opt/otelcol/configs/postgresql-maximum-extraction.yaml:/etc/otelcol/config.yaml:ro
      - /opt/otelcol/data:/var/lib/otelcol
    env_file:
      - /opt/otelcol/.env
    ports:
      - "13133:13133"  # Health check
      - "8888:8888"    # Metrics
    networks:
      - monitoring
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "5"
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '2'
        reservations:
          memory: 512M
          cpus: '0.5'

networks:
  monitoring:
    driver: bridge
```

5. **Deploy the collector**
```bash
cd /opt/otelcol
docker-compose up -d

# Check status
docker-compose ps
docker-compose logs -f
```

#### Enhanced Mode

1. **Build custom collector image**
```dockerfile
# Dockerfile.enhanced
FROM golang:1.21 as builder

WORKDIR /build

# Copy custom components
COPY receivers/ ./receivers/
COPY processors/ ./processors/

# Build collector with custom components
RUN go install go.opentelemetry.io/collector/cmd/builder@latest
COPY otelcol-builder.yaml .
RUN builder --config=otelcol-builder.yaml

FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=builder /build/dist/otelcol-custom /otelcol

EXPOSE 13133 4317 4318 8888 9090

ENTRYPOINT ["/otelcol"]
```

2. **Build and deploy**
```bash
# Build custom image
docker build -f Dockerfile.enhanced -t otelcol-enhanced:latest .

# Update docker-compose.yaml to use custom image
sed -i 's|otel/opentelemetry-collector-contrib:latest|otelcol-enhanced:latest|' docker-compose.yaml

# Deploy
docker-compose up -d
```

### Option 2: Systemd Service Deployment

#### Install Collector Binary

1. **Download and install**
```bash
# Create directories
sudo mkdir -p /opt/otelcol/bin
sudo mkdir -p /etc/otelcol
sudo mkdir -p /var/lib/otelcol
sudo mkdir -p /var/log/otelcol

# Download collector
OTEL_VERSION="0.88.0"
wget https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${OTEL_VERSION}/otelcol-contrib_${OTEL_VERSION}_linux_amd64.tar.gz
sudo tar -xzf otelcol-contrib_${OTEL_VERSION}_linux_amd64.tar.gz -C /opt/otelcol/bin/

# Set permissions
sudo chmod +x /opt/otelcol/bin/otelcol-contrib
```

2. **Create configuration**
```bash
sudo cp /path/to/config-only-base.yaml /etc/otelcol/config.yaml
```

3. **Create systemd service**
```ini
# /etc/systemd/system/otelcol.service
[Unit]
Description=OpenTelemetry Collector - Database Intelligence
After=network.target

[Service]
Type=simple
User=otelcol
Group=otelcol
ExecStart=/opt/otelcol/bin/otelcol-contrib --config=/etc/otelcol/config.yaml
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=otelcol
EnvironmentFile=/etc/otelcol/otelcol.env

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/otelcol /var/log/otelcol

# Resource limits
LimitNOFILE=65536
MemoryLimit=2G
CPUQuota=200%

[Install]
WantedBy=multi-user.target
```

4. **Create user and set permissions**
```bash
# Create user
sudo useradd -r -s /bin/false otelcol

# Set ownership
sudo chown -R otelcol:otelcol /opt/otelcol
sudo chown -R otelcol:otelcol /etc/otelcol
sudo chown -R otelcol:otelcol /var/lib/otelcol
sudo chown -R otelcol:otelcol /var/log/otelcol
```

5. **Start the service**
```bash
sudo systemctl daemon-reload
sudo systemctl enable otelcol
sudo systemctl start otelcol

# Check status
sudo systemctl status otelcol
sudo journalctl -u otelcol -f
```

### Option 3: Kubernetes Deployment

#### Using Helm Chart

1. **Create values file**
```yaml
# values.yaml
mode: deployment
replicaCount: 2

image:
  repository: otel/opentelemetry-collector-contrib
  tag: latest
  pullPolicy: IfNotPresent

config:
  receivers:
    postgresql:
      endpoint: "postgresql://postgres-service:5432/mydb"
      username: "${DB_USERNAME}"
      password: "${DB_PASSWORD}"
      collection_interval: 30s
    
  processors:
    memory_limiter:
      limit_mib: 1024
    batch:
      timeout: 10s
    resource:
      attributes:
        - key: service.name
          value: "postgresql-k8s"
          action: insert
  
  exporters:
    otlp:
      endpoint: "${NEW_RELIC_OTLP_ENDPOINT}"
      headers:
        api-key: "${NEW_RELIC_LICENSE_KEY}"
  
  service:
    pipelines:
      metrics:
        receivers: [postgresql]
        processors: [memory_limiter, resource, batch]
        exporters: [otlp]

resources:
  limits:
    memory: 2Gi
    cpu: 2
  requests:
    memory: 512Mi
    cpu: 500m

env:
  - name: DB_USERNAME
    valueFrom:
      secretKeyRef:
        name: db-credentials
        key: username
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: db-credentials
        key: password
  - name: NEW_RELIC_LICENSE_KEY
    valueFrom:
      secretKeyRef:
        name: newrelic-license
        key: license_key
  - name: NEW_RELIC_OTLP_ENDPOINT
    value: "https://otlp.nr-data.net:4318"

serviceMonitor:
  enabled: true

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
  targetMemoryUtilizationPercentage: 80
```

2. **Deploy using Helm**
```bash
# Add OpenTelemetry Helm repository
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

# Create namespace
kubectl create namespace otel-system

# Create secrets
kubectl create secret generic db-credentials \
  --from-literal=username=otel_monitor \
  --from-literal=password=secure_password \
  -n otel-system

kubectl create secret generic newrelic-license \
  --from-literal=license_key=your_license_key \
  -n otel-system

# Deploy collector
helm install otelcol-db-intelligence \
  open-telemetry/opentelemetry-collector \
  --namespace otel-system \
  --values values.yaml

# Check deployment
kubectl get pods -n otel-system
kubectl logs -n otel-system -l app.kubernetes.io/name=opentelemetry-collector -f
```

#### Using Raw Kubernetes Manifests

1. **Create ConfigMap**
```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otelcol-config
  namespace: otel-system
data:
  config.yaml: |
    receivers:
      postgresql:
        endpoint: "postgresql://postgres-service:5432/mydb"
        username: "${DB_USERNAME}"
        password: "${DB_PASSWORD}"
        collection_interval: 30s
        
    processors:
      memory_limiter:
        limit_mib: 1024
      batch:
        timeout: 10s
      resource:
        attributes:
          - key: service.name
            value: "postgresql-k8s"
            action: insert
            
    exporters:
      otlp:
        endpoint: "${NEW_RELIC_OTLP_ENDPOINT}"
        headers:
          api-key: "${NEW_RELIC_LICENSE_KEY}"
          
    service:
      pipelines:
        metrics:
          receivers: [postgresql]
          processors: [memory_limiter, resource, batch]
          exporters: [otlp]
```

2. **Create Deployment**
```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otelcol-db-intelligence
  namespace: otel-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: otelcol-db-intelligence
  template:
    metadata:
      labels:
        app: otelcol-db-intelligence
    spec:
      serviceAccountName: otelcol
      containers:
      - name: otelcol
        image: otel/opentelemetry-collector-contrib:latest
        args:
          - --config=/conf/config.yaml
        resources:
          limits:
            memory: 2Gi
            cpu: 2
          requests:
            memory: 512Mi
            cpu: 500m
        env:
        - name: DB_USERNAME
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: username
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: password
        - name: NEW_RELIC_LICENSE_KEY
          valueFrom:
            secretKeyRef:
              name: newrelic-license
              key: license_key
        - name: NEW_RELIC_OTLP_ENDPOINT
          value: "https://otlp.nr-data.net:4318"
        volumeMounts:
        - name: config
          mountPath: /conf
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
      volumes:
      - name: config
        configMap:
          name: otelcol-config
```

## Post-Deployment Validation

### 1. Check Collector Health

```bash
# Docker
curl http://localhost:13133/health

# Kubernetes
kubectl port-forward -n otel-system svc/otelcol-db-intelligence 13133:13133
curl http://localhost:13133/health
```

### 2. Verify Metrics Collection

```bash
# Check collector metrics
curl http://localhost:8888/metrics | grep otelcol_

# Look for:
# - otelcol_receiver_accepted_metric_points
# - otelcol_exporter_sent_metric_points
# - otelcol_processor_batch_batch_send_size
```

### 3. Validate in New Relic

```sql
-- Check metric ingestion
FROM Metric 
SELECT count(*) 
WHERE metric.name LIKE 'postgresql.%' OR metric.name LIKE 'mysql.%'
SINCE 5 minutes ago

-- Verify specific metrics
FROM Metric 
SELECT latest(postgresql.connections.active) 
WHERE service.name = 'postgresql-prod-01'
```

## High Availability Deployment

### Multi-Collector Setup

1. **Deploy multiple collectors**
```yaml
# For Kubernetes, set replicas
spec:
  replicas: 3
  
# For Docker, use docker-compose scale
docker-compose up -d --scale otelcol=3
```

2. **Use load balancer for database connections**
```yaml
receivers:
  postgresql:
    endpoint: "postgresql://pgbouncer:6432/mydb"
```

3. **Configure anti-affinity (Kubernetes)**
```yaml
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchExpressions:
        - key: app
          operator: In
          values:
          - otelcol-db-intelligence
      topologyKey: kubernetes.io/hostname
```

## Security Considerations

### 1. Use Secrets Management

```bash
# Kubernetes - External Secrets Operator
kubectl apply -f https://raw.githubusercontent.com/external-secrets/external-secrets/main/deploy/crds/bundle.yaml

# Docker - Use Docker Secrets
docker secret create db_password ./password.txt
```

### 2. Enable TLS

```yaml
receivers:
  postgresql:
    endpoint: "postgresql://host:5432/mydb?sslmode=require"
    tls:
      ca_file: /certs/ca.crt
      cert_file: /certs/client.crt
      key_file: /certs/client.key
```

### 3. Network Policies (Kubernetes)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: otelcol-network-policy
spec:
  podSelector:
    matchLabels:
      app: otelcol-db-intelligence
  policyTypes:
  - Ingress
  - Egress
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: database
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443  # HTTPS to New Relic
```

## Maintenance

### Log Rotation

```bash
# For systemd deployment
sudo cat > /etc/logrotate.d/otelcol << EOF
/var/log/otelcol/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0640 otelcol otelcol
    postrotate
        systemctl reload otelcol
    endscript
}
EOF
```

### Backup Configuration

```bash
# Backup script
#!/bin/bash
BACKUP_DIR="/backup/otelcol"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR
cp /etc/otelcol/config.yaml $BACKUP_DIR/config_$DATE.yaml
cp /etc/otelcol/otelcol.env $BACKUP_DIR/env_$DATE

# Keep only last 30 days
find $BACKUP_DIR -name "config_*.yaml" -mtime +30 -delete
find $BACKUP_DIR -name "env_*" -mtime +30 -delete
```

## Troubleshooting

### Common Issues

1. **Connection refused to database**
   - Check firewall rules
   - Verify database user permissions
   - Test connection manually

2. **High memory usage**
   - Reduce batch size
   - Increase memory limits
   - Enable sampling

3. **Metrics not appearing in New Relic**
   - Verify API key
   - Check for NrIntegrationError events
   - Review collector logs

### Debug Mode

```yaml
service:
  telemetry:
    logs:
      level: debug
      
exporters:
  debug:
    verbosity: detailed
```

## Summary

This deployment guide covers:
- Multiple deployment options (Docker, systemd, Kubernetes)
- Security best practices
- High availability configurations
- Post-deployment validation
- Maintenance procedures

Choose the deployment method that best fits your infrastructure and follow the security recommendations for production deployments.