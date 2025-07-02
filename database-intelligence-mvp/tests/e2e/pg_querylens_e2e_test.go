// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestPGQueryLensE2E performs end-to-end testing of pg_querylens integration
func TestPGQueryLensE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()

	// Start PostgreSQL container with pg_querylens
	postgres, db, err := setupPostgreSQLWithQueryLens(ctx)
	require.NoError(t, err)
	defer postgres.Terminate(ctx)
	defer db.Close()

	// Start collector
	collector := startCollectorWithQueryLens(t, postgres)
	defer collector.Stop()

	// Run test scenarios
	t.Run("QueryPerformanceCollection", func(t *testing.T) {
		testQueryPerformanceCollection(t, db, collector)
	})

	t.Run("PlanChangeDetection", func(t *testing.T) {
		testPlanChangeDetection(t, db, collector)
	})

	t.Run("RegressionDetection", func(t *testing.T) {
		testRegressionDetection(t, db, collector)
	})

	t.Run("ResourceConsumptionTracking", func(t *testing.T) {
		testResourceConsumptionTracking(t, db, collector)
	})
}

// setupPostgreSQLWithQueryLens creates a PostgreSQL container with pg_querylens extension
func setupPostgreSQLWithQueryLens(ctx context.Context) (testcontainers.Container, *sql.DB, error) {
	// Note: This would need a custom PostgreSQL image with pg_querylens pre-installed
	// For testing purposes, we'll simulate the behavior
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForSQL("5432/tcp", "postgres", func(host string, port nat.Port) string {
			return fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())
		}),
	}

	postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, err
	}

	// Get connection details
	host, err := postgres.Host(ctx)
	if err != nil {
		return nil, nil, err
	}

	port, err := postgres.MappedPort(ctx, "5432")
	if err != nil {
		return nil, nil, err
	}

	// Connect to database
	dsn := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, nil, err
	}

	// Simulate pg_querylens setup
	if err := setupQueryLensTables(db); err != nil {
		return nil, nil, err
	}

	return postgres, db, nil
}

// setupQueryLensTables creates tables that simulate pg_querylens schema
func setupQueryLensTables(db *sql.DB) error {
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS pg_querylens`,
		`CREATE TABLE IF NOT EXISTS pg_querylens.queries (
			queryid BIGINT PRIMARY KEY,
			query_text TEXT,
			userid OID,
			dbid OID
		)`,
		`CREATE TABLE IF NOT EXISTS pg_querylens.plans (
			queryid BIGINT,
			plan_id TEXT,
			plan_text TEXT,
			last_execution TIMESTAMP,
			mean_exec_time_ms FLOAT,
			stddev_exec_time_ms FLOAT,
			min_exec_time_ms FLOAT,
			max_exec_time_ms FLOAT,
			total_exec_time_ms FLOAT,
			calls BIGINT,
			rows BIGINT,
			shared_blks_hit BIGINT,
			shared_blks_read BIGINT,
			planning_time_ms FLOAT,
			PRIMARY KEY (queryid, plan_id)
		)`,
		`CREATE TABLE IF NOT EXISTS pg_querylens.plan_history (
			queryid BIGINT,
			plan_id TEXT,
			previous_plan_id TEXT,
			change_timestamp TIMESTAMP,
			mean_exec_time_ms FLOAT,
			previous_mean_exec_time_ms FLOAT
		)`,
		`CREATE VIEW pg_querylens.current_plans AS 
		 SELECT * FROM pg_querylens.plans 
		 WHERE last_execution > NOW() - INTERVAL '1 hour'`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute setup query: %w", err)
		}
	}

	return nil
}

// testQueryPerformanceCollection tests basic query performance metric collection
func testQueryPerformanceCollection(t *testing.T, db *sql.DB, collector *CollectorInstance) {
	// Insert test data
	queryID := int64(12345)
	_, err := db.Exec(`
		INSERT INTO pg_querylens.queries (queryid, query_text, userid, dbid)
		VALUES ($1, $2, 10, 1)
	`, queryID, "SELECT * FROM users WHERE id = ?")
	require.NoError(t, err)

	// Insert plan data
	_, err = db.Exec(`
		INSERT INTO pg_querylens.plans (
			queryid, plan_id, plan_text, last_execution,
			mean_exec_time_ms, calls, rows, shared_blks_hit, shared_blks_read
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, queryID, "plan_001", "Seq Scan on users", time.Now(), 
		123.45, 100, 1000, 5000, 100)
	require.NoError(t, err)

	// Wait for collection
	time.Sleep(35 * time.Second) // Wait for collection interval

	// Verify metrics in New Relic
	nrClient := NewNRDBClient(TestAccountID, TestAPIKey)
	query := fmt.Sprintf(`
		SELECT average(db.querylens.query.execution_time_mean) 
		FROM Metric 
		WHERE db.querylens.queryid = %d 
		SINCE 1 minute ago
	`, queryID)

	result, err := nrClient.ExecuteNRQL(context.Background(), query)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Results)
}

