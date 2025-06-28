package postgresqlquery

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/semconv/v1.6.1"
)

// addSlowQueryMetric adds a slow query metric
func (r *postgresqlQueryReceiver) addSlowQueryMetric(
	metrics pmetric.Metrics,
	dbName string,
	queryID string,
	meanTime float64,
	calls int64,
	totalTime float64,
	rows int64,
) {
	rm := metrics.ResourceMetrics().AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr(semconv.AttributeServiceName, "postgresql")
	resource.Attributes().PutStr(semconv.AttributeDBName, dbName)
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/postgresqlquery")
	
	// Mean time metric
	meanTimeMetric := sm.Metrics().AppendEmpty()
	meanTimeMetric.SetName("postgresql.query.mean_time")
	meanTimeMetric.SetDescription("Average execution time of the query")
	meanTimeMetric.SetUnit("ms")
	
	meanTimeGauge := meanTimeMetric.SetEmptyGauge()
	dp := meanTimeGauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(meanTime)
	dp.Attributes().PutStr("query_id", queryID)
	
	// Calls metric
	callsMetric := sm.Metrics().AppendEmpty()
	callsMetric.SetName("postgresql.query.calls")
	callsMetric.SetDescription("Number of times the query was executed")
	callsMetric.SetUnit("{calls}")
	
	callsSum := callsMetric.SetEmptySum()
	callsSum.SetIsMonotonic(true)
	callsSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	cdp := callsSum.DataPoints().AppendEmpty()
	cdp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	cdp.SetIntValue(calls)
	cdp.Attributes().PutStr("query_id", queryID)
	
	// Total time metric
	totalTimeMetric := sm.Metrics().AppendEmpty()
	totalTimeMetric.SetName("postgresql.query.total_time")
	totalTimeMetric.SetDescription("Total execution time of the query")
	totalTimeMetric.SetUnit("ms")
	
	totalTimeSum := totalTimeMetric.SetEmptySum()
	totalTimeSum.SetIsMonotonic(true)
	totalTimeSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	tdp := totalTimeSum.DataPoints().AppendEmpty()
	tdp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	tdp.SetDoubleValue(totalTime)
	tdp.Attributes().PutStr("query_id", queryID)
	
	// Rows metric
	rowsMetric := sm.Metrics().AppendEmpty()
	rowsMetric.SetName("postgresql.query.rows")
	rowsMetric.SetDescription("Total rows returned by the query")
	rowsMetric.SetUnit("{rows}")
	
	rowsSum := rowsMetric.SetEmptySum()
	rowsSum.SetIsMonotonic(true)
	rowsSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	rdp := rowsSum.DataPoints().AppendEmpty()
	rdp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	rdp.SetIntValue(rows)
	rdp.Attributes().PutStr("query_id", queryID)
	
	r.collectionStats.RecordMetric()
}

// addSlowQueryLog adds a slow query log entry
func (r *postgresqlQueryReceiver) addSlowQueryLog(
	logs plog.Logs,
	dbName string,
	queryID string,
	query string,
	meanTime float64,
	planHash string,
) {
	rl := logs.ResourceLogs().AppendEmpty()
	resource := rl.Resource()
	resource.Attributes().PutStr(semconv.AttributeServiceName, "postgresql")
	resource.Attributes().PutStr(semconv.AttributeDBName, dbName)
	
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("otelcol/postgresqlquery")
	
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	logRecord.SetSeverityNumber(plog.SeverityNumberWarn)
	logRecord.SetSeverityText("WARN")
	
	// Set body
	logRecord.Body().SetStr("Slow query detected")
	
	// Set attributes
	attrs := logRecord.Attributes()
	attrs.PutStr("query_id", queryID)
	attrs.PutStr("query", query)
	attrs.PutDouble("mean_time_ms", meanTime)
	if planHash != "" {
		attrs.PutStr("plan_hash", planHash)
	}
	
	r.collectionStats.RecordLog()
}

