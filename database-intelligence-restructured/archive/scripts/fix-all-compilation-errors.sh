#!/bin/bash

echo "=== Comprehensive Fix for All Compilation Errors ==="

# 1. Fix featuredetector DatabaseVersion field
echo "1. Fixing featuredetector DatabaseVersion..."
cd common/featuredetector

# Check current FeatureSet struct
echo "Current FeatureSet struct:"
grep -A 20 "type FeatureSet struct" types.go | grep -B 20 "^}"

# Add DatabaseVersion field if not present
if ! grep -q "DatabaseVersion" types.go; then
    echo "Adding DatabaseVersion field..."
    sed -i.bak '/Version.*string.*`json:"version"`/a\
	DatabaseVersion string   `json:"database_version"`' types.go
else
    echo "DatabaseVersion field already exists"
fi

cd ../..

# 2. Fix NRI config.go syntax errors properly
echo ""
echo "2. Fixing NRI config.go syntax errors..."
cd exporters/nri

# Backup current broken file
cp config.go config.go.broken

# Check if the clean version was already created
if [ -f config.go ] && grep -q "type RateLimitConfig struct" config.go; then
    echo "Config still has RateLimitConfig, using clean version..."
    # Use the clean version from previous fix
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

// createDefaultConfig creates the default configuration
func createDefaultConfig() *Config {
	return &Config{
		IntegrationName:    "com.newrelic.database-intelligence",
		IntegrationVersion: "1.0.0",
		ProtocolVersion:    4,
		OutputMode:         "stdout",
		Entity: EntityConfig{
			Type:       "DATABASE",
			NameSource: "db.system",
			Attributes: map[string]string{
				"provider": "database-intelligence",
			},
		},
		MetricRules: []MetricRule{
			{
				SourcePattern: "db.*",
				TargetName:    "db.{{.metric_suffix}}",
				NRIType:       "GAUGE",
				ScaleFactor:   1.0,
			},
		},
		EventRules: []EventRule{
			{
				SourcePattern:   "db.error.*",
				EventType:       "DatabaseError",
				Category:        "ERROR",
				SummaryTemplate: "Database error: {{.error_message}}",
			},
		},
		Timeout: 30 * time.Second,
	}
}
EOF
fi

cd ../..

# 3. Fix kernelmetrics config
echo ""
echo "3. Fixing kernelmetrics config..."
cd receivers/kernelmetrics

# Check if config.go has the required types
if ! grep -q "type CPUMetricsConfig struct" config.go; then
    echo "Adding missing config types to config.go..."
    cat >> config.go << 'EOF'

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

# Check if CollectionInterval is in Config struct
if ! grep -q "CollectionInterval" config.go; then
    echo "Adding CollectionInterval to Config struct..."
    sed -i.bak '/type Config struct {/a\
	CollectionInterval time.Duration     `mapstructure:"collection_interval"`\
	CPUMetrics         CPUMetricsConfig     `mapstructure:"cpu_metrics"`\
	MemoryMetrics      MemoryMetricsConfig  `mapstructure:"memory_metrics"`\
	DiskMetrics        DiskMetricsConfig    `mapstructure:"disk_metrics"`\
	NetworkMetrics     NetworkMetricsConfig `mapstructure:"network_metrics"`\
	ProcessMetrics     ProcessMetricsConfig `mapstructure:"process_metrics"`' config.go
fi

cd ../..

# 4. Fix adaptive sampler
echo ""
echo "4. Fixing adaptive sampler..."
cd processors/adaptivesampler

# Fix config.go - add MaxSamplesPerSecond if missing
if ! grep -q "MaxSamplesPerSecond" config.go; then
    echo "Adding MaxSamplesPerSecond to config..."
    sed -i.bak '/SamplingPercentage.*float64/a\
	MaxSamplesPerSecond     int                       `mapstructure:"max_samples_per_second"`' config.go
fi

# Fix the metrics struct
echo "Fixing metrics struct..."
cat > metrics_fixed.go << 'EOF'
package adaptivesampler

import (
	"context"
	"sync/atomic"
	
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

// adaptiveSamplerMetrics implements the processor.Metrics interface
type adaptiveSamplerMetrics struct {
	config         *Config
	logger         *zap.Logger
	nextConsumer   consumer.Traces
	algorithm      *adaptiveAlgorithm
	sampledCount   int64
	droppedCount   int64
}

// Capabilities returns the capabilities of the processor
func (asm *adaptiveSamplerMetrics) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start starts the processor
func (asm *adaptiveSamplerMetrics) Start(ctx context.Context, host component.Host) error {
	return nil
}

// Shutdown shuts down the processor
func (asm *adaptiveSamplerMetrics) Shutdown(ctx context.Context) error {
	return nil
}

// ConsumeTraces implements the consumer.Traces interface
func (asm *adaptiveSamplerMetrics) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Process traces through adaptive sampling
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		sss := rs.ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			ss := sss.At(j)
			spans := ss.Spans()
			
			// Keep track of spans to remove
			var toRemove []int
			
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if !asm.algorithm.shouldSample(span) {
					toRemove = append(toRemove, k)
					atomic.AddInt64(&asm.droppedCount, 1)
				} else {
					atomic.AddInt64(&asm.sampledCount, 1)
				}
			}
			
			// Remove spans in reverse order
			for idx := len(toRemove) - 1; idx >= 0; idx-- {
				spans.RemoveAt(toRemove[idx])
			}
		}
	}
	
	// Forward to next consumer
	return asm.nextConsumer.ConsumeTraces(ctx, td)
}

// newAdaptiveSamplerProcessor creates a new adaptive sampler processor
func newAdaptiveSamplerProcessor(cfg *Config, logger *zap.Logger, nextConsumer consumer.Traces) processor.Traces {
	algorithm := newAdaptiveAlgorithm(cfg, logger)
	
	return &adaptiveSamplerMetrics{
		config:       cfg,
		logger:       logger,
		nextConsumer: nextConsumer,
		algorithm:    algorithm,
	}
}
EOF

# Update factory.go to use the correct function
sed -i.bak 's/newAdaptiveSamplerMetrics(set.TelemetrySettings)/newAdaptiveSamplerProcessor(cfg, set.Logger, nextConsumer)/g' factory.go

# Remove the old metrics.go and rename the fixed one
rm -f metrics.go
mv metrics_fixed.go metrics.go

cd ../..

# 5. Build again
echo ""
echo "=== Attempting to build production collector ==="
cd distributions/production

if GOWORK=off go build -o otelcol-database-intelligence .; then
    echo "✓ Build successful!"
    ls -la otelcol-database-intelligence
    
    echo ""
    echo "Testing binary..."
    ./otelcol-database-intelligence --version || true
else
    echo "⚠ Build still failing. Remaining errors:"
    GOWORK=off go build . 2>&1 | grep -E "^#|:.*:" | head -30
fi

cd ../..

echo ""
echo "=== Fix attempt complete ==="