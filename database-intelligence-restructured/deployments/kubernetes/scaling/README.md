# Kubernetes Deployment with Horizontal Scaling

This directory contains Kubernetes manifests for deploying the Database Intelligence collector with horizontal scaling capabilities.

## Overview

The deployment demonstrates:
- Horizontal scaling with multiple collector replicas
- Redis-based distributed coordination
- Automatic pod scaling based on metrics
- Resource assignment across collector nodes
- Health monitoring and metrics collection

## Components

### 1. Namespace
- `namespace.yaml`: Creates the `database-intelligence` namespace

### 2. Redis Deployment
- `redis.yaml`: Deploys Redis for distributed state storage
- Used for leader election and resource assignment coordination

### 3. Configuration
- `configmap.yaml`: Contains collector configuration with scaling enabled
- Configures receivers for PostgreSQL, MySQL, MongoDB, and Redis
- Enables horizontal scaling with Redis coordination

### 4. Collector Deployment
- `deployment.yaml`: Deploys the collector with 3 initial replicas
- Includes service account and RBAC permissions
- Configures health checks and resource limits

### 5. Horizontal Pod Autoscaler
- `hpa.yaml`: Configures automatic scaling from 3 to 10 replicas
- Scales based on CPU, memory, and connection pool utilization

### 6. Monitoring
- `podmonitor.yaml`: Configures Prometheus monitoring for collector pods

## Deployment Steps

1. **Create namespace:**
   ```bash
   kubectl apply -f namespace.yaml
   ```

2. **Deploy Redis:**
   ```bash
   kubectl apply -f redis.yaml
   ```

3. **Create configuration:**
   ```bash
   kubectl apply -f configmap.yaml
   ```

4. **Deploy collectors:**
   ```bash
   kubectl apply -f deployment.yaml
   ```

5. **Configure autoscaling:**
   ```bash
   kubectl apply -f hpa.yaml
   ```

6. **Setup monitoring (if using Prometheus Operator):**
   ```bash
   kubectl apply -f podmonitor.yaml
   ```

## Configuration

### Scaling Configuration

The scaling configuration in `configmap.yaml`:

```yaml
scaling:
  enabled: true
  mode: redis  # Use Redis for distributed coordination
  redis:
    address: redis.database-intelligence.svc.cluster.local:6379
    db: 0
    key_prefix: "dbintel:scaling:"
    leader_ttl: 30s
  coordinator:
    heartbeat_interval: 30s
    node_timeout: 90s
    rebalance_interval: 5m
    min_rebalance_interval: 1m
  receiver_scaling:
    check_interval: 30s
    resource_prefix: "db:"
    ignore_assignments: false
```

### Key Features

1. **Leader Election**: One collector node acts as the leader for coordination
2. **Resource Assignment**: Databases are distributed across collector nodes
3. **Automatic Rebalancing**: Resources are rebalanced when nodes join/leave
4. **Health Monitoring**: Nodes send heartbeats; stale nodes are removed

## Monitoring

### Metrics

The collectors expose metrics at `:8889/metrics`:
- `dbintel_scaling_nodes_total`: Total active collector nodes
- `dbintel_scaling_assignments_total`: Total resource assignments
- `dbintel_scaling_leader`: Current leader node (1 for leader, 0 for follower)
- `dbintel_scaling_rebalance_total`: Total rebalance operations

### Health Checks

Health endpoint at `:13133/`:
- Liveness probe: Checks if the collector is running
- Readiness probe: Checks if the collector is ready to receive traffic

## Scaling Behavior

### Scale Up
- Triggers when CPU > 70% or Memory > 80%
- Also scales based on connection pool utilization
- Can scale up by 50% or 2 pods per minute

### Scale Down
- More conservative to prevent flapping
- Scales down by 10% or 1 pod per minute
- 5-minute stabilization window

## Testing Horizontal Scaling

1. **Check initial deployment:**
   ```bash
   kubectl get pods -n database-intelligence
   kubectl get hpa -n database-intelligence
   ```

2. **Monitor resource assignments:**
   ```bash
   kubectl logs -n database-intelligence -l app=dbintel-collector --tail=100 | grep -i "assignment"
   ```

3. **Simulate load to trigger scaling:**
   ```bash
   # Port-forward to a collector pod
   kubectl port-forward -n database-intelligence deployment/dbintel-collector 8889:8889
   
   # Generate load (example)
   while true; do curl http://localhost:8889/metrics > /dev/null; done
   ```

4. **Watch scaling events:**
   ```bash
   kubectl get events -n database-intelligence --watch
   ```

## Troubleshooting

### Common Issues

1. **Pods not starting:**
   - Check logs: `kubectl logs -n database-intelligence <pod-name>`
   - Verify Redis is running: `kubectl get pods -n database-intelligence`

2. **Scaling not working:**
   - Check HPA status: `kubectl describe hpa -n database-intelligence dbintel-collector`
   - Verify metrics server is installed

3. **Resource assignment issues:**
   - Check coordinator logs for rebalancing
   - Verify Redis connectivity

### Debug Commands

```bash
# Get all resources
kubectl get all -n database-intelligence

# Describe deployment
kubectl describe deployment -n database-intelligence dbintel-collector

# Check HPA status
kubectl get hpa -n database-intelligence dbintel-collector --watch

# View collector logs
kubectl logs -n database-intelligence -l app=dbintel-collector -f

# Check Redis
kubectl exec -it -n database-intelligence deployment/redis -- redis-cli
```

## Production Considerations

1. **Redis Persistence**: Consider using persistent volumes for Redis
2. **Resource Limits**: Adjust based on your workload
3. **Network Policies**: Implement proper network segmentation
4. **Security**: Use proper secrets management for credentials
5. **Monitoring**: Integrate with your monitoring stack

## Customization

### Adjusting Replica Count

Edit `deployment.yaml`:
```yaml
spec:
  replicas: 5  # Change initial replica count
```

Edit `hpa.yaml`:
```yaml
spec:
  minReplicas: 5   # Minimum replicas
  maxReplicas: 20  # Maximum replicas
```

### Changing Scaling Metrics

Add custom metrics to `hpa.yaml`:
```yaml
metrics:
- type: External
  external:
    metric:
      name: database_query_rate
      selector:
        matchLabels:
          deployment: dbintel-collector
    target:
      type: AverageValue
      averageValue: "1000"
```