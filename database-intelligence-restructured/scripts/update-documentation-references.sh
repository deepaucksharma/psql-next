#!/bin/bash
# Script to update all documentation cross-references

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Updating Documentation Cross-References ===${NC}"

# 1. Update guide references
echo -e "${YELLOW}Updating guide references...${NC}"

# Fix references to Redis (which we removed) with references to other databases
find docs -name "*.md" -type f | while read file; do
    # Remove Redis references
    sed -i '' '/REDIS_MAXIMUM_GUIDE/d' "$file" 2>/dev/null || true
    sed -i '' '/redis-maximum-extraction\.yaml/d' "$file" 2>/dev/null || true
    
    # Update script paths
    sed -i '' 's|development/scripts/|scripts/|g' "$file" 2>/dev/null || true
    sed -i '' 's|docs/archive/scripts/|scripts/archive/|g' "$file" 2>/dev/null || true
    
    # Update config paths
    sed -i '' 's|configs/config-only-mode\.yaml|configs/postgresql-maximum-extraction.yaml|g' "$file" 2>/dev/null || true
    sed -i '' 's|configs/custom-mode\.yaml|configs/profiles/enterprise.yaml|g' "$file" 2>/dev/null || true
done

# 2. Update main README with correct links
echo -e "${YELLOW}Updating README links...${NC}"

# Ensure README has correct guide links
cat > /tmp/readme_guides.tmp << 'EOF'
### Essential Guides
- [**Quick Start Guide**](docs/guides/QUICK_START.md) - Get running in 5 minutes
- [**Configuration Guide**](docs/guides/CONFIGURATION.md) - All configuration options
- [**Deployment Guide**](docs/guides/DEPLOYMENT.md) - Production deployment
- [**Troubleshooting Guide**](docs/guides/TROUBLESHOOTING.md) - Common issues and solutions

### Database-Specific Guides
- [**PostgreSQL Maximum Extraction**](docs/guides/CONFIG_ONLY_MAXIMUM_GUIDE.md) - PostgreSQL comprehensive guide
- [**MySQL Maximum Extraction**](docs/guides/MYSQL_MAXIMUM_GUIDE.md) - MySQL monitoring guide
- [**MongoDB Maximum Extraction**](docs/guides/MONGODB_MAXIMUM_GUIDE.md) - MongoDB monitoring guide
- [**MSSQL Maximum Extraction**](docs/guides/MSSQL_MAXIMUM_GUIDE.md) - SQL Server monitoring guide
- [**Oracle Maximum Extraction**](docs/guides/ORACLE_MAXIMUM_GUIDE.md) - Oracle monitoring guide
EOF

# 3. Create a comprehensive deployment guide
echo -e "${YELLOW}Creating unified deployment guide...${NC}"
cat > docs/guides/DEPLOYMENT.md << 'EOF'
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
EOF

# 4. Update TROUBLESHOOTING.md with database-specific sections
echo -e "${YELLOW}Updating troubleshooting guide...${NC}"
cat > docs/guides/TROUBLESHOOTING.md << 'EOF'
# Troubleshooting Guide

This guide helps you resolve common issues with Database Intelligence.

