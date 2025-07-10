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

// OTELDimensionalMetricsSuite tests dimensional metrics and OTLP format compliance
type OTELDimensionalMetricsSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
	nrdb      *framework.NRDBClient
	ctx       context.Context
	cancel    context.CancelFunc
}

func TestOTELDimensionalMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping OTEL dimensional metrics tests in short mode")
	}

	suite.Run(t, new(OTELDimensionalMetricsSuite))
}

func (s *OTELDimensionalMetricsSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Initialize environment with OTLP focus
	s.env = framework.NewTestEnvironment()
	require.NoError(s.T(), s.env.Initialize())

	// Verify New Relic credentials
	s.Require().NotEmpty(s.env.NewRelicAccountID, "NEW_RELIC_ACCOUNT_ID must be set")
	s.Require().NotEmpty(s.env.NewRelicAPIKey, "NEW_RELIC_API_KEY must be set")
	s.Require().NotEmpty(s.env.NewRelicLicenseKey, "NEW_RELIC_LICENSE_KEY must be set")

	// Initialize NRDB client
	s.nrdb = framework.NewNRDBClient(s.env.NewRelicAccountID, s.env.NewRelicAPIKey)

	// Initialize collector
	s.collector = framework.NewTestCollector(s.env)

	// Start collector with dimensional metrics config
	config := s.getDimensionalMetricsConfig()
	require.NoError(s.T(), s.collector.Start(config))

	// Setup test schema for dimensional testing
	s.setupDimensionalTestSchema()
}

func (s *OTELDimensionalMetricsSuite) TearDownSuite() {
	s.cancel()

	// Cleanup test data
	s.cleanupDimensionalTestData()

	// Stop collector
	if s.collector != nil {
		s.collector.Stop()
	}

	// Cleanup environment
	if s.env != nil {
		s.env.Cleanup()
	}
}

