package mongodb

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Verifier verifies MongoDB metrics
type Verifier struct {
	promAPI v1.API
}

// MetricPoint represents a single metric data point
type MetricPoint struct {
	Labels map[string]string
	Value  float64
	Time   time.Time
}

// NewVerifier creates a new MongoDB metrics verifier
func NewVerifier(prometheusEndpoint string) *Verifier {
	client, err := api.NewClient(api.Config{
		Address: prometheusEndpoint,
		RoundTripper: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
		},
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create Prometheus client: %v", err))
	}
	
	return &Verifier{
		promAPI: v1.NewAPI(client),
	}
}

// GetMetrics retrieves metrics by name
func (v *Verifier) GetMetrics(metricName string) ([]MetricPoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Query for the metric
	query := metricName
	result, warnings, err := v.promAPI.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	
	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}
	
	// Parse results
	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}
	
	metrics := make([]MetricPoint, 0, len(vector))
	for _, sample := range vector {
		labels := make(map[string]string)
		for k, v := range sample.Metric {
			labels[string(k)] = string(v)
		}
		
		metrics = append(metrics, MetricPoint{
			Labels: labels,
			Value:  float64(sample.Value),
			Time:   sample.Timestamp.Time(),
		})
	}
	
	return metrics, nil
}

// GetOperationCounts returns operation counts by type
func (v *Verifier) GetOperationCounts() (map[string]float64, error) {
	metrics, err := v.GetMetrics("mongodb_test_operation_count")
	if err != nil {
		return nil, err
	}
	
	counts := make(map[string]float64)
	for _, metric := range metrics {
		if opType, ok := metric.Labels["operation"]; ok {
			counts[opType] = metric.Value
		}
	}
	
	return counts, nil
}

// GetLatencyPercentiles returns latency percentiles
func (v *Verifier) GetLatencyPercentiles() (map[string]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	percentiles := map[string]string{
		"p50": "0.5",
		"p95": "0.95",
		"p99": "0.99",
	}
	
	results := make(map[string]float64)
	
	for name, quantile := range percentiles {
		query := fmt.Sprintf(
			`histogram_quantile(%s, sum(rate(mongodb_test_operation_latency_bucket[5m])) by (le))`,
			quantile,
		)
		
		result, _, err := v.promAPI.Query(ctx, query, time.Now())
		if err != nil {
			return nil, fmt.Errorf("failed to query %s: %w", name, err)
		}
		
		if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
			// Convert from seconds to milliseconds
			results[name] = float64(vector[0].Value) * 1000
		}
	}
	
	return results, nil
}

// GetThroughput returns operations per second
func (v *Verifier) GetThroughput() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	query := `sum(rate(mongodb_test_operation_count[1m]))`
	result, _, err := v.promAPI.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}
	
	if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
		return float64(vector[0].Value), nil
	}
	
	return 0, fmt.Errorf("no throughput data available")
}

// VerifyDatabaseMetrics checks that all expected database metrics exist
func (v *Verifier) VerifyDatabaseMetrics(expectedDatabases []string) error {
	metrics, err := v.GetMetrics("mongodb_test_database_size")
	if err != nil {
		return err
	}
	
	foundDatabases := make(map[string]bool)
	for _, metric := range metrics {
		if db, ok := metric.Labels["database"]; ok {
			foundDatabases[db] = true
		}
	}
	
	for _, expected := range expectedDatabases {
		if !foundDatabases[expected] {
			return fmt.Errorf("missing metrics for database: %s", expected)
		}
	}
	
	return nil
}

// VerifyCollectionMetrics checks collection-level metrics
func (v *Verifier) VerifyCollectionMetrics(database string, expectedCollections []string) error {
	// Check collection count metrics
	countMetrics, err := v.GetMetrics("mongodb_test_collection_count")
	if err != nil {
		return err
	}
	
	foundCollections := make(map[string]bool)
	for _, metric := range countMetrics {
		if metric.Labels["database"] == database {
			if coll, ok := metric.Labels["collection"]; ok {
				foundCollections[coll] = true
			}
		}
	}
	
	for _, expected := range expectedCollections {
		if !foundCollections[expected] {
			return fmt.Errorf("missing count metrics for collection: %s.%s", database, expected)
		}
	}
	
	// Check collection size metrics
	sizeMetrics, err := v.GetMetrics("mongodb_test_collection_size")
	if err != nil {
		return err
	}
	
	foundSizes := make(map[string]bool)
	for _, metric := range sizeMetrics {
		if metric.Labels["database"] == database {
			if coll, ok := metric.Labels["collection"]; ok {
				foundSizes[coll] = true
				if metric.Value <= 0 {
					return fmt.Errorf("collection %s.%s has invalid size: %f", database, coll, metric.Value)
				}
			}
		}
	}
	
	for _, expected := range expectedCollections {
		if !foundSizes[expected] {
			return fmt.Errorf("missing size metrics for collection: %s.%s", database, expected)
		}
	}
	
	return nil
}

// WaitForMetrics waits for metrics to appear
func (v *Verifier) WaitForMetrics(metricName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		metrics, err := v.GetMetrics(metricName)
		if err == nil && len(metrics) > 0 {
			return nil
		}
		
		time.Sleep(2 * time.Second)
	}
	
	return fmt.Errorf("timeout waiting for metric: %s", metricName)
}

// GetReplicationLag returns the maximum replication lag
func (v *Verifier) GetReplicationLag() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	query := `max(mongodb_test_replication_lag_seconds)`
	result, _, err := v.promAPI.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}
	
	if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
		return float64(vector[0].Value), nil
	}
	
	return 0, fmt.Errorf("no replication lag data available")
}

// GetConnectionCount returns the current connection count
func (v *Verifier) GetConnectionCount() (int, error) {
	metrics, err := v.GetMetrics("mongodb_test_connection_count")
	if err != nil {
		return 0, err
	}
	
	totalConnections := 0
	for _, metric := range metrics {
		totalConnections += int(metric.Value)
	}
	
	return totalConnections, nil
}

// ValidateMetricLabels ensures metrics have expected labels
func (v *Verifier) ValidateMetricLabels(metricName string, requiredLabels []string) error {
	metrics, err := v.GetMetrics(metricName)
	if err != nil {
		return err
	}
	
	if len(metrics) == 0 {
		return fmt.Errorf("no metrics found for: %s", metricName)
	}
	
	// Check first metric for required labels
	sample := metrics[0]
	for _, label := range requiredLabels {
		if _, ok := sample.Labels[label]; !ok {
			return fmt.Errorf("metric %s missing required label: %s", metricName, label)
		}
	}
	
	return nil
}