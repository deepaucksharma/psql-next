# Database Intelligence MVP - Consolidated Deployment & Operations Guide

## Overview

This guide consolidates all deployment, infrastructure, and operational knowledge from the Database Intelligence MVP archive into a comprehensive production-ready reference.

## Deployment Architecture

### Infrastructure Evolution Summary

**Phase 1 - Custom Implementation**
- 30+ shell scripts for various operations
- 10+ docker-compose files for different environments
- Scattered configuration files with overlapping functionality
- Manual deployment processes with high operational overhead

**Phase 2 - Consolidation & Standardization**
- Unified Taskfile replacing shell scripts
- Single docker-compose with environment profiles
- Streamlined configuration management
- Automated deployment pipelines

**Phase 3 - Production Modernization**
- Kubernetes-ready Helm charts
- Infrastructure as Code with Terraform
- Comprehensive monitoring and observability
- Enterprise-grade security and compliance

## Current Production Architecture

### Deployment Models

#### 1. Single-Instance Deployment (Recommended for MVP)
```yaml
# Suitable for: Development, testing, small-scale production
resources:
  memory: 512Mi - 1Gi
  cpu: 250m - 500m
features:
  - In-memory state management
  - Local configuration
  - File-based state persistence
  - Simple monitoring
```

#### 2. High-Availability Deployment 
```yaml
# Suitable for: Large-scale production, enterprise environments
resources:
  memory: 1Gi - 2Gi per instance
  cpu: 500m - 1000m per instance
  replicas: 3+
features:
  - Distributed state management
  - Load balancing
  - Automatic failover
  - Advanced monitoring
```

### Infrastructure Components

#### Core Services
- **Database Intelligence Collector**: Main OTEL collector with custom processors
- **PostgreSQL/MySQL Databases**: Source databases being monitored
- **New Relic**: Destination for metrics and observability data
- **Monitoring Stack**: Prometheus, Grafana, health checks

#### Supporting Infrastructure
- **Container Registry**: Docker images for deployment
- **Configuration Management**: Environment-specific configurations
- **Secret Management**: Secure credential storage
- **Log Management**: Centralized logging and analysis

## Taskfile Implementation

### Complete Task Reference (50+ tasks)

#### Development Tasks
```yaml
dev:setup:
  desc: Complete development environment setup
  cmds:
    - task: deps:install
    - task: config:generate
    - task: db:start
    - task: collector:build

dev:start:
  desc: Start development environment with hot reload
  cmds:
    - docker-compose --profile development up -d
    - ./dist/collector --config config/collector-minimal.yaml
    
dev:test:
  desc: Run comprehensive test suite
  cmds:
    - task: test:unit
    - task: test:integration
    - task: test:e2e

dev:clean:
  desc: Clean development environment
  cmds:
    - docker-compose down -v
    - rm -rf dist/ logs/ metrics.json
```

#### Build & Release Tasks
```yaml
build:collector:
  desc: Build optimized collector binary
  cmds:
    - go mod tidy
    - go build -ldflags="-s -w" -o dist/collector .
    - chmod +x dist/collector

build:docker:
  desc: Build Docker image
  cmds:
    - docker build -t database-intelligence-collector:{{.VERSION}} .
    - docker tag database-intelligence-collector:{{.VERSION}} database-intelligence-collector:latest

release:prepare:
  desc: Prepare release artifacts
  cmds:
    - task: build:collector
    - task: build:docker
    - task: test:all
    - task: docs:generate
```

#### Deployment Tasks
```yaml
deploy:staging:
  desc: Deploy to staging environment
  cmds:
    - helm upgrade --install db-intelligence ./helm/database-intelligence-collector 
      --namespace staging --values helm/values-staging.yaml

deploy:prod:
  desc: Deploy to production
  cmds:
    - task: deploy:verify-prerequisites
    - helm upgrade --install db-intelligence ./helm/database-intelligence-collector 
      --namespace production --values helm/values-production.yaml
    - task: deploy:health-check

deploy:rollback:
  desc: Rollback production deployment
  cmds:
    - helm rollback db-intelligence --namespace production
    - task: deploy:health-check
```

