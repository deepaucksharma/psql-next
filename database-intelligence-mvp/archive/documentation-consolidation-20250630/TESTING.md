# Testing Guide - Database Intelligence Collector

## ✅ Comprehensive Testing Framework

The Database Intelligence Collector includes a sophisticated, production-grade testing suite with comprehensive end-to-end validation capabilities.

## Testing Architecture

### 1. E2E Main Test Suite (`tests/e2e/e2e_main_test.go`)

**Basic E2E Testing Framework**:
- **Testcontainers Integration**: Automated PostgreSQL container setup with seed data
- **OTEL Collector Integration**: Full collector lifecycle testing
- **New Relic Validation**: Direct NRQL query validation against live NRDB
- **Pipeline Integrity**: Complete data flow validation from database to New Relic

**Key Test Cases**:
- `TestDatabaseConnection()`: Database connectivity validation
- `TestPipelineIntegrity()`: End-to-end data flow verification
- `TestDataAccuracy()`: Ground truth comparison between source and exported data
- `TestOHISemanticParity()`: Verification of OTEL metrics providing same insights as OHI events
- `TestDimensionalCorrectness()`: Metric dimension validation
- `TestEntitySynthesis()`: New Relic entity creation verification
- `TestAdvancedProcessors()`: Custom processor validation

### 2. E2E Metrics Flow Test Suite (`tests/e2e/e2e_metrics_flow_test.go`)

**Advanced Comprehensive Testing Framework** (973+ lines):

#### Test Infrastructure Components
```go
type E2EMetricsFlowTestSuite struct {
    // Core infrastructure
    pgContainer         *postgres.PostgresContainer
    mysqlContainer      *mysql.MySQLContainer
    collector           *otelcol.Collector
    
    // Advanced testing utilities
    workloadGenerators  map[string]*WorkloadGenerator
    metricValidator     *MetricValidator
    performanceBench    *PerformanceBenchmark
    nrdbValidator       *NRDBValidator
    resourceMonitor     *ResourceMonitor
    stressTestManager   *StressTestManager
    
    // Test configuration and results
    testConfig          *E2ETestConfig
    testResults         *E2ETestResults
}
```

#### Comprehensive Test Configuration
```go
type E2ETestConfig struct {
    // Multi-database support
    PostgreSQLConfig DatabaseConfig
    MySQLConfig      DatabaseConfig
    
    // Workload configuration
    WorkloadDuration      time.Duration
    WorkloadConcurrency   int
    WorkloadTypes         []string // oltp, olap, mixed, pii_test, performance_test
    
    // Validation framework
    MetricValidationRules []ValidationRule
    NRDBValidationQueries []NRDBQuery
    
    // Performance testing
    LoadTestDuration      time.Duration
    MaxConcurrentQueries  int
    StressTestScenarios   []StressTestScenario
    
    // New Relic integration
    NewRelicConfig       NewRelicConfig
    
    // Test thresholds
    Thresholds           TestThresholds
}
```

## Advanced Test Scenarios

### 1. Database-Specific Testing
```go
// PostgreSQL metrics flow validation
func TestPostgreSQLMetricsFlow()

// MySQL metrics flow validation  
func TestMySQLMetricsFlow()

// Cross-database performance comparison
func TestQueryPerformanceTracking()
```

### 2. Data Quality & Security Testing
```go
// Comprehensive PII detection and sanitization
func TestPIISanitizationValidation()

// Data quality validation framework
func TestVerificationProcessorHealthChecks()
```

### 3. Advanced Processor Testing
```go
// Adaptive sampling under different load conditions
func TestAdaptiveSamplingBehavior()

// Circuit breaker activation and recovery scenarios
func TestCircuitBreakerActivationRecovery()

// Health checks and auto-tuning validation
func TestVerificationProcessorHealthChecks()
```

### 4. Stress & Performance Testing
```go
// High load and stress scenarios
func TestHighLoadStressTesting()

// Database failover scenarios
func TestDatabaseFailoverScenarios()

// Memory pressure and resource limits
func TestMemoryPressureResourceLimits()
```

