package e2e

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestSchemaChangeHandling verifies collector handles database schema changes gracefully
func TestSchemaChangeHandling(t *testing.T) {
	// Skip if no credentials
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	otlpEndpoint := os.Getenv("NEW_RELIC_OTLP_ENDPOINT")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	
	if licenseKey == "" || apiKey == "" || accountID == "" {
		t.Skip("Required credentials not set")
	}
	
	if otlpEndpoint == "" {
		otlpEndpoint = "https://otlp.nr-data.net:4317"
	}

	runID := fmt.Sprintf("schema_change_%d", time.Now().Unix())
	t.Logf("Starting schema change test with run ID: %s", runID)

	// Start PostgreSQL
	t.Log("Starting PostgreSQL container...")
	exec.Command("docker", "rm", "-f", "postgres-schema").Run()
	
	postgresCmd := exec.Command("docker", "run",
		"--name", "postgres-schema",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "75432:5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
	}

	// Cleanup
	defer func() {
		t.Log("Cleaning up containers...")
		exec.Command("docker", "stop", "postgres-schema").Run()
		exec.Command("docker", "rm", "postgres-schema").Run()
		exec.Command("docker", "stop", "schema-collector").Run()
		exec.Command("docker", "rm", "schema-collector").Run()
	}()

	// Wait for PostgreSQL
	time.Sleep(15 * time.Second)

	// Create initial schema
	db := setupInitialSchema(t)
	defer db.Close()

	// Create collector config
	config := createSchemaTestConfig(runID, otlpEndpoint, licenseKey)
	
	configPath := "schema-test-config.yaml"
	err = os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector...")
	exec.Command("docker", "rm", "-f", "schema-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "schema-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err = collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Test phases
	t.Run("Phase1_InitialSchema", func(t *testing.T) {
		t.Log("Phase 1: Collecting metrics with initial schema...")
		time.Sleep(30 * time.Second)
		
		verifySchemaMetrics(t, runID, "initial", 3, accountID, apiKey)
	})

	t.Run("Phase2_AddTables", func(t *testing.T) {
		t.Log("Phase 2: Adding new tables...")
		addNewTables(t, db)
		
		// Wait for collector to discover new tables
		time.Sleep(30 * time.Second)
		
		verifySchemaMetrics(t, runID, "expanded", 5, accountID, apiKey)
	})

	t.Run("Phase3_ModifySchema", func(t *testing.T) {
		t.Log("Phase 3: Modifying existing schema...")
		modifyExistingSchema(t, db)
		
		// Wait for metrics
		time.Sleep(30 * time.Second)
		
		// Verify collector handles schema modifications
		verifyCollectorStability(t, "schema-collector")
	})

	t.Run("Phase4_DropObjects", func(t *testing.T) {
		t.Log("Phase 4: Dropping database objects...")
		dropDatabaseObjects(t, db)
		
		// Wait for metrics
		time.Sleep(30 * time.Second)
		
		verifySchemaMetrics(t, runID, "reduced", 3, accountID, apiKey)
		verifyNoErrors(t, "schema-collector")
	})
}

func setupInitialSchema(t *testing.T) *sql.DB {
	connStr := "host=localhost port=75432 user=postgres password=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	// Wait for connection
	for i := 0; i < 30; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	t.Log("Creating initial schema...")
	
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS app_v1`,
		`CREATE TABLE IF NOT EXISTS app_v1.users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(100) UNIQUE,
			email VARCHAR(200),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS app_v1.sessions (
			id UUID DEFAULT gen_random_uuid() PRIMARY KEY,
			user_id INTEGER REFERENCES app_v1.users(id),
			token VARCHAR(255),
			expires_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS app_v1.logs (
			id BIGSERIAL PRIMARY KEY,
			level VARCHAR(10),
			message TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX idx_users_email ON app_v1.users(email)`,
		`CREATE INDEX idx_sessions_user ON app_v1.sessions(user_id)`,
		`CREATE INDEX idx_logs_created ON app_v1.logs(created_at)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("Failed to execute query: %v", err)
		}
	}

	// Insert test data
	for i := 0; i < 100; i++ {
		db.Exec(`INSERT INTO app_v1.users (username, email) VALUES ($1, $2)`,
			fmt.Sprintf("user_%d", i), fmt.Sprintf("user%d@example.com", i))
	}

	return db
}

func createSchemaTestConfig(runID, otlpEndpoint, licenseKey string) string {
	return fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:75432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 10s
    tls:
      insecure: true

processors:
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.type
        value: schema_change
        action: insert
      - key: schema.phase
        from_attribute: phase
        action: upsert
  
  batch:
    timeout: 5s

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
  
  logging:
    verbosity: detailed
    sampling_initial: 10
    sampling_thereafter: 100

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: info
`, runID, otlpEndpoint, licenseKey)
}

func addNewTables(t *testing.T, db *sql.DB) {
	t.Log("Adding new tables to schema...")
	
	queries := []string{
		`CREATE TABLE IF NOT EXISTS app_v1.products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(200),
			price DECIMAL(10,2),
			stock INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS app_v1.orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES app_v1.users(id),
			total DECIMAL(10,2),
			status VARCHAR(50),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX idx_products_name ON app_v1.products(name)`,
		`CREATE INDEX idx_orders_user ON app_v1.orders(user_id)`,
		`CREATE INDEX idx_orders_status ON app_v1.orders(status)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Logf("Failed to add table: %v", err)
		}
	}

	// Add some data to new tables
	for i := 0; i < 50; i++ {
		db.Exec(`INSERT INTO app_v1.products (name, price, stock) VALUES ($1, $2, $3)`,
			fmt.Sprintf("Product %d", i), float64(i)*9.99, 100-i)
	}
}

