# Unified Deployment Guide - Database Intelligence MVP

## Prerequisites

### System Requirements
- **Go**: 1.21+ (Note: Avoid 1.24.3 - use 1.21 or 1.22)
- **Docker**: 20.10+ with Docker Compose
- **Kubernetes**: 1.24+ (for K8s deployments)
- **Databases**: PostgreSQL 12+ | MySQL 8.0+

### Database Permissions
```sql
-- PostgreSQL
CREATE USER otel_user WITH PASSWORD 'secure_password';
GRANT CONNECT ON DATABASE your_db TO otel_user;
GRANT USAGE ON SCHEMA public TO otel_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO otel_user;

-- MySQL  
CREATE USER 'otel_user'@'%' IDENTIFIED BY 'secure_password';
GRANT SELECT ON your_db.* TO 'otel_user'@'%';
FLUSH PRIVILEGES;
```

## Quick Start Deployment

### Option 1: Docker Compose (Recommended)
```bash
# Clone repository
git clone <repo-url>
cd database-intelligence-mvp

# Start full stack
docker-compose up -d

# Verify deployment
docker-compose ps
curl http://localhost:8080/health
```

### Option 2: Binary Deployment
```bash
# Build collector
go build -o dist/database-intelligence-collector

# Run with basic config
./dist/database-intelligence-collector --config=config/collector.yaml
```

## Production Deployments

### Docker Production Stack
```bash
# Production compose with optimized settings
docker-compose -f docker-compose.production.yml up -d

# Includes:
# - Resource limits
# - Health checks  
# - Persistent volumes
# - Production logging
```

### Kubernetes Production
```bash
# Apply namespace and RBAC
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/rbac.yaml

# Deploy collector with config
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

# Enable autoscaling
kubectl apply -f k8s/hpa.yaml
```

### Helm Production Deployment
```bash
# Install chart
helm repo add database-intelligence ./deployments/helm/
helm install db-intelligence database-intelligence/database-intelligence \
  --namespace db-intelligence \
  --create-namespace \
  --values deployments/helm/database-intelligence/values-production.yaml
```

## Configuration Options

### Basic Configuration
```yaml
# config/collector.yaml
receivers:
  postgresql:
    endpoint: ${DB_HOST}:5432
    username: ${DB_USER}
    password: ${DB_PASSWORD}
    databases: [${DB_NAME}]
    collection_interval: 60s
    
  mysql:
    endpoint: ${DB_HOST}:3306
    username: ${DB_USER} 
    password: ${DB_PASSWORD}
    database: ${DB_NAME}
    collection_interval: 60s

processors:
  # All 7 processors enabled by default
  planattributeextractor:
    anonymization: true
    plan_threshold_ms: 100
    
  adaptivesampler:
    sampling_percentage: 10
    
  circuitbreaker:
    failure_threshold: 5
    timeout_seconds: 30

exporters:
  otlphttp/newrelic:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
```

### Enterprise Configuration
```yaml
# config/collector-enterprise.yaml  
processors:
  # Database processors
  planattributeextractor:
    anonymization: true
    pii_detection: true
    normalize_queries: true
    
  # Enterprise processors
  nrerrormonitor:
    error_threshold: 10
    severity_mapping: true
    
  costcontrol:
    cpu_limit_percent: 80
    memory_limit_mb: 2048
    budget_alerts: true
    
  querycorrelator:
    trace_correlation: true
    service_mapping: true

exporters:
  otlphttp/newrelic:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    tls:
      insecure: false
      cert_file: /etc/ssl/certs/client.crt
      key_file: /etc/ssl/private/client.key
```

## Environment-Specific Deployments

### Development Environment
```bash
# Use dev overlay
docker-compose -f docker-compose.yml -f config/overlays/dev/docker-compose.override.yml up -d

# Features:
# - Debug logging enabled
# - No sampling (100% data collection)
# - Local file exports
# - Hot configuration reload
```

### Staging Environment  
```bash
# Staging deployment
kubectl apply -k config/overlays/staging/

# Features:
# - Production-like configuration
# - Reduced resource limits
# - Synthetic data generation
# - Enhanced monitoring
```

### Production Environment
```bash
# Production deployment
kubectl apply -k config/overlays/production/

# Features:
# - High availability (3+ replicas)
# - Horizontal Pod Autoscaler
# - Network policies
# - Resource limits and requests
# - Persistent volumes
# - Monitoring and alerting
```

## Database-Specific Setup

