# Immediate Integration Steps

## Quick Wins - Start Today

### 1. Activate Connection Pooling in SQL Receivers

**File**: `receivers/enhancedsql/receiver.go`
```go
import (
    "database-intelligence-restructured/core/internal/database"
)

// Add to receiver struct
type enhancedSQLReceiver struct {
    pool *database.ConnectionPool
    // ... existing fields
}

// Modify Start method
func (r *enhancedSQLReceiver) Start(ctx context.Context, host component.Host) error {
    // Initialize connection pool
    r.pool = database.NewConnectionPool(
        database.WithMaxConnections(10),
        database.WithIdleConnections(5),
        database.WithConnectionTimeout(30*time.Second),
    )
    
    // Use pooled connections
    conn, err := r.pool.Get(ctx)
    if err != nil {
        return err
    }
    defer r.pool.Put(conn)
    
    // ... rest of implementation
}
```

### 2. Add Health Checks to Enterprise Distribution

**File**: `distributions/enterprise/main.go`
```go
import (
    "database-intelligence-restructured/core/internal/health"
    "net/http"
)

func main() {
    // Add after existing setup
    healthChecker := health.NewChecker()
    
    // Register component checks
    healthChecker.RegisterCheck("postgresql", checkPostgreSQLHealth)
    healthChecker.RegisterCheck("newrelic_export", checkNewRelicExport)
    healthChecker.RegisterCheck("processors", checkProcessorHealth)
    
    // Start health endpoint
    go func() {
        http.HandleFunc("/health", healthChecker.Handler())
        http.HandleFunc("/health/live", healthChecker.LivenessHandler())
        http.HandleFunc("/health/ready", healthChecker.ReadinessHandler())
        http.ListenAndServe(":8080", nil)
    }()
    
    // ... rest of main
}

func checkPostgreSQLHealth() error {
    // Check if PostgreSQL receivers are working
    return nil
}

func checkNewRelicExport() error {
    // Check if exports to New Relic are succeeding
    return nil
}
```

### 3. Implement Rate Limiting for New Relic Exporter

**File**: `exporters/nri/exporter.go`
```go
import (
    "database-intelligence-restructured/core/internal/ratelimit"
)

type nriExporter struct {
    limiter *ratelimit.Limiter
    // ... existing fields
}

func newNRIExporter(cfg *Config) (*nriExporter, error) {
    return &nriExporter{
        limiter: ratelimit.NewLimiter(
            ratelimit.WithRPS(500), // 500 requests per second
            ratelimit.WithBurst(100),
        ),
        // ... other fields
    }, nil
}

func (e *nriExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
    // Apply rate limiting
    if err := e.limiter.Wait(ctx); err != nil {
        return fmt.Errorf("rate limit exceeded: %w", err)
    }
    
    // ... existing push logic
}
```

### 4. Create First Integration Test

**File**: `tests/integration/postgresql_newrelic_test.go`
```go
package integration

import (
    "testing"
    "time"
    "database-intelligence-restructured/tests/e2e/framework"
)

func TestPostgreSQLToNewRelicPipeline(t *testing.T) {
    // Start collector with test config
    collector := framework.StartCollector(t, "test-config.yaml")
    defer collector.Stop()
    
    // Generate test data in PostgreSQL
    generateTestQueries(t)
    
    // Wait for metrics to flow
    time.Sleep(30 * time.Second)
    
    // Verify in NRDB
    client := framework.NewNRDBClient(t)
    
    // Check slow queries arrived
    slowQueries := client.Query(`
        SELECT count(*) 
        FROM PostgresSlowQueries 
        WHERE timestamp > now() - 1 minute
    `)
    assert.Greater(t, slowQueries, 0)
    
    // Check metrics arrived
    metrics := client.Query(`
        SELECT count(*) 
        FROM Metric 
        WHERE metricName LIKE 'postgres.%' 
        AND timestamp > now() - 1 minute
    `)
    assert.Greater(t, metrics, 0)
}
```

### 5. Add Secrets Management

**File**: `core/cmd/secrets_integration.go`
```go
package cmd

import (
    "database-intelligence-restructured/core/internal/secrets"
    "os"
)

// Add to collector initialization
func initializeSecrets() error {
    manager := secrets.NewManager(
        secrets.WithProvider("env"), // Start with env vars
    )
    
    // Register secrets
    manager.Register("newrelic.api_key", os.Getenv("NEW_RELIC_API_KEY"))
    manager.Register("postgres.password", os.Getenv("POSTGRES_PASSWORD"))
    
    // Make available to components
    secrets.SetGlobalManager(manager)
    
    return nil
}
```

