// Package enhancedsql provides an enhanced SQL receiver with feature detection and fallback
package enhancedsql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
	
	"github.com/database-intelligence/common/featuredetector"
	"github.com/database-intelligence/common/queryselector"
	"github.com/database-intelligence/internal/database"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	
	// Database drivers
	_ "github.com/lib/pq"           // PostgreSQL
	_ "github.com/go-sql-driver/mysql" // MySQL
)

// Receiver implements an enhanced SQL receiver with feature detection
type Receiver struct {
	config          *Config
	logger          *zap.Logger
	db              *sql.DB
	detector        featuredetector.Detector
	selector        *queryselector.QuerySelector
	metricsConsumer consumer.Metrics
	logsConsumer    consumer.Logs
	
	// Collection state
	wg             sync.WaitGroup
	cancel         context.CancelFunc
	collectionTick *time.Ticker
	
	// Feature detection state
	lastFeatureCheck time.Time
	featureSet       *featuredetector.FeatureSet
	featureMutex     sync.RWMutex
	
	// Metrics
	successCount   uint64
	errorCount     uint64
	fallbackCount  uint64
}

// NewReceiver creates a new enhanced SQL receiver
func NewReceiver(
	config *Config,
	logger *zap.Logger,
	metricsConsumer consumer.Metrics,
	logsConsumer consumer.Logs,
) (*Receiver, error) {
	return &Receiver{
		config:          config,
		logger:          logger,
		metricsConsumer: metricsConsumer,
		logsConsumer:    logsConsumer,
	}, nil
}

// Start implements the component.Component interface
func (r *Receiver) Start(ctx context.Context, host component.Host) error {
	r.logger.Info("Starting enhanced SQL receiver",
		zap.String("driver", r.config.Driver),
		zap.String("datasource", r.config.getDatasourceMasked()))
	
	// Create connection pool configuration
	poolConfig := database.DefaultConnectionPoolConfig()
	
	// Override with user-provided settings if specified
	if r.config.MaxOpenConnections > 0 {
		poolConfig.MaxOpenConnections = r.config.MaxOpenConnections
	}
	if r.config.MaxIdleConnections > 0 {
		poolConfig.MaxIdleConnections = r.config.MaxIdleConnections
	}
	
	// Validate pool configuration for security
	if err := database.ValidatePoolConfig(poolConfig); err != nil {
		return fmt.Errorf("invalid connection pool configuration: %w", err)
	}
	
	// Connect to database with secure connection pooling
	db, err := database.OpenWithSecurePool(r.config.Driver, r.config.Datasource, poolConfig, r.logger)
	if err != nil {
		return fmt.Errorf("failed to establish secure database connection: %w", err)
	}
	
	r.db = db
	
	// Create feature detector
	detectorConfig := featuredetector.DetectionConfig{
		CacheDuration:      r.config.FeatureDetection.CacheDuration,
		RetryAttempts:      r.config.FeatureDetection.RetryAttempts,
		RetryDelay:         r.config.FeatureDetection.RetryDelay,
		TimeoutPerCheck:    r.config.FeatureDetection.TimeoutPerCheck,
		SkipCloudDetection: r.config.FeatureDetection.SkipCloudDetection,
	}
	
	switch r.config.Driver {
	case "postgres":
		r.detector = featuredetector.NewPostgreSQLDetector(r.db, r.logger, detectorConfig)
	case "mysql":
		r.detector = featuredetector.NewMySQLDetector(r.db, r.logger, detectorConfig)
	default:
		return fmt.Errorf("unsupported driver for feature detection: %s", r.config.Driver)
	}
	
	// Create query selector
	selectorConfig := queryselector.Config{
		CacheDuration: r.config.FeatureDetection.CacheDuration,
	}
	r.selector = queryselector.NewQuerySelector(r.detector, r.logger, selectorConfig)
	
	// Load custom queries if provided
	if err := r.loadCustomQueries(); err != nil {
		return fmt.Errorf("failed to load custom queries: %w", err)
	}
	
	// Perform initial feature detection
	if err := r.detectFeatures(ctx); err != nil {
		r.logger.Warn("Initial feature detection failed", zap.Error(err))
		// Continue anyway - we'll use fallback queries
	}
	
	// Start collection
	collectionCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	
	r.collectionTick = time.NewTicker(r.config.CollectionInterval)
	
	r.wg.Add(1)
	go r.collectionLoop(collectionCtx)
	
	r.logger.Info("Enhanced SQL receiver started successfully")
	return nil
}

