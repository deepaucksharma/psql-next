// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package verification

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// typeStr is the type string for the verification processor
	typeStr = "verification"
	// stability is the stability level of the processor
	stability = component.StabilityLevelBeta
)

var componentType = component.MustNewType(typeStr)

// GetType returns the type of this processor
func GetType() component.Type {
	return componentType
}

// NewFactory creates a new processor factory for the verification processor
func NewFactory() processor.Factory {
	return processor.NewFactory(
		componentType,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, stability),
	)
}

// createLogsProcessor creates a logs processor
func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	
	vCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: %T", cfg)
	}
	
	if err := vCfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	
	// Create concurrent version for better performance
	vp, err := NewConcurrentVerificationProcessor(set.Logger, vCfg, nextConsumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create verification processor: %w", err)
	}
	
	return vp, nil
}