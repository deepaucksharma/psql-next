package enhancedsql

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	
	"github.com/database-intelligence/common/featuredetector"
	"github.com/database-intelligence/common/queryselector"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// collect performs a collection cycle
func (r *Receiver) collect(ctx context.Context) {
	startTime := time.Now()
	
	// Get selected queries for all categories
	queries, err := r.selector.GetAllQueries(ctx)
	if err != nil {
		r.logger.Error("Failed to select queries", zap.Error(err))
		r.errorCount++
		return
	}
	
	// Execute queries for each category
	for category, queryDef := range queries {
		if queryDef == nil {
			continue
		}
		
		// Check if this is a fallback query
		isFallback := queryDef.Priority <= 20 // Low priority indicates fallback
		if isFallback {
			r.fallbackCount++
		}
		
		// Find query configuration
		var queryConfig *QueryConfig
		for _, qc := range r.config.Queries {
			if qc.Category == string(category) {
				queryConfig = &qc
				break
			}
		}
		
		if queryConfig == nil {
			r.logger.Debug("No configuration for category, skipping",
				zap.String("category", string(category)))
			continue
		}
		
		// Execute query
		if err := r.executeQuery(ctx, queryDef, queryConfig); err != nil {
			r.logger.Error("Failed to execute query",
				zap.String("category", string(category)),
				zap.String("query", queryDef.Name),
				zap.Error(err))
			r.errorCount++
			
			// Try fallback if available
			if queryDef.FallbackName != "" {
				r.tryFallback(ctx, category, queryDef.FallbackName, queryConfig)
			}
		} else {
			r.successCount++
		}
	}
	
	r.logger.Debug("Collection cycle completed",
		zap.Duration("duration", time.Since(startTime)),
		zap.Int("queries_executed", len(queries)))
}

// executeQuery executes a single query and processes results
func (r *Receiver) executeQuery(ctx context.Context, queryDef *featuredetector.QueryDefinition, config *QueryConfig) error {
	// Set query timeout
	queryCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()
	
	// Prepare query arguments
	args := r.prepareQueryArgs(config)
	
	// Execute query
	rows, err := r.db.QueryContext(queryCtx, queryDef.SQL, args...)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()
	
	// Get column information
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}
	
	// Process results based on output type
	if len(config.Metrics) > 0 {
		return r.processMetrics(ctx, rows, columns, config)
	} else if len(config.Logs) > 0 {
		return r.processLogs(ctx, rows, columns, config)
	}
	
	return fmt.Errorf("no output configuration (metrics or logs) specified")
}

// processMetrics processes query results as metrics
func (r *Receiver) processMetrics(ctx context.Context, rows *sql.Rows, columns []string, config *QueryConfig) error {
	timestamp := time.Now()
	
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	
	// Set resource attributes
	resource := rm.Resource()
	resource.Attributes().PutStr("db.system", r.config.Driver)
	if r.config.ResourceAttributes != nil {
		for k, v := range r.config.ResourceAttributes {
			resource.Attributes().PutStr(k, v)
		}
	}
	
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("enhancedsql")
	
	// Create metrics based on configuration
	metricMap := make(map[string]pmetric.Metric)
	for _, metricConfig := range config.Metrics {
		metric := sm.Metrics().AppendEmpty()
		metric.SetName(metricConfig.MetricName)
		if metricConfig.Description != "" {
			metric.SetDescription(metricConfig.Description)
		}
		
		// Set metric type
		switch metricConfig.ValueType {
		case "gauge", "int", "double":
			metric.SetEmptyGauge()
		case "sum":
			sum := metric.SetEmptySum()
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		case "histogram":
			metric.SetEmptyHistogram()
		default:
			metric.SetEmptyGauge()
		}
		
		metricMap[metricConfig.MetricName] = metric
	}
	
	// Process rows
	scanArgs := make([]interface{}, len(columns))
	for i := range scanArgs {
		scanArgs[i] = new(interface{})
	}
	
	rowCount := 0
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			r.logger.Warn("Failed to scan row", zap.Error(err))
			continue
		}
		
		// Create column value map
		values := make(map[string]interface{})
		for i, col := range columns {
			values[col] = *(scanArgs[i].(*interface{}))
		}
		
		// Process each metric configuration
		for _, metricConfig := range config.Metrics {
			metric := metricMap[metricConfig.MetricName]
			
			// Get value
			value, ok := values[metricConfig.ValueColumn]
			if !ok {
				r.logger.Warn("Value column not found",
					zap.String("column", metricConfig.ValueColumn))
				continue
			}
			
			// Convert value
			var numericValue float64
			switch v := value.(type) {
			case int64:
				numericValue = float64(v)
			case float64:
				numericValue = v
			case int:
				numericValue = float64(v)
			case string:
				// Try to parse
				if _, err := fmt.Sscanf(v, "%f", &numericValue); err != nil {
					r.logger.Warn("Failed to parse numeric value",
						zap.String("value", v),
						zap.Error(err))
					continue
				}
			default:
				r.logger.Warn("Unsupported value type",
					zap.String("type", fmt.Sprintf("%T", v)))
				continue
			}
			
			// Add data point
			switch metric.Type() {
			case pmetric.MetricTypeGauge:
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pmetric.NewTimestampFromTime(timestamp))
				dp.SetDoubleValue(numericValue)
				
				// Add attributes
				for _, attrCol := range metricConfig.AttributeColumns {
					if attrVal, ok := values[attrCol]; ok {
						dp.Attributes().PutStr(attrCol, fmt.Sprintf("%v", attrVal))
					}
				}
				
			case pmetric.MetricTypeSum:
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetTimestamp(pmetric.NewTimestampFromTime(timestamp))
				dp.SetDoubleValue(numericValue)
				
				// Add attributes
				for _, attrCol := range metricConfig.AttributeColumns {
					if attrVal, ok := values[attrCol]; ok {
						dp.Attributes().PutStr(attrCol, fmt.Sprintf("%v", attrVal))
					}
				}
			}
		}
		
		rowCount++
		if config.MaxRows > 0 && rowCount >= config.MaxRows {
			break
		}
	}
	
	// Send metrics
	if r.metricsConsumer != nil && rowCount > 0 {
		if err := r.metricsConsumer.ConsumeMetrics(ctx, metrics); err != nil {
			return fmt.Errorf("failed to send metrics: %w", err)
		}
	}
	
	r.logger.Debug("Processed metrics",
		zap.String("query", config.Name),
		zap.Int("rows", rowCount))
	
	return nil
}

