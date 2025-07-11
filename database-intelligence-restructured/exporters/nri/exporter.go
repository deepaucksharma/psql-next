package nri

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// nriMetricsExporter exports metrics to New Relic Infrastructure
type nriMetricsExporter struct {
	config         *Config
	logger         *zap.Logger
	writer         nriWriter
	metricPatterns map[string]*regexp.Regexp
	mu             sync.RWMutex
}

// nriLogsExporter exports logs to New Relic Infrastructure
type nriLogsExporter struct {
	config       *Config
	logger       *zap.Logger
	writer       nriWriter
	eventPatterns map[string]*regexp.Regexp
	mu           sync.RWMutex
}

// start starts the exporter
func (exp *nriMetricsExporter) start(ctx context.Context, host component.Host) error {
	exp.logger.Info("Starting NRI metrics exporter",
		zap.String("integration_name", exp.config.IntegrationName),
		zap.String("output_mode", exp.config.OutputMode))
	
	// Compile metric patterns
	exp.mu.Lock()
	defer exp.mu.Unlock()
	
	for _, rule := range exp.config.MetricRules {
		pattern, err := regexp.Compile(rule.SourcePattern)
		if err != nil {
			return fmt.Errorf("invalid metric pattern %s: %w", rule.SourcePattern, err)
		}
		exp.metricPatterns[rule.SourcePattern] = pattern
	}
	
	return nil
}

// shutdown stops the exporter
func (exp *nriMetricsExporter) shutdown(ctx context.Context) error {
	exp.logger.Info("Shutting down NRI metrics exporter")
	return exp.writer.close()
}

// exportMetrics exports metrics to NRI
func (exp *nriMetricsExporter) exportMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Convert metrics to NRI format
	nriPayload := exp.convertMetrics(md)
	
	// Write payload
	if err := exp.writer.write(nriPayload); err != nil {
		return fmt.Errorf("failed to write metrics: %w", err)
	}
	
	return nil
}

// newMetricsExporter creates a new metrics exporter
func newMetricsExporter(config *Config, settings component.TelemetrySettings) (*nriMetricsExporter, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Create writer
	writer, err := newNRIWriter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create writer: %w", err)
	}
	
	exp := &nriMetricsExporter{
		config:         config,
		logger:         settings.Logger,
		writer:         writer,
		metricPatterns: make(map[string]*regexp.Regexp),
	}
	
	return exp, nil
}

// newLogsExporter creates a new logs exporter
func newLogsExporter(config *Config, settings component.TelemetrySettings) (*nriLogsExporter, error) {
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Create writer
	writer, err := newNRIWriter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create writer: %w", err)
	}
	
	exp := &nriLogsExporter{
		config:        config,
		logger:        settings.Logger,
		writer:        writer,
		eventPatterns: make(map[string]*regexp.Regexp),
	}
	
	return exp, nil
}

// start starts the logs exporter
func (exp *nriLogsExporter) start(ctx context.Context, host component.Host) error {
	exp.logger.Info("Starting NRI logs exporter",
		zap.String("integration_name", exp.config.IntegrationName),
		zap.String("output_mode", exp.config.OutputMode))
	
	// Compile event patterns
	exp.mu.Lock()
	defer exp.mu.Unlock()
	
	for _, rule := range exp.config.EventRules {
		pattern, err := regexp.Compile(rule.SourcePattern)
		if err != nil {
			return fmt.Errorf("invalid event pattern %s: %w", rule.SourcePattern, err)
		}
		exp.eventPatterns[rule.SourcePattern] = pattern
	}
	
	return nil
}

// shutdown stops the logs exporter
func (exp *nriLogsExporter) shutdown(ctx context.Context) error {
	exp.logger.Info("Shutting down NRI logs exporter")
	return exp.writer.close()
}

// exportLogs exports logs to NRI
func (exp *nriLogsExporter) exportLogs(ctx context.Context, ld plog.Logs) error {
	// Convert logs to NRI events
	nriPayload := exp.convertLogs(ld)
	
	// Write payload
	if err := exp.writer.write(nriPayload); err != nil {
		return fmt.Errorf("failed to write logs: %w", err)
	}
	
	return nil
}

