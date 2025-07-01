package querycorrelator

import (
	"context"
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// queryCorrelator correlates individual query metrics with database and table metrics
type queryCorrelator struct {
	config       *Config
	logger       *zap.Logger
	nextConsumer consumer.Metrics

	// Correlation state
	queryIndex    map[string]*queryInfo
	tableIndex    map[string]*tableInfo
	databaseIndex map[string]*databaseInfo
	mutex         sync.RWMutex

	// Metrics
	correlationsCreated int64
	metricsEnriched    int64
}

type queryInfo struct {
	queryID       string
	queryText     string
	database      string
	statementType string
	primaryTable  string
	lastSeen      time.Time
	execCount     int64
	totalTime     float64
}

type tableInfo struct {
	database      string
	schema        string
	table         string
	modifications int64
	deadTuples    int64
	lastVacuum    time.Time
	lastAnalyze   time.Time
}

type databaseInfo struct {
	name           string
	totalQueries   int64
	slowQueries    int64
	totalExecTime  float64
	activeBackends int64
}

// Start initializes the processor
func (p *queryCorrelator) Start(ctx context.Context, host component.Host) error {
	p.logger.Info("Starting query correlator processor")
	
	// Start background cleanup
	go p.cleanupLoop()
	
	return nil
}

// Shutdown stops the processor
func (p *queryCorrelator) Shutdown(context.Context) error {
	p.logger.Info("Shutting down query correlator processor")
	return nil
}

// Capabilities returns the consumer capabilities
func (p *queryCorrelator) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

// ConsumeMetrics processes metrics and adds correlations
func (p *queryCorrelator) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// First pass: index all metrics
	p.indexMetrics(md)
	
	// Second pass: enrich metrics with correlations
	p.enrichMetrics(md)
	
	// Pass to next consumer
	return p.nextConsumer.ConsumeMetrics(ctx, md)
}

// indexMetrics builds indices of queries, tables, and databases
func (p *queryCorrelator) indexMetrics(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				p.indexMetric(metric)
			}
		}
	}
}

// indexMetric indexes individual metrics by type
func (p *queryCorrelator) indexMetric(metric pmetric.Metric) {
	switch metric.Name() {
	case "db.query.execution_count", "db.query.total_time", "db.query.mean_time":
		p.indexQueryMetric(metric)
	case "db.table.modifications", "db.table.dead_tuples":
		p.indexTableMetric(metric)
	case "postgresql.database.backends", "db.connections.active":
		p.indexDatabaseMetric(metric)
	}
}

// indexQueryMetric indexes query performance metrics
func (p *queryCorrelator) indexQueryMetric(metric pmetric.Metric) {
	var dps pmetric.NumberDataPointSlice
	
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = metric.Sum().DataPoints()
	default:
		return
	}
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		attrs := dp.Attributes()
		
		queryID, _ := attrs.Get("queryid")
		if queryID.Str() == "" {
			continue
		}
		
		query, exists := p.queryIndex[queryID.Str()]
		if !exists {
			query = &queryInfo{
				queryID:  queryID.Str(),
				lastSeen: time.Now(),
			}
			p.queryIndex[queryID.Str()] = query
		}
		
		// Update query info
		if db, ok := attrs.Get("database_name"); ok {
			query.database = db.Str()
		}
		if stmt, ok := attrs.Get("statement_type"); ok {
			query.statementType = stmt.Str()
		}
		if table, ok := attrs.Get("primary_table"); ok {
			query.primaryTable = table.Str()
		}
		if text, ok := attrs.Get("query_text"); ok {
			query.queryText = text.Str()
		}
		
		// Update metrics
		switch metric.Name() {
		case "db.query.execution_count":
			query.execCount = dp.IntValue()
		case "db.query.total_time":
			query.totalTime = dp.DoubleValue()
		}
		
		query.lastSeen = time.Now()
	}
}

// indexTableMetric indexes table statistics
func (p *queryCorrelator) indexTableMetric(metric pmetric.Metric) {
	var dps pmetric.NumberDataPointSlice
	
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = metric.Sum().DataPoints()
	default:
		return
	}
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		attrs := dp.Attributes()
		
		schema, _ := attrs.Get("schemaname")
		table, _ := attrs.Get("tablename")
		if schema.Str() == "" || table.Str() == "" {
			continue
		}
		
		key := fmt.Sprintf("%s.%s", schema.Str(), table.Str())
		tbl, exists := p.tableIndex[key]
		if !exists {
			tbl = &tableInfo{
				schema: schema.Str(),
				table:  table.Str(),
			}
			p.tableIndex[key] = tbl
		}
		
		// Update table info
		switch metric.Name() {
		case "db.table.modifications":
			tbl.modifications = dp.IntValue()
		case "db.table.dead_tuples":
			tbl.deadTuples = dp.IntValue()
		}
	}
}

