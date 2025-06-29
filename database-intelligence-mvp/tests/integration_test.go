package tests

import (
    "context"
    "database/sql"
    "fmt"
    "strconv"
    "strings"
    "testing"
    "time"
    
    _ "github.com/lib/pq"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

// TestPostgreSQLIntegration tests the full PostgreSQL monitoring flow
func TestPostgreSQLIntegration(t *testing.T) {
    ctx := context.Background()
    
    // Start PostgreSQL container
    pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image:        "postgres:15-alpine",
            ExposedPorts: []string{"5432/tcp"},
            Env: map[string]string{
                "POSTGRES_PASSWORD": "test",
                "POSTGRES_DB":       "testdb",
            },
            WaitingFor: wait.ForListeningPort("5432/tcp"),
        },
        Started: true,
    })
    require.NoError(t, err)
    defer pgContainer.Terminate(ctx)
    
    // Get connection details
    host, err := pgContainer.Host(ctx)
    require.NoError(t, err)
    
    port, err := pgContainer.MappedPort(ctx, "5432")
    require.NoError(t, err)
    
    // Connect to PostgreSQL
    dsn := fmt.Sprintf("postgres://postgres:test@%s:%s/testdb?sslmode=disable", host, port.Port())
    db, err := sql.Open("postgres", dsn)
    require.NoError(t, err)
    defer db.Close()
    
    // Enable pg_stat_statements
    _, err = db.Exec("CREATE EXTENSION IF NOT EXISTS pg_stat_statements")
    require.NoError(t, err)
    
    // Create test data
    _, err = db.Exec(`
        CREATE TABLE test_table (
            id SERIAL PRIMARY KEY,
            data TEXT
        )
    `)
    require.NoError(t, err)
    
    // Generate some queries
    for i := 0; i < 100; i++ {
        _, err = db.Exec("INSERT INTO test_table (data) VALUES ($1)", fmt.Sprintf("test-%d", i))
        require.NoError(t, err)
    }
    
    // Start collector
    collectorCfg := createTestCollectorConfig(dsn)
    collector := startTestCollector(t, collectorCfg)
    defer collector.Stop()
    
    // Wait for metrics
    time.Sleep(10 * time.Second)
    
    // Verify metrics were collected
    metrics := collector.GetCollectedMetrics()
    
    // Check basic PostgreSQL metrics
    assert.True(t, hasMetric(metrics, "postgresql.commits"))
    assert.True(t, hasMetric(metrics, "postgresql.blocks_read"))
    assert.True(t, hasMetric(metrics, "postgresql.connection.count"))
    
    // Check query metrics
    assert.True(t, hasMetric(metrics, "db.query.count"))
    assert.True(t, hasMetric(metrics, "db.query.mean_duration"))
    
    // Verify dimensions
    queryMetrics := getMetricsByName(metrics, "db.query.count")
    for _, m := range queryMetrics {
        attrs := m.Attributes()
        assert.True(t, attrs.Has("database_name"))
        assert.True(t, attrs.Has("statement_type"))
    }
}

// TestAdaptiveSampling tests the adaptive sampling algorithm
func TestAdaptiveSampling(t *testing.T) {
    // Create test queries with different characteristics
    testCases := []struct {
        name           string
        queryMetrics   QueryMetrics
        expectedRate   float64
        tolerance      float64
    }{
        {
            name: "High cost query",
            queryMetrics: QueryMetrics{
                Duration: 1500.0, // 1.5 seconds
                HasError: false,
            },
            expectedRate: 1.0, // 100% sampling
            tolerance:    0.0,
        },
        {
            name: "Error query",
            queryMetrics: QueryMetrics{
                Duration: 100.0,
                HasError: true,
            },
            expectedRate: 1.0, // Always sample errors
            tolerance:    0.0,
        },
        {
            name: "Low cost query",
            queryMetrics: QueryMetrics{
                Duration: 10.0,
                HasError: false,
            },
            expectedRate: 0.1, // 10% sampling
            tolerance:    0.05,
        },
    }
    
    algorithm := NewTestAdaptiveAlgorithm()
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            rate := algorithm.CalculateSampleRate(context.Background(), "test-query", tc.queryMetrics)
            assert.InDelta(t, tc.expectedRate, rate, tc.tolerance)
        })
    }
}

