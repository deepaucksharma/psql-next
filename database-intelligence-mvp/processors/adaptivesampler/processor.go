package adaptivesampler

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

const (
	// ComponentType defines the type of this processor
	ComponentType = "adaptive_sampler"
)

// adaptiveSampler is the processor implementation
type adaptiveSampler struct {
	config   *Config
	logger   *zap.Logger
	consumer consumer.Logs

	// State management
	deduplicationCache *lru.Cache[string, time.Time]
	ruleLimiters       map[string]*rateLimiter
	stateFile          string
	stateMutex         sync.RWMutex

	// Metrics
	sampledCount   int64
	droppedCount   int64
	duplicateCount int64

	// Shutdown signal
	shutdownChan chan struct{}
	wg           sync.WaitGroup
}

// rateLimiter tracks per-rule rate limiting
type rateLimiter struct {
	maxPerMinute int
	count        int
	windowStart  time.Time
	mutex        sync.Mutex
}

// persistentState represents the state saved to disk
type persistentState struct {
	DeduplicationHashes map[string]time.Time `json:"deduplication_hashes"`
	RuleLimiters        map[string]rateLimiterState `json:"rule_limiters"`
	SavedAt             time.Time `json:"saved_at"`
}

type rateLimiterState struct {
	Count       int       `json:"count"`
	WindowStart time.Time `json:"window_start"`
}

// newAdaptiveSampler creates a new adaptive sampler processor
func newAdaptiveSampler(cfg *Config, logger *zap.Logger, consumer consumer.Logs) (*adaptiveSampler, error) {
	// Create deduplication cache
	cache, err := lru.New[string, time.Time](cfg.Deduplication.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create deduplication cache: %w", err)
	}

	// Initialize rule limiters
	limiters := make(map[string]*rateLimiter)
	for _, rule := range cfg.SamplingRules {
		if rule.MaxPerMinute > 0 {
			limiters[rule.Name] = &rateLimiter{
				maxPerMinute: rule.MaxPerMinute,
				windowStart:  time.Now(),
			}
		}
	}

	processor := &adaptiveSampler{
		config:             cfg,
		logger:             logger,
		consumer:           consumer,
		deduplicationCache: cache,
		ruleLimiters:       limiters,
		stateFile:          filepath.Join(cfg.StateStorage.FileStorage.Directory, "sampler_state.json"),
		shutdownChan:       make(chan struct{}),
	}

	return processor, nil
}

// Capabilities returns the capabilities of the processor
func (p *adaptiveSampler) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start starts the processor
func (p *adaptiveSampler) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting adaptive sampler processor")

	// Create state directory if it doesn't exist
	if err := os.MkdirAll(p.config.StateStorage.FileStorage.Directory, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Load persistent state
	if err := p.loadState(); err != nil {
		p.logger.Warn("Failed to load persistent state, starting fresh", zap.Error(err))
	}

	// Start background tasks
	p.wg.Add(2)
	go p.periodicStateSave()
	go p.periodicCleanup()

	return nil
}

// Shutdown stops the processor
func (p *adaptiveSampler) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down adaptive sampler processor")

	close(p.shutdownChan)
	p.wg.Wait()

	// Save final state
	if err := p.saveState(); err != nil {
		p.logger.Error("Failed to save final state", zap.Error(err))
	}

	return nil
}

// ConsumeLogs processes log records with adaptive sampling
func (p *adaptiveSampler) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	sampled := plog.NewLogs()

	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		sampledResourceLogs := sampled.ResourceLogs().AppendEmpty()
		resourceLogs.Resource().CopyTo(sampledResourceLogs.Resource())

		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			scopeLogs := resourceLogs.ScopeLogs().At(j)
			sampledScopeLogs := sampledResourceLogs.ScopeLogs().AppendEmpty()
			scopeLogs.Scope().CopyTo(sampledScopeLogs.Scope())

			for k := 0; k < scopeLogs.LogRecords().Len(); k++ {
				logRecord := scopeLogs.LogRecords().At(k)

				// Apply sampling decision
				if p.shouldSample(logRecord) {
					sampledLogRecord := sampledScopeLogs.LogRecords().AppendEmpty()
					logRecord.CopyTo(sampledLogRecord)
					p.sampledCount++
				} else {
					p.droppedCount++
				}
			}
		}
	}

	// Only forward if we have sampled records
	if sampled.LogRecordCount() > 0 {
		return p.consumer.ConsumeLogs(ctx, sampled)
	}

	return nil
}