**Update configs to use secrets**:
```yaml
exporters:
  newrelic:
    api_key: ${secret:newrelic.api_key}

receivers:
  sqlquery:
    driver: postgres
    connection_string: "host=localhost user=postgres password=${secret:postgres.password}"
```

### 6. Enable Performance Monitoring

**File**: `processors/performance_monitor.go`
```go
package processors

import (
    "database-intelligence-restructured/core/internal/performance"
)

type performanceMonitor struct {
    optimizer *performance.Optimizer
}

func newPerformanceMonitor() *performanceMonitor {
    return &performanceMonitor{
        optimizer: performance.NewOptimizer(
            performance.WithAutoTuning(true),
            performance.WithMetricsCollection(true),
        ),
    }
}

// Add to process methods
func (p *performanceMonitor) processMetrics(ctx context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
    start := time.Now()
    
    // Process metrics
    result, err := p.doProcess(ctx, metrics)
    
    // Record performance
    p.optimizer.RecordLatency("process_metrics", time.Since(start))
    p.optimizer.RecordThroughput("metrics_processed", metrics.MetricCount())
    
    // Get optimization suggestions
    if suggestions := p.optimizer.GetSuggestions(); len(suggestions) > 0 {
        for _, s := range suggestions {
            logger.Info("Performance suggestion", zap.String("suggestion", s))
        }
    }
    
    return result, err
}
```

## Testing the Integrations

### 1. Test Connection Pooling
```bash
# Monitor connection count
psql -c "SELECT count(*) FROM pg_stat_activity WHERE application_name LIKE 'otel%'"

# Should see stable connection count with pooling
```

### 2. Test Health Endpoint
```bash
# Check overall health
curl http://localhost:8080/health

# Check liveness
curl http://localhost:8080/health/live

# Check readiness
curl http://localhost:8080/health/ready
```

### 3. Monitor Rate Limiting
```bash
# Check collector logs for rate limit messages
docker logs database-intelligence-collector 2>&1 | grep -i "rate"
```

### 4. Verify Secrets Integration
```bash
# Start with secrets
NEW_RELIC_API_KEY=your-key POSTGRES_PASSWORD=pass ./database-intelligence-collector --config config.yaml

# Should see no plaintext passwords in logs
```

## Configuration Changes

### Update Base Configuration
**File**: `configs/base/collectors-base.yaml`
```yaml
extensions:
  health_check:
    endpoint: 0.0.0.0:8080
    path: /health
    
  zpages:
    endpoint: 0.0.0.0:55679

service:
  extensions: [health_check, zpages]
  pipelines:
    metrics:
      processors:
        - performance_monitor  # Add performance monitoring
        - conventions_validator  # Add convention validation
```

### Create Integration Test Config
**File**: `tests/integration/configs/full-integration.yaml`
```yaml
receivers:
  sqlquery/with_pool:
    driver: postgres
    connection_string: ${secret:postgres.connection_string}
    queries:
      - sql: "SELECT * FROM pg_stat_statements"
        metric_name: postgres.statements
    connection_pool:
      max_connections: 10
      idle_connections: 5

processors:
  batch:
    timeout: 10s
  
  performance_monitor:
    enabled: true
    auto_tune: true
    
  rate_limiter:
    rps: 1000

exporters:
  newrelic:
    api_key: ${secret:newrelic.api_key}
    rate_limit:
      enabled: true
      rps: 500

service:
  pipelines:
    metrics:
      receivers: [sqlquery/with_pool]
      processors: [batch, performance_monitor, rate_limiter]
      exporters: [newrelic]
```

## Validation Steps

1. **Start Enhanced Collector**:
```bash
./run_enhanced_collector.sh
```

2. **Monitor Health**:
```bash
watch -n 5 'curl -s localhost:8080/health | jq .'
```

3. **Check Performance Metrics**:
```bash
curl localhost:55679/debug/tracez
```

4. **Verify in New Relic**:
```bash
./validate_integration.sh
```

## Next Actions

1. **Today**: Implement connection pooling and health checks
2. **Tomorrow**: Add rate limiting and secrets management
3. **This Week**: Complete first integration tests
4. **Next Week**: Roll out to all distributions

This approach immediately adds value by:
- Reducing database connection overhead
- Providing production health monitoring
- Preventing API rate limit errors
- Securing credentials
- Enabling performance insights