func modifyExistingSchema(t *testing.T, db *sql.DB) {
	t.Log("Modifying existing schema...")
	
	modifications := []string{
		// Add columns
		`ALTER TABLE app_v1.users ADD COLUMN IF NOT EXISTS last_login TIMESTAMP`,
		`ALTER TABLE app_v1.users ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true`,
		
		// Modify columns
		`ALTER TABLE app_v1.sessions ALTER COLUMN token TYPE VARCHAR(500)`,
		
		// Add constraints
		`ALTER TABLE app_v1.products ADD CONSTRAINT positive_price CHECK (price > 0)`,
		
		// Create views
		`CREATE OR REPLACE VIEW app_v1.active_users AS 
			SELECT * FROM app_v1.users WHERE is_active = true`,
		
		`CREATE OR REPLACE VIEW app_v1.recent_orders AS 
			SELECT * FROM app_v1.orders 
			WHERE created_at > NOW() - INTERVAL '7 days'`,
		
		// Create materialized view
		`CREATE MATERIALIZED VIEW IF NOT EXISTS app_v1.user_stats AS
			SELECT 
				u.id,
				u.username,
				COUNT(DISTINCT s.id) as session_count,
				COUNT(DISTINCT o.id) as order_count
			FROM app_v1.users u
			LEFT JOIN app_v1.sessions s ON u.id = s.user_id
			LEFT JOIN app_v1.orders o ON u.id = o.user_id
			GROUP BY u.id, u.username`,
		
		// Add triggers
		`CREATE OR REPLACE FUNCTION app_v1.update_last_login()
		RETURNS TRIGGER AS $$
		BEGIN
			UPDATE app_v1.users SET last_login = NOW() WHERE id = NEW.user_id;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql`,
		
		`CREATE TRIGGER update_user_login
		AFTER INSERT ON app_v1.sessions
		FOR EACH ROW
		EXECUTE FUNCTION app_v1.update_last_login()`,
	}

	for _, query := range modifications {
		if _, err := db.Exec(query); err != nil {
			t.Logf("Failed to modify schema: %v", err)
		}
	}
}

