package circuitbreaker

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// State represents the circuit breaker state
type State int

const (
	// Closed - normal operation
	Closed State = iota
	// Open - circuit is open, rejecting requests
	Open
	// HalfOpen - testing if service has recovered
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// circuitBreakerProcessor implements the circuit breaker pattern for database safety
type circuitBreakerProcessor struct {
	config   *Config
	logger   *zap.Logger
	consumer consumer.Logs

	// Circuit breaker state
	state        State
	failureCount int
	successCount int
	lastFailure  time.Time
	stateMutex   sync.RWMutex

	// Per-database circuit breakers
	databaseStates map[string]*databaseCircuitState
	dbStatesMutex  sync.RWMutex

	// Concurrency control
	semaphore chan struct{}

	// Adaptive timeout
	currentTimeout time.Duration
	timeoutMutex   sync.RWMutex

	// Metrics
	totalRequests    int64
	failedRequests   int64
	rejectedRequests int64

	// New Relic integration tracking
	nrErrors           int64
	cardinalityWarnings int64

	// Performance tracking
	throughputMonitor  *ThroughputMonitor
	latencyTracker    *LatencyTracker
	errorClassifier   *ErrorClassifier
	memoryMonitor     *MemoryMonitor

	// Shutdown
	shutdownChan chan struct{}
	wg           sync.WaitGroup
}

// databaseCircuitState tracks circuit state per database
type databaseCircuitState struct {
	state        State
	failureCount int
	successCount int
	lastFailure  time.Time
	errorRate    float64
	avgDuration  time.Duration
	mutex        sync.RWMutex
}

// newCircuitBreakerProcessor creates a new circuit breaker processor
func newCircuitBreakerProcessor(cfg *Config, logger *zap.Logger, consumer consumer.Logs) *circuitBreakerProcessor {
	return &circuitBreakerProcessor{
		config:            cfg,
		logger:            logger,
		consumer:          consumer,
		state:             Closed,
		databaseStates:    make(map[string]*databaseCircuitState),
		semaphore:         make(chan struct{}, cfg.MaxConcurrentRequests),
		currentTimeout:    cfg.BaseTimeout,
		shutdownChan:      make(chan struct{}),
		throughputMonitor: NewThroughputMonitor(time.Minute),
		latencyTracker:    NewLatencyTracker(1000),
		errorClassifier:   NewErrorClassifier(),
		memoryMonitor:     NewMemoryMonitor(cfg.MemoryThresholdMB),
	}
}

// Capabilities returns the capabilities of the processor
func (p *circuitBreakerProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// Start starts the processor
func (p *circuitBreakerProcessor) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting circuit breaker processor",
		zap.Int("failure_threshold", p.config.FailureThreshold),
		zap.Int("success_threshold", p.config.SuccessThreshold),
		zap.Duration("open_state_timeout", p.config.OpenStateTimeout),
		zap.Int("max_concurrent_requests", p.config.MaxConcurrentRequests))

	// Start health monitoring
	p.wg.Add(1)
	go p.healthMonitor()

	return nil
}

// Shutdown stops the processor
func (p *circuitBreakerProcessor) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down circuit breaker processor")
	close(p.shutdownChan)
	p.wg.Wait()
	return nil
}

