package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/go-sql-driver/mysql"
	"github.com/newrelic/newrelic-client-go/newrelic"
	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// E2ETestConfig holds end-to-end test configuration
type E2ETestConfig struct {
	// Database connections
	PostgresURL string
	MySQLURL    string
	
	// Collector endpoint
	CollectorHealthEndpoint string
	CollectorMetricsEndpoint string
	
	// New Relic configuration
	NewRelicAPIKey  string
	NewRelicAccount int
	
	// Test parameters
	TestDuration    time.Duration
	VerifyInterval  time.Duration
	MetricTolerance float64
}

// TestE2EPostgreSQLToNRDB tests the complete flow from PostgreSQL to New Relic
func TestE2EPostgreSQLToNRDB(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := loadE2EConfig(t)
	
	// Phase 1: Setup and verify collector is running
	t.Run("VerifyCollectorHealth", func(t *testing.T) {
		verifyCollectorHealth(t, config)
	})
	
	// Phase 2: Generate database activity
	t.Run("GeneratePostgreSQLActivity", func(t *testing.T) {
		generatePostgreSQLActivity(t, config)
	})
	
	// Phase 3: Wait for metrics to flow through pipeline
	time.Sleep(30 * time.Second) // Allow time for batching and export
	
	// Phase 4: Verify metrics in NRDB
	t.Run("VerifyPostgreSQLMetricsInNRDB", func(t *testing.T) {
		verifyPostgreSQLMetricsInNRDB(t, config)
	})
	
	// Phase 5: Verify query performance metrics
	t.Run("VerifyQueryPerformanceMetrics", func(t *testing.T) {
		verifyQueryPerformanceMetrics(t, config)
	})
	
	// Phase 6: Verify OHI compatibility
	t.Run("VerifyOHICompatibility", func(t *testing.T) {
		verifyOHICompatibility(t, config)
	})
	
	// Phase 7: Verify feature detection
	t.Run("VerifyFeatureDetection", func(t *testing.T) {
		verifyPostgreSQLFeatureDetection(t, config)
	})
	
	// Phase 8: Verify graceful fallback
	t.Run("VerifyGracefulFallback", func(t *testing.T) {
		verifyGracefulFallback(t, config)
	})
}

// TestE2EMySQLToNRDB tests the complete flow from MySQL to New Relic
func TestE2EMySQLToNRDB(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := loadE2EConfig(t)
	
	// Phase 1: Setup and verify collector is running
	t.Run("VerifyCollectorHealth", func(t *testing.T) {
		verifyCollectorHealth(t, config)
	})
	
	// Phase 2: Generate database activity
	t.Run("GenerateMySQLActivity", func(t *testing.T) {
		generateMySQLActivity(t, config)
	})
	
	// Phase 3: Wait for metrics to flow
	time.Sleep(30 * time.Second)
	
	// Phase 4: Verify metrics in NRDB
	t.Run("VerifyMySQLMetricsInNRDB", func(t *testing.T) {
		verifyMySQLMetricsInNRDB(t, config)
	})
	
	// Phase 5: Verify InnoDB metrics
	t.Run("VerifyInnoDBMetrics", func(t *testing.T) {
		verifyInnoDBMetrics(t, config)
	})
	
	// Phase 6: Verify MySQL feature detection
	t.Run("VerifyMySQLFeatureDetection", func(t *testing.T) {
		verifyMySQLFeatureDetection(t, config)
	})
}

