# Anomaly Detector Module

Statistical anomaly detection module for the Database Intelligence system that monitors metrics from other modules and identifies anomalous patterns.

## Features

- **Multi-Module Metric Federation**: Pulls metrics from core-metrics, sql-intelligence, and wait-profiler modules
- **Statistical Anomaly Detection**: Uses z-score based detection for identifying deviations
- **Real-time Alert Generation**: Creates alerts when anomalies exceed configured thresholds
- **Multiple Anomaly Types**:
  - Connection spikes
  - Query latency deviations
  - Wait event anomalies
  - Resource usage patterns

## Architecture

The module uses Prometheus federation to collect metrics from other running modules and applies statistical transformations to detect anomalies:

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  core-metrics   │     │ sql-intelligence│     │  wait-profiler  │
│   (port 8081)   │     │   (port 8082)   │     │   (port 8083)   │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                         │
         └───────────────────────┴─────────────────────────┘
                                 │
                                 ▼
                      ┌─────────────────────┐
                      │  anomaly-detector   │
                      │    (port 8084)      │
                      │                     │
                      │ • Z-score calc      │
                      │ • Threshold check   │
                      │ • Alert generation  │
                      └─────────────────────┘
```

## Quick Start

### Prerequisites

The anomaly detector requires other modules to be running:

```bash
# Start dependent modules (from monorepo root)
cd ../core-metrics && make run
cd ../sql-intelligence && make run
cd ../wait-profiler && make run
```

### Running the Module

```bash
# Build the module
make build

# Check if dependencies are accessible
make check-dependencies

# Run the module
make run

# View logs
make logs

# Check status and metrics
make status

# View generated alerts
make alerts

# Stop the module
make stop
```

## Configuration

### Environment Variables

- `EXPORT_PORT`: Prometheus metrics port (default: 8084)
- `CORE_METRICS_ENDPOINT`: Core metrics federation endpoint (default: http://host.docker.internal:8081/metrics)
- `SQL_INTELLIGENCE_ENDPOINT`: SQL intelligence federation endpoint (default: http://host.docker.internal:8082/metrics)
- `WAIT_PROFILER_ENDPOINT`: Wait profiler federation endpoint (default: http://host.docker.internal:8083/metrics)

### Anomaly Detection Thresholds

Configure z-score thresholds for different anomaly types:

- `CONNECTION_SPIKE_THRESHOLD`: Connection spike detection threshold (default: 2.0)
- `LATENCY_DEVIATION_THRESHOLD`: Query latency deviation threshold (default: 3.0)
- `WAIT_EVENT_THRESHOLD`: Wait event anomaly threshold (default: 2.5)
- `RESOURCE_USAGE_THRESHOLD`: Resource usage anomaly threshold (default: 2.0)

## Metrics Exposed

### Anomaly Scores

All anomaly scores are z-scores (standard deviations from mean):

- `anomaly_score_connections`: Connection count deviation score
- `anomaly_score_query_latency`: Query latency deviation score
- `anomaly_score_wait_events`: Wait event time deviation score
- `anomaly_score_cpu`: CPU usage deviation score

### Anomaly Alerts

Alert metrics (value=1 when active):

- `anomaly_alert{anomaly_type="connection_spike",alert_severity="high"}`
- `anomaly_alert{anomaly_type="latency_deviation",alert_severity="critical"}`
- `anomaly_alert{anomaly_type="wait_anomaly",alert_severity="medium"}`
- `anomaly_alert{anomaly_type="resource_usage",alert_severity="medium"}`

## Detection Methodology

The module uses statistical z-score calculation for anomaly detection:

```
z-score = (current_value - baseline_mean) / baseline_stddev
```

When the z-score exceeds the configured threshold for a metric type, an anomaly alert is generated.

### Baseline Calculation

Currently using simplified static baselines. In production, implement:
- Rolling window statistics
- Time-series decomposition
- Seasonal adjustment
- Machine learning models

## Integration Examples

### Grafana Dashboard

Create alerts based on anomaly metrics:

```promql
# High severity alerts
anomaly_alert{alert_severity="critical"} == 1

# Connection anomalies over time
anomaly_score_connections > 2

# Multi-metric correlation
(anomaly_score_connections > 2) and (anomaly_score_query_latency > 2)
```

### Alertmanager Integration

Configure Alertmanager to receive anomaly alerts:

```yaml
groups:
  - name: database_anomalies
    rules:
      - alert: DatabaseConnectionSpike
        expr: anomaly_alert{anomaly_type="connection_spike"} == 1
        for: 5m
        annotations:
          summary: "Anomalous connection spike detected"
          
      - alert: QueryLatencyAnomaly
        expr: anomaly_alert{anomaly_type="latency_deviation"} == 1
        for: 3m
        annotations:
          summary: "Query latency anomaly detected"
```

## Testing

```bash
# Run all tests
make test

# Simulate anomalies (run from another terminal)
# Spike connections on the MySQL instance
for i in {1..100}; do mysql -h localhost -P 3306 -u root -ptest -e "SELECT 1" & done

# Check if anomalies are detected
make status
```

## Advanced Configuration

### Custom Anomaly Detection

Modify `config/collector.yaml` to add custom anomaly detection:

```yaml
transform/custom_anomaly:
  metric_statements:
    - context: metric
      statements:
        - set(name, "anomaly_score_custom") where name == "your_metric_name"
        - set(value, your_anomaly_calculation) where name == "anomaly_score_custom"
```

### Multi-Metric Correlation

Add correlation detection in the transform processor:

```yaml
- set(name, "anomaly_correlation") where anomaly_score_connections > 2 and anomaly_score_latency > 2
- set(attributes["correlation_type"], "connection_latency") where name == "anomaly_correlation"
```

## Production Considerations

1. **Baseline Service**: Implement a proper baseline calculation service with:
   - Historical data storage
   - Adaptive learning
   - Seasonality handling

2. **Alert Fatigue**: Implement:
   - Alert suppression
   - Anomaly clustering
   - Root cause analysis

3. **Performance**: Consider:
   - Sampling high-frequency metrics
   - Using streaming analytics
   - Distributed anomaly detection

4. **Integration**: Connect to:
   - PagerDuty for critical alerts
   - Slack for notifications
   - Incident management systems