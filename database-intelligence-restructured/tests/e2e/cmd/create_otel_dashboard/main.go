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
	"strconv"

	"github.com/joho/godotenv"
)

const nerdGraphEndpoint = "https://api.newrelic.com/graphql"

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type DashboardResponse struct {
	Data struct {
		DashboardCreate struct {
			EntityResult struct {
				GUID string `json:"guid"`
			} `json:"entityResult"`
			Errors []struct {
				Description string `json:"description"`
				Type        string `json:"type"`
			} `json:"errors"`
		} `json:"dashboardCreate"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func loadEnv() error {
	// Try to load .env from current directory
	err := godotenv.Load()
	if err != nil {
		// Try to load from project root
		rootPath := filepath.Join("..", "..", "..", "..")
		envPath := filepath.Join(rootPath, ".env")
		err = godotenv.Load(envPath)
		if err != nil {
			log.Printf("Warning: Could not load .env file: %v", err)
		}
	}
	return nil
}

func readGraphQLQuery() (string, error) {
	queryPath := filepath.Join("..", "..", "nerdgraph", "create_otel_dashboard.graphql")
	content, err := os.ReadFile(queryPath)
	if err != nil {
		return "", fmt.Errorf("failed to read GraphQL query: %w", err)
	}
	return string(content), nil
}

func createDashboard(apiKey string, accountID int, query string) (*DashboardResponse, error) {
	variables := map[string]interface{}{
		"accountId": accountID,
	}

	reqBody := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var dashboardResp DashboardResponse
	if err := json.Unmarshal(body, &dashboardResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &dashboardResp, nil
}

func main() {
	// Load environment variables
	if err := loadEnv(); err != nil {
		log.Printf("Warning: %v", err)
	}

	// Get API key and account ID
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	if apiKey == "" {
		log.Fatal("NEW_RELIC_API_KEY environment variable is required")
	}

	accountIDStr := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	if accountIDStr == "" {
		log.Fatal("NEW_RELIC_ACCOUNT_ID environment variable is required")
	}

	accountID, err := strconv.Atoi(accountIDStr)
	if err != nil {
		log.Fatalf("Invalid account ID: %v", err)
	}

	// Read GraphQL query
	query, err := readGraphQLQuery()
	if err != nil {
		log.Fatalf("Failed to read query: %v", err)
	}

	fmt.Println("Creating OpenTelemetry PostgreSQL Dashboard...")
	fmt.Printf("Account ID: %d\n", accountID)

	// Create dashboard
	resp, err := createDashboard(apiKey, accountID, query)
	if err != nil {
		log.Fatalf("Failed to create dashboard: %v", err)
	}

	// Check for GraphQL errors
	if len(resp.Errors) > 0 {
		log.Println("GraphQL errors:")
		for _, err := range resp.Errors {
			log.Printf("  - %s", err.Message)
		}
		os.Exit(1)
	}

	// Check for mutation errors
	if len(resp.Data.DashboardCreate.Errors) > 0 {
		log.Println("Dashboard creation errors:")
		for _, err := range resp.Data.DashboardCreate.Errors {
			log.Printf("  - %s: %s", err.Type, err.Description)
		}
		os.Exit(1)
	}

	// Success!
	guid := resp.Data.DashboardCreate.EntityResult.GUID
	if guid != "" {
		fmt.Println("\n✅ Dashboard created successfully!")
		fmt.Printf("Dashboard GUID: %s\n", guid)
		
		// Construct dashboard URL
		dashboardURL := fmt.Sprintf("https://one.newrelic.com/dashboards?account=%d&state=%s", accountID, guid)
		fmt.Printf("Dashboard URL: %s\n", dashboardURL)
	} else {
		log.Fatal("Dashboard was created but no GUID was returned")
	}
}