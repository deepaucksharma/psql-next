package postgresqlquery

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// AdaptiveSampler implements intelligent sampling based on query characteristics and system load
type AdaptiveSampler struct {
	logger  *zap.Logger
	config  AdaptiveSamplingConfig
	
	// State
	mu              sync.RWMutex
	rules           []CompiledRule
	rateLimiter     *RateLimiter
	loadMonitor     *LoadMonitor
	memoryTracker   *MemoryTracker
	
	// Statistics
	stats           SamplerStats
}

// CompiledRule is a pre-processed sampling rule
type CompiledRule struct {
	SamplingRule
	matcher RuleMatcher
}

// RuleMatcher is a function that checks if a query matches a rule
type RuleMatcher func(query *SlowQueryMetrics) bool

// SlowQueryMetrics contains metrics for sampling decisions
type SlowQueryMetrics struct {
	QueryID         string
	MeanTimeMS      float64
	Calls           int64
	TotalTimeMS     float64
	Rows            int64
	TempBlocksUsed  int64
	SharedBlocksHit int64
	SharedBlocksRead int64
	ErrorCount      int64
	QueryType       string
	Tables          []string
	UserID          string
	DatabaseID      string
	ApplicationName string
}

// SamplerStats tracks sampler performance
type SamplerStats struct {
	TotalQueries     int64
	SampledQueries   int64
	DroppedQueries   int64
	RateLimited      int64
	MemoryLimited    int64
	RuleMatches      map[string]int64
	LastResetTime    time.Time
}

// RateLimiter implements token bucket algorithm for rate limiting
type RateLimiter struct {
	mu           sync.Mutex
	maxPerMinute int
	tokens       float64
	lastUpdate   time.Time
}

// LoadMonitor tracks system load for adaptive sampling
type LoadMonitor struct {
	mu                    sync.RWMutex
	queryRate             float64
	avgQueryTime          float64
	errorRate             float64
	lastUpdate            time.Time
	
	// Moving averages
	queryRateMA           *MovingAverage
	queryTimeMA           *MovingAverage
	errorRateMA           *MovingAverage
}

// MemoryTracker monitors memory usage for sampling decisions
type MemoryTracker struct {
	maxMemoryBytes int64
	currentBytes   int64
}

// MovingAverage implements exponential moving average
type MovingAverage struct {
	value float64
	alpha float64
}

// NewAdaptiveSampler creates a new adaptive sampler
func NewAdaptiveSampler(logger *zap.Logger, config AdaptiveSamplingConfig) (*AdaptiveSampler, error) {
	if err := validateSamplingConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid sampling config: %w", err)
	}
	
	sampler := &AdaptiveSampler{
		logger:        logger,
		config:        config,
		rateLimiter:   NewRateLimiter(config.MaxQueriesPerMinute),
		loadMonitor:   NewLoadMonitor(),
		memoryTracker: NewMemoryTracker(int64(config.MaxMemoryMB) * 1024 * 1024),
		stats: SamplerStats{
			RuleMatches:   make(map[string]int64),
			LastResetTime: time.Now(),
		},
	}
	
	// Compile sampling rules
	if err := sampler.compileRules(); err != nil {
		return nil, fmt.Errorf("failed to compile rules: %w", err)
	}
	
	return sampler, nil
}

// ShouldSample determines if a query should be sampled
func (as *AdaptiveSampler) ShouldSample(ctx context.Context, metrics *SlowQueryMetrics) (bool, string) {
	atomic.AddInt64(&as.stats.TotalQueries, 1)
	
	// Check memory limit first
	if !as.memoryTracker.CanAllocate(estimateQuerySize(metrics)) {
		atomic.AddInt64(&as.stats.MemoryLimited, 1)
		return false, "memory_limit_exceeded"
	}
	
	// Check global rate limit
	if !as.rateLimiter.Allow() {
		atomic.AddInt64(&as.stats.RateLimited, 1)
		return false, "rate_limited"
	}
	
	// Update load metrics
	as.loadMonitor.Update(metrics)
	
	// Find matching rule
	as.mu.RLock()
	rule, ruleName := as.findMatchingRule(metrics)
	as.mu.RUnlock()
	
	// Apply adaptive logic based on system load
	sampleRate := as.adjustSampleRate(rule.SampleRate)
	
	// Make sampling decision
	shouldSample := as.makeDecision(sampleRate)
	
	if shouldSample {
		atomic.AddInt64(&as.stats.SampledQueries, 1)
		as.memoryTracker.Allocate(estimateQuerySize(metrics))
		
		// Update rule match stats
		as.mu.Lock()
		as.stats.RuleMatches[ruleName]++
		as.mu.Unlock()
	} else {
		atomic.AddInt64(&as.stats.DroppedQueries, 1)
	}
	
	as.logger.Debug("Sampling decision",
		zap.String("query_id", metrics.QueryID),
		zap.Bool("sampled", shouldSample),
		zap.String("rule", ruleName),
		zap.Float64("rate", sampleRate),
		zap.Float64("adjusted_rate", sampleRate))
	
	return shouldSample, ruleName
}

