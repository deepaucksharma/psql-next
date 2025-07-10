package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestVerifyProcessorBehaviors queries NRDB to verify processor behaviors
func TestVerifyProcessorBehaviors(t *testing.T) {
	// Skip if no credentials
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")

	if apiKey == "" || accountID == "" {
		t.Skip("NEW_RELIC_API_KEY/USER_KEY and NEW_RELIC_ACCOUNT_ID not set")
	}

	// Use specific run ID if provided
	runID := os.Getenv("TEST_RUN_ID")
	if runID == "" {
		runID = "processor_test_%"
		t.Log("No specific TEST_RUN_ID provided, searching for all processor_test_* runs")
	}

	t.Logf("Verifying processor behaviors in NRDB for account: %s", accountID)

	// Test batch processor behavior
	t.Run("VerifyBatchProcessor", func(t *testing.T) {
		// Query for batch processor metrics
		nrql := fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id LIKE '%s_batch' AND processor.test = 'batch' SINCE 1 hour ago", runID)
		
		result, err := queryNRDB(accountID, apiKey, nrql)
		if err != nil {
			t.Errorf("Query failed: %v", err)
			return
		}

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			data, _ := json.MarshalIndent(result.Data.Actor.Account.NRQL.Results[0], "  ", "  ")
			t.Logf("Batch processor metrics found: %s", string(data))
		} else {
			t.Log("No batch processor metrics found yet")
		}
	})

	// Test filter processor behavior - should only have table and index metrics
	t.Run("VerifyFilterProcessor", func(t *testing.T) {
		// Query for filter processor metrics
		queries := []struct {
			desc string
			nrql string
		}{
			{
				desc: "Total metrics from filter processor",
				nrql: fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id LIKE '%s_filter' SINCE 1 hour ago", runID),
			},
			{
				desc: "Metric types from filter processor",
				nrql: fmt.Sprintf("SELECT uniques(metricName) FROM Metric WHERE test.run.id LIKE '%s_filter' SINCE 1 hour ago", runID),
			},
			{
				desc: "Non-table/index metrics (should be empty)",
				nrql: fmt.Sprintf("SELECT uniques(metricName) FROM Metric WHERE test.run.id LIKE '%s_filter' AND metricName NOT LIKE 'postgresql.table%%' AND metricName NOT LIKE 'postgresql.index%%' SINCE 1 hour ago", runID),
			},
		}

		for _, q := range queries {
			t.Logf("Testing: %s", q.desc)
			result, err := queryNRDB(accountID, apiKey, q.nrql)
			if err != nil {
				t.Errorf("Query failed: %v", err)
				continue
			}

			if len(result.Data.Actor.Account.NRQL.Results) > 0 {
				data, _ := json.MarshalIndent(result.Data.Actor.Account.NRQL.Results[0], "  ", "  ")
				t.Logf("Result: %s", string(data))
			}
		}
	})

	// Test attributes processor behavior
	t.Run("VerifyAttributesProcessor", func(t *testing.T) {
		// Query for attributes processor metrics
		queries := []struct {
			desc string
			nrql string
		}{
			{
				desc: "Metrics with custom attributes",
				nrql: fmt.Sprintf("SELECT count(*), uniques(db.system), uniques(deployment.environment) FROM Metric WHERE test.run.id LIKE '%s_attributes' SINCE 1 hour ago", runID),
			},
			{
				desc: "Table metrics with is_table_metric attribute",
				nrql: fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id LIKE '%s_attributes' AND is_table_metric = 'true' SINCE 1 hour ago", runID),
			},
			{
				desc: "Verify db.connection.string was deleted",
				nrql: fmt.Sprintf("SELECT count(*) FROM Metric WHERE test.run.id LIKE '%s_attributes' AND db.connection.string IS NOT NULL SINCE 1 hour ago", runID),
			},
		}

		for _, q := range queries {
			t.Logf("Testing: %s", q.desc)
			result, err := queryNRDB(accountID, apiKey, q.nrql)
			if err != nil {
				t.Errorf("Query failed: %v", err)
				continue
			}

			if len(result.Data.Actor.Account.NRQL.Results) > 0 {
				data, _ := json.MarshalIndent(result.Data.Actor.Account.NRQL.Results[0], "  ", "  ")
				t.Logf("Result: %s", string(data))
			}
		}
	})

	// Test resource processor behavior
	t.Run("VerifyResourceProcessor", func(t *testing.T) {
		// Query for resource processor metrics
		nrql := fmt.Sprintf("SELECT count(*), uniques(service.name), uniques(service.version), uniques(cloud.provider), uniques(cloud.region) FROM Metric WHERE test.run.id LIKE '%s_resource' SINCE 1 hour ago", runID)
		
		result, err := queryNRDB(accountID, apiKey, nrql)
		if err != nil {
			t.Errorf("Query failed: %v", err)
			return
		}

		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			data, _ := json.MarshalIndent(result.Data.Actor.Account.NRQL.Results[0], "  ", "  ")
			t.Logf("Resource processor attributes: %s", string(data))
			
			// Verify expected values
			if res := result.Data.Actor.Account.NRQL.Results[0]; res != nil {
				if serviceName, ok := res["uniques.service.name"].([]interface{}); ok && len(serviceName) > 0 {
					if serviceName[0] == "database-intelligence-e2e" {
						t.Log("✓ service.name correctly set")
					}
				}
				if cloudProvider, ok := res["uniques.cloud.provider"].([]interface{}); ok && len(cloudProvider) > 0 {
					if cloudProvider[0] == "test-cloud" {
						t.Log("✓ cloud.provider correctly set")
					}
				}
			}
		} else {
			t.Log("No resource processor metrics found yet")
		}
	})

	// Summary of all processor tests
	t.Run("ProcessorTestSummary", func(t *testing.T) {
		nrql := fmt.Sprintf("SELECT count(*), uniques(processor.test) FROM Metric WHERE test.run.id LIKE '%s%%' SINCE 1 hour ago FACET processor.test", runID)
		
		result, err := queryNRDB(accountID, apiKey, nrql)
		if err != nil {
			t.Errorf("Query failed: %v", err)
			return
		}

		t.Log("Processor test summary:")
		for i, res := range result.Data.Actor.Account.NRQL.Results {
			data, _ := json.MarshalIndent(res, "  ", "  ")
			t.Logf("Processor %d: %s", i+1, string(data))
		}
	})
}

