package nrerrormonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// nrErrorMonitor tracks potential New Relic integration errors
type nrErrorMonitor struct {
	config       *Config
	logger       *zap.Logger
	nextConsumer consumer.Metrics
	
	// Error tracking
	errorCounts  map[string]*errorTracker
	mutex        sync.RWMutex
	
	// Metrics generation
	lastReport   time.Time
	shutdownCh   chan struct{}
	wg           sync.WaitGroup
}

type errorTracker struct {
	category      string
	count         int64
	lastSeen      time.Time
	lastMessage   string
	alertFired    bool
}

// Start begins the error monitoring processor
func (p *nrErrorMonitor) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting NrIntegrationError monitor processor")
	
	p.shutdownCh = make(chan struct{})
	
	// Start monitoring goroutine
	p.wg.Add(1)
	go p.monitoringLoop()
	
	return nil
}

// Shutdown stops the processor
func (p *nrErrorMonitor) Shutdown(context.Context) error {
	p.logger.Info("Shutting down NrIntegrationError monitor processor")
	
	close(p.shutdownCh)
	p.wg.Wait()
	
	return nil
}

// Capabilities returns the consumer capabilities
func (p *nrErrorMonitor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeMetrics analyzes metrics for potential integration errors
func (p *nrErrorMonitor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Analyze metrics for common patterns that lead to NrIntegrationError
	p.analyzeMetrics(md)
	
	// Pass through to next consumer
	return p.nextConsumer.ConsumeMetrics(ctx, md)
}

// analyzeMetrics checks for patterns that commonly cause NrIntegrationError
func (p *nrErrorMonitor) analyzeMetrics(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		
		// Check resource attributes
		p.checkResourceAttributes(rm.Resource().Attributes())
		
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				p.checkMetric(metric)
			}
		}
	}
}

// checkResourceAttributes validates resource-level attributes
func (p *nrErrorMonitor) checkResourceAttributes(attrs pcommon.Map) {
	// Check for missing critical attributes
	if _, ok := attrs.Get("service.name"); !ok {
		p.recordError("missing_attribute", "Missing required service.name attribute")
	}
	
	// Check for overly long attribute values
	attrs.Range(func(k string, v pcommon.Value) bool {
		if v.Type() == pcommon.ValueTypeStr && len(v.Str()) > p.config.MaxAttributeLength {
			p.recordError("attribute_too_long", 
				fmt.Sprintf("Attribute %s exceeds max length: %d > %d", k, len(v.Str()), p.config.MaxAttributeLength))
		}
		return true
	})
}

// checkMetric validates individual metrics
func (p *nrErrorMonitor) checkMetric(metric pmetric.Metric) {
	// Check metric name length
	if len(metric.Name()) > p.config.MaxMetricNameLength {
		p.recordError("metric_name_too_long",
			fmt.Sprintf("Metric name exceeds max length: %s (%d > %d)", 
				metric.Name(), len(metric.Name()), p.config.MaxMetricNameLength))
	}
	
	// Check cardinality
	uniqueTimeSeries := p.countUniqueTimeSeries(metric)
	if uniqueTimeSeries > p.config.CardinalityWarningThreshold {
		p.recordError("high_cardinality",
			fmt.Sprintf("Metric %s has high cardinality: %d unique time series", 
				metric.Name(), uniqueTimeSeries))
	}
	
	// Check for invalid metric types
	switch metric.Type() {
	case pmetric.MetricTypeEmpty:
		p.recordError("invalid_metric_type",
			fmt.Sprintf("Metric %s has empty type", metric.Name()))
	case pmetric.MetricTypeSum:
		if sum := metric.Sum(); !sum.IsMonotonic() && sum.AggregationTemporality() == pmetric.AggregationTemporalityDelta {
			p.recordError("invalid_sum_metric",
				fmt.Sprintf("Metric %s is non-monotonic delta sum (not supported)", metric.Name()))
		}
	}
}

