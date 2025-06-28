package postgresqlquery

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ActiveSessionHistorySampler implements high-frequency sampling of active sessions
type ActiveSessionHistorySampler struct {
	logger       *zap.Logger
	db           *sql.DB
	dbName       string
	interval     time.Duration
	
	// Ring buffer for samples
	samples      *RingBuffer
	
	// Control
	wg           sync.WaitGroup
	stopChan     chan struct{}
	
	// Statistics
	stats        ASHStats
	statsMu      sync.RWMutex
}

// ASHSample represents a single Active Session History sample
type ASHSample struct {
	Timestamp       time.Time
	PID             int32
	SessionID       string
	User            string
	Database        string
	ApplicationName string
	ClientAddr      string
	State           string
	StateChangeTime time.Time
	WaitEventType   sql.NullString
	WaitEvent       sql.NullString
	Query           sql.NullString
	QueryStart      sql.NullTime
	BackendType     string
	BackendStart    time.Time
	
	// Extended information
	LockType        sql.NullString
	LockMode        sql.NullString
	BlockedBy       sql.NullInt32
	
	// Derived fields
	QueryDuration   time.Duration
	StateDuration   time.Duration
	IsBlocked       bool
	IsActive        bool
}

// ASHStats tracks ASH sampling statistics
type ASHStats struct {
	SamplesCollected   int64
	ErrorCount         int64
	LastSampleTime     time.Time
	AvgSampleDuration  time.Duration
	ActiveSessionsAvg  float64
	WaitingSessionsAvg float64
}

// RingBuffer is a fixed-size circular buffer for ASH samples
type RingBuffer struct {
	mu       sync.RWMutex
	samples  []ASHSample
	capacity int
	head     int
	tail     int
	size     int
}

// NewActiveSessionHistorySampler creates a new ASH sampler
func NewActiveSessionHistorySampler(
	logger *zap.Logger,
	db *sql.DB,
	dbName string,
	interval time.Duration,
	bufferSize int,
) *ActiveSessionHistorySampler {
	return &ActiveSessionHistorySampler{
		logger:   logger,
		db:       db,
		dbName:   dbName,
		interval: interval,
		samples:  NewRingBuffer(bufferSize),
		stopChan: make(chan struct{}),
	}
}

// Start begins the ASH sampling loop
func (ash *ActiveSessionHistorySampler) Start() error {
	ash.logger.Info("Starting Active Session History sampler",
		zap.String("database", ash.dbName),
		zap.Duration("interval", ash.interval))
	
	ash.wg.Add(1)
	go ash.samplingLoop()
	
	return nil
}

// Stop halts the ASH sampling
func (ash *ActiveSessionHistorySampler) Stop() error {
	ash.logger.Info("Stopping Active Session History sampler",
		zap.String("database", ash.dbName))
	
	close(ash.stopChan)
	ash.wg.Wait()
	
	return nil
}

// samplingLoop runs the main sampling cycle
func (ash *ActiveSessionHistorySampler) samplingLoop() {
	defer ash.wg.Done()
	
	ticker := time.NewTicker(ash.interval)
	defer ticker.Stop()
	
	// Run initial sample immediately
	ash.collectSample()
	
	for {
		select {
		case <-ticker.C:
			ash.collectSample()
		case <-ash.stopChan:
			return
		}
	}
}

