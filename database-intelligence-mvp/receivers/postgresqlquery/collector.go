package postgresqlquery

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// collectSlowQueries collects slow queries from pg_stat_statements
func (r *postgresqlQueryReceiver) collectSlowQueries(
	ctx context.Context, 
	conn *dbConnection,
	metrics pmetric.Metrics,
	logs plog.Logs,
) error {
	if !conn.capabilities.HasPgStatStatements {
		return nil
	}
	
	threshold := r.config.SlowQueryThresholdMS
	if conn.config.SlowQueryThresholdMS != nil {
		threshold = *conn.config.SlowQueryThresholdMS
	}
	
	query := `
		SELECT 
			queryid::text,
			query,
			calls,
			total_exec_time,
			mean_exec_time,
			rows,
			100.0 * shared_blks_hit / NULLIF(shared_blks_hit + shared_blks_read, 0) AS hit_percent,
			temp_blks_read + temp_blks_written as temp_blocks,
			blk_read_time + blk_write_time as io_time
		FROM pg_stat_statements
		WHERE mean_exec_time > $1
			AND query NOT LIKE '%pg_stat_statements%'
		ORDER BY mean_exec_time DESC
		LIMIT $2
	`
	
	rows, err := conn.db.QueryContext(ctx, query, threshold, r.config.MaxQueriesPerCycle)
	if err != nil {
		return fmt.Errorf("failed to query pg_stat_statements: %w", err)
	}
	defer rows.Close()
	
	count := 0
	for rows.Next() {
		var (
			queryID      string
			queryText    string
			calls        int64
			totalTime    float64
			meanTime     float64
			rowsReturned int64
			hitPercent   sql.NullFloat64
			tempBlocks   sql.NullInt64
			ioTime       sql.NullFloat64
		)
		
		err := rows.Scan(
			&queryID,
			&queryText,
			&calls,
			&totalTime,
			&meanTime,
			&rowsReturned,
			&hitPercent,
			&tempBlocks,
			&ioTime,
		)
		if err != nil {
			r.logger.Warn("Failed to scan slow query row", zap.Error(err))
			continue
		}
		
		// Check if we should sample this query
		if r.adaptiveSampler != nil {
			slowQueryMetrics := &SlowQueryMetrics{
				QueryID:         queryID,
				MeanTimeMS:      meanTime,
				Calls:           calls,
				TotalTimeMS:     totalTime,
				Rows:            rowsReturned,
				TempBlocksUsed:  tempBlocks.Int64,
				DatabaseID:      conn.config.Name,
			}
			
			shouldSample, rule := r.adaptiveSampler.ShouldSample(ctx, slowQueryMetrics)
			if !shouldSample {
				r.logger.Debug("Query not sampled",
					zap.String("query_id", queryID),
					zap.String("rule", rule))
				continue
			}
		}
		
		// Sanitize query if needed
		if r.config.SanitizePII {
			queryText = r.sanitizeQuery(queryText)
		}
		
		// Update query fingerprint cache
		r.queryFingerprints.Update(queryID, meanTime)
		
		// Add metrics
		r.addSlowQueryMetric(metrics, conn.config.Name, queryID, meanTime, calls, totalTime, rowsReturned)
		
		// Add log entry for slow query
		if meanTime > r.config.PlanCollectionThresholdMS {
			r.addSlowQueryLog(logs, conn.config.Name, queryID, queryText, meanTime, "")
		}
		
		// Check for plan regression if enabled
		if r.config.EnablePlanRegression && meanTime > r.config.PlanCollectionThresholdMS {
			if err := r.checkPlanRegression(ctx, conn, queryID, queryText, meanTime, logs); err != nil {
				r.logger.Warn("Failed to check plan regression",
					zap.String("query_id", queryID),
					zap.Error(err))
			}
		}
		
		count++
		r.collectionStats.RecordQuery(conn.config.Name)
	}
	
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating slow queries: %w", err)
	}
	
	r.logger.Debug("Collected slow queries",
		zap.String("database", conn.config.Name),
		zap.Int("count", count))
	
	return nil
}

// collectWaitEvents collects wait event statistics
func (r *postgresqlQueryReceiver) collectWaitEvents(
	ctx context.Context,
	conn *dbConnection,
	metrics pmetric.Metrics,
) error {
	// If ASH is enabled and running, use ASH data
	if r.config.EnableASH && conn.ashSampler != nil {
		ashStats := conn.ashSampler.GetStats()
		r.addASHMetric(metrics, conn.config.Name, ashStats)
		
		// Analyze recent wait events
		waitAnalysis := conn.ashSampler.AnalyzeWaitEvents(30 * time.Second)
		for _, analysis := range waitAnalysis {
			r.addWaitEventMetric(
				metrics,
				conn.config.Name,
				analysis.EventType,
				analysis.EventName,
				int64(analysis.Count),
			)
		}
		
		return nil
	}
	
	// Otherwise, use pg_stat_activity snapshot
	query := `
		SELECT 
			wait_event_type,
			wait_event,
			COUNT(*) as count
		FROM pg_stat_activity
		WHERE wait_event IS NOT NULL
			AND pid != pg_backend_pid()
		GROUP BY wait_event_type, wait_event
		ORDER BY count DESC
		LIMIT 50
	`
	
	rows, err := conn.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query wait events: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var (
			eventType string
			eventName string
			count     int64
		)
		
		if err := rows.Scan(&eventType, &eventName, &count); err != nil {
			r.logger.Warn("Failed to scan wait event row", zap.Error(err))
			continue
		}
		
		r.addWaitEventMetric(metrics, conn.config.Name, eventType, eventName, count)
	}
	
	return rows.Err()
}

