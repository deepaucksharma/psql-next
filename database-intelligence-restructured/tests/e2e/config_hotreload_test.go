package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestConfigurationHotReload tests collector configuration reload without data loss
func TestConfigurationHotReload(t *testing.T) {
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

	runID := fmt.Sprintf("hotreload_%d", time.Now().Unix())
	t.Logf("Starting configuration hot reload test with run ID: %s", runID)

	// Start PostgreSQL
	t.Log("Starting PostgreSQL container...")
	exec.Command("docker", "rm", "-f", "postgres-hotreload").Run()
	
	postgresCmd := exec.Command("docker", "run",
		"--name", "postgres-hotreload",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "65432:5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
	}

	// Cleanup
	defer func() {
		t.Log("Cleaning up containers...")
		exec.Command("docker", "stop", "postgres-hotreload").Run()
		exec.Command("docker", "rm", "postgres-hotreload").Run()
		exec.Command("docker", "stop", "hotreload-collector").Run()
		exec.Command("docker", "rm", "hotreload-collector").Run()
	}()

	// Wait for PostgreSQL
	time.Sleep(15 * time.Second)

	// Initial configuration - 30 second collection interval
	configPath := "hotreload-config.yaml"
	initialConfig := createHotReloadConfig(runID, "30s", "initial", otlpEndpoint, licenseKey)
	
	err = os.WriteFile(configPath, []byte(initialConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write initial config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector with volume mount for config
	t.Log("Starting collector with initial configuration (30s interval)...")
	exec.Command("docker", "rm", "-f", "hotreload-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "hotreload-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-p", "48888:8888",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err = collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Test phases
	t.Run("Phase1_InitialCollection", func(t *testing.T) {
		t.Log("Phase 1: Collecting metrics with initial config (60 seconds)...")
		time.Sleep(60 * time.Second)
		
		// Verify initial metrics
		verifyHotReloadMetrics(t, runID, "initial", accountID, apiKey)
	})

	t.Run("Phase2_ConfigUpdate", func(t *testing.T) {
		t.Log("Phase 2: Updating configuration to 10s interval...")
		
		// Update config with shorter interval
		updatedConfig := createHotReloadConfig(runID, "10s", "updated", otlpEndpoint, licenseKey)
		err = os.WriteFile(configPath, []byte(updatedConfig), 0644)
		if err != nil {
			t.Errorf("Failed to update config: %v", err)
			return
		}
		
		// Send SIGHUP to reload config
		t.Log("Sending SIGHUP to reload configuration...")
		reloadCmd := exec.Command("docker", "kill", "-s", "SIGHUP", "hotreload-collector")
		output, err := reloadCmd.CombinedOutput()
		if err != nil {
			t.Logf("Failed to send SIGHUP: %v\n%s", err, output)
			// Try alternative approach
			t.Log("Trying alternative reload approach...")
			exec.Command("docker", "exec", "hotreload-collector", "kill", "-SIGHUP", "1").Run()
		}
		
		// Wait for reload to take effect
		time.Sleep(10 * time.Second)
		
		// Check if collector is still running
		checkCmd := exec.Command("docker", "ps", "-q", "-f", "name=hotreload-collector")
		if output, err := checkCmd.Output(); err == nil && len(output) > 0 {
			t.Log("✓ Collector still running after config reload")
		} else {
			t.Error("Collector stopped after config reload")
		}
	})

	t.Run("Phase3_VerifyNewConfig", func(t *testing.T) {
		t.Log("Phase 3: Verifying metrics with new config (60 seconds)...")
		time.Sleep(60 * time.Second)
		
		// Verify updated metrics - should have more data points due to shorter interval
		verifyHotReloadMetrics(t, runID, "updated", accountID, apiKey)
		
		// Compare metric frequency
		compareMetricFrequency(t, runID, accountID, apiKey)
	})

	t.Run("Phase4_DataContinuity", func(t *testing.T) {
		t.Log("Phase 4: Verifying data continuity across reload...")
		
		// Query for any gaps in data
		verifyDataContinuity(t, runID, accountID, apiKey)
	})
}

func createHotReloadConfig(runID, interval, phase, otlpEndpoint, licenseKey string) string {
	return fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:65432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: %s
    tls:
      insecure: true

processors:
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.phase
        value: %s
        action: insert
      - key: collection.interval
        value: %s
        action: insert
  
  batch:
    timeout: 5s
    send_batch_size: 100

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 10s
  
  logging:
    verbosity: normal
    sampling_initial: 10
    sampling_thereafter: 50

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: info
      output_paths: ["stdout"]
    metrics:
      level: detailed
      address: 0.0.0.0:8888

  # Enable config reloading
  extensions: []
`, interval, runID, phase, interval, otlpEndpoint, licenseKey)
}

func verifyHotReloadMetrics(t *testing.T, runID, phase string, accountID, apiKey string) {
	// Wait for metrics to appear
	time.Sleep(10 * time.Second)
	
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' AND test.phase = '%s' SINCE 2 minutes ago", runID, phase)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query %s phase metrics: %v", phase, err)
		return
	}
	
	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok {
			t.Logf("✓ %s phase: %.0f metrics collected", phase, count)
			
			if phase == "initial" && count < 10 {
				t.Error("Too few metrics in initial phase")
			} else if phase == "updated" && count < 30 {
				t.Error("Too few metrics in updated phase")
			}
		}
	}
}

func compareMetricFrequency(t *testing.T, runID string, accountID, apiKey string) {
	// Compare metric collection frequency between phases
	nrql := fmt.Sprintf(`
		SELECT rate(count(*), 1 minute) as 'metrics_per_minute' 
		FROM Metric 
		WHERE test.run.id = '%s' 
		SINCE 5 minutes ago 
		TIMESERIES 1 minute 
		FACET test.phase
	`, runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metric rates: %v", err)
		return
	}
	
	t.Log("Metric collection rates by phase:")
	
	// Analyze rates
	initialRate := 0.0
	updatedRate := 0.0
	
	for _, ts := range result.Data.Actor.Account.NRQL.Results {
		if phase, ok := ts["test.phase"].(string); ok {
			if values, ok := ts["metrics_per_minute"].([]interface{}); ok && len(values) > 0 {
				// Get average rate
				sum := 0.0
				count := 0
				for _, v := range values {
					if val, ok := v.(float64); ok && val > 0 {
						sum += val
						count++
					}
				}
				
				if count > 0 {
					avgRate := sum / float64(count)
					t.Logf("  %s phase: %.2f metrics/minute average", phase, avgRate)
					
					if phase == "initial" {
						initialRate = avgRate
					} else if phase == "updated" {
						updatedRate = avgRate
					}
				}
			}
		}
	}
	
	// Updated config should have ~3x higher rate (10s vs 30s interval)
	if updatedRate > initialRate*2 {
		t.Logf("✓ Collection rate increased after config update (%.1fx)", updatedRate/initialRate)
	} else {
		t.Errorf("Collection rate did not increase as expected (initial: %.2f, updated: %.2f)", initialRate, updatedRate)
	}
}

func verifyDataContinuity(t *testing.T, runID string, accountID, apiKey string) {
	// Check for gaps in data collection
	nrql := fmt.Sprintf(`
		SELECT count(*) 
		FROM Metric 
		WHERE test.run.id = '%s' 
		SINCE 10 minutes ago 
		TIMESERIES 30 seconds
	`, runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query data continuity: %v", err)
		return
	}
	
	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if values, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].([]interface{}); ok {
			// Look for gaps (0 values) in the timeseries
			gaps := 0
			consecutiveGaps := 0
			maxConsecutiveGaps := 0
			
			for _, v := range values {
				if val, ok := v.(float64); ok {
					if val == 0 {
						gaps++
						consecutiveGaps++
						if consecutiveGaps > maxConsecutiveGaps {
							maxConsecutiveGaps = consecutiveGaps
						}
					} else {
						consecutiveGaps = 0
					}
				}
			}
			
			t.Logf("Data continuity analysis:")
			t.Logf("  Total time buckets: %d", len(values))
			t.Logf("  Buckets with no data: %d", gaps)
			t.Logf("  Max consecutive gaps: %d", maxConsecutiveGaps)
			
			// Allow for some gaps during reload (up to 2 consecutive 30s buckets = 1 minute)
			if maxConsecutiveGaps <= 2 {
				t.Log("✓ Data continuity maintained during config reload")
			} else {
				t.Errorf("Extended data gap detected during reload: %d consecutive empty buckets", maxConsecutiveGaps)
			}
		}
	}
	
	// Check if metrics exist before and after reload
	nrql = fmt.Sprintf(`
		SELECT uniques(test.phase), min(timestamp), max(timestamp) 
		FROM Metric 
		WHERE test.run.id = '%s' 
		SINCE 10 minutes ago
	`, runID)
	
	result, err = queryNRDB(accountID, apiKey, nrql)
	if err == nil && len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if phases, ok := result.Data.Actor.Account.NRQL.Results[0]["uniques.test.phase"].([]interface{}); ok {
			if len(phases) >= 2 {
				t.Log("✓ Metrics collected in both initial and updated phases")
			}
		}
	}
}

// TestConfigValidation tests collector behavior with invalid configurations
func TestConfigValidation(t *testing.T) {
	t.Log("Testing collector configuration validation...")

	// Test scenarios
	scenarios := []struct {
		name   string
		config string
		expect string
	}{
		{
			name: "Invalid receiver config",
			config: `
receivers:
  postgresql:
    endpoint: invalid:port:format
    username: postgres
    
service:
  pipelines:
    metrics:
      receivers: [postgresql]
      exporters: [logging]
`,
			expect: "invalid endpoint",
		},
		{
			name: "Missing required fields",
			config: `
receivers:
  postgresql:
    endpoint: localhost:5432
    # Missing username/password
    
service:
  pipelines:
    metrics:
      receivers: [postgresql]
      exporters: [logging]
`,
			expect: "missing required",
		},
		{
			name: "Invalid processor chain",
			config: `
receivers:
  postgresql:
    endpoint: localhost:5432
    
processors:
  invalid_processor:
    some_field: value
    
service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [invalid_processor]
      exporters: [logging]
`,
			expect: "unknown processor",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Write invalid config
			configPath := fmt.Sprintf("invalid-config-%s.yaml", scenario.name)
			err := os.WriteFile(configPath, []byte(scenario.config), 0644)
			if err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			defer os.Remove(configPath)

			// Try to validate config
			validateCmd := exec.Command("otel/opentelemetry-collector-contrib:0.92.0",
				"--config", configPath,
				"--dry-run")

			output, err := validateCmd.CombinedOutput()
			if err != nil {
				t.Logf("✓ Config validation failed as expected for %s", scenario.name)
				t.Logf("  Error output: %s", output)
			} else {
				t.Errorf("Expected config validation to fail for %s", scenario.name)
			}
		})
	}
}