# Deployment Guide

## Overview

Production deployment guide for the Database Intelligence OpenTelemetry Collector across different environments and platforms.

## Quick Deployment Options

### 1. Binary Deployment
```bash
# Download pre-built binary
wget https://github.com/database-intelligence/releases/download/v2.0.0/database-intelligence-collector-linux-amd64.tar.gz

# Extract and configure
tar -xzf database-intelligence-collector-linux-amd64.tar.gz
cd database-intelligence-collector

# Set environment variables
export NEW_RELIC_LICENSE_KEY=your_license_key
export POSTGRES_PASSWORD=your_postgres_password

# Run collector
./database-intelligence-collector --config=config/production.yaml
```

### 2. Docker Deployment
```bash
# Using Docker Compose
docker-compose up -d

# Or standalone container
docker run -d \
  --name database-intelligence \
  -e NEW_RELIC_LICENSE_KEY=your_license_key \
  -e POSTGRES_HOST=postgres.example.com \
  -e POSTGRES_PASSWORD=your_password \
  -p 13133:13133 \
  -p 8888:8888 \
  database-intelligence:latest
```

### 3. Kubernetes Deployment
```bash
# Apply Kubernetes manifests
kubectl apply -f deployments/kubernetes/

# Or using Helm
helm install database-intelligence ./deployments/helm/
```

## Docker Deployment

### Docker Compose (Recommended)
```yaml
# docker-compose.yml
version: '3.8'

services:
  database-intelligence:
    image: database-intelligence:latest
    container_name: db-intelligence-collector
    restart: unless-stopped
    environment:
      - NEW_RELIC_LICENSE_KEY=${NEW_RELIC_LICENSE_KEY}
      - NEW_RELIC_ACCOUNT_ID=${NEW_RELIC_ACCOUNT_ID}
      - POSTGRES_HOST=${POSTGRES_HOST}
      - POSTGRES_PORT=${POSTGRES_PORT:-5432}
      - POSTGRES_USER=${POSTGRES_USER}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=${POSTGRES_DB}
      - MYSQL_HOST=${MYSQL_HOST}
      - MYSQL_PORT=${MYSQL_PORT:-3306}
      - MYSQL_USER=${MYSQL_USER}
      - MYSQL_PASSWORD=${MYSQL_PASSWORD}
      - MYSQL_DB=${MYSQL_DB}
      - ENVIRONMENT=${ENVIRONMENT:-production}
      - LOG_LEVEL=${LOG_LEVEL:-info}
    ports:
      - "13133:13133"  # Health check
      - "8888:8888"    # Metrics endpoint
      - "4317:4317"    # OTLP gRPC
      - "4318:4318"    # OTLP HTTP
    volumes:
      - ./config:/etc/otelcol/config:ro
      - ./logs:/var/log/otelcol
    networks:
      - monitoring
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:13133/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  # Optional: Include databases for testing
  postgres:
    image: postgres:15-alpine
    container_name: db-intelligence-postgres
    environment:
      POSTGRES_DB: testdb
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init-scripts:/docker-entrypoint-initdb.d
    networks:
      - monitoring

  mysql:
    image: mysql:8.0
    container_name: db-intelligence-mysql
    environment:
      MYSQL_ROOT_PASSWORD: mysql
      MYSQL_DATABASE: testdb
      MYSQL_USER: mysql
      MYSQL_PASSWORD: mysql
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
    networks:
      - monitoring

volumes:
  postgres_data:
  mysql_data:

networks:
  monitoring:
    driver: bridge
```

### Environment File
```bash
# .env
NEW_RELIC_LICENSE_KEY=your_license_key_here
NEW_RELIC_ACCOUNT_ID=1234567
POSTGRES_HOST=postgres
POSTGRES_USER=postgres
POSTGRES_PASSWORD=secure_password
POSTGRES_DB=production_db
MYSQL_HOST=mysql
MYSQL_USER=root
MYSQL_PASSWORD=secure_password
MYSQL_DB=production_db
ENVIRONMENT=production
LOG_LEVEL=info
```

