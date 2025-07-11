// Package suites contains comprehensive end-to-end test implementations
package suites

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/database-intelligence/tests/e2e/framework"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// ComprehensiveTestSuite runs the full set of e2e tests for database intelligence
type ComprehensiveTestSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
	ctx       context.Context
	cancel    context.CancelFunc
}

// SetupSuite initializes the test environment once for all tests
func (s *ComprehensiveTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Minute)
	
	// Initialize test environment
	env, err := framework.NewTestEnvironment(s.ctx, framework.TestConfig{
		DatabaseType: "postgresql",
		EnableMySQL:  true,
		EnableRedis:  false,
	})
	s.Require().NoError(err, "Failed to create test environment")
	s.env = env

	// Start collector with comprehensive configuration
	collector, err := framework.NewTestCollector(s.ctx, framework.CollectorConfig{
		ConfigPath: "../configs/collector-test.yaml",
		LogLevel:   "debug",
		Features: []string{
			"postgresql",
			"mysql",
			"hostmetrics",
			"sqlquery",
			"enhancedsql",
			"ash",
		},
	})
	s.Require().NoError(err, "Failed to create test collector")
	s.collector = collector

	// Wait for collector to be ready
	s.Require().NoError(s.collector.WaitForReady(s.ctx, 2*time.Minute))
}

// TearDownSuite cleans up after all tests
func (s *ComprehensiveTestSuite) TearDownSuite() {
	if s.collector != nil {
		s.collector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
	s.cancel()
}

// Test01_PostgreSQLBasicMetrics validates core PostgreSQL metrics collection
func (s *ComprehensiveTestSuite) Test01_PostgreSQLBasicMetrics() {
	// Generate test workload
	workload := s.env.CreateWorkload("basic_postgres", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       10,
		ConnectionCount: 5,
		Operations: []string{
			"SELECT", "INSERT", "UPDATE", "DELETE",
		},
	})
	
	// Run workload
	err := workload.Run(s.ctx)
	s.Require().NoError(err, "Failed to run workload")

	// Wait for metrics to be collected
	time.Sleep(30 * time.Second)

	// Validate metrics
	metrics, err := s.collector.GetMetrics(s.ctx, "postgresql.*")
	s.Require().NoError(err, "Failed to get PostgreSQL metrics")

	// Check required metrics are present
	requiredMetrics := []string{
		"postgresql.connections.active",
		"postgresql.connections.idle",
		"postgresql.connections.max",
		"postgresql.transactions.committed",
		"postgresql.transactions.rolled_back",
		"postgresql.blocks.hit",
		"postgresql.blocks.read",
		"postgresql.database.size",
		"postgresql.table.count",
		"postgresql.index.scans",
		"postgresql.sequential_scans",
		"postgresql.deadlocks",
		"postgresql.temp_files.created",
		"postgresql.wal.generated",
	}

	foundMetrics := make(map[string]bool)
	for _, m := range metrics {
		foundMetrics[m.Name()] = true
	}

	for _, required := range requiredMetrics {
		s.Assert().True(foundMetrics[required], "Missing required metric: %s", required)
	}

	// Validate metric attributes
	s.validateMetricAttributes(metrics)
}

// Test02_MySQLBasicMetrics validates core MySQL metrics collection
func (s *ComprehensiveTestSuite) Test02_MySQLBasicMetrics() {
	// Generate MySQL workload
	workload := s.env.CreateWorkload("basic_mysql", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       10,
		ConnectionCount: 5,
		DatabaseType:    "mysql",
		Operations: []string{
			"SELECT", "INSERT", "UPDATE", "DELETE",
		},
	})
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err, "Failed to run MySQL workload")

	time.Sleep(30 * time.Second)

	// Validate MySQL metrics
	metrics, err := s.collector.GetMetrics(s.ctx, "mysql.*")
	s.Require().NoError(err, "Failed to get MySQL metrics")

	requiredMetrics := []string{
		"mysql.connections",
		"mysql.threads",
		"mysql.queries",
		"mysql.slow_queries",
		"mysql.innodb.buffer_pool.pages",
		"mysql.innodb.row_locks",
		"mysql.commands",
		"mysql.handlers",
		"mysql.table_locks_waited",
		"mysql.aborted_connects",
	}

	foundMetrics := make(map[string]bool)
	for _, m := range metrics {
		foundMetrics[m.Name()] = true
	}

	for _, required := range requiredMetrics {
		s.Assert().True(foundMetrics[required], "Missing required MySQL metric: %s", required)
	}
}

