package adaptivesampler

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
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

	// State management (in-memory only)
	deduplicationCache *lru.Cache[string, time.Time]
	ruleLimiters       map[string]*rateLimiter
	globalRateLimiter  *rateLimiter // Global rate limiter for MaxRecordsPerSecond
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

	// Initialize global rate limiter
	var globalLimiter *rateLimiter
	if cfg.MaxRecordsPerSecond > 0 {
		// Convert per-second to per-minute for consistency with existing rate limiter
		globalLimiter = &rateLimiter{
			maxPerMinute: cfg.MaxRecordsPerSecond * 60,
			windowStart:  time.Now(),
		}
	}

	processor := &adaptiveSampler{
		config:             cfg,
		logger:             logger,
		consumer:           consumer,
		deduplicationCache: cache,
		ruleLimiters:       limiters,
		globalRateLimiter:  globalLimiter,
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

	// Start background cleanup only (no state persistence)
	p.wg.Add(1)
	go p.periodicCleanup()

	// Sort rules by priority (highest first)
	sort.Slice(p.config.SamplingRules, func(i, j int) bool {
		return p.config.SamplingRules[i].Priority > p.config.SamplingRules[j].Priority
	})

	return nil
}

// Shutdown stops the processor
func (p *adaptiveSampler) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down adaptive sampler processor")

	close(p.shutdownChan)
	p.wg.Wait()

	// No state persistence needed for in-memory mode
	p.logger.Info("Adaptive sampler shutdown complete", 
		zap.Int64("total_sampled", p.sampledCount),
		zap.Int64("total_dropped", p.droppedCount),
		zap.Int64("total_duplicates", p.duplicateCount))

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

				// Check global rate limit first
				if p.globalRateLimiter != nil && !p.checkGlobalRateLimit() {
					if p.config.EnableDebugLogging {
						p.logger.Debug("Log record dropped due to global rate limit",
							zap.Int("max_records_per_second", p.config.MaxRecordsPerSecond))
					}
					p.droppedCount++
					continue
				}

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

// isDuplicate checks if a record is a duplicate based on hash with graceful degradation
func (p *adaptiveSampler) isDuplicate(record plog.LogRecord) bool {
	hashAttr, exists := record.Attributes().Get(p.config.Deduplication.HashAttribute)
	if !exists {
		// Log debug message if hash attribute is missing
		if p.config.EnableDebugLogging {
			p.logger.Debug("Hash attribute missing from record, deduplication disabled for this record",
				zap.String("hash_attribute", p.config.Deduplication.HashAttribute),
				zap.String("suggestion", "Check if planattributeextractor is enabled and hash generation is configured"))
		}
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
	for _, rule := range p.config.SamplingRules {
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

// conditionMatches evaluates a single condition with graceful handling of missing attributes
func (p *adaptiveSampler) conditionMatches(condition SamplingCondition, record plog.LogRecord) bool {
	attr, exists := record.Attributes().Get(condition.Attribute)
	if !exists {
		// Handle missing attributes gracefully
		if condition.Operator == "exists" {
			expectedExists, ok := condition.Value.(bool)
			if !ok {
				if p.config.EnableDebugLogging {
					p.logger.Debug("Invalid exists condition value", zap.String("attribute", condition.Attribute))
				}
				return false
			}
			return !expectedExists // If attribute doesn't exist and we expect it not to exist
		}
		
		// For missing attributes with other operators, log a warning and fail gracefully
		if p.config.EnableDebugLogging {
			p.logger.Debug("Attribute missing from record, condition fails",
				zap.String("attribute", condition.Attribute),
				zap.String("operator", condition.Operator),
				zap.String("suggestion", "Check if planattributeextractor is enabled and working"))
		}
		return false
	}

	if condition.Operator == "exists" {
		expectedExists, ok := condition.Value.(bool)
		if !ok {
			if p.config.EnableDebugLogging {
				p.logger.Debug("Invalid exists condition value", zap.String("attribute", condition.Attribute))
			}
			return false
		}
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

// checkGlobalRateLimit checks if the global rate limit allows this record
func (p *adaptiveSampler) checkGlobalRateLimit() bool {
	if p.globalRateLimiter == nil {
		return true // No global rate limiting configured
	}

	p.globalRateLimiter.mutex.Lock()
	defer p.globalRateLimiter.mutex.Unlock()

	now := time.Now()
	
	// For global rate limiting, use a rolling second window
	if now.Sub(p.globalRateLimiter.windowStart) >= time.Second {
		p.globalRateLimiter.count = 0
		p.globalRateLimiter.windowStart = now
	}

	// Check if we're under the limit (per second)
	maxPerSecond := p.globalRateLimiter.maxPerMinute / 60
	if p.globalRateLimiter.count >= maxPerSecond {
		return false
	}

	p.globalRateLimiter.count++
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

	// Generate cryptographically secure random number between 0 and 1
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		// Log the error and fail securely rather than using weak randomness
		p.logger.Error("Failed to generate secure random number for sampling", zap.Error(err))
		// Fail closed - reject the sample to maintain security
		return false
	}

	random := float64(n.Int64()) / float64(math.MaxInt64)
	return random < rate
}


// periodicCleanup cleans up expired cache entries
func (p *adaptiveSampler) periodicCleanup() {
	defer p.wg.Done()

	// Default to 60 seconds if cleanup interval is not set
	cleanupInterval := p.config.Deduplication.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 60 * time.Second
	}

	ticker := time.NewTicker(cleanupInterval)
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

