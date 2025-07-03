package nri

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// nriMetricsExporter exports metrics in NRI format
type nriMetricsExporter struct {
	config   *Config
	logger   *zap.Logger
	writer   *nriWriter
	mu       sync.Mutex
	
	// Compiled patterns for efficiency
	metricPatterns map[string]*regexp.Regexp
}

// nriLogsExporter exports logs as NRI events
type nriLogsExporter struct {
	config   *Config
	logger   *zap.Logger
	writer   *nriWriter
	mu       sync.Mutex
	
	// Compiled patterns
	eventPatterns map[string]*regexp.Regexp
}

// NRI data structures
type nriPayload struct {
	Name               string       `json:"name"`
	IntegrationVersion string       `json:"integration_version"`
	ProtocolVersion    string       `json:"protocol_version"`
	Data               []nriEntity  `json:"data"`
}

type nriEntity struct {
	Entity     nriEntityInfo        `json:"entity"`
	Metrics    []nriMetric          `json:"metrics,omitempty"`
	Events     []nriEvent           `json:"events,omitempty"`
	Inventory  map[string]inventory `json:"inventory,omitempty"`
}

type nriEntityInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	DisplayName string            `json:"displayName,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type nriMetric struct {
	EventType  string                 `json:"event_type"`
	Timestamp  int64                  `json:"timestamp"`
	Attributes map[string]interface{} `json:"attributes"`
}

type nriEvent struct {
	EventType  string                 `json:"eventType"`
	Timestamp  int64                  `json:"timestamp"`
	Category   string                 `json:"category,omitempty"`
	Summary    string                 `json:"summary"`
	Attributes map[string]interface{} `json:"attributes"`
}

type inventory map[string]interface{}

// newMetricsExporter creates a new NRI metrics exporter
func newMetricsExporter(config *Config, settings exporter.Settings) (*nriMetricsExporter, error) {
	writer, err := newNRIWriter(config, settings.Logger)
	if err != nil {
		return nil, err
	}
	
	exp := &nriMetricsExporter{
		config:         config,
		logger:         settings.Logger,
		writer:         writer,
		metricPatterns: make(map[string]*regexp.Regexp),
	}
	
	// Compile metric patterns
	for _, rule := range config.MetricRules {
		pattern := strings.ReplaceAll(rule.SourcePattern, "*", ".*")
		re, err := regexp.Compile("^" + pattern + "$")
		if err != nil {
			return nil, fmt.Errorf("invalid metric pattern %s: %w", rule.SourcePattern, err)
		}
		exp.metricPatterns[rule.SourcePattern] = re
	}
	
	return exp, nil
}

// newLogsExporter creates a new NRI logs exporter
func newLogsExporter(config *Config, settings exporter.Settings) (*nriLogsExporter, error) {
	writer, err := newNRIWriter(config, settings.Logger)
	if err != nil {
		return nil, err
	}
	
	exp := &nriLogsExporter{
		config:        config,
		logger:        settings.Logger,
		writer:        writer,
		eventPatterns: make(map[string]*regexp.Regexp),
	}
	
	// Compile event patterns
	for _, rule := range config.EventRules {
		pattern := strings.ReplaceAll(rule.SourcePattern, "*", ".*")
		re, err := regexp.Compile("^" + pattern + "$")
		if err != nil {
			return nil, fmt.Errorf("invalid event pattern %s: %w", rule.SourcePattern, err)
		}
		exp.eventPatterns[rule.SourcePattern] = re
	}
	
	return exp, nil
}

// start initializes the exporter
func (e *nriMetricsExporter) start(ctx context.Context, host component.Host) error {
	return e.writer.start()
}

// shutdown cleans up the exporter
func (e *nriMetricsExporter) shutdown(ctx context.Context) error {
	return e.writer.close()
}

// exportMetrics exports metrics in NRI format
func (e *nriMetricsExporter) exportMetrics(ctx context.Context, md pmetric.Metrics) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Convert metrics to NRI format
	payload := e.convertToNRI(md)
	
	// Write payload
	return e.writer.write(payload)
}

// convertToNRI converts OpenTelemetry metrics to NRI format
func (e *nriMetricsExporter) convertToNRI(md pmetric.Metrics) *nriPayload {
	payload := &nriPayload{
		Name:               e.config.IntegrationName,
		IntegrationVersion: e.config.IntegrationVersion,
		ProtocolVersion:    fmt.Sprintf("%d", e.config.ProtocolVersion),
		Data:               []nriEntity{},
	}
	
	// Group metrics by entity
	entities := make(map[string]*nriEntity)
	
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resource := rm.Resource()
		
		// Create entity key
		entityKey := e.getEntityKey(resource.Attributes())
		
		// Get or create entity
		entity, exists := entities[entityKey]
		if !exists {
			entity = &nriEntity{
				Entity: e.createEntityInfo(resource.Attributes()),
				Metrics: []nriMetric{},
			}
			entities[entityKey] = entity
		}
		
		// Process scope metrics
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				nriMetrics := e.convertMetric(metric, resource.Attributes())
				entity.Metrics = append(entity.Metrics, nriMetrics...)
			}
		}
	}
	
	// Add entities to payload
	for _, entity := range entities {
		payload.Data = append(payload.Data, *entity)
	}
	
	return payload
}

// convertMetric converts a single metric to NRI format
func (e *nriMetricsExporter) convertMetric(metric pmetric.Metric, resourceAttrs pcommon.Map) []nriMetric {
	var nriMetrics []nriMetric
	
	// Find matching rule
	var matchedRule *MetricRule
	for _, rule := range e.config.MetricRules {
		if re, ok := e.metricPatterns[rule.SourcePattern]; ok {
			if re.MatchString(metric.Name()) {
				matchedRule = &rule
				break
			}
		}
	}
	
	if matchedRule == nil {
		// No matching rule, use default conversion
		matchedRule = &MetricRule{
			TargetName:  metric.Name(),
			NRIType:     "GAUGE",
			ScaleFactor: 1.0,
		}
	}
	
	// Convert based on metric type
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		nriMetrics = e.convertGauge(metric.Gauge(), metric.Name(), matchedRule, resourceAttrs)
	case pmetric.MetricTypeSum:
		nriMetrics = e.convertSum(metric.Sum(), metric.Name(), matchedRule, resourceAttrs)
	case pmetric.MetricTypeHistogram:
		nriMetrics = e.convertHistogram(metric.Histogram(), metric.Name(), matchedRule, resourceAttrs)
	}
	
	return nriMetrics
}

// convertGauge converts gauge metrics
func (e *nriMetricsExporter) convertGauge(gauge pmetric.Gauge, name string, rule *MetricRule, resourceAttrs pcommon.Map) []nriMetric {
	var metrics []nriMetric
	
	dps := gauge.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		metric := nriMetric{
			EventType:  e.config.IntegrationName + "Sample",
			Timestamp:  dp.Timestamp().AsTime().Unix(),
			Attributes: make(map[string]interface{}),
		}
		
		// Set metric name and value
		targetName := e.applyNameTemplate(rule.TargetName, name)
		metric.Attributes[targetName] = dp.DoubleValue() * rule.ScaleFactor
		metric.Attributes["metricType"] = rule.NRIType
		
		// Add resource attributes
		resourceAttrs.Range(func(k string, v pcommon.Value) bool {
			metric.Attributes[k] = v.AsString()
			return true
		})
		
		// Add data point attributes with mapping
		dp.Attributes().Range(func(k string, v pcommon.Value) bool {
			mappedKey := k
			if mapped, ok := rule.AttributeMappings[k]; ok {
				mappedKey = mapped
			}
			
			// Check include/exclude
			if e.shouldIncludeAttribute(k, rule) {
				metric.Attributes[mappedKey] = v.AsString()
			}
			return true
		})
		
		metrics = append(metrics, metric)
	}
	
	return metrics
}

// convertSum converts sum metrics
func (e *nriMetricsExporter) convertSum(sum pmetric.Sum, name string, rule *MetricRule, resourceAttrs pcommon.Map) []nriMetric {
	var metrics []nriMetric
	
	dps := sum.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		metric := nriMetric{
			EventType:  e.config.IntegrationName + "Sample",
			Timestamp:  dp.Timestamp().AsTime().Unix(),
			Attributes: make(map[string]interface{}),
		}
		
		// Determine NRI type based on temporality
		nriType := rule.NRIType
		if nriType == "" {
			if sum.IsMonotonic() {
				nriType = "RATE"
			} else {
				nriType = "DELTA"
			}
		}
		
		// Set metric name and value
		targetName := e.applyNameTemplate(rule.TargetName, name)
		metric.Attributes[targetName] = dp.DoubleValue() * rule.ScaleFactor
		metric.Attributes["metricType"] = nriType
		
		// Add attributes
		resourceAttrs.Range(func(k string, v pcommon.Value) bool {
			metric.Attributes[k] = v.AsString()
			return true
		})
		
		dp.Attributes().Range(func(k string, v pcommon.Value) bool {
			mappedKey := k
			if mapped, ok := rule.AttributeMappings[k]; ok {
				mappedKey = mapped
			}
			
			if e.shouldIncludeAttribute(k, rule) {
				metric.Attributes[mappedKey] = v.AsString()
			}
			return true
		})
		
		metrics = append(metrics, metric)
	}
	
	return metrics
}

// convertHistogram converts histogram metrics
func (e *nriMetricsExporter) convertHistogram(hist pmetric.Histogram, name string, rule *MetricRule, resourceAttrs pcommon.Map) []nriMetric {
	var metrics []nriMetric
	
	dps := hist.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Create base metric for summary statistics
		baseMetric := nriMetric{
			EventType:  e.config.IntegrationName + "Sample",
			Timestamp:  dp.Timestamp().AsTime().Unix(),
			Attributes: make(map[string]interface{}),
		}
		
		// Add resource and datapoint attributes
		resourceAttrs.Range(func(k string, v pcommon.Value) bool {
			baseMetric.Attributes[k] = v.AsString()
			return true
		})
		
		dp.Attributes().Range(func(k string, v pcommon.Value) bool {
			mappedKey := k
			if mapped, ok := rule.AttributeMappings[k]; ok {
				mappedKey = mapped
			}
			
			if e.shouldIncludeAttribute(k, rule) {
				baseMetric.Attributes[mappedKey] = v.AsString()
			}
			return true
		})
		
		// Add summary statistics
		targetName := e.applyNameTemplate(rule.TargetName, name)
		
		// Count
		countMetric := baseMetric
		countMetric.Attributes = copyAttributes(baseMetric.Attributes)
		countMetric.Attributes[targetName+".count"] = float64(dp.Count())
		countMetric.Attributes["metricType"] = "GAUGE"
		metrics = append(metrics, countMetric)
		
		// Sum
		if dp.HasSum() {
			sumMetric := baseMetric
			sumMetric.Attributes = copyAttributes(baseMetric.Attributes)
			sumMetric.Attributes[targetName+".sum"] = dp.Sum() * rule.ScaleFactor
			sumMetric.Attributes["metricType"] = "GAUGE"
			metrics = append(metrics, sumMetric)
		}
		
		// Min
		if dp.HasMin() {
			minMetric := baseMetric
			minMetric.Attributes = copyAttributes(baseMetric.Attributes)
			minMetric.Attributes[targetName+".min"] = dp.Min() * rule.ScaleFactor
			minMetric.Attributes["metricType"] = "GAUGE"
			metrics = append(metrics, minMetric)
		}
		
		// Max
		if dp.HasMax() {
			maxMetric := baseMetric
			maxMetric.Attributes = copyAttributes(baseMetric.Attributes)
			maxMetric.Attributes[targetName+".max"] = dp.Max() * rule.ScaleFactor
			maxMetric.Attributes["metricType"] = "GAUGE"
			metrics = append(metrics, maxMetric)
		}
		
		// Percentiles from explicit bounds
		if dp.ExplicitBounds().Len() > 0 {
			percentiles := calculatePercentilesFromHistogram(dp)
			for p, value := range percentiles {
				pMetric := baseMetric
				pMetric.Attributes = copyAttributes(baseMetric.Attributes)
				pMetric.Attributes[fmt.Sprintf("%s.p%d", targetName, p)] = value * rule.ScaleFactor
				pMetric.Attributes["metricType"] = "GAUGE"
				metrics = append(metrics, pMetric)
			}
		}
	}
	
	return metrics
}

// Helper functions

func (e *nriMetricsExporter) getEntityKey(attrs pcommon.Map) string {
	// Create a unique key for the entity based on identifying attributes
	var parts []string
	
	// Common database identifiers
	identifiers := []string{"db.system", "db.name", "host.name", "service.name"}
	
	for _, id := range identifiers {
		if val, ok := attrs.Get(id); ok {
			parts = append(parts, val.AsString())
		}
	}
	
	if len(parts) == 0 {
		return "default"
	}
	
	return strings.Join(parts, ":")
}

func (e *nriMetricsExporter) createEntityInfo(attrs pcommon.Map) nriEntityInfo {
	info := nriEntityInfo{
		Type:     e.config.Entity.Type,
		Metadata: make(map[string]string),
	}
	
	// Set entity name
	if e.config.Entity.NameSource != "" {
		if val, ok := attrs.Get(e.config.Entity.NameSource); ok {
			info.Name = val.AsString()
		}
	}
	
	if info.Name == "" {
		info.Name = "unknown"
	}
	
	// Set display name using template
	if e.config.Entity.DisplayNameTemplate != "" {
		// Simple template replacement
		displayName := e.config.Entity.DisplayNameTemplate
		attrs.Range(func(k string, v pcommon.Value) bool {
			placeholder := "{{." + k + "}}"
			displayName = strings.ReplaceAll(displayName, placeholder, v.AsString())
			return true
		})
		info.DisplayName = displayName
	}
	
	// Add configured attributes
	for k, v := range e.config.Entity.Attributes {
		info.Metadata[k] = v
	}
	
	// Add selected resource attributes as metadata
	attrs.Range(func(k string, v pcommon.Value) bool {
		if strings.HasPrefix(k, "db.") || strings.HasPrefix(k, "host.") {
			info.Metadata[k] = v.AsString()
		}
		return true
	})
	
	return info
}

func (e *nriMetricsExporter) applyNameTemplate(template, metricName string) string {
	// Extract metric suffix
	parts := strings.Split(metricName, ".")
	suffix := parts[len(parts)-1]
	
	// Replace placeholders
	result := strings.ReplaceAll(template, "{{.metric_suffix}}", suffix)
	result = strings.ReplaceAll(result, "{{.metric_name}}", metricName)
	
	return result
}

func (e *nriMetricsExporter) shouldIncludeAttribute(attr string, rule *MetricRule) bool {
	// Check exclude list first
	for _, excluded := range rule.ExcludeAttributes {
		if attr == excluded {
			return false
		}
	}
	
	// If include list is specified, attribute must be in it
	if len(rule.IncludeAttributes) > 0 {
		for _, included := range rule.IncludeAttributes {
			if attr == included {
				return true
			}
		}
		return false
	}
	
	// Default to include
	return true
}

func copyAttributes(attrs map[string]interface{}) map[string]interface{} {
	copy := make(map[string]interface{})
	for k, v := range attrs {
		copy[k] = v
	}
	return copy
}

func calculatePercentilesFromHistogram(dp pmetric.HistogramDataPoint) map[int]float64 {
	// Simplified percentile calculation from histogram buckets
	// In production, this would use proper statistical methods
	percentiles := make(map[int]float64)
	
	if dp.Count() == 0 {
		return percentiles
	}
	
	// Calculate approximate percentiles
	totalCount := dp.Count()
	p50Count := uint64(float64(totalCount) * 0.5)
	p90Count := uint64(float64(totalCount) * 0.9)
	p95Count := uint64(float64(totalCount) * 0.95)
	p99Count := uint64(float64(totalCount) * 0.99)
	
	var cumulativeCount uint64
	bounds := dp.ExplicitBounds()
	buckets := dp.BucketCounts()
	
	for i := 0; i < buckets.Len() && i < bounds.Len(); i++ {
		cumulativeCount += buckets.At(i)
		
		if cumulativeCount >= p50Count && percentiles[50] == 0 {
			percentiles[50] = bounds.At(i)
		}
		if cumulativeCount >= p90Count && percentiles[90] == 0 {
			percentiles[90] = bounds.At(i)
		}
		if cumulativeCount >= p95Count && percentiles[95] == 0 {
			percentiles[95] = bounds.At(i)
		}
		if cumulativeCount >= p99Count && percentiles[99] == 0 {
			percentiles[99] = bounds.At(i)
		}
	}
	
	return percentiles
}

// Logs exporter methods

func (e *nriLogsExporter) start(ctx context.Context, host component.Host) error {
	return e.writer.start()
}

func (e *nriLogsExporter) shutdown(ctx context.Context) error {
	return e.writer.close()
}

func (e *nriLogsExporter) exportLogs(ctx context.Context, ld plog.Logs) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Convert logs to NRI events
	payload := e.convertLogsToNRI(ld)
	
	// Write payload
	return e.writer.write(payload)
}

func (e *nriLogsExporter) convertLogsToNRI(ld plog.Logs) *nriPayload {
	payload := &nriPayload{
		Name:               e.config.IntegrationName,
		IntegrationVersion: e.config.IntegrationVersion,
		ProtocolVersion:    fmt.Sprintf("%d", e.config.ProtocolVersion),
		Data:               []nriEntity{},
	}
	
	// Group events by entity
	entities := make(map[string]*nriEntity)
	
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		
		// Create entity key
		entityKey := e.getEntityKey(resource.Attributes())
		
		// Get or create entity
		entity, exists := entities[entityKey]
		if !exists {
			entity = &nriEntity{
				Entity: e.createEntityInfo(resource.Attributes()),
				Events: []nriEvent{},
			}
			entities[entityKey] = entity
		}
		
		// Process scope logs
		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			logRecords := sl.LogRecords()
			
			for k := 0; k < logRecords.Len(); k++ {
				logRecord := logRecords.At(k)
				event := e.convertLogRecord(logRecord, resource.Attributes())
				if event != nil {
					entity.Events = append(entity.Events, *event)
				}
			}
		}
	}
	
	// Add entities to payload
	for _, entity := range entities {
		if len(entity.Events) > 0 {
			payload.Data = append(payload.Data, *entity)
		}
	}
	
	return payload
}

func (e *nriLogsExporter) convertLogRecord(lr plog.LogRecord, resourceAttrs pcommon.Map) *nriEvent {
	// Find matching event rule
	var matchedRule *EventRule
	
	// Check body for pattern matching
	body := lr.Body().AsString()
	for _, rule := range e.config.EventRules {
		if re, ok := e.eventPatterns[rule.SourcePattern]; ok {
			if re.MatchString(body) {
				matchedRule = &rule
				break
			}
		}
	}
	
	if matchedRule == nil {
		// No matching rule, skip
		return nil
	}
	
	event := &nriEvent{
		EventType:  matchedRule.EventType,
		Timestamp:  lr.Timestamp().AsTime().Unix(),
		Category:   matchedRule.Category,
		Attributes: make(map[string]interface{}),
	}
	
	// Build summary using template
	summary := matchedRule.SummaryTemplate
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		placeholder := "{{." + k + "}}"
		summary = strings.ReplaceAll(summary, placeholder, v.AsString())
		return true
	})
	event.Summary = summary
	
	// Add attributes
	resourceAttrs.Range(func(k string, v pcommon.Value) bool {
		event.Attributes[k] = v.AsString()
		return true
	})
	
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		mappedKey := k
		if mapped, ok := matchedRule.AttributeMappings[k]; ok {
			mappedKey = mapped
		}
		event.Attributes[mappedKey] = v.AsString()
		return true
	})
	
	// Add severity
	event.Attributes["severity"] = lr.SeverityText()
	
	return event
}