// Shutdown implements the component.Component interface
func (r *Receiver) Shutdown(ctx context.Context) error {
	r.logger.Info("Shutting down enhanced SQL receiver")
	
	// Stop collection
	if r.cancel != nil {
		r.cancel()
	}
	
	if r.collectionTick != nil {
		r.collectionTick.Stop()
	}
	
	// Wait for collection to stop
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Collection stopped
	case <-ctx.Done():
		return ctx.Err()
	}
	
	// Close database
	if r.db != nil {
		if err := r.db.Close(); err != nil {
			r.logger.Warn("Error closing database", zap.Error(err))
		}
	}
	
	r.logger.Info("Enhanced SQL receiver shutdown complete",
		zap.Uint64("success_count", r.successCount),
		zap.Uint64("error_count", r.errorCount),
		zap.Uint64("fallback_count", r.fallbackCount))
	
	return nil
}

// collectionLoop runs the main collection loop
func (r *Receiver) collectionLoop(ctx context.Context) {
	defer r.wg.Done()
	
	// Collect immediately
	r.collect(ctx)
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.collectionTick.C:
			// Check if we need to refresh feature detection
			if time.Since(r.lastFeatureCheck) > r.config.FeatureDetection.RefreshInterval {
				if err := r.detectFeatures(ctx); err != nil {
					r.logger.Warn("Feature detection refresh failed", zap.Error(err))
				}
			}
			
			// Collect metrics
			r.collect(ctx)
		}
	}
}

// detectFeatures performs feature detection
func (r *Receiver) detectFeatures(ctx context.Context) error {
	detectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	features, err := r.detector.DetectFeatures(detectCtx)
	if err != nil {
		return err
	}
	
	r.featureMutex.Lock()
	r.featureSet = features
	r.lastFeatureCheck = time.Now()
	r.featureMutex.Unlock()
	
	// Refresh query selection
	if err := r.selector.RefreshSelection(ctx); err != nil {
		r.logger.Warn("Failed to refresh query selection", zap.Error(err))
	}
	
	// Log detected features
	r.logDetectedFeatures(features)
	
	// Emit feature metrics
	r.emitFeatureMetrics(ctx, features)
	
	return nil
}

// logDetectedFeatures logs detected features
func (r *Receiver) logDetectedFeatures(features *featuredetector.FeatureSet) {
	extensions := []string{}
	for name, ext := range features.Extensions {
		if ext.Available {
			extensions = append(extensions, name)
		}
	}
	
	capabilities := []string{}
	for name, cap := range features.Capabilities {
		if cap.Available {
			capabilities = append(capabilities, name)
		}
	}
	
	r.logger.Info("Database features detected",
		zap.String("database_type", features.DatabaseType),
		zap.String("server_version", features.ServerVersion),
		zap.String("cloud_provider", features.CloudProvider),
		zap.Strings("available_extensions", extensions),
		zap.Strings("available_capabilities", capabilities),
		zap.Int("detection_errors", len(features.DetectionErrors)))
}

