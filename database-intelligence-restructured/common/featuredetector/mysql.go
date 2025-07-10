package featuredetector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	
	"go.uber.org/zap"
)

// MySQLDetector implements feature detection for MySQL
type MySQLDetector struct {
	*BaseDetector
}

// MySQL-specific features
const (
	// Performance Schema tables
	CapPerfSchemaEnabled          = "performance_schema_enabled"
	CapPerfSchemaStatementsDigest = "events_statements_summary_by_digest"
	CapPerfSchemaWaits           = "events_waits_summary"
	CapPerfSchemaStages          = "events_stages_summary"
	
	// MySQL specific
	CapSlowQueryLog      = "slow_query_log"
	CapGeneralLog        = "general_log"
	CapInnoDBMetrics     = "innodb_metrics"
	CapQueryCacheEnabled = "query_cache_enabled"
	
	// Cloud variants
	MySQLVariantPercona  = "Percona"
	MySQLVariantMariaDB  = "MariaDB"
	MySQLVariantAurora   = "Aurora"
	MySQLVariantStandard = "MySQL"
)

// NewMySQLDetector creates a new MySQL feature detector
func NewMySQLDetector(db *sql.DB, logger *zap.Logger, config DetectionConfig) *MySQLDetector {
	return &MySQLDetector{
		BaseDetector: NewBaseDetector(db, logger, config),
	}
}

// DetectFeatures performs comprehensive feature detection for MySQL
func (md *MySQLDetector) DetectFeatures(ctx context.Context) (*FeatureSet, error) {
	// Check cache first
	if cached := md.GetCachedFeatures(); cached != nil {
		md.logger.Debug("Using cached feature detection results")
		return cached, nil
	}
	
	md.logger.Info("Starting MySQL feature detection")
	
	features := &FeatureSet{
		DatabaseType:    DatabaseTypeMySQL,
		Extensions:      make(map[string]*Feature), // MySQL uses features, not extensions
		Capabilities:    make(map[string]*Feature),
		LastDetection:   time.Now(),
		DetectionErrors: []string{},
	}
	
	// Detect server version and variant
	if err := md.detectServerVersion(ctx, features); err != nil {
		features.DetectionErrors = append(features.DetectionErrors,
			fmt.Sprintf("version detection: %v", err))
		md.logger.Warn("Failed to detect server version", zap.Error(err))
	}
	
	// Detect Performance Schema
	if err := md.detectPerformanceSchema(ctx, features); err != nil {
		features.DetectionErrors = append(features.DetectionErrors,
			fmt.Sprintf("performance schema detection: %v", err))
		md.logger.Warn("Failed to detect performance schema", zap.Error(err))
	}
	
	// Detect capabilities
	if err := md.detectCapabilities(ctx, features); err != nil {
		features.DetectionErrors = append(features.DetectionErrors,
			fmt.Sprintf("capability detection: %v", err))
		md.logger.Warn("Failed to detect capabilities", zap.Error(err))
	}
	
	// Detect cloud provider
	if !md.detectionConfig.SkipCloudDetection {
		if err := md.detectCloudProvider(ctx, features); err != nil {
			features.DetectionErrors = append(features.DetectionErrors,
				fmt.Sprintf("cloud detection: %v", err))
			md.logger.Warn("Failed to detect cloud provider", zap.Error(err))
		}
	}
	
	// Cache results
	md.SetCache(features)
	
	md.logger.Info("MySQL feature detection completed",
		zap.Int("features", len(features.Extensions)),
		zap.Int("capabilities", len(features.Capabilities)),
		zap.String("cloud_provider", features.CloudProvider),
		zap.Int("errors", len(features.DetectionErrors)))
	
	return features, nil
}

// detectServerVersion detects MySQL server version and variant
func (md *MySQLDetector) detectServerVersion(ctx context.Context, features *FeatureSet) error {
	var version, versionComment string
	
	err := md.db.QueryRowContext(ctx, "SELECT VERSION(), @@version_comment").Scan(&version, &versionComment)
	if err != nil {
		return &DetectionError{
			Phase:   "version",
			Message: "failed to query version",
			Err:     err,
		}
	}
	
	features.ServerVersion = version
	
	// Detect MySQL variant
	variant := MySQLVariantStandard
	if strings.Contains(strings.ToLower(version), "percona") {
		variant = MySQLVariantPercona
	} else if strings.Contains(strings.ToLower(version), "mariadb") {
		variant = MySQLVariantMariaDB
	} else if strings.Contains(strings.ToLower(versionComment), "aurora") {
		variant = MySQLVariantAurora
	}
	
	if features.Metadata == nil {
		features.Metadata = make(map[string]interface{})
	}
	features.Metadata["variant"] = variant
	features.Metadata["version_comment"] = versionComment
	
	// Parse major version
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		features.Metadata["major_version"] = parts[0] + "." + parts[1]
	}
	
	return nil
}

