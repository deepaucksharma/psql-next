# Operational Procedures

## Daily Operations

### Health Checks
```bash
# Test all connections
./operate/test-connection.sh

# Validate metrics flow
./operate/validate-metrics.sh

# Run diagnostics
./operate/diagnose.sh
```

### Monitoring
- Check New Relic dashboards
- Review active advisories
- Monitor anomaly alerts
- Track replication lag

## Common Tasks

### Generate Workload
```bash
# Start continuous workload
./operate/generate-workload.sh

# Stop workload
pkill -f generate-workload.sh
```

### View Logs
```bash
# Collector logs
docker compose logs -f otel-collector

# MySQL logs
docker compose logs -f mysql-primary
docker compose logs -f mysql-replica

# All services
docker compose logs -f
```

### Restart Services
```bash
# Restart collector only
docker compose restart otel-collector

# Restart all services
docker compose restart

# Full restart
docker compose down && docker compose up -d
```

## Maintenance

### Update Configuration
1. Edit `config/collector/master.yaml`
2. Restart collector: `docker compose restart otel-collector`
3. Verify with: `./operate/validate-config.sh`

### Scale Testing
```bash
# Run full test suite
./operate/full-test.sh

# Generate heavy load
for i in {1..5}; do
  ./operate/generate-workload.sh &
done
```

### Backup Procedures
```bash
# Backup MySQL data
docker compose exec mysql-primary mysqldump -u root -p ecommerce > backup.sql

# Backup configurations
tar -czf configs-backup.tar.gz config/
```

## Performance Tuning

### Reduce Collection Interval
```bash
export MYSQL_COLLECTION_INTERVAL=10s
export SQL_INTELLIGENCE_INTERVAL=30s
./deploy/deploy.sh
```

### Adjust Memory Limits
```bash
export MEMORY_LIMIT_PERCENT=90
export GOMEMLIMIT=4GiB
docker compose up -d
```

### Switch Deployment Modes
```bash
# Switch to standard mode for production
export DEPLOYMENT_MODE=standard
docker compose up -d

# Switch to minimal for development
export DEPLOYMENT_MODE=minimal
docker compose up -d
```

## Monitoring Endpoints

### Local Metrics
- **Health Check**: http://localhost:13133/
  - Returns "Server available" when healthy
  
- **Internal Metrics**: http://localhost:8888/metrics
  - OpenTelemetry collector internal metrics
  
- **Prometheus Format**: http://localhost:8889/metrics
  - MySQL metrics in Prometheus format
  
- **Debug zPages**: http://localhost:55679/
  - Pipeline status: /debug/pipelinez
  - Service status: /debug/servicez
  
- **Performance Profiling**: http://localhost:1777/debug/pprof/
  - CPU profile
  - Memory profile
  - Goroutine dump

### New Relic Queries

**Basic Metrics**:
```sql
FROM Metric 
SELECT * 
WHERE instrumentation.provider = 'opentelemetry' 
SINCE 5 minutes ago
```

**MySQL Intelligence**:
```sql
FROM Metric 
SELECT * 
WHERE metricName = 'mysql.intelligence.comprehensive' 
SINCE 10 minutes ago
```

**Wait Analysis**:
```sql
FROM Metric 
SELECT sum(value) as 'Total Wait Time' 
WHERE metricName = 'mysql.query.wait_profile' 
FACET attributes['query_hash'] 
SINCE 1 hour ago
```

**Anomalies**:
```sql
FROM Metric 
SELECT * 
WHERE attributes['anomaly.detected'] = true 
SINCE 30 minutes ago
```

## Alert Response

### High Connection Usage
1. Check current connections: `SHOW PROCESSLIST`
2. Identify long-running queries
3. Consider increasing max_connections
4. Scale horizontally if needed

### Replication Lag
1. Check replica status: `SHOW SLAVE STATUS\G`
2. Identify blocking queries on primary
3. Optimize slow queries
4. Consider read replica scaling

### Critical Anomalies
1. Review anomaly details in dashboard
2. Check advisor recommendations
3. Implement suggested fixes
4. Monitor for resolution

## Capacity Planning

### Metrics to Monitor
- Connection usage percentage
- Query rate trends
- Buffer pool utilization
- Disk I/O patterns
- Replication lag trends

### Scaling Decisions
- **Vertical**: When CPU/Memory consistently > 80%
- **Horizontal**: When read traffic dominates
- **Optimization**: When specific queries cause issues