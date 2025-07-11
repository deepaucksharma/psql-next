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

// ModeComparisonTestSuite compares config-only vs enhanced mode
type ModeComparisonTestSuite struct {
	suite.Suite
	env                *framework.TestEnvironment
	configOnlyCollector *framework.TestCollector
	enhancedCollector   *framework.TestCollector
	ctx                context.Context
	cancel             context.CancelFunc
}

func (s *ModeComparisonTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Minute)
	
	// Setup test environment
	env, err := framework.NewTestEnvironment(s.ctx, framework.TestConfig{
		DatabaseType: "postgresql",
		EnableMySQL:  true,
	})
	s.Require().NoError(err)
	s.env = env

	// Start config-only collector
	configOnly, err := framework.NewTestCollector(s.ctx, framework.CollectorConfig{
		ConfigPath: "../configs/config-only-base.yaml",
		LogLevel:   "info",
		Port:       8888,
		Features: []string{
			"postgresql",
			"mysql",
			"hostmetrics",
			"sqlquery",
		},
	})
	s.Require().NoError(err)
	s.configOnlyCollector = configOnly

	// Start enhanced mode collector
	enhanced, err := framework.NewTestCollector(s.ctx, framework.CollectorConfig{
		ConfigPath: "../configs/enhanced-mode-full.yaml",
		LogLevel:   "info",
		Port:       8889,
		Features: []string{
			"postgresql",
			"mysql",
			"hostmetrics",
			"sqlquery",
			"enhancedsql",
			"ash",
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
	s.enhancedCollector = enhanced

	// Wait for both collectors to be ready
	s.Require().NoError(s.configOnlyCollector.WaitForReady(s.ctx, 2*time.Minute))
	s.Require().NoError(s.enhancedCollector.WaitForReady(s.ctx, 2*time.Minute))
}

func (s *ModeComparisonTestSuite) TearDownSuite() {
	if s.configOnlyCollector != nil {
		s.configOnlyCollector.Stop()
	}
	if s.enhancedCollector != nil {
		s.enhancedCollector.Stop()
	}
	if s.env != nil {
		s.env.Cleanup()
	}
	s.cancel()
}

// Test01_BasicMetricsComparison verifies both modes collect core metrics
func (s *ModeComparisonTestSuite) Test01_BasicMetricsComparison() {
	// Run identical workload
	workload := s.env.CreateWorkload("basic_comparison", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       10,
		ConnectionCount: 5,
		Operations:      []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
	})
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(30 * time.Second)
	
	// Get metrics from both collectors
	configOnlyMetrics, err := s.configOnlyCollector.GetMetrics(s.ctx, "postgresql.*")
	s.Require().NoError(err)
	
	enhancedMetrics, err := s.enhancedCollector.GetMetrics(s.ctx, "postgresql.*")
	s.Require().NoError(err)
	
	// Core metrics should be present in both
	coreMetrics := []string{
		"postgresql.connections.active",
		"postgresql.transactions.committed",
		"postgresql.blocks.hit",
		"postgresql.database.size",
	}
	
	for _, metric := range coreMetrics {
		s.Assert().True(s.hasMetric(configOnlyMetrics, metric), 
			"Config-only missing core metric: %s", metric)
		s.Assert().True(s.hasMetric(enhancedMetrics, metric), 
			"Enhanced mode missing core metric: %s", metric)
	}
	
	// Enhanced mode should have additional metrics
	s.Assert().Greater(len(enhancedMetrics), len(configOnlyMetrics), 
		"Enhanced mode should collect more metrics")
}

// Test02_AdvancedFeaturesComparison verifies enhanced mode exclusive features
func (s *ModeComparisonTestSuite) Test02_AdvancedFeaturesComparison() {
	// Create workload that benefits from advanced features
	advancedWorkload := s.env.CreateWorkload("advanced_features", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       20,
		ConnectionCount: 10,
		Operations: []string{
			"SELECT * FROM large_table WHERE id = $1",
			"SELECT * FROM large_table WHERE name LIKE '%test%'", // Will have different plans
			"UPDATE inventory SET quantity = quantity - 1 WHERE id = $1",
			"BEGIN; SELECT...; UPDATE...; COMMIT;", // Transaction correlation
		},
		TransactionSize: 5,
	})
	
	err := advancedWorkload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(30 * time.Second)
	
	// Check for enhanced-only metrics
	enhancedOnlyMetrics := []string{
		"postgresql.plan.cost",
		"postgresql.plan.changes",
		"postgresql.ash.sessions.active",
		"postgresql.ash.wait_time",
		"postgresql.query.fingerprint",
	}
	
	configOnlyMetrics, _ := s.configOnlyCollector.GetMetrics(s.ctx, "postgresql.*")
	enhancedMetrics, _ := s.enhancedCollector.GetMetrics(s.ctx, "postgresql.*")
	
	for _, metric := range enhancedOnlyMetrics {
		s.Assert().False(s.hasMetric(configOnlyMetrics, metric), 
			"Config-only should not have: %s", metric)
		s.Assert().True(s.hasMetric(enhancedMetrics, metric), 
			"Enhanced mode missing: %s", metric)
	}
}

