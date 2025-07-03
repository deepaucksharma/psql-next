# Database Intelligence E2E Tests - Comprehensive Documentation

## Overview

This document provides a comprehensive analysis of the End-to-End (E2E) test suite for the Database Intelligence MVP project. The test suite validates the complete data flow from database operations through the OpenTelemetry collector to New Relic Database (NRDB), ensuring all features work correctly in real-world scenarios.

## Test Suite Architecture

### Core Components

1. **Test Environment Management** (`test_environment.go`)
   - Manages PostgreSQL containers using TestContainers
   - Provides mock NRDB exporters for testing
   - Handles collector lifecycle management
   - Simulates various failure scenarios

2. **Test Infrastructure** (`test_helpers.go`)
   - Database connection management
   - Load pattern generation
   - Query execution utilities
   - Metric validation helpers

3. **Supporting Utilities**
   - Configuration template management
   - Data integrity tracking
   - Network simulation tools

## Test Files Analysis

### 1. Main Test Suite (`e2e_main_test.go`)

**Purpose**: Core end-to-end validation suite using testify/suite pattern

**Test Functions**:
- `TestDatabaseConnection()` - Validates basic PostgreSQL connectivity
- `TestPipelineIntegrity()` - Tests complete data pipeline to New Relic
- `TestDataAccuracy()` - Validates data integrity between source and destination
- `TestOHISemanticParity()` - Ensures OTEL metrics provide same insights as OHI events
- `TestDimensionalCorrectness()` - Verifies metrics have correct dimensions
- `TestEntitySynthesis()` - Validates New Relic entity creation
- `TestAdvancedProcessors()` - Tests custom processor functionality

**Key Features**:
- Uses TestContainers for PostgreSQL setup
- Real New Relic API integration via GraphQL
- Comprehensive metric validation
- Circuit breaker testing
- Adaptive sampling validation

**Dependencies**:
- PostgreSQL 15-alpine container
- New Relic API credentials (NEW_RELIC_ACCOUNT_ID, NEW_RELIC_LICENSE_KEY)
- OpenTelemetry collector binary
- Auto_explain extension setup

**Expected Outcomes**:
- All metrics reach NRDB within 30 seconds
- No integration errors in New Relic
- Correct metric naming and dimensionality
- Working circuit breaker protection

### 2. Comprehensive E2E Test (`comprehensive_e2e_test.go`)

**Purpose**: Advanced end-to-end testing with real NRDB validation

**Test Functions**:
- `TestComprehensiveE2EFlow()` - Complete pipeline validation
- `testPostgreSQLMetricsFlow()` - PostgreSQL-specific metric validation
- `testPGQueryLensFlow()` - pg_querylens integration testing
- `testAllProcessors()` - All processor validation
- `testDataIntegrity()` - Data accuracy verification
- `testEndToEndLatency()` - Performance SLA validation

**Key Features**:
- Real New Relic API queries
- Precise workload tracking with checksums
- Latency measurement (30-second SLA)
- Plan change detection
- PII redaction validation
- Cost control testing

**Special Configurations**:
- ASH high-frequency sampling (1-second resolution)
- Enhanced PII detection patterns
- Budget enforcement with aggressive mode
- Query correlation in transactions

**Dependencies**:
- Real New Relic account access
- pg_querylens extension
- Multiple database connections
- Cost control configuration

### 3. Full Pipeline Test (`full_pipeline_e2e_test.go`)

**Purpose**: Complete multi-database pipeline testing with Docker Compose

**Test Functions**:
- `TestFullPipelineE2E()` - Main pipeline test
- `testPostgreSQLPipeline()` - PostgreSQL data flow
- `testMySQLPipeline()` - MySQL data flow
- `testPIISanitization()` - PII handling validation
- `testQueryCorrelation()` - Cross-query correlation
- `testCostControl()` - Cost management validation
- `testNRDBExport()` - NRDB export verification

**Key Features**:
- Multi-database support (PostgreSQL + MySQL)
- Docker Compose orchestration
- OTLP protocol validation
- Mock server verification
- High cardinality testing

**Dependencies**:
- Docker Compose environment
- MySQL and PostgreSQL containers
- Mock OTLP server
- OTLP protocol support

### 4. Real E2E Test (`real_e2e_test.go`)

**Purpose**: Production-like testing with real database workloads

**Test Functions**:
- `TestRealE2EPipeline()` - Main production-like test
- `generateDatabaseLoadReal()` - Realistic database activity
- `testPIIQueries()` - PII pattern testing
- `testExpensiveQueries()` - Performance impact testing
- `testHighCardinality()` - Cardinality management
- `testQueryCorrelationReal()` - Transaction correlation
- `validateMetricsCollection()` - Metric validation

