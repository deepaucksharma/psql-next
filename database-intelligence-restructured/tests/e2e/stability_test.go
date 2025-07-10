package e2e

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestLongRunningStability runs collector for extended period to verify stability
func TestLongRunningStability(t *testing.T) {
	// Skip if no credentials or not explicitly requested
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	otlpEndpoint := os.Getenv("NEW_RELIC_OTLP_ENDPOINT")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	
	// Check for explicit long test flag
	runLongTests := os.Getenv("RUN_LONG_TESTS")
	if runLongTests != "true" {
		t.Skip("Long-running tests disabled. Set RUN_LONG_TESTS=true to enable")
	}
	
	if licenseKey == "" || apiKey == "" || accountID == "" {
		t.Skip("Required credentials not set")
	}
	
	if otlpEndpoint == "" {
		otlpEndpoint = "https://otlp.nr-data.net:4317"
	}

	// Test duration (default 2 hours, configurable)
	testDuration := 2 * time.Hour
	if customDuration := os.Getenv("STABILITY_TEST_DURATION"); customDuration != "" {
		if d, err := time.ParseDuration(customDuration); err == nil {
			testDuration = d
		}
	}

	runID := fmt.Sprintf("stability_%d", time.Now().Unix())
	t.Logf("Starting %v stability test with run ID: %s", testDuration, runID)

	// Start PostgreSQL
	t.Log("Starting PostgreSQL for stability test...")
	exec.Command("docker", "rm", "-f", "postgres-stability").Run()
	
	postgresCmd := exec.Command("docker", "run",
		"--name", "postgres-stability",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_MAX_CONNECTIONS=200",
		"-p", "25432:5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
	}

	// Cleanup
	defer func() {
		t.Log("Cleaning up containers...")
		exec.Command("docker", "stop", "postgres-stability").Run()
		exec.Command("docker", "rm", "postgres-stability").Run()
		exec.Command("docker", "stop", "stability-collector").Run()
		exec.Command("docker", "rm", "stability-collector").Run()
	}()

	// Wait for PostgreSQL
	time.Sleep(20 * time.Second)

	// Setup test database with realistic schema
	setupStabilityTestDB(t)

	// Create collector config
	config := createStabilityTestConfig(runID, otlpEndpoint, licenseKey)
	
	configPath := "stability-test-config.yaml"
	err = os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector for stability test...")
	exec.Command("docker", "rm", "-f", "stability-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "stability-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"--memory", "512m",
		"--cpus", "1",
		"-p", "68888:8888",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err = collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Start monitoring
	ctx := &StabilityTestContext{
		RunID:        runID,
		StartTime:    time.Now(),
		Duration:     testDuration,
		AccountID:    accountID,
		APIKey:       apiKey,
		CheckpointInterval: 15 * time.Minute,
	}

	// Run stability test
	runStabilityTest(t, ctx)
}

type StabilityTestContext struct {
	RunID              string
	StartTime          time.Time
	Duration           time.Duration
	AccountID          string
	APIKey             string
	CheckpointInterval time.Duration
	
	mu              sync.Mutex
	metricsReceived int64
	errors          []string
	checkpoints     []StabilityCheckpoint
}

type StabilityCheckpoint struct {
	Time            time.Time
	MetricsReceived int64
	MemoryUsageMB   float64
	CPUPercent      float64
	Errors          int
	Status          string
}

func setupStabilityTestDB(t *testing.T) {
	connStr := "host=localhost port=25432 user=postgres password=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Wait for connection
	for i := 0; i < 30; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	t.Log("Creating realistic database schema...")
	
	queries := []string{
		// E-commerce schema
		`CREATE SCHEMA IF NOT EXISTS ecommerce`,
		`CREATE TABLE IF NOT EXISTS ecommerce.customers (
			id SERIAL PRIMARY KEY,
			email VARCHAR(200) UNIQUE,
			name VARCHAR(200),
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS ecommerce.products (
			id SERIAL PRIMARY KEY,
			sku VARCHAR(100) UNIQUE,
			name VARCHAR(200),
			price DECIMAL(10,2),
			stock INTEGER,
			category VARCHAR(100),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS ecommerce.orders (
			id SERIAL PRIMARY KEY,
			customer_id INTEGER REFERENCES ecommerce.customers(id),
			order_date TIMESTAMP DEFAULT NOW(),
			status VARCHAR(50),
			total DECIMAL(10,2)
		)`,
		`CREATE TABLE IF NOT EXISTS ecommerce.order_items (
			id SERIAL PRIMARY KEY,
			order_id INTEGER REFERENCES ecommerce.orders(id),
			product_id INTEGER REFERENCES ecommerce.products(id),
			quantity INTEGER,
			price DECIMAL(10,2)
		)`,
		// Indexes
		`CREATE INDEX idx_orders_customer ON ecommerce.orders(customer_id)`,
		`CREATE INDEX idx_orders_date ON ecommerce.orders(order_date)`,
		`CREATE INDEX idx_products_category ON ecommerce.products(category)`,
		`CREATE INDEX idx_order_items_order ON ecommerce.order_items(order_id)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Logf("Failed to execute query: %v", err)
		}
	}

	// Insert initial data
	t.Log("Populating with test data...")
	
	// Customers
	for i := 0; i < 10000; i++ {
		db.Exec(`INSERT INTO ecommerce.customers (email, name) VALUES ($1, $2)`,
			fmt.Sprintf("customer%d@example.com", i),
			fmt.Sprintf("Customer %d", i))
	}
	
	// Products
	categories := []string{"Electronics", "Books", "Clothing", "Home", "Sports"}
	for i := 0; i < 5000; i++ {
		db.Exec(`INSERT INTO ecommerce.products (sku, name, price, stock, category) VALUES ($1, $2, $3, $4, $5)`,
			fmt.Sprintf("SKU-%05d", i),
			fmt.Sprintf("Product %d", i),
			float64(i%100)+9.99,
			100+i%500,
			categories[i%len(categories)])
	}
}

func createStabilityTestConfig(runID, otlpEndpoint, licenseKey string) string {
	return fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:25432
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - postgres
    collection_interval: 30s
    tls:
      insecure: true

processors:
  memory_limiter:
    check_interval: 5s
    limit_mib: 400
    spike_limit_mib: 100
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.type
        value: stability
        action: insert
      - key: test.phase
        from_attribute: phase
        action: upsert
  
  batch:
    timeout: 30s
    send_batch_size: 500
    send_batch_max_size: 1000

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
      initial_interval: 5s
      max_interval: 300s
      max_elapsed_time: 1800s
  
  logging:
    verbosity: normal
    sampling_initial: 10
    sampling_thereafter: 10000

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [memory_limiter, attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: info
      output_paths: ["stdout", "/tmp/collector.log"]
    metrics:
      level: detailed
      address: 0.0.0.0:8888
`, runID, otlpEndpoint, licenseKey)
}

func runStabilityTest(t *testing.T, ctx *StabilityTestContext) {
	// Start workload generator
	workloadDone := make(chan bool)
	go generateStabilityWorkload(t, ctx, workloadDone)
	defer close(workloadDone)

	// Start monitoring
	monitorDone := make(chan bool)
	go monitorStability(t, ctx, monitorDone)
	defer close(monitorDone)

	// Run for specified duration
	ticker := time.NewTicker(ctx.CheckpointInterval)
	defer ticker.Stop()
	
	testEnd := time.After(ctx.Duration)
	
	for {
		select {
		case <-ticker.C:
			// Take checkpoint
			checkpoint := takeStabilityCheckpoint(t, ctx)
			ctx.mu.Lock()
			ctx.checkpoints = append(ctx.checkpoints, checkpoint)
			ctx.mu.Unlock()
			
			t.Logf("Checkpoint at %v: %s", 
				time.Since(ctx.StartTime).Round(time.Second), 
				checkpoint.Status)
			
		case <-testEnd:
			// Test completed
			t.Log("Stability test completed")
			generateStabilityReport(t, ctx)
			return
		}
	}
}

func generateStabilityWorkload(t *testing.T, ctx *StabilityTestContext, done chan bool) {
	connStr := "host=localhost port=25432 user=postgres password=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Logf("Failed to connect for workload: %v", err)
		return
	}
	defer db.Close()

	// Simulate realistic workload patterns
	for {
		select {
		case <-done:
			return
		default:
			// Vary workload throughout the day
			hour := time.Now().Hour()
			
			// Simulate business hours vs off-hours
			var workloadMultiplier int
			if hour >= 9 && hour <= 17 {
				workloadMultiplier = 10 // High load
			} else if hour >= 18 && hour <= 22 {
				workloadMultiplier = 5  // Medium load
			} else {
				workloadMultiplier = 1  // Low load
			}
			
			// Generate queries
			for i := 0; i < workloadMultiplier; i++ {
				go func() {
					// Random query types
					switch i % 5 {
					case 0:
						// Customer lookup
						db.Query(`SELECT * FROM ecommerce.customers WHERE id = $1`, i%10000)
					case 1:
						// Product search
						db.Query(`SELECT * FROM ecommerce.products WHERE category = $1 LIMIT 20`, 
							[]string{"Electronics", "Books", "Clothing"}[i%3])
					case 2:
						// Order history
						db.Query(`SELECT o.*, oi.* FROM ecommerce.orders o 
							JOIN ecommerce.order_items oi ON o.id = oi.order_id 
							WHERE o.customer_id = $1 
							ORDER BY o.order_date DESC LIMIT 10`, i%10000)
					case 3:
						// Analytics query
						db.Query(`SELECT category, COUNT(*), AVG(price) 
							FROM ecommerce.products 
							GROUP BY category`)
					case 4:
						// Insert new order
						db.Exec(`INSERT INTO ecommerce.orders (customer_id, status, total) 
							VALUES ($1, 'pending', $2)`, i%10000, float64(i)*10.5)
					}
				}()
			}
			
			// Variable sleep based on load
			time.Sleep(time.Duration(1000/workloadMultiplier) * time.Millisecond)
		}
	}
}