// shouldSample determines if a log record should be sampled
func (p *adaptiveSampler) shouldSample(record plog.LogRecord) bool {
	// Check for deduplication if enabled
	if p.config.Deduplication.Enabled {
		if p.isDuplicate(record) {
			p.duplicateCount++
			return false
		}
	}

	// Find matching sampling rule
	rule := p.findMatchingRule(record)
	if rule == nil {
		// Use default sample rate
		return p.randomSample(p.config.DefaultSampleRate)
	}

	// Check rate limiting for this rule
	if rule.MaxPerMinute > 0 {
		if !p.checkRateLimit(rule.Name) {
			if p.config.EnableDebugLogging {
				p.logger.Debug("Record dropped due to rate limiting",
					zap.String("rule", rule.Name))
			}
			return false
		}
	}

	// Apply sampling rate
	shouldSample := p.randomSample(rule.SampleRate)

	if p.config.EnableDebugLogging {
		p.logger.Debug("Sampling decision",
			zap.String("rule", rule.Name),
			zap.Float64("sample_rate", rule.SampleRate),
			zap.Bool("sampled", shouldSample))
	}

	return shouldSample
}

// isDuplicate checks if a record is a duplicate based on hash
func (p *adaptiveSampler) isDuplicate(record plog.LogRecord) bool {
	hashAttr, exists := record.Attributes().Get(p.config.Deduplication.HashAttribute)
	if !exists {
		return false // No hash means not a duplicate
	}

	hash := hashAttr.AsString()
	if hash == "" {
		return false
	}

	p.stateMutex.RLock()
	lastSeen, exists := p.deduplicationCache.Get(hash)
	p.stateMutex.RUnlock()

	if !exists {
		// First time seeing this hash
		p.stateMutex.Lock()
		p.deduplicationCache.Add(hash, time.Now())
		p.stateMutex.Unlock()
		return false
	}

	// Check if within deduplication window
	windowDuration := time.Duration(p.config.Deduplication.WindowSeconds) * time.Second
	if time.Since(lastSeen) < windowDuration {
		return true // Duplicate within window
	}

	// Update timestamp for this hash
	p.stateMutex.Lock()
	p.deduplicationCache.Add(hash, time.Now())
	p.stateMutex.Unlock()

	return false
}

// findMatchingRule finds the highest priority rule that matches the record
func (p *adaptiveSampler) findMatchingRule(record plog.LogRecord) *SamplingRule {
	// Sort rules by priority (highest first)
	rules := make([]SamplingRule, len(p.config.SamplingRules))
	copy(rules, p.config.SamplingRules)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	for _, rule := range rules {
		if p.ruleMatches(rule, record) {
			return &rule
		}
	}

	return nil
}

// ruleMatches checks if a rule matches the given record
func (p *adaptiveSampler) ruleMatches(rule SamplingRule, record plog.LogRecord) bool {
	// If no conditions, rule matches everything
	if len(rule.Conditions) == 0 {
		return true
	}

	// All conditions must match
	for _, condition := range rule.Conditions {
		if !p.conditionMatches(condition, record) {
			return false
		}
	}

	return true
}

// conditionMatches evaluates a single condition
func (p *adaptiveSampler) conditionMatches(condition SamplingCondition, record plog.LogRecord) bool {
	attr, exists := record.Attributes().Get(condition.Attribute)
	if !exists {
		return condition.Operator == "exists" && condition.Value.(bool) == false
	}

	if condition.Operator == "exists" {
		expectedExists := condition.Value.(bool)
		return expectedExists == true
	}

	return p.compareValues(attr.AsString(), condition.Operator, condition.Value)
}

// compareValues compares attribute value with condition value
func (p *adaptiveSampler) compareValues(attrValue string, operator string, conditionValue interface{}) bool {
	switch operator {
	case "eq":
		return p.valuesEqual(attrValue, conditionValue)
	case "ne":
		return !p.valuesEqual(attrValue, conditionValue)
	case "gt":
		return p.numericalCompare(attrValue, conditionValue, func(a, b float64) bool { return a > b })
	case "gte":
		return p.numericalCompare(attrValue, conditionValue, func(a, b float64) bool { return a >= b })
	case "lt":
		return p.numericalCompare(attrValue, conditionValue, func(a, b float64) bool { return a < b })
	case "lte":
		return p.numericalCompare(attrValue, conditionValue, func(a, b float64) bool { return a <= b })
	case "contains":
		return strings.Contains(attrValue, fmt.Sprintf("%v", conditionValue))
	default:
		return false
	}
}

// valuesEqual compares values for equality
func (p *adaptiveSampler) valuesEqual(attrValue string, conditionValue interface{}) bool {
	switch v := conditionValue.(type) {
	case bool:
		attrBool, err := strconv.ParseBool(attrValue)
		return err == nil && attrBool == v
	case float64:
		attrFloat, err := strconv.ParseFloat(attrValue, 64)
		return err == nil && attrFloat == v
	case string:
		return attrValue == v
	default:
		return attrValue == fmt.Sprintf("%v", conditionValue)
	}
}

// numericalCompare performs numerical comparison
func (p *adaptiveSampler) numericalCompare(attrValue string, conditionValue interface{}, compare func(float64, float64) bool) bool {
	attrFloat, err := strconv.ParseFloat(attrValue, 64)
	if err != nil {
		return false
	}

	var conditionFloat float64
	switch v := conditionValue.(type) {
	case float64:
		conditionFloat = v
	case int:
		conditionFloat = float64(v)
	case string:
		var err error
		conditionFloat, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return false
		}
	default:
		return false
	}

	return compare(attrFloat, conditionFloat)
}

