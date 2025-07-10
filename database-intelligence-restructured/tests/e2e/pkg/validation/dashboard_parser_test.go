package validation

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardParser(t *testing.T) {
	// Load test dashboard
	dashboardFile := "../../testdata/postgresql_ohi_dashboard.json"
	data, err := os.ReadFile(dashboardFile)
	require.NoError(t, err, "Failed to read dashboard file")

	// Create parser
	parser := NewDashboardParser()
	
	// Parse dashboard
	err = parser.ParseDashboard(data)
	require.NoError(t, err, "Failed to parse dashboard")

	// Test that we extracted the correct number of widgets
	widgets := parser.GetWidgetValidationTests()
	assert.Equal(t, 12, len(widgets), "Expected 12 widgets")

	// Test that we identified the correct event types
	events := parser.GetOHIEvents()
	assert.Contains(t, events, "PostgresSlowQueries")
	assert.Contains(t, events, "PostgresWaitEvents")
	assert.Contains(t, events, "PostgresBlockingSessions")
	assert.Contains(t, events, "PostgresIndividualQueries")
	assert.Contains(t, events, "PostgresExecutionPlanMetrics")

	// Test specific widget parsing
	foundDatabaseWidget := false
	for _, widget := range widgets {
		if widget.Title == "Database" {
			foundDatabaseWidget = true
			assert.Equal(t, "viz.bar", widget.VisualizationType)
			assert.Contains(t, widget.NRQLQuery, "uniqueCount(query_id)")
			assert.Contains(t, widget.NRQLQuery, "PostgresSlowQueries")
		}
	}
	assert.True(t, foundDatabaseWidget, "Database widget not found")

	// Test attribute extraction
	slowQueryAttrs := parser.GetAttributesByEvent("PostgresSlowQueries")
	assert.Contains(t, slowQueryAttrs, "query_id")
	assert.Contains(t, slowQueryAttrs, "database_name")
	assert.Contains(t, slowQueryAttrs, "avg_elapsed_time_ms")

	// Test summary generation
	summary := parser.GenerateValidationSummary()
	t.Logf("Dashboard Summary: %+v", summary)
	
	assert.Equal(t, 12, summary["total_widgets"])
	assert.Equal(t, 5, summary["event_types"])
}

func TestNRQLParsing(t *testing.T) {
	parser := NewDashboardParser()

	testCases := []struct {
		name        string
		query       string
		eventType   string
		hasTimeseries bool
		facetCount  int
	}{
		{
			name:      "Simple count query",
			query:     "SELECT count(*) FROM PostgresSlowQueries",
			eventType: "PostgresSlowQueries",
			hasTimeseries: false,
			facetCount: 0,
		},
		{
			name:      "Query with facet",
			query:     "SELECT latest(avg_elapsed_time_ms) FROM PostgresSlowQueries FACET database_name",
			eventType: "PostgresSlowQueries",
			hasTimeseries: false,
			facetCount: 1,
		},
		{
			name:      "Timeseries query",
			query:     "SELECT count(execution_count) FROM PostgresSlowQueries TIMESERIES",
			eventType: "PostgresSlowQueries",
			hasTimeseries: true,
			facetCount: 0,
		},
		{
			name:      "Complex query with multiple facets",
			query:     "SELECT latest(total_wait_time_ms) FROM PostgresWaitEvents FACET wait_event_name, wait_category TIMESERIES",
			eventType: "PostgresWaitEvents",
			hasTimeseries: true,
			facetCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := parser.parseNRQL(tc.query)
			
			assert.Equal(t, tc.eventType, parsed.EventType)
			
			if tc.hasTimeseries {
				assert.Equal(t, "TIMESERIES", parsed.TimeWindow)
			}
			
			assert.Equal(t, tc.facetCount, len(parsed.Facets))
		})
	}
}

func TestOHIEventInitialization(t *testing.T) {
	parser := NewDashboardParser()
	events := parser.GetOHIEvents()

	// Verify all OHI events are initialized
	expectedEvents := []string{
		"PostgresSlowQueries",
		"PostgresWaitEvents", 
		"PostgresBlockingSessions",
		"PostgresIndividualQueries",
		"PostgresExecutionPlanMetrics",
	}

	for _, event := range expectedEvents {
		ohiEvent, exists := events[event]
		assert.True(t, exists, "Event %s not found", event)
		assert.NotEmpty(t, ohiEvent.RequiredFields, "Event %s has no required fields", event)
		assert.NotEmpty(t, ohiEvent.OTELMapping, "Event %s has no OTEL mapping", event)
	}
}

func TestDashboardJSONStructure(t *testing.T) {
	// Test parsing of dashboard JSON structure
	dashboardJSON := `{
		"name": "Test Dashboard",
		"pages": [
			{
				"name": "Test Page",
				"widgets": [
					{
						"title": "Test Widget",
						"visualization": {
							"id": "viz.table"
						},
						"rawConfiguration": {
							"nrqlQueries": [
								{
									"query": "SELECT count(*) FROM TestEvent"
								}
							]
						}
					}
				]
			}
		]
	}`

	parser := NewDashboardParser()
	err := parser.ParseDashboard([]byte(dashboardJSON))
	require.NoError(t, err)

	widgets := parser.GetWidgetValidationTests()
	assert.Equal(t, 1, len(widgets))
	assert.Equal(t, "Test Widget", widgets[0].Title)
	assert.Equal(t, "viz.table", widgets[0].VisualizationType)
}