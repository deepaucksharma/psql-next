# Components Index

## Processors

### Core Processors
- `adaptivesampler` - Adaptive sampling based on load
- `circuitbreaker` - Circuit breaker for reliability  
- `costcontrol` - Cost control through data reduction
- `nrerrormonitor` - New Relic error monitoring
- `planattributeextractor` - Extract query plan attributes
- `querycorrelator` - Correlate queries across databases
- `verification` - Data verification processor

### Status
All processors have:
- ✓ go.mod files
- ✓ Test coverage
- ✓ Factory implementations

## Receivers

### Custom Receivers  
- `ashdatareceiver` - Active Session History data collection
- `autoexplainreceiver` - PostgreSQL auto_explain log parsing
- `kernelmetrics` - Kernel-level metrics collection

### Status
All receivers follow OTEL receiver patterns.

## Internal Packages

- `featuredetector` - Feature detection for databases
- `processor` - Shared processor utilities
- `queryselector` - Query selection logic

## Build

To build all components:
```bash
./scripts/building/build-collector.sh
```

## Testing

To test all components:
```bash
go test ./components/...
```