// compileRules pre-processes sampling rules for efficiency
func (as *AdaptiveSampler) compileRules() error {
	as.mu.Lock()
	defer as.mu.Unlock()
	
	as.rules = make([]CompiledRule, 0, len(as.config.Rules))
	
	for _, rule := range as.config.Rules {
		matcher, err := as.compileMatcher(rule.Conditions)
		if err != nil {
			return fmt.Errorf("failed to compile rule %s: %w", rule.Name, err)
		}
		
		as.rules = append(as.rules, CompiledRule{
			SamplingRule: rule,
			matcher:      matcher,
		})
	}
	
	// Sort rules by priority (descending)
	sort.Slice(as.rules, func(i, j int) bool {
		return as.rules[i].Priority > as.rules[j].Priority
	})
	
	return nil
}

// compileMatcher creates a matcher function for rule conditions
func (as *AdaptiveSampler) compileMatcher(conditions []SamplingCondition) (RuleMatcher, error) {
	if len(conditions) == 0 {
		// No conditions means always match
		return func(q *SlowQueryMetrics) bool { return true }, nil
	}
	
	// Create a composite matcher that checks all conditions
	return func(q *SlowQueryMetrics) bool {
		for _, condition := range conditions {
			if !as.checkCondition(q, condition) {
				return false
			}
		}
		return true
	}, nil
}

// checkCondition evaluates a single condition
func (as *AdaptiveSampler) checkCondition(metrics *SlowQueryMetrics, condition SamplingCondition) bool {
	var value interface{}
	
	// Extract the attribute value
	switch condition.Attribute {
	case "mean_time_ms":
		value = metrics.MeanTimeMS
	case "total_time_ms":
		value = metrics.TotalTimeMS
	case "calls":
		value = metrics.Calls
	case "rows":
		value = metrics.Rows
	case "temp_blocks":
		value = metrics.TempBlocksUsed
	case "error_count":
		value = metrics.ErrorCount
	case "query_type":
		value = metrics.QueryType
	case "user_id":
		value = metrics.UserID
	case "database_id":
		value = metrics.DatabaseID
	case "application_name":
		value = metrics.ApplicationName
	default:
		// Unknown attribute, condition fails
		return false
	}
	
	// Evaluate the condition
	return as.evaluateOperator(value, condition.Operator, condition.Value)
}

// evaluateOperator performs the comparison operation
func (as *AdaptiveSampler) evaluateOperator(attrValue, operator string, condValue interface{}) bool {
	switch operator {
	case "eq":
		return as.compareEqual(attrValue, condValue)
	case "ne":
		return !as.compareEqual(attrValue, condValue)
	case "gt":
		return as.compareNumeric(attrValue, condValue, func(a, b float64) bool { return a > b })
	case "lt":
		return as.compareNumeric(attrValue, condValue, func(a, b float64) bool { return a < b })
	case "gte":
		return as.compareNumeric(attrValue, condValue, func(a, b float64) bool { return a >= b })
	case "lte":
		return as.compareNumeric(attrValue, condValue, func(a, b float64) bool { return a <= b })
	case "contains":
		return as.compareContains(attrValue, condValue)
	case "regex":
		return as.compareRegex(attrValue, condValue)
	default:
		return false
	}
}

