// Package adaptivesampler implements an adaptive sampling processor
// that can work with external state storage for high availability
package adaptivesampler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// adaptiveSamplerRefactoredProcessor implements adaptive sampling with external state
type adaptiveSamplerRefactoredProcessor struct {
	config *Config
	logger *zap.Logger

	// Next consumers in the pipeline
	nextMetrics consumer.Metrics
	nextLogs    consumer.Logs
	nextTraces  consumer.Traces

	// Sampling strategies
	strategies map[string]SamplingStrategy
	mu         sync.RWMutex

	// Deduplication with external state
	dedupeStore DeduplicationStore

	// Rate limiting
	rateLimiter *RateLimiter

	// Metrics
	samplingMetrics *SamplingMetrics

	// Shutdown
	shutdownChan chan struct{}
	wg           sync.WaitGroup
}

// SamplingStrategy interface for different sampling strategies
type SamplingStrategy interface {
	// ShouldSample decides if a data point should be sampled
	ShouldSample(ctx context.Context, attributes pcommon.Map) (bool, float64)
	// UpdateStrategy updates the strategy based on feedback
	UpdateStrategy(ctx context.Context, feedback StrategyFeedback) error
	// GetCurrentRate returns the current sampling rate
	GetCurrentRate() float64
}

// DeduplicationStore interface for external deduplication state
type DeduplicationStore interface {
	// CheckAndSet checks if a hash exists and sets it if not
	// Returns true if the hash is new (should be sampled)
	CheckAndSet(ctx context.Context, hash string, ttl time.Duration) (bool, error)
	// GetStats returns deduplication statistics
	GetStats(ctx context.Context) (*DedupeStats, error)
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	maxRate      float64
	currentRate  float64
	tokens       float64
	lastUpdate   time.Time
	mu           sync.Mutex
}

// SamplingMetrics tracks sampling performance
type SamplingMetrics struct {
	TotalItems      int64
	SampledItems    int64
	DroppedItems    int64
	DedupedItems    int64
	RateLimitedItems int64
	mu              sync.Mutex
}

// DedupeStats represents deduplication statistics
type DedupeStats struct {
	TotalChecks   int64
	UniqueItems   int64
	DuplicateItems int64
}

// StrategyFeedback provides feedback to sampling strategies
type StrategyFeedback struct {
	VolumePerSecond float64
	ErrorRate       float64
	AverageLatency  time.Duration
	ResourceUsage   float64
}

// newAdaptiveSamplerRefactoredProcessor creates a new adaptive sampler
func newAdaptiveSamplerRefactoredProcessor(
	cfg *Config,
	logger *zap.Logger,
	nextMetrics consumer.Metrics,
	nextLogs consumer.Logs,
	nextTraces consumer.Traces,
	dedupeStore DeduplicationStore,
) (*adaptiveSamplerRefactoredProcessor, error) {

	p := &adaptiveSamplerRefactoredProcessor{
		config:      cfg,
		logger:      logger,
		nextMetrics: nextMetrics,
		nextLogs:    nextLogs,
		nextTraces:  nextTraces,
		strategies:  make(map[string]SamplingStrategy),
		dedupeStore: dedupeStore,
		rateLimiter: &RateLimiter{
			maxRate:     cfg.MaxSamplesPerSecond,
			currentRate: cfg.MaxSamplesPerSecond,
			tokens:      cfg.MaxSamplesPerSecond,
			lastUpdate:  time.Now(),
		},
		samplingMetrics: &SamplingMetrics{},
		shutdownChan:    make(chan struct{}),
	}

	// Initialize sampling strategies
	if err := p.initializeStrategies(); err != nil {
		return nil, fmt.Errorf("failed to initialize strategies: %w", err)
	}

	return p, nil
}

// Start implements the component.Component interface
func (p *adaptiveSamplerRefactoredProcessor) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting adaptive sampler processor")

	// Start strategy updater
	p.wg.Add(1)
	go p.strategyUpdater(ctx)

	// Start metrics reporter
	p.wg.Add(1)
	go p.metricsReporter(ctx)

	return nil
}

// Shutdown implements the component.Component interface
func (p *adaptiveSamplerRefactoredProcessor) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down adaptive sampler processor")

	close(p.shutdownChan)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ConsumeMetrics implements the consumer.Metrics interface
func (p *adaptiveSamplerRefactoredProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Process each resource metric
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		
		// Apply sampling to each metric
		for j := rm.ScopeMetrics().Len() - 1; j >= 0; j-- {
			sm := rm.ScopeMetrics().At(j)
			
			for k := sm.Metrics().Len() - 1; k >= 0; k-- {
				metric := sm.Metrics().At(k)
				
				// Check if metric should be sampled
				if !p.shouldSampleMetric(ctx, rm.Resource().Attributes(), metric) {
					sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
						return m.Name() == metric.Name()
					})
				}
			}
			
			// Remove empty scopes
			if sm.Metrics().Len() == 0 {
				rm.ScopeMetrics().RemoveIf(func(s pmetric.ScopeMetrics) bool {
					return s.Metrics().Len() == 0
				})
			}
		}
	}

	// Remove empty resources
	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		return rm.ScopeMetrics().Len() == 0
	})

	// Pass to next consumer if there's data
	if md.ResourceMetrics().Len() > 0 {
		return p.nextMetrics.ConsumeMetrics(ctx, md)
	}

	return nil
}

// ConsumeLogs implements the consumer.Logs interface
func (p *adaptiveSamplerRefactoredProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Process each resource log
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		
		for j := rl.ScopeLogs().Len() - 1; j >= 0; j-- {
			sl := rl.ScopeLogs().At(j)
			
			for k := sl.LogRecords().Len() - 1; k >= 0; k-- {
				record := sl.LogRecords().At(k)
				
				// Check if log should be sampled
				if !p.shouldSampleLog(ctx, rl.Resource().Attributes(), record) {
					sl.LogRecords().RemoveIf(func(lr plog.LogRecord) bool {
						return lr.Timestamp() == record.Timestamp()
					})
				}
			}
			
			// Remove empty scopes
			if sl.LogRecords().Len() == 0 {
				rl.ScopeLogs().RemoveIf(func(s plog.ScopeLogs) bool {
					return s.LogRecords().Len() == 0
				})
			}
		}
	}

	// Remove empty resources
	ld.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
		return rl.ScopeLogs().Len() == 0
	})

	// Pass to next consumer if there's data
	if ld.ResourceLogs().Len() > 0 {
		return p.nextLogs.ConsumeLogs(ctx, ld)
	}

	return nil
}

// ConsumeTraces implements the consumer.Traces interface
func (p *adaptiveSamplerRefactoredProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// For traces, we typically sample at the trace level, not span level
	for i := td.ResourceSpans().Len() - 1; i >= 0; i-- {
		rs := td.ResourceSpans().At(i)
		
		// Check if trace should be sampled
		if !p.shouldSampleTrace(ctx, rs) {
			td.ResourceSpans().RemoveIf(func(r ptrace.ResourceSpans) bool {
				// Compare by checking if it's the same resource
				return r.Resource().Attributes().AsRaw() == rs.Resource().Attributes().AsRaw()
			})
		}
	}

	// Pass to next consumer if there's data
	if td.ResourceSpans().Len() > 0 {
		return p.nextTraces.ConsumeTraces(ctx, td)
	}

	return nil
}

// shouldSampleMetric determines if a metric should be sampled
func (p *adaptiveSamplerRefactoredProcessor) shouldSampleMetric(
	ctx context.Context,
	resourceAttrs pcommon.Map,
	metric pmetric.Metric,
) bool {
	p.samplingMetrics.mu.Lock()
	p.samplingMetrics.TotalItems++
	p.samplingMetrics.mu.Unlock()

	// Check deduplication
	hash := p.computeMetricHash(resourceAttrs, metric)
	if p.dedupeStore != nil {
		isNew, err := p.dedupeStore.CheckAndSet(ctx, hash, p.config.DeduplicationTTL)
		if err != nil {
			p.logger.Warn("Deduplication check failed", zap.Error(err))
		} else if !isNew {
			p.samplingMetrics.mu.Lock()
			p.samplingMetrics.DedupedItems++
			p.samplingMetrics.mu.Unlock()
			return false
		}
	}

	// Check rate limit
	if !p.rateLimiter.Allow() {
		p.samplingMetrics.mu.Lock()
		p.samplingMetrics.RateLimitedItems++
		p.samplingMetrics.mu.Unlock()
		return false
	}

	// Apply sampling strategy
	strategyName := p.selectStrategy(resourceAttrs)
	strategy, exists := p.strategies[strategyName]
	if !exists {
		strategy = p.strategies["default"]
	}

	shouldSample, _ := strategy.ShouldSample(ctx, resourceAttrs)
	
	if shouldSample {
		p.samplingMetrics.mu.Lock()
		p.samplingMetrics.SampledItems++
		p.samplingMetrics.mu.Unlock()
	} else {
		p.samplingMetrics.mu.Lock()
		p.samplingMetrics.DroppedItems++
		p.samplingMetrics.mu.Unlock()
	}

	return shouldSample
}

