// Package adaptivesampler implements query-based adaptive sampling
// This addresses an OTEL gap - standard samplers don't adapt based on query performance
package adaptivesampler

import (
	"context"
	"math"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

// Config for adaptive sampler
type Config struct {
	// Queries taking longer than this are always sampled
	HighCostThresholdMs float64 `mapstructure:"high_cost_threshold_ms"`
	// Minimum sampling rate
	MinSamplingRate float64 `mapstructure:"min_sampling_rate"`
	// Maximum sampling rate  
	MaxSamplingRate float64 `mapstructure:"max_sampling_rate"`
	// Boost factor for queries with errors
	ErrorBoostFactor float64 `mapstructure:"error_boost_factor"`
}

// adaptiveSamplerProcessor implements adaptive sampling based on query performance
type adaptiveSamplerProcessor struct {
	config *Config
	logger *zap.Logger
	next   consumer.Logs
}

// newAdaptiveSamplerProcessor creates a new adaptive sampler
func newAdaptiveSamplerProcessor(config *Config, logger *zap.Logger, next consumer.Logs) *adaptiveSamplerProcessor {
	return &adaptiveSamplerProcessor{
		config: config,
		logger: logger,
		next:   next,
	}
}

// ConsumeLogs implements the consumer.Logs interface
func (asp *adaptiveSamplerProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Process each log record
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		sls := rl.ScopeLogs()
		
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			logs := sl.LogRecords()
			
			// Filter logs based on adaptive sampling
			logs.RemoveIf(func(lr plog.LogRecord) bool {
				return !asp.shouldSample(lr)
			})
		}
	}
	
	// Forward to next consumer
	return asp.next.ConsumeLogs(ctx, ld)
}

// shouldSample determines if a log record should be sampled
func (asp *adaptiveSamplerProcessor) shouldSample(lr plog.LogRecord) bool {
	attrs := lr.Attributes()
	
	// Extract query duration
	duration, hasDuration := attrs.Get("avg_duration_ms")
	if !hasDuration {
		// No duration info, use default rate
		return asp.randomSample(asp.config.MinSamplingRate)
	}
	
	// Check for errors
	hasError := false
	if errAttr, ok := attrs.Get("has_error"); ok {
		hasError = errAttr.Bool()
	}
	
	// Calculate sampling rate based on cost
	durationMs := duration.Double()
	samplingRate := asp.calculateSamplingRate(durationMs, hasError)
	
	return asp.randomSample(samplingRate)
}

// calculateSamplingRate determines the sampling rate based on query characteristics
func (asp *adaptiveSamplerProcessor) calculateSamplingRate(durationMs float64, hasError bool) float64 {
	// Always sample errors
	if hasError {
		return asp.config.MaxSamplingRate
	}
	
	// High cost queries get higher sampling rate
	if durationMs >= asp.config.HighCostThresholdMs {
		return asp.config.MaxSamplingRate
	}
	
	// Linear interpolation for sampling rate
	ratio := durationMs / asp.config.HighCostThresholdMs
	rate := asp.config.MinSamplingRate + (asp.config.MaxSamplingRate-asp.config.MinSamplingRate)*ratio
	
	return math.Min(rate, asp.config.MaxSamplingRate)
}

// randomSample performs random sampling
func (asp *adaptiveSamplerProcessor) randomSample(rate float64) bool {
	// Simple random sampling
	return pcommon.NewTraceID().HexString()[:2] < fmt.Sprintf("%02x", int(rate*255))
}

// Capabilities returns the consumer capabilities
func (asp *adaptiveSamplerProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start starts the processor
func (asp *adaptiveSamplerProcessor) Start(ctx context.Context, host component.Host) error {
	asp.logger.Info("Starting adaptive sampler processor")
	return nil
}

// Shutdown stops the processor
func (asp *adaptiveSamplerProcessor) Shutdown(ctx context.Context) error {
	asp.logger.Info("Stopping adaptive sampler processor")
	return nil
}