# MySQL Intelligence - New Relic Integration Guide

## Overview

This guide provides comprehensive instructions for deploying and using the MySQL Intelligence monitoring system with New Relic. All components are designed to send data to New Relic's NRDB for analysis, visualization, and alerting.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Environment Setup](#environment-setup)
3. [Deployment](#deployment)
4. [New Relic Dashboard Access](#new-relic-dashboard-access)
5. [NRQL Queries](#nrql-queries)
6. [Alerting](#alerting)
7. [Cost Optimization](#cost-optimization)
8. [Troubleshooting](#troubleshooting)

## Prerequisites

### Required New Relic Resources

1. **New Relic Account** with:
   - License Key (Ingest API)
   - API Key (NerdGraph access)
   - Account ID
   - OTLP endpoint access

2. **Permissions**:
   - Dashboard creation
   - Alert policy management
   - Synthetic monitor creation
   - Workload management

### System Requirements

- Docker & Docker Compose
- 16+ CPU cores, 16+ GB RAM (for full deployment)
- Network access to New Relic endpoints

## Environment Setup

### 1. Create Environment File

```bash
# Create .env file in project root
cat > .env << EOF
# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your-license-key-here
NEW_RELIC_API_KEY=your-api-key-here
NEW_RELIC_ACCOUNT_ID=your-account-id-here
NEW_RELIC_REGION=US  # or EU

# OTLP Configuration
NEW_RELIC_OTLP_ENDPOINT=https://otlp.nr-data.net
# For EU: NEW_RELIC_OTLP_ENDPOINT=https://otlp.eu01.nr-data.net

# MySQL Configuration
MYSQL_ENDPOINT=mysql-master:3306
MYSQL_USER=root
MYSQL_PASSWORD=rootpassword
MYSQL_DATABASE=test_db

# Deployment Configuration
ENVIRONMENT=production
CLUSTER_NAME=mysql-intelligence-prod
TEAM_NAME=database-team
COST_CENTER=engineering

# Feature Flags
ENABLE_SQL_INTELLIGENCE=true
STATISTICAL_FEATURES_ENABLED=true
BUSINESS_CONTEXT_ENABLED=true
CIRCUIT_FAILURE_THRESHOLD=5
ROLLOUT_PERCENTAGE=100
EOF
```

### 2. Automated New Relic Setup

```bash
# Make setup script executable
chmod +x shared/newrelic/scripts/setup-newrelic.sh

# Run automated setup
./shared/newrelic/scripts/setup-newrelic.sh
```

This script will create:
- MySQL Intelligence Dashboard
- Alert policies and conditions
- Workloads
- Synthetic monitors
- Applied Intelligence workflows

## Deployment

### Option 1: Full Enhanced Deployment (Recommended)

```bash
# Deploy all modules with New Relic integration
make run-enhanced

# Verify deployment
make health
```

### Option 2: Selective Module Deployment

```bash
# Deploy core monitoring
make run-enhanced-core-metrics

# Deploy SQL intelligence
make run-enhanced-sql-intelligence

# Deploy cross-signal correlation
make run-cross-signal-correlator
```

### Option 3: Docker Compose with New Relic

```bash
# Navigate to integration directory
cd integration

# Deploy with enhanced configuration
docker-compose -f docker-compose.enhanced.yaml up -d
```

## New Relic Dashboard Access

### 1. Access Pre-built Dashboard

After running the setup script, access your dashboard:

1. Log in to [New Relic One](https://one.newrelic.com)
2. Navigate to **Dashboards**
3. Search for "MySQL Intelligence - Complete Monitoring"
4. Or use the direct link provided by the setup script

### 2. Dashboard Pages

The dashboard includes 8 pages:

1. **Executive Overview**
   - Fleet health score
   - Business impact trends
   - Top performance issues
   - Intelligence score distribution

2. **SQL Intelligence Analysis**
   - Query wait profiles
   - Statistical anomaly detection
   - Resource pressure correlation
   - Query pattern recognition

3. **Replication & High Availability**
   - Replication lag trends
   - GTID execution status
   - Health scoring

4. **Business Impact & SLA**
   - SLA violations
   - Revenue impact tracking
   - Critical table operations

5. **Performance Advisory**
   - Automated recommendations
   - Index optimization opportunities
   - Resource saturation alerts

6. **Cross-Signal Correlation**
   - Trace-to-metric correlation
   - Exemplar distribution
   - Slow query log analysis

7. **Canary & Synthetic Tests**
   - Baseline deviation tracking
   - Workload simulation results
   - Health status

8. **Anomaly Detection**
   - Anomaly timeline
   - Severity distribution
   - Top anomalous queries

## NRQL Queries

### Essential Queries

#### 1. MySQL Health Overview
```sql
SELECT 
  average(mysql.health.score) as 'Health Score',
  uniqueCount(entity.name) as 'Instances',
  percentage(count(*), WHERE replication.severity = 'healthy') as 'Healthy %'
FROM Metric 
WHERE entity.type = 'MYSQL_INSTANCE' 
SINCE 1 hour ago
```

#### 2. Top Intelligence Issues
```sql
SELECT 
  latest(query_text) as 'Query',
  average(intelligence_score) as 'Score',
  latest(recommendations) as 'Recommendation'
FROM Metric 
WHERE metricName = 'mysql.intelligence.comprehensive' 
  AND intelligence_score > 100
FACET query_digest 
SINCE 1 hour ago
LIMIT 20
```

#### 3. Business Impact Analysis
```sql
SELECT 
  sum(business.revenue_impact) as 'Revenue Impact',
  count(*) as 'Operations'
FROM Metric 
WHERE business_criticality IN ('CRITICAL', 'HIGH')
FACET db_schema, business_criticality 
TIMESERIES 1 hour
SINCE 24 hours ago
```

#### 4. Wait Event Analysis
```sql
SELECT 
  histogram(current_wait_time_ms, 50, 20) 
FROM Metric 
FACET wait.primary_category 
WHERE current_wait_time_ms > 0 
SINCE 1 hour ago
```

#### 5. Anomaly Detection
```sql
SELECT 
  count(*) as 'Anomalies',
  average(statistics.anomaly_score) as 'Avg Score'
FROM Metric 
WHERE statistics.is_anomaly = true 
FACET statistics.workload_type, anomaly_type 
TIMESERIES AUTO
SINCE 6 hours ago
```

### Advanced Queries

#### 1. Query Performance with Traces
```sql
SELECT 
  average(traces_spanmetrics_latency) as 'Span Latency',
  count(*) as 'Operations'
FROM Metric 
WHERE db.system = 'mysql' 
  AND exemplar.trace_id IS NOT NULL
FACET db.operation 
TIMESERIES AUTO
SINCE 1 hour ago
```

#### 2. Replication Lag Prediction
```sql
SELECT 
  predictLinear(mysql.replica.time_behind_source, 3600) as 'Predicted Lag (1hr)'
FROM Metric 
WHERE entity.name LIKE '%replica%' 
FACET entity.name 
SINCE 6 hours ago
```

#### 3. Cost Analysis by Query
```sql
SELECT 
  sum(cost.compute_impact) as 'Compute Cost',
  sum(cost.io_impact) as 'IO Cost',
  latest(query_text) as 'Query'
FROM Metric 
WHERE cost.compute_impact > 0 
FACET query_digest 
SINCE 24 hours ago
LIMIT 50
```

## Alerting

### Pre-configured Alerts

The setup script creates these alert conditions:

1. **Query Intelligence Score Critical**
   - Threshold: Score > 150
   - Duration: 5 minutes

2. **Replication Lag High**
   - Threshold: > 300 seconds
   - Duration: 3 minutes

3. **Business Revenue Impact**
   - Threshold: > $1000/hour
   - Duration: 1 minute

4. **Anomaly Detection Rate**
   - Threshold: > 50 anomalies/5min
   - Duration: 5 minutes

5. **Canary Test Failures**
   - Threshold: > 20% failure rate
   - Duration: 3 minutes

### Custom Alert Creation

```sql
-- Example: Create alert for lock escalation
SELECT count(*) 
FROM Metric 
WHERE advisor.type = 'lock_escalation' 
  AND current_exec_time_ms > 5000
```

### Alert Channels

Configure notification channels in New Relic:

1. Navigate to **Alerts & AI** > **Notification channels**
2. Add channels:
   - Email
   - Slack
   - PagerDuty
   - Webhook (for integration with alert-manager)

## Cost Optimization

### 1. Data Drop Rules

Create drop rules for high-volume, low-value metrics:

```graphql
mutation CreateDropRule {
  nrqlDropRulesCreate(
    accountId: YOUR_ACCOUNT_ID
    rules: [{
      nrql: "SELECT * FROM Metric WHERE metricName = 'mysql.heartbeat' AND value < 1"
      description: "Drop low-value heartbeat metrics"
      action: DROP_DATA
    }]
  ) {
    successes { id }
  }
}
```

### 2. Metric Aggregation

Configure edge aggregation in collectors:

```yaml
processors:
  metricstransform/aggregation:
    transforms:
      - include: mysql.query.count
        action: combine
        submatch_case: "strict"
        operations:
          - action: aggregate_labels
            label_set: [query_type]
            aggregation_type: sum
```

### 3. Sampling Strategies

Adjust sampling for non-critical metrics:

```yaml
processors:
  probabilistic_sampler:
    sampling_percentage: 10  # Sample 10% of metrics
    attribute_source: "query_digest"
```

## Troubleshooting

### 1. No Data in New Relic

```bash
# Check collector logs
docker-compose logs -f integration-collector | grep -i error

# Verify API key
curl -X POST https://api.newrelic.com/graphql \
  -H "API-Key: $NEW_RELIC_API_KEY" \
  -d '{"query": "{ actor { user { email } } }"}'

# Test OTLP endpoint
curl -X POST $NEW_RELIC_OTLP_ENDPOINT/v1/metrics \
  -H "api-key: $NEW_RELIC_LICENSE_KEY" \
  -H "Content-Type: application/x-protobuf"
```

### 2. High Data Ingestion Costs

```sql
-- Analyze data usage by metric
SELECT 
  metricName,
  bytecountestimate() / 1e9 as 'GB',
  uniqueCount(dimensions()) as 'Cardinality'
FROM Metric 
WHERE instrumentation.provider = 'opentelemetry'
FACET metricName 
SINCE 1 day ago
LIMIT 50
```

### 3. Missing Entities

Ensure entity synthesis attributes are present:

```yaml
processors:
  attributes/entity_synthesis:
    actions:
      - key: entity.type
        value: "MYSQL_INSTANCE"
        action: insert
      - key: entity.guid
        value: "MYSQL|${CLUSTER_NAME}|${MYSQL_ENDPOINT}"
        action: insert
```

### 4. Alert Fatigue

Tune alert conditions:

```sql
-- Analyze alert frequency
SELECT 
  count(*) as 'Alerts',
  latest(conditionName) as 'Condition'
FROM NrAiIncident 
FACET conditionName 
SINCE 1 week ago
```

## Best Practices

### 1. Tag Strategy

Use consistent tags for filtering:

```yaml
processors:
  attributes/tags:
    actions:
      - key: tags.environment
        value: ${ENVIRONMENT}
      - key: tags.team
        value: ${TEAM_NAME}
      - key: tags.criticality
        value: ${BUSINESS_CRITICALITY}
```

### 2. Workload Organization

Create workloads by:
- Environment (prod, staging, dev)
- Business function (payments, inventory)
- Team ownership

### 3. Dashboard Variables

Use dashboard variables for flexibility:

```json
{
  "variables": [{
    "name": "environment",
    "type": "NRQL",
    "query": "SELECT uniques(environment) FROM Metric"
  }]
}
```

### 4. Regular Reviews

Weekly tasks:
- Review top intelligence scores
- Check anomaly trends
- Validate business impact accuracy
- Update alert thresholds

## Advanced Integration

### 1. Terraform Management

```hcl
resource "newrelic_dashboard" "mysql_intelligence" {
  name = "MySQL Intelligence Dashboard"
  permissions = "public_read_write"
  
  page {
    name = "Overview"
    
    widget_billboard {
      title = "Health Score"
      nrql_query {
        query = "SELECT average(mysql.health.score) FROM Metric"
      }
    }
  }
}
```

### 2. CI/CD Integration

```yaml
# .github/workflows/newrelic-deploy.yml
steps:
  - name: Deploy to New Relic
    env:
      NEW_RELIC_API_KEY: ${{ secrets.NEW_RELIC_API_KEY }}
    run: |
      ./shared/newrelic/scripts/setup-newrelic.sh
```

### 3. Custom Visualizations

Create custom visualizations using New Relic's Nerdpack SDK for specialized views of MySQL intelligence data.

## Support

For issues or questions:

1. Check New Relic [documentation](https://docs.newrelic.com)
2. Review collector logs: `make logs-<module>`
3. Use New Relic support channels
4. Consult the [Implementation Status](../../IMPLEMENTATION_STATUS.md)

Remember to replace placeholder values (YOUR_ACCOUNT_ID, etc.) with actual values from your New Relic account.