# NRDB Events to OTel Metrics Mapping Guide

This document provides a comprehensive mapping between traditional New Relic events and OpenTelemetry (OTel) metrics.

## Overview

New Relic traditionally uses event-based data models (e.g., SystemSample, ProcessSample, DatabaseSample) while OpenTelemetry uses a metric-based approach with standardized semantic conventions.

## Key Differences

### New Relic Events
- **Structure**: Event-based with attributes
- **Query**: Using NRQL (e.g., `SELECT * FROM SystemSample`)
- **Naming**: PascalCase event types with "Sample" suffix
- **Attributes**: Flat structure with specific attribute names

### OTel Metrics
- **Structure**: Metric points with dimensions (labels)
- **Query**: Using NRQL on Metric type (e.g., `SELECT * FROM Metric WHERE metricName = 'system.cpu.utilization'`)
- **Naming**: Dot-separated, lowercase with semantic conventions
- **Attributes**: Hierarchical with standard prefixes (e.g., `otel.`, `service.`)

## Common Mappings

### System Metrics

| New Relic Event | NR Attribute | OTel Metric | OTel Unit | Description |
|-----------------|--------------|-------------|-----------|-------------|
| SystemSample | cpuPercent | system.cpu.utilization | 1 (ratio) | CPU utilization percentage |
| SystemSample | memoryUsedPercent | system.memory.utilization | 1 (ratio) | Memory utilization percentage |
| SystemSample | memoryUsedBytes | system.memory.usage | By | Memory usage in bytes |
| SystemSample | diskUsedPercent | system.filesystem.utilization | 1 (ratio) | Disk utilization percentage |
| SystemSample | diskReadBytesPerSecond | system.disk.io | By | Disk I/O bytes |
| SystemSample | networkReceiveBytesPerSecond | system.network.io | By | Network I/O bytes |

### Process Metrics

| New Relic Event | NR Attribute | OTel Metric | OTel Unit | Description |
|-----------------|--------------|-------------|-----------|-------------|
| ProcessSample | cpuPercent | process.cpu.utilization | 1 (ratio) | Process CPU utilization |
| ProcessSample | memoryResidentSizeBytes | process.memory.usage | By | Process memory usage |
| ProcessSample | threadCount | process.threads | {threads} | Number of threads |
| ProcessSample | fileDescriptorCount | process.open_file_descriptors | {files} | Open file descriptors |

### Database Metrics

| New Relic Event | NR Attribute | OTel Metric | OTel Unit | Description |
|-----------------|--------------|-------------|-----------|-------------|
| DatabaseSample | db.connectionCount | db.client.connections.usage | {connections} | Active DB connections |
| DatabaseSample | db.maxConnections | db.client.connections.max | {connections} | Maximum DB connections |
| DatastoreSample | query.averageDuration | db.query.duration | ms | Query execution time |
| PostgresqlDatabaseSample | db.bufferHitRatio | postgresql.buffer_cache.hit_ratio | 1 (ratio) | Buffer cache hit ratio |
| PostgresqlDatabaseSample | db.blocksRead | postgresql.blocks_read | {blocks} | Blocks read from disk |

### Application Metrics

| New Relic Event | NR Attribute | OTel Metric | OTel Unit | Description |
|-----------------|--------------|-------------|-----------|-------------|
| Transaction | duration | http.server.duration | ms | HTTP request duration |
| Transaction | errorCount | http.server.errors | {errors} | HTTP error count |
| Transaction | throughput | http.server.request_count | {requests} | Request throughput |
| Span | duration | trace.span.duration | ms | Span duration |

## Querying Examples

### New Relic Event Query
```sql
SELECT average(cpuPercent), average(memoryUsedPercent) 
FROM SystemSample 
WHERE hostname = 'web-server-01' 
SINCE 1 hour ago 
TIMESERIES
```

### Equivalent OTel Metric Query
```sql
SELECT average(system.cpu.utilization), average(system.memory.utilization) 
FROM Metric 
WHERE host.name = 'web-server-01' 
SINCE 1 hour ago 
TIMESERIES
```

## Attribute Mapping

### Common Dimension Mappings

| New Relic Attribute | OTel Attribute | Description |
|---------------------|----------------|-------------|
| hostname | host.name | Host identifier |
| entityName | service.name | Service name |
| processDisplayName | process.executable.name | Process name |
| processId | process.pid | Process ID |
| container.id | container.id | Container identifier |
| aws.region | cloud.region | Cloud region |
| aws.accountId | cloud.account.id | Cloud account ID |

### Resource Attributes

OTel metrics include resource attributes that identify the source:

- `service.name`: Name of the service
- `service.namespace`: Service namespace
- `service.instance.id`: Unique instance identifier
- `service.version`: Service version
- `telemetry.sdk.name`: OTel SDK name
- `telemetry.sdk.version`: OTel SDK version
- `telemetry.sdk.language`: Programming language

## Best Practices

1. **Use Semantic Conventions**: Follow OTel semantic conventions for consistent metric naming
2. **Include Resource Attributes**: Always include service.name and other identifying attributes
3. **Proper Units**: Use standard units (By for bytes, ms for milliseconds, etc.)
4. **Metric Types**: Choose appropriate metric types (gauge, counter, histogram)
5. **Cardinality**: Be mindful of label cardinality to avoid metric explosion

## Migration Considerations

When migrating from New Relic events to OTel metrics:

1. **Data Continuity**: Plan for parallel collection during transition
2. **Dashboard Updates**: Update NRQL queries to use Metric instead of event types
3. **Alert Updates**: Modify alert conditions to use new metric names
4. **Historical Data**: Consider data retention needs for historical comparisons
5. **Custom Attributes**: Map custom attributes to OTel conventions

## Tools and Scripts

Use the provided Python script to analyze your specific mappings:

```bash
# Analyze all events and metrics
python scripts/analyze-nrdb-otel-mapping.py

# Analyze specific event type
python scripts/analyze-nrdb-otel-mapping.py --event-type SystemSample

# Analyze specific metric
python scripts/analyze-nrdb-otel-mapping.py --metric-name system.cpu.utilization
```

The script will generate a detailed JSON report with:
- Available event types and their attributes
- OTel metrics and their dimensions
- Suggested mappings between events and metrics
- Common attribute mappings