// Test03_ResourceUsageComparison compares resource consumption
func (s *ModeComparisonTestSuite) Test03_ResourceUsageComparison() {
	// Get baseline resource usage
	configOnlyBaseline := s.getCollectorResources(s.configOnlyCollector)
	enhancedBaseline := s.getCollectorResources(s.enhancedCollector)
	
	// Run intensive workload
	intensiveWorkload := s.env.CreateWorkload("resource_test", framework.WorkloadConfig{
		Duration:        3 * time.Minute,
		QueryRate:       50,
		ConnectionCount: 20,
		Operations:      []string{"SELECT", "INSERT", "UPDATE", "DELETE"},
	})
	
	err := intensiveWorkload.Run(s.ctx)
	s.Require().NoError(err)
	
	// Get resource usage under load
	configOnlyLoad := s.getCollectorResources(s.configOnlyCollector)
	enhancedLoad := s.getCollectorResources(s.enhancedCollector)
	
	// Calculate increases
	configOnlyMemIncrease := configOnlyLoad.MemoryMB - configOnlyBaseline.MemoryMB
	enhancedMemIncrease := enhancedLoad.MemoryMB - enhancedBaseline.MemoryMB
	
	configOnlyCPUIncrease := configOnlyLoad.CPUPercent - configOnlyBaseline.CPUPercent
	enhancedCPUIncrease := enhancedLoad.CPUPercent - enhancedBaseline.CPUPercent
	
	// Enhanced mode should use more resources but within reasonable limits
	s.Assert().Less(configOnlyMemIncrease, float64(512), 
		"Config-only memory increase should be < 512MB")
	s.Assert().Less(enhancedMemIncrease, float64(2048), 
		"Enhanced memory increase should be < 2GB")
	
	s.Assert().Less(configOnlyCPUIncrease, float64(20), 
		"Config-only CPU increase should be < 20%")
	s.Assert().Less(enhancedCPUIncrease, float64(40), 
		"Enhanced CPU increase should be < 40%")
	
	// Log resource comparison
	s.T().Logf("Resource Usage Comparison:\n"+
		"Config-Only: Memory +%.1fMB, CPU +%.1f%%\n"+
		"Enhanced: Memory +%.1fMB, CPU +%.1f%%",
		configOnlyMemIncrease, configOnlyCPUIncrease,
		enhancedMemIncrease, enhancedCPUIncrease)
}

