# Deployment Guide

This guide covers deployment strategies, procedures, and best practices for the Database Intelligence Collector in production environments.

## Deployment Overview

### Current Architecture
- **Single Instance**: Production-ready single instance deployment
- **In-Memory State**: No external dependencies (Redis removed)
- **Fast Recovery**: 2-3 second startup time
- **Resource Efficient**: 200-300MB memory typical usage

### Deployment Options
1. **Docker Compose** - Simple, self-contained
2. **Kubernetes** - Cloud-native, scalable
3. **Systemd** - Traditional Linux deployment
4. **Cloud Services** - AWS ECS, GCP Cloud Run, Azure Container Instances

## Pre-Deployment Checklist

### ✅ Infrastructure Requirements
- [ ] Target servers meet minimum requirements (2 CPU, 1GB RAM)
- [ ] Network connectivity to databases verified
- [ ] Firewall rules configured for required ports
- [ ] DNS/hostnames resolved correctly

### ✅ Database Preparation
- [ ] Monitoring users created with appropriate permissions
- [ ] Connection strings tested from deployment environment
- [ ] Query performance impact assessed
- [ ] Backup procedures verified

### ✅ Configuration Review
- [ ] Environment-specific configuration prepared
- [ ] Sensitive values stored in secrets management
- [ ] Resource limits appropriate for workload
- [ ] Sampling rules match business requirements

### ✅ Operational Readiness
- [ ] Monitoring dashboards prepared
- [ ] Alert rules configured
- [ ] Runbook accessible to operators
- [ ] Rollback procedure documented

## Deployment Strategies

### 1. Docker Compose Deployment

**Best for**: Single-server deployments, development environments

#### Directory Structure
```
deployment/
├── docker-compose.yaml
├── .env
├── config/
│   └── collector-production.yaml
└── scripts/
    ├── deploy.sh
    └── health-check.sh
```

#### Deployment Steps
```bash
# 1. Prepare configuration
export DEPLOY_ENV=production
envsubst < config/collector-template.yaml > config/collector-production.yaml

# 2. Validate configuration
docker run --rm -v $(pwd)/config:/config \
  database-intelligence/collector:latest \
  --config=/config/collector-production.yaml \
  --dry-run

# 3. Deploy
docker-compose up -d

# 4. Verify health
./scripts/health-check.sh

# 5. Monitor logs
docker-compose logs -f collector
```

#### docker-compose.yaml
```yaml
version: '3.8'

services:
  collector:
    image: database-intelligence/collector:${VERSION:-latest}
    container_name: db-intel-collector
    restart: always
    
    ports:
      - "13133:13133"  # Health check
      - "8888:8888"    # Metrics (internal only)
    
    volumes:
      - ./config/collector-production.yaml:/etc/otel-collector-config.yaml:ro
      - /var/log/db-intel:/var/log/collector
    
    environment:
      - POSTGRES_HOST=${POSTGRES_HOST}
      - POSTGRES_USER=${POSTGRES_USER}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - MYSQL_HOST=${MYSQL_HOST}
      - MYSQL_USER=${MYSQL_USER}
      - MYSQL_PASSWORD=${MYSQL_PASSWORD}
      - NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY}
      - ENVIRONMENT=${ENVIRONMENT:-production}
    
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 256M
    
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:13133/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "5"
```

### 2. Kubernetes Deployment

**Best for**: Cloud environments, auto-scaling needed

#### Deployment Manifest
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: db-intelligence-collector
  namespace: monitoring
  labels:
    app: db-intelligence
    component: collector
spec:
  replicas: 1  # Single instance
  selector:
    matchLabels:
      app: db-intelligence
      component: collector
  template:
    metadata:
      labels:
        app: db-intelligence
        component: collector
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8888"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: db-intelligence-collector
      
      containers:
      - name: collector
        image: database-intelligence/collector:1.0.0
        imagePullPolicy: Always
        
        ports:
        - name: health
          containerPort: 13133
          protocol: TCP
        - name: metrics
          containerPort: 8888
          protocol: TCP
        
        env:
        - name: ENVIRONMENT
          value: "production"
        - name: POSTGRES_HOST
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: postgres-host
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: postgres-user
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: postgres-password
        
        resources:
          requests:
            memory: "256Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "2000m"
        
        livenessProbe:
          httpGet:
            path: /health/live
            port: health
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3
        
        readinessProbe:
          httpGet:
            path: /health/ready
            port: health
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        
        volumeMounts:
        - name: config
          mountPath: /etc/otel-collector-config.yaml
          subPath: config.yaml
          readOnly: true
        - name: cache
          mountPath: /var/lib/otel
      
      volumes:
      - name: config
        configMap:
          name: collector-config
      - name: cache
        emptyDir:
          sizeLimit: 1Gi
