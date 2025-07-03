package planattributeextractor

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config represents the configuration for the plan attribute extractor processor
type Config struct {
	// TimeoutMS is the maximum time in milliseconds to spend processing each record
	TimeoutMS int `mapstructure:"timeout_ms"`

	// ErrorMode determines how to handle extraction errors
	// Valid values: "ignore", "propagate"
	ErrorMode string `mapstructure:"error_mode"`

	// PostgreSQLRules defines extraction rules for PostgreSQL plans
	PostgreSQLRules PostgreSQLExtractionRules `mapstructure:"postgresql_rules"`

	// MySQLRules defines extraction rules for MySQL metadata
	MySQLRules MySQLExtractionRules `mapstructure:"mysql_rules"`

	// HashConfig configures plan hash generation
	HashConfig HashGenerationConfig `mapstructure:"hash_config"`

	// EnableDebugLogging enables detailed debug logging
	EnableDebugLogging bool `mapstructure:"enable_debug_logging"`

	// UnsafePlanCollection enables direct EXPLAIN collection (NOT RECOMMENDED FOR PRODUCTION)
	UnsafePlanCollection bool `mapstructure:"unsafe_plan_collection"`

	// SafeMode ensures the processor only works with pre-collected plan data
	SafeMode bool `mapstructure:"safe_mode"`

	// QueryAnonymization configures query text anonymization
	QueryAnonymization QueryAnonymizationConfig `mapstructure:"query_anonymization"`

	// QueryLens configures pg_querylens integration
	QueryLens QueryLensConfig `mapstructure:"querylens"`
}

// PostgreSQLExtractionRules defines how to extract attributes from PostgreSQL JSON plans
type PostgreSQLExtractionRules struct {
	// DetectionJSONPath is the JSONPath to detect if this is a PostgreSQL plan
	DetectionJSONPath string `mapstructure:"detection_jsonpath"`

	// Extractions maps attribute names to JSONPath expressions
	Extractions map[string]string `mapstructure:"extractions"`

	// DerivedAttributes defines computed attributes
	DerivedAttributes map[string]string `mapstructure:"derived"`
}

// MySQLExtractionRules defines how to extract attributes from MySQL metadata
type MySQLExtractionRules struct {
	// DetectionJSONPath is the JSONPath to detect if this is MySQL metadata
	DetectionJSONPath string `mapstructure:"detection_jsonpath"`

	// Extractions maps attribute names to JSONPath expressions
	Extractions map[string]string `mapstructure:"extractions"`
}

// HashGenerationConfig configures how plan hashes are generated
type HashGenerationConfig struct {
	// Include specifies which attributes to include in the hash
	Include []string `mapstructure:"include"`

	// Output specifies the attribute name for the generated hash
	Output string `mapstructure:"output"`

	// Algorithm specifies the hash algorithm (only sha256 supported for security)
	Algorithm string `mapstructure:"algorithm"`
}

// QueryAnonymizationConfig configures query text anonymization
type QueryAnonymizationConfig struct {
	// Enabled determines if query anonymization is active
	Enabled bool `mapstructure:"enabled"`

	// AttributesToAnonymize lists the attribute names containing query text to anonymize
	AttributesToAnonymize []string `mapstructure:"attributes_to_anonymize"`

	// GenerateFingerprint creates a normalized query fingerprint for pattern detection
	GenerateFingerprint bool `mapstructure:"generate_fingerprint"`

	// FingerprintAttribute specifies where to store the query fingerprint
	FingerprintAttribute string `mapstructure:"fingerprint_attribute"`
}

