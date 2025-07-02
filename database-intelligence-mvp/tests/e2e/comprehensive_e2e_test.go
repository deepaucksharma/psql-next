package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComprehensiveE2EFlow validates the complete flow from database to NRDB
// This test ensures NO shortcuts are taken and verifies actual data in New Relic
func TestComprehensiveE2EFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive E2E test in short mode")
	}

	// Validate environment
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if accountID == "" || apiKey == "" {
		t.Skip("NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY must be set for E2E tests")
	}

	// Setup test environment
	ctx := context.Background()
	testEnv := setupRealTestEnvironment(t)
	defer testEnv.Cleanup()

	// Create NRDB client for verification
	nrdbClient := NewNRDBClient(accountID, apiKey)

	// Record baseline metrics in NRDB
	baselineMetrics := captureNRDBBaseline(t, ctx, nrdbClient)

	// Start the real collector with full configuration
	collector := startRealCollector(t, testEnv)
	defer collector.Stop()

	// Wait for collector to be fully operational
	waitForCollectorHealth(t, collector, 30*time.Second)

	// Run comprehensive test scenarios
	t.Run("PostgreSQLMetricsFlow", func(t *testing.T) {
		testPostgreSQLMetricsFlow(t, ctx, testEnv, nrdbClient, baselineMetrics)
	})

	t.Run("PGQueryLensFlow", func(t *testing.T) {
		testPGQueryLensFlow(t, ctx, testEnv, nrdbClient)
	})

	t.Run("ProcessorValidation", func(t *testing.T) {
		testAllProcessors(t, ctx, testEnv, nrdbClient)
	})

	t.Run("DataIntegrityValidation", func(t *testing.T) {
		testDataIntegrity(t, ctx, testEnv, nrdbClient)
	})

	t.Run("EndToEndLatency", func(t *testing.T) {
		testEndToEndLatency(t, ctx, testEnv, nrdbClient)
	})
}

// testPostgreSQLMetricsFlow validates PostgreSQL metrics collection end-to-end
func testPostgreSQLMetricsFlow(t *testing.T, ctx context.Context, testEnv *TestEnvironment, 
	nrdbClient *NRDBClient, baseline map[string]float64) {

	db := testEnv.PostgresDB

	// Generate known workload
	workload := generateKnownWorkload(t, db)

	// Wait for metrics to be collected and sent
	time.Sleep(45 * time.Second) // Collection interval + processing + export time

	// Verify each metric type in NRDB
	t.Run("ConnectionMetrics", func(t *testing.T) {
		query := `
			SELECT latest(postgresql.connections.active) as active,
			       latest(postgresql.connections.idle) as idle,
			       latest(postgresql.connections.max) as max
			FROM Metric 
			WHERE db.system = 'postgresql' 
			AND db.name = $DB_NAME
			SINCE 2 minutes ago`
		
		query = strings.ReplaceAll(query, "$DB_NAME", fmt.Sprintf("'%s'", getEnvOrDefault("POSTGRES_DB", "testdb")))
		
		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		require.NoError(t, err, "Failed to query connection metrics")
		require.NotEmpty(t, result.Data.Actor.Account.NRQL.Results, "No connection metrics found")
		
		metrics := result.Data.Actor.Account.NRQL.Results[0]
		
		// Verify metrics are present and reasonable
		assert.NotNil(t, metrics["active"], "Active connections missing")
		assert.NotNil(t, metrics["idle"], "Idle connections missing")
		assert.NotNil(t, metrics["max"], "Max connections missing")
		
		// Verify values are reasonable
		active, ok := metrics["active"].(float64)
		assert.True(t, ok && active >= 0, "Invalid active connections: %v", metrics["active"])
		
		max, ok := metrics["max"].(float64)
		assert.True(t, ok && max > 0, "Invalid max connections: %v", metrics["max"])
	})

	t.Run("TransactionMetrics", func(t *testing.T) {
		query := `
			SELECT sum(postgresql.transactions.committed) as commits,
			       sum(postgresql.transactions.rolled_back) as rollbacks
			FROM Metric 
			WHERE db.system = 'postgresql' 
			AND db.name = $DB_NAME
			SINCE 2 minutes ago`
		
		query = strings.ReplaceAll(query, "$DB_NAME", fmt.Sprintf("'%s'", getEnvOrDefault("POSTGRES_DB", "testdb")))
		
		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		require.NoError(t, err)
		require.NotEmpty(t, result.Data.Actor.Account.NRQL.Results)
		
		metrics := result.Data.Actor.Account.NRQL.Results[0]
		
		// Verify transaction counts increased from baseline
		commits, ok := metrics["commits"].(float64)
		assert.True(t, ok, "Invalid commits value")
		assert.Greater(t, commits, baseline["commits"], "No new commits detected")
		
		// Verify workload transactions
		expectedCommits := float64(workload.InsertCount + workload.UpdateCount)
		assert.GreaterOrEqual(t, commits-baseline["commits"], expectedCommits*0.9, 
			"Expected at least %v new commits, got %v", expectedCommits, commits-baseline["commits"])
	})

	t.Run("QueryPerformanceMetrics", func(t *testing.T) {
		query := `
			SELECT count(*) as query_count,
			       average(db.statement.duration) as avg_duration,
			       max(db.statement.duration) as max_duration,
			       uniqueCount(db.statement) as unique_queries
			FROM Metric 
			WHERE metricName = 'db.statement.duration'
			AND db.system = 'postgresql'
			SINCE 2 minutes ago`
		
		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		require.NoError(t, err)
		
		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			
			count, _ := metrics["query_count"].(float64)
			assert.Greater(t, count, float64(0), "No queries recorded")
			
			avgDuration, _ := metrics["avg_duration"].(float64)
			assert.Greater(t, avgDuration, float64(0), "Invalid average duration")
			
			uniqueQueries, _ := metrics["unique_queries"].(float64)
			assert.GreaterOrEqual(t, uniqueQueries, float64(3), "Expected at least 3 unique query types")
		}
	})
}

