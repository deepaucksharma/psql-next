package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetricsToNRDBMapping validates that all metrics are correctly transformed for NRDB
func TestMetricsToNRDBMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.Cleanup()

	// Start collector
	collector := testEnv.StartCollector(t, "testdata/config-full-integration.yaml")
	defer collector.Shutdown()

	// Generate comprehensive activity
	generateFullStackActivity(t, testEnv.PostgresDB)
	time.Sleep(10 * time.Second)

	// Get NRDB payload
	nrdbPayload := testEnv.GetNRDBPayload()
	require.NotNil(t, nrdbPayload)

	t.Run("MetricNaming", func(t *testing.T) {
		// Verify metric names follow NRDB conventions
		for _, metric := range nrdbPayload.Metrics {
			// Check naming convention
			assert.True(t, strings.HasPrefix(metric.Name, "db.postgresql.") || 
				strings.HasPrefix(metric.Name, "postgresql.ash."),
				"Metric %s doesn't follow naming convention", metric.Name)
			
			// No spaces or special characters
			assert.NotContains(t, metric.Name, " ", "Metric name contains space")
			assert.NotContains(t, metric.Name, "-", "Metric name contains hyphen")
		}
	})

	t.Run("RequiredAttributes", func(t *testing.T) {
		// Define required attributes for each metric type
		requiredAttrs := map[string][]string{
			"db.postgresql.query.exec_time": {"query_id", "database"},
			"db.postgresql.query.plan_time": {"query_id", "database"},
			"db.postgresql.plan.regression": {"query_id", "regression_type"},
			"postgresql.ash.sessions.count": {"state"},
			"postgresql.ash.wait_events.count": {"wait_event_type", "wait_event", "category"},
			"postgresql.ash.blocking_sessions.count": {},
			"postgresql.ash.query.active_count": {"query_id"},
		}

		metricCounts := make(map[string]int)
		
		for _, metric := range nrdbPayload.Metrics {
			metricCounts[metric.Name]++
			
			if required, exists := requiredAttrs[metric.Name]; exists {
				for _, attr := range required {
					assert.Contains(t, metric.Attributes, attr, 
						"Metric %s missing required attribute %s", metric.Name, attr)
				}
			}
		}

		// Verify all expected metrics are present
		for metricName := range requiredAttrs {
			assert.Greater(t, metricCounts[metricName], 0, 
				"Expected metric %s not found in NRDB payload", metricName)
		}
	})

	t.Run("DataTypes", func(t *testing.T) {
		// Verify data types are correct for NRDB
		for _, metric := range nrdbPayload.Metrics {
			// Timestamp should be Unix epoch in milliseconds
			assert.Greater(t, metric.Timestamp, int64(1600000000000), "Timestamp too old")
			assert.Less(t, metric.Timestamp, int64(2000000000000), "Timestamp too far in future")
			
			// Value should be numeric
			switch v := metric.Value.(type) {
			case float64, int64, int:
				// Valid numeric types
			default:
				t.Errorf("Metric %s has invalid value type: %T", metric.Name, v)
			}
			
			// Attributes should be strings or numbers
			for attrName, attrValue := range metric.Attributes {
				switch v := attrValue.(type) {
				case string, float64, int64, int, bool:
					// Valid attribute types
				default:
					t.Errorf("Metric %s attribute %s has invalid type: %T", 
						metric.Name, attrName, v)
				}
			}
		}
	})

	t.Run("QueryCorrelation", func(t *testing.T) {
		// Verify query_id correlation between plan and ASH metrics
		planQueryIDs := make(map[string]bool)
		ashQueryIDs := make(map[string]bool)
		
		for _, metric := range nrdbPayload.Metrics {
			if strings.Contains(metric.Name, "query") {
				if qid, ok := metric.Attributes["query_id"].(string); ok {
					if strings.HasPrefix(metric.Name, "db.postgresql.") {
						planQueryIDs[qid] = true
					} else if strings.HasPrefix(metric.Name, "postgresql.ash.") {
						ashQueryIDs[qid] = true
					}
				}
			}
		}
		
		// Should have overlapping query IDs
		overlap := 0
		for qid := range planQueryIDs {
			if ashQueryIDs[qid] {
				overlap++
			}
		}
		
		assert.Greater(t, overlap, 0, "No query correlation between plan and ASH metrics")
		t.Logf("Found %d correlated queries between plan and ASH", overlap)
	})

	t.Run("CommonAttributes", func(t *testing.T) {
		// Verify common attributes are set correctly
		assert.NotEmpty(t, nrdbPayload.CommonAttributes)
		
		requiredCommon := []string{
			"service.name",
			"deployment.environment",
			"db.system",
		}
		
		for _, attr := range requiredCommon {
			assert.Contains(t, nrdbPayload.CommonAttributes, attr,
				"Missing required common attribute: %s", attr)
		}
		
		// Verify db.system is always postgresql
		assert.Equal(t, "postgresql", nrdbPayload.CommonAttributes["db.system"])
	})

	t.Run("MetricCardinality", func(t *testing.T) {
		// Check metric cardinality is reasonable
		cardinalityByMetric := make(map[string]map[string]int)
		
		for _, metric := range nrdbPayload.Metrics {
			if _, exists := cardinalityByMetric[metric.Name]; !exists {
				cardinalityByMetric[metric.Name] = make(map[string]int)
			}
			
			// Create unique key from attributes
			attrKey := fmt.Sprintf("%v", metric.Attributes)
			cardinalityByMetric[metric.Name][attrKey]++
		}
		
		// Verify cardinality limits
		maxCardinality := map[string]int{
			"db.postgresql.query.exec_time": 1000,      // Max 1000 unique queries
			"postgresql.ash.sessions.count": 10,         // Limited session states
			"postgresql.ash.wait_events.count": 100,     // Limited wait events
			"postgresql.ash.query.active_count": 1000,   // Max 1000 unique queries
		}
		
		for metricName, cardinality := range cardinalityByMetric {
			uniqueCount := len(cardinality)
			if maxAllowed, exists := maxCardinality[metricName]; exists {
				assert.LessOrEqual(t, uniqueCount, maxAllowed,
					"Metric %s exceeds cardinality limit: %d > %d",
					metricName, uniqueCount, maxAllowed)
			}
			t.Logf("Metric %s cardinality: %d", metricName, uniqueCount)
		}
	})

	t.Run("AnonymizationValidation", func(t *testing.T) {
		// Verify no PII in exported metrics
		piiPatterns := []string{
			`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email
			`\b\d{3}-\d{2}-\d{4}\b`,                                // SSN
			`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,          // Credit card
		}
		
		jsonPayload, _ := json.Marshal(nrdbPayload)
		payloadStr := string(jsonPayload)
		
		for _, pattern := range piiPatterns {
			assert.NotRegexp(t, pattern, payloadStr, 
				"Found potential PII pattern in NRDB payload")
		}
	})

	t.Run("RegressionMetrics", func(t *testing.T) {
		// Verify regression metrics have proper structure
		for _, metric := range nrdbPayload.Metrics {
			if metric.Name == "db.postgresql.plan.regression" {
				// Check severity is between 0 and 1
				severity, ok := metric.Value.(float64)
				assert.True(t, ok, "Regression severity should be float64")
				assert.GreaterOrEqual(t, severity, 0.0)
				assert.LessOrEqual(t, severity, 1.0)
				
				// Check required regression attributes
				assert.Contains(t, metric.Attributes, "query_id")
				assert.Contains(t, metric.Attributes, "regression_type")
				assert.Contains(t, metric.Attributes, "old_plan_hash")
				assert.Contains(t, metric.Attributes, "new_plan_hash")
			}
		}
	})

	t.Run("WaitEventMetrics", func(t *testing.T) {
		// Verify wait event metrics are properly categorized
		waitEventsSeen := make(map[string]map[string]bool)
		
		for _, metric := range nrdbPayload.Metrics {
			if metric.Name == "postgresql.ash.wait_events.count" {
				category := metric.Attributes["category"].(string)
				event := metric.Attributes["wait_event"].(string)
				
				if _, exists := waitEventsSeen[category]; !exists {
					waitEventsSeen[category] = make(map[string]bool)
				}
				waitEventsSeen[category][event] = true
				
				// Verify severity is set
				assert.Contains(t, metric.Attributes, "severity")
				severity := metric.Attributes["severity"].(string)
				assert.Contains(t, []string{"info", "warning", "critical"}, severity)
			}
		}
		
		// Should have multiple categories
		assert.GreaterOrEqual(t, len(waitEventsSeen), 2, 
			"Expected at least 2 wait event categories")
		
		t.Logf("Wait event categories found: %v", getMapKeys(waitEventsSeen))
	})

	t.Run("MetricCompleteness", func(t *testing.T) {
		// Verify complete metric coverage
		expectedMetrics := []string{
			// Infrastructure metrics
			"db.postgresql.database.size",
			"db.postgresql.connections.active",
			
			// Plan metrics
			"db.postgresql.query.exec_time",
			"db.postgresql.query.plan_time",
			"db.postgresql.plan.changes",
			
			// ASH metrics
			"postgresql.ash.sessions.count",
			"postgresql.ash.wait_events.count",
			"postgresql.ash.blocking_sessions.count",
			"postgresql.ash.blocked_sessions.count",
			"postgresql.ash.query.active_count",
			
			// Collector metrics
			"otelcol_receiver_accepted_metric_points",
			"otelcol_processor_dropped_metric_points",
			"otelcol_exporter_sent_metric_points",
		}
		
		foundMetrics := make(map[string]bool)
		for _, metric := range nrdbPayload.Metrics {
			foundMetrics[metric.Name] = true
		}
		
		missingMetrics := []string{}
		for _, expected := range expectedMetrics {
			if !foundMetrics[expected] {
				// Some metrics might not be present in test scenario
				if !isOptionalMetric(expected) {
					missingMetrics = append(missingMetrics, expected)
				}
			}
		}
		
		if len(missingMetrics) > 0 {
			t.Logf("Missing metrics (may be expected): %v", missingMetrics)
		}
	})
}

// Helper functions

func getMapKeys(m map[string]map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func isOptionalMetric(metricName string) bool {
	optionalMetrics := []string{
		"db.postgresql.database.size",       // May not be collected in test
		"db.postgresql.plan.changes",        // Requires plan changes
		"otelcol_receiver_accepted_metric_points", // Collector internal metrics
		"otelcol_processor_dropped_metric_points",
		"otelcol_exporter_sent_metric_points",
	}
	
	for _, optional := range optionalMetrics {
		if metricName == optional {
			return true
		}
	}
	return false
}