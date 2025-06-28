package postgresqlquery

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"go.uber.org/zap"
)

// dbConnection represents a database connection with its state
type dbConnection struct {
	db              *sql.DB
	config          DatabaseConfig
	logger          *zap.Logger
	
	// Connection state
	mu              sync.RWMutex
	connected       bool
	lastError       error
	errorCount      int
	lastErrorTime   time.Time
	
	// Capabilities
	capabilities    DatabaseCapabilities
	
	// ASH sampler (if enabled)
	ashSampler      *ActiveSessionHistorySampler
}

// DatabaseCapabilities represents detected database capabilities
type DatabaseCapabilities struct {
	Version             string
	IsSuperuser         bool
	HasPgStatStatements bool
	HasPgWaitSampling   bool
	HasPgStatKcache     bool
	HasPgQuerylens      bool
	IsRDS               bool
	IsAzure             bool
	IsGCP               bool
	MaxConnections      int
	SharedBuffers       string
	EffectiveCacheSize  string
}

// newDBConnection creates a new database connection
func newDBConnection(config DatabaseConfig, logger *zap.Logger) *dbConnection {
	return &dbConnection{
		config: config,
		logger: logger.With(zap.String("database", config.Name)),
	}
}

// Connect establishes the database connection
func (dbc *dbConnection) Connect(ctx context.Context) error {
	dbc.mu.Lock()
	defer dbc.mu.Unlock()
	
	if dbc.connected && dbc.db != nil {
		return nil
	}
	
	// Open database connection
	db, err := sql.Open("postgres", string(dbc.config.DSN))
	if err != nil {
		dbc.recordError(err)
		return fmt.Errorf("failed to open database: %w", err)
	}
	
	// Configure connection pool
	db.SetMaxOpenConns(dbc.config.MaxOpenConnections)
	db.SetMaxIdleConns(dbc.config.MaxIdleConnections)
	db.SetConnMaxLifetime(dbc.config.ConnectionMaxLifetime)
	db.SetConnMaxIdleTime(dbc.config.ConnectionMaxIdleTime)
	
	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		dbc.recordError(err)
		return fmt.Errorf("failed to ping database: %w", err)
	}
	
	dbc.db = db
	dbc.connected = true
	dbc.errorCount = 0
	
	// Detect capabilities
	if err := dbc.detectCapabilities(ctx); err != nil {
		dbc.logger.Warn("Failed to detect some capabilities", zap.Error(err))
	}
	
	dbc.logger.Info("Connected to database",
		zap.String("version", dbc.capabilities.Version),
		zap.Bool("pg_stat_statements", dbc.capabilities.HasPgStatStatements),
		zap.Bool("pg_wait_sampling", dbc.capabilities.HasPgWaitSampling),
		zap.Bool("pg_stat_kcache", dbc.capabilities.HasPgStatKcache),
		zap.Bool("is_rds", dbc.capabilities.IsRDS),
		zap.Bool("is_azure", dbc.capabilities.IsAzure),
		zap.Bool("is_gcp", dbc.capabilities.IsGCP))
	
	return nil
}

// Disconnect closes the database connection
func (dbc *dbConnection) Disconnect() error {
	dbc.mu.Lock()
	defer dbc.mu.Unlock()
	
	if !dbc.connected || dbc.db == nil {
		return nil
	}
	
	// Stop ASH sampler if running
	if dbc.ashSampler != nil {
		if err := dbc.ashSampler.Stop(); err != nil {
			dbc.logger.Warn("Failed to stop ASH sampler", zap.Error(err))
		}
		dbc.ashSampler = nil
	}
	
	err := dbc.db.Close()
	dbc.db = nil
	dbc.connected = false
	
	return err
}

