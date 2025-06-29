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

	// Algorithm specifies the hash algorithm (sha256, sha1, md5)
	Algorithm string `mapstructure:"algorithm"`
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
		validAlgorithms := map[string]bool{
			"sha256": true,
			"sha1":   true,
			"md5":    true,
		}
		if !validAlgorithms[cfg.HashConfig.Algorithm] {
			return fmt.Errorf("invalid hash algorithm: %s", cfg.HashConfig.Algorithm)
		}
	}

	return nil
}

// CreateDefaultConfig creates a default configuration
func createDefaultConfig() component.Config {
	return &Config{
		TimeoutMS: 100,
		ErrorMode: "ignore",
		PostgreSQLRules: PostgreSQLExtractionRules{
			DetectionJSONPath: "$[0].Plan",
			Extractions: map[string]string{
				"db.query.plan.cost":           "$[0].Plan['Total Cost']",
				"db.query.plan.rows":           "$[0].Plan['Plan Rows']",
				"db.query.plan.width":          "$[0].Plan['Plan Width']",
				"db.query.plan.operation":      "$[0].Plan['Node Type']",
				"db.query.plan.startup_cost":   "$[0].Plan['Startup Cost']",
				"db.query.plan.actual_rows":    "$[0].Plan['Actual Rows']",
				"db.query.plan.actual_loops":   "$[0].Plan['Actual Loops']",
				"db.query.plan.shared_hit":     "$[0].Plan['Shared Hit Blocks']",
				"db.query.plan.shared_read":    "$[0].Plan['Shared Read Blocks']",
				"db.query.plan.temp_read":      "$[0].Plan['Temp Read Blocks']",
				"db.query.plan.temp_written":   "$[0].Plan['Temp Written Blocks']",
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
			DetectionJSONPath: "$.system",
			Extractions: map[string]string{
				"db.query.plan.avg_rows":      "$.avg_rows",
				"db.query.digest":             "$.digest",
				"db.query.plan.execution_count": "$.execution_count",
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
	}
}

// GetTimeout returns the configured timeout as a duration
func (cfg *Config) GetTimeout() time.Duration {
	return time.Duration(cfg.TimeoutMS) * time.Millisecond
}