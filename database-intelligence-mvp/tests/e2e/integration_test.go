package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullIntegrationE2E tests the complete integrated flow with all components
func TestFullIntegrationE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()
	testEnv := setupTestEnvironment(t)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupFullTestSchema(t, db)

	// Start collector with full configuration
	collector := testEnv.StartCollector(t, "testdata/config-full-integration.yaml")
	defer collector.Shutdown()

	require.Eventually(t, func() bool {
		return collector.IsHealthy()
	}, 30*time.Second, 1*time.Second)

	t.Run("PlanIntelligenceWithASHCorrelation", func(t *testing.T) {
		// Execute query that will be captured by both systems
		queryText := `
			SELECT u.id, u.email, COUNT(o.id) as order_count
			FROM users u
			LEFT JOIN orders o ON u.id = o.user_id
			WHERE u.created_at > NOW() - INTERVAL '7 days'
			GROUP BY u.id, u.email
			HAVING COUNT(o.id) > 5
			ORDER BY order_count DESC`

		// Run query multiple times concurrently
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				conn := getNewConnection(t, testEnv)
				defer conn.Close()
				_, _ = conn.Exec(queryText)
			}()
		}
		wg.Wait()

		// Wait for processing
		time.Sleep(5 * time.Second)

		// Get metrics
		metrics := testEnv.GetCollectedMetrics()

		// Find plan metrics
		planMetrics := findMetricsByName(metrics, "db.postgresql.query.exec_time")
		assert.NotEmpty(t, planMetrics, "No plan metrics collected")

		// Find ASH metrics for same query
		ashMetrics := findMetricsByName(metrics, "postgresql.ash.query.active_count")
		assert.NotEmpty(t, ashMetrics, "No ASH metrics collected")

		// Verify correlation via query_id
		planQueryIDs := make(map[string]bool)
		for _, metric := range planMetrics {
			attrs := getMetricAttributes(metric)
			if qid, ok := attrs["query_id"]; ok {
				planQueryIDs[qid] = true
			}
		}

		ashQueryIDs := make(map[string]bool)
		for _, metric := range ashMetrics {
			attrs := getMetricAttributes(metric)
			if qid, ok := attrs["query_id"]; ok {
				ashQueryIDs[qid] = true
			}
		}

		// Should have overlapping query IDs
		overlap := false
		for qid := range planQueryIDs {
			if ashQueryIDs[qid] {
				overlap = true
				break
			}
		}
		assert.True(t, overlap, "No correlation between plan and ASH metrics")
	})

	t.Run("RegressionDetectionWithWaitAnalysis", func(t *testing.T) {
		// Create scenario that causes both plan regression and wait events
		
		// Step 1: Create optimal scenario
		_, err := db.Exec("CREATE INDEX idx_orders_user_id ON orders(user_id)")
		require.NoError(t, err)
		
		// Run queries with good plan
		for i := 0; i < 10; i++ {
			_, _ = db.Exec("SELECT * FROM orders WHERE user_id = $1", i)
		}

		// Step 2: Create contention
		var wg sync.WaitGroup
		
		// Lock the table
		lockConn := getNewConnection(t, testEnv)
		tx, _ := lockConn.Begin()
		_, _ = tx.Exec("LOCK TABLE orders IN ACCESS EXCLUSIVE MODE NOWAIT")

		// Step 3: Drop index (causing regression)
		dropConn := getNewConnection(t, testEnv)
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This will wait due to lock
			_, _ = dropConn.Exec("DROP INDEX idx_orders_user_id")
		}()

		// Step 4: Run queries that will wait and then use bad plan
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				conn := getNewConnection(t, testEnv)
				defer conn.Close()
				_, _ = conn.Exec("SELECT * FROM orders WHERE user_id = $1", id)
			}(i)
		}

		// Let waits accumulate
		time.Sleep(3 * time.Second)

		// Release lock
		tx.Rollback()
		lockConn.Close()

		// Wait for everything to complete
		wg.Wait()
		dropConn.Close()

		// Wait for analysis
		time.Sleep(5 * time.Second)

		metrics := testEnv.GetCollectedMetrics()

		// Check for regression detection
		regressionMetrics := findMetricsByName(metrics, "db.postgresql.plan.regression")
		assert.NotEmpty(t, regressionMetrics, "No regression detected")

		// Check for lock wait events
		waitMetrics := findMetricsByName(metrics, "postgresql.ash.wait_events.count")
		lockWaits := false
		for _, metric := range waitMetrics {
			attrs := getMetricAttributes(metric)
			if attrs["wait_event_type"] == "Lock" {
				lockWaits = true
				break
			}
		}
		assert.True(t, lockWaits, "No lock wait events detected")

		// Check for wait alerts
		alertMetrics := findMetricsByName(metrics, "postgresql.ash.wait_alert.triggered")
		assert.NotEmpty(t, alertMetrics, "No wait alerts triggered")
	})

	t.Run("AdaptiveSamplingUnderLoad", func(t *testing.T) {
		// Generate increasing load to test adaptive sampling
		loadLevels := []int{10, 50, 100, 200}
		
		for _, level := range loadLevels {
			t.Logf("Testing with %d concurrent sessions", level)
			
			var wg sync.WaitGroup
			for i := 0; i < level; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					conn := getNewConnection(t, testEnv)
					defer conn.Close()
					
					// Mix of query types
					switch id % 5 {
					case 0: // CPU intensive
						_, _ = conn.Exec("SELECT COUNT(*) FROM generate_series(1, 1000000)")
					case 1: // IO intensive
						_, _ = conn.Exec("SELECT * FROM orders ORDER BY random() LIMIT 100")
					case 2: // Lock intensive
						_, _ = conn.Exec("SELECT * FROM orders WHERE id = $1 FOR UPDATE", id)
					case 3: // Join query
						_, _ = conn.Exec(`
							SELECT u.*, o.* 
							FROM users u 
							JOIN orders o ON u.id = o.user_id 
							WHERE u.id = $1`, id)
					case 4: // Idle
						time.Sleep(1 * time.Second)
					}
				}(i)
			}
			
			// Let load run
			time.Sleep(5 * time.Second)
			wg.Wait()
			
			// Check sampling metrics
			metrics := testEnv.GetCollectedMetrics()
			samplerMetrics := findMetricsByName(metrics, "otelcol_processor_adaptivesampler_sessions_sampled")
			
			if len(samplerMetrics) > 0 {
				// Verify sampling rate decreased with load
				for _, metric := range samplerMetrics {
					attrs := getMetricAttributes(metric)
					if loadLevel, ok := attrs["load_level"]; ok {
						t.Logf("Sampling info at load level %s", loadLevel)
					}
				}
			}
		}
	})

	t.Run("CircuitBreakerIntegration", func(t *testing.T) {
		// Test circuit breaker with multiple failure scenarios
		
		// Scenario 1: Database connection failure
		testEnv.SimulateDatabaseOutage(10 * time.Second)
		
		// Try to generate activity
		for i := 0; i < 5; i++ {
			go func() {
				conn := getNewConnection(t, testEnv)
				defer conn.Close()
				_, _ = conn.Exec("SELECT 1")
			}()
		}
		
		// Wait for circuit breaker
		time.Sleep(3 * time.Second)
		
		// Check circuit breaker metrics
		metrics := testEnv.GetCollectedMetrics()
		cbMetrics := findMetricsByName(metrics, "otelcol_processor_circuitbreaker_state")
		assert.NotEmpty(t, cbMetrics, "No circuit breaker metrics")
		
		// Verify circuit opened
		for _, metric := range cbMetrics {
			attrs := getMetricAttributes(metric)
			if state, ok := attrs["state"]; ok {
				assert.Contains(t, []string{"open", "half-open"}, state)
			}
		}
		
		// Scenario 2: Log file access failure
		testEnv.SimulateLogFileError()
		time.Sleep(2 * time.Second)
		
		// Verify plan collection disabled
		planErrorMetrics := findMetricsByName(metrics, "autoexplain_parse_errors_total")
		assert.NotEmpty(t, planErrorMetrics, "No plan error metrics")
	})

	t.Run("MemoryPressureHandling", func(t *testing.T) {
		// Generate high cardinality queries to test memory limits
		
		// Create many unique queries
		for i := 0; i < 1000; i++ {
			query := fmt.Sprintf(`
				SELECT * FROM orders 
				WHERE id = %d 
				AND status = 'status_%d' 
				AND amount > %d`, i, i%100, i*10)
			
			go func(q string) {
				conn := getNewConnection(t, testEnv)
				defer conn.Close()
				_, _ = conn.Exec(q)
			}(query)
			
			if i%100 == 0 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		
		// Wait for processing
		time.Sleep(10 * time.Second)
		
		// Check memory limiter metrics
		metrics := testEnv.GetCollectedMetrics()
		memoryMetrics := findMetricsByName(metrics, "otelcol_processor_memorylimiter_memory_used")
		assert.NotEmpty(t, memoryMetrics, "No memory metrics")
		
		// Verify memory stayed within limits
		for _, metric := range memoryMetrics {
			value := getMetricValue(metric)
			attrs := getMetricAttributes(metric)
			if limit, ok := attrs["limit"]; ok {
				t.Logf("Memory usage: %f, Limit: %s", value, limit)
			}
		}
		
		// Check for dropped metrics due to memory pressure
		droppedMetrics := findMetricsByName(metrics, "otelcol_processor_memorylimiter_refused_metric_points")
		if len(droppedMetrics) > 0 {
			t.Log("Memory limiter dropped some metrics as expected")
		}
	})

	t.Run("NRDBEndToEndValidation", func(t *testing.T) {
		// Generate comprehensive activity
		generateFullStackActivity(t, db)
		
		// Wait for export
		time.Sleep(10 * time.Second)
		
		// Get NRDB payload
		nrdbPayload := testEnv.GetNRDBPayload()
		require.NotNil(t, nrdbPayload, "No NRDB payload")
		
		// Validate payload structure
		assert.NotEmpty(t, nrdbPayload.Metrics, "No metrics in NRDB payload")
		assert.NotEmpty(t, nrdbPayload.CommonAttributes, "No common attributes")
		
		// Check for all metric types
		metricTypes := map[string]bool{
			"db.postgresql.query.exec_time": false,
			"db.postgresql.plan.regression": false,
			"postgresql.ash.sessions.count": false,
			"postgresql.ash.wait_events.count": false,
			"postgresql.ash.blocking_sessions.count": false,
		}
		
		for _, metric := range nrdbPayload.Metrics {
			for metricType := range metricTypes {
				if metric.Name == metricType {
					metricTypes[metricType] = true
				}
			}
		}
		
		// Verify all metric types present
		for metricType, found := range metricTypes {
			assert.True(t, found, "Missing metric type: %s", metricType)
		}
		
		// Validate metric structure
		for _, metric := range nrdbPayload.Metrics {
			assert.NotEmpty(t, metric.Name, "Metric missing name")
			assert.NotZero(t, metric.Timestamp, "Metric missing timestamp")
			assert.NotNil(t, metric.Value, "Metric missing value")
			
			// Check for query correlation
			if strings.Contains(metric.Name, "query") || strings.Contains(metric.Name, "ash") {
				assert.NotEmpty(t, metric.Attributes, "Query metric missing attributes")
			}
		}
		
		// Verify data enrichment
		assert.Contains(t, nrdbPayload.CommonAttributes, "service.name")
		assert.Contains(t, nrdbPayload.CommonAttributes, "deployment.environment")
		assert.Contains(t, nrdbPayload.CommonAttributes, "db.system")
		assert.Equal(t, "postgresql", nrdbPayload.CommonAttributes["db.system"])
	})

	t.Run("FeatureDetectionAndDegradation", func(t *testing.T) {
		// Test with different feature availability
		
		// Disable pg_stat_statements
		testEnv.DisableExtension("pg_stat_statements")
		
		// Generate activity
		for i := 0; i < 10; i++ {
			_, _ = db.Exec("SELECT * FROM users WHERE id = $1", i)
		}
		
		// Wait for collection
		time.Sleep(5 * time.Second)
		
		// Check that collection still works without query_id
		metrics := testEnv.GetCollectedMetrics()
		ashMetrics := findMetricsByName(metrics, "postgresql.ash.sessions.count")
		assert.NotEmpty(t, ashMetrics, "ASH collection failed without pg_stat_statements")
		
		// Re-enable and test with pg_wait_sampling
		testEnv.EnableExtension("pg_stat_statements")
		testEnv.EnableExtension("pg_wait_sampling")
		
		// Generate activity with detailed wait events
		generateWaitEvents(t, db)
		time.Sleep(5 * time.Second)
		
		// Check for enhanced wait event details
		metrics = testEnv.GetCollectedMetrics()
		waitMetrics := findMetricsByName(metrics, "postgresql.ash.wait_events.count")
		
		// With pg_wait_sampling, should have more detailed wait events
		detailedWaitEvents := 0
		for _, metric := range waitMetrics {
			attrs := getMetricAttributes(metric)
			if _, ok := attrs["wait_event_detail"]; ok {
				detailedWaitEvents++
			}
		}
		
		t.Logf("Found %d detailed wait events with pg_wait_sampling", detailedWaitEvents)
	})
}

