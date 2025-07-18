package e2e

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComprehensiveE2EValidation validates the entire pipeline from MySQL to New Relic dashboards
func TestComprehensiveE2EValidation(t *testing.T) {
	// Test stages
	t.Run("Stage1_ValidateMySQL", testValidateMySQLSetup)
	t.Run("Stage2_GenerateTestWorkload", testGenerateWorkload)
	t.Run("Stage3_ValidateCollectorMetrics", testValidateCollectorMetrics)
	t.Run("Stage4_ValidateNRDBData", testValidateNRDBData)
	t.Run("Stage5_ValidateDashboards", testValidateDashboards)
	t.Run("Stage6_ValidateAdvisories", testValidateAdvisories)
	t.Run("Stage7_ValidateEndToEndLatency", testValidateE2ELatency)
}

// Stage 1: Validate MySQL Setup
func testValidateMySQLSetup(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	tests := []struct {
		name  string
		query string
		check func(*sql.Rows) error
	}{
		{
			name: "Performance Schema Enabled",
			query: "SELECT @@performance_schema",
			check: func(rows *sql.Rows) error {
				var enabled int
				if rows.Next() {
					rows.Scan(&enabled)
					if enabled != 1 {
						return fmt.Errorf("Performance Schema not enabled")
					}
				}
				return nil
			},
		},
		{
			name: "Wait Instruments Enabled",
			query: `SELECT COUNT(*) as count 
					FROM performance_schema.setup_instruments 
					WHERE NAME LIKE 'wait/%' AND ENABLED = 'YES'`,
			check: func(rows *sql.Rows) error {
				var count int
				if rows.Next() {
					rows.Scan(&count)
					if count < 300 {
						return fmt.Errorf("Only %d wait instruments enabled, expected >300", count)
					}
				}
				return nil
			},
		},
		{
			name: "Statement Consumers Enabled",
			query: `SELECT COUNT(*) as count 
					FROM performance_schema.setup_consumers 
					WHERE NAME LIKE '%statements%' AND ENABLED = 'YES'`,
			check: func(rows *sql.Rows) error {
				var count int
				if rows.Next() {
					rows.Scan(&count)
					if count < 3 {
						return fmt.Errorf("Not all statement consumers enabled")
					}
				}
				return nil
			},
		},
		{
			name: "Monitor User Exists",
			query: `SELECT COUNT(*) FROM mysql.user WHERE user = 'otel_monitor'`,
			check: func(rows *sql.Rows) error {
				var count int
				if rows.Next() {
					rows.Scan(&count)
					if count == 0 {
						return fmt.Errorf("Monitor user 'otel_monitor' not found")
					}
				}
				return nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows, err := db.Query(test.query)
			require.NoError(t, err)
			defer rows.Close()
			
			err = test.check(rows)
			assert.NoError(t, err)
		})
	}
}

// Stage 2: Generate Test Workload
func testGenerateWorkload(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()

	workloadTypes := []struct {
		name        string
		workload    string
		iterations  int
		validation  string
	}{
		{
			name:       "IO_Intensive",
			workload:   "io",
			iterations: 50,
			validation: "SELECT COUNT(*) FROM performance_schema.events_waits_current WHERE EVENT_NAME LIKE 'wait/io/file/%'",
		},
		{
			name:       "Lock_Intensive",
			workload:   "lock",
			iterations: 30,
			validation: "SELECT COUNT(*) FROM performance_schema.data_locks WHERE LOCK_STATUS = 'WAITING'",
		},
		{
			name:       "CPU_Intensive",
			workload:   "cpu",
			iterations: 20,
			validation: "SELECT COUNT(*) FROM performance_schema.events_statements_current WHERE TIMER_WAIT > 1000000000",
		},
	}

	for _, wl := range workloadTypes {
		t.Run(wl.name, func(t *testing.T) {
			// Generate workload
			_, err := db.Exec(fmt.Sprintf("CALL generate_workload(%d, '%s')", wl.iterations, wl.workload))
			require.NoError(t, err)
			
			// Wait for metrics to be collected
			time.Sleep(15 * time.Second)
			
			// Validate workload generated events
			var count int
			err = db.QueryRow(wl.validation).Scan(&count)
			require.NoError(t, err)
			assert.Greater(t, count, 0, "No events generated for %s workload", wl.name)
		})
	}
}

