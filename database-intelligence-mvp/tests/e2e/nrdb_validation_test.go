//go:build e2e

package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NRDBValidationTest validates end-to-end data flow from source databases to New Relic
type NRDBValidationTest struct {
	*testing.T
	nrAPIKey      string
	nrAccountID   string
	nrQueryURL    string
	testStartTime time.Time
}

// TestEndToEndDataFlow validates the complete data pipeline from databases to NRDB
func TestEndToEndDataFlow(t *testing.T) {
	// Skip if not in e2e mode
	if os.Getenv("E2E_TESTS") != "true" {
		t.Skip("Skipping e2e tests - set E2E_TESTS=true to run")
	}

	test := &NRDBValidationTest{
		T:             t,
		nrAPIKey:      os.Getenv("NEW_RELIC_LICENSE_KEY"),
		nrAccountID:   os.Getenv("NEW_RELIC_ACCOUNT_ID"),
		nrQueryURL:    "https://api.newrelic.com/graphql",
		testStartTime: time.Now(),
	}

	// Validate prerequisites
	require.NotEmpty(t, test.nrAPIKey, "NEW_RELIC_LICENSE_KEY must be set")
	require.NotEmpty(t, test.nrAccountID, "NEW_RELIC_ACCOUNT_ID must be set")

	// Run test scenarios
	t.Run("PostgreSQL_Metrics_Flow", test.testPostgreSQLMetricsFlow)
	t.Run("MySQL_Metrics_Flow", test.testMySQLMetricsFlow)
	t.Run("Custom_Query_Metrics", test.testCustomQueryMetrics)
	t.Run("Processor_Validation", test.testProcessorFunctionality)
	t.Run("Data_Completeness", test.testDataCompleteness)
}

func (t *NRDBValidationTest) testPostgreSQLMetricsFlow(tt *testing.T) {
	// Generate test load on PostgreSQL
	pgHost := os.Getenv("POSTGRES_HOST")
	if pgHost == "" {
		pgHost = "localhost"
	}

	connStr := fmt.Sprintf("host=%s port=5432 user=postgres password=postgres dbname=testdb sslmode=disable", pgHost)
	db, err := sql.Open("postgres", connStr)
	require.NoError(tt, err)
	defer db.Close()

	// Create test data and generate metrics
	t.generatePostgreSQLLoad(tt, db)

	// Wait for data to flow through collector
	time.Sleep(2 * time.Minute)

	// Query NRDB for PostgreSQL metrics
	query := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql.%%' SINCE %d seconds ago") {
					results
				}
			}
		}
	}`, t.nrAccountID, int(time.Since(t.testStartTime).Seconds()))

	results := t.queryNRDB(tt, query)
	
	// Validate metrics exist
	var count float64
	if len(results) > 0 {
		if c, ok := results[0]["count"].(float64); ok {
			count = c
		}
	}
	
	assert.Greater(tt, count, float64(0), "Should have PostgreSQL metrics in NRDB")

	// Validate specific metrics
	t.validatePostgreSQLMetrics(tt)
}

func (t *NRDBValidationTest) testMySQLMetricsFlow(tt *testing.T) {
	// Generate test load on MySQL
	mysqlHost := os.Getenv("MYSQL_HOST")
	if mysqlHost == "" {
		mysqlHost = "localhost"
	}

	connStr := fmt.Sprintf("root:mysql@tcp(%s:3306)/testdb", mysqlHost)
	db, err := sql.Open("mysql", connStr)
	require.NoError(tt, err)
	defer db.Close()

	// Create test data and generate metrics
	t.generateMySQLLoad(tt, db)

	// Wait for data to flow
	time.Sleep(2 * time.Minute)

	// Query NRDB for MySQL metrics
	query := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "SELECT count(*) FROM Metric WHERE metricName LIKE 'mysql.%%' SINCE %d seconds ago") {
					results
				}
			}
		}
	}`, t.nrAccountID, int(time.Since(t.testStartTime).Seconds()))

	results := t.queryNRDB(tt, query)
	
	// Validate metrics exist
	var count float64
	if len(results) > 0 {
		if c, ok := results[0]["count"].(float64); ok {
			count = c
		}
	}
	
	assert.Greater(tt, count, float64(0), "Should have MySQL metrics in NRDB")

	// Validate specific metrics
	t.validateMySQLMetrics(tt)
}

func (t *NRDBValidationTest) testCustomQueryMetrics(tt *testing.T) {
	// Test custom SQL query receiver metrics
	time.Sleep(1 * time.Minute)

	query := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "SELECT count(*), average(value) FROM Metric WHERE metricName = 'sqlquery.active_connections' SINCE %d seconds ago") {
					results
				}
			}
		}
	}`, t.nrAccountID, int(time.Since(t.testStartTime).Seconds()))

	results := t.queryNRDB(tt, query)
	
	if len(results) > 0 {
		count, _ := results[0]["count"].(float64)
		assert.Greater(tt, count, float64(0), "Should have custom query metrics")
		
		if avg, ok := results[0]["average.value"].(float64); ok {
			assert.Greater(tt, avg, float64(0), "Should have valid connection count")
		}
	}
}

func (t *NRDBValidationTest) testProcessorFunctionality(tt *testing.T) {
	// Validate adaptive sampling
	tt.Run("Adaptive_Sampling", func(ttt *testing.T) {
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT count(*) FROM Metric WHERE sampled = true AND duration_ms > 1000 SINCE %d seconds ago") {
						results
					}
				}
			}
		}`, t.nrAccountID, int(time.Since(t.testStartTime).Seconds()))

		results := t.queryNRDB(ttt, query)
		
		if len(results) > 0 {
			count, _ := results[0]["count"].(float64)
			assert.Greater(ttt, count, float64(0), "Slow queries should be sampled")
		}
	})

	// Validate plan extraction
	tt.Run("Plan_Extraction", func(ttt *testing.T) {
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT count(*) FROM Metric WHERE plan.hash IS NOT NULL SINCE %d seconds ago") {
						results
					}
				}
			}
		}`, t.nrAccountID, int(time.Since(t.testStartTime).Seconds()))

		results := t.queryNRDB(ttt, query)
		
		if len(results) > 0 {
			count, _ := results[0]["count"].(float64)
			assert.Greater(ttt, count, float64(0), "Should have plan hashes")
		}
	})

	// Validate PII protection
	tt.Run("PII_Protection", func(ttt *testing.T) {
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "SELECT count(*) FROM Metric WHERE query_text LIKE '%%@%%' OR query_text LIKE '%%SSN%%' SINCE %d seconds ago") {
						results
					}
				}
			}
		}`, t.nrAccountID, int(time.Since(t.testStartTime).Seconds()))

		results := t.queryNRDB(ttt, query)
		
		if len(results) > 0 {
			count, _ := results[0]["count"].(float64)
			assert.Equal(ttt, float64(0), count, "Should not have PII in metrics")
		}
	})
}

