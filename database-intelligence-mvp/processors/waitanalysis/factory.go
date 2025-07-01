package waitanalysis

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

// NewFactory returns a new factory for the wait analysis processor
func NewFactory() processor.Factory {
	return processor.NewFactory(
		Type,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}