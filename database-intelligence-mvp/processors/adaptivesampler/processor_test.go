package adaptivesampler

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

func TestNewAdaptiveSampler(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := &consumertest.LogsSink{}
	
	processor, err := newAdaptiveSampler(cfg, logger, consumer)
	require.NoError(t, err)
	require.NotNil(t, processor)
}

func TestAdaptiveSampler_ProcessLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.InMemoryOnly = true
	cfg.SamplingRules = []SamplingRule{
		{
			Name:              "test-rule",
			Priority:          1,
			SampleRate:        0.5,
			Conditions: []SamplingCondition{
				{
					Attribute: "service.name",
					Operator:  "eq",
					Value:     "test-service",
				},
			},
			MaxPerMinute: 100,
		},
	}
	
	logger := zap.NewNop()
	consumer := &consumertest.LogsSink{}
	processor, err := newAdaptiveSampler(cfg, logger, consumer)
	require.NoError(t, err)
	
	// Start the processor
	ctx := context.Background()
	err = processor.Start(ctx, nil)
	require.NoError(t, err)
	
	// Create test logs
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Attributes().PutStr("service.name", "test-service")
	lr.Attributes().PutStr("query", "SELECT * FROM users")
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	// Process logs multiple times to test sampling
	sampled := 0
	total := 100
	for i := 0; i < total; i++ {
		err = processor.ConsumeLogs(ctx, logs)
		require.NoError(t, err)
		
		consumedLogs := consumer.AllLogs()
		if len(consumedLogs) > sampled {
			sampled = len(consumedLogs)
		}
	}
	
	// Should sample approximately 50%
	assert.Greater(t, sampled, 30)
	assert.Less(t, sampled, 70)
	
	// Shutdown
	err = processor.Shutdown(ctx)
	require.NoError(t, err)
}

func TestAdaptiveSampler_Deduplication(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.InMemoryOnly = true
	cfg.Deduplication = DeduplicationConfig{
		Enabled:       true,
		WindowSeconds: 60,
		CacheSize:     1000,
	}
	
	logger := zap.NewNop()
	consumer := &consumertest.LogsSink{}
	processor, err := newAdaptiveSampler(cfg, logger, consumer)
	require.NoError(t, err)
	
	ctx := context.Background()
	err = processor.Start(ctx, nil)
	require.NoError(t, err)
	
	// Create identical logs
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Attributes().PutStr("query", "SELECT * FROM users WHERE id = 1")
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	// Process same log multiple times
	for i := 0; i < 5; i++ {
		err = processor.ConsumeLogs(ctx, logs)
		require.NoError(t, err)
	}
	
	// Should only have one log due to deduplication
	assert.Equal(t, 1, len(consumer.AllLogs()))
	
	err = processor.Shutdown(ctx)
	require.NoError(t, err)
}

func TestAdaptiveSampler_RateLimiting(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.InMemoryOnly = true
	cfg.MaxRecordsPerSecond = 10
	
	logger := zap.NewNop()
	consumer := &consumertest.LogsSink{}
	processor, err := newAdaptiveSampler(cfg, logger, consumer)
	require.NoError(t, err)
	
	ctx := context.Background()
	err = processor.Start(ctx, nil)
	require.NoError(t, err)
	
	// Create test logs
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Attributes().PutStr("query", "SELECT * FROM users")
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	// Process many logs quickly
	for i := 0; i < 50; i++ {
		_ = processor.ConsumeLogs(ctx, logs)
	}
	
	// Should be rate limited
	consumed := len(consumer.AllLogs())
	assert.LessOrEqual(t, consumed, 15) // Allow some buffer
	
	err = processor.Shutdown(ctx)
	require.NoError(t, err)
}

func TestAdaptiveSampler_MultipleRules(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.InMemoryOnly = true
	cfg.SamplingRules = []SamplingRule{
		{
			Name:             "high-priority",
			Priority:         10,
			SampleRate:       1.0,
			Conditions: []SamplingCondition{
				{
					Attribute: "severity",
					Operator:  "eq",
					Value:     "ERROR",
				},
			},
		},
		{
			Name:             "low-priority",
			Priority:         1,
			SampleRate:       0.1,
			Conditions: []SamplingCondition{
				{
					Attribute: "severity",
					Operator:  "eq",
					Value:     "INFO",
				},
			},
		},
	}
	
	logger := zap.NewNop()
	consumer := &consumertest.LogsSink{}
	processor, err := newAdaptiveSampler(cfg, logger, consumer)
	require.NoError(t, err)
	
	ctx := context.Background()
	err = processor.Start(ctx, nil)
	require.NoError(t, err)
	
	// Process ERROR logs - should all be sampled
	errorLogs := plog.NewLogs()
	rl := errorLogs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Attributes().PutStr("severity", "ERROR")
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	for i := 0; i < 10; i++ {
		err = processor.ConsumeLogs(ctx, errorLogs)
		require.NoError(t, err)
	}
	
	// Should have all ERROR logs
	assert.Equal(t, 10, len(consumer.AllLogs()))
	
	err = processor.Shutdown(ctx)
	require.NoError(t, err)
}

func TestAdaptiveSampler_InvalidConfiguration(t *testing.T) {
	testCases := []struct {
		name      string
		configure func(*Config)
		wantErr   bool
	}{
		{
			name: "negative sample rate",
			configure: func(cfg *Config) {
				cfg.DefaultSampleRate = -0.5
			},
			wantErr: true,
		},
		{
			name: "sample rate > 1",
			configure: func(cfg *Config) {
				cfg.DefaultSampleRate = 1.5
			},
			wantErr: true,
		},
		{
			name: "invalid rule percentage",
			configure: func(cfg *Config) {
				cfg.SamplingRules = []SamplingRule{
					{
						Name:             "invalid",
						SampleRate: 1.5,
					},
				}
			},
			wantErr: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			tc.configure(cfg)
			
			err := cfg.Validate()
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}