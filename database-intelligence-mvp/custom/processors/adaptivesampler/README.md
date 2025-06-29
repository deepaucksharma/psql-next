# Adaptive Sampler Processor

## Overview

The Adaptive Sampler is a custom OpenTelemetry processor that provides intelligent sampling based on query performance characteristics. It fills the gap where OTEL's standard probabilistic sampler cannot adapt based on metric values.

## Why This Processor?

Standard OTEL sampling is probabilistic and static. Database monitoring needs dynamic sampling that:
- Always captures slow queries
- Reduces volume for fast, repetitive queries  
- Adapts to error conditions
- Preserves important signals while managing costs

## Features

- **Rule-based sampling**: Define conditions and sampling rates
- **Performance-aware**: Higher sampling for slow queries
- **Error detection**: Always sample errors and failures
- **Minimal overhead**: Efficient implementation
- **OTEL-native**: Follows processor interface standards

## Configuration

```yaml
processors:
  adaptive_sampler:
    # Default sampling rate (percentage)
    default_sampling_rate: 10.0
    
    # Sampling rules (evaluated in order)
    rules:
      - name: "errors"
        condition: "severity == 'ERROR'"
        sampling_rate: 100.0
        
      - name: "slow_queries"  
        condition: "mean_exec_time > 1000"  # ms
        sampling_rate: 100.0
        
      - name: "moderate_queries"
        condition: "mean_exec_time > 100"
        sampling_rate: 50.0
```

## Supported Conditions

Simple condition syntax:
- `mean_exec_time > 1000` - Execution time over 1 second
- `severity == 'ERROR'` - Error severity
- `has_error == true` - Error flag present

## Usage Example

```yaml
service:
  pipelines:
    metrics/queries:
      receivers: [sqlquery]
      processors: 
        - memory_limiter
        - adaptive_sampler  # Apply intelligent sampling
        - batch
      exporters: [otlp]
```

## Implementation Details

The processor:
1. Evaluates rules in order for each metric/log
2. Applies first matching rule's sampling rate
3. Uses default rate if no rules match
4. Preserves all attributes and metadata

## Metrics

The processor exposes:
- `adaptive_sampler_decisions_total` - Sampling decisions made
- `adaptive_sampler_dropped_total` - Items dropped
- `adaptive_sampler_sampled_total` - Items sampled

## Development

```go
// Core interface implementation
type simpleAdaptiveSampler struct {
    logger      *zap.Logger
    config      *Config
    nextMetrics consumer.Metrics
}

func (p *simpleAdaptiveSampler) ConsumeMetrics(
    ctx context.Context, 
    md pmetric.Metrics,
) error {
    // Apply sampling logic
    // Forward to next consumer
}
```

## Testing

```bash
# Run unit tests
go test ./processors/adaptivesampler/...

# Test configuration
go run ./processors/adaptivesampler/validate_config.go
```

## Best Practices

1. **Order rules carefully** - First match wins
2. **Set appropriate defaults** - Not too low
3. **Monitor sampling rates** - Ensure important data isn't dropped
4. **Test rules thoroughly** - Validate conditions work as expected