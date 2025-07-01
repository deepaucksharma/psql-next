# Performance Testing Framework

This directory contains performance tests for the Database Intelligence Collector, focusing on throughput, latency, and resource utilization under various load conditions.

## Overview

The performance testing framework provides:
- Load generation with realistic database workloads
- Throughput and latency measurements
- Resource utilization tracking
- Performance regression detection
- Comparative benchmarking

## Test Categories

### 1. Processor Performance Tests
- Individual processor benchmarks
- Pipeline throughput testing
- Memory allocation profiling
- CPU utilization analysis

### 2. End-to-End Performance Tests
- Full pipeline load testing
- Database query impact analysis
- Export throughput validation
- Network bandwidth utilization

### 3. Stress Tests
- Maximum throughput identification
- Resource exhaustion scenarios
- Graceful degradation validation
- Recovery time measurement

## Running Performance Tests

### Basic Performance Test
```bash
cd tests/performance
go test -bench=. -benchmem -benchtime=30s
```

### CPU Profiling
```bash
go test -bench=. -cpuprofile=cpu.prof
go tool pprof cpu.prof
```

### Memory Profiling
```bash
go test -bench=. -memprofile=mem.prof
go tool pprof mem.prof
```

### Load Testing
```bash
go test -v -run TestLoadScenarios -timeout 30m
```

## Performance Baselines

### Target Metrics
- **Throughput**: 10,000+ metrics/second
- **Latency**: < 5ms processing time
- **Memory**: < 512MB under normal load
- **CPU**: < 20% on 2-core system

### Current Performance
Results from latest benchmarks:
- Adaptive Sampler: 2.3μs/operation
- Circuit Breaker: 1.8μs/operation
- Plan Extractor: 15.2μs/operation
- Verification: 8.7μs/operation

## Test Scenarios

### 1. Normal Load (light_load.yaml)
- 10 concurrent databases
- 100 queries/second per database
- 5% slow queries
- Expected: No resource pressure

### 2. Peak Load (peak_load.yaml)
- 50 concurrent databases
- 500 queries/second per database
- 20% slow queries
- Expected: 80% resource utilization

### 3. Stress Load (stress_load.yaml)
- 100 concurrent databases
- 1000 queries/second per database
- 40% slow queries
- Expected: Graceful degradation

## Performance Optimization Guide

### 1. Memory Optimization
- Use object pools for frequent allocations
- Minimize string allocations
- Efficient attribute maps
- Proper batch sizing

### 2. CPU Optimization
- Avoid regex in hot paths
- Cache computed values
- Parallel processing where applicable
- Efficient sampling algorithms

### 3. I/O Optimization
- Batch database queries
- Async export operations
- Connection pooling
- Compression for exports

## Regression Detection

Performance tests run automatically on:
- Pull requests (compared to main)
- Nightly builds (historical tracking)
- Release candidates (full suite)

Regressions > 10% trigger alerts.

## Profiling Tools

### pprof Integration
```go
import _ "net/http/pprof"

// In main.go
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

### Continuous Profiling
Access profiling endpoints:
- http://localhost:6060/debug/pprof/
- http://localhost:6060/debug/pprof/heap
- http://localhost:6060/debug/pprof/profile?seconds=30

## Best Practices

1. **Benchmark Isolation**
   - Run on dedicated hardware
   - Disable CPU frequency scaling
   - Multiple runs for consistency

2. **Realistic Workloads**
   - Use production query patterns
   - Include error scenarios
   - Vary cardinality levels

3. **Resource Monitoring**
   - Track all resource types
   - Monitor over time
   - Identify bottlenecks

4. **Result Analysis**
   - Statistical significance
   - Outlier detection
   - Trend analysis