// Package planattributeextractor provides plan dictionary and regression tracking
package planattributeextractor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PlanDictionary stores execution plans and tracks changes
type PlanDictionary struct {
	plans           map[string]*ExecutionPlan   // key: plan_id
	plansByQuery    map[string][]string         // key: query_id, value: plan_ids
	regressions     map[string]*PlanRegression  // key: regression_id
	mu              sync.RWMutex
	logger          *zap.Logger
	retentionPeriod time.Duration
	maxPlansPerQuery int
}

// ExecutionPlan represents a unique query execution plan
type ExecutionPlan struct {
	PlanID              string                 `json:"plan_id"`
	QueryID             string                 `json:"query_id"`
	PlanHash            string                 `json:"plan_hash"`
	DatabaseName        string                 `json:"database_name"`
	PlanJSON            json.RawMessage        `json:"plan_json"`
	PlanText            string                 `json:"plan_text"`
	TotalCost           float64                `json:"total_cost"`
	FirstSeen           time.Time              `json:"first_seen"`
	LastSeen            time.Time              `json:"last_seen"`
	ExecutionCount      int64                  `json:"execution_count"`
	
	// Performance metrics
	AvgDurationMs       float64                `json:"avg_duration_ms"`
	MinDurationMs       float64                `json:"min_duration_ms"`
	MaxDurationMs       float64                `json:"max_duration_ms"`
	StddevDurationMs    float64                `json:"stddev_duration_ms"`
	P95DurationMs       float64                `json:"p95_duration_ms"`
	P99DurationMs       float64                `json:"p99_duration_ms"`
	
	// Resource usage
	AvgRowsReturned     float64                `json:"avg_rows_returned"`
	AvgDiskReads        float64                `json:"avg_disk_reads"`
	AvgDiskWrites       float64                `json:"avg_disk_writes"`
	AvgTempSpaceKB      float64                `json:"avg_temp_space_kb"`
	
	// Plan characteristics
	NodeTypes           map[string]int         `json:"node_types"`
	JoinTypes           map[string]int         `json:"join_types"`
	IndexesUsed         []string               `json:"indexes_used"`
	TablesAccessed      []string               `json:"tables_accessed"`
	HasSeqScan          bool                   `json:"has_seq_scan"`
	HasNestedLoop       bool                   `json:"has_nested_loop"`
	HasHashJoin         bool                   `json:"has_hash_join"`
	HasSort             bool                   `json:"has_sort"`
	EstimatedRows       float64                `json:"estimated_rows"`
	ActualRows          float64                `json:"actual_rows"`
	
	// Plan quality metrics
	PlanningTimeMs      float64                `json:"planning_time_ms"`
	EstimationAccuracy  float64                `json:"estimation_accuracy"`
	CostAccuracy        float64                `json:"cost_accuracy"`
}

// PlanRegression represents a detected plan change with performance impact
type PlanRegression struct {
	RegressionID        string                 `json:"regression_id"`
	QueryID             string                 `json:"query_id"`
	DatabaseName        string                 `json:"database_name"`
	OldPlanID           string                 `json:"old_plan_id"`
	NewPlanID           string                 `json:"new_plan_id"`
	DetectedAt          time.Time              `json:"detected_at"`
	
	// Performance comparison
	OldAvgDurationMs    float64                `json:"old_avg_duration_ms"`
	NewAvgDurationMs    float64                `json:"new_avg_duration_ms"`
	PerformanceChangePct float64               `json:"performance_change_pct"`
	
	// Resource usage comparison
	DiskReadsChange     float64                `json:"disk_reads_change"`
	TempSpaceChange     float64                `json:"temp_space_change"`
	
	// Regression details
	IsRegression        bool                   `json:"is_regression"`
	RegressionSeverity  string                 `json:"regression_severity"` // "minor", "moderate", "severe"
	PossibleCauses      []string               `json:"possible_causes"`
	Recommendations     []string               `json:"recommendations"`
	
	// Statistical confidence
	StatisticalConfidence float64              `json:"statistical_confidence"`
	SampleSize          int                    `json:"sample_size"`
}