### Custom Dockerfile
```dockerfile
FROM alpine:3.18

# Install dependencies
RUN apk add --no-cache \
    ca-certificates \
    curl \
    tzdata

# Create non-root user
RUN addgroup -g 1001 otelcol && \
    adduser -D -u 1001 -G otelcol otelcol

# Copy binary and config
COPY dist/database-intelligence-collector /usr/local/bin/
COPY config/ /etc/otelcol/config/

# Set permissions
RUN chmod +x /usr/local/bin/database-intelligence-collector && \
    chown -R otelcol:otelcol /etc/otelcol

# Switch to non-root user
USER otelcol

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:13133/health || exit 1

# Expose ports
EXPOSE 13133 8888 4317 4318

# Run collector
ENTRYPOINT ["/usr/local/bin/database-intelligence-collector"]
CMD ["--config=/etc/otelcol/config/production.yaml"]
```

## Kubernetes Deployment

### Namespace and ConfigMap
```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: database-intelligence
  labels:
    name: database-intelligence

---
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: db-intelligence-config
  namespace: database-intelligence
data:
  collector.yaml: |
    receivers:
      postgresql:
        endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
        username: ${POSTGRES_USER}
        password: ${POSTGRES_PASSWORD}
        databases: 
          - ${POSTGRES_DB}
        collection_interval: 10s

    processors:
      memory_limiter:
        limit_mib: 512
        spike_limit_mib: 128
      batch:
        timeout: 10s
        send_batch_size: 1024
      resource:
        attributes:
          - key: k8s.cluster.name
            value: ${K8S_CLUSTER_NAME}
            action: insert
          - key: k8s.namespace.name
            value: database-intelligence
            action: insert

    exporters:
      otlp:
        endpoint: otlp.nr-data.net:4317
        headers:
          api-key: ${NEW_RELIC_LICENSE_KEY}
        compression: gzip

    extensions:
      health_check:
        endpoint: 0.0.0.0:13133

    service:
      extensions: [health_check]
      pipelines:
        metrics:
          receivers: [postgresql]
          processors: [memory_limiter, resource, batch]
          exporters: [otlp]
      telemetry:
        logs:
          level: info
        metrics:
          address: 0.0.0.0:8888
```

### Secret Management
```yaml
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: db-intelligence-secrets
  namespace: database-intelligence
type: Opaque
stringData:
  new-relic-license-key: "your_license_key_here"
  postgres-password: "your_postgres_password"
  mysql-password: "your_mysql_password"
```

### Deployment
```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: database-intelligence
  namespace: database-intelligence
  labels:
    app: database-intelligence
spec:
  replicas: 3
  selector:
    matchLabels:
      app: database-intelligence
  template:
    metadata:
      labels:
        app: database-intelligence
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8888"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: database-intelligence
      securityContext:
        runAsNonRoot: true
        runAsUser: 1001
        fsGroup: 1001
      containers:
      - name: collector
        image: database-intelligence:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 13133
          name: health
          protocol: TCP
        - containerPort: 8888
          name: metrics
          protocol: TCP
        - containerPort: 4317
          name: otlp-grpc
          protocol: TCP
        - containerPort: 4318
          name: otlp-http
          protocol: TCP
        env:
        - name: NEW_RELIC_LICENSE_KEY
          valueFrom:
            secretKeyRef:
              name: db-intelligence-secrets
              key: new-relic-license-key
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: db-intelligence-secrets
              key: postgres-password
        - name: POSTGRES_HOST
          value: "postgres.database.svc.cluster.local"
        - name: POSTGRES_PORT
          value: "5432"
        - name: POSTGRES_USER
          value: "postgres"
        - name: POSTGRES_DB
          value: "production"
        - name: K8S_CLUSTER_NAME
          value: "production-cluster"
        volumeMounts:
        - name: config
          mountPath: /etc/otelcol/config
          readOnly: true
        resources:
          requests:
            memory: 512Mi
            cpu: 500m
          limits:
            memory: 1Gi
            cpu: 1000m
        livenessProbe:
          httpGet:
            path: /health
            port: 13133
          initialDelaySeconds: 30
          periodSeconds: 30
          timeoutSeconds: 10
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health
            port: 13133
          initialDelaySeconds: 10
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL
      volumes:
      - name: config
        configMap:
          name: db-intelligence-config
      restartPolicy: Always
      terminationGracePeriodSeconds: 30
```

