package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// getEnvOrDefault returns env value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// TestLocalCollectorRun tests running the collector locally without Docker
func TestLocalCollectorRun(t *testing.T) {
	// Skip if no credentials
	if os.Getenv("NEW_RELIC_LICENSE_KEY") == "" {
		t.Skip("NEW_RELIC_LICENSE_KEY not set")
	}
	
	t.Log("Starting local collector test...")
	
	// Create a simple config that doesn't require a database
	collectorConfig := fmt.Sprintf(`
receivers:
  hostmetrics:
    collection_interval: 10s
    scrapers:
      cpu:
      memory:
      disk:
      filesystem:
      network:
      
processors:
  batch:
    timeout: 5s
    
  attributes:
    actions:
      - key: test.run.id
        value: local_collector_test_%d
        action: insert
      - key: environment
        value: e2e-test-local
        action: insert
      - key: collector.test
        value: true
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
      receivers: [hostmetrics]
      processors: [attributes, batch]
      exporters: [otlp/newrelic, debug]

  telemetry:
    logs:
      level: debug
`, time.Now().Unix(), 
   os.Getenv("NEW_RELIC_OTLP_ENDPOINT"), 
   os.Getenv("NEW_RELIC_LICENSE_KEY"))
	
	// Write config file
	configPath := "local-test-collector-config.yaml"
	err := os.WriteFile(configPath, []byte(collectorConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)
	
	// Find collector path
	collectorPath := filepath.Join("..", "..", "working-collector", "collector")
	if _, err := os.Stat(collectorPath); err != nil {
		t.Fatalf("Collector not found at %s: %v", collectorPath, err)
	}
	
	// Make sure it's executable
	if err := os.Chmod(collectorPath, 0755); err != nil {
		t.Logf("Warning: Failed to chmod collector: %v", err)
	}
	
	// Start collector
	t.Logf("Starting collector from: %s", collectorPath)
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
			t.Log("Stopping collector...")
			collectorCmd.Process.Kill()
			collectorCmd.Wait()
		}
	}()
	
	// Wait for collection cycles
	t.Log("Waiting for metric collection (30 seconds)...")
	time.Sleep(30 * time.Second)
	
	// Check if collector is still running
	if collectorCmd.ProcessState != nil && collectorCmd.ProcessState.Exited() {
		t.Fatal("Collector exited unexpectedly")
	}
	
	t.Log("Test completed successfully!")
	t.Logf("Check New Relic UI for metrics with test.run.id = local_collector_test_%d", time.Now().Unix())
}

// TestLocalCollectorWithPostgres tests the collector with a local PostgreSQL if available
func TestLocalCollectorWithPostgres(t *testing.T) {
	// Skip if no credentials
	if os.Getenv("NEW_RELIC_LICENSE_KEY") == "" {
		t.Skip("NEW_RELIC_LICENSE_KEY not set")
	}
	
	// Check if local PostgreSQL is available
	checkCmd := exec.Command("pg_isready", "-h", "localhost", "-p", "5432")
	if err := checkCmd.Run(); err != nil {
		t.Skip("Local PostgreSQL not available")
	}
	
	t.Log("Starting local collector test with PostgreSQL...")
	
	// Create collector config for PostgreSQL
	collectorConfig := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: localhost:5432
    transport: tcp
    username: %s
    password: %s
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
        value: local_postgres_test_%d
        action: insert
      - key: environment
        value: e2e-test-local-pg
        action: insert
      - key: database.type
        value: postgresql
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
`, getEnvOrDefault("POSTGRES_USER", "postgres"),
   getEnvOrDefault("POSTGRES_PASSWORD", "postgres"),
   time.Now().Unix(), 
   os.Getenv("NEW_RELIC_OTLP_ENDPOINT"), 
   os.Getenv("NEW_RELIC_LICENSE_KEY"))
	
	// Write config file
	configPath := "local-postgres-test-config.yaml"
	err := os.WriteFile(configPath, []byte(collectorConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)
	
	// Find collector path
	collectorPath := filepath.Join("..", "..", "working-collector", "collector")
	
	// Start collector
	t.Logf("Starting collector for PostgreSQL...")
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
			t.Log("Stopping collector...")
			collectorCmd.Process.Kill()
			collectorCmd.Wait()
		}
	}()
	
	// Wait for collection cycles
	t.Log("Waiting for PostgreSQL metric collection (45 seconds)...")
	time.Sleep(45 * time.Second)
	
	// Check if collector is still running
	if collectorCmd.ProcessState != nil && collectorCmd.ProcessState.Exited() {
		t.Fatal("Collector exited unexpectedly")
	}
	
	t.Log("Test completed successfully!")
	t.Logf("Check New Relic UI for PostgreSQL metrics with test.run.id = local_postgres_test_%d", time.Now().Unix())
}