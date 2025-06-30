# Monitoring Guide

This guide covers monitoring, alerting, and observability for the Database Intelligence Collector.

## Monitoring Overview

The Database Intelligence Collector provides comprehensive self-monitoring through:
- Health check endpoints
- Prometheus metrics
- Debug interfaces
- Component-level status reporting

## Health Monitoring

### Health Endpoints

| Endpoint | Purpose | Response |
|----------|---------|----------|
| `:13133/health` | Overall health status | JSON with component details |
| `:13133/health/live` | Kubernetes liveness probe | 200 OK or 503 |
| `:13133/health/ready` | Kubernetes readiness probe | 200 OK when ready |

### Health Check Response
```json
{
  "healthy": true,
  "timestamp": "2025-06-30T10:00:00Z",
  "version": "1.0.0",
  "uptime": 3600,
  "components": {
    "adaptive_sampler": {
      "healthy": true,
      "metrics": {
        "cache_size": 8500,
        "cache_capacity": 10000,
        "sample_rate": 0.05,
        "rules_active": 3
      }
    },
    "circuit_breaker": {
      "healthy": true,
      "metrics": {
        "open_circuits": 0,
        "total_circuits": 5,
        "trips_last_hour": 2
      }
    },
    "plan_extractor": {
      "healthy": true,
      "metrics": {
        "cache_hit_rate": 0.85,
        "avg_parse_time_ms": 2.3
      }
    },
    "verification": {
      "healthy": true,
      "metrics": {
        "pii_detections": 42,
        "scan_rate": 1000
      }
    }
  },
  "resource_usage": {
    "memory_usage_mb": 245.5,
    "memory_limit_mb": 512,
    "cpu_usage_percent": 15.3,
    "goroutines": 42
  }
}
```

## Metrics Monitoring

### Prometheus Endpoint
The collector exposes metrics at `:8888/metrics` in Prometheus format.

### Core Metrics

#### Collector Health Metrics
```prometheus
# Uptime in seconds
otelcol_process_uptime

# Memory usage
otelcol_process_runtime_heap_alloc_bytes
otelcol_process_runtime_total_alloc_bytes
process_resident_memory_bytes

# CPU usage
otelcol_process_cpu_seconds

# Go runtime
go_goroutines
go_memstats_gc_cpu_fraction
```

#### Pipeline Metrics
```prometheus
# Receiver metrics
otelcol_receiver_accepted_metric_points{receiver="postgresql"}
otelcol_receiver_refused_metric_points{receiver="postgresql"}

# Processor metrics
otelcol_processor_accepted_metric_points{processor="adaptive_sampler"}
otelcol_processor_refused_metric_points{processor="adaptive_sampler"}
otelcol_processor_dropped_metric_points{processor="adaptive_sampler"}

# Exporter metrics
otelcol_exporter_sent_metric_points{exporter="otlp/newrelic"}
otelcol_exporter_send_failed_metric_points{exporter="otlp/newrelic"}
otelcol_exporter_enqueue_failed_metric_points{exporter="otlp/newrelic"}
```

### Custom Processor Metrics

#### Adaptive Sampler
```prometheus
# Records processed
adaptive_sampler_records_processed_total

# Records dropped by reason
adaptive_sampler_records_dropped_total{reason="sampling"}
adaptive_sampler_records_dropped_total{reason="deduplication"}
adaptive_sampler_records_dropped_total{reason="rate_limit"}

# Cache performance
adaptive_sampler_cache_hit_rate
adaptive_sampler_cache_size
adaptive_sampler_cache_evictions_total

# Rule matches
adaptive_sampler_rule_matches_total{rule="slow_queries"}
adaptive_sampler_current_sample_rate{rule="default"}
```

#### Circuit Breaker
```prometheus
# Circuit state (0=closed, 1=half_open, 2=open)
circuit_breaker_state{database="production_db"}

# Trip counts
circuit_breaker_trips_total{database="production_db", reason="failure"}
circuit_breaker_trips_total{database="production_db", reason="resource"}

# Recovery metrics
circuit_breaker_recovery_attempts_total{database="production_db"}
circuit_breaker_recovery_time_seconds{database="production_db"}

# Failure tracking
circuit_breaker_failures_total{database="production_db", type="timeout"}
circuit_breaker_failures_total{database="production_db", type="error"}
```

