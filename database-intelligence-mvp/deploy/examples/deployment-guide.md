# Database Intelligence MVP - Deployment Guide

This guide explains how to deploy the Database Intelligence MVP using the OTEL-first architecture across different environments. Each deployment example showcases different levels of complexity and features.

## Overview

The Database Intelligence MVP follows an **OpenTelemetry-first approach**, using standard OTEL components wherever possible and custom processors only for specific gaps that standard components cannot fill.

### Key Features

- **Standard OTEL Components**: Uses PostgreSQL, MySQL, and SQL Query receivers
- **Custom Processors**: Adaptive sampling, circuit breaker, and verification for production needs
- **Multiple Databases**: Supports PostgreSQL and MySQL monitoring
- **Cloud Native**: Kubernetes-ready with HA and auto-scaling
- **Production Ready**: Includes monitoring, alerting, and security features

## Architecture Principles

### OTEL-First Approach

1. **Standard Receivers**: Use built-in OTEL receivers for infrastructure metrics
2. **SQL Query Receiver**: For complex database-specific queries
3. **Custom Processors**: Only for unique business logic gaps
4. **Standard Exporters**: OTLP to New Relic, Prometheus for local metrics

### Deployment Options

| Option | Use Case | Complexity | Features |
|--------|----------|------------|----------|
| Docker Simple | Development, Testing | Low | Single database, basic monitoring |
| Docker Production | Production, Multi-DB | High | HA, custom processors, full stack |
| Kubernetes Minimal | Basic K8s deployment | Medium | Single replica, essential features |
| Kubernetes Production | Enterprise K8s | High | HA, auto-scaling, security |

## Deployment Examples

### 1. Docker Compose - Simple Setup

**File**: `docker-compose-simple.yaml`

Perfect for development and testing with a single PostgreSQL database.

#### Features
- Single OpenTelemetry Collector instance
- PostgreSQL with pg_stat_statements enabled
- Basic query monitoring
- Health checks
- PII sanitization
- Load generator for testing

#### Prerequisites
```bash
# Set environment variables
export NEW_RELIC_LICENSE_KEY="your-license-key"
export OTLP_ENDPOINT="https://otlp.nr-data.net:4318"
```

#### Deployment
```bash
# Navigate to examples directory
cd deploy/examples

# Start with PostgreSQL database
docker-compose -f docker-compose-simple.yaml up -d

# Start with load testing
docker-compose -f docker-compose-simple.yaml --profile load-test up -d

# Check health
curl http://localhost:13133/

# View metrics
curl http://localhost:8889/metrics

# View debug info
open http://localhost:55679/debug/tracez
```

#### Configuration Files
- `configs/collector-simple.yaml`: Basic OTEL configuration
- `init-scripts/postgres-simple-init.sql`: Database setup
- `init-scripts/postgres-monitoring-setup.sql`: Monitoring user setup

### 2. Docker Compose - Production Setup

**File**: `docker-compose-production.yaml`

Full production setup with PostgreSQL, MySQL, HA, and monitoring stack.

#### Features
- Primary/secondary collector instances (HA)
- PostgreSQL and MySQL databases
- Custom processors (adaptive sampling, circuit breaker)
- Prometheus and Grafana for monitoring
- Nginx load balancer
- Health monitoring and failover
- Log aggregation with Fluent Bit
- Comprehensive security and resource limits

#### Prerequisites
```bash
# Set all environment variables
export NEW_RELIC_LICENSE_KEY="your-license-key"
export OTLP_ENDPOINT="https://otlp.nr-data.net:4318"
```

#### Deployment
```bash
# Start full production stack
docker-compose -f docker-compose-production.yaml up -d

# Start with HA secondary collector
docker-compose -f docker-compose-production.yaml --profile ha up -d

# Start with log aggregation
docker-compose -f docker-compose-production.yaml --profile logging up -d

# Access services
echo "Grafana: http://localhost:3000 (admin/admin123)"
echo "Prometheus: http://localhost:9090"
echo "Collector Primary: http://localhost:13133"
echo "Collector Secondary: http://localhost:13134"
```

#### Configuration Files
- `configs/collector-production.yaml`: Full production configuration
- `configs/prometheus.yml`: Prometheus scraping configuration
- `configs/grafana-*.yml`: Grafana provisioning
- `configs/nginx.conf`: Load balancer configuration
- `scripts/health-monitor.sh`: Failover monitoring