// loadCustomQueries loads custom query definitions
func (r *Receiver) loadCustomQueries() error {
	for _, queryDef := range r.config.CustomQueries {
		category := queryselector.QueryCategory(queryDef.Category)
		
		query := featuredetector.QueryDefinition{
			Name:         queryDef.Name,
			SQL:          queryDef.SQL,
			Priority:     queryDef.Priority,
			Description:  queryDef.Description,
			Requirements: queryDef.Requirements,
		}
		
		r.selector.AddQuery(category, query)
		
		r.logger.Debug("Loaded custom query",
			zap.String("category", queryDef.Category),
			zap.String("name", queryDef.Name),
			zap.Int("priority", queryDef.Priority))
	}
	
	return nil
}

// emitFeatureMetrics emits metrics about detected features
func (r *Receiver) emitFeatureMetrics(ctx context.Context, features *featuredetector.FeatureSet) {
	timestamp := time.Now()
	
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	
	// Set resource attributes
	resource := rm.Resource()
	resource.Attributes().PutStr("db.system", features.DatabaseType)
	resource.Attributes().PutStr("db.version", features.ServerVersion)
	if features.CloudProvider != "" {
		resource.Attributes().PutStr("cloud.provider", features.CloudProvider)
	}
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("enhancedsql")
	
	// Extension availability metric
	extensionMetric := sm.Metrics().AppendEmpty()
	extensionMetric.SetName("db.feature.extension.available")
	extensionMetric.SetDescription("Database extension availability")
	gauge := extensionMetric.SetEmptyGauge()
	
	for name, ext := range features.Extensions {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pmetric.NewTimestampFromTime(timestamp))
		dp.Attributes().PutStr("extension", name)
		if ext.Available {
			dp.SetIntValue(1)
		} else {
			dp.SetIntValue(0)
		}
	}
	
	// Capability availability metric
	capabilityMetric := sm.Metrics().AppendEmpty()
	capabilityMetric.SetName("db.feature.capability.available")
	capabilityMetric.SetDescription("Database capability availability")
	capGauge := capabilityMetric.SetEmptyGauge()
	
	for name, cap := range features.Capabilities {
		dp := capGauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pmetric.NewTimestampFromTime(timestamp))
		dp.Attributes().PutStr("capability", name)
		if cap.Available {
			dp.SetIntValue(1)
		} else {
			dp.SetIntValue(0)
		}
	}
	
	// Send metrics
	if r.metricsConsumer != nil {
		if err := r.metricsConsumer.ConsumeMetrics(ctx, metrics); err != nil {
			r.logger.Warn("Failed to send feature metrics", zap.Error(err))
		}
	}
}

// collect performs the main data collection
func (r *Receiver) collect(ctx context.Context) {
	startTime := time.Now()
	
	// Get current feature set for intelligent query selection
	r.featureMutex.RLock()
	features := r.featureSet
	r.featureMutex.RUnlock()
	
	// Collect metrics and logs
	if err := r.collectMetrics(ctx, features); err != nil {
		r.errorCount++
		r.logger.Error("Failed to collect metrics", zap.Error(err))
	}
	
	if err := r.collectLogs(ctx, features); err != nil {
		r.errorCount++
		r.logger.Error("Failed to collect logs", zap.Error(err))
	}
	
	r.successCount++
	
	duration := time.Since(startTime)
	r.logger.Debug("Collection completed",
		zap.Duration("duration", duration),
		zap.Uint64("success_count", r.successCount),
		zap.Uint64("error_count", r.errorCount))
}

