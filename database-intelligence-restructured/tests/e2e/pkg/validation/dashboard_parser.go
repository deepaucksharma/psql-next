package validation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// DashboardParser parses New Relic dashboard JSON to extract NRQL queries and validation requirements
type DashboardParser struct {
	dashboardJSON map[string]interface{}
	nrqlQueries   []NRQLQuery
	ohiEvents     map[string]*OHIEvent
	attributes    map[string][]string
}

// NRQLQuery represents a parsed NRQL query from the dashboard
type NRQLQuery struct {
	WidgetTitle    string
	Query          string
	EventType      string
	Metrics        []string
	Attributes     []string
	Aggregations   []string
	TimeWindow     string
	Facets         []string
	Visualization  string
}

// OHIEvent represents an OHI event type with its fields
type OHIEvent struct {
	Name           string
	RequiredFields []string
	OptionalFields []string
	OTELMapping    string
	Description    string
}

// DashboardWidget represents a widget from the dashboard
type DashboardWidget struct {
	Title              string
	NRQLQuery          string
	VisualizationType  string
	RequiredMetrics    []string
	RequiredAttributes []string
	Layout             WidgetLayout
}

// WidgetLayout represents widget positioning
type WidgetLayout struct {
	Column int
	Row    int
	Width  int
	Height int
}

// NewDashboardParser creates a new dashboard parser
func NewDashboardParser() *DashboardParser {
	return &DashboardParser{
		nrqlQueries: []NRQLQuery{},
		ohiEvents:   make(map[string]*OHIEvent),
		attributes:  make(map[string][]string),
	}
}

// ParseDashboard parses a dashboard JSON and extracts all validation requirements
func (p *DashboardParser) ParseDashboard(dashboardData []byte) error {
	if err := json.Unmarshal(dashboardData, &p.dashboardJSON); err != nil {
		return fmt.Errorf("failed to parse dashboard JSON: %w", err)
	}

	// Initialize OHI events based on PostgreSQL dashboard
	p.initializeOHIEvents()

	// Extract pages
	pages, ok := p.dashboardJSON["pages"].([]interface{})
	if !ok {
		return fmt.Errorf("dashboard has no pages")
	}

	// Parse each page
	for _, page := range pages {
		if err := p.parsePage(page.(map[string]interface{})); err != nil {
			return fmt.Errorf("failed to parse page: %w", err)
		}
	}

	return nil
}

// initializeOHIEvents sets up known OHI event types
func (p *DashboardParser) initializeOHIEvents() {
	// PostgresSlowQueries event
	p.ohiEvents["PostgresSlowQueries"] = &OHIEvent{
		Name:        "PostgresSlowQueries",
		OTELMapping: "Metric WHERE db.system = 'postgresql'",
		RequiredFields: []string{
			"query_id", "query_text", "database_name",
			"execution_count", "avg_elapsed_time_ms",
		},
		OptionalFields: []string{
			"schema_name", "statement_type",
			"avg_disk_reads", "avg_disk_writes",
			"total_exec_time", "rows",
		},
		Description: "Slow query performance metrics",
	}

	// PostgresWaitEvents event
	p.ohiEvents["PostgresWaitEvents"] = &OHIEvent{
		Name:        "PostgresWaitEvents",
		OTELMapping: "Metric WHERE db.system = 'postgresql' AND wait.event_name IS NOT NULL",
		RequiredFields: []string{
			"wait_event_name", "total_wait_time_ms",
		},
		OptionalFields: []string{
			"wait_category", "database_name", "query_id",
		},
		Description: "Database wait event metrics",
	}

	// PostgresBlockingSessions event
	p.ohiEvents["PostgresBlockingSessions"] = &OHIEvent{
		Name:        "PostgresBlockingSessions",
		OTELMapping: "Log WHERE db.system = 'postgresql' AND blocking.detected = true",
		RequiredFields: []string{
			"blocked_pid", "blocking_pid",
			"database_name",
		},
		OptionalFields: []string{
			"blocked_query", "blocked_query_id", "blocked_query_start",
			"blocking_query", "blocking_query_id", "blocking_query_start",
			"blocking_database",
		},
		Description: "Blocking session detection",
	}

	// PostgresIndividualQueries event
	p.ohiEvents["PostgresIndividualQueries"] = &OHIEvent{
		Name:        "PostgresIndividualQueries",
		OTELMapping: "Metric WHERE db.system = 'postgresql'",
		RequiredFields: []string{
			"query_id", "query_text",
		},
		OptionalFields: []string{
			"avg_cpu_time_ms", "plan_id",
		},
		Description: "Individual query details",
	}

	// PostgresExecutionPlanMetrics event
	p.ohiEvents["PostgresExecutionPlanMetrics"] = &OHIEvent{
		Name:        "PostgresExecutionPlanMetrics",
		OTELMapping: "Metric WHERE db.system = 'postgresql' AND db.plan.node_type IS NOT NULL",
		RequiredFields: []string{
			"plan_id", "level_id", "node_type",
		},
		OptionalFields: []string{
			"query_id", "query_text", "total_cost", "startup_cost",
			"plan_rows", "actual_startup_time", "actual_total_time",
			"actual_rows", "actual_loops", "shared_hit_block",
			"shared_read_blocks", "shared_dirtied_blocks",
			"shared_written_blocks", "local_hit_block",
			"local_read_blocks", "local_dirtied_blocks",
			"local_written_blocks", "temp_read_block",
			"temp_written_blocks", "database_name",
		},
		Description: "Query execution plan metrics",
	}
}

