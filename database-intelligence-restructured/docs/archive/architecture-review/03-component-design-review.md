# Component Design Review

## Critical Design Flaws

### 1. No Component Interfaces
```go
// Current: Direct implementation, no abstraction
type adaptiveSamplerProcessor struct {
    logger    *zap.Logger
    config    *Config
    algorithm *AdaptiveAlgorithm
}

// Cannot swap implementations or isolate for testing
```
**Impact**: Components tightly coupled, cannot modify independently

### 2. Inconsistent Component Structure
```go
// Processor A
type processorA struct {
    logger *zap.Logger
    cfg    *Config  // "cfg"
}

// Processor B  
type processorB struct {
    log    *zap.Logger  // "log" not "logger"
    config *Config      // "config" not "cfg"
}
```
**Impact**: No code reuse, maintenance confusion

### 3. No Lifecycle Management
```go
// Current: Each component does its own thing
func (r *receiver) Start(ctx context.Context, host component.Host) error {
    // No standard startup sequence
    // No resource management
    // No graceful shutdown
}
```
**Impact**: Components crash, hang, or leak resources

### 4. Memory Leaks
```go
type adaptiveSamplerProcessor struct {
    samples map[string][]sample  // Never cleaned up!
}

func (p *processor) Process(metrics pdata.Metrics) {
    // Keeps growing forever
    p.samples[key] = append(p.samples[key], newSample)
}
```
**Impact**: OOM kills, degraded performance

## Required Fixes

### Fix 1: Define Minimal Interfaces
```go
// Minimum viable interfaces for isolation
type Component interface {
    Start(context.Context) error
    Shutdown(context.Context) error
}

type MetricsProcessor interface {
    Component
    ProcessMetrics(context.Context, pdata.Metrics) (pdata.Metrics, error)
}
```

### Fix 2: Standard Component Base
```go
type BaseComponent struct {
    logger *zap.Logger
    state  atomic.Value // running/stopped
}

func (b *BaseComponent) Start(ctx context.Context) error {
    if !b.state.CompareAndSwap("stopped", "running") {
        return errors.New("already running")
    }
    return nil
}
```

### Fix 3: Resource Management
```go
type ManagedProcessor struct {
    BaseComponent
    maxMemory int64
    current   int64
}

func (p *ManagedProcessor) Process(metrics pdata.Metrics) error {
    size := estimateSize(metrics)
    if atomic.AddInt64(&p.current, size) > p.maxMemory {
        atomic.AddInt64(&p.current, -size)
        return errors.New("memory limit exceeded")
    }
    defer atomic.AddInt64(&p.current, -size)
    
    return p.processInternal(metrics)
}
```

### Fix 4: Fix Memory Leaks
```go
type FixedSamplerProcessor struct {
    samples map[string]*ring.Ring  // Bounded size
    maxSize int
}

func (p *FixedSamplerProcessor) addSample(key string, sample Sample) {
    if p.samples[key] == nil {
        p.samples[key] = ring.New(p.maxSize)
    }
    p.samples[key].Value = sample
    p.samples[key] = p.samples[key].Next()
}
```

## Migration Path

1. **Add Interfaces** - Define minimal interfaces first
2. **Wrap Existing** - Wrap current components with interfaces
3. **Fix Leaks** - Address memory issues component by component
4. **Standardize** - Enforce consistent patterns

## Success Metrics
- All components implement standard interfaces
- Zero memory leaks
- Consistent component structure
- Graceful shutdown for all components