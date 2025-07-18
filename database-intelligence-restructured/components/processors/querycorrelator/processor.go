package querycorrelator

import (
	"context"
	"crypto/md5"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	
	"github.com/database-intelligence/db-intel/components/internal/boundedmap"
)

// queryCorrelator correlates individual query metrics with database and table metrics
type queryCorrelator struct {
	config       *Config
	logger       *zap.Logger
	nextConsumer consumer.Metrics

	// Correlation state with bounded maps
	queryIndex    *boundedmap.BoundedMap
	tableIndex    *boundedmap.BoundedMap
	databaseIndex *boundedmap.BoundedMap
	mutex         sync.RWMutex

	// Metrics
	correlationsCreated int64
	metricsEnriched    int64

	// Shutdown management
	shutdownChan chan struct{}
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
	close(p.shutdownChan)
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
		
		queryVal, exists := p.queryIndex.Get(queryID.Str())
		var query *queryInfo
		if !exists {
			query = &queryInfo{
				queryID:  queryID.Str(),
				lastSeen: time.Now(),
			}
			p.queryIndex.Put(queryID.Str(), query)
		} else {
			query = queryVal.(*queryInfo)
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
		tblVal, exists := p.tableIndex.Get(key)
		var tbl *tableInfo
		if !exists {
			tbl = &tableInfo{
				schema: schema.Str(),
				table:  table.Str(),
			}
			p.tableIndex.Put(key, tbl)
		} else {
			tbl = tblVal.(*tableInfo)
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
		
		dbVal, exists := p.databaseIndex.Get(dbName.Str())
		var db *databaseInfo
		if !exists {
			db = &databaseInfo{
				name: dbName.Str(),
			}
			p.databaseIndex.Put(dbName.Str(), db)
		} else {
			db = dbVal.(*databaseInfo)
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
	
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		p.enrichDataPoints(metric.Gauge().DataPoints())
	case pmetric.MetricTypeSum:
		p.enrichDataPoints(metric.Sum().DataPoints())
	case pmetric.MetricTypeHistogram:
		p.enrichHistogramDataPoints(metric.Histogram().DataPoints())
	default:
		return
	}
}

// enrichDataPoints enriches number data points
func (p *queryCorrelator) enrichDataPoints(dps pmetric.NumberDataPointSlice) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		p.addCorrelationAttributes(dp.Attributes())
	}
}

// enrichHistogramDataPoints enriches histogram data points
func (p *queryCorrelator) enrichHistogramDataPoints(dps pmetric.HistogramDataPointSlice) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		attrs := dp.Attributes()
		
		// For histogram metrics, add the average duration as an attribute for categorization
		if dp.Count() > 0 {
			avgDuration := dp.Sum() / float64(dp.Count())
			attrs.PutDouble("_avg_duration_ms", avgDuration)
		}
		
		p.addCorrelationAttributes(attrs)
	}
}

// addCorrelationAttributes adds correlation attributes to a data point
func (p *queryCorrelator) addCorrelationAttributes(attrs pcommon.Map) {
		queryID, _ := attrs.Get("queryid")
		if queryID.Str() == "" {
			// For metrics without queryid, try to generate one from query text
			if queryText, ok := attrs.Get("query.text"); ok && queryText.Str() != "" {
				// Generate a simple ID from query text
				hash := md5.Sum([]byte(queryText.Str()))
				queryIDStr := fmt.Sprintf("%x", hash[:8])
				attrs.PutStr("correlation.query_id", queryIDStr)
				
				// Check if this is a maintenance query
				if p.isMaintenanceQuery(queryText.Str()) {
					attrs.PutBool("query.is_maintenance", true)
				}
				
				// Extract tables from query text if possible
				tables := p.extractTablesFromQuery(queryText.Str())
				if len(tables) > 0 {
					attrs.PutStr("correlation.tables", tables)
				}
				
				// Add performance category based on duration
				if p.config.CorrelationAttributes.AddQueryCategory {
					p.addPerformanceCategory(attrs)
				}
				
				p.metricsEnriched++
				return
			}
			return
		}
		
		// Get query info
		queryVal, exists := p.queryIndex.Get(queryID.Str())
		if !exists {
			return
		}
		query := queryVal.(*queryInfo)
		
		// Add correlation attributes
		attrs.PutStr("correlation.query_id", query.queryID)
		attrs.PutStr("correlation.database", query.database)
		attrs.PutStr("correlation.statement_type", query.statementType)
		
		// Add query performance category
		if query.totalTime > 0 && query.execCount > 0 {
			avgTime := query.totalTime / float64(query.execCount)
			if avgTime > p.config.QueryCategorization.SlowQueryThresholdMs {
				attrs.PutStr("performance.category", "slow")
			} else if avgTime > p.config.QueryCategorization.ModerateQueryThresholdMs {
				attrs.PutStr("performance.category", "moderate")
			} else {
				attrs.PutStr("performance.category", "fast")
			}
		}
		
		// Add table correlation if available
		if query.primaryTable != "" {
			attrs.PutStr("correlation.table", query.primaryTable)
			
			// Look up table info
			if tblVal, exists := p.tableIndex.Get(query.primaryTable); exists {
				tbl := tblVal.(*tableInfo)
				attrs.PutInt("table.modifications", tbl.modifications)
				attrs.PutInt("table.dead_tuples", tbl.deadTuples)
				
				// Add maintenance indicator
				if tbl.deadTuples > 1000 {
					attrs.PutBool("table.needs_vacuum", true)
				}
			}
		}
		
		// Add database correlation
		if dbVal, exists := p.databaseIndex.Get(query.database); exists {
			db := dbVal.(*databaseInfo)
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
		"db.query.duration", // Add support for duration histogram
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
	
	for {
		select {
		case <-ticker.C:
			p.cleanupOldData()
		case <-p.shutdownChan:
			return
		}
	}
}

// cleanupOldData removes correlation data older than retention period
func (p *queryCorrelator) cleanupOldData() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	
	// Clean up old queries using the bounded map's cleanup method
	removed := p.queryIndex.CleanupOlderThan(p.config.RetentionPeriod)
	p.logger.Debug("Cleaned up old queries", zap.Int("removed", removed))
	
	p.logger.Debug("Cleaned up correlation data",
		zap.Int("remaining_queries", p.queryIndex.Len()),
		zap.Int("remaining_tables", p.tableIndex.Len()),
		zap.Int("remaining_databases", p.databaseIndex.Len()),
	)
}

