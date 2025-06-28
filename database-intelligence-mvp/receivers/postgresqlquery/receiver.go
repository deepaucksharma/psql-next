// Package postgresqlquery implements a PostgreSQL query receiver for OpenTelemetry Collector
package postgresqlquery

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/lib/pq"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// postgresqlQueryReceiver implements the receiver.Metrics and receiver.Logs interfaces
type postgresqlQueryReceiver struct {
	config           *Config
	logger           *zap.Logger
	metricsConsumer  consumer.Metrics
	logsConsumer     consumer.Logs
	
	// Database connections - one per configured database
	connections      map[string]*dbConnection
	
	// State management
	planCache        *PlanCache
	queryFingerprints *QueryFingerprintCache
	collectionStats  *CollectionStats
	
	// Control
	wg               sync.WaitGroup
	shutdownChan     chan struct{}
	ticker           *time.Ticker
}

// dbConnection represents a connection to a specific database
type dbConnection struct {
	db               *sql.DB
	dsn              string
	name             string
	capabilities     map[string]bool
	lastError        error
	errorCount       int
}

// PlanCache tracks query execution plans for regression detection
type PlanCache struct {
	mu    sync.RWMutex
	cache map[string]*PlanInfo // key: database_name:query_id
}

type PlanInfo struct {
	Hash         string
	Cost         float64
	Timestamp    time.Time
	NodeCount    int
	LastSeen     time.Time
	ChangeCount  int
}

// QueryFingerprintCache normalizes and tracks query patterns
type QueryFingerprintCache struct {
	mu    sync.RWMutex
	cache map[string]*QueryFingerprint
}

type QueryFingerprint struct {
	NormalizedText string
	ParameterCount int
	Tables         []string
	FirstSeen      time.Time
	LastSeen       time.Time
	TotalExecutions int64
}

// CollectionStats tracks receiver performance
type CollectionStats struct {
	mu                     sync.RWMutex
	LastCollectionDuration time.Duration
	QueriesCollected       int64
	ErrorsEncountered      int64
	PlanChangesDetected    int64
	DatabasesMonitored     int
}

// Ensure interfaces are implemented
var _ receiver.Metrics = (*postgresqlQueryReceiver)(nil)
var _ receiver.Logs = (*postgresqlQueryReceiver)(nil)

// newPostgresqlQueryReceiver creates a new PostgreSQL query receiver
func newPostgresqlQueryReceiver(
	cfg *Config,
	logger *zap.Logger,
	metricsConsumer consumer.Metrics,
	logsConsumer consumer.Logs,
) (*postgresqlQueryReceiver, error) {
	
	return &postgresqlQueryReceiver{
		config:           cfg,
		logger:           logger,
		metricsConsumer:  metricsConsumer,
		logsConsumer:     logsConsumer,
		connections:      make(map[string]*dbConnection),
		planCache:        &PlanCache{cache: make(map[string]*PlanInfo)},
		queryFingerprints: &QueryFingerprintCache{cache: make(map[string]*QueryFingerprint)},
		collectionStats:  &CollectionStats{},
		shutdownChan:     make(chan struct{}),
	}, nil
}

// Start implements the receiver.Metrics interface
func (r *postgresqlQueryReceiver) Start(ctx context.Context, host component.Host) error {
	r.logger.Info("Starting PostgreSQL query receiver", 
		zap.Int("databases", len(r.config.Databases)),
		zap.Duration("interval", r.config.CollectionInterval))
	
	// Initialize connections for each database
	for _, dbConfig := range r.config.Databases {
		if err := r.initializeConnection(dbConfig); err != nil {
			// Log error but continue with other databases
			r.logger.Error("Failed to initialize database connection",
				zap.String("database", dbConfig.Name),
				zap.Error(err))
			continue
		}
	}
	
	if len(r.connections) == 0 {
		return fmt.Errorf("no database connections could be initialized")
	}
	
	// Start collection ticker
	r.ticker = time.NewTicker(r.config.CollectionInterval)
	
	// Start collection loop
	r.wg.Add(1)
	go r.collectionLoop(ctx)
	
	// Start stats reporter
	r.wg.Add(1)
	go r.statsReporter(ctx)
	
	return nil
}

