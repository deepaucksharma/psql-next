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

// TestMultiplePostgreSQLInstances tests collector with multiple PostgreSQL databases
func TestMultiplePostgreSQLInstances(t *testing.T) {
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

	runID := fmt.Sprintf("multi_postgres_%d", time.Now().Unix())
	t.Logf("Starting multiple PostgreSQL instances test with run ID: %s", runID)

	// Instance configurations
	instances := []struct {
		name     string
		port     string
		role     string
		workload string
	}{
		{name: "pg-primary", port: "5432", role: "primary", workload: "write-heavy"},
		{name: "pg-secondary", port: "5433", role: "secondary", workload: "read-heavy"},
		{name: "pg-analytics", port: "5434", role: "analytics", workload: "analytical"},
	}

	// Start all PostgreSQL instances
	t.Log("Starting multiple PostgreSQL instances...")
	for _, inst := range instances {
		startPostgreSQLInstance(t, inst.name, inst.port)
	}

	// Cleanup
	defer func() {
		t.Log("Cleaning up containers...")
		for _, inst := range instances {
			exec.Command("docker", "stop", inst.name).Run()
			exec.Command("docker", "rm", inst.name).Run()
		}
		exec.Command("docker", "stop", "multi-postgres-collector").Run()
		exec.Command("docker", "rm", "multi-postgres-collector").Run()
	}()

	// Wait for all instances to be ready
	t.Log("Waiting for PostgreSQL instances to be ready...")
	time.Sleep(20 * time.Second)

	// Create different schemas on each instance
	var wg sync.WaitGroup
	for _, inst := range instances {
		wg.Add(1)
		go func(instance struct {
			name     string
			port     string
			role     string
			workload string
		}) {
			defer wg.Done()
			setupInstanceSchema(t, instance.port, instance.role, instance.workload)
		}(inst)
	}
	wg.Wait()

	// Create collector config for multiple instances
	config := createMultiInstanceConfig(instances, runID, otlpEndpoint, licenseKey)
	
	// Write config
	configPath := "multi-postgres-config.yaml"
	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	defer os.Remove(configPath)

	// Get absolute path
	absConfigPath, _ := exec.Command("pwd").Output()
	absConfigPathStr := strings.TrimSpace(string(absConfigPath)) + "/" + configPath

	// Start collector
	t.Log("Starting collector for multiple instances...")
	exec.Command("docker", "rm", "-f", "multi-postgres-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "multi-postgres-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-p", "28888:8888",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err := collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Generate workload on each instance
	t.Log("Generating workload on each instance...")
	generateMultiInstanceWorkload(t, instances)

	// Wait for metrics collection
	t.Log("Waiting for metrics collection (60 seconds)...")
	time.Sleep(60 * time.Second)

	// Verify metrics from all instances
	t.Run("VerifyAllInstances", func(t *testing.T) {
		verifyMultiInstanceMetrics(t, runID, instances, accountID, apiKey)
	})

	// Check collector performance with multiple instances
	t.Run("CollectorPerformance", func(t *testing.T) {
		checkMultiInstanceCollectorStats(t)
	})

	// Verify instance-specific attributes
	t.Run("InstanceAttributes", func(t *testing.T) {
		verifyInstanceAttributes(t, runID, instances, accountID, apiKey)
	})
}

func startPostgreSQLInstance(t *testing.T, name, port string) {
	t.Logf("Starting PostgreSQL instance %s on port %s...", name, port)
	
	// Remove existing container if any
	exec.Command("docker", "rm", "-f", name).Run()
	
	postgresCmd := exec.Command("docker", "run",
		"--name", name,
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_DB=testdb",
		"-p", port+":5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL %s: %v\n%s", name, err, output)
	}
}

func setupInstanceSchema(t *testing.T, port, role, workload string) {
	// Connect to instance
	connStr := fmt.Sprintf("host=localhost port=%s user=postgres password=postgres dbname=testdb sslmode=disable", port)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL on port %s: %v", port, err)
	}
	defer db.Close()

	// Wait for connection
	for i := 0; i < 30; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	t.Logf("Setting up schema for %s instance (port %s)...", role, port)

	// Create schema based on role
	switch role {
	case "primary":
		createPrimarySchema(t, db)
	case "secondary":
		createSecondarySchema(t, db)
	case "analytics":
		createAnalyticsSchema(t, db)
	}
}

func createPrimarySchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS primary_data`,
		`CREATE TABLE IF NOT EXISTS primary_data.users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(100) UNIQUE,
			email VARCHAR(200),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS primary_data.orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER,
			total DECIMAL(10,2),
			status VARCHAR(20),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX idx_orders_user ON primary_data.orders(user_id)`,
		`CREATE INDEX idx_orders_status ON primary_data.orders(status)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Logf("Failed to execute query: %v", err)
		}
	}

	// Insert sample data
	for i := 0; i < 1000; i++ {
		db.Exec(`INSERT INTO primary_data.users (username, email) VALUES ($1, $2)`,
			fmt.Sprintf("user_%d", i), fmt.Sprintf("user%d@example.com", i))
	}
}

func createSecondarySchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS reporting`,
		`CREATE TABLE IF NOT EXISTS reporting.daily_stats (
			id SERIAL PRIMARY KEY,
			stat_date DATE,
			metric_name VARCHAR(100),
			metric_value DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS reporting.user_activity (
			id SERIAL PRIMARY KEY,
			user_id INTEGER,
			activity_type VARCHAR(50),
			activity_time TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX idx_daily_stats_date ON reporting.daily_stats(stat_date)`,
		`CREATE INDEX idx_user_activity_user ON reporting.user_activity(user_id)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Logf("Failed to execute query: %v", err)
		}
	}
}

func createAnalyticsSchema(t *testing.T, db *sql.DB) {
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS analytics`,
		`CREATE TABLE IF NOT EXISTS analytics.fact_sales (
			id SERIAL PRIMARY KEY,
			product_id INTEGER,
			customer_id INTEGER,
			quantity INTEGER,
			revenue DECIMAL(12,2),
			sale_date DATE,
			region VARCHAR(50)
		)`,
		`CREATE TABLE IF NOT EXISTS analytics.dim_products (
			product_id SERIAL PRIMARY KEY,
			product_name VARCHAR(200),
			category VARCHAR(100),
			price DECIMAL(10,2)
		)`,
		`CREATE TABLE IF NOT EXISTS analytics.dim_customers (
			customer_id SERIAL PRIMARY KEY,
			customer_name VARCHAR(200),
			segment VARCHAR(50),
			region VARCHAR(50)
		)`,
		`CREATE INDEX idx_fact_sales_date ON analytics.fact_sales(sale_date)`,
		`CREATE INDEX idx_fact_sales_product ON analytics.fact_sales(product_id)`,
		`CREATE INDEX idx_fact_sales_customer ON analytics.fact_sales(customer_id)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Logf("Failed to execute query: %v", err)
		}
	}
}

func createMultiInstanceConfig(instances []struct {
	name     string
	port     string
	role     string
	workload string
}, runID, otlpEndpoint, licenseKey string) string {
	// Build receivers section
	receiversConfig := "receivers:\n"
	for _, inst := range instances {
		receiversConfig += fmt.Sprintf(`  postgresql/%s:
    endpoint: host.docker.internal:%s
    transport: tcp
    username: postgres
    password: postgres
    databases:
      - testdb
    collection_interval: 10s
    tls:
      insecure: true
    resource_attributes:
      db.instance.id: %s
      db.instance.role: %s
      db.instance.port: %s

`, inst.name, inst.port, inst.name, inst.role, inst.port)
	}

	// Build service pipeline
	receiverList := []string{}
	for _, inst := range instances {
		receiverList = append(receiverList, fmt.Sprintf("postgresql/%s", inst.name))
	}

	return fmt.Sprintf(`%s
processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 1024
    spike_limit_mib: 256
    
  attributes:
    actions:
      - key: test.run.id
        value: %s
        action: insert
      - key: test.type
        value: multi_instance
        action: insert
      - key: environment
        value: e2e-test
        action: insert
  
  resource:
    attributes:
      - key: service.name
        value: multi-postgres-collector
        action: insert
      - key: deployment.type
        value: multi-instance
        action: insert
  
  batch:
    timeout: 10s
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
  
  logging:
    verbosity: normal
    sampling_initial: 10
    sampling_thereafter: 100

service:
  pipelines:
    metrics:
      receivers: [%s]
      processors: [memory_limiter, attributes, resource, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: 0.0.0.0:8888
`, receiversConfig, runID, otlpEndpoint, licenseKey, strings.Join(receiverList, ", "))
}

