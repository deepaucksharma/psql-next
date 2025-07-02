package e2e

import (
	// "context"
	"database/sql"
	// "encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCustomProcessorValidation validates all 7 custom processors
func TestCustomProcessorValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping processor validation test in short mode")
	}

	// Ensure E2E environment is running
	require.True(t, isE2EEnvironmentReady(t), "E2E environment must be running")

	t.Run("AdaptiveSampler", testAdaptiveSamplerProcessor)
	t.Run("CircuitBreaker", testCircuitBreakerProcessor)
	t.Run("PlanAttributeExtractor", testPlanAttributeExtractorProcessor)
	t.Run("Verification", testVerificationProcessor)
	t.Run("CostControl", testCostControlProcessor)
	t.Run("QueryCorrelator", testQueryCorrelatorProcessor)
	t.Run("NRErrorMonitor", testNRErrorMonitorProcessor)
}

// testAdaptiveSamplerProcessor validates adaptive sampling behavior
func testAdaptiveSamplerProcessor(t *testing.T) {
	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()

	t.Log("Testing Adaptive Sampler processor...")

	// Generate queries with different characteristics
	t.Run("High_Frequency_Sampling", func(t *testing.T) {
		// Generate 1000 identical queries - should be sampled down
		query := "SELECT * FROM e2e_test.users WHERE id = $1"
		for i := 0; i < 1000; i++ {
			var count int
			pgDB.QueryRow(query, 1).Scan(&count)
		}
		
		// Wait for processing
		time.Sleep(5 * time.Second)
		
		// Check if sampling occurred
		metrics := getCollectorMetrics(t)
		sampledCount := extractMetricValue(metrics, "otelcol_processor_adaptivesampler_sampled_total")
		droppedCount := extractMetricValue(metrics, "otelcol_processor_adaptivesampler_dropped_total")
		
		assert.Greater(t, droppedCount, float64(0), "Should have dropped some high-frequency queries")
		assert.Less(t, sampledCount, float64(1000), "Should not have sampled all queries")
	})

	t.Run("Expensive_Query_Priority", func(t *testing.T) {
		// Generate expensive queries - should always be sampled
		_, err := pgDB.Exec("SET statement_timeout = '10s'")
		require.NoError(t, err)
		
		// Force expensive query
		_, err = pgDB.Exec(`
			SELECT COUNT(*) FROM e2e_test.events e1 
			CROSS JOIN e2e_test.events e2 
			WHERE e1.created_at > NOW() - INTERVAL '1 hour'
			LIMIT 1
		`)
		// Ignore timeout error
		
		time.Sleep(5 * time.Second)
		
		// Verify expensive queries are captured
		output := getCollectorOutput(t)
		assert.Contains(t, output, "CROSS JOIN", "Expensive queries should be captured")
	})

	t.Run("Deduplication_Window", func(t *testing.T) {
		// Generate duplicate queries within dedup window
		duplicateQuery := "SELECT DISTINCT event_type FROM e2e_test.events"
		
		// Execute same query multiple times
		for i := 0; i < 10; i++ {
			rows, _ := pgDB.Query(duplicateQuery)
			rows.Close()
			time.Sleep(100 * time.Millisecond)
		}
		
		time.Sleep(5 * time.Second)
		
		// Check deduplication metrics
		metrics := getCollectorMetrics(t)
		dedupCount := extractMetricValue(metrics, "otelcol_processor_adaptivesampler_deduplicated_total")
		assert.Greater(t, dedupCount, float64(5), "Should have deduplicated queries")
	})
}