// detectPerformanceSchema detects Performance Schema availability
func (md *MySQLDetector) detectPerformanceSchema(ctx context.Context, features *FeatureSet) error {
	// Check if Performance Schema is enabled
	var perfSchemaEnabled string
	err := md.db.QueryRowContext(ctx, "SELECT @@performance_schema").Scan(&perfSchemaEnabled)
	if err != nil {
		features.Capabilities[CapPerfSchemaEnabled] = &Feature{
			Name:         CapPerfSchemaEnabled,
			Available:    false,
			LastChecked:  time.Now(),
			ErrorMessage: err.Error(),
		}
		return nil
	}
	
	enabled := perfSchemaEnabled == "1" || strings.ToLower(perfSchemaEnabled) == "on"
	features.Capabilities[CapPerfSchemaEnabled] = &Feature{
		Name:        CapPerfSchemaEnabled,
		Available:   enabled,
		LastChecked: time.Now(),
	}
	
	if !enabled {
		return nil
	}
	
	// Check specific Performance Schema tables
	// Using a safe approach with predefined queries to avoid SQL injection patterns
	perfSchemaQueries := map[string]string{
		CapPerfSchemaStatementsDigest: "SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest LIMIT 1",
		CapPerfSchemaWaits:           "SELECT COUNT(*) FROM performance_schema.events_waits_summary_global_by_event_name LIMIT 1",
		CapPerfSchemaStages:          "SELECT COUNT(*) FROM performance_schema.events_stages_summary_global_by_event_name LIMIT 1",
	}
	
	for capability, query := range perfSchemaQueries {
		var count int
		err := md.db.QueryRowContext(ctx, query).Scan(&count)
		
		features.Capabilities[capability] = &Feature{
			Name:        capability,
			Available:   err == nil,
			LastChecked: time.Now(),
		}
		
		if err != nil {
			features.Capabilities[capability].ErrorMessage = "table not accessible"
		}
	}
	
	// Check if statement instrumentation is enabled
	var stmtEnabled int
	err = md.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM performance_schema.setup_instruments 
		WHERE name LIKE 'statement/%' 
			AND enabled = 'YES'
	`).Scan(&stmtEnabled)
	
	if err == nil && stmtEnabled > 0 {
		features.Capabilities["statement_instrumentation"] = &Feature{
			Name:        "statement_instrumentation",
			Available:   true,
			LastChecked: time.Now(),
			Metadata: map[string]interface{}{
				"enabled_count": stmtEnabled,
			},
		}
	}
	
	return nil
}

// detectCapabilities detects MySQL configuration capabilities
func (md *MySQLDetector) detectCapabilities(ctx context.Context, features *FeatureSet) error {
	// Check various MySQL settings
	capabilityQueries := map[string]struct {
		query     string
		checkFunc func(value string) bool
	}{
		CapSlowQueryLog: {
			query:     "SELECT @@slow_query_log",
			checkFunc: func(v string) bool { return v == "1" || strings.ToLower(v) == "on" },
		},
		CapGeneralLog: {
			query:     "SELECT @@general_log",
			checkFunc: func(v string) bool { return v == "1" || strings.ToLower(v) == "on" },
		},
		CapQueryCacheEnabled: {
			query:     "SELECT @@query_cache_type",
			checkFunc: func(v string) bool { return v != "0" && strings.ToLower(v) != "off" },
		},
		"long_query_time": {
			query:     "SELECT @@long_query_time",
			checkFunc: func(v string) bool { return true }, // Just store the value
		},
		"max_connections": {
			query:     "SELECT @@max_connections",
			checkFunc: func(v string) bool { return true },
		},
	}
	
	for capability, check := range capabilityQueries {
		var value string
		err := md.db.QueryRowContext(ctx, check.query).Scan(&value)
		
		if err != nil {
			features.Capabilities[capability] = &Feature{
				Name:         capability,
				Available:    false,
				LastChecked:  time.Now(),
				ErrorMessage: err.Error(),
			}
			continue
		}
		
		features.Capabilities[capability] = &Feature{
			Name:        capability,
			Available:   check.checkFunc(value),
			Version:     value,
			LastChecked: time.Now(),
		}
	}
	
	// Check InnoDB metrics
	md.checkInnoDBMetrics(ctx, features)
	
	// Check processlist access
	md.checkProcessListAccess(ctx, features)
	
	return nil
}

// checkInnoDBMetrics checks InnoDB-specific metrics availability
func (md *MySQLDetector) checkInnoDBMetrics(ctx context.Context, features *FeatureSet) error {
	// Check if we can query InnoDB status
	var unused string
	err := md.db.QueryRowContext(ctx, "SHOW ENGINE INNODB STATUS").Scan(&unused, &unused, &unused)
	
	features.Capabilities[CapInnoDBMetrics] = &Feature{
		Name:        CapInnoDBMetrics,
		Available:   err == nil,
		LastChecked: time.Now(),
	}
	
	if err != nil {
		features.Capabilities[CapInnoDBMetrics].ErrorMessage = "cannot access InnoDB status"
	}
	
	// Check information_schema.innodb_metrics (MySQL 5.6+)
	var metricsCount int
	err = md.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.innodb_metrics").Scan(&metricsCount)
	
	if err == nil {
		features.Capabilities["innodb_metrics_table"] = &Feature{
			Name:        "innodb_metrics_table",
			Available:   true,
			LastChecked: time.Now(),
			Metadata: map[string]interface{}{
				"metrics_count": metricsCount,
			},
		}
	}
	
	return nil
}

// checkProcessListAccess checks if we can query processlist
func (md *MySQLDetector) checkProcessListAccess(ctx context.Context, features *FeatureSet) error {
	var count int
	err := md.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.processlist").Scan(&count)
	
	features.Capabilities["processlist_access"] = &Feature{
		Name:        "processlist_access",
		Available:   err == nil,
		LastChecked: time.Now(),
	}
	
	if err != nil {
		features.Capabilities["processlist_access"].ErrorMessage = "cannot access processlist"
	}
	
	return nil
}

// detectCloudProvider detects if MySQL is running on a cloud provider
func (md *MySQLDetector) detectCloudProvider(ctx context.Context, features *FeatureSet) error {
	features.CloudProvider = CloudProviderLocal // Default
	
	// Check for Aurora MySQL
	variant, _ := features.Metadata["variant"].(string)
	if variant == MySQLVariantAurora {
		features.CloudProvider = CloudProviderAWS
		features.Metadata["cloud_variant"] = "Aurora MySQL"
		
		// Try to get Aurora version
		var auroraVersion string
		if err := md.db.QueryRowContext(ctx, "SELECT AURORA_VERSION()").Scan(&auroraVersion); err == nil {
			features.Metadata["aurora_version"] = auroraVersion
		}
		return nil
	}
	
	// Check for AWS RDS MySQL
	var rdsCheck string
	err := md.db.QueryRowContext(ctx, "SELECT @@basedir").Scan(&rdsCheck)
	if err == nil && strings.Contains(rdsCheck, "/rdsdbbin/") {
		features.CloudProvider = CloudProviderAWS
		features.Metadata["cloud_variant"] = "RDS MySQL"
		return nil
	}
	
	// Check for Google Cloud SQL MySQL
	err = md.db.QueryRowContext(ctx, "SELECT @@hostname").Scan(&rdsCheck)
	if err == nil && strings.Contains(rdsCheck, ".cloudsql.") {
		features.CloudProvider = CloudProviderGCP
		features.Metadata["cloud_variant"] = "Cloud SQL MySQL"
		return nil
	}
	
	// Check for Azure Database for MySQL
	var azureCheck string
	err = md.db.QueryRowContext(ctx, "SHOW VARIABLES LIKE 'azure%'").Scan(&azureCheck, &azureCheck)
	if err == nil {
		features.CloudProvider = CloudProviderAzure
		features.Metadata["cloud_variant"] = "Azure Database for MySQL"
		return nil
	}
	
	return nil
}