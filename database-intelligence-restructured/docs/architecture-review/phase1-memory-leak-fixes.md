# Phase 1.2: Memory Leak Fixes

## Summary
Fixed memory leaks in components by implementing bounded data structures and proper cleanup mechanisms.

## Memory Leak Patterns Identified

### 1. Unbounded Maps
- **Problem**: Maps that grow indefinitely without cleanup
- **Components Affected**: 
  - querycorrelator processor
  - ASH receiver storage
  - enhancedsql receiver
  
### 2. Missing Size Limits
- **Problem**: No enforcement of configured limits
- **Example**: MaxQueriesTracked config not enforced

### 3. No Cleanup Mechanisms
- **Problem**: Old data never removed from memory
- **Impact**: Memory growth over time

## Fixes Implemented

### 1. Created Bounded Map Implementation
```go
// components/internal/boundedmap/boundedmap.go
type BoundedMap struct {
    data     map[string]interface{}
    lru      *list.List
    maxSize  int
    mu       sync.RWMutex
    onEvict  func(key string, value interface{})
}
```

Features:
- LRU eviction when size limit reached
- Thread-safe operations
- Time-based cleanup support
- Eviction callbacks for monitoring

### 2. Fixed Query Correlator Processor
```go
// Before: Unbounded maps
queryIndex: make(map[string]*queryInfo)

// After: Bounded maps with limits
queryIndex: boundedmap.New(processorConfig.MaxQueriesTracked, onEvict)
```

Changes:
- Replaced unbounded maps with BoundedMap
- Enforced MaxQueriesTracked configuration
- Added automatic cleanup of old entries
- Fixed retention period enforcement

### 3. Fixed ASH Storage
```go
// Added size limits to aggregated windows
type AggregatedWindow struct {
    // ... existing fields ...
    MaxQueries   int  // Maximum queries to track
    MaxWaits     int  // Maximum wait events to track
}
```

## Components Still Needing Fixes

### 1. enhancedsql receiver
- Add query result caching limits
- Implement connection pool size limits

### 2. nri exporter
- Add buffer size limits
- Implement backpressure handling

### 3. kernelmetrics receiver
- Add metric collection limits
- Implement sliding window for historical data

## Best Practices Going Forward

### 1. Always Use Bounded Collections
```go
// Bad
cache := make(map[string]interface{})

// Good
cache := boundedmap.New(maxSize, onEvict)
```

### 2. Implement Cleanup Goroutines
```go
func (c *Component) cleanupLoop() {
    ticker := time.NewTicker(cleanupInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            c.cleanup()
        case <-c.shutdownChan:
            return
        }
    }
}
```

### 3. Monitor Resource Usage
```go
func (c *Component) reportMetrics() {
    c.metrics.CacheSize.Set(float64(c.cache.Len()))
    c.metrics.MemoryUsage.Set(float64(runtime.MemStats.Alloc))
}
```

## Testing Memory Leaks

### 1. Load Test
```bash
# Run collector with high load
# Monitor memory usage over time
# Should stabilize after warmup
```

### 2. Profiling
```go
import _ "net/http/pprof"

// Access http://localhost:6060/debug/pprof/heap
```

## Success Metrics
- Memory usage stabilizes under constant load
- No OOM errors in 24-hour test runs
- Configured limits are enforced
- Old data is cleaned up automatically