# New Relic Integration Guide for Database Intelligence

## Overview

This guide provides comprehensive instructions for integrating the Database Intelligence solution with New Relic using OpenTelemetry Protocol (OTLP). It covers configuration, validation, troubleshooting, and best practices for both config-only and enhanced deployment modes.

## Prerequisites

- New Relic account with appropriate license
- OTLP endpoint access enabled
- Database read-only credentials
- OpenTelemetry Collector (vanilla or New Relic distribution)

## New Relic OTLP Endpoints

### Regional Endpoints

| Region | OTLP Endpoint | Port |
|--------|--------------|------|
| US | `https://otlp.nr-data.net` | 4318 (HTTP) |
| EU | `https://otlp.eu01.nr-data.net` | 4318 (HTTP) |
| FedRAMP | Contact New Relic Support | 4318 (HTTP) |

### Authentication

New Relic requires API key authentication via headers:

```yaml
exporters:
  otlp:
    endpoint: "${NEW_RELIC_OTLP_ENDPOINT}"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"
```

## Metric Data Model Alignment

### Naming Conventions

Our metrics follow OpenTelemetry semantic conventions, which New Relic automatically maps to its data model:

| OTel Metric Name | New Relic Query | Type | Description |
|------------------|-----------------|------|-------------|
| `postgresql.connections.active` | `FROM Metric SELECT latest(postgresql.connections.active)` | Gauge | Active connections |
| `postgresql.transactions.committed` | `FROM Metric SELECT rate(sum(postgresql.transactions.committed), 1 minute)` | Sum | Transaction rate |
| `postgresql.blocks.hit` | `FROM Metric SELECT average(postgresql.blocks.hit)` | Sum | Buffer hits |
| `postgresql.query.duration` | `FROM Metric SELECT histogram(postgresql.query.duration)` | Histogram | Query latency |
| `system.cpu.utilization` | `FROM Metric SELECT average(system.cpu.utilization)` | Gauge | CPU usage |

### Resource Attributes

Essential attributes for New Relic entity synthesis:

```yaml
resource:
  attributes:
    # Required for entity creation
    - key: service.name
      value: "postgresql-prod-db01"
      action: upsert
    
    # Database identification
    - key: db.system
      value: "postgresql"  # or "mysql"
      action: insert
    
    - key: db.name
      value: "${DATABASE_NAME}"
      action: insert
    
    # Host identification
    - key: host.name
      value: "${HOSTNAME}"
      action: insert
    
    - key: host.id
      value: "${HOST_ID}"
      action: insert
    
    # Environment context
    - key: deployment.environment
      value: "${ENVIRONMENT}"  # dev, staging, prod
      action: insert
    
    # Telemetry metadata
    - key: telemetry.sdk.name
      value: "opentelemetry"
      action: insert
    
    - key: telemetry.sdk.language
      value: "go"
      action: insert
    
    - key: telemetry.sdk.version
      value: "${OTEL_VERSION}"
      action: insert
```

## Configuration Examples

### Basic Configuration (Config-Only Mode)

```yaml
# config/newrelic-basic.yaml
receivers:
  postgresql:
    endpoint: "${DB_ENDPOINT}"
    username: "${DB_RO_USERNAME}"
    password: "${DB_RO_PASSWORD}"
    collection_interval: 30s
    databases:
      - "*"

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    
  resource:
    attributes:
      - key: service.name
        value: "${SERVICE_NAME}"
        action: upsert
      - key: db.system
        value: "postgresql"
        action: insert
        
  cumulativetodelta:
    include:
      match_type: regexp
      metric_names:
        - "postgresql.transactions.*"
        - "postgresql.blocks.*"
        
  batch:
    timeout: 10s
    send_batch_size: 1000

exporters:
  otlp:
    endpoint: "https://otlp.nr-data.net:4318"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"
    compression: gzip
    retry_on_failure:
      enabled: true
      max_elapsed_time: 300s

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, resource, cumulativetodelta, batch]
      exporters: [otlp]
```

### Enhanced Configuration (With Custom Components)

```yaml
# config/newrelic-enhanced.yaml
receivers:
  postgresql:
    endpoint: "${DB_ENDPOINT}"
    collection_interval: 10s
    
  enhancedsql:
    endpoint: "${DB_ENDPOINT}"
    features:
      query_stats:
        enabled: true
        top_n_queries: 100
      execution_plans:
        enabled: true
        
  ash:
    endpoint: "${DB_ENDPOINT}"
    sampling_interval: 1s

processors:
  memory_limiter:
    limit_mib: 1024
    
  adaptive_sampler:
    rules:
      - metric_pattern: "postgresql.query.*"
        base_rate: 0.1
        spike_detection:
          enabled: true
          
  verification:
    cardinality_limits:
      max_series_per_metric: 10000
      
  nrerrormonitor:
    monitor:
      - integration_errors
      - rate_limits
    emit_metrics: true
    
  costcontrol:
    budget:
      max_dpm: 5000000  # 5M data points/minute
      
  batch:
    send_batch_size: 5000
    timeout: 5s

exporters:
  otlp:
    endpoint: "https://otlp.nr-data.net:4318"
    headers:
      api-key: "${NEW_RELIC_LICENSE_KEY}"
    compression: gzip
    sending_queue:
      enabled: true
      queue_size: 10000

service:
  pipelines:
    metrics:
      receivers: [postgresql, enhancedsql, ash]
      processors: 
        - memory_limiter
        - adaptive_sampler
        - verification
        - costcontrol
        - nrerrormonitor
        - batch
      exporters: [otlp]
```

