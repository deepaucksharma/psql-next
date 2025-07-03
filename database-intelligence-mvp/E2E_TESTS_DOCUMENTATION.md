# E2E Tests Documentation

## Overview

The E2E test suite validates the complete database intelligence pipeline from database monitoring to New Relic integration. Tests are organized by functionality and complexity levels.

## Test Categories

### 🔌 **Core Integration Tests**
Basic connectivity and pipeline validation

| Test File | Purpose | Key Validation | Run Time |
|-----------|---------|----------------|----------|
| `e2e_main_test.go` | Basic connectivity | Database connections, OTLP pipeline | 2-5 min |
| `full_pipeline_e2e_test.go` | Docker orchestration | Container networking, service discovery | 5-10 min |
| `metrics_validation_test.go` | Data transformation | OTLP to NRDB format conversion | 3-7 min |

### 🌐 **New Relic Integration Tests**
Real NRDB integration and API validation

| Test File | Purpose | Key Validation | Dependencies |
|-----------|---------|----------------|--------------|
| `comprehensive_e2e_test.go` | Real NRDB validation | 30s latency SLA, 1000+ QPS | NR License Key |
| `true_e2e_validation_test.go` | No-mock validation | Real API responses, dashboards | NR Account Access |
| `nrdb_validation_test.go` | NRQL compatibility | Dashboard queries, aggregations | NRDB API Key |

### 🛡️ **Security & Intelligence Tests**
PII handling and query analysis

| Test File | Purpose | Key Features | Notes |
|-----------|---------|--------------|-------|
| `security_pii_test.go` | PII detection/redaction | Email, SSN, CC patterns | 99%+ accuracy |
| `plan_intelligence_test.go` | Query plan analysis | Regression detection, anonymization | Requires auto_explain |
| `ash_test.go` | Session monitoring | Active Session History, wait events | Real-time analysis |

### 🚀 **Performance & Scale Tests**
Load testing and scalability validation

| Test File | Purpose | Performance Targets | Duration |
|-----------|---------|-------------------|----------|
| `performance_scale_test.go` | Comprehensive load testing | 1000 QPS, <10ms latency | 5-15 min |
| `real_e2e_test.go` | Production workloads | OLTP/Analytics patterns | 10-20 min |
| `monitoring_test.go` | System observability | Metrics, alerts, dashboards | 5-10 min |

### 🔧 **Reliability Tests**
Error handling and recovery validation

| Test File | Purpose | Error Scenarios | Recovery Tests |
|-----------|---------|----------------|----------------|
| `error_scenarios_test.go` | Failure handling | Connection, query, resource errors | Circuit breakers, retries |

---

## Quick Start

### Basic Connectivity Test
```bash
# Test database connections and basic pipeline
go test ./tests/e2e/ -run TestDatabaseConnectivity -v
```

### Full Integration Test
```bash
# Complete pipeline with real NRDB (requires keys)
NR_LICENSE_KEY=xxx NR_ACCOUNT_ID=xxx go test ./tests/e2e/ -run TestComprehensiveE2EFlow -v
```

### Performance Testing
```bash
# Load and scale testing
go test ./tests/e2e/ -run TestPerformanceAndScale -v -timeout=20m
```

### All E2E Tests
```bash
# Complete test suite
go test ./tests/e2e/... -v -timeout=30m
```

---

## Key Test Functions by Category

### Database Integration
- **Connection Validation**: PostgreSQL, MySQL connectivity
- **Extension Testing**: auto_explain, pg_stat_statements compatibility
- **Schema Setup**: Test data creation and cleanup
- **Transaction Handling**: ACID compliance and rollback testing

### Metrics Pipeline
- **OTLP Collection**: Receiver configuration and data ingestion
- **Processor Chain**: Adaptive sampling, circuit breakers, cost control
- **Format Transformation**: OTLP to NRDB conversion
- **Export Validation**: New Relic API integration

### Security & Compliance
- **PII Detection**: Email, SSN, credit card, phone number patterns
- **Data Anonymization**: `[REDACTED_EMAIL]`, `[REDACTED_SSN]` tokens
- **SQL Injection Prevention**: Query sanitization validation
- **Compliance Validation**: Security policy adherence

### Performance Intelligence
- **Query Plan Analysis**: PostgreSQL EXPLAIN parsing
- **Regression Detection**: Statistical performance degradation alerts
- **ASH Monitoring**: Session state tracking, wait event analysis
- **Resource Monitoring**: CPU, memory, I/O utilization

### Reliability & Scale
- **Circuit Breakers**: Failure threshold and recovery testing
- **Load Testing**: Sustained 1000+ QPS with <10ms latency
- **Memory Efficiency**: Leak detection and garbage collection
- **Error Recovery**: Graceful degradation and fallback mechanisms

---

## Environment Setup

### Required Components
```yaml
# Database Requirements
PostgreSQL: 13+ with extensions [auto_explain, pg_stat_statements]
MySQL: 8.0+

# New Relic Integration
License Key: NR_LICENSE_KEY environment variable
Account ID: NR_ACCOUNT_ID for NRQL queries
API Key: NR_API_KEY for dashboard validation

# Infrastructure
Docker: 20.10+ with Compose 2.0+
System: 8GB RAM, 50GB disk, network connectivity
```

