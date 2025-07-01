# Active Session History (ASH) Receiver

The ASH receiver provides high-frequency sampling of active database sessions, enabling detailed performance analysis and troubleshooting similar to Oracle's Active Session History.

## Overview

The ASH receiver samples `pg_stat_activity` at regular intervals (typically 1 second) to capture a detailed history of database activity. It includes advanced features like adaptive sampling, wait event analysis, blocking detection, and multi-window aggregation.

## Features

- **High-Frequency Sampling**: 1-second sampling interval for detailed visibility
- **Adaptive Sampling**: Automatically adjusts sampling based on system load
- **Wait Event Analysis**: Categorizes and tracks wait events
- **Blocking Detection**: Identifies blocking chains and lock dependencies
- **Resource Tracking**: Monitors CPU, I/O, and memory usage patterns
- **Time-Window Aggregation**: Multiple aggregation windows (1m, 5m, 15m, 1h)
- **Feature Detection**: Automatically detects available PostgreSQL extensions

## Configuration

### Basic Configuration

```yaml
receivers:
  ash:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    database: postgres
    collection_interval: 1s
```

### Full Configuration

```yaml
receivers:
  ash:
    # Database connection
    endpoint: localhost:5432
    username: postgres
    password: postgres
    database: postgres
    
    # Collection settings
    collection_interval: 1s        # Standard ASH sampling interval
    retention_duration: 1h         # Keep 1 hour of detailed data
    
    # Adaptive sampling configuration
    sampling:
      enabled: true
      sample_rate: 1.0             # Base sample rate (100%)
      active_session_rate: 1.0     # Sample all active sessions
      blocked_session_rate: 1.0    # Sample all blocked sessions
      long_running_threshold: 10s  # Mark queries > 10s as long-running
      adaptive_sampling: true      # Reduce sampling under high load
    
    # Storage configuration
    storage:
      buffer_size: 3600            # 1 hour at 1 sample/second
      aggregation_windows:
        - 1m                       # 1-minute aggregations
        - 5m                       # 5-minute aggregations
        - 15m                      # 15-minute aggregations
        - 1h                       # 1-hour aggregations
      compression_enabled: true    # Enable snapshot compression
    
    # Analysis features
    analysis:
      wait_event_analysis: true    # Categorize and analyze wait events
      blocking_analysis: true      # Detect blocking chains
      resource_analysis: true      # Track resource usage patterns
      anomaly_detection: true      # Detect unusual patterns
      top_query_analysis: true     # Identify top resource consumers
      trend_analysis: true         # Track performance trends
    
    # Feature detection
    feature_detection:
      enabled: true
      check_interval: 5m
      required_extensions:
        - pg_stat_statements       # For query identification
        - pg_wait_sampling         # For detailed wait events (optional)
```

## Metrics

The ASH receiver generates comprehensive metrics about database activity:

### Session Metrics

- **`postgresql.ash.sessions.count`** (gauge)
  - Description: Number of sessions by state
  - Unit: {sessions}
  - Attributes: `state` (active, idle, idle_in_transaction, etc.)

- **`postgresql.ash.sessions.duration`** (gauge)
  - Description: Duration of current session state
  - Unit: seconds
  - Attributes: `state`, `username`, `application_name`

### Wait Event Metrics

- **`postgresql.ash.wait_events.count`** (gauge)
  - Description: Number of sessions waiting on specific events
  - Unit: {sessions}
  - Attributes: `wait_event_type`, `wait_event`, `category`, `severity`

- **`postgresql.ash.wait_category.count`** (gauge)
  - Description: Count of sessions by wait event category
  - Unit: {sessions}
  - Attributes: `category` (Lock, IO, CPU, Network, IPC)

### Blocking Metrics

- **`postgresql.ash.blocking_sessions.count`** (gauge)
  - Description: Number of sessions blocking other sessions
  - Unit: {sessions}
  - Attributes: `username`, `application_name`

