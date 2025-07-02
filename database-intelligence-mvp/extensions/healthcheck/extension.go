// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// HealthCheckExtension provides comprehensive health checking with verification integration
type HealthCheckExtension struct {
	config           *Config
	logger           *zap.Logger
	server           *http.Server
	healthStatus     *HealthStatus
	verificationAPI  *VerificationAPI
	shutdownChan     chan struct{}
	wg              sync.WaitGroup
}

// HealthStatus tracks overall system health
type HealthStatus struct {
	mu                    sync.RWMutex
	Status                string                     `json:"status"`
	LastCheck             time.Time                  `json:"last_check"`
	CollectorHealth       ComponentHealth            `json:"collector"`
	DataIngestion         DataIngestionHealth        `json:"data_ingestion"`
	NewRelicIntegration   NewRelicIntegrationHealth  `json:"newrelic_integration"`
	DatabaseConnections   map[string]DatabaseHealth  `json:"databases"`
	VerificationMetrics   VerificationMetrics        `json:"verification"`
}

// ComponentHealth tracks collector component health
type ComponentHealth struct {
	Healthy    bool      `json:"healthy"`
	Uptime     string    `json:"uptime"`
	StartTime  time.Time `json:"start_time"`
	Version    string    `json:"version"`
}

// DataIngestionHealth tracks data flow health
type DataIngestionHealth struct {
	RecordsProcessed      int64     `json:"records_processed"`
	LastDataReceived      time.Time `json:"last_data_received"`
	DataFreshness         string    `json:"data_freshness"`
	ProcessingRate        float64   `json:"processing_rate"`
}

// NewRelicIntegrationHealth tracks NR integration health
type NewRelicIntegrationHealth struct {
	Connected             bool      `json:"connected"`
	LastSuccessfulExport  time.Time `json:"last_successful_export"`
	IntegrationErrors     int64     `json:"integration_errors"`
	ExportSuccessRate     float64   `json:"export_success_rate"`
	EntitySynthesisRate   float64   `json:"entity_synthesis_rate"`
}

// DatabaseHealth tracks per-database health
type DatabaseHealth struct {
	Connected         bool      `json:"connected"`
	LastQuery         time.Time `json:"last_query"`
	CircuitBreakerState string  `json:"circuit_breaker_state"`
	ErrorRate         float64   `json:"error_rate"`
	AverageLatency    float64   `json:"average_latency_ms"`
}

// VerificationMetrics from the verification processor
type VerificationMetrics struct {
	EntityCorrelationRate  float64            `json:"entity_correlation_rate"`
	QueryNormalizationRate float64            `json:"query_normalization_rate"`
	CardinalityWarnings    int64              `json:"cardinality_warnings"`
	RemediationSuggestions []string           `json:"remediation_suggestions"`
}

// VerificationAPI provides access to verification data
type VerificationAPI struct {
	mu              sync.RWMutex
	latestReport    map[string]interface{}
	feedbackHistory []FeedbackEvent
}

