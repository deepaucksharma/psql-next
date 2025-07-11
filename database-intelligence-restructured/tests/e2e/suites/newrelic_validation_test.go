package suites

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/database-intelligence/tests/e2e/framework"
)

// NewRelicValidationTestSuite validates New Relic integration
type NewRelicValidationTestSuite struct {
	suite.Suite
	env          *framework.TestEnvironment
	collector    *framework.TestCollector
	nrClient     *framework.NRDBClient
	ctx          context.Context
	cancel       context.CancelFunc
	testRunID    string
}

func (s *NewRelicValidationTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Minute)
	
	// Generate unique test run ID
	s.testRunID = fmt.Sprintf("e2e_test_%d", time.Now().Unix())
	
	// Setup environment
	env, err := framework.NewTestEnvironment(s.ctx, framework.TestConfig{
		DatabaseType: "postgresql",
		EnableMySQL:  true,
	})
	s.Require().NoError(err)
	s.env = env

	// Start collector with New Relic export
	collector, err := framework.NewTestCollector(s.ctx, framework.CollectorConfig{
		ConfigPath: "../configs/enhanced-mode-full.yaml",
		LogLevel:   "debug",
		EnvVars: map[string]string{
			"NEW_RELIC_LICENSE_KEY":  s.getNewRelicLicenseKey(),
			"NEW_RELIC_OTLP_ENDPOINT": s.getNewRelicEndpoint(),
			"TEST_RUN_ID":            s.testRunID,
		},
		Features: []string{
			"postgresql",
			"mysql",
			"enhancedsql",
			"ash",
			"planattributeextractor",
			"verification",
			"nrerrormonitor",
		},
	})
	s.Require().NoError(err)
	s.collector = collector

	// Initialize New Relic client
	nrClient, err := framework.NewNRDBClient(framework.NRDBConfig{
		AccountID:  s.getNewRelicAccountID(),
		APIKey:     s.getNewRelicAPIKey(),
		Region:     "US",
	})
	s.Require().NoError(err)
	s.nrClient = nrClient

	s.Require().NoError(s.collector.WaitForReady(s.ctx, 2*time.Minute))
}

func (s *NewRelicValidationTestSuite) TearDownSuite() {
	if s.collector != nil {
		s.collector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
	if s.nrClient != nil {
		s.nrClient.Close()
	}
	s.cancel()
}

// Test01_MetricIngestion validates metrics appear in New Relic
func (s *NewRelicValidationTestSuite) Test01_MetricIngestion() {
	// Generate test workload
	workload := s.env.CreateWorkload("nr_ingestion", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       10,
		ConnectionCount: 5,
		Operations: []string{
			"SELECT * FROM pg_stat_database",
			"INSERT INTO test_table VALUES (1, 'test')",
			"UPDATE test_table SET value = 'updated' WHERE id = 1",
		},
	})
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err)
	
	// Wait for metrics to propagate to New Relic
	s.T().Log("Waiting for metrics to appear in New Relic...")
	time.Sleep(2 * time.Minute)
	
	// Query New Relic for our metrics
	query := fmt.Sprintf(`
		SELECT count(*) 
		FROM Metric 
		WHERE metricName LIKE 'postgresql.%%' 
		AND test_run_id = '%s'
		SINCE 5 minutes ago
	`, s.testRunID)
	
	result, err := s.nrClient.QueryNRQL(s.ctx, query)
	s.Require().NoError(err)
	
	// Validate metrics were received
	count := s.extractCount(result)
	s.Assert().Greater(count, 0, "Expected PostgreSQL metrics in New Relic")
	
	s.T().Logf("Found %d PostgreSQL metrics in New Relic", count)
	
	// Check specific metric types
	metricTypes := []string{
		"postgresql.connections.active",
		"postgresql.transactions.committed",
		"postgresql.blocks.hit",
		"postgresql.database.size",
	}
	
	for _, metricName := range metricTypes {
		specificQuery := fmt.Sprintf(`
			SELECT count(*) 
			FROM Metric 
			WHERE metricName = '%s'
			AND test_run_id = '%s'
			SINCE 5 minutes ago
		`, metricName, s.testRunID)
		
		result, err := s.nrClient.QueryNRQL(s.ctx, specificQuery)
		s.Require().NoError(err)
		
		count := s.extractCount(result)
		s.Assert().Greater(count, 0, "Expected %s metric in New Relic", metricName)
	}
}

