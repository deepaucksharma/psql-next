package querycorrelator

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

const (
	// TypeStr is the type string for this processor
	TypeStr = "querycorrelator"
	// stability is the stability level of this processor
	stability = component.StabilityLevelBeta
)

// NewFactory creates a new processor factory
func NewFactory() processor.Factory {
	return processor.NewFactory(
		TypeStr,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}

// createDefaultConfig creates the default configuration
func createDefaultConfig() component.Config {
	return &Config{
		RetentionPeriod:   24 * time.Hour,
		CleanupInterval:   1 * time.Hour,
		EnableTableCorrelation: true,
		EnableDatabaseCorrelation: true,
		MaxQueriesTracked: 10000,
		CorrelationAttributes: CorrelationAttributesConfig{
			AddQueryCategory:         true,
			AddTableStats:           true,
			AddLoadContribution:     true,
			AddMaintenanceIndicators: true,
		},
	}
}

// createMetricsProcessor creates a metrics processor
func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
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

	correlator := &queryCorrelator{
		config:        processorConfig,
		logger:        set.Logger,
		nextConsumer:  nextConsumer,
		queryIndex:    make(map[string]*queryInfo),
		tableIndex:    make(map[string]*tableInfo),
		databaseIndex: make(map[string]*databaseInfo),
	}

	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		correlator.ConsumeMetrics,
		processorhelper.WithCapabilities(correlator.Capabilities()),
		processorhelper.WithStart(correlator.Start),
		processorhelper.WithShutdown(correlator.Shutdown),
	)
}