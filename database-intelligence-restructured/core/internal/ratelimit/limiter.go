package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimiterConfig contains configuration for rate limiting
type RateLimiterConfig struct {
	// Default rate limit settings
	DefaultRPS   float64 `mapstructure:"default_rps"`
	DefaultBurst int     `mapstructure:"default_burst"`
	
	// Per-database overrides
	DatabaseLimits map[string]DatabaseLimit `mapstructure:"database_limits"`
	
	// Global safety limits
	GlobalMaxRPS   float64 `mapstructure:"global_max_rps"`
	GlobalMaxBurst int     `mapstructure:"global_max_burst"`
	
	// Adaptive rate limiting
	EnableAdaptive bool    `mapstructure:"enable_adaptive"`
	MinRPS         float64 `mapstructure:"min_rps"`
	MaxRPS         float64 `mapstructure:"max_rps"`
	
	// Monitoring
	EnableMetrics bool `mapstructure:"enable_metrics"`
}

// DatabaseLimit contains rate limit settings for a specific database
type DatabaseLimit struct {
	RPS   float64 `mapstructure:"rps"`
	Burst int     `mapstructure:"burst"`
	
	// Time-based limits
	Schedule []ScheduledLimit `mapstructure:"schedule"`
}

// ScheduledLimit applies different limits based on time
type ScheduledLimit struct {
	StartHour int     `mapstructure:"start_hour"`
	EndHour   int     `mapstructure:"end_hour"`
	RPS       float64 `mapstructure:"rps"`
	Burst     int     `mapstructure:"burst"`
}

// DatabaseRateLimiter manages rate limiting per database
type DatabaseRateLimiter struct {
	config    RateLimiterConfig
	logger    *zap.Logger
	limiters  map[string]*adaptiveLimiter
	global    *rate.Limiter
	mu        sync.RWMutex
	metrics   *RateLimiterMetrics
}

// adaptiveLimiter wraps a rate limiter with adaptive capabilities
type adaptiveLimiter struct {
	limiter       *rate.Limiter
	config        DatabaseLimit
	successCount  int64
	rejectCount   int64
	lastAdjusted  time.Time
	currentRPS    float64
	mu            sync.RWMutex
}

// RateLimiterMetrics tracks rate limiter statistics
type RateLimiterMetrics struct {
	RequestsAllowed   map[string]int64
	RequestsRejected  map[string]int64
	CurrentRPS        map[string]float64
	AdjustmentsMade   int64
	mu                sync.RWMutex
}

// NewDatabaseRateLimiter creates a new rate limiter
func NewDatabaseRateLimiter(config RateLimiterConfig, logger *zap.Logger) *DatabaseRateLimiter {
	drl := &DatabaseRateLimiter{
		config:   config,
		logger:   logger,
		limiters: make(map[string]*adaptiveLimiter),
		metrics: &RateLimiterMetrics{
			RequestsAllowed:  make(map[string]int64),
			RequestsRejected: make(map[string]int64),
			CurrentRPS:       make(map[string]float64),
		},
	}
	
	// Create global limiter
	if config.GlobalMaxRPS > 0 {
		drl.global = rate.NewLimiter(
			rate.Limit(config.GlobalMaxRPS),
			config.GlobalMaxBurst,
		)
	}
	
	// Pre-create limiters for configured databases
	for db, limit := range config.DatabaseLimits {
		drl.createLimiter(db, limit)
	}
	
	return drl
}

// Allow checks if a request should be allowed for a database
func (drl *DatabaseRateLimiter) Allow(ctx context.Context, database string) bool {
	// Check global limit first
	if drl.global != nil && !drl.global.Allow() {
		drl.recordRejection("_global")
		return false
	}
	
	// Get or create database-specific limiter
	limiter := drl.getLimiter(database)
	
	// Check if allowed
	allowed := limiter.Allow()
	
	// Record metrics
	if allowed {
		drl.recordAllowed(database)
	} else {
		drl.recordRejection(database)
	}
	
	// Adaptive adjustment if enabled
	if drl.config.EnableAdaptive {
		drl.maybeAdjustRate(database, limiter)
	}
	
	return allowed
}

// AllowN checks if N requests should be allowed
func (drl *DatabaseRateLimiter) AllowN(ctx context.Context, database string, n int) bool {
	// Check global limit first
	if drl.global != nil && !drl.global.AllowN(time.Now(), n) {
		drl.recordRejection("_global")
		return false
	}
	
	// Get or create database-specific limiter
	limiter := drl.getLimiter(database)
	
	// Check if allowed
	allowed := limiter.AllowN(n)
	
	// Record metrics
	if allowed {
		drl.recordAllowedN(database, n)
	} else {
		drl.recordRejectionN(database, n)
	}
	
	return allowed
}

