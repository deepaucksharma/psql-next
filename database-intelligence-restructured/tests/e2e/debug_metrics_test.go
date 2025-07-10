package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// TestDebugMetrics helps debug what metrics are in NRDB
func TestDebugMetrics(t *testing.T) {
	// Skip if no credentials
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")

	if apiKey == "" || accountID == "" {
		t.Skip("NEW_RELIC_API_KEY/USER_KEY and NEW_RELIC_ACCOUNT_ID not set")
	}

	runID := os.Getenv("TEST_RUN_ID")
	if runID == "" {
		runID = "accuracy_test_%"
		t.Log("No specific TEST_RUN_ID provided, searching for accuracy_test_* runs")
	}

	t.Logf("Debugging metrics for run ID: %s", runID)

	// Query to see all table names
	queries := []struct {
		desc string
		nrql string
	}{
		{
			desc: "All unique table names",
			nrql: fmt.Sprintf("SELECT uniques(postgresql.table.name) FROM Metric WHERE test.run.id LIKE '%s' SINCE 2 hours ago", runID),
		},
		{
			desc: "Sample table metrics with all attributes",
			nrql: fmt.Sprintf("SELECT * FROM Metric WHERE test.run.id LIKE '%s' AND metricName = 'postgresql.table.size' SINCE 2 hours ago LIMIT 5", runID),
		},
		{
			desc: "Row metrics with states",
			nrql: fmt.Sprintf("SELECT postgresql.table.name, state, postgresql.rows FROM Metric WHERE test.run.id LIKE '%s' AND metricName = 'postgresql.rows' SINCE 2 hours ago LIMIT 20", runID),
		},
		{
			desc: "Tables containing 'accuracy_test'",
			nrql: fmt.Sprintf("SELECT uniques(postgresql.table.name) FROM Metric WHERE test.run.id LIKE '%s' AND postgresql.table.name LIKE '%%accuracy_test%%' SINCE 2 hours ago", runID),
		},
		{
			desc: "All metric names for accuracy test",
			nrql: fmt.Sprintf("SELECT uniques(metricName) FROM Metric WHERE test.run.id LIKE '%s' SINCE 2 hours ago", runID),
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
				t.Logf("Found %d results", len(result.Data.Actor.Account.NRQL.Results))
				
				// Pretty print results
				for i, res := range result.Data.Actor.Account.NRQL.Results {
					data, _ := json.MarshalIndent(res, "  ", "  ")
					t.Logf("Result %d:\n%s", i+1, string(data))
				}
			}
		})
	}
}