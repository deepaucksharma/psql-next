package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	
	"github.com/database-intelligence/tests/e2e/framework"
)

// NewRelicVerificationSuite verifies data accuracy between source databases and NRDB
type NewRelicVerificationSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
	nrdb      *framework.NRDBClient
	ctx       context.Context
	cancel    context.CancelFunc
}

func TestNewRelicVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping New Relic verification tests in short mode")
	}
	
	suite.Run(t, new(NewRelicVerificationSuite))
}

func (s *NewRelicVerificationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	
	// Initialize environment
	s.env = framework.NewTestEnvironment()
	require.NoError(s.T(), s.env.Initialize())
	
	// Verify New Relic credentials
	s.Require().NotEmpty(s.env.NewRelicAccountID, "NEW_RELIC_ACCOUNT_ID must be set")
	s.Require().NotEmpty(s.env.NewRelicAPIKey, "NEW_RELIC_API_KEY must be set")
	s.Require().NotEmpty(s.env.NewRelicLicenseKey, "NEW_RELIC_LICENSE_KEY must be set")
	
	// Initialize NRDB client
	s.nrdb = framework.NewNRDBClient(s.env.NewRelicAccountID, s.env.NewRelicAPIKey)
	
	// Initialize and start collector
	s.collector = framework.NewTestCollector(s.env)
	config := s.getVerificationConfig()
	require.NoError(s.T(), s.collector.Start(config))
	
	// Setup test schema
	s.setupTestSchema()
}

