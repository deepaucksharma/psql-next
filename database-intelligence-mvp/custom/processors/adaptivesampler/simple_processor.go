// Package adaptivesampler provides adaptive sampling based on query performance
// This fills a gap where OTEL's probabilistic sampler can't adapt to query characteristics
package adaptivesampler

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

const (
	// TypeStr is the type string for the adaptive sampler processor
	TypeStr   = "adaptive_sampler"
	stability = component.StabilityLevelBeta
)

// Config for adaptive sampling
type Config struct {
	Rules []SamplingRule `mapstructure:"rules"`
	DefaultSamplingRate float64 `mapstructure:"default_sampling_rate"`
}

// SamplingRule defines when to apply specific sampling rates
type SamplingRule struct {
	Name         string  `mapstructure:"name"`
	Condition    string  `mapstructure:"condition"`
	SamplingRate float64 `mapstructure:"sampling_rate"`
}

// simpleAdaptiveSampler implements basic adaptive sampling
// Minimal implementation - just what OTEL doesn't provide
type simpleAdaptiveSampler struct {
	logger          *zap.Logger
	config          *Config
	nextMetrics     consumer.Metrics
	nextLogs        consumer.Logs
	
	// Simple in-memory state
	samplingDecisions sync.Map // queryID -> samplingRate
}

// NewFactory creates the factory for adaptive sampler
func NewFactory() processor.Factory {
	return processor.NewFactory(
		TypeStr,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
		processor.WithLogs(createLogsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		DefaultSamplingRate: 10.0,
		Rules: []SamplingRule{
			{
				Name:         "slow_queries",
				Condition:    "mean_exec_time > 1000",
				SamplingRate: 100.0,
			},
		},
	}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	config := cfg.(*Config)
	return &simpleAdaptiveSampler{
		logger:      set.Logger,
		config:      config,
		nextMetrics: nextConsumer,
	}, nil
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	config := cfg.(*Config)
	return &simpleAdaptiveSampler{
		logger:   set.Logger,
		config:   config,
		nextLogs: nextConsumer,
	}, nil
}

// Start the processor
func (p *simpleAdaptiveSampler) Start(context.Context, component.Host) error {
	p.logger.Info("Starting adaptive sampler processor")
	return nil
}

// Shutdown the processor
func (p *simpleAdaptiveSampler) Shutdown(context.Context) error {
	return nil
}

// ConsumeMetrics applies adaptive sampling to metrics
func (p *simpleAdaptiveSampler) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Apply sampling rules to each metric
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			
			// Check each metric against rules
			for k := sm.Metrics().Len() - 1; k >= 0; k-- {
				metric := sm.Metrics().At(k)
				
				// Simple sampling decision based on metric attributes
				if !p.shouldSampleMetric(metric) {
					sm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
						return m.Name() == metric.Name()
					})
				}
			}
		}
	}
	
	// Forward to next consumer
	if md.MetricCount() > 0 {
		return p.nextMetrics.ConsumeMetrics(ctx, md)
	}
	return nil
}

// ConsumeLogs applies adaptive sampling to logs
func (p *simpleAdaptiveSampler) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Apply sampling to query logs
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			
			// Check each log against rules
			for k := sl.LogRecords().Len() - 1; k >= 0; k-- {
				record := sl.LogRecords().At(k)
				
				if !p.shouldSampleLog(record) {
					sl.LogRecords().RemoveIf(func(lr plog.LogRecord) bool {
						return lr.Timestamp() == record.Timestamp()
					})
				}
			}
		}
	}
	
	// Forward to next consumer
	if ld.LogRecordCount() > 0 {
		return p.nextLogs.ConsumeLogs(ctx, ld)
	}
	return nil
}

// shouldSampleMetric makes sampling decision for metrics
func (p *simpleAdaptiveSampler) shouldSampleMetric(metric pmetric.Metric) bool {
	// Extract query performance attributes
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if metric.Gauge().DataPoints().Len() > 0 {
			dp := metric.Gauge().DataPoints().At(0)
			return p.evaluateSamplingRules(dp.Attributes())
		}
	case pmetric.MetricTypeSum:
		if metric.Sum().DataPoints().Len() > 0 {
			dp := metric.Sum().DataPoints().At(0)
			return p.evaluateSamplingRules(dp.Attributes())
		}
	}
	
	// Default sampling
	return randomSample(p.config.DefaultSamplingRate)
}

// shouldSampleLog makes sampling decision for logs
func (p *simpleAdaptiveSampler) shouldSampleLog(record plog.LogRecord) bool {
	return p.evaluateSamplingRules(record.Attributes())
}

// evaluateSamplingRules checks attributes against configured rules
func (p *simpleAdaptiveSampler) evaluateSamplingRules(attrs pcommon.Map) bool {
	// Check for query execution time
	if execTime, ok := attrs.Get("mean_exec_time"); ok {
		execTimeMs := execTime.Double()
		
		// Apply rules based on execution time
		for _, rule := range p.config.Rules {
			if p.matchesCondition(rule.Condition, execTimeMs) {
				return randomSample(rule.SamplingRate)
			}
		}
	}
	
	// Check for errors - always sample
	if severity, ok := attrs.Get("severity"); ok {
		if severity.Str() == "ERROR" || severity.Str() == "FATAL" {
			return true
		}
	}
	
	return randomSample(p.config.DefaultSamplingRate)
}

// matchesCondition evaluates simple conditions
func (p *simpleAdaptiveSampler) matchesCondition(condition string, execTimeMs float64) bool {
	// Simple condition parsing - in production use a proper expression evaluator
	switch condition {
	case "mean_exec_time > 1000":
		return execTimeMs > 1000
	case "mean_exec_time > 100":
		return execTimeMs > 100
	case "mean_exec_time <= 100":
		return execTimeMs <= 100
	default:
		return false
	}
}

// randomSample makes a random sampling decision
func randomSample(rate float64) bool {
	// Simple random sampling - in production use deterministic sampling
	return (rand.Float64() * 100) < rate
}