// addPlanRegressionLog adds a plan regression log entry
func (r *postgresqlQueryReceiver) addPlanRegressionLog(
	logs plog.Logs,
	dbName string,
	regression *PlanRegression,
) {
	rl := logs.ResourceLogs().AppendEmpty()
	resource := rl.Resource()
	resource.Attributes().PutStr(semconv.AttributeServiceName, "postgresql")
	resource.Attributes().PutStr(semconv.AttributeDBName, dbName)
	
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("otelcol/postgresqlquery")
	
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(regression.DetectedAt))
	
	// Set severity based on regression severity
	switch regression.Severity {
	case "severe":
		logRecord.SetSeverityNumber(plog.SeverityNumberError)
		logRecord.SetSeverityText("ERROR")
	case "moderate":
		logRecord.SetSeverityNumber(plog.SeverityNumberWarn)
		logRecord.SetSeverityText("WARN")
	default:
		logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
		logRecord.SetSeverityText("INFO")
	}
	
	// Set body
	logRecord.Body().SetStr("Query plan regression detected")
	
	// Set attributes
	attrs := logRecord.Attributes()
	attrs.PutStr("query_id", regression.QueryID)
	attrs.PutStr("old_plan_hash", regression.OldPlanHash)
	attrs.PutStr("new_plan_hash", regression.NewPlanHash)
	attrs.PutDouble("old_cost", regression.OldCost)
	attrs.PutDouble("new_cost", regression.NewCost)
	attrs.PutDouble("cost_increase_pct", regression.CostIncrease)
	attrs.PutStr("severity", regression.Severity)
	
	// Add change details
	if len(regression.ChangeDetails) > 0 {
		changeList := attrs.PutEmptySlice("changes")
		for _, change := range regression.ChangeDetails {
			changeList.AppendEmpty().SetStr(change)
		}
	}
	
	r.collectionStats.RecordLog()
}

// addWaitEventMetric adds wait event metrics
func (r *postgresqlQueryReceiver) addWaitEventMetric(
	metrics pmetric.Metrics,
	dbName string,
	eventType string,
	eventName string,
	count int64,
) {
	rm := metrics.ResourceMetrics().AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr(semconv.AttributeServiceName, "postgresql")
	resource.Attributes().PutStr(semconv.AttributeDBName, dbName)
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/postgresqlquery")
	
	// Wait event count metric
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("postgresql.wait_event.count")
	metric.SetDescription("Number of times a wait event occurred")
	metric.SetUnit("{events}")
	
	sum := metric.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetIntValue(count)
	dp.Attributes().PutStr("wait_event_type", eventType)
	dp.Attributes().PutStr("wait_event", eventName)
	
	r.collectionStats.RecordMetric()
}

// addASHMetric adds Active Session History metrics
func (r *postgresqlQueryReceiver) addASHMetric(
	metrics pmetric.Metrics,
	dbName string,
	stats ASHStats,
) {
	rm := metrics.ResourceMetrics().AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr(semconv.AttributeServiceName, "postgresql")
	resource.Attributes().PutStr(semconv.AttributeDBName, dbName)
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/postgresqlquery")
	
	// Active sessions metric
	activeMetric := sm.Metrics().AppendEmpty()
	activeMetric.SetName("postgresql.ash.active_sessions")
	activeMetric.SetDescription("Average number of active sessions")
	activeMetric.SetUnit("{sessions}")
	
	activeGauge := activeMetric.SetEmptyGauge()
	adp := activeGauge.DataPoints().AppendEmpty()
	adp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	adp.SetDoubleValue(stats.ActiveSessionsAvg)
	
	// Waiting sessions metric
	waitingMetric := sm.Metrics().AppendEmpty()
	waitingMetric.SetName("postgresql.ash.waiting_sessions")
	waitingMetric.SetDescription("Average number of waiting sessions")
	waitingMetric.SetUnit("{sessions}")
	
	waitingGauge := waitingMetric.SetEmptyGauge()
	wdp := waitingGauge.DataPoints().AppendEmpty()
	wdp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	wdp.SetDoubleValue(stats.WaitingSessionsAvg)
	
	// Sample collection time metric
	sampleTimeMetric := sm.Metrics().AppendEmpty()
	sampleTimeMetric.SetName("postgresql.ash.sample_duration")
	sampleTimeMetric.SetDescription("Average time to collect ASH sample")
	sampleTimeMetric.SetUnit("ms")
	
	sampleTimeGauge := sampleTimeMetric.SetEmptyGauge()
	sdp := sampleTimeGauge.DataPoints().AppendEmpty()
	sdp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	sdp.SetDoubleValue(float64(stats.AvgSampleDuration.Milliseconds()))
	
	r.collectionStats.RecordMetric()
}

