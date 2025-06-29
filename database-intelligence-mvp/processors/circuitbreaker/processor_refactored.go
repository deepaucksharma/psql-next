// Package circuitbreaker implements a generic circuit breaker processor
// that can protect any backend system, not just databases
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
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// State represents the circuit breaker state
type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

// circuitBreakerRefactoredProcessor implements a generic circuit breaker
type circuitBreakerRefactoredProcessor struct {
	config *Config
	logger *zap.Logger

	// Next consumers in the pipeline
	nextMetrics consumer.Metrics
	nextLogs    consumer.Logs
	nextTraces  consumer.Traces

	// Circuit breaker state management
	circuits map[string]*Circuit // Key is configurable (e.g., service name, endpoint)
	mu       sync.RWMutex

	// Metrics for monitoring the circuit breaker itself
	circuitMetrics *CircuitMetrics

	// External state store interface for HA
	stateStore StateStore

	// Shutdown
	shutdownChan chan struct{}
	wg           sync.WaitGroup
}

// Circuit represents a single circuit breaker instance
type Circuit struct {
	ID              string
	State           State
	FailureCount    int64
	SuccessCount    int64
	ConsecutiveFails int64
	LastFailure     time.Time
	LastSuccess     time.Time
	LastStateChange time.Time
	HalfOpenAttempts int

	// Thresholds
	FailureThreshold   int64
	SuccessThreshold   int64
	Timeout            time.Duration
	HalfOpenMaxAttempts int
}

// CircuitMetrics tracks circuit breaker performance
type CircuitMetrics struct {
	TotalRequests   int64
	BlockedRequests int64
	PassedRequests  int64
	StateChanges    map[string]int64 // state -> count
	mu              sync.Mutex
}

// StateStore interface for external state persistence (Redis, etcd, etc.)
type StateStore interface {
	SaveCircuitState(ctx context.Context, circuitID string, state *Circuit) error
	LoadCircuitState(ctx context.Context, circuitID string) (*Circuit, error)
	ListCircuits(ctx context.Context) ([]string, error)
}

// newCircuitBreakerRefactoredProcessor creates a new circuit breaker processor
func newCircuitBreakerRefactoredProcessor(
	cfg *Config,
	logger *zap.Logger,
	nextMetrics consumer.Metrics,
	nextLogs consumer.Logs,
	nextTraces consumer.Traces,
	stateStore StateStore,
) (*circuitBreakerRefactoredProcessor, error) {
	
	p := &circuitBreakerRefactoredProcessor{
		config:      cfg,
		logger:      logger,
		nextMetrics: nextMetrics,
		nextLogs:    nextLogs,
		nextTraces:  nextTraces,
		circuits:    make(map[string]*Circuit),
		circuitMetrics: &CircuitMetrics{
			StateChanges: make(map[string]int64),
		},
		stateStore:   stateStore,
		shutdownChan: make(chan struct{}),
	}

	// Load existing circuit states if using external store
	if stateStore != nil {
		if err := p.loadCircuitStates(context.Background()); err != nil {
			logger.Warn("Failed to load circuit states", zap.Error(err))
		}
	}

	return p, nil
}

// Start implements the component.Component interface
func (p *circuitBreakerRefactoredProcessor) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting circuit breaker processor")

	// Start state management goroutine
	p.wg.Add(1)
	go p.stateManager(ctx)

	// Start metrics reporter
	p.wg.Add(1)
	go p.metricsReporter(ctx)

	return nil
}

// Shutdown implements the component.Component interface
func (p *circuitBreakerRefactoredProcessor) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down circuit breaker processor")

	close(p.shutdownChan)

	// Wait for goroutines
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ConsumeMetrics implements the consumer.Metrics interface
func (p *circuitBreakerRefactoredProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Extract circuit ID from metrics
	circuitID := p.extractCircuitID(md)
	
	// Check circuit state
	allowed, circuit := p.checkCircuit(circuitID)
	
	p.circuitMetrics.mu.Lock()
	p.circuitMetrics.TotalRequests++
	if allowed {
		p.circuitMetrics.PassedRequests++
	} else {
		p.circuitMetrics.BlockedRequests++
	}
	p.circuitMetrics.mu.Unlock()

	if !allowed {
		p.logger.Debug("Circuit breaker OPEN, blocking metrics",
			zap.String("circuit_id", circuitID),
			zap.String("state", string(circuit.State)))
		return nil // Drop the data
	}

	// Pass to next consumer and track result
	err := p.nextMetrics.ConsumeMetrics(ctx, md)
	p.recordResult(circuitID, err)

	return err
}

