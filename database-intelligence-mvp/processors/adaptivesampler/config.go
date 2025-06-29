package adaptivesampler

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config represents the configuration for the adaptive sampler processor
type Config struct {
	// StateStorage configures persistent state management
	StateStorage StateStorageConfig `mapstructure:"state_storage"`

	// Deduplication settings
	Deduplication DeduplicationConfig `mapstructure:"deduplication"`

	// SamplingRules define the sampling strategy
	SamplingRules []SamplingRule `mapstructure:"rules"`

	// DefaultSampleRate is used when no rules match
	DefaultSampleRate float64 `mapstructure:"default_sample_rate"`

	// MaxRecordsPerSecond limits throughput for safety
	MaxRecordsPerSecond int `mapstructure:"max_records_per_second"`

	// EnableDebugLogging enables detailed debug output
	EnableDebugLogging bool `mapstructure:"enable_debug_logging"`
}

// StateStorageConfig configures how state is stored
type StateStorageConfig struct {
	// Type of storage: "file_storage" only for MVP
	Type string `mapstructure:"type"`

	// FileStorage configuration for file-based storage
	FileStorage FileStorageConfig `mapstructure:"file_storage"`
}

// FileStorageConfig configures file-based state storage
type FileStorageConfig struct {
	// Directory where state files are stored
	Directory string `mapstructure:"directory"`

	// SyncInterval how often to sync to disk
	SyncInterval time.Duration `mapstructure:"sync_interval"`

	// CompactionInterval how often to compact state files
	CompactionInterval time.Duration `mapstructure:"compaction_interval"`

	// MaxSizeMB maximum size of state files before compaction
	MaxSizeMB int `mapstructure:"max_size_mb"`

	// BackupRetention how many backup files to keep
	BackupRetention int `mapstructure:"backup_retention"`
}

// DeduplicationConfig configures duplicate detection
type DeduplicationConfig struct {
	// Enabled turns on deduplication
	Enabled bool `mapstructure:"enabled"`

	// CacheSize maximum number of hashes to track in memory
	CacheSize int `mapstructure:"cache_size"`

	// WindowSeconds time window for deduplication
	WindowSeconds int `mapstructure:"window_seconds"`

	// HashAttribute name of attribute containing plan hash
	HashAttribute string `mapstructure:"hash_attribute"`

	// CleanupInterval how often to clean expired hashes
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// SamplingRule defines a sampling strategy
type SamplingRule struct {
	// Name of the rule for debugging
	Name string `mapstructure:"name"`

	// Priority determines evaluation order (higher = first)
	Priority int `mapstructure:"priority"`

	// SampleRate probability of keeping the record (0.0 to 1.0)
	SampleRate float64 `mapstructure:"sample_rate"`

	// Conditions that must be met for this rule to apply
	Conditions []SamplingCondition `mapstructure:"conditions"`

	// MaxPerMinute limits records matched by this rule
	MaxPerMinute int `mapstructure:"max_per_minute,omitempty"`
}

// SamplingCondition defines a condition for sampling
type SamplingCondition struct {
	// Attribute name to evaluate
	Attribute string `mapstructure:"attribute"`

	// Operator for comparison
	Operator string `mapstructure:"operator"`

	// Value to compare against
	Value interface{} `mapstructure:"value"`
}

// Validate checks the processor configuration
func (cfg *Config) Validate() error {
	if cfg.StateStorage.Type != "file_storage" {
		return fmt.Errorf("only file_storage is supported for state storage, got: %s", cfg.StateStorage.Type)
	}

	if cfg.StateStorage.FileStorage.Directory == "" {
		return fmt.Errorf("file storage directory must be specified")
	}

	if cfg.StateStorage.FileStorage.MaxSizeMB <= 0 {
		return fmt.Errorf("max_size_mb must be positive, got: %d", cfg.StateStorage.FileStorage.MaxSizeMB)
	}

	if cfg.DefaultSampleRate < 0.0 || cfg.DefaultSampleRate > 1.0 {
		return fmt.Errorf("default_sample_rate must be between 0.0 and 1.0, got: %f", cfg.DefaultSampleRate)
	}

	if cfg.MaxRecordsPerSecond <= 0 {
		return fmt.Errorf("max_records_per_second must be positive, got: %d", cfg.MaxRecordsPerSecond)
	}

	if cfg.Deduplication.Enabled {
		if cfg.Deduplication.CacheSize <= 0 {
			return fmt.Errorf("deduplication cache_size must be positive, got: %d", cfg.Deduplication.CacheSize)
		}
		if cfg.Deduplication.WindowSeconds <= 0 {
			return fmt.Errorf("deduplication window_seconds must be positive, got: %d", cfg.Deduplication.WindowSeconds)
		}
		if cfg.Deduplication.HashAttribute == "" {
			return fmt.Errorf("deduplication hash_attribute must be specified")
		}
	}

	// Validate sampling rules
	for i, rule := range cfg.SamplingRules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("invalid sampling rule %d (%s): %w", i, rule.Name, err)
		}
	}

	return nil
}

