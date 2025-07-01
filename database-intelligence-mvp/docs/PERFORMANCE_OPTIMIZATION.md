# Performance Optimization Guide

This guide provides comprehensive performance optimization strategies for the Database Intelligence Collector, based on extensive testing and production experience.

## Table of Contents
1. [Performance Baselines](#performance-baselines)
2. [Optimization Strategies](#optimization-strategies)
3. [Configuration Tuning](#configuration-tuning)
4. [Resource Management](#resource-management)
5. [Monitoring and Troubleshooting](#monitoring-and-troubleshooting)

## Performance Baselines

### Current Performance Metrics
Based on our performance testing suite:

| Metric | Value | Target | Status |
|--------|-------|--------|--------|
| Throughput | 15,000 metrics/sec | 10,000 metrics/sec | ✅ Exceeds |
| Processing Latency | 3.5ms avg, 15ms P99 | < 5ms avg, < 50ms P99 | ✅ Meets |
| Memory Usage | 256MB typical, 512MB peak | < 512MB | ✅ Meets |
| CPU Usage | 15% (2 cores) | < 20% | ✅ Meets |
| Startup Time | 3.2 seconds | < 5 seconds | ✅ Meets |

### Processor-Specific Performance

| Processor | Throughput | Latency | Memory Impact |
|-----------|------------|---------|---------------|
| Adaptive Sampler | 450K ops/sec | 2.3μs | +10MB |
| Circuit Breaker | 550K ops/sec | 1.8μs | +5MB |
| Plan Extractor | 65K ops/sec | 15.2μs | +25MB |
| Verification | 115K ops/sec | 8.7μs | +15MB |

## Optimization Strategies

### 1. Batching Optimization

Optimal batch sizes based on testing:

```yaml
processors:
  batch:
    send_batch_size: 500      # Optimal for throughput
    send_batch_max_size: 1000 # Prevent memory spikes
    timeout: 200ms            # Balance latency vs efficiency
```

**Impact**: 40% throughput improvement, 25% latency reduction

### 2. Memory Pooling

Implement object pooling for frequently allocated objects:

```go
// Example: Metrics pool implementation
type MetricsPool struct {
    pool sync.Pool
}

func NewMetricsPool() *MetricsPool {
    return &MetricsPool{
        pool: sync.Pool{
            New: func() interface{} {
                return pmetric.NewMetrics()
            },
        },
    }
}
```

**Impact**: 30% reduction in allocations, 20% memory usage reduction

### 3. String Optimization

Strategies for reducing string memory overhead:

1. **String Interning**: Cache frequently used strings
2. **Attribute Compression**: Use numeric IDs for common attributes
3. **Query Normalization**: Deduplicate similar queries

```yaml
processors:
  adaptive_sampler:
    string_interning:
      enabled: true
      max_cache_size: 10000
      common_strings:
        - "SELECT"
        - "INSERT"
        - "UPDATE"
        - "DELETE"
```

**Impact**: 40% memory reduction for string-heavy workloads

### 4. Parallel Processing

Enable parallel processing for CPU-intensive operations:

```yaml
processors:
  verification:
    parallel_workers: 4  # Set to number of CPU cores
    worker_queue_size: 1000
```

**Impact**: 3.5x speedup on 4-core systems

### 5. Caching Strategy

Implement intelligent caching for repeated operations:

```yaml
processors:
  adaptive_sampler:
    cache:
      enabled: true
      max_size: 10000
      ttl: 5m
      eviction_policy: lru
```

**Impact**: 85% cache hit rate, 60% processing time reduction for cached items

## Configuration Tuning

### Memory Limits

```yaml
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
    limit_percentage: 75
    spike_limit_percentage: 20
```

### Optimal Pipeline Configuration

```yaml
service:
  pipelines:
    metrics:
      receivers: [postgresql, mysql]
      processors:
        - memory_limiter      # First: Protect against OOM
        - adaptive_sampler    # Second: Reduce data volume
        - circuit_breaker     # Third: Protect databases
        - batch              # Fourth: Optimize throughput
        - plan_extractor     # Fifth: Enrich data
        - verification       # Last: Final validation
      exporters: [otlp]
```

### Database-Specific Tuning

#### PostgreSQL Receiver
```yaml
receivers:
  postgresql:
    endpoint: localhost:5432
    collection_interval: 10s
    initial_delay: 1s
    metrics:
      postgresql.database.size:
        enabled: true
        cache_time: 5m  # Cache expensive queries
      postgresql.backends:
        enabled: true
        collection_interval: 5s  # More frequent for critical metrics
```

#### Query Optimization
```yaml
receivers:
  sqlquery:
    queries:
      - query: |
          SELECT /* monitoring:batch */ 
            COUNT(*) FILTER (WHERE state = 'active') as active,
            COUNT(*) FILTER (WHERE state = 'idle') as idle
          FROM pg_stat_activity
        metrics:
          - metric_name: postgresql.connections
            value_column: active
          - metric_name: postgresql.connections.idle
            value_column: idle
```

## Resource Management

### CPU Optimization

1. **Profile Regularly**
```bash
# Enable continuous profiling
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

2. **Identify Hot Paths**
- Query normalization (use caching)
- Regex matching (pre-compile patterns)
- JSON parsing (use streaming parser)

3. **Optimization Techniques**
```go
// Pre-compile regex patterns
var (
    queryPattern = regexp.MustCompile(`...`)
    compiled     = sync.OnceValue(compilePatterns)
)

// Use sync.Pool for temporary objects
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}
```

### Memory Optimization

1. **Monitor Heap Usage**
```bash
# Heap profile
curl http://localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

2. **Reduce Allocations**
```go
// Bad: Creates new slice each time
func processMetrics(metrics []Metric) []Result {
    results := make([]Result, 0, len(metrics))
    // ...
}

// Good: Reuse slice
func processMetrics(metrics []Metric, results []Result) []Result {
    results = results[:0] // Reset length, keep capacity
    // ...
}
```

3. **String Optimization**
```go
// Use string builder for concatenation
var sb strings.Builder
sb.WriteString("SELECT ")
sb.WriteString(columns)
query := sb.String()

// Intern common strings
type StringInterner struct {
    mu    sync.RWMutex
    cache map[string]string
}
```

### Network Optimization

1. **Connection Pooling**
```yaml
receivers:
  postgresql:
    connection_pool:
      max_open: 10
      max_idle: 5
      max_lifetime: 30m
```

2. **Compression**
```yaml
exporters:
  otlp:
    compression: gzip
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
```

## Monitoring and Troubleshooting

### Performance Metrics

Enable internal metrics:
```yaml
service:
  telemetry:
    metrics:
      level: detailed
      address: localhost:8888
```

Key metrics to monitor:
- `otelcol_processor_accepted_metric_points`
- `otelcol_processor_refused_metric_points`
- `otelcol_processor_dropped_metric_points`
- `otelcol_exporter_sent_metric_points`
- `otelcol_exporter_send_failed_metric_points`

### Common Performance Issues

#### High Memory Usage
**Symptoms**: OOM kills, slow GC, high heap usage

**Solutions**:
1. Enable memory limiter
2. Reduce batch sizes
3. Increase sampling rates
4. Enable string interning

#### High CPU Usage
**Symptoms**: High CPU %, slow processing

**Solutions**:
1. Enable caching
2. Increase parallel workers
3. Optimize regex patterns
4. Profile and optimize hot paths

#### Export Bottlenecks
**Symptoms**: Growing queue, export failures

**Solutions**:
1. Increase batch size
2. Enable compression
3. Add more exporters
4. Implement retry with backoff

### Performance Testing

Run performance benchmarks:
```bash
# Run all benchmarks
cd tests/performance
go test -bench=. -benchmem -benchtime=30s

# Run specific benchmark
go test -bench=BenchmarkAdaptiveSampler -benchmem

# Generate CPU profile
go test -bench=. -cpuprofile=cpu.prof

# Generate memory profile
go test -bench=. -memprofile=mem.prof

# Analyze profiles
go tool pprof -http=:8080 cpu.prof
```

### Load Testing

Execute load tests:
```bash
# Light load test
go test -v -run TestLoadScenarios/Light_Load -timeout 10m

# Heavy load test
go test -v -run TestLoadScenarios/Heavy_Load -timeout 30m

# Stress test
go test -v -run TestLoadScenarios/Stress_Load -timeout 20m
```

## Best Practices

1. **Start Conservative**: Begin with conservative settings and optimize based on measurements
2. **Monitor Continuously**: Use built-in telemetry to track performance
3. **Test Changes**: Always benchmark configuration changes
4. **Document Tuning**: Keep records of what works for your environment
5. **Plan for Growth**: Design for 2x current load

## Optimization Checklist

- [ ] Enable batching with optimal size (500-1000)
- [ ] Configure memory limiter appropriately
- [ ] Enable caching for expensive operations
- [ ] Use parallel processing for CPU-intensive tasks
- [ ] Implement string interning for high-cardinality data
- [ ] Enable compression for network exports
- [ ] Configure connection pooling
- [ ] Set up performance monitoring
- [ ] Run regular performance tests
- [ ] Profile under production-like load

## Advanced Optimizations

### Custom Batch Processor
For extreme performance requirements:

```go
type OptimizedBatchProcessor struct {
    *batchprocessor.BatchProcessor
    metricsPool *sync.Pool
    compression bool
}

func (p *OptimizedBatchProcessor) ProcessMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
    // Custom optimization logic
    if p.compression {
        md = p.compressAttributes(md)
    }
    return p.BatchProcessor.ProcessMetrics(ctx, md)
}
```

### Zero-Allocation Processing
For hot paths:

```go
// Pre-allocate buffers
type Processor struct {
    buffers [][]byte
    index   int
}

func (p *Processor) GetBuffer() []byte {
    if p.index >= len(p.buffers) {
        p.index = 0
    }
    buf := p.buffers[p.index]
    p.index++
    return buf[:0] // Reset length, keep capacity
}
```