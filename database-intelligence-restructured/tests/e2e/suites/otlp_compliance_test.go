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

// OTLPComplianceTestSuite tests OTLP format compliance and dimensional metrics
type OTLPComplianceTestSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
	nrdb      *framework.NRDBClient
	ctx       context.Context
	cancel    context.CancelFunc
}

func TestOTLPCompliance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OTLP compliance tests in short mode")
	}

	suite.Run(t, new(OTLPComplianceTestSuite))
}

func (s *OTLPComplianceTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Initialize environment
	s.env = framework.NewTestEnvironment()
	require.NoError(s.T(), s.env.Initialize())

	// Verify credentials
	require.NotEmpty(s.T(), s.env.NewRelicAccountID, "NEW_RELIC_ACCOUNT_ID must be set")
	require.NotEmpty(s.T(), s.env.NewRelicAPIKey, "NEW_RELIC_API_KEY must be set")
	require.NotEmpty(s.T(), s.env.NewRelicLicenseKey, "NEW_RELIC_LICENSE_KEY must be set")

	// Initialize NRDB client
	s.nrdb = framework.NewNRDBClient(s.env.NewRelicAccountID, s.env.NewRelicAPIKey)

	// Initialize collector
	s.collector = framework.NewTestCollector(s.env)

	// Start collector with OTLP compliance config
	config := s.getOTLPComplianceConfig()
	require.NoError(s.T(), s.collector.Start(config))

	// Setup test schema
	s.setupOTLPTestSchema()
}