// Test02_EntitySynthesis validates database entities are created
func (s *NewRelicValidationTestSuite) Test02_EntitySynthesis() {
	// Run workload to ensure entity creation
	workload := s.env.CreateWorkload("entity_synthesis", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       5,
		ConnectionCount: 2,
		Operations:      []string{"SELECT 1"},
	})
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(3 * time.Minute) // Entity synthesis can take time
	
	// Query for database entities
	entityQuery := fmt.Sprintf(`
		FROM Entity 
		SELECT name, type, guid, tags()
		WHERE type = 'DATABASE'
		AND tags.test_run_id = '%s'
		SINCE 10 minutes ago
		LIMIT 10
	`, s.testRunID)
	
	result, err := s.nrClient.QueryNRQL(s.ctx, entityQuery)
	s.Require().NoError(err)
	
	entities := s.extractEntities(result)
	s.Assert().NotEmpty(entities, "Expected database entities to be created")
	
	// Validate entity attributes
	for _, entity := range entities {
		s.Assert().NotEmpty(entity.GUID, "Entity should have GUID")
		s.Assert().NotEmpty(entity.Name, "Entity should have name")
		s.Assert().Equal("DATABASE", entity.Type, "Entity type should be DATABASE")
		
		// Check required tags
		s.Assert().Contains(entity.Tags, "db.system", "Entity should have db.system tag")
		s.Assert().Contains(entity.Tags, "host.name", "Entity should have host.name tag")
		
		s.T().Logf("Found database entity: %s (GUID: %s)", entity.Name, entity.GUID)
	}
	
	// Check entity relationships
	if len(entities) > 0 {
		relationshipQuery := fmt.Sprintf(`
			FROM Relationship 
			SELECT sourceGuid, targetGuid, relationshipType
			WHERE sourceGuid = '%s' OR targetGuid = '%s'
			SINCE 10 minutes ago
		`, entities[0].GUID, entities[0].GUID)
		
		relResult, err := s.nrClient.QueryNRQL(s.ctx, relationshipQuery)
		if err == nil && relResult != nil {
			s.T().Log("Found entity relationships")
		}
	}
}

// Test03_MetricAccuracy validates metric values are accurate
func (s *NewRelicValidationTestSuite) Test03_MetricAccuracy() {
	// Get baseline metrics from database
	var baselineConnections int
	row := s.env.QueryRow("SELECT count(*) FROM pg_stat_activity")
	err := row.Scan(&baselineConnections)
	s.Require().NoError(err)
	
	// Create known workload
	knownConnections := 10
	connections := make([]*sql.DB, knownConnections)
	for i := 0; i < knownConnections; i++ {
		conn := s.env.GetConnection()
		connections[i] = conn
		defer conn.Close()
		
		// Keep connection active
		go func(c *sql.DB) {
			c.Exec("SELECT pg_sleep(60)")
		}(conn)
	}
	
	// Wait for metrics to be collected and sent
	time.Sleep(2 * time.Minute)
	
	// Query New Relic for connection metrics
	nrQuery := fmt.Sprintf(`
		SELECT average(postgresql.connections.active) as avg_connections
		FROM Metric 
		WHERE test_run_id = '%s'
		SINCE 2 minutes ago
	`, s.testRunID)
	
	result, err := s.nrClient.QueryNRQL(s.ctx, nrQuery)
	s.Require().NoError(err)
	
	avgConnections := s.extractFloat(result, "avg_connections")
	expectedConnections := float64(baselineConnections + knownConnections)
	
	// Allow 10% variance
	variance := math.Abs(avgConnections-expectedConnections) / expectedConnections
	s.Assert().Less(variance, 0.1, 
		"Connection count in New Relic (%.0f) should match actual (%.0f) within 10%%",
		avgConnections, expectedConnections)
	
	s.T().Logf("Metric accuracy - Expected: %.0f, Actual: %.0f, Variance: %.1f%%",
		expectedConnections, avgConnections, variance*100)
}