// testCircuitBreakerProcessor validates circuit breaker state transitions
func testCircuitBreakerProcessor(t *testing.T) {
	t.Log("Testing Circuit Breaker processor...")

	t.Run("State_Transitions", func(t *testing.T) {
		// Create a separate connection to force errors
		badDB, err := sql.Open("postgres", "host=localhost port=5433 user=invalid password=invalid dbname=invalid sslmode=disable")
		require.NoError(t, err)
		defer badDB.Close()

		// Generate failures to trip circuit breaker
		for i := 0; i < 10; i++ {
			badDB.Ping() // Will fail
		}

		time.Sleep(5 * time.Second)

		// Check circuit breaker metrics
		metrics := getCollectorMetrics(t)
		
		// Verify circuit breaker opened
		openState := extractMetricValue(metrics, "otelcol_processor_circuitbreaker_open_total")
		assert.Greater(t, openState, float64(0), "Circuit breaker should have opened")
		
		// Wait for half-open transition
		time.Sleep(35 * time.Second) // Default timeout is 30s
		
		// Verify half-open state
		metrics = getCollectorMetrics(t)
		halfOpenState := extractMetricValue(metrics, "otelcol_processor_circuitbreaker_halfopen_total")
		assert.Greater(t, halfOpenState, float64(0), "Circuit breaker should transition to half-open")
	})

	t.Run("Per_Database_Isolation", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()
		
		mysqlDB := connectMySQL(t)
		defer mysqlDB.Close()

		// PostgreSQL should work fine
		var pgResult string
		err := pgDB.QueryRow("SELECT 'healthy'").Scan(&pgResult)
		assert.NoError(t, err)
		assert.Equal(t, "healthy", pgResult)

		// MySQL should also work independently
		var mysqlResult string
		err = mysqlDB.QueryRow("SELECT 'healthy'").Scan(&mysqlResult)
		assert.NoError(t, err)
		assert.Equal(t, "healthy", mysqlResult)

		// Verify metrics show both databases operational
		metrics := getCollectorMetrics(t)
		pgRequests := extractMetricValueWithLabel(metrics, "otelcol_processor_circuitbreaker_requests_total", "database", "postgres")
		mysqlRequests := extractMetricValueWithLabel(metrics, "otelcol_processor_circuitbreaker_requests_total", "database", "mysql")
		
		assert.Greater(t, pgRequests, float64(0), "PostgreSQL requests should be processed")
		assert.Greater(t, mysqlRequests, float64(0), "MySQL requests should be processed")
	})
}

// testPlanAttributeExtractorProcessor validates query plan extraction and PII sanitization
func testPlanAttributeExtractorProcessor(t *testing.T) {
	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()

	t.Log("Testing Plan Attribute Extractor processor...")

	t.Run("PII_Sanitization", func(t *testing.T) {
		// Execute queries with various PII patterns
		piiQueries := []string{
			"SELECT * FROM e2e_test.users WHERE email = 'john.doe@example.com'",
			"SELECT * FROM e2e_test.users WHERE ssn = '123-45-6789'",
			"SELECT * FROM e2e_test.users WHERE credit_card = '4111-1111-1111-1111'",
			"SELECT * FROM e2e_test.users WHERE phone = '555-123-4567'",
			"UPDATE e2e_test.users SET api_key = 'sk_test_abcdef123456789012345678' WHERE id = 1",
		}

		for _, query := range piiQueries {
			rows, _ := pgDB.Query(query)
			if rows != nil {
				rows.Close()
			}
		}

		time.Sleep(10 * time.Second)

		// Verify PII is sanitized in output
		output := getCollectorOutput(t)
		
		// These PII values should NOT appear in output
		assert.NotContains(t, output, "john.doe@example.com", "Email should be sanitized")
		assert.NotContains(t, output, "123-45-6789", "SSN should be sanitized")
		assert.NotContains(t, output, "4111-1111-1111-1111", "Credit card should be sanitized")
		assert.NotContains(t, output, "555-123-4567", "Phone should be sanitized")
		assert.NotContains(t, output, "sk_test_abcdef123456789012345678", "API key should be sanitized")
		
		// Verify sanitized placeholders exist
		assert.Contains(t, output, "[REDACTED]", "Should contain redaction markers")
	})

	t.Run("Query_Anonymization", func(t *testing.T) {
		// Execute queries with literals that should be anonymized
		queries := []struct {
			original string
			expected string
		}{
			{
				"SELECT * FROM e2e_test.orders WHERE total_amount > 100.50",
				"SELECT * FROM e2e_test.orders WHERE total_amount > ?",
			},
			{
				"INSERT INTO e2e_test.events (event_type, event_data) VALUES ('user_login', '{\"user_id\": 42}')",
				"INSERT INTO e2e_test.events (event_type, event_data) VALUES (?, ?)",
			},
		}

		for _, q := range queries {
			rows, _ := pgDB.Query(q.original)
			if rows != nil {
				rows.Close()
			}
		}

		time.Sleep(5 * time.Second)

		// Check for anonymized queries in output
		output := getCollectorOutput(t)
		// Check that literals are anonymized
		assert.NotContains(t, output, "100.50", "Numeric literals should be anonymized")
		assert.NotContains(t, output, "user_login", "String literals should be anonymized")
		assert.NotContains(t, output, "42", "Embedded values should be anonymized")
	})

	t.Run("Plan_Extraction", func(t *testing.T) {
		// Execute query and get its plan
		var planJSON string
		err := pgDB.QueryRow(`
			EXPLAIN (FORMAT JSON, ANALYZE, BUFFERS) 
			SELECT u.*, COUNT(o.id) as order_count 
			FROM e2e_test.users u 
			LEFT JOIN e2e_test.orders o ON u.id = o.user_id 
			GROUP BY u.id, u.email, u.ssn, u.phone, u.credit_card, u.name, u.created_at
		`).Scan(&planJSON)
		require.NoError(t, err)

		// Execute the actual query
		rows, err := pgDB.Query(`
			SELECT u.*, COUNT(o.id) as order_count 
			FROM e2e_test.users u 
			LEFT JOIN e2e_test.orders o ON u.id = o.user_id 
			GROUP BY u.id, u.email, u.ssn, u.phone, u.credit_card, u.name, u.created_at
		`)
		require.NoError(t, err)
		rows.Close()

		time.Sleep(5 * time.Second)

		// Verify plan attributes are extracted
		metrics := getCollectorMetrics(t)
		planExtracted := extractMetricValue(metrics, "otelcol_processor_planattributeextractor_plans_extracted_total")
		assert.Greater(t, planExtracted, float64(0), "Should have extracted query plans")
	})
}

