// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package validators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// NRDBValidator validates data in New Relic Database (NRDB)
type NRDBValidator struct {
	logger         *zap.Logger
	config         *NRDBConfig
	httpClient     *http.Client
	queryResults   map[string]*NRDBQueryResult
	mutex          sync.RWMutex
}

// NRDBConfig defines configuration for NRDB validation
type NRDBConfig struct {
	APIKey        string        `json:"api_key"`
	AccountID     string        `json:"account_id"`
	Region        string        `json:"region"`
	GraphQLURL    string        `json:"graphql_url"`
	QueryTimeout  time.Duration `json:"query_timeout"`
	RetryAttempts int           `json:"retry_attempts"`
	RetryDelay    time.Duration `json:"retry_delay"`
}

// NRDBQuery represents a New Relic database query
type NRDBQuery struct {
	Name            string        `json:"name"`
	Query           string        `json:"query"`
	ExpectedResults int           `json:"expected_results"`
	Timeout         time.Duration `json:"timeout"`
	RetryCount      int           `json:"retry_count"`
	Critical        bool          `json:"critical"`
	Description     string        `json:"description"`
	Tags            []string      `json:"tags"`
}

// NRDBQueryResult contains the result of a NRDB query
type NRDBQueryResult struct {
	QueryName       string                   `json:"query_name"`
	Query           string                   `json:"query"`
	Results         []map[string]interface{} `json:"results"`
	ResultCount     int                      `json:"result_count"`
	ExecutionTimeMS int64                    `json:"execution_time_ms"`
	Timestamp       time.Time                `json:"timestamp"`
	Success         bool                     `json:"success"`
	ErrorMessage    string                   `json:"error_message,omitempty"`
	Metadata        map[string]interface{}   `json:"metadata"`
}