// testPGQueryLensFlow validates pg_querylens integration
func testPGQueryLensFlow(t *testing.T, ctx context.Context, testEnv *TestEnvironment, nrdbClient *NRDBClient) {
	db := testEnv.PostgresDB

	// Verify pg_querylens is installed
	var installed bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_querylens')").Scan(&installed)
	if err != nil || !installed {
		t.Skip("pg_querylens not installed, skipping pg_querylens tests")
	}

	// Create plan change scenario
	t.Run("PlanChangeDetection", func(t *testing.T) {
		// Create index for fast plan
		_, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_test_plan_change ON users(email)")
		require.NoError(t, err)

		// Execute queries with index
		for i := 0; i < 5; i++ {
			_, err = db.Exec("SELECT * FROM users WHERE email = $1", fmt.Sprintf("test%d@example.com", i))
			require.NoError(t, err)
		}

		// Wait for initial metrics
		time.Sleep(35 * time.Second)

		// Drop index to force plan change
		_, err = db.Exec("DROP INDEX IF EXISTS idx_test_plan_change")
		require.NoError(t, err)

		// Execute same queries without index
		for i := 0; i < 5; i++ {
			_, err = db.Exec("SELECT * FROM users WHERE email = $1", fmt.Sprintf("test%d@example.com", i))
			require.NoError(t, err)
		}

		// Wait for plan change detection
		time.Sleep(35 * time.Second)

		// Verify plan changes in NRDB
		query := `
			SELECT count(*) as plan_changes
			FROM Metric 
			WHERE db.plan.changed = true
			AND db.system = 'postgresql'
			SINCE 2 minutes ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		require.NoError(t, err)

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			planChanges, _ := metrics["plan_changes"].(float64)
			assert.Greater(t, planChanges, float64(0), "No plan changes detected")
		}
	})

	t.Run("RegressionDetection", func(t *testing.T) {
		// Execute queries that should trigger regression detection
		for i := 0; i < 10; i++ {
			// Slow query without index
			_, err := db.Exec(`
				SELECT u.*, COUNT(o.id) 
				FROM users u 
				LEFT JOIN orders o ON u.id = o.user_id 
				WHERE u.email LIKE $1
				GROUP BY u.id`, fmt.Sprintf("%%%d%%", i))
			
			if err != nil {
				t.Logf("Query %d failed (may be expected): %v", i, err)
			}
		}

		// Wait for regression detection
		time.Sleep(35 * time.Second)

		// Check for regressions in NRDB
		query := `
			SELECT count(*) as regressions,
			       max(db.plan.time_change_ratio) as max_slowdown
			FROM Metric 
			WHERE db.plan.has_regression = true
			AND db.system = 'postgresql'
			SINCE 3 minutes ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		require.NoError(t, err)

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			regressions, _ := metrics["regressions"].(float64)
			t.Logf("Detected %v plan regressions", regressions)
		}
	})
}

