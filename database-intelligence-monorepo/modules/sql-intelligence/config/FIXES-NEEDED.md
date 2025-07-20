# SQL Intelligence Configuration Fixes

Based on validation and debugging, the following fixes are needed for the full collector.yaml:

## 1. OTTL Transform Processor Fixes

### Issue: Context mismatch
- Current: `context: metric` 
- Fix: Use `context: datapoint` for attribute operations

### Issue: Function availability
- `Hour()`, `Now()`, `DayOfWeek()` functions not available in metric context
- Simplify or use static values for MVP

## 2. Routing Processor

### Issue: Component type mismatch
- `routing` is available as a **connector**, not a processor
- Either remove routing or restructure pipelines to use it as a connector

## 3. Memory Ballast Extension

### Issue: Extension not found
- `memory_ballast` extension is deprecated/removed
- Remove from extensions section

## 4. Metric Transform Naming

### Issue: Incorrect processor name
- Change `metrictransform` to `metricstransform` (with 's')

## 5. SQL Query Data Types

### Issue: Float to int conversion errors
- Add `value_type: double` for metrics with decimal values
- Use proper ROUND() functions in SQL queries

## 6. Prometheus Exporter Namespace

### Issue: Duplicate label names
- The `namespace: mysql` setting causes conflicts
- Consider removing namespace or adjusting attribute names

## 7. Pipeline Optimization

### Recommendations:
1. Start with simplified transform processors
2. Add complexity incrementally after validation
3. Consider using `filter` processor instead of complex routing
4. Test each processor in isolation

## Working Configuration Pattern

The working configuration demonstrates:
- Simple attribute enrichment without complex OTTL
- Direct metric collection without transformation
- Basic filtering and batching
- Successful New Relic integration

## Next Steps

1. Apply fixes incrementally to collector.yaml
2. Test after each major change
3. Validate metrics in NRDB at each step
4. Document any new issues discovered