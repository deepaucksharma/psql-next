package postgresqlquery

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
)

// Config represents the receiver configuration
type Config struct {
	// Databases to monitor
	Databases []DatabaseConfig `mapstructure:"databases"`

	// Collection interval
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// Query timeout for safety
	QueryTimeout time.Duration `mapstructure:"query_timeout"`

	// Slow query threshold in milliseconds
	SlowQueryThresholdMS float64 `mapstructure:"slow_query_threshold_ms"`

	// Maximum queries to collect per cycle
	MaxQueriesPerCycle int `mapstructure:"max_queries_per_cycle"`

	// Maximum execution plans to collect per cycle
	MaxPlansPerCycle int `mapstructure:"max_plans_per_cycle"`

	// Plan collection threshold in milliseconds
	PlanCollectionThresholdMS float64 `mapstructure:"plan_collection_threshold_ms"`

	// Enable plan regression detection
	EnablePlanRegression bool `mapstructure:"enable_plan_regression"`

	// Enable Active Session History (ASH) sampling
	EnableASH bool `mapstructure:"enable_ash"`

	// ASH sampling interval
	ASHSamplingInterval time.Duration `mapstructure:"ash_sampling_interval"`

	// Enable extended metrics (requires pg_stat_kcache)
	EnableExtendedMetrics bool `mapstructure:"enable_extended_metrics"`

	// Minimal mode - only collect essential metrics
	MinimalMode bool `mapstructure:"minimal_mode"`

	// Maximum errors per database before circuit breaker
	MaxErrorsPerDatabase int `mapstructure:"max_errors_per_database"`

	// PII sanitization settings
	SanitizePII bool `mapstructure:"sanitize_pii"`

	// Adaptive sampling configuration
	AdaptiveSampling AdaptiveSamplingConfig `mapstructure:"adaptive_sampling"`
}