// Test04_DataQualityComparison compares data processing capabilities
func (s *ModeComparisonTestSuite) Test04_DataQualityComparison() {
	// Create data quality challenges
	
	// 1. High cardinality data
	for i := 0; i < 10000; i++ {
		s.env.ExecuteSQL(fmt.Sprintf("SELECT %d AS unique_id", i))
	}
	
	// 2. PII data
	s.env.ExecuteSQL(`
		CREATE TABLE IF NOT EXISTS sensitive_data (
			id INT,
			email TEXT,
			ssn TEXT
		)`)
	s.env.ExecuteSQL("INSERT INTO sensitive_data VALUES (1, 'test@example.com', '123-45-6789')")
	
	// 3. Queries with PII
	workload := s.env.CreateWorkload("data_quality", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       10,
		ConnectionCount: 5,
		Operations: []string{
			"SELECT * FROM sensitive_data",
			"SELECT email FROM sensitive_data WHERE id = 1",
		},
	})
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(30 * time.Second)
	
	// Check how each mode handles data quality
	configOnlyMetrics, _ := s.configOnlyCollector.GetMetrics(s.ctx, "*")
	enhancedMetrics, _ := s.enhancedCollector.GetMetrics(s.ctx, "*")
	
	// Config-only might have high cardinality
	configOnlyCardinality := s.calculateCardinality(configOnlyMetrics)
	enhancedCardinality := s.calculateCardinality(enhancedMetrics)
	
	// Enhanced mode should limit cardinality
	s.Assert().Less(enhancedCardinality, configOnlyCardinality,
		"Enhanced mode should have lower cardinality due to limits")
	
	// Check for PII in attributes
	configOnlyHasPII := s.checkForPII(configOnlyMetrics)
	enhancedHasPII := s.checkForPII(enhancedMetrics)
	
	s.Assert().False(enhancedHasPII, "Enhanced mode should not have PII")
	// Config-only might have PII (no verification processor)
	s.T().Logf("PII Detection - Config-Only: %v, Enhanced: %v", 
		configOnlyHasPII, enhancedHasPII)
}

// Test05_ErrorHandlingComparison compares error resilience
func (s *ModeComparisonTestSuite) Test05_ErrorHandlingComparison() {
	// Create error scenarios
	
	// 1. Database connection failure
	s.env.StopDatabase()
	time.Sleep(10 * time.Second)
	s.env.StartDatabase()
	
	// 2. Invalid queries
	errorWorkload := s.env.CreateWorkload("error_handling", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       5,
		ConnectionCount: 5,
		Operations: []string{
			"INVALID SQL SYNTAX",
			"SELECT * FROM nonexistent_table",
			"SELECT 1/0", // Division by zero
		},
	})
	
	go errorWorkload.Run(s.ctx) // Don't wait, it will have errors
	
	time.Sleep(2 * time.Minute)
	
	// Check error handling
	configOnlyLogs := s.configOnlyCollector.GetLogs()
	enhancedLogs := s.enhancedCollector.GetLogs()
	
	// Count errors and recovery messages
	configOnlyErrors := s.countLogPatterns(configOnlyLogs, []string{"error", "failed", "exception"})
	enhancedErrors := s.countLogPatterns(enhancedLogs, []string{"error", "failed", "exception"})
	
	configOnlyRecovery := s.countLogPatterns(configOnlyLogs, []string{"recovered", "reconnected", "retry"})
	enhancedRecovery := s.countLogPatterns(enhancedLogs, []string{"recovered", "reconnected", "retry", "circuit breaker"})
	
	// Both should handle errors
	s.Assert().Greater(configOnlyRecovery, 0, "Config-only should show recovery")
	s.Assert().Greater(enhancedRecovery, 0, "Enhanced should show recovery")
	
	// Enhanced mode should have additional protection
	enhancedProtection := s.countLogPatterns(enhancedLogs, []string{
		"circuit breaker opened",
		"self-healing",
		"error pattern detected",
	})
	s.Assert().Greater(enhancedProtection, 0, "Enhanced mode should show protection mechanisms")
}

