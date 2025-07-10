package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/database-intelligence/tests/e2e/framework"
)

// OTLPFormatValidationSuite tests OTLP format compliance and dimensional metrics
type OTLPFormatValidationSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
	nrdb      *framework.NRDBClient
	ctx       context.Context
	cancel    context.CancelFunc
}

func TestOTLPFormatValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OTLP format validation tests in short mode")
	}

	suite.Run(t, new(OTLPFormatValidationSuite))
}

func (s *OTLPFormatValidationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Initialize environment
	s.env = framework.NewTestEnvironment()
	require.NoError(s.T(), s.env.Initialize())

	// Verify New Relic credentials are available
	require.NotEmpty(s.T(), s.env.NewRelicAccountID, "NEW_RELIC_ACCOUNT_ID must be set")
	require.NotEmpty(s.T(), s.env.NewRelicAPIKey, "NEW_RELIC_API_KEY must be set")
	require.NotEmpty(s.T(), s.env.NewRelicLicenseKey, "NEW_RELIC_LICENSE_KEY must be set")

	// Initialize NRDB client
	s.nrdb = framework.NewNRDBClient(s.env.NewRelicAccountID, s.env.NewRelicAPIKey)

	// Initialize collector with OTLP-focused configuration
	s.collector = framework.NewTestCollector(s.env)

	// Start collector with OTLP validation config
	config := s.getOTLPValidationConfig()
	require.NoError(s.T(), s.collector.Start(config))

	// Setup test schema for dimensional testing
	s.setupOTLPTestSchema()

	// Wait for collector to stabilize
	time.Sleep(15 * time.Second)
}

func (s *OTLPFormatValidationSuite) TearDownSuite() {
	s.cancel()

	// Cleanup test data
	s.cleanupOTLPTestData()

	// Stop collector
	if s.collector != nil {
		s.collector.Stop()
	}

	// Cleanup environment
	if s.env != nil {
		s.env.Cleanup()
	}
}

// Test 1: OTLP Dimensional Metrics Schema Validation
func (s *OTLPFormatValidationSuite) TestOTLPDimensionalMetricsSchema() {
	s.T().Log("Testing OTLP dimensional metrics schema compliance...")

	// Generate database activity with known dimensional patterns
	s.generateDimensionalTestWorkload()

	// Wait for metrics collection cycle
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test core dimensional attributes for database metrics
	expectedDimensions := map[string][]string{
		"db.system":          {"postgresql"},
		"db.operation":       {"SELECT", "INSERT", "UPDATE"},
		"service.name":       {"database-intelligence-collector"},
		"environment":        {"otlp-test"},
		"telemetry.sdk.name": {"opentelemetry"},
	}

	// Verify dimensional integrity for key metrics
	keyMetrics := []string{
		"postgresql.backends",
		"postgresql.database.size",
		"postgresql.commits",
		"postgresql.rollbacks",
	}

	for _, metricName := range keyMetrics {
		for dimensionKey, expectedValues := range expectedDimensions {
			for _, expectedValue := range expectedValues {
				err := s.nrdb.VerifyMetric(ctx, metricName, map[string]interface{}{
					dimensionKey: expectedValue,
				}, "10 minutes ago")

				// Core dimensions should always be present
				if dimensionKey == "db.system" || dimensionKey == "service.name" {
					assert.NoError(s.T(), err, "Core dimension %s=%s should exist for metric %s",
						dimensionKey, expectedValue, metricName)
				}
			}
		}
	}

	s.T().Log("✓ OTLP dimensional metrics schema validation passed")
}

