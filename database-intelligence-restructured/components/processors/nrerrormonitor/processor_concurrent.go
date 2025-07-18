package nrerrormonitor

import (
	"context"
	"time"

	"github.com/database-intelligence/db-intel/components/processors/base"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// ConcurrentProcessor is an improved version with proper context handling
type ConcurrentProcessor struct {
	*nrErrorMonitor                  // Embed the original processor
	*base.ConcurrentProcessor        // Embed base concurrent functionality
	errorChannel chan errorEvent     // Channel for async error processing
	workerPool   *base.WorkerPool     // Worker pool for processing
}

// errorEvent represents an error to be processed
type errorEvent struct {
	timestamp    time.Time
	apiName      string
	errorMessage string
	statusCode   int
}

// NewConcurrentProcessor creates a new concurrent error monitor processor
func NewConcurrentProcessor(
	logger *zap.Logger,
	config *Config,
	nextConsumer consumer.Metrics,
) *ConcurrentProcessor {
	// Create the original processor
	p := newNrErrorMonitor(config, logger, nextConsumer)

	return &ConcurrentProcessor{
		nrErrorMonitor:      p,
		ConcurrentProcessor: base.NewConcurrentProcessor(logger),
		errorChannel:        make(chan errorEvent, 1000), // Buffered channel
	}
}

// Start starts the concurrent processor
func (cp *ConcurrentProcessor) Start(ctx context.Context, host component.Host) error {
	// Initialize base concurrent processor
	if err := cp.ConcurrentProcessor.Start(ctx, host); err != nil {
		return err
	}

	// Start worker pool for error processing
	cp.workerPool = cp.NewWorkerPool(4) // 4 workers for error processing
	cp.workerPool.Start()

	// Start error consumer goroutine
	cp.StartBackgroundTask("error-consumer", 100*time.Millisecond, cp.consumeErrors)

	// Start monitoring loop with proper interval
	cp.StartBackgroundTask("monitoring-loop", cp.config.ReportingInterval, cp.monitorErrors)

	cp.logger.Info("Started concurrent NR error monitor processor",
		zap.Duration("reporting_interval", cp.config.ReportingInterval))

	return nil
}

// Shutdown stops the concurrent processor
func (cp *ConcurrentProcessor) Shutdown(ctx context.Context) error {
	cp.logger.Info("Shutting down concurrent NR error monitor processor")

	// Stop worker pool
	if cp.workerPool != nil {
		cp.workerPool.Stop()
	}

	// Close error channel
	close(cp.errorChannel)

	// Shutdown base concurrent processor
	return cp.ConcurrentProcessor.Shutdown(ctx)
}

// ConsumeMetrics processes metrics and extracts errors asynchronously
func (cp *ConcurrentProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Check if we're shutting down
	if cp.IsShuttingDown() {
		return nil
	}

	// Process errors asynchronously
	resourceMetrics := md.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		rm := resourceMetrics.At(i)
		scopeMetrics := rm.ScopeMetrics()
		
		for j := 0; j < scopeMetrics.Len(); j++ {
			sm := scopeMetrics.At(j)
			metrics := sm.Metrics()
			
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				cp.processMetricAsync(metric)
			}
		}
	}

	// Forward to next consumer with timeout
	return cp.ExecuteWithContext(5*time.Second, func(ctx context.Context) error {
		return cp.nextConsumer.ConsumeMetrics(ctx, md)
	})
}

// processMetricAsync processes a metric asynchronously
func (cp *ConcurrentProcessor) processMetricAsync(metric pmetric.Metric) {
	// Only process NR API metrics
	if metric.Name() != "newrelic.api.errors" && metric.Name() != "newrelic.api.requests" {
		return
	}

	// Submit to worker pool
	err := cp.workerPool.Submit(func() {
		cp.processMetricInWorker(metric)
	})
	
	if err != nil {
		cp.logger.Warn("Failed to submit metric for processing", zap.Error(err))
	}
}