### Service and Ingress
```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: database-intelligence
  namespace: database-intelligence
  labels:
    app: database-intelligence
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8888"
spec:
  selector:
    app: database-intelligence
  ports:
  - name: health
    port: 13133
    targetPort: 13133
    protocol: TCP
  - name: metrics
    port: 8888
    targetPort: 8888
    protocol: TCP
  - name: otlp-grpc
    port: 4317
    targetPort: 4317
    protocol: TCP
  - name: otlp-http
    port: 4318
    targetPort: 4318
    protocol: TCP
  type: ClusterIP

---
# serviceaccount.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: database-intelligence
  namespace: database-intelligence

---
# rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: database-intelligence
rules:
- apiGroups: [""]
  resources: ["pods", "nodes", "services", "endpoints"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: database-intelligence
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: database-intelligence
subjects:
- kind: ServiceAccount
  name: database-intelligence
  namespace: database-intelligence
```

### Horizontal Pod Autoscaler
```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: database-intelligence
  namespace: database-intelligence
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: database-intelligence
  minReplicas: 3
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
        value: 10
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
```

## Helm Deployment

### Chart.yaml
```yaml
apiVersion: v2
name: database-intelligence
description: Database Intelligence OpenTelemetry Collector
type: application
version: 2.0.0
appVersion: "2.0.0"
keywords:
  - opentelemetry
  - database
  - monitoring
  - postgresql
  - mysql
  - newrelic
maintainers:
  - name: Database Intelligence Team
sources:
  - https://github.com/database-intelligence/database-intelligence-collector
```

### values.yaml
```yaml
# Default values for database-intelligence
replicaCount: 3

image:
  repository: database-intelligence
  pullPolicy: IfNotPresent
  tag: "latest"

nameOverride: ""
fullnameOverride: ""

serviceAccount:
  create: true
  annotations: {}
  name: ""

podAnnotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8888"
  prometheus.io/path: "/metrics"

podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1001
  fsGroup: 1001

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL

service:
  type: ClusterIP
  ports:
    health: 13133
    metrics: 8888
    otlpGrpc: 4317
    otlpHttp: 4318

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80

# Database configuration
postgresql:
  enabled: true
  host: postgres.database.svc.cluster.local
  port: 5432
  username: postgres
  database: production
  # Password from secret

mysql:
  enabled: true
  host: mysql.database.svc.cluster.local
  port: 3306
  username: root
  database: production
  # Password from secret

# New Relic configuration
newrelic:
  # License key from secret
  accountId: "1234567"
  endpoint: "otlp.nr-data.net:4317"

# OpenTelemetry configuration
otelcol:
  config:
    receivers:
      postgresql:
        collection_interval: 10s
      mysql:
        collection_interval: 10s
    processors:
      memory_limiter:
        limit_mib: 512
        spike_limit_mib: 128
      batch:
        timeout: 10s
        send_batch_size: 1024
    exporters:
      otlp:
        compression: gzip
        retry_on_failure:
          enabled: true
    extensions:
      health_check:
        endpoint: 0.0.0.0:13133

# Secrets (override in production)
secrets:
  newRelicLicenseKey: "your_license_key_here"
  postgresPassword: "your_postgres_password"
  mysqlPassword: "your_mysql_password"
```

### Install with Helm
```bash
# Add Helm repository (if published)
helm repo add database-intelligence https://charts.database-intelligence.com
helm repo update

# Install from repository
helm install my-db-intelligence database-intelligence/database-intelligence \
  --namespace database-intelligence \
  --create-namespace \
  --set secrets.newRelicLicenseKey="your_license_key" \
  --set postgresql.host="your-postgres-host" \
  --set secrets.postgresPassword="your_postgres_password"

# Or install from local chart
helm install my-db-intelligence ./deployments/helm/ \
  --namespace database-intelligence \
  --create-namespace \
  --values ./deployments/helm/values-production.yaml
```

