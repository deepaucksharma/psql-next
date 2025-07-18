// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package verification

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/database-intelligence/db-intel/components/processors/base"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// ConcurrentVerificationProcessor is an improved version with proper context handling
type ConcurrentVerificationProcessor struct {
	*VerificationProcessor         // Embed the original processor
	*base.ConcurrentProcessor      // Embed base concurrent functionality
	verificationWorkerPool *base.WorkerPool
	piiDetectionWorkerPool *base.WorkerPool
	
	// Metrics for concurrent processing
	concurrentMetrics struct {
		logsProcessed     atomic.Int64
		piiChecksQueued   atomic.Int64
		piiChecksComplete atomic.Int64
		verificationTasks atomic.Int64
	}
}

// NewConcurrentVerificationProcessor creates a new concurrent verification processor
func NewConcurrentVerificationProcessor(
	logger *zap.Logger,
	config *Config,
	nextConsumer consumer.Logs,
) (*ConcurrentVerificationProcessor, error) {
	// Create the original processor
	vp, err := newVerificationProcessor(logger, config, nextConsumer)
	if err != nil {
		return nil, err
	}

	return &ConcurrentVerificationProcessor{
		VerificationProcessor: vp,
		ConcurrentProcessor:  base.NewConcurrentProcessor(logger),
	}, nil
}

// Start starts the concurrent processor
func (cvp *ConcurrentVerificationProcessor) Start(ctx context.Context, host component.Host) error {
	// Initialize base concurrent processor
	if err := cvp.ConcurrentProcessor.Start(ctx, host); err != nil {
		return err
	}

	// Start worker pools
	cvp.verificationWorkerPool = cvp.NewWorkerPool(runtime.NumCPU())
	cvp.verificationWorkerPool.Start()

	if cvp.config.PIIDetection.Enabled {
		cvp.piiDetectionWorkerPool = cvp.NewWorkerPool(4) // 4 workers for PII detection
		cvp.piiDetectionWorkerPool.Start()
	}

	// Start feedback processor with proper context
	cvp.StartBackgroundTask("feedback-processor", 100*time.Millisecond, cvp.processFeedbackWithContext)

	// Start periodic verification if enabled
	if cvp.config.EnablePeriodicVerification {
		cvp.StartBackgroundTask("periodic-verification", cvp.config.VerificationInterval, cvp.performVerificationWithContext)
	}

	// Start continuous health checks if enabled
	if cvp.config.EnableContinuousHealthChecks {
		cvp.StartBackgroundTask("health-checks", cvp.config.HealthCheckInterval, cvp.performHealthCheckWithContext)
	}

	// Start resource monitoring
	cvp.StartBackgroundTask("resource-monitoring", 30*time.Second, cvp.updateSystemMetricsWithContext)

	cvp.logger.Info("Started concurrent verification processor",
		zap.Int("verification_workers", runtime.NumCPU()),
		zap.Bool("pii_detection", cvp.config.PIIDetection.Enabled))

	return nil
}

// Shutdown stops the concurrent processor
func (cvp *ConcurrentVerificationProcessor) Shutdown(ctx context.Context) error {
	cvp.logger.Info("Shutting down concurrent verification processor")

	// Stop worker pools
	if cvp.verificationWorkerPool != nil {
		cvp.verificationWorkerPool.Stop()
	}
	if cvp.piiDetectionWorkerPool != nil {
		cvp.piiDetectionWorkerPool.Stop()
	}

	// Close feedback channel
	close(cvp.feedbackChannel)

	// Shutdown base concurrent processor
	return cvp.ConcurrentProcessor.Shutdown(ctx)
}

// ConsumeLogs processes logs concurrently
func (cvp *ConcurrentVerificationProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Check if we're shutting down
	if cvp.IsShuttingDown() {
		return nil
	}

	startTime := time.Now()
	cvp.concurrentMetrics.logsProcessed.Add(int64(ld.LogRecordCount()))

	// Process logs concurrently
	rls := ld.ResourceLogs()
	tasks := make([]func(context.Context) error, 0, rls.Len())

	for i := 0; i < rls.Len(); i++ {
		rlIndex := i
		tasks = append(tasks, func(ctx context.Context) error {
			rl := rls.At(rlIndex)
			return cvp.processResourceLogs(ctx, rl)
		})
	}

	// Run all resource processing tasks concurrently
	if err := cvp.RunConcurrent(ctx, tasks...); err != nil {
		cvp.logger.Warn("Error processing resource logs", zap.Error(err))
	}

	// Forward to next consumer with timeout
	err := cvp.ExecuteWithContext(5*time.Second, func(ctx context.Context) error {
		return cvp.nextConsumer.ConsumeLogs(ctx, ld)
	})

	// Track performance
	cvp.performanceTracker.mu.Lock()
	cvp.performanceTracker.totalLatency += time.Since(startTime)
	if err != nil {
		cvp.performanceTracker.errorCount++
	}
	cvp.performanceTracker.mu.Unlock()

	return err
}

