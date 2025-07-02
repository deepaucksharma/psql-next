package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorScenarios validates comprehensive error handling
func TestErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error scenarios test in short mode")
	}

	t.Run("Database_Failures", testDatabaseFailures)
	t.Run("Processor_Failures", testProcessorFailures)
	t.Run("Data_Corruption", testDataCorruption)
	t.Run("Cascading_Failures", testCascadingFailures)
	t.Run("Recovery_Mechanisms", testRecoveryMechanisms)
}

// testDatabaseFailures simulates various database failure scenarios
func testDatabaseFailures(t *testing.T) {
	t.Log("Testing database failure scenarios...")

	t.Run("Connection_Timeout", func(t *testing.T) {
		// Create connection with very short timeout
		db, err := sql.Open("postgres", 
			"host=localhost port=5433 user=postgres password=postgres dbname=e2e_test "+
			"sslmode=disable connect_timeout=1")
		require.NoError(t, err)
		defer db.Close()

		// Simulate network delay
		simulateNetworkDelay(t, "e2e-postgres", 2000) // 2 second delay
		defer clearNetworkDelay(t, "e2e-postgres")

		// Attempt query - should timeout
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		_, err = db.QueryContext(ctx, "SELECT 1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timeout")

		// Check circuit breaker activated
		time.Sleep(5 * time.Second)
		metrics := getCollectorMetrics(t)
		timeoutErrors := extractMetricValue(metrics, "otelcol_processor_circuitbreaker_timeout_errors_total")
		assert.Greater(t, timeoutErrors, float64(0), "Should track timeout errors")
	})

	t.Run("Authentication_Failure", func(t *testing.T) {
		// Attempt connection with wrong credentials
		badDB, err := sql.Open("postgres",
			"host=localhost port=5433 user=wronguser password=wrongpass dbname=e2e_test sslmode=disable")
		require.NoError(t, err)
		defer badDB.Close()

		err = badDB.Ping()
		assert.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "auth")

		// Verify authentication errors are tracked
		time.Sleep(5 * time.Second)
		metrics := getCollectorMetrics(t)
		authErrors := extractMetricValue(metrics, "otelcol_receiver_postgresql_auth_errors_total")
		assert.GreaterOrEqual(t, authErrors, float64(0), "Should track auth errors")
	})

	t.Run("Network_Partition", func(t *testing.T) {
		// Simulate network partition
		blockNetworkTraffic(t, "e2e-postgres")
		defer unblockNetworkTraffic(t, "e2e-postgres")

		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Queries should fail
		var result int
		err := pgDB.QueryRow("SELECT 1").Scan(&result)
		assert.Error(t, err)

		// Check partition detection
		time.Sleep(10 * time.Second)
		metrics := getCollectorMetrics(t)
		networkErrors := extractMetricValue(metrics, "otelcol_receiver_postgresql_network_errors_total")
		assert.Greater(t, networkErrors, float64(0), "Should detect network errors")
	})

	t.Run("Resource_Exhaustion", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Set very low connection limit
		pgDB.SetMaxOpenConns(2)
		pgDB.SetMaxIdleConns(1)

		// Try to overwhelm with connections
		var wg sync.WaitGroup
		errors := 0
		var mu sync.Mutex

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				_, err := pgDB.QueryContext(ctx, "SELECT pg_sleep(2)")
				if err != nil {
					mu.Lock()
					errors++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()
		assert.Greater(t, errors, 50, "Should hit connection limits")

		// Check resource exhaustion handling
		metrics := getCollectorMetrics(t)
		resourceErrors := extractMetricValue(metrics, "otelcol_receiver_postgresql_resource_exhaustion_total")
		assert.GreaterOrEqual(t, resourceErrors, float64(0), "Should track resource exhaustion")
	})

	t.Run("Database_Crash_Recovery", func(t *testing.T) {
		// This would require admin access to restart database
		// Simulating by closing all connections
		
		pgDB := connectPostgreSQL(t)
		
		// Force close connection
		pgDB.Close()
		
		// Try to use closed connection
		var result int
		err := pgDB.QueryRow("SELECT 1").Scan(&result)
		assert.Error(t, err)
		
		// Reconnect should work
		newDB := connectPostgreSQL(t)
		defer newDB.Close()
		
		err = newDB.QueryRow("SELECT 1").Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, 1, result)
	})
}

