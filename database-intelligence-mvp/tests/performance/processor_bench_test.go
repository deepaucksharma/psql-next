package performance

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/database-intelligence-mvp/processors/adaptivesampler"
	"github.com/database-intelligence-mvp/processors/circuitbreaker"
	"github.com/database-intelligence-mvp/processors/planattributeextractor"
	"github.com/database-intelligence-mvp/processors/verification"
)

// BenchmarkAdaptiveSampler measures the performance of the adaptive sampler
func BenchmarkAdaptiveSampler(b *testing.B) {
	cfg := &adaptivesampler.Config{
		Rules: []adaptivesampler.SamplingRule{
			{
				Name:       "slow_queries",
				Expression: `attributes["db.statement.duration"] > 1000`,
				SampleRate: 1.0,
			},
			{
				Name:       "error_queries",
				Expression: `attributes["db.statement.error"] != nil`,
				SampleRate: 1.0,
			},
		},
		DefaultSamplingRate: 0.1,
		InMemoryOnly:       true,
	}

	processor, err := adaptivesampler.NewAdaptiveSampler(cfg, zap.NewNop())
	require.NoError(b, err)

	// Create test metrics
	metrics := generateTestMetrics(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := processor.ProcessMetrics(ctx, metrics)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(b.N*1000)/b.Elapsed().Seconds(), "metrics/sec")
}

// BenchmarkCircuitBreaker measures the performance of the circuit breaker
func BenchmarkCircuitBreaker(b *testing.B) {
	cfg := &circuitbreaker.Config{
		FailureThreshold:    5,
		SuccessThreshold:    2,
		Timeout:            30 * time.Second,
		HalfOpenMaxRequests: 3,
		BackoffMultiplier:   2.0,
		MaxBackoff:         5 * time.Minute,
	}

	processor, err := circuitbreaker.NewCircuitBreaker(cfg, zap.NewNop())
	require.NoError(b, err)

	metrics := generateTestMetrics(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := processor.ProcessMetrics(ctx, metrics)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(b.N*100)/b.Elapsed().Seconds(), "metrics/sec")
}

// BenchmarkPlanAttributeExtractor measures plan extraction performance
func BenchmarkPlanAttributeExtractor(b *testing.B) {
	cfg := &planattributeextractor.Config{
		SafeMode: true,
		Timeout:  100 * time.Millisecond,
		MaxPlanSize: 10 * 1024,
		AnonymizePlans: true,
		PlanAnonymization: planattributeextractor.PlanAnonymizationConfig{
			Enabled:            true,
			AnonymizeFilters:   true,
			AnonymizeJoinConds: true,
			RemoveCostEstimates: true,
			SensitiveNodeTypes: []string{"Function Scan", "CTE Scan"},
		},
	}

	processor, err := planattributeextractor.NewPlanAttributeExtractor(cfg, zap.NewNop())
	require.NoError(b, err)

	// Create metrics with plan data
	metrics := generateMetricsWithPlans(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := processor.ProcessMetrics(ctx, metrics)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(b.N*100)/b.Elapsed().Seconds(), "metrics/sec")
}

// BenchmarkVerificationProcessor measures verification performance
func BenchmarkVerificationProcessor(b *testing.B) {
	cfg := &verification.Config{
		PIIDetection: verification.PIIDetectionConfig{
			Enabled: true,
			Patterns: map[string]string{
				"credit_card": `\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,
				"ssn":         `\b\d{3}-\d{2}-\d{4}\b`,
				"email":       `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
			},
			ScanQueryText: true,
			ScanPlanJSON:  true,
			ActionOnDetection: "redact",
		},
		DataQuality: verification.DataQualityConfig{
			Enabled: true,
			RequiredAttributes: []string{"db.system", "db.name"},
			MaxAttributeLength: 1000,
			MaxMetricValue:    1e9,
		},
		CardinalityProtection: verification.CardinalityProtectionConfig{
			Enabled: true,
			MaxUniqueQueries: 10000,
			MaxUniquePlans:   5000,
			MaxUniqueUsers:   1000,
			WindowDuration:   5 * time.Minute,
		},
		AutoTuning: verification.AutoTuningConfig{
			Enabled: true,
			TargetFalsePositiveRate: 0.01,
			AdjustmentInterval:      5 * time.Minute,
		},
	}

	processor, err := verification.NewVerificationProcessor(cfg, zap.NewNop())
	require.NoError(b, err)

	metrics := generateTestMetrics(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := processor.ProcessMetrics(ctx, metrics)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(b.N*100)/b.Elapsed().Seconds(), "metrics/sec")
}