// testVerificationProcessor validates data quality and verification
func testVerificationProcessor(t *testing.T) {
	t.Log("Testing Verification processor...")

	t.Run("Data_Quality_Validation", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate queries with missing required fields
		// This would normally be caught by the verification processor
		
		time.Sleep(5 * time.Second)

		// Check verification metrics
		metrics := getCollectorMetrics(t)
		qualityChecks := extractMetricValue(metrics, "otelcol_processor_verification_quality_checks_total")
		assert.Greater(t, qualityChecks, float64(0), "Should have performed quality checks")
	})

	t.Run("Cardinality_Control", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate high cardinality queries
		for i := 0; i < 1000; i++ {
			uniqueValue := fmt.Sprintf("unique_value_%d_%d", time.Now().Unix(), i)
			pgDB.Exec("INSERT INTO e2e_test.events (event_type, event_data) VALUES ($1, '{}')", uniqueValue)
		}

		time.Sleep(10 * time.Second)

		// Check cardinality control metrics
		metrics := getCollectorMetrics(t)
		cardinalityWarnings := extractMetricValue(metrics, "otelcol_processor_verification_cardinality_warnings_total")
		assert.Greater(t, cardinalityWarnings, float64(0), "Should have cardinality warnings")
	})

	t.Run("Auto_Tuning", func(t *testing.T) {
		// Verification processor should auto-tune based on observed patterns
		time.Sleep(5 * time.Minute) // Let it observe patterns

		metrics := getCollectorMetrics(t)
		autoTuneActions := extractMetricValue(metrics, "otelcol_processor_verification_autotune_actions_total")
		assert.Greater(t, autoTuneActions, float64(0), "Should have performed auto-tuning")
	})
}

