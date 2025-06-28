package planattributeextractor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

// ComponentType is the name of this processor
const componentType = "plan_attribute_extractor"

// NewFactory creates a new factory for the plan attribute extractor processor
func NewFactory() processor.Factory {
	return processor.NewFactory(
		componentType,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, component.StabilityLevelAlpha),
	)
}

// createLogsProcessor creates a new plan attribute extractor processor for logs
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
		zap.String("component", componentType),
		zap.String("component_kind", "processor"),
	)
	
	logger.Info("Creating plan attribute extractor processor",
		zap.Int("timeout_ms", processorConfig.TimeoutMS),
		zap.String("error_mode", processorConfig.ErrorMode),
		zap.Bool("debug_logging", processorConfig.EnableDebugLogging))
	
	// Create and return the processor
	processor := newPlanAttributeExtractor(processorConfig, logger, nextConsumer)
	
	return processor, nil
}

// GetType returns the type of this processor
func GetType() component.Type {
	return componentType
}