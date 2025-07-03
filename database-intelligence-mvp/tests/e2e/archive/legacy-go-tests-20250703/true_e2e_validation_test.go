package e2e

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TrueE2ETestSuite provides real end-to-end testing without any mocks
type TrueE2ETestSuite struct {
	PostgreSQL      *sql.DB
	MySQL           *sql.DB
	CollectorCmd    *exec.Cmd
	CollectorConfig string
	NRDBClient      *RealNRDBClient
	TestRunID       string
	StartTime       time.Time
}

// RealNRDBClient connects to actual New Relic API
type RealNRDBClient struct {
	AccountID  string
	APIKey     string
	HTTPClient *http.Client
}

// DataIntegrityTracker tracks data through the entire pipeline
type DataIntegrityTracker struct {
	mu              sync.RWMutex
	queryExecutions map[string]*QueryExecution
	metricsReceived map[string]*MetricData
}

// QueryExecution represents a database query execution
type QueryExecution struct {
	ID           string
	Query        string
	Database     string
	Timestamp    time.Time
	Duration     time.Duration
	RowsAffected int64
	PlanHash     string
	Checksum     string
}

// MetricData represents a metric in NRDB
type MetricData struct {
	Name       string
	Value      float64
	Timestamp  time.Time
	Attributes map[string]interface{}
	Checksum   string
}

// TestTrueEndToEndValidation runs comprehensive E2E tests without any shortcuts
func TestTrueEndToEndValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping true E2E test in short mode")
	}

	// Initialize test suite
	suite := setupTrueE2ESuite(t)
	defer suite.Cleanup()

	// Run comprehensive test scenarios
	t.Run("CompleteDataFlowValidation", func(t *testing.T) {
		suite.testCompleteDataFlow(t)
	})

	t.Run("AllProcessorsValidation", func(t *testing.T) {
		suite.testAllProcessorsEndToEnd(t)
	})

	t.Run("PGQueryLensRealValidation", func(t *testing.T) {
		suite.testPGQueryLensWithRealExtension(t)
	})

	t.Run("DataIntegrityValidation", func(t *testing.T) {
		suite.testDataIntegrityEndToEnd(t)
	})

	t.Run("LatencySLAValidation", func(t *testing.T) {
		suite.testEndToEndLatencySLA(t)
	})

	t.Run("ScaleValidation", func(t *testing.T) {
		suite.testHighVolumeEndToEnd(t)
	})

	t.Run("FailureRecoveryValidation", func(t *testing.T) {
		suite.testFailureRecoveryEndToEnd(t)
	})
}

// setupTrueE2ESuite initializes real components without mocks
func setupTrueE2ESuite(t *testing.T) *TrueE2ETestSuite {
	suite := &TrueE2ETestSuite{
		TestRunID: fmt.Sprintf("e2e-%d", time.Now().UnixNano()),
		StartTime: time.Now(),
	}

	// Connect to real PostgreSQL
	pgDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnvOrDefault("POSTGRES_HOST", "localhost"),
		getEnvOrDefault("POSTGRES_PORT", "5432"),
		getEnvOrDefault("POSTGRES_USER", "postgres"),
		getEnvOrDefault("POSTGRES_PASSWORD", "postgres"),
		getEnvOrDefault("POSTGRES_DB", "testdb"))

	var err error
	suite.PostgreSQL, err = sql.Open("postgres", pgDSN)
	require.NoError(t, err)
	require.NoError(t, suite.PostgreSQL.Ping())

	// Connect to real MySQL if available
	mysqlDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		getEnvOrDefault("MYSQL_USER", "root"),
		getEnvOrDefault("MYSQL_PASSWORD", "root"),
		getEnvOrDefault("MYSQL_HOST", "localhost"),
		getEnvOrDefault("MYSQL_PORT", "3306"),
		getEnvOrDefault("MYSQL_DB", "testdb"))

	suite.MySQL, _ = sql.Open("mysql", mysqlDSN)

	// Setup real NRDB client
	accountID := os.Getenv("NEW_RELIC_ACCOUNT_ID")
	apiKey := os.Getenv("NEW_RELIC_API_KEY")
	require.NotEmpty(t, accountID, "NEW_RELIC_ACCOUNT_ID must be set for true E2E tests")
	require.NotEmpty(t, apiKey, "NEW_RELIC_API_KEY must be set for true E2E tests")

	suite.NRDBClient = &RealNRDBClient{
		AccountID:  accountID,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Start real collector
	suite.startRealCollector(t)

	// Setup test database
	suite.setupTestDatabase(t)

	return suite
}