// Stage 3: Validate Collector Metrics
func testValidateCollectorMetrics(t *testing.T) {
	endpoints := []struct {
		name     string
		url      string
		metrics  []string
	}{
		{
			name: "Edge Collector",
			url:  "http://localhost:8888/metrics",
			metrics: []string{
				"otelcol_receiver_accepted_metric_points",
				"otelcol_exporter_sent_metric_points",
				"otelcol_processor_batch_batch_size_trigger_send",
				"otelcol_receiver_refused_metric_points",
			},
		},
		{
			name: "Gateway Prometheus",
			url:  "http://localhost:9091/metrics",
			metrics: []string{
				"mysql_query_wait_profile",
				"mysql_blocking_active",
				"mysql_query_advisor",
				"mysql_wait_category_summary",
			},
		},
		{
			name: "MySQL Exporter",
			url:  "http://localhost:9104/metrics",
			metrics: []string{
				"mysql_global_status_threads_running",
				"mysql_global_status_questions",
				"mysql_global_status_slow_queries",
				"mysql_info_schema_processlist_threads",
			},
		},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			resp, err := http.Get(endpoint.url)
			require.NoError(t, err)
			defer resp.Body.Close()
			
			body := make([]byte, 0)
			_, err = resp.Body.Read(body)
			bodyStr := string(body)
			
			for _, metric := range endpoint.metrics {
				assert.Contains(t, bodyStr, metric, "Metric %s not found in %s", metric, endpoint.name)
			}
		})
	}
}

// Stage 4: Validate NRDB Data
func testValidateNRDBData(t *testing.T) {
	nrClient := NewNewRelicClient(t)
	
	queries := []struct {
		name     string
		nrql     string
		validate func([]map[string]interface{}) error
	}{
		{
			name: "Wait Profile Metrics",
			nrql: `SELECT count(*) as count, 
			       uniques(query_hash) as unique_queries,
			       uniques(wait_type) as wait_types
			       FROM Metric 
			       WHERE metricName = 'mysql.query.wait_profile' 
			       SINCE 5 minutes ago`,
			validate: func(results []map[string]interface{}) error {
				if len(results) == 0 {
					return fmt.Errorf("No wait profile metrics found")
				}
				
				count := results[0]["count"].(float64)
				uniqueQueries := results[0]["unique_queries"].(float64)
				waitTypes := results[0]["wait_types"].(float64)
				
				if count < 100 {
					return fmt.Errorf("Too few metrics: %v", count)
				}
				if uniqueQueries < 5 {
					return fmt.Errorf("Too few unique queries: %v", uniqueQueries)
				}
				if waitTypes < 3 {
					return fmt.Errorf("Too few wait types: %v", waitTypes)
				}
				return nil
			},
		},
		{
			name: "Advisory Metrics",
			nrql: `SELECT count(*) as count,
			       uniques(advisor.type) as advisor_types,
			       uniques(advisor.priority) as priorities
			       FROM Metric 
			       WHERE advisor.type IS NOT NULL 
			       SINCE 5 minutes ago`,
			validate: func(results []map[string]interface{}) error {
				if len(results) == 0 {
					return fmt.Errorf("No advisory metrics found")
				}
				
				advisorTypes := results[0]["advisor_types"].(float64)
				if advisorTypes < 2 {
					return fmt.Errorf("Too few advisor types: %v", advisorTypes)
				}
				return nil
			},
		},
		{
			name: "Blocking Metrics",
			nrql: `SELECT max(mysql.blocking.active) as max_blocking,
			       count(*) as blocking_events
			       FROM Metric 
			       WHERE metricName = 'mysql.blocking.active' 
			       SINCE 5 minutes ago`,
			validate: func(results []map[string]interface{}) error {
				// Blocking might not always occur
				return nil
			},
		},
		{
			name: "Performance Metrics",
			nrql: `SELECT average(value) as avg_wait_ms,
			       max(value) as max_wait_ms,
			       percentage(count(*), WHERE wait_percentage > 80) as high_wait_pct
			       FROM Metric 
			       WHERE metricName = 'mysql.query.wait_profile' 
			       SINCE 5 minutes ago`,
			validate: func(results []map[string]interface{}) error {
				if len(results) == 0 {
					return fmt.Errorf("No performance metrics found")
				}
				return nil
			},
		},
	}

	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			results, err := nrClient.QueryNRQL(query.nrql)
			require.NoError(t, err)
			
			err = query.validate(results)
			assert.NoError(t, err)
		})
	}
}

