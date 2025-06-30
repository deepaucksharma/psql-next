# Production Deployment Guide

## Overview

This guide provides step-by-step instructions for deploying the Database Intelligence Collector in production environments.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Deployment Options](#deployment-options)
3. [Docker Deployment](#docker-deployment)
4. [Kubernetes Deployment](#kubernetes-deployment)
5. [Configuration Management](#configuration-management)
6. [Security Hardening](#security-hardening)
7. [Monitoring Setup](#monitoring-setup)
8. [Troubleshooting](#troubleshooting)
9. [Maintenance](#maintenance)

## Prerequisites

### System Requirements

- **CPU**: 2+ cores recommended
- **Memory**: 1GB minimum, 2GB recommended
- **Disk**: 10GB for logs and metrics storage
- **Network**: Outbound HTTPS (443) for New Relic

### Database Access

1. **PostgreSQL**:
   - Version 12+ with pg_stat_statements
   - Monitoring user with SELECT permissions
   - Network connectivity from collector

2. **MySQL**:
   - Version 5.7+ or 8.0+
   - Performance schema enabled
   - Monitoring user with appropriate grants

### New Relic Account

- Active New Relic account
- License key for OTLP ingestion
- OTLP endpoint URL (US/EU region)

## Deployment Options

### Option 1: Docker (Recommended for Single Instance)

Best for:
- Simple deployments
- Single database instances
- Quick setup

### Option 2: Kubernetes (Recommended for Scale)

Best for:
- Multi-cluster environments
- High availability requirements
- Auto-scaling needs

### Option 3: Binary Installation

Best for:
- Bare metal servers
- Systemd environments
- Custom configurations

## Docker Deployment

### 1. Build the Image

```bash
# Clone repository
git clone https://github.com/database-intelligence-mvp/database-intelligence-mvp.git
cd database-intelligence-mvp

# Build Docker image
docker build -t database-intelligence-collector:latest .
```

### 2. Create Environment File

```bash
cat > .env.production << EOF
# PostgreSQL Configuration
POSTGRES_HOST=your-postgres-host
POSTGRES_PORT=5432
POSTGRES_USER=monitor
POSTGRES_PASSWORD=secure-password
POSTGRES_DATABASE=production
POSTGRES_TLS_INSECURE=false

# MySQL Configuration
MYSQL_HOST=your-mysql-host
MYSQL_PORT=3306
MYSQL_USER=monitor
MYSQL_PASSWORD=secure-password
MYSQL_DATABASE=production
MYSQL_TLS_INSECURE=false

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your-license-key
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318

# Collector Settings
ENVIRONMENT=production
AWS_REGION=us-east-1
DEPLOYMENT_TYPE=docker
SERVICE_VERSION=1.0.0
COLLECTION_INTERVAL=30s
MEMORY_LIMIT_PERCENT=75
BATCH_SIZE=1000
BATCH_TIMEOUT=10s
LOG_LEVEL=info
DEBUG_VERBOSITY=normal
EOF
```

### 3. Run with Docker Compose

```bash
# Start the full stack
docker-compose -f docker-compose.production.yml up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f otel-collector
```

### 4. Verify Deployment

```bash
# Check health endpoint
curl http://localhost:13133/health

# Check metrics endpoint
curl http://localhost:8888/metrics

# Check zpages
open http://localhost:55679/debug/tracez
```

## Kubernetes Deployment

### 1. Prepare Cluster

```bash
# Create namespace
kubectl apply -f k8s/namespace.yaml

# Create secrets
kubectl create secret generic database-credentials \
  --from-literal=postgres-host=postgres.database.svc.cluster.local \
  --from-literal=postgres-password=your-password \
  --from-literal=mysql-host=mysql.database.svc.cluster.local \
  --from-literal=mysql-password=your-password \
  --from-literal=new-relic-license-key=your-key \
  -n database-intelligence
```

### 2. Deploy Collector

```bash
# Apply all manifests
kubectl apply -f k8s/

# Wait for rollout
kubectl rollout status deployment/database-intelligence-collector -n database-intelligence

# Check pods
kubectl get pods -n database-intelligence
```

### 3. Configure Ingress (Optional)

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: collector-monitoring
  namespace: database-intelligence
spec:
  rules:
  - host: collector-metrics.your-domain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: database-intelligence-collector
            port:
              number: 8888
```

### 4. Enable Auto-scaling

```bash
# Apply HPA
kubectl apply -f k8s/hpa.yaml

# Monitor scaling
kubectl get hpa -n database-intelligence -w
```

## Configuration Management

### 1. Environment-Specific Configs

```bash
# Production
config/
├── production/
│   ├── collector.yaml
│   ├── processors.yaml
│   └── exporters.yaml
├── staging/
│   └── collector.yaml
└── development/
    └── collector.yaml
```

### 2. Secret Management

**Using Kubernetes Secrets:**
```bash
# Create from files
kubectl create secret generic collector-config \
  --from-file=collector.yaml=config/production/collector.yaml \
  -n database-intelligence
```

**Using HashiCorp Vault:**
```bash
# Store secrets
vault kv put secret/database-intelligence \
  postgres_password=xxx \
  mysql_password=xxx \
  new_relic_key=xxx
```

### 3. Configuration Validation

```bash
# Validate configuration
./dist/database-intelligence-collector validate \
  --config=config/production/collector.yaml

# Dry run
./dist/database-intelligence-collector --config=config/production/collector.yaml --dry-run
```

## Security Hardening

### 1. Network Security

```yaml
# NetworkPolicy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: collector-network-policy
spec:
  podSelector:
    matchLabels:
      app: database-intelligence-collector
  policyTypes:
  - Ingress
  - Egress
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  # Allow database connections
  - to:
    - namespaceSelector:
        matchLabels:
          name: database
    ports:
    - protocol: TCP
      port: 5432
    - protocol: TCP
      port: 3306
  # Allow New Relic
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
    ports:
    - protocol: TCP
      port: 443
```

### 2. RBAC Configuration

```yaml
# Minimal permissions
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: collector-minimal
rules:
- apiGroups: [""]
  resources: ["pods", "services"]
  verbs: ["get", "list"]
```

### 3. Security Scanning

```bash
# Scan Docker image
trivy image database-intelligence-collector:latest

# Scan Kubernetes manifests
kubesec scan k8s/*.yaml

# Check CIS benchmarks
kube-bench run --targets node
```

## Monitoring Setup

### 1. Prometheus Integration

```yaml
# ServiceMonitor for Prometheus Operator
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: database-intelligence-collector
spec:
  selector:
    matchLabels:
      app: database-intelligence-collector
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

### 2. Grafana Dashboards

```bash
# Import dashboard
curl -X POST http://grafana:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $GRAFANA_API_KEY" \
  -d @monitoring/grafana-dashboard.json
```

### 3. Alerting Rules

```bash
# Apply Prometheus rules
kubectl apply -f monitoring/alerts.yaml

# Configure alert routing
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmanager-config
data:
  alertmanager.yml: |
    route:
      group_by: ['alertname', 'cluster', 'service']
      group_wait: 10s
      group_interval: 10m
      repeat_interval: 12h
      receiver: 'database-team'
    receivers:
    - name: 'database-team'
      email_configs:
      - to: 'database-team@company.com'
EOF
```

## Troubleshooting

### 1. Common Issues

**Collector not starting:**
```bash
# Check logs
kubectl logs -n database-intelligence deployment/database-intelligence-collector

# Check events
kubectl describe pod -n database-intelligence

# Validate config
kubectl exec -n database-intelligence deployment/database-intelligence-collector -- \
  /usr/local/bin/otelcol validate --config=/etc/otel/config.yaml
```

**Connection failures:**
```bash
# Test database connectivity
kubectl run -it --rm debug --image=postgres:15 --restart=Never -- \
  psql -h postgres.database.svc.cluster.local -U monitor -c "SELECT 1"

# Check DNS resolution
kubectl run -it --rm debug --image=busybox --restart=Never -- \
  nslookup postgres.database.svc.cluster.local
```

**High memory usage:**
```bash
# Get memory stats
kubectl top pod -n database-intelligence

# Adjust limits
kubectl set resources deployment/database-intelligence-collector \
  -n database-intelligence \
  --limits=memory=2Gi
```

### 2. Debug Mode

```yaml
# Enable debug logging
service:
  telemetry:
    logs:
      level: debug
      
# Enable component debug
exporters:
  debug:
    verbosity: detailed
```

### 3. Performance Tuning

```bash
# Run performance benchmark
./benchmarks/performance-test.sh

# Analyze results
cat benchmarks/results/benchmark_results.csv

# Adjust based on findings
# - Increase batch size for high volume
# - Reduce collection interval for lower load
# - Tune memory limits based on usage
```

## Maintenance

### 1. Upgrades

```bash
# Rolling update in Kubernetes
kubectl set image deployment/database-intelligence-collector \
  collector=database-intelligence-collector:v1.1.0 \
  -n database-intelligence

# Monitor rollout
kubectl rollout status deployment/database-intelligence-collector \
  -n database-intelligence
```

### 2. Backup Configuration

```bash
# Backup current config
kubectl get configmap otel-collector-config \
  -n database-intelligence \
  -o yaml > backup/config-$(date +%Y%m%d).yaml

# Backup secrets (encrypted)
kubectl get secret database-credentials \
  -n database-intelligence \
  -o yaml | kubeseal > backup/secrets-$(date +%Y%m%d).yaml
```

### 3. Log Rotation

```bash
# Configure log rotation
cat > /etc/logrotate.d/otel-collector << EOF
/var/log/otel/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0644 otel otel
}
EOF
```

## Best Practices

1. **Start Small**: Deploy to staging first
2. **Monitor Impact**: Watch database load during rollout
3. **Use Sampling**: Start with conservative sampling rates
4. **Regular Reviews**: Check metrics volume and costs
5. **Automate**: Use CI/CD for deployments
6. **Document**: Keep runbooks updated

## Support

- GitHub Issues: [Report bugs](https://github.com/database-intelligence-mvp/issues)
- Documentation: [Full docs](https://docs.database-intelligence.io)
- Community: [Slack channel](https://otel-community.slack.com)