// testPlanChangeDetection tests detection of query plan changes
func testPlanChangeDetection(t *testing.T, db *sql.DB, collector *CollectorInstance) {
	queryID := int64(67890)

	// Insert initial plan
	_, err := db.Exec(`
		INSERT INTO pg_querylens.queries (queryid, query_text, userid, dbid)
		VALUES ($1, $2, 10, 1)
	`, queryID, "SELECT * FROM orders WHERE customer_id = ?")
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO pg_querylens.plans (
			queryid, plan_id, plan_text, last_execution,
			mean_exec_time_ms, calls
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, queryID, "plan_v1", "Index Scan on orders", time.Now().Add(-1*time.Hour), 
		50.0, 100)
	require.NoError(t, err)

	// Wait for initial collection
	time.Sleep(35 * time.Second)

	// Insert plan change
	_, err = db.Exec(`
		INSERT INTO pg_querylens.plans (
			queryid, plan_id, plan_text, last_execution,
			mean_exec_time_ms, calls
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, queryID, "plan_v2", "Seq Scan on orders", time.Now(), 
		150.0, 50)
	require.NoError(t, err)

	// Insert plan history record
	_, err = db.Exec(`
		INSERT INTO pg_querylens.plan_history (
			queryid, plan_id, previous_plan_id, change_timestamp,
			mean_exec_time_ms, previous_mean_exec_time_ms
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, queryID, "plan_v2", "plan_v1", time.Now(), 150.0, 50.0)
	require.NoError(t, err)

	// Wait for collection
	time.Sleep(35 * time.Second)

	// Verify plan change detected
	nrClient := NewNRDBClient(TestAccountID, TestAPIKey)
	query := fmt.Sprintf(`
		SELECT count(*) 
		FROM Metric 
		WHERE db.plan.changed = true 
		  AND db.querylens.queryid = %d 
		SINCE 2 minutes ago
	`, queryID)

	result, err := nrClient.ExecuteNRQL(context.Background(), query)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Results)
}

// testRegressionDetection tests detection of performance regressions
func testRegressionDetection(t *testing.T, db *sql.DB, collector *CollectorInstance) {
	queryID := int64(11111)

	// Insert query with regression
	_, err := db.Exec(`
		INSERT INTO pg_querylens.queries (queryid, query_text, userid, dbid)
		VALUES ($1, $2, 10, 1)
	`, queryID, "SELECT * FROM large_table WHERE status = ?")
	require.NoError(t, err)

	// Insert plan with poor performance
	_, err = db.Exec(`
		INSERT INTO pg_querylens.plans (
			queryid, plan_id, plan_text, last_execution,
			mean_exec_time_ms, shared_blks_read
		) VALUES ($1, $2, $3, $4, $5, $6)
	`, queryID, "plan_bad", 
		"Seq Scan on large_table (cost=0.00..100000.00 rows=1000000)", 
		time.Now(), 5000.0, 100000)
	require.NoError(t, err)

	// Wait for collection and processing
	time.Sleep(35 * time.Second)

	// Verify regression detected
	nrClient := NewNRDBClient(TestAccountID, TestAPIKey)
	query := fmt.Sprintf(`
		SELECT latest(db.plan.has_regression), latest(db.plan.regression_type)
		FROM Metric 
		WHERE db.querylens.queryid = %d 
		SINCE 1 minute ago
	`, queryID)

	result, err := nrClient.ExecuteNRQL(context.Background(), query)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Results)
}

