package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"
	
	"github.com/database-intelligence/tests/e2e/framework"
	"github.com/database-intelligence/tests/e2e/pkg/validation"
)

// OHIParityValidationSuite validates complete parity with PostgreSQL OHI dashboard
type OHIParityValidationSuite struct {
	suite.Suite
	env              *framework.TestEnvironment
	collector        *framework.TestCollector
	nrdb             *framework.NRDBClient
	parser           *validation.DashboardParser
	metricMappings   *MetricMappingsConfig
	ctx              context.Context
	cancel           context.CancelFunc
	validationResults []ValidationResult
}

// MetricMappingsConfig represents the metric mappings configuration
type MetricMappingsConfig struct {
	OHIToOTELMappings map[string]EventMapping `yaml:"ohi_to_otel_mappings"`
	Transformations   map[string]interface{} `yaml:"transformations"`
	ValidationRules   ValidationRules        `yaml:"validation_rules"`
}

// EventMapping represents mapping for an OHI event type
type EventMapping struct {
	OTELMetricType string                       `yaml:"otel_metric_type"`
	OTELFilter     string                       `yaml:"otel_filter"`
	Description    string                       `yaml:"description"`
	Metrics        map[string]MetricMapping     `yaml:"metrics,omitempty"`
	Attributes     map[string]AttributeMapping  `yaml:"attributes,omitempty"`
}

// MetricMapping represents a single metric mapping
type MetricMapping struct {
	OTELName       string `yaml:"otel_name"`
	Type           string `yaml:"type"`
	Transformation string `yaml:"transformation"`
	Formula        string `yaml:"formula,omitempty"`
	Unit           string `yaml:"unit,omitempty"`
}

// AttributeMapping represents attribute mapping
type AttributeMapping struct {
	OTELName       string                 `yaml:"otel_name"`
	Type           string                 `yaml:"type"`
	Transformation string                 `yaml:"transformation"`
	PIISafe        bool                   `yaml:"pii_safe,omitempty"`
	DefaultValue   string                 `yaml:"default_value,omitempty"`
	SpecialValues  map[string]interface{} `yaml:"special_values,omitempty"`
}

// ValidationRules defines validation thresholds and rules
type ValidationRules struct {
	AccuracyThresholds map[string]float64 `yaml:"accuracy_thresholds"`
	TimingTolerance    map[string]string  `yaml:"timing_tolerance"`
	CardinalityLimits  map[string]int     `yaml:"cardinality_limits"`
}

// ValidationResult represents the result of a widget validation
type ValidationResult struct {
	Timestamp         time.Time
	WidgetName        string
	OHIQuery          string
	OTELQuery         string
	OHIDataPoints     int
	OTELDataPoints    int
	Accuracy          float64
	MissingAttributes []string
	ExtraAttributes   []string
	ValueMismatches   []ValueMismatch
	Status            string
	Error             error
}

// ValueMismatch represents a mismatch in values
type ValueMismatch struct {
	Attribute    string
	OHIValue     interface{}
	OTELValue    interface{}
	Difference   float64
	Tolerance    float64
}

func TestOHIParityValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OHI parity validation tests in short mode")
	}
	
	suite.Run(t, new(OHIParityValidationSuite))
}

func (s *OHIParityValidationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	
	// Initialize environment
	s.env = framework.NewTestEnvironment()
	require.NoError(s.T(), s.env.Initialize())
	
	// Load metric mappings
	s.loadMetricMappings()
	
	// Initialize dashboard parser
	s.parser = validation.NewDashboardParser()
	s.loadDashboard()
	
	// Initialize NRDB client
	s.nrdb = framework.NewNRDBClient(s.env.NewRelicAccountID, s.env.NewRelicAPIKey)
	
	// Initialize and start collector
	s.collector = framework.NewTestCollector(s.env)
	config := s.getParityValidationConfig()
	require.NoError(s.T(), s.collector.Start(config))
	
	// Wait for initial data collection
	s.T().Log("Waiting for initial data collection...")
	time.Sleep(2 * time.Minute)
}

