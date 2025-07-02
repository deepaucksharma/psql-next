package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/database-intelligence-mvp/processors/adaptivesampler"
	"github.com/database-intelligence-mvp/processors/circuitbreaker"
	"github.com/database-intelligence-mvp/processors/costcontrol"
	"github.com/database-intelligence-mvp/processors/nrerrormonitor"
	"github.com/database-intelligence-mvp/processors/planattributeextractor"
	"github.com/database-intelligence-mvp/processors/querycorrelator"
	"github.com/database-intelligence-mvp/processors/verification"
)

// TestFullProcessorPipeline tests all processors working together
func TestFullProcessorPipeline(t *testing.T) {
	// Create processor settings
	settings := processor.Settings{
		ID: component.MustNewID("test"),
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}

	// Create final consumer to collect processed data
	sink := &consumertest.MetricsSink{}

	// Create processors in reverse order (last processor first)
	processors := []struct {
		name    string
		factory processor.Factory
		config  func(component.Config)
		create  func(processor.Factory, component.Config) (processor.Metrics, error)
	}{
		{
			name:    "verification",
			factory: verification.NewFactory(),
			config: func(cfg component.Config) {
				c := cfg.(*verification.Config)
				c.PIIDetection.Enabled = true
				c.EnablePeriodicVerification = true
			},
			create: func(f processor.Factory, cfg component.Config) (processor.Metrics, error) {
				return f.CreateMetrics(context.Background(), settings, cfg, sink)
			},
		},
		{
			name:    "nrerrormonitor",
			factory: nrerrormonitor.NewFactory(),
			config: func(cfg component.Config) {
				c := cfg.(*nrerrormonitor.Config)
				c.MaxAttributeLength = 1000
				c.CardinalityWarningThreshold = 100
			},
			create: nil, // Will be set to chain to previous processor
		},
		{
			name:    "costcontrol",
			factory: costcontrol.NewFactory(),
			config: func(cfg component.Config) {
				c := cfg.(*costcontrol.Config)
				c.MonthlyBudgetUSD = 1000.0
				c.MetricCardinalityLimit = 10000
			},
			create: nil,
		},
		{
			name:    "querycorrelator",
			factory: querycorrelator.NewFactory(),
			config: func(cfg component.Config) {
				c := cfg.(*querycorrelator.Config)
				c.EnableTableCorrelation = true
				c.EnableDatabaseCorrelation = true
			},
			create: nil,
		},
		{
			name:    "planattributeextractor",
			factory: planattributeextractor.NewFactory(),
			config: func(cfg component.Config) {
				c := cfg.(*planattributeextractor.Config)
				c.SafeMode = true
				c.ErrorMode = "ignore"
			},
			create: nil,
		},
		{
			name:    "circuitbreaker",
			factory: circuitbreaker.NewFactory(),
			config: func(cfg component.Config) {
				c := cfg.(*circuitbreaker.Config)
				c.FailureThreshold = 5
				c.SuccessThreshold = 2
				c.Timeout = 30 * time.Second
			},
			create: nil,
		},
	}

	// Build the pipeline by chaining processors
	var pipeline consumer.Metrics = sink
	for i := 0; i < len(processors); i++ {
		p := processors[i]
		
		// Create config
		cfg := p.factory.CreateDefaultConfig()
		if p.config != nil {
			p.config(cfg)
		}
		
		// Create processor with next in chain
		nextConsumer := pipeline
		if p.create == nil {
			p.create = func(f processor.Factory, cfg component.Config) (processor.Metrics, error) {
				return f.CreateMetrics(context.Background(), settings, cfg, nextConsumer)
			}
		}
		
		proc, err := p.create(p.factory, cfg)
		require.NoError(t, err, "Failed to create %s processor", p.name)
		
		// Start processor
		err = proc.Start(context.Background(), nil)
		require.NoError(t, err, "Failed to start %s processor", p.name)
		defer proc.Shutdown(context.Background())
		
		// This processor becomes the next consumer for the previous one
		pipeline = proc
	}

	// Test with various metric scenarios
	t.Run("normal_metrics", func(t *testing.T) {
		metrics := generateTestMetrics(10)
		err := pipeline.ConsumeMetrics(context.Background(), metrics)
		require.NoError(t, err)
		
		// Verify metrics made it through
		assert.Greater(t, len(sink.AllMetrics()), 0)
	})

	t.Run("metrics_with_pii", func(t *testing.T) {
		metrics := generateMetricsWithPII()
		err := pipeline.ConsumeMetrics(context.Background(), metrics)
		require.NoError(t, err)
		
		// Verify PII was sanitized
		processedMetrics := sink.AllMetrics()
		for _, m := range processedMetrics {
			checkNoPII(t, m)
		}
	})

	t.Run("high_cardinality_metrics", func(t *testing.T) {
		metrics := generateHighCardinalityMetrics(1000)
		err := pipeline.ConsumeMetrics(context.Background(), metrics)
		require.NoError(t, err)
		
		// Cost control should have reduced cardinality
		processedMetrics := sink.AllMetrics()
		assert.NotEmpty(t, processedMetrics)
	})

	t.Run("metrics_with_plans", func(t *testing.T) {
		metrics := generateMetricsWithPlans(5)
		err := pipeline.ConsumeMetrics(context.Background(), metrics)
		require.NoError(t, err)
		
		// Plan attributes should be extracted
		processedMetrics := sink.AllMetrics()
		foundPlanHash := false
		for _, m := range processedMetrics {
			if hasPlanHashAttribute(m) {
				foundPlanHash = true
				break
			}
		}
		assert.True(t, foundPlanHash, "Should have found plan hash attributes")
	})
}