// processLogs processes query results as logs
func (r *Receiver) processLogs(ctx context.Context, rows *sql.Rows, columns []string, config *QueryConfig) error {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	
	// Set resource attributes
	resource := rl.Resource()
	resource.Attributes().PutStr("db.system", r.config.Driver)
	if r.config.ResourceAttributes != nil {
		for k, v := range r.config.ResourceAttributes {
			resource.Attributes().PutStr(k, v)
		}
	}
	
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("enhancedsql")
	
	// Process rows
	scanArgs := make([]interface{}, len(columns))
	for i := range scanArgs {
		scanArgs[i] = new(interface{})
	}
	
	rowCount := 0
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			r.logger.Warn("Failed to scan row", zap.Error(err))
			continue
		}
		
		// Create column value map
		values := make(map[string]interface{})
		for i, col := range columns {
			values[col] = *(scanArgs[i].(*interface{}))
		}
		
		// Process each log configuration
		for _, logConfig := range config.Logs {
			logRecord := sl.LogRecords().AppendEmpty()
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			
			// Set body
			if bodyVal, ok := values[logConfig.BodyColumn]; ok {
				logRecord.Body().SetStr(fmt.Sprintf("%v", bodyVal))
			}
			
			// Set severity
			if logConfig.SeverityColumn != "" {
				if sevVal, ok := values[logConfig.SeverityColumn]; ok {
					// Map severity value to plog severity
					logRecord.SetSeverityNumber(mapSeverity(fmt.Sprintf("%v", sevVal)))
				}
			}
			
			// Add attributes
			for attrName, attrCol := range logConfig.Attributes {
				if attrVal, ok := values[attrCol]; ok {
					logRecord.Attributes().PutStr(attrName, fmt.Sprintf("%v", attrVal))
				}
			}
		}
		
		rowCount++
		if config.MaxRows > 0 && rowCount >= config.MaxRows {
			break
		}
	}
	
	// Send logs
	if r.logsConsumer != nil && rowCount > 0 {
		if err := r.logsConsumer.ConsumeLogs(ctx, logs); err != nil {
			return fmt.Errorf("failed to send logs: %w", err)
		}
	}
	
	r.logger.Debug("Processed logs",
		zap.String("query", config.Name),
		zap.Int("rows", rowCount))
	
	return nil
}

// prepareQueryArgs prepares query arguments based on configuration
func (r *Receiver) prepareQueryArgs(config *QueryConfig) []interface{} {
	args := make([]interface{}, 0, len(config.Parameters))
	
	for _, param := range config.Parameters {
		switch param.Type {
		case "int":
			args = append(args, param.DefaultInt)
		case "float":
			args = append(args, param.DefaultFloat)
		case "string":
			args = append(args, param.DefaultString)
		case "duration":
			// Convert duration to appropriate unit
			d := param.DefaultDuration
			switch param.Unit {
			case "ms", "milliseconds":
				args = append(args, d.Milliseconds())
			case "s", "seconds":
				args = append(args, int(d.Seconds()))
			case "m", "minutes":
				args = append(args, int(d.Minutes()))
			default:
				args = append(args, d.Milliseconds())
			}
		default:
			args = append(args, param.DefaultString)
		}
	}
	
	return args
}

// tryFallback attempts to use a fallback query
func (r *Receiver) tryFallback(ctx context.Context, category queryselector.QueryCategory, fallbackName string, config *QueryConfig) {
	r.logger.Info("Attempting fallback query",
		zap.String("category", string(category)),
		zap.String("fallback", fallbackName))
	
	// This would need to be implemented to look up the fallback query by name
	// For now, we'll just log the attempt
	r.fallbackCount++
}

// mapSeverity maps string severity to plog severity number
func mapSeverity(severity string) plog.SeverityNumber {
	switch severity {
	case "TRACE", "trace":
		return plog.SeverityNumberTrace
	case "DEBUG", "debug":
		return plog.SeverityNumberDebug
	case "INFO", "info":
		return plog.SeverityNumberInfo
	case "WARN", "warn", "WARNING", "warning":
		return plog.SeverityNumberWarn
	case "ERROR", "error":
		return plog.SeverityNumberError
	case "FATAL", "fatal":
		return plog.SeverityNumberFatal
	default:
		return plog.SeverityNumberUnspecified
	}
}