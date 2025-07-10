package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const nerdGraphEndpoint = "https://api.newrelic.com/graphql"

type QueryTest struct {
	Name        string
	Description string
	Query       string
}

var queries = []QueryTest{
	// Bird's-Eye View Page
	{
		Name:        "Unique Queries by Database",
		Description: "Count of unique queries per database",
		Query:       "SELECT uniqueCount(attributes.db.postgresql.query_id) FROM Metric WHERE metricName LIKE 'postgres.slow_queries%' FACET attributes.db.name SINCE 1 hour ago",
	},
	{
		Name:        "Average Query Execution Time",
		Description: "Average execution time by query",
		Query:       "SELECT latest(postgres.slow_queries.elapsed_time) FROM Metric WHERE attributes.db.statement != '<insufficient privilege>' FACET attributes.db.statement SINCE 1 hour ago LIMIT 20",
	},
	{
		Name:        "Query Execution Count Over Time",
		Description: "Total query executions over time",
		Query:       "SELECT sum(postgres.slow_queries.count) FROM Metric TIMESERIES AUTO SINCE 1 hour ago",
	},
	{
		Name:        "Top Wait Events",
		Description: "Most common wait events",
		Query:       "SELECT sum(postgres.wait_events) FROM Metric FACET attributes.db.wait_event.name WHERE attributes.db.wait_event.name IS NOT NULL SINCE 1 hour ago LIMIT 20",
	},
	{
		Name:        "Slowest Queries Table",
		Description: "Detailed slow query information",
		Query:       "SELECT latest(attributes.db.name) as 'Database', latest(attributes.db.statement) as 'Query', latest(postgres.slow_queries.elapsed_time) as 'Avg Time (ms)' FROM Metric WHERE metricName LIKE 'postgres.slow_queries%' FACET attributes.db.postgresql.query_id SINCE 1 hour ago LIMIT 10",
	},
	{
		Name:        "Disk Read Activity",
		Description: "Average disk reads by database",
		Query:       "SELECT average(postgres.slow_queries.disk_reads) as 'Avg Disk Reads' FROM Metric WHERE metricName = 'postgres.slow_queries.disk_reads' FACET attributes.db.name TIMESERIES AUTO SINCE 1 hour ago",
	},
	{
		Name:        "Disk Write Activity",
		Description: "Average disk writes by database",
		Query:       "SELECT average(postgres.slow_queries.disk_writes) as 'Avg Disk Writes' FROM Metric WHERE metricName = 'postgres.slow_queries.disk_writes' FACET attributes.db.name TIMESERIES AUTO SINCE 1 hour ago",
	},
	{
		Name:        "Blocking Sessions",
		Description: "Active blocking sessions",
		Query:       "SELECT latest(attributes.db.blocking.blocked_pid) as 'Blocked PID', latest(attributes.db.blocking.blocking_pid) as 'Blocking PID' FROM Metric WHERE metricName = 'postgres.blocking_sessions' FACET attributes.db.blocking.blocked_pid SINCE 1 hour ago LIMIT 10",
	},
	// Database Health Metrics
	{
		Name:        "Active Connections",
		Description: "Current backend connections",
		Query:       "SELECT latest(postgresql.backends) FROM Metric WHERE metricName = 'postgresql.backends' FACET attributes.postgresql.database.name SINCE 1 hour ago",
	},
	{
		Name:        "Database Size",
		Description: "Database sizes in MB",
		Query:       "SELECT latest(postgresql.db_size) / 1024 / 1024 as 'Size (MB)' FROM Metric WHERE metricName = 'postgresql.db_size' FACET attributes.postgresql.database.name SINCE 1 hour ago",
	},
	{
		Name:        "Transaction Rate",
		Description: "Commits and rollbacks",
		Query:       "SELECT sum(postgresql.commits) as 'Commits', sum(postgresql.rollbacks) as 'Rollbacks' FROM Metric WHERE metricName IN ('postgresql.commits', 'postgresql.rollbacks') TIMESERIES AUTO SINCE 1 hour ago",
	},
	// Wait Event Analysis
	{
		Name:        "Wait Events by Category",
		Description: "Wait events grouped by category",
		Query:       "SELECT sum(postgres.wait_events) FROM Metric WHERE metricName = 'postgres.wait_events' AND attributes.db.wait_event.name IS NOT NULL FACET attributes.db.wait_event.category SINCE 1 hour ago",
	},
	{
		Name:        "Wait Event Timeline",
		Description: "Wait events over time",
		Query:       "SELECT sum(postgres.wait_events) FROM Metric WHERE metricName = 'postgres.wait_events' AND attributes.db.wait_event.name IS NOT NULL FACET attributes.db.wait_event.name TIMESERIES AUTO SINCE 1 hour ago LIMIT 10",
	},
	// Query Performance
	{
		Name:        "Query Performance by Operation",
		Description: "Average time by SQL operation type",
		Query:       "SELECT average(postgres.slow_queries.elapsed_time) as 'Avg Time (ms)' FROM Metric WHERE metricName = 'postgres.slow_queries.elapsed_time' FACET attributes.db.operation SINCE 1 hour ago",
	},
	{
		Name:        "Query Count by Operation",
		Description: "Number of queries by type",
		Query:       "SELECT sum(postgres.slow_queries.count) FROM Metric WHERE metricName = 'postgres.slow_queries.count' FACET attributes.db.operation SINCE 1 hour ago",
	},
	// Individual Queries
	{
		Name:        "CPU Time by Query",
		Description: "CPU time for individual queries",
		Query:       "SELECT latest(attributes.db.statement) as 'Query', latest(postgres.individual_queries.cpu_time) as 'CPU Time (ms)' FROM Metric WHERE metricName = 'postgres.individual_queries.cpu_time' FACET attributes.db.postgresql.query_id SINCE 1 hour ago LIMIT 10",
	},
	// Execution Plans
	{
		Name:        "Query Execution Plans",
		Description: "Detailed execution plan metrics",
		Query:       "SELECT latest(attributes.db.plan.node_type) as 'Node Type', latest(postgres.execution_plan.cost) as 'Cost' FROM Metric WHERE metricName LIKE 'postgres.execution_plan%' FACET attributes.db.postgresql.plan_id SINCE 1 hour ago LIMIT 10",
	},
	// Additional validation queries
	{
		Name:        "All PostgreSQL Metrics",
		Description: "List all available PostgreSQL metrics",
		Query:       "SELECT uniques(metricName) FROM Metric WHERE metricName LIKE 'postgres%' OR metricName LIKE 'postgresql%' SINCE 1 hour ago",
	},
	{
		Name:        "Metric Attributes",
		Description: "Show all attributes for slow queries",
		Query:       "SELECT keyset() FROM Metric WHERE metricName = 'postgres.slow_queries.elapsed_time' SINCE 1 hour ago LIMIT 1",
	},
}

