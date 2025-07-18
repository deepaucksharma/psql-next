package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PoolManager manages connection pools for multiple databases
type PoolManager struct {
	mu         sync.RWMutex
	pools      map[string]*ManagedPool
	logger     *zap.Logger
	config     *PoolManagerConfig
	shutdownCh chan struct{}
	wg         sync.WaitGroup
}

// PoolManagerConfig configures the pool manager
type PoolManagerConfig struct {
	// Default pool configuration
	DefaultPoolConfig ConnectionPoolConfig
	
	// Health check interval
	HealthCheckInterval time.Duration
	
	// Metrics collection interval
	MetricsInterval time.Duration
	
	// Enable automatic pool adjustment based on load
	EnableAutoScaling bool
	
	// Maximum total connections across all pools
	GlobalMaxConnections int
}

// ManagedPool represents a managed connection pool
type ManagedPool struct {
	name       string
	client     Client
	driver     Driver
	config     *Config
	poolConfig ConnectionPoolConfig
	
	// Metrics
	mu          sync.RWMutex
	createdAt   time.Time
	lastUsed    time.Time
	totalConns  int64
	activeConns int32
	errors      int64
	
	// Health status
	healthy     bool
	lastHealthCheck time.Time
}

// NewPoolManager creates a new connection pool manager
func NewPoolManager(config *PoolManagerConfig, logger *zap.Logger) *PoolManager {
	if config == nil {
		config = &PoolManagerConfig{
			DefaultPoolConfig:   DefaultConnectionPoolConfig(),
			HealthCheckInterval: 30 * time.Second,
			MetricsInterval:     60 * time.Second,
			EnableAutoScaling:   true,
			GlobalMaxConnections: 500,
		}
	}
	
	pm := &PoolManager{
		pools:      make(map[string]*ManagedPool),
		logger:     logger,
		config:     config,
		shutdownCh: make(chan struct{}),
	}
	
	// Start background workers
	pm.wg.Add(2)
	go pm.healthCheckLoop()
	go pm.metricsLoop()
	
	return pm
}

// GetPool gets or creates a connection pool for the given configuration
func (pm *PoolManager) GetPool(ctx context.Context, name string, driver Driver, config *Config) (Client, error) {
	pm.mu.RLock()
	pool, exists := pm.pools[name]
	pm.mu.RUnlock()
	
	if exists {
		pool.mu.Lock()
		pool.lastUsed = time.Now()
		pool.mu.Unlock()
		return pool.client, nil
	}
	
	// Create new pool
	return pm.createPool(ctx, name, driver, config)
}

// createPool creates a new managed pool
func (pm *PoolManager) createPool(ctx context.Context, name string, driver Driver, config *Config) (Client, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	// Double-check after acquiring write lock
	if pool, exists := pm.pools[name]; exists {
		return pool.client, nil
	}
	
	// Apply pool configuration
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = pm.config.DefaultPoolConfig.MaxOpenConnections
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = pm.config.DefaultPoolConfig.MaxIdleConnections
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = pm.config.DefaultPoolConfig.ConnMaxLifetime
	}
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = pm.config.DefaultPoolConfig.ConnMaxIdleTime
	}
	
	// Check global connection limit
	totalConns := pm.getTotalConnections()
	if totalConns+config.MaxOpenConns > pm.config.GlobalMaxConnections {
		return nil, fmt.Errorf("would exceed global connection limit: %d + %d > %d",
			totalConns, config.MaxOpenConns, pm.config.GlobalMaxConnections)
	}
	
	// Create connection
	client, err := driver.Connect(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}
	
	// Create managed pool
	pool := &ManagedPool{
		name:       name,
		client:     client,
		driver:     driver,
		config:     config,
		poolConfig: ConnectionPoolConfig{
			MaxOpenConnections: config.MaxOpenConns,
			MaxIdleConnections: config.MaxIdleConns,
			ConnMaxLifetime:    config.ConnMaxLifetime,
			ConnMaxIdleTime:    config.ConnMaxIdleTime,
		},
		createdAt:  time.Now(),
		lastUsed:   time.Now(),
		healthy:    true,
		lastHealthCheck: time.Now(),
	}
	
	pm.pools[name] = pool
	
	pm.logger.Info("Created new connection pool",
		zap.String("name", name),
		zap.String("driver", driver.Name()),
		zap.Int("max_open", config.MaxOpenConns),
		zap.Int("max_idle", config.MaxIdleConns))
	
	return client, nil
}

