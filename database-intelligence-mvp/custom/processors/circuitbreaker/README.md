# Circuit Breaker Processor

## Overview

The Circuit Breaker is a custom OpenTelemetry processor that protects databases from monitoring overload. It implements the circuit breaker pattern to prevent cascading failures when databases are under stress.

## Why This Processor?

Standard OTEL doesn't provide database-aware protection mechanisms. This processor:
- Prevents monitoring from overwhelming struggling databases
- Automatically recovers when conditions improve
- Provides visibility into circuit state
- Protects both the database and the monitoring system

## Features

- **Three-state design**: Closed (normal), Open (blocking), Half-Open (testing)
- **Configurable thresholds**: Error rate and volume based
- **Automatic recovery**: Self-healing with configurable timings
- **Metrics exposure**: Track circuit state and decisions
- **Zero data loss**: Gracefully sheds load without crashes

## Configuration

```yaml
processors:
  circuit_breaker:
    # Error threshold to open circuit (percentage)
    error_threshold_percent: 50.0
    
    # Volume threshold (queries per second)
    volume_threshold_qps: 1000.0
    
    # How often to evaluate circuit state
    evaluation_interval: 30s
    
    # How long to keep circuit open
    break_duration: 5m
    
    # Requests to allow in half-open state
    half_open_requests: 10
```

## Circuit States

### Closed (Normal Operation)
- All requests pass through
- Monitors error rate and volume
- Transitions to Open if thresholds exceeded

### Open (Protection Mode)
- Blocks all requests
- Protects database from load
- Waits for break_duration before testing

### Half-Open (Recovery Testing)
- Allows limited requests through
- Tests if database has recovered
- Returns to Closed if successful, Open if not

## Usage Example

```yaml
service:
  pipelines:
    metrics/database:
      receivers: [postgresql]
      processors: 
        - memory_limiter
        - circuit_breaker  # Protect database
        - batch
      exporters: [otlp]
```

## Metrics

The processor exposes:
- `circuit_breaker.state` - Current state (0=closed, 1=open, 2=half-open)
- `circuit_breaker.requests.total` - Total requests
- `circuit_breaker.requests.allowed` - Requests allowed through
- `circuit_breaker.requests.blocked` - Requests blocked
- `circuit_breaker.transitions` - State transitions

## Implementation Details

```go
// State machine implementation
type State int

const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

// Evaluation logic
func (cb *circuitBreaker) evaluate() {
    errorRate := cb.errorCount / cb.requestCount * 100
    qps := cb.requestCount / duration.Seconds()
    
    if errorRate > cb.config.ErrorThresholdPercent ||
       qps > cb.config.VolumeThresholdQPS {
        cb.transitionToOpen()
    }
}
```

## Best Practices

1. **Set conservative thresholds** - Better to be cautious
2. **Monitor circuit state** - Alert on extended open states
3. **Test break duration** - Find optimal recovery time
4. **Use with memory_limiter** - Defense in depth
5. **Consider workload patterns** - Adjust for peak times

## Testing

```bash
# Run unit tests
go test ./processors/circuitbreaker/...

# Simulate high load
go run ./processors/circuitbreaker/load_test.go
```

## Troubleshooting

### Circuit Stuck Open
- Check database health
- Review error threshold
- Increase break duration

### Too Sensitive
- Increase error threshold
- Extend evaluation interval
- Check for transient errors

### Not Triggering
- Lower thresholds
- Check metric calculation
- Verify error detection