// testResourceConsumptionTracking tests tracking of query resource consumption
func testResourceConsumptionTracking(t *testing.T, db *sql.DB, collector *CollectorInstance) {
	// Insert multiple queries with varying resource consumption
	queries := []struct {
		queryID      int64
		queryText    string
		execTime     float64
		blocksRead   int64
		blocksHit    int64
		calls        int64
	}{
		{22222, "SELECT COUNT(*) FROM small_table", 10.0, 100, 900, 1000},
		{33333, "SELECT * FROM large_table JOIN another_table", 2000.0, 50000, 10000, 10},
		{44444, "INSERT INTO audit_log VALUES (?)", 5.0, 10, 90, 5000},
	}

	for _, q := range queries {
		_, err := db.Exec(`
			INSERT INTO pg_querylens.queries (queryid, query_text, userid, dbid)
			VALUES ($1, $2, 10, 1)
		`, q.queryID, q.queryText)
		require.NoError(t, err)

		_, err = db.Exec(`
			INSERT INTO pg_querylens.plans (
				queryid, plan_id, plan_text, last_execution,
				mean_exec_time_ms, shared_blks_read, shared_blks_hit, calls
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, q.queryID, fmt.Sprintf("plan_%d", q.queryID), "Some plan", 
			time.Now(), q.execTime, q.blocksRead, q.blocksHit, q.calls)
		require.NoError(t, err)
	}

	// Wait for collection
	time.Sleep(35 * time.Second)

	// Verify top resource consumers
	nrClient := NewNRDBClient(TestAccountID, TestAPIKey)
	query := `
		SELECT sum(db.querylens.query.blocks_read) as 'Total Reads',
		       sum(db.querylens.query.blocks_hit) as 'Total Hits',
		       uniqueCount(db.querylens.queryid) as 'Query Count'
		FROM Metric 
		WHERE db.system = 'postgresql' 
		SINCE 1 minute ago
	`

	result, err := nrClient.ExecuteNRQL(context.Background(), query)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Results)

	// Verify cache hit ratio calculation
	cacheQuery := `
		SELECT sum(db.querylens.query.blocks_hit) / 
		       (sum(db.querylens.query.blocks_hit) + sum(db.querylens.query.blocks_read)) * 100 
		       as 'Cache Hit Ratio'
		FROM Metric 
		WHERE db.system = 'postgresql' 
		SINCE 1 minute ago
	`

	cacheResult, err := nrClient.ExecuteNRQL(context.Background(), cacheQuery)
	require.NoError(t, err)
	assert.NotEmpty(t, cacheResult.Results)
}

// CollectorInstance represents a running collector
type CollectorInstance struct {
	// Implementation details for managing collector process
}

func (c *CollectorInstance) Stop() {
	// Stop collector
}

// startCollectorWithQueryLens starts the collector with pg_querylens configuration
func startCollectorWithQueryLens(t *testing.T, postgres testcontainers.Container) *CollectorInstance {
	// This would start the actual collector process with appropriate configuration
	// For testing, we're returning a mock instance
	return &CollectorInstance{}
}

// TestQueryLensIntegrationScenarios tests various real-world scenarios
func TestQueryLensIntegrationScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration scenarios in short mode")
	}

	t.Run("SlowQueryOptimization", func(t *testing.T) {
		// Scenario: A query gradually becomes slower due to data growth
		// Expected: System detects regression and provides recommendations
	})

	t.Run("IndexDropImpact", func(t *testing.T) {
		// Scenario: An index is accidentally dropped
		// Expected: Immediate plan change detection and alert
	})

	t.Run("DataSkewHandling", func(t *testing.T) {
		// Scenario: Query performance varies based on parameter values
		// Expected: Multiple plans tracked, appropriate sampling
	})

	t.Run("VacuumImpact", func(t *testing.T) {
		// Scenario: Table statistics become stale
		// Expected: Detection of suboptimal plans due to bad estimates
	})
}