func (t *NRDBValidationTest) testDataCompleteness(tt *testing.T) {
	// Test that all expected metric types are present
	expectedMetrics := []string{
		"postgresql.database.size",
		"postgresql.backends",
		"postgresql.commits",
		"postgresql.rollbacks",
		"mysql.database.size",
		"mysql.threads",
		"mysql.questions",
		"mysql.slow_queries",
	}

	for _, metricName := range expectedMetrics {
		tt.Run(metricName, func(ttt *testing.T) {
			query := fmt.Sprintf(`{
				actor {
					account(id: %s) {
						nrql(query: "SELECT count(*) FROM Metric WHERE metricName = '%s' SINCE %d seconds ago") {
							results
						}
					}
				}
			}`, t.nrAccountID, metricName, int(time.Since(t.testStartTime).Seconds()))

			results := t.queryNRDB(ttt, query)
			
			if len(results) > 0 {
				count, _ := results[0]["count"].(float64)
				assert.Greater(ttt, count, float64(0), fmt.Sprintf("Should have %s metrics", metricName))
			}
		})
	}
}

// Helper methods

func (t *NRDBValidationTest) generatePostgreSQLLoad(tt *testing.T, db *sql.DB) {
	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test (
			id SERIAL PRIMARY KEY,
			data TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(tt, err)

	// Generate various query patterns
	for i := 0; i < 100; i++ {
		// Fast queries
		_, _ = db.Exec("SELECT 1")
		
		// Slow queries
		_, _ = db.Exec("SELECT pg_sleep(0.1)")
		
		// DML operations
		_, _ = db.Exec("INSERT INTO e2e_test (data) VALUES ($1)", fmt.Sprintf("test_data_%d", i))
		
		// Queries with PII (to test sanitization)
		_, _ = db.Exec("SELECT * FROM e2e_test WHERE data = 'user@example.com' OR data = '123-45-6789'")
	}

	// Cleanup
	defer db.Exec("DROP TABLE IF EXISTS e2e_test")
}

func (t *NRDBValidationTest) generateMySQLLoad(tt *testing.T, db *sql.DB) {
	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test (
			id INT AUTO_INCREMENT PRIMARY KEY,
			data TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(tt, err)

	// Generate various query patterns
	for i := 0; i < 100; i++ {
		// Fast queries
		_, _ = db.Exec("SELECT 1")
		
		// Slow queries
		_, _ = db.Exec("SELECT SLEEP(0.1)")
		
		// DML operations
		_, _ = db.Exec("INSERT INTO e2e_test (data) VALUES (?)", fmt.Sprintf("test_data_%d", i))
	}

	// Cleanup
	defer db.Exec("DROP TABLE IF EXISTS e2e_test")
}

func (t *NRDBValidationTest) queryNRDB(tt *testing.T, query string) []map[string]interface{} {
	client := &http.Client{Timeout: 30 * time.Second}
	
	reqBody := map[string]string{"query": query}
	jsonBody, err := json.Marshal(reqBody)
	require.NoError(tt, err)

	req, err := http.NewRequest("POST", t.nrQueryURL, bytes.NewBuffer(jsonBody))
	require.NoError(tt, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Api-Key", t.nrAPIKey)

	resp, err := client.Do(req)
	require.NoError(tt, err)
	defer resp.Body.Close()

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
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(tt, err)

	return result.Data.Actor.Account.NRQL.Results
}

func (t *NRDBValidationTest) validatePostgreSQLMetrics(tt *testing.T) {
	// Validate specific PostgreSQL metrics and their attributes
	query := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "SELECT average(value), max(value), min(value) FROM Metric WHERE metricName = 'postgresql.database.size' FACET db.name SINCE %d seconds ago") {
					results
				}
			}
		}
	}`, t.nrAccountID, int(time.Since(t.testStartTime).Seconds()))

	results := t.queryNRDB(tt, query)
	assert.NotEmpty(tt, results, "Should have database size metrics with proper faceting")
}

func (t *NRDBValidationTest) validateMySQLMetrics(tt *testing.T) {
	// Validate specific MySQL metrics and their attributes
	query := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "SELECT average(value), max(value) FROM Metric WHERE metricName = 'mysql.threads' SINCE %d seconds ago") {
					results
				}
			}
		}
	}`, t.nrAccountID, int(time.Since(t.testStartTime).Seconds()))

	results := t.queryNRDB(tt, query)
	assert.NotEmpty(tt, results, "Should have MySQL thread metrics")
}