package postgresqlquery

import (
	"time"
)

// UnifiedAdaptiveSamplerConfig provides configuration that can be shared
// between receiver's internal sampler and the adaptive sampler processor
type UnifiedAdaptiveSamplerConfig struct {
	// Base configuration
	Enabled               bool          `mapstructure:"enabled"`
	SlowQueryThresholdMs  float64       `mapstructure:"slow_query_threshold_ms"`
	BaseSamplingRate      float64       `mapstructure:"base_sampling_rate"`
	MaxSamplingRate       float64       `mapstructure:"max_sampling_rate"`
	SamplingWindowSeconds int           `mapstructure:"sampling_window_seconds"`
	
	// Coordination settings
	CoordinationMode      string        `mapstructure:"coordination_mode"` // "receiver", "processor", "hybrid"
	
	// Performance-based sampling rules
	PerformanceRules      []PerformanceRule `mapstructure:"performance_rules"`
	
	// Resource-based sampling rules
	ResourceRules         []ResourceRule    `mapstructure:"resource_rules"`
	
	// Database-specific overrides
	DatabaseOverrides     map[string]DatabaseSamplingConfig `mapstructure:"database_overrides"`
	
	// Deduplication settings
	DeduplicationEnabled  bool          `mapstructure:"deduplication_enabled"`
	DeduplicationWindow   time.Duration `mapstructure:"deduplication_window"`
	
	// Circuit breaker integration
	CircuitBreakerEnabled bool          `mapstructure:"circuit_breaker_enabled"`
	ErrorThreshold        int           `mapstructure:"error_threshold"`
	RecoveryTime          time.Duration `mapstructure:"recovery_time"`
}

// PerformanceRule defines sampling based on query performance
type PerformanceRule struct {
	Name            string  `mapstructure:"name"`
	MinDurationMs   float64 `mapstructure:"min_duration_ms"`
	MaxDurationMs   float64 `mapstructure:"max_duration_ms"`
	SamplingRate    float64 `mapstructure:"sampling_rate"`
	Priority        int     `mapstructure:"priority"`
}

// ResourceRule defines sampling based on resource usage
type ResourceRule struct {
	Name                string  `mapstructure:"name"`
	MinBlocksRead       int64   `mapstructure:"min_blocks_read"`
	MinTempBlocksWrite  int64   `mapstructure:"min_temp_blocks_write"`
	MinRows             int64   `mapstructure:"min_rows"`
	SamplingRate        float64 `mapstructure:"sampling_rate"`
	Priority            int     `mapstructure:"priority"`
}

// DatabaseSamplingConfig provides per-database sampling overrides
type DatabaseSamplingConfig struct {
	BaseSamplingRate     float64              `mapstructure:"base_sampling_rate"`
	MaxSamplingRate      float64              `mapstructure:"max_sampling_rate"`
	SlowQueryThresholdMs float64              `mapstructure:"slow_query_threshold_ms"`
	PerformanceRules     []PerformanceRule    `mapstructure:"performance_rules"`
	ResourceRules        []ResourceRule       `mapstructure:"resource_rules"`
}

// DefaultUnifiedAdaptiveSamplerConfig returns default configuration
func DefaultUnifiedAdaptiveSamplerConfig() UnifiedAdaptiveSamplerConfig {
	return UnifiedAdaptiveSamplerConfig{
		Enabled:               true,
		SlowQueryThresholdMs:  1000,
		BaseSamplingRate:      0.1,
		MaxSamplingRate:       1.0,
		SamplingWindowSeconds: 300,
		CoordinationMode:      "hybrid",
		
		PerformanceRules: []PerformanceRule{
			{
				Name:          "critical_slow_queries",
				MinDurationMs: 5000,
				MaxDurationMs: -1, // no upper limit
				SamplingRate:  1.0,
				Priority:      100,
			},
			{
				Name:          "slow_queries",
				MinDurationMs: 1000,
				MaxDurationMs: 5000,
				SamplingRate:  0.5,
				Priority:      50,
			},
			{
				Name:          "medium_queries",
				MinDurationMs: 100,
				MaxDurationMs: 1000,
				SamplingRate:  0.2,
				Priority:      30,
			},
		},
		
		ResourceRules: []ResourceRule{
			{
				Name:               "high_temp_usage",
				MinTempBlocksWrite: 10000,
				SamplingRate:       1.0,
				Priority:           90,
			},
			{
				Name:          "high_block_reads",
				MinBlocksRead: 100000,
				SamplingRate:  0.8,
				Priority:      80,
			},
			{
				Name:        "large_result_sets",
				MinRows:     1000000,
				SamplingRate: 0.7,
				Priority:    70,
			},
		},
		
		DeduplicationEnabled: true,
		DeduplicationWindow:  5 * time.Minute,
		
		CircuitBreakerEnabled: true,
		ErrorThreshold:        10,
		RecoveryTime:          30 * time.Second,
		
		DatabaseOverrides: make(map[string]DatabaseSamplingConfig),
	}
}

