# Quick Start Guide

Build and run the Database Intelligence Collector with working OpenTelemetry components. This guide shows how to build the collector binary and verify the working components.

## Prerequisites

- Docker and Docker Compose
- Task (build automation tool)
- (Optional) Go 1.21+ for building from source
- (Optional) New Relic account with license key for cloud export

## Install Task

```bash
# macOS
brew install go-task/tap/go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin

# Windows (PowerShell)
iwr -useb https://taskfile.dev/install.ps1 | iex
```

## Quick Start Options

### Option 1: Build Working Collector (Recommended)

```bash
# Clone repository
git clone https://github.com/database-intelligence-mvp
cd database-intelligence-mvp

# Install OpenTelemetry Collector Builder
go install go.opentelemetry.io/collector/cmd/builder@v0.127.0

# Build the collector (creates ./dist/database-intelligence-collector)
export PATH="$HOME/go/bin:$PATH"
builder --config=ocb-config.yaml

# Verify successful build
./dist/database-intelligence-collector components
git clone https://github.com/database-intelligence-mvp/database-intelligence-mvp.git
cd database-intelligence-mvp

# Start databases
docker-compose up -d postgres mysql

# Get pre-built minimal collector
wget https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v0.96.0/otelcol-contrib_0.96.0_linux_amd64.tar.gz
tar -xzf otelcol-contrib_0.96.0_linux_amd64.tar.gz

# Run with minimal config
./otelcol-contrib --config=configs/postgresql-maximum-extraction.yaml
```

### Option 2: Build Minimal Collector

```bash
# Install OCB builder
task setup:tools

# Build minimal collector (no custom processors)
task build:minimal

# Run minimal collector
./dist/db-intelligence-minimal --config=config/collector-full-local.yaml
```

### Option 3: Docker Compose (All-in-One)

```bash
# Set up environment
cp .env.example .env
# Edit .env with your database passwords only

# Start everything
task docker:simple

# Or manually
docker-compose --profile simple up -d
```

This starts:
- PostgreSQL (port 5432)
- MySQL (port 3306)
- Minimal collector (metrics on port 8888)

### Option 4: Full Setup with Task

```bash
# Complete automated setup
task quickstart

# This will:
# 1. Install build tools
# 2. Build minimal collector
# 3. Start databases
# 4. Configure environment
# 5. Start collecting metrics
```

## Verify Installation

### 1. Check Collector Health
```bash
# Health endpoint
curl http://localhost:13134/
# Should return "OK"

# Metrics endpoint
curl http://localhost:8888/metrics | head -20
```

### 2. Check Database Metrics
```bash
# PostgreSQL metrics (22 total)
curl -s http://localhost:8888/metrics | grep postgresql_ | wc -l

# MySQL metrics (77 total)
curl -s http://localhost:8888/metrics | grep mysql_ | wc -l
```

### 3. View Specific Metrics
```bash
# Database sizes
curl -s http://localhost:8888/metrics | grep -E "postgresql_db_size|mysql_database_size"

# Connection counts
curl -s http://localhost:8888/metrics | grep -E "postgresql_backends|mysql_threads"
```

## What's Next?

### Add New Relic Export
```yaml
# Add to configs/postgresql-maximum-extraction.yaml
exporters:
  otlp/newrelic:
    endpoint: otlp.nr-data.net:4317
    headers:
      api-key: YOUR_LICENSE_KEY_HERE
    compression: gzip
```

### Add Custom SQL Queries
```yaml
# Add to receivers section
sqlquery/custom:
  driver: postgres
  datasource: "host=localhost port=5432 user=postgres password=postgres dbname=testdb sslmode=disable"
  queries:
    - sql: "SELECT COUNT(*) as active_connections FROM pg_stat_activity"
      metrics:
        - metric_name: custom.active_connections
          value_column: active_connections
```

### Scale Up
```bash
# Add more databases
task db:add-replica

# Enable experimental features
task build MODE=experimental

# Deploy to Kubernetes
task deploy:helm
```

## Environment Configuration

### Minimal Required Variables

For the minimal collector, you only need database credentials:

```bash
# Default values work for Docker Compose setup
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=testdb

MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=mysql
MYSQL_DB=testdb

# Optional: For New Relic export
NEW_RELIC_LICENSE_KEY=your_license_key_here
```

## Troubleshooting Quick Checks

```bash
# No metrics?
curl http://localhost:8888/metrics

# Database connection issues?
docker ps  # Check if databases are running
docker logs db-intelligence-postgres
docker logs db-intelligence-mysql

# Collector not starting?
./dist/db-intelligence-minimal --config=configs/postgresql-maximum-extraction.yaml --log-level=debug
```

## Common Issues

1. **Port already in use**: Stop existing services or change ports in config
2. **Database connection refused**: Wait 10-15 seconds for databases to start
3. **No metrics showing**: Check collector logs for errors
4. **Memory issues**: Reduce batch size in configuration

## Next Steps

- [Full Configuration Guide](CONFIGURATION.md) - Advanced configuration options
- [Troubleshooting Guide](TROUBLESHOOTING.md) - Detailed problem solving
- [Architecture Overview](ARCHITECTURE.md) - Understanding the system
- [Deployment Guide](DEPLOYMENT.md) - Production deployment options

## Getting Help

```bash
# See all available commands
task --list

# Check current status
task status

# View logs
task logs:all
```