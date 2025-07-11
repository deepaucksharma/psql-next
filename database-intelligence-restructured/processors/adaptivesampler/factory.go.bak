package adaptivesampler

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

// ComponentType is the name of this processor
var componentType = component.MustNewType("adaptivesampler")

// NewFactory creates a new factory for the adaptive sampler processor
func NewFactory() processor.Factory {
	return processor.NewFactory(
		componentType,
		CreateDefaultConfig,
		processor.WithLogs(createLogsProcessor, component.StabilityLevelAlpha),
		processor.WithMetrics(createMetricsProcessor, component.StabilityLevelAlpha),
	)
}

// createLogsProcessor creates a new adaptive sampler processor for logs
func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	
	// Cast config to our specific type
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type: expected *Config, got %T", cfg)
	}
	
	// Validate configuration
	if err := processorConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Create logger with component context
	logger := set.Logger.With(
		zap.String("component", componentType.String()),
		zap.String("component_kind", "processor"),
	)
	
	logger.Info("Creating adaptive sampler processor",
		zap.Bool("in_memory_only", processorConfig.InMemoryOnly),
		zap.Bool("deduplication_enabled", processorConfig.Deduplication.Enabled),
		zap.Int("num_sampling_rules", len(processorConfig.SamplingRules)),
		zap.Float64("default_sample_rate", processorConfig.DefaultSampleRate),
		zap.Int("max_records_per_second", processorConfig.MaxRecordsPerSecond),
		zap.Bool("debug_logging", processorConfig.EnableDebugLogging))
	
	// Create and return the processor
	processor, err := newAdaptiveSampler(processorConfig, logger, nextConsumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create adaptive sampler: %w", err)
	}
	
	return processor, nil
}

// createMetricsProcessor creates a new adaptive sampler processor for metrics
func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	
	// Cast config to our specific type
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid configuration type: expected *Config, got %T", cfg)
	}
	
	// Validate configuration
	if err := processorConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Create logger with component context
	logger := set.Logger.With(
		zap.String("component", componentType.String()),
		zap.String("component_kind", "processor"),
	)
	
	logger.Info("Creating adaptive sampler processor for metrics",
		zap.Bool("in_memory_only", processorConfig.InMemoryOnly),
		zap.Bool("deduplication_enabled", processorConfig.Deduplication.Enabled),
		zap.Int("num_sampling_rules", len(processorConfig.SamplingRules)),
		zap.Float64("default_sample_rate", processorConfig.DefaultSampleRate),
		zap.Int("max_records_per_second", processorConfig.MaxRecordsPerSecond),
		zap.Bool("debug_logging", processorConfig.EnableDebugLogging))
	
	// Create and return the processor with metrics consumer
	processor, err := newAdaptiveSamplerMetrics(processorConfig, logger, nextConsumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create adaptive sampler for metrics: %w", err)
	}
	
	return processor, nil
}

// GetType returns the type of this processor
func GetType() component.Type {
	return componentType
}