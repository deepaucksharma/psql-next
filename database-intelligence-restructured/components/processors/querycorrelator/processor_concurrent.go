package querycorrelator

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/database-intelligence/db-intel/components/internal/boundedmap"
	"github.com/database-intelligence/db-intel/components/processors/base"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// ConcurrentQueryCorrelator is an improved version with proper context handling
type ConcurrentQueryCorrelator struct {
	*queryCorrelator              // Embed the original processor
	*base.ConcurrentProcessor     // Embed base concurrent functionality
	indexingWorkerPool    *base.WorkerPool
	enrichmentWorkerPool  *base.WorkerPool
	
	// Wait groups for phase synchronization
	indexingWaitGroup   *sync.WaitGroup
	enrichmentWaitGroup *sync.WaitGroup
	
	// Metrics for concurrent processing
	concurrentMetrics struct {
		metricsProcessed    atomic.Int64
		indexingTasks       atomic.Int64
		enrichmentTasks     atomic.Int64
		correlationsCreated atomic.Int64
	}
}

// NewConcurrentQueryCorrelator creates a new concurrent query correlator
func NewConcurrentQueryCorrelator(
	logger *zap.Logger,
	config *Config,
	nextConsumer consumer.Metrics,
) *ConcurrentQueryCorrelator {
	// Create the original processor
	qc := &queryCorrelator{
		logger:        logger,
		config:        config,
		nextConsumer:  nextConsumer,
		queryIndex:    boundedmap.New(config.MaxQueryCount, nil),
		tableIndex:    boundedmap.New(config.MaxTableCount, nil),
		databaseIndex: boundedmap.New(config.MaxDatabaseCount, nil),
		shutdownChan:  make(chan struct{}),
	}

	return &ConcurrentQueryCorrelator{
		queryCorrelator:     qc,
		ConcurrentProcessor: base.NewConcurrentProcessor(logger),
	}
}

// Start starts the concurrent processor
func (cqc *ConcurrentQueryCorrelator) Start(ctx context.Context, host component.Host) error {
	// Initialize base concurrent processor
	if err := cqc.ConcurrentProcessor.Start(ctx, host); err != nil {
		return err
	}

	// Start worker pools
	numWorkers := runtime.NumCPU()
	cqc.indexingWorkerPool = cqc.NewWorkerPool(numWorkers)
	cqc.indexingWorkerPool.Start()

	cqc.enrichmentWorkerPool = cqc.NewWorkerPool(numWorkers)
	cqc.enrichmentWorkerPool.Start()

	// Start cleanup loop with proper context
	cqc.StartBackgroundTask("cleanup", cqc.config.CleanupInterval, cqc.cleanupOldDataWithContext)

	cqc.logger.Info("Started concurrent query correlator processor",
		zap.Int("indexing_workers", numWorkers),
		zap.Int("enrichment_workers", numWorkers))

	return nil
}

// Shutdown stops the concurrent processor
func (cqc *ConcurrentQueryCorrelator) Shutdown(ctx context.Context) error {
	cqc.logger.Info("Shutting down concurrent query correlator processor")

	// Stop worker pools
	if cqc.indexingWorkerPool != nil {
		cqc.indexingWorkerPool.Stop()
	}
	if cqc.enrichmentWorkerPool != nil {
		cqc.enrichmentWorkerPool.Stop()
	}

	// Shutdown base concurrent processor
	return cqc.ConcurrentProcessor.Shutdown(ctx)
}

// ConsumeMetrics processes metrics concurrently
func (cqc *ConcurrentQueryCorrelator) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Check if we're shutting down
	if cqc.IsShuttingDown() {
		return nil
	}

	cqc.concurrentMetrics.metricsProcessed.Add(int64(md.DataPointCount()))

	// Phase 1: Concurrent indexing
	cqc.indexMetricsConcurrently(md)

	// Wait for indexing to complete before enrichment
	// This ensures all metrics are indexed before we start enriching
	cqc.waitForIndexingCompletion()

	// Phase 2: Concurrent enrichment
	cqc.enrichMetricsConcurrently(md)

	// Wait for enrichment to complete
	cqc.waitForEnrichmentCompletion()

	// Forward to next consumer with timeout
	return cqc.ExecuteWithContext(5*time.Second, func(ctx context.Context) error {
		return cqc.nextConsumer.ConsumeMetrics(ctx, md)
	})
}

