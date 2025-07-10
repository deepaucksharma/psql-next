package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/database-intelligence/tests/e2e/framework"
)

func main() {
	// Load environment
	fmt.Println("=== Testing Connectivity ===")
	fmt.Println()

	// Check environment variables
	fmt.Println("Environment Variables:")
	fmt.Printf("NEW_RELIC_ACCOUNT_ID: %s\n", maskValue(os.Getenv("NEW_RELIC_ACCOUNT_ID")))
	fmt.Printf("NEW_RELIC_LICENSE_KEY: %s\n", maskValue(os.Getenv("NEW_RELIC_LICENSE_KEY")))
	fmt.Printf("NEW_RELIC_API_KEY: %s\n", maskValue(os.Getenv("NEW_RELIC_API_KEY")))
	fmt.Printf("POSTGRES_HOST: %s\n", getEnvOrDefault("POSTGRES_HOST", "localhost"))
	fmt.Printf("POSTGRES_PORT: %s\n", getEnvOrDefault("POSTGRES_PORT", "5432"))
	fmt.Println()

	// Test PostgreSQL connection
	fmt.Println("Testing PostgreSQL Connection...")
	testPostgreSQL()
	
	// Test New Relic API connection
	fmt.Println("\nTesting New Relic API Connection...")
	testNewRelicAPI()
}

func testPostgreSQL() {
	host := getEnvOrDefault("POSTGRES_HOST", "localhost")
	port := getEnvOrDefault("POSTGRES_PORT", "5432")
	user := getEnvOrDefault("POSTGRES_USER", "postgres")
	password := getEnvOrDefault("POSTGRES_PASSWORD", "postgres")
	dbname := getEnvOrDefault("POSTGRES_DB", "testdb")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("❌ Failed to open PostgreSQL connection: %v", err)
		return
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Printf("❌ Failed to ping PostgreSQL: %v", err)
		return
	}

	// Run a simple query
	var version string
	err = db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		log.Printf("❌ Failed to query PostgreSQL: %v", err)
		return
	}

	fmt.Printf("✅ PostgreSQL connected successfully!\n")
	fmt.Printf("   Version: %.80s...\n", version)

	// Check if pg_stat_statements is available
	var hasStatStatements bool
	err = db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'
		)
	`).Scan(&hasStatStatements)

	if err == nil && hasStatStatements {
		fmt.Println("✅ pg_stat_statements extension is installed")
		
		// Check if we can query it
		var count int
		err = db.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_statements").Scan(&count)
		if err != nil {
			fmt.Printf("⚠️  pg_stat_statements installed but not accessible: %v\n", err)
		} else {
			fmt.Printf("   Found %d statements in pg_stat_statements\n", count)
		}
	} else {
		fmt.Println("⚠️  pg_stat_statements extension is NOT installed")
	}
}

func testNewRelicAPI() {
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")

	if accountID == "" || accountID == "YOUR_ACCOUNT_ID_HERE" {
		fmt.Println("❌ NEW_RELIC_ACCOUNT_ID not configured")
		return
	}

	if licenseKey == "" {
		fmt.Println("❌ NEW_RELIC_LICENSE_KEY not configured")
		return
	}

	fmt.Printf("✅ New Relic Account ID: %s\n", accountID)
	fmt.Printf("✅ New Relic License Key: %s\n", maskValue(licenseKey))

	if apiKey == "" || apiKey == "YOUR_API_KEY_HERE" {
		fmt.Println("⚠️  NEW_RELIC_API_KEY not configured - NRDB queries will not work")
		fmt.Println("   To get an API key:")
		fmt.Println("   1. Go to https://one.newrelic.com/api-keys")
		fmt.Println("   2. Create a new key with 'Query your data' permission")
		fmt.Println("   3. Set NEW_RELIC_API_KEY environment variable")
		return
	}

	// If we have an API key, try to create NRDB client
	nrdb := framework.NewNRDBClient(accountID, apiKey)
	if nrdb == nil {
		fmt.Println("❌ Failed to create NRDB client")
		return
	}

	// Try a simple query
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	query := "SELECT count(*) FROM Metric WHERE db.system = 'postgresql' SINCE 1 hour ago"
	result, err := nrdb.Query(ctx, query)
	if err != nil {
		fmt.Printf("❌ NRDB query failed: %v\n", err)
		fmt.Println("   This might be because:")
		fmt.Println("   - API key doesn't have query permissions")
		fmt.Println("   - No data has been sent yet")
		fmt.Println("   - Account ID mismatch")
		return
	}

	fmt.Printf("✅ NRDB query successful!\n")
	fmt.Printf("   Result: %v\n", result)
}

func maskValue(value string) string {
	if value == "" {
		return "(not set)"
	}
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}