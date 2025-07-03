package processor_tests

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type AdaptiveSamplerE2ETestSuite struct {
	suite.Suite
	collector    *TestCollector
	metricsGen   *MetricsGenerator
	validator    *MetricsValidator
	testEnv      *TestEnvironment
}

func TestAdaptiveSamplerE2E(t *testing.T) {
	suite.Run(t, new(AdaptiveSamplerE2ETestSuite))
}

func (s *AdaptiveSamplerE2ETestSuite) SetupSuite() {
	s.testEnv = NewTestEnvironment()
	s.collector = NewTestCollector(s.testEnv)
	s.metricsGen = NewMetricsGenerator()
	s.validator = NewMetricsValidator()
	
	// Start collector with adaptive sampler configuration
	config := `
processors:
  adaptivesampler:
    rules:
      - name: high_priority_errors
        priority: 1
        sample_rate: 1.0  # Keep all errors
        filter: 'attributes["error"] == true'
      - name: slow_queries
        priority: 2
        sample_rate: 0.5
        filter: 'attributes["duration_ms"] > 1000'
      - name: normal_traffic
        priority: 3
        sample_rate: 0.1
        filter: 'true'
    deduplication:
      enabled: true
      window: 60s
      cache_size: 100000
    rate_limiting:
      enabled: true
      max_rate: 1000
      burst: 100
`
	require.NoError(s.T(), s.collector.Start(config))
}

func (s *AdaptiveSamplerE2ETestSuite) TearDownSuite() {
	s.collector.Stop()
	s.testEnv.Cleanup()
}

