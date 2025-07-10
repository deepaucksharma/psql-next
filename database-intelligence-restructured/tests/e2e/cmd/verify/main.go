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
				} `json:"nrql"`
			} `json:"account"`
		} `json:"actor"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func main() {
	fmt.Println("=== New Relic Connection Verification ===")
	fmt.Println()

	// Check environment variables
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}

	if accountID == "" || apiKey == "" {
		log.Fatal("Missing required environment variables. Please set NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY (or NEW_RELIC_USER_KEY)")
	}

	fmt.Printf("Account ID: %s\n", accountID)
	fmt.Printf("API Key: %s...%s (hidden)\n", apiKey[:4], apiKey[len(apiKey)-4:])
	fmt.Println()

	// Test NRDB connection
	fmt.Println("Testing NRDB connection...")
	
	// Simple query
	nrql := "SELECT count(*) FROM Metric SINCE 1 hour ago"
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
		log.Fatal("Failed to create request:", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("API-Key", apiKey)
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Failed to execute request:", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var result NRQLResponse
	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatal("Failed to parse response:", err)
	}
	
	if len(result.Errors) > 0 {
		log.Fatal("GraphQL errors:", result.Errors)
	}
	
	fmt.Println("✓ Successfully connected to NRDB!")
	
	// Check for database metrics
	fmt.Println("\nChecking for database metrics...")
	
	queries := []struct {
		name string
		nrql string
	}{
		{
			name: "PostgreSQL metrics in last 24h",
			nrql: "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 1 day ago",
		},
		{
			name: "MySQL metrics in last 24h",
			nrql: "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'mysql%' SINCE 1 day ago",
		},
		{
			name: "Recent metrics (any)",
			nrql: "SELECT count(*) FROM Metric SINCE 5 minutes ago",
		},
	}
	
	for _, q := range queries {
		fmt.Printf("\n%s:\n", q.name)
		
		query := fmt.Sprintf(`{
			actor {
				account(id: %s) {
					nrql(query: "%s") {
						results
					}
				}
			}
		}`, accountID, q.nrql)
		
		requestBody, _ := json.Marshal(GraphQLRequest{Query: query})
		req, _ := http.NewRequest("POST", "https://api.newrelic.com/graphql", bytes.NewBuffer(requestBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("API-Key", apiKey)
		
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("  ✗ Query failed: %v\n", err)
			continue
		}
		
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		
		var result NRQLResponse
		json.Unmarshal(body, &result)
		
		if len(result.Data.Actor.Account.NRQL.Results) > 0 {
			fmt.Printf("  ✓ Found data\n")
			for k, v := range result.Data.Actor.Account.NRQL.Results[0] {
				fmt.Printf("    %s: %v\n", k, v)
			}
		} else {
			fmt.Printf("  ✗ No data found\n")
		}
	}
	
	fmt.Println("\n✓ Verification complete!")
}