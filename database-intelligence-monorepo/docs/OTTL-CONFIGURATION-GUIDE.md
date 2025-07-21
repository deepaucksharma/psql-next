# OpenTelemetry Transformation Language (OTTL) Configuration Guide

This guide documents the correct usage of OTTL in OpenTelemetry Collector configurations and common pitfalls to avoid.

## Table of Contents
- [OTTL Context Types](#ottl-context-types)
- [Common Issues and Fixes](#common-issues-and-fixes)
- [Best Practices](#best-practices)
- [Validation Tools](#validation-tools)

## OTTL Context Types

OTTL provides different contexts for accessing data at various levels:

### 1. Metric Context
```yaml
transform/example:
  metric_statements:
    - context: metric
      statements:
        # Available: name, description, unit, type, aggregation_temporality
        - set(name, "new.metric.name") where name == "old.metric.name"
        - set(description, "Updated description")
        - set(unit, "ms")
```

### 2. Datapoint Context
```yaml
transform/example:
  metric_statements:
    - context: datapoint
      statements:
        # Available: value, attributes, time, flags
        # NOT available: metric.name, metric.unit, metric.description
        - set(value, value * 1000) where value > 0
        - set(attributes["new_attr"], "value")
        - limit(attributes["high_cardinality"], 100)
```

### 3. Scope Context
```yaml
transform/example:
  metric_statements:
    - context: scope
      statements:
        # Available: scope name, version, attributes
        - set(name, "instrumentation.scope")
        - set(version, "1.0.0")
```

## Common Issues and Fixes

### Issue 1: Using metric.name in datapoint context

❌ **Incorrect:**
```yaml
- context: datapoint
  statements:
    - set(attributes["type"], "slow") where metric.name == "query.duration" and value > 1000
```

✅ **Correct:**
```yaml
# Option 1: Use metric context
- context: metric
  statements:
    - set(name, "query.duration.slow") where name == "query.duration"

# Option 2: Split into two contexts
- context: metric
  statements:
    - set(attributes["metric_type"], "duration") where name == "query.duration"
- context: datapoint
  statements:
    - set(attributes["speed"], "slow") where value > 1000
```

### Issue 2: Using metric.value instead of value

❌ **Incorrect:**
```yaml
- context: datapoint
  statements:
    - set(metric.value, metric.value * 1000)
```

✅ **Correct:**
```yaml
- context: datapoint
  statements:
    - set(value, value * 1000)
```

### Issue 3: Invalid telemetry configuration

❌ **Incorrect:**
```yaml
telemetry:
  metrics:
    address: 0.0.0.0:8889  # This is deprecated
```

✅ **Correct:**
```yaml
telemetry:
  metrics:
    level: detailed
    address: 0.0.0.0:8888  # This goes in service.telemetry.metrics
```

### Issue 4: Filter in attributes processor

❌ **Incorrect:**
```yaml
attributes:
  actions:
    - key: environment
      value: production
      action: insert
      filter:  # This is not valid
        match_type: strict
```

✅ **Correct:**
```yaml
# Use a separate filter processor
filter:
  metrics:
    include:
      match_type: strict
      metric_names:
        - "mysql.*"

attributes:
  actions:
    - key: environment
      value: production
      action: insert
```

## Best Practices

### 1. Choose the Right Context

- **Use `metric` context for:**
  - Renaming metrics
  - Modifying metric metadata (description, unit)
  - Filtering based on metric names
  - Adding metric-level attributes

- **Use `datapoint` context for:**
  - Modifying values
  - Adding/modifying datapoint attributes
  - Time-based operations
  - Value-based filtering

### 2. Pipeline Organization

```yaml
processors:
  # 1. Always start with memory_limiter
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    
  # 2. Batch for efficiency
  batch:
    timeout: 10s
    
  # 3. Resource detection
  resourcedetection:
    detectors: [env, system, docker]
    
  # 4. Metric-level transformations
  transform/metrics:
    metric_statements:
      - context: metric
        statements:
          - set(name, "db." + name) where name =~ "mysql.*"
          
  # 5. Datapoint-level transformations
  transform/values:
    metric_statements:
      - context: datapoint
        statements:
          - set(value, value * 1000) where unit == "s"
          
  # 6. Add attributes last
  attributes:
    actions:
      - key: module
        value: my-module
        action: insert
```

### 3. Error Handling

Always use `error_mode: ignore` for production:

```yaml
transform/production:
  error_mode: ignore  # Continue processing on errors
  metric_statements:
    - context: metric
      statements:
        - set(name, "fallback") where name == nil
```

### 4. Performance Optimization

```yaml
# Limit high-cardinality attributes
transform/cardinality:
  metric_statements:
    - context: datapoint
      statements:
        - limit(attributes["user_id"], 1000)
        - limit(attributes["session_id"], 500)
        - delete_key(attributes, "debug_info")
```

## Validation Tools

### 1. Configuration Validation Script

Run the validation script to check for common issues:
```bash
./scripts/validate-configurations.sh
```

### 2. Automated Fix Script

Fix common issues automatically:
```bash
./scripts/fix-common-issues.sh
```

### 3. Manual Validation

Check specific patterns:
```bash
# Find metric.name in datapoint context
grep -B2 -A2 "context: datapoint" config/*.yaml | grep "metric\\.name"

# Find invalid telemetry config
grep -A5 "telemetry:" config/*.yaml | grep -A3 "metrics:" | grep "address:"

# Find filter in attributes processor
grep -B5 -A5 "attributes/" config/*.yaml | grep "filter:"
```

## Module-Specific Examples

### Core Metrics
```yaml
transform/metric_enrichment:
  error_mode: ignore
  metric_statements:
    - context: metric
      statements:
        - set(attributes["metric.category"], "mysql")
        - set(attributes["metric.subcategory"], "core")
        - set(attributes["nr.highValue"], true) 
          where name == "mysql.replica.time_behind_source"
```

### Wait Profiler
```yaml
transform/wait_analysis:
  error_mode: ignore
  metric_statements:
    - context: metric
      statements:
        - set(name, "mysql.wait." + name) 
          where name =~ "events_waits.*"
    - context: datapoint
      statements:
        - set(attributes["severity"], "critical") 
          where value > 1000
```

### Business Impact
```yaml
transform/business_scoring:
  error_mode: ignore
  metric_statements:
    - context: metric
      statements:
        - set(attributes["business.impact"], "high")
          where name =~ ".*checkout.*" or name =~ ".*payment.*"
    - context: datapoint
      statements:
        - set(attributes["sla.violation"], true)
          where value > attributes["sla.threshold"]
```

## Troubleshooting

### Debug OTTL Transformations

Use the debug exporter to see transformation results:
```yaml
exporters:
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 100

service:
  pipelines:
    metrics/debug:
      receivers: [mysql]
      processors: [transform/test]
      exporters: [debug]
```

### Common Error Messages

1. **"segment 'name' from path 'metric.name' is not a valid path"**
   - You're trying to access metric properties in datapoint context
   - Solution: Change to metric context or use appropriate datapoint fields

2. **"invalid action type 'filter'"**
   - Filter is not a valid action in attributes processor
   - Solution: Use a separate filter processor

3. **"'service.telemetry.metrics' has invalid keys: address"**
   - Old telemetry configuration format
   - Solution: Remove metrics.address from telemetry section

## References

- [OTTL Documentation](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl)
- [Transform Processor](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/transformprocessor)
- [OpenTelemetry Collector Configuration](https://opentelemetry.io/docs/collector/configuration/)