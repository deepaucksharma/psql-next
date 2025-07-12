# New Relic Database Intelligence Dashboards

This directory contains comprehensive New Relic dashboards for monitoring databases using both Config-Only and Custom/Enhanced modes.

## Dashboard Overview

### 1. Unified Parallel Dashboard (`unified-parallel-dashboard.json`)
**Recommended for parallel deployments**

A single comprehensive dashboard that monitors both deployment modes simultaneously:
- **Executive Overview**: Deployment status, health scores, active sessions
- **PostgreSQL - Both Modes**: Connections, transactions, wait events, query intelligence
- **MySQL - Both Modes**: Thread status, query performance, buffer pools, locks
- **Enhanced Features**: ASH heatmaps, query plans, intelligent processing (Custom mode only)
- **Mode Comparison**: Metric coverage, performance impact, intelligence value
- **System Resources**: CPU, memory, and network usage by mode
- **Alerting Recommendations**: Suggested alerts and current conditions

### 2. Config-Only Dashboard (`config-only-dashboard.json`)
**For standard OpenTelemetry deployments**

Focuses on metrics from standard OTel receivers:
- Database health score and overview
- PostgreSQL standard metrics (connections, checkpoints, replication)
- MySQL standard metrics (threads, queries, buffer pools)
- System resource utilization

### 3. Custom Mode Dashboard (`custom-mode-dashboard.json`)
**For enhanced feature deployments**

Showcases advanced monitoring capabilities:
- Active Session History (ASH) with wait event analysis
- Query intelligence with plan extraction
- Enhanced SQL metrics with detailed statistics
- Intelligent processing (circuit breaker, cost control, adaptive sampling)
- Kernel and process-level metrics

### 4. Comparison Dashboard (`comparison-dashboard.json`)
**For evaluating modes side-by-side**

Direct comparison between modes:
- Performance comparison
- Feature availability matrix
- Query intelligence capabilities
- Resource usage and cost analysis
- Intelligence value metrics

## Deployment

### Prerequisites
```bash
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"
```

### Deploy Dashboards

#### Option 1: Using the deployment script (Recommended)
```bash
# Deploy all services and dashboards
./scripts/deploy-parallel-modes.sh

# Just deploy dashboards
./scripts/migrate-dashboard.sh deploy dashboards/newrelic/unified-parallel-dashboard.json
```

#### Option 2: Using New Relic CLI
```bash
# Install New Relic CLI
curl -Ls https://download.newrelic.com/install/newrelic-cli/scripts/install.sh | bash

# Deploy dashboard
newrelic entity dashboard create --accountId $NEW_RELIC_ACCOUNT_ID --dashboard file://unified-parallel-dashboard.json
```

#### Option 3: Using GraphQL API
```bash
# Deploy via API
curl -X POST https://api.newrelic.com/graphql \
  -H "Api-Key: $NEW_RELIC_API_KEY" \
  -H "Content-Type: application/json" \
  -d @- <<EOF
{
  "query": "mutation { dashboardCreate(accountId: $NEW_RELIC_ACCOUNT_ID, dashboard: $(cat unified-parallel-dashboard.json)) { entityResult { guid } } }"
}
EOF
```

## Key Metrics by Mode

### Both Modes
- `postgresql.backends` - Active connections
- `postgresql.commits/rollbacks` - Transaction rates
- `postgresql.database.size` - Database sizes
- `mysql.threads` - MySQL connections
- `mysql.query.count` - Query rates
- `system.cpu/memory.utilization` - Resource usage

### Custom Mode Only
- `db.ash.active_sessions` - Real-time session states
- `db.ash.wait_events` - Wait event analysis
- `db.ash.blocked_sessions` - Blocking detection
- `db.ash.long_running_queries` - Slow query detection
- `postgres.slow_queries.*` - Enhanced query metrics with plans
- `adaptive_sampling_rate` - Intelligent sampling
- `circuit_breaker_state` - Protection status
- `cost_control_datapoints_*` - Cost optimization

## Dashboard Variables

All dashboards support these variables:
- **mode**: Filter by deployment mode (All/Config-Only/Custom)
- **database**: Filter by database name
- **timeRange**: Adjust time window (5min to 7 days)

## Alerting Recommendations

### Critical Alerts (Both Modes)
```nrql
-- High Connection Count
SELECT latest(postgresql.backends) FROM Metric 
WHERE deployment.mode IN ('config-only', 'custom') 
FACET db.name

-- Deadlocks
SELECT sum(postgresql.deadlocks) FROM Metric 
WHERE deployment.mode IN ('config-only', 'custom')

-- Replication Lag
SELECT max(postgresql.wal.lag) FROM Metric 
WHERE deployment.mode IN ('config-only', 'custom')
```

### Enhanced Alerts (Custom Mode)
```nrql
-- Blocked Sessions
SELECT sum(db.ash.blocked_sessions) FROM Metric 
WHERE deployment.mode = 'custom'

-- Long Running Queries
SELECT sum(db.ash.long_running_queries) FROM Metric 
WHERE deployment.mode = 'custom'

-- Circuit Breaker Trips
SELECT sum(circuit_breaker_trips) FROM Metric 
WHERE deployment.mode = 'custom'
```

## Best Practices

1. **Start with Unified Dashboard**: Get a complete view of both modes
2. **Use Mode Filter**: Focus on specific deployment mode when troubleshooting
3. **Set Time Range Appropriately**: 
   - 5-30 minutes for real-time monitoring
   - 1-6 hours for performance analysis
   - 24 hours-7 days for capacity planning
4. **Create Alerts**: Use the recommended queries to set up proactive alerting
5. **Monitor Cost**: Track DPM (data points per minute) in comparison widgets

## Troubleshooting

### No Data Showing
1. Verify collectors are running: `docker ps`
2. Check deployment.mode attribute is set correctly
3. Confirm NEW_RELIC_LICENSE_KEY is valid
4. Review collector logs for errors

### Missing Custom Mode Metrics
1. Ensure custom collector image is built
2. Verify ASH receiver is configured
3. Check database permissions for pg_stat_* views
4. Review enhanced SQL queries for errors

### Performance Issues
1. Check circuit breaker status in custom mode
2. Monitor adaptive sampling rate
3. Review cost control metrics
4. Adjust collection intervals if needed

## Support

For dashboard issues:
1. Validate NRQL syntax in Query Builder
2. Check metric names with: `SELECT uniques(metricName) FROM Metric`
3. Verify attributes with: `SELECT keyset() FROM Metric`
4. Contact Database Intelligence team for custom metrics