# Database Intelligence Collector MVP

A next-generation OpenTelemetry collector for PostgreSQL that provides advanced database observability with execution plan analysis, Active Session History (ASH), and intelligent performance monitoring.

## Overview

The Database Intelligence Collector extends the OpenTelemetry Collector with specialized components for deep PostgreSQL monitoring, providing capabilities similar to enterprise database monitoring solutions while maintaining the flexibility and openness of OpenTelemetry.

## Key Features

### 🔍 Plan Intelligence
- **Safe Plan Collection**: Collects execution plans from auto_explain logs without query overhead
- **Plan Anonymization**: Automatically removes PII and sensitive data from plans
- **Regression Detection**: Statistical analysis to identify performance degradations
- **Plan Versioning**: Tracks plan changes over time with history

### 📊 Active Session History (ASH)
- **High-Frequency Sampling**: 1-second resolution session sampling
- **Adaptive Sampling**: Automatically adjusts based on system load
- **Wait Event Analysis**: Categorizes and analyzes database wait events
- **Blocking Detection**: Identifies and tracks blocking chains

### 🛡️ Security & Robustness
- **PII Protection**: Multi-layer approach to data anonymization
- **Circuit Breakers**: Prevents cascade failures
- **Feature Detection**: Gracefully handles missing extensions
- **Resource Limits**: Memory and CPU usage controls

### 🎯 Intelligent Processing
- **Adaptive Sampling**: Load-aware metric collection
- **Anomaly Detection**: Statistical analysis for outliers
- **Workload Classification**: Identifies OLTP vs OLAP patterns
- **Performance Recommendations**: Automated insights

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    PostgreSQL Database                       │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │ pg_stat_*   │  │ auto_explain │  │ pg_stat_activity│   │
│  └─────────────┘  └──────────────┘  └─────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
┌─────────────────────────────────────────────────────────────┐
│                    Receiver Layer                            │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │ PostgreSQL  │  │ AutoExplain  │  │      ASH        │   │
│  │  Receiver   │  │   Receiver   │  │   Receiver      │   │
│  └─────────────┘  └──────────────┘  └─────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
┌─────────────────────────────────────────────────────────────┐
│                   Processor Layer                            │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │    Plan     │  │    Wait      │  │   Adaptive      │   │
│  │ Anonymizer  │  │  Analysis    │  │   Sampler       │   │
│  └─────────────┘  └──────────────┘  └─────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                            │
┌─────────────────────────────────────────────────────────────┐
│                    Export Layer                              │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │ Prometheus  │  │     OTLP     │  │   New Relic     │   │
│  └─────────────┘  └──────────────┘  └─────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- PostgreSQL 12+ with auto_explain enabled
- OpenTelemetry Collector Contrib
- Go 1.21+ (for building)

### Basic Setup

1. **Enable auto_explain in PostgreSQL**:
```sql
ALTER SYSTEM SET shared_preload_libraries = 'auto_explain';
ALTER SYSTEM SET auto_explain.log_min_duration = 100;
ALTER SYSTEM SET auto_explain.log_analyze = true;
ALTER SYSTEM SET auto_explain.log_format = 'json';
SELECT pg_reload_conf();
```

2. **Configure the collector**:
```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    databases: [postgres]
    
  autoexplain:
    log_path: /var/log/postgresql/postgresql.log
    log_format: json
    
  ash:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    database: postgres
    collection_interval: 1s

processors:
  memory_limiter:
    limit_percentage: 80
    
exporters:
  prometheus:
    endpoint: 0.0.0.0:8888

service:
  pipelines:
    metrics:
      receivers: [postgresql, autoexplain, ash]
      processors: [memory_limiter]
      exporters: [prometheus]
```

3. **Run the collector**:
```bash
otelcol --config=config.yaml
```

## Configuration Examples

### Plan Intelligence Configuration

```yaml
receivers:
  autoexplain:
    log_path: /var/log/postgresql/postgresql.log
    log_format: json
    
    plan_collection:
      enabled: true
      min_duration: 100ms
      max_plans_per_query: 10
      
      regression_detection:
        enabled: true
        performance_degradation_threshold: 0.2
        statistical_confidence: 0.95
    
    plan_anonymization:
      enabled: true
      anonymize_filters: true
      sensitive_patterns: [email, ssn, credit_card]
```

### ASH Configuration

```yaml
receivers:
  ash:
    collection_interval: 1s
    
    sampling:
      enabled: true
      sample_rate: 1.0
      adaptive_sampling: true
      long_running_threshold: 10s
    
    storage:
      buffer_size: 3600
      aggregation_windows: [1m, 5m, 15m, 1h]
    
    analysis:
      wait_event_analysis: true
      blocking_analysis: true
      anomaly_detection: true
```

