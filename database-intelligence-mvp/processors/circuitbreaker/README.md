# Circuit Breaker Processor

A custom OpenTelemetry Collector processor that provides a circuit breaker pattern to protect databases from being overloaded by the collector.

## Features

- **Global and Per-Database States**: The circuit breaker can operate globally or on a per-database basis.
- **Adaptive Timeouts**: Dynamically adjusts request timeouts based on recent performance.
- **Resource Monitoring**: Opens the circuit if memory or CPU usage exceeds configured thresholds.
- **Concurrency Control**: Limits the number of concurrent requests to the database.
- **New Relic Integration**: Detects New Relic-specific errors (e.g., cardinality limits) and opens the circuit.

## Configuration

```yaml
processors:
  circuit_breaker:
    failure_threshold: 5
    success_threshold: 3
    open_state_timeout: 30s
    max_concurrent_requests: 100
    base_timeout: 5s
    max_timeout: 30s
    enable_adaptive_timeout: true
    health_check_interval: 10s
    memory_threshold_mb: 512
    cpu_threshold_percent: 80.0
```

## Usage in Collector

```yaml
service:
  pipelines:
    logs:
      processors:
        - memory_limiter
        - circuit_breaker
        - batch
```

## Building

To use this processor in a custom collector build:

1. Add to your `go.mod`:
```go
require github.com/newrelic/database-intelligence-mvp/processors/circuitbreaker v1.0.0
```

2. Import in your main.go:
```go
import "github.com/newrelic/database-intelligence-mvp/processors/circuitbreaker"
```

3. Register the factory:
```go
factories.Processors[circuitbreaker.GetType()] = circuitbreaker.NewFactory()
```
