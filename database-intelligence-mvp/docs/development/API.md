# API Reference

## Overview

This document provides a reference for the internal APIs and interfaces used in the Database Intelligence Collector.

## Custom Processors API

### Base Processor Interface

All custom processors implement the OpenTelemetry processor interface:

```go
type Processor interface {
    Start(ctx context.Context, host component.Host) error
    Shutdown(ctx context.Context) error
    Capabilities() consumer.Capabilities
}
```

### Plan Attribute Extractor

#### Configuration

```go
type Config struct {
    TimeoutMS            int                       `mapstructure:"timeout_ms"`
    ErrorMode           string                    `mapstructure:"error_mode"`
    PostgreSQLRules     PostgreSQLExtractionRules `mapstructure:"postgresql_rules"`
    MySQLRules          MySQLExtractionRules      `mapstructure:"mysql_rules"`
    HashConfig          HashGenerationConfig      `mapstructure:"hash_config"`
    EnableDebugLogging  bool                      `mapstructure:"enable_debug_logging"`
    SafeMode            bool                      `mapstructure:"safe_mode"`
}
```

#### Methods

```go
// ProcessLogs processes log entries to extract query plan attributes
func (p *planAttributeExtractor) ProcessLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error)

// extractPlanAttributes extracts attributes from query plans
func (p *planAttributeExtractor) extractPlanAttributes(record plog.LogRecord) error
```

### Adaptive Sampler

#### Configuration

```go
type Config struct {
    InMemoryOnly        bool             `mapstructure:"in_memory_only"`
    Rules               []SamplingRule   `mapstructure:"rules"`
    DefaultSamplingRate float64          `mapstructure:"default_sampling_rate"`
    CacheSize           int              `mapstructure:"cache_size"`
    CacheTTL            time.Duration    `mapstructure:"cache_ttl"`
}
```

#### Sampling Rule

```go
type SamplingRule struct {
    Name         string  `mapstructure:"name"`
    Condition    string  `mapstructure:"condition"`
    SamplingRate float64 `mapstructure:"sampling_rate"`
    Priority     int     `mapstructure:"priority"`
}
```

### Circuit Breaker

#### Configuration

```go
type Config struct {
    FailureThreshold   int           `mapstructure:"failure_threshold"`
    Timeout           time.Duration `mapstructure:"timeout"`
    HalfOpenRequests  int           `mapstructure:"half_open_requests"`
    Databases         []DatabaseConfig `mapstructure:"databases"`
}
```

#### State Management

```go
type CircuitState int

const (
    StateClosed CircuitState = iota
    StateOpen
    StateHalfOpen
)
```

### Verification Processor

#### Configuration

```go
type Config struct {
    PIIDetection     PIIDetectionConfig     `mapstructure:"pii_detection"`
    QualityChecks    QualityCheckConfig     `mapstructure:"quality_checks"`
    AutoTuning       AutoTuningConfig       `mapstructure:"auto_tuning"`
    CardinalityLimit int                    `mapstructure:"cardinality_limit"`
}
```

## Metrics API

### Internal Metrics

All processors expose internal metrics via OpenTelemetry's metrics API:

```go
// Create meter
meter := otel.Meter("database-intelligence-collector")

// Record metrics
recordsProcessed, _ := meter.Int64Counter("records_processed")
recordsProcessed.Add(ctx, 1, metric.WithAttributes(
    attribute.String("processor", "planattributeextractor"),
))
```

### Available Metrics

| Metric Name | Type | Description |
|------------|------|-------------|
| `otelcol_processor_accepted_metric_points` | Counter | Accepted metric points |
| `otelcol_processor_refused_metric_points` | Counter | Refused metric points |
| `otelcol_processor_dropped_metric_points` | Counter | Dropped metric points |
| `processor_batch_size` | Histogram | Batch sizes processed |
| `processor_processing_time` | Histogram | Processing duration |

## Extension Points

### Custom Receivers

To add a new receiver:

1. Implement the receiver interface
2. Register in `ocb-config.yaml`
3. Add configuration schema

```go
type ReceiverFactory interface {
    Type() component.Type
    CreateDefaultConfig() component.Config
    CreateMetricsReceiver(...) (receiver.Metrics, error)
}
```

### Custom Exporters

To add a new exporter:

1. Implement the exporter interface
2. Register in `ocb-config.yaml`
3. Add configuration schema

```go
type ExporterFactory interface {
    Type() component.Type
    CreateDefaultConfig() component.Config
    CreateMetricsExporter(...) (exporter.Metrics, error)
}
```

## Configuration API

### Environment Variable Expansion

All configuration values support environment variable expansion:

```yaml
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
```

### Configuration Validation

Each component implements validation:

```go
func (cfg *Config) Validate() error {
    if cfg.TimeoutMS <= 0 {
        return fmt.Errorf("timeout_ms must be positive")
    }
    return nil
}
```

## Health Check API

### Endpoints

- `/health` - Overall health status
- `/health/live` - Liveness probe
- `/health/ready` - Readiness probe

### Response Format

```json
{
  "status": "healthy",
  "components": {
    "postgresql_receiver": "healthy",
    "mysql_receiver": "healthy",
    "otlp_exporter": "healthy"
  },
  "timestamp": "2025-06-30T12:00:00Z"
}
```

## Telemetry API

### Logging

Using zap logger:

```go
logger := zap.NewLogger()
logger.Info("Processing batch",
    zap.Int("size", batchSize),
    zap.Duration("duration", processingTime),
)
```

### Tracing

Using OpenTelemetry tracing:

```go
tracer := otel.Tracer("processor")
ctx, span := tracer.Start(ctx, "process_batch")
defer span.End()
```

## Error Handling

### Error Types

```go
var (
    ErrInvalidConfig = errors.New("invalid configuration")
    ErrTimeout       = errors.New("operation timeout")
    ErrCircuitOpen   = errors.New("circuit breaker open")
)
```

### Error Propagation

Errors should be wrapped with context:

```go
if err != nil {
    return fmt.Errorf("failed to process batch: %w", err)
}
```

## Testing Utilities

### Mock Factories

```go
func NewMockProcessorFactory() processor.Factory {
    return processor.NewFactory(
        "mock",
        createDefaultConfig,
        processor.WithMetrics(createMetricsProcessor, stability.LevelStable),
    )
}
```

### Test Helpers

```go
// GenerateTestMetrics creates test metric data
func GenerateTestMetrics(count int) pmetric.Metrics

// GenerateTestLogs creates test log data
func GenerateTestLogs(count int) plog.Logs
```

## Version Compatibility

| Component | Min Version | Max Version |
|-----------|------------|-------------|
| OpenTelemetry Collector | 0.96.0 | 0.127.0 |
| Go | 1.21 | 1.22 |
| PostgreSQL | 12 | 16 |
| MySQL | 5.7 | 8.0 |