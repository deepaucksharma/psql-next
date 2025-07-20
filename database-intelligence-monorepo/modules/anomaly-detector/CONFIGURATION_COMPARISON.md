# Anomaly Detector Configuration Comparison

## Summary of Important Features Preserved

### From collector-enterprise.yaml (Original Default)
✅ **Z-score Calculation Logic**: Fully preserved in collector-functional.yaml
- Connection spike detection with z-score calculation
- Query latency deviation detection
- Wait event anomaly detection
- Resource usage pattern detection

✅ **Severity Classification**: Preserved with improved logic
- Low, medium, high, critical severity levels based on z-score thresholds
- Both positive and negative deviations are detected

✅ **Federation from Other Modules**: Preserved
- Pulls metrics from core-metrics, sql-intelligence, and wait-profiler
- Same metric filtering patterns maintained

✅ **Alert Generation**: Preserved with improvements
- Creates anomaly_alert metrics for high severity anomalies
- Alert messages include context (z-score, current value, baseline)

✅ **New Relic Integration**: Improved
- Dynamic entity synthesis based on instance labels (fixes multi-target issue)
- Proper attributes for New Relic entity creation

### Key Improvements in collector-functional.yaml

1. **Single Pipeline Architecture**
   - Processes metrics only once (fixes the inefficient dual pipeline issue)
   - Uses routing processor to send alerts to different exporters

2. **Dynamic Entity Synthesis**
   - Entity GUID now includes instance attribute: `ANOMALY|cluster|instance`
   - Prevents "Franken-entity" problem in multi-database environments

3. **Cleaner Configuration**
   - Removed duplicate processing logic
   - More maintainable with generic z-score calculation
   - Clear separation of concerns

4. **Additional Metrics Support**
   - Added support for mysql_threads_running
   - Added support for mysql_buffer_pool_usage
   - Generic anomaly detection for any metric with baselines

### Features Removed (Not Actually Implemented)

❌ **Dynamic Baseline Learning**: Was never implemented, only static values used
❌ **ML/AI Detection**: Never existed, only statistical z-score
❌ **Seasonal Adjustment**: Not implemented in any configuration
❌ **External Baseline Service**: baseline_calculator.py was unused

## Migration Guide

To use the new functional configuration:

```bash
# Default will now use collector-functional.yaml
docker-compose up -d

# To use old configuration (not recommended)
COLLECTOR_CONFIG=collector-enterprise.yaml docker-compose up -d
```

## Metrics Collected

The functional configuration preserves all important metrics:

### Input Metrics (via Federation)
- mysql_connections_current
- mysql_query_duration_milliseconds
- mysql_wait_time_total
- mysql_threads_running
- mysql_buffer_pool_usage
- mysql_operations
- mysql_slow_queries
- mysql_statement_executions

### Output Metrics
- anomaly_score_* (z-scores for detected anomalies)
- anomaly_alert (high severity alerts)
- All original metrics with anomaly attributes added

## Recommendation

Use `collector-functional.yaml` as it:
1. Preserves all actual working functionality
2. Fixes the identified architectural issues
3. Improves New Relic entity handling
4. Is more maintainable and efficient

The other configurations can be archived or removed as they either don't work correctly or duplicate functionality.