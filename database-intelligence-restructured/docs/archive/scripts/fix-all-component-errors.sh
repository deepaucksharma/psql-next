#!/bin/bash

echo "=== Fixing All Component Errors ==="

# 1. Fix queryselector DatabaseVersion reference
echo "1. Fixing queryselector DatabaseVersion..."
cd common/queryselector
sed -i.bak 's/features\.DatabaseVersion/features.ServerVersion/g' selector.go
cd ../..

# 2. Fix NRI exporter config.go
echo "2. Fixing NRI exporter config..."
cd exporters/nri

# Check current state of config.go
if grep -q "RateLimitConfig" config.go; then
    echo "Recreating clean config.go..."
    cat > config.go << 'EOF'
package nri

import (
	"errors"
	"time"
)

// Config represents the exporter config settings
type Config struct {
	// NRI Integration settings
	IntegrationName    string `mapstructure:"integration_name"`
	IntegrationVersion string `mapstructure:"integration_version"`
	ProtocolVersion    int    `mapstructure:"protocol_version"`
	
	// Output settings
	OutputPath string `mapstructure:"output_path"`
	OutputMode string `mapstructure:"output_mode"` // "file", "stdout", "http"
	
	// HTTP endpoint (if output_mode is "http")
	HTTPEndpoint string `mapstructure:"http_endpoint"`
	
	// Entity configuration
	Entity EntityConfig `mapstructure:"entity"`
	
	// Metric transformation rules
	MetricRules []MetricRule `mapstructure:"metric_rules"`
	
	// Event transformation rules
	EventRules []EventRule `mapstructure:"event_rules"`
	
	// General settings
	Timeout time.Duration `mapstructure:"timeout"`
}

// EntityConfig defines how to construct NRI entities
type EntityConfig struct {
	Type                string            `mapstructure:"type"`
	NameSource          string            `mapstructure:"name_source"`
	DisplayNameTemplate string            `mapstructure:"display_name_template"`
	Attributes          map[string]string `mapstructure:"attributes"`
}

// MetricRule defines how to transform OTel metrics to NRI format
type MetricRule struct {
	SourcePattern     string            `mapstructure:"source_pattern"`
	TargetName        string            `mapstructure:"target_name"`
	NRIType           string            `mapstructure:"nri_type"`
	ScaleFactor       float64           `mapstructure:"scale_factor"`
	AttributeMappings map[string]string `mapstructure:"attribute_mappings"`
	IncludeAttributes []string          `mapstructure:"include_attributes"`
	ExcludeAttributes []string          `mapstructure:"exclude_attributes"`
}

// EventRule defines how to transform OTel logs to NRI events
type EventRule struct {
	SourcePattern     string            `mapstructure:"source_pattern"`
	EventType         string            `mapstructure:"event_type"`
	Category          string            `mapstructure:"category"`
	SummaryTemplate   string            `mapstructure:"summary_template"`
	AttributeMappings map[string]string `mapstructure:"attribute_mappings"`
}

// Validate validates the configuration
func (cfg *Config) Validate() error {
	if cfg.IntegrationName == "" {
		return errors.New("integration_name is required")
	}
	
	if cfg.ProtocolVersion < 1 || cfg.ProtocolVersion > 4 {
		return errors.New("protocol_version must be between 1 and 4")
	}
	
	switch cfg.OutputMode {
	case "file":
		if cfg.OutputPath == "" {
			return errors.New("output_path is required when output_mode is 'file'")
		}
	case "http":
		if cfg.HTTPEndpoint == "" {
			return errors.New("http_endpoint is required when output_mode is 'http'")
		}
	case "stdout":
		// No additional validation needed
	default:
		return errors.New("output_mode must be one of: file, stdout, http")
	}
	
	// Validate entity config
	if cfg.Entity.Type == "" {
		return errors.New("entity.type is required")
	}
	
	return nil
}
EOF
fi

cd ../..

# 3. Fix kernelmetrics config
echo "3. Fixing kernelmetrics config..."
cd receivers/kernelmetrics

# Add missing config fields
if ! grep -q "CollectionInterval" config.go; then
    echo "Adding missing config fields..."
    cat >> config.go << 'EOF'