#### Operations Tasks
```yaml
ops:health:
  desc: Comprehensive health check
  cmds:
    - curl -f http://localhost:13133/health || exit 1
    - task: ops:metrics-check
    - task: ops:database-connectivity

ops:metrics:
  desc: Collect operational metrics
  cmds:
    - curl -s http://localhost:8888/metrics > /tmp/collector-metrics.txt
    - echo "Metrics collected at $(date)"

ops:backup:
  desc: Backup configurations and state
  cmds:
    - tar -czf backup-$(date +%Y%m%d-%H%M%S).tar.gz config/ /var/lib/otel/
    - echo "Backup completed"

ops:logs:
  desc: Tail collector logs with filtering
  cmds:
    - tail -f /var/log/otel/collector.log | grep -E "(ERROR|WARN|processor)"
```

### Docker Compose Unification

#### Single Compose File with Profiles
```yaml
# docker-compose.yaml
version: '3.8'

services:
  collector:
    build: .
    image: database-intelligence-collector:latest
    profiles: [development, staging, production]
    environment:
      - ENVIRONMENT=${ENVIRONMENT:-development}
      - LOG_LEVEL=${LOG_LEVEL:-info}
    volumes:
      - ./config:/etc/otel:ro
      - collector-data:/var/lib/otel
      - ./logs:/var/log/otel
    ports:
      - "${METRICS_PORT:-8888}:8888"
      - "${HEALTH_PORT:-13133}:13133"
    depends_on:
      postgres:
        condition: service_healthy
      mysql:
        condition: service_healthy
    restart: unless-stopped
    
  postgres:
    image: postgres:13
    profiles: [development, staging]
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-testdb}
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-postgres}
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./tests/e2e/sql/postgres-init.sql:/docker-entrypoint-initdb.d/init.sql
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-postgres}"]
      interval: 30s
      timeout: 10s
      retries: 3
      
  mysql:
    image: mysql:8.0
    profiles: [development, staging]
    environment:
      MYSQL_DATABASE: ${MYSQL_DB:-testdb}
      MYSQL_USER: ${MYSQL_USER:-mysql}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD:-mysql}
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD:-mysql}
    volumes:
      - mysql-data:/var/lib/mysql
      - ./tests/e2e/sql/mysql-init.sql:/docker-entrypoint-initdb.d/init.sql
    ports:
      - "${MYSQL_PORT:-3306}:3306"
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 30s
      timeout: 10s
      retries: 3
      
  prometheus:
    image: prom/prometheus:latest
    profiles: [staging, production]
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    ports:
      - "9090:9090"
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      
  grafana:
    image: grafana/grafana:latest
    profiles: [staging, production]
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD:-admin}
    volumes:
      - grafana-data:/var/lib/grafana
      - ./monitoring/grafana:/etc/grafana/provisioning
    ports:
      - "3000:3000"

volumes:
  collector-data:
  postgres-data:
  mysql-data:
  prometheus-data:
  grafana-data:

networks:
  default:
    name: database-intelligence-network
```

#### Profile Usage Examples
```bash
# Development with local databases
docker-compose --profile development up -d

# Staging with monitoring
docker-compose --profile staging up -d

# Production (connects to external databases)
docker-compose --profile production up -d
```

## Kubernetes Deployment

### Helm Chart Architecture

#### Chart Structure
```
helm/database-intelligence-collector/
├── Chart.yaml                 # Chart metadata
├── values.yaml               # Default values
├── values-development.yaml   # Development overrides
├── values-staging.yaml       # Staging overrides  
├── values-production.yaml    # Production overrides
├── templates/
│   ├── deployment.yaml       # Main collector deployment
│   ├── configmap.yaml       # Configuration management
│   ├── secret.yaml          # Credential management
│   ├── service.yaml         # Service definition
│   ├── servicemonitor.yaml  # Prometheus monitoring
│   ├── hpa.yaml             # Horizontal pod autoscaler
│   ├── pdb.yaml             # Pod disruption budget
│   └── networkpolicy.yaml   # Network security
└── charts/                  # Sub-charts for dependencies
```