// NewPlanDictionary creates a new plan dictionary
func NewPlanDictionary(logger *zap.Logger, retentionPeriod time.Duration, maxPlansPerQuery int) *PlanDictionary {
	return &PlanDictionary{
		plans:            make(map[string]*ExecutionPlan),
		plansByQuery:     make(map[string][]string),
		regressions:      make(map[string]*PlanRegression),
		logger:           logger,
		retentionPeriod:  retentionPeriod,
		maxPlansPerQuery: maxPlansPerQuery,
	}
}

// AddPlan adds or updates an execution plan
func (pd *PlanDictionary) AddPlan(plan *ExecutionPlan) (*PlanRegression, error) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	
	// Generate plan ID if not provided
	if plan.PlanID == "" {
		plan.PlanID = pd.generatePlanID(plan)
	}
	
	// Check if plan already exists
	if existingPlan, exists := pd.plans[plan.PlanID]; exists {
		// Update existing plan metrics
		pd.updatePlanMetrics(existingPlan, plan)
		return nil, nil
	}
	
	// New plan - check for regression
	var regression *PlanRegression
	if previousPlans := pd.plansByQuery[plan.QueryID]; len(previousPlans) > 0 {
		// Get the most recent plan for this query
		lastPlanID := previousPlans[len(previousPlans)-1]
		if lastPlan, exists := pd.plans[lastPlanID]; exists {
			regression = pd.detectRegression(lastPlan, plan)
			if regression != nil {
				pd.regressions[regression.RegressionID] = regression
			}
		}
	}
	
	// Add new plan
	pd.plans[plan.PlanID] = plan
	pd.plansByQuery[plan.QueryID] = append(pd.plansByQuery[plan.QueryID], plan.PlanID)
	
	// Enforce max plans per query
	if len(pd.plansByQuery[plan.QueryID]) > pd.maxPlansPerQuery {
		oldestPlanID := pd.plansByQuery[plan.QueryID][0]
		delete(pd.plans, oldestPlanID)
		pd.plansByQuery[plan.QueryID] = pd.plansByQuery[plan.QueryID][1:]
	}
	
	pd.logger.Info("Added new execution plan",
		zap.String("plan_id", plan.PlanID),
		zap.String("query_id", plan.QueryID),
		zap.Float64("total_cost", plan.TotalCost),
		zap.Bool("has_regression", regression != nil))
	
	return regression, nil
}

