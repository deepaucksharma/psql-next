// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package validators

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// MetricValidator validates metrics collected by the OTEL collector
type MetricValidator struct {
	logger              *zap.Logger
	config              *ValidationConfig
	collectedMetrics    map[string]*MetricData
	validationResults   []ValidationResult
	mutex               sync.RWMutex
	
	// Validation rules
	rules               []ValidationRule
	
	// Metric collection endpoints
	prometheusEndpoint  string
	debugEndpoint       string
	healthEndpoint      string
}

// ValidationConfig defines configuration for metric validation
type ValidationConfig struct {
	// Endpoints
	PrometheusEndpoint   string        `json:"prometheus_endpoint"`
	DebugEndpoint        string        `json:"debug_endpoint"`
	HealthEndpoint       string        `json:"health_endpoint"`
	
	// Validation settings
	ValidationTimeout    time.Duration `json:"validation_timeout"`
	RetryAttempts        int           `json:"retry_attempts"`
	RetryDelay           time.Duration `json:"retry_delay"`
	
	// Thresholds
	DefaultTolerances    map[string]float64 `json:"default_tolerances"`
	
	// Rule configuration
	EnableBuiltinRules   bool          `json:"enable_builtin_rules"`
	CustomRules          []ValidationRule `json:"custom_rules"`
}

// MetricData represents collected metric data
type MetricData struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Value       interface{}            `json:"value"`
	Labels      map[string]string      `json:"labels"`
	Timestamp   time.Time              `json:"timestamp"`
	Attributes  map[string]interface{} `json:"attributes"`
	Description string                 `json:"description"`
	Unit        string                 `json:"unit"`
}

// ValidationRule defines a validation rule for metrics
type ValidationRule struct {
	Name            string                 `json:"name"`
	MetricName      string                 `json:"metric_name"`
	MetricPattern   string                 `json:"metric_pattern"`
	ExpectedValue   interface{}            `json:"expected_value"`
	Tolerance       float64                `json:"tolerance"`
	Operator        string                 `json:"operator"` // gt, lt, eq, ne, between, exists, regex
	Attributes      map[string]interface{} `json:"attributes"`
	Labels          map[string]string      `json:"labels"`
	Description     string                 `json:"description"`
	Critical        bool                   `json:"critical"`
	Timeout         time.Duration          `json:"timeout"`
	
	// Advanced validation
	Dependencies    []string               `json:"dependencies"`
	PreConditions   []string               `json:"pre_conditions"`
	PostValidation  []string               `json:"post_validation"`
	
	// Time-based validation
	TimeWindow      time.Duration          `json:"time_window"`
	MinSamples      int                    `json:"min_samples"`
	AggregationType string                 `json:"aggregation_type"` // sum, avg, min, max, count
}

