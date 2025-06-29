# Verification Processor

A comprehensive OpenTelemetry Collector processor that provides real-time feedback on data quality, integration health, and automatic optimization capabilities.

## Features

### Core Verification
- **Periodic Health Checks**: Runs periodic checks on the collector's health and integration status.
- **Data Freshness**: Monitors data freshness and alerts if data becomes stale.
- **Entity Correlation**: Verifies entity correlation rate and alerts if it's below threshold.
- **Query Normalization**: Verifies query normalization effectiveness for cardinality management.
- **Feedback as Logs**: Exports feedback events as structured telemetry.

### Advanced Quality Validation
- **Schema Validation**: Validates log records against expected data types and structures.
- **Required Fields**: Enforces presence of critical fields in log records.
- **Cardinality Monitoring**: Tracks and alerts on high-cardinality fields to prevent metric explosion.
- **Data Type Validation**: Ensures data types match expectations for downstream processing.

### PII Detection & Sanitization
- **Pattern-Based Detection**: Uses regex patterns to identify potential PII in logs.
- **Auto-Sanitization**: Optionally sanitizes detected PII automatically.
- **Configurable Sensitivity**: Adjustable sensitivity levels (low, medium, high).
- **Custom Patterns**: Support for custom PII detection patterns.
- **Field Exclusions**: Ability to exclude specific fields from PII scanning.

### Continuous Health Monitoring
- **System Resource Monitoring**: Tracks memory, CPU, and disk usage.
- **Database Connectivity**: Tests connectivity to monitored databases.
- **Performance Metrics**: Monitors throughput, latency, and error rates.
- **Alert Thresholds**: Configurable thresholds for system resources.

### Auto-Tuning Engine
- **Performance Analysis**: Analyzes processing performance trends over time.
- **Automatic Recommendations**: Generates tuning recommendations based on performance data.
- **Confidence Scoring**: Provides confidence scores for recommendations.
- **Auto-Application**: Optionally applies high-confidence recommendations automatically.
- **Parameter Optimization**: Optimizes processor parameters for better performance.

### Self-Healing Capabilities
- **Automatic Retry**: Retries failed operations with exponential backoff.
- **Memory Management**: Automatically triggers garbage collection and cache cleanup.
- **Connection Recovery**: Attempts to recover from database connectivity issues.
- **Issue Classification**: Categorizes issues for appropriate healing strategies.
- **Healing History**: Tracks self-healing actions and success rates.

## Configuration

### Basic Configuration

```yaml
processors:
  verification:
    # Core verification settings
    enable_periodic_verification: true
    verification_interval: 5m
    data_freshness_threshold: 10m
    min_entity_correlation_rate: 0.8
    min_normalization_rate: 0.9
    require_entity_synthesis: true
    export_feedback_as_logs: true
```

### Advanced Configuration

```yaml
processors:
  verification:
    # Core verification
    enable_periodic_verification: true
    verification_interval: 5m
    data_freshness_threshold: 10m
    min_entity_correlation_rate: 0.8
    min_normalization_rate: 0.9
    require_entity_synthesis: true
    export_feedback_as_logs: true
    
    # Continuous health monitoring
    enable_continuous_health_checks: true
    health_check_interval: 30s
    health_thresholds:
      memory_percent: 85.0
      cpu_percent: 80.0
      disk_percent: 90.0
      network_latency: 5s
    
    # Quality validation rules
    quality_rules:
      required_fields:
        - database_name
        - query_id
        - duration_ms
      enable_schema_validation: true
      cardinality_limits:
        query_id: 10000
        database_name: 100
      data_type_validation:
        duration_ms: "double"
        error_count: "int"
    
    # PII detection and sanitization
    pii_detection:
      enabled: true
      auto_sanitize: false
      sensitivity_level: "medium"
      custom_patterns:
        - '\b\d{16}\b'  # Credit card numbers
      exclude_fields:
        - query_hash
        - plan_hash
    
    # Auto-tuning engine
    enable_auto_tuning: true
    auto_tuning_interval: 10m
    auto_tuning_config:
      enable_auto_apply: false
      min_confidence_level: 0.8
      max_parameter_change: 0.2
    
    # Self-healing capabilities
    enable_self_healing: true
    self_healing_interval: 1m
    self_healing_config:
      max_retries: 3
      backoff_multiplier: 2.0
      enabled_issue_types:
        - consumer_error
        - high_memory
        - database_connectivity
```

### Configuration Options

#### Core Settings
- `enable_periodic_verification`: Enable/disable periodic health checks
- `verification_interval`: How often to run verification checks
- `data_freshness_threshold`: Maximum time without data before alerting
- `min_entity_correlation_rate`: Minimum acceptable entity correlation rate (0.0-1.0)
- `min_normalization_rate`: Minimum acceptable query normalization rate (0.0-1.0)
- `require_entity_synthesis`: Enforce entity synthesis attributes
- `export_feedback_as_logs`: Export feedback events as telemetry

#### Health Monitoring
- `enable_continuous_health_checks`: Enable continuous system monitoring
- `health_check_interval`: Frequency of health checks
- `health_thresholds`: Alert thresholds for system resources

#### Quality Rules
- `required_fields`: List of fields that must be present
- `enable_schema_validation`: Enable data type validation
- `cardinality_limits`: Maximum unique values per field
- `data_type_validation`: Expected data types for fields

#### PII Detection
- `enabled`: Enable PII detection
- `auto_sanitize`: Automatically sanitize detected PII
- `sensitivity_level`: Detection sensitivity (low/medium/high)
- `custom_patterns`: Additional regex patterns for PII detection
- `exclude_fields`: Fields to exclude from PII scanning

