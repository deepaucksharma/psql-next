# E2E Tests Quick Reference

## Test Suite Overview

| Category | Files | Purpose | Avg Runtime |
|----------|-------|---------|-------------|
| **🔌 Core** | 3 files | Basic connectivity & pipeline | 5-15 min |
| **🌐 NRDB** | 3 files | New Relic integration | 10-20 min |
| **🛡️ Security** | 3 files | PII & query intelligence | 15-25 min |
| **🚀 Performance** | 3 files | Load & scale testing | 20-40 min |
| **🔧 Reliability** | 1 file | Error handling | 5-10 min |

## Quick Commands

```bash
# Basic connectivity (no external deps)
go test ./tests/e2e/ -run TestDatabaseConnectivity -v

# Performance testing
go test ./tests/e2e/ -run TestPerformanceAndScale -v -timeout=20m

# Real NRDB integration (requires API keys)
NR_LICENSE_KEY=xxx NR_ACCOUNT_ID=xxx \
  go test ./tests/e2e/ -run TestComprehensiveE2EFlow -v

# All tests (requires full setup)
go test ./tests/e2e/... -v -timeout=30m

# Docker-based testing
docker-compose -f docker-compose.e2e.yml up -d && \
  go test ./tests/e2e/ -run TestFullPipelineWithDockerCompose -v
```

## Environment Setup

### Essential Variables
```bash
export NEW_RELIC_LICENSE_KEY="your-key"    # Required for NRDB tests
export NEW_RELIC_ACCOUNT_ID="123456"       # Required for NRQL queries  
export NEW_RELIC_API_KEY="NRAK-xxx"        # Required for API tests
```

### Database Requirements
```yaml
PostgreSQL: 13+ with auto_explain, pg_stat_statements
MySQL: 8.0+
Docker: 20.10+ (for container tests)
```

## Test Categories Detail

### 🔌 Core Integration (2-15 min)
- `e2e_main_test.go` - Basic database connectivity & OTLP pipeline
- `full_pipeline_e2e_test.go` - Docker orchestration & networking  
- `metrics_validation_test.go` - OTLP to NRDB format conversion

### 🌐 New Relic Integration (10-25 min) *Requires API Keys*
- `comprehensive_e2e_test.go` - Real NRDB validation with 30s SLA
- `true_e2e_validation_test.go` - No-mock API responses & dashboards
- `nrdb_validation_test.go` - NRQL query compatibility testing

### 🛡️ Security & Intelligence (15-30 min)
- `security_pii_test.go` - PII detection (email, SSN, CC) with 99%+ accuracy
- `plan_intelligence_test.go` - Query plan analysis & regression detection
- `ash_test.go` - Active Session History & wait event monitoring

### 🚀 Performance & Scale (20-45 min)
- `performance_scale_test.go` - Load testing (1000+ QPS, <10ms latency)
- `real_e2e_test.go` - Production OLTP/Analytics workload patterns
- `monitoring_test.go` - System observability, metrics & alerting

### 🔧 Reliability (5-15 min)
- `error_scenarios_test.go` - Circuit breakers, retries & error recovery

## Performance SLAs

| Metric | Target | Typical | Test Validates |
|--------|--------|---------|----------------|
| Database Connection | <5s | 1-2s | Connection establishment |
| NRDB End-to-End Latency | <30s | 15-25s | Data appears in New Relic |
| Query Processing | >1000 QPS | 2000+ QPS | Sustained throughput |
| PII Detection Accuracy | >99% | 99.5%+ | Pattern recognition |
| Plan Analysis Speed | <100ms | 50-80ms | Query plan processing |

## Troubleshooting

### Database Issues
```bash
# Check connectivity
pg_isready -h localhost -p 5432
mysql -h localhost -P 3306 -e "SELECT 1"

# Verify extensions
psql -c "SELECT extname FROM pg_extension WHERE extname IN ('auto_explain', 'pg_stat_statements');"
```

### New Relic Issues  
```bash
# Test API access
curl -H "Api-Key: $NR_API_KEY" "https://api.newrelic.com/graphql" \
  -d '{"query": "{ actor { user { name } } }"}'

# Test NRDB query
curl -H "Api-Key: $NR_API_KEY" \
  "https://insights-api.newrelic.com/v1/accounts/$NR_ACCOUNT_ID/query" \
  -d "nrql=SELECT count(*) FROM Metric SINCE 1 hour ago"
```

### Performance Issues
- **High Latency**: Check database indexes, system resources
- **Low Throughput**: Monitor CPU/memory, network connectivity
- **Memory Leaks**: Enable profiling, check garbage collection
- **Connection Limits**: Adjust max_connections, pool sizes

### Container Issues
```bash
# Check status
docker-compose -f docker-compose.e2e.yml ps

# View logs  
docker-compose logs collector postgres mysql

# Test internal connectivity
docker-compose exec postgres pg_isready
docker-compose exec mysql mysqladmin ping
```

## Test Data Requirements

### Core Schema
```sql
-- Minimal test schema
CREATE TABLE users (id SERIAL, email VARCHAR(255), ssn VARCHAR(11));
CREATE TABLE orders (id SERIAL, user_id INT, amount DECIMAL(10,2)); 
CREATE TABLE events (id SERIAL, event_type VARCHAR(100), event_data JSONB);

-- Required extensions
CREATE EXTENSION auto_explain;
CREATE EXTENSION pg_stat_statements;
```

### Sample Data Scale
- **Users**: 1K-10K records (with PII for security tests)
- **Orders**: 10K-100K records (for performance testing)
- **Events**: 100K-1M records (for high-cardinality testing)

## Resource Requirements

| Component | Min CPU | Min Memory | Notes |
|-----------|---------|------------|-------|
| PostgreSQL | 1 core | 1GB | With extensions |
| MySQL | 1 core | 512MB | Basic setup |
| Collector | 2 cores | 2GB | Peak load handling |
| Test Runner | 1 core | 1GB | Concurrent execution |

## CI/CD Integration

```yaml
# GitHub Actions example
jobs:
  e2e-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:13
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
        with:
          go-version: 1.21
      - name: Run E2E Tests
        run: go test ./tests/e2e/... -v -timeout=30m
        env:
          NEW_RELIC_LICENSE_KEY: ${{ secrets.NR_LICENSE_KEY }}
          NEW_RELIC_ACCOUNT_ID: ${{ secrets.NR_ACCOUNT_ID }}
```

This quick reference provides immediate access to essential commands, setup requirements, and troubleshooting steps for the E2E test suite.