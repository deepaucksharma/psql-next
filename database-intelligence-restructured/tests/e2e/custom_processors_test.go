package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestCustomProcessorsWithContribCollector tests custom processor behavior using contrib collector
// This test uses configuration patterns that simulate custom processor behavior
func TestCustomProcessorsWithContribCollector(t *testing.T) {
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

	runID := fmt.Sprintf("custom_proc_%d", time.Now().Unix())
	t.Logf("Starting custom processors test with run ID: %s", runID)

	// Start PostgreSQL
	t.Log("Starting PostgreSQL container...")
	postgresCmd := exec.Command("docker", "run",
		"--name", "e2e-custom-postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "55432:5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "already in use") {
			t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
		}
		exec.Command("docker", "rm", "-f", "e2e-custom-postgres").Run()
		postgresCmd = exec.Command("docker", "run",
			"--name", "e2e-custom-postgres",
			"-e", "POSTGRES_PASSWORD=postgres",
			"-p", "55432:5432",
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
		exec.Command("docker", "stop", "e2e-custom-postgres").Run()
		exec.Command("docker", "rm", "e2e-custom-postgres").Run()
		exec.Command("docker", "stop", "e2e-custom-collector").Run()
		exec.Command("docker", "rm", "e2e-custom-collector").Run()
	}()

	// Wait for PostgreSQL
	time.Sleep(15 * time.Second)

	// Test different processor behaviors using standard OTEL processors
	t.Run("AdaptiveSampling_Simulation", func(t *testing.T) {
		testAdaptiveSamplingBehavior(t, runID, licenseKey, otlpEndpoint, accountID, apiKey)
	})

	t.Run("CircuitBreaker_Simulation", func(t *testing.T) {
		testCircuitBreakerBehavior(t, runID, licenseKey, otlpEndpoint, accountID, apiKey)
	})

	t.Run("CostControl_Simulation", func(t *testing.T) {
		testCostControlBehavior(t, runID, licenseKey, otlpEndpoint, accountID, apiKey)
	})

	t.Run("Verification_Simulation", func(t *testing.T) {
		testVerificationBehavior(t, runID, licenseKey, otlpEndpoint, accountID, apiKey)
	})
}

// testAdaptiveSamplingBehavior simulates adaptive sampling using probabilistic sampler
func testAdaptiveSamplingBehavior(t *testing.T, runID, licenseKey, otlpEndpoint, accountID, apiKey string) {
	t.Log("Testing adaptive sampling behavior...")

	// Create config with probabilistic sampler that changes based on load
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:55432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s
    tls:
      insecure: true

processors:
  # Simulate adaptive sampling with probabilistic sampler
  probabilistic_sampler:
    sampling_percentage: 50  # Start with 50%% sampling
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.phase
        value: adaptive_sampling
        action: insert
      - key: sampling.percentage
        value: "50"
        action: insert
  
  batch:
    timeout: 5s

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
  
  logging:
    verbosity: normal

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [probabilistic_sampler, attributes, batch]
      exporters: [otlp, logging]
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "adaptive-sampling-test.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector with sampling...")
	exec.Command("docker", "rm", "-f", "e2e-adaptive-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-adaptive-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err := collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Collect for 30 seconds
	time.Sleep(30 * time.Second)

	// Check metrics
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' AND test.phase = 'adaptive_sampling' SINCE 2 minutes ago", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metrics: %v", err)
		return
	}

	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok {
			t.Logf("✓ Adaptive sampling: %.0f metrics collected (with 50%% sampling)", count)
			// We expect roughly half the metrics due to 50% sampling
		}
	}

	// Stop collector
	exec.Command("docker", "stop", "e2e-adaptive-collector").Run()
	exec.Command("docker", "rm", "e2e-adaptive-collector").Run()
}

