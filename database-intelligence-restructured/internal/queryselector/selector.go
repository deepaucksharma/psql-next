// Package queryselector provides intelligent query selection based on database features
package queryselector

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	
	"github.com/database-intelligence/db-intel/internal/featuredetector"
	"go.uber.org/zap"
)

// QueryCategory represents a category of queries
type QueryCategory string

const (
	CategorySlowQueries   QueryCategory = "slow_queries"
	CategoryActiveSession QueryCategory = "active_sessions"
	CategoryWaitEvents    QueryCategory = "wait_events"
	CategoryTableStats    QueryCategory = "table_stats"
	CategoryConnections   QueryCategory = "connections"
	CategoryReplication   QueryCategory = "replication"
)

// QuerySelector manages query selection based on features
type QuerySelector struct {
	detector     featuredetector.Detector
	queryLibrary map[QueryCategory][]featuredetector.QueryDefinition
	logger       *zap.Logger
	mu           sync.RWMutex
	
	// Cache selected queries
	selectedQueries map[QueryCategory]*featuredetector.QueryDefinition
	cacheTime      time.Time
	cacheDuration  time.Duration
}

// Config for QuerySelector
type Config struct {
	CacheDuration time.Duration `mapstructure:"cache_duration"`
	QueryLibrary  string        `mapstructure:"query_library"` // Path to query definitions
}

// NewQuerySelector creates a new query selector
func NewQuerySelector(detector featuredetector.Detector, logger *zap.Logger, config Config) *QuerySelector {
	if config.CacheDuration == 0 {
		config.CacheDuration = 5 * time.Minute
	}
	
	qs := &QuerySelector{
		detector:        detector,
		queryLibrary:    make(map[QueryCategory][]featuredetector.QueryDefinition),
		logger:          logger,
		selectedQueries: make(map[QueryCategory]*featuredetector.QueryDefinition),
		cacheDuration:   config.CacheDuration,
	}
	
	// Initialize with default queries
	qs.initializeDefaultQueries()
	
	return qs
}

// GetQuery returns the best query for a category
func (qs *QuerySelector) GetQuery(ctx context.Context, category QueryCategory) (*featuredetector.QueryDefinition, error) {
	// Check cache
	qs.mu.RLock()
	if cached, exists := qs.selectedQueries[category]; exists && time.Since(qs.cacheTime) < qs.cacheDuration {
		qs.mu.RUnlock()
		return cached, nil
	}
	qs.mu.RUnlock()
	
	// Get latest features
	features, err := qs.detector.DetectFeatures(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to detect features: %w", err)
	}
	
	// Select best query
	queries, exists := qs.queryLibrary[category]
	if !exists || len(queries) == 0 {
		return nil, fmt.Errorf("no queries defined for category: %s", category)
	}
	
	selected, err := qs.selectBestQuery(features, queries)
	if err != nil {
		return nil, err
	}
	
	// Cache selection
	qs.mu.Lock()
	qs.selectedQueries[category] = selected
	qs.cacheTime = time.Now()
	qs.mu.Unlock()
	
	qs.logger.Info("Selected query for category",
		zap.String("category", string(category)),
		zap.String("query", selected.Name),
		zap.Int("priority", selected.Priority))
	
	return selected, nil
}

// GetAllQueries returns all selected queries for all categories
func (qs *QuerySelector) GetAllQueries(ctx context.Context) (map[QueryCategory]*featuredetector.QueryDefinition, error) {
	result := make(map[QueryCategory]*featuredetector.QueryDefinition)
	
	for category := range qs.queryLibrary {
		query, err := qs.GetQuery(ctx, category)
		if err != nil {
			qs.logger.Warn("Failed to select query for category",
				zap.String("category", string(category)),
				zap.Error(err))
			continue
		}
		result[category] = query
	}
	
	return result, nil
}

// RefreshSelection forces a refresh of query selection
func (qs *QuerySelector) RefreshSelection(ctx context.Context) error {
	qs.mu.Lock()
	qs.selectedQueries = make(map[QueryCategory]*featuredetector.QueryDefinition)
	qs.cacheTime = time.Time{}
	qs.mu.Unlock()
	
	_, err := qs.GetAllQueries(ctx)
	return err
}

