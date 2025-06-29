package adaptivesampler

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// EnhancedConfig represents the enhanced configuration for the adaptive sampler processor
type EnhancedConfig struct {
	// InMemoryOnly forces in-memory-only operation (no persistence)
	InMemoryOnly bool `mapstructure:"in_memory_only"`

	// Deduplication settings
	Deduplication DeduplicationConfig `mapstructure:"deduplication"`

	// SamplingRules define the sampling strategy
	SamplingRules []SamplingRule `mapstructure:"rules"`

	// RuleTemplates for dynamic rule generation
	RuleTemplates map[string]RuleTemplate `mapstructure:"rule_templates"`

	// DefaultSampleRate is used when no rules match
	DefaultSampleRate float64 `mapstructure:"default_sample_rate"`

	// MaxRecordsPerSecond limits throughput for safety
	MaxRecordsPerSecond int `mapstructure:"max_records_per_second"`

	// SlowQueryThresholdMs configurable threshold for slow queries
	SlowQueryThresholdMs float64 `mapstructure:"slow_query_threshold_ms"`

	// EnvironmentOverrides for per-environment configuration
	EnvironmentOverrides map[string]EnvironmentConfig `mapstructure:"environment_overrides"`

	// EnableDebugLogging enables detailed debug output
	EnableDebugLogging bool `mapstructure:"enable_debug_logging"`

	// MetricsConfig for processor telemetry
	MetricsConfig ProcessorMetricsConfig `mapstructure:"metrics"`
}

// RuleTemplate defines a template for dynamic rule generation
type RuleTemplate struct {
	Name              string                       `mapstructure:"name"`
	Priority          int                          `mapstructure:"priority"`
	SampleRate        float64                      `mapstructure:"sample_rate"`
	ConditionTemplate []SamplingConditionTemplate  `mapstructure:"condition_template"`
	MaxPerMinute      int                          `mapstructure:"max_per_minute,omitempty"`
	ThresholdMs       float64                      `mapstructure:"threshold_ms,omitempty"`
	ExecutionsPerMin  int                          `mapstructure:"executions_per_minute,omitempty"`
}

// SamplingConditionTemplate for dynamic condition generation
type SamplingConditionTemplate struct {
	Attribute      string `mapstructure:"attribute"`
	Operator       string `mapstructure:"operator"`
	ValueTemplate  string `mapstructure:"value_template"` // Can reference env vars
}

// EnvironmentConfig for per-environment overrides
type EnvironmentConfig struct {
	SlowQueryThresholdMs float64            `mapstructure:"slow_query_threshold_ms"`
	MaxRecordsPerSecond  int                `mapstructure:"max_records_per_second"`
	DefaultSampleRate    float64            `mapstructure:"default_sample_rate"`
	RuleOverrides        map[string]float64 `mapstructure:"rule_overrides"` // rule_name -> sample_rate
}

// ProcessorMetricsConfig for telemetry configuration
type ProcessorMetricsConfig struct {
	Enabled              bool          `mapstructure:"enabled"`
	Interval             time.Duration `mapstructure:"interval"`
	IncludeRuleMetrics   bool          `mapstructure:"include_rule_metrics"`
	IncludeCacheMetrics  bool          `mapstructure:"include_cache_metrics"`
	ExportToNewRelic     bool          `mapstructure:"export_to_new_relic"`
}

// createEnhancedDefaultConfig creates an enhanced default configuration
func createEnhancedDefaultConfig() component.Config {
	return &EnhancedConfig{
		InMemoryOnly: true,
		Deduplication: DeduplicationConfig{
			Enabled:         true,
			CacheSize:       10000,
			WindowSeconds:   300,
			HashAttribute:   "db.query.plan.hash",
			CleanupInterval: 60 * time.Second,
		},
		RuleTemplates: map[string]RuleTemplate{
			"critical_queries": {
				Name:       "critical_queries",
				Priority:   100,
				SampleRate: 1.0,
				ThresholdMs: 1000.0, // Default, overridable
				ConditionTemplate: []SamplingConditionTemplate{
					{
						Attribute:     "avg_duration_ms",
						Operator:      "gt",
						ValueTemplate: "${CRITICAL_QUERY_MS:1000}",
					},
				},
			},
			"high_volume_queries": {
				Name:             "high_volume_queries",
				Priority:         50,
				SampleRate:       0.01,
				ExecutionsPerMin: 1000, // Default, overridable
				MaxPerMinute:     10,
				ConditionTemplate: []SamplingConditionTemplate{
					{
						Attribute:     "execution_count",
						Operator:      "gt",
						ValueTemplate: "${HIGH_VOLUME_THRESHOLD:1000}",
					},
				},
			},
		},
		SlowQueryThresholdMs: 1000.0,
		DefaultSampleRate:    0.1,
		MaxRecordsPerSecond:  1000,
		EnvironmentOverrides: map[string]EnvironmentConfig{
			"production": {
				SlowQueryThresholdMs: 2000,
				MaxRecordsPerSecond:  500,
				DefaultSampleRate:    0.05,
				RuleOverrides: map[string]float64{
					"high_volume_queries": 0.001,
				},
			},
			"staging": {
				SlowQueryThresholdMs: 500,
				MaxRecordsPerSecond:  2000,
				DefaultSampleRate:    0.5,
			},
		},
		EnableDebugLogging: false,
		MetricsConfig: ProcessorMetricsConfig{
			Enabled:             true,
			Interval:            30 * time.Second,
			IncludeRuleMetrics:  true,
			IncludeCacheMetrics: true,
			ExportToNewRelic:    true,
		},
	}
}

