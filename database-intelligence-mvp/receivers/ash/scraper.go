package ash

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	// PostgreSQL driver
	_ "github.com/lib/pq"
)

// ashScraper implements the scraper interface for ASH data collection
type ashScraper struct {
	config       *Config
	logger       *zap.Logger
	db           *sql.DB
	collector    *ASHCollector
	storage      *ASHStorage
	sampler      *AdaptiveSampler
	features     *FeatureDetector
	logsConsumer consumer.Logs
	
	// Metrics
	collectionsTotal   int64
	collectionErrors   int64
	samplesCollected   int64
	sessionsProcessed  int64
}

// newScraper creates a new ASH scraper
func newScraper(config *Config, settings receiver.Settings) (*ashScraper, error) {
	return &ashScraper{
		config: config,
		logger: settings.Logger.Named("ash_scraper"),
		sampler: NewAdaptiveSampler(config.SamplingConfig, settings.Logger.Named("sampler")),
		features: NewFeatureDetector(settings.Logger.Named("features")),
	}, nil
}

// start initializes the database connection and components
func (s *ashScraper) start(ctx context.Context, host component.Host) error {
	// Connect to database
	db, err := sql.Open(s.config.Driver, s.config.DataSource)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	
	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}
	
	s.db = db
	
	// Configure connection pool
	s.db.SetMaxOpenConns(5)
	s.db.SetMaxIdleConns(2)
	s.db.SetConnMaxLifetime(5 * time.Minute)
	
	// Initialize storage
	s.storage = NewASHStorage(
		s.config.BufferSize,
		s.config.RetentionDuration,
		s.config.AggregationWindows,
		s.logger.Named("storage"),
	)
	
	// Initialize collector
	s.collector = NewASHCollector(
		s.db,
		s.storage,
		s.sampler,
		s.config,
		s.logger.Named("collector"),
	)
	
	// Detect database features
	if s.config.EnableFeatureDetection {
		if err := s.features.DetectFeatures(ctx, s.db); err != nil {
			s.logger.Warn("Failed to detect database features", zap.Error(err))
		}
	}
	
	s.logger.Info("ASH scraper started",
		zap.String("driver", s.config.Driver),
		zap.Float64("sampling_rate", s.config.SamplingConfig.BaseRate),
		zap.Int("buffer_size", s.config.BufferSize))
	
	return nil
}

// shutdown closes the database connection and cleans up resources
func (s *ashScraper) shutdown(ctx context.Context) error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// scrape collects ASH data and converts it to metrics
func (s *ashScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.collectionsTotal++
	
	// Collect session snapshot
	snapshot, err := s.collector.CollectSnapshot(ctx)
	if err != nil {
		s.collectionErrors++
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, 0)
	}
	
	s.samplesCollected++
	s.sessionsProcessed += int64(len(snapshot.Sessions))
	
	// Convert to metrics
	metrics := s.convertToMetrics(snapshot)
	
	// Also send as logs if we have a logs consumer
	if s.logsConsumer != nil {
		logs := s.convertToLogs(snapshot)
		if err := s.logsConsumer.ConsumeLogs(ctx, logs); err != nil {
			s.logger.Warn("Failed to send ASH logs", zap.Error(err))
		}
	}
	
	return metrics, nil
}

// convertToMetrics converts ASH snapshot to OpenTelemetry metrics
func (s *ashScraper) convertToMetrics(snapshot *SessionSnapshot) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	
	// Set resource attributes
	resource := rm.Resource()
	resource.Attributes().PutStr("db.system", s.config.Driver)
	resource.Attributes().PutStr("service.name", "ash")
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("ash")
	sm.Scope().SetVersion("1.0.0")
	
	// Active sessions by state
	s.addSessionStateMetrics(sm.Metrics(), snapshot)
	
	// Wait event metrics
	s.addWaitEventMetrics(sm.Metrics(), snapshot)
	
	// Blocking metrics
	s.addBlockingMetrics(sm.Metrics(), snapshot)
	
	// Query performance metrics
	s.addQueryMetrics(sm.Metrics(), snapshot)
	
	// System metrics
	s.addSystemMetrics(sm.Metrics(), snapshot)
	
	return metrics
}

// addSessionStateMetrics adds metrics for session states
func (s *ashScraper) addSessionStateMetrics(metrics pmetric.MetricSlice, snapshot *SessionSnapshot) {
	metric := metrics.AppendEmpty()
	metric.SetName("db.ash.active_sessions")
	metric.SetDescription("Number of active database sessions by state")
	metric.SetUnit("1")
	
	gauge := metric.SetEmptyGauge()
	
	// Count sessions by state
	stateCounts := make(map[string]int)
	for _, session := range snapshot.Sessions {
		stateCounts[session.State]++
	}
	
	// Add data points for each state
	for state, count := range stateCounts {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
		dp.SetIntValue(int64(count))
		dp.Attributes().PutStr("state", state)
		dp.Attributes().PutStr("database_name", snapshot.DatabaseName)
	}
}

