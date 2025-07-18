package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	// Type is the value of "type" key in configuration.
	typeStr = "mongodb"
	// Stability level
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for MongoDB receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability))
}

func createMetricsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer component.MetricsConsumer,
) (receiver.Metrics, error) {
	c := cfg.(*Config)
	s := newScraper(params, c)

	scraper, err := scraperhelper.NewScraper(
		typeStr,
		s.scrape,
		scraperhelper.WithStart(s.start),
		scraperhelper.WithShutdown(s.shutdown),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create scraper: %w", err)
	}

	opt := scraperhelper.AddScraper(scraper)
	return scraperhelper.NewScraperControllerReceiver(
		&c.CollectionInterval,
		params,
		consumer,
		opt,
	)
}