### 5. Integration Testing
```go
// NRDB integration with validation
func TestNRDBIntegrationValidation()

// Error scenarios and recovery mechanisms
func TestErrorScenariosRecovery()
```

## Testing Capabilities

### 1. Workload Generation
- **Mixed OLTP/OLAP**: Realistic database workloads
- **Performance Testing**: Specific performance test patterns
- **PII Testing**: Workloads that generate PII data for sanitization testing
- **Concurrent Users**: Configurable concurrency levels
- **Query Patterns**: Various query types and complexities

### 2. Validation Framework
```go
type ValidationRule struct {
    Name            string
    MetricName      string
    ExpectedValue   interface{}
    Tolerance       float64
    Operator        string // gt, lt, eq, ne, between
    Attributes      map[string]interface{}
    Critical        bool
}
```

### 3. NRDB Query Validation
```go
type NRDBQuery struct {
    Name            string
    Query           string
    ExpectedResults int
    Timeout         time.Duration
    RetryCount      int
    Critical        bool
}
```

### 4. Performance Benchmarking
```go
type PerformanceMetrics struct {
    TotalRequests      int64
    SuccessfulRequests int64
    FailedRequests     int64
    AverageLatencyMS   float64
    P95LatencyMS       float64
    P99LatencyMS       float64
    ThroughputRPS      float64
    ErrorRate          float64
    DataTransferMB     float64
}
```

### 5. Resource Monitoring
```go
type ResourceUsageMetrics struct {
    AverageMemoryMB     float64
    PeakMemoryMB        int
    AverageCPUPercent   float64
    PeakCPUPercent      float64
    NetworkTransferMB   float64
}
```

### 6. Quality Metrics
```go
type QualityMetrics struct {
    DataQualityScore     float64
    EntityCorrelationRate float64
    PIISanitizationRate  float64
    QueryNormalizationRate float64
    CardinalityScore     float64
    SchemaComplianceRate float64
}
```

## Test Environment Setup

### Prerequisites
```bash
# Required environment variables
export NEW_RELIC_API_KEY="your-api-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"
export NEW_RELIC_LICENSE_KEY="your-license-key"
```

### Running Tests

#### Basic E2E Tests
```bash
# Run basic E2E test suite
go test -tags=e2e ./tests/e2e/e2e_main_test.go -v

# Skip tests in short mode
go test -short ./tests/e2e/... # Skips E2E tests
```

#### Comprehensive Metrics Flow Tests
```bash
# Run full comprehensive test suite
go test -tags=e2e ./tests/e2e/e2e_metrics_flow_test.go -v

# Run with custom configuration
E2E_CONFIG_PATH=./test-config.json go test -tags=e2e ./tests/e2e/... -v
```

#### Specific Test Categories
```bash
# Run only PostgreSQL tests
go test -tags=e2e -run TestPostgreSQLMetricsFlow ./tests/e2e/... -v

# Run only PII sanitization tests
go test -tags=e2e -run TestPIISanitizationValidation ./tests/e2e/... -v

# Run only stress tests
go test -tags=e2e -run TestHighLoadStressTesting ./tests/e2e/... -v
```

### Legacy Task-Based Testing

The original testing framework is still available via Task commands:

- **Run all tests**: `task test:all`
- **Run unit tests**: `task test:unit`
- **Run integration tests**: `task test:integration`
- **Run E2E tests**: `task test:e2e`

## Test Configuration

### Default Test Configuration
The test suite creates default configurations automatically, but you can customize with:

