# Custom Components Design for Enhanced Database Intelligence

## Overview

This document details the design and implementation of custom OpenTelemetry receivers and processors for advanced database intelligence capabilities. These components extend the standard OTel framework while maintaining compatibility with the collector's architecture.

## Custom Receivers

### Enhanced SQL Receiver

The Enhanced SQL Receiver extends standard database metric collection with deep query performance insights.

#### Architecture

```go
type EnhancedSQLReceiver struct {
    BaseReceiver
    queryAnalyzer    *QueryAnalyzer
    planCollector    *PlanCollector
    waitEventTracker *WaitEventTracker
    lockAnalyzer     *LockAnalyzer
}
```

#### Key Features

1. **Query Performance Analytics**
   - Captures query execution statistics beyond basic metrics
   - Tracks query patterns and fingerprints
   - Monitors query plan changes over time
   - Identifies query regressions

2. **Execution Plan Collection**
   - Retrieves and parses execution plans
   - Calculates plan stability scores
   - Detects plan regressions
   - Stores plan history for comparison

3. **Wait Event Analysis**
   - Samples active queries for wait events
   - Categorizes waits (I/O, Lock, CPU, Network)
   - Builds wait event profiles
   - Correlates waits with queries

4. **Lock Dependency Detection**
   - Identifies blocking chains
   - Maps lock dependencies
   - Detects deadlock risks
   - Tracks lock hold times

#### Configuration Schema

```yaml
receivers:
  enhancedsql:
    # Connection settings
    endpoint: "postgresql://host:port/database"
    username: "${DB_USERNAME}"
    password: "${DB_PASSWORD}"
    
    # Collection settings
    collection_interval: 10s
    query_timeout: 5s
    max_concurrent_queries: 10
    
    # Feature toggles
    features:
      query_stats:
        enabled: true
        top_n_queries: 100
        min_execution_time: 100ms
        capture_bind_parameters: false
        
      execution_plans:
        enabled: true
        capture_actual_plans: false
        plan_cache_size: 1000
        regression_threshold: 1.5
        
      wait_events:
        enabled: true
        sampling_rate: 1.0
        event_categories:
          - io
          - lock
          - cpu
          - network
          
      lock_analysis:
        enabled: true
        blocking_threshold: 1s
        deadlock_detection: true
        
    # Database-specific settings
    postgresql:
      extensions_required:
        - pg_stat_statements
        - pg_querylens  # Custom extension for plan tracking
      use_prepared_statements: true
      
    mysql:
      performance_schema_required: true
      sys_schema_required: true
```

#### Metrics Produced

```yaml
# Query performance metrics
postgresql.query.execution:
  type: histogram
  unit: ms
  attributes:
    - query.fingerprint
    - query.database
    - query.user
    - query.application

postgresql.query.calls:
  type: sum
  attributes:
    - query.fingerprint

postgresql.query.rows:
  type: histogram
  attributes:
    - query.fingerprint
    - query.operation  # SELECT, INSERT, UPDATE, DELETE

# Plan metrics
postgresql.plan.cost:
  type: gauge
  attributes:
    - query.fingerprint
    - plan.hash

postgresql.plan.changes:
  type: sum
  attributes:
    - query.fingerprint
    - change.type  # improvement, regression, neutral

postgresql.plan.regression_score:
  type: gauge
  attributes:
    - query.fingerprint
    - plan.hash

# Wait event metrics
postgresql.wait.time:
  type: histogram
  unit: ms
  attributes:
    - wait.event_type
    - wait.event_name
    - query.fingerprint

# Lock metrics
postgresql.locks.blocking_duration:
  type: histogram
  unit: ms
  attributes:
    - lock.type
    - lock.mode

postgresql.locks.chain_depth:
  type: histogram
  attributes:
    - lock.type
```

### Active Session History (ASH) Receiver

The ASH Receiver implements database session sampling for real-time performance analysis.

#### Architecture

```go
type ASHReceiver struct {
    BaseReceiver
    sampler         *SessionSampler
    circularBuffer  *CircularBuffer
    analyzer        *SessionAnalyzer
    eventDetector   *AnomalyDetector
}
```

#### Key Features

1. **High-Frequency Session Sampling**
   - 1-second sampling interval
   - Minimal overhead design
   - Circular buffer storage
   - Configurable retention

2. **Session State Analysis**
   - Active vs idle detection
   - Query association
   - Wait event capture
   - Resource usage tracking

