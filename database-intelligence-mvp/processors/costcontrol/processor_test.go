package costcontrol

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestNewCostControlProcessor(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	
	processor := newCostControlProcessor(logger, cfg, consumer)
	require.NotNil(t, processor)
}

func TestCostControlProcessor_UnderBudget(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MonthlyBudgetUSD = 1000.0
	cfg.MetricCardinalityLimit = 100
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newCostControlProcessor(logger, cfg, consumer)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create small metrics that should pass through
	metrics := createTestMetrics(10, 5) // 10 metrics, 5 attributes each
	
	// Process metrics multiple times
	for i := 0; i < 10; i++ {
		err = processor.ConsumeMetrics(context.Background(), metrics)
		require.NoError(t, err)
	}
	
	// All metrics should have been forwarded
	assert.Equal(t, 10, len(consumer.AllMetrics()))
}

func TestCostControlProcessor_OverBudget(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MonthlyBudgetUSD = 0.01 // Very small budget
	cfg.AggressiveMode = true
	cfg.MetricCardinalityLimit = 10
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newCostControlProcessor(logger, cfg, consumer)
	
	// Manually set the processor to over-budget state
	processor.costTracker.projectedCostUSD = 1.0
	processor.costTracker.totalBytesProcessed = 3 * 1024 * 1024 * 1024 // 3GB
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create metrics with high cardinality
	metrics := createTestMetrics(100, 20) // 100 metrics, 20 attributes each
	
	// Process metrics
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	
	// Should have filtered out most metrics
	processedMetrics := consumer.AllMetrics()
	require.Equal(t, 1, len(processedMetrics))
	
	// Check that cardinality was reduced
	totalMetrics := 0
	for _, m := range processedMetrics {
		for i := 0; i < m.ResourceMetrics().Len(); i++ {
			rm := m.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				totalMetrics += sm.Metrics().Len()
			}
		}
	}
	assert.Less(t, totalMetrics, 100) // Should have reduced metrics
}

func TestCostControlProcessor_CardinalityReduction(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MetricCardinalityLimit = 5
	cfg.HighCardinalityDimensions = []string{"query_id", "user_id", "session_id"}
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newCostControlProcessor(logger, cfg, consumer)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create metrics with high cardinality attributes
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	for i := 0; i < 10; i++ {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("db.query.duration")
		metric.SetEmptyHistogram()
		dp := metric.Histogram().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("query_id", string(rune('a'+i)))
		dp.Attributes().PutStr("user_id", string(rune('A'+i)))
		dp.Attributes().PutStr("db.name", "testdb")
		dp.SetCount(1)
		dp.SetSum(float64(i))
	}
	
	// Process metrics
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	
	// Verify high cardinality dimensions were removed
	processedMetrics := consumer.AllMetrics()[0]
	metric := processedMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	dp := metric.Histogram().DataPoints().At(0)
	
	_, exists := dp.Attributes().Get("query_id")
	assert.False(t, exists, "High cardinality dimension should be removed")
	
	_, exists = dp.Attributes().Get("user_id")
	assert.False(t, exists, "High cardinality dimension should be removed")
	
	_, exists = dp.Attributes().Get("db.name")
	assert.True(t, exists, "Low cardinality dimension should be kept")
}

// Helper functions

func createTestMetrics(numMetrics, numAttributes int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	
	for i := 0; i < numMetrics; i++ {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("test.metric." + string(rune('a'+i)))
		metric.SetEmptyGauge()
		dp := metric.Gauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.SetTimestamp(pmetric.NewTimestampFromTime(time.Now()))
		
		for j := 0; j < numAttributes; j++ {
			dp.Attributes().PutStr("attr"+string(rune('A'+j)), "value"+string(rune('0'+j)))
		}
	}
	
	return metrics
}