```

#### Service Definition
```yaml
apiVersion: v1
kind: Service
metadata:
  name: db-intelligence-collector
  namespace: monitoring
  labels:
    app: db-intelligence
    component: collector
spec:
  type: ClusterIP
  ports:
  - name: health
    port: 13133
    targetPort: health
  - name: metrics
    port: 8888
    targetPort: metrics
  selector:
    app: db-intelligence
    component: collector
```

#### ConfigMap
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: collector-config
  namespace: monitoring
data:
  config.yaml: |
    receivers:
      postgresql:
        endpoint: ${env:POSTGRES_HOST}:5432
        username: ${env:POSTGRES_USER}
        password: ${env:POSTGRES_PASSWORD}
        databases:
          - postgres
          - ${env:POSTGRES_DB}
        collection_interval: 30s
    
    processors:
      memory_limiter:
        check_interval: 1s
        limit_percentage: 75
        spike_limit_percentage: 20
      
      adaptive_sampler:
        in_memory_only: true
        default_sample_rate: 0.1
        environment_overrides:
          production:
            slow_query_threshold_ms: 2000
            max_records_per_second: 500
    
    exporters:
      otlp/newrelic:
        endpoint: otlp.nr-data.net:4317
        headers:
          api-key: ${env:NEW_RELIC_LICENSE_KEY}
    
    service:
      pipelines:
        metrics:
          receivers: [postgresql]
          processors: [memory_limiter, adaptive_sampler]
          exporters: [otlp/newrelic]
```

### 3. Systemd Deployment

**Best for**: Traditional Linux servers, bare metal

#### Installation Script
```bash
#!/bin/bash
# deploy-systemd.sh

set -e

# Variables
INSTALL_DIR="/opt/db-intelligence"
CONFIG_DIR="/etc/db-intelligence"
LOG_DIR="/var/log/db-intelligence"
USER="dbintel"
GROUP="dbintel"

# Create user
if ! id "$USER" &>/dev/null; then
    useradd -r -s /bin/false -d "$INSTALL_DIR" "$USER"
fi

# Create directories
mkdir -p "$INSTALL_DIR" "$CONFIG_DIR" "$LOG_DIR"

# Download and install binary
ARCH=$(uname -m)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
VERSION="1.0.0"

wget -O "$INSTALL_DIR/collector" \
  "https://github.com/database-intelligence-mvp/releases/download/v${VERSION}/collector-${OS}-${ARCH}"

chmod +x "$INSTALL_DIR/collector"

# Set permissions
chown -R "$USER:$GROUP" "$INSTALL_DIR" "$CONFIG_DIR" "$LOG_DIR"

# Install systemd service
cat > /etc/systemd/system/db-intelligence-collector.service <<EOF
[Unit]
Description=Database Intelligence Collector
After=network.target
Documentation=https://github.com/database-intelligence-mvp

[Service]
Type=simple
User=$USER
Group=$GROUP
WorkingDirectory=$INSTALL_DIR

ExecStartPre=/bin/bash -c 'source $CONFIG_DIR/environment && $INSTALL_DIR/collector --config=$CONFIG_DIR/config.yaml --dry-run'
ExecStart=/bin/bash -c 'source $CONFIG_DIR/environment && exec $INSTALL_DIR/collector --config=$CONFIG_DIR/config.yaml'

Restart=always
RestartSec=10
StandardOutput=append:$LOG_DIR/collector.log
StandardError=append:$LOG_DIR/collector.log

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$LOG_DIR

# Resource Limits
LimitNOFILE=65536
MemoryLimit=1G
CPUQuota=200%
TasksMax=4096

# Environment
EnvironmentFile=$CONFIG_DIR/environment

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable db-intelligence-collector
systemctl start db-intelligence-collector
```

