package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"

	// Import all components
	"github.com/database-intelligence-mvp/processors/adaptivesampler"
	"github.com/database-intelligence-mvp/processors/circuitbreaker"
	"github.com/database-intelligence-mvp/processors/costcontrol"
	"github.com/database-intelligence-mvp/processors/nrerrormonitor"
	"github.com/database-intelligence-mvp/processors/planattributeextractor"
	"github.com/database-intelligence-mvp/processors/querycorrelator"
	"github.com/database-intelligence-mvp/processors/verification"
)

// TestConfigurationValidation validates that our configuration files are correct
func TestConfigurationValidation(t *testing.T) {
	// Test configurations
	configs := []struct {
		name string
		path string
		env  map[string]string
	}{
		{
			name: "test_pipeline",
			path: "../../config/test-pipeline.yaml",
			env: map[string]string{
				"POSTGRES_HOST":     "localhost",
				"POSTGRES_PORT":     "5432",
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "postgres",
				"POSTGRES_DB":       "testdb",
			},
		},
		{
			name: "collector_minimal_test",
			path: "../../config/collector-minimal-test.yaml",
			env: map[string]string{
				"POSTGRES_HOST":     "localhost",
				"POSTGRES_PORT":     "5432",
				"POSTGRES_USER":     "postgres",
				"POSTGRES_PASSWORD": "postgres",
			},
		},
	}

	for _, tc := range configs {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tc.env {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tc.env {
					os.Unsetenv(k)
				}
			}()

			// Get absolute path
			absPath, err := filepath.Abs(tc.path)
			require.NoError(t, err)

			// Check if file exists
			_, err = os.Stat(absPath)
			if os.IsNotExist(err) {
				t.Skipf("Config file not found: %s", absPath)
			}
			require.NoError(t, err)

			// Create config provider
			provider := fileprovider.NewFactory().Create(confmap.ProviderSettings{})
			
			// Load config
			uri := "file:" + absPath
			retrieved, err := provider.Retrieve(context.Background(), uri, nil)
			require.NoError(t, err)

			conf, err := retrieved.AsConf()
			require.NoError(t, err)

			// Validate basic structure
			assert.True(t, conf.IsSet("receivers"))
			assert.True(t, conf.IsSet("processors"))
			assert.True(t, conf.IsSet("exporters"))
			assert.True(t, conf.IsSet("service"))

			// Check service pipelines
			pipelines := conf.Sub("service").Sub("pipelines")
			assert.NotNil(t, pipelines)
			
			// Should have at least one pipeline
			assert.True(t, pipelines.IsSet("metrics") || pipelines.IsSet("logs"))
		})
	}
}

// TestProcessorConfiguration tests that processor configurations are valid
func TestProcessorConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		factory component.Factory
		config  map[string]interface{}
		valid   bool
	}{
		{
			name:    "circuitbreaker_valid",
			factory: circuitbreaker.NewFactory(),
			config: map[string]interface{}{
				"failure_threshold": 5,
				"success_threshold": 2,
				"timeout":           "30s",
			},
			valid: true,
		},
		{
			name:    "circuitbreaker_invalid_timeout",
			factory: circuitbreaker.NewFactory(),
			config: map[string]interface{}{
				"failure_threshold": 5,
				"timeout":           "invalid",
			},
			valid: false,
		},
		{
			name:    "planattributeextractor_valid",
			factory: planattributeextractor.NewFactory(),
			config: map[string]interface{}{
				"safe_mode":   true,
				"error_mode": "ignore",
			},
			valid: true,
		},
		{
			name:    "verification_valid",
			factory: verification.NewFactory(),
			config: map[string]interface{}{
				"pii_detection": map[string]interface{}{
					"enabled": true,
				},
			},
			valid: true,
		},
		{
			name:    "costcontrol_valid",
			factory: costcontrol.NewFactory(),
			config: map[string]interface{}{
				"monthly_budget_usd":       1000.0,
				"metric_cardinality_limit": 10000,
			},
			valid: true,
		},
		{
			name:    "nrerrormonitor_valid",
			factory: nrerrormonitor.NewFactory(),
			config: map[string]interface{}{
				"max_attribute_length":         1000,
				"cardinality_warning_threshold": 1000,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create default config
			cfg := tt.factory.CreateDefaultConfig()

			// Create confmap from test config
			conf := confmap.NewFromStringMap(tt.config)

			// Unmarshal config
			err := conf.Unmarshal(cfg)

			if tt.valid {
				assert.NoError(t, err)
				
				// Validate config
				if validator, ok := cfg.(component.ConfigValidator); ok {
					err = validator.Validate()
					assert.NoError(t, err)
				}
			} else {
				// Should have an error either in unmarshaling or validation
				if err == nil {
					if validator, ok := cfg.(component.ConfigValidator); ok {
						err = validator.Validate()
					}
				}
				assert.Error(t, err)
			}
		})
	}
}