// countUniqueTimeSeries estimates cardinality for a metric
func (p *nrErrorMonitor) countUniqueTimeSeries(metric pmetric.Metric) int {
	uniqueCombos := make(map[string]struct{})
	
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			key := p.attributesToKey(dp.Attributes())
			uniqueCombos[key] = struct{}{}
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			key := p.attributesToKey(dp.Attributes())
			uniqueCombos[key] = struct{}{}
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			key := p.attributesToKey(dp.Attributes())
			uniqueCombos[key] = struct{}{}
		}
	}
	
	return len(uniqueCombos)
}

// attributesToKey creates a unique key from attributes
func (p *nrErrorMonitor) attributesToKey(attrs pcommon.Map) string {
	// Simple implementation - in production would use a more efficient method
	raw := attrs.AsRaw()
	b, _ := json.Marshal(raw)
	return string(b)
}

// recordError tracks detected errors
func (p *nrErrorMonitor) recordError(category, message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	tracker, exists := p.errorCounts[category]
	if !exists {
		tracker = &errorTracker{
			category: category,
		}
		p.errorCounts[category] = tracker
	}
	
	tracker.count++
	tracker.lastSeen = time.Now()
	tracker.lastMessage = message
	
	// Log if this is a new or recurring error
	if tracker.count == 1 || time.Since(tracker.lastSeen) > p.config.ErrorSuppressionDuration {
		p.logger.Warn("Potential NrIntegrationError detected",
			zap.String("category", category),
			zap.String("message", message),
			zap.Int64("occurrences", tracker.count))
	}
	
	// Check if we need to fire an alert
	if !tracker.alertFired && tracker.count >= p.config.AlertThreshold {
		p.fireAlert(category, message, tracker.count)
		tracker.alertFired = true
	}
}

// fireAlert generates alert metrics for detected errors
func (p *nrErrorMonitor) fireAlert(category, message string, count int64) {
	p.logger.Error("NrIntegrationError threshold exceeded - generating alert",
		zap.String("category", category),
		zap.String("message", message),
		zap.Int64("count", count))
	
	// In a real implementation, this would generate a metric that triggers
	// a New Relic alert policy
}

// monitoringLoop periodically generates summary metrics
func (p *nrErrorMonitor) monitoringLoop() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(p.config.ReportingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.generateSummaryMetrics()
		case <-p.shutdownCh:
			return
		}
	}
}

// generateSummaryMetrics creates metrics summarizing detected errors
func (p *nrErrorMonitor) generateSummaryMetrics() {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	if len(p.errorCounts) == 0 {
		return
	}
	
	// Create metrics
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	
	// Set resource attributes
	rm.Resource().Attributes().PutStr("service.name", "otel-collector")
	rm.Resource().Attributes().PutStr("collector.type", "nr-error-monitor")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/nrerrormonitor")
	
	// Create gauge for each error category
	for category, tracker := range p.errorCounts {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("otelcol.nrerror.potential")
		metric.SetDescription("Potential NrIntegrationError detected by monitor")
		metric.SetUnit("1")
		
		gauge := metric.SetEmptyGauge()
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetIntValue(tracker.count)
		
		// Add attributes
		dp.Attributes().PutStr("error.category", category)
		dp.Attributes().PutStr("error.last_message", tracker.lastMessage)
		dp.Attributes().PutInt("error.minutes_since_last", int64(time.Since(tracker.lastSeen).Minutes()))
	}
	
	// Send metrics through pipeline
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := p.nextConsumer.ConsumeMetrics(ctx, md); err != nil {
		p.logger.Error("Failed to send error monitoring metrics", zap.Error(err))
	}
}

// ProcessorCreateSettings returns the settings for creating the processor
func newNrErrorMonitor(config *Config, logger *zap.Logger, nextConsumer consumer.Metrics) *nrErrorMonitor {
	return &nrErrorMonitor{
		config:       config,
		logger:       logger,
		nextConsumer: nextConsumer,
		errorCounts:  make(map[string]*errorTracker),
		lastReport:   time.Now(),
	}
}