// convertMetrics converts OTel metrics to NRI format
func (exp *nriMetricsExporter) convertMetrics(md pmetric.Metrics) interface{} {
	exp.mu.RLock()
	defer exp.mu.RUnlock()
	
	// Create NRI payload
	payload := map[string]interface{}{
		"name":                exp.config.IntegrationName,
		"integration_version": exp.config.IntegrationVersion,
		"protocol_version":    exp.config.ProtocolVersion,
		"data":                []interface{}{},
	}
	
	data := make([]interface{}, 0)
	
	// Process resource metrics
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resource := rm.Resource()
		
		// Create entity from resource
		entity := exp.createEntity(resource)
		
		// Process scope metrics
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			
			// Process metrics
			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				
				// Convert metric based on matching rules
				if nriMetrics := exp.convertMetric(metric, resource); len(nriMetrics) > 0 {
					entityData := map[string]interface{}{
						"entity":  entity,
						"metrics": nriMetrics,
					}
					data = append(data, entityData)
				}
			}
		}
	}
	
	payload["data"] = data
	return payload
}

// convertLogs converts OTel logs to NRI events
func (exp *nriLogsExporter) convertLogs(ld plog.Logs) interface{} {
	exp.mu.RLock()
	defer exp.mu.RUnlock()
	
	// Create NRI payload
	payload := map[string]interface{}{
		"name":                exp.config.IntegrationName,
		"integration_version": exp.config.IntegrationVersion,
		"protocol_version":    exp.config.ProtocolVersion,
		"data":                []interface{}{},
	}
	
	data := make([]interface{}, 0)
	
	// Process resource logs
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		
		// Create entity from resource
		entity := exp.createEntity(resource)
		
		// Process scope logs
		sls := rl.ScopeLogs()
		for j := 0; j < sls.Len(); j++ {
			sl := sls.At(j)
			
			// Process log records
			logs := sl.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				
				// Convert log to event based on matching rules
				if event := exp.convertLogToEvent(log, resource); event != nil {
					entityData := map[string]interface{}{
						"entity": entity,
						"events": []interface{}{event},
					}
					data = append(data, entityData)
				}
			}
		}
	}
	
	payload["data"] = data
	return payload
}

// createEntity creates an NRI entity from resource attributes
func (exp *nriMetricsExporter) createEntity(resource pcommon.Resource) map[string]interface{} {
	entity := map[string]interface{}{
		"type": exp.config.Entity.Type,
	}
	
	// Get entity name from configured source
	attrs := resource.Attributes()
	if nameValue, ok := attrs.Get(exp.config.Entity.NameSource); ok {
		entity["name"] = nameValue.AsString()
	} else {
		entity["name"] = "unknown"
	}
	
	// Set display name if template is configured
	if exp.config.Entity.DisplayNameTemplate != "" {
		entity["displayName"] = exp.expandTemplate(exp.config.Entity.DisplayNameTemplate, attrs)
	}
	
	// Add configured attributes
	if len(exp.config.Entity.Attributes) > 0 {
		entity["metadata"] = exp.config.Entity.Attributes
	}
	
	return entity
}

// createEntity creates an NRI entity from resource attributes (logs exporter)
func (exp *nriLogsExporter) createEntity(resource pcommon.Resource) map[string]interface{} {
	entity := map[string]interface{}{
		"type": exp.config.Entity.Type,
	}
	
	// Get entity name from configured source
	attrs := resource.Attributes()
	if nameValue, ok := attrs.Get(exp.config.Entity.NameSource); ok {
		entity["name"] = nameValue.AsString()
	} else {
		entity["name"] = "unknown"
	}
	
	// Set display name if template is configured
	if exp.config.Entity.DisplayNameTemplate != "" {
		entity["displayName"] = exp.expandTemplate(exp.config.Entity.DisplayNameTemplate, attrs)
	}
	
	// Add configured attributes
	if len(exp.config.Entity.Attributes) > 0 {
		entity["metadata"] = exp.config.Entity.Attributes
	}
	
	return entity
}

