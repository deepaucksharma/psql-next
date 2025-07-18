package base

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.uber.org/zap"
)

// ConcurrentProcessor provides base functionality for processors that need concurrent operations
type ConcurrentProcessor struct {
	logger         *zap.Logger
	startCtx       context.Context    // Context from Start method
	startCancel    context.CancelFunc // Cancel function for startCtx
	shutdownChan   chan struct{}
	shutdownOnce   sync.Once
	wg             sync.WaitGroup
	mu             sync.RWMutex
	host           component.Host
	telemetryLevel configtelemetry.Level
}

// NewConcurrentProcessor creates a new base concurrent processor
func NewConcurrentProcessor(logger *zap.Logger) *ConcurrentProcessor {
	return &ConcurrentProcessor{
		logger:       logger,
		shutdownChan: make(chan struct{}),
	}
}

// Start initializes the concurrent processor
func (cp *ConcurrentProcessor) Start(ctx context.Context, host component.Host) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Store the context for use in background operations
	cp.startCtx, cp.startCancel = context.WithCancel(ctx)
	cp.host = host
	
	// Set default telemetry level
	cp.telemetryLevel = configtelemetry.LevelNormal

	return nil
}

// Shutdown gracefully shuts down the concurrent processor
func (cp *ConcurrentProcessor) Shutdown(ctx context.Context) error {
	var shutdownErr error
	
	cp.shutdownOnce.Do(func() {
		// Cancel the start context to signal all operations to stop
		if cp.startCancel != nil {
			cp.startCancel()
		}
		
		// Close shutdown channel
		close(cp.shutdownChan)
		
		// Wait for all goroutines with timeout
		done := make(chan struct{})
		go func() {
			cp.wg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			cp.logger.Debug("All goroutines stopped successfully")
		case <-ctx.Done():
			shutdownErr = ctx.Err()
			cp.logger.Warn("Shutdown timeout exceeded, some goroutines may still be running")
		}
	})
	
	return shutdownErr
}

// StartBackgroundTask starts a background task with proper context and lifecycle management
func (cp *ConcurrentProcessor) StartBackgroundTask(name string, interval time.Duration, task func(context.Context) error) {
	cp.wg.Add(1)
	go func() {
		defer cp.wg.Done()
		cp.runBackgroundTask(name, interval, task)
	}()
}

// runBackgroundTask runs a periodic background task
func (cp *ConcurrentProcessor) runBackgroundTask(name string, interval time.Duration, task func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	cp.logger.Info("Starting background task", zap.String("task", name), zap.Duration("interval", interval))

	for {
		select {
		case <-ticker.C:
			// Create a timeout context from the start context
			ctx, cancel := context.WithTimeout(cp.startCtx, interval/2)
			
			startTime := time.Now()
			err := task(ctx)
			duration := time.Since(startTime)
			
			cancel() // Clean up the context
			
			if err != nil {
				if ctx.Err() != nil {
					cp.logger.Warn("Background task cancelled or timed out",
						zap.String("task", name),
						zap.Error(err),
						zap.Duration("duration", duration))
				} else {
					cp.logger.Error("Background task failed",
						zap.String("task", name),
						zap.Error(err),
						zap.Duration("duration", duration))
				}
			} else if cp.telemetryLevel >= configtelemetry.LevelDetailed {
				cp.logger.Debug("Background task completed",
					zap.String("task", name),
					zap.Duration("duration", duration))
			}
			
		case <-cp.startCtx.Done():
			cp.logger.Info("Stopping background task due to context cancellation",
				zap.String("task", name))
			return
			
		case <-cp.shutdownChan:
			cp.logger.Info("Stopping background task due to shutdown",
				zap.String("task", name))
			return
		}
	}
}

// ExecuteWithContext executes a function with a timeout context derived from the start context
func (cp *ConcurrentProcessor) ExecuteWithContext(timeout time.Duration, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(cp.startCtx, timeout)
	defer cancel()
	return fn(ctx)
}

// GetContext returns a context that should be used for operations
// This context will be cancelled when the processor is shutting down
func (cp *ConcurrentProcessor) GetContext() context.Context {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	if cp.startCtx != nil {
		return cp.startCtx
	}
	// Fallback to background context if Start hasn't been called yet
	// This should not happen in normal operation
	return context.Background()
}

// IsShuttingDown returns true if the processor is shutting down
func (cp *ConcurrentProcessor) IsShuttingDown() bool {
	select {
	case <-cp.shutdownChan:
		return true
	default:
		return false
	}
}

// RunConcurrent runs multiple functions concurrently with proper error handling
func (cp *ConcurrentProcessor) RunConcurrent(ctx context.Context, tasks ...func(context.Context) error) error {
	if len(tasks) == 0 {
		return nil
	}

	errChan := make(chan error, len(tasks))
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, fn func(context.Context) error) {
			defer wg.Done()
			if err := fn(ctx); err != nil {
				errChan <- err
			}
		}(i, task)
	}

	// Wait for all tasks to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(errChan)
		// Return first error if any
		for err := range errChan {
			if err != nil {
				return err
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WorkerPool provides a concurrent worker pool pattern
type WorkerPool struct {
	cp       *ConcurrentProcessor
	workers  int
	jobQueue chan func()
	once     sync.Once
}

// NewWorkerPool creates a new worker pool
func (cp *ConcurrentProcessor) NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		cp:       cp,
		workers:  workers,
		jobQueue: make(chan func(), workers*2),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	wp.once.Do(func() {
		for i := 0; i < wp.workers; i++ {
			wp.cp.wg.Add(1)
			go wp.worker(i)
		}
	})
}

// Submit submits a job to the worker pool
func (wp *WorkerPool) Submit(job func()) error {
	select {
	case wp.jobQueue <- job:
		return nil
	case <-wp.cp.shutdownChan:
		return component.ErrDataTypeIsNotSupported
	default:
		// Queue is full
		return component.ErrDataTypeIsNotSupported
	}
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	close(wp.jobQueue)
}

func (wp *WorkerPool) worker(id int) {
	defer wp.cp.wg.Done()
	
	wp.cp.logger.Debug("Worker started", zap.Int("worker_id", id))
	
	for {
		select {
		case job, ok := <-wp.jobQueue:
			if !ok {
				wp.cp.logger.Debug("Worker stopping, job queue closed", zap.Int("worker_id", id))
				return
			}
			job()
			
		case <-wp.cp.shutdownChan:
			wp.cp.logger.Debug("Worker stopping due to shutdown", zap.Int("worker_id", id))
			return
		}
	}
}