# Deployment Guide

## Critical Deployment Constraint

⚠️ **SINGLE INSTANCE ONLY**: Due to file-based state storage, this collector MUST run as a single instance. Multiple instances will cause data inconsistencies.

## Deployment Patterns

### Option 1: StatefulSet (Recommended)

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: nr-db-intelligence-collector
spec:
  serviceName: nr-db-intelligence
  replicas: 1  # MUST BE 1
  selector:
    matchLabels:
      app: nr-db-intelligence
  template:
    metadata:
      labels:
        app: nr-db-intelligence
    spec:
      containers:
      - name: collector
        image: otel/opentelemetry-collector-contrib:latest
        args: ["--config=/etc/otel/config.yaml"]
        
        # Resource limits
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
        
        # Volume mounts
        volumeMounts:
        - name: config
          mountPath: /etc/otel
        - name: state
          mountPath: /var/lib/otel/storage
        - name: logs
          mountPath: /var/log
          
      volumes:
      - name: config
        configMap:
          name: collector-config
      - name: logs
        hostPath:
          path: /var/log
          
  # Persistent storage for state
  volumeClaimTemplates:
  - metadata:
      name: state
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

**Why StatefulSet?**
- Stable storage (state persistence)
- Ordered deployment/scaling
- Stable network identity
- Persistent volume claims

### Option 2: DaemonSet (For Node-Local Collection)

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: nr-db-intelligence-collector
spec:
  selector:
    matchLabels:
      app: nr-db-intelligence
  template:
    metadata:
      labels:
        app: nr-db-intelligence
    spec:
      # Node selection (only database nodes)
      nodeSelector:
        node-role: database
        
      containers:
      - name: collector
        image: otel/opentelemetry-collector-contrib:latest
        
        # Mount local logs
        volumeMounts:
        - name: pg-logs
          mountPath: /var/log/postgresql
          readOnly: true
        - name: state
          mountPath: /var/lib/otel/storage
          
      volumes:
      - name: pg-logs
        hostPath:
          path: /var/log/postgresql
      - name: state
        hostPath:
          path: /var/lib/otel/storage
          type: DirectoryOrCreate
```

**Why DaemonSet?**
- One collector per database node
- Direct log file access
- Natural sharding by node
- No network hops for logs

### Option 3: Single Pod (Development/Testing)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nr-db-intelligence-collector
spec:
  containers:
  - name: collector
    image: otel/opentelemetry-collector-contrib:latest
    args: ["--config=/etc/otel/config.yaml"]
    
    # Environment variables
    env:
    - name: NEW_RELIC_LICENSE_KEY
      valueFrom:
        secretKeyRef:
          name: newrelic
          key: license-key
    - name: PG_REPLICA_DSN
      valueFrom:
        secretKeyRef:
          name: database-credentials
          key: pg-replica-dsn
```

## Configuration Management

### ConfigMap Structure

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: collector-config
data:
  config.yaml: |
    # Full collector configuration here
    receivers:
      sqlquery/postgresql_plans_safe:
        # ... configuration ...
```

### Secret Management

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: database-credentials
type: Opaque
stringData:
  pg-replica-dsn: "postgres://user:pass@replica:5432/db?sslmode=require"
  mysql-readonly-dsn: "user:pass@tcp(replica:3306)/db?tls=true"
  new-relic-license-key: "eu01xx..."
```

## Network Configuration

### Ingress Requirements

The collector needs OUTBOUND access to:
- Database replicas (port 5432/3306/27017)
- New Relic OTLP endpoint (port 4317/4318)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: collector-network-policy
spec:
  podSelector:
    matchLabels:
      app: nr-db-intelligence
  policyTypes:
  - Egress
  egress:
  # Database access
  - to:
    - namespaceSelector:
        matchLabels:
          name: databases
    ports:
    - protocol: TCP
      port: 5432  # PostgreSQL
    - protocol: TCP
      port: 3306  # MySQL
      
  # New Relic access
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443   # HTTPS
    - protocol: TCP
      port: 4317  # OTLP gRPC
```

### Service Definition (for health checks)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: collector-health
spec:
  selector:
    app: nr-db-intelligence
  ports:
  - name: health
    port: 13133
    targetPort: 13133
  - name: metrics
    port: 8888
    targetPort: 8888
```

## Health Monitoring

### Liveness Probe

```yaml
livenessProbe:
  httpGet:
    path: /
    port: 13133
  initialDelaySeconds: 30
  periodSeconds: 30
  timeoutSeconds: 5
  failureThreshold: 3
```

### Readiness Probe

```yaml
readinessProbe:
  httpGet:
    path: /
    port: 13133
  initialDelaySeconds: 10
  periodSeconds: 10
  timeoutSeconds: 5
  successThreshold: 1
  failureThreshold: 3
```

## Storage Considerations

### Persistent Volume Requirements
- **Size**: 10Gi minimum (for state storage)
- **Access Mode**: ReadWriteOnce
- **Storage Class**: Fast SSD preferred
- **Backup**: Not required (state is rebuildable)

### Log Volume Requirements
- **Size**: Depends on database log volume
- **Rotation**: Ensure logs rotate to prevent fill
- **Permissions**: Read-only mount

## Security Hardening

### Pod Security Context

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 10001
  fsGroup: 10001
  capabilities:
    drop:
    - ALL
  readOnlyRootFilesystem: true
```

### Required Volume Mounts

```yaml
volumeMounts:
# For read-only root filesystem
- name: tmp
  mountPath: /tmp
- name: cache
  mountPath: /var/cache
```

## Rollout Strategy

### Phase 1: Single Database Test
1. Deploy to development environment
2. Configure for single PostgreSQL replica
3. Verify data flow to New Relic
4. Monitor for 24 hours

### Phase 2: Production Pilot
1. Deploy to production (single database)
2. Start with 5-minute collection interval
3. Monitor database impact
4. Gradually decrease to 1-minute interval

### Phase 3: Full Rollout
1. Add additional databases one at a time
2. Monitor collector resource usage
3. Adjust memory limits as needed
4. Document any issues

## Rollback Plan

If issues occur:
1. **Immediate**: Scale StatefulSet to 0 replicas
2. **Investigation**: Check collector logs
3. **Fix Forward**: Update configuration
4. **Clean State**: Delete PVC if state corrupted
5. **Restart**: Scale back to 1 replica

## Monitoring Checklist

- [ ] Collector pod is running
- [ ] Health endpoint responds (port 13133)
- [ ] Metrics endpoint responds (port 8888)
- [ ] Logs show successful collection
- [ ] Data appears in New Relic
- [ ] No database performance impact
- [ ] State storage is persisted