### 4. Cloud-Specific Deployments

#### AWS ECS Task Definition
```json
{
  "family": "db-intelligence-collector",
  "taskRoleArn": "arn:aws:iam::123456789012:role/db-intel-task-role",
  "executionRoleArn": "arn:aws:iam::123456789012:role/db-intel-execution-role",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "1024",
  "memory": "2048",
  "containerDefinitions": [
    {
      "name": "collector",
      "image": "database-intelligence/collector:1.0.0",
      "essential": true,
      "environment": [
        {"name": "ENVIRONMENT", "value": "production"}
      ],
      "secrets": [
        {
          "name": "POSTGRES_PASSWORD",
          "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:db-credentials:postgres_password::"
        },
        {
          "name": "NEW_RELIC_LICENSE_KEY",
          "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789012:secret:newrelic:license_key::"
        }
      ],
      "mountPoints": [
        {
          "sourceVolume": "config",
          "containerPath": "/etc/otel-collector-config.yaml",
          "readOnly": true
        }
      ],
      "healthCheck": {
        "command": ["CMD-SHELL", "curl -f http://localhost:13133/health || exit 1"],
        "interval": 30,
        "timeout": 5,
        "retries": 3,
        "startPeriod": 10
      },
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/db-intelligence-collector",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "collector"
        }
      }
    }
  ],
  "volumes": [
    {
      "name": "config",
      "efsVolumeConfiguration": {
        "fileSystemId": "fs-12345678",
        "rootDirectory": "/config"
      }
    }
  ]
}
```

## Production Configuration

### Resource Allocation

| Environment | CPU | Memory | Sampling Rate | Collection Interval |
|------------|-----|---------|---------------|-------------------|
| Development | 0.5 cores | 256MB | 1.0 | 10s |
| Staging | 1 core | 512MB | 0.5 | 30s |
| Production | 2 cores | 1GB | 0.1 | 60s |
| High-Volume | 4 cores | 2GB | 0.01 | 120s |

### Security Hardening

#### 1. Network Security
```yaml
# Firewall rules (iptables)
-A INPUT -p tcp --dport 13133 -s 10.0.0.0/8 -j ACCEPT  # Health checks from internal
-A INPUT -p tcp --dport 8888 -s 127.0.0.1 -j ACCEPT    # Metrics localhost only
-A INPUT -p tcp --dport 13133 -j DROP                  # Block external health
-A INPUT -p tcp --dport 8888 -j DROP                   # Block external metrics
```

#### 2. Secrets Management
```bash
# AWS Secrets Manager
aws secretsmanager create-secret \
  --name db-intelligence/postgres \
  --secret-string '{"username":"otel_monitor","password":"secure_password"}'

# Kubernetes Secrets
kubectl create secret generic db-credentials \
  --from-literal=postgres-user=otel_monitor \
  --from-literal=postgres-password=secure_password \
  -n monitoring

# HashiCorp Vault
vault kv put secret/db-intelligence/postgres \
  username=otel_monitor \
  password=secure_password
```

#### 3. TLS Configuration
```yaml
# Enable TLS for exports
exporters:
  otlp:
    endpoint: otlp.example.com:4317
    tls:
      ca_file: /etc/ssl/certs/ca-bundle.crt
      cert_file: /etc/ssl/certs/collector.crt
      key_file: /etc/ssl/private/collector.key
      insecure_skip_verify: false
```

## Deployment Validation

