//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ComprehensiveNRDBValidation validates complete data flow with shape and detail verification
type ComprehensiveNRDBValidation struct {
	*testing.T
	nrAPIKey      string
	nrAccountID   string
	nrQueryURL    string
	testStartTime time.Time
	testRunID     string
}

// TestComprehensiveDataValidation validates the complete data pipeline with detailed shape verification
func TestComprehensiveDataValidation(t *testing.T) {
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping e2e tests - set E2E_TESTS=true to run")
	}

	testRunID := fmt.Sprintf("e2e_%d", time.Now().Unix())
	os.Setenv("TEST_RUN_ID", testRunID)

	test := &ComprehensiveNRDBValidation{
		T:             t,
		nrAPIKey:      os.Getenv("NEW_RELIC_LICENSE_KEY"),
		nrAccountID:   os.Getenv("NEW_RELIC_ACCOUNT_ID"),
		nrQueryURL:    "https://api.newrelic.com/graphql",
		testStartTime: time.Now(),
		testRunID:     testRunID,
	}

	require.NotEmpty(t, test.nrAPIKey, "NEW_RELIC_LICENSE_KEY must be set")
	require.NotEmpty(t, test.nrAccountID, "NEW_RELIC_ACCOUNT_ID must be set")

	// Ensure collector is running
	t.Log("Starting comprehensive NRDB validation tests...")
	t.Logf("Test Run ID: %s", testRunID)

	// Run comprehensive test scenarios
	t.Run("Setup_Test_Data", test.setupTestData)
	t.Run("Validate_PostgreSQL_Metrics_Shape", test.validatePostgreSQLMetricsShape)
	t.Run("Validate_MySQL_Metrics_Shape", test.validateMySQLMetricsShape)
	t.Run("Validate_Metric_Attributes", test.validateMetricAttributes)
	t.Run("Validate_Processor_Effects", test.validateProcessorEffects)
	t.Run("Validate_Data_Accuracy", test.validateDataAccuracy)
	t.Run("Validate_Semantic_Conventions", test.validateSemanticConventions)
}

func (v *ComprehensiveNRDBValidation) setupTestData(t *testing.T) {
	// Create known test data in both databases
	t.Run("PostgreSQL_Test_Data", func(tt *testing.T) {
		db := v.connectPostgreSQL(tt)
		defer db.Close()

		// Create test schema
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS e2e_metrics_test (
				id SERIAL PRIMARY KEY,
				metric_name VARCHAR(255),
				metric_value DECIMAL(10,2),
				tags JSONB,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`)
		require.NoError(tt, err)

		// Insert known values
		testData := []struct {
			name  string
			value float64
			tags  string
		}{
			{"test.gauge", 42.5, `{"environment": "e2e", "test_run": "` + v.testRunID + `"}`},
			{"test.counter", 100.0, `{"environment": "e2e", "test_run": "` + v.testRunID + `"}`},
			{"test.histogram", 250.75, `{"environment": "e2e", "test_run": "` + v.testRunID + `"}`},
		}

		for _, td := range testData {
			_, err = db.Exec(
				"INSERT INTO e2e_metrics_test (metric_name, metric_value, tags) VALUES ($1, $2, $3)",
				td.name, td.value, td.tags,
			)
			require.NoError(tt, err)
		}

		// Generate queries with specific patterns
		v.generateKnownWorkload(tt, db)
	})

	t.Run("MySQL_Test_Data", func(tt *testing.T) {
		db := v.connectMySQL(tt)
		defer db.Close()

		// Create test schema
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS e2e_metrics_test (
				id INT AUTO_INCREMENT PRIMARY KEY,
				metric_name VARCHAR(255),
				metric_value DECIMAL(10,2),
				tags JSON,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`)
		require.NoError(tt, err)

		// Insert similar test data
		_, err = db.Exec(
			"INSERT INTO e2e_metrics_test (metric_name, metric_value, tags) VALUES (?, ?, ?)",
			"test.mysql.gauge", 84.5, `{"environment": "e2e", "test_run": "`+v.testRunID+`"}`,
		)
		require.NoError(tt, err)
	})

	// Wait for initial collection
	t.Log("Waiting for initial metric collection...")
	time.Sleep(30 * time.Second)
}

