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
	
	if cfg.IntegrationVersion == "" {
		cfg.IntegrationVersion = "1.0.0"
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
			return errors.New("metric_rules source_pattern is required")
		}
		if rule.TargetName == "" {
			return errors.New("metric_rules target_name is required")
		}
		if rule.ScaleFactor == 0 {
			cfg.MetricRules[i].ScaleFactor = 1.0
		}
	}
	
	// Set default timeout
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	
	return nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
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