// collectSample collects one ASH sample
func (ash *ActiveSessionHistorySampler) collectSample() {
	start := time.Now()
	
	// Query for active sessions with extended information
	query := `
		SELECT 
			pid,
			pid::text || '-' || backend_start::text as session_id,
			usename,
			datname,
			application_name,
			client_addr::text,
			state,
			state_change,
			wait_event_type,
			wait_event,
			query,
			query_start,
			backend_type,
			backend_start,
			-- Lock information
			l.locktype,
			l.mode,
			l.pid as blocked_by
		FROM pg_stat_activity a
		LEFT JOIN LATERAL (
			SELECT 
				locktype,
				mode,
				blocking.pid
			FROM pg_locks blocked
			JOIN pg_locks blocking ON 
				blocking.locktype = blocked.locktype
				AND blocking.database IS NOT DISTINCT FROM blocked.database
				AND blocking.relation IS NOT DISTINCT FROM blocked.relation
				AND blocking.page IS NOT DISTINCT FROM blocked.page
				AND blocking.tuple IS NOT DISTINCT FROM blocked.tuple
				AND blocking.virtualxid IS NOT DISTINCT FROM blocked.virtualxid
				AND blocking.transactionid IS NOT DISTINCT FROM blocked.transactionid
				AND blocking.classid IS NOT DISTINCT FROM blocked.classid
				AND blocking.objid IS NOT DISTINCT FROM blocked.objid
				AND blocking.objsubid IS NOT DISTINCT FROM blocked.objsubid
				AND blocking.pid != blocked.pid
			WHERE blocked.pid = a.pid
				AND NOT blocked.granted
			LIMIT 1
		) l ON true
		WHERE a.pid != pg_backend_pid()
			AND a.datname = current_database()
	`
	
	ctx, cancel := context.WithTimeout(context.Background(), ash.interval/2)
	defer cancel()
	
	rows, err := ash.db.QueryContext(ctx, query)
	if err != nil {
		ash.statsMu.Lock()
		ash.stats.ErrorCount++
		ash.statsMu.Unlock()
		
		ash.logger.Error("Failed to collect ASH sample",
			zap.String("database", ash.dbName),
			zap.Error(err))
		return
	}
	defer rows.Close()
	
	timestamp := time.Now()
	activeSessions := 0
	waitingSessions := 0
	
	for rows.Next() {
		var sample ASHSample
		sample.Timestamp = timestamp
		
		err := rows.Scan(
			&sample.PID,
			&sample.SessionID,
			&sample.User,
			&sample.Database,
			&sample.ApplicationName,
			&sample.ClientAddr,
			&sample.State,
			&sample.StateChangeTime,
			&sample.WaitEventType,
			&sample.WaitEvent,
			&sample.Query,
			&sample.QueryStart,
			&sample.BackendType,
			&sample.BackendStart,
			&sample.LockType,
			&sample.LockMode,
			&sample.BlockedBy,
		)
		if err != nil {
			ash.logger.Warn("Failed to scan ASH row", zap.Error(err))
			continue
		}
		
		// Calculate derived fields
		if sample.QueryStart.Valid {
			sample.QueryDuration = timestamp.Sub(sample.QueryStart.Time)
		}
		sample.StateDuration = timestamp.Sub(sample.StateChangeTime)
		sample.IsBlocked = sample.BlockedBy.Valid
		sample.IsActive = sample.State == "active"
		
		// Update counters
		if sample.IsActive {
			activeSessions++
		}
		if sample.WaitEvent.Valid {
			waitingSessions++
		}
		
		// Add to ring buffer
		ash.samples.Add(sample)
	}
	
	if err := rows.Err(); err != nil {
		ash.statsMu.Lock()
		ash.stats.ErrorCount++
		ash.statsMu.Unlock()
		
		ash.logger.Error("Error iterating ASH rows",
			zap.String("database", ash.dbName),
			zap.Error(err))
		return
	}
	
	// Update statistics
	ash.statsMu.Lock()
	ash.stats.SamplesCollected++
	ash.stats.LastSampleTime = timestamp
	
	// Update moving averages
	alpha := 0.1 // Smoothing factor
	ash.stats.ActiveSessionsAvg = alpha*float64(activeSessions) + (1-alpha)*ash.stats.ActiveSessionsAvg
	ash.stats.WaitingSessionsAvg = alpha*float64(waitingSessions) + (1-alpha)*ash.stats.WaitingSessionsAvg
	
	// Update average sample duration
	sampleDuration := time.Since(start)
	if ash.stats.AvgSampleDuration == 0 {
		ash.stats.AvgSampleDuration = sampleDuration
	} else {
		ash.stats.AvgSampleDuration = time.Duration(
			alpha*float64(sampleDuration) + (1-alpha)*float64(ash.stats.AvgSampleDuration))
	}
	ash.statsMu.Unlock()
	
	ash.logger.Debug("Collected ASH sample",
		zap.String("database", ash.dbName),
		zap.Int("active_sessions", activeSessions),
		zap.Int("waiting_sessions", waitingSessions),
		zap.Duration("duration", sampleDuration))
}

// GetRecentSamples returns samples from the last N seconds
func (ash *ActiveSessionHistorySampler) GetRecentSamples(duration time.Duration) []ASHSample {
	cutoff := time.Now().Add(-duration)
	return ash.samples.GetSince(cutoff)
}

// GetStats returns current ASH statistics
func (ash *ActiveSessionHistorySampler) GetStats() ASHStats {
	ash.statsMu.RLock()
	defer ash.statsMu.RUnlock()
	return ash.stats
}