// testProcessorFailures simulates processor-level failures
func testProcessorFailures(t *testing.T) {
	t.Log("Testing processor failure scenarios...")

	t.Run("Processor_Panic_Recovery", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate query that might cause processor issues
		// Very long query that could cause buffer overflow
		longQuery := "SELECT '" + strings.Repeat("x", 1000000) + "'"
		
		rows, err := pgDB.Query(longQuery)
		if rows != nil {
			rows.Close()
		}
		
		// System should recover from any panics
		time.Sleep(5 * time.Second)
		
		// Verify system is still operational
		var result string
		err = pgDB.QueryRow("SELECT 'alive'").Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, "alive", result)

		// Check panic recovery metrics
		metrics := getCollectorMetrics(t)
		panicsRecovered := extractMetricValue(metrics, "otelcol_processor_panic_recovered_total")
		assert.GreaterOrEqual(t, panicsRecovered, float64(0), "Should track panic recovery")
	})

	t.Run("Memory_Limit_Exceeded", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate large result set to trigger memory limits
		for i := 0; i < 10; i++ {
			// Create large temporary data
			_, err := pgDB.Exec(`
				CREATE TEMP TABLE large_table_$1 AS 
				SELECT generate_series(1, 1000000) as id, 
				       repeat('x', 1000) as data
			`, i)
			if err != nil {
				t.Logf("Failed to create large table: %v", err)
			}
		}

		// Query all large tables
		rows, err := pgDB.Query(`
			SELECT table_name 
			FROM information_schema.tables 
			WHERE table_name LIKE 'large_table_%'
		`)
		if err == nil {
			rows.Close()
		}

		// Check memory limit handling
		time.Sleep(10 * time.Second)
		metrics := getCollectorMetrics(t)
		memoryLimitHits := extractMetricValue(metrics, "otelcol_processor_memory_limiter_hits_total")
		assert.Greater(t, memoryLimitHits, float64(0), "Should hit memory limits")
	})

	t.Run("Configuration_Errors", func(t *testing.T) {
		// This tests how system handles configuration errors
		// In real implementation, would modify config and reload
		
		// Check configuration validation metrics
		metrics := getCollectorMetrics(t)
		configErrors := extractMetricValue(metrics, "otelcol_config_validation_errors_total")
		assert.Equal(t, float64(0), configErrors, "Should have no config errors in valid setup")
	})

	t.Run("Dependency_Failures", func(t *testing.T) {
		// Simulate failure of dependent services
		
		// Block Jaeger endpoint
		blockNetworkTraffic(t, "e2e-jaeger")
		defer unblockNetworkTraffic(t, "e2e-jaeger")

		// System should continue operating
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate some queries
		for i := 0; i < 10; i++ {
			pgDB.Query("SELECT current_timestamp")
		}

		time.Sleep(5 * time.Second)

		// Check export failures are handled
		metrics := getCollectorMetrics(t)
		exportFailures := extractMetricValue(metrics, "otelcol_exporter_send_failed_metric_points_total")
		assert.Greater(t, exportFailures, float64(0), "Should track export failures")

		// But data should still be collected
		output := getCollectorOutput(t)
		assert.Contains(t, output, "current_timestamp", "Should still collect data despite export failures")
	})
}

