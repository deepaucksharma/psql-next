# High Availability Gateway Configuration Guide

## Overview

The HA Gateway configuration provides resilience, scalability, and redundancy for the MySQL wait-based monitoring pipeline. This guide covers deployment, operations, and troubleshooting.

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│Edge Collector│     │Edge Collector│     │Edge Collector│
│      1       │     │      2       │     │      N       │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────────────┴───────────────────┘
                           │
                    ┌──────┴──────┐
                    │  HAProxy LB  │
                    └──────┬──────┘
       ┌───────────────────┼───────────────────┐
       │                   │                   │
┌──────┴──────┐     ┌──────┴──────┐     ┌──────┴──────┐
│  Gateway 1   │     │  Gateway 2   │     │  Gateway 3   │
│  (Primary)   │     │  (Primary)   │     │(Cross-Region)│
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────────────┴───────────────────┘
                           │
                    ┌──────┴──────┐
                    │  New Relic   │
                    └─────────────┘
```

## Key Features

### 1. Load Balancing
- HAProxy distributes traffic across gateway instances
- Client IP affinity for consistent routing
- Health check-based routing

### 2. Redundancy
- Multiple gateway instances in each region
- Cross-region replication for disaster recovery
- Automatic failover on instance failure

### 3. Scalability
- Horizontal scaling with HPA (Kubernetes)
- Consistent hashing for metric routing
- Queue-based buffering for traffic spikes

### 4. Data Integrity
- Deduplication across gateway instances
- Persistent queues for data durability
- Circuit breakers for backend protection

## Deployment

### Docker Compose (Development/Testing)

```bash
# Start HA setup
docker-compose -f docker-compose-ha.yml up -d

# Check gateway health
curl http://localhost:13135/health  # Gateway 1
curl http://localhost:13136/health  # Gateway 2
curl http://localhost:13137/health  # Gateway 3

# View HAProxy stats
open http://localhost:8404/stats
```

### Kubernetes (Production)

```bash
# Create namespace and deploy
kubectl apply -f deployments/kubernetes/gateway-ha-deployment.yaml

# Check deployment status
kubectl -n mysql-monitoring get pods -l app=otel-gateway-ha

# View service endpoints
kubectl -n mysql-monitoring get svc otel-gateway-ha

# Check HPA status
kubectl -n mysql-monitoring get hpa otel-gateway-ha-hpa
```

## Configuration

### Environment Variables

```bash
# Required
export NEW_RELIC_LICENSE_KEY="your-license-key"
export PRIMARY_BACKEND_ENDPOINT="otlp-backend-1.monitoring.internal:4317"
export SECONDARY_BACKEND_ENDPOINT="otlp-backend-2.monitoring.internal:4317"

# Optional
export GATEWAY_REGION="us-east-1"
export GATEWAY_AZ="us-east-1a"
export GATEWAY_CLUSTER="primary"
export CROSS_REGION_ENDPOINT="otlp-gateway-us-west-2.monitoring.internal:4317"
```

### Key Configuration Parameters

#### Memory Management
```yaml
processors:
  memory_limiter:
    limit_mib: 1024        # Higher limit for gateway
    spike_limit_mib: 256   # Buffer for traffic spikes
```

#### Load Balancing
```yaml
processors:
  loadbalancing:
    resolver:
      static:
        hostnames:
          - backend1:4317
          - backend2:4317
          - backend3:4317
```

#### Queue Configuration
```yaml
exporters:
  otlp/primary_backend:
    sending_queue:
      enabled: true
      num_consumers: 10    # Parallel consumers
      queue_size: 50000    # Large queue for buffering
      storage: file_storage/queue  # Persistent storage
```

## Operations

### Monitoring Gateway Health

```bash
# Check gateway metrics
curl -s http://localhost:8890/metrics | grep -E 'otelcol_receiver_accepted|otelcol_exporter_sent'

# Monitor queue size
curl -s http://localhost:8890/metrics | grep 'otelcol_exporter_queue_size'

# Check error rates
curl -s http://localhost:8890/metrics | grep 'otelcol_exporter_send_failed'
```

### Scaling Operations

#### Manual Scaling (Kubernetes)
```bash
# Scale up
kubectl -n mysql-monitoring scale statefulset otel-gateway-ha --replicas=5

# Scale down (ensure minimum 3 for HA)
kubectl -n mysql-monitoring scale statefulset otel-gateway-ha --replicas=3
```

#### Auto-scaling Tuning
```yaml
# Edit HPA configuration
kubectl -n mysql-monitoring edit hpa otel-gateway-ha-hpa

# Adjust thresholds
spec:
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        averageUtilization: 70  # Adjust as needed
```

### Maintenance Procedures

#### Rolling Updates
```bash
# Update gateway image
kubectl -n mysql-monitoring set image statefulset/otel-gateway-ha \
  otel-collector=otel/opentelemetry-collector-contrib:0.92.0

