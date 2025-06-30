package planattributeextractor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
)

func TestNewPlanAttributeExtractor(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := consumertest.NewNop()

	processor := newPlanAttributeExtractor(cfg, logger, consumer)
	
	assert.NotNil(t, processor)
	assert.NotNil(t, processor.config)
	assert.NotNil(t, processor.logger)
	assert.NotNil(t, processor.consumer)
	assert.NotNil(t, processor.queryAnonymizer)
}

func TestPlanAttributeExtractor_QueryAnonymization(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.QueryAnonymization.Enabled = true
	cfg.QueryAnonymization.AttributesToAnonymize = []string{"query_text", "db.statement"}
	cfg.QueryAnonymization.GenerateFingerprint = true
	cfg.QueryAnonymization.FingerprintAttribute = "db.query.fingerprint"

	logger := zap.NewNop()
	consumer := consumertest.NewNop()
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	// Create test log record with query text
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	
	// Add query text attributes
	lr.Attributes().PutStr("query_text", "SELECT * FROM users WHERE id = 123 AND email = 'user@example.com'")
	lr.Attributes().PutStr("db.statement", "INSERT INTO logs (ip) VALUES ('192.168.1.100')")
	
	// Process the logs
	ctx := context.Background()
	err := processor.ConsumeLogs(ctx, logs)
	require.NoError(t, err)
	
	// Verify anonymization
	processedLogs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	
	// Check query_text was anonymized
	queryText, exists := processedLogs.Attributes().Get("query_text")
	assert.True(t, exists)
	assert.Equal(t, "SELECT * FROM users WHERE id = ? AND email = ?", queryText.Str())
	
	// Check db.statement was anonymized
	dbStatement, exists := processedLogs.Attributes().Get("db.statement")
	assert.True(t, exists)
	assert.Equal(t, "INSERT INTO logs (ip) VALUES (?)", dbStatement.Str())
	
	// Check fingerprint was generated
	fingerprint, exists := processedLogs.Attributes().Get("db.query.fingerprint")
	assert.True(t, exists)
	assert.NotEmpty(t, fingerprint.Str())
}

func TestPlanAttributeExtractor_AnonymizationDisabled(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.QueryAnonymization.Enabled = false

	logger := zap.NewNop()
	consumer := consumertest.NewNop()
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	// Create test log record with query text
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	
	originalQuery := "SELECT * FROM users WHERE id = 123"
	lr.Attributes().PutStr("query_text", originalQuery)
	
	// Process the logs
	ctx := context.Background()
	err := processor.ConsumeLogs(ctx, logs)
	require.NoError(t, err)
	
	// Verify query was NOT anonymized
	processedLogs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	queryText, exists := processedLogs.Attributes().Get("query_text")
	assert.True(t, exists)
	assert.Equal(t, originalQuery, queryText.Str())
}

func TestPlanAttributeExtractor_PostgreSQLPlanExtraction(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := consumertest.NewNop()
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	// Create test log record with PostgreSQL plan
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	
	// Add PostgreSQL plan JSON
	planJSON := `[{"Plan": {"Node Type": "Seq Scan", "Total Cost": 123.45, "Plan Rows": 1000, "Plan Width": 32}}]`
	lr.Attributes().PutStr("plan_json", planJSON)
	
	// Process the logs
	ctx := context.Background()
	err := processor.ConsumeLogs(ctx, logs)
	require.NoError(t, err)
	
	// Verify attributes were extracted
	processedLogs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	
	// Check extracted attributes
	cost, exists := processedLogs.Attributes().Get("db.query.plan.cost")
	assert.True(t, exists)
	assert.Equal(t, float64(123.45), cost.Double())
	
	rows, exists := processedLogs.Attributes().Get("db.query.plan.rows")
	assert.True(t, exists)
	assert.Equal(t, int64(1000), rows.Int())
	
	operation, exists := processedLogs.Attributes().Get("db.query.plan.operation")
	assert.True(t, exists)
	assert.Equal(t, "Seq Scan", operation.Str())
	
	// Check derived attributes
	hasSeqScan, exists := processedLogs.Attributes().Get("db.query.plan.has_seq_scan")
	assert.True(t, exists)
	assert.True(t, hasSeqScan.Bool())
}

func TestPlanAttributeExtractor_Timeout(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.TimeoutMS = 1 // Very short timeout
	logger := zap.NewNop()
	consumer := consumertest.NewNop()
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	// Create test log record
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	
	// Add large plan that will take time to process
	largeJSON := `[{"Plan": {"Node Type": "Seq Scan", "Total Cost": 123.45`
	for i := 0; i < 1000; i++ {
		largeJSON += `, "Field` + string(rune(i)) + `": "Value"`
	}
	largeJSON += `}}]`
	lr.Attributes().PutStr("plan_json", largeJSON)
	
	// Create context with immediate cancellation to simulate timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
	defer cancel()
	
	// Process should handle timeout gracefully
	err := processor.ConsumeLogs(ctx, logs)
	// Error propagation depends on error_mode configuration
	if cfg.ErrorMode == "propagate" {
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
	}
}

func TestPlanAttributeExtractor_ErrorModes(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     string
		expectError   bool
	}{
		{
			name:        "ignore mode",
			errorMode:   "ignore",
			expectError: false,
		},
		{
			name:        "propagate mode",
			errorMode:   "propagate",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.ErrorMode = tt.errorMode
			cfg.TimeoutMS = 1 // Force timeout
			
			logger := zap.NewNop()
			consumer := consumertest.NewNop()
			processor := newPlanAttributeExtractor(cfg, logger, consumer)

			// Create test log record
			logs := plog.NewLogs()
			rl := logs.ResourceLogs().AppendEmpty()
			sl := rl.ScopeLogs().AppendEmpty()
			lr := sl.LogRecords().AppendEmpty()
			lr.Attributes().PutStr("plan_json", `invalid json`)

			// Process with immediate timeout
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
			defer cancel()
			
			err := processor.ConsumeLogs(ctx, logs)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPlanAttributeExtractor_HashGeneration(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.HashConfig.Include = []string{"query_text", "db.query.plan.operation"}
	cfg.HashConfig.Output = "plan_hash"
	cfg.HashConfig.Algorithm = "sha256"
	
	logger := zap.NewNop()
	consumer := consumertest.NewNop()
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	// Create test log record
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	
	lr.Attributes().PutStr("query_text", "SELECT * FROM users")
	lr.Attributes().PutStr("db.query.plan.operation", "Seq Scan")
	
	// Process the logs
	ctx := context.Background()
	err := processor.ConsumeLogs(ctx, logs)
	require.NoError(t, err)
	
	// Verify hash was generated
	processedLogs := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	hash, exists := processedLogs.Attributes().Get("plan_hash")
	assert.True(t, exists)
	assert.NotEmpty(t, hash.Str())
	assert.Len(t, hash.Str(), 64) // SHA256 produces 64 hex characters
}

func TestPlanAttributeExtractor_StartShutdown(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	settings := processortest.NewNopSettings(component.MustNewType("test"))
	processor, err := createLogsProcessor(context.Background(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Test Start
	err = processor.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)

	// Test Shutdown
	err = processor.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestPlanAttributeExtractor_Capabilities(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	logger := zap.NewNop()
	consumer := consumertest.NewNop()
	processor := newPlanAttributeExtractor(cfg, logger, consumer)

	capabilities := processor.Capabilities()
	assert.True(t, capabilities.MutatesData)
}