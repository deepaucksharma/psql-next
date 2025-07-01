package nrerrormonitor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestNewNRErrorMonitor(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	
	processor := newNRErrorMonitor(logger, cfg, consumer)
	require.NotNil(t, processor)
}

func TestNRErrorMonitor_ValidMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newNRErrorMonitor(logger, cfg, consumer)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create valid metrics
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	
	// Add required semantic conventions
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	rm.Resource().Attributes().PutStr("service.version", "1.0.0")
	rm.Resource().Attributes().PutStr("host.id", "test-host")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.connections.active")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(10)
	dp.Attributes().PutStr("db.system", "postgresql")
	dp.Attributes().PutStr("db.name", "testdb")
	
	// Process metrics
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	
	// Metrics should pass through
	assert.Equal(t, 1, len(consumer.AllMetrics()))
}

func TestNRErrorMonitor_MissingSemanticConventions(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.AlertThreshold = 1 // Alert on first error
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newNRErrorMonitor(logger, cfg, consumer)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create metrics missing required attributes
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	// Missing service.name and other required attributes
	
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.connections.active")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(10)
	// Missing db.system attribute
	
	// Process metrics
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	
	// Metrics should still be forwarded
	assert.Equal(t, 1, len(consumer.AllMetrics()))
	
	// Error count should have increased
	assert.Greater(t, processor.errorCount, uint64(0))
}

func TestNRErrorMonitor_LongAttributeNames(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaxAttributeLength = 50
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newNRErrorMonitor(logger, cfg, consumer)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create metrics with long attribute names
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(10)
	
	// Add attribute with name too long
	longAttrName := "this_is_a_very_long_attribute_name_that_exceeds_the_maximum_allowed_length_for_new_relic"
	dp.Attributes().PutStr(longAttrName, "value")
	
	// Process metrics
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	
	// Should have detected the issue
	assert.Greater(t, processor.errorCount, uint64(0))
}

func TestNRErrorMonitor_HighCardinality(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.CardinalityWarningThreshold = 10
	cfg.CardinalityErrorThreshold = 20
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newNRErrorMonitor(logger, cfg, consumer)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create metrics with high cardinality
	for i := 0; i < 25; i++ {
		metrics := pmetric.NewMetrics()
		rm := metrics.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("high.cardinality.metric")
		metric.SetEmptyGauge()
		dp := metric.Gauge().DataPoints().AppendEmpty()
		dp.SetIntValue(int64(i))
		dp.Attributes().PutStr("unique_id", string(rune('a'+i)))
		
		err = processor.ConsumeMetrics(context.Background(), metrics)
		require.NoError(t, err)
	}
	
	// Should have detected high cardinality
	assert.Greater(t, processor.errorCount, uint64(0))
}

func TestNRErrorMonitor_MetricNameValidation(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaxMetricNameLength = 100
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newNRErrorMonitor(logger, cfg, consumer)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create metrics with invalid names
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	// Metric name too long
	longMetric := sm.Metrics().AppendEmpty()
	longMetric.SetName("this.is.a.very.long.metric.name.that.exceeds.the.maximum.allowed.length.for.new.relic.metrics.and.should.trigger.an.error")
	longMetric.SetEmptyGauge()
	longMetric.Gauge().DataPoints().AppendEmpty().SetIntValue(1)
	
	// Metric name with invalid characters
	invalidMetric := sm.Metrics().AppendEmpty()
	invalidMetric.SetName("metric@with#invalid$chars")
	invalidMetric.SetEmptyGauge()
	invalidMetric.Gauge().DataPoints().AppendEmpty().SetIntValue(1)
	
	// Process metrics
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	
	// Should have detected the issues
	assert.Greater(t, processor.errorCount, uint64(0))
}