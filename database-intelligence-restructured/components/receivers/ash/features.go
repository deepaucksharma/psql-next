package ash

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FeatureDetector detects available database features and extensions
type FeatureDetector struct {
	logger    *zap.Logger
	mu        sync.RWMutex
	
	// Detected features
	features  map[string]bool
	version   string
	lastCheck time.Time
	
	// Cache duration
	cacheDuration time.Duration
}

// NewFeatureDetector creates a new feature detector
func NewFeatureDetector(logger *zap.Logger) *FeatureDetector {
	return &FeatureDetector{
		logger:        logger,
		features:      make(map[string]bool),
		cacheDuration: 5 * time.Minute,
	}
}

// DetectFeatures detects available database features
func (fd *FeatureDetector) DetectFeatures(ctx context.Context, db *sql.DB) error {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	
	// Check cache
	if time.Since(fd.lastCheck) < fd.cacheDuration {
		return nil
	}
	
	// Reset features
	fd.features = make(map[string]bool)
	
	// Detect PostgreSQL version
	if err := fd.detectVersion(ctx, db); err != nil {
		fd.logger.Warn("Failed to detect PostgreSQL version", zap.Error(err))
	}
	
	// Detect extensions
	if err := fd.detectExtensions(ctx, db); err != nil {
		fd.logger.Warn("Failed to detect extensions", zap.Error(err))
	}
	
	// Detect specific capabilities
	fd.detectCapabilities(ctx, db)
	
	fd.lastCheck = time.Now()
	
	fd.logger.Info("Feature detection completed",
		zap.String("version", fd.version),
		zap.Any("features", fd.features))
	
	return nil
}

// detectVersion detects the PostgreSQL version
func (fd *FeatureDetector) detectVersion(ctx context.Context, db *sql.DB) error {
	var version string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return err
	}
	
	fd.version = version
	
	// Parse major version for feature detection
	var major int
	if n, _ := sql_sscanf(version, "PostgreSQL %d", &major); n == 1 {
		fd.features["pg_version_"+string(rune(major))] = true
		
		// Version-specific features
		if major >= 14 {
			fd.features["pg_stat_progress_copy"] = true
		}
		if major >= 13 {
			fd.features["pg_stat_progress_analyze"] = true
			fd.features["pg_stat_progress_basebackup"] = true
		}
		if major >= 12 {
			fd.features["pg_stat_progress_cluster"] = true
			fd.features["pg_stat_progress_create_index"] = true
		}
	}
	
	return nil
}

// detectExtensions detects installed extensions
func (fd *FeatureDetector) detectExtensions(ctx context.Context, db *sql.DB) error {
	query := `
		SELECT extname 
		FROM pg_extension 
		WHERE extname IN (
			'pg_stat_statements',
			'pg_wait_sampling',
			'auto_explain',
			'pg_qualstats',
			'pg_stat_kcache',
			'pg_buffercache'
		)
	`
	
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var extName string
		if err := rows.Scan(&extName); err != nil {
			continue
		}
		fd.features[extName] = true
	}
	
	return rows.Err()
}

// detectCapabilities detects specific database capabilities
func (fd *FeatureDetector) detectCapabilities(ctx context.Context, db *sql.DB) {
	// Check for track_io_timing
	var trackIOTiming string
	err := db.QueryRowContext(ctx, "SHOW track_io_timing").Scan(&trackIOTiming)
	if err == nil && trackIOTiming == "on" {
		fd.features["track_io_timing"] = true
	}
	
	// Check for track_activity_query_size
	var querySize int
	err = db.QueryRowContext(ctx, "SHOW track_activity_query_size").Scan(&querySize)
	if err == nil {
		fd.features["track_activity_query_size"] = true
		if querySize >= 4096 {
			fd.features["large_query_tracking"] = true
		}
	}
	
	// Check for log_lock_waits
	var logLockWaits string
	err = db.QueryRowContext(ctx, "SHOW log_lock_waits").Scan(&logLockWaits)
	if err == nil && logLockWaits == "on" {
		fd.features["log_lock_waits"] = true
	}
	
	// Check table existence for custom monitoring
	fd.checkTableExists(ctx, db, "pg_stat_user_tables", "user_tables_stats")
	fd.checkTableExists(ctx, db, "pg_stat_user_indexes", "user_indexes_stats")
	fd.checkTableExists(ctx, db, "pg_statio_user_tables", "user_io_stats")
}

// checkTableExists checks if a table exists and sets a feature flag
func (fd *FeatureDetector) checkTableExists(ctx context.Context, db *sql.DB, tableName, featureName string) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'pg_catalog' 
			AND table_name = $1
		)
	`
	
	err := db.QueryRowContext(ctx, query, tableName).Scan(&exists)
	if err == nil && exists {
		fd.features[featureName] = true
	}
}

// HasFeature checks if a feature is available
func (fd *FeatureDetector) HasFeature(feature string) bool {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	return fd.features[feature]
}

// GetVersion returns the detected PostgreSQL version
func (fd *FeatureDetector) GetVersion() string {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	return fd.version
}

// GetFeatures returns all detected features
func (fd *FeatureDetector) GetFeatures() map[string]bool {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	
	// Return a copy
	features := make(map[string]bool)
	for k, v := range fd.features {
		features[k] = v
	}
	return features
}

// sql_sscanf is a simplified sscanf for SQL version parsing
func sql_sscanf(str, format string, args ...interface{}) (int, error) {
	// This is a simplified implementation
	// In production, you'd use a proper parsing library
	var major int
	n, err := fmt.Sscanf(str, format, &major)
	if err != nil {
		return 0, err
	}
	if len(args) > 0 {
		if ptr, ok := args[0].(*int); ok {
			*ptr = major
		}
	}
	return n, nil
}