// ConsumeLogs processes logs through the circuit breaker
func (p *circuitBreakerProcessor) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	p.totalRequests++
	p.throughputMonitor.RecordRequest()

	// Check throughput limits
	if p.throughputMonitor.IsOverloaded() {
		p.rejectedRequests++
		p.logger.Warn("Throughput limit exceeded, rejecting request",
			zap.Float64("current_rate", p.throughputMonitor.GetRate()),
			zap.Int64("rejected_requests", p.rejectedRequests))
		return fmt.Errorf("throughput limit exceeded")
	}

	// Check memory pressure
	if p.memoryMonitor.IsUnderPressure() {
		p.rejectedRequests++
		p.logger.Warn("Memory pressure detected, rejecting request",
			zap.Float64("memory_usage_percent", p.memoryMonitor.GetUsagePercent()),
			zap.Int64("rejected_requests", p.rejectedRequests))
		return fmt.Errorf("memory pressure detected")
	}

	// Extract database information and check for New Relic errors
	databases := p.extractDatabaseInfo(logs)
	
	// Check global circuit state
	if !p.allowRequest() {
		p.rejectedRequests++
		p.logger.Warn("Global circuit breaker open, rejecting request",
			zap.String("state", p.getState().String()),
			zap.Int64("rejected_requests", p.rejectedRequests))
		return fmt.Errorf("circuit breaker open")
	}

	// Check per-database circuit states
	for _, dbName := range databases {
		if !p.allowDatabaseRequest(dbName) {
			p.rejectedRequests++
			p.logger.Warn("Database circuit breaker open, rejecting request",
				zap.String("database", dbName),
				zap.Int64("rejected_requests", p.rejectedRequests))
			
			// Remove logs for this database
			p.filterLogsForDatabase(logs, dbName)
		}
	}

	// If all logs were filtered out, return early
	if logs.LogRecordCount() == 0 {
		return nil
	}

	// Acquire semaphore for concurrency control
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Semaphore full, reject request
		p.onFailure(fmt.Errorf("max concurrent requests exceeded"))
		return fmt.Errorf("max concurrent requests exceeded")
	}

	// Create timeout context
	timeout := p.getCurrentTimeout()
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Process logs
	start := time.Now()
	err := p.consumer.ConsumeLogs(timeoutCtx, logs)
	duration := time.Since(start)

	// Record latency
	p.latencyTracker.RecordLatency(duration)

	// Handle result
	if err != nil {
		// Classify error
		errorType := p.errorClassifier.ClassifyError(err)
		
		p.onFailure(err)
		p.failedRequests++
		
		// Update per-database states
		for _, dbName := range databases {
			p.onDatabaseFailure(dbName, err, duration)
		}
		
		// Check for New Relic specific errors
		if p.isNewRelicError(err) {
			p.nrErrors++
			p.logger.Error("New Relic integration error detected",
				zap.Error(err),
				zap.String("error_type", errorType),
				zap.Int64("nr_errors", p.nrErrors))
		}
		
		// Adjust timeout if adaptive timeout is enabled
		if p.config.EnableAdaptiveTimeout {
			p.adjustTimeout(duration, false)
		}
		
		// Log error with classification
		p.logger.Error("Request failed",
			zap.Error(err),
			zap.String("error_type", errorType),
			zap.Duration("duration", duration),
			zap.Int64("failed_requests", p.failedRequests))
		
		return err
	}

	p.onSuccess()
	
	// Update per-database states
	for _, dbName := range databases {
		p.onDatabaseSuccess(dbName, duration)
	}
	
	// Adjust timeout if adaptive timeout is enabled
	if p.config.EnableAdaptiveTimeout {
		p.adjustTimeout(duration, true)
	}

	return nil
}

// allowRequest checks if the request should be allowed
func (p *circuitBreakerProcessor) allowRequest() bool {
	p.stateMutex.RLock()
	defer p.stateMutex.RUnlock()

	switch p.state {
	case Closed:
		return true
	case Open:
		// Check if we should transition to half-open
		if time.Since(p.lastFailure) > p.config.OpenStateTimeout {
			p.stateMutex.RUnlock()
			p.stateMutex.Lock()
			if p.state == Open && time.Since(p.lastFailure) > p.config.OpenStateTimeout {
				p.state = HalfOpen
				p.successCount = 0
				p.logger.Info("Circuit breaker transitioning to half-open")
			}
			p.stateMutex.Unlock()
			p.stateMutex.RLock()
			return p.state == HalfOpen
		}
		return false
	case HalfOpen:
		return true
	default:
		return false
	}
}

// onSuccess handles successful requests
func (p *circuitBreakerProcessor) onSuccess() {
	p.stateMutex.Lock()
	defer p.stateMutex.Unlock()

	switch p.state {
	case Closed:
		p.failureCount = 0
	case HalfOpen:
		p.successCount++
		if p.successCount >= p.config.SuccessThreshold {
			p.state = Closed
			p.failureCount = 0
			p.successCount = 0
			p.logger.Info("Circuit breaker closed after successful recovery")
		}
	}
}

// onFailure handles failed requests
func (p *circuitBreakerProcessor) onFailure(err error) {
	p.stateMutex.Lock()
	defer p.stateMutex.Unlock()

	p.failureCount++
	p.lastFailure = time.Now()

	switch p.state {
	case Closed:
		if p.failureCount >= p.config.FailureThreshold {
			p.state = Open
			p.logger.Error("Circuit breaker opened due to failures",
				zap.Int("failure_count", p.failureCount),
				zap.Int("threshold", p.config.FailureThreshold),
				zap.Error(err))
		}
	case HalfOpen:
		p.state = Open
		p.successCount = 0
		p.logger.Error("Circuit breaker reopened after failure in half-open state",
			zap.Error(err))
	}
}

