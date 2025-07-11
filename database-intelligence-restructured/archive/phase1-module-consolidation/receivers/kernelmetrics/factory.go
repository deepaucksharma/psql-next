package kernelmetrics

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr   = "kernelmetrics"
	stability = component.StabilityLevelBeta
)

var errConfigNotKM = errors.New("config is not for kernel metrics receiver")

// NewFactory creates a new kernel metrics receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

// createDefaultConfig creates the default configuration
func createDefaultConfig() component.Config {
	return DefaultConfig()
}

// createMetricsReceiver creates a metrics receiver
func createMetricsReceiver(
	ctx context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	kmCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotKM
	}

	// Validate the configuration
	if err := kmCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create the receiver
	r := &kernelMetricsReceiver{
		config:       kmCfg,
		logger:       settings.Logger,
		consumer:     consumer,
		shutdownChan: make(chan struct{}),
	}

	// Return the receiver directly since it implements receiver.Metrics
	return r, nil
}