## Validation and Testing

### 1. Verify Metric Ingestion

```sql
-- Check if metrics are arriving
SELECT count(*) 
FROM Metric 
WHERE metric.name LIKE 'postgresql.%' 
SINCE 5 minutes ago

-- View specific metric values
SELECT average(postgresql.connections.active) 
FROM Metric 
WHERE service.name = 'postgresql-prod-db01'
TIMESERIES SINCE 30 minutes ago
```

### 2. Check for Integration Errors

```sql
-- Look for any ingestion errors
SELECT count(*), latest(message) 
FROM NrIntegrationError 
WHERE category = 'MetricAPI'
FACET error.type 
SINCE 1 hour ago

-- Check for cardinality violations
SELECT count(*) 
FROM NrIntegrationError 
WHERE message LIKE '%cardinality%' 
SINCE 1 day ago
```

### 3. Verify Entity Creation

```sql
-- Check if database entities are created
FROM Entity 
SELECT * 
WHERE type = 'DATABASE' 
AND name LIKE '%postgresql%'

-- Verify entity relationships
FROM Relationship 
SELECT * 
WHERE source.type = 'DATABASE' 
OR target.type = 'DATABASE'
```

### 4. Monitor Data Volume

```sql
-- Track ingestion rate
SELECT rate(count(*), 1 minute) as 'DPM' 
FROM Metric 
WHERE metric.name LIKE 'postgresql.%' 
TIMESERIES SINCE 1 hour ago

-- Check metric cardinality
SELECT uniqueCount(dimensions()) 
FROM Metric 
WHERE metric.name LIKE 'postgresql.%' 
FACET metric.name 
SINCE 1 hour ago
```

## Dashboards and Alerts

### Sample Dashboard JSON

```json
{
  "name": "Database Intelligence - PostgreSQL",
  "pages": [
    {
      "name": "Overview",
      "widgets": [
        {
          "title": "Active Connections",
          "visualization": "line",
          "query": "SELECT latest(postgresql.connections.active) FROM Metric TIMESERIES"
        },
        {
          "title": "Transaction Rate",
          "visualization": "line",
          "query": "SELECT rate(sum(postgresql.transactions.committed), 1 minute) FROM Metric TIMESERIES"
        },
        {
          "title": "Query Duration Distribution",
          "visualization": "histogram",
          "query": "SELECT histogram(postgresql.query.duration, 20, 10) FROM Metric"
        },
        {
          "title": "Cache Hit Ratio",
          "visualization": "billboard",
          "query": "SELECT (sum(postgresql.blocks.hit) / (sum(postgresql.blocks.hit) + sum(postgresql.blocks.read))) * 100 as 'Hit Ratio %' FROM Metric"
        }
      ]
    }
  ]
}
```

### Alert Conditions

```yaml
# High connection usage alert
- name: "Database Connection Limit"
  query: |
    SELECT latest(postgresql.connections.active) / latest(postgresql.connections.max) * 100 
    FROM Metric 
    WHERE service.name = 'postgresql-prod-db01'
  threshold:
    critical: 90
    warning: 80
    duration: 5 minutes

# Query performance degradation
- name: "Slow Query Detection"
  query: |
    SELECT percentile(postgresql.query.duration, 95) 
    FROM Metric 
    WHERE service.name = 'postgresql-prod-db01'
  threshold:
    critical: 5000  # 5 seconds
    warning: 2000   # 2 seconds
    duration: 10 minutes

# Integration health
- name: "OTLP Integration Errors"
  query: |
    SELECT count(*) 
    FROM NrIntegrationError 
    WHERE category = 'MetricAPI'
  threshold:
    critical: 10
    duration: 5 minutes
```

## Troubleshooting

### Common Issues

#### 1. No Data in New Relic

**Symptoms**: Queries return no results

**Checks**:
```bash
# Verify collector is running
curl http://localhost:13133/health

# Check collector metrics
curl http://localhost:8888/metrics | grep otlp_exporter

# Review collector logs
grep -i error /var/log/otelcol/collector.log
```

**Solutions**:
- Verify API key is correct
- Check network connectivity to OTLP endpoint
- Ensure correct endpoint URL (with protocol and port)
- Verify TLS/SSL certificates if applicable

