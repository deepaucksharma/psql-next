package circuitbreaker

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/go-redis/redis/v8"
    "go.opentelemetry.io/collector/pdata/plog"
    "go.opentelemetry.io/collector/pdata/pmetric"
    "go.uber.org/zap"
)

// State represents the circuit breaker state
type State int

const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

// CircuitBreaker protects databases from monitoring overhead
type CircuitBreaker struct {
    logger      *zap.Logger
    redisClient *redis.Client
    config      *Config
    
    mu       sync.RWMutex
    breakers map[string]*DatabaseBreaker
}

// DatabaseBreaker is a circuit breaker for a specific database
type DatabaseBreaker struct {
    DatabaseName     string
    State            State
    ErrorCount       int64
    SuccessCount     int64
    ConsecutiveFails int64
    LastFailTime     time.Time
    LastStateChange  time.Time
    HalfOpenRequests int64
}

// NewCircuitBreaker creates a new circuit breaker processor
func NewCircuitBreaker(logger *zap.Logger, redisClient *redis.Client, config *Config) *CircuitBreaker {
    return &CircuitBreaker{
        logger:      logger,
        redisClient: redisClient,
        config:      config,
        breakers:    make(map[string]*DatabaseBreaker),
    }
}

// ProcessMetrics applies circuit breaker logic to metrics
func (cb *CircuitBreaker) ProcessMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
    // Extract database from resource attributes
    rms := metrics.ResourceMetrics()
    for i := 0; i < rms.Len(); i++ {
        rm := rms.At(i)
        attrs := rm.Resource().Attributes()
        
        databaseName, ok := attrs.Get("database_name")
        if !ok {
            continue
        }
        
        dbName := databaseName.Str()
        
        // Check circuit breaker state
        if cb.shouldBlock(ctx, dbName) {
            cb.logger.Warn("Circuit breaker OPEN - dropping metrics",
                zap.String("database", dbName))
            
            // Remove this resource metric
            rms.RemoveIf(func(r pmetric.ResourceMetrics) bool {
                db, _ := r.Resource().Attributes().Get("database_name")
                return db.Str() == dbName
            })
            
            continue
        }
        
        // Process metrics and track success/failure
        if err := cb.processDatabase(ctx, dbName, rm); err != nil {
            cb.recordFailure(ctx, dbName)
        } else {
            cb.recordSuccess(ctx, dbName)
        }
    }
    
    return metrics, nil
}

// ProcessLogs applies circuit breaker logic to logs
func (cb *CircuitBreaker) ProcessLogs(ctx context.Context, logs plog.Logs) (plog.Logs, error) {
    // Similar implementation for logs
    rls := logs.ResourceLogs()
    for i := 0; i < rls.Len(); i++ {
        rl := rls.At(i)
        attrs := rl.Resource().Attributes()
        
        databaseName, ok := attrs.Get("database_name")
        if !ok {
            continue
        }
        
        dbName := databaseName.Str()
        
        if cb.shouldBlock(ctx, dbName) {
            cb.logger.Warn("Circuit breaker OPEN - dropping logs",
                zap.String("database", dbName))
            
            rls.RemoveIf(func(r plog.ResourceLogs) bool {
                db, _ := r.Resource().Attributes().Get("database_name")
                return db.Str() == dbName
            })
        }
    }
    
    return logs, nil
}

// shouldBlock determines if requests should be blocked
func (cb *CircuitBreaker) shouldBlock(ctx context.Context, dbName string) bool {
    cb.mu.RLock()
    breaker, exists := cb.breakers[dbName]
    cb.mu.RUnlock()
    
    if !exists {
        // Create new breaker
        cb.mu.Lock()
        breaker = &DatabaseBreaker{
            DatabaseName:    dbName,
            State:           StateClosed,
            LastStateChange: time.Now(),
        }
        cb.breakers[dbName] = breaker
        cb.mu.Unlock()
    }
    
    switch breaker.State {
    case StateClosed:
        return false
        
    case StateOpen:
        // Check if it's time to try half-open
        if time.Since(breaker.LastStateChange) > cb.config.OpenTimeout {
            cb.transitionToHalfOpen(dbName)
            return false
        }
        return true
        
    case StateHalfOpen:
        // Allow limited requests in half-open state
        cb.mu.Lock()
        defer cb.mu.Unlock()
        
        if breaker.HalfOpenRequests < cb.config.HalfOpenRequests {
            breaker.HalfOpenRequests++
            return false
        }
        return true
    }
    
    return false
}

