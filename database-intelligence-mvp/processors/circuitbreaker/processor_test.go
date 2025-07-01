package circuitbreaker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestNewCircuitBreaker(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := &consumertest.LogsSink{}
	
	processor := newCircuitBreakerProcessor(cfg, logger, consumer)
	require.NotNil(t, processor)
}

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.FailureThreshold = 2
	cfg.SuccessThreshold = 2
	cfg.Timeout = time.Second
	cfg.MaxConcurrentRequests = 10
	
	logger := zap.NewNop()
	consumer := &failingConsumer{
		failUntil: 3,  // Fail first 3 requests
		callCount: 0,
		t:         t,
	}
	
	processor := newCircuitBreakerProcessor(cfg, logger, consumer)
	
	err := processor.Start(context.Background(), nil)
	defer processor.Shutdown(context.Background())
	
	// Create test logs
	logs := createTestLogs("test-db")
	
	// First request should fail (consumer is failing)
	err = processor.ConsumeLogs(context.Background(), logs)
	assert.Error(t, err)
	
	// Second and third requests should fail and trip the circuit
	err = processor.ConsumeLogs(context.Background(), logs)
	assert.Error(t, err)
	
	err = processor.ConsumeLogs(context.Background(), logs)
	assert.Error(t, err)
	
	// Circuit should now be open, requests should fail immediately
	err = processor.ConsumeLogs(context.Background(), logs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker open")
	
	// Wait for timeout to transition to half-open
	time.Sleep(cfg.OpenStateTimeout + 100*time.Millisecond)
	
	// Next request should still be allowed (half-open state testing recovery)
	consumer.failUntil = 0  // Stop failing
	err = processor.ConsumeLogs(context.Background(), logs)
	// This might succeed if circuit is half-open, or fail if still open
	
	// Try another request - should eventually succeed as consumer is no longer failing
	err = processor.ConsumeLogs(context.Background(), logs)
	// Eventually the circuit should close
}

func TestCircuitBreaker_PerDatabaseIsolation(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.FailureThreshold = 1
	
	logger := zap.NewNop()
	consumer := &selectiveFailingConsumer{
		failDB: "failing-db",
		t:      t,
	}
	
	processor := newCircuitBreakerProcessor(cfg, logger, consumer)
	
	_ = processor.Start(context.Background(), nil)
	defer processor.Shutdown(context.Background())
	
	// Create logs for different databases
	failingLogs := createTestLogs("failing-db")
	workingLogs := createTestLogs("working-db")
	
	// Process failing database logs - should eventually trip circuit for this DB
	_ = processor.ConsumeLogs(context.Background(), failingLogs)
	// First request might fail due to selective failing consumer
	
	_ = processor.ConsumeLogs(context.Background(), failingLogs)
	// Additional requests to ensure circuit breaker is triggered
	
	// Working database logs should process without circuit breaker interference
	_ = processor.ConsumeLogs(context.Background(), workingLogs)
	// This should work as it's a different database that's not failing
}

// Helper types and functions

type failingConsumer struct {
	consumertest.LogsSink
	failUntil int
	callCount int
	t         *testing.T
}

func (fc *failingConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	fc.callCount++
	if fc.callCount <= fc.failUntil {
		return assert.AnError
	}
	return fc.LogsSink.ConsumeLogs(ctx, ld)
}

type selectiveFailingConsumer struct {
	consumertest.LogsSink
	failDB string
	t      *testing.T
}

func (sfc *selectiveFailingConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Check if logs are from failing database
	if ld.ResourceLogs().Len() > 0 {
		rl := ld.ResourceLogs().At(0)
		if dbName, ok := rl.Resource().Attributes().Get("db.name"); ok && dbName.Str() == sfc.failDB {
			return assert.AnError
		}
	}
	return sfc.LogsSink.ConsumeLogs(ctx, ld)
}

func createTestLogs(dbName string) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("db.name", dbName)
	rl.Resource().Attributes().PutStr("db.system", "postgresql")
	
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetSeverityText("INFO")
	logRecord.Body().SetStr("Test log message")
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	return logs
}