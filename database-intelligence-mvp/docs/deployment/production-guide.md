# Production Deployment Guide

This guide covers best practices and recommendations for deploying the Database Intelligence Collector in production environments.

## Prerequisites

### PostgreSQL Configuration

1. **Enable Required Extensions**:
```sql
-- Required
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Optional but recommended
CREATE EXTENSION IF NOT EXISTS pg_wait_sampling;
```

2. **Configure auto_explain**:
```sql
-- Add to postgresql.conf or use ALTER SYSTEM
ALTER SYSTEM SET shared_preload_libraries = 'auto_explain,pg_stat_statements';
ALTER SYSTEM SET auto_explain.log_min_duration = 1000;  -- Start with 1 second
ALTER SYSTEM SET auto_explain.log_analyze = true;
ALTER SYSTEM SET auto_explain.log_buffers = true;
ALTER SYSTEM SET auto_explain.log_format = 'json';
ALTER SYSTEM SET auto_explain.log_nested_statements = true;
ALTER SYSTEM SET auto_explain.sample_rate = 0.1;  -- Sample 10% in production

-- Reload configuration
SELECT pg_reload_conf();
```

3. **Create Monitoring User**:
```sql
-- Create dedicated monitoring user
CREATE USER dbintel_monitor WITH PASSWORD 'strong_password_here';

-- Grant necessary permissions
GRANT pg_monitor TO dbintel_monitor;
GRANT SELECT ON pg_stat_statements TO dbintel_monitor;

-- For specific databases
GRANT CONNECT ON DATABASE production_db TO dbintel_monitor;
GRANT USAGE ON SCHEMA public TO dbintel_monitor;
```

### System Requirements

- **CPU**: 2-4 cores recommended
- **Memory**: 2-4 GB RAM
- **Disk**: 10 GB for logs and temporary data
- **Network**: Low latency connection to PostgreSQL

## Configuration

### Production Configuration Template

```yaml
# /etc/otel/collector-production.yaml

extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    
  zpages:
    endpoint: 0.0.0.0:55679

receivers:
  # Standard PostgreSQL metrics
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases: 
      - ${env:POSTGRES_DB}
    tls:
      insecure: false
      ca_file: /etc/ssl/certs/postgres-ca.crt
    collection_interval: 60s
    
  # Plan Intelligence
  autoexplain:
    log_path: ${env:POSTGRES_LOG_PATH}
    log_format: json
    
    database:
      endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
      username: ${env:POSTGRES_USER}
      password: ${env:POSTGRES_PASSWORD}
      database: ${env:POSTGRES_DB}
      ssl_mode: require
      max_connections: 5  # Limit connections
    
    plan_collection:
      enabled: true
      min_duration: 1s              # Only queries > 1 second
      max_plans_per_query: 5        # Limit history
      retention_duration: 12h       # Shorter retention
      
      regression_detection:
        enabled: true
        performance_degradation_threshold: 0.3  # 30% threshold
        min_executions: 20           # Higher minimum
    
    plan_anonymization:
      enabled: true                  # Always enable in production
      anonymize_filters: true
      anonymize_join_conditions: true
      remove_cost_estimates: false
  
  # Active Session History
  ash:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    database: ${env:POSTGRES_DB}
    
    collection_interval: 1s
    retention_duration: 30m          # Shorter retention
    
    sampling:
      enabled: true
      sample_rate: 0.2               # 20% baseline
      active_session_rate: 5.0       # Boost to 100% for active
      blocked_session_rate: 5.0      # Always sample blocked
      long_running_threshold: 30s    # Higher threshold
      adaptive_sampling: true        # Enable adaptation
    
    storage:
      buffer_size: 1800             # 30 minutes
      aggregation_windows: [1m, 5m, 15m]
      compression_enabled: true      # Enable compression
    
    analysis:
      wait_event_analysis: true
      blocking_analysis: true
      resource_analysis: false       # Disable if not needed
      anomaly_detection: true
      top_query_analysis: true
      trend_analysis: false          # Disable for performance

processors:
  # Memory protection
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 20
    
  # Resource attributes
  resource:
    attributes:
      - key: service.name
        value: postgresql
        action: upsert
      - key: deployment.environment
        value: production
        action: upsert
      - key: db.cluster
        value: ${env:DB_CLUSTER_NAME}
        action: upsert
  
  # Circuit breaker
  circuitbreaker:
    failure_threshold: 5
    timeout: 30s
    cooldown_period: 5m
    
  # Adaptive sampling
  adaptivesampler:
    in_memory_only: true
    default_sampling_rate: 0.1
    
    rules:
      - name: always_errors
        conditions:
          - attribute: severity
            operator: gte
            value: ERROR
        sample_rate: 1.0
        
      - name: plan_regressions
        conditions:
          - attribute: event_type
            value: plan_regression
        sample_rate: 1.0
        
      - name: blocked_sessions
        conditions:
          - attribute: blocked
            value: true
        sample_rate: 1.0
  
  # Batch processing
  batch:
    timeout: 10s
    send_batch_size: 1000
    send_batch_max_size: 2000

exporters:
  # Primary exporter (e.g., New Relic)
  otlp/primary:
    endpoint: ${env:OTLP_ENDPOINT}
    headers:
      api-key: ${env:OTLP_API_KEY}
    compression: gzip
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 300s
      max_elapsed_time: 900s
    sending_queue:
      enabled: true
      num_consumers: 2
      queue_size: 1000
    timeout: 30s
    
  # Local Prometheus for alerting
  prometheus:
    endpoint: 0.0.0.0:8888
    namespace: dbintel
    resource_to_telemetry_conversion:
      enabled: true
    enable_open_metrics: true

service:
  extensions: [health_check, zpages]
  
  pipelines:
    # Infrastructure metrics
    metrics/infrastructure:
      receivers: [postgresql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp/primary, prometheus]
    
    # Plan intelligence
    metrics/plans:
      receivers: [autoexplain]
      processors: [memory_limiter, resource, circuitbreaker, adaptivesampler, batch]
      exporters: [otlp/primary]
    
    # ASH metrics
    metrics/ash:
      receivers: [ash]
      processors: [memory_limiter, resource, adaptivesampler, batch]
      exporters: [otlp/primary, prometheus]
      
  telemetry:
    logs:
      level: warn  # Production log level
      encoding: json
      output_paths: ["/var/log/otel/collector.log"]
      error_output_paths: ["/var/log/otel/collector-error.log"]
    
    metrics:
      level: normal
      address: 0.0.0.0:8888
```