// recordFailure records a failed request
func (cb *CircuitBreaker) recordFailure(ctx context.Context, dbName string) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    breaker := cb.breakers[dbName]
    breaker.ErrorCount++
    breaker.ConsecutiveFails++
    breaker.LastFailTime = time.Now()
    
    // Calculate error rate
    totalRequests := breaker.ErrorCount + breaker.SuccessCount
    if totalRequests < cb.config.RequestThreshold {
        return // Not enough data
    }
    
    errorRate := float64(breaker.ErrorCount) / float64(totalRequests)
    
    // Check if we should open the circuit
    if breaker.State == StateClosed && errorRate > cb.config.ErrorThreshold {
        cb.transitionToOpen(dbName)
    } else if breaker.State == StateHalfOpen {
        // Failed in half-open, go back to open
        cb.transitionToOpen(dbName)
    }
    
    // Sync with Redis
    cb.syncToRedis(ctx, dbName, breaker)
}

// recordSuccess records a successful request
func (cb *CircuitBreaker) recordSuccess(ctx context.Context, dbName string) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    breaker := cb.breakers[dbName]
    breaker.SuccessCount++
    breaker.ConsecutiveFails = 0
    
    // If in half-open state and successful, transition to closed
    if breaker.State == StateHalfOpen {
        if breaker.HalfOpenRequests >= cb.config.HalfOpenRequests {
            cb.transitionToClosed(dbName)
        }
    }
    
    // Sync with Redis
    cb.syncToRedis(ctx, dbName, breaker)
}

// State transitions
func (cb *CircuitBreaker) transitionToOpen(dbName string) {
    breaker := cb.breakers[dbName]
    breaker.State = StateOpen
    breaker.LastStateChange = time.Now()
    breaker.HalfOpenRequests = 0
    
    cb.logger.Warn("Circuit breaker transitioned to OPEN",
        zap.String("database", dbName),
        zap.Int64("error_count", breaker.ErrorCount),
        zap.Int64("consecutive_fails", breaker.ConsecutiveFails))
}

func (cb *CircuitBreaker) transitionToHalfOpen(dbName string) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    breaker := cb.breakers[dbName]
    breaker.State = StateHalfOpen
    breaker.LastStateChange = time.Now()
    breaker.HalfOpenRequests = 0
    
    cb.logger.Info("Circuit breaker transitioned to HALF-OPEN",
        zap.String("database", dbName))
}

func (cb *CircuitBreaker) transitionToClosed(dbName string) {
    breaker := cb.breakers[dbName]
    breaker.State = StateClosed
    breaker.LastStateChange = time.Now()
    breaker.ErrorCount = 0
    breaker.SuccessCount = 0
    breaker.HalfOpenRequests = 0
    
    cb.logger.Info("Circuit breaker transitioned to CLOSED",
        zap.String("database", dbName))
}

// syncToRedis synchronizes breaker state to Redis
func (cb *CircuitBreaker) syncToRedis(ctx context.Context, dbName string, breaker *DatabaseBreaker) {
    key := fmt.Sprintf("circuit_breaker:%s", dbName)
    data := map[string]interface{}{
        "state":             int(breaker.State),
        "error_count":       breaker.ErrorCount,
        "success_count":     breaker.SuccessCount,
        "consecutive_fails": breaker.ConsecutiveFails,
        "last_fail_time":    breaker.LastFailTime.Unix(),
        "last_state_change": breaker.LastStateChange.Unix(),
    }
    
    if err := cb.redisClient.HMSet(ctx, key, data).Err(); err != nil {
        cb.logger.Warn("Failed to sync circuit breaker state to Redis",
            zap.String("database", dbName),
            zap.Error(err))
    }
}

// LoadFromRedis loads circuit breaker states from Redis
func (cb *CircuitBreaker) LoadFromRedis(ctx context.Context) error {
    keys, err := cb.redisClient.Keys(ctx, "circuit_breaker:*").Result()
    if err != nil {
        return err
    }
    
    for _, key := range keys {
        dbName := key[len("circuit_breaker:"):]
        data, err := cb.redisClient.HGetAll(ctx, key).Result()
        if err != nil {
            continue
        }
        
        breaker := &DatabaseBreaker{
            DatabaseName: dbName,
        }
        
        // Parse state data
        if v, ok := data["state"]; ok {
            state, _ := strconv.Atoi(v)
            breaker.State = State(state)
        }
        if v, ok := data["error_count"]; ok {
            breaker.ErrorCount, _ = strconv.ParseInt(v, 10, 64)
        }
        if v, ok := data["success_count"]; ok {
            breaker.SuccessCount, _ = strconv.ParseInt(v, 10, 64)
        }
        
        cb.mu.Lock()
        cb.breakers[dbName] = breaker
        cb.mu.Unlock()
    }
    
    return nil
}

// processDatabase simulates processing metrics for a database
func (cb *CircuitBreaker) processDatabase(ctx context.Context, dbName string, rm pmetric.ResourceMetrics) error {
    // Placeholder for actual processing logic
    // In real implementation, this would check query response times,
    // connection counts, etc. to determine if the database is healthy
    return nil
}