func generateMultiInstanceWorkload(t *testing.T, instances []struct {
	name     string
	port     string
	role     string
	workload string
}) {
	var wg sync.WaitGroup
	
	for _, inst := range instances {
		wg.Add(1)
		go func(instance struct {
			name     string
			port     string
			role     string
			workload string
		}) {
			defer wg.Done()
			
			connStr := fmt.Sprintf("host=localhost port=%s user=postgres password=postgres dbname=testdb sslmode=disable", instance.port)
			db, err := sql.Open("postgres", connStr)
			if err != nil {
				t.Logf("Failed to connect for workload generation: %v", err)
				return
			}
			defer db.Close()
			
			t.Logf("Generating %s workload on %s...", instance.workload, instance.name)
			
			// Generate workload based on type
			for i := 0; i < 30; i++ {
				switch instance.workload {
				case "write-heavy":
					db.Exec(`INSERT INTO primary_data.orders (user_id, total, status) VALUES ($1, $2, $3)`,
						i%100, float64(i)*10.5, "pending")
				case "read-heavy":
					db.Query(`SELECT COUNT(*) FROM reporting.daily_stats WHERE stat_date > CURRENT_DATE - INTERVAL '7 days'`)
					db.Query(`SELECT * FROM reporting.user_activity ORDER BY activity_time DESC LIMIT 10`)
				case "analytical":
					db.Query(`SELECT region, SUM(revenue) FROM analytics.fact_sales GROUP BY region`)
					db.Query(`SELECT p.category, COUNT(DISTINCT f.customer_id) 
						FROM analytics.fact_sales f 
						JOIN analytics.dim_products p ON f.product_id = p.product_id 
						GROUP BY p.category`)
				}
				time.Sleep(2 * time.Second)
			}
		}(inst)
	}
	
	wg.Wait()
}

func verifyMultiInstanceMetrics(t *testing.T, runID string, instances []struct {
	name     string
	port     string
	role     string
	workload string
}, accountID, apiKey string) {
	// Wait for metrics
	time.Sleep(10 * time.Second)

	// Query metrics for all instances
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' SINCE 5 minutes ago FACET db.instance.id", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query metrics: %v", err)
		return
	}

	// Verify we have metrics from all instances
	instanceCounts := make(map[string]int)
	for _, res := range result.Data.Actor.Account.NRQL.Results {
		if instanceID, ok := res["db.instance.id"].(string); ok {
			if count, ok := res["count"].(float64); ok {
				instanceCounts[instanceID] = int(count)
			}
		}
	}

	t.Log("Metrics by instance:")
	allInstancesFound := true
	for _, inst := range instances {
		if count, found := instanceCounts[inst.name]; found {
			t.Logf("  ✓ %s: %d metrics collected", inst.name, count)
		} else {
			t.Errorf("  ✗ %s: No metrics found", inst.name)
			allInstancesFound = false
		}
	}

	if allInstancesFound && len(instanceCounts) == len(instances) {
		t.Log("✓ All PostgreSQL instances are being monitored")
	}

	// Check total metrics volume
	totalMetrics := 0
	for _, count := range instanceCounts {
		totalMetrics += count
	}
	t.Logf("Total metrics across all instances: %d", totalMetrics)
}

func checkMultiInstanceCollectorStats(t *testing.T) {
	// Check collector resource usage
	statsCmd := exec.Command("docker", "stats", "--no-stream", "--format",
		"table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}", "multi-postgres-collector")
	stats, err := statsCmd.Output()
	if err != nil {
		t.Logf("Failed to get collector stats: %v", err)
		return
	}
	
	t.Logf("Collector resource usage with multiple instances:\n%s", stats)

	// Check collector metrics endpoint
	metricsCmd := exec.Command("curl", "-s", "http://localhost:28888/metrics")
	metrics, err := metricsCmd.Output()
	if err != nil {
		t.Logf("Failed to get collector metrics: %v", err)
		return
	}

	metricsStr := string(metrics)
	
	// Look for receiver metrics for each instance
	for _, inst := range []string{"pg-primary", "pg-secondary", "pg-analytics"} {
		if strings.Contains(metricsStr, fmt.Sprintf("receiver/postgresql/%s", inst)) {
			t.Logf("✓ Receiver metrics found for %s", inst)
		}
	}
}

func verifyInstanceAttributes(t *testing.T, runID string, instances []struct {
	name     string
	port     string
	role     string
	workload string
}, accountID, apiKey string) {
	// Query for role distribution
	nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id = '%s' SINCE 5 minutes ago FACET db.instance.role", runID)
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Errorf("Failed to query role metrics: %v", err)
		return
	}

	t.Log("Metrics by role:")
	for _, res := range result.Data.Actor.Account.NRQL.Results {
		if role, ok := res["db.instance.role"].(string); ok {
			if count, ok := res["count"].(float64); ok {
				t.Logf("  %s: %.0f metrics", role, count)
			}
		}
	}

	// Verify specific metrics per instance type
	for _, inst := range instances {
		nrql := fmt.Sprintf("SELECT uniques(metricName) FROM Metric WHERE test.run.id = '%s' AND db.instance.id = '%s' SINCE 5 minutes ago",
			runID, inst.name)
		
		result, err := queryNRDB(accountID, apiKey, nrql)
		if err != nil {
			continue
		}

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			if metricNames, ok := result.Data.Actor.Account.NRQL.Results[0]["uniques.metricName"].([]interface{}); ok {
				t.Logf("Instance %s (%s) collecting %d unique metric types", inst.name, inst.role, len(metricNames))
			}
		}
	}
}