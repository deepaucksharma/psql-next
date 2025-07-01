package enhancedsql

import (
	"errors"
	"fmt"
	"strings"
	"time"
	
	"github.com/database-intelligence-mvp/common/featuredetector"
	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the enhanced SQL receiver
type Config struct {
	// Driver is the database driver (postgres, mysql, etc.)
	Driver string `mapstructure:"driver"`
	
	// Datasource is the database connection string
	Datasource string `mapstructure:"datasource"`
	
	// CollectionInterval is how often to run queries
	CollectionInterval time.Duration `mapstructure:"collection_interval"`
	
	// Connection pool settings
	MaxOpenConnections int `mapstructure:"max_open_connections"`
	MaxIdleConnections int `mapstructure:"max_idle_connections"`
	
	// Feature detection configuration
	FeatureDetection FeatureDetectionConfig `mapstructure:"feature_detection"`
	
	// Query configurations
	Queries []QueryConfig `mapstructure:"queries"`
	
	// Custom query definitions
	CustomQueries []CustomQueryDefinition `mapstructure:"custom_queries"`
	
	// Resource attributes to add to all metrics/logs
	ResourceAttributes map[string]string `mapstructure:"resource_attributes"`
}

// FeatureDetectionConfig configures feature detection behavior
type FeatureDetectionConfig struct {
	// Enabled controls whether to perform feature detection
	Enabled bool `mapstructure:"enabled"`
	
	// CacheDuration how long to cache detection results
	CacheDuration time.Duration `mapstructure:"cache_duration"`
	
	// RefreshInterval how often to refresh feature detection
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	
	// RetryAttempts for failed detections
	RetryAttempts int `mapstructure:"retry_attempts"`
	
	// RetryDelay between attempts
	RetryDelay time.Duration `mapstructure:"retry_delay"`
	
	// TimeoutPerCheck for individual checks
	TimeoutPerCheck time.Duration `mapstructure:"timeout_per_check"`
	
	// SkipCloudDetection disables cloud provider detection
	SkipCloudDetection bool `mapstructure:"skip_cloud_detection"`
}

// QueryConfig defines configuration for a query category
type QueryConfig struct {
	// Name of the query configuration
	Name string `mapstructure:"name"`
	
	// Category of queries (slow_queries, active_sessions, etc.)
	Category string `mapstructure:"category"`
	
	// Timeout for query execution
	Timeout time.Duration `mapstructure:"timeout"`
	
	// MaxRows limits the number of rows returned
	MaxRows int `mapstructure:"max_rows"`
	
	// Parameters for the query
	Parameters []QueryParameter `mapstructure:"parameters"`
	
	// Metrics to generate from query results
	Metrics []MetricConfig `mapstructure:"metrics"`
	
	// Logs to generate from query results
	Logs []LogConfig `mapstructure:"logs"`
}

// QueryParameter defines a query parameter
type QueryParameter struct {
	// Name of the parameter
	Name string `mapstructure:"name"`
	
	// Type of the parameter (int, float, string, duration)
	Type string `mapstructure:"type"`
	
	// Default values
	DefaultInt      int           `mapstructure:"default_int"`
	DefaultFloat    float64       `mapstructure:"default_float"`
	DefaultString   string        `mapstructure:"default_string"`
	DefaultDuration time.Duration `mapstructure:"default_duration"`
	
	// Unit for duration parameters (ms, s, m)
	Unit string `mapstructure:"unit"`
}

// MetricConfig defines how to create metrics from query results
type MetricConfig struct {
	// MetricName is the name of the metric to create
	MetricName string `mapstructure:"metric_name"`
	
	// Description of the metric
	Description string `mapstructure:"description"`
	
	// ValueColumn is the column containing the metric value
	ValueColumn string `mapstructure:"value_column"`
	
	// ValueType is the type of metric (gauge, sum, histogram)
	ValueType string `mapstructure:"value_type"`
	
	// AttributeColumns are columns to use as metric attributes
	AttributeColumns []string `mapstructure:"attribute_columns"`
}

// LogConfig defines how to create logs from query results
type LogConfig struct {
	// BodyColumn is the column to use as log body
	BodyColumn string `mapstructure:"body_column"`
	
	// SeverityColumn is the column containing severity
	SeverityColumn string `mapstructure:"severity_column"`
	
	// Attributes maps attribute names to column names
	Attributes map[string]string `mapstructure:"attributes"`
}

// CustomQueryDefinition allows users to define custom queries
type CustomQueryDefinition struct {
	// Name of the query
	Name string `mapstructure:"name"`
	
	// Category where this query belongs
	Category string `mapstructure:"category"`
	
	// SQL query text
	SQL string `mapstructure:"sql"`
	
	// Priority for query selection (higher = preferred)
	Priority int `mapstructure:"priority"`
	
	// Description of what this query does
	Description string `mapstructure:"description"`
	
	// Requirements for this query
	Requirements featuredetector.QueryRequirements `mapstructure:"requirements"`
}

// Validate checks the configuration
func (cfg *Config) Validate() error {
	if cfg.Driver == "" {
		return errors.New("driver is required")
	}
	
	if cfg.Datasource == "" {
		return errors.New("datasource is required")
	}
	
	// Validate driver
	switch cfg.Driver {
	case "postgres", "mysql", "sqlserver", "oracle":
		// Valid drivers
	default:
		return fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}
	
	// Validate collection interval
	if cfg.CollectionInterval <= 0 {
		return errors.New("collection_interval must be positive")
	}
	
	// Validate feature detection
	if cfg.FeatureDetection.Enabled {
		if cfg.FeatureDetection.CacheDuration <= 0 {
			cfg.FeatureDetection.CacheDuration = 5 * time.Minute
		}
		if cfg.FeatureDetection.RefreshInterval <= 0 {
			cfg.FeatureDetection.RefreshInterval = 30 * time.Minute
		}
		if cfg.FeatureDetection.TimeoutPerCheck <= 0 {
			cfg.FeatureDetection.TimeoutPerCheck = 3 * time.Second
		}
		if cfg.FeatureDetection.RetryAttempts < 0 {
			cfg.FeatureDetection.RetryAttempts = 3
		}
		if cfg.FeatureDetection.RetryDelay <= 0 {
			cfg.FeatureDetection.RetryDelay = time.Second
		}
	}
	
	// Validate queries
	if len(cfg.Queries) == 0 && len(cfg.CustomQueries) == 0 {
		return errors.New("at least one query must be configured")
	}
	
	for i, query := range cfg.Queries {
		if query.Name == "" {
			return fmt.Errorf("query[%d]: name is required", i)
		}
		if query.Category == "" {
			return fmt.Errorf("query[%d]: category is required", i)
		}
		if query.Timeout <= 0 {
			cfg.Queries[i].Timeout = 30 * time.Second
		}
		
		// Validate parameters
		for j, param := range query.Parameters {
			if param.Name == "" {
				return fmt.Errorf("query[%d].parameters[%d]: name is required", i, j)
			}
			if param.Type == "" {
				cfg.Queries[i].Parameters[j].Type = "string"
			}
		}
		
		// Validate outputs
		if len(query.Metrics) == 0 && len(query.Logs) == 0 {
			return fmt.Errorf("query[%d]: must have at least one metric or log configuration", i)
		}
	}
	
	// Validate custom queries
	for i, custom := range cfg.CustomQueries {
		if custom.Name == "" {
			return fmt.Errorf("custom_queries[%d]: name is required", i)
		}
		if custom.Category == "" {
			return fmt.Errorf("custom_queries[%d]: category is required", i)
		}
		if custom.SQL == "" {
			return fmt.Errorf("custom_queries[%d]: sql is required", i)
		}
	}
	
	return nil
}

// getDatasourceMasked returns datasource with password masked
func (cfg *Config) getDatasourceMasked() string {
	// Simple masking - in production, use more sophisticated approach
	parts := strings.Split(cfg.Datasource, "password=")
	if len(parts) > 1 {
		endIdx := strings.IndexAny(parts[1], " &")
		if endIdx == -1 {
			endIdx = len(parts[1])
		}
		return parts[0] + "password=***" + parts[1][endIdx:]
	}
	return cfg.Datasource
}

// CreateDefaultConfig creates the default configuration
func createDefaultConfig() component.Config {
	return &Config{
		Driver:                    "postgres",
		CollectionInterval:        60 * time.Second,
		MaxOpenConnections:        10,
		MaxIdleConnections:        5,
		FeatureDetection: FeatureDetectionConfig{
			Enabled:            true,
			CacheDuration:      5 * time.Minute,
			RefreshInterval:    30 * time.Minute,
			TimeoutPerCheck:    3 * time.Second,
			RetryAttempts:      3,
			RetryDelay:         time.Second,
			SkipCloudDetection: false,
		},
		Queries: []QueryConfig{
			{
				Name:     "slow_queries",
				Category: "slow_queries",
				Timeout:  30 * time.Second,
				MaxRows:  100,
				Parameters: []QueryParameter{
					{
						Name:          "min_duration",
						Type:          "duration",
						DefaultDuration: 50 * time.Millisecond,
						Unit:          "ms",
					},
					{
						Name:       "limit",
						Type:       "int",
						DefaultInt: 20,
					},
				},
			},
		},
	}
}