// getState safely returns the current state
func (p *circuitBreakerProcessor) getState() State {
	p.stateMutex.RLock()
	defer p.stateMutex.RUnlock()
	return p.state
}

// getCurrentTimeout safely returns the current timeout
func (p *circuitBreakerProcessor) getCurrentTimeout() time.Duration {
	p.timeoutMutex.RLock()
	defer p.timeoutMutex.RUnlock()
	return p.currentTimeout
}

// adjustTimeout adjusts the timeout based on recent performance
func (p *circuitBreakerProcessor) adjustTimeout(duration time.Duration, success bool) {
	p.timeoutMutex.Lock()
	defer p.timeoutMutex.Unlock()

	if success {
		// Successful request - slightly decrease timeout if it was much faster
		if duration < p.currentTimeout/2 {
			newTimeout := p.currentTimeout * 9 / 10 // Decrease by 10%
			if newTimeout >= p.config.BaseTimeout {
				p.currentTimeout = newTimeout
			}
		}
	} else {
		// Failed request - increase timeout
		newTimeout := p.currentTimeout * 11 / 10 // Increase by 10%
		if newTimeout <= p.config.MaxTimeout {
			p.currentTimeout = newTimeout
		}
	}
}

// healthMonitor monitors system health and can open circuit based on resource usage
func (p *circuitBreakerProcessor) healthMonitor() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.checkSystemHealth()
		case <-p.shutdownChan:
			return
		}
	}
}

// checkSystemHealth checks system resources and opens circuit if thresholds exceeded
func (p *circuitBreakerProcessor) checkSystemHealth() {
	// Check memory usage
	if p.config.MemoryThresholdMB > 0 {
		memUsageMB := p.getMemoryUsageMB()
		if memUsageMB > p.config.MemoryThresholdMB {
			p.logger.Error("Memory threshold exceeded, opening circuit breaker",
				zap.Int("current_mb", memUsageMB),
				zap.Int("threshold_mb", p.config.MemoryThresholdMB))
			p.onFailure(fmt.Errorf("memory threshold exceeded: %dMB > %dMB", memUsageMB, p.config.MemoryThresholdMB))
		}
	}

	// Check CPU usage
	if p.config.CPUThresholdPercent > 0 {
		cpuPercent := p.getCPUUsagePercent()
		if cpuPercent > p.config.CPUThresholdPercent {
			p.logger.Error("CPU threshold exceeded, opening circuit breaker",
				zap.Float64("current_percent", cpuPercent),
				zap.Float64("threshold_percent", p.config.CPUThresholdPercent))
			p.onFailure(fmt.Errorf("CPU threshold exceeded: %.2f%% > %.2f%%", cpuPercent, p.config.CPUThresholdPercent))
		}
	}

	// Check New Relic error rate
	if p.nrErrors > 10 { // More than 10 NR errors
		p.logger.Error("High New Relic error rate detected",
			zap.Int64("nr_errors", p.nrErrors),
			zap.Int64("cardinality_warnings", p.cardinalityWarnings))
		
		// Open circuit if too many NR errors
		if p.nrErrors > 50 {
			p.onFailure(fmt.Errorf("excessive New Relic errors: %d", p.nrErrors))
		}
	}

	// Update memory usage (in production, get actual memory stats)
	// p.memoryMonitor.UpdateUsage(getCurrentMemoryUsage())

	// Get performance metrics
	p50, p95, p99 := p.latencyTracker.GetPercentiles()
	errorStats := p.errorClassifier.GetErrorStats()

	// Log comprehensive status
	state := p.getState()
	p.logger.Info("Circuit breaker health check",
		zap.String("state", state.String()),
		zap.Int("failure_count", p.failureCount),
		zap.Int("success_count", p.successCount),
		zap.Duration("current_timeout", p.getCurrentTimeout()),
		zap.Int64("nr_errors", p.nrErrors),
		zap.Float64("throughput_rate", p.throughputMonitor.GetRate()),
		zap.Float64("memory_usage_percent", p.memoryMonitor.GetUsagePercent()),
		zap.Duration("latency_p50", p50),
		zap.Duration("latency_p95", p95),
		zap.Duration("latency_p99", p99),
		zap.Any("error_stats", errorStats))
	
	// Log per-database states if any are not closed
	p.dbStatesMutex.RLock()
	for dbName, dbState := range p.databaseStates {
		dbState.mutex.RLock()
		if dbState.state != Closed {
			p.logger.Info("Database circuit breaker status",
				zap.String("database", dbName),
				zap.String("state", dbState.state.String()),
				zap.Int("failure_count", dbState.failureCount),
				zap.Float64("error_rate", dbState.errorRate),
				zap.Duration("avg_duration", dbState.avgDuration))
		}
		dbState.mutex.RUnlock()
	}
	p.dbStatesMutex.RUnlock()

	// Check if we should open circuit based on error patterns
	if errorStats["total"] > 100 {
		criticalErrors := errorStats["memory"] + errorStats["disk"] + errorStats["authentication"]
		if criticalErrors > 10 {
			p.logger.Error("Critical errors detected, opening circuit breaker",
				zap.Int64("critical_errors", criticalErrors),
				zap.Any("error_breakdown", errorStats))
			p.onFailure(fmt.Errorf("too many critical errors: %d", criticalErrors))
		}
	}
}

