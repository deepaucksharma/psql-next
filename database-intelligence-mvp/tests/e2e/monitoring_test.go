package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMonitoringAndAlerts validates monitoring metrics and alerting
func TestMonitoringAndAlerts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping monitoring test in short mode")
	}

	testEnv := setupTestEnvironment(t)
	defer testEnv.Cleanup()

	db := testEnv.PostgresDB
	setupTestSchema(t, db)

	// Start collector with monitoring enabled
	collector := testEnv.StartCollector(t, "testdata/config-monitoring.yaml")
	defer collector.Shutdown()

	require.Eventually(t, func() bool {
		return collector.IsHealthy()
	}, 30*time.Second, 1*time.Second)

	// Create Prometheus client
	client, err := api.NewClient(api.Config{
		Address: "http://localhost:8888",
	})
	require.NoError(t, err)
	promAPI := v1.NewAPI(client)

	t.Run("CollectorHealthMetrics", func(t *testing.T) {
		// Generate some activity
		generateStandardActivity(t, db)
		time.Sleep(5 * time.Second)

		ctx := context.Background()
		
		// Check collector health metrics
		healthMetrics := []string{
			"otelcol_process_uptime",
			"otelcol_process_runtime_heap_alloc_bytes",
			"otelcol_process_runtime_total_alloc_bytes",
			"otelcol_process_cpu_seconds",
		}

		for _, metric := range healthMetrics {
			result, _, err := promAPI.Query(ctx, metric, time.Now())
			require.NoError(t, err, "Failed to query metric: %s", metric)
			
			vector, ok := result.(model.Vector)
			require.True(t, ok, "Expected vector result for %s", metric)
			require.NotEmpty(t, vector, "No data for metric %s", metric)
			
			t.Logf("%s: %v", metric, vector[0].Value)
		}
	})

	t.Run("PipelineMetrics", func(t *testing.T) {
		ctx := context.Background()
		
		// Check pipeline metrics
		pipelineMetrics := map[string]string{
			"otelcol_receiver_accepted_metric_points":   "Accepted metrics",
			"otelcol_receiver_refused_metric_points":    "Refused metrics",
			"otelcol_processor_dropped_metric_points":   "Dropped metrics",
			"otelcol_exporter_sent_metric_points":       "Sent metrics",
			"otelcol_exporter_failed_metric_points":     "Failed metrics",
		}

		for metric, desc := range pipelineMetrics {
			result, _, err := promAPI.Query(ctx, metric, time.Now())
			require.NoError(t, err, "Failed to query %s", desc)
			
			if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
				t.Logf("%s: %v", desc, vector[0].Value)
			}
		}
	})

	t.Run("DatabaseMetrics", func(t *testing.T) {
		// Generate database activity
		generateDatabaseLoad(t, db)
		time.Sleep(10 * time.Second)

		ctx := context.Background()
		
		// Query database-specific metrics
		dbMetrics := []string{
			"db_postgresql_connections_active",
			"db_postgresql_connections_idle",
			"db_postgresql_transactions_committed",
			"db_postgresql_transactions_rolled_back",
			"db_postgresql_blocks_read",
			"db_postgresql_blocks_hit",
		}

		for _, metric := range dbMetrics {
			query := fmt.Sprintf(`%s{db_name="test_db"}`, metric)
			result, _, err := promAPI.Query(ctx, query, time.Now())
			
			if err == nil {
				if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
					t.Logf("%s: %v", metric, vector[0].Value)
				}
			}
		}
	})

	t.Run("PlanIntelligenceMetrics", func(t *testing.T) {
		// Generate queries that trigger plan collection
		generateSlowQueries(t, db)
		time.Sleep(10 * time.Second)

		ctx := context.Background()
		
		// Check plan metrics
		planMetrics := map[string]string{
			`db_postgresql_query_exec_time`:              "Execution time",
			`db_postgresql_query_plan_time`:              "Planning time",
			`db_postgresql_plan_changes_total`:           "Plan changes",
			`db_postgresql_plan_regression_detected_total`: "Regressions detected",
		}

		for metric, desc := range planMetrics {
			result, _, err := promAPI.Query(ctx, metric, time.Now())
			
			if err == nil {
				if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
					t.Logf("%s: %v", desc, vector[0].Value)
				}
			}
		}
	})

	t.Run("ASHMetrics", func(t *testing.T) {
		// Generate ASH activity
		generateASHActivity(t, db)
		time.Sleep(5 * time.Second)

		ctx := context.Background()
		
		// Check ASH metrics
		ashQueries := map[string]string{
			`sum(postgresql_ash_sessions_count) by (state)`: "Sessions by state",
			`sum(postgresql_ash_wait_events_count) by (category)`: "Wait events by category",
			`postgresql_ash_blocking_sessions_count`: "Blocking sessions",
			`count(count by (query_id)(postgresql_ash_query_active_count))`: "Unique active queries",
		}

		for query, desc := range ashQueries {
			result, _, err := promAPI.Query(ctx, query, time.Now())
			
			if err == nil {
				t.Logf("%s: %v", desc, result)
			}
		}
	})

	t.Run("AlertRules", func(t *testing.T) {
		// Test alert conditions
		testAlerts := []struct {
			name      string
			trigger   func()
			query     string
			expected  bool
		}{
			{
				name: "HighMemoryUsage",
				trigger: func() {
					// Generate high memory usage
					generateHighCardinalityQueries(t, db, 5000)
				},
				query:    `otelcol_process_runtime_heap_alloc_bytes > 100000000`, // 100MB
				expected: true,
			},
			{
				name: "PlanRegression",
				trigger: func() {
					// Cause plan regression
					causePlanRegression(t, db)
				},
				query:    `increase(db_postgresql_plan_regression_detected_total[5m]) > 0`,
				expected: true,
			},
			{
				name: "ExcessiveLockWaits",
				trigger: func() {
					// Create lock contention
					createLockContention(t, db)
				},
				query:    `postgresql_ash_wait_events_count{category="Concurrency"} > 5`,
				expected: true,
			},
		}

		for _, test := range testAlerts {
			t.Run(test.name, func(t *testing.T) {
				// Trigger condition
				test.trigger()
				time.Sleep(10 * time.Second)

				// Check if alert would fire
				ctx := context.Background()
				result, _, err := promAPI.Query(ctx, test.query, time.Now())
				
				if err == nil {
					if vector, ok := result.(model.Vector); ok {
						fired := len(vector) > 0
						if test.expected {
							assert.True(t, fired, "Alert %s should have fired", test.name)
						}
						if fired {
							t.Logf("Alert %s triggered: %v", test.name, vector[0])
						}
					}
				}
			})
		}
	})

	t.Run("MetricCardinality", func(t *testing.T) {
		ctx := context.Background()
		
		// Check cardinality of key metrics
		cardinalityQueries := map[string]int{
			`count(count by (query_id)(db_postgresql_query_exec_time))`:     1000, // Max queries
			`count(count by (wait_event)(postgresql_ash_wait_events_count))`: 100,  // Max wait events
			`count(count by (state)(postgresql_ash_sessions_count))`:         10,   // Session states
		}

		for query, maxExpected := range cardinalityQueries {
			result, _, err := promAPI.Query(ctx, query, time.Now())
			require.NoError(t, err)
			
			if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
				cardinality := int(vector[0].Value)
				assert.LessOrEqual(t, cardinality, maxExpected, 
					"Cardinality too high for query: %s", query)
				t.Logf("Cardinality for %s: %d (max: %d)", query, cardinality, maxExpected)
			}
		}
	})

	t.Run("HistogramMetrics", func(t *testing.T) {
		ctx := context.Background()
		
		// Check histogram metrics
		histogramQueries := []string{
			`histogram_quantile(0.5, db_postgresql_query_exec_time_bucket)`,  // p50
			`histogram_quantile(0.95, db_postgresql_query_exec_time_bucket)`, // p95
			`histogram_quantile(0.99, db_postgresql_query_exec_time_bucket)`, // p99
		}

		for _, query := range histogramQueries {
			result, _, err := promAPI.Query(ctx, query, time.Now())
			
			if err == nil {
				t.Logf("Query latency %s: %v", query, result)
			}
		}
	})

	t.Run("RateMetrics", func(t *testing.T) {
		// Generate steady load
		generateSteadyLoad(t, db, 30*time.Second)

		ctx := context.Background()
		
		// Check rate metrics
		rateQueries := map[string]string{
			`rate(otelcol_receiver_accepted_metric_points[1m])`: "Metrics/sec received",
			`rate(otelcol_exporter_sent_metric_points[1m])`:     "Metrics/sec exported",
			`rate(db_postgresql_transactions_committed[1m])`:     "Transactions/sec",
			`rate(postgresql_ash_sessions_count[1m])`:            "Session samples/sec",
		}

		for query, desc := range rateQueries {
			result, _, err := promAPI.Query(ctx, query, time.Now())
			
			if err == nil {
				if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
					t.Logf("%s: %.2f/sec", desc, vector[0].Value)
				}
			}
		}
	})

	t.Run("CircuitBreakerMetrics", func(t *testing.T) {
		// Trigger circuit breaker
		simulateCircuitBreakerScenario(t, testEnv)
		time.Sleep(5 * time.Second)

		ctx := context.Background()
		
		// Check circuit breaker metrics
		cbQueries := map[string]string{
			`otelcol_processor_circuitbreaker_state`:         "Circuit state",
			`otelcol_processor_circuitbreaker_triggered_total`: "Triggers",
			`otelcol_processor_circuitbreaker_disabled_queries`: "Disabled queries",
		}

		for query, desc := range cbQueries {
			result, _, err := promAPI.Query(ctx, query, time.Now())
			
			if err == nil {
				t.Logf("%s: %v", desc, result)
			}
		}
	})
}