// Test 2: OpenTelemetry Semantic Conventions Compliance
func (s *OTLPFormatValidationSuite) TestSemanticConventionsCompliance() {
	s.T().Log("Testing OpenTelemetry semantic conventions compliance...")

	// Generate activity for semantic conventions testing
	s.generateSemanticConventionsWorkload()

	// Wait for collection
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test database semantic conventions per OTEL spec
	// https://opentelemetry.io/docs/specs/semconv/database/

	// Verify required database attributes exist
	requiredDbAttributes := []string{
		"db.system",      // Database system (postgresql, mysql, etc.)
		"db.name",        // Database name
		"server.address", // Database server address
		"server.port",    // Database server port
	}

	for _, attr := range requiredDbAttributes {
		found, err := s.nrdb.CheckAttributeExists(ctx, "postgresql.backends", attr, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.True(s.T(), found, "Required database attribute %s should be present", attr)
	}

	// Test metric naming conventions
	conventionTests := []struct {
		metricPattern string
		description   string
	}{
		{"postgresql.*", "PostgreSQL metrics should follow postgresql.* naming"},
		{"db.*", "Generic database metrics should follow db.* naming"},
		{"*.count", "Count metrics should end with .count"},
		{"*.duration", "Duration metrics should end with .duration"},
	}

	for _, test := range conventionTests {
		count, err := s.nrdb.CountMetricsMatchingPattern(ctx, test.metricPattern, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.Greater(s.T(), count, 0, test.description)
	}

	s.T().Log("✓ OpenTelemetry semantic conventions compliance passed")
}

// Test 3: Cardinality Control and High-Cardinality Prevention
func (s *OTLPFormatValidationSuite) TestCardinalityControl() {
	s.T().Log("Testing cardinality control and explosion prevention...")

	// Generate high-cardinality workload to test controls
	s.generateHighCardinalityWorkload()

	// Wait for processing
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Verify cardinality is controlled by processors
	totalCardinality, err := s.nrdb.GetTotalMetricCardinality(ctx, "10 minutes ago")
	require.NoError(s.T(), err)

	// Should be limited by costcontrol processor
	assert.Less(s.T(), totalCardinality, 5000, "Total cardinality should be controlled to prevent explosion")

	// Verify high-cardinality dimensions are properly handled
	highCardinalityMetrics, err := s.nrdb.GetHighCardinalityMetrics(ctx, 100, "10 minutes ago")
	require.NoError(s.T(), err)

	// Most metrics should have reasonable cardinality
	reasonableCardinalityCount := 0
	for _, metric := range highCardinalityMetrics {
		if metric.Cardinality < 50 {
			reasonableCardinalityCount++
		}
	}

	ratio := float64(reasonableCardinalityCount) / float64(len(highCardinalityMetrics))
	assert.Greater(s.T(), ratio, 0.8, "At least 80%% of metrics should have reasonable cardinality")

	s.T().Log("✓ Cardinality control test passed")
}

// Test 4: OTLP Protocol Format Validation
func (s *OTLPFormatValidationSuite) TestOTLPProtocolFormat() {
	s.T().Log("Testing OTLP protocol format validation...")

	// Generate specific test data for protocol validation
	s.generateOTLPProtocolTestData()

	// Wait for export
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test metric format compliance
	// Verify metrics follow OTLP structure

	// Check for proper resource attributes
	resourceAttributes := []string{
		"service.name",
		"service.version",
		"service.instance.id",
		"telemetry.sdk.name",
		"telemetry.sdk.version",
		"telemetry.sdk.language",
	}

	for _, attr := range resourceAttributes {
		found, err := s.nrdb.CheckResourceAttributeExists(ctx, attr, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.True(s.T(), found, "OTLP resource attribute %s should be present", attr)
	}

	// Verify metric data types are correct
	metricTypeTests := []struct {
		metricName   string
		expectedType string
	}{
		{"postgresql.backends", "gauge"},
		{"postgresql.commits", "counter"},
		{"postgresql.database.size", "gauge"},
	}

	for _, test := range metricTypeTests {
		actualType, err := s.nrdb.GetMetricDataType(ctx, test.metricName, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.Equal(s.T(), test.expectedType, actualType,
			"Metric %s should have type %s", test.metricName, test.expectedType)
	}

	s.T().Log("✓ OTLP protocol format validation passed")
}

// Test 5: Processor Pipeline Validation
func (s *OTLPFormatValidationSuite) TestProcessorPipelineValidation() {
	s.T().Log("Testing processor pipeline validation...")

	// Generate test data that exercises all processors
	s.generateProcessorTestWorkload()

	// Wait for processing
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Verify verification processor is working
	piiTestResults, err := s.nrdb.SearchForPIIInMetrics(ctx, []string{
		"123-45-6789",      // SSN pattern
		"test@example.com", // Email pattern
		"555-1234",         // Phone pattern
	}, "10 minutes ago")
	require.NoError(s.T(), err)
	assert.Empty(s.T(), piiTestResults, "PII should be sanitized by verification processor")

	// Verify cost control processor effects
	costMetrics, err := s.nrdb.GetCostControlMetrics(ctx, "10 minutes ago")
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), costMetrics, "Cost control processor should generate metrics")

	// Verify planattributeextractor processor
	planAttributes, err := s.nrdb.GetPlanAttributes(ctx, "10 minutes ago")
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), planAttributes, "Plan attribute extractor should add plan data")

	s.T().Log("✓ Processor pipeline validation passed")
}

// Helper methods for generating test workloads

func (s *OTLPFormatValidationSuite) setupOTLPTestSchema() {
	// Create test tables for dimensional metrics testing
	_, err := s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS otlp_test_users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50),
			email VARCHAR(100),
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		s.T().Logf("Warning: Could not create test schema: %v", err)
	}

	_, err = s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS otlp_test_orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES otlp_test_users(id),
			amount DECIMAL(10,2),
			order_date TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		s.T().Logf("Warning: Could not create orders table: %v", err)
	}
}

func (s *OTLPFormatValidationSuite) generateDimensionalTestWorkload() {
	// Generate database operations with clear dimensional patterns
	operations := []string{
		"SELECT COUNT(*) FROM otlp_test_users",
		"INSERT INTO otlp_test_users (username, email) VALUES ('test_user', 'test@example.com')",
		"UPDATE otlp_test_users SET username = 'updated_user' WHERE id = 1",
		"SELECT * FROM otlp_test_users WHERE created_at > NOW() - INTERVAL '1 hour'",
		"SELECT u.username, COUNT(o.id) FROM otlp_test_users u LEFT JOIN otlp_test_orders o ON u.id = o.user_id GROUP BY u.username",
	}

	for _, op := range operations {
		_, err := s.env.PostgresDB.Exec(op)
		if err != nil {
			s.T().Logf("Operation failed (expected): %v", err)
		}
		time.Sleep(100 * time.Millisecond) // Spread out operations
	}
}