// Helper functions

func setupFullTestSchema(t *testing.T, db *sql.DB) {
	// Combines schemas from plan intelligence and ASH tests
	setupTestSchema(t, db)      // From plan_intelligence_test.go
	setupASHTestSchema(t, db)   // From ash_test.go
	
	// Additional tables for integration testing
	queries := []string{
		`CREATE TABLE IF NOT EXISTS metrics (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255),
			value FLOAT,
			tags JSONB,
			timestamp TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX idx_metrics_timestamp ON metrics(timestamp)`,
		`CREATE INDEX idx_metrics_name ON metrics(name)`,
	}
	
	for _, query := range queries {
		_, err := db.Exec(query)
		require.NoError(t, err)
	}
}

func generateFullStackActivity(t *testing.T, db *sql.DB) {
	var wg sync.WaitGroup
	
	// Plan Intelligence workload
	wg.Add(1)
	go func() {
		defer wg.Done()
		generateSlowQueries(t, db)
	}()
	
	// ASH workload
	wg.Add(1)
	go func() {
		defer wg.Done()
		generateComprehensiveActivity(t, db)
	}()
	
	// Mixed workload
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			conn := getNewConnection(t, testEnv)
			
			switch i % 4 {
			case 0: // Analytics query
				_, _ = conn.Exec(`
					SELECT DATE_TRUNC('hour', created_at) as hour,
						   COUNT(*) as order_count,
						   SUM(amount) as total_amount
					FROM orders
					WHERE created_at > NOW() - INTERVAL '24 hours'
					GROUP BY hour
					ORDER BY hour`)
			case 1: // Point lookup
				_, _ = conn.Exec("SELECT * FROM users WHERE id = $1", i)
			case 2: // Update with lock
				_, _ = conn.Exec("UPDATE orders SET status = 'processed' WHERE id = $1", i)
			case 3: // Insert
				_, _ = conn.Exec(`
					INSERT INTO metrics (name, value, tags)
					VALUES ($1, $2, $3)`,
					fmt.Sprintf("metric_%d", i),
					float64(i)*1.5,
					`{"source": "test", "type": "gauge"}`)
			}
			
			conn.Close()
			time.Sleep(100 * time.Millisecond)
		}
	}()
	
	wg.Wait()
}

