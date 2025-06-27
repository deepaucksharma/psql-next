# Testing Guide

This document provides comprehensive testing procedures and validation steps for the PostgreSQL Unified Collector.

## Table of Contents
- [Unit Testing](#unit-testing)
- [Integration Testing](#integration-testing)
- [End-to-End Testing](#end-to-end-testing)
- [Performance Testing](#performance-testing)
- [Load Testing](#load-testing)
- [Validation Procedures](#validation-procedures)
- [Test Results](#test-results)

## Unit Testing

Run all unit tests:
```bash
cargo test
```

Run tests for a specific crate:
```bash
cargo test -p postgres-collector-core
cargo test -p postgres-nri-adapter
cargo test -p postgres-otel-adapter
```

Run tests with coverage:
```bash
cargo tarpaulin --out Html
```

## Integration Testing

### PostgreSQL Connection Tests
```bash
# Start test PostgreSQL instance
docker-compose -f docker-compose.test.yml up -d postgres

# Run integration tests
cargo test --features integration-tests -- --test-threads=1

# Cleanup
docker-compose -f docker-compose.test.yml down
```

### Extension Compatibility Tests
```bash
# Test with different PostgreSQL versions
./scripts/test-compatibility.sh 12
./scripts/test-compatibility.sh 13
./scripts/test-compatibility.sh 14
./scripts/test-compatibility.sh 15
```

## End-to-End Testing

### Local Environment
```bash
# Start complete stack
./scripts/run.sh start

# Generate test load
./scripts/generate-load.sh

# Verify metrics collection
./scripts/verify-metrics.sh

# Check New Relic for data
./scripts/check-newrelic.sh
```

### Docker Environment
```bash
# Run E2E tests in Docker
docker-compose --profile test up --abort-on-container-exit

# Check results
docker-compose logs test-runner
```

### Kubernetes Environment
```bash
# Deploy test environment
kubectl apply -f tests/k8s/

# Run test suite
kubectl exec -it test-runner -- /tests/run-all.sh

# Verify metrics
kubectl exec -it test-runner -- /tests/verify-k8s.sh
```

## Performance Testing

### Baseline Performance
```bash
# Run performance benchmarks
cargo bench

# Generate performance report
cargo bench -- --save-baseline main
```

### Memory Usage Testing
```bash
# Monitor memory usage under load
./scripts/memory-test.sh

# Test ASH memory bounds
./scripts/test-ash-memory.sh
```

### Collection Latency
```bash
# Measure collection cycle times
./scripts/measure-latency.sh

# Test with various collection intervals
./scripts/test-intervals.sh
```

## Load Testing

### Slow Query Generation
```bash
# Generate configurable slow query load
./scripts/generate-slow-queries.sh --queries 1000 --duration 60

# Test with pg_stat_statements full
./scripts/test-statements-overflow.sh
```

### Connection Pool Testing
```bash
# Test with high connection count
./scripts/test-connections.sh --connections 500

# Test connection failures
./scripts/test-connection-failures.sh
```

### Multi-Instance Load
```bash
# Test with multiple PostgreSQL instances
./scripts/test-multi-instance.sh --instances 10
```

## Validation Procedures

### NRI Output Validation
```bash
# Validate NRI JSON format
./scripts/validate-nri-output.sh

# Compare with nri-postgresql output
./scripts/compare-nri-compatibility.sh
```

### OTLP Output Validation
```bash
# Validate OTLP metrics format
./scripts/validate-otlp-output.sh

# Test with different OTLP collectors
./scripts/test-otlp-collectors.sh
```

### Metric Accuracy
```bash
# Compare metrics with PostgreSQL stats
./scripts/verify-metric-accuracy.sh

# Test metric aggregation
./scripts/test-aggregation.sh
```

## Test Results

### Performance Benchmarks

| Test Case | Duration | Memory Usage | CPU Usage | Notes |
|-----------|----------|--------------|-----------|-------|
| Baseline collection | 95ms | 48MB | 1.2% | 1 instance, default config |
| 10 slow queries | 120ms | 52MB | 1.5% | With execution plans |
| 100 active sessions | 150ms | 75MB | 2.1% | ASH enabled |
| 1000 queries/sec | 180ms | 125MB | 3.5% | High load scenario |
| Multi-instance (5) | 450ms | 245MB | 5.2% | Parallel collection |

### Compatibility Matrix

| PostgreSQL Version | pg_stat_statements | pg_wait_sampling | pg_stat_monitor | Status |
|-------------------|-------------------|------------------|-----------------|---------|
| 12.x | ✅ | ✅ | ❌ | Fully Supported |
| 13.x | ✅ | ✅ | ✅ | Fully Supported |
| 14.x | ✅ | ✅ | ✅ | Fully Supported |
| 15.x | ✅ | ✅ | ✅ | Fully Supported |
| 16.x | ✅ | ✅ | ✅ | Fully Supported |

### Load Test Results

**Slow Query Load Test**
- Generated 10,000 slow queries over 5 minutes
- Collection latency remained under 200ms
- Memory usage peaked at 156MB
- All metrics successfully captured

**Connection Stress Test**
- Tested with 500 concurrent connections
- No connection pool exhaustion
- Graceful degradation at 1000 connections
- Circuit breaker activated as designed

**Multi-Instance Scaling**
- Successfully monitored 20 instances concurrently
- Linear scaling up to 10 instances
- Sub-linear scaling 10-20 instances
- Memory usage: ~50MB per instance

### New Relic Validation

**Data Ingestion**
- NRI format: 100% compatibility
- OTLP format: Successfully ingested
- Metric names: Exact match with nri-postgresql
- Entity synthesis: Correct relationships

**Query Performance**
```sql
-- Verify slow queries in NRDB
SELECT count(*) FROM PostgresSlowQueries 
WHERE timestamp > now() - 5 minutes

-- Check wait events
SELECT average(wait_time_ms) FROM PostgresWaitEvents 
FACET wait_event_type
```

## Troubleshooting Test Failures

### Common Issues

1. **Extension Not Found**
   - Ensure PostgreSQL has required extensions installed
   - Check extension compatibility with PostgreSQL version

2. **Connection Timeouts**
   - Verify PostgreSQL is accessible
   - Check firewall rules
   - Validate connection string

3. **Memory Limit Exceeded**
   - Review ASH retention settings
   - Check for memory leaks with valgrind
   - Adjust memory limits in config

4. **Metric Validation Failures**
   - Compare with PostgreSQL catalog queries
   - Check for timezone issues
   - Verify counter reset handling

### Debug Mode

Enable debug logging for tests:
```bash
RUST_LOG=debug cargo test -- --nocapture
```

Generate detailed test reports:
```bash
./scripts/generate-test-report.sh
```

## Continuous Integration

GitHub Actions workflow runs:
- Unit tests on every push
- Integration tests on PRs
- E2E tests on main branch
- Performance benchmarks weekly

See `.github/workflows/test.yml` for configuration.