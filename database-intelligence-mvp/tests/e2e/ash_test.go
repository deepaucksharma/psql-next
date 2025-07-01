package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TestASHE2E tests the complete ASH flow from PostgreSQL to NRDB
func TestASHE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()
	testEnv := setupTestEnvironment(t)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupASHTestSchema(t, db)

	// Start collector with ASH configuration
	collector := testEnv.StartCollector(t, "testdata/config-ash.yaml")
	defer collector.Shutdown()

	// Wait for collector to be ready
	require.Eventually(t, func() bool {
		return collector.IsHealthy()
	}, 30*time.Second, 1*time.Second)

	t.Run("SessionSampling", func(t *testing.T) {
		// Create multiple concurrent sessions
		var wg sync.WaitGroup
		sessionCount := 20

		for i := 0; i < sessionCount; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				conn := getNewConnection(t, testEnv)
				defer conn.Close()

				// Execute queries with different patterns
				switch id % 4 {
				case 0: // Active query
					_, _ = conn.Exec("SELECT pg_sleep(2)")
				case 1: // Blocked session
					simulateBlockedSession(t, conn, id)
				case 2: // Idle in transaction
					tx, _ := conn.Begin()
					_, _ = tx.Exec("SELECT 1")
					time.Sleep(3 * time.Second)
					tx.Rollback()
				case 3: // Normal query
					_, _ = conn.Exec("SELECT COUNT(*) FROM ash_test_table")
				}
			}(i)
		}

		// Let sessions run
		time.Sleep(5 * time.Second)

		// Check collected metrics
		metrics := testEnv.GetCollectedMetrics()

		// Verify session count metrics
		sessionMetrics := findMetricsByName(metrics, "postgresql.ash.sessions.count")
		assert.NotEmpty(t, sessionMetrics, "No session count metrics collected")

		// Check for different session states
		states := make(map[string]bool)
		for _, metric := range sessionMetrics {
			attrs := getMetricAttributes(metric)
			if state, ok := attrs["state"]; ok {
				states[state] = true
			}
		}
		assert.True(t, states["active"], "No active sessions detected")
		assert.True(t, states["idle in transaction"], "No idle in transaction sessions detected")

		wg.Wait()
	})

	t.Run("WaitEventAnalysis", func(t *testing.T) {
		// Generate different wait events
		generateWaitEvents(t, db)

		// Wait for collection
		time.Sleep(3 * time.Second)

		metrics := testEnv.GetCollectedMetrics()

		// Check wait event metrics
		waitMetrics := findMetricsByName(metrics, "postgresql.ash.wait_events.count")
		assert.NotEmpty(t, waitMetrics, "No wait event metrics collected")

		// Verify wait event categorization
		categories := make(map[string]bool)
		for _, metric := range waitMetrics {
			attrs := getMetricAttributes(metric)
			if category, ok := attrs["category"]; ok {
				categories[category] = true
			}
			// Check for severity
			assert.Contains(t, attrs, "severity", "Missing severity attribute")
		}

		// Should have multiple categories
		assert.True(t, len(categories) >= 2, "Expected multiple wait event categories")

		// Check category summary metrics
		categoryMetrics := findMetricsByName(metrics, "postgresql.ash.wait_category.count")
		assert.NotEmpty(t, categoryMetrics, "No wait category metrics collected")
	})

	t.Run("BlockingDetection", func(t *testing.T) {
		// Create blocking chain
		blockingConn := getNewConnection(t, testEnv)
		blockedConn1 := getNewConnection(t, testEnv)
		blockedConn2 := getNewConnection(t, testEnv)

		// Start transaction that holds lock
		tx, err := blockingConn.Begin()
		require.NoError(t, err)
		_, err = tx.Exec("UPDATE ash_test_table SET value = value + 1 WHERE id = 1")
		require.NoError(t, err)

		// Try to update same row from other connections (will block)
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			_, _ = blockedConn1.Exec("UPDATE ash_test_table SET value = value + 2 WHERE id = 1")
		}()

		go func() {
			defer wg.Done()
			_, _ = blockedConn2.Exec("UPDATE ash_test_table SET value = value + 3 WHERE id = 1")
		}()

		// Let blocking occur
		time.Sleep(3 * time.Second)

		// Check blocking metrics
		metrics := testEnv.GetCollectedMetrics()

		blockingMetrics := findMetricsByName(metrics, "postgresql.ash.blocking_sessions.count")
		assert.NotEmpty(t, blockingMetrics, "No blocking session metrics")

		blockedMetrics := findMetricsByName(metrics, "postgresql.ash.blocked_sessions.count")
		assert.NotEmpty(t, blockedMetrics, "No blocked session metrics")

		// Verify counts
		for _, metric := range blockedMetrics {
			value := getMetricValue(metric)
			assert.GreaterOrEqual(t, value, 2.0, "Should detect at least 2 blocked sessions")
		}

		// Release lock
		tx.Rollback()
		wg.Wait()

		blockingConn.Close()
		blockedConn1.Close()
		blockedConn2.Close()
	})

	t.Run("AdaptiveSampling", func(t *testing.T) {
		// Create many sessions to trigger adaptive sampling
		var connections []*sql.DB
		sessionTarget := 100

		// Create sessions gradually
		for i := 0; i < sessionTarget; i++ {
			conn := getNewConnection(t, testEnv)
			connections = append(connections, conn)
			
			// Execute light query
			go func(c *sql.DB) {
				_, _ = c.Exec("SELECT 1")
			}(conn)
			
			if i%10 == 0 {
				time.Sleep(100 * time.Millisecond)
			}
		}

		// Wait for sampling adjustment
		time.Sleep(5 * time.Second)

		// Check sampler metrics
		metrics := testEnv.GetCollectedMetrics()
		samplerMetrics := findMetricsByName(metrics, "otelcol_processor_adaptivesampler_sample_rate")
		assert.NotEmpty(t, samplerMetrics, "No adaptive sampler metrics")

		// Clean up connections
		for _, conn := range connections {
			conn.Close()
		}
	})

	t.Run("QueryActivityTracking", func(t *testing.T) {
		// Execute same query from multiple sessions
		queryID := executeTrackedQuery(t, db, 5)

		// Wait for collection
		time.Sleep(3 * time.Second)

		metrics := testEnv.GetCollectedMetrics()

		// Check query activity metrics
		queryMetrics := findMetricsByName(metrics, "postgresql.ash.query.active_count")
		assert.NotEmpty(t, queryMetrics, "No query activity metrics")

		// Find our query
		found := false
		for _, metric := range queryMetrics {
			attrs := getMetricAttributes(metric)
			if attrs["query_id"] == queryID {
				found = true
				value := getMetricValue(metric)
				assert.GreaterOrEqual(t, value, 3.0, "Should detect multiple active sessions for query")
			}
		}
		assert.True(t, found, "Tracked query not found in metrics")

		// Check query duration metrics
		durationMetrics := findMetricsByName(metrics, "postgresql.ash.query.duration")
		assert.NotEmpty(t, durationMetrics, "No query duration metrics")
	})

	t.Run("TimeWindowAggregation", func(t *testing.T) {
		// Generate activity over time
		for i := 0; i < 10; i++ {
			generateBurstActivity(t, db, 10)
			time.Sleep(30 * time.Second)
		}

		// Check aggregated metrics
		metrics := testEnv.GetCollectedMetrics()

		// Look for aggregation window indicators
		for _, metric := range metrics {
			attrs := getMetricAttributes(metric)
			if window, ok := attrs["aggregation_window"]; ok {
				// Verify different windows exist
				assert.Contains(t, []string{"1m", "5m", "15m", "1h"}, window)
			}
		}
	})

	t.Run("NRDBExportWithASH", func(t *testing.T) {
		// Generate comprehensive activity
		generateComprehensiveActivity(t, db)

		// Wait for export
		time.Sleep(5 * time.Second)

		nrdbPayload := testEnv.GetNRDBPayload()
		require.NotNil(t, nrdbPayload)

		// Verify ASH metrics in NRDB format
		ashMetrics := filterNRDBMetricsByPrefix(nrdbPayload.Metrics, "postgresql.ash")
		assert.NotEmpty(t, ashMetrics, "No ASH metrics in NRDB payload")

		// Check metric diversity
		metricTypes := make(map[string]bool)
		for _, metric := range ashMetrics {
			metricTypes[metric.Name] = true
		}
		assert.GreaterOrEqual(t, len(metricTypes), 5, "Expected diverse ASH metric types")

		// Verify dimensional data
		for _, metric := range ashMetrics {
			// ASH metrics should have rich dimensions
			assert.NotEmpty(t, metric.Attributes, "ASH metric missing attributes")
			
			// Common attributes
			assert.Contains(t, metric.CommonAttributes, "service.name")
			assert.Contains(t, metric.CommonAttributes, "db.system")
		}
	})

	t.Run("WaitEventAlerts", func(t *testing.T) {
		// Generate excessive lock waits
		generateExcessiveLockWaits(t, db, 20)

		// Wait for alert processing
		time.Sleep(5 * time.Second)

		// Check for alert metrics
		metrics := testEnv.GetCollectedMetrics()
		alertMetrics := findMetricsByName(metrics, "postgresql.ash.wait_alert.triggered")
		assert.NotEmpty(t, alertMetrics, "No wait event alerts triggered")

		// Verify alert attributes
		for _, metric := range alertMetrics {
			attrs := getMetricAttributes(metric)
			assert.Contains(t, attrs, "rule", "Missing alert rule name")
			assert.Contains(t, attrs, "severity", "Missing alert severity")
		}
	})
}

