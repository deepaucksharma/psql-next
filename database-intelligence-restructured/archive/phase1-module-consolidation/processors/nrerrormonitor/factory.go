package nrerrormonitor

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// TypeStr is the type string for this processor
	TypeStr = "nrerrormonitor"
	// stability is the stability level of this processor
	stability = component.StabilityLevelBeta
)

// NewFactory creates a new processor factory
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(TypeStr),
		CreateDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}

// CreateDefaultConfig creates the default configuration
func CreateDefaultConfig() component.Config {
	return &Config{
		MaxAttributeLength:          4096,
		MaxMetricNameLength:         255,
		CardinalityWarningThreshold: 10000,
		AlertThreshold:              100,
		ReportingInterval:           60 * time.Second,
		ErrorSuppressionDuration:    5 * time.Minute,
		EnableProactiveValidation:   true,
	}
}

// createMetricsProcessor creates a metrics processor
func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}

	if err := processorConfig.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	processor := newNrErrorMonitor(processorConfig, set.Logger, nextConsumer)

	return processor, nil
}