// RemovePool removes a connection pool
func (pm *PoolManager) RemovePool(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pool, exists := pm.pools[name]
	if !exists {
		return fmt.Errorf("pool %s not found", name)
	}
	
	// Close the connection
	if err := pool.client.Close(); err != nil {
		pm.logger.Warn("Error closing pool connection",
			zap.String("name", name),
			zap.Error(err))
	}
	
	delete(pm.pools, name)
	
	pm.logger.Info("Removed connection pool",
		zap.String("name", name))
	
	return nil
}

// GetPoolStats returns statistics for a specific pool
func (pm *PoolManager) GetPoolStats(name string) (*PoolStats, error) {
	pm.mu.RLock()
	pool, exists := pm.pools[name]
	pm.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("pool %s not found", name)
	}
	
	pool.mu.RLock()
	defer pool.mu.RUnlock()
	
	stats := pool.client.Stats()
	
	return &PoolStats{
		Name:              name,
		DatabaseType:      pool.client.DatabaseType(),
		CreatedAt:         pool.createdAt,
		LastUsed:          pool.lastUsed,
		Healthy:           pool.healthy,
		LastHealthCheck:   pool.lastHealthCheck,
		OpenConnections:   stats.OpenConnections,
		InUse:             stats.InUse,
		Idle:              stats.Idle,
		WaitCount:         stats.WaitCount,
		WaitDuration:      stats.WaitDuration,
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxIdleTimeClosed: stats.MaxIdleTimeClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
		TotalConnections:  pool.totalConns,
		ActiveConnections: int(pool.activeConns),
		Errors:            pool.errors,
	}, nil
}

// GetAllPoolStats returns statistics for all pools
func (pm *PoolManager) GetAllPoolStats() []*PoolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	stats := make([]*PoolStats, 0, len(pm.pools))
	
	for name, pool := range pm.pools {
		pool.mu.RLock()
		clientStats := pool.client.Stats()
		
		poolStats := &PoolStats{
			Name:              name,
			DatabaseType:      pool.client.DatabaseType(),
			CreatedAt:         pool.createdAt,
			LastUsed:          pool.lastUsed,
			Healthy:           pool.healthy,
			LastHealthCheck:   pool.lastHealthCheck,
			OpenConnections:   clientStats.OpenConnections,
			InUse:             clientStats.InUse,
			Idle:              clientStats.Idle,
			WaitCount:         clientStats.WaitCount,
			WaitDuration:      clientStats.WaitDuration,
			MaxIdleClosed:     clientStats.MaxIdleClosed,
			MaxIdleTimeClosed: clientStats.MaxIdleTimeClosed,
			MaxLifetimeClosed: clientStats.MaxLifetimeClosed,
			TotalConnections:  pool.totalConns,
			ActiveConnections: int(pool.activeConns),
			Errors:            pool.errors,
		}
		pool.mu.RUnlock()
		
		stats = append(stats, poolStats)
	}
	
	return stats
}

// healthCheckLoop performs periodic health checks on all pools
func (pm *PoolManager) healthCheckLoop() {
	defer pm.wg.Done()
	
	ticker := time.NewTicker(pm.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pm.performHealthChecks()
		case <-pm.shutdownCh:
			return
		}
	}
}

// performHealthChecks checks the health of all pools
func (pm *PoolManager) performHealthChecks() {
	pm.mu.RLock()
	pools := make([]*ManagedPool, 0, len(pm.pools))
	for _, pool := range pm.pools {
		pools = append(pools, pool)
	}
	pm.mu.RUnlock()
	
	for _, pool := range pools {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := pool.client.Ping(ctx)
		cancel()
		
		pool.mu.Lock()
		pool.lastHealthCheck = time.Now()
		pool.healthy = err == nil
		if err != nil {
			pool.errors++
			pm.logger.Warn("Pool health check failed",
				zap.String("name", pool.name),
				zap.Error(err))
		}
		pool.mu.Unlock()
	}
}

