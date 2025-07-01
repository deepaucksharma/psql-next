// Package enhancedsql provides an enhanced SQL receiver with feature detection and fallback
package enhancedsql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
	
	"github.com/database-intelligence-mvp/common/featuredetector"
	"github.com/database-intelligence-mvp/common/queryselector"
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
	
	// Connect to database
	db, err := sql.Open(r.config.Driver, r.config.Datasource)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	
	// Configure connection pool
	if r.config.MaxOpenConnections > 0 {
		db.SetMaxOpenConns(r.config.MaxOpenConnections)
	}
	if r.config.MaxIdleConnections > 0 {
		db.SetMaxIdleConns(r.config.MaxIdleConnections)
	}
	
	// Test connection
	testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(testCtx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
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