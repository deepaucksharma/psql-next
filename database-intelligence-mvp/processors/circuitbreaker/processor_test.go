package circuitbreaker

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

func TestNewCircuitBreaker(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := consumertest.NewMetrics()
	
	processor, err := newCircuitBreaker(logger, consumer, cfg)
	require.NoError(t, err)
	require.NotNil(t, processor)
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.FailureThreshold = 2
	cfg.SuccessThreshold = 2
	cfg.Timeout = time.Second
	cfg.HalfOpenRequests = 1
	
	logger := zap.NewNop()
	consumer := &failingConsumer{
		failUntil: 2,
		t:         t,
	}
	
	processor, err := newCircuitBreaker(logger, consumer, cfg)
	require.NoError(t, err)
	
	err = processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create test metrics
	metrics := createTestMetrics("test-db")
	
	// First request should succeed (closed state)
	err = processor.ConsumeMetrics(context.Background(), metrics)
	assert.NoError(t, err)
	
	// Second and third requests should fail and trip the circuit
	err = processor.ConsumeMetrics(context.Background(), metrics)
	assert.Error(t, err)
	
	err = processor.ConsumeMetrics(context.Background(), metrics)
	assert.Error(t, err)
	
	// Circuit should now be open, requests should fail immediately
	err = processor.ConsumeMetrics(context.Background(), metrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
	
	// Wait for timeout to transition to half-open
	time.Sleep(cfg.Timeout + 100*time.Millisecond)
	
	// Next request should succeed (half-open state, consumer no longer failing)
	consumer.failUntil = 0
	err = processor.ConsumeMetrics(context.Background(), metrics)
	assert.NoError(t, err)
	
	// One more success should close the circuit
	err = processor.ConsumeMetrics(context.Background(), metrics)
	assert.NoError(t, err)
}

func TestCircuitBreaker_PerDatabaseIsolation(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.FailureThreshold = 1
	
	logger := zap.NewNop()
	consumer := &selectiveFailingConsumer{
		failDB: "failing-db",
		t:      t,
	}
	
	processor, err := newCircuitBreaker(logger, consumer, cfg)
	require.NoError(t, err)
	
	err = processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create metrics for different databases
	failingMetrics := createTestMetrics("failing-db")
	workingMetrics := createTestMetrics("working-db")
	
	// Process failing database metrics - should trip circuit for this DB
	err = processor.ConsumeMetrics(context.Background(), failingMetrics)
	assert.Error(t, err)
	
	err = processor.ConsumeMetrics(context.Background(), failingMetrics)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")
	
	// Working database should still function normally
	err = processor.ConsumeMetrics(context.Background(), workingMetrics)
	assert.NoError(t, err)
}

// Helper types and functions

type failingConsumer struct {
	consumertest.MetricsSink
	failUntil int
	callCount int
	t         *testing.T
}

func (fc *failingConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	fc.callCount++
	if fc.callCount <= fc.failUntil {
		return assert.AnError
	}
	return fc.MetricsSink.ConsumeMetrics(ctx, md)
}

type selectiveFailingConsumer struct {
	consumertest.MetricsSink
	failDB string
	t      *testing.T
}

func (sfc *selectiveFailingConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Check if metrics are from failing database
	if md.ResourceMetrics().Len() > 0 {
		rm := md.ResourceMetrics().At(0)
		if dbName, ok := rm.Resource().Attributes().Get("db.name"); ok && dbName.Str() == sfc.failDB {
			return assert.AnError
		}
	}
	return sfc.MetricsSink.ConsumeMetrics(ctx, md)
}

func createTestMetrics(dbName string) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("db.name", dbName)
	rm.Resource().Attributes().PutStr("db.system", "postgresql")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.connections.active")
	metric.SetEmptyGauge()
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(10)
	dp.SetTimestamp(pmetric.NewTimestampFromTime(time.Now()))
	
	return metrics
}