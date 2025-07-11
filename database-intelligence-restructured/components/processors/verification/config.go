// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package verification

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the verification processor
type Config struct {
	// EnablePeriodicVerification enables periodic health checks
	EnablePeriodicVerification bool `mapstructure:"enable_periodic_verification"`
	
	// VerificationInterval sets how often to run verification checks
	VerificationInterval time.Duration `mapstructure:"verification_interval"`
	
	// DataFreshnessThreshold sets the maximum time without data before alerting
	DataFreshnessThreshold time.Duration `mapstructure:"data_freshness_threshold"`
	
	// MinEntityCorrelationRate sets the minimum acceptable entity correlation rate (0.0-1.0)
	MinEntityCorrelationRate float64 `mapstructure:"min_entity_correlation_rate"`
	
	// MinNormalizationRate sets the minimum acceptable query normalization rate (0.0-1.0)
	MinNormalizationRate float64 `mapstructure:"min_normalization_rate"`
	
	// RequireEntitySynthesis enforces entity synthesis attributes
	RequireEntitySynthesis bool `mapstructure:"require_entity_synthesis"`
	
	// ExportFeedbackAsLogs exports feedback events as telemetry
	ExportFeedbackAsLogs bool `mapstructure:"export_feedback_as_logs"`
	
	// FeedbackEndpoint is the endpoint to send feedback (optional)
	FeedbackEndpoint string `mapstructure:"feedback_endpoint"`
	
	// VerificationQueries are custom NRQL queries to run for verification
	VerificationQueries []VerificationQuery `mapstructure:"verification_queries"`
	
	// EnableContinuousHealthChecks enables continuous system health monitoring
	EnableContinuousHealthChecks bool `mapstructure:"enable_continuous_health_checks"`
	
	// HealthCheckInterval sets how often to perform health checks
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval"`
	
	// HealthThresholds defines system resource alert thresholds
	HealthThresholds HealthThresholdsConfig `mapstructure:"health_thresholds"`
	
	// QualityRules defines data quality validation rules
	QualityRules QualityRulesConfig `mapstructure:"quality_rules"`
	
	// PIIDetection configures PII detection and sanitization
	PIIDetection PIIDetectionConfig `mapstructure:"pii_detection"`
}

// VerificationQuery defines a custom verification query
type VerificationQuery struct {
	Name        string        `mapstructure:"name"`
	Query       string        `mapstructure:"query"`
	Interval    time.Duration `mapstructure:"interval"`
	Threshold   float64       `mapstructure:"threshold"`
	Comparison  string        `mapstructure:"comparison"` // gt, lt, eq
}

// HealthThresholdsConfig defines system resource alert thresholds
type HealthThresholdsConfig struct {
	MemoryPercent  float64       `mapstructure:"memory_percent"`
	CPUPercent     float64       `mapstructure:"cpu_percent"`
	DiskPercent    float64       `mapstructure:"disk_percent"`
	NetworkLatency time.Duration `mapstructure:"network_latency"`
}

// QualityRulesConfig defines data quality validation rules
type QualityRulesConfig struct {
	RequiredFields         []string          `mapstructure:"required_fields"`
	EnableSchemaValidation bool              `mapstructure:"enable_schema_validation"`
	CardinalityLimits      map[string]int    `mapstructure:"cardinality_limits"`
	DataTypeValidation     map[string]string `mapstructure:"data_type_validation"`
}

// PIIDetectionConfig configures PII detection and sanitization
type PIIDetectionConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	AutoSanitize    bool     `mapstructure:"auto_sanitize"`
	CustomPatterns  []string `mapstructure:"custom_patterns"`
	ExcludeFields   []string `mapstructure:"exclude_fields"`
	SensitivityLevel string  `mapstructure:"sensitivity_level"` // low, medium, high
}


// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if cfg.EnablePeriodicVerification {
		if cfg.VerificationInterval <= 0 {
			return errors.New("verification_interval must be positive when periodic verification is enabled")
		}
	}
	
	if cfg.DataFreshnessThreshold <= 0 {
		cfg.DataFreshnessThreshold = 10 * time.Minute // Default
	}
	
	if cfg.MinEntityCorrelationRate < 0 || cfg.MinEntityCorrelationRate > 1 {
		return errors.New("min_entity_correlation_rate must be between 0.0 and 1.0")
	}
	
	if cfg.MinNormalizationRate < 0 || cfg.MinNormalizationRate > 1 {
		return errors.New("min_normalization_rate must be between 0.0 and 1.0")
	}
	
	// Validate health check configuration
	if cfg.EnableContinuousHealthChecks {
		if cfg.HealthCheckInterval <= 0 {
			return errors.New("health_check_interval must be positive when continuous health checks are enabled")
		}
		
		if cfg.HealthThresholds.MemoryPercent < 0 || cfg.HealthThresholds.MemoryPercent > 100 {
			return errors.New("health_thresholds.memory_percent must be between 0 and 100")
		}
		
		if cfg.HealthThresholds.CPUPercent < 0 || cfg.HealthThresholds.CPUPercent > 100 {
			return errors.New("health_thresholds.cpu_percent must be between 0 and 100")
		}
		
		if cfg.HealthThresholds.DiskPercent < 0 || cfg.HealthThresholds.DiskPercent > 100 {
			return errors.New("health_thresholds.disk_percent must be between 0 and 100")
		}
	}
	
	
	// Validate PII detection configuration
	if cfg.PIIDetection.Enabled {
		validSensitivityLevels := map[string]bool{
			"low": true, "medium": true, "high": true,
		}
		if cfg.PIIDetection.SensitivityLevel != "" && !validSensitivityLevels[cfg.PIIDetection.SensitivityLevel] {
			return errors.New("pii_detection.sensitivity_level must be 'low', 'medium', or 'high'")
		}
	}
	
	// Validate custom queries
	for _, q := range cfg.VerificationQueries {
		if q.Name == "" {
			return errors.New("verification query name cannot be empty")
		}
		if q.Query == "" {
			return errors.New("verification query cannot be empty")
		}
		if q.Comparison != "gt" && q.Comparison != "lt" && q.Comparison != "eq" {
			return errors.New("verification query comparison must be 'gt', 'lt', or 'eq'")
		}
	}
	
	return nil
}

// createDefaultConfig creates the default configuration for the verification processor
func createDefaultConfig() component.Config {
	return &Config{
		EnablePeriodicVerification: true,
		VerificationInterval:       5 * time.Minute,
		DataFreshnessThreshold:     10 * time.Minute,
		MinEntityCorrelationRate:   0.8, // 80%
		MinNormalizationRate:       0.9, // 90%
		RequireEntitySynthesis:     true,
		ExportFeedbackAsLogs:       true,
		
		// Continuous health checks
		EnableContinuousHealthChecks: true,
		HealthCheckInterval:         30 * time.Second,
		HealthThresholds: HealthThresholdsConfig{
			MemoryPercent:  85.0, // 85%
			CPUPercent:     80.0, // 80%
			DiskPercent:    90.0, // 90%
			NetworkLatency: 5 * time.Second,
		},
		
		// Quality validation rules
		QualityRules: QualityRulesConfig{
			RequiredFields: []string{
				"database_name",
				"query_id",
				"duration_ms",
			},
			EnableSchemaValidation: true,
			CardinalityLimits: map[string]int{
				"query_id":      10000,
				"database_name": 100,
				"table_name":    1000,
			},
			DataTypeValidation: map[string]string{
				"duration_ms":   "double",
				"error_count":   "int",
				"database_name": "string",
			},
		},
		
		// PII detection
		PIIDetection: PIIDetectionConfig{
			Enabled:          true,
			AutoSanitize:     false, // Conservative default
			SensitivityLevel: "medium",
			ExcludeFields: []string{
				"query_hash",
				"plan_hash",
				"database_name",
			},
		},
		
		VerificationQueries: []VerificationQuery{
			{
				Name:       "integration_errors",
				Query:      "SELECT count(*) FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' SINCE 5 minutes ago",
				Interval:   5 * time.Minute,
				Threshold:  10,
				Comparison: "lt",
			},
			{
				Name:       "data_freshness",
				Query:      "SELECT count(*) FROM Log WHERE collector.name = 'database-intelligence' SINCE 5 minutes ago",
				Interval:   5 * time.Minute,
				Threshold:  1,
				Comparison: "gt",
			},
			{
				Name:       "quality_score",
				Query:      "SELECT average(quality_score) FROM Log WHERE collector.name = 'database-intelligence' SINCE 10 minutes ago",
				Interval:   10 * time.Minute,
				Threshold:  0.8,
				Comparison: "gt",
			},
		},
	}
}