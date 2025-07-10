package framework

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Extended NRDB client methods for dimensional metrics and OTLP testing

// VerifyMetricWithDimensions verifies a metric exists with specific dimensional attributes
func (c *NRDBClient) VerifyMetricWithDimensions(ctx context.Context, metricName string, dimensions map[string]interface{}, since string) error {
	// Build NRQL query with dimensional filters
	nrql := fmt.Sprintf("SELECT * FROM Metric WHERE metricName = '%s'", metricName)

	for key, value := range dimensions {
		switch v := value.(type) {
		case string:
			nrql += fmt.Sprintf(" AND `%s` = '%s'", key, v)
		case int, int64, float64:
			nrql += fmt.Sprintf(" AND `%s` = %v", key, v)
		}
	}

	nrql += fmt.Sprintf(" SINCE %s LIMIT 1", since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return fmt.Errorf("failed to query metric with dimensions: %w", err)
	}

	if len(result.Results) == 0 {
		return fmt.Errorf("metric %s not found with dimensions %v", metricName, dimensions)
	}

	return nil
}

// GetMetricCardinality returns the cardinality of a specific metric
func (c *NRDBClient) GetMetricCardinality(ctx context.Context, metricName string, since string) (int, error) {
	nrql := fmt.Sprintf("SELECT uniqueCount(*) as cardinality FROM Metric WHERE metricName = '%s' SINCE %s", metricName, since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return 0, err
	}

	if len(result.Results) == 0 {
		return 0, fmt.Errorf("no cardinality data found for metric %s", metricName)
	}

	cardinality, ok := result.Results[0]["cardinality"].(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected cardinality type for metric %s", metricName)
	}

	return int(cardinality), nil
}

// GetTotalMetricCardinality returns total cardinality across all metrics
func (c *NRDBClient) GetTotalMetricCardinality(ctx context.Context, since string) (int, error) {
	nrql := fmt.Sprintf("SELECT uniqueCount(metricName) as total_cardinality FROM Metric SINCE %s", since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return 0, err
	}

	if len(result.Results) == 0 {
		return 0, fmt.Errorf("no cardinality data found")
	}

	cardinality, ok := result.Results[0]["total_cardinality"].(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected total cardinality type")
	}

	return int(cardinality), nil
}

// MetricCardinalityInfo represents cardinality information for a metric
type MetricCardinalityInfo struct {
	MetricName  string
	Cardinality int
}

// GetHighCardinalityMetrics returns metrics with cardinality above threshold
func (c *NRDBClient) GetHighCardinalityMetrics(ctx context.Context, threshold int, since string) ([]MetricCardinalityInfo, error) {
	nrql := fmt.Sprintf("SELECT metricName, uniqueCount(*) as cardinality FROM Metric SINCE %s FACET metricName", since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return nil, err
	}

	var highCardinalityMetrics []MetricCardinalityInfo
	for _, row := range result.Results {
		metricName, _ := row["metricName"].(string)
		cardinality, ok := row["cardinality"].(float64)
		if !ok {
			continue
		}

		if int(cardinality) >= threshold {
			highCardinalityMetrics = append(highCardinalityMetrics, MetricCardinalityInfo{
				MetricName:  metricName,
				Cardinality: int(cardinality),
			})
		}
	}

	return highCardinalityMetrics, nil
}

// CheckAttributeExists checks if a specific attribute exists in metrics
func (c *NRDBClient) CheckAttributeExists(ctx context.Context, metricName string, attributeName string, since string) (bool, error) {
	nrql := fmt.Sprintf("SELECT * FROM Metric WHERE metricName = '%s' AND `%s` IS NOT NULL SINCE %s LIMIT 1",
		metricName, attributeName, since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return false, err
	}

	return len(result.Results) > 0, nil
}

// CheckResourceAttributeExists checks if a resource attribute exists
func (c *NRDBClient) CheckResourceAttributeExists(ctx context.Context, attributeName string, since string) (bool, error) {
	nrql := fmt.Sprintf("SELECT * FROM Metric WHERE `%s` IS NOT NULL SINCE %s LIMIT 1", attributeName, since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return false, err
	}

	return len(result.Results) > 0, nil
}

// VerifyResourceAttributeExists verifies that a resource attribute exists
func (c *NRDBClient) VerifyResourceAttributeExists(ctx context.Context, attributeName string, since string) (bool, error) {
	return c.CheckResourceAttributeExists(ctx, attributeName, since)
}