## Metrics Reference

### Plan Intelligence Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `db.postgresql.query.plan_time` | Gauge | Time spent planning queries |
| `db.postgresql.query.exec_time` | Gauge | Query execution time |
| `db.postgresql.plan.changes` | Counter | Number of plan changes detected |
| `db.postgresql.plan.regression` | Gauge | Plan regression severity (0-1) |

### ASH Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `postgresql.ash.sessions.count` | Gauge | Active sessions by state |
| `postgresql.ash.wait_events.count` | Gauge | Sessions by wait event |
| `postgresql.ash.blocking_sessions.count` | Gauge | Number of blocking sessions |
| `postgresql.ash.query.active_count` | Gauge | Active sessions per query |

## Advanced Features

### Wait Event Analysis

The wait analysis processor categorizes wait events:

```yaml
processors:
  waitanalysis:
    patterns:
      - name: lock_waits
        event_types: ["Lock"]
        category: "Concurrency"
        severity: "warning"
    
    alert_rules:
      - name: excessive_lock_waits
        condition: "wait_time > 5s AND event_type = 'Lock'"
        threshold: 10
        window: 1m
```

### Adaptive Sampling

Automatically adjusts collection based on load:

```yaml
processors:
  adaptivesampler:
    rules:
      - name: plan_regressions
        conditions:
          - attribute: event_type
            value: plan_regression
        sample_rate: 1.0  # Always collect
        
      - name: slow_queries
        conditions:
          - attribute: mean_exec_time_ms
            operator: gt
            value: 1000
        sample_rate: 0.8
```

## Deployment

### Docker

```bash
docker run -v $(pwd)/config.yaml:/etc/otel/config.yaml \
  -v /var/log/postgresql:/var/log/postgresql:ro \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/otel/config.yaml
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: db-intelligence-collector
spec:
  template:
    spec:
      containers:
      - name: collector
        image: otel/opentelemetry-collector-contrib:latest
        volumeMounts:
        - name: config
          mountPath: /etc/otel
        - name: pg-logs
          mountPath: /var/log/postgresql
          readOnly: true
```

### Systemd

```ini
[Unit]
Description=Database Intelligence Collector
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/otelcol --config=/etc/otel/config.yaml
Restart=on-failure
User=otel

[Install]
WantedBy=multi-user.target
```

## Performance Considerations

### Resource Requirements

- **Memory**: 100-500MB depending on configuration
- **CPU**: < 2% typical overhead
- **Disk**: Minimal (log reading only)
- **Network**: 1 connection per receiver

### Optimization Tips

1. **Sampling**: Start with lower sample rates in production
2. **Retention**: Adjust buffer sizes based on memory constraints
3. **Filtering**: Use min_duration to reduce plan collection
4. **Batching**: Configure appropriate batch sizes

## Security

### Best Practices

1. **Use Read-Only Credentials**: Create dedicated monitoring user
2. **Enable Anonymization**: Always anonymize plans in production
3. **Secure Connections**: Use SSL/TLS for database connections
4. **Access Control**: Restrict collector endpoints

### Monitoring User Setup

```sql
CREATE USER monitoring WITH PASSWORD 'secure_password';
GRANT pg_monitor TO monitoring;
GRANT SELECT ON pg_stat_statements TO monitoring;
```

## Troubleshooting

### Common Issues

1. **No Metrics Collected**
   - Check receiver configuration
   - Verify database connectivity
   - Review collector logs

2. **High Memory Usage**
   - Reduce buffer sizes
   - Enable adaptive sampling
   - Lower retention periods

3. **Missing Plans**
   - Verify auto_explain is loaded
   - Check log file permissions
   - Review min_duration setting

### Debug Mode

```yaml
service:
  telemetry:
    logs:
      level: debug
      encoding: json
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/yourusername/db-otel.git

# Install dependencies
cd db-otel
go mod download

# Build
go build -o otelcol ./cmd/otelcol

# Test
go test ./...
```

## Documentation

- [Receiver Documentation](docs/receivers/)
  - [AutoExplain Receiver](docs/receivers/autoexplain-receiver.md)
  - [ASH Receiver](docs/receivers/ash-receiver.md)
- [Processor Documentation](docs/processors/)
  - [Wait Analysis Processor](docs/processors/waitanalysis-processor.md)
- [Architecture](docs/architecture/)
  - [Plan Intelligence](docs/architecture/plan-intelligence.md)
  - [ASH Implementation](docs/architecture/ash-implementation.md)

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- OpenTelemetry Community
- PostgreSQL Community
- Inspired by Oracle ASH and OEM