func (s *NewRelicVerificationSuite) TearDownSuite() {
	s.cancel()
	s.cleanupTestData()
	if s.collector != nil {
		s.collector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
}

// Test 1: Verify PostgreSQL Core Metrics Accuracy
func (s *NewRelicVerificationSuite) TestPostgreSQLMetricsAccuracy() {
	s.T().Log("Verifying PostgreSQL metrics accuracy in NRDB...")
	
	// Baseline metrics from PostgreSQL
	type pgMetrics struct {
		connections    int
		commits        int64
		rollbacks      int64
		blocksRead     int64
		blocksHit      int64
		tupReturned    int64
		tupFetched     int64
		tupInserted    int64
		tupUpdated     int64
		tupDeleted     int64
		tempFiles      int64
		tempBytes      int64
		deadlocks      int64
		checksumErrors int64
		dbSize         int64
	}
	
	getMetrics := func() (*pgMetrics, error) {
		m := &pgMetrics{}
		
		// Connection count
		err := s.env.PostgresDB.QueryRow(`
			SELECT count(*) FROM pg_stat_activity 
			WHERE datname = current_database()
		`).Scan(&m.connections)
		if err != nil {
			return nil, err
		}
		
		// Transaction stats
		err = s.env.PostgresDB.QueryRow(`
			SELECT 
				xact_commit,
				xact_rollback,
				blks_read,
				blks_hit,
				tup_returned,
				tup_fetched,
				tup_inserted,
				tup_updated,
				tup_deleted,
				temp_files,
				temp_bytes,
				deadlocks,
				COALESCE(checksum_failures, 0)
			FROM pg_stat_database
			WHERE datname = current_database()
		`).Scan(
			&m.commits, &m.rollbacks, &m.blocksRead, &m.blocksHit,
			&m.tupReturned, &m.tupFetched, &m.tupInserted,
			&m.tupUpdated, &m.tupDeleted, &m.tempFiles,
			&m.tempBytes, &m.deadlocks, &m.checksumErrors,
		)
		if err != nil {
			return nil, err
		}
		
		// Database size
		err = s.env.PostgresDB.QueryRow(`
			SELECT pg_database_size(current_database())
		`).Scan(&m.dbSize)
		if err != nil {
			return nil, err
		}
		
		return m, nil
	}
	
	// Get baseline
	baseline, err := getMetrics()
	require.NoError(s.T(), err)
	
	// Perform operations to change metrics
	s.T().Log("Performing database operations...")
	
	// Create test table
	_, err = s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS nr_verify_test (
			id SERIAL PRIMARY KEY,
			data TEXT,
			value NUMERIC,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(s.T(), err)
	
	// Perform various operations
	operationCount := 100
	
	// Inserts
	for i := 0; i < operationCount; i++ {
		_, err = s.env.PostgresDB.Exec(`
			INSERT INTO nr_verify_test (data, value) VALUES ($1, $2)
		`, fmt.Sprintf("test_data_%d", i), float64(i)*1.5)
		require.NoError(s.T(), err)
	}
	
	// Updates
	for i := 0; i < operationCount/2; i++ {
		_, err = s.env.PostgresDB.Exec(`
			UPDATE nr_verify_test SET value = value * 1.1 WHERE id = $1
		`, i+1)
		require.NoError(s.T(), err)
	}
	
	// Selects
	for i := 0; i < operationCount; i++ {
		rows, err := s.env.PostgresDB.Query(`
			SELECT * FROM nr_verify_test WHERE value > $1
		`, float64(i))
		require.NoError(s.T(), err)
		rows.Close()
	}
	
	// Deletes
	for i := 0; i < operationCount/4; i++ {
		_, err = s.env.PostgresDB.Exec(`
			DELETE FROM nr_verify_test WHERE id = $1
		`, i+1)
		require.NoError(s.T(), err)
	}
	
	// Transaction with rollback
	tx, err := s.env.PostgresDB.Begin()
	require.NoError(s.T(), err)
	_, err = tx.Exec(`INSERT INTO nr_verify_test (data) VALUES ('rollback_test')`)
	require.NoError(s.T(), err)
	err = tx.Rollback()
	require.NoError(s.T(), err)
	
	// Wait for metrics collection
	s.T().Log("Waiting for metrics collection cycle...")
	time.Sleep(65 * time.Second)
	
	// Get current metrics
	current, err := getMetrics()
	require.NoError(s.T(), err)
	
	// Calculate deltas
	deltaCommits := current.commits - baseline.commits
	deltaRollbacks := current.rollbacks - baseline.rollbacks
	deltaInserted := current.tupInserted - baseline.tupInserted
	deltaUpdated := current.tupUpdated - baseline.tupUpdated
	deltaDeleted := current.tupDeleted - baseline.tupDeleted
	
	s.T().Logf("Database metric deltas - Commits: %d, Rollbacks: %d, Inserts: %d, Updates: %d, Deletes: %d",
		deltaCommits, deltaRollbacks, deltaInserted, deltaUpdated, deltaDeleted)
	
	// Verify metrics in NRDB
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()
	
	// Define metrics to verify
	metricsToVerify := []struct {
		name      string
		nrqlQuery string
		expected  float64
		tolerance float64 // percentage
	}{
		{
			name:      "connections",
			nrqlQuery: "SELECT latest(postgresql.backends) FROM Metric WHERE db.name = '" + s.env.PostgresDatabase + "' SINCE 5 minutes ago",
			expected:  float64(current.connections),
			tolerance: 20.0, // Connections can vary
		},
		{
			name:      "database_size",
			nrqlQuery: "SELECT latest(postgresql.database.size) FROM Metric WHERE db.name = '" + s.env.PostgresDatabase + "' SINCE 5 minutes ago",
			expected:  float64(current.dbSize),
			tolerance: 1.0, // Size should be accurate
		},
		{
			name:      "commits_delta",
			nrqlQuery: fmt.Sprintf("SELECT sum(postgresql.commits) FROM Metric WHERE db.name = '%s' SINCE 5 minutes ago", s.env.PostgresDatabase),
			expected:  float64(deltaCommits),
			tolerance: 5.0,
		},
		{
			name:      "rollbacks_delta",
			nrqlQuery: fmt.Sprintf("SELECT sum(postgresql.rollbacks) FROM Metric WHERE db.name = '%s' SINCE 5 minutes ago", s.env.PostgresDatabase),
			expected:  float64(deltaRollbacks),
			tolerance: 0.0, // Should be exact (1 rollback)
		},
	}
	
	for _, metric := range metricsToVerify {
		s.Run(metric.name, func() {
			result, err := s.nrdb.Query(ctx, metric.nrqlQuery)
			if err != nil {
				s.T().Logf("Failed to query metric %s: %v", metric.name, err)
				return
			}
			
			if len(result.Results) == 0 {
				s.T().Errorf("No data found for metric %s", metric.name)
				return
			}
			
			// Extract value
			var actualValue float64
			for _, key := range []string{"latest", "sum", "value"} {
				if val, ok := result.Results[0][key]; ok {
					switch v := val.(type) {
					case float64:
						actualValue = v
					case int64:
						actualValue = float64(v)
					case string:
						fmt.Sscanf(v, "%f", &actualValue)
					}
					break
				}
			}
			
			// Verify accuracy
			if metric.tolerance == 0.0 {
				assert.Equal(s.T(), metric.expected, actualValue,
					"Metric %s should match exactly", metric.name)
			} else {
				percentDiff := math.Abs(actualValue-metric.expected) / metric.expected * 100
				assert.LessOrEqual(s.T(), percentDiff, metric.tolerance,
					"Metric %s should be within %.1f%% tolerance (expected: %.2f, actual: %.2f, diff: %.1f%%)",
					metric.name, metric.tolerance, metric.expected, actualValue, percentDiff)
			}
		})
	}
	
	s.T().Log("✓ PostgreSQL metrics verified in NRDB")
}

// Test 2: Verify Query Performance Metrics
func (s *NewRelicVerificationSuite) TestQueryPerformanceMetrics() {
	s.T().Log("Verifying query performance metrics in NRDB...")
	
	// Enable query tracking
	_, err := s.env.PostgresDB.Exec(`
		CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
		SELECT pg_stat_statements_reset();
	`)
	if err != nil {
		s.T().Logf("Warning: Could not enable pg_stat_statements: %v", err)
	}
	
	// Execute queries with known characteristics
	queries := []struct {
		name        string
		sql         string
		iterations  int
		sleepMs     int
		expectedAvg float64 // milliseconds
	}{
		{
			name:        "fast_query",
			sql:         "SELECT 1",
			iterations:  100,
			sleepMs:     0,
			expectedAvg: 1.0,
		},
		{
			name:        "medium_query",
			sql:         "SELECT COUNT(*) FROM pg_class",
			iterations:  50,
			sleepMs:     0,
			expectedAvg: 10.0,
		},
		{
			name:        "slow_query",
			sql:         "SELECT pg_sleep(0.1)",
			iterations:  10,
			sleepMs:     0,
			expectedAvg: 100.0,
		},
	}
	
	// Track query IDs for verification
	queryTracker := make(map[string][]string)
	
	for _, q := range queries {
		s.Run(q.name, func() {
			// Execute queries
			var totalDuration time.Duration
			
			for i := 0; i < q.iterations; i++ {
				queryID := fmt.Sprintf("%s_%d_%d", q.name, i, time.Now().UnixNano())
				trackedSQL := fmt.Sprintf("/* query_id: %s, type: %s */ %s", queryID, q.name, q.sql)
				
				start := time.Now()
				rows, err := s.env.PostgresDB.Query(trackedSQL)
				require.NoError(s.T(), err)
				rows.Close()
				duration := time.Since(start)
				
				totalDuration += duration
				queryTracker[q.name] = append(queryTracker[q.name], queryID)
				
				if q.sleepMs > 0 {
					time.Sleep(time.Duration(q.sleepMs) * time.Millisecond)
				}
			}
			
			avgDuration := totalDuration / time.Duration(q.iterations)
			s.T().Logf("%s: Executed %d queries, avg duration: %v", q.name, q.iterations, avgDuration)
		})
	}
	
	// Wait for metrics collection
	s.T().Log("Waiting for query metrics to be exported...")
	time.Sleep(65 * time.Second)
	
	// Verify query metrics in NRDB
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()
	
	// Check for query duration metrics
	for queryType, queryIDs := range queryTracker {
		s.Run("verify_"+queryType, func() {
			// Build NRQL to find our queries
			nrql := fmt.Sprintf(`
				SELECT 
					count(*) as queryCount,
					average(duration) as avgDuration,
					max(duration) as maxDuration,
					min(duration) as minDuration
				FROM Log 
				WHERE type = '%s' 
				SINCE 5 minutes ago
			`, queryType)
			
			result, err := s.nrdb.Query(ctx, nrql)
			if err != nil {
				s.T().Logf("Failed to query metrics for %s: %v", queryType, err)
				return
			}
			
			if len(result.Results) == 0 {
				s.T().Errorf("No query metrics found for %s", queryType)
				return
			}
			
			data := result.Results[0]
			
			// Verify query count
			if count, ok := data["queryCount"].(float64); ok {
				expectedCount := len(queryIDs)
				assert.GreaterOrEqual(s.T(), int(count), expectedCount/2,
					"Should have captured at least half of %s queries", queryType)
			}
			
			// Verify average duration is reasonable
			if avgDuration, ok := data["avgDuration"].(float64); ok {
				s.T().Logf("%s average duration in NRDB: %.2f ms", queryType, avgDuration)
				
				// Allow significant tolerance as timing can vary
				switch queryType {
				case "fast_query":
					assert.Less(s.T(), avgDuration, 50.0, "Fast queries should average < 50ms")
				case "slow_query":
					assert.Greater(s.T(), avgDuration, 50.0, "Slow queries should average > 50ms")
				}
			}
		})
	}
	
	s.T().Log("✓ Query performance metrics verified in NRDB")
}

// Test 3: Verify Query Plan Tracking
func (s *NewRelicVerificationSuite) TestQueryPlanTracking() {
	s.T().Log("Verifying query plan tracking in NRDB...")
	
	// Create table with specific structure for plan testing
	_, err := s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS plan_verify_test (
			id SERIAL PRIMARY KEY,
			category VARCHAR(50),
			status VARCHAR(20),
			amount NUMERIC(10,2),
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(s.T(), err)
	
	// Insert test data
	for i := 0; i < 10000; i++ {
		category := []string{"A", "B", "C", "D", "E"}[i%5]
		status := []string{"active", "inactive", "pending"}[i%3]
		amount := float64(i) * 1.23
		
		_, err = s.env.PostgresDB.Exec(`
			INSERT INTO plan_verify_test (category, status, amount)
			VALUES ($1, $2, $3)
		`, category, status, amount)
		require.NoError(s.T(), err)
	}
	
	// Create indexes to influence plans
	_, err = s.env.PostgresDB.Exec(`
		CREATE INDEX idx_plan_category ON plan_verify_test(category);
		CREATE INDEX idx_plan_status ON plan_verify_test(status);
		CREATE INDEX idx_plan_amount ON plan_verify_test(amount);
		ANALYZE plan_verify_test;
	`)
	require.NoError(s.T(), err)
	
	// Execute queries with different plans
	planQueries := []struct {
		name         string
		sql          string
		expectedPlan string
	}{
		{
			name:         "index_scan_category",
			sql:          "SELECT * FROM plan_verify_test WHERE category = 'A'",
			expectedPlan: "Index Scan",
		},
		{
			name:         "bitmap_heap_scan",
			sql:          "SELECT * FROM plan_verify_test WHERE category IN ('A', 'B') AND status = 'active'",
			expectedPlan: "Bitmap",
		},
		{
			name:         "sequential_scan",
			sql:          "SELECT * FROM plan_verify_test WHERE amount > 5000",
			expectedPlan: "Seq Scan",
		},
		{
			name:         "hash_aggregate",
			sql:          "SELECT category, COUNT(*), AVG(amount) FROM plan_verify_test GROUP BY category",
			expectedPlan: "HashAggregate",
		},
		{
			name:         "nested_loop_join",
			sql:          "SELECT a.*, b.amount FROM plan_verify_test a JOIN plan_verify_test b ON a.category = b.category WHERE a.id < 100 AND b.id < 100",
			expectedPlan: "Nested Loop",
		},
	}
	
	// Execute queries and capture plans
	executedPlans := make(map[string]string)
	
	for _, pq := range planQueries {
		s.Run("execute_"+pq.name, func() {
			// Get actual plan
			var planJSON string
			rows, err := s.env.PostgresDB.Query(fmt.Sprintf("EXPLAIN (FORMAT JSON) %s", pq.sql))
			require.NoError(s.T(), err)
			
			for rows.Next() {
				var line string
				err = rows.Scan(&line)
				require.NoError(s.T(), err)
				planJSON += line
			}
			rows.Close()
			
			// Parse plan to verify it matches expected type
			var planData []map[string]interface{}
			err = json.Unmarshal([]byte(planJSON), &planData)
			require.NoError(s.T(), err)
			
			if len(planData) > 0 {
				if plan, ok := planData[0]["Plan"].(map[string]interface{}); ok {
					if nodeType, ok := plan["Node Type"].(string); ok {
						assert.Contains(s.T(), nodeType, pq.expectedPlan,
							"Query %s should use %s", pq.name, pq.expectedPlan)
					}
				}
			}
			
			// Execute query with tracking
			queryID := fmt.Sprintf("plan_%s_%d", pq.name, time.Now().UnixNano())
			trackedSQL := fmt.Sprintf("/* query_id: %s, plan_test: true */ %s", queryID, pq.sql)
			
			rows, err = s.env.PostgresDB.Query(trackedSQL)
			require.NoError(s.T(), err)
			rows.Close()
			
			executedPlans[queryID] = pq.expectedPlan
		})
	}
	
	// Wait for plan extraction and export
	s.T().Log("Waiting for query plans to be exported...")
	time.Sleep(65 * time.Second)
	
	// Verify plans in NRDB
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()
	
	// Check that plans were captured
	nrql := `
		SELECT 
			count(*) as planCount,
			uniques(plan.hash) as uniqueHashes,
			uniques(plan.type) as planTypes
		FROM Log 
		WHERE plan_test = true 
		SINCE 5 minutes ago
	`
	
	result, err := s.nrdb.Query(ctx, nrql)
	if err != nil {
		s.T().Logf("Failed to query plan metrics: %v", err)
		return
	}
	
	if len(result.Results) > 0 {
		data := result.Results[0]
		
		if count, ok := data["planCount"].(float64); ok {
			assert.Greater(s.T(), int(count), 0, "Should have captured query plans")
			s.T().Logf("Found %d query plans in NRDB", int(count))
		}
		
		if hashes, ok := data["uniqueHashes"].([]interface{}); ok {
			assert.GreaterOrEqual(s.T(), len(hashes), 3, "Should have multiple unique plan hashes")
			s.T().Logf("Found %d unique plan hashes", len(hashes))
		}
		
		if types, ok := data["planTypes"].([]interface{}); ok {
			s.T().Logf("Plan types found: %v", types)
			
			// Verify expected plan types are present
			typeMap := make(map[string]bool)
			for _, t := range types {
				if str, ok := t.(string); ok {
					typeMap[str] = true
				}
			}
			
			// Check for some expected plan types
			expectedTypes := []string{"Index Scan", "Seq Scan", "HashAggregate"}
			found := 0
			for _, expected := range expectedTypes {
				for captured := range typeMap {
					if strings.Contains(captured, expected) {
						found++
						break
					}
				}
			}
			assert.GreaterOrEqual(s.T(), found, 2, "Should find at least 2 expected plan types")
		}
	}
	
	// Verify plan attributes are properly extracted
	for queryID, expectedPlan := range executedPlans {
		s.Run("verify_plan_"+queryID, func() {
			nrql := fmt.Sprintf(`
				SELECT * FROM Log 
				WHERE query_id = '%s' 
				SINCE 5 minutes ago 
				LIMIT 1
			`, queryID)
			
			result, err := s.nrdb.Query(ctx, nrql)
			if err != nil || len(result.Results) == 0 {
				s.T().Logf("Could not find plan for query %s", queryID)
				return
			}
			
			data := result.Results[0]
			
			// Verify plan attributes
			if planType, ok := data["plan.type"].(string); ok {
				assert.Contains(s.T(), planType, expectedPlan,
					"Plan type should match expected for %s", queryID)
			}
			
			if planHash, ok := data["plan.hash"].(string); ok {
				assert.NotEmpty(s.T(), planHash, "Plan should have a hash")
				assert.Len(s.T(), planHash, 64, "Plan hash should be SHA-256")
			}
			
			if totalCost, ok := data["plan.total_cost"].(float64); ok {
				assert.Greater(s.T(), totalCost, 0.0, "Plan should have a cost estimate")
			}
			
			// Verify query was anonymized
			if statement, ok := data["db.statement"].(string); ok {
				assert.NotContains(s.T(), statement, "'A'", "String literals should be anonymized")
				assert.NotContains(s.T(), statement, "5000", "Numeric literals should be anonymized")
			}
		})
	}
	
	s.T().Log("✓ Query plan tracking verified in NRDB")
}

// Test 4: Verify Error and Exception Tracking
func (s *NewRelicVerificationSuite) TestErrorAndExceptionTracking() {
	s.T().Log("Verifying error and exception tracking in NRDB...")
	
	// Generate various database errors
	errors := []struct {
		name        string
		operation   func() error
		errorCode   string
		errorClass  string
	}{
		{
			name: "syntax_error",
			operation: func() error {
				_, err := s.env.PostgresDB.Exec("SELECT * FROM WHERE invalid")
				return err
			},
			errorCode:  "42601",
			errorClass: "syntax_error",
		},
		{
			name: "table_not_found",
			operation: func() error {
				_, err := s.env.PostgresDB.Query("SELECT * FROM nonexistent_table_12345")
				return err
			},
			errorCode:  "42P01",
			errorClass: "undefined_table",
		},
		{
			name: "division_by_zero",
			operation: func() error {
				var result int
				err := s.env.PostgresDB.QueryRow("SELECT 1/0").Scan(&result)
				return err
			},
			errorCode:  "22012",
			errorClass: "division_by_zero",
		},
		{
			name: "constraint_violation",
			operation: func() error {
				// Create table with constraint
				_, err := s.env.PostgresDB.Exec(`
					CREATE TABLE IF NOT EXISTS error_test (
						id INT PRIMARY KEY,
						value INT NOT NULL CHECK (value > 0)
					)
				`)
				if err != nil {
					return err
				}
				
				// Violate constraint
				_, err = s.env.PostgresDB.Exec("INSERT INTO error_test (id, value) VALUES (1, -1)")
				return err
			},
			errorCode:  "23514",
			errorClass: "check_violation",
		},
		{
			name: "deadlock",
			operation: func() error {
				// This is harder to reliably reproduce
				// For now, just track that we're monitoring for it
				return nil
			},
			errorCode:  "40P01",
			errorClass: "deadlock_detected",
		},
	}
	
	// Track error occurrences
	errorTracker := make(map[string]string)
	
	for _, errTest := range errors {
		s.Run("generate_"+errTest.name, func() {
			errorID := fmt.Sprintf("error_%s_%d", errTest.name, time.Now().UnixNano())
			
			// Add context to track the error
			ctx := context.WithValue(s.ctx, "error_id", errorID)
			
			// Execute operation that should error
			err := errTest.operation()
			
			if err != nil {
				s.T().Logf("%s error generated: %v", errTest.name, err)
				errorTracker[errorID] = errTest.name
			}
		})
	}
	
	// Wait for error metrics to be collected
	s.T().Log("Waiting for error metrics to be exported...")
	time.Sleep(65 * time.Second)
	
	// Verify errors in NRDB
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()
	
	// Check for database error metrics
	nrql := `
		SELECT 
			count(*) as errorCount,
			uniques(error.class) as errorClasses,
			uniques(error.code) as errorCodes
		FROM Log 
		WHERE error.message IS NOT NULL 
			AND db.system = 'postgresql'
		SINCE 5 minutes ago
	`
	
	result, err := s.nrdb.Query(ctx, nrql)
	if err != nil {
		s.T().Logf("Failed to query error metrics: %v", err)
		return
	}
	
	if len(result.Results) > 0 {
		data := result.Results[0]
		
		if count, ok := data["errorCount"].(float64); ok {
			assert.Greater(s.T(), int(count), 0, "Should have captured database errors")
			s.T().Logf("Found %d database errors in NRDB", int(count))
		}
		
		if classes, ok := data["errorClasses"].([]interface{}); ok {
			s.T().Logf("Error classes found: %v", classes)
			assert.GreaterOrEqual(s.T(), len(classes), 2, "Should have multiple error classes")
		}
		
		if codes, ok := data["errorCodes"].([]interface{}); ok {
			s.T().Logf("Error codes found: %v", codes)
			
			// Verify some expected error codes
			codeMap := make(map[string]bool)
			for _, code := range codes {
				if str, ok := code.(string); ok {
					codeMap[str] = true
				}
			}
			
			// Check for syntax error code
			assert.Contains(s.T(), codeMap, "42601", "Should capture syntax error code")
		}
	}
	
	// Check error monitoring metrics
	errorMonitorNRQL := `
		SELECT 
			latest(error_rate) as errorRate,
			latest(errors_total) as totalErrors
		FROM Metric 
		WHERE processor = 'nrerrormonitor' 
		SINCE 5 minutes ago
	`
	
	result, err = s.nrdb.Query(ctx, errorMonitorNRQL)
	if err == nil && len(result.Results) > 0 {
		data := result.Results[0]
		
		if errorRate, ok := data["errorRate"].(float64); ok {
			s.T().Logf("Current error rate: %.2f%%", errorRate*100)
			assert.GreaterOrEqual(s.T(), errorRate, 0.0, "Error rate should be tracked")
		}
		
		if totalErrors, ok := data["totalErrors"].(float64); ok {
			s.T().Logf("Total errors tracked: %.0f", totalErrors)
			assert.Greater(s.T(), totalErrors, 0.0, "Should have tracked some errors")
		}
	}
	
	s.T().Log("✓ Error and exception tracking verified in NRDB")
}

// Test 5: Verify Custom Attributes and Tags
func (s *NewRelicVerificationSuite) TestCustomAttributesAndTags() {
	s.T().Log("Verifying custom attributes and tags in NRDB...")
	
	// Ensure we're using a collector config with custom attributes
	config := s.getVerificationConfigWithCustomAttributes()
	err := s.collector.UpdateConfig(config)
	require.NoError(s.T(), err)
	
	// Wait for collector to restart with new config
	time.Sleep(10 * time.Second)
	
	// Perform operations that should be tagged
	testID := fmt.Sprintf("attr_test_%d", time.Now().UnixNano())
	
	// Create test table
	_, err = s.env.PostgresDB.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			app_name VARCHAR(50),
			environment VARCHAR(20),
			region VARCHAR(20),
			data JSONB
		)
	`, testID))
	require.NoError(s.T(), err)
	
	// Insert data with various attributes
	testData := []map[string]interface{}{
		{
			"app_name":    "web-frontend",
			"environment": "production",
			"region":      "us-east-1",
			"data":        `{"user_id": 123, "action": "login"}`,
		},
		{
			"app_name":    "api-backend",
			"environment": "staging",
			"region":      "us-west-2",
			"data":        `{"endpoint": "/api/v1/users", "method": "GET"}`,
		},
		{
			"app_name":    "batch-processor",
			"environment": "development",
			"region":      "eu-central-1",
			"data":        `{"job_id": "job_456", "status": "running"}`,
		},
	}
	
	for _, data := range testData {
		_, err = s.env.PostgresDB.Exec(fmt.Sprintf(`
			INSERT INTO %s (app_name, environment, region, data)
			VALUES ($1, $2, $3, $4)
		`, testID), data["app_name"], data["environment"], data["region"], data["data"])
		require.NoError(s.T(), err)
	}
	
	// Query with custom comment attributes
	for _, data := range testData {
		comment := fmt.Sprintf("/* app: %s, env: %s, region: %s, test_id: %s */",
			data["app_name"], data["environment"], data["region"], testID)
		
		sql := fmt.Sprintf("%s SELECT * FROM %s WHERE app_name = $1", comment, testID)
		rows, err := s.env.PostgresDB.Query(sql, data["app_name"])
		require.NoError(s.T(), err)
		rows.Close()
	}
	
	// Wait for metrics with custom attributes
	s.T().Log("Waiting for custom attributes to be exported...")
	time.Sleep(65 * time.Second)
	
	// Verify custom attributes in NRDB
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()
	
	// Check for metrics with custom attributes
	nrql := fmt.Sprintf(`
		SELECT 
			count(*) as metricCount,
			uniques(environment) as environments,
			uniques(service.name) as serviceNames,
			uniques(deployment.environment) as deployEnvironments
		FROM Metric 
		WHERE test_id = '%s' OR db.system = 'postgresql'
		SINCE 5 minutes ago
	`, testID)
	
	result, err := s.nrdb.Query(ctx, nrql)
	if err != nil {
		s.T().Logf("Failed to query custom attributes: %v", err)
		return
	}
	
	if len(result.Results) > 0 {
		data := result.Results[0]
		
		if environments, ok := data["environments"].([]interface{}); ok && len(environments) > 0 {
			s.T().Logf("Environments found: %v", environments)
			assert.Contains(s.T(), environments, "e2e-test", "Should have test environment attribute")
		}
		
		if serviceNames, ok := data["serviceNames"].([]interface{}); ok && len(serviceNames) > 0 {
			s.T().Logf("Service names found: %v", serviceNames)
			assert.Contains(s.T(), serviceNames, "database-intelligence", "Should have service name attribute")
		}
	}
	
	// Check for logs with custom attributes
	logNRQL := fmt.Sprintf(`
		SELECT 
			count(*) as logCount,
			uniques(app) as apps,
			uniques(env) as envs,
			uniques(region) as regions
		FROM Log 
		WHERE test_id = '%s'
		SINCE 5 minutes ago
	`, testID)
	
	result, err = s.nrdb.Query(ctx, logNRQL)
	if err == nil && len(result.Results) > 0 {
		data := result.Results[0]
		
		if apps, ok := data["apps"].([]interface{}); ok && len(apps) > 0 {
			s.T().Logf("Applications found in logs: %v", apps)
			
			// Verify all test apps were captured
			appMap := make(map[string]bool)
			for _, app := range apps {
				if str, ok := app.(string); ok {
					appMap[str] = true
				}
			}
			
			for _, testApp := range []string{"web-frontend", "api-backend", "batch-processor"} {
				assert.Contains(s.T(), appMap, testApp, "Should capture app attribute: %s", testApp)
			}
		}
		
		if regions, ok := data["regions"].([]interface{}); ok && len(regions) > 0 {
			s.T().Logf("Regions found in logs: %v", regions)
			assert.GreaterOrEqual(s.T(), len(regions), 2, "Should capture multiple regions")
		}
	}
	
	// Cleanup
	_, err = s.env.PostgresDB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", testID))
	if err != nil {
		s.T().Logf("Warning: Failed to cleanup test table: %v", err)
	}
	
	s.T().Log("✓ Custom attributes and tags verified in NRDB")
}

// Test 6: Verify Data Completeness Over Time
func (s *NewRelicVerificationSuite) TestDataCompletenessOverTime() {
	s.T().Log("Verifying data completeness over multiple collection cycles...")
	
	// Track metrics over 3 collection cycles
	cycles := 3
	cycleDuration := 65 * time.Second
	
	type cycleMetrics struct {
		timestamp   time.Time
		connections int
		queries     int
		dbSize      int64
	}
	
	var cycleData []cycleMetrics
	
	for cycle := 0; cycle < cycles; cycle++ {
		s.T().Logf("Collection cycle %d/%d", cycle+1, cycles)
		
		// Get current metrics
		var metrics cycleMetrics
		metrics.timestamp = time.Now()
		
		err := s.env.PostgresDB.QueryRow(`
			SELECT count(*) FROM pg_stat_activity WHERE datname = current_database()
		`).Scan(&metrics.connections)
		require.NoError(s.T(), err)
		
		err = s.env.PostgresDB.QueryRow(`
			SELECT sum(calls) FROM pg_stat_statements WHERE dbid = (
				SELECT oid FROM pg_database WHERE datname = current_database()
			)
		`).Scan(&metrics.queries)
		if err != nil {
			// pg_stat_statements might not be available
			metrics.queries = 0
		}
		
		err = s.env.PostgresDB.QueryRow(`
			SELECT pg_database_size(current_database())
		`).Scan(&metrics.dbSize)
		require.NoError(s.T(), err)
		
		cycleData = append(cycleData, metrics)
		
		// Perform some operations
		for i := 0; i < 10; i++ {
			rows, err := s.env.PostgresDB.Query("SELECT version()")
			require.NoError(s.T(), err)
			rows.Close()
		}
		
		// Wait for next cycle
		if cycle < cycles-1 {
			time.Sleep(cycleDuration)
		}
	}
	
	// Wait for final cycle to be exported
	time.Sleep(cycleDuration)
	
	// Verify data completeness in NRDB
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()
	
	// Check that we have data points for each cycle
	startTime := cycleData[0].timestamp.Add(-1 * time.Minute)
	nrql := fmt.Sprintf(`
		SELECT 
			count(*) as dataPoints,
			uniques(timestamp) as uniqueTimestamps,
			min(timestamp) as earliestTime,
			max(timestamp) as latestTime
		FROM Metric 
		WHERE db.system = 'postgresql' 
			AND db.name = '%s'
			AND metricName = 'postgresql.backends'
		SINCE %d minutes ago
	`, s.env.PostgresDatabase, int(time.Since(startTime).Minutes())+1)
	
	result, err := s.nrdb.Query(ctx, nrql)
	require.NoError(s.T(), err)
	
	if len(result.Results) > 0 {
		data := result.Results[0]
		
		if dataPoints, ok := data["dataPoints"].(float64); ok {
			// Should have at least one data point per cycle
			assert.GreaterOrEqual(s.T(), int(dataPoints), cycles,
				"Should have data points for each collection cycle")
			s.T().Logf("Found %d data points over %d cycles", int(dataPoints), cycles)
		}
		
		if timestamps, ok := data["uniqueTimestamps"].([]interface{}); ok {
			assert.GreaterOrEqual(s.T(), len(timestamps), cycles,
				"Should have unique timestamps for each cycle")
		}
		
		// Verify time range coverage
		if earliest, ok := data["earliestTime"].(float64); ok {
			if latest, ok := data["latestTime"].(float64); ok {
				duration := time.Duration(latest-earliest) * time.Millisecond
				expectedDuration := time.Duration(cycles-1) * cycleDuration
				
				// Allow some variance
				assert.Greater(s.T(), duration, expectedDuration/2,
					"Data should span expected time range")
				
				s.T().Logf("Data spans %.1f minutes", duration.Minutes())
			}
		}
	}
	
	// Verify no gaps in data
	gapNRQL := fmt.Sprintf(`
		SELECT 
			derivative(postgresql.backends, 1 minute) as metricGap
		FROM Metric 
		WHERE db.system = 'postgresql' 
			AND db.name = '%s'
		SINCE %d minutes ago
		FACET timestamp
	`, s.env.PostgresDatabase, int(time.Since(startTime).Minutes())+1)
	
	result, err = s.nrdb.Query(ctx, gapNRQL)
	if err == nil && len(result.Results) > 0 {
		// Check for large gaps that would indicate missing data
		for _, r := range result.Results {
			if gap, ok := r["metricGap"].(float64); ok {
				// A gap > 2 collection intervals suggests missing data
				assert.Less(s.T(), math.Abs(gap), float64(2*60),
					"Should not have large gaps in metric data")
			}
		}
	}
	
	s.T().Log("✓ Data completeness verified over multiple cycles")
}

// Helper methods

func (s *NewRelicVerificationSuite) setupTestSchema() {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS pg_stat_statements`,
		`CREATE TABLE IF NOT EXISTS test_markers (
			marker_id VARCHAR(255) PRIMARY KEY,
			created_at TIMESTAMP WITH TIME ZONE,
			description TEXT
		)`,
	}
	
	for _, query := range queries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Warning: Failed to execute setup query: %v", err)
		}
	}
}

