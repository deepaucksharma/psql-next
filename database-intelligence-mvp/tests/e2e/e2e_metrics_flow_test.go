// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/otelcol"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// E2EMetricsFlowTestSuite contains comprehensive end-to-end tests for the entire metrics flow
type E2EMetricsFlowTestSuite struct {
	suite.Suite
	logger              *zap.Logger
	pgContainer         *postgres.PostgresContainer
	mysqlContainer      *mysql.MySQLContainer
	collector           *otelcol.Collector
	pgDB                *sql.DB
	mysqlDB             *sql.DB
	testDataDir         string
	workloadGenerators  map[string]*WorkloadGenerator
	metricValidator     *MetricValidator
	performanceBench    *PerformanceBenchmark
	reportGenerator     *TestReportGenerator
	nrdbValidator       *NRDBValidator
	resourceMonitor     *ResourceMonitor
	stressTestManager   *StressTestManager
	
	// Test configuration
	testConfig          *E2ETestConfig
	testResults         *E2ETestResults
	
	// Test control
	testMutex           sync.RWMutex
	activeWorkloads     map[string]*WorkloadSession
	testStartTime       time.Time
	cleanupFunctions    []func()
}

// E2ETestConfig defines configuration for E2E tests
type E2ETestConfig struct {
	// Database configuration
	PostgreSQLConfig DatabaseConfig `json:"postgresql_config"`
	MySQLConfig      DatabaseConfig `json:"mysql_config"`
	
	// Workload configuration
	WorkloadDuration      time.Duration `json:"workload_duration"`
	WorkloadConcurrency   int          `json:"workload_concurrency"`
	WorkloadTypes         []string     `json:"workload_types"`
	
	// Metric validation configuration
	MetricValidationRules []ValidationRule `json:"metric_validation_rules"`
	NRDBValidationQueries []NRDBQuery      `json:"nrdb_validation_queries"`
	
	// Performance testing configuration
	LoadTestDuration      time.Duration `json:"load_test_duration"`
	MaxConcurrentQueries  int          `json:"max_concurrent_queries"`
	StressTestScenarios   []StressTestScenario `json:"stress_test_scenarios"`
	
	// NRDB integration
	NewRelicConfig       NewRelicConfig `json:"new_relic_config"`
	
	// Test thresholds
	Thresholds           TestThresholds `json:"thresholds"`
}

// DatabaseConfig defines database-specific configuration
type DatabaseConfig struct {
	Host            string            `json:"host"`
	Port            int               `json:"port"`
	Database        string            `json:"database"`
	Username        string            `json:"username"`
	Password        string            `json:"password"`
	ConnectionPool  int               `json:"connection_pool"`
	InitQueries     []string          `json:"init_queries"`
	Extensions      []string          `json:"extensions"`
	CustomConfig    map[string]string `json:"custom_config"`
}

// ValidationRule defines a metric validation rule
type ValidationRule struct {
	Name            string                 `json:"name"`
	MetricName      string                 `json:"metric_name"`
	ExpectedValue   interface{}            `json:"expected_value"`
	Tolerance       float64                `json:"tolerance"`
	Operator        string                 `json:"operator"` // gt, lt, eq, ne, between
	Attributes      map[string]interface{} `json:"attributes"`
	Description     string                 `json:"description"`
	Critical        bool                   `json:"critical"`
}

// NRDBQuery defines a New Relic database query for validation
type NRDBQuery struct {
	Name            string        `json:"name"`
	Query           string        `json:"query"`
	ExpectedResults int           `json:"expected_results"`
	Timeout         time.Duration `json:"timeout"`
	RetryCount      int           `json:"retry_count"`
	Critical        bool          `json:"critical"`
}

// StressTestScenario defines a stress testing scenario
type StressTestScenario struct {
	Name                string        `json:"name"`
	Duration            time.Duration `json:"duration"`
	ConcurrentUsers     int           `json:"concurrent_users"`
	QueryPattern        string        `json:"query_pattern"`
	ResourceLimits      ResourceLimits `json:"resource_limits"`
	ExpectedFailureRate float64       `json:"expected_failure_rate"`
}

