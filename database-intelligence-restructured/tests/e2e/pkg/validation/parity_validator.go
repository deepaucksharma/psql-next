package validation

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// ParityValidator validates metric parity between OHI and OpenTelemetry
type ParityValidator struct {
	ohiClient       DataClient
	otelClient      DataClient
	mappingRegistry *MetricMappingRegistry
	tolerance       float64
	config          *ParityConfig
}

// DataClient interface for querying data sources
type DataClient interface {
	Query(ctx context.Context, query string) ([]map[string]interface{}, error)
	GetMetricValue(ctx context.Context, metric string, filters map[string]string) (float64, error)
}

// ParityConfig contains validation configuration
type ParityConfig struct {
	DefaultTolerance   float64                    `yaml:"default_tolerance"`
	MetricTolerances   map[string]float64         `yaml:"metric_tolerances"`
	TimeWindow         string                     `yaml:"time_window"`
	AggregationMethod  string                     `yaml:"aggregation_method"`
	IgnoreAttributes   []string                   `yaml:"ignore_attributes"`
	AttributeMappings  map[string]string          `yaml:"attribute_mappings"`
	ValidationProfiles map[string]ValidationProfile `yaml:"validation_profiles"`
}

// ValidationProfile defines a validation profile
type ValidationProfile struct {
	Name         string             `yaml:"name"`
	Description  string             `yaml:"description"`
	Tolerance    float64            `yaml:"tolerance"`
	Metrics      []string           `yaml:"metrics"`
	Attributes   []string           `yaml:"attributes"`
	CustomRules  []CustomRule       `yaml:"custom_rules"`
}

// CustomRule defines custom validation rules
type CustomRule struct {
	Name      string                 `yaml:"name"`
	Type      string                 `yaml:"type"`
	Config    map[string]interface{} `yaml:"config"`
}

// MetricMappingRegistry manages metric mappings
type MetricMappingRegistry struct {
	mappings        map[string]*MetricMapping
	transformations map[string]TransformationFunc
	eventMappings   map[string]*EventMapping
}

// MetricMapping defines how an OHI metric maps to OTEL
type MetricMapping struct {
	OHIName         string
	OTELName        string
	Type            MetricType
	Transformation  string
	Formula         string
	Unit            string
	Attributes      map[string]*AttributeMapping
}

// AttributeMapping defines attribute mappings
type AttributeMapping struct {
	OHIName         string
	OTELName        string
	Transformation  string
	DefaultValue    interface{}
	Required        bool
}

// EventMapping defines event type mappings
type EventMapping struct {
	OHIEvent        string
	OTELMetricType  string
	OTELFilter      string
	RequiredFields  []string
	OptionalFields  []string
}

// MetricType represents the type of metric
type MetricType string

const (
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeCounter   MetricType = "counter"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
)

// TransformationFunc defines a metric transformation function
type TransformationFunc func(value interface{}, params map[string]interface{}) (interface{}, error)

// ValidationResult contains detailed validation results
type ValidationResult struct {
	Timestamp       time.Time
	MetricName      string
	OHIValue        interface{}
	OTELValue       interface{}
	Accuracy        float64
	Status          ValidationStatus
	Issues          []ValidationIssue
	Metadata        map[string]interface{}
}

// ValidationStatus represents the validation status
type ValidationStatus string

const (
	ValidationStatusPassed  ValidationStatus = "PASSED"
	ValidationStatusFailed  ValidationStatus = "FAILED"
	ValidationStatusWarning ValidationStatus = "WARNING"
	ValidationStatusSkipped ValidationStatus = "SKIPPED"
)

// ValidationIssue represents a validation issue
type ValidationIssue struct {
	Type        IssueType
	Severity    IssueSeverity
	Message     string
	Details     map[string]interface{}
	Suggestion  string
}

// IssueType represents the type of validation issue
type IssueType string

const (
	IssueTypeMissingData     IssueType = "MISSING_DATA"
	IssueTypeValueMismatch   IssueType = "VALUE_MISMATCH"
	IssueTypeTypeMismatch    IssueType = "TYPE_MISMATCH"
	IssueTypeCardinalityHigh IssueType = "CARDINALITY_HIGH"
	IssueTypeTimingSkew      IssueType = "TIMING_SKEW"
)

