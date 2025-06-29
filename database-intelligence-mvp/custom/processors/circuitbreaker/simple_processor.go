// Package circuitbreaker protects databases from monitoring overload
// This fills a gap where OTEL doesn't provide database-aware circuit breaking
package circuitbreaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

const (
	// TypeStr is the type string for the circuit breaker processor
	TypeStr   = "circuit_breaker"
	stability = component.StabilityLevelBeta
)

// Config for circuit breaker
type Config struct {
	ErrorThresholdPercent float64       `mapstructure:"error_threshold_percent"`
	VolumeThresholdQPS    float64       `mapstructure:"volume_threshold_qps"`
	EvaluationInterval    time.Duration `mapstructure:"evaluation_interval"`
	BreakDuration         time.Duration `mapstructure:"break_duration"`
	HalfOpenRequests      int           `mapstructure:"half_open_requests"`
}

// State of the circuit
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// simpleCircuitBreaker protects databases from overload
type simpleCircuitBreaker struct {
	logger      *zap.Logger
	config      *Config
	nextMetrics consumer.Metrics
	nextLogs    consumer.Logs

	// Circuit state
	state            State
	stateChangedAt   time.Time
	consecutiveFailures int
	halfOpenAttempts int
	mu               sync.RWMutex

	// Metrics for circuit decisions
	requestCount   int64
	errorCount     int64
	lastResetTime  time.Time
	metricsMu      sync.Mutex
}

// NewFactory creates the factory for circuit breaker
func NewFactory() processor.Factory {
	return processor.NewFactory(
		TypeStr,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
		processor.WithLogs(createLogsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ErrorThresholdPercent: 50.0,
		VolumeThresholdQPS:    1000.0,
		EvaluationInterval:    30 * time.Second,
		BreakDuration:         5 * time.Minute,
		HalfOpenRequests:      10,
	}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	config := cfg.(*Config)
	return &simpleCircuitBreaker{
		logger:        set.Logger,
		config:        config,
		nextMetrics:   nextConsumer,
		state:         StateClosed,
		lastResetTime: time.Now(),
	}, nil
}

func createLogsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {
	config := cfg.(*Config)
	return &simpleCircuitBreaker{
		logger:        set.Logger,
		config:        config,
		nextLogs:      nextConsumer,
		state:         StateClosed,
		lastResetTime: time.Now(),
	}, nil
}

// Start the processor
func (cb *simpleCircuitBreaker) Start(ctx context.Context, host component.Host) error {
	cb.logger.Info("Starting circuit breaker processor")
	
	// Start evaluation ticker
	go cb.evaluateCircuit(ctx)
	
	return nil
}

// Shutdown the processor
func (cb *simpleCircuitBreaker) Shutdown(context.Context) error {
	return nil
}

// ConsumeMetrics checks circuit before forwarding metrics
func (cb *simpleCircuitBreaker) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if !cb.allowRequest() {
		cb.logger.Debug("Circuit breaker OPEN, dropping metrics")
		return nil // Drop data to protect database
	}

	// Forward to next consumer
	err := cb.nextMetrics.ConsumeMetrics(ctx, md)
	cb.recordResult(err)
	
	return err
}

// ConsumeLogs checks circuit before forwarding logs
func (cb *simpleCircuitBreaker) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if !cb.allowRequest() {
		cb.logger.Debug("Circuit breaker OPEN, dropping logs")
		return nil // Drop data to protect database
	}

	// Forward to next consumer
	err := cb.nextLogs.ConsumeLogs(ctx, ld)
	cb.recordResult(err)
	
	return err
}

// allowRequest checks if request should be allowed
func (cb *simpleCircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
		
	case StateOpen:
		// Check if we should transition to half-open
		if time.Since(cb.stateChangedAt) > cb.config.BreakDuration {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.transitionToHalfOpen()
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
		
	case StateHalfOpen:
		// Allow limited requests
		if cb.halfOpenAttempts < cb.config.HalfOpenRequests {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.halfOpenAttempts++
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
		
	default:
		return false
	}
}

// recordResult records the result of a request
func (cb *simpleCircuitBreaker) recordResult(err error) {
	cb.metricsMu.Lock()
	defer cb.metricsMu.Unlock()

	cb.requestCount++
	
	if err != nil {
		cb.errorCount++
		cb.consecutiveFailures++
		
		// Check if we should open the circuit
		cb.mu.Lock()
		if cb.state == StateHalfOpen {
			// Failed in half-open, go back to open
			cb.transitionToOpen("error in half-open state")
		}
		cb.mu.Unlock()
	} else {
		cb.consecutiveFailures = 0
		
		// Check if we should close the circuit
		cb.mu.Lock()
		if cb.state == StateHalfOpen && cb.halfOpenAttempts >= cb.config.HalfOpenRequests {
			cb.transitionToClosed()
		}
		cb.mu.Unlock()
	}
}

// evaluateCircuit periodically evaluates circuit health
func (cb *simpleCircuitBreaker) evaluateCircuit(ctx context.Context) {
	ticker := time.NewTicker(cb.config.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cb.evaluate()
		case <-ctx.Done():
			return
		}
	}
}

// evaluate checks metrics and updates circuit state
func (cb *simpleCircuitBreaker) evaluate() {
	cb.metricsMu.Lock()
	
	// Calculate metrics
	duration := time.Since(cb.lastResetTime).Seconds()
	qps := float64(cb.requestCount) / duration
	errorRate := float64(cb.errorCount) / float64(cb.requestCount) * 100
	
	// Reset counters
	cb.requestCount = 0
	cb.errorCount = 0
	cb.lastResetTime = time.Now()
	
	cb.metricsMu.Unlock()

	// Make circuit decisions
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateClosed {
		// Check if we should open
		if errorRate > cb.config.ErrorThresholdPercent {
			cb.transitionToOpen(fmt.Sprintf("error rate %.1f%% exceeds threshold", errorRate))
		} else if qps > cb.config.VolumeThresholdQPS {
			cb.transitionToOpen(fmt.Sprintf("QPS %.1f exceeds threshold", qps))
		}
	}
	
	cb.logger.Debug("Circuit breaker evaluation",
		zap.String("state", cb.getStateName()),
		zap.Float64("qps", qps),
		zap.Float64("error_rate", errorRate))
}

// State transition methods
func (cb *simpleCircuitBreaker) transitionToOpen(reason string) {
	cb.state = StateOpen
	cb.stateChangedAt = time.Now()
	cb.halfOpenAttempts = 0
	
	cb.logger.Warn("Circuit breaker opened",
		zap.String("reason", reason))
}

func (cb *simpleCircuitBreaker) transitionToHalfOpen() {
	cb.state = StateHalfOpen
	cb.stateChangedAt = time.Now()
	cb.halfOpenAttempts = 0
	
	cb.logger.Info("Circuit breaker half-open")
}

func (cb *simpleCircuitBreaker) transitionToClosed() {
	cb.state = StateClosed
	cb.stateChangedAt = time.Now()
	cb.consecutiveFailures = 0
	cb.halfOpenAttempts = 0
	
	cb.logger.Info("Circuit breaker closed")
}

func (cb *simpleCircuitBreaker) getStateName() string {
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}