// getMemoryUsageMB returns current memory usage in MB
func (p *circuitBreakerProcessor) getMemoryUsageMB() int {
	// This is a simplified implementation
	// In production, you might want to use more sophisticated memory monitoring
	return 0 // Placeholder
}

// getCPUUsagePercent returns current CPU usage percentage
func (p *circuitBreakerProcessor) getCPUUsagePercent() float64 {
	// This is a simplified implementation
	// In production, you might want to use more sophisticated CPU monitoring
	return 0.0 // Placeholder
}

// extractDatabaseInfo extracts database names from logs
func (p *circuitBreakerProcessor) extractDatabaseInfo(logs plog.Logs) []string {
	databases := make(map[string]bool)
	
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				if dbName, exists := lr.Attributes().Get("database_name"); exists {
					databases[dbName.Str()] = true
				}
			}
		}
	}
	
	result := make([]string, 0, len(databases))
	for db := range databases {
		result = append(result, db)
	}
	return result
}

// allowDatabaseRequest checks if requests for a specific database should be allowed
func (p *circuitBreakerProcessor) allowDatabaseRequest(dbName string) bool {
	p.dbStatesMutex.RLock()
	state, exists := p.databaseStates[dbName]
	p.dbStatesMutex.RUnlock()
	
	if !exists {
		return true // No state means allow
	}
	
	state.mutex.RLock()
	defer state.mutex.RUnlock()
	
	switch state.state {
	case Closed:
		return true
	case Open:
		if time.Since(state.lastFailure) > p.config.OpenStateTimeout {
			// Transition to half-open
			state.mutex.RUnlock()
			state.mutex.Lock()
			if state.state == Open && time.Since(state.lastFailure) > p.config.OpenStateTimeout {
				state.state = HalfOpen
				state.successCount = 0
				p.logger.Info("Database circuit breaker transitioning to half-open",
					zap.String("database", dbName))
			}
			state.mutex.Unlock()
			state.mutex.RLock()
			return state.state == HalfOpen
		}
		return false
	case HalfOpen:
		return true
	default:
		return false
	}
}

// filterLogsForDatabase removes logs for a specific database
func (p *circuitBreakerProcessor) filterLogsForDatabase(logs plog.Logs, dbName string) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			
			// Filter log records
			filtered := sl.LogRecords()
			for k := filtered.Len() - 1; k >= 0; k-- {
				lr := filtered.At(k)
				if db, exists := lr.Attributes().Get("database_name"); exists && db.Str() == dbName {
					filtered.RemoveIf(func(record plog.LogRecord) bool {
						if recordDB, ok := record.Attributes().Get("database_name"); ok {
							return recordDB.Str() == dbName
						}
						return false
					})
				}
			}
		}
	}
}

