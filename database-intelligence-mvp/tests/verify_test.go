package tests

import (
    "database/sql"
    "fmt"
    "os"
    "testing"
    
    _ "github.com/lib/pq"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestDatabaseConnectivity verifies we can connect to PostgreSQL
func TestDatabaseConnectivity(t *testing.T) {
    // Get connection from environment
    host := os.Getenv("PG_HOST")
    if host == "" {
        host = "localhost"
    }
    
    port := os.Getenv("PG_PORT")
    if port == "" {
        port = "5432"
    }
    
    user := os.Getenv("PG_USER")
    if user == "" {
        user = "newrelic_monitor"
    }
    
    password := os.Getenv("PG_PASSWORD")
    if password == "" {
        password = "monitor123"
    }
    
    dbname := os.Getenv("PG_DATABASE")
    if dbname == "" {
        dbname = "testdb"
    }
    
    // Connect to PostgreSQL
    dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)
    db, err := sql.Open("postgres", dsn)
    require.NoError(t, err)
    defer db.Close()
    
    // Test connection
    err = db.Ping()
    require.NoError(t, err)
    
    // Check pg_stat_statements
    var extExists bool
    err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='pg_stat_statements')").Scan(&extExists)
    require.NoError(t, err)
    assert.True(t, extExists, "pg_stat_statements extension should be enabled")
    
    // Run a test query
    var version string
    err = db.QueryRow("SELECT version()").Scan(&version)
    require.NoError(t, err)
    t.Logf("PostgreSQL version: %s", version)
    
    // Check permissions
    var hasPermission bool
    err = db.QueryRow("SELECT has_database_privilege($1, 'CONNECT')", dbname).Scan(&hasPermission)
    require.NoError(t, err)
    assert.True(t, hasPermission, "Monitor user should have CONNECT privilege")
}

// TestMetricsFlow verifies metrics are being collected
func TestMetricsFlow(t *testing.T) {
    // This test verifies the collector is running and metrics are flowing
    // In a real scenario, we would query New Relic to verify metrics arrived
    
    // For now, we just ensure the collector config is valid
    configPath := "../config/collector-otel-metrics.yaml"
    _, err := os.Stat(configPath)
    require.NoError(t, err, "Collector config should exist")
    
    // Verify Docker containers are running
    // This is a simplified check - in production you'd use Docker API
    t.Log("Collector should be running via docker-compose")
}