### 3. Kubernetes - Minimal Setup

**File**: `kubernetes-minimal.yaml`

Basic Kubernetes deployment with essential features.

#### Features
- Single collector deployment
- PostgreSQL test database
- Basic RBAC and security
- Health probes
- Resource limits
- Service discovery

#### Prerequisites
```bash
# Ensure kubectl is configured
kubectl cluster-info

# Create namespace and secrets
kubectl create namespace db-intelligence
kubectl create secret generic db-intelligence-secrets \
  --from-literal=new-relic-license-key="$NEW_RELIC_LICENSE_KEY" \
  --namespace=db-intelligence
```

#### Deployment
```bash
# Deploy minimal setup
kubectl apply -f kubernetes-minimal.yaml

# Check deployment status
kubectl get pods -n db-intelligence
kubectl get services -n db-intelligence

# Access collector
kubectl port-forward -n db-intelligence svc/otel-collector-service 13133:13133

# Check health
curl http://localhost:13133/

# View logs
kubectl logs -n db-intelligence deployment/otel-collector -f
```

#### Optional Test Database
```bash
# Deploy test PostgreSQL database
kubectl apply -f kubernetes-minimal.yaml --profile test-database
```

### 4. Kubernetes - Production Setup

**File**: `kubernetes-production.yaml`

Enterprise-grade Kubernetes deployment with full features.

#### Features
- StatefulSet with 3 replicas for HA
- Horizontal Pod Autoscaler (HPA)
- Pod Disruption Budget (PDB)
- Anti-affinity for high availability
- NetworkPolicy for security
- Comprehensive RBAC
- Persistent storage
- Full monitoring stack integration
- Custom processors enabled

#### Prerequisites
```bash
# Ensure production Kubernetes cluster
kubectl cluster-info

# Create production secrets
kubectl create secret generic db-intelligence-secrets \
  --from-literal=new-relic-license-key="$NEW_RELIC_LICENSE_KEY" \
  --from-literal=postgres-password="secure-password" \
  --from-literal=mysql-password="secure-password" \
  --namespace=db-intelligence
```

#### Deployment
```bash
# Deploy production setup
kubectl apply -f kubernetes-production.yaml

# Monitor deployment
kubectl get statefulset -n db-intelligence
kubectl get hpa -n db-intelligence
kubectl get pdb -n db-intelligence

# Check pod distribution
kubectl get pods -n db-intelligence -o wide

# Access services
kubectl port-forward -n db-intelligence svc/otel-collector-service 8889:8889
kubectl port-forward -n db-intelligence svc/otel-collector-service 55679:55679

# View metrics
curl http://localhost:8889/metrics

# Debug interface
open http://localhost:55679/debug/tracez
```

#### Scaling
```bash
# Manual scaling
kubectl scale statefulset otel-collector --replicas=5 -n db-intelligence

# Check autoscaler
kubectl describe hpa otel-collector-hpa -n db-intelligence
```

## Environment Variables

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `NEW_RELIC_LICENSE_KEY` | New Relic license key | `eu01xxNRAL-your-key` |
| `OTLP_ENDPOINT` | OTLP endpoint URL | `https://otlp.nr-data.net:4318` |

### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ENVIRONMENT` | Deployment environment | `development` |
| `COLLECTION_INTERVAL` | Metrics collection interval | `30s` |
| `LOG_LEVEL` | Collector log level | `info` |
| `ADAPTIVE_SAMPLING_ENABLED` | Enable adaptive sampling | `true` |
| `CIRCUIT_BREAKER_ENABLED` | Enable circuit breaker | `true` |
| `VERIFICATION_ENABLED` | Enable verification processor | `true` |

## Monitoring and Observability

### Health Checks

All deployments include health check endpoints:

```bash
# Collector health
curl http://localhost:13133/health

# Detailed health info
curl http://localhost:13133/health?details=true
```

### Metrics Endpoints

```bash
# Prometheus metrics
curl http://localhost:8889/metrics

# Internal collector metrics
curl http://localhost:8888/metrics
```

### Debug Information

```bash
# zPages interface
open http://localhost:55679/debug/tracez
open http://localhost:55679/debug/pipelinez
open http://localhost:55679/debug/servicez
```

### Log Analysis

```bash
# Docker logs
docker logs db-intel-collector-simple -f

# Kubernetes logs
kubectl logs -n db-intelligence deployment/otel-collector -f

# Structured log analysis
docker logs db-intel-collector-simple 2>&1 | jq '.'
```

