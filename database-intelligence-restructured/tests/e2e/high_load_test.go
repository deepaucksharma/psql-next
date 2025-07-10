package e2e

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestHighLoadBehavior tests collector under high database query load
func TestHighLoadBehavior(t *testing.T) {
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

	runID := fmt.Sprintf("high_load_%d", time.Now().Unix())
	t.Logf("Starting high load test with run ID: %s", runID)

	// Start PostgreSQL with higher resources
	t.Log("Starting PostgreSQL container with increased resources...")
	postgresCmd := exec.Command("docker", "run",
		"--name", "e2e-highload-postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_SHARED_BUFFERS=256MB",
		"-e", "POSTGRES_MAX_CONNECTIONS=200",
		"-p", "45432:5432",
		"--memory", "1g",
		"--cpus", "2",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "already in use") {
			t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
		}
		exec.Command("docker", "rm", "-f", "e2e-highload-postgres").Run()
		postgresCmd = exec.Command("docker", "run",
			"--name", "e2e-highload-postgres",
			"-e", "POSTGRES_PASSWORD=postgres",
			"-e", "POSTGRES_SHARED_BUFFERS=256MB",
			"-e", "POSTGRES_MAX_CONNECTIONS=200",
			"-p", "45432:5432",
			"--memory", "1g",
			"--cpus", "2",
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
		exec.Command("docker", "stop", "e2e-highload-postgres").Run()
		exec.Command("docker", "rm", "e2e-highload-postgres").Run()
		exec.Command("docker", "stop", "e2e-highload-collector").Run()
		exec.Command("docker", "rm", "e2e-highload-collector").Run()
	}()

	// Wait for PostgreSQL
	time.Sleep(20 * time.Second)

	// Connect and create load test schema
	db, err := sql.Open("postgres", "host=localhost port=45432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Create load test data
	t.Log("Creating load test schema...")
	createLoadTestSchema(t, db)

	// Start collector with memory limiter
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:45432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 5s
    tls:
      insecure: true

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512
    spike_limit_mib: 128
    
  batch:
    timeout: 10s
    send_batch_size: 1000
    send_batch_max_size: 2000
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.type
        value: high_load
        action: insert
      - key: load.phase
        from_attribute: phase
        action: upsert

exporters:
  otlp:
    endpoint: %s
    headers:
      api-key: %s
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 5000
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 30s
  
  logging:
    loglevel: info
    sampling_initial: 100
    sampling_thereafter: 1000

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: 0.0.0.0:8888
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "highload-test-config.yaml"
	err = os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector with resource limits
	t.Log("Starting collector with resource limits...")
	exec.Command("docker", "rm", "-f", "e2e-highload-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-highload-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"--memory", "768m",
		"--cpus", "1",
		"-p", "18888:8888",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err = collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Test phases
	t.Run("Phase1_Baseline", func(t *testing.T) {
		t.Log("Phase 1: Baseline metrics (30 seconds)...")
		time.Sleep(30 * time.Second)
		
		// Check collector metrics
		checkCollectorMetrics(t, "baseline")
	})

	t.Run("Phase2_ModerateLoad", func(t *testing.T) {
		t.Log("Phase 2: Moderate load (10 concurrent queries)...")
		
		var wg sync.WaitGroup
		stopChan := make(chan bool)
		
		// Start load generators
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go generateDatabaseLoad(t, db, i, "moderate", stopChan, &wg)
		}
		
		// Run for 60 seconds
		time.Sleep(60 * time.Second)
		close(stopChan)
		wg.Wait()
		
		// Check metrics
		checkCollectorMetrics(t, "moderate")
		verifyLoadPhaseMetrics(t, runID, "moderate", accountID, apiKey)
	})

	t.Run("Phase3_HighLoad", func(t *testing.T) {
		t.Log("Phase 3: High load (50 concurrent queries)...")
		
		var wg sync.WaitGroup
		stopChan := make(chan bool)
		
		// Start more load generators
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go generateDatabaseLoad(t, db, i, "high", stopChan, &wg)
		}
		
		// Run for 60 seconds
		time.Sleep(60 * time.Second)
		close(stopChan)
		wg.Wait()
		
		// Check metrics and logs
		checkCollectorMetrics(t, "high")
		checkCollectorLogs(t)
		verifyLoadPhaseMetrics(t, runID, "high", accountID, apiKey)
	})

	t.Run("Phase4_Recovery", func(t *testing.T) {
		t.Log("Phase 4: Recovery phase (no load, 30 seconds)...")
		time.Sleep(30 * time.Second)
		
		// Verify metrics returned to normal
		checkCollectorMetrics(t, "recovery")
		
		// Final stats
		getCollectorStats(t)
	})
}

func createLoadTestSchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS load_test`,
		`CREATE TABLE IF NOT EXISTS load_test.users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(100) UNIQUE,
			email VARCHAR(200),
			created_at TIMESTAMP DEFAULT NOW(),
			last_login TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS load_test.transactions (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES load_test.users(id),
			amount DECIMAL(10,2),
			status VARCHAR(20),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS load_test.sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id INTEGER,
			data JSONB,
			expires_at TIMESTAMP
		)`,
		`CREATE INDEX idx_transactions_user ON load_test.transactions(user_id)`,
		`CREATE INDEX idx_transactions_created ON load_test.transactions(created_at)`,
		`CREATE INDEX idx_sessions_expires ON load_test.sessions(expires_at)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("Failed to create schema: %v", err)
		}
	}

	// Insert initial data
	t.Log("Inserting initial test data...")
	for i := 0; i < 10000; i++ {
		_, err := db.Exec(`INSERT INTO load_test.users (username, email) VALUES ($1, $2)`,
			fmt.Sprintf("user_%d", i),
			fmt.Sprintf("user%d@example.com", i))
		if err != nil && !strings.Contains(err.Error(), "duplicate key") {
			t.Logf("Failed to insert user: %v", err)
		}
	}
}

