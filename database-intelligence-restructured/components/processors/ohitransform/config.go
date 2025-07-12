package ohitransform

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the OHI transform processor.
type Config struct {
	// TransformRules defines the rules for transforming OTel metrics to OHI events
	TransformRules []TransformRule `mapstructure:"transform_rules"`
	
	// EnableMetricToEvent converts metrics to event-like structure for OHI compatibility
	EnableMetricToEvent bool `mapstructure:"enable_metric_to_event"`
	
	// PreserveOriginalMetrics keeps original metrics alongside transformed ones
	PreserveOriginalMetrics bool `mapstructure:"preserve_original_metrics"`
}

// TransformRule defines a single transformation from OTel metric to OHI event
type TransformRule struct {
	// SourceMetric is the OTel metric name to transform
	SourceMetric string `mapstructure:"source_metric"`
	
	// TargetEvent is the OHI event name to create
	TargetEvent string `mapstructure:"target_event"`
	
	// Mappings defines attribute name mappings from OTel to OHI
	Mappings map[string]string `mapstructure:"mappings"`
	
	// Filters can be used to only transform metrics matching certain criteria
	Filters map[string]string `mapstructure:"filters"`
	
	// AggregationMethod for multi-value metrics (e.g., "latest", "sum", "avg")
	AggregationMethod string `mapstructure:"aggregation_method"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.TransformRules) == 0 {
		return fmt.Errorf("at least one transform rule must be specified")
	}
	
	for i, rule := range cfg.TransformRules {
		if rule.SourceMetric == "" {
			return fmt.Errorf("transform rule %d: source_metric cannot be empty", i)
		}
		if rule.TargetEvent == "" {
			return fmt.Errorf("transform rule %d: target_event cannot be empty", i)
		}
		if len(rule.Mappings) == 0 {
			return fmt.Errorf("transform rule %d: at least one mapping must be specified", i)
		}
		if rule.AggregationMethod == "" {
			cfg.TransformRules[i].AggregationMethod = "latest"
		}
	}
	
	return nil
}