// Shutdown implements the receiver.Metrics interface
func (r *postgresqlQueryReceiver) Shutdown(ctx context.Context) error {
	r.logger.Info("Shutting down PostgreSQL query receiver")
	
	// Stop ticker
	if r.ticker != nil {
		r.ticker.Stop()
	}
	
	// Signal shutdown
	close(r.shutdownChan)
	
	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		r.logger.Info("All goroutines stopped")
	case <-time.After(30 * time.Second):
		r.logger.Warn("Shutdown timeout exceeded")
	}
	
	// Close all database connections
	for name, conn := range r.connections {
		if err := conn.db.Close(); err != nil {
			r.logger.Error("Failed to close database connection",
				zap.String("database", name),
				zap.Error(err))
		}
	}
	
	return nil
}

// initializeConnection establishes connection to a database and detects capabilities
func (r *postgresqlQueryReceiver) initializeConnection(dbConfig DatabaseConfig) error {
	db, err := sql.Open("postgres", dbConfig.DSN)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	
	// Configure connection pool
	db.SetMaxOpenConns(dbConfig.MaxOpenConnections)
	db.SetMaxIdleConns(dbConfig.MaxIdleConnections)
	db.SetConnMaxLifetime(dbConfig.ConnectionMaxLifetime)
	db.SetConnMaxIdleTime(dbConfig.ConnectionMaxIdleTime)
	
	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}
	
	// Detect capabilities
	capabilities, err := r.detectCapabilities(ctx, db)
	if err != nil {
		r.logger.Warn("Failed to detect all capabilities",
			zap.String("database", dbConfig.Name),
			zap.Error(err))
		capabilities = make(map[string]bool) // Continue with empty capabilities
	}
	
	conn := &dbConnection{
		db:           db,
		dsn:          dbConfig.DSN,
		name:         dbConfig.Name,
		capabilities: capabilities,
	}
	
	r.connections[dbConfig.Name] = conn
	
	r.logger.Info("Initialized database connection",
		zap.String("database", dbConfig.Name),
		zap.Any("capabilities", capabilities))
	
	return nil
}

