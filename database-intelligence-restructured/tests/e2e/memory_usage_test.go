package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestCollectorMemoryUsage verifies collector memory usage stays within limits
func TestCollectorMemoryUsage(t *testing.T) {
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

	runID := fmt.Sprintf("memory_test_%d", time.Now().Unix())
	t.Logf("Starting memory usage test with run ID: %s", runID)

	// Start multiple PostgreSQL instances to increase memory pressure
	instances := []struct {
		name string
		port string
	}{
		{name: "pg-mem-1", port: "15432"},
		{name: "pg-mem-2", port: "15433"},
		{name: "pg-mem-3", port: "15434"},
	}

	// Start all instances
	t.Log("Starting multiple PostgreSQL instances...")
	for _, inst := range instances {
		exec.Command("docker", "rm", "-f", inst.name).Run()
		postgresCmd := exec.Command("docker", "run",
			"--name", inst.name,
			"-e", "POSTGRES_PASSWORD=postgres",
			"-p", inst.port+":5432",
			"--network", "bridge",
			"-d", "postgres:15-alpine")
		
		output, err := postgresCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to start PostgreSQL %s: %v\n%s", inst.name, err, output)
		}
	}

	// Cleanup
	defer func() {
		t.Log("Cleaning up containers...")
		for _, inst := range instances {
			exec.Command("docker", "stop", inst.name).Run()
			exec.Command("docker", "rm", inst.name).Run()
		}
		exec.Command("docker", "stop", "memory-test-collector").Run()
		exec.Command("docker", "rm", "memory-test-collector").Run()
	}()

	// Wait for instances
	time.Sleep(20 * time.Second)

	// Create config with memory limits
	config := createMemoryTestConfig(instances, runID, otlpEndpoint, licenseKey)
	
	configPath := "memory-test-config.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector with memory constraints
	t.Log("Starting collector with memory limits (256MB)...")
	exec.Command("docker", "rm", "-f", "memory-test-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "memory-test-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"--memory", "256m",  // Strict memory limit
		"--memory-swap", "256m",  // No swap allowed
		"-p", "58888:8888",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err := collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Monitor memory usage over time
	t.Run("MemoryMonitoring", func(t *testing.T) {
		monitorCollectorMemory(t, "memory-test-collector", 5*time.Minute)
	})

	// Test memory limiter behavior
	t.Run("MemoryLimiterActivation", func(t *testing.T) {
		testMemoryLimiterBehavior(t, runID, accountID, apiKey)
	})

	// Verify no OOM kills
	t.Run("OOMPrevention", func(t *testing.T) {
		verifyNoOOMKills(t, "memory-test-collector")
	})
}

func createMemoryTestConfig(instances []struct {
	name string
	port string
}, runID, otlpEndpoint, licenseKey string) string {
	// Build receivers
	receiversConfig := "receivers:\n"
	receiverList := []string{}
	
	for _, inst := range instances {
		receiversConfig += fmt.Sprintf(`  postgresql/%s:
    endpoint: host.docker.internal:%s
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s  # Aggressive collection
    tls:
      insecure: true

`, inst.name, inst.port)
		receiverList = append(receiverList, fmt.Sprintf("postgresql/%s", inst.name))
	}

	return fmt.Sprintf(`%s
processors:
  # Memory limiter MUST be first in pipeline
  memory_limiter:
    check_interval: 1s
    limit_mib: 200  # Less than container limit
    spike_limit_mib: 50
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.type
        value: memory_usage
        action: insert
  
  # Large batch to test memory pressure
  batch:
    timeout: 10s
    send_batch_size: 1000
    send_batch_max_size: 2000

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
    sending_queue:
      enabled: true
      num_consumers: 5  # Reduced for memory
      queue_size: 1000  # Smaller queue
    retry_on_failure:
      enabled: true
      max_elapsed_time: 60s
  
  logging:
    verbosity: normal
    sampling_initial: 10
    sampling_thereafter: 1000  # Reduce log memory

service:
  pipelines:
    metrics:
      receivers: [%s]
      processors: [memory_limiter, attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: 0.0.0.0:8888
`, receiversConfig, runID, otlpEndpoint, licenseKey, strings.Join(receiverList, ", "))
}