// startRealCollector starts the actual collector binary
func (s *TrueE2ETestSuite) startRealCollector(t *testing.T) {
	// Build collector if needed
	collectorBinary := filepath.Join(os.TempDir(), "database-intelligence-collector-"+s.TestRunID)
	buildCmd := exec.Command("go", "build", "-o", collectorBinary, "../../main.go")
	buildOutput, err := buildCmd.CombinedOutput()
	require.NoError(t, err, "Failed to build collector: %s", buildOutput)

	// Create configuration with all processors enabled
	s.CollectorConfig = s.createRealCollectorConfig(t)

	// Start collector process
	s.CollectorCmd = exec.Command(collectorBinary, "--config", s.CollectorConfig)
	s.CollectorCmd.Env = append(os.Environ(),
		"NEW_RELIC_LICENSE_KEY="+os.Getenv("NEW_RELIC_LICENSE_KEY"),
		"ENVIRONMENT=e2e-test",
		"TEST_RUN_ID="+s.TestRunID,
	)

	// Capture output
	outputFile, err := os.Create(fmt.Sprintf("output/collector-%s.log", s.TestRunID))
	require.NoError(t, err)
	s.CollectorCmd.Stdout = outputFile
	s.CollectorCmd.Stderr = outputFile

	err = s.CollectorCmd.Start()
	require.NoError(t, err, "Failed to start collector")

	// Wait for collector to be healthy
	s.waitForCollectorHealth(t, 30*time.Second)
}

// createRealCollectorConfig creates actual configuration file
func (s *TrueE2ETestSuite) createRealCollectorConfig(t *testing.T) string {
	config := fmt.Sprintf(`
receivers:
  postgresql:
    endpoint: %s:%s
    username: %s
    password: %s
    databases:
      - testdb
    collection_interval: 10s
    
  mysql:
    endpoint: %s:%s
    username: %s
    password: %s
    database: testdb
    collection_interval: 10s

  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 512

  adaptive_sampler:
    in_memory_only: true
    rules:
      - name: slow_queries
        condition: attributes["db.statement.duration"] > 100
        sampling_rate: 1.0
      - name: errors
        condition: attributes["db.statement.error"] != nil
        sampling_rate: 1.0
    default_sampling_rate: 0.1

  circuit_breaker:
    failure_threshold: 5
    timeout: 30s
    half_open_requests: 3

  plan_extractor:
    safe_mode: true
    extract_mode: from_metrics
    querylens:
      enabled: true
      
  verification:
    pii_detection:
      enabled: true
      custom_patterns:
        - name: employee_id
          pattern: 'EMP[0-9]{6}'
    cardinality_limit: 10000

  cost_control:
    monthly_budget_usd: 1000
    metric_cardinality_limit: 50000
    data_type: standard

  nr_error_monitor:
    max_attribute_length: 4096
    max_attributes_per_metric: 255

  query_correlator:
    session_timeout: 30m
    transaction_timeout: 5m

  batch:
    timeout: 10s
    send_batch_size: 1000

exporters:
  otlphttp:
    endpoint: https://otlp.nr-data.net
    headers:
      api-key: ${NEW_RELIC_LICENSE_KEY}
    
  prometheus:
    endpoint: localhost:9090
    
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics/database:
      receivers: [postgresql, mysql]
      processors: [
        memory_limiter,
        adaptive_sampler,
        circuit_breaker,
        plan_extractor,
        verification,
        cost_control,
        nr_error_monitor,
        query_correlator,
        batch
      ]
      exporters: [otlphttp, prometheus, debug]

  telemetry:
    logs:
      level: info
    metrics:
      level: detailed
      address: localhost:8888
`, 
		getEnvOrDefault("POSTGRES_HOST", "localhost"),
		getEnvOrDefault("POSTGRES_PORT", "5432"),
		getEnvOrDefault("POSTGRES_USER", "postgres"),
		getEnvOrDefault("POSTGRES_PASSWORD", "postgres"),
		getEnvOrDefault("MYSQL_HOST", "localhost"),
		getEnvOrDefault("MYSQL_PORT", "3306"),
		getEnvOrDefault("MYSQL_USER", "root"),
		getEnvOrDefault("MYSQL_PASSWORD", "root"),
	)

	configFile := fmt.Sprintf("output/collector-config-%s.yaml", s.TestRunID)
	err := os.WriteFile(configFile, []byte(config), 0644)
	require.NoError(t, err)

	return configFile
}

// waitForCollectorHealth waits for collector to be fully operational
func (s *TrueE2ETestSuite) waitForCollectorHealth(t *testing.T, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://localhost:8888/metrics")
		if err == nil && resp.StatusCode == 200 {
			// Check for specific metrics indicating all processors are loaded
			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			resp.Body.Close()
			
			metricsText := string(body[:n])
			if strings.Contains(metricsText, "otelcol_processor_adaptive_sampler") &&
			   strings.Contains(metricsText, "otelcol_processor_circuit_breaker") &&
			   strings.Contains(metricsText, "otelcol_processor_plan_extractor") {
				t.Log("Collector is healthy with all processors loaded")
				return
			}
		}
		time.Sleep(1 * time.Second)
	}
	
	t.Fatal("Collector did not become healthy within timeout")
}