// ConsumeLogs implements the consumer.Logs interface
func (p *circuitBreakerRefactoredProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Extract circuit ID from logs
	circuitID := p.extractCircuitIDFromLogs(ld)
	
	// Check circuit state
	allowed, circuit := p.checkCircuit(circuitID)
	
	if !allowed {
		p.logger.Debug("Circuit breaker OPEN, blocking logs",
			zap.String("circuit_id", circuitID),
			zap.String("state", string(circuit.State)))
		return nil // Drop the data
	}

	// Pass to next consumer and track result
	err := p.nextLogs.ConsumeLogs(ctx, ld)
	p.recordResult(circuitID, err)

	return err
}

// ConsumeTraces implements the consumer.Traces interface
func (p *circuitBreakerRefactoredProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Extract circuit ID from traces
	circuitID := p.extractCircuitIDFromTraces(td)
	
	// Check circuit state
	allowed, circuit := p.checkCircuit(circuitID)
	
	if !allowed {
		p.logger.Debug("Circuit breaker OPEN, blocking traces",
			zap.String("circuit_id", circuitID),
			zap.String("state", string(circuit.State)))
		return nil // Drop the data
	}

	// Pass to next consumer and track result
	err := p.nextTraces.ConsumeTraces(ctx, td)
	p.recordResult(circuitID, err)

	return err
}

// checkCircuit checks if a circuit allows traffic
func (p *circuitBreakerRefactoredProcessor) checkCircuit(circuitID string) (bool, *Circuit) {
	p.mu.Lock()
	defer p.mu.Unlock()

	circuit, exists := p.circuits[circuitID]
	if !exists {
		// Create new circuit
		circuit = &Circuit{
			ID:                  circuitID,
			State:               StateClosed,
			FailureThreshold:    p.config.FailureThreshold,
			SuccessThreshold:    p.config.SuccessThreshold,
			Timeout:             p.config.Timeout,
			HalfOpenMaxAttempts: p.config.HalfOpenMaxAttempts,
			LastStateChange:     time.Now(),
		}
		p.circuits[circuitID] = circuit
	}

	switch circuit.State {
	case StateClosed:
		return true, circuit
		
	case StateOpen:
		// Check if timeout has passed
		if time.Since(circuit.LastStateChange) > circuit.Timeout {
			// Transition to half-open
			p.transitionState(circuit, StateHalfOpen)
			return true, circuit
		}
		return false, circuit
		
	case StateHalfOpen:
		// Allow limited traffic
		if circuit.HalfOpenAttempts < circuit.HalfOpenMaxAttempts {
			circuit.HalfOpenAttempts++
			return true, circuit
		}
		return false, circuit
		
	default:
		return false, circuit
	}
}

// recordResult records the result of a request
func (p *circuitBreakerRefactoredProcessor) recordResult(circuitID string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	circuit, exists := p.circuits[circuitID]
	if !exists {
		return
	}

	if err != nil {
		// Failure
		circuit.FailureCount++
		circuit.ConsecutiveFails++
		circuit.LastFailure = time.Now()

		// Check state transitions
		if circuit.State == StateClosed && circuit.ConsecutiveFails >= circuit.FailureThreshold {
			p.transitionState(circuit, StateOpen)
		} else if circuit.State == StateHalfOpen {
			// Failed in half-open, go back to open
			p.transitionState(circuit, StateOpen)
		}
	} else {
		// Success
		circuit.SuccessCount++
		circuit.ConsecutiveFails = 0
		circuit.LastSuccess = time.Now()

		// Check state transitions
		if circuit.State == StateHalfOpen {
			// Succeeded in half-open, check if we can close
			if circuit.SuccessCount >= circuit.SuccessThreshold {
				p.transitionState(circuit, StateClosed)
			}
		}
	}

	// Persist state if using external store
	if p.stateStore != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.stateStore.SaveCircuitState(ctx, circuitID, circuit); err != nil {
				p.logger.Warn("Failed to persist circuit state",
					zap.String("circuit_id", circuitID),
					zap.Error(err))
			}
		}()
	}
}

