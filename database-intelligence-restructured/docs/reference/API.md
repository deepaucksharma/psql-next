# API Reference

This document provides the API reference for custom components in Database Intelligence.

## Table of Contents
- [Receivers](#receivers)
- [Processors](#processors)
- [Exporters](#exporters)
- [Extensions](#extensions)

## Receivers

### ASH Receiver

The Active Session History (ASH) receiver collects real-time session data from PostgreSQL.

#### Configuration

```go
type Config struct {
    // Database connection string
    Datasource string `mapstructure:"datasource"`
    
    // Collection interval (default: 1s)
    CollectionInterval time.Duration `mapstructure:"collection_interval"`
    
    // Sampling configuration
    Sampling SamplingConfig `mapstructure:"sampling"`
    
    // Buffer size for samples (default: 10000)
    BufferSize int `mapstructure:"buffer_size"`
    
    // How long to retain samples (default: 1h)
    RetentionDuration time.Duration `mapstructure:"retention_duration"`
    
    // Aggregation windows
    AggregationWindows []time.Duration `mapstructure:"aggregation_windows"`
    
    // Feature flags
    EnableFeatureDetection  bool `mapstructure:"enable_feature_detection"`
    EnableWaitAnalysis      bool `mapstructure:"enable_wait_analysis"`
    EnableBlockingAnalysis  bool `mapstructure:"enable_blocking_analysis"`
    EnableAnomalyDetection  bool `mapstructure:"enable_anomaly_detection"`
    
    // Thresholds
    SlowQueryThresholdMs   int `mapstructure:"slow_query_threshold_ms"`
    BlockedSessionThreshold int `mapstructure:"blocked_session_threshold"`
}

type SamplingConfig struct {
    BaseRate               float64 `mapstructure:"base_rate"`
    MinRate                float64 `mapstructure:"min_rate"`
    MaxRate                float64 `mapstructure:"max_rate"`
    LowSessionThreshold    int     `mapstructure:"low_session_threshold"`
    HighSessionThreshold   int     `mapstructure:"high_session_threshold"`
    AlwaysSampleBlocked    bool    `mapstructure:"always_sample_blocked"`
    AlwaysSampleLongRunning bool   `mapstructure:"always_sample_long_running"`
    AlwaysSampleMaintenance bool   `mapstructure:"always_sample_maintenance"`
}
```

#### ASH Sample Structure

```go
type ASHSample struct {
    Timestamp       time.Time
    SessionID       string
    PID             int
    DatabaseName    string
    Username        string
    ApplicationName string
    ClientAddr      string
    BackendStart    time.Time
    XactStart       *time.Time
    QueryStart      *time.Time
    StateChange     time.Time
    State           string
    BackendType     string
    WaitEventType   *string
    WaitEvent       *string
    Query           *string
    QueryID         *string
    BlockingPID     int
    BlockingQuery   *string
}
```

#### Metrics Generated

- `db.ash.active_sessions` - Number of active sessions by state
- `db.ash.wait_events` - Wait event occurrences
- `db.ash.blocked_sessions` - Number of blocked sessions
- `db.ash.long_running_queries` - Queries exceeding threshold

### Enhanced SQL Receiver

Executes custom SQL queries to collect additional metrics.

#### Configuration

```go
type Config struct {
    Driver     string        `mapstructure:"driver"`
    Datasource string        `mapstructure:"datasource"`
    Queries    []QueryConfig `mapstructure:"queries"`
}

type QueryConfig struct {
    Name    string         `mapstructure:"name"`
    SQL     string         `mapstructure:"sql"`
    Metrics []MetricConfig `mapstructure:"metrics"`
}

type MetricConfig struct {
    MetricName       string   `mapstructure:"metric_name"`
    ValueColumn      string   `mapstructure:"value_column"`
    AttributeColumns []string `mapstructure:"attribute_columns"`
    ValueType        string   `mapstructure:"value_type"`
    DataType         string   `mapstructure:"data_type"`
}
```

### Kernel Metrics Receiver

Collects low-level system metrics.

#### Configuration

```go
type Config struct {
    CollectionInterval     time.Duration `mapstructure:"collection_interval"`
    EnableDiskMetrics      bool         `mapstructure:"enable_disk_metrics"`
    EnableNetworkMetrics   bool         `mapstructure:"enable_network_metrics"`
    EnableProcessMetrics   bool         `mapstructure:"enable_process_metrics"`
}
```

## Processors

### Adaptive Sampler

Dynamically adjusts sampling rate based on load.

#### Configuration

```go
type Config struct {
    SamplingPercentage      float64       `mapstructure:"sampling_percentage"`
    EvaluationInterval      time.Duration `mapstructure:"evaluation_interval"`
    DecisionWait            time.Duration `mapstructure:"decision_wait"`
    NumTraces               uint64        `mapstructure:"num_traces"`
    ExpectedNewTracesPerSec uint64        `mapstructure:"expected_new_traces_per_sec"`
    Policies                []PolicyConfig `mapstructure:"policies"`
}

type PolicyConfig struct {
    PolicyType         string  `mapstructure:"policy_type"`
    SamplingPercentage float64 `mapstructure:"sampling_percentage"`
}
```

#### Algorithm

```go
type AdaptiveAlgorithm interface {
    // Calculate new sampling rate based on current metrics
    CalculateSamplingRate(metrics Metrics) float64
    
    // Update algorithm state with observed data
    UpdateState(observedRate float64, droppedCount int64)
}

type Metrics struct {
    CurrentRate      float64
    TargetRate       float64
    DroppedCount     int64
    ProcessedCount   int64
    QueueLength      int
    MemoryUsage      float64
}
```

### Circuit Breaker

Protects against overload conditions.

#### Configuration

```go
type Config struct {
    FailureThreshold int           `mapstructure:"failure_threshold"`
    RecoveryTimeout  time.Duration `mapstructure:"recovery_timeout"`
    MetricsLimit     int           `mapstructure:"metrics_limit"`
}
```

#### States

```go
type State int

const (
    StateClosed State = iota  // Normal operation
    StateOpen                 // Rejecting requests
    StateHalfOpen            // Testing recovery
)
```

#### Interface

```go
type CircuitBreaker interface {
    // Check if request should be allowed
    Allow() bool
    
    // Record success
    RecordSuccess()
    
    // Record failure
    RecordFailure()
    
    // Get current state
    State() State
}
```

### Cost Control

Limits data points to control costs.

#### Configuration

```go
type Config struct {
    MaxDatapointsPerMinute int    `mapstructure:"max_datapoints_per_minute"`
    EnforcementMode        string `mapstructure:"enforcement_mode"` // "drop" or "sample"
    HighCardinalityDimensions []string `mapstructure:"high_cardinality_dimensions"`
}
```

### Plan Attribute Extractor

Extracts query execution plans.

#### Configuration

```go
type Config struct {
    Timeout           time.Duration `mapstructure:"timeout"`
    CacheSize         int          `mapstructure:"cache_size"`
    ExtractParameters bool         `mapstructure:"extract_parameters"`
}
```

#### Plan Structure

```go
type QueryPlan struct {
    PlanID        string
    PlanType      string // "Seq Scan", "Index Scan", etc.
    EstimatedCost float64
    EstimatedRows int64
    ActualTime    float64
    ActualRows    int64
    Buffers       BufferUsage
    Children      []QueryPlan
}

type BufferUsage struct {
    Shared    BufferMetrics
    Local     BufferMetrics
    Temp      BufferMetrics
}

type BufferMetrics struct {
    Hit     int64
    Read    int64
    Dirtied int64
    Written int64
}
```

### Query Correlator

Correlates related queries.

#### Configuration

```go
type Config struct {
    CorrelationWindow    time.Duration `mapstructure:"correlation_window"`
    MaxCorrelatedQueries int          `mapstructure:"max_correlated_queries"`
    QueryCategorizationConfig QueryCategorizationConfig `mapstructure:"query_categorization"`
}

type QueryCategorizationConfig struct {
    SlowQueryThresholdMs   int `mapstructure:"slow_query_threshold_ms"`
    FrequentQueryThreshold int `mapstructure:"frequent_query_threshold"`
}
```

### OHI Transform

Transforms metrics for OHI compatibility.

#### Configuration

```go
type Config struct {
    TransformRules []TransformRule `mapstructure:"transform_rules"`
}

type TransformRule struct {
    SourceMetric string            `mapstructure:"source_metric"`
    TargetEvent  string            `mapstructure:"target_event"`
    Mappings     map[string]string `mapstructure:"mappings"`
}
```

## Exporters

### NRI Exporter

Exports to New Relic Infrastructure format.

#### Configuration

```go
type Config struct {
    LicenseKey string       `mapstructure:"license_key"`
    Events     EventsConfig `mapstructure:"events"`
    Metrics    MetricsConfig `mapstructure:"metrics"`
}

type EventsConfig struct {
    Enabled bool `mapstructure:"enabled"`
}

type MetricsConfig struct {
    Enabled bool `mapstructure:"enabled"`
}
```

## Extensions

### PostgreSQL Query Extension

Provides query execution capabilities.

#### Configuration

```go
type Config struct {
    Datasource string `mapstructure:"datasource"`
}
```

#### Interface

```go
type QueryExecutor interface {
    // Execute query and return results
    Execute(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
    
    // Execute query returning single row
    QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
    
    // Get query plan
    ExplainQuery(ctx context.Context, query string) (*QueryPlan, error)
}
```

## Error Handling

All components follow consistent error handling:

```go
// Component errors
type ComponentError struct {
    Component string
    Operation string
    Err       error
}

func (e ComponentError) Error() string {
    return fmt.Sprintf("%s: %s failed: %v", e.Component, e.Operation, e.Err)
}

// Validation errors
type ValidationError struct {
    Field   string
    Message string
}

func (c *Config) Validate() error {
    var errs []ValidationError
    
    if c.Datasource == "" {
        errs = append(errs, ValidationError{
            Field:   "datasource",
            Message: "datasource is required",
        })
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("validation failed: %v", errs)
    }
    
    return nil
}
```

## Metrics Format

All metrics follow OpenTelemetry conventions:

```go
type Metric struct {
    Name        string
    Description string
    Unit        string
    Type        MetricType // Gauge, Counter, Histogram
    Attributes  map[string]interface{}
    Value       interface{}
    Timestamp   time.Time
}
```

## Context Propagation

All components support context for cancellation and tracing:

```go
func (r *receiver) Start(ctx context.Context, host component.Host) error {
    r.ctx = ctx
    go r.collect()
    return nil
}

func (r *receiver) collect() {
    ticker := time.NewTicker(r.config.CollectionInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-r.ctx.Done():
            return
        case <-ticker.C:
            r.scrapeMetrics()
        }
    }
}
```

## Logging

All components use OpenTelemetry's zap logger:

```go
import "go.uber.org/zap"

type receiver struct {
    logger *zap.Logger
    config *Config
}

func (r *receiver) Start(ctx context.Context, host component.Host) error {
    r.logger.Info("Starting receiver",
        zap.String("datasource", r.config.Datasource),
        zap.Duration("interval", r.config.CollectionInterval),
    )
    return nil
}
```

## Testing

Components include comprehensive test utilities:

```go
// Test factory
func TestFactory(t *testing.T) {
    factory := NewFactory()
    assert.NotNil(t, factory)
    
    cfg := factory.CreateDefaultConfig()
    assert.NoError(t, cfg.Validate())
}

// Mock components
type MockReceiver struct {
    mock.Mock
}

func (m *MockReceiver) Start(ctx context.Context, host component.Host) error {
    args := m.Called(ctx, host)
    return args.Error(0)
}
```