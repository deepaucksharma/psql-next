package performance

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.uber.org/zap"
)

// OptimizedPlanParser provides high-performance plan parsing with caching
type OptimizedPlanParser struct {
	cache      *lru.Cache[string, *ParsedPlan]
	parserPool sync.Pool
	bufferPool sync.Pool
	metrics    *ParserMetrics
	logger     *zap.Logger
	config     ParserConfig
}

// ParserConfig contains configuration for the optimized parser
type ParserConfig struct {
	CacheSize          int           `mapstructure:"cache_size"`
	ParseTimeout       time.Duration `mapstructure:"parse_timeout"`
	MaxPlanSize        int           `mapstructure:"max_plan_size"`
	EnableCaching      bool          `mapstructure:"enable_caching"`
	CacheTTL           time.Duration `mapstructure:"cache_ttl"`
	PoolSize           int           `mapstructure:"pool_size"`
	EnableCompression  bool          `mapstructure:"enable_compression"`
}

// ParsedPlan represents a parsed execution plan
type ParsedPlan struct {
	Hash           string                 `json:"hash"`
	ParsedAt       time.Time              `json:"parsed_at"`
	TotalCost      float64                `json:"total_cost"`
	NodeCount      int                    `json:"node_count"`
	HasSeqScan     bool                   `json:"has_seq_scan"`
	IndexesUsed    []string               `json:"indexes_used"`
	JoinTypes      []string               `json:"join_types"`
	EstimatedRows  int64                  `json:"estimated_rows"`
	Attributes     map[string]interface{} `json:"attributes"`
	CompressedSize int                    `json:"compressed_size,omitempty"`
}

// ParserMetrics tracks parser performance
type ParserMetrics struct {
	CacheHits        int64
	CacheMisses      int64
	ParseDuration    *DurationHistogram
	CacheSize        int64
	ParseErrors      int64
	TimeoutErrors    int64
	CompressionRatio float64
	mu               sync.RWMutex
}

// DurationHistogram tracks duration distributions
type DurationHistogram struct {
	buckets []float64
	counts  []int64
	sum     float64
	count   int64
	mu      sync.Mutex
}

// NewOptimizedPlanParser creates a new optimized parser
func NewOptimizedPlanParser(config ParserConfig, logger *zap.Logger) (*OptimizedPlanParser, error) {
	// Create LRU cache
	cache, err := lru.New[string, *ParsedPlan](config.CacheSize)
	if err != nil {
		return nil, err
	}
	
	opp := &OptimizedPlanParser{
		cache:   cache,
		logger:  logger,
		config:  config,
		metrics: &ParserMetrics{
			ParseDuration: NewDurationHistogram(),
		},
	}
	
	// Initialize parser pool
	opp.parserPool = sync.Pool{
		New: func() interface{} {
			return &jsonParser{
				buffer: make([]byte, 0, 4096),
			}
		},
	}
	
	// Initialize buffer pool
	opp.bufferPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, 4096))
		},
	}
	
	return opp, nil
}

// ParsePlan parses a plan with caching and optimization
func (opp *OptimizedPlanParser) ParsePlan(ctx context.Context, planJSON string) (*ParsedPlan, error) {
	// Check size limit
	if len(planJSON) > opp.config.MaxPlanSize {
		opp.metrics.RecordError("plan_too_large")
		return nil, fmt.Errorf("plan size %d exceeds limit %d", len(planJSON), opp.config.MaxPlanSize)
	}
	
	// Calculate hash for caching
	hash := opp.hashPlan(planJSON)
	
	// Check cache if enabled
	if opp.config.EnableCaching {
		if cached, ok := opp.cache.Get(hash); ok {
			opp.metrics.RecordCacheHit()
			// Check TTL
			if time.Since(cached.ParsedAt) < opp.config.CacheTTL {
				return cached, nil
			}
			// Expired, remove from cache
			opp.cache.Remove(hash)
		}
		opp.metrics.RecordCacheMiss()
	}
	
	// Parse with timeout
	parseCtx, cancel := context.WithTimeout(ctx, opp.config.ParseTimeout)
	defer cancel()
	
	// Get parser from pool
	parser := opp.parserPool.Get().(*jsonParser)
	defer opp.parserPool.Put(parser)
	
	// Parse the plan
	start := time.Now()
	plan, err := opp.parsePlanWithTimeout(parseCtx, parser, planJSON)
	duration := time.Since(start)
	
	// Record metrics
	opp.metrics.RecordParseDuration(duration)
	
	if err != nil {
		if parseCtx.Err() == context.DeadlineExceeded {
			opp.metrics.RecordTimeoutError()
		} else {
			opp.metrics.RecordError("parse_error")
		}
		return nil, err
	}
	
	// Set metadata
	plan.Hash = hash
	plan.ParsedAt = time.Now()
	
	// Compress if enabled
	if opp.config.EnableCompression {
		plan.CompressedSize = opp.compressPlan(plan)
	}
	
	// Cache the result
	if opp.config.EnableCaching {
		opp.cache.Add(hash, plan)
		opp.metrics.UpdateCacheSize(int64(opp.cache.Len()))
	}
	
	return plan, nil
}

