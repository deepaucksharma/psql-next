package waitanalysis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

const (
	// Type is the processor type string
	Type = "waitanalysis"
	// The stability level of the processor
	stability = component.StabilityLevelBeta
)

// waitAnalysisProcessor implements wait event analysis and categorization
type waitAnalysisProcessor struct {
	config       *Config
	logger       *zap.Logger
	waitPatterns map[string]*WaitPattern
	alertRules   []WaitAlertRule
	alertHistory *AlertHistory
}

// WaitPattern defines a pattern for categorizing wait events
type WaitPattern struct {
	Name        string
	EventTypes  []string
	Events      []string
	Category    string
	Severity    string
	Description string
}

// WaitAlertRule defines alerting rules based on wait events
type WaitAlertRule struct {
	Name      string
	Condition string
	Threshold float64
	Window    time.Duration
	Action    string
}

// AlertHistory tracks recent alerts to avoid duplicates
type AlertHistory struct {
	alerts map[string]time.Time
}

// NewFactory creates a factory for wait analysis processor
func NewFactory() processor.Factory {
	return processor.NewFactory(
		Type,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}

// createDefaultConfig creates the default configuration
func createDefaultConfig() component.Config {
	return &Config{
		Enabled: true,
		Patterns: []PatternConfig{
			{
				Name:       "lock_waits",
				EventTypes: []string{"Lock"},
				Category:   "Concurrency",
				Severity:   "warning",
			},
			{
				Name:       "io_waits",
				EventTypes: []string{"IO"},
				Events:     []string{"DataFileRead", "DataFileWrite", "WALWrite"},
				Category:   "Storage",
				Severity:   "info",
			},
		},
		AlertRules: []AlertRuleConfig{
			{
				Name:      "excessive_lock_waits",
				Condition: "wait_time > 5s AND event_type = 'Lock'",
				Threshold: 10,
				Window:    "1m",
				Action:    "alert",
			},
		},
	}
}

// createMetricsProcessor creates a metrics processor
func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processorCfg := cfg.(*Config)
	
	if !processorCfg.Enabled {
		return processorhelper.NewMetricsProcessor(
			ctx,
			set,
			cfg,
			nextConsumer,
			func(context.Context, pmetric.Metrics) (pmetric.Metrics, error) {
				return pmetric.Metrics{}, nil
			},
		)
	}
	
	proc := &waitAnalysisProcessor{
		config:       processorCfg,
		logger:       set.Logger,
		waitPatterns: make(map[string]*WaitPattern),
		alertHistory: &AlertHistory{
			alerts: make(map[string]time.Time),
		},
	}
	
	// Initialize patterns
	for _, pattern := range processorCfg.Patterns {
		proc.waitPatterns[pattern.Name] = &WaitPattern{
			Name:        pattern.Name,
			EventTypes:  pattern.EventTypes,
			Events:      pattern.Events,
			Category:    pattern.Category,
			Severity:    pattern.Severity,
			Description: pattern.Description,
		}
	}
	
	// Initialize alert rules
	for _, rule := range processorCfg.AlertRules {
		window, _ := time.ParseDuration(rule.Window)
		proc.alertRules = append(proc.alertRules, WaitAlertRule{
			Name:      rule.Name,
			Condition: rule.Condition,
			Threshold: rule.Threshold,
			Window:    window,
			Action:    rule.Action,
		})
	}
	
	return processorhelper.NewMetricsProcessor(
		ctx,
		set,
		cfg,
		nextConsumer,
		proc.processMetrics,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
	)
}

// processMetrics processes metrics to analyze wait events
func (wap *waitAnalysisProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			
			// Process wait event metrics
			wap.processWaitEventMetrics(metrics)
			
			// Add analysis metrics
			wap.addAnalysisMetrics(metrics)
		}
	}
	
	return md, nil
}

// processWaitEventMetrics analyzes wait event metrics
func (wap *waitAnalysisProcessor) processWaitEventMetrics(metrics pmetric.MetricSlice) {
	waitEventMetrics := make(map[string]*WaitEventSummary)
	
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		
		// Look for ASH wait event metrics
		if strings.HasPrefix(metric.Name(), "postgresql.ash.wait_events") {
			switch metric.Type() {
			case pmetric.MetricTypeGauge:
				gauge := metric.Gauge()
				wap.processWaitEventGauge(gauge, waitEventMetrics)
			}
		}
	}
	
	// Check alert rules
	wap.checkAlertRules(waitEventMetrics)
}