// TestCircuitBreaker tests circuit breaker functionality
func TestCircuitBreaker(t *testing.T) {
    cb := NewTestCircuitBreaker()
    ctx := context.Background()
    
    // Test normal operation (closed state)
    assert.False(t, cb.ShouldBlock(ctx, "testdb"))
    
    // Simulate failures
    for i := 0; i < 10; i++ {
        cb.RecordFailure(ctx, "testdb")
    }
    
    // Should transition to open
    assert.True(t, cb.ShouldBlock(ctx, "testdb"))
    
    // Wait for timeout
    time.Sleep(2 * time.Second)
    
    // Should allow half-open
    assert.False(t, cb.ShouldBlock(ctx, "testdb"))
    
    // Record success
    cb.RecordSuccess(ctx, "testdb")
    
    // Should transition back to closed
    assert.False(t, cb.ShouldBlock(ctx, "testdb"))
}

// TestQueryPlanCollection tests query plan collection
func TestQueryPlanCollection(t *testing.T) {
    db := setupTestDatabase(t)
    defer db.Close()
    
    // Create a query that will show up in pg_stat_statements
    _, err := db.Exec("SELECT * FROM pg_class WHERE relname = 'test'")
    require.NoError(t, err)
    
    collector := NewQueryPlanCollector(db, testLogger(), testConfig())
    plans, err := collector.CollectQueryPlans(context.Background())
    require.NoError(t, err)
    
    // Should have collected at least one plan
    assert.NotEmpty(t, plans)
    
    // Verify plan structure
    for _, plan := range plans {
        assert.NotEmpty(t, plan.QueryID)
        assert.NotEmpty(t, plan.QueryText)
        assert.NotEmpty(t, plan.Plan)
        assert.Greater(t, plan.MeanExecTime, 0.0)
    }
}

// TestHighAvailability tests HA configuration with Redis
func TestHighAvailability(t *testing.T) {
    ctx := context.Background()
    
    // Start Redis container
    redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: testcontainers.ContainerRequest{
            Image:        "redis:7-alpine",
            ExposedPorts: []string{"6379/tcp"},
            WaitingFor:   wait.ForListeningPort("6379/tcp"),
        },
        Started: true,
    })
    require.NoError(t, err)
    defer redisContainer.Terminate(ctx)
    
    // Start two collector instances
    collector1 := startCollectorWithRedis(t, redisContainer, "collector-1")
    collector2 := startCollectorWithRedis(t, redisContainer, "collector-2")
    
    defer collector1.Stop()
    defer collector2.Stop()
    
    // Both should be running
    assert.True(t, collector1.IsHealthy())
    assert.True(t, collector2.IsHealthy())
    
    // Simulate workload
    generateTestWorkload(t)
    
    // Stop collector1
    collector1.Stop()
    
    // Collector2 should still be collecting
    time.Sleep(5 * time.Second)
    metrics := collector2.GetCollectedMetrics()
    assert.NotEmpty(t, metrics)
}

// Helper functions

func hasMetric(metrics []Metric, name string) bool {
    for _, m := range metrics {
        if m.Name() == name {
            return true
        }
    }
    return false
}

func getMetricsByName(metrics []Metric, name string) []Metric {
    var result []Metric
    for _, m := range metrics {
        if m.Name() == name {
            result = append(result, m)
        }
    }
    return result
}

func createTestCollectorConfig(dsn string) *CollectorConfig {
    return &CollectorConfig{
        DSN:                dsn,
        CollectionInterval: 5 * time.Second,
        Exporters:          []string{"memory"}, // Export to memory for testing
    }
}

func setupTestDatabase(t *testing.T) *sql.DB {
    // Setup code for test database
    return nil
}

func generateTestWorkload(t *testing.T) {
    // Generate database workload for testing
}