// collectMetrics executes queries and collects metrics
func (r *Receiver) collectMetrics(ctx context.Context, features *featuredetector.FeatureSet) error {
	collectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	// Get appropriate queries for current features
	queries := r.selector.GetQueriesForCategory(queryselector.CategoryMetrics)
	if len(queries) == 0 {
		r.fallbackCount++
		r.logger.Debug("No feature-specific queries available, using fallback")
		queries = r.getFallbackMetricQueries()
	}
	
	timestamp := time.Now()
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	
	// Set resource attributes
	resource := rm.Resource()
	if features != nil {
		resource.Attributes().PutStr("db.system", features.DatabaseType)
		resource.Attributes().PutStr("db.version", features.ServerVersion)
		if features.CloudProvider != "" {
			resource.Attributes().PutStr("cloud.provider", features.CloudProvider)
		}
	}
	resource.Attributes().PutStr("db.connection.string", r.config.getDatasourceMasked())
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("enhancedsql")
	sm.Scope().SetVersion("1.0.0")
	
	// Execute queries and create metrics
	for _, queryDef := range queries {
		if err := r.executeMetricQuery(collectCtx, queryDef, sm, timestamp); err != nil {
			r.logger.Warn("Failed to execute metric query",
				zap.String("query", queryDef.Name),
				zap.Error(err))
			continue
		}
	}
	
	// Send metrics if we have any
	if metrics.MetricCount() > 0 && r.metricsConsumer != nil {
		if err := r.metricsConsumer.ConsumeMetrics(collectCtx, metrics); err != nil {
			return fmt.Errorf("failed to send metrics: %w", err)
		}
	}
	
	return nil
}

// collectLogs collects logs including pg_querylens data if available
func (r *Receiver) collectLogs(ctx context.Context, features *featuredetector.FeatureSet) error {
	if r.logsConsumer == nil {
		return nil
	}
	
	collectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	// Get log queries (includes pg_querylens if available)
	queries := r.selector.GetQueriesForCategory(queryselector.CategoryLogs)
	if len(queries) == 0 {
		return nil // No log collection needed
	}
	
	timestamp := time.Now()
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	
	// Set resource attributes
	resource := rl.Resource()
	if features != nil {
		resource.Attributes().PutStr("db.system", features.DatabaseType)
		resource.Attributes().PutStr("db.version", features.ServerVersion)
	}
	
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("enhancedsql")
	
	// Execute log queries
	for _, queryDef := range queries {
		if err := r.executeLogQuery(collectCtx, queryDef, sl, timestamp); err != nil {
			r.logger.Warn("Failed to execute log query",
				zap.String("query", queryDef.Name),
				zap.Error(err))
			continue
		}
	}
	
	// Send logs if we have any
	if logs.LogRecordCount() > 0 {
		if err := r.logsConsumer.ConsumeLogs(collectCtx, logs); err != nil {
			return fmt.Errorf("failed to send logs: %w", err)
		}
	}
	
	return nil
}

// executeMetricQuery executes a single metric query
func (r *Receiver) executeMetricQuery(ctx context.Context, queryDef featuredetector.QueryDefinition, sm pmetric.ScopeMetrics, timestamp time.Time) error {
	rows, err := r.db.QueryContext(ctx, queryDef.SQL)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()
	
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}
	
	// Create metric based on query results
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(queryDef.Name)
	metric.SetDescription(queryDef.Description)
	
	gauge := metric.SetEmptyGauge()
	
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			r.logger.Warn("Failed to scan row", zap.Error(err))
			continue
		}
		
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pmetric.NewTimestampFromTime(timestamp))
		
		// Process columns - first numeric column is the value, others become attributes
		var metricValue float64
		valueSet := false
		
		for i, column := range columns {
			value := values[i]
			
			switch v := value.(type) {
			case int64:
				if !valueSet {
					metricValue = float64(v)
					valueSet = true
				} else {
					dp.Attributes().PutInt(column, v)
				}
			case float64:
				if !valueSet {
					metricValue = v
					valueSet = true
				} else {
					dp.Attributes().PutDouble(column, v)
				}
			case string:
				dp.Attributes().PutStr(column, v)
			case []byte:
				dp.Attributes().PutStr(column, string(v))
			case bool:
				if !valueSet {
					if v {
						metricValue = 1
					} else {
						metricValue = 0
					}
					valueSet = true
				} else {
					dp.Attributes().PutBool(column, v)
				}
			default:
				if value != nil {
					dp.Attributes().PutStr(column, fmt.Sprintf("%v", value))
				}
			}
		}
		
		if valueSet {
			dp.SetDoubleValue(metricValue)
		}
	}
	
	return rows.Err()
}