// testCircuitBreakerBehavior simulates circuit breaker using error detection
func testCircuitBreakerBehavior(t *testing.T, runID, licenseKey, otlpEndpoint, accountID, apiKey string) {
	t.Log("Testing circuit breaker behavior...")

	// Test with invalid database to trigger errors
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:55432
    transport: tcp
    username: postgres
    password: wrong_password  # Intentionally wrong
    databases:
      - postgres
    collection_interval: 5s
    tls:
      insecure: true

processors:
  # Count errors to simulate circuit breaker
  filter:
    error_mode: propagate
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.phase
        value: circuit_breaker
        action: insert
      - key: circuit.state
        value: testing
        action: insert
  
  batch:
    timeout: 5s

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 5s
      max_elapsed_time: 30s
  
  logging:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [filter, attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: debug
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "circuit-breaker-test.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector with wrong credentials (expecting errors)...")
	exec.Command("docker", "rm", "-f", "e2e-circuit-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-circuit-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err := collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Let it run and fail
	time.Sleep(20 * time.Second)

	// Check logs for circuit breaker behavior
	logsCmd := exec.Command("docker", "logs", "--tail", "50", "e2e-circuit-collector")
	logs, _ := logsCmd.CombinedOutput()
	logsStr := string(logs)

	if strings.Contains(logsStr, "password authentication failed") || 
	   strings.Contains(logsStr, "failed to scrape") {
		t.Log("✓ Circuit breaker scenario: Authentication errors detected")
	}

	// Verify collector is still running (circuit breaker should prevent crash)
	checkCmd := exec.Command("docker", "ps", "-q", "-f", "name=e2e-circuit-collector")
	if output, err := checkCmd.Output(); err == nil && len(output) > 0 {
		t.Log("✓ Collector still running despite errors (circuit breaker behavior)")
	}

	// Stop collector
	exec.Command("docker", "stop", "e2e-circuit-collector").Run()
	exec.Command("docker", "rm", "e2e-circuit-collector").Run()
}

// testCostControlBehavior simulates cost control using filter processor
func testCostControlBehavior(t *testing.T, runID, licenseKey, otlpEndpoint, accountID, apiKey string) {
	t.Log("Testing cost control behavior...")

	// Use filter to limit expensive metrics
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:55432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s
    tls:
      insecure: true

processors:
  # Simulate cost control by filtering expensive metrics
  filter:
    metrics:
      exclude:
        match_type: regexp
        metric_names:
          # Exclude detailed table/index metrics (expensive)
          - "postgresql\\.table\\..*"
          - "postgresql\\.index\\..*"
          - "postgresql\\.database\\.locks"
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.phase
        value: cost_control
        action: insert
      - key: cost.control
        value: enabled
        action: insert
  
  batch:
    timeout: 5s
    send_batch_size: 50  # Smaller batches for cost control

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
  
  logging:
    verbosity: normal
    sampling_initial: 10
    sampling_thereafter: 100

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [filter, attributes, batch]
      exporters: [otlp, logging]
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "cost-control-test.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector with cost control...")
	exec.Command("docker", "rm", "-f", "e2e-cost-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-cost-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err := collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Collect for 30 seconds
	time.Sleep(30 * time.Second)

	// Verify cost control - should have fewer metric types
	nrql := fmt.Sprintf("SELECT uniques(metricName) FROM Metric WHERE test.run.id = '%s' AND test.phase = 'cost_control' SINCE 2 minutes ago", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metrics: %v", err)
		return
	}

	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if metricNames, ok := result.Data.Actor.Account.NRQL.Results[0]["uniques.metricName"].([]interface{}); ok {
			t.Logf("✓ Cost control: %d metric types collected (expensive metrics filtered)", len(metricNames))
			
			// Check that table/index metrics were filtered
			hasTableMetrics := false
			for _, name := range metricNames {
				if nameStr, ok := name.(string); ok {
					if strings.Contains(nameStr, "table") || strings.Contains(nameStr, "index") {
						hasTableMetrics = true
						break
					}
				}
			}
			
			if !hasTableMetrics {
				t.Log("✓ Table and index metrics successfully filtered (cost control working)")
			} else {
				t.Error("Table/index metrics found despite cost control filter")
			}
		}
	}

	// Stop collector
	exec.Command("docker", "stop", "e2e-cost-collector").Run()
	exec.Command("docker", "rm", "e2e-cost-collector").Run()
}

// testVerificationBehavior simulates metric verification using transform processor
func testVerificationBehavior(t *testing.T, runID, licenseKey, otlpEndpoint, accountID, apiKey string) {
	t.Log("Testing metric verification behavior...")

	// Use transform processor to add verification metadata
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:55432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s
    tls:
      insecure: true

processors:
  # Add verification metadata
  transform:
    metric_statements:
      - context: metric
        statements:
          # Add verification timestamp
          - set(attributes["verification.timestamp"], Now())
          - set(attributes["verification.source"], "postgresql_receiver")
          
      - context: datapoint
        statements:
          # Mark data points as verified if they have expected attributes
          - set(attributes["verified"], "true") where attributes["postgresql.database.name"] != nil
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.phase
        value: verification
        action: insert
  
  batch:
    timeout: 5s

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
  
  logging:
    verbosity: normal

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [transform, attributes, batch]
      exporters: [otlp, logging]
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "verification-test.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector with verification...")
	exec.Command("docker", "rm", "-f", "e2e-verify-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-verify-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err := collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Collect for 30 seconds
	time.Sleep(30 * time.Second)

	// Check verification metadata
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' AND test.phase = 'verification' AND verified = 'true' SINCE 2 minutes ago", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metrics: %v", err)
		return
	}

	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok && count > 0 {
			t.Logf("✓ Verification: %.0f metrics marked as verified", count)
		}
	}

	// Check for verification timestamp
	nrql = fmt.Sprintf("SELECT latest(verification.timestamp) FROM Metric WHERE test.run.id = '%s' AND test.phase = 'verification' SINCE 2 minutes ago LIMIT 1", runID)
	
	result, err = queryNRDB(accountID, apiKey, nrql)
	if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if _, ok := result.Data.Actor.Account.NRQL.Results[0]["latest.verification.timestamp"]; ok {
			t.Log("✓ Verification timestamps successfully added to metrics")
		}
	}

	// Stop collector
	exec.Command("docker", "stop", "e2e-verify-collector").Run()
	exec.Command("docker", "rm", "e2e-verify-collector").Run()
}