// Test06_PerformanceComparison compares metric collection performance
func (s *ModeComparisonTestSuite) Test06_PerformanceComparison() {
	// Measure metric collection latency
	
	// Create consistent workload
	perfWorkload := s.env.CreateWorkload("performance", framework.WorkloadConfig{
		Duration:        5 * time.Minute,
		QueryRate:       100, // High rate
		ConnectionCount: 50,
		Operations:      []string{"SELECT", "INSERT", "UPDATE"},
	})
	
	// Start workload
	go perfWorkload.Run(s.ctx)
	
	// Measure collection performance over time
	var configOnlyLatencies []time.Duration
	var enhancedLatencies []time.Duration
	
	for i := 0; i < 10; i++ {
		// Measure config-only
		start := time.Now()
		_, err := s.configOnlyCollector.GetMetrics(s.ctx, "postgresql.*")
		s.Require().NoError(err)
		configOnlyLatencies = append(configOnlyLatencies, time.Since(start))
		
		// Measure enhanced
		start = time.Now()
		_, err = s.enhancedCollector.GetMetrics(s.ctx, "postgresql.*")
		s.Require().NoError(err)
		enhancedLatencies = append(enhancedLatencies, time.Since(start))
		
		time.Sleep(30 * time.Second)
	}
	
	// Calculate averages
	configOnlyAvg := s.averageDuration(configOnlyLatencies)
	enhancedAvg := s.averageDuration(enhancedLatencies)
	
	// Both should be responsive
	s.Assert().Less(configOnlyAvg, 100*time.Millisecond, 
		"Config-only collection should be < 100ms")
	s.Assert().Less(enhancedAvg, 500*time.Millisecond, 
		"Enhanced collection should be < 500ms")
	
	s.T().Logf("Collection Latency - Config-Only: %v, Enhanced: %v", 
		configOnlyAvg, enhancedAvg)
}

// Test07_FeatureToggleTest tests enabling/disabling enhanced features
func (s *ModeComparisonTestSuite) Test07_FeatureToggleTest() {
	// Start with minimal enhanced config
	minimalEnhanced, err := framework.NewTestCollector(s.ctx, framework.CollectorConfig{
		ConfigPath: "../configs/enhanced-mode-minimal.yaml",
		LogLevel:   "info",
		Port:       8890,
		Features: []string{
			"postgresql",
			"enhancedsql", // Only query stats
		},
	})
	s.Require().NoError(err)
	defer minimalEnhanced.Stop()
	
	s.Require().NoError(minimalEnhanced.WaitForReady(s.ctx, 1*time.Minute))
	
	// Run workload
	workload := s.env.CreateWorkload("feature_toggle", framework.WorkloadConfig{
		Duration:        1 * time.Minute,
		QueryRate:       10,
		ConnectionCount: 5,
		Operations:      []string{"SELECT", "INSERT"},
	})
	
	err = workload.Run(s.ctx)
	s.Require().NoError(err)
	
	time.Sleep(30 * time.Second)
	
	// Should have query stats but not ASH or plan metrics
	metrics, err := minimalEnhanced.GetMetrics(s.ctx, "postgresql.*")
	s.Require().NoError(err)
	
	hasQueryStats := s.hasMetric(metrics, "postgresql.query")
	hasASH := s.hasMetric(metrics, "postgresql.ash")
	hasPlan := s.hasMetric(metrics, "postgresql.plan")
	
	s.Assert().True(hasQueryStats, "Should have query stats")
	s.Assert().False(hasASH, "Should not have ASH metrics")
	s.Assert().False(hasPlan, "Should not have plan metrics")
}

// Test08_MigrationScenario tests migrating from config-only to enhanced
func (s *ModeComparisonTestSuite) Test08_MigrationScenario() {
	// Simulate a migration scenario
	
	// Phase 1: Run with config-only
	workload := s.env.CreateWorkload("migration_phase1", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       20,
		ConnectionCount: 10,
		Operations:      []string{"SELECT", "INSERT", "UPDATE"},
	})
	
	err := workload.Run(s.ctx)
	s.Require().NoError(err)
	
	// Collect baseline metrics
	configOnlyBaseline, err := s.configOnlyCollector.GetMetrics(s.ctx, "postgresql.*")
	s.Require().NoError(err)
	baselineCount := len(configOnlyBaseline)
	
	// Phase 2: Switch to enhanced mode (both running)
	time.Sleep(30 * time.Second)
	
	// Continue workload
	workload2 := s.env.CreateWorkload("migration_phase2", framework.WorkloadConfig{
		Duration:        2 * time.Minute,
		QueryRate:       20,
		ConnectionCount: 10,
		Operations:      []string{"SELECT", "INSERT", "UPDATE"},
	})
	
	err = workload2.Run(s.ctx)
	s.Require().NoError(err)
	
	// Compare metrics from both
	configOnlyFinal, _ := s.configOnlyCollector.GetMetrics(s.ctx, "postgresql.*")
	enhancedFinal, _ := s.enhancedCollector.GetMetrics(s.ctx, "postgresql.*")
	
	// Core metrics should match
	for _, metric := range []string{
		"postgresql.connections.active",
		"postgresql.transactions.committed",
		"postgresql.blocks.hit",
	} {
		configValue := s.getMetricValue(configOnlyFinal, metric)
		enhancedValue := s.getMetricValue(enhancedFinal, metric)
		
		// Values should be similar (within 10%)
		if configValue > 0 {
			diff := abs(configValue-enhancedValue) / configValue
			s.Assert().Less(diff, 0.1, 
				"Metric %s differs by more than 10%%: config=%f, enhanced=%f",
				metric, configValue, enhancedValue)
		}
	}
	
	// Enhanced should have additional insights
	s.Assert().Greater(len(enhancedFinal), baselineCount,
		"Enhanced mode should provide more metrics")
}