// IssueSeverity represents the severity of an issue
type IssueSeverity string

const (
	IssueSeverityCritical IssueSeverity = "CRITICAL"
	IssueSeverityHigh     IssueSeverity = "HIGH"
	IssueSeverityMedium   IssueSeverity = "MEDIUM"
	IssueSeverityLow      IssueSeverity = "LOW"
)

// NewParityValidator creates a new parity validator
func NewParityValidator(ohiClient, otelClient DataClient, mappingsFile string) (*ParityValidator, error) {
	registry, err := LoadMappingRegistry(mappingsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load mapping registry: %w", err)
	}

	config := &ParityConfig{
		DefaultTolerance:  0.05, // 5% default tolerance
		TimeWindow:       "30 minutes",
		AggregationMethod: "average",
	}

	return &ParityValidator{
		ohiClient:       ohiClient,
		otelClient:      otelClient,
		mappingRegistry: registry,
		tolerance:       config.DefaultTolerance,
		config:          config,
	}, nil
}

// LoadMappingRegistry loads metric mappings from file
func LoadMappingRegistry(filename string) (*MetricMappingRegistry, error) {
	// Load and parse mapping file
	registry := &MetricMappingRegistry{
		mappings:        make(map[string]*MetricMapping),
		transformations: make(map[string]TransformationFunc),
		eventMappings:   make(map[string]*EventMapping),
	}

	// Register default transformations
	registry.RegisterDefaultTransformations()

	return registry, nil
}

// RegisterDefaultTransformations registers default transformation functions
func (r *MetricMappingRegistry) RegisterDefaultTransformations() {
	// Direct transformation (no change)
	r.transformations["direct"] = func(value interface{}, params map[string]interface{}) (interface{}, error) {
		return value, nil
	}

	// Rate per second transformation
	r.transformations["rate_per_second"] = func(value interface{}, params map[string]interface{}) (interface{}, error) {
		if v, ok := value.(float64); ok {
			interval := params["interval"].(float64)
			return v / interval, nil
		}
		return nil, fmt.Errorf("invalid value type for rate_per_second")
	}

	// Anonymize transformation
	r.transformations["anonymize"] = func(value interface{}, params map[string]interface{}) (interface{}, error) {
		if str, ok := value.(string); ok {
			// Simple anonymization - replace literals
			anonymized := anonymizeQuery(str)
			return anonymized, nil
		}
		return value, nil
	}

	// Uppercase transformation
	r.transformations["uppercase"] = func(value interface{}, params map[string]interface{}) (interface{}, error) {
		if str, ok := value.(string); ok {
			return strings.ToUpper(str), nil
		}
		return value, nil
	}
}

// ValidateWidget validates a specific dashboard widget
func (v *ParityValidator) ValidateWidget(ctx context.Context, widget DashboardWidget) (*ValidationResult, error) {
	// Transform OHI query to OTEL query
	otelQuery, err := v.transformQuery(widget.NRQLQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to transform query: %w", err)
	}

	// Execute both queries
	ohiData, err := v.ohiClient.Query(ctx, widget.NRQLQuery)
	if err != nil {
		return nil, fmt.Errorf("OHI query failed: %w", err)
	}

	otelData, err := v.otelClient.Query(ctx, otelQuery)
	if err != nil {
		return nil, fmt.Errorf("OTEL query failed: %w", err)
	}

	// Compare results
	result := v.compareData(widget.Title, ohiData, otelData)
	return result, nil
}