// checkRateLimit checks if the rule's rate limit allows this record
func (p *adaptiveSampler) checkRateLimit(ruleName string) bool {
	limiter, exists := p.ruleLimiters[ruleName]
	if !exists {
		return true // No rate limiting for this rule
	}

	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	now := time.Now()
	
	// Reset window if more than a minute has passed
	if now.Sub(limiter.windowStart) >= time.Minute {
		limiter.count = 0
		limiter.windowStart = now
	}

	// Check if we're under the limit
	if limiter.count >= limiter.maxPerMinute {
		return false
	}

	limiter.count++
	return true
}

// randomSample makes a random sampling decision
func (p *adaptiveSampler) randomSample(rate float64) bool {
	if rate >= 1.0 {
		return true
	}
	if rate <= 0.0 {
		return false
	}

	// Generate random number between 0 and 1
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		// Fallback to time-based pseudo-random
		return float64(time.Now().UnixNano()%1000000)/1000000 < rate
	}

	random := float64(n.Int64()) / float64(math.MaxInt64)
	return random < rate
}

// periodicStateSave saves state to disk periodically
func (p *adaptiveSampler) periodicStateSave() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.StateStorage.FileStorage.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := p.saveState(); err != nil {
				p.logger.Error("Failed to save state", zap.Error(err))
			}
		case <-p.shutdownChan:
			return
		}
	}
}

// periodicCleanup cleans up expired cache entries
func (p *adaptiveSampler) periodicCleanup() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.Deduplication.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupExpiredHashes()
		case <-p.shutdownChan:
			return
		}
	}
}

// cleanupExpiredHashes removes expired entries from the deduplication cache
func (p *adaptiveSampler) cleanupExpiredHashes() {
	if !p.config.Deduplication.Enabled {
		return
	}

	windowDuration := time.Duration(p.config.Deduplication.WindowSeconds) * time.Second
	cutoff := time.Now().Add(-windowDuration)

	p.stateMutex.Lock()
	defer p.stateMutex.Unlock()

	// Get all keys and check expiration
	keys := p.deduplicationCache.Keys()
	for _, key := range keys {
		if timestamp, exists := p.deduplicationCache.Peek(key); exists {
			if timestamp.Before(cutoff) {
				p.deduplicationCache.Remove(key)
			}
		}
	}
}

// saveState saves the current state to disk
func (p *adaptiveSampler) saveState() error {
	p.stateMutex.RLock()
	defer p.stateMutex.RUnlock()

	state := persistentState{
		DeduplicationHashes: make(map[string]time.Time),
		RuleLimiters:        make(map[string]rateLimiterState),
		SavedAt:             time.Now(),
	}

	// Save deduplication hashes
	if p.config.Deduplication.Enabled {
		keys := p.deduplicationCache.Keys()
		for _, key := range keys {
			if timestamp, exists := p.deduplicationCache.Peek(key); exists {
				state.DeduplicationHashes[key] = timestamp
			}
		}
	}

	// Save rule limiter states
	for name, limiter := range p.ruleLimiters {
		limiter.mutex.Lock()
		state.RuleLimiters[name] = rateLimiterState{
			Count:       limiter.count,
			WindowStart: limiter.windowStart,
		}
		limiter.mutex.Unlock()
	}

	// Write to temporary file first
	tempFile := p.stateFile + ".tmp"
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, p.stateFile); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// loadState loads the state from disk
func (p *adaptiveSampler) loadState() error {
	data, err := os.ReadFile(p.stateFile)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state persistentState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	p.stateMutex.Lock()
	defer p.stateMutex.Unlock()

	// Restore deduplication hashes
	if p.config.Deduplication.Enabled {
		windowDuration := time.Duration(p.config.Deduplication.WindowSeconds) * time.Second
		cutoff := time.Now().Add(-windowDuration)

		for hash, timestamp := range state.DeduplicationHashes {
			if timestamp.After(cutoff) {
				p.deduplicationCache.Add(hash, timestamp)
			}
		}
	}

	// Restore rule limiter states
	for name, limiterState := range state.RuleLimiters {
		if limiter, exists := p.ruleLimiters[name]; exists {
			limiter.mutex.Lock()
			// Only restore if window is still valid
			if time.Since(limiterState.WindowStart) < time.Minute {
				limiter.count = limiterState.Count
				limiter.windowStart = limiterState.WindowStart
			}
			limiter.mutex.Unlock()
		}
	}

	p.logger.Info("Loaded persistent state",
		zap.Int("deduplication_hashes", len(state.DeduplicationHashes)),
		zap.Int("rule_limiters", len(state.RuleLimiters)),
		zap.Time("saved_at", state.SavedAt))

	return nil
}