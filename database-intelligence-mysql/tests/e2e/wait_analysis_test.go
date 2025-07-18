package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/database-intelligence/mysql-monitoring/tests/e2e/framework"
	"go.uber.org/zap"
)

// WaitAnalysisTestSuite validates wait-based monitoring functionality
type WaitAnalysisTestSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	validator *framework.MetricValidator
	logger    *zap.Logger
}

func TestWaitAnalysis(t *testing.T) {
	suite.Run(t, new(WaitAnalysisTestSuite))
}

func (s *WaitAnalysisTestSuite) SetupSuite() {
	s.logger, _ = zap.NewDevelopment()
	
	// Setup test environment
	s.env = framework.NewTestEnvironment()
	err := s.env.Setup()
	s.Require().NoError(err)
	
	// Initialize validator with gateway endpoint
	s.validator = framework.NewMetricValidator(s.logger, "http://localhost:9091/metrics")
	
	// Ensure Performance Schema is properly configured
	s.ensurePerformanceSchemaSetup()
	
	// Wait for collectors to initialize
	time.Sleep(20 * time.Second)
}

func (s *WaitAnalysisTestSuite) TearDownSuite() {
	s.env.Teardown()
}

func (s *WaitAnalysisTestSuite) TestWaitProfileCollection() {
	s.Run("Wait profile metrics should be collected", func() {
		// Generate workload with known wait patterns
		s.generateIOWaitWorkload()
		time.Sleep(15 * time.Second)
		
		// Verify wait profile metrics exist
		exists, err := s.validator.ValidateMetricExists("mysql_query_wait_profile")
		s.NoError(err)
		s.True(exists, "Wait profile metrics should exist")
		
		// Verify wait categories are present
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		s.Contains(metrics, `wait_category="io"`, "I/O wait category should be present")
		s.Contains(metrics, `wait_category="lock"`, "Lock wait category should be present")
		s.Contains(metrics, `wait_category="cpu"`, "CPU wait category should be present")
	})
}

func (s *WaitAnalysisTestSuite) TestWaitSeverityCalculation() {
	s.Run("Wait severity should be correctly calculated", func() {
		// Generate high-wait query
		s.generateHighWaitQuery()
		time.Sleep(15 * time.Second)
		
		// Check for critical severity
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		s.Contains(metrics, `wait_severity="critical"`, "Critical wait severity should be detected")
		s.Contains(metrics, `wait_severity="high"`, "High wait severity should be detected")
	})
}

func (s *WaitAnalysisTestSuite) TestAdvisoryGeneration() {
	s.Run("Advisories should be generated for performance issues", func() {
		// Generate queries that trigger advisories
		s.generateMissingIndexQuery()
		s.generateTempTableQuery()
		time.Sleep(20 * time.Second)
		
		// Check for advisories
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		// Verify missing index advisory
		s.Contains(metrics, `advisor_type="missing_index"`, "Missing index advisory should be generated")
		s.Contains(metrics, `advisor_recommendation=`, "Advisory should include recommendation")
		
		// Verify temp table advisory
		s.Contains(metrics, `advisor_type="temp_table_to_disk"`, "Temp table advisory should be generated")
		
		// Check advisory priorities
		s.Contains(metrics, `advisor_priority="P1"`, "P1 advisories should be generated for critical issues")
	})
}

