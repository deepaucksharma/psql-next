// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package verification

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

// VerificationProcessor provides real-time feedback on data quality and integration health
type VerificationProcessor struct {
	logger           *zap.Logger
	nextConsumer     consumer.Logs
	config           *Config
	metrics          *VerificationMetrics
	feedbackChannel  chan FeedbackEvent
	shutdownChan     chan struct{}
	wg              sync.WaitGroup
}

// VerificationMetrics tracks integration health metrics
type VerificationMetrics struct {
	mu                     sync.RWMutex
	recordsProcessed       int64
	entitiesCreated        int64
	errorsDetected         int64
	cardinalityWarnings    int64
	lastDataTimestamp      time.Time
	entityCorrelationRate  float64
	queryNormalizationRate float64
	databaseMetrics        map[string]*DatabaseMetrics
}

// DatabaseMetrics tracks per-database metrics
type DatabaseMetrics struct {
	recordCount           int64
	lastSeen             time.Time
	entityCorrelationRate float64
	averageQueryDuration  float64
	circuitBreakerState   string
}

// FeedbackEvent represents a verification feedback event
type FeedbackEvent struct {
	Timestamp   time.Time               `json:"timestamp"`
	Level       string                  `json:"level"`
	Category    string                  `json:"category"`
	Message     string                  `json:"message"`
	Database    string                  `json:"database,omitempty"`
	Metrics     map[string]interface{}  `json:"metrics,omitempty"`
	Remediation string                  `json:"remediation,omitempty"`
}

// newVerificationProcessor creates a new verification processor
func newVerificationProcessor(
	logger *zap.Logger,
	config *Config,
	nextConsumer consumer.Logs,
) (*VerificationProcessor, error) {
	
	vp := &VerificationProcessor{
		logger:          logger,
		nextConsumer:    nextConsumer,
		config:          config,
		metrics:         &VerificationMetrics{
			databaseMetrics: make(map[string]*DatabaseMetrics),
		},
		feedbackChannel: make(chan FeedbackEvent, 1000),
		shutdownChan:    make(chan struct{}),
	}
	
	// Start feedback processor
	vp.wg.Add(1)
	go vp.processFeedback()
	
	// Start periodic verification
	if config.EnablePeriodicVerification {
		vp.wg.Add(1)
		go vp.periodicVerification()
	}
	
	return vp, nil
}

// Start implements the component.Component interface
func (vp *VerificationProcessor) Start(ctx context.Context, host component.Host) error {
	vp.logger.Info("Starting verification processor")
	return nil
}

// Shutdown implements the component.Component interface
func (vp *VerificationProcessor) Shutdown(ctx context.Context) error {
	vp.logger.Info("Shutting down verification processor")
	close(vp.shutdownChan)
	vp.wg.Wait()
	close(vp.feedbackChannel)
	return nil
}

// ConsumeLogs implements the consumer.Logs interface
func (vp *VerificationProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// Process logs and collect verification metrics
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resource := rl.Resource()
		
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				vp.verifyLogRecord(resource, lr)
			}
		}
	}
	
	// Check for issues and generate feedback
	vp.checkIntegrationHealth()
	
	// Pass to next consumer
	return vp.nextConsumer.ConsumeLogs(ctx, ld)
}

