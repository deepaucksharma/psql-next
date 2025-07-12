package ohitransform

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type ohiTransformProcessor struct {
	config *Config
	logger *zap.Logger
}

// processMetrics transforms OTel metrics to OHI-compatible format
func (otp *ohiTransformProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	// Create a new metrics object for transformed data
	transformed := pmetric.NewMetrics()
	
	// Process each resource metric
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resource := rm.Resource()
		
		// Create corresponding resource in transformed metrics
		trm := transformed.ResourceMetrics().AppendEmpty()
		resource.CopyTo(trm.Resource())
		
		// Process scope metrics
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			
			// Create corresponding scope in transformed metrics
			tsm := trm.ScopeMetrics().AppendEmpty()
			sm.Scope().CopyTo(tsm.Scope())
			
			// Process metrics
			metrics := sm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				
				// Check if this metric needs transformation
				if rule := otp.findTransformRule(metric.Name()); rule != nil {
					otp.transformMetric(metric, rule, tsm)
				} else if otp.config.PreserveOriginalMetrics {
					// Copy metric as-is if no transformation rule found
					newMetric := tsm.Metrics().AppendEmpty()
					metric.CopyTo(newMetric)
				}
			}
		}
	}
	
	// If preserving original metrics, merge them with transformed
	if otp.config.PreserveOriginalMetrics {
		// Copy all original metrics
		rms = md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			rm := rms.At(i)
			trm := transformed.ResourceMetrics().AppendEmpty()
			rm.CopyTo(trm)
		}
	}
	
	return transformed, nil
}

// findTransformRule finds a matching transform rule for the given metric name
func (otp *ohiTransformProcessor) findTransformRule(metricName string) *TransformRule {
	for i := range otp.config.TransformRules {
		if otp.config.TransformRules[i].SourceMetric == metricName {
			return &otp.config.TransformRules[i]
		}
	}
	return nil
}

// transformMetric applies transformation rule to convert OTel metric to OHI event format
func (otp *ohiTransformProcessor) transformMetric(metric pmetric.Metric, rule *TransformRule, tsm pmetric.ScopeMetrics) {
	// Create new metric with OHI event name
	newMetric := tsm.Metrics().AppendEmpty()
	newMetric.SetName(rule.TargetEvent)
	newMetric.SetDescription(fmt.Sprintf("Transformed from %s", metric.Name()))
	
	// Handle different metric types
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		otp.transformGauge(metric.Gauge(), rule, newMetric)
	case pmetric.MetricTypeSum:
		otp.transformSum(metric.Sum(), rule, newMetric)
	case pmetric.MetricTypeHistogram:
		otp.transformHistogram(metric.Histogram(), rule, newMetric)
	case pmetric.MetricTypeSummary:
		otp.transformSummary(metric.Summary(), rule, newMetric)
	default:
		otp.logger.Warn("unsupported metric type for transformation",
			zap.String("metric", metric.Name()),
			zap.String("type", metric.Type().String()))
	}
}

// transformGauge transforms gauge metrics
func (otp *ohiTransformProcessor) transformGauge(gauge pmetric.Gauge, rule *TransformRule, newMetric pmetric.Metric) {
	newGauge := newMetric.SetEmptyGauge()
	dps := gauge.DataPoints()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Check filters
		if !otp.matchesFilters(dp.Attributes(), rule.Filters) {
			continue
		}
		
		newDp := newGauge.DataPoints().AppendEmpty()
		otp.transformDataPoint(dp, rule, newDp)
	}
}

// transformSum transforms sum metrics
func (otp *ohiTransformProcessor) transformSum(sum pmetric.Sum, rule *TransformRule, newMetric pmetric.Metric) {
	// Convert sum to gauge for OHI compatibility
	newGauge := newMetric.SetEmptyGauge()
	dps := sum.DataPoints()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Check filters
		if !otp.matchesFilters(dp.Attributes(), rule.Filters) {
			continue
		}
		
		newDp := newGauge.DataPoints().AppendEmpty()
		otp.transformNumberDataPoint(dp, rule, newDp)
	}
}

// transformHistogram transforms histogram metrics
func (otp *ohiTransformProcessor) transformHistogram(hist pmetric.Histogram, rule *TransformRule, newMetric pmetric.Metric) {
	// Convert histogram to multiple gauge metrics for OHI compatibility
	newGauge := newMetric.SetEmptyGauge()
	dps := hist.DataPoints()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Check filters
		if !otp.matchesFilters(dp.Attributes(), rule.Filters) {
			continue
		}
		
		// Create data points for different histogram statistics
		if dp.HasSum() {
			sumDp := newGauge.DataPoints().AppendEmpty()
			sumDp.SetTimestamp(dp.Timestamp())
			sumDp.SetDoubleValue(dp.Sum())
			otp.transformAttributes(dp.Attributes(), rule, sumDp.Attributes())
			sumDp.Attributes().PutStr("statistic", "sum")
		}
		
		if dp.HasCount() {
			countDp := newGauge.DataPoints().AppendEmpty()
			countDp.SetTimestamp(dp.Timestamp())
			countDp.SetIntValue(int64(dp.Count()))
			otp.transformAttributes(dp.Attributes(), rule, countDp.Attributes())
			countDp.Attributes().PutStr("statistic", "count")
		}
		
		// Add percentiles if available
		buckets := dp.BucketCounts()
		bounds := dp.ExplicitBounds()
		if buckets.Len() > 0 && bounds.Len() > 0 {
			// Calculate p95, p99, etc.
			otp.addPercentiles(dp, rule, newGauge)
		}
	}
}

