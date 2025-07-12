package ash

import (
	"sync"
	"time"
)

// ASHStorage stores historical ASH samples for analysis
type ASHStorage struct {
	mu              sync.RWMutex
	samples         []ASHSample
	maxSize         int
	retentionPeriod time.Duration
	lastCleanup     time.Time
}

// NewASHStorage creates a new ASH storage
func NewASHStorage(maxSize int, retentionPeriod time.Duration) *ASHStorage {
	return &ASHStorage{
		samples:         make([]ASHSample, 0, maxSize),
		maxSize:         maxSize,
		retentionPeriod: retentionPeriod,
		lastCleanup:     time.Now(),
	}
}

// AddSamples adds new samples to storage
func (s *ASHStorage) AddSamples(samples []ASHSample) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add new samples
	s.samples = append(s.samples, samples...)

	// Cleanup if needed
	if time.Since(s.lastCleanup) > time.Minute {
		s.cleanup()
	}

	// Ensure we don't exceed max size
	if len(s.samples) > s.maxSize {
		// Keep the most recent samples
		start := len(s.samples) - s.maxSize
		s.samples = s.samples[start:]
	}
}

// GetSamples returns samples within the specified time range
func (s *ASHStorage) GetSamples(start, end time.Time) []ASHSample {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []ASHSample
	for _, sample := range s.samples {
		if sample.SampleTime.After(start) && sample.SampleTime.Before(end) {
			result = append(result, sample)
		}
	}
	return result
}

// GetRecentSamples returns samples from the last N seconds
func (s *ASHStorage) GetRecentSamples(seconds int) []ASHSample {
	end := time.Now()
	start := end.Add(-time.Duration(seconds) * time.Second)
	return s.GetSamples(start, end)
}

// cleanup removes old samples beyond retention period
func (s *ASHStorage) cleanup() {
	cutoff := time.Now().Add(-s.retentionPeriod)
	
	// Find first sample to keep
	keepIndex := 0
	for i, sample := range s.samples {
		if sample.SampleTime.After(cutoff) {
			keepIndex = i
			break
		}
	}

	// Remove old samples
	if keepIndex > 0 {
		s.samples = s.samples[keepIndex:]
	}
	
	s.lastCleanup = time.Now()
}

// GetStatistics returns storage statistics
func (s *ASHStorage) GetStatistics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["sample_count"] = len(s.samples)
	stats["max_size"] = s.maxSize
	stats["retention_period"] = s.retentionPeriod.String()
	
	if len(s.samples) > 0 {
		stats["oldest_sample"] = s.samples[0].SampleTime
		stats["newest_sample"] = s.samples[len(s.samples)-1].SampleTime
	}

	return stats
}

// AnalyzeTopWaitEvents analyzes top wait events in recent samples
func (s *ASHStorage) AnalyzeTopWaitEvents(minutes int) map[string]int {
	samples := s.GetRecentSamples(minutes * 60)
	
	waitEvents := make(map[string]int)
	for _, sample := range samples {
		if sample.WaitEvent != "" {
			key := sample.WaitEventType + ":" + sample.WaitEvent
			waitEvents[key]++
		}
	}
	
	return waitEvents
}

// AnalyzeTopQueries analyzes top queries by execution time
func (s *ASHStorage) AnalyzeTopQueries(minutes int) []QueryStats {
	samples := s.GetRecentSamples(minutes * 60)
	
	// Group by query
	queryMap := make(map[string]*QueryStats)
	for _, sample := range samples {
		if sample.Query == "" {
			continue
		}
		
		// Normalize query (simple version - in production use fingerprinting)
		normalizedQuery := normalizeQuery(sample.Query)
		
		stats, exists := queryMap[normalizedQuery]
		if !exists {
			stats = &QueryStats{
				Query:        normalizedQuery,
				FirstSeen:    sample.SampleTime,
				LastSeen:     sample.SampleTime,
			}
			queryMap[normalizedQuery] = stats
		}
		
		stats.SampleCount++
		stats.TotalDuration += sample.QueryDuration
		if sample.QueryDuration > stats.MaxDuration {
			stats.MaxDuration = sample.QueryDuration
		}
		if sample.SampleTime.After(stats.LastSeen) {
			stats.LastSeen = sample.SampleTime
		}
	}
	
	// Convert to slice and sort by total duration
	var result []QueryStats
	for _, stats := range queryMap {
		stats.AvgDuration = stats.TotalDuration / time.Duration(stats.SampleCount)
		result = append(result, *stats)
	}
	
	// Sort by total duration (descending)
	// In production, use sort.Slice
	
	return result
}

// QueryStats holds statistics for a query
type QueryStats struct {
	Query         string
	SampleCount   int
	TotalDuration time.Duration
	AvgDuration   time.Duration
	MaxDuration   time.Duration
	FirstSeen     time.Time
	LastSeen      time.Time
}

// normalizeQuery performs basic query normalization
func normalizeQuery(query string) string {
	// This is a very simple implementation
	// In production, use a proper SQL parser/fingerprinter
	
	if len(query) > 100 {
		return query[:100] + "..."
	}
	return query
}