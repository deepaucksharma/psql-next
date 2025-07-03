package ash

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ASHCollector handles the collection of Active Session History data
type ASHCollector struct {
	db       *sql.DB
	storage  *ASHStorage
	sampler  *AdaptiveSampler
	config   *Config
	logger   *zap.Logger
	
	// Prepared statements
	sessionStmt  *sql.Stmt
	blockingStmt *sql.Stmt
}

// SessionSnapshot represents a point-in-time snapshot of database sessions
type SessionSnapshot struct {
	Timestamp    time.Time
	DatabaseName string
	Sessions     []*Session
	SampleRate   float64
}

// Session represents a database session
type Session struct {
	PID             int
	Username        string
	DatabaseName    string
	ApplicationName string
	ClientAddr      string
	BackendStart    time.Time
	QueryStart      *time.Time
	State           string
	WaitEventType   *string
	WaitEvent       *string
	QueryID         *string
	Query           string
	BlockingPID     *int
	LockType        *string
	BackendType     string
}

// NewASHCollector creates a new ASH collector
func NewASHCollector(db *sql.DB, storage *ASHStorage, sampler *AdaptiveSampler, config *Config, logger *zap.Logger) *ASHCollector {
	return &ASHCollector{
		db:      db,
		storage: storage,
		sampler: sampler,
		config:  config,
		logger:  logger,
	}
}

// CollectSnapshot collects a snapshot of active sessions
func (c *ASHCollector) CollectSnapshot(ctx context.Context) (*SessionSnapshot, error) {
	// Get current sampling rate based on load
	sessionCount, err := c.getActiveSessionCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get session count: %w", err)
	}
	
	sampleRate := c.sampler.CalculateSampleRate(sessionCount)
	
	// Collect sessions
	sessions, err := c.collectSessions(ctx, sampleRate)
	if err != nil {
		return nil, fmt.Errorf("failed to collect sessions: %w", err)
	}
	
	// Get database name
	var dbName string
	err = c.db.QueryRowContext(ctx, "SELECT current_database()").Scan(&dbName)
	if err != nil {
		dbName = "unknown"
	}
	
	snapshot := &SessionSnapshot{
		Timestamp:    time.Now(),
		DatabaseName: dbName,
		Sessions:     sessions,
		SampleRate:   sampleRate,
	}
	
	// Store in circular buffer
	c.storage.AddSnapshot(snapshot)
	
	// Update sampler metrics
	c.sampler.UpdateMetrics(sessionCount, len(sessions))
	
	c.logger.Debug("Collected ASH snapshot",
		zap.Int("total_sessions", sessionCount),
		zap.Int("sampled_sessions", len(sessions)),
		zap.Float64("sample_rate", sampleRate))
	
	return snapshot, nil
}

// getActiveSessionCount returns the count of active sessions
func (c *ASHCollector) getActiveSessionCount(ctx context.Context) (int, error) {
	var count int
	query := `
		SELECT COUNT(*) 
		FROM pg_stat_activity 
		WHERE backend_type = 'client backend' 
		AND pid != pg_backend_pid()
	`
	
	err := c.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

// collectSessions collects session data with sampling
func (c *ASHCollector) collectSessions(ctx context.Context, sampleRate float64) ([]*Session, error) {
	query := `
		WITH active_sessions AS (
			SELECT 
				a.pid,
				a.usename,
				a.datname,
				a.application_name,
				a.client_addr::text as client_addr,
				a.backend_start,
				a.xact_start,
				a.query_start,
				a.state_change,
				a.wait_event_type,
				a.wait_event,
				a.state,
				a.backend_xid,
				a.backend_xmin,
				a.backend_type,
				LEFT(a.query, 4096) as query,
				-- Blocking information from pg_locks
				(SELECT blocking.pid 
				 FROM pg_locks blocked
				 JOIN pg_locks blocking ON blocking.locktype = blocked.locktype
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
				 AND blocking.granted
				 LIMIT 1) as blocking_pid,
				(SELECT blocked.locktype
				 FROM pg_locks blocked
				 WHERE blocked.pid = a.pid
				 AND NOT blocked.granted
				 LIMIT 1) as lock_type
			FROM pg_stat_activity a
			WHERE a.backend_type = 'client backend'
			AND a.pid != pg_backend_pid()
		)
		SELECT * FROM active_sessions
		WHERE 
			-- Always sample critical sessions
			state = 'active'
			OR wait_event IS NOT NULL
			OR blocking_pid IS NOT NULL
			OR (query_start IS NOT NULL AND NOW() - query_start > interval '%d milliseconds')
			-- Random sampling for others
			OR random() <= $1
		ORDER BY 
			CASE 
				WHEN blocking_pid IS NOT NULL THEN 0
				WHEN wait_event IS NOT NULL THEN 1
				WHEN state = 'active' THEN 2
				ELSE 3
			END,
			query_start ASC
	`
	
	query = fmt.Sprintf(query, c.config.SlowQueryThresholdMs)
	
	rows, err := c.db.QueryContext(ctx, query, sampleRate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var sessions []*Session
	for rows.Next() {
		session := &Session{}
		
		err := rows.Scan(
			&session.PID,
			&session.Username,
			&session.DatabaseName,
			&session.ApplicationName,
			&session.ClientAddr,
			&session.BackendStart,
			&sql.NullTime{}, // xact_start (not used)
			&session.QueryStart,
			&sql.NullTime{}, // state_change (not used)
			&session.WaitEventType,
			&session.WaitEvent,
			&session.State,
			&sql.NullInt64{}, // backend_xid (not used)
			&sql.NullInt64{}, // backend_xmin (not used)
			&session.BackendType,
			&session.Query,
			&session.BlockingPID,
			&session.LockType,
		)
		if err != nil {
			c.logger.Warn("Failed to scan session row", zap.Error(err))
			continue
		}
		
		// Try to get query ID from pg_stat_statements if available
		if c.config.EnableFeatureDetection {
			queryID := c.getQueryID(ctx, session.Query)
			if queryID != "" {
				session.QueryID = &queryID
			}
		}
		
		sessions = append(sessions, session)
	}
	
	return sessions, rows.Err()
}

// getQueryID attempts to get query ID from pg_stat_statements
func (c *ASHCollector) getQueryID(ctx context.Context, queryText string) string {
	// This is a simplified version - in production you'd want to cache this check
	var queryID string
	query := `
		SELECT queryid::text 
		FROM pg_stat_statements 
		WHERE query = $1 
		LIMIT 1
	`
	
	err := c.db.QueryRowContext(ctx, query, queryText).Scan(&queryID)
	if err != nil {
		// pg_stat_statements might not be available
		return ""
	}
	
	return queryID
}