// loadE2EConfig loads test configuration from environment
func loadE2EConfig(t *testing.T) *E2ETestConfig {
	return &E2ETestConfig{
		PostgresURL:              getEnvOrDefault("POSTGRES_URL", "postgres://postgres:password@localhost:5432/testdb?sslmode=disable"),
		MySQLURL:                 getEnvOrDefault("MYSQL_URL", "root:password@tcp(localhost:3306)/testdb"),
		CollectorHealthEndpoint:  getEnvOrDefault("COLLECTOR_HEALTH_ENDPOINT", "http://localhost:13133/health"),
		CollectorMetricsEndpoint: getEnvOrDefault("COLLECTOR_METRICS_ENDPOINT", "http://localhost:8888/metrics"),
		NewRelicAPIKey:          os.Getenv("NEW_RELIC_API_KEY"),
		NewRelicAccount:         getEnvAsInt("NEW_RELIC_ACCOUNT_ID", 0),
		TestDuration:            5 * time.Minute,
		VerifyInterval:          30 * time.Second,
		MetricTolerance:         0.05, // 5% tolerance
	}
}

// verifyCollectorHealth checks if the collector is running and healthy
func verifyCollectorHealth(t *testing.T, config *E2ETestConfig) {
	resp, err := http.Get(config.CollectorHealthEndpoint)
	require.NoError(t, err, "Failed to reach collector health endpoint")
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read health response")
	
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Collector is not healthy: %s", string(body))
	
	// Verify collector metrics endpoint
	metricsResp, err := http.Get(config.CollectorMetricsEndpoint)
	require.NoError(t, err, "Failed to reach collector metrics endpoint")
	defer metricsResp.Body.Close()
	
	assert.Equal(t, http.StatusOK, metricsResp.StatusCode, "Collector metrics endpoint not available")
}

// generatePostgreSQLActivity creates database activity for testing
func generatePostgreSQLActivity(t *testing.T, config *E2ETestConfig) {
	db, err := sql.Open("postgres", config.PostgresURL)
	require.NoError(t, err, "Failed to connect to PostgreSQL")
	defer db.Close()
	
	// Create test table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test (
			id SERIAL PRIMARY KEY,
			data TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err, "Failed to create test table")
	
	// Generate various query patterns
	ctx := context.Background()
	
	// INSERT queries
	for i := 0; i < 100; i++ {
		_, err = db.ExecContext(ctx, 
			"INSERT INTO e2e_test (data) VALUES ($1)",
			fmt.Sprintf("test_data_%d", i))
		assert.NoError(t, err, "Failed to insert test data")
	}
	
	// SELECT queries (some slow)
	for i := 0; i < 50; i++ {
		var count int
		err = db.QueryRowContext(ctx, 
			"SELECT COUNT(*) FROM e2e_test WHERE data LIKE $1",
			fmt.Sprintf("%%test_data_%d%%", i)).Scan(&count)
		assert.NoError(t, err, "Failed to execute SELECT query")
	}
	
	// UPDATE queries
	for i := 0; i < 30; i++ {
		_, err = db.ExecContext(ctx,
			"UPDATE e2e_test SET data = $1 WHERE id = $2",
			fmt.Sprintf("updated_data_%d", i), i)
		assert.NoError(t, err, "Failed to execute UPDATE query")
	}
	
	// Complex JOIN query (intentionally slow)
	_, err = db.ExecContext(ctx, `
		SELECT t1.*, t2.data as related_data
		FROM e2e_test t1
		CROSS JOIN e2e_test t2
		WHERE t1.id != t2.id
		LIMIT 10
	`)
	assert.NoError(t, err, "Failed to execute complex query")
	
	// Generate connection churn
	for i := 0; i < 10; i++ {
		conn, err := db.Conn(ctx)
		if err == nil {
			time.Sleep(100 * time.Millisecond)
			conn.Close()
		}
	}
	
	t.Logf("Generated PostgreSQL test activity: 100 INSERTs, 50 SELECTs, 30 UPDATEs")
}

