# Implementation Gaps: Required Code Changes

## Overview
This document outlines the specific code changes needed to bridge the gaps between the implementation review recommendations and the current state.

## 1. Redis State Store Implementations

### 1.1 Circuit Breaker Redis State Store

**File to create**: `processors/circuitbreaker/redis_state_store.go`

```go
package circuitbreaker

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/go-redis/redis/v8"
)

type RedisStateStore struct {
    client *redis.Client
    prefix string
    ttl    time.Duration
}

func NewRedisStateStore(client *redis.Client, prefix string) StateStore {
    return &RedisStateStore{
        client: client,
        prefix: prefix,
        ttl:    24 * time.Hour,
    }
}

func (r *RedisStateStore) SaveCircuitState(ctx context.Context, circuitID string, state *Circuit) error {
    key := fmt.Sprintf("%s:circuit:%s", r.prefix, circuitID)
    data, err := json.Marshal(state)
    if err != nil {
        return err
    }
    return r.client.Set(ctx, key, data, r.ttl).Err()
}

func (r *RedisStateStore) LoadCircuitState(ctx context.Context, circuitID string) (*Circuit, error) {
    key := fmt.Sprintf("%s:circuit:%s", r.prefix, circuitID)
    data, err := r.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    
    var circuit Circuit
    if err := json.Unmarshal(data, &circuit); err != nil {
        return nil, err
    }
    return &circuit, nil
}

func (r *RedisStateStore) ListCircuits(ctx context.Context) ([]string, error) {
    pattern := fmt.Sprintf("%s:circuit:*", r.prefix)
    keys, err := r.client.Keys(ctx, pattern).Result()
    if err != nil {
        return nil, err
    }
    
    circuits := make([]string, 0, len(keys))
    prefix := fmt.Sprintf("%s:circuit:", r.prefix)
    for _, key := range keys {
        circuitID := key[len(prefix):]
        circuits = append(circuits, circuitID)
    }
    return circuits, nil
}
```

### 1.2 Adaptive Sampler Redis Deduplication Store

**File to create**: `processors/adaptivesampler/redis_dedupe_store.go`

```go
package adaptivesampler

import (
    "context"
    "fmt"
    "time"
    
    "github.com/go-redis/redis/v8"
)

type RedisDeduplicationStore struct {
    client *redis.Client
    prefix string
}

func NewRedisDeduplicationStore(client *redis.Client, prefix string) DeduplicationStore {
    return &RedisDeduplicationStore{
        client: client,
        prefix: prefix,
    }
}

func (r *RedisDeduplicationStore) CheckAndSet(ctx context.Context, hash string, ttl time.Duration) (bool, error) {
    key := fmt.Sprintf("%s:dedupe:%s", r.prefix, hash)
    
    // SetNX returns true if the key was set (didn't exist)
    result, err := r.client.SetNX(ctx, key, time.Now().Unix(), ttl).Result()
    if err != nil {
        return false, err
    }
    
    return result, nil
}

func (r *RedisDeduplicationStore) GetStats(ctx context.Context) (*DedupeStats, error) {
    pattern := fmt.Sprintf("%s:dedupe:*", r.prefix)
    
    var totalChecks, uniqueItems int64
    var cursor uint64
    
    for {
        keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 1000).Result()
        if err != nil {
            return nil, err
        }
        
        uniqueItems += int64(len(keys))
        cursor = nextCursor
        
        if cursor == 0 {
            break
        }
    }
    
    // Get total checks from a counter
    totalChecks, _ = r.client.Get(ctx, fmt.Sprintf("%s:stats:total_checks", r.prefix)).Int64()
    
    return &DedupeStats{
        TotalChecks:    totalChecks,
        UniqueItems:    uniqueItems,
        DuplicateItems: totalChecks - uniqueItems,
    }, nil
}
```

## 2. Sampling Strategy Implementations

### 2.1 Base Strategy Interface Implementation

**File to create**: `processors/adaptivesampler/strategies.go`