#### Plan Attribute Extractor
```prometheus
# Plans processed
plan_extractor_plans_processed_total{database_type="postgresql"}
plan_extractor_plans_processed_total{database_type="mysql"}

# Parse performance
plan_extractor_parse_duration_milliseconds_bucket{le="1"}
plan_extractor_parse_duration_milliseconds_bucket{le="5"}
plan_extractor_parse_duration_milliseconds_bucket{le="10"}

# Cache metrics
plan_extractor_cache_hit_rate
plan_extractor_cache_size
plan_extractor_cache_memory_bytes

# Errors
plan_extractor_parse_errors_total{error_type="timeout"}
plan_extractor_parse_errors_total{error_type="invalid_json"}
```

#### Verification Processor
```prometheus
# PII detections
verification_pii_detections_total{pattern="ssn", action="mask"}
verification_pii_detections_total{pattern="credit_card", action="remove"}
verification_pii_detections_total{pattern="email", action="mask"}

# Quality checks
verification_quality_failures_total{check="null_check", attribute="duration_ms"}
verification_quality_failures_total{check="range_check", attribute="row_count"}

# Performance
verification_processing_duration_ms_bucket{le="1"}
verification_processing_duration_ms_bucket{le="5"}
verification_scan_rate

# Auto-tuning
verification_autotuning_adjustments_total{direction="increase"}
verification_autotuning_adjustments_total{direction="decrease"}
verification_false_positive_rate
```

## Dashboard Setup

### Grafana Dashboard

#### Collector Overview
```json
{
  "dashboard": {
    "title": "Database Intelligence Collector",
    "panels": [
      {
        "title": "Collector Health",
        "targets": [
          {
            "expr": "up{job=\"db-intelligence-collector\"}"
          }
        ]
      },
      {
        "title": "Memory Usage",
        "targets": [
          {
            "expr": "process_resident_memory_bytes{job=\"db-intelligence-collector\"} / 1024 / 1024"
          }
        ]
      },
      {
        "title": "Processing Rate",
        "targets": [
          {
            "expr": "rate(otelcol_processor_accepted_metric_points[5m])"
          }
        ]
      },
      {
        "title": "Error Rate",
        "targets": [
          {
            "expr": "rate(otelcol_exporter_send_failed_metric_points[5m]) / rate(otelcol_exporter_sent_metric_points[5m])"
          }
        ]
      }
    ]
  }
}
```

#### Key Queries

**Processing Pipeline Health**
```promql
# Pipeline throughput
sum(rate(otelcol_receiver_accepted_metric_points[5m])) by (receiver)

# Processing latency (estimated)
histogram_quantile(0.99, 
  sum(rate(otelcol_processor_batch_timeout_trigger_send[5m])) by (le)
)

# Dropped metrics
sum(rate(otelcol_processor_dropped_metric_points[5m])) by (processor)
```

**Circuit Breaker Monitoring**
```promql
# Open circuits
count(circuit_breaker_state == 2)

# Circuit breaker effectiveness
rate(circuit_breaker_trips_total[1h]) / rate(otelcol_receiver_accepted_metric_points[1h])

# Recovery time
histogram_quantile(0.95, circuit_breaker_recovery_time_seconds)
```

**Sampling Effectiveness**
```promql
# Actual vs target sampling rate
adaptive_sampler_current_sample_rate / adaptive_sampler_target_sample_rate

# Data reduction ratio
1 - (rate(otelcol_processor_accepted_metric_points{processor="adaptive_sampler"}[5m]) / 
     rate(otelcol_receiver_accepted_metric_points[5m]))

# Cache efficiency
adaptive_sampler_cache_hit_rate
```

## Alerting Rules

### Critical Alerts

```yaml
groups:
  - name: db_intelligence_critical
    interval: 30s
    rules:
      - alert: CollectorDown
        expr: up{job="db-intelligence-collector"} == 0
        for: 2m
        labels:
          severity: critical
          team: database-platform
        annotations:
          summary: "Database Intelligence Collector is down"
          description: "Collector {{ $labels.instance }} has been down for more than 2 minutes"
          
      - alert: HighMemoryUsage
        expr: |
          process_resident_memory_bytes{job="db-intelligence-collector"} / 
          otelcol_process_memory_limit > 0.9
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Collector memory usage above 90%"
          description: "Memory usage is {{ $value | humanizePercentage }} of limit"
          
      - alert: ExportFailureRate
        expr: |
          rate(otelcol_exporter_send_failed_metric_points[5m]) / 
          rate(otelcol_exporter_sent_metric_points[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High export failure rate"
          description: "{{ $value | humanizePercentage }} of exports are failing"
```

### Warning Alerts