// isMaintenanceQuery checks if a query is a maintenance operation
func (p *queryCorrelator) isMaintenanceQuery(queryText string) bool {
	queryUpper := strings.ToUpper(queryText)
	maintenanceKeywords := []string{
		"VACUUM",
		"ANALYZE",
		"REINDEX",
		"CREATE INDEX",
		"DROP INDEX",
		"ALTER TABLE",
		"CLUSTER",
		"CHECKPOINT",
	}
	
	for _, keyword := range maintenanceKeywords {
		if strings.Contains(queryUpper, keyword) {
			return true
		}
	}
	
	return false
}

// extractTablesFromQuery attempts to extract table names from a query
func (p *queryCorrelator) extractTablesFromQuery(queryText string) string {
	// Simple regex to find table names after FROM, JOIN, UPDATE, INSERT INTO
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)FROM\s+([a-zA-Z0-9_]+)`),
		regexp.MustCompile(`(?i)JOIN\s+([a-zA-Z0-9_]+)`),
		regexp.MustCompile(`(?i)UPDATE\s+([a-zA-Z0-9_]+)`),
		regexp.MustCompile(`(?i)INSERT\s+INTO\s+([a-zA-Z0-9_]+)`),
		regexp.MustCompile(`(?i)DELETE\s+FROM\s+([a-zA-Z0-9_]+)`),
	}
	
	tables := make(map[string]bool)
	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(queryText, -1)
		for _, match := range matches {
			if len(match) > 1 {
				tables[match[1]] = true
			}
		}
	}
	
	if len(tables) == 0 {
		return ""
	}
	
	// Convert map to comma-separated string
	var tableList []string
	for table := range tables {
		tableList = append(tableList, table)
	}
	
	return strings.Join(tableList, ",")
}

// addPerformanceCategory adds a performance category based on query duration
func (p *queryCorrelator) addPerformanceCategory(attrs pcommon.Map) {
	// Check for duration value in various possible attributes
	var duration float64
	var found bool
	
	// Try to get duration from common attribute names
	if val, ok := attrs.Get("_avg_duration_ms"); ok {
		switch val.Type() {
		case pcommon.ValueTypeDouble:
			duration = val.Double()
			found = true
		case pcommon.ValueTypeInt:
			duration = float64(val.Int())
			found = true
		}
	} else if val, ok := attrs.Get("duration"); ok {
		switch val.Type() {
		case pcommon.ValueTypeDouble:
			duration = val.Double()
			found = true
		case pcommon.ValueTypeInt:
			duration = float64(val.Int())
			found = true
		}
	} else if val, ok := attrs.Get("duration_ms"); ok {
		switch val.Type() {
		case pcommon.ValueTypeDouble:
			duration = val.Double()
			found = true
		case pcommon.ValueTypeInt:
			duration = float64(val.Int())
			found = true
		}
	}
	
	if !found {
		// Default to moderate if we can't determine duration
		attrs.PutStr("query.performance_category", "moderate")
		return
	}
	
	// Categorize based on duration in milliseconds
	if duration > 100 {
		attrs.PutStr("query.performance_category", "slow")
	} else if duration > 50 {
		attrs.PutStr("query.performance_category", "moderate")
	} else {
		attrs.PutStr("query.performance_category", "fast")
	}
}