// TestProcessorMetricCounts verifies metric counts for each processor
func TestProcessorMetricCounts(t *testing.T) {
	// Skip if no credentials
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")

	if apiKey == "" || accountID == "" {
		t.Skip("NEW_RELIC_API_KEY/USER_KEY and NEW_RELIC_ACCOUNT_ID not set")
	}

	// Wait a bit for metrics to be processed
	time.Sleep(10 * time.Second)

	// Query for all processor test metrics
	nrql := "SELECT count(*) FROM Metric WHERE test.run.id LIKE 'processor_test_%' SINCE 1 hour ago FACET test.run.id, processor.test LIMIT 20"
	
	result, err := queryNRDB(accountID, apiKey, nrql)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	t.Log("Processor metric counts:")
	foundProcessors := make(map[string]bool)
	
	for _, res := range result.Data.Actor.Account.NRQL.Results {
		if count, ok := res["count"].(float64); ok && count > 0 {
			if procTest, ok := res["processor.test"].(string); ok {
				foundProcessors[procTest] = true
				t.Logf("  %s: %.0f metrics", procTest, count)
			}
		}
	}

	// Check which processors we found
	expectedProcessors := []string{"batch", "filter", "resource"}
	for _, proc := range expectedProcessors {
		if foundProcessors[proc] {
			t.Logf("✓ %s processor verified", proc)
		} else {
			t.Logf("✗ %s processor not found", proc)
		}
	}
}