// VerifyAttributeExists verifies that an attribute exists for a metric
func (c *NRDBClient) VerifyAttributeExists(ctx context.Context, metricName string, attributeName string, since string) (bool, error) {
	return c.CheckAttributeExists(ctx, metricName, attributeName, since)
}

// VerifyMetricExists checks if a metric exists
func (c *NRDBClient) VerifyMetricExists(ctx context.Context, metricName string, since string) (bool, error) {
	nrql := fmt.Sprintf("SELECT * FROM Metric WHERE metricName = '%s' SINCE %s LIMIT 1", metricName, since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return false, err
	}

	return len(result.Results) > 0, nil
}

// GetMetricType returns the data type of a metric
func (c *NRDBClient) GetMetricType(ctx context.Context, metricName string, since string) (string, error) {
	// This is a simplified implementation - in practice, OTEL metric types
	// might be determined by other attributes or conventions
	nrql := fmt.Sprintf("SELECT * FROM Metric WHERE metricName = '%s' SINCE %s LIMIT 1", metricName, since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return "", err
	}

	if len(result.Results) == 0 {
		return "", fmt.Errorf("metric %s not found", metricName)
	}

	// Determine type based on metric naming conventions and available fields
	metricRow := result.Results[0]

	// Check for histogram-specific fields
	if _, hasSum := metricRow["sum"]; hasSum {
		if _, hasCount := metricRow["count"]; hasCount {
			return "histogram", nil
		}
	}

	// Check for counter patterns
	if strings.Contains(metricName, ".count") || strings.Contains(metricName, ".total") {
		return "counter", nil
	}

	// Default to gauge
	return "gauge", nil
}

// GetMetricDataType returns the OTLP data type of a metric
func (c *NRDBClient) GetMetricDataType(ctx context.Context, metricName string, since string) (string, error) {
	return c.GetMetricType(ctx, metricName, since)
}

// CountMetricsMatchingPattern counts metrics matching a pattern
func (c *NRDBClient) CountMetricsMatchingPattern(ctx context.Context, pattern string, since string) (int, error) {
	// Convert simple glob patterns to NRQL LIKE patterns
	likePattern := strings.ReplaceAll(pattern, "*", "%")

	nrql := fmt.Sprintf("SELECT uniqueCount(metricName) as count FROM Metric WHERE metricName LIKE '%s' SINCE %s",
		likePattern, since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return 0, err
	}

	if len(result.Results) == 0 {
		return 0, nil
	}

	count, ok := result.Results[0]["count"].(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected count type")
	}

	return int(count), nil
}

// PIISearchResult represents PII found in metrics
type PIISearchResult struct {
	MetricName string
	Value      string
	PIIType    string
}

// SearchForPIIInMetrics searches for PII patterns in metric data
func (c *NRDBClient) SearchForPIIInMetrics(ctx context.Context, piiPatterns []string, since string) ([]PIISearchResult, error) {
	var results []PIISearchResult

	// Search in metric attributes and values
	for _, pattern := range piiPatterns {
		// Escape pattern for NRQL
		escapedPattern := strings.ReplaceAll(pattern, "'", "\\'")

		nrql := fmt.Sprintf("SELECT * FROM Metric WHERE toString(*) LIKE '%%%s%%' SINCE %s LIMIT 10",
			escapedPattern, since)

		result, err := c.Query(ctx, nrql)
		if err != nil {
			continue // Skip on error
		}

		for _, row := range result.Results {
			metricName, _ := row["metricName"].(string)
			results = append(results, PIISearchResult{
				MetricName: metricName,
				Value:      pattern,
				PIIType:    getPIIType(pattern),
			})
		}
	}

	return results, nil
}

// CostControlMetric represents cost control metrics
type CostControlMetric struct {
	Timestamp      string
	EstimatedCost  float64
	MetricCount    int
	CardinalityMax int
}