// Validate checks a sampling rule
func (rule *SamplingRule) Validate() error {
	if rule.Name == "" {
		return fmt.Errorf("rule name cannot be empty")
	}

	if rule.SampleRate < 0.0 || rule.SampleRate > 1.0 {
		return fmt.Errorf("sample_rate must be between 0.0 and 1.0, got: %f", rule.SampleRate)
	}

	if rule.MaxPerMinute < 0 {
		return fmt.Errorf("max_per_minute cannot be negative, got: %d", rule.MaxPerMinute)
	}

	// Validate conditions
	for i, condition := range rule.Conditions {
		if err := condition.Validate(); err != nil {
			return fmt.Errorf("invalid condition %d: %w", i, err)
		}
	}

	return nil
}

// Validate checks a sampling condition
func (condition *SamplingCondition) Validate() error {
	if condition.Attribute == "" {
		return fmt.Errorf("condition attribute cannot be empty")
	}

	validOperators := map[string]bool{
		"eq":       true, // equals
		"ne":       true, // not equals
		"gt":       true, // greater than
		"gte":      true, // greater than or equal
		"lt":       true, // less than
		"lte":      true, // less than or equal
		"contains": true, // string contains
		"exists":   true, // attribute exists
	}

	if !validOperators[condition.Operator] {
		return fmt.Errorf("invalid operator: %s", condition.Operator)
	}

	if condition.Operator != "exists" && condition.Value == nil {
		return fmt.Errorf("value required for operator: %s", condition.Operator)
	}

	return nil
}

// createDefaultConfig creates a default configuration
func createDefaultConfig() component.Config {
	return &Config{
		StateStorage: StateStorageConfig{
			Type: "file_storage",
			FileStorage: FileStorageConfig{
				Directory:          "/var/lib/otel/sampling_state",
				SyncInterval:       10 * time.Second,
				CompactionInterval: 300 * time.Second, // 5 minutes
				MaxSizeMB:          100,
				BackupRetention:    3,
			},
		},
		Deduplication: DeduplicationConfig{
			Enabled:         true,
			CacheSize:       10000,
			WindowSeconds:   300, // 5 minutes
			HashAttribute:   "db.query.plan.hash",
			CleanupInterval: 60 * time.Second,
		},
		SamplingRules: []SamplingRule{
			{
				Name:       "critical_queries",
				Priority:   100,
				SampleRate: 1.0,
				Conditions: []SamplingCondition{
					{
						Attribute: "avg_duration_ms",
						Operator:  "gt",
						Value:     1000.0,
					},
				},
			},
			{
				Name:       "missing_indexes",
				Priority:   90,
				SampleRate: 1.0,
				Conditions: []SamplingCondition{
					{
						Attribute: "db.query.plan.has_seq_scan",
						Operator:  "eq",
						Value:     true,
					},
					{
						Attribute: "db.query.plan.rows",
						Operator:  "gt",
						Value:     10000.0,
					},
				},
			},
			{
				Name:         "high_frequency",
				Priority:     50,
				SampleRate:   0.01,
				MaxPerMinute: 10,
				Conditions: []SamplingCondition{
					{
						Attribute: "execution_count",
						Operator:  "gt",
						Value:     1000.0,
					},
				},
			},
			{
				Name:       "default",
				Priority:   0,
				SampleRate: 0.1,
			},
		},
		DefaultSampleRate:   0.1,
		MaxRecordsPerSecond: 1000,
		EnableDebugLogging:  false,
	}
}