#### Auto-Tuning
- `enable_auto_tuning`: Enable automatic performance tuning
- `auto_tuning_interval`: Frequency of tuning analysis
- `enable_auto_apply`: Automatically apply high-confidence recommendations
- `min_confidence_level`: Minimum confidence for auto-application
- `max_parameter_change`: Maximum allowed parameter change percentage

#### Self-Healing
- `enable_self_healing`: Enable automatic issue remediation
- `self_healing_interval`: Frequency of healing checks
- `max_retries`: Maximum retry attempts for failed operations
- `backoff_multiplier`: Exponential backoff multiplier
- `enabled_issue_types`: Types of issues to automatically heal

## Usage in Collector

### Basic Pipeline Usage

```yaml
service:
  pipelines:
    logs:
      receivers: [postgresqlquery]
      processors:
        - memory_limiter
        - circuitbreaker     # For safety
        - verification       # Enhanced verification
        - resource          # Entity synthesis
        - batch             # Batching
      exporters: [otlp]
```

### Production Pipeline with Full Features

```yaml
service:
  pipelines:
    logs:
      receivers: [postgresqlquery]
      processors:
        - memory_limiter
        - circuitbreaker
        - verification      # All features enabled
        - transform         # Query normalization
        - resource          # Entity synthesis  
        - batch
      exporters: [otlp, debug]
    
    # Separate pipeline for verification feedback
    logs/verification:
      receivers: [verification]  # Feedback events
      processors: [batch]
      exporters: [otlp]
```

## Feedback Events

The processor generates structured feedback events with the following categories:

- **quality_validation**: Schema and data quality issues
- **pii_detection**: PII detected in logs
- **system_health**: Resource utilization alerts
- **auto_tuning**: Performance tuning recommendations
- **self_healing**: Automatic remediation actions
- **entity_synthesis**: Entity correlation issues
- **cardinality_management**: High cardinality warnings

### Example Feedback Event

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "WARNING",
  "category": "quality_validation",
  "message": "Missing required field: duration_ms",
  "database": "postgres_primary",
  "remediation": "Ensure all required fields are present in log records",
  "severity": 5,
  "auto_fixed": false,
  "metrics": {
    "missing_field": "duration_ms",
    "record_count": 1
  }
}
```

## Monitoring & Alerting

### Key Metrics to Monitor

1. **Quality Score**: Overall data quality percentage
2. **PII Violations**: Number of PII patterns detected
3. **Entity Correlation Rate**: Percentage of records with entity attributes
4. **Self-Healing Success Rate**: Percentage of successful healing actions
5. **System Resource Usage**: Memory, CPU, disk utilization

### Recommended Alerts

```yaml
# Example Prometheus alerting rules
groups:
  - name: verification_processor
    rules:
      - alert: HighPIIViolations
        expr: rate(verification_pii_violations_total[5m]) > 0.1
        annotations:
          summary: "High rate of PII violations detected"
          
      - alert: LowEntityCorrelation
        expr: verification_entity_correlation_rate < 0.8
        annotations:
          summary: "Entity correlation rate below threshold"
          
      - alert: QualityScoreDropped
        expr: verification_quality_score < 0.9
        annotations:
          summary: "Data quality score has dropped"
```

## Building

### Using OCB (OpenTelemetry Collector Builder)

1. Add to your `ocb-config.yaml`:
```yaml
processors:
  - gomod: github.com/database-intelligence/database-intelligence-mvp/processors/verification v0.1.0
    path: ./processors/verification
```

2. Build the collector:
```bash
ocb --config ocb-config.yaml
```

### Manual Integration

1. Add to your `go.mod`:
```go
require github.com/database-intelligence/database-intelligence-mvp/processors/verification v0.1.0
```

2. Import in your main.go:
```go
import "github.com/database-intelligence/database-intelligence-mvp/processors/verification"
```

3. Register the factory:
```go
factories.Processors["verification"] = verification.NewFactory()
```

## Testing

Run the comprehensive test suite:

```bash
cd processors/verification
go test -v ./...
```

### Test Coverage

The test suite covers:
- Basic verification functionality
- Quality validation rules
- PII detection and sanitization
- Health monitoring
- Auto-tuning engine
- Self-healing capabilities
- Configuration validation

## Performance Considerations

### Resource Usage

- **Memory**: Approximately 10-50MB depending on cache sizes
- **CPU**: Low overhead, typically <5% additional CPU usage
- **Latency**: Adds minimal latency (~1-5ms per batch)

### Tuning Recommendations

1. **For High Throughput**: 
   - Disable auto-sanitization of PII
   - Increase health check intervals
   - Reduce quality validation scope

2. **For High Security**:
   - Enable PII auto-sanitization
   - Use high sensitivity PII detection
   - Enable all quality validation rules

3. **For Production Stability**:
   - Enable self-healing
   - Set conservative auto-tuning thresholds
   - Monitor feedback events closely

## Troubleshooting

### Common Issues

1. **High Memory Usage**
   - Reduce cardinality limits
   - Increase cleanup intervals
   - Enable auto-healing memory management

2. **PII False Positives**
   - Adjust sensitivity level
   - Add exclude fields
   - Customize PII patterns

3. **Performance Impact**
   - Disable unnecessary features
   - Increase batch sizes
   - Tune health check intervals

### Debug Mode

Enable detailed logging:

```yaml
processors:
  verification:
    export_feedback_as_logs: true
    # Add debug-level logging in collector config
```

## Security Considerations

- PII detection patterns are configurable and should be reviewed
- Auto-sanitization permanently modifies log data
- Health monitoring may expose system information
- Self-healing actions should be monitored for security implications