// Helper functions

func setupASHTestSchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS ash_test_table (
			id SERIAL PRIMARY KEY,
			value INT,
			data TEXT
		)`,
		`INSERT INTO ash_test_table (value, data)
		 SELECT i, 'test data ' || i
		 FROM generate_series(1, 1000) i`,
		`CREATE INDEX idx_ash_test_value ON ash_test_table(value)`,
		`ANALYZE ash_test_table`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		require.NoError(t, err)
	}
}

func getNewConnection(t *testing.T, env *TestEnvironment) *sql.DB {
	conn, err := sql.Open("postgres", env.PostgresDSN)
	require.NoError(t, err)
	return conn
}

func simulateBlockedSession(t *testing.T, conn *sql.DB, id int) {
	// Try to acquire a lock that's held by another session
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	_, err := conn.ExecContext(ctx, 
		"SELECT pg_advisory_lock($1)", 
		12345) // Common lock ID to create contention
	
	if err == nil {
		// If we got the lock, hold it briefly
		time.Sleep(1 * time.Second)
		conn.Exec("SELECT pg_advisory_unlock($1)", 12345)
	}
}

func generateWaitEvents(t *testing.T, db *sql.DB) {
	// IO waits
	go func() {
		conn := getNewConnection(t, testEnv)
		defer conn.Close()
		_, _ = conn.Exec("SELECT * FROM ash_test_table ORDER BY random()")
	}()

	// Lock waits
	go func() {
		conn := getNewConnection(t, testEnv)
		defer conn.Close()
		_, _ = conn.Exec("SELECT * FROM ash_test_table FOR UPDATE")
	}()

	// Client waits
	go func() {
		conn := getNewConnection(t, testEnv)
		defer conn.Close()
		_, _ = conn.Exec("COPY ash_test_table TO STDOUT")
	}()
}

func executeTrackedQuery(t *testing.T, db *sql.DB, concurrency int) string {
	query := `
		SELECT t1.*, t2.value as related_value
		FROM ash_test_table t1
		JOIN ash_test_table t2 ON t1.value = t2.value
		WHERE t1.id < 100
		ORDER BY t1.value DESC
		LIMIT 50`

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			_, _ = conn.Exec(query)
			time.Sleep(2 * time.Second) // Keep query active
		}()
	}

	// Get query ID (simplified - in real test would query pg_stat_statements)
	var queryID string
	err := db.QueryRow(`
		SELECT queryid::text 
		FROM pg_stat_statements 
		WHERE query LIKE '%ash_test_table t1%' 
		LIMIT 1`).Scan(&queryID)
	
	if err != nil {
		queryID = "test_query_id"
	}

	wg.Wait()
	return queryID
}

func generateBurstActivity(t *testing.T, db *sql.DB, sessionCount int) {
	var wg sync.WaitGroup
	for i := 0; i < sessionCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			
			// Random activity
			switch id % 3 {
			case 0:
				_, _ = conn.Exec("UPDATE ash_test_table SET value = value + 1 WHERE id = $1", id)
			case 1:
				_, _ = conn.Exec("SELECT COUNT(*) FROM ash_test_table WHERE value > $1", id*10)
			case 2:
				_, _ = conn.Exec("INSERT INTO ash_test_table (value, data) VALUES ($1, $2)", 
					id, fmt.Sprintf("burst data %d", id))
			}
		}(i)
	}
	wg.Wait()
}

func generateComprehensiveActivity(t *testing.T, db *sql.DB) {
	// Mix of different activities
	activities := []func(){
		// Long running queries
		func() {
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			_, _ = conn.Exec("SELECT pg_sleep(3)")
		},
		// Blocking chains
		func() {
			simulateBlockingChain(t, db)
		},
		// High frequency short queries
		func() {
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			for i := 0; i < 50; i++ {
				_, _ = conn.Exec("SELECT 1")
				time.Sleep(10 * time.Millisecond)
			}
		},
		// DDL operations
		func() {
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			_, _ = conn.Exec("CREATE TEMP TABLE ash_temp AS SELECT * FROM ash_test_table LIMIT 10")
			_, _ = conn.Exec("DROP TABLE ash_temp")
		},
	}

	var wg sync.WaitGroup
	for _, activity := range activities {
		wg.Add(1)
		go func(fn func()) {
			defer wg.Done()
			fn()
		}(activity)
	}
	wg.Wait()
}

func simulateBlockingChain(t *testing.T, db *sql.DB) {
	// Create a chain: conn1 blocks conn2, conn2 blocks conn3
	conn1 := getNewConnection(t, testEnv)
	conn2 := getNewConnection(t, testEnv)
	conn3 := getNewConnection(t, testEnv)
	defer conn1.Close()
	defer conn2.Close()
	defer conn3.Close()

	// Transaction 1: lock row 1
	tx1, _ := conn1.Begin()
	tx1.Exec("UPDATE ash_test_table SET value = 100 WHERE id = 1")

	// Transaction 2: lock row 2, then try to lock row 1 (blocked by tx1)
	tx2, _ := conn2.Begin()
	tx2.Exec("UPDATE ash_test_table SET value = 200 WHERE id = 2")
	go func() {
		tx2.Exec("UPDATE ash_test_table SET value = 201 WHERE id = 1")
	}()

	// Transaction 3: try to lock row 2 (blocked by tx2)
	go func() {
		conn3.Exec("UPDATE ash_test_table SET value = 300 WHERE id = 2")
	}()

	// Let the chain exist for a bit
	time.Sleep(2 * time.Second)

	// Release locks in reverse order
	tx1.Rollback()
	tx2.Rollback()
}

func generateExcessiveLockWaits(t *testing.T, db *sql.DB, sessionCount int) {
	// Hold a lock on a popular row
	lockConn := getNewConnection(t, testEnv)
	tx, _ := lockConn.Begin()
	tx.Exec("UPDATE ash_test_table SET value = 999 WHERE id = 1")

	// Many sessions try to update the same row
	var wg sync.WaitGroup
	for i := 0; i < sessionCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			
			_, _ = conn.ExecContext(ctx, 
				"UPDATE ash_test_table SET value = $1 WHERE id = 1", 
				id)
		}(i)
	}

	// Let waits accumulate
	time.Sleep(6 * time.Second)

	// Release lock
	tx.Rollback()
	lockConn.Close()
	wg.Wait()
}

// Test configuration for ASH
const testASHConfig = `
receivers:
  ash:
    endpoint: localhost:5432
    username: test_user
    password: test_password
    database: test_db
    
    collection_interval: 1s
    retention_duration: 30m
    
    sampling:
      enabled: true
      sample_rate: 1.0  # 100% for testing
      active_session_rate: 1.0
      blocked_session_rate: 1.0
      long_running_threshold: 2s
      adaptive_sampling: true
    
    storage:
      buffer_size: 1800
      aggregation_windows: [1m, 5m, 15m, 1h]
      compression_enabled: true
    
    analysis:
      wait_event_analysis: true
      blocking_analysis: true
      resource_analysis: true
      anomaly_detection: true
      top_query_analysis: true

processors:
  waitanalysis:
    enabled: true
    patterns:
      - name: lock_waits
        event_types: ["Lock"]
        category: "Concurrency"
        severity: "warning"
    
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
      - name: blocked_sessions
        conditions:
          - attribute: blocked
            value: true
        sample_rate: 1.0

exporters:
  otlp/newrelic:
    endpoint: localhost:4317
    headers:
      api-key: test-api-key

service:
  pipelines:
    metrics:
      receivers: [ash]
      processors: [waitanalysis, adaptivesampler]
      exporters: [otlp/newrelic]
`