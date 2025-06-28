# Verification Processor

A custom OpenTelemetry Collector processor that provides real-time feedback on data quality and integration health.

## Features

- **Periodic Health Checks**: Runs periodic checks on the collector's health.
- **Data Freshness**: Monitors the data freshness and alerts if the data is stale.
- **Entity Correlation**: Verifies the entity correlation rate and alerts if it's low.
- **Query Normalization**: Verifies the query normalization rate and alerts if it's low.
- **Feedback as Logs**: Exports feedback events as logs for further analysis.

## Configuration

```yaml
processors:
  verification:
    enable_periodic_verification: true
    verification_interval: 5m
    data_freshness_threshold: 10m
    min_entity_correlation_rate: 0.8
    min_normalization_rate: 0.9
    require_entity_synthesis: true
    export_feedback_as_logs: true
```

## Usage in Collector

```yaml
service:
  pipelines:
    logs:
      processors:
        - memory_limiter
        - verification
        - batch
```

## Building

To use this processor in a custom collector build:

1. Add to your `go.mod`:
```go
require github.com/newrelic/database-intelligence-mvp/processors/verification v1.0.0
```

2. Import in your main.go:
```go
import "github.com/newrelic/database-intelligence-mvp/processors/verification"
```

3. Register the factory:
```go
factories.Processors[verification.GetType()] = verification.NewFactory()
```
