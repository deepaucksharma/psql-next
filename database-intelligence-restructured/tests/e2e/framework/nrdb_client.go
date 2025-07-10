package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NRDBClient represents a client for querying New Relic Database
type NRDBClient struct {
	accountID  string
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

// NewNRDBClient creates a new NRDB client
func NewNRDBClient(accountID, apiKey string) *NRDBClient {
	return &NRDBClient{
		accountID: accountID,
		apiKey:    apiKey,
		endpoint:  "https://api.newrelic.com/graphql",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NRQLResult represents the result of an NRQL query
type NRQLResult struct {
	Results []map[string]interface{} `json:"results"`
	Facets  []string                 `json:"facets"`
	Total   int                      `json:"total"`
}

// Query executes an NRQL query against NRDB
func (c *NRDBClient) Query(ctx context.Context, nrql string) (*NRQLResult, error) {
	query := fmt.Sprintf(`
		{
			actor {
				account(id: %s) {
					nrql(query: "%s") {
						results
					}
				}
			}
		}
	`, c.accountID, nrql)
	
	requestBody := map[string]string{
		"query": query,
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("API-Key", c.apiKey)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("NRDB query failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var response struct {
		Data struct {
			Actor struct {
				Account struct {
					NRQL NRQLResult `json:"nrql"`
				} `json:"account"`
			} `json:"actor"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("NRDB query errors: %v", response.Errors)
	}
	
	return &response.Data.Actor.Account.NRQL, nil
}

// WaitForData waits for data to appear in NRDB
func (c *NRDBClient) WaitForData(ctx context.Context, nrql string, timeout time.Duration) (*NRQLResult, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for data")
			}
			
			result, err := c.Query(ctx, nrql)
			if err != nil {
				continue // Retry on error
			}
			
			if len(result.Results) > 0 {
				return result, nil
			}
		}
	}
}

// VerifyMetric verifies a metric exists in NRDB with expected attributes
func (c *NRDBClient) VerifyMetric(ctx context.Context, metricName string, attributes map[string]interface{}, since string) error {
	// Build NRQL query
	nrql := fmt.Sprintf("SELECT * FROM Metric WHERE metricName = '%s'", metricName)
	
	for key, value := range attributes {
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
		return fmt.Errorf("failed to query metric: %w", err)
	}
	
	if len(result.Results) == 0 {
		return fmt.Errorf("metric %s not found with attributes %v", metricName, attributes)
	}
	
	return nil
}

// GetMetricValue retrieves the latest value of a metric
func (c *NRDBClient) GetMetricValue(ctx context.Context, metricName string, since string) (float64, error) {
	nrql := fmt.Sprintf("SELECT latest(%s) as value FROM Metric WHERE metricName = '%s' SINCE %s",
		metricName, metricName, since)
	
	result, err := c.Query(ctx, nrql)
	if err != nil {
		return 0, err
	}
	
	if len(result.Results) == 0 {
		return 0, fmt.Errorf("no data found for metric %s", metricName)
	}
	
	value, ok := result.Results[0]["value"].(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected value type for metric %s", metricName)
	}
	
	return value, nil
}

// GetMetricSum retrieves the sum of a metric over a time period
func (c *NRDBClient) GetMetricSum(ctx context.Context, metricName string, since string) (float64, error) {
	nrql := fmt.Sprintf("SELECT sum(%s) as total FROM Metric WHERE metricName = '%s' SINCE %s",
		metricName, metricName, since)
	
	result, err := c.Query(ctx, nrql)
	if err != nil {
		return 0, err
	}
	
	if len(result.Results) == 0 {
		return 0, fmt.Errorf("no data found for metric %s", metricName)
	}
	
	total, ok := result.Results[0]["total"].(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected value type for metric %s sum", metricName)
	}
	
	return total, nil
}

// VerifyLog verifies a log entry exists in NRDB
func (c *NRDBClient) VerifyLog(ctx context.Context, attributes map[string]interface{}, since string) error {
	nrql := "SELECT * FROM Log WHERE 1=1"
	
	for key, value := range attributes {
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
		return fmt.Errorf("failed to query logs: %w", err)
	}
	
	if len(result.Results) == 0 {
		return fmt.Errorf("log entry not found with attributes %v", attributes)
	}
	
	return nil
}

// GetQueryPlans retrieves query plans from NRDB
func (c *NRDBClient) GetQueryPlans(ctx context.Context, queryHash string, since string) ([]map[string]interface{}, error) {
	nrql := fmt.Sprintf("SELECT * FROM Log WHERE plan.hash = '%s' SINCE %s", queryHash, since)
	
	result, err := c.Query(ctx, nrql)
	if err != nil {
		return nil, err
	}
	
	return result.Results, nil
}

// CompareMetrics compares metrics between source database and NRDB
func (c *NRDBClient) CompareMetrics(ctx context.Context, sourceValue float64, metricName string, tolerance float64) error {
	nrdbValue, err := c.GetMetricValue(ctx, metricName, "5 minutes ago")
	if err != nil {
		return err
	}
	
	diff := abs(sourceValue - nrdbValue)
	if tolerance == 0 {
		// Exact match required
		if diff > 0 {
			return fmt.Errorf("metric %s mismatch: source=%f, nrdb=%f", metricName, sourceValue, nrdbValue)
		}
	} else {
		// Percentage tolerance
		percentDiff := diff / sourceValue * 100
		if percentDiff > tolerance {
			return fmt.Errorf("metric %s exceeds tolerance: source=%f, nrdb=%f, diff=%.2f%%", 
				metricName, sourceValue, nrdbValue, percentDiff)
		}
	}
	
	return nil
}

// Helper function for absolute value
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}