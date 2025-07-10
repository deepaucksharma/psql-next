# Pipeline Ordering Guide

This guide explains the recommended ordering of processors in OpenTelemetry Collector pipelines for the Database Intelligence MVP.

## Overview

The order of processors in a pipeline matters because each processor may:
- Add attributes that subsequent processors depend on
- Filter data that affects downstream processing
- Transform data in ways that impact later stages

## Recommended Pipeline Order

### Logs Pipeline

```yaml
processors:
  - memory_limiter          # 1. Protect against OOM (always first)
  - planattributeextractor  # 2. Extract plan attributes for downstream use
  - adaptive_sampler        # 3. Sample based on plan attributes
  - circuit_breaker         # 4. Protect databases from overload
  - verification            # 5. Verify data quality and detect PII
  - resource               # 6. Add resource attributes
  - transform              # 7. Transform and enrich data
  - batch                  # 8. Batch for efficiency (always last)
```

### Metrics Pipeline

```yaml
processors:
  - memory_limiter          # 1. Protect against OOM (always first)
  - circuit_breaker         # 2. Protect databases from overload
  - adaptive_sampler        # 3. Sample high-cardinality metrics
  - filter                 # 4. Filter unwanted metrics
  - resource               # 5. Add resource attributes
  - transform              # 6. Transform and enrich data
  - batch                  # 7. Batch for efficiency (always last)
```

## Processor Dependencies

### Plan Attribute Extractor
- **Must come before**: adaptive_sampler (if using plan-based sampling)
- **Reason**: Extracts plan hash attributes that adaptive_sampler uses for deduplication

### Adaptive Sampler
- **Depends on**: planattributeextractor (for plan hash)
- **Must come before**: expensive processors
- **Reason**: Reduces data volume early in the pipeline

### Circuit Breaker
- **Should come early**: To protect databases from overload
- **Uses**: database_name attribute to track per-database circuits

### Verification
- **Should come late**: After filtering and sampling
- **Reason**: Performs expensive PII detection and quality checks

## Common Patterns

### 1. Basic Pipeline
```yaml
processors: [memory_limiter, batch]
```

### 2. Sampling Pipeline
```yaml
processors: [memory_limiter, adaptive_sampler, batch]
```

### 3. Full Intelligence Pipeline
```yaml
processors: [memory_limiter, planattributeextractor, adaptive_sampler, circuit_breaker, verification, batch]
```

## Configuration Examples

### Example 1: High-Volume Environment
```yaml
service:
  pipelines:
    logs:
      receivers: [filelog]
      processors:
        - memory_limiter      # Prevent OOM
        - planattributeextractor  # Extract plans
        - adaptive_sampler    # Aggressive sampling
        - circuit_breaker     # Protect DBs
        - batch              # Batch for throughput
      exporters: [otlp]
```

### Example 2: High-Security Environment
```yaml
service:
  pipelines:
    logs:
      receivers: [filelog]
      processors:
        - memory_limiter      # Prevent OOM
        - verification        # PII detection first
        - planattributeextractor  # Extract plans
        - resource           # Add security tags
        - transform          # Redact sensitive data
        - batch             # Batch
      exporters: [otlp]
```

## Validation

To validate your pipeline ordering:

1. Check for attribute dependencies
2. Ensure protective processors come first (memory_limiter, circuit_breaker)
3. Place expensive processors after filtering/sampling
4. Always end with batch processor for efficiency

## Troubleshooting

### Missing Attributes
**Symptom**: Adaptive sampler not deduplicating
**Cause**: planattributeextractor not before adaptive_sampler
**Fix**: Reorder processors

### High Memory Usage
**Symptom**: Collector OOM
**Cause**: memory_limiter not first or missing
**Fix**: Add memory_limiter as first processor

### Database Overload
**Symptom**: Database performance degradation
**Cause**: circuit_breaker missing or too late
**Fix**: Add circuit_breaker early in pipeline