# New Relic Deployment Guide

This directory contains configuration files for deploying PostgreSQL OpenTelemetry Collector with New Relic.

## Prerequisites

1. New Relic account with a valid license key
2. PostgreSQL database with required extensions (pg_stat_statements)
3. Docker or Kubernetes environment

## Configuration

### Environment Variables

Set these environment variables before deployment:

```bash
export NEWRELIC_API_KEY="your-license-key-here"
export NEWRELIC_REGION="US"  # or "EU"
```

### Docker Deployment

Use the New Relic-specific docker-compose file:

```bash
cd deployments/docker
docker-compose -f docker-compose-newrelic.yml up -d
```

### Kubernetes Deployment

1. Create the New Relic credentials secret:
```bash
kubectl create secret generic newrelic-credentials \
  --from-literal=NEWRELIC_API_KEY=$NEWRELIC_API_KEY \
  -n postgres-monitoring
```

2. Apply the Kubernetes manifests:
```bash
kubectl apply -k deployments/kubernetes/overlays/production
```

## Dashboard Setup

### Importing the Dashboard

1. Log in to your New Relic account
2. Navigate to Dashboards
3. Click "Import dashboard"
4. Copy the contents of `dashboard.json`
5. Paste and import

### Dashboard Panels

The dashboard includes:
- **Overview**: Database status, query performance, active sessions
- **Query Analysis**: Top slow queries, operation breakdown, duration distribution
- **Blocking & Locks**: Blocking sessions and lock type analysis
- **Buffer Cache**: Hit ratio and operation metrics
- **Collector Health**: Collection duration and metrics collected

## Alert Configuration

### Setting Up Alerts

1. Navigate to Alerts & AI in New Relic
2. Create a new alert policy named "PostgreSQL Monitoring"
3. Add conditions from `alerts.json`:
   - High Query Duration
   - Blocking Sessions Detected
   - Low Buffer Cache Hit Ratio
   - High Wait Event Count
   - Collector Failed
   - High Temp Block Usage

### Alert Channels

Configure notification channels:
- Email
- Slack
- PagerDuty
- Webhooks

## Metric Reference

### Key Metrics

| Metric Name | Description | Unit |
|------------|-------------|------|
| `postgresql.query.duration` | Query execution duration | milliseconds |
| `postgresql.query.count` | Number of query executions | count |
| `postgresql.query.rows` | Rows returned/affected | count |
| `postgresql.wait.duration` | Wait event duration | milliseconds |
| `postgresql.wait.count` | Wait event occurrences | count |
| `postgresql.blocking.sessions` | Number of blocking sessions | count |
| `postgresql.blocking.time` | Blocking duration | seconds |
| `postgresql.shared_buffer.hits` | Buffer cache hits | count |
| `postgresql.shared_buffer.reads` | Buffer cache reads | count |
| `postgresql.temp_blocks.written` | Temporary blocks written | count |

### Dimensions

Common dimensions across metrics:
- `db.name`: Database name
- `db.schema`: Schema name
- `db.operation`: SQL operation type
- `db.query.fingerprint`: Normalized query hash
- `db.user`: Database user
- `postgresql.wait.type`: Wait event type
- `postgresql.wait.event`: Specific wait event
- `postgresql.lock.type`: Lock type
- `service.name`: Service identifier
- `service.namespace`: Environment/namespace
- `deployment.environment`: Deployment environment

## NRQL Examples

### Find Top Slow Queries
```sql
SELECT 
  average(postgresql.query.duration) as 'Avg Duration',
  sum(postgresql.query.count) as 'Executions',
  latest(db.query.fingerprint) as 'Query Fingerprint'
FROM Metric 
WHERE db.system = 'postgresql' 
FACET db.query.fingerprint 
SINCE 1 hour ago 
LIMIT 20
```

### Monitor Wait Events
```sql
SELECT 
  sum(postgresql.wait.count) 
FROM Metric 
WHERE db.system = 'postgresql' 
FACET postgresql.wait.event, postgresql.wait.type 
TIMESERIES AUTO
```

### Buffer Cache Efficiency
```sql
SELECT 
  rate(sum(postgresql.shared_buffer.hits), 1 minute) / 
  (rate(sum(postgresql.shared_buffer.hits), 1 minute) + 
   rate(sum(postgresql.shared_buffer.reads), 1 minute)) * 100 
  AS 'Hit Ratio %' 
FROM Metric 
WHERE db.system = 'postgresql' 
TIMESERIES AUTO
```

## Troubleshooting

### No Data in New Relic

1. Check collector logs:
```bash
docker logs postgres-otel-collector
# or
kubectl logs -n postgres-monitoring deployment/postgres-collector
```

2. Verify environment variables are set correctly
3. Ensure network connectivity to New Relic endpoints
4. Check if API key is valid and has proper permissions

### High Cardinality Issues

If experiencing high cardinality:
1. Adjust `MAX_QUERY_FINGERPRINTS` to limit unique queries
2. Reduce `MAX_WAIT_EVENTS` for wait event types
3. Consider adjusting collection interval

### Performance Tuning

Optimize collector performance:
- Increase `WORKER_THREADS` for parallel processing
- Adjust `METRIC_BATCH_SIZE` for export efficiency
- Configure `MAX_CONCURRENT_EXPORTS` based on network capacity

## Support

For issues or questions:
1. Check collector logs for errors
2. Review New Relic documentation
3. Contact New Relic support with your account details