// NRDBValidationResult contains NRDB validation results
type NRDBValidationResult struct {
	QueryName       string    `json:"query_name"`
	Query           string    `json:"query"`
	ExpectedResults int       `json:"expected_results"`
	ActualResults   int       `json:"actual_results"`
	Passed          bool      `json:"passed"`
	ExecutionTimeMS int64     `json:"execution_time_ms"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	Critical        bool      `json:"critical"`
	Details         map[string]interface{} `json:"details"`
}

// GraphQLRequest represents a GraphQL request to New Relic
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL response from New Relic
type GraphQLResponse struct {
	Data   interface{} `json:"data"`
	Errors []struct {
		Message   string                 `json:"message"`
		Locations []map[string]interface{} `json:"locations,omitempty"`
		Path      []interface{}          `json:"path,omitempty"`
	} `json:"errors,omitempty"`
}

// NerdGraphActorResponse represents the actor response structure
type NerdGraphActorResponse struct {
	Actor struct {
		Account struct {
			NRQL struct {
				Results []map[string]interface{} `json:"results"`
			} `json:"nrql"`
		} `json:"account"`
	} `json:"actor"`
}

// NewNRDBValidator creates a new NRDB validator
func NewNRDBValidator(logger *zap.Logger, config *NRDBConfig) *NRDBValidator {
	// Set default GraphQL URL if not provided
	if config.GraphQLURL == "" {
		if config.Region == "EU" {
			config.GraphQLURL = "https://api.eu.newrelic.com/graphql"
		} else {
			config.GraphQLURL = "https://api.newrelic.com/graphql"
		}
	}
	
	// Set default timeout if not provided
	if config.QueryTimeout == 0 {
		config.QueryTimeout = 30 * time.Second
	}
	
	return &NRDBValidator{
		logger:       logger,
		config:       config,
		httpClient:   &http.Client{Timeout: config.QueryTimeout},
		queryResults: make(map[string]*NRDBQueryResult),
	}
}

// ValidateNRDBQueries validates a set of NRDB queries
func (nv *NRDBValidator) ValidateNRDBQueries(ctx context.Context, queries []NRDBQuery) ([]NRDBValidationResult, error) {
	nv.logger.Info("Starting NRDB validation", zap.Int("query_count", len(queries)))
	
	var results []NRDBValidationResult
	var wg sync.WaitGroup
	var resultsMutex sync.Mutex
	
	// Execute queries concurrently
	for _, query := range queries {
		wg.Add(1)
		go func(q NRDBQuery) {
			defer wg.Done()
			
			result := nv.executeAndValidateQuery(ctx, q)
			
			resultsMutex.Lock()
			results = append(results, result)
			resultsMutex.Unlock()
		}(query)
	}
	
	wg.Wait()
	
	nv.logger.Info("NRDB validation completed", 
		zap.Int("total_queries", len(queries)),
		zap.Int("results", len(results)))
	
	return results, nil
}

// executeAndValidateQuery executes a single NRDB query and validates the result
func (nv *NRDBValidator) executeAndValidateQuery(ctx context.Context, query NRDBQuery) NRDBValidationResult {
	startTime := time.Now()
	
	result := NRDBValidationResult{
		QueryName:       query.Name,
		Query:           query.Query,
		ExpectedResults: query.ExpectedResults,
		Critical:        query.Critical,
		Timestamp:       startTime,
		Details:         make(map[string]interface{}),
	}
	
	// Execute the query with retries
	queryResult, err := nv.executeQueryWithRetries(ctx, query)
	if err != nil {
		result.Passed = false
		result.ErrorMessage = err.Error()
		result.ExecutionTimeMS = time.Since(startTime).Milliseconds()
		return result
	}
	
	// Store query result
	nv.mutex.Lock()
	nv.queryResults[query.Name] = queryResult
	nv.mutex.Unlock()
	
	// Validate the result
	result.ActualResults = queryResult.ResultCount
	result.ExecutionTimeMS = queryResult.ExecutionTimeMS
	result.Passed = nv.validateQueryResult(query, queryResult)
	
	if !result.Passed {
		result.ErrorMessage = fmt.Sprintf("Expected %d results, got %d", 
			query.ExpectedResults, queryResult.ResultCount)
	}
	
	// Add detailed information
	result.Details["query_success"] = queryResult.Success
	result.Details["has_data"] = queryResult.ResultCount > 0
	result.Details["execution_time_ms"] = queryResult.ExecutionTimeMS
	result.Details["result_sample"] = nv.getSampleResults(queryResult.Results, 3)
	
	return result
}

// executeQueryWithRetries executes a NRDB query with retry logic
func (nv *NRDBValidator) executeQueryWithRetries(ctx context.Context, query NRDBQuery) (*NRDBQueryResult, error) {
	var lastErr error
	retryCount := query.RetryCount
	if retryCount == 0 {
		retryCount = nv.config.RetryAttempts
	}
	
	for attempt := 0; attempt <= retryCount; attempt++ {
		if attempt > 0 {
			nv.logger.Info("Retrying NRDB query", 
				zap.String("query", query.Name),
				zap.Int("attempt", attempt))
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(nv.config.RetryDelay):
			}
		}
		
		result, err := nv.executeNRDBQuery(ctx, query)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		nv.logger.Warn("NRDB query attempt failed",
			zap.String("query", query.Name),
			zap.Int("attempt", attempt),
			zap.Error(err))
	}
	
	return nil, fmt.Errorf("failed after %d attempts: %w", retryCount+1, lastErr)
}

// executeNRDBQuery executes a single NRDB query
func (nv *NRDBValidator) executeNRDBQuery(ctx context.Context, query NRDBQuery) (*NRDBQueryResult, error) {
	startTime := time.Now()
	
	// Construct GraphQL query
	graphqlQuery := nv.buildGraphQLQuery(query.Query)
	
	// Create request
	reqBody := GraphQLRequest{
		Query: graphqlQuery,
		Variables: map[string]interface{}{
			"accountId": nv.config.AccountID,
		},
	}
	
	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", nv.config.GraphQLURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("API-Key", nv.config.APIKey)
	
	// Execute request
	resp, err := nv.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Parse response
	var graphqlResp GraphQLResponse
	if err := json.Unmarshal(respBody, &graphqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Check for GraphQL errors
	if len(graphqlResp.Errors) > 0 {
		var errorMessages []string
		for _, err := range graphqlResp.Errors {
			errorMessages = append(errorMessages, err.Message)
		}
		return nil, fmt.Errorf("GraphQL errors: %s", strings.Join(errorMessages, "; "))
	}
	
	// Extract NRQL results
	results, err := nv.extractNRQLResults(graphqlResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to extract results: %w", err)
	}
	
	executionTime := time.Since(startTime)
	
	queryResult := &NRDBQueryResult{
		QueryName:       query.Name,
		Query:           query.Query,
		Results:         results,
		ResultCount:     len(results),
		ExecutionTimeMS: executionTime.Milliseconds(),
		Timestamp:       startTime,
		Success:         true,
		Metadata: map[string]interface{}{
			"response_size": len(respBody),
			"http_status":   resp.StatusCode,
		},
	}
	
	return queryResult, nil
}

// buildGraphQLQuery builds a GraphQL query for NRQL execution
func (nv *NRDBValidator) buildGraphQLQuery(nrqlQuery string) string {
	return fmt.Sprintf(`
		query($accountId: Int!) {
			actor {
				account(id: $accountId) {
					nrql(query: "%s") {
						results
					}
				}
			}
		}
	`, strings.ReplaceAll(nrqlQuery, `"`, `\"`))
}