3. **Blocking Chain Detection**
   - Real-time blocking analysis
   - Multi-level chain tracking
   - Root blocker identification
   - Victim impact assessment

4. **Historical Analysis**
   - Time-based aggregations
   - Trend detection
   - Anomaly identification
   - Performance baselines

#### Configuration Schema

```yaml
receivers:
  ash:
    # Connection settings
    endpoint: "${DB_ENDPOINT}"
    credentials: "${DB_CREDENTIALS}"
    
    # Sampling configuration
    sampling:
      interval: 1s
      jitter: 100ms  # Prevent thundering herd
      timeout: 500ms
      max_sessions: 1000
      
    # Storage settings
    retention:
      in_memory: 1h
      on_disk: 24h
      compression: true
      
    # Analysis features
    features:
      session_sampling:
        enabled: true
        include_idle: false
        capture_sql: true
        capture_plan: false
        
      wait_analysis:
        enabled: true
        categorization: true
        correlation: true
        
      blocking_analysis:
        enabled: true
        min_blocking_duration: 100ms
        max_chain_depth: 10
        
      anomaly_detection:
        enabled: true
        algorithms:
          - sudden_spike
          - gradual_drift
          - pattern_break
```

#### Data Model

```go
type SessionSample struct {
    Timestamp    time.Time
    SessionID    string
    State        SessionState
    Query        *QueryInfo
    WaitEvent    *WaitEvent
    Resources    *ResourceUsage
    BlockingInfo *BlockingInfo
}

type QueryInfo struct {
    SQL         string
    Fingerprint string
    StartTime   time.Time
    PlanID      string
}

type WaitEvent struct {
    Type     string
    Name     string
    Duration time.Duration
}

type BlockingInfo struct {
    BlockedBy   string
    BlockingSQL string
    ChainDepth  int
    RootBlocker string
}
```

#### Metrics and Events

```yaml
# Session metrics
database.ash.sessions.active:
  type: gauge
  attributes:
    - session.state
    - session.wait_class

database.ash.sessions.distribution:
  type: histogram
  attributes:
    - session.state

# Wait metrics  
database.ash.wait.time:
  type: histogram
  unit: ms
  attributes:
    - wait.class
    - wait.event

database.ash.wait.count:
  type: sum
  attributes:
    - wait.class

# Blocking metrics
database.ash.blocking.chains:
  type: gauge
  attributes:
    - chain.depth

database.ash.blocking.duration:
  type: histogram
  unit: ms

# Events (sent as logs)
database.ash.blocking_detected:
  severity: WARNING
  attributes:
    - blocker.session_id
    - blocker.query
    - blocked.session_ids
    - chain.depth
    - detection.time
```

## Custom Processors

### Processor Pipeline Architecture

The seven custom processors form an intelligent data processing pipeline:

```
┌─────────────────┐     ┌──────────────┐     ┌────────────────────┐
│ AdaptiveSampler │────▶│CircuitBreaker│────▶│PlanAttributeExtract│
└─────────────────┘     └──────────────┘     └────────────────────┘
                                                        │
                                                        ▼
┌──────────────┐     ┌─────────────┐     ┌─────────────────────┐
│QueryCorrelator│◀───│NRErrorMonitor│◀────│    Verification     │
└──────────────┘     └─────────────┘     └─────────────────────┘
                            ▲
                            │
                     ┌──────────────┐
                     │ CostControl  │
                     └──────────────┘
```

### 1. Adaptive Sampler Processor

Dynamically adjusts sampling rates based on metric importance and system load.

#### Design

```go
type AdaptiveSamplerProcessor struct {
    BaseProcessor
    rules          []SamplingRule
    loadMonitor    *LoadMonitor
    budgetTracker  *BudgetTracker
    decisionCache  *LRUCache
}

type SamplingRule struct {
    MetricPattern   string
    BaseRate        float64
    ImportanceScore float64
    SpikeDetection  *SpikeDetectionConfig
    Conditions      []Condition
}
```

#### Algorithm

```python
def calculate_sampling_rate(metric, rule, current_load):
    base_rate = rule.base_rate
    
    # Adjust for system load
    load_factor = 1.0 - (current_load / 100.0) * 0.5
    
    # Spike detection adjustment
    if spike_detected(metric):
        spike_factor = rule.spike_config.increase_rate
    else:
        spike_factor = 1.0
    
    # Importance weighting
    importance_factor = rule.importance_score
    
    # Budget constraints
    budget_factor = get_budget_multiplier()
    
    final_rate = min(1.0, 
        base_rate * load_factor * spike_factor * 
        importance_factor * budget_factor
    )
    
    return final_rate
```

