package ash

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ASHSample represents a single Active Session History sample
type ASHSample struct {
	SampleTime      time.Time
	SessionID       int64
	PID             int32
	State           string
	WaitEvent       string
	WaitEventType   string
	Query           string
	QueryStart      time.Time
	QueryDuration   time.Duration
	BlockingPID     int32
	DatabaseName    string
	UserName        string
	ApplicationName string
	ClientAddr      string
}

// ASHSampler handles sampling of active sessions
type ASHSampler struct {
	logger *zap.Logger
	config *Config
}

// NewASHSampler creates a new ASH sampler
func NewASHSampler(logger *zap.Logger, config *Config) *ASHSampler {
	return &ASHSampler{
		logger: logger,
		config: config,
	}
}

// Sample collects current active session samples
func (s *ASHSampler) Sample(ctx context.Context, db *sql.DB) ([]ASHSample, error) {
	switch s.config.Driver {
	case "postgres", "postgresql":
		return s.samplePostgreSQL(ctx, db)
	case "mysql":
		return s.sampleMySQL(ctx, db)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", s.config.Driver)
	}
}

// samplePostgreSQL samples active sessions from PostgreSQL
func (s *ASHSampler) samplePostgreSQL(ctx context.Context, db *sql.DB) ([]ASHSample, error) {
	query := `
		SELECT 
			NOW() as sample_time,
			sa.pid,
			sa.state,
			sa.wait_event,
			sa.wait_event_type,
			sa.query,
			sa.query_start,
			CASE 
				WHEN sa.query_start IS NOT NULL 
				THEN EXTRACT(EPOCH FROM (NOW() - sa.query_start))::INT 
				ELSE 0 
			END as query_duration_seconds,
			sa.datname,
			sa.usename,
			sa.application_name,
			sa.client_addr::text,
			-- Get blocking PID if exists
			COALESCE(
				(SELECT blocking.pid 
				 FROM pg_stat_activity blocking 
				 WHERE blocking.pid = ANY(pg_blocking_pids(sa.pid))
				 LIMIT 1), 
				0
			) as blocking_pid
		FROM pg_stat_activity sa
		WHERE sa.pid != pg_backend_pid()
			AND sa.datname IS NOT NULL
			AND sa.datname NOT IN ('template0', 'template1')
	`

	// Add database filter if specified
	if s.config.Database != "" {
		query += fmt.Sprintf(" AND sa.datname = '%s'", s.config.Database)
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active sessions: %w", err)
	}
	defer rows.Close()

	var samples []ASHSample
	for rows.Next() {
		var sample ASHSample
		var queryStart sql.NullTime
		var waitEvent, waitEventType, clientAddr sql.NullString
		var queryDurationSeconds int

		err := rows.Scan(
			&sample.SampleTime,
			&sample.PID,
			&sample.State,
			&waitEvent,
			&waitEventType,
			&sample.Query,
			&queryStart,
			&queryDurationSeconds,
			&sample.DatabaseName,
			&sample.UserName,
			&sample.ApplicationName,
			&clientAddr,
			&sample.BlockingPID,
		)
		if err != nil {
			s.logger.Warn("Failed to scan row", zap.Error(err))
			continue
		}

		// Convert nullable fields
		if waitEvent.Valid {
			sample.WaitEvent = waitEvent.String
		}
		if waitEventType.Valid {
			sample.WaitEventType = waitEventType.String
		}
		if queryStart.Valid {
			sample.QueryStart = queryStart.Time
			sample.QueryDuration = time.Duration(queryDurationSeconds) * time.Second
		}
		if clientAddr.Valid {
			sample.ClientAddr = clientAddr.String
		}

		// Apply sampling logic if needed
		if s.shouldSample(sample) {
			samples = append(samples, sample)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	s.logger.Debug("Collected ASH samples",
		zap.Int("total_sessions", len(samples)),
		zap.String("database", s.config.Database))

	return samples, nil
}

// sampleMySQL samples active sessions from MySQL
func (s *ASHSampler) sampleMySQL(ctx context.Context, db *sql.DB) ([]ASHSample, error) {
	query := `
		SELECT 
			NOW() as sample_time,
			p.ID as session_id,
			p.STATE,
			p.INFO as query,
			p.TIME as query_duration_seconds,
			p.DB as database_name,
			p.USER as user_name,
			CONCAT(p.HOST) as client_addr,
			-- Check for blocking
			CASE 
				WHEN t.TRX_STATE = 'LOCK WAIT' THEN 
					(SELECT BLOCKING_THREAD_ID 
					 FROM performance_schema.data_lock_waits 
					 WHERE REQUESTING_THREAD_ID = p.ID 
					 LIMIT 1)
				ELSE 0
			END as blocking_id
		FROM performance_schema.threads p
		LEFT JOIN information_schema.INNODB_TRX t ON t.TRX_MYSQL_THREAD_ID = p.PROCESSLIST_ID
		WHERE p.TYPE = 'FOREGROUND'
			AND p.PROCESSLIST_ID IS NOT NULL
	`

	// Add database filter if specified
	if s.config.Database != "" {
		query += fmt.Sprintf(" AND p.DB = '%s'", s.config.Database)
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		// Fallback to simpler query if performance_schema is not available
		return s.sampleMySQLFallback(ctx, db)
	}
	defer rows.Close()

	var samples []ASHSample
	for rows.Next() {
		var sample ASHSample
		var query, dbName, userName, clientAddr sql.NullString
		var queryDurationSeconds sql.NullInt64
		var blockingID sql.NullInt64

		err := rows.Scan(
			&sample.SampleTime,
			&sample.SessionID,
			&sample.State,
			&query,
			&queryDurationSeconds,
			&dbName,
			&userName,
			&clientAddr,
			&blockingID,
		)
		if err != nil {
			s.logger.Warn("Failed to scan row", zap.Error(err))
			continue
		}

		// Convert nullable fields
		if query.Valid {
			sample.Query = query.String
		}
		if queryDurationSeconds.Valid {
			sample.QueryDuration = time.Duration(queryDurationSeconds.Int64) * time.Second
			sample.QueryStart = sample.SampleTime.Add(-sample.QueryDuration)
		}
		if dbName.Valid {
			sample.DatabaseName = dbName.String
		}
		if userName.Valid {
			sample.UserName = userName.String
		}
		if clientAddr.Valid {
			sample.ClientAddr = clientAddr.String
		}
		if blockingID.Valid && blockingID.Int64 > 0 {
			sample.BlockingPID = int32(blockingID.Int64)
		}

		// MySQL doesn't have wait events in the same way as PostgreSQL
		// We can infer some from the state
		if sample.State == "Waiting for table metadata lock" {
			sample.WaitEvent = "metadata lock"
			sample.WaitEventType = "Lock"
		} else if sample.State == "Waiting for table level lock" {
			sample.WaitEvent = "table lock"
			sample.WaitEventType = "Lock"
		}

		if s.shouldSample(sample) {
			samples = append(samples, sample)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return samples, nil
}

// sampleMySQLFallback uses SHOW PROCESSLIST for older MySQL versions
func (s *ASHSampler) sampleMySQLFallback(ctx context.Context, db *sql.DB) ([]ASHSample, error) {
	rows, err := db.QueryContext(ctx, "SHOW FULL PROCESSLIST")
	if err != nil {
		return nil, fmt.Errorf("failed to query processlist: %w", err)
	}
	defer rows.Close()

	var samples []ASHSample
	for rows.Next() {
		var sample ASHSample
		var id int64
		var user, host, dbName, command, state, info sql.NullString
		var timeSeconds sql.NullInt64

		err := rows.Scan(&id, &user, &host, &dbName, &command, &timeSeconds, &state, &info)
		if err != nil {
			s.logger.Warn("Failed to scan processlist row", zap.Error(err))
			continue
		}

		sample.SampleTime = time.Now()
		sample.SessionID = id
		
		if user.Valid {
			sample.UserName = user.String
		}
		if host.Valid {
			sample.ClientAddr = host.String
		}
		if dbName.Valid {
			sample.DatabaseName = dbName.String
		}
		if state.Valid {
			sample.State = state.String
		} else if command.Valid {
			sample.State = command.String
		}
		if info.Valid {
			sample.Query = info.String
		}
		if timeSeconds.Valid {
			sample.QueryDuration = time.Duration(timeSeconds.Int64) * time.Second
			sample.QueryStart = sample.SampleTime.Add(-sample.QueryDuration)
		}

		if s.shouldSample(sample) {
			samples = append(samples, sample)
		}
	}

	return samples, nil
}

// shouldSample determines if a session should be included in the sample
func (s *ASHSampler) shouldSample(sample ASHSample) bool {
	// Always include certain types of sessions
	if s.config.IncludeIdleSessions && sample.State == "idle" {
		return true
	}

	// Always include blocked sessions
	if sample.BlockingPID > 0 {
		return true
	}

	// Always include long-running queries
	if sample.QueryDuration > s.config.LongRunningThreshold {
		return true
	}

	// Always include waiting sessions
	if sample.WaitEvent != "" {
		return true
	}

	// Include active sessions
	if sample.State == "active" || sample.State == "Query" {
		return true
	}

	// Apply sampling rate for other sessions
	// This is simplified - in production you'd use the AdaptiveSampler
	return s.config.SamplingRate >= 1.0 || (time.Now().UnixNano()%100) < int64(s.config.SamplingRate*100)
}