// generateMySQLActivity creates MySQL database activity for testing
func generateMySQLActivity(t *testing.T, config *E2ETestConfig) {
	db, err := sql.Open("mysql", config.MySQLURL)
	require.NoError(t, err, "Failed to connect to MySQL")
	defer db.Close()
	
	// Create test table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS e2e_test (
			id INT AUTO_INCREMENT PRIMARY KEY,
			data VARCHAR(255),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err, "Failed to create test table")
	
	ctx := context.Background()
	
	// Generate InnoDB activity
	for i := 0; i < 100; i++ {
		_, err = db.ExecContext(ctx,
			"INSERT INTO e2e_test (data) VALUES (?)",
			fmt.Sprintf("mysql_test_%d", i))
		assert.NoError(t, err, "Failed to insert MySQL test data")
	}
	
	// Query cache activity
	for i := 0; i < 50; i++ {
		var data string
		err = db.QueryRowContext(ctx,
			"SELECT data FROM e2e_test WHERE id = ?", i%10).Scan(&data)
		if err != nil && err != sql.ErrNoRows {
			assert.NoError(t, err, "Failed to execute cached query")
		}
	}
	
	t.Logf("Generated MySQL test activity: 100 INSERTs, 50 cached SELECTs")
}

// verifyPostgreSQLMetricsInNRDB checks if PostgreSQL metrics appear in New Relic
func verifyPostgreSQLMetricsInNRDB(t *testing.T, config *E2ETestConfig) {
	if config.NewRelicAPIKey == "" {
		t.Skip("NEW_RELIC_API_KEY not set, skipping NRDB verification")
	}
	
	nrClient, err := newrelic.New(
		newrelic.ConfigPersonalAPIKey(config.NewRelicAPIKey),
		newrelic.ConfigRegion("US"),
	)
	require.NoError(t, err, "Failed to create New Relic client")
	
	// Core metrics to verify
	metricsToCheck := []string{
		"db.connections.active",
		"db.commitsPerSecond",
		"db.rollbacksPerSecond",
		"db.reads.blocksPerSecond",
		"db.writes.blocksPerSecond",
	}
	
	for _, metricName := range metricsToCheck {
		t.Run(metricName, func(t *testing.T) {
			query := fmt.Sprintf(`
				SELECT average(%s) 
				FROM Metric 
				WHERE db.system = 'postgresql'
				SINCE 5 minutes ago
			`, metricName)
			
			result, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(query))
			require.NoError(t, err, "Failed to query NRDB for metric %s", metricName)
			
			assert.NotEmpty(t, result.Results, "No data found for metric %s", metricName)
			
			if len(result.Results) > 0 {
				value, ok := result.Results[0]["average"].(float64)
				assert.True(t, ok, "Invalid metric value type for %s", metricName)
				assert.GreaterOrEqual(t, value, 0.0, "Metric %s has invalid value", metricName)
				t.Logf("Metric %s = %.2f", metricName, value)
			}
		})
	}
}