```go
package adaptivesampler

import (
    "context"
    "math/rand"
    "sync"
    "time"
    
    "go.opentelemetry.io/collector/pdata/pcommon"
)

// ProbabilisticStrategy implements fixed-rate probabilistic sampling
type ProbabilisticStrategy struct {
    rate float64
    mu   sync.RWMutex
}

func NewProbabilisticStrategy(rate float64) SamplingStrategy {
    return &ProbabilisticStrategy{rate: rate}
}

func (p *ProbabilisticStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    return rand.Float64() < p.rate, p.rate
}

func (p *ProbabilisticStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
    // Fixed rate strategy doesn't update
    return nil
}

func (p *ProbabilisticStrategy) GetCurrentRate() float64 {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return p.rate
}

// AdaptiveRateStrategy adjusts sampling based on volume
type AdaptiveRateStrategy struct {
    config      StrategyConfig
    currentRate float64
    targetQPS   float64
    mu          sync.RWMutex
}

func NewAdaptiveRateStrategy(config StrategyConfig) SamplingStrategy {
    return &AdaptiveRateStrategy{
        config:      config,
        currentRate: config.InitialRate,
        targetQPS:   config.TargetSamplesPerSecond,
    }
}

func (a *AdaptiveRateStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
    a.mu.RLock()
    defer a.mu.RUnlock()
    
    return rand.Float64() < a.currentRate, a.currentRate
}

func (a *AdaptiveRateStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    // Adjust rate based on volume
    if feedback.VolumePerSecond > 0 {
        desiredRate := a.targetQPS / feedback.VolumePerSecond
        
        // Apply smoothing
        a.currentRate = a.currentRate*0.7 + desiredRate*0.3
        
        // Apply bounds
        if a.currentRate > a.config.MaxRate {
            a.currentRate = a.config.MaxRate
        }
        if a.currentRate < a.config.MinRate {
            a.currentRate = a.config.MinRate
        }
    }
    
    return nil
}

func (a *AdaptiveRateStrategy) GetCurrentRate() float64 {
    a.mu.RLock()
    defer a.mu.RUnlock()
    return a.currentRate
}

// AdaptiveCostStrategy samples based on query cost/duration
type AdaptiveCostStrategy struct {
    config      StrategyConfig
    costRates   map[string]float64 // cost bucket -> rate
    mu          sync.RWMutex
}

func NewAdaptiveCostStrategy(config StrategyConfig) SamplingStrategy {
    return &AdaptiveCostStrategy{
        config: config,
        costRates: map[string]float64{
            "low":    0.1,
            "medium": 0.5,
            "high":   1.0,
        },
    }
}

func (a *AdaptiveCostStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
    a.mu.RLock()
    defer a.mu.RUnlock()
    
    // Extract cost from attributes
    costBucket := "medium"
    if cost, ok := attributes.Get("query.cost"); ok {
        if cost.Double() < 100 {
            costBucket = "low"
        } else if cost.Double() > 1000 {
            costBucket = "high"
        }
    }
    
    rate := a.costRates[costBucket]
    return rand.Float64() < rate, rate
}

func (a *AdaptiveCostStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
    // Could adjust cost thresholds based on feedback
    return nil
}

func (a *AdaptiveCostStrategy) GetCurrentRate() float64 {
    a.mu.RLock()
    defer a.mu.RUnlock()
    
    // Return average rate
    total := 0.0
    for _, rate := range a.costRates {
        total += rate
    }
    return total / float64(len(a.costRates))
}

// AdaptiveErrorStrategy increases sampling for errors
type AdaptiveErrorStrategy struct {
    config         StrategyConfig
    baseRate       float64
    errorRate      float64
    recentErrors   int64
    recentTotal    int64
    mu             sync.RWMutex
}

func NewAdaptiveErrorStrategy(config StrategyConfig) SamplingStrategy {
    return &AdaptiveErrorStrategy{
        config:    config,
        baseRate:  config.InitialRate,
        errorRate: 1.0, // Always sample errors
    }
}

func (a *AdaptiveErrorStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
    a.mu.RLock()
    defer a.mu.RUnlock()
    
    // Check if this is an error
    if severity, ok := attributes.Get("severity"); ok {
        if severity.Str() == "ERROR" || severity.Str() == "FATAL" {
            return true, a.errorRate
        }
    }
    
    // Use base rate for non-errors
    return rand.Float64() < a.baseRate, a.baseRate
}

func (a *AdaptiveErrorStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    // Adjust base rate based on error rate
    if feedback.ErrorRate > 0.1 {
        // High error rate - increase sampling
        a.baseRate = min(a.baseRate*1.1, a.config.MaxRate)
    } else if feedback.ErrorRate < 0.01 {
        // Low error rate - can decrease sampling
        a.baseRate = max(a.baseRate*0.9, a.config.MinRate)
    }
    
    return nil
}

func (a *AdaptiveErrorStrategy) GetCurrentRate() float64 {
    a.mu.RLock()
    defer a.mu.RUnlock()
    return a.baseRate
}

func min(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}

func max(a, b float64) float64 {
    if a > b {
        return a
    }
    return b
}
```

