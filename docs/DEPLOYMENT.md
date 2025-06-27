# Deployment Guide

## Table of Contents
- [Prerequisites](#prerequisites)
- [Configuration](#configuration)
- [Deployment Options](#deployment-options)
- [PostgreSQL Setup](#postgresql-setup)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### PostgreSQL Requirements
- PostgreSQL 12 or higher
- `pg_stat_statements` extension enabled
- Monitoring user with appropriate permissions
- (Optional) `pg_wait_sampling` for enhanced wait events
- (Optional) `pg_stat_monitor` for detailed query metrics

### System Requirements
- Linux, macOS, or Windows
- 256MB RAM minimum (512MB recommended)
- Network access to PostgreSQL instance
- New Relic license key

## Configuration

### Environment Variables

```bash
# Required
NEW_RELIC_LICENSE_KEY=your_license_key_here
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=monitoring
POSTGRES_PASSWORD=secure_password
POSTGRES_DATABASE=postgres

# Optional
COLLECTOR_MODE=hybrid              # nri, otel, or hybrid
COLLECTION_INTERVAL_SECS=30
HEALTH_CHECK_PORT=8080
SANITIZE_QUERY_TEXT=true
SANITIZATION_MODE=smart            # full, smart, or none
```

### Configuration File (config.toml)

```toml
# Connection settings
connection_string = "postgresql://monitoring:password@localhost:5432/postgres"
host = "localhost"
port = 5432
databases = ["postgres", "myapp"]
max_connections = 5
connect_timeout_secs = 30

# Collection settings
collection_interval_secs = 30
collection_mode = "hybrid"
query_monitoring_count_threshold = 20
query_monitoring_response_time_threshold = 500

# Features
enable_extended_metrics = true
enable_ash = true
ash_sample_interval_secs = 1
ash_retention_hours = 1
ash_max_memory_mb = 100

# Query sanitization
sanitize_query_text = true
sanitization_mode = "smart"

# PgBouncer (optional)
[pgbouncer]
enabled = false
admin_connection_string = "postgresql://pgbouncer@localhost:6432/pgbouncer"

# NRI output
[outputs.nri]
enabled = true
entity_key = "${HOSTNAME}:${PORT}"
integration_name = "com.newrelic.postgresql"

# OTLP output
[outputs.otlp]
enabled = true
endpoint = "http://localhost:4317"
compression = "gzip"
timeout_secs = 30
headers = [["api-key", "${NEW_RELIC_LICENSE_KEY}"]]

# Multi-instance support
[[instances]]
name = "primary"
connection_string = "postgresql://user:pass@primary:5432/db"
enabled = true

[[instances]]
name = "replica"
connection_string = "postgresql://user:pass@replica:5432/db"
enabled = true
```

## Deployment Options

### 1. Standalone Binary

```bash
# Download and install
wget https://github.com/newrelic/postgres-unified-collector/releases/latest/download/postgres-unified-collector
chmod +x postgres-unified-collector
sudo mv postgres-unified-collector /usr/local/bin/

# Run with config file
postgres-unified-collector --config /etc/postgres-collector/config.toml

# Run with environment variables
export NEW_RELIC_LICENSE_KEY=your_key
postgres-unified-collector --mode nri
```

### 2. Docker

```bash
# Using docker-compose
docker-compose up -d

# Standalone container
docker run -d \
  --name postgres-collector \
  -e NEW_RELIC_LICENSE_KEY=your_key \
  -e POSTGRES_HOST=postgres \
  -e POSTGRES_PORT=5432 \
  -e POSTGRES_USER=monitoring \
  -e POSTGRES_PASSWORD=password \
  -e COLLECTOR_MODE=hybrid \
  -p 8080:8080 \
  newrelic/postgres-unified-collector:latest
```

### 3. Kubernetes

#### Single Collector Deployment

```bash
# Using provided script
./scripts/deploy-k8s.sh

# Manual deployment
kubectl create namespace postgres-monitoring
kubectl create secret generic postgres-credentials \
  --from-literal=username=monitoring \
  --from-literal=password=password \
  -n postgres-monitoring

kubectl create secret generic newrelic-license \
  --from-literal=key=your_license_key \
  -n postgres-monitoring

kubectl apply -f deployments/kubernetes/
```

#### Dual Collector Deployment (NRI + OTLP)

```bash
# Deploy both NRI and OTLP collectors
./scripts/deploy-dual-collectors.sh deploy

# Check status
./scripts/deploy-dual-collectors.sh status

# View logs
kubectl logs -f deployment/postgres-collector-nri -n postgres-monitoring
kubectl logs -f deployment/postgres-collector-otlp -n postgres-monitoring
```

### 4. Systemd Service

```ini
# /etc/systemd/system/postgres-collector.service
[Unit]
Description=PostgreSQL Unified Collector
After=network.target

[Service]
Type=simple
User=postgres-collector
Group=postgres-collector
ExecStart=/usr/local/bin/postgres-unified-collector --config /etc/postgres-collector/config.toml
Restart=always
RestartSec=10
Environment="NEW_RELIC_LICENSE_KEY=your_key"

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start
sudo systemctl enable postgres-collector
sudo systemctl start postgres-collector
sudo systemctl status postgres-collector
```

### 5. New Relic Infrastructure Integration

```yaml
# /etc/newrelic-infra/integrations.d/postgresql-config.yml
integrations:
  - name: nri-postgresql
    command: postgres-unified-collector
    arguments:
      mode: nri
    env:
      HOSTNAME: ${HOSTNAME}
      PORT: 5432
      USERNAME: monitoring
      PASSWORD: secure_password
```

## PostgreSQL Setup

### 1. Enable Required Extensions

```sql
-- Required
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Optional but recommended
CREATE EXTENSION IF NOT EXISTS pg_wait_sampling;
CREATE EXTENSION IF NOT EXISTS pg_stat_monitor;

-- Configure pg_stat_statements
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET pg_stat_statements.track = 'all';
ALTER SYSTEM SET pg_stat_statements.max = 10000;

-- Reload configuration
SELECT pg_reload_conf();
```

### 2. Create Monitoring User

```sql
-- Create user
CREATE USER monitoring WITH PASSWORD 'secure_password';

-- Grant permissions
GRANT pg_monitor TO monitoring;
GRANT CONNECT ON DATABASE postgres TO monitoring;

-- For each database to monitor
GRANT CONNECT ON DATABASE myapp TO monitoring;
\c myapp
GRANT USAGE ON SCHEMA public TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring;
```

### 3. Configure pg_hba.conf

```conf
# Allow monitoring user from collector
host    all    monitoring    10.0.0.0/8    md5
host    all    monitoring    172.16.0.0/12    md5
```

## Monitoring

### Health Check Endpoints

- **Health**: `http://localhost:8080/health`
  ```json
  {
    "status": "healthy",
    "last_collection": "2024-01-15T10:30:00Z",
    "metrics_sent": 150,
    "metrics_failed": 0
  }
  ```

- **Readiness**: `http://localhost:8080/ready`
- **Metrics**: `http://localhost:9090/metrics` (Prometheus format)

### Verification Script

```bash
# Run verification
./scripts/verify-metrics.sh

# Check specific event types
./scripts/verify-metrics.sh --type PostgresSlowQueries
```

### New Relic Dashboards

1. **Infrastructure**: View PostgreSQL entities
2. **Query Insights**: Slow query analysis
3. **Custom Dashboards**: Import from `dashboards/` directory

## Troubleshooting

### Common Issues

#### 1. Connection Refused
```bash
# Check PostgreSQL is running
pg_isready -h localhost -p 5432

# Verify connection string
psql "postgresql://monitoring:password@localhost:5432/postgres"
```

#### 2. Missing Extensions
```sql
-- Check installed extensions
SELECT * FROM pg_available_extensions WHERE name LIKE 'pg_stat%';

-- Install missing extension
CREATE EXTENSION pg_stat_statements;
```

#### 3. Permission Denied
```sql
-- Verify user permissions
\du monitoring

-- Grant missing permissions
GRANT pg_monitor TO monitoring;
```

#### 4. No Metrics in New Relic
```bash
# Check collector logs
docker logs postgres-collector

# Verify license key
echo $NEW_RELIC_LICENSE_KEY

# Test NRI output
postgres-unified-collector --mode nri --dry-run
```

### Debug Mode

```bash
# Enable debug logging
export RUST_LOG=debug
postgres-unified-collector --debug

# Or in config
[logging]
level = "debug"
```

### Performance Tuning

```toml
# Adjust for high-volume databases
collection_interval_secs = 60
query_monitoring_count_threshold = 50
max_connections = 20

# Reduce memory usage
ash_max_samples = 1000
ash_max_memory_mb = 50

# Sampling for high traffic
[sampling]
mode = "adaptive"
base_sample_rate = 0.1
```

## Security Best Practices

1. **Use Secrets Management**
   - Kubernetes secrets
   - HashiCorp Vault
   - AWS Secrets Manager

2. **Network Security**
   - Use SSL/TLS connections
   - Restrict collector IP addresses
   - Use private networks

3. **Least Privilege**
   - Only grant necessary permissions
   - Use read-only accounts
   - Separate accounts per database

4. **Query Sanitization**
   - Enable PII removal
   - Review sanitization patterns
   - Audit sanitized queries

## Migration from nri-postgresql

### 1. Binary Replacement
```bash
# Backup existing
sudo mv /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql{,.backup}

# Install new collector
sudo cp postgres-unified-collector /var/db/newrelic-infra/newrelic-integrations/bin/nri-postgresql
```

### 2. Configuration
- Existing environment variables work as-is
- No changes required to integration config

### 3. Validation
```bash
# Test compatibility
postgres-unified-collector --mode nri --dry-run

# Compare outputs
diff <(nri-postgresql.backup --metrics) <(postgres-unified-collector --mode nri)
```