// BenchmarkFullPipeline measures the performance of all processors combined
func BenchmarkFullPipeline(b *testing.B) {
	// Create all processors
	sampler, err := adaptivesampler.NewAdaptiveSampler(&adaptivesampler.Config{
		DefaultSamplingRate: 0.1,
		InMemoryOnly:       true,
	}, zap.NewNop())
	require.NoError(b, err)

	breaker, err := circuitbreaker.NewCircuitBreaker(&circuitbreaker.Config{
		FailureThreshold: 5,
		Timeout:         30 * time.Second,
	}, zap.NewNop())
	require.NoError(b, err)

	extractor, err := planattributeextractor.NewPlanAttributeExtractor(&planattributeextractor.Config{
		SafeMode: true,
		Timeout:  100 * time.Millisecond,
	}, zap.NewNop())
	require.NoError(b, err)

	verifier, err := verification.NewVerificationProcessor(&verification.Config{
		PIIDetection: verification.PIIDetectionConfig{
			Enabled: true,
		},
	}, zap.NewNop())
	require.NoError(b, err)

	metrics := generateMetricsWithPlans(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		
		// Process through full pipeline
		m := metrics
		m, err = sampler.ProcessMetrics(ctx, m)
		if err != nil {
			b.Fatal(err)
		}
		
		m, err = breaker.ProcessMetrics(ctx, m)
		if err != nil {
			b.Fatal(err)
		}
		
		m, err = extractor.ProcessMetrics(ctx, m)
		if err != nil {
			b.Fatal(err)
		}
		
		_, err = verifier.ProcessMetrics(ctx, m)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(b.N*1000)/b.Elapsed().Seconds(), "metrics/sec")
}

// Helper function to generate test metrics
func generateTestMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	
	// Set resource attributes
	rm.Resource().Attributes().PutStr("service.name", "postgres-test")
	rm.Resource().Attributes().PutStr("db.system", "postgresql")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	
	for i := 0; i < count; i++ {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(fmt.Sprintf("db.query.duration_%d", i%10))
		
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(float64(i % 1000))
		
		// Add attributes
		dp.Attributes().PutStr("db.statement", fmt.Sprintf("SELECT * FROM table_%d", i%100))
		dp.Attributes().PutInt("db.statement.duration", int64(i%2000))
		dp.Attributes().PutStr("db.user", fmt.Sprintf("user_%d", i%20))
		dp.Attributes().PutStr("db.name", fmt.Sprintf("db_%d", i%5))
		
		// Add some slow queries
		if i%20 == 0 {
			dp.Attributes().PutInt("db.statement.duration", 5000)
		}
		
		// Add some errors
		if i%50 == 0 {
			dp.Attributes().PutStr("db.statement.error", "timeout")
		}
	}
	
	return metrics
}

// Helper function to generate metrics with plan data
func generateMetricsWithPlans(count int) pmetric.Metrics {
	metrics := generateTestMetrics(count)
	
	// Add plan data to some metrics
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	
	for i := 0; i < sm.Metrics().Len(); i++ {
		if i%5 == 0 {
			metric := sm.Metrics().At(i)
			dp := metric.Gauge().DataPoints().At(0)
			
			// Add plan JSON
			planJSON := `{
				"Plan": {
					"Node Type": "Nested Loop",
					"Plans": [
						{
							"Node Type": "Index Scan",
							"Index Name": "users_pkey",
							"Filter": "(email = 'test@example.com')"
						},
						{
							"Node Type": "Seq Scan",
							"Relation Name": "orders",
							"Filter": "(total > 100.00)"
						}
					]
				}
			}`
			dp.Attributes().PutStr("db.plan.json", planJSON)
			dp.Attributes().PutStr("db.plan.text", "Nested Loop -> Index Scan, Seq Scan")
		}
	}
	
	return metrics
}

// BenchmarkMemoryAllocations tracks memory allocations per operation
func BenchmarkMemoryAllocations(b *testing.B) {
	scenarios := []struct {
		name string
		size int
	}{
		{"Small_10", 10},
		{"Medium_100", 100},
		{"Large_1000", 1000},
		{"XLarge_10000", 10000},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			cfg := &adaptivesampler.Config{
				DefaultSamplingRate: 0.1,
				InMemoryOnly:       true,
			}

			processor, err := adaptivesampler.NewAdaptiveSampler(cfg, zap.NewNop())
			require.NoError(b, err)

			metrics := generateTestMetrics(scenario.size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				_, err := processor.ProcessMetrics(ctx, metrics)
				if err != nil {
					b.Fatal(err)
				}
			}

			b.ReportMetric(float64(b.N*scenario.size)/b.Elapsed().Seconds(), "metrics/sec")
		})
	}
}

// BenchmarkHighCardinality tests performance with high cardinality data
func BenchmarkHighCardinality(b *testing.B) {
	cfg := &verification.Config{
		CardinalityProtection: verification.CardinalityProtectionConfig{
			Enabled: true,
			MaxUniqueQueries: 10000,
			MaxUniquePlans:   5000,
			MaxUniqueUsers:   1000,
			WindowDuration:   5 * time.Minute,
		},
	}

	processor, err := verification.NewVerificationProcessor(cfg, zap.NewNop())
	require.NoError(b, err)

	// Generate high cardinality metrics
	metrics := generateHighCardinalityMetrics(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := processor.ProcessMetrics(ctx, metrics)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ReportMetric(float64(b.N*1000)/b.Elapsed().Seconds(), "metrics/sec")
}

// Helper to generate high cardinality metrics
func generateHighCardinalityMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "postgres-test")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	
	for i := 0; i < count; i++ {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("db.query.duration")
		
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(float64(i))
		
		// High cardinality attributes
		dp.Attributes().PutStr("query.id", fmt.Sprintf("q_%d_%d", i, time.Now().UnixNano()))
		dp.Attributes().PutStr("session.id", fmt.Sprintf("s_%d", i))
		dp.Attributes().PutStr("client.ip", fmt.Sprintf("192.168.%d.%d", i%255, (i+1)%255))
		dp.Attributes().PutStr("app.version", fmt.Sprintf("v1.%d.%d", i%100, i%10))
	}
	
	return metrics
}