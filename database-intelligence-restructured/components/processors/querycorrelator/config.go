package querycorrelator

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the query correlator processor
type Config struct {
	// RetentionPeriod is how long to keep correlation data
	RetentionPeriod time.Duration `mapstructure:"retention_period"`
	
	// CleanupInterval is how often to clean up old data
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	
	// EnableTableCorrelation enables correlation with table statistics
	EnableTableCorrelation bool `mapstructure:"enable_table_correlation"`
	
	// EnableDatabaseCorrelation enables correlation with database statistics
	EnableDatabaseCorrelation bool `mapstructure:"enable_database_correlation"`
	
	// MaxQueriesTracked is the maximum number of queries to track
	MaxQueriesTracked int `mapstructure:"max_queries_tracked"`
	
	// MaxQueryCount is the maximum number of queries to keep in memory
	MaxQueryCount int `mapstructure:"max_query_count"`
	
	// MaxTableCount is the maximum number of tables to keep in memory
	MaxTableCount int `mapstructure:"max_table_count"`
	
	// MaxDatabaseCount is the maximum number of databases to keep in memory
	MaxDatabaseCount int `mapstructure:"max_database_count"`
	
	// CorrelationAttributes defines which attributes to add
	CorrelationAttributes CorrelationAttributesConfig `mapstructure:"correlation_attributes"`
	
	// QueryCategorization defines query performance categorization thresholds
	QueryCategorization QueryCategorizationConfig `mapstructure:"query_categorization"`
}

// CorrelationAttributesConfig defines which correlation attributes to add
type CorrelationAttributesConfig struct {
	// AddQueryCategory adds performance category (slow/moderate/fast)
	AddQueryCategory bool `mapstructure:"add_query_category"`
	
	// AddTableStats adds table modification and dead tuple counts
	AddTableStats bool `mapstructure:"add_table_stats"`
	
	// AddLoadContribution adds query's contribution to database load
	AddLoadContribution bool `mapstructure:"add_load_contribution"`
	
	// AddMaintenanceIndicators adds indicators like needs_vacuum
	AddMaintenanceIndicators bool `mapstructure:"add_maintenance_indicators"`
}

// QueryCategorizationConfig defines thresholds for query performance categorization
type QueryCategorizationConfig struct {
	// SlowQueryThresholdMs defines the threshold for slow queries in milliseconds
	SlowQueryThresholdMs float64 `mapstructure:"slow_query_threshold_ms"`
	
	// ModerateQueryThresholdMs defines the threshold for moderate queries in milliseconds
	ModerateQueryThresholdMs float64 `mapstructure:"moderate_query_threshold_ms"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if cfg.RetentionPeriod <= 0 {
		return fmt.Errorf("retention_period must be positive, got %v", cfg.RetentionPeriod)
	}
	
	if cfg.CleanupInterval <= 0 {
		return fmt.Errorf("cleanup_interval must be positive, got %v", cfg.CleanupInterval)
	}
	
	if cfg.CleanupInterval > cfg.RetentionPeriod {
		return fmt.Errorf("cleanup_interval (%v) should not be greater than retention_period (%v)",
			cfg.CleanupInterval, cfg.RetentionPeriod)
	}
	
	if cfg.MaxQueriesTracked < 0 {
		return fmt.Errorf("max_queries_tracked must be non-negative, got %d", cfg.MaxQueriesTracked)
	}
	
	return nil
}