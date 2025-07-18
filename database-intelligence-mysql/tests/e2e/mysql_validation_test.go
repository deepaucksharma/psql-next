package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/database-intelligence/mysql-monitoring/tests/e2e/framework"
	"go.uber.org/zap"
)

// MySQLValidationTestSuite validates MySQL metrics collection
type MySQLValidationTestSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	validator *framework.MetricValidator
	logger    *zap.Logger
	
	collectorProcess *exec.Cmd
}

func TestMySQLValidation(t *testing.T) {
	suite.Run(t, new(MySQLValidationTestSuite))
}

func (s *MySQLValidationTestSuite) SetupSuite() {
	// Initialize logger
	s.logger, _ = zap.NewDevelopment()
	
	// Setup test environment
	s.env = framework.NewTestEnvironment()
	err := s.env.Setup()
	s.Require().NoError(err, "Failed to setup test environment")
	
	// Create test data
	err = s.env.CreateTestData()
	s.Require().NoError(err, "Failed to create test data")
	
	// Initialize validator
	s.validator = framework.NewMetricValidator(s.logger, "http://localhost:9090/metrics")
	
	// Start OpenTelemetry Collector
	s.startCollector()
	
	// Wait for collector to initialize
	time.Sleep(10 * time.Second)
}

func (s *MySQLValidationTestSuite) TearDownSuite() {
	// Stop collector
	if s.collectorProcess != nil {
		s.collectorProcess.Process.Kill()
	}
	
	// Teardown environment
	s.env.Teardown()
}

func (s *MySQLValidationTestSuite) TestBasicMetricsCollection() {
	s.Run("MySQL metrics should be collected", func() {
		// Validate that basic MySQL metrics are present
		err := s.validator.ValidateMySQLMetrics()
		s.NoError(err, "Basic MySQL metrics validation failed")
	})
	
	s.Run("Connection metrics should be accurate", func() {
		// Get current connection count from MySQL
		var activeConnections int
		err := s.env.GetDB().QueryRow("SELECT COUNT(*) FROM information_schema.processlist").Scan(&activeConnections)
		s.NoError(err)
		
		// Validate metric value
		result := s.validator.ValidateMetricValue("mysql_connections_active", float64(activeConnections))
		s.True(result.Passed, "Connection count validation failed: expected %f, got %f", 
			result.Expected, result.Actual)
	})
}

func (s *MySQLValidationTestSuite) TestQueryMetrics() {
	s.Run("Query metrics should increase with workload", func() {
		// Get initial query count
		initialQueries, err := s.validator.getMetricValue("mysql_queries_total")
		s.NoError(err)
		
		// Generate workload
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		go s.env.CreateKnownWorkload(5 * time.Second)
		<-ctx.Done()
		cancel()
		
		// Wait for metrics to update
		time.Sleep(5 * time.Second)
		
		// Get new query count
		finalQueries, err := s.validator.getMetricValue("mysql_queries_total")
		s.NoError(err)
		
		// Verify queries increased
		s.Greater(finalQueries, initialQueries, "Query count should have increased")
	})
}

func (s *MySQLValidationTestSuite) TestPerformanceSchemaMetrics() {
	s.Run("Performance schema metrics should be collected", func() {
		metrics := []string{
			"mysql_statement_event_count_total",
			"mysql_statement_event_wait_time_total",
			"mysql_table_io_wait_count_total",
			"mysql_table_io_wait_time_total",
		}
		
		for _, metric := range metrics {
			exists, err := s.validator.ValidateMetricExists(metric)
			s.NoError(err)
			s.True(exists, "Metric %s should exist", metric)
		}
	})
}

