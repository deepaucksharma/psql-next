package redis

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the Redis enhanced receiver
type Config struct {
	// Endpoint is the Redis endpoint to connect to
	Endpoint string `mapstructure:"endpoint"`

	// Password for authentication
	Password string `mapstructure:"password"`

	// Username for Redis ACL (Redis 6.0+)
	Username string `mapstructure:"username"`

	// Database number to connect to
	Database int `mapstructure:"database"`

	// CollectionInterval is how often to collect metrics
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// Cluster configuration
	Cluster ClusterConfig `mapstructure:"cluster"`

	// Sentinel configuration
	Sentinel SentinelConfig `mapstructure:"sentinel"`

	// Metrics configuration
	Metrics MetricsConfig `mapstructure:"metrics"`

	// SlowLog configuration
	SlowLog SlowLogConfig `mapstructure:"slow_log"`

	// Connection configuration
	MaxConns        int           `mapstructure:"max_conns"`
	MinIdleConns    int           `mapstructure:"min_idle_conns"`
	MaxRetries      int           `mapstructure:"max_retries"`
	ConnectTimeout  time.Duration `mapstructure:"connect_timeout"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	PoolTimeout     time.Duration `mapstructure:"pool_timeout"`

	// TLS configuration
	TLS TLSConfig `mapstructure:"tls"`

	// Resource attributes to add to all metrics
	ResourceAttributes map[string]string `mapstructure:"resource_attributes"`
}

// ClusterConfig defines Redis cluster configuration
type ClusterConfig struct {
	// Enabled controls whether to use cluster mode
	Enabled bool `mapstructure:"enabled"`

	// Nodes is a list of cluster nodes (overrides Endpoint)
	Nodes []string `mapstructure:"nodes"`

	// CollectPerNodeMetrics enables per-node metric collection
	CollectPerNodeMetrics bool `mapstructure:"collect_per_node_metrics"`

	// CollectClusterInfo enables cluster info collection
	CollectClusterInfo bool `mapstructure:"collect_cluster_info"`

	// CollectSlotMetrics enables slot distribution metrics
	CollectSlotMetrics bool `mapstructure:"collect_slot_metrics"`

	// MaxRedirects for cluster operations
	MaxRedirects int `mapstructure:"max_redirects"`

	// ReadOnly allows read operations on slave nodes
	ReadOnly bool `mapstructure:"read_only"`

	// RouteByLatency routes read operations to lowest latency node
	RouteByLatency bool `mapstructure:"route_by_latency"`

	// RouteRandomly routes read operations randomly
	RouteRandomly bool `mapstructure:"route_randomly"`
}

// SentinelConfig defines Redis Sentinel configuration
type SentinelConfig struct {
	// Enabled controls whether to use Sentinel mode
	Enabled bool `mapstructure:"enabled"`

	// MasterName is the name of the master to monitor
	MasterName string `mapstructure:"master_name"`

	// SentinelAddrs is a list of sentinel addresses
	SentinelAddrs []string `mapstructure:"sentinel_addrs"`

	// SentinelPassword for sentinel authentication
	SentinelPassword string `mapstructure:"sentinel_password"`

	// CollectSentinelMetrics enables sentinel-specific metrics
	CollectSentinelMetrics bool `mapstructure:"collect_sentinel_metrics"`
}

// MetricsConfig defines which metrics to collect
type MetricsConfig struct {
	// ServerInfo sections to collect
	ServerInfo ServerInfoConfig `mapstructure:"server_info"`

	// CommandStats enables command statistics collection
	CommandStats bool `mapstructure:"command_stats"`

	// KeyspaceStats enables keyspace statistics
	KeyspaceStats bool `mapstructure:"keyspace_stats"`

	// LatencyStats enables latency statistics
	LatencyStats bool `mapstructure:"latency_stats"`

	// MemoryStats enables detailed memory statistics
	MemoryStats bool `mapstructure:"memory_stats"`

	// ClientList enables client connection details
	ClientList bool `mapstructure:"client_list"`

	// ModuleList enables module information
	ModuleList bool `mapstructure:"module_list"`

	// StreamMetrics enables stream-specific metrics
	StreamMetrics bool `mapstructure:"stream_metrics"`

	// Custom commands to run for metrics
	CustomCommands []CustomCommand `mapstructure:"custom_commands"`
}

// ServerInfoConfig defines which INFO sections to collect
type ServerInfoConfig struct {
	// Server section
	Server bool `mapstructure:"server"`

	// Clients section
	Clients bool `mapstructure:"clients"`

	// Memory section
	Memory bool `mapstructure:"memory"`

	// Persistence section
	Persistence bool `mapstructure:"persistence"`

	// Stats section
	Stats bool `mapstructure:"stats"`

	// Replication section
	Replication bool `mapstructure:"replication"`

	// CPU section
	CPU bool `mapstructure:"cpu"`

	// Cluster section
	Cluster bool `mapstructure:"cluster"`

	// Keyspace section
	Keyspace bool `mapstructure:"keyspace"`
}

// SlowLogConfig defines slow log monitoring configuration
type SlowLogConfig struct {
	// Enabled controls whether to collect slow log entries
	Enabled bool `mapstructure:"enabled"`

	// MaxEntries to fetch per collection
	MaxEntries int `mapstructure:"max_entries"`

	// IncludeCommands includes full command in metrics
	IncludeCommands bool `mapstructure:"include_commands"`

	// TrackPosition tracks position in slow log
	TrackPosition bool `mapstructure:"track_position"`
}

// CustomCommand defines a custom Redis command for metrics
type CustomCommand struct {
	// Name of the metric
	Name string `mapstructure:"name"`

	// Command to execute
	Command string `mapstructure:"command"`

	// Args for the command
	Args []string `mapstructure:"args"`

	// Type of metric (gauge, counter, histogram)
	Type string `mapstructure:"type"`

	// Description of the metric
	Description string `mapstructure:"description"`

	// ValueExtractor to extract numeric value from response
	ValueExtractor string `mapstructure:"value_extractor"`

	// Labels to extract from response
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

	// ServerName for TLS verification
	ServerName string `mapstructure:"server_name"`
}

// Validate checks the configuration
func (cfg *Config) Validate() error {
	// Validate endpoint or cluster nodes
	if !cfg.Cluster.Enabled && !cfg.Sentinel.Enabled && cfg.Endpoint == "" {
		return errors.New("endpoint is required when not using cluster or sentinel mode")
	}

	if cfg.Cluster.Enabled && len(cfg.Cluster.Nodes) == 0 {
		return errors.New("cluster.nodes is required when cluster mode is enabled")
	}

	if cfg.Sentinel.Enabled {
		if cfg.Sentinel.MasterName == "" {
			return errors.New("sentinel.master_name is required when sentinel mode is enabled")
		}
		if len(cfg.Sentinel.SentinelAddrs) == 0 {
			return errors.New("sentinel.sentinel_addrs is required when sentinel mode is enabled")
		}
	}

	// Validate endpoint format
	if cfg.Endpoint != "" {
		if _, err := url.Parse("redis://" + cfg.Endpoint); err != nil {
			return fmt.Errorf("invalid endpoint format: %w", err)
		}
	}

	// Validate collection interval
	if cfg.CollectionInterval <= 0 {
		return errors.New("collection_interval must be positive")
	}

	// Validate connection pool settings
	if cfg.MaxConns > 0 && cfg.MinIdleConns > cfg.MaxConns {
		return errors.New("min_idle_conns cannot be greater than max_conns")
	}

	// Validate cluster configuration
	if cfg.Cluster.Enabled {
		if cfg.Cluster.MaxRedirects < 0 {
			cfg.Cluster.MaxRedirects = 3
		}
		for _, node := range cfg.Cluster.Nodes {
			if _, err := url.Parse("redis://" + node); err != nil {
				return fmt.Errorf("invalid cluster node format %s: %w", node, err)
			}
		}
	}

	// Validate slow log configuration
	if cfg.SlowLog.Enabled {
		if cfg.SlowLog.MaxEntries <= 0 {
			cfg.SlowLog.MaxEntries = 128
		}
	}

	// Validate custom commands
	for i, cmd := range cfg.Metrics.CustomCommands {
		if cmd.Name == "" {
			return fmt.Errorf("custom_commands[%d]: name is required", i)
		}
		if cmd.Command == "" {
			return fmt.Errorf("custom_commands[%d]: command is required", i)
		}
		switch cmd.Type {
		case "gauge", "counter", "histogram":
			// Valid types
		default:
			return fmt.Errorf("custom_commands[%d]: invalid type %s", i, cmd.Type)
		}
	}

	// Validate TLS configuration
	if cfg.TLS.Enabled {
		if cfg.TLS.CertFile != "" && cfg.TLS.KeyFile == "" {
			return errors.New("tls.key_file is required when tls.cert_file is specified")
		}
		if cfg.TLS.KeyFile != "" && cfg.TLS.CertFile == "" {
			return errors.New("tls.cert_file is required when tls.key_file is specified")
		}
	}

	return nil
}

// getEndpointMasked returns endpoint with password masked
func (cfg *Config) getEndpointMasked() string {
	if cfg.Password != "" {
		return cfg.Endpoint + " (password masked)"
	}
	return cfg.Endpoint
}

// CreateDefaultConfig creates the default configuration
func createDefaultConfig() component.Config {
	return &Config{
		Endpoint:           "localhost:6379",
		Database:           0,
		CollectionInterval: 60 * time.Second,
		MaxConns:           10,
		MinIdleConns:       0,
		MaxRetries:         3,
		ConnectTimeout:     5 * time.Second,
		ReadTimeout:        3 * time.Second,
		WriteTimeout:       3 * time.Second,
		PoolTimeout:        4 * time.Second,
		Cluster: ClusterConfig{
			Enabled:               false,
			CollectPerNodeMetrics: true,
			CollectClusterInfo:    true,
			CollectSlotMetrics:    true,
			MaxRedirects:          3,
			ReadOnly:              false,
			RouteByLatency:        true,
			RouteRandomly:         false,
		},
		Sentinel: SentinelConfig{
			Enabled:                false,
			CollectSentinelMetrics: true,
		},
		Metrics: MetricsConfig{
			ServerInfo: ServerInfoConfig{
				Server:      true,
				Clients:     true,
				Memory:      true,
				Persistence: true,
				Stats:       true,
				Replication: true,
				CPU:         true,
				Cluster:     true,
				Keyspace:    true,
			},
			CommandStats:  true,
			KeyspaceStats: true,
			LatencyStats:  true,
			MemoryStats:   true,
			ClientList:    false, // Can be expensive
			ModuleList:    true,
			StreamMetrics: true,
		},
		SlowLog: SlowLogConfig{
			Enabled:         false,
			MaxEntries:      128,
			IncludeCommands: true,
			TrackPosition:   true,
		},
		TLS: TLSConfig{
			Enabled:  false,
			Insecure: false,
		},
	}
}