type GraphQLRequest struct {
	Query string `json:"query"`
}

type NRDBResponse struct {
	Data struct {
		Actor struct {
			Account struct {
				NRQL struct {
					Results json.RawMessage `json:"results"`
				} `json:"nrql"`
			} `json:"account"`
		} `json:"actor"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func loadEnv() error {
	err := godotenv.Load()
	if err != nil {
		rootPath := filepath.Join("..", "..", "..", "..")
		envPath := filepath.Join(rootPath, ".env")
		err = godotenv.Load(envPath)
		if err != nil {
			return fmt.Errorf("could not load .env file: %w", err)
		}
	}
	return nil
}

func executeNRQL(apiKey, accountID, nrqlQuery string) (json.RawMessage, error) {
	escapedQuery := strings.ReplaceAll(nrqlQuery, `"`, `\"`)
	graphQLQuery := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "%s") {
					results
				}
			}
		}
	}`, accountID, escapedQuery)

	reqBody := GraphQLRequest{Query: graphQLQuery}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", nerdGraphEndpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("API-Key", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var nrdbResp NRDBResponse
	if err := json.Unmarshal(body, &nrdbResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(nrdbResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", nrdbResp.Errors)
	}

	return nrdbResp.Data.Actor.Account.NRQL.Results, nil
}

func main() {
	if err := loadEnv(); err != nil {
		log.Printf("Warning: %v", err)
	}

	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		log.Fatal("NEW_RELIC_API_KEY environment variable is required")
	}

	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	if accountID == "" {
		log.Fatal("NEW_RELIC_ACCOUNT_ID environment variable is required")
	}

	fmt.Println("🔍 Validating OpenTelemetry PostgreSQL Queries")
	fmt.Printf("Account ID: %s\n", accountID)
	fmt.Println(strings.Repeat("=", 80))

	successCount := 0
	failureCount := 0

	for i, test := range queries {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(queries), test.Name)
		fmt.Printf("Description: %s\n", test.Description)
		fmt.Printf("Query: %s\n", test.Query)
		
		results, err := executeNRQL(apiKey, accountID, test.Query)
		if err != nil {
			fmt.Printf("❌ FAILED: %v\n", err)
			failureCount++
			continue
		}

		// Parse results to check if we got data
		var resultData []map[string]interface{}
		if err := json.Unmarshal(results, &resultData); err != nil {
			fmt.Printf("❌ FAILED to parse results: %v\n", err)
			failureCount++
			continue
		}

		if len(resultData) == 0 {
			fmt.Printf("⚠️  WARNING: Query returned no results\n")
		} else {
			fmt.Printf("✅ SUCCESS: Query returned %d results\n", len(resultData))
			
			// Show first few results for validation
			if len(resultData) > 0 && len(resultData[0]) > 0 {
				fmt.Println("Sample result:")
				sample, _ := json.MarshalIndent(resultData[0], "  ", "  ")
				fmt.Printf("  %s\n", string(sample))
			}
		}
		successCount++
	}

	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("✅ Successful queries: %d\n", successCount)
	fmt.Printf("❌ Failed queries: %d\n", failureCount)
	fmt.Printf("Total queries tested: %d\n", len(queries))

	if failureCount > 0 {
		os.Exit(1)
	}
}