package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type GraphQLRequest struct {
	Query string `json:"query"`
}

type NRQLResponse struct {
	Data struct {
		Actor struct {
			Account struct {
				NRQL struct {
					Results []map[string]interface{} `json:"results"`
					Facets  []string                 `json:"facets"`
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

func main() {
	// Load credentials
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	
	if accountID == "" || apiKey == "" {
		log.Fatal("Missing NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY")
	}
	
	fmt.Println("=== NRDB Query Test ===")
	fmt.Printf("Account: %s\n\n", accountID)
	
	// Test queries to understand what data we have
	queries := []struct {
		desc string
		nrql string
	}{
		{
			desc: "Recent PostgreSQL metrics",
			nrql: "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 1 hour ago LIMIT 100",
		},
		{
			desc: "Sample PostgreSQL data points",
			nrql: "SELECT * FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 10 minutes ago LIMIT 5",
		},
		{
			desc: "Database logs",
			nrql: "SELECT * FROM Log WHERE db.system IS NOT NULL SINCE 1 hour ago LIMIT 5",
		},
		{
			desc: "All metric names",
			nrql: "SELECT uniques(metricName) FROM Metric SINCE 1 hour ago LIMIT 20",
		},
		{
			desc: "Test metrics we might have sent",
			nrql: "SELECT * FROM Metric WHERE test.id IS NOT NULL OR pipeline.test IS NOT NULL SINCE 1 day ago LIMIT 10",
		},
	}
	
	for _, q := range queries {
		fmt.Printf("\n=== %s ===\n", q.desc)
		fmt.Printf("Query: %s\n\n", q.nrql)
		
		result, err := queryNRDB(accountID, apiKey, q.nrql)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		
		if len(result.Data.Actor.Account.NRQL.Results) == 0 {
			fmt.Println("No results found")
			continue
		}
		
		// Pretty print results
		for i, r := range result.Data.Actor.Account.NRQL.Results {
			fmt.Printf("Result %d:\n", i+1)
			data, _ := json.MarshalIndent(r, "  ", "  ")
			fmt.Println(string(data))
		}
	}
}