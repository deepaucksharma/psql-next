# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a comprehensive MySQL monitoring solution using OpenTelemetry Collector with advanced features including:
- SQL intelligence engine with wait analysis
- ML-based anomaly detection
- Business impact tracking and SLA monitoring
- Performance advisory system with actionable recommendations

The solution sends metrics to New Relic via OTLP and optionally to Prometheus.

## Key Commands

### Quick Start
```bash
# Full deployment with workload
./start.sh

# Specific deployment mode
./start.sh deploy --minimal   # Low resources
./start.sh deploy --standard  # Production
./start.sh deploy --advanced  # All features (default)

# Run tests
./start.sh test

# Stop everything
./start.sh stop
```

### Development Commands
```bash
# Deploy with Docker Compose
docker compose up -d

# Deploy with custom settings
export DEPLOYMENT_MODE=advanced
./deploy/deploy.sh --with-workload

# Check health
./operate/diagnose.sh
./operate/test-connection.sh
./operate/validate-metrics.sh

# Generate workload
./operate/generate-workload.sh

# View logs
docker compose logs -f otel-collector

# Run full test suite
./operate/full-test.sh
```

## Architecture

### Configuration Hierarchy
```
.env → docker-compose.yml → config/collector/master.yaml → MySQL configs
```

### Master Configuration (`config/collector/master.yaml`)
The 1,442-line master configuration contains:
- **Receivers**: MySQL primary/replica, SQL intelligence queries, host metrics
- **Processors**: 20+ processors for ML features, anomaly detection, business context
- **Exporters**: New Relic OTLP, Prometheus, debug outputs
- **Pipelines**: Multiple specialized pipelines activated by deployment mode

### Deployment Modes & Pipelines

1. **minimal**: Basic monitoring
   - Pipeline: `metrics/minimal`
   - Features: Basic MySQL + host metrics

2. **standard**: Production recommended
   - Pipeline: `metrics/standard`
   - Features: Adds replica monitoring, enrichment

3. **advanced**: Full intelligence (default)
   - Pipelines: `metrics/critical_realtime`, `metrics/analysis`, `metrics/ml_features`, `metrics/business`, `metrics/enhanced_standard`
   - Features: SQL intelligence, wait analysis, ML, anomaly detection, advisory

4. **debug**: Troubleshooting
   - Pipeline: `metrics/debug` + all others
   - Features: All features + verbose logging, file output

### SQL Intelligence Engine

When `ENABLE_SQL_INTELLIGENCE=true`, the collector executes a comprehensive 500-line SQL query with CTEs that analyze:
- Real-time wait events
- Historical patterns
- Lock analysis
- Resource metrics
- Query performance profiles

This data feeds into multiple processors that generate:
- Wait categorization
- Anomaly scores
- Performance recommendations
- Business impact estimates

## Critical Environment Variables

### Required
- `NEW_RELIC_LICENSE_KEY` - New Relic ingest key
- `NEW_RELIC_ACCOUNT_ID` - For NerdGraph features

### Key Configuration
- `DEPLOYMENT_MODE` - Controls active pipelines
- `ENABLE_SQL_INTELLIGENCE` - Enables deep SQL analysis
- `MYSQL_PRIMARY_ENDPOINT` - Primary MySQL connection
- `MYSQL_USER` / `MYSQL_PASSWORD` - Monitoring credentials

### Feature Flags
- `WAIT_PROFILE_ENABLED` - Query wait analysis
- `ML_FEATURES_ENABLED` - ML feature generation
- `ANOMALY_DETECTION_ENABLED` - Anomaly detection
- `ADVISOR_ENGINE_ENABLED` - Performance recommendations
- `BUSINESS_CONTEXT_ENABLED` - Business impact tracking

## Project Structure Patterns

### Scripts Organization
- `deploy/` - Setup and deployment scripts
- `operate/` - Operational tools (validation, testing, workload)
- Scripts follow naming: `{action}-{target}.sh`

### Configuration Layout
- `config/collector/master.yaml` - Main OTEL config
- `config/mysql/*.cnf` - MySQL server configs
- `config/newrelic/dashboards.json` - Dashboard definitions
- Example configs in `examples/configurations/`

### MySQL Initialization
- `mysql/init/01-schema.sql` - Creates monitoring user, schema, procedures
- `mysql/init/02-sample-data.sql` - Generates test data
- Procedures: `place_order()`, `browse_products()`, `run_analytics()`, `manage_cart()`

## Key Metrics and Attributes

### Standard MySQL Metrics
All 40+ MySQL metrics prefixed with `mysql.` including:
- `mysql.threads`, `mysql.query.count`, `mysql.buffer_pool.usage`
- `mysql.replica.lag`, `mysql.locks.deadlock`

### Intelligence Metrics
- `mysql.intelligence.comprehensive` - Main intelligence score
- `mysql.query.wait_profile` - Wait analysis data
- `mysql.health.score` - Overall health metric

### Important Attributes
- `anomaly.detected`, `anomaly.severity` - Anomaly detection
- `advisor.type`, `advisor.recommendation` - Advisory engine
- `wait.category` - Wait event categorization
- `business.revenue_impact` - Business impact

## Testing and Validation

### Connection Testing
```bash
# Test MySQL connectivity
./operate/test-connection.sh

# Validate configuration
./operate/validate-config.sh
```

### Metric Validation
```bash
# Check metrics in New Relic
./operate/validate-metrics.sh

# Local endpoints
curl http://localhost:13133/        # Health
curl http://localhost:8888/metrics  # Internal metrics
curl http://localhost:8889/metrics  # Prometheus format
```

### New Relic Queries
```sql
-- Basic check
FROM Metric SELECT * WHERE instrumentation.provider = 'opentelemetry' SINCE 5 minutes ago

-- Intelligence metrics
FROM Metric SELECT * WHERE metricName = 'mysql.intelligence.comprehensive'

-- Anomalies
FROM Metric SELECT * WHERE attributes['anomaly.detected'] = true
```

## Important: Use Existing Scripts

**NEVER create temporary, alternate, or simplified implementations of existing functionality.** The `operate/` directory contains battle-tested scripts for all common operations:

- **`diagnose.sh`** - Comprehensive system diagnostics
- **`test-connection.sh`** - MySQL connectivity testing
- **`validate-metrics.sh`** - New Relic metric validation
- **`validate-config.sh`** - Configuration validation
- **`generate-workload.sh`** - MySQL workload generation
- **`full-test.sh`** - Complete test suite

When asked to perform operations like testing, validation, or workload generation, ALWAYS use these existing scripts. They handle edge cases, proper error handling, and integrate with the overall system architecture.

Example:
- ❌ DON'T: Create a new `test-mysql.sh` or `check-metrics.py`
- ✅ DO: Use `./operate/test-connection.sh` or `./operate/validate-metrics.sh`

## Troubleshooting Patterns

### No Metrics
1. Check collector logs: `docker compose logs otel-collector`
2. Verify API key: `docker compose exec otel-collector env | grep NEW_RELIC`
3. Test connectivity: `docker exec otel-collector nc -zv otlp.nr-data.net 4318`

### High Memory Usage
- Adjust `GOMEMLIMIT` and `MEMORY_LIMIT_PERCENT`
- Switch to `standard` or `minimal` deployment mode
- Increase collection intervals

### Configuration Issues
- Validate YAML: `./operate/validate-config.sh`
- Check pipeline activation based on deployment mode
- Ensure required environment variables are set