// metricsLoop collects and logs metrics periodically
func (pm *PoolManager) metricsLoop() {
	defer pm.wg.Done()
	
	ticker := time.NewTicker(pm.config.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pm.collectMetrics()
		case <-pm.shutdownCh:
			return
		}
	}
}

// collectMetrics collects metrics from all pools
func (pm *PoolManager) collectMetrics() {
	stats := pm.GetAllPoolStats()
	
	totalOpen := 0
	totalInUse := 0
	totalIdle := 0
	unhealthyPools := 0
	
	for _, stat := range stats {
		totalOpen += stat.OpenConnections
		totalInUse += stat.InUse
		totalIdle += stat.Idle
		if !stat.Healthy {
			unhealthyPools++
		}
	}
	
	pm.logger.Info("Connection pool metrics",
		zap.Int("total_pools", len(stats)),
		zap.Int("total_open", totalOpen),
		zap.Int("total_in_use", totalInUse),
		zap.Int("total_idle", totalIdle),
		zap.Int("unhealthy_pools", unhealthyPools))
	
	// Auto-scale pools if enabled
	if pm.config.EnableAutoScaling {
		pm.autoScalePools(stats)
	}
}

// autoScalePools adjusts pool sizes based on usage
func (pm *PoolManager) autoScalePools(stats []*PoolStats) {
	for _, stat := range stats {
		utilization := float64(stat.InUse) / float64(stat.OpenConnections)
		
		// Scale up if utilization is high
		if utilization > 0.8 && stat.WaitCount > 0 {
			pm.scalePool(stat.Name, true)
		}
		// Scale down if utilization is low
		if utilization < 0.2 && stat.OpenConnections > 5 {
			pm.scalePool(stat.Name, false)
		}
	}
}

// scalePool adjusts the size of a pool
func (pm *PoolManager) scalePool(name string, scaleUp bool) {
	pm.mu.RLock()
	pool, exists := pm.pools[name]
	pm.mu.RUnlock()
	
	if !exists {
		return
	}
	
	// This is a placeholder - actual implementation would depend on the database driver
	// supporting dynamic pool resizing
	if scaleUp {
		pm.logger.Info("Would scale up pool",
			zap.String("name", name),
			zap.Int("current_size", pool.poolConfig.MaxOpenConnections))
	} else {
		pm.logger.Info("Would scale down pool",
			zap.String("name", name),
			zap.Int("current_size", pool.poolConfig.MaxOpenConnections))
	}
}

// getTotalConnections returns the total number of connections across all pools
func (pm *PoolManager) getTotalConnections() int {
	total := 0
	for _, pool := range pm.pools {
		total += pool.poolConfig.MaxOpenConnections
	}
	return total
}

// Shutdown gracefully shuts down the pool manager
func (pm *PoolManager) Shutdown() error {
	close(pm.shutdownCh)
	pm.wg.Wait()
	
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	var errors []error
	
	// Close all pools
	for name, pool := range pm.pools {
		if err := pool.client.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close pool %s: %w", name, err))
		}
	}
	
	// Clear pools
	pm.pools = make(map[string]*ManagedPool)
	
	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}
	
	return nil
}

// PoolStats represents statistics for a connection pool
type PoolStats struct {
	Name              string
	DatabaseType      string
	CreatedAt         time.Time
	LastUsed          time.Time
	Healthy           bool
	LastHealthCheck   time.Time
	OpenConnections   int
	InUse             int
	Idle              int
	WaitCount         int64
	WaitDuration      time.Duration
	MaxIdleClosed     int64
	MaxIdleTimeClosed int64
	MaxLifetimeClosed int64
	TotalConnections  int64
	ActiveConnections int
	Errors            int64
}