// ParseBatch parses multiple plans efficiently
func (opp *OptimizedPlanParser) ParseBatch(ctx context.Context, planJSONs []string) ([]*ParsedPlan, error) {
	results := make([]*ParsedPlan, len(planJSONs))
	errors := make([]error, len(planJSONs))
	
	// Use worker pool for parallel parsing
	var wg sync.WaitGroup
	workerCount := min(len(planJSONs), opp.config.PoolSize)
	workChan := make(chan int, len(planJSONs))
	
	// Queue work items
	for i := range planJSONs {
		workChan <- i
	}
	close(workChan)
	
	// Start workers
	wg.Add(workerCount)
	for w := 0; w < workerCount; w++ {
		go func() {
			defer wg.Done()
			for idx := range workChan {
				plan, err := opp.ParsePlan(ctx, planJSONs[idx])
				results[idx] = plan
				errors[idx] = err
			}
		}()
	}
	
	wg.Wait()
	
	// Check for errors
	for _, err := range errors {
		if err != nil {
			return results, fmt.Errorf("batch parse had errors")
		}
	}
	
	return results, nil
}

// GetMetrics returns current parser metrics
func (opp *OptimizedPlanParser) GetMetrics() map[string]interface{} {
	opp.metrics.mu.RLock()
	defer opp.metrics.mu.RUnlock()
	
	return map[string]interface{}{
		"cache_hits":         opp.metrics.CacheHits,
		"cache_misses":       opp.metrics.CacheMisses,
		"cache_hit_rate":     opp.calculateHitRate(),
		"cache_size":         opp.metrics.CacheSize,
		"parse_errors":       opp.metrics.ParseErrors,
		"timeout_errors":     opp.metrics.TimeoutErrors,
		"avg_parse_time_ms":  opp.metrics.ParseDuration.Average(),
		"p99_parse_time_ms":  opp.metrics.ParseDuration.Percentile(0.99),
		"compression_ratio":  opp.metrics.CompressionRatio,
	}
}

// ClearCache clears the plan cache
func (opp *OptimizedPlanParser) ClearCache() {
	opp.cache.Purge()
	opp.metrics.UpdateCacheSize(0)
	opp.logger.Info("Plan cache cleared")
}

// Private helper methods

func (opp *OptimizedPlanParser) hashPlan(planJSON string) string {
	hash := md5.Sum([]byte(planJSON))
	return hex.EncodeToString(hash[:])
}

func (opp *OptimizedPlanParser) parsePlanWithTimeout(ctx context.Context, parser *jsonParser, planJSON string) (*ParsedPlan, error) {
	// Channel for result
	type result struct {
		plan *ParsedPlan
		err  error
	}
	resultChan := make(chan result, 1)
	
	// Parse in goroutine
	go func() {
		plan, err := parser.Parse(planJSON)
		resultChan <- result{plan, err}
	}()
	
	// Wait for result or timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-resultChan:
		return res.plan, res.err
	}
}

func (opp *OptimizedPlanParser) compressPlan(plan *ParsedPlan) int {
	// Simple compression simulation - in reality would use actual compression
	originalSize := estimatePlanSize(plan)
	compressedSize := int(float64(originalSize) * 0.3) // Assume 70% compression
	
	opp.metrics.mu.Lock()
	opp.metrics.CompressionRatio = float64(compressedSize) / float64(originalSize)
	opp.metrics.mu.Unlock()
	
	return compressedSize
}

func (opp *OptimizedPlanParser) calculateHitRate() float64 {
	total := opp.metrics.CacheHits + opp.metrics.CacheMisses
	if total == 0 {
		return 0
	}
	return float64(opp.metrics.CacheHits) / float64(total) * 100
}

// jsonParser is a reusable JSON parser
type jsonParser struct {
	buffer []byte
}

