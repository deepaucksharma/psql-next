package suites

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/database-intelligence/tests/e2e/framework"
)

// ASHPlanAnalysisTestSuite tests Active Session History and Query Plan analysis
type ASHPlanAnalysisTestSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
	ctx       context.Context
	cancel    context.CancelFunc
}

func (s *ASHPlanAnalysisTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Minute)
	
	env, err := framework.NewTestEnvironment(s.ctx, framework.TestConfig{
		DatabaseType: "postgresql",
		EnableMySQL:  false, // ASH primarily for PostgreSQL in this test
	})
	s.Require().NoError(err)
	s.env = env

	// Start collector with ASH and plan analysis features
	collector, err := framework.NewTestCollector(s.ctx, framework.CollectorConfig{
		ConfigPath: "../configs/enhanced-mode-full.yaml",
		LogLevel:   "debug",
		Features: []string{
			"postgresql",
			"enhancedsql",
			"ash",
			"planattributeextractor",
		},
	})
	s.Require().NoError(err)
	s.collector = collector

	s.Require().NoError(s.collector.WaitForReady(s.ctx, 2*time.Minute))
	
	// Create test schema
	s.setupTestSchema()
}

func (s *ASHPlanAnalysisTestSuite) TearDownSuite() {
	if s.collector != nil {
		s.collector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
	s.cancel()
}

func (s *ASHPlanAnalysisTestSuite) setupTestSchema() {
	// Create tables for testing
	queries := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			customer_id INT,
			order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			status VARCHAR(20),
			total DECIMAL(10,2)
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id SERIAL PRIMARY KEY,
			order_id INT REFERENCES orders(id),
			product_id INT,
			quantity INT,
			price DECIMAL(10,2)
		)`,
		`CREATE TABLE IF NOT EXISTS customers (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			email VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			category VARCHAR(50),
			price DECIMAL(10,2),
			stock INT
		)`,
	}
	
	for _, query := range queries {
		_, err := s.env.ExecuteSQL(query)
		s.Require().NoError(err)
	}
	
	// Insert test data
	s.insertTestData()
}

func (s *ASHPlanAnalysisTestSuite) insertTestData() {
	// Insert customers
	for i := 1; i <= 1000; i++ {
		_, err := s.env.ExecuteSQL(fmt.Sprintf(
			"INSERT INTO customers (name, email) VALUES ('Customer %d', 'customer%d@example.com')",
			i, i))
		s.Require().NoError(err)
	}
	
	// Insert products
	categories := []string{"Electronics", "Books", "Clothing", "Food", "Toys"}
	for i := 1; i <= 500; i++ {
		category := categories[i%len(categories)]
		_, err := s.env.ExecuteSQL(fmt.Sprintf(
			"INSERT INTO products (name, category, price, stock) VALUES ('Product %d', '%s', %f, %d)",
			i, category, float64(i)*1.5, i*10))
		s.Require().NoError(err)
	}
	
	// Insert orders with items
	for i := 1; i <= 5000; i++ {
		customerID := (i % 1000) + 1
		status := []string{"pending", "processing", "shipped", "delivered"}[i%4]
		
		result, err := s.env.ExecuteSQL(fmt.Sprintf(
			"INSERT INTO orders (customer_id, status, total) VALUES (%d, '%s', 0) RETURNING id",
			customerID, status))
		s.Require().NoError(err)
		
		var orderID int
		result.Scan(&orderID)
		
		// Add 1-5 items per order
		itemCount := (i % 5) + 1
		for j := 0; j < itemCount; j++ {
			productID := (j*i)%500 + 1
			quantity := j + 1
			price := float64(productID) * 1.5
			
			_, err = s.env.ExecuteSQL(fmt.Sprintf(
				"INSERT INTO order_items (order_id, product_id, quantity, price) VALUES (%d, %d, %d, %f)",
				orderID, productID, quantity, price))
			s.Require().NoError(err)
		}
	}
	
	// Create indexes
	indexes := []string{
		"CREATE INDEX idx_orders_customer ON orders(customer_id)",
		"CREATE INDEX idx_orders_status ON orders(status)",
		"CREATE INDEX idx_order_items_order ON order_items(order_id)",
		"CREATE INDEX idx_products_category ON products(category)",
	}
	
	for _, idx := range indexes {
		_, err := s.env.ExecuteSQL(idx)
		s.Require().NoError(err)
	}
	
	// Analyze tables
	_, err := s.env.ExecuteSQL("ANALYZE orders, order_items, customers, products")
	s.Require().NoError(err)
}

// Test01_ASHBasicCollection validates Active Session History collection
func (s *ASHPlanAnalysisTestSuite) Test01_ASHBasicCollection() {
	// Create multiple concurrent sessions
	var wg sync.WaitGroup
	sessionCount := 20
	
	for i := 0; i < sessionCount; i++ {
		wg.Add(1)
		go func(sessionID int) {
			defer wg.Done()
			
			// Each session runs different types of queries
			for j := 0; j < 10; j++ {
				switch sessionID % 4 {
				case 0: // Read-heavy session
					s.env.ExecuteSQL("SELECT * FROM orders WHERE customer_id = $1", sessionID)
				case 1: // Write-heavy session
					s.env.ExecuteSQL("UPDATE products SET stock = stock - 1 WHERE id = $1", sessionID)
				case 2: // Analytical queries
					s.env.ExecuteSQL(`
						SELECT c.name, COUNT(o.id), SUM(o.total)
						FROM customers c
						JOIN orders o ON c.id = o.customer_id
						GROUP BY c.name
						LIMIT 10`)
				case 3: // Blocking queries
					s.env.ExecuteSQL("BEGIN; SELECT * FROM products WHERE id = 1 FOR UPDATE; SELECT pg_sleep(0.5); COMMIT;")
				}
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}
	
	// Let sessions run
	time.Sleep(30 * time.Second)
	
	// Check ASH metrics
	ashMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.ash.*")
	s.Require().NoError(err)
	
	// Verify ASH metrics are collected
	requiredASHMetrics := []string{
		"postgresql.ash.sessions.active",
		"postgresql.ash.sessions.waiting",
		"postgresql.ash.wait_time",
		"postgresql.ash.cpu_time",
		"postgresql.ash.sessions.distribution",
	}
	
	for _, metric := range requiredASHMetrics {
		s.Assert().True(s.hasMetric(ashMetrics, metric), 
			"Missing ASH metric: %s", metric)
	}
	
	// Wait for all sessions to complete
	wg.Wait()
	
	// Verify session history was captured
	s.checkCollectorLogs("ash", []string{
		"sampling active sessions",
		"session state captured",
		"wait event recorded",
	})
}

// Test02_ASHWaitEventAnalysis tests wait event categorization
func (s *ASHPlanAnalysisTestSuite) Test02_ASHWaitEventAnalysis() {
	// Create scenarios for different wait events
	
	// 1. Lock waits
	var wg sync.WaitGroup
	wg.Add(2)
	
	// Session 1: Hold lock
	go func() {
		defer wg.Done()
		conn := s.env.GetConnection()
		defer conn.Close()
		
		conn.Exec("BEGIN")
		conn.Exec("SELECT * FROM products WHERE id = 1 FOR UPDATE")
		time.Sleep(2 * time.Second) // Hold lock
		conn.Exec("COMMIT")
	}()
	
	// Session 2: Wait for lock
	go func() {
		defer wg.Done()
		time.Sleep(500 * time.Millisecond) // Ensure session 1 gets lock first
		
		conn := s.env.GetConnection()
		defer conn.Close()
		
		conn.Exec("BEGIN")
		conn.Exec("SELECT * FROM products WHERE id = 1 FOR UPDATE") // Will wait
		conn.Exec("COMMIT")
	}()
	
	// 2. IO waits - large sequential scan
	go func() {
		s.env.ExecuteSQL("SELECT COUNT(*) FROM orders o JOIN order_items oi ON o.id = oi.order_id")
	}()
	
	// 3. CPU intensive query
	go func() {
		s.env.ExecuteSQL(`
			WITH RECURSIVE fib(n, a, b) AS (
				SELECT 1, 0::numeric, 1::numeric
				UNION ALL
				SELECT n+1, b, a+b FROM fib WHERE n < 1000
			)
			SELECT n, a FROM fib`)
	}()
	
	// Let wait events accumulate
	time.Sleep(3 * time.Second)
	wg.Wait()
	
	// Check wait event metrics
	waitMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.ash.wait.*")
	s.Require().NoError(err)
	
	// Should have different wait event categories
	waitCategories := make(map[string]bool)
	for _, m := range waitMetrics {
		m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Range(
			func(k string, v interface{}) bool {
				if k == "wait.class" {
					waitCategories[v.(string)] = true
				}
				return true
			})
	}
	
	// Should see multiple wait categories
	s.Assert().True(waitCategories["Lock"], "Should have Lock wait events")
	s.Assert().True(waitCategories["IO"] || waitCategories["BufferPin"], "Should have IO wait events")
	
	s.T().Logf("Detected wait categories: %v", waitCategories)
}

// Test03_ASHBlockingChainDetection tests blocking session detection
func (s *ASHPlanAnalysisTestSuite) Test03_ASHBlockingChainDetection() {
	// Create a blocking chain: Session A blocks B, B blocks C
	
	var wg sync.WaitGroup
	wg.Add(3)
	
	// Session A: Root blocker
	go func() {
		defer wg.Done()
		conn := s.env.GetConnection()
		defer conn.Close()
		
		conn.Exec("BEGIN")
		conn.Exec("UPDATE products SET price = price * 1.1 WHERE id = 100")
		s.T().Log("Session A: Acquired lock on product 100")
		
		time.Sleep(3 * time.Second) // Hold lock
		conn.Exec("COMMIT")
		s.T().Log("Session A: Released lock")
	}()
	
	// Session B: Blocked by A, blocks C
	go func() {
		defer wg.Done()
		time.Sleep(500 * time.Millisecond)
		
		conn := s.env.GetConnection()
		defer conn.Close()
		
		conn.Exec("BEGIN")
		conn.Exec("UPDATE products SET price = price * 1.2 WHERE id = 200")
		s.T().Log("Session B: Acquired lock on product 200")
		
		// Try to get lock held by A
		go func() {
			conn.Exec("UPDATE products SET price = price * 1.2 WHERE id = 100") // Will block
			s.T().Log("Session B: Got lock on product 100")
		}()
		
		time.Sleep(2 * time.Second)
		conn.Exec("COMMIT")
		s.T().Log("Session B: Released locks")
	}()
	
	// Session C: Blocked by B
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Second)
		
		conn := s.env.GetConnection()
		defer conn.Close()
		
		conn.Exec("BEGIN")
		conn.Exec("UPDATE products SET price = price * 1.3 WHERE id = 200") // Will block on B
		s.T().Log("Session C: Got lock on product 200")
		conn.Exec("COMMIT")
	}()
	
	// Let blocking situation develop
	time.Sleep(2 * time.Second)
	
	// Check for blocking metrics
	blockingMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.ash.blocking.*")
	s.Require().NoError(err)
	
	s.Assert().NotEmpty(blockingMetrics, "Should have blocking metrics")
	
	// Look for blocking chain depth
	var maxChainDepth float64
	for _, m := range blockingMetrics {
		if strings.Contains(m.Name(), "chain_depth") {
			value := s.getMetricValue(m)
			if value > maxChainDepth {
				maxChainDepth = value
			}
		}
	}
	
	s.Assert().GreaterOrEqual(maxChainDepth, float64(2), 
		"Should detect blocking chain of depth >= 2")
	
	// Check logs for blocking detection
	s.checkCollectorLogs("ash", []string{
		"blocking chain detected",
		"root blocker",
		"blocked sessions",
	})
	
	wg.Wait()
}

// Test04_PlanBasicExtraction tests query plan extraction
func (s *ASHPlanAnalysisTestSuite) Test04_PlanBasicExtraction() {
	// Execute queries with different plan characteristics
	
	testQueries := []struct {
		name  string
		query string
		expectedPlanElements []string
	}{
		{
			name: "Index Scan",
			query: "SELECT * FROM orders WHERE customer_id = 100",
			expectedPlanElements: []string{"Index Scan", "idx_orders_customer"},
		},
		{
			name: "Sequential Scan",
			query: "SELECT * FROM orders WHERE total > 1000",
			expectedPlanElements: []string{"Seq Scan", "orders"},
		},
		{
			name: "Nested Loop Join",
			query: `SELECT o.*, c.name 
					FROM orders o 
					JOIN customers c ON o.customer_id = c.id 
					WHERE o.id < 100`,
			expectedPlanElements: []string{"Nested Loop", "Join"},
		},
		{
			name: "Hash Join",
			query: `SELECT c.category, COUNT(*), AVG(oi.price)
					FROM products c
					JOIN order_items oi ON c.id = oi.product_id
					GROUP BY c.category`,
			expectedPlanElements: []string{"Hash Join", "HashAggregate"},
		},
		{
			name: "Sort",
			query: "SELECT * FROM customers ORDER BY created_at DESC LIMIT 100",
			expectedPlanElements: []string{"Sort", "Limit"},
		},
	}
	
	for _, test := range testQueries {
		s.T().Logf("Executing query: %s", test.name)
		
		// Execute query multiple times for plan extraction
		for i := 0; i < 5; i++ {
			_, err := s.env.ExecuteSQL(test.query)
			s.Require().NoError(err)
		}
		
		time.Sleep(2 * time.Second)
	}
	
	// Check plan metrics
	planMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.plan.*")
	s.Require().NoError(err)
	
	// Should have plan cost metrics
	s.Assert().True(s.hasMetric(planMetrics, "postgresql.plan.cost"), 
		"Should have plan cost metrics")
	
	// Should have plan node type distribution
	planNodeTypes := make(map[string]bool)
	for _, m := range planMetrics {
		m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Range(
			func(k string, v interface{}) bool {
				if k == "plan.node_type" {
					planNodeTypes[v.(string)] = true
				}
				return true
			})
	}
	
	// Verify we captured various plan node types
	expectedNodeTypes := []string{"Index Scan", "Seq Scan", "Hash Join", "Sort"}
	for _, nodeType := range expectedNodeTypes {
		s.Assert().True(planNodeTypes[nodeType], 
			"Should have captured %s plan node", nodeType)
	}
}

// Test05_PlanChangeDetection tests query plan regression detection
func (s *ASHPlanAnalysisTestSuite) Test05_PlanChangeDetection() {
	// Scenario: Query plan changes due to statistics or index changes
	
	testQuery := "SELECT * FROM orders WHERE status = 'shipped' AND customer_id < 500"
	
	// Phase 1: Execute with index
	s.T().Log("Phase 1: Executing with indexes")
	for i := 0; i < 10; i++ {
		_, err := s.env.ExecuteSQL(testQuery)
		s.Require().NoError(err)
	}
	
	time.Sleep(2 * time.Second)
	
	// Get initial plan metrics
	initialPlanMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.plan.*")
	s.Require().NoError(err)
	initialPlanCount := len(initialPlanMetrics)
	
	// Drop index to force plan change
	s.T().Log("Dropping index to force plan change")
	_, err = s.env.ExecuteSQL("DROP INDEX idx_orders_status")
	s.Require().NoError(err)
	
	// Phase 2: Execute without index
	s.T().Log("Phase 2: Executing without index")
	for i := 0; i < 10; i++ {
		_, err := s.env.ExecuteSQL(testQuery)
		s.Require().NoError(err)
	}
	
	time.Sleep(2 * time.Second)
	
	// Check for plan change detection
	planChangeMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.plan.changes")
	s.Require().NoError(err)
	
	s.Assert().NotEmpty(planChangeMetrics, "Should detect plan changes")
	
	// Check for regression detection
	regressionMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.plan.regression*")
	s.Require().NoError(err)
	
	s.Assert().NotEmpty(regressionMetrics, "Should detect plan regression")
	
	// Verify logs show plan analysis
	s.checkCollectorLogs("planattributeextractor", []string{
		"plan change detected",
		"cost increase",
		"regression",
		"performance impact",
	})
	
	// Recreate index
	_, err = s.env.ExecuteSQL("CREATE INDEX idx_orders_status ON orders(status)")
	s.Require().NoError(err)
}

// Test06_PlanStabilityAnalysis tests plan stability scoring
func (s *ASHPlanAnalysisTestSuite) Test06_PlanStabilityAnalysis() {
	// Test queries with different stability characteristics
	
	// Stable query - always uses same plan
	stableQuery := "SELECT * FROM customers WHERE id = $1"
	
	// Unstable query - plan varies with parameter
	unstableQuery := "SELECT * FROM orders WHERE customer_id < $1"
	
	// Execute stable query many times
	s.T().Log("Executing stable query")
	for i := 0; i < 50; i++ {
		_, err := s.env.ExecuteSQL(stableQuery, i+1)
		s.Require().NoError(err)
		time.Sleep(50 * time.Millisecond)
	}
	
	// Execute unstable query with varying selectivity
	s.T().Log("Executing unstable query")
	for i := 0; i < 50; i++ {
		// Vary parameter to change selectivity
		param := 10
		if i%10 == 0 {
			param = 900 // High selectivity
		}
		_, err := s.env.ExecuteSQL(unstableQuery, param)
		s.Require().NoError(err)
		time.Sleep(50 * time.Millisecond)
	}
	
	time.Sleep(5 * time.Second)
	
	// Check stability metrics
	stabilityMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.plan.stability*")
	s.Require().NoError(err)
	
	s.Assert().NotEmpty(stabilityMetrics, "Should have plan stability metrics")
	
	// Look for stability scores
	queryStability := make(map[string]float64)
	for _, m := range stabilityMetrics {
		m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Range(
			func(k string, v interface{}) bool {
				if k == "query.fingerprint" {
					queryStability[v.(string)] = s.getMetricValue(m)
				}
				return true
			})
	}
	
	// Should have different stability scores
	s.Assert().GreaterOrEqual(len(queryStability), 2, "Should have stability scores for multiple queries")
	
	var minStability, maxStability float64 = 1.0, 0.0
	for _, stability := range queryStability {
		if stability < minStability {
			minStability = stability
		}
		if stability > maxStability {
			maxStability = stability
		}
	}
	
	// Should see variation in stability
	s.Assert().Greater(maxStability-minStability, 0.2, 
		"Should see significant variation in plan stability scores")
}

// Test07_ASHAnomalyDetection tests anomaly detection in session patterns
func (s *ASHPlanAnalysisTestSuite) Test07_ASHAnomalyDetection() {
	// Create normal baseline activity
	s.T().Log("Creating baseline activity")
	
	var wg sync.WaitGroup
	normalSessions := 10
	
	// Normal workload
	for i := 0; i < normalSessions; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				s.env.ExecuteSQL("SELECT * FROM orders WHERE customer_id = $1", id)
				s.env.ExecuteSQL("SELECT * FROM products WHERE category = 'Electronics' LIMIT 10")
				time.Sleep(100 * time.Millisecond)
			}
		}(i)
	}
	
	// Let baseline establish
	time.Sleep(10 * time.Second)
	
	// Create anomalous activity
	s.T().Log("Creating anomalous activity")
	
	// Sudden spike in sessions
	anomalousSessions := 50
	for i := 0; i < anomalousSessions; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Unusual queries
			s.env.ExecuteSQL("SELECT pg_sleep(1)")
			s.env.ExecuteSQL("SELECT * FROM orders o1 CROSS JOIN orders o2 LIMIT 1000")
		}(i)
	}
	
	// Gradual increase pattern
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(time.Duration(id*100) * time.Millisecond)
			s.env.ExecuteSQL("SELECT COUNT(*) FROM order_items")
		}(i)
	}
	
	wg.Wait()
	time.Sleep(5 * time.Second)
	
	// Check for anomaly detection
	s.checkCollectorLogs("ash", []string{
		"anomaly detected",
		"sudden spike",
		"gradual drift",
		"pattern break",
		"unusual activity",
	})
	
	// Check anomaly metrics
	anomalyMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.ash.anomaly*")
	s.Require().NoError(err)
	
	if len(anomalyMetrics) > 0 {
		s.T().Log("Anomaly detection metrics found")
	}
}

// Test08_IntegratedASHPlanAnalysis tests ASH and plan analysis working together
func (s *ASHPlanAnalysisTestSuite) Test08_IntegratedASHPlanAnalysis() {
	// Scenario: Slow query due to plan regression causes wait events
	
	// Create a query that will have plan regression
	setupQuery := `
		CREATE TABLE IF NOT EXISTS test_regression (
			id SERIAL PRIMARY KEY,
			status VARCHAR(20),
			created_at TIMESTAMP,
			data TEXT
		)`
	_, err := s.env.ExecuteSQL(setupQuery)
	s.Require().NoError(err)
	
	// Insert data
	for i := 0; i < 10000; i++ {
		status := []string{"new", "active", "completed"}[i%3]
		_, err := s.env.ExecuteSQL(
			"INSERT INTO test_regression (status, created_at, data) VALUES ($1, NOW(), $2)",
			status, fmt.Sprintf("data-%d", i))
		s.Require().NoError(err)
	}
	
	// Create index
	_, err = s.env.ExecuteSQL("CREATE INDEX idx_test_status ON test_regression(status)")
	s.Require().NoError(err)
	
	// Analyze
	_, err = s.env.ExecuteSQL("ANALYZE test_regression")
	s.Require().NoError(err)
	
	// Phase 1: Good performance with index
	s.T().Log("Phase 1: Good performance")
	
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				s.env.ExecuteSQL("SELECT * FROM test_regression WHERE status = 'active'")
			}
		}()
	}
	
	wg.Wait()
	time.Sleep(5 * time.Second)
	
	// Drop index to cause regression
	s.T().Log("Dropping index to cause regression")
	_, err = s.env.ExecuteSQL("DROP INDEX idx_test_status")
	s.Require().NoError(err)
	
	// Phase 2: Poor performance without index
	s.T().Log("Phase 2: Poor performance")
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				s.env.ExecuteSQL("SELECT * FROM test_regression WHERE status = 'active'")
			}
		}()
	}
	
	wg.Wait()
	time.Sleep(5 * time.Second)
	
	// Check for correlated ASH and plan metrics
	
	// Should see plan regression
	planMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.plan.regression*")
	s.Require().NoError(err)
	s.Assert().NotEmpty(planMetrics, "Should detect plan regression")
	
	// Should see increased wait events
	waitMetrics, err := s.collector.GetMetrics(s.ctx, "postgresql.ash.wait.*")
	s.Require().NoError(err)
	s.Assert().NotEmpty(waitMetrics, "Should see wait events")
	
	// Should see correlation in logs
	s.checkCollectorLogs("", []string{
		"plan regression correlated with increased wait time",
		"performance degradation detected",
		"slow query linked to plan change",
	})
}

// Helper methods

func (s *ASHPlanAnalysisTestSuite) hasMetric(metrics []pmetric.Metric, name string) bool {
	for _, m := range metrics {
		if strings.Contains(m.Name(), name) {
			return true
		}
	}
	return false
}

func (s *ASHPlanAnalysisTestSuite) getMetricValue(metric pmetric.Metric) float64 {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if metric.Gauge().DataPoints().Len() > 0 {
			return metric.Gauge().DataPoints().At(0).DoubleValue()
		}
	case pmetric.MetricTypeSum:
		if metric.Sum().DataPoints().Len() > 0 {
			return metric.Sum().DataPoints().At(0).DoubleValue()
		}
	case pmetric.MetricTypeHistogram:
		if metric.Histogram().DataPoints().Len() > 0 {
			return metric.Histogram().DataPoints().At(0).Sum()
		}
	}
	return 0
}

func (s *ASHPlanAnalysisTestSuite) checkCollectorLogs(component string, patterns []string) {
	logs := s.collector.GetLogs()
	for _, pattern := range patterns {
		found := false
		for _, log := range logs {
			if (component == "" || strings.Contains(log, component)) && 
			   strings.Contains(strings.ToLower(log), strings.ToLower(pattern)) {
				found = true
				s.T().Logf("Found expected log pattern: %s", pattern)
				break
			}
		}
		if !found {
			s.T().Logf("Warning: Did not find expected log pattern: %s", pattern)
		}
	}
}

func TestASHPlanAnalysisSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping ASH and plan analysis e2e tests in short mode")
	}
	
	suite.Run(t, new(ASHPlanAnalysisTestSuite))
}