// convertMetric converts a single metric based on rules
func (exp *nriMetricsExporter) convertMetric(metric pmetric.Metric, resource pcommon.Resource) []map[string]interface{} {
	metricName := metric.Name()
	
	// Find matching rule
	var matchedRule *MetricRule
	for _, rule := range exp.config.MetricRules {
		if pattern, ok := exp.metricPatterns[rule.SourcePattern]; ok && pattern.MatchString(metricName) {
			matchedRule = &rule
			break
		}
	}
	
	if matchedRule == nil {
		return nil
	}
	
	// Convert based on metric type
	var nriMetrics []map[string]interface{}
	
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		nriMetrics = exp.convertGauge(metric.Gauge(), metricName, matchedRule)
	case pmetric.MetricTypeSum:
		nriMetrics = exp.convertSum(metric.Sum(), metricName, matchedRule)
	case pmetric.MetricTypeHistogram:
		nriMetrics = exp.convertHistogram(metric.Histogram(), metricName, matchedRule)
	case pmetric.MetricTypeSummary:
		nriMetrics = exp.convertSummary(metric.Summary(), metricName, matchedRule)
	}
	
	return nriMetrics
}

// convertGauge converts a gauge metric
func (exp *nriMetricsExporter) convertGauge(gauge pmetric.Gauge, metricName string, rule *MetricRule) []map[string]interface{} {
	var metrics []map[string]interface{}
	
	dps := gauge.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Create NRI metric
		nriMetric := map[string]interface{}{
			"name":      exp.expandMetricName(rule.TargetName, metricName),
			"type":      rule.NRIType,
			"timestamp": dp.Timestamp().AsTime().Unix(),
		}
		
		// Set value based on type
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			nriMetric["value"] = float64(dp.IntValue()) * rule.ScaleFactor
		case pmetric.NumberDataPointValueTypeDouble:
			nriMetric["value"] = dp.DoubleValue() * rule.ScaleFactor
		}
		
		// Add attributes
		attrs := exp.filterAttributes(dp.Attributes(), rule)
		if len(attrs) > 0 {
			nriMetric["attributes"] = attrs
		}
		
		metrics = append(metrics, nriMetric)
	}
	
	return metrics
}

// convertSum converts a sum metric
func (exp *nriMetricsExporter) convertSum(sum pmetric.Sum, metricName string, rule *MetricRule) []map[string]interface{} {
	var metrics []map[string]interface{}
	
	// Determine NRI type based on sum properties
	nriType := rule.NRIType
	if nriType == "" {
		if sum.IsMonotonic() {
			nriType = "CUMULATIVE"
		} else {
			nriType = "GAUGE"
		}
	}
	
	dps := sum.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Create NRI metric
		nriMetric := map[string]interface{}{
			"name":      exp.expandMetricName(rule.TargetName, metricName),
			"type":      nriType,
			"timestamp": dp.Timestamp().AsTime().Unix(),
		}
		
		// Set value based on type
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			nriMetric["value"] = float64(dp.IntValue()) * rule.ScaleFactor
		case pmetric.NumberDataPointValueTypeDouble:
			nriMetric["value"] = dp.DoubleValue() * rule.ScaleFactor
		}
		
		// Add attributes
		attrs := exp.filterAttributes(dp.Attributes(), rule)
		if len(attrs) > 0 {
			nriMetric["attributes"] = attrs
		}
		
		metrics = append(metrics, nriMetric)
	}
	
	return metrics
}