// Wait blocks until a request can proceed
func (drl *DatabaseRateLimiter) Wait(ctx context.Context, database string) error {
	// Check global limit first
	if drl.global != nil {
		if err := drl.global.Wait(ctx); err != nil {
			return fmt.Errorf("global rate limit: %w", err)
		}
	}
	
	// Get or create database-specific limiter
	limiter := drl.getLimiter(database)
	
	// Wait for permission
	err := limiter.Wait(ctx)
	if err == nil {
		drl.recordAllowed(database)
	}
	
	return err
}

// UpdateLimit updates the rate limit for a database
func (drl *DatabaseRateLimiter) UpdateLimit(database string, rps float64, burst int) {
	drl.mu.Lock()
	defer drl.mu.Unlock()
	
	if limiter, exists := drl.limiters[database]; exists {
		limiter.UpdateLimit(rps, burst)
		drl.logger.Info("Updated rate limit",
			zap.String("database", database),
			zap.Float64("rps", rps),
			zap.Int("burst", burst),
		)
	}
}

// GetMetrics returns current rate limiter metrics
func (drl *DatabaseRateLimiter) GetMetrics() map[string]interface{} {
	drl.metrics.mu.RLock()
	defer drl.metrics.mu.RUnlock()
	
	metrics := make(map[string]interface{})
	
	// Copy allowed/rejected counts
	allowed := make(map[string]int64)
	rejected := make(map[string]int64)
	rps := make(map[string]float64)
	
	for k, v := range drl.metrics.RequestsAllowed {
		allowed[k] = v
	}
	for k, v := range drl.metrics.RequestsRejected {
		rejected[k] = v
	}
	for k, v := range drl.metrics.CurrentRPS {
		rps[k] = v
	}
	
	metrics["requests_allowed"] = allowed
	metrics["requests_rejected"] = rejected
	metrics["current_rps"] = rps
	metrics["adjustments_made"] = drl.metrics.AdjustmentsMade
	
	return metrics
}

// Private helper methods

func (drl *DatabaseRateLimiter) getLimiter(database string) *adaptiveLimiter {
	drl.mu.RLock()
	limiter, exists := drl.limiters[database]
	drl.mu.RUnlock()
	
	if exists {
		return limiter
	}
	
	// Create new limiter with default config
	drl.mu.Lock()
	defer drl.mu.Unlock()
	
	// Double-check after acquiring write lock
	if limiter, exists = drl.limiters[database]; exists {
		return limiter
	}
	
	// Use database-specific config if available
	if dbLimit, ok := drl.config.DatabaseLimits[database]; ok {
		return drl.createLimiter(database, dbLimit)
	}
	
	// Use default config
	return drl.createDefaultLimiter(database)
}

func (drl *DatabaseRateLimiter) createLimiter(database string, config DatabaseLimit) *adaptiveLimiter {
	rps := config.RPS
	burst := config.Burst
	
	// Apply scheduled limits if configured
	if len(config.Schedule) > 0 {
		now := time.Now()
		hour := now.Hour()
		
		for _, schedule := range config.Schedule {
			if hour >= schedule.StartHour && hour < schedule.EndHour {
				rps = schedule.RPS
				burst = schedule.Burst
				break
			}
		}
	}
	
	limiter := &adaptiveLimiter{
		limiter:      rate.NewLimiter(rate.Limit(rps), burst),
		config:       config,
		currentRPS:   rps,
		lastAdjusted: time.Now(),
	}
	
	drl.limiters[database] = limiter
	drl.logger.Info("Created rate limiter",
		zap.String("database", database),
		zap.Float64("rps", rps),
		zap.Int("burst", burst),
	)
	
	return limiter
}

func (drl *DatabaseRateLimiter) createDefaultLimiter(database string) *adaptiveLimiter {
	limiter := &adaptiveLimiter{
		limiter: rate.NewLimiter(
			rate.Limit(drl.config.DefaultRPS),
			drl.config.DefaultBurst,
		),
		currentRPS:   drl.config.DefaultRPS,
		lastAdjusted: time.Now(),
	}
	
	drl.limiters[database] = limiter
	drl.logger.Info("Created default rate limiter",
		zap.String("database", database),
		zap.Float64("rps", drl.config.DefaultRPS),
		zap.Int("burst", drl.config.DefaultBurst),
	)
	
	return limiter
}

func (drl *DatabaseRateLimiter) recordAllowed(database string) {
	drl.metrics.mu.Lock()
	defer drl.metrics.mu.Unlock()
	drl.metrics.RequestsAllowed[database]++
}

func (drl *DatabaseRateLimiter) recordAllowedN(database string, n int) {
	drl.metrics.mu.Lock()
	defer drl.metrics.mu.Unlock()
	drl.metrics.RequestsAllowed[database] += int64(n)
}