// TestAdaptiveSamplerWithLogs tests the adaptive sampler with logs
func TestAdaptiveSamplerWithLogs(t *testing.T) {
	settings := processor.Settings{
		ID: component.MustNewID("test"),
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}

	// Create logs sink
	sink := &consumertest.LogsSink{}

	// Create adaptive sampler
	factory := adaptivesampler.NewFactory()
	cfg := factory.CreateDefaultConfig().(*adaptivesampler.Config)
	cfg.DefaultSampleRate = 0.5
	cfg.InMemoryOnly = true
	cfg.Deduplication.Enabled = true

	sampler, err := factory.CreateLogs(context.Background(), settings, cfg, sink)
	require.NoError(t, err)

	err = sampler.Start(context.Background(), nil)
	require.NoError(t, err)
	defer sampler.Shutdown(context.Background())

	// Test sampling
	logs := generateTestLogs(100)
	err = sampler.ConsumeLogs(context.Background(), logs)
	require.NoError(t, err)

	// Should have sampled ~50%
	assert.Greater(t, len(sink.AllLogs()), 30)
	assert.Less(t, len(sink.AllLogs()), 70)

	// Test deduplication
	sink.Reset()
	duplicateLogs := generateDuplicateLogs(10)
	err = sampler.ConsumeLogs(context.Background(), duplicateLogs)
	require.NoError(t, err)

	// Should have deduplicated
	assert.Equal(t, 1, len(sink.AllLogs()))
}

// TestProcessorErrorHandling tests error handling across processors
func TestProcessorErrorHandling(t *testing.T) {
	settings := processor.Settings{
		ID: component.MustNewID("test"),
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}

	// Create a consumer that returns errors
	errorConsumer := &erroringConsumer{
		err: fmt.Errorf("simulated downstream error"),
	}

	// Create circuit breaker
	factory := circuitbreaker.NewFactory()
	cfg := factory.CreateDefaultConfig().(*circuitbreaker.Config)
	cfg.FailureThreshold = 3
	cfg.SuccessThreshold = 2
	cfg.OpenStateTimeout = 5 * time.Second

	breaker, err := factory.CreateMetrics(context.Background(), settings, cfg, errorConsumer)
	require.NoError(t, err)

	err = breaker.Start(context.Background(), nil)
	require.NoError(t, err)
	defer breaker.Shutdown(context.Background())

	// Send metrics until circuit opens
	metrics := generateTestMetrics(1)
	
	// First few should fail but circuit should still be closed
	for i := 0; i < 3; i++ {
		err = breaker.ConsumeMetrics(context.Background(), metrics)
		assert.Error(t, err)
	}

	// Circuit should now be open
	err = breaker.ConsumeMetrics(context.Background(), metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
}

// Helper functions

func generateTestMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-db")
	rm.Resource().Attributes().PutStr("db.system", "postgresql")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	
	for i := 0; i < count; i++ {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("db.query.duration")
		
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetDoubleValue(float64(i * 10))
		
		dp.Attributes().PutStr("db.statement", fmt.Sprintf("SELECT * FROM table_%d", i))
		dp.Attributes().PutStr("db.user", fmt.Sprintf("user_%d", i%5))
		dp.Attributes().PutStr("db.name", "testdb")
	}
	
	return metrics
}