// CollectionInterval is the interval at which metrics are collected
type Config struct {
	CollectionInterval time.Duration        `mapstructure:"collection_interval"`
	CPUMetrics         CPUMetricsConfig     `mapstructure:"cpu_metrics"`
	MemoryMetrics      MemoryMetricsConfig  `mapstructure:"memory_metrics"`
	DiskMetrics        DiskMetricsConfig    `mapstructure:"disk_metrics"`
	NetworkMetrics     NetworkMetricsConfig `mapstructure:"network_metrics"`
	ProcessMetrics     ProcessMetricsConfig `mapstructure:"process_metrics"`
}

// CPUMetricsConfig configures CPU metrics collection
type CPUMetricsConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	DetailedMetrics bool `mapstructure:"detailed_metrics"`
}

// MemoryMetricsConfig configures memory metrics collection
type MemoryMetricsConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	DetailedMetrics bool `mapstructure:"detailed_metrics"`
}

// DiskMetricsConfig configures disk metrics collection
type DiskMetricsConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	DetailedMetrics bool `mapstructure:"detailed_metrics"`
}

// NetworkMetricsConfig configures network metrics collection
type NetworkMetricsConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	DetailedMetrics bool `mapstructure:"detailed_metrics"`
}

// ProcessMetricsConfig configures process metrics collection
type ProcessMetricsConfig struct {
	Enabled         bool `mapstructure:"enabled"`
	TopProcessCount int  `mapstructure:"top_process_count"`
}
EOF
fi

cd ../..

# 4. Fix ASH receiver
echo "4. Fixing ASH receiver..."
cd receivers/ash

# Add missing constants and fix config
if ! grep -q "defaultSessionsQuery" config.go; then
    echo "Adding default queries to config.go..."
    cat >> config.go << 'EOF'

const (
	defaultSessionsQuery = `SELECT * FROM v$session WHERE status = 'ACTIVE'`
	defaultHistoryQuery  = `SELECT * FROM v$active_session_history WHERE sample_time > :1`
)

// Config with additional fields
type ConfigWithInterval struct {
	Config
	CollectionInterval time.Duration     `mapstructure:"collection_interval"`
	Queries            map[string]string `mapstructure:"queries"`
}
EOF
fi

cd ../..

# 5. Fix adaptivesampler
echo "5. Fixing adaptivesampler..."
cd processors/adaptivesampler

# Remove the duplicate/conflicting metrics.go
if [ -f metrics.go ]; then
    echo "Removing conflicting metrics.go..."
    rm -f metrics.go
fi

# Add missing MaxSamplesPerSecond to config
if ! grep -q "MaxSamplesPerSecond" config.go; then
    sed -i.bak '/SamplingPercentage.*float64/a\
	MaxSamplesPerSecond     int                       `mapstructure:"max_samples_per_second"`' config.go
fi

# Fix factory.go to use the correct function
sed -i.bak 's/newAdaptiveSamplerMetrics(/newAdaptiveSamplerProcessor(/g' factory.go

# Add the missing processor constructor function to processor.go
if ! grep -q "newAdaptiveSamplerProcessor" processor.go; then
    cat >> processor.go << 'EOF'

// newAdaptiveSamplerProcessor creates a new adaptive sampler processor
func newAdaptiveSamplerProcessor(cfg *Config, set component.TelemetrySettings, nextConsumer consumer.Traces) processor.Traces {
	return &adaptiveSamplerProcessor{
		config:       cfg,
		logger:       set.Logger,
		nextConsumer: nextConsumer,
		algorithm:    newAdaptiveAlgorithm(cfg, set.Logger),
	}
}
EOF
fi

cd ../..

# 6. Now try building again
echo ""
echo "=== Testing build with fixes ==="
cd distributions/production

export GOWORK=off
rm -f go.sum

if go mod tidy && go build -o otelcol-complete .; then
    echo "✓ Build successful!"
    ls -lh otelcol-complete
    
    echo ""
    echo "Components available:"
    ./otelcol-complete components 2>&1 | grep -A 20 "receivers:" | head -40
else
    echo "✗ Build still failing"
    echo "Remaining errors:"
    go build . 2>&1 | grep -E "^#|error:" | head -20
fi

cd ../..

echo ""
echo "=== Fix attempt complete ===">