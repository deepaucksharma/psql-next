// Package circuitbreaker implements database protection via circuit breaking
// This addresses an OTEL gap - standard processors don't protect databases from monitoring overhead
package circuitbreaker

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Config for circuit breaker
type Config struct {
	// Error threshold to open circuit (0.0-1.0)
	ErrorThreshold float64 `mapstructure:"error_threshold"`
	// Duration before attempting to close circuit
	ResetTimeout time.Duration `mapstructure:"reset_timeout"`
	// Number of requests to track
	WindowSize int `mapstructure:"window_size"`
}

// State represents circuit state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

// circuitBreakerProcessor protects databases from excessive monitoring
type circuitBreakerProcessor struct {
	config        *Config
	logger        *zap.Logger
	nextMetrics   consumer.Metrics
	nextLogs      consumer.Logs
	
	mu            sync.RWMutex
	circuits      map[string]*circuit
}

// circuit tracks state for a specific database
type circuit struct {
	state         State
	failures      int
	successes     int
	lastFailTime  time.Time
	requests      []bool // ring buffer of recent requests (true=success)
	requestIndex  int
}

// newCircuitBreakerProcessor creates a new circuit breaker
func newCircuitBreakerProcessor(config *Config, logger *zap.Logger) *circuitBreakerProcessor {
	return &circuitBreakerProcessor{
		config:   config,
		logger:   logger,
		circuits: make(map[string]*circuit),
	}
}

// ConsumeMetrics implements the consumer.Metrics interface
func (cbp *circuitBreakerProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Extract database identifier from resource attributes
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		dbName := rm.Resource().Attributes().PutStr("db.name", "unknown")
		
		// Check circuit state
		if cbp.shouldBlock(dbName.Str()) {
			cbp.logger.Debug("Circuit open, dropping metrics", zap.String("database", dbName.Str()))
			// Remove this resource metric
			rms.RemoveIf(func(r pmetric.ResourceMetrics) bool {
				name, _ := r.Resource().Attributes().Get("db.name")
				return name.Str() == dbName.Str()
			})
		}
	}
	
	// Forward remaining metrics
	if rms.Len() > 0 && cbp.nextMetrics != nil {
		return cbp.nextMetrics.ConsumeMetrics(ctx, md)
	}
	return nil
}

// ConsumeLogs implements the consumer.Logs interface
func (cbp *circuitBreakerProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Similar logic for logs
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		dbName := rl.Resource().Attributes().PutStr("db.name", "unknown")
		
		// Check for errors in logs
		hasError := cbp.checkLogsForErrors(rl)
		cbp.recordRequest(dbName.Str(), !hasError)
		
		// Check circuit state
		if cbp.shouldBlock(dbName.Str()) {
			cbp.logger.Debug("Circuit open, dropping logs", zap.String("database", dbName.Str()))
			// Remove this resource log
			rls.RemoveIf(func(r plog.ResourceLogs) bool {
				name, _ := r.Resource().Attributes().Get("db.name")
				return name.Str() == dbName.Str()
			})
		}
	}
	
	// Forward remaining logs
	if rls.Len() > 0 && cbp.nextLogs != nil {
		return cbp.nextLogs.ConsumeLogs(ctx, ld)
	}
	return nil
}

// shouldBlock checks if requests should be blocked for a database
func (cbp *circuitBreakerProcessor) shouldBlock(dbName string) bool {
	cbp.mu.Lock()
	defer cbp.mu.Unlock()
	
	c, exists := cbp.circuits[dbName]
	if !exists {
		// Create new circuit
		c = &circuit{
			state:    StateClosed,
			requests: make([]bool, cbp.config.WindowSize),
		}
		cbp.circuits[dbName] = c
	}
	
	switch c.state {
	case StateClosed:
		// Check if we should open
		errorRate := cbp.calculateErrorRate(c)
		if errorRate > cbp.config.ErrorThreshold {
			c.state = StateOpen
			c.lastFailTime = time.Now()
			cbp.logger.Warn("Circuit opened", zap.String("database", dbName), zap.Float64("error_rate", errorRate))
			return true
		}
		return false
		
	case StateOpen:
		// Check if we should try half-open
		if time.Since(c.lastFailTime) > cbp.config.ResetTimeout {
			c.state = StateHalfOpen
			cbp.logger.Info("Circuit half-open", zap.String("database", dbName))
			return false
		}
		return true
		
	case StateHalfOpen:
		// Let one request through
		return false
	}
	
	return false
}

// recordRequest records a request result
func (cbp *circuitBreakerProcessor) recordRequest(dbName string, success bool) {
	cbp.mu.Lock()
	defer cbp.mu.Unlock()
	
	c, exists := cbp.circuits[dbName]
	if !exists {
		return
	}
	
	// Update ring buffer
	c.requests[c.requestIndex] = success
	c.requestIndex = (c.requestIndex + 1) % len(c.requests)
	
	// Update state for half-open
	if c.state == StateHalfOpen {
		if success {
			c.state = StateClosed
			cbp.logger.Info("Circuit closed", zap.String("database", dbName))
		} else {
			c.state = StateOpen
			c.lastFailTime = time.Now()
			cbp.logger.Warn("Circuit re-opened", zap.String("database", dbName))
		}
	}
}

// calculateErrorRate calculates the error rate from recent requests
func (cbp *circuitBreakerProcessor) calculateErrorRate(c *circuit) float64 {
	failures := 0
	total := 0
	
	for _, success := range c.requests {
		total++
		if !success {
			failures++
		}
	}
	
	if total == 0 {
		return 0
	}
	
	return float64(failures) / float64(total)
}

// checkLogsForErrors checks if logs contain errors
func (cbp *circuitBreakerProcessor) checkLogsForErrors(rl plog.ResourceLogs) bool {
	sls := rl.ScopeLogs()
	for i := 0; i < sls.Len(); i++ {
		sl := sls.At(i)
		logs := sl.LogRecords()
		for j := 0; j < logs.Len(); j++ {
			lr := logs.At(j)
			// Check severity
			if lr.SeverityNumber() >= plog.SeverityNumberError {
				return true
			}
			// Check attributes
			if errAttr, ok := lr.Attributes().Get("has_error"); ok && errAttr.Bool() {
				return true
			}
		}
	}
	return false
}

// Capabilities returns the consumer capabilities
func (cbp *circuitBreakerProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// Start starts the processor
func (cbp *circuitBreakerProcessor) Start(ctx context.Context, host component.Host) error {
	cbp.logger.Info("Starting circuit breaker processor")
	return nil
}

// Shutdown stops the processor
func (cbp *circuitBreakerProcessor) Shutdown(ctx context.Context) error {
	cbp.logger.Info("Stopping circuit breaker processor")
	return nil
}