// detectCapabilities checks what PostgreSQL features are available
func (r *postgresqlQueryReceiver) detectCapabilities(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	capabilities := make(map[string]bool)
	
	// Check PostgreSQL version
	var version string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err == nil {
		r.logger.Debug("PostgreSQL version", zap.String("version", version))
		
		// Detect cloud providers
		if contains(version, "rds") || contains(version, "aurora") {
			capabilities["is_rds"] = true
		} else if contains(version, "CloudSQL") {
			capabilities["is_cloud_sql"] = true
		} else if contains(version, "Database for PostgreSQL") {
			capabilities["is_azure"] = true
		}
	}
	
	// Check for pg_stat_statements
	var hasStatements bool
	err = db.QueryRowContext(ctx, 
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements')").Scan(&hasStatements)
	if err == nil && hasStatements {
		capabilities["pg_stat_statements"] = true
		
		// Check if track_io_timing is enabled
		var trackIO string
		err = db.QueryRowContext(ctx,
			"SELECT setting FROM pg_settings WHERE name = 'track_io_timing'").Scan(&trackIO)
		if err == nil && trackIO == "on" {
			capabilities["track_io_timing"] = true
		}
		
		// Check pg_stat_statements.track setting
		var track string
		err = db.QueryRowContext(ctx,
			"SELECT setting FROM pg_settings WHERE name = 'pg_stat_statements.track'").Scan(&track)
		if err == nil && (track == "all" || track == "top") {
			capabilities["pg_stat_statements_track_all"] = true
		}
	}
	
	// Check for pg_wait_sampling (not available on RDS)
	if !capabilities["is_rds"] {
		var hasWaitSampling bool
		err = db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_wait_sampling')").Scan(&hasWaitSampling)
		if err == nil && hasWaitSampling {
			capabilities["pg_wait_sampling"] = true
		}
	}
	
	// Check for pg_stat_kcache
	var hasKcache bool
	err = db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_kcache')").Scan(&hasKcache)
	if err == nil && hasKcache {
		capabilities["pg_stat_kcache"] = true
	}
	
	// Check for auto_explain
	var autoExplainDuration string
	err = db.QueryRowContext(ctx,
		"SELECT setting FROM pg_settings WHERE name = 'auto_explain.log_min_duration'").Scan(&autoExplainDuration)
	if err == nil && autoExplainDuration != "-1" {
		capabilities["auto_explain"] = true
	}
	
	// Check if we can read query plans (requires appropriate permissions)
	var canExplain bool
	_, err = db.ExecContext(ctx, "EXPLAIN (FORMAT JSON) SELECT 1")
	if err == nil {
		canExplain = true
	}
	capabilities["can_explain"] = canExplain
	
	return capabilities, nil
}

// collectionLoop runs the main collection cycle
func (r *postgresqlQueryReceiver) collectionLoop(ctx context.Context) {
	defer r.wg.Done()
	
	// Run initial collection
	r.collect(ctx)
	
	for {
		select {
		case <-r.ticker.C:
			r.collect(ctx)
		case <-r.shutdownChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// collect performs one collection cycle across all databases
func (r *postgresqlQueryReceiver) collect(ctx context.Context) {
	start := time.Now()
	
	metrics := pmetric.NewMetrics()
	logs := plog.NewLogs()
	
	// Collect from each database in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for name, conn := range r.connections {
		wg.Add(1)
		go func(dbName string, dbConn *dbConnection) {
			defer wg.Done()
			
			dbMetrics, dbLogs, err := r.collectFromDatabase(ctx, dbName, dbConn)
			if err != nil {
				r.logger.Error("Failed to collect from database",
					zap.String("database", dbName),
					zap.Error(err))
				
				dbConn.lastError = err
				dbConn.errorCount++
				
				// Circuit breaker logic
				if dbConn.errorCount > r.config.MaxErrorsPerDatabase {
					r.logger.Warn("Database circuit breaker triggered",
						zap.String("database", dbName),
						zap.Int("error_count", dbConn.errorCount))
				}
				return
			}
			
			// Reset error count on success
			dbConn.errorCount = 0
			dbConn.lastError = nil
			
			// Merge results
			mu.Lock()
			dbMetrics.ResourceMetrics().MoveAndAppendTo(metrics.ResourceMetrics())
			dbLogs.ResourceLogs().MoveAndAppendTo(logs.ResourceLogs())
			mu.Unlock()
		}(name, conn)
	}
	
	wg.Wait()
	
	// Update stats
	r.collectionStats.mu.Lock()
	r.collectionStats.LastCollectionDuration = time.Since(start)
	r.collectionStats.DatabasesMonitored = len(r.connections)
	r.collectionStats.mu.Unlock()
	
	// Send to consumers
	if metrics.MetricCount() > 0 {
		if err := r.metricsConsumer.ConsumeMetrics(ctx, metrics); err != nil {
			r.logger.Error("Failed to consume metrics", zap.Error(err))
		}
	}
	
	if logs.LogRecordCount() > 0 {
		if err := r.logsConsumer.ConsumeLogs(ctx, logs); err != nil {
			r.logger.Error("Failed to consume logs", zap.Error(err))
		}
	}
}

// collectFromDatabase collects metrics and logs from a single database
func (r *postgresqlQueryReceiver) collectFromDatabase(
	ctx context.Context,
	dbName string,
	conn *dbConnection,
) (pmetric.Metrics, plog.Logs, error) {
	
	metrics := pmetric.NewMetrics()
	logs := plog.NewLogs()
	
	// Create resource for this database
	resource := metrics.ResourceMetrics().AppendEmpty()
	resourceAttrs := resource.Resource().Attributes()
	resourceAttrs.PutStr("service.name", "postgresql")
	resourceAttrs.PutStr("db.system", "postgresql")
	resourceAttrs.PutStr("db.name", dbName)
	resourceAttrs.PutStr("db.connection.string", sanitizeDSN(conn.dsn))
	
	// Copy resource to logs
	logResource := logs.ResourceLogs().AppendEmpty()
	resource.Resource().CopyTo(logResource.Resource())
	
	// Collect different metric types based on capabilities
	var wg sync.WaitGroup
	var mu sync.Mutex
	var collectionErrors []error
	
	// Database-level metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := r.collectDatabaseMetrics(ctx, conn, resource); err != nil {
			mu.Lock()
			collectionErrors = append(collectionErrors, fmt.Errorf("database metrics: %w", err))
			mu.Unlock()
		}
	}()
	
	// Slow queries
	if conn.capabilities["pg_stat_statements"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.collectSlowQueries(ctx, conn, resource, logResource); err != nil {
				mu.Lock()
				collectionErrors = append(collectionErrors, fmt.Errorf("slow queries: %w", err))
				mu.Unlock()
			}
		}()
	}
	
	// Wait events
	if conn.capabilities["pg_wait_sampling"] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.collectWaitEvents(ctx, conn, resource); err != nil {
				mu.Lock()
				collectionErrors = append(collectionErrors, fmt.Errorf("wait events: %w", err))
				mu.Unlock()
			}
		}()
	} else {
		// Fallback to sampling pg_stat_activity
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.collectWaitEventsFromActivity(ctx, conn, resource); err != nil {
				mu.Lock()
				collectionErrors = append(collectionErrors, fmt.Errorf("wait events fallback: %w", err))
				mu.Unlock()
			}
		}()
	}
	
	// Table and index statistics
	if !r.config.MinimalMode {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.collectTableMetrics(ctx, conn, resource); err != nil {
				mu.Lock()
				collectionErrors = append(collectionErrors, fmt.Errorf("table metrics: %w", err))
				mu.Unlock()
			}
		}()
	}
	
	wg.Wait()
	
	// Return first error if any
	if len(collectionErrors) > 0 {
		return metrics, logs, collectionErrors[0]
	}
	
	return metrics, logs, nil
}

