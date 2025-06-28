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
}

// VerificationQuery defines a custom verification query
type VerificationQuery struct {
	Name        string        `mapstructure:"name"`
	Query       string        `mapstructure:"query"`
	Interval    time.Duration `mapstructure:"interval"`
	Threshold   float64       `mapstructure:"threshold"`
	Comparison  string        `mapstructure:"comparison"` // gt, lt, eq
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
		},
	}
}