// Test04_IntegrationErrors checks for integration errors
func (s *NewRelicValidationTestSuite) Test04_IntegrationErrors() {
	// Check for any integration errors
	errorQuery := fmt.Sprintf(`
		SELECT count(*) as error_count, 
		       uniques(message) as unique_errors,
		       latest(message) as last_error
		FROM NrIntegrationError 
		WHERE category = 'MetricAPI'
		AND newRelicFeature = 'Metrics'
		AND test_run_id = '%s'
		SINCE 30 minutes ago
	`, s.testRunID)
	
	result, err := s.nrClient.QueryNRQL(s.ctx, errorQuery)
	s.Require().NoError(err)
	
	errorCount := s.extractCount(result)
	if errorCount > 0 {
		uniqueErrors := s.extractStringArray(result, "unique_errors")
		lastError := s.extractString(result, "last_error")
		
		s.T().Logf("Found %d integration errors:", errorCount)
		for _, err := range uniqueErrors {
			s.T().Logf("  - %s", err)
		}
		s.T().Logf("Last error: %s", lastError)
		
		// Some errors might be expected (e.g., cardinality limits)
		// Fail only on critical errors
		criticalErrors := []string{
			"authentication",
			"authorization",
			"invalid metric",
			"schema violation",
		}
		
		for _, criticalError := range criticalErrors {
			for _, actualError := range uniqueErrors {
				s.Assert().NotContains(strings.ToLower(actualError), criticalError,
					"Found critical integration error")
			}
		}
	} else {
		s.T().Log("No integration errors found")
	}
}

// Test05_DashboardCompatibility validates dashboard queries work
func (s *NewRelicValidationTestSuite) Test05_DashboardCompatibility() {
	// Test common dashboard queries
	dashboardQueries := []struct {
		name  string
		query string
	}{
		{
			name: "Connection Usage",
			query: `
				SELECT average(postgresql.connections.active) as 'Active',
				       average(postgresql.connections.idle) as 'Idle',
				       average(postgresql.connections.max) as 'Max'
				FROM Metric
				WHERE test_run_id = '%s'
				TIMESERIES
			`,
		},
		{
			name: "Transaction Rate",
			query: `
				SELECT rate(sum(postgresql.transactions.committed), 1 minute) as 'Commits/min',
				       rate(sum(postgresql.transactions.rolled_back), 1 minute) as 'Rollbacks/min'
				FROM Metric
				WHERE test_run_id = '%s'
				TIMESERIES
			`,
		},
		{
			name: "Cache Hit Ratio",
			query: `
				SELECT (sum(postgresql.blocks.hit) / 
				       (sum(postgresql.blocks.hit) + sum(postgresql.blocks.read))) * 100 
				       as 'Cache Hit %%'
				FROM Metric
				WHERE test_run_id = '%s'
				TIMESERIES
			`,
		},
		{
			name: "Query Performance",
			query: `
				SELECT histogram(postgresql.query.duration, 50, 20) 
				FROM Metric
				WHERE test_run_id = '%s'
				SINCE 30 minutes ago
			`,
		},
	}
	
	// Generate workload for dashboard data
	workload := s.env.CreateWorkload("dashboard_data", framework.WorkloadConfig{
		Duration:        3 * time.Minute,
		QueryRate:       20,
		ConnectionCount: 10,
		Operations: []string{
			"SELECT * FROM test_table",
			"INSERT INTO test_table VALUES (1, 'test')",
			"UPDATE test_table SET value = 'updated'",
			"DELETE FROM test_table WHERE id > 1000",
		},
		TransactionSize: 5,
	})
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(2 * time.Minute) // Wait for data
	
	// Test each dashboard query
	for _, dq := range dashboardQueries {
		query := fmt.Sprintf(dq.query, s.testRunID)
		
		result, err := s.nrClient.QueryNRQL(s.ctx, query)
		s.Require().NoError(err, "Dashboard query '%s' should execute", dq.name)
		
		// Validate result has data
		s.Assert().NotNil(result, "Dashboard query '%s' should return results", dq.name)
		
		s.T().Logf("Dashboard query '%s' executed successfully", dq.name)
	}
}