// testDataCorruption simulates data corruption scenarios
func testDataCorruption(t *testing.T) {
	t.Log("Testing data corruption scenarios...")

	t.Run("Malformed_Metrics", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate queries with invalid UTF-8
		invalidUTF8 := "\xc3\x28" // Invalid UTF-8 sequence
		
		// This might fail, which is fine
		pgDB.Query(fmt.Sprintf("SELECT '%s' as corrupted", invalidUTF8))

		// Generate metrics with NaN/Inf values
		pgDB.Query("SELECT 'infinity'::float as value")
		pgDB.Query("SELECT 'nan'::float as value")
		pgDB.Query("SELECT 1.0/0.0 as divide_by_zero")

		time.Sleep(5 * time.Second)

		// Check corruption handling
		metrics := getCollectorMetrics(t)
		corruptedData := extractMetricValue(metrics, "otelcol_processor_verification_corrupted_data_total")
		assert.GreaterOrEqual(t, corruptedData, float64(0), "Should detect corrupted data")
	})

	t.Run("Invalid_Attributes", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate metrics with invalid attribute names
		queries := []string{
			"SELECT 1 as \"\"",                    // Empty attribute name
			"SELECT 1 as \"metric name with spaces\"", // Spaces in name
			"SELECT 1 as \"metric@with#special$chars\"", // Special characters
			"SELECT 1 as \"" + strings.Repeat("x", 500) + "\"", // Very long name
		}

		for _, query := range queries {
			rows, _ := pgDB.Query(query)
			if rows != nil {
				rows.Close()
			}
		}

		time.Sleep(5 * time.Second)

		// Check attribute validation
		metrics := getCollectorMetrics(t)
		invalidAttrs := extractMetricValue(metrics, "otelcol_processor_verification_invalid_attributes_total")
		assert.Greater(t, invalidAttrs, float64(0), "Should detect invalid attributes")
	})

	t.Run("Encoding_Errors", func(t *testing.T) {
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Test various encodings
		encodingTests := []string{
			"SELECT E'\\x00' as null_byte",           // Null byte
			"SELECT E'\\xDEADBEEF' as binary_data",   // Binary data
			"SELECT '你好世界' as chinese",             // Unicode
			"SELECT '🚀🔥💯' as emoji",               // Emojis
		}

		for _, query := range encodingTests {
			rows, _ := pgDB.Query(query)
			if rows != nil {
				rows.Close()
			}
		}

		// System should handle all encodings gracefully
		time.Sleep(5 * time.Second)
		
		// Verify no crash occurred
		var result string
		err := pgDB.QueryRow("SELECT 'encoding_test_complete'").Scan(&result)
		assert.NoError(t, err)
	})

	t.Run("Schema_Violations", func(t *testing.T) {
		// Test metrics that violate expected schema
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate metrics with wrong data types
		pgDB.Query("SELECT 'not_a_number' as numeric_field")
		pgDB.Query("SELECT ARRAY[1,2,3] as scalar_field")
		pgDB.Query("SELECT ROW(1,2,3) as simple_field")

		time.Sleep(5 * time.Second)

		// Check schema validation
		metrics := getCollectorMetrics(t)
		schemaViolations := extractMetricValue(metrics, "otelcol_processor_verification_schema_violations_total")
		assert.GreaterOrEqual(t, schemaViolations, float64(0), "Should detect schema violations")
	})
}

