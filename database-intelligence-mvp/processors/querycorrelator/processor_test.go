package querycorrelator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestNewQueryCorrelator(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := &consumertest.MetricsSink{}
	
	processor := &queryCorrelator{
		config:        cfg,
		logger:        logger,
		nextConsumer:  consumer,
		queryIndex:    make(map[string]*queryInfo),
		tableIndex:    make(map[string]*tableInfo),
		databaseIndex: make(map[string]*databaseInfo),
	}
	require.NotNil(t, processor)
}

func TestQueryCorrelator_BasicCorrelation(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := &consumertest.MetricsSink{}
	processor := &queryCorrelator{
		config:        cfg,
		logger:        logger,
		nextConsumer:  consumer,
		queryIndex:    make(map[string]*queryInfo),
		tableIndex:    make(map[string]*tableInfo),
		databaseIndex: make(map[string]*databaseInfo),
	}
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	// First, send table metrics
	tableMetrics := createTableMetrics("testdb", "users", 1000)
	err = processor.ConsumeMetrics(context.Background(), tableMetrics)
	require.NoError(t, err)
	
	// Then send query metrics that reference the table
	queryMetrics := createQueryMetrics("testdb", "SELECT * FROM users WHERE id = ?", 100*time.Millisecond)
	err = processor.ConsumeMetrics(context.Background(), queryMetrics)
	require.NoError(t, err)
	
	// Check that correlation attributes were added
	processedMetrics := consumer.AllMetrics()
	require.Greater(t, len(processedMetrics), 0)
	
	// Find the query metric
	var queryMetric pmetric.Metric
	found := false
	for _, m := range processedMetrics {
		for i := 0; i < m.ResourceMetrics().Len(); i++ {
			rm := m.ResourceMetrics().At(i)
			for j := 0; j < rm.ScopeMetrics().Len(); j++ {
				sm := rm.ScopeMetrics().At(j)
				for k := 0; k < sm.Metrics().Len(); k++ {
					metric := sm.Metrics().At(k)
					if metric.Name() == "db.query.duration" {
						queryMetric = metric
						found = true
						break
					}
				}
			}
		}
	}
	
	require.True(t, found, "Query metric not found")
	
	// Check correlation attributes
	dp := queryMetric.Histogram().DataPoints().At(0)
	
	tables, exists := dp.Attributes().Get("correlation.tables")
	assert.True(t, exists)
	assert.Contains(t, tables.Str(), "users")
	
	queryID, exists := dp.Attributes().Get("correlation.query_id")
	assert.True(t, exists)
	assert.NotEmpty(t, queryID.Str())
}

func TestQueryCorrelator_QueryCategorization(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	// TODO: Add query categories to config
	// cfg.QueryCategories.SlowQueryThresholdMs = 100
	// TODO: Add query categories to config
	// cfg.QueryCategories.ModerateQueryThresholdMs = 50
	
	logger := zap.NewNop()
	consumer := &consumertest.MetricsSink{}
	processor := &queryCorrelator{
		config:        cfg,
		logger:        logger,
		nextConsumer:  consumer,
		queryIndex:    make(map[string]*queryInfo),
		tableIndex:    make(map[string]*tableInfo),
		databaseIndex: make(map[string]*databaseInfo),
	}
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	testCases := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"fast query", 10 * time.Millisecond, "fast"},
		{"moderate query", 75 * time.Millisecond, "moderate"},
		{"slow query", 200 * time.Millisecond, "slow"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := createQueryMetrics("testdb", "SELECT * FROM test", tc.duration)
			err = processor.ConsumeMetrics(context.Background(), metrics)
			require.NoError(t, err)
			
			// Check the last processed metric
			allMetrics := consumer.AllMetrics()
			lastMetric := allMetrics[len(allMetrics)-1]
			
			metric := lastMetric.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			dp := metric.Histogram().DataPoints().At(0)
			
			category, exists := dp.Attributes().Get("query.performance_category")
			assert.True(t, exists)
			assert.Equal(t, tc.expected, category.Str())
		})
	}
}

func TestQueryCorrelator_MaintenanceIndicators(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := &consumertest.MetricsSink{}
	processor := &queryCorrelator{
		config:        cfg,
		logger:        logger,
		nextConsumer:  consumer,
		queryIndex:    make(map[string]*queryInfo),
		tableIndex:    make(map[string]*tableInfo),
		databaseIndex: make(map[string]*databaseInfo),
	}
	
	err := processor.Start(context.Background(), nil)
	require.NoError(t, err)
	defer processor.Shutdown(context.Background())
	
	maintenanceQueries := []string{
		"VACUUM ANALYZE users",
		"REINDEX TABLE orders",
		"ANALYZE products",
		"CREATE INDEX idx_users_email ON users(email)",
	}
	
	for _, query := range maintenanceQueries {
		metrics := createQueryMetrics("testdb", query, 5*time.Second)
		err = processor.ConsumeMetrics(context.Background(), metrics)
		require.NoError(t, err)
		
		// Check that maintenance indicator was added
		allMetrics := consumer.AllMetrics()
		lastMetric := allMetrics[len(allMetrics)-1]
		
		metric := lastMetric.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		dp := metric.Histogram().DataPoints().At(0)
		
		isMaintenance, exists := dp.Attributes().Get("query.is_maintenance")
		assert.True(t, exists)
		assert.True(t, isMaintenance.Bool())
	}
}

// Helper functions

func createTableMetrics(dbName, tableName string, rowCount int64) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("db.system", "postgresql")
	rm.Resource().Attributes().PutStr("db.name", dbName)
	
	sm := rm.ScopeMetrics().AppendEmpty()
	
	// Table size metric
	sizeMetric := sm.Metrics().AppendEmpty()
	sizeMetric.SetName("db.table.size")
	sizeMetric.SetEmptyGauge()
	dp := sizeMetric.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(rowCount * 100) // Approximate size
	dp.Attributes().PutStr("table.name", tableName)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	// Row count metric
	rowMetric := sm.Metrics().AppendEmpty()
	rowMetric.SetName("db.table.row_count")
	rowMetric.SetEmptyGauge()
	dp2 := rowMetric.Gauge().DataPoints().AppendEmpty()
	dp2.SetIntValue(rowCount)
	dp2.Attributes().PutStr("table.name", tableName)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	
	return metrics
}

func createQueryMetrics(dbName, queryText string, duration time.Duration) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("db.system", "postgresql")
	rm.Resource().Attributes().PutStr("db.name", dbName)
	
	sm := rm.ScopeMetrics().AppendEmpty()
	
	// Query duration metric
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("db.query.duration")
	metric.SetEmptyHistogram()
	metric.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	
	dp := metric.Histogram().DataPoints().AppendEmpty()
	dp.SetCount(1)
	dp.SetSum(duration.Seconds() * 1000) // Convert to milliseconds
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.Attributes().PutStr("query.text", queryText)
	dp.Attributes().PutStr("db.operation", "SELECT")
	
	// Add bucket counts for histogram
	dp.BucketCounts().FromRaw([]uint64{0, 0, 1, 0, 0})
	dp.ExplicitBounds().FromRaw([]float64{10, 50, 100, 500})
	
	return metrics
}