// convertHistogram converts a histogram metric
func (exp *nriMetricsExporter) convertHistogram(histogram pmetric.Histogram, metricName string, rule *MetricRule) []map[string]interface{} {
	var metrics []map[string]interface{}
	
	dps := histogram.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Create summary metrics
		baseAttrs := exp.filterAttributes(dp.Attributes(), rule)
		timestamp := dp.Timestamp().AsTime().Unix()
		
		// Count metric
		metrics = append(metrics, map[string]interface{}{
			"name":       exp.expandMetricName(rule.TargetName+".count", metricName),
			"type":       "CUMULATIVE",
			"value":      float64(dp.Count()),
			"timestamp":  timestamp,
			"attributes": baseAttrs,
		})
		
		// Sum metric
		if dp.HasSum() {
			metrics = append(metrics, map[string]interface{}{
				"name":       exp.expandMetricName(rule.TargetName+".sum", metricName),
				"type":       "CUMULATIVE",
				"value":      dp.Sum() * rule.ScaleFactor,
				"timestamp":  timestamp,
				"attributes": baseAttrs,
			})
		}
		
		// Bucket metrics
		buckets := dp.BucketCounts()
		bounds := dp.ExplicitBounds()
		for j := 0; j < buckets.Len() && j < bounds.Len(); j++ {
			bucketAttrs := make(map[string]interface{})
			for k, v := range baseAttrs {
				bucketAttrs[k] = v
			}
			bucketAttrs["bucket.max"] = bounds.At(j)
			
			metrics = append(metrics, map[string]interface{}{
				"name":       exp.expandMetricName(rule.TargetName+".bucket", metricName),
				"type":       "CUMULATIVE",
				"value":      float64(buckets.At(j)),
				"timestamp":  timestamp,
				"attributes": bucketAttrs,
			})
		}
	}
	
	return metrics
}

// convertSummary converts a summary metric
func (exp *nriMetricsExporter) convertSummary(summary pmetric.Summary, metricName string, rule *MetricRule) []map[string]interface{} {
	var metrics []map[string]interface{}
	
	dps := summary.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Create summary metrics
		baseAttrs := exp.filterAttributes(dp.Attributes(), rule)
		timestamp := dp.Timestamp().AsTime().Unix()
		
		// Count metric
		metrics = append(metrics, map[string]interface{}{
			"name":       exp.expandMetricName(rule.TargetName+".count", metricName),
			"type":       "CUMULATIVE",
			"value":      float64(dp.Count()),
			"timestamp":  timestamp,
			"attributes": baseAttrs,
		})
		
		// Sum metric
		metrics = append(metrics, map[string]interface{}{
			"name":       exp.expandMetricName(rule.TargetName+".sum", metricName),
			"type":       "CUMULATIVE",
			"value":      dp.Sum() * rule.ScaleFactor,
			"timestamp":  timestamp,
			"attributes": baseAttrs,
		})
		
		// Quantile metrics
		quantiles := dp.QuantileValues()
		for j := 0; j < quantiles.Len(); j++ {
			q := quantiles.At(j)
			quantileAttrs := make(map[string]interface{})
			for k, v := range baseAttrs {
				quantileAttrs[k] = v
			}
			quantileAttrs["quantile"] = q.Quantile()
			
			metrics = append(metrics, map[string]interface{}{
				"name":       exp.expandMetricName(rule.TargetName+".quantile", metricName),
				"type":       "GAUGE",
				"value":      q.Value() * rule.ScaleFactor,
				"timestamp":  timestamp,
				"attributes": quantileAttrs,
			})
		}
	}
	
	return metrics
}