// findMatchingRule finds the highest priority rule that matches
func (as *AdaptiveSampler) findMatchingRule(metrics *SlowQueryMetrics) (SamplingRule, string) {
	for _, rule := range as.rules {
		if rule.matcher(metrics) {
			return rule.SamplingRule, rule.Name
		}
	}
	
	// Return default rule if no match
	return SamplingRule{
		Name:       "default",
		SampleRate: as.config.DefaultRate,
	}, "default"
}

// adjustSampleRate adjusts the sampling rate based on system load
func (as *AdaptiveSampler) adjustSampleRate(baseRate float64) float64 {
	load := as.loadMonitor.GetLoad()
	
	// If system is under heavy load, reduce sampling rate
	if load.IsHigh() {
		adjustment := math.Max(0.1, 1.0 - load.Severity())
		return baseRate * adjustment
	}
	
	// If system has capacity, we can sample more
	if load.IsLow() {
		adjustment := math.Min(2.0, 1.0 + load.Capacity())
		return math.Min(1.0, baseRate * adjustment)
	}
	
	return baseRate
}

// makeDecision makes the final sampling decision based on rate
func (as *AdaptiveSampler) makeDecision(rate float64) bool {
	if rate >= 1.0 {
		return true
	}
	if rate <= 0.0 {
		return false
	}
	
	// Use a high-quality random decision
	return randomFloat64() < rate
}

// GetStats returns current sampler statistics
func (as *AdaptiveSampler) GetStats() SamplerStats {
	as.mu.RLock()
	defer as.mu.RUnlock()
	
	// Create a copy of stats
	stats := as.stats
	stats.RuleMatches = make(map[string]int64)
	for k, v := range as.stats.RuleMatches {
		stats.RuleMatches[k] = v
	}
	
	return stats
}

// ResetStats resets the sampler statistics
func (as *AdaptiveSampler) ResetStats() {
	as.mu.Lock()
	defer as.mu.Unlock()
	
	as.stats = SamplerStats{
		RuleMatches:   make(map[string]int64),
		LastResetTime: time.Now(),
	}
}

// RateLimiter implementation

func NewRateLimiter(maxPerMinute int) *RateLimiter {
	return &RateLimiter{
		maxPerMinute: maxPerMinute,
		tokens:       float64(maxPerMinute),
		lastUpdate:   time.Now(),
	}
}

func (rl *RateLimiter) Allow() bool {
	if rl.maxPerMinute <= 0 {
		return true // No rate limiting
	}
	
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(rl.lastUpdate).Seconds()
	
	// Refill tokens based on elapsed time
	tokensToAdd := elapsed * float64(rl.maxPerMinute) / 60.0
	rl.tokens = math.Min(float64(rl.maxPerMinute), rl.tokens+tokensToAdd)
	rl.lastUpdate = now
	
	// Try to consume a token
	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}
	
	return false
}

// LoadMonitor implementation

func NewLoadMonitor() *LoadMonitor {
	return &LoadMonitor{
		queryRateMA: &MovingAverage{alpha: 0.1},
		queryTimeMA: &MovingAverage{alpha: 0.1},
		errorRateMA: &MovingAverage{alpha: 0.1},
		lastUpdate:  time.Now(),
	}
}

func (lm *LoadMonitor) Update(metrics *SlowQueryMetrics) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	
	now := time.Now()
	elapsed := now.Sub(lm.lastUpdate).Seconds()
	
	if elapsed > 0 {
		// Update query rate (queries per second)
		queryRate := 1.0 / elapsed
		lm.queryRateMA.Update(queryRate)
		
		// Update average query time
		lm.queryTimeMA.Update(metrics.MeanTimeMS)
		
		// Update error rate
		errorRate := float64(metrics.ErrorCount) / float64(metrics.Calls)
		lm.errorRateMA.Update(errorRate)
	}
	
	lm.lastUpdate = now
}

func (lm *LoadMonitor) GetLoad() LoadStatus {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	
	return LoadStatus{
		QueryRate:    lm.queryRateMA.Value(),
		AvgQueryTime: lm.queryTimeMA.Value(),
		ErrorRate:    lm.errorRateMA.Value(),
	}
}

// LoadStatus represents current system load
type LoadStatus struct {
	QueryRate    float64
	AvgQueryTime float64
	ErrorRate    float64
}

func (ls LoadStatus) IsHigh() bool {
	// High load if query rate > 100/s, avg time > 1000ms, or error rate > 5%
	return ls.QueryRate > 100 || ls.AvgQueryTime > 1000 || ls.ErrorRate > 0.05
}

