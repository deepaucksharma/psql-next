package adaptivesampler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestNewAdaptiveSampler(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := consumertest.NewTraces()
	
	processor, err := newAdaptiveSampler(logger, consumer, cfg)
	require.NoError(t, err)
	require.NotNil(t, processor)
}

func TestAdaptiveSampler_ProcessTraces(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.InMemoryOnly = true // Use in-memory cache only
	cfg.Rules = []SamplingRule{
		{
			Name:         "test-rule",
			SamplingRate: 0.5,
			Conditions: map[string]string{
				"service.name": "test-service",
			},
		},
	}
	
	logger := zap.NewNop()
	consumer := consumertest.NewTraces()
	processor, err := newAdaptiveSampler(logger, consumer, cfg)
	require.NoError(t, err)
	
	// Start the processor
	err = processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Create test traces
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	
	// Process traces
	err = processor.ConsumeTraces(context.Background(), traces)
	require.NoError(t, err)
	
	// Verify traces were forwarded to consumer
	assert.Equal(t, 1, consumer.SpanCount())
}

func TestAdaptiveSampler_SamplingDecision(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.InMemoryOnly = true
	cfg.DefaultSamplingRate = 0.0 // Default to not sampling
	cfg.Rules = []SamplingRule{
		{
			Name:         "high-priority",
			SamplingRate: 1.0,
			Conditions: map[string]string{
				"priority": "high",
			},
		},
		{
			Name:         "low-priority",
			SamplingRate: 0.0,
			Conditions: map[string]string{
				"priority": "low",
			},
		},
	}
	
	logger := zap.NewNop()
	consumer := consumertest.NewTraces()
	processor, err := newAdaptiveSampler(logger, consumer, cfg)
	require.NoError(t, err)
	
	err = processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// Test high priority trace (should be sampled)
	highPriorityTraces := ptrace.NewTraces()
	rs := highPriorityTraces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("priority", "high")
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetName("high-priority-span")
	span.SetTraceID([16]byte{1})
	span.SetSpanID([8]byte{1})
	
	err = processor.ConsumeTraces(context.Background(), highPriorityTraces)
	require.NoError(t, err)
	
	// Test low priority trace (should not be sampled)
	lowPriorityTraces := ptrace.NewTraces()
	rs2 := lowPriorityTraces.ResourceSpans().AppendEmpty()
	rs2.Resource().Attributes().PutStr("priority", "low")
	ils2 := rs2.ScopeSpans().AppendEmpty()
	span2 := ils2.Spans().AppendEmpty()
	span2.SetName("low-priority-span")
	span2.SetTraceID([16]byte{2})
	span2.SetSpanID([8]byte{2})
	
	err = processor.ConsumeTraces(context.Background(), lowPriorityTraces)
	require.NoError(t, err)
	
	// Verify only high priority trace was forwarded
	assert.Equal(t, 1, consumer.SpanCount())
}