// ApplyEnvironmentOverrides applies environment-specific configuration
func (cfg *EnhancedConfig) ApplyEnvironmentOverrides(environment string) {
	if envConfig, exists := cfg.EnvironmentOverrides[environment]; exists {
		if envConfig.SlowQueryThresholdMs > 0 {
			cfg.SlowQueryThresholdMs = envConfig.SlowQueryThresholdMs
		}
		if envConfig.MaxRecordsPerSecond > 0 {
			cfg.MaxRecordsPerSecond = envConfig.MaxRecordsPerSecond
		}
		if envConfig.DefaultSampleRate > 0 {
			cfg.DefaultSampleRate = envConfig.DefaultSampleRate
		}
		
		// Apply rule overrides
		for ruleName, sampleRate := range envConfig.RuleOverrides {
			for i := range cfg.SamplingRules {
				if cfg.SamplingRules[i].Name == ruleName {
					cfg.SamplingRules[i].SampleRate = sampleRate
					break
				}
			}
		}
	}
}

// GenerateRulesFromTemplates creates rules from templates with environment substitution
func (cfg *EnhancedConfig) GenerateRulesFromTemplates() error {
	generatedRules := make([]SamplingRule, 0, len(cfg.RuleTemplates))
	
	for _, template := range cfg.RuleTemplates {
		rule := SamplingRule{
			Name:         template.Name,
			Priority:     template.Priority,
			SampleRate:   template.SampleRate,
			MaxPerMinute: template.MaxPerMinute,
		}
		
		// Generate conditions from templates
		conditions := make([]SamplingCondition, 0, len(template.ConditionTemplate))
		for _, condTemplate := range template.ConditionTemplate {
			// In production, this would do actual env var substitution
			// For now, we'll use the template values directly
			value := condTemplate.ValueTemplate
			if template.ThresholdMs > 0 && condTemplate.Attribute == "avg_duration_ms" {
				value = template.ThresholdMs
			} else if template.ExecutionsPerMin > 0 && condTemplate.Attribute == "execution_count" {
				value = float64(template.ExecutionsPerMin)
			}
			
			conditions = append(conditions, SamplingCondition{
				Attribute: condTemplate.Attribute,
				Operator:  condTemplate.Operator,
				Value:     value,
			})
		}
		
		rule.Conditions = conditions
		generatedRules = append(generatedRules, rule)
	}
	
	// Merge with existing rules
	cfg.SamplingRules = append(cfg.SamplingRules, generatedRules...)
	
	return nil
}

// ValidateEnhanced performs enhanced validation
func (cfg *EnhancedConfig) ValidateEnhanced() error {
	// Basic validation
	if err := cfg.Validate(); err != nil {
		return err
	}
	
	// Enhanced validation
	if cfg.SlowQueryThresholdMs <= 0 {
		return fmt.Errorf("slow_query_threshold_ms must be positive, got: %f", cfg.SlowQueryThresholdMs)
	}
	
	// Validate environment overrides
	for env, envConfig := range cfg.EnvironmentOverrides {
		if envConfig.SlowQueryThresholdMs < 0 {
			return fmt.Errorf("environment %s: slow_query_threshold_ms cannot be negative", env)
		}
		if envConfig.MaxRecordsPerSecond < 0 {
			return fmt.Errorf("environment %s: max_records_per_second cannot be negative", env)
		}
		if envConfig.DefaultSampleRate < 0 || envConfig.DefaultSampleRate > 1.0 {
			return fmt.Errorf("environment %s: default_sample_rate must be between 0.0 and 1.0", env)
		}
	}
	
	// Validate metrics config
	if cfg.MetricsConfig.Enabled && cfg.MetricsConfig.Interval <= 0 {
		return fmt.Errorf("metrics interval must be positive when metrics are enabled")
	}
	
	return nil
}

// Validate checks the processor configuration (delegates to original)
func (cfg *EnhancedConfig) Validate() error {
	// Convert to base config for validation
	baseConfig := &Config{
		InMemoryOnly:        cfg.InMemoryOnly,
		Deduplication:       cfg.Deduplication,
		SamplingRules:       cfg.SamplingRules,
		DefaultSampleRate:   cfg.DefaultSampleRate,
		MaxRecordsPerSecond: cfg.MaxRecordsPerSecond,
		EnableDebugLogging:  cfg.EnableDebugLogging,
	}
	
	return baseConfig.Validate()
}