## Table of Contents
- [General Issues](#general-issues)
- [PostgreSQL Issues](#postgresql-issues)
- [MySQL Issues](#mysql-issues)
- [MongoDB Issues](#mongodb-issues)
- [MSSQL Issues](#mssql-issues)
- [Oracle Issues](#oracle-issues)
- [Performance Issues](#performance-issues)
- [New Relic Integration](#new-relic-integration)

## General Issues

### No Metrics Appearing

1. **Check collector status**:
   ```bash
   docker logs otel-collector
   ```

2. **Verify configuration**:
   ```bash
   ./scripts/validate-config.sh configs/postgresql-maximum-extraction.yaml
   ```

3. **Test connectivity**:
   ```bash
   ./scripts/test-database-config.sh postgresql 60
   ```

4. **Check New Relic**:
   ```sql
   SELECT count(*) FROM Metric 
   WHERE collector.name = 'database-intelligence-*' 
   SINCE 5 minutes ago
   ```

### Authentication Errors

1. **Verify credentials**:
   ```bash
   # Test database connection directly
   psql -h localhost -U postgres -d postgres
   mysql -h localhost -u root -p
   mongosh mongodb://localhost:27017
   sqlcmd -S localhost -U sa -P password
   sqlplus user/pass@localhost:1521/ORCLPDB1
   ```

2. **Check environment variables**:
   ```bash
   env | grep -E "(POSTGRES|MYSQL|MONGODB|MSSQL|ORACLE)"
   ```

### High Memory Usage

1. **Adjust memory limits**:
   ```yaml
   processors:
     memory_limiter:
       limit_mib: 512  # Reduce limit
       spike_limit_mib: 128
   ```

2. **Reduce cardinality**:
   ```yaml
   processors:
     filter/reduce_cardinality:
       metrics:
         exclude:
           metric_names:
             - "*.query.*"  # Exclude per-query metrics
   ```

## PostgreSQL Issues

### pg_stat_statements Not Available

```sql
-- Enable extension
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- Verify it's loaded
SELECT * FROM pg_extension WHERE extname = 'pg_stat_statements';

-- Check postgresql.conf
SHOW shared_preload_libraries;
```

### Connection Pool Exhausted

```yaml
# Reduce collection frequency
sqlquery/ash:
  collection_interval: 10s  # Increase from 1s
```

### Replication Metrics Missing

```sql
-- Check if replica
SELECT pg_is_in_recovery();

-- Verify replication slots
SELECT * FROM pg_replication_slots;
```

## MySQL Issues

### Performance Schema Disabled

```sql
-- Check if enabled
SHOW VARIABLES LIKE 'performance_schema';

-- Enable in my.cnf
[mysqld]
performance_schema=ON

-- Restart MySQL
systemctl restart mysql
```

### Access Denied Errors

```sql
-- Grant required permissions
GRANT PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'otel_monitor'@'%';
FLUSH PRIVILEGES;
```

### Slow Query Metrics Missing

```sql
-- Check slow query log
SHOW VARIABLES LIKE 'slow_query_log%';

-- Enable if needed
SET GLOBAL slow_query_log = 'ON';
SET GLOBAL long_query_time = 1;
```

## MongoDB Issues

### Authentication Failed

```javascript
// Verify user exists
use admin
db.getUsers()

// Create monitoring user
db.createUser({
  user: "otel_monitor",
  pwd: "password",
  roles: [
    { role: "clusterMonitor", db: "admin" },
    { role: "read", db: "local" }
  ]
})
```

### currentOp Permission Denied

```javascript
// Grant required role
db.grantRolesToUser("otel_monitor", [
  { role: "clusterMonitor", db: "admin" }
])
```

### Atlas Metrics Not Working

1. Verify API keys are set
2. Check project name matches exactly
3. Ensure IP whitelist includes collector

## MSSQL Issues

### Connection Timeout

```yaml
# Increase timeout in connection string
datasource: "sqlserver://user:pass@host:1433?connection+timeout=30"
```

### Permission Errors

```sql
-- Grant required permissions
GRANT VIEW SERVER STATE TO otel_monitor;
GRANT VIEW ANY DEFINITION TO otel_monitor;

-- For each database
USE [YourDatabase];
GRANT VIEW DATABASE STATE TO otel_monitor;
```

### Always On AG Metrics Missing

```sql
-- Check if AG is configured
SELECT * FROM sys.availability_groups;

-- Verify permissions
SELECT * FROM fn_my_permissions(NULL, 'SERVER')
WHERE permission_name LIKE '%VIEW%';
```

## Oracle Issues

### ORA-12154: TNS Error

```yaml
# Use full connection string
datasource: "oracle://user:pass@(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=localhost)(PORT=1521))(CONNECT_DATA=(SERVICE_NAME=ORCLPDB1)))"
```

### Missing V$ Views

```sql
-- Grant access
GRANT SELECT ANY DICTIONARY TO otel_monitor;
GRANT SELECT ON V_$SESSION TO otel_monitor;
GRANT SELECT ON V_$SYSSTAT TO otel_monitor;
```

### Character Set Issues

```bash
# Set NLS_LANG
export NLS_LANG=AMERICAN_AMERICA.AL32UTF8
```

## Performance Issues

### Collector Using Too Much CPU

1. **Reduce collection frequency**:
   ```yaml
   collection_interval: 60s  # Increase intervals
   ```

2. **Enable sampling**:
   ```yaml
   processors:
     probabilistic_sampler:
       sampling_percentage: 10
   ```

### Metrics Delayed

1. **Adjust batch settings**:
   ```yaml
   processors:
     batch:
       timeout: 5s  # Reduce timeout
       send_batch_size: 500  # Smaller batches
   ```

### Network Timeouts

```yaml
exporters:
  otlp/newrelic:
    timeout: 60s  # Increase timeout
    retry_on_failure:
      max_elapsed_time: 600s
```

## New Relic Integration

### Invalid License Key

```bash
# Verify key format
echo $NEW_RELIC_LICENSE_KEY | wc -c  # Should be 40 characters

# Test with curl
curl -X POST https://metric-api.newrelic.com/metric/v1 \
  -H "Api-Key: $NEW_RELIC_LICENSE_KEY" \
  -H "Content-Type: application/json" \
  -d '[{"metrics":[]}]'
```

### Wrong Region

```yaml
# For EU region
exporters:
  otlp/newrelic:
    endpoint: otlp.eu01.nr-data.net:4317
```

### Metrics Not Appearing

1. **Check account ID**:
   ```sql
   SELECT count(*) FROM Metric 
   WHERE true 
   SINCE 1 hour ago
   ```

2. **Verify metric names**:
   ```sql
   SELECT uniques(metricName) FROM Metric 
   WHERE metricName LIKE 'postgresql%' 
   SINCE 1 hour ago
   ```

## Debug Mode

Enable debug logging:

```yaml
service:
  telemetry:
    logs:
      level: debug
      
exporters:
  debug:
    verbosity: detailed
```

## Getting Help

1. **Check logs**: Always start with collector logs
2. **Validate config**: Use provided validation scripts
3. **Test connectivity**: Ensure database is reachable
4. **Review permissions**: Database user needs specific grants
5. **Open an issue**: https://github.com/newrelic/database-intelligence/issues

## Common Error Messages

| Error | Cause | Solution |
|-------|-------|----------|
| `connection refused` | Database not running or wrong host/port | Verify database is running and accessible |
| `authentication failed` | Wrong credentials | Check username/password |
| `permission denied` | Missing grants | Grant required permissions |
| `context deadline exceeded` | Timeout | Increase timeout values |
| `no such host` | DNS resolution failed | Verify hostname |
| `SSL/TLS required` | Security enforcement | Enable TLS in configuration |
| `out of memory` | Memory limit exceeded | Increase memory_limiter |
EOF

echo -e "${GREEN}✓ Documentation references updated${NC}"
echo -e "${GREEN}✓ Created unified deployment guide${NC}"
echo -e "${GREEN}✓ Updated troubleshooting guide${NC}"