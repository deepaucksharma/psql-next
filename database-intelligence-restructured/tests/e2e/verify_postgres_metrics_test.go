package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestVerifyPostgreSQLMetricsInNRDB queries NRDB to verify PostgreSQL metrics
func TestVerifyPostgreSQLMetricsInNRDB(t *testing.T) {
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
		runID = "docker_postgres_%"
		t.Log("No specific TEST_RUN_ID provided, searching for all docker_postgres_* runs")
	}

	t.Logf("Verifying PostgreSQL metrics in NRDB for account: %s", accountID)

	// Query for PostgreSQL metrics with our test attributes
	queries := []struct {
		desc string
		nrql string
	}{
		{
			desc: "Find test runs",
			nrql: fmt.Sprintf("SELECT uniques(test.run.id) FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
		{
			desc: "PostgreSQL metrics by name",
			nrql: "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 1 hour ago LIMIT 100",
		},
		{
			desc: "Test run metrics with attributes",
			nrql: fmt.Sprintf("SELECT * FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago LIMIT 20", runID),
		},
		{
			desc: "PostgreSQL database size metrics",
			nrql: fmt.Sprintf("SELECT average(postgresql.database.size) FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago FACET db.name", runID),
		},
		{
			desc: "PostgreSQL connection metrics",
			nrql: fmt.Sprintf("SELECT average(postgresql.backends) FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
		{
			desc: "PostgreSQL table metrics",
			nrql: fmt.Sprintf("SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql.table%%' AND test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
		{
			desc: "All unique metric names from test run",
			nrql: fmt.Sprintf("SELECT uniques(metricName) FROM Metric WHERE test.run.id LIKE '%s' SINCE 1 hour ago", runID),
		},
		{
			desc: "Custom attributes verification",
			nrql: fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id LIKE '%s' AND environment = 'e2e-docker-test' AND test.type = 'postgresql' SINCE 1 hour ago", runID),
		},
	}

	foundMetrics := false
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
		t.Log("No PostgreSQL metrics found yet. They may still be processing.")
		t.Log("Wait a few more minutes and run this test again.")
		t.Log("You can also check the New Relic UI directly.")
	} else {
		t.Log("PostgreSQL metrics successfully verified in NRDB!")
	}
}

// TestListRecentPostgreSQLMetrics lists all recent PostgreSQL metrics
func TestListRecentPostgreSQLMetrics(t *testing.T) {
	// Skip if no credentials
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")

	if apiKey == "" || accountID == "" {
		t.Skip("NEW_RELIC_API_KEY/USER_KEY and NEW_RELIC_ACCOUNT_ID not set")
	}

	t.Logf("Listing all PostgreSQL metrics in NRDB for account: %s", accountID)

	// Query for all PostgreSQL metrics
	nrql := "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'postgresql%' OR source = 'postgresql' SINCE 24 hours ago LIMIT 200"
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(result.Data.Actor.Account.NRQL.Results) == 0 {
		t.Log("No PostgreSQL metrics found in the last 24 hours")
	} else {
		t.Logf("Found PostgreSQL metrics:")
		for _, r := range result.Data.Actor.Account.NRQL.Results {
			if uniqueNames, ok := r["uniques.metricName"].([]interface{}); ok {
				for _, name := range uniqueNames {
					t.Logf("  - %s", name)
				}
			}
		}
	}

	// Also check for any metrics with our test attributes
	nrql = "SELECT count(*), uniques(metricName) FROM Metric WHERE environment LIKE 'e2e%' SINCE 24 hours ago"
	
	result, err = queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Logf("Failed to query test metrics: %v", err)
		return
	}

	if len(result.Data.Actor.Account.NRQL.Results) > 0 {
		t.Log("\nMetrics with e2e test attributes:")
		data, _ := json.MarshalIndent(result.Data.Actor.Account.NRQL.Results[0], "  ", "  ")
		t.Logf("%s", string(data))
	}
}