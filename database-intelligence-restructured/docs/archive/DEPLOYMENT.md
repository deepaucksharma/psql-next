# Deployment Guide

Production deployment options for Database Intelligence with OpenTelemetry.

## 🐳 Docker Deployment (Recommended)

### Quick Start with Docker Compose

```bash
# 1. Download deployment files
curl -L -o docker-compose.yml \
  https://raw.githubusercontent.com/your-repo/deployments/docker/compose/docker-compose.yaml

# 2. Create environment file
cat > .env << EOF
NEW_RELIC_LICENSE_KEY=your_license_key
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
DB_ENDPOINT=postgresql://otel_monitor:password@your_db:5432/mydb
SERVICE_NAME=my-database
ENVIRONMENT=production
EOF

# 3. Deploy
docker-compose up -d
```

### Manual Docker Run

```bash
docker run -d \
  --name db-otel \
  -p 13133:13133 \
  -p 8888:8888 \
  -e NEW_RELIC_LICENSE_KEY="your_license_key" \
  -e DB_ENDPOINT="postgresql://otel_monitor:password@host.docker.internal:5432/mydb" \
  -e SERVICE_NAME="my-database" \
  -e ENVIRONMENT="production" \
  -v $(pwd)/config.yaml:/etc/otelcol/config.yaml \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otelcol/config.yaml
```

## ☸️ Kubernetes Deployment

### Using Helm (Recommended)

```bash
# Add OpenTelemetry Helm repo
helm repo add open-telemetry https://open-telemetry.github.io/opentelemetry-helm-charts
helm repo update

# Create values file
cat > values.yaml << EOF
mode: deployment
replicaCount: 1

config:
  receivers:
    postgresql:
      endpoint: "\${DB_ENDPOINT}"
      username: "\${DB_USERNAME}"
      password: "\${DB_PASSWORD}"
      collection_interval: 30s
  
  processors:
    memory_limiter:
      limit_mib: 512
    resource:
      attributes:
        - key: service.name
          value: "\${SERVICE_NAME}"
          action: upsert
    batch: {}
  
  exporters:
    otlp:
      endpoint: "\${NEW_RELIC_OTLP_ENDPOINT}"
      headers:
        api-key: "\${NEW_RELIC_LICENSE_KEY}"
  
  service:
    pipelines:
      metrics:
        receivers: [postgresql]
        processors: [memory_limiter, resource, batch]
        exporters: [otlp]

extraEnvs:
  - name: NEW_RELIC_LICENSE_KEY
    valueFrom:
      secretKeyRef:
        name: nr-license-key
        key: license-key
  - name: DB_USERNAME
    valueFrom:
      secretKeyRef:
        name: db-credentials
        key: username
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: db-credentials
        key: password

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi
EOF

# Create secrets
kubectl create secret generic nr-license-key \
  --from-literal=license-key=your_license_key

kubectl create secret generic db-credentials \
  --from-literal=username=otel_monitor \
  --from-literal=password=secure_password

# Deploy
helm install db-otel open-telemetry/opentelemetry-collector -f values.yaml
```

## 🖥️ Systemd Deployment

### Install as Service

```bash
# 1. Download collector binary
sudo curl -L -o /usr/local/bin/otelcol \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/latest/download/otelcol_linux_amd64
sudo chmod +x /usr/local/bin/otelcol

# 2. Create user and directories
sudo useradd --no-create-home --shell /bin/false otelcol
sudo mkdir -p /etc/otelcol /var/lib/otelcol /var/log/otelcol
sudo chown otelcol:otelcol /var/lib/otelcol /var/log/otelcol

# 3. Copy configuration
sudo cp config.yaml /etc/otelcol/config.yaml
sudo chown otelcol:otelcol /etc/otelcol/config.yaml

# 4. Create environment file
sudo tee /etc/otelcol/environment << EOF
NEW_RELIC_LICENSE_KEY=your_license_key
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
DB_ENDPOINT=postgresql://otel_monitor:password@localhost:5432/mydb
SERVICE_NAME=prod-postgres
ENVIRONMENT=production
EOF

# 5. Create systemd service
sudo tee /etc/systemd/system/otelcol.service << EOF
[Unit]
Description=OpenTelemetry Collector
After=network.target

[Service]
Type=simple
User=otelcol
Group=otelcol
ExecStart=/usr/local/bin/otelcol --config=/etc/otelcol/config.yaml
EnvironmentFile=/etc/otelcol/environment
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 6. Enable and start
sudo systemctl daemon-reload
sudo systemctl enable otelcol
sudo systemctl start otelcol
```

## 🔒 Security Best Practices

### Database Security

```sql
-- Create dedicated monitoring user
CREATE USER otel_monitor WITH PASSWORD 'secure_random_password';
GRANT CONNECT ON DATABASE mydb TO otel_monitor;
GRANT USAGE ON SCHEMA public TO otel_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO otel_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA information_schema TO otel_monitor;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO otel_monitor;
```

### Secret Management

```bash
# Kubernetes secrets
kubectl create secret generic db-credentials \
  --from-literal=username=otel_monitor \
  --from-literal=password=secure_password

# Docker secrets
echo "secure_password" | docker secret create db_password -
```

## 🎯 Performance Tuning

### Resource Allocation

| Environment | CPU | Memory | Storage |
|-------------|-----|--------|---------|
| Development | 100m | 256Mi | 1Gi |
| Staging | 200m | 512Mi | 5Gi |
| Production | 500m | 1Gi | 20Gi |

### Collection Intervals

```yaml
receivers:
  postgresql:
    collection_interval: 30s  # Standard metrics
  sqlquery:
    collection_interval: 60s  # Custom queries
```

## 🚨 Troubleshooting

### Health Checks

```bash
# Health endpoint
curl http://localhost:13133/health

# Metrics endpoint  
curl http://localhost:8888/metrics

# Kubernetes
kubectl port-forward deployment/db-otel 13133:13133
curl http://localhost:13133/health
```

### Common Issues

1. **Database Connection**:
   ```bash
   # Test connection
   psql "${DB_ENDPOINT}" -c "SELECT 1"
   ```

2. **Memory Issues**:
   ```yaml
   processors:
     memory_limiter:
       limit_mib: 512
   ```

3. **Export Failures**:
   ```bash
   # Test New Relic connectivity
   curl -v -X POST "${NEW_RELIC_OTLP_ENDPOINT}/v1/metrics" \
     -H "Api-Key: ${NEW_RELIC_LICENSE_KEY}"
   ```

This deployment guide provides production-ready configurations for all major platforms.