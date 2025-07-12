package ash

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

// Config represents the receiver configuration
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	
	// Database connection settings
	Driver              string `mapstructure:"driver"`
	Datasource          string `mapstructure:"datasource"`
	Database            string `mapstructure:"database"`
	
	// Collection settings
	CollectionInterval  time.Duration   `mapstructure:"collection_interval"`
	SamplingConfig      SamplingConfig  `mapstructure:"sampling"`
	SamplingRate        float64         `mapstructure:"sampling_rate"`
	IncludeIdleSessions bool            `mapstructure:"include_idle_sessions"`
	LongRunningThreshold time.Duration  `mapstructure:"long_running_threshold"`
	
	// Storage settings
	BufferSize         int           `mapstructure:"buffer_size"`
	RetentionDuration  time.Duration `mapstructure:"retention_duration"`
	AggregationWindows []time.Duration `mapstructure:"aggregation_windows"`
	
	// Feature detection
	EnableFeatureDetection bool `mapstructure:"enable_feature_detection"`
	
	// Analysis settings
	EnableWaitAnalysis     bool `mapstructure:"enable_wait_analysis"`
	EnableBlockingAnalysis bool `mapstructure:"enable_blocking_analysis"`
	EnableAnomalyDetection bool `mapstructure:"enable_anomaly_detection"`
	
	// Performance thresholds
	SlowQueryThresholdMs   int64 `mapstructure:"slow_query_threshold_ms"`
	BlockedSessionThreshold int   `mapstructure:"blocked_session_threshold"`
	
	// Retry configuration
	BackOffConfig configretry.BackOffConfig `mapstructure:"retry_on_failure"`
}

// SamplingConfig controls adaptive sampling behavior
type SamplingConfig struct {
	// Base sampling rate (0.0 to 1.0)
	BaseRate float64 `mapstructure:"base_rate"`
	
	// Minimum sampling rate to maintain
	MinRate float64 `mapstructure:"min_rate"`
	
	// Maximum sampling rate
	MaxRate float64 `mapstructure:"max_rate"`
	
	// Session count thresholds for adaptive sampling
	LowSessionThreshold  int `mapstructure:"low_session_threshold"`
	HighSessionThreshold int `mapstructure:"high_session_threshold"`
	
	// Always sample these session types
	AlwaysSampleBlocked      bool `mapstructure:"always_sample_blocked"`
	AlwaysSampleLongRunning  bool `mapstructure:"always_sample_long_running"`
	AlwaysSampleMaintenance  bool `mapstructure:"always_sample_maintenance"`
}

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Driver == "" {
		return errors.New("driver must be specified")
	}
	
	if cfg.Datasource == "" {
		return errors.New("datasource must be specified")
	}
	
	// Validate supported drivers
	switch cfg.Driver {
	case "postgres", "postgresql", "pgx", "mysql":
		// Supported
	default:
		return fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}
	
	// Validate sampling config
	if cfg.SamplingConfig.BaseRate < 0 || cfg.SamplingConfig.BaseRate > 1 {
		return errors.New("sampling base_rate must be between 0.0 and 1.0")
	}
	
	if cfg.SamplingConfig.MinRate < 0 || cfg.SamplingConfig.MinRate > 1 {
		return errors.New("sampling min_rate must be between 0.0 and 1.0")
	}
	
	if cfg.SamplingConfig.MaxRate < 0 || cfg.SamplingConfig.MaxRate > 1 {
		return errors.New("sampling max_rate must be between 0.0 and 1.0")
	}
	
	if cfg.SamplingConfig.MinRate > cfg.SamplingConfig.MaxRate {
		return errors.New("sampling min_rate cannot be greater than max_rate")
	}
	
	// Validate buffer settings
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 10000 // Default
	}
	
	if cfg.RetentionDuration <= 0 {
		cfg.RetentionDuration = 1 * time.Hour // Default
	}
	
	// Set default aggregation windows if not specified
	if len(cfg.AggregationWindows) == 0 {
		cfg.AggregationWindows = []time.Duration{
			1 * time.Minute,
			5 * time.Minute,
			15 * time.Minute,
			1 * time.Hour,
		}
	}
	
	// Set default thresholds
	if cfg.SlowQueryThresholdMs <= 0 {
		cfg.SlowQueryThresholdMs = 1000 // 1 second
	}
	
	if cfg.BlockedSessionThreshold <= 0 {
		cfg.BlockedSessionThreshold = 5
	}
	
	return nil
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Second,
			InitialDelay:       1 * time.Second,
		},
		BufferSize:        10000,
		RetentionDuration: 1 * time.Hour,
		AggregationWindows: []time.Duration{
			1 * time.Minute,
			5 * time.Minute,
			15 * time.Minute,
			1 * time.Hour,
		},
		SamplingConfig: SamplingConfig{
			BaseRate:                 1.0,
			MinRate:                  0.1,
			MaxRate:                  1.0,
			LowSessionThreshold:      50,
			HighSessionThreshold:     500,
			AlwaysSampleBlocked:      true,
			AlwaysSampleLongRunning:  true,
			AlwaysSampleMaintenance:  true,
		},
		EnableFeatureDetection: true,
		EnableWaitAnalysis:     true,
		EnableBlockingAnalysis: true,
		EnableAnomalyDetection: false,
		SlowQueryThresholdMs:   1000,
		BlockedSessionThreshold: 5,
		BackOffConfig: configretry.BackOffConfig{
			Enabled:             true,
			InitialInterval:     5 * time.Second,
			MaxInterval:         30 * time.Second,
			MaxElapsedTime:      5 * time.Minute,
		},
	}
}
const (
	defaultSessionsQuery = `SELECT * FROM v$session WHERE status = 'ACTIVE'`
	defaultHistoryQuery  = `SELECT * FROM v$active_session_history WHERE sample_time > :1`
)

// Config with additional fields
type ConfigWithInterval struct {
	Config
	CollectionInterval time.Duration     `mapstructure:"collection_interval"`
	Queries            map[string]string `mapstructure:"queries"`
}