- **`postgresql.ash.blocked_sessions.count`** (gauge)
  - Description: Number of sessions blocked by other sessions
  - Unit: {sessions}
  - Attributes: `wait_event`, `blocking_pid`

### Query Metrics

- **`postgresql.ash.query.active_count`** (gauge)
  - Description: Number of active sessions per query
  - Unit: {sessions}
  - Attributes: `query_id`

- **`postgresql.ash.query.duration`** (gauge)
  - Description: Current duration of active queries
  - Unit: seconds
  - Attributes: `query_id`, `username`

## Adaptive Sampling

The ASH receiver includes intelligent adaptive sampling to manage overhead:

### Sampling Rules

1. **Always Sampled**:
   - Blocked sessions
   - Sessions with critical wait events (locks, buffer pins)
   - Long-running queries (> threshold)
   - Autovacuum and background workers

2. **Load-Based Adjustment**:
   - < 50 sessions: 100% sampling
   - 50-500 sessions: Configured sample rate
   - > 500 sessions: Reduced sampling (10-90% reduction)

3. **Session-Specific Rates**:
   - Active sessions: `base_rate * active_session_rate`
   - Blocked sessions: `base_rate * blocked_session_rate`
   - Idle sessions: Lower sampling rate

### Configuration Example

```yaml
sampling:
  enabled: true
  sample_rate: 1.0                 # Start with 100%
  active_session_rate: 1.0         # Keep all active sessions
  blocked_session_rate: 1.0        # Keep all blocked sessions
  long_running_threshold: 10s      # Always sample queries > 10s
  adaptive_sampling: true          # Enable load-based adjustment
```

## Wait Event Analysis

The ASH receiver categorizes wait events for better analysis:

### Wait Event Categories

1. **Lock Waits** (Concurrency)
   - `Lock:relation` - Table-level locks
   - `Lock:tuple` - Row-level locks
   - `Lock:transactionid` - Transaction locks
   - `Lock:advisory` - Advisory locks

2. **I/O Waits** (Storage)
   - `IO:DataFileRead` - Reading data files
   - `IO:DataFileWrite` - Writing data files
   - `IO:WALWrite` - Writing WAL files

3. **CPU Waits** (Compute)
   - CPU-bound operations
   - Query execution

4. **Network Waits** (Communication)
   - `Client:ClientRead` - Waiting for client
   - `Client:ClientWrite` - Sending to client

5. **IPC Waits** (Internal)
   - Process communication
   - Shared memory access

### Wait Analysis Processor

Enhance wait event analysis with the wait analysis processor:

```yaml
processors:
  waitanalysis:
    enabled: true
    patterns:
      - name: lock_waits
        event_types: ["Lock"]
        category: "Concurrency"
        severity: "warning"
    
    alert_rules:
      - name: excessive_lock_waits
        condition: "wait_time > 5s AND event_type = 'Lock'"
        threshold: 10
        window: 1m
        action: alert
```

## Blocking Analysis

The ASH receiver provides detailed blocking chain analysis:

### Blocking Detection Query

The receiver uses an optimized query to detect blocking relationships:

```sql
WITH blocking_info AS (
  SELECT 
    blocked.pid AS blocked_pid,
    blocking.pid AS blocking_pid,
    blocked_locks.locktype
  FROM pg_locks blocked_locks
  JOIN pg_locks blocking_locks ON ...
  WHERE NOT blocked_locks.granted
)
```

### Blocking Metrics

- Identifies blocking chains
- Tracks lock types involved
- Monitors blocking duration
- Detects deadlock risks

## Storage and Aggregation

### Circular Buffer

- Fixed-size memory buffer
- Automatic oldest-data eviction
- Configurable retention period
- Efficient memory usage

### Time-Window Aggregation

Maintains aggregated data for multiple windows:

