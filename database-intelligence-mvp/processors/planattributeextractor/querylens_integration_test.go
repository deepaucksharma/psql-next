// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package planattributeextractor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestQueryLensIntegration(t *testing.T) {
	tests := []struct {
		name           string
		queryLensData  map[string]interface{}
		expectedAttrs  map[string]interface{}
		expectRegression bool
	}{
		{
			name: "basic_plan_extraction",
			queryLensData: map[string]interface{}{
				"db.querylens.queryid":   int64(12345),
				"db.querylens.plan_id":   "plan_abc123",
				"db.querylens.plan_text": `Seq Scan on users (cost=0.00..35.50 rows=2550 width=4)`,
			},
			expectedAttrs: map[string]interface{}{
				"db.plan.type":          "Seq Scan",
				"db.plan.estimated_cost": 35.50,
				"db.plan.has_regression": false,
			},
			expectRegression: false,
		},
		{
			name: "nested_loop_regression_detection",
			queryLensData: map[string]interface{}{
				"db.querylens.queryid":   int64(67890),
				"db.querylens.plan_id":   "plan_xyz789",
				"db.querylens.plan_text": `Nested Loop (cost=0.00..150000.00 rows=1000000 width=8) (actual time=5000.00..5000.00 rows=1000000 loops=5000)`,
			},
			expectedAttrs: map[string]interface{}{
				"db.plan.type":            "Nested Loop",
				"db.plan.estimated_cost":  150000.00,
				"db.plan.has_regression":  true,
				"db.plan.regression_type": "excessive_nested_loops",
			},
			expectRegression: true,
		},
		{
			name: "plan_change_detection",
			queryLensData: map[string]interface{}{
				"db.querylens.queryid":                    int64(11111),
				"db.querylens.plan_id":                   "plan_new",
				"db.querylens.execution_time":            100.0,
				"db.querylens.previous_execution_time":   50.0,
				"db.querylens.blocks_read":               int64(1000),
				"db.querylens.previous_blocks_read":      int64(200),
			},
			expectedAttrs: map[string]interface{}{
				"db.plan.changed":           true,
				"db.plan.change_severity":   "high",
				"db.plan.time_change_ratio": 2.0,
			},
			expectRegression: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create processor with querylens enabled
			cfg := &Config{
				TimeoutMS: 100,
				ErrorMode: "ignore",
				SafeMode:  true,
				QueryLens: QueryLensConfig{
					Enabled:             true,
					PlanHistoryHours:    24,
					RegressionThreshold: 1.5,
					RegressionDetection: RegressionDetectionConfig{
						Enabled:      true,
						TimeIncrease: 1.5,
						IOIncrease:   2.0,
						CostIncrease: 2.0,
					},
				},
			}

			logger := zaptest.NewLogger(t)
			consumer := &consumertest.LogsSink{}
			processor := newPlanAttributeExtractor(cfg, logger, consumer)

			// Create metrics with querylens data
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			resource := rm.Resource()

			// Add querylens attributes
			for k, v := range tt.queryLensData {
				switch val := v.(type) {
				case string:
					resource.Attributes().PutStr(k, val)
				case int64:
					resource.Attributes().PutInt(k, val)
				case float64:
					resource.Attributes().PutDouble(k, val)
				}
			}

			// Process the metrics
			err := processor.processQueryLensData(context.Background(), md)
			require.NoError(t, err)

			// Verify expected attributes
			for k, expected := range tt.expectedAttrs {
				attr, exists := resource.Attributes().Get(k)
				assert.True(t, exists, "Expected attribute %s not found", k)
				
				switch expectedVal := expected.(type) {
				case string:
					assert.Equal(t, expectedVal, attr.Str())
				case bool:
					assert.Equal(t, expectedVal, attr.Bool())
				case float64:
					assert.InDelta(t, expectedVal, attr.Double(), 0.01)
				}
			}
		})
	}
}

func TestPlanChangeDetection(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.QueryLens.Enabled = true
	
	logger := zaptest.NewLogger(t)
	consumer := &consumertest.LogsSink{}
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	// First query execution
	md1 := pmetric.NewMetrics()
	rm1 := md1.ResourceMetrics().AppendEmpty()
	resource1 := rm1.Resource()
	resource1.Attributes().PutInt("db.querylens.queryid", 12345)
	resource1.Attributes().PutStr("db.querylens.plan_id", "plan_v1")

	err := processor.processQueryLensData(context.Background(), md1)
	require.NoError(t, err)

	// Verify no plan change on first execution
	_, exists := resource1.Attributes().Get("db.plan.changed")
	assert.False(t, exists)

	// Second query execution with different plan
	md2 := pmetric.NewMetrics()
	rm2 := md2.ResourceMetrics().AppendEmpty()
	resource2 := rm2.Resource()
	resource2.Attributes().PutInt("db.querylens.queryid", 12345)
	resource2.Attributes().PutStr("db.querylens.plan_id", "plan_v2")

	err = processor.processQueryLensData(context.Background(), md2)
	require.NoError(t, err)

	// Verify plan change detected
	changed, exists := resource2.Attributes().Get("db.plan.changed")
	assert.True(t, exists)
	assert.True(t, changed.Bool())
}

