package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// PooledClient wraps a database client with connection pooling
type PooledClient struct {
	db           *sql.DB
	driver       string
	dbType       string
	logger       *zap.Logger
	poolConfig   ConnectionPoolConfig
	
	// Metrics
	queryCount   atomic.Int64
	errorCount   atomic.Int64
	totalLatency atomic.Int64
	
	// Circuit breaker
	mu                sync.RWMutex
	consecutiveErrors int
	circuitOpen       bool
	circuitOpenUntil  time.Time
}

// NewPooledClient creates a new pooled database client
func NewPooledClient(driver, dataSource string, config ConnectionPoolConfig, logger *zap.Logger) (*PooledClient, error) {
	// Validate configuration
	if err := ValidatePoolConfig(config); err != nil {
		return nil, fmt.Errorf("invalid pool configuration: %w", err)
	}
	
	// Open database with pooling
	db, err := OpenWithSecurePool(driver, dataSource, config, logger)
	if err != nil {
		return nil, err
	}
	
	// Determine database type from driver
	dbType := driver
	if driver == "postgres" {
		dbType = "postgresql"
	}
	
	return &PooledClient{
		db:         db,
		driver:     driver,
		dbType:     dbType,
		logger:     logger,
		poolConfig: config,
	}, nil
}

// Query executes a query that returns rows
func (pc *PooledClient) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	// Check circuit breaker
	if pc.isCircuitOpen() {
		pc.errorCount.Add(1)
		return nil, fmt.Errorf("circuit breaker is open")
	}
	
	start := time.Now()
	rows, err := pc.db.QueryContext(ctx, query, args...)
	latency := time.Since(start)
	
	pc.queryCount.Add(1)
	pc.totalLatency.Add(int64(latency))
	
	if err != nil {
		pc.recordError()
		return nil, err
	}
	
	pc.recordSuccess()
	return &sqlRows{rows: rows}, nil
}

// Exec executes a query that doesn't return rows
func (pc *PooledClient) Exec(ctx context.Context, query string, args ...interface{}) (Result, error) {
	// Check circuit breaker
	if pc.isCircuitOpen() {
		pc.errorCount.Add(1)
		return nil, fmt.Errorf("circuit breaker is open")
	}
	
	start := time.Now()
	result, err := pc.db.ExecContext(ctx, query, args...)
	latency := time.Since(start)
	
	pc.queryCount.Add(1)
	pc.totalLatency.Add(int64(latency))
	
	if err != nil {
		pc.recordError()
		return nil, err
	}
	
	pc.recordSuccess()
	return result, nil
}

// Close closes the database connection
func (pc *PooledClient) Close() error {
	if pc.logger != nil {
		stats := pc.db.Stats()
		pc.logger.Info("Closing pooled database connection",
			zap.String("driver", pc.driver),
			zap.Int64("total_queries", pc.queryCount.Load()),
			zap.Int64("total_errors", pc.errorCount.Load()),
			zap.Int("final_open_connections", stats.OpenConnections))
	}
	
	return pc.db.Close()
}

// Ping verifies the connection is alive
func (pc *PooledClient) Ping(ctx context.Context) error {
	// Check circuit breaker
	if pc.isCircuitOpen() {
		return fmt.Errorf("circuit breaker is open")
	}
	
	err := pc.db.PingContext(ctx)
	if err != nil {
		pc.recordError()
		return err
	}
	
	pc.recordSuccess()
	return nil
}

// Stats returns connection statistics
func (pc *PooledClient) Stats() Statistics {
	dbStats := pc.db.Stats()
	
	return Statistics{
		OpenConnections:    dbStats.OpenConnections,
		InUse:              dbStats.InUse,
		Idle:               dbStats.Idle,
		WaitCount:          dbStats.WaitCount,
		WaitDuration:       dbStats.WaitDuration,
		MaxIdleClosed:      dbStats.MaxIdleClosed,
		MaxIdleTimeClosed:  dbStats.MaxIdleTimeClosed,
		MaxLifetimeClosed:  dbStats.MaxLifetimeClosed,
	}
}

// DatabaseType returns the type of database
func (pc *PooledClient) DatabaseType() string {
	return pc.dbType
}

// GetMetrics returns client metrics
func (pc *PooledClient) GetMetrics() ClientMetrics {
	queryCount := pc.queryCount.Load()
	errorCount := pc.errorCount.Load()
	totalLatency := pc.totalLatency.Load()
	
	avgLatency := time.Duration(0)
	if queryCount > 0 {
		avgLatency = time.Duration(totalLatency / queryCount)
	}
	
	return ClientMetrics{
		QueryCount:        queryCount,
		ErrorCount:        errorCount,
		TotalLatency:      time.Duration(totalLatency),
		AverageLatency:    avgLatency,
		ErrorRate:         float64(errorCount) / float64(queryCount),
		CircuitBreakerOpen: pc.isCircuitOpen(),
	}
}