// selectBestQuery selects the best query based on features
func (qs *QuerySelector) selectBestQuery(features *featuredetector.FeatureSet, queries []featuredetector.QueryDefinition) (*featuredetector.QueryDefinition, error) {
	// Sort by priority (highest first)
	sorted := make([]featuredetector.QueryDefinition, len(queries))
	copy(sorted, queries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})
	
	// Find first query that meets requirements
	for _, query := range sorted {
		if qs.meetsRequirements(features, &query) {
			return &query, nil
		}
		
		qs.logger.Debug("Query requirements not met",
			zap.String("query", query.Name),
			zap.Strings("required_extensions", query.Requirements.RequiredExtensions),
			zap.Strings("required_capabilities", query.Requirements.RequiredCapabilities))
	}
	
	// If no query meets requirements, use the lowest priority one as fallback
	if len(sorted) > 0 {
		fallback := &sorted[len(sorted)-1]
		qs.logger.Warn("No query meets all requirements, using fallback",
			zap.String("query", fallback.Name))
		return fallback, nil
	}
	
	return nil, fmt.Errorf("no queries available")
}

// meetsRequirements checks if features meet query requirements
func (qs *QuerySelector) meetsRequirements(features *featuredetector.FeatureSet, query *featuredetector.QueryDefinition) bool {
	// Check extensions
	for _, ext := range query.Requirements.RequiredExtensions {
		if !features.HasFeature(ext) {
			return false
		}
	}
	
	// Check capabilities
	for _, cap := range query.Requirements.RequiredCapabilities {
		if !features.HasFeature(cap) {
			return false
		}
	}
	
	// Check version requirements
	if query.Requirements.MinimumVersion != "" && features.ServerVersion != "" {
		if !isVersionSufficient(features.ServerVersion, query.Requirements.MinimumVersion) {
			return false
		}
	}
	
	return true
}

// AddQuery adds a query definition to the library
func (qs *QuerySelector) AddQuery(category QueryCategory, query featuredetector.QueryDefinition) {
	qs.mu.Lock()
	defer qs.mu.Unlock()
	
	qs.queryLibrary[category] = append(qs.queryLibrary[category], query)
}

