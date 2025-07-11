#!/bin/bash

echo "=== Fixing Compilation Errors ==="

# 1. Fix featuredetector - add missing DatabaseVersion field
echo "Fixing featuredetector..."
cd common/featuredetector

# Add DatabaseVersion to FeatureSet struct
sed -i.bak '/"Version".*string/a\
	DatabaseVersion string   `json:"database_version"`' types.go

cd ../..

# 2. Fix verification processor - remove unused import
echo "Fixing verification processor..."
cd processors/verification
sed -i.bak '/^import (/,/^)/{/crypto\/md5/d;}' processor.go
cd ../..

# 3. Fix NRI exporter config syntax errors
echo "Fixing NRI exporter config..."
cd exporters/nri

# The config.go file has syntax errors from incomplete rate limiter removal
# Let's check what's wrong
echo "Checking config.go syntax..."
go fmt config.go 2>&1 | head -10 || true

# Fix the malformed if statements in exporter.go
echo "Fixing exporter.go syntax..."
sed -i.bak 's|if // rateLimiter != nil {|if false /* rate limiter disabled */ {|g' exporter.go
sed -i.bak 's|if !// rateLimiter.Allow|if false /* !rateLimiter.Allow|g' exporter.go
sed -i.bak 's|// rateLimiter.GetMetrics()|nil /* rateLimiter.GetMetrics() */|g' exporter.go

cd ../..

# 4. Fix ASH receiver scraper API changes
echo "Fixing ASH receiver..."
cd receivers/ash

# Update factory.go for new scraper API
cat > factory_temp.go << 'EOF'
package ash

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
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
	return &Config{
		CollectionInterval: 60 * time.Second,
		Queries: map[string]string{
			"sessions": defaultSessionsQuery,
			"history":  defaultHistoryQuery,
		},
	}
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

	scrp, err := newASHScraper(ashCfg, settings)
	if err != nil {
		return nil, err
	}

	scraperOpts := []scraperhelper.ControllerOption{
		scraperhelper.WithCollectionInterval(ashCfg.CollectionInterval),
	}

	return scraperhelper.NewController(
		component.KindReceiver,
		component.MustNewType(typeStr),
		consumer,
		scraperhelper.WithScraper(component.MustNewType(typeStr), scrp.scrape),
		scraperOpts...,
	)
}

// newASHScraper creates a new ASH scraper
func newASHScraper(cfg *Config, settings receiver.Settings) (*ashScraper, error) {
	return &ashScraper{
		config:   cfg,
		settings: settings.TelemetrySettings,
	}, nil
}
EOF

mv factory_temp.go factory.go

cd ../..

# 5. Fix kernelmetrics receiver
echo "Fixing kernelmetrics receiver..."
cd receivers/kernelmetrics

# Update factory.go for new scraper API
cat > factory_temp.go << 'EOF'
package kernelmetrics

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
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
	return &Config{
		CollectionInterval: 10 * time.Second,
		CPUMetrics: CPUMetricsConfig{
			Enabled:         true,
			DetailedMetrics: true,
		},
		MemoryMetrics: MemoryMetricsConfig{
			Enabled:         true,
			DetailedMetrics: true,
		},
		DiskMetrics: DiskMetricsConfig{
			Enabled:         true,
			DetailedMetrics: true,
		},
		NetworkMetrics: NetworkMetricsConfig{
			Enabled:         true,
			DetailedMetrics: true,
		},
		ProcessMetrics: ProcessMetricsConfig{
			Enabled:         true,
			TopProcessCount: 10,
		},
	}
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

	scrp := &kmScraper{
		config:   kmCfg,
		settings: settings.TelemetrySettings,
	}

	scraperOpts := []scraperhelper.ControllerOption{
		scraperhelper.WithCollectionInterval(kmCfg.CollectionInterval),
	}

	return scraperhelper.NewController(
		component.KindReceiver,
		component.MustNewType(typeStr),
		consumer,
		scraperhelper.WithScraper(component.MustNewType(typeStr), scrp.scrape),
		scraperOpts...,
	)
}
EOF

mv factory_temp.go factory.go

# Remove unused imports from scraper.go
sed -i.bak '/^import (/,/^)/{/"fmt"/d;}' scraper.go
sed -i.bak '/^import (/,/^)/{/"go.opentelemetry.io\/collector\/scraper\/scrapererror"/d;}' scraper.go

cd ../..

# 6. Fix adaptive sampler processor
echo "Fixing adaptive sampler processor..."
cd processors/adaptivesampler

# Add missing field to Config struct
sed -i.bak '/SamplingPercentage.*float64/a\
	MaxSamplesPerSecond     int                       `mapstructure:"max_samples_per_second"`' config.go

# Add missing metrics function
cat >> metrics.go << 'EOF'

// newAdaptiveSamplerMetrics creates a new adaptive sampler metrics instance
func newAdaptiveSamplerMetrics(telemetry component.TelemetrySettings) (*adaptiveSamplerMetrics, error) {
	meter := telemetry.MeterProvider.Meter("github.com/database-intelligence/processors/adaptivesampler")
	
	sampledCount, err := meter.Int64Counter(
		"adaptive_sampler_sampled_count",
		metric.WithDescription("Number of spans sampled"),
	)
	if err != nil {
		return nil, err
	}
	
	droppedCount, err := meter.Int64Counter(
		"adaptive_sampler_dropped_count",
		metric.WithDescription("Number of spans dropped"),
	)
	if err != nil {
		return nil, err
	}
	
	return &adaptiveSamplerMetrics{
		sampledCount: sampledCount,
		droppedCount: droppedCount,
	}, nil
}
EOF

# Remove unused import
sed -i.bak '/^import (/,/^)/{/"go.opentelemetry.io\/collector\/pdata\/pmetric"/d;}' processor.go

cd ../..

# 7. Fix NRI config.go syntax errors more thoroughly
echo "Fixing NRI config syntax errors..."
cd exporters/nri

# Let's check the specific lines with errors
echo "Lines around 118 in config.go:"
sed -n '115,120p' config.go

echo ""
echo "Lines around 214 in config.go:"
sed -n '210,220p' config.go

echo ""
echo "Lines around 281 in config.go:"
sed -n '278,285p' config.go

cd ../..

echo ""
echo "=== Attempting to build again ==="
cd distributions/production
GOWORK=off go build -o otelcol-database-intelligence . 2>&1 | head -30

cd ../..

echo ""
echo "=== Fix attempt complete ==="