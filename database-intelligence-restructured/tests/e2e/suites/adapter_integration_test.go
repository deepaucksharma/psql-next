package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	
	"github.com/database-intelligence/tests/e2e/framework"
)

// AdapterIntegrationSuite tests all adapters (receivers, processors, exporters) integration
type AdapterIntegrationSuite struct {
	suite.Suite
	env       *framework.TestEnvironment
	collector *framework.TestCollector
	nrdb      *framework.NRDBClient
	ctx       context.Context
	cancel    context.CancelFunc
}

func TestAdapterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping adapter integration tests in short mode")
	}
	
	suite.Run(t, new(AdapterIntegrationSuite))
}

func (s *AdapterIntegrationSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithCancel(context.Background())
	
	// Initialize environment
	s.env = framework.NewTestEnvironment()
	require.NoError(s.T(), s.env.Initialize())
	
	// Initialize NRDB client if credentials available
	if s.env.NewRelicAccountID != "" && s.env.NewRelicAPIKey != "" {
		s.nrdb = framework.NewNRDBClient(s.env.NewRelicAccountID, s.env.NewRelicAPIKey)
	}
	
	// Initialize collector
	s.collector = framework.NewTestCollector(s.env)
}

func (s *AdapterIntegrationSuite) TearDownSuite() {
	s.cancel()
	if s.collector != nil {
		s.collector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
}

// Test 1: PostgreSQL Receiver Integration
func (s *AdapterIntegrationSuite) TestPostgreSQLReceiver() {
	s.T().Log("Testing PostgreSQL receiver integration...")
	
	config := `
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    databases:
      - ${POSTGRES_DB}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 10s
    tls:
      insecure: true
    resource_attributes:
      db.system: postgresql

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 100

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      exporters: [debug]
`
	
	// Start collector with PostgreSQL receiver
	err := s.collector.Start(config)
	require.NoError(s.T(), err)
	defer s.collector.Stop()
	
	// Perform database operations
	s.performPostgreSQLOperations()
	
	// Wait for collection
	time.Sleep(15 * time.Second)
	
	// Verify metrics were collected
	logs, err := s.collector.GetLogs()
	require.NoError(s.T(), err)
	
	// Check for expected PostgreSQL metrics
	expectedMetrics := []string{
		"postgresql.backends",
		"postgresql.blocks.read",
		"postgresql.blocks.hit",
		"postgresql.commits",
		"postgresql.rollbacks",
		"postgresql.database.size",
		"postgresql.table.count",
		"postgresql.index.scans",
		"postgresql.sequential.scans",
	}
	
	for _, metric := range expectedMetrics {
		assert.Contains(s.T(), logs, metric, "Should collect %s metric", metric)
	}
	
	s.T().Log("✓ PostgreSQL receiver integration test passed")
}

// Test 2: MySQL Receiver Integration
func (s *AdapterIntegrationSuite) TestMySQLReceiver() {
	if !s.env.MySQLEnabled || s.env.MySQLDB == nil {
		s.T().Skip("MySQL not enabled")
	}
	
	s.T().Log("Testing MySQL receiver integration...")
	
	config := `
receivers:
  mysql:
    endpoint: ${MYSQL_HOST}:${MYSQL_PORT}
    username: ${MYSQL_USER}
    password: ${MYSQL_PASSWORD}
    database: ${MYSQL_DB}
    collection_interval: 10s

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [mysql]
      exporters: [debug]
`
	
	// Restart collector with MySQL receiver
	s.collector.Stop()
	err := s.collector.Start(config)
	require.NoError(s.T(), err)
	defer s.collector.Stop()
	
	// Perform MySQL operations
	s.performMySQLOperations()
	
	// Wait for collection
	time.Sleep(15 * time.Second)
	
	// Verify metrics
	logs, err := s.collector.GetLogs()
	require.NoError(s.T(), err)
	
	expectedMetrics := []string{
		"mysql.buffer_pool.pages",
		"mysql.buffer_pool.data",
		"mysql.buffer_pool.usage",
		"mysql.commands",
		"mysql.threads",
		"mysql.questions",
		"mysql.slow_queries",
	}
	
	for _, metric := range expectedMetrics {
		assert.Contains(s.T(), logs, metric, "Should collect %s metric", metric)
	}
	
	s.T().Log("✓ MySQL receiver integration test passed")
}

// Test 3: SQLQuery Receiver Integration
func (s *AdapterIntegrationSuite) TestSQLQueryReceiver() {
	s.T().Log("Testing SQLQuery receiver integration...")
	
	// Create test table
	_, err := s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS sqlquery_test (
			id SERIAL PRIMARY KEY,
			metric_name VARCHAR(100),
			metric_value NUMERIC,
			tags JSONB,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(s.T(), err)
	
	// Insert test data
	for i := 0; i < 10; i++ {
		_, err = s.env.PostgresDB.Exec(`
			INSERT INTO sqlquery_test (metric_name, metric_value, tags)
			VALUES ($1, $2, $3)
		`, fmt.Sprintf("custom_metric_%d", i%3), rand.Float64()*100, `{"source": "test"}`)
		require.NoError(s.T(), err)
	}
	
	config := `
receivers:
  sqlquery/custom:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} sslmode=disable"
    collection_interval: 10s
    queries:
      - sql: |
          SELECT 
            metric_name,
            AVG(metric_value) as avg_value,
            COUNT(*) as count,
            MAX(metric_value) as max_value,
            MIN(metric_value) as min_value
          FROM sqlquery_test
          WHERE created_at > NOW() - INTERVAL '5 minutes'
          GROUP BY metric_name
        metrics:
          - metric_name: custom.metric.average
            value_column: avg_value
            attribute_columns: [metric_name]
            value_type: double
          - metric_name: custom.metric.count
            value_column: count
            attribute_columns: [metric_name]
            value_type: int
          - metric_name: custom.metric.max
            value_column: max_value
            attribute_columns: [metric_name]
          - metric_name: custom.metric.min
            value_column: min_value
            attribute_columns: [metric_name]

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [sqlquery/custom]
      exporters: [debug]
`
	
	// Restart collector
	s.collector.Stop()
	err = s.collector.Start(config)
	require.NoError(s.T(), err)
	defer s.collector.Stop()
	
	// Wait for collection
	time.Sleep(15 * time.Second)
	
	// Verify custom metrics
	logs, err := s.collector.GetLogs()
	require.NoError(s.T(), err)
	
	assert.Contains(s.T(), logs, "custom.metric.average", "Should collect custom average metric")
	assert.Contains(s.T(), logs, "custom.metric.count", "Should collect custom count metric")
	assert.Contains(s.T(), logs, "custom.metric.max", "Should collect custom max metric")
	assert.Contains(s.T(), logs, "custom.metric.min", "Should collect custom min metric")
	
	// Cleanup
	_, err = s.env.PostgresDB.Exec("DROP TABLE IF EXISTS sqlquery_test")
	require.NoError(s.T(), err)
	
	s.T().Log("✓ SQLQuery receiver integration test passed")
}

// Test 4: All Processors Pipeline Integration
func (s *AdapterIntegrationSuite) TestProcessorsPipeline() {
	s.T().Log("Testing all processors in pipeline...")
	
	config := `
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    databases: [${POSTGRES_DB}]
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 10s
    tls:
      insecure: true

processors:
  # Custom processors
  adaptivesampler:
    decision_wait: 5s
    num_traces_kept: 100
    expected_new_traces_per_sec: 10
    
  circuitbreaker:
    failure_threshold: 0.5
    failure_count_threshold: 5
    success_threshold: 0.8
    observation_window: 1m
    
  planattributeextractor:
    enabled: true
    cache_size: 100
    
  querycorrelator:
    correlation_window: 1m
    max_correlation_size: 50
    
  verification:
    enabled: true
    pii_detection:
      enabled: true
      categories: ["email", "phone", "ssn"]
    
  costcontrol:
    enabled: true
    cost_per_million_data_points: 0.25
    
  nrerrormonitor:
    enabled: true
    error_rate_threshold: 0.1
    
  # Standard processors
  batch:
    timeout: 5s
    send_batch_size: 100
    
  attributes:
    actions:
      - key: processor.test
        action: insert
        value: "all_processors"
      - key: test.timestamp
        action: insert
        value: "${TIMESTAMP}"

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 100

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [
        verification,
        adaptivesampler,
        circuitbreaker,
        costcontrol,
        nrerrormonitor,
        attributes,
        batch
      ]
      exporters: [debug]
`
	
	// Restart collector with all processors
	s.collector.Stop()
	err := s.collector.Start(config)
	require.NoError(s.T(), err)
	defer s.collector.Stop()
	
	// Generate varied workload to trigger different processors
	s.T().Log("Generating workload to test processors...")
	
	// 1. Generate normal queries
	for i := 0; i < 50; i++ {
		rows, err := s.env.PostgresDB.Query("SELECT version()")
		if err == nil {
			rows.Close()
		}
	}
	
	// 2. Generate queries with PII-like patterns (for verification processor)
	testPII := []string{
		"SELECT * FROM users WHERE email = 'test@example.com'",
		"SELECT * FROM customers WHERE phone = '555-1234'",
		"SELECT * FROM accounts WHERE ssn = '123-45-6789'",
	}
	
	for _, query := range testPII {
		// These should be detected by verification processor
		rows, err := s.env.PostgresDB.Query(query)
		if err == nil {
			rows.Close()
		}
	}
	
	// 3. Generate errors (for error monitor)
	for i := 0; i < 10; i++ {
		_, _ = s.env.PostgresDB.Query("SELECT * FROM nonexistent_table")
	}
	
	// Wait for processing
	time.Sleep(15 * time.Second)
	
	// Verify processors worked
	logs, err := s.collector.GetLogs()
	require.NoError(s.T(), err)
	
	// Check for processor evidence in logs
	processorChecks := []struct {
		processor string
		evidence  []string
	}{
		{
			processor: "verification",
			evidence:  []string{"verification", "checksum", "pii"},
		},
		{
			processor: "adaptivesampler",
			evidence:  []string{"sampling", "adaptive", "decision"},
		},
		{
			processor: "circuitbreaker",
			evidence:  []string{"circuit", "breaker", "state"},
		},
		{
			processor: "costcontrol",
			evidence:  []string{"cost", "estimate", "data_points"},
		},
		{
			processor: "nrerrormonitor",
			evidence:  []string{"error", "monitor", "rate"},
		},
		{
			processor: "attributes",
			evidence:  []string{"processor.test", "all_processors"},
		},
		{
			processor: "batch",
			evidence:  []string{"batch", "timeout", "size"},
		},
	}
	
	for _, check := range processorChecks {
		found := false
		for _, evidence := range check.evidence {
			if contains(logs, evidence) {
				found = true
				break
			}
		}
		assert.True(s.T(), found, "Should find evidence of %s processor", check.processor)
	}
	
	s.T().Log("✓ All processors pipeline integration test passed")
}

// Test 5: Exporter Integration
func (s *AdapterIntegrationSuite) TestExporterIntegration() {
	s.T().Log("Testing exporter integration...")
	
	// Skip if no New Relic credentials
	if s.nrdb == nil {
		s.T().Skip("New Relic credentials not configured")
	}
	
	testID := fmt.Sprintf("exporter_test_%d", time.Now().UnixNano())
	
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    databases: [${POSTGRES_DB}]
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 10s
    tls:
      insecure: true

processors:
  attributes:
    actions:
      - key: test.id
        action: insert
        value: "%s"
      - key: exporter.test
        action: insert
        value: "true"

exporters:
  otlp/newrelic:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    retry_on_failure:
      enabled: true
      initial_interval: 1s
      max_interval: 10s
      max_elapsed_time: 30s
    
  prometheus:
    endpoint: "0.0.0.0:8889"
    namespace: db_intelligence_test
    
  debug:
    verbosity: basic

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes]
      exporters: [otlp/newrelic, prometheus, debug]
`, testID)
	
	// Restart collector
	s.collector.Stop()
	err := s.collector.Start(config)
	require.NoError(s.T(), err)
	defer s.collector.Stop()
	
	// Generate some metrics
	for i := 0; i < 10; i++ {
		rows, err := s.env.PostgresDB.Query("SELECT COUNT(*) FROM pg_stat_activity")
		require.NoError(s.T(), err)
		rows.Close()
	}
	
	// Wait for export
	s.T().Log("Waiting for metrics to be exported...")
	time.Sleep(65 * time.Second)
	
	// Verify in NRDB
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()
	
	err = s.nrdb.VerifyMetric(ctx, "postgresql.backends", map[string]interface{}{
		"test.id":       testID,
		"exporter.test": "true",
	}, "5 minutes ago")
	assert.NoError(s.T(), err, "Metrics should be exported to New Relic")
	
	// TODO: Verify Prometheus metrics endpoint
	// This would require querying localhost:8889/metrics
	
	s.T().Log("✓ Exporter integration test passed")
}

// Test 6: NRI Exporter Integration
func (s *AdapterIntegrationSuite) TestNRIExporter() {
	s.T().Log("Testing NRI exporter integration...")
	
	config := `
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    databases: [${POSTGRES_DB}]
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 10s
    tls:
      insecure: true

processors:
  attributes:
    actions:
      - key: nri.test
        action: insert
        value: "true"

exporters:
  nri:
    integration_name: "com.newrelic.postgresql"
    integration_version: "2.0.0"
    
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [postgresql]
      processors: [attributes]
      exporters: [nri, debug]
`
	
	// Restart collector
	s.collector.Stop()
	err := s.collector.Start(config)
	require.NoError(s.T(), err)
	defer s.collector.Stop()
	
	// Wait for collection
	time.Sleep(15 * time.Second)
	
	// Verify NRI format in logs
	logs, err := s.collector.GetLogs()
	require.NoError(s.T(), err)
	
	// Check for NRI format markers
	assert.Contains(s.T(), logs, "com.newrelic.postgresql", "Should use NRI integration name")
	assert.Contains(s.T(), logs, "nri.test", "Should include custom attributes")
	
	s.T().Log("✓ NRI exporter integration test passed")
}

// Test 7: ASH Receiver Integration
func (s *AdapterIntegrationSuite) TestASHReceiver() {
	s.T().Log("Testing ASH receiver integration...")
	
	// Create ASH-like view (simplified)
	_, err := s.env.PostgresDB.Exec(`
		CREATE OR REPLACE VIEW v_ash_test AS
		SELECT 
			pid,
			usename,
			application_name,
			state,
			query,
			backend_start,
			state_change,
			wait_event_type,
			wait_event
		FROM pg_stat_activity
		WHERE pid != pg_backend_pid()
	`)
	require.NoError(s.T(), err)
	defer s.env.PostgresDB.Exec("DROP VIEW IF EXISTS v_ash_test")
	
	config := `
receivers:
  ash:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    database: ${POSTGRES_DB}
    collection_interval: 5s
    sampling_interval: 1s
    ash_query: "SELECT * FROM v_ash_test"

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [ash]
      exporters: [debug]
`
	
	// Restart collector
	s.collector.Stop()
	err = s.collector.Start(config)
	require.NoError(s.T(), err)
	defer s.collector.Stop()
	
	// Generate activity
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				rows, err := s.env.PostgresDB.Query("SELECT pg_sleep(0.1)")
				if err == nil {
					rows.Close()
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	
	// Wait for sampling
	time.Sleep(10 * time.Second)
	close(done)
	
	// Verify ASH metrics
	logs, err := s.collector.GetLogs()
	require.NoError(s.T(), err)
	
	assert.Contains(s.T(), logs, "ash.active_sessions", "Should collect ASH metrics")
	assert.Contains(s.T(), logs, "wait_event", "Should include wait event information")
	
	s.T().Log("✓ ASH receiver integration test passed")
}

// Test 8: Enhanced SQL Receiver Integration
func (s *AdapterIntegrationSuite) TestEnhancedSQLReceiver() {
	s.T().Log("Testing enhanced SQL receiver integration...")
	
	// Create test schema
	_, err := s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS enhanced_sql_test (
			id SERIAL PRIMARY KEY,
			query_text TEXT,
			execution_count INT DEFAULT 0,
			total_time NUMERIC DEFAULT 0,
			mean_time NUMERIC DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	require.NoError(s.T(), err)
	defer s.env.PostgresDB.Exec("DROP TABLE IF EXISTS enhanced_sql_test")
	
	// Insert sample data
	queries := []string{
		"SELECT * FROM users WHERE id = ?",
		"INSERT INTO logs (message) VALUES (?)",
		"UPDATE settings SET value = ? WHERE key = ?",
		"DELETE FROM temp_data WHERE created < ?",
	}
	
	for i, q := range queries {
		_, err = s.env.PostgresDB.Exec(`
			INSERT INTO enhanced_sql_test (query_text, execution_count, total_time, mean_time)
			VALUES ($1, $2, $3, $4)
		`, q, (i+1)*100, float64(i+1)*1000.5, float64(i+1)*10.5)
		require.NoError(s.T(), err)
	}
	
	config := `
receivers:
  enhancedsql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    database: ${POSTGRES_DB}
    collection_interval: 10s
    queries:
      top_queries: |
        SELECT 
          query_text,
          execution_count,
          total_time,
          mean_time
        FROM enhanced_sql_test
        ORDER BY total_time DESC
        LIMIT 10

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [enhancedsql]
      exporters: [debug]
`
	
	// Restart collector
	s.collector.Stop()
	err = s.collector.Start(config)
	require.NoError(s.T(), err)
	defer s.collector.Stop()
	
	// Wait for collection
	time.Sleep(15 * time.Second)
	
	// Verify enhanced metrics
	logs, err := s.collector.GetLogs()
	require.NoError(s.T(), err)
	
	assert.Contains(s.T(), logs, "query_text", "Should collect query text")
	assert.Contains(s.T(), logs, "execution_count", "Should collect execution count")
	assert.Contains(s.T(), logs, "mean_time", "Should collect mean time")
	
	s.T().Log("✓ Enhanced SQL receiver integration test passed")
}

// Test 9: Full Pipeline Integration
func (s *AdapterIntegrationSuite) TestFullPipelineIntegration() {
	s.T().Log("Testing full pipeline integration with all components...")
	
	// Skip if no New Relic credentials
	if s.nrdb == nil {
		s.T().Skip("New Relic credentials not configured for full pipeline test")
	}
	
	fullConfig := `
receivers:
  postgresql:
    endpoint: ${POSTGRES_HOST}:${POSTGRES_PORT}
    databases: [${POSTGRES_DB}]
    username: ${POSTGRES_USER}
    password: ${POSTGRES_PASSWORD}
    collection_interval: 10s
    tls:
      insecure: true
    
  sqlquery/queries:
    driver: postgres
    datasource: "host=${POSTGRES_HOST} port=${POSTGRES_PORT} user=${POSTGRES_USER} password=${POSTGRES_PASSWORD} dbname=${POSTGRES_DB} sslmode=disable"
    collection_interval: 10s
    queries:
      - sql: "SELECT current_database() as database, NOW() as timestamp"
        logs:
          - body_column: database
            attributes:
              log.type: "database_check"

processors:
  # All custom processors
  adaptivesampler:
    decision_wait: 5s
  circuitbreaker:
    failure_threshold: 0.5
  planattributeextractor:
    enabled: true
  querycorrelator:
    correlation_window: 1m
  verification:
    enabled: true
  costcontrol:
    enabled: true
  nrerrormonitor:
    enabled: true
    
  # Standard processors
  batch:
    timeout: 5s
  attributes:
    actions:
      - key: pipeline.test
        action: insert
        value: "full_integration"

exporters:
  otlp/newrelic:
    endpoint: ${NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
  
  nri:
    integration_name: "com.newrelic.postgresql"
    
  prometheus:
    endpoint: "0.0.0.0:8889"
    
  debug:
    verbosity: basic

service:
  pipelines:
    metrics/postgresql:
      receivers: [postgresql]
      processors: [
        verification,
        adaptivesampler,
        circuitbreaker,
        costcontrol,
        nrerrormonitor,
        attributes,
        batch
      ]
      exporters: [otlp/newrelic, nri, prometheus, debug]
    
    logs/queries:
      receivers: [sqlquery/queries]
      processors: [
        planattributeextractor,
        querycorrelator,
        verification,
        attributes
      ]
      exporters: [otlp/newrelic, debug]
`
	
	// Restart collector with full pipeline
	s.collector.Stop()
	err := s.collector.Start(fullConfig)
	require.NoError(s.T(), err)
	defer s.collector.Stop()
	
	// Generate comprehensive workload
	s.T().Log("Generating comprehensive workload...")
	s.generateComprehensiveWorkload()
	
	// Wait for full processing cycle
	time.Sleep(65 * time.Second)
	
	// Verify end-to-end functionality
	ctx, cancel := context.WithTimeout(s.ctx, 2*time.Minute)
	defer cancel()
	
	// Check metrics pipeline
	err = s.nrdb.VerifyMetric(ctx, "postgresql.backends", map[string]interface{}{
		"pipeline.test": "full_integration",
	}, "5 minutes ago")
	assert.NoError(s.T(), err, "Metrics pipeline should work end-to-end")
	
	// Check logs pipeline
	err = s.nrdb.VerifyLog(ctx, map[string]interface{}{
		"log.type":      "database_check",
		"pipeline.test": "full_integration",
	}, "5 minutes ago")
	assert.NoError(s.T(), err, "Logs pipeline should work end-to-end")
	
	s.T().Log("✓ Full pipeline integration test passed")
}

// Helper methods

func (s *AdapterIntegrationSuite) performPostgreSQLOperations() {
	// Create test table
	s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS adapter_test (
			id SERIAL PRIMARY KEY,
			data TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	
	// Perform various operations
	for i := 0; i < 20; i++ {
		s.env.PostgresDB.Exec("INSERT INTO adapter_test (data) VALUES ($1)", fmt.Sprintf("test_%d", i))
	}
	
	for i := 0; i < 10; i++ {
		rows, _ := s.env.PostgresDB.Query("SELECT * FROM adapter_test LIMIT 10")
		if rows != nil {
			rows.Close()
		}
	}
	
	s.env.PostgresDB.Exec("UPDATE adapter_test SET data = 'updated' WHERE id < 5")
	s.env.PostgresDB.Exec("DELETE FROM adapter_test WHERE id > 15")
	
	// Transaction
	tx, _ := s.env.PostgresDB.Begin()
	tx.Exec("INSERT INTO adapter_test (data) VALUES ('transaction_test')")
	tx.Commit()
	
	// Cleanup
	s.env.PostgresDB.Exec("DROP TABLE IF EXISTS adapter_test")
}

func (s *AdapterIntegrationSuite) performMySQLOperations() {
	// Create test table
	s.env.MySQLDB.Exec(`
		CREATE TABLE IF NOT EXISTS adapter_test (
			id INT AUTO_INCREMENT PRIMARY KEY,
			data VARCHAR(255),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	
	// Perform operations
	for i := 0; i < 20; i++ {
		s.env.MySQLDB.Exec("INSERT INTO adapter_test (data) VALUES (?)", fmt.Sprintf("mysql_test_%d", i))
	}
	
	rows, _ := s.env.MySQLDB.Query("SELECT COUNT(*) FROM adapter_test")
	if rows != nil {
		rows.Close()
	}
	
	// Cleanup
	s.env.MySQLDB.Exec("DROP TABLE IF EXISTS adapter_test")
}

func (s *AdapterIntegrationSuite) generateComprehensiveWorkload() {
	// 1. Normal queries
	for i := 0; i < 50; i++ {
		rows, _ := s.env.PostgresDB.Query("SELECT version()")
		if rows != nil {
			rows.Close()
		}
	}
	
	// 2. Complex queries
	s.env.PostgresDB.Exec(`
		CREATE TABLE IF NOT EXISTS workload_test (
			id SERIAL PRIMARY KEY,
			category VARCHAR(50),
			value NUMERIC,
			data JSONB
		)
	`)
	
	for i := 0; i < 100; i++ {
		category := []string{"A", "B", "C"}[i%3]
		s.env.PostgresDB.Exec(`
			INSERT INTO workload_test (category, value, data)
			VALUES ($1, $2, $3)
		`, category, rand.Float64()*1000, fmt.Sprintf(`{"index": %d}`, i))
	}
	
	// 3. Aggregations
	rows, _ := s.env.PostgresDB.Query(`
		SELECT category, COUNT(*), AVG(value), MAX(value), MIN(value)
		FROM workload_test
		GROUP BY category
	`)
	if rows != nil {
		rows.Close()
	}
	
	// 4. Joins
	rows, _ = s.env.PostgresDB.Query(`
		SELECT a.*, b.value as b_value
		FROM workload_test a
		JOIN workload_test b ON a.category = b.category
		WHERE a.id != b.id
		LIMIT 10
	`)
	if rows != nil {
		rows.Close()
	}
	
	// 5. Errors
	s.env.PostgresDB.Query("SELECT * FROM nonexistent_table")
	s.env.PostgresDB.Query("SELECT 1/0")
	
	// 6. Transactions
	tx, _ := s.env.PostgresDB.Begin()
	tx.Exec("UPDATE workload_test SET value = value * 1.1 WHERE category = 'A'")
	tx.Commit()
	
	tx, _ = s.env.PostgresDB.Begin()
	tx.Exec("DELETE FROM workload_test WHERE id > 1000")
	tx.Rollback()
	
	// Cleanup
	s.env.PostgresDB.Exec("DROP TABLE IF EXISTS workload_test")
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}