// verifyQueryPerformanceMetrics checks if query performance metrics are captured
func verifyQueryPerformanceMetrics(t *testing.T, config *E2ETestConfig) {
	if config.NewRelicAPIKey == "" {
		t.Skip("NEW_RELIC_API_KEY not set, skipping NRDB verification")
	}
	
	nrClient, err := newrelic.New(
		newrelic.ConfigPersonalAPIKey(config.NewRelicAPIKey),
		newrelic.ConfigRegion("US"),
	)
	require.NoError(t, err, "Failed to create New Relic client")
	
	// Check for query metrics
	query := `
		SELECT 
			sum(db.query.count) as total_queries,
			average(db.query.mean_duration) as avg_duration,
			uniqueCount(statement_type) as query_types
		FROM Metric 
		WHERE db.system = 'postgresql'
		AND telemetry.source = 'otel'
		SINCE 5 minutes ago
	`
	
	result, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(query))
	require.NoError(t, err, "Failed to query query performance metrics")
	
	if len(result.Results) > 0 {
		totalQueries, _ := result.Results[0]["total_queries"].(float64)
		avgDuration, _ := result.Results[0]["avg_duration"].(float64)
		queryTypes, _ := result.Results[0]["query_types"].(float64)
		
		assert.Greater(t, totalQueries, 0.0, "No queries captured")
		assert.Greater(t, avgDuration, 0.0, "No query duration captured")
		assert.GreaterOrEqual(t, queryTypes, 3.0, "Expected at least 3 query types (INSERT, SELECT, UPDATE)")
		
		t.Logf("Query metrics: total=%v, avg_duration=%vms, types=%v", 
			totalQueries, avgDuration, queryTypes)
	}
	
	// Verify query correlation attributes
	correlationQuery := `
		SELECT 
			uniqueCount(correlation.query_id) as correlated_queries,
			uniqueCount(correlation.table) as tables_tracked
		FROM Metric 
		WHERE db.system = 'postgresql'
		AND correlation.query_id IS NOT NULL
		SINCE 5 minutes ago
	`
	
	correlationResult, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(correlationQuery))
	require.NoError(t, err, "Failed to query correlation metrics")
	
	if len(correlationResult.Results) > 0 {
		correlatedQueries, _ := correlationResult.Results[0]["correlated_queries"].(float64)
		assert.Greater(t, correlatedQueries, 0.0, "No query correlations found")
		t.Logf("Found %v correlated queries", correlatedQueries)
	}
}

// verifyOHICompatibility checks if OHI-compatible metrics are present
func verifyOHICompatibility(t *testing.T, config *E2ETestConfig) {
	if config.NewRelicAPIKey == "" {
		t.Skip("NEW_RELIC_API_KEY not set, skipping NRDB verification")
	}
	
	nrClient, err := newrelic.New(
		newrelic.ConfigPersonalAPIKey(config.NewRelicAPIKey),
		newrelic.ConfigRegion("US"),
	)
	require.NoError(t, err, "Failed to create New Relic client")
	
	// Check for OHI entity attributes
	entityQuery := `
		SELECT 
			uniqueCount(entity.name) as entities,
			uniqueCount(entity.type) as entity_types,
			latest(integration.name) as integration_name,
			latest(integration.version) as integration_version
		FROM Metric 
		WHERE db.system = 'postgresql'
		AND entity.name IS NOT NULL
		SINCE 5 minutes ago
	`
	
	result, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(entityQuery))
	require.NoError(t, err, "Failed to query OHI compatibility attributes")
	
	if len(result.Results) > 0 {
		entities, _ := result.Results[0]["entities"].(float64)
		integrationName, _ := result.Results[0]["integration_name"].(string)
		
		assert.Greater(t, entities, 0.0, "No entities found")
		assert.Contains(t, integrationName, "otel", "Integration name should indicate OTEL")
		
		t.Logf("OHI compatibility: %v entities, integration=%s", entities, integrationName)
	}
}

// verifyMySQLMetricsInNRDB checks if MySQL metrics appear in New Relic
func verifyMySQLMetricsInNRDB(t *testing.T, config *E2ETestConfig) {
	if config.NewRelicAPIKey == "" {
		t.Skip("NEW_RELIC_API_KEY not set, skipping NRDB verification")
	}
	
	nrClient, err := newrelic.New(
		newrelic.ConfigPersonalAPIKey(config.NewRelicAPIKey),
		newrelic.ConfigRegion("US"),
	)
	require.NoError(t, err, "Failed to create New Relic client")
	
	// Check MySQL-specific metrics
	query := `
		SELECT 
			latest(db.connections.active) as connections,
			latest(db.handler.writePerSecond) as writes,
			latest(db.queryCacheHitsPerSecond) as cache_hits
		FROM Metric 
		WHERE db.system = 'mysql'
		SINCE 5 minutes ago
	`
	
	result, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(query))
	require.NoError(t, err, "Failed to query MySQL metrics")
	
	assert.NotEmpty(t, result.Results, "No MySQL metrics found in NRDB")
}

