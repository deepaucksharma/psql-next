# Cross-Signal Correlator Module

## Overview

The Cross-Signal Correlator module implements trace, log, and metric correlation features from the master-enhanced.yaml configuration. It provides:

- **Trace-to-metrics correlation** with exemplar support
- **Log-to-metrics conversion** from MySQL slow query logs
- **Span metrics generation** with configurable dimensions
- **Cross-signal context propagation**

## Features

### Signal Correlation
- Correlates traces, logs, and metrics using trace IDs and span IDs
- Generates exemplars for Prometheus metrics
- Parses MySQL slow query logs for metric generation
- Enriches metrics with trace context

### Advanced Capabilities
- Configurable histogram buckets for latency distributions
- Dimension key caching for performance
- Group-by-trace processing for correlation
- Federation from other monitoring modules

## Quick Start

```bash
# Run with standard configuration
make run

# Run with enhanced configuration (all features)
make run-enhanced

# Check health
make health

# View logs
make logs
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `COLLECTOR_CONFIG` | Config file to use | `collector.yaml` |
| `TRACE_CORRELATION_ENABLED` | Enable trace correlation | `true` |
| `LOG_CORRELATION_ENABLED` | Enable log correlation | `true` |
| `EXEMPLAR_ENABLED` | Enable exemplar generation | `true` |
| `CORE_METRICS_ENDPOINT` | Core metrics module endpoint | `core-metrics:8081` |
| `SQL_INTELLIGENCE_ENDPOINT` | SQL intelligence endpoint | `sql-intelligence:8082` |
| `WAIT_PROFILER_ENDPOINT` | Wait profiler endpoint | `wait-profiler:8083` |

### Ports

- `4317`: OTLP gRPC receiver (traces)
- `4318`: OTLP HTTP receiver (traces)
- `8892`: Prometheus metrics exporter
- `8888`: Internal telemetry metrics
- `13137`: Health check endpoint
- `1777`: pprof endpoint
- `55679`: zPages endpoint

## Pipelines

### 1. Traces Pipeline
Receives traces via OTLP and:
- Enriches with correlation IDs
- Forwards to span metrics connector
- Exports correlated traces

### 2. Logs Pipeline
Processes MySQL slow query logs:
- Parses query time, lock time, rows examined
- Converts to metrics via count connector
- Adds severity based on duration

### 3. Metrics from Logs
Receives metrics from log processing:
- Adds correlation attributes
- Exports to Prometheus with exemplars

### 4. Metrics from Spans
Receives metrics from span processing:
- Generates latency histograms
- Adds exemplar information
- Enriches with trace context

### 5. Federated Metrics
Pulls metrics from other modules:
- Adds correlation attributes
- Enables cross-module analysis

## Integration

### With SQL Intelligence Module
```yaml
# Pull query metrics for correlation
prometheus:
  config:
    scrape_configs:
      - job_name: 'sql-intelligence'
        static_configs:
          - targets: ['sql-intelligence:8082']
```

### With Application Traces
```yaml
# Send traces to correlator
otlp:
  endpoint: cross-signal-correlator:4317
  headers:
    correlation: enabled
```

### With MySQL Logs
```yaml
# Mount slow query log
volumes:
  - /var/log/mysql/slow.log:/var/log/mysql/slow.log:ro
```

## Metrics Generated

### From Traces
- `traces_spanmetrics_latency`: Latency histogram with exemplars
- `traces_spanmetrics_calls_total`: Total span count by operation

### From Logs
- `mysql.slowlog.count`: Count of slow queries
- `mysql.slowlog.query_time`: Query execution time
- `mysql.slowlog.lock_time`: Lock wait time

### Correlation Attributes
- `correlation.trace_id`: Trace identifier
- `correlation.span_id`: Span identifier
- `exemplar.trace_id`: For Prometheus exemplars
- `query_hash`: Query fingerprint for correlation

## Deployment Patterns

### Standalone Correlation
```bash
# Just correlation features
docker-compose up -d
```

### With Full Intelligence Stack
```bash
# Run with all modules
cd ../../integration
docker-compose -f docker-compose.all.yaml up -d
```

### Custom Configuration
```bash
# Use your own config
docker run -v ./my-config.yaml:/etc/otel/collector.yaml \
  database-intelligence/cross-signal-correlator:latest
```

## Troubleshooting

### No Traces Received
1. Check OTLP endpoints are accessible
2. Verify trace export configuration in applications
3. Check firewall rules for ports 4317/4318

### No Log Metrics
1. Verify slow query log path is mounted
2. Check log file permissions
3. Ensure slow query logging is enabled in MySQL

### Missing Exemplars
1. Verify Prometheus scraper supports OpenMetrics
2. Check `exemplar.enabled: true` in configuration
3. Ensure traces are being received

### High Memory Usage
1. Adjust `MEMORY_LIMIT_PERCENT`
2. Reduce `num_traces` in groupbytrace processor
3. Lower histogram bucket count

## Development

### Running Tests
```bash
make test
```

### Validating Configuration
```bash
make validate-config
make validate-enhanced
```

### Debugging
```bash
# Access container shell
make shell

# View internal metrics
curl http://localhost:8888/metrics

# Check zPages
open http://localhost:55679/debug/tracez
```