// onDatabaseFailure handles failures for a specific database
func (p *circuitBreakerProcessor) onDatabaseFailure(dbName string, err error, duration time.Duration) {
	p.dbStatesMutex.Lock()
	state, exists := p.databaseStates[dbName]
	if !exists {
		state = &databaseCircuitState{
			state: Closed,
		}
		p.databaseStates[dbName] = state
	}
	p.dbStatesMutex.Unlock()
	
	state.mutex.Lock()
	defer state.mutex.Unlock()
	
	state.failureCount++
	state.lastFailure = time.Now()
	
	// Update error rate
	state.errorRate = float64(state.failureCount) / float64(state.failureCount+state.successCount)
	
	switch state.state {
	case Closed:
		if state.failureCount >= p.config.FailureThreshold {
			state.state = Open
			p.logger.Error("Database circuit breaker opened due to failures",
				zap.String("database", dbName),
				zap.Int("failure_count", state.failureCount),
				zap.Float64("error_rate", state.errorRate),
				zap.Error(err))
		}
	case HalfOpen:
		state.state = Open
		state.successCount = 0
		p.logger.Error("Database circuit breaker reopened after failure in half-open state",
			zap.String("database", dbName),
			zap.Error(err))
	}
}

// onDatabaseSuccess handles successful requests for a specific database
func (p *circuitBreakerProcessor) onDatabaseSuccess(dbName string, duration time.Duration) {
	p.dbStatesMutex.RLock()
	state, exists := p.databaseStates[dbName]
	p.dbStatesMutex.RUnlock()
	
	if !exists {
		return // No state to update
	}
	
	state.mutex.Lock()
	defer state.mutex.Unlock()
	
	// Update average duration
	if state.avgDuration == 0 {
		state.avgDuration = duration
	} else {
		state.avgDuration = (state.avgDuration + duration) / 2
	}
	
	switch state.state {
	case Closed:
		state.failureCount = 0
		state.successCount++
	case HalfOpen:
		state.successCount++
		if state.successCount >= p.config.SuccessThreshold {
			state.state = Closed
			state.failureCount = 0
			state.successCount = 0
			p.logger.Info("Database circuit breaker closed after successful recovery",
				zap.String("database", dbName),
				zap.Duration("avg_duration", state.avgDuration))
		}
	}
	
	// Update error rate
	if state.failureCount+state.successCount > 0 {
		state.errorRate = float64(state.failureCount) / float64(state.failureCount+state.successCount)
	}
}

// isNewRelicError checks if the error is related to New Relic integration
func (p *circuitBreakerProcessor) isNewRelicError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	nrErrorPatterns := []string{
		"cardinality",
		"NrIntegrationError",
		"api-key",
		"rate limit",
		"quota exceeded",
		"unique time series",
	}
	
	for _, pattern := range nrErrorPatterns {
		if contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || 
		 len(s) > len(substr) && 
		 (s[:len(substr)] == substr || 
		  s[len(s)-len(substr):] == substr ||
		  len(substr) > 0 && len(s) > len(substr) && 
		  containsMiddle(s, substr)))
}

// containsMiddle checks if substring exists in the middle of string
func containsMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ThroughputMonitor tracks request throughput
type ThroughputMonitor struct {
	mu              sync.RWMutex
	windowSize      time.Duration
	requests        []time.Time
	maxThroughput   float64
	currentRate     float64
}

// NewThroughputMonitor creates a new throughput monitor
func NewThroughputMonitor(windowSize time.Duration) *ThroughputMonitor {
	return &ThroughputMonitor{
		windowSize:    windowSize,
		requests:      make([]time.Time, 0, 1000),
		maxThroughput: 1000, // Default max 1000 requests per window
	}
}

// RecordRequest records a new request
func (tm *ThroughputMonitor) RecordRequest() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	tm.requests = append(tm.requests, now)

	// Clean old requests
	cutoff := now.Add(-tm.windowSize)
	i := 0
	for i < len(tm.requests) && tm.requests[i].Before(cutoff) {
		i++
	}
	tm.requests = tm.requests[i:]

	// Calculate current rate
	tm.currentRate = float64(len(tm.requests)) / tm.windowSize.Seconds()
}

// GetRate returns the current throughput rate
func (tm *ThroughputMonitor) GetRate() float64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.currentRate
}

// IsOverloaded checks if throughput exceeds threshold
func (tm *ThroughputMonitor) IsOverloaded() bool {
	return tm.GetRate() > tm.maxThroughput
}

// LatencyTracker tracks request latencies
type LatencyTracker struct {
	mu         sync.RWMutex
	latencies  []time.Duration
	maxSize    int
	p50        time.Duration
	p95        time.Duration
	p99        time.Duration
}

