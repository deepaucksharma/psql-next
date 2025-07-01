package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NRDBClient represents a client for querying New Relic
type NRDBClient struct {
	accountID   string
	apiKey      string
	queryURL    string
	httpClient  *http.Client
}

// NewNRDBClient creates a new NRDB client
func NewNRDBClient(accountID, apiKey string) *NRDBClient {
	return &NRDBClient{
		accountID:  accountID,
		apiKey:     apiKey,
		queryURL:   fmt.Sprintf("https://api.newrelic.com/graphql"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryResult represents NRQL query results
type QueryResult struct {
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

// ExecuteNRQL executes an NRQL query
func (c *NRDBClient) ExecuteNRQL(ctx context.Context, query string) (*QueryResult, error) {
	graphQLQuery := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "%s") {
					results
				}
			}
		}
	}`, c.accountID, strings.ReplaceAll(query, `"`, `\"`))

	reqBody := map[string]string{"query": graphQLQuery}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.queryURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result QueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("NRQL error: %s", result.Errors[0].Message)
	}

	return &result, nil
}

// TestNRQLDashboardQueries validates all NRQL queries for New Relic dashboards
func TestNRQLDashboardQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping NRDB validation test in short mode")
	}

	// Check for New Relic credentials
	accountID := getEnvOrSkip(t, "NEW_RELIC_ACCOUNT_ID")
	apiKey := getEnvOrSkip(t, "NEW_RELIC_API_KEY")

	testEnv := setupTestEnvironment(t)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupTestSchema(t, db)

	// Start collector with New Relic export
	collector := testEnv.StartCollector(t, "testdata/config-newrelic.yaml")
	defer collector.Shutdown()

	// Generate comprehensive test data
	generateComprehensiveTestData(t, db, testEnv)

	// Wait for data to reach New Relic
	time.Sleep(30 * time.Second)

	// Create NRDB client
	nrdbClient := NewNRDBClient(accountID, apiKey)
	ctx := context.Background()

	t.Run("PostgreSQLOverviewDashboard", func(t *testing.T) {
		queries := map[string]string{
			"ActiveConnections": `
				SELECT latest(postgresql.connections.active) 
				FROM Metric 
				WHERE db.system = 'postgresql' 
				FACET db.name 
				SINCE 5 minutes ago`,

			"TransactionRate": `
				SELECT rate(sum(postgresql.transactions.committed), 1 minute) as 'Commits/min',
					   rate(sum(postgresql.transactions.rolled_back), 1 minute) as 'Rollbacks/min'
				FROM Metric 
				WHERE db.system = 'postgresql' 
				TIMESERIES 
				SINCE 30 minutes ago`,

			"CacheHitRatio": `
				SELECT (sum(postgresql.blocks.hit) / (sum(postgresql.blocks.hit) + sum(postgresql.blocks.read))) * 100 as 'Cache Hit %'
				FROM Metric 
				WHERE db.system = 'postgresql' 
				FACET db.name 
				SINCE 1 hour ago`,

			"DatabaseSize": `
				SELECT latest(postgresql.database.size) / 1024 / 1024 as 'Size (MB)'
				FROM Metric 
				WHERE db.system = 'postgresql' 
				FACET db.name`,

			"TopQueries": `
				SELECT count(*), average(query.exec_time_ms) 
				FROM Metric 
				WHERE metricName = 'postgresql.query.execution' 
				FACET query.normalized 
				LIMIT 10 
				SINCE 1 hour ago`,
		}

		validateNRQLQueries(t, ctx, nrdbClient, queries)
	})

	t.Run("PlanIntelligenceDashboard", func(t *testing.T) {
		queries := map[string]string{
			"PlanChanges": `
				SELECT count(*) 
				FROM Metric 
				WHERE metricName = 'postgresql.plan.change' 
				FACET query.normalized, plan.change_type 
				SINCE 1 hour ago`,

			"PlanRegressions": `
				SELECT count(*) as 'Regressions', 
					   average(plan.cost_increase_ratio) as 'Avg Cost Increase'
				FROM Metric 
				WHERE metricName = 'postgresql.plan.regression' 
				TIMESERIES 
				SINCE 2 hours ago`,

			"QueryPerformanceTrend": `
				SELECT average(query.exec_time_ms), 
					   average(query.plan_time_ms),
					   percentile(query.exec_time_ms, 95) as 'p95 Exec Time'
				FROM Metric 
				WHERE metricName = 'postgresql.query.execution' 
				FACET query.normalized 
				TIMESERIES 
				SINCE 3 hours ago`,

			"TopRegressions": `
				SELECT query.normalized, 
					   plan.old_cost, 
					   plan.new_cost,
					   plan.cost_increase_ratio,
					   plan.performance_impact
				FROM Metric 
				WHERE metricName = 'postgresql.plan.regression' 
				ORDER BY plan.cost_increase_ratio DESC 
				LIMIT 20 
				SINCE 24 hours ago`,

			"PlanNodeAnalysis": `
				SELECT count(*) 
				FROM Metric 
				WHERE metricName = 'postgresql.plan.node' 
				FACET plan.node_type, plan.issue_type 
				SINCE 1 hour ago`,

			"QueryPlanDistribution": `
				SELECT uniqueCount(plan.hash) as 'Unique Plans',
					   count(*) as 'Total Executions'
				FROM Metric 
				WHERE metricName = 'postgresql.query.execution' AND plan.hash IS NOT NULL
				FACET query.normalized 
				SINCE 6 hours ago`,
		}

		validateNRQLQueries(t, ctx, nrdbClient, queries)
	})

	t.Run("ASHDashboard", func(t *testing.T) {
		queries := map[string]string{
			"ActiveSessionsOverTime": `
				SELECT count(*) 
				FROM Metric 
				WHERE metricName = 'postgresql.ash.session' 
				FACET session.state 
				TIMESERIES 
				SINCE 30 minutes ago`,

			"WaitEventDistribution": `
				SELECT sum(wait.duration_ms) 
				FROM Metric 
				WHERE metricName = 'postgresql.ash.wait_event' 
				FACET wait.event_type, wait.event_name 
				SINCE 1 hour ago`,

			"TopWaitEvents": `
				SELECT sum(wait.duration_ms) as 'Total Wait Time',
					   count(*) as 'Wait Count',
					   average(wait.duration_ms) as 'Avg Wait'
				FROM Metric 
				WHERE metricName = 'postgresql.ash.wait_event' 
				FACET wait.event_name 
				ORDER BY sum(wait.duration_ms) DESC 
				LIMIT 10 
				SINCE 1 hour ago`,

			"BlockingAnalysis": `
				SELECT blocking.query as 'Blocking Query',
					   blocked.query as 'Blocked Query',
					   count(*) as 'Block Count',
					   max(block.duration_ms) as 'Max Block Duration'
				FROM Metric 
				WHERE metricName = 'postgresql.ash.blocking' 
				FACET blocking.pid, blocked.pid 
				SINCE 30 minutes ago`,

			"SessionActivity": `
				SELECT uniqueCount(session.pid) as 'Unique Sessions',
					   count(*) as 'Total Samples'
				FROM Metric 
				WHERE metricName = 'postgresql.ash.session' AND session.state = 'active'
				FACET query.normalized 
				TIMESERIES 
				SINCE 1 hour ago`,

			"ResourceUtilization": `
				SELECT average(session.cpu_usage) as 'CPU %',
					   average(session.memory_mb) as 'Memory MB',
					   sum(session.io_wait_ms) as 'IO Wait'
				FROM Metric 
				WHERE metricName = 'postgresql.ash.session' 
				FACET session.backend_type 
				SINCE 30 minutes ago`,
		}

		validateNRQLQueries(t, ctx, nrdbClient, queries)
	})

	t.Run("IntegratedIntelligenceDashboard", func(t *testing.T) {
		queries := map[string]string{
			"QueryPerformanceWithWaits": `
				SELECT average(query.exec_time_ms) as 'Exec Time',
					   sum(wait.duration_ms) as 'Wait Time',
					   count(DISTINCT plan.hash) as 'Plan Count'
				FROM Metric 
				WHERE metricName IN ('postgresql.query.execution', 'postgresql.ash.wait_event')
				FACET query.normalized 
				SINCE 2 hours ago`,

			"PlanRegressionImpact": `
				SELECT plan.regression_detected as 'Has Regression',
					   average(session.count) as 'Active Sessions',
					   sum(wait.duration_ms) as 'Total Wait'
				FROM Metric 
				WHERE query.normalized IS NOT NULL
				FACET query.normalized 
				SINCE 1 hour ago`,

			"QueryHealthScore": `
				SELECT query.normalized,
					   (100 - (plan.regression_count * 10 + 
					   wait.excessive_count * 5 + 
					   (query.exec_time_ms / 100))) as 'Health Score'
				FROM Metric 
				WHERE metricName = 'postgresql.query.health' 
				ORDER BY 'Health Score' ASC 
				LIMIT 20 
				SINCE 24 hours ago`,

			"AdaptiveSamplingEffectiveness": `
				SELECT sampling.rule as 'Rule',
					   sampling.rate as 'Sample Rate',
					   count(*) as 'Samples Collected',
					   uniqueCount(query.normalized) as 'Unique Queries'
				FROM Metric 
				WHERE metricName = 'postgresql.adaptive_sampling' 
				FACET sampling.rule 
				SINCE 1 hour ago`,
		}

		validateNRQLQueries(t, ctx, nrdbClient, queries)
	})

	t.Run("AlertingQueries", func(t *testing.T) {
		alertQueries := map[string]string{
			"HighPlanRegressionRate": `
				SELECT count(*) 
				FROM Metric 
				WHERE metricName = 'postgresql.plan.regression' 
				SINCE 5 minutes ago`,

			"ExcessiveLockWaits": `
				SELECT sum(wait.duration_ms) 
				FROM Metric 
				WHERE metricName = 'postgresql.ash.wait_event' 
				AND wait.event_type = 'Lock' 
				SINCE 5 minutes ago`,

			"QueryPerformanceDegradation": `
				SELECT percentile(query.exec_time_ms, 95) 
				FROM Metric 
				WHERE metricName = 'postgresql.query.execution' 
				FACET query.normalized 
				SINCE 10 minutes ago 
				COMPARE WITH 1 hour ago`,

			"DatabaseConnectionSaturation": `
				SELECT (latest(postgresql.connections.active) / 
						latest(postgresql.connections.max)) * 100 as 'Connection Usage %'
				FROM Metric 
				WHERE db.system = 'postgresql' 
				FACET db.name`,

			"CircuitBreakerActivation": `
				SELECT count(*) 
				FROM Metric 
				WHERE metricName = 'otelcol.processor.circuitbreaker.triggered' 
				SINCE 5 minutes ago`,
		}

		// Validate alert queries return valid results
		for name, query := range alertQueries {
			t.Run(name, func(t *testing.T) {
				result, err := nrdbClient.ExecuteNRQL(ctx, query)
				require.NoError(t, err, "Alert query %s failed", name)
				assert.NotNil(t, result.Data.Actor.Account.NRQL.Results, 
					"Alert query %s returned no results", name)
			})
		}
	})

	t.Run("DataIntegrityValidation", func(t *testing.T) {
		// Verify required attributes are present
		integrityQueries := map[string]string{
			"QueryAttributes": `
				SELECT uniques(query.normalized), 
					   uniques(query.database),
					   uniques(query.user),
					   uniques(query.application_name)
				FROM Metric 
				WHERE metricName = 'postgresql.query.execution' 
				SINCE 1 hour ago`,

			"PlanAttributes": `
				SELECT uniques(plan.hash),
					   uniques(plan.node_type),
					   uniques(plan.anonymized)
				FROM Metric 
				WHERE metricName LIKE 'postgresql.plan%' 
				SINCE 1 hour ago`,

			"ASHAttributes": `
				SELECT uniques(session.state),
					   uniques(wait.event_type),
					   uniques(session.backend_type)
				FROM Metric 
				WHERE metricName LIKE 'postgresql.ash%' 
				SINCE 1 hour ago`,
		}

		for name, query := range integrityQueries {
			t.Run(name, func(t *testing.T) {
				result, err := nrdbClient.ExecuteNRQL(ctx, query)
				require.NoError(t, err, "Integrity query %s failed", name)
				
				results := result.Data.Actor.Account.NRQL.Results
				require.NotEmpty(t, results, "No results for integrity check %s", name)
				
				// Verify attributes have values
				for key, value := range results[0] {
					assert.NotNil(t, value, "Attribute %s is nil in %s", key, name)
					if arr, ok := value.([]interface{}); ok {
						assert.NotEmpty(t, arr, "Attribute %s is empty in %s", key, name)
					}
				}
			})
		}
	})
}

