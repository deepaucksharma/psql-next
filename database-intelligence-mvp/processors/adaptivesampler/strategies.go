package adaptivesampler

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// ProbabilisticStrategy implements simple probability-based sampling
type ProbabilisticStrategy struct {
	rate float64
	mu   sync.RWMutex
}

// NewProbabilisticStrategy creates a new probabilistic strategy
func NewProbabilisticStrategy(rate float64) *ProbabilisticStrategy {
	return &ProbabilisticStrategy{
		rate: rate,
	}
}

// ShouldSample decides based on probability
func (s *ProbabilisticStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
	s.mu.RLock()
	rate := s.rate
	s.mu.RUnlock()

	return rand.Float64() < rate, rate
}

// UpdateStrategy updates the sampling rate
func (s *ProbabilisticStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
	// Simple probabilistic strategy doesn't adapt
	return nil
}

// GetCurrentRate returns the current sampling rate
func (s *ProbabilisticStrategy) GetCurrentRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rate
}

// AdaptiveRateStrategy adjusts sampling based on volume
type AdaptiveRateStrategy struct {
	config      StrategyConfig
	currentRate float64
	mu          sync.RWMutex
}

// NewAdaptiveRateStrategy creates a new adaptive rate strategy
func NewAdaptiveRateStrategy(config StrategyConfig) *AdaptiveRateStrategy {
	return &AdaptiveRateStrategy{
		config:      config,
		currentRate: config.InitialRate.Value(),
	}
}

// ShouldSample decides based on current adaptive rate
func (s *AdaptiveRateStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
	s.mu.RLock()
	rate := s.currentRate
	s.mu.RUnlock()

	return rand.Float64() < rate, rate
}

// UpdateStrategy adjusts rate based on volume
func (s *AdaptiveRateStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Adjust based on volume
	if feedback.VolumePerSecond > s.config.VolumeThresholds.High {
		// Decrease sampling when volume is high
		s.currentRate = s.currentRate * 0.9
		if s.currentRate < s.config.MinRate.Value() {
			s.currentRate = s.config.MinRate.Value()
		}
	} else if feedback.VolumePerSecond < s.config.VolumeThresholds.Low {
		// Increase sampling when volume is low
		s.currentRate = s.currentRate * 1.1
		if s.currentRate > s.config.MaxRate.Value() {
			s.currentRate = s.config.MaxRate.Value()
		}
	}

	return nil
}

// GetCurrentRate returns the current sampling rate
func (s *AdaptiveRateStrategy) GetCurrentRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentRate
}

// AdaptiveCostStrategy samples based on query cost
type AdaptiveCostStrategy struct {
	config      StrategyConfig
	currentRate float64
	mu          sync.RWMutex
}

// NewAdaptiveCostStrategy creates a new cost-based strategy
func NewAdaptiveCostStrategy(config StrategyConfig) *AdaptiveCostStrategy {
	return &AdaptiveCostStrategy{
		config:      config,
		currentRate: config.InitialRate.Value(),
	}
}

// ShouldSample samples expensive queries more frequently
func (s *AdaptiveCostStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
	s.mu.RLock()
	baseRate := s.currentRate
	s.mu.RUnlock()

	// Check if query cost is available
	cost, exists := attributes.Get("query.cost")
	if !exists {
		return rand.Float64() < baseRate, baseRate
	}

	costValue := cost.Double()
	
	// Always sample high-cost queries
	if costValue > s.config.CostThresholds.High {
		return true, 1.0
	}

	// Use base rate for normal cost queries
	return rand.Float64() < baseRate, baseRate
}

// UpdateStrategy adjusts based on cost distribution
func (s *AdaptiveCostStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Adjust rate based on average cost
	if feedback.AverageCost > s.config.CostThresholds.High {
		// Increase sampling when costs are high
		s.currentRate = s.config.MaxRate.Value()
	} else {
		s.currentRate = s.config.InitialRate.Value()
	}

	return nil
}

// GetCurrentRate returns the current sampling rate
func (s *AdaptiveCostStrategy) GetCurrentRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentRate
}

// AdaptiveErrorStrategy samples based on error rates
type AdaptiveErrorStrategy struct {
	config      StrategyConfig
	currentRate float64
	mu          sync.RWMutex
}

// NewAdaptiveErrorStrategy creates a new error-based strategy
func NewAdaptiveErrorStrategy(config StrategyConfig) *AdaptiveErrorStrategy {
	return &AdaptiveErrorStrategy{
		config:      config,
		currentRate: config.InitialRate.Value(),
	}
}

// ShouldSample always samples errors
func (s *AdaptiveErrorStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
	// Always sample if it's an error
	if severity, ok := attributes.Get("severity"); ok {
		if severity.Str() == "ERROR" || severity.Str() == "FATAL" {
			return true, 1.0
		}
	}

	// Check for error indicators
	if hasError, ok := attributes.Get("has_error"); ok && hasError.Bool() {
		return true, 1.0
	}

	s.mu.RLock()
	rate := s.currentRate
	s.mu.RUnlock()

	return rand.Float64() < rate, rate
}

// UpdateStrategy adjusts based on error rate
func (s *AdaptiveErrorStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Increase sampling when error rate is high
	if feedback.ErrorRate > s.config.ErrorThresholds.High {
		s.currentRate = s.config.MaxRate.Value()
	} else if feedback.ErrorRate < s.config.ErrorThresholds.Low {
		s.currentRate = s.config.InitialRate.Value()
	}

	return nil
}

// GetCurrentRate returns the current sampling rate
func (s *AdaptiveErrorStrategy) GetCurrentRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentRate
}

// AlwaysSampleStrategy samples everything
type AlwaysSampleStrategy struct{}

// NewAlwaysSampleStrategy creates a strategy that samples everything
func NewAlwaysSampleStrategy() *AlwaysSampleStrategy {
	return &AlwaysSampleStrategy{}
}

// ShouldSample always returns true
func (s *AlwaysSampleStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
	return true, 1.0
}

// UpdateStrategy does nothing
func (s *AlwaysSampleStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
	return nil
}

// GetCurrentRate always returns 1.0
func (s *AlwaysSampleStrategy) GetCurrentRate() float64 {
	return 1.0
}

// NeverSampleStrategy drops everything
type NeverSampleStrategy struct{}

// NewNeverSampleStrategy creates a strategy that drops everything
func NewNeverSampleStrategy() *NeverSampleStrategy {
	return &NeverSampleStrategy{}
}

// ShouldSample always returns false
func (s *NeverSampleStrategy) ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64) {
	return false, 0.0
}

// UpdateStrategy does nothing
func (s *NeverSampleStrategy) UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error {
	return nil
}

// GetCurrentRate always returns 0.0
func (s *NeverSampleStrategy) GetCurrentRate() float64 {
	return 0.0
}