# Monitor rollout
kubectl -n mysql-monitoring rollout status statefulset/otel-gateway-ha
```

#### Draining Gateway Instance
```bash
# Remove from load balancer
kubectl -n mysql-monitoring label pod otel-gateway-ha-0 drain=true

# Wait for connections to drain
sleep 60

# Delete pod for maintenance
kubectl -n mysql-monitoring delete pod otel-gateway-ha-0
```

## Troubleshooting

### Common Issues

#### 1. Gateway Not Receiving Data
```bash
# Check load balancer
curl -v telnet://localhost:4319

# Verify edge collector connectivity
docker exec edge-collector-1 curl -s http://gateway-lb:13133/health

# Check firewall rules
sudo iptables -L -n | grep 4317
```

#### 2. High Memory Usage
```bash
# Check current usage
curl -s http://localhost:8890/metrics | grep process_resident_memory_bytes

# Review queue sizes
curl -s http://localhost:8890/metrics | grep queue_size

# Adjust memory limits if needed
kubectl -n mysql-monitoring edit statefulset otel-gateway-ha
```

#### 3. Uneven Load Distribution
```bash
# Check HAProxy stats
curl -s http://localhost:8404/stats

# Verify backend health
for i in 1 2 3; do
  echo "Gateway $i:"
  curl -s http://gateway-ha-$i:8888/metrics | grep receiver_accepted_metric_points
done
```

### Performance Tuning

#### For High Throughput
```yaml
processors:
  batch:
    timeout: 5s
    send_batch_size: 10000      # Larger batches
    send_batch_max_size: 15000

exporters:
  otlp/primary_backend:
    sending_queue:
      num_consumers: 20         # More parallel consumers
      queue_size: 100000        # Larger queue
```

#### For Low Latency
```yaml
processors:
  batch:
    timeout: 1s                 # Shorter timeout
    send_batch_size: 1000       # Smaller batches

processors:
  memory_limiter:
    check_interval: 500ms       # More frequent checks
```

## Best Practices

### 1. Deployment
- Always deploy at least 3 gateway instances
- Distribute instances across availability zones
- Use anti-affinity rules to prevent co-location

### 2. Configuration
- Set appropriate resource limits based on traffic
- Configure persistent storage for queues
- Enable circuit breakers for backend protection

### 3. Monitoring
- Set up alerts for queue overflow
- Monitor gateway-to-backend latency
- Track deduplication effectiveness

### 4. Security
- Use TLS for all connections
- Implement authentication between components
- Regularly rotate credentials

## Integration Examples

### Sending to Multiple Backends
```yaml
service:
  pipelines:
    metrics/multi:
      receivers: [otlp/primary]
      processors: [memory_limiter, batch]
      exporters: 
        - otlp/primary_backend
        - otlp/secondary_backend
        - otlp/cross_region
```

### Priority-Based Routing
```yaml
processors:
  routing/priority:
    from_attribute: priority
    table:
      - value: P0
        exporters: [otlp/priority_backend]
      - value: P1
        exporters: [otlp/primary_backend]
      - value: P2
        exporters: [otlp/secondary_backend]
```

## Disaster Recovery

### Failover Procedures
1. **Automatic Failover**: HAProxy handles instance failures automatically
2. **Region Failover**: Update DNS/load balancer to point to DR region
3. **Data Recovery**: Persistent queues preserve data during outages

### Backup Strategy
```bash
# Backup queue data
kubectl -n mysql-monitoring exec otel-gateway-ha-0 -- \
  tar -czf /tmp/queue-backup.tar.gz /var/lib/otel/gateway/queue

# Copy backup
kubectl -n mysql-monitoring cp otel-gateway-ha-0:/tmp/queue-backup.tar.gz ./queue-backup.tar.gz
```

## Capacity Planning

### Sizing Guidelines

| Metric Rate | Gateway Instances | Memory per Instance | CPU per Instance |
|-------------|------------------|-------------------|------------------|
| < 100k/sec  | 3                | 2GB               | 1 core          |
| 100k-500k/sec | 5              | 4GB               | 2 cores         |
| 500k-1M/sec | 8                | 8GB               | 4 cores         |
| > 1M/sec    | 12+              | 16GB              | 8 cores         |

### Queue Sizing
```
Queue Size = (Metric Rate × Batch Timeout × Safety Factor) / Number of Instances
Example: (100k/sec × 5sec × 2) / 3 = ~333k per instance
```

## Monitoring Dashboards

Create Grafana dashboards to monitor:
- Gateway throughput and latency
- Queue utilization and overflow
- Load distribution across instances
- Error rates and circuit breaker status
- Cross-region replication lag

## References

- [OpenTelemetry Collector Scaling](https://opentelemetry.io/docs/collector/scaling/)
- [HAProxy Configuration](http://www.haproxy.org/documentation.html)
- [Kubernetes StatefulSets](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)