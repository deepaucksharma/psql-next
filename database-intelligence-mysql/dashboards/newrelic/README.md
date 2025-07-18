# MySQL Wait-Based Monitoring Dashboards for New Relic

This directory contains enhanced MySQL monitoring dashboards for New Relic that focus on wait-based performance analysis, following the SolarWinds DPA methodology.

## Dashboard Overview

### 1. MySQL Performance Monitoring - Enhanced OpenTelemetry
**File:** `mysql-dashboard-enhanced.json`

A comprehensive MySQL monitoring dashboard with advanced wait-based analysis capabilities:

#### Key Features:
- **Database Health Score**: Composite metric showing overall database health
- **Connection Utilization**: Real-time connection pool monitoring with alerts
- **Query Performance Metrics**: QPS, slow queries, and wait time analysis
- **Buffer Pool Efficiency**: Hit rates and utilization metrics
- **InnoDB Deep Dive**: Detailed storage engine performance metrics
- **Table & Index Performance**: I/O wait analysis at table and index level
- **Replication & HA**: Comprehensive replication lag and status monitoring
- **Resource Utilization**: System resource usage and capacity planning
- **Performance Trends**: 7-day historical analysis for capacity planning

#### Pages:
1. **Overview**: High-level performance metrics with intelligent alerts
2. **Wait Analysis**: Comprehensive wait-based performance analysis
3. **Query Intelligence**: Advanced query analysis and optimization insights
4. **InnoDB Deep Dive**: Storage engine performance metrics
5. **Table & Index Performance**: Detailed I/O and lock analysis
6. **Replication & HA**: Replication monitoring and GTID tracking
7. **Resource Utilization**: Connection pools, network I/O, and cache performance
8. **Performance Trends**: Long-term trends and capacity planning

### 2. MySQL Wait-Based Performance Analysis - Enhanced
**File:** `wait-analysis-dashboard-enhanced.json`

Advanced wait-time analysis dashboard with intelligent insights and anomaly detection:

#### Key Features:
- **Real-Time Wait Analysis**: Live wait time monitoring with categorization
- **Anomaly Detection**: Automatic detection of performance anomalies
- **Blocking Chain Analysis**: Visual representation of lock dependencies
- **Query Intelligence**: Smart advisories and optimization recommendations
- **Resource Correlation**: Wait times correlated with system resources
- **Business Impact Analysis**: SLI impact and revenue at risk calculations
- **Predictive Analytics**: Wait time predictions and capacity forecasting

#### Pages:
1. **Wait Time Overview**: Database-wide wait analysis and trends
2. **Top Wait Contributors**: Queries causing the most wait time
3. **Blocking & Lock Analysis**: Detailed lock waits and deadlock detection
4. **Query Intelligence & Advisories**: Optimization opportunities
5. **Resource Correlation**: System resource impact on wait times
6. **SLI & Business Impact**: Service level and business impact metrics
7. **Predictive Analytics**: Forecasting and trend analysis

### 3. MySQL Query Detail Analysis - Enhanced
**File:** `query-detail-dashboard-enhanced.json`

Deep dive into individual query performance with advanced analytics:

#### Key Features:
- **Query Performance Profile**: Comprehensive single-query analysis
- **Execution Plan Analysis**: Plan changes and optimization opportunities
- **Wait Time Deep Dive**: Detailed wait event analysis
- **Instance Performance Comparison**: Query performance across instances
- **Impact Analysis**: Business and service impact assessment
- **Cost Analysis**: Query execution cost and optimization ROI

#### Pages:
1. **Query Performance Profile**: Overview and execution statistics
2. **Performance Trends**: Historical analysis and regression detection
3. **Wait Analysis Deep Dive**: Detailed wait time breakdown
4. **Execution Plan Analysis**: Query plan and index usage
5. **Instance Performance**: Cross-instance performance comparison
6. **Impact Analysis**: Business impact and cost analysis

#### Dashboard Variables:
- `query_hash`: Select specific query for analysis
- `query_pattern`: Filter queries by pattern