// FeedbackEvent from verification processor
type FeedbackEvent struct {
	Timestamp   time.Time              `json:"timestamp"`
	Level       string                 `json:"level"`
	Category    string                 `json:"category"`
	Message     string                 `json:"message"`
	Database    string                 `json:"database,omitempty"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
	Remediation string                 `json:"remediation,omitempty"`
}

// newHealthCheckExtension creates a new health check extension
func newHealthCheckExtension(cfg *Config, logger *zap.Logger) (*HealthCheckExtension, error) {
	hce := &HealthCheckExtension{
		config:       cfg,
		logger:       logger,
		healthStatus: &HealthStatus{
			Status:              "initializing",
			DatabaseConnections: make(map[string]DatabaseHealth),
		},
		verificationAPI: &VerificationAPI{
			feedbackHistory: make([]FeedbackEvent, 0, 1000),
		},
		shutdownChan: make(chan struct{}),
	}
	
	// Initialize health status
	hce.healthStatus.CollectorHealth.StartTime = time.Now()
	hce.healthStatus.CollectorHealth.Version = "2.1.0"
	
	return hce, nil
}

// Start implements the extension.Extension interface
func (hce *HealthCheckExtension) Start(ctx context.Context, host component.Host) error {
	hce.logger.Info("Starting health check extension")
	
	// Create HTTP server
	mux := http.NewServeMux()
	
	// Register endpoints
	mux.HandleFunc("/health", hce.handleHealth)
	mux.HandleFunc("/health/detailed", hce.handleDetailedHealth)
	mux.HandleFunc("/health/verification", hce.handleVerification)
	mux.HandleFunc("/health/feedback", hce.handleFeedbackHistory)
	mux.HandleFunc("/health/remediation", hce.handleRemediation)
	
	hce.server = &http.Server{
		Addr:    hce.config.Endpoint,
		Handler: mux,
	}
	
	// Start server
	hce.wg.Add(1)
	go func() {
		defer hce.wg.Done()
		if err := hce.server.ListenAndServe(); err != http.ErrServerClosed {
							hce.logger.Error("Health check server error", zap.Error(err))

		}
	}()
	
	// Start periodic health checks
	hce.wg.Add(1)
	go hce.runPeriodicHealthChecks()
	
	// Update status
	hce.healthStatus.mu.Lock()
	hce.healthStatus.Status = "healthy"
	hce.healthStatus.CollectorHealth.Healthy = true
	hce.healthStatus.mu.Unlock()
	
	return nil
}

// Shutdown implements the extension.Extension interface
func (hce *HealthCheckExtension) Shutdown(ctx context.Context) error {
	hce.logger.Info("Shutting down health check extension")
	
	close(hce.shutdownChan)
	
	if hce.server != nil {
		if err := hce.server.Shutdown(ctx); err != nil {
			hce.logger.Error("Error shutting down health server", zap.Error(err))
		}
	}
	
	hce.wg.Wait()
	return nil
}

// HTTP Handlers

func (hce *HealthCheckExtension) handleHealth(w http.ResponseWriter, r *http.Request) {
	hce.healthStatus.mu.RLock()
	status := hce.healthStatus.Status
	hce.healthStatus.mu.RUnlock()
	
	if status == "healthy" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(status))
	}
}

func (hce *HealthCheckExtension) handleDetailedHealth(w http.ResponseWriter, r *http.Request) {
	hce.healthStatus.mu.RLock()
	
	// Calculate uptime
	uptime := time.Since(hce.healthStatus.CollectorHealth.StartTime)
	hce.healthStatus.CollectorHealth.Uptime = uptime.String()
	
	// Calculate data freshness
	if !hce.healthStatus.DataIngestion.LastDataReceived.IsZero() {
		freshness := time.Since(hce.healthStatus.DataIngestion.LastDataReceived)
		hce.healthStatus.DataIngestion.DataFreshness = freshness.String()
	}
	
	// Create response
	response, err := json.MarshalIndent(hce.healthStatus, "", "  ")
	hce.healthStatus.mu.RUnlock()
	
	if err != nil {
		http.Error(w, "Failed to generate health status", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (hce *HealthCheckExtension) handleVerification(w http.ResponseWriter, r *http.Request) {
	hce.verificationAPI.mu.RLock()
	response, err := json.MarshalIndent(hce.verificationAPI.latestReport, "", "  ")
	hce.verificationAPI.mu.RUnlock()
	
	if err != nil {
		http.Error(w, "Failed to generate verification report", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (hce *HealthCheckExtension) handleFeedbackHistory(w http.ResponseWriter, r *http.Request) {
	hce.verificationAPI.mu.RLock()
	
	// Get last 100 feedback events
	start := 0
	if len(hce.verificationAPI.feedbackHistory) > 100 {
		start = len(hce.verificationAPI.feedbackHistory) - 100
	}
	
	recent := hce.verificationAPI.feedbackHistory[start:]
	response, err := json.MarshalIndent(recent, "", "  ")
	hce.verificationAPI.mu.RUnlock()
	
	if err != nil {
		http.Error(w, "Failed to generate feedback history", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func (hce *HealthCheckExtension) handleRemediation(w http.ResponseWriter, r *http.Request) {
	hce.healthStatus.mu.RLock()
	
	remediations := []map[string]string{}
	
	// Check for issues and provide remediation
	if hce.healthStatus.NewRelicIntegration.IntegrationErrors > 0 {
		remediations = append(remediations, map[string]string{
			"issue":       "Integration errors detected",
			"remediation": "Check NrIntegrationError events in New Relic",
			"query":       "SELECT * FROM NrIntegrationError WHERE newRelicFeature = 'Metrics' SINCE 1 hour ago",
		})
	}
	
	if hce.healthStatus.NewRelicIntegration.EntitySynthesisRate < 0.8 {
		remediations = append(remediations, map[string]string{
			"issue":       "Low entity synthesis rate",
			"remediation": "Review entity synthesis processor configuration",
			"action":      "Ensure entity.guid, entity.type, and service.name attributes are set",
		})
	}
	
	// Check database health
	for dbName, dbHealth := range hce.healthStatus.DatabaseConnections {
		if dbHealth.CircuitBreakerState == "open" {
			remediations = append(remediations, map[string]string{
				"issue":       fmt.Sprintf("Circuit breaker open for database %s", dbName),
				"remediation": "Check database connectivity and query performance",
				"action":      "Review slow queries and connection pool settings",
			})
		}
		
		if dbHealth.ErrorRate > 0.1 {
			remediations = append(remediations, map[string]string{
				"issue":       fmt.Sprintf("High error rate for database %s: %.2f%%", dbName, dbHealth.ErrorRate*100),
				"remediation": "Investigate query errors and timeouts",
				"action":      "Check database logs and permissions",
			})
		}
	}
	
	hce.healthStatus.mu.RUnlock()
	
	response, _ := json.MarshalIndent(map[string]interface{}{
		"timestamp":    time.Now(),
		"remediations": remediations,
	}, "", "  ")
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

// Periodic health checks
func (hce *HealthCheckExtension) runPeriodicHealthChecks() {
	defer hce.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			hce.performHealthCheck()
		case <-hce.shutdownChan:
			return
		}
	}
}

func (hce *HealthCheckExtension) performHealthCheck() {
	hce.healthStatus.mu.Lock()
	defer hce.healthStatus.mu.Unlock()
	
	hce.healthStatus.LastCheck = time.Now()
	
	// Determine overall health status
	overallHealthy := true
	
	// Check data freshness
	if !hce.healthStatus.DataIngestion.LastDataReceived.IsZero() {
		freshness := time.Since(hce.healthStatus.DataIngestion.LastDataReceived)
		if freshness > 10*time.Minute {
			overallHealthy = false
			hce.healthStatus.Status = "degraded - no recent data"
		}
	}
	
	// Check integration errors
	if hce.healthStatus.NewRelicIntegration.IntegrationErrors > 10 {
		overallHealthy = false
		hce.healthStatus.Status = "degraded - integration errors"
	}
	
	// Check export success rate
	if hce.healthStatus.NewRelicIntegration.ExportSuccessRate < 0.95 {
		overallHealthy = false
		hce.healthStatus.Status = "degraded - low export success rate"
	}
	
	// Check database health
	for _, dbHealth := range hce.healthStatus.DatabaseConnections {
		if dbHealth.CircuitBreakerState == "open" {
			overallHealthy = false
			hce.healthStatus.Status = "degraded - circuit breaker open"
			break
		}
		if dbHealth.ErrorRate > 0.2 {
			overallHealthy = false
			hce.healthStatus.Status = "degraded - high database error rate"
			break
		}
	}
	
	if overallHealthy {
		hce.healthStatus.Status = "healthy"
	}
}

// UpdateVerificationReport updates the latest verification report
func (hce *HealthCheckExtension) UpdateVerificationReport(report map[string]interface{}) {
	hce.verificationAPI.mu.Lock()
	hce.verificationAPI.latestReport = report
	hce.verificationAPI.mu.Unlock()
	
	// Update health status with verification metrics
	if metrics, ok := report["verification"].(VerificationMetrics); ok {
		hce.healthStatus.mu.Lock()
		hce.healthStatus.VerificationMetrics = metrics
		hce.healthStatus.mu.Unlock()
	}
}

// AddFeedbackEvent adds a feedback event to history
func (hce *HealthCheckExtension) AddFeedbackEvent(event FeedbackEvent) {
	hce.verificationAPI.mu.Lock()
	hce.verificationAPI.feedbackHistory = append(hce.verificationAPI.feedbackHistory, event)
	
	// Keep only last 1000 events
	if len(hce.verificationAPI.feedbackHistory) > 1000 {
		hce.verificationAPI.feedbackHistory = hce.verificationAPI.feedbackHistory[100:]
	}
	hce.verificationAPI.mu.Unlock()
	
	// Update health metrics based on feedback
	hce.updateHealthFromFeedback(event)
}

func (hce *HealthCheckExtension) updateHealthFromFeedback(event FeedbackEvent) {
	hce.healthStatus.mu.Lock()
	defer hce.healthStatus.mu.Unlock()
	
	// Update based on feedback category
	switch event.Category {
	case "data_freshness":
		if event.Level == "ERROR" {
			hce.healthStatus.DataIngestion.DataFreshness = "stale"
		}
		
	case "integration_errors":
		if event.Level == "ERROR" {
			hce.healthStatus.NewRelicIntegration.IntegrationErrors++
		}
		
	case "entity_synthesis":
		if rate, ok := event.Metrics["correlation_rate"].(float64); ok {
			hce.healthStatus.NewRelicIntegration.EntitySynthesisRate = rate
		}
		
	case "circuit_breaker":
		if event.Database != "" {
			db := hce.healthStatus.DatabaseConnections[event.Database]
			db.CircuitBreakerState = "open"
			hce.healthStatus.DatabaseConnections[event.Database] = db
		}
		
	case "health_report":
		// Process periodic health report
		if metrics, ok := event.Metrics["verification"].(map[string]interface{}); ok {
			if entityRate, ok := metrics["entity_correlation_rate"].(float64); ok {
				hce.healthStatus.VerificationMetrics.EntityCorrelationRate = entityRate
			}
			if normRate, ok := metrics["query_normalization_rate"].(float64); ok {
				hce.healthStatus.VerificationMetrics.QueryNormalizationRate = normRate
			}
		}
	}
}