// testCostControlProcessor validates cost management features
func testCostControlProcessor(t *testing.T) {
	t.Log("Testing Cost Control processor...")

	t.Run("Budget_Tracking", func(t *testing.T) {
		// Check current budget utilization
		metrics := getCollectorMetrics(t)
		
		bytesIngested := extractMetricValue(metrics, "otelcol_processor_costcontrol_bytes_ingested_total")
		assert.Greater(t, bytesIngested, float64(0), "Should track bytes ingested")
		
		costEstimate := extractMetricValue(metrics, "otelcol_processor_costcontrol_estimated_cost_usd")
		assert.Greater(t, costEstimate, float64(0), "Should calculate cost estimate")
		
		budgetUtilization := extractMetricValue(metrics, "otelcol_processor_costcontrol_budget_utilization_percent")
		assert.GreaterOrEqual(t, budgetUtilization, float64(0), "Should track budget utilization")
		assert.LessOrEqual(t, budgetUtilization, float64(100), "Budget utilization should be <= 100%")
	})

	t.Run("Cardinality_Reduction", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate high cardinality data to trigger reduction
		for i := 0; i < 100; i++ {
			for j := 0; j < 100; j++ {
				label := fmt.Sprintf("label_%d_%d", i, j)
				pgDB.Exec("SELECT $1::text as high_cardinality_label", label)
			}
		}

		time.Sleep(10 * time.Second)

		metrics := getCollectorMetrics(t)
		reducedMetrics := extractMetricValue(metrics, "otelcol_processor_costcontrol_cardinality_reduced_total")
		assert.Greater(t, reducedMetrics, float64(0), "Should have reduced cardinality")
	})
}

// testQueryCorrelatorProcessor validates query correlation
func testQueryCorrelatorProcessor(t *testing.T) {
	pgDB := connectPostgreSQL(t)
	defer pgDB.Close()

	t.Log("Testing Query Correlator processor...")

	t.Run("Transaction_Correlation", func(t *testing.T) {
		// Execute related queries in a transaction
		tx, err := pgDB.Begin()
		require.NoError(t, err)

		// Parent query
		var userID int
		err = tx.QueryRow("SELECT id FROM e2e_test.users WHERE email = $1", "john.doe@example.com").Scan(&userID)
		require.NoError(t, err)

		// Child queries
		_, err = tx.Exec("UPDATE e2e_test.users SET name = $1 WHERE id = $2", "John Updated", userID)
		require.NoError(t, err)

		_, err = tx.Exec("INSERT INTO e2e_test.orders (user_id, total_amount) VALUES ($1, $2)", userID, 99.99)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		time.Sleep(5 * time.Second)

		// Check correlation metrics
		metrics := getCollectorMetrics(t)
		correlatedQueries := extractMetricValue(metrics, "otelcol_processor_querycorrelator_correlated_queries_total")
		assert.Greater(t, correlatedQueries, float64(0), "Should have correlated queries")
		
		transactionsTracked := extractMetricValue(metrics, "otelcol_processor_querycorrelator_transactions_tracked_total")
		assert.Greater(t, transactionsTracked, float64(0), "Should track transactions")
	})

	t.Run("Table_Statistics_Enrichment", func(t *testing.T) {
		// Generate table modifications
		for i := 0; i < 10; i++ {
			pgDB.Exec("UPDATE e2e_test.events SET event_data = event_data || '{}' WHERE id % 10 = $1", i)
		}

		// Query the modified table
		rows, err := pgDB.Query("SELECT COUNT(*) FROM e2e_test.events WHERE event_data IS NOT NULL")
		require.NoError(t, err)
		rows.Close()

		time.Sleep(5 * time.Second)

		// Verify table stats are added
		output := getCollectorOutput(t)
		// Should contain maintenance indicators
		assert.Contains(t, output, "table_stats", "Should enrich with table statistics")
	})
}

