# Performance and Stress Testing

This directory contains performance and stress testing scenarios for the Database Intelligence MVP.

## Test Scenarios

### 1. Baseline Performance Tests
- **Purpose**: Establish performance baselines for metric collection
- **Metrics**: CPU usage, memory consumption, metric throughput, latency
- **Target**: Process 10,000 metrics/second with <100ms latency

### 2. High Volume Query Tests  
- **Purpose**: Test system behavior under high query volumes
- **Scenarios**: 
  - 1,000 concurrent queries
  - 10,000 unique query patterns
  - 100,000 queries/minute
- **Validation**: Adaptive sampling adjusts correctly, no metric loss

### 3. Memory Pressure Tests
- **Purpose**: Validate memory limiter and circuit breaker behavior
- **Scenarios**:
  - Gradual memory increase
  - Sudden memory spike
  - Sustained high memory usage
- **Validation**: Graceful degradation, no OOM crashes

### 4. Database Failover Tests
- **Purpose**: Test resilience during database failures
- **Scenarios**:
  - Primary database failure
  - Network partition
  - Connection pool exhaustion
- **Validation**: Circuit breaker activates, automatic recovery

### 5. PII Detection Performance
- **Purpose**: Measure PII sanitization overhead
- **Scenarios**:
  - Various regex complexity levels
  - Different data volumes
  - Multiple PII patterns
- **Target**: <5% performance overhead

### 6. End-to-End Latency Tests
- **Purpose**: Measure complete metric pipeline latency
- **Path**: Database → Collector → Processors → New Relic
- **Target**: <2 seconds end-to-end

## Running Tests

```bash
# Run all performance tests
make test-performance

# Run specific test scenario
make test-performance-baseline
make test-performance-stress
make test-performance-failover

# Generate performance report
make performance-report
```

## Test Infrastructure

- **Load Generator**: Simulates realistic database workloads
- **Metric Validator**: Verifies metrics in New Relic
- **Resource Monitor**: Tracks system resource usage
- **Report Generator**: Creates detailed performance reports

## Benchmarks

Current benchmarks (to be updated):
- Metric ingestion rate: TBD
- Processing latency: TBD  
- Memory usage: TBD
- CPU usage: TBD

## Future Enhancements

1. Automated performance regression detection
2. Continuous performance monitoring in CI/CD
3. ML-based anomaly detection for performance issues
4. Comparative benchmarks with other solutions