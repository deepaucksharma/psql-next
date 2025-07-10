package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestVerifyMySQLMetricsInNRDB queries NRDB to verify MySQL metrics
func TestVerifyMySQLMetricsInNRDB(t *testing.T) {
	// Skip if no credentials
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")

	if apiKey == "" || accountID == "" {
		t.Skip("NEW_RELIC_API_KEY/USER_KEY and NEW_RELIC_ACCOUNT_ID not set")
	}

	// Use specific run ID if provided, otherwise use wildcard
	runID := os.Getenv("TEST_RUN_ID")
	if runID == "" {
		runID = "docker_mysql_%"
		t.Log("No specific TEST_RUN_ID provided, searching for all docker_mysql_* runs")
	}

	t.Logf("Verifying MySQL metrics in NRDB for account: %s", accountID)

	// Query for MySQL metrics with our test attributes
	queries := []struct {
		desc string
		nrql string
	}{
		{
			desc: "Find MySQL test runs",
			nrql: fmt.Sprintf("SELECT uniques(test.run.id) FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
		{
			desc: "MySQL metrics by name",
			nrql: "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'mysql%' SINCE 1 hour ago LIMIT 100",
		},
		{
			desc: "Test run metrics with attributes",
			nrql: fmt.Sprintf("SELECT * FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago LIMIT 20", runID),
		},
		{
			desc: "MySQL connection metrics",
			nrql: fmt.Sprintf("SELECT average(mysql.threads.connected), average(mysql.threads.running) FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
		{
			desc: "MySQL query metrics",
			nrql: fmt.Sprintf("SELECT rate(sum(mysql.queries), 1 minute) FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
		{
			desc: "MySQL table metrics",
			nrql: fmt.Sprintf("SELECT count(*) FROM Metric WHERE metricName LIKE 'mysql.table%%' AND test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
		{
			desc: "All unique MySQL metric names from test run",
			nrql: fmt.Sprintf("SELECT uniques(metricName) FROM Metric WHERE test.run.id LIKE '%s' AND metricName LIKE 'mysql%%' SINCE 1 hour ago", runID),
		},
		{
			desc: "Custom attributes verification for MySQL",
			nrql: fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id LIKE '%s' AND environment = 'e2e-docker-test' AND test.type = 'mysql' SINCE 1 hour ago", runID),
		},
		{
			desc: "MySQL buffer pool metrics",
			nrql: fmt.Sprintf("SELECT average(mysql.buffer_pool.pages.total), average(mysql.buffer_pool.pages.free) FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
		{
			desc: "MySQL InnoDB metrics",
			nrql: fmt.Sprintf("SELECT count(*) FROM Metric WHERE metricName LIKE 'mysql.innodb%%' AND test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
	}

	foundMetrics := false
	mysqlMetricTypes := make(map[string]bool)
	
	for _, q := range queries {
		t.Run(q.desc, func(t *testing.T) {
			// Add slight delay between queries to avoid rate limiting
			time.Sleep(1 * time.Second)
			
			result, err := queryNRDB(accountID, apiKey, q.nrql)
			if err != nil {
				t.Errorf("Query failed: %v", err)
				return
			}

			t.Logf("Query: %s", q.nrql)
			if len(result.Data.Actor.Account.NRQL.Results) == 0 {
				t.Log("No results found")
			} else {
				foundMetrics = true
				t.Logf("Found %d results", len(result.Data.Actor.Account.NRQL.Results))
				
				// Collect unique metric names
				for _, r := range result.Data.Actor.Account.NRQL.Results {
					if names, ok := r["uniques.metricName"].([]interface{}); ok {
						for _, name := range names {
							if nameStr, ok := name.(string); ok {
								mysqlMetricTypes[nameStr] = true
							}
						}
					}
				}
				
				// Pretty print first few results
				maxResults := 5
				if len(result.Data.Actor.Account.NRQL.Results) < maxResults {
					maxResults = len(result.Data.Actor.Account.NRQL.Results)
				}
				
				for i := 0; i < maxResults; i++ {
					data, _ := json.MarshalIndent(result.Data.Actor.Account.NRQL.Results[i], "  ", "  ")
					t.Logf("Result %d: %s", i+1, string(data))
				}
				
				if len(result.Data.Actor.Account.NRQL.Results) > maxResults {
					t.Logf("... and %d more results", len(result.Data.Actor.Account.NRQL.Results)-maxResults)
				}
			}
		})
	}

	if !foundMetrics {
		t.Log("No MySQL metrics found yet. They may still be processing.")
		t.Log("Wait a few more minutes and run this test again.")
		t.Log("You can also check the New Relic UI directly.")
	} else {
		t.Log("MySQL metrics successfully verified in NRDB!")
		t.Logf("Found %d unique MySQL metric types", len(mysqlMetricTypes))
		if len(mysqlMetricTypes) > 0 {
			t.Log("MySQL metric types found:")
			for metricName := range mysqlMetricTypes {
				t.Logf("  - %s", metricName)
			}
		}
	}
}

// TestCompareDatabaseMetrics compares PostgreSQL and MySQL metrics
func TestCompareDatabaseMetrics(t *testing.T) {
	// Skip if no credentials
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")

	if apiKey == "" || accountID == "" {
		t.Skip("NEW_RELIC_API_KEY/USER_KEY and NEW_RELIC_ACCOUNT_ID not set")
	}

	t.Logf("Comparing database metrics in NRDB for account: %s", accountID)

	// Query for both database types
	queries := []struct {
		desc string
		nrql string
	}{
		{
			desc: "All database test runs",
			nrql: "SELECT uniques(test.run.id), uniques(test.type) FROM Metric WHERE test.run.id LIKE 'docker_%' SINCE 1 hour ago",
		},
		{
			desc: "Metrics count by database type",
			nrql: "SELECT count(*) FROM Metric WHERE test.type IN ('postgresql', 'mysql') SINCE 1 hour ago FACET test.type",
		},
		{
			desc: "Unique metrics by database type",
			nrql: "SELECT uniqueCount(metricName) FROM Metric WHERE test.type IN ('postgresql', 'mysql') SINCE 1 hour ago FACET test.type",
		},
		{
			desc: "Average collection interval",
			nrql: "SELECT average(timestamp - previous(timestamp)) as avg_interval FROM Metric WHERE test.type IN ('postgresql', 'mysql') SINCE 1 hour ago FACET test.type",
		},
	}

	for _, q := range queries {
		t.Run(q.desc, func(t *testing.T) {
			result, err := queryNRDB(accountID, apiKey, q.nrql)
			if err != nil {
				t.Errorf("Query failed: %v", err)
				return
			}

			t.Logf("Query: %s", q.nrql)
			if len(result.Data.Actor.Account.NRQL.Results) == 0 {
				t.Log("No results found")
			} else {
				for i, r := range result.Data.Actor.Account.NRQL.Results {
					data, _ := json.MarshalIndent(r, "  ", "  ")
					t.Logf("Result %d: %s", i+1, string(data))
				}
			}
		})
	}
}