### 2. Circuit Breaker Processor

Protects the database from monitoring overhead by temporarily disabling collection.

#### States

```
┌─────────┐  threshold   ┌──────┐  timeout  ┌───────────┐
│ CLOSED  │─────────────▶│ OPEN │──────────▶│ HALF_OPEN │
└─────────┘              └──────┘           └───────────┘
     ▲                                            │
     └────────────────success─────────────────────┘
```

#### Implementation

```go
type CircuitBreakerProcessor struct {
    state           State
    failureCount    int64
    lastFailureTime time.Time
    halfOpenTests   int
    
    thresholds      Thresholds
    metrics         *CircuitMetrics
}

type Thresholds struct {
    CPUPercent      float64
    QueryTimeMs     int64
    ErrorRate       float64
    FailureCount    int
    SuccessCount    int
}
```

### 3. Plan Attribute Extractor Processor

Enriches metrics with query plan intelligence.

#### Features

```go
type PlanAttributeExtractor struct {
    planCache       *PlanCache
    planAnalyzer    *PlanAnalyzer
    regressionDet   *RegressionDetector
    attributes      []string
}

func (p *PlanAttributeExtractor) Extract(metric pmetric.Metric) {
    if plan := p.getPlan(metric); plan != nil {
        // Extract cost metrics
        metric.Attributes().PutDouble("plan.total_cost", plan.TotalCost)
        metric.Attributes().PutInt("plan.rows_estimate", plan.RowsEstimate)
        
        // Detect regressions
        if regression := p.detectRegression(plan); regression != nil {
            metric.Attributes().PutBool("plan.regression", true)
            metric.Attributes().PutDouble("plan.regression_score", 
                regression.Score)
        }
        
        // Plan stability
        stability := p.calculateStability(plan.QueryID)
        metric.Attributes().PutDouble("plan.stability_score", stability)
    }
}
```

### 4. Verification Processor

Ensures data quality, security, and compliance.

#### Capabilities

```go
type VerificationProcessor struct {
    piiDetector      *PIIDetector
    schemaValidator  *SchemaValidator
    cardinalityGuard *CardinalityGuard
    qualityChecker   *QualityChecker
}

// PII Detection patterns
var piiPatterns = []PIIPattern{
    {Name: "SSN", Regex: `\b\d{3}-\d{2}-\d{4}\b`},
    {Name: "Email", Regex: `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`},
    {Name: "CreditCard", Regex: `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`},
}

// Cardinality limits
type CardinalityLimits struct {
    MaxSeriesPerMetric     int
    MaxAttributesPerMetric int
    MaxUniqueValues        int
    ActionOnExceed         CardinalityAction
}
```

### 5. Cost Control Processor

Manages telemetry volume to stay within budget constraints.

#### Priority System

```go
type CostControlProcessor struct {
    budget          *Budget
    priorityEngine  *PriorityEngine
    dropStrategy    DropStrategy
    costCalculator  *CostCalculator
}

type Priority int
const (
    Critical Priority = iota
    High
    Medium  
    Low
)

type PriorityRule struct {
    Pattern  string
    Priority Priority
    Rationale string
}

func (c *CostControlProcessor) Process(metrics pmetric.Metrics) {
    currentCost := c.calculateCost(metrics)
    
    if currentCost > c.budget.Limit {
        // Sort by priority
        prioritized := c.priorityEngine.Prioritize(metrics)
        
        // Drop lowest priority until within budget
        for currentCost > c.budget.Limit && len(prioritized) > 0 {
            dropped := c.dropLowestPriority(prioritized)
            currentCost -= c.calculateCost(dropped)
        }
    }
}
```

### 6. NR Error Monitor Processor

Monitors New Relic integration health and surfaces issues.

#### Monitoring Capabilities

```go
type NRErrorMonitorProcessor struct {
    errorDetector   *ErrorDetector
    metricsEmitter  *MetricsEmitter
    alertManager    *AlertManager
    errorPatterns   []ErrorPattern
}

type ErrorPattern struct {
    Type        ErrorType
    Pattern     string
    Severity    Severity
    Action      RemediationAction
}

// Error types monitored
const (
    IntegrationError ErrorType = iota
    RateLimitError
    SchemaViolation
    CardinalityLimit
    AuthenticationError
)

// Remediation actions
type RemediationAction struct {
    Type        ActionType
    Parameters  map[string]interface{}
    Retry       bool
    BackoffMs   int
}
```