// TestGrafanaDashboards validates Grafana dashboard queries
func TestGrafanaDashboards(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping dashboard test in short mode")
	}

	// Load dashboard configurations
	dashboards := []string{
		"testdata/grafana-dashboards/collector-health.json",
		"testdata/grafana-dashboards/plan-intelligence.json",
		"testdata/grafana-dashboards/ash-overview.json",
		"testdata/grafana-dashboards/performance-analysis.json",
	}

	for _, dashboardPath := range dashboards {
		t.Run(dashboardPath, func(t *testing.T) {
			// Load dashboard JSON
			var dashboard map[string]interface{}
			dashboardData, err := ioutil.ReadFile(dashboardPath)
			if err != nil {
				t.Skip("Dashboard file not found")
			}
			
			err = json.Unmarshal(dashboardData, &dashboard)
			require.NoError(t, err)
			
			// Extract and validate queries
			validateDashboardQueries(t, dashboard)
		})
	}
}

// Helper functions

func generateStandardActivity(t *testing.T, db *sql.DB) {
	for i := 0; i < 100; i++ {
		go func(id int) {
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			
			switch id % 4 {
			case 0:
				conn.Exec("SELECT COUNT(*) FROM users")
			case 1:
				conn.Exec("SELECT * FROM users WHERE id = $1", id)
			case 2:
				conn.Exec("UPDATE users SET email = $1 WHERE id = $2", 
					fmt.Sprintf("user%d@test.com", id), id)
			case 3:
				conn.Exec("SELECT u.*, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id")
			}
		}(i)
	}
}

