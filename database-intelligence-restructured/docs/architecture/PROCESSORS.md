# Database Intelligence Collector - Processor Documentation

## Overview

The Database Intelligence Collector implements six custom processors that enhance OpenTelemetry's capabilities for database monitoring. These processors are divided into Core processors (focused on database-specific functionality) and Enterprise processors (focused on operational excellence and cost management).

## Core Processors

### 1. Adaptive Sampler (`adaptivesampler`)

**Purpose**: Intelligent, rule-based sampling to reduce data volume while preserving important traces.

**Key Features**:
- Expression-based rule evaluation
- In-memory state management with LRU cache
- TTL-based deduplication
- Configurable default sampling rate

**Configuration**:
```yaml
processors:
  adaptivesampler:
    in_memory_only: true
    default_sample_rate: 0.1
    rules:
      - name: capture_errors
        conditions:
          - attribute: error
            operator: eq
            value: true
        sample_rate: 1.0
      - name: slow_queries
        conditions:
          - attribute: duration_ms
            operator: gt
            value: 1000
        sample_rate: 0.5
```

**Use Cases**:
- Reducing trace volume for high-traffic databases
- Ensuring critical errors are always captured
- Sampling slow queries for performance analysis

### 2. Circuit Breaker (`circuitbreaker`)

**Purpose**: Protects databases from monitoring overload using circuit breaker pattern.

**Key Features**:
- Three-state FSM (closed, open, half-open)
- Per-database tracking
- Adaptive timeout calculation
- Self-healing with exponential backoff

**Configuration**:
```yaml
processors:
  circuitbreaker:
    failure_threshold: 10
    timeout_duration: 30s
    half_open_requests: 5
    recovery_duration: 60s
```

**Use Cases**:
- Preventing monitoring from impacting database performance
- Automatic recovery when databases stabilize
- Protection against cascading failures

### 3. Plan Attribute Extractor (`planattributeextractor`)

**Purpose**: Extracts and analyzes query execution plans from database traces.

**Key Features**:
- PostgreSQL and MySQL plan parsing
- Query fingerprinting and deduplication
- Safe mode operation (no direct EXPLAIN calls)
- Plan cost analysis

**Configuration**:
```yaml
processors:
  planattributeextractor:
    safe_mode: true
    timeout_ms: 5000
    query_anonymization:
      enabled: true
      generate_fingerprint: true
    error_mode: ignore
```

**Use Cases**:
- Identifying expensive query patterns
- Tracking query plan changes over time
- Correlating performance with execution strategies

### 4. Verification Processor (`verification`)

**Purpose**: Ensures data quality and compliance through validation and PII detection.

**Key Features**:
- Comprehensive PII pattern detection
- Data quality validation
- Cardinality protection
- Auto-tuning capabilities

**Configuration**:
```yaml
processors:
  verification:
    pii_detection:
      enabled: true
      sensitivity: high
      patterns:
        - credit_card
        - ssn
        - email
        - phone
    quality_checks:
      enabled: true
      max_attribute_length: 1000
      max_attributes_per_span: 128
    auto_tuning:
      enabled: true
      learning_duration: 24h
```

**Use Cases**:
- Compliance with data privacy regulations
- Preventing high-cardinality explosions
- Maintaining data quality standards
- Automatic optimization based on workload

## Enterprise Processors

### 5. Cost Control Processor (`costcontrol`)

**Purpose**: Manages telemetry costs by enforcing budgets and reducing data intelligently.

**Key Features**:
- Monthly budget enforcement
- Intelligent data reduction strategies
- Cardinality management
- Support for both standard and Data Plus pricing

**Configuration**:
```yaml
processors:
  costcontrol:
    monthly_budget_usd: 5000
    price_per_gb: 0.4  # or higher for Data Plus
    metric_cardinality_limit: 10000
    aggressive_mode: false  # Auto-enables when over budget
    data_plus_enabled: false
```

