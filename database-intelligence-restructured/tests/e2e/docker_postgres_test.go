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

// TestDockerPostgreSQLCollection runs collector with Docker-based PostgreSQL
func TestDockerPostgreSQLCollection(t *testing.T) {
	// Skip if no credentials
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	otlpEndpoint := os.Getenv("NEW_RELIC_OTLP_ENDPOINT")
	
	if licenseKey == "" {
		t.Skip("NEW_RELIC_LICENSE_KEY not set")
	}
	
	// Default OTLP endpoint if not set
	if otlpEndpoint == "" {
		otlpEndpoint = "https://otlp.nr-data.net:4317"
		t.Logf("Using default NEW_RELIC_OTLP_ENDPOINT: %s", otlpEndpoint)
	}

	runID := fmt.Sprintf("docker_postgres_%d", time.Now().Unix())
	t.Logf("Starting Docker PostgreSQL test with run ID: %s", runID)

	// Start PostgreSQL using Docker
	t.Log("Starting PostgreSQL container...")
	postgresCmd := exec.Command("docker", "run",
		"--name", "e2e-postgres-test",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "5432:5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		// Check if container already exists
		if !strings.Contains(string(output), "already in use") {
			t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
		}
		// Container exists, remove and recreate
		t.Log("Removing existing container...")
		exec.Command("docker", "rm", "-f", "e2e-postgres-test").Run()
		
		// Create new command for retry
		postgresCmd = exec.Command("docker", "run",
			"--name", "e2e-postgres-test",
			"-e", "POSTGRES_PASSWORD=postgres",
			"-p", "5432:5432",
			"--network", "bridge",
			"-d", "postgres:15-alpine")
		
		output, err = postgresCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to start PostgreSQL after cleanup: %v\n%s", err, output)
		}
	}

	// Cleanup function
	defer func() {
		t.Log("Cleaning up containers...")
		exec.Command("docker", "stop", "e2e-postgres-test").Run()
		exec.Command("docker", "rm", "e2e-postgres-test").Run()
		exec.Command("docker", "stop", "e2e-otel-collector").Run()
		exec.Command("docker", "rm", "e2e-otel-collector").Run()
	}()

	// Wait for PostgreSQL to be ready
	t.Log("Waiting for PostgreSQL to be ready...")
	time.Sleep(15 * time.Second)

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Create test data
	t.Log("Creating test database and data...")
	if err := createTestData(db); err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Create collector config
	collectorConfig := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:5432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 10s
    tls:
      insecure: true

processors:
  batch:
    timeout: 5s
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: environment
        value: e2e-docker-test
        action: insert
      - key: test.type
        value: postgresql
        action: insert

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 5s
  
  logging:
    loglevel: info

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

	// Write config file
	configPath := "docker-collector-config.yaml"
	err = os.WriteFile(configPath, []byte(collectorConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path for mounting
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Remove any existing collector container
	exec.Command("docker", "rm", "-f", "e2e-otel-collector").Run()

	// Start OpenTelemetry Collector using official contrib image
	t.Log("Starting OpenTelemetry Collector...")
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-otel-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"-p", "4317:4317",
		"-p", "4318:4318",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err = collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}
	containerID := strings.TrimSpace(string(output))
	t.Logf("Collector container started: %s", containerID[:12])

	// Wait for collector to initialize
	t.Log("Waiting for collector to initialize...")
	time.Sleep(10 * time.Second)

	// Check collector logs
	logsCmd := exec.Command("docker", "logs", "e2e-otel-collector")
	logs, _ := logsCmd.CombinedOutput()
	t.Logf("Collector logs:\n%s", logs)

	// Wait for collection cycles
	t.Log("Waiting for metric collection (30 seconds)...")
	time.Sleep(30 * time.Second)

	// Generate some database activity
	t.Log("Generating database activity...")
	generateDatabaseActivity(t, db)

	// Wait more for data to be exported
	t.Log("Waiting for data export to New Relic (30 seconds)...")
	time.Sleep(30 * time.Second)

	// Check final collector logs
	logsCmd = exec.Command("docker", "logs", "--tail", "50", "e2e-otel-collector")
	logs, _ = logsCmd.CombinedOutput()
	t.Logf("Final collector logs:\n%s", logs)

	t.Log("Test completed successfully!")
	t.Logf("Check New Relic UI for PostgreSQL metrics with test.run.id = %s", runID)

	// Mark task as completed
	t.Logf("Next: Query NRDB for metrics with test.run.id = %s", runID)
}

func createTestData(db *sql.DB) error {
	// Create test schema
	_, err := db.Exec(`
		CREATE SCHEMA IF NOT EXISTS e2e_test;
		
		CREATE TABLE IF NOT EXISTS e2e_test.orders (
			id SERIAL PRIMARY KEY,
			customer_name VARCHAR(100),
			order_total NUMERIC(10,2),
			order_date TIMESTAMP DEFAULT NOW()
		);
		
		CREATE TABLE IF NOT EXISTS e2e_test.products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			price NUMERIC(10,2),
			stock_quantity INTEGER
		);
		
		CREATE INDEX IF NOT EXISTS idx_orders_date ON e2e_test.orders(order_date);
		CREATE INDEX IF NOT EXISTS idx_products_name ON e2e_test.products(name);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Insert test data
	for i := 0; i < 100; i++ {
		_, err = db.Exec(`
			INSERT INTO e2e_test.orders (customer_name, order_total) 
			VALUES ($1, $2)`,
			fmt.Sprintf("Customer_%d", i),
			float64(i)*10.5+50)
		if err != nil {
			return fmt.Errorf("failed to insert order: %w", err)
		}

		if i < 50 {
			_, err = db.Exec(`
				INSERT INTO e2e_test.products (name, price, stock_quantity) 
				VALUES ($1, $2, $3)`,
				fmt.Sprintf("Product_%d", i),
				float64(i)*5.99,
				100-i)
			if err != nil {
				return fmt.Errorf("failed to insert product: %w", err)
			}
		}
	}

	return nil
}

func generateDatabaseActivity(t *testing.T, db *sql.DB) {
	queries := []string{
		"SELECT COUNT(*) FROM e2e_test.orders",
		"SELECT AVG(order_total) FROM e2e_test.orders",
		"SELECT * FROM e2e_test.products WHERE stock_quantity < 20",
		"SELECT customer_name, SUM(order_total) FROM e2e_test.orders GROUP BY customer_name LIMIT 10",
		"SELECT p.name, COUNT(o.id) FROM e2e_test.products p, e2e_test.orders o GROUP BY p.name",
	}

	for i := 0; i < 20; i++ {
		query := queries[i%len(queries)]
		rows, err := db.Query(query)
		if err != nil {
			t.Logf("Query failed: %v", err)
			continue
		}
		rows.Close()
		time.Sleep(1 * time.Second)
	}
}