### PostgreSQL with pg_querylens
```sql
-- Install extension
CREATE EXTENSION IF NOT EXISTS pg_querylens;

-- Configure extension
ALTER SYSTEM SET pg_querylens.track = 'all';
ALTER SYSTEM SET pg_querylens.max_plans = 1000;
SELECT pg_reload_conf();

-- Verify installation
SELECT * FROM pg_querylens_plans LIMIT 5;
```

```yaml
# Collector config for pg_querylens
processors:
  planattributeextractor:
    pg_querylens:
      enabled: true
      plan_threshold_ms: 100
      anonymization: true
      max_plans_per_query: 5
```

### MySQL Performance Schema
```sql
-- Enable Performance Schema
SET GLOBAL performance_schema = ON;

-- Configure statement tracking
UPDATE performance_schema.setup_consumers 
SET ENABLED = 'YES' 
WHERE NAME = 'statements_digest';

-- Verify setup
SELECT * FROM performance_schema.events_statements_summary_by_digest 
LIMIT 5;
```

## Security Hardening

### mTLS Configuration
```yaml
# Enterprise security config
exporters:
  otlphttp/newrelic:
    tls:
      insecure: false
      cert_file: /etc/ssl/certs/client.crt
      key_file: /etc/ssl/private/client.key
      ca_file: /etc/ssl/certs/ca.crt
      server_name_override: otlp.nr-data.net
```

### Kubernetes Security
```yaml
# Security context
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 2000
  capabilities:
    drop:
      - ALL
    add:
      - NET_BIND_SERVICE

# Network policy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: db-intelligence-netpol
spec:
  podSelector:
    matchLabels:
      app: database-intelligence
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: prometheus
  egress:
    - to: []
      ports:
        - protocol: TCP
          port: 5432  # PostgreSQL
        - protocol: TCP
          port: 3306  # MySQL
        - protocol: TCP
          port: 443   # OTLP export
```

## Monitoring & Observability

### Health Checks
```yaml
# Docker health check
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 60s

# Kubernetes health checks
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
```

### Metrics Collection
```yaml
# Prometheus metrics endpoint
service:
  telemetry:
    metrics:
      address: 0.0.0.0:8888
      level: detailed
```

### Logging Configuration
```yaml
# Structured logging
service:
  telemetry:
    logs:
      level: "info"
      development: false
      encoding: "json"
      output_paths: ["stdout"]
      error_output_paths: ["stderr"]
```

## Scaling and Performance

### Horizontal Scaling
```yaml
# Kubernetes HPA
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: db-intelligence-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: database-intelligence
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
```

### Resource Limits
```yaml
# Production resource limits
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

## Troubleshooting Deployment

### Common Issues

**Build Failures**
```bash
# Use specific Go version
go version  # Ensure 1.21+ (not 1.24.3)

# Clean build
go mod tidy
go build -o dist/database-intelligence-collector
```

**Database Connection Issues**
```bash
# Test database connectivity
docker run --rm -it postgres:13 psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT 1;"

# Check collector logs
docker logs <collector-container> 2>&1 | grep -i error
```

**OTLP Export Issues**
```bash
# Verify New Relic connectivity
curl -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
     -H "Content-Type: application/json" \
     https://otlp.nr-data.net/v1/metrics

# Check firewall/network policies
kubectl get networkpolicies
```

### Debug Configuration
```yaml
# Enable debug logging
service:
  telemetry:
    logs:
      level: "debug"
      
# Enable pprof endpoint
service:
  extensions: [pprof]
```

### Performance Validation
```bash
# Check processing latency
curl http://localhost:8888/metrics | grep otelcol_processor_accepted_spans

# Monitor resource usage
kubectl top pods -l app=database-intelligence

# Validate database impact
# Run before/after performance tests on database
```

## Backup and Recovery

### Configuration Backup
```bash
# Backup configurations
kubectl get configmap db-intelligence-config -o yaml > config-backup.yaml

# Backup secrets
kubectl get secret db-intelligence-secrets -o yaml > secrets-backup.yaml
```

### Data Recovery
```bash
# Restore from backup
kubectl apply -f config-backup.yaml
kubectl apply -f secrets-backup.yaml

# Restart deployment
kubectl rollout restart deployment/database-intelligence
```

---

**Deployment Status**: ✅ Production Ready  
**Tested Environments**: Docker, Kubernetes, Helm  
**Security Level**: Enterprise-grade with mTLS support  
**Scalability**: Horizontal scaling to 10+ replicas validated