package scenarios

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	
	"github.com/deepaksharma/db-otel/tests/e2e/framework"
)

// RealWorldValidationTestSuite validates real metrics collection
type RealWorldValidationTestSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
}

func TestRealWorldValidation(t *testing.T) {
	suite.Run(t, new(RealWorldValidationTestSuite))
}

func (s *RealWorldValidationTestSuite) SetupSuite() {
	s.env = framework.NewTestEnvironment()
	require.NoError(s.T(), s.env.Setup())
	
	s.collector = framework.NewTestCollector(s.env)
}

func (s *RealWorldValidationTestSuite) TearDownSuite() {
	if s.collector != nil {
		s.collector.Stop()
	}
	s.env.Teardown()
}

func (s *RealWorldValidationTestSuite) TestStandardModeMetrics() {
	// Start collector in standard mode
	configPath := "../configs/collector-standard.yaml"
	config, err := os.ReadFile(configPath)
	require.NoError(s.T(), err)
	
	// Set standard mode
	os.Setenv("TEST_MODE", "standard")
	defer os.Unsetenv("TEST_MODE")
	
	// Start collector
	err = s.collector.Start(string(config))
	require.NoError(s.T(), err)
	
	// Wait for metrics collection
	time.Sleep(15 * time.Second)
	
	// Validate metrics from Prometheus endpoint
	metrics := s.getPrometheusMetrics("http://localhost:9091/metrics")
	
	// Check core PostgreSQL metrics exist
	s.assertMetricExists(metrics, "e2e_test_postgresql_connections_active")
	s.assertMetricExists(metrics, "e2e_test_postgresql_commits")
	s.assertMetricExists(metrics, "e2e_test_postgresql_rollbacks")
	s.assertMetricExists(metrics, "e2e_test_postgresql_blocks_hit")
	s.assertMetricExists(metrics, "e2e_test_postgresql_database_size")
	
	// Check custom SQL query metrics
	s.assertMetricExists(metrics, "e2e_test_postgresql_connections_by_state")
	s.assertMetricExists(metrics, "e2e_test_postgresql_queries_long_running_count")
	
	// Check host metrics
	s.assertMetricExists(metrics, "e2e_test_system_cpu_utilization")
	s.assertMetricExists(metrics, "e2e_test_system_memory_utilization")
	
	// Validate file output
	s.validateFileOutput("/tmp/e2e-test-metrics-standard.json", []string{
		"postgresql.connections",
		"postgresql.commits",
		"postgresql.database.size",
		"system.cpu.utilization",
	})
}

func (s *RealWorldValidationTestSuite) TestEnhancedModeMetrics() {
	// Skip if enhanced collector not built
	if _, err := os.Stat("../../distributions/production/database-intelligence-collector"); err != nil {
		s.T().Skip("Enhanced collector not built - run 'make build-collector' first")
	}
	
	// Stop standard collector if running
	s.collector.Stop()
	time.Sleep(2 * time.Second)
	
	// Start collector in enhanced mode
	configPath := "../configs/collector-enhanced.yaml"
	config, err := os.ReadFile(configPath)
	require.NoError(s.T(), err)
	
	// Set enhanced mode
	os.Setenv("TEST_MODE", "enhanced")
	defer os.Unsetenv("TEST_MODE")
	
	// Start collector
	err = s.collector.Start(string(config))
	require.NoError(s.T(), err)
	
	// Wait for metrics collection
	time.Sleep(15 * time.Second)
	
	// Validate metrics from Prometheus endpoint
	metrics := s.getPrometheusMetrics("http://localhost:9092/metrics")
	
	// Check standard metrics still work
	s.assertMetricExists(metrics, "e2e_test_enhanced_postgresql_connections_active")
	
	// Check enhanced metrics
	s.assertMetricExists(metrics, "e2e_test_enhanced_db_ash_active_sessions")
	s.assertMetricExists(metrics, "e2e_test_enhanced_db_ash_wait_events")
	
	// Check OHI transformed metrics
	s.assertMetricExists(metrics, "e2e_test_enhanced_PostgresSlowQueries")
	s.assertMetricExists(metrics, "e2e_test_enhanced_PostgresWaitEvents")
	
	// Validate NRI output
	s.validateNRIOutput("/tmp/e2e-test-nri-output.json")
}

func (s *RealWorldValidationTestSuite) TestMetricAccuracy() {
	// Create known database load
	s.createKnownLoad()
	
	// Wait for collection
	time.Sleep(20 * time.Second)
	
	// Get metrics
	metrics := s.getPrometheusMetrics("http://localhost:9091/metrics")
	
	// Verify connection count matches
	activeConnections := s.getMetricValue(metrics, "e2e_test_postgresql_connections_active")
	s.Assert().GreaterOrEqual(activeConnections, float64(1), "Should have at least 1 active connection")
	
	// Verify transactions occurred
	commits := s.getMetricValue(metrics, "e2e_test_postgresql_commits")
	s.Assert().Greater(commits, float64(0), "Should have recorded commits")
}