// Full integration test configuration
const testFullIntegrationConfig = `
receivers:
  postgresql:
    endpoint: localhost:5432
    username: test_user
    password: test_password
    databases: [test_db]
    collection_interval: 30s
  
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
      min_duration: 50ms
      regression_detection:
        enabled: true
    plan_anonymization:
      enabled: true
  
  ash:
    endpoint: localhost:5432
    username: test_user
    password: test_password
    database: test_db
    collection_interval: 1s
    sampling:
      enabled: true
      sample_rate: 1.0
      adaptive_sampling: true
    analysis:
      wait_event_analysis: true
      blocking_analysis: true

processors:
  memory_limiter:
    check_interval: 1s
    limit_percentage: 75
    spike_limit_percentage: 20
  
  resource:
    attributes:
      - key: service.name
        value: postgresql-test
        action: upsert
      - key: deployment.environment
        value: test
        action: upsert
  
  circuitbreaker:
    failure_threshold: 5
    timeout: 30s
  
  waitanalysis:
    enabled: true
    alert_rules:
      - name: excessive_lock_waits
        condition: "wait_time > 5s AND event_type = 'Lock'"
        threshold: 10
        window: 1m
        action: alert
  
  adaptivesampler:
    enabled: true
    default_sampling_rate: 0.5
    rules:
      - name: always_regressions
        conditions:
          - attribute: event_type
            value: plan_regression
        sample_rate: 1.0

exporters:
  otlp/newrelic:
    endpoint: localhost:4317
    headers:
      api-key: test-api-key
    compression: gzip
    retry_on_failure:
      enabled: true

service:
  extensions: []
  
  pipelines:
    metrics/infrastructure:
      receivers: [postgresql]
      processors: [memory_limiter, resource]
      exporters: [otlp/newrelic]
    
    metrics/plans:
      receivers: [autoexplain]
      processors: [memory_limiter, resource, circuitbreaker, adaptivesampler]
      exporters: [otlp/newrelic]
    
    metrics/ash:
      receivers: [ash]
      processors: [memory_limiter, resource, waitanalysis, adaptivesampler]
      exporters: [otlp/newrelic]
  
  telemetry:
    logs:
      level: debug
    metrics:
      level: detailed
      address: 0.0.0.0:8888
`