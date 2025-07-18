# MySQL Wait-Based Monitoring - Quick Start Guide

## Prerequisites Checklist

- [ ] Docker Desktop installed and signed in
- [ ] New Relic License Key obtained
- [ ] Ports available: 3306, 4317, 8888, 9091
- [ ] 4GB RAM available for containers

## Step 1: Environment Setup

```bash
# Clone and navigate to project
cd database-intelligence-mysql

# Copy environment template
cp .env.example .env

# Edit .env and set your New Relic License Key
# NEW_RELIC_LICENSE_KEY=your_actual_license_key_here
vim .env
```

## Step 2: Validate Everything

```bash
# Run comprehensive validation
./scripts/validate-everything.sh

# Expected output:
# ✓ Docker is installed
# ✓ All configuration files exist
# ✓ SQL queries validated
# ✓ Ports are available
```

## Step 3: Start Services

```bash
# Start all services
docker-compose up -d

# Wait for services to be healthy (about 30 seconds)
docker-compose ps

# Check logs if needed
docker-compose logs -f mysql-primary
docker-compose logs -f otel-collector-edge
```

## Step 4: Verify MySQL Setup

```bash
# Check Performance Schema is enabled
docker exec mysql-primary mysql -u root -prootpassword -e "
SELECT @@performance_schema;
SELECT COUNT(*) FROM performance_schema.setup_instruments 
WHERE NAME LIKE 'wait/%' AND ENABLED = 'YES';"

# Should show:
# @@performance_schema = 1
# COUNT(*) > 300
```

## Step 5: Generate Test Workload

```bash
# Create test data
docker exec mysql-primary mysql -u root -prootpassword production -e "
CALL generate_workload(100, 'mixed');"

# Monitor in real-time
./scripts/monitor-waits.sh waits
```

## Step 6: Check Metrics Flow

```bash
# 1. Edge Collector Metrics
curl -s http://localhost:8888/metrics | grep mysql_query_wait_profile

# 2. Gateway Metrics  
curl -s http://localhost:9091/metrics | grep mysql_wait

# 3. Check for Advisories
curl -s http://localhost:9091/metrics | grep advisor_type

# 4. Prometheus Exporter
curl -s http://localhost:9104/metrics | grep mysql_global_status
```

## Step 7: View in New Relic

1. Log into New Relic
2. Navigate to **Metrics Explorer**
3. Search for `mysql.query.wait_profile`
4. Create dashboard from template

## Common Commands

```bash
# Stop all services
docker-compose down

# Restart a specific service
docker-compose restart otel-gateway

# View real-time logs
docker-compose logs -f --tail=50

# Check collector health
curl http://localhost:13133/health

# Run performance test
docker run --rm --network mysql-network \
  mysql-loadgen --pattern=mixed --duration=5m

# Export metrics to file
curl -s http://localhost:9091/metrics > metrics-snapshot.txt
```

## Troubleshooting Quick Fixes

### No metrics appearing
```bash
# Check collector is receiving data
docker logs otel-collector-edge | grep "MetricsReceived"

# Verify MySQL connection
docker exec otel-collector-edge \
  mysql -h mysql-primary -u otel_monitor -potelmonitorpass -e "SELECT 1"
```

### High memory usage
```bash
# Switch to light-load configuration
docker-compose down
export CONFIG_PROFILE=light-load
docker-compose up -d
```

### Blocking queries detected
```bash
# Find and kill blocking session
docker exec mysql-primary mysql -u root -prootpassword -e "
SELECT * FROM information_schema.innodb_trx WHERE trx_state = 'LOCK WAIT';
KILL <blocking_thread_id>;"
```

## Performance Validation

```bash
# Check overhead on MySQL
docker stats mysql-primary --no-stream

# Expected: <5% additional CPU usage

# Check collector resource usage  
docker stats otel-collector-edge --no-stream

# Expected: <3% CPU, <200MB memory
```

## Next Steps

1. **Review Dashboards**: Import provided JSON dashboards to New Relic
2. **Set Up Alerts**: Configure alerts based on wait thresholds
3. **Tune Collection**: Adjust intervals based on your workload
4. **Enable HA**: Deploy gateway-ha configuration for production

## Quick Reference

| Component | Port | Health Check URL | Logs Command |
|-----------|------|------------------|--------------|
| MySQL Primary | 3306 | - | `docker logs mysql-primary` |
| Edge Collector | 8888 | http://localhost:13133/health | `docker logs otel-collector-edge` |
| Gateway | 4317 | http://localhost:13134/health | `docker logs otel-gateway` |
| Prometheus | 9091 | http://localhost:9091/metrics | - |
| MySQL Exporter | 9104 | http://localhost:9104/metrics | `docker logs mysql-exporter-primary` |

## Support

- Configuration Issues: Check `./TROUBLESHOOTING.md`
- SQL Query Details: See `./docs/DEEP_WAIT_ANALYSIS.md`
- Operational Procedures: Review `./docs/OPERATIONAL_RUNBOOKS.md`

---
Remember: The goal is actionable insights with <1% overhead!