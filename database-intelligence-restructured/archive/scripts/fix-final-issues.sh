#!/bin/bash

echo "=== Fixing Final Compilation Issues ==="

# 1. Fix NRI config.go - it has incomplete rate limiter removal
echo "Fixing NRI config.go..."
cd exporters/nri

# Let's see what's in config.go around the problematic areas
echo "Checking config.go structure..."
grep -n "type.*struct" config.go | head -5

# Check if RateLimitConfig type exists
if grep -q "type RateLimitConfig struct" config.go; then
    echo "Found RateLimitConfig, cleaning up..."
    # Remove the entire RateLimitConfig struct and related code
    # This is complex, so let's create a clean version
    cp config.go config.go.broken
    
    # Extract the clean parts and recreate
    cat > config_clean.go << 'EOF'
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
	
	// Validate metric rules
	for i, rule := range cfg.MetricRules {
		if rule.SourcePattern == "" {
			return errors.New("metric_rules[" + string(rune(i)) + "].source_pattern is required")
		}
		if rule.TargetName == "" {
			return errors.New("metric_rules[" + string(rune(i)) + "].target_name is required")
		}
		if rule.ScaleFactor == 0 {
			cfg.MetricRules[i].ScaleFactor = 1.0
		}
	}
	
	// Validate event rules
	for i, rule := range cfg.EventRules {
		if rule.SourcePattern == "" {
			return errors.New("event_rules[" + string(rune(i)) + "].source_pattern is required")
		}
		if rule.EventType == "" {
			return errors.New("event_rules[" + string(rune(i)) + "].event_type is required")
		}
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

    mv config_clean.go config.go
fi

cd ../..

# 2. Fix adaptive sampler metrics.go - it's missing package declaration
echo "Fixing adaptive sampler metrics.go..."
cd processors/adaptivesampler

# Check if metrics.go exists and starts with func
if [ -f metrics.go ] && head -1 metrics.go | grep -q "^func"; then
    echo "Fixing metrics.go package declaration..."
    # Add package declaration at the beginning
    echo -e "package adaptivesampler\n\nimport (\n\t\"go.opentelemetry.io/collector/component\"\n\t\"go.opentelemetry.io/otel/metric\"\n)\n" > metrics_temp.go
    cat metrics.go >> metrics_temp.go
    mv metrics_temp.go metrics.go
fi

cd ../..

# 3. Fix the malformed comment in NRI exporter
echo "Fixing NRI exporter comments..."
cd exporters/nri
sed -i.bak 's|if false /\* !rateLimiter.Allow|if false /\* !rateLimiter.Allow \*/|g' exporter.go
cd ../..

# 4. Build again
echo ""
echo "=== Building production collector ==="
cd distributions/production
if GOWORK=off go build -o otelcol-database-intelligence .; then
    echo "✓ Build successful!"
    ls -la otelcol-database-intelligence
    
    echo ""
    echo "Testing binary..."
    ./otelcol-database-intelligence --version
    
    echo ""
    echo "Available components:"
    ./otelcol-database-intelligence components 2>&1 | grep -E "(receivers:|processors:|exporters:)" -A 20
else
    echo "⚠ Build still failing. Checking remaining errors..."
    GOWORK=off go build . 2>&1 | grep -E "^#|error:" | head -20
fi

cd ../..

echo ""
echo "=== Fix complete ==="