**Scenarios Tested**:
- OLTP workload patterns
- Analytics query patterns
- Database error scenarios
- Connection failure recovery
- Query error handling
- Constraint violations

**Key Features**:
- Realistic query patterns
- Concurrent session management
- Error injection testing
- Performance monitoring
- Workload simulation

### 5. True E2E Validation (`true_e2e_validation_test.go`)

**Purpose**: No-mock comprehensive validation with real New Relic integration

**Test Functions**:
- `TestTrueEndToEndValidation()` - Complete real-world test
- `testCompleteDataFlow()` - End-to-end data tracking
- `testAllProcessorsEndToEnd()` - All processor validation
- `testPGQueryLensWithRealExtension()` - Real pg_querylens testing
- `testDataIntegrityEndToEnd()` - Complete data integrity
- `testEndToEndLatencySLA()` - Latency SLA validation
- `testHighVolumeEndToEnd()` - Scale testing (1000 QPS)
- `testFailureRecoveryEndToEnd()` - Failure recovery validation

**Key Features**:
- Real collector binary execution
- Actual New Relic API integration
- Data integrity tracking with checksums
- Performance testing (1000 QPS target)
- Failure simulation and recovery
- Complete processor chain validation

**Advanced Features**:
- Builds collector from source
- Uses real configuration files
- Tracks data through entire pipeline
- Validates in actual NRDB
- Tests at production scale

### 6. NRDB Validation Test (`nrdb_validation_test.go`)

**Purpose**: New Relic Dashboard Query Language (NRQL) validation

**Test Functions**:
- `TestNRQLDashboardQueries()` - Dashboard query validation
- `validateNRQLQueries()` - NRQL execution validation
- `generateComprehensiveTestData()` - Test data generation

**Dashboard Categories Tested**:
- **PostgreSQL Overview Dashboard**
  - Active connections monitoring
  - Transaction rate tracking
  - Cache hit ratio calculation
  - Database size monitoring
  - Top queries analysis

- **Plan Intelligence Dashboard**
  - Plan change detection
  - Regression analysis
  - Performance trend monitoring
  - Top regressions identification
  - Plan node analysis

- **ASH (Active Session History) Dashboard**
  - Active sessions over time
  - Wait event distribution
  - Blocking analysis
  - Session activity tracking
  - Resource utilization

- **Integrated Intelligence Dashboard**
  - Query performance with waits
  - Plan regression impact
  - Query health scoring
  - Adaptive sampling effectiveness

- **Alerting Queries**
  - High plan regression rate
  - Excessive lock waits
  - Query performance degradation
  - Connection saturation
  - Circuit breaker activation

**Key Features**:
- Real NRQL query execution
- Data integrity verification
- Attribute validation
- Result format checking

### 7. Metrics Validation Test (`metrics_validation_test.go`)

**Purpose**: OTLP to NRDB metric transformation validation

**Test Functions**:
- `TestMetricsToNRDBMapping()` - Metric transformation validation

**Validation Areas**:
- **Metric Naming**: NRDB naming convention compliance
- **Required Attributes**: Essential attribute presence validation
- **Data Types**: Correct data type enforcement
- **Query Correlation**: Cross-metric correlation validation
- **Common Attributes**: Shared attribute validation
- **Metric Cardinality**: Cardinality limit enforcement
- **Anonymization**: PII redaction validation
- **Regression Metrics**: Plan regression data structure
- **Wait Event Metrics**: ASH wait event categorization
- **Metric Completeness**: Full metric coverage validation

**Key Features**:
- OTLP format validation
- NRDB schema compliance
- Cardinality monitoring
- PII detection verification

### 8. Security and PII Test (`security_pii_test.go`)

**Purpose**: Comprehensive security and privacy validation

**Test Functions**:
- `TestSecurityAndPII()` - Main security test suite
- `testPIIAnonymization()` - PII pattern detection and redaction
- `testSQLInjectionPrevention()` - SQL injection protection
- `testDataLeakPrevention()` - Data leak prevention
- `testComplianceValidation()` - Regulatory compliance

**PII Patterns Tested**:
- **Email Addresses**: Multiple formats including plus addressing
- **Social Security Numbers**: With and without dashes
- **Credit Card Numbers**: Visa, Mastercard, Amex formats
- **Phone Numbers**: Various US and international formats
- **API Keys and Secrets**: AWS secrets, Bearer tokens
- **IP Addresses**: IPv4 and IPv6
- **Complex Scenarios**: Multiple PII in single queries