// shouldSampleLog determines if a log should be sampled
func (p *adaptiveSamplerRefactoredProcessor) shouldSampleLog(
	ctx context.Context,
	resourceAttrs pcommon.Map,
	record plog.LogRecord,
) bool {
	// Combine resource and record attributes for sampling decision
	allAttrs := pcommon.NewMap()
	resourceAttrs.CopyTo(allAttrs)
	record.Attributes().CopyTo(allAttrs)

	// Check for error logs - always sample errors
	if severityText := record.SeverityText(); severityText == "ERROR" || severityText == "FATAL" {
		return true
	}

	// Apply regular sampling logic
	return p.shouldSampleWithAttributes(ctx, allAttrs)
}

// shouldSampleTrace determines if a trace should be sampled
func (p *adaptiveSamplerRefactoredProcessor) shouldSampleTrace(
	ctx context.Context,
	rs ptrace.ResourceSpans,
) bool {
	// For traces, check if any span has an error
	for i := 0; i < rs.ScopeSpans().Len(); i++ {
		ss := rs.ScopeSpans().At(i)
		for j := 0; j < ss.Spans().Len(); j++ {
			span := ss.Spans().At(j)
			if span.Status().Code() == ptrace.StatusCodeError {
				return true // Always sample error traces
			}
		}
	}

	// Apply regular sampling logic
	return p.shouldSampleWithAttributes(ctx, rs.Resource().Attributes())
}

// shouldSampleWithAttributes applies sampling logic to attributes
func (p *adaptiveSamplerRefactoredProcessor) shouldSampleWithAttributes(
	ctx context.Context,
	attrs pcommon.Map,
) bool {
	strategyName := p.selectStrategy(attrs)
	strategy, exists := p.strategies[strategyName]
	if !exists {
		strategy = p.strategies["default"]
	}

	shouldSample, _ := strategy.ShouldSample(ctx, attrs)
	return shouldSample
}

// computeMetricHash computes a hash for deduplication
func (p *adaptiveSamplerRefactoredProcessor) computeMetricHash(
	resourceAttrs pcommon.Map,
	metric pmetric.Metric,
) string {
	h := sha256.New()
	
	// Include metric name
	h.Write([]byte(metric.Name()))
	
	// Include key resource attributes
	if service, ok := resourceAttrs.Get("service.name"); ok {
		h.Write([]byte(service.AsString()))
	}
	if dbName, ok := resourceAttrs.Get("db.name"); ok {
		h.Write([]byte(dbName.AsString()))
	}
	
	// Include metric-specific identifiers based on type
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		// For gauges, include the data point attributes
		if metric.Gauge().DataPoints().Len() > 0 {
			dp := metric.Gauge().DataPoints().At(0)
			dp.Attributes().Range(func(k string, v pcommon.Value) bool {
				h.Write([]byte(k))
				h.Write([]byte(v.AsString()))
				return true
			})
		}
	case pmetric.MetricTypeSum:
		// Similar for sums
		if metric.Sum().DataPoints().Len() > 0 {
			dp := metric.Sum().DataPoints().At(0)
			dp.Attributes().Range(func(k string, v pcommon.Value) bool {
				h.Write([]byte(k))
				h.Write([]byte(v.AsString()))
				return true
			})
		}
	}
	
	return hex.EncodeToString(h.Sum(nil))
}

// selectStrategy selects the appropriate sampling strategy
func (p *adaptiveSamplerRefactoredProcessor) selectStrategy(attrs pcommon.Map) string {
	// Check rules in order
	for _, rule := range p.config.Rules {
		if p.matchesRule(attrs, rule) {
			return rule.Strategy
		}
	}
	
	return "default"
}

// matchesRule checks if attributes match a sampling rule
func (p *adaptiveSamplerRefactoredProcessor) matchesRule(attrs pcommon.Map, rule SamplingRule) bool {
	for key, value := range rule.Attributes {
		if attrVal, ok := attrs.Get(key); !ok || attrVal.AsString() != value {
			return false
		}
	}
	return true
}