// verifyLogRecord performs verification on a single log record
func (vp *VerificationProcessor) verifyLogRecord(resource plog.Resource, lr plog.LogRecord) {
	vp.metrics.mu.Lock()
	defer vp.metrics.mu.Unlock()
	
	vp.metrics.recordsProcessed++
	vp.metrics.lastDataTimestamp = time.Now()
	
	attrs := lr.Attributes()
	
	// Extract database name
	dbName := ""
	if db, ok := attrs.Get("database_name"); ok {
		dbName = db.Str()
	}
	
	// Initialize database metrics if needed
	if dbName != "" {
		if _, exists := vp.metrics.databaseMetrics[dbName]; !exists {
			vp.metrics.databaseMetrics[dbName] = &DatabaseMetrics{}
		}
		dbMetrics := vp.metrics.databaseMetrics[dbName]
		dbMetrics.recordCount++
		dbMetrics.lastSeen = time.Now()
		
		// Check query duration
		if duration, ok := attrs.Get("duration_ms"); ok {
			dbMetrics.averageQueryDuration = 
				(dbMetrics.averageQueryDuration*float64(dbMetrics.recordCount-1) + duration.Double()) / 
				float64(dbMetrics.recordCount)
		}
	}
	
	// Verify entity synthesis attributes
	hasEntityGuid := false
	hasEntityType := false
	hasServiceName := false
	hasHostId := false
	
	if _, ok := attrs.Get("entity.guid"); ok {
		hasEntityGuid = true
		vp.metrics.entitiesCreated++
	}
	if _, ok := attrs.Get("entity.type"); ok {
		hasEntityType = true
	}
	if _, ok := attrs.Get("service.name"); ok {
		hasServiceName = true
	}
	if _, ok := attrs.Get("host.id"); ok {
		hasHostId = true
	}
	
	// Calculate entity correlation rate
	if dbName != "" {
		dbMetrics := vp.metrics.databaseMetrics[dbName]
		if hasEntityGuid {
			dbMetrics.entityCorrelationRate = 
				(dbMetrics.entityCorrelationRate*float64(dbMetrics.recordCount-1) + 1) / 
				float64(dbMetrics.recordCount)
		} else {
			dbMetrics.entityCorrelationRate = 
				(dbMetrics.entityCorrelationRate*float64(dbMetrics.recordCount-1)) / 
				float64(dbMetrics.recordCount)
		}
	}
	
	// Check for missing critical attributes
	if !hasEntityGuid && vp.config.RequireEntitySynthesis {
		vp.sendFeedback(FeedbackEvent{
			Timestamp: time.Now(),
			Level:     "WARNING",
			Category:  "entity_synthesis",
			Database:  dbName,
			Message:   "Missing entity.guid attribute for proper New Relic entity correlation",
			Remediation: "Ensure resource/entity_synthesis processor is configured correctly",
		})
	}
	
	// Verify query normalization
	if _, hasNormalized := attrs.Get("db.query.normalized"); hasNormalized {
		if _, hasFingerprint := attrs.Get("db.query.fingerprint"); hasFingerprint {
			vp.metrics.queryNormalizationRate = 
				(vp.metrics.queryNormalizationRate*float64(vp.metrics.recordsProcessed-1) + 1) / 
				float64(vp.metrics.recordsProcessed)
		}
	}
	
	// Check for circuit breaker state
	if cbState, ok := attrs.Get("cb.state"); ok {
		if dbName != "" {
			vp.metrics.databaseMetrics[dbName].circuitBreakerState = cbState.Str()
			
			if cbState.Str() == "open" {
				vp.sendFeedback(FeedbackEvent{
					Timestamp: time.Now(),
					Level:     "ERROR",
					Category:  "circuit_breaker",
					Database:  dbName,
					Message:   fmt.Sprintf("Circuit breaker OPEN for database %s", dbName),
					Remediation: "Check database connectivity and query performance",
				})
			}
		}
	}
}

// checkIntegrationHealth performs periodic health checks
func (vp *VerificationProcessor) checkIntegrationHealth() {
	vp.metrics.mu.RLock()
	defer vp.metrics.mu.RUnlock()
	
	// Check data freshness
	if time.Since(vp.metrics.lastDataTimestamp) > vp.config.DataFreshnessThreshold {
		vp.sendFeedback(FeedbackEvent{
			Timestamp: time.Now(),
			Level:     "ERROR",
			Category:  "data_freshness",
			Message:   fmt.Sprintf("No data received for %v", time.Since(vp.metrics.lastDataTimestamp)),
			Remediation: "Check collector configuration and database connectivity",
		})
	}
	
	// Check entity correlation rate
	totalCorrelation := 0.0
	dbCount := 0
	for dbName, metrics := range vp.metrics.databaseMetrics {
		if metrics.recordCount > 0 {
			totalCorrelation += metrics.entityCorrelationRate
			dbCount++
			
			// Alert on low correlation rates
			if metrics.entityCorrelationRate < vp.config.MinEntityCorrelationRate {
				vp.sendFeedback(FeedbackEvent{
					Timestamp: time.Now(),
					Level:     "WARNING",
					Category:  "entity_correlation",
					Database:  dbName,
					Message:   fmt.Sprintf("Low entity correlation rate: %.2f%%", 
						metrics.entityCorrelationRate*100),
					Metrics: map[string]interface{}{
						"correlation_rate": metrics.entityCorrelationRate,
						"record_count":     metrics.recordCount,
					},
					Remediation: "Review entity synthesis processor configuration",
				})
			}
		}
	}
	
	// Update overall correlation rate
	if dbCount > 0 {
		vp.metrics.entityCorrelationRate = totalCorrelation / float64(dbCount)
	}
	
	// Check query normalization effectiveness
	if vp.metrics.queryNormalizationRate < vp.config.MinNormalizationRate {
		vp.sendFeedback(FeedbackEvent{
			Timestamp: time.Now(),
			Level:     "WARNING",
			Category:  "cardinality_management",
			Message:   fmt.Sprintf("Low query normalization rate: %.2f%%", 
				vp.metrics.queryNormalizationRate*100),
			Remediation: "Check transform/query_normalization processor configuration",
		})
		vp.metrics.cardinalityWarnings++
	}
}