// ValidateMetric validates a specific metric
func (v *ParityValidator) ValidateMetric(ctx context.Context, metricName string, filters map[string]string) (*ValidationResult, error) {
	// Get metric mapping
	mapping, exists := v.mappingRegistry.mappings[metricName]
	if !exists {
		return nil, fmt.Errorf("no mapping found for metric: %s", metricName)
	}

	// Get OHI value
	ohiValue, err := v.ohiClient.GetMetricValue(ctx, metricName, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get OHI metric: %w", err)
	}

	// Transform filters for OTEL
	otelFilters := v.transformFilters(filters, mapping.Attributes)

	// Get OTEL value
	otelValue, err := v.otelClient.GetMetricValue(ctx, mapping.OTELName, otelFilters)
	if err != nil {
		return nil, fmt.Errorf("failed to get OTEL metric: %w", err)
	}

	// Apply transformation if needed
	if mapping.Transformation != "" && mapping.Transformation != "direct" {
		transformer, exists := v.mappingRegistry.transformations[mapping.Transformation]
		if exists {
			transformed, err := transformer(otelValue, nil)
			if err == nil {
				otelValue = transformed.(float64)
			}
		}
	}

	// Calculate accuracy
	accuracy := v.calculateAccuracy(ohiValue, otelValue)
	
	result := &ValidationResult{
		Timestamp:  time.Now(),
		MetricName: metricName,
		OHIValue:   ohiValue,
		OTELValue:  otelValue,
		Accuracy:   accuracy,
		Status:     v.determineStatus(accuracy),
		Issues:     []ValidationIssue{},
		Metadata: map[string]interface{}{
			"mapping":         mapping.OTELName,
			"transformation":  mapping.Transformation,
			"tolerance":       v.getTolerance(metricName),
		},
	}

	// Add issues if accuracy is low
	if accuracy < v.getTolerance(metricName) {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:     IssueTypeValueMismatch,
			Severity: IssueSeverityHigh,
			Message:  fmt.Sprintf("Value mismatch: OHI=%v, OTEL=%v, Accuracy=%.2f%%", ohiValue, otelValue, accuracy*100),
			Details: map[string]interface{}{
				"difference": math.Abs(ohiValue - otelValue),
				"percentage": (math.Abs(ohiValue - otelValue) / ohiValue) * 100,
			},
		})
	}

	return result, nil
}

// ValidateAllWidgets validates all widgets in a dashboard
func (v *ParityValidator) ValidateAllWidgets(ctx context.Context, widgets []DashboardWidget) ([]*ValidationResult, error) {
	results := make([]*ValidationResult, 0, len(widgets))

	for _, widget := range widgets {
		result, err := v.ValidateWidget(ctx, widget)
		if err != nil {
			// Create error result
			result = &ValidationResult{
				Timestamp:  time.Now(),
				MetricName: widget.Title,
				Status:     ValidationStatusFailed,
				Issues: []ValidationIssue{
					{
						Type:     IssueTypeMissingData,
						Severity: IssueSeverityCritical,
						Message:  err.Error(),
					},
				},
			}
		}
		results = append(results, result)
	}

	return results, nil
}

// transformQuery transforms an OHI NRQL query to OTEL format
func (v *ParityValidator) transformQuery(ohiQuery string) (string, error) {
	// Parse the query
	query := strings.ToUpper(ohiQuery)
	
	// Extract event type
	var eventType string
	if idx := strings.Index(query, "FROM"); idx >= 0 {
		parts := strings.Fields(query[idx:])
		if len(parts) >= 2 {
			eventType = parts[1]
		}
	}

	// Get event mapping
	eventMapping, exists := v.mappingRegistry.eventMappings[eventType]
	if !exists {
		return "", fmt.Errorf("no mapping for event type: %s", eventType)
	}

	// Transform to OTEL query
	otelQuery := strings.Replace(ohiQuery, eventType, eventMapping.OTELMetricType, 1)
	
	// Add OTEL filter
	if eventMapping.OTELFilter != "" {
		if strings.Contains(strings.ToUpper(otelQuery), "WHERE") {
			otelQuery = strings.Replace(otelQuery, "WHERE", fmt.Sprintf("WHERE %s AND", eventMapping.OTELFilter), 1)
		} else if strings.Contains(strings.ToUpper(otelQuery), "FACET") {
			otelQuery = strings.Replace(otelQuery, "FACET", fmt.Sprintf("WHERE %s FACET", eventMapping.OTELFilter), 1)
		} else {
			otelQuery += fmt.Sprintf(" WHERE %s", eventMapping.OTELFilter)
		}
	}

	// Transform attribute names
	for ohiAttr, mapping := range v.mappingRegistry.getAttributeMappings(eventType) {
		otelQuery = strings.ReplaceAll(otelQuery, ohiAttr, mapping.OTELName)
	}

	// Add time window if not present
	if !strings.Contains(strings.ToUpper(otelQuery), "SINCE") {
		otelQuery += fmt.Sprintf(" SINCE %s ago", v.config.TimeWindow)
	}

	return otelQuery, nil
}

