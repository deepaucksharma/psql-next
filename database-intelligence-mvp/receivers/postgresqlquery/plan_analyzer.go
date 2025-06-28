package postgresqlquery

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PlanAnalyzer handles query plan analysis and regression detection
type PlanAnalyzer struct {
	logger    *zap.Logger
	planCache *PlanCache
	mu        sync.RWMutex
	
	// Statistics
	stats PlanAnalyzerStats
}

// PlanAnalyzerStats tracks plan analysis statistics
type PlanAnalyzerStats struct {
	PlansAnalyzed       int64
	RegressionDetected  int64
	CacheHits           int64
	CacheMisses         int64
	ExplainErrors       int64
}

// QueryPlan represents a parsed execution plan
type QueryPlan struct {
	QueryID       string
	PlanHash      string
	PlanJSON      json.RawMessage
	TotalCost     float64
	ExecutionTime float64
	NodeCount     int
	JoinCount     int
	ScanTypes     map[string]int
	IndexesUsed   []string
	Parallelism   bool
	Timestamp     time.Time
}

// PlanNode represents a node in the execution plan tree
type PlanNode struct {
	NodeType         string          `json:"Node Type"`
	ParentRelation   string          `json:"Parent Relationship,omitempty"`
	JoinType         string          `json:"Join Type,omitempty"`
	IndexName        string          `json:"Index Name,omitempty"`
	RelationName     string          `json:"Relation Name,omitempty"`
	Alias            string          `json:"Alias,omitempty"`
	StartupCost      float64         `json:"Startup Cost"`
	TotalCost        float64         `json:"Total Cost"`
	PlanRows         float64         `json:"Plan Rows"`
	PlanWidth        int             `json:"Plan Width"`
	ActualStartupTime float64        `json:"Actual Startup Time,omitempty"`
	ActualTotalTime  float64         `json:"Actual Total Time,omitempty"`
	ActualRows       float64         `json:"Actual Rows,omitempty"`
	ActualLoops      int             `json:"Actual Loops,omitempty"`
	Workers          int             `json:"Workers,omitempty"`
	WorkersPlanned   int             `json:"Workers Planned,omitempty"`
	WorkersLaunched  int             `json:"Workers Launched,omitempty"`
	SharedHitBlocks  int64           `json:"Shared Hit Blocks,omitempty"`
	SharedReadBlocks int64           `json:"Shared Read Blocks,omitempty"`
	Plans            []PlanNode      `json:"Plans,omitempty"`
}

// PlanRegression represents a detected plan change
type PlanRegression struct {
	QueryID         string
	OldPlanHash     string
	NewPlanHash     string
	OldCost         float64
	NewCost         float64
	CostIncrease    float64
	DetectedAt      time.Time
	Severity        string // "minor", "moderate", "severe"
	ChangeDetails   []string
}

// NewPlanAnalyzer creates a new plan analyzer
func NewPlanAnalyzer(logger *zap.Logger, cacheSize int) *PlanAnalyzer {
	return &PlanAnalyzer{
		logger: logger,
		planCache: &PlanCache{
			cache: make(map[string]*PlanInfo),
		},
	}
}

// AnalyzePlan analyzes an execution plan and checks for regression
func (pa *PlanAnalyzer) AnalyzePlan(
	ctx context.Context,
	db *sql.DB,
	queryID string,
	queryText string,
	currentExecTime float64,
) (*QueryPlan, *PlanRegression, error) {
	
	// Prepare query for EXPLAIN
	preparedQuery, err := pa.prepareQueryForExplain(queryText)
	if err != nil {
		pa.mu.Lock()
		pa.stats.ExplainErrors++
		pa.mu.Unlock()
		return nil, nil, fmt.Errorf("failed to prepare query: %w", err)
	}
	
	// Get execution plan
	plan, err := pa.getExecutionPlan(ctx, db, preparedQuery)
	if err != nil {
		pa.mu.Lock()
		pa.stats.ExplainErrors++
		pa.mu.Unlock()
		return nil, nil, fmt.Errorf("failed to get execution plan: %w", err)
	}
	
	// Parse and analyze plan
	analyzedPlan, err := pa.parsePlan(queryID, plan)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse plan: %w", err)
	}
	
	analyzedPlan.ExecutionTime = currentExecTime
	
	// Check for regression
	regression := pa.checkRegression(queryID, analyzedPlan)
	
	// Update cache
	pa.updatePlanCache(queryID, analyzedPlan)
	
	// Update stats
	pa.mu.Lock()
	pa.stats.PlansAnalyzed++
	if regression != nil {
		pa.stats.RegressionDetected++
	}
	pa.mu.Unlock()
	
	return analyzedPlan, regression, nil
}

