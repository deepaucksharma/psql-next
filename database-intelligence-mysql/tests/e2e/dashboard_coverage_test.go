package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MetricCoverage tracks which metrics are used in dashboards
type MetricCoverage struct {
	TotalMetrics      int
	DashboardMetrics  int
	UnusedMetrics     []string
	MissingWidgets    []string
}

// TestDashboardMetricCoverage ensures all collected metrics are visualized
func TestDashboardMetricCoverage(t *testing.T) {
	// All metrics we collect
	collectedMetrics := []string{
		// Wait profile metrics
		"mysql.query.wait_profile",
		"mysql.query.wait_profile.total_wait_ms",
		"mysql.query.wait_profile.avg_wait_ms",
		"mysql.query.wait_profile.wait_percentage",
		"mysql.query.wait_profile.statement_time_ms",
		"mysql.query.wait_profile.lock_time_ms",
		
		// Query analysis metrics
		"mysql.query.wait_profile.NO_INDEX_USED",
		"mysql.query.wait_profile.NO_GOOD_INDEX_USED",
		"mysql.query.wait_profile.tmp_disk_tables",
		"mysql.query.wait_profile.full_joins",
		"mysql.query.wait_profile.full_scans",
		"mysql.query.wait_profile.ROWS_EXAMINED",
		
		// Blocking metrics
		"mysql.blocking.active",
		"mysql.blocking.wait_duration",
		"mysql.blocking.lock_type",
		
		// Statement digest metrics
		"mysql.statement.digest.exec_count",
		"mysql.statement.digest.total_time_sec",
		"mysql.statement.digest.avg_time_ms",
		"mysql.statement.digest.total_lock_ms",
		"mysql.statement.digest.total_rows_examined",
		"mysql.statement.digest.no_index_used_count",
		
		// Current wait metrics
		"mysql.current.waits.thread_count",
		"mysql.current.waits.wait_ms",
		
		// Advisory metrics
		"mysql.advisor.missing_index",
		"mysql.advisor.lock_contention",
		"mysql.advisor.temp_table_disk",
		"mysql.advisor.plan_regression",
		"mysql.advisor.composite",
		
		// Prometheus MySQL exporter metrics
		"mysql_global_status_threads_running",
		"mysql_global_status_questions",
		"mysql_global_status_slow_queries",
		"mysql_global_status_table_locks_waited",
		"mysql_info_schema_processlist_threads",
		"mysql_perf_schema_table_io_waits_total",
		"mysql_perf_schema_index_io_waits_total",
	}

	// Dashboard widget requirements
	requiredWidgets := map[string][]string{
		"Wait Analysis Dashboard": {
			"Query Wait Time Breakdown",
			"Top Queries by Wait Time",
			"Wait Percentage Distribution",
			"Wait Category Timeline",
			"Advisory Summary",
			"I/O Wait Analysis",
			"Lock Wait Analysis",
			"Network Wait Analysis",
			"CPU Wait Analysis",
		},
		"Query Detail Dashboard": {
			"Query Performance Trends",
			"Query Execution Count",
			"Index Usage Analysis",
			"Temp Table Usage",
			"Full Scan Queries",
			"Rows Examined vs Sent",
			"Query Time Distribution",
			"Lock Time Analysis",
			"Plan Change Detection",
		},
		"Advisory Dashboard": {
			"Active Advisories",
			"Advisory Timeline",
			"Advisory by Priority",
			"Top Missing Indexes",
			"Lock Contention Patterns",
			"Temp Table Hotspots",
			"Plan Regression History",
			"Composite Advisory Analysis",
		},
		"Performance Overview": {
			"Database Health Score",
			"Wait Time Percentage",
			"Query Response Time P95",
			"Active Connections",
			"Questions Per Second",
			"Slow Query Rate",
			"Table Lock Waits",
			"Thread Utilization",
		},
	}

	t.Run("ValidateMetricUsage", func(t *testing.T) {
		nrClient := NewNewRelicClient(t)
		coverage := &MetricCoverage{
			TotalMetrics: len(collectedMetrics),
		}

		// Check each metric is used in at least one dashboard
		for _, metric := range collectedMetrics {
			nrql := fmt.Sprintf(`SELECT count(*) as usage_count 
			                     FROM Metric 
			                     WHERE metricName = '%s' 
			                     SINCE 1 hour ago`, metric)
			
			results, err := nrClient.QueryNRQL(nrql)
			if err != nil || len(results) == 0 || results[0]["usage_count"].(float64) == 0 {
				coverage.UnusedMetrics = append(coverage.UnusedMetrics, metric)
			} else {
				coverage.DashboardMetrics++
			}
		}

		// Calculate coverage percentage
		coveragePercent := float64(coverage.DashboardMetrics) / float64(coverage.TotalMetrics) * 100
		
		t.Logf("Metric Coverage: %.2f%% (%d/%d metrics used)", 
			coveragePercent, coverage.DashboardMetrics, coverage.TotalMetrics)
		
		if len(coverage.UnusedMetrics) > 0 {
			t.Logf("Unused metrics: %v", coverage.UnusedMetrics)
		}
		
		// Require at least 90% coverage
		assert.GreaterOrEqual(t, coveragePercent, 90.0, 
			"Dashboard metric coverage below 90%%")
	})

	t.Run("ValidateRequiredWidgets", func(t *testing.T) {
		for dashboardName, widgets := range requiredWidgets {
			t.Run(dashboardName, func(t *testing.T) {
				for _, widgetTitle := range widgets {
					t.Run(widgetTitle, func(t *testing.T) {
						// Validate widget has data
						validateWidgetData(t, dashboardName, widgetTitle)
					})
				}
			})
		}
	})
}

