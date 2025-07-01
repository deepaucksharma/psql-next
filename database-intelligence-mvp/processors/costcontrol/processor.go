package costcontrol

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// costControlProcessor implements intelligent data reduction for cost optimization
type costControlProcessor struct {
	config         *Config
	logger         *zap.Logger
	nextTraces     consumer.Traces
	nextMetrics    consumer.Metrics
	nextLogs       consumer.Logs
	
	// Cost tracking
	costTracker    *costTracker
	mutex          sync.RWMutex
	
	// Cardinality tracking for metrics
	metricCardinality map[string]*cardinalityTracker
	
	// Shutdown
	shutdownCh     chan struct{}
	wg             sync.WaitGroup
}

type costTracker struct {
	currentMonth      time.Time
	bytesIngested     int64
	estimatedCostUSD  float64
	projectedCostUSD  float64
	lastUpdate        time.Time
}

type cardinalityTracker struct {
	metricName        string
	uniqueTimeSeries  map[string]time.Time  // Track unique combinations
	lastCleanup       time.Time
}

// Start begins the cost control processor
func (p *costControlProcessor) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting cost control processor")
	
	p.shutdownCh = make(chan struct{})
	
	// Start cost monitoring goroutine
	p.wg.Add(1)
	go p.costMonitoringLoop()
	
	// Start cardinality cleanup goroutine
	p.wg.Add(1)
	go p.cardinalityCleanupLoop()
	
	return nil
}

// Shutdown stops the processor
func (p *costControlProcessor) Shutdown(context.Context) error {
	p.logger.Info("Shutting down cost control processor")
	
	close(p.shutdownCh)
	p.wg.Wait()
	
	// Log final cost report
	p.logCostReport()
	
	return nil
}

// Capabilities returns the consumer capabilities
func (p *costControlProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// ConsumeTraces applies cost control to traces
func (p *costControlProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Track data volume
	dataSize := p.estimateTraceSize(td)
	p.updateCostTracking(dataSize, "traces")
	
	// Apply intelligent sampling if over budget
	if p.isOverBudget() {
		td = p.applyAggressiveTraceSampling(td)
	}
	
	// Remove high-cost attributes
	p.removeExpensiveTraceAttributes(td)
	
	return p.nextTraces.ConsumeTraces(ctx, td)
}

// ConsumeMetrics applies cost control to metrics
func (p *costControlProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Track data volume
	dataSize := p.estimateMetricSize(md)
	p.updateCostTracking(dataSize, "metrics")
	
	// Apply cardinality reduction
	md = p.reduceMetricCardinality(md)
	
	// Drop low-value metrics if over budget
	if p.isOverBudget() {
		md = p.dropLowValueMetrics(md)
	}
	
	return p.nextMetrics.ConsumeMetrics(ctx, md)
}

// ConsumeLogs applies cost control to logs
func (p *costControlProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Track data volume
	dataSize := p.estimateLogSize(ld)
	p.updateCostTracking(dataSize, "logs")
	
	// Apply aggressive filtering if over budget
	if p.isOverBudget() {
		ld = p.applyAggressiveLogFiltering(ld)
	}
	
	// Truncate large log bodies
	p.truncateLargeLogs(ld)
	
	return p.nextLogs.ConsumeLogs(ctx, ld)
}

// reduceMetricCardinality removes high-cardinality attributes
func (p *costControlProcessor) reduceMetricCardinality(md pmetric.Metrics) pmetric.Metrics {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				p.processMetricCardinality(metric)
			}
		}
	}
	
	return md
}

// processMetricCardinality tracks and reduces cardinality for a metric
func (p *costControlProcessor) processMetricCardinality(metric pmetric.Metric) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	tracker, exists := p.metricCardinality[metric.Name()]
	if !exists {
		tracker = &cardinalityTracker{
			metricName:       metric.Name(),
			uniqueTimeSeries: make(map[string]time.Time),
			lastCleanup:      time.Now(),
		}
		p.metricCardinality[metric.Name()] = tracker
	}
	
	// Count current cardinality
	currentCardinality := p.countMetricCardinality(metric)
	
	// If exceeding threshold, remove high-cardinality attributes
	if currentCardinality > p.config.MetricCardinalityLimit {
		p.logger.Warn("Metric exceeds cardinality limit - removing attributes",
			zap.String("metric", metric.Name()),
			zap.Int("cardinality", currentCardinality),
			zap.Int("limit", p.config.MetricCardinalityLimit))
		
		p.removeHighCardinalityAttributes(metric)
	}
}

