package circuitbreaker

import (
	"context"
	"fmt"
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

	// Concurrency control
	semaphore chan struct{}

	// Adaptive timeout
	currentTimeout time.Duration
	timeoutMutex   sync.RWMutex

	// Metrics
	totalRequests    int64
	failedRequests   int64
	rejectedRequests int64

	// Shutdown
	shutdownChan chan struct{}
	wg           sync.WaitGroup
}

// newCircuitBreakerProcessor creates a new circuit breaker processor
func newCircuitBreakerProcessor(cfg *Config, logger *zap.Logger, consumer consumer.Logs) *circuitBreakerProcessor {
	return &circuitBreakerProcessor{
		config:         cfg,
		logger:         logger,
		consumer:       consumer,
		state:          Closed,
		semaphore:      make(chan struct{}, cfg.MaxConcurrentRequests),
		currentTimeout: cfg.BaseTimeout,
		shutdownChan:   make(chan struct{}),
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

	// Check circuit state
	if !p.allowRequest() {
		p.rejectedRequests++
		p.logger.Warn("Circuit breaker open, rejecting request",
			zap.String("state", p.getState().String()),
			zap.Int64("rejected_requests", p.rejectedRequests))
		return fmt.Errorf("circuit breaker open")
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

	// Handle result
	if err != nil {
		p.onFailure(err)
		p.failedRequests++
		
		// Adjust timeout if adaptive timeout is enabled
		if p.config.EnableAdaptiveTimeout {
			p.adjustTimeout(duration, false)
		}
		
		return err
	}

	p.onSuccess()
	
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

	// Log current state periodically
	state := p.getState()
	if state != Closed {
		p.logger.Info("Circuit breaker status",
			zap.String("state", state.String()),
			zap.Int("failure_count", p.failureCount),
			zap.Int("success_count", p.successCount),
			zap.Duration("current_timeout", p.getCurrentTimeout()))
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