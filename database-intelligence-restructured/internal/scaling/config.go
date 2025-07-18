package scaling

import (
	"fmt"
	"time"
)

// Config configures the horizontal scaling system
type Config struct {
	// Enabled enables horizontal scaling
	Enabled bool `mapstructure:"enabled"`
	
	// Mode specifies the coordination mode: "memory" or "redis"
	Mode string `mapstructure:"mode"`
	
	// NodeID is the unique identifier for this node
	// If not specified, a UUID will be generated
	NodeID string `mapstructure:"node_id"`
	
	// Coordinator configuration
	Coordinator CoordinatorSettings `mapstructure:"coordinator"`
	
	// Redis configuration (only used when mode is "redis")
	Redis RedisSettings `mapstructure:"redis"`
	
	// Receiver-specific scaling configuration
	ReceiverScaling ReceiverScalingSettings `mapstructure:"receiver_scaling"`
}

// CoordinatorSettings configures the coordinator
type CoordinatorSettings struct {
	// HeartbeatInterval is how often nodes send heartbeats
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	
	// NodeTimeout is how long before a node is considered dead
	NodeTimeout time.Duration `mapstructure:"node_timeout"`
	
	// RebalanceInterval is how often to check for rebalancing
	RebalanceInterval time.Duration `mapstructure:"rebalance_interval"`
	
	// MinRebalanceInterval is the minimum time between rebalances
	MinRebalanceInterval time.Duration `mapstructure:"min_rebalance_interval"`
}

// RedisSettings configures Redis storage
type RedisSettings struct {
	// Address is the Redis server address
	Address string `mapstructure:"address"`
	
	// Password is the Redis password (optional)
	Password string `mapstructure:"password"`
	
	// DB is the Redis database number
	DB int `mapstructure:"db"`
	
	// KeyPrefix is the prefix for all Redis keys
	KeyPrefix string `mapstructure:"key_prefix"`
	
	// LeaderTTL is the TTL for leader election
	LeaderTTL time.Duration `mapstructure:"leader_ttl"`
}

// ReceiverScalingSettings configures receiver-specific scaling
type ReceiverScalingSettings struct {
	// CheckInterval is how often receivers check assignments
	CheckInterval time.Duration `mapstructure:"check_interval"`
	
	// ResourcePrefix is the prefix for resource names
	ResourcePrefix string `mapstructure:"resource_prefix"`
	
	// IgnoreAssignments forces all resources to be processed (single-node mode)
	IgnoreAssignments bool `mapstructure:"ignore_assignments"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled: false,
		Mode:    "memory",
		Coordinator: CoordinatorSettings{
			HeartbeatInterval:    30 * time.Second,
			NodeTimeout:          90 * time.Second,
			RebalanceInterval:    5 * time.Minute,
			MinRebalanceInterval: 1 * time.Minute,
		},
		Redis: RedisSettings{
			Address:   "localhost:6379",
			DB:        0,
			KeyPrefix: "dbintel:scaling:",
			LeaderTTL: 30 * time.Second,
		},
		ReceiverScaling: ReceiverScalingSettings{
			CheckInterval:     30 * time.Second,
			ResourcePrefix:    "",
			IgnoreAssignments: false,
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	
	// Validate mode
	if c.Mode != "memory" && c.Mode != "redis" {
		return fmt.Errorf("invalid mode: %s (must be 'memory' or 'redis')", c.Mode)
	}
	
	// Validate Redis settings if using Redis mode
	if c.Mode == "redis" {
		if c.Redis.Address == "" {
			return fmt.Errorf("redis address is required when using redis mode")
		}
	}
	
	// Validate coordinator settings
	if c.Coordinator.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat interval must be positive")
	}
	if c.Coordinator.NodeTimeout <= 0 {
		return fmt.Errorf("node timeout must be positive")
	}
	if c.Coordinator.NodeTimeout < c.Coordinator.HeartbeatInterval {
		return fmt.Errorf("node timeout must be greater than heartbeat interval")
	}
	if c.Coordinator.RebalanceInterval <= 0 {
		return fmt.Errorf("rebalance interval must be positive")
	}
	if c.Coordinator.MinRebalanceInterval <= 0 {
		return fmt.Errorf("min rebalance interval must be positive")
	}
	
	// Validate receiver scaling settings
	if c.ReceiverScaling.CheckInterval <= 0 {
		return fmt.Errorf("receiver check interval must be positive")
	}
	
	return nil
}

// BuildCoordinatorConfig builds the coordinator configuration
func (c *Config) BuildCoordinatorConfig() *CoordinatorConfig {
	return &CoordinatorConfig{
		NodeID:               c.NodeID,
		HeartbeatInterval:    c.Coordinator.HeartbeatInterval,
		NodeTimeout:          c.Coordinator.NodeTimeout,
		RebalanceInterval:    c.Coordinator.RebalanceInterval,
		MinRebalanceInterval: c.Coordinator.MinRebalanceInterval,
	}
}

// BuildScalingConfig builds the receiver scaling configuration
func (c *Config) BuildScalingConfig() *ScalingConfig {
	return &ScalingConfig{
		Enabled:           c.Enabled,
		CheckInterval:     c.ReceiverScaling.CheckInterval,
		ResourcePrefix:    c.ReceiverScaling.ResourcePrefix,
		IgnoreAssignments: c.ReceiverScaling.IgnoreAssignments,
	}
}

// BuildRedisConfig builds the Redis configuration
func (c *Config) BuildRedisConfig() *RedisConfig {
	return &RedisConfig{
		Address:   c.Redis.Address,
		Password:  c.Redis.Password,
		DB:        c.Redis.DB,
		KeyPrefix: c.Redis.KeyPrefix,
		LeaderTTL: c.Redis.LeaderTTL,
	}
}