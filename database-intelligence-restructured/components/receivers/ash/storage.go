package ash

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// ASHStorage manages the circular buffer and time window aggregation
type ASHStorage struct {
	buffer       *CircularBuffer
	aggregator   *TimeWindowAggregator
	retention    time.Duration
	logger       *zap.Logger
	mu           sync.RWMutex
}

// CircularBuffer implements a fixed-size circular buffer for session snapshots
type CircularBuffer struct {
	snapshots []*SessionSnapshot
	capacity  int
	head      int
	tail      int
	size      int
	mu        sync.RWMutex
}

// TimeWindowAggregator aggregates snapshots into time windows
type TimeWindowAggregator struct {
	windows map[time.Duration]*AggregatedWindow
	mu      sync.RWMutex
}

// AggregatedWindow represents aggregated statistics for a time window
type AggregatedWindow struct {
	Window       time.Duration
	StartTime    time.Time
	EndTime      time.Time
	SessionCount map[string]int      // By state
	WaitEvents   map[string]int      // By event
	QueryStats   map[string]*QuerySummary
	WaitStats    map[string]*WaitSummary
	TopQueries   []QuerySummary
	TopWaits     []WaitSummary
	MaxQueries   int  // Maximum queries to track
	MaxWaits     int  // Maximum wait events to track
}

// QuerySummary represents aggregated query statistics
type QuerySummary struct {
	QueryID        string
	QueryText      string
	ExecutionCount int
	TotalDuration  time.Duration
	AvgDuration    time.Duration
	MaxDuration    time.Duration
}

// WaitSummary represents aggregated wait event statistics
type WaitSummary struct {
	WaitEventType string
	WaitEvent     string
	WaitCount     int
	TotalSessions int
}

// NewASHStorage creates a new ASH storage system
func NewASHStorage(bufferSize int, retention time.Duration, windows []time.Duration, logger *zap.Logger) *ASHStorage {
	aggregator := &TimeWindowAggregator{
		windows: make(map[time.Duration]*AggregatedWindow),
	}
	
	// Initialize aggregation windows with bounded sizes
	for _, window := range windows {
		aggregator.windows[window] = &AggregatedWindow{
			Window:       window,
			SessionCount: make(map[string]int),
			WaitEvents:   make(map[string]int),
			QueryStats:   make(map[string]*QuerySummary),
			WaitStats:    make(map[string]*WaitSummary),
			MaxQueries:   1000,  // Limit queries per window
			MaxWaits:     500,   // Limit wait events per window
		}
	}
	
	return &ASHStorage{
		buffer:     NewCircularBuffer(bufferSize),
		aggregator: aggregator,
		retention:  retention,
		logger:     logger,
	}
}

// AddSnapshot adds a new snapshot to storage
func (s *ASHStorage) AddSnapshot(snapshot *SessionSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Add to circular buffer
	s.buffer.Add(snapshot)
	
	// Update aggregations
	s.updateAggregations(snapshot)
	
	// Clean up old data
	s.cleanupOldData()
}

// GetRecentSnapshots returns snapshots from the last N minutes
func (s *ASHStorage) GetRecentSnapshots(duration time.Duration) []*SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	since := time.Now().Add(-duration)
	return s.buffer.GetSince(since)
}

// GetAggregatedWindow returns aggregated statistics for a time window
func (s *ASHStorage) GetAggregatedWindow(window time.Duration) *AggregatedWindow {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if agg, exists := s.aggregator.windows[window]; exists {
		return agg
	}
	return nil
}

// updateAggregations updates all aggregation windows with new snapshot
func (s *ASHStorage) updateAggregations(snapshot *SessionSnapshot) {
	for window, agg := range s.aggregator.windows {
		// Check if we need to reset the window
		if time.Since(agg.StartTime) > window {
			s.resetAggregatedWindow(agg, window)
		}
		
		// Update aggregations
		s.updateWindowAggregation(agg, snapshot)
	}
}

// resetAggregatedWindow resets an aggregation window
func (s *ASHStorage) resetAggregatedWindow(agg *AggregatedWindow, window time.Duration) {
	agg.StartTime = time.Now()
	agg.EndTime = time.Now().Add(window)
	agg.SessionCount = make(map[string]int)
	agg.WaitEvents = make(map[string]int)
	agg.QueryStats = make(map[string]*QuerySummary)
	agg.WaitStats = make(map[string]*WaitSummary)
	agg.TopQueries = nil
	agg.TopWaits = nil
}

