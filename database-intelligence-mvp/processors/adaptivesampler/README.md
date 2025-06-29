# Adaptive Sampler Processor

A custom OpenTelemetry Collector processor that provides intelligent sampling with deduplication and file-based state persistence.

## Features

- **Rule-Based Sampling**: Priority-ordered sampling rules with conditions
- **Deduplication**: Hash-based duplicate detection with configurable time windows
- **File-Based State**: Persistent state storage for single-instance deployments
- **Rate Limiting**: Per-rule rate limiting with sliding windows
- **Smart Defaults**: Production-ready sampling strategies included

## Key Capabilities

### Intelligent Sampling Rules
- Always sample critical queries (>1000ms duration)
- Always sample missing index scenarios (seq scans on large tables)  
- Reduce sampling for high-frequency queries
- Configurable conditions and sample rates

### Deduplication Engine
- SHA256-based plan hash deduplication
- Configurable time windows (default: 5 minutes)
- LRU cache with configurable size
- Automatic cleanup of expired hashes

### State Persistence
- File-based storage for single-instance deployments
- Automatic state save/restore on startup
- Configurable sync and compaction intervals
- Backup retention for reliability

## Configuration

```yaml
processors:
  adaptive_sampler:
    # State storage configuration
    state_storage:
      type: file_storage
      file_storage:
        directory: /var/lib/otel/sampling_state
        sync_interval: 10s
        compaction_interval: 300s
        max_size_mb: 100
    
    # Deduplication settings
    deduplication:
      enabled: true
      cache_size: 10000
      window_seconds: 300
      hash_attribute: db.query.plan.hash
    
    # Sampling rules (evaluated by priority)
    rules:
      # Rule 1: Always sample critical queries
      - name: critical_queries
        priority: 100
        sample_rate: 1.0
        conditions:
          - attribute: avg_duration_ms
            operator: gt
            value: 1000
      
      # Rule 2: Always sample missing indexes
      - name: missing_indexes
        priority: 90
        sample_rate: 1.0
        conditions:
          - attribute: db.query.plan.has_seq_scan
            operator: eq
            value: true
          - attribute: db.query.plan.rows
            operator: gt
            value: 10000
      
      # Rule 3: Reduce high-frequency queries
      - name: high_frequency
        priority: 50
        sample_rate: 0.01
        max_per_minute: 10
        conditions:
          - attribute: execution_count
            operator: gt
            value: 1000
      
      # Default rule (lowest priority)
      - name: default
        priority: 0
        sample_rate: 0.1
    
    # Global settings
    default_sample_rate: 0.1
    max_records_per_second: 1000
```

## Sampling Rule Conditions

### Supported Operators
- `eq` - Equals
- `ne` - Not equals  
- `gt` - Greater than
- `gte` - Greater than or equal
- `lt` - Less than
- `lte` - Less than or equal
- `contains` - String contains
- `exists` - Attribute exists

### Example Conditions
```yaml
conditions:
  # Numeric comparison
  - attribute: avg_duration_ms
    operator: gt
    value: 500
  
  # Boolean check
  - attribute: db.query.plan.has_seq_scan
    operator: eq
    value: true
  
  # String matching
  - attribute: database_name
    operator: contains
    value: prod
  
  # Existence check
  - attribute: plan_json
    operator: exists
    value: true
```

## State Management

### File Storage Structure
```
/var/lib/otel/sampling_state/
├── sampler_state.json      # Current state
├── sampler_state.json.bak1 # Backup 1
├── sampler_state.json.bak2 # Backup 2
└── sampler_state.json.bak3 # Backup 3
```

### State Content
- Deduplication hash cache with timestamps
- Rate limiter states and windows
- Metadata (save time, cache statistics)

## Performance Characteristics

### Memory Usage
- Deduplication cache: ~1MB per 10,000 hashes
- Rule limiters: ~1KB per rule
- State storage: Minimal overhead

### Disk Usage
- State file: ~1-10MB depending on cache size
- Grows with number of unique query hashes
- Automatic compaction prevents unbounded growth

### Processing Latency
- Hash lookup: O(1) average case
- Rule evaluation: O(number of rules)
- Typical latency: <1ms per record

## Safety Features

### Single Instance Constraint
⚠️ **CRITICAL**: This processor MUST run as a single instance due to file-based state storage. Multiple instances will have inconsistent deduplication behavior.

### Error Handling
- Graceful degradation on state corruption
- Automatic fallback to default sampling
- State backup and recovery mechanisms

### Resource Protection
- Configurable memory limits for cache
- Rate limiting to prevent data volume spikes
- Timeout protection for all operations

## Monitoring

### Key Metrics
```
# Records processed
otelcol_processor_adaptive_sampler_records_total

# Sampling decisions
otelcol_processor_adaptive_sampler_sampled_total
otelcol_processor_adaptive_sampler_dropped_total

# Deduplication
otelcol_processor_adaptive_sampler_duplicates_total

# State operations
otelcol_processor_adaptive_sampler_state_saves_total
otelcol_processor_adaptive_sampler_state_errors_total
```

### Log Messages
- State save/restore operations
- Cache statistics and cleanup
- Rule matching and sampling decisions (debug mode)

## Production Deployment

### Kubernetes Requirements
```yaml
# StatefulSet with persistent volume
volumeClaimTemplates:
  - metadata:
      name: sampler-state
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 1Gi

# Volume mount
volumeMounts:
  - name: sampler-state
    mountPath: /var/lib/otel/sampling_state
```

### Backup Strategy
1. State files are automatically backed up
2. Use volume snapshots for point-in-time recovery
3. Monitor state file growth and compaction

## Troubleshooting

### High Memory Usage
- Reduce `deduplication.cache_size`
- Decrease `deduplication.window_seconds`
- Enable more aggressive compaction

### State Corruption
```bash
# Check state file integrity
cat /var/lib/otel/sampling_state/sampler_state.json | jq '.'

# Clear corrupted state (will restart fresh)
rm /var/lib/otel/sampling_state/sampler_state.json*
```

### Unexpected Sampling Behavior
- Enable `enable_debug_logging: true`
- Check rule priorities and conditions
- Verify deduplication hash generation

This processor is the foundation for managing data volume while ensuring critical performance issues are always captured.