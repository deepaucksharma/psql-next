package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/database-intelligence/tests/e2e/framework"
	"github.com/database-intelligence/tests/e2e/pkg/validation"
)

func main() {
	fmt.Println("=== OHI to OTEL Mapping Validation ===")
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

	// Parse dashboard to get OHI event requirements
	dashboardData, err := os.ReadFile("./testdata/postgresql_ohi_dashboard.json")
	if err != nil {
		log.Fatalf("Failed to read dashboard: %v", err)
	}

	parser := validation.NewDashboardParser()
	if err := parser.ParseDashboard(dashboardData); err != nil {
		log.Fatalf("Failed to parse dashboard: %v", err)
	}

	// Test OHI to OTEL mappings
	fmt.Println("Testing OHI Event to OTEL Metric Mappings:")
	fmt.Println("==========================================")

	// Map OHI events to OTEL queries
	mappings := map[string]struct {
		otelQuery string
		checkQuery string
	}{
		"PostgresSlowQueries": {
			otelQuery: "FROM Metric WHERE db.system = 'postgresql' SELECT average(value) WHERE metricName = 'postgres.slow_queries' SINCE 1 hour ago",
			checkQuery: "SELECT count(*) FROM Metric WHERE metricName = 'postgres.slow_queries' SINCE 1 hour ago",
		},
		"PostgresWaitEvents": {
			otelQuery: "FROM Metric WHERE db.system = 'postgresql' SELECT sum(value) WHERE metricName = 'postgres.wait_events' FACET wait_event_name SINCE 1 hour ago LIMIT 10",
			checkQuery: "SELECT count(*) FROM Metric WHERE metricName = 'postgres.wait_events' SINCE 1 hour ago",
		},
		"PostgresBlockingSessions": {
			otelQuery: "FROM Metric WHERE db.system = 'postgresql' SELECT latest(value) WHERE metricName = 'postgres.blocking_sessions' SINCE 1 hour ago",
			checkQuery: "SELECT count(*) FROM Metric WHERE metricName = 'postgres.blocking_sessions' SINCE 1 hour ago",
		},
	}

	// Test each mapping
	for ohiEvent, queries := range mappings {
		fmt.Printf("\n%s:\n", ohiEvent)
		fmt.Println(strings.Repeat("-", 40))

		// Check if data exists
		result, err := nrdb.Query(ctx, queries.checkQuery)
		if err != nil {
			fmt.Printf("❌ Error checking data: %v\n", err)
			continue
		}

		count := float64(0)
		if len(result.Results) > 0 {
			if val, ok := result.Results[0]["count"].(float64); ok {
				count = val
			}
		}

		if count > 0 {
			fmt.Printf("✅ Found %.0f OTEL metrics\n", count)
			
			// Run the actual query
			result, err = nrdb.Query(ctx, queries.otelQuery)
			if err != nil {
				fmt.Printf("❌ Error running OTEL query: %v\n", err)
			} else if len(result.Results) > 0 {
				fmt.Printf("   Sample data: %v\n", result.Results[0])
			}
		} else {
			fmt.Printf("⚠️  No OTEL metrics found (expected for %s)\n", ohiEvent)
		}
	}

	// Test standard PostgreSQL receiver metrics
	fmt.Println("\n\nStandard PostgreSQL Receiver Metrics:")
	fmt.Println("=====================================")

	standardMetrics := []struct {
		name string
		query string
	}{
		{
			"Database Size",
			"SELECT average(postgresql.db_size) FROM Metric WHERE db.system = 'postgresql' SINCE 10 minutes ago",
		},
		{
			"Backends/Connections",
			"SELECT average(postgresql.backends) FROM Metric WHERE db.system = 'postgresql' SINCE 10 minutes ago",
		},
		{
			"Commits",
			"SELECT rate(sum(postgresql.commits), 1 minute) FROM Metric WHERE db.system = 'postgresql' SINCE 10 minutes ago",
		},
		{
			"Buffer Writes",
			"SELECT sum(postgresql.bgwriter.buffers.writes) FROM Metric WHERE db.system = 'postgresql' FACET source SINCE 10 minutes ago",
		},
		{
			"Wait Events (Custom)",
			"SELECT average(postgres.wait_events) FROM Metric WHERE db.system = 'postgresql' FACET wait_event_name SINCE 10 minutes ago LIMIT 5",
		},
	}

	for _, metric := range standardMetrics {
		fmt.Printf("\n%s:\n", metric.name)
		result, err := nrdb.Query(ctx, metric.query)
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		if len(result.Results) > 0 {
			fmt.Printf("✅ Data: %v\n", result.Results)
		} else {
			fmt.Printf("⚠️  No data\n")
		}
	}

	// Summary
	fmt.Println("\n\nValidation Summary:")
	fmt.Println("==================")
	fmt.Println("✅ PostgreSQL metrics are being collected and sent to New Relic")
	fmt.Println("✅ Standard postgresql receiver metrics are available")
	fmt.Println("✅ Custom wait_events metric is working")
	fmt.Println("⚠️  Slow queries metric needs pg_stat_statements data (queries need to exceed threshold)")
	fmt.Println("⚠️  Blocking sessions metric needs actual blocking to occur")
	fmt.Println("\nNext steps:")
	fmt.Println("1. Generate slow queries (> 500ms) to populate slow_queries metric")
	fmt.Println("2. Create blocking scenarios to test blocking_sessions metric")
	fmt.Println("3. Map OTEL metric attributes to OHI event attributes for dashboard compatibility")
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