// generatePlanID creates a unique plan identifier
func (pd *PlanDictionary) generatePlanID(plan *ExecutionPlan) string {
	h := sha256.New()
	h.Write([]byte(plan.QueryID))
	h.Write([]byte(plan.PlanHash))
	h.Write([]byte(fmt.Sprintf("%.2f", plan.TotalCost)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// updatePlanMetrics updates metrics for an existing plan
func (pd *PlanDictionary) updatePlanMetrics(existing, new *ExecutionPlan) {
	existing.LastSeen = time.Now()
	existing.ExecutionCount++
	
	// Update performance metrics using exponential moving average
	alpha := 0.2 // Smoothing factor
	existing.AvgDurationMs = alpha*new.AvgDurationMs + (1-alpha)*existing.AvgDurationMs
	existing.MinDurationMs = min(existing.MinDurationMs, new.MinDurationMs)
	existing.MaxDurationMs = max(existing.MaxDurationMs, new.MaxDurationMs)
	
	// Update resource usage
	existing.AvgRowsReturned = alpha*new.AvgRowsReturned + (1-alpha)*existing.AvgRowsReturned
	existing.AvgDiskReads = alpha*new.AvgDiskReads + (1-alpha)*existing.AvgDiskReads
	existing.AvgDiskWrites = alpha*new.AvgDiskWrites + (1-alpha)*existing.AvgDiskWrites
	existing.AvgTempSpaceKB = alpha*new.AvgTempSpaceKB + (1-alpha)*existing.AvgTempSpaceKB
}

// detectRegression checks if a plan change represents a performance regression
func (pd *PlanDictionary) detectRegression(oldPlan, newPlan *ExecutionPlan) *PlanRegression {
	performanceChange := (newPlan.AvgDurationMs - oldPlan.AvgDurationMs) / oldPlan.AvgDurationMs
	
	// Only consider it a regression if performance degraded by more than 20%
	if performanceChange < 0.2 {
		return nil
	}
	
	regression := &PlanRegression{
		RegressionID:         pd.generateRegressionID(oldPlan, newPlan),
		QueryID:              newPlan.QueryID,
		DatabaseName:         newPlan.DatabaseName,
		OldPlanID:            oldPlan.PlanID,
		NewPlanID:            newPlan.PlanID,
		DetectedAt:           time.Now(),
		OldAvgDurationMs:     oldPlan.AvgDurationMs,
		NewAvgDurationMs:     newPlan.AvgDurationMs,
		PerformanceChangePct: performanceChange * 100,
		DiskReadsChange:      newPlan.AvgDiskReads - oldPlan.AvgDiskReads,
		TempSpaceChange:      newPlan.AvgTempSpaceKB - oldPlan.AvgTempSpaceKB,
		IsRegression:         true,
	}
	
	// Determine severity
	switch {
	case performanceChange >= 5.0:
		regression.RegressionSeverity = "severe"
	case performanceChange >= 1.0:
		regression.RegressionSeverity = "moderate"
	default:
		regression.RegressionSeverity = "minor"
	}
	
	// Analyze possible causes
	regression.PossibleCauses = pd.analyzePlanChangeCauses(oldPlan, newPlan)
	regression.Recommendations = pd.generateRecommendations(oldPlan, newPlan, regression)
	
	// Calculate statistical confidence (simplified)
	regression.StatisticalConfidence = pd.calculateConfidence(oldPlan, newPlan)
	regression.SampleSize = int(oldPlan.ExecutionCount + newPlan.ExecutionCount)
	
	return regression
}

// analyzePlanChangeCauses identifies why the plan changed
func (pd *PlanDictionary) analyzePlanChangeCauses(oldPlan, newPlan *ExecutionPlan) []string {
	causes := []string{}
	
	// Check for missing indexes
	if !oldPlan.HasSeqScan && newPlan.HasSeqScan {
		causes = append(causes, "Sequential scan introduced - possible missing index")
	}
	
	// Check for join method changes
	if oldPlan.HasHashJoin && !newPlan.HasHashJoin && newPlan.HasNestedLoop {
		causes = append(causes, "Hash join changed to nested loop - possible statistics issue")
	}
	
	// Check for estimation errors
	if newPlan.EstimationAccuracy < 0.5 {
		causes = append(causes, "Poor row estimation accuracy - statistics may be outdated")
	}
	
	// Check for temp space usage
	if newPlan.AvgTempSpaceKB > oldPlan.AvgTempSpaceKB*2 {
		causes = append(causes, "Significant increase in temp space usage - possible memory pressure")
	}
	
	// Check for disk I/O increase
	if newPlan.AvgDiskReads > oldPlan.AvgDiskReads*2 {
		causes = append(causes, "Significant increase in disk reads - possible cache issues")
	}
	
	if len(causes) == 0 {
		causes = append(causes, "Plan change detected but cause unclear - review plan details")
	}
	
	return causes
}

// generateRecommendations provides actionable recommendations
func (pd *PlanDictionary) generateRecommendations(oldPlan, newPlan *ExecutionPlan, regression *PlanRegression) []string {
	recommendations := []string{}
	
	// Generic recommendations
	recommendations = append(recommendations, "Review query execution plan details")
	recommendations = append(recommendations, "Check table and index statistics freshness")
	
	// Specific recommendations based on regression
	if regression.RegressionSeverity == "severe" {
		recommendations = append(recommendations, "URGENT: Consider forcing old plan using hints if available")
		recommendations = append(recommendations, "Run ANALYZE on affected tables immediately")
	}
	
	if newPlan.HasSeqScan && !oldPlan.HasSeqScan {
		recommendations = append(recommendations, "Create index to eliminate sequential scan")
	}
	
	if newPlan.AvgTempSpaceKB > 1024*10 { // > 10MB temp space
		recommendations = append(recommendations, "Increase work_mem to reduce temp file usage")
	}
	
	if newPlan.EstimationAccuracy < 0.5 {
		recommendations = append(recommendations, "Update table statistics with higher sampling rate")
		recommendations = append(recommendations, "Consider creating extended statistics on correlated columns")
	}
	
	return recommendations
}

// calculateConfidence calculates statistical confidence in the regression
func (pd *PlanDictionary) calculateConfidence(oldPlan, newPlan *ExecutionPlan) float64 {
	// Simplified confidence calculation based on sample size and variance
	minSamples := min(float64(oldPlan.ExecutionCount), float64(newPlan.ExecutionCount))
	
	if minSamples < 10 {
		return 0.5 // Low confidence with small sample
	} else if minSamples < 100 {
		return 0.75 // Medium confidence
	} else {
		return 0.95 // High confidence with large sample
	}
}

// GetPlan retrieves a plan by ID
func (pd *PlanDictionary) GetPlan(planID string) (*ExecutionPlan, bool) {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	
	plan, exists := pd.plans[planID]
	return plan, exists
}

// GetPlansForQuery retrieves all plans for a query
func (pd *PlanDictionary) GetPlansForQuery(queryID string) []*ExecutionPlan {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	
	planIDs := pd.plansByQuery[queryID]
	plans := make([]*ExecutionPlan, 0, len(planIDs))
	
	for _, planID := range planIDs {
		if plan, exists := pd.plans[planID]; exists {
			plans = append(plans, plan)
		}
	}
	
	return plans
}

// GetRecentRegressions retrieves recent plan regressions
func (pd *PlanDictionary) GetRecentRegressions(since time.Time) []*PlanRegression {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	
	regressions := []*PlanRegression{}
	for _, regression := range pd.regressions {
		if regression.DetectedAt.After(since) {
			regressions = append(regressions, regression)
		}
	}
	
	return regressions
}

// CleanupOldPlans removes plans older than retention period
func (pd *PlanDictionary) CleanupOldPlans() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	
	cutoff := time.Now().Add(-pd.retentionPeriod)
	removedCount := 0
	
	for planID, plan := range pd.plans {
		if plan.LastSeen.Before(cutoff) {
			delete(pd.plans, planID)
			removedCount++
			
			// Remove from query index
			if planIDs, exists := pd.plansByQuery[plan.QueryID]; exists {
				newPlanIDs := []string{}
				for _, id := range planIDs {
					if id != planID {
						newPlanIDs = append(newPlanIDs, id)
					}
				}
				pd.plansByQuery[plan.QueryID] = newPlanIDs
			}
		}
	}
	
	// Cleanup old regressions
	for regressionID, regression := range pd.regressions {
		if regression.DetectedAt.Before(cutoff) {
			delete(pd.regressions, regressionID)
		}
	}
	
	if removedCount > 0 {
		pd.logger.Info("Cleaned up old execution plans",
			zap.Int("removed_count", removedCount),
			zap.Int("remaining_plans", len(pd.plans)),
			zap.Int("remaining_regressions", len(pd.regressions)))
	}
}

// generateRegressionID creates a unique regression identifier
func (pd *PlanDictionary) generateRegressionID(oldPlan, newPlan *ExecutionPlan) string {
	h := sha256.New()
	h.Write([]byte(oldPlan.PlanID))
	h.Write([]byte(newPlan.PlanID))
	h.Write([]byte(time.Now().Format(time.RFC3339)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Helper functions
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}