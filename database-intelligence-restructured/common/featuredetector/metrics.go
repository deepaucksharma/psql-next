package featuredetector

import (
	"context"
	"time"
	
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// MetricsBuilder creates metrics from feature detection results
type MetricsBuilder struct {
	scope string
}

// NewMetricsBuilder creates a new metrics builder
func NewMetricsBuilder(scope string) *MetricsBuilder {
	return &MetricsBuilder{
		scope: scope,
	}
}

// BuildMetrics creates OTLP metrics from feature detection results
func (mb *MetricsBuilder) BuildMetrics(features *FeatureSet) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	
	// Set resource attributes
	resource := rm.Resource()
	resource.Attributes().PutStr("db.system", features.DatabaseType)
	resource.Attributes().PutStr("db.version", features.ServerVersion)
	if features.CloudProvider != "" && features.CloudProvider != CloudProviderLocal {
		resource.Attributes().PutStr("cloud.provider", features.CloudProvider)
		if variant, ok := features.Metadata["cloud_variant"].(string); ok {
			resource.Attributes().PutStr("cloud.platform", variant)
		}
	}
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName(mb.scope)
	
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	
	// Extension availability metric
	mb.addExtensionMetrics(sm, features, timestamp)
	
	// Capability availability metric
	mb.addCapabilityMetrics(sm, features, timestamp)
	
	// Feature detection health metric
	mb.addHealthMetrics(sm, features, timestamp)
	
	// Query compatibility metrics
	mb.addQueryCompatibilityMetrics(sm, features, timestamp)
	
	return metrics
}

// addExtensionMetrics adds extension availability metrics
func (mb *MetricsBuilder) addExtensionMetrics(sm pmetric.ScopeMetrics, features *FeatureSet, timestamp pcommon.Timestamp) {
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.feature.extension.available")
	metric.SetDescription("Indicates whether a database extension is available (1) or not (0)")
	metric.SetUnit("1")
	gauge := metric.SetEmptyGauge()
	
	for name, ext := range features.Extensions {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(timestamp)
		dp.Attributes().PutStr("extension", name)
		
		if ext.Available {
			dp.SetIntValue(1)
			if ext.Version != "" {
				dp.Attributes().PutStr("version", ext.Version)
			}
		} else {
			dp.SetIntValue(0)
			if ext.ErrorMessage != "" {
				dp.Attributes().PutStr("error", ext.ErrorMessage)
			}
		}
	}
}

// addCapabilityMetrics adds capability availability metrics
func (mb *MetricsBuilder) addCapabilityMetrics(sm pmetric.ScopeMetrics, features *FeatureSet, timestamp pcommon.Timestamp) {
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.feature.capability.available")
	metric.SetDescription("Indicates whether a database capability is available (1) or not (0)")
	metric.SetUnit("1")
	gauge := metric.SetEmptyGauge()
	
	for name, cap := range features.Capabilities {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(timestamp)
		dp.Attributes().PutStr("capability", name)
		
		if cap.Available {
			dp.SetIntValue(1)
			if cap.Version != "" {
				dp.Attributes().PutStr("value", cap.Version)
			}
		} else {
			dp.SetIntValue(0)
			if cap.ErrorMessage != "" {
				dp.Attributes().PutStr("error", cap.ErrorMessage)
			}
		}
	}
}

// addHealthMetrics adds feature detection health metrics
func (mb *MetricsBuilder) addHealthMetrics(sm pmetric.ScopeMetrics, features *FeatureSet, timestamp pcommon.Timestamp) {
	// Detection errors count
	errorsMetric := sm.Metrics().AppendEmpty()
	errorsMetric.SetName("db.feature.detection.errors")
	errorsMetric.SetDescription("Number of errors during feature detection")
	errorsMetric.SetUnit("1")
	errorsSum := errorsMetric.SetEmptySum()
	errorsSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	dp := errorsSum.DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetIntValue(int64(len(features.DetectionErrors)))
	
	// Detection age metric
	ageMetric := sm.Metrics().AppendEmpty()
	ageMetric.SetName("db.feature.detection.age")
	ageMetric.SetDescription("Time since last feature detection")
	ageMetric.SetUnit("s")
	ageGauge := ageMetric.SetEmptyGauge()
	
	ageDp := ageGauge.DataPoints().AppendEmpty()
	ageDp.SetTimestamp(timestamp)
	ageDp.SetDoubleValue(time.Since(features.LastDetection).Seconds())
	
	// Total features detected
	totalMetric := sm.Metrics().AppendEmpty()
	totalMetric.SetName("db.feature.total")
	totalMetric.SetDescription("Total number of features detected")
	totalMetric.SetUnit("1")
	totalGauge := totalMetric.SetEmptyGauge()
	
	totalDp := totalGauge.DataPoints().AppendEmpty()
	totalDp.SetTimestamp(timestamp)
	totalDp.SetIntValue(int64(len(features.Extensions) + len(features.Capabilities)))
	totalDp.Attributes().PutStr("type", "all")
	
	// Available features
	availableDp := totalGauge.DataPoints().AppendEmpty()
	availableDp.SetTimestamp(timestamp)
	available := 0
	for _, ext := range features.Extensions {
		if ext.Available {
			available++
		}
	}
	for _, cap := range features.Capabilities {
		if cap.Available {
			available++
		}
	}
	availableDp.SetIntValue(int64(available))
	availableDp.Attributes().PutStr("type", "available")
}

// addQueryCompatibilityMetrics adds metrics about query compatibility
func (mb *MetricsBuilder) addQueryCompatibilityMetrics(sm pmetric.ScopeMetrics, features *FeatureSet, timestamp pcommon.Timestamp) {
	// This would be populated by the query selector
	// For now, we'll add a placeholder metric structure
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.query.compatibility")
	metric.SetDescription("Indicates whether queries are compatible with current features")
	metric.SetUnit("1")
	gauge := metric.SetEmptyGauge()
	
	// Add data points based on known query categories
	categories := []string{"slow_queries", "active_sessions", "wait_events", "table_stats"}
	
	for _, category := range categories {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(timestamp)
		dp.Attributes().PutStr("category", category)
		
		// Check basic compatibility
		compatible := mb.checkCategoryCompatibility(features, category)
		if compatible {
			dp.SetIntValue(1)
		} else {
			dp.SetIntValue(0)
		}
	}
}

// checkCategoryCompatibility checks if a query category is compatible
func (mb *MetricsBuilder) checkCategoryCompatibility(features *FeatureSet, category string) bool {
	switch features.DatabaseType {
	case DatabaseTypePostgreSQL:
		switch category {
		case "slow_queries":
			return features.HasFeature(ExtPgStatStatements) || features.HasFeature(ExtPgStatMonitor)
		case "wait_events":
			return features.HasFeature(ExtPgWaitSampling)
		default:
			return true // Most categories work with basic permissions
		}
		
	case DatabaseTypeMySQL:
		switch category {
		case "slow_queries":
			return features.HasFeature(CapPerfSchemaEnabled) && features.HasFeature(CapPerfSchemaStatementsDigest)
		case "wait_events":
			return features.HasFeature(CapPerfSchemaEnabled) && features.HasFeature(CapPerfSchemaWaits)
		default:
			return true
		}
		
	default:
		return false
	}
}

// EmitFeatureMetrics is a convenience function to emit metrics for a feature set
func EmitFeatureMetrics(ctx context.Context, features *FeatureSet, scope string) pmetric.Metrics {
	builder := NewMetricsBuilder(scope)
	return builder.BuildMetrics(features)
}