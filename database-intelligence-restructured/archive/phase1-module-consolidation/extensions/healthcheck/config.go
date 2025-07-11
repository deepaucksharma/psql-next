// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the health check extension
type Config struct {
	// Endpoint is the address where the health check server listens
	Endpoint string `mapstructure:"endpoint"`
	
	// Path is the HTTP path for the basic health check
	Path string `mapstructure:"path"`
	
	// CheckCollectorPipeline enables pipeline health checking
	CheckCollectorPipeline PipelineCheckConfig `mapstructure:"check_collector_pipeline"`
	
	// VerificationIntegration enables integration with verification processor
	VerificationIntegration VerificationIntegrationConfig `mapstructure:"verification_integration"`
	
	// NewRelicValidation enables New Relic specific health checks
	NewRelicValidation NewRelicValidationConfig `mapstructure:"newrelic_validation"`
}

// PipelineCheckConfig configures pipeline health checking
type PipelineCheckConfig struct {
	// Enabled enables pipeline checking
	Enabled bool `mapstructure:"enabled"`
	
	// Interval is how often to check pipeline health
	Interval time.Duration `mapstructure:"interval"`
	
	// ExporterFailureThreshold is the number of failures before marking unhealthy
	ExporterFailureThreshold int `mapstructure:"exporter_failure_threshold"`
}

// VerificationIntegrationConfig configures verification integration
type VerificationIntegrationConfig struct {
	// Enabled enables verification integration
	Enabled bool `mapstructure:"enabled"`
	
	// FeedbackEndpoint is where to receive verification feedback
	FeedbackEndpoint string `mapstructure:"feedback_endpoint"`
	
	// MaxFeedbackHistory is the maximum number of feedback events to store
	MaxFeedbackHistory int `mapstructure:"max_feedback_history"`
}

// NewRelicValidationConfig configures New Relic validation
type NewRelicValidationConfig struct {
	// Enabled enables New Relic validation
	Enabled bool `mapstructure:"enabled"`
	
	// APIKey for New Relic queries
	APIKey string `mapstructure:"api_key"`
	
	// AccountID for New Relic
	AccountID string `mapstructure:"account_id"`
	
	// ValidationQueries to run
	ValidationQueries []ValidationQuery `mapstructure:"validation_queries"`
}

// ValidationQuery defines a validation query
type ValidationQuery struct {
	Name     string        `mapstructure:"name"`
	Query    string        `mapstructure:"query"`
	Interval time.Duration `mapstructure:"interval"`
}

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "0.0.0.0:13133"
	}
	
	if cfg.Path == "" {
		cfg.Path = "/health"
	}
	
	if cfg.CheckCollectorPipeline.Enabled {
		if cfg.CheckCollectorPipeline.Interval <= 0 {
			cfg.CheckCollectorPipeline.Interval = 5 * time.Minute
		}
		
		if cfg.CheckCollectorPipeline.ExporterFailureThreshold <= 0 {
			cfg.CheckCollectorPipeline.ExporterFailureThreshold = 5
		}
	}
	
	if cfg.VerificationIntegration.Enabled {
		if cfg.VerificationIntegration.MaxFeedbackHistory <= 0 {
			cfg.VerificationIntegration.MaxFeedbackHistory = 1000
		}
	}
	
	if cfg.NewRelicValidation.Enabled {
		if cfg.NewRelicValidation.APIKey == "" {
			return errors.New("New Relic API key required when validation is enabled")
		}
		
		if cfg.NewRelicValidation.AccountID == "" {
			return errors.New("New Relic account ID required when validation is enabled")
		}
	}
	
	return nil
}

// createDefaultConfig creates the default configuration
func createDefaultConfig() component.Config {
	return &Config{
		Endpoint: "0.0.0.0:13133",
		Path:     "/health",
		CheckCollectorPipeline: PipelineCheckConfig{
			Enabled:                  true,
			Interval:                 5 * time.Minute,
			ExporterFailureThreshold: 5,
		},
		VerificationIntegration: VerificationIntegrationConfig{
			Enabled:            true,
			FeedbackEndpoint:   "localhost:13134",
			MaxFeedbackHistory: 1000,
		},
		NewRelicValidation: NewRelicValidationConfig{
			Enabled: false, // Requires API key
			ValidationQueries: []ValidationQuery{
				{
					Name:     "integration_errors",
					Query:    "SELECT count(*) FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' SINCE 5 minutes ago",
					Interval: 5 * time.Minute,
				},
				{
					Name:     "data_freshness",
					Query:    "SELECT latest(timestamp) FROM Log WHERE collector.name = 'database-intelligence' SINCE 10 minutes ago",
					Interval: 5 * time.Minute,
				},
			},
		},
	}
}