func monitorStability(t *testing.T, ctx *StabilityTestContext, done chan bool) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			// Query recent metrics
			nrql := fmt.Sprintf(`
				SELECT count(*) 
				FROM Metric 
				WHERE test.run.id = '%s' 
				SINCE 2 minutes ago
			`, ctx.RunID)
			
			result, err := queryNRDB(ctx.AccountID, ctx.APIKey, nrql)
			if err != nil {
				ctx.mu.Lock()
				ctx.errors = append(ctx.errors, fmt.Sprintf("Query error: %v", err))
				ctx.mu.Unlock()
				continue
			}
			
			if len(result.Data.Actor.Account.NRQL.Results) > 0 {
				if count, ok := result.Data.Actor.Account.NRQL.Results[0]["count"].(float64); ok {
					ctx.mu.Lock()
					ctx.metricsReceived += int64(count)
					ctx.mu.Unlock()
				}
			}
		}
	}
}

func takeStabilityCheckpoint(t *testing.T, ctx *StabilityTestContext) StabilityCheckpoint {
	checkpoint := StabilityCheckpoint{
		Time: time.Now(),
	}
	
	// Get container stats
	statsCmd := exec.Command("docker", "stats", "--no-stream", "--format", 
		"{{json .}}", "stability-collector")
	output, err := statsCmd.Output()
	if err == nil {
		var stats map[string]interface{}
		if err := json.Unmarshal(output, &stats); err == nil {
			// Parse memory usage
			if memStr, ok := stats["MemUsage"].(string); ok {
				parts := strings.Split(memStr, " / ")
				if len(parts) > 0 {
					usedStr := strings.TrimSuffix(parts[0], "MiB")
					if used, err := strconv.ParseFloat(usedStr, 64); err == nil {
						checkpoint.MemoryUsageMB = used
					}
				}
			}
			
			// Parse CPU percentage
			if cpuStr, ok := stats["CPUPerc"].(string); ok {
				cpuStr = strings.TrimSuffix(cpuStr, "%")
				if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
					checkpoint.CPUPercent = cpu
				}
			}
		}
	}
	
	// Get metrics count
	ctx.mu.Lock()
	checkpoint.MetricsReceived = ctx.metricsReceived
	checkpoint.Errors = len(ctx.errors)
	ctx.mu.Unlock()
	
	// Determine status
	if checkpoint.Errors > 0 {
		checkpoint.Status = fmt.Sprintf("Degraded - %d errors", checkpoint.Errors)
	} else if checkpoint.MemoryUsageMB > 400 {
		checkpoint.Status = "Warning - High memory usage"
	} else {
		checkpoint.Status = "Healthy"
	}
	
	return checkpoint
}