// collectDatabaseMetrics collects database-level statistics
func (r *postgresqlQueryReceiver) collectDatabaseMetrics(
	ctx context.Context,
	conn *dbConnection,
	resource pmetric.ResourceMetrics,
) error {
	query := `
		WITH db_stats AS (
			SELECT 
				pg_database_size(current_database()) as size_bytes,
				numbackends,
				xact_commit,
				xact_rollback,
				blks_read,
				blks_hit,
				tup_returned,
				tup_fetched,
				tup_inserted,
				tup_updated,
				tup_deleted,
				conflicts,
				temp_files,
				temp_bytes,
				deadlocks,
				checksum_failures,
				blk_read_time,
				blk_write_time
			FROM pg_stat_database
			WHERE datname = current_database()
		),
		connections AS (
			SELECT 
				COUNT(*) FILTER (WHERE state = 'active') as active,
				COUNT(*) FILTER (WHERE state = 'idle') as idle,
				COUNT(*) FILTER (WHERE state = 'idle in transaction') as idle_in_transaction,
				COUNT(*) FILTER (WHERE wait_event IS NOT NULL) as waiting
			FROM pg_stat_activity
			WHERE datname = current_database()
		)
		SELECT 
			ds.*,
			c.active as active_connections,
			c.idle as idle_connections,
			c.idle_in_transaction as idle_in_transaction_connections,
			c.waiting as waiting_connections
		FROM db_stats ds, connections c
	`
	
	var stats struct {
		SizeBytes                   int64
		NumBackends                 int32
		XactCommit                  int64
		XactRollback                int64
		BlksRead                    int64
		BlksHit                     int64
		TupReturned                 int64
		TupFetched                  int64
		TupInserted                 int64
		TupUpdated                  int64
		TupDeleted                  int64
		Conflicts                   int64
		TempFiles                   int64
		TempBytes                   int64
		Deadlocks                   int64
		ChecksumFailures            sql.NullInt64
		BlkReadTime                 sql.NullFloat64
		BlkWriteTime                sql.NullFloat64
		ActiveConnections           int
		IdleConnections             int
		IdleInTransactionConnections int
		WaitingConnections          int
	}
	
	ctx, cancel := context.WithTimeout(ctx, r.config.QueryTimeout)
	defer cancel()
	
	err := conn.db.QueryRowContext(ctx, query).Scan(
		&stats.SizeBytes,
		&stats.NumBackends,
		&stats.XactCommit,
		&stats.XactRollback,
		&stats.BlksRead,
		&stats.BlksHit,
		&stats.TupReturned,
		&stats.TupFetched,
		&stats.TupInserted,
		&stats.TupUpdated,
		&stats.TupDeleted,
		&stats.Conflicts,
		&stats.TempFiles,
		&stats.TempBytes,
		&stats.Deadlocks,
		&stats.ChecksumFailures,
		&stats.BlkReadTime,
		&stats.BlkWriteTime,
		&stats.ActiveConnections,
		&stats.IdleConnections,
		&stats.IdleInTransactionConnections,
		&stats.WaitingConnections,
	)
	if err != nil {
		return fmt.Errorf("failed to query database stats: %w", err)
	}
	
	// Create metrics
	scopeMetrics := resource.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("postgresql.database")
	
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	
	// Database size
	sizeMetric := scopeMetrics.Metrics().AppendEmpty()
	sizeMetric.SetName("postgresql.database.size")
	sizeMetric.SetDescription("Database size in bytes")
	sizeMetric.SetUnit("By")
	gauge := sizeMetric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetIntValue(stats.SizeBytes)
	
	// Connection metrics
	r.addConnectionMetrics(scopeMetrics, timestamp, stats.ActiveConnections, 
		stats.IdleConnections, stats.IdleInTransactionConnections, stats.WaitingConnections)
	
	// Transaction metrics
	r.addTransactionMetrics(scopeMetrics, timestamp, stats.XactCommit, stats.XactRollback)
	
	// Buffer cache metrics
	r.addBufferMetrics(scopeMetrics, timestamp, stats.BlksRead, stats.BlksHit)
	
	// Tuple metrics
	r.addTupleMetrics(scopeMetrics, timestamp, stats.TupReturned, stats.TupFetched,
		stats.TupInserted, stats.TupUpdated, stats.TupDeleted)
	
	// Conflict and error metrics
	r.addConflictMetrics(scopeMetrics, timestamp, stats.Conflicts, stats.Deadlocks, 
		stats.ChecksumFailures.Int64)
	
	// Temp file metrics
	r.addTempFileMetrics(scopeMetrics, timestamp, stats.TempFiles, stats.TempBytes)
	
	// I/O timing metrics if available
	if stats.BlkReadTime.Valid && stats.BlkWriteTime.Valid {
		r.addIOTimingMetrics(scopeMetrics, timestamp, stats.BlkReadTime.Float64, 
			stats.BlkWriteTime.Float64)
	}
	
	return nil
}

