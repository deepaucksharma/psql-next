package kernelmetrics

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

const (
	// typeStr is the type of the receiver
	typeStr = "kernelmetrics"
	// stability is the stability level of the receiver
	stability = component.StabilityLevelAlpha
)

// NewFactory creates a new kernel metrics receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

// createDefaultConfig creates the default configuration for the receiver
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
	// Check platform support
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("kernel metrics receiver is only supported on Linux, current OS: %s", runtime.GOOS)
	}
	
	kmCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config type")
	}

	scraper, err := newScraper(kmCfg, settings)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&kmCfg.ControllerConfig,
		settings,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}