## Deployment Options

### Docker Deployment

```dockerfile
# Dockerfile
FROM otel/opentelemetry-collector-contrib:latest

# Add custom configuration
COPY collector-production.yaml /etc/otel/config.yaml

# Add certificates
COPY certs/postgres-ca.crt /etc/ssl/certs/

# Create log directory
RUN mkdir -p /var/log/otel

# Run as non-root
USER 10001

EXPOSE 8888 13133 55679

CMD ["--config", "/etc/otel/config.yaml"]
```

```yaml
# docker-compose.yml
version: '3.8'

services:
  db-intelligence-collector:
    build: .
    container_name: dbintel-collector
    restart: unless-stopped
    
    environment:
      - POSTGRES_HOST=postgresql.internal
      - POSTGRES_PORT=5432
      - POSTGRES_USER=dbintel_monitor
      - POSTGRES_PASSWORD_FILE=/run/secrets/postgres_password
      - POSTGRES_DB=production
      - POSTGRES_LOG_PATH=/var/log/postgresql/postgresql.log
      - DB_CLUSTER_NAME=prod-primary
      - OTLP_ENDPOINT=otlp.monitoring.internal:4317
      - OTLP_API_KEY_FILE=/run/secrets/otlp_api_key
    
    secrets:
      - postgres_password
      - otlp_api_key
    
    volumes:
      - /var/log/postgresql:/var/log/postgresql:ro
      - collector-logs:/var/log/otel
    
    ports:
      - "8888:8888"    # Prometheus metrics
      - "13133:13133"  # Health check
    
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
    
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:13133/health"]
      interval: 30s
      timeout: 10s
      retries: 3

secrets:
  postgres_password:
    external: true
  otlp_api_key:
    external: true

volumes:
  collector-logs:
```

### Kubernetes Deployment

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dbintel-collector-config
  namespace: monitoring
data:
  collector.yaml: |
    # Full configuration here
