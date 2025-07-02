# Production Deployment Guide

This guide provides comprehensive instructions for deploying the Database Intelligence Collector in production environments.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Architecture Overview](#architecture-overview)
3. [Pre-deployment Checklist](#pre-deployment-checklist)
4. [Deployment Methods](#deployment-methods)
5. [Configuration Management](#configuration-management)
6. [Security Considerations](#security-considerations)
7. [Performance Tuning](#performance-tuning)
8. [Monitoring and Observability](#monitoring-and-observability)
9. [Troubleshooting](#troubleshooting)
10. [Maintenance and Updates](#maintenance-and-updates)

## Prerequisites

### System Requirements

- **Kubernetes**: Version 1.21 or higher
- **Helm**: Version 3.8.0 or higher
- **Database Access**: 
  - PostgreSQL 12+ or MySQL 8.0+
  - Database user with appropriate monitoring permissions
- **New Relic Account**: Valid license key with OTLP ingest enabled
- **Resources**:
  - Minimum: 2 CPU cores, 4GB RAM per collector instance
  - Recommended: 4 CPU cores, 8GB RAM per collector instance

### Database Permissions

#### PostgreSQL
```sql
-- Create monitoring user
CREATE USER monitoring WITH PASSWORD 'secure_password';

-- Grant necessary permissions
GRANT pg_monitor TO monitoring;
GRANT USAGE ON SCHEMA pg_catalog TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO monitoring;

-- For pg_querylens functionality (REQUIRED for plan intelligence)
-- First install pg_querylens extension
CREATE EXTENSION IF NOT EXISTS pg_querylens;

-- Grant permissions on pg_querylens schema
GRANT USAGE ON SCHEMA pg_querylens TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_querylens TO monitoring;
ALTER DEFAULT PRIVILEGES IN SCHEMA pg_querylens GRANT SELECT ON TABLES TO monitoring;

-- Configure pg_querylens
ALTER SYSTEM SET pg_querylens.enabled = 'on';
ALTER SYSTEM SET pg_querylens.track_planning = 'on';
ALTER SYSTEM SET pg_querylens.max_plan_length = 10000;
ALTER SYSTEM SET pg_querylens.plan_format = 'json';

-- For auto_explain functionality (optional)
ALTER SYSTEM SET shared_preload_libraries = 'pg_querylens,auto_explain';
ALTER SYSTEM SET auto_explain.log_min_duration = '100ms';
ALTER SYSTEM SET auto_explain.log_analyze = 'on';
ALTER SYSTEM SET auto_explain.log_buffers = 'on';
-- Restart PostgreSQL after these changes
```

#### MySQL
```sql
-- Create monitoring user
CREATE USER 'monitoring'@'%' IDENTIFIED BY 'secure_password';

-- Grant necessary permissions
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'monitoring'@'%';
GRANT SELECT ON performance_schema.* TO 'monitoring'@'%';
GRANT SELECT ON mysql.* TO 'monitoring'@'%';
```

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         Production Cluster                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐     ┌─────────────────┐                  │
│  │   PostgreSQL    │     │     MySQL       │                  │
│  │   Instances     │     │   Instances     │                  │
│  └────────┬────────┘     └────────┬────────┘                  │
│           │                       │                            │
│           └───────────┬───────────┘                            │
│                       │                                        │
│              ┌────────▼────────┐                              │
│              │   Collector     │                              │
│              │   Deployment    │                              │
│              │  (Replicated)   │                              │
│              └────────┬────────┘                              │
│                       │                                        │
│                       │ OTLP/gRPC                             │
│                       │                                        │
└───────────────────────┼─────────────────────────────────────┘
                        │
                        ▼
                ┌──────────────┐
                │  New Relic   │
                │   Platform   │
                └──────────────┘
```

## Pre-deployment Checklist

- [ ] Database credentials prepared and tested
- [ ] pg_querylens extension installed and configured
- [ ] New Relic license key obtained
- [ ] Network connectivity verified (database → collector → New Relic)
- [ ] Kubernetes namespace created
- [ ] Resource quotas defined
- [ ] Security policies reviewed
- [ ] Backup and rollback procedures documented
- [ ] Change management process followed
- [ ] Monitoring dashboards prepared
- [ ] pg_querylens permissions verified
- [ ] Plan regression thresholds configured

## Deployment Methods

### Method 1: Helm Chart Deployment (Recommended)

#### 1. Add Helm Repository
```bash
helm repo add database-intelligence https://database-intelligence-mvp.github.io/helm-charts
helm repo update
```

#### 2. Create Values File
Create `production-values.yaml`:
```yaml
# Production configuration
replicaCount: 3

image:
  repository: ghcr.io/database-intelligence-mvp/database-intelligence-collector
  tag: "1.0.0"
  pullPolicy: IfNotPresent

config:
  postgres:
    enabled: true
    endpoint: postgres-primary.production.svc.cluster.local
    port: 5432
    username: monitoring
    # Use secrets for passwords
    existingSecret: postgres-monitoring-secret
    passwordKey: password
    database: postgres
    sslmode: require
    collectionInterval: 30s
    
  # pg_querylens integration
  querylens:
    enabled: true
    collectionInterval: 30s
    planHistoryHours: 24
    regressionDetection:
      enabled: true
      timeIncrease: 1.5      # 50% slower triggers detection
      ioIncrease: 2.0        # 100% more I/O triggers detection
      costIncrease: 2.0      # 100% higher cost triggers detection
    alertOnRegression: true
    
  mysql:
    enabled: false  # Enable if needed
    
  newrelic:
    # Use secret for license key
    existingSecret: newrelic-secret
    licenseKeyKey: license-key
    endpoint: otlp.nr-data.net:4317
    environment: production
    
  sampling:
    enabled: true
    defaultRate: 0.1  # 10% sampling
    rules:
      - name: slow_queries
        expression: 'attributes["db.statement.duration"] > 1000'
        sampleRate: 1.0  # 100% for slow queries
      - name: errors
        expression: 'attributes["db.statement.error"] != ""'
        sampleRate: 1.0  # 100% for errors
        
  piiDetection:
    enabled: true
    action: redact
    customPatterns:
      - name: internal_ids
        pattern: 'ID-[0-9]{10}'
        
  circuitBreaker:
    enabled: true
    threshold: 0.5
    timeout: 30s
    halfOpenRequests: 10

resources:
  limits:
    cpu: 2
    memory: 4Gi
  requests:
    cpu: 1
    memory: 2Gi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPU: 70
  targetMemory: 80
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

persistence:
  enabled: true
  size: 50Gi
  storageClass: fast-ssd
  
serviceAccount:
  create: true
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/database-intelligence

podAnnotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8888"

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL

networkPolicy:
  enabled: true
  allowExternal: false
  
metrics:
  enabled: true
  serviceMonitor:
    enabled: true
    namespace: monitoring
    interval: 30s

nodeSelector:
  node-role.kubernetes.io/monitoring: "true"

tolerations:
  - key: "monitoring"
    operator: "Equal"
    value: "true"
    effect: "NoSchedule"

affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - database-intelligence
        topologyKey: kubernetes.io/hostname
```

#### 3. Create Secrets
```bash
# Create PostgreSQL secret
kubectl create secret generic postgres-monitoring-secret \
  --from-literal=password='your-secure-password' \
  -n database-intelligence

# Create New Relic secret
kubectl create secret generic newrelic-secret \
  --from-literal=license-key='your-license-key' \
  -n database-intelligence
```

#### 4. Deploy
```bash
helm install database-intelligence \
  database-intelligence/database-intelligence \
  -f production-values.yaml \
  -n database-intelligence \
  --create-namespace
```

### Method 2: Kubernetes Manifests

#### 1. Apply Manifests
```bash
# Clone repository
git clone https://github.com/database-intelligence-mvp/database-intelligence-collector.git
cd database-intelligence-collector

# Apply manifests
kubectl apply -f deployments/kubernetes/namespace.yaml
kubectl apply -f deployments/kubernetes/secret.yaml
kubectl apply -f deployments/kubernetes/configmap.yaml
kubectl apply -f deployments/kubernetes/rbac.yaml
kubectl apply -f deployments/kubernetes/deployment.yaml
kubectl apply -f deployments/kubernetes/service.yaml
kubectl apply -f deployments/kubernetes/hpa.yaml
kubectl apply -f deployments/kubernetes/networkpolicy.yaml
```

### Method 3: GitOps Deployment

#### ArgoCD Application
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: database-intelligence
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/database-intelligence-mvp/database-intelligence-collector
    targetRevision: v1.0.0
    path: deployments/helm/database-intelligence
    helm:
      valueFiles:
      - values-production.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: database-intelligence
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```

## Configuration Management

### Environment-Specific Configurations

#### Development
```yaml
config:
  logLevel: debug
  sampling:
    defaultRate: 1.0  # 100% sampling
  circuitBreaker:
    enabled: false
```

#### Staging
```yaml
config:
  logLevel: info
  sampling:
    defaultRate: 0.5  # 50% sampling
  circuitBreaker:
    enabled: true
    threshold: 0.3
```

#### Production
```yaml
config:
  logLevel: warn
  sampling:
    defaultRate: 0.1  # 10% sampling
  circuitBreaker:
    enabled: true
    threshold: 0.5
```

### Dynamic Configuration Updates

The collector supports hot-reloading of configuration:

```bash
# Update ConfigMap
kubectl edit configmap database-intelligence-config -n database-intelligence

# Trigger reload
kubectl rollout restart deployment/database-intelligence -n database-intelligence
```

## Security Considerations

### 1. Network Security

#### Ingress Rules
```yaml
# Allow only from database subnets
networkPolicy:
  ingress:
    - from:
        - ipBlock:
            cidr: 10.0.0.0/16  # Database subnet
```

#### Egress Rules
```yaml
# Restrict outbound to New Relic only
networkPolicy:
  egress:
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
            - 10.0.0.0/8
            - 172.16.0.0/12
            - 192.168.0.0/16
      ports:
      - protocol: TCP
        port: 4317
```

### 2. Secrets Management

#### Using External Secrets Operator
```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault-backend
spec:
  provider:
    vault:
      server: "https://vault.example.com"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "database-intelligence"
---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: database-credentials
spec:
  secretStoreRef:
    name: vault-backend
  target:
    name: postgres-monitoring-secret
  data:
    - secretKey: password
      remoteRef:
        key: database/postgres/monitoring
        property: password
```

### 3. RBAC Configuration

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: database-intelligence
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "watch", "list"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
  resourceNames: ["postgres-monitoring-secret", "newrelic-secret"]
```

### 4. Pod Security Standards

```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: database-intelligence
spec:
  privileged: false
  allowPrivilegeEscalation: false
  requiredDropCapabilities:
    - ALL
  volumes:
    - 'configMap'
    - 'emptyDir'
    - 'projected'
    - 'secret'
    - 'persistentVolumeClaim'
  hostNetwork: false
  hostIPC: false
  hostPID: false
  runAsUser:
    rule: 'MustRunAsNonRoot'
  seLinux:
    rule: 'RunAsAny'
  fsGroup:
    rule: 'RunAsAny'
  readOnlyRootFilesystem: true
```

## Performance Tuning

### 1. Resource Optimization

#### CPU and Memory
```yaml
resources:
  requests:
    cpu: "1"      # Guaranteed 1 CPU
    memory: "2Gi" # Guaranteed 2GB RAM
  limits:
    cpu: "2"      # Max 2 CPUs
    memory: "4Gi" # Max 4GB RAM
```

#### JVM Settings (if applicable)
```yaml
env:
  - name: JAVA_OPTS
    value: "-Xms2g -Xmx2g -XX:+UseG1GC -XX:MaxGCPauseMillis=100"
```

### 2. Collection Tuning

#### Batch Processing
```yaml
processors:
  batch:
    timeout: 10s
    send_batch_size: 1000
    send_batch_max_size: 2000
```

#### Memory Limiter
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 3072
    spike_limit_mib: 512
```

### 3. Database Connection Pooling

```yaml
config:
  postgres:
    connectionPool:
      maxOpen: 10
      maxIdle: 5
      maxLifetime: 30m
      idleTimeout: 10m
```

### 4. Sampling Strategies

#### Adaptive Sampling
```yaml
sampling:
  adaptive:
    enabled: true
    targetRate: 1000  # Target 1000 samples/sec
    rules:
      - priority: 1
        condition: 'attributes["db.statement.error"] != ""'
        sampleRate: 1.0
      - priority: 2
        condition: 'attributes["db.statement.duration"] > 1000'
        sampleRate: 0.5
      - priority: 3
        condition: 'attributes["db.statement.duration"] > 100'
        sampleRate: 0.1
```

## Monitoring and Observability

### 1. Health Checks

#### Liveness Probe
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 13133
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

#### Readiness Probe
```yaml
readinessProbe:
  httpGet:
    path: /health/ready
    port: 13133
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

### 2. Metrics Endpoints

```bash
# Collector metrics
curl http://collector-service:8888/metrics

# Prometheus metrics
curl http://collector-service:8889/metrics

# Health status
curl http://collector-service:13133/health
```

### 3. Logging Configuration

```yaml
config:
  logging:
    level: info
    format: json
    outputs:
      - stdout
    fields:
      service: database-intelligence
      environment: production
```

### 4. Distributed Tracing

```yaml
config:
  tracing:
    enabled: true
    samplingRate: 0.01  # 1% of requests
    exporter: otlp
    endpoint: trace-collector.monitoring:4317
```

## Troubleshooting

### Common Issues

#### 1. Connection Failures
```bash
# Check connectivity
kubectl exec -it deployment/database-intelligence -- nc -zv postgres-host 5432

# Check DNS resolution
kubectl exec -it deployment/database-intelligence -- nslookup postgres-host

# View logs
kubectl logs -f deployment/database-intelligence -n database-intelligence
```

#### 2. High Memory Usage
```bash
# Check memory usage
kubectl top pod -n database-intelligence

# Analyze heap dump (if JVM-based)
kubectl exec -it pod/database-intelligence-xxx -- jmap -dump:format=b,file=/tmp/heap.hprof 1
kubectl cp database-intelligence-xxx:/tmp/heap.hprof ./heap.hprof
```

#### 3. Slow Performance
```bash
# Enable debug logging
kubectl set env deployment/database-intelligence LOG_LEVEL=debug

# Check processing metrics
curl http://localhost:8888/metrics | grep -E 'processed|dropped|failed'
```

### Debug Mode

Enable debug mode for detailed diagnostics:
```yaml
config:
  debug:
    enabled: true
    verbosity: detailed
    endpoints:
      - traces
      - metrics
    sampling: 1.0
```

## Maintenance and Updates

### 1. Rolling Updates

```bash
# Update image
kubectl set image deployment/database-intelligence \
  database-intelligence=ghcr.io/database-intelligence-mvp/database-intelligence-collector:v1.1.0 \
  -n database-intelligence

# Monitor rollout
kubectl rollout status deployment/database-intelligence -n database-intelligence
```

### 2. Backup Procedures

```bash
# Backup configuration
kubectl get configmap database-intelligence-config -o yaml > config-backup.yaml
kubectl get secret -n database-intelligence -o yaml > secrets-backup.yaml

# Backup persistent data
kubectl exec -it database-intelligence-0 -- tar czf /tmp/data-backup.tar.gz /var/lib/collector
kubectl cp database-intelligence-0:/tmp/data-backup.tar.gz ./data-backup.tar.gz
```

### 3. Rollback Procedures

```bash
# Rollback to previous version
kubectl rollout undo deployment/database-intelligence -n database-intelligence

# Rollback to specific revision
kubectl rollout undo deployment/database-intelligence --to-revision=2 -n database-intelligence
```

### 4. Maintenance Windows

```bash
# Scale down during maintenance
kubectl scale deployment/database-intelligence --replicas=0 -n database-intelligence

# Perform maintenance...

# Scale back up
kubectl scale deployment/database-intelligence --replicas=3 -n database-intelligence
```

## Disaster Recovery

### 1. Backup Strategy
- Configuration: Daily backups to S3/GCS
- Secrets: Encrypted backups to secure storage
- Persistent data: Snapshot-based backups

### 2. Recovery Procedures
1. Restore namespace and RBAC
2. Restore secrets from backup
3. Restore ConfigMaps
4. Deploy collector with saved configuration
5. Verify connectivity and data flow

### 3. Business Continuity
- Multi-region deployment for HA
- Automated failover procedures
- Regular DR drills
- Documentation and runbooks

## Support and Resources

- **Documentation**: https://github.com/database-intelligence-mvp/database-intelligence-collector/docs
- **Issue Tracker**: https://github.com/database-intelligence-mvp/database-intelligence-collector/issues
- **Community Forum**: https://discuss.newrelic.com/c/database-intelligence
- **Emergency Support**: support@database-intelligence.io

## Appendix

### A. Complete Production Checklist

- [ ] Database permissions verified
- [ ] pg_querylens extension installed and configured
- [ ] pg_querylens schema permissions granted
- [ ] Network connectivity tested
- [ ] Secrets management configured
- [ ] Resource limits set appropriately
- [ ] Autoscaling configured
- [ ] Monitoring dashboards created
- [ ] Plan regression alerts configured
- [ ] Backup procedures tested
- [ ] Rollback procedures documented
- [ ] Security policies applied
- [ ] Performance baselines established
- [ ] Documentation updated
- [ ] Team trained on operations
- [ ] Support contacts documented
- [ ] Change management completed

### B. pg_querylens Verification

```sql
-- Verify pg_querylens is installed
SELECT * FROM pg_extension WHERE extname = 'pg_querylens';

-- Check pg_querylens is collecting data
SELECT COUNT(*) FROM pg_querylens.queries;
SELECT COUNT(*) FROM pg_querylens.plans;

-- Verify monitoring user can access pg_querylens
SET ROLE monitoring;
SELECT * FROM pg_querylens.current_plans LIMIT 1;

-- Check for recent plan changes
SELECT 
  queryid,
  COUNT(DISTINCT plan_id) as plan_versions,
  MAX(last_execution) as last_seen
FROM pg_querylens.plans
WHERE last_execution > NOW() - INTERVAL '1 hour'
GROUP BY queryid
HAVING COUNT(DISTINCT plan_id) > 1;
```

### C. Troubleshooting pg_querylens

#### No Data in pg_querylens Tables
```sql
-- Check if extension is enabled
SHOW pg_querylens.enabled;

-- Check shared_preload_libraries
SHOW shared_preload_libraries;

-- Verify track_planning is on
SHOW pg_querylens.track_planning;
```

#### Permission Errors
```sql
-- Re-grant permissions
GRANT USAGE ON SCHEMA pg_querylens TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_querylens TO monitoring;
```

#### High Memory Usage from pg_querylens
```sql
-- Check plan storage size
SELECT 
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables 
WHERE schemaname = 'pg_querylens'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;

-- Clean old data if needed
DELETE FROM pg_querylens.plans 
WHERE last_execution < NOW() - INTERVAL '30 days';
```