// NewLatencyTracker creates a new latency tracker
func NewLatencyTracker(maxSize int) *LatencyTracker {
	return &LatencyTracker{
		latencies: make([]time.Duration, 0, maxSize),
		maxSize:   maxSize,
	}
}

// RecordLatency records a request latency
func (lt *LatencyTracker) RecordLatency(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.latencies = append(lt.latencies, latency)
	if len(lt.latencies) > lt.maxSize {
		lt.latencies = lt.latencies[1:]
	}

	// Calculate percentiles
	if len(lt.latencies) > 0 {
		sorted := make([]time.Duration, len(lt.latencies))
		copy(sorted, lt.latencies)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i] < sorted[j]
		})

		lt.p50 = sorted[len(sorted)*50/100]
		lt.p95 = sorted[len(sorted)*95/100]
		lt.p99 = sorted[len(sorted)*99/100]
	}
}

// GetPercentiles returns latency percentiles
func (lt *LatencyTracker) GetPercentiles() (p50, p95, p99 time.Duration) {
	lt.mu.RLock()
	defer lt.mu.RUnlock()
	return lt.p50, lt.p95, lt.p99
}

// ErrorClassifier classifies errors by type
type ErrorClassifier struct {
	mu           sync.RWMutex
	errorCounts  map[string]int64
	totalErrors  int64
	classifiers  []ErrorClassification
}

// ErrorClassification defines an error classification rule
type ErrorClassification struct {
	Name      string
	Pattern   string
	Severity  string
	Retryable bool
}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier() *ErrorClassifier {
	return &ErrorClassifier{
		errorCounts: make(map[string]int64),
		classifiers: []ErrorClassification{
			{Name: "timeout", Pattern: "context deadline exceeded", Severity: "warning", Retryable: true},
			{Name: "connection", Pattern: "connection refused", Severity: "error", Retryable: true},
			{Name: "authentication", Pattern: "authentication failed", Severity: "critical", Retryable: false},
			{Name: "cardinality", Pattern: "cardinality", Severity: "warning", Retryable: false},
			{Name: "rate_limit", Pattern: "rate limit", Severity: "warning", Retryable: true},
			{Name: "memory", Pattern: "out of memory", Severity: "critical", Retryable: false},
			{Name: "disk", Pattern: "disk full", Severity: "critical", Retryable: false},
		},
	}
}

// ClassifyError classifies an error
func (ec *ErrorClassifier) ClassifyError(err error) string {
	if err == nil {
		return "none"
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()

	errStr := err.Error()
	for _, classifier := range ec.classifiers {
		if contains(errStr, classifier.Pattern) {
			ec.errorCounts[classifier.Name]++
			ec.totalErrors++
			return classifier.Name
		}
	}

	ec.errorCounts["unknown"]++
	ec.totalErrors++
	return "unknown"
}

// GetErrorStats returns error statistics
func (ec *ErrorClassifier) GetErrorStats() map[string]int64 {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	stats := make(map[string]int64)
	for k, v := range ec.errorCounts {
		stats[k] = v
	}
	stats["total"] = ec.totalErrors
	return stats
}

// MemoryMonitor monitors memory usage
type MemoryMonitor struct {
	mu                sync.RWMutex
	maxMemoryBytes    int64
	currentBytes      int64
	highWaterMark     int64
	pressureThreshold float64
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor(maxMemoryMB int) *MemoryMonitor {
	return &MemoryMonitor{
		maxMemoryBytes:    int64(maxMemoryMB) * 1024 * 1024,
		pressureThreshold: 0.8, // Alert at 80% usage
	}
}

// UpdateUsage updates current memory usage
func (mm *MemoryMonitor) UpdateUsage(bytes int64) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	mm.currentBytes = bytes
	if bytes > mm.highWaterMark {
		mm.highWaterMark = bytes
	}
}

// IsUnderPressure checks if memory is under pressure
func (mm *MemoryMonitor) IsUnderPressure() bool {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if mm.maxMemoryBytes <= 0 {
		return false
	}

	usage := float64(mm.currentBytes) / float64(mm.maxMemoryBytes)
	return usage > mm.pressureThreshold
}

// GetUsagePercent returns memory usage percentage
func (mm *MemoryMonitor) GetUsagePercent() float64 {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if mm.maxMemoryBytes <= 0 {
		return 0
	}

	return float64(mm.currentBytes) / float64(mm.maxMemoryBytes) * 100
}