// Validate checks the processor configuration
func (cfg *Config) Validate() error {
	if cfg.TimeoutMS <= 0 {
		return fmt.Errorf("timeout_ms must be positive, got %d", cfg.TimeoutMS)
	}

	if cfg.TimeoutMS > 10000 {
		return fmt.Errorf("timeout_ms must be <= 10000ms for safety, got %d", cfg.TimeoutMS)
	}

	if cfg.ErrorMode != "ignore" && cfg.ErrorMode != "propagate" {
		return fmt.Errorf("error_mode must be 'ignore' or 'propagate', got %s", cfg.ErrorMode)
	}

	if cfg.HashConfig.Algorithm != "" {
		// Only SHA-256 is supported for security reasons
		if cfg.HashConfig.Algorithm != "sha256" {
			return fmt.Errorf("unsupported hash algorithm: %s (only sha256 is supported for security)", cfg.HashConfig.Algorithm)
		}
	}

	// Safety validation
	if cfg.UnsafePlanCollection && !cfg.SafeMode {
		return fmt.Errorf("unsafe_plan_collection is enabled but safe_mode is false - this is dangerous for production databases")
	}

	// Force safe mode in production
	cfg.SafeMode = true
	cfg.UnsafePlanCollection = false

	return nil
}

// CreateDefaultConfig creates a default configuration
func createDefaultConfig() component.Config {
	return &Config{
		TimeoutMS: 100,
		ErrorMode: "ignore",
		PostgreSQLRules: PostgreSQLExtractionRules{
			DetectionJSONPath: "0.Plan",
			Extractions: map[string]string{
				"db.query.plan.cost":           "0.Plan.Total Cost",
				"db.query.plan.rows":           "0.Plan.Plan Rows",
				"db.query.plan.width":          "0.Plan.Plan Width",
				"db.query.plan.operation":      "0.Plan.Node Type",
				"db.query.plan.startup_cost":   "0.Plan.Startup Cost",
				"db.query.plan.actual_rows":    "0.Plan.Actual Rows",
				"db.query.plan.actual_loops":   "0.Plan.Actual Loops",
				"db.query.plan.shared_hit":     "0.Plan.Shared Hit Blocks",
				"db.query.plan.shared_read":    "0.Plan.Shared Read Blocks",
				"db.query.plan.temp_read":      "0.Plan.Temp Read Blocks",
				"db.query.plan.temp_written":   "0.Plan.Temp Written Blocks",
			},
			DerivedAttributes: map[string]string{
				"db.query.plan.has_seq_scan":     "has_substr_in_plan(plan_json, 'Seq Scan')",
				"db.query.plan.has_nested_loop":  "has_substr_in_plan(plan_json, 'Nested Loop')",
				"db.query.plan.has_hash_join":    "has_substr_in_plan(plan_json, 'Hash Join')",
				"db.query.plan.has_sort":         "has_substr_in_plan(plan_json, 'Sort')",
				"db.query.plan.depth":            "json_depth(plan_json)",
				"db.query.plan.node_count":       "json_node_count(plan_json)",
				"db.query.plan.efficiency":       "calculate_efficiency(cost, rows)",
			},
		},
		MySQLRules: MySQLExtractionRules{
			DetectionJSONPath: "system",
			Extractions: map[string]string{
				"db.query.plan.avg_rows":      "avg_rows",
				"db.query.digest":             "digest",
				"db.query.plan.execution_count": "execution_count",
			},
		},
		HashConfig: HashGenerationConfig{
			Include: []string{
				"query_text",
				"db.query.plan.operation",
				"db.query.plan.cost",
				"database_name",
			},
			Output:    "db.query.plan.hash",
			Algorithm: "sha256",
		},
		EnableDebugLogging: false,
		UnsafePlanCollection: false,
		SafeMode: true,
		QueryAnonymization: QueryAnonymizationConfig{
			Enabled:               true,
			AttributesToAnonymize: []string{"query_text", "db.statement", "db.query"},
			GenerateFingerprint:   true,
			FingerprintAttribute:  "db.query.fingerprint",
		},
		QueryLens: QueryLensConfig{
			Enabled:              false, // Disabled by default, enable when pg_querylens is available
			PlanHistoryHours:     24,
			RegressionThreshold:  1.5,
			RegressionDetection: RegressionDetectionConfig{
				Enabled:      true,
				TimeIncrease: 1.5,
				IOIncrease:   2.0,
				CostIncrease: 2.0,
			},
			AlertOnRegression: false,
		},
	}
}

// GetTimeout returns the configured timeout as a duration
func (cfg *Config) GetTimeout() time.Duration {
	return time.Duration(cfg.TimeoutMS) * time.Millisecond
}