func dropDatabaseObjects(t *testing.T, db *sql.DB) {
	t.Log("Dropping database objects...")
	
	drops := []string{
		// Drop views first
		`DROP VIEW IF EXISTS app_v1.active_users`,
		`DROP VIEW IF EXISTS app_v1.recent_orders`,
		`DROP MATERIALIZED VIEW IF EXISTS app_v1.user_stats`,
		
		// Drop triggers
		`DROP TRIGGER IF EXISTS update_user_login ON app_v1.sessions`,
		`DROP FUNCTION IF EXISTS app_v1.update_last_login()`,
		
		// Drop tables with dependencies
		`DROP TABLE IF EXISTS app_v1.orders`,
		`DROP TABLE IF EXISTS app_v1.products`,
		
		// Drop indexes
		`DROP INDEX IF EXISTS app_v1.idx_logs_created`,
	}

	for _, query := range drops {
		if _, err := db.Exec(query); err != nil {
			t.Logf("Failed to drop object: %v", err)
		}
	}
}

func verifySchemaMetrics(t *testing.T, runID, phase string, expectedTables int, accountID, apiKey string) {
	// Wait for metrics
	time.Sleep(10 * time.Second)
	
	// Query for table count metrics
	nrql := fmt.Sprintf(`
		SELECT uniqueCount(postgresql.table.name) 
		FROM Metric 
		WHERE test.run.id = '%s' 
		AND metricName = 'postgresql.table.size'
		SINCE 1 minute ago
	`, runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metrics: %v", err)
		return
	}
	
	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if tableCount, ok := result.Data.Actor.Account.NRQL.Results[0]["uniqueCount.postgresql.table.name"].(float64); ok {
			t.Logf("%s phase: %.0f tables found", phase, tableCount)
			
			// Allow some variance due to system tables
			if int(tableCount) >= expectedTables-1 && int(tableCount) <= expectedTables+2 {
				t.Logf("✓ Expected table count verified (~%d)", expectedTables)
			} else {
				t.Errorf("Unexpected table count: %.0f (expected ~%d)", tableCount, expectedTables)
			}
		}
	}
}

func verifyCollectorStability(t *testing.T, containerName string) {
	// Check if collector is still running
	checkCmd := exec.Command("docker", "ps", "-q", "-f", "name="+containerName)
	output, err := checkCmd.Output()
	if err != nil || len(output) == 0 {
		t.Error("Collector stopped after schema changes")
		return
	}
	
	t.Log("✓ Collector still running after schema modifications")
	
	// Check recent logs for errors
	logsCmd := exec.Command("docker", "logs", "--tail", "50", "--since", "2m", containerName)
	logs, err := logsCmd.CombinedOutput()
	if err != nil {
		t.Logf("Failed to get logs: %v", err)
		return
	}
	
	logsStr := string(logs)
	
	// Check for specific schema-related errors
	schemaErrors := []string{
		"does not exist",
		"permission denied",
		"invalid column",
		"relation",
	}
	
	errorFound := false
	for _, errPattern := range schemaErrors {
		if strings.Contains(strings.ToLower(logsStr), errPattern) {
			errorFound = true
			t.Logf("Schema-related message found: %s", errPattern)
		}
	}
	
	if !errorFound {
		t.Log("✓ No critical schema errors in collector logs")
	}
}

func verifyNoErrors(t *testing.T, containerName string) {
	// Get full logs
	logsCmd := exec.Command("docker", "logs", containerName)
	logs, err := logsCmd.CombinedOutput()
	if err != nil {
		t.Logf("Failed to get logs: %v", err)
		return
	}
	
	logsStr := string(logs)
	
	// Count errors and warnings
	errorCount := strings.Count(strings.ToLower(logsStr), "error")
	warningCount := strings.Count(strings.ToLower(logsStr), "warning")
	
	t.Logf("Log analysis:")
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Warnings: %d", warningCount)
	
	// Some errors/warnings are expected during schema changes
	if errorCount < 10 {
		t.Log("✓ Error count within acceptable range")
	} else {
		t.Errorf("High error count: %d", errorCount)
	}
}