// detectCapabilities detects database capabilities and extensions
func (dbc *dbConnection) detectCapabilities(ctx context.Context) error {
	// Get PostgreSQL version
	var version string
	err := dbc.db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}
	dbc.capabilities.Version = version
	
	// Check if superuser
	var isSuperuser bool
	err = dbc.db.QueryRowContext(ctx, "SELECT current_setting('is_superuser') = 'on'").Scan(&isSuperuser)
	if err == nil {
		dbc.capabilities.IsSuperuser = isSuperuser
	}
	
	// Check for extensions
	query := `
		SELECT extname 
		FROM pg_extension 
		WHERE extname IN ('pg_stat_statements', 'pg_wait_sampling', 'pg_stat_kcache', 'pg_querylens')
	`
	rows, err := dbc.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to check extensions: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var extName string
		if err := rows.Scan(&extName); err != nil {
			continue
		}
		
		switch extName {
		case "pg_stat_statements":
			dbc.capabilities.HasPgStatStatements = true
		case "pg_wait_sampling":
			dbc.capabilities.HasPgWaitSampling = true
		case "pg_stat_kcache":
			dbc.capabilities.HasPgStatKcache = true
		case "pg_querylens":
			dbc.capabilities.HasPgQuerylens = true
		}
	}
	
	// Detect cloud provider
	dbc.detectCloudProvider()
	
	// Get some important settings
	dbc.getServerSettings(ctx)
	
	return nil
}

// detectCloudProvider detects if running on a cloud provider
func (dbc *dbConnection) detectCloudProvider() {
	version := strings.ToLower(dbc.capabilities.Version)
	
	// RDS detection
	if strings.Contains(version, "rds") || strings.Contains(version, "aurora") {
		dbc.capabilities.IsRDS = true
	}
	
	// Azure detection
	if strings.Contains(version, "azure") || strings.Contains(version, "microsoft") {
		dbc.capabilities.IsAzure = true
	}
	
	// GCP detection
	if strings.Contains(version, "google") || strings.Contains(version, "cloudsql") {
		dbc.capabilities.IsGCP = true
	}
}

// getServerSettings retrieves important server settings
func (dbc *dbConnection) getServerSettings(ctx context.Context) {
	settings := []struct {
		name  string
		dest  *string
		isInt bool
		intDest *int
	}{
		{name: "shared_buffers", dest: &dbc.capabilities.SharedBuffers},
		{name: "effective_cache_size", dest: &dbc.capabilities.EffectiveCacheSize},
		{name: "max_connections", isInt: true, intDest: &dbc.capabilities.MaxConnections},
	}
	
	for _, setting := range settings {
		var value string
		err := dbc.db.QueryRowContext(ctx, 
			"SELECT setting FROM pg_settings WHERE name = $1", 
			setting.name).Scan(&value)
		
		if err != nil {
			continue
		}
		
		if setting.isInt {
			var intVal int
			fmt.Sscanf(value, "%d", &intVal)
			*setting.intDest = intVal
		} else {
			*setting.dest = value
		}
	}
}

// IsHealthy checks if the connection is healthy
func (dbc *dbConnection) IsHealthy(ctx context.Context) bool {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	
	if !dbc.connected || dbc.db == nil {
		return false
	}
	
	// Check if we've hit error threshold
	if dbc.errorCount >= 5 {
		return false
	}
	
	// Ping the database
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	if err := dbc.db.PingContext(ctx); err != nil {
		dbc.recordError(err)
		return false
	}
	
	return true
}

// recordError records an error occurrence
func (dbc *dbConnection) recordError(err error) {
	dbc.lastError = err
	dbc.lastErrorTime = time.Now()
	dbc.errorCount++
}

// resetErrorCount resets the error counter
func (dbc *dbConnection) resetErrorCount() {
	dbc.errorCount = 0
}

// GetDB returns the underlying database connection
func (dbc *dbConnection) GetDB() *sql.DB {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	return dbc.db
}

// GetCapabilities returns the detected capabilities
func (dbc *dbConnection) GetCapabilities() DatabaseCapabilities {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	return dbc.capabilities
}

// StartASH starts the Active Session History sampler
func (dbc *dbConnection) StartASH(interval time.Duration, bufferSize int) error {
	dbc.mu.Lock()
	defer dbc.mu.Unlock()
	
	if dbc.ashSampler != nil {
		return fmt.Errorf("ASH sampler already running")
	}
	
	if !dbc.connected || dbc.db == nil {
		return fmt.Errorf("database not connected")
	}
	
	dbc.ashSampler = NewActiveSessionHistorySampler(
		dbc.logger,
		dbc.db,
		dbc.config.Name,
		interval,
		bufferSize,
	)
	
	return dbc.ashSampler.Start()
}

// GetASHSampler returns the ASH sampler if running
func (dbc *dbConnection) GetASHSampler() *ActiveSessionHistorySampler {
	dbc.mu.RLock()
	defer dbc.mu.RUnlock()
	return dbc.ashSampler
}