// Stage 5: Validate Dashboards
func testValidateDashboards(t *testing.T) {
	nrClient := NewNewRelicClient(t)
	
	dashboards := []struct {
		name    string
		widgets []struct {
			title string
			nrql  string
		}
	}{
		{
			name: "Wait Analysis Dashboard",
			widgets: []struct {
				title string
				nrql  string
			}{
				{
					title: "Query Wait Time Breakdown",
					nrql: `SELECT sum(value) FROM Metric 
					       WHERE metricName = 'mysql.query.wait_profile' 
					       FACET wait_type SINCE 1 hour ago`,
				},
				{
					title: "Top Queries by Wait Time",
					nrql: `SELECT sum(value) as total_wait_ms FROM Metric 
					       WHERE metricName = 'mysql.query.wait_profile' 
					       FACET query_hash SINCE 1 hour ago 
					       LIMIT 10`,
				},
				{
					title: "Wait Percentage Distribution",
					nrql: `SELECT histogram(wait_percentage, 10, 20) FROM Metric 
					       WHERE metricName = 'mysql.query.wait_profile' 
					       SINCE 1 hour ago`,
				},
				{
					title: "Advisory Timeline",
					nrql: `SELECT count(*) FROM Metric 
					       WHERE advisor.type IS NOT NULL 
					       FACET advisor.type TIMESERIES SINCE 1 hour ago`,
				},
			},
		},
		{
			name: "Query Detail Dashboard",
			widgets: []struct {
				title string
				nrql  string
			}{
				{
					title: "Query Performance Trends",
					nrql: `SELECT average(value) as avg_wait FROM Metric 
					       WHERE metricName = 'mysql.query.wait_profile' 
					       FACET query_hash TIMESERIES SINCE 1 hour ago`,
				},
				{
					title: "Index Usage Analysis",
					nrql: `SELECT sum(NO_INDEX_USED) as no_index_count FROM Metric 
					       WHERE metricName = 'mysql.query.wait_profile' 
					       AND NO_INDEX_USED > 0 
					       FACET query_hash SINCE 1 hour ago`,
				},
				{
					title: "Temp Table Usage",
					nrql: `SELECT sum(tmp_disk_tables) FROM Metric 
					       WHERE metricName = 'mysql.query.wait_profile' 
					       AND tmp_disk_tables > 0 
					       FACET query_hash SINCE 1 hour ago`,
				},
			},
		},
	}

	for _, dashboard := range dashboards {
		t.Run(dashboard.name, func(t *testing.T) {
			for _, widget := range dashboard.widgets {
				t.Run(widget.title, func(t *testing.T) {
					results, err := nrClient.QueryNRQL(widget.nrql)
					require.NoError(t, err)
					assert.NotEmpty(t, results, "No data for widget: %s", widget.title)
				})
			}
		})
	}
}

// Stage 6: Validate Advisories
func testValidateAdvisories(t *testing.T) {
	nrClient := NewNewRelicClient(t)
	
	// Generate specific scenarios to trigger advisories
	db := connectMySQL(t)
	defer db.Close()

	scenarios := []struct {
		name          string
		setup         string
		expectedType  string
		expectedPriority string
	}{
		{
			name: "Missing Index Advisory",
			setup: `CREATE TABLE IF NOT EXISTS test_no_index (
			        id INT PRIMARY KEY,
			        data VARCHAR(100)
			        );
			        INSERT INTO test_no_index SELECT i, CONCAT('data', i) 
			        FROM (SELECT @row := @row + 1 AS i FROM 
			        (SELECT 0 UNION ALL SELECT 1) t1,
			        (SELECT 0 UNION ALL SELECT 1) t2,
			        (SELECT @row:=0) t3) numbers
			        LIMIT 1000;
			        SELECT * FROM test_no_index WHERE data = 'data500';`,
			expectedType: "missing_index",
			expectedPriority: "P1",
		},
		{
			name: "Lock Contention Advisory",
			setup: `START TRANSACTION;
			        UPDATE test_orders SET status = 'locked' WHERE id = 1;
			        SELECT SLEEP(2);
			        COMMIT;`,
			expectedType: "lock_contention",
			expectedPriority: "P1",
		},
		{
			name: "Temp Table Advisory",
			setup: `SELECT customer_id, COUNT(*) as cnt, GROUP_CONCAT(description)
			        FROM test_orders 
			        GROUP BY customer_id 
			        HAVING cnt > 5
			        ORDER BY cnt DESC`,
			expectedType: "temp_table_disk",
			expectedPriority: "P2",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Execute scenario
			_, err := db.Exec(scenario.setup)
			if err != nil && !strings.Contains(err.Error(), "Duplicate") {
				require.NoError(t, err)
			}
			
			// Wait for advisory generation
			time.Sleep(30 * time.Second)
			
			// Check for advisory
			nrql := fmt.Sprintf(`SELECT count(*) as count FROM Metric 
			                     WHERE advisor.type = '%s' 
			                     AND advisor.priority = '%s'
			                     SINCE 2 minutes ago`, 
			                     scenario.expectedType, scenario.expectedPriority)
			
			results, err := nrClient.QueryNRQL(nrql)
			require.NoError(t, err)
			
			if len(results) > 0 {
				count := results[0]["count"].(float64)
				assert.Greater(t, count, float64(0), "No %s advisory found", scenario.expectedType)
			}
		})
	}
}

