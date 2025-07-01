# Wait Analysis Processor

The Wait Analysis processor categorizes and analyzes wait events from ASH data, providing insights into database performance bottlenecks and enabling intelligent alerting.

## Overview

The Wait Analysis processor enriches ASH metrics with wait event categorization, pattern matching, and alerting capabilities. It helps identify performance issues by analyzing wait event patterns and trends.

## Features

- **Wait Event Categorization**: Groups wait events into logical categories
- **Pattern Recognition**: Identifies known problematic wait patterns
- **Severity Classification**: Assigns severity levels to wait events
- **Alert Rule Engine**: Triggers alerts based on wait event conditions
- **Metric Enrichment**: Adds category and severity attributes to metrics

## Configuration

### Basic Configuration

```yaml
processors:
  waitanalysis:
    enabled: true
```

### Full Configuration

```yaml
processors:
  waitanalysis:
    enabled: true
    
    # Wait event patterns for categorization
    patterns:
      # Lock-related waits
      - name: lock_waits
        event_types: ["Lock"]
        category: "Concurrency"
        severity: "warning"
        description: "Lock contention between sessions"
      
      # I/O-related waits
      - name: io_waits
        event_types: ["IO"]
        events: ["DataFileRead", "DataFileWrite", "WALWrite"]
        category: "Storage"
        severity: "info"
        description: "Storage I/O operations"
      
      # CPU-related waits
      - name: cpu_waits
        event_types: ["CPU"]
        category: "Compute"
        severity: "info"
        description: "CPU-bound operations"
      
      # Network-related waits
      - name: network_waits
        event_types: ["Client", "IPC"]
        category: "Network"
        severity: "info"
        description: "Network communication waits"
      
      # Buffer-related waits
      - name: buffer_waits
        event_types: ["BufferPin"]
        events: ["BufferPin"]
        category: "Memory"
        severity: "warning"
        description: "Buffer contention"
      
      # Extension-related waits
      - name: extension_waits
        event_types: ["Extension"]
        category: "Extension"
        severity: "info"
        description: "Extension-specific waits"
    
    # Alert rules based on wait events
    alert_rules:
      # Lock wait alerts
      - name: excessive_lock_waits
        condition: "wait_time > 5s AND event_type = 'Lock'"
        threshold: 10           # More than 10 sessions
        window: 1m             # Within 1 minute
        action: alert
      
      # I/O saturation alerts
      - name: io_saturation
        condition: "event IN ('DataFileRead', 'DataFileWrite') AND wait_time > 100ms"
        threshold: 50          # More than 50% of sessions
        window: 5m            # Within 5 minutes
        action: alert
      
      # Blocking chain alerts
      - name: blocking_chain_detected
        condition: "event = 'Lock:transactionid' AND blocking_sessions > 0"
        threshold: 5           # Chain length > 5
        window: 30s           # Within 30 seconds
        action: alert
      
      # Buffer contention alerts
      - name: buffer_contention
        condition: "event = 'BufferPin:BufferPin'"
        threshold: 20          # More than 20 sessions
        window: 2m            # Within 2 minutes
        action: alert
```

## Wait Event Categories

### Concurrency (Lock Waits)
Indicates contention between sessions for database resources.

**Common Events**:
- `Lock:relation` - Table-level lock contention
- `Lock:tuple` - Row-level lock contention
- `Lock:transactionid` - Transaction lock waits
- `Lock:advisory` - Advisory lock waits

**Severity**: Warning/Critical
**Action**: Investigate blocking queries, consider application design changes

### Storage (I/O Waits)
Indicates storage subsystem performance issues.

**Common Events**:
- `IO:DataFileRead` - Reading data from disk
- `IO:DataFileWrite` - Writing data to disk
- `IO:WALWrite` - Writing to WAL
- `IO:WALSync` - Syncing WAL to disk

**Severity**: Info/Warning
**Action**: Check storage performance, consider faster storage or caching

### Compute (CPU Waits)
Indicates CPU-bound operations.

**Common Events**:
- CPU-intensive query execution
- Complex calculations
- Sorting operations

**Severity**: Info
**Action**: Optimize queries, consider adding indexes

### Network (Communication Waits)
Indicates network or client communication issues.

**Common Events**:
- `Client:ClientRead` - Waiting for client input
- `Client:ClientWrite` - Sending data to client
- `IPC:ProcSignal` - Inter-process communication

**Severity**: Info
**Action**: Check network latency, client application performance

### Memory (Buffer Waits)
Indicates memory contention issues.

**Common Events**:
- `BufferPin:BufferPin` - Buffer pin contention
- Shared buffer conflicts

**Severity**: Warning
**Action**: Increase shared_buffers, optimize concurrent access patterns

## Alert Rules

### Rule Syntax

Alert rules support basic conditions:

```yaml
alert_rules:
  - name: rule_name
    condition: "expression"
    threshold: numeric_value
    window: duration
    action: alert|log
```

### Condition Examples

1. **Simple Event Match**:
   ```yaml
   condition: "event_type = 'Lock'"
   ```

2. **Multiple Events**:
   ```yaml
   condition: "event IN ('DataFileRead', 'DataFileWrite')"
   ```