#### Production Values
```yaml
# values-production.yaml
replicaCount: 3

image:
  repository: database-intelligence-collector
  tag: "1.0.0"
  pullPolicy: IfNotPresent

config:
  environment: production
  logLevel: info
  collectorConfig: |
    # Full production configuration here
    
resources:
  limits:
    memory: 2Gi
    cpu: 1000m
  requests:
    memory: 1Gi
    cpu: 500m

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80

podDisruptionBudget:
  enabled: true
  minAvailable: 2

service:
  type: ClusterIP
  ports:
    metrics: 8888
    health: 13133

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: db-intelligence.company.com
      paths:
        - path: /
          pathType: Prefix

persistence:
  enabled: true
  size: 10Gi
  storageClass: ssd

monitoring:
  serviceMonitor:
    enabled: true
    interval: 30s
  grafana:
    dashboardsEnabled: true

security:
  podSecurityContext:
    runAsNonRoot: true
    runAsUser: 1000
    fsGroup: 2000
  securityContext:
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    capabilities:
      drop:
        - ALL

networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: monitoring
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              name: databases
```

#### Deployment Commands
```bash
# Install/upgrade production deployment
helm upgrade --install db-intelligence ./helm/database-intelligence-collector \
  --namespace production \
  --values helm/values-production.yaml \
  --wait --timeout 600s

# Rollback if needed
helm rollback db-intelligence --namespace production

# Check deployment status
kubectl get pods -n production -l app=database-intelligence-collector
kubectl logs -n production -l app=database-intelligence-collector --tail=100
```

## Configuration Management

### Environment-Specific Configuration Overlay

#### Base Configuration Template
```yaml
# config/base.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: collector-config-base
data:
  collector.yaml: |
    receivers:
      postgresql: &postgresql-base
        collection_interval: 10s
        transport: tcp
      mysql: &mysql-base
        collection_interval: 10s
        transport: tcp
    
    processors: &processors-base
      memory_limiter:
        check_interval: 5s
      batch:
        timeout: 1s
        send_batch_size: 1000
        
    service: &service-base
      telemetry:
        logs:
          level: info
        metrics:
          address: ":8888"
```

#### Environment Overlays
```yaml
# config/production-overlay.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: collector-config-production
data:
  collector.yaml: |
    receivers:
      postgresql:
        <<: *postgresql-base
        tls:
          ca_file: /etc/ssl/certs/ca.crt
          cert_file: /etc/ssl/certs/client.crt
          key_file: /etc/ssl/private/client.key
      mysql:
        <<: *mysql-base
        tls:
          ca_file: /etc/ssl/certs/ca.crt
          cert_file: /etc/ssl/certs/client.crt
          key_file: /etc/ssl/private/client.key
    
    processors:
      <<: *processors-base
      memory_limiter:
        limit_mib: 2048
        spike_limit_mib: 512
      circuit_breaker:
        failure_threshold: 5
        timeout: 30s
      adaptive_sampler:
        default_sampling_rate: 10
      verification:
        pii_detection:
          enabled: true
    
    service:
      <<: *service-base
      telemetry:
        logs:
          level: info
          output_paths: ["/var/log/otel/collector.log"]
```

### Secret Management
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: database-credentials
type: Opaque
stringData:
  postgres-url: "postgresql://user:password@postgres.database.svc.cluster.local:5432/testdb"
  mysql-url: "mysql://user:password@mysql.database.svc.cluster.local:3306/testdb"
  newrelic-license-key: "your-license-key"
```

## Monitoring & Observability

### Comprehensive Monitoring Stack

#### Prometheus Configuration
```yaml
# monitoring/prometheus.yml
global:
  scrape_interval: 30s
  evaluation_interval: 30s

rule_files:
  - "rules/*.yml"

