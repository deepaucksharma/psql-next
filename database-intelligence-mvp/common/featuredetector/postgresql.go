package featuredetector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	
	"go.uber.org/zap"
)

// PostgreSQLDetector implements feature detection for PostgreSQL
type PostgreSQLDetector struct {
	*BaseDetector
}

// NewPostgreSQLDetector creates a new PostgreSQL feature detector
func NewPostgreSQLDetector(db *sql.DB, logger *zap.Logger, config DetectionConfig) *PostgreSQLDetector {
	return &PostgreSQLDetector{
		BaseDetector: NewBaseDetector(db, logger, config),
	}
}

// DetectFeatures performs comprehensive feature detection for PostgreSQL
func (pd *PostgreSQLDetector) DetectFeatures(ctx context.Context) (*FeatureSet, error) {
	// Check cache first
	if cached := pd.GetCachedFeatures(); cached != nil {
		pd.logger.Debug("Using cached feature detection results")
		return cached, nil
	}
	
	pd.logger.Info("Starting PostgreSQL feature detection")
	
	features := &FeatureSet{
		DatabaseType:    DatabaseTypePostgreSQL,
		Extensions:      make(map[string]*Feature),
		Capabilities:    make(map[string]*Feature),
		LastDetection:   time.Now(),
		DetectionErrors: []string{},
	}
	
	// Detect server version
	if err := pd.detectServerVersion(ctx, features); err != nil {
		features.DetectionErrors = append(features.DetectionErrors, 
			fmt.Sprintf("version detection: %v", err))
		pd.logger.Warn("Failed to detect server version", zap.Error(err))
	}
	
	// Detect extensions
	if err := pd.detectExtensions(ctx, features); err != nil {
		features.DetectionErrors = append(features.DetectionErrors,
			fmt.Sprintf("extension detection: %v", err))
		pd.logger.Warn("Failed to detect extensions", zap.Error(err))
	}
	
	// Detect capabilities
	if err := pd.detectCapabilities(ctx, features); err != nil {
		features.DetectionErrors = append(features.DetectionErrors,
			fmt.Sprintf("capability detection: %v", err))
		pd.logger.Warn("Failed to detect capabilities", zap.Error(err))
	}
	
	// Detect cloud provider
	if !pd.detectionConfig.SkipCloudDetection {
		if err := pd.detectCloudProvider(ctx, features); err != nil {
			features.DetectionErrors = append(features.DetectionErrors,
				fmt.Sprintf("cloud detection: %v", err))
			pd.logger.Warn("Failed to detect cloud provider", zap.Error(err))
		}
	}
	
	// Cache results
	pd.SetCache(features)
	
	pd.logger.Info("PostgreSQL feature detection completed",
		zap.Int("extensions", len(features.Extensions)),
		zap.Int("capabilities", len(features.Capabilities)),
		zap.String("cloud_provider", features.CloudProvider),
		zap.Int("errors", len(features.DetectionErrors)))
	
	return features, nil
}

// detectServerVersion detects PostgreSQL server version
func (pd *PostgreSQLDetector) detectServerVersion(ctx context.Context, features *FeatureSet) error {
	query := "SELECT version()"
	
	var version string
	err := pd.db.QueryRowContext(ctx, query).Scan(&version)
	if err != nil {
		return &DetectionError{
			Phase:   "version",
			Message: "failed to query version",
			Err:     err,
		}
	}
	
	features.ServerVersion = version
	
	// Parse major version for compatibility checks
	if strings.HasPrefix(version, "PostgreSQL") {
		parts := strings.Fields(version)
		if len(parts) >= 2 {
			features.Metadata = map[string]interface{}{
				"major_version": parts[1],
			}
		}
	}
	
	return nil
}

