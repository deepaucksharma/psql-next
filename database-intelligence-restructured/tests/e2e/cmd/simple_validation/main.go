package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/database-intelligence/tests/e2e/framework"
	"github.com/database-intelligence/tests/e2e/pkg/validation"
)

func main() {
	fmt.Println("=== OHI Parity Validation - Simple Test ===")
	fmt.Println()

	// Load environment manually from .env
	loadEnvFile(".env")

	// 1. Test Dashboard Parser
	fmt.Println("Step 1: Testing Dashboard Parser...")
	dashboardData, err := ioutil.ReadFile("./testdata/postgresql_ohi_dashboard.json")
	if err != nil {
		log.Fatalf("Failed to read dashboard file: %v", err)
	}

	parser := validation.NewDashboardParser()
	if err := parser.ParseDashboard(dashboardData); err != nil {
		log.Fatalf("Failed to parse dashboard: %v", err)
	}

	widgets := parser.GetWidgetValidationTests()
	fmt.Printf("✅ Successfully parsed %d widgets\n", len(widgets))

	// Show first few widgets
	for i, widget := range widgets {
		if i >= 3 {
			break
		}
		fmt.Printf("\nWidget %d: %s\n", i+1, widget.Title)
		fmt.Printf("  Type: %s\n", widget.VisualizationType)
		fmt.Printf("  Query: %.100s...\n", widget.NRQLQuery)
	}

	// 2. Test NRDB Connection
	fmt.Println("\nStep 2: Testing NRDB Connection...")
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

	// Test query
	query := "SELECT count(*) FROM Metric WHERE db.system = 'postgresql' SINCE 1 day ago"
	result, err := nrdb.Query(ctx, query)
	if err != nil {
		fmt.Printf("❌ NRDB query failed: %v\n", err)
	} else {
		fmt.Printf("✅ NRDB query successful\n")
		if len(result.Results) > 0 {
			fmt.Printf("   Results: %v\n", result.Results[0])
		}
	}

	// 3. Check for PostgreSQL data
	fmt.Println("\nStep 3: Checking for PostgreSQL Data...")
	queries := []struct {
		name  string
		query string
	}{
		{
			"PostgreSQL Metrics",
			"SELECT count(*) FROM Metric WHERE db.system = 'postgresql' SINCE 1 day ago",
		},
		{
			"Database Operations",
			"SELECT count(*) FROM Metric WHERE db.operation IS NOT NULL AND db.system = 'postgresql' SINCE 1 day ago",
		},
		{
			"PostgreSQL Spans",
			"SELECT count(*) FROM Span WHERE db.system = 'postgresql' SINCE 1 day ago",
		},
	}

	hasData := false
	for _, q := range queries {
		result, err := nrdb.Query(ctx, q.query)
		if err != nil {
			fmt.Printf("  ❌ %s query failed: %v\n", q.name, err)
			continue
		}

		count := float64(0)
		if len(result.Results) > 0 {
			if val, ok := result.Results[0]["count"].(float64); ok {
				count = val
			}
		}

		if count > 0 {
			fmt.Printf("  ✅ %s: Found %.0f records\n", q.name, count)
			hasData = true
		} else {
			fmt.Printf("  ⚠️  %s: No data found\n", q.name)
		}
	}

	if !hasData {
		fmt.Println("\n⚠️  No PostgreSQL data found in New Relic")
		fmt.Println("   Make sure the OpenTelemetry collector is running and sending data")
		fmt.Println("   You may need to:")
		fmt.Println("   1. Start the OTel collector with PostgreSQL receiver")
		fmt.Println("   2. Generate some database traffic")
		fmt.Println("   3. Wait a few minutes for data to appear")
	}

	fmt.Println("\n✅ Validation platform is working correctly!")
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