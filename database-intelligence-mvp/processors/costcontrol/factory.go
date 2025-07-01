package costcontrol

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

const (
	// TypeStr is the type string for this processor
	TypeStr = "costcontrol"
	// stability is the stability level of this processor
	stability = component.StabilityLevelBeta
)

// NewFactory creates a new processor factory
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(TypeStr),
		CreateDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
		processor.WithMetrics(createMetricsProcessor, stability),
		processor.WithLogs(createLogsProcessor, stability),
	)
}

// CreateDefaultConfig creates the default configuration
func CreateDefaultConfig() component.Config {
	return &Config{
		MonthlyBudgetUSD:       1000.0,
		PricePerGB:            0.35,
		MetricCardinalityLimit: 10000,
		SlowSpanThresholdMs:   2000,
		MaxLogBodySize:        10240,
		ReportingInterval:     60 * time.Second,
		AggressiveMode:        false,
		DataPlusEnabled:       false,
	}
}

// createTracesProcessor creates a traces processor
func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}

	if err := processorConfig.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	processor := newCostControlProcessor(processorConfig, set.Logger)
	processor.nextTraces = nextConsumer

	return processor, nil
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

	processor := newCostControlProcessor(processorConfig, set.Logger)
	processor.nextMetrics = nextConsumer

	return processor, nil
}

// createLogsProcessor creates a logs processor
func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}

	if err := processorConfig.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	processor := newCostControlProcessor(processorConfig, set.Logger)
	processor.nextLogs = nextConsumer

	return processor, nil
}

// newCostControlProcessor creates a new cost control processor instance
func newCostControlProcessor(config *Config, logger *zap.Logger) *costControlProcessor {
	return &costControlProcessor{
		config:            config,
		logger:            logger,
		costTracker:       &costTracker{currentMonth: time.Now()},
		metricCardinality: make(map[string]*cardinalityTracker),
	}
}