package framework

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

// TestEnvironment manages the test environment setup
type TestEnvironment struct {
	MySQLHost     string
	MySQLPort     int
	MySQLUser     string
	MySQLPassword string
	MySQLDatabase string
	
	OTelHost      string
	OTelPort      int
	
	logger        *zap.Logger
	db            *sql.DB
}

// NewTestEnvironment creates a new test environment
func NewTestEnvironment() *TestEnvironment {
	logger, _ := zap.NewDevelopment()
	
	return &TestEnvironment{
		MySQLHost:     getEnvOrDefault("MYSQL_HOST", "localhost"),
		MySQLPort:     getEnvOrDefaultInt("MYSQL_PORT", 3306),
		MySQLUser:     getEnvOrDefault("MYSQL_USER", "root"),
		MySQLPassword: getEnvOrDefault("MYSQL_PASSWORD", "rootpassword"),
		MySQLDatabase: getEnvOrDefault("MYSQL_DATABASE", "test"),
		
		OTelHost:      getEnvOrDefault("OTEL_HOST", "localhost"),
		OTelPort:      getEnvOrDefaultInt("OTEL_PORT", 4317),
		
		logger:        logger,
	}
}

// Setup initializes the test environment
func (e *TestEnvironment) Setup() error {
	// Connect to MySQL
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		e.MySQLUser, e.MySQLPassword, e.MySQLHost, e.MySQLPort, e.MySQLDatabase)
	
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}
	
	// Set connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping MySQL: %w", err)
	}
	
	e.db = db
	e.logger.Info("Test environment setup complete",
		zap.String("mysql_host", e.MySQLHost),
		zap.Int("mysql_port", e.MySQLPort))
	
	return nil
}

// Teardown cleans up the test environment
func (e *TestEnvironment) Teardown() {
	if e.db != nil {
		e.db.Close()
	}
	e.logger.Info("Test environment teardown complete")
}

// GetDB returns the database connection
func (e *TestEnvironment) GetDB() *sql.DB {
	return e.db
}

// CreateTestData creates test data in the database
func (e *TestEnvironment) CreateTestData() error {
	// Create test table
	query := `
	CREATE TABLE IF NOT EXISTS test_metrics (
		id INT PRIMARY KEY AUTO_INCREMENT,
		metric_name VARCHAR(100),
		metric_value FLOAT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	
	if _, err := e.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create test table: %w", err)
	}
	
	// Insert test data
	for i := 0; i < 100; i++ {
		_, err := e.db.Exec(
			"INSERT INTO test_metrics (metric_name, metric_value) VALUES (?, ?)",
			fmt.Sprintf("test_metric_%d", i%10),
			float64(i)*1.5,
		)
		if err != nil {
			return fmt.Errorf("failed to insert test data: %w", err)
		}
	}
	
	return nil
}

// CreateKnownWorkload generates a known workload for validation
func (e *TestEnvironment) CreateKnownWorkload(duration time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Execute various queries
			queries := []string{
				"SELECT COUNT(*) FROM test_metrics",
				"SELECT AVG(metric_value) FROM test_metrics",
				"SELECT * FROM test_metrics WHERE id = ?",
				"UPDATE test_metrics SET metric_value = metric_value + 1 WHERE id = ?",
			}
			
			for _, q := range queries {
				if q == queries[2] || q == queries[3] {
					// Queries with parameters
					_, err := e.db.Exec(q, 1+int(time.Now().Unix())%100)
					if err != nil {
						e.logger.Warn("Query failed", zap.Error(err))
					}
				} else {
					// Queries without parameters
					_, err := e.db.Query(q)
					if err != nil {
						e.logger.Warn("Query failed", zap.Error(err))
					}
				}
			}
		}
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}