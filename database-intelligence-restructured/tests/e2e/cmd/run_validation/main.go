package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/database-intelligence/tests/e2e/framework"
	"github.com/database-intelligence/tests/e2e/pkg/validation"
)

func main() {
	fmt.Println("=== Running OHI Parity Validation Platform ===")
	fmt.Println()

	// Load environment
	loadEnvFile(".env")

	// Verify environment
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	apiKey := os.Getenv("NEW_RELIC_USER_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("NEW_RELIC_API_KEY")
	}

	if accountID == "" || apiKey == "" {
		log.Fatal("Missing required environment variables: NEW_RELIC_ACCOUNT_ID and NEW_RELIC_API_KEY/NEW_RELIC_USER_KEY")
	}

	fmt.Printf("Using New Relic Account: %s\n", accountID)
	fmt.Println()

	// Create NRDB client
	nrdb := framework.NewNRDBClient(accountID, apiKey)

	// 1. Parse Dashboard
	fmt.Println("Step 1: Parsing PostgreSQL OHI Dashboard...")
	parser := validation.NewDashboardParser()
	if err := parser.ParseFile("./testdata/postgresql_ohi_dashboard.json"); err != nil {
		log.Fatalf("Failed to parse dashboard: %v", err)
	}

	widgets := parser.GetWidgets()
	fmt.Printf("✅ Parsed %d widgets\n", len(widgets))
	fmt.Println()

	// 2. Load Metric Mappings
	fmt.Println("Step 2: Loading OHI to OTEL Metric Mappings...")
	mappings, err := validation.LoadMetricMappings("./configs/validation/metric_mappings.yaml")
	if err != nil {
		log.Fatalf("Failed to load metric mappings: %v", err)
	}
	fmt.Printf("✅ Loaded mappings for %d OHI events\n", len(mappings.Events))
	fmt.Println()

	// 3. Validate each widget
	fmt.Println("Step 3: Validating Dashboard Widgets...")
	ctx := context.Background()
	validator := validation.NewParityValidator(nrdb, mappings)

	successCount := 0
	failureCount := 0

	for i, widget := range widgets {
		fmt.Printf("\nWidget %d/%d: %s\n", i+1, len(widgets), widget.Name)
		fmt.Printf("  Type: %s\n", widget.VisualizationType)
		fmt.Printf("  Query: %.100s...\n", widget.Query)

		// Extract OHI event from query
		ohiEvent := parser.ExtractEventFromQuery(widget.Query)
		if ohiEvent == "" {
			fmt.Println("  ⚠️  Could not extract OHI event from query")
			failureCount++
			continue
		}

		fmt.Printf("  OHI Event: %s\n", ohiEvent)

		// Get OTEL equivalent
		mapping, exists := mappings.Events[ohiEvent]
		if !exists {
			fmt.Printf("  ❌ No OTEL mapping found for %s\n", ohiEvent)
			failureCount++
			continue
		}

		fmt.Printf("  OTEL Mapping: %s\n", mapping.OTELMetricType)

		// Run validation
		result, err := validator.ValidateWidget(ctx, widget)
		if err != nil {
			fmt.Printf("  ❌ Validation error: %v\n", err)
			failureCount++
			continue
		}

		if result.Valid {
			fmt.Printf("  ✅ Validation passed (score: %.2f)\n", result.Score)
			successCount++
		} else {
			fmt.Printf("  ❌ Validation failed (score: %.2f)\n", result.Score)
			for _, issue := range result.Issues {
				fmt.Printf("     - %s: %s\n", issue.Severity, issue.Message)
			}
			failureCount++
		}
	}

	fmt.Println("\n=== Validation Summary ===")
	fmt.Printf("Total Widgets: %d\n", len(widgets))
	fmt.Printf("Successful: %d\n", successCount)
	fmt.Printf("Failed: %d\n", failureCount)
	fmt.Printf("Success Rate: %.2f%%\n", float64(successCount)/float64(len(widgets))*100)

	// 4. Check for missing data
	fmt.Println("\nStep 4: Checking for Data Availability...")
	checkDataAvailability(ctx, nrdb)

	if failureCount > 0 {
		os.Exit(1)
	}
}

func checkDataAvailability(ctx context.Context, nrdb *framework.NRDBClient) {
	queries := []struct {
		name  string
		query string
	}{
		{"PostgreSQL Metrics", "SELECT count(*) FROM Metric WHERE db.system = 'postgresql' SINCE 1 hour ago"},
		{"PostgreSQL Logs", "SELECT count(*) FROM Log WHERE db.system = 'postgresql' SINCE 1 hour ago"},
		{"PostgreSQL Spans", "SELECT count(*) FROM Span WHERE db.system = 'postgresql' SINCE 1 hour ago"},
	}

	for _, q := range queries {
		result, err := nrdb.Query(ctx, q.query)
		if err != nil {
			fmt.Printf("  ❌ %s query failed: %v\n", q.name, err)
			continue
		}

		if len(result.Results) > 0 {
			if count, ok := result.Results[0]["count"].(float64); ok && count > 0 {
				fmt.Printf("  ✅ %s: Found %.0f records\n", q.name, count)
			} else {
				fmt.Printf("  ⚠️  %s: No data found\n", q.name)
			}
		}
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
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Split on first = sign
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes if present
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		
		// Set environment variable if not already set
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}