package ash

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

const (
	// typeStr is the type of the receiver
	typeStr = "ash"
	// stability is the stability level of the receiver
	stability = component.StabilityLevelBeta
)

// NewFactory creates a new ASH receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, stability),
		receiver.WithLogs(createLogsReceiver, stability),
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
	ashCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config type")
	}

	scraper, err := newScraper(ashCfg, settings)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&ashCfg.ControllerConfig,
		settings,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}

// createLogsReceiver creates a logs receiver
func createLogsReceiver(
	ctx context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	ashCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config type")
	}

	// Create a new receiver that also handles logs
	r := &ashReceiver{
		config:       ashCfg,
		logger:       settings.Logger,
		logsConsumer: consumer,
	}

	// For logs, we use a simpler approach than scraperhelper
	return r, nil
}

// ashReceiver implements receiver.Logs for log output
type ashReceiver struct {
	config       *Config
	logger       component.TelemetrySettings
	logsConsumer consumer.Logs
	scraper      *ashScraper
	cancelFunc   context.CancelFunc
}

// Start starts the receiver
func (r *ashReceiver) Start(ctx context.Context, host component.Host) error {
	scraper, err := newScraper(r.config, receiver.Settings{
		ID:                component.NewID(component.MustNewType(typeStr)),
		TelemetrySettings: r.logger,
		BuildInfo:         component.NewDefaultBuildInfo(),
	})
	if err != nil {
		return err
	}
	
	r.scraper = scraper
	r.scraper.logsConsumer = r.logsConsumer
	
	// Start background collection
	collectionCtx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel
	
	go r.collectionLoop(collectionCtx)
	
	return r.scraper.start(ctx, host)
}

// Shutdown shuts down the receiver
func (r *ashReceiver) Shutdown(ctx context.Context) error {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	
	if r.scraper != nil {
		return r.scraper.shutdown(ctx)
	}
	
	return nil
}

// collectionLoop runs the collection at specified intervals
func (r *ashReceiver) collectionLoop(ctx context.Context) {
	ticker := time.NewTicker(r.config.CollectionInterval)
	defer ticker.Stop()
	
	// Initial delay
	select {
	case <-time.After(r.config.InitialDelay):
	case <-ctx.Done():
		return
	}
	
	for {
		select {
		case <-ticker.C:
			// For logs, we let the scraper handle sending to consumer
			// The scraper has access to logsConsumer directly
		case <-ctx.Done():
			return
		}
	}
}