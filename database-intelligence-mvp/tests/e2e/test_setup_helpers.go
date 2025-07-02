package e2e

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

// setupTestSchema creates the test database schema
func setupTestSchema(t *testing.T, db *sql.DB) {
	t.Log("Setting up test schema")

	// Create schema for E2E tests
	_, err := db.Exec(`CREATE SCHEMA IF NOT EXISTS e2e_test`)
	require.NoError(t, err)

	// Create users table with PII fields
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test.users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			phone VARCHAR(50),
			ssn VARCHAR(20),
			credit_card VARCHAR(50),
			last_login TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(t, err)

	// Create orders table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test.orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES e2e_test.users(id),
			total DECIMAL(10,2),
			status VARCHAR(50),
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(t, err)

	// Create indexes for testing plan changes
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_users_email ON e2e_test.users(email)`)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_orders_user_id ON e2e_test.orders(user_id)`)
	require.NoError(t, err)

	// Create functions for generating test queries
	_, err = db.Exec(`
		CREATE OR REPLACE FUNCTION e2e_test.generate_join_query()
		RETURNS TABLE(id INT, email VARCHAR, order_count BIGINT) AS $$
		BEGIN
			RETURN QUERY
			SELECT u.id, u.email, COUNT(o.id) as order_count
			FROM e2e_test.users u
			LEFT JOIN e2e_test.orders o ON u.id = o.user_id
			GROUP BY u.id, u.email;
		END;
		$$ LANGUAGE plpgsql;
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE OR REPLACE FUNCTION e2e_test.generate_expensive_query()
		RETURNS TABLE(user_id INT, total_spent DECIMAL) AS $$
		BEGIN
			-- Simulate expensive query with pg_sleep
			PERFORM pg_sleep(0.1);
			
			RETURN QUERY
			WITH user_totals AS (
				SELECT user_id, SUM(total) as total_spent
				FROM e2e_test.orders
				GROUP BY user_id
			)
			SELECT ut.user_id, ut.total_spent
			FROM user_totals ut
			WHERE ut.total_spent > 100
			ORDER BY ut.total_spent DESC;
		END;
		$$ LANGUAGE plpgsql;
	`)
	require.NoError(t, err)

	// Create view for testing
	_, err = db.Exec(`
		CREATE OR REPLACE VIEW e2e_test.user_order_summary AS
		SELECT 
			u.id,
			u.email,
			COUNT(o.id) as order_count,
			COALESCE(SUM(o.total), 0) as total_spent,
			MAX(o.created_at) as last_order_date
		FROM e2e_test.users u
		LEFT JOIN e2e_test.orders o ON u.id = o.user_id
		GROUP BY u.id, u.email
	`)
	require.NoError(t, err)

	// Insert test data
	generator := NewTestDataGenerator(db)
	generator.GenerateUsers(t, 100)
	
	var maxUserID int
	err = db.QueryRow("SELECT MAX(id) FROM e2e_test.users").Scan(&maxUserID)
	require.NoError(t, err)
	
	generator.GenerateOrders(t, 500, maxUserID)

	// Analyze tables for query planner
	db.Exec("ANALYZE e2e_test.users")
	db.Exec("ANALYZE e2e_test.orders")

	t.Log("Test schema setup complete")
}

// getNewConnection creates a new database connection for concurrent testing
func getNewConnection(t *testing.T, testEnv *TestEnvironment) *sql.DB {
	conn, err := sql.Open("postgres", testEnv.PostgresDSN)
	require.NoError(t, err)
	
	err = conn.Ping()
	require.NoError(t, err)
	
	return conn
}

// GetMetrics retrieves metrics from the MockNRDBExporter
func (m *MockNRDBExporter) GetMetrics() []NRDBMetric {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var allMetrics []NRDBMetric
	for _, payload := range m.payloads {
		allMetrics = append(allMetrics, payload.Metrics...)
	}
	return allMetrics
}

// Test configuration constants
const (
	testPlanIntelligenceConfig = `
receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    transport: tcp
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DB}
    collection_interval: 5s

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
  
  planattributeextractor:
    safe_mode: true
    error_mode: ignore
    timeout: 500ms
    query_anonymization:
      enabled: true
      preserve_structure: true

  batch:
    timeout: 10s

exporters:
  otlp:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, planattributeextractor, batch]
      exporters: [otlp]
`

	testASHConfig = `
receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    transport: tcp
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DB}
    collection_interval: 1s

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
  
  batch:
    timeout: 5s

exporters:
  otlp:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, batch]
      exporters: [otlp]
`

	testFullIntegrationConfig = `
receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    transport: tcp
    username: ${env:POSTGRES_USER}
    password: ${env:POSTGRES_PASSWORD}
    databases:
      - ${env:POSTGRES_DB}
    collection_interval: 10s

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
  
  resource:
    attributes:
      - key: service.name
        value: database-intelligence-test
        action: insert
  
  batch:
    timeout: 10s

exporters:
  otlp:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, resource, batch]
      exporters: [otlp]
`
)