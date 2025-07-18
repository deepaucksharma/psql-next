package ash

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// ashReceiver implements the receiver.Metrics interface
type ashReceiver struct {
	config       *Config
	logger       *zap.Logger
	consumer     consumer.Metrics
	db           *sql.DB
	shutdownChan chan struct{}
	wg           sync.WaitGroup
	cancel       context.CancelFunc
	ticker       *time.Ticker
	sampler      *AdaptiveSampler
	storage      *ASHStorage
}

// Start implements the receiver.Metrics interface
func (r *ashReceiver) Start(ctx context.Context, host component.Host) error {
	r.logger.Info("Starting ASH receiver",
		zap.String("driver", r.config.Driver),
		zap.Duration("collection_interval", r.config.CollectionInterval),
		zap.Bool("feature_detection", r.config.EnableFeatureDetection))

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	// Connect to database
	db, err := sql.Open(r.config.Driver, r.config.Datasource)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	
	// Test connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}
	
	r.db = db
	
	// Initialize sampler
	r.sampler = NewAdaptiveSampler(r.config.SamplingConfig, r.logger.Named("sampler"))
	
	// Initialize storage
	r.storage = NewASHStorage(
		r.config.BufferSize,
		r.config.RetentionDuration,
		r.config.AggregationWindows,
		r.logger.Named("storage"),
	)
	
	r.logger.Info("Successfully connected to database")

	// Start collection ticker
	r.ticker = time.NewTicker(r.config.CollectionInterval)
	
	// Start collection goroutine
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.collect(ctx)
	}()

	return nil
}

// Shutdown implements the receiver.Metrics interface
func (r *ashReceiver) Shutdown(ctx context.Context) error {
	r.logger.Info("Shutting down ASH receiver")

	// Cancel the context
	if r.cancel != nil {
		r.cancel()
	}

	// Stop ticker
	if r.ticker != nil {
		r.ticker.Stop()
	}

	// Close database connection
	if r.db != nil {
		if err := r.db.Close(); err != nil {
			r.logger.Error("Failed to close database connection", zap.Error(err))
		}
	}


	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// collect periodically collects ASH data
func (r *ashReceiver) collect(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.ticker.C:
			if err := r.scrapeMetrics(ctx); err != nil {
				r.logger.Error("Failed to scrape ASH metrics", zap.Error(err))
			}
		}
	}
}

// scrapeMetrics scrapes ASH metrics from the database
func (r *ashReceiver) scrapeMetrics(ctx context.Context) error {
	// Collect current activity snapshot
	collector := NewASHCollector(r.db, r.storage, r.sampler, r.config, r.logger)
	snapshot, err := collector.CollectSnapshot(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect ASH data: %w", err)
	}
	
	// Store snapshot for historical analysis
	r.storage.AddSnapshot(snapshot)
	
	// Create metrics from snapshot
	md := r.createMetricsFromSnapshot(snapshot)
	
	// Send metrics to consumer
	return r.consumer.ConsumeMetrics(ctx, md)
}

// createMetrics converts ASH samples to OpenTelemetry metrics
func (r *ashReceiver) createMetricsFromSnapshot(snapshot *SessionSnapshot) pmetric.Metrics {
	md := pmetric.NewMetrics()
	
	// Add resource metrics
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "database-intelligence")
	rm.Resource().Attributes().PutStr("db.system", r.config.Driver)
	if r.config.Database != "" {
		rm.Resource().Attributes().PutStr("db.name", r.config.Database)
	}
	
	// Create scope metrics
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("ash_receiver")
	sm.Scope().SetVersion("1.0.0")
	
	// Generate metrics from snapshot
	r.createActiveSessionMetrics(sm, snapshot)
	r.createWaitEventMetrics(sm, snapshot)
	r.createBlockingSessionMetrics(sm, snapshot)
	r.createLongRunningQueryMetrics(sm, snapshot)
	
	return md
}

// createActiveSessionMetrics creates metrics for active sessions
func (r *ashReceiver) createActiveSessionMetrics(sm pmetric.ScopeMetrics, snapshot *SessionSnapshot) {
	// Count sessions by state
	stateCounts := make(map[string]int)
	for _, session := range snapshot.Sessions {
		stateCounts[session.State]++
	}
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.ash.active_sessions")
	metric.SetDescription("Number of active database sessions by state")
	metric.SetUnit("{session}")
	
	gauge := metric.SetEmptyGauge()
	
	for state, count := range stateCounts {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetIntValue(int64(count))
		dp.Attributes().PutStr("state", state)
	}
}

// createWaitEventMetrics creates metrics for wait events
func (r *ashReceiver) createWaitEventMetrics(sm pmetric.ScopeMetrics, snapshot *SessionSnapshot) {
	// Count wait events
	waitEventCounts := make(map[string]int)
	for _, session := range snapshot.Sessions {
		if session.WaitEvent != nil && *session.WaitEvent != "" {
			eventType := ""
			if session.WaitEventType != nil {
				eventType = *session.WaitEventType
			}
			key := eventType + ":" + *session.WaitEvent
			waitEventCounts[key]++
		}
	}
	
	if len(waitEventCounts) == 0 {
		return
	}
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.ash.wait_events")
	metric.SetDescription("Count of sessions waiting on specific events")
	metric.SetUnit("{event}")
	
	gauge := metric.SetEmptyGauge()
	
	for eventKey, count := range waitEventCounts {
		parts := strings.SplitN(eventKey, ":", 2)
		if len(parts) != 2 {
			continue
		}
		
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		dp.SetIntValue(int64(count))
		dp.Attributes().PutStr("wait_event_type", parts[0])
		dp.Attributes().PutStr("wait_event_name", parts[1])
	}
}

// createBlockingSessionMetrics creates metrics for blocking sessions
func (r *ashReceiver) createBlockingSessionMetrics(sm pmetric.ScopeMetrics, snapshot *SessionSnapshot) {
	blockingCount := 0
	for _, session := range snapshot.Sessions {
		if session.BlockingPID != nil && *session.BlockingPID > 0 {
			blockingCount++
		}
	}
	
	if blockingCount == 0 {
		return
	}
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.ash.blocked_sessions")
	metric.SetDescription("Number of sessions blocked by other sessions")
	metric.SetUnit("{session}")
	
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetIntValue(int64(blockingCount))
}

// createLongRunningQueryMetrics creates metrics for long-running queries
func (r *ashReceiver) createLongRunningQueryMetrics(sm pmetric.ScopeMetrics, snapshot *SessionSnapshot) {
	longRunningCount := 0
	longRunningThreshold := 5 * time.Minute // Configurable
	
	for _, session := range snapshot.Sessions {
		if session.QueryStart != nil {
			queryDuration := snapshot.Timestamp.Sub(*session.QueryStart)
			if queryDuration > longRunningThreshold {
				longRunningCount++
			}
		}
	}
	
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.ash.long_running_queries")
	metric.SetDescription("Number of queries running longer than threshold")
	metric.SetUnit("{query}")
	
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetIntValue(int64(longRunningCount))
	dp.Attributes().PutStr("threshold", longRunningThreshold.String())
}