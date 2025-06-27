# PostgreSQL Unified Collector - Pure OpenTelemetry Solution

This is a pure OpenTelemetry implementation of the PostgreSQL Unified Collector, with no dependencies on New Relic Infrastructure Agent or OHI.

## Architecture

```
┌─────────────────┐      ┌──────────────────┐      ┌─────────────────┐
│   PostgreSQL    │      │ Unified Collector│      │ OTEL Collector  │
│   Database      │◄─────│                  │─────►│                 │
│                 │      │ - Slow Queries   │ OTLP │ - Receives      │
│ pg_stat_statements     │ - Blocking       │      │ - Processes     │
└─────────────────┘      │   Sessions       │      │ - Exports       │
                         └──────────────────┘      └────────┬────────┘
                                                            │
                              ┌─────────────────────────────┴────────┐
                              │                                      │
                         ┌────▼─────┐                        ┌──────▼──────┐
                         │Prometheus│                        │   Grafana   │
                         │          │◄───────────────────────│             │
                         │ Storage  │    PromQL Queries      │ Dashboards  │
                         └──────────┘                        └─────────────┘
```

## Features

- **Pure OTLP Output**: Metrics are sent using OpenTelemetry Protocol (OTLP)
- **No Vendor Lock-in**: Works with any OTLP-compatible backend
- **Prometheus Integration**: Metrics exposed in Prometheus format
- **Grafana Dashboards**: Pre-configured dashboards for visualization
- **Kubernetes Ready**: Full Kubernetes deployment manifests included

## Quick Start

### Docker Compose

1. **Start the stack:**
   ```bash
   docker-compose -f docker-compose-otel.yml up -d
   ```

2. **Access the services:**
   - Grafana: http://localhost:3000 (admin/admin)
   - Prometheus: http://localhost:9090
   - OTEL Collector Health: http://localhost:13133
   - Collector Health: http://localhost:8080/health

3. **Generate test data:**
   ```bash
   ./generate-slow-queries.sh
   ```

### Kubernetes

1. **Deploy to Kubernetes:**
   ```bash
   kubectl apply -f deployments/kubernetes/postgres-collector-otel.yaml
   ```

2. **Check deployment:**
   ```bash
   kubectl -n postgres-monitoring get pods
   kubectl -n postgres-monitoring logs -l app=postgres-collector
   ```

3. **Port forward to access:**
   ```bash
   # Prometheus metrics
   kubectl -n postgres-monitoring port-forward svc/otel-collector-prometheus 8889:8889
   ```

## Configuration

### Collector Configuration (config-otel.toml)

```toml
# PostgreSQL connection
connection_string = "postgres://user:pass@host:5432/db"
databases = ["db1", "db2"]

# Metric collection
[slow_queries]
enabled = true
min_duration_ms = 500
interval = 30

[blocking_sessions]
enabled = true
min_blocking_duration_ms = 1000
interval = 30

# OTLP output
[outputs.otlp]
enabled = true
endpoint = "http://otel-collector:4317"
timeout_seconds = 10
compression = "gzip"

[outputs.otlp.resource_attributes]
service.name = "postgresql-unified-collector"
service.version = "1.0.0"
db.system = "postgresql"
```

### OpenTelemetry Collector Configuration

The OTEL Collector receives metrics via OTLP and can export to multiple backends:

- **Prometheus**: Exposes metrics in Prometheus format
- **Logging**: Outputs metrics to stdout for debugging
- **OTLP**: Forward to other OTLP-compatible backends

## Metrics Collected

### Slow Query Metrics
- `postgresql_slow_query_avg_elapsed_time_ms` - Average execution time
- `postgresql_slow_query_execution_count` - Number of executions
- `postgresql_slow_query_avg_disk_reads` - Average disk reads
- `postgresql_slow_query_avg_disk_writes` - Average disk writes

### Blocking Session Metrics
- `postgresql_blocking_session_count` - Number of blocking sessions
- `postgresql_blocking_duration_ms` - Duration of blocks

### Labels/Attributes
- `database_name` - Database name
- `schema_name` - Schema name
- `query_text` - Sanitized query text
- `statement_type` - SQL statement type (SELECT, INSERT, etc.)
- `blocking_pid` - Process ID of blocking session
- `blocked_pid` - Process ID of blocked session

## Integration with Other Backends

### Export to Jaeger (Traces)
```yaml
exporters:
  jaeger:
    endpoint: jaeger-collector:14250
    tls:
      insecure: true
```

### Export to Elasticsearch
```yaml
exporters:
  elasticsearch:
    endpoints: ["https://elastic:9200"]
    index: postgresql-metrics
```

### Export to Cloud Providers
```yaml
# AWS CloudWatch
exporters:
  awsemf:
    region: us-east-1
    namespace: PostgreSQL/Metrics

# Google Cloud Monitoring
exporters:
  googlecloud:
    project: my-project
    
# Azure Monitor
exporters:
  azuremonitor:
    instrumentation_key: "your-key"
```

## Monitoring Multiple Databases

To monitor multiple PostgreSQL instances:

1. **Deploy multiple collectors:**
   ```yaml
   - name: DB1_CONNECTION_STRING
     value: "postgres://user:pass@db1:5432/mydb"
   - name: DB2_CONNECTION_STRING
     value: "postgres://user:pass@db2:5432/mydb"
   ```

2. **Use environment variable substitution in config**
3. **Add instance labels in OTLP attributes**

## Security Considerations

1. **Connection Strings**: Use Kubernetes secrets for database credentials
2. **TLS**: Enable TLS for OTLP communication in production
3. **Query Sanitization**: The collector automatically sanitizes queries to remove PII
4. **Network Policies**: Restrict network access between components

## Troubleshooting

### Check Collector Logs
```bash
docker-compose -f docker-compose-otel.yml logs postgres-collector
```

### Verify OTLP Connection
```bash
docker-compose -f docker-compose-otel.yml logs otel-collector | grep otlp
```

### Check Prometheus Metrics
```bash
curl http://localhost:8889/metrics | grep postgresql
```

### Debug OTEL Collector
Access zpages for debugging: http://localhost:55679/debug/tracez

## Production Deployment

### High Availability
- Deploy multiple OTEL Collector replicas
- Use persistent storage for Prometheus
- Configure proper resource limits

### Scaling
- Horizontal scaling of collectors for multiple databases
- Use OTEL Collector's load balancing exporter
- Implement sampling for high-volume environments

### Backup and Retention
- Configure Prometheus retention policies
- Export to long-term storage (S3, GCS, etc.)
- Set up alerting rules in Prometheus

## Contributing

The collector is designed to be extensible. To add new metrics:

1. Implement the metric collection in Rust
2. Add OTLP metric conversion
3. Update Grafana dashboards
4. Submit a pull request

## License

This project is licensed under the MIT License.