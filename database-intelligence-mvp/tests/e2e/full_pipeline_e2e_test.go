package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestFullPipelineE2E tests the complete data flow from databases to NRDB
func TestFullPipelineE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()
	
	// Start docker-compose environment
	composeFile := filepath.Join(".", "docker-compose.e2e.yml")
	identifier := strings.ToLower(t.Name())
	
	compose, err := compose.NewDockerCompose(composeFile)
	require.NoError(t, err, "Failed to create docker-compose")
	
	t.Cleanup(func() {
		assert.NoError(t, compose.Down(context.Background(), compose.RemoveOrphans(true), compose.RemoveImagesLocal()))
	})
	
	// Start services
	err = compose.Up(ctx, compose.Wait(true))
	require.NoError(t, err, "Failed to start docker-compose services")
	
	// Wait for services to be ready
	time.Sleep(30 * time.Second)
	
	// Run test suites
	t.Run("PostgreSQL_Pipeline", func(t *testing.T) {
		testPostgreSQLPipeline(t, ctx)
	})
	
	t.Run("MySQL_Pipeline", func(t *testing.T) {
		testMySQLPipeline(t, ctx)
	})
	
	t.Run("PII_Sanitization", func(t *testing.T) {
		testPIISanitization(t, ctx)
	})
	
	t.Run("Query_Correlation", func(t *testing.T) {
		testQueryCorrelation(t, ctx)
	})
	
	t.Run("Cost_Control", func(t *testing.T) {
		testCostControl(t, ctx)
	})
	
	t.Run("NRDB_Export", func(t *testing.T) {
		testNRDBExport(t, ctx)
	})
}