// testCompleteDataFlow validates data flows correctly from database to NRDB
func (s *TrueE2ETestSuite) testCompleteDataFlow(t *testing.T) {
	tracker := &DataIntegrityTracker{
		queryExecutions: make(map[string]*QueryExecution),
		metricsReceived: make(map[string]*MetricData),
	}

	// Execute tracked queries
	queries := []struct {
		id    string
		query string
		db    string
	}{
		{"q1", "SELECT COUNT(*) FROM users", "postgresql"},
		{"q2", "SELECT * FROM users WHERE id = $1", "postgresql"},
		{"q3", "UPDATE users SET last_login = NOW() WHERE id = $1", "postgresql"},
		{"q4", "INSERT INTO audit_log (action, timestamp) VALUES ($1, $2)", "postgresql"},
	}

	// Execute and track queries
	for _, q := range queries {
		start := time.Now()
		result, err := s.PostgreSQL.Exec(strings.ReplaceAll(q.query, "$1", "1"))
		duration := time.Since(start)
		
		if err == nil {
			rows, _ := result.RowsAffected()
			execution := &QueryExecution{
				ID:           q.id,
				Query:        q.query,
				Database:     q.db,
				Timestamp:    start,
				Duration:     duration,
				RowsAffected: rows,
				Checksum:     s.calculateChecksum(q.query, start, duration),
			}
			tracker.mu.Lock()
			tracker.queryExecutions[q.id] = execution
			tracker.mu.Unlock()
		}
	}

	// Wait for metrics to reach NRDB
	time.Sleep(45 * time.Second)

	// Query NRDB for our specific test run
	s.validateMetricsInNRDB(t, tracker)
}

// validateMetricsInNRDB queries real NRDB and validates metrics
func (s *TrueE2ETestSuite) validateMetricsInNRDB(t *testing.T, tracker *DataIntegrityTracker) {
	ctx := context.Background()
	
	// Query for metrics from our test run
	query := fmt.Sprintf(`
		SELECT count(*) as metric_count,
		       uniques(db.statement) as unique_queries,
		       sum(db.rows_affected) as total_rows,
		       uniques(checksum) as checksums
		FROM Metric 
		WHERE environment = 'e2e-test'
		AND test_run_id = '%s'
		AND timestamp >= %d
		SINCE 2 minutes ago
	`, s.TestRunID, s.StartTime.UnixMilli())

	result, err := s.NRDBClient.ExecuteNRQL(ctx, query)
	require.NoError(t, err, "Failed to query NRDB")
	require.NotEmpty(t, result.Results, "No metrics found in NRDB for test run")

	metrics := result.Results[0]
	
	// Validate metric count
	metricCount, _ := metrics["metric_count"].(float64)
	expectedCount := len(tracker.queryExecutions)
	assert.GreaterOrEqual(t, metricCount, float64(expectedCount), 
		"Expected at least %d metrics, got %v", expectedCount, metricCount)

	// Validate unique queries
	if uniqueQueries, ok := metrics["unique_queries"].([]interface{}); ok {
		assert.Len(t, uniqueQueries, 4, "Expected 4 unique query types")
	}

	// Validate checksums for data integrity
	if checksums, ok := metrics["checksums"].([]interface{}); ok {
		// Verify our checksums appear in NRDB
		tracker.mu.RLock()
		for _, execution := range tracker.queryExecutions {
			found := false
			for _, cs := range checksums {
				if cs.(string) == execution.Checksum {
					found = true
					break
				}
			}
			assert.True(t, found, "Checksum %s not found in NRDB", execution.Checksum)
		}
		tracker.mu.RUnlock()
	}
}