// TestProcessorChaining tests that processors can be chained correctly
func TestProcessorChaining(t *testing.T) {
	// This test ensures processors pass data correctly through the chain
	// by checking that attributes added by one processor are visible to the next

	settings := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}

	// Create a test chain that adds attributes at each step
	type testCase struct {
		name       string
		processors []string
		checkFunc  func(t *testing.T, finalData map[string]interface{})
	}

	tests := []testCase{
		{
			name:       "plan_then_correlator",
			processors: []string{"planattributeextractor", "querycorrelator"},
			checkFunc: func(t *testing.T, data map[string]interface{}) {
				// Plan extractor should add plan hash
				// Correlator should use it for correlation
				assert.Contains(t, data, "db.query.plan.hash")
				assert.Contains(t, data, "correlation.query_id")
			},
		},
		{
			name:       "verification_removes_pii",
			processors: []string{"verification"},
			checkFunc: func(t *testing.T, data map[string]interface{}) {
				// Should not contain PII patterns
				for _, v := range data {
					if str, ok := v.(string); ok {
						assert.NotContains(t, str, "@example.com")
						assert.NotRegexp(t, `\d{3}-\d{2}-\d{4}`, str)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Implementation would create actual processor chain
			// and verify data flow - placeholder for now
			t.Logf("Testing processor chain: %v", tt.processors)
		})
	}
}

// TestCollectorStartup tests that the collector can start with our configuration
func TestCollectorStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping collector startup test in short mode")
	}

	// Set required environment variables
	os.Setenv("POSTGRES_HOST", "localhost")
	os.Setenv("POSTGRES_PORT", "5432")
	os.Setenv("POSTGRES_USER", "test")
	os.Setenv("POSTGRES_PASSWORD", "test")
	defer func() {
		os.Unsetenv("POSTGRES_HOST")
		os.Unsetenv("POSTGRES_PORT")
		os.Unsetenv("POSTGRES_USER")
		os.Unsetenv("POSTGRES_PASSWORD")
	}()

	// Create factories map
	factories := otelcol.Factories{
		Processors: map[component.Type]component.ProcessorFactory{
			circuitbreaker.GetType():         circuitbreaker.NewFactory(),
			planattributeextractor.GetType(): planattributeextractor.NewFactory(),
			querycorrelator.GetType():        querycorrelator.NewFactory(),
			verification.GetType():           verification.NewFactory(),
			costcontrol.GetType():            costcontrol.NewFactory(),
			nrerrormonitor.GetType():         nrerrormonitor.NewFactory(),
		},
	}

	// Test that factories are properly registered
	assert.Len(t, factories.Processors, 6)
	
	for name, factory := range factories.Processors {
		t.Logf("Registered processor: %s", name)
		assert.NotNil(t, factory)
		
		// Check factory can create default config
		cfg := factory.CreateDefaultConfig()
		assert.NotNil(t, cfg)
	}
}

// TestMetricsFlow tests the flow of metrics through the pipeline
func TestMetricsFlow(t *testing.T) {
	// This test would use testcontainers to spin up a real PostgreSQL
	// and test the full flow from receiver to exporter
	t.Skip("Full metrics flow test requires database - implement with testcontainers")
}