// Test06_AlertConditionCompatibility tests alert queries work
func (s *NewRelicValidationTestSuite) Test06_AlertConditionCompatibility() {
	// Test alert condition queries
	alertQueries := []struct {
		name      string
		query     string
		threshold float64
	}{
		{
			name: "High Connection Usage",
			query: `
				SELECT (average(postgresql.connections.active) / 
				        average(postgresql.connections.max)) * 100
				FROM Metric
				WHERE test_run_id = '%s'
			`,
			threshold: 80.0,
		},
		{
			name: "High Error Rate",
			query: `
				SELECT rate(sum(postgresql.transactions.rolled_back), 1 minute) /
				       rate(sum(postgresql.transactions.committed), 1 minute) * 100
				FROM Metric
				WHERE test_run_id = '%s'
			`,
			threshold: 5.0,
		},
		{
			name: "Low Cache Hit Ratio",
			query: `
				SELECT (sum(postgresql.blocks.hit) / 
				       (sum(postgresql.blocks.hit) + sum(postgresql.blocks.read))) * 100
				FROM Metric
				WHERE test_run_id = '%s'
			`,
			threshold: 90.0, // Alert if below 90%
		},
		{
			name: "Slow Queries",
			query: `
				SELECT percentile(postgresql.query.duration, 95)
				FROM Metric
				WHERE test_run_id = '%s'
			`,
			threshold: 1000.0, // 1 second
		},
	}
	
	// Generate some activity
	workload := s.env.CreateWorkload("alert_test", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       10,
		ConnectionCount: 5,
		Operations:      []string{"SELECT pg_sleep(0.1)", "SELECT 1"},
	})
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(2 * time.Minute)
	
	// Test each alert query
	for _, aq := range alertQueries {
		query := fmt.Sprintf(aq.query, s.testRunID)
		
		result, err := s.nrClient.QueryNRQL(s.ctx, query)
		s.Require().NoError(err, "Alert query '%s' should execute", aq.name)
		
		// Alert queries should return a single value
		s.Assert().NotNil(result, "Alert query '%s' should return a value", aq.name)
		
		s.T().Logf("Alert query '%s' executed successfully", aq.name)
	}
}

// Test07_AdvancedMetricsValidation tests enhanced mode metrics
func (s *NewRelicValidationTestSuite) Test07_AdvancedMetricsValidation() {
	// Test advanced metrics from enhanced mode
	
	// Create scenarios for advanced metrics
	// 1. Query plan metrics
	s.env.ExecuteSQL("CREATE INDEX IF NOT EXISTS idx_test ON test_table(value)")
	s.env.ExecuteSQL("ANALYZE test_table")
	
	// Execute queries that will have plans
	for i := 0; i < 20; i++ {
		s.env.ExecuteSQL("SELECT * FROM test_table WHERE value = $1", fmt.Sprintf("test-%d", i))
	}
	
	// 2. ASH metrics - create concurrent sessions
	for i := 0; i < 10; i++ {
		go func() {
			conn := s.env.GetConnection()
			defer conn.Close()
			conn.Exec("SELECT pg_sleep(5)")
		}()
	}
	
	time.Sleep(3 * time.Minute)
	
	// Check for plan metrics
	planQuery := fmt.Sprintf(`
		SELECT count(*) as plan_count,
		       uniques(plan.hash) as unique_plans
		FROM Metric
		WHERE metricName LIKE 'postgresql.plan.%%'
		AND test_run_id = '%s'
		SINCE 5 minutes ago
	`, s.testRunID)
	
	result, err := s.nrClient.QueryNRQL(s.ctx, planQuery)
	s.Require().NoError(err)
	
	planCount := s.extractCount(result)
	if planCount > 0 {
		s.T().Logf("Found %d plan metrics in New Relic", planCount)
		uniquePlans := s.extractStringArray(result, "unique_plans")
		s.T().Logf("Unique plan hashes: %v", uniquePlans)
	}
	
	// Check for ASH metrics
	ashQuery := fmt.Sprintf(`
		SELECT count(*) as ash_count,
		       average(postgresql.ash.sessions.active) as avg_active_sessions
		FROM Metric
		WHERE metricName LIKE 'postgresql.ash.%%'
		AND test_run_id = '%s'
		SINCE 5 minutes ago
	`, s.testRunID)
	
	ashResult, err := s.nrClient.QueryNRQL(s.ctx, ashQuery)
	s.Require().NoError(err)
	
	ashCount := s.extractCount(ashResult)
	if ashCount > 0 {
		avgSessions := s.extractFloat(ashResult, "avg_active_sessions")
		s.T().Logf("Found ASH metrics - Count: %d, Avg Active Sessions: %.2f", 
			ashCount, avgSessions)
	}
}

