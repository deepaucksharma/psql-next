package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// NRDBQueryTest queries NRDB to verify what metrics we have
func TestNRDBQueryForMetrics(t *testing.T) {
	// Skip if no credentials
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	
	if apiKey == "" || accountID == "" {
		t.Skip("NEW_RELIC_API_KEY/USER_KEY and NEW_RELIC_ACCOUNT_ID not set")
	}
	
	t.Logf("Testing NRDB access for account: %s", accountID)
	
	// Query for recent test metrics
	queries := []struct {
		desc string
		nrql string
	}{
		{
			desc: "Check for test.run.id metrics",
			nrql: "SELECT * FROM Metric WHERE test.run.id LIKE 'first_real_e2e_%' SINCE 1 hour ago LIMIT 10",
		},
		{
			desc: "PostgreSQL metrics",
			nrql: "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 1 hour ago LIMIT 100",
		},
		{
			desc: "Recent database metrics with our test attributes",
			nrql: "SELECT * FROM Metric WHERE environment = 'e2e-test' SINCE 1 hour ago LIMIT 10",
		},
		{
			desc: "All metrics in last hour",
			nrql: "SELECT uniques(metricName) FROM Metric SINCE 1 hour ago LIMIT 20",
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

type GraphQLRequest struct {
	Query string `json:"query"`
}

type NRQLResponse struct {
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

func queryNRDB(accountID, apiKey, nrql string) (*NRQLResponse, error) {
	query := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "%s") {
					results
				}
			}
		}
	}`, accountID, nrql)

	requestBody, _ := json.Marshal(GraphQLRequest{Query: query})
	
	req, err := http.NewRequest("POST", "https://api.newrelic.com/graphql", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("API-Key", apiKey)
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var result NRQLResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", result.Errors)
	}
	
	return &result, nil
}