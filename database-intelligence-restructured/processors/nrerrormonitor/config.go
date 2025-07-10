package nrerrormonitor

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config configures the NrIntegrationError monitoring processor
type Config struct {
	// MaxAttributeLength is the maximum allowed length for attribute values
	MaxAttributeLength int `mapstructure:"max_attribute_length"`
	
	// MaxMetricNameLength is the maximum allowed length for metric names
	MaxMetricNameLength int `mapstructure:"max_metric_name_length"`
	
	// CardinalityWarningThreshold warns when a metric exceeds this cardinality
	CardinalityWarningThreshold int `mapstructure:"cardinality_warning_threshold"`
	
	// AlertThreshold triggers an alert after this many errors
	AlertThreshold int64 `mapstructure:"alert_threshold"`
	
	// ReportingInterval for summary metrics
	ReportingInterval time.Duration `mapstructure:"reporting_interval"`
	
	// ErrorSuppressionDuration suppresses duplicate error logs
	ErrorSuppressionDuration time.Duration `mapstructure:"error_suppression_duration"`
	
	// EnableProactiveValidation performs additional checks
	EnableProactiveValidation bool `mapstructure:"enable_proactive_validation"`
}

// Validate checks the processor configuration
func (cfg *Config) Validate() error {
	if cfg.MaxAttributeLength <= 0 {
		return fmt.Errorf("max_attribute_length must be positive")
	}
	
	if cfg.MaxMetricNameLength <= 0 {
		return fmt.Errorf("max_metric_name_length must be positive")
	}
	
	if cfg.CardinalityWarningThreshold <= 0 {
		return fmt.Errorf("cardinality_warning_threshold must be positive")
	}
	
	if cfg.AlertThreshold <= 0 {
		return fmt.Errorf("alert_threshold must be positive")
	}
	
	if cfg.ReportingInterval <= 0 {
		return fmt.Errorf("reporting_interval must be positive")
	}
	
	return nil
}

// createDefaultConfig creates the default configuration
func createDefaultConfig() component.Config {
	return &Config{
		MaxAttributeLength:          4096,  // New Relic limit
		MaxMetricNameLength:         255,   // New Relic limit
		CardinalityWarningThreshold: 10000, // Warn on high cardinality
		AlertThreshold:              100,   // Alert after 100 errors
		ReportingInterval:           60 * time.Second,
		ErrorSuppressionDuration:    5 * time.Minute,
		EnableProactiveValidation:   true,
	}
}