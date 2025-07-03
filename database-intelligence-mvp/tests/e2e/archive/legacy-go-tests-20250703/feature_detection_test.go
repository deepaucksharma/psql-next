package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2EFeatureDetectionPostgreSQL tests feature detection for PostgreSQL
func TestE2EFeatureDetectionPostgreSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := loadE2EConfig(t)
	db, err := sql.Open("postgres", config.PostgresURL)
	require.NoError(t, err, "Failed to connect to PostgreSQL")
	defer db.Close()

	ctx := context.Background()

	// Test 1: Detect installed extensions
	t.Run("DetectInstalledExtensions", func(t *testing.T) {
		var extensions []string
		rows, err := db.QueryContext(ctx, `
			SELECT extname 
			FROM pg_extension 
			WHERE extname IN ('pg_stat_statements', 'pg_stat_monitor', 'pg_wait_sampling', 'auto_explain')
		`)
		require.NoError(t, err, "Failed to query extensions")
		defer rows.Close()

		for rows.Next() {
			var ext string
			err := rows.Scan(&ext)
			require.NoError(t, err)
			extensions = append(extensions, ext)
		}

		t.Logf("Detected PostgreSQL extensions: %v", extensions)
		
		// At minimum, we expect no errors even if no extensions are installed
		assert.NotNil(t, extensions, "Extension detection should not fail")
	})

	// Test 2: Detect capabilities
	t.Run("DetectCapabilities", func(t *testing.T) {
		capabilities := make(map[string]string)
		
		// Check track_io_timing
		var trackIOTiming string
		err := db.QueryRowContext(ctx, "SHOW track_io_timing").Scan(&trackIOTiming)
		if err == nil {
			capabilities["track_io_timing"] = trackIOTiming
		}

		// Check shared_preload_libraries
		var sharedPreload string
		err = db.QueryRowContext(ctx, "SHOW shared_preload_libraries").Scan(&sharedPreload)
		if err == nil {
			capabilities["shared_preload_libraries"] = sharedPreload
		}

		t.Logf("PostgreSQL capabilities: %v", capabilities)
		assert.NotEmpty(t, capabilities, "Should detect at least one capability")
	})

	// Test 3: Detect cloud provider
	t.Run("DetectCloudProvider", func(t *testing.T) {
		cloudProvider := detectPostgreSQLCloudProvider(ctx, db)
		t.Logf("Detected cloud provider: %s", cloudProvider)
		// No assertion - could be "none" for local instances
	})

	// Test 4: Test query fallback with missing extension
	t.Run("TestQueryFallback", func(t *testing.T) {
		// First check if pg_stat_statements exists
		var hasStatStatements bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'
			)
		`).Scan(&hasStatStatements)
		require.NoError(t, err)

		// Try to query with fallback logic
		query := `
			WITH feature_check AS (
				SELECT EXISTS (
					SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'
				) as has_extension
			)
			SELECT 
				CASE 
					WHEN fc.has_extension THEN 'pg_stat_statements'
					ELSE 'pg_stat_activity'
				END as source,
				COUNT(*) as query_count
			FROM feature_check fc
			LEFT JOIN pg_stat_activity psa ON NOT fc.has_extension
			GROUP BY source
		`

		var source string
		var count int
		err = db.QueryRowContext(ctx, query).Scan(&source, &count)
		require.NoError(t, err, "Fallback query should work")

		t.Logf("Query source: %s, count: %d", source, count)
		assert.NotEmpty(t, source, "Should have a valid query source")
	})
}

// TestE2EFeatureDetectionMySQL tests feature detection for MySQL
func TestE2EFeatureDetectionMySQL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := loadE2EConfig(t)
	db, err := sql.Open("mysql", config.MySQLURL)
	require.NoError(t, err, "Failed to connect to MySQL")
	defer db.Close()

	ctx := context.Background()

	// Test 1: Detect Performance Schema
	t.Run("DetectPerformanceSchema", func(t *testing.T) {
		var perfSchemaEnabled string
		err := db.QueryRowContext(ctx, "SELECT @@performance_schema").Scan(&perfSchemaEnabled)
		
		if err != nil {
			t.Logf("Could not query performance_schema: %v", err)
		} else {
			t.Logf("Performance Schema enabled: %s", perfSchemaEnabled)
			assert.Contains(t, []string{"0", "1", "ON", "OFF"}, perfSchemaEnabled)
		}
	})

	// Test 2: Detect MySQL capabilities
	t.Run("DetectCapabilities", func(t *testing.T) {
		capabilities := make(map[string]string)

		// Check slow query log
		var slowQueryLog string
		err := db.QueryRowContext(ctx, "SELECT @@slow_query_log").Scan(&slowQueryLog)
		if err == nil {
			capabilities["slow_query_log"] = slowQueryLog
		}

		// Check query cache
		var queryCacheType string
		err = db.QueryRowContext(ctx, "SELECT @@query_cache_type").Scan(&queryCacheType)
		if err == nil {
			capabilities["query_cache_type"] = queryCacheType
		}

		t.Logf("MySQL capabilities: %v", capabilities)
		assert.NotEmpty(t, capabilities, "Should detect at least one capability")
	})

	// Test 3: Test Performance Schema fallback
	t.Run("TestPerformanceSchemaFallback", func(t *testing.T) {
		// Try Performance Schema first, fall back to PROCESSLIST
		query := `
			SELECT 
				CASE 
					WHEN @@performance_schema = 1 THEN 'performance_schema'
					ELSE 'processlist'
				END as source,
				COUNT(*) as active_queries
			FROM information_schema.PROCESSLIST
			WHERE COMMAND != 'Sleep'
		`

		var source string
		var count int
		err := db.QueryRowContext(ctx, query).Scan(&source, &count)
		
		if err != nil {
			// Even simpler fallback
			err = db.QueryRowContext(ctx, "SELECT 'processlist', COUNT(*) FROM information_schema.PROCESSLIST").Scan(&source, &count)
		}
		
		require.NoError(t, err, "Fallback query should work")
		t.Logf("Query source: %s, active queries: %d", source, count)
	})
}

// TestE2EGracefulDegradation tests that the collector handles missing features gracefully
func TestE2EGracefulDegradation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := loadE2EConfig(t)

	// Test with minimal PostgreSQL (no extensions)
	t.Run("PostgreSQLMinimal", func(t *testing.T) {
		db, err := sql.Open("postgres", config.PostgresURL)
		require.NoError(t, err)
		defer db.Close()

		ctx := context.Background()

		// Simulate queries that would fail without extensions
		queries := []struct {
			name  string
			query string
		}{
			{
				name: "pg_stat_statements_missing",
				query: `
					SELECT COUNT(*) 
					FROM information_schema.tables 
					WHERE table_name = 'pg_stat_statements'
				`,
			},
			{
				name: "basic_activity_always_works",
				query: `
					SELECT COUNT(*) 
					FROM pg_stat_activity 
					WHERE state = 'active'
				`,
			},
		}

		for _, q := range queries {
			t.Run(q.name, func(t *testing.T) {
				var count int
				err := db.QueryRowContext(ctx, q.query).Scan(&count)
				// Should not error even if table doesn't exist
				if err != nil {
					t.Logf("Query %s returned error (expected for missing features): %v", q.name, err)
				} else {
					t.Logf("Query %s returned count: %d", q.name, count)
				}
			})
		}
	})

	// Test circuit breaker behavior
	t.Run("CircuitBreakerProtection", func(t *testing.T) {
		// Wait for collector to be running
		time.Sleep(5 * time.Second)

		// Check collector health - should be healthy even with missing features
		resp, err := http.Get(config.CollectorHealthEndpoint)
		require.NoError(t, err, "Collector should be reachable")
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Collector should be healthy despite missing features")

		// Check metrics endpoint
		metricsResp, err := http.Get(config.CollectorMetricsEndpoint)
		require.NoError(t, err, "Metrics endpoint should be reachable")
		defer metricsResp.Body.Close()

		body, err := io.ReadAll(metricsResp.Body)
		require.NoError(t, err)

		metrics := string(body)
		
		// Look for signs of graceful degradation
		if strings.Contains(metrics, "circuitbreaker_state") {
			t.Log("Circuit breaker is active and protecting the system")
		}
		if strings.Contains(metrics, "disabled_queries") {
			t.Log("Some queries have been disabled due to errors")
		}
		if strings.Contains(metrics, "fallback_count") {
			t.Log("Fallback queries are being used")
		}
	})
}

// Helper function to detect PostgreSQL cloud provider
func detectPostgreSQLCloudProvider(ctx context.Context, db *sql.DB) string {
	// Check for AWS RDS
	var rdsValue string
	err := db.QueryRowContext(ctx, `
		SELECT setting 
		FROM pg_settings 
		WHERE name = 'rds.superuser_reserved_connections'
		LIMIT 1
	`).Scan(&rdsValue)
	if err == nil && rdsValue != "" {
		// Check if it's Aurora
		var auroraVersion string
		err = db.QueryRowContext(ctx, "SELECT aurora_version()").Scan(&auroraVersion)
		if err == nil && auroraVersion != "" {
			return "aws_aurora"
		}
		return "aws_rds"
	}

	// Check for Google Cloud SQL
	var gcpValue string
	err = db.QueryRowContext(ctx, "SHOW cloudsql.iam_authentication").Scan(&gcpValue)
	if err == nil {
		return "gcp_cloudsql"
	}

	// Check for Azure
	var azureValue string
	err = db.QueryRowContext(ctx, `
		SELECT setting 
		FROM pg_settings 
		WHERE name LIKE 'azure.%'
		LIMIT 1
	`).Scan(&azureValue)
	if err == nil && azureValue != "" {
		return "azure_database"
	}

	return "none"
}

// TestE2EFeatureMetricsFlow verifies that feature detection metrics flow to NRDB
func TestE2EFeatureMetricsFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := loadE2EConfig(t)
	
	// Ensure databases have some features to detect
	t.Run("SetupFeatures", func(t *testing.T) {
		db, err := sql.Open("postgres", config.PostgresURL)
		require.NoError(t, err)
		defer db.Close()

		// Try to create pg_stat_statements extension if we have permissions
		_, err = db.Exec("CREATE EXTENSION IF NOT EXISTS pg_stat_statements")
		if err != nil {
			t.Logf("Could not create pg_stat_statements (may lack permissions): %v", err)
		}

		// Set track_io_timing if possible
		_, err = db.Exec("SET track_io_timing = on")
		if err != nil {
			t.Logf("Could not set track_io_timing: %v", err)
		}
	})

	// Wait for feature detection to run
	time.Sleep(10 * time.Second)

	// Verify metrics are being collected
	t.Run("VerifyCollectorMetrics", func(t *testing.T) {
		resp, err := http.Get(config.CollectorMetricsEndpoint)
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		metrics := string(body)

		// Check for feature detection metrics
		assert.Contains(t, metrics, "db_feature", "Should have feature detection metrics")
		
		// Log what we found
		lines := strings.Split(metrics, "\n")
		for _, line := range lines {
			if strings.Contains(line, "db_feature") && !strings.HasPrefix(line, "#") {
				t.Logf("Feature metric: %s", line)
			}
		}
	})

	// Verify in NRDB if we have credentials
	if config.NewRelicAPIKey != "" {
		t.Run("VerifyInNRDB", func(t *testing.T) {
			// Implementation provided in main test file
			verifyPostgreSQLFeatureDetection(t, config)
		})
	}
}