package enhancedsql

import (
	"context"
	"fmt"
	
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	// Type is the type of the receiver
	Type = "enhancedsql"
	// stability is the stability level of the receiver
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a new receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
		receiver.WithLogs(createLogsReceiver, stability),
	)
}

// createMetricsReceiver creates a metrics receiver
func createMetricsReceiver(
	ctx context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	receiverCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}
	
	if err := receiverCfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	receiver, err := NewReceiver(receiverCfg, set.Logger, consumer, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create receiver: %w", err)
	}
	
	return receiver, nil
}

// createLogsReceiver creates a logs receiver
func createLogsReceiver(
	ctx context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	receiverCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}
	
	if err := receiverCfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	receiver, err := NewReceiver(receiverCfg, set.Logger, nil, consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create receiver: %w", err)
	}
	
	return receiver, nil
}