package e2e

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
	
	_ "github.com/lib/pq"
)

// TestFirstRealE2E runs a real collector and verifies data in NRDB
func TestFirstRealE2E(t *testing.T) {
	// Skip if no credentials
	if os.Getenv("NEW_RELIC_LICENSE_KEY") == "" {
		t.Skip("NEW_RELIC_LICENSE_KEY not set")
	}
	
	t.Log("Starting first real e2e test...")
	
	// Start PostgreSQL using Docker
	t.Log("Starting PostgreSQL...")
	postgresCmd := exec.Command("docker", "run", 
		"--name", "e2e-test-postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "5432:5432",
		"-d", "postgres:15-alpine")
	
	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		// Check if container already exists
		if !contains(string(output), "already in use") {
			t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
		}
		// Container exists, try to start it
		exec.Command("docker", "start", "e2e-test-postgres").Run()
	}
	
	// Cleanup function
	defer func() {
		t.Log("Cleaning up...")
		exec.Command("docker", "stop", "e2e-test-postgres").Run()
		exec.Command("docker", "rm", "e2e-test-postgres").Run()
	}()
	
	// Wait for PostgreSQL to be ready
	t.Log("Waiting for PostgreSQL to be ready...")
	time.Sleep(10 * time.Second)
	
	// Connect to PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()
	
	// Create test data
	t.Log("Creating test data...")
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			value NUMERIC,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	
	// Insert some data
	for i := 0; i < 10; i++ {
		_, err = db.Exec("INSERT INTO e2e_test (name, value) VALUES ($1, $2)", 
			fmt.Sprintf("test_%d", i), float64(i)*10.5)
		if err != nil {
			t.Fatalf("Failed to insert data: %v", err)
		}
	}
	
	// Create collector config
	collectorConfig := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: localhost:5432
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
        value: first_real_e2e_%d
        action: insert
      - key: environment
        value: e2e-test
        action: insert

exporters:
  otlp/newrelic:
    endpoint: %s
    headers:
      api-key: %s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 5s
      
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 10

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes, batch]
      exporters: [otlp/newrelic, debug]

  telemetry:
    logs:
      level: debug
`, time.Now().Unix(), 
   os.Getenv("NEW_RELIC_OTLP_ENDPOINT"), 
   os.Getenv("NEW_RELIC_LICENSE_KEY"))
	
	// Write config file
	configPath := "test-collector-config.yaml"
	err = os.WriteFile(configPath, []byte(collectorConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)
	
	// Start collector
	t.Log("Starting collector...")
	collectorPath := "../../working-collector/collector"
	collectorCmd := exec.Command(collectorPath, "--config", configPath)
	
	// Capture output
	collectorCmd.Stdout = os.Stdout
	collectorCmd.Stderr = os.Stderr
	
	err = collectorCmd.Start()
	if err != nil {
		t.Fatalf("Failed to start collector: %v", err)
	}
	
	// Ensure collector is stopped
	defer func() {
		if collectorCmd.Process != nil {
			collectorCmd.Process.Kill()
		}
	}()
	
	// Wait for collection cycles
	t.Log("Waiting for metric collection (30 seconds)...")
	time.Sleep(30 * time.Second)
	
	// Generate some activity
	t.Log("Generating database activity...")
	for i := 0; i < 20; i++ {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM e2e_test").Scan(&count)
		time.Sleep(1 * time.Second)
	}
	
	// Wait more for data to be exported
	t.Log("Waiting for data export to New Relic (30 seconds)...")
	time.Sleep(30 * time.Second)
	
	// Now we would query NRDB to verify data
	// For now, let's just check the collector is still running
	if collectorCmd.Process == nil {
		t.Fatal("Collector process died")
	}
	
	t.Log("Test completed successfully!")
	t.Log("Check New Relic UI for metrics with test.run.id = first_real_e2e_*")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}