// collectTableMetrics collects table-level metrics
func (r *postgresqlQueryReceiver) collectTableMetrics(
	ctx context.Context,
	conn *dbConnection,
	metrics pmetric.Metrics,
) error {
	if r.config.MinimalMode {
		return nil // Skip table metrics in minimal mode
	}
	
	query := `
		SELECT 
			schemaname,
			tablename,
			seq_scan,
			seq_tup_read,
			idx_scan,
			idx_tup_fetch,
			n_tup_ins,
			n_tup_upd,
			n_tup_del,
			n_live_tup,
			n_dead_tup
		FROM pg_stat_user_tables
		WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY seq_tup_read + idx_tup_fetch DESC
		LIMIT 100
	`
	
	rows, err := conn.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query table statistics: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var (
			schemaName  string
			tableName   string
			seqScan     sql.NullInt64
			seqTupRead  sql.NullInt64
			idxScan     sql.NullInt64
			idxTupFetch sql.NullInt64
			nTupIns     sql.NullInt64
			nTupUpd     sql.NullInt64
			nTupDel     sql.NullInt64
			nLiveTup    sql.NullInt64
			nDeadTup    sql.NullInt64
		)
		
		err := rows.Scan(
			&schemaName,
			&tableName,
			&seqScan,
			&seqTupRead,
			&idxScan,
			&idxTupFetch,
			&nTupIns,
			&nTupUpd,
			&nTupDel,
			&nLiveTup,
			&nDeadTup,
		)
		if err != nil {
			r.logger.Warn("Failed to scan table stats row", zap.Error(err))
			continue
		}
		
		r.addTableMetric(
			metrics,
			conn.config.Name,
			schemaName,
			tableName,
			seqScan.Int64,
			seqTupRead.Int64,
			idxScan.Int64,
			idxTupFetch.Int64,
			nTupIns.Int64,
			nTupUpd.Int64,
			nTupDel.Int64,
			nLiveTup.Int64,
			nDeadTup.Int64,
		)
	}
	
	return rows.Err()
}

// checkPlanRegression checks for plan regression and collects the plan if needed
func (r *postgresqlQueryReceiver) checkPlanRegression(
	ctx context.Context,
	conn *dbConnection,
	queryID string,
	queryText string,
	currentExecTime float64,
	logs plog.Logs,
) error {
	if r.planAnalyzer == nil {
		return nil
	}
	
	plan, regression, err := r.planAnalyzer.AnalyzePlan(
		ctx,
		conn.db,
		queryID,
		queryText,
		currentExecTime,
	)
	if err != nil {
		return err
	}
	
	if plan != nil {
		r.collectionStats.RecordPlan(conn.config.Name)
	}
	
	if regression != nil {
		r.addPlanRegressionLog(logs, conn.config.Name, regression)
	}
	
	return nil
}

// sanitizeQuery removes potentially sensitive information from queries
func (r *postgresqlQueryReceiver) sanitizeQuery(query string) string {
	// This is a simple implementation - in production, use a proper SQL parser
	
	// Remove string literals
	query = regexp.MustCompile(`'[^']*'`).ReplaceAllString(query, "'?'")
	
	// Remove numeric literals that might be IDs
	query = regexp.MustCompile(`\b\d{4,}\b`).ReplaceAllString(query, "?")
	
	// Remove potential email addresses
	query = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`).ReplaceAllString(query, "?@?")
	
	// Normalize whitespace
	query = strings.Join(strings.Fields(query), " ")
	
	return query
}

// collectExtendedMetrics collects metrics from pg_stat_kcache if available
func (r *postgresqlQueryReceiver) collectExtendedMetrics(
	ctx context.Context,
	conn *dbConnection,
	metrics pmetric.Metrics,
) error {
	if !r.config.EnableExtendedMetrics || !conn.capabilities.HasPgStatKcache {
		return nil
	}
	
	query := `
		SELECT 
			queryid::text,
			plan_reads,
			plan_writes,
			plan_user_time,
			plan_system_time,
			plan_minflts,
			plan_majflts,
			plan_nvcsws,
			plan_nivcsws
		FROM pg_stat_kcache k
		JOIN pg_stat_statements s USING (queryid, userid, dbid)
		WHERE s.mean_exec_time > $1
		ORDER BY plan_user_time + plan_system_time DESC
		LIMIT 50
	`
	
	rows, err := conn.db.QueryContext(ctx, query, r.config.SlowQueryThresholdMS)
	if err != nil {
		// pg_stat_kcache might not be available, log and continue
		r.logger.Debug("Failed to query pg_stat_kcache", zap.Error(err))
		return nil
	}
	defer rows.Close()
	
	// Process extended metrics...
	// This would add CPU and I/O metrics per query
	
	return rows.Err()
}