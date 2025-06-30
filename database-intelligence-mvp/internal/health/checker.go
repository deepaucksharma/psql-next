package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ComponentHealth represents the health status of a single component
type ComponentHealth struct {
	Name        string                 `json:"name"`
	Healthy     bool                   `json:"healthy"`
	LastChecked time.Time              `json:"last_checked"`
	Message     string                 `json:"message,omitempty"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
}

// HealthStatus represents the overall health of the collector
type HealthStatus struct {
	Healthy           bool                       `json:"healthy"`
	Timestamp         time.Time                  `json:"timestamp"`
	Version           string                     `json:"version"`
	Uptime            time.Duration              `json:"uptime"`
	Components        map[string]ComponentHealth `json:"components"`
	ResourceUsage     ResourceMetrics            `json:"resource_usage"`
	PipelineStatus    map[string]PipelineHealth  `json:"pipeline_status"`
}

// ResourceMetrics contains resource usage information
type ResourceMetrics struct {
	MemoryUsageMB   float64 `json:"memory_usage_mb"`
	MemoryLimitMB   float64 `json:"memory_limit_mb"`
	CPUUsagePercent float64 `json:"cpu_usage_percent"`
	GoroutineCount  int     `json:"goroutine_count"`
}

// PipelineHealth represents the health of a single pipeline
type PipelineHealth struct {
	Name             string  `json:"name"`
	Healthy          bool    `json:"healthy"`
	RecordsProcessed int64   `json:"records_processed"`
	RecordsDropped   int64   `json:"records_dropped"`
	ErrorRate        float64 `json:"error_rate"`
	Latency          float64 `json:"latency_ms"`
}

// HealthCheckable interface for components that can report health
type HealthCheckable interface {
	IsHealthy() bool
	GetHealthMetrics() map[string]interface{}
}

// HealthChecker manages health checks for all components
type HealthChecker struct {
	logger         *zap.Logger
	components     map[string]HealthCheckable
	startTime      time.Time
	version        string
	checkInterval  time.Duration
	mu             sync.RWMutex
	
	// Cached health status
	lastStatus     *HealthStatus
	lastCheck      time.Time
	
	// Resource monitor
	resourceMonitor *ResourceMonitor
	
	// Pipeline monitors
	pipelineMonitors map[string]*PipelineMonitor
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger *zap.Logger, version string) *HealthChecker {
	return &HealthChecker{
		logger:           logger,
		components:       make(map[string]HealthCheckable),
		startTime:        time.Now(),
		version:          version,
		checkInterval:    5 * time.Second,
		resourceMonitor:  NewResourceMonitor(),
		pipelineMonitors: make(map[string]*PipelineMonitor),
	}
}

// RegisterComponent registers a component for health checking
func (hc *HealthChecker) RegisterComponent(name string, component HealthCheckable) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.components[name] = component
	hc.logger.Info("Registered component for health check", zap.String("component", name))
}

// RegisterPipeline registers a pipeline for monitoring
func (hc *HealthChecker) RegisterPipeline(name string, monitor *PipelineMonitor) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	hc.pipelineMonitors[name] = monitor
	hc.logger.Info("Registered pipeline for health check", zap.String("pipeline", name))
}

// CheckHealth performs a comprehensive health check
func (hc *HealthChecker) CheckHealth(ctx context.Context) *HealthStatus {
	hc.mu.RLock()
	
	// Return cached status if recent
	if time.Since(hc.lastCheck) < hc.checkInterval && hc.lastStatus != nil {
		hc.mu.RUnlock()
		return hc.lastStatus
	}
	hc.mu.RUnlock()
	
	// Perform new health check
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	status := &HealthStatus{
		Healthy:        true,
		Timestamp:      time.Now(),
		Version:        hc.version,
		Uptime:         time.Since(hc.startTime),
		Components:     make(map[string]ComponentHealth),
		PipelineStatus: make(map[string]PipelineHealth),
	}
	
	// Check each component
	for name, component := range hc.components {
		compHealth := ComponentHealth{
			Name:        name,
			Healthy:     component.IsHealthy(),
			LastChecked: time.Now(),
			Metrics:     component.GetHealthMetrics(),
		}
		
		if !compHealth.Healthy {
			status.Healthy = false
			compHealth.Message = "Component is unhealthy"
		}
		
		status.Components[name] = compHealth
	}
	
	// Check resource usage
	status.ResourceUsage = hc.resourceMonitor.GetMetrics()
	
	// Check if resources are within limits
	if status.ResourceUsage.MemoryUsageMB > status.ResourceUsage.MemoryLimitMB*0.9 {
		status.Healthy = false
	}
	if status.ResourceUsage.CPUUsagePercent > 90 {
		status.Healthy = false
	}
	
	// Check pipeline health
	for name, monitor := range hc.pipelineMonitors {
		pipelineHealth := monitor.GetHealth()
		status.PipelineStatus[name] = pipelineHealth
		
		if !pipelineHealth.Healthy {
			status.Healthy = false
		}
	}
	
	// Cache the status
	hc.lastStatus = status
	hc.lastCheck = time.Now()
	
	return status
}

// LivenessHandler returns an HTTP handler for liveness checks
func (hc *HealthChecker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Simple liveness check - are we running?
		response := map[string]interface{}{
			"status": "alive",
			"uptime": time.Since(hc.startTime).Seconds(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// ReadinessHandler returns an HTTP handler for readiness checks
func (hc *HealthChecker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status := hc.CheckHealth(ctx)
		
		// Determine HTTP status code
		httpStatus := http.StatusOK
		if !status.Healthy {
			httpStatus = http.StatusServiceUnavailable
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(status)
	}
}

// DetailedHealthHandler returns an HTTP handler for detailed health information
func (hc *HealthChecker) DetailedHealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status := hc.CheckHealth(ctx)
		
		// Add additional debugging information
		debugInfo := struct {
			*HealthStatus
			Debug DebugInfo `json:"debug"`
		}{
			HealthStatus: status,
			Debug: DebugInfo{
				ConfiguredComponents: len(hc.components),
				ConfiguredPipelines:  len(hc.pipelineMonitors),
				LastCheckDuration:    time.Since(hc.lastCheck).Seconds(),
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(debugInfo)
	}
}

// DebugInfo contains additional debugging information
type DebugInfo struct {
	ConfiguredComponents int     `json:"configured_components"`
	ConfiguredPipelines  int     `json:"configured_pipelines"`
	LastCheckDuration    float64 `json:"last_check_duration_seconds"`
}

// StartBackgroundCheck starts periodic health checks
func (hc *HealthChecker) StartBackgroundCheck(ctx context.Context) {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := hc.CheckHealth(ctx)
			
			// Log if health status changed
			if hc.lastStatus != nil && hc.lastStatus.Healthy != status.Healthy {
				if status.Healthy {
					hc.logger.Info("System health recovered")
				} else {
					hc.logger.Warn("System health degraded", 
						zap.Any("status", status))
				}
			}
		}
	}
}

// PipelineMonitor monitors a single pipeline's health
type PipelineMonitor struct {
	name             string
	recordsProcessed int64
	recordsDropped   int64
	errors           int64
	lastProcessTime  time.Time
	mu               sync.RWMutex
}

// NewPipelineMonitor creates a new pipeline monitor
func NewPipelineMonitor(name string) *PipelineMonitor {
	return &PipelineMonitor{
		name:            name,
		lastProcessTime: time.Now(),
	}
}

// RecordProcessed increments the processed counter
func (pm *PipelineMonitor) RecordProcessed(count int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.recordsProcessed += count
	pm.lastProcessTime = time.Now()
}

// RecordDropped increments the dropped counter
func (pm *PipelineMonitor) RecordDropped(count int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.recordsDropped += count
}

// RecordError increments the error counter
func (pm *PipelineMonitor) RecordError() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.errors++
}

// GetHealth returns the pipeline health status
func (pm *PipelineMonitor) GetHealth() PipelineHealth {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	total := pm.recordsProcessed + pm.recordsDropped
	errorRate := float64(0)
	if total > 0 {
		errorRate = float64(pm.errors) / float64(total) * 100
	}
	
	// Consider unhealthy if no processing in last 5 minutes
	healthy := time.Since(pm.lastProcessTime) < 5*time.Minute
	
	// Also unhealthy if error rate > 5%
	if errorRate > 5.0 {
		healthy = false
	}
	
	return PipelineHealth{
		Name:             pm.name,
		Healthy:          healthy,
		RecordsProcessed: pm.recordsProcessed,
		RecordsDropped:   pm.recordsDropped,
		ErrorRate:        errorRate,
		Latency:          0, // Would need histogram for this
	}
}

// ResourceMonitor monitors system resources
type ResourceMonitor struct {
	// In a real implementation, this would use runtime and system metrics
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{}
}

// GetMetrics returns current resource metrics
func (rm *ResourceMonitor) GetMetrics() ResourceMetrics {
	// Simplified implementation - in production would use actual metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	return ResourceMetrics{
		MemoryUsageMB:   float64(m.Alloc) / 1024 / 1024,
		MemoryLimitMB:   float64(m.Sys) / 1024 / 1024,
		CPUUsagePercent: 0, // Would need proper CPU monitoring
		GoroutineCount:  runtime.NumGoroutine(),
	}
}

// Add missing import
var runtime = struct {
	MemStats       func(*runtime.MemStats)
	ReadMemStats   func(*runtime.MemStats)
	NumGoroutine   func() int
}{
	// Mock implementation for example
}

// MemStats represents memory statistics
type MemStats struct {
	Alloc uint64
	Sys   uint64
}