func (v *ComprehensiveNRDBValidation) validatePostgreSQLMetricsShape(t *testing.T) {
	// Define expected PostgreSQL metrics and their attributes
	expectedMetrics := []struct {
		metricName       string
		requiredAttrs    []string
		optionalAttrs    []string
		valueValidation  func(float64) bool
	}{
		{
			metricName:    "postgresql.database.size",
			requiredAttrs: []string{"db.system", "db.name", "host.name"},
			optionalAttrs: []string{"db.connection_string"},
			valueValidation: func(v float64) bool {
				return v > 0 // Database size should be positive
			},
		},
		{
			metricName:    "postgresql.backends",
			requiredAttrs: []string{"db.system", "host.name"},
			optionalAttrs: []string{"state"},
			valueValidation: func(v float64) bool {
				return v >= 0 // Backend count can be 0
			},
		},
		{
			metricName:    "postgresql.commits",
			requiredAttrs: []string{"db.system", "db.name", "host.name"},
			optionalAttrs: []string{"db.operation"},
			valueValidation: func(v float64) bool {
				return v >= 0 // Commit count starts at 0
			},
		},
	}

	for _, expected := range expectedMetrics {
		t.Run(expected.metricName, func(tt *testing.T) {
			// Query for the metric with all attributes
			query := fmt.Sprintf(`{
				actor {
					account(id: %s) {
						nrql(query: "SELECT * FROM Metric WHERE metricName = '%s' AND test.run_id = '%s' SINCE 5 minutes ago LIMIT 10") {
							results
						}
					}
				}
			}`, v.nrAccountID, expected.metricName, v.testRunID)

			results := v.queryNRDB(tt, query)
			require.NotEmpty(tt, results, "Should have %s metrics", expected.metricName)

			// Validate each metric instance
			for i, result := range results {
				tt.Logf("Validating %s instance %d", expected.metricName, i)

				// Check required attributes
				for _, attr := range expected.requiredAttrs {
					assert.Contains(tt, result, attr, 
						"Metric %s should have required attribute %s", expected.metricName, attr)
					assert.NotEmpty(tt, result[attr], 
						"Required attribute %s should not be empty", attr)
				}

				// Validate metric value
				if value, ok := result["value"].(float64); ok {
					assert.True(tt, expected.valueValidation(value), 
						"Value %f failed validation for %s", value, expected.metricName)
				} else {
					tt.Errorf("Metric %s missing 'value' field", expected.metricName)
				}

				// Check standard attributes
				assert.Contains(tt, result, "timestamp", "Should have timestamp")
				assert.Contains(tt, result, "test.environment", "Should have test environment")
				assert.Equal(tt, "e2e", result["test.environment"], "Should be e2e environment")
				assert.Equal(tt, v.testRunID, result["test.run_id"], "Should have correct run ID")
			}
		})
	}
}

func (v *ComprehensiveNRDBValidation) validateMySQLMetricsShape(t *testing.T) {
	expectedMetrics := []struct {
		metricName       string
		requiredAttrs    []string
		valueValidation  func(float64) bool
	}{
		{
			metricName:    "mysql.threads",
			requiredAttrs: []string{"db.system", "host.name"},
			valueValidation: func(v float64) bool {
				return v > 0 // At least one thread
			},
		},
		{
			metricName:    "mysql.questions",
			requiredAttrs: []string{"db.system", "host.name"},
			valueValidation: func(v float64) bool {
				return v >= 0 // Questions counter
			},
		},
	}

	for _, expected := range expectedMetrics {
		t.Run(expected.metricName, func(tt *testing.T) {
			query := fmt.Sprintf(`{
				actor {
					account(id: %s) {
						nrql(query: "SELECT * FROM Metric WHERE metricName = '%s' AND test.run_id = '%s' SINCE 5 minutes ago LIMIT 10") {
							results
						}
					}
				}
			}`, v.nrAccountID, expected.metricName, v.testRunID)

			results := v.queryNRDB(tt, query)
			require.NotEmpty(tt, results, "Should have %s metrics", expected.metricName)

			for _, result := range results {
				// Validate MySQL-specific attributes
				assert.Equal(tt, "mysql", result["db.system"], "Should be mysql system")
				
				// Check value
				if value, ok := result["value"].(float64); ok {
					assert.True(tt, expected.valueValidation(value))
				}
			}
		})
	}
}

