package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/database-intelligence/tests/e2e/framework"
)

func main() {
	fmt.Println("=== Checking New Relic Data ===")
	fmt.Println()

	// Load environment
	loadEnvFile(".env")

	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	apiKey := os.Getenv("NEW_RELIC_USER_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_API_KEY")
	}

	if accountID == "" || apiKey == "" {
		log.Fatal("Missing required environment variables")
	}

	nrdb := framework.NewNRDBClient(accountID, apiKey)
	ctx := context.Background()

	// Check various metric queries
	queries := []struct {
		name  string
		query string
	}{
		{
			"All metrics in last hour",
			"SELECT count(*) FROM Metric SINCE 1 hour ago",
		},
		{
			"Metrics with db.system attribute",
			"SELECT count(*) FROM Metric WHERE db.system IS NOT NULL SINCE 1 hour ago",
		},
		{
			"PostgreSQL metrics",
			"SELECT count(*) FROM Metric WHERE db.system = 'postgresql' SINCE 1 hour ago",
		},
		{
			"Metrics by name",
			"SELECT uniques(metricName) FROM Metric WHERE db.system = 'postgresql' SINCE 1 hour ago LIMIT 50",
		},
		{
			"PostgreSQL specific metrics",
			"SELECT count(*) FROM Metric WHERE metricName LIKE 'postgresql%' SINCE 1 hour ago",
		},
		{
			"Postgres wait events",
			"SELECT count(*) FROM Metric WHERE metricName = 'postgres.wait_events' SINCE 1 hour ago",
		},
		{
			"Service name metrics",
			"SELECT count(*) FROM Metric WHERE service.name = 'database-intelligence' SINCE 1 hour ago",
		},
		{
			"Environment metrics",
			"SELECT count(*) FROM Metric WHERE environment = 'e2e-test' SINCE 1 hour ago",
		},
	}

	for _, q := range queries {
		fmt.Printf("\nQuery: %s\n", q.name)
		fmt.Printf("NRQL: %s\n", q.query)
		
		result, err := nrdb.Query(ctx, q.query)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		if len(result.Results) > 0 {
			fmt.Printf("✅ Results: %v\n", result.Results[0])
		} else {
			fmt.Printf("⚠️  No results\n")
		}
	}

	// Check for any recent data
	fmt.Println("\n=== Checking for ANY recent data ===")
	recentQuery := "SELECT count(*) FROM Metric, Event, Log, Span SINCE 5 minutes ago"
	result, err := nrdb.Query(ctx, recentQuery)
	if err != nil {
		fmt.Printf("❌ Error checking recent data: %v\n", err)
	} else if len(result.Results) > 0 {
		fmt.Printf("✅ Recent data found: %v\n", result.Results[0])
	}
}

func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}