// DatabaseConfig represents configuration for a single database
type DatabaseConfig struct {
	// Database name (for identification)
	Name string `mapstructure:"name"`

	// PostgreSQL connection string
	DSN configopaque.String `mapstructure:"dsn"`

	// Connection pool settings
	MaxOpenConnections    int           `mapstructure:"max_open_connections"`
	MaxIdleConnections    int           `mapstructure:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `mapstructure:"connection_max_lifetime"`
	ConnectionMaxIdleTime time.Duration `mapstructure:"connection_max_idle_time"`

	// Database-specific overrides
	CollectionInterval   *time.Duration `mapstructure:"collection_interval"`
	SlowQueryThresholdMS *float64       `mapstructure:"slow_query_threshold_ms"`
	Enabled              bool           `mapstructure:"enabled"`
}

// AdaptiveSamplingConfig controls adaptive sampling behavior
type AdaptiveSamplingConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Sampling rules based on query characteristics
	Rules []SamplingRule `mapstructure:"rules"`

	// Default sampling rate (0.0 to 1.0)
	DefaultRate float64 `mapstructure:"default_rate"`

	// Rate limiting
	MaxQueriesPerMinute int `mapstructure:"max_queries_per_minute"`

	// Memory limit for sampling state
	MaxMemoryMB int `mapstructure:"max_memory_mb"`
}

// SamplingRule defines when and how to sample queries
type SamplingRule struct {
	// Rule name for identification
	Name string `mapstructure:"name"`

	// Conditions that must match
	Conditions []SamplingCondition `mapstructure:"conditions"`

	// Sampling rate when rule matches (0.0 to 1.0)
	SampleRate float64 `mapstructure:"sample_rate"`

	// Priority (higher priority rules are evaluated first)
	Priority int `mapstructure:"priority"`
}

// SamplingCondition defines a condition for sampling
type SamplingCondition struct {
	// Attribute to check
	Attribute string `mapstructure:"attribute"`

	// Operator: eq, ne, gt, lt, gte, lte, contains, regex
	Operator string `mapstructure:"operator"`

	// Value to compare against
	Value interface{} `mapstructure:"value"`
}

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if len(cfg.Databases) == 0 {
		return fmt.Errorf("at least one database must be configured")
	}

	if cfg.CollectionInterval < 10*time.Second {
		return fmt.Errorf("collection_interval must be at least 10s")
	}

	if cfg.QueryTimeout < 1*time.Second {
		return fmt.Errorf("query_timeout must be at least 1s")
	}

	if cfg.QueryTimeout > cfg.CollectionInterval {
		return fmt.Errorf("query_timeout must be less than collection_interval")
	}

	if cfg.SlowQueryThresholdMS < 0 {
		return fmt.Errorf("slow_query_threshold_ms must be non-negative")
	}

	if cfg.MaxQueriesPerCycle < 1 {
		return fmt.Errorf("max_queries_per_cycle must be at least 1")
	}

	if cfg.MaxPlansPerCycle < 0 {
		return fmt.Errorf("max_plans_per_cycle must be non-negative")
	}

	if cfg.PlanCollectionThresholdMS < 0 {
		return fmt.Errorf("plan_collection_threshold_ms must be non-negative")
	}

	if cfg.ASHSamplingInterval < 1*time.Second && cfg.EnableASH {
		return fmt.Errorf("ash_sampling_interval must be at least 1s when ASH is enabled")
	}

	if cfg.MaxErrorsPerDatabase < 1 {
		cfg.MaxErrorsPerDatabase = 10 // Default
	}

	// Validate each database configuration
	seenNames := make(map[string]bool)
	for i, db := range cfg.Databases {
		if db.Name == "" {
			return fmt.Errorf("database[%d].name is required", i)
		}

		if seenNames[db.Name] {
			return fmt.Errorf("duplicate database name: %s", db.Name)
		}
		seenNames[db.Name] = true

		if db.DSN == "" {
			return fmt.Errorf("database[%d].dsn is required", i)
		}

		// Apply defaults for connection pool
		if db.MaxOpenConnections <= 0 {
			cfg.Databases[i].MaxOpenConnections = 2
		}

		if db.MaxIdleConnections <= 0 {
			cfg.Databases[i].MaxIdleConnections = 1
		}

		if db.ConnectionMaxLifetime <= 0 {
			cfg.Databases[i].ConnectionMaxLifetime = 5 * time.Minute
		}

		if db.ConnectionMaxIdleTime <= 0 {
			cfg.Databases[i].ConnectionMaxIdleTime = 1 * time.Minute
		}

		// Validate database-specific overrides
		if db.CollectionInterval != nil && *db.CollectionInterval < 10*time.Second {
			return fmt.Errorf("database[%d].collection_interval must be at least 10s", i)
		}

		if db.SlowQueryThresholdMS != nil && *db.SlowQueryThresholdMS < 0 {
			return fmt.Errorf("database[%d].slow_query_threshold_ms must be non-negative", i)
		}
	}

	// Validate adaptive sampling configuration
	if cfg.AdaptiveSampling.Enabled {
		if cfg.AdaptiveSampling.DefaultRate < 0 || cfg.AdaptiveSampling.DefaultRate > 1 {
			return fmt.Errorf("adaptive_sampling.default_rate must be between 0.0 and 1.0")
		}

		if cfg.AdaptiveSampling.MaxQueriesPerMinute < 0 {
			return fmt.Errorf("adaptive_sampling.max_queries_per_minute must be non-negative")
		}

		if cfg.AdaptiveSampling.MaxMemoryMB < 0 {
			return fmt.Errorf("adaptive_sampling.max_memory_mb must be non-negative")
		}

		// Validate sampling rules
		for i, rule := range cfg.AdaptiveSampling.Rules {
			if rule.Name == "" {
				return fmt.Errorf("adaptive_sampling.rules[%d].name is required", i)
			}

			if rule.SampleRate < 0 || rule.SampleRate > 1 {
				return fmt.Errorf("adaptive_sampling.rules[%d].sample_rate must be between 0.0 and 1.0", i)
			}

			// Validate conditions
			for j, condition := range rule.Conditions {
				if condition.Attribute == "" {
					return fmt.Errorf("adaptive_sampling.rules[%d].conditions[%d].attribute is required", i, j)
				}

				switch condition.Operator {
				case "eq", "ne", "gt", "lt", "gte", "lte", "contains", "regex":
					// Valid operators
				default:
					return fmt.Errorf("adaptive_sampling.rules[%d].conditions[%d].operator '%s' is invalid", 
						i, j, condition.Operator)
				}

				if condition.Value == nil {
					return fmt.Errorf("adaptive_sampling.rules[%d].conditions[%d].value is required", i, j)
				}
			}
		}
	}

	return nil
}

// Default returns the default configuration
func Default() *Config {
	return &Config{
		CollectionInterval:        60 * time.Second,
		QueryTimeout:              10 * time.Second,
		SlowQueryThresholdMS:      100.0,
		MaxQueriesPerCycle:        100,
		MaxPlansPerCycle:          10,
		PlanCollectionThresholdMS: 1000.0,
		EnablePlanRegression:      true,
		EnableASH:                 false,
		ASHSamplingInterval:       1 * time.Second,
		EnableExtendedMetrics:     true,
		MinimalMode:               false,
		MaxErrorsPerDatabase:      10,
		SanitizePII:               true,
		AdaptiveSampling: AdaptiveSamplingConfig{
			Enabled:             true,
			DefaultRate:         1.0,
			MaxQueriesPerMinute: 1000,
			MaxMemoryMB:         100,
			Rules: []SamplingRule{
				{
					Name:     "always_sample_slow",
					Priority: 100,
					Conditions: []SamplingCondition{
						{
							Attribute: "mean_time_ms",
							Operator:  "gt",
							Value:     1000.0,
						},
					},
					SampleRate: 1.0,
				},
				{
					Name:     "sample_normal_queries",
					Priority: 50,
					Conditions: []SamplingCondition{
						{
							Attribute: "mean_time_ms",
							Operator:  "lte",
							Value:     1000.0,
						},
					},
					SampleRate: 0.1,
				},
			},
		},
	}
}