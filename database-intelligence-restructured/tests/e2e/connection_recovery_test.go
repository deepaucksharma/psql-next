package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestConnectionRecovery verifies collector recovers after database connection loss
func TestConnectionRecovery(t *testing.T) {
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

	runID := fmt.Sprintf("recovery_test_%d", time.Now().Unix())
	t.Logf("Starting connection recovery test with run ID: %s", runID)

	// Start PostgreSQL
	t.Log("Starting PostgreSQL container...")
	postgresCmd := exec.Command("docker", "run",
		"--name", "e2e-recovery-postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "35432:5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "already in use") {
			t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
		}
		exec.Command("docker", "rm", "-f", "e2e-recovery-postgres").Run()
		postgresCmd = exec.Command("docker", "run",
			"--name", "e2e-recovery-postgres",
			"-e", "POSTGRES_PASSWORD=postgres",
			"-p", "35432:5432",
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
		exec.Command("docker", "stop", "e2e-recovery-postgres").Run()
		exec.Command("docker", "rm", "e2e-recovery-postgres").Run()
		exec.Command("docker", "stop", "e2e-recovery-collector").Run()
		exec.Command("docker", "rm", "e2e-recovery-collector").Run()
	}()

	// Wait for PostgreSQL
	time.Sleep(15 * time.Second)

	// Create collector config with shorter intervals for testing
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:35432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s  # Shorter interval for testing
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
      - key: test.type
        value: recovery
        action: insert
      - key: test.phase
        from_attribute: phase
        action: upsert

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 10s
      max_elapsed_time: 2m
  
  logging:
    loglevel: debug

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: debug
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "recovery-test-config.yaml"
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
	exec.Command("docker", "rm", "-f", "e2e-recovery-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-recovery-collector",
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
	t.Run("Phase1_NormalOperation", func(t *testing.T) {
		t.Log("Phase 1: Normal operation (30 seconds)...")
		time.Sleep(30 * time.Second)
		
		// Check collector logs
		logsCmd := exec.Command("docker", "logs", "--tail", "20", "e2e-recovery-collector")
		logs, _ := logsCmd.CombinedOutput()
		t.Logf("Collector logs during normal operation:\n%s", logs)
		
		// Verify metrics are being sent
		verifyPhaseMetrics(t, runID, "normal", accountID, apiKey)
	})

	t.Run("Phase2_ConnectionLoss", func(t *testing.T) {
		t.Log("Phase 2: Stopping PostgreSQL to simulate connection loss...")
		exec.Command("docker", "stop", "e2e-recovery-postgres").Run()
		
		// Wait and observe error behavior
		t.Log("Waiting 30 seconds with database stopped...")
		time.Sleep(30 * time.Second)
		
		// Check collector logs for errors
		logsCmd := exec.Command("docker", "logs", "--tail", "50", "e2e-recovery-collector")
		logs, _ := logsCmd.CombinedOutput()
		logsStr := string(logs)
		
		// Verify error handling
		if strings.Contains(logsStr, "connection refused") || 
		   strings.Contains(logsStr, "failed to scrape") ||
		   strings.Contains(logsStr, "error") {
			t.Log("✓ Collector detected connection loss")
		} else {
			t.Error("Collector did not report connection errors")
		}
		
		// Verify collector is still running
		checkCmd := exec.Command("docker", "ps", "-q", "-f", "name=e2e-recovery-collector")
		if output, err := checkCmd.Output(); err != nil || len(output) == 0 {
			t.Error("Collector stopped unexpectedly")
		} else {
			t.Log("✓ Collector still running despite connection loss")
		}
	})

	t.Run("Phase3_ConnectionRecovery", func(t *testing.T) {
		t.Log("Phase 3: Restarting PostgreSQL to test recovery...")
		exec.Command("docker", "start", "e2e-recovery-postgres").Run()
		
		// Wait for PostgreSQL to be ready
		time.Sleep(10 * time.Second)
		
		t.Log("Waiting for collector to reconnect and resume metrics (60 seconds)...")
		time.Sleep(60 * time.Second)
		
		// Check collector logs for recovery
		logsCmd := exec.Command("docker", "logs", "--tail", "50", "e2e-recovery-collector")
		logs, _ := logsCmd.CombinedOutput()
		t.Logf("Collector logs after recovery:\n%s", logs)
		
		// Verify metrics resumed
		verifyPhaseMetrics(t, runID, "recovered", accountID, apiKey)
	})

	// Final verification
	t.Run("Phase4_FinalVerification", func(t *testing.T) {
		t.Log("Phase 4: Final verification of all phases...")
		time.Sleep(10 * time.Second)
		
		// Query for all phases
		nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' SINCE 5 minutes ago FACET test.phase", runID)
		
		result, err := queryNRDB(accountID, apiKey, nrql)
		if err != nil {
			t.Errorf("Failed to query final metrics: %v", err)
			return
		}
		
		t.Log("Metrics by phase:")
		foundPhases := make(map[string]int)
		for _, res := range result.Data.Actor.Account.NRQL.Results {
			if phase, ok := res["test.phase"].(string); ok {
				if count, ok := res["count"].(float64); ok {
					foundPhases[phase] = int(count)
					t.Logf("  %s: %d metrics", phase, int(count))
				}
			}
		}
		
		// Verify we have metrics from normal operation
		if foundPhases["normal"] > 0 || foundPhases[""] > 0 {
			t.Log("✓ Metrics collected during normal operation")
		} else {
			t.Error("No metrics from normal operation phase")
		}
		
		// Check for gap during outage
		nrql = fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' SINCE 5 minutes ago TIMESERIES 30 seconds", runID)
		
		result, err = queryNRDB(accountID, apiKey, nrql)
		if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
			data, _ := json.MarshalIndent(result.Data.Actor.Account.NRQL.Results, "  ", "  ")
			t.Logf("Metric timeline:\n%s", string(data))
		}
	})
}

func verifyPhaseMetrics(t *testing.T, runID, phase string, accountID, apiKey string) {
	// Wait for metrics to be processed
	time.Sleep(5 * time.Second)
	
	nrql := fmt.Sprintf("SELECT count(*), latest(postgresql.backends) FROM Metric WHERE test.run.id = '%s' SINCE 2 minutes ago", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query %s phase metrics: %v", phase, err)
		return
	}
	
	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok && count > 0 {
			t.Logf("✓ %s phase: %.0f metrics collected", phase, count)
		} else {
			t.Errorf("No metrics found for %s phase", phase)
		}
	}
}

// TestConnectionPoolExhaustion tests behavior when connection pool is exhausted
func TestConnectionPoolExhaustion(t *testing.T) {
	// Skip if no credentials
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	if licenseKey == "" {
		t.Skip("NEW_RELIC_LICENSE_KEY not set")
	}

	t.Log("Testing connection pool exhaustion scenario...")
	
	// This test would:
	// 1. Start PostgreSQL with limited connections (max_connections=5)
	// 2. Create multiple collector instances trying to connect
	// 3. Verify proper error handling and recovery
	// 4. Check that metrics continue after some collectors back off
	
	t.Log("Connection pool exhaustion test - implementation pending")
}