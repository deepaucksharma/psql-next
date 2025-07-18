package framework

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// MetricValidator validates collected metrics
type MetricValidator struct {
	logger         *zap.Logger
	prometheusURL  string
	tolerance      float64
}

// NewMetricValidator creates a new metric validator
func NewMetricValidator(logger *zap.Logger, prometheusURL string) *MetricValidator {
	return &MetricValidator{
		logger:        logger,
		prometheusURL: prometheusURL,
		tolerance:     0.05, // 5% tolerance by default
	}
}

// ValidationResult represents the result of a validation
type ValidationResult struct {
	MetricName string
	Expected   float64
	Actual     float64
	Passed     bool
	Error      string
}

// ValidateMetricExists checks if a metric exists in Prometheus
func (v *MetricValidator) ValidateMetricExists(metricName string) (bool, error) {
	metrics, err := v.fetchPrometheusMetrics()
	if err != nil {
		return false, fmt.Errorf("failed to fetch metrics: %w", err)
	}
	
	for _, line := range strings.Split(metrics, "\n") {
		if strings.HasPrefix(line, metricName) && !strings.HasPrefix(line, "#") {
			return true, nil
		}
	}
	
	return false, nil
}

// ValidateMetricValue checks if a metric has the expected value within tolerance
func (v *MetricValidator) ValidateMetricValue(metricName string, expectedValue float64) ValidationResult {
	result := ValidationResult{
		MetricName: metricName,
		Expected:   expectedValue,
	}
	
	value, err := v.getMetricValue(metricName)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	
	result.Actual = value
	
	// Calculate accuracy
	diff := abs(expectedValue - value)
	avg := (expectedValue + value) / 2
	
	if avg == 0 {
		result.Passed = diff == 0
	} else {
		accuracy := 1.0 - (diff / avg)
		result.Passed = accuracy >= (1.0 - v.tolerance)
	}
	
	return result
}

// ValidateMetrics validates multiple metrics
func (v *MetricValidator) ValidateMetrics(expectations map[string]float64) []ValidationResult {
	var results []ValidationResult
	
	for metric, expected := range expectations {
		result := v.ValidateMetricValue(metric, expected)
		results = append(results, result)
		
		v.logger.Info("Metric validation",
			zap.String("metric", metric),
			zap.Float64("expected", expected),
			zap.Float64("actual", result.Actual),
			zap.Bool("passed", result.Passed))
	}
	
	return results
}

// ValidateMySQLMetrics validates standard MySQL metrics are present
func (v *MetricValidator) ValidateMySQLMetrics() error {
	requiredMetrics := []string{
		"mysql_connections_active",
		"mysql_connections_total",
		"mysql_queries_total",
		"mysql_slow_queries_total",
		"mysql_buffer_pool_usage",
		"mysql_innodb_row_operations_total",
		"mysql_table_io_wait_count_total",
		"mysql_table_io_wait_time_total",
	}
	
	missingMetrics := []string{}
	
	for _, metric := range requiredMetrics {
		exists, err := v.ValidateMetricExists(metric)
		if err != nil {
			return fmt.Errorf("failed to check metric %s: %w", metric, err)
		}
		
		if !exists {
			missingMetrics = append(missingMetrics, metric)
		}
	}
	
	if len(missingMetrics) > 0 {
		return fmt.Errorf("missing required metrics: %v", missingMetrics)
	}
	
	return nil
}

// ValidateFileOutput validates metrics written to file
func (v *MetricValidator) ValidateFileOutput(filepath string, expectedMetrics []string) error {
	// Read file
	file, err := io.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	
	// Parse JSON lines
	lines := strings.Split(string(file), "\n")
	foundMetrics := make(map[string]bool)
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		var metric map[string]interface{}
		if err := json.Unmarshal([]byte(line), &metric); err != nil {
			continue
		}
		
		// Check for metric name in various formats
		if name, ok := metric["name"].(string); ok {
			foundMetrics[name] = true
		}
		if name, ok := metric["metric_name"].(string); ok {
			foundMetrics[name] = true
		}
	}
	
	// Check all expected metrics are present
	missingMetrics := []string{}
	for _, expected := range expectedMetrics {
		if !foundMetrics[expected] {
			missingMetrics = append(missingMetrics, expected)
		}
	}
	
	if len(missingMetrics) > 0 {
		return fmt.Errorf("missing metrics in file output: %v", missingMetrics)
	}
	
	return nil
}

// fetchPrometheusMetrics fetches all metrics from Prometheus endpoint
func (v *MetricValidator) fetchPrometheusMetrics() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get(v.prometheusURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	return string(body), nil
}

// getMetricValue extracts the value of a specific metric
func (v *MetricValidator) getMetricValue(metricName string) (float64, error) {
	metrics, err := v.fetchPrometheusMetrics()
	if err != nil {
		return 0, fmt.Errorf("failed to fetch metrics: %w", err)
	}
	
	scanner := bufio.NewScanner(strings.NewReader(metrics))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, metricName) && !strings.HasPrefix(line, "#") {
			// Extract value from format: metric_name{labels} value
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var value float64
				if _, err := fmt.Sscanf(parts[len(parts)-1], "%f", &value); err == nil {
					return value, nil
				}
			}
		}
	}
	
	return 0, fmt.Errorf("metric %s not found", metricName)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}