// ValidationResult contains the result of a validation rule execution
type ValidationResult struct {
	RuleName        string                 `json:"rule_name"`
	MetricName      string                 `json:"metric_name"`
	ExpectedValue   interface{}            `json:"expected_value"`
	ActualValue     interface{}            `json:"actual_value"`
	Passed          bool                   `json:"passed"`
	Tolerance       float64                `json:"tolerance"`
	Operator        string                 `json:"operator"`
	ErrorMessage    string                 `json:"error_message,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
	Duration        time.Duration          `json:"duration"`
	Critical        bool                   `json:"critical"`
	Attributes      map[string]interface{} `json:"attributes"`
	Metadata        map[string]interface{} `json:"metadata"`
}

// PrometheusMetric represents a Prometheus metric
type PrometheusMetric struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

// PrometheusResponse represents a Prometheus query response
type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string             `json:"resultType"`
		Result     []PrometheusMetric `json:"result"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

// NewMetricValidator creates a new metric validator
func NewMetricValidator(logger *zap.Logger, config *ValidationConfig) *MetricValidator {
	mv := &MetricValidator{
		logger:            logger,
		config:            config,
		collectedMetrics:  make(map[string]*MetricData),
		validationResults: make([]ValidationResult, 0),
		prometheusEndpoint: config.PrometheusEndpoint,
		debugEndpoint:     config.DebugEndpoint,
		healthEndpoint:    config.HealthEndpoint,
	}
	
	// Initialize with builtin rules if enabled
	if config.EnableBuiltinRules {
		mv.initializeBuiltinRules()
	}
	
	// Add custom rules
	mv.rules = append(mv.rules, config.CustomRules...)
	
	return mv
}

// ValidateMetrics validates all configured metrics
func (mv *MetricValidator) ValidateMetrics(ctx context.Context) ([]ValidationResult, error) {
	mv.logger.Info("Starting metric validation")
	
	// Clear previous results
	mv.mutex.Lock()
	mv.validationResults = make([]ValidationResult, 0)
	mv.mutex.Unlock()
	
	// Collect current metrics
	if err := mv.collectMetrics(ctx); err != nil {
		return nil, fmt.Errorf("failed to collect metrics: %w", err)
	}
	
	// Execute validation rules
	for _, rule := range mv.rules {
		result := mv.executeValidationRule(ctx, rule)
		
		mv.mutex.Lock()
		mv.validationResults = append(mv.validationResults, result)
		mv.mutex.Unlock()
		
		mv.logger.Info("Validation rule executed",
			zap.String("rule", rule.Name),
			zap.Bool("passed", result.Passed),
			zap.String("metric", rule.MetricName))
	}
	
	// Return copy of results
	mv.mutex.RLock()
	defer mv.mutex.RUnlock()
	results := make([]ValidationResult, len(mv.validationResults))
	copy(results, mv.validationResults)
	
	return results, nil
}

// collectMetrics collects metrics from various endpoints
func (mv *MetricValidator) collectMetrics(ctx context.Context) error {
	mv.logger.Info("Collecting metrics from endpoints")
	
	var wg sync.WaitGroup
	var errors []error
	var errorMutex sync.Mutex
	
	// Collect from Prometheus endpoint
	if mv.prometheusEndpoint != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := mv.collectPrometheusMetrics(ctx); err != nil {
				errorMutex.Lock()
				errors = append(errors, fmt.Errorf("prometheus collection failed: %w", err))
				errorMutex.Unlock()
			}
		}()
	}
	
	// Collect from debug endpoint
	if mv.debugEndpoint != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := mv.collectDebugMetrics(ctx); err != nil {
				errorMutex.Lock()
				errors = append(errors, fmt.Errorf("debug collection failed: %w", err))
				errorMutex.Unlock()
			}
		}()
	}
	
	// Check health endpoint
	if mv.healthEndpoint != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := mv.checkHealthEndpoint(ctx); err != nil {
				errorMutex.Lock()
				errors = append(errors, fmt.Errorf("health check failed: %w", err))
				errorMutex.Unlock()
			}
		}()
	}
	
	wg.Wait()
	
	if len(errors) > 0 {
		return fmt.Errorf("metric collection errors: %v", errors)
	}
	
	return nil
}

// collectPrometheusMetrics collects metrics from Prometheus endpoint
func (mv *MetricValidator) collectPrometheusMetrics(ctx context.Context) error {
	// Common Prometheus queries for database intelligence metrics
	queries := []string{
		"up",
		"otelcol_processor_accepted_metric_points_total",
		"otelcol_processor_dropped_metric_points_total",
		"otelcol_exporter_sent_metric_points_total",
		"otelcol_receiver_accepted_metric_points_total",
		"postgresql_db_count",
		"postgresql_query_count",
		"mysql_db_count",
		"mysql_query_count",
		"database_query_duration_seconds",
		"database_connection_count",
		"verification_processor_records_processed_total",
		"verification_processor_errors_total",
		"circuit_breaker_state",
	}
	
	for _, query := range queries {
		if err := mv.executePrometheusQuery(ctx, query); err != nil {
			mv.logger.Warn("Failed to execute Prometheus query",
				zap.String("query", query),
				zap.Error(err))
		}
	}
	
	return nil
}

