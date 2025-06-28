package postgresqlquery

import (
	"sync"
	"time"
)

// PlanCache stores historical plan information for regression detection
type PlanCache struct {
	mu    sync.RWMutex
	cache map[string]*PlanInfo
}

// PlanInfo stores information about a query plan
type PlanInfo struct {
	Hash        string
	Cost        float64
	Timestamp   time.Time
	NodeCount   int
	LastSeen    time.Time
	ChangeCount int
}

// QueryFingerprintCache manages query fingerprints and metadata
type QueryFingerprintCache struct {
	mu    sync.RWMutex
	cache map[string]*QueryMetadata
}

// QueryMetadata stores metadata about a query
type QueryMetadata struct {
	Fingerprint     string
	NormalizedQuery string
	FirstSeen       time.Time
	LastSeen        time.Time
	ExecutionCount  int64
	TotalTime       float64
	MeanTime        float64
	MaxTime         float64
	MinTime         float64
}

// NewPlanCache creates a new plan cache
func NewPlanCache() *PlanCache {
	return &PlanCache{
		cache: make(map[string]*PlanInfo),
	}
}

// NewQueryFingerprintCache creates a new query fingerprint cache
func NewQueryFingerprintCache() *QueryFingerprintCache {
	return &QueryFingerprintCache{
		cache: make(map[string]*QueryMetadata),
	}
}

// Get retrieves plan info for a query ID
func (pc *PlanCache) Get(queryID string) (*PlanInfo, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	info, exists := pc.cache[queryID]
	return info, exists
}

// Set stores plan info for a query ID
func (pc *PlanCache) Set(queryID string, info *PlanInfo) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.cache[queryID] = info
}

// Size returns the number of cached plans
func (pc *PlanCache) Size() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.cache)
}

// GetOrCreate retrieves or creates query metadata
func (qfc *QueryFingerprintCache) GetOrCreate(queryID string) *QueryMetadata {
	qfc.mu.Lock()
	defer qfc.mu.Unlock()
	
	metadata, exists := qfc.cache[queryID]
	if !exists {
		metadata = &QueryMetadata{
			FirstSeen: time.Now(),
			LastSeen:  time.Now(),
		}
		qfc.cache[queryID] = metadata
	}
	return metadata
}

// Update updates query execution statistics
func (qfc *QueryFingerprintCache) Update(queryID string, execTime float64) {
	qfc.mu.Lock()
	defer qfc.mu.Unlock()
	
	metadata, exists := qfc.cache[queryID]
	if !exists {
		metadata = &QueryMetadata{
			FirstSeen: time.Now(),
			MinTime:   execTime,
			MaxTime:   execTime,
		}
		qfc.cache[queryID] = metadata
	}
	
	metadata.LastSeen = time.Now()
	metadata.ExecutionCount++
	metadata.TotalTime += execTime
	metadata.MeanTime = metadata.TotalTime / float64(metadata.ExecutionCount)
	
	if execTime < metadata.MinTime {
		metadata.MinTime = execTime
	}
	if execTime > metadata.MaxTime {
		metadata.MaxTime = execTime
	}
}

// Size returns the number of cached query fingerprints
func (qfc *QueryFingerprintCache) Size() int {
	qfc.mu.RLock()
	defer qfc.mu.RUnlock()
	return len(qfc.cache)
}

// Cleanup removes old entries from the cache
func (qfc *QueryFingerprintCache) Cleanup(maxAge time.Duration) int {
	qfc.mu.Lock()
	defer qfc.mu.Unlock()
	
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	
	for queryID, metadata := range qfc.cache {
		if metadata.LastSeen.Before(cutoff) {
			delete(qfc.cache, queryID)
			removed++
		}
	}
	
	return removed
}