package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"
	
	"github.com/database-intelligence-mvp/common/featuredetector"
	"github.com/database-intelligence-mvp/common/queryselector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"go.uber.org/zap"
	
	_ "github.com/lib/pq"
	_ "github.com/go-sql-driver/mysql"
)

// TestPostgreSQLFeatureDetection tests feature detection for various PostgreSQL configurations
func TestPostgreSQLFeatureDetection(t *testing.T) {
	tests := []struct {
		name       string
		setupSQL   []string
		expected   map[string]bool
		skipReason string
	}{
		{
			name: "minimal_postgres",
			setupSQL: []string{
				// No extensions, just base PostgreSQL
			},
			expected: map[string]bool{
				featuredetector.ExtPgStatStatements: false,
				featuredetector.ExtPgStatMonitor:    false,
				featuredetector.ExtPgWaitSampling:   false,
				featuredetector.CapTrackIOTiming:    false,
			},
		},
		{
			name: "postgres_with_pg_stat_statements",
			setupSQL: []string{
				"CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
			},
			expected: map[string]bool{
				featuredetector.ExtPgStatStatements: true,
				featuredetector.ExtPgStatMonitor:    false,
				featuredetector.CapTrackIOTiming:    false,
			},
		},
		{
			name: "postgres_with_io_timing",
			setupSQL: []string{
				"CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
				"ALTER SYSTEM SET track_io_timing = on",
				"SELECT pg_reload_conf()",
			},
			expected: map[string]bool{
				featuredetector.ExtPgStatStatements: true,
				featuredetector.CapTrackIOTiming:    true,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}
			
			ctx := context.Background()
			
			// Start PostgreSQL container
			pgContainer, err := postgres.RunContainer(ctx,
				testcontainers.WithImage("postgres:15-alpine"),
				postgres.WithDatabase("testdb"),
				postgres.WithUsername("testuser"),
				postgres.WithPassword("testpass"),
				testcontainers.WithWaitStrategy(
					testcontainers.ForLog("database system is ready to accept connections").
						WithOccurrence(2).
						WithStartupTimeout(30*time.Second),
				),
			)
			require.NoError(t, err)
			defer pgContainer.Terminate(ctx)
			
			// Get connection string
			connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
			require.NoError(t, err)
			
			// Connect to database
			db, err := sql.Open("postgres", connStr)
			require.NoError(t, err)
			defer db.Close()
			
			// Setup database
			for _, query := range tt.setupSQL {
				_, err := db.Exec(query)
				if err != nil {
					t.Logf("Setup query failed (may be expected): %v", err)
				}
			}
			
			// Create feature detector
			logger := zap.NewNop()
			config := featuredetector.DetectionConfig{
				CacheDuration:   5 * time.Minute,
				TimeoutPerCheck: 3 * time.Second,
			}
			detector := featuredetector.NewPostgreSQLDetector(db, logger, config)
			
			// Detect features
			features, err := detector.DetectFeatures(ctx)
			require.NoError(t, err)
			
			// Verify expected features
			for feature, expectedAvailable := range tt.expected {
				actual := features.HasFeature(feature)
				assert.Equal(t, expectedAvailable, actual, 
					"Feature %s: expected %v, got %v", feature, expectedAvailable, actual)
			}
			
			// Log detected features for debugging
			t.Logf("Detected features: %+v", features)
		})
	}
}