```yaml
      - alert: CircuitBreakerOpen
        expr: circuit_breaker_state == 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Circuit breaker open for {{ $labels.database }}"
          description: "Database {{ $labels.database }} circuit has been open for 5 minutes"
          
      - alert: HighProcessingLatency
        expr: |
          histogram_quantile(0.99, 
            rate(otelcol_processor_batch_timeout_trigger_send[5m])
          ) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High processing latency"
          description: "P99 latency is {{ $value }}s"
          
      - alert: LowSamplingRate
        expr: adaptive_sampler_current_sample_rate < 0.001
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Very low sampling rate"
          description: "Sampling rate is {{ $value }}, may miss important data"
```

## Debug Interfaces

### zPages
Access debug information at `:55679/debug/`

- `/debug/tracez` - Trace information
- `/debug/pipelinez` - Pipeline statistics
- `/debug/extensionz` - Extension information
- `/debug/featurez` - Feature gates

### pprof Endpoints
For performance profiling (if enabled):

```bash
# CPU profile
go tool pprof http://localhost:1777/debug/pprof/profile?seconds=30

# Heap profile
go tool pprof http://localhost:1777/debug/pprof/heap

# Goroutine profile
curl http://localhost:1777/debug/pprof/goroutine?debug=1
```

## Log Monitoring

### Log Levels
```yaml
service:
  telemetry:
    logs:
      level: info  # debug, info, warn, error
      development: false
      encoding: json
      output_paths: ["stdout", "/var/log/collector/collector.log"]
```

### Key Log Patterns

**Startup Logs**
```
{"level":"info","msg":"Starting Database Intelligence Collector","version":"1.0.0"}
{"level":"info","msg":"Loaded processor","name":"adaptive_sampler"}
{"level":"info","msg":"Everything is ready. Begin running and processing data."}
```

**Error Patterns**
```
{"level":"error","msg":"Failed to scrape metrics","error":"connection refused"}
{"level":"error","msg":"Export failed","error":"context deadline exceeded"}
{"level":"error","msg":"Circuit opened","database":"production_db","reason":"threshold"}
```

### Log Aggregation Queries

**Elasticsearch/OpenSearch**
```json
{
  "query": {
    "bool": {
      "must": [
        {"term": {"kubernetes.labels.app": "db-intelligence"}},
        {"term": {"level": "error"}},
        {"range": {"@timestamp": {"gte": "now-1h"}}}
      ]
    }
  },
  "aggs": {
    "error_types": {
      "terms": {"field": "error.type"}
    }
  }
}
```

## SLI/SLO Monitoring

### Service Level Indicators

| SLI | Definition | Target |
|-----|------------|--------|
| Availability | Collector health check success | 99.9% |
| Latency | P99 processing time < 100ms | 99% |
| Error Rate | Export failure rate < 1% | 99% |
| Data Freshness | Metrics age < 2 minutes | 95% |

### SLO Queries

```promql
# Availability SLI
avg_over_time(up{job="db-intelligence-collector"}[30d])

# Latency SLI
histogram_quantile(0.99,
  rate(otelcol_processor_batch_timeout_trigger_send[30d])
) < 0.1

# Error Rate SLI
1 - (
  sum(rate(otelcol_exporter_send_failed_metric_points[30d])) /
  sum(rate(otelcol_exporter_sent_metric_points[30d]))
)

# Data Freshness SLI
time() - otelcol_receiver_scraped_metric_points_timestamp < 120
```

## Troubleshooting with Metrics

### High Memory Usage
```promql
# Check cache sizes
adaptive_sampler_cache_size
plan_extractor_cache_size

# Check batch queue
otelcol_processor_batch_metadata_cardinality

# Check goroutines
go_goroutines
```

### Processing Bottlenecks
```promql
# Compare input/output rates by processor
rate(otelcol_receiver_accepted_metric_points[5m]) - 
rate(otelcol_processor_accepted_metric_points{processor="adaptive_sampler"}[5m])

# Check processor queue lengths
otelcol_processor_queue_length
```

### Export Issues
```promql
# Check retry queue
otelcol_exporter_queue_size

# Check connection pool
otelcol_exporter_otlp_connection_pool_size
```

## Integration with Monitoring Systems

### Prometheus Configuration
```yaml
global:
  scrape_interval: 30s

scrape_configs:
  - job_name: 'db-intelligence-collector'
    static_configs:
      - targets: ['collector:8888']
    metric_relabel_configs:
      - source_labels: [__name__]
        regex: 'go_.*'
        action: drop  # Drop verbose Go metrics
```

### New Relic Integration
```yaml
# Collector sends its own metrics
exporters:
  otlp/newrelic:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    
service:
  telemetry:
    metrics:
      readers:
        - periodic:
            interval: 60s
            exporter:
              otlp:
                endpoint: otlp.nr-data.net:4317
```

---

**Document Version**: 1.0.0  
**Last Updated**: June 30, 2025