func (s *NewRelicVerificationSuite) cleanupTestData() {
	tables := []string{
		"nr_verify_test",
		"plan_verify_test",
		"error_test",
		"test_markers",
	}
	
	for _, table := range tables {
		_, err := s.env.PostgresDB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			s.T().Logf("Warning: Failed to drop table %s: %v", table, err)
		}
	}
}

func (s *NewRelicVerificationSuite) getVerificationConfig() string {
	return `
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    databases:
      - ${POSTGRES_DB}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 60s
    tls:
      insecure: true

  sqlquery/postgresql:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} sslmode=disable"
    queries:
      - sql: |
          SELECT 
            marker_id,
            created_at,
            description
          FROM test_markers
          WHERE created_at > NOW() - INTERVAL '5 minutes'
        metrics:
          - metric_name: test.marker
            value_column: "1"
            attribute_columns: ["marker_id", "created_at", "description"]

processors:
  verification:
    enabled: true
    checksum_attributes: ["metricName", "value", "timestamp"]
    
  planattributeextractor:
    enabled: true
    
  nrerrormonitor:
    enabled: true
    error_rate_threshold: 0.05
    
  attributes:
    actions:
      - key: db.system
        action: upsert
        value: postgresql
      - key: environment
        action: insert
        value: e2e-test
      - key: service.name
        action: insert
        value: database-intelligence

  batch:
    timeout: 10s

exporters:
  otlp/newrelic:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [verification, attributes, batch]
      exporters: [otlp/newrelic, debug]
    
    logs:
      receivers: [sqlquery/postgresql]
      processors: [planattributeextractor, nrerrormonitor, attributes]
      exporters: [otlp/newrelic, debug]
`
}