## Deployment

### Prerequisites
1. New Relic account with API access
2. Environment variables set:
   ```bash
   export NEW_RELIC_API_KEY="your-api-key"
   export NEW_RELIC_ACCOUNT_ID="your-account-id"
   ```
3. Tools installed: `curl`, `jq`

### Deploy Dashboards
```bash
# Deploy all dashboards
./scripts/deploy-newrelic-dashboards.sh deploy

# List deployed dashboards
./scripts/deploy-newrelic-dashboards.sh list
```

### Verify NRQL Queries
```bash
# Verify all queries return data
./scripts/verify-nrql-queries.sh verify

# Test specific widget
./scripts/verify-nrql-queries.sh test "MySQL Performance" "Connection Utilization"
```

## Key Metrics and Attributes

### Required OpenTelemetry Attributes
- `instrumentation.provider`: Must be set to 'opentelemetry'
- `mysql.instance.endpoint`: MySQL instance identifier
- `mysql.instance.role`: Instance role (primary/replica)
- `service.name`: Service identifier

### Wait Analysis Attributes
- `wait.category`: Type of wait (io, lock, cpu, network)
- `wait.severity`: Wait severity level (critical, high, medium, low)
- `wait_percentage`: Percentage of execution time spent waiting
- `mysql.query.wait_profile`: Total wait time for query

### Advisory Attributes
- `advisor.type`: Type of advisory (missing_index, slow_query, etc.)
- `advisor.recommendation`: Specific recommendation text
- `advisor.priority`: Priority level (P1, P2, P3)
- `advisor.expected_improvement`: Expected performance improvement

### Query Performance Attributes
- `query_hash`: Unique query identifier
- `statement_time_ms`: Total query execution time
- `lock_time_ms`: Time spent waiting for locks
- `ROWS_EXAMINED`: Number of rows examined
- `full_scans`: Number of full table scans

## Alerts and Thresholds

### Connection Utilization
- Warning: 80% utilization
- Critical: 90% utilization

### Replication Lag
- Warning: 10 seconds
- Critical: 30 seconds

### Database Health Score
- Warning: Below 70
- Critical: Below 50

### P1 Advisories
- Critical: Any P1 advisory detected

### Anomalies
- Warning: 5 anomalies detected
- Critical: 10 anomalies detected

## Customization

### Adding Custom Metrics
1. Edit the dashboard JSON file
2. Add new widget configuration
3. Update NRQL query with your metrics
4. Redeploy using deployment script

### Modifying Thresholds
1. Locate threshold configuration in widget
2. Update `value` and `alertSeverity`
3. Redeploy dashboard

### Creating Custom Views
1. Clone existing dashboard JSON
2. Modify pages and widgets
3. Update dashboard name and description
4. Deploy as new dashboard

## Troubleshooting

### No Data Showing
1. Verify OpenTelemetry collector is running
2. Check `instrumentation.provider = 'opentelemetry'` is set
3. Confirm data exists for selected time range
4. Run query verification script

### Query Errors
1. Check NRQL syntax in verification report
2. Verify attribute names match your data
3. Ensure account ID is correctly set
4. Check API key permissions

### Performance Issues
1. Reduce time range for queries
2. Add LIMIT clauses where appropriate
3. Use sampling for high-cardinality data
4. Consider creating summary metrics

## Best Practices

1. **Regular Monitoring**: Review dashboards daily for performance insights
2. **Act on Advisories**: Prioritize P1 advisories for immediate action
3. **Track Trends**: Use trend data for capacity planning
4. **Correlate Metrics**: Look for patterns between wait times and resources
5. **Optimize Queries**: Focus on top wait contributors first
6. **Monitor Replication**: Keep replication lag under 10 seconds
7. **Capacity Planning**: Use predictive analytics for resource planning

## Support

For issues or questions:
1. Check query verification report for errors
2. Review New Relic documentation for NRQL syntax
3. Verify OpenTelemetry collector configuration
4. Contact your database administration team