### 1. Health Verification Script
```bash
#!/bin/bash
# validate-deployment.sh

HEALTH_ENDPOINT="http://localhost:13133/health"
METRICS_ENDPOINT="http://localhost:8888/metrics"
TIMEOUT=300  # 5 minutes
INTERVAL=10

echo "Waiting for collector to be healthy..."
start_time=$(date +%s)

while true; do
    if curl -sf "$HEALTH_ENDPOINT" > /dev/null; then
        echo "✓ Collector is healthy"
        break
    fi
    
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))
    
    if [ $elapsed -gt $TIMEOUT ]; then
        echo "✗ Timeout waiting for collector to be healthy"
        exit 1
    fi
    
    echo "Waiting... ($elapsed seconds elapsed)"
    sleep $INTERVAL
done

# Verify metrics
echo "Checking metrics..."
metrics=$(curl -sf "$METRICS_ENDPOINT" | grep -E "otelcol_process_uptime|postgresql_up|mysql_up")

if [ -n "$metrics" ]; then
    echo "✓ Metrics are being collected"
    echo "$metrics" | head -5
else
    echo "✗ No metrics found"
    exit 1
fi

# Check component health
echo "Checking components..."
components=$(curl -sf "$HEALTH_ENDPOINT" | jq -r '.components | keys[]')

for component in adaptive_sampler circuit_breaker plan_extractor verification; do
    if echo "$components" | grep -q "$component"; then
        echo "✓ $component is healthy"
    else
        echo "✗ $component is not healthy"
        exit 1
    fi
done

echo "✓ Deployment validation successful"
```

### 2. Smoke Test
```bash
# Generate test load
for i in {1..100}; do
    psql -h $POSTGRES_HOST -U test_user -c "SELECT pg_sleep(0.1);" &
done
wait

# Check if metrics were collected
sleep 60
curl -s http://localhost:8888/metrics | grep "postgresql_queries_total"
```

## Rollback Procedures

### 1. Docker Rollback
```bash
# Tag current version before update
docker tag database-intelligence/collector:latest database-intelligence/collector:rollback

# If rollback needed
docker-compose down
docker tag database-intelligence/collector:rollback database-intelligence/collector:latest
docker-compose up -d
```

### 2. Kubernetes Rollback
```bash
# Check rollout history
kubectl rollout history deployment/db-intelligence-collector -n monitoring

# Rollback to previous version
kubectl rollout undo deployment/db-intelligence-collector -n monitoring

# Or to specific revision
kubectl rollout undo deployment/db-intelligence-collector --to-revision=2 -n monitoring

# Monitor rollback
kubectl rollout status deployment/db-intelligence-collector -n monitoring
```

### 3. Configuration Rollback
```bash
# Keep versioned configs
cp config/collector.yaml config/collector-$(date +%Y%m%d-%H%M%S).yaml

# Rollback
cp config/collector-backup.yaml config/collector.yaml
# Restart collector
```

## Monitoring the Deployment

### Key Metrics to Monitor

| Metric | Alert Threshold | Action |
|--------|----------------|---------|
| CPU Usage | >80% for 5min | Scale up or optimize config |
| Memory Usage | >90% | Increase limits or reduce cache |
| Export Failures | >1% | Check network and credentials |
| Circuit Breaker Opens | >5/hour | Investigate database health |
| Processing Latency | >100ms p99 | Review sampling rules |

### Grafana Dashboard Queries

```promql
# Collector Health Score
up{job="db-intelligence-collector"}

# Processing Rate
rate(otelcol_processor_accepted_metric_points[5m])

# Error Rate
rate(otelcol_exporter_send_failed_metric_points[5m]) / 
rate(otelcol_exporter_sent_metric_points[5m])

# Memory Usage
process_resident_memory_bytes{job="db-intelligence-collector"} / 1024 / 1024

# Circuit Breaker Status
circuit_breaker_state{job="db-intelligence-collector"}
```

## Troubleshooting Deployments

### Common Issues

#### Issue: Collector Won't Start
```bash
# Check logs
docker logs db-intel-collector
kubectl logs deployment/db-intelligence-collector -n monitoring
journalctl -u db-intelligence-collector

# Validate config
collector --config=config.yaml --dry-run
```

#### Issue: No Metrics Collected
```bash
# Check database connectivity
docker exec db-intel-collector nc -zv postgres-host 5432

# Test credentials
docker exec db-intel-collector psql -h postgres-host -U otel_monitor -c "SELECT 1"

# Check permissions
SELECT has_database_privilege('otel_monitor', 'postgres', 'CONNECT');
```

#### Issue: High Memory Usage
```yaml
# Reduce cache sizes
adaptive_sampler:
  deduplication:
    cache_size: 5000  # Reduced from 10000

# Increase memory limit check frequency
memory_limiter:
  check_interval: 500ms  # Reduced from 1s

# Reduce batch size
batch:
  send_batch_size: 500  # Reduced from 1000
```

---

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025