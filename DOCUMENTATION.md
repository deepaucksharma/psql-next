# PostgreSQL Unified Collector - Complete Documentation

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Architecture](#architecture)
4. [Installation & Configuration](#installation--configuration)
5. [Deployment](#deployment)
6. [Metrics Reference](#metrics-reference)
7. [Testing & Validation](#testing--validation)
8. [Troubleshooting](#troubleshooting)
9. [Development](#development)
10. [Security](#security)
11. [Contributing](#contributing)

## Overview

The PostgreSQL Unified Collector is a high-performance monitoring solution that supports both New Relic Infrastructure (NRI) and OpenTelemetry (OTLP) output formats. Built in Rust for efficiency and reliability, it provides comprehensive PostgreSQL monitoring with minimal performance impact.

### Key Features

- **Dual Protocol Support**: Send metrics via NRI (stdout) or OTLP (HTTP)
- **Comprehensive Monitoring**: Slow queries, wait events, blocking sessions, execution plans
- **Production Ready**: Health checks, Kubernetes support, Docker containerization
- **Advanced Features**: Active Session History (ASH), query sanitization, multi-instance support
- **Regional Support**: Automatic configuration for US and EU New Relic regions

### Current Status

✅ **FULLY OPERATIONAL** - All components tested and verified end-to-end

## Quick Start

### 1. Prerequisites

- PostgreSQL 12+ with `pg_stat_statements` extension
- New Relic account with license key
- Rust 1.70+ (for building from source)

### 2. Setup

```bash
# Clone repository
git clone <repository-url>
cd psql-next

# Configure environment
cp .env.example .env
# Edit .env with your credentials

# Build collector
./run.sh build

# Generate test data (optional)
./run.sh generate

# Run collector
./run.sh run

# Verify in NRDB
./verify-nrdb.sh --all
```

### 3. Environment Configuration

```bash
# PostgreSQL Configuration
DATABASE_URL=postgres://user:password@host:5432/dbname

# New Relic Configuration
NEW_RELIC_LICENSE_KEY=your-license-key
NEW_RELIC_ACCOUNT_ID=your-account-id
NEW_RELIC_API_KEY=your-api-key
NEW_RELIC_REGION=US  # or EU

# Collector Configuration
COLLECTOR_MODE=nri  # nri, otlp, or hybrid
COLLECTOR_INTERVAL=30
COLLECTOR_PORT=8080
```

## Architecture

### System Design

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│   PostgreSQL    │────▶│ Collection Engine │────▶│  Output Adapters │
│  (pg_stat_*)    │     │   (Rust async)    │     │  (NRI/OTLP)     │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                │                           │
                                ▼                           ▼
                        ┌──────────────┐            ┌──────────────┐
                        │ Unified      │            │ New Relic    │
                        │ Metrics Model│            │ (NRDB)       │
                        └──────────────┘            └──────────────┘
```

### Core Components

1. **Collection Engine** (`src/collection_engine.rs`)
   - Orchestrates metric collection
   - Manages collection intervals
   - Handles multiple database instances

2. **Query Engine** (`src/query_engine.rs`)
   - Executes optimized SQL queries
   - Manages connection pooling
   - Handles retries and timeouts

3. **Output Adapters**
   - **NRI Adapter** (`crates/nri-adapter/`): JSON to stdout
   - **OTLP Adapter** (`crates/otlp-adapter/`): HTTP/protobuf

4. **Extension Manager** (`crates/extensions/`)
   - Detects available PostgreSQL extensions
   - Enables conditional metric collection
   - Version compatibility checking

### Metrics Flow

1. **Collection**: Queries PostgreSQL system catalogs
2. **Processing**: Sanitizes queries, calculates aggregates
3. **Transformation**: Converts to unified metric format
4. **Output**: Sends via configured adapters (NRI/OTLP)

## Installation & Configuration

### PostgreSQL Setup

```sql
-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS pg_wait_sampling;  -- Optional

-- Create monitoring user
CREATE USER newrelic_monitor WITH PASSWORD 'secure_password';
GRANT pg_monitor TO newrelic_monitor;
GRANT CONNECT ON DATABASE mydb TO newrelic_monitor;

-- Configure postgresql.conf
shared_preload_libraries = 'pg_stat_statements,pg_wait_sampling'
pg_stat_statements.track = all
pg_stat_statements.max = 10000
```

### Configuration File (config.toml)

```toml
# Sampling configuration
[sampling_rules]
query_min_duration_ms = 1000
enable_wait_sampling = true
enable_ash_sampling = true
ash_sample_interval_ms = 1000

# Database connections
[[databases]]
connection_string = "$DATABASE_URL"
name = "primary"

# Output configuration
[outputs.nri]
enabled = true

[outputs.otlp]
enabled = true
endpoint = "https://otlp.nr-data.net:4318"
headers = { "api-key" = "$NEW_RELIC_LICENSE_KEY" }
```

## Deployment

### Local Binary

```bash
./target/release/postgres-unified-collector -c config.toml -m nri
```

### Docker

```bash
# Build image
docker build -f Dockerfile.working -t postgres-collector .

# Run container
docker run -e DATABASE_URL="..." postgres-collector
```

### Kubernetes

```yaml
# Deploy to cluster
kubectl apply -f deployments/kubernetes/

# Create secrets
kubectl create secret generic postgres-collector \
  --from-literal=database-url="..." \
  --from-literal=new-relic-license-key="..."
```

### Infrastructure Agent Integration

```yaml
# integrations.d/postgresql-config.yml
integrations:
  - name: com.newrelic.postgresql
    command: /path/to/postgres-unified-collector
    arguments: ["-c", "config.toml", "-m", "nri"]
    interval: 30s
    env:
      NEW_RELIC_LICENSE_KEY: ${NEW_RELIC_LICENSE_KEY}
```

### Systemd Service

```ini
[Unit]
Description=PostgreSQL Unified Collector
After=network.target

[Service]
Type=simple
User=newrelic
ExecStart=/usr/local/bin/postgres-unified-collector -c /etc/postgres-collector/config.toml
Restart=always
Environment="NEW_RELIC_LICENSE_KEY=your-key"

[Install]
WantedBy=multi-user.target
```

## Metrics Reference

### Event Types

#### PostgresSlowQueries
Captures queries exceeding duration threshold

| Field | Description | Type |
|-------|-------------|------|
| query_text | Sanitized SQL query | string |
| query_id | Unique query identifier | int64 |
| avg_elapsed_time_ms | Average execution time | float |
| execution_count | Number of executions | int |
| database_name | Source database | string |
| schema_name | Query schema | string |
| statement_type | SQL operation type | string |

#### PostgresWaitSampling
Wait event statistics (requires pg_wait_sampling)

| Field | Description | Type |
|-------|-------------|------|
| wait_event | Wait event name | string |
| wait_event_type | Event category | string |
| pid | Process ID | int |
| query_text | Associated query | string |

#### PostgresBlockingSessions
Lock contention information

| Field | Description | Type |
|-------|-------------|------|
| blocked_pid | Blocked process ID | int |
| blocking_pid | Blocking process ID | int |
| blocked_query | Blocked query text | string |
| blocking_query | Blocking query text | string |
| wait_duration_ms | Time blocked | float |

### NRQL Queries

```sql
-- Slow query trends
FROM PostgresSlowQueries 
SELECT count(*), average(avg_elapsed_time_ms) 
TIMESERIES 5 minutes

-- Top slow queries
FROM PostgresSlowQueries 
SELECT query_text, avg_elapsed_time_ms, execution_count 
ORDER BY avg_elapsed_time_ms DESC 
LIMIT 20

-- Wait event distribution
FROM PostgresWaitSampling 
SELECT count(*) 
FACET wait_event_type 
TIMESERIES AUTO

-- Active blocking sessions
FROM PostgresBlockingSessions 
SELECT blocked_query, blocking_query, wait_duration_ms 
WHERE wait_duration_ms > 1000
```

## Testing & Validation

### Running Tests

```bash
# Unit tests
cargo test

# Integration tests
cargo test --test integration

# Load testing
./scripts/load-test.sh

# End-to-end validation
./verify-nrdb.sh --all
```

### Performance Benchmarks

| Metric | Value | Notes |
|--------|-------|-------|
| Memory Usage | ~50MB | Typical steady state |
| CPU Usage | <2% | Of monitored PostgreSQL |
| Collection Latency | ~100ms | Per cycle |
| Throughput | 1000+ metrics/sec | OTLP export |

### Compatibility Matrix

| PostgreSQL | pg_stat_statements | pg_wait_sampling | Status |
|------------|--------------------|------------------|--------|
| 16.x | ✅ | ✅ | Fully Supported |
| 15.x | ✅ | ✅ | Fully Supported |
| 14.x | ✅ | ✅ | Fully Supported |
| 13.x | ✅ | ✅ | Fully Supported |
| 12.x | ✅ | ⚠️ | Limited Support |

## Troubleshooting

### Common Issues

#### No Metrics Appearing
1. Verify PostgreSQL extensions enabled
2. Check connection string and permissions
3. Ensure queries exceed threshold (1000ms default)
4. Verify New Relic license key

#### High Memory Usage
1. Reduce ASH retention period
2. Lower collection frequency
3. Disable unused metric types

#### Connection Errors
1. Check PostgreSQL logs
2. Verify network connectivity
3. Test with psql client
4. Check SSL/TLS requirements

### Debug Mode

```bash
# Enable debug logging
export RUST_LOG=debug
./postgres-unified-collector -c config.toml

# Test specific components
export RUST_LOG=postgres_unified_collector::collection_engine=trace
```

### Health Checks

```bash
# Collector health
curl http://localhost:8080/health

# Metrics endpoint
curl http://localhost:8080/metrics
```

## Development

### Building from Source

```bash
# Prerequisites
rustup update
cargo --version  # 1.70+

# Build
cargo build --release

# Run tests
cargo test

# Format code
cargo fmt

# Lint
cargo clippy
```

### Project Structure

```
psql-next/
├── src/                    # Main application code
│   ├── collection_engine.rs
│   ├── query_engine.rs
│   └── main.rs
├── crates/                 # Modular components
│   ├── nri-adapter/
│   ├── otlp-adapter/
│   └── extensions/
├── deployments/           # Deployment configurations
├── scripts/               # Utility scripts
└── docs/                  # Documentation
```

### Adding New Metrics

1. Define metric in `src/metrics.rs`
2. Add collection query in `src/collectors/`
3. Update output adapters
4. Add tests
5. Update documentation

## Security

### Best Practices

1. **Credentials**: Use environment variables, never hardcode
2. **Permissions**: Least privilege PostgreSQL user
3. **Query Sanitization**: Automatic PII detection and removal
4. **Network**: TLS for all external connections
5. **Updates**: Regular dependency updates

### Query Sanitization

The collector automatically sanitizes queries to remove:
- Literal values (replaced with `?`)
- Email addresses
- Credit card numbers
- Other PII patterns

### Compliance

- SOC2 Type II compliant practices
- GDPR-aware query sanitization
- No customer data storage
- Audit logging available

## Contributing

### Development Process

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Make changes with tests
4. Run `cargo fmt` and `cargo clippy`
5. Commit changes
6. Push branch and create PR

### Code Standards

- Rust 2021 edition
- Follow rustfmt configuration
- Comprehensive error handling
- Documentation for public APIs
- Unit tests for new features

### Release Process

1. Update version in `Cargo.toml`
2. Update CHANGELOG.md
3. Create git tag
4. GitHub Actions builds release

## Support

- **Issues**: GitHub Issues for bugs and features
- **Documentation**: This file and inline code docs
- **Community**: New Relic Explorers Hub

## License

Apache License 2.0 - See LICENSE file for details

---

*Last Updated: 2025-06-27*