## Production Configuration

### Security Hardening
```yaml
# security-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: security-config
data:
  collector.yaml: |
    receivers:
      postgresql:
        endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
        username: ${POSTGRES_USER}
        password: ${POSTGRES_PASSWORD}
        ssl_mode: require
        ssl_cert: /certs/client.crt
        ssl_key: /certs/client.key
        ssl_ca: /certs/ca.crt

    processors:
      verification:
        pii_detection:
          enabled: true
          patterns: [email, ssn, credit_card, phone]
          action: redact
        data_quality:
          required_fields: [db.name, db.system]

    extensions:
      health_check:
        endpoint: 0.0.0.0:13133
      pprof:
        endpoint: localhost:1777  # Localhost only
```

### Network Policies
```yaml
# network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: database-intelligence
  namespace: database-intelligence
spec:
  podSelector:
    matchLabels:
      app: database-intelligence
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
      port: 8888  # Metrics
    - protocol: TCP
      port: 13133  # Health
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 5432  # PostgreSQL
    - protocol: TCP
      port: 3306  # MySQL
    - protocol: TCP
      port: 4317  # New Relic OTLP
    - protocol: TCP
      port: 443   # HTTPS
    - protocol: TCP
      port: 53    # DNS
    - protocol: UDP
      port: 53    # DNS
```

## Monitoring and Alerting

### Prometheus Monitoring
```yaml
# servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: database-intelligence
  namespace: database-intelligence
  labels:
    app: database-intelligence
spec:
  selector:
    matchLabels:
      app: database-intelligence
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

### Grafana Dashboard
```json
{
  "dashboard": {
    "title": "Database Intelligence Collector",
    "panels": [
      {
        "title": "Metrics Processed",
        "type": "stat",
        "targets": [
          {
            "expr": "rate(otelcol_processor_accepted_metric_points_total[5m])",
            "legendFormat": "{{ processor }}"
          }
        ]
      },
      {
        "title": "Memory Usage",
        "type": "graph",
        "targets": [
          {
            "expr": "process_resident_memory_bytes{job=\"database-intelligence\"}",
            "legendFormat": "Memory Usage"
          }
        ]
      }
    ]
  }
}
```

### Alerting Rules
```yaml
# alerts.yaml
groups:
- name: database-intelligence
  rules:
  - alert: CollectorDown
    expr: up{job="database-intelligence"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Database Intelligence Collector is down"
      
  - alert: HighMemoryUsage
    expr: process_resident_memory_bytes{job="database-intelligence"} > 1073741824  # 1GB
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High memory usage detected"
      
  - alert: DroppedMetrics
    expr: rate(otelcol_processor_dropped_metric_points_total[5m]) > 0
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "Metrics are being dropped"
```

## Troubleshooting Deployment

### Common Issues

**Health Check Failing**
```bash
# Check health endpoint
kubectl port-forward deployment/database-intelligence 13133:13133
curl http://localhost:13133/health

# Check logs
kubectl logs -l app=database-intelligence --tail=100
```

**Database Connection Issues**
```bash
# Test database connectivity from pod
kubectl exec -it deployment/database-intelligence -- sh
curl -v telnet://postgres.database.svc.cluster.local:5432

# Check DNS resolution
kubectl exec -it deployment/database-intelligence -- nslookup postgres.database.svc.cluster.local
```

**Memory Issues**
```bash
# Check memory usage
kubectl top pods -l app=database-intelligence

# Check memory limits
kubectl describe pod -l app=database-intelligence | grep -A5 Limits
```

**Configuration Issues**
```bash
# Validate configuration
kubectl get configmap db-intelligence-config -o yaml

# Check environment variables
kubectl exec -it deployment/database-intelligence -- env | grep -E "(POSTGRES|MYSQL|NEW_RELIC)"
```

This deployment guide provides production-ready configurations for all major deployment platforms with security, monitoring, and troubleshooting capabilities.