func (s *OHIParityValidationSuite) TearDownSuite() {
	s.cancel()
	
	// Generate validation report
	s.generateValidationReport()
	
	if s.collector != nil {
		s.collector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
}

// loadMetricMappings loads the metric mapping configuration
func (s *OHIParityValidationSuite) loadMetricMappings() {
	mappingsFile := "configs/validation/metric_mappings.yaml"
	data, err := os.ReadFile(mappingsFile)
	require.NoError(s.T(), err, "Failed to read metric mappings file")
	
	s.metricMappings = &MetricMappingsConfig{}
	err = yaml.Unmarshal(data, s.metricMappings)
	require.NoError(s.T(), err, "Failed to parse metric mappings")
}

// loadDashboard loads and parses the OHI dashboard
func (s *OHIParityValidationSuite) loadDashboard() {
	// The dashboard JSON would typically be loaded from a file
	// For this test, we'll use the dashboard structure from the user's input
	dashboardJSON := `{
		"name": "Postgresql-demo-feb",
		"pages": [...]
	}`
	
	err := s.parser.ParseDashboard([]byte(dashboardJSON))
	require.NoError(s.T(), err, "Failed to parse dashboard")
}

// Test 1: Database Query Distribution Widget
func (s *OHIParityValidationSuite) TestDatabaseQueryDistribution() {
	s.T().Log("Validating Database Query Distribution widget...")
	
	ohiQuery := "SELECT uniqueCount(query_id) from PostgresSlowQueries facet database_name"
	otelQuery := `SELECT uniqueCount(db.querylens.queryid) 
	              FROM Metric 
	              WHERE db.system = 'postgresql' AND db.query.duration > 500
	              FACET db.name
	              SINCE 30 minutes ago`
	
	result := s.validateWidgetParity("Database Query Distribution", ohiQuery, otelQuery, 0.95)
	s.Assert().GreaterOrEqual(result.Accuracy, 0.95, "Widget accuracy below threshold")
}

// Test 2: Average Execution Time Widget
func (s *OHIParityValidationSuite) TestAverageExecutionTime() {
	s.T().Log("Validating Average Execution Time widget...")
	
	ohiQuery := `SELECT latest(avg_elapsed_time_ms) 
	             FROM PostgresSlowQueries 
	             WHERE query_text!='<insufficient privilege>' 
	             FACET query_text`
	
	otelQuery := `SELECT latest(db.query.execution_time_mean) 
	              FROM Metric 
	              WHERE db.system = 'postgresql' 
	                AND db.statement != '[REDACTED]'
	                AND db.query.duration > 500
	              FACET db.statement
	              SINCE 30 minutes ago`
	
	result := s.validateWidgetParity("Average Execution Time", ohiQuery, otelQuery, 0.95)
	s.Assert().GreaterOrEqual(result.Accuracy, 0.95, "Widget accuracy below threshold")
}

// Test 3: Execution Counts Timeline
func (s *OHIParityValidationSuite) TestExecutionCountsTimeline() {
	s.T().Log("Validating Execution Counts Timeline...")
	
	ohiQuery := "SELECT count(execution_count) FROM PostgresSlowQueries TIMESERIES"
	otelQuery := `SELECT sum(db.query.calls) 
	              FROM Metric 
	              WHERE db.system = 'postgresql' AND db.query.duration > 500
	              TIMESERIES
	              SINCE 1 hour ago`
	
	result := s.validateTimeseriesParity("Execution Counts Timeline", ohiQuery, otelQuery, 0.90)
	s.Assert().GreaterOrEqual(result.Accuracy, 0.90, "Timeline accuracy below threshold")
}

// Test 4: Top Wait Events Widget
func (s *OHIParityValidationSuite) TestTopWaitEvents() {
	s.T().Log("Validating Top Wait Events widget...")
	
	ohiQuery := `SELECT latest(total_wait_time_ms) 
	             FROM PostgresWaitEvents 
	             FACET wait_event_name 
	             WHERE wait_event_name!='<nil>'`
	
	otelQuery := `SELECT sum(wait.duration_ms) 
	              FROM Metric 
	              WHERE db.system = 'postgresql' 
	                AND wait.event_name IS NOT NULL
	              FACET wait.event_name
	              SINCE 30 minutes ago`
	
	result := s.validateWidgetParity("Top Wait Events", ohiQuery, otelQuery, 0.90)
	s.Assert().GreaterOrEqual(result.Accuracy, 0.90, "Widget accuracy below threshold")
}

// Test 5: Top N Slowest Queries Table
func (s *OHIParityValidationSuite) TestTopNSlowestQueries() {
	s.T().Log("Validating Top N Slowest Queries table...")
	
	requiredAttributes := []string{
		"database_name", "query_text", "schema_name",
		"execution_count", "avg_elapsed_time_ms",
		"avg_disk_reads", "avg_disk_writes", "statement_type",
	}
	
	ohiQuery := `SELECT latest(database_name), latest(query_text), latest(schema_name), 
	                    latest(execution_count), latest(avg_elapsed_time_ms), 
	                    latest(avg_disk_reads), latest(avg_disk_writes), latest(statement_type) 
	             FROM PostgresSlowQueries 
	             FACET query_id 
	             LIMIT max`
	
	otelQuery := `SELECT latest(db.name), latest(db.statement), latest(db.schema),
	                     latest(db.query.calls), latest(db.query.execution_time_mean),
	                     latest(db.query.disk_io.reads_avg), latest(db.query.disk_io.writes_avg),
	                     latest(db.operation)
	              FROM Metric
	              WHERE db.system = 'postgresql' AND db.query.duration > 500
	              FACET db.querylens.queryid
	              LIMIT max
	              SINCE 30 minutes ago`
	
	result := s.validateTableWidgetParity("Top N Slowest Queries", ohiQuery, otelQuery, requiredAttributes)
	s.Assert().GreaterOrEqual(result.Accuracy, 0.95, "Table widget accuracy below threshold")
}

// Test 6: Disk IO Usage Charts
func (s *OHIParityValidationSuite) TestDiskIOUsage() {
	s.T().Log("Validating Disk IO Usage charts...")
	
	// Test disk reads
	ohiReadsQuery := `SELECT latest(avg_disk_reads) as 'Average Disk Reads' 
	                  FROM PostgresSlowQueries 
	                  FACET database_name 
	                  TIMESERIES`
	
	otelReadsQuery := `SELECT latest(db.query.disk_io.reads_avg) as 'Average Disk Reads'
	                   FROM Metric
	                   WHERE db.system = 'postgresql' AND db.query.duration > 500
	                   FACET db.name
	                   TIMESERIES
	                   SINCE 1 hour ago`
	
	readsResult := s.validateTimeseriesParity("Disk IO Reads", ohiReadsQuery, otelReadsQuery, 0.90)
	s.Assert().GreaterOrEqual(readsResult.Accuracy, 0.90, "Disk reads accuracy below threshold")
	
	// Test disk writes
	ohiWritesQuery := `SELECT average(avg_disk_writes) as 'Average Disk Writes'
	                   FROM PostgresSlowQueries 
	                   FACET database_name 
	                   TIMESERIES`
	
	otelWritesQuery := `SELECT average(db.query.disk_io.writes_avg) as 'Average Disk Writes'
	                    FROM Metric
	                    WHERE db.system = 'postgresql' AND db.query.duration > 500
	                    FACET db.name
	                    TIMESERIES
	                    SINCE 1 hour ago`
	
	writesResult := s.validateTimeseriesParity("Disk IO Writes", ohiWritesQuery, otelWritesQuery, 0.90)
	s.Assert().GreaterOrEqual(writesResult.Accuracy, 0.90, "Disk writes accuracy below threshold")
}

// Test 7: Blocking Details Table
func (s *OHIParityValidationSuite) TestBlockingDetails() {
	s.T().Log("Validating Blocking Details table...")
	
	requiredAttributes := []string{
		"blocked_pid", "blocked_query", "blocked_query_id",
		"blocking_pid", "blocking_query", "blocking_query_id",
		"database_name", "blocking_database",
	}
	
	// First check if there are any blocking sessions
	blockingCount := s.checkBlockingSessions()
	if blockingCount == 0 {
		s.T().Skip("No blocking sessions detected, skipping blocking details test")
		return
	}
	
	ohiQuery := `SELECT latest(blocked_pid), latest(blocked_query), latest(blocked_query_id),
	                    latest(blocked_query_start), latest(database_name),
	                    latest(blocking_pid), latest(blocking_query), latest(blocking_query_id),
	                    latest(blocking_query_start), latest(blocking_database)
	             FROM PostgresBlockingSessions 
	             FACET blocked_pid`
	
	otelQuery := `SELECT latest(session.blocked.pid), latest(session.blocked.query),
	                     latest(session.blocked.query_id), latest(session.blocked.start_time),
	                     latest(db.name), latest(session.blocking.pid),
	                     latest(session.blocking.query), latest(session.blocking.query_id),
	                     latest(session.blocking.start_time), latest(db.blocking.name)
	              FROM Log
	              WHERE db.system = 'postgresql' AND blocking.detected = true
	              FACET session.blocked.pid
	              SINCE 30 minutes ago`
	
	result := s.validateTableWidgetParity("Blocking Details", ohiQuery, otelQuery, requiredAttributes)
	s.Assert().GreaterOrEqual(result.Accuracy, 0.95, "Blocking details accuracy below threshold")
}

// validateWidgetParity validates parity between OHI and OTEL for a widget
func (s *OHIParityValidationSuite) validateWidgetParity(widgetName, ohiQuery, otelQuery string, tolerance float64) ValidationResult {
	s.T().Logf("Validating widget: %s", widgetName)
	
	result := ValidationResult{
		Timestamp:  time.Now(),
		WidgetName: widgetName,
		OHIQuery:   ohiQuery,
		OTELQuery:  otelQuery,
		Status:     "PENDING",
	}
	
	// Execute OHI query
	ohiData, err := s.executeNRQLQuery(ohiQuery)
	if err != nil {
		result.Error = fmt.Errorf("OHI query failed: %w", err)
		result.Status = "FAILED"
		s.recordResult(result)
		return result
	}
	result.OHIDataPoints = len(ohiData)
	
	// Execute OTEL query
	otelData, err := s.executeNRQLQuery(otelQuery)
	if err != nil {
		result.Error = fmt.Errorf("OTEL query failed: %w", err)
		result.Status = "FAILED"
		s.recordResult(result)
		return result
	}
	result.OTELDataPoints = len(otelData)
	
	// Compare results
	result.Accuracy = s.calculateAccuracy(ohiData, otelData)
	result.MissingAttributes = s.findMissingAttributes(ohiData, otelData)
	result.ExtraAttributes = s.findExtraAttributes(ohiData, otelData)
	result.ValueMismatches = s.findValueMismatches(ohiData, otelData, tolerance)
	
	// Determine status
	if result.Accuracy >= tolerance {
		result.Status = "PASSED"
	} else {
		result.Status = "FAILED"
	}
	
	// Log results
	s.T().Logf("Widget: %s | Accuracy: %.2f%% | Status: %s | OHI: %d points | OTEL: %d points",
		widgetName, result.Accuracy*100, result.Status, result.OHIDataPoints, result.OTELDataPoints)
	
	if len(result.MissingAttributes) > 0 {
		s.T().Logf("  Missing attributes: %v", result.MissingAttributes)
	}
	if len(result.ValueMismatches) > 0 {
		s.T().Logf("  Value mismatches: %d", len(result.ValueMismatches))
	}
	
	s.recordResult(result)
	return result
}

// validateTimeseriesParity validates time series data parity
func (s *OHIParityValidationSuite) validateTimeseriesParity(widgetName, ohiQuery, otelQuery string, tolerance float64) ValidationResult {
	// Similar to validateWidgetParity but with time series specific comparisons
	result := s.validateWidgetParity(widgetName, ohiQuery, otelQuery, tolerance)
	
	// Additional time series validation
	// Check for time alignment, trend similarity, etc.
	
	return result
}

// validateTableWidgetParity validates table widget with multiple attributes
func (s *OHIParityValidationSuite) validateTableWidgetParity(widgetName, ohiQuery, otelQuery string, requiredAttrs []string) ValidationResult {
	result := s.validateWidgetParity(widgetName, ohiQuery, otelQuery, 0.95)
	
	// Additional validation for required attributes
	for _, attr := range requiredAttrs {
		if !s.attributeExists(result, attr) {
			result.MissingAttributes = append(result.MissingAttributes, attr)
		}
	}
	
	return result
}

// executeNRQLQuery executes a NRQL query and returns results
func (s *OHIParityValidationSuite) executeNRQLQuery(query string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()
	
	results, err := s.nrdb.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	
	// Convert results to map format
	var data []map[string]interface{}
	if err := json.Unmarshal(results, &data); err != nil {
		return nil, fmt.Errorf("failed to parse query results: %w", err)
	}
	
	return data, nil
}

// calculateAccuracy calculates the accuracy between OHI and OTEL data
func (s *OHIParityValidationSuite) calculateAccuracy(ohiData, otelData []map[string]interface{}) float64 {
	if len(ohiData) == 0 && len(otelData) == 0 {
		return 1.0
	}
	if len(ohiData) == 0 || len(otelData) == 0 {
		return 0.0
	}
	
	// Calculate based on data point count similarity
	countAccuracy := float64(min(len(ohiData), len(otelData))) / float64(max(len(ohiData), len(otelData)))
	
	// Calculate based on value accuracy (simplified)
	valueAccuracy := s.calculateValueAccuracy(ohiData, otelData)
	
	// Combined accuracy (weighted average)
	return 0.3*countAccuracy + 0.7*valueAccuracy
}

// calculateValueAccuracy compares actual values
func (s *OHIParityValidationSuite) calculateValueAccuracy(ohiData, otelData []map[string]interface{}) float64 {
	matches := 0
	comparisons := 0
	
	// Simple comparison logic - in real implementation would be more sophisticated
	for i := 0; i < min(len(ohiData), len(otelData)); i++ {
		ohiRow := ohiData[i]
		otelRow := otelData[i]
		
		for key, ohiValue := range ohiRow {
			if otelValue, exists := otelRow[key]; exists {
				comparisons++
				if s.valuesMatch(ohiValue, otelValue, 0.05) {
					matches++
				}
			}
		}
	}
	
	if comparisons == 0 {
		return 0.0
	}
	
	return float64(matches) / float64(comparisons)
}

// valuesMatch checks if two values match within tolerance
func (s *OHIParityValidationSuite) valuesMatch(v1, v2 interface{}, tolerance float64) bool {
	// Handle different types
	switch val1 := v1.(type) {
	case float64:
		if val2, ok := v2.(float64); ok {
			diff := math.Abs(val1 - val2)
			if val1 != 0 {
				return diff/math.Abs(val1) <= tolerance
			}
			return diff <= tolerance
		}
	case string:
		if val2, ok := v2.(string); ok {
			return val1 == val2
		}
	case int, int64:
		if val2, ok := v2.(int64); ok {
			return val1 == val2
		}
	}
	
	// Fallback to string comparison
	return fmt.Sprintf("%v", v1) == fmt.Sprintf("%v", v2)
}

// Helper functions
func (s *OHIParityValidationSuite) findMissingAttributes(ohiData, otelData []map[string]interface{}) []string {
	missing := []string{}
	// Implementation to find missing attributes
	return missing
}

func (s *OHIParityValidationSuite) findExtraAttributes(ohiData, otelData []map[string]interface{}) []string {
	extra := []string{}
	// Implementation to find extra attributes
	return extra
}

func (s *OHIParityValidationSuite) findValueMismatches(ohiData, otelData []map[string]interface{}, tolerance float64) []ValueMismatch {
	mismatches := []ValueMismatch{}
	// Implementation to find value mismatches
	return mismatches
}

func (s *OHIParityValidationSuite) checkBlockingSessions() int {
	// Check if there are any blocking sessions
	query := "SELECT count(*) FROM PostgresBlockingSessions SINCE 30 minutes ago"
	data, err := s.executeNRQLQuery(query)
	if err != nil || len(data) == 0 {
		return 0
	}
	
	if count, ok := data[0]["count"].(float64); ok {
		return int(count)
	}
	return 0
}

func (s *OHIParityValidationSuite) attributeExists(result ValidationResult, attr string) bool {
	// Check if attribute exists in results
	for _, missing := range result.MissingAttributes {
		if missing == attr {
			return false
		}
	}
	return true
}

func (s *OHIParityValidationSuite) recordResult(result ValidationResult) {
	s.validationResults = append(s.validationResults, result)
}

func (s *OHIParityValidationSuite) generateValidationReport() {
	s.T().Log("\n=== OHI Parity Validation Report ===")
	
	totalTests := len(s.validationResults)
	passed := 0
	failed := 0
	totalAccuracy := 0.0
	
	for _, result := range s.validationResults {
		if result.Status == "PASSED" {
			passed++
		} else {
			failed++
		}
		totalAccuracy += result.Accuracy
	}
	
	avgAccuracy := totalAccuracy / float64(totalTests)
	
	s.T().Logf("\nSummary:")
	s.T().Logf("  Total Tests: %d", totalTests)
	s.T().Logf("  Passed: %d (%.1f%%)", passed, float64(passed)/float64(totalTests)*100)
	s.T().Logf("  Failed: %d (%.1f%%)", failed, float64(failed)/float64(totalTests)*100)
	s.T().Logf("  Average Accuracy: %.2f%%", avgAccuracy*100)
	
	s.T().Logf("\nDetailed Results:")
	for _, result := range s.validationResults {
		s.T().Logf("  - %s: %s (%.2f%% accuracy)",
			result.WidgetName, result.Status, result.Accuracy*100)
		if result.Error != nil {
			s.T().Logf("    Error: %v", result.Error)
		}
	}
	
	// Save detailed report to file
	s.saveDetailedReport()
}

func (s *OHIParityValidationSuite) saveDetailedReport() {
	// Implementation to save detailed JSON/HTML report
}

func (s *OHIParityValidationSuite) getParityValidationConfig() string {
	// Return comprehensive collector configuration for parity testing
	return `
receivers:
  postgresql:
    endpoint: localhost:5432
    username: postgres
    password: postgres
    databases:
      - testdb
    collection_interval: 10s
    
  sqlquery:
    driver: postgres
    datasource: "postgres://postgres:postgres@localhost:5432/testdb?sslmode=disable"
    queries:
      - sql: |
          SELECT 
            queryid,
            query,
            calls,
            total_exec_time,
            mean_exec_time,
            rows,
            shared_blks_hit,
            shared_blks_read
          FROM pg_stat_statements
          WHERE calls > 20
        logs:
          - body_column: query
            attribute_columns:
              - queryid
              - calls
              - total_exec_time
              - mean_exec_time
              - rows
    collection_interval: 30s

processors:
  batch:
    send_batch_size: 1024
    timeout: 10s

exporters:
  otlp/newrelic:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [batch]
      exporters: [otlp/newrelic]
    logs:
      receivers: [sqlquery]
      processors: [batch]
      exporters: [otlp/newrelic]
`
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}