// processWaitEventGauge processes wait event gauge metrics
func (wap *waitAnalysisProcessor) processWaitEventGauge(gauge pmetric.Gauge, summary map[string]*WaitEventSummary) {
	dps := gauge.DataPoints()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		waitEventType, _ := dp.Attributes().Get("wait_event_type")
		waitEvent, _ := dp.Attributes().Get("wait_event")
		
		if waitEventType.Str() == "" || waitEvent.Str() == "" {
			continue
		}
		
		// Categorize wait event
		pattern := wap.identifyPattern(waitEventType.Str(), waitEvent.Str())
		if pattern == nil {
			continue
		}
		
		// Update summary
		key := fmt.Sprintf("%s:%s", waitEventType.Str(), waitEvent.Str())
		if _, exists := summary[key]; !exists {
			summary[key] = &WaitEventSummary{
				EventType: waitEventType.Str(),
				Event:     waitEvent.Str(),
				Category:  pattern.Category,
				Severity:  pattern.Severity,
				Count:     0,
			}
		}
		
		summary[key].Count += int(dp.IntValue())
		
		// Add category attribute to the data point
		dp.Attributes().PutStr("category", pattern.Category)
		dp.Attributes().PutStr("severity", pattern.Severity)
	}
}

// identifyPattern identifies the pattern for a wait event
func (wap *waitAnalysisProcessor) identifyPattern(eventType, event string) *WaitPattern {
	for _, pattern := range wap.waitPatterns {
		// Check event type match
		typeMatch := false
		for _, t := range pattern.EventTypes {
			if t == eventType {
				typeMatch = true
				break
			}
		}
		
		if !typeMatch && len(pattern.EventTypes) > 0 {
			continue
		}
		
		// Check event match
		if len(pattern.Events) > 0 {
			eventMatch := false
			for _, e := range pattern.Events {
				if e == event {
					eventMatch = true
					break
				}
			}
			if !eventMatch {
				continue
			}
		}
		
		return pattern
	}
	
	// Default pattern
	return &WaitPattern{
		Category: "Other",
		Severity: "info",
	}
}

// addAnalysisMetrics adds wait event analysis metrics
func (wap *waitAnalysisProcessor) addAnalysisMetrics(metrics pmetric.MetricSlice) {
	// Add wait event category summary metric
	categoryMetric := metrics.AppendEmpty()
	categoryMetric.SetName("postgresql.ash.wait_category.count")
	categoryMetric.SetDescription("Count of sessions by wait event category")
	categoryMetric.SetUnit("{sessions}")
	
	categoryGauge := categoryMetric.SetEmptyGauge()
	
	// Aggregate by category from existing metrics
	categoryCount := make(map[string]int64)
	
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() != "postgresql.ash.wait_events.count" {
			continue
		}
		
		if metric.Type() == pmetric.MetricTypeGauge {
			gauge := metric.Gauge()
			dps := gauge.DataPoints()
			
			for j := 0; j < dps.Len(); j++ {
				dp := dps.At(j)
				if category, ok := dp.Attributes().Get("category"); ok {
					categoryCount[category.Str()] += dp.IntValue()
				}
			}
		}
	}
	
	// Create data points for categories
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	for category, count := range categoryCount {
		dp := categoryGauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(timestamp)
		dp.SetIntValue(count)
		dp.Attributes().PutStr("category", category)
	}
}

// checkAlertRules checks if any alert rules are triggered
func (wap *waitAnalysisProcessor) checkAlertRules(waitEvents map[string]*WaitEventSummary) {
	now := time.Now()
	
	for _, rule := range wap.alertRules {
		// Simple rule evaluation (in production, use a proper expression evaluator)
		triggered := false
		
		// Count events matching the condition
		matchCount := 0
		for _, summary := range waitEvents {
			if wap.matchesCondition(summary, rule.Condition) {
				matchCount += summary.Count
			}
		}
		
		if float64(matchCount) > rule.Threshold {
			triggered = true
		}
		
		if triggered {
			// Check if we've already alerted recently
			lastAlert, exists := wap.alertHistory.alerts[rule.Name]
			if !exists || now.Sub(lastAlert) > rule.Window {
				wap.logger.Warn("Wait event alert triggered",
					zap.String("rule", rule.Name),
					zap.Int("match_count", matchCount),
					zap.Float64("threshold", rule.Threshold),
				)
				wap.alertHistory.alerts[rule.Name] = now
			}
		}
	}
}

// matchesCondition checks if a wait event matches an alert condition
func (wap *waitAnalysisProcessor) matchesCondition(summary *WaitEventSummary, condition string) bool {
	// Simple implementation - in production, use a proper expression parser
	if strings.Contains(condition, "event_type = 'Lock'") && summary.EventType == "Lock" {
		return true
	}
	if strings.Contains(condition, "event IN ('DataFileRead', 'DataFileWrite')") {
		return summary.Event == "DataFileRead" || summary.Event == "DataFileWrite"
	}
	return false
}

// WaitEventSummary summarizes a wait event
type WaitEventSummary struct {
	EventType string
	Event     string
	Category  string
	Severity  string
	Count     int
}