// removeHighCardinalityAttributes removes attributes that contribute to high cardinality
func (p *costControlProcessor) removeHighCardinalityAttributes(metric pmetric.Metric) {
	// List of attributes known to cause high cardinality
	highCardAttrs := []string{
		"user.id", "session.id", "request.id", "trace.id", "span.id",
		"http.request.id", "transaction.id", "correlation.id",
		"client.address", "client.socket.address", "net.peer.ip",
		"http.user_agent", "user_agent.original",
	}
	
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := metric.Gauge().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			p.removeAttributesFromDataPoint(dps.At(i).Attributes(), highCardAttrs)
		}
	case pmetric.MetricTypeSum:
		dps := metric.Sum().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			p.removeAttributesFromDataPoint(dps.At(i).Attributes(), highCardAttrs)
		}
	case pmetric.MetricTypeHistogram:
		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			p.removeAttributesFromDataPoint(dps.At(i).Attributes(), highCardAttrs)
		}
	}
}

// removeAttributesFromDataPoint removes specified attributes
func (p *costControlProcessor) removeAttributesFromDataPoint(attrs pcommon.Map, toRemove []string) {
	for _, attr := range toRemove {
		attrs.Remove(attr)
	}
}

// applyAggressiveTraceSampling reduces trace volume when over budget
func (p *costControlProcessor) applyAggressiveTraceSampling(td ptrace.Traces) ptrace.Traces {
	// This is a simplified version - in production would use more sophisticated sampling
	newTd := ptrace.NewTraces()
	
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		
		// Only keep traces with errors or high latency
		if p.shouldKeepResourceSpans(rs) {
			newRs := newTd.ResourceSpans().AppendEmpty()
			rs.CopyTo(newRs)
		}
	}
	
	return newTd
}

// shouldKeepResourceSpans determines if spans should be kept
func (p *costControlProcessor) shouldKeepResourceSpans(rs ptrace.ResourceSpans) bool {
	sss := rs.ScopeSpans()
	for i := 0; i < sss.Len(); i++ {
		ss := sss.At(i)
		spans := ss.Spans()
		
		for j := 0; j < spans.Len(); j++ {
			span := spans.At(j)
			
			// Keep error spans
			if span.Status().Code() == ptrace.StatusCodeError {
				return true
			}
			
			// Keep slow spans
			duration := span.EndTimestamp() - span.StartTimestamp()
			if duration > pcommon.Timestamp(p.config.SlowSpanThresholdMs*1_000_000) {
				return true
			}
		}
	}
	
	return false
}

// applyAggressiveLogFiltering reduces log volume when over budget
func (p *costControlProcessor) applyAggressiveLogFiltering(ld plog.Logs) plog.Logs {
	newLd := plog.NewLogs()
	
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		newRl := newLd.ResourceLogs().AppendEmpty()
		rl.Resource().CopyTo(newRl.Resource())
		
		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			newSl := newRl.ScopeLogs().AppendEmpty()
			sl.Scope().CopyTo(newSl.Scope())
			
			logs := sl.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				
				// Only keep WARN and above when over budget
				if log.SeverityNumber() >= plog.SeverityNumberWarn {
					newLog := newSl.LogRecords().AppendEmpty()
					log.CopyTo(newLog)
				}
			}
		}
	}
	
	return newLd
}

// updateCostTracking updates the cost tracking metrics
func (p *costControlProcessor) updateCostTracking(bytes int64, dataType string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	p.costTracker.bytesIngested += bytes
	
	// Calculate cost based on New Relic pricing
	// $0.35/GB for standard, $0.55/GB for Data Plus
	pricePerGB := p.config.PricePerGB
	costIncrement := float64(bytes) / (1024 * 1024 * 1024) * pricePerGB
	
	p.costTracker.estimatedCostUSD += costIncrement
	p.costTracker.lastUpdate = time.Now()
	
	// Update monthly projection
	daysSoFar := time.Since(p.costTracker.currentMonth).Hours() / 24
	if daysSoFar > 0 {
		dailyRate := p.costTracker.estimatedCostUSD / daysSoFar
		p.costTracker.projectedCostUSD = dailyRate * 30
	}
	
	// Log if exceeding budget
	if p.costTracker.projectedCostUSD > p.config.MonthlyBudgetUSD {
		p.logger.Warn("Projected to exceed monthly budget",
			zap.Float64("current_cost", p.costTracker.estimatedCostUSD),
			zap.Float64("projected_cost", p.costTracker.projectedCostUSD),
			zap.Float64("budget", p.config.MonthlyBudgetUSD),
			zap.String("data_type", dataType))
	}
}

// isOverBudget checks if current usage exceeds budget
func (p *costControlProcessor) isOverBudget() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	return p.costTracker.projectedCostUSD > p.config.MonthlyBudgetUSD
}

