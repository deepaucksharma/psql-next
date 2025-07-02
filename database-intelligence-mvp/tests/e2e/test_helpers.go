package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// QueryPattern represents a database query pattern for testing
type QueryPattern struct {
	Name        string
	Query       string
	Parameters  []interface{}
	Duration    time.Duration
	ShouldBlock bool
	ShouldFail  bool
}

// LoadPattern represents a database load pattern
type LoadPattern struct {
	Name            string
	Connections     int
	Duration        time.Duration
	QueryPatterns   []QueryPattern
	ThinkTime       time.Duration
	RampUpTime      time.Duration
}

// generateSlowQueries generates queries that trigger auto_explain
func generateSlowQueries(t *testing.T, db *sql.DB) {
	queries := []string{
		`SELECT u.*, COUNT(o.id) as order_count 
		 FROM users u 
		 LEFT JOIN orders o ON u.id = o.user_id 
		 GROUP BY u.id 
		 HAVING COUNT(o.id) > 5`,
		
		`SELECT * FROM users u1 
		 WHERE EXISTS (
		   SELECT 1 FROM users u2 
		   WHERE u2.email LIKE '%@example.com' 
		   AND u2.created_at > u1.created_at
		 )`,
		
		`WITH RECURSIVE user_tree AS (
		   SELECT id, email, 1 as level 
		   FROM users WHERE id = 1
		   UNION ALL
		   SELECT u.id, u.email, ut.level + 1
		   FROM users u
		   JOIN user_tree ut ON u.id = ut.id + 1
		   WHERE ut.level < 10
		 )
		 SELECT * FROM user_tree`,
	}

	for i, query := range queries {
		// Add delay to make query slow
		slowQuery := fmt.Sprintf("SELECT pg_sleep(0.1); %s", query)
		_, err := db.Exec(slowQuery)
		if err != nil {
			t.Logf("Slow query %d failed (expected for some): %v", i, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// causePlanRegression creates a scenario that causes plan regression
func causePlanRegression(t *testing.T, db *sql.DB) {
	// Create index
	_, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_email_regress ON users(email)")
	require.NoError(t, err)

	// Run queries with index (fast plan)
	for i := 0; i < 10; i++ {
		email := fmt.Sprintf("user%d@example.com", i)
		db.Exec("SELECT * FROM users WHERE email = $1", email)
	}

	// Drop index to cause regression
	_, err = db.Exec("DROP INDEX IF EXISTS idx_users_email_regress")
	require.NoError(t, err)

	// Run same queries without index (slow plan)
	for i := 0; i < 10; i++ {
		email := fmt.Sprintf("user%d@example.com", i)
		db.Exec("SELECT * FROM users WHERE email = $1", email)
	}
}

// generateASHActivity generates various session states for ASH testing
func generateASHActivity(t *testing.T, db *sql.DB) {
	var wg sync.WaitGroup

	// Active sessions
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := sql.Open("postgres", getTestDSN())
			if err != nil {
				return
			}
			defer conn.Close()

			// CPU intensive query
			conn.Exec("SELECT COUNT(*) FROM generate_series(1, 1000000)")
		}(i)
	}

	// Idle in transaction
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := sql.Open("postgres", getTestDSN())
			if err != nil {
				return
			}
			defer conn.Close()

			tx, _ := conn.Begin()
			tx.Exec("SELECT 1")
			time.Sleep(3 * time.Second)
			tx.Commit()
		}(i)
	}

	// Blocked sessions
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := sql.Open("postgres", getTestDSN())
		if err != nil {
			return
		}
		defer conn.Close()

		// Hold lock
		tx, _ := conn.Begin()
		tx.Exec("UPDATE users SET email = 'locked@example.com' WHERE id = 1")
		
		// Let other sessions try to acquire lock
		time.Sleep(2 * time.Second)
		
		tx.Rollback()
	}()

	// Sessions trying to acquire lock
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := sql.Open("postgres", getTestDSN())
			if err != nil {
				return
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			conn.ExecContext(ctx, "UPDATE users SET email = $1 WHERE id = 1", 
				fmt.Sprintf("blocked%d@example.com", id))
		}(i)
	}

	wg.Wait()
}

// createLockContention creates explicit lock contention scenarios
func createLockContention(t *testing.T, db *sql.DB) {
	// Start a transaction that holds a lock
	lockConn, err := sql.Open("postgres", getTestDSN())
	require.NoError(t, err)
	defer lockConn.Close()

	tx, err := lockConn.Begin()
	require.NoError(t, err)

	_, err = tx.Exec("UPDATE users SET email = 'locked@example.com' WHERE id = 1")
	require.NoError(t, err)

	// Start multiple goroutines trying to update the same row
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			conn, err := sql.Open("postgres", getTestDSN())
			if err != nil {
				return
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			_, err = conn.ExecContext(ctx, 
				"UPDATE users SET email = $1 WHERE id = 1", 
				fmt.Sprintf("waiting%d@example.com", id))
			
			if err != nil {
				t.Logf("Expected lock timeout for goroutine %d: %v", id, err)
			}
		}(i)
	}

	// Hold the lock for a bit
	time.Sleep(3 * time.Second)
	
	// Release the lock
	tx.Rollback()
	
	// Wait for all goroutines to complete
	wg.Wait()
}

// generateHighCardinalityQueries creates many unique queries
func generateHighCardinalityQueries(t *testing.T, db *sql.DB, count int) {
	for i := 0; i < count; i++ {
		// Generate unique query with random elements
		uniqueID := rand.Intn(1000000)
		randomString := generateRandomString(10)
		
		query := fmt.Sprintf(
			"SELECT %d as id, '%s' as random_value, COUNT(*) FROM users WHERE id > %d",
			uniqueID, randomString, rand.Intn(100))
		
		db.Exec(query)
		
		if i%100 == 0 {
			t.Logf("Generated %d high cardinality queries", i)
		}
	}
}