func (s *OTLPFormatValidationSuite) generateSemanticConventionsWorkload() {
	// Generate operations that test semantic conventions
	semanticQueries := []string{
		"SELECT version()",
		"SELECT current_database()",
		"SELECT COUNT(*) FROM information_schema.tables",
		"SELECT pg_database_size(current_database())",
		"SELECT COUNT(*) FROM pg_stat_activity",
		"SELECT schemaname, tablename FROM pg_tables LIMIT 5",
	}

	for _, query := range semanticQueries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Semantic query failed (expected): %v", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *OTLPFormatValidationSuite) generateHighCardinalityWorkload() {
	// Generate high-cardinality data to test controls
	for i := 0; i < 500; i++ {
		query := fmt.Sprintf(
			"INSERT INTO otlp_test_users (username, email) VALUES ('user_%d', 'user_%d@example.com')",
			i, i,
		)
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("High cardinality insert failed: %v", err)
		}

		if i%10 == 0 {
			time.Sleep(10 * time.Millisecond) // Brief pause every 10 inserts
		}
	}
}

func (s *OTLPFormatValidationSuite) generateOTLPProtocolTestData() {
	// Generate data specifically for OTLP protocol testing
	protocolQueries := []string{
		"SELECT 'OTLP_TEST_MARKER' as test_marker, NOW() as timestamp",
		"SELECT COUNT(*) as connection_count FROM pg_stat_activity",
		"SELECT datname, numbackends FROM pg_stat_database WHERE datname = current_database()",
	}

	for _, query := range protocolQueries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Protocol test query failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *OTLPFormatValidationSuite) generateProcessorTestWorkload() {
	// Generate data to test processor functionality

	// Test PII detection
	piiQueries := []string{
		"SELECT 'user@example.com' as email_test",
		"SELECT '123-45-6789' as ssn_test",
		"SELECT '555-123-4567' as phone_test",
	}

	for _, query := range piiQueries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("PII test query failed: %v", err)
		}
	}

	// Test complex queries for plan extraction
	complexQueries := []string{
		"SELECT u.id, u.username, COUNT(o.id) as order_count FROM otlp_test_users u LEFT JOIN otlp_test_orders o ON u.id = o.user_id GROUP BY u.id, u.username ORDER BY order_count DESC",
		"SELECT * FROM otlp_test_users WHERE created_at BETWEEN NOW() - INTERVAL '1 day' AND NOW()",
	}

	for _, query := range complexQueries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Complex query failed: %v", err)
		}
	}
}

func (s *OTLPFormatValidationSuite) cleanupOTLPTestData() {
	if s.env.PostgresDB != nil {
		_, _ = s.env.PostgresDB.Exec("DROP TABLE IF EXISTS otlp_test_orders")
		_, _ = s.env.PostgresDB.Exec("DROP TABLE IF EXISTS otlp_test_users")
	}
}

func (s *OTLPFormatValidationSuite) getOTLPValidationConfig() string {
	return `
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    database: ${POSTGRES_DB}
    collection_interval: 30s
    tls:
      insecure: true
    
processors:
  # OTLP-focused processor chain
  resource:
    attributes:
      - key: service.name
        value: database-intelligence-collector
        action: upsert
      - key: service.version
        value: 1.0.0-otlp-test
        action: upsert
      - key: service.instance.id
        value: otlp-test-instance
        action: upsert
      - key: environment
        value: otlp-test
        action: upsert
      - key: telemetry.sdk.name
        value: opentelemetry
        action: upsert
      - key: telemetry.sdk.version
        value: 1.0.0
        action: upsert
      - key: telemetry.sdk.language
        value: go
        action: upsert
        
  verification:
    enabled: true
    pii_detection:
      enabled: true
      categories: ["email", "phone", "ssn", "credit_card"]
    cardinality_limit: 1000
    semantic_conventions:
      enforce: true
      
  costcontrol:
    enabled: true
    max_cardinality: 1000
    cost_per_million_data_points: 0.25
    alert_threshold: 100
    
  planattributeextractor:
    enabled: true
    cache_size: 500
    
  batch:
    timeout: 10s
    send_batch_size: 512
    send_batch_max_size: 1024

exporters:
  otlp/newrelic:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    compression: gzip
    timeout: 30s
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 10s
      max_elapsed_time: 60s
      
  debug:
    verbosity: detailed
    sampling_initial: 10
    sampling_thereafter: 10

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [resource, verification, costcontrol, planattributeextractor, batch]
      exporters: [otlp/newrelic, debug]
      
  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: 0.0.0.0:8888
      
  extensions:
    health_check:
      endpoint: 0.0.0.0:13133
      check_collector_pipeline:
        enabled: true
        interval: 5s
        exporter_failure_threshold: 5
`
}