func (drl *DatabaseRateLimiter) recordRejection(database string) {
	drl.metrics.mu.Lock()
	defer drl.metrics.mu.Unlock()
	drl.metrics.RequestsRejected[database]++
	
	drl.logger.Debug("Rate limit exceeded",
		zap.String("database", database),
		zap.Int64("total_rejected", drl.metrics.RequestsRejected[database]),
	)
}

func (drl *DatabaseRateLimiter) recordRejectionN(database string, n int) {
	drl.metrics.mu.Lock()
	defer drl.metrics.mu.Unlock()
	drl.metrics.RequestsRejected[database] += int64(n)
}

func (drl *DatabaseRateLimiter) maybeAdjustRate(database string, limiter *adaptiveLimiter) {
	// Only adjust every 30 seconds
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	
	if time.Since(limiter.lastAdjusted) < 30*time.Second {
		return
	}
	
	// Calculate rejection rate
	total := limiter.successCount + limiter.rejectCount
	if total < 100 { // Need sufficient samples
		return
	}
	
	rejectionRate := float64(limiter.rejectCount) / float64(total)
	
	// Adjust based on rejection rate
	var newRPS float64
	if rejectionRate > 0.1 { // More than 10% rejected
		// Decrease rate by 10%
		newRPS = limiter.currentRPS * 0.9
	} else if rejectionRate < 0.01 { // Less than 1% rejected
		// Increase rate by 5%
		newRPS = limiter.currentRPS * 1.05
	} else {
		return // No adjustment needed
	}
	
	// Apply limits
	if newRPS < drl.config.MinRPS {
		newRPS = drl.config.MinRPS
	}
	if newRPS > drl.config.MaxRPS {
		newRPS = drl.config.MaxRPS
	}
	
	// Update limiter
	limiter.limiter.SetLimit(rate.Limit(newRPS))
	limiter.currentRPS = newRPS
	limiter.lastAdjusted = time.Now()
	
	// Reset counters
	limiter.successCount = 0
	limiter.rejectCount = 0
	
	// Update metrics
	drl.metrics.mu.Lock()
	drl.metrics.AdjustmentsMade++
	drl.metrics.CurrentRPS[database] = newRPS
	drl.metrics.mu.Unlock()
	
	drl.logger.Info("Adjusted rate limit",
		zap.String("database", database),
		zap.Float64("old_rps", limiter.currentRPS),
		zap.Float64("new_rps", newRPS),
		zap.Float64("rejection_rate", rejectionRate),
	)
}

// adaptiveLimiter methods

func (al *adaptiveLimiter) Allow() bool {
	allowed := al.limiter.Allow()
	
	al.mu.Lock()
	if allowed {
		al.successCount++
	} else {
		al.rejectCount++
	}
	al.mu.Unlock()
	
	return allowed
}

func (al *adaptiveLimiter) AllowN(n int) bool {
	allowed := al.limiter.AllowN(time.Now(), n)
	
	al.mu.Lock()
	if allowed {
		al.successCount += int64(n)
	} else {
		al.rejectCount += int64(n)
	}
	al.mu.Unlock()
	
	return allowed
}

func (al *adaptiveLimiter) Wait(ctx context.Context) error {
	err := al.limiter.Wait(ctx)
	
	if err == nil {
		al.mu.Lock()
		al.successCount++
		al.mu.Unlock()
	}
	
	return err
}

func (al *adaptiveLimiter) UpdateLimit(rps float64, burst int) {
	al.mu.Lock()
	defer al.mu.Unlock()
	
	al.limiter.SetLimit(rate.Limit(rps))
	al.limiter.SetBurst(burst)
	al.currentRPS = rps
}

// StartScheduleChecker starts a background goroutine to apply scheduled limits
func (drl *DatabaseRateLimiter) StartScheduleChecker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			drl.applyScheduledLimits()
		}
	}
}

func (drl *DatabaseRateLimiter) applyScheduledLimits() {
	hour := time.Now().Hour()
	
	drl.mu.RLock()
	defer drl.mu.RUnlock()
	
	for database, limiter := range drl.limiters {
		config := limiter.config
		if len(config.Schedule) == 0 {
			continue
		}
		
		// Find applicable schedule
		for _, schedule := range config.Schedule {
			if hour >= schedule.StartHour && hour < schedule.EndHour {
				if limiter.currentRPS != schedule.RPS {
					limiter.UpdateLimit(schedule.RPS, schedule.Burst)
					drl.logger.Info("Applied scheduled rate limit",
						zap.String("database", database),
						zap.Int("hour", hour),
						zap.Float64("rps", schedule.RPS),
					)
				}
				break
			}
		}
	}
}