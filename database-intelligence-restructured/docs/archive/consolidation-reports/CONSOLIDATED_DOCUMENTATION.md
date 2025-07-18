# Database Intelligence for PostgreSQL - Complete Documentation

## Table of Contents

1. [Project Overview](#project-overview)
2. [Architecture & Design](#architecture--design)
3. [Getting Started](#getting-started)
4. [Configuration Guide](#configuration-guide)
5. [Deployment Guide](#deployment-guide)
6. [Testing & Validation](#testing--validation)
7. [Troubleshooting](#troubleshooting)
8. [Development History](#development-history)
9. [API & Components Reference](#api--components-reference)
10. [Migration Guides](#migration-guides)

---

## Project Overview

### What is Database Intelligence?

Database Intelligence is an advanced PostgreSQL monitoring solution built on OpenTelemetry that provides:

- **Config-Only Mode**: Standard OpenTelemetry components collecting 35+ PostgreSQL metrics
- **Custom/Enhanced Mode**: Additional features including Active Session History (ASH), query intelligence, and adaptive sampling

### Current Status (PostgreSQL-Only Implementation)

- ✅ **MySQL Removed**: Project now focuses exclusively on PostgreSQL
- ✅ **Maximum Metrics**: Config-only mode collects all 35+ available PostgreSQL metrics
- ✅ **Production Ready**: Both modes can run in parallel for comparison
- ✅ **Comprehensive Testing**: Full test suite and validation tools

### Key Features

#### Config-Only Mode
- Standard PostgreSQL receiver (35+ metrics)
- SQL Query receiver for custom metrics
- Host metrics collection
- New Relic OTLP export
- Low resource usage (~200MB RAM)

#### Custom/Enhanced Mode
- Everything in Config-Only PLUS:
- Active Session History (ASH) for real-time monitoring
- Query plan extraction and analysis
- Wait event analysis
- Blocked session detection
- Adaptive sampling for cost control
- Circuit breaker protection
- Query correlation

---

## Architecture & Design

### System Architecture

```
     ┌─────────────────────┐
     │   PostgreSQL DB     │
     │   (Shared)          │
     └──────────┬──────────┘
                │
     ┌──────────┴──────────┐
     │                     │
┌────▼──────────────┐     ┌──────────▼────────────┐
│ Config-Only       │     │ Custom/Enhanced       │
│ Collector         │     │ Collector             │
│                   │     │                       │
│ • PostgreSQL recv │     │ • PostgreSQL recv     │
│ • SQL Query recv  │     │ • ASH receiver        │
│ • Host Metrics    │     │ • Enhanced SQL recv   │
│                   │     │ • Kernel Metrics      │
│ Standard          │     │ • Adaptive Sampling   │
│ Processors        │     │ • Circuit Breaker     │
│                   │     │ • Query Plans         │
│                   │     │ • Cost Control        │
└────────┬──────────┘     └──────────┬────────────┘
         │                           │
         └────────────┬──────────────┘
                      │
              ┌───────▼────────┐
              │  New Relic     │
              │  • PostgreSQL  │
              │    Dashboard   │
              │  • Comparison  │
              └────────────────┘
```

### Component Architecture

#### Receivers
- **postgresql**: Standard OTel receiver for PostgreSQL metrics
- **sqlquery**: Custom SQL queries for additional metrics
- **ash**: Active Session History (custom mode only)
- **enhancedsql**: Enhanced SQL metrics (custom mode only)
- **kernelmetrics**: System kernel metrics (custom mode only)
- **hostmetrics**: CPU, memory, disk, network metrics

#### Processors
- **Standard**: batch, memory_limiter, resourcedetection, attributes
- **Enhanced** (custom mode):
  - **adaptivesampler**: Dynamic sampling based on load
  - **circuitbreaker**: Protection against overload
  - **costcontrol**: Limit data points per minute
  - **planattributeextractor**: Extract query plans
  - **querycorrelator**: Correlate related queries
  - **ohitransform**: OHI compatibility transformation

#### Exporters
- **otlp**: New Relic OTLP endpoint
- **nri**: New Relic Infrastructure format (custom mode)
- **debug**: Development troubleshooting

### Module Structure

The project was restructured from 15+ separate modules to a single consolidated module to fix:
- Version conflicts between OpenTelemetry dependencies
- Circular dependency risks
- Maintenance overhead

---

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go 1.21+ (for development)
- PostgreSQL 12+ 
- New Relic account with:
  - Valid license key
  - OTLP ingestion enabled

### Quick Start (5 minutes)

1. **Clone and setup environment**:
```bash
git clone https://github.com/newrelic/database-intelligence
cd database-intelligence

# Set required environment variables
export NEW_RELIC_LICENSE_KEY="your-license-key"
export NEW_RELIC_ACCOUNT_ID="your-account-id"
```

2. **Deploy parallel setup**:
```bash
./scripts/deploy-parallel-modes.sh
```

3. **Generate test data**:
```bash
# Option 1: Comprehensive test generator
cd tools/postgres-test-generator
go run main.go

# Option 2: Load generator
cd tools/load-generator
go run main.go -pattern=mixed -qps=50
```

4. **Verify metrics**:
```bash
./scripts/verify-metrics.sh
```

5. **Deploy dashboard**:
```bash
./scripts/migrate-dashboard.sh deploy dashboards/newrelic/postgresql-parallel-dashboard.json
```

### Verification

Check metrics in New Relic:
```sql
SELECT uniques(metricName) FROM Metric 
WHERE deployment.mode IN ('config-only', 'custom') 
AND metricName LIKE 'postgresql%' 
SINCE 30 minutes ago
```

---

## Configuration Guide

### Environment Variables

#### Required
- `NEW_RELIC_LICENSE_KEY`: Your New Relic license key
- `NEW_RELIC_ACCOUNT_ID`: Your New Relic account ID

#### PostgreSQL Connection
- `POSTGRES_HOST`: PostgreSQL host (default: localhost)
- `POSTGRES_PORT`: PostgreSQL port (default: 5432)
- `POSTGRES_USER`: PostgreSQL user (default: postgres)
- `POSTGRES_PASSWORD`: PostgreSQL password (default: postgres)
- `POSTGRES_DB`: Database name (default: testdb)

#### Optional
- `NEW_RELIC_OTLP_ENDPOINT`: OTLP endpoint (default: https://otlp.nr-data.net:4317)
- `OTEL_SERVICE_NAME`: Service name for telemetry
- `DEPLOYMENT_MODE`: Mode identifier (config-only or custom)

### Config-Only Mode Configuration

Located in `deployments/docker/compose/configs/postgresql-maximum-extraction.yaml`:

```yaml
receivers:
  postgresql:
    endpoint: ${env:POSTGRES_HOST}:${env:POSTGRES_PORT}
    collection_interval: 10s
    metrics:
      # 35+ metrics enabled
      postgresql.backends:
        enabled: true
      postgresql.commits:
        enabled: true
      # ... all other metrics

  sqlquery/postgresql:
    queries:
      - sql: "SELECT state, COUNT(*) FROM pg_stat_activity..."
        metrics:
          - metric_name: pg.connection_count

processors:
  attributes:
    actions:
      - key: deployment.mode
        value: config-only

exporters:
  otlp:
    endpoint: ${env:NEW_RELIC_OTLP_ENDPOINT}
    headers:
      api-key: ${env:NEW_RELIC_LICENSE_KEY}
```

### Custom Mode Configuration

Additional components in `custom-mode.yaml`:

```yaml
receivers:
  ash:
    collection_interval: 1s
    sampling:
      base_rate: 1.0
      adaptive: true
    
  enhancedsql:
    queries:
      - name: query_stats
        sql: "SELECT * FROM pg_stat_statements..."

processors:
  adaptivesampler:
    sampling_percentage: 100
    
  circuitbreaker:
    failure_threshold: 5
    
  costcontrol:
    max_datapoints_per_minute: 1000000
```

### PostgreSQL Metrics Reference

All 35+ metrics collected in config-only mode:

```
postgresql.backends              # Active connections
postgresql.bgwriter.*            # Background writer stats
postgresql.blocks_read           # Disk blocks read
postgresql.blks_hit             # Buffer cache hits
postgresql.blks_read            # Physical reads
postgresql.buffer.hit           # Buffer hit ratio
postgresql.commits              # Transaction commits
postgresql.conflicts            # Query conflicts
postgresql.connection.max       # Max connections setting
postgresql.database.*           # Database-level metrics
postgresql.deadlocks            # Deadlock count
postgresql.index.*              # Index usage stats
postgresql.live_rows            # Live row count
postgresql.locks                # Lock statistics
postgresql.operations           # Various operations
postgresql.replication.*        # Replication metrics
postgresql.rollbacks            # Transaction rollbacks
postgresql.rows                 # Row operations
postgresql.sequential_scans     # Sequential scan count
postgresql.stat_activity.count  # Connection states
postgresql.table.*              # Table-level metrics
postgresql.temp_files           # Temporary file usage
postgresql.wal.*                # Write-ahead log metrics
```

---

## Deployment Guide

### Docker Deployment

#### Build Images
```bash
# Build custom collector
docker build -t newrelic/database-intelligence-enterprise:latest \
  -f deployments/docker/Dockerfile.enterprise .

# Build load generator
docker build -t newrelic/database-intelligence-loadgen:latest \
  -f deployments/docker/Dockerfile.loadgen .
```

#### Run with Docker Compose
```bash
cd deployments/docker/compose
docker compose -f docker-compose-parallel.yaml up -d
```

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: db-intel-collector
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: collector
        image: newrelic/database-intelligence-enterprise:latest
        env:
        - name: NEW_RELIC_LICENSE_KEY
          valueFrom:
            secretKeyRef:
              name: newrelic
              key: license-key
```

### Production Considerations

#### Resource Requirements
- **Config-Only Mode**: 200MB RAM, 0.5 CPU
- **Custom Mode**: 500MB-1GB RAM, 1-2 CPU

#### Scaling
- Run multiple collectors for different database clusters
- Use deployment.mode attribute to separate metrics
- Consider sampling in custom mode for high-volume databases

#### Security
- Use secrets management for credentials
- Enable TLS for PostgreSQL connections
- Restrict collector network access
- Review PostgreSQL user permissions

---

## Testing & Validation

### Test Tools

#### PostgreSQL Test Generator
Exercises all PostgreSQL metrics:
```bash
cd tools/postgres-test-generator
go run main.go \
  -workers=10 \
  -deadlocks=true \
  -temp-files=true \
  -interval=100ms
```

#### Load Generator
Multiple load patterns:
```bash
cd tools/load-generator
go run main.go -pattern=mixed -qps=50

# Patterns: simple, complex, analytical, blocking, mixed, stress
```

### Validation Scripts

#### Verify Metrics Collection
```bash
./scripts/verify-metrics.sh
# Generates: metrics-verification-report.md
```

#### End-to-End Validation
```bash
./scripts/validate-metrics-e2e.sh
# Generates: e2e-validation-queries.md with 100+ NRQL queries
```

### Test Scenarios

1. **Connection Pool Testing**
   - Verify postgresql.backends metric
   - Test connection limits
   - Monitor connection states

2. **Transaction Testing**
   - Verify commits/rollbacks ratio
   - Test transaction performance
   - Monitor deadlocks

3. **Performance Testing**
   - Buffer cache hit ratio
   - Sequential vs index scans
   - Query performance metrics

4. **Replication Testing**
   - WAL lag monitoring
   - Replication delay metrics

---

## Troubleshooting

### Common Issues

#### No Metrics in New Relic

1. **Check collector logs**:
```bash
docker logs db-intel-collector-config-only 2>&1 | grep -i error
```

2. **Verify credentials**:
```bash
docker exec db-intel-collector-config-only env | grep NEW_RELIC
```

3. **Test connectivity**:
```sql
-- In New Relic Query Builder
SELECT count(*) FROM Metric 
WHERE deployment.mode IN ('config-only', 'custom') 
SINCE 5 minutes ago
```

#### Missing Specific Metrics

1. **Check PostgreSQL permissions**:
```sql
-- Required grants
GRANT SELECT ON pg_stat_database TO monitoring_user;
GRANT SELECT ON pg_stat_activity TO monitoring_user;
GRANT SELECT ON pg_stat_statements TO monitoring_user;
```

2. **Enable extensions**:
```sql
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

3. **Verify metric configuration**:
```bash
docker exec db-intel-collector-config-only \
  cat /etc/otel-collector-config.yaml | grep "postgresql.deadlocks"
```

#### Performance Issues

1. **Reduce collection frequency**:
```yaml
postgresql:
  collection_interval: 30s  # Increase from 10s
```

2. **Enable sampling** (custom mode):
```yaml
adaptivesampler:
  sampling_percentage: 50
```

3. **Limit SQL queries**:
```yaml
sqlquery/postgresql:
  collection_interval: 60s
  queries:
    - sql: "SELECT ... LIMIT 100"
```

### Debug Mode

Enable debug logging:
```yaml
service:
  telemetry:
    logs:
      level: debug

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 10
```

---

## Development History

### Project Evolution

1. **Initial Design**: Dual PostgreSQL/MySQL support with custom components
2. **Architecture Review**: Found 15+ module conflicts, configuration chaos
3. **Phase 1 Fixes**: Module consolidation, memory leak fixes, configuration cleanup
4. **PostgreSQL Focus**: Removed MySQL, maximized PostgreSQL metrics
5. **Current State**: Production-ready parallel deployment

### Key Architectural Decisions

1. **Module Consolidation**: Fixed version conflicts by merging 15+ modules
2. **Configuration Simplification**: Reduced from 25+ configs to 2 main configs
3. **PostgreSQL-Only**: Focused scope for better quality
4. **Parallel Deployment**: Allow gradual migration from config-only to custom

### Major Milestones

- ✅ Fixed circular dependencies and version conflicts
- ✅ Implemented comprehensive error handling
- ✅ Removed hardcoded credentials (47 files cleaned)
- ✅ Added memory leak prevention
- ✅ Created production Docker images (71.8MB)
- ✅ Implemented all PostgreSQL metrics
- ✅ Built comprehensive testing suite

---

## API & Components Reference

### Custom Receivers

#### ASH Receiver
```go
type Config struct {
    Datasource           string
    CollectionInterval   time.Duration
    Sampling            SamplingConfig
    BufferSize          int
    RetentionDuration   time.Duration
    SlowQueryThreshold  int
}

type ASHSample struct {
    Timestamp       time.Time
    SessionID       string
    State          string
    WaitEventType  string
    WaitEventName  string
    BlockingPID    int
    QueryStart     time.Time
}
```

#### Enhanced SQL Receiver
```go
type QueryConfig struct {
    Name    string
    SQL     string
    Metrics []MetricConfig
}

type MetricConfig struct {
    MetricName      string
    ValueColumn     string
    AttributeColumns []string
    ValueType       string
}
```

### Custom Processors

#### Adaptive Sampler
```go
type Config struct {
    SamplingPercentage      float64
    EvaluationInterval      time.Duration
    DecisionWait           time.Duration
    NumTraces              uint64
    ExpectedNewTracesPerSec uint64
}
```

#### Circuit Breaker
```go
type Config struct {
    FailureThreshold   int
    RecoveryTimeout    time.Duration
    MetricsLimit       int
}
```

#### Cost Control
```go
type Config struct {
    MaxDatapointsPerMinute int
    EnforcementMode        string // "drop" or "sample"
}
```

---

## Migration Guides

### From MySQL/PostgreSQL to PostgreSQL-Only

1. **Update Docker Compose**:
   - Remove MySQL service
   - Remove MySQL environment variables
   - Update health checks

2. **Update Configurations**:
   - Remove MySQL receiver sections
   - Remove MySQL from pipelines
   - Update SQL query receivers

3. **Update Dashboards**:
   - Remove MySQL widgets
   - Update NRQL queries
   - Deploy PostgreSQL-only dashboard

### From Config-Only to Custom Mode

1. **Evaluate Features**:
   - Do you need ASH for session monitoring?
   - Do you need query plan analysis?
   - Can you handle 2-3x resource usage?

2. **Test in Parallel**:
   ```bash
   # Both modes run simultaneously
   ./scripts/deploy-parallel-modes.sh
   ```

3. **Compare Metrics**:
   ```sql
   -- In New Relic
   FROM Metric SELECT uniqueCount(metricName) 
   FACET deployment.mode 
   WHERE metricName LIKE 'postgresql%'
   ```

4. **Gradual Migration**:
   - Start with non-production databases
   - Monitor resource usage
   - Adjust sampling rates
   - Roll out to production

### From OHI to OpenTelemetry

The custom mode includes OHI transformation for compatibility:

```yaml
processors:
  ohitransform:
    transform_rules:
      - source_metric: "db.ash.active_sessions"
        target_event: "PostgresSlowQueries"
        mappings:
          "db.postgresql.query_id": "query_id"
```

---

## Appendices

### A. File Structure
```
database-intelligence-restructured/
├── components/           # Custom components
│   ├── receivers/       # ASH, enhanced SQL, kernel metrics
│   ├── processors/      # Adaptive sampling, circuit breaker, etc.
│   └── exporters/       # NRI exporter
├── configs/             # Sample configurations
├── dashboards/          # New Relic dashboards
├── deployments/         # Docker and K8s configs
├── scripts/            # Automation scripts
├── tests/              # Test suites
└── tools/              # Load generators
```

### B. Metrics Quick Reference

| Metric Category | Count | Key Metrics |
|----------------|-------|-------------|
| Connections | 5 | backends, connection.max, stat_activity.count |
| Transactions | 3 | commits, rollbacks, deadlocks |
| Block I/O | 4 | blks_hit, blks_read, blocks_read, buffer.hit |
| Row Operations | 2 | rows, database.rows |
| Indexes | 3 | index.scans, index.size, sequential_scans |
| WAL/Replication | 4 | wal.lag, wal.age, wal.delay, replication.data_delay |
| Background Writer | 7 | bgwriter.*, checkpoints_timed, checkpoints_req |
| Database/Tables | 8 | database.size, table.size, vacuum.count, etc. |
| Performance | 4 | temp_files, conflicts, locks, operations |

### C. Useful Commands

```bash
# Check PostgreSQL metrics
psql -c "SELECT * FROM pg_stat_database"
psql -c "SELECT * FROM pg_stat_activity"

# Monitor collectors
docker stats db-intel-collector-config-only
docker logs -f db-intel-collector-custom

# Generate specific load patterns
./tools/load-generator/main -pattern=blocking
./tools/postgres-test-generator/main -deadlocks=true

# Quick metric check
curl -s http://localhost:4318/v1/metrics | jq '.resourceMetrics[0].scopeMetrics[0].metrics[].name' | grep postgresql
```

---

## Support & Contributing

- **Issues**: GitHub Issues for bug reports
- **Documentation**: This consolidated guide
- **Dashboards**: Pre-built New Relic dashboards
- **Community**: OpenTelemetry Slack #database-monitoring

---

*Last Updated: [Current Date]*
*Version: PostgreSQL-Only Implementation*
*Status: Production Ready*