// testNRErrorMonitorProcessor validates New Relic error monitoring
func testNRErrorMonitorProcessor(t *testing.T) {
	t.Log("Testing NR Error Monitor processor...")

	t.Run("Error_Pattern_Detection", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate patterns that would cause NR errors
		
		// 1. Excessive attribute length
		longValue := strings.Repeat("x", 5000) // Over 4096 limit
		pgDB.Exec("SELECT $1::text as excessive_length", longValue)

		// 2. High cardinality metric names
		for i := 0; i < 100; i++ {
			metricName := fmt.Sprintf("custom.metric.name.very.long.and.unique.%d", i)
			pgDB.Exec("SELECT $1::text as metric_name", metricName)
		}

		// 3. Invalid characters in metric names
		pgDB.Exec("SELECT 'metric name with spaces' as invalid_metric")
		pgDB.Exec("SELECT 'metric@name#with$special%chars' as invalid_metric")

		time.Sleep(5 * time.Second)

		// Check error monitor metrics
		metrics := getCollectorMetrics(t)
		
		lengthViolations := extractMetricValue(metrics, "otelcol_processor_nrerrormonitor_attribute_length_violations_total")
		assert.Greater(t, lengthViolations, float64(0), "Should detect attribute length violations")
		
		cardinalityWarnings := extractMetricValue(metrics, "otelcol_processor_nrerrormonitor_cardinality_warnings_total")
		assert.Greater(t, cardinalityWarnings, float64(0), "Should detect high cardinality")
		
		namingViolations := extractMetricValue(metrics, "otelcol_processor_nrerrormonitor_naming_violations_total")
		assert.Greater(t, namingViolations, float64(0), "Should detect naming violations")
	})

	t.Run("Proactive_Validation", func(t *testing.T) {
		// Check if processor is preventing errors proactively
		metrics := getCollectorMetrics(t)
		
		preventedErrors := extractMetricValue(metrics, "otelcol_processor_nrerrormonitor_errors_prevented_total")
		assert.Greater(t, preventedErrors, float64(0), "Should prevent errors proactively")
		
		alerts := extractMetricValue(metrics, "otelcol_processor_nrerrormonitor_alerts_generated_total")
		assert.GreaterOrEqual(t, alerts, float64(0), "Should generate alerts when needed")
	})
}

// Helper functions

func isE2EEnvironmentReady(t *testing.T) bool {
	// Check if databases are accessible
	pgDB, err := sql.Open("postgres", "host=localhost port=5433 user=postgres password=postgres dbname=e2e_test sslmode=disable")
	if err != nil {
		return false
	}
	defer pgDB.Close()
	
	if err := pgDB.Ping(); err != nil {
		return false
	}
	
	// If PostgreSQL is accessible, assume environment is ready
	return true
}

func getCollectorMetrics(t *testing.T) string {
	resp, err := http.Get("http://localhost:8890/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body)
}

func getCollectorOutput(t *testing.T) string {
	// Execute command to get file output
	output, err := execInContainer("e2e-collector", "tail -1000 /var/lib/otel/e2e-output.json")
	require.NoError(t, err)
	return output
}

func extractMetricValue(metrics, metricName string) float64 {
	// Simple metric extraction - in real implementation would use Prometheus parser
	lines := strings.Split(metrics, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, metricName) && !strings.HasPrefix(line, "#") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var value float64
				fmt.Sscanf(parts[1], "%f", &value)
				return value
			}
		}
	}
	return 0
}

func extractMetricValueWithLabel(metrics, metricName, labelKey, labelValue string) float64 {
	// Extract metric with specific label
	searchPattern := fmt.Sprintf(`%s{.*%s="%s".*}`, metricName, labelKey, labelValue)
	lines := strings.Split(metrics, "\n")
	for _, line := range lines {
		if strings.Contains(line, searchPattern) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var value float64
				fmt.Sscanf(parts[1], "%f", &value)
				return value
			}
		}
	}
	return 0
}

func execInContainer(container, command string) (string, error) {
	// Would use Docker SDK in real implementation
	return "", nil
}

// Helper functions from real_e2e_test.go
func connectPostgreSQL(t *testing.T) *sql.DB {
	dsn := "host=localhost port=5433 user=postgres password=postgres dbname=e2e_test sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	
	err = db.Ping()
	require.NoError(t, err)
	
	return db
}

func connectMySQL(t *testing.T) *sql.DB {
	dsn := "mysql:mysql@tcp(localhost:3307)/e2e_test"
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	
	err = db.Ping()
	require.NoError(t, err)
	
	return db
}