// updateWindowAggregation updates a single aggregation window
func (s *ASHStorage) updateWindowAggregation(agg *AggregatedWindow, snapshot *SessionSnapshot) {
	for _, session := range snapshot.Sessions {
		// Update session state counts
		agg.SessionCount[session.State]++
		
		// Update wait event counts
		if session.WaitEventType != nil && session.WaitEvent != nil {
			waitKey := *session.WaitEventType + ":" + *session.WaitEvent
			agg.WaitEvents[waitKey]++
			
			if ws, exists := agg.WaitStats[waitKey]; exists {
				ws.WaitCount++
				ws.TotalSessions++
			} else {
				agg.WaitStats[waitKey] = &WaitSummary{
					WaitEventType: *session.WaitEventType,
					WaitEvent:     *session.WaitEvent,
					WaitCount:     1,
					TotalSessions: 1,
				}
			}
		}
		
		// Update query statistics
		if session.QueryID != nil && session.QueryStart != nil {
			duration := snapshot.Timestamp.Sub(*session.QueryStart)
			
			if qs, exists := agg.QueryStats[*session.QueryID]; exists {
				qs.ExecutionCount++
				qs.TotalDuration += duration
				qs.AvgDuration = qs.TotalDuration / time.Duration(qs.ExecutionCount)
				if duration > qs.MaxDuration {
					qs.MaxDuration = duration
				}
			} else {
				agg.QueryStats[*session.QueryID] = &QuerySummary{
					QueryID:        *session.QueryID,
					QueryText:      session.Query,
					ExecutionCount: 1,
					TotalDuration:  duration,
					AvgDuration:    duration,
					MaxDuration:    duration,
				}
			}
		}
	}
}

// cleanupOldData removes data older than retention period
func (s *ASHStorage) cleanupOldData() {
	cutoff := time.Now().Add(-s.retention)
	removed := s.buffer.RemoveBefore(cutoff)
	
	if removed > 0 {
		s.logger.Debug("Cleaned up old ASH data",
			zap.Int("removed_snapshots", removed),
			zap.Duration("retention", s.retention))
	}
}

// NewCircularBuffer creates a new circular buffer
func NewCircularBuffer(capacity int) *CircularBuffer {
	return &CircularBuffer{
		snapshots: make([]*SessionSnapshot, capacity),
		capacity:  capacity,
		head:      0,
		tail:      0,
		size:      0,
	}
}

// Add adds a snapshot to the buffer
func (cb *CircularBuffer) Add(snapshot *SessionSnapshot) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.snapshots[cb.head] = snapshot
	cb.head = (cb.head + 1) % cb.capacity
	
	if cb.size < cb.capacity {
		cb.size++
	} else {
		// Buffer is full, move tail forward
		cb.tail = (cb.tail + 1) % cb.capacity
	}
}

// GetSince returns all snapshots since the given time
func (cb *CircularBuffer) GetSince(since time.Time) []*SessionSnapshot {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	var result []*SessionSnapshot
	
	for i := 0; i < cb.size; i++ {
		idx := (cb.tail + i) % cb.capacity
		if cb.snapshots[idx] != nil && cb.snapshots[idx].Timestamp.After(since) {
			result = append(result, cb.snapshots[idx])
		}
	}
	
	return result
}

// RemoveBefore removes snapshots before the given time
func (cb *CircularBuffer) RemoveBefore(before time.Time) int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	removed := 0
	newTail := cb.tail
	
	for i := 0; i < cb.size; i++ {
		idx := (cb.tail + i) % cb.capacity
		if cb.snapshots[idx] != nil && cb.snapshots[idx].Timestamp.Before(before) {
			cb.snapshots[idx] = nil
			removed++
			newTail = (newTail + 1) % cb.capacity
		} else {
			break
		}
	}
	
	cb.tail = newTail
	cb.size -= removed
	
	return removed
}

// GetSize returns the current number of snapshots in the buffer
func (cb *CircularBuffer) GetSize() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size
}