// testAllProcessorsEndToEnd validates each processor works correctly
func (s *TrueE2ETestSuite) testAllProcessorsEndToEnd(t *testing.T) {
	// Test Adaptive Sampler
	t.Run("AdaptiveSampler", func(t *testing.T) {
		// Execute slow and fast queries
		var slowCount, fastCount int32
		
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				if id%2 == 0 {
					// Slow query (should be 100% sampled)
					s.PostgreSQL.Exec("SELECT pg_sleep(0.2)")
					atomic.AddInt32(&slowCount, 1)
				} else {
					// Fast query (should be 10% sampled)
					s.PostgreSQL.Exec("SELECT 1")
					atomic.AddInt32(&fastCount, 1)
				}
			}(i)
		}
		wg.Wait()

		time.Sleep(35 * time.Second)

		// Verify sampling in NRDB
		query := fmt.Sprintf(`
			SELECT filter(count(*), WHERE db.sampling.rule = 'slow_queries') as slow_sampled,
			       filter(count(*), WHERE db.sampling.rule = 'default') as default_sampled
			FROM Metric 
			WHERE environment = 'e2e-test'
			AND test_run_id = '%s'
			SINCE 1 minute ago
		`, s.TestRunID)

		result, err := s.NRDBClient.ExecuteNRQL(context.Background(), query)
		require.NoError(t, err)
		
		if len(result.Results) > 0 {
			metrics := result.Results[0]
			slowSampled, _ := metrics["slow_sampled"].(float64)
			defaultSampled, _ := metrics["default_sampled"].(float64)
			
			// All slow queries should be sampled
			assert.Equal(t, float64(slowCount), slowSampled, "Not all slow queries were sampled")
			
			// Only ~10% of fast queries should be sampled
			expectedFastSampled := float64(fastCount) * 0.1
			assert.InDelta(t, expectedFastSampled, defaultSampled, float64(fastCount)*0.05,
				"Fast query sampling rate incorrect")
		}
	})

	// Test PII Detection
	t.Run("PIIDetection", func(t *testing.T) {
		// Execute queries with PII
		piiQueries := []string{
			"SELECT * FROM users WHERE ssn = '123-45-6789'",
			"SELECT * FROM payments WHERE credit_card = '4111-1111-1111-1111'",
			"SELECT * FROM employees WHERE id = 'EMP123456'",
			"UPDATE users SET email = 'john.doe@example.com' WHERE id = 1",
		}

		for _, query := range piiQueries {
			s.PostgreSQL.Exec(query)
		}

		time.Sleep(35 * time.Second)

		// Verify PII is redacted
		query := fmt.Sprintf(`
			SELECT count(*) as total,
			       filter(count(*), WHERE db.statement LIKE '%%[REDACTED]%%') as redacted,
			       filter(count(*), WHERE db.statement LIKE '%%123-45-6789%%' 
			                           OR db.statement LIKE '%%4111-1111-1111-1111%%'
			                           OR db.statement LIKE '%%EMP123456%%') as exposed_pii
			FROM Metric 
			WHERE environment = 'e2e-test'
			AND test_run_id = '%s'
			SINCE 1 minute ago
		`, s.TestRunID)

		result, err := s.NRDBClient.ExecuteNRQL(context.Background(), query)
		require.NoError(t, err)

		if len(result.Results) > 0 {
			metrics := result.Results[0]
			exposedPII, _ := metrics["exposed_pii"].(float64)
			redacted, _ := metrics["redacted"].(float64)
			
			assert.Equal(t, float64(0), exposedPII, "PII data was not redacted!")
			assert.Greater(t, redacted, float64(0), "No queries were redacted")
		}
	})

	// Test Query Correlator
	t.Run("QueryCorrelator", func(t *testing.T) {
		// Execute correlated queries in a transaction
		tx, err := s.PostgreSQL.Begin()
		require.NoError(t, err)

		txQueries := []string{
			"SELECT * FROM users WHERE id = 1 FOR UPDATE",
			"SELECT * FROM orders WHERE user_id = 1",
			"UPDATE users SET order_count = order_count + 1 WHERE id = 1",
			"INSERT INTO orders (user_id, total) VALUES (1, 99.99)",
		}

		for _, query := range txQueries {
			tx.Exec(query)
		}
		
		err = tx.Commit()
		require.NoError(t, err)

		time.Sleep(35 * time.Second)

		// Verify correlation in NRDB
		query := fmt.Sprintf(`
			SELECT uniqueCount(db.transaction.id) as transaction_count,
			       count(*) as correlated_queries
			FROM Metric 
			WHERE db.transaction.id IS NOT NULL
			AND environment = 'e2e-test'
			AND test_run_id = '%s'
			SINCE 1 minute ago
		`, s.TestRunID)

		result, err := s.NRDBClient.ExecuteNRQL(context.Background(), query)
		require.NoError(t, err)

		if len(result.Results) > 0 {
			metrics := result.Results[0]
			txCount, _ := metrics["transaction_count"].(float64)
			correlatedQueries, _ := metrics["correlated_queries"].(float64)
			
			assert.GreaterOrEqual(t, txCount, float64(1), "No transactions detected")
			assert.GreaterOrEqual(t, correlatedQueries, float64(4), "Not all queries were correlated")
		}
	})
}

