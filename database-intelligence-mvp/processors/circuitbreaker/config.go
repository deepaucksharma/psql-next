package circuitbreaker

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config represents the configuration for the circuit breaker processor
type Config struct {
	// FailureThreshold number of failures to open circuit
	FailureThreshold int `mapstructure:"failure_threshold"`

	// SuccessThreshold number of successes to close from half-open
	SuccessThreshold int `mapstructure:"success_threshold"`

	// OpenStateTimeout how long to stay open before trying half-open
	OpenStateTimeout time.Duration `mapstructure:"open_state_timeout"`

	// MaxConcurrentRequests maximum concurrent requests allowed
	MaxConcurrentRequests int `mapstructure:"max_concurrent_requests"`

	// BaseTimeout base timeout for requests
	BaseTimeout time.Duration `mapstructure:"base_timeout"`

	// MaxTimeout maximum timeout for adaptive timeout
	MaxTimeout time.Duration `mapstructure:"max_timeout"`

	// EnableAdaptiveTimeout enables dynamic timeout adjustment
	EnableAdaptiveTimeout bool `mapstructure:"enable_adaptive_timeout"`

	// HealthCheckInterval how often to check system health
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval"`

	// MemoryThresholdMB opens circuit if memory usage exceeds this (0 = disabled)
	MemoryThresholdMB int `mapstructure:"memory_threshold_mb"`

	// CPUThresholdPercent opens circuit if CPU usage exceeds this (0 = disabled)
	CPUThresholdPercent float64 `mapstructure:"cpu_threshold_percent"`

	// EnableDebugLogging enables detailed debug output
	EnableDebugLogging bool `mapstructure:"enable_debug_logging"`

	// NewRelicErrorPatterns is a list of strings that indicate a New Relic integration error
	NewRelicErrorPatterns []string `mapstructure:"new_relic_error_patterns"`
	
	// ErrorPatterns define patterns for feature-aware error handling
	ErrorPatterns []ErrorPatternConfig `mapstructure:"error_patterns"`
	
	// QueryFallbacks define fallback queries for primary queries
	QueryFallbacks map[string]string `mapstructure:"query_fallbacks"`
}

// ErrorPatternConfig defines configuration for error pattern matching
type ErrorPatternConfig struct {
	Pattern     string        `mapstructure:"pattern"`
	Action      string        `mapstructure:"action"`
	Feature     string        `mapstructure:"feature"`
	Backoff     time.Duration `mapstructure:"backoff"`
	Description string        `mapstructure:"description"`
}

// Validate checks the processor configuration
func (cfg *Config) Validate() error {
	if cfg.FailureThreshold <= 0 {
		return fmt.Errorf("failure_threshold must be positive, got: %d", cfg.FailureThreshold)
	}

	if cfg.SuccessThreshold <= 0 {
		return fmt.Errorf("success_threshold must be positive, got: %d", cfg.SuccessThreshold)
	}

	if cfg.OpenStateTimeout <= 0 {
		return fmt.Errorf("open_state_timeout must be positive, got: %v", cfg.OpenStateTimeout)
	}

	if cfg.MaxConcurrentRequests <= 0 {
		return fmt.Errorf("max_concurrent_requests must be positive, got: %d", cfg.MaxConcurrentRequests)
	}

	if cfg.BaseTimeout <= 0 {
		return fmt.Errorf("base_timeout must be positive, got: %v", cfg.BaseTimeout)
	}

	if cfg.MaxTimeout <= 0 {
		return fmt.Errorf("max_timeout must be positive, got: %v", cfg.MaxTimeout)
	}

	if cfg.BaseTimeout > cfg.MaxTimeout {
		return fmt.Errorf("base_timeout (%v) cannot be greater than max_timeout (%v)", cfg.BaseTimeout, cfg.MaxTimeout)
	}

	if cfg.HealthCheckInterval <= 0 {
		return fmt.Errorf("health_check_interval must be positive, got: %v", cfg.HealthCheckInterval)
	}

	if cfg.MemoryThresholdMB < 0 {
		return fmt.Errorf("memory_threshold_mb cannot be negative, got: %d", cfg.MemoryThresholdMB)
	}

	if cfg.CPUThresholdPercent < 0 || cfg.CPUThresholdPercent > 100 {
		return fmt.Errorf("cpu_threshold_percent must be between 0 and 100, got: %f", cfg.CPUThresholdPercent)
	}

	return nil
}

// createDefaultConfig creates a default configuration
func createDefaultConfig() component.Config {
	return &Config{
		FailureThreshold:      5,
		SuccessThreshold:      3,
		OpenStateTimeout:      30 * time.Second,
		MaxConcurrentRequests: 100,
		BaseTimeout:           5 * time.Second,
		MaxTimeout:            30 * time.Second,
		EnableAdaptiveTimeout: true,
		HealthCheckInterval:   10 * time.Second,
		MemoryThresholdMB:     512, // 512MB
		CPUThresholdPercent:   80.0, // 80%
		EnableDebugLogging:    false,
		NewRelicErrorPatterns: []string{
			"cardinality",
			"NrIntegrationError",
			"api-key",
			"rate limit",
			"quota exceeded",
			"unique time series",
		},
	}
}