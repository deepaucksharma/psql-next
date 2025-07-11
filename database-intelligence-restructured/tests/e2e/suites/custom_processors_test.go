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
)

// CustomProcessorsTestSuite tests all 7 custom processors
type CustomProcessorsTestSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
	ctx       context.Context
	cancel    context.CancelFunc
}

func (s *CustomProcessorsTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Minute)
	
	env, err := framework.NewTestEnvironment(s.ctx, framework.TestConfig{
		DatabaseType: "postgresql",
		EnableMySQL:  true,
	})
	s.Require().NoError(err)
	s.env = env

	// Start collector with enhanced mode (all custom processors)
	collector, err := framework.NewTestCollector(s.ctx, framework.CollectorConfig{
		ConfigPath: "../configs/enhanced-mode-full.yaml",
		LogLevel:   "debug",
		Features: []string{
			"adaptivesampler",
			"circuitbreaker",
			"planattributeextractor",
			"verification",
			"costcontrol",
			"nrerrormonitor",
			"querycorrelator",
		},
	})
	s.Require().NoError(err)
	s.collector = collector

	s.Require().NoError(s.collector.WaitForReady(s.ctx, 2*time.Minute))
}

func (s *CustomProcessorsTestSuite) TearDownSuite() {
	if s.collector != nil {
		s.collector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
	s.cancel()
}

// Test01_AdaptiveSampler tests dynamic sampling based on load
func (s *CustomProcessorsTestSuite) Test01_AdaptiveSampler() {
	// Phase 1: Low load - expect high sampling rate
	lowLoadWorkload := s.env.CreateWorkload("adaptive_low", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       1, // Low rate
		ConnectionCount: 2,
		Operations:      []string{"SELECT"},
	})
	
	err := lowLoadWorkload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(30 * time.Second)
	
	lowLoadMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.query.*")
	s.Require().NoError(err)
	lowLoadCount := len(lowLoadMetrics)

	// Phase 2: High load - expect lower sampling rate
	highLoadWorkload := s.env.CreateWorkload("adaptive_high", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       100, // High rate
		ConnectionCount: 20,
		Operations:      []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
	})
	
	err = highLoadWorkload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(30 * time.Second)
	
	highLoadMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.query.*")
	s.Require().NoError(err)
	
	// Calculate metrics per query ratio
	lowLoadRatio := float64(lowLoadCount) / float64(1*60) // queries per second
	highLoadRatio := float64(len(highLoadMetrics)-lowLoadCount) / float64(100*60)
	
	// High load should have lower sampling ratio
	s.Assert().Less(highLoadRatio, lowLoadRatio, 
		"Expected adaptive sampler to reduce sampling under high load")
	
	// Check for spike detection
	s.checkCollectorLogs("adaptive_sampler", []string{
		"spike detected",
		"adjusting sample rate",
		"load factor",
	})
}

// Test02_CircuitBreaker tests database protection mechanisms
func (s *CustomProcessorsTestSuite) Test02_CircuitBreaker() {
	// Create a workload that will stress the database
	stressWorkload := s.env.CreateWorkload("circuit_breaker", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       200, // Very high rate
		ConnectionCount: 100, // Many connections
		Operations: []string{
			"SELECT pg_sleep(1)", // Slow queries
			"SELECT * FROM generate_series(1, 1000000)", // Heavy queries
		},
	})
	
	// Monitor database CPU before stress
	initialCPU := s.getDatabaseCPU()
	
	// Start stress workload
	go stressWorkload.Run(s.ctx)
	
	// Wait for circuit breaker to potentially trigger
	time.Sleep(30 * time.Second)
	
	// Check if circuit breaker opened
	logs := s.collector.GetLogs()
	circuitOpened := false
	for _, log := range logs {
		if strings.Contains(log, "circuit breaker opened") ||
		   strings.Contains(log, "CircuitBreaker state: OPEN") {
			circuitOpened = true
			break
		}
	}
	
	// If database CPU exceeded threshold, circuit should open
	currentCPU := s.getDatabaseCPU()
	if currentCPU > 80 {
		s.Assert().True(circuitOpened, 
			"Expected circuit breaker to open when database CPU > 80%% (was %.1f%%)", currentCPU)
	}
	
	// Verify recovery
	time.Sleep(2 * time.Minute) // Wait for recovery
	
	// Circuit should transition to half-open or closed
	s.checkCollectorLogs("circuitbreaker", []string{
		"state: HALF_OPEN",
		"state: CLOSED",
		"circuit breaker recovery",
	})
}