// addWaitEventMetrics adds metrics for wait events
func (s *ashScraper) addWaitEventMetrics(metrics pmetric.MetricSlice, snapshot *SessionSnapshot) {
	metric := metrics.AppendEmpty()
	metric.SetName("db.ash.wait_events")
	metric.SetDescription("Count of sessions waiting on specific events")
	metric.SetUnit("1")
	
	gauge := metric.SetEmptyGauge()
	
	// Count wait events
	waitCounts := make(map[string]int)
	for _, session := range snapshot.Sessions {
		if session.WaitEventType != nil && session.WaitEvent != nil {
			key := fmt.Sprintf("%s:%s", *session.WaitEventType, *session.WaitEvent)
			waitCounts[key]++
		}
	}
	
	// Add data points for each wait event
	for waitKey, count := range waitCounts {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
		dp.SetIntValue(int64(count))
		
		// Parse wait event type and name
		var eventType, eventName string
		if n, _ := fmt.Sscanf(waitKey, "%[^:]:%s", &eventType, &eventName); n == 2 {
			dp.Attributes().PutStr("wait_event_type", eventType)
			dp.Attributes().PutStr("wait_event", eventName)
		}
		dp.Attributes().PutStr("database_name", snapshot.DatabaseName)
	}
}

// addBlockingMetrics adds metrics for blocking sessions
func (s *ashScraper) addBlockingMetrics(metrics pmetric.MetricSlice, snapshot *SessionSnapshot) {
	// Blocked sessions count
	blockedMetric := metrics.AppendEmpty()
	blockedMetric.SetName("db.ash.blocked_sessions")
	blockedMetric.SetDescription("Number of sessions that are blocked")
	blockedMetric.SetUnit("1")
	
	blockedGauge := blockedMetric.SetEmptyGauge()
	
	// Blocking chains depth
	chainMetric := metrics.AppendEmpty()
	chainMetric.SetName("db.ash.blocking_chain_depth")
	chainMetric.SetDescription("Maximum depth of blocking chains")
	chainMetric.SetUnit("1")
	
	chainGauge := chainMetric.SetEmptyGauge()
	
	// Count blocked sessions and analyze chains
	blockedCount := 0
	maxChainDepth := 0
	
	for _, session := range snapshot.Sessions {
		if session.BlockingPID != nil && *session.BlockingPID > 0 {
			blockedCount++
			
			// Calculate chain depth
			depth := s.calculateBlockingChainDepth(session, snapshot.Sessions)
			if depth > maxChainDepth {
				maxChainDepth = depth
			}
		}
	}
	
	// Add data points
	dp := blockedGauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
	dp.SetIntValue(int64(blockedCount))
	dp.Attributes().PutStr("database_name", snapshot.DatabaseName)
	
	if maxChainDepth > 0 {
		chainDp := chainGauge.DataPoints().AppendEmpty()
		chainDp.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
		chainDp.SetIntValue(int64(maxChainDepth))
		chainDp.Attributes().PutStr("database_name", snapshot.DatabaseName)
	}
}

// addQueryMetrics adds metrics for query performance
func (s *ashScraper) addQueryMetrics(metrics pmetric.MetricSlice, snapshot *SessionSnapshot) {
	// Long running queries
	longRunningMetric := metrics.AppendEmpty()
	longRunningMetric.SetName("db.ash.long_running_queries")
	longRunningMetric.SetDescription("Number of queries running longer than threshold")
	longRunningMetric.SetUnit("1")
	
	gauge := longRunningMetric.SetEmptyGauge()
	
	longRunningCount := 0
	for _, session := range snapshot.Sessions {
		if session.QueryStart != nil {
			duration := snapshot.Timestamp.Sub(*session.QueryStart)
			if duration.Milliseconds() > s.config.SlowQueryThresholdMs {
				longRunningCount++
			}
		}
	}
	
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
	dp.SetIntValue(int64(longRunningCount))
	dp.Attributes().PutStr("database_name", snapshot.DatabaseName)
	dp.Attributes().PutInt("threshold_ms", s.config.SlowQueryThresholdMs)
}

