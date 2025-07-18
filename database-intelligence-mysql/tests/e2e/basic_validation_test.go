package e2e

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicConnectivity validates basic MySQL and New Relic connectivity
func TestBasicConnectivity(t *testing.T) {
	t.Run("MySQL_Connection", func(t *testing.T) {
		// Test MySQL connection with environment variables
		host := getEnvOrDefault("MYSQL_HOST", "localhost")
		port := getEnvOrDefault("MYSQL_PORT", "3306")
		user := getEnvOrDefault("MYSQL_USER", "root")
		password := getEnvOrDefault("MYSQL_PASSWORD", "rootpassword")
		database := getEnvOrDefault("MYSQL_DATABASE", "production")
		
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, database)
		db, err := sql.Open("mysql", dsn)
		require.NoError(t, err, "Failed to open MySQL connection")
		defer db.Close()
		
		err = db.Ping()
		require.NoError(t, err, "Failed to ping MySQL")
		
		// Check Performance Schema
		var psEnabled int
		err = db.QueryRow("SELECT @@performance_schema").Scan(&psEnabled)
		require.NoError(t, err)
		assert.Equal(t, 1, psEnabled, "Performance Schema should be enabled")
		
		t.Log("✓ MySQL connection successful")
	})
	
	t.Run("NewRelic_Credentials", func(t *testing.T) {
		// Validate New Relic credentials are set
		apiKey := os.Getenv("NEW_RELIC_API_KEY")
		accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
		licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
		
		assert.NotEmpty(t, apiKey, "NEW_RELIC_API_KEY should be set")
		assert.NotEmpty(t, accountID, "NEW_RELIC_ACCOUNT_ID should be set")
		assert.NotEmpty(t, licenseKey, "NEW_RELIC_LICENSE_KEY should be set")
		
		// Validate format
		assert.True(t, len(apiKey) > 20, "API key seems too short")
		assert.True(t, len(licenseKey) > 30, "License key seems too short")
		
		t.Logf("✓ New Relic credentials validated")
		t.Logf("  Account ID: %s", accountID)
		t.Logf("  License Key: %s...%s", licenseKey[:10], licenseKey[len(licenseKey)-4:])
	})
	
	t.Run("Generate_Simple_Workload", func(t *testing.T) {
		db := connectMySQL(t)
		defer db.Close()
		
		// Create a simple test table
		_, err := db.Exec(`CREATE TABLE IF NOT EXISTS e2e_test (
			id INT PRIMARY KEY AUTO_INCREMENT,
			data VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`)
		require.NoError(t, err)
		
		// Insert some test data
		for i := 0; i < 10; i++ {
			_, err = db.Exec("INSERT INTO e2e_test (data) VALUES (?)", 
				fmt.Sprintf("test_data_%d_%d", i, time.Now().Unix()))
			require.NoError(t, err)
		}
		
		// Run a query that should generate wait events
		rows, err := db.Query("SELECT * FROM e2e_test WHERE data LIKE ?", "%test%")
		require.NoError(t, err)
		defer rows.Close()
		
		count := 0
		for rows.Next() {
			count++
		}
		assert.GreaterOrEqual(t, count, 10, "Should have at least 10 rows")
		
		t.Log("✓ Test workload generated successfully")
		
		// Cleanup
		_, _ = db.Exec("DROP TABLE IF EXISTS e2e_test")
	})
}

// TestBasicNewRelicQuery validates basic NRQL query execution
func TestBasicNewRelicQuery(t *testing.T) {
	// Skip if credentials not set
	if os.Getenv("NEW_RELIC_API_KEY") == "" {
		t.Skip("NEW_RELIC_API_KEY not set")
	}
	
	nrClient := NewNewRelicClient(t)
	
	// Simple query to validate connectivity
	nrql := `SELECT count(*) as total FROM Metric SINCE 1 minute ago`
	
	results, err := nrClient.QueryNRQL(nrql)
	require.NoError(t, err, "Failed to execute NRQL query")
	
	assert.NotNil(t, results, "Results should not be nil")
	t.Logf("✓ New Relic query executed successfully")
	
	if len(results) > 0 {
		t.Logf("  Result: %+v", results[0])
	}
}

// Helper function to get environment variable with default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}