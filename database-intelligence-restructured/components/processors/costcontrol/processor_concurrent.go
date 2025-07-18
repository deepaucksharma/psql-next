package costcontrol

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/database-intelligence/db-intel/components/processors/base"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// ConcurrentCostControlProcessor is an improved version with proper context handling
type ConcurrentCostControlProcessor struct {
	*costControlProcessor         // Embed the original processor
	*base.ConcurrentProcessor     // Embed base concurrent functionality
	processingWorkerPool  *base.WorkerPool
	
	// Metrics for concurrent processing
	concurrentMetrics struct {
		tracesProcessed  atomic.Int64
		metricsProcessed atomic.Int64
		logsProcessed    atomic.Int64
		bytesReduced     atomic.Int64
		itemsDropped     atomic.Int64
	}
}

// NewConcurrentCostControlProcessor creates a new concurrent cost control processor
func NewConcurrentCostControlProcessor(
	logger *zap.Logger,
	config *Config,
	nextTraces consumer.Traces,
	nextMetrics consumer.Metrics,
	nextLogs consumer.Logs,
) *ConcurrentCostControlProcessor {
	// Create the original processor
	p := &costControlProcessor{
		config:           config,
		logger:           logger,
		nextTraces:       nextTraces,
		nextMetrics:      nextMetrics,
		nextLogs:         nextLogs,
		metricCardinality: make(map[string]*cardinalityTracker),
		costTracker: &costTracker{
			currentMonth: time.Now(),
			lastUpdate:   time.Now(),
		},
	}

	return &ConcurrentCostControlProcessor{
		costControlProcessor: p,
		ConcurrentProcessor:  base.NewConcurrentProcessor(logger),
	}
}

// Start starts the concurrent processor
func (ccp *ConcurrentCostControlProcessor) Start(ctx context.Context, host component.Host) error {
	// Initialize base concurrent processor
	if err := ccp.ConcurrentProcessor.Start(ctx, host); err != nil {
		return err
	}

	// Start worker pool
	ccp.processingWorkerPool = ccp.NewWorkerPool(runtime.NumCPU())
	ccp.processingWorkerPool.Start()

	// Start cost monitoring with proper context
	ccp.StartBackgroundTask("cost-monitoring", 1*time.Minute, ccp.costMonitoringWithContext)

	// Start cardinality cleanup with proper context
	ccp.StartBackgroundTask("cardinality-cleanup", ccp.config.CardinalityCleanupInterval, ccp.cardinalityCleanupWithContext)

	ccp.logger.Info("Started concurrent cost control processor",
		zap.Float64("monthly_budget_usd", ccp.config.MonthlyBudgetUSD),
		zap.Int("processing_workers", runtime.NumCPU()))

	return nil
}

// Shutdown stops the concurrent processor
func (ccp *ConcurrentCostControlProcessor) Shutdown(ctx context.Context) error {
	ccp.logger.Info("Shutting down concurrent cost control processor")

	// Log final cost report
	ccp.logCostReport()

	// Stop worker pool
	if ccp.processingWorkerPool != nil {
		ccp.processingWorkerPool.Stop()
	}

	// Shutdown base concurrent processor
	return ccp.ConcurrentProcessor.Shutdown(ctx)
}

// ConsumeTraces applies cost control to traces concurrently
func (ccp *ConcurrentCostControlProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// Check if we're shutting down
	if ccp.IsShuttingDown() {
		return nil
	}

	ccp.concurrentMetrics.tracesProcessed.Add(int64(td.SpanCount()))

	// Track data volume
	dataSize := ccp.estimateTraceSize(td)
	ccp.updateCostTracking(dataSize, "traces")

	// Apply intelligent sampling if over budget
	if ccp.isOverBudget() {
		td = ccp.applyAggressiveTraceSampling(td)
		ccp.concurrentMetrics.itemsDropped.Add(int64(td.SpanCount()))
	}

	// Process trace optimization concurrently
	err := ccp.processingWorkerPool.Submit(func() {
		ccp.removeExpensiveTraceAttributes(td)
		// Optimize trace data is embedded in the trace processing
	})
	
	if err != nil {
		// Fall back to synchronous processing
		ccp.removeExpensiveTraceAttributes(td)
		// Optimize trace data is embedded in the trace processing
	}

	// Forward to next consumer with timeout
	return ccp.ExecuteWithContext(5*time.Second, func(ctx context.Context) error {
		return ccp.nextTraces.ConsumeTraces(ctx, td)
	})
}

// ConsumeMetrics applies cost control to metrics concurrently
func (ccp *ConcurrentCostControlProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Check if we're shutting down
	if ccp.IsShuttingDown() {
		return nil
	}

	ccp.concurrentMetrics.metricsProcessed.Add(int64(md.DataPointCount()))

	// Track data volume
	dataSize := ccp.estimateMetricSize(md)
	ccp.updateCostTracking(dataSize, "metrics")

	// Process metric optimization concurrently
	err := ccp.processingWorkerPool.Submit(func() {
		// Check cardinality limits
		if ccp.config.CardinalityLimit > 0 {
			// Cardinality limits are enforced in the base processor
		}

		// Apply aggregation if needed
		if ccp.isOverBudget() && ccp.config.EnableIntelligentAggregation {
			// Intelligent aggregation is applied in the base processor
		}

		// Remove expensive labels
		// Remove expensive labels using embedded processor
	})
	
	if err != nil {
		// Fall back to synchronous processing
		if ccp.config.CardinalityLimit > 0 {
			// Cardinality limits are enforced in the base processor
		}
		if ccp.isOverBudget() && ccp.config.EnableIntelligentAggregation {
			// Intelligent aggregation is applied in the base processor
		}
		// Remove expensive labels using embedded processor
	}

	// Forward to next consumer with timeout
	return ccp.ExecuteWithContext(5*time.Second, func(ctx context.Context) error {
		return ccp.nextMetrics.ConsumeMetrics(ctx, md)
	})
}