## Security Considerations

### Network Security

- **Docker**: Internal networks with restricted access
- **Kubernetes**: NetworkPolicy restricts ingress/egress
- **TLS**: HTTPS endpoints for external communication

### Access Control

- **Docker**: Non-root user execution
- **Kubernetes**: RBAC with minimal required permissions
- **Secrets**: Encrypted secret storage

### Data Protection

- **PII Sanitization**: Automatic removal of sensitive data
- **Query Sanitization**: Literal values replaced with placeholders
- **Audit Logging**: All configuration changes logged

## Troubleshooting

### Common Issues

#### Collector Not Starting
```bash
# Check configuration
docker-compose -f docker-compose-simple.yaml config

# Validate collector config
docker run --rm -v $(pwd)/configs:/configs \
  otel/opentelemetry-collector-contrib:latest \
  --config=/configs/collector-simple.yaml --dry-run
```

#### Database Connection Issues
```bash
# Test PostgreSQL connection
docker exec -it db-intel-postgres-simple \
  psql -U testuser -d testdb -c "SELECT 1"

# Check monitoring user permissions
docker exec -it db-intel-postgres-simple \
  psql -U monitoring -d testdb -c "SELECT * FROM pg_stat_statements LIMIT 1"
```

#### Missing Metrics in New Relic
```bash
# Check OTLP export
curl -X POST ${OTLP_ENDPOINT}/v1/metrics \
  -H "api-key: ${NEW_RELIC_LICENSE_KEY}" \
  -H "Content-Type: application/x-protobuf" \
  --data-binary @test-metrics.pb

# Verify collector pipeline
curl http://localhost:55679/debug/pipelinez
```

### Debug Mode

Enable debug mode for detailed troubleshooting:

```yaml
# Add to exporters section
debug:
  verbosity: detailed
  sampling_initial: 1
  sampling_thereafter: 1
```

### Performance Tuning

#### Memory Optimization
```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512  # Adjust based on available memory
    spike_limit_mib: 128
```

#### Batching Optimization
```yaml
processors:
  batch:
    timeout: 5s
    send_batch_size: 1000  # Increase for higher throughput
    send_batch_max_size: 2000
```

## Migration Guide

### From Legacy Custom Receivers

1. **Identify Custom Logic**: Extract business logic from custom receivers
2. **Map to SQL Queries**: Convert custom queries to SQL Query receiver format
3. **Migrate Processors**: Move custom processing to standard transform processors
4. **Test Thoroughly**: Validate data parity between old and new approaches

### From Direct Database Monitoring

1. **Add Monitoring User**: Create dedicated user with minimal permissions
2. **Enable Extensions**: Install pg_stat_statements and other monitoring extensions
3. **Configure Receivers**: Set up OTEL receivers for your database
4. **Validate Metrics**: Ensure metric compatibility with existing dashboards

## Best Practices

### Configuration Management

- **Version Control**: Store all configurations in Git
- **Environment Separation**: Use different configs for dev/staging/prod
- **Secret Management**: Use proper secret management tools
- **Validation**: Always validate configurations before deployment

### Monitoring Strategy

- **Layered Monitoring**: Infrastructure + Application + Business metrics
- **SLA Monitoring**: Track database SLA metrics
- **Alerting**: Set up proactive alerts for issues
- **Capacity Planning**: Monitor resource usage trends

### Security

- **Least Privilege**: Minimal database permissions for monitoring
- **Network Segmentation**: Isolate monitoring infrastructure
- **Regular Updates**: Keep OTEL collector versions current
- **Audit Trails**: Log all configuration changes

## Support and Resources

### Documentation
- [OpenTelemetry Collector Documentation](https://opentelemetry.io/docs/collector/)
- [New Relic OTLP Integration](https://docs.newrelic.com/docs/more-integrations/open-source-telemetry-integrations/opentelemetry/)
- [PostgreSQL Monitoring Guide](https://www.postgresql.org/docs/current/monitoring-stats.html)

### Community
- [OpenTelemetry Community](https://opentelemetry.io/community/)
- [CNCF Slack #opentelemetry](https://cloud-native.slack.com/archives/C0150BTKJKA)

### Issues and Feedback
For issues specific to this implementation, please check the project's issue tracker and troubleshooting documentation.