// transitionState transitions a circuit to a new state
func (p *circuitBreakerRefactoredProcessor) transitionState(circuit *Circuit, newState State) {
	oldState := circuit.State
	circuit.State = newState
	circuit.LastStateChange = time.Now()

	// Reset counters based on new state
	switch newState {
	case StateClosed:
		circuit.ConsecutiveFails = 0
		circuit.HalfOpenAttempts = 0
	case StateOpen:
		circuit.SuccessCount = 0
	case StateHalfOpen:
		circuit.HalfOpenAttempts = 0
		circuit.SuccessCount = 0
	}

	p.circuitMetrics.mu.Lock()
	p.circuitMetrics.StateChanges[fmt.Sprintf("%s->%s", oldState, newState)]++
	p.circuitMetrics.mu.Unlock()

	p.logger.Info("Circuit state changed",
		zap.String("circuit_id", circuit.ID),
		zap.String("old_state", string(oldState)),
		zap.String("new_state", string(newState)))
}

// extractCircuitID extracts circuit ID from metrics
// This is configurable based on the Config.CircuitIDAttribute
func (p *circuitBreakerRefactoredProcessor) extractCircuitID(md pmetric.Metrics) string {
	// Default circuit ID
	defaultID := "default"

	if p.config.CircuitIDAttribute == "" {
		return defaultID
	}

	// Look for the attribute in resource metrics
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		if val, ok := rm.Resource().Attributes().Get(p.config.CircuitIDAttribute); ok {
			return val.AsString()
		}
	}

	return defaultID
}

// extractCircuitIDFromLogs extracts circuit ID from logs
func (p *circuitBreakerRefactoredProcessor) extractCircuitIDFromLogs(ld plog.Logs) string {
	defaultID := "default"

	if p.config.CircuitIDAttribute == "" {
		return defaultID
	}

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		if val, ok := rl.Resource().Attributes().Get(p.config.CircuitIDAttribute); ok {
			return val.AsString()
		}
	}

	return defaultID
}

// extractCircuitIDFromTraces extracts circuit ID from traces
func (p *circuitBreakerRefactoredProcessor) extractCircuitIDFromTraces(td ptrace.Traces) string {
	defaultID := "default"

	if p.config.CircuitIDAttribute == "" {
		return defaultID
	}

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		if val, ok := rs.Resource().Attributes().Get(p.config.CircuitIDAttribute); ok {
			return val.AsString()
		}
	}

	return defaultID
}

// stateManager periodically checks circuit states
func (p *circuitBreakerRefactoredProcessor) stateManager(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.checkCircuitTimeouts()
		case <-p.shutdownChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// checkCircuitTimeouts checks for circuits that should transition due to timeout
func (p *circuitBreakerRefactoredProcessor) checkCircuitTimeouts() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, circuit := range p.circuits {
		if circuit.State == StateOpen && time.Since(circuit.LastStateChange) > circuit.Timeout {
			p.transitionState(circuit, StateHalfOpen)
		}
	}
}

// metricsReporter periodically reports circuit breaker metrics
func (p *circuitBreakerRefactoredProcessor) metricsReporter(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.reportMetrics()
		case <-p.shutdownChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// reportMetrics logs circuit breaker metrics
func (p *circuitBreakerRefactoredProcessor) reportMetrics() {
	p.circuitMetrics.mu.Lock()
	defer p.circuitMetrics.mu.Unlock()

	p.logger.Info("Circuit breaker metrics",
		zap.Int64("total_requests", p.circuitMetrics.TotalRequests),
		zap.Int64("passed_requests", p.circuitMetrics.PassedRequests),
		zap.Int64("blocked_requests", p.circuitMetrics.BlockedRequests),
		zap.Any("state_changes", p.circuitMetrics.StateChanges))

	// Count circuits by state
	p.mu.RLock()
	stateCounts := make(map[State]int)
	for _, circuit := range p.circuits {
		stateCounts[circuit.State]++
	}
	p.mu.RUnlock()

	p.logger.Info("Circuit states",
		zap.Any("state_counts", stateCounts),
		zap.Int("total_circuits", len(p.circuits)))
}

// loadCircuitStates loads circuit states from external store
func (p *circuitBreakerRefactoredProcessor) loadCircuitStates(ctx context.Context) error {
	if p.stateStore == nil {
		return nil
	}

	circuitIDs, err := p.stateStore.ListCircuits(ctx)
	if err != nil {
		return fmt.Errorf("failed to list circuits: %w", err)
	}

	for _, id := range circuitIDs {
		circuit, err := p.stateStore.LoadCircuitState(ctx, id)
		if err != nil {
			p.logger.Warn("Failed to load circuit state",
				zap.String("circuit_id", id),
				zap.Error(err))
			continue
		}

		p.mu.Lock()
		p.circuits[id] = circuit
		p.mu.Unlock()
	}

	p.logger.Info("Loaded circuit states",
		zap.Int("count", len(circuitIDs)))

	return nil
}