// Test: High-Volume Deduplication
func (s *AdaptiveSamplerE2ETestSuite) TestHighVolumeDeduplication() {
	ctx := context.Background()
	
	// Generate 100K identical queries with slight timestamp variations
	baseMetric := s.metricsGen.GenerateMetric("db.query.duration", 150.5, map[string]string{
		"db.name":      "production",
		"db.statement": "SELECT * FROM users WHERE id = ?",
		"db.operation": "SELECT",
	})
	
	var sentCount int32
	var wg sync.WaitGroup
	
	// Send metrics in parallel to simulate high volume
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < 10000; j++ {
				metric := s.cloneMetricWithTimestamp(baseMetric, time.Now().Add(time.Duration(j)*time.Millisecond))
				err := s.collector.SendMetrics(ctx, metric)
				if err == nil {
					atomic.AddInt32(&sentCount, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	time.Sleep(5 * time.Second) // Allow processing
	
	// Verify deduplication worked
	processedMetrics := s.collector.GetProcessedMetrics()
	
	// Should have significantly fewer metrics due to deduplication
	assert.Less(s.T(), len(processedMetrics), int(sentCount)/100, 
		"Deduplication should reduce metrics by at least 99%")
	
	// Verify first occurrence was kept
	assert.Greater(s.T(), len(processedMetrics), 0, "At least one metric should be processed")
	
	// Check memory usage stayed within limits
	memStats := s.collector.GetMemoryStats()
	assert.Less(s.T(), memStats.CacheSize, 100000, "Cache size should not exceed limit")
}

// Test: Complex Rule Evaluation with CEL
func (s *AdaptiveSamplerE2ETestSuite) TestComplexRuleEvaluation() {
	ctx := context.Background()
	
	// Test nested CEL expressions
	complexRules := []struct {
		name       string
		expression string
		metric     pmetric.Metrics
		shouldPass bool
	}{
		{
			name:       "nested_and_or",
			expression: `(attributes["db.name"] == "prod" && attributes["duration_ms"] > 1000) || attributes["alert.level"] == "critical"`,
			metric: s.metricsGen.GenerateMetric("db.query.count", 1, map[string]string{
				"db.name":      "prod",
				"duration_ms":  "1500",
				"alert.level":  "warning",
			}),
			shouldPass: true, // First condition matches
		},
		{
			name:       "complex_numeric_comparison",
			expression: `double(attributes["cpu_usage"]) > 0.8 && double(attributes["memory_usage"]) > 0.7`,
			metric: s.metricsGen.GenerateMetric("system.resource", 1, map[string]string{
				"cpu_usage":    "0.85",
				"memory_usage": "0.75",
			}),
			shouldPass: true,
		},
		{
			name:       "string_operations",
			expression: `attributes["query_type"].startsWith("SELECT") && attributes["table_name"].contains("users")`,
			metric: s.metricsGen.GenerateMetric("db.query", 1, map[string]string{
				"query_type":  "SELECT COUNT(*)",
				"table_name":  "active_users",
			}),
			shouldPass: true,
		},
		{
			name:       "regex_matching",
			expression: `attributes["error_message"].matches(".*timeout.*") || attributes["error_code"].matches("^5[0-9]{2}$")`,
			metric: s.metricsGen.GenerateMetric("db.error", 1, map[string]string{
				"error_message": "connection timeout after 30s",
				"error_code":    "408",
			}),
			shouldPass: true, // First condition matches
		},
	}
	
	for _, tc := range complexRules {
		s.Run(tc.name, func() {
			// Update sampler with test rule
			err := s.collector.UpdateSamplerRule(tc.name, tc.expression, 1.0)
			require.NoError(s.T(), err)
			
			// Send metric
			err = s.collector.SendMetrics(ctx, tc.metric)
			require.NoError(s.T(), err)
			
			time.Sleep(1 * time.Second)
			
			// Verify processing
			processed := s.collector.GetProcessedMetricsWithFilter(tc.name)
			if tc.shouldPass {
				assert.NotEmpty(s.T(), processed, "Metric should be sampled")
			} else {
				assert.Empty(s.T(), processed, "Metric should be filtered out")
			}
		})
	}
}

// Test: Rate Limiting Under Variable Load
func (s *AdaptiveSamplerE2ETestSuite) TestRateLimitingUnderLoad() {
	ctx := context.Background()
	
	loadPatterns := []struct {
		name        string
		pattern     func(t time.Time) int // QPS as function of time
		duration    time.Duration
		maxRate     int
		tolerance   float64
	}{
		{
			name: "burst_scenario",
			pattern: func(t time.Time) int {
				// 10K QPS burst for 1 second, then 100 QPS
				if t.Second() < 1 {
					return 10000
				}
				return 100
			},
			duration:  10 * time.Second,
			maxRate:   1000,
			tolerance: 0.05, // 5% tolerance
		},
		{
			name: "gradual_ramp",
			pattern: func(t time.Time) int {
				// Ramp from 100 to 5000 QPS over 30 seconds
				seconds := t.Unix() % 30
				return 100 + int(float64(seconds)/30.0*4900)
			},
			duration:  30 * time.Second,
			maxRate:   1000,
			tolerance: 0.05,
		},
		{
			name: "oscillating_load",
			pattern: func(t time.Time) int {
				// Sine wave pattern between 100 and 3000 QPS
				seconds := float64(t.Unix() % 60)
				return int(1550 + 1450*math.Sin(seconds*math.Pi/30))
			},
			duration:  60 * time.Second,
			maxRate:   1000,
			tolerance: 0.05,
		},
	}
	
	for _, pattern := range loadPatterns {
		s.Run(pattern.name, func() {
			// Configure rate limiter
			err := s.collector.SetRateLimit(pattern.maxRate)
			require.NoError(s.T(), err)
			
			// Reset counters
			s.collector.ResetCounters()
			
			// Generate load according to pattern
			start := time.Now()
			ticker := time.NewTicker(100 * time.Millisecond) // 10 Hz sampling
			defer ticker.Stop()
			
			var totalSent, totalAccepted int64
			done := time.After(pattern.duration)
			
			for {
				select {
				case t := <-ticker.C:
					targetQPS := pattern.pattern(t)
					metricsToSend := targetQPS / 10 // Since we tick at 10 Hz
					
					// Send metrics
					for i := 0; i < metricsToSend; i++ {
						metric := s.metricsGen.GenerateRandomMetric()
						err := s.collector.SendMetrics(ctx, metric)
						if err == nil {
							atomic.AddInt64(&totalSent, 1)
						}
					}
					
				case <-done:
					// Verify rate limiting accuracy
					elapsed := time.Since(start).Seconds()
					actualRate := float64(s.collector.GetAcceptedCount()) / elapsed
					expectedRate := float64(pattern.maxRate)
					
					// Check rate is within tolerance
					deviation := math.Abs(actualRate-expectedRate) / expectedRate
					assert.Less(s.T(), deviation, pattern.tolerance,
						"Rate limiting deviation should be within %.1f%% (actual: %.2f, expected: %.2f)",
						pattern.tolerance*100, actualRate, expectedRate)
					
					return
				}
			}
		})
	}
}

// Test: Cryptographic Sampling Distribution
func (s *AdaptiveSamplerE2ETestSuite) TestSamplingDistribution() {
	ctx := context.Background()
	
	testCases := []struct {
		sampleRate    float64
		totalMetrics  int
		expectedCount int
		tolerance     float64
	}{
		{0.01, 100000, 1000, 0.1},   // 1% sampling
		{0.1, 100000, 10000, 0.05},  // 10% sampling
		{0.5, 100000, 50000, 0.02},  // 50% sampling
		{0.999, 10000, 9990, 0.01},  // 99.9% sampling
	}
	
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("rate_%.3f", tc.sampleRate), func() {
			// Configure sampler
			err := s.collector.UpdateSamplerRule("distribution_test", "true", tc.sampleRate)
			require.NoError(s.T(), err)
			
			// Reset counters
			s.collector.ResetCounters()
			
			// Generate metrics with unique IDs for deterministic sampling
			sampled := 0
			for i := 0; i < tc.totalMetrics; i++ {
				metric := s.metricsGen.GenerateMetric("test.metric", float64(i), map[string]string{
					"unique_id": fmt.Sprintf("id_%d", i),
				})
				
				err := s.collector.SendMetrics(ctx, metric)
				require.NoError(s.T(), err)
			}
			
			time.Sleep(5 * time.Second) // Allow processing
			
			// Get sampled count
			sampledCount := len(s.collector.GetProcessedMetrics())
			
			// Verify distribution is within tolerance
			deviation := math.Abs(float64(sampledCount-tc.expectedCount)) / float64(tc.expectedCount)
			assert.Less(s.T(), deviation, tc.tolerance,
				"Sampling distribution should be within %.1f%% of expected (got %d, expected %d)",
				tc.tolerance*100, sampledCount, tc.expectedCount)
			
			// Chi-square test for uniformity
			if tc.sampleRate > 0.1 && tc.sampleRate < 0.9 {
				chiSquare := s.calculateChiSquare(sampledCount, tc.expectedCount, tc.totalMetrics)
				// Critical value for chi-square with 1 degree of freedom at 0.05 significance
				assert.Less(s.T(), chiSquare, 3.841, "Distribution should be uniform (chi-square test)")
			}
		})
	}
}

// Test: Priority-Based Rule Ordering
func (s *AdaptiveSamplerE2ETestSuite) TestPriorityBasedRuleOrdering() {
	ctx := context.Background()
	
	// Configure multiple overlapping rules with different priorities
	rules := []struct {
		name       string
		priority   int
		sampleRate float64
		filter     string
	}{
		{"catch_all", 100, 0.01, "true"},                          // Lowest priority
		{"slow_queries", 50, 0.5, `attributes["duration_ms"] > 1000`},
		{"errors", 10, 1.0, `attributes["error"] == true`},
		{"critical_errors", 1, 1.0, `attributes["error"] == true && attributes["severity"] == "critical"`}, // Highest priority
	}
	
	// Configure rules
	for _, rule := range rules {
		err := s.collector.AddSamplerRule(rule.name, rule.priority, rule.filter, rule.sampleRate)
		require.NoError(s.T(), err)
	}
	
	// Test metrics that match multiple rules
	testMetrics := []struct {
		name         string
		attributes   map[string]string
		expectedRule string
	}{
		{
			name: "critical_error_slow",
			attributes: map[string]string{
				"error":       "true",
				"severity":    "critical",
				"duration_ms": "2000",
			},
			expectedRule: "critical_errors", // Should match highest priority
		},
		{
			name: "normal_error_slow",
			attributes: map[string]string{
				"error":       "true",
				"severity":    "warning",
				"duration_ms": "2000",
			},
			expectedRule: "errors", // Should match errors rule
		},
		{
			name: "slow_query_only",
			attributes: map[string]string{
				"duration_ms": "1500",
			},
			expectedRule: "slow_queries",
		},
		{
			name:         "normal_query",
			attributes:   map[string]string{"duration_ms": "50"},
			expectedRule: "catch_all",
		},
	}
	
	for _, tm := range testMetrics {
		s.Run(tm.name, func() {
			metric := s.metricsGen.GenerateMetric("test.metric", 1.0, tm.attributes)
			
			// Send metric multiple times to verify consistent rule application
			for i := 0; i < 10; i++ {
				err := s.collector.SendMetrics(ctx, metric)
				require.NoError(s.T(), err)
			}
			
			time.Sleep(1 * time.Second)
			
			// Verify correct rule was applied
			appliedRule := s.collector.GetAppliedRule(metric)
			assert.Equal(s.T(), tm.expectedRule, appliedRule,
				"Metric should be processed by the highest priority matching rule")
		})
	}
}

// Test: Memory Efficiency and LRU Cache Behavior
func (s *AdaptiveSamplerE2ETestSuite) TestMemoryEfficiencyAndLRU() {
	ctx := context.Background()
	
	// Configure small cache to test LRU eviction
	err := s.collector.UpdateSamplerConfig(map[string]interface{}{
		"deduplication": map[string]interface{}{
			"enabled":    true,
			"window":     "60s",
			"cache_size": 1000, // Small cache
		},
	})
	require.NoError(s.T(), err)
	
	// Phase 1: Fill cache with 1000 unique metrics
	phase1Metrics := make([]pmetric.Metrics, 1000)
	for i := 0; i < 1000; i++ {
		metric := s.metricsGen.GenerateMetric("test.metric", float64(i), map[string]string{
			"unique_id": fmt.Sprintf("phase1_%d", i),
		})
		phase1Metrics[i] = metric
		err := s.collector.SendMetrics(ctx, metric)
		require.NoError(s.T(), err)
	}
	
	time.Sleep(2 * time.Second)
	
	// Phase 2: Send 500 new metrics (should evict oldest 500)
	for i := 0; i < 500; i++ {
		metric := s.metricsGen.GenerateMetric("test.metric", float64(i), map[string]string{
			"unique_id": fmt.Sprintf("phase2_%d", i),
		})
		err := s.collector.SendMetrics(ctx, metric)
		require.NoError(s.T(), err)
	}
	
	// Phase 3: Resend first 500 metrics from phase 1
	for i := 0; i < 500; i++ {
		err := s.collector.SendMetrics(ctx, phase1Metrics[i])
		require.NoError(s.T(), err)
	}
	
	time.Sleep(2 * time.Second)
	
	// Verify behavior
	stats := s.collector.GetDeduplicationStats()
	
	// First 500 from phase1 should have been evicted and re-added
	assert.Greater(s.T(), stats.Evictions, int64(400), "Should have evicted entries")
	assert.Equal(s.T(), int64(1000), stats.CacheSize, "Cache should be at capacity")
	
	// Memory usage should be bounded
	memStats := s.collector.GetMemoryStats()
	assert.Less(s.T(), memStats.HeapAlloc, uint64(10*1024*1024), "Memory usage should be under 10MB")
}

// Helper methods

func (s *AdaptiveSamplerE2ETestSuite) cloneMetricWithTimestamp(original pmetric.Metrics, ts time.Time) pmetric.Metrics {
	cloned := pmetric.NewMetrics()
	original.CopyTo(cloned)
	
	// Update timestamp
	rms := cloned.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			metrics := sms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					dps := metric.Gauge().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dps.At(l).SetTimestamp(pcommon.NewTimestampFromTime(ts))
					}
				}
			}
		}
	}
	
	return cloned
}

func (s *AdaptiveSamplerE2ETestSuite) calculateChiSquare(observed, expected, total int) float64 {
	notObserved := total - observed
	notExpected := total - expected
	
	chi2 := math.Pow(float64(observed-expected), 2) / float64(expected)
	chi2 += math.Pow(float64(notObserved-notExpected), 2) / float64(notExpected)
	
	return chi2
}