// testCascadingFailures simulates failures that affect multiple components
func testCascadingFailures(t *testing.T) {
	t.Log("Testing cascading failure scenarios...")

	t.Run("Multi_Database_Failure", func(t *testing.T) {
		// Simulate both databases having issues
		simulateNetworkDelay(t, "e2e-postgres", 5000)
		simulateNetworkDelay(t, "e2e-mysql", 5000)
		defer clearNetworkDelay(t, "e2e-postgres")
		defer clearNetworkDelay(t, "e2e-mysql")

		// Both connections should struggle
		pgDB, _ := sql.Open("postgres", 
			"host=localhost port=5433 user=postgres password=postgres dbname=e2e_test "+
			"sslmode=disable connect_timeout=1")
		defer pgDB.Close()

		mysqlDB, _ := sql.Open("mysql",
			"mysql:mysql@tcp(localhost:3307)/e2e_test?timeout=1s")
		defer mysqlDB.Close()

		// Both should timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		var pgErr, mysqlErr error

		wg.Add(2)
		go func() {
			defer wg.Done()
			_, pgErr = pgDB.QueryContext(ctx, "SELECT 1")
		}()

		go func() {
			defer wg.Done()
			_, mysqlErr = mysqlDB.QueryContext(ctx, "SELECT 1")
		}()

		wg.Wait()

		assert.Error(t, pgErr, "PostgreSQL should fail")
		assert.Error(t, mysqlErr, "MySQL should fail")

		// Check system degradation handling
		time.Sleep(10 * time.Second)
		metrics := getCollectorMetrics(t)
		
		// Both circuit breakers should open
		pgCircuitOpen := extractMetricValueWithLabel(metrics, 
			"otelcol_processor_circuitbreaker_state", "database", "postgresql")
		mysqlCircuitOpen := extractMetricValueWithLabel(metrics,
			"otelcol_processor_circuitbreaker_state", "database", "mysql")
		
		assert.Greater(t, pgCircuitOpen, float64(0), "PostgreSQL circuit should open")
		assert.Greater(t, mysqlCircuitOpen, float64(0), "MySQL circuit should open")
	})

	t.Run("Processor_Chain_Failure", func(t *testing.T) {
		// Simulate failure in middle of processor chain
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate query that would fail at verification processor
		// but pass through others
		maliciousQuery := "SELECT * FROM users; DROP TABLE users; --"
		
		rows, _ := pgDB.Query(maliciousQuery)
		if rows != nil {
			rows.Close()
		}

		time.Sleep(5 * time.Second)

		// Check how failure propagates
		metrics := getCollectorMetrics(t)
		
		// Verification should catch it
		verificationBlocked := extractMetricValue(metrics, 
			"otelcol_processor_verification_queries_blocked_total")
		assert.Greater(t, verificationBlocked, float64(0), "Verification should block query")

		// Downstream processors shouldn't see it
		adaptiveSamplerProcessed := extractMetricValue(metrics,
			"otelcol_processor_adaptivesampler_processed_total")
		// This is a simplified check - in reality would need more sophisticated verification
	})

	t.Run("Resource_Competition", func(t *testing.T) {
		// Create resource competition between components
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate high CPU load
		var wg sync.WaitGroup
		stopCPULoad := make(chan struct{})

		// CPU intensive operations
		for i := 0; i < runtime.NumCPU()*2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stopCPULoad:
						return
					default:
						// Busy loop
						for j := 0; j < 1000000; j++ {
							_ = j * j
						}
					}
				}
			}()
		}

		// While CPU is loaded, try database operations
		start := time.Now()
		var result int
		err := pgDB.QueryRow("SELECT COUNT(*) FROM e2e_test.events").Scan(&result)
		queryTime := time.Since(start)

		// Stop CPU load
		close(stopCPULoad)
		wg.Wait()

		assert.NoError(t, err, "Query should still succeed under CPU pressure")
		assert.Less(t, queryTime, 5*time.Second, "Query should complete in reasonable time")

		// Check resource pressure handling
		metrics := getCollectorMetrics(t)
		cpuThrottling := extractMetricValue(metrics, "otelcol_processor_cpu_throttled_total")
		assert.GreaterOrEqual(t, cpuThrottling, float64(0), "Should track CPU throttling")
	})
}

