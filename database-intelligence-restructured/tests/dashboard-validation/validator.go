package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/newrelic/newrelic-client-go/newrelic"
	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
)

// QueryPair represents an OHI and OTel query pair for comparison
type QueryPair struct {
	Name      string
	OHIQuery  string
	OTelQuery string
	Tolerance float64 // Acceptable difference percentage
}

// ValidationResult contains the results of a query comparison
type ValidationResult struct {
	QueryName   string
	OHIValue    float64
	OTelValue   float64
	Difference  float64
	Percentage  float64
	Passed      bool
	Error       error
	ExecutedAt  time.Time
}

// DashboardValidator validates data parity between OHI and OTel dashboards
type DashboardValidator struct {
	client    *newrelic.NewRelic
	accountID int
	queries   []QueryPair
}

// NewDashboardValidator creates a new validator instance
func NewDashboardValidator(apiKey string, accountID int) (*DashboardValidator, error) {
	client, err := newrelic.New(newrelic.ConfigPersonalAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create New Relic client: %w", err)
	}

	return &DashboardValidator{
		client:    client,
		accountID: accountID,
		queries:   getDefaultQueryPairs(),
	}, nil
}

// getDefaultQueryPairs returns the standard query pairs for validation
func getDefaultQueryPairs() []QueryPair {
	return []QueryPair{
		{
			Name:      "Average Query Execution Time",
			OHIQuery:  "SELECT average(avg_elapsed_time_ms) FROM PostgresSlowQueries SINCE 1 hour ago",
			OTelQuery: "SELECT average(postgres.slow_queries.elapsed_time) FROM Metric WHERE metricName = 'postgres.slow_queries.elapsed_time' SINCE 1 hour ago",
			Tolerance: 0.01, // 1% tolerance
		},
		{
			Name:      "Total Query Count",
			OHIQuery:  "SELECT sum(execution_count) FROM PostgresSlowQueries SINCE 1 hour ago",
			OTelQuery: "SELECT sum(postgres.slow_queries.count) FROM Metric WHERE metricName = 'postgres.slow_queries.count' SINCE 1 hour ago",
			Tolerance: 0.001, // 0.1% tolerance for counts
		},
		{
			Name:      "Active Sessions",
			OHIQuery:  "SELECT latest(session_count) FROM PostgresInstance WHERE state = 'active' SINCE 5 minutes ago",
			OTelQuery: "SELECT latest(db.ash.active_sessions) FROM Metric WHERE attributes.state = 'active' SINCE 5 minutes ago",
			Tolerance: 0.0, // Exact match for session counts
		},
		{
			Name:      "Disk Read Operations",
			OHIQuery:  "SELECT sum(avg_disk_reads) FROM PostgresSlowQueries SINCE 1 hour ago",
			OTelQuery: "SELECT sum(postgres.slow_queries.disk_reads) FROM Metric WHERE metricName = 'postgres.slow_queries.disk_reads' SINCE 1 hour ago",
			Tolerance: 0.05, // 5% tolerance for I/O metrics
		},
		{
			Name:      "Wait Event Time",
			OHIQuery:  "SELECT sum(total_wait_time_ms) FROM PostgresWaitEvents SINCE 30 minutes ago",
			OTelQuery: "SELECT sum(db.ash.wait_events) FROM Metric WHERE metricName = 'db.ash.wait_events' SINCE 30 minutes ago",
			Tolerance: 0.02, // 2% tolerance
		},
		{
			Name:      "Blocked Sessions",
			OHIQuery:  "SELECT count(*) FROM PostgresInstance WHERE blocking_pid IS NOT NULL SINCE 10 minutes ago",
			OTelQuery: "SELECT sum(db.ash.blocked_sessions) FROM Metric WHERE metricName = 'db.ash.blocked_sessions' SINCE 10 minutes ago",
			Tolerance: 0.0, // Exact match
		},
	}
}