// executePrometheusQuery executes a Prometheus query and stores results
func (mv *MetricValidator) executePrometheusQuery(ctx context.Context, query string) error {
	url := fmt.Sprintf("%s/api/v1/query?query=%s", mv.prometheusEndpoint, query)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{Timeout: mv.config.ValidationTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	
	var promResp PrometheusResponse
	if err := json.Unmarshal(body, &promResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	
	if promResp.Status != "success" {
		return fmt.Errorf("prometheus query failed: %s", promResp.Error)
	}
	
	// Store metrics
	mv.storePrometheusResults(query, promResp.Data.Result)
	
	return nil
}

// storePrometheusResults stores Prometheus query results
func (mv *MetricValidator) storePrometheusResults(query string, results []PrometheusMetric) {
	mv.mutex.Lock()
	defer mv.mutex.Unlock()
	
	for _, result := range results {
		// Extract value
		var value interface{}
		if len(result.Value) >= 2 {
			if valueStr, ok := result.Value[1].(string); ok {
				if parsedValue, err := strconv.ParseFloat(valueStr, 64); err == nil {
					value = parsedValue
				} else {
					value = valueStr
				}
			}
		}
		
		// Create metric data
		metricData := &MetricData{
			Name:      query,
			Type:      "prometheus",
			Value:     value,
			Labels:    result.Metric,
			Timestamp: time.Now(),
		}
		
		// Generate unique key
		key := mv.generateMetricKey(query, result.Metric)
		mv.collectedMetrics[key] = metricData
	}
}

// collectDebugMetrics collects metrics from debug endpoint
func (mv *MetricValidator) collectDebugMetrics(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", mv.debugEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{Timeout: mv.config.ValidationTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get debug metrics: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("debug endpoint returned status: %d", resp.StatusCode)
	}
	
	// Debug endpoint typically returns text/plain with metric descriptions
	// This is implementation-specific and would need to be adapted
	// based on the actual debug exporter format
	
	return nil
}

// checkHealthEndpoint checks the health endpoint
func (mv *MetricValidator) checkHealthEndpoint(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", mv.healthEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{Timeout: mv.config.ValidationTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check health: %w", err)
	}
	defer resp.Body.Close()
	
	// Store health status as a metric
	mv.mutex.Lock()
	mv.collectedMetrics["health_status"] = &MetricData{
		Name:      "health_status",
		Type:      "health",
		Value:     resp.StatusCode == http.StatusOK,
		Timestamp: time.Now(),
	}
	mv.mutex.Unlock()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

// executeValidationRule executes a single validation rule
func (mv *MetricValidator) executeValidationRule(ctx context.Context, rule ValidationRule) ValidationResult {
	startTime := time.Now()
	
	result := ValidationResult{
		RuleName:      rule.Name,
		MetricName:    rule.MetricName,
		ExpectedValue: rule.ExpectedValue,
		Tolerance:     rule.Tolerance,
		Operator:      rule.Operator,
		Timestamp:     startTime,
		Critical:      rule.Critical,
		Attributes:    make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}
	
	// Find matching metrics
	metrics := mv.findMatchingMetrics(rule)
	if len(metrics) == 0 {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("No metrics found matching pattern: %s", rule.MetricName)
		result.Duration = time.Since(startTime)
		return result
	}
	
	// Execute validation based on operator
	result.Passed, result.ActualValue, result.ErrorMessage = mv.validateMetrics(rule, metrics)
	result.Duration = time.Since(startTime)
	
	// Add metadata
	result.Metadata["metric_count"] = len(metrics)
	result.Metadata["rule_timeout"] = rule.Timeout
	result.Metadata["validation_duration"] = result.Duration
	
	return result
}

// findMatchingMetrics finds metrics that match the validation rule
func (mv *MetricValidator) findMatchingMetrics(rule ValidationRule) []*MetricData {
	mv.mutex.RLock()
	defer mv.mutex.RUnlock()
	
	var matchingMetrics []*MetricData
	
	for key, metric := range mv.collectedMetrics {
		// Check metric name/pattern match
		nameMatch := false
		if rule.MetricName != "" {
			nameMatch = metric.Name == rule.MetricName
		}
		if rule.MetricPattern != "" {
			nameMatch = nameMatch || strings.Contains(metric.Name, rule.MetricPattern)
		}
		
		if !nameMatch {
			continue
		}
		
		// Check attribute filters
		attributeMatch := true
		for attrKey, attrValue := range rule.Attributes {
			if metric.Attributes != nil {
				if value, exists := metric.Attributes[attrKey]; !exists || value != attrValue {
					attributeMatch = false
					break
				}
			} else {
				attributeMatch = false
				break
			}
		}
		
		// Check label filters
		labelMatch := true
		for labelKey, labelValue := range rule.Labels {
			if metric.Labels != nil {
				if value, exists := metric.Labels[labelKey]; !exists || value != labelValue {
					labelMatch = false
					break
				}
			} else {
				labelMatch = false
				break
			}
		}
		
		if attributeMatch && labelMatch {
			matchingMetrics = append(matchingMetrics, metric)
		}
	}
	
	return matchingMetrics
}

