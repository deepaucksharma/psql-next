package postgresqlquery

import (
	"sync"
	"time"
)

// CollectionStats tracks collection statistics
type CollectionStats struct {
	mu sync.RWMutex

	// Collection metrics
	CollectionCycles   int64
	QueriesCollected   int64
	PlansCollected     int64
	MetricsEmitted     int64
	LogsEmitted        int64
	
	// Error tracking
	CollectionErrors   int64
	ConnectionErrors   int64
	QueryErrors        int64
	
	// Timing statistics
	LastCollectionTime    time.Time
	TotalCollectionTime   time.Duration
	AvgCollectionTime     time.Duration
	MaxCollectionTime     time.Duration
	
	// Per-database stats
	DatabaseStats map[string]*DatabaseStats
}

// DatabaseStats tracks per-database statistics
type DatabaseStats struct {
	// Connection state
	Connected          bool
	LastConnectTime    time.Time
	LastError          error
	LastErrorTime      time.Time
	ConsecutiveErrors  int
	
	// Collection metrics
	QueriesCollected   int64
	PlansCollected     int64
	ExtensionsDetected map[string]bool
	
	// Performance
	AvgQueryTime       time.Duration
	SlowQueryCount     int64
}

// NewCollectionStats creates new collection statistics
func NewCollectionStats() *CollectionStats {
	return &CollectionStats{
		DatabaseStats: make(map[string]*DatabaseStats),
	}
}

// RecordCollection records a collection cycle
func (cs *CollectionStats) RecordCollection(duration time.Duration) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	cs.CollectionCycles++
	cs.LastCollectionTime = time.Now()
	cs.TotalCollectionTime += duration
	cs.AvgCollectionTime = cs.TotalCollectionTime / time.Duration(cs.CollectionCycles)
	
	if duration > cs.MaxCollectionTime {
		cs.MaxCollectionTime = duration
	}
}

// RecordQuery records a collected query
func (cs *CollectionStats) RecordQuery(database string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	cs.QueriesCollected++
	
	if dbStats, exists := cs.DatabaseStats[database]; exists {
		dbStats.QueriesCollected++
	}
}

// RecordPlan records a collected plan
func (cs *CollectionStats) RecordPlan(database string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	cs.PlansCollected++
	
	if dbStats, exists := cs.DatabaseStats[database]; exists {
		dbStats.PlansCollected++
	}
}

// RecordMetric records an emitted metric
func (cs *CollectionStats) RecordMetric() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.MetricsEmitted++
}

// RecordLog records an emitted log
func (cs *CollectionStats) RecordLog() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.LogsEmitted++
}

// RecordError records an error
func (cs *CollectionStats) RecordError(errorType string, database string, err error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	switch errorType {
	case "collection":
		cs.CollectionErrors++
	case "connection":
		cs.ConnectionErrors++
	case "query":
		cs.QueryErrors++
	}
	
	if database != "" {
		dbStats, exists := cs.DatabaseStats[database]
		if !exists {
			dbStats = &DatabaseStats{
				ExtensionsDetected: make(map[string]bool),
			}
			cs.DatabaseStats[database] = dbStats
		}
		
		dbStats.LastError = err
		dbStats.LastErrorTime = time.Now()
		dbStats.ConsecutiveErrors++
		
		// Circuit breaker logic could be implemented here
	}
}

// RecordConnection records a successful connection
func (cs *CollectionStats) RecordConnection(database string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	dbStats, exists := cs.DatabaseStats[database]
	if !exists {
		dbStats = &DatabaseStats{
			ExtensionsDetected: make(map[string]bool),
		}
		cs.DatabaseStats[database] = dbStats
	}
	
	dbStats.Connected = true
	dbStats.LastConnectTime = time.Now()
	dbStats.ConsecutiveErrors = 0
}

// RecordExtension records a detected extension
func (cs *CollectionStats) RecordExtension(database, extension string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	dbStats, exists := cs.DatabaseStats[database]
	if !exists {
		dbStats = &DatabaseStats{
			ExtensionsDetected: make(map[string]bool),
		}
		cs.DatabaseStats[database] = dbStats
	}
	
	dbStats.ExtensionsDetected[extension] = true
}

// GetDatabaseStats returns statistics for a specific database
func (cs *CollectionStats) GetDatabaseStats(database string) *DatabaseStats {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	
	return cs.DatabaseStats[database]
}

// GetSummary returns a summary of collection statistics
func (cs *CollectionStats) GetSummary() map[string]interface{} {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	
	summary := map[string]interface{}{
		"collection_cycles":   cs.CollectionCycles,
		"queries_collected":   cs.QueriesCollected,
		"plans_collected":     cs.PlansCollected,
		"metrics_emitted":     cs.MetricsEmitted,
		"logs_emitted":        cs.LogsEmitted,
		"collection_errors":   cs.CollectionErrors,
		"connection_errors":   cs.ConnectionErrors,
		"query_errors":        cs.QueryErrors,
		"last_collection":     cs.LastCollectionTime,
		"avg_collection_time": cs.AvgCollectionTime,
		"max_collection_time": cs.MaxCollectionTime,
		"databases":           len(cs.DatabaseStats),
	}
	
	// Add per-database summaries
	dbSummaries := make(map[string]interface{})
	for name, stats := range cs.DatabaseStats {
		dbSummaries[name] = map[string]interface{}{
			"connected":          stats.Connected,
			"queries_collected":  stats.QueriesCollected,
			"plans_collected":    stats.PlansCollected,
			"consecutive_errors": stats.ConsecutiveErrors,
			"extensions":         stats.ExtensionsDetected,
		}
	}
	summary["database_details"] = dbSummaries
	
	return summary
}