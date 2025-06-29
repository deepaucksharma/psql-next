# Monitoring Dashboard Templates

This directory contains dashboard templates for monitoring the Database Intelligence OTEL Collector.

## Available Dashboards

### 1. Database Performance Overview
Monitor overall database health and performance across all databases.

**Key Metrics:**
- Query execution time (p50, p95, p99)
- Query throughput (queries/second)
- Active sessions by state
- Database connections
- Cache hit ratios

### 2. Collector Health Dashboard
Monitor the OTEL collector itself.

**Key Metrics:**
- Collector uptime and health
- Memory and CPU usage
- Processing latency by processor
- Error rates
- Circuit breaker states

### 3. Query Performance Analysis
Deep dive into query performance.

**Key Metrics:**
- Top slow queries
- Query execution trends
- Query plan analysis
- Sampling rates by query type
- PII detection alerts

### 4. Operational Dashboard
For operations teams to monitor system health.

**Key Metrics:**
- System health score
- Alert summary
- Resource usage trends
- Data flow rates
- Verification status

## New Relic Dashboard Templates

### Database Performance Overview
```json
{
  "name": "Database Intelligence - Performance Overview",
  "pages": [
    {
      "name": "Overview",
      "widgets": [
        {
          "title": "Query Performance Trend",
          "configuration": {
            "nrql": "SELECT average(db.query.exec_time.mean) as 'Avg Execution Time' FROM Metric WHERE service.name = 'database-intelligence' TIMESERIES AUTO"
          }
        },
        {
          "title": "Top Slow Queries",
          "configuration": {
            "nrql": "SELECT average(db.query.exec_time.mean) FROM Metric WHERE service.name = 'database-intelligence' FACET query.text LIMIT 10"
          }
        },
        {
          "title": "Active Sessions",
          "configuration": {
            "nrql": "SELECT sum(db.sessions.active) FROM Metric WHERE service.name = 'database-intelligence' FACET session.state TIMESERIES AUTO"
          }
        },
        {
          "title": "Database Connections",
          "configuration": {
            "nrql": "SELECT latest(postgresql.backends) as 'Connections' FROM Metric WHERE service.name = 'database-intelligence' TIMESERIES AUTO"
          }
        }
      ]
    }
  ]
}
```

### Collector Health Dashboard
```json
{
  "name": "Database Intelligence - Collector Health",
  "pages": [
    {
      "name": "Health",
      "widgets": [
        {
          "title": "Collector Uptime",
          "configuration": {
            "nrql": "SELECT percentage(count(*), WHERE health_check = 'ok') as 'Uptime %' FROM Metric WHERE service.name = 'database-intelligence' SINCE 24 hours ago"
          }
        },
        {
          "title": "Memory Usage",
          "configuration": {
            "nrql": "SELECT average(otelcol_process_memory_rss) / 1048576 as 'Memory (MB)' FROM Metric WHERE service.name = 'database-intelligence' TIMESERIES AUTO"
          }
        },
        {
          "title": "Processing Latency",
          "configuration": {
            "nrql": "SELECT average(otelcol_processor_process_duration) * 1000 as 'Latency (ms)' FROM Metric WHERE service.name = 'database-intelligence' FACET processor TIMESERIES AUTO"
          }
        },
        {
          "title": "Circuit Breaker Status",
          "configuration": {
            "nrql": "SELECT uniqueCount(database) FROM Metric WHERE service.name = 'database-intelligence' FACET circuit_breaker_state SINCE 1 hour ago"
          }
        }
      ]
    }
  ]
}
```

## Grafana Dashboard Templates

### Database Performance (Grafana)
```json
{
  "dashboard": {
    "title": "Database Intelligence - Performance",
    "panels": [
      {
        "title": "Query Execution Time",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, db_query_exec_time_mean)",
            "legendFormat": "p95"
          },
          {
            "expr": "histogram_quantile(0.99, db_query_exec_time_mean)",
            "legendFormat": "p99"
          }
        ]
      },
      {
        "title": "Queries Per Second",
        "targets": [
          {
            "expr": "rate(db_query_calls[5m])",
            "legendFormat": "{{query_id}}"
          }
        ]
      }
    ]
  }
}
```

### Collector Health (Grafana)
```json
{
  "dashboard": {
    "title": "Database Intelligence - Collector Health",
    "panels": [
      {
        "title": "Memory Usage",
        "targets": [
          {
            "expr": "process_resident_memory_bytes{service_name=\"database-intelligence\"} / 1048576"
          }
        ]
      },
      {
        "title": "CPU Usage",
        "targets": [
          {
            "expr": "rate(process_cpu_seconds_total{service_name=\"database-intelligence\"}[5m]) * 100"
          }
        ]
      }
    ]
  }
}
```

## Prometheus Alert Rules

```yaml
groups:
  - name: database_intelligence
    interval: 30s
    rules:
      - alert: CollectorDown
        expr: up{job="database-intelligence"} == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Database Intelligence collector is down"
          
      - alert: HighMemoryUsage
        expr: process_resident_memory_bytes{service_name="database-intelligence"} / 1048576 > 800
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Collector memory usage above 800MB"
          
      - alert: CircuitBreakerOpen
        expr: circuit_breaker_state{state="open"} > 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Circuit breaker is open for {{ $labels.database }}"
          
      - alert: HighErrorRate
        expr: rate(otelcol_processor_refused_metric_points[5m]) > 0.05
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High error rate in metric processing"
```

## Usage Instructions

### For New Relic
1. Log in to New Relic
2. Navigate to Dashboards
3. Click "Import dashboard"
4. Paste the JSON template
5. Update the service name if different
6. Save dashboard

### For Grafana
1. Log in to Grafana
2. Create new dashboard
3. Import JSON
4. Update data source to your Prometheus instance
5. Adjust time ranges and variables

### For Prometheus Alerts
1. Add rules to your Prometheus configuration
2. Reload Prometheus
3. Configure AlertManager for notifications

## Customization

All dashboards can be customized:
- Add additional metrics
- Change time ranges
- Add filters for specific databases
- Create custom visualizations
- Add annotations for deployments

## Best Practices

1. **Start with Overview**: Use the overview dashboard for general monitoring
2. **Drill Down**: Use specialized dashboards for investigation
3. **Set Alerts**: Configure alerts for critical metrics
4. **Regular Review**: Review dashboards weekly to ensure relevance
5. **Share Knowledge**: Document any custom queries or insights