// ResourceLimits defines resource constraints for stress testing
type ResourceLimits struct {
	MaxMemoryMB   int `json:"max_memory_mb"`
	MaxCPUPercent int `json:"max_cpu_percent"`
	MaxDiskMB     int `json:"max_disk_mb"`
}

// NewRelicConfig defines New Relic integration configuration
type NewRelicConfig struct {
	APIKey        string `json:"api_key"`
	AccountID     string `json:"account_id"`
	Region        string `json:"region"`
	LicenseKey    string `json:"license_key"`
	OTLPEndpoint  string `json:"otlp_endpoint"`
}

// TestThresholds defines pass/fail thresholds for tests
type TestThresholds struct {
	MaxLatencyMS         int     `json:"max_latency_ms"`
	MinThroughputRPS     float64 `json:"min_throughput_rps"`
	MaxErrorRate         float64 `json:"max_error_rate"`
	MinDataQualityScore  float64 `json:"min_data_quality_score"`
	MaxMemoryUsageMB     int     `json:"max_memory_usage_mb"`
	MaxCPUUsagePercent   float64 `json:"max_cpu_usage_percent"`
	MinEntityCorrelation float64 `json:"min_entity_correlation"`
	MaxCardinalityScore  float64 `json:"max_cardinality_score"`
}

// E2ETestResults contains comprehensive test results
type E2ETestResults struct {
	TestSuite          string                     `json:"test_suite"`
	StartTime          time.Time                  `json:"start_time"`
	EndTime            time.Time                  `json:"end_time"`
	Duration           time.Duration              `json:"duration"`
	TotalTests         int                        `json:"total_tests"`
	PassedTests        int                        `json:"passed_tests"`
	FailedTests        int                        `json:"failed_tests"`
	SkippedTests       int                        `json:"skipped_tests"`
	
	// Performance metrics
	PerformanceMetrics PerformanceMetrics         `json:"performance_metrics"`
	
	// Database-specific results
	DatabaseResults    map[string]*DatabaseResults `json:"database_results"`
	
	// Validation results
	ValidationResults  []ValidationResult         `json:"validation_results"`
	
	// NRDB validation results
	NRDBResults       []NRDBValidationResult     `json:"nrdb_results"`
	
	// Stress test results
	StressTestResults []StressTestResult         `json:"stress_test_results"`
	
	// Resource usage
	ResourceUsage     ResourceUsageMetrics       `json:"resource_usage"`
	
	// Quality metrics
	QualityMetrics    QualityMetrics             `json:"quality_metrics"`
	
	// Errors and warnings
	Errors            []TestError                `json:"errors"`
	Warnings          []TestWarning              `json:"warnings"`
	
	// Test artifacts
	Artifacts         []TestArtifact             `json:"artifacts"`
}

// PerformanceMetrics contains performance test results
type PerformanceMetrics struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	AverageLatencyMS   float64       `json:"average_latency_ms"`
	P95LatencyMS       float64       `json:"p95_latency_ms"`
	P99LatencyMS       float64       `json:"p99_latency_ms"`
	ThroughputRPS      float64       `json:"throughput_rps"`
	ErrorRate          float64       `json:"error_rate"`
	DataTransferMB     float64       `json:"data_transfer_mb"`
}

// DatabaseResults contains database-specific test results
type DatabaseResults struct {
	DatabaseType       string    `json:"database_type"`
	QueriesExecuted    int64     `json:"queries_executed"`
	QueriesSuccessful  int64     `json:"queries_successful"`
	QueriesFailed      int64     `json:"queries_failed"`
	AverageQueryTimeMS float64   `json:"average_query_time_ms"`
	MetricsCollected   int64     `json:"metrics_collected"`
	LastMetricTime     time.Time `json:"last_metric_time"`
	PIIViolations      int       `json:"pii_violations"`
	CircuitBreakerTrips int      `json:"circuit_breaker_trips"`
}

