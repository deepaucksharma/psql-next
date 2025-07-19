# Alert Manager Module

The Alert Manager module is responsible for receiving, processing, aggregating, and routing alerts from various components in the Database Intelligence system. It provides centralized alert management with deduplication, grouping, and multiple export options.

## Features

- **Alert Reception**: Receives alerts via OTLP (gRPC and HTTP)
- **Alert Processing**: Groups, deduplicates, and enriches alerts
- **Multi-Channel Export**: Sends alerts to webhooks, files, and Prometheus
- **Alert Aggregation**: Groups alerts by service, severity, type, and database
- **State Management**: Tracks alert states and timestamps
- **Flexible Routing**: Configurable alert routing and filtering

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ Anomaly Detector│     │ Query Analyzer  │     │ Other Modules   │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                         │
         └───────────────────────┴─────────────────────────┘
                                 │
                                 ▼
                    ┌────────────────────────┐
                    │   OTLP Receivers       │
                    │  (gRPC:8089, HTTP:4318)│
                    └────────────┬───────────┘
                                 │
                    ┌────────────▼───────────┐
                    │   Alert Processors     │
                    │ • Filter & Deduplicate │
                    │ • Transform & Enrich   │
                    │ • Group by Attributes  │
                    └────────────┬───────────┘
                                 │
              ┌──────────────────┼──────────────────┐
              │                  │                  │
     ┌────────▼────────┐ ┌──────▼──────┐ ┌────────▼────────┐
     │ Webhook Export  │ │ File Export │ │ Prometheus Exp  │
     │  (HTTP POST)    │ │   (JSON)    │ │  (Port 9091)    │
     └─────────────────┘ └─────────────┘ └─────────────────┘
```

## Quick Start

1. **Start the service**:
   ```bash
   make up
   ```

2. **Check health**:
   ```bash
   make health-check
   ```

3. **Send a test alert**:
   ```bash
   make send-test-alert
   ```

4. **View alerts**:
   ```bash
   make view-alerts
   ```

## Configuration

### Environment Variables

- `WEBHOOK_ENDPOINT`: URL for webhook notifications (default: `http://localhost:9999/alerts`)
- `ALERT_FILE_PATH`: Path for alert log files (default: `/var/log/alerts/alerts.json`)
- `OTEL_LOG_LEVEL`: Logging level (default: `info`)
- `UPSTREAM_OTLP_ENDPOINT`: Upstream OTLP endpoint for forwarding

### Alert Attributes

The alert manager expects alerts to have specific attributes:

```json
{
  "alert.severity": "critical|warning|info",
  "alert.type": "performance|availability|security|anomaly",
  "alert.timestamp": "ISO8601 timestamp",
  "service.name": "source service",
  "db.system": "mysql|postgresql|mongodb",
  "db.name": "database name"
}
```

## Endpoints

- **OTLP gRPC**: `localhost:8089` - Receive alerts via gRPC
- **OTLP HTTP**: `localhost:4318` - Receive alerts via HTTP
- **Prometheus**: `localhost:9091/metrics` - Alert metrics
- **Health Check**: `localhost:13134/health` - Service health

## Alert Processing Pipeline

1. **Reception**: Alerts received via OTLP protocols
2. **Filtering**: Only valid alert metrics are processed
3. **Transformation**: Alerts are enriched with metadata
4. **Grouping**: Alerts grouped by key attributes
5. **Export**: Alerts sent to configured destinations

## Webhook Format

Alerts sent to webhooks are in JSON format:

```json
{
  "resourceMetrics": [{
    "resource": {
      "attributes": [{
        "key": "service.name",
        "value": {"stringValue": "database-service"}
      }]
    },
    "scopeMetrics": [{
      "metrics": [{
        "name": "alert.database.high_cpu",
        "gauge": {
          "dataPoints": [{
            "asDouble": 95,
            "attributes": [{
              "key": "alert.severity",
              "value": {"stringValue": "critical"}
            }]
          }]
        }
      }]
    }]
  }]
}
```

## Available Commands

```bash
make help           # Show all available commands
make build          # Build the container
make up             # Start the service
make down           # Stop the service
make restart        # Restart the service
make logs           # View service logs
make shell          # Open shell in container
make test           # Run connectivity tests
make validate       # Validate configuration
make config-test    # Test configuration (dry-run)
make clean          # Clean up volumes
```

## Integration with Other Modules

### Sending Alerts to Alert Manager

Configure OTLP exporters in other modules:

```yaml
exporters:
  otlp/alerts:
    endpoint: alert-manager:8089
    tls:
      insecure: true
```

### Example Alert Metric

```yaml
metrics:
  - name: "alert.database.slow_query"
    type: gauge
    value: 1
    attributes:
      - alert.severity: "warning"
      - alert.type: "performance"
      - db.system: "mysql"
      - db.name: "production"
      - query.duration_ms: 5000
```

## Monitoring

Monitor the alert manager using Prometheus metrics:

```bash
# View all alert manager metrics
curl http://localhost:9091/metrics | grep alert_manager_

# Key metrics:
# - alert_manager_alerts_received_total
# - alert_manager_alerts_processed_total
# - alert_manager_alerts_exported_total
# - alert_manager_webhook_failures_total
```

## Troubleshooting

### No alerts being received
1. Check OTLP endpoints are accessible
2. Verify sender configuration
3. Check logs: `make logs`

### Webhook failures
1. Verify webhook endpoint is reachable
2. Check webhook response codes in logs
3. Test with local webhook: `make dev-webhook`

### High memory usage
1. Adjust memory limits in processor
2. Reduce batch size
3. Check for alert storms

## Development

### Testing Alert Processing

1. Start a test webhook receiver:
   ```bash
   make dev-webhook
   ```

2. Send test alerts:
   ```bash
   make send-test-alert
   ```

3. View processed alerts:
   ```bash
   make view-alerts
   ```

### Adding Custom Processors

Edit `config/collector.yaml` to add custom processing logic:

```yaml
processors:
  custom_processor:
    # Your custom configuration
```

## Best Practices

1. **Alert Naming**: Use consistent naming convention (e.g., `alert.{component}.{issue}`)
2. **Severity Levels**: Use standard severity levels (critical, warning, info)
3. **Deduplication**: Include unique identifiers in alert attributes
4. **Batching**: Configure appropriate batch sizes for performance
5. **Retention**: Set appropriate retention for alert files