// detectExtensions detects available PostgreSQL extensions
func (pd *PostgreSQLDetector) detectExtensions(ctx context.Context, features *FeatureSet) error {
	// Query for installed extensions
	installedQuery := `
		SELECT 
			e.extname,
			e.extversion,
			n.nspname as schema,
			c.description
		FROM pg_extension e
		LEFT JOIN pg_namespace n ON n.oid = e.extnamespace
		LEFT JOIN pg_description c ON c.objoid = e.oid
		ORDER BY e.extname
	`
	
	rows, err := pd.ExecuteWithTimeout(ctx, installedQuery)
	if err != nil {
		return &DetectionError{
			Phase:   "extensions",
			Message: "failed to query installed extensions",
			Err:     err,
		}
	}
	defer rows.Close()
	
	// Process installed extensions
	for rows.Next() {
		var name, version, schema sql.NullString
		var description sql.NullString
		
		if err := rows.Scan(&name, &version, &schema, &description); err != nil {
			pd.logger.Warn("Failed to scan extension row", zap.Error(err))
			continue
		}
		
		if name.Valid {
			features.Extensions[name.String] = &Feature{
				Name:        name.String,
				Available:   true,
				Version:     version.String,
				LastChecked: time.Now(),
				Metadata: map[string]interface{}{
					"schema":      schema.String,
					"description": description.String,
				},
			}
		}
	}
	
	// Query for available but not installed extensions
	availableQuery := `
		SELECT 
			name,
			default_version,
			comment
		FROM pg_available_extensions
		WHERE installed_version IS NULL
			AND name IN ($1, $2, $3, $4, $5, $6, $7)
		ORDER BY name
	`
	
	// Check specific extensions we care about
	importantExtensions := []interface{}{
		ExtPgStatStatements,
		ExtPgStatMonitor,
		ExtPgWaitSampling,
		ExtAutoExplain,
		ExtPgQuerylens,
		ExtPgStatKcache,
		ExtPgQualstats,
	}
	
	availRows, err := pd.ExecuteWithTimeout(ctx, availableQuery, importantExtensions...)
	if err != nil {
		// Not critical - some databases don't allow querying available extensions
		pd.logger.Debug("Could not query available extensions", zap.Error(err))
	} else {
		defer availRows.Close()
		
		for availRows.Next() {
			var name, version, comment sql.NullString
			
			if err := availRows.Scan(&name, &version, &comment); err != nil {
				pd.logger.Warn("Failed to scan available extension row", zap.Error(err))
				continue
			}
			
			if name.Valid && features.Extensions[name.String] == nil {
				features.Extensions[name.String] = &Feature{
					Name:         name.String,
					Available:    false,
					Version:      version.String,
					LastChecked:  time.Now(),
					ErrorMessage: "not installed",
					Metadata: map[string]interface{}{
						"available_version": version.String,
						"comment":          comment.String,
					},
				}
			}
		}
	}
	
	// Special check for auto_explain (loaded via shared_preload_libraries)
	if err := pd.checkAutoExplain(ctx, features); err != nil {
		pd.logger.Debug("Failed to check auto_explain", zap.Error(err))
	}
	
	return nil
}

// detectCapabilities detects PostgreSQL configuration capabilities
func (pd *PostgreSQLDetector) detectCapabilities(ctx context.Context, features *FeatureSet) error {
	// Check key configuration parameters
	capabilityQueries := map[string]string{
		CapTrackIOTiming:     "SHOW track_io_timing",
		CapTrackFunctions:    "SHOW track_functions",
		CapTrackActivitySize: "SHOW track_activity_query_size",
		CapStatementTimeout:  "SHOW statement_timeout",
		CapSharedPreload:     "SHOW shared_preload_libraries",
	}
	
	for capability, query := range capabilityQueries {
		var value string
		err := pd.db.QueryRowContext(ctx, query).Scan(&value)
		
		if err != nil {
			features.Capabilities[capability] = &Feature{
				Name:         capability,
				Available:    false,
				LastChecked:  time.Now(),
				ErrorMessage: err.Error(),
			}
			continue
		}
		
		// Determine if capability is enabled
		available := false
		switch capability {
		case CapTrackIOTiming, CapTrackFunctions:
			available = value == "on" || value == "true"
		case CapTrackActivitySize:
			// Check if it's large enough for meaningful queries
			var size int
			if _, err := fmt.Sscanf(value, "%d", &size); err == nil {
				available = size >= 1024
			}
		case CapStatementTimeout:
			// Just check if it's configurable
			available = true
		case CapSharedPreload:
			// Will be parsed for specific libraries
			available = value != ""
		default:
			available = value != "" && value != "off" && value != "false"
		}
		
		features.Capabilities[capability] = &Feature{
			Name:        capability,
			Available:   available,
			Version:     value,
			LastChecked: time.Now(),
		}
	}
	
	// Check for specific query capabilities
	pd.checkQueryCapabilities(ctx, features)
	
	return nil
}