// extractNRQLResults extracts NRQL results from GraphQL response
func (nv *NRDBValidator) extractNRQLResults(data interface{}) ([]map[string]interface{}, error) {
	// Convert to JSON and back to get proper structure
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}
	
	var actorResp NerdGraphActorResponse
	if err := json.Unmarshal(dataJSON, &actorResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actor response: %w", err)
	}
	
	return actorResp.Actor.Account.NRQL.Results, nil
}

// validateQueryResult validates a query result against expectations
func (nv *NRDBValidator) validateQueryResult(query NRDBQuery, result *NRDBQueryResult) bool {
	if !result.Success {
		return false
	}
	
	// Validate result count
	switch {
	case query.ExpectedResults == 0:
		// Expecting no results
		return result.ResultCount == 0
	case query.ExpectedResults > 0:
		// Expecting specific number of results
		return result.ResultCount >= query.ExpectedResults
	default:
		// Expecting any results (negative expected results means "any")
		return result.ResultCount > 0
	}
}

// getSampleResults returns a sample of results for debugging
func (nv *NRDBValidator) getSampleResults(results []map[string]interface{}, maxSamples int) []map[string]interface{} {
	if len(results) == 0 {
		return results
	}
	
	if len(results) <= maxSamples {
		return results
	}
	
	return results[:maxSamples]
}

// ValidateEntitySynthesis validates that entities are being properly synthesized in NRDB
func (nv *NRDBValidator) ValidateEntitySynthesis(ctx context.Context) (*NRDBValidationResult, error) {
	query := NRDBQuery{
		Name: "entity_synthesis_validation",
		Query: `
			SELECT count(*) 
			FROM Metric 
			WHERE instrumentation.provider = 'database-intelligence' 
			AND entity.guid IS NOT NULL 
			SINCE 10 minutes ago
		`,
		ExpectedResults: 1,
		Critical:        true,
		Description:     "Validate that database entities are being synthesized properly",
	}
	
	result := nv.executeAndValidateQuery(ctx, query)
	return &result, nil
}

// ValidateMetricCardinality validates metric cardinality in NRDB
func (nv *NRDBValidator) ValidateMetricCardinality(ctx context.Context) (*NRDBValidationResult, error) {
	query := NRDBQuery{
		Name: "metric_cardinality_validation",
		Query: `
			SELECT cardinality() 
			FROM Metric 
			WHERE instrumentation.provider = 'database-intelligence' 
			SINCE 1 hour ago 
			FACET metricName
		`,
		ExpectedResults: 1,
		Critical:        false,
		Description:     "Validate metric cardinality is within acceptable limits",
	}
	
	result := nv.executeAndValidateQuery(ctx, query)
	
	// Additional validation for cardinality limits
	if result.Passed && len(nv.queryResults[query.Name].Results) > 0 {
		for _, resultRow := range nv.queryResults[query.Name].Results {
			if cardinality, ok := resultRow["cardinality"].(float64); ok {
				if cardinality > 10000 { // Example threshold
					result.Passed = false
					result.ErrorMessage = fmt.Sprintf("High cardinality detected: %.0f", cardinality)
					break
				}
			}
		}
	}
	
	return &result, nil
}