// prepareQueryForExplain prepares a query for EXPLAIN by handling parameters
func (pa *PlanAnalyzer) prepareQueryForExplain(queryText string) (string, error) {
	// Remove trailing semicolon if present
	queryText = strings.TrimSpace(queryText)
	queryText = strings.TrimSuffix(queryText, ";")
	
	// Replace parameter placeholders with sample values
	// This is a simplified approach - in production, use proper SQL parser
	prepared := queryText
	
	// Replace numbered parameters ($1, $2, etc.)
	paramRegex := regexp.MustCompile(`\$(\d+)`)
	prepared = paramRegex.ReplaceAllStringFunc(prepared, func(match string) string {
		// Replace with a safe default value
		return "1"
	})
	
	// Add LIMIT if not present to prevent large result sets
	lowerQuery := strings.ToLower(prepared)
	if !strings.Contains(lowerQuery, " limit ") {
		// Check if it's a SELECT query
		if strings.HasPrefix(strings.TrimSpace(lowerQuery), "select") {
			prepared += " LIMIT 1"
		}
	}
	
	// Ensure it's safe to explain
	if pa.isUnsafeToExplain(prepared) {
		return "", fmt.Errorf("query is unsafe to explain")
	}
	
	return prepared, nil
}

// isUnsafeToExplain checks if a query is unsafe to EXPLAIN
func (pa *PlanAnalyzer) isUnsafeToExplain(query string) bool {
	lowerQuery := strings.ToLower(query)
	
	// Don't explain DDL or utility commands
	unsafePatterns := []string{
		"create ", "alter ", "drop ", "truncate ",
		"grant ", "revoke ", "vacuum ", "analyze ",
		"copy ", "listen ", "notify ", "set ",
		"show ", "begin", "commit", "rollback",
		"savepoint", "prepare", "execute", "deallocate",
	}
	
	for _, pattern := range unsafePatterns {
		if strings.Contains(lowerQuery, pattern) {
			return true
		}
	}
	
	return false
}

// getExecutionPlan retrieves the execution plan for a query
func (pa *PlanAnalyzer) getExecutionPlan(ctx context.Context, db *sql.DB, query string) (json.RawMessage, error) {
	// Set timeout for EXPLAIN
	explainCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	// Use EXPLAIN with JSON format, without ANALYZE to avoid executing the query
	explainQuery := fmt.Sprintf("EXPLAIN (FORMAT JSON, BUFFERS true, VERBOSE true) %s", query)
	
	var planJSON json.RawMessage
	err := db.QueryRowContext(explainCtx, explainQuery).Scan(&planJSON)
	if err != nil {
		return nil, err
	}
	
	return planJSON, nil
}