// convertLogToEvent converts a log record to an NRI event
func (exp *nriLogsExporter) convertLogToEvent(log plog.LogRecord, resource pcommon.Resource) map[string]interface{} {
	// Get log attributes
	attrs := log.Attributes()
	
	// Check if log matches any event rule
	var matchedRule *EventRule
	for _, rule := range exp.config.EventRules {
		if pattern, ok := exp.eventPatterns[rule.SourcePattern]; ok {
			// Check various fields for pattern match
			if pattern.MatchString(log.Body().AsString()) ||
				pattern.MatchString(log.SeverityText()) {
				matchedRule = &rule
				break
			}
			
			// Check attributes
			attrs.Range(func(k string, v pcommon.Value) bool {
				if pattern.MatchString(k) || pattern.MatchString(v.AsString()) {
					matchedRule = &rule
					return false
				}
				return true
			})
			
			if matchedRule != nil {
				break
			}
		}
	}
	
	if matchedRule == nil {
		return nil
	}
	
	// Create NRI event
	event := map[string]interface{}{
		"eventType": matchedRule.EventType,
		"category":  matchedRule.Category,
		"timestamp": log.Timestamp().AsTime().Unix(),
	}
	
	// Set summary
	if matchedRule.SummaryTemplate != "" {
		event["summary"] = exp.expandTemplate(matchedRule.SummaryTemplate, attrs)
	} else {
		event["summary"] = log.Body().AsString()
	}
	
	// Add severity
	if log.SeverityText() != "" {
		event["severity"] = log.SeverityText()
	}
	
	// Map attributes
	eventAttrs := make(map[string]interface{})
	
	// Apply attribute mappings
	if len(matchedRule.AttributeMappings) > 0 {
		for source, target := range matchedRule.AttributeMappings {
			if value, ok := attrs.Get(source); ok {
				eventAttrs[target] = value.AsString()
			}
		}
	} else {
		// Add all attributes
		attrs.Range(func(k string, v pcommon.Value) bool {
			eventAttrs[k] = v.AsString()
			return true
		})
	}
	
	if len(eventAttrs) > 0 {
		event["attributes"] = eventAttrs
	}
	
	return event
}

// filterAttributes filters attributes based on rules
func (exp *nriMetricsExporter) filterAttributes(attrs pcommon.Map, rule *MetricRule) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Apply attribute mappings if configured
	if len(rule.AttributeMappings) > 0 {
		for source, target := range rule.AttributeMappings {
			if value, ok := attrs.Get(source); ok {
				result[target] = value.AsString()
			}
		}
		return result
	}
	
	// Otherwise, apply include/exclude filters
	attrs.Range(func(k string, v pcommon.Value) bool {
		// Check exclude list
		excluded := false
		for _, pattern := range rule.ExcludeAttributes {
			if matched, _ := regexp.MatchString(pattern, k); matched {
				excluded = true
				break
			}
		}
		
		if excluded {
			return true
		}
		
		// Check include list
		if len(rule.IncludeAttributes) > 0 {
			included := false
			for _, pattern := range rule.IncludeAttributes {
				if matched, _ := regexp.MatchString(pattern, k); matched {
					included = true
					break
				}
			}
			
			if !included {
				return true
			}
		}
		
		// Add attribute
		result[k] = v.AsString()
		return true
	})
	
	return result
}

// expandMetricName expands metric name template
func (exp *nriMetricsExporter) expandMetricName(template, originalName string) string {
	// Extract metric suffix from original name
	parts := strings.Split(originalName, ".")
	suffix := parts[len(parts)-1]
	
	// Simple template expansion
	result := strings.ReplaceAll(template, "{{.metric_suffix}}", suffix)
	result = strings.ReplaceAll(result, "{{.metric_name}}", originalName)
	
	return result
}

// expandTemplate expands a template with attributes
func (exp *nriMetricsExporter) expandTemplate(template string, attrs pcommon.Map) string {
	result := template
	
	// Simple template expansion
	attrs.Range(func(k string, v pcommon.Value) bool {
		placeholder := fmt.Sprintf("{{.%s}}", k)
		result = strings.ReplaceAll(result, placeholder, v.AsString())
		return true
	})
	
	return result
}

// expandTemplate expands a template with attributes (logs exporter)
func (exp *nriLogsExporter) expandTemplate(template string, attrs pcommon.Map) string {
	result := template
	
	// Simple template expansion
	attrs.Range(func(k string, v pcommon.Value) bool {
		placeholder := fmt.Sprintf("{{.%s}}", k)
		result = strings.ReplaceAll(result, placeholder, v.AsString())
		return true
	})
	
	return result
}