#### 2. High Cardinality Warnings

**Symptoms**: NrIntegrationError events about cardinality

**Query to identify**:
```sql
SELECT count(*), max(cardinality) 
FROM NrIntegrationError 
WHERE message LIKE '%cardinality%' 
FACET metric.name
```

**Solutions**:
- Enable cardinality limiting in verification processor
- Add attribute filtering to remove high-cardinality dimensions
- Use adaptive sampling for high-volume metrics

#### 3. Missing Entity Relationships

**Symptoms**: Database entities not linked to hosts

**Solutions**:
- Ensure both `host.name` and `service.name` attributes are set
- Verify consistent attribute values across metrics
- Check entity synthesis rules in New Relic

#### 4. Metric Type Mismatches

**Symptoms**: Incorrect aggregations or missing rate calculations

**Solutions**:
- Use cumulativetodelta processor for counter metrics
- Verify metric types match New Relic expectations
- Check temporality settings (delta vs cumulative)

### Debug Configuration

```yaml
# Enable debug logging
service:
  telemetry:
    logs:
      level: debug
      initial_fields:
        service: otel-collector

# Add debug exporter
exporters:
  debug:
    verbosity: detailed
    sampling_initial: 2
    sampling_thereafter: 500

# Add to pipeline for testing
service:
  pipelines:
    metrics:
      exporters: [otlp, debug]
```

## Best Practices

### 1. Resource Management
- Set appropriate memory limits based on metric volume
- Use batching to reduce API calls
- Enable compression for network efficiency

### 2. Data Quality
- Implement PII detection and removal
- Set cardinality limits to control costs
- Use consistent attribute naming

### 3. Monitoring the Monitor
- Export collector self-metrics
- Set up alerts for integration errors
- Track data point usage against limits

### 4. Security
- Use environment variables for sensitive data
- Rotate API keys regularly
- Enable TLS for all connections

### 5. Performance
- Start with longer collection intervals
- Enable sampling for high-volume metrics
- Use filtering to drop unnecessary data

## Cost Optimization

### Estimate Data Usage

```sql
-- Calculate current usage
SELECT 
  sum(metric.count) as 'Total Metrics',
  sum(metric.count) / 60 as 'Metrics Per Minute'
FROM (
  SELECT count(*) as 'metric.count'
  FROM Metric 
  WHERE metric.name LIKE 'postgresql.%'
  FACET metric.name
  LIMIT MAX
  SINCE 1 hour ago
)

-- Project monthly usage
SELECT 
  sum(metric.count) * 24 * 30 as 'Projected Monthly Metrics'
FROM (
  SELECT count(*) as 'metric.count'
  FROM Metric 
  WHERE metric.name LIKE 'postgresql.%'
  SINCE 1 hour ago
)
```

### Optimization Strategies

1. **Adjust Collection Intervals**
   ```yaml
   postgresql:
     collection_interval: 60s  # Increase from 10s
   ```

2. **Enable Sampling**
   ```yaml
   adaptive_sampler:
     rules:
       - metric_pattern: "postgresql.query.*"
         base_rate: 0.1  # Sample 10%
   ```

3. **Filter Unnecessary Metrics**
   ```yaml
   filter:
     metrics:
       exclude:
         match_type: regexp
         metric_names:
           - "postgresql.wal.*"  # If not using replication
   ```

4. **Implement Cost Controls**
   ```yaml
   costcontrol:
     budget:
       max_dpm: 1000000
     priority_rules:
       - pattern: "postgresql.connections.*"
         priority: high
   ```

## Migration from Legacy Integration

### Mapping Legacy Events to Metrics

| Legacy Event | Legacy Attribute | New Metric | Notes |
|--------------|------------------|------------|-------|
| DatabaseSample | provider.connectionCount | postgresql.connections.active | Direct mapping |
| DatabaseSample | provider.transactionsPerSecond | postgresql.transactions.committed | Now a counter, calculate rate |
| DatabaseSample | provider.bufferHitPercent | postgresql.blocks.hit / postgresql.blocks.read | Calculate ratio |
| QuerySample | duration | postgresql.query.duration | Now a histogram |

### Parallel Running Strategy

1. Deploy OTel collector alongside legacy integration
2. Use different `service.name` to avoid conflicts
3. Validate data parity before switching
4. Gradually transition dashboards and alerts
5. Disable legacy integration once validated

## Summary

This integration guide provides a complete path to successfully deploying Database Intelligence with New Relic. Key points:

- Use OTLP protocol for vendor-neutral integration
- Follow semantic conventions for automatic entity creation
- Implement progressive enhancement from basic to advanced monitoring
- Monitor integration health and optimize costs
- Validate thoroughly before production deployment

For additional support, consult New Relic's OpenTelemetry documentation or contact support with specific OTLP-related questions.