// transformSummary transforms summary metrics
func (otp *ohiTransformProcessor) transformSummary(summary pmetric.Summary, rule *TransformRule, newMetric pmetric.Metric) {
	newGauge := newMetric.SetEmptyGauge()
	dps := summary.DataPoints()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		
		// Check filters
		if !otp.matchesFilters(dp.Attributes(), rule.Filters) {
			continue
		}
		
		// Create data points for sum and count
		if dp.Sum() != 0 {
			sumDp := newGauge.DataPoints().AppendEmpty()
			sumDp.SetTimestamp(dp.Timestamp())
			sumDp.SetDoubleValue(dp.Sum())
			otp.transformAttributes(dp.Attributes(), rule, sumDp.Attributes())
			sumDp.Attributes().PutStr("statistic", "sum")
		}
		
		countDp := newGauge.DataPoints().AppendEmpty()
		countDp.SetTimestamp(dp.Timestamp())
		countDp.SetIntValue(int64(dp.Count()))
		otp.transformAttributes(dp.Attributes(), rule, countDp.Attributes())
		countDp.Attributes().PutStr("statistic", "count")
		
		// Add quantiles
		quantiles := dp.QuantileValues()
		for j := 0; j < quantiles.Len(); j++ {
			q := quantiles.At(j)
			qDp := newGauge.DataPoints().AppendEmpty()
			qDp.SetTimestamp(dp.Timestamp())
			qDp.SetDoubleValue(q.Value())
			otp.transformAttributes(dp.Attributes(), rule, qDp.Attributes())
			qDp.Attributes().PutStr("statistic", fmt.Sprintf("p%d", int(q.Quantile()*100)))
		}
	}
}

// Helper methods

func (otp *ohiTransformProcessor) transformDataPoint(dp pmetric.NumberDataPoint, rule *TransformRule, newDp pmetric.NumberDataPoint) {
	newDp.SetTimestamp(dp.Timestamp())
	
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		newDp.SetIntValue(dp.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		newDp.SetDoubleValue(dp.DoubleValue())
	}
	
	otp.transformAttributes(dp.Attributes(), rule, newDp.Attributes())
}

func (otp *ohiTransformProcessor) transformNumberDataPoint(dp pmetric.NumberDataPoint, rule *TransformRule, newDp pmetric.NumberDataPoint) {
	otp.transformDataPoint(dp, rule, newDp)
}

func (otp *ohiTransformProcessor) transformAttributes(src pcommon.Map, rule *TransformRule, dest pcommon.Map) {
	// Apply attribute mappings
	for otelAttr, ohiAttr := range rule.Mappings {
		if val, ok := src.Get(otelAttr); ok {
			dest.PutStr(ohiAttr, val.AsString())
		}
	}
	
	// Add timestamp for OHI compatibility
	dest.PutInt("timestamp", time.Now().Unix())
	
	// Add event type marker
	dest.PutStr("eventType", rule.TargetEvent)
	
	// Copy any unmapped attributes as-is
	src.Range(func(k string, v pcommon.Value) bool {
		if _, mapped := rule.Mappings[k]; !mapped {
			v.CopyTo(dest.PutEmpty(k))
		}
		return true
	})
}

func (otp *ohiTransformProcessor) matchesFilters(attrs pcommon.Map, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}
	
	for k, v := range filters {
		if val, ok := attrs.Get(k); !ok || val.AsString() != v {
			return false
		}
	}
	
	return true
}

func (otp *ohiTransformProcessor) addPercentiles(dp pmetric.HistogramDataPoint, rule *TransformRule, gauge pmetric.Gauge) {
	// Calculate common percentiles from histogram buckets
	total := dp.Count()
	if total == 0 {
		return
	}
	
	percentiles := []float64{0.5, 0.95, 0.99}
	buckets := dp.BucketCounts()
	bounds := dp.ExplicitBounds()
	
	cumulative := uint64(0)
	for _, p := range percentiles {
		target := uint64(float64(total) * p)
		
		for i := 0; i < buckets.Len(); i++ {
			cumulative += buckets.At(i)
			if cumulative >= target {
				// Interpolate the percentile value
				var value float64
				if i == 0 {
					value = bounds.At(0) / 2 // Estimate for first bucket
				} else if i < bounds.Len() {
					value = (bounds.At(i-1) + bounds.At(i)) / 2
				} else {
					value = bounds.At(bounds.Len()-1) * 1.5 // Estimate for last bucket
				}
				
				pDp := gauge.DataPoints().AppendEmpty()
				pDp.SetTimestamp(dp.Timestamp())
				pDp.SetDoubleValue(value)
				otp.transformAttributes(dp.Attributes(), rule, pDp.Attributes())
				pDp.Attributes().PutStr("statistic", fmt.Sprintf("p%d", int(p*100)))
				break
			}
		}
	}
}