```

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dbintel-collector
  namespace: monitoring
  labels:
    app: dbintel-collector
spec:
  replicas: 2  # HA deployment
  selector:
    matchLabels:
      app: dbintel-collector
  template:
    metadata:
      labels:
        app: dbintel-collector
    spec:
      serviceAccountName: dbintel-collector
      
      containers:
      - name: collector
        image: otel/opentelemetry-collector-contrib:latest
        
        args:
          - --config=/etc/otel/collector.yaml
        
        env:
        - name: POSTGRES_HOST
          value: postgresql.database.svc.cluster.local
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: dbintel-secrets
              key: postgres-user
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: dbintel-secrets
              key: postgres-password
        
        resources:
          limits:
            cpu: 2
            memory: 2Gi
          requests:
            cpu: 500m
            memory: 1Gi
        
        ports:
        - containerPort: 8888
          name: prometheus
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
          initialDelaySeconds: 10
          periodSeconds: 10
        
        volumeMounts:
        - name: config
          mountPath: /etc/otel
        - name: postgres-logs
          mountPath: /var/log/postgresql
          readOnly: true
        - name: varlog
          mountPath: /var/log/otel
      
      volumes:
      - name: config
        configMap:
          name: dbintel-collector-config
      - name: postgres-logs
        hostPath:
          path: /var/log/postgresql
          type: Directory
      - name: varlog
        emptyDir: {}
```

## Security Hardening

### 1. Network Security

```yaml
# Network Policy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: dbintel-collector-netpol
spec:
  podSelector:
    matchLabels:
      app: dbintel-collector
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - port: 8888    # Prometheus
    - port: 13133   # Health
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: database
    ports:
    - port: 5432    # PostgreSQL
  - to:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - port: 4317    # OTLP
```

### 2. Secret Management

```bash
# Create secrets securely
kubectl create secret generic dbintel-secrets \
  --from-literal=postgres-user=dbintel_monitor \
  --from-file=postgres-password=./postgres-password.txt \
  --from-file=otlp-api-key=./otlp-api-key.txt \
  -n monitoring

# Encrypt secrets at rest
kubectl patch secret dbintel-secrets -n monitoring \
  -p '{"metadata":{"annotations":{"encryption":"enabled"}}}'
```

### 3. RBAC Configuration

```yaml
# rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: dbintel-collector
  namespace: monitoring
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: dbintel-collector
  namespace: monitoring
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: dbintel-collector
  namespace: monitoring
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: dbintel-collector
subjects:
- kind: ServiceAccount
  name: dbintel-collector
  namespace: monitoring
```

## Monitoring & Alerting

### Prometheus Alerts

```yaml
# alerts.yaml
groups:
- name: dbintel_collector
  interval: 30s
  rules:
  - alert: CollectorDown
    expr: up{job="dbintel-collector"} == 0
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "DB Intelligence Collector is down"
      description: "{{ $labels.instance }} has been down for more than 5 minutes."
  
  - alert: HighMemoryUsage
    expr: |
      otelcol_process_memory_rss{job="dbintel-collector"} 
      / otelcol_process_memory_limit{job="dbintel-collector"} > 0.8
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "Collector memory usage is high"
      description: "Memory usage is above 80% for {{ $labels.instance }}"
  
  - alert: PlanRegressionDetected
    expr: |
      rate(db_postgresql_plan_regression_detected_total[5m]) > 0
    labels:
      severity: warning
    annotations:
      summary: "Query plan regression detected"
      description: "Regression detected for query {{ $labels.query_id }}"
  
  - alert: ExcessiveLockWaits
    expr: |
      postgresql_ash_wait_events_count{category="Concurrency"} > 50
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High lock contention detected"
      description: "More than 50 sessions waiting on locks"
```

### Grafana Dashboards

Key dashboards to implement:

1. **Collector Health Dashboard**
   - Collector uptime and restarts
   - Memory and CPU usage
   - Pipeline metrics (received, processed, exported)
   - Error rates

2. **Plan Intelligence Dashboard**
   - Plan changes over time
   - Regression detection alerts
   - Top queries by plan changes
   - Cost analysis trends

3. **ASH Overview Dashboard**
   - Session state distribution
   - Wait event heatmap
   - Blocking chain visualization
   - Top SQL by active sessions

4. **Performance Analysis Dashboard**
   - Query performance trends
   - Wait event categories
   - Resource utilization
   - Anomaly detection results

## Performance Tuning

### 1. Sampling Optimization

```yaml
# Start conservative and increase gradually
sampling:
  sample_rate: 0.1  # Start with 10%
  
# Monitor impact and adjust:
# - If overhead < 1% CPU: increase to 0.2
# - If overhead < 2% CPU: increase to 0.5
# - If overhead > 3% CPU: decrease rate
```

