# Anomaly Detector Module

Statistical anomaly detection for MySQL metrics using threshold-based analysis.

## Overview

The anomaly detector module monitors metrics from other modules (core-metrics, sql-intelligence, wait-profiler) and applies statistical analysis to detect anomalous patterns. It uses threshold-based detection to identify deviations in:

- Connection patterns
- Query performance
- Wait events
- Resource usage

## Features

### Implemented
- **Threshold-based Detection**: Configurable thresholds for different metric types
- **Severity Classification**: Low, medium, high, and critical severity levels
- **Multi-source Federation**: Pulls metrics from multiple modules via Prometheus federation
- **Anomaly Score Calculation**: Simple scoring based on deviation from expected values
- **Alert Generation**: File-based alert export for detected anomalies
- **New Relic Integration**: Sends anomaly metrics to New Relic with proper entity synthesis

### Architecture

```
[Core Metrics] ─┐
[SQL Intelligence] ─┼─> [Anomaly Detector] ─> [New Relic]
[Wait Profiler] ─┘                        └─> [Alert File]
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CORE_METRICS_ENDPOINT` | Core metrics federation endpoint | `core-metrics:8081` |
| `SQL_INTELLIGENCE_ENDPOINT` | SQL intelligence federation endpoint | `sql-intelligence:8082` |
| `WAIT_PROFILER_ENDPOINT` | Wait profiler federation endpoint | `wait-profiler:8083` |
| `NEW_RELIC_LICENSE_KEY` | New Relic license key | Required |
| `NEW_RELIC_OTLP_ENDPOINT` | New Relic OTLP endpoint | Required |
| `ENVIRONMENT` | Deployment environment | `production` |

### Detection Thresholds

The module uses the following default thresholds:

- **Connections**: High=200, Low=10
- **Query Duration**: High=1000ms (1 second)
- **Wait Events**: High=5000ms

These are currently hardcoded but could be made configurable via environment variables.

## Metrics

### Input Metrics (via Federation)
- `mysql_connections_current`
- `mysql_threads_running`
- `mysql_buffer_pool_usage`
- `mysql_operations`
- `mysql_query_duration_milliseconds`
- `mysql_slow_queries`
- `mysql_statement_executions`
- `mysql_wait_*`
- `wait_profiler_*`

### Output Metrics
- `anomaly_score_connections`: Anomaly score for connection metrics
- `anomaly_score_query_duration`: Anomaly score for query performance
- `anomaly_score_wait_event`: Anomaly score for wait events

Each anomaly metric includes attributes:
- `original_value`: The actual metric value
- `anomaly_score`: Calculated deviation score
- `severity`: low/medium/high/critical
- `is_anomaly`: Boolean flag
- `metric_type`: Type of metric being analyzed

## Usage

### Basic Deployment

```bash
docker-compose up -d
```

### With Dependencies

To run with mock dependencies:

```bash
docker-compose --profile with-dependencies up -d
```

## ⚠️ Health Check Policy

**IMPORTANT**: Health check endpoints (port 13133) have been intentionally removed from production code.

- **For validation**: Use `shared/validation/health-check-all.sh`
- **Documentation**: See `shared/validation/README-health-check.md`
- **Do NOT**: Add health check endpoints back to production configs
- **Do NOT**: Expose port 13133 in Docker configurations

### Check Status

```bash
# Metrics check (production endpoint)
curl http://localhost:8084/metrics | grep anomaly_

# View metrics
curl http://localhost:8084/metrics

# Check alerts
cat /tmp/anomaly-alerts/alerts.json
```

## Pipeline Architecture

The module uses two pipelines:

1. **Main Pipeline**: Processes all metrics, calculates anomaly scores, and exports to New Relic
2. **Alert Pipeline**: Filters only anomalous metrics and writes to alert file

This design ensures efficient processing without duplication.

## Future Enhancements

To implement true statistical anomaly detection:

1. **Rolling Window Statistics**: Implement proper baseline calculation using historical data
2. **Dynamic Thresholds**: Calculate thresholds based on standard deviations
3. **Seasonal Patterns**: Account for time-of-day and day-of-week patterns
4. **Multiple Algorithms**: Add MAD (Median Absolute Deviation) and other statistical methods
5. **External Baseline Service**: Implement a separate service for complex baseline calculations

## Troubleshooting

### No Anomalies Detected
- Check if source modules are running and exposing metrics
- Verify federation endpoints are accessible
- Review threshold settings - they may need adjustment for your workload

### High Memory Usage
- Reduce `send_batch_size` in batch processor
- Lower `limit_mib` in memory_limiter
- Decrease scrape frequency

### Missing Metrics
- Ensure source modules are configured correctly
- Check network connectivity between modules
- Verify metric names match the regex patterns in metric_relabel_configs