```yaml
storage:
  buffer_size: 3600              # 1 hour of raw data
  aggregation_windows:
    - 1m                         # Last minute details
    - 5m                         # 5-minute summaries
    - 15m                        # 15-minute trends
    - 1h                         # Hourly overview
```

### Aggregated Metrics

For each window:
- Session count by state
- Top queries by activity
- Wait event distribution
- Resource usage patterns

## Best Practices

### 1. Sampling Configuration

**Development**:
```yaml
sampling:
  sample_rate: 1.0               # Sample everything
  adaptive_sampling: false       # Disable adaptation
```

**Production**:
```yaml
sampling:
  sample_rate: 0.2               # Sample 20% baseline
  adaptive_sampling: true        # Enable adaptation
  active_session_rate: 5.0       # Boost active sessions to 100%
```

### 2. Resource Management

- **Memory Usage**: ~50MB for 1 hour of data (3600 samples)
- **CPU Overhead**: < 1% with adaptive sampling
- **Network Traffic**: Minimal (local database queries)

### 3. Integration Patterns

**With Plan Intelligence**:
```yaml
pipelines:
  metrics/ash:
    receivers: [ash]
    processors: [waitanalysis, planattributeextractor]
    exporters: [otlp]
```

**With Anomaly Detection**:
```yaml
processors:
  anomalydetector/sessions:
    rules:
      - name: session_spike
        metric: session_count
        method: stddev
        threshold: 3
```

## Troubleshooting

### High Memory Usage

1. Reduce buffer size:
   ```yaml
   storage:
     buffer_size: 1800  # 30 minutes instead of 1 hour
   ```

2. Enable compression:
   ```yaml
   storage:
     compression_enabled: true
   ```

3. Reduce sampling rate:
   ```yaml
   sampling:
     sample_rate: 0.1  # 10% sampling
   ```

### Missing Data

1. Check database connectivity
2. Verify required permissions:
   ```sql
   GRANT SELECT ON pg_stat_activity TO monitoring_user;
   GRANT SELECT ON pg_locks TO monitoring_user;
   ```

3. Enable debug logging:
   ```yaml
   service:
     telemetry:
       logs:
         level: debug
   ```

### Performance Impact

Monitor collector performance:
```yaml
service:
  telemetry:
    metrics:
      level: detailed
      address: 0.0.0.0:8888
```

## Security Considerations

1. **Connection Security**:
   - Use SSL/TLS connections
   - Implement connection pooling
   - Use read-only credentials

2. **Data Protection**:
   - Query text may contain sensitive data
   - Consider query obfuscation
   - Implement access controls

3. **Resource Limits**:
   - Set appropriate memory limits
   - Configure connection limits
   - Implement circuit breakers

## Example Dashboards

### Session Activity Dashboard
- Current session distribution by state
- Wait event heatmap
- Blocking chain visualization
- Top queries by active sessions

### Performance Analysis Dashboard
- Query duration trends
- Wait event categories over time
- Resource utilization patterns
- Anomaly detection alerts

### Troubleshooting Dashboard
- Current blocking chains
- Long-running queries
- Wait event details
- Session history timeline

## Advanced Use Cases

### 1. Workload Characterization
```yaml
analysis:
  workload_profiling: true
  profile_categories:
    - oltp
    - olap
    - maintenance
```

### 2. Capacity Planning
- Track session growth trends
- Identify resource bottlenecks
- Predict scaling needs

### 3. Performance Forensics
- Historical session analysis
- Incident investigation
- Root cause analysis

## Integration Examples

### With Prometheus
```yaml
exporters:
  prometheus:
    endpoint: 0.0.0.0:8889
    namespace: ash
    metric_expiration: 5m
```

### With New Relic
```yaml
exporters:
  otlp/newrelic:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
```

### With Grafana
```yaml
exporters:
  prometheus:
    endpoint: 0.0.0.0:8889
    
# Grafana data source: Prometheus
# Dashboard templates available in /dashboards/
```