// TestMySQLFeatureDetection tests feature detection for MySQL configurations
func TestMySQLFeatureDetection(t *testing.T) {
	tests := []struct {
		name       string
		setupSQL   []string
		expected   map[string]bool
		skipReason string
	}{
		{
			name: "mysql_with_performance_schema",
			setupSQL: []string{
				// Performance schema is usually enabled by default
			},
			expected: map[string]bool{
				featuredetector.CapPerfSchemaEnabled:          true,
				featuredetector.CapPerfSchemaStatementsDigest: true,
				featuredetector.CapSlowQueryLog:               false,
			},
		},
		{
			name: "mysql_with_slow_query_log",
			setupSQL: []string{
				"SET GLOBAL slow_query_log = 1",
				"SET GLOBAL long_query_time = 1",
			},
			expected: map[string]bool{
				featuredetector.CapPerfSchemaEnabled: true,
				featuredetector.CapSlowQueryLog:      true,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipReason != "" {
				t.Skip(tt.skipReason)
			}
			
			ctx := context.Background()
			
			// Start MySQL container
			mysqlContainer, err := mysql.RunContainer(ctx,
				testcontainers.WithImage("mysql:8.0"),
				mysql.WithDatabase("testdb"),
				mysql.WithUsername("root"),
				mysql.WithPassword("testpass"),
			)
			require.NoError(t, err)
			defer mysqlContainer.Terminate(ctx)
			
			// Get connection string
			host, err := mysqlContainer.Host(ctx)
			require.NoError(t, err)
			
			port, err := mysqlContainer.MappedPort(ctx, "3306")
			require.NoError(t, err)
			
			dsn := "root:testpass@tcp(" + host + ":" + port.Port() + ")/testdb"
			
			// Connect to database
			db, err := sql.Open("mysql", dsn)
			require.NoError(t, err)
			defer db.Close()
			
			// Wait for MySQL to be ready
			for i := 0; i < 30; i++ {
				if err := db.Ping(); err == nil {
					break
				}
				time.Sleep(time.Second)
			}
			
			// Setup database
			for _, query := range tt.setupSQL {
				_, err := db.Exec(query)
				if err != nil {
					t.Logf("Setup query failed (may be expected): %v", err)
				}
			}
			
			// Create feature detector
			logger := zap.NewNop()
			config := featuredetector.DetectionConfig{
				CacheDuration:   5 * time.Minute,
				TimeoutPerCheck: 3 * time.Second,
			}
			detector := featuredetector.NewMySQLDetector(db, logger, config)
			
			// Detect features
			features, err := detector.DetectFeatures(ctx)
			require.NoError(t, err)
			
			// Verify expected features
			for feature, expectedAvailable := range tt.expected {
				actual := features.HasFeature(feature)
				assert.Equal(t, expectedAvailable, actual,
					"Feature %s: expected %v, got %v", feature, expectedAvailable, actual)
			}
			
			// Log detected features for debugging
			t.Logf("Detected features: %+v", features)
		})
	}
}

// TestQuerySelection tests query selection based on features
func TestQuerySelection(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	
	// Create mock feature set
	features := &featuredetector.FeatureSet{
		DatabaseType:  featuredetector.DatabaseTypePostgreSQL,
		ServerVersion: "PostgreSQL 15.0",
		Extensions: map[string]*featuredetector.Feature{
			featuredetector.ExtPgStatStatements: {
				Name:      featuredetector.ExtPgStatStatements,
				Available: true,
				Version:   "1.10",
			},
			featuredetector.ExtPgStatMonitor: {
				Name:      featuredetector.ExtPgStatMonitor,
				Available: false,
			},
		},
		Capabilities: map[string]*featuredetector.Feature{
			featuredetector.CapTrackIOTiming: {
				Name:      featuredetector.CapTrackIOTiming,
				Available: true,
				Version:   "on",
			},
		},
		LastDetection: time.Now(),
	}
	
	// Create mock detector
	mockDetector := &mockFeatureDetector{features: features}
	
	// Create query selector
	selector := queryselector.NewQuerySelector(mockDetector, logger, queryselector.Config{
		CacheDuration: 5 * time.Minute,
	})
	
	// Test slow query selection
	query, err := selector.GetQuery(ctx, queryselector.CategorySlowQueries)
	require.NoError(t, err)
	require.NotNil(t, query)
	
	// Should select pg_stat_statements with I/O timing since pg_stat_monitor is not available
	assert.Equal(t, "pg_stat_statements_io_timing", query.Name)
	assert.Equal(t, 90, query.Priority)
}

// TestCircuitBreakerWithFeatureErrors tests circuit breaker handling of feature-related errors
func TestCircuitBreakerWithFeatureErrors(t *testing.T) {
	// This would test the circuit breaker's ability to handle errors like:
	// - "relation pg_stat_statements does not exist"
	// - "permission denied"
	// - "extension not installed"
	// And verify it takes appropriate actions (disable query, use fallback, etc.)
	
	// Implementation would require setting up the circuit breaker processor
	// and simulating various database error conditions
}

// mockFeatureDetector implements a mock feature detector for testing
type mockFeatureDetector struct {
	features *featuredetector.FeatureSet
}

func (m *mockFeatureDetector) DetectFeatures(ctx context.Context) (*featuredetector.FeatureSet, error) {
	return m.features, nil
}

func (m *mockFeatureDetector) GetCachedFeatures() *featuredetector.FeatureSet {
	return m.features
}

func (m *mockFeatureDetector) ValidateQuery(query *featuredetector.QueryDefinition) error {
	// Simple validation based on requirements
	for _, ext := range query.Requirements.RequiredExtensions {
		if !m.features.HasFeature(ext) {
			return &featuredetector.MissingFeatureError{
				FeatureType: "extension",
				FeatureName: ext,
			}
		}
	}
	
	for _, cap := range query.Requirements.RequiredCapabilities {
		if !m.features.HasFeature(cap) {
			return &featuredetector.MissingFeatureError{
				FeatureType: "capability",
				FeatureName: cap,
			}
		}
	}
	
	return nil
}

func (m *mockFeatureDetector) SelectBestQuery(queries []featuredetector.QueryDefinition) (*featuredetector.QueryDefinition, error) {
	// Not used in these tests
	return nil, nil
}