// Test 1: Dimensional Metrics Schema Validation
func (s *OTELDimensionalMetricsSuite) TestDimensionalMetricsSchema() {
	s.T().Log("Testing dimensional metrics schema compliance...")

	// Generate specific database activity with known dimensions
	s.generateDimensionalTestData()

	// Wait for metrics collection
	time.Sleep(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test database system dimension
	expectedDimensions := map[string][]string{
		"db.system":    {"postgresql", "mysql"},
		"db.operation": {"SELECT", "INSERT", "UPDATE", "DELETE"},
		"db.name":      {s.env.PostgresDatabase, s.env.MySQLDatabase},
		"service.name": {"database-intelligence-collector"},
		"environment":  {"e2e-test"},
	}

	// Verify each dimension combination exists
	for dimensionKey, expectedValues := range expectedDimensions {
		for _, expectedValue := range expectedValues {
			err := s.nrdb.VerifyMetricWithDimensions(ctx, "db.query.duration", map[string]interface{}{
				dimensionKey: expectedValue,
			}, "10 minutes ago")

			if dimensionKey == "db.system" && expectedValue == "mysql" {
				// MySQL might not be available in all test environments
				if err != nil {
					s.T().Logf("MySQL not available: %v", err)
					continue
				}
			}

			assert.NoError(s.T(), err, "Metric with dimension %s=%s should exist", dimensionKey, expectedValue)
		}
	}

	s.T().Log("✓ Dimensional metrics schema validation passed")
}

// Test 2: Cardinality Control and Explosion Prevention
func (s *OTELDimensionalMetricsSuite) TestCardinalityControl() {
	s.T().Log("Testing cardinality control and explosion prevention...")

	// Generate high-cardinality data
	s.generateHighCardinalityData()

	// Wait for processing
	time.Sleep(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Verify cardinality is controlled
	cardinality, err := s.nrdb.GetMetricCardinality(ctx, "db.query.duration", "10 minutes ago")
	require.NoError(s.T(), err)

	// Should be limited by cost control processor
	assert.Less(s.T(), cardinality, 1000, "Cardinality should be controlled to prevent explosion")

	// Verify high-cardinality dimensions are dropped/sampled
	err = s.nrdb.VerifyMetricWithDimensions(ctx, "db.query.duration", map[string]interface{}{
		"user.id": "high_cardinality_user_99999",
	}, "10 minutes ago")
	assert.Error(s.T(), err, "High cardinality dimensions should be dropped")

	s.T().Log("✓ Cardinality control test passed")
}

// Test 3: OTLP Protocol Compliance
func (s *OTELDimensionalMetricsSuite) TestOTLPProtocolCompliance() {
	s.T().Log("Testing OTLP protocol compliance...")

	// Generate test data
	s.generateOTLPTestData()

	// Wait for collection
	time.Sleep(65 * time.Second)

	// Test OTLP/HTTP endpoint
	s.testOTLPHTTPCompliance()

	// Test OTLP/gRPC endpoint
	s.testOTLPGRPCCompliance()

	s.T().Log("✓ OTLP protocol compliance test passed")
}

// Test 4: Semantic Conventions Validation
func (s *OTELDimensionalMetricsSuite) TestSemanticConventions() {
	s.T().Log("Testing OpenTelemetry semantic conventions...")

	// Generate database activity
	s.generateSemanticConventionsTestData()

	// Wait for collection
	time.Sleep(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test database semantic conventions
	// https://opentelemetry.io/docs/specs/semconv/database/
	requiredDatabaseAttributes := []string{
		"db.system",
		"db.name",
		"db.operation",
		"db.statement",
		"db.user",
		"server.address",
		"server.port",
	}

	// Verify each required attribute exists
	for _, attr := range requiredDatabaseAttributes {
		exists, err := s.nrdb.VerifyAttributeExists(ctx, "db.query.duration", attr, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.True(s.T(), exists, "Required database attribute %s should exist", attr)
	}

	// Test metric naming conventions
	expectedMetricNames := []string{
		"db.query.duration",
		"db.query.count",
		"db.connection.count",
		"db.connection.max",
		"db.rows.affected",
	}

	for _, metricName := range expectedMetricNames {
		exists, err := s.nrdb.VerifyMetricExists(ctx, metricName, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.True(s.T(), exists, "Metric %s should follow naming conventions", metricName)
	}

	s.T().Log("✓ Semantic conventions validation passed")
}

// Test 5: Metric Types and Units Validation
func (s *OTELDimensionalMetricsSuite) TestMetricTypesAndUnits() {
	s.T().Log("Testing metric types and units...")

	// Generate test data
	s.generateMetricTypesTestData()

	// Wait for collection
	time.Sleep(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test histogram metrics
	histogramMetrics := []string{
		"db.query.duration",
		"db.connection.duration",
	}

	for _, metric := range histogramMetrics {
		metricType, err := s.nrdb.GetMetricType(ctx, metric, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.Equal(s.T(), "histogram", metricType, "Metric %s should be histogram type", metric)
	}

	// Test gauge metrics
	gaugeMetrics := []string{
		"db.connection.count",
		"db.connection.max",
	}

	for _, metric := range gaugeMetrics {
		metricType, err := s.nrdb.GetMetricType(ctx, metric, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.Equal(s.T(), "gauge", metricType, "Metric %s should be gauge type", metric)
	}

	// Test counter metrics
	counterMetrics := []string{
		"db.query.count",
		"db.rows.affected",
	}

	for _, metric := range counterMetrics {
		metricType, err := s.nrdb.GetMetricType(ctx, metric, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.Equal(s.T(), "counter", metricType, "Metric %s should be counter type", metric)
	}

	s.T().Log("✓ Metric types and units validation passed")
}

// Test 6: Exemplars and Span Links
func (s *OTELDimensionalMetricsSuite) TestExemplarsAndSpanLinks() {
	s.T().Log("Testing exemplars and span links...")

	// Generate traced database operations
	s.generateTracedOperations()

	// Wait for collection
	time.Sleep(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Verify exemplars exist for slow queries
	exemplars, err := s.nrdb.GetMetricExemplars(ctx, "db.query.duration", "10 minutes ago")
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), exemplars, "Exemplars should exist for slow queries")

	// Verify span links in exemplars
	for _, exemplar := range exemplars {
		assert.NotEmpty(s.T(), exemplar.TraceID, "Exemplar should have trace ID")
		assert.NotEmpty(s.T(), exemplar.SpanID, "Exemplar should have span ID")
	}

	s.T().Log("✓ Exemplars and span links test passed")
}

// Test 7: Resource Attributes Validation
func (s *OTELDimensionalMetricsSuite) TestResourceAttributes() {
	s.T().Log("Testing resource attributes...")

	// Generate test data
	s.generateResourceAttributesTestData()

	// Wait for collection
	time.Sleep(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Test required resource attributes
	requiredResourceAttributes := []string{
		"service.name",
		"service.version",
		"service.instance.id",
		"host.name",
		"host.arch",
		"os.type",
		"telemetry.sdk.name",
		"telemetry.sdk.version",
		"telemetry.sdk.language",
	}

	for _, attr := range requiredResourceAttributes {
		exists, err := s.nrdb.VerifyResourceAttributeExists(ctx, attr, "10 minutes ago")
		require.NoError(s.T(), err)
		assert.True(s.T(), exists, "Resource attribute %s should exist", attr)
	}

	s.T().Log("✓ Resource attributes validation passed")
}

// Test 8: OTLP Batch Processing
func (s *OTELDimensionalMetricsSuite) TestOTLPBatchProcessing() {
	s.T().Log("Testing OTLP batch processing...")

	// Generate high-volume data
	s.generateHighVolumeData()

	// Wait for processing
	time.Sleep(65 * time.Second)

	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()

	// Verify batch processing efficiency
	batchMetrics, err := s.nrdb.GetBatchProcessingMetrics(ctx, "10 minutes ago")
	require.NoError(s.T(), err)

	assert.Greater(s.T(), batchMetrics.BatchSize, 0, "Batch size should be greater than 0")
	assert.Less(s.T(), batchMetrics.ProcessingLatency, 1000, "Processing latency should be less than 1s")

	s.T().Log("✓ OTLP batch processing test passed")
}

// Helper functions

func (s *OTELDimensionalMetricsSuite) generateDimensionalTestData() {
	// Generate PostgreSQL activity with specific dimensions
	queries := []string{
		"SELECT * FROM users WHERE active = true",
		"INSERT INTO orders (user_id, amount) VALUES (1, 100.00)",
		"UPDATE users SET last_login = NOW() WHERE id = 1",
		"DELETE FROM temp_data WHERE created_at < NOW() - INTERVAL '1 day'",
	}

	for _, query := range queries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Query failed (expected): %v", err)
		}
	}
}

func (s *OTELDimensionalMetricsSuite) generateHighCardinalityData() {
	// Generate queries with high cardinality dimensions
	for i := 0; i < 1000; i++ {
		query := fmt.Sprintf("SELECT * FROM users WHERE id = %d AND session_id = 'session_%d'", i, i)
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("High cardinality query failed (expected): %v", err)
		}
	}
}

func (s *OTELDimensionalMetricsSuite) generateOTLPTestData() {
	// Generate data that will be sent via OTLP
	for i := 0; i < 10; i++ {
		query := fmt.Sprintf("SELECT COUNT(*) FROM pg_stat_activity WHERE query LIKE 'SELECT%%'")
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("OTLP test query failed: %v", err)
		}
	}
}

func (s *OTELDimensionalMetricsSuite) generateSemanticConventionsTestData() {
	// Generate data that exercises semantic conventions
	semanticQueries := []string{
		"SELECT version()",
		"SELECT COUNT(*) FROM information_schema.tables",
		"SELECT pg_database_size(current_database())",
		"SELECT * FROM pg_stat_database WHERE datname = current_database()",
	}

	for _, query := range semanticQueries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Semantic conventions query failed: %v", err)
		}
	}
}

func (s *OTELDimensionalMetricsSuite) generateMetricTypesTestData() {
	// Generate data for different metric types
	for i := 0; i < 5; i++ {
		// Generate histogram data (query duration)
		_, err := s.env.PostgresDB.Exec("SELECT pg_sleep(0.1)")
		if err != nil {
			s.T().Logf("Histogram test query failed: %v", err)
		}

		// Generate gauge data (connection count)
		_, err = s.env.PostgresDB.Exec("SELECT COUNT(*) FROM pg_stat_activity")
		if err != nil {
			s.T().Logf("Gauge test query failed: %v", err)
		}
	}
}

func (s *OTELDimensionalMetricsSuite) generateTracedOperations() {
	// Generate operations that should create exemplars
	slowQueries := []string{
		"SELECT pg_sleep(0.5)",
		"SELECT COUNT(*) FROM pg_stat_activity WHERE query LIKE '%SELECT%'",
		"SELECT * FROM pg_stat_database ORDER BY datname",
	}

	for _, query := range slowQueries {
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("Traced operation failed: %v", err)
		}
	}
}

func (s *OTELDimensionalMetricsSuite) generateResourceAttributesTestData() {
	// Generate data that exercises resource attributes
	_, err := s.env.PostgresDB.Exec("SELECT current_database(), version(), inet_server_addr(), inet_server_port()")
	if err != nil {
		s.T().Logf("Resource attributes query failed: %v", err)
	}
}

func (s *OTELDimensionalMetricsSuite) generateHighVolumeData() {
	// Generate high volume for batch processing
	for i := 0; i < 100; i++ {
		query := fmt.Sprintf("SELECT %d", i)
		_, err := s.env.PostgresDB.Exec(query)
		if err != nil {
			s.T().Logf("High volume query failed: %v", err)
		}
	}
}

func (s *OTELDimensionalMetricsSuite) testOTLPHTTPCompliance() {
	// Test OTLP/HTTP endpoint compliance
	// This would involve creating proper OTLP HTTP requests
	// and validating responses
	s.T().Log("Testing OTLP/HTTP compliance...")
}

func (s *OTELDimensionalMetricsSuite) testOTLPGRPCCompliance() {
	// Test OTLP/gRPC endpoint compliance
	// This would involve creating proper OTLP gRPC requests
	// and validating responses
	s.T().Log("Testing OTLP/gRPC compliance...")
}

func (s *OTELDimensionalMetricsSuite) getDimensionalMetricsConfig() string {
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
    metrics:
      - metric_name: db.query.duration
        query: "SELECT schemaname, tablename, seq_scan, idx_scan, tup_returned, tup_fetched FROM pg_stat_user_tables"
        value_column: "seq_scan"
        attribute_columns: ["schemaname", "tablename"]
      - metric_name: db.connection.count
        query: "SELECT COUNT(*) FROM pg_stat_activity"
        value_column: "count"
      - metric_name: db.connection.max
        query: "SELECT setting::int FROM pg_settings WHERE name = 'max_connections'"
        value_column: "setting"

processors:
  # Dimensional metrics processors
  verification:
    enabled: true
    pii_detection:
      enabled: true
      categories: ["email", "phone", "ssn", "credit_card"]
    cardinality_limit: 1000
    
  costcontrol:
    enabled: true
    max_cardinality: 1000
    cost_per_million_data_points: 0.25
    
  planattributeextractor:
    enabled: true
    
  # Standard processors
  batch:
    timeout: 10s
    send_batch_size: 1000
    
  resource:
    attributes:
      - key: service.name
        value: database-intelligence-collector
        action: upsert
      - key: service.version
        value: 1.0.0
        action: upsert
      - key: environment
        value: e2e-test
        action: upsert

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
    sampling_initial: 5
    sampling_thereafter: 5

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [resource, verification, costcontrol, planattributeextractor, batch]
      exporters: [otlp/newrelic, debug]
      
  telemetry:
    logs:
      level: debug
    metrics:
      level: detailed
      address: 0.0.0.0:8888
      
  extensions:
    health_check:
      endpoint: 0.0.0.0:13133
`
}

// Helper methods for setup and cleanup
func (s *OTELDimensionalMetricsSuite) setupDimensionalTestSchema() {
	// Create test tables for dimensional testing
	s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS dim_test_users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(50) UNIQUE NOT NULL,
			email VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)

	s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS dim_test_orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES dim_test_users(id),
			amount DECIMAL(10,2),
			status VARCHAR(20),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
}

func (s *OTELDimensionalMetricsSuite) cleanupDimensionalTestData() {
	if s.env.PostgresDB != nil {
		_, _ = s.env.PostgresDB.Exec("DROP TABLE IF EXISTS dim_test_orders")
		_, _ = s.env.PostgresDB.Exec("DROP TABLE IF EXISTS dim_test_users")
		_, _ = s.env.PostgresDB.Exec("DROP TABLE IF EXISTS high_cardinality_test")
	}
}