// Test03_PlanAttributeExtractor tests query plan extraction and regression detection
func (s *CustomProcessorsTestSuite) Test03_PlanAttributeExtractor() {
	// Create a table with index
	_, err := s.env.ExecuteSQL("CREATE TABLE IF NOT EXISTS plan_test (id INT PRIMARY KEY, data TEXT)")
	s.Require().NoError(err)
	
	// Insert initial data
	for i := 0; i < 100; i++ {
		_, err = s.env.ExecuteSQL(fmt.Sprintf("INSERT INTO plan_test VALUES (%d, 'data%d')", i, i))
		s.Require().NoError(err)
	}
	
	// Phase 1: Queries using index
	indexWorkload := s.env.CreateWorkload("plan_indexed", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       5,
		ConnectionCount: 2,
		Operations: []string{
			"SELECT * FROM plan_test WHERE id = 1",
			"SELECT * FROM plan_test WHERE id BETWEEN 10 AND 20",
		},
	})
	
	err = indexWorkload.Run(s.ctx)
	s.Require().NoError(err)
	
	// Drop index to cause plan change
	_, err = s.env.ExecuteSQL("DROP INDEX IF EXISTS plan_test_pkey CASCADE")
	s.Require().NoError(err)
	
	// Phase 2: Same queries without index (plan regression)
	seqScanWorkload := s.env.CreateWorkload("plan_seqscan", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       5,
		ConnectionCount: 2,
		Operations: []string{
			"SELECT * FROM plan_test WHERE id = 1",
			"SELECT * FROM plan_test WHERE id BETWEEN 10 AND 20",
		},
	})
	
	err = seqScanWorkload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(30 * time.Second)
	
	// Check for plan metrics
	metrics, err := s.collector.GetMetrics(s.ctx, "postgresql.plan.*")
	s.Require().NoError(err)
	
	// Should have plan cost and change metrics
	foundPlanCost := false
	foundPlanChanges := false
	foundRegression := false
	
	for _, m := range metrics {
		switch m.Name() {
		case "postgresql.plan.cost":
			foundPlanCost = true
		case "postgresql.plan.changes":
			foundPlanChanges = true
		case "postgresql.plan.regression_detected":
			foundRegression = true
		}
	}
	
	s.Assert().True(foundPlanCost, "Expected plan cost metrics")
	s.Assert().True(foundPlanChanges, "Expected plan change metrics")
	s.Assert().True(foundRegression, "Expected plan regression detection")
	
	// Check logs for plan analysis
	s.checkCollectorLogs("planattributeextractor", []string{
		"plan change detected",
		"regression detected",
		"cost increase",
	})
}

// Test04_Verification tests PII detection and cardinality limits
func (s *CustomProcessorsTestSuite) Test04_Verification() {
	// Create test data with PII
	_, err := s.env.ExecuteSQL(`
		CREATE TABLE IF NOT EXISTS users_pii (
			id INT PRIMARY KEY,
			email TEXT,
			ssn TEXT,
			credit_card TEXT,
			name TEXT
		)`)
	s.Require().NoError(err)
	
	// Insert data with PII patterns
	piiData := []struct {
		email      string
		ssn        string
		creditCard string
		name       string
	}{
		{"john@example.com", "123-45-6789", "4111-1111-1111-1111", "John Doe"},
		{"jane@test.org", "987-65-4321", "5500 0000 0000 0004", "Jane Smith"},
	}
	
	for i, data := range piiData {
		_, err = s.env.ExecuteSQL(fmt.Sprintf(
			"INSERT INTO users_pii VALUES (%d, '%s', '%s', '%s', '%s')",
			i, data.email, data.ssn, data.creditCard, data.name))
		s.Require().NoError(err)
	}
	
	// Run queries that would expose PII
	piiWorkload := s.env.CreateWorkload("pii_test", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       5,
		ConnectionCount: 2,
		Operations: []string{
			"SELECT * FROM users_pii",
			"SELECT email, ssn, credit_card FROM users_pii WHERE id = 1",
		},
	})
	
	err = piiWorkload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(30 * time.Second)
	
	// Check that PII was detected and redacted
	metrics, err := s.collector.GetMetrics(s.ctx, "*")
	s.Require().NoError(err)
	
	// Verify no PII patterns in metric attributes
	for _, m := range metrics {
		m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Range(
			func(k string, v interface{}) bool {
				value := fmt.Sprintf("%v", v)
				s.Assert().NotContains(value, "@example.com", "Found unredacted email")
				s.Assert().NotRegexp(`\d{3}-\d{2}-\d{4}`, value, "Found unredacted SSN")
				s.Assert().NotRegexp(`\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`, value, "Found unredacted credit card")
				return true
			})
	}
	
	// Test cardinality limits
	// Create high cardinality scenario
	for i := 0; i < 20000; i++ {
		go func(id int) {
			s.env.ExecuteSQL(fmt.Sprintf("SELECT %d AS unique_id", id))
		}(i)
	}
	
	time.Sleep(1 * time.Minute)
	
	// Check that cardinality was limited
	s.checkCollectorLogs("verification", []string{
		"PII detected",
		"redacted",
		"cardinality limit exceeded",
		"dropping high cardinality",
	})
}

