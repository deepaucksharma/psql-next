package adaptivesampler

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// ProcessorMetrics holds all metrics for the adaptive sampler processor
type ProcessorMetrics struct {
	meter              metric.Meter
	logger             *zap.Logger
	
	// Core processing metrics
	recordsProcessed   metric.Int64Counter
	recordsDropped     metric.Int64Counter
	recordsSampled     metric.Int64Counter
	processingTime     metric.Float64Histogram
	
	// Rule evaluation metrics
	rulesEvaluated     metric.Int64Counter
	ruleMatches        metric.Int64Counter
	ruleEvalTime       metric.Float64Histogram
	ruleMatchesByName  map[string]metric.Int64Counter
	
	// Cache metrics
	cacheHits          metric.Int64Counter
	cacheMisses        metric.Int64Counter
	cacheEvictions     metric.Int64Counter
	cacheSize          metric.Int64Gauge
	
	// Rate limiting metrics
	rateLimitHits      metric.Int64Counter
	rateLimitMisses    metric.Int64Counter
	
	// Health metrics
	lastProcessedTime  time.Time
	isHealthy          bool
	healthMutex        sync.RWMutex
	
	// Configuration
	config             ProcessorMetricsConfig
}

// NewProcessorMetrics creates a new metrics instance
func NewProcessorMetrics(config ProcessorMetricsConfig, logger *zap.Logger) (*ProcessorMetrics, error) {
	meter := otel.Meter("adaptivesampler")
	
	pm := &ProcessorMetrics{
		meter:             meter,
		logger:            logger,
		config:            config,
		ruleMatchesByName: make(map[string]metric.Int64Counter),
		isHealthy:         true,
	}
	
	// Initialize core metrics
	var err error
	
	pm.recordsProcessed, err = meter.Int64Counter(
		"adaptive_sampler_records_processed",
		metric.WithDescription("Total number of records processed"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	
	pm.recordsDropped, err = meter.Int64Counter(
		"adaptive_sampler_records_dropped",
		metric.WithDescription("Total number of records dropped"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	
	pm.recordsSampled, err = meter.Int64Counter(
		"adaptive_sampler_records_sampled",
		metric.WithDescription("Total number of records sampled"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	
	pm.processingTime, err = meter.Float64Histogram(
		"adaptive_sampler_processing_time",
		metric.WithDescription("Time taken to process records"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}
	
	// Initialize rule metrics
	pm.rulesEvaluated, err = meter.Int64Counter(
		"adaptive_sampler_rules_evaluated",
		metric.WithDescription("Total number of rule evaluations"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	
	pm.ruleMatches, err = meter.Int64Counter(
		"adaptive_sampler_rule_matches",
		metric.WithDescription("Total number of rule matches"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	
	pm.ruleEvalTime, err = meter.Float64Histogram(
		"adaptive_sampler_rule_eval_time",
		metric.WithDescription("Time taken to evaluate rules"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}
	
	// Initialize cache metrics
	if config.IncludeCacheMetrics {
		pm.cacheHits, err = meter.Int64Counter(
			"adaptive_sampler_cache_hits",
			metric.WithDescription("Number of cache hits"),
			metric.WithUnit("1"),
		)
		if err != nil {
			return nil, err
		}
		
		pm.cacheMisses, err = meter.Int64Counter(
			"adaptive_sampler_cache_misses",
			metric.WithDescription("Number of cache misses"),
			metric.WithUnit("1"),
		)
		if err != nil {
			return nil, err
		}
		
		pm.cacheEvictions, err = meter.Int64Counter(
			"adaptive_sampler_cache_evictions",
			metric.WithDescription("Number of cache evictions"),
			metric.WithUnit("1"),
		)
		if err != nil {
			return nil, err
		}
		
		pm.cacheSize, err = meter.Int64Gauge(
			"adaptive_sampler_cache_size",
			metric.WithDescription("Current cache size"),
			metric.WithUnit("1"),
		)
		if err != nil {
			return nil, err
		}
	}
	
	// Initialize rate limiting metrics
	pm.rateLimitHits, err = meter.Int64Counter(
		"adaptive_sampler_rate_limit_hits",
		metric.WithDescription("Number of rate limit hits"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	
	pm.rateLimitMisses, err = meter.Int64Counter(
		"adaptive_sampler_rate_limit_misses",
		metric.WithDescription("Number of rate limit misses"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return nil, err
	}
	
	return pm, nil
}

// RecordProcessed increments the processed counter
func (pm *ProcessorMetrics) RecordProcessed(ctx context.Context, count int64, attributes ...attribute.KeyValue) {
	pm.recordsProcessed.Add(ctx, count, metric.WithAttributes(attributes...))
	pm.updateLastProcessedTime()
}

// RecordDropped increments the dropped counter
func (pm *ProcessorMetrics) RecordDropped(ctx context.Context, count int64, reason string) {
	attrs := []attribute.KeyValue{
		attribute.String("reason", reason),
	}
	pm.recordsDropped.Add(ctx, count, metric.WithAttributes(attrs...))
}

// RecordSampled increments the sampled counter
func (pm *ProcessorMetrics) RecordSampled(ctx context.Context, count int64, ruleName string) {
	attrs := []attribute.KeyValue{
		attribute.String("rule", ruleName),
	}
	pm.recordsSampled.Add(ctx, count, metric.WithAttributes(attrs...))
	
	// Track per-rule metrics if enabled
	if pm.config.IncludeRuleMetrics {
		pm.recordRuleMatch(ctx, ruleName)
	}
}

// RecordProcessingTime records the time taken to process
func (pm *ProcessorMetrics) RecordProcessingTime(ctx context.Context, durationMs float64) {
	pm.processingTime.Record(ctx, durationMs)
}

// RecordRuleEvaluation records rule evaluation metrics
func (pm *ProcessorMetrics) RecordRuleEvaluation(ctx context.Context, ruleName string, matched bool, durationMs float64) {
	attrs := []attribute.KeyValue{
		attribute.String("rule", ruleName),
		attribute.Bool("matched", matched),
	}
	
	pm.rulesEvaluated.Add(ctx, 1, metric.WithAttributes(attrs...))
	pm.ruleEvalTime.Record(ctx, durationMs, metric.WithAttributes(attrs...))
	
	if matched {
		pm.ruleMatches.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordCacheHit records a cache hit
func (pm *ProcessorMetrics) RecordCacheHit(ctx context.Context) {
	if pm.config.IncludeCacheMetrics && pm.cacheHits != nil {
		pm.cacheHits.Add(ctx, 1)
	}
}

// RecordCacheMiss records a cache miss
func (pm *ProcessorMetrics) RecordCacheMiss(ctx context.Context) {
	if pm.config.IncludeCacheMetrics && pm.cacheMisses != nil {
		pm.cacheMisses.Add(ctx, 1)
	}
}

// UpdateCacheSize updates the current cache size
func (pm *ProcessorMetrics) UpdateCacheSize(ctx context.Context, size int64) {
	if pm.config.IncludeCacheMetrics && pm.cacheSize != nil {
		pm.cacheSize.Record(ctx, size)
	}
}

// RecordRateLimitHit records when rate limit is hit
func (pm *ProcessorMetrics) RecordRateLimitHit(ctx context.Context, ruleName string) {
	attrs := []attribute.KeyValue{
		attribute.String("rule", ruleName),
	}
	pm.rateLimitHits.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// GetHealthMetrics returns current health metrics
func (pm *ProcessorMetrics) GetHealthMetrics() map[string]interface{} {
	pm.healthMutex.RLock()
	defer pm.healthMutex.RUnlock()
	
	timeSinceLastProcessed := time.Since(pm.lastProcessedTime)
	
	return map[string]interface{}{
		"healthy":                 pm.isHealthy,
		"last_processed_time":     pm.lastProcessedTime,
		"time_since_last_processed": timeSinceLastProcessed.Seconds(),
		"stale":                   timeSinceLastProcessed > 5*time.Minute,
	}
}

// IsHealthy returns the current health status
func (pm *ProcessorMetrics) IsHealthy() bool {
	pm.healthMutex.RLock()
	defer pm.healthMutex.RUnlock()
	
	// Consider unhealthy if no processing in last 5 minutes
	if time.Since(pm.lastProcessedTime) > 5*time.Minute {
		return false
	}
	
	return pm.isHealthy
}

// SetHealthy updates the health status
func (pm *ProcessorMetrics) SetHealthy(healthy bool) {
	pm.healthMutex.Lock()
	defer pm.healthMutex.Unlock()
	pm.isHealthy = healthy
}

// Private helper methods

func (pm *ProcessorMetrics) updateLastProcessedTime() {
	pm.healthMutex.Lock()
	defer pm.healthMutex.Unlock()
	pm.lastProcessedTime = time.Now()
}

func (pm *ProcessorMetrics) recordRuleMatch(ctx context.Context, ruleName string) {
	counter, exists := pm.ruleMatchesByName[ruleName]
	if !exists {
		// Create counter for this rule
		var err error
		counter, err = pm.meter.Int64Counter(
			"adaptive_sampler_rule_matches_by_name",
			metric.WithDescription("Rule matches by rule name"),
			metric.WithUnit("1"),
		)
		if err != nil {
			pm.logger.Error("Failed to create rule counter", zap.Error(err))
			return
		}
		pm.ruleMatchesByName[ruleName] = counter
	}
	
	counter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("rule_name", ruleName),
	))
}

// StartMetricsReporter starts a background metrics reporter
func (pm *ProcessorMetrics) StartMetricsReporter(ctx context.Context) {
	if !pm.config.Enabled || pm.config.Interval <= 0 {
		return
	}
	
	ticker := time.NewTicker(pm.config.Interval)
	go func() {
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pm.reportHealthMetrics(ctx)
			}
		}
	}()
}

func (pm *ProcessorMetrics) reportHealthMetrics(ctx context.Context) {
	health := pm.GetHealthMetrics()
	
	// Log health status
	if stale, ok := health["stale"].(bool); ok && stale {
		pm.logger.Warn("Processor appears stale", 
			zap.Any("health_metrics", health))
	}
	
	// Could also emit as metrics if needed
	// This is where you'd send additional telemetry to New Relic
}