// validateNRQLQueries validates a set of NRQL queries
func validateNRQLQueries(t *testing.T, ctx context.Context, client *NRDBClient, queries map[string]string) {
	for name, query := range queries {
		t.Run(name, func(t *testing.T) {
			// Clean up query formatting
			query = strings.TrimSpace(query)
			query = strings.ReplaceAll(query, "\n", " ")
			query = strings.ReplaceAll(query, "\t", " ")
			query = strings.Join(strings.Fields(query), " ")

			// Execute query
			result, err := client.ExecuteNRQL(ctx, query)
			require.NoError(t, err, "Query failed: %s\nNRQL: %s", name, query)

			// Validate results
			results := result.Data.Actor.Account.NRQL.Results
			assert.NotNil(t, results, "Query returned nil results: %s", name)
			
			// Log results for debugging
			if len(results) > 0 {
				t.Logf("%s returned %d results", name, len(results))
				if len(results) <= 5 {
					for i, r := range results {
						t.Logf("  Result %d: %+v", i, r)
					}
				}
			} else {
				t.Logf("%s returned no results (might be normal for new data)", name)
			}
		})
	}
}

// generateComprehensiveTestData generates test data for NRQL validation
func generateComprehensiveTestData(t *testing.T, db *sql.DB, testEnv *TestEnvironment) {
	// Generate various query patterns
	queries := []string{
		"SELECT * FROM users WHERE id = $1",
		"SELECT u.*, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id",
		"UPDATE users SET last_login = NOW() WHERE id = $1",
		"INSERT INTO orders (user_id, total) VALUES ($1, $2)",
		"DELETE FROM old_sessions WHERE created_at < NOW() - INTERVAL '1 day'",
		"SELECT * FROM users ORDER BY created_at DESC LIMIT 100",
		"SELECT COUNT(*) FROM users WHERE email LIKE '%@example.com'",
	}

	// Generate load with different patterns
	for i := 0; i < 100; i++ {
		query := queries[i%len(queries)]
		
		// Execute with different parameters to create plan variations
		if strings.Contains(query, "$1") {
			db.Exec(query, i%10)
		} else {
			db.Exec(query)
		}
		
		// Create some slow queries
		if i%20 == 0 {
			db.Exec("SELECT pg_sleep(0.5)")
		}
		
		// Create lock contention
		if i%30 == 0 {
			go func() {
				tx, _ := db.Begin()
				tx.Exec("SELECT * FROM users WHERE id = 1 FOR UPDATE")
				time.Sleep(2 * time.Second)
				tx.Rollback()
			}()
		}
	}

	// Create plan regression scenario
	db.Exec("CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)")
	for i := 0; i < 10; i++ {
		db.Exec("SELECT * FROM users WHERE email = $1", fmt.Sprintf("user%d@example.com", i))
	}
	db.Exec("DROP INDEX IF EXISTS idx_users_email")
	for i := 0; i < 10; i++ {
		db.Exec("SELECT * FROM users WHERE email = $1", fmt.Sprintf("user%d@example.com", i))
	}

	// Generate ASH activity
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn := getNewConnection(t, testEnv)
			defer conn.Close()

			switch id % 4 {
			case 0:
				// CPU intensive
				conn.Exec("SELECT COUNT(*) FROM generate_series(1, 1000000)")
			case 1:
				// IO intensive
				conn.Exec("SELECT * FROM users ORDER BY random() LIMIT 1000")
			case 2:
				// Lock wait
				conn.Exec("UPDATE users SET email = $1 WHERE id = 1", fmt.Sprintf("blocked%d@example.com", id))
			case 3:
				// Idle in transaction
				tx, _ := conn.Begin()
				tx.Exec("SELECT 1")
				time.Sleep(3 * time.Second)
				tx.Commit()
			}
		}(i)
	}
	wg.Wait()
}

// getEnvOrSkip gets environment variable or skips test
func getEnvOrSkip(t *testing.T, key string) string {
	value := os.Getenv(key)
	if value == "" {
		t.Skipf("Skipping test: %s not set", key)
	}
	return value
}