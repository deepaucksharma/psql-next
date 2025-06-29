package circuitbreaker

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

// ComponentType is the name of this processor
var componentType = component.MustNewType("circuitbreaker")

// NewFactory creates a new factory for the circuit breaker processor
func NewFactory() processor.Factory {
	return processor.NewFactory(
		componentType,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, component.StabilityLevelAlpha),
	)
}

// createLogsProcessor creates a new circuit breaker processor for logs
func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
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
	
	logger.Info("Creating circuit breaker processor",
		zap.Int("failure_threshold", processorConfig.FailureThreshold),
		zap.Int("success_threshold", processorConfig.SuccessThreshold),
		zap.Duration("open_state_timeout", processorConfig.OpenStateTimeout),
		zap.Int("max_concurrent_requests", processorConfig.MaxConcurrentRequests),
		zap.Duration("base_timeout", processorConfig.BaseTimeout),
		zap.Duration("max_timeout", processorConfig.MaxTimeout),
		zap.Bool("adaptive_timeout", processorConfig.EnableAdaptiveTimeout),
		zap.Duration("health_check_interval", processorConfig.HealthCheckInterval),
		zap.Int("memory_threshold_mb", processorConfig.MemoryThresholdMB),
		zap.Float64("cpu_threshold_percent", processorConfig.CPUThresholdPercent),
		zap.Bool("debug_logging", processorConfig.EnableDebugLogging))
	
	// Create and return the processor
	processor := newCircuitBreakerProcessor(processorConfig, logger, nextConsumer)
	
	return processor, nil
}

// GetType returns the type of this processor
func GetType() component.Type {
	return componentType
}