### 7. Query Correlator Processor

Links related queries and transactions for holistic analysis.

#### Correlation Logic

```go
type QueryCorrelatorProcessor struct {
    correlationEngine *CorrelationEngine
    sessionTracker    *SessionTracker
    traceBuilder      *TraceBuilder
    windowSize        time.Duration
}

type CorrelationKey struct {
    SessionID       string
    TransactionID   string
    ApplicationName string
    UserID          string
}

func (q *QueryCorrelatorProcessor) Correlate(metric pmetric.Metric) {
    key := q.extractCorrelationKey(metric)
    
    // Find related metrics
    related := q.correlationEngine.FindRelated(key, q.windowSize)
    
    // Build correlation graph
    graph := q.buildCorrelationGraph(metric, related)
    
    // Add correlation attributes
    metric.Attributes().PutString("correlation.id", graph.ID)
    metric.Attributes().PutInt("correlation.depth", graph.Depth)
    metric.Attributes().PutString("correlation.root", graph.RootQuery)
    
    // Optionally emit as trace
    if q.traceBuilder.Enabled {
        trace := q.traceBuilder.BuildTrace(graph)
        q.emitTrace(trace)
    }
}
```

## Integration with OTel Collector

### Factory Pattern

```go
// Receiver factory
func NewEnhancedSQLReceiverFactory() receiver.Factory {
    return receiver.NewFactory(
        typeStr,
        createDefaultConfig,
        receiver.WithMetrics(createMetricsReceiver, stability),
    )
}

// Processor factory  
func NewAdaptiveSamplerProcessorFactory() processor.Factory {
    return processor.NewFactory(
        typeStr,
        createDefaultConfig,
        processor.WithMetrics(createMetricsProcessor, stability),
    )
}
```

### Configuration Integration

```yaml
# Collector build configuration
receivers:
  - gomod: github.com/db-otel/receivers/enhancedsqlreceiver v1.0.0
  - gomod: github.com/db-otel/receivers/ashreceiver v1.0.0

processors:
  - gomod: github.com/db-otel/processors/adaptivesampler v1.0.0
  - gomod: github.com/db-otel/processors/circuitbreaker v1.0.0
  - gomod: github.com/db-otel/processors/planattributeextractor v1.0.0
  - gomod: github.com/db-otel/processors/verification v1.0.0
  - gomod: github.com/db-otel/processors/costcontrol v1.0.0
  - gomod: github.com/db-otel/processors/nrerrormonitor v1.0.0
  - gomod: github.com/db-otel/processors/querycorrelator v1.0.0
```

## Performance Considerations

### Resource Usage

| Component | CPU | Memory | Network |
|-----------|-----|--------|---------|
| EnhancedSQL Receiver | Medium | High (plan cache) | Low |
| ASH Receiver | Low | Medium (buffer) | Low |
| Adaptive Sampler | Low | Low | None |
| Circuit Breaker | Minimal | Minimal | None |
| Plan Extractor | Medium | High (cache) | None |
| Verification | Medium | Low | None |
| Cost Control | Low | Low | None |
| NR Monitor | Low | Low | Low |
| Correlator | Medium | Medium | None |

### Optimization Strategies

1. **Caching**
   - LRU caches for plans, queries, decisions
   - TTL-based expiration
   - Size limits

2. **Batching**
   - Bulk operations for efficiency
   - Configurable batch sizes
   - Timeout-based flushing

3. **Concurrency**
   - Worker pools for parallel processing
   - Lock-free data structures where possible
   - Careful synchronization

4. **Memory Management**
   - Object pooling for frequent allocations
   - Circular buffers for ASH data
   - Periodic cleanup routines

## Testing Strategy

### Unit Tests
- Component isolation
- Mock dependencies
- Edge case coverage

### Integration Tests
- Full pipeline testing
- Database connectivity
- New Relic integration

### Performance Tests
- Load testing
- Memory profiling
- CPU profiling

### Chaos Testing
- Database failures
- Network issues
- Resource exhaustion

## Summary

The custom components provide advanced database intelligence capabilities while maintaining OTel compatibility. They can be selectively enabled based on requirements, ensuring flexibility and performance optimization.