func generateStabilityReport(t *testing.T, ctx *StabilityTestContext) {
	t.Log(strings.Repeat("=", 60))
	t.Log("STABILITY TEST REPORT")
	t.Log(strings.Repeat("=", 60))
	
	t.Logf("Run ID: %s", ctx.RunID)
	t.Logf("Duration: %v", ctx.Duration)
	t.Logf("Start Time: %s", ctx.StartTime.Format(time.RFC3339))
	t.Logf("End Time: %s", time.Now().Format(time.RFC3339))
	
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	
	t.Logf("\nMetrics Summary:")
	t.Logf("  Total Metrics Received: %d", ctx.metricsReceived)
	t.Logf("  Average Rate: %.2f metrics/min", float64(ctx.metricsReceived)/ctx.Duration.Minutes())
	t.Logf("  Total Errors: %d", len(ctx.errors))
	
	if len(ctx.checkpoints) > 0 {
		t.Logf("\nResource Usage:")
		
		var maxMem, avgMem, maxCPU, avgCPU float64
		for _, cp := range ctx.checkpoints {
			if cp.MemoryUsageMB > maxMem {
				maxMem = cp.MemoryUsageMB
			}
			avgMem += cp.MemoryUsageMB
			
			if cp.CPUPercent > maxCPU {
				maxCPU = cp.CPUPercent
			}
			avgCPU += cp.CPUPercent
		}
		
		avgMem = avgMem / float64(len(ctx.checkpoints))
		avgCPU = avgCPU / float64(len(ctx.checkpoints))
		
		t.Logf("  Memory - Avg: %.2f MB, Max: %.2f MB", avgMem, maxMem)
		t.Logf("  CPU - Avg: %.2f%%, Max: %.2f%%", avgCPU, maxCPU)
		
		// Check for memory growth
		if len(ctx.checkpoints) > 2 {
			firstMem := ctx.checkpoints[0].MemoryUsageMB
			lastMem := ctx.checkpoints[len(ctx.checkpoints)-1].MemoryUsageMB
			growth := lastMem - firstMem
			
			t.Logf("  Memory Growth: %.2f MB (%.1f%%)", growth, (growth/firstMem)*100)
			
			if growth > firstMem*0.5 {
				t.Log("  ⚠️ WARNING: Significant memory growth detected")
			}
		}
	}
	
	// Error summary
	if len(ctx.errors) > 0 {
		t.Logf("\nErrors Encountered:")
		errorTypes := make(map[string]int)
		for _, err := range ctx.errors {
			// Categorize errors
			if strings.Contains(err, "connection") {
				errorTypes["Connection"]++
			} else if strings.Contains(err, "timeout") {
				errorTypes["Timeout"]++
			} else {
				errorTypes["Other"]++
			}
		}
		
		for errType, count := range errorTypes {
			t.Logf("  %s: %d", errType, count)
		}
	}
	
	// Overall assessment
	t.Logf("\nOverall Assessment:")
	if len(ctx.errors) == 0 && ctx.metricsReceived > 0 {
		t.Log("  ✅ PASSED - Collector remained stable throughout test")
	} else if float64(len(ctx.errors))/float64(ctx.metricsReceived) < 0.01 {
		t.Log("  ⚠️ PASSED WITH WARNINGS - Minor issues detected (<1% error rate)")
	} else {
		t.Log("  ❌ FAILED - Significant stability issues detected")
	}
	
	t.Log(strings.Repeat("=", 60))
}