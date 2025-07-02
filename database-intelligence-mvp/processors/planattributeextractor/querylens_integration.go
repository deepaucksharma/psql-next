// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package planattributeextractor

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// QueryLensData represents data from pg_querylens extension
type QueryLensData struct {
	QueryID     int64       `json:"queryid"`
	PlanID      string      `json:"plan_id"`
	PlanText    string      `json:"plan_text"`
	PlanChanges []PlanChange `json:"plan_changes"`
	Metrics     PlanMetrics  `json:"metrics"`
}

// PlanChange represents a change in query execution plan
type PlanChange struct {
	Timestamp      time.Time `json:"timestamp"`
	OldPlanID      string    `json:"old_plan_id"`
	NewPlanID      string    `json:"new_plan_id"`
	CostChange     float64   `json:"cost_change"`
	PerformanceImpact string `json:"performance_impact"`
}

// PlanMetrics contains performance metrics for a plan
type PlanMetrics struct {
	MeanTime       float64 `json:"mean_time_ms"`
	TotalTime      float64 `json:"total_time_ms"`
	StdDevTime     float64 `json:"stddev_time_ms"`
	ExecutionCount int64   `json:"execution_count"`
	BlocksRead     int64   `json:"blocks_read"`
	BlocksHit      int64   `json:"blocks_hit"`
	TempBlocks     int64   `json:"temp_blocks"`
}

// PlanInsights contains analyzed insights from a query plan
type PlanInsights struct {
	PlanType       string  `json:"plan_type"`
	EstimatedCost  float64 `json:"estimated_cost"`
	ActualCost     float64 `json:"actual_cost"`
	HasRegression  bool    `json:"has_regression"`
	RegressionType string  `json:"regression_type"`
	Recommendations []string `json:"recommendations"`
}

// RegressionAnalysis contains results of regression detection
type RegressionAnalysis struct {
	Detected      bool    `json:"detected"`
	Severity      string  `json:"severity"`
	TimeIncrease  float64 `json:"time_increase_ratio"`
	IOIncrease    float64 `json:"io_increase_ratio"`
	CostIncrease  float64 `json:"cost_increase_ratio"`
	IORegression  bool    `json:"io_regression"`
	TimeRegression bool   `json:"time_regression"`
}

// processQueryLensData processes metrics with pg_querylens data
func (p *planAttributeExtractor) processQueryLensData(ctx context.Context, md pmetric.Metrics) error {
	resourceMetrics := md.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		rm := resourceMetrics.At(i)
		resource := rm.Resource()
		
		// Check if this is querylens data
		queryIDAttr, hasQueryID := resource.Attributes().Get("db.querylens.queryid")
		if !hasQueryID {
			continue
		}
		
		queryID := queryIDAttr.Int()
		
		// Get plan text if available
		planTextAttr, hasPlanText := resource.Attributes().Get("db.querylens.plan_text")
		if hasPlanText {
			planText := planTextAttr.Str()
			
			// Extract plan insights
			planInsights := p.analyzeQueryLensPlan(planText)
			
			// Add insights as attributes
			resource.Attributes().PutStr("db.plan.type", planInsights.PlanType)
			resource.Attributes().PutDouble("db.plan.estimated_cost", planInsights.EstimatedCost)
			resource.Attributes().PutBool("db.plan.has_regression", planInsights.HasRegression)
			
			if planInsights.HasRegression {
				resource.Attributes().PutStr("db.plan.regression_type", planInsights.RegressionType)
			}
			
			// Add recommendations if any
			if len(planInsights.Recommendations) > 0 {
				resource.Attributes().PutStr("db.plan.recommendations", strings.Join(planInsights.Recommendations, "; "))
			}
		}
		
		// Detect plan changes
		planIDAttr, hasPlanID := resource.Attributes().Get("db.querylens.plan_id")
		if hasPlanID {
			planID := planIDAttr.Str()
			if p.detectPlanChange(queryID, planID) {
				resource.Attributes().PutBool("db.plan.changed", true)
				
				// Get previous plan for comparison
				if prevPlan, exists := p.planHistory[queryID]; exists {
					regression := p.analyzePlanChange(prevPlan, planID, resource.Attributes())
					resource.Attributes().PutStr("db.plan.change_severity", regression.Severity)
					resource.Attributes().PutDouble("db.plan.time_change_ratio", regression.TimeIncrease)
				}
			}
			
			// Store current plan in history
			p.updatePlanHistory(queryID, planID)
		}
		
		// Process scope metrics for detailed analysis
		p.processScopeMetrics(rm.ScopeMetrics())
	}
	
	return nil
}