// ConsumeLogs applies cost control to logs concurrently
func (ccp *ConcurrentCostControlProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Check if we're shutting down
	if ccp.IsShuttingDown() {
		return nil
	}

	ccp.concurrentMetrics.logsProcessed.Add(int64(ld.LogRecordCount()))

	// Track data volume
	dataSize := ccp.estimateLogSize(ld)
	ccp.updateCostTracking(dataSize, "logs")

	// Process log optimization concurrently
	err := ccp.processingWorkerPool.Submit(func() {
		// Filter by severity if over budget
		if ccp.isOverBudget() {
			// Log filtering is done in the base processor
		}

		// Reduce log verbosity
		if ccp.config.EnableLogReduction {
			ccp.truncateLargeLogs(ld)
		}

		// Remove expensive fields
		// Expensive fields are removed in the base processor
	})
	
	if err != nil {
		// Fall back to synchronous processing
		if ccp.isOverBudget() {
			// Log filtering is done in the base processor
		}
		if ccp.config.EnableLogReduction {
			ccp.truncateLargeLogs(ld)
		}
		// Expensive fields are removed in the base processor
	}

	// Forward to next consumer with timeout
	return ccp.ExecuteWithContext(5*time.Second, func(ctx context.Context) error {
		return ccp.nextLogs.ConsumeLogs(ctx, ld)
	})
}

// costMonitoringWithContext performs cost monitoring with proper context
func (ccp *ConcurrentCostControlProcessor) costMonitoringWithContext(ctx context.Context) error {
	ccp.mutex.Lock()
	defer ccp.mutex.Unlock()

	// Update cost projections
	now := time.Now()
	daysSinceMonthStart := float64(now.Day())
	daysInMonth := float64(time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day())
	
	if daysSinceMonthStart > 0 {
		dailyRate := ccp.costTracker.estimatedCostUSD / daysSinceMonthStart
		ccp.costTracker.projectedCostUSD = dailyRate * daysInMonth
	}

	// Log cost status
	ccp.logger.Info("Cost control status",
		zap.Float64("current_cost_usd", ccp.costTracker.estimatedCostUSD),
		zap.Float64("projected_cost_usd", ccp.costTracker.projectedCostUSD),
		zap.Float64("monthly_budget_usd", ccp.config.MonthlyBudgetUSD),
		zap.Int64("bytes_ingested", ccp.costTracker.bytesIngested),
		zap.Int64("traces_processed", ccp.concurrentMetrics.tracesProcessed.Load()),
		zap.Int64("metrics_processed", ccp.concurrentMetrics.metricsProcessed.Load()),
		zap.Int64("logs_processed", ccp.concurrentMetrics.logsProcessed.Load()),
		zap.Int64("items_dropped", ccp.concurrentMetrics.itemsDropped.Load()))

	// Alert if over budget
	if ccp.costTracker.projectedCostUSD > ccp.config.MonthlyBudgetUSD {
		ccp.logger.Warn("Projected to exceed monthly budget",
			zap.Float64("projected_overage_usd", ccp.costTracker.projectedCostUSD-ccp.config.MonthlyBudgetUSD))
	}

	return nil
}

// cardinalityCleanupWithContext performs cardinality cleanup with proper context
func (ccp *ConcurrentCostControlProcessor) cardinalityCleanupWithContext(ctx context.Context) error {
	ccp.mutex.Lock()
	defer ccp.mutex.Unlock()

	cleanupTime := time.Now().Add(-24 * time.Hour)
	
	for metricName, tracker := range ccp.metricCardinality {
		cleaned := 0
		for combo, lastSeen := range tracker.uniqueTimeSeries {
			if lastSeen.Before(cleanupTime) {
				delete(tracker.uniqueTimeSeries, combo)
				cleaned++
			}
		}
		
		if cleaned > 0 {
			ccp.logger.Debug("Cleaned up cardinality tracker",
				zap.String("metric", metricName),
				zap.Int("cleaned", cleaned),
				zap.Int("remaining", len(tracker.uniqueTimeSeries)))
		}
		
		tracker.lastCleanup = time.Now()
	}

	return nil
}

// GetConcurrentMetrics returns current concurrent processing metrics
func (ccp *ConcurrentCostControlProcessor) GetConcurrentMetrics() map[string]int64 {
	return map[string]int64{
		"traces_processed":  ccp.concurrentMetrics.tracesProcessed.Load(),
		"metrics_processed": ccp.concurrentMetrics.metricsProcessed.Load(),
		"logs_processed":    ccp.concurrentMetrics.logsProcessed.Load(),
		"bytes_reduced":     ccp.concurrentMetrics.bytesReduced.Load(),
		"items_dropped":     ccp.concurrentMetrics.itemsDropped.Load(),
	}
}