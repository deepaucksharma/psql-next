package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TestPlanIntelligenceE2E tests the complete flow from PostgreSQL auto_explain to NRDB
func TestPlanIntelligenceE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup test environment
	ctx := context.Background()
	testEnv := setupTestEnvironment(t)
	defer testEnv.Cleanup()

	// Create test database and tables
	db := testEnv.PostgresDB
	setupTestSchema(t, db)

	// Start collector with plan intelligence configuration
	collector := testEnv.StartCollector(t, "testdata/config-plan-intelligence.yaml")
	defer collector.Shutdown()

	// Wait for collector to be ready
	require.Eventually(t, func() bool {
		return collector.IsHealthy()
	}, 30*time.Second, 1*time.Second, "Collector did not become healthy")

	t.Run("AutoExplainLogCollection", func(t *testing.T) {
		// Generate queries that will trigger auto_explain
		generateSlowQueries(t, db)

		// Wait for log processing
		time.Sleep(5 * time.Second)

		// Verify metrics were collected
		metrics := testEnv.GetCollectedMetrics()
		
		// Check for plan time metrics
		planTimeMetrics := findMetricsByName(metrics, "db.postgresql.query.plan_time")
		assert.NotEmpty(t, planTimeMetrics, "No plan time metrics collected")
		
		// Check for execution time metrics
		execTimeMetrics := findMetricsByName(metrics, "db.postgresql.query.exec_time")
		assert.NotEmpty(t, execTimeMetrics, "No execution time metrics collected")
		
		// Verify query attributes
		for _, metric := range execTimeMetrics {
			attrs := getMetricAttributes(metric)
			assert.Contains(t, attrs, "query_id", "Missing query_id attribute")
			assert.Contains(t, attrs, "database", "Missing database attribute")
		}
	})

	t.Run("PlanAnonymization", func(t *testing.T) {
		// Execute query with PII
		_, err := db.Exec(`
			SELECT * FROM users 
			WHERE email = 'john.doe@example.com' 
			AND ssn = '123-45-6789'
			AND credit_card = '4111111111111111'
		`)
		require.NoError(t, err)

		// Wait for processing
		time.Sleep(3 * time.Second)

		// Check logs for anonymized content
		logs := testEnv.GetCollectedLogs()
		for _, log := range logs {
			body := log.Body().AsString()
			
			// Verify PII is anonymized
			assert.NotContains(t, body, "john.doe@example.com", "Email not anonymized")
			assert.NotContains(t, body, "123-45-6789", "SSN not anonymized")
			assert.NotContains(t, body, "4111111111111111", "Credit card not anonymized")
			
			// Verify anonymization tokens are present
			if strings.Contains(body, "Filter") {
				assert.Contains(t, body, "<EMAIL_REDACTED>", "Email redaction token missing")
				assert.Contains(t, body, "<SSN_REDACTED>", "SSN redaction token missing")
				assert.Contains(t, body, "<CC_REDACTED>", "CC redaction token missing")
			}
		}
	})

	t.Run("PlanRegressionDetection", func(t *testing.T) {
		// Create index to change plan
		_, err := db.Exec("CREATE INDEX idx_users_email ON users(email)")
		require.NoError(t, err)

		// Run same query multiple times to establish baseline
		for i := 0; i < 15; i++ {
			_, err = db.Exec("SELECT * FROM users WHERE email = 'test@example.com'")
			require.NoError(t, err)
			time.Sleep(100 * time.Millisecond)
		}

		// Drop index to cause plan regression
		_, err = db.Exec("DROP INDEX idx_users_email")
		require.NoError(t, err)

		// Run query again with worse plan
		for i := 0; i < 15; i++ {
			_, err = db.Exec("SELECT * FROM users WHERE email = 'test@example.com'")
			require.NoError(t, err)
			time.Sleep(100 * time.Millisecond)
		}

		// Wait for regression detection
		time.Sleep(5 * time.Second)

		// Check for regression metrics
		metrics := testEnv.GetCollectedMetrics()
		regressionMetrics := findMetricsByName(metrics, "db.postgresql.plan.regression")
		assert.NotEmpty(t, regressionMetrics, "No regression metrics detected")

		// Verify regression attributes
		for _, metric := range regressionMetrics {
			attrs := getMetricAttributes(metric)
			assert.Contains(t, attrs, "query_id", "Missing query_id in regression")
			assert.Contains(t, attrs, "regression_type", "Missing regression_type")
			assert.Contains(t, attrs, "old_plan_hash", "Missing old_plan_hash")
			assert.Contains(t, attrs, "new_plan_hash", "Missing new_plan_hash")
			
			// Check regression severity
			value := getMetricValue(metric)
			assert.Greater(t, value, 0.0, "Regression severity should be > 0")
		}

		// Check for plan change counter
		changeMetrics := findMetricsByName(metrics, "db.postgresql.plan.changes")
		assert.NotEmpty(t, changeMetrics, "No plan change metrics detected")
	})

	t.Run("NRDBExport", func(t *testing.T) {
		// Verify metrics are exported to NRDB format
		nrdbPayload := testEnv.GetNRDBPayload()
		require.NotNil(t, nrdbPayload, "No NRDB payload generated")

		// Check for required NRDB attributes
		assert.Contains(t, nrdbPayload.CommonAttributes, "service.name")
		assert.Contains(t, nrdbPayload.CommonAttributes, "db.system")
		assert.Equal(t, "postgresql", nrdbPayload.CommonAttributes["db.system"])

		// Verify metric transformation
		planMetrics := filterNRDBMetrics(nrdbPayload.Metrics, "db.postgresql.query.plan_time")
		assert.NotEmpty(t, planMetrics, "Plan metrics not found in NRDB payload")

		for _, metric := range planMetrics {
			// Check NRDB metric structure
			assert.NotEmpty(t, metric.Name, "Metric name is empty")
			assert.NotZero(t, metric.Timestamp, "Metric timestamp is zero")
			assert.NotNil(t, metric.Value, "Metric value is nil")
			assert.NotEmpty(t, metric.Attributes["query_id"], "Query ID missing in NRDB metric")
		}
	})

	t.Run("CircuitBreakerProtection", func(t *testing.T) {
		// Simulate auto_explain not loaded error
		testEnv.SimulateAutoExplainError()

		// Generate queries
		for i := 0; i < 10; i++ {
			_, _ = db.Exec("SELECT * FROM users")
		}

		// Wait for circuit breaker to trigger
		time.Sleep(3 * time.Second)

		// Verify circuit breaker metrics
		metrics := testEnv.GetCollectedMetrics()
		cbMetrics := findMetricsByName(metrics, "otelcol_processor_circuitbreaker_triggered")
		assert.NotEmpty(t, cbMetrics, "Circuit breaker did not trigger")

		// Verify collection is disabled
		testEnv.RestoreAutoExplain()
		time.Sleep(2 * time.Second)

		// Check that collection doesn't immediately resume
		prevCount := len(testEnv.GetCollectedMetrics())
		_, _ = db.Exec("SELECT * FROM users WHERE id = 1")
		time.Sleep(1 * time.Second)
		newCount := len(testEnv.GetCollectedMetrics())
		assert.Equal(t, prevCount, newCount, "Collection resumed too quickly after circuit breaker")
	})
}