// analyzeQueryLensPlan extracts insights from a PostgreSQL execution plan
func (p *planAttributeExtractor) analyzeQueryLensPlan(planText string) PlanInsights {
	insights := PlanInsights{
		Recommendations: []string{},
	}
	
	// Extract plan type (Seq Scan, Index Scan, etc.)
	planTypeRegex := regexp.MustCompile(`(Seq Scan|Index Scan|Index Only Scan|Bitmap Heap Scan|Hash Join|Nested Loop|Merge Join|Hash|Sort|Aggregate|Limit)`)
	if matches := planTypeRegex.FindAllStringSubmatch(planText, -1); len(matches) > 0 {
		// Get the most significant operation
		insights.PlanType = matches[0][1]
		
		// Check for potentially problematic operations
		for _, match := range matches {
			operation := match[1]
			if operation == "Seq Scan" {
				insights.Recommendations = append(insights.Recommendations, 
					"Consider adding an index to avoid sequential scan")
			}
			if operation == "Nested Loop" && strings.Contains(planText, "loops=") {
				// Check for high loop counts
				loopRegex := regexp.MustCompile(`loops=(\d+)`)
				if loopMatches := loopRegex.FindStringSubmatch(planText); len(loopMatches) > 1 {
					loops, _ := strconv.Atoi(loopMatches[1])
					if loops > 1000 {
						insights.Recommendations = append(insights.Recommendations,
							fmt.Sprintf("High nested loop count (%d) detected, consider query optimization", loops))
					}
				}
			}
		}
	}
	
	// Extract estimated cost
	costRegex := regexp.MustCompile(`cost=(\d+\.?\d*)\.\.(\d+\.?\d*)`)
	if matches := costRegex.FindStringSubmatch(planText); len(matches) > 2 {
		insights.EstimatedCost, _ = strconv.ParseFloat(matches[2], 64)
	}
	
	// Extract actual time if available
	actualTimeRegex := regexp.MustCompile(`actual time=(\d+\.?\d*)\.\.(\d+\.?\d*)`)
	if matches := actualTimeRegex.FindStringSubmatch(planText); len(matches) > 2 {
		insights.ActualCost, _ = strconv.ParseFloat(matches[2], 64)
	}
	
	// Detect potential regressions
	insights.HasRegression, insights.RegressionType = p.detectRegression(planText)
	
	// Check for missing indexes
	if strings.Contains(planText, "Filter:") && strings.Contains(planText, "Seq Scan") {
		insights.Recommendations = append(insights.Recommendations,
			"Sequential scan with filter detected, index may improve performance")
	}
	
	// Check for sorts that might benefit from indexes
	if strings.Contains(planText, "Sort Method: external") {
		insights.Recommendations = append(insights.Recommendations,
			"External sort detected, consider increasing work_mem or adding index")
	}
	
	return insights
}

// detectRegression analyzes plan text for performance regression indicators
func (p *planAttributeExtractor) detectRegression(planText string) (bool, string) {
	regressionIndicators := []struct {
		pattern string
		regressionType string
	}{
		{`Seq Scan.*rows=\d{6,}`, "large_sequential_scan"},
		{`Nested Loop.*loops=\d{4,}`, "excessive_nested_loops"},
		{`Sort Method: external`, "disk_sort"},
		{`Hash Batches: [2-9]\d*|Hash Batches: \d{2,}`, "hash_batches"},
		{`Parallel workers planned: 0`, "missing_parallelism"},
		{`Buffers:.*read=\d{5,}`, "high_io_read"},
		{`Temp Written: \d+`, "temp_file_usage"},
	}
	
	for _, indicator := range regressionIndicators {
		if matched, _ := regexp.MatchString(indicator.pattern, planText); matched {
			return true, indicator.regressionType
		}
	}
	
	return false, ""
}

// detectPlanChange checks if the plan has changed for a query
func (p *planAttributeExtractor) detectPlanChange(queryID int64, currentPlanID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.planHistory == nil {
		p.planHistory = make(map[int64]string)
	}
	
	previousPlanID, exists := p.planHistory[queryID]
	return exists && previousPlanID != currentPlanID
}