**Security Tests**:
- SQL injection attempt detection
- Dangerous pattern filtering
- Error message sanitization
- Log file PII prevention
- Metric label PII prevention

**Compliance Standards**:
- **GDPR**: Right to erasure, data minimization
- **HIPAA**: Medical record protection
- **PCI DSS**: Credit card data protection
- **SOC2**: Security principle compliance

### 9. Plan Intelligence Test (`plan_intelligence_test.go`)

**Purpose**: PostgreSQL query plan analysis and regression detection

**Test Functions**:
- `TestPlanIntelligenceE2E()` - Main plan intelligence test
- `setupTestSchemaPlanIntelligence()` - Test schema setup
- `generateSlowQueriesPlanIntelligence()` - Plan-triggering queries

**Features Tested**:
- **Auto-explain Log Collection**: PostgreSQL auto_explain integration
- **Plan Anonymization**: PII redaction in execution plans
- **Plan Regression Detection**: Performance degradation identification
- **NRDB Export**: Plan metrics in New Relic format
- **Circuit Breaker Protection**: auto_explain failure handling

**Key Scenarios**:
- Index creation and removal for plan changes
- Complex queries with multiple joins
- CTE (Common Table Expression) queries
- Subquery optimization testing

**Configuration Requirements**:
- auto_explain extension loaded
- JSON format logging enabled
- Minimum duration thresholds
- Plan anonymization patterns

### 10. ASH Test (`ash_test.go`)

**Purpose**: Active Session History monitoring validation

**Test Functions**:
- `TestASHE2E()` - Main ASH test suite
- `setupASHTestSchema()` - ASH test data setup
- `generateWaitEvents()` - Wait event simulation
- `executeTrackedQuery()` - Query activity tracking

**ASH Features Tested**:
- **Session Sampling**: Multi-state session monitoring
- **Wait Event Analysis**: Categorized wait event tracking
- **Blocking Detection**: Lock contention identification
- **Adaptive Sampling**: High-volume session handling
- **Query Activity Tracking**: Multi-session query monitoring
- **Time Window Aggregation**: Historical data aggregation
- **Wait Event Alerts**: Threshold-based alerting

**Session States Monitored**:
- Active queries
- Idle in transaction
- Blocked sessions
- Normal query execution

**Wait Event Categories**:
- IO waits
- Lock waits
- Client waits
- CPU waits

### 11. Error Scenarios Test (`error_scenarios_test.go`)

**Purpose**: Comprehensive error handling and failure recovery

**Test Functions**:
- `TestErrorScenarios()` - Main error test suite
- `testDatabaseFailures()` - Database-level failures
- `testProcessorFailures()` - Processor-level failures
- `testDataCorruption()` - Data corruption scenarios
- `testCascadingFailures()` - Multi-component failures
- `testRecoveryMechanisms()` - Recovery validation

**Database Failure Scenarios**:
- Connection timeouts
- Authentication failures
- Network partitions
- Resource exhaustion
- Database crash recovery

**Processor Failure Scenarios**:
- Panic recovery
- Memory limit exceeded
- Configuration errors
- Dependency failures

**Data Corruption Scenarios**:
- Malformed metrics
- Invalid attributes
- Encoding errors
- Schema violations

**Recovery Mechanisms**:
- Automatic reconnection
- Graceful degradation
- State recovery
- Self-healing

## Test Infrastructure

### Environment Setup

**Prerequisites**:
- Docker and Docker Compose
- PostgreSQL with required extensions
- OpenTelemetry Collector
- New Relic account (for real tests)
- Go test environment

**Required Extensions**:
- `pg_stat_statements` - Query statistics
- `auto_explain` - Query plan logging
- `pg_querylens` - Plan regression detection (optional)

**Environment Variables**:
```bash
# Required for real NRDB tests
NEW_RELIC_ACCOUNT_ID=your_account_id
NEW_RELIC_API_KEY=your_api_key
NEW_RELIC_LICENSE_KEY=your_license_key

# Database configuration
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=testdb

# MySQL configuration (if used)
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=root
MYSQL_DB=testdb
```

### Configuration Templates

Tests use various collector configurations:
- `config-plan-intelligence.yaml` - Plan analysis configuration
- `config-ash.yaml` - ASH monitoring configuration
- `config-full-integration.yaml` - Complete feature set
- `config-newrelic.yaml` - New Relic export configuration

### Mock Services

**Mock NRDB Exporter**:
- Simulates New Relic OTLP endpoint
- Captures metrics for validation
- Converts OTLP to NRDB format
- Provides query interface for tests