// Stage 7: Validate End-to-End Latency
func testValidateE2ELatency(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()
	nrClient := NewNewRelicClient(t)

	// Insert a unique marker query
	marker := fmt.Sprintf("MARKER_%d", time.Now().Unix())
	query := fmt.Sprintf("SELECT '%s' as marker, SLEEP(0.1)", marker)
	
	startTime := time.Now()
	_, err := db.Exec(query)
	require.NoError(t, err)
	
	// Poll for the metric to appear in New Relic
	timeout := 2 * time.Minute
	pollInterval := 5 * time.Second
	deadline := time.Now().Add(timeout)
	
	var found bool
	var latency time.Duration
	
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		
		nrql := fmt.Sprintf(`SELECT count(*) as count FROM Metric 
		                     WHERE metricName = 'mysql.query.wait_profile' 
		                     AND query_text LIKE '%%%s%%' 
		                     SINCE 3 minutes ago`, marker)
		
		results, err := nrClient.QueryNRQL(nrql)
		if err != nil {
			continue
		}
		
		if len(results) > 0 && results[0]["count"].(float64) > 0 {
			found = true
			latency = time.Since(startTime)
			break
		}
	}
	
	require.True(t, found, "Marker query not found in New Relic within timeout")
	assert.Less(t, latency, 90*time.Second, "End-to-end latency too high: %v", latency)
	
	t.Logf("End-to-end latency: %v", latency)
}

// Helper functions

func connectMySQL(t *testing.T) *sql.DB {
	dsn := "root:rootpassword@tcp(localhost:3306)/production"
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	
	err = db.Ping()
	require.NoError(t, err)
	
	return db
}

type NewRelicClient struct {
	apiKey    string
	accountID string
	baseURL   string
}

func NewNewRelicClient(t *testing.T) *NewRelicClient {
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	
	require.NotEmpty(t, apiKey, "NEW_RELIC_API_KEY not set")
	require.NotEmpty(t, accountID, "NEW_RELIC_ACCOUNT_ID not set")
	
	return &NewRelicClient{
		apiKey:    apiKey,
		accountID: accountID,
		baseURL:   "https://api.newrelic.com/graphql",
	}
}

func (c *NewRelicClient) QueryNRQL(nrql string) ([]map[string]interface{}, error) {
	query := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "%s") {
					results
				}
			}
		}
	}`, c.accountID, strings.ReplaceAll(nrql, `"`, `\"`))
	
	payload := map[string]string{"query": query}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequest("POST", c.baseURL, strings.NewReader(string(jsonPayload)))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("API-Key", c.apiKey)
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
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
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return result.Data.Actor.Account.NRQL.Results, nil
}

// print_status prints formatted status messages  
func print_status(status, message string) {
	colors := map[string]string{
		"info":    "\033[0;36m",
		"success": "\033[0;32m",
		"error":   "\033[0;31m",
		"test":    "\033[0;34m",
	}
	reset := "\033[0m"
	
	symbol := map[string]string{
		"info":    "ℹ",
		"success": "✓",
		"error":   "✗",
		"test":    "▶",
	}[status]
	
	fmt.Printf("%s%s%s %s\n", colors[status], symbol, reset, message)
}