func (v *ComprehensiveNRDBValidation) validateMetricAttributes(t *testing.T) {
	// Validate that metrics have proper semantic convention attributes
	t.Run("Database_Resource_Attributes", func(tt *testing.T) {
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT uniques(db.system), uniques(db.name), uniques(host.name), uniques(service.name) FROM Metric WHERE test.run_id = '%s' SINCE 5 minutes ago") {
						results
					}
				}
			}
		}`, v.nrAccountID, v.testRunID)

		results := v.queryNRDB(tt, query)
		require.NotEmpty(tt, results)

		result := results[0]
		
		// Validate db.system values
		if dbSystems, ok := result["uniques.db.system"].([]interface{}); ok {
			assert.Contains(tt, dbSystems, "postgresql", "Should have postgresql")
			assert.Contains(tt, dbSystems, "mysql", "Should have mysql")
		}

		// Validate db.name values
		if dbNames, ok := result["uniques.db.name"].([]interface{}); ok {
			assert.Contains(tt, dbNames, "testdb", "Should have testdb")
		}
	})

	t.Run("Custom_Attributes", func(tt *testing.T) {
		// Verify our custom attributes are present
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT count(*) FROM Metric WHERE test.environment = 'e2e' AND test.run_id = '%s' SINCE 5 minutes ago") {
						results
					}
				}
			}
		}`, v.nrAccountID, v.testRunID)

		results := v.queryNRDB(tt, query)
		require.NotEmpty(tt, results)

		count := results[0]["count"].(float64)
		assert.Greater(tt, count, float64(0), "Should have metrics with custom attributes")
	})
}

func (v *ComprehensiveNRDBValidation) validateProcessorEffects(t *testing.T) {
	// Wait for workload data to be processed
	time.Sleep(2 * time.Minute)

	t.Run("Adaptive_Sampler_Effect", func(tt *testing.T) {
		// Verify slow queries are sampled at 100%
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT percentage(count(*), WHERE sampled = true) as sample_rate FROM Metric WHERE duration_ms > 1000 AND test.run_id = '%s' SINCE 5 minutes ago") {
						results
					}
				}
			}
		}`, v.nrAccountID, v.testRunID)

		results := v.queryNRDB(tt, query)
		if len(results) > 0 {
			if sampleRate, ok := results[0]["sample_rate"].(float64); ok {
				assert.Equal(tt, float64(100), sampleRate, "Slow queries should be 100% sampled")
			}
		}
	})

	t.Run("Plan_Extraction_Validation", func(tt *testing.T) {
		// Check that plan attributes are extracted
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT * FROM Metric WHERE plan.hash IS NOT NULL AND test.run_id = '%s' SINCE 5 minutes ago LIMIT 5") {
						results
					}
				}
			}
		}`, v.nrAccountID, v.testRunID)

		results := v.queryNRDB(tt, query)
		if len(results) > 0 {
			for _, result := range results {
				// Validate plan attributes
				assert.Contains(tt, result, "plan.hash", "Should have plan hash")
				assert.Contains(tt, result, "plan.total_cost", "Should have plan cost")
				
				// Plan hash should be non-empty string
				if hash, ok := result["plan.hash"].(string); ok {
					assert.NotEmpty(tt, hash, "Plan hash should not be empty")
					assert.Len(tt, hash, 64, "Plan hash should be SHA256")
				}
			}
		}
	})

	t.Run("PII_Sanitization_Validation", func(tt *testing.T) {
		// Ensure no PII patterns in query text
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT * FROM Metric WHERE query_text IS NOT NULL AND test.run_id = '%s' SINCE 5 minutes ago LIMIT 100") {
						results
					}
				}
			}
		}`, v.nrAccountID, v.testRunID)

		results := v.queryNRDB(tt, query)
		for _, result := range results {
			if queryText, ok := result["query_text"].(string); ok {
				// Check for PII patterns
				assert.NotContains(tt, queryText, "@example.com", "Should not contain email")
				assert.NotContains(tt, queryText, "123-45-6789", "Should not contain SSN")
				assert.NotRegexp(tt, `\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`, queryText, "Should not contain credit card")
				
				// Check for masking
				if strings.Contains(queryText, "****") {
					tt.Log("Found masked content in query text")
				}
			}
		}
	})

	t.Run("Circuit_Breaker_Metrics", func(tt *testing.T) {
		// Check circuit breaker state attributes
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT uniques(circuit_breaker.state), uniques(circuit_breaker.database) FROM Metric WHERE circuit_breaker.state IS NOT NULL AND test.run_id = '%s' SINCE 5 minutes ago") {
						results
					}
				}
			}
		}`, v.nrAccountID, v.testRunID)

		results := v.queryNRDB(tt, query)
		if len(results) > 0 {
			result := results[0]
			if states, ok := result["uniques.circuit_breaker.state"].([]interface{}); ok {
				// Should mostly be "closed" in healthy operation
				assert.Contains(tt, states, "closed", "Should have closed state")
			}
		}
	})
}