**Mock Servers**:
- OTLP receiver simulation
- Metrics validation endpoints
- Health check endpoints

## Test Execution

### Running Individual Tests

```bash
# Run specific test file
go test -v ./tests/e2e -run TestComprehensiveE2EFlow

# Run with short mode (skips long tests)
go test -short ./tests/e2e

# Run specific test category
go test -v ./tests/e2e -run TestPlanIntelligence

# Run with New Relic integration
NEW_RELIC_ACCOUNT_ID=xxx NEW_RELIC_API_KEY=xxx go test -v ./tests/e2e -run TestNRQLDashboard
```

### Running Complete Suite

```bash
# Run all E2E tests
make test-e2e

# Run with Docker Compose environment
make test-e2e-docker

# Run comprehensive validation
make test-e2e-comprehensive
```

### Test Execution Scripts

Available scripts in the e2e directory:
- `run-e2e-tests.sh` - Basic E2E test execution
- `run-comprehensive-e2e-tests.sh` - Full test suite
- `run-real-e2e.sh` - Real environment testing
- `validate-data-shape.sh` - Data validation
- `simulate-nrdb-queries.sh` - NRDB simulation

## Validation Criteria

### Performance Metrics

**Latency SLAs**:
- End-to-end latency: < 30 seconds (from query execution to NRDB)
- Collection interval: 10-60 seconds
- Processing latency: < 5 seconds per batch

**Throughput Targets**:
- PostgreSQL: 1000+ QPS sustained
- MySQL: 500+ QPS sustained
- ASH sampling: 1-second resolution
- Metric cardinality: < 50,000 unique series

### Data Integrity

**Accuracy Requirements**:
- 99.9% data accuracy (measured via checksums)
- No data loss during normal operations
- < 1% acceptable loss during failures
- Exact query correlation across processors

**Completeness Validation**:
- All expected metric types present
- Required attributes populated
- Proper dimensional data
- Correct aggregation windows

### Security Compliance

**PII Protection**:
- 100% PII pattern detection rate
- Zero PII data in exported metrics
- Proper redaction markers
- Compliance with GDPR, HIPAA, PCI DSS

**Security Measures**:
- SQL injection prevention
- Data leak prevention
- Secure credential handling
- Error message sanitization

## Troubleshooting

### Common Issues

**Container Startup Failures**:
- Check Docker daemon status
- Verify port availability
- Review container logs
- Ensure sufficient resources

**Database Connection Issues**:
- Verify credentials
- Check network connectivity
- Review PostgreSQL logs
- Validate extension installation

**Metric Validation Failures**:
- Check collector configuration
- Verify processor chain
- Review metric naming
- Validate attribute presence

**New Relic Integration Issues**:
- Verify API credentials
- Check account permissions
- Review OTLP endpoint
- Validate query syntax

### Debug Commands

```bash
# Check container status
docker ps

# View container logs
docker logs e2e-postgres
docker logs e2e-collector

# Check metric endpoint
curl http://localhost:8888/metrics

# Validate collector health
curl http://localhost:13133/health

# Check OTLP endpoint
curl -X POST http://localhost:4317/v1/metrics
```

### Log Analysis

**Important Log Files**:
- Collector logs: `/var/log/otel/collector.log`
- PostgreSQL logs: `/var/log/postgresql/postgresql.log`
- Test output: `test-results/e2e-output.json`

**Key Log Patterns**:
- `ERROR` - Error conditions
- `WARN` - Warning conditions
- `metrics exported` - Successful exports
- `circuit breaker` - Protection activations
- `pii detected` - PII redaction events

## Future Enhancements

### Planned Test Additions

1. **Multi-tenant Testing**: Validate data isolation
2. **Disaster Recovery**: Cross-region failover
3. **Kubernetes Integration**: Cloud-native deployment
4. **Performance Benchmarking**: Automated performance regression
5. **Chaos Engineering**: Advanced failure scenarios

### Test Infrastructure Improvements

1. **Parallel Execution**: Faster test completion
2. **Dynamic Scaling**: Auto-scaling test environments
3. **Visual Dashboards**: Real-time test monitoring
4. **Automated Reporting**: Test result analysis
5. **Integration Pipelines**: CI/CD integration

## Conclusion

The Database Intelligence E2E test suite provides comprehensive validation of the complete system, from database operations through metric collection, processing, and export to New Relic. The tests ensure data accuracy, performance compliance, security requirements, and operational reliability across various scenarios including normal operations, failure conditions, and recovery situations.

The test suite serves as both validation and documentation of system behavior, providing confidence in production deployments and enabling safe feature development and performance optimization.