// parsePage parses a dashboard page
func (p *DashboardParser) parsePage(page map[string]interface{}) error {
	pageName := page["name"].(string)
	
	widgets, ok := page["widgets"].([]interface{})
	if !ok {
		return nil // Page might not have widgets
	}

	for _, widget := range widgets {
		widgetMap := widget.(map[string]interface{})
		if err := p.parseWidget(pageName, widgetMap); err != nil {
			return fmt.Errorf("failed to parse widget: %w", err)
		}
	}

	return nil
}

// parseWidget parses a dashboard widget
func (p *DashboardParser) parseWidget(pageName string, widget map[string]interface{}) error {
	title := widget["title"].(string)
	
	// Extract visualization type
	vizType := ""
	if viz, ok := widget["visualization"].(map[string]interface{}); ok {
		vizType = viz["id"].(string)
	}

	// Extract NRQL queries
	if rawConfig, ok := widget["rawConfiguration"].(map[string]interface{}); ok {
		if nrqlQueries, ok := rawConfig["nrqlQueries"].([]interface{}); ok {
			for _, nrqlQuery := range nrqlQueries {
				queryMap := nrqlQuery.(map[string]interface{})
				query := queryMap["query"].(string)
				
				parsedQuery := p.parseNRQL(query)
				parsedQuery.WidgetTitle = title
				parsedQuery.Visualization = vizType
				
				p.nrqlQueries = append(p.nrqlQueries, parsedQuery)
				
				// Track attributes by event type
				if parsedQuery.EventType != "" {
					if _, exists := p.attributes[parsedQuery.EventType]; !exists {
						p.attributes[parsedQuery.EventType] = []string{}
					}
					p.attributes[parsedQuery.EventType] = append(p.attributes[parsedQuery.EventType], parsedQuery.Attributes...)
				}
			}
		}
	}

	return nil
}

// parseNRQL parses a NRQL query to extract its components
func (p *DashboardParser) parseNRQL(query string) NRQLQuery {
	parsed := NRQLQuery{
		Query:        query,
		Metrics:      []string{},
		Attributes:   []string{},
		Aggregations: []string{},
		Facets:       []string{},
	}

	// Normalize query for parsing
	normalizedQuery := strings.ToUpper(query)

	// Extract event type from FROM clause
	fromRegex := regexp.MustCompile(`FROM\s+(\w+)`)
	if matches := fromRegex.FindStringSubmatch(query); len(matches) > 1 {
		parsed.EventType = matches[1]
	}

	// Extract SELECT clause
	selectRegex := regexp.MustCompile(`SELECT\s+(.+?)\s+FROM`)
	if matches := selectRegex.FindStringSubmatch(normalizedQuery); len(matches) > 1 {
		parsed.parseSelectClause(matches[1], query)
	}

	// Extract FACET clause
	facetRegex := regexp.MustCompile(`FACET\s+(.+?)(?:\s+LIMIT|\s+SINCE|\s+TIMESERIES|$)`)
	if matches := facetRegex.FindStringSubmatch(normalizedQuery); len(matches) > 1 {
		facets := strings.Split(matches[1], ",")
		for _, facet := range facets {
			parsed.Facets = append(parsed.Facets, strings.TrimSpace(facet))
		}
	}

	// Check for TIMESERIES
	if strings.Contains(normalizedQuery, "TIMESERIES") {
		parsed.TimeWindow = "TIMESERIES"
	}

	// Extract SINCE clause
	sinceRegex := regexp.MustCompile(`SINCE\s+(.+?)(?:\s+|$)`)
	if matches := sinceRegex.FindStringSubmatch(query); len(matches) > 1 {
		parsed.TimeWindow = matches[1]
	}

	return parsed
}

