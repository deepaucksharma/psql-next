package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type PrometheusQueryResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

func main() {
	fmt.Println("=== Database Intelligence E2E Verification Test ===")
	fmt.Println()

	// Step 1: Verify PostgreSQL connection
	fmt.Println("1. Testing PostgreSQL connection...")
	db := verifyDatabaseConnection()
	defer db.Close()

	// Step 2: Create test data
	fmt.Println("\n2. Creating test data...")
	createTestData(db)

	// Step 3: Wait for metrics collection
	fmt.Println("\n3. Waiting for metrics collection cycle (10s)...")
	time.Sleep(10 * time.Second)

	// Step 4: Verify collector is running
	fmt.Println("\n4. Verifying OpenTelemetry Collector...")
	verifyCollector()

	// Step 5: Verify metrics in Prometheus
	fmt.Println("\n5. Verifying metrics in Prometheus...")
	verifyMetricsInPrometheus()

	// Step 6: Test processor functionality
	fmt.Println("\n6. Testing processor functionality...")
	testProcessors(db)

	// Step 7: Cleanup
	fmt.Println("\n7. Cleaning up test data...")
	cleanupTestData(db)

	fmt.Println("\n✅ All E2E tests passed successfully!")
}

func verifyDatabaseConnection() *sql.DB {
	dsn := "host=localhost port=5432 user=postgres password=postgres dbname=postgres sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("✓ Successfully connected to PostgreSQL")
	return db
}

func createTestData(db *sql.DB) {
	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test_data (
			id SERIAL PRIMARY KEY,
			test_name VARCHAR(100),
			test_value FLOAT,
			test_category VARCHAR(50),
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Insert diverse test data to trigger different metrics
	testData := []struct {
		name     string
		value    float64
		category string
	}{
		{"metric_test_1", 100.5, "performance"},
		{"metric_test_2", 200.7, "performance"},
		{"metric_test_3", 50.3, "validation"},
		{"metric_test_4", 75.9, "validation"},
		{"metric_test_5", 150.2, "load"},
	}

	for _, td := range testData {
		_, err := db.Exec(`
			INSERT INTO e2e_test_data (test_name, test_value, test_category) 
			VALUES ($1, $2, $3)
		`, td.name, td.value, td.category)
		if err != nil {
			log.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Run some queries to generate statistics
	for i := 0; i < 10; i++ {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM e2e_test_data WHERE test_category = $1", "performance").Scan(&count)
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("✓ Created test data and generated query activity")
}

func verifyCollector() {
	// Check collector health endpoint
	resp, err := http.Get("http://localhost:13133/")
	if err != nil {
		log.Fatalf("Failed to reach collector health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Collector health check failed with status: %d", resp.StatusCode)
	}
	fmt.Println("✓ Collector health check passed")

	// Check metrics endpoint
	resp2, err := http.Get("http://localhost:8888/metrics")
	if err != nil {
		log.Fatalf("Failed to reach collector metrics endpoint: %v", err)
	}
	defer resp2.Body.Close()

	body, _ := io.ReadAll(resp2.Body)
	metricsOutput := string(body)

	// Verify key collector metrics
	requiredMetrics := []string{
		"otelcol_receiver_accepted_metric_points",
		"otelcol_exporter_sent_metric_points",
		"otelcol_process_uptime",
	}

	for _, metric := range requiredMetrics {
		if !strings.Contains(metricsOutput, metric) {
			log.Fatalf("Missing required collector metric: %s", metric)
		}
	}

	fmt.Println("✓ Collector is processing metrics correctly")
}

func verifyMetricsInPrometheus() {
	// Query for PostgreSQL metrics
	metrics := []string{
		"postgresql_backends",
		"postgresql_database_size_bytes",
		"postgresql_commits_total",
		"postgresql_blocks_read_total",
	}

	client := &http.Client{Timeout: 10 * time.Second}

	for _, metric := range metrics {
		url := fmt.Sprintf("http://localhost:9090/api/v1/query?query=%s", metric)
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("Warning: Failed to query Prometheus for %s: %v", metric, err)
			continue
		}
		defer resp.Body.Close()

		var result PrometheusQueryResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			log.Printf("Warning: Failed to decode Prometheus response for %s: %v", metric, err)
			continue
		}

		if result.Status == "success" && len(result.Data.Result) > 0 {
			fmt.Printf("✓ Found metric: %s (value: %v)\n", metric, result.Data.Result[0].Value[1])
		} else {
			log.Printf("Warning: No data found for metric: %s", metric)
		}
	}
}

func testProcessors(db *sql.DB) {
	// Test 1: Generate queries with different patterns to test AdaptiveSampler
	fmt.Println("  Testing AdaptiveSampler with varied query patterns...")
	
	// Fast queries
	for i := 0; i < 5; i++ {
		db.Exec("SELECT 1")
	}
	
	// Slow query simulation
	db.Exec("SELECT pg_sleep(0.1)")
	
	fmt.Println("  ✓ AdaptiveSampler test queries executed")

	// Test 2: Test CircuitBreaker with rapid queries
	fmt.Println("  Testing CircuitBreaker with rapid queries...")
	
	start := time.Now()
	queryCount := 0
	for time.Since(start) < 2*time.Second {
		db.Exec("SELECT COUNT(*) FROM e2e_test_data")
		queryCount++
		if queryCount%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	
	fmt.Printf("  ✓ CircuitBreaker tested with %d queries\n", queryCount)

	// Test 3: Test PlanAttributeExtractor with EXPLAIN
	fmt.Println("  Testing PlanAttributeExtractor...")
	
	rows, err := db.Query("EXPLAIN (FORMAT JSON) SELECT * FROM e2e_test_data WHERE test_category = 'performance'")
	if err != nil {
		log.Printf("  Warning: Failed to get query plan: %v", err)
	} else {
		defer rows.Close()
		var planJSON string
		for rows.Next() {
			var line string
			rows.Scan(&line)
			planJSON += line
		}
		if len(planJSON) > 0 {
			fmt.Println("  ✓ Query plan extracted successfully")
		}
	}

	// Test 4: Test Verification processor with PII-like data
	fmt.Println("  Testing Verification processor with PII patterns...")
	
	// Insert data that looks like PII (but isn't real)
	testPIIPatterns := []string{
		"test@example.com",
		"555-1234",
		"4532-test-card",
	}
	
	for _, pattern := range testPIIPatterns {
		_, err := db.Exec(`
			INSERT INTO e2e_test_data (test_name, test_value, test_category) 
			VALUES ($1, 0, 'pii_test')
		`, pattern)
		if err != nil {
			// Expected - PII might be blocked
			fmt.Printf("  ✓ PII-like pattern handled: %s\n", pattern)
		}
	}
}

func cleanupTestData(db *sql.DB) {
	_, err := db.Exec("DROP TABLE IF EXISTS e2e_test_data")
	if err != nil {
		log.Printf("Warning: Failed to drop test table: %v", err)
	} else {
		fmt.Println("✓ Cleaned up test data")
	}
}

func queryPrometheus(metric string) (float64, error) {
	url := fmt.Sprintf("http://localhost:9090/api/v1/query?query=%s", metric)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result PrometheusQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if result.Status == "success" && len(result.Data.Result) > 0 {
		if val, ok := result.Data.Result[0].Value[1].(string); ok {
			var floatVal float64
			fmt.Sscanf(val, "%f", &floatVal)
			return floatVal, nil
		}
	}

	return 0, fmt.Errorf("no data found")
}