// validateMetrics validates metrics against a rule
func (mv *MetricValidator) validateMetrics(rule ValidationRule, metrics []*MetricData) (bool, interface{}, string) {
	if len(metrics) == 0 {
		return false, nil, "No metrics to validate"
	}
	
	// For time-window validation, filter metrics by time
	if rule.TimeWindow > 0 {
		cutoff := time.Now().Add(-rule.TimeWindow)
		var filteredMetrics []*MetricData
		for _, metric := range metrics {
			if metric.Timestamp.After(cutoff) {
				filteredMetrics = append(filteredMetrics, metric)
			}
		}
		metrics = filteredMetrics
	}
	
	// Check minimum samples requirement
	if rule.MinSamples > 0 && len(metrics) < rule.MinSamples {
		return false, len(metrics), 
			fmt.Sprintf("Insufficient samples: got %d, required %d", len(metrics), rule.MinSamples)
	}
	
	// Aggregate values if needed
	var actualValue interface{}
	if rule.AggregationType != "" {
		actualValue = mv.aggregateMetricValues(metrics, rule.AggregationType)
	} else if len(metrics) == 1 {
		actualValue = metrics[0].Value
	} else {
		// Multiple metrics without aggregation - use the latest
		latest := metrics[0]
		for _, metric := range metrics[1:] {
			if metric.Timestamp.After(latest.Timestamp) {
				latest = metric
			}
		}
		actualValue = latest.Value
	}
	
	// Perform validation based on operator
	return mv.validateValue(actualValue, rule.ExpectedValue, rule.Operator, rule.Tolerance)
}

// aggregateMetricValues aggregates metric values
func (mv *MetricValidator) aggregateMetricValues(metrics []*MetricData, aggregationType string) interface{} {
	if len(metrics) == 0 {
		return nil
	}
	
	// Convert values to float64 for numerical operations
	var values []float64
	for _, metric := range metrics {
		if value, ok := mv.convertToFloat(metric.Value); ok {
			values = append(values, value)
		}
	}
	
	if len(values) == 0 {
		return nil
	}
	
	switch aggregationType {
	case "sum":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum
		
	case "avg":
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		return sum / float64(len(values))
		
	case "min":
		min := values[0]
		for _, v := range values[1:] {
			if v < min {
				min = v
			}
		}
		return min
		
	case "max":
		max := values[0]
		for _, v := range values[1:] {
			if v > max {
				max = v
			}
		}
		return max
		
	case "count":
		return float64(len(values))
		
	default:
		// Default to latest value
		return values[len(values)-1]
	}
}

// validateValue validates a value against expected value using operator
func (mv *MetricValidator) validateValue(actual, expected interface{}, operator string, tolerance float64) (bool, interface{}, string) {
	switch operator {
	case "exists":
		return actual != nil, actual, ""
		
	case "eq":
		return mv.compareValues(actual, expected, tolerance, "eq")
		
	case "ne":
		passed, _, _ := mv.compareValues(actual, expected, tolerance, "eq")
		return !passed, actual, ""
		
	case "gt":
		return mv.compareValues(actual, expected, tolerance, "gt")
		
	case "lt":
		return mv.compareValues(actual, expected, tolerance, "lt")
		
	case "between":
		// Expected should be an array [min, max]
		if expectedArray, ok := expected.([]interface{}); ok && len(expectedArray) == 2 {
			min, max := expectedArray[0], expectedArray[1]
			minPassed, _, _ := mv.compareValues(actual, min, tolerance, "gt")
			maxPassed, _, _ := mv.compareValues(actual, max, tolerance, "lt")
			return minPassed && maxPassed, actual, ""
		}
		return false, actual, "Invalid 'between' expected value format"
		
	default:
		return false, actual, fmt.Sprintf("Unknown operator: %s", operator)
	}
}

// compareValues compares two values with tolerance
func (mv *MetricValidator) compareValues(actual, expected interface{}, tolerance float64, operator string) (bool, interface{}, string) {
	actualFloat, actualOk := mv.convertToFloat(actual)
	expectedFloat, expectedOk := mv.convertToFloat(expected)
	
	if actualOk && expectedOk {
		diff := math.Abs(actualFloat - expectedFloat)
		toleranceValue := expectedFloat * tolerance
		
		switch operator {
		case "eq":
			return diff <= toleranceValue, actual, ""
		case "gt":
			return actualFloat > expectedFloat-toleranceValue, actual, ""
		case "lt":
			return actualFloat < expectedFloat+toleranceValue, actual, ""
		}
	}
	
	// String comparison
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	
	switch operator {
	case "eq":
		return actualStr == expectedStr, actual, ""
	default:
		return false, actual, "Cannot compare non-numeric values with operator: " + operator
	}
}

