# NRDB Verification Guide

This guide explains how to verify that the Database Intelligence system is properly sending data to New Relic Database (NRDB).

## Consolidated Verification Script

The main verification tool is `verify-nrdb-comprehensive.sh` which consolidates all verification functionality:

### Usage

```bash
# Quick verification (default)
./scripts/verify-nrdb-comprehensive.sh

# Full system verification
./scripts/verify-nrdb-comprehensive.sh full

# Check specific module
./scripts/verify-nrdb-comprehensive.sh modules -m core-metrics

# Validate dashboard queries
./scripts/verify-nrdb-comprehensive.sh dashboard

# Continuous monitoring
./scripts/verify-nrdb-comprehensive.sh continuous

# With custom time range
./scripts/verify-nrdb-comprehensive.sh full -t 30  # Last 30 minutes
```

### Verification Modes

#### 1. Quick Mode (default)
- Fast check of overall data flow
- Shows total metrics count
- Lists active modules with metric counts
- Takes ~5 seconds

#### 2. Full Mode
Comprehensive verification including:
- Docker container status for all 11 modules
- Module metrics summary with health indicators
- Critical metrics validation (connections, queries, replication lag, etc.)
- MySQL entity status
- Alert/anomaly detection
- Data freshness check

#### 3. Modules Mode
- Deep dive into specific module metrics
- Shows sample metric names
- Displays key metric values
- Use `-m <module-name>` for specific module

#### 4. Dashboard Mode
- Validates common dashboard queries
- Tests entity queries, uptime, latency, etc.
- Shows pass/fail for each query
- Useful for dashboard troubleshooting

#### 5. Continuous Mode
- Real-time monitoring every 30 seconds
- Alerts on stale data (>5 minutes old)
- Shows active/inactive modules
- Press Ctrl+C to stop

## Other Verification Scripts

### 1. validate-all-modules.sh
Original comprehensive validation script that checks:
- Docker container status
- New Relic data flow
- Specific metric types
- Anomaly detection status

### 2. monitor-data-flow.sh
Advanced continuous monitoring with:
- Metric freshness tracking
- Entity health monitoring
- Dashboard widget performance
- Webhook/Slack alerting
- API status monitoring

### 3. validate-dashboard-queries.sh
Focused on dashboard query validation:
- Tests 6 critical NRQL queries
- Validates query syntax
- Checks for non-zero results

## Prerequisites

### Required Environment Variables
```bash
export NEW_RELIC_API_KEY="your-api-key"
export NEW_RELIC_ACCOUNT_ID="3630072"  # Optional, defaults to this
```

### Required Tools
- `curl` - For API calls
- `python3` - For JSON parsing
- `docker` - For container status checks
- `jq` - Optional, for verbose output

## Common NRQL Queries

### Check All Modules
```sql
SELECT count(*) FROM Metric 
WHERE module IS NOT NULL 
SINCE 5 minutes ago 
FACET module
```

### Check Specific Module
```sql
SELECT * FROM Metric 
WHERE module = 'core-metrics' 
SINCE 10 minutes ago 
LIMIT 100
```

### Check MySQL Metrics
```sql
SELECT average(mysql.connection.count), 
       average(mysql.queries),
       average(mysql.slow_queries)
FROM Metric 
WHERE module = 'core-metrics' 
SINCE 30 minutes ago 
TIMESERIES
```

### Check Entity Synthesis
```sql
SELECT uniqueCount(entity.name) 
FROM Metric 
WHERE entity.type = 'MYSQL_INSTANCE' 
SINCE 1 hour ago 
FACET entity.name
```

### Check for Anomalies
```sql
SELECT count(*) FROM Metric 
WHERE module = 'anomaly-detector' 
  AND metricName LIKE '%anomaly%' 
SINCE 30 minutes ago
```

### Check Data Freshness
```sql
SELECT latest(timestamp) FROM Metric 
WHERE module IS NOT NULL 
SINCE 1 hour ago 
FACET module
```

## Troubleshooting

### No Data in NRDB

1. **Check Container Status**
   ```bash
   docker ps | grep -E "(metrics|intelligence|profiler)"
   ```

2. **Check Container Logs**
   ```bash
   docker logs <container-name> 2>&1 | tail -50
   ```

3. **Verify API Key**
   ```bash
   echo $NEW_RELIC_API_KEY
   # Should show your key, not empty
   ```

4. **Check Network Connectivity**
   ```bash
   curl -s -X POST https://api.newrelic.com/graphql \
     -H "API-Key: $NEW_RELIC_API_KEY" \
     -H 'Content-Type: application/json' \
     -d '{"query": "{ actor { user { name } } }"}'
   ```

### Partial Data

1. **Check Federation Endpoints**
   ```bash
   ./scripts/verify-nrdb-comprehensive.sh full -v
   ```

2. **Verify Module Endpoints**
   ```bash
   docker exec <container> env | grep ENDPOINT
   ```

3. **Check OTTL Errors**
   ```bash
   docker logs <container> 2>&1 | grep -i "error.*ottl"
   ```

### Stale Data

1. **Check Batch Settings**
   - Look for batch timeout in collector.yaml
   - Default should be 10s or less

2. **Check Memory Limits**
   ```bash
   docker stats --no-stream | grep -E "(metrics|intelligence)"
   ```

3. **Check Export Queue**
   ```bash
   docker logs <container> 2>&1 | grep -i "queue"
   ```

## Best Practices

1. **Regular Monitoring**
   - Run quick verification every hour
   - Run full verification daily
   - Use continuous mode during troubleshooting

2. **Alert Setup**
   - Configure monitor-data-flow.sh with webhooks
   - Set up New Relic alerts for missing data

3. **Dashboard Validation**
   - Run dashboard validation after any query changes
   - Test queries in Query Builder first

4. **Module Health**
   - Each module should send >100 metrics/minute
   - Data should be fresh (<1 minute old)
   - No critical alerts in normal operation

## Integration with CI/CD

```yaml
# Example GitHub Actions step
- name: Verify NRDB Data
  env:
    NEW_RELIC_API_KEY: ${{ secrets.NEW_RELIC_API_KEY }}
  run: |
    ./scripts/verify-nrdb-comprehensive.sh full
    if [ $? -ne 0 ]; then
      echo "NRDB verification failed"
      exit 1
    fi
```

## Summary

The consolidated `verify-nrdb-comprehensive.sh` script provides all necessary verification functionality in one tool. Use:
- **Quick mode** for rapid checks
- **Full mode** for comprehensive verification
- **Continuous mode** for real-time monitoring
- **Dashboard mode** for query validation
- **Modules mode** for deep dives

Combined with proper environment setup and regular monitoring, this ensures reliable data flow from Database Intelligence to New Relic.