// collectSlowQueries collects slow query information from pg_stat_statements
func (r *postgresqlQueryReceiver) collectSlowQueries(
	ctx context.Context,
	conn *dbConnection,
	resource pmetric.ResourceMetrics,
	logResource plog.ResourceLogs,
) error {
	// Build query based on available capabilities
	var query string
	if conn.capabilities["pg_stat_kcache"] && conn.capabilities["track_io_timing"] {
		// Full query with all extensions
		query = r.buildFullSlowQuerySQL()
	} else if conn.capabilities["track_io_timing"] {
		// Query with I/O timing but no kcache
		query = r.buildSlowQuerySQLWithIO()
	} else {
		// Basic query
		query = r.buildBasicSlowQuerySQL()
	}
	
	ctx, cancel := context.WithTimeout(ctx, r.config.QueryTimeout)
	defer cancel()
	
	rows, err := conn.db.QueryContext(ctx, query, 
		r.config.SlowQueryThresholdMS,
		r.config.MaxQueriesPerCycle)
	if err != nil {
		return fmt.Errorf("failed to query slow queries: %w", err)
	}
	defer rows.Close()
	
	scopeMetrics := resource.ScopeMetrics().AppendEmpty()
	scopeMetrics.Scope().SetName("postgresql.queries")
	
	scopeLogs := logResource.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("postgresql.queries")
	
	timestamp := pcommon.NewTimestampFromTime(time.Now())
	queriesProcessed := 0
	
	for rows.Next() {
		var q slowQueryRow
		if err := r.scanSlowQueryRow(rows, &q, conn.capabilities); err != nil {
			r.logger.Warn("Failed to scan slow query row", zap.Error(err))
			continue
		}
		
		// Update query fingerprint cache
		fingerprint := r.updateQueryFingerprint(q.QueryID, q.QueryText)
		
		// Create metric for this query
		r.addSlowQueryMetric(scopeMetrics, timestamp, &q, fingerprint)
		
		// Create log record for detailed information
		r.addSlowQueryLog(scopeLogs, timestamp, &q, fingerprint)
		
		// Check for plan regression if enabled
		if r.config.EnablePlanRegression && conn.capabilities["can_explain"] &&
			q.MeanTime > r.config.PlanCollectionThresholdMS {
			go r.checkPlanRegression(ctx, conn, &q)
		}
		
		queriesProcessed++
	}
	
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating slow queries: %w", err)
	}
	
	// Update stats
	r.collectionStats.mu.Lock()
	r.collectionStats.QueriesCollected += int64(queriesProcessed)
	r.collectionStats.mu.Unlock()
	
	return nil
}