// parsePlan parses the JSON execution plan
func (pa *PlanAnalyzer) parsePlan(queryID string, planJSON json.RawMessage) (*QueryPlan, error) {
	// Parse the plan array
	var planArray []struct {
		Plan PlanNode `json:"Plan"`
	}
	
	if err := json.Unmarshal(planJSON, &planArray); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan: %w", err)
	}
	
	if len(planArray) == 0 {
		return nil, fmt.Errorf("empty plan array")
	}
	
	rootNode := planArray[0].Plan
	
	// Analyze the plan tree
	analysis := &QueryPlan{
		QueryID:     queryID,
		PlanJSON:    planJSON,
		TotalCost:   rootNode.TotalCost,
		NodeCount:   0,
		JoinCount:   0,
		ScanTypes:   make(map[string]int),
		IndexesUsed: []string{},
		Parallelism: rootNode.WorkersPlanned > 0,
		Timestamp:   time.Now(),
	}
	
	// Traverse the plan tree
	pa.analyzePlanNode(&rootNode, analysis)
	
	// Generate plan hash
	analysis.PlanHash = pa.generatePlanHash(&rootNode)
	
	return analysis, nil
}

// analyzePlanNode recursively analyzes plan nodes
func (pa *PlanAnalyzer) analyzePlanNode(node *PlanNode, analysis *QueryPlan) {
	analysis.NodeCount++
	
	// Count node types
	nodeType := node.NodeType
	analysis.ScanTypes[nodeType]++
	
	// Track joins
	if node.JoinType != "" {
		analysis.JoinCount++
	}
	
	// Track index usage
	if node.IndexName != "" && !contains(analysis.IndexesUsed, node.IndexName) {
		analysis.IndexesUsed = append(analysis.IndexesUsed, node.IndexName)
	}
	
	// Recursively analyze child nodes
	for i := range node.Plans {
		pa.analyzePlanNode(&node.Plans[i], analysis)
	}
}