func (v *ComprehensiveNRDBValidation) validateDataAccuracy(t *testing.T) {
	// Generate specific known workload and validate it appears correctly
	db := v.connectPostgreSQL(t)
	defer db.Close()

	// Create a specific number of test records
	testCount := 50
	testTableName := fmt.Sprintf("accuracy_test_%s", v.testRunID)
	
	_, err := db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			data VARCHAR(255)
		)
	`, testTableName))
	require.NoError(t, err)

	// Insert exact number of records
	for i := 0; i < testCount; i++ {
		_, err = db.Exec(fmt.Sprintf("INSERT INTO %s (data) VALUES ($1)", testTableName), 
			fmt.Sprintf("test_record_%d", i))
		require.NoError(t, err)
	}

	// Wait for metrics to be collected
	time.Sleep(90 * time.Second)

	// Query for table size metric
	query := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "SELECT latest(value) FROM Metric WHERE metricName = 'postgresql.table.size' AND table.name = '%s' SINCE 3 minutes ago") {
					results
				}
			}
		}
	}`, v.nrAccountID, testTableName)

	results := v.queryNRDB(t, query)
	if len(results) > 0 {
		if tableSize, ok := results[0]["latest.value"].(float64); ok {
			// Table should have size > 0
			assert.Greater(t, tableSize, float64(0), "Table size should be positive")
			t.Logf("Table %s size: %f bytes", testTableName, tableSize)
		}
	}

	// Cleanup
	db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", testTableName))
}

