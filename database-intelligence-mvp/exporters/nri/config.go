package nri

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/config/configretry"
)

// Config represents the NRI exporter configuration
type Config struct {
	// Integration name for NRI format
	IntegrationName string `mapstructure:"integration_name"`
	
	// Integration version
	IntegrationVersion string `mapstructure:"integration_version"`
	
	// Protocol version (1 or 2)
	ProtocolVersion int `mapstructure:"protocol_version"`
	
	// Entity configuration
	Entity EntityConfig `mapstructure:"entity"`
	
	// Output settings
	Output OutputConfig `mapstructure:"output"`
	
	// Metric transformation rules
	MetricRules []MetricRule `mapstructure:"metric_rules"`
	
	// Event transformation rules
	EventRules []EventRule `mapstructure:"event_rules"`
	
	// Retry configuration
	BackOffConfig configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	
	// Timeout for export operations
	Timeout time.Duration `mapstructure:"timeout"`
}

// EntityConfig defines how to construct NRI entities
type EntityConfig struct {
	// Entity type (e.g., "PostgreSQLInstance", "MySQLInstance")
	Type string `mapstructure:"type"`
	
	// How to determine entity name
	NameSource string `mapstructure:"name_source"`
	
	// Additional entity attributes
	Attributes map[string]string `mapstructure:"attributes"`
	
	// Display name template
	DisplayNameTemplate string `mapstructure:"display_name_template"`
}

// OutputConfig defines where to send NRI data
type OutputConfig struct {
	// Output mode: "stdout", "file", "http"
	Mode string `mapstructure:"mode"`
	
	// File path (for file mode)
	FilePath string `mapstructure:"file_path"`
	
	// HTTP endpoint (for http mode)
	HTTPEndpoint string `mapstructure:"http_endpoint"`
	
	// New Relic API key (for http mode)
	APIKey string `mapstructure:"api_key"`
	
	// Batch settings
	BatchSize int           `mapstructure:"batch_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval"`
}

// MetricRule defines how to transform OTLP metrics to NRI format
type MetricRule struct {
	// Source metric name pattern (supports wildcards)
	SourcePattern string `mapstructure:"source_pattern"`
	
	// Target metric name (can use placeholders)
	TargetName string `mapstructure:"target_name"`
	
	// Metric type in NRI (GAUGE, RATE, DELTA)
	NRIType string `mapstructure:"nri_type"`
	
	// Attribute mappings
	AttributeMappings map[string]string `mapstructure:"attribute_mappings"`
	
	// Attributes to include/exclude
	IncludeAttributes []string `mapstructure:"include_attributes"`
	ExcludeAttributes []string `mapstructure:"exclude_attributes"`
	
	// Value transformations
	ScaleFactor float64 `mapstructure:"scale_factor"`
	Unit        string  `mapstructure:"unit"`
}

// EventRule defines how to transform OTLP logs to NRI events
type EventRule struct {
	// Source log pattern
	SourcePattern string `mapstructure:"source_pattern"`
	
	// Event type in NRI
	EventType string `mapstructure:"event_type"`
	
	// Category
	Category string `mapstructure:"category"`
	
	// Summary template
	SummaryTemplate string `mapstructure:"summary_template"`
	
	// Attribute mappings
	AttributeMappings map[string]string `mapstructure:"attribute_mappings"`
}

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if cfg.IntegrationName == "" {
		return errors.New("integration_name is required")
	}
	
	if cfg.IntegrationVersion == "" {
		cfg.IntegrationVersion = "1.0.0"
	}
	
	if cfg.ProtocolVersion != 1 && cfg.ProtocolVersion != 2 {
		return errors.New("protocol_version must be 1 or 2")
	}
	
	// Validate entity configuration
	if cfg.Entity.Type == "" {
		return errors.New("entity.type is required")
	}
	
	// Validate output configuration
	switch cfg.Output.Mode {
	case "stdout", "file", "http":
		// Valid modes
	default:
		return errors.New("output.mode must be one of: stdout, file, http")
	}
	
	if cfg.Output.Mode == "file" && cfg.Output.FilePath == "" {
		return errors.New("output.file_path is required when mode is 'file'")
	}
	
	if cfg.Output.Mode == "http" && cfg.Output.HTTPEndpoint == "" {
		return errors.New("output.http_endpoint is required when mode is 'http'")
	}
	
	// Set defaults
	if cfg.Output.BatchSize <= 0 {
		cfg.Output.BatchSize = 100
	}
	
	if cfg.Output.FlushInterval <= 0 {
		cfg.Output.FlushInterval = 10 * time.Second
	}
	
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
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
	
	return nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		IntegrationName:    "com.newrelic.postgresql",
		IntegrationVersion: "1.0.0",
		ProtocolVersion:    2,
		Entity: EntityConfig{
			Type:       "PostgreSQLInstance",
			NameSource: "resource.db.system",
			Attributes: map[string]string{
				"provider": "opentelemetry",
			},
			DisplayNameTemplate: "{{.database_name}} ({{.host}}:{{.port}})",
		},
		Output: OutputConfig{
			Mode:          "stdout",
			BatchSize:     100,
			FlushInterval: 10 * time.Second,
		},
		MetricRules: []MetricRule{
			{
				SourcePattern: "db.query.*",
				TargetName:    "query.{{.metric_suffix}}",
				NRIType:       "GAUGE",
				AttributeMappings: map[string]string{
					"queryid":       "query_id",
					"database_name": "database",
				},
			},
			{
				SourcePattern: "postgresql.database.*",
				TargetName:    "db.{{.metric_suffix}}",
				NRIType:       "GAUGE",
			},
		},
		EventRules: []EventRule{
			{
				SourcePattern:   "db.slow_query",
				EventType:       "PostgreSQLSlowQuery",
				Category:        "Performance",
				SummaryTemplate: "Slow query detected: {{.query_text}}",
			},
		},
		BackOffConfig: configretry.BackOffConfig{
			Enabled:         true,
			InitialInterval: 5 * time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  5 * time.Minute,
		},
		Timeout: 30 * time.Second,
	}
}