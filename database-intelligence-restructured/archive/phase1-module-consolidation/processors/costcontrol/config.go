package costcontrol

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config configures the cost control processor
type Config struct {
	// MonthlyBudgetUSD is the target monthly spend in USD
	MonthlyBudgetUSD float64 `mapstructure:"monthly_budget_usd"`
	
	// PricePerGB is the cost per GB of data ingested
	// Default: 0.35 for standard, 0.55 for Data Plus
	PricePerGB float64 `mapstructure:"price_per_gb"`
	
	// MetricCardinalityLimit is the max unique time series per metric
	MetricCardinalityLimit int `mapstructure:"metric_cardinality_limit"`
	
	// SlowSpanThresholdMs defines what constitutes a slow span
	SlowSpanThresholdMs int64 `mapstructure:"slow_span_threshold_ms"`
	
	// MaxLogBodySize is the maximum size for log bodies
	MaxLogBodySize int `mapstructure:"max_log_body_size"`
	
	// ReportingInterval for cost metrics
	ReportingInterval time.Duration `mapstructure:"reporting_interval"`
	
	// AggressiveMode enables more aggressive cost reduction
	AggressiveMode bool `mapstructure:"aggressive_mode"`
	
	// DataPlusEnabled indicates if using New Relic Data Plus
	DataPlusEnabled bool `mapstructure:"data_plus_enabled"`
	
	// HighCardinalityDimensions defines which dimensions to remove for cardinality reduction
	HighCardinalityDimensions []string `mapstructure:"high_cardinality_dimensions"`
}

// Validate checks the processor configuration
func (cfg *Config) Validate() error {
	if cfg.MonthlyBudgetUSD <= 0 {
		return fmt.Errorf("monthly_budget_usd must be positive")
	}
	
	if cfg.PricePerGB <= 0 {
		return fmt.Errorf("price_per_gb must be positive")
	}
	
	if cfg.MetricCardinalityLimit <= 0 {
		return fmt.Errorf("metric_cardinality_limit must be positive")
	}
	
	if cfg.SlowSpanThresholdMs <= 0 {
		return fmt.Errorf("slow_span_threshold_ms must be positive")
	}
	
	if cfg.MaxLogBodySize <= 0 {
		return fmt.Errorf("max_log_body_size must be positive")
	}
	
	if cfg.ReportingInterval <= 0 {
		return fmt.Errorf("reporting_interval must be positive")
	}
	
	return nil
}

// createDefaultConfig creates the default configuration
func createDefaultConfig() component.Config {
	return &Config{
		MonthlyBudgetUSD:       1000.0, // $1000/month default budget
		PricePerGB:            0.35,    // Standard pricing
		MetricCardinalityLimit: 10000,  // 10k unique series per metric
		SlowSpanThresholdMs:   2000,    // 2 second threshold
		MaxLogBodySize:        10240,   // 10KB max log body
		ReportingInterval:     60 * time.Second,
		AggressiveMode:        false,
		DataPlusEnabled:       false,
	}
}