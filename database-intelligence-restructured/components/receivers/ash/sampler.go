package ash

import (
	"math"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AdaptiveSampler implements intelligent sampling based on session load
type AdaptiveSampler struct {
	config         SamplingConfig
	logger         *zap.Logger
	mu             sync.RWMutex
	
	// Current state
	currentRate    float64
	lastAdjustment time.Time
	
	// Metrics for adaptation
	recentSessionCounts []int
	recentSampleCounts  []int
	maxHistorySize      int
}

// LoadMetrics tracks system load for sampling decisions
type LoadMetrics struct {
	SessionCount       int
	ActiveSessions     int
	BlockedSessions    int
	LongRunningSessions int
	CPUUsage           float64
	MemoryUsage        float64
}

// NewAdaptiveSampler creates a new adaptive sampler
func NewAdaptiveSampler(config SamplingConfig, logger *zap.Logger) *AdaptiveSampler {
	return &AdaptiveSampler{
		config:              config,
		logger:              logger,
		currentRate:         config.BaseRate,
		lastAdjustment:      time.Now(),
		recentSessionCounts: make([]int, 0, 60),
		recentSampleCounts:  make([]int, 0, 60),
		maxHistorySize:      60,
	}
}

// CalculateSampleRate determines the sampling rate based on current load
func (s *AdaptiveSampler) CalculateSampleRate(sessionCount int) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Start with base rate
	rate := s.config.BaseRate
	
	// Adjust based on session count
	if sessionCount < s.config.LowSessionThreshold {
		// Low load - sample everything
		rate = 1.0
	} else if sessionCount > s.config.HighSessionThreshold {
		// High load - reduce sampling
		// Use exponential decay
		excess := float64(sessionCount - s.config.HighSessionThreshold)
		reduction := 1.0 - (excess / float64(s.config.HighSessionThreshold))
		rate = s.config.BaseRate * math.Max(reduction, 0.1)
	}
	
	// Apply min/max constraints
	rate = math.Max(rate, s.config.MinRate)
	rate = math.Min(rate, s.config.MaxRate)
	
	// Smooth rate changes to avoid oscillation
	if time.Since(s.lastAdjustment) > 10*time.Second {
		// Apply exponential moving average
		alpha := 0.3
		s.currentRate = alpha*rate + (1-alpha)*s.currentRate
		s.lastAdjustment = time.Now()
	} else {
		rate = s.currentRate
	}
	
	s.logger.Debug("Calculated sampling rate",
		zap.Int("session_count", sessionCount),
		zap.Float64("sample_rate", rate),
		zap.Float64("current_rate", s.currentRate))
	
	return rate
}

// ShouldSample determines if a specific session should be sampled
func (s *AdaptiveSampler) ShouldSample(session *ASHSample, baseRate float64) bool {
	// Always sample critical sessions regardless of rate
	if s.config.AlwaysSampleBlocked && session.BlockingPID > 0 {
		return true
	}
	
	// Always sample long-running queries
	if s.config.AlwaysSampleLongRunning && session.QueryDuration > 5*time.Minute {
		return true
	}
	
	// Always sample maintenance operations
	if s.config.AlwaysSampleMaintenance && s.isMaintenanceQuery(session.Query) {
		return true
	}
	
	// Apply session-specific multipliers
	multiplier := 1.0
	
	// Increase sampling for active sessions
	if session.State == "active" {
		multiplier *= 2.0
	}
	
	// Increase sampling for waiting sessions
	if session.WaitEvent != "" {
		multiplier *= 1.5
	}
	
	// Increase sampling for sessions with high impact
	if s.isHighImpactQuery(session.Query) {
		multiplier *= 2.0
	}
	
	// Final sampling decision
	effectiveRate := math.Min(baseRate*multiplier, 1.0)
	return randFloat() <= effectiveRate
}

// UpdateMetrics updates sampler metrics for adaptation
func (s *AdaptiveSampler) UpdateMetrics(sessionCount, sampleCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Add to history
	s.recentSessionCounts = append(s.recentSessionCounts, sessionCount)
	s.recentSampleCounts = append(s.recentSampleCounts, sampleCount)
	
	// Maintain history size
	if len(s.recentSessionCounts) > s.maxHistorySize {
		s.recentSessionCounts = s.recentSessionCounts[1:]
		s.recentSampleCounts = s.recentSampleCounts[1:]
	}
	
	// Log if sampling rate is significantly different from expected
	if sessionCount > 0 {
		actualRate := float64(sampleCount) / float64(sessionCount)
		if math.Abs(actualRate-s.currentRate) > 0.2 {
			s.logger.Warn("Sampling rate deviation",
				zap.Float64("expected_rate", s.currentRate),
				zap.Float64("actual_rate", actualRate),
				zap.Int("session_count", sessionCount),
				zap.Int("sample_count", sampleCount))
		}
	}
}

// GetCurrentRate returns the current sampling rate
func (s *AdaptiveSampler) GetCurrentRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentRate
}

// GetStatistics returns sampling statistics
func (s *AdaptiveSampler) GetStatistics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	stats := make(map[string]interface{})
	stats["current_rate"] = s.currentRate
	stats["last_adjustment"] = s.lastAdjustment
	
	// Calculate average session count
	if len(s.recentSessionCounts) > 0 {
		sum := 0
		for _, count := range s.recentSessionCounts {
			sum += count
		}
		stats["avg_session_count"] = float64(sum) / float64(len(s.recentSessionCounts))
	}
	
	// Calculate average sample count
	if len(s.recentSampleCounts) > 0 {
		sum := 0
		for _, count := range s.recentSampleCounts {
			sum += count
		}
		stats["avg_sample_count"] = float64(sum) / float64(len(s.recentSampleCounts))
	}
	
	return stats
}

// isMaintenanceQuery checks if a query is a maintenance operation
func (s *AdaptiveSampler) isMaintenanceQuery(query string) bool {
	maintenanceKeywords := []string{
		"VACUUM",
		"ANALYZE",
		"REINDEX",
		"CREATE INDEX",
		"DROP INDEX",
		"ALTER TABLE",
		"CLUSTER",
		"CHECKPOINT",
	}
	
	upperQuery := strings.ToUpper(query[:min(100, len(query))]) // Check first 100 chars
	for _, keyword := range maintenanceKeywords {
		if strings.Contains(upperQuery, keyword) {
			return true
		}
	}
	
	return false
}

// isHighImpactQuery checks if a query is likely to have high impact
func (s *AdaptiveSampler) isHighImpactQuery(query string) bool {
	highImpactPatterns := []string{
		"DELETE FROM",
		"UPDATE",
		"INSERT INTO",
		"TRUNCATE",
		"DROP TABLE",
		"CREATE TABLE",
		"ALTER TABLE",
	}
	
	upperQuery := strings.ToUpper(query[:min(50, len(query))]) // Check first 50 chars
	for _, pattern := range highImpactPatterns {
		if strings.Contains(upperQuery, pattern) {
			return true
		}
	}
	
	return false
}

// Helper functions

func randFloat() float64 {
	// Simple pseudo-random float between 0 and 1
	return float64(time.Now().UnixNano()%1000000) / 1000000.0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

