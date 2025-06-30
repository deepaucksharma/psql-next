# Custom Processors - Detailed Documentation

This document provides comprehensive documentation for the four custom processors in the Database Intelligence Collector.

## Table of Contents
1. [Adaptive Sampler](#adaptive-sampler)
2. [Circuit Breaker](#circuit-breaker)
3. [Plan Attribute Extractor](#plan-attribute-extractor)
4. [Verification Processor](#verification-processor)

---

## Adaptive Sampler

**Location**: `processors/adaptivesampler/`  
**Lines of Code**: 576  
**Status**: Production Ready

### Overview
The Adaptive Sampler provides intelligent, rule-based sampling of database metrics to reduce data volume while preserving important signals.

### Architecture
```go
type adaptiveSamplerProcessor struct {
    cfg                Config
    logger             *zap.Logger
    deduplicationCache *lru.Cache[string, time.Time]
    ruleLimiters       map[string]*rateLimiter
    stateMutex         sync.RWMutex
}
```

### Key Features

#### 1. Rule-Based Sampling
```yaml
rules:
  - name: slow_queries
    conditions:
      - attribute: duration_ms
        operator: gt
        value: 1000
    sample_rate: 1.0  # Always sample slow queries
    
  - name: high_cost_queries
    conditions:
      - attribute: total_cost
        operator: gt
        value: 10000
      - attribute: db.system
        operator: eq
        value: postgresql
    sample_rate: 0.5
```

#### 2. Expression Evaluation
Supports operators: `eq`, `ne`, `gt`, `lt`, `gte`, `lte`, `contains`, `regex`

```go
func evaluateCondition(condition Condition, attributes pcommon.Map) bool {
    switch condition.Operator {
    case "gt":
        return compareNumeric(attrValue, condition.Value, func(a, b float64) bool { return a > b })
    case "regex":
        return regexp.MustCompile(condition.Value).MatchString(attrStr)
    }
}
```

#### 3. Deduplication Cache
```go
// LRU cache prevents duplicate events
type DeduplicationConfig struct {
    Enabled      bool          `mapstructure:"enabled"`
    CacheSize    int           `mapstructure:"cache_size"`    // Default: 10000
    TTL          time.Duration `mapstructure:"ttl"`           // Default: 60s
    KeyFields    []string      `mapstructure:"key_fields"`    // Fields for cache key
}
```

#### 4. Rate Limiting
```go
type rateLimiter struct {
    rate       float64
    burst      int
    limiter    *rate.Limiter
    lastUpdate time.Time
}
```

### Configuration Reference

```yaml
adaptive_sampler:
  # Force in-memory mode (required for production)
  in_memory_only: true
  
  # Enable debug logging
  enable_debug_logging: false
  
  # Default sampling rate if no rules match
  default_sample_rate: 0.1
  
  # Environment-specific overrides
  environment_overrides:
    production:
      slow_query_threshold_ms: 2000
      max_records_per_second: 500
    staging:
      slow_query_threshold_ms: 500
      max_records_per_second: 2000
  
  # Sampling rules
  rules:
    - name: critical_database
      conditions:
        - attribute: db.name
          operator: eq
          value: "orders"
      sample_rate: 1.0
      
    - name: development_traffic
      conditions:
        - attribute: environment
          operator: eq
          value: "development"
      sample_rate: 0.01
  
  # Deduplication configuration
  deduplication:
    enabled: true
    cache_size: 10000
    ttl: 60s
    key_fields: ["query_hash", "db.name"]
```

### Metrics Exposed

```prometheus
# Total records processed
adaptive_sampler_records_processed_total

# Records dropped by reason
adaptive_sampler_records_dropped_total{reason="sampling|deduplication|rate_limit"}

# Cache hit rate
adaptive_sampler_cache_hit_rate

# Rule match counts
adaptive_sampler_rule_matches_total{rule="rule_name"}

# Current sampling rate
adaptive_sampler_current_rate{rule="rule_name"}
```

### Performance Characteristics
- **Memory**: O(cache_size) - typically 50-100MB
- **CPU**: O(n*m) where n=records, m=rules
- **Latency**: <1ms per record typical

---

## Circuit Breaker

**Location**: `processors/circuitbreaker/`  
**Lines of Code**: 922  
**Status**: Production Ready

### Overview
The Circuit Breaker protects databases from overload by automatically stopping metric collection when failures exceed thresholds.

### Architecture
```go
type circuitBreakerProcessor struct {
    config         Config
    logger         *zap.Logger
    circuits       map[string]*circuit
    mutex          sync.RWMutex
    healthChecker  *healthChecker
}

type circuit struct {
    state              State
    failures           int
    successfulRequests int
    lastFailureTime    time.Time
    lastTransitionTime time.Time
    mutex              sync.RWMutex
}
```

### State Machine
```
┌─────────┐  failures > threshold   ┌────────┐
│ Closed  │ ───────────────────────>│  Open  │
└─────────┘                         └────────┘
     ▲                                   │
     │ success                           │ timeout
     │                                   ▼
     └───────────────────────────── Half-Open
                  failure
```

### Key Features

#### 1. Per-Database Isolation
Each database has an independent circuit breaker:
```go
circuits map[string]*circuit  // key: database name
```

#### 2. Failure Detection
```go
// Failures that trigger circuit opening
- Connection timeouts
- Query errors
- High latency (>timeout_duration)
- Resource exhaustion
- New Relic API errors (NrIntegrationError)
```

#### 3. Automatic Recovery
```go
// Exponential backoff for recovery attempts
type RecoveryConfig struct {
    InitialInterval   time.Duration  // Default: 30s
    MaxInterval       time.Duration  // Default: 300s
    BackoffMultiplier float64        // Default: 2.0
}
```

#### 4. Resource-Based Triggers
```go
// Open circuit if resources exceeded
if memoryUsage > config.MemoryThreshold {
    circuit.Open("memory_pressure")
}
if cpuUsage > config.CPUThreshold {
    circuit.Open("cpu_pressure")
}
```

### Configuration Reference

```yaml
circuit_breaker:
  # Failure threshold before opening
  failure_threshold: 5
  
  # Timeout before trying half-open
  open_state_timeout: 60s
  
  # Requests to allow in half-open state
  half_open_requests: 3
  
  # Request timeout duration
  timeout_duration: 30s
  
  # Resource-based triggers
  resource_triggers:
    memory_threshold_percent: 80
    cpu_threshold_percent: 70
  
  # Per-database overrides
  database_configs:
    production_primary:
      failure_threshold: 10
      timeout_duration: 60s
    analytics:
      failure_threshold: 3
      timeout_duration: 10s
  
  # Recovery configuration
  recovery:
    initial_interval: 30s
    max_interval: 300s
    backoff_multiplier: 2.0
```

### Metrics Exposed

```prometheus
# Circuit breaker state (0=closed, 1=half-open, 2=open)
circuit_breaker_state{database="db_name", state="open|closed|half_open"}

# Total trips
circuit_breaker_trips_total{database="db_name", reason="failure|resource"}

# Recovery attempts
circuit_breaker_recovery_attempts_total{database="db_name"}

# Time in current state
circuit_breaker_state_duration_seconds{database="db_name", state="open"}

# Failure count
circuit_breaker_failures_total{database="db_name", type="timeout|error|resource"}
```

### Manual Control API

```bash
# Open circuit manually
curl -X POST http://localhost:13133/circuit_breaker/open \
  -d '{"database": "production_primary", "reason": "maintenance"}'

# Close circuit manually
curl -X POST http://localhost:13133/circuit_breaker/close \
  -d '{"database": "production_primary"}'

# Get circuit status
curl http://localhost:13133/circuit_breaker/status
```

### Performance Characteristics
- **Memory**: O(n) where n=number of databases
- **CPU**: Minimal - only state checks
- **Latency**: <0.1ms per check

---

## Plan Attribute Extractor

**Location**: `processors/planattributeextractor/`  
**Lines of Code**: 391  
**Status**: Production Ready

### Overview
Extracts query execution plans from PostgreSQL and MySQL, adding plan metadata as attributes to metrics.

### Architecture
```go
type planAttributeExtractor struct {
    config        Config
    logger        *zap.Logger
    parserCache   *lru.Cache[string, *ParsedPlan]
    parserPool    sync.Pool
    metrics       *extractorMetrics
}

type ParsedPlan struct {
    TotalCost    float64
    ExecutionTime float64
    PlanHash     string
    Operations   []string
    Timestamp    time.Time
}
```

### Key Features

#### 1. Multi-Database Support

**PostgreSQL Plans**:
```json
{
  "Plan": {
    "Node Type": "Seq Scan",
    "Total Cost": 1234.56,
    "Plan Rows": 1000,
    "Plan Width": 32
  }
}
```

**MySQL Plans**:
```json
{
  "query_block": {
    "select_id": 1,
    "cost_info": {
      "query_cost": "1234.56"
    }
  }
}
```

#### 2. Plan Hashing
```go
// Consistent hash for plan deduplication
func generatePlanHash(plan interface{}) string {
    // Remove variable parts (costs, rows)
    normalized := normalizePlan(plan)
    hash := sha256.Sum256([]byte(normalized))
    return hex.EncodeToString(hash[:])
}
```

#### 3. Parser Pooling
```go
// Reuse parser objects
parserPool: sync.Pool{
    New: func() interface{} {
        return &planParser{
            buffer: make([]byte, 0, 4096),
        }
    },
}
```

#### 4. Timeout Protection
```go
ctx, cancel := context.WithTimeout(ctx, p.config.ParseTimeout)
defer cancel()

parsedPlan, err := p.parseWithTimeout(ctx, planStr)
```

### Configuration Reference

```yaml
plan_attribute_extractor:
  # Enable/disable extraction
  enabled: true
  
  # Parsing timeout
  parse_timeout: 5s
  
  # Maximum plan size to parse
  max_plan_size: 100KB
  
  # Cache configuration
  cache:
    size: 1000
    ttl: 300s
  
  # Attributes to extract
  extract_attributes:
    - total_cost
    - execution_time
    - plan_hash
    - node_types
  
  # Database-specific settings
  postgresql:
    extract_explain_analyze: true
    extract_buffers: false
    extract_timing: true
    
  mysql:
    extract_cost_info: true
    extract_optimizer_trace: false
```

### Extracted Attributes

| Attribute | Type | Description |
|-----------|------|-------------|
| `plan.total_cost` | float64 | Total query cost |
| `plan.execution_time_ms` | float64 | Actual execution time |
| `plan.hash` | string | Unique plan identifier |
| `plan.node_types` | []string | Operation types used |
| `plan.is_cached` | bool | Whether plan was cached |
| `plan.database_type` | string | postgresql or mysql |

### Metrics Exposed

```prometheus
# Plans processed
plan_extractor_plans_processed_total{database_type="postgresql|mysql"}

# Parse errors
plan_extractor_parse_errors_total{database_type="...", error_type="..."}

# Cache hit rate
plan_extractor_cache_hit_rate

# Parse duration
plan_extractor_parse_duration_milliseconds{quantile="0.5|0.9|0.99"}

# Plan sizes
plan_extractor_plan_size_bytes{quantile="0.5|0.9|0.99"}
```

### Performance Characteristics
- **Memory**: O(cache_size) + parser pools
- **CPU**: JSON parsing overhead
- **Latency**: 1-5ms typical, timeout at 5s

---

## Verification Processor

**Location**: `processors/verification/`  
**Lines of Code**: 1,353  
**Status**: Production Ready

### Overview
Performs data quality validation and PII detection on metrics and attributes.

### Architecture
```go
type verificationProcessor struct {
    config          Config
    logger          *zap.Logger
    piiDetector     *piiDetector
    qualityChecker  *qualityChecker
    autoTuner       *autoTuner
    metrics         *verificationMetrics
}
```

### Key Features

#### 1. PII Detection

**Supported Patterns**:
```yaml
pii_patterns:
  # US Social Security Numbers
  ssn:
    pattern: '\b\d{3}-\d{2}-\d{4}\b'
    confidence: high
    
  # Credit Card Numbers (with Luhn check)
  credit_card:
    pattern: '\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b'
    validator: luhn_algorithm
    confidence: high
    
  # Email Addresses
  email:
    pattern: '\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b'
    confidence: medium
    
  # Phone Numbers (multiple formats)
  phone:
    patterns:
      - '\b\d{3}[-.]?\d{3}[-.]?\d{4}\b'  # US
      - '\b\+\d{1,3}\s?\d{1,14}\b'       # International
    confidence: medium
    
  # IP Addresses
  ip_address:
    pattern: '\b(?:\d{1,3}\.){3}\d{1,3}\b'
    validator: valid_ip_range
    confidence: low
    
  # Custom patterns
  custom:
    - name: employee_id
      pattern: '\bEMP\d{6}\b'
      confidence: high
```

#### 2. Data Quality Checks

```go
type QualityCheck interface {
    Name() string
    Check(value interface{}) (bool, string)
}

// Built-in checks
- NullValueCheck
- RangeCheck
- FormatCheck
- UniquenessCheck
- CompletenessCheck
```

#### 3. Auto-Tuning

```go
type AutoTuner struct {
    // Adjusts sensitivity based on false positive rates
    sensitivityAdjustment float64
    
    // Learns from operator feedback
    feedbackLoop *FeedbackCollector
    
    // Adapts patterns based on data
    patternAdapter *PatternAdapter
}
```

#### 4. Sanitization Actions

```yaml
sanitization_actions:
  # Masking
  mask:
    replacement: "****"
    preserve_length: true
    
  # Hashing
  hash:
    algorithm: sha256
    salt: "${SANITIZATION_SALT}"
    
  # Removal
  remove:
    delete_attribute: true
    
  # Tokenization
  tokenize:
    token_prefix: "TOK_"
    store_mapping: false
```

### Configuration Reference

```yaml
verification:
  # Enable/disable verification
  enabled: true
  
  # PII Detection
  pii_detection:
    enabled: true
    sensitivity: medium  # low, medium, high
    
    # Patterns to check
    patterns:
      - ssn
      - credit_card
      - email
      - phone
      
    # Custom patterns
    custom_patterns:
      - name: api_key
        pattern: '\b[A-Za-z0-9]{32}\b'
        confidence: high
        
    # Attributes to scan
    scan_attributes:
      - query_text
      - error_message
      - user_comment
      
    # Sanitization
    sanitization:
      default_action: mask
      actions:
        ssn: hash
        credit_card: remove
        email: mask
  
  # Quality Checks
  quality_checks:
    enabled: true
    checks:
      - name: null_check
        attributes: ["duration_ms", "row_count"]
        
      - name: range_check
        attribute: duration_ms
        min: 0
        max: 3600000  # 1 hour
        
      - name: format_check
        attribute: timestamp
        format: RFC3339
  
  # Auto-tuning
  auto_tuning:
    enabled: true
    learning_rate: 0.01
    feedback_endpoint: "/verification/feedback"
    
  # Performance settings
  performance:
    max_attribute_length: 10000
    scan_timeout: 100ms
    parallel_scanners: 4
```

### Metrics Exposed

```prometheus
# PII detections
verification_pii_detections_total{pattern="ssn|credit_card|email", action="mask|remove|hash"}

# Quality check failures
verification_quality_failures_total{check="null_check|range_check", attribute="..."}

# Processing performance
verification_processing_duration_ms{quantile="0.5|0.9|0.99"}

# Sanitization actions
verification_sanitization_total{action="mask|remove|hash|tokenize"}

# Auto-tuning adjustments
verification_autotuning_adjustments_total{direction="increase|decrease"}
```

### API Endpoints

```bash
# Submit feedback for auto-tuning
POST /verification/feedback
{
  "detection_id": "det_123",
  "was_correct": false,
  "pattern": "email",
  "value": "not-an-email"
}

# Get detection statistics
GET /verification/stats
{
  "total_scans": 1000000,
  "detections": {
    "ssn": 42,
    "credit_card": 13,
    "email": 256
  },
  "false_positive_rate": 0.02
}

# Update patterns dynamically
POST /verification/patterns
{
  "name": "custom_id",
  "pattern": "\\bID\\d{8}\\b",
  "confidence": "high"
}
```

### Performance Characteristics
- **Memory**: O(n) where n=number of patterns
- **CPU**: Regex matching overhead
- **Latency**: 0.5-2ms typical per record
- **Throughput**: 50K+ records/second

---

## Processor Interaction

### Processing Pipeline Order
```
1. Memory Limiter (standard)
   ↓
2. Adaptive Sampler (reduce volume)
   ↓
3. Circuit Breaker (protect sources)
   ↓
4. Plan Extractor (enrich data)
   ↓
5. Verification (ensure quality)
   ↓
6. Batch (optimize export)
```

### Inter-Processor Communication
Processors communicate through metric attributes:
```go
// Adaptive Sampler adds:
metric.Attributes().PutBool("sampled", true)
metric.Attributes().PutStr("sample_rule", "slow_queries")

// Circuit Breaker checks:
if attrs.Get("circuit_breaker.skip").Bool() {
    return nil  // Skip processing
}

// Plan Extractor adds:
metric.Attributes().PutStr("plan.hash", planHash)
metric.Attributes().PutDouble("plan.cost", totalCost)

// Verification reads all attributes for scanning
```

### Graceful Degradation
Each processor handles missing dependencies:
```go
// If plan extractor is disabled/fails
if !attrs.Exists("plan.hash") {
    // Continue without plan data
    p.logger.Debug("No plan data available")
}

// If circuit breaker is removed
if !p.circuitBreakerEnabled() {
    // Process all metrics without protection
}
```

---

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025