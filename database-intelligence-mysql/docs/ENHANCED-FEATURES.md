# Enhanced OpenTelemetry Features Guide

This guide explains the advanced OpenTelemetry-native optimizations available in the enhanced master configuration (v4.0.0).

## Quick Start

To enable enhanced features:

```bash
# Set deployment mode
export DEPLOYMENT_MODE=enhanced

# Update docker-compose.yml to use the enhanced config
sed -i 's|master.yaml|master-enhanced.yaml|g' docker-compose.yml

# Start with enhanced features
docker-compose up -d
```

## Feature Overview

### 1. Cross-Signal Correlation

Automatically correlate metrics, traces, and logs:

- **Trace Context**: Send MySQL queries with trace context for end-to-end visibility
- **Log-to-Metrics**: Convert slow query logs into metrics
- **Span Metrics**: Generate RED metrics from spans with exemplars

```yaml
# Example: Send traced MySQL query
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "resourceSpans": [{
      "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "api-service"}}]},
      "scopeSpans": [{
        "spans": [{
          "name": "mysql.query",
          "attributes": [
            {"key": "db.statement", "value": {"stringValue": "SELECT * FROM orders"}},
            {"key": "db.schema", "value": {"stringValue": "orders"}}
          ]
        }]
      }]
    }]
  }'
```

### 2. Exemplars for Debugging

Link metrics to specific trace examples:

```nrql
# Find slow queries with trace examples
SELECT histogram(duration, 10, 20) 
FROM Span 
WHERE name = 'mysql.query' 
FACET db.statement 
WITH EXEMPLARS
```

### 3. Edge Processing Intelligence

Reduce data volume with smart aggregation:

- **Local Aggregation**: Combine similar queries by hash
- **Delta Conversion**: Convert counters to rates at the edge
- **Cardinality Control**: Limit high-cardinality dimensions

### 4. Circuit Breaker Protection

Prevent cascade failures:

```yaml
# Circuit breaker activates when:
# - 5 consecutive export failures
# - Falls back to secondary endpoint
# - Auto-recovers after 30s
```

### 5. Persistent Queue with Overflow

Never lose data during outages:

- **File-backed Queue**: Survives collector restarts
- **Overflow Protection**: Automatic disk space management
- **Priority Queuing**: Critical metrics sent first

### 6. Multi-Tenant Routing

Route data based on schema criticality:

```yaml
# Critical schemas (orders, payments, customers):
# - No compression, low latency
# - Dedicated export pipeline
# - Higher retry attempts

# Standard schemas:
# - Gzip compression
# - Standard batching
# - Normal priority

# Batch schemas (analytics, reporting):
# - Zstd compression
# - Large batches
# - Lower priority
```

### 7. Data Quality Scoring

Automatic confidence scoring:

```nrql
# Filter by data quality
SELECT * 
FROM Metric 
WHERE metricName = 'mysql.intelligence.comprehensive' 
AND attributes['data.confidence'] > 80
```

Quality factors:
- Historical data availability
- Wait profile completeness
- Performance metrics presence
- Pattern detection confidence

### 8. Synthetic Monitoring

Baseline establishment with canary queries:

```sql
-- Canary queries run every 60s:
-- 1. Basic connectivity check
-- 2. Lock acquisition test
-- 3. Metadata query performance
```

Monitor canary health:
```nrql
SELECT average(mysql.canary.latency) 
TIMESERIES 
SINCE 1 hour ago
```

### 9. Automated Response Hooks

Trigger actions based on conditions:

```yaml
# Automatic actions flagged:
# - Kill long-running blocking queries
# - Index creation recommendations
# - Scale-up suggestions

# Integration with automation tools:
SELECT latest(attributes['action.required']) as action,
       latest(query_digest) as target
FROM Metric 
WHERE attributes['action.required'] IS NOT NULL
```

### 10. Progressive Rollout

Test features gradually:

```bash
# Start with 10% of queries
export ROLLOUT_PERCENTAGE=10

# Monitor impact
# Gradually increase to 100%
export ROLLOUT_PERCENTAGE=50
export ROLLOUT_PERCENTAGE=100
```

