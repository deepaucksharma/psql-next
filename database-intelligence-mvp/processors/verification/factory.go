// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package verification

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// typeStr is the type string for the verification processor
	typeStr = "verification"
	// stability is the stability level of the processor
	stability = component.StabilityLevelBeta
)

// NewFactory creates a new processor factory for the verification processor
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithLogs(createLogsProcessor, stability),
	)
}

// createLogsProcessor creates a logs processor
func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
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
	
	vp, err := newVerificationProcessor(set.Logger, vCfg, nextConsumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create verification processor: %w", err)
	}
	
	return processorhelper.NewLogsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		vp.ConsumeLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		processorhelper.WithStart(vp.Start),
		processorhelper.WithShutdown(vp.Shutdown),
	)
}