// generatePlanHash generates a stable hash for plan structure
func (pa *PlanAnalyzer) generatePlanHash(node *PlanNode) string {
	h := md5.New()
	pa.hashPlanNode(h, node)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// hashPlanNode recursively hashes plan nodes
func (pa *PlanAnalyzer) hashPlanNode(h hash.Hash, node *PlanNode) {
	// Include node type
	h.Write([]byte(node.NodeType))
	
	// Include join type if present
	if node.JoinType != "" {
		h.Write([]byte(node.JoinType))
	}
	
	// Include index name if present
	if node.IndexName != "" {
		h.Write([]byte(node.IndexName))
	}
	
	// Include relation name if present
	if node.RelationName != "" {
		h.Write([]byte(node.RelationName))
	}
	
	// Include parallelism info
	if node.WorkersPlanned > 0 {
		h.Write([]byte(fmt.Sprintf("parallel:%d", node.WorkersPlanned)))
	}
	
	// Recursively hash child nodes
	for i := range node.Plans {
		pa.hashPlanNode(h, &node.Plans[i])
	}
}

// checkRegression checks if the plan has regressed
func (pa *PlanAnalyzer) checkRegression(queryID string, newPlan *QueryPlan) *PlanRegression {
	pa.planCache.mu.RLock()
	oldPlanInfo, exists := pa.planCache.cache[queryID]
	pa.planCache.mu.RUnlock()
	
	if !exists {
		// First time seeing this query
		pa.mu.Lock()
		pa.stats.CacheMisses++
		pa.mu.Unlock()
		return nil
	}
	
	pa.mu.Lock()
	pa.stats.CacheHits++
	pa.mu.Unlock()
	
	// Check if plan hash changed
	if oldPlanInfo.Hash == newPlan.PlanHash {
		// Same plan, update last seen time
		pa.planCache.mu.Lock()
		oldPlanInfo.LastSeen = time.Now()
		pa.planCache.mu.Unlock()
		return nil
	}
	
	// Plan changed - analyze the regression
	regression := &PlanRegression{
		QueryID:      queryID,
		OldPlanHash:  oldPlanInfo.Hash,
		NewPlanHash:  newPlan.PlanHash,
		OldCost:      oldPlanInfo.Cost,
		NewCost:      newPlan.TotalCost,
		CostIncrease: (newPlan.TotalCost - oldPlanInfo.Cost) / oldPlanInfo.Cost * 100,
		DetectedAt:   time.Now(),
		ChangeDetails: []string{},
	}
	
	// Determine severity
	if regression.CostIncrease > 100 {
		regression.Severity = "severe"
	} else if regression.CostIncrease > 50 {
		regression.Severity = "moderate"
	} else {
		regression.Severity = "minor"
	}
	
	// Analyze what changed
	regression.ChangeDetails = pa.analyzeChanges(oldPlanInfo, newPlan)
	
	// Update change count
	pa.planCache.mu.Lock()
	oldPlanInfo.ChangeCount++
	pa.planCache.mu.Unlock()
	
	pa.logger.Warn("Plan regression detected",
		zap.String("query_id", queryID),
		zap.String("severity", regression.Severity),
		zap.Float64("cost_increase", regression.CostIncrease),
		zap.Strings("changes", regression.ChangeDetails))
	
	return regression
}

// analyzeChanges analyzes what changed between plans
func (pa *PlanAnalyzer) analyzeChanges(oldPlan *PlanInfo, newPlan *QueryPlan) []string {
	changes := []string{}
	
	// Cost change
	if newPlan.TotalCost > oldPlan.Cost*1.1 {
		changes = append(changes, fmt.Sprintf("Cost increased from %.2f to %.2f", 
			oldPlan.Cost, newPlan.TotalCost))
	}
	
	// Node count change
	if newPlan.NodeCount > oldPlan.NodeCount {
		changes = append(changes, fmt.Sprintf("Plan complexity increased (%d -> %d nodes)", 
			oldPlan.NodeCount, newPlan.NodeCount))
	}
	
	// Other changes would require storing more details in PlanInfo
	// This is a simplified version
	
	return changes
}

// updatePlanCache updates the plan cache with new plan info
func (pa *PlanAnalyzer) updatePlanCache(queryID string, plan *QueryPlan) {
	pa.planCache.mu.Lock()
	defer pa.planCache.mu.Unlock()
	
	pa.planCache.cache[queryID] = &PlanInfo{
		Hash:        plan.PlanHash,
		Cost:        plan.TotalCost,
		Timestamp:   plan.Timestamp,
		NodeCount:   plan.NodeCount,
		LastSeen:    time.Now(),
		ChangeCount: 0,
	}
}

// GetStats returns plan analyzer statistics
func (pa *PlanAnalyzer) GetStats() PlanAnalyzerStats {
	pa.mu.RLock()
	defer pa.mu.RUnlock()
	return pa.stats
}

// CleanupCache removes old entries from the plan cache
func (pa *PlanAnalyzer) CleanupCache(maxAge time.Duration) int {
	pa.planCache.mu.Lock()
	defer pa.planCache.mu.Unlock()
	
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	
	for queryID, planInfo := range pa.planCache.cache {
		if planInfo.LastSeen.Before(cutoff) {
			delete(pa.planCache.cache, queryID)
			removed++
		}
	}
	
	if removed > 0 {
		pa.logger.Debug("Cleaned up plan cache",
			zap.Int("removed", removed),
			zap.Int("remaining", len(pa.planCache.cache)))
	}
	
	return removed
}

// ExportCache exports the plan cache for persistence
func (pa *PlanAnalyzer) ExportCache() map[string]PlanInfo {
	pa.planCache.mu.RLock()
	defer pa.planCache.mu.RUnlock()
	
	export := make(map[string]PlanInfo)
	for k, v := range pa.planCache.cache {
		export[k] = *v
	}
	
	return export
}

// ImportCache imports a previously exported cache
func (pa *PlanAnalyzer) ImportCache(cache map[string]PlanInfo) {
	pa.planCache.mu.Lock()
	defer pa.planCache.mu.Unlock()
	
	for k, v := range cache {
		// Only import recent entries
		if time.Since(v.LastSeen) < 24*time.Hour {
			pa.planCache.cache[k] = &v
		}
	}
	
	pa.logger.Info("Imported plan cache",
		zap.Int("entries", len(pa.planCache.cache)))
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}