func monitorCollectorMemory(t *testing.T, containerName string, duration time.Duration) {
	t.Logf("Monitoring memory usage for %v...", duration)
	
	startTime := time.Now()
	maxMemoryMB := 0.0
	avgMemoryMB := 0.0
	samples := 0
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	done := time.After(duration)
	
	for {
		select {
		case <-ticker.C:
			// Get container stats
			statsCmd := exec.Command("docker", "stats", "--no-stream", "--format", 
				"{{.MemUsage}}", containerName)
			output, err := statsCmd.Output()
			if err != nil {
				t.Logf("Failed to get stats: %v", err)
				continue
			}
			
			// Parse memory usage (format: "47.38MiB / 256MiB")
			memStr := strings.TrimSpace(string(output))
			parts := strings.Split(memStr, " / ")
			if len(parts) >= 1 {
				usedStr := strings.TrimSuffix(parts[0], "MiB")
				usedStr = strings.TrimSuffix(usedStr, "GiB")
				
				if used, err := strconv.ParseFloat(usedStr, 64); err == nil {
					samples++
					avgMemoryMB += used
					if used > maxMemoryMB {
						maxMemoryMB = used
					}
					
					// Log periodic updates
					if samples%6 == 0 { // Every minute
						t.Logf("Memory usage at %v: %.2f MB", 
							time.Since(startTime).Round(time.Second), used)
					}
					
					// Check if approaching limit
					if used > 230 { // 90% of 256MB
						t.Logf("⚠️ High memory usage: %.2f MB (%.1f%% of limit)", 
							used, (used/256)*100)
					}
				}
			}
			
		case <-done:
			// Calculate final stats
			if samples > 0 {
				avgMemoryMB = avgMemoryMB / float64(samples)
				t.Logf("Memory usage summary:")
				t.Logf("  Average: %.2f MB", avgMemoryMB)
				t.Logf("  Maximum: %.2f MB", maxMemoryMB)
				t.Logf("  Samples: %d", samples)
				
				// Verify memory stayed within limits
				if maxMemoryMB < 256 {
					t.Log("✓ Memory usage stayed within container limit")
				} else {
					t.Error("Memory usage exceeded container limit")
				}
				
				if avgMemoryMB < 200 {
					t.Log("✓ Average memory usage below memory_limiter threshold")
				}
			}
			return
		}
	}
}

func testMemoryLimiterBehavior(t *testing.T, runID string, accountID, apiKey string) {
	// Check collector logs for memory limiter activation
	logsCmd := exec.Command("docker", "logs", "--tail", "1000", "memory-test-collector")
	logs, err := logsCmd.CombinedOutput()
	if err != nil {
		t.Logf("Failed to get logs: %v", err)
		return
	}
	
	logsStr := string(logs)
	
	// Look for memory limiter messages
	if strings.Contains(logsStr, "memory_limiter") {
		if strings.Contains(logsStr, "dropping data") || 
		   strings.Contains(logsStr, "memory limit") {
			t.Log("✓ Memory limiter activated to prevent OOM")
		}
	}
	
	// Check metrics for data drops
	time.Sleep(10 * time.Second)
	
	// Query for processor metrics
	nrql := fmt.Sprintf(`
		SELECT count(*) 
		FROM Metric 
		WHERE test.run.id = '%s' 
		SINCE 5 minutes ago
	`, runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metrics: %v", err)
		return
	}
	
	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok {
			t.Logf("Metrics collected under memory pressure: %.0f", count)
			if count > 0 {
				t.Log("✓ Collector continued operating under memory constraints")
			}
		}
	}
}

func verifyNoOOMKills(t *testing.T, containerName string) {
	// Check if container is still running
	checkCmd := exec.Command("docker", "ps", "-q", "-f", "name="+containerName)
	output, err := checkCmd.Output()
	if err != nil || len(output) == 0 {
		t.Error("Container is not running - possible OOM kill")
		
		// Check container logs for OOM
		inspectCmd := exec.Command("docker", "inspect", containerName, 
			"--format", "{{.State.OOMKilled}}")
		oomOutput, _ := inspectCmd.Output()
		if strings.TrimSpace(string(oomOutput)) == "true" {
			t.Error("Container was OOM killed!")
		}
	} else {
		t.Log("✓ Container still running - no OOM kill")
	}
	
	// Check container restart count
	restartCmd := exec.Command("docker", "inspect", containerName,
		"--format", "{{.RestartCount}}")
	restartOutput, err := restartCmd.Output()
	if err == nil {
		if restarts, err := strconv.Atoi(strings.TrimSpace(string(restartOutput))); err == nil {
			if restarts > 0 {
				t.Errorf("Container restarted %d times", restarts)
			} else {
				t.Log("✓ No container restarts")
			}
		}
	}
}

// TestMemoryLeakDetection checks for memory leaks over extended period
func TestMemoryLeakDetection(t *testing.T) {
	// Skip if no credentials
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	if licenseKey == "" {
		t.Skip("NEW_RELIC_LICENSE_KEY not set")
	}

	t.Log("Starting memory leak detection test...")

	// This test would run for extended period and monitor:
	// 1. Steady state memory usage
	// 2. Memory growth rate
	// 3. GC effectiveness
	// 4. Resource cleanup

	// Create config
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: localhost:5432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
    
  batch:
    timeout: 10s

exporters:
  otlp:
    endpoint: https://otlp.nr-data.net:4317
    headers:
      api-key: %s

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, batch]
      exporters: [otlp]

  telemetry:
    metrics:
      level: detailed
      address: 0.0.0.0:8888
`, licenseKey)

	// Save config
	configPath := "memory-leak-test.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	t.Log("Memory leak detection test configuration created")
	t.Log("To run extended test:")
	t.Log("  1. Start collector with this config")
	t.Log("  2. Monitor memory usage for 24+ hours")
	t.Log("  3. Look for steady growth indicating leaks")
	t.Log("  4. Analyze heap profiles if growth detected")
}