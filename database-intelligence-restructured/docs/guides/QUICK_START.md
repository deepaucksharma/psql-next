# Quick Start Guide

Get Database Intelligence with OpenTelemetry running in 5 minutes.

## 🚀 Prerequisites (2 minutes)

1. **Database Setup**:
   ```sql
   -- PostgreSQL: Create monitoring user
   CREATE USER otel_monitor WITH LOGIN PASSWORD 'secure_password';
   GRANT CONNECT ON DATABASE your_db TO otel_monitor;
   GRANT USAGE ON SCHEMA public TO otel_monitor;
   GRANT SELECT ON ALL TABLES IN SCHEMA public TO otel_monitor;
   
   -- MySQL: Create monitoring user  
   CREATE USER 'otel_monitor'@'%' IDENTIFIED BY 'secure_password';
   GRANT SELECT ON your_db.* TO 'otel_monitor'@'%';
   FLUSH PRIVILEGES;
   ```

2. **New Relic Setup**:
   - Get your license key from [New Relic API Keys](https://one.newrelic.com/launcher/api-keys-ui.api-keys-launcher)
   - Note your OTLP endpoint (usually `https://otlp.nr-data.net:4318`)

## ⚡ Option 1: Docker (Fastest)

```bash
# 1. Create environment file
cat > .env << EOF
NEW_RELIC_LICENSE_KEY=your_license_key_here
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net:4318
DB_ENDPOINT=postgresql://otel_monitor:secure_password@your_db_host:5432/your_db
SERVICE_NAME=my-database
ENVIRONMENT=production
EOF

# 2. Download and run
curl -L -o docker-compose.yml \
  https://raw.githubusercontent.com/your-repo/deployments/docker/compose/docker-compose.yaml

docker-compose up -d
```

## 🔧 Option 2: Manual Setup

### Step 1: Download Collector
```bash
# Linux
curl -L -o otelcol \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/latest/download/otelcol_linux_amd64
chmod +x otelcol

# macOS  
curl -L -o otelcol \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/latest/download/otelcol_darwin_amd64
chmod +x otelcol

# Windows
curl -L -o otelcol.exe \
  https://github.com/open-telemetry/opentelemetry-collector-releases/releases/latest/download/otelcol_windows_amd64.exe
```

### Step 2: Download Configuration
```bash
# PostgreSQL
curl -L -o config.yaml \
  https://raw.githubusercontent.com/your-repo/configs/examples/config-only-base.yaml

# MySQL
curl -L -o config.yaml \
  https://raw.githubusercontent.com/your-repo/configs/examples/config-only-mysql.yaml
```

### Step 3: Set Environment Variables
```bash
export NEW_RELIC_LICENSE_KEY="your_license_key_here"
export NEW_RELIC_OTLP_ENDPOINT="https://otlp.nr-data.net:4318"
export DB_ENDPOINT="postgresql://otel_monitor:secure_password@localhost:5432/mydb"
export SERVICE_NAME="my-database"
export ENVIRONMENT="production"
```

### Step 4: Run Collector
```bash
./otelcol --config=config.yaml
```

## ✅ Verification (1 minute)

### 1. Check Collector Health
```bash
# Health endpoint
curl http://localhost:13133/health

# Expected response: {"status":"OK"}
```

### 2. Check Metrics Collection
```bash
# Prometheus metrics endpoint
curl http://localhost:8888/metrics | grep otelcol

# Look for:
# - otelcol_receiver_accepted_metric_points
# - otelcol_exporter_sent_metric_points
```

### 3. Verify New Relic Data
1. Go to [New Relic Infrastructure](https://one.newrelic.com/infra)
2. Look for your service under "Hosts" or "Third-party services"
3. Check for metrics like:
   - `postgresql.connections.active`
   - `postgresql.commits`
   - `postgresql.database.size`

## 📊 What You'll See in New Relic

### Automatic Dashboards
- **Database Overview**: Connection counts, transaction rates
- **Performance Metrics**: Query rates, cache hit ratios
- **Resource Usage**: CPU, memory, disk I/O
- **Health Indicators**: Deadlocks, replication lag

### Key Metrics
```
postgresql.connections.active      # Active connections
postgresql.connections.idle        # Idle connections  
postgresql.transactions.committed  # Committed transactions/sec
postgresql.cache.hit_ratio         # Buffer cache hit ratio
postgresql.database.size           # Database size in bytes
postgresql.locks.relation          # Table-level locks
postgresql.replication.lag         # Replication lag
```

## 🔧 Customization

### Add Custom SQL Queries
Edit your config.yaml to include custom metrics:

```yaml
receivers:
  sqlquery:
    driver: postgres
    datasource: "${DB_ENDPOINT}"
    queries:
      - sql: |
          SELECT 
            schemaname,
            tablename,
            pg_total_relation_size(schemaname||'.'||tablename) as size_bytes
          FROM pg_tables 
          WHERE schemaname = 'public'
        metrics:
          - metric_name: custom.table.size
            value_column: size_bytes
            value_type: gauge
            attribute_columns: [schemaname, tablename]
```

### Adjust Collection Intervals
```yaml
receivers:
  postgresql:
    collection_interval: 15s  # Default: 30s
  sqlquery:
    collection_interval: 60s  # Default: 60s
```

### Add Multiple Databases
```yaml
receivers:
  postgresql/prod:
    endpoint: "${PROD_DB_ENDPOINT}"
    # ... config
  postgresql/staging:
    endpoint: "${STAGING_DB_ENDPOINT}" 
    # ... config

service:
  pipelines:
    metrics:
      receivers: [postgresql/prod, postgresql/staging]
```

## 🚨 Troubleshooting

### Collector Won't Start
```bash
# Check configuration syntax
./otelcol --config=config.yaml --dry-run

# Enable debug logging
export OTEL_LOG_LEVEL=debug
./otelcol --config=config.yaml
```

### No Database Connection
```bash
# Test database connectivity
psql -h localhost -U otel_monitor -d mydb -c "SELECT 1;"

# Check firewall/security groups
telnet your_db_host 5432
```

### No Data in New Relic
```bash
# Check OTLP endpoint
curl -v -X POST "${NEW_RELIC_OTLP_ENDPOINT}/v1/metrics" \
  -H "Api-Key: ${NEW_RELIC_LICENSE_KEY}" \
  -H "Content-Type: application/x-protobuf"

# Verify license key format (should be 40 characters)
echo $NEW_RELIC_LICENSE_KEY | wc -c
```

### High Resource Usage
```yaml
# Add memory limiter
processors:
  memory_limiter:
    limit_mib: 512
    spike_limit_mib: 128

# Reduce collection frequency
receivers:
  postgresql:
    collection_interval: 60s  # From 30s
```

## 📋 Next Steps

1. **Production Deployment**: See [Deployment Guide](DEPLOYMENT.md)
2. **Advanced Configuration**: Review [Configuration Reference](CONFIGURATION.md)
3. **Monitoring Setup**: Configure alerts and dashboards
4. **Performance Tuning**: Optimize for your workload

## 🎯 Common Use Cases

### Multi-Environment Monitoring
```yaml
resource:
  attributes:
    - key: environment
      value: "${ENVIRONMENT}"
    - key: cluster
      value: "${CLUSTER_NAME}"
```

### High-Frequency Monitoring
```yaml
receivers:
  postgresql:
    collection_interval: 10s
processors:
  batch:
    send_batch_size: 1000
    timeout: 5s
```

### Security Hardening
```yaml
receivers:
  postgresql:
    tls:
      insecure_skip_verify: false
      cert_file: /path/to/cert.pem
      key_file: /path/to/key.pem
```

Ready to go deeper? Check out the [Configuration Reference](CONFIGURATION.md) for complete customization options.