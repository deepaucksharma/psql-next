package ash

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr   = "ash"
	stability = component.StabilityLevelBeta
)

var errConfigNotASH = errors.New("config is not for ASH receiver")

// NewFactory creates a new ASH receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

// createDefaultConfig creates the default configuration for ASH receiver
func createDefaultConfig() component.Config {
	return DefaultConfig()
}

// createMetricsReceiver creates a metrics receiver based on provided config.
func createMetricsReceiver(
	ctx context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	ashCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotASH
	}

	// Validate the configuration
	if err := ashCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create the receiver
	r := &ashReceiver{
		config:       ashCfg,
		logger:       settings.Logger,
		consumer:     consumer,
		shutdownChan: make(chan struct{}),
	}

	// Return the receiver directly
	return r, nil
}