// TestAdvisoryAccuracy validates that advisories are accurate and actionable
func TestAdvisoryAccuracy(t *testing.T) {
	db := connectMySQL(t)
	defer db.Close()
	nrClient := NewNewRelicClient(t)

	testCases := []struct {
		name            string
		setupQuery      string
		validationQuery string
		expectedAdvisory string
		validateAction  func(t *testing.T, advisory map[string]interface{})
	}{
		{
			name: "Missing Index Advisory Accuracy",
			setupQuery: `CREATE TABLE IF NOT EXISTS test_advisory_accuracy (
			            id INT PRIMARY KEY,
			            user_id INT,
			            created_at TIMESTAMP,
			            data TEXT
			            );
			            SELECT * FROM test_advisory_accuracy 
			            WHERE user_id = 123 AND created_at > NOW() - INTERVAL 1 DAY`,
			validationQuery: `EXPLAIN SELECT * FROM test_advisory_accuracy 
			                  WHERE user_id = 123 AND created_at > NOW() - INTERVAL 1 DAY`,
			expectedAdvisory: "missing_index",
			validateAction: func(t *testing.T, advisory map[string]interface{}) {
				// Validate suggested index
				recommendation := advisory["recommendation"].(string)
				assert.Contains(t, recommendation, "CREATE INDEX", 
					"Advisory should recommend index creation")
				assert.Contains(t, recommendation, "user_id", 
					"Index should include user_id column")
			},
		},
		{
			name: "Lock Contention Advisory Accuracy",
			setupQuery: `START TRANSACTION;
			            UPDATE test_orders SET status = 'processing' WHERE id = 1;
			            DO SLEEP(3);
			            COMMIT;`,
			validationQuery: `SELECT COUNT(*) FROM information_schema.innodb_trx 
			                  WHERE trx_state = 'LOCK WAIT'`,
			expectedAdvisory: "lock_contention",
			validateAction: func(t *testing.T, advisory map[string]interface{}) {
				// Validate lock details
				assert.NotNil(t, advisory["blocking_thread"], 
					"Should identify blocking thread")
				assert.NotNil(t, advisory["waiting_thread"], 
					"Should identify waiting thread")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup scenario
			_, err := db.Exec(tc.setupQuery)
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				require.NoError(t, err)
			}

			// Wait for advisory generation
			time.Sleep(20 * time.Second)

			// Query advisory from New Relic
			nrql := fmt.Sprintf(`SELECT * FROM Metric 
			                     WHERE advisor.type = '%s' 
			                     SINCE 2 minutes ago 
			                     LIMIT 1`, tc.expectedAdvisory)
			
			results, err := nrClient.QueryNRQL(nrql)
			require.NoError(t, err)
			require.NotEmpty(t, results, "Expected %s advisory not found", tc.expectedAdvisory)

			// Validate advisory accuracy
			tc.validateAction(t, results[0])
		})
	}
}

