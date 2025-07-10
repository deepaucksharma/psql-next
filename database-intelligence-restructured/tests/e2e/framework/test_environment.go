package framework

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/go-sql-driver/mysql"
)

// TestEnvironment represents the complete test environment
type TestEnvironment struct {
	// Database connections
	PostgresDB *sql.DB
	MySQLDB    *sql.DB
	
	// Configuration
	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDatabase string
	
	MySQLHost     string
	MySQLPort     int
	MySQLUser     string
	MySQLPassword string
	MySQLDatabase string
	MySQLEnabled  bool
	
	// New Relic configuration
	NewRelicAccountID string
	NewRelicAPIKey    string
	NewRelicLicenseKey string
	NewRelicEndpoint  string
	
	// Collector configuration
	CollectorEndpoint string
	MetricsEndpoint   string
	HealthEndpoint    string
	
	// Test data
	TestDataPath string
	TempDir      string
}

// NewTestEnvironment creates a new test environment from environment variables
func NewTestEnvironment() *TestEnvironment {
	env := &TestEnvironment{
		// PostgreSQL defaults
		PostgresHost:     getEnvOrDefault("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnvOrDefaultInt("POSTGRES_PORT", 5432),
		PostgresUser:     getEnvOrDefault("POSTGRES_USER", "postgres"),
		PostgresPassword: getEnvOrDefault("POSTGRES_PASSWORD", "postgres"),
		PostgresDatabase: getEnvOrDefault("POSTGRES_DB", "testdb"),
		
		// MySQL defaults
		MySQLHost:     getEnvOrDefault("MYSQL_HOST", "localhost"),
		MySQLPort:     getEnvOrDefaultInt("MYSQL_PORT", 3306),
		MySQLUser:     getEnvOrDefault("MYSQL_USER", "root"),
		MySQLPassword: getEnvOrDefault("MYSQL_PASSWORD", "root"),
		MySQLDatabase: getEnvOrDefault("MYSQL_DB", "testdb"),
		MySQLEnabled:  getEnvOrDefaultBool("MYSQL_ENABLED", true),
		
		// New Relic configuration
		NewRelicAccountID:  os.Getenv("NEW_RELIC_ACCOUNT_ID"),
		NewRelicAPIKey:     os.Getenv("NEW_RELIC_API_KEY"),
		NewRelicLicenseKey: os.Getenv("NEW_RELIC_LICENSE_KEY"),
		NewRelicEndpoint:   getEnvOrDefault("NEW_RELIC_OTLP_ENDPOINT", "otlp.nr-data.net:4317"),
		
		// Collector endpoints
		CollectorEndpoint: getEnvOrDefault("COLLECTOR_ENDPOINT", "localhost:4317"),
		MetricsEndpoint:   getEnvOrDefault("METRICS_ENDPOINT", "localhost:8889"),
		HealthEndpoint:    getEnvOrDefault("HEALTH_ENDPOINT", "localhost:13133"),
		
		// Test data
		TestDataPath: getEnvOrDefault("TEST_DATA_PATH", "./testdata"),
		TempDir:      getEnvOrDefault("TEST_TEMP_DIR", "/tmp/db-intelligence-e2e"),
	}
	
	return env
}

// Initialize sets up the test environment
func (env *TestEnvironment) Initialize() error {
	// Create temp directory
	if err := os.MkdirAll(env.TempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	
	// Connect to PostgreSQL
	pgDSN := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		env.PostgresHost, env.PostgresPort, env.PostgresUser, env.PostgresPassword, env.PostgresDatabase)
	
	var err error
	env.PostgresDB, err = sql.Open("postgres", pgDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	
	if err := env.PostgresDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}
	
	// Connect to MySQL if enabled
	if env.MySQLEnabled {
		mysqlDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			env.MySQLUser, env.MySQLPassword, env.MySQLHost, env.MySQLPort, env.MySQLDatabase)
		
		env.MySQLDB, err = sql.Open("mysql", mysqlDSN)
		if err != nil {
			return fmt.Errorf("failed to connect to MySQL: %w", err)
		}
		
		if err := env.MySQLDB.Ping(); err != nil {
			return fmt.Errorf("failed to ping MySQL: %w", err)
		}
	}
	
	return nil
}

// WaitForCollector waits for the collector to be ready
func (env *TestEnvironment) WaitForCollector(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := fmt.Sprintf("http://%s/", env.HealthEndpoint)
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for collector")
		case <-ticker.C:
			resp, err := client.Get(healthURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

// Cleanup cleans up the test environment
func (env *TestEnvironment) Cleanup() error {
	var errors []error
	
	if env.PostgresDB != nil {
		if err := env.PostgresDB.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close PostgreSQL: %w", err))
		}
	}
	
	if env.MySQLDB != nil {
		if err := env.MySQLDB.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close MySQL: %w", err))
		}
	}
	
	// Clean up temp directory
	if err := os.RemoveAll(env.TempDir); err != nil {
		errors = append(errors, fmt.Errorf("failed to remove temp dir: %w", err))
		}
	
	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	
	return nil
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvOrDefaultBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}