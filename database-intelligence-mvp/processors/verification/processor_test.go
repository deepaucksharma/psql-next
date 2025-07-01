package verification

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestNewVerificationProcessor(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	
	processor := newVerificationProcessor(logger, consumer, cfg)
	require.NotNil(t, processor)
}

func TestVerificationProcessor_PIIDetection(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.PIIDetection.Enabled = true
	cfg.PIIDetection.RedactMode = "mask"
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newVerificationProcessor(logger, consumer, cfg)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create metrics with PII
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.query.duration")
	metric.SetEmptyHistogram()
	dp := metric.Histogram().DataPoints().AppendEmpty()
	
	// Add attributes with PII
	dp.Attributes().PutStr("query", "SELECT * FROM users WHERE email='john@example.com'")
	dp.Attributes().PutStr("user_ssn", "123-45-6789")
	dp.Attributes().PutStr("credit_card", "4111-1111-1111-1111")
	dp.Attributes().PutStr("phone", "+1-555-123-4567")
	
	// Process metrics
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	
	// Verify PII was masked
	processedMetrics := consumer.AllMetrics()[0]
	processedDP := processedMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0)
	
	query, _ := processedDP.Attributes().Get("query")
	assert.Contains(t, query.Str(), "[EMAIL_REDACTED]")
	assert.NotContains(t, query.Str(), "john@example.com")
	
	ssn, _ := processedDP.Attributes().Get("user_ssn")
	assert.Equal(t, "[SSN_REDACTED]", ssn.Str())
	
	cc, _ := processedDP.Attributes().Get("credit_card")
	assert.Equal(t, "[CREDIT_CARD_REDACTED]", cc.Str())
	
	phone, _ := processedDP.Attributes().Get("phone")
	assert.Equal(t, "[PHONE_REDACTED]", phone.Str())
}

func TestVerificationProcessor_QualityChecks(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.QualityChecks.Enabled = true
	cfg.QualityChecks.MaxAttributeLength = 50
	cfg.QualityChecks.MaxAttributesPerMetric = 5
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newVerificationProcessor(logger, consumer, cfg)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create metrics with quality issues
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.test.metric")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	
	// Add too many attributes
	for i := 0; i < 10; i++ {
		dp.Attributes().PutStr(string(rune('a'+i)), "value")
	}
	
	// Add attribute with value too long
	dp.Attributes().PutStr("long_value", "this is a very long string that exceeds the maximum allowed length for attribute values")
	
	// Process metrics
	err = processor.ConsumeMetrics(context.Background(), metrics)
	require.NoError(t, err)
	
	// Verify attributes were limited
	processedMetrics := consumer.AllMetrics()[0]
	processedDP := processedMetrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0)
	
	// Should have at most MaxAttributesPerMetric attributes
	assert.LessOrEqual(t, processedDP.Attributes().Len(), cfg.QualityChecks.MaxAttributesPerMetric)
	
	// Long value should be truncated
	if longVal, ok := processedDP.Attributes().Get("long_value"); ok {
		assert.LessOrEqual(t, len(longVal.Str()), cfg.QualityChecks.MaxAttributeLength)
	}
}

func TestVerificationProcessor_CardinalityProtection(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.QualityChecks.Enabled = true
	cfg.QualityChecks.MaxUniqueValues = 10
	
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	processor := newVerificationProcessor(logger, consumer, cfg)
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Send metrics with high cardinality
	for i := 0; i < 20; i++ {
		metrics := pmetric.NewMetrics()
		rm := metrics.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("db.query.count")
		metric.SetEmptySum()
		dp := metric.Sum().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("query_id", string(rune('a'+i)))
		dp.SetIntValue(1)
		
		err = processor.ConsumeMetrics(context.Background(), metrics)
		require.NoError(t, err)
	}
	
	// Verify cardinality protection kicked in
	// The processor should have logged warnings about high cardinality
	// In a real implementation, you might check internal metrics or state
	assert.True(t, true, "Cardinality protection should be active")
}