// executeLogQuery executes a query and creates log records
func (r *Receiver) executeLogQuery(ctx context.Context, queryDef featuredetector.QueryDefinition, sl plog.ScopeLogs, timestamp time.Time) error {
	rows, err := r.db.QueryContext(ctx, queryDef.SQL)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()
	
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}
	
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		if err := rows.Scan(valuePtrs...); err != nil {
			r.logger.Warn("Failed to scan row", zap.Error(err))
			continue
		}
		
		// Create log record
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.SetTimestamp(plog.NewTimestampFromTime(timestamp))
		logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
		logRecord.SetSeverityText("INFO")
		
		// Build log body with query results
		bodyBuilder := strings.Builder{}
		bodyBuilder.WriteString(fmt.Sprintf("Query: %s | ", queryDef.Name))
		
		// Add all values as attributes and build body
		for i, column := range columns {
			value := values[i]
			
			if i > 0 {
				bodyBuilder.WriteString(" | ")
			}
			
			switch v := value.(type) {
			case int64:
				logRecord.Attributes().PutInt(column, v)
				bodyBuilder.WriteString(fmt.Sprintf("%s: %d", column, v))
			case float64:
				logRecord.Attributes().PutDouble(column, v)
				bodyBuilder.WriteString(fmt.Sprintf("%s: %.6f", column, v))
			case string:
				logRecord.Attributes().PutStr(column, v)
				bodyBuilder.WriteString(fmt.Sprintf("%s: %s", column, v))
			case []byte:
				strVal := string(v)
				logRecord.Attributes().PutStr(column, strVal)
				bodyBuilder.WriteString(fmt.Sprintf("%s: %s", column, strVal))
			case bool:
				logRecord.Attributes().PutBool(column, v)
				bodyBuilder.WriteString(fmt.Sprintf("%s: %t", column, v))
			case time.Time:
				timestamp := v.Format(time.RFC3339)
				logRecord.Attributes().PutStr(column, timestamp)
				bodyBuilder.WriteString(fmt.Sprintf("%s: %s", column, timestamp))
			default:
				if value != nil {
					strVal := fmt.Sprintf("%v", value)
					logRecord.Attributes().PutStr(column, strVal)
					bodyBuilder.WriteString(fmt.Sprintf("%s: %s", column, strVal))
				}
			}
		}
		
		logRecord.Body().SetStr(bodyBuilder.String())
		
		// Add standard attributes
		logRecord.Attributes().PutStr("source", "enhancedsql")
		logRecord.Attributes().PutStr("query_name", queryDef.Name)
		logRecord.Attributes().PutStr("query_category", "database_telemetry")
	}
	
	return rows.Err()
}

// getFallbackMetricQueries returns basic queries when feature detection fails
func (r *Receiver) getFallbackMetricQueries() []featuredetector.QueryDefinition {
	queries := []featuredetector.QueryDefinition{}
	
	switch r.config.Driver {
	case "postgres":
		queries = append(queries, featuredetector.QueryDefinition{
			Name:        "pg_database_size",
			SQL:         "SELECT pg_database_size(current_database()) as database_size_bytes",
			Description: "Current database size in bytes",
			Priority:    100,
		})
		
		queries = append(queries, featuredetector.QueryDefinition{
			Name:        "pg_connection_count",
			SQL:         "SELECT count(*) as active_connections FROM pg_stat_activity WHERE state = 'active'",
			Description: "Number of active database connections",
			Priority:    90,
		})
		
	case "mysql":
		queries = append(queries, featuredetector.QueryDefinition{
			Name:        "mysql_connection_count",
			SQL:         "SELECT VARIABLE_VALUE as active_connections FROM performance_schema.global_status WHERE VARIABLE_NAME = 'Threads_connected'",
			Description: "Number of active MySQL connections",
			Priority:    90,
		})
	}
	
	return queries
}