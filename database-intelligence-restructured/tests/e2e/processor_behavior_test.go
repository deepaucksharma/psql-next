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

// TestProcessorBehaviors tests processor behaviors using standard OTEL collector
func TestProcessorBehaviors(t *testing.T) {
	// Skip if no credentials
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	otlpEndpoint := os.Getenv("NEW_RELIC_OTLP_ENDPOINT")
	
	if licenseKey == "" {
		t.Skip("NEW_RELIC_LICENSE_KEY not set")
	}
	
	if otlpEndpoint == "" {
		otlpEndpoint = "https://otlp.nr-data.net:4317"
	}

	runID := fmt.Sprintf("processor_test_%d", time.Now().Unix())
	t.Logf("Starting processor behavior test with run ID: %s", runID)

	// Start PostgreSQL
	t.Log("Starting PostgreSQL container...")
	postgresCmd := exec.Command("docker", "run",
		"--name", "e2e-processor-postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "15432:5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "already in use") {
			t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
		}
		exec.Command("docker", "rm", "-f", "e2e-processor-postgres").Run()
		postgresCmd = exec.Command("docker", "run",
			"--name", "e2e-processor-postgres",
			"-e", "POSTGRES_PASSWORD=postgres",
			"-p", "15432:5432",
			"--network", "bridge",
			"-d", "postgres:15-alpine")
		output, err = postgresCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to start PostgreSQL after cleanup: %v\n%s", err, output)
		}
	}

	// Cleanup
	defer func() {
		t.Log("Cleaning up containers...")
		exec.Command("docker", "stop", "e2e-processor-postgres").Run()
		exec.Command("docker", "rm", "e2e-processor-postgres").Run()
		exec.Command("docker", "stop", "e2e-processor-collector").Run()
		exec.Command("docker", "rm", "e2e-processor-collector").Run()
	}()

	// Wait for PostgreSQL
	time.Sleep(15 * time.Second)

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=15432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Create test data
	t.Log("Creating test data...")
	createProcessorTestData(db)

	// Test different processor configurations
	t.Run("BatchProcessor", func(t *testing.T) {
		testBatchProcessor(t, runID, otlpEndpoint, licenseKey)
	})

	t.Run("FilterProcessor", func(t *testing.T) {
		testFilterProcessor(t, runID, otlpEndpoint, licenseKey)
	})

	t.Run("AttributesProcessor", func(t *testing.T) {
		testAttributesProcessor(t, runID, otlpEndpoint, licenseKey)
	})

	t.Run("ResourceProcessor", func(t *testing.T) {
		testResourceProcessor(t, runID, otlpEndpoint, licenseKey)
	})
}

func testBatchProcessor(t *testing.T, runID, otlpEndpoint, licenseKey string) {
	// Test batch processor with different batch sizes
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:15432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s

processors:
  batch:
    timeout: 10s
    send_batch_size: 100
    send_batch_max_size: 200
    
  attributes:
    actions:
      - key: test.run.id
        value: %s_batch
        action: insert
      - key: processor.test
        value: batch
        action: insert

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
  
  logging:
    loglevel: debug

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch, attributes]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: debug
`, runID, otlpEndpoint, licenseKey)

	runCollectorTest(t, "batch-processor-test", config, 60*time.Second)
}

func testFilterProcessor(t *testing.T, runID, otlpEndpoint, licenseKey string) {
	// Test filter processor - only send table metrics
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:15432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s

processors:
  filter:
    metrics:
      include:
        match_type: regexp
        metric_names:
          - "postgresql\\.table\\..*"
          - "postgresql\\.index\\..*"
    
  attributes:
    actions:
      - key: test.run.id
        value: %s_filter
        action: insert
      - key: processor.test
        value: filter
        action: insert

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [filter, attributes]
      exporters: [otlp]
`, runID, otlpEndpoint, licenseKey)

	runCollectorTest(t, "filter-processor-test", config, 60*time.Second)
}

func testAttributesProcessor(t *testing.T, runID, otlpEndpoint, licenseKey string) {
	// Test attributes processor with complex transformations
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:15432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s

processors:
  attributes:
    actions:
      - key: test.run.id
        value: %s_attributes
        action: insert
      - key: processor.test
        value: attributes
        action: insert
      - key: db.system
        value: postgresql
        action: insert
      - key: deployment.environment
        value: e2e-test
        action: insert
      - key: db.connection.string
        action: delete
      - key: custom.metric.type
        from_attribute: metricName
        action: insert
      - key: is_table_metric
        value: true
        action: insert
        include:
          match_type: regexp
          metric_names:
            - "postgresql\\.table\\..*"

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes]
      exporters: [otlp]
`, runID, otlpEndpoint, licenseKey)

	runCollectorTest(t, "attributes-processor-test", config, 60*time.Second)
}

func testResourceProcessor(t *testing.T, runID, otlpEndpoint, licenseKey string) {
	// Test resource processor
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:15432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s

processors:
  resource:
    attributes:
      - key: test.run.id
        value: %s_resource
        action: insert
      - key: processor.test
        value: resource
        action: insert
      - key: service.name
        value: database-intelligence-e2e
        action: insert
      - key: service.version
        value: test-1.0.0
        action: insert
      - key: cloud.provider
        value: test-cloud
        action: insert
      - key: cloud.region
        value: us-test-1
        action: insert

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [resource]
      exporters: [otlp]
`, runID, otlpEndpoint, licenseKey)

	runCollectorTest(t, "resource-processor-test", config, 60*time.Second)
}

func runCollectorTest(t *testing.T, name, config string, duration time.Duration) {
	t.Logf("Running %s...", name)
	
	// Write config
	configPath := fmt.Sprintf("%s-config.yaml", name)
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Remove existing container
	exec.Command("docker", "rm", "-f", "e2e-processor-collector").Run()

	// Start collector
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-processor-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err := collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Wait for collection
	t.Logf("Waiting for %s...", duration)
	time.Sleep(duration)

	// Check logs
	logsCmd := exec.Command("docker", "logs", "--tail", "20", "e2e-processor-collector")
	logs, _ := logsCmd.CombinedOutput()
	t.Logf("%s logs:\n%s", name, logs)

	// Stop collector
	exec.Command("docker", "stop", "e2e-processor-collector").Run()
	exec.Command("docker", "rm", "e2e-processor-collector").Run()
}

func createProcessorTestData(db *sql.DB) error {
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS processor_test`,
		`CREATE TABLE IF NOT EXISTS processor_test.test_table (
			id SERIAL PRIMARY KEY,
			data TEXT,
			value NUMERIC,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_test_created ON processor_test.test_table(created_at)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}

	// Insert data
	for i := 0; i < 100; i++ {
		_, err := db.Exec(`
			INSERT INTO processor_test.test_table (data, value) 
			VALUES ($1, $2)`,
			fmt.Sprintf("test_data_%d", i),
			float64(i)*1.5)
		if err != nil {
			return err
		}
	}

	return nil
}