// convertToFloat converts various types to float64
func (mv *MetricValidator) convertToFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

// generateMetricKey generates a unique key for a metric
func (mv *MetricValidator) generateMetricKey(name string, labels map[string]string) string {
	key := name
	
	// Sort labels for consistent key generation
	var labelPairs []string
	for k, v := range labels {
		labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
	}
	
	if len(labelPairs) > 0 {
		key += "{" + strings.Join(labelPairs, ",") + "}"
	}
	
	return key
}

// initializeBuiltinRules initializes builtin validation rules
func (mv *MetricValidator) initializeBuiltinRules() {
	builtinRules := []ValidationRule{
		{
			Name:          "health_check",
			MetricName:    "health_status",
			ExpectedValue: true,
			Operator:      "eq",
			Description:   "Verify that health endpoint is accessible",
			Critical:      true,
			Timeout:       30 * time.Second,
		},
		{
			Name:          "postgresql_metrics_present",
			MetricPattern: "postgresql",
			Operator:      "exists",
			Description:   "Verify that PostgreSQL metrics are being collected",
			Critical:      true,
			TimeWindow:    5 * time.Minute,
			MinSamples:    1,
		},
		{
			Name:          "mysql_metrics_present",
			MetricPattern: "mysql",
			Operator:      "exists",
			Description:   "Verify that MySQL metrics are being collected",
			Critical:      true,
			TimeWindow:    5 * time.Minute,
			MinSamples:    1,
		},
		{
			Name:          "otel_receiver_metrics",
			MetricName:    "otelcol_receiver_accepted_metric_points_total",
			ExpectedValue: 0,
			Operator:      "gt",
			Description:   "Verify that OTEL receiver is accepting metrics",
			Critical:      true,
			TimeWindow:    5 * time.Minute,
			AggregationType: "sum",
		},
		{
			Name:          "otel_processor_metrics",
			MetricName:    "otelcol_processor_accepted_metric_points_total",
			ExpectedValue: 0,
			Operator:      "gt",
			Description:   "Verify that OTEL processor is processing metrics",
			Critical:      true,
			TimeWindow:    5 * time.Minute,
			AggregationType: "sum",
		},
		{
			Name:          "low_error_rate",
			MetricName:    "otelcol_processor_dropped_metric_points_total",
			ExpectedValue: 100,
			Operator:      "lt",
			Tolerance:     0.1,
			Description:   "Verify low metric drop rate",
			Critical:      false,
			TimeWindow:    5 * time.Minute,
			AggregationType: "sum",
		},
		{
			Name:          "verification_processor_active",
			MetricName:    "verification_processor_records_processed_total",
			ExpectedValue: 0,
			Operator:      "gt",
			Description:   "Verify that verification processor is active",
			Critical:      true,
			TimeWindow:    5 * time.Minute,
			AggregationType: "sum",
		},
		{
			Name:          "circuit_breaker_closed",
			MetricName:    "circuit_breaker_state",
			ExpectedValue: 0, // 0 = closed, 1 = open
			Operator:      "eq",
			Description:   "Verify that circuit breakers are in closed state",
			Critical:      false,
			TimeWindow:    1 * time.Minute,
		},
	}
	
	mv.rules = append(mv.rules, builtinRules...)
}

// GetValidationSummary returns a summary of validation results
func (mv *MetricValidator) GetValidationSummary() map[string]interface{} {
	mv.mutex.RLock()
	defer mv.mutex.RUnlock()
	
	totalRules := len(mv.validationResults)
	passedRules := 0
	failedRules := 0
	criticalFailures := 0
	
	for _, result := range mv.validationResults {
		if result.Passed {
			passedRules++
		} else {
			failedRules++
			if result.Critical {
				criticalFailures++
			}
		}
	}
	
	return map[string]interface{}{
		"total_rules":        totalRules,
		"passed_rules":       passedRules,
		"failed_rules":       failedRules,
		"critical_failures":  criticalFailures,
		"success_rate":       float64(passedRules) / float64(totalRules),
		"collected_metrics":  len(mv.collectedMetrics),
	}
}