// compareData compares OHI and OTEL data sets
func (v *ParityValidator) compareData(metricName string, ohiData, otelData []map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		Timestamp:  time.Now(),
		MetricName: metricName,
		Issues:     []ValidationIssue{},
		Metadata: map[string]interface{}{
			"ohi_count":  len(ohiData),
			"otel_count": len(otelData),
		},
	}

	// Check data availability
	if len(ohiData) == 0 && len(otelData) == 0 {
		result.Status = ValidationStatusSkipped
		result.Accuracy = 1.0
		return result
	}

	if len(ohiData) == 0 || len(otelData) == 0 {
		result.Status = ValidationStatusFailed
		result.Accuracy = 0.0
		result.Issues = append(result.Issues, ValidationIssue{
			Type:     IssueTypeMissingData,
			Severity: IssueSeverityCritical,
			Message:  fmt.Sprintf("Missing data: OHI=%d rows, OTEL=%d rows", len(ohiData), len(otelData)),
		})
		return result
	}

	// Calculate overall accuracy
	countAccuracy := float64(min(len(ohiData), len(otelData))) / float64(max(len(ohiData), len(otelData)))
	valueAccuracy := v.calculateDataSetAccuracy(ohiData, otelData)
	
	result.Accuracy = 0.3*countAccuracy + 0.7*valueAccuracy
	result.Status = v.determineStatus(result.Accuracy)

	// Add specific issues
	if countAccuracy < 0.9 {
		result.Issues = append(result.Issues, ValidationIssue{
			Type:     IssueTypeCardinalityHigh,
			Severity: IssueSeverityMedium,
			Message:  fmt.Sprintf("Row count mismatch: OHI=%d, OTEL=%d", len(ohiData), len(otelData)),
		})
	}

	return result
}

// calculateDataSetAccuracy calculates accuracy between two data sets
func (v *ParityValidator) calculateDataSetAccuracy(ohiData, otelData []map[string]interface{}) float64 {
	totalComparisons := 0
	matches := 0

	// Create index for faster lookup
	otelIndex := make(map[string]map[string]interface{})
	for _, row := range otelData {
		key := v.generateRowKey(row)
		otelIndex[key] = row
	}

	// Compare each OHI row with OTEL
	for _, ohiRow := range ohiData {
		key := v.generateRowKey(ohiRow)
		if otelRow, exists := otelIndex[key]; exists {
			rowMatches, rowComparisons := v.compareRows(ohiRow, otelRow)
			matches += rowMatches
			totalComparisons += rowComparisons
		}
	}

	if totalComparisons == 0 {
		return 0.0
	}

	return float64(matches) / float64(totalComparisons)
}

// generateRowKey generates a unique key for a data row
func (v *ParityValidator) generateRowKey(row map[string]interface{}) string {
	// Use facet values or primary identifiers as key
	var keyParts []string
	
	// Common identifier fields
	identifiers := []string{"query_id", "database_name", "wait_event_name", "blocked_pid"}
	
	for _, id := range identifiers {
		if val, exists := row[id]; exists {
			keyParts = append(keyParts, fmt.Sprintf("%v", val))
		}
	}

	return strings.Join(keyParts, "|")
}

// compareRows compares two data rows
func (v *ParityValidator) compareRows(ohiRow, otelRow map[string]interface{}) (matches, comparisons int) {
	for key, ohiValue := range ohiRow {
		// Skip ignored attributes
		if v.shouldIgnoreAttribute(key) {
			continue
		}

		// Map attribute name if needed
		otelKey := v.mapAttributeName(key)
		
		if otelValue, exists := otelRow[otelKey]; exists {
			comparisons++
			if v.valuesMatch(ohiValue, otelValue) {
				matches++
			}
		}
	}

	return matches, comparisons
}

