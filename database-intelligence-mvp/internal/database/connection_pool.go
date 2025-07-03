// Package database provides secure database connection utilities with connection pooling
package database

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ConnectionPoolConfig defines secure connection pool configuration
type ConnectionPoolConfig struct {
	// Maximum number of open connections to prevent resource exhaustion
	MaxOpenConnections int `json:"max_open_connections" yaml:"max_open_connections"`
	
	// Maximum number of idle connections in the pool
	MaxIdleConnections int `json:"max_idle_connections" yaml:"max_idle_connections"`
	
	// Maximum amount of time a connection may be reused (security)
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	
	// Maximum amount of time a connection may be idle (performance)
	ConnMaxIdleTime time.Duration `json:"conn_max_idle_time" yaml:"conn_max_idle_time"`
}

// DefaultConnectionPoolConfig returns secure default connection pool settings
func DefaultConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxOpenConnections: 25,                // Prevent connection exhaustion
		MaxIdleConnections: 5,                 // Limit idle connections
		ConnMaxLifetime:    5 * time.Minute,   // Force reconnection for security
		ConnMaxIdleTime:    5 * time.Minute,   // Close idle connections
	}
}

// TestConnectionPoolConfig returns connection pool settings optimized for testing
func TestConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxOpenConnections: 10,                // Lower for tests
		MaxIdleConnections: 2,                 // Minimal for tests
		ConnMaxLifetime:    2 * time.Minute,   // Shorter for tests
		ConnMaxIdleTime:    1 * time.Minute,   // Shorter for tests
	}
}

// ValidationConnectionPoolConfig returns connection pool settings for validation tools
func ValidationConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxOpenConnections: 5,                 // Very conservative for validation
		MaxIdleConnections: 1,                 // Minimal idle connections
		ConnMaxLifetime:    3 * time.Minute,   // Moderate lifetime
		ConnMaxIdleTime:    2 * time.Minute,   // Quick cleanup
	}
}

// LoadTestConnectionPoolConfig returns connection pool settings optimized for load testing
func LoadTestConnectionPoolConfig() ConnectionPoolConfig {
	return ConnectionPoolConfig{
		MaxOpenConnections: 50,                // Higher for load testing
		MaxIdleConnections: 10,                // More idle connections for performance
		ConnMaxLifetime:    10 * time.Minute,  // Longer lifetime for stability
		ConnMaxIdleTime:    5 * time.Minute,   // Standard idle time
	}
}

// ConfigureConnectionPool applies secure connection pool settings to a database connection
func ConfigureConnectionPool(db *sql.DB, config ConnectionPoolConfig, logger *zap.Logger) {
	if logger != nil {
		logger.Info("Configuring database connection pool",
			zap.Int("max_open_connections", config.MaxOpenConnections),
			zap.Int("max_idle_connections", config.MaxIdleConnections),
			zap.Duration("conn_max_lifetime", config.ConnMaxLifetime),
			zap.Duration("conn_max_idle_time", config.ConnMaxIdleTime))
	}
	
	// Set maximum open connections (security: prevent exhaustion)
	db.SetMaxOpenConns(config.MaxOpenConnections)
	
	// Set maximum idle connections (performance: balance resource usage)
	db.SetMaxIdleConns(config.MaxIdleConnections)
	
	// Set connection lifetime (security: force periodic reconnection)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	
	// Set idle timeout (performance: close unused connections)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)
}

// OpenWithSecurePool opens a database connection with secure connection pooling
func OpenWithSecurePool(driver, dataSource string, config ConnectionPoolConfig, logger *zap.Logger) (*sql.DB, error) {
	// Open database connection
	db, err := sql.Open(driver, dataSource)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	
	// Configure secure connection pool
	ConfigureConnectionPool(db, config, logger)
	
	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}
	
	if logger != nil {
		logger.Info("Database connection established with secure pooling",
			zap.String("driver", driver))
	}
	
	return db, nil
}

// GetPoolStats returns current connection pool statistics
func GetPoolStats(db *sql.DB) sql.DBStats {
	return db.Stats()
}

// LogPoolStats logs current connection pool statistics
func LogPoolStats(db *sql.DB, logger *zap.Logger, component string) {
	if logger == nil {
		return
	}
	
	stats := db.Stats()
	
	logger.Info("Database connection pool statistics",
		zap.String("component", component),
		zap.Int("open_connections", stats.OpenConnections),
		zap.Int("in_use", stats.InUse),
		zap.Int("idle", stats.Idle),
		zap.Int64("wait_count", stats.WaitCount),
		zap.Duration("wait_duration", stats.WaitDuration),
		zap.Int64("max_idle_closed", stats.MaxIdleClosed),
		zap.Int64("max_idle_time_closed", stats.MaxIdleTimeClosed),
		zap.Int64("max_lifetime_closed", stats.MaxLifetimeClosed))
}

// ValidatePoolConfig validates connection pool configuration for security
func ValidatePoolConfig(config ConnectionPoolConfig) error {
	if config.MaxOpenConnections <= 0 {
		return fmt.Errorf("max_open_connections must be positive")
	}
	
	if config.MaxIdleConnections < 0 {
		return fmt.Errorf("max_idle_connections cannot be negative")
	}
	
	if config.MaxIdleConnections > config.MaxOpenConnections {
		return fmt.Errorf("max_idle_connections (%d) cannot exceed max_open_connections (%d)",
			config.MaxIdleConnections, config.MaxOpenConnections)
	}
	
	if config.ConnMaxLifetime <= 0 {
		return fmt.Errorf("conn_max_lifetime must be positive for security")
	}
	
	if config.ConnMaxIdleTime <= 0 {
		return fmt.Errorf("conn_max_idle_time must be positive for performance")
	}
	
	// Security: Warn about potentially insecure configurations
	if config.MaxOpenConnections > 100 {
		return fmt.Errorf("max_open_connections (%d) exceeds recommended maximum (100) for security",
			config.MaxOpenConnections)
	}
	
	if config.ConnMaxLifetime > 30*time.Minute {
		return fmt.Errorf("conn_max_lifetime (%v) exceeds recommended maximum (30m) for security",
			config.ConnMaxLifetime)
	}
	
	return nil
}