// AnalyzeWaitEvents analyzes wait events over a time period
func (ash *ActiveSessionHistorySampler) AnalyzeWaitEvents(duration time.Duration) map[string]WaitEventAnalysis {
	samples := ash.GetRecentSamples(duration)
	analysis := make(map[string]WaitEventAnalysis)
	
	for _, sample := range samples {
		if !sample.WaitEvent.Valid {
			continue
		}
		
		key := fmt.Sprintf("%s:%s", sample.WaitEventType.String, sample.WaitEvent.String)
		wa, exists := analysis[key]
		if !exists {
			wa = WaitEventAnalysis{
				EventType: sample.WaitEventType.String,
				EventName: sample.WaitEvent.String,
			}
		}
		
		wa.Count++
		wa.TotalTime += ash.interval // Approximate wait time
		
		// Track unique sessions
		if wa.Sessions == nil {
			wa.Sessions = make(map[string]bool)
		}
		wa.Sessions[sample.SessionID] = true
		
		// Track queries affected
		if sample.Query.Valid && wa.Queries == nil {
			wa.Queries = make(map[string]bool)
		}
		if sample.Query.Valid {
			wa.Queries[sample.Query.String] = true
		}
		
		analysis[key] = wa
	}
	
	return analysis
}

// AnalyzeBlockingSessions analyzes blocking patterns
func (ash *ActiveSessionHistorySampler) AnalyzeBlockingSessions(duration time.Duration) []BlockingAnalysis {
	samples := ash.GetRecentSamples(duration)
	
	// Group by blocking PID
	blockingMap := make(map[int32]*BlockingAnalysis)
	
	for _, sample := range samples {
		if !sample.IsBlocked || !sample.BlockedBy.Valid {
			continue
		}
		
		blockerPID := sample.BlockedBy.Int32
		analysis, exists := blockingMap[blockerPID]
		if !exists {
			analysis = &BlockingAnalysis{
				BlockerPID: blockerPID,
				BlockedPIDs: make(map[int32]bool),
			}
			blockingMap[blockerPID] = analysis
		}
		
		analysis.BlockedPIDs[sample.PID] = true
		analysis.TotalBlockedTime += ash.interval
		
		// Find blocker details from samples
		for _, s := range samples {
			if s.PID == blockerPID {
				analysis.BlockerQuery = s.Query.String
				analysis.BlockerUser = s.User
				analysis.BlockerState = s.State
				break
			}
		}
	}
	
	// Convert to slice
	var results []BlockingAnalysis
	for _, analysis := range blockingMap {
		analysis.BlockedCount = len(analysis.BlockedPIDs)
		results = append(results, *analysis)
	}
	
	return results
}

// WaitEventAnalysis represents analysis of a specific wait event
type WaitEventAnalysis struct {
	EventType   string
	EventName   string
	Count       int
	TotalTime   time.Duration
	Sessions    map[string]bool
	Queries     map[string]bool
}

// BlockingAnalysis represents analysis of blocking sessions
type BlockingAnalysis struct {
	BlockerPID       int32
	BlockerQuery     string
	BlockerUser      string
	BlockerState     string
	BlockedPIDs      map[int32]bool
	BlockedCount     int
	TotalBlockedTime time.Duration
}

// RingBuffer implementation

// NewRingBuffer creates a new ring buffer
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		samples:  make([]ASHSample, capacity),
		capacity: capacity,
	}
}

// Add adds a sample to the ring buffer
func (rb *RingBuffer) Add(sample ASHSample) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	
	rb.samples[rb.head] = sample
	rb.head = (rb.head + 1) % rb.capacity
	
	if rb.size < rb.capacity {
		rb.size++
	} else {
		rb.tail = (rb.tail + 1) % rb.capacity
	}
}

// GetSince returns all samples since the given time
func (rb *RingBuffer) GetSince(since time.Time) []ASHSample {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	
	var result []ASHSample
	
	if rb.size == 0 {
		return result
	}
	
	// Iterate through the buffer
	for i := 0; i < rb.size; i++ {
		idx := (rb.tail + i) % rb.capacity
		if rb.samples[idx].Timestamp.After(since) {
			result = append(result, rb.samples[idx])
		}
	}
	
	return result
}

// GetAll returns all samples in the buffer
func (rb *RingBuffer) GetAll() []ASHSample {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	
	result := make([]ASHSample, rb.size)
	
	for i := 0; i < rb.size; i++ {
		idx := (rb.tail + i) % rb.capacity
		result[i] = rb.samples[idx]
	}
	
	return result
}

// Size returns the current number of samples
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}