// updatePlanHistory updates the plan history for a query
func (p *planAttributeExtractor) updatePlanHistory(queryID int64, planID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.planHistory == nil {
		p.planHistory = make(map[int64]string)
	}
	
	p.planHistory[queryID] = planID
}

// analyzePlanChange compares old and new plans to determine regression severity
func (p *planAttributeExtractor) analyzePlanChange(oldPlanID, newPlanID string, attrs pcommon.Map) RegressionAnalysis {
	analysis := RegressionAnalysis{
		Detected: true,
		Severity: "low",
	}
	
	// Get performance metrics from attributes
	if execTime, ok := attrs.Get("db.querylens.execution_time"); ok {
		if prevExecTime, ok := attrs.Get("db.querylens.previous_execution_time"); ok {
			current := execTime.Double()
			previous := prevExecTime.Double()
			if previous > 0 {
				analysis.TimeIncrease = current / previous
				if analysis.TimeIncrease > 2.0 {
					analysis.Severity = "critical"
					analysis.TimeRegression = true
				} else if analysis.TimeIncrease > 1.5 {
					analysis.Severity = "high"
					analysis.TimeRegression = true
				} else if analysis.TimeIncrease > 1.2 {
					analysis.Severity = "medium"
				}
			}
		}
	}
	
	// Check I/O metrics
	if blocksRead, ok := attrs.Get("db.querylens.blocks_read"); ok {
		if prevBlocksRead, ok := attrs.Get("db.querylens.previous_blocks_read"); ok {
			current := float64(blocksRead.Int())
			previous := float64(prevBlocksRead.Int())
			if previous > 0 {
				analysis.IOIncrease = current / previous
				if analysis.IOIncrease > 2.0 {
					analysis.IORegression = true
					if analysis.Severity == "low" {
						analysis.Severity = "medium"
					}
				}
			}
		}
	}
	
	return analysis
}

// processScopeMetrics processes the metrics within each scope
func (p *planAttributeExtractor) processScopeMetrics(scopeMetrics pmetric.ScopeMetricsSlice) {
	for i := 0; i < scopeMetrics.Len(); i++ {
		sm := scopeMetrics.At(i)
		metrics := sm.Metrics()
		
		for j := 0; j < metrics.Len(); j++ {
			metric := metrics.At(j)
			
			// Process different metric types
			switch metric.Type() {
			case pmetric.MetricTypeGauge:
				p.processGaugeMetric(metric.Gauge())
			case pmetric.MetricTypeSum:
				p.processSumMetric(metric.Sum())
			}
		}
	}
}

// processGaugeMetric processes gauge metrics
func (p *planAttributeExtractor) processGaugeMetric(gauge pmetric.Gauge) {
	dataPoints := gauge.DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)
		
		// Add query lens specific processing if needed
		if queryID, ok := dp.Attributes().Get("db.querylens.queryid"); ok {
			p.logger.Debug("Processing gauge metric for query", 
				zap.Int64("queryid", queryID.Int()),
				zap.Float64("value", dp.DoubleValue()))
		}
	}
}

// processSumMetric processes sum metrics
func (p *planAttributeExtractor) processSumMetric(sum pmetric.Sum) {
	dataPoints := sum.DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)
		
		// Add query lens specific processing if needed
		if queryID, ok := dp.Attributes().Get("db.querylens.queryid"); ok {
			p.logger.Debug("Processing sum metric for query",
				zap.Int64("queryid", queryID.Int()),
				zap.Int64("value", dp.IntValue()))
		}
	}
}

// QueryLensConfig contains configuration for pg_querylens integration
type QueryLensConfig struct {
	Enabled              bool                     `mapstructure:"enabled"`
	PlanHistoryHours     int                      `mapstructure:"plan_history_hours"`
	RegressionThreshold  float64                  `mapstructure:"regression_threshold"`
	RegressionDetection  RegressionDetectionConfig `mapstructure:"regression_detection"`
	AlertOnRegression    bool                     `mapstructure:"alert_on_regression"`
}

// RegressionDetectionConfig contains regression detection settings
type RegressionDetectionConfig struct {
	Enabled       bool    `mapstructure:"enabled"`
	TimeIncrease  float64 `mapstructure:"time_increase"`
	IOIncrease    float64 `mapstructure:"io_increase"`
	CostIncrease  float64 `mapstructure:"cost_increase"`
}