**Use Cases**:
- Staying within telemetry budget
- Automatic data reduction when approaching limits
- Prioritizing high-value telemetry
- Cost-aware sampling decisions

### 6. NrIntegrationError Monitor (`nrerrormonitor`)

**Purpose**: Proactively detects patterns that lead to New Relic integration errors.

**Key Features**:
- Real-time validation of semantic conventions
- Cardinality threshold monitoring
- Attribute length validation
- Alert generation for critical issues

**Configuration**:
```yaml
processors:
  nrerrormonitor:
    max_attribute_length: 4095
    max_metric_name_length: 255
    cardinality_warning_threshold: 10000
    alert_threshold: 100
    reporting_interval: 60s
    error_suppression_duration: 5m
```

**Use Cases**:
- Preventing data rejection by New Relic
- Early warning for cardinality issues
- Ensuring semantic convention compliance
- Reducing troubleshooting time

## Processor Pipeline Best Practices

### Recommended Order

1. **Memory Limiter** (standard) - Prevent OOM
2. **Resource** (standard) - Add resource attributes
3. **NR Error Monitor** - Validate early
4. **Circuit Breaker** - Protect databases
5. **Plan Extractor** - Enrich with plans
6. **Verification** - Ensure compliance
7. **Adaptive Sampler** - Reduce volume
8. **Cost Control** - Enforce budgets
9. **Batch** (standard) - Optimize exports

### Example Pipeline Configuration

```yaml
service:
  pipelines:
    traces:
      receivers: [otlp, postgresql]
      processors: 
        - memory_limiter
        - resource
        - nrerrormonitor
        - circuitbreaker
        - planattributeextractor
        - verification
        - adaptivesampler
        - costcontrol
        - batch
      exporters: [otlp/newrelic]
```

## Performance Considerations

### Resource Usage

| Processor | CPU Impact | Memory Impact | Latency Added |
|-----------|------------|---------------|---------------|
| adaptivesampler | Low | Medium (LRU cache) | <1ms |
| circuitbreaker | Very Low | Low | <0.5ms |
| planattributeextractor | Medium | Low | 2-5ms |
| verification | Medium | Low | 1-3ms |
| costcontrol | Low | Medium (cardinality tracking) | <1ms |
| nrerrormonitor | Low | Low | <1ms |

### Scaling Recommendations

- **Stateless Processors**: All processors except adaptive sampler can scale horizontally
- **Adaptive Sampler**: Use in-memory mode for horizontal scaling
- **Gateway Pattern**: Deploy processors in a central gateway for better resource utilization

## Troubleshooting

### Common Issues

1. **Processor Not Found**
   - Ensure processor is registered in main.go
   - Check TypeStr matches configuration

2. **High Memory Usage**
   - Adjust LRU cache sizes
   - Enable memory_limiter processor
   - Review cardinality limits

3. **Data Not Appearing**
   - Check circuit breaker state
   - Verify sampling rules
   - Review cost control settings

### Debug Tools

- **Metrics Endpoint**: `http://localhost:8888/metrics`
- **zPages**: `http://localhost:55679/debug/tracez`
- **Health Check**: `http://localhost:13133/health`
- **Processor Metrics**: Each processor exports operational metrics

## Future Enhancements

### Planned Features

1. **Machine Learning Integration**
   - Anomaly detection in query patterns
   - Predictive sampling based on historical data

2. **Advanced Cost Optimization**
   - Multi-cloud cost awareness
   - Dynamic budget allocation

3. **Enhanced Security**
   - Encryption at rest for state files
   - Advanced PII detection patterns

4. **Kubernetes Native Features**
   - CRD-based configuration
   - Operator for lifecycle management

---

For more details on the overall architecture, see the [Architecture Overview](../ARCHITECTURE.md).
For enterprise deployment patterns, see the [Enterprise Architecture Guide](../ENTERPRISE_ARCHITECTURE.md).