// Test03_HostMetrics validates host-level metrics collection
func (s *ComprehensiveTestSuite) Test03_HostMetrics() {
	time.Sleep(30 * time.Second) // Let host metrics accumulate

	metrics, err := s.collector.GetMetrics(s.ctx, "system.*")
	s.Require().NoError(err, "Failed to get host metrics")

	requiredMetrics := []string{
		"system.cpu.utilization",
		"system.memory.utilization",
		"system.memory.usage",
		"system.disk.io",
		"system.disk.operations",
		"system.network.io",
		"system.filesystem.utilization",
		"system.cpu.load_average.1m",
		"system.cpu.load_average.5m",
		"system.cpu.load_average.15m",
	}

	foundMetrics := make(map[string]bool)
	for _, m := range metrics {
		foundMetrics[m.Name()] = true
	}

	for _, required := range requiredMetrics {
		s.Assert().True(foundMetrics[required], "Missing required host metric: %s", required)
	}
}

// Test04_CustomSQLQueries validates custom SQL query receiver
func (s *ComprehensiveTestSuite) Test04_CustomSQLQueries() {
	// The sqlquery receiver should be collecting custom metrics
	time.Sleep(60 * time.Second) // Wait for custom query interval

	metrics, err := s.collector.GetMetrics(s.ctx, "postgresql.queries.*")
	s.Require().NoError(err, "Failed to get custom query metrics")

	// Check for custom metrics defined in sqlquery receiver
	expectedMetrics := []string{
		"postgresql.queries.long_running.count",
		"postgresql.queries.duration.max",
		"postgresql.connections.by_state",
	}

	foundMetrics := make(map[string]bool)
	for _, m := range metrics {
		foundMetrics[m.Name()] = true
	}

	for _, expected := range expectedMetrics {
		s.Assert().True(foundMetrics[expected], "Missing custom query metric: %s", expected)
	}
}

// Test05_ConnectionPoolMetrics validates connection pool monitoring
func (s *ComprehensiveTestSuite) Test05_ConnectionPoolMetrics() {
	// Create connection pool stress
	workload := s.env.CreateWorkload("connection_pool", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		ConnectionCount: 50, // High connection count
		QueryRate:       1,  // Low query rate
		Operations:      []string{"SELECT"},
	})

	err := workload.Run(s.ctx)
	s.Require().NoError(err)

	time.Sleep(30 * time.Second)

	metrics, err := s.collector.GetMetrics(s.ctx, "postgresql.connections.*")
	s.Require().NoError(err)

	// Verify connection metrics show the load
	var activeConnections float64
	for _, m := range metrics {
		if m.Name() == "postgresql.connections.active" {
			// Get the latest data point
			m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).DoubleValue()
			activeConnections = m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).DoubleValue()
			break
		}
	}

	s.Assert().Greater(activeConnections, float64(10), "Expected high connection count during pool test")
}

// Test06_TransactionMetrics validates transaction monitoring
func (s *ComprehensiveTestSuite) Test06_TransactionMetrics() {
	// Create transaction-heavy workload
	workload := s.env.CreateWorkload("transactions", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       20,
		ConnectionCount: 10,
		Operations: []string{
			"BEGIN", "INSERT", "UPDATE", "COMMIT", "ROLLBACK",
		},
		TransactionSize: 5, // 5 operations per transaction
	})

	initialMetrics, _ := s.collector.GetMetrics(s.ctx, "postgresql.transactions.*")
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err)

	time.Sleep(30 * time.Second)

	finalMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.transactions.*")
	s.Require().NoError(err)

	// Verify transaction counts increased
	s.Assert().Greater(len(finalMetrics), len(initialMetrics), "Expected transaction metrics to increase")
}

// Test07_QueryPerformanceMetrics validates query performance tracking
func (s *ComprehensiveTestSuite) Test07_QueryPerformanceMetrics() {
	// Create workload with varied query performance
	workload := s.env.CreateWorkload("query_performance", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       10,
		ConnectionCount: 5,
		Operations: []string{
			"SELECT pg_sleep(0.001)", // Fast query
			"SELECT pg_sleep(0.1)",   // Medium query
			"SELECT pg_sleep(1)",     // Slow query
		},
	})

	err := workload.Run(s.ctx)
	s.Require().NoError(err)

	time.Sleep(30 * time.Second)

	// Check for query duration metrics
	metrics, err := s.collector.GetMetrics(s.ctx, "postgresql.query.*")
	s.Require().NoError(err)

	// Look for duration histogram
	var foundDurationMetric bool
	for _, m := range metrics {
		if strings.Contains(m.Name(), "duration") {
			foundDurationMetric = true
			// Validate it's a histogram
			s.Assert().Equal(pmetric.MetricTypeHistogram, m.Type())
			break
		}
	}
	s.Assert().True(foundDurationMetric, "Expected to find query duration metrics")
}