// indexMetricsConcurrently indexes metrics using worker pool
func (cqc *ConcurrentQueryCorrelator) indexMetricsConcurrently(md pmetric.Metrics) {
	var wg sync.WaitGroup
	rms := md.ResourceMetrics()
	
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				cqc.concurrentMetrics.indexingTasks.Add(1)
				wg.Add(1)
				
				// Submit indexing task
				err := cqc.indexingWorkerPool.Submit(func() {
					defer wg.Done()
					cqc.indexMetric(metric)
				})
				
				if err != nil {
					wg.Done()
					cqc.logger.Warn("Failed to submit indexing task", zap.Error(err))
				}
			}
		}
	}
	
	// Store wait group for phase synchronization
	cqc.indexingWaitGroup = &wg
}

// enrichMetricsConcurrently enriches metrics using worker pool
func (cqc *ConcurrentQueryCorrelator) enrichMetricsConcurrently(md pmetric.Metrics) {
	var wg sync.WaitGroup
	rms := md.ResourceMetrics()
	
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				
				// Only enrich query metrics
				if cqc.isQueryMetric(metric.Name()) {
					cqc.concurrentMetrics.enrichmentTasks.Add(1)
					wg.Add(1)
					
					// Submit enrichment task
					err := cqc.enrichmentWorkerPool.Submit(func() {
						defer wg.Done()
						cqc.enrichMetric(metric)
					})
					
					if err != nil {
						wg.Done()
						cqc.logger.Warn("Failed to submit enrichment task", zap.Error(err))
					}
				}
			}
		}
	}
	
	// Store wait group for phase synchronization
	cqc.enrichmentWaitGroup = &wg
}

// waitForIndexingCompletion waits for all indexing tasks to complete
func (cqc *ConcurrentQueryCorrelator) waitForIndexingCompletion() {
	if cqc.indexingWaitGroup != nil {
		cqc.indexingWaitGroup.Wait()
		cqc.indexingWaitGroup = nil
	}
}

// waitForEnrichmentCompletion waits for all enrichment tasks to complete
func (cqc *ConcurrentQueryCorrelator) waitForEnrichmentCompletion() {
	if cqc.enrichmentWaitGroup != nil {
		cqc.enrichmentWaitGroup.Wait()
		cqc.enrichmentWaitGroup = nil
	}
}

// cleanupOldDataWithContext performs cleanup with proper context
func (cqc *ConcurrentQueryCorrelator) cleanupOldDataWithContext(ctx context.Context) error {
	cqc.cleanupOldData()
	
	// Log concurrent processing metrics
	cqc.logger.Debug("Concurrent correlation metrics",
		zap.Int64("metrics_processed", cqc.concurrentMetrics.metricsProcessed.Load()),
		zap.Int64("indexing_tasks", cqc.concurrentMetrics.indexingTasks.Load()),
		zap.Int64("enrichment_tasks", cqc.concurrentMetrics.enrichmentTasks.Load()),
		zap.Int64("correlations_created", cqc.correlationsCreated))
	
	return nil
}

// GetConcurrentMetrics returns current concurrent processing metrics
func (cqc *ConcurrentQueryCorrelator) GetConcurrentMetrics() map[string]int64 {
	return map[string]int64{
		"metrics_processed":    cqc.concurrentMetrics.metricsProcessed.Load(),
		"indexing_tasks":       cqc.concurrentMetrics.indexingTasks.Load(),
		"enrichment_tasks":     cqc.concurrentMetrics.enrichmentTasks.Load(),
		"correlations_created": cqc.correlationsCreated,
	}
}