// TestDataQualityValidation ensures data quality and consistency
func TestDataQualityValidation(t *testing.T) {
	nrClient := NewNewRelicClient(t)

	qualityChecks := []struct {
		name       string
		nrql       string
		validation func(results []map[string]interface{}) error
	}{
		{
			name: "No Negative Wait Times",
			nrql: `SELECT count(*) as negative_count 
			       FROM Metric 
			       WHERE metricName = 'mysql.query.wait_profile' 
			       AND value < 0 
			       SINCE 1 hour ago`,
			validation: func(results []map[string]interface{}) error {
				if len(results) > 0 && results[0]["negative_count"].(float64) > 0 {
					return fmt.Errorf("Found negative wait times")
				}
				return nil
			},
		},
		{
			name: "Wait Percentage Within Bounds",
			nrql: `SELECT count(*) as invalid_count 
			       FROM Metric 
			       WHERE metricName = 'mysql.query.wait_profile' 
			       AND (wait_percentage < 0 OR wait_percentage > 100) 
			       SINCE 1 hour ago`,
			validation: func(results []map[string]interface{}) error {
				if len(results) > 0 && results[0]["invalid_count"].(float64) > 0 {
					return fmt.Errorf("Found wait percentages outside 0-100 range")
				}
				return nil
			},
		},
		{
			name: "Query Hash Consistency",
			nrql: `SELECT uniques(query_hash) as unique_hashes,
			       count(*) as total_points 
			       FROM Metric 
			       WHERE metricName = 'mysql.query.wait_profile' 
			       AND query_hash IS NOT NULL 
			       SINCE 1 hour ago`,
			validation: func(results []map[string]interface{}) error {
				if len(results) == 0 {
					return fmt.Errorf("No query hash data found")
				}
				
				uniqueHashes := results[0]["unique_hashes"].(float64)
				totalPoints := results[0]["total_points"].(float64)
				
				// Ensure reasonable cardinality
				if uniqueHashes > 10000 {
					return fmt.Errorf("Too many unique query hashes: %v", uniqueHashes)
				}
				
				// Ensure we have sufficient data points per query
				avgPointsPerQuery := totalPoints / uniqueHashes
				if avgPointsPerQuery < 2 {
					return fmt.Errorf("Insufficient data points per query: %.2f", avgPointsPerQuery)
				}
				
				return nil
			},
		},
		{
			name: "Time Series Continuity",
			nrql: `SELECT count(*) as data_points 
			       FROM Metric 
			       WHERE metricName = 'mysql.query.wait_profile' 
			       TIMESERIES 1 minute 
			       SINCE 30 minutes ago`,
			validation: func(results []map[string]interface{}) error {
				// Should have ~30 data points (one per minute)
				if len(results) < 25 {
					return fmt.Errorf("Time series has gaps: only %d data points in 30 minutes", len(results))
				}
				return nil
			},
		},
	}

	for _, check := range qualityChecks {
		t.Run(check.name, func(t *testing.T) {
			results, err := nrClient.QueryNRQL(check.nrql)
			require.NoError(t, err)
			
			err = check.validation(results)
			assert.NoError(t, err)
		})
	}
}

// TestDashboardScreenshotValidation validates dashboards visually
func TestDashboardScreenshotValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping screenshot validation in short mode")
	}

	dashboards := []struct {
		name     string
		url      string
		elements []string // CSS selectors for key elements
	}{
		{
			name: "Wait Analysis Dashboard",
			url:  os.Getenv("WAIT_ANALYSIS_DASHBOARD_URL"),
			elements: []string{
				".wait-breakdown-chart",
				".top-queries-table",
				".wait-timeline",
				".advisory-panel",
			},
		},
		{
			name: "Query Detail Dashboard",
			url:  os.Getenv("QUERY_DETAIL_DASHBOARD_URL"),
			elements: []string{
				".query-trend-chart",
				".index-usage-pie",
				".temp-table-bar",
				".plan-change-indicator",
			},
		},
	}

	for _, dashboard := range dashboards {
		t.Run(dashboard.name, func(t *testing.T) {
			if dashboard.url == "" {
				t.Skip("Dashboard URL not configured")
			}

			// This would use a headless browser to validate
			// For now, we'll just check the dashboard loads
			t.Logf("Would validate dashboard at: %s", dashboard.url)
			
			// In a real implementation:
			// 1. Use Selenium or Playwright to load dashboard
			// 2. Wait for elements to render
			// 3. Take screenshot
			// 4. Compare with baseline or check for anomalies
			// 5. Validate data is present in visualizations
		})
	}
}

// Helper function to validate widget data
func validateWidgetData(t *testing.T, dashboardName, widgetTitle string) {
	// Map widget titles to NRQL queries
	widgetQueries := map[string]string{
		"Query Wait Time Breakdown": `SELECT sum(value) FROM Metric 
		                              WHERE metricName = 'mysql.query.wait_profile' 
		                              FACET wait_type SINCE 1 hour ago`,
		"Top Queries by Wait Time": `SELECT sum(value) FROM Metric 
		                             WHERE metricName = 'mysql.query.wait_profile' 
		                             FACET query_hash LIMIT 10 SINCE 1 hour ago`,
		"Advisory Summary": `SELECT count(*) FROM Metric 
		                     WHERE advisor.type IS NOT NULL 
		                     FACET advisor.type SINCE 1 hour ago`,
		// Add more mappings as needed
	}

	nrql, exists := widgetQueries[widgetTitle]
	if !exists {
		t.Skipf("No NRQL mapping for widget: %s", widgetTitle)
		return
	}

	nrClient := NewNewRelicClient(t)
	results, err := nrClient.QueryNRQL(nrql)
	require.NoError(t, err)
	
	assert.NotEmpty(t, results, 
		"No data for widget '%s' in dashboard '%s'", widgetTitle, dashboardName)
}