// checkAutoExplain checks if auto_explain is loaded
func (pd *PostgreSQLDetector) checkAutoExplain(ctx context.Context, features *FeatureSet) error {
	// Check if auto_explain is in shared_preload_libraries
	if sharedPreload, exists := features.Capabilities[CapSharedPreload]; exists && sharedPreload.Available {
		libraries := sharedPreload.Version
		if strings.Contains(libraries, "auto_explain") {
			features.Extensions[ExtAutoExplain] = &Feature{
				Name:        ExtAutoExplain,
				Available:   true,
				LastChecked: time.Now(),
				Metadata: map[string]interface{}{
					"loaded_via": "shared_preload_libraries",
				},
			}
			
			// Try to get auto_explain settings
			autoExplainSettings := []string{
				"auto_explain.log_min_duration",
				"auto_explain.log_analyze",
				"auto_explain.log_format",
			}
			
			settings := make(map[string]string)
			for _, setting := range autoExplainSettings {
				var value string
				if err := pd.db.QueryRowContext(ctx, "SHOW "+setting).Scan(&value); err == nil {
					settings[setting] = value
				}
			}
			
			features.Extensions[ExtAutoExplain].Metadata["settings"] = settings
		}
	}
	
	return nil
}

// checkQueryCapabilities checks specific query-related capabilities
func (pd *PostgreSQLDetector) checkQueryCapabilities(ctx context.Context, features *FeatureSet) error {
	// Check if we can query pg_stat_activity effectively
	query := `
		SELECT COUNT(*) 
		FROM pg_stat_activity 
		WHERE state = 'active' 
			AND pid != pg_backend_pid()
	`
	
	var count int
	if err := pd.db.QueryRowContext(ctx, query).Scan(&count); err == nil {
		features.Capabilities["query_pg_stat_activity"] = &Feature{
			Name:        "query_pg_stat_activity",
			Available:   true,
			LastChecked: time.Now(),
		}
	}
	
	// Check if we have permissions for pg_stat_statements
	if ext, exists := features.Extensions[ExtPgStatStatements]; exists && ext.Available {
		stmtQuery := "SELECT COUNT(*) FROM pg_stat_statements LIMIT 1"
		if err := pd.db.QueryRowContext(ctx, stmtQuery).Scan(&count); err == nil {
			features.Capabilities["read_pg_stat_statements"] = &Feature{
				Name:        "read_pg_stat_statements",
				Available:   true,
				LastChecked: time.Now(),
			}
		} else {
			features.Capabilities["read_pg_stat_statements"] = &Feature{
				Name:         "read_pg_stat_statements",
				Available:    false,
				LastChecked:  time.Now(),
				ErrorMessage: "permission denied or view not accessible",
			}
		}
	}
	
	return nil
}

// detectCloudProvider detects if PostgreSQL is running on a cloud provider
func (pd *PostgreSQLDetector) detectCloudProvider(ctx context.Context, features *FeatureSet) error {
	features.CloudProvider = CloudProviderLocal // Default
	
	// Check for AWS RDS
	var rdsCheck string
	err := pd.db.QueryRowContext(ctx, "SELECT setting FROM pg_settings WHERE name = 'rds.superuser_reserved_connections'").Scan(&rdsCheck)
	if err == nil {
		features.CloudProvider = CloudProviderAWS
		features.Metadata = map[string]interface{}{
			"cloud_variant": "RDS",
		}
		return nil
	}
	
	// Check for Aurora
	err = pd.db.QueryRowContext(ctx, "SELECT aurora_version()").Scan(&rdsCheck)
	if err == nil {
		features.CloudProvider = CloudProviderAWS
		features.Metadata = map[string]interface{}{
			"cloud_variant":  "Aurora",
			"aurora_version": rdsCheck,
		}
		return nil
	}
	
	// Check for Google Cloud SQL
	var gcpCheck string
	err = pd.db.QueryRowContext(ctx, "SHOW cloudsql.iam_authentication").Scan(&gcpCheck)
	if err == nil {
		features.CloudProvider = CloudProviderGCP
		features.Metadata = map[string]interface{}{
			"cloud_variant": "CloudSQL",
		}
		return nil
	}
	
	// Check for Azure Database
	var azureCheck string
	err = pd.db.QueryRowContext(ctx, "SELECT setting FROM pg_settings WHERE name LIKE 'azure.%' LIMIT 1").Scan(&azureCheck)
	if err == nil {
		features.CloudProvider = CloudProviderAzure
		features.Metadata = map[string]interface{}{
			"cloud_variant": "Azure Database",
		}
		return nil
	}
	
	return nil
}