func (s *WaitAnalysisTestSuite) TestBlockingDetection() {
	s.Run("Blocking chains should be detected", func() {
		// Create blocking scenario
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		var wg sync.WaitGroup
		wg.Add(2)
		
		// Start blocking transaction
		go func() {
			defer wg.Done()
			tx, err := s.env.GetDB().BeginTx(ctx, nil)
			if err != nil {
				s.logger.Error("Failed to start transaction", zap.Error(err))
				return
			}
			defer tx.Rollback()
			
			// Lock rows
			_, err = tx.Exec("UPDATE inventory SET reserved_quantity = reserved_quantity + 1 WHERE product_id IN (1,2,3) ORDER BY product_id")
			if err != nil {
				s.logger.Error("Failed to lock rows", zap.Error(err))
				return
			}
			
			// Hold lock for 10 seconds
			time.Sleep(10 * time.Second)
		}()
		
		// Start blocked transaction after a delay
		go func() {
			defer wg.Done()
			time.Sleep(2 * time.Second)
			
			tx, err := s.env.GetDB().BeginTx(ctx, nil)
			if err != nil {
				s.logger.Error("Failed to start blocked transaction", zap.Error(err))
				return
			}
			defer tx.Rollback()
			
			// Try to update same rows (will be blocked)
			_, err = tx.Exec("UPDATE inventory SET reserved_quantity = reserved_quantity + 1 WHERE product_id = 1")
			if err != nil {
				s.logger.Warn("Blocked query failed", zap.Error(err))
			}
		}()
		
		// Wait for blocking scenario to develop
		time.Sleep(5 * time.Second)
		
		// Verify blocking metrics
		exists, err := s.validator.ValidateMetricExists("mysql_blocking_active")
		s.NoError(err)
		s.True(exists, "Blocking metrics should exist")
		
		// Wait for transactions to complete
		wg.Wait()
	})
}

func (s *WaitAnalysisTestSuite) TestPlanChangeDetection() {
	s.Run("Plan changes should be detected", func() {
		// Get baseline execution stats
		s.executeQueryMultipleTimes("SELECT * FROM orders WHERE customer_id = 1", 10)
		time.Sleep(15 * time.Second)
		
		// Force a plan change by adding many rows
		s.addManyOrdersForCustomer(1, 10000)
		
		// Execute query again
		s.executeQueryMultipleTimes("SELECT * FROM orders WHERE customer_id = 1", 10)
		time.Sleep(15 * time.Second)
		
		// Check for plan fingerprint changes
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		s.Contains(metrics, "plan_fingerprint=", "Plan fingerprints should be generated")
	})
}

func (s *WaitAnalysisTestSuite) TestCompositeAdvisories() {
	s.Run("Composite advisories should detect complex issues", func() {
		// Create scenario for lock escalation due to missing index
		s.generateLockEscalationScenario()
		time.Sleep(20 * time.Second)
		
		// Verify composite advisory
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		s.Contains(metrics, `advisor_composite="lock_escalation_missing_index"`, 
			"Lock escalation due to missing index should be detected")
	})
}

func (s *WaitAnalysisTestSuite) TestWaitTrendAnalysis() {
	s.Run("Wait trends should be tracked", func() {
		// Execute query with increasing wait times
		for i := 0; i < 5; i++ {
			s.generateVariableWaitQuery(i * 100)
			time.Sleep(5 * time.Second)
		}
		
		// Check for regression detection
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		s.Contains(metrics, `wait_trend="regression"`, "Wait regression should be detected")
	})
}

func (s *WaitAnalysisTestSuite) TestSLIImpactDetection() {
	s.Run("SLI impacting queries should be identified", func() {
		// Generate slow queries
		s.generateSlowQuery(6000) // 6 second query
		time.Sleep(15 * time.Second)
		
		// Verify SLI impact detection
		metrics, err := s.validator.fetchPrometheusMetrics()
		s.NoError(err)
		
		s.Contains(metrics, `sli_impacting="true"`, "SLI impacting queries should be marked")
	})
}

// Helper methods

func (s *WaitAnalysisTestSuite) ensurePerformanceSchemaSetup() {
	queries := []string{
		"UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME LIKE 'wait/%'",
		"UPDATE performance_schema.setup_instruments SET ENABLED = 'YES', TIMED = 'YES' WHERE NAME LIKE 'statement/%'",
		"UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME LIKE '%waits%'",
		"UPDATE performance_schema.setup_consumers SET ENABLED = 'YES' WHERE NAME LIKE '%statements%'",
	}
	
	for _, query := range queries {
		_, err := s.env.GetDB().Exec(query)
		if err != nil {
			s.logger.Warn("Failed to configure Performance Schema", 
				zap.String("query", query), 
				zap.Error(err))
		}
	}
}