```json
{
  "postgresql_config": {
    "database": "testdb",
    "username": "testuser", 
    "password": "testpass",
    "connection_pool": 10,
    "extensions": ["pg_stat_statements"]
  },
  "mysql_config": {
    "database": "testdb",
    "username": "testuser",
    "password": "testpass", 
    "connection_pool": 10
  },
  "workload_duration": "300s",
  "workload_concurrency": 10,
  "workload_types": ["oltp", "olap", "mixed", "pii_test", "performance_test"],
  "load_test_duration": "600s",
  "max_concurrent_queries": 100,
  "thresholds": {
    "max_latency_ms": 1000,
    "min_throughput_rps": 10.0,
    "max_error_rate": 0.05,
    "min_data_quality_score": 0.9,
    "max_memory_usage_mb": 512,
    "max_cpu_usage_percent": 80.0
  }
}
```

## Test Data Management

### Container Initialization
- **PostgreSQL**: Automated setup with `pg_stat_statements` extension
- **MySQL**: Performance schema enabled and configured
- **Seed Data**: Realistic test data including PII patterns for sanitization testing
- **Schema Setup**: Complete database schemas with indexes and constraints

### Test Data Patterns
- **User Data**: Realistic user profiles with PII (SSN, credit cards, emails)
- **Transaction Data**: E-commerce transaction patterns
- **Query Patterns**: Realistic OLTP and OLAP query workloads
- **Performance Data**: Slow queries, high-frequency queries, complex joins

## Validation & Reporting

### Test Result Structure
```go
type E2ETestResults struct {
    TestSuite          string
    StartTime          time.Time
    EndTime            time.Time
    Duration           time.Duration
    TotalTests         int
    PassedTests        int
    FailedTests        int
    
    // Detailed metrics
    PerformanceMetrics PerformanceMetrics
    DatabaseResults    map[string]*DatabaseResults
    ValidationResults  []ValidationResult
    NRDBResults       []NRDBValidationResult
    StressTestResults []StressTestResult
    ResourceUsage     ResourceUsageMetrics
    QualityMetrics    QualityMetrics
    
    // Issues and artifacts
    Errors            []TestError
    Warnings          []TestWarning
    Artifacts         []TestArtifact
}
```

### Comprehensive Reporting
- **Performance Reports**: Latency, throughput, resource usage
- **Quality Reports**: Data quality scores, PII sanitization rates
- **Error Analysis**: Detailed error categorization and stack traces
- **Resource Usage**: CPU, memory, network utilization over time
- **Validation Reports**: Rule validation results with pass/fail status

## CI/CD Integration

### GitHub Actions Integration
```yaml
name: E2E Tests
on: [push, pull_request]
jobs:
  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Setup Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.21
      - name: Run E2E Tests
        env:
          NEW_RELIC_API_KEY: ${{ secrets.NEW_RELIC_API_KEY }}
          NEW_RELIC_ACCOUNT_ID: ${{ secrets.NEW_RELIC_ACCOUNT_ID }}
          NEW_RELIC_LICENSE_KEY: ${{ secrets.NEW_RELIC_LICENSE_KEY }}
        run: |
          go test -tags=e2e ./tests/e2e/... -v
```

## Test Environment Requirements

### Local Development
- **Docker**: For testcontainers (PostgreSQL, MySQL)
- **Go 1.21+**: For test execution
- **Network Access**: For New Relic API calls
- **Memory**: 2GB+ recommended for full test suite
- **Disk**: 1GB+ for container images and test data

### Production Testing
- **Staging Environment**: Dedicated test environment
- **Database Access**: Test databases with realistic data volumes
- **New Relic Account**: Dedicated test account for validation
- **Resource Monitoring**: Infrastructure monitoring during tests

## Test Maintenance

### Regular Test Execution
- **Pre-commit**: Basic connectivity and build tests
- **Daily**: Full E2E test suite execution
- **Weekly**: Comprehensive stress and performance testing
- **Release**: Complete validation including manual verification

### Test Data Refresh
- **Seed Data Updates**: Regular updates to test data patterns
- **Schema Evolution**: Test schema changes and migrations
- **Performance Baselines**: Update performance expectations based on improvements

This comprehensive testing framework ensures the Database Intelligence Collector meets enterprise-grade reliability and performance standards.