// ValidationResult contains validation test results
type ValidationResult struct {
	RuleName        string      `json:"rule_name"`
	MetricName      string      `json:"metric_name"`
	ExpectedValue   interface{} `json:"expected_value"`
	ActualValue     interface{} `json:"actual_value"`
	Passed          bool        `json:"passed"`
	Tolerance       float64     `json:"tolerance"`
	ErrorMessage    string      `json:"error_message,omitempty"`
	Timestamp       time.Time   `json:"timestamp"`
	Critical        bool        `json:"critical"`
}

// NRDBValidationResult contains NRDB validation results
type NRDBValidationResult struct {
	QueryName       string    `json:"query_name"`
	Query           string    `json:"query"`
	ExpectedResults int       `json:"expected_results"`
	ActualResults   int       `json:"actual_results"`
	Passed          bool      `json:"passed"`
	ExecutionTimeMS int64     `json:"execution_time_ms"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
	Critical        bool      `json:"critical"`
}

// StressTestResult contains stress test results
type StressTestResult struct {
	ScenarioName    string          `json:"scenario_name"`
	Duration        time.Duration   `json:"duration"`
	ConcurrentUsers int             `json:"concurrent_users"`
	TotalRequests   int64           `json:"total_requests"`
	FailedRequests  int64           `json:"failed_requests"`
	FailureRate     float64         `json:"failure_rate"`
	ResourcePeaks   ResourcePeaks   `json:"resource_peaks"`
	Passed          bool            `json:"passed"`
	ErrorMessages   []string        `json:"error_messages"`
}

// ResourcePeaks contains peak resource usage during stress tests
type ResourcePeaks struct {
	MaxMemoryMB     int     `json:"max_memory_mb"`
	MaxCPUPercent   float64 `json:"max_cpu_percent"`
	MaxDiskUsageMB  int     `json:"max_disk_usage_mb"`
	MaxNetworkMbps  float64 `json:"max_network_mbps"`
}

// ResourceUsageMetrics contains resource usage metrics
type ResourceUsageMetrics struct {
	AverageMemoryMB     float64 `json:"average_memory_mb"`
	PeakMemoryMB        int     `json:"peak_memory_mb"`
	AverageCPUPercent   float64 `json:"average_cpu_percent"`
	PeakCPUPercent      float64 `json:"peak_cpu_percent"`
	AverageDiskUsageMB  float64 `json:"average_disk_usage_mb"`
	PeakDiskUsageMB     int     `json:"peak_disk_usage_mb"`
	NetworkTransferMB   float64 `json:"network_transfer_mb"`
}

// QualityMetrics contains data quality metrics
type QualityMetrics struct {
	DataQualityScore     float64 `json:"data_quality_score"`
	EntityCorrelationRate float64 `json:"entity_correlation_rate"`
	PIISanitizationRate  float64 `json:"pii_sanitization_rate"`
	QueryNormalizationRate float64 `json:"query_normalization_rate"`
	CardinalityScore     float64 `json:"cardinality_score"`
	SchemaComplianceRate float64 `json:"schema_compliance_rate"`
}

// TestError represents a test error
type TestError struct {
	Test        string    `json:"test"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"`
	Timestamp   time.Time `json:"timestamp"`
	Context     string    `json:"context"`
	Stacktrace  string    `json:"stacktrace,omitempty"`
}