// addTableMetric adds table-level metrics
func (r *postgresqlQueryReceiver) addTableMetric(
	metrics pmetric.Metrics,
	dbName string,
	schemaName string,
	tableName string,
	seqScans int64,
	seqTupRead int64,
	idxScans int64,
	idxTupFetch int64,
	nTupIns int64,
	nTupUpd int64,
	nTupDel int64,
	nLiveTup int64,
	nDeadTup int64,
) {
	rm := metrics.ResourceMetrics().AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr(semconv.AttributeServiceName, "postgresql")
	resource.Attributes().PutStr(semconv.AttributeDBName, dbName)
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/postgresqlquery")
	
	// Common attributes for all table metrics
	commonAttrs := func(dp pmetric.NumberDataPoint) {
		dp.Attributes().PutStr("schema", schemaName)
		dp.Attributes().PutStr("table", tableName)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	}
	
	// Sequential scans
	seqScanMetric := sm.Metrics().AppendEmpty()
	seqScanMetric.SetName("postgresql.table.seq_scan")
	seqScanMetric.SetDescription("Number of sequential scans on this table")
	seqScanMetric.SetUnit("{scans}")
	
	seqScanSum := seqScanMetric.SetEmptySum()
	seqScanSum.SetIsMonotonic(true)
	seqScanSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	ssdp := seqScanSum.DataPoints().AppendEmpty()
	commonAttrs(&ssdp)
	ssdp.SetIntValue(seqScans)
	
	// Index scans
	idxScanMetric := sm.Metrics().AppendEmpty()
	idxScanMetric.SetName("postgresql.table.idx_scan")
	idxScanMetric.SetDescription("Number of index scans on this table")
	idxScanMetric.SetUnit("{scans}")
	
	idxScanSum := idxScanMetric.SetEmptySum()
	idxScanSum.SetIsMonotonic(true)
	idxScanSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	isdp := idxScanSum.DataPoints().AppendEmpty()
	commonAttrs(&isdp)
	isdp.SetIntValue(idxScans)
	
	// Live tuples
	liveTupMetric := sm.Metrics().AppendEmpty()
	liveTupMetric.SetName("postgresql.table.n_live_tup")
	liveTupMetric.SetDescription("Estimated number of live rows")
	liveTupMetric.SetUnit("{rows}")
	
	liveTupGauge := liveTupMetric.SetEmptyGauge()
	ltdp := liveTupGauge.DataPoints().AppendEmpty()
	commonAttrs(&ltdp)
	ltdp.SetIntValue(nLiveTup)
	
	// Dead tuples
	deadTupMetric := sm.Metrics().AppendEmpty()
	deadTupMetric.SetName("postgresql.table.n_dead_tup")
	deadTupMetric.SetDescription("Estimated number of dead rows")
	deadTupMetric.SetUnit("{rows}")
	
	deadTupGauge := deadTupMetric.SetEmptyGauge()
	dtdp := deadTupGauge.DataPoints().AppendEmpty()
	commonAttrs(&dtdp)
	dtdp.SetIntValue(nDeadTup)
	
	r.collectionStats.RecordMetric()
}