func generateMetricsWithPII() pmetric.Metrics {
	metrics := generateTestMetrics(5)
	
	// Add PII to some metrics
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		dp := metric.Gauge().DataPoints().At(0)
		
		// Add various PII patterns
		switch i % 4 {
		case 0:
			dp.Attributes().PutStr("query.text", "SELECT * FROM users WHERE email='john@example.com'")
		case 1:
			dp.Attributes().PutStr("query.text", "UPDATE accounts SET ssn='123-45-6789'")
		case 2:
			dp.Attributes().PutStr("query.text", "INSERT INTO payments (card) VALUES ('4111-1111-1111-1111')")
		case 3:
			dp.Attributes().PutStr("user.phone", "555-123-4567")
		}
	}
	
	return metrics
}

func generateHighCardinalityMetrics(count int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "high-cardinality-db")
	
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
		dp.Attributes().PutStr("trace.id", fmt.Sprintf("t_%d", i))
		dp.Attributes().PutStr("request.id", fmt.Sprintf("r_%d", i))
	}
	
	return metrics
}

func generateMetricsWithPlans(count int) pmetric.Metrics {
	metrics := generateTestMetrics(count)
	
	// Add plan data
	rm := metrics.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics().At(0)
	
	for i := 0; i < sm.Metrics().Len(); i++ {
		metric := sm.Metrics().At(i)
		dp := metric.Gauge().DataPoints().At(0)
		
		planJSON := fmt.Sprintf(`{
			"Plan": {
				"Node Type": "Nested Loop",
				"Plans": [
					{"Node Type": "Index Scan", "Index Name": "idx_%d"},
					{"Node Type": "Seq Scan", "Relation Name": "table_%d"}
				]
			}
		}`, i, i)
		
		dp.Attributes().PutStr("db.plan.json", planJSON)
	}
	
	return metrics
}

func generateTestLogs(count int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-db")
	
	sl := rl.ScopeLogs().AppendEmpty()
	
	for i := 0; i < count; i++ {
		lr := sl.LogRecords().AppendEmpty()
		lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		lr.SetSeverityNumber(plog.SeverityNumberInfo)
		lr.Body().SetStr(fmt.Sprintf("Query executed: SELECT * FROM table_%d", i))
		lr.Attributes().PutStr("query.id", fmt.Sprintf("q_%d", i))
		lr.Attributes().PutStr("db.statement", fmt.Sprintf("SELECT * FROM table_%d", i))
	}
	
	return logs
}

func generateDuplicateLogs(count int) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-db")
	
	sl := rl.ScopeLogs().AppendEmpty()
	
	// Generate same log multiple times
	for i := 0; i < count; i++ {
		lr := sl.LogRecords().AppendEmpty()
		lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		lr.SetSeverityNumber(plog.SeverityNumberInfo)
		lr.Body().SetStr("Same query executed")
		lr.Attributes().PutStr("query.id", "duplicate-query")
		lr.Attributes().PutStr("db.statement", "SELECT * FROM users")
		lr.Attributes().PutStr("db.query.plan.hash", "same-hash-12345")
	}
	
	return logs
}

func checkNoPII(t *testing.T, metrics pmetric.Metrics) {
	piiPatterns := []string{
		`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // email
		`\b\d{3}-\d{2}-\d{4}\b`,                                // SSN
		`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`,          // credit card
		`\b\d{3}-\d{3}-\d{4}\b`,                                // phone
	}
	
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				
				// Check gauge data points
				if metric.Type() == pmetric.MetricTypeGauge {
					for l := 0; l < metric.Gauge().DataPoints().Len(); l++ {
						dp := metric.Gauge().DataPoints().At(l)
						dp.Attributes().Range(func(key string, value pcommon.Value) bool {
							if value.Type() == pcommon.ValueTypeStr {
								for _, pattern := range piiPatterns {
									assert.NotRegexp(t, pattern, value.Str(), 
										"Found PII in attribute %s", key)
								}
							}
							return true
						})
					}
				}
			}
		}
	}
}

func hasPlanHashAttribute(metrics pmetric.Metrics) bool {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				
				if metric.Type() == pmetric.MetricTypeGauge {
					for l := 0; l < metric.Gauge().DataPoints().Len(); l++ {
						dp := metric.Gauge().DataPoints().At(l)
						if _, ok := dp.Attributes().Get("db.query.plan.hash"); ok {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

// erroringConsumer is a test consumer that always returns errors
type erroringConsumer struct {
	err error
}

func (e *erroringConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return e.err
}

func (e *erroringConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}