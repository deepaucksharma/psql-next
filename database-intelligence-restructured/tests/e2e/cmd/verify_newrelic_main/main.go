package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/database-intelligence/tests/e2e/framework"
)

func main() {
	fmt.Println("=== New Relic Connection Verification ===")
	fmt.Println()

	// Check environment variables
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_USER_KEY")
	}
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")

	if accountID == "" || apiKey == "" {
		log.Fatal("Missing required environment variables. Please set NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY (or NEW_RELIC_USER_KEY)")
	}

	fmt.Printf("Account ID: %s\n", accountID)
	fmt.Printf("API Key: %s...%s (hidden)\n", apiKey[:4], apiKey[len(apiKey)-4:])
	if licenseKey != "" {
		fmt.Printf("License Key: %s...%s (hidden)\n", licenseKey[:4], licenseKey[len(licenseKey)-4:])
	}
	fmt.Println()

	// Create NRDB client
	client := framework.NewNRDBClient(accountID, apiKey)

	// Test query
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Testing NRDB connection...")
	
	// Simple query to test connection
	nrql := "SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 1 hour ago"
	
	result, err := client.Query(ctx, nrql)
	if err != nil {
		log.Fatalf("Failed to query NRDB: %v", err)
	}

	fmt.Println("✓ Successfully connected to NRDB!")
	fmt.Printf("Query returned %d results\n", len(result.Results))

	// Try to find any database metrics
	fmt.Println("\nChecking for recent database metrics...")
	
	queries := []struct {
		name string
		nrql string
	}{
		{
			name: "PostgreSQL metrics",
			nrql: "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 1 day ago",
		},
		{
			name: "MySQL metrics",
			nrql: "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'mysql%' SINCE 1 day ago",
		},
		{
			name: "Database logs",
			nrql: "SELECT count(*) FROM Log WHERE db.system IS NOT NULL SINCE 1 day ago",
		},
	}

	for _, q := range queries {
		result, err := client.Query(ctx, q.nrql)
		if err != nil {
			fmt.Printf("✗ %s query failed: %v\n", q.name, err)
			continue
		}

		if len(result.Results) > 0 {
			fmt.Printf("✓ %s: Found data\n", q.name)
			if metrics, ok := result.Results[0]["uniques"]; ok {
				if metricList, ok := metrics.([]interface{}); ok {
					fmt.Printf("  Found %d unique metrics\n", len(metricList))
					// Show first few metrics
					for i, m := range metricList {
						if i >= 5 {
							fmt.Printf("  ... and %d more\n", len(metricList)-5)
							break
						}
						fmt.Printf("  - %v\n", m)
					}
				}
			} else if count, ok := result.Results[0]["count"]; ok {
				fmt.Printf("  Count: %v\n", count)
			}
		} else {
			fmt.Printf("✗ %s: No data found\n", q.name)
		}
	}

	fmt.Println("\n✓ New Relic verification complete!")
}