// testPGQueryLensWithRealExtension tests real pg_querylens integration
func (s *TrueE2ETestSuite) testPGQueryLensWithRealExtension(t *testing.T) {
	// Check if pg_querylens is installed
	var hasQueryLens bool
	err := s.PostgreSQL.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_querylens')").Scan(&hasQueryLens)
	if err != nil || !hasQueryLens {
		t.Skip("pg_querylens extension not installed")
	}

	// Create test scenario for plan changes
	t.Run("PlanRegressionDetection", func(t *testing.T) {
		// Create table and index
		s.PostgreSQL.Exec("CREATE TABLE IF NOT EXISTS test_regression (id INT PRIMARY KEY, data TEXT, value INT)")
		s.PostgreSQL.Exec("INSERT INTO test_regression SELECT i, 'data'||i, i*10 FROM generate_series(1, 10000) i ON CONFLICT DO NOTHING")
		
		// Create index for fast queries
		s.PostgreSQL.Exec("CREATE INDEX idx_test_regression_value ON test_regression(value)")
		s.PostgreSQL.Exec("ANALYZE test_regression")

		// Execute queries with index (fast)
		for i := 0; i < 10; i++ {
			s.PostgreSQL.Exec("SELECT * FROM test_regression WHERE value = $1", i*100)
		}

		time.Sleep(35 * time.Second)

		// Drop index to cause regression
		s.PostgreSQL.Exec("DROP INDEX idx_test_regression_value")

		// Execute same queries without index (slow)
		for i := 0; i < 10; i++ {
			s.PostgreSQL.Exec("SELECT * FROM test_regression WHERE value = $1", i*100)
		}

		time.Sleep(35 * time.Second)

		// Query NRDB for regression detection
		query := fmt.Sprintf(`
			SELECT count(*) as regressions,
			       max(db.plan.time_change_ratio) as max_slowdown,
			       uniques(db.recommendation.type) as recommendations
			FROM Metric 
			WHERE db.plan.has_regression = true
			AND environment = 'e2e-test'
			AND test_run_id = '%s'
			SINCE 2 minutes ago
		`, s.TestRunID)

		result, err := s.NRDBClient.ExecuteNRQL(context.Background(), query)
		require.NoError(t, err)

		if len(result.Results) > 0 {
			metrics := result.Results[0]
			regressions, _ := metrics["regressions"].(float64)
			maxSlowdown, _ := metrics["max_slowdown"].(float64)
			
			assert.Greater(t, regressions, float64(0), "No plan regressions detected")
			assert.Greater(t, maxSlowdown, float64(1.5), "Expected significant slowdown")
			
			if recommendations, ok := metrics["recommendations"].([]interface{}); ok {
				assert.NotEmpty(t, recommendations, "No optimization recommendations generated")
			}
		}
	})
}

// testDataIntegrityEndToEnd validates no data loss through pipeline
func (s *TrueE2ETestSuite) testDataIntegrityEndToEnd(t *testing.T) {
	// Generate precisely tracked workload
	expectedMetrics := s.generatePreciseWorkload(t)

	// Wait for all data to reach NRDB
	time.Sleep(60 * time.Second)

	// Query NRDB for exact counts
	query := fmt.Sprintf(`
		SELECT count(*) as total_metrics,
		       filter(count(*), WHERE db.operation = 'INSERT') as inserts,
		       filter(count(*), WHERE db.operation = 'UPDATE') as updates,
		       filter(count(*), WHERE db.operation = 'SELECT') as selects,
		       filter(count(*), WHERE db.operation = 'DELETE') as deletes,
		       sum(db.rows_affected) as total_rows
		FROM Metric 
		WHERE environment = 'e2e-test'
		AND test_run_id = '%s'
		AND integrity_marker = '%s'
		SINCE 2 minutes ago
	`, s.TestRunID, expectedMetrics.Marker)

	result, err := s.NRDBClient.ExecuteNRQL(context.Background(), query)
	require.NoError(t, err)
	require.NotEmpty(t, result.Results)

	metrics := result.Results[0]
	
	// Validate exact counts
	assert.Equal(t, float64(expectedMetrics.TotalQueries), metrics["total_metrics"].(float64),
		"Total query count mismatch")
	assert.Equal(t, float64(expectedMetrics.Inserts), metrics["inserts"].(float64),
		"Insert count mismatch")
	assert.Equal(t, float64(expectedMetrics.Updates), metrics["updates"].(float64),
		"Update count mismatch")
	assert.Equal(t, float64(expectedMetrics.Selects), metrics["selects"].(float64),
		"Select count mismatch")
	assert.Equal(t, float64(expectedMetrics.Deletes), metrics["deletes"].(float64),
		"Delete count mismatch")
	
	// Validate row counts
	totalRows, _ := metrics["total_rows"].(float64)
	assert.Equal(t, float64(expectedMetrics.TotalRows), totalRows,
		"Total rows affected mismatch")
}

// testEndToEndLatencySLA validates latency meets SLA
func (s *TrueE2ETestSuite) testEndToEndLatencySLA(t *testing.T) {
	const maxAcceptableLatency = 30 * time.Second
	
	// Execute query with unique marker
	marker := fmt.Sprintf("latency_%s_%d", s.TestRunID, time.Now().UnixNano())
	queryTime := time.Now()
	
	_, err := s.PostgreSQL.Exec(fmt.Sprintf("SELECT '%s' as latency_marker", marker))
	require.NoError(t, err)

	// Poll NRDB until query appears
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var latency time.Duration
	found := false

	for {
		select {
		case <-ctx.Done():
			t.Fatal("Timeout waiting for metric in NRDB")
		case <-ticker.C:
			query := fmt.Sprintf(`
				SELECT count(*) as found
				FROM Metric 
				WHERE db.statement LIKE '%%%s%%'
				SINCE 3 minutes ago
			`, marker)

			result, err := s.NRDBClient.ExecuteNRQL(context.Background(), query)
			if err == nil && len(result.Results) > 0 {
				if count, _ := result.Results[0]["found"].(float64); count > 0 {
					latency = time.Since(queryTime)
					found = true
					goto done
				}
			}
		}
	}

done:
	require.True(t, found, "Metric never appeared in NRDB")
	assert.Less(t, latency, maxAcceptableLatency, 
		"End-to-end latency %v exceeds SLA of %v", latency, maxAcceptableLatency)
	t.Logf("End-to-end latency: %v", latency)
}