// processResourceLogs processes logs for a single resource
func (cvp *ConcurrentVerificationProcessor) processResourceLogs(ctx context.Context, rl plog.ResourceLogs) error {
	resource := rl.Resource()
	
	// Verify resource attributes
	cvp.verifyResourceAttributes(resource.Attributes())
	
	// Process scope logs
	sls := rl.ScopeLogs()
	for j := 0; j < sls.Len(); j++ {
		sl := sls.At(j)
		logs := sl.LogRecords()
		
		// Submit log verification tasks to worker pool
		for k := 0; k < logs.Len(); k++ {
			logIndex := k
			cvp.concurrentMetrics.verificationTasks.Add(1)
			
			err := cvp.verificationWorkerPool.Submit(func() {
				log := logs.At(logIndex)
				if err := cvp.verifyLogRecordConcurrent(log); err != nil {
					cvp.logger.Debug("Log verification failed", zap.Error(err))
					cvp.metrics.mu.Lock()
					cvp.metrics.errorsDetected++
					cvp.metrics.mu.Unlock()
				}
			})
			
			if err != nil {
				cvp.logger.Warn("Failed to submit verification task", zap.Error(err))
			}
		}
	}
	
	return nil
}

// verifyLogRecordConcurrent verifies a log record with concurrent PII detection
func (cvp *ConcurrentVerificationProcessor) verifyLogRecordConcurrent(log plog.LogRecord) error {
	attrs := log.Attributes()
	
	// Check required fields
	missing := cvp.checkRequiredFields(attrs)
	if len(missing) > 0 {
		cvp.sendFeedback(FeedbackEvent{
			Timestamp: time.Now(),
			Level:     "WARNING",
			Category:  "missing_fields",
			Message:   fmt.Sprintf("Missing required fields: %v", missing),
			Severity:  6,
		})
	}
	
	// Submit PII detection to separate worker pool if enabled
	if cvp.config.PIIDetection.Enabled && cvp.piiDetectionWorkerPool != nil {
		cvp.concurrentMetrics.piiChecksQueued.Add(1)
		
		// Create a copy of attributes for async PII detection
		attrsCopy := pcommon.NewMap()
		attrs.CopyTo(attrsCopy)
		
		err := cvp.piiDetectionWorkerPool.Submit(func() {
			cvp.detectPIIAsync(attrsCopy, attrs)
			cvp.concurrentMetrics.piiChecksComplete.Add(1)
		})
		
		if err != nil {
			// Fall back to synchronous PII detection if worker pool is full
			cvp.detectPII(attrs)
		}
	}
	
	// Validate data quality
	cvp.validateDataQuality(attrs)
	
	// Check cardinality
	cvp.checkCardinality(attrs)
	
	return nil
}

// detectPIIAsync performs PII detection asynchronously
func (cvp *ConcurrentVerificationProcessor) detectPIIAsync(attrsCopy pcommon.Map, originalAttrs pcommon.Map) {
	attrsCopy.Range(func(key string, value pcommon.Value) bool {
		// Skip excluded fields
		for _, exclude := range cvp.config.PIIDetection.ExcludeFields {
			if key == exclude {
				return true
			}
		}
		
		// Check common PII field names
		for _, piiField := range cvp.piiDetector.commonPIIFields {
			if strings.Contains(strings.ToLower(key), piiField) {
				cvp.sendFeedback(FeedbackEvent{
					Timestamp: time.Now(),
					Level:     "WARNING",
					Category:  "pii_detected",
					Message:   fmt.Sprintf("Potential PII in field: %s", key),
					Severity:  8,
				})
				
				if cvp.config.PIIDetection.AutoSanitize {
					// Sanitize in the original attributes
					originalAttrs.PutStr(key, "[REDACTED]")
				}
			}
		}
		
		// Check PII patterns in values
		if value.Type() == pcommon.ValueTypeStr {
			for _, pattern := range cvp.piiDetector.patterns {
				if pattern.MatchString(value.Str()) {
					cvp.sendFeedback(FeedbackEvent{
						Timestamp: time.Now(),
						Level:     "WARNING",
						Category:  "pii_pattern_detected",
						Message:   fmt.Sprintf("PII pattern detected in field: %s", key),
						Severity:  8,
					})
					
					if cvp.config.PIIDetection.AutoSanitize {
						// Sanitize in the original attributes
						originalAttrs.PutStr(key, pattern.ReplaceAllString(value.Str(), "[REDACTED]"))
					}
				}
			}
		}
		
		return true
	})
}