## Configuration Reference

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DEPLOYMENT_MODE` | Set to `enhanced` for all features | `advanced` |
| `ML_FEATURES_ENABLED` | Enable ML-based scoring | `true` |
| `WAIT_PROFILE_ENABLED` | Enable wait profiling | `true` |
| `ANOMALY_DETECTION_ENABLED` | Enable anomaly detection | `true` |
| `ROLLOUT_PERCENTAGE` | Progressive rollout percentage | `10` |
| `CIRCUIT_FAILURE_THRESHOLD` | Failures before circuit opens | `5` |
| `CIRCUIT_RECOVERY_TIMEOUT` | Recovery wait time | `30s` |

### Performance Impact

Enhanced features add approximately:
- **CPU**: +15-20% overhead
- **Memory**: +200-300MB
- **Network**: Varies based on trace/log volume
- **Storage**: 100MB-1GB for persistent queues

### Best Practices

1. **Start Small**: Begin with ROLLOUT_PERCENTAGE=10
2. **Monitor Resources**: Watch collector metrics closely
3. **Tune Batching**: Adjust batch sizes for your workload
4. **Configure Tenants**: Map schemas to appropriate priority
5. **Set Alerts**: Monitor circuit breaker state

### Troubleshooting

Check enhanced features status:

```bash
# View circuit breaker state
curl http://localhost:55679/debug/tracez

# Check persistent queue
ls -la /tmp/otel-storage/

# Verify trace reception
curl http://localhost:13133/health

# Review data quality scores
docker logs otel-collector | grep "data.confidence"
```

## NRQL Queries for Enhanced Features

### ML Anomaly Detection
```nrql
SELECT count(*) as 'Anomalies Detected',
       average(attributes['ml.anomaly_score']) as 'Avg Score',
       max(attributes['ml.baseline_deviation']) as 'Max Deviation'
FROM Metric 
WHERE attributes['ml.is_anomaly'] = true 
FACET attributes['ml.workload_type']
TIMESERIES
```

### Business Impact Dashboard
```nrql
SELECT sum(attributes['business.revenue_impact']) as 'Revenue at Risk',
       sum(attributes['cost.compute_impact']) as 'Compute Cost',
       filter(count(*), WHERE attributes['business.sla_violated'] = true) as 'SLA Violations'
FROM Metric 
FACET attributes['business_criticality']
SINCE 1 hour ago
```

### Tenant Performance
```nrql
SELECT average(value) as 'Intelligence Score'
FROM Metric 
WHERE metricName = 'mysql.intelligence.comprehensive'
FACET attributes['db_schema'], attributes['X-Tenant-Priority']
TIMESERIES
```

### Data Quality Monitoring
```nrql
SELECT histogram(attributes['data.confidence'], 10, 10) as 'Confidence Distribution',
       filter(count(*), WHERE attributes['data.quality'] = 'low') as 'Low Quality Records'
FROM Metric
SINCE 1 hour ago
```

## Migration Guide

### From Advanced to Enhanced

1. **Backup Current State**
   ```bash
   cp config/collector/master.yaml config/collector/master.yaml.backup
   ```

2. **Update Configuration**
   ```bash
   # Use enhanced config
   export DEPLOYMENT_MODE=enhanced
   
   # Update docker-compose
   sed -i 's|master.yaml|master-enhanced.yaml|g' docker-compose.yml
   ```

3. **Gradual Rollout**
   ```bash
   # Start with 10%
   export ROLLOUT_PERCENTAGE=10
   docker-compose up -d
   
   # Monitor for 1 hour
   # If stable, increase to 50%
   export ROLLOUT_PERCENTAGE=50
   docker-compose up -d
   
   # After validation, full rollout
   export ROLLOUT_PERCENTAGE=100
   docker-compose up -d
   ```

4. **Verify Features**
   ```bash
   # Check logs for new features
   docker logs otel-collector | grep -E "(circuit|tenant|exemplar|canary)"
   ```

### Rollback Procedure

If issues arise:

```bash
# Revert to advanced mode
export DEPLOYMENT_MODE=advanced

# Restore original config
sed -i 's|master-enhanced.yaml|master.yaml|g' docker-compose.yml

# Restart
docker-compose up -d
```

## Next Steps

1. **Enable Tracing**: Configure your applications to send traces
2. **Setup Log Collection**: Point MySQL slow query logs to the collector
3. **Configure Automation**: Integrate action.required attributes with your automation tools
4. **Customize Business Rules**: Adjust revenue impact calculations for your business
5. **Monitor Circuit Breaker**: Set up alerts for circuit state changes

For questions or issues, refer to the main documentation or submit a GitHub issue.