// Helper functions

func setupTestSchemaPlanIntelligence(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255),
			ssn VARCHAR(20),
			credit_card VARCHAR(20),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INT REFERENCES users(id),
			amount DECIMAL(10,2),
			status VARCHAR(50),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`INSERT INTO users (email, ssn, credit_card) 
		 SELECT 
			'user' || i || '@example.com',
			'123-45-' || lpad(i::text, 4, '0'),
			'4111111111111' || lpad(i::text, 3, '0')
		 FROM generate_series(1, 1000) i`,
		`INSERT INTO orders (user_id, amount, status)
		 SELECT 
			(random() * 999 + 1)::int,
			random() * 1000,
			CASE WHEN random() < 0.8 THEN 'completed' ELSE 'pending' END
		 FROM generate_series(1, 10000) i`,
		`ANALYZE users`,
		`ANALYZE orders`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		require.NoError(t, err, "Failed to execute setup query: %s", query)
	}
}

func generateSlowQueriesPlanIntelligence(t *testing.T, db *sql.DB) {
	queries := []string{
		// Query that will trigger auto_explain
		`SELECT u.*, COUNT(o.id) as order_count, SUM(o.amount) as total_amount
		 FROM users u
		 LEFT JOIN orders o ON u.id = o.user_id
		 WHERE u.created_at > NOW() - INTERVAL '30 days'
		 GROUP BY u.id
		 HAVING COUNT(o.id) > 5
		 ORDER BY total_amount DESC
		 LIMIT 100`,
		
		// Complex join query
		`WITH user_stats AS (
			SELECT user_id, 
				   COUNT(*) as order_count,
				   AVG(amount) as avg_amount
			FROM orders
			GROUP BY user_id
		)
		SELECT u.*, us.order_count, us.avg_amount
		FROM users u
		JOIN user_stats us ON u.id = us.user_id
		WHERE us.order_count > 10`,
		
		// Query with subquery
		`SELECT * FROM users
		 WHERE id IN (
			SELECT DISTINCT user_id 
			FROM orders 
			WHERE amount > (SELECT AVG(amount) * 2 FROM orders)
		 )`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		assert.NoError(t, err, "Failed to execute slow query")
		time.Sleep(500 * time.Millisecond) // Ensure queries are logged
	}
}

// Test configuration
const planIntelligenceTestConfigData = `
receivers:
  autoexplain:
    log_path: /tmp/test-postgresql.log
    log_format: json
    
    database:
      endpoint: localhost:5432
      username: test_user
      password: test_password
      database: test_db
    
    plan_collection:
      enabled: true
      min_duration: 10ms  # Low threshold for testing
      max_plans_per_query: 10
      retention_duration: 1h
      
      regression_detection:
        enabled: true
        performance_degradation_threshold: 0.2
        cost_increase_threshold: 0.3
        min_executions: 10
        statistical_confidence: 0.95
    
    plan_anonymization:
      enabled: true
      anonymize_filters: true
      sensitive_patterns:
        - email
        - ssn
        - credit_card
        - phone
        - ip_address

processors:
  memory_limiter:
    limit_percentage: 80
  
  circuitbreaker:
    failure_threshold: 5
    timeout: 30s
    
    error_patterns:
      - pattern: "auto_explain.*not loaded"
        action: disable_plan_collection
        backoff: 1h

exporters:
  otlp/newrelic:
    endpoint: localhost:4317
    headers:
      api-key: test-api-key

service:
  pipelines:
    metrics:
      receivers: [autoexplain]
      processors: [memory_limiter, circuitbreaker]
      exporters: [otlp/newrelic]
`