// costMonitoringLoop periodically reports cost metrics
func (p *costControlProcessor) costMonitoringLoop() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(p.config.ReportingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.generateCostMetrics()
		case <-p.shutdownCh:
			return
		}
	}
}

// logCostReport logs the current cost status
func (p *costControlProcessor) logCostReport() {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	p.logger.Info("Cost control report",
		zap.Int64("bytes_ingested", p.costTracker.bytesIngested),
		zap.Float64("estimated_cost_usd", p.costTracker.estimatedCostUSD),
		zap.Float64("projected_monthly_cost_usd", p.costTracker.projectedCostUSD),
		zap.Float64("monthly_budget_usd", p.config.MonthlyBudgetUSD),
		zap.Float64("budget_utilization_percent", 
			(p.costTracker.projectedCostUSD/p.config.MonthlyBudgetUSD)*100))
}

// Helper functions for size estimation
func (p *costControlProcessor) estimateTraceSize(td ptrace.Traces) int64 {
	// Rough estimation - in production would be more accurate
	return int64(td.SpanCount() * 1024) // Assume ~1KB per span
}

func (p *costControlProcessor) estimateMetricSize(md pmetric.Metrics) int64 {
	// Rough estimation
	return int64(md.DataPointCount() * 100) // Assume ~100 bytes per data point
}

func (p *costControlProcessor) estimateLogSize(ld plog.Logs) int64 {
	// Rough estimation
	return int64(ld.LogRecordCount() * 500) // Assume ~500 bytes per log
}

// Additional helper methods...
func (p *costControlProcessor) countMetricCardinality(metric pmetric.Metric) int {
	uniqueCombos := make(map[string]struct{})
	
	// Implementation simplified for brevity
	// Would track unique attribute combinations
	
	return len(uniqueCombos)
}

func (p *costControlProcessor) truncateLargeLogs(ld plog.Logs) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		sls := rl.ScopeLogs()
		
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			logs := sl.LogRecords()
			
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				
				// Truncate large log bodies
				if body := log.Body(); body.Type() == pcommon.ValueTypeStr {
					if len(body.Str()) > p.config.MaxLogBodySize {
						truncated := body.Str()[:p.config.MaxLogBodySize] + "... [truncated]"
						body.SetStr(truncated)
					}
				}
			}
		}
	}
}

func (p *costControlProcessor) dropLowValueMetrics(md pmetric.Metrics) pmetric.Metrics {
	// List of metrics to drop when over budget
	lowValueMetrics := map[string]bool{
		"system.cpu.utilization":     false, // Keep
		"system.memory.utilization":  false, // Keep  
		"http.server.duration":       false, // Keep
		"db.client.connections.idle": true,  // Drop
		"runtime.uptime":            true,  // Drop
		"process.cpu.time":          true,  // Drop
	}
	
	newMd := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		newRm := newMd.ResourceMetrics().AppendEmpty()
		rm.Resource().CopyTo(newRm.Resource())
		
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			newSm := newRm.ScopeMetrics().AppendEmpty()
			sm.Scope().CopyTo(newSm.Scope())
			
			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				
				// Check if this is a low-value metric
				if shouldDrop, exists := lowValueMetrics[metric.Name()]; !exists || !shouldDrop {
					newMetric := newSm.Metrics().AppendEmpty()
					metric.CopyTo(newMetric)
				}
			}
		}
	}
	
	return newMd
}

func (p *costControlProcessor) generateCostMetrics() {
	// Implementation would generate actual metrics
	// This is simplified for brevity
	p.logCostReport()
}

func (p *costControlProcessor) cardinalityCleanupLoop() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.cleanupCardinalityTrackers()
		case <-p.shutdownCh:
			return
		}
	}
}

func (p *costControlProcessor) cleanupCardinalityTrackers() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	// Clean up old entries in cardinality trackers
	for metricName, tracker := range p.metricCardinality {
		if time.Since(tracker.lastCleanup) > 24*time.Hour {
			delete(p.metricCardinality, metricName)
		}
	}
}

func (p *costControlProcessor) removeExpensiveTraceAttributes(td ptrace.Traces) {
	// Remove large/expensive attributes from traces
	expensiveAttrs := []string{
		"http.request.body",
		"http.response.body", 
		"messaging.message.payload",
		"db.statement",  // Keep query fingerprint instead
	}
	
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		sss := rs.ScopeSpans()
		
		for j := 0; j < sss.Len(); j++ {
			ss := sss.At(j)
			spans := ss.Spans()
			
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				attrs := span.Attributes()
				
				for _, attr := range expensiveAttrs {
					attrs.Remove(attr)
				}
			}
		}
	}
}