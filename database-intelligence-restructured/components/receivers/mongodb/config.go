package mongodb

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the MongoDB enhanced receiver
type Config struct {
	// URI is the MongoDB connection string
	URI string `mapstructure:"uri"`

	// Database to monitor (if empty, monitors all databases)
	Database string `mapstructure:"database"`

	// Collections to monitor (if empty, monitors all collections)
	Collections []string `mapstructure:"collections"`

	// CollectionInterval is how often to collect metrics
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// ReplicaSet configuration
	ReplicaSet ReplicaSetConfig `mapstructure:"replica_set"`

	// Sharding configuration
	Sharding ShardingConfig `mapstructure:"sharding"`

	// Metrics configuration
	Metrics MetricsConfig `mapstructure:"metrics"`

	// Query monitoring configuration
	QueryMonitoring QueryMonitoringConfig `mapstructure:"query_monitoring"`

	// Connection pool settings
	MaxPoolSize    uint64        `mapstructure:"max_pool_size"`
	MinPoolSize    uint64        `mapstructure:"min_pool_size"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
	SocketTimeout  time.Duration `mapstructure:"socket_timeout"`

	// Resource attributes to add to all metrics
	ResourceAttributes map[string]string `mapstructure:"resource_attributes"`

	// TLS configuration
	TLS TLSConfig `mapstructure:"tls"`
}

// ReplicaSetConfig defines replica set monitoring configuration
type ReplicaSetConfig struct {
	// Enabled controls whether to collect replica set metrics
	Enabled bool `mapstructure:"enabled"`

	// CollectOplogMetrics enables oplog metrics collection
	CollectOplogMetrics bool `mapstructure:"collect_oplog_metrics"`

	// CollectReplLagMetrics enables replication lag metrics
	CollectReplLagMetrics bool `mapstructure:"collect_repl_lag_metrics"`

	// OplogWindow defines how far back to look in oplog
	OplogWindow time.Duration `mapstructure:"oplog_window"`

	// LagThreshold for alerting on replication lag
	LagThreshold time.Duration `mapstructure:"lag_threshold"`
}

// ShardingConfig defines sharding monitoring configuration
type ShardingConfig struct {
	// Enabled controls whether to collect sharding metrics
	Enabled bool `mapstructure:"enabled"`

	// CollectChunkMetrics enables chunk distribution metrics
	CollectChunkMetrics bool `mapstructure:"collect_chunk_metrics"`

	// CollectBalancerMetrics enables balancer metrics
	CollectBalancerMetrics bool `mapstructure:"collect_balancer_metrics"`

	// ChunkMetricsInterval for chunk distribution checks
	ChunkMetricsInterval time.Duration `mapstructure:"chunk_metrics_interval"`
}

// MetricsConfig defines which metrics to collect
type MetricsConfig struct {
	// ServerStatus metrics
	ServerStatus bool `mapstructure:"server_status"`

	// DatabaseStats metrics
	DatabaseStats bool `mapstructure:"database_stats"`

	// CollectionStats metrics
	CollectionStats bool `mapstructure:"collection_stats"`

	// IndexStats metrics
	IndexStats bool `mapstructure:"index_stats"`

	// CurrentOp metrics
	CurrentOp bool `mapstructure:"current_op"`

	// Profile metrics
	Profile bool `mapstructure:"profile"`

	// WiredTiger metrics
	WiredTiger bool `mapstructure:"wired_tiger"`

	// Custom metrics definitions
	CustomMetrics []CustomMetricConfig `mapstructure:"custom_metrics"`
}

// QueryMonitoringConfig defines query monitoring configuration
type QueryMonitoringConfig struct {
	// Enabled controls whether to monitor queries
	Enabled bool `mapstructure:"enabled"`

	// ProfileLevel (0=off, 1=slow ops, 2=all)
	ProfileLevel int `mapstructure:"profile_level"`

	// SlowOpThreshold for profiling
	SlowOpThreshold time.Duration `mapstructure:"slow_op_threshold"`

	// MaxQueries to track
	MaxQueries int `mapstructure:"max_queries"`

	// CollectQueryPlans whether to collect query plans
	CollectQueryPlans bool `mapstructure:"collect_query_plans"`

	// CollectQueryShapes whether to normalize queries
	CollectQueryShapes bool `mapstructure:"collect_query_shapes"`
}

// CustomMetricConfig defines a custom metric to collect
type CustomMetricConfig struct {
	// Name of the metric
	Name string `mapstructure:"name"`

	// Command to run (e.g., "dbStats", "collStats")
	Command string `mapstructure:"command"`

	// Database to run command against
	Database string `mapstructure:"database"`

	// Collection for collection-level commands
	Collection string `mapstructure:"collection"`

	// Path to extract value from result (JSON path)
	ValuePath string `mapstructure:"value_path"`

	// Type of metric (gauge, counter, histogram)
	Type string `mapstructure:"type"`

	// Description of the metric
	Description string `mapstructure:"description"`

	// Labels to extract from result
	Labels map[string]string `mapstructure:"labels"`
}

// TLSConfig defines TLS configuration
type TLSConfig struct {
	// Enabled controls whether to use TLS
	Enabled bool `mapstructure:"enabled"`

	// CAFile path to CA certificate
	CAFile string `mapstructure:"ca_file"`

	// CertFile path to client certificate
	CertFile string `mapstructure:"cert_file"`

	// KeyFile path to client key
	KeyFile string `mapstructure:"key_file"`

	// Insecure allows insecure TLS connections
	Insecure bool `mapstructure:"insecure"`
}

// Validate checks the configuration
func (cfg *Config) Validate() error {
	if cfg.URI == "" {
		return errors.New("uri is required")
	}

	// Validate URI format
	u, err := url.Parse(cfg.URI)
	if err != nil {
		return fmt.Errorf("invalid uri: %w", err)
	}

	if u.Scheme != "mongodb" && u.Scheme != "mongodb+srv" {
		return fmt.Errorf("uri must use mongodb:// or mongodb+srv:// scheme")
	}

	// Validate collection interval
	if cfg.CollectionInterval <= 0 {
		return errors.New("collection_interval must be positive")
	}

	// Validate connection pool settings
	if cfg.MaxPoolSize > 0 && cfg.MinPoolSize > cfg.MaxPoolSize {
		return errors.New("min_pool_size cannot be greater than max_pool_size")
	}

	// Validate replica set configuration
	if cfg.ReplicaSet.Enabled {
		if cfg.ReplicaSet.OplogWindow <= 0 {
			cfg.ReplicaSet.OplogWindow = 1 * time.Hour
		}
		if cfg.ReplicaSet.LagThreshold <= 0 {
			cfg.ReplicaSet.LagThreshold = 30 * time.Second
		}
	}

	// Validate sharding configuration
	if cfg.Sharding.Enabled {
		if cfg.Sharding.ChunkMetricsInterval <= 0 {
			cfg.Sharding.ChunkMetricsInterval = 5 * time.Minute
		}
	}

	// Validate query monitoring
	if cfg.QueryMonitoring.Enabled {
		if cfg.QueryMonitoring.ProfileLevel < 0 || cfg.QueryMonitoring.ProfileLevel > 2 {
			return errors.New("profile_level must be 0, 1, or 2")
		}
		if cfg.QueryMonitoring.SlowOpThreshold <= 0 {
			cfg.QueryMonitoring.SlowOpThreshold = 100 * time.Millisecond
		}
		if cfg.QueryMonitoring.MaxQueries <= 0 {
			cfg.QueryMonitoring.MaxQueries = 1000
		}
	}

	// Validate custom metrics
	for i, metric := range cfg.Metrics.CustomMetrics {
		if metric.Name == "" {
			return fmt.Errorf("custom_metrics[%d]: name is required", i)
		}
		if metric.Command == "" {
			return fmt.Errorf("custom_metrics[%d]: command is required", i)
		}
		if metric.ValuePath == "" {
			return fmt.Errorf("custom_metrics[%d]: value_path is required", i)
		}
		switch metric.Type {
		case "gauge", "counter", "histogram":
			// Valid types
		default:
			return fmt.Errorf("custom_metrics[%d]: invalid type %s", i, metric.Type)
		}
	}

	// Validate TLS configuration
	if cfg.TLS.Enabled {
		if cfg.TLS.CAFile == "" && !cfg.TLS.Insecure {
			return errors.New("tls.ca_file is required when TLS is enabled and not insecure")
		}
		if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile == "" {
			return errors.New("tls.key_file is required when tls.cert_file is specified")
		}
		if cfg.TLS.KeyFile != "" && cfg.TLS.CertFile == "" {
			return errors.New("tls.cert_file is required when tls.key_file is specified")
		}
	}

	return nil
}

// getURIMasked returns URI with password masked
func (cfg *Config) getURIMasked() string {
	u, err := url.Parse(cfg.URI)
	if err != nil {
		return cfg.URI
	}

	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "***")
		}
	}

	return u.String()
}

// CreateDefaultConfig creates the default configuration
func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval: 60 * time.Second,
		MaxPoolSize:        100,
		MinPoolSize:        0,
		ConnectTimeout:     10 * time.Second,
		SocketTimeout:      0,
		ReplicaSet: ReplicaSetConfig{
			Enabled:               true,
			CollectOplogMetrics:   true,
			CollectReplLagMetrics: true,
			OplogWindow:           1 * time.Hour,
			LagThreshold:          30 * time.Second,
		},
		Sharding: ShardingConfig{
			Enabled:                false,
			CollectChunkMetrics:    true,
			CollectBalancerMetrics: true,
			ChunkMetricsInterval:   5 * time.Minute,
		},
		Metrics: MetricsConfig{
			ServerStatus:    true,
			DatabaseStats:   true,
			CollectionStats: true,
			IndexStats:      true,
			CurrentOp:       true,
			Profile:         false,
			WiredTiger:      true,
		},
		QueryMonitoring: QueryMonitoringConfig{
			Enabled:            false,
			ProfileLevel:       1,
			SlowOpThreshold:    100 * time.Millisecond,
			MaxQueries:         1000,
			CollectQueryPlans:  true,
			CollectQueryShapes: true,
		},
		TLS: TLSConfig{
			Enabled:  false,
			Insecure: false,
		},
	}
}

// extractHost extracts the primary host from the MongoDB URI
func (cfg *Config) extractHost() string {
	u, err := url.Parse(cfg.URI)
	if err != nil {
		return "unknown"
	}

	// For SRV URIs, just return the hostname
	if u.Scheme == "mongodb+srv" {
		return u.Host
	}

	// For regular URIs, return the first host
	hosts := strings.Split(u.Host, ",")
	if len(hosts) > 0 {
		return hosts[0]
	}

	return "unknown"
}