// ValidateDataFreshness validates that data is fresh in NRDB
func (nv *NRDBValidator) ValidateDataFreshness(ctx context.Context, maxAge time.Duration) (*NRDBValidationResult, error) {
	query := NRDBQuery{
		Name: "data_freshness_validation",
		Query: fmt.Sprintf(`
			SELECT latest(timestamp) 
			FROM Metric 
			WHERE instrumentation.provider = 'database-intelligence' 
			SINCE %d minutes ago
		`, int(maxAge.Minutes())),
		ExpectedResults: 1,
		Critical:        true,
		Description:     "Validate that recent data is available in NRDB",
	}
	
	result := nv.executeAndValidateQuery(ctx, query)
	
	// Additional validation for timestamp freshness
	if result.Passed && len(nv.queryResults[query.Name].Results) > 0 {
		if timestampRow := nv.queryResults[query.Name].Results[0]; timestampRow != nil {
			if latestTimestamp, ok := timestampRow["latest"].(float64); ok {
				latestTime := time.Unix(int64(latestTimestamp/1000), 0)
				if time.Since(latestTime) > maxAge {
					result.Passed = false
					result.ErrorMessage = fmt.Sprintf("Data is stale: latest timestamp is %v", latestTime)
				}
			}
		}
	}
	
	return &result, nil
}

// ValidateQueryNormalization validates that queries are being normalized
func (nv *NRDBValidator) ValidateQueryNormalization(ctx context.Context) (*NRDBValidationResult, error) {
	query := NRDBQuery{
		Name: "query_normalization_validation",
		Query: `
			SELECT count(*) 
			FROM Log 
			WHERE service.name = 'database-intelligence' 
			AND query.normalized IS NOT NULL 
			AND query.fingerprint IS NOT NULL 
			SINCE 10 minutes ago
		`,
		ExpectedResults: 1,
		Critical:        true,
		Description:     "Validate that database queries are being normalized",
	}
	
	result := nv.executeAndValidateQuery(ctx, query)
	return &result, nil
}

// ValidatePIISanitization validates that PII is being properly sanitized
func (nv *NRDBValidator) ValidatePIISanitization(ctx context.Context) (*NRDBValidationResult, error) {
	// This query checks for common PII patterns that should NOT be present
	query := NRDBQuery{
		Name: "pii_sanitization_validation",
		Query: `
			SELECT count(*) 
			FROM Log 
			WHERE service.name = 'database-intelligence' 
			AND (
				message LIKE '%@%' OR 
				message RLIKE '[0-9]{3}-[0-9]{2}-[0-9]{4}' OR
				message RLIKE '[0-9]{4}[\s-]?[0-9]{4}[\s-]?[0-9]{4}[\s-]?[0-9]{4}'
			)
			SINCE 10 minutes ago
		`,
		ExpectedResults: 0, // We expect NO results (no PII should be present)
		Critical:        true,
		Description:     "Validate that PII patterns are not present in logs",
	}
	
	result := nv.executeAndValidateQuery(ctx, query)
	
	// For this validation, success means NO results found
	if result.ActualResults == 0 {
		result.Passed = true
		result.ErrorMessage = ""
	} else {
		result.Passed = false
		result.ErrorMessage = fmt.Sprintf("Found %d potential PII violations", result.ActualResults)
	}
	
	return &result, nil
}

// GetQueryResults returns stored query results
func (nv *NRDBValidator) GetQueryResults() map[string]*NRDBQueryResult {
	nv.mutex.RLock()
	defer nv.mutex.RUnlock()
	
	// Return a copy
	results := make(map[string]*NRDBQueryResult)
	for k, v := range nv.queryResults {
		results[k] = v
	}
	
	return results
}

// GetValidationSummary returns a summary of NRDB validation results
func (nv *NRDBValidator) GetValidationSummary(results []NRDBValidationResult) map[string]interface{} {
	totalQueries := len(results)
	passedQueries := 0
	failedQueries := 0
	criticalFailures := 0
	var totalExecutionTime int64
	
	for _, result := range results {
		totalExecutionTime += result.ExecutionTimeMS
		
		if result.Passed {
			passedQueries++
		} else {
			failedQueries++
			if result.Critical {
				criticalFailures++
			}
		}
	}
	
	var averageExecutionTime float64
	if totalQueries > 0 {
		averageExecutionTime = float64(totalExecutionTime) / float64(totalQueries)
	}
	
	return map[string]interface{}{
		"total_queries":         totalQueries,
		"passed_queries":        passedQueries,
		"failed_queries":        failedQueries,
		"critical_failures":     criticalFailures,
		"success_rate":          float64(passedQueries) / float64(totalQueries),
		"avg_execution_time_ms": averageExecutionTime,
		"total_execution_time_ms": totalExecutionTime,
	}
}