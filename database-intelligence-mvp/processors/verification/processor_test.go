// Copyright Database Intelligence MVP
// SPDX-License-Identifier: Apache-2.0

package verification

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestVerificationProcessor_Basic(t *testing.T) {
	// Create a basic configuration
	config := &Config{
		EnablePeriodicVerification: false, // Disable for testing
		DataFreshnessThreshold:     10 * time.Minute,
		MinEntityCorrelationRate:   0.8,
		MinNormalizationRate:       0.9,
		RequireEntitySynthesis:     true,
		ExportFeedbackAsLogs:       false,
		
		// Disable advanced features for basic test
		EnableContinuousHealthChecks: false,
		EnableAutoTuning:            false,
		EnableSelfHealing:           false,
		
		QualityRules: QualityRulesConfig{
			RequiredFields: []string{"database_name", "query_id"},
		},
		PIIDetection: PIIDetectionConfig{
			Enabled: false, // Disable for basic test
		},
	}

	// Create test logger
	logger := zap.NewNop()

	// Create mock consumer
	mockConsumer := &consumertest.LogsSink{}

	// Create processor
	processor, err := newVerificationProcessor(logger, config, mockConsumer)
	require.NoError(t, err)
	require.NotNil(t, processor)

	// Test basic log processing
	ctx := context.Background()
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	// Add required fields
	lr.Attributes().PutStr("database_name", "test_db")
	lr.Attributes().PutStr("query_id", "12345")
	lr.Body().SetStr("SELECT * FROM users WHERE id = 1")

	// Process logs
	err = processor.ConsumeLogs(ctx, logs)
	assert.NoError(t, err)

	// Verify that logs were passed through
	assert.Equal(t, 1, len(mockConsumer.AllLogs()))

	// Shutdown processor
	err = processor.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestVerificationProcessor_QualityValidation(t *testing.T) {
	config := &Config{
		EnablePeriodicVerification:   false,
		EnableContinuousHealthChecks: false,
		EnableAutoTuning:            false,
		EnableSelfHealing:           false,
		ExportFeedbackAsLogs:        false,
		
		QualityRules: QualityRulesConfig{
			RequiredFields:         []string{"database_name", "query_id", "duration_ms"},
			EnableSchemaValidation: true,
		},
		PIIDetection: PIIDetectionConfig{
			Enabled: false,
		},
	}

	logger := zap.NewNop()
	mockConsumer := &consumertest.LogsSink{}

	processor, err := newVerificationProcessor(logger, config, mockConsumer)
	require.NoError(t, err)

	ctx := context.Background()
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	// Add only some required fields (missing duration_ms)
	lr.Attributes().PutStr("database_name", "test_db")
	lr.Attributes().PutStr("query_id", "12345")

	err = processor.ConsumeLogs(ctx, logs)
	assert.NoError(t, err)

	// Check that quality validation detected missing field
	processor.qualityValidator.mu.RLock()
	assert.Greater(t, processor.qualityValidator.missingRequiredFields, int64(0))
	processor.qualityValidator.mu.RUnlock()

	err = processor.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestVerificationProcessor_PIIDetection(t *testing.T) {
	config := &Config{
		EnablePeriodicVerification:   false,
		EnableContinuousHealthChecks: false,
		EnableAutoTuning:            false,
		EnableSelfHealing:           false,
		ExportFeedbackAsLogs:        false,
		
		QualityRules: QualityRulesConfig{
			RequiredFields: []string{},
		},
		PIIDetection: PIIDetectionConfig{
			Enabled:          true,
			AutoSanitize:     true,
			SensitivityLevel: "medium",
		},
	}

	logger := zap.NewNop()
	mockConsumer := &consumertest.LogsSink{}

	processor, err := newVerificationProcessor(logger, config, mockConsumer)
	require.NoError(t, err)

	ctx := context.Background()
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()

	// Add log with potential PII
	lr.Body().SetStr("User email: john.doe@example.com called query")
	lr.Attributes().PutStr("user_email", "jane.doe@example.com")

	err = processor.ConsumeLogs(ctx, logs)
	assert.NoError(t, err)

	// Check that PII was detected and sanitized
	processor.piiDetector.mu.RLock()
	assert.Greater(t, processor.piiDetector.violations, int64(0))
	assert.Greater(t, processor.piiDetector.sanitizedFields, int64(0))
	processor.piiDetector.mu.RUnlock()

	// Verify sanitization occurred
	processedLogs := mockConsumer.AllLogs()[0]
	processedLR := processedLogs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	assert.Contains(t, processedLR.Body().Str(), "[REDACTED]")

	err = processor.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestVerificationProcessor_HealthCheck(t *testing.T) {
	config := &Config{
		EnablePeriodicVerification:   false,
		EnableContinuousHealthChecks: true,
		HealthCheckInterval:         100 * time.Millisecond, // Fast for testing
		EnableAutoTuning:            false,
		EnableSelfHealing:           false,
		ExportFeedbackAsLogs:        false,
		
		HealthThresholds: HealthThresholdsConfig{
			MemoryPercent: 90.0,
			CPUPercent:    90.0,
			DiskPercent:   95.0,
		},
		QualityRules: QualityRulesConfig{},
		PIIDetection: PIIDetectionConfig{Enabled: false},
	}

	logger := zap.NewNop()
	mockConsumer := &consumertest.LogsSink{}

	processor, err := newVerificationProcessor(logger, config, mockConsumer)
	require.NoError(t, err)

	// Start the processor to begin health checks
	err = processor.Start(context.Background(), nil)
	assert.NoError(t, err)

	// Wait for at least one health check cycle
	time.Sleep(200 * time.Millisecond)

	// Verify health check ran
	processor.healthChecker.mu.RLock()
	assert.False(t, processor.healthChecker.lastHealthCheck.IsZero())
	processor.healthChecker.mu.RUnlock()

	err = processor.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestVerificationProcessor_AutoTuning(t *testing.T) {
	config := &Config{
		EnablePeriodicVerification:   false,
		EnableContinuousHealthChecks: false,
		EnableAutoTuning:            true,
		AutoTuningInterval:          100 * time.Millisecond, // Fast for testing
		EnableSelfHealing:           false,
		ExportFeedbackAsLogs:        false,
		
		AutoTuningConfig: AutoTuningConfig{
			EnableAutoApply:    false,
			MinConfidenceLevel: 0.5,
		},
		QualityRules: QualityRulesConfig{},
		PIIDetection: PIIDetectionConfig{Enabled: false},
	}

	logger := zap.NewNop()
	mockConsumer := &consumertest.LogsSink{}

	processor, err := newVerificationProcessor(logger, config, mockConsumer)
	require.NoError(t, err)

	err = processor.Start(context.Background(), nil)
	assert.NoError(t, err)

	// Wait for auto-tuning cycle
	time.Sleep(200 * time.Millisecond)

	// Verify auto-tuning engine is working
	processor.feedbackEngine.mu.RLock()
	assert.True(t, processor.feedbackEngine.autoTuningEnabled)
	processor.feedbackEngine.mu.RUnlock()

	err = processor.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestVerificationProcessor_SelfHealing(t *testing.T) {
	config := &Config{
		EnablePeriodicVerification:   false,
		EnableContinuousHealthChecks: false,
		EnableAutoTuning:            false,
		EnableSelfHealing:           true,
		SelfHealingInterval:         100 * time.Millisecond, // Fast for testing
		ExportFeedbackAsLogs:        false,
		
		SelfHealingConfig: SelfHealingConfig{
			MaxRetries:        2,
			BackoffMultiplier: 2.0,
			EnabledIssueTypes: []string{"consumer_error", "high_memory"},
		},
		QualityRules: QualityRulesConfig{},
		PIIDetection: PIIDetectionConfig{Enabled: false},
	}

	logger := zap.NewNop()
	mockConsumer := &consumertest.LogsSink{}

	processor, err := newVerificationProcessor(logger, config, mockConsumer)
	require.NoError(t, err)

	err = processor.Start(context.Background(), nil)
	assert.NoError(t, err)

	// Simulate a healing scenario
	processor.attemptSelfHealing("high_memory", assert.AnError, nil)

	// Wait for self-healing cycle
	time.Sleep(200 * time.Millisecond)

	// Verify self-healing is working
	processor.selfHealer.mu.RLock()
	assert.True(t, processor.selfHealer.healingEnabled)
	assert.Greater(t, len(processor.selfHealer.healingHistory), 0)
	processor.selfHealer.mu.RUnlock()

	err = processor.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid_config",
			config: &Config{
				EnablePeriodicVerification: true,
				VerificationInterval:       5 * time.Minute,
				MinEntityCorrelationRate:   0.8,
				MinNormalizationRate:       0.9,
				EnableContinuousHealthChecks: true,
				HealthCheckInterval:         30 * time.Second,
				HealthThresholds: HealthThresholdsConfig{
					MemoryPercent: 85.0,
					CPUPercent:    80.0,
					DiskPercent:   90.0,
				},
				EnableAutoTuning:   true,
				AutoTuningInterval: 10 * time.Minute,
				AutoTuningConfig: AutoTuningConfig{
					MinConfidenceLevel: 0.8,
					MaxParameterChange: 0.2,
				},
				EnableSelfHealing:   true,
				SelfHealingInterval: 1 * time.Minute,
				SelfHealingConfig: SelfHealingConfig{
					MaxRetries:        3,
					BackoffMultiplier: 2.0,
				},
				PIIDetection: PIIDetectionConfig{
					Enabled:          true,
					SensitivityLevel: "medium",
				},
			},
			expectError: false,
		},
		{
			name: "invalid_memory_threshold",
			config: &Config{
				EnableContinuousHealthChecks: true,
				HealthCheckInterval:          30 * time.Second,
				HealthThresholds: HealthThresholdsConfig{
					MemoryPercent: 150.0, // Invalid: > 100
				},
			},
			expectError: true,
		},
		{
			name: "invalid_confidence_level",
			config: &Config{
				EnableAutoTuning:   true,
				AutoTuningInterval: 10 * time.Minute,
				AutoTuningConfig: AutoTuningConfig{
					MinConfidenceLevel: 1.5, // Invalid: > 1.0
				},
			},
			expectError: true,
		},
		{
			name: "invalid_backoff_multiplier",
			config: &Config{
				EnableSelfHealing:   true,
				SelfHealingInterval: 1 * time.Minute,
				SelfHealingConfig: SelfHealingConfig{
					BackoffMultiplier: 0.5, // Invalid: <= 1.0
				},
			},
			expectError: true,
		},
		{
			name: "invalid_pii_sensitivity",
			config: &Config{
				PIIDetection: PIIDetectionConfig{
					Enabled:          true,
					SensitivityLevel: "invalid", // Invalid level
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInitializePIIPatterns(t *testing.T) {
	patterns := initializePIIPatterns()
	assert.Greater(t, len(patterns), 0)

	// Test email pattern
	emailPattern := patterns[0]
	assert.True(t, emailPattern.MatchString("test@example.com"))
	assert.False(t, emailPattern.MatchString("not-an-email"))
}