func testPostgreSQLPipeline(t *testing.T, ctx context.Context) {
	// Connect to PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=5433 user=postgres password=postgres dbname=e2e_test sslmode=disable")
	require.NoError(t, err)
	defer db.Close()
	
	// Execute test queries
	queries := []struct {
		name  string
		query string
		args  []interface{}
	}{
		{
			name:  "simple_select",
			query: "SELECT * FROM e2e_test.users WHERE email = $1",
			args:  []interface{}{"test@example.com"},
		},
		{
			name:  "join_query",
			query: "SELECT * FROM e2e_test.generate_join_query()",
			args:  []interface{}{},
		},
		{
			name:  "expensive_query",
			query: "SELECT * FROM e2e_test.generate_expensive_query()",
			args:  []interface{}{},
		},
	}
	
	for _, q := range queries {
		t.Run(q.name, func(t *testing.T) {
			// Execute query multiple times for sampling
			for i := 0; i < 10; i++ {
				rows, err := db.QueryContext(ctx, q.query, q.args...)
				require.NoError(t, err)
				rows.Close()
			}
		})
	}
	
	// Wait for metrics collection
	time.Sleep(15 * time.Second)
	
	// Verify metrics were collected
	resp, err := http.Get("http://localhost:8890/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	
	metrics := string(body)
	assert.Contains(t, metrics, "postgresql_backends")
	assert.Contains(t, metrics, "postgresql_commits")
	assert.Contains(t, metrics, "db_query_stats")
	
	// Check processed metrics in file output
	outputFile := filepath.Join(".", "output", "e2e-output.json")
	if _, err := os.Stat(outputFile); err == nil {
		data, err := os.ReadFile(outputFile)
		require.NoError(t, err)
		
		// Verify data structure
		var output map[string]interface{}
		err = json.Unmarshal(data, &output)
		require.NoError(t, err)
		
		// Check for expected attributes
		assert.Contains(t, string(data), "service.name")
		assert.Contains(t, string(data), "database-intelligence-e2e")
		assert.Contains(t, string(data), "db.system")
		assert.Contains(t, string(data), "postgresql")
	}
}

func testMySQLPipeline(t *testing.T, ctx context.Context) {
	// Connect to MySQL
	db, err := sql.Open("mysql", "mysql:mysql@tcp(localhost:3307)/e2e_test")
	require.NoError(t, err)
	defer db.Close()
	
	// Execute test queries
	queries := []struct {
		name  string
		query string
	}{
		{
			name:  "simple_select",
			query: "SELECT * FROM users WHERE email = 'test@example.com'",
		},
		{
			name:  "stored_procedure",
			query: "CALL generate_join_query()",
		},
		{
			name:  "complex_view",
			query: "SELECT * FROM user_order_summary",
		},
	}
	
	for _, q := range queries {
		t.Run(q.name, func(t *testing.T) {
			// Execute query multiple times
			for i := 0; i < 10; i++ {
				rows, err := db.QueryContext(ctx, q.query)
				require.NoError(t, err)
				rows.Close()
			}
		})
	}
	
	// Wait for metrics collection
	time.Sleep(15 * time.Second)
	
	// Verify MySQL metrics
	resp, err := http.Get("http://localhost:8890/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	
	metrics := string(body)
	assert.Contains(t, metrics, "mysql_buffer_pool")
	assert.Contains(t, metrics, "mysql_operations")
}

func testPIISanitization(t *testing.T, ctx context.Context) {
	// Send logs with PII via OTLP
	payload := `{
		"resourceLogs": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": {"stringValue": "test-pii"}
				}]
			},
			"scopeLogs": [{
				"logRecords": [{
					"timeUnixNano": "1234567890000000000",
					"body": {"stringValue": "Query: SELECT * FROM users WHERE email='john.doe@example.com' AND ssn='123-45-6789'"},
					"attributes": [{
						"key": "db.statement",
						"value": {"stringValue": "SELECT * FROM users WHERE email='john.doe@example.com' AND ssn='123-45-6789'"}
					}, {
						"key": "user.credit_card",
						"value": {"stringValue": "4111-1111-1111-1111"}
					}]
				}]
			}]
		}]
	}`
	
	resp, err := http.Post("http://localhost:4321/v1/logs", "application/json", strings.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	
	// Wait for processing
	time.Sleep(5 * time.Second)
	
	// Check output file for PII sanitization
	outputFile := filepath.Join(".", "output", "e2e-output.json")
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	
	output := string(data)
	
	// Verify PII was redacted
	assert.NotContains(t, output, "john.doe@example.com")
	assert.NotContains(t, output, "123-45-6789")
	assert.NotContains(t, output, "4111-1111-1111-1111")
	
	// Should contain redacted markers
	assert.Contains(t, output, "[REDACTED_EMAIL]")
	assert.Contains(t, output, "[REDACTED_SSN]")
	assert.Contains(t, output, "[REDACTED_CREDIT_CARD]")
}

func testQueryCorrelation(t *testing.T, ctx context.Context) {
	// Connect to PostgreSQL
	db, err := sql.Open("postgres", "host=localhost port=5433 user=postgres password=postgres dbname=e2e_test sslmode=disable")
	require.NoError(t, err)
	defer db.Close()
	
	// Execute correlated queries
	// First, get user IDs
	rows, err := db.Query("SELECT id FROM e2e_test.users")
	require.NoError(t, err)
	
	var userIDs []int
	for rows.Next() {
		var id int
		err := rows.Scan(&id)
		require.NoError(t, err)
		userIDs = append(userIDs, id)
	}
	rows.Close()
	
	// Then query orders for each user
	for _, userID := range userIDs {
		_, err = db.Exec("SELECT * FROM e2e_test.orders WHERE user_id = $1", userID)
		require.NoError(t, err)
	}
	
	// Wait for correlation
	time.Sleep(15 * time.Second)
	
	// Check metrics for correlation attributes
	resp, err := http.Get("http://localhost:8890/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	
	metrics := string(body)
	
	// Should have correlation attributes
	assert.Contains(t, metrics, "correlation_query_category")
	assert.Contains(t, metrics, "correlation_table_name")
}

func testCostControl(t *testing.T, ctx context.Context) {
	// Generate high cardinality metrics
	for i := 0; i < 100; i++ {
		payload := fmt.Sprintf(`{
			"resourceMetrics": [{
				"resource": {
					"attributes": [{
						"key": "service.name",
						"value": {"stringValue": "high-cardinality-test"}
					}]
				},
				"scopeMetrics": [{
					"metrics": [{
						"name": "test.metric.%d",
						"gauge": {
							"dataPoints": [{
								"timeUnixNano": "%d",
								"asDouble": %f,
								"attributes": [{
									"key": "unique_id",
									"value": {"stringValue": "id_%d"}
								}]
							}]
						}
					}]
				}]
			}]
		}`, i, time.Now().UnixNano(), float64(i), i)
		
		resp, err := http.Post("http://localhost:4321/v1/metrics", "application/json", strings.NewReader(payload))
		require.NoError(t, err)
		resp.Body.Close()
	}
	
	// Wait for processing
	time.Sleep(10 * time.Second)
	
	// Check cost control metrics
	resp, err := http.Get("http://localhost:8890/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	
	metrics := string(body)
	
	// Should have cost tracking metrics
	assert.Contains(t, metrics, "dbintel_cost_bytes_ingested_total")
	assert.Contains(t, metrics, "dbintel_cost_estimated_monthly_cost")
	assert.Contains(t, metrics, "dbintel_cardinality_unique_metrics")
}

func testNRDBExport(t *testing.T, ctx context.Context) {
	// Check mock server received data
	resp, err := http.Get("http://localhost:4319/mockserver/verify")
	require.NoError(t, err)
	defer resp.Body.Close()
	
	// Get request log from mock server
	logResp, err := http.Get("http://localhost:4319/mockserver/retrieve?type=REQUESTS")
	require.NoError(t, err)
	defer logResp.Body.Close()
	
	body, err := io.ReadAll(logResp.Body)
	require.NoError(t, err)
	
	var requests []map[string]interface{}
	err = json.Unmarshal(body, &requests)
	require.NoError(t, err)
	
	// Verify requests were made to OTLP endpoints
	metricsRequests := 0
	logsRequests := 0
	
	for _, req := range requests {
		if req["path"] == "/v1/metrics" {
			metricsRequests++
		}
		if req["path"] == "/v1/logs" {
			logsRequests++
		}
	}
	
	assert.Greater(t, metricsRequests, 0, "Should have sent metrics to NRDB")
	assert.Greater(t, logsRequests, 0, "Should have sent logs to NRDB")
	
	// Verify data format
	if len(requests) > 0 {
		// Check for proper OTLP format
		for _, req := range requests {
			if body, ok := req["body"].(map[string]interface{}); ok {
				// Should have resourceMetrics or resourceLogs
				assert.True(t, 
					body["resourceMetrics"] != nil || body["resourceLogs"] != nil,
					"Request should contain OTLP formatted data")
			}
		}
	}
}