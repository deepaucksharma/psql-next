package postgresqlquery

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const (
	// TypeStr is the type string for the PostgreSQL query receiver
	TypeStr = "postgresqlquery"
	
	// Stability level
	stability = component.StabilityLevelBeta
)

// NewFactory creates a new PostgreSQL query receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		TypeStr,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, stability),
		receiver.WithMetrics(createMetricsReceiver, stability),
	)
}

// createDefaultConfig creates the default configuration for the receiver
func createDefaultConfig() component.Config {
	return &Config{
		CollectionInterval:  "60s",
		Timeout:            "3000ms",
		MaxOpenConnections: 2,
		MaxIdleConnections: 1,
		MaxIdleTime:        "10m",
		
		// Enhanced query configuration
		QueryConfig: QueryConfig{
			MaxQueriesPerCollection: 100,
			MinQueryTimeMs:         10,
			EnableExplainPlan:      false, // Disabled by default for safety
			ExplainSafetyChecks:    true,
			MaxDatabaseSizeBytes:   100 * 1024 * 1024 * 1024, // 100GB
		},
		
		// Unified adaptive sampling configuration
		AdaptiveSampling: UnifiedAdaptiveSamplerConfig{
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
					MaxDurationMs: -1,
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
			},
			
			ResourceRules: []ResourceRule{
				{
					Name:               "high_temp_usage",
					MinTempBlocksWrite: 10000,
					SamplingRate:       1.0,
					Priority:           90,
				},
			},
			
			DeduplicationEnabled:  true,
			CircuitBreakerEnabled: true,
			ErrorThreshold:        10,
		},
		
		// Circuit breaker configuration
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:                  true,
			FailureThreshold:         5,
			RecoveryTimeout:          "30s",
			ConsecutiveFailures:      3,
			HalfOpenMaxRequests:      10,
		},
		
		// Attribute mapping for processor compatibility
		AttributeMapping: map[string]string{
			"postgresql.query.mean_time":        "avg_duration_ms",
			"postgresql.query.calls":            "execution_count",
			"postgresql.query.rows":             "rows_affected",
			"postgresql.query.shared_blks_hit":  "shared_blocks_hit",
			"postgresql.query.shared_blks_read": "shared_blocks_read",
			"postgresql.query.temp_blks_written": "temp_blocks_written",
			"db.name":                          "database_name",
			"query_id":                         "query_id",
			"query.performance_category":       "performance_category",
		},
		
		// Resource attributes
		ResourceAttributes: map[string]string{
			"service.name":   "database-intelligence-mvp",
			"db.system":      "postgresql",
			"receiver.type":  TypeStr,
			"receiver.version": "2.0.0",
		},
	}
}

// createLogsReceiver creates a logs receiver
func createLogsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	config := cfg.(*Config)
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Create the receiver with enhanced capabilities
	r := &postgresqlQueryReceiver{
		config:           config,
		logger:           params.Logger,
		logsConsumer:     consumer,
		
		// Initialize enhanced components
		adaptiveSampler:  nil, // Will be initialized in Start()
		circuitBreaker:   nil, // Will be initialized in Start()
		planAnalyzer:     nil, // Will be initialized in Start()
		attributeMapper:  createAttributeMapper(config.AttributeMapping),
		statsCollector:   newStatsCollector(),
	}
	
	return receiverhelper.NewLogsReceiver(
		ctx,
		params,
		cfg,
		consumer,
		r.Start,
		r.Shutdown,
		receiverhelper.WithStart(r.Start),
		receiverhelper.WithShutdown(r.Shutdown),
	)
}

// createMetricsReceiver creates a metrics receiver
func createMetricsReceiver(
	ctx context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	config := cfg.(*Config)
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Create the receiver with metrics support
	r := &postgresqlQueryReceiver{
		config:           config,
		logger:           params.Logger,
		metricsConsumer:  consumer,
		
		// Initialize enhanced components
		adaptiveSampler:  nil, // Will be initialized in Start()
		circuitBreaker:   nil, // Will be initialized in Start()
		planAnalyzer:     nil, // Will be initialized in Start()
		attributeMapper:  createAttributeMapper(config.AttributeMapping),
		statsCollector:   newStatsCollector(),
	}
	
	return receiverhelper.NewMetricsReceiver(
		ctx,
		params,
		cfg,
		consumer,
		r.Start,
		r.Shutdown,
		receiverhelper.WithStart(r.Start),
		receiverhelper.WithShutdown(r.Shutdown),
	)
}

// createAttributeMapper creates an attribute mapper from configuration
func createAttributeMapper(mappings map[string]string) *AttributeMapper {
	return &AttributeMapper{
		mappings: mappings,
	}
}

// AttributeMapper handles attribute name mapping for processor compatibility
type AttributeMapper struct {
	mappings map[string]string
}

// MapAttributes maps receiver attributes to processor-expected names
func (m *AttributeMapper) MapAttributes(attrs map[string]interface{}) map[string]interface{} {
	mapped := make(map[string]interface{})
	
	// Copy all original attributes
	for k, v := range attrs {
		mapped[k] = v
	}
	
	// Apply mappings
	for from, to := range m.mappings {
		if val, exists := attrs[from]; exists {
			mapped[to] = val
		}
	}
	
	return mapped
}

// ValidateProcessorCompatibility checks if attributes are compatible with processors
func (m *AttributeMapper) ValidateProcessorCompatibility(attrs map[string]interface{}) []string {
	var missing []string
	
	// Check for required processor attributes
	requiredAttrs := []string{
		"avg_duration_ms",
		"database_name",
		"query_id",
	}
	
	for _, required := range requiredAttrs {
		if _, exists := attrs[required]; !exists {
			// Check if it can be mapped
			canMap := false
			for from, to := range m.mappings {
				if to == required {
					if _, exists := attrs[from]; exists {
						canMap = true
						break
					}
				}
			}
			if !canMap {
				missing = append(missing, required)
			}
		}
	}
	
	return missing
}