// ValidateQuery executes both queries and compares results
func (v *DashboardValidator) ValidateQuery(ctx context.Context, pair QueryPair) ValidationResult {
	result := ValidationResult{
		QueryName:  pair.Name,
		ExecutedAt: time.Now(),
	}

	// Execute OHI query
	ohiValue, err := v.executeQuery(ctx, pair.OHIQuery)
	if err != nil {
		result.Error = fmt.Errorf("OHI query error: %w", err)
		return result
	}
	result.OHIValue = ohiValue

	// Execute OTel query
	otelValue, err := v.executeQuery(ctx, pair.OTelQuery)
	if err != nil {
		result.Error = fmt.Errorf("OTel query error: %w", err)
		return result
	}
	result.OTelValue = otelValue

	// Calculate difference
	result.Difference = math.Abs(ohiValue - otelValue)
	if ohiValue != 0 {
		result.Percentage = (result.Difference / math.Abs(ohiValue)) * 100
	}

	// Check if within tolerance
	if ohiValue == 0 && otelValue == 0 {
		result.Passed = true
	} else if ohiValue != 0 {
		result.Passed = result.Percentage <= (pair.Tolerance * 100)
	} else {
		result.Passed = false
	}

	return result
}

// executeQuery runs a NRQL query and returns the numeric result
func (v *DashboardValidator) executeQuery(ctx context.Context, query string) (float64, error) {
	nrqlQuery := nrdb.NRQL(query)
	resp, err := v.client.Nrdb.QueryWithContext(ctx, v.accountID, nrqlQuery)
	if err != nil {
		return 0, err
	}

	// Parse the response
	if len(resp.Results) == 0 {
		return 0, nil
	}

	// Extract numeric value from first result
	result := resp.Results[0]
	for _, v := range result {
		switch val := v.(type) {
		case float64:
			return val, nil
		case int64:
			return float64(val), nil
		case json.Number:
			f, _ := val.Float64()
			return f, nil
		}
	}

	return 0, fmt.Errorf("no numeric value found in query result")
}

// ValidateAll runs all query validations
func (v *DashboardValidator) ValidateAll(ctx context.Context) ([]ValidationResult, error) {
	results := make([]ValidationResult, 0, len(v.queries))
	
	for _, pair := range v.queries {
		log.Printf("Validating: %s\n", pair.Name)
		result := v.ValidateQuery(ctx, pair)
		results = append(results, result)
		
		// Add delay to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}
	
	return results, nil
}

// GenerateReport creates a summary report of validation results
func GenerateReport(results []ValidationResult) {
	fmt.Println("\n=== Dashboard Migration Validation Report ===")
	fmt.Printf("Executed at: %s\n\n", time.Now().Format(time.RFC3339))
	
	passed := 0
	failed := 0
	
	for _, r := range results {
		status := "✅ PASS"
		if !r.Passed {
			status = "❌ FAIL"
			failed++
		} else {
			passed++
		}
		
		fmt.Printf("%s %s\n", status, r.QueryName)
		if r.Error != nil {
			fmt.Printf("   Error: %v\n", r.Error)
		} else {
			fmt.Printf("   OHI:  %.2f\n", r.OHIValue)
			fmt.Printf("   OTel: %.2f\n", r.OTelValue)
			fmt.Printf("   Diff: %.2f (%.2f%%)\n", r.Difference, r.Percentage)
		}
		fmt.Println()
	}
	
	fmt.Printf("Summary: %d passed, %d failed (%.1f%% success rate)\n", 
		passed, failed, float64(passed)/float64(len(results))*100)
	
	if failed > 0 {
		fmt.Println("\n⚠️  Some validations failed. Review the differences above.")
	} else {
		fmt.Println("\n✅ All validations passed! Safe to proceed with migration.")
	}
}

func main() {
	// Get credentials from environment
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		log.Fatal("NEW_RELIC_API_KEY environment variable is required")
	}
	
	accountID := 0
	fmt.Sscanf(os.Getenv("NEW_RELIC_ACCOUNT_ID"), "%d", &accountID)
	if accountID == 0 {
		log.Fatal("NEW_RELIC_ACCOUNT_ID environment variable is required")
	}
	
	// Create validator
	validator, err := NewDashboardValidator(apiKey, accountID)
	if err != nil {
		log.Fatalf("Failed to create validator: %v", err)
	}
	
	// Run validations
	ctx := context.Background()
	results, err := validator.ValidateAll(ctx)
	if err != nil {
		log.Fatalf("Validation failed: %v", err)
	}
	
	// Generate report
	GenerateReport(results)
	
	// Exit with appropriate code
	for _, r := range results {
		if !r.Passed {
			os.Exit(1)
		}
	}
	os.Exit(0)
}