func (s *MySQLValidationTestSuite) TestInnoDBMetrics() {
	s.Run("InnoDB metrics should be collected", func() {
		metrics := []string{
			"mysql_buffer_pool_usage",
			"mysql_buffer_pool_pages_total",
			"mysql_innodb_row_operations_total",
			"mysql_innodb_row_lock_waits_total",
		}
		
		for _, metric := range metrics {
			exists, err := s.validator.ValidateMetricExists(metric)
			s.NoError(err)
			s.True(exists, "Metric %s should exist", metric)
		}
	})
}

func (s *MySQLValidationTestSuite) TestReplicationMetrics() {
	// Skip if not running with replication
	if os.Getenv("MYSQL_REPLICA_HOST") == "" {
		s.T().Skip("Replication not configured")
	}
	
	s.Run("Replication metrics should be collected", func() {
		metrics := []string{
			"mysql_replica_time_behind_source_seconds",
			"mysql_replica_sql_delay_seconds",
		}
		
		for _, metric := range metrics {
			exists, err := s.validator.ValidateMetricExists(metric)
			s.NoError(err)
			s.True(exists, "Metric %s should exist", metric)
		}
	})
}

func (s *MySQLValidationTestSuite) TestFileExporter() {
	s.Run("Metrics should be exported to file", func() {
		// Check if file exporter is configured
		outputFile := "/tmp/mysql-metrics.json"
		if _, err := os.Stat(outputFile); os.IsNotExist(err) {
			s.T().Skip("File exporter not configured")
		}
		
		// Validate file contains expected metrics
		expectedMetrics := []string{
			"mysql.connections",
			"mysql.queries",
			"mysql.buffer_pool.usage",
			"mysql.innodb.row_operations",
		}
		
		err := s.validator.ValidateFileOutput(outputFile, expectedMetrics)
		s.NoError(err, "File output validation failed")
	})
}

func (s *MySQLValidationTestSuite) TestMetricLabels() {
	s.Run("Metrics should have proper labels", func() {
		// This would require parsing Prometheus format with labels
		// For now, just check that metrics exist
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		// Check for expected labels in the output
		s.Contains(metrics, "database=", "Metrics should have database label")
		s.Contains(metrics, "host=", "Metrics should have host label")
	})
}

// Helper method to start the OpenTelemetry Collector
func (s *MySQLValidationTestSuite) startCollector() {
	// Look for collector binary
	collectorPath := "./otelcol"
	if _, err := os.Stat(collectorPath); os.IsNotExist(err) {
		collectorPath = "otelcol-contrib"
		if _, err := os.Stat(collectorPath); os.IsNotExist(err) {
			s.T().Skip("OpenTelemetry Collector binary not found")
		}
	}
	
	// Start collector with test configuration
	configPath := "../../config/otel-collector-config.yaml"
	s.collectorProcess = exec.Command(collectorPath, "--config", configPath)
	
	// Set environment variables
	s.collectorProcess.Env = append(os.Environ(),
		fmt.Sprintf("MYSQL_HOST=%s", s.env.MySQLHost),
		fmt.Sprintf("MYSQL_PORT=%d", s.env.MySQLPort),
		fmt.Sprintf("MYSQL_USER=%s", s.env.MySQLUser),
		fmt.Sprintf("MYSQL_PASSWORD=%s", s.env.MySQLPassword),
		fmt.Sprintf("MYSQL_DATABASE=%s", s.env.MySQLDatabase),
	)
	
	// Start the process
	err := s.collectorProcess.Start()
	s.Require().NoError(err, "Failed to start collector")
	
	s.logger.Info("OpenTelemetry Collector started",
		zap.String("path", collectorPath),
		zap.String("config", configPath))
}

// Helper to get metric value (accessing unexported method via interface)
func (s *MySQLValidationTestSuite) getMetricValue(metricName string) (float64, error) {
	// This is a workaround - in real implementation, make the method public
	// or use the ValidateMetricValue method
	result := s.validator.ValidateMetricValue(metricName, 0)
	if result.Error != "" {
		return 0, fmt.Errorf(result.Error)
	}
	return result.Actual, nil
}