// TestWarning represents a test warning
type TestWarning struct {
	Test        string    `json:"test"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Context     string    `json:"context"`
}

// TestArtifact represents a test artifact
type TestArtifact struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
}

// WorkloadSession represents an active workload session
type WorkloadSession struct {
	ID          string
	Type        string
	StartTime   time.Time
	Generator   *WorkloadGenerator
	Cancel      context.CancelFunc
	Stats       *WorkloadStats
	Errors      []error
}

// WorkloadStats contains workload execution statistics
type WorkloadStats struct {
	QueriesExecuted    int64
	QueriesSuccessful  int64
	QueriesFailed      int64
	AverageLatencyMS   float64
	TotalDataBytes     int64
	LastQueryTime      time.Time
}

// SetupSuite initializes the test environment
func (suite *E2EMetricsFlowTestSuite) SetupSuite() {
	suite.logger = zaptest.NewLogger(suite.T())
	suite.testStartTime = time.Now()
	
	// Initialize test configuration
	suite.testConfig = suite.loadTestConfig()
	suite.testResults = &E2ETestResults{
		TestSuite:         "E2E Metrics Flow",
		StartTime:         suite.testStartTime,
		DatabaseResults:   make(map[string]*DatabaseResults),
		ValidationResults: make([]ValidationResult, 0),
		NRDBResults:      make([]NRDBValidationResult, 0),
		StressTestResults: make([]StressTestResult, 0),
		Errors:           make([]TestError, 0),
		Warnings:         make([]TestWarning, 0),
		Artifacts:        make([]TestArtifact, 0),
	}
	
	// Create test data directory
	suite.testDataDir = filepath.Join(os.TempDir(), fmt.Sprintf("e2e-metrics-flow-tests-%d", time.Now().Unix()))
	err := os.MkdirAll(suite.testDataDir, 0755)
	require.NoError(suite.T(), err)
	
	// Initialize test components
	suite.activeWorkloads = make(map[string]*WorkloadSession)
	suite.workloadGenerators = make(map[string]*WorkloadGenerator)
	
	// Start test containers
	suite.startTestContainers()
	
	// Setup test databases
	suite.setupTestDatabases()
	
	// Initialize validators and utilities
	suite.initializeTestUtilities()
	
	// Start resource monitoring
	suite.startResourceMonitoring()
	
	suite.logger.Info("E2E test suite initialized successfully")
}

// TearDownSuite cleans up the test environment
func (suite *E2EMetricsFlowTestSuite) TearDownSuite() {
	suite.logger.Info("Starting E2E test suite cleanup")
	
	// Stop all active workloads
	for _, session := range suite.activeWorkloads {
		if session.Cancel != nil {
			session.Cancel()
		}
	}
	
	// Stop resource monitoring
	if suite.resourceMonitor != nil {
		suite.resourceMonitor.Stop()
	}
	
	// Shutdown collector
	if suite.collector != nil {
		suite.collector.Shutdown()
	}
	
	// Close database connections
	if suite.pgDB != nil {
		suite.pgDB.Close()
	}
	if suite.mysqlDB != nil {
		suite.mysqlDB.Close()
	}
	
	// Terminate containers
	if suite.pgContainer != nil {
		suite.pgContainer.Terminate(context.Background())
	}
	if suite.mysqlContainer != nil {
		suite.mysqlContainer.Terminate(context.Background())
	}
	
	// Execute cleanup functions
	for _, cleanup := range suite.cleanupFunctions {
		cleanup()
	}
	
	// Finalize test results
	suite.finalizeTestResults()
	
	// Generate comprehensive test report
	suite.generateTestReport()
	
	suite.logger.Info("E2E test suite cleanup completed")
}

// TestPostgreSQLMetricsFlow tests PostgreSQL metrics collection end-to-end
func (suite *E2EMetricsFlowTestSuite) TestPostgreSQLMetricsFlow() {
	suite.logger.Info("Starting PostgreSQL metrics flow test")
	
	// Start PostgreSQL-specific workload
	workloadSession := suite.startWorkload("postgresql", "mixed_oltp_olap")
	defer suite.stopWorkload(workloadSession.ID)
	
	// Start collector with PostgreSQL configuration
	collectorConfig := suite.createPostgreSQLCollectorConfig()
	collector := suite.startCollector(collectorConfig)
	defer collector.Shutdown()
	
	// Wait for metric collection to stabilize
	time.Sleep(30 * time.Second)
	
	// Run comprehensive validation
	suite.validatePostgreSQLMetrics()
	
	// Test PII sanitization with realistic data
	suite.testPIISanitization("postgresql")
	
	// Test adaptive sampling behavior
	suite.testAdaptiveSampling("postgresql")
	
	// Test circuit breaker functionality
	suite.testCircuitBreaker("postgresql")
	
	// Validate metrics in NRDB
	suite.validateNRDBData("postgresql")
	
	suite.logger.Info("PostgreSQL metrics flow test completed")
}

// TestMySQLMetricsFlow tests MySQL metrics collection end-to-end
func (suite *E2EMetricsFlowTestSuite) TestMySQLMetricsFlow() {
	suite.logger.Info("Starting MySQL metrics flow test")
	
	// Start MySQL-specific workload
	workloadSession := suite.startWorkload("mysql", "mixed_oltp_olap")
	defer suite.stopWorkload(workloadSession.ID)
	
	// Start collector with MySQL configuration
	collectorConfig := suite.createMySQLCollectorConfig()
	collector := suite.startCollector(collectorConfig)
	defer collector.Shutdown()
	
	// Wait for metric collection to stabilize
	time.Sleep(30 * time.Second)
	
	// Run comprehensive validation
	suite.validateMySQLMetrics()
	
	// Test performance schema metrics
	suite.testPerformanceSchemaMetrics()
	
	// Test slow query detection
	suite.testSlowQueryDetection("mysql")
	
	// Test infrastructure metrics
	suite.testInfrastructureMetrics("mysql")
	
	// Validate metrics in NRDB
	suite.validateNRDBData("mysql")
	
	suite.logger.Info("MySQL metrics flow test completed")
}

// TestQueryPerformanceTracking tests query performance tracking with actual workloads
func (suite *E2EMetricsFlowTestSuite) TestQueryPerformanceTracking() {
	suite.logger.Info("Starting query performance tracking test")
	
	// Start mixed workloads on both databases
	pgSession := suite.startWorkload("postgresql", "performance_test")
	mysqlSession := suite.startWorkload("mysql", "performance_test")
	defer func() {
		suite.stopWorkload(pgSession.ID)
		suite.stopWorkload(mysqlSession.ID)
	}()
	
	// Start collector with query tracking configuration
	collectorConfig := suite.createQueryTrackingCollectorConfig()
	collector := suite.startCollector(collectorConfig)
	defer collector.Shutdown()
	
	// Execute specific performance test queries
	suite.executePerformanceTestQueries()
	
	// Wait for metrics to be collected
	time.Sleep(45 * time.Second)
	
	// Validate query performance metrics
	suite.validateQueryPerformanceMetrics()
	
	// Test query normalization
	suite.testQueryNormalization()
	
	// Test query fingerprinting
	suite.testQueryFingerprinting()
	
	suite.logger.Info("Query performance tracking test completed")
}

// TestPIISanitizationValidation tests PII sanitization with real sensitive data patterns
func (suite *E2EMetricsFlowTestSuite) TestPIISanitizationValidation() {
	suite.logger.Info("Starting PII sanitization validation test")
	
	// Insert test data with various PII patterns
	suite.insertPIITestData()
	
	// Start workload that will generate queries with PII
	piiSession := suite.startWorkload("postgresql", "pii_test")
	defer suite.stopWorkload(piiSession.ID)
	
	// Start collector with PII detection and sanitization
	collectorConfig := suite.createPIISanitizationCollectorConfig()
	collector := suite.startCollector(collectorConfig)
	defer collector.Shutdown()
	
	// Wait for PII processing
	time.Sleep(30 * time.Second)
	
	// Validate PII detection and sanitization
	suite.validatePIISanitization()
	
	// Test various PII patterns
	suite.testPIIPatterns()
	
	// Validate no PII in exported metrics
	suite.validateNoPIIInExportedData()
	
	suite.logger.Info("PII sanitization validation test completed")
}

// TestAdaptiveSamplingBehavior tests adaptive sampling under different load conditions
func (suite *E2EMetricsFlowTestSuite) TestAdaptiveSamplingBehavior() {
	suite.logger.Info("Starting adaptive sampling behavior test")
	
	// Test low load sampling
	suite.testSamplingUnderLoad("low")
	
	// Test medium load sampling
	suite.testSamplingUnderLoad("medium")
	
	// Test high load sampling
	suite.testSamplingUnderLoad("high")
	
	// Test extreme load sampling
	suite.testSamplingUnderLoad("extreme")
	
	// Validate sampling decisions
	suite.validateSamplingDecisions()
	
	suite.logger.Info("Adaptive sampling behavior test completed")
}

// TestCircuitBreakerActivationRecovery tests circuit breaker activation and recovery
func (suite *E2EMetricsFlowTestSuite) TestCircuitBreakerActivationRecovery() {
	suite.logger.Info("Starting circuit breaker activation and recovery test")
	
	// Test circuit breaker with database connectivity issues
	suite.testCircuitBreakerWithConnectivityIssues()
	
	// Test circuit breaker with high error rates
	suite.testCircuitBreakerWithHighErrorRate()
	
	// Test circuit breaker recovery
	suite.testCircuitBreakerRecovery()
	
	// Validate circuit breaker metrics
	suite.validateCircuitBreakerMetrics()
	
	suite.logger.Info("Circuit breaker activation and recovery test completed")
}

// TestVerificationProcessorHealthChecks tests verification processor health checks and auto-tuning
func (suite *E2EMetricsFlowTestSuite) TestVerificationProcessorHealthChecks() {
	suite.logger.Info("Starting verification processor health checks test")
	
	// Start collector with verification processor
	collectorConfig := suite.createVerificationProcessorConfig()
	collector := suite.startCollector(collectorConfig)
	defer collector.Shutdown()
	
	// Generate mixed workload
	session := suite.startWorkload("mixed", "verification_test")
	defer suite.stopWorkload(session.ID)
	
	// Wait for verification processor to start
	time.Sleep(30 * time.Second)
	
	// Test health check functionality
	suite.testHealthCheckFunctionality()
	
	// Test auto-tuning capabilities
	suite.testAutoTuningCapabilities()
	
	// Test self-healing functionality
	suite.testSelfHealingFunctionality()
	
	// Validate verification processor metrics
	suite.validateVerificationProcessorMetrics()
	
	suite.logger.Info("Verification processor health checks test completed")
}

// TestHighLoadStressTesting tests high load and stress scenarios
func (suite *E2EMetricsFlowTestSuite) TestHighLoadStressTesting() {
	suite.logger.Info("Starting high load and stress testing")
	
	// Execute all defined stress test scenarios
	for _, scenario := range suite.testConfig.StressTestScenarios {
		suite.executeStressTestScenario(scenario)
	}
	
	// Validate system stability under stress
	suite.validateSystemStabilityUnderStress()
	
	// Test resource limits
	suite.testResourceLimits()
	
	// Test recovery after stress
	suite.testRecoveryAfterStress()
	
	suite.logger.Info("High load and stress testing completed")
}

// TestDatabaseFailoverScenarios tests database failover scenarios
func (suite *E2EMetricsFlowTestSuite) TestDatabaseFailoverScenarios() {
	suite.logger.Info("Starting database failover scenarios test")
	
	// Test PostgreSQL failover
	suite.testPostgreSQLFailover()
	
	// Test MySQL failover
	suite.testMySQLFailover()
	
	// Test collector behavior during failover
	suite.testCollectorBehaviorDuringFailover()
	
	// Test recovery after failover
	suite.testRecoveryAfterFailover()
	
	suite.logger.Info("Database failover scenarios test completed")
}

// TestMemoryPressureResourceLimits tests memory pressure and resource limits
func (suite *E2EMetricsFlowTestSuite) TestMemoryPressureResourceLimits() {
	suite.logger.Info("Starting memory pressure and resource limits test")
	
	// Test memory pressure scenarios
	suite.testMemoryPressureScenarios()
	
	// Test resource limit enforcement
	suite.testResourceLimitEnforcement()
	
	// Test memory cleanup mechanisms
	suite.testMemoryCleanupMechanisms()
	
	// Test graceful degradation
	suite.testGracefulDegradation()
	
	suite.logger.Info("Memory pressure and resource limits test completed")
}

// TestNRDBIntegrationValidation tests metric flow to New Relic/NRDB with validation
func (suite *E2EMetricsFlowTestSuite) TestNRDBIntegrationValidation() {
	suite.logger.Info("Starting NRDB integration validation test")
	
	// Start collector with NRDB export configuration
	collectorConfig := suite.createNRDBExportConfig()
	collector := suite.startCollector(collectorConfig)
	defer collector.Shutdown()
	
	// Generate test data
	session := suite.startWorkload("mixed", "nrdb_test")
	defer suite.stopWorkload(session.ID)
	
	// Wait for data export
	time.Sleep(60 * time.Second)
	
	// Validate data in NRDB
	suite.validateNRDBIntegration()
	
	// Test NRDB query performance
	suite.testNRDBQueryPerformance()
	
	// Test data retention and cleanup
	suite.testNRDBDataRetention()
	
	suite.logger.Info("NRDB integration validation test completed")
}

// TestErrorScenariosRecovery tests error scenarios and recovery mechanisms
func (suite *E2EMetricsFlowTestSuite) TestErrorScenariosRecovery() {
	suite.logger.Info("Starting error scenarios and recovery test")
	
	// Test network connectivity errors
	suite.testNetworkConnectivityErrors()
	
	// Test database connection errors
	suite.testDatabaseConnectionErrors()
	
	// Test NRDB export errors
	suite.testNRDBExportErrors()
	
	// Test configuration errors
	suite.testConfigurationErrors()
	
	// Test recovery mechanisms
	suite.testRecoveryMechanisms()
	
	suite.logger.Info("Error scenarios and recovery test completed")
}

// Helper Methods

// loadTestConfig loads test configuration from file or creates default
func (suite *E2EMetricsFlowTestSuite) loadTestConfig() *E2ETestConfig {
	configPath := filepath.Join(suite.testDataDir, "e2e_test_config.json")
	
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default configuration
		return suite.createDefaultTestConfig()
	}
	
	// Load configuration from file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		suite.logger.Warn("Failed to read config file, using defaults", zap.Error(err))
		return suite.createDefaultTestConfig()
	}
	
	var config E2ETestConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		suite.logger.Warn("Failed to parse config file, using defaults", zap.Error(err))
		return suite.createDefaultTestConfig()
	}
	
	return &config
}

// createDefaultTestConfig creates a default test configuration
func (suite *E2EMetricsFlowTestSuite) createDefaultTestConfig() *E2ETestConfig {
	return &E2ETestConfig{
		PostgreSQLConfig: DatabaseConfig{
			Database:       "testdb",
			Username:       "testuser",
			Password:       "testpass",
			ConnectionPool: 10,
			Extensions:     []string{"pg_stat_statements"},
			InitQueries: []string{
				"CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
				"SELECT pg_stat_statements_reset()",
			},
		},
		MySQLConfig: DatabaseConfig{
			Database:       "testdb",
			Username:       "testuser",
			Password:       "testpass",
			ConnectionPool: 10,
			InitQueries: []string{
				"SET GLOBAL performance_schema = ON",
				"SET GLOBAL slow_query_log = ON",
			},
		},
		WorkloadDuration:     300 * time.Second, // 5 minutes
		WorkloadConcurrency:  10,
		WorkloadTypes:        []string{"oltp", "olap", "mixed", "pii_test", "performance_test"},
		LoadTestDuration:     600 * time.Second, // 10 minutes
		MaxConcurrentQueries: 100,
		
		MetricValidationRules: []ValidationRule{
			{
				Name:        "postgresql_query_count",
				MetricName:  "postgresql.query.count",
				Operator:    "gt",
				ExpectedValue: 0,
				Critical:    true,
			},
			{
				Name:        "mysql_query_duration",
				MetricName:  "mysql.query.duration",
				Operator:    "gt",
				ExpectedValue: 0,
				Critical:    true,
			},
		},
		
		NRDBValidationQueries: []NRDBQuery{
			{
				Name:            "metrics_presence",
				Query:           "SELECT count(*) FROM Metric WHERE instrumentation.provider = 'database-intelligence' SINCE 10 minutes ago",
				ExpectedResults: 1,
				Timeout:         30 * time.Second,
				RetryCount:      3,
				Critical:        true,
			},
		},
		
		StressTestScenarios: []StressTestScenario{
			{
				Name:            "high_concurrency",
				Duration:        300 * time.Second,
				ConcurrentUsers: 50,
				QueryPattern:    "mixed",
				ResourceLimits: ResourceLimits{
					MaxMemoryMB:   512,
					MaxCPUPercent: 80,
				},
				ExpectedFailureRate: 0.05,
			},
		},
		
		Thresholds: TestThresholds{
			MaxLatencyMS:         1000,
			MinThroughputRPS:     10.0,
			MaxErrorRate:         0.05,
			MinDataQualityScore:  0.9,
			MaxMemoryUsageMB:     512,
			MaxCPUUsagePercent:   80.0,
			MinEntityCorrelation: 0.8,
			MaxCardinalityScore:  0.7,
		},
	}
}

// startTestContainers starts PostgreSQL and MySQL containers for testing
func (suite *E2EMetricsFlowTestSuite) startTestContainers() {
	ctx := context.Background()
	
	// Start PostgreSQL container
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		postgres.WithInitScripts(filepath.Join("containers", "postgres-init.sql")),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(suite.T(), err)
	suite.pgContainer = pgContainer
	
	// Start MySQL container
	mysqlContainer, err := mysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8.0"),
		mysql.WithDatabase("testdb"),
		mysql.WithUsername("testuser"),
		mysql.WithPassword("testpass"),
		mysql.WithScripts(filepath.Join("containers", "mysql-init.sql")),
		testcontainers.WithWaitStrategy(
			wait.ForLog("port: 3306  MySQL Community Server").
				WithStartupTimeout(120*time.Second)),
	)
	require.NoError(suite.T(), err)
	suite.mysqlContainer = mysqlContainer
	
	suite.logger.Info("Test containers started successfully")
}

// setupTestDatabases connects to databases and sets up test schemas
func (suite *E2EMetricsFlowTestSuite) setupTestDatabases() {
	ctx := context.Background()
	
	// Setup PostgreSQL
	pgConnStr, err := suite.pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(suite.T(), err)
	
	suite.pgDB, err = sql.Open("postgres", pgConnStr)
	require.NoError(suite.T(), err)
	
	// Initialize PostgreSQL database
	suite.initializePostgreSQLDatabase()
	
	// Setup MySQL
	mysqlConnStr, err := suite.mysqlContainer.ConnectionString(ctx)
	require.NoError(suite.T(), err)
	
	suite.mysqlDB, err = sql.Open("mysql", mysqlConnStr)
	require.NoError(suite.T(), err)
	
	// Initialize MySQL database
	suite.initializeMySQLDatabase()
	
	suite.logger.Info("Test databases setup completed")
}

// initializeTestUtilities initializes various test utilities
func (suite *E2EMetricsFlowTestSuite) initializeTestUtilities() {
	// Initialize metric validator
	suite.metricValidator = NewMetricValidator(suite.logger, suite.testConfig)
	
	// Initialize performance benchmark
	suite.performanceBench = NewPerformanceBenchmark(suite.logger, suite.testConfig)
	
	// Initialize report generator
	suite.reportGenerator = NewTestReportGenerator(suite.logger, suite.testDataDir)
	
	// Initialize NRDB validator
	suite.nrdbValidator = NewNRDBValidator(suite.logger, suite.testConfig.NewRelicConfig)
	
	// Initialize stress test manager
	suite.stressTestManager = NewStressTestManager(suite.logger, suite.testConfig)
	
	// Initialize workload generators
	suite.initializeWorkloadGenerators()
	
	suite.logger.Info("Test utilities initialized successfully")
}

// Additional helper methods would be implemented here...
// This file would continue with the implementation of all the test methods,
// helper functions, and utility classes referenced above.

// TestSuite runs all E2E tests
func TestE2EMetricsFlowSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}
	
	// Check for required environment variables
	if os.Getenv("NEW_RELIC_API_KEY") == "" {
		t.Skip("NEW_RELIC_API_KEY not set, skipping E2E tests")
	}
	
	suite.Run(t, new(E2EMetricsFlowTestSuite))
}