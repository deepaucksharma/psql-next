package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// PoolMonitor monitors connection pool health and generates metrics
type PoolMonitor struct {
	mu          sync.RWMutex
	poolManager *PoolManager
	logger      *zap.Logger
	interval    time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
	
	// Alerting thresholds
	thresholds PoolThresholds
	
	// Alert state
	alerts map[string]*PoolAlert
}

// PoolThresholds defines thresholds for pool monitoring alerts
type PoolThresholds struct {
	// Connection thresholds
	MaxConnectionUtilization float64 // Percentage (0-1)
	MinIdleConnections       int
	MaxWaitCount             int64
	MaxWaitDuration          time.Duration
	
	// Health thresholds
	MaxConsecutiveFailures   int
	UnhealthyDuration        time.Duration
	
	// Performance thresholds
	MaxAverageQueryTime      time.Duration
	MaxErrorRate             float64 // Percentage (0-1)
}

// PoolAlert represents an active alert for a pool
type PoolAlert struct {
	PoolName    string
	AlertType   string
	Message     string
	Severity    string // "warning", "critical"
	FirstSeen   time.Time
	LastSeen    time.Time
	Count       int
}

// DefaultPoolThresholds returns reasonable default thresholds
func DefaultPoolThresholds() PoolThresholds {
	return PoolThresholds{
		MaxConnectionUtilization: 0.9,  // Alert at 90% utilization
		MinIdleConnections:       1,    // Alert if no idle connections
		MaxWaitCount:             100,  // Alert after 100 waits
		MaxWaitDuration:          5 * time.Second,
		MaxConsecutiveFailures:   3,
		UnhealthyDuration:        5 * time.Minute,
		MaxAverageQueryTime:      1 * time.Second,
		MaxErrorRate:             0.05, // 5% error rate
	}
}

// NewPoolMonitor creates a new pool monitor
func NewPoolMonitor(poolManager *PoolManager, logger *zap.Logger, interval time.Duration) *PoolMonitor {
	return &PoolMonitor{
		poolManager: poolManager,
		logger:      logger,
		interval:    interval,
		stopCh:      make(chan struct{}),
		thresholds:  DefaultPoolThresholds(),
		alerts:      make(map[string]*PoolAlert),
	}
}

// Start begins monitoring
func (pm *PoolMonitor) Start() {
	pm.wg.Add(1)
	go pm.monitorLoop()
}

// Stop stops monitoring
func (pm *PoolMonitor) Stop() {
	close(pm.stopCh)
	pm.wg.Wait()
}

// monitorLoop runs the monitoring loop
func (pm *PoolMonitor) monitorLoop() {
	defer pm.wg.Done()
	
	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pm.checkPools()
		case <-pm.stopCh:
			return
		}
	}
}

// checkPools checks all pools for issues
func (pm *PoolMonitor) checkPools() {
	stats := pm.poolManager.GetAllPoolStats()
	
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Track which alerts are still active
	activeAlerts := make(map[string]bool)
	
	for _, stat := range stats {
		// Check connection utilization
		if stat.OpenConnections > 0 {
			utilization := float64(stat.InUse) / float64(stat.OpenConnections)
			if utilization > pm.thresholds.MaxConnectionUtilization {
				pm.createOrUpdateAlert(stat.Name, "high_utilization",
					fmt.Sprintf("Connection utilization %.1f%% exceeds threshold %.1f%%",
						utilization*100, pm.thresholds.MaxConnectionUtilization*100),
					"warning")
				activeAlerts[stat.Name+"_high_utilization"] = true
			}
		}
		
		// Check idle connections
		if stat.Idle < pm.thresholds.MinIdleConnections && stat.OpenConnections > 0 {
			pm.createOrUpdateAlert(stat.Name, "low_idle",
				fmt.Sprintf("Only %d idle connections, below minimum %d",
					stat.Idle, pm.thresholds.MinIdleConnections),
				"warning")
			activeAlerts[stat.Name+"_low_idle"] = true
		}
		
		// Check wait count
		if stat.WaitCount > pm.thresholds.MaxWaitCount {
			pm.createOrUpdateAlert(stat.Name, "high_wait_count",
				fmt.Sprintf("Wait count %d exceeds threshold %d",
					stat.WaitCount, pm.thresholds.MaxWaitCount),
				"warning")
			activeAlerts[stat.Name+"_high_wait_count"] = true
		}
		
		// Check health status
		if !stat.Healthy {
			pm.createOrUpdateAlert(stat.Name, "unhealthy",
				"Connection pool is unhealthy",
				"critical")
			activeAlerts[stat.Name+"_unhealthy"] = true
		}
	}
	
	// Clear resolved alerts
	for key, alert := range pm.alerts {
		if !activeAlerts[key] {
			pm.logger.Info("Pool alert resolved",
				zap.String("pool", alert.PoolName),
				zap.String("type", alert.AlertType),
				zap.Duration("duration", time.Since(alert.FirstSeen)))
			delete(pm.alerts, key)
		}
	}
}