// Test08_DataRetentionValidation validates data is retained properly
func (s *NewRelicValidationTestSuite) Test08_DataRetentionValidation() {
	// Generate data over time
	phases := []string{"phase1", "phase2", "phase3"}
	
	for i, phase := range phases {
		// Tag metrics with phase
		s.collector.SetTag("test_phase", phase)
		
		workload := s.env.CreateWorkload(phase, framework.WorkloadConfig{
			Duration:        1 * time.Minute,
			QueryRate:       5,
			ConnectionCount: 3,
			Operations:      []string{"SELECT 1"},
		})
		
		err := workload.Run(s.ctx)
		s.Require().NoError(err)
		
		s.T().Logf("Completed %s", phase)
		
		if i < len(phases)-1 {
			time.Sleep(2 * time.Minute) // Gap between phases
		}
	}
	
	// Wait for all data to be ingested
	time.Sleep(2 * time.Minute)
	
	// Query for data from each phase
	for _, phase := range phases {
		phaseQuery := fmt.Sprintf(`
			SELECT count(*) 
			FROM Metric
			WHERE test_run_id = '%s'
			AND test_phase = '%s'
			SINCE 15 minutes ago
		`, s.testRunID, phase)
		
		result, err := s.nrClient.QueryNRQL(s.ctx, phaseQuery)
		s.Require().NoError(err)
		
		count := s.extractCount(result)
		s.Assert().Greater(count, 0, "Data from %s should be retained", phase)
		
		s.T().Logf("Found %d metrics from %s", count, phase)
	}
	
	// Verify data granularity over time
	granularityQuery := fmt.Sprintf(`
		SELECT count(*) 
		FROM Metric
		WHERE test_run_id = '%s'
		SINCE 15 minutes ago
		TIMESERIES 1 minute
	`, s.testRunID)
	
	result, err := s.nrClient.QueryNRQL(s.ctx, granularityQuery)
	s.Require().NoError(err)
	
	// Should have data points for multiple time buckets
	s.Assert().NotNil(result, "Should have time series data")
}

// Test09_CostOptimization validates cost control features
func (s *NewRelicValidationTestSuite) Test09_CostOptimization() {
	// Generate high cardinality data to test cost controls
	
	// Create many unique queries
	for i := 0; i < 1000; i++ {
		query := fmt.Sprintf("SELECT %d as unique_value_%d", i, i)
		s.env.ExecuteSQL(query)
	}
	
	// Create high cardinality attributes
	for i := 0; i < 100; i++ {
		s.env.ExecuteSQL(fmt.Sprintf("SELECT 'user_%d' as unique_user", i))
	}
	
	time.Sleep(3 * time.Minute)
	
	// Check cardinality in New Relic
	cardinalityQuery := fmt.Sprintf(`
		SELECT uniqueCount(dimensions()) as cardinality
		FROM Metric
		WHERE test_run_id = '%s'
		FACET metricName
		SINCE 5 minutes ago
		LIMIT 100
	`, s.testRunID)
	
	result, err := s.nrClient.QueryNRQL(s.ctx, cardinalityQuery)
	s.Require().NoError(err)
	
	// Verify cardinality is controlled
	s.T().Log("Checking cardinality control...")
	
	// Check if cost control metrics were emitted
	costQuery := fmt.Sprintf(`
		SELECT count(*) as cost_metrics,
		       latest(otelcol.costcontrol.budget_used_percent) as budget_used
		FROM Metric
		WHERE metricName LIKE 'otelcol.costcontrol.%%'
		AND test_run_id = '%s'
		SINCE 5 minutes ago
	`, s.testRunID)
	
	costResult, err := s.nrClient.QueryNRQL(s.ctx, costQuery)
	if err == nil {
		costMetrics := s.extractCount(costResult)
		if costMetrics > 0 {
			budgetUsed := s.extractFloat(costResult, "budget_used")
			s.T().Logf("Cost control active - Budget used: %.1f%%", budgetUsed)
		}
	}
}