func (v *ComprehensiveNRDBValidation) validateSemanticConventions(t *testing.T) {
	// Validate that metrics follow OpenTelemetry semantic conventions
	query := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "SELECT * FROM Metric WHERE test.run_id = '%s' SINCE 5 minutes ago LIMIT 50") {
					results
				}
			}
		}
	}`, v.nrAccountID, v.testRunID)

	results := v.queryNRDB(t, query)
	require.NotEmpty(t, results, "Should have metrics to validate")

	for _, metric := range results {
		// Check metric naming convention
		if metricName, ok := metric["metricName"].(string); ok {
			// PostgreSQL metrics should start with postgresql.
			if strings.Contains(metricName, "postgres") {
				assert.True(t, strings.HasPrefix(metricName, "postgresql."), 
					"PostgreSQL metric %s should start with 'postgresql.'", metricName)
			}
			
			// MySQL metrics should start with mysql.
			if strings.Contains(metricName, "mysql") {
				assert.True(t, strings.HasPrefix(metricName, "mysql."), 
					"MySQL metric %s should start with 'mysql.'", metricName)
			}
		}

		// Validate timestamp format
		if timestamp, ok := metric["timestamp"].(float64); ok {
			// Timestamp should be recent (within last hour)
			timestampTime := time.Unix(int64(timestamp/1000), 0)
			assert.WithinDuration(t, time.Now(), timestampTime, time.Hour, 
				"Timestamp should be within last hour")
		}

		// Validate common attributes
		if dbSystem, ok := metric["db.system"].(string); ok {
			assert.Contains(t, []string{"postgresql", "mysql"}, dbSystem, 
				"db.system should be valid database type")
		}
	}
}

// Helper methods

func (v *ComprehensiveNRDBValidation) connectPostgreSQL(t *testing.T) *sql.DB {
	pgHost := os.Getenv("POSTGRES_HOST")
	if pgHost == "" {
		pgHost = "localhost"
	}

	connStr := fmt.Sprintf("host=%s port=5432 user=postgres password=postgres dbname=testdb sslmode=disable", pgHost)
	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err)

	err = db.Ping()
	require.NoError(t, err)

	return db
}

func (v *ComprehensiveNRDBValidation) connectMySQL(t *testing.T) *sql.DB {
	mysqlHost := os.Getenv("MYSQL_HOST")
	if mysqlHost == "" {
		mysqlHost = "localhost"
	}

	connStr := fmt.Sprintf("root:mysql@tcp(%s:3306)/testdb", mysqlHost)
	db, err := sql.Open("mysql", connStr)
	require.NoError(t, err)

	err = db.Ping()
	require.NoError(t, err)

	return db
}

func (v *ComprehensiveNRDBValidation) generateKnownWorkload(t *testing.T, db *sql.DB) {
	// Generate queries with known patterns
	
	// 1. Slow queries (should be 100% sampled)
	for i := 0; i < 5; i++ {
		_, err := db.Exec("SELECT pg_sleep(0.2)")
		assert.NoError(t, err)
	}

	// 2. Fast queries (should follow default sampling)
	for i := 0; i < 20; i++ {
		_, err := db.Exec("SELECT 1")
		assert.NoError(t, err)
	}

	// 3. Queries with PII (should be sanitized)
	piiQueries := []string{
		"SELECT * FROM users WHERE email = 'test@example.com'",
		"SELECT * FROM customers WHERE ssn = '123-45-6789'",
		"SELECT * FROM payments WHERE card_number = '4111-1111-1111-1111'",
	}

	for _, query := range piiQueries {
		_, _ = db.Exec(query) // Ignore errors as tables may not exist
	}

	// 4. Complex queries (for plan extraction)
	_, err := db.Exec(`
		SELECT 
			t1.id, 
			t1.metric_name, 
			COUNT(t2.id) as related_count
		FROM 
			e2e_metrics_test t1
		LEFT JOIN 
			e2e_metrics_test t2 ON t1.id = t2.id
		GROUP BY 
			t1.id, t1.metric_name
		HAVING 
			COUNT(t2.id) > 0
	`)
	assert.NoError(t, err)
}

func (v *ComprehensiveNRDBValidation) queryNRDB(t *testing.T, query string) []map[string]interface{} {
	client := &http.Client{Timeout: 30 * time.Second}
	
	reqBody := map[string]string{"query": query}
	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", v.nrQueryURL, bytes.NewBuffer(jsonBody))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", v.nrAPIKey)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("NRDB query failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			Actor struct {
				Account struct {
					NRQL struct {
						Results []map[string]interface{} `json:"results"`
					} `json:"nrql"`
				} `json:"account"`
			} `json:"actor"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	if len(result.Errors) > 0 {
		t.Fatalf("NRDB query returned errors: %v", result.Errors)
	}

	return result.Data.Actor.Account.NRQL.Results
}