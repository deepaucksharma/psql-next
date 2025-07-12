// Package featuredetector provides database capability detection for graceful fallback
package featuredetector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
	
	"go.uber.org/zap"
)

// Feature represents a database feature or extension
type Feature struct {
	Name         string                 `json:"name"`
	Available    bool                   `json:"available"`
	Version      string                 `json:"version,omitempty"`
	LastChecked  time.Time             `json:"last_checked"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
}

// FeatureSet represents all detected features for a database
type FeatureSet struct {
	DatabaseType    string              `json:"database_type"`
	ServerVersion   string              `json:"server_version"`
	Extensions      map[string]*Feature `json:"extensions"`
	Capabilities    map[string]*Feature `json:"capabilities"`
	CloudProvider   string              `json:"cloud_provider,omitempty"`
	LastDetection   time.Time           `json:"last_detection"`
	DetectionErrors []string            `json:"detection_errors,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// QueryRequirements defines what features a query needs
type QueryRequirements struct {
	RequiredExtensions []string `json:"required_extensions"`
	RequiredCapabilities []string `json:"required_capabilities"`
	MinimumVersion     string   `json:"minimum_version,omitempty"`
}

// QueryDefinition defines a query with its requirements
type QueryDefinition struct {
	Name         string            `mapstructure:"name"`
	SQL          string            `mapstructure:"sql"`
	Requirements QueryRequirements `mapstructure:"requirements"`
	Priority     int               `mapstructure:"priority"`
	FallbackName string            `mapstructure:"fallback"`
	Description  string            `mapstructure:"description"`
}

// Detector interface for feature detection
type Detector interface {
	// DetectFeatures performs feature detection for the database
	DetectFeatures(ctx context.Context) (*FeatureSet, error)
	
	// GetCachedFeatures returns cached features if still valid
	GetCachedFeatures() *FeatureSet
	
	// ValidateQuery checks if a query can run with current features
	ValidateQuery(query *QueryDefinition) error
	
	// SelectBestQuery selects the best query from alternatives
	SelectBestQuery(queries []QueryDefinition) (*QueryDefinition, error)
}

// BaseDetector provides common detection functionality
type BaseDetector struct {
	db              *sql.DB
	logger          *zap.Logger
	cache           *FeatureSet
	cacheMutex      sync.RWMutex
	cacheDuration   time.Duration
	detectionConfig DetectionConfig
}

// DetectionConfig configures feature detection behavior
type DetectionConfig struct {
	// CacheDuration how long to cache detection results
	CacheDuration time.Duration `mapstructure:"cache_duration"`
	
	// RetryAttempts for failed detections
	RetryAttempts int `mapstructure:"retry_attempts"`
	
	// RetryDelay between attempts
	RetryDelay time.Duration `mapstructure:"retry_delay"`
	
	// TimeoutPerCheck for individual checks
	TimeoutPerCheck time.Duration `mapstructure:"timeout_per_check"`
	
	// SkipCloudDetection disables cloud provider detection
	SkipCloudDetection bool `mapstructure:"skip_cloud_detection"`
	
	// CustomQueries for additional feature checks
	CustomQueries map[string]string `mapstructure:"custom_queries"`
}

// CloudProvider types
const (
	CloudProviderAWS    = "aws"
	CloudProviderGCP    = "gcp"
	CloudProviderAzure  = "azure"
	CloudProviderLocal  = "local"
	CloudProviderUnknown = "unknown"
)

// Database types
const (
	DatabaseTypePostgreSQL = "postgresql"
	DatabaseTypeMySQL      = "mysql"
	DatabaseTypeOracle     = "oracle"
	DatabaseTypeSQLServer  = "sqlserver"
)

// Common PostgreSQL extensions
const (
	ExtPgStatStatements  = "pg_stat_statements"
	ExtPgStatMonitor     = "pg_stat_monitor"
	ExtPgWaitSampling    = "pg_wait_sampling"
	ExtAutoExplain       = "auto_explain"
	ExtPgQuerylens       = "pg_querylens"
	ExtPgStatKcache      = "pg_stat_kcache"
	ExtPgQualstats       = "pg_qualstats"
)

// Common capabilities
const (
	CapTrackIOTiming     = "track_io_timing"
	CapTrackFunctions    = "track_functions"
	CapTrackActivitySize = "track_activity_query_size"
	CapSharedPreload     = "shared_preload_libraries"
	CapStatementTimeout  = "statement_timeout"
)

// NewBaseDetector creates a base detector
func NewBaseDetector(db *sql.DB, logger *zap.Logger, config DetectionConfig) *BaseDetector {
	if config.CacheDuration == 0 {
		config.CacheDuration = 5 * time.Minute
	}
	if config.TimeoutPerCheck == 0 {
		config.TimeoutPerCheck = 3 * time.Second
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	
	return &BaseDetector{
		db:              db,
		logger:          logger,
		cacheDuration:   config.CacheDuration,
		detectionConfig: config,
	}
}

// GetCachedFeatures returns cached features if still valid
func (bd *BaseDetector) GetCachedFeatures() *FeatureSet {
	bd.cacheMutex.RLock()
	defer bd.cacheMutex.RUnlock()
	
	if bd.cache == nil {
		return nil
	}
	
	// Check if cache is still valid
	if time.Since(bd.cache.LastDetection) > bd.cacheDuration {
		return nil
	}
	
	return bd.cache
}

// SetCache updates the cache
func (bd *BaseDetector) SetCache(features *FeatureSet) {
	bd.cacheMutex.Lock()
	defer bd.cacheMutex.Unlock()
	
	bd.cache = features
}

// ValidateQuery checks if a query can run with current features
func (bd *BaseDetector) ValidateQuery(query *QueryDefinition) error {
	features := bd.GetCachedFeatures()
	if features == nil {
		return ErrNoFeatureData
	}
	
	// Check required extensions
	for _, ext := range query.Requirements.RequiredExtensions {
		if feature, exists := features.Extensions[ext]; !exists || !feature.Available {
			return &MissingFeatureError{
				FeatureType: "extension",
				FeatureName: ext,
			}
		}
	}
	
	// Check required capabilities
	for _, cap := range query.Requirements.RequiredCapabilities {
		if feature, exists := features.Capabilities[cap]; !exists || !feature.Available {
			return &MissingFeatureError{
				FeatureType: "capability",
				FeatureName: cap,
			}
		}
	}
	
	// Check minimum version if specified
	if query.Requirements.MinimumVersion != "" && features.DatabaseVersion != "" {
		if !isVersionSufficient(features.DatabaseVersion, query.Requirements.MinimumVersion) {
			return &MissingFeatureError{
				FeatureType: "version",
				FeatureName: fmt.Sprintf("minimum version %s (current: %s)", query.Requirements.MinimumVersion, features.DatabaseVersion),
			}
		}
	}
	
	return nil
}

// SelectBestQuery selects the best available query from alternatives
func (bd *BaseDetector) SelectBestQuery(queries []QueryDefinition) (*QueryDefinition, error) {
	if len(queries) == 0 {
		return nil, ErrNoQueries
	}
	
	// Sort by priority (higher is better)
	sortedQueries := make([]QueryDefinition, len(queries))
	copy(sortedQueries, queries)
	
	// Try queries in priority order
	for i := range sortedQueries {
		for j := i + 1; j < len(sortedQueries); j++ {
			if sortedQueries[j].Priority > sortedQueries[i].Priority {
				sortedQueries[i], sortedQueries[j] = sortedQueries[j], sortedQueries[i]
			}
		}
	}
	
	// Find first query that meets requirements
	for _, query := range sortedQueries {
		if err := bd.ValidateQuery(&query); err == nil {
			bd.logger.Debug("Selected query based on features",
				zap.String("query", query.Name),
				zap.Int("priority", query.Priority))
			return &query, nil
		} else {
			bd.logger.Debug("Query requirements not met",
				zap.String("query", query.Name),
				zap.Error(err))
		}
	}
	
	// No query meets requirements - use lowest priority as fallback
	return &sortedQueries[len(sortedQueries)-1], nil
}

// ExecuteWithTimeout executes a query with timeout
func (bd *BaseDetector) ExecuteWithTimeout(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, bd.detectionConfig.TimeoutPerCheck)
	defer cancel()
	
	return bd.db.QueryContext(timeoutCtx, query, args...)
}

// HasFeature checks if a specific feature is available
func (fs *FeatureSet) HasFeature(name string) bool {
	// Check extensions
	if ext, exists := fs.Extensions[name]; exists && ext.Available {
		return true
	}
	
	// Check capabilities
	if cap, exists := fs.Capabilities[name]; exists && cap.Available {
		return true
	}
	
	return false
}

// GetFeatureVersion returns the version of a feature if available
func (fs *FeatureSet) GetFeatureVersion(name string) string {
	if ext, exists := fs.Extensions[name]; exists && ext.Available {
		return ext.Version
	}
	
	if cap, exists := fs.Capabilities[name]; exists && cap.Available {
		return cap.Version
	}
	
	return ""
}

// isVersionSufficient checks if current version meets minimum requirement
// Supports simple numeric version comparison (e.g., "10.5", "14.2")
func isVersionSufficient(current, minimum string) bool {
	// Split versions into parts
	currentParts := strings.Split(current, ".")
	minimumParts := strings.Split(minimum, ".")
	
	// Compare each part
	for i := 0; i < len(minimumParts); i++ {
		if i >= len(currentParts) {
			// Current version has fewer parts, assume 0
			return false
		}
		
		// Parse numeric parts (ignore non-numeric suffixes)
		var currentNum, minNum int
		fmt.Sscanf(currentParts[i], "%d", &currentNum)
		fmt.Sscanf(minimumParts[i], "%d", &minNum)
		
		if currentNum < minNum {
			return false
		} else if currentNum > minNum {
			return true
		}
		// If equal, continue to next part
	}
	
	return true
}