// SetMaxConnections dynamically adjusts the maximum connections
func (pc *PooledClient) SetMaxConnections(maxOpen, maxIdle int) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	oldMaxOpen := pc.poolConfig.MaxOpenConnections
	oldMaxIdle := pc.poolConfig.MaxIdleConnections
	
	pc.db.SetMaxOpenConns(maxOpen)
	pc.db.SetMaxIdleConns(maxIdle)
	
	pc.poolConfig.MaxOpenConnections = maxOpen
	pc.poolConfig.MaxIdleConnections = maxIdle
	
	if pc.logger != nil {
		pc.logger.Info("Adjusted connection pool size",
			zap.Int("old_max_open", oldMaxOpen),
			zap.Int("new_max_open", maxOpen),
			zap.Int("old_max_idle", oldMaxIdle),
			zap.Int("new_max_idle", maxIdle))
	}
}

// isCircuitOpen checks if the circuit breaker is open
func (pc *PooledClient) isCircuitOpen() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	if !pc.circuitOpen {
		return false
	}
	
	// Check if circuit should be closed
	if time.Now().After(pc.circuitOpenUntil) {
		pc.mu.RUnlock()
		pc.mu.Lock()
		pc.circuitOpen = false
		pc.consecutiveErrors = 0
		pc.mu.Unlock()
		pc.mu.RLock()
		
		if pc.logger != nil {
			pc.logger.Info("Circuit breaker closed")
		}
		
		return false
	}
	
	return true
}

// recordError records an error and potentially opens the circuit
func (pc *PooledClient) recordError() {
	pc.errorCount.Add(1)
	
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	pc.consecutiveErrors++
	
	// Open circuit after 5 consecutive errors
	if pc.consecutiveErrors >= 5 && !pc.circuitOpen {
		pc.circuitOpen = true
		pc.circuitOpenUntil = time.Now().Add(30 * time.Second)
		
		if pc.logger != nil {
			pc.logger.Warn("Circuit breaker opened due to consecutive errors",
				zap.Int("consecutive_errors", pc.consecutiveErrors))
		}
	}
}

// recordSuccess records a successful operation
func (pc *PooledClient) recordSuccess() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	
	pc.consecutiveErrors = 0
}

// sqlRows wraps sql.Rows to implement our Rows interface
type sqlRows struct {
	rows *sql.Rows
}

func (r *sqlRows) Next() bool {
	return r.rows.Next()
}

func (r *sqlRows) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

func (r *sqlRows) Close() error {
	return r.rows.Close()
}

func (r *sqlRows) Columns() ([]string, error) {
	return r.rows.Columns()
}

func (r *sqlRows) Err() error {
	return r.rows.Err()
}

// ClientMetrics represents client performance metrics
type ClientMetrics struct {
	QueryCount         int64
	ErrorCount         int64
	TotalLatency       time.Duration
	AverageLatency     time.Duration
	ErrorRate          float64
	CircuitBreakerOpen bool
}

// PooledClientConfig extends Config with pooling-specific settings
type PooledClientConfig struct {
	*Config
	
	// Circuit breaker settings
	CircuitBreakerThreshold   int           // Number of consecutive errors to open circuit
	CircuitBreakerTimeout     time.Duration // How long to keep circuit open
	
	// Dynamic scaling settings
	EnableDynamicScaling      bool
	ScaleUpThreshold          float64 // Connection utilization threshold to scale up
	ScaleDownThreshold        float64 // Connection utilization threshold to scale down
	ScaleCheckInterval        time.Duration
}

// NewPooledClientFromConfig creates a pooled client from configuration
func NewPooledClientFromConfig(cfg *PooledClientConfig, logger *zap.Logger) (*PooledClient, error) {
	// Build connection string based on database type
	var dataSource string
	var driver string
	
	// Simple connection string builder - would be more sophisticated in production
	switch cfg.DatabaseType() {
	case "postgresql":
		driver = "postgres"
		dataSource = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database)
	case "mysql":
		driver = "mysql"
		dataSource = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.DatabaseType())
	}
	
	poolConfig := ConnectionPoolConfig{
		MaxOpenConnections: cfg.MaxOpenConns,
		MaxIdleConnections: cfg.MaxIdleConns,
		ConnMaxLifetime:    cfg.ConnMaxLifetime,
		ConnMaxIdleTime:    cfg.ConnMaxIdleTime,
	}
	
	return NewPooledClient(driver, dataSource, poolConfig, logger)
}