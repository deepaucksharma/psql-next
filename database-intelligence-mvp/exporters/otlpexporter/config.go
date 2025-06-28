package otlpexporter

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for the OTLP exporter
type Config struct {
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`

	// Endpoint is the OTLP receiver endpoint (e.g., "localhost:4317")
	Endpoint string `mapstructure:"endpoint"`

	// Headers are additional headers to send with each request
	Headers map[string]string `mapstructure:"headers"`

	// Compression specifies the compression type to use
	Compression configcompression.CompressionType `mapstructure:"compression"`

	// Insecure disables TLS when true
	Insecure bool `mapstructure:"insecure"`

	// TLSSettings for secure connections
	TLSSettings configgrpc.TLSConfig `mapstructure:"tls"`

	// RetryConfig for handling transient failures
	RetryConfig *RetryConfig `mapstructure:"retry"`

	// PostgreSQL specific configuration
	DeploymentType string `mapstructure:"deployment_type"` // "self-managed", "rds", "aurora", "azure", "gcp"
	Region         string `mapstructure:"region"`

	// Data transformation settings
	Transform TransformConfig `mapstructure:"transform"`
}

// RetryConfig defines retry behavior
type RetryConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	InitialInterval time.Duration `mapstructure:"initial_interval"`
	MaxInterval     time.Duration `mapstructure:"max_interval"`
	MaxElapsedTime  time.Duration `mapstructure:"max_elapsed_time"`
}

// TransformConfig defines data transformation settings
type TransformConfig struct {
	// AddDatabaseLabels adds database name as a label to all metrics
	AddDatabaseLabels bool `mapstructure:"add_database_labels"`

	// NormalizeQueryText normalizes SQL query text
	NormalizeQueryText bool `mapstructure:"normalize_query_text"`

	// IncludeQueryPlans includes query execution plans
	IncludeQueryPlans bool `mapstructure:"include_query_plans"`

	// SanitizeSensitiveData removes sensitive information
	SanitizeSensitiveData bool `mapstructure:"sanitize_sensitive_data"`

	// MetricPrefix adds a prefix to all metric names
	MetricPrefix string `mapstructure:"metric_prefix"`
}

// Validate checks if the configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return errors.New("endpoint is required")
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.DeploymentType == "" {
		cfg.DeploymentType = "self-managed"
	}

	// Validate compression
	if cfg.Compression != "" && cfg.Compression != configcompression.Gzip && 
		cfg.Compression != configcompression.Zlib && cfg.Compression != configcompression.Deflate &&
		cfg.Compression != configcompression.Snappy && cfg.Compression != configcompression.Zstd {
		return errors.New("invalid compression type")
	}

	// Validate retry config
	if cfg.RetryConfig != nil {
		if cfg.RetryConfig.InitialInterval <= 0 {
			cfg.RetryConfig.InitialInterval = time.Second
		}
		if cfg.RetryConfig.MaxInterval <= 0 {
			cfg.RetryConfig.MaxInterval = 30 * time.Second
		}
		if cfg.RetryConfig.MaxElapsedTime <= 0 {
			cfg.RetryConfig.MaxElapsedTime = 5 * time.Minute
		}
	}

	return nil
}

// createDefaultConfig creates the default configuration for the exporter
func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: 30 * time.Second,
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 10,
			QueueSize:    5000,
		},
		Compression: configcompression.Gzip,
		RetryConfig: &RetryConfig{
			Enabled:         true,
			InitialInterval: time.Second,
			MaxInterval:     30 * time.Second,
			MaxElapsedTime:  5 * time.Minute,
		},
		Transform: TransformConfig{
			AddDatabaseLabels:     true,
			NormalizeQueryText:    true,
			IncludeQueryPlans:     true,
			SanitizeSensitiveData: true,
			MetricPrefix:          "postgresql.",
		},
	}
}