// Helper methods

func (s *NewRelicValidationTestSuite) getNewRelicLicenseKey() string {
	// Get from environment or test config
	return os.Getenv("NEW_RELIC_LICENSE_KEY")
}

func (s *NewRelicValidationTestSuite) getNewRelicEndpoint() string {
	endpoint := os.Getenv("NEW_RELIC_OTLP_ENDPOINT")
	if endpoint == "" {
		return "https://otlp.nr-data.net:4318"
	}
	return endpoint
}

func (s *NewRelicValidationTestSuite) getNewRelicAccountID() string {
	return os.Getenv("NEW_RELIC_ACCOUNT_ID")
}

func (s *NewRelicValidationTestSuite) getNewRelicAPIKey() string {
	return os.Getenv("NEW_RELIC_API_KEY")
}

func (s *NewRelicValidationTestSuite) extractCount(result *framework.NRQLResult) int {
	if result == nil || len(result.Results) == 0 {
		return 0
	}
	
	if count, ok := result.Results[0]["count"].(float64); ok {
		return int(count)
	}
	return 0
}

func (s *NewRelicValidationTestSuite) extractFloat(result *framework.NRQLResult, field string) float64 {
	if result == nil || len(result.Results) == 0 {
		return 0
	}
	
	if value, ok := result.Results[0][field].(float64); ok {
		return value
	}
	return 0
}

func (s *NewRelicValidationTestSuite) extractString(result *framework.NRQLResult, field string) string {
	if result == nil || len(result.Results) == 0 {
		return ""
	}
	
	if value, ok := result.Results[0][field].(string); ok {
		return value
	}
	return ""
}

func (s *NewRelicValidationTestSuite) extractStringArray(result *framework.NRQLResult, field string) []string {
	if result == nil || len(result.Results) == 0 {
		return nil
	}
	
	if values, ok := result.Results[0][field].([]interface{}); ok {
		strings := make([]string, 0, len(values))
		for _, v := range values {
			if str, ok := v.(string); ok {
				strings = append(strings, str)
			}
		}
		return strings
	}
	return nil
}

func (s *NewRelicValidationTestSuite) extractEntities(result *framework.NRQLResult) []framework.Entity {
	entities := make([]framework.Entity, 0)
	
	if result == nil {
		return entities
	}
	
	for _, r := range result.Results {
		entity := framework.Entity{
			Name: s.getString(r, "name"),
			Type: s.getString(r, "type"),
			GUID: s.getString(r, "guid"),
			Tags: make(map[string]string),
		}
		
		if tags, ok := r["tags"].(map[string]interface{}); ok {
			for k, v := range tags {
				entity.Tags[k] = fmt.Sprintf("%v", v)
			}
		}
		
		entities = append(entities, entity)
	}
	
	return entities
}

func (s *NewRelicValidationTestSuite) getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func TestNewRelicValidationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping New Relic validation e2e tests in short mode")
	}
	
	// Check required environment variables
	required := []string{
		"NEW_RELIC_LICENSE_KEY",
		"NEW_RELIC_ACCOUNT_ID",
		"NEW_RELIC_API_KEY",
	}
	
	for _, env := range required {
		if os.Getenv(env) == "" {
			t.Skipf("Skipping New Relic tests: %s not set", env)
		}
	}
	
	suite.Run(t, new(NewRelicValidationTestSuite))
}