// GetCostControlMetrics returns cost control processor metrics
func (c *NRDBClient) GetCostControlMetrics(ctx context.Context, since string) ([]CostControlMetric, error) {
	nrql := fmt.Sprintf("SELECT * FROM Metric WHERE metricName LIKE 'telemetry.cost%%' OR processor = 'costcontrol' SINCE %s", since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return nil, err
	}

	var metrics []CostControlMetric
	for _, row := range result.Results {
		timestamp, _ := row["timestamp"].(string)
		cost, _ := row["estimated_cost"].(float64)
		count, _ := row["metric_count"].(float64)
		cardinalityMax, _ := row["cardinality_max"].(float64)

		metrics = append(metrics, CostControlMetric{
			Timestamp:      timestamp,
			EstimatedCost:  cost,
			MetricCount:    int(count),
			CardinalityMax: int(cardinalityMax),
		})
	}

	return metrics, nil
}

// PlanAttribute represents query plan attributes
type PlanAttribute struct {
	MetricName string
	PlanHash   string
	PlanJSON   string
	QueryText  string
}

// GetPlanAttributes returns query plan attributes from planattributeextractor
func (c *NRDBClient) GetPlanAttributes(ctx context.Context, since string) ([]PlanAttribute, error) {
	nrql := fmt.Sprintf("SELECT * FROM Metric WHERE `db.plan.hash` IS NOT NULL OR `db.plan.json` IS NOT NULL SINCE %s", since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return nil, err
	}

	var attributes []PlanAttribute
	for _, row := range result.Results {
		metricName, _ := row["metricName"].(string)
		planHash, _ := row["db.plan.hash"].(string)
		planJSON, _ := row["db.plan.json"].(string)
		queryText, _ := row["db.statement"].(string)

		attributes = append(attributes, PlanAttribute{
			MetricName: metricName,
			PlanHash:   planHash,
			PlanJSON:   planJSON,
			QueryText:  queryText,
		})
	}

	return attributes, nil
}

// ExemplarInfo represents metric exemplar information
type ExemplarInfo struct {
	MetricName string
	TraceID    string
	SpanID     string
	Value      float64
	Timestamp  string
}

// GetMetricExemplars returns exemplars for a metric
func (c *NRDBClient) GetMetricExemplars(ctx context.Context, metricName string, since string) ([]ExemplarInfo, error) {
	nrql := fmt.Sprintf("SELECT * FROM Metric WHERE metricName = '%s' AND traceId IS NOT NULL SINCE %s",
		metricName, since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return nil, err
	}

	var exemplars []ExemplarInfo
	for _, row := range result.Results {
		traceID, _ := row["traceId"].(string)
		spanID, _ := row["spanId"].(string)
		value, _ := row["value"].(float64)
		timestamp, _ := row["timestamp"].(string)

		exemplars = append(exemplars, ExemplarInfo{
			MetricName: metricName,
			TraceID:    traceID,
			SpanID:     spanID,
			Value:      value,
			Timestamp:  timestamp,
		})
	}

	return exemplars, nil
}

// BatchMetrics represents batch processing metrics
type BatchMetrics struct {
	BatchSize         int
	ProcessingLatency int
	ThroughputPerSec  float64
}

// GetBatchProcessingMetrics returns batch processing performance metrics
func (c *NRDBClient) GetBatchProcessingMetrics(ctx context.Context, since string) (*BatchMetrics, error) {
	nrql := fmt.Sprintf("SELECT average(batch_size) as avg_batch_size, average(processing_latency_ms) as avg_latency, sum(throughput) as total_throughput FROM Metric WHERE processor = 'batch' SINCE %s", since)

	result, err := c.Query(ctx, nrql)
	if err != nil {
		return nil, err
	}

	if len(result.Results) == 0 {
		return &BatchMetrics{}, nil
	}

	row := result.Results[0]
	batchSize, _ := row["avg_batch_size"].(float64)
	latency, _ := row["avg_latency"].(float64)
	throughput, _ := row["total_throughput"].(float64)

	return &BatchMetrics{
		BatchSize:         int(batchSize),
		ProcessingLatency: int(latency),
		ThroughputPerSec:  throughput,
	}, nil
}

// Helper function to determine PII type from pattern
func getPIIType(pattern string) string {
	// SSN pattern
	if matched, _ := regexp.MatchString(`\d{3}-\d{2}-\d{4}`, pattern); matched {
		return "ssn"
	}

	// Email pattern
	if matched, _ := regexp.MatchString(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, pattern); matched {
		return "email"
	}

	// Phone pattern
	if matched, _ := regexp.MatchString(`\d{3}-\d{3}-\d{4}`, pattern); matched {
		return "phone"
	}

	return "unknown"
}