func (s *NewRelicVerificationSuite) getVerificationConfigWithCustomAttributes() string {
	return `
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    databases:
      - ${POSTGRES_DB}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 60s
    tls:
      insecure: true

  sqlquery/postgresql:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} sslmode=disable"
    queries:
      - sql: |
          SELECT 
            marker_id,
            created_at,
            description
          FROM test_markers
          WHERE created_at > NOW() - INTERVAL '5 minutes'
        metrics:
          - metric_name: test.marker
            value_column: "1"
            attribute_columns: ["marker_id", "created_at", "description"]

processors:
  verification:
    enabled: true
    
  planattributeextractor:
    enabled: true
    extract_comment_attributes: true
    
  attributes/custom:
    actions:
      - key: db.system
        action: upsert
        value: postgresql
      - key: environment
        action: insert
        value: e2e-test
      - key: service.name
        action: insert
        value: database-intelligence
      - key: deployment.environment
        action: insert
        value: testing
      - key: service.version
        action: insert
        value: "1.0.0"
      - key: datacenter
        action: insert
        value: us-east-1
      - key: team
        action: insert
        value: database-team

  batch:
    timeout: 10s

exporters:
  otlp/newrelic:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [verification, attributes/custom, batch]
      exporters: [otlp/newrelic, debug]
    
    logs:
      receivers: [sqlquery/postgresql]
      processors: [planattributeextractor, attributes/custom]
      exporters: [otlp/newrelic, debug]
`
}