func (s *OTLPComplianceTestSuite) TearDownSuite() {
	s.cancel()
	s.cleanupOTLPTestData()

	if s.collector != nil {
		s.collector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
}

// Test 1: OTLP Dimensional Metrics Schema Validation
func (s *OTLPComplianceTestSuite) TestOTLPDimensionalSchema() {
	s.T().Log("Testing OTLP dimensional metrics schema compliance...")

	// Generate dimensional test workload
	s.generateDimensionalWorkload()

	// Wait for collection
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test dimensional attributes
	expectedDimensions := map[string][]string{
		"db.system":    {"postgresql"},
		"service.name": {"database-intelligence-collector"},
		"environment":  {"otlp-compliance-test"},
	}

	// Verify dimensions exist for key metrics
	keyMetrics := []string{
		"postgresql.backends",
		"postgresql.database.size",
	}

	for _, metric := range keyMetrics {
		for dimKey, dimValues := range expectedDimensions {
			for _, dimValue := range dimValues {
				err := s.nrdb.VerifyMetric(ctx, metric, map[string]interface{}{
					dimKey: dimValue,
				}, "10 minutes ago")
				assert.NoError(s.T(), err, "Dimension %s=%s should exist for metric %s",
					dimKey, dimValue, metric)
			}
		}
	}

	s.T().Log("✓ OTLP dimensional schema validation passed")
}

// Test 2: Cardinality Control Validation
func (s *OTLPComplianceTestSuite) TestCardinalityControl() {
	s.T().Log("Testing cardinality control...")

	// Generate high-cardinality workload
	s.generateHighCardinalityWorkload()

	// Wait for processing
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Check total cardinality is controlled
	totalCardinality, err := s.nrdb.GetTotalMetricCardinality(ctx, "10 minutes ago")
	require.NoError(s.T(), err)

	// Should be limited by costcontrol processor
	assert.Less(s.T(), totalCardinality, 2000, "Total cardinality should be controlled")

	s.T().Log("✓ Cardinality control validation passed")
}

// Test 3: Semantic Conventions Compliance
func (s *OTLPComplianceTestSuite) TestSemanticConventions() {
	s.T().Log("Testing OpenTelemetry semantic conventions...")

	// Generate semantic conventions test data
	s.generateSemanticWorkload()

	// Wait for collection
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test required database attributes per OTEL spec
	requiredAttributes := []string{
		"db.system",
		"db.name",
		"server.address",
		"server.port",
	}

	for _, attr := range requiredAttributes {
		found, err := s.nrdb.CheckAttributeExists(ctx, "postgresql.backends", attr, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.True(s.T(), found, "Required attribute %s should be present", attr)
	}

	// Test resource attributes
	resourceAttributes := []string{
		"service.name",
		"service.version",
		"telemetry.sdk.name",
	}

	for _, attr := range resourceAttributes {
		found, err := s.nrdb.CheckResourceAttributeExists(ctx, attr, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.True(s.T(), found, "Resource attribute %s should be present", attr)
	}

	s.T().Log("✓ Semantic conventions compliance passed")
}

// Test 4: Metric Types and Format Validation
func (s *OTLPComplianceTestSuite) TestMetricTypesFormat() {
	s.T().Log("Testing metric types and format...")

	// Generate metric types test data
	s.generateMetricTypesWorkload()

	// Wait for collection
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test metric type compliance
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

	s.T().Log("✓ Metric types and format validation passed")
}

// Test 5: Advanced Processor Pipeline Validation
func (s *OTLPComplianceTestSuite) TestAdvancedProcessorPipeline() {
	s.T().Log("Testing advanced processor pipeline...")

	// Generate processor test workload
	s.generateProcessorWorkload()

	// Wait for processing
	s.collector.WaitForMetricCollection(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test PII sanitization
	piiResults, err := s.nrdb.SearchForPIIInMetrics(ctx, []string{
		"123-45-6789",
		"test@example.com",
		"555-1234",
	}, "10 minutes ago")
	require.NoError(s.T(), err)
	assert.Empty(s.T(), piiResults, "PII should be sanitized")

	// Test cost control
	costMetrics, err := s.nrdb.GetCostControlMetrics(ctx, "10 minutes ago")
	require.NoError(s.T(), err)
	s.T().Logf("Found %d cost control metrics", len(costMetrics))

	// Test plan attributes
	planAttrs, err := s.nrdb.GetPlanAttributes(ctx, "10 minutes ago")
	require.NoError(s.T(), err)
	s.T().Logf("Found %d plan attributes", len(planAttrs))

	s.T().Log("✓ Advanced processor pipeline validation passed")
}

// Helper methods for test workload generation

func (s *OTLPComplianceTestSuite) setupOTLPTestSchema() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS otlp_compliance_test (
			id SERIAL PRIMARY KEY,
			test_type VARCHAR(50),
			test_data TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_otlp_test_type ON otlp_compliance_test(test_type)`,
	}

	for _, query := range queries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Warning: Schema setup failed: %v", err)
		}
	}
}

func (s *OTLPComplianceTestSuite) generateDimensionalWorkload() {
	operations := []string{
		"SELECT COUNT(*) FROM otlp_compliance_test",
		"INSERT INTO otlp_compliance_test (test_type, test_data) VALUES ('dimensional', 'test_data_1')",
		"UPDATE otlp_compliance_test SET test_data = 'updated' WHERE test_type = 'dimensional'",
		"SELECT * FROM otlp_compliance_test WHERE test_type = 'dimensional'",
	}

	for _, op := range operations {
		_, err := s.env.PostgresDB.Exec(op)
		if err != nil {
			s.T().Logf("Dimensional operation failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *OTLPComplianceTestSuite) generateHighCardinalityWorkload() {
	for i := 0; i < 200; i++ {
		query := fmt.Sprintf(
			"INSERT INTO otlp_compliance_test (test_type, test_data) VALUES ('cardinality_%d', 'data_%d')",
			i, i,
		)
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Cardinality insert failed: %v", err)
		}

		if i%20 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func (s *OTLPComplianceTestSuite) generateSemanticWorkload() {
	semanticQueries := []string{
		"SELECT version()",
		"SELECT current_database()",
		"SELECT COUNT(*) FROM information_schema.tables",
		"SELECT pg_database_size(current_database())",
		"SELECT COUNT(*) FROM pg_stat_activity",
	}

	for _, query := range semanticQueries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Semantic query failed: %v", err)
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func (s *OTLPComplianceTestSuite) generateMetricTypesWorkload() {
	for i := 0; i < 10; i++ {
		// Generate gauge data
		_, err := s.env.PostgresDB.Exec("SELECT COUNT(*) FROM pg_stat_activity")
		if err != nil {
			s.T().Logf("Gauge query failed: %v", err)
		}

		// Generate counter data
		_, err = s.env.PostgresDB.Exec("SELECT SUM(xact_commit) FROM pg_stat_database")
		if err != nil {
			s.T().Logf("Counter query failed: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func (s *OTLPComplianceTestSuite) generateProcessorWorkload() {
	// Test PII detection
	piiQueries := []string{
		"SELECT 'user@example.com' as email_test",
		"SELECT '123-45-6789' as ssn_test",
		"SELECT '555-123-4567' as phone_test",
	}

	for _, query := range piiQueries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("PII query failed: %v", err)
		}
	}

	// Test complex queries for plan extraction
	complexQueries := []string{
		"SELECT t.test_type, COUNT(*) FROM otlp_compliance_test t GROUP BY t.test_type ORDER BY COUNT(*) DESC",
		"SELECT * FROM otlp_compliance_test WHERE created_at > NOW() - INTERVAL '1 hour'",
	}

	for _, query := range complexQueries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Complex query failed: %v", err)
		}
	}
}

func (s *OTLPComplianceTestSuite) cleanupOTLPTestData() {
	if s.env.PostgresDB != nil {
		_, _ = s.env.PostgresDB.Exec("DROP TABLE IF EXISTS otlp_compliance_test")
	}
}

func (s *OTLPComplianceTestSuite) getOTLPComplianceConfig() string {
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
  # Resource processor with OTLP-compliant attributes
  resource:
    attributes:
      - key: service.name
        value: database-intelligence-collector
        action: upsert
      - key: service.version
        value: 1.0.0-otlp-compliance
        action: upsert
      - key: service.instance.id
        value: otlp-compliance-test-instance
        action: upsert
      - key: environment
        value: otlp-compliance-test
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
        
  # Verification processor for PII and compliance
  verification:
    enabled: true
    pii_detection:
      enabled: true
      categories: ["email", "phone", "ssn", "credit_card"]
    cardinality_limit: 1000
    semantic_conventions:
      enforce: true
      required_attributes: ["db.system", "db.name", "server.address"]
      
  # Cost control for cardinality management
  costcontrol:
    enabled: true
    max_cardinality: 1000
    cost_per_million_data_points: 0.25
    alert_threshold: 100
    cardinality_explosion_prevention: true
    
  # Plan attribute extractor
  planattributeextractor:
    enabled: true
    cache_size: 500
    plan_analysis: true
    
  # Adaptive sampler for performance
  adaptivesampler:
    decision_wait: 5s
    num_traces_kept: 50
    expected_new_traces_per_sec: 5
    
  # Batch processor for OTLP efficiency
  batch:
    timeout: 10s
    send_batch_size: 512
    send_batch_max_size: 1024

exporters:
  # OTLP exporter to New Relic
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
    sending_queue:
      enabled: true
      num_consumers: 2
      queue_size: 100
      
  # Debug exporter for validation
  debug:
    verbosity: detailed
    sampling_initial: 5
    sampling_thereafter: 5

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [resource, verification, costcontrol, planattributeextractor, adaptivesampler, batch]
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