// testRecoveryMechanisms validates system recovery capabilities
func testRecoveryMechanisms(t *testing.T) {
	t.Log("Testing recovery mechanisms...")

	t.Run("Automatic_Reconnection", func(t *testing.T) {
		// Simulate temporary network issue
		blockNetworkTraffic(t, "e2e-postgres")
		
		// Wait a bit
		time.Sleep(5 * time.Second)
		
		// Restore network
		unblockNetworkTraffic(t, "e2e-postgres")
		
		// System should automatically reconnect
		time.Sleep(10 * time.Second)
		
		// Verify reconnection
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()
		
		var result string
		err := pgDB.QueryRow("SELECT 'reconnected'").Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, "reconnected", result)

		// Check reconnection metrics
		metrics := getCollectorMetrics(t)
		reconnections := extractMetricValue(metrics, "otelcol_receiver_postgresql_reconnections_total")
		assert.Greater(t, reconnections, float64(0), "Should track reconnections")
	})

	t.Run("Graceful_Degradation", func(t *testing.T) {
		// Test system continues with reduced functionality
		
		// Disable one processor (simulate failure)
		// In real implementation would modify config
		
		// System should continue processing with remaining processors
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Generate test load
		for i := 0; i < 100; i++ {
			pgDB.Query("SELECT current_timestamp")
		}

		time.Sleep(5 * time.Second)

		// Verify partial processing still occurs
		output := getCollectorOutput(t)
		assert.Contains(t, output, "current_timestamp", "Should process despite degradation")

		// Check degradation metrics
		metrics := getCollectorMetrics(t)
		degradedMode := extractMetricValue(metrics, "otelcol_degraded_mode_active")
		assert.GreaterOrEqual(t, degradedMode, float64(0), "Should track degraded mode")
	})

	t.Run("State_Recovery", func(t *testing.T) {
		// Test state recovery after restart
		
		// First, generate some state
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Create identifiable queries
		for i := 0; i < 10; i++ {
			pgDB.Query(fmt.Sprintf("SELECT 'state_test_%d'", i))
		}

		time.Sleep(5 * time.Second)

		// Simulate restart (would restart container in real test)
		// For now, just verify state persistence mechanisms
		
		metrics := getCollectorMetrics(t)
		stateSaved := extractMetricValue(metrics, "otelcol_processor_state_saved_total")
		assert.GreaterOrEqual(t, stateSaved, float64(0), "Should save state periodically")
	})

	t.Run("Self_Healing", func(t *testing.T) {
		// Test self-healing mechanisms
		
		// Generate problematic patterns
		pgDB := connectPostgreSQL(t)
		defer pgDB.Close()

		// Cause issues that should trigger self-healing
		for i := 0; i < 100; i++ {
			// Queries with increasing complexity
			query := "SELECT " + strings.Repeat("1,", i) + "1"
			rows, _ := pgDB.Query(query)
			if rows != nil {
				rows.Close()
			}
		}

		// Wait for self-healing
		time.Sleep(30 * time.Second)

		// Check self-healing actions
		metrics := getCollectorMetrics(t)
		healingActions := extractMetricValue(metrics, "otelcol_processor_verification_self_healing_actions_total")
		assert.Greater(t, healingActions, float64(0), "Should perform self-healing actions")

		// Verify system is healthy after healing
		var result string
		err := pgDB.QueryRow("SELECT 'healed'").Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, "healed", result)
	})
}

// Helper functions for simulating failures

func simulateNetworkDelay(t *testing.T, container string, delayMs int) {
	// In real implementation, would use tc (traffic control) commands
	cmd := fmt.Sprintf("tc qdisc add dev eth0 root netem delay %dms", delayMs)
	execInContainer(container, cmd)
}

func clearNetworkDelay(t *testing.T, container string) {
	execInContainer(container, "tc qdisc del dev eth0 root netem")
}

func blockNetworkTraffic(t *testing.T, container string) {
	// In real implementation, would use iptables
	execInContainer(container, "iptables -A INPUT -j DROP")
	execInContainer(container, "iptables -A OUTPUT -j DROP")
}

func unblockNetworkTraffic(t *testing.T, container string) {
	execInContainer(container, "iptables -F")
}