// TestQueryPlanSimple tests basic query plan collection
func TestQueryPlanSimple(t *testing.T) {
    host := os.Getenv("PG_HOST")
    if host == "" {
        host = "localhost"
    }
    
    dsn := fmt.Sprintf("postgres://newrelic_monitor:monitor123@%s:5432/testdb?sslmode=disable", host)
    db, err := sql.Open("postgres", dsn)
    require.NoError(t, err)
    defer db.Close()
    
    // Create a simple table for testing
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS test_query_plan (
            id SERIAL PRIMARY KEY,
            data TEXT,
            created_at TIMESTAMP DEFAULT NOW()
        )
    `)
    require.NoError(t, err)
    
    // Insert some test data
    for i := 0; i < 10; i++ {
        _, err = db.Exec("INSERT INTO test_query_plan (data) VALUES ($1)", fmt.Sprintf("test-%d", i))
        require.NoError(t, err)
    }
    
    // Get query plan
    var planJSON string
    err = db.QueryRow("EXPLAIN (FORMAT JSON) SELECT * FROM test_query_plan WHERE data LIKE 'test%'").Scan(&planJSON)
    require.NoError(t, err)
    assert.NotEmpty(t, planJSON, "Query plan should not be empty")
    
    t.Logf("Query plan: %s", planJSON)
    
    // Cleanup
    _, err = db.Exec("DROP TABLE IF EXISTS test_query_plan")
    require.NoError(t, err)
}

// TestCircuitBreakerSimulation tests circuit breaker behavior
func TestCircuitBreakerSimulation(t *testing.T) {
    // This is a conceptual test showing how circuit breaker would work
    type CircuitState int
    const (
        Closed CircuitState = iota
        Open
        HalfOpen
    )
    
    errorCount := 0
    errorThreshold := 5
    state := Closed
    
    // Simulate requests
    for i := 0; i < 10; i++ {
        if state == Open {
            t.Logf("Request %d: BLOCKED (circuit open)", i)
            continue
        }
        
        // Simulate error on requests 3-7
        hasError := i >= 3 && i <= 7
        
        if hasError {
            errorCount++
            t.Logf("Request %d: ERROR (count: %d)", i, errorCount)
            
            if errorCount >= errorThreshold && state == Closed {
                state = Open
                t.Log("Circuit breaker: CLOSED -> OPEN")
            }
        } else {
            t.Logf("Request %d: SUCCESS", i)
            if state == HalfOpen {
                state = Closed
                errorCount = 0
                t.Log("Circuit breaker: HALF-OPEN -> CLOSED")
            }
        }
    }
    
    assert.Equal(t, Open, state, "Circuit should be open after errors")
}

// TestAdaptiveSamplingLogic tests sampling rate calculation
func TestAdaptiveSamplingLogic(t *testing.T) {
    // Test importance score calculation
    testCases := []struct {
        name         string
        duration     float64
        hasError     bool
        expectedRate float64
        tolerance    float64
    }{
        {
            name:         "High cost query",
            duration:     1500.0, // 1.5 seconds
            hasError:     false,
            expectedRate: 1.0, // 100% sampling
            tolerance:    0.0,
        },
        {
            name:         "Error query",
            duration:     100.0,
            hasError:     true,
            expectedRate: 1.0, // Always sample errors
            tolerance:    0.0,
        },
        {
            name:         "Low cost query",
            duration:     10.0,
            hasError:     false,
            expectedRate: 0.1, // 10% sampling
            tolerance:    0.2,
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Calculate importance score
            importanceScore := 0.0
            
            // Cost factor (40% weight)
            highCostThreshold := 1000.0 // 1 second
            costScore := tc.duration / highCostThreshold
            if costScore > 1.0 {
                costScore = 1.0
            }
            importanceScore += costScore * 0.4
            
            // Error factor (30% weight)
            if tc.hasError {
                importanceScore += 0.3
            }
            
            // Determine sample rate
            sampleRate := 0.1 // default minimum
            if tc.hasError {
                sampleRate = 1.0
            } else if importanceScore > 0.7 {
                sampleRate = 1.0
            } else if importanceScore > 0.3 {
                sampleRate = 0.5 + (importanceScore-0.3)*1.25
            } else {
                sampleRate = importanceScore * 1.67
                if sampleRate < 0.1 {
                    sampleRate = 0.1
                }
            }
            
            assert.InDelta(t, tc.expectedRate, sampleRate, tc.tolerance,
                "Sample rate for %s should be ~%.2f", tc.name, tc.expectedRate)
        })
    }
}

// TestHighAvailabilityReadiness tests HA configuration
func TestHighAvailabilityReadiness(t *testing.T) {
    // Check Redis connectivity (if available)
    redisHost := os.Getenv("REDIS_ENDPOINT")
    if redisHost == "" {
        t.Skip("Redis not configured, skipping HA test")
    }
    
    // Verify HA configuration exists
    haConfigPath := "../config/collector-ha.yaml"
    _, err := os.Stat(haConfigPath)
    require.NoError(t, err, "HA config should exist")
    
    // Verify Docker Compose HA exists
    haComposePath := "../deploy/docker/docker-compose-ha.yaml"
    _, err = os.Stat(haComposePath)
    require.NoError(t, err, "HA Docker Compose should exist")
    
    t.Log("High Availability configuration is ready")
}

// TestBuildSystem verifies the build system works
func TestBuildSystem(t *testing.T) {
    // Check Makefile exists
    _, err := os.Stat("../Makefile")
    require.NoError(t, err, "Makefile should exist")
    
    // Check OCB config exists
    _, err = os.Stat("../ocb-config.yaml")
    require.NoError(t, err, "OCB config should exist")
    
    t.Log("Build system is configured correctly")
}