scrape_configs:
  - job_name: 'database-intelligence-collector'
    static_configs:
      - targets: ['collector:8888']
    scrape_interval: 15s
    metrics_path: /metrics
    
  - job_name: 'database-intelligence-health'
    static_configs:
      - targets: ['collector:13133']
    scrape_interval: 30s
    metrics_path: /health
```

#### Grafana Dashboard Templates
```json
{
  "dashboard": {
    "title": "Database Intelligence Collector",
    "panels": [
      {
        "title": "Metrics Throughput",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(otel_collector_processor_accepted_metric_points_total[5m])",
            "legendFormat": "{{processor}} - accepted"
          }
        ]
      },
      {
        "title": "Processing Latency",
        "type": "graph", 
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(otel_collector_processor_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      }
    ]
  }
}
```

#### Alert Rules
```yaml
# monitoring/rules/alerts.yml
groups:
  - name: database-intelligence-collector
    rules:
      - alert: CollectorDown
        expr: up{job="database-intelligence-collector"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Database Intelligence Collector is down"
          
      - alert: HighMemoryUsage
        expr: otel_collector_process_memory_rss > 1073741824  # 1GB
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Collector memory usage is high"
          
      - alert: ProcessingErrors
        expr: rate(otel_collector_processor_dropped_metric_points_total[5m]) > 10
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High rate of processing errors"
```

## Production Deployment Procedures

### Pre-Deployment Checklist

#### Infrastructure Readiness
- [ ] Kubernetes cluster available and accessible
- [ ] Database connectivity verified (PostgreSQL/MySQL)
- [ ] New Relic account configured with license key
- [ ] DNS records configured for ingress
- [ ] SSL certificates provisioned
- [ ] Storage classes available for persistence

#### Configuration Validation
- [ ] Configuration files validated against schema
- [ ] Environment variables set correctly
- [ ] Secrets created and accessible
- [ ] Resource limits appropriate for environment
- [ ] Network policies configured for security

#### Testing Validation
- [ ] Unit tests passing
- [ ] Integration tests passing  
- [ ] E2E tests passing with New Relic validation
- [ ] Performance benchmarks meet requirements
- [ ] Security scans completed

### Deployment Process

#### 1. Staging Deployment
```bash
# Deploy to staging
task deploy:staging

# Validate deployment
task ops:health
task test:e2e-staging

# Performance validation
task test:performance-staging
```

#### 2. Production Deployment
```bash
# Final pre-deployment checks
task deploy:verify-prerequisites

# Deploy with rollback capability
helm upgrade --install db-intelligence ./helm/database-intelligence-collector \
  --namespace production \
  --values helm/values-production.yaml \
  --wait --timeout 600s \
  --atomic  # Automatic rollback on failure

# Post-deployment validation
task ops:health
task ops:metrics
```

#### 3. Health Validation
```bash
# Check all pods are running
kubectl get pods -n production -l app=database-intelligence-collector

# Validate metrics endpoint
curl -f http://db-intelligence.company.com/metrics

# Check New Relic data flow
# Wait 5 minutes then verify data in New Relic dashboards
```

### Rollback Procedures

#### Automatic Rollback (Helm)
```bash
# Rollback to previous version
helm rollback db-intelligence --namespace production

# Verify rollback success
kubectl get pods -n production -l app=database-intelligence-collector
task ops:health
```

#### Manual Rollback
```bash
# Scale down current deployment
kubectl scale deployment db-intelligence --replicas=0 -n production

# Deploy known good version
helm upgrade db-intelligence ./helm/database-intelligence-collector \
  --namespace production \
  --values helm/values-production.yaml \
  --set image.tag=last-known-good-version

# Scale up and validate
kubectl scale deployment db-intelligence --replicas=3 -n production
task ops:health
```

## Operational Procedures

### Daily Operations

#### Health Monitoring
```bash
# Morning health check
task ops:health
kubectl get pods -n production -l app=database-intelligence-collector

# Check resource utilization
kubectl top pods -n production -l app=database-intelligence-collector

# Verify New Relic data flow
# Check dashboards for data freshness (< 5 minutes old)
```

#### Log Management
```bash
# Check for errors in last hour
kubectl logs -n production -l app=database-intelligence-collector --since=1h | grep ERROR

# Monitor specific processor logs
kubectl logs -n production -l app=database-intelligence-collector --tail=100 | grep processor

# Collect logs for analysis
task ops:logs > daily-logs-$(date +%Y%m%d).txt
```

### Troubleshooting Procedures

#### Common Issues

**High Memory Usage**
```bash
# Check memory utilization
kubectl top pods -n production -l app=database-intelligence-collector

# Check memory_limiter processor configuration
kubectl get configmap collector-config -n production -o yaml | grep -A 10 memory_limiter

# Scale up memory limits if needed
helm upgrade db-intelligence ./helm/database-intelligence-collector \
  --namespace production \
  --set resources.limits.memory=4Gi
```

**Database Connectivity Issues**
```bash
# Test database connectivity from collector pod
kubectl exec -it deployment/db-intelligence -n production -- \
  psql -h postgres.database.svc.cluster.local -U postgres -d testdb -c "SELECT 1"

# Check circuit breaker status
curl -s http://db-intelligence.company.com/metrics | grep circuit_breaker

# Review database credentials
kubectl get secret database-credentials -n production -o yaml
```

**Processing Delays**
```bash
# Check processing metrics
curl -s http://db-intelligence.company.com/metrics | grep processor_duration

# Check queue depths
curl -s http://db-intelligence.company.com/metrics | grep queue_size

# Review batch processor configuration
kubectl get configmap collector-config -n production -o yaml | grep -A 5 batch
```

### Maintenance Procedures

#### Planned Maintenance
```bash
# Scale down to single instance
kubectl scale deployment db-intelligence --replicas=1 -n production

# Perform maintenance (configuration updates, etc.)
helm upgrade db-intelligence ./helm/database-intelligence-collector \
  --namespace production \
  --values helm/values-production.yaml

# Scale back up
kubectl scale deployment db-intelligence --replicas=3 -n production

# Validate operation
task ops:health
```

#### Emergency Procedures
```bash
# Emergency shutdown
kubectl scale deployment db-intelligence --replicas=0 -n production

# Emergency restart with minimal configuration
helm upgrade db-intelligence ./helm/database-intelligence-collector \
  --namespace production \
  --values helm/values-minimal.yaml

# Gradual restoration
# After resolving issues, restore full configuration
helm upgrade db-intelligence ./helm/database-intelligence-collector \
  --namespace production \
  --values helm/values-production.yaml
```

## Performance Optimization

### Resource Optimization

#### Memory Management
- **Baseline**: 512Mi request, 1Gi limit for development
- **Production**: 1Gi request, 2Gi limit for production workloads
- **High-scale**: 2Gi request, 4Gi limit for large deployments

#### CPU Optimization
- **Baseline**: 250m request, 500m limit
- **Production**: 500m request, 1000m limit
- **High-throughput**: 1000m request, 2000m limit

#### Storage Considerations
- **State files**: 1Gi persistent storage for adaptive sampler state
- **Logs**: 5Gi for log retention (7 days)
- **Metrics cache**: Memory-based, no persistent storage needed

### Configuration Tuning

#### Batch Processing
```yaml
batch:
  timeout: 1s          # Low latency
  send_batch_size: 1000 # Balanced throughput
  send_batch_max_size: 1500 # Prevent memory spikes
```

#### Memory Limiter
```yaml
memory_limiter:
  limit_mib: 1024      # Match container limit
  spike_limit_mib: 256 # Allow temporary spikes
  check_interval: 5s   # Frequent checks
```

## Security Implementation

### Network Security
```yaml
# NetworkPolicy for production
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: db-intelligence-network-policy
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
          port: 8888
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              name: databases
      ports:
        - protocol: TCP
          port: 5432
        - protocol: TCP
          port: 3306
```

### Pod Security
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 2000
  seccompProfile:
    type: RuntimeDefault
containerSecurityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

---

**Document Status**: Production Ready  
**Last Updated**: 2025-06-30  
**Coverage**: Complete consolidation of all deployment and operational procedures