// processFeedbackWithContext processes feedback events with context
func (cvp *ConcurrentVerificationProcessor) processFeedbackWithContext(ctx context.Context) error {
	processed := 0
	for {
		select {
		case event := <-cvp.feedbackChannel:
			// Export as log if configured
			if cvp.config.ExportFeedbackAsLogs {
				cvp.exportFeedbackAsLogWithContext(ctx, event)
			}
			
			// Log locally
			switch event.Level {
			case "ERROR":
				cvp.logger.Error(event.Message,
					zap.String("category", event.Category),
					zap.Int("severity", event.Severity))
			case "WARNING":
				cvp.logger.Warn(event.Message,
					zap.String("category", event.Category),
					zap.Int("severity", event.Severity))
			default:
				cvp.logger.Info(event.Message,
					zap.String("category", event.Category),
					zap.Int("severity", event.Severity))
			}
			
			processed++
			
		case <-ctx.Done():
			if processed > 0 {
				cvp.logger.Debug("Processed feedback events", zap.Int("count", processed))
			}
			return ctx.Err()
			
		default:
			// No more events to process
			if processed > 0 {
				cvp.logger.Debug("Processed feedback events", zap.Int("count", processed))
			}
			return nil
		}
	}
}

// exportFeedbackAsLogWithContext exports feedback with proper context
func (cvp *ConcurrentVerificationProcessor) exportFeedbackAsLogWithContext(ctx context.Context, event FeedbackEvent) {
	// Create a new log record
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	
	// Set resource attributes
	resource := rl.Resource()
	resource.Attributes().PutStr("service.name", "database-intelligence-verification")
	resource.Attributes().PutStr("service.version", "2.0.0")
	
	// Create log record
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	
	// Set log attributes
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(event.Timestamp))
	logRecord.SetSeverityNumber(plog.SeverityNumber(event.Severity))
	logRecord.SetSeverityText(event.Level)
	
	// Set body
	body, _ := json.Marshal(event)
	logRecord.Body().SetStr(string(body))
	
	// Set attributes
	attrs := logRecord.Attributes()
	attrs.PutStr("feedback.category", event.Category)
	attrs.PutStr("feedback.level", event.Level)
	if event.Database != "" {
		attrs.PutStr("database.name", event.Database)
	}
	
	// Send to export with the provided context
	if err := cvp.nextConsumer.ConsumeLogs(ctx, logs); err != nil {
		cvp.logger.Error("Failed to export feedback as log", zap.Error(err))
	}
}

// performVerificationWithContext performs verification with proper context
func (cvp *ConcurrentVerificationProcessor) performVerificationWithContext(ctx context.Context) error {
	cvp.performVerification()
	return nil
}

// performHealthCheckWithContext performs health check with proper context
func (cvp *ConcurrentVerificationProcessor) performHealthCheckWithContext(ctx context.Context) error {
	cvp.performHealthCheck()
	return nil
}

// updateSystemMetricsWithContext updates system metrics with proper context
func (cvp *ConcurrentVerificationProcessor) updateSystemMetricsWithContext(ctx context.Context) error {
	cvp.updateSystemMetrics()
	cvp.updatePerformanceMetrics()
	
	// Log concurrent processing metrics
	cvp.logger.Debug("Concurrent processing metrics",
		zap.Int64("logs_processed", cvp.concurrentMetrics.logsProcessed.Load()),
		zap.Int64("pii_checks_queued", cvp.concurrentMetrics.piiChecksQueued.Load()),
		zap.Int64("pii_checks_complete", cvp.concurrentMetrics.piiChecksComplete.Load()),
		zap.Int64("verification_tasks", cvp.concurrentMetrics.verificationTasks.Load()))
	
	return nil
}

// GetConcurrentMetrics returns current concurrent processing metrics
func (cvp *ConcurrentVerificationProcessor) GetConcurrentMetrics() map[string]int64 {
	return map[string]int64{
		"logs_processed":      cvp.concurrentMetrics.logsProcessed.Load(),
		"pii_checks_queued":   cvp.concurrentMetrics.piiChecksQueued.Load(),
		"pii_checks_complete": cvp.concurrentMetrics.piiChecksComplete.Load(),
		"verification_tasks":  cvp.concurrentMetrics.verificationTasks.Load(),
	}
}