// applyLoadPattern applies a specific load pattern to the database
func applyLoadPattern(t *testing.T, db *sql.DB, pattern LoadPattern) {
	t.Logf("Applying load pattern: %s", pattern.Name)
	
	ctx, cancel := context.WithTimeout(context.Background(), pattern.Duration)
	defer cancel()

	var wg sync.WaitGroup
	
	// Ramp up connections gradually
	for i := 0; i < pattern.Connections; i++ {
		wg.Add(1)
		
		// Stagger connection starts during ramp-up
		if pattern.RampUpTime > 0 {
			delay := pattern.RampUpTime / time.Duration(pattern.Connections) * time.Duration(i)
			time.Sleep(delay)
		}
		
		go func(connID int) {
			defer wg.Done()
			
			conn, err := sql.Open("postgres", getTestDSN())
			if err != nil {
				t.Logf("Failed to create connection %d: %v", connID, err)
				return
			}
			defer conn.Close()

			// Execute queries until context expires
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Pick a random query pattern
					queryPattern := pattern.QueryPatterns[rand.Intn(len(pattern.QueryPatterns))]
					
					// Execute the query
					start := time.Now()
					_, err := conn.Exec(queryPattern.Query, queryPattern.Parameters...)
					duration := time.Since(start)
					
					if err != nil && !queryPattern.ShouldFail {
						t.Logf("Query failed unexpectedly: %v", err)
					}
					
					// Log slow queries
					if duration > 100*time.Millisecond {
						t.Logf("Slow query detected: %s took %v", queryPattern.Name, duration)
					}
					
					// Think time between queries
					if pattern.ThinkTime > 0 {
						time.Sleep(pattern.ThinkTime + time.Duration(rand.Intn(int(pattern.ThinkTime/2))))
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	t.Logf("Load pattern %s completed", pattern.Name)
}

// validateMetricPresence checks if expected metrics are present
func validateMetricPresence(t *testing.T, exporter *MockNRDBExporter, expectedMetrics []string) {
	metrics := exporter.GetMetrics()
	
	metricNames := make(map[string]bool)
	for _, m := range metrics {
		metricNames[m.Name] = true
	}
	
	for _, expected := range expectedMetrics {
		assert.True(t, metricNames[expected], "Expected metric %s not found", expected)
	}
}

// waitForMetrics waits for specific metrics to appear
func waitForMetrics(t *testing.T, exporter *MockNRDBExporter, metricName string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		metrics := exporter.GetMetrics()
		for _, m := range metrics {
			if m.Name == metricName {
				return true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	return false
}

// getTestDSN returns the PostgreSQL connection string for tests
func getTestDSN() string {
	host := getEnvOrDefault("POSTGRES_HOST", "localhost")
	port := getEnvOrDefault("POSTGRES_PORT", "5432")
	user := getEnvOrDefault("POSTGRES_USER", "test_user")
	password := getEnvOrDefault("POSTGRES_PASSWORD", "test_password")
	dbname := getEnvOrDefault("POSTGRES_DB", "test_db")
	
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
}

// getEnvOrDefault returns environment variable or default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// Standard load patterns for testing
var (
	// Light load pattern - simulates normal operation
	LightLoadPattern = LoadPattern{
		Name:        "Light Load",
		Connections: 10,
		Duration:    1 * time.Minute,
		ThinkTime:   1 * time.Second,
		RampUpTime:  10 * time.Second,
		QueryPatterns: []QueryPattern{
			{Name: "simple_select", Query: "SELECT * FROM users WHERE id = $1", Parameters: []interface{}{1}},
			{Name: "count_query", Query: "SELECT COUNT(*) FROM users"},
			{Name: "join_query", Query: "SELECT u.*, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id LIMIT 10"},
		},
	}

	// Heavy load pattern - simulates peak traffic
	HeavyLoadPattern = LoadPattern{
		Name:        "Heavy Load",
		Connections: 50,
		Duration:    5 * time.Minute,
		ThinkTime:   100 * time.Millisecond,
		RampUpTime:  30 * time.Second,
		QueryPatterns: []QueryPattern{
			{Name: "simple_select", Query: "SELECT * FROM users WHERE id = $1", Parameters: []interface{}{1}},
			{Name: "complex_join", Query: "SELECT u.*, o.* FROM users u JOIN orders o ON u.id = o.user_id WHERE o.created_at > NOW() - INTERVAL '1 day'"},
			{Name: "aggregation", Query: "SELECT user_id, COUNT(*), SUM(total) FROM orders GROUP BY user_id"},
			{Name: "slow_query", Query: "SELECT pg_sleep(0.1), COUNT(*) FROM users", Duration: 100 * time.Millisecond},
		},
	}

	// Spike load pattern - simulates sudden traffic spike
	SpikeLoadPattern = LoadPattern{
		Name:        "Spike Load",
		Connections: 100,
		Duration:    2 * time.Minute,
		ThinkTime:   50 * time.Millisecond,
		RampUpTime:  5 * time.Second,
		QueryPatterns: []QueryPattern{
			{Name: "rapid_select", Query: "SELECT id, email FROM users WHERE id = $1", Parameters: []interface{}{1}},
			{Name: "rapid_update", Query: "UPDATE users SET last_login = NOW() WHERE id = $1", Parameters: []interface{}{1}},
		},
	}
)