// verifyInnoDBMetrics checks if InnoDB-specific metrics are captured
func verifyInnoDBMetrics(t *testing.T, config *E2ETestConfig) {
	if config.NewRelicAPIKey == "" {
		t.Skip("NEW_RELIC_API_KEY not set, skipping NRDB verification")
	}
	
	nrClient, err := newrelic.New(
		newrelic.ConfigPersonalAPIKey(config.NewRelicAPIKey),
		newrelic.ConfigRegion("US"),
	)
	require.NoError(t, err, "Failed to create New Relic client")
	
	// Check InnoDB metrics
	query := `
		SELECT 
			latest(db.innodb.bufferPoolDataPages) as data_pages,
			latest(db.innodb.bufferPoolPagesFlushedPerSecond) as page_flushes
		FROM Metric 
		WHERE db.system = 'mysql'
		SINCE 5 minutes ago
	`
	
	result, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(query))
	require.NoError(t, err, "Failed to query InnoDB metrics")
	
	if len(result.Results) > 0 {
		dataPages, _ := result.Results[0]["data_pages"].(float64)
		assert.Greater(t, dataPages, 0.0, "No InnoDB data pages metric found")
		t.Logf("InnoDB metrics verified: data_pages=%v", dataPages)
	}
}

// TestE2EProcessorChain verifies the processor chain is working correctly
func TestE2EProcessorChain(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}
	
	config := loadE2EConfig(t)
	
	// Check collector's own metrics to verify processors
	resp, err := http.Get(config.CollectorMetricsEndpoint)
	require.NoError(t, err, "Failed to get collector metrics")
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read metrics")
	
	metrics := string(body)
	
	// Verify each processor is active
	processors := []string{
		"processor_batch_batch_send_size",
		"processor_querycorrelator_correlations_created",
		"processor_adaptivesampler_traces_sampled",
		"processor_circuitbreaker_circuit_state",
		"processor_verification_pii_detections",
		"processor_costcontrol_data_reduced",
		"processor_nrerrormonitor_errors_detected",
	}
	
	for _, processor := range processors {
		assert.Contains(t, metrics, processor, "Processor %s metrics not found", processor)
	}
	
	t.Logf("All processors verified in metrics pipeline")
}

// verifyPostgreSQLFeatureDetection checks if feature detection is working
func verifyPostgreSQLFeatureDetection(t *testing.T, config *E2ETestConfig) {
	if config.NewRelicAPIKey == "" {
		t.Skip("NEW_RELIC_API_KEY not set, skipping NRDB verification")
	}
	
	nrClient, err := newrelic.New(
		newrelic.ConfigPersonalAPIKey(config.NewRelicAPIKey),
		newrelic.ConfigRegion("US"),
	)
	require.NoError(t, err, "Failed to create New Relic client")
	
	// Check feature detection metrics
	query := `
		SELECT 
			uniqueCount(extension) as extensions_detected,
			uniqueCount(capability) as capabilities_detected,
			latest(cloud_provider) as cloud_provider
		FROM Metric 
		WHERE metricName = 'db.feature.extension.available'
		   OR metricName = 'db.feature.capability.available'
		SINCE 5 minutes ago
	`
	
	result, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(query))
	require.NoError(t, err, "Failed to query feature detection metrics")
	
	if len(result.Results) > 0 {
		extensions, _ := result.Results[0]["extensions_detected"].(float64)
		capabilities, _ := result.Results[0]["capabilities_detected"].(float64)
		cloudProvider, _ := result.Results[0]["cloud_provider"].(string)
		
		assert.Greater(t, extensions, 0.0, "No extensions detected")
		assert.Greater(t, capabilities, 0.0, "No capabilities detected")
		
		t.Logf("Feature detection: %v extensions, %v capabilities, cloud=%s", 
			extensions, capabilities, cloudProvider)
	}
	
	// Check specific extension availability
	extensionQuery := `
		SELECT 
			latest(value) as available,
			latest(version) as version
		FROM Metric 
		WHERE metricName = 'db.feature.extension.available'
		  AND extension = 'pg_stat_statements'
		SINCE 5 minutes ago
	`
	
	extResult, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(extensionQuery))
	require.NoError(t, err, "Failed to query pg_stat_statements availability")
	
	if len(extResult.Results) > 0 {
		available, _ := extResult.Results[0]["available"].(float64)
		version, _ := extResult.Results[0]["version"].(string)
		
		if available > 0 {
			t.Logf("pg_stat_statements is available (version %s)", version)
		} else {
			t.Logf("pg_stat_statements is NOT available")
		}
	}
}