## 3. Factory Updates

### 3.1 Circuit Breaker Factory Update

**File to modify**: `processors/circuitbreaker/factory.go`

Add support for choosing between original and refactored implementation:

```go
func createLogsProcessor(
    ctx context.Context,
    set processor.CreateSettings,
    cfg component.Config,
    nextConsumer consumer.Logs,
) (processor.Logs, error) {
    pCfg := cfg.(*Config)
    
    // Check if refactored version should be used
    if pCfg.UseRefactored {
        var stateStore StateStore
        
        // Initialize state store based on configuration
        if pCfg.StateStore != nil && pCfg.StateStore.Type == "redis" {
            redisClient := redis.NewClient(&redis.Options{
                Addr:     pCfg.StateStore.Redis.Endpoint,
                Password: pCfg.StateStore.Redis.Password,
                DB:       pCfg.StateStore.Redis.Database,
            })
            
            stateStore = NewRedisStateStore(redisClient, "circuitbreaker")
        }
        
        return newCircuitBreakerRefactoredProcessor(
            pCfg,
            set.Logger,
            nil, // metrics consumer
            nextConsumer,
            nil, // traces consumer
            stateStore,
        )
    }
    
    // Fall back to original implementation
    return newCircuitBreakerProcessor(pCfg, set.Logger, nextConsumer), nil
}
```

### 3.2 Adaptive Sampler Factory Update

**File to modify**: `processors/adaptivesampler/factory.go`

Similar update to support refactored version:

```go
func createLogsProcessor(
    ctx context.Context,
    set processor.CreateSettings,
    cfg component.Config,
    nextConsumer consumer.Logs,
) (processor.Logs, error) {
    pCfg := cfg.(*Config)
    
    // Check if refactored version should be used
    if pCfg.UseRefactored {
        var dedupeStore DeduplicationStore
        
        // Initialize deduplication store
        if pCfg.DeduplicationStore != nil && pCfg.DeduplicationStore.Type == "redis" {
            redisClient := redis.NewClient(&redis.Options{
                Addr:     pCfg.DeduplicationStore.Redis.Endpoint,
                Password: pCfg.DeduplicationStore.Redis.Password,
                DB:       pCfg.DeduplicationStore.Redis.Database,
            })
            
            dedupeStore = NewRedisDeduplicationStore(redisClient, "adaptivesampler")
        }
        
        return newAdaptiveSamplerRefactoredProcessor(
            pCfg,
            set.Logger,
            nil, // metrics consumer
            nextConsumer,
            nil, // traces consumer
            dedupeStore,
        )
    }
    
    // Fall back to original implementation
    return newAdaptiveSampler(pCfg, set.Logger, nextConsumer)
}
```

## 4. Configuration Schema Updates

### 4.1 Circuit Breaker Config Update

**File to modify**: `processors/circuitbreaker/config.go`

```go
type Config struct {
    // Existing fields...
    
    // New fields for refactored version
    UseRefactored bool              `mapstructure:"use_refactored"`
    StateStore    *StateStoreConfig `mapstructure:"state_store"`
}

type StateStoreConfig struct {
    Type  string       `mapstructure:"type"` // "file" or "redis"
    Redis *RedisConfig `mapstructure:"redis"`
}

type RedisConfig struct {
    Endpoint string `mapstructure:"endpoint"`
    Password string `mapstructure:"password"`
    Database int    `mapstructure:"database"`
}
```

### 4.2 Adaptive Sampler Config Update

**File to modify**: `processors/adaptivesampler/config.go`

```go
type Config struct {
    // Existing fields...
    
    // New fields for refactored version
    UseRefactored       bool                    `mapstructure:"use_refactored"`
    DeduplicationStore  *DeduplicationStoreConfig `mapstructure:"deduplication_store"`
    Strategies          map[string]StrategyConfig `mapstructure:"strategies"`
}

type DeduplicationStoreConfig struct {
    Type  string       `mapstructure:"type"` // "memory" or "redis"
    Redis *RedisConfig `mapstructure:"redis"`
}

type StrategyConfig struct {
    Type                   string  `mapstructure:"type"`
    InitialRate            float64 `mapstructure:"initial_rate"`
    MinRate                float64 `mapstructure:"min_rate"`
    MaxRate                float64 `mapstructure:"max_rate"`
    TargetSamplesPerSecond float64 `mapstructure:"target_samples_per_second"`
}
```