// Test05_CostControl tests budget management and prioritization
func (s *CustomProcessorsTestSuite) Test05_CostControl() {
	// Generate metrics at different priority levels
	
	// Critical metrics (connections, locks)
	criticalWorkload := s.env.CreateWorkload("critical_metrics", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       50,
		ConnectionCount: 20,
		Operations:      []string{"SELECT", "INSERT"},
	})
	
	// Low priority metrics (table stats)
	_, err := s.env.ExecuteSQL("ANALYZE") // Trigger table statistics
	s.Require().NoError(err)
	
	// Run workloads to exceed budget
	go criticalWorkload.Run(s.ctx)
	
	// Generate excessive metrics
	for i := 0; i < 100; i++ {
		tableName := fmt.Sprintf("temp_table_%d", i)
		s.env.ExecuteSQL(fmt.Sprintf("CREATE TABLE %s (id INT)", tableName))
		s.env.ExecuteSQL(fmt.Sprintf("INSERT INTO %s VALUES (1)", tableName))
		s.env.ExecuteSQL(fmt.Sprintf("SELECT * FROM %s", tableName))
	}
	
	time.Sleep(2 * time.Minute)
	
	// Verify critical metrics were preserved
	criticalMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.connections.*")
	s.Require().NoError(err)
	s.Assert().NotEmpty(criticalMetrics, "Critical metrics should be preserved")
	
	lockMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.locks.*")
	s.Require().NoError(err)
	s.Assert().NotEmpty(lockMetrics, "Lock metrics should be preserved")
	
	// Check that low priority metrics were dropped
	s.checkCollectorLogs("costcontrol", []string{
		"budget exceeded",
		"dropping low priority metrics",
		"cost threshold reached",
		"priority: low",
	})
	
	// Verify cost metrics are emitted
	costMetrics, err := s.collector.GetMetrics(s.ctx, "otelcol.costcontrol.*")
	s.Require().NoError(err)
	s.Assert().NotEmpty(costMetrics, "Expected cost control metrics")
}

// Test06_NRErrorMonitor tests New Relic integration error detection
func (s *CustomProcessorsTestSuite) Test06_NRErrorMonitor() {
	// Simulate various New Relic errors
	
	// 1. High cardinality error
	for i := 0; i < 100000; i++ {
		s.env.ExecuteSQL(fmt.Sprintf("SELECT %d AS unique_metric_%d", i, i))
	}
	
	// 2. Invalid metric names
	s.env.ExecuteSQL("SELECT 'value' AS \"metric with spaces\"")
	s.env.ExecuteSQL("SELECT 'value' AS \"metric.with..dots\"")
	
	// 3. Schema violations
	// Create metrics with invalid types
	s.collector.InjectMetric(framework.Metric{
		Name:  "invalid.metric.type",
		Value: "string_instead_of_number",
		Type:  "gauge",
	})
	
	time.Sleep(1 * time.Minute)
	
	// Check for error detection
	s.checkCollectorLogs("nrerrormonitor", []string{
		"NrIntegrationError detected",
		"cardinality limit",
		"schema violation",
		"invalid metric",
		"error pattern matched",
	})
	
	// Verify error metrics are emitted
	errorMetrics, err := s.collector.GetMetrics(s.ctx, "otelcol.nrerror.*")
	s.Require().NoError(err)
	s.Assert().NotEmpty(errorMetrics, "Expected NR error monitor metrics")
	
	// Check for self-healing actions
	s.checkCollectorLogs("nrerrormonitor", []string{
		"self-healing",
		"reducing batch size",
		"retry with backoff",
		"adjusted collection",
	})
}