// valuesMatch checks if two values match within tolerance
func (v *ParityValidator) valuesMatch(v1, v2 interface{}) bool {
	switch val1 := v1.(type) {
	case float64:
		if val2, ok := v2.(float64); ok {
			if val1 == 0 && val2 == 0 {
				return true
			}
			diff := math.Abs(val1 - val2)
			avg := (math.Abs(val1) + math.Abs(val2)) / 2
			return diff/avg <= v.tolerance
		}
	case string:
		if val2, ok := v2.(string); ok {
			return val1 == val2 || v.stringsMatchWithTransform(val1, val2)
		}
	case int, int64:
		val1Float := float64(val1.(int64))
		if val2Float, ok := toFloat64(v2); ok {
			return v.valuesMatch(val1Float, val2Float)
		}
	}

	// Fallback to string comparison
	return fmt.Sprintf("%v", v1) == fmt.Sprintf("%v", v2)
}

// stringsMatchWithTransform checks if strings match after transformation
func (v *ParityValidator) stringsMatchWithTransform(s1, s2 string) bool {
	// Handle special cases
	specialMappings := map[string]string{
		"<nil>":                    "",
		"<insufficient privilege>": "[REDACTED]",
	}

	if mapped, exists := specialMappings[s1]; exists {
		s1 = mapped
	}
	if mapped, exists := specialMappings[s2]; exists {
		s2 = mapped
	}

	return s1 == s2
}

// Helper methods

func (v *ParityValidator) calculateAccuracy(v1, v2 float64) float64 {
	if v1 == 0 && v2 == 0 {
		return 1.0
	}
	if v1 == 0 || v2 == 0 {
		return 0.0
	}
	
	diff := math.Abs(v1 - v2)
	avg := (math.Abs(v1) + math.Abs(v2)) / 2
	accuracy := 1.0 - (diff / avg)
	
	if accuracy < 0 {
		accuracy = 0
	}
	
	return accuracy
}

func (v *ParityValidator) determineStatus(accuracy float64) ValidationStatus {
	tolerance := v.tolerance
	
	if accuracy >= (1.0 - tolerance) {
		return ValidationStatusPassed
	} else if accuracy >= (1.0 - tolerance*2) {
		return ValidationStatusWarning
	}
	return ValidationStatusFailed
}

func (v *ParityValidator) getTolerance(metricName string) float64 {
	if tolerance, exists := v.config.MetricTolerances[metricName]; exists {
		return tolerance
	}
	return v.config.DefaultTolerance
}

func (v *ParityValidator) transformFilters(filters map[string]string, mappings map[string]*AttributeMapping) map[string]string {
	transformed := make(map[string]string)
	
	for key, value := range filters {
		if mapping, exists := mappings[key]; exists {
			transformed[mapping.OTELName] = value
		} else {
			transformed[key] = value
		}
	}
	
	return transformed
}

func (v *ParityValidator) shouldIgnoreAttribute(attr string) bool {
	for _, ignored := range v.config.IgnoreAttributes {
		if attr == ignored {
			return true
		}
	}
	return false
}

func (v *ParityValidator) mapAttributeName(ohiAttr string) string {
	if mapped, exists := v.config.AttributeMappings[ohiAttr]; exists {
		return mapped
	}
	return ohiAttr
}

func (r *MetricMappingRegistry) getAttributeMappings(eventType string) map[string]*AttributeMapping {
	// Return attribute mappings for the event type
	mappings := make(map[string]*AttributeMapping)
	
	if _, exists := r.eventMappings[eventType]; exists {
		// Populate from event mapping configuration
	}
	
	return mappings
}

// Helper functions

func anonymizeQuery(query string) string {
	// Simple query anonymization
	// Replace literals with placeholders
	// Remove email addresses, numbers, etc.
	anonymized := query
	
	// Replace numbers
	anonymized = strings.ReplaceAll(anonymized, "[0-9]+", "?")
	
	// Replace strings in quotes
	anonymized = strings.ReplaceAll(anonymized, "'[^']*'", "?")
	
	return anonymized
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		// Try to parse string as float
		var f float64
		_, err := fmt.Sscanf(val, "%f", &f)
		return f, err == nil
	}
	return 0, false
}

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