## 5. Migration Path

### 5.1 Feature Flags

Create a feature flag system to gradually migrate:

**File to create**: `internal/featureflags/flags.go`

```go
package featureflags

import (
    "os"
    "strconv"
)

type Flags struct {
    UseRefactoredReceiver       bool
    UseRefactoredCircuitBreaker bool
    UseRefactoredSampler        bool
    UseRedisStateStore          bool
}

func Load() *Flags {
    return &Flags{
        UseRefactoredReceiver:       getBool("USE_REFACTORED_RECEIVER", false),
        UseRefactoredCircuitBreaker: getBool("USE_REFACTORED_CIRCUIT_BREAKER", false),
        UseRefactoredSampler:        getBool("USE_REFACTORED_SAMPLER", false),
        UseRedisStateStore:          getBool("USE_REDIS_STATE_STORE", false),
    }
}

func getBool(key string, defaultValue bool) bool {
    if val := os.Getenv(key); val != "" {
        if b, err := strconv.ParseBool(val); err == nil {
            return b
        }
    }
    return defaultValue
}
```

## 6. Testing Requirements

### 6.1 Integration Tests for Redis State Stores

**File to create**: `tests/integration/redis_state_test.go`

```go
package integration

import (
    "context"
    "testing"
    "time"
    
    "github.com/go-redis/redis/v8"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/newrelic/database-intelligence-mvp/processors/circuitbreaker"
    "github.com/newrelic/database-intelligence-mvp/processors/adaptivesampler"
)

func TestRedisCircuitBreakerStateStore(t *testing.T) {
    // Skip if Redis not available
    client := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    defer client.Close()
    
    if err := client.Ping(context.Background()).Err(); err != nil {
        t.Skip("Redis not available")
    }
    
    store := circuitbreaker.NewRedisStateStore(client, "test")
    ctx := context.Background()
    
    // Test save and load
    circuit := &circuitbreaker.Circuit{
        ID:               "test-circuit",
        State:            circuitbreaker.StateOpen,
        FailureCount:     5,
        FailureThreshold: 3,
    }
    
    err := store.SaveCircuitState(ctx, circuit.ID, circuit)
    require.NoError(t, err)
    
    loaded, err := store.LoadCircuitState(ctx, circuit.ID)
    require.NoError(t, err)
    assert.Equal(t, circuit.ID, loaded.ID)
    assert.Equal(t, circuit.State, loaded.State)
}

func TestRedisDeduplicationStore(t *testing.T) {
    // Similar test for deduplication store
}
```

## 7. Documentation Updates

### 7.1 Migration Guide

**File to create**: `docs/MIGRATION_TO_REFACTORED.md`

```markdown
# Migration Guide: Moving to Refactored Components

## Overview
This guide helps you migrate from the original components to the refactored versions.

## Phase 1: Enable Feature Flags
Set environment variables to enable refactored components:
```bash
export USE_REFACTORED_RECEIVER=true
export USE_REFACTORED_CIRCUIT_BREAKER=true
export USE_REFACTORED_SAMPLER=true
```

## Phase 2: Configure Redis (Optional)
If using Redis for state storage:
```yaml
processors:
  circuitbreaker:
    use_refactored: true
    state_store:
      type: redis
      redis:
        endpoint: localhost:6379
        password: ""
        database: 0
```

## Phase 3: Validate Functionality
1. Monitor metrics to ensure correct behavior
2. Check logs for any errors
3. Verify state persistence works correctly

## Phase 4: Remove Original Components
Once validated, remove original implementations from codebase.
```

## Summary

The key code changes needed are:

1. **Implement Redis state stores** for both circuit breaker and adaptive sampler
2. **Create sampling strategy implementations** for the adaptive sampler
3. **Update factory files** to support both implementations during migration
4. **Add configuration support** for choosing implementations
5. **Create integration tests** for new components
6. **Document the migration path** clearly

This phased approach allows for gradual migration while maintaining stability.