### Environment Variables
```bash
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id" 
export NEW_RELIC_API_KEY="your-api-key"
export POSTGRES_HOST="localhost"
export MYSQL_HOST="localhost"
```

### Test Data Schema
```sql
-- Core tables for testing
CREATE TABLE users (id SERIAL, email VARCHAR(255), ssn VARCHAR(11));
CREATE TABLE orders (id SERIAL, user_id INT, amount DECIMAL(10,2));
CREATE TABLE events (id SERIAL, event_type VARCHAR(100), event_data JSONB);

-- Extensions for advanced features
CREATE EXTENSION IF NOT EXISTS auto_explain;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

---

## Performance Benchmarks

### Expected Performance Metrics

| Metric Category | Target | Typical Performance | SLA |
|----------------|--------|-------------------|-----|
| **Database Connectivity** | <5s connection | 1-2s | 5s max |
| **Metric Collection** | >95% success rate | 99%+ | 95% min |
| **NRDB Latency** | <30s end-to-end | 15-25s | 30s max |
| **Query Processing** | >1000 QPS | 2000+ QPS | 1000 min |
| **PII Detection** | >99% accuracy | 99.5%+ | 99% min |
| **Plan Analysis** | <100ms processing | 50-80ms | 100ms max |

### Resource Requirements

| Component | CPU | Memory | Network | Notes |
|-----------|-----|--------|---------|-------|
| PostgreSQL | 1-2 cores | 1-2GB | 100Mbps | With extensions |
| MySQL | 1 core | 512MB | 50Mbps | Basic setup |
| Collector | 2-4 cores | 2-4GB | 200Mbps | Peak load |
| Test Runner | 1-2 cores | 1GB | 100Mbps | Concurrent tests |

---

## Troubleshooting Guide

### Common Issues & Solutions

#### Database Connection Failures
```bash
# Verify database status
pg_isready -h localhost -p 5432
mysql -h localhost -P 3306 -e "SELECT 1"

# Check extensions
psql -c "SELECT * FROM pg_extension WHERE extname IN ('auto_explain', 'pg_stat_statements');"
```

#### New Relic Integration Issues
```bash
# Test API connectivity
curl -H "Api-Key: $NR_API_KEY" "https://api.newrelic.com/graphql" \
  -d '{"query": "{ actor { user { name } } }"}'

# Verify NRDB access
curl -H "Api-Key: $NR_API_KEY" \
  "https://insights-api.newrelic.com/v1/accounts/$NR_ACCOUNT_ID/query" \
  -d "nrql=SELECT count(*) FROM Metric SINCE 1 hour ago"
```

#### Performance Test Failures
- **High Latency**: Check database indexes, query optimization
- **Low Throughput**: Monitor system resources, network latency
- **Memory Issues**: Review collector configuration, enable profiling
- **Connection Limits**: Adjust database max_connections setting

#### Container Issues
```bash
# Check container status
docker-compose -f docker-compose.e2e.yml ps

# View logs
docker-compose logs collector postgres mysql

# Test connectivity
docker-compose exec postgres pg_isready
docker-compose exec mysql mysqladmin ping
```

---

## Test Patterns & Examples

### Database Connection Pattern
```go
func connectDatabase(t *testing.T, dsn string) *sql.DB {
    db, err := sql.Open("postgres", dsn)
    require.NoError(t, err)
    require.NoError(t, db.Ping())
    return db
}
```

### Metric Validation Pattern
```go
func validateMetrics(t *testing.T, metrics []pmetric.Metrics) {
    assert.NotEmpty(t, metrics)
    for _, metric := range metrics {
        attrs := getMetricAttributes(metric)
        assert.Contains(t, attrs, "db.system")
        assert.Contains(t, attrs, "service.name")
    }
}
```

### NRDB Query Pattern
```go
func validateNRDBData(t *testing.T, client *NRDBClient, nrql string) {
    result, err := client.ExecuteNRQL(ctx, nrql)
    require.NoError(t, err)
    assert.NotEmpty(t, result.Results)
}
```

### PII Detection Pattern
```go
func validatePIIRedaction(t *testing.T, logs []string) {
    for _, log := range logs {
        assert.NotContains(t, log, "@example.com")
        assert.Contains(t, log, "[REDACTED_EMAIL]")
    }
}
```

---

## Maintenance & Updates

### Adding New Tests
1. **Identify Category**: Core, Integration, Security, Performance, Reliability
2. **Follow Patterns**: Use existing test patterns and helper functions
3. **Add Documentation**: Update this file with test details
4. **Performance Baselines**: Establish expected metrics and SLAs

### Updating Baselines
- Monitor test performance trends
- Adjust SLAs based on infrastructure changes
- Update resource requirements as system scales
- Review and update error thresholds

### CI/CD Integration
```yaml
# Example GitHub Actions configuration
- name: Run E2E Tests
  run: |
    go test ./tests/e2e/... -v -timeout=30m
  env:
    NEW_RELIC_LICENSE_KEY: ${{ secrets.NR_LICENSE_KEY }}
    NEW_RELIC_ACCOUNT_ID: ${{ secrets.NR_ACCOUNT_ID }}
```

This consolidated documentation provides a comprehensive yet organized guide to the E2E test suite, focusing on practical usage, clear categorization, and actionable troubleshooting information.