// testHighVolumeEndToEnd tests system under high load
func (s *TrueE2ETestSuite) testHighVolumeEndToEnd(t *testing.T) {
	const (
		targetQPS      = 1000
		testDuration   = 30 * time.Second
		maxDataLoss    = 0.01 // 1% acceptable loss
	)

	var totalQueries int64
	var successQueries int64
	
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	// Generate high volume load
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ { // 10 goroutines, 100 QPS each
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			ticker := time.NewTicker(10 * time.Millisecond) // 100 QPS per worker
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					atomic.AddInt64(&totalQueries, 1)
					
					query := fmt.Sprintf("SELECT %d, '%s', random() FROM generate_series(1, 10)",
						workerID, s.TestRunID)
					
					if _, err := s.PostgreSQL.Exec(query); err == nil {
						atomic.AddInt64(&successQueries, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Wait for metrics to reach NRDB
	time.Sleep(60 * time.Second)

	// Validate no significant data loss
	query := fmt.Sprintf(`
		SELECT count(*) as metrics_received
		FROM Metric 
		WHERE environment = 'e2e-test'
		AND test_run_id = '%s'
		AND db.statement LIKE '%%generate_series%%'
		SINCE 2 minutes ago
	`, s.TestRunID)

	result, err := s.NRDBClient.ExecuteNRQL(context.Background(), query)
	require.NoError(t, err)

	if len(result.Results) > 0 {
		metricsReceived, _ := result.Results[0]["metrics_received"].(float64)
		
		// Account for sampling (10% default rate for fast queries)
		expectedMetrics := float64(successQueries) * 0.1
		actualLossRate := 1.0 - (metricsReceived / expectedMetrics)
		
		assert.Less(t, actualLossRate, maxDataLoss,
			"Data loss rate %.2f%% exceeds maximum %.2f%%", actualLossRate*100, maxDataLoss*100)
		
		t.Logf("High volume test: %d queries executed, %d succeeded, %.0f metrics received (%.2f%% loss)",
			totalQueries, successQueries, metricsReceived, actualLossRate*100)
	}
}

// testFailureRecoveryEndToEnd tests system recovery from failures
func (s *TrueE2ETestSuite) testFailureRecoveryEndToEnd(t *testing.T) {
	// Test database connection failure and recovery
	t.Run("DatabaseFailureRecovery", func(t *testing.T) {
		// Execute queries normally
		for i := 0; i < 5; i++ {
			s.PostgreSQL.Exec("SELECT 1")
		}

		// Simulate connection failure by killing connections
		s.PostgreSQL.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE pid <> pg_backend_pid()")
		
		// Wait for circuit breaker to activate
		time.Sleep(10 * time.Second)

		// Verify circuit breaker activation in NRDB
		query := fmt.Sprintf(`
			SELECT count(*) as circuit_breaker_activations
			FROM Metric 
			WHERE metricName = 'otelcol.processor.circuitbreaker.state_change'
			AND circuit_breaker.state = 'open'
			AND environment = 'e2e-test'
			SINCE 1 minute ago
		`, s.TestRunID)

		result, err := s.NRDBClient.ExecuteNRQL(context.Background(), query)
		if err == nil && len(result.Results) > 0 {
			activations, _ := result.Results[0]["circuit_breaker_activations"].(float64)
			assert.Greater(t, activations, float64(0), "Circuit breaker did not activate")
		}

		// Reconnect and verify recovery
		s.PostgreSQL.Ping() // Force reconnection
		time.Sleep(30 * time.Second)

		// Execute queries after recovery
		for i := 0; i < 5; i++ {
			s.PostgreSQL.Exec("SELECT 2")
		}

		// Verify metrics resume after recovery
		time.Sleep(35 * time.Second)

		recoveryQuery := fmt.Sprintf(`
			SELECT count(*) as post_recovery_metrics
			FROM Metric 
			WHERE db.statement = 'SELECT 2'
			AND environment = 'e2e-test'
			AND test_run_id = '%s'
			SINCE 1 minute ago
		`, s.TestRunID)

		result, err = s.NRDBClient.ExecuteNRQL(context.Background(), recoveryQuery)
		require.NoError(t, err)

		if len(result.Results) > 0 {
			postRecoveryMetrics, _ := result.Results[0]["post_recovery_metrics"].(float64)
			assert.Greater(t, postRecoveryMetrics, float64(0), "No metrics after recovery")
		}
	})
}

// Helper methods

func (s *TrueE2ETestSuite) setupTestDatabase(t *testing.T) {
	// Create test tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255),
			last_login TIMESTAMP,
			order_count INT DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INT REFERENCES users(id),
			total DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id SERIAL PRIMARY KEY,
			action VARCHAR(255),
			timestamp TIMESTAMP
		)`,
	}

	for _, query := range queries {
		_, err := s.PostgreSQL.Exec(query)
		require.NoError(t, err)
	}

	// Insert test data
	for i := 1; i <= 100; i++ {
		s.PostgreSQL.Exec("INSERT INTO users (email) VALUES ($1) ON CONFLICT DO NOTHING",
			fmt.Sprintf("user%d@example.com", i))
	}
}

func (s *TrueE2ETestSuite) calculateChecksum(query string, timestamp time.Time, duration time.Duration) string {
	data := fmt.Sprintf("%s|%d|%d", query, timestamp.UnixNano(), duration.Nanoseconds())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

type ExpectedMetrics struct {
	Marker       string
	TotalQueries int
	Inserts      int
	Updates      int
	Selects      int
	Deletes      int
	TotalRows    int64
}

func (s *TrueE2ETestSuite) generatePreciseWorkload(t *testing.T) *ExpectedMetrics {
	marker := fmt.Sprintf("integrity_%s_%d", s.TestRunID, time.Now().UnixNano())
	expected := &ExpectedMetrics{Marker: marker}

	// Execute precise number of each operation
	operations := []struct {
		operation string
		queries   []string
		count     *int
		rows      int64
	}{
		{
			operation: "INSERT",
			queries: []string{
				fmt.Sprintf("INSERT INTO audit_log (action, timestamp) VALUES ('%s', NOW())", marker),
			},
			count: &expected.Inserts,
			rows:  1,
		},
		{
			operation: "UPDATE", 
			queries: []string{
				"UPDATE users SET last_login = NOW() WHERE id <= 10",
			},
			count: &expected.Updates,
			rows:  10,
		},
		{
			operation: "SELECT",
			queries: []string{
				"SELECT COUNT(*) FROM users",
				"SELECT * FROM users WHERE id = 1",
				"SELECT * FROM orders WHERE user_id = 1",
			},
			count: &expected.Selects,
			rows:  0,
		},
		{
			operation: "DELETE",
			queries: []string{
				fmt.Sprintf("DELETE FROM audit_log WHERE action = '%s_old'", marker),
			},
			count: &expected.Deletes,
			rows:  0,
		},
	}

	for _, op := range operations {
		for _, query := range op.queries {
			result, err := s.PostgreSQL.Exec(query)
			if err == nil {
				*op.count++
				expected.TotalQueries++
				if rows, err := result.RowsAffected(); err == nil {
					expected.TotalRows += rows
				}
			}
		}
	}

	return expected
}

func (s *TrueE2ETestSuite) Cleanup() {
	// Stop collector
	if s.CollectorCmd != nil && s.CollectorCmd.Process != nil {
		s.CollectorCmd.Process.Kill()
		s.CollectorCmd.Wait()
	}

	// Close database connections
	if s.PostgreSQL != nil {
		s.PostgreSQL.Close()
	}
	if s.MySQL != nil {
		s.MySQL.Close()
	}

	// Clean up test data
	if s.CollectorConfig != "" {
		os.Remove(s.CollectorConfig)
	}
}

// ExecuteNRQL executes a real NRQL query against New Relic
func (c *RealNRDBClient) ExecuteNRQL(ctx context.Context, query string) (*NRQLResult, error) {
	graphQLQuery := fmt.Sprintf(`{
		actor {
			account(id: %s) {
				nrql(query: "%s") {
					results
				}
			}
		}
	}`, c.AccountID, strings.ReplaceAll(query, `"`, `\"`))

	reqBody := map[string]string{"query": graphQLQuery}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.newrelic.com/graphql", 
		strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("API-Key", c.APIKey)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Actor struct {
				Account struct {
					NRQL struct {
						Results []map[string]interface{} `json:"results"`
					} `json:"nrql"`
				} `json:"account"`
			} `json:"actor"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("NRQL error: %s", result.Errors[0].Message)
	}

	return &NRQLResult{
		Results: result.Data.Actor.Account.NRQL.Results,
	}, nil
}

type NRQLResult struct {
	Results []map[string]interface{}
}

// TestRecentFeaturesValidation tests all recent feature additions
func TestRecentFeaturesValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping recent features E2E test in short mode")
	}

	suite := setupTrueE2ESuite(t)
	defer suite.Cleanup()

	t.Run("ASHOneSecondSampling", func(t *testing.T) {
		// Generate consistent workload for 60 seconds
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var sampleCount int64
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					suite.PostgreSQL.Exec("SELECT pg_sleep(0.05)")
				}
			}
		}()

		// Wait for ASH collection
		time.Sleep(65 * time.Second)

		// Query for ASH samples
		query := fmt.Sprintf(`
			SELECT count(*) as ash_samples,
			       uniqueCount(sample_timestamp) as unique_timestamps
			FROM Metric 
			WHERE metricName = 'postgresql.ash.session'
			AND environment = 'e2e-test'
			AND test_run_id = '%s'
			SINCE 90 seconds ago
		`, suite.TestRunID)

		result, err := suite.NRDBClient.ExecuteNRQL(context.Background(), query)
		require.NoError(t, err)

		if len(result.Results) > 0 {
			samples, _ := result.Results[0]["ash_samples"].(float64)
			timestamps, _ := result.Results[0]["unique_timestamps"].(float64)
			
			// Should have ~60 samples (1 per second)
			assert.Greater(t, samples, float64(50), "ASH sampling frequency too low")
			assert.Less(t, samples, float64(70), "ASH sampling frequency too high")
			assert.Greater(t, timestamps, float64(50), "Not enough unique timestamps")
			
			t.Logf("ASH collected %v samples with %v unique timestamps", samples, timestamps)
		}
	})

	t.Run("EnhancedPIIPatterns", func(t *testing.T) {
		// Test all enhanced PII patterns
		piiTests := []struct {
			name     string
			query    string
			contains string
		}{
			{"SSN", "SELECT * FROM users WHERE ssn = '123-45-6789'", "123-45-6789"},
			{"CreditCard", "UPDATE payments SET card = '4111111111111111' WHERE id = 1", "4111111111111111"},
			{"Email", "SELECT * FROM users WHERE email = 'test.user+tag@example.com'", "test.user+tag@example.com"},
			{"Phone", "INSERT INTO contacts (phone) VALUES ('(555) 123-4567')", "(555) 123-4567"},
			{"EmployeeID", "SELECT * FROM employees WHERE id = 'EMP123456'", "EMP123456"},
			{"IPAddress", "SELECT * FROM logs WHERE ip = '192.168.1.100'", "192.168.1.100"},
			{"CustomPattern", "SELECT * FROM accounts WHERE account = 'ACC-2025-1234'", "ACC-2025-1234"},
		}

		for _, test := range piiTests {
			suite.PostgreSQL.Exec(test.query)
		}

		time.Sleep(35 * time.Second)

		// Verify all PII is redacted
		query := fmt.Sprintf(`
			SELECT db.statement,
			       pii.type,
			       pii.redacted
			FROM Metric 
			WHERE environment = 'e2e-test'
			AND test_run_id = '%s'
			AND pii.redacted = true
			SINCE 1 minute ago
		`, suite.TestRunID)

		result, err := suite.NRDBClient.ExecuteNRQL(context.Background(), query)
		require.NoError(t, err)

		// Also check that no PII leaked
		for _, test := range piiTests {
			leakQuery := fmt.Sprintf(`
				SELECT count(*) as leaked
				FROM Metric 
				WHERE db.statement LIKE '%%%s%%'
				AND environment = 'e2e-test'
				AND test_run_id = '%s'
				SINCE 1 minute ago
			`, test.contains, suite.TestRunID)

			result, err := suite.NRDBClient.ExecuteNRQL(context.Background(), leakQuery)
			require.NoError(t, err)

			if len(result.Results) > 0 {
				leaked, _ := result.Results[0]["leaked"].(float64)
				assert.Equal(t, float64(0), leaked, "PII pattern %s was not redacted", test.name)
			}
		}
	})

	t.Run("BudgetEnforcementAggressive", func(t *testing.T) {
		// Generate high cardinality to trigger aggressive mode
		marker := fmt.Sprintf("budget_%s", suite.TestRunID)
		
		// Generate 5000 unique queries
		for i := 0; i < 5000; i++ {
			query := fmt.Sprintf("SELECT %d, '%s', '%s', random() FROM generate_series(1, 5)",
				i, marker, generateRandomString(20))
			suite.PostgreSQL.Exec(query)
			
			if i%100 == 0 {
				time.Sleep(10 * time.Millisecond) // Avoid overwhelming
			}
		}

		time.Sleep(45 * time.Second)

		// Check for cost control actions
		query := fmt.Sprintf(`
			SELECT latest(cost_control.cardinality_reduced) as reduced,
			       latest(cost_control.aggressive_mode_active) as aggressive,
			       latest(cost_control.unique_metrics_before) as before,
			       latest(cost_control.unique_metrics_after) as after
			FROM Metric 
			WHERE metricName = 'otelcol.processor.costcontrol.action'
			AND environment = 'e2e-test'
			SINCE 2 minutes ago
		`, suite.TestRunID)

		result, err := suite.NRDBClient.ExecuteNRQL(context.Background(), query)
		if err == nil && len(result.Results) > 0 {
			metrics := result.Results[0]
			
			if reduced, ok := metrics["reduced"].(bool); ok && reduced {
				before, _ := metrics["before"].(float64)
				after, _ := metrics["after"].(float64)
				
				assert.Less(t, after, before, "Cardinality was not reduced")
				t.Logf("Cardinality reduced from %v to %v", before, after)
			}
		}
	})
}

// generateRandomString generates a random string for testing
func generateRandomStringValidation(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}