# Migration Guide: Documentation to Reality

## For New Users

If you're just starting, use these files:

1. **README**: Use `README-ALIGNED.md` - it reflects what actually works
2. **Configuration**: Use `config/collector.yaml` - it's the tested configuration  
3. **Architecture**: Read `ARCHITECTURE-REALITY.md` - it shows the actual design
4. **Deployment**: Use single-instance deployment only

## For Existing Users

If you've been trying to use features that don't work:

### Query Plan Collection

**You Expected**: Real PostgreSQL execution plans  
**Reality**: Static JSON placeholder

**Migration Steps**:
1. Stop expecting plan data in New Relic
2. Use auto_explain logs instead (if available)
3. Wait for safe EXPLAIN implementation

### Adaptive Sampling

**You Expected**: Smart workload-based sampling  
**Reality**: Simple 10% probabilistic sampling

**Migration Steps**:
1. Adjust `sampling_percentage` in `probabilistic_sampler`
2. Don't look for adaptive sampling configuration
3. Monitor data volume and adjust percentage manually

### High Availability

**You Expected**: Multi-instance with leader election  
**Reality**: Single instance only

**Migration Steps**:
1. Scale down to 1 replica immediately
2. Remove any StatefulSet configurations
3. Ignore HA deployment examples for now

### Circuit Breaker

**You Expected**: Per-database failure isolation  
**Reality**: Not active in the collector

**Migration Steps**:
1. Implement connection timeouts instead
2. Monitor failures via collector metrics
3. Manual intervention for problem databases

## Configuration Changes

### Remove Non-Existent Processors

❌ **Before** (Doesn't Work):
```yaml
processors:
  memory_limiter:
  adaptivesampler:    # Doesn't exist
  circuitbreaker:     # Doesn't exist  
  planattributeextractor: # Doesn't exist
```

✅ **After** (Works):
```yaml
processors:
  memory_limiter:
  transform/sanitize_pii:
  probabilistic_sampler:
  batch:
```

### Fix Database Expectations

❌ **Before** (Expecting too much):
```yaml
# Expecting real plans
SELECT pg_get_json_plan(query) as plan
```

✅ **After** (Reality):
```yaml
# Acknowledge we get metadata only
SELECT 
  queryid,
  query,
  mean_exec_time,
  calls
FROM pg_stat_statements
```

### Single Instance Deployment

❌ **Before** (Multiple instances):
```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  replicas: 3  # Causes duplicate data
```

✅ **After** (Single instance):
```yaml
apiVersion: apps/v1
kind: Deployment
spec:
  replicas: 1  # Required for consistency
```

## Monitoring Adjustments

### Metrics That Don't Exist

These metrics are mentioned in docs but don't exist:
- `database_intelligence_adaptive_sampling_rate`
- `database_intelligence_plan_changes_detected`
- `database_intelligence_circuit_breaker_state`

### Metrics That Actually Work

Monitor these instead:
- `otelcol_receiver_accepted_log_records`
- `otelcol_processor_dropped_log_records`
- `otelcol_exporter_sent_log_records`
- `otelcol_process_memory_rss`

## New Relic Queries

### Remove Complex Queries

❌ **Before** (Looking for plan data):
```sql
SELECT * FROM DatabasePlan 
WHERE entity.guid = 'YOUR_DATABASE_GUID'
```

✅ **After** (Query metadata only):
```sql
SELECT 
  average(avg_duration_ms),
  sum(execution_count)
FROM Log
WHERE query_id IS NOT NULL
FACET query_id
SINCE 1 hour ago
```

## File Structure Cleanup

### Move Experimental Code

```bash
# Create experimental directory
mkdir experimental

# Move unintegrated components
mv receivers/postgresqlquery experimental/
mv processors/adaptivesampler experimental/
mv processors/circuitbreaker experimental/
mv processors/planattributeextractor experimental/
```

### Update Documentation

```bash
# Replace with aligned versions
mv README-ALIGNED.md README.md
mv CONFIGURATION-ALIGNED.md CONFIGURATION.md
mv ARCHITECTURE-REALITY.md ARCHITECTURE.md

# Add status document
cp IMPLEMENTATION-STATUS.md docs/
```

## Communication Template

For communicating changes to your team:

> **Subject**: Database Intelligence Collector - Important Updates
> 
> Team,
> 
> We've identified some gaps between our documentation and actual implementation. Here's what you need to know:
> 
> **What Works**:
> - Query performance metadata collection ✅
> - PII sanitization ✅
> - Basic 10% sampling ✅
> - New Relic OTLP export ✅
> 
> **What Doesn't Work** (yet):
> - Query execution plans ❌
> - Adaptive sampling ❌
> - Multi-instance deployment ❌
> - Circuit breaker ❌
> 
> **Action Required**:
> 1. Use single-instance deployment only
> 2. Expect query metadata, not plans
> 3. Adjust sampling percentage manually
> 4. Monitor using standard OTEL metrics
> 
> The current implementation is stable and production-ready. We're being transparent about limitations while we work on enhancements.

## Timeline for Feature Delivery

Based on current state, realistic timeline:

**Q1 2024**: 
- Documentation alignment ✅
- Single instance stability ✅

**Q2 2024**:
- Circuit breaker integration
- Safe EXPLAIN for top query

**Q3 2024**:
- Basic adaptive sampling
- Plan change detection

**Q4 2024**:
- Multi-instance support
- Full plan analysis

## Support During Migration

1. Check `IMPLEMENTATION-STATUS.md` for feature availability
2. Use standard OTEL docs for component behavior
3. Focus on what works rather than what's planned
4. Report issues with actual features, not documented ones

Remember: The current implementation collects valuable query performance data safely. That's a solid foundation to build on.