3. **Wait Time Threshold**:
   ```yaml
   condition: "wait_time > 5s AND event_type = 'Lock'"
   ```

4. **Complex Conditions**:
   ```yaml
   condition: "event = 'Lock:relation' AND wait_time > 10s AND sessions > 5"
   ```

## Metrics Enhancement

The processor adds the following attributes to ASH metrics:

### Added Attributes

- **`category`**: Wait event category (Concurrency, Storage, etc.)
- **`severity`**: Event severity (info, warning, critical)
- **`pattern_name`**: Matched pattern name
- **`alert_triggered`**: Whether an alert was triggered

### Example Enhanced Metric

Original:
```json
{
  "name": "postgresql.ash.wait_events.count",
  "value": 15,
  "attributes": {
    "wait_event_type": "Lock",
    "wait_event": "relation"
  }
}
```

Enhanced:
```json
{
  "name": "postgresql.ash.wait_events.count",
  "value": 15,
  "attributes": {
    "wait_event_type": "Lock",
    "wait_event": "relation",
    "category": "Concurrency",
    "severity": "warning",
    "pattern_name": "lock_waits"
  }
}
```

## Integration with ASH Receiver

### Pipeline Configuration

```yaml
service:
  pipelines:
    metrics/ash:
      receivers: [ash]
      processors: [waitanalysis, batch]
      exporters: [prometheus, otlp]
```

### Processing Order

1. ASH receiver collects session snapshots
2. Wait analysis processor categorizes wait events
3. Metrics are enriched with categories and severity
4. Alert rules are evaluated
5. Enhanced metrics are exported

## Alert Integration

### Alert Actions

1. **`alert`**: Generates an alert metric and logs warning
2. **`log`**: Only logs the alert condition

### Alert Metrics

When alerts are triggered, the processor generates:

```
postgresql.ash.wait_alert.triggered{
  rule="excessive_lock_waits",
  severity="warning",
  threshold="10",
  actual_value="15"
} 1
```

### Webhook Integration

Configure alert webhooks in the exporter:

```yaml
exporters:
  webhook:
    endpoint: https://alerts.example.com/webhook
    format: json
    alerts_only: true
```

## Performance Considerations

### Processing Overhead

- **CPU**: Minimal (<1% per 1000 metrics/sec)
- **Memory**: ~10MB for pattern and rule storage
- **Latency**: <1ms per metric

### Optimization Tips

1. **Limit Patterns**: Only configure necessary patterns
2. **Efficient Rules**: Use simple conditions when possible
3. **Batch Processing**: Process metrics in batches

## Troubleshooting

### No Categorization

1. Check pattern configuration:
   ```yaml
   patterns:
     - name: test_pattern
       event_types: ["Lock"]  # Verify event types match
   ```

2. Enable debug logging:
   ```yaml
   service:
     telemetry:
       logs:
         level: debug
   ```

### Alerts Not Firing

1. Verify threshold values
2. Check window duration
3. Ensure condition syntax is correct
4. Monitor actual metric values

### High Memory Usage

1. Reduce alert history retention
2. Simplify pattern matching
3. Decrease rule complexity

## Best Practices

### 1. Pattern Design

**Do**:
- Group related events logically
- Use descriptive categories
- Set appropriate severity levels

**Don't**:
- Create overlapping patterns
- Use overly complex matching
- Set all events to critical severity

### 2. Alert Configuration

**Do**:
- Set realistic thresholds
- Use appropriate time windows
- Test rules in development first

**Don't**:
- Create too many alerts
- Use very short windows (<30s)
- Alert on normal behavior

### 3. Integration

**Do**:
- Place after ASH receiver
- Use with metric batching
- Export to appropriate backends

**Don't**:
- Chain multiple wait analysis processors
- Process non-ASH metrics
- Skip metric validation

## Example Use Cases

### 1. Lock Contention Analysis

```yaml
patterns:
  - name: table_locks
    events: ["relation"]
    category: "TableLocks"
    severity: "critical"

alert_rules:
  - name: table_lock_storm
    condition: "event = 'Lock:relation'"
    threshold: 50
    window: 30s
    action: alert
```

### 2. I/O Performance Monitoring

```yaml
patterns:
  - name: slow_io
    events: ["DataFileRead"]
    category: "SlowIO"
    severity: "warning"

alert_rules:
  - name: io_degradation
    condition: "event = 'IO:DataFileRead' AND wait_time > 500ms"
    threshold: 25
    window: 5m
    action: alert
```

### 3. Application Timeout Detection

```yaml
patterns:
  - name: client_timeouts
    events: ["ClientRead"]
    category: "ClientTimeout"
    severity: "warning"

alert_rules:
  - name: client_timeout_spike
    condition: "event = 'Client:ClientRead' AND wait_time > 30s"
    threshold: 10
    window: 2m
    action: alert
```

## Monitoring the Processor

### Processor Metrics

The processor exposes its own metrics:

- `otelcol_processor_waitanalysis_patterns_matched` - Patterns matched
- `otelcol_processor_waitanalysis_alerts_triggered` - Alerts triggered
- `otelcol_processor_waitanalysis_processing_time` - Processing duration

### Health Checks

```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:13133
    path: /health
    check_collector_pipeline:
      enabled: true
```