# MySQL Wait-Based Monitoring - Troubleshooting Guide

## Common Issues and Solutions

### 1. Docker Sign-in Required

**Error**: "Sign-in enforcement is enabled. Open Docker Desktop..."

**Solution**:
1. Open Docker Desktop application
2. Sign in with your organization account
3. Verify sign-in: `docker pull hello-world`

### 2. New Relic License Key Missing

**Error**: Metrics not appearing in New Relic

**Solution**:
1. Get your New Relic Ingest License Key from: https://one.newrelic.com/api-keys
2. Update .env file: `NEW_RELIC_LICENSE_KEY=your_actual_key`
3. Restart gateway: `docker-compose restart otel-gateway`

### 3. MySQL Connection Failed

**Error**: "Access denied for user 'otel_monitor'"

**Possible causes**:
- Init scripts didn't run
- MySQL not fully started

**Solution**:
```bash
# Check MySQL logs
docker logs mysql-primary

# Manually create monitoring user
docker exec mysql-primary mysql -u root -p${MYSQL_ROOT_PASSWORD} -e "
CREATE USER IF NOT EXISTS 'otel_monitor'@'%' IDENTIFIED BY 'otelmonitorpass';
GRANT SELECT, PROCESS, REPLICATION CLIENT ON *.* TO 'otel_monitor'@'%';
FLUSH PRIVILEGES;"
```

### 4. No Metrics Appearing

**Diagnostic steps**:
```bash
# 1. Check collector status
docker logs otel-collector-edge | tail -20

# 2. Verify Performance Schema
docker exec mysql-primary mysql -u otel_monitor -potelmonitorpass -e "
SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest;"

# 3. Check metrics endpoint
curl -s http://localhost:8888/metrics | grep receiver_accepted_metric_points

# 4. Verify gateway connection
docker logs otel-gateway | grep "connection refused"
```

### 5. High Memory Usage

**Symptoms**: Collector using >500MB memory

**Solutions**:
1. Adjust memory limits in configs
2. Increase collection intervals
3. Enable sampling for non-critical queries
4. Use light-load configuration

### 6. Missing Advisories

**Check advisory processing**:
```bash
# View gateway logs
docker logs otel-gateway | grep -i "advisor"

# Check metrics
curl -s http://localhost:9091/metrics | grep advisor_type | wc -l
```

## Quick Diagnostic Commands

```bash
# Full system check
./scripts/validate-everything.sh

# Monitor real-time waits
./scripts/monitor-waits.sh waits

# Check all services
docker-compose ps

# View all logs
docker-compose logs -f

# Restart everything
docker-compose down && docker-compose up -d
```

## Performance Tuning

For different environments, use appropriate configurations:
- Development: `config/tuning/light-load.yaml`
- Production: `config/tuning/heavy-load.yaml`
- Critical: `config/tuning/critical-system.yaml`

## Getting Help

1. Check operational runbooks: `docs/OPERATIONAL_RUNBOOKS.md`
2. Review architecture: `docs/WAIT_BASED_MONITORING_GUIDE.md`
3. Run validation: `./scripts/validate-everything.sh`