// addSystemMetrics adds system-level metrics
func (s *ashScraper) addSystemMetrics(metrics pmetric.MetricSlice, snapshot *SessionSnapshot) {
	// Collection statistics
	statsMetric := metrics.AppendEmpty()
	statsMetric.SetName("db.ash.collection_stats")
	statsMetric.SetDescription("ASH collection statistics")
	statsMetric.SetUnit("1")
	
	gauge := statsMetric.SetEmptyGauge()
	
	// Total collections
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
	dp.SetIntValue(s.collectionsTotal)
	dp.Attributes().PutStr("stat_type", "total_collections")
	
	// Collection errors
	dp = gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
	dp.SetIntValue(s.collectionErrors)
	dp.Attributes().PutStr("stat_type", "collection_errors")
	
	// Sessions processed
	dp = gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
	dp.SetIntValue(s.sessionsProcessed)
	dp.Attributes().PutStr("stat_type", "sessions_processed")
	
	// Current sampling rate
	dp = gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
	dp.SetDoubleValue(s.sampler.GetCurrentRate())
	dp.Attributes().PutStr("stat_type", "sampling_rate")
}

// convertToLogs converts ASH snapshot to OpenTelemetry logs
func (s *ashScraper) convertToLogs(snapshot *SessionSnapshot) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	
	// Set resource attributes
	resource := rl.Resource()
	resource.Attributes().PutStr("db.system", s.config.Driver)
	resource.Attributes().PutStr("service.name", "ash")
	
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("ash")
	
	// Create log record for each sampled session
	for _, session := range snapshot.Sessions {
		logRecord := sl.LogRecords().AppendEmpty()
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(snapshot.Timestamp))
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		
		// Set body to query text
		logRecord.Body().SetStr(session.Query)
		
		// Add all session attributes
		attrs := logRecord.Attributes()
		attrs.PutInt("pid", int64(session.PID))
		attrs.PutStr("username", session.Username)
		attrs.PutStr("database_name", session.DatabaseName)
		attrs.PutStr("application_name", session.ApplicationName)
		attrs.PutStr("client_addr", session.ClientAddr)
		attrs.PutStr("state", session.State)
		attrs.PutStr("backend_type", session.BackendType)
		
		if session.QueryID != nil {
			attrs.PutStr("query_id", *session.QueryID)
		}
		
		if session.WaitEventType != nil {
			attrs.PutStr("wait_event_type", *session.WaitEventType)
		}
		
		if session.WaitEvent != nil {
			attrs.PutStr("wait_event", *session.WaitEvent)
		}
		
		if session.BlockingPID != nil && *session.BlockingPID > 0 {
			attrs.PutInt("blocking_pid", int64(*session.BlockingPID))
		}
		
		if session.LockType != nil {
			attrs.PutStr("lock_type", *session.LockType)
		}
		
		// Add timing information
		attrs.PutStr("backend_start", session.BackendStart.Format(time.RFC3339))
		
		if session.QueryStart != nil {
			attrs.PutStr("query_start", session.QueryStart.Format(time.RFC3339))
			queryDuration := snapshot.Timestamp.Sub(*session.QueryStart)
			attrs.PutInt("query_duration_ms", queryDuration.Milliseconds())
		}
		
		// Add performance indicators
		attrs.PutBool("is_slow_query", false)
		if session.QueryStart != nil {
			duration := snapshot.Timestamp.Sub(*session.QueryStart)
			if duration.Milliseconds() > s.config.SlowQueryThresholdMs {
				attrs.PutBool("is_slow_query", true)
			}
		}
		
		attrs.PutBool("is_blocked", session.BlockingPID != nil && *session.BlockingPID > 0)
		attrs.PutBool("is_waiting", session.WaitEvent != nil)
		
		// Set severity based on session state
		if session.BlockingPID != nil && *session.BlockingPID > 0 {
			logRecord.SetSeverityNumber(plog.SeverityNumberWarn)
			logRecord.SetSeverityText("WARN")
		} else if session.State == "active" {
			logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
			logRecord.SetSeverityText("INFO")
		} else {
			logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
			logRecord.SetSeverityText("DEBUG")
		}
	}
	
	return logs
}

// calculateBlockingChainDepth calculates the depth of a blocking chain
func (s *ashScraper) calculateBlockingChainDepth(session *Session, allSessions []*Session) int {
	depth := 0
	currentPID := session.PID
	visited := make(map[int]bool)
	
	for {
		visited[currentPID] = true
		found := false
		
		for _, s := range allSessions {
			if s.BlockingPID != nil && *s.BlockingPID == currentPID {
				if visited[s.PID] {
					// Circular dependency detected
					break
				}
				currentPID = s.PID
				depth++
				found = true
				break
			}
		}
		
		if !found {
			break
		}
	}
	
	return depth
}