// initializeStrategies initializes sampling strategies
func (p *adaptiveSamplerRefactoredProcessor) initializeStrategies() error {
	// Create default strategy
	p.strategies["default"] = NewProbabilisticStrategy(p.config.DefaultSamplingRate)
	
	// Create configured strategies
	for name, cfg := range p.config.Strategies {
		switch cfg.Type {
		case "probabilistic":
			p.strategies[name] = NewProbabilisticStrategy(cfg.InitialRate)
		case "adaptive_rate":
			p.strategies[name] = NewAdaptiveRateStrategy(cfg)
		case "adaptive_cost":
			p.strategies[name] = NewAdaptiveCostStrategy(cfg)
		case "adaptive_error":
			p.strategies[name] = NewAdaptiveErrorStrategy(cfg)
		default:
			return fmt.Errorf("unknown strategy type: %s", cfg.Type)
		}
	}
	
	return nil
}

// RateLimiter.Allow checks if a request should be allowed
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(r.lastUpdate).Seconds()
	r.lastUpdate = now
	
	// Replenish tokens
	r.tokens += elapsed * r.currentRate
	if r.tokens > r.maxRate {
		r.tokens = r.maxRate
	}
	
	// Check if we have a token
	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	
	return false
}

// strategyUpdater periodically updates sampling strategies
func (p *adaptiveSamplerRefactoredProcessor) strategyUpdater(ctx context.Context) {
	defer p.wg.Done()
	
	ticker := time.NewTicker(p.config.StrategyUpdateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.updateStrategies(ctx)
		case <-p.shutdownChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// updateStrategies updates all adaptive strategies
func (p *adaptiveSamplerRefactoredProcessor) updateStrategies(ctx context.Context) {
	// Collect feedback metrics
	feedback := p.collectFeedback()
	
	p.mu.RLock()
	strategies := make(map[string]SamplingStrategy)
	for k, v := range p.strategies {
		strategies[k] = v
	}
	p.mu.RUnlock()
	
	// Update each strategy
	for name, strategy := range strategies {
		if err := strategy.UpdateStrategy(ctx, feedback); err != nil {
			p.logger.Warn("Failed to update strategy",
				zap.String("strategy", name),
				zap.Error(err))
		}
	}
}

// collectFeedback collects system feedback for strategies
func (p *adaptiveSamplerRefactoredProcessor) collectFeedback() StrategyFeedback {
	p.samplingMetrics.mu.Lock()
	total := p.samplingMetrics.TotalItems
	p.samplingMetrics.mu.Unlock()
	
	// Calculate volume per second
	volumePerSecond := float64(total) / time.Since(time.Now()).Seconds()
	
	return StrategyFeedback{
		VolumePerSecond: volumePerSecond,
		ErrorRate:       0.0, // Would be calculated from actual error metrics
		AverageLatency:  0,   // Would be calculated from actual latency metrics
		ResourceUsage:   0.0, // Would be calculated from actual resource metrics
	}
}

// metricsReporter periodically reports sampling metrics
func (p *adaptiveSamplerRefactoredProcessor) metricsReporter(ctx context.Context) {
	defer p.wg.Done()
	
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.reportMetrics()
		case <-p.shutdownChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// reportMetrics logs sampling metrics
func (p *adaptiveSamplerRefactoredProcessor) reportMetrics() {
	p.samplingMetrics.mu.Lock()
	defer p.samplingMetrics.mu.Unlock()
	
	samplingRate := float64(p.samplingMetrics.SampledItems) / float64(p.samplingMetrics.TotalItems)
	
	p.logger.Info("Adaptive sampling metrics",
		zap.Int64("total_items", p.samplingMetrics.TotalItems),
		zap.Int64("sampled_items", p.samplingMetrics.SampledItems),
		zap.Int64("dropped_items", p.samplingMetrics.DroppedItems),
		zap.Int64("deduped_items", p.samplingMetrics.DedupedItems),
		zap.Int64("rate_limited_items", p.samplingMetrics.RateLimitedItems),
		zap.Float64("effective_sampling_rate", samplingRate))
	
	// Log strategy rates
	p.mu.RLock()
	for name, strategy := range p.strategies {
		p.logger.Info("Strategy rate",
			zap.String("strategy", name),
			zap.Float64("rate", strategy.GetCurrentRate()))
	}
	p.mu.RUnlock()
}