// Test08_ErrorAndDeadlockMetrics validates error condition monitoring
func (s *ComprehensiveTestSuite) Test08_ErrorAndDeadlockMetrics() {
	// Create workload that causes errors and potential deadlocks
	workload := s.env.CreateWorkload("errors_deadlocks", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       5,
		ConnectionCount: 10,
		Operations: []string{
			"INVALID SQL", // Syntax error
			"SELECT * FROM nonexistent_table", // Table not found
			"UPDATE test_table SET value = value + 1 WHERE id = 1", // Potential lock conflicts
		},
		ConcurrentUpdates: true, // Enable concurrent updates to same rows
	})

	err := workload.Run(s.ctx)
	// Errors are expected, so we don't fail on error

	time.Sleep(30 * time.Second)

	// Check for error-related metrics
	metrics, err := s.collector.GetMetrics(s.ctx, "postgresql.*")
	s.Require().NoError(err)

	// Look for rollback and deadlock metrics
	var foundRollbacks, foundDeadlocks bool
	for _, m := range metrics {
		if m.Name() == "postgresql.transactions.rolled_back" {
			foundRollbacks = true
		}
		if m.Name() == "postgresql.deadlocks" {
			foundDeadlocks = true
		}
	}
	s.Assert().True(foundRollbacks, "Expected to find rollback metrics")
	s.Assert().True(foundDeadlocks, "Expected to find deadlock metrics")
}

// Test09_MultiDatabaseMetrics validates metrics from multiple databases
func (s *ComprehensiveTestSuite) Test09_MultiDatabaseMetrics() {
	// Create additional databases
	err := s.env.CreateDatabase("testdb2")
	s.Require().NoError(err)
	err = s.env.CreateDatabase("testdb3")
	s.Require().NoError(err)

	// Run workload on each database
	for i, db := range []string{"postgres", "testdb2", "testdb3"} {
		workload := s.env.CreateWorkload(fmt.Sprintf("multi_db_%d", i), framework.WorkloadConfig{
			Duration:        1 * time.Minute,
			QueryRate:       5,
			ConnectionCount: 2,
			Database:        db,
			Operations:      []string{"SELECT", "INSERT"},
		})
		
		go workload.Run(s.ctx) // Run concurrently
	}

	time.Sleep(2 * time.Minute)

	// Verify metrics have database dimension
	metrics, err := s.collector.GetMetrics(s.ctx, "postgresql.*")
	s.Require().NoError(err)

	databases := make(map[string]bool)
	for _, m := range metrics {
		// Extract database attribute
		m.ResourceMetrics().At(0).Resource().Attributes().Range(func(k string, v interface{}) bool {
			if k == "db.name" {
				databases[v.(string)] = true
			}
			return true
		})
	}

	s.Assert().True(databases["postgres"], "Missing metrics for postgres database")
	s.Assert().True(databases["testdb2"], "Missing metrics for testdb2 database")
	s.Assert().True(databases["testdb3"], "Missing metrics for testdb3 database")
}

// Test10_MetricAttributeValidation ensures all metrics have required attributes
func (s *ComprehensiveTestSuite) Test10_MetricAttributeValidation() {
	metrics, err := s.collector.GetMetrics(s.ctx, "*")
	s.Require().NoError(err)

	requiredResourceAttributes := []string{
		"service.name",
		"db.system",
		"host.name",
		"telemetry.sdk.name",
		"telemetry.sdk.language",
	}

	for _, m := range metrics {
		resourceAttrs := make(map[string]bool)
		m.ResourceMetrics().At(0).Resource().Attributes().Range(func(k string, v interface{}) bool {
			resourceAttrs[k] = true
			return true
		})

		for _, required := range requiredResourceAttributes {
			if strings.HasPrefix(m.Name(), "postgresql.") || strings.HasPrefix(m.Name(), "mysql.") {
				s.Assert().True(resourceAttrs[required], 
					"Metric %s missing required attribute: %s", m.Name(), required)
			}
		}
	}
}

// validateMetricAttributes checks that metrics have proper attributes set
func (s *ComprehensiveTestSuite) validateMetricAttributes(metrics []pmetric.Metric) {
	for _, metric := range metrics {
		// Each metric should have resource attributes
		s.Assert().NotEmpty(metric.ResourceMetrics())
		
		if metric.ResourceMetrics().Len() > 0 {
			resource := metric.ResourceMetrics().At(0).Resource()
			attrs := resource.Attributes()
			
			// Check for essential attributes
			serviceName, exists := attrs.Get("service.name")
			s.Assert().True(exists, "Missing service.name attribute")
			s.Assert().NotEmpty(serviceName.AsString())
			
			// For database metrics, check db.system
			if strings.HasPrefix(metric.Name(), "postgresql.") {
				dbSystem, exists := attrs.Get("db.system")
				s.Assert().True(exists, "Missing db.system attribute")
				s.Assert().Equal("postgresql", dbSystem.AsString())
			}
		}
	}
}

// TestComprehensiveSuite runs the comprehensive test suite
func TestComprehensiveSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive e2e tests in short mode")
	}
	
	suite.Run(t, new(ComprehensiveTestSuite))
}