// ShouldSample determines if a query should be sampled based on unified rules
func (c *UnifiedAdaptiveSamplerConfig) ShouldSample(metrics QueryMetrics) (bool, float64) {
	// Check if sampling is enabled
	if !c.Enabled {
		return false, 0
	}
	
	// Get database-specific config if available
	dbConfig, hasOverride := c.DatabaseOverrides[metrics.DatabaseName]
	if !hasOverride {
		dbConfig = DatabaseSamplingConfig{
			BaseSamplingRate:     c.BaseSamplingRate,
			MaxSamplingRate:      c.MaxSamplingRate,
			SlowQueryThresholdMs: c.SlowQueryThresholdMs,
			PerformanceRules:     c.PerformanceRules,
			ResourceRules:        c.ResourceRules,
		}
	}
	
	// Start with base sampling rate
	samplingRate := dbConfig.BaseSamplingRate
	highestPriority := 0
	
	// Apply performance rules
	for _, rule := range dbConfig.PerformanceRules {
		if rule.Priority <= highestPriority {
			continue
		}
		
		if metrics.MeanDurationMs >= rule.MinDurationMs {
			if rule.MaxDurationMs < 0 || metrics.MeanDurationMs <= rule.MaxDurationMs {
				samplingRate = rule.SamplingRate
				highestPriority = rule.Priority
			}
		}
	}
	
	// Apply resource rules
	for _, rule := range dbConfig.ResourceRules {
		if rule.Priority <= highestPriority {
			continue
		}
		
		matches := true
		if rule.MinBlocksRead > 0 && metrics.SharedBlocksRead < rule.MinBlocksRead {
			matches = false
		}
		if rule.MinTempBlocksWrite > 0 && metrics.TempBlocksWritten < rule.MinTempBlocksWrite {
			matches = false
		}
		if rule.MinRows > 0 && metrics.RowsAffected < rule.MinRows {
			matches = false
		}
		
		if matches {
			samplingRate = rule.SamplingRate
			highestPriority = rule.Priority
		}
	}
	
	// Cap at max sampling rate
	if samplingRate > dbConfig.MaxSamplingRate {
		samplingRate = dbConfig.MaxSamplingRate
	}
	
	// Make sampling decision
	return samplingRate > 0, samplingRate
}

// QueryMetrics contains metrics for sampling decisions
type QueryMetrics struct {
	QueryID            string
	DatabaseName       string
	MeanDurationMs     float64
	ExecutionCount     int64
	RowsAffected       int64
	SharedBlocksHit    int64
	SharedBlocksRead   int64
	TempBlocksWritten  int64
	StddevDurationMs   float64
	MinDurationMs      float64
	MaxDurationMs      float64
	PerformanceCategory string
}

// GetAttributeMapping returns the attribute names expected by the processor
func (c *UnifiedAdaptiveSamplerConfig) GetAttributeMapping() map[string]string {
	return map[string]string{
		// Receiver attributes -> Processor expected attributes
		"postgresql.query.mean_time":      "avg_duration_ms",
		"postgresql.query.calls":          "execution_count",
		"postgresql.query.rows":           "rows_affected",
		"postgresql.query.shared_blks_hit": "shared_blocks_hit",
		"postgresql.query.shared_blks_read": "shared_blocks_read",
		"postgresql.query.temp_blks_written": "temp_blocks_written",
		"db.name":                        "database_name",
		"query_id":                       "query_id",
		"query.performance_category":     "performance_category",
	}
}