// periodicVerification runs periodic health checks
func (vp *VerificationProcessor) periodicVerification() {
	defer vp.wg.Done()
	
	ticker := time.NewTicker(vp.config.VerificationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			vp.generateHealthReport()
		case <-vp.shutdownChan:
			return
		}
	}
}

// generateHealthReport creates a comprehensive health report
func (vp *VerificationProcessor) generateHealthReport() {
	vp.metrics.mu.RLock()
	defer vp.metrics.mu.RUnlock()
	
	report := map[string]interface{}{
		"timestamp":               time.Now(),
		"records_processed":       vp.metrics.recordsProcessed,
		"entities_created":        vp.metrics.entitiesCreated,
		"errors_detected":         vp.metrics.errorsDetected,
		"cardinality_warnings":    vp.metrics.cardinalityWarnings,
		"entity_correlation_rate": vp.metrics.entityCorrelationRate,
		"query_normalization_rate": vp.metrics.queryNormalizationRate,
		"databases":               make(map[string]interface{}),
	}
	
	// Add per-database metrics
	for dbName, metrics := range vp.metrics.databaseMetrics {
		report["databases"].(map[string]interface{})[dbName] = map[string]interface{}{
			"record_count":           metrics.recordCount,
			"last_seen":              metrics.lastSeen,
			"entity_correlation_rate": metrics.entityCorrelationRate,
			"average_query_duration": metrics.averageQueryDuration,
			"circuit_breaker_state":  metrics.circuitBreakerState,
		}
	}
	
	// Log the report
	reportJSON, _ := json.MarshalIndent(report, "", "  ")
	vp.logger.Info("Verification health report", 
		zap.String("report", string(reportJSON)))
	
	// Send as feedback event
	vp.sendFeedback(FeedbackEvent{
		Timestamp: time.Now(),
		Level:     "INFO",
		Category:  "health_report",
		Message:   "Periodic health report generated",
		Metrics:   report,
	})
}

// sendFeedback sends a feedback event
func (vp *VerificationProcessor) sendFeedback(event FeedbackEvent) {
	select {
	case vp.feedbackChannel <- event:
	default:
		vp.logger.Warn("Feedback channel full, dropping event")
	}
}

// processFeedback handles feedback events
func (vp *VerificationProcessor) processFeedback() {
	defer vp.wg.Done()
	
	for {
		select {
		case event := <-vp.feedbackChannel:
			// Log the feedback
			vp.logger.Info("Verification feedback",
				zap.String("level", event.Level),
				zap.String("category", event.Category),
				zap.String("message", event.Message),
				zap.String("database", event.Database),
				zap.String("remediation", event.Remediation),
			)
			
			// Update error counter
			if event.Level == "ERROR" {
				vp.metrics.mu.Lock()
				vp.metrics.errorsDetected++
				vp.metrics.mu.Unlock()
			}
			
			// Export feedback as telemetry
			if vp.config.ExportFeedbackAsLogs {
				vp.exportFeedbackEvent(event)
			}
			
		case <-vp.shutdownChan:
			return
		}
	}
}

// exportFeedbackEvent exports feedback as telemetry
func (vp *VerificationProcessor) exportFeedbackEvent(event FeedbackEvent) {
	// Create a log record for the feedback event
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	
	// Set resource attributes
	rl.Resource().Attributes().PutStr("service.name", "database-intelligence-verification")
	rl.Resource().Attributes().PutStr("feedback.category", event.Category)
	
	// Create log record
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("verification_processor")
	
	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(plog.Timestamp(event.Timestamp.UnixNano()))
	lr.SetSeverityText(event.Level)
	lr.Body().SetStr(event.Message)
	
	// Add attributes
	lr.Attributes().PutStr("feedback.level", event.Level)
	lr.Attributes().PutStr("feedback.category", event.Category)
	if event.Database != "" {
		lr.Attributes().PutStr("database_name", event.Database)
	}
	if event.Remediation != "" {
		lr.Attributes().PutStr("feedback.remediation", event.Remediation)
	}
	
	// Add metrics as attributes
	for k, v := range event.Metrics {
		switch val := v.(type) {
		case float64:
			lr.Attributes().PutDouble(fmt.Sprintf("feedback.metrics.%s", k), val)
		case int64:
			lr.Attributes().PutInt(fmt.Sprintf("feedback.metrics.%s", k), val)
		case string:
			lr.Attributes().PutStr(fmt.Sprintf("feedback.metrics.%s", k), val)
		}
	}
	
	// Send to next consumer
	ctx := context.Background()
	if err := vp.nextConsumer.ConsumeLogs(ctx, ld); err != nil {
		vp.logger.Error("Failed to export feedback event", zap.Error(err))
	}
}