// indexDatabaseMetric indexes database-level metrics
func (p *queryCorrelator) indexDatabaseMetric(metric pmetric.Metric) {
	var dps pmetric.NumberDataPointSlice
	
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = metric.Sum().DataPoints()
	default:
		return
	}
	
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		attrs := dp.Attributes()
		
		dbName, _ := attrs.Get("database_name")
		if dbName.Str() == "" {
			continue
		}
		
		db, exists := p.databaseIndex[dbName.Str()]
		if !exists {
			db = &databaseInfo{
				name: dbName.Str(),
			}
			p.databaseIndex[dbName.Str()] = db
		}
		
		// Update database info
		switch metric.Name() {
		case "postgresql.database.backends", "db.connections.active":
			db.activeBackends = dp.IntValue()
		}
	}
}

// enrichMetrics adds correlation attributes to metrics
func (p *queryCorrelator) enrichMetrics(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		sms := rm.ScopeMetrics()
		
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			metrics := sm.Metrics()
			
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				p.enrichMetric(metric)
			}
		}
	}
}

// enrichMetric adds correlation data to individual metrics
func (p *queryCorrelator) enrichMetric(metric pmetric.Metric) {
	// Only enrich query metrics
	if !p.isQueryMetric(metric.Name()) {
		return
	}
	
	var dps pmetric.NumberDataPointSlice
	
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = metric.Sum().DataPoints()
	default:
		return
	}
	
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		attrs := dp.Attributes()
		
		queryID, _ := attrs.Get("queryid")
		if queryID.Str() == "" {
			continue
		}
		
		// Get query info
		query, exists := p.queryIndex[queryID.Str()]
		if !exists {
			continue
		}
		
		// Add correlation attributes
		attrs.PutStr("correlation.query_id", query.queryID)
		attrs.PutStr("correlation.database", query.database)
		attrs.PutStr("correlation.statement_type", query.statementType)
		
		// Add query performance category
		if query.totalTime > 0 && query.execCount > 0 {
			avgTime := query.totalTime / float64(query.execCount)
			if avgTime > 1000 {
				attrs.PutStr("performance.category", "slow")
			} else if avgTime > 100 {
				attrs.PutStr("performance.category", "moderate")
			} else {
				attrs.PutStr("performance.category", "fast")
			}
		}
		
		// Add table correlation if available
		if query.primaryTable != "" {
			attrs.PutStr("correlation.table", query.primaryTable)
			
			// Look up table info
			if tbl, exists := p.tableIndex[query.primaryTable]; exists {
				attrs.PutInt("table.modifications", tbl.modifications)
				attrs.PutInt("table.dead_tuples", tbl.deadTuples)
				
				// Add maintenance indicator
				if tbl.deadTuples > 1000 {
					attrs.PutBool("table.needs_vacuum", true)
				}
			}
		}
		
		// Add database correlation
		if db, exists := p.databaseIndex[query.database]; exists {
			attrs.PutInt("database.active_backends", db.activeBackends)
			
			// Calculate query's contribution to database load
			if db.totalExecTime > 0 {
				contribution := (query.totalTime / db.totalExecTime) * 100
				attrs.PutDouble("query.load_contribution_pct", contribution)
			}
		}
		
		// Generate correlation hash for tracking
		correlationID := p.generateCorrelationID(query)
		attrs.PutStr("correlation.id", correlationID)
		
		p.metricsEnriched++
	}
}

// isQueryMetric checks if a metric is query-related
func (p *queryCorrelator) isQueryMetric(name string) bool {
	queryMetrics := []string{
		"db.query.execution_count",
		"db.query.total_time",
		"db.query.mean_time",
		"db.query.rows_returned",
		"db.query.cache_hit_ratio",
		"db.query.blocks_read",
		"db.query.blocks_hit",
		"db.query.temp_blocks",
		"db.query.io_time",
	}
	
	for _, qm := range queryMetrics {
		if name == qm {
			return true
		}
	}
	return false
}

// generateCorrelationID creates a unique ID for correlation tracking
func (p *queryCorrelator) generateCorrelationID(query *queryInfo) string {
	data := fmt.Sprintf("%s:%s:%s:%s",
		query.queryID,
		query.database,
		query.statementType,
		query.primaryTable,
	)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// cleanupLoop periodically cleans up old correlation data
func (p *queryCorrelator) cleanupLoop() {
	ticker := time.NewTicker(p.config.CleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		p.cleanupOldData()
	}
}

// cleanupOldData removes correlation data older than retention period
func (p *queryCorrelator) cleanupOldData() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	cutoff := time.Now().Add(-p.config.RetentionPeriod)
	
	// Clean up old queries
	for id, query := range p.queryIndex {
		if query.lastSeen.Before(cutoff) {
			delete(p.queryIndex, id)
		}
	}
	
	p.logger.Debug("Cleaned up correlation data",
		zap.Int("remaining_queries", len(p.queryIndex)),
		zap.Int("remaining_tables", len(p.tableIndex)),
		zap.Int("remaining_databases", len(p.databaseIndex)),
	)
}