# Deployment Guide

This guide covers deploying Database Intelligence in various environments.

## Table of Contents
- [Quick Start](#quick-start)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Configuration Management](#configuration-management)
- [Security Best Practices](#security-best-practices)
- [Monitoring and Troubleshooting](#monitoring-and-troubleshooting)

## Quick Start

The fastest way to get started is using Docker Compose:

```bash
# 1. Clone the repository
git clone https://github.com/newrelic/database-intelligence
cd database-intelligence

# 2. Copy environment template
cp configs/env-templates/postgresql.env .env

# 3. Edit .env with your credentials
vim .env

# 4. Start services
docker-compose -f docker-compose.databases.yml up -d

# 5. Verify metrics
./scripts/validate-metrics.sh postgresql
```

## Docker Deployment

### Single Database Deployment

For deploying a single database collector:

```bash
# PostgreSQL
docker run -d \
  --name otel-postgres \
  -v $(pwd)/configs/postgresql-maximum-extraction.yaml:/etc/otel-collector-config.yaml \
  --env-file configs/env-templates/postgresql.env \
  otel/opentelemetry-collector-contrib:latest

# MySQL
docker run -d \
  --name otel-mysql \
  -v $(pwd)/configs/mysql-maximum-extraction.yaml:/etc/otel-collector-config.yaml \
  --env-file configs/env-templates/mysql.env \
  otel/opentelemetry-collector-contrib:latest

# MongoDB
docker run -d \
  --name otel-mongodb \
  -v $(pwd)/configs/mongodb-maximum-extraction.yaml:/etc/otel-collector-config.yaml \
  --env-file configs/env-templates/mongodb.env \
  otel/opentelemetry-collector-contrib:latest

# MSSQL
docker run -d \
  --name otel-mssql \
  -v $(pwd)/configs/mssql-maximum-extraction.yaml:/etc/otel-collector-config.yaml \
  --env-file configs/env-templates/mssql.env \
  otel/opentelemetry-collector-contrib:latest

# Oracle
docker run -d \
  --name otel-oracle \
  -v $(pwd)/configs/oracle-maximum-extraction.yaml:/etc/otel-collector-config.yaml \
  --env-file configs/env-templates/oracle.env \
  otel/opentelemetry-collector-contrib:latest
```

### Multi-Database Deployment

Use the provided docker-compose file:

```bash
# Start all databases and collectors
./scripts/start-all-databases.sh

# Stop all services
./scripts/stop-all-databases.sh
```

## Kubernetes Deployment

### Using Helm Charts

```bash
# Add the OpenTelemetry Helm repository
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

# Install for PostgreSQL
helm install otel-postgres open-telemetry/opentelemetry-collector \
  --set mode=deployment \
  --set config.file=configs/postgresql-maximum-extraction.yaml \
  --set-file config.config=configs/postgresql-maximum-extraction.yaml

# Install for other databases similarly
```

### Using Kubernetes Manifests

Create a ConfigMap with your configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
data:
  collector.yaml: |
    # Paste your database-specific configuration here
```

Deploy the collector:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
spec:
  replicas: 1
  selector:
    matchLabels:
      app: otel-collector
  template:
    metadata:
      labels:
        app: otel-collector
    spec:
      containers:
      - name: otel-collector
        image: otel/opentelemetry-collector-contrib:latest
        args: ["--config=/etc/otel-collector-config.yaml"]
        volumeMounts:
        - name: config
          mountPath: /etc
        env:
        - name: NEW_RELIC_LICENSE_KEY
          valueFrom:
            secretKeyRef:
              name: newrelic-license
              key: license-key
      volumes:
      - name: config
        configMap:
          name: otel-collector-config
```

## Configuration Management

### Environment Variables

Use environment files for sensitive data:

```bash
# Create from template
cp configs/env-templates/${DATABASE}.env .env

# Edit with your values
vim .env

# Use with Docker
docker run --env-file .env ...

# Use with Kubernetes
kubectl create secret generic db-credentials --from-env-file=.env
```

### Secret Management

For production deployments:

1. **Kubernetes Secrets**:
   ```bash
   kubectl create secret generic db-credentials \
     --from-literal=POSTGRES_PASSWORD=secretpass \
     --from-literal=NEW_RELIC_LICENSE_KEY=licensekey
   ```

2. **HashiCorp Vault**:
   ```yaml
   env:
   - name: POSTGRES_PASSWORD
     value: ${vault:secret/data/database#password}
   ```

3. **AWS Secrets Manager**:
   ```yaml
   env:
   - name: POSTGRES_PASSWORD
     valueFrom:
       secretKeyRef:
         name: db-password
         key: password
   ```

## Security Best Practices

### Network Security

1. **Use TLS/SSL** for all database connections:
   ```yaml
   postgresql:
     endpoint: ${POSTGRES_HOST}:5432
     transport: tcp
     tls:
       insecure: false
       ca_file: /etc/ssl/certs/ca.crt
       cert_file: /etc/ssl/certs/client.crt
       key_file: /etc/ssl/certs/client.key
   ```

2. **Network Policies** in Kubernetes:
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: otel-collector-netpol
   spec:
     podSelector:
       matchLabels:
         app: otel-collector
     policyTypes:
     - Ingress
     - Egress
     egress:
     - to:
       - namespaceSelector:
           matchLabels:
             name: database
       ports:
       - protocol: TCP
         port: 5432
   ```

### Authentication

1. **Use least-privilege database users**
2. **Rotate credentials regularly**
3. **Never commit credentials to version control**

## Monitoring and Troubleshooting

### Health Checks

All collectors expose health endpoints:
- Health: `http://localhost:13133/health`
- Metrics: `http://localhost:8888/metrics`
- pprof: `http://localhost:1777/debug/pprof/`

### Common Issues

1. **No metrics appearing**:
   ```bash
   # Check collector logs
   docker logs otel-collector
   
   # Validate configuration
   ./scripts/validate-config.sh configs/postgresql-maximum-extraction.yaml
   
   # Test database connectivity
   ./scripts/test-database-config.sh postgresql
   ```

2. **High memory usage**:
   - Adjust memory_limiter settings
   - Increase batch timeout
   - Enable sampling

3. **Connection errors**:
   - Verify credentials
   - Check network connectivity
   - Ensure database user has required permissions

### Performance Tuning

1. **Batch Processing**:
   ```yaml
   processors:
     batch:
       timeout: 30s  # Increase for better compression
       send_batch_size: 5000  # Larger batches
   ```

2. **Memory Limits**:
   ```yaml
   processors:
     memory_limiter:
       limit_mib: 2048  # Increase for high-volume
       spike_limit_mib: 512
   ```

3. **Collection Intervals**:
   - Adjust based on metric importance
   - Use different pipelines for different frequencies

## Production Checklist

- [ ] Credentials stored securely
- [ ] TLS/SSL enabled for database connections
- [ ] Resource limits configured
- [ ] Health checks enabled
- [ ] Monitoring alerts configured
- [ ] Backup configuration in place
- [ ] Log rotation configured
- [ ] Network policies applied
- [ ] Regular credential rotation scheduled
- [ ] Documentation updated

## Next Steps

- Review [Troubleshooting Guide](TROUBLESHOOTING.md) for common issues
- Configure [New Relic Dashboards](https://docs.newrelic.com/docs/query-your-data/explore-query-data/dashboards/introduction-dashboards/)
- Set up [Alerting](https://docs.newrelic.com/docs/alerts-applied-intelligence/new-relic-alerts/get-started/introduction-applied-intelligence/)
