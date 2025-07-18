package enhancedsql

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/database-intelligence/db-intel/internal/database"
	"go.uber.org/zap"
)

// PoolAdapter manages database connections with pooling for the enhanced SQL receiver
type PoolAdapter struct {
	mu          sync.RWMutex
	poolManager *database.PoolManager
	logger      *zap.Logger
	config      *Config
}

// NewPoolAdapter creates a new pool adapter
func NewPoolAdapter(config *Config, logger *zap.Logger) *PoolAdapter {
	poolConfig := &database.PoolManagerConfig{
		DefaultPoolConfig: database.ConnectionPoolConfig{
			MaxOpenConnections: config.MaxOpenConnections,
			MaxIdleConnections: config.MaxIdleConnections,
			ConnMaxLifetime:    5 * time.Minute,
			ConnMaxIdleTime:    5 * time.Minute,
		},
		HealthCheckInterval:  30 * time.Second,
		MetricsInterval:      60 * time.Second,
		EnableAutoScaling:    true,
		GlobalMaxConnections: 100,
	}
	
	// Use defaults if not specified
	if poolConfig.DefaultPoolConfig.MaxOpenConnections == 0 {
		poolConfig.DefaultPoolConfig = database.DefaultConnectionPoolConfig()
	}
	
	return &PoolAdapter{
		poolManager: database.NewPoolManager(poolConfig, logger),
		logger:      logger,
		config:      config,
	}
}

// GetConnection gets or creates a pooled connection
func (pa *PoolAdapter) GetConnection(ctx context.Context) (database.Client, error) {
	// Parse datasource to get configuration
	driver, err := database.GetDriver(pa.config.Driver)
	if err != nil {
		return nil, fmt.Errorf("unknown driver %s: %w", pa.config.Driver, err)
	}
	
	// Parse the datasource
	dbConfig, err := driver.ParseDSN(pa.config.Datasource)
	if err != nil {
		return nil, fmt.Errorf("failed to parse datasource: %w", err)
	}
	
	// Apply pool settings from receiver config
	if pa.config.MaxOpenConnections > 0 {
		dbConfig.MaxOpenConns = pa.config.MaxOpenConnections
	}
	if pa.config.MaxIdleConnections > 0 {
		dbConfig.MaxIdleConns = pa.config.MaxIdleConnections
	}
	
	// Generate pool name based on database connection
	poolName := fmt.Sprintf("%s_%s_%d_%s", 
		pa.config.Driver, 
		dbConfig.Host, 
		dbConfig.Port, 
		dbConfig.Database)
	
	// Get or create connection from pool
	return pa.poolManager.GetPool(ctx, poolName, driver, dbConfig)
}

// GetPoolStats returns statistics for all managed pools
func (pa *PoolAdapter) GetPoolStats() []*database.PoolStats {
	return pa.poolManager.GetAllPoolStats()
}

// LogPoolMetrics logs metrics for all pools
func (pa *PoolAdapter) LogPoolMetrics() {
	stats := pa.GetPoolStats()
	
	for _, stat := range stats {
		pa.logger.Info("Connection pool metrics",
			zap.String("pool", stat.Name),
			zap.String("database_type", stat.DatabaseType),
			zap.Bool("healthy", stat.Healthy),
			zap.Int("open_connections", stat.OpenConnections),
			zap.Int("in_use", stat.InUse),
			zap.Int("idle", stat.Idle),
			zap.Int64("wait_count", stat.WaitCount),
			zap.Duration("wait_duration", stat.WaitDuration))
		
		// Log warnings for unhealthy pools
		if !stat.Healthy {
			pa.logger.Warn("Unhealthy connection pool detected",
				zap.String("pool", stat.Name),
				zap.Time("last_health_check", stat.LastHealthCheck))
		}
		
		// Log warnings for high wait counts
		if stat.WaitCount > 100 {
			pa.logger.Warn("High connection wait count",
				zap.String("pool", stat.Name),
				zap.Int64("wait_count", stat.WaitCount),
				zap.Duration("total_wait_duration", stat.WaitDuration))
		}
	}
}

// Shutdown gracefully shuts down the pool adapter
func (pa *PoolAdapter) Shutdown() error {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	
	// Log final pool statistics
	pa.LogPoolMetrics()
	
	// Shutdown pool manager
	return pa.poolManager.Shutdown()
}

// ExecuteQuery executes a query using a pooled connection
func (pa *PoolAdapter) ExecuteQuery(ctx context.Context, query string, args ...interface{}) (database.Rows, error) {
	client, err := pa.GetConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	
	// Note: Connection is managed by the pool, no need to close
	return client.Query(ctx, query, args...)
}

// GetDatabaseType returns the database type from the configuration
func (pa *PoolAdapter) GetDatabaseType() string {
	switch pa.config.Driver {
	case "postgres":
		return "postgresql"
	case "mysql":
		return "mysql"
	default:
		return pa.config.Driver
	}
}