// Test07_QueryCorrelator tests query and transaction correlation
func (s *CustomProcessorsTestSuite) Test07_QueryCorrelator() {
	// Create correlated transactions
	
	// Transaction 1: Multi-step process
	txn1 := s.env.BeginTransaction("order_processing")
	txn1.Execute("INSERT INTO orders (id, status) VALUES (1, 'pending')")
	txn1.Execute("UPDATE inventory SET quantity = quantity - 1 WHERE product_id = 100")
	txn1.Execute("INSERT INTO order_items (order_id, product_id) VALUES (1, 100)")
	txn1.Execute("UPDATE orders SET status = 'confirmed' WHERE id = 1")
	err := txn1.Commit()
	s.Require().NoError(err)
	
	// Transaction 2: Related queries in same session
	session := s.env.CreateSession("user_session_123")
	session.Execute("SELECT * FROM users WHERE id = 1")
	session.Execute("SELECT * FROM orders WHERE user_id = 1")
	session.Execute("SELECT * FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE user_id = 1)")
	session.Close()
	
	time.Sleep(30 * time.Second)
	
	// Check for correlation attributes
	metrics, err := s.collector.GetMetrics(s.ctx, "postgresql.query.*")
	s.Require().NoError(err)
	
	// Look for correlation attributes
	foundCorrelationID := false
	foundTransactionID := false
	foundSessionID := false
	
	for _, m := range metrics {
		m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Range(
			func(k string, v interface{}) bool {
				switch k {
				case "correlation.id":
					foundCorrelationID = true
				case "transaction.id":
					foundTransactionID = true
				case "session.id":
					foundSessionID = true
				}
				return true
			})
	}
	
	s.Assert().True(foundCorrelationID, "Expected correlation ID in metrics")
	s.Assert().True(foundTransactionID, "Expected transaction ID in metrics")
	s.Assert().True(foundSessionID, "Expected session ID in metrics")
	
	// Check for trace generation
	traces := s.collector.GetTraces()
	s.Assert().NotEmpty(traces, "Expected correlated traces to be generated")
	
	// Verify correlation depth
	s.checkCollectorLogs("querycorrelator", []string{
		"correlation established",
		"transaction flow captured",
		"correlation depth",
		"related queries found",
	})
}

// Test08_ProcessorChainIntegration tests all processors working together
func (s *CustomProcessorsTestSuite) Test08_ProcessorChainIntegration() {
	// Create a complex scenario that exercises all processors
	
	// High load with PII data and plan changes
	complexWorkload := s.env.CreateWorkload("processor_chain", framework.WorkloadConfig{
		Duration:        3 * time.Minute,
		QueryRate:       100,
		ConnectionCount: 50,
		Operations: []string{
			"SELECT * FROM users_pii WHERE email = 'test@example.com'",
			"INSERT INTO orders VALUES (random(), now())",
			"UPDATE inventory SET quantity = quantity - 1",
			"SELECT pg_sleep(0.5)", // Slow query
			"BEGIN; UPDATE accounts SET balance = balance - 100; COMMIT;",
		},
		TransactionSize:   5,
		ConcurrentUpdates: true,
	})
	
	// Create index then drop it mid-test
	_, err := s.env.ExecuteSQL("CREATE INDEX idx_test ON orders(id)")
	s.Require().NoError(err)
	
	go func() {
		time.Sleep(1 * time.Minute)
		s.env.ExecuteSQL("DROP INDEX idx_test")
	}()
	
	// Run complex workload
	err = complexWorkload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(1 * time.Minute)
	
	// Verify all processors functioned
	processorLogs := map[string][]string{
		"adaptive_sampler":       {"sample rate adjusted", "spike detected"},
		"circuitbreaker":        {"monitoring thresholds", "state: CLOSED"},
		"planattributeextractor": {"plan extracted", "cost calculated"},
		"verification":          {"PII detected", "cardinality checked"},
		"costcontrol":          {"budget monitored", "priority applied"},
		"nrerrormonitor":       {"monitoring errors", "no errors detected"},
		"querycorrelator":      {"correlation found", "transaction linked"},
	}
	
	for processor, expectedLogs := range processorLogs {
		for _, logPattern := range expectedLogs {
			s.Assert().True(
				s.findInLogs(processor, logPattern),
				"Expected %s to log: %s", processor, logPattern)
		}
	}
	
	// Verify metrics were processed correctly
	finalMetrics, err := s.collector.GetMetrics(s.ctx, "*")
	s.Require().NoError(err)
	s.Assert().NotEmpty(finalMetrics, "Expected processed metrics")
}

// Helper methods

func (s *CustomProcessorsTestSuite) getDatabaseCPU() float64 {
	result, err := s.env.ExecuteSQL(`
		SELECT 
			(SELECT count(*) FROM pg_stat_activity WHERE state = 'active') * 100.0 / 
			(SELECT current_setting('max_connections')::int) as cpu_estimate
	`)
	if err != nil {
		return 0
	}
	
	var cpu float64
	result.Scan(&cpu)
	return cpu
}

func (s *CustomProcessorsTestSuite) checkCollectorLogs(component string, patterns []string) {
	logs := s.collector.GetLogs()
	for _, pattern := range patterns {
		found := false
		for _, log := range logs {
			if strings.Contains(log, component) && strings.Contains(log, pattern) {
				found = true
				break
			}
		}
		s.Assert().True(found, "Expected %s logs to contain: %s", component, pattern)
	}
}

func (s *CustomProcessorsTestSuite) findInLogs(component, pattern string) bool {
	logs := s.collector.GetLogs()
	for _, log := range logs {
		if strings.Contains(log, component) && strings.Contains(log, pattern) {
			return true
		}
	}
	return false
}

func TestCustomProcessorsSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping custom processors e2e tests in short mode")
	}
	
	suite.Run(t, new(CustomProcessorsTestSuite))
}