package waitanalysis

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

// Config represents the processor configuration
type Config struct {
	// Enabled controls whether the processor is active
	Enabled bool `mapstructure:"enabled"`
	
	// Patterns for categorizing wait events
	Patterns []PatternConfig `mapstructure:"patterns"`
	
	// Alert rules based on wait events
	AlertRules []AlertRuleConfig `mapstructure:"alert_rules"`
}

// PatternConfig defines a wait event pattern
type PatternConfig struct {
	Name        string   `mapstructure:"name"`
	EventTypes  []string `mapstructure:"event_types"`
	Events      []string `mapstructure:"events"`
	Category    string   `mapstructure:"category"`
	Severity    string   `mapstructure:"severity"`
	Description string   `mapstructure:"description"`
}

// AlertRuleConfig defines an alert rule
type AlertRuleConfig struct {
	Name      string  `mapstructure:"name"`
	Condition string  `mapstructure:"condition"`
	Threshold float64 `mapstructure:"threshold"`
	Window    string  `mapstructure:"window"`
	Action    string  `mapstructure:"action"`
}

var (
	errNoPatterns = errors.New("at least one pattern must be configured")
	errInvalidSeverity = errors.New("severity must be one of: info, warning, critical")
	errInvalidAction = errors.New("action must be one of: alert, log")
)

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Enabled && len(cfg.Patterns) == 0 {
		return errNoPatterns
	}
	
	// Validate patterns
	validSeverities := map[string]bool{
		"info":     true,
		"warning":  true,
		"critical": true,
	}
	
	for _, pattern := range cfg.Patterns {
		if !validSeverities[pattern.Severity] {
			return errInvalidSeverity
		}
	}
	
	// Validate alert rules
	validActions := map[string]bool{
		"alert": true,
		"log":   true,
	}
	
	for _, rule := range cfg.AlertRules {
		if !validActions[rule.Action] {
			return errInvalidAction
		}
	}
	
	return nil
}

// Compile-time check to ensure Config implements component.Config
var _ component.Config = (*Config)(nil)