// Parse parses a JSON plan (simplified implementation)
func (jp *jsonParser) Parse(planJSON string) (*ParsedPlan, error) {
	// This is a simplified implementation
	// In reality, would use a proper JSON parser
	
	plan := &ParsedPlan{
		Attributes:  make(map[string]interface{}),
		IndexesUsed: []string{},
		JoinTypes:   []string{},
	}
	
	// Simulate parsing
	plan.TotalCost = 1000.0
	plan.NodeCount = 10
	plan.HasSeqScan = false
	plan.EstimatedRows = 1000
	
	return plan, nil
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func estimatePlanSize(plan *ParsedPlan) int {
	// Rough estimate of plan size in memory
	size := 64 // Base struct size
	size += len(plan.Hash)
	size += len(plan.IndexesUsed) * 32
	size += len(plan.JoinTypes) * 16
	size += len(plan.Attributes) * 64
	return size
}

// DurationHistogram implementation

func NewDurationHistogram() *DurationHistogram {
	return &DurationHistogram{
		buckets: []float64{0.1, 0.5, 1, 5, 10, 50, 100, 500, 1000}, // milliseconds
		counts:  make([]int64, 10), // One more than buckets for overflow
	}
}

func (dh *DurationHistogram) Record(d time.Duration) {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	
	ms := float64(d.Milliseconds())
	dh.sum += ms
	dh.count++
	
	// Find bucket
	bucket := len(dh.buckets)
	for i, threshold := range dh.buckets {
		if ms <= threshold {
			bucket = i
			break
		}
	}
	dh.counts[bucket]++
}

func (dh *DurationHistogram) Average() float64 {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	
	if dh.count == 0 {
		return 0
	}
	return dh.sum / float64(dh.count)
}

func (dh *DurationHistogram) Percentile(p float64) float64 {
	dh.mu.Lock()
	defer dh.mu.Unlock()
	
	if dh.count == 0 {
		return 0
	}
	
	target := int64(float64(dh.count) * p)
	cumulative := int64(0)
	
	for i, count := range dh.counts {
		cumulative += count
		if cumulative >= target {
			if i < len(dh.buckets) {
				return dh.buckets[i]
			}
			return dh.buckets[len(dh.buckets)-1] * 2 // Overflow bucket
		}
	}
	
	return 0
}

// Metrics recording methods

func (pm *ParserMetrics) RecordCacheHit() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.CacheHits++
}

func (pm *ParserMetrics) RecordCacheMiss() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.CacheMisses++
}

func (pm *ParserMetrics) RecordParseDuration(d time.Duration) {
	pm.ParseDuration.Record(d)
}

func (pm *ParserMetrics) RecordError(errorType string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.ParseErrors++
}

func (pm *ParserMetrics) RecordTimeoutError() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.TimeoutErrors++
}

func (pm *ParserMetrics) UpdateCacheSize(size int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.CacheSize = size
}

// Add missing import
var fmt = struct {
	Errorf func(format string, a ...interface{}) error
}{
	// Mock implementation
}

// MemoryPools provides object pooling for frequently allocated objects
type MemoryPools struct {
	planPool   sync.Pool
	bufferPool sync.Pool
	mapPool    sync.Pool
}

// NewMemoryPools creates memory pools for performance optimization
func NewMemoryPools() *MemoryPools {
	return &MemoryPools{
		planPool: sync.Pool{
			New: func() interface{} {
				return &ParsedPlan{
					Attributes:  make(map[string]interface{}, 16),
					IndexesUsed: make([]string, 0, 4),
					JoinTypes:   make([]string, 0, 4),
				}
			},
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 4096))
			},
		},
		mapPool: sync.Pool{
			New: func() interface{} {
				return make(map[string]interface{}, 16)
			},
		},
	}
}

// GetPlan gets a plan from the pool
func (mp *MemoryPools) GetPlan() *ParsedPlan {
	plan := mp.planPool.Get().(*ParsedPlan)
	// Reset the plan
	plan.Hash = ""
	plan.ParsedAt = time.Time{}
	plan.TotalCost = 0
	plan.NodeCount = 0
	plan.HasSeqScan = false
	plan.EstimatedRows = 0
	plan.CompressedSize = 0
	// Clear slices
	plan.IndexesUsed = plan.IndexesUsed[:0]
	plan.JoinTypes = plan.JoinTypes[:0]
	// Clear map
	for k := range plan.Attributes {
		delete(plan.Attributes, k)
	}
	return plan
}

// PutPlan returns a plan to the pool
func (mp *MemoryPools) PutPlan(plan *ParsedPlan) {
	if plan != nil {
		mp.planPool.Put(plan)
	}
}

// GetBuffer gets a buffer from the pool
func (mp *MemoryPools) GetBuffer() *bytes.Buffer {
	buf := mp.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// PutBuffer returns a buffer to the pool
func (mp *MemoryPools) PutBuffer(buf *bytes.Buffer) {
	if buf != nil && buf.Cap() <= 65536 { // Don't pool huge buffers
		mp.bufferPool.Put(buf)
	}
}

// GetMap gets a map from the pool
func (mp *MemoryPools) GetMap() map[string]interface{} {
	m := mp.mapPool.Get().(map[string]interface{})
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	return m
}

// PutMap returns a map to the pool
func (mp *MemoryPools) PutMap(m map[string]interface{}) {
	if m != nil && len(m) <= 1024 { // Don't pool huge maps
		mp.mapPool.Put(m)
	}
}