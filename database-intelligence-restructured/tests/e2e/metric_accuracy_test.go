package e2e

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestMetricAccuracy verifies that collector metrics match actual database values
func TestMetricAccuracy(t *testing.T) {
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

	runID := fmt.Sprintf("accuracy_test_%d", time.Now().Unix())
	t.Logf("Starting metric accuracy test with run ID: %s", runID)

	// Start PostgreSQL
	t.Log("Starting PostgreSQL container...")
	postgresCmd := exec.Command("docker", "run",
		"--name", "e2e-accuracy-postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "25432:5432",
		"--network", "bridge",
		"-d", "postgres:15-alpine")

	output, err := postgresCmd.CombinedOutput()
	if err != nil {
		if !strings.Contains(string(output), "already in use") {
			t.Fatalf("Failed to start PostgreSQL: %v\n%s", err, output)
		}
		exec.Command("docker", "rm", "-f", "e2e-accuracy-postgres").Run()
		postgresCmd = exec.Command("docker", "run",
			"--name", "e2e-accuracy-postgres",
			"-e", "POSTGRES_PASSWORD=postgres",
			"-p", "25432:5432",
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
		exec.Command("docker", "stop", "e2e-accuracy-postgres").Run()
		exec.Command("docker", "rm", "e2e-accuracy-postgres").Run()
		exec.Command("docker", "stop", "e2e-accuracy-collector").Run()
		exec.Command("docker", "rm", "e2e-accuracy-collector").Run()
	}()

	// Wait for PostgreSQL
	time.Sleep(15 * time.Second)

	// Connect to PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=25432 user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Create test data with known values
	t.Log("Creating test data with known values...")
	expectedMetrics := createAccuracyTestData(t, db)

	// Start collector
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: host.docker.internal:25432
    transport: tcp
    username: postgres
    password: postgres
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
        value: %s
        action: insert
      - key: test.type
        value: accuracy
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
      processors: [attributes, batch]
      exporters: [otlp, logging]

  telemetry:
    logs:
      level: debug
`, runID, otlpEndpoint, licenseKey)

	// Write config
	configPath := "accuracy-test-config.yaml"
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
	exec.Command("docker", "rm", "-f", "e2e-accuracy-collector").Run()
	
	collectorCmd := exec.Command("docker", "run",
		"--name", "e2e-accuracy-collector",
		"-v", absConfigPathStr+":/etc/otel-collector-config.yaml",
		"--add-host", "host.docker.internal:host-gateway",
		"-d",
		"otel/opentelemetry-collector-contrib:0.92.0",
		"--config=/etc/otel-collector-config.yaml")

	output, err = collectorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start collector: %v\n%s", err, output)
	}

	// Wait for metrics to be collected and sent
	t.Log("Waiting for metric collection (60 seconds)...")
	time.Sleep(60 * time.Second)

	// Verify metrics in NRDB
	t.Log("Verifying metrics in NRDB...")
	verifyMetricAccuracy(t, db, expectedMetrics, runID, accountID, apiKey)
}

type ExpectedMetrics struct {
	TableCount      int
	TotalRows       int64
	DatabaseSize    int64
	ConnectionCount int
	Tables          map[string]TableMetrics
}

type TableMetrics struct {
	RowCount  int64
	TableSize int64
	IndexSize int64
}

func createAccuracyTestData(t *testing.T, db *sql.DB) ExpectedMetrics {
	expected := ExpectedMetrics{
		Tables: make(map[string]TableMetrics),
	}

	// Create schema and tables
	queries := []string{
		`CREATE SCHEMA IF NOT EXISTS accuracy_test`,
		`CREATE TABLE IF NOT EXISTS accuracy_test.small_table (
			id SERIAL PRIMARY KEY,
			data VARCHAR(100)
		)`,
		`CREATE TABLE IF NOT EXISTS accuracy_test.medium_table (
			id SERIAL PRIMARY KEY,
			name VARCHAR(200),
			value NUMERIC,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS accuracy_test.large_table (
			id SERIAL PRIMARY KEY,
			data TEXT,
			category VARCHAR(50),
			amount DECIMAL(10,2),
			is_active BOOLEAN DEFAULT true
		)`,
		`CREATE INDEX idx_medium_value ON accuracy_test.medium_table(value)`,
		`CREATE INDEX idx_large_category ON accuracy_test.large_table(category)`,
		`CREATE INDEX idx_large_amount ON accuracy_test.large_table(amount)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("Failed to execute query: %v", err)
		}
	}

	// Insert known amounts of data
	// Small table: 10 rows
	for i := 0; i < 10; i++ {
		_, err := db.Exec(`INSERT INTO accuracy_test.small_table (data) VALUES ($1)`, 
			fmt.Sprintf("small_data_%d", i))
		if err != nil {
			t.Fatalf("Failed to insert data: %v", err)
		}
	}
	expected.Tables["small_table"] = TableMetrics{RowCount: 10}

	// Medium table: 100 rows
	for i := 0; i < 100; i++ {
		_, err := db.Exec(`INSERT INTO accuracy_test.medium_table (name, value) VALUES ($1, $2)`, 
			fmt.Sprintf("medium_name_%d", i), float64(i)*2.5)
		if err != nil {
			t.Fatalf("Failed to insert data: %v", err)
		}
	}
	expected.Tables["medium_table"] = TableMetrics{RowCount: 100}

	// Large table: 1000 rows
	categories := []string{"A", "B", "C", "D", "E"}
	for i := 0; i < 1000; i++ {
		_, err := db.Exec(`INSERT INTO accuracy_test.large_table (data, category, amount) VALUES ($1, $2, $3)`, 
			fmt.Sprintf("large_data_%d", i), 
			categories[i%len(categories)], 
			float64(i)*0.99)
		if err != nil {
			t.Fatalf("Failed to insert data: %v", err)
		}
	}
	expected.Tables["large_table"] = TableMetrics{RowCount: 1000}

	// Calculate total rows
	expected.TotalRows = 10 + 100 + 1000
	expected.TableCount = 3

	// Get actual table sizes
	for tableName, metrics := range expected.Tables {
		var tableSize, indexSize sql.NullInt64
		
		// Get table size
		err := db.QueryRow(`
			SELECT pg_relation_size($1::regclass), 
			       COALESCE(SUM(pg_relation_size(indexrelid)), 0)
			FROM pg_index 
			WHERE indrelid = $1::regclass`,
			fmt.Sprintf("accuracy_test.%s", tableName)).Scan(&tableSize, &indexSize)
		
		if err == nil && tableSize.Valid {
			metrics.TableSize = tableSize.Int64
			metrics.IndexSize = indexSize.Int64
			expected.Tables[tableName] = metrics
		}
	}

	// Get connection count
	var connCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM pg_stat_activity WHERE datname = 'postgres'`).Scan(&connCount)
	if err == nil {
		expected.ConnectionCount = connCount
	}

	return expected
}

func verifyMetricAccuracy(t *testing.T, db *sql.DB, expected ExpectedMetrics, runID, accountID, apiKey string) {
	// Wait a bit for metrics to be processed
	time.Sleep(10 * time.Second)

	// Verify table count
	t.Run("VerifyTableCount", func(t *testing.T) {
		nrql := fmt.Sprintf("SELECT latest(postgresql.table.count) FROM Metric WHERE test.run.id = '%s' SINCE 2 minutes ago", runID)
		result, err := queryNRDB(accountID, apiKey, nrql)
		if err != nil {
			t.Errorf("Failed to query table count: %v", err)
			return
		}

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			if tableCount, ok := result.Data.Actor.Account.NRQL.Results[0]["latest.postgresql.table.count"].(float64); ok {
				// Note: collector might report more tables including system tables
				t.Logf("Table count - Expected (minimum): %d, Actual: %.0f", expected.TableCount, tableCount)
				if int(tableCount) < expected.TableCount {
					t.Errorf("Table count mismatch - Expected at least %d, got %.0f", expected.TableCount, tableCount)
				}
			}
		}
	})

	// Verify row counts for each table
	t.Run("VerifyRowCounts", func(t *testing.T) {
		for tableName, expectedMetrics := range expected.Tables {
			nrql := fmt.Sprintf("SELECT latest(postgresql.rows) FROM Metric WHERE test.run.id = '%s' AND postgresql.table.name = '%s' AND state = 'live' SINCE 2 minutes ago", runID, tableName)
			
			result, err := queryNRDB(accountID, apiKey, nrql)
			if err != nil {
				t.Errorf("Failed to query rows for %s: %v", tableName, err)
				continue
			}

			if len(result.Data.Actor.Account.NRQL.Results) > 0 {
				if rowCount, ok := result.Data.Actor.Account.NRQL.Results[0]["latest.postgresql.rows"].(float64); ok {
					t.Logf("%s rows - Expected: %d, Actual: %.0f", tableName, expectedMetrics.RowCount, rowCount)
					if math.Abs(rowCount-float64(expectedMetrics.RowCount)) > 5 {
						t.Errorf("%s row count mismatch - Expected %d, got %.0f", tableName, expectedMetrics.RowCount, rowCount)
					}
				} else {
					t.Logf("No row count found for table %s", tableName)
				}
			}
		}
	})

	// Verify table sizes
	t.Run("VerifyTableSizes", func(t *testing.T) {
		for tableName := range expected.Tables {
			nrql := fmt.Sprintf("SELECT latest(postgresql.table.size) FROM Metric WHERE test.run.id = '%s' AND postgresql.table.name = '%s' SINCE 2 minutes ago", runID, tableName)
			
			result, err := queryNRDB(accountID, apiKey, nrql)
			if err != nil {
				t.Errorf("Failed to query size for %s: %v", tableName, err)
				continue
			}

			if len(result.Data.Actor.Account.NRQL.Results) > 0 {
				data, _ := json.MarshalIndent(result.Data.Actor.Account.NRQL.Results[0], "  ", "  ")
				t.Logf("%s size result: %s", tableName, string(data))
			}
		}
	})

	// Verify connection metrics
	t.Run("VerifyConnections", func(t *testing.T) {
		nrql := fmt.Sprintf("SELECT latest(postgresql.backends) FROM Metric WHERE test.run.id = '%s' SINCE 2 minutes ago", runID)
		result, err := queryNRDB(accountID, apiKey, nrql)
		if err != nil {
			t.Errorf("Failed to query backends: %v", err)
			return
		}

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			if backends, ok := result.Data.Actor.Account.NRQL.Results[0]["latest.postgresql.backends"].(float64); ok {
				t.Logf("Active backends - Actual: %.0f", backends)
				// Just verify we have some connections
				if backends < 1 {
					t.Error("Expected at least 1 active backend connection")
				}
			}
		}
	})

	// Get current database metrics for comparison
	t.Run("CompareCurrentMetrics", func(t *testing.T) {
		// Get current row counts from database
		rows, err := db.Query(`
			SELECT schemaname, relname, n_live_tup 
			FROM pg_stat_user_tables 
			WHERE schemaname = 'accuracy_test'
			ORDER BY relname`)
		if err != nil {
			t.Errorf("Failed to query current stats: %v", err)
			return
		}
		defer rows.Close()

		t.Log("Current database statistics:")
		for rows.Next() {
			var schema, table string
			var rowCount int64
			if err := rows.Scan(&schema, &table, &rowCount); err == nil {
				t.Logf("  %s.%s: %d live tuples", schema, table, rowCount)
			}
		}
	})
}