# Database Intelligence Complete Dashboard Guide

## Overview

The Database Intelligence Complete Dashboard provides comprehensive monitoring across all custom OpenTelemetry components, offering deep insights into database performance, system-level metrics, error tracking, and cost control.

## Dashboard Pages

### 1. Overview Page
High-level health indicators and key performance metrics:
- **Active Database Sessions**: Real-time session states (active, idle, etc.)
- **Blocked Sessions**: Sessions waiting on locks
- **Long Running Queries**: Queries exceeding thresholds
- **Top Wait Events**: Most common database wait events
- **Query Performance Categories**: Distribution of slow/moderate/fast queries
- **Collection Health**: Status of ASH and kernel metric collection

### 2. Query Analysis Page
Deep dive into query performance and correlations:
- **Query Execution Patterns**: Trends by statement type (SELECT, INSERT, UPDATE, etc.)
- **Query Load Distribution**: Which queries contribute most to database load
- **Tables Needing Maintenance**: Tables with high dead tuple counts
- **Query Performance Trends**: Percentile analysis (p50, p90, p99)
- **Database Load Contributors**: Top queries by load contribution

### 3. System Performance Page
Kernel-level metrics from eBPF monitoring:
- **System Call Activity**: Rate of system calls by type
- **File I/O Throughput**: Read/write throughput trends
- **File Read Latency**: Distribution histogram
- **Lock Contention**: Breakdown by lock type (mutex, rwlock, spinlock, futex)
- **CPU Usage by Function**: Top CPU-consuming functions
- **Database Query Starts**: Query initiation rate

### 4. Error Monitoring Page
Integration error detection and tracking:
- **Potential NR Integration Errors**: Errors by category
- **Error Timeline**: Temporal error patterns
- **Recent Errors**: Detailed error messages and timing
- **Error Categories Distribution**: Pie chart of error types
- **Time Since Last Error**: Health indicator

### 5. Cost Control Page
Data ingestion and cost management:
- **Ingestion Rate by Pipeline**: Metrics vs logs ingestion rates
- **Metric Cardinality**: Unique metric count over time
- **Top Metrics by Volume**: Highest volume metrics
- **Adaptive Sampling Stats**: Sampling effectiveness

### 6. Database Details Page
PostgreSQL-specific metrics:
- **Connection Pool Status**: Backend connection states
- **Transaction Rate**: Commits and rollbacks per minute
- **Buffer Cache Hit Ratio**: Cache effectiveness
- **Database Size Growth**: Storage trends
- **Table Bloat Analysis**: Dead rows and vacuum stats
- **Replication Lag**: Replica synchronization status
- **Checkpoint Activity**: Write checkpoint patterns

## Metrics Reference

### ASH Receiver Metrics
| Metric Name | Description | Key Attributes |
|------------|-------------|----------------|
| `db.ash.active_sessions` | Active database sessions | state, database_name |
| `db.ash.wait_events` | Sessions waiting on events | wait_event_type, wait_event, database_name |
| `db.ash.blocked_sessions` | Blocked session count | database_name |
| `db.ash.blocking_chain_depth` | Max blocking chain depth | database_name |
| `db.ash.long_running_queries` | Queries exceeding threshold | database_name, threshold_ms |
| `db.ash.collection_stats` | Collection statistics | stat_type |

### KernelMetrics Receiver Metrics
| Metric Name | Description | Key Attributes |
|------------|-------------|----------------|
| `kernel.syscall.count` | System call counts | syscall, process |
| `kernel.file.read.bytes` | File read volume | process |
| `kernel.file.read.latency` | File read latency | process |
| `kernel.cpu.usage` | CPU usage by function | function, process |
| `kernel.lock.contentions` | Lock contention events | lock_type, process |
| `kernel.db.query.start` | Database query starts | process, db_type |
| `kernel.collection.stats` | eBPF collection stats | stat |

### Processor-Added Attributes
| Processor | Attributes Added | Purpose |
|-----------|-----------------|---------|
| QueryCorrelator | `correlation.*`, `performance.category`, `query.*`, `table.*`, `database.*` | Query-table-database correlation |
| NRErrorMonitor | `error.category`, `error.last_message`, `error.minutes_since_last` | Error tracking |
| AdaptiveSampler | `adaptivesampler.sampled`, `adaptivesampler.dropped`, `adaptivesampler.rule_name` | Sampling decisions |

## Query Patterns

### Basic Metric Query
```sql
SELECT latest(metric_name) 
FROM Metric 
WHERE instrumentation.provider = 'otel'
```

### Time Series with Faceting
```sql
SELECT latest(metric_name) 
FROM Metric 
FACET attribute_name 
TIMESERIES AUTO 
WHERE instrumentation.provider = 'otel'
```

### Percentile Analysis
```sql
SELECT percentile(metric_name, 50, 90, 99) 
FROM Metric 
WHERE instrumentation.provider = 'otel'
```

### Rate Calculations
```sql
SELECT rate(sum(metric_name), 1 minute) 
FROM Metric 
WHERE instrumentation.provider = 'otel'
```

## Deployment Instructions

1. **Prerequisites**:
   - New Relic account with API key
   - Account ID
   - jq and curl installed

2. **Deploy Dashboard**:
   ```bash
   ./verify-and-deploy-dashboard.sh <account_id> <api_key>
   ```

3. **Configure Collector**:
   ```yaml
   exporters:
     otlp:
       endpoint: otlp.nr-data.net:4317
       headers:
         api-key: YOUR_NEW_RELIC_LICENSE_KEY
   ```

## Troubleshooting

### No Data Appearing
1. Verify collector is running: `./otelcol-complete --config=config.yaml`
2. Check New Relic API key is correct
3. Ensure `instrumentation.provider = 'otel'` is set
4. Wait 2-3 minutes for data to appear

### Query Validation Failures
- Expected for metrics not yet collected
- Dashboard can be created anyway
- Queries will work once data flows

### Missing Metrics
- ASH receiver requires database connection configuration
- KernelMetrics requires appropriate permissions (CAP_BPF)
- Some metrics only appear under specific conditions

## Customization

### Adding Filters
Add WHERE clauses to focus on specific databases:
```sql
WHERE database_name = 'production'
```

### Adjusting Time Ranges
Use the time range variable or modify queries:
```sql
SINCE 1 hour ago UNTIL now
```

### Creating Alerts
Convert any query to an alert condition:
1. Copy the NRQL query
2. Create alert in New Relic UI
3. Set thresholds based on widget configurations