// initializeDefaultQueries sets up default query library
func (qs *QuerySelector) initializeDefaultQueries() {
	// PostgreSQL slow queries
	qs.queryLibrary[CategorySlowQueries] = []featuredetector.QueryDefinition{
		{
			Name: "pg_stat_monitor_slow_queries",
			SQL: `SELECT 
				queryid::text as query_id,
				query,
				calls,
				total_time,
				mean_time,
				p99 as p99_time,
				rows,
				shared_blks_hit,
				shared_blks_read,
				cpu_user_time,
				cpu_sys_time,
				wal_records,
				wal_bytes
			FROM pg_stat_monitor
			WHERE mean_time > $1
				AND query NOT LIKE '%pg_%'
			ORDER BY mean_time DESC
			LIMIT $2`,
			Requirements: featuredetector.QueryRequirements{
				RequiredExtensions: []string{featuredetector.ExtPgStatMonitor},
			},
			Priority:    100,
			Description: "Advanced slow query stats with CPU and WAL metrics",
		},
		{
			Name: "pg_stat_statements_io_timing",
			SQL: `SELECT 
				queryid::text as query_id,
				query as query_text,
				calls as execution_count,
				total_exec_time as total_time,
				mean_exec_time as mean_time,
				stddev_exec_time as stddev_time,
				rows,
				shared_blks_hit,
				shared_blks_read,
				blk_read_time,
				blk_write_time,
				(total_exec_time - COALESCE(blk_read_time + blk_write_time, 0)) as cpu_time
			FROM pg_stat_statements
			WHERE mean_exec_time > $1
				AND query NOT LIKE '%pg_%'
			ORDER BY mean_exec_time DESC
			LIMIT $2`,
			Requirements: featuredetector.QueryRequirements{
				RequiredExtensions:   []string{featuredetector.ExtPgStatStatements},
				RequiredCapabilities: []string{featuredetector.CapTrackIOTiming},
			},
			Priority:    90,
			Description: "Slow queries with I/O timing breakdown",
		},
		{
			Name: "pg_stat_statements_basic",
			SQL: `SELECT 
				queryid::text as query_id,
				query as query_text,
				calls as execution_count,
				total_exec_time as total_time,
				mean_exec_time as mean_time,
				rows,
				shared_blks_hit,
				shared_blks_read
			FROM pg_stat_statements
			WHERE mean_exec_time > $1
				AND query NOT LIKE '%pg_%'
			ORDER BY mean_exec_time DESC
			LIMIT $2`,
			Requirements: featuredetector.QueryRequirements{
				RequiredExtensions: []string{featuredetector.ExtPgStatStatements},
			},
			Priority:    50,
			Description: "Basic slow query stats",
		},
		{
			Name: "pg_stat_activity_fallback",
			SQL: `SELECT 
				pid::text as query_id,
				query as query_text,
				1 as execution_count,
				EXTRACT(EPOCH FROM (now() - query_start)) * 1000 as total_time,
				EXTRACT(EPOCH FROM (now() - query_start)) * 1000 as mean_time,
				0 as rows,
				0 as shared_blks_hit,
				0 as shared_blks_read
			FROM pg_stat_activity
			WHERE state = 'active' 
				AND query_start < now() - interval '1 second'
				AND query NOT LIKE '%pg_%'
			ORDER BY query_start
			LIMIT $2`,
			Requirements: featuredetector.QueryRequirements{},
			Priority:     10,
			Description:  "Fallback using current activity only",
		},
	}
	
	// PostgreSQL active sessions
	qs.queryLibrary[CategoryActiveSession] = []featuredetector.QueryDefinition{
		{
			Name: "pg_stat_activity_with_waits",
			SQL: `SELECT 
				pid,
				usename,
				application_name,
				client_addr::text,
				backend_start,
				query_start,
				state_change,
				state,
				wait_event_type,
				wait_event,
				query,
				backend_type,
				CASE 
					WHEN wait_event IS NOT NULL THEN 
						EXTRACT(EPOCH FROM (now() - state_change)) 
					ELSE NULL 
				END as wait_duration_seconds
			FROM pg_stat_activity
			WHERE state != 'idle'
				AND backend_type = 'client backend'`,
			Requirements: featuredetector.QueryRequirements{},
			Priority:     50,
			Description:  "Active sessions with wait information",
		},
		{
			Name: "pg_stat_activity_count",
			SQL: `SELECT 
				COUNT(*) as active_sessions,
				state,
				wait_event_type,
				wait_event
			FROM pg_stat_activity
			WHERE state != 'idle'
			GROUP BY state, wait_event_type, wait_event`,
			Requirements: featuredetector.QueryRequirements{},
			Priority:     10,
			Description:  "Basic active session counts",
		},
	}
	
	// MySQL slow queries
	qs.queryLibrary[CategorySlowQueries] = append(qs.queryLibrary[CategorySlowQueries], 
		featuredetector.QueryDefinition{
			Name: "mysql_performance_schema_digest",
			SQL: `SELECT
				DIGEST as query_id,
				DIGEST_TEXT as query_text,
				COUNT_STAR as execution_count,
				SUM_TIMER_WAIT/1000000000 as total_time,
				AVG_TIMER_WAIT/1000000000 as mean_time,
				MAX_TIMER_WAIT/1000000000 as max_time,
				SUM_ROWS_SENT as rows,
				SUM_ROWS_EXAMINED as rows_examined,
				SUM_SELECT as select_count,
				SUM_NO_INDEX_USED as no_index_count,
				SCHEMA_NAME as database_name
			FROM performance_schema.events_statements_summary_by_digest
			WHERE SCHEMA_NAME IS NOT NULL
				AND AVG_TIMER_WAIT > $1 * 1000000000
			ORDER BY AVG_TIMER_WAIT DESC
			LIMIT $2`,
			Requirements: featuredetector.QueryRequirements{
				RequiredCapabilities: []string{
					featuredetector.CapPerfSchemaEnabled,
					featuredetector.CapPerfSchemaStatementsDigest,
				},
			},
			Priority:    100,
			Description: "MySQL slow queries from performance schema",
		},
		featuredetector.QueryDefinition{
			Name: "mysql_processlist_fallback",
			SQL: `SELECT
				ID as query_id,
				INFO as query_text,
				1 as execution_count,
				TIME as total_time,
				TIME as mean_time,
				0 as max_time,
				0 as rows,
				0 as rows_examined,
				0 as select_count,
				0 as no_index_count,
				DB as database_name
			FROM information_schema.PROCESSLIST
			WHERE COMMAND != 'Sleep'
				AND TIME > $1 / 1000
				AND INFO IS NOT NULL
			ORDER BY TIME DESC
			LIMIT $2`,
			Requirements: featuredetector.QueryRequirements{
				RequiredCapabilities: []string{"processlist_access"},
			},
			Priority:    10,
			Description: "MySQL fallback using processlist",
		},
	)
}

// isVersionSufficient checks if current version meets minimum requirement
// Supports simple numeric version comparison (e.g., "10.5", "14.2")
func isVersionSufficient(current, minimum string) bool {
	// Split versions into parts
	currentParts := strings.Split(current, ".")
	minimumParts := strings.Split(minimum, ".")
	
	// Compare each part
	for i := 0; i < len(minimumParts); i++ {
		if i >= len(currentParts) {
			// Current version has fewer parts, assume 0
			return false
		}
		
		// Parse numeric parts (ignore non-numeric suffixes)
		var currentNum, minNum int
		fmt.Sscanf(currentParts[i], "%d", &currentNum)
		fmt.Sscanf(minimumParts[i], "%d", &minNum)
		
		if currentNum < minNum {
			return false
		} else if currentNum > minNum {
			return true
		}
		// If equal, continue to next part
	}
	
	return true
}