func (s *WaitAnalysisTestSuite) generateIOWaitWorkload() {
	// Use the stored procedure from initialization
	_, err := s.env.GetDB().Exec("USE wait_analysis_test")
	s.NoError(err)
	
	_, err = s.env.GetDB().Exec("CALL generate_io_waits()")
	if err != nil {
		s.logger.Warn("Failed to generate I/O waits", zap.Error(err))
	}
}

func (s *WaitAnalysisTestSuite) generateHighWaitQuery() {
	// Query that will have high wait percentage
	query := `
		SELECT COUNT(*) 
		FROM audit_log 
		WHERE created_at > DATE_SUB(NOW(), INTERVAL 1 YEAR)
		AND action = 'UPDATE'
	`
	_, err := s.env.GetDB().Query(query)
	if err != nil {
		s.logger.Warn("Failed to execute high wait query", zap.Error(err))
	}
}

func (s *WaitAnalysisTestSuite) generateMissingIndexQuery() {
	// Query on order_items without index on order_id
	query := `
		SELECT oi.*, o.order_date, o.status
		FROM order_items oi
		JOIN orders o ON oi.order_id = o.order_id
		WHERE o.order_date > DATE_SUB(NOW(), INTERVAL 30 DAY)
	`
	_, err := s.env.GetDB().Query(query)
	if err != nil {
		s.logger.Warn("Failed to execute missing index query", zap.Error(err))
	}
}

func (s *WaitAnalysisTestSuite) generateTempTableQuery() {
	_, err := s.env.GetDB().Exec("USE wait_analysis_test")
	s.NoError(err)
	
	_, err = s.env.GetDB().Exec("CALL generate_temp_table_waits()")
	if err != nil {
		s.logger.Warn("Failed to generate temp table waits", zap.Error(err))
	}
}

func (s *WaitAnalysisTestSuite) generateLockEscalationScenario() {
	// Update without index causes table lock
	query := `
		UPDATE order_items 
		SET discount = discount + 0.01
		WHERE unit_price > 50
	`
	_, err := s.env.GetDB().Exec(query)
	if err != nil {
		s.logger.Warn("Failed to generate lock escalation", zap.Error(err))
	}
}

func (s *WaitAnalysisTestSuite) executeQueryMultipleTimes(query string, times int) {
	for i := 0; i < times; i++ {
		_, err := s.env.GetDB().Query(query)
		if err != nil {
			s.logger.Warn("Query execution failed", zap.Error(err))
		}
	}
}

func (s *WaitAnalysisTestSuite) addManyOrdersForCustomer(customerID int, count int) {
	tx, err := s.env.GetDB().Begin()
	s.NoError(err)
	defer tx.Rollback()
	
	stmt, err := tx.Prepare("INSERT INTO orders (customer_id, total_amount) VALUES (?, ?)")
	s.NoError(err)
	defer stmt.Close()
	
	for i := 0; i < count; i++ {
		_, err = stmt.Exec(customerID, float64(i)*10.5)
		if err != nil {
			s.logger.Warn("Failed to insert order", zap.Error(err))
		}
	}
	
	err = tx.Commit()
	s.NoError(err)
}

func (s *WaitAnalysisTestSuite) generateVariableWaitQuery(delayMs int) {
	query := fmt.Sprintf("SELECT SLEEP(%f)", float64(delayMs)/1000)
	_, err := s.env.GetDB().Query(query)
	if err != nil {
		s.logger.Warn("Failed to execute variable wait query", zap.Error(err))
	}
}

func (s *WaitAnalysisTestSuite) generateSlowQuery(durationMs int) {
	query := fmt.Sprintf("SELECT SLEEP(%f)", float64(durationMs)/1000)
	_, err := s.env.GetDB().Query(query)
	if err != nil {
		s.logger.Warn("Failed to execute slow query", zap.Error(err))
	}
}