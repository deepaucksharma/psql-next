# Database Intelligence Collector - Quick Start Guide

Get up and running with the Database Intelligence OpenTelemetry Collector in minutes!

## Prerequisites
- Docker and Docker Compose (for quick start)
- PostgreSQL and/or MySQL database access
- New Relic account with License Key

## Quick Start with Docker

### 1. Clone and Setup
```bash
# Clone the repository
git clone https://github.com/deepaksharma/db-otel.git
cd db-otel/database-intelligence-restructured

# Copy environment template
cp configs/templates/env.template.fixed .env
```

### 2. Configure Environment
Edit `.env` file with your database and New Relic credentials:
```bash
# Essential configurations
NEW_RELIC_LICENSE_KEY=your_license_key_here
POSTGRES_HOST=your-postgres-host
POSTGRES_USER=your-postgres-user
POSTGRES_PASSWORD=your-postgres-password
MYSQL_HOST=your-mysql-host
MYSQL_USER=your-mysql-user
MYSQL_PASSWORD=your-mysql-password
```

### 3. Run with Docker Compose
```bash
# Start the collector
docker-compose -f deployments/docker/compose/docker-compose.yaml up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f collector
```

### 4. Verify in New Relic
Go to New Relic One and check for metrics:
- Navigate to **APM & Services** > **Database Intelligence**
- Or use NRQL: `FROM Metric SELECT * WHERE service.name = 'database-intelligence-collector'`

## Manual Installation

### 1. Build the Collector
```bash
# Run the build script
./scripts/build-collector.sh

# Or build manually
go install go.opentelemetry.io/collector/cmd/builder@v0.105.0
builder --config=otelcol-builder-config-complete.yaml
```

### 2. Choose Configuration Level

#### Basic (Standard Components Only)
```bash
cd distributions/production
./database-intelligence-collector --config=../../configs/production/config-basic.yaml
```

#### Enhanced (With Resource Processors)
```bash
cd distributions/production
./database-intelligence-collector --config=../../configs/production/config-enhanced.yaml
```

#### Full (All Custom Components)
```bash
cd distributions/production
./database-intelligence-collector --config=../../configs/production/config-full.yaml
```

## Configuration Examples

### Minimal Configuration
For testing with a single PostgreSQL database:
```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    databases: [postgres]
    collection_interval: 60s

processors:
  batch:
    timeout: 1s

exporters:
  otlphttp:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlphttp]
```

### Production Configuration
For production use with both PostgreSQL and MySQL:
```yaml
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:5432
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    databases: [${POSTGRES_DB}]
    collection_interval: 60s
    tls:
      insecure: false
      ca_file: ${POSTGRES_CA_FILE}

  mysql:
    endpoint: ${MYSQL_HOST}:3306
    username: ${MYSQL_USER}
    password: ${MYSQL_PASSWORD}
    database: ${MYSQL_DB}
    collection_interval: 60s

processors:
  memory_limiter:
    limit_mib: 512
  resource:
    attributes:
      - key: service.name
        value: database-intelligence-collector
        action: upsert
      - key: deployment.environment
        value: production
        action: upsert
  batch:
    timeout: 1s
    send_batch_size: 1024

exporters:
  otlphttp:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    compression: gzip
    retry_on_failure:
      enabled: true

service:
  pipelines:
    metrics/postgresql:
      receivers: [postgresql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlphttp]
    metrics/mysql:
      receivers: [mysql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlphttp]
```

## Kubernetes Deployment

### Using Helm
```bash
# Add custom values
cat > values.yaml <<EOF
image:
  repository: ghcr.io/deepaksharma/db-otel/database-intelligence-collector
  tag: latest

config:
  newrelic:
    licenseKey: your-license-key
  postgres:
    host: postgres.default.svc.cluster.local
    user: postgres
    password: postgres
  mysql:
    host: mysql.default.svc.cluster.local
    user: root
    password: mysql
EOF

# Install
helm install database-intelligence ./deployments/kubernetes/helm -f values.yaml
```

### Using kubectl
```bash
# Create namespace
kubectl create namespace database-intelligence

# Create secret for credentials
kubectl create secret generic db-credentials \
  --from-literal=postgres-password=your-password \
  --from-literal=mysql-password=your-password \
  --from-literal=newrelic-license-key=your-key \
  -n database-intelligence

# Apply manifests
kubectl apply -f deployments/kubernetes/manifests/ -n database-intelligence
```

## Monitoring and Troubleshooting

### Health Check
```bash
# Check collector health
curl http://localhost:13133/health

# View collector metrics
curl http://localhost:8888/metrics
```

### Common Issues

#### No Data in New Relic
1. Check license key is correct
2. Verify network connectivity: `curl -I https://otlp.nr-data.net`
3. Check logs for errors: `docker logs collector`
4. Ensure service.name attribute is set

#### High Memory Usage
Adjust memory limits in configuration:
```yaml
processors:
  memory_limiter:
    limit_mib: 256  # Reduce from default
    spike_limit_mib: 64
```

#### Connection Errors
Verify database connectivity:
```bash
# PostgreSQL
psql -h $POSTGRES_HOST -U $POSTGRES_USER -d postgres -c "SELECT 1"

# MySQL
mysql -h $MYSQL_HOST -u $MYSQL_USER -p -e "SELECT 1"
```

## Next Steps

### Enable Custom Components
1. Build collector with custom components:
   ```bash
   builder --config=otelcol-builder-config-complete.yaml
   ```

2. Use the full configuration:
   ```bash
   ./database-intelligence-collector --config=configs/production/config-full.yaml
   ```

### Create Dashboards
Import the example dashboard:
1. Go to New Relic One > Dashboards
2. Click "Import dashboard"
3. Upload `dashboards/newrelic/database-intelligence-dashboard.json`

### Set Up Alerts
Use the provided alert configurations:
```bash
# View available alerts
cat dashboards/newrelic/alerts-config.yaml

# Apply via New Relic API or Terraform
```

### Scale Out
For multiple databases:
1. Use configuration management (Ansible, Terraform)
2. Deploy multiple collector instances
3. Use service discovery for dynamic databases

## Resources
- [Custom Components Guide](./custom-components-guide.md)
- [NRQL Query Examples](../dashboards/newrelic/nrql-queries.md)
- [Architecture Documentation](./architecture/)
- [Troubleshooting Guide](./troubleshooting.md)

## Support
- GitHub Issues: https://github.com/deepaksharma/db-otel/issues
- New Relic Support: https://support.newrelic.com
- OpenTelemetry Community: https://opentelemetry.io/community/