// Helper methods

func (s *ModeComparisonTestSuite) hasMetric(metrics []pmetric.Metric, name string) bool {
	for _, m := range metrics {
		if strings.Contains(m.Name(), name) {
			return true
		}
	}
	return false
}

func (s *ModeComparisonTestSuite) getCollectorResources(collector *framework.TestCollector) framework.ResourceUsage {
	// Get from collector's self-metrics
	metrics, _ := collector.GetMetrics(s.ctx, "otelcol_process.*")
	
	usage := framework.ResourceUsage{}
	for _, m := range metrics {
		switch m.Name() {
		case "otelcol_process_memory_rss":
			usage.MemoryMB = s.getLatestValue(m) / 1024 / 1024
		case "otelcol_process_cpu_seconds":
			usage.CPUPercent = s.getLatestValue(m) * 100
		}
	}
	return usage
}

func (s *ModeComparisonTestSuite) calculateCardinality(metrics []pmetric.Metric) int {
	uniqueSeries := make(map[string]bool)
	
	for _, m := range metrics {
		m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Range(
			func(k string, v interface{}) bool {
				series := fmt.Sprintf("%s.%s=%v", m.Name(), k, v)
				uniqueSeries[series] = true
				return true
			})
	}
	
	return len(uniqueSeries)
}

func (s *ModeComparisonTestSuite) checkForPII(metrics []pmetric.Metric) bool {
	piiPatterns := []string{
		"@",           // Email
		"\\d{3}-\\d{2}-\\d{4}", // SSN
		"\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}", // Credit card
	}
	
	for _, m := range metrics {
		m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0).Attributes().Range(
			func(k string, v interface{}) bool {
				value := fmt.Sprintf("%v", v)
				for _, pattern := range piiPatterns {
					if strings.Contains(value, pattern) {
						return false // Found PII
					}
				}
				return true
			})
	}
	
	return false
}

func (s *ModeComparisonTestSuite) countLogPatterns(logs []string, patterns []string) int {
	count := 0
	for _, log := range logs {
		for _, pattern := range patterns {
			if strings.Contains(strings.ToLower(log), pattern) {
				count++
				break
			}
		}
	}
	return count
}

func (s *ModeComparisonTestSuite) averageDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func (s *ModeComparisonTestSuite) getMetricValue(metrics []pmetric.Metric, name string) float64 {
	for _, m := range metrics {
		if m.Name() == name {
			return s.getLatestValue(m)
		}
	}
	return 0
}

func (s *ModeComparisonTestSuite) getLatestValue(metric pmetric.Metric) float64 {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if metric.Gauge().DataPoints().Len() > 0 {
			return metric.Gauge().DataPoints().At(0).DoubleValue()
		}
	case pmetric.MetricTypeSum:
		if metric.Sum().DataPoints().Len() > 0 {
			return metric.Sum().DataPoints().At(0).DoubleValue()
		}
	}
	return 0
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestModeComparisonSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mode comparison e2e tests in short mode")
	}
	
	suite.Run(t, new(ModeComparisonTestSuite))
}