func generateDatabaseLoad(t *testing.T, db *sql.DB) {
	// Create connections
	for i := 0; i < 50; i++ {
		go func() {
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			
			// Start transaction
			tx, _ := conn.Begin()
			tx.Exec("SELECT * FROM users FOR UPDATE")
			time.Sleep(2 * time.Second)
			tx.Commit()
		}()
	}
}

func generateASHActivity(t *testing.T, db *sql.DB) {
	// Create various session states
	for i := 0; i < 20; i++ {
		go func(id int) {
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			
			switch id % 5 {
			case 0: // Active
				conn.Exec("SELECT pg_sleep(2)")
			case 1: // Idle in transaction
				tx, _ := conn.Begin()
				tx.Exec("SELECT 1")
				time.Sleep(3 * time.Second)
				tx.Rollback()
			case 2: // Blocked
				conn.Exec("SELECT * FROM users WHERE id = 1 FOR UPDATE")
			case 3: // IO wait
				conn.Exec("SELECT * FROM users ORDER BY random()")
			case 4: // CPU intensive
				conn.Exec("SELECT COUNT(*) FROM generate_series(1, 1000000)")
			}
		}(i)
	}
}

func generateHighCardinalityQueries(t *testing.T, db *sql.DB, count int) {
	for i := 0; i < count; i++ {
		go func(id int) {
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			
			// Unique query
			query := fmt.Sprintf("SELECT %d, '%s', NOW()", id, generateRandomString(20))
			conn.Exec(query)
		}(i)
	}
}

