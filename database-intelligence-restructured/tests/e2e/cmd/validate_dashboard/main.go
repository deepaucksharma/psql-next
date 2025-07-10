package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/database-intelligence/tests/e2e/pkg/validation"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: validate_dashboard <dashboard.json>")
	}

	dashboardFile := os.Args[1]
	
	// Read dashboard file
	data, err := os.ReadFile(dashboardFile)
	if err != nil {
		log.Fatalf("Failed to read dashboard file: %v", err)
	}

	// Create parser
	parser := validation.NewDashboardParser()
	
	// Parse dashboard
	if err := parser.ParseDashboard(data); err != nil {
		log.Fatalf("Failed to parse dashboard: %v", err)
	}

	// Print results
	fmt.Println("Dashboard Analysis Results:")
	fmt.Println("==========================")
	
	// Summary
	summary := parser.GenerateValidationSummary()
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Printf("Summary:\n%s\n\n", summaryJSON)
	
	// Widgets
	widgets := parser.GetWidgetValidationTests()
	fmt.Printf("Found %d widgets:\n", len(widgets))
	for i, widget := range widgets {
		fmt.Printf("%d. %s (%s)\n", i+1, widget.Title, widget.VisualizationType)
		fmt.Printf("   Query: %.100s...\n", widget.NRQLQuery)
	}
	
	// OHI Events
	fmt.Println("\nOHI Events Used:")
	events := parser.GetOHIEvents()
	for name, event := range events {
		fmt.Printf("- %s\n", name)
		fmt.Printf("  Required Fields: %v\n", event.RequiredFields)
		fmt.Printf("  OTEL Mapping: %s\n", event.OTELMapping)
	}
	
	// Attributes by event
	fmt.Println("\nAttributes by Event:")
	for _, eventName := range summary["events_used"].([]string) {
		attrs := parser.GetAttributesByEvent(eventName)
		fmt.Printf("- %s: %v\n", eventName, attrs)
	}
}