// Helper methods for building SQL queries based on capabilities
func (r *postgresqlQueryReceiver) buildBasicSlowQuerySQL() string {
	return `
		SELECT 
			s.queryid::text as query_id,
			s.query,
			s.userid::text,
			s.dbid::text,
			s.calls,
			s.total_exec_time,
			s.mean_exec_time,
			s.stddev_exec_time,
			s.min_exec_time,
			s.max_exec_time,
			s.rows,
			s.shared_blks_hit,
			s.shared_blks_read,
			s.shared_blks_dirtied,
			s.shared_blks_written,
			s.local_blks_hit,
			s.local_blks_read,
			s.temp_blks_read,
			s.temp_blks_written,
			0::float as blk_read_time,
			0::float as blk_write_time,
			0::float as user_time,
			0::float as system_time
		FROM pg_stat_statements s
		WHERE s.mean_exec_time > $1
			AND s.query NOT LIKE '%pg_%'
			AND s.query NOT LIKE '%EXPLAIN%'
		ORDER BY s.total_exec_time DESC
		LIMIT $2
	`
}

func (r *postgresqlQueryReceiver) buildSlowQuerySQLWithIO() string {
	return `
		SELECT 
			s.queryid::text as query_id,
			s.query,
			s.userid::text,
			s.dbid::text,
			s.calls,
			s.total_exec_time,
			s.mean_exec_time,
			s.stddev_exec_time,
			s.min_exec_time,
			s.max_exec_time,
			s.rows,
			s.shared_blks_hit,
			s.shared_blks_read,
			s.shared_blks_dirtied,
			s.shared_blks_written,
			s.local_blks_hit,
			s.local_blks_read,
			s.temp_blks_read,
			s.temp_blks_written,
			s.blk_read_time,
			s.blk_write_time,
			0::float as user_time,
			0::float as system_time
		FROM pg_stat_statements s
		WHERE s.mean_exec_time > $1
			AND s.query NOT LIKE '%pg_%'
			AND s.query NOT LIKE '%EXPLAIN%'
		ORDER BY s.total_exec_time DESC
		LIMIT $2
	`
}

func (r *postgresqlQueryReceiver) buildFullSlowQuerySQL() string {
	return `
		SELECT 
			s.queryid::text as query_id,
			s.query,
			s.userid::text,
			s.dbid::text,
			s.calls,
			s.total_exec_time,
			s.mean_exec_time,
			s.stddev_exec_time,
			s.min_exec_time,
			s.max_exec_time,
			s.rows,
			s.shared_blks_hit,
			s.shared_blks_read,
			s.shared_blks_dirtied,
			s.shared_blks_written,
			s.local_blks_hit,
			s.local_blks_read,
			s.temp_blks_read,
			s.temp_blks_written,
			s.blk_read_time,
			s.blk_write_time,
			COALESCE(k.user_time, 0) as user_time,
			COALESCE(k.system_time, 0) as system_time
		FROM pg_stat_statements s
		LEFT JOIN pg_stat_kcache() k ON s.queryid = k.queryid 
			AND s.userid = k.userid AND s.dbid = k.dbid
		WHERE s.mean_exec_time > $1
			AND s.query NOT LIKE '%pg_%'
			AND s.query NOT LIKE '%EXPLAIN%'
		ORDER BY s.total_exec_time DESC
		LIMIT $2
	`
}

// Additional helper methods...

