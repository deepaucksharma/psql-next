// Package e2e contains end-to-end tests for the Database Intelligence Collector.
// This file resolves package conflicts and provides shared test infrastructure.
package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// Common test infrastructure to avoid duplicates

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Setup test environment
	setupTestEnvironment()
	
	// Run tests
	code := m.Run()
	
	// Cleanup
	cleanupTestEnvironment()
	
	os.Exit(code)
}

func setupTestEnvironment() {
	// Create output directory
	os.MkdirAll("output", 0755)
	os.MkdirAll("test-results", 0755)
}

func cleanupTestEnvironment() {
	// Cleanup is handled by individual tests
}

// getEnvOrDefaultUnique avoids conflicts with duplicate declarations
func getEnvOrDefaultUnique(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// createTestDatabase creates a test database connection
func createTestDatabase(t *testing.T, dbType string) *sql.DB {
	var dsn string
	
	switch dbType {
	case "postgres":
		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			getEnvOrDefaultUnique("POSTGRES_HOST", "localhost"),
			getEnvOrDefaultUnique("POSTGRES_PORT", "5432"),
			getEnvOrDefaultUnique("POSTGRES_USER", "postgres"),
			getEnvOrDefaultUnique("POSTGRES_PASSWORD", "postgres"),
			getEnvOrDefaultUnique("POSTGRES_DB", "testdb"))
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
			getEnvOrDefaultUnique("MYSQL_USER", "root"),
			getEnvOrDefaultUnique("MYSQL_PASSWORD", "root"),
			getEnvOrDefaultUnique("MYSQL_HOST", "localhost"),
			getEnvOrDefaultUnique("MYSQL_PORT", "3306"),
			getEnvOrDefaultUnique("MYSQL_DB", "testdb"))
	default:
		t.Fatalf("Unknown database type: %s", dbType)
	}
	
	db, err := sql.Open(dbType, dsn)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("Cannot connect to %s database: %v", dbType, err)
	}
	
	return db
}

// generateTestWorkload generates a standard test workload
func generateTestWorkload(t *testing.T, db *sql.DB, workloadType string) {
	switch workloadType {
	case "basic":
		// Basic queries
		queries := []string{
			"SELECT 1",
			"SELECT COUNT(*) FROM information_schema.tables",
			"SELECT version()",
		}
		for _, q := range queries {
			db.Exec(q)
		}
		
	case "complex":
		// Complex queries with joins and aggregations
		setupTestTables(t, db)
		queries := []string{
			"SELECT u.id, COUNT(o.id) FROM test_users u LEFT JOIN test_orders o ON u.id = o.user_id GROUP BY u.id",
			"SELECT AVG(total), MAX(total), MIN(total) FROM test_orders",
			"SELECT * FROM test_users WHERE created_at > NOW() - INTERVAL '1 day'",
		}
		for _, q := range queries {
			db.Exec(q)
		}
		
	case "high-volume":
		// High volume queries
		for i := 0; i < 1000; i++ {
			db.Exec(fmt.Sprintf("SELECT %d", i))
		}
	}
}

func setupTestTables(t *testing.T, db *sql.DB) {
	// Create test tables if not exist
	tables := []string{
		`CREATE TABLE IF NOT EXISTS test_users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS test_orders (
			id SERIAL PRIMARY KEY,
			user_id INT,
			total DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
	}
	
	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			// Ignore errors - tables might already exist
			t.Logf("Table creation warning: %v", err)
		}
	}
	
	// Insert test data
	for i := 1; i <= 10; i++ {
		db.Exec("INSERT INTO test_users (email) VALUES ($1) ON CONFLICT DO NOTHING",
			fmt.Sprintf("test%d@example.com", i))
		db.Exec("INSERT INTO test_orders (user_id, total) VALUES ($1, $2)",
			i, float64(i*10))
	}
}