// createOrUpdateAlert creates or updates an alert
func (pm *PoolMonitor) createOrUpdateAlert(poolName, alertType, message, severity string) {
	key := poolName + "_" + alertType
	
	if alert, exists := pm.alerts[key]; exists {
		alert.LastSeen = time.Now()
		alert.Count++
	} else {
		alert := &PoolAlert{
			PoolName:  poolName,
			AlertType: alertType,
			Message:   message,
			Severity:  severity,
			FirstSeen: time.Now(),
			LastSeen:  time.Now(),
			Count:     1,
		}
		pm.alerts[key] = alert
		
		// Log new alert
		if severity == "critical" {
			pm.logger.Error("New pool alert",
				zap.String("pool", poolName),
				zap.String("type", alertType),
				zap.String("message", message))
		} else {
			pm.logger.Warn("New pool alert",
				zap.String("pool", poolName),
				zap.String("type", alertType),
				zap.String("message", message))
		}
	}
}

// GetActiveAlerts returns currently active alerts
func (pm *PoolMonitor) GetActiveAlerts() []*PoolAlert {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	alerts := make([]*PoolAlert, 0, len(pm.alerts))
	for _, alert := range pm.alerts {
		alerts = append(alerts, alert)
	}
	
	return alerts
}

// GenerateMetrics generates OpenTelemetry metrics for connection pools
func (pm *PoolMonitor) GenerateMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	
	// Set resource attributes
	resource := rm.Resource()
	resource.Attributes().PutStr("service.name", "database-connection-pools")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/database/pools")
	
	stats := pm.poolManager.GetAllPoolStats()
	now := pcommon.NewTimestampFromTime(time.Now())
	
	// Connection metrics
	connMetric := sm.Metrics().AppendEmpty()
	connMetric.SetName("database.connection_pool.connections")
	connMetric.SetDescription("Number of connections in the pool")
	connMetric.SetUnit("connections")
	connGauge := connMetric.SetEmptyGauge()
	
	// Utilization metric
	utilMetric := sm.Metrics().AppendEmpty()
	utilMetric.SetName("database.connection_pool.utilization")
	utilMetric.SetDescription("Connection pool utilization percentage")
	utilMetric.SetUnit("percent")
	utilGauge := utilMetric.SetEmptyGauge()
	
	// Wait metrics
	waitMetric := sm.Metrics().AppendEmpty()
	waitMetric.SetName("database.connection_pool.wait_count")
	waitMetric.SetDescription("Number of times waited for a connection")
	waitMetric.SetUnit("waits")
	waitSum := waitMetric.SetEmptySum()
	waitSum.SetIsMonotonic(true)
	waitSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	// Health metric
	healthMetric := sm.Metrics().AppendEmpty()
	healthMetric.SetName("database.connection_pool.health")
	healthMetric.SetDescription("Health status of the connection pool (1=healthy, 0=unhealthy)")
	healthMetric.SetUnit("status")
	healthGauge := healthMetric.SetEmptyGauge()
	
	for _, stat := range stats {
		// Connection states
		states := []struct {
			state string
			value int64
		}{
			{"open", int64(stat.OpenConnections)},
			{"in_use", int64(stat.InUse)},
			{"idle", int64(stat.Idle)},
		}
		
		for _, s := range states {
			dp := connGauge.DataPoints().AppendEmpty()
			dp.SetTimestamp(now)
			dp.SetIntValue(s.value)
			dp.Attributes().PutStr("pool.name", stat.Name)
			dp.Attributes().PutStr("database.type", stat.DatabaseType)
			dp.Attributes().PutStr("state", s.state)
		}
		
		// Utilization
		if stat.OpenConnections > 0 {
			utilization := float64(stat.InUse) / float64(stat.OpenConnections) * 100
			dp := utilGauge.DataPoints().AppendEmpty()
			dp.SetTimestamp(now)
			dp.SetDoubleValue(utilization)
			dp.Attributes().PutStr("pool.name", stat.Name)
			dp.Attributes().PutStr("database.type", stat.DatabaseType)
		}
		
		// Wait count
		dp := waitSum.DataPoints().AppendEmpty()
		dp.SetTimestamp(now)
		dp.SetIntValue(stat.WaitCount)
		dp.Attributes().PutStr("pool.name", stat.Name)
		dp.Attributes().PutStr("database.type", stat.DatabaseType)
		
		// Health status
		healthValue := int64(0)
		if stat.Healthy {
			healthValue = 1
		}
		hdp := healthGauge.DataPoints().AppendEmpty()
		hdp.SetTimestamp(now)
		hdp.SetIntValue(healthValue)
		hdp.Attributes().PutStr("pool.name", stat.Name)
		hdp.Attributes().PutStr("database.type", stat.DatabaseType)
	}
	
	// Alert metrics
	alertMetric := sm.Metrics().AppendEmpty()
	alertMetric.SetName("database.connection_pool.alerts")
	alertMetric.SetDescription("Active connection pool alerts")
	alertMetric.SetUnit("alerts")
	alertGauge := alertMetric.SetEmptyGauge()
	
	pm.mu.RLock()
	for _, alert := range pm.alerts {
		dp := alertGauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(now)
		dp.SetIntValue(1)
		dp.Attributes().PutStr("pool.name", alert.PoolName)
		dp.Attributes().PutStr("alert.type", alert.AlertType)
		dp.Attributes().PutStr("severity", alert.Severity)
		dp.Attributes().PutStr("message", alert.Message)
		dp.Attributes().PutInt("occurrence_count", int64(alert.Count))
	}
	pm.mu.RUnlock()
	
	return md
}