### 2. Memory Management

```yaml
# Adjust based on available memory
storage:
  buffer_size: 900   # 15 minutes for constrained environments
  
memory_limiter:
  limit_percentage: 60  # Lower for shared environments
```

### 3. Query Optimization

```sql
-- Create indexes for monitoring queries
CREATE INDEX CONCURRENTLY idx_pg_stat_activity_state 
  ON pg_catalog.pg_stat_activity (state) 
  WHERE state != 'idle';

-- Analyze tables regularly
ANALYZE pg_stat_activity;
ANALYZE pg_locks;
```

## Maintenance

### Log Rotation

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
    postrotate
        pkill -USR1 otelcol
    endscript
}
```

### Backup Configuration

```bash
#!/bin/bash
# backup-collector-config.sh

BACKUP_DIR="/backup/otel-collector"
DATE=$(date +%Y%m%d_%H%M%S)

# Backup configuration
cp /etc/otel/collector.yaml $BACKUP_DIR/collector-$DATE.yaml

# Backup secrets (encrypted)
kubectl get secret dbintel-secrets -o yaml | \
  gpg --encrypt -r backup@company.com > $BACKUP_DIR/secrets-$DATE.yaml.gpg

# Keep only last 30 days
find $BACKUP_DIR -name "*.yaml*" -mtime +30 -delete
```

### Health Monitoring Script

```bash
#!/bin/bash
# check-collector-health.sh

HEALTH_ENDPOINT="http://localhost:13133/health"
METRICS_ENDPOINT="http://localhost:8888/metrics"

# Check health endpoint
if ! curl -sf $HEALTH_ENDPOINT > /dev/null; then
    echo "ERROR: Collector health check failed"
    exit 1
fi

# Check metrics endpoint
if ! curl -sf $METRICS_ENDPOINT | grep -q "otelcol_receiver_accepted_metric_points"; then
    echo "ERROR: Collector not processing metrics"
    exit 1
fi

# Check memory usage
MEMORY_USAGE=$(curl -sf $METRICS_ENDPOINT | grep "otelcol_process_memory_rss" | awk '{print $2}')
MEMORY_LIMIT=$(curl -sf $METRICS_ENDPOINT | grep "otelcol_process_memory_limit" | awk '{print $2}')

if (( $(echo "$MEMORY_USAGE > $MEMORY_LIMIT * 0.9" | bc -l) )); then
    echo "WARNING: Memory usage above 90%"
fi

echo "OK: Collector is healthy"
```

## Troubleshooting Production Issues

### Common Issues and Solutions

1. **High CPU Usage**
   - Reduce ASH sampling rate
   - Increase collection intervals
   - Enable adaptive sampling
   - Check for expensive queries in monitoring

2. **Memory Leaks**
   - Reduce buffer sizes
   - Enable memory limiter
   - Check for plan store growth
   - Restart collector periodically

3. **Missing Metrics**
   - Verify PostgreSQL connectivity
   - Check log file permissions
   - Review receiver logs
   - Validate configuration

4. **Export Failures**
   - Check network connectivity
   - Verify API keys
   - Review retry configuration
   - Monitor queue sizes

### Emergency Procedures

```bash
# Quick disable of expensive features
kubectl set env deployment/dbintel-collector \
  ASH_ENABLED=false \
  PLAN_REGRESSION_ENABLED=false \
  -n monitoring

# Scale down in emergency
kubectl scale deployment/dbintel-collector --replicas=0 -n monitoring

# Restart with minimal config
kubectl create configmap emergency-config \
  --from-file=collector-minimal.yaml \
  -n monitoring
```

## Capacity Planning

### Sizing Guidelines

| Database Size | Sessions | Recommended Config |
|--------------|----------|-------------------|
| < 100 GB | < 100 | 1 CPU, 1 GB RAM |
| 100-500 GB | 100-500 | 2 CPU, 2 GB RAM |
| 500 GB - 1 TB | 500-1000 | 4 CPU, 4 GB RAM |
| > 1 TB | > 1000 | 8 CPU, 8 GB RAM |

### Scaling Strategies

1. **Horizontal Scaling**: Deploy multiple collectors for different database clusters
2. **Vertical Scaling**: Increase resources for high-load databases
3. **Federated Collection**: Use remote write for centralized storage
4. **Sampling Adjustment**: Reduce sampling rates for very large deployments