// statsReporter periodically reports receiver statistics
func (r *postgresqlQueryReceiver) statsReporter(ctx context.Context) {
	defer r.wg.Done()
	
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			r.reportStats()
		case <-r.shutdownChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (r *postgresqlQueryReceiver) reportStats() {
	r.collectionStats.mu.RLock()
	defer r.collectionStats.mu.RUnlock()
	
	r.logger.Info("PostgreSQL receiver statistics",
		zap.Duration("last_collection_duration", r.collectionStats.LastCollectionDuration),
		zap.Int64("queries_collected", r.collectionStats.QueriesCollected),
		zap.Int64("errors_encountered", r.collectionStats.ErrorsEncountered),
		zap.Int64("plan_changes_detected", r.collectionStats.PlanChangesDetected),
		zap.Int("databases_monitored", r.collectionStats.DatabasesMonitored),
	)
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}

func sanitizeDSN(dsn string) string {
	// Remove password from DSN for security
	u, err := pq.ParseURL(dsn)
	if err != nil {
		return "invalid_dsn"
	}
	
	// Parse and remove password
	if idx := strings.Index(u, "password="); idx != -1 {
		end := strings.Index(u[idx:], " ")
		if end == -1 {
			end = len(u) - idx
		}
		u = u[:idx] + "password=***" + u[idx+end:]
	}
	
	return u
}

// slowQueryRow represents a row from pg_stat_statements
type slowQueryRow struct {
	QueryID          string
	QueryText        string
	UserID           string
	DbID             string
	Calls            int64
	TotalTime        float64
	MeanTime         float64
	StddevTime       float64
	MinTime          float64
	MaxTime          float64
	Rows             int64
	SharedBlksHit    int64
	SharedBlksRead   int64
	SharedBlksDirtied int64
	SharedBlksWritten int64
	LocalBlksHit     int64
	LocalBlksRead    int64
	TempBlksRead     int64
	TempBlksWritten  int64
	BlkReadTime      float64
	BlkWriteTime     float64
	UserTime         float64
	SystemTime       float64
}

// scanSlowQueryRow scans a row into slowQueryRow struct
func (r *postgresqlQueryReceiver) scanSlowQueryRow(rows *sql.Rows, q *slowQueryRow, capabilities map[string]bool) error {
	return rows.Scan(
		&q.QueryID,
		&q.QueryText,
		&q.UserID,
		&q.DbID,
		&q.Calls,
		&q.TotalTime,
		&q.MeanTime,
		&q.StddevTime,
		&q.MinTime,
		&q.MaxTime,
		&q.Rows,
		&q.SharedBlksHit,
		&q.SharedBlksRead,
		&q.SharedBlksDirtied,
		&q.SharedBlksWritten,
		&q.LocalBlksHit,
		&q.LocalBlksRead,
		&q.TempBlksRead,
		&q.TempBlksWritten,
		&q.BlkReadTime,
		&q.BlkWriteTime,
		&q.UserTime,
		&q.SystemTime,
	)
}

// updateQueryFingerprint updates the query fingerprint cache
func (r *postgresqlQueryReceiver) updateQueryFingerprint(queryID, queryText string) *QueryFingerprint {
	r.queryFingerprints.mu.Lock()
	defer r.queryFingerprints.mu.Unlock()
	
	fp, exists := r.queryFingerprints.cache[queryID]
	if !exists {
		fp = &QueryFingerprint{
			NormalizedText: normalizeQuery(queryText),
			ParameterCount: countParameters(queryText),
			Tables:         extractTables(queryText),
			FirstSeen:      time.Now(),
		}
		r.queryFingerprints.cache[queryID] = fp
	}
	
	fp.LastSeen = time.Now()
	fp.TotalExecutions++
	
	return fp
}

// normalizeQuery normalizes a query for fingerprinting
func normalizeQuery(query string) string {
	// Simple normalization - in production use pg_query or similar
	normalized := strings.ToLower(query)
	normalized = strings.TrimSpace(normalized)
	
	// Remove extra whitespace
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	
	// Remove comments
	normalized = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(normalized, "")
	normalized = regexp.MustCompile(`--.*$`).ReplaceAllString(normalized, "")
	
	return normalized
}

// countParameters counts the number of parameters in a query
func countParameters(query string) int {
	return strings.Count(query, "$")
}

// extractTables extracts table names from a query (simplified)
func extractTables(query string) []string {
	// This is a simplified implementation
	// In production, use a proper SQL parser
	tables := []string{}
	
	// Look for common patterns
	re := regexp.MustCompile(`(?i)(?:FROM|JOIN)\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	matches := re.FindAllStringSubmatch(query, -1)
	
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			table := strings.ToLower(match[1])
			if !seen[table] {
				tables = append(tables, table)
				seen[table] = true
			}
		}
	}
	
	return tables
}