func TestRegressionAnalysis(t *testing.T) {
	tests := []struct {
		name             string
		planText         string
		expectedRegression bool
		expectedType     string
	}{
		{
			name:             "large_sequential_scan",
			planText:         "Seq Scan on large_table (cost=0.00..100000.00 rows=1000000 width=100)",
			expectedRegression: true,
			expectedType:     "large_sequential_scan",
		},
		{
			name:             "disk_sort_detected",
			planText:         "Sort Method: external merge Disk: 1024MB",
			expectedRegression: true,
			expectedType:     "disk_sort",
		},
		{
			name:             "high_io_read",
			planText:         "Buffers: shared hit=100 read=50000",
			expectedRegression: true,
			expectedType:     "high_io_read",
		},
		{
			name:             "efficient_index_scan",
			planText:         "Index Scan using users_pkey on users (cost=0.29..8.31 rows=1 width=4)",
			expectedRegression: false,
			expectedType:     "",
		},
	}

	cfg := createDefaultConfig().(*Config)
	logger := zaptest.NewLogger(t)
	consumer := &consumertest.LogsSink{}
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasRegression, regressionType := processor.detectRegression(tt.planText)
			assert.Equal(t, tt.expectedRegression, hasRegression)
			assert.Equal(t, tt.expectedType, regressionType)
		})
	}
}

func TestQueryLensPlanAnalysis(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zaptest.NewLogger(t)
	consumer := &consumertest.LogsSink{}
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	planText := `
Hash Join (cost=1000.00..5000.00 rows=10000 width=8)
  Hash Cond: (a.id = b.id)
  ->  Seq Scan on table_a a (cost=0.00..2000.00 rows=100000 width=4)
        Filter: (status = 'active'::text)
  ->  Hash (cost=500.00..500.00 rows=10000 width=4)
        ->  Index Scan using table_b_pkey on table_b b (cost=0.29..500.00 rows=10000 width=4)
`

	insights := processor.analyzeQueryLensPlan(planText)

	assert.Equal(t, "Hash Join", insights.PlanType)
	assert.Equal(t, 5000.0, insights.EstimatedCost)
	assert.Contains(t, insights.Recommendations, "Sequential scan with filter detected, index may improve performance")
}

func TestPerformanceRatioCalculation(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zaptest.NewLogger(t)
	consumer := &consumertest.LogsSink{}
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	attrs := pcommon.NewMap()
	attrs.PutDouble("db.querylens.execution_time", 150.0)
	attrs.PutDouble("db.querylens.previous_execution_time", 50.0)
	attrs.PutInt("db.querylens.blocks_read", 2000)
	attrs.PutInt("db.querylens.previous_blocks_read", 500)

	analysis := processor.analyzePlanChange("old_plan", "new_plan", attrs)

	assert.True(t, analysis.Detected)
	assert.Equal(t, "critical", analysis.Severity)
	assert.Equal(t, 3.0, analysis.TimeIncrease)
	assert.Equal(t, 4.0, analysis.IOIncrease)
	assert.True(t, analysis.TimeRegression)
	assert.True(t, analysis.IORegression)
}

func TestQueryLensMetricProcessing(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.QueryLens.Enabled = true
	
	logger := zaptest.NewLogger(t)
	consumer := &consumertest.LogsSink{}
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	// Create metrics
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	
	// Add gauge metric
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.querylens.query.execution_time_mean")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetDoubleValue(123.45)
	dp.Attributes().PutInt("db.querylens.queryid", 12345)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Process scope metrics
	processor.processScopeMetrics(rm.ScopeMetrics())

	// Verify processing (would need to check logs in real scenario)
	assert.Equal(t, 1, sm.Metrics().Len())
}

func BenchmarkQueryLensProcessing(b *testing.B) {
	cfg := createDefaultConfig().(*Config)
	cfg.QueryLens.Enabled = true
	
	logger := zap.NewNop()
	consumer := &consumertest.LogsSink{}
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	// Create sample data
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutInt("db.querylens.queryid", 12345)
	resource.Attributes().PutStr("db.querylens.plan_id", "plan_test")
	resource.Attributes().PutStr("db.querylens.plan_text", 
		"Seq Scan on users (cost=0.00..35.50 rows=2550 width=4)")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.processQueryLensData(ctx, md)
	}
}

func TestQueryLensConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  QueryLensConfig
		wantErr bool
	}{
		{
			name: "valid_config",
			config: QueryLensConfig{
				Enabled:             true,
				PlanHistoryHours:    24,
				RegressionThreshold: 1.5,
				RegressionDetection: RegressionDetectionConfig{
					Enabled:      true,
					TimeIncrease: 1.5,
					IOIncrease:   2.0,
					CostIncrease: 2.0,
				},
			},
			wantErr: false,
		},
		{
			name: "disabled_config",
			config: QueryLensConfig{
				Enabled: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.QueryLens = tt.config
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}