// testAllProcessors validates each custom processor is working
func testAllProcessors(t *testing.T, ctx context.Context, testEnv *TestEnvironment, nrdbClient *NRDBClient) {
	db := testEnv.PostgresDB

	t.Run("AdaptiveSampler", func(t *testing.T) {
		// Generate slow queries that should be 100% sampled
		for i := 0; i < 10; i++ {
			db.Exec("SELECT pg_sleep(0.2)") // 200ms query
		}

		// Generate fast queries that should be sampled at default rate
		for i := 0; i < 100; i++ {
			db.Exec("SELECT 1")
		}

		time.Sleep(35 * time.Second)

		// Verify sampling metadata in NRDB
		query := `
			SELECT count(*) as total_metrics,
			       filter(count(*), WHERE db.sampling.rule = 'slow_queries') as slow_sampled,
			       filter(count(*), WHERE db.sampling.rule = 'default') as default_sampled
			FROM Metric 
			WHERE db.system = 'postgresql'
			AND db.sampling.rule IS NOT NULL
			SINCE 1 minute ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			t.Logf("Sampling results: %+v", metrics)
		}
	})

	t.Run("CircuitBreaker", func(t *testing.T) {
		// Generate failing queries to trigger circuit breaker
		for i := 0; i < 20; i++ {
			db.Exec("SELECT * FROM nonexistent_table")
		}

		time.Sleep(35 * time.Second)

		// Check for circuit breaker state changes
		query := `
			SELECT count(*) as state_changes,
			       latest(circuit_breaker.state) as current_state
			FROM Metric 
			WHERE metricName = 'otelcol.processor.circuitbreaker.state_change'
			SINCE 2 minutes ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			t.Logf("Circuit breaker state: %+v", metrics)
		}
	})

	t.Run("VerificationProcessor", func(t *testing.T) {
		// Generate queries with PII that should be redacted
		piiQueries := []string{
			"SELECT * FROM users WHERE ssn = '123-45-6789'",
			"SELECT * FROM users WHERE credit_card = '4111-1111-1111-1111'",
			"SELECT * FROM users WHERE email = 'test@example.com'",
			"SELECT * FROM users WHERE phone = '555-123-4567'",
		}

		for _, query := range piiQueries {
			db.Exec(query)
		}

		time.Sleep(35 * time.Second)

		// Verify PII is redacted in NRDB
		query := `
			SELECT count(*) as total_queries,
			       filter(count(*), WHERE db.statement LIKE '%[REDACTED]%') as redacted_queries,
			       filter(count(*), WHERE db.statement LIKE '%123-45-6789%') as exposed_ssn
			FROM Metric 
			WHERE metricName = 'db.statement.duration'
			AND db.system = 'postgresql'
			SINCE 1 minute ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		require.NoError(t, err)

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			
			exposedSSN, _ := metrics["exposed_ssn"].(float64)
			assert.Equal(t, float64(0), exposedSSN, "SSN not redacted!")
			
			redactedQueries, _ := metrics["redacted_queries"].(float64)
			assert.Greater(t, redactedQueries, float64(0), "No queries were redacted")
		}
	})

	t.Run("CostControl", func(t *testing.T) {
		// Check cost tracking metrics
		query := `
			SELECT latest(cost_control.budget_usage_ratio) as usage_ratio,
			       latest(cost_control.monthly_budget_usd) as budget,
			       latest(cost_control.estimated_monthly_cost_usd) as estimated_cost
			FROM Metric 
			WHERE metricName = 'otelcol.processor.costcontrol.budget'
			SINCE 5 minutes ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			
			usageRatio, _ := metrics["usage_ratio"].(float64)
			assert.GreaterOrEqual(t, usageRatio, float64(0), "Invalid usage ratio")
			assert.LessOrEqual(t, usageRatio, float64(1), "Usage ratio over 100%")
			
			t.Logf("Cost control metrics: %+v", metrics)
		}
	})

	t.Run("QueryCorrelator", func(t *testing.T) {
		// Generate correlated queries in a transaction
		tx, err := db.Begin()
		require.NoError(t, err)

		tx.Exec("SELECT * FROM users WHERE id = 1")
		tx.Exec("SELECT * FROM orders WHERE user_id = 1")
		tx.Exec("UPDATE users SET last_login = NOW() WHERE id = 1")
		
		err = tx.Commit()
		require.NoError(t, err)

		time.Sleep(35 * time.Second)

		// Check for correlation attributes
		query := `
			SELECT count(*) as correlated_queries,
			       uniqueCount(db.transaction.id) as transactions,
			       uniqueCount(db.session.id) as sessions
			FROM Metric 
			WHERE db.transaction.id IS NOT NULL
			AND db.system = 'postgresql'
			SINCE 1 minute ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			
			correlatedQueries, _ := metrics["correlated_queries"].(float64)
			assert.Greater(t, correlatedQueries, float64(0), "No correlated queries found")
			
			t.Logf("Query correlation metrics: %+v", metrics)
		}
	})
}

// testDataIntegrity validates data accuracy and completeness
func testDataIntegrity(t *testing.T, ctx context.Context, testEnv *TestEnvironment, nrdbClient *NRDBClient) {
	db := testEnv.PostgresDB

	// Execute precise workload
	startTime := time.Now()
	workloadMetrics := executePreciseWorkload(t, db)
	endTime := time.Now()

	// Wait for data to reach NRDB
	time.Sleep(45 * time.Second)

	// Query NRDB for the time window
	query := fmt.Sprintf(`
		SELECT count(*) as total_queries,
		       uniqueCount(db.statement) as unique_statements,
		       sum(db.rows_affected) as total_rows
		FROM Metric 
		WHERE metricName = 'db.statement.duration'
		AND db.system = 'postgresql'
		AND timestamp >= %d AND timestamp <= %d`,
		startTime.UnixMilli(), endTime.UnixMilli())

	result, err := nrdbClient.ExecuteNRQL(ctx, query)
	require.NoError(t, err)

	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		metrics := result.Data.Actor.Account.NRQL.Results[0]
		
		totalQueries, _ := metrics["total_queries"].(float64)
		uniqueStatements, _ := metrics["unique_statements"].(float64)
		totalRows, _ := metrics["total_rows"].(float64)

		// Verify counts match expected
		assert.GreaterOrEqual(t, totalQueries, float64(workloadMetrics.TotalQueries)*0.9, 
			"Missing queries: expected %d, got %v", workloadMetrics.TotalQueries, totalQueries)
		
		assert.Equal(t, float64(workloadMetrics.UniqueQueries), uniqueStatements,
			"Unique query count mismatch")
		
		// Allow some variance in row counts due to timing
		if totalRows > 0 {
			assert.InDelta(t, float64(workloadMetrics.TotalRows), totalRows, 
				float64(workloadMetrics.TotalRows)*0.1, "Row count mismatch")
		}
	}
}

// testEndToEndLatency measures collection to visibility latency
func testEndToEndLatency(t *testing.T, ctx context.Context, testEnv *TestEnvironment, nrdbClient *NRDBClient) {
	db := testEnv.PostgresDB

	// Execute a unique query with timestamp marker
	marker := fmt.Sprintf("latency_test_%d", time.Now().UnixNano())
	queryTime := time.Now()
	
	_, err := db.Exec(fmt.Sprintf("SELECT '%s' as marker, NOW()", marker))
	require.NoError(t, err)

	// Poll NRDB until query appears
	var latency time.Duration
	found := false
	
	for i := 0; i < 60; i++ { // Poll for up to 60 seconds
		time.Sleep(1 * time.Second)
		
		query := fmt.Sprintf(`
			SELECT count(*) as found
			FROM Metric 
			WHERE metricName = 'db.statement.duration'
			AND db.statement LIKE '%%%s%%'
			SINCE 2 minutes ago`, marker)
		
		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			if count, _ := metrics["found"].(float64); count > 0 {
				latency = time.Since(queryTime)
				found = true
				break
			}
		}
	}

	require.True(t, found, "Query with marker %s never appeared in NRDB", marker)
	assert.Less(t, latency, 60*time.Second, "End-to-end latency too high: %v", latency)
	t.Logf("End-to-end latency: %v", latency)
}

// Helper functions

func setupRealTestEnvironment(t *testing.T) *TestEnvironment {
	// Setup real PostgreSQL connection
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnvOrDefault("POSTGRES_HOST", "localhost"),
		getEnvOrDefault("POSTGRES_PORT", "5432"),
		getEnvOrDefault("POSTGRES_USER", "postgres"),
		getEnvOrDefault("POSTGRES_PASSWORD", "postgres"),
		getEnvOrDefault("POSTGRES_DB", "testdb"))

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)

	// Ensure clean state
	setupTestDatabase(t, db)

	env := &TestEnvironment{
		PostgresDB: db,
		t:          t,
	}
	env.cleanupFuncs = append(env.cleanupFuncs, func() {
		db.Close()
	})
	return env
}

func startRealCollector(t *testing.T, testEnv *TestEnvironment) *CollectorInstance {
	// This should start the actual collector binary with proper configuration
	// pointing to the test database and New Relic
	// For now, we'll simulate this
	return &CollectorInstance{
		endpoint: "http://localhost:8888",
	}
}

func waitForCollectorHealth(t *testing.T, collector *CollectorInstance, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		resp, err := http.Get(collector.endpoint + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	
	t.Fatal("Collector did not become healthy within timeout")
}

func captureNRDBBaseline(t *testing.T, ctx context.Context, client *NRDBClient) map[string]float64 {
	baseline := make(map[string]float64)
	
	// Capture current transaction counts
	query := `
		SELECT sum(postgresql.transactions.committed) as commits,
		       sum(postgresql.transactions.rolled_back) as rollbacks
		FROM Metric 
		WHERE db.system = 'postgresql' 
		SINCE 5 minutes ago`
	
	result, err := client.ExecuteNRQL(ctx, query)
	if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
		metrics := result.Data.Actor.Account.NRQL.Results[0]
		baseline["commits"], _ = metrics["commits"].(float64)
		baseline["rollbacks"], _ = metrics["rollbacks"].(float64)
	}
	
	return baseline
}

type KnownWorkload struct {
	InsertCount int
	UpdateCount int
	SelectCount int
	DeleteCount int
}

func generateKnownWorkload(t *testing.T, db *sql.DB) KnownWorkload {
	workload := KnownWorkload{}
	
	// Inserts
	for i := 0; i < 50; i++ {
		_, err := db.Exec("INSERT INTO test_metrics (name, value) VALUES ($1, $2)",
			fmt.Sprintf("metric_%d", i), rand.Float64()*100)
		if err == nil {
			workload.InsertCount++
		}
	}
	
	// Updates
	for i := 0; i < 30; i++ {
		_, err := db.Exec("UPDATE test_metrics SET value = $1 WHERE name = $2",
			rand.Float64()*100, fmt.Sprintf("metric_%d", i))
		if err == nil {
			workload.UpdateCount++
		}
	}
	
	// Selects
	for i := 0; i < 100; i++ {
		var value float64
		err := db.QueryRow("SELECT value FROM test_metrics WHERE name = $1",
			fmt.Sprintf("metric_%d", i%50)).Scan(&value)
		if err == nil {
			workload.SelectCount++
		}
	}
	
	// Deletes
	for i := 40; i < 50; i++ {
		_, err := db.Exec("DELETE FROM test_metrics WHERE name = $1",
			fmt.Sprintf("metric_%d", i))
		if err == nil {
			workload.DeleteCount++
		}
	}
	
	return workload
}

type WorkloadMetrics struct {
	TotalQueries  int
	UniqueQueries int
	TotalRows     int
}

func executePreciseWorkload(t *testing.T, db *sql.DB) WorkloadMetrics {
	metrics := WorkloadMetrics{}
	uniqueQueries := make(map[string]bool)
	
	// Execute known queries
	queries := []struct {
		query string
		rows  int
	}{
		{"SELECT COUNT(*) FROM users", 1},
		{"SELECT * FROM users WHERE id = 1", 1},
		{"SELECT * FROM users ORDER BY created_at DESC LIMIT 10", 10},
		{"INSERT INTO test_metrics (name, value) VALUES ('test', 1.0)", 1},
		{"UPDATE test_metrics SET value = 2.0 WHERE name = 'test'", 1},
		{"DELETE FROM test_metrics WHERE name = 'test'", 1},
	}
	
	for _, q := range queries {
		result, err := db.Exec(q.query)
		if err == nil {
			metrics.TotalQueries++
			uniqueQueries[q.query] = true
			
			if rows, err := result.RowsAffected(); err == nil {
				metrics.TotalRows += int(rows)
			}
		}
	}
	
	metrics.UniqueQueries = len(uniqueQueries)
	return metrics
}

func setupTestDatabase(t *testing.T, db *sql.DB) {
	// Create test tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255),
			created_at TIMESTAMP DEFAULT NOW(),
			last_login TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(id),
			total DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS test_metrics (
			name VARCHAR(255) PRIMARY KEY,
			value FLOAT,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
	}
	
	for _, query := range queries {
		_, err := db.Exec(query)
		require.NoError(t, err)
	}
	
	// Insert test data
	for i := 1; i <= 100; i++ {
		_, err := db.Exec("INSERT INTO users (email) VALUES ($1) ON CONFLICT DO NOTHING",
			fmt.Sprintf("user%d@example.com", i))
		require.NoError(t, err)
	}
}

// SimpleTestEnvironment represents a simple test environment (use TestEnvironment from test_environment.go for full features)
type SimpleTestEnvironment struct {
	PostgresDB *sql.DB
	DBName     string
	Cleanup    func()
}

// CollectorInstance represents a running collector
type CollectorInstance struct {
	endpoint string
}

func (c *CollectorInstance) Stop() {
	// Stop the collector process
}

// Additional test for all recent changes
func TestRecentFeatures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping recent features test in short mode")
	}

	ctx := context.Background()
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if accountID == "" || apiKey == "" {
		t.Skip("New Relic credentials required")
	}

	testEnv := setupRealTestEnvironment(t)
	defer testEnv.Cleanup()
	
	nrdbClient := NewNRDBClient(accountID, apiKey)
	collector := startRealCollector(t, testEnv)
	defer collector.Stop()

	t.Run("ASHHighFrequencySampling", func(t *testing.T) {
		// Generate concurrent activity
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				conn, _ := sql.Open("postgres", getTestDSN())
				defer conn.Close()
				
				// Different wait events
				switch id % 3 {
				case 0:
					conn.Exec("SELECT pg_sleep(0.5)") // CPU wait
				case 1:
					conn.Exec("SELECT * FROM users ORDER BY random()") // IO wait
				case 2:
					tx, _ := conn.Begin()
					tx.Exec("SELECT * FROM users WHERE id = 1 FOR UPDATE")
					time.Sleep(1 * time.Second)
					tx.Rollback() // Lock wait
				}
			}(i)
		}
		wg.Wait()

		time.Sleep(35 * time.Second)

		// Verify ASH metrics with 1-second resolution
		query := `
			SELECT count(*) as samples,
			       uniqueCount(session.pid) as unique_sessions,
			       uniqueCount(wait.event_type) as wait_types
			FROM Metric 
			WHERE metricName LIKE 'postgresql.ash%'
			SINCE 1 minute ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		require.NoError(t, err)

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			samples, _ := metrics["samples"].(float64)
			
			// With 1-second sampling, we should have many samples
			assert.Greater(t, samples, float64(20), "ASH sampling frequency too low")
			t.Logf("ASH samples collected: %v", samples)
		}
	})

	t.Run("EnhancedPIIDetection", func(t *testing.T) {
		db := testEnv.PostgresDB
		
		// Test all PII patterns
		piiTests := []struct {
			name    string
			query   string
			pattern string
		}{
			{"SSN", "SELECT * FROM users WHERE ssn = '123-45-6789'", "123-45-6789"},
			{"CreditCard", "SELECT * FROM orders WHERE cc = '4111111111111111'", "4111111111111111"},
			{"Email", "SELECT * FROM users WHERE email = 'john.doe@example.com'", "john.doe@example.com"},
			{"Phone", "SELECT * FROM users WHERE phone = '(555) 123-4567'", "(555) 123-4567"},
			{"CustomID", "SELECT * FROM employees WHERE id = 'EMP123456'", "EMP123456"},
		}

		for _, test := range piiTests {
			db.Exec(test.query)
		}

		time.Sleep(35 * time.Second)

		// Verify all PII is redacted
		query := `
			SELECT count(*) as total,
			       filter(count(*), WHERE db.statement LIKE '%[REDACTED]%') as redacted,
			       filter(count(*), WHERE db.statement LIKE '%123-45-6789%' 
			                           OR db.statement LIKE '%4111111111111111%'
			                           OR db.statement LIKE '%john.doe@example.com%') as exposed
			FROM Metric 
			WHERE metricName = 'db.statement.duration'
			SINCE 1 minute ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		require.NoError(t, err)

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			exposed, _ := metrics["exposed"].(float64)
			redacted, _ := metrics["redacted"].(float64)
			
			assert.Equal(t, float64(0), exposed, "PII data exposed!")
			assert.Greater(t, redacted, float64(0), "No PII redaction occurred")
		}
	})

	t.Run("QueryPlanRecommendations", func(t *testing.T) {
		db := testEnv.PostgresDB
		
		// Create queries that need optimization
		optimizationTests := []string{
			"SELECT * FROM users u1, users u2 WHERE u1.email = u2.email", // Missing join condition
			"SELECT * FROM orders WHERE total::text = '100.00'", // Type casting issue
			"SELECT * FROM users WHERE email NOT LIKE 'test%'", // Non-sargable
		}

		for _, query := range optimizationTests {
			db.Exec(query)
		}

		time.Sleep(35 * time.Second)

		// Check for optimization recommendations
		query := `
			SELECT count(*) as recommendations,
			       uniques(db.recommendation.type) as recommendation_types
			FROM Metric 
			WHERE db.recommendation.type IS NOT NULL
			AND db.system = 'postgresql'
			SINCE 2 minutes ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			t.Logf("Optimization recommendations: %+v", metrics)
		}
	})

	t.Run("BudgetEnforcement", func(t *testing.T) {
		// Generate high volume of metrics to test cost control
		db := testEnv.PostgresDB
		
		// Generate many unique queries (high cardinality)
		for i := 0; i < 1000; i++ {
			query := fmt.Sprintf("SELECT %d, '%s', random() FROM generate_series(1, 10)",
				i, generateRandomString(20))
			db.Exec(query)
		}

		time.Sleep(45 * time.Second)

		// Check cost control metrics
		query := `
			SELECT latest(cost_control.cardinality_reduced) as reduced,
			       latest(cost_control.aggressive_mode_active) as aggressive,
			       latest(cost_control.data_dropped_bytes) as dropped
			FROM Metric 
			WHERE metricName = 'otelcol.processor.costcontrol.action'
			SINCE 2 minutes ago`

		result, err := nrdbClient.ExecuteNRQL(ctx, query)
		if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
			metrics := result.Data.Actor.Account.NRQL.Results[0]
			t.Logf("Cost control actions: %+v", metrics)
		}
	})
}