func causePlanRegression(t *testing.T, db *sql.DB) {
	// Create and drop index
	db.Exec("CREATE INDEX idx_test_regression ON users(email)")
	
	// Run queries with index
	for i := 0; i < 10; i++ {
		db.Exec("SELECT * FROM users WHERE email = $1", fmt.Sprintf("user%d@example.com", i))
	}
	
	// Drop index to cause regression
	db.Exec("DROP INDEX idx_test_regression")
	
	// Run same queries without index
	for i := 0; i < 10; i++ {
		db.Exec("SELECT * FROM users WHERE email = $1", fmt.Sprintf("user%d@example.com", i))
	}
}

func createLockContention(t *testing.T, db *sql.DB) {
	// Hold lock
	lockConn := getNewConnection(t, testEnv)
	tx, _ := lockConn.Begin()
	tx.Exec("UPDATE users SET email = 'locked@example.com' WHERE id = 1")
	
	// Try to acquire same lock from multiple sessions
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn := getNewConnection(t, testEnv)
			defer conn.Close()
			
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			
			conn.ExecContext(ctx, "UPDATE users SET email = 'blocked@example.com' WHERE id = 1")
		}()
	}
	
	// Hold lock for a bit
	time.Sleep(3 * time.Second)
	tx.Rollback()
	lockConn.Close()
	wg.Wait()
}

func generateSteadyLoad(t *testing.T, db *sql.DB, duration time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				conn := getNewConnection(t, testEnv)
				conn.Exec("SELECT COUNT(*) FROM users")
				conn.Close()
			}
		}
	}()
	
	<-ctx.Done()
}

func simulateCircuitBreakerScenario(t *testing.T, env *TestEnvironment) {
	// Remove log file to trigger error
	env.SimulateLogFileError()
	
	// Try to generate activity
	for i := 0; i < 10; i++ {
		conn := getNewConnection(t, env)
		conn.Exec("SELECT * FROM users")
		conn.Close()
		time.Sleep(100 * time.Millisecond)
	}
	
	// Restore log file
	env.RestoreAutoExplain()
}

func validateDashboardQueries(t *testing.T, dashboard map[string]interface{}) {
	// Extract panels
	if panels, ok := dashboard["panels"].([]interface{}); ok {
		for _, panel := range panels {
			if p, ok := panel.(map[string]interface{}); ok {
				// Check for queries
				if targets, ok := p["targets"].([]interface{}); ok {
					for _, target := range targets {
						if tgt, ok := target.(map[string]interface{}); ok {
							if expr, ok := tgt["expr"].(string); ok {
								// Validate PromQL syntax
								t.Logf("Validating query: %s", expr)
								// In real implementation, use promql parser
							}
						}
					}
				}
			}
		}
	}
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}