// verifyMySQLFeatureDetection checks MySQL feature detection
func verifyMySQLFeatureDetection(t *testing.T, config *E2ETestConfig) {
	if config.NewRelicAPIKey == "" {
		t.Skip("NEW_RELIC_API_KEY not set, skipping NRDB verification")
	}
	
	nrClient, err := newrelic.New(
		newrelic.ConfigPersonalAPIKey(config.NewRelicAPIKey),
		newrelic.ConfigRegion("US"),
	)
	require.NoError(t, err, "Failed to create New Relic client")
	
	// Check MySQL feature detection
	query := `
		SELECT 
			latest(value) as performance_schema_enabled,
			uniqueCount(capability) as mysql_capabilities
		FROM Metric 
		WHERE db.system = 'mysql'
		  AND (metricName = 'db.feature.capability.available' 
		       AND capability = 'performance_schema')
		SINCE 5 minutes ago
	`
	
	result, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(query))
	require.NoError(t, err, "Failed to query MySQL feature detection")
	
	if len(result.Results) > 0 {
		perfSchema, _ := result.Results[0]["performance_schema_enabled"].(float64)
		capabilities, _ := result.Results[0]["mysql_capabilities"].(float64)
		
		t.Logf("MySQL features: performance_schema=%v, capabilities=%v", 
			perfSchema > 0, capabilities)
	}
}

// verifyGracefulFallback tests that queries fall back gracefully when features are missing
func verifyGracefulFallback(t *testing.T, config *E2ETestConfig) {
	// Check collector metrics for fallback usage
	resp, err := http.Get(config.CollectorMetricsEndpoint)
	require.NoError(t, err, "Failed to get collector metrics")
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read metrics")
	
	metrics := string(body)
	
	// Check for fallback metrics
	if strings.Contains(metrics, "enhancedsql_fallback_count") {
		t.Logf("Fallback queries are being used")
	}
	
	// Check for disabled queries due to missing features
	if strings.Contains(metrics, "circuitbreaker_disabled_queries") {
		t.Logf("Some queries have been disabled due to missing features")
	}
	
	// Verify data is still being collected despite missing features
	if config.NewRelicAPIKey != "" {
		nrClient, err := newrelic.New(
			newrelic.ConfigPersonalAPIKey(config.NewRelicAPIKey),
			newrelic.ConfigRegion("US"),
		)
		require.NoError(t, err, "Failed to create New Relic client")
		
		// Check that basic metrics are still being collected
		query := `
			SELECT count(*) as metric_count
			FROM Metric 
			WHERE db.system IN ('postgresql', 'mysql')
			SINCE 5 minutes ago
		`
		
		result, err := nrClient.Nrdb.Query(config.NewRelicAccount, nrdb.NRQL(query))
		require.NoError(t, err, "Failed to query metrics")
		
		if len(result.Results) > 0 {
			metricCount, _ := result.Results[0]["metric_count"].(float64)
			assert.Greater(t, metricCount, 0.0, "No metrics collected despite fallback")
			t.Logf("Collected %v metrics with graceful fallback", metricCount)
		}
	}
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		fmt.Sscanf(value, "%d", &intVal)
		return intVal
	}
	return defaultValue
}