func generateDatabaseLoad(t *testing.T, db *sql.DB, workerID int, phase string, stop chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	
	queries := []string{
		// Simple queries
		`SELECT COUNT(*) FROM load_test.users`,
		`SELECT * FROM load_test.users ORDER BY RANDOM() LIMIT 10`,
		
		// Complex queries
		`SELECT u.username, COUNT(t.id) as txn_count, SUM(t.amount) as total 
		 FROM load_test.users u 
		 LEFT JOIN load_test.transactions t ON u.id = t.user_id 
		 GROUP BY u.username 
		 ORDER BY txn_count DESC 
		 LIMIT 20`,
		
		// Write operations
		`INSERT INTO load_test.transactions (user_id, amount, status) 
		 VALUES ($1, $2, $3)`,
		
		`UPDATE load_test.users SET last_login = NOW() WHERE id = $1`,
		
		// Heavy query
		`WITH user_stats AS (
			SELECT user_id, COUNT(*) as txn_count, AVG(amount) as avg_amount
			FROM load_test.transactions
			GROUP BY user_id
		)
		SELECT u.*, us.txn_count, us.avg_amount
		FROM load_test.users u
		JOIN user_stats us ON u.id = us.user_id
		WHERE us.txn_count > 5`,
	}
	
	queryCount := 0
	for {
		select {
		case <-stop:
			t.Logf("Worker %d (%s phase) executed %d queries", workerID, phase, queryCount)
			return
		default:
			// Execute random query
			queryIdx := queryCount % len(queries)
			query := queries[queryIdx]
			
			if strings.Contains(query, "$1") {
				// Parameterized query
				userID := (workerID*100 + queryCount) % 10000
				if strings.Contains(query, "$3") {
					_, err := db.Exec(query, userID, float64(queryCount)*1.5, "completed")
					if err != nil && queryCount == 0 {
						t.Logf("Worker %d query error: %v", workerID, err)
					}
				} else {
					_, err := db.Exec(query, userID)
					if err != nil && queryCount == 0 {
						t.Logf("Worker %d query error: %v", workerID, err)
					}
				}
			} else {
				// Non-parameterized query
				rows, err := db.Query(query)
				if err != nil && queryCount == 0 {
					t.Logf("Worker %d query error: %v", workerID, err)
				} else if rows != nil {
					rows.Close()
				}
			}
			
			queryCount++
			
			// Vary the pace based on phase
			if phase == "moderate" {
				time.Sleep(100 * time.Millisecond)
			} else if phase == "high" {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
}

func checkCollectorMetrics(t *testing.T, phase string) {
	// Check collector's internal metrics endpoint
	resp, err := exec.Command("curl", "-s", "http://localhost:18888/metrics").Output()
	if err != nil {
		t.Logf("Failed to get collector metrics: %v", err)
		return
	}
	
	metrics := string(resp)
	
	// Look for key metrics
	if strings.Contains(metrics, "otelcol_processor_batch_batch_send_size") {
		t.Logf("✓ %s phase: Batch processor metrics present", phase)
	}
	
	if strings.Contains(metrics, "otelcol_processor_memory_limiter") {
		t.Logf("✓ %s phase: Memory limiter active", phase)
	}
	
	if strings.Contains(metrics, "otelcol_exporter_queue_size") {
		t.Logf("✓ %s phase: Exporter queue metrics present", phase)
	}
}

func checkCollectorLogs(t *testing.T) {
	logsCmd := exec.Command("docker", "logs", "--tail", "100", "e2e-highload-collector")
	logs, _ := logsCmd.CombinedOutput()
	logsStr := string(logs)
	
	// Check for memory pressure
	if strings.Contains(logsStr, "memory_limiter") && strings.Contains(logsStr, "dropping data") {
		t.Log("⚠️  Memory limiter triggered - some data may be dropped")
	}
	
	// Check for queue saturation
	if strings.Contains(logsStr, "queue is full") {
		t.Log("⚠️  Export queue saturated")
	}
}

func getCollectorStats(t *testing.T) {
	// Get container stats
	statsCmd := exec.Command("docker", "stats", "--no-stream", "--format", 
		"table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}", "e2e-highload-collector")
	stats, _ := statsCmd.Output()
	t.Logf("Container stats:\n%s", stats)
}

func verifyLoadPhaseMetrics(t *testing.T, runID, phase string, accountID, apiKey string) {
	// Wait for metrics
	time.Sleep(10 * time.Second)
	
	// Query metrics volume
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' AND load.phase = '%s' SINCE 5 minutes ago", runID, phase)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query %s phase metrics: %v", phase, err)
		return
	}
	
	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok {
			t.Logf("✓ %s phase: %.0f metrics collected", phase, count)
		}
	}
}