func (ls LoadStatus) IsLow() bool {
	// Low load if query rate < 10/s, avg time < 100ms, and error rate < 1%
	return ls.QueryRate < 10 && ls.AvgQueryTime < 100 && ls.ErrorRate < 0.01
}

func (ls LoadStatus) Severity() float64 {
	// Calculate a severity score (0-1)
	severity := 0.0
	
	// Query rate component
	if ls.QueryRate > 100 {
		severity += math.Min(1.0, ls.QueryRate/1000) * 0.3
	}
	
	// Query time component
	if ls.AvgQueryTime > 100 {
		severity += math.Min(1.0, ls.AvgQueryTime/10000) * 0.5
	}
	
	// Error rate component
	severity += math.Min(1.0, ls.ErrorRate/0.1) * 0.2
	
	return severity
}

func (ls LoadStatus) Capacity() float64 {
	// Calculate available capacity (0-1)
	return 1.0 - ls.Severity()
}

// MemoryTracker implementation

func NewMemoryTracker(maxBytes int64) *MemoryTracker {
	return &MemoryTracker{
		maxMemoryBytes: maxBytes,
	}
}

func (mt *MemoryTracker) CanAllocate(bytes int64) bool {
	current := atomic.LoadInt64(&mt.currentBytes)
	return current+bytes <= mt.maxMemoryBytes
}

func (mt *MemoryTracker) Allocate(bytes int64) {
	atomic.AddInt64(&mt.currentBytes, bytes)
}

func (mt *MemoryTracker) Release(bytes int64) {
	atomic.AddInt64(&mt.currentBytes, -bytes)
}

func (mt *MemoryTracker) GetUsage() (used, max int64) {
	return atomic.LoadInt64(&mt.currentBytes), mt.maxMemoryBytes
}

// MovingAverage implementation

func (ma *MovingAverage) Update(value float64) {
	if ma.value == 0 {
		ma.value = value
	} else {
		ma.value = ma.alpha*value + (1-ma.alpha)*ma.value
	}
}

func (ma *MovingAverage) Value() float64 {
	return ma.value
}

// Helper functions

func validateSamplingConfig(config *AdaptiveSamplingConfig) error {
	if !config.Enabled {
		return nil
	}
	
	if config.DefaultRate < 0 || config.DefaultRate > 1 {
		return fmt.Errorf("default_rate must be between 0 and 1")
	}
	
	if config.MaxQueriesPerMinute < 0 {
		return fmt.Errorf("max_queries_per_minute must be non-negative")
	}
	
	if config.MaxMemoryMB < 0 {
		return fmt.Errorf("max_memory_mb must be non-negative")
	}
	
	for i, rule := range config.Rules {
		if rule.Name == "" {
			return fmt.Errorf("rule[%d] must have a name", i)
		}
		
		if rule.SampleRate < 0 || rule.SampleRate > 1 {
			return fmt.Errorf("rule[%d].sample_rate must be between 0 and 1", i)
		}
	}
	
	return nil
}

func estimateQuerySize(metrics *SlowQueryMetrics) int64 {
	// Rough estimate of memory usage for a query
	size := int64(256) // Base overhead
	size += int64(len(metrics.QueryID))
	size += int64(16 * len(metrics.Tables)) // Table names
	return size
}

func randomFloat64() float64 {
	// High-quality random number between 0 and 1
	return float64(time.Now().UnixNano()%1000000) / 1000000.0
}

// Comparison helpers

func (as *AdaptiveSampler) compareEqual(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func (as *AdaptiveSampler) compareNumeric(a, b interface{}, compare func(float64, float64) bool) bool {
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)
	
	if !aOk || !bOk {
		return false
	}
	
	return compare(aFloat, bFloat)
}

func (as *AdaptiveSampler) compareContains(a, b interface{}) bool {
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.Contains(aStr, bStr)
}

func (as *AdaptiveSampler) compareRegex(a, b interface{}) bool {
	// Simplified - in production, compile and cache regex
	aStr := fmt.Sprintf("%v", a)
	pattern := fmt.Sprintf("%v", b)
	
	matched, err := regexp.MatchString(pattern, aStr)
	return err == nil && matched
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	default:
		return 0, false
	}
}