// processMetricInWorker processes a metric in a worker goroutine
func (cp *ConcurrentProcessor) processMetricInWorker(metric pmetric.Metric) {
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		sum := metric.Sum()
		if sum.DataPoints().Len() == 0 {
			return
		}
		
		dp := sum.DataPoints().At(0)
		attrs := dp.Attributes()
		
		apiName, _ := attrs.Get("api.name")
		errorMsg, hasError := attrs.Get("error.message")
		statusCode, _ := attrs.Get("http.status_code")
		
		if hasError && errorMsg.Str() != "" {
			event := errorEvent{
				timestamp:    dp.Timestamp().AsTime(),
				apiName:      apiName.Str(),
				errorMessage: errorMsg.Str(),
				statusCode:   int(statusCode.Int()),
			}
			
			// Non-blocking send to channel
			select {
			case cp.errorChannel <- event:
			default:
				cp.logger.Warn("Error channel full, dropping event")
			}
		}
	}
}

// consumeErrors consumes errors from the channel
func (cp *ConcurrentProcessor) consumeErrors(ctx context.Context) error {
	consumed := 0
	for {
		select {
		case event, ok := <-cp.errorChannel:
			if !ok {
				return nil // Channel closed
			}
			
			// Update error tracker
			cp.mutex.Lock()
			tracker, exists := cp.errorCounts[event.apiName]
			if !exists {
				tracker = &errorTracker{
					category: event.apiName,
				}
				cp.errorCounts[event.apiName] = tracker
			}
			tracker.count++
			tracker.lastSeen = event.timestamp
			tracker.lastMessage = event.errorMessage
			cp.mutex.Unlock()
			
			consumed++
			
		case <-ctx.Done():
			if consumed > 0 {
				cp.logger.Debug("Consumed errors in batch", zap.Int("count", consumed))
			}
			return ctx.Err()
			
		default:
			// No more events to consume
			if consumed > 0 {
				cp.logger.Debug("Consumed errors in batch", zap.Int("count", consumed))
			}
			return nil
		}
	}
}

// monitorErrors monitors errors and generates metrics
func (cp *ConcurrentProcessor) monitorErrors(ctx context.Context) error {
	cp.logger.Debug("Running error monitoring check")

	// Get current error state
	cp.mutex.RLock()
	// Create a copy of error data
	errorData := make(map[string]*errorTracker)
	for k, v := range cp.errorCounts {
		errorData[k] = &errorTracker{
			category:    v.category,
			count:       v.count,
			lastSeen:    v.lastSeen,
			lastMessage: v.lastMessage,
			alertFired:  v.alertFired,
		}
	}
	cp.mutex.RUnlock()

	if len(errorData) == 0 {
		cp.logger.Debug("No errors to report")
		return nil
	}

	// Generate metrics
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/nrerrormonitor")

	// Error count metric
	errorMetric := sm.Metrics().AppendEmpty()
	errorMetric.SetName("newrelic.api.error.count")
	errorMetric.SetDescription("Count of New Relic API errors by category")
	errorMetric.SetUnit("errors")
	
	gauge := errorMetric.SetEmptyGauge()
	now := pcommon.NewTimestampFromTime(time.Now())

	// Add data points for each error category
	for category, tracker := range errorData {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(now)
		dp.SetIntValue(tracker.count)
		dp.Attributes().PutStr("error.category", category)
		dp.Attributes().PutStr("error.last_message", tracker.lastMessage)
		dp.Attributes().PutInt("error.minutes_since_last", int64(time.Since(tracker.lastSeen).Minutes()))
	}

	// Send metrics using the stored context
	return cp.ExecuteWithContext(5*time.Second, func(ctx context.Context) error {
		return cp.nextConsumer.ConsumeMetrics(ctx, md)
	})
}

// Capabilities returns the capabilities of the processor
func (cp *ConcurrentProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}