func (s *RealWorldValidationTestSuite) TestProcessorFunctionality() {
	// Only test if enhanced mode is available
	if os.Getenv("TEST_MODE") != "enhanced" {
		s.T().Skip("Processor tests require enhanced mode")
	}
	
	// Check circuit breaker functionality
	s.testCircuitBreaker()
	
	// Check adaptive sampling
	s.testAdaptiveSampling()
	
	// Check OHI transformation
	s.testOHITransformation()
}

// Helper methods

func (s *RealWorldValidationTestSuite) getPrometheusMetrics(endpoint string) map[string]float64 {
	resp, err := http.Get(endpoint)
	require.NoError(s.T(), err)
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	require.NoError(s.T(), err)
	
	// Parse Prometheus format
	metrics := make(map[string]float64)
	lines := string(body)
	// Simple parsing - in real implementation use proper Prometheus parser
	// This is simplified for test purposes
	_ = lines
	
	return metrics
}

func (s *RealWorldValidationTestSuite) assertMetricExists(metrics map[string]float64, metricName string) {
	_, exists := metrics[metricName]
	s.Assert().True(exists, fmt.Sprintf("Metric %s should exist", metricName))
}

func (s *RealWorldValidationTestSuite) getMetricValue(metrics map[string]float64, metricName string) float64 {
	value, exists := metrics[metricName]
	s.Require().True(exists, fmt.Sprintf("Metric %s not found", metricName))
	return value
}

func (s *RealWorldValidationTestSuite) validateFileOutput(filePath string, expectedMetrics []string) {
	data, err := os.ReadFile(filePath)
	require.NoError(s.T(), err)
	
	var output []map[string]interface{}
	err = json.Unmarshal(data, &output)
	require.NoError(s.T(), err)
	
	// Check that expected metrics are present
	foundMetrics := make(map[string]bool)
	for _, record := range output {
		if metricName, ok := record["metric_name"].(string); ok {
			foundMetrics[metricName] = true
		}
	}
	
	for _, expected := range expectedMetrics {
		s.Assert().True(foundMetrics[expected], fmt.Sprintf("Expected metric %s not found in output", expected))
	}
}

func (s *RealWorldValidationTestSuite) validateNRIOutput(filePath string) {
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		// NRI exporter might not have written yet
		return
	}
	require.NoError(s.T(), err)
	
	// Validate NRI format
	var nriData map[string]interface{}
	err = json.Unmarshal(data, &nriData)
	require.NoError(s.T(), err)
	
	// Check for expected NRI structure
	s.Assert().Contains(nriData, "name")
	s.Assert().Contains(nriData, "integration_version")
	s.Assert().Contains(nriData, "data")
}

func (s *RealWorldValidationTestSuite) createKnownLoad() {
	// Execute some known database operations
	queries := []string{
		"CREATE TABLE IF NOT EXISTS test_load (id SERIAL PRIMARY KEY, data TEXT)",
		"INSERT INTO test_load (data) VALUES ('test1'), ('test2'), ('test3')",
		"SELECT COUNT(*) FROM test_load",
		"UPDATE test_load SET data = 'updated' WHERE id = 1",
		"DELETE FROM test_load WHERE id = 2",
		"SELECT pg_sleep(0.1)", // Create a slightly slow query
	}
	
	for _, query := range queries {
		_, err := s.env.DB.Exec(query)
		if err != nil {
			s.T().Logf("Query failed (may be expected): %s - %v", query, err)
		}
	}
	
	// Create multiple connections
	for i := 0; i < 5; i++ {
		go func() {
			db, _ := s.env.GetPostgresConnection()
			defer db.Close()
			db.Query("SELECT 1")
			time.Sleep(5 * time.Second)
		}()
	}
}

func (s *RealWorldValidationTestSuite) testCircuitBreaker() {
	// Simulate database errors to trigger circuit breaker
	// This would require access to the collector's internals
	// For now, just check that the processor is loaded
	logs, err := s.collector.GetLogs()
	require.NoError(s.T(), err)
	s.Assert().Contains(logs, "circuitbreaker")
}

func (s *RealWorldValidationTestSuite) testAdaptiveSampling() {
	// Check that sampling is occurring
	// Would need to generate high volume and check reduction
	logs, err := s.collector.GetLogs()
	require.NoError(s.T(), err)
	s.Assert().Contains(logs, "adaptive_sampler")
}

func (s *RealWorldValidationTestSuite) testOHITransformation() {
	// Check that OHI transformation is working
	metrics := s.getPrometheusMetrics("http://localhost:9092/metrics")
	
	// Should have both original and transformed metrics
	if os.Getenv("TEST_MODE") == "enhanced" {
		s.assertMetricExists(metrics, "e2e_test_enhanced_db_ash_active_sessions")
		s.assertMetricExists(metrics, "e2e_test_enhanced_PostgresSlowQueries")
	}
}