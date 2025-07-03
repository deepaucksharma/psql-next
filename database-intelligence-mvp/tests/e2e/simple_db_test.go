package e2e

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimplePostgreSQLConnection(t *testing.T) {
	// Connect to PostgreSQL
	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()

	// Test connection
	err = db.Ping()
	require.NoError(t, err)

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test (
			id SERIAL PRIMARY KEY,
			name TEXT,
			value FLOAT,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(t, err)

	// Insert test data
	testName := fmt.Sprintf("test_%d", time.Now().Unix())
	testValue := 42.5
	
	var insertedID int
	err = db.QueryRow(`
		INSERT INTO e2e_test (name, value) 
		VALUES ($1, $2) 
		RETURNING id
	`, testName, testValue).Scan(&insertedID)
	require.NoError(t, err)

	t.Logf("Inserted test record with ID: %d", insertedID)

	// Query data back
	var retrievedName string
	var retrievedValue float64
	err = db.QueryRow(`
		SELECT name, value FROM e2e_test WHERE id = $1
	`, insertedID).Scan(&retrievedName, &retrievedValue)
	require.NoError(t, err)

	// Verify data
	assert.Equal(t, testName, retrievedName)
	assert.Equal(t, testValue, retrievedValue)

	// Query statistics
	var tableCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pg_stat_user_tables 
		WHERE schemaname = 'public'
	`).Scan(&tableCount)
	require.NoError(t, err)

	t.Logf("Found %d user tables", tableCount)
	assert.Greater(t, tableCount, 0)

	// Check if metrics would be collected
	var rowsFetched sql.NullInt64
	err = db.QueryRow(`
		SELECT SUM(n_tup_fetched) 
		FROM pg_stat_user_tables 
		WHERE schemaname = 'public'
	`).Scan(&rowsFetched)
	require.NoError(t, err)

	if rowsFetched.Valid {
		t.Logf("Total rows fetched from user tables: %d", rowsFetched.Int64)
	}

	// Cleanup
	_, err = db.Exec(`DROP TABLE IF EXISTS e2e_test`)
	require.NoError(t, err)
}

func TestMetricsCollection(t *testing.T) {
	// This test simulates what metrics would be collected
	
	// Connect to PostgreSQL
	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()

	// Simulate metric collection queries
	metricsQueries := []struct {
		name  string
		query string
	}{
		{
			name: "database_size",
			query: `SELECT pg_database_size(current_database())`,
		},
		{
			name: "connection_count",
			query: `SELECT COUNT(*) FROM pg_stat_activity`,
		},
		{
			name: "transaction_count",
			query: `SELECT xact_commit + xact_rollback FROM pg_stat_database WHERE datname = current_database()`,
		},
		{
			name: "cache_hit_ratio",
			query: `SELECT 
				CASE 
					WHEN blks_hit + blks_read = 0 THEN 0 
					ELSE blks_hit::float / (blks_hit + blks_read) 
				END as ratio
				FROM pg_stat_database 
				WHERE datname = current_database()`,
		},
	}

	for _, mq := range metricsQueries {
		t.Run(mq.name, func(t *testing.T) {
			var result sql.NullFloat64
			err := db.QueryRow(mq.query).Scan(&result)
			require.NoError(t, err)

			if result.Valid {
				t.Logf("%s: %.2f", mq.name, result.Float64)
			} else {
				t.Logf("%s: NULL", mq.name)
			}
		})
	}
}

func TestQueryPlanExtraction(t *testing.T) {
	// Connect to PostgreSQL
	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()

	// Create test table with data
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS plan_test (
			id SERIAL PRIMARY KEY,
			data TEXT,
			category INT
		)
	`)
	require.NoError(t, err)
	defer db.Exec(`DROP TABLE IF EXISTS plan_test`)

	// Insert test data
	for i := 0; i < 100; i++ {
		_, err = db.Exec(`
			INSERT INTO plan_test (data, category) 
			VALUES ($1, $2)
		`, fmt.Sprintf("data_%d", i), i%10)
		require.NoError(t, err)
	}

	// Get query plan
	rows, err := db.Query(`EXPLAIN (FORMAT JSON) SELECT * FROM plan_test WHERE category = 5`)
	require.NoError(t, err)
	defer rows.Close()

	var planJSON string
	for rows.Next() {
		var line string
		err = rows.Scan(&line)
		require.NoError(t, err)
		planJSON += line
	}

	t.Logf("Query plan: %s", planJSON)
	assert.Contains(t, planJSON, "Plan")

	// Test with ANALYZE
	rows2, err := db.Query(`EXPLAIN (ANALYZE, FORMAT JSON) SELECT COUNT(*) FROM plan_test`)
	require.NoError(t, err)
	defer rows2.Close()

	var analyzePlan string
	for rows2.Next() {
		var line string
		err = rows2.Scan(&line)
		require.NoError(t, err)
		analyzePlan += line
	}

	t.Logf("Analyze plan length: %d", len(analyzePlan))
	assert.Contains(t, analyzePlan, "Execution Time")
}