// parseSelectClause parses the SELECT portion of a NRQL query
func (q *NRQLQuery) parseSelectClause(selectClause string, originalQuery string) {
	// Common aggregation functions
	aggregations := []string{
		"count", "uniqueCount", "sum", "average", "latest",
		"max", "min", "percentile", "histogram",
	}

	// Extract aggregations and their targets
	for _, agg := range aggregations {
		regex := regexp.MustCompile(fmt.Sprintf(`(?i)%s\s*\(\s*([^)]+)\s*\)`, agg))
		matches := regex.FindAllStringSubmatch(originalQuery, -1)
		
		for _, match := range matches {
			if len(match) > 1 {
				q.Aggregations = append(q.Aggregations, agg)
				
				// Extract the field being aggregated
				field := strings.TrimSpace(match[1])
				field = strings.Trim(field, "'\"")
				
				// Check if it's a simple field or expression
				if !strings.Contains(field, " ") && !strings.Contains(field, "(") {
					q.Attributes = append(q.Attributes, field)
				}
			}
		}
	}
}

// GetWidgetValidationTests generates validation tests for all widgets
func (p *DashboardParser) GetWidgetValidationTests() []DashboardWidget {
	widgets := []DashboardWidget{}
	
	for _, query := range p.nrqlQueries {
		widget := DashboardWidget{
			Title:              query.WidgetTitle,
			NRQLQuery:          query.Query,
			VisualizationType:  query.Visualization,
			RequiredMetrics:    query.Metrics,
			RequiredAttributes: query.Attributes,
		}
		
		widgets = append(widgets, widget)
	}
	
	return widgets
}

// GetOHIEvents returns all detected OHI events
func (p *DashboardParser) GetOHIEvents() map[string]*OHIEvent {
	return p.ohiEvents
}

// GetAttributesByEvent returns all attributes used for a specific event type
func (p *DashboardParser) GetAttributesByEvent(eventType string) []string {
	return p.attributes[eventType]
}

// GenerateValidationSummary creates a summary of validation requirements
func (p *DashboardParser) GenerateValidationSummary() map[string]interface{} {
	summary := map[string]interface{}{
		"total_widgets":     len(p.nrqlQueries),
		"event_types":       len(p.ohiEvents),
		"unique_attributes": p.getUniqueAttributeCount(),
		"widgets_by_viz":    p.getWidgetsByVisualization(),
		"events_used":       p.getUsedEvents(),
	}
	
	return summary
}

// getUniqueAttributeCount returns the total number of unique attributes across all events
func (p *DashboardParser) getUniqueAttributeCount() int {
	uniqueAttrs := make(map[string]bool)
	
	for _, attrs := range p.attributes {
		for _, attr := range attrs {
			uniqueAttrs[attr] = true
		}
	}
	
	return len(uniqueAttrs)
}

// getWidgetsByVisualization groups widgets by their visualization type
func (p *DashboardParser) getWidgetsByVisualization() map[string]int {
	vizCounts := make(map[string]int)
	
	for _, query := range p.nrqlQueries {
		vizCounts[query.Visualization]++
	}
	
	return vizCounts
}

// getUsedEvents